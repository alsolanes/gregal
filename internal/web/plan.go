package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// planSystem obliga exploració read-only i tanca amb pla numerat + verificació
// (mateix contracte que el /plan del TUI).
const planSystem = agent.PlanSystem

// exploracio és el resultat d'un loop read-only: el text, si és parcial
// (pressupost o error a mig camí: vegeu agent.Loop) i l'historial per
// poder-lo continuar.
type exploracio struct {
	text    string
	parcial bool
	motiu   string
	hist    []llm.Message
}

// runReadOnlyLoop fa un loop d'exploració (decisions inspect: escriure queda
// registrat com a no disponible, mai demana res). Nucli compartit de jobs i
// /api/plan. Amb prev, continua aquella exploració amb pressupost nou.
func (s *Server) runReadOnlyLoop(ctx context.Context, brief string, maxSteps int, prev ...[]llm.Message) (exploracio, error) {
	s.mu.Lock()
	p, role := s.roleRef()
	s.mu.Unlock()
	return s.runReadOnlyLoopForRole(ctx, brief, maxSteps, p, role, prev...)
}

// runReadOnlyLoopForRole és el mateix loop amb target explícit: els vents
// (xat, rotllana i cor) hi passen el seu (windRoleRef: rol base + model
// per defecte propi), la feina hi passa el del rol.
func (s *Server) runReadOnlyLoopForRole(ctx context.Context, brief string, maxSteps int, p config.Provider, role config.Role, prev ...[]llm.Message) (exploracio, error) {
	loop := &agent.Loop{
		MaxSteps: maxSteps,
		Lang:     s.cfg.Lang(),
		Step: func(ctx context.Context, h []llm.Message) (string, []llm.ToolCall, error) {
			cctx, cancel := context.WithTimeout(ctx, 180*time.Second)
			defer cancel()
			content, calls, _, err := s.client.ChatWithToolsFO(cctx, s.cfg.PrimTarget(p, role), s.cfg.FallbackTarget(role), h, role.Temperature, role.MaxTokens, agent.SpecsAll(), nil)
			return content, calls, err
		},
		// La resposta final va sense eines: cap crida no es pot executar.
		Finalize: func(ctx context.Context, h []llm.Message) (string, error) {
			cctx, cancel := context.WithTimeout(ctx, agent.TimeoutDecisio)
			defer cancel()
			out, _, err := s.client.ChatFO(cctx, s.cfg.PrimTarget(p, role), s.cfg.FallbackTarget(role), h, role.Temperature, role.MaxTokens, nil)
			return out, err
		},
		RunTool: func(ctx context.Context, name, argsJSON string) (string, []string, error) {
			return agent.ExecCtx(agent.WithSkillsDirs(ctx, s.userSkillsDirs()...), s.id, s.cwd, name, argsJSON)
		},
		Decide: func(name, argsJSON string) (string, string) {
			return s.policy.Decide(agent.ModeInspect, name, argsJSON)
		},
	}
	var out string
	var err error
	if len(prev) > 0 && len(prev[0]) > 0 {
		out, _, err = loop.Continua(ctx, prev[0])
	} else {
		out, _, err = loop.Run(ctx, brief)
	}
	return exploracio{text: strings.TrimSpace(out), parcial: loop.Parcial, motiu: loop.Motiu, hist: loop.Hist}, err
}

// planBrief munta la consigna d'exploració amb memòria del projecte.
func (s *Server) planBrief(task string) string {
	return agent.PlanBrief(s.cwd, task, s.cfg.Lang())
}

// handlePlan explora en read-only i torna el pla (sense actuar).
func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var req struct {
		Task string `json:"task"`
		// Continue reprèn l'última exploració parcial amb pressupost nou i
		// la conversa intacta (el botó «Continua explorant»).
		Continue bool `json:"continue"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (strings.TrimSpace(req.Task) == "" && !req.Continue) {
		http.Error(w, "tasca buida", 400)
		return
	}
	s.mu.Lock()
	if s.agentBusy || s.jobRunning || s.planRunning {
		s.mu.Unlock()
		http.Error(w, "agent ocupat: torna-ho a provar", 409)
		return
	}
	prevHist, prevTask := s.planHist, s.planTask
	if req.Continue && len(prevHist) == 0 {
		s.mu.Unlock()
		http.Error(w, "no hi ha cap exploració per continuar", 409)
		return
	}
	s.planRunning = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.planRunning = false
		s.mu.Unlock()
	}()
	defer func() {
		if rec := recover(); rec != nil {
			http.Error(w, fmt.Sprintf("exploració interrompuda: %v", rec), 500)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	task := strings.TrimSpace(req.Task)
	var res exploracio
	var err error
	if req.Continue {
		task = prevTask
		res, err = s.runReadOnlyLoop(ctx, "", agent.PassosPla(s.cfg.Agent.MaxSteps), prevHist)
	} else {
		res, err = s.runReadOnlyLoop(ctx, s.planBrief(task), agent.PassosPla(s.cfg.Agent.MaxSteps))
	}
	s.mu.Lock()
	s.planHist, s.planTask = res.hist, task
	s.mu.Unlock()
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	plan := res.text
	if plan == "" {
		http.Error(w, "exploració buida", 502)
		return
	}
	out := map[string]any{"plan": plan}
	if res.parcial {
		// Parcial: el pressupost (o el model) s'ha acabat abans que el pla.
		// La UI ho marca i ofereix continuar.
		out["partial"] = true
		out["partial_reason"] = res.motiu
		out["max_steps"] = agent.PassosPla(s.cfg.Agent.MaxSteps)
	}
	// Crític del pla (plan.critic, per defecte on): problemes, no un pla nou.
	if c := s.planCritic(ctx, task, plan); c != nil {
		out["critic"] = c
	}
	writeJSON(w, out)
}
