package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
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
	// temps diu on ha anat el temps del torn (crides al model i eines).
	temps tempsTorn
}

// tempsTorn és el recompte final d'un loop: passos de model i temps de cada part.
type tempsTorn struct {
	passos int
	model  time.Duration
	eines  time.Duration
}

// loopTemps compta passos i temps d'un loop de lectura, per saber on s'espera.
type loopTemps struct {
	mu     sync.Mutex
	passos int
	model  time.Duration
	eines  time.Duration
}

func (t *loopTemps) addModel(d time.Duration) { t.mu.Lock(); t.passos++; t.model += d; t.mu.Unlock() }
func (t *loopTemps) addEina(d time.Duration)  { t.mu.Lock(); t.eines += d; t.mu.Unlock() }

// String resumeix el torn: «3 passos · model 21.4s · eines 9.8s».
func (t tempsTorn) String() string {
	return fmt.Sprintf("%d passos · model %.1fs · eines %.1fs", t.passos, t.model.Seconds(), t.eines.Seconds())
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
	return s.runReadOnlyLoopForRoleStream(ctx, brief, maxSteps, p, role, nil, prev...)
}

// runReadOnlyLoopForRoleStream és el mateix loop amb emissió en directe:
// emit rep ("token", fragment) del text que genera el model i ("phase",
// noms d'eines separats per coma) quan un pas acaba en crides. Els tokens
// d'un pas que acaba en eines són especulatius (poden ser JSON d'eina):
// el client els descarta quan arriba la fase. Amb emit == nil les crides
// són les clàssiques (comportament idèntic al d'abans).
func (s *Server) runReadOnlyLoopForRoleStream(ctx context.Context, brief string, maxSteps int, p config.Provider, role config.Role, emit func(kind, text string), prev ...[]llm.Message) (exploracio, error) {
	stream := emit != nil
	if emit == nil {
		emit = func(_, _ string) {}
	}
	var temps loopTemps
	loop := &agent.Loop{
		MaxSteps: maxSteps,
		Lang:     s.cfg.Lang(),
		Step: func(ctx context.Context, h []llm.Message) (string, []llm.ToolCall, error) {
			cctx, cancel := context.WithTimeout(ctx, 900*time.Second)
			defer cancel()
			defer func(t0 time.Time) { temps.addModel(time.Since(t0)) }(time.Now())
			if !stream {
				content, calls, _, err := s.client.ChatWithToolsFO(cctx, s.cfg.PrimTarget(p, role), s.cfg.FallbackTarget(role), h, role.Temperature, role.MaxTokens, agent.SpecsAll(), nil)
				return content, calls, err
			}
			content, calls, _, err := s.client.ChatStreamWithToolsFO(cctx, s.cfg.PrimTarget(p, role), s.cfg.FallbackTarget(role), h, role.Temperature, role.MaxTokens, agent.SpecsAll(), func(tok string) { emit("token", tok) }, nil, nil)
			if err == nil && len(calls) > 0 {
				names := make([]string, 0, len(calls))
				for _, c := range calls {
					names = append(names, c.Function.Name)
				}
				emit("phase", strings.Join(names, ","))
			}
			return content, calls, err
		},
		// La resposta final va sense eines: cap crida no es pot executar.
		Finalize: func(ctx context.Context, h []llm.Message) (string, error) {
			cctx, cancel := context.WithTimeout(ctx, agent.TimeoutDecisio)
			defer cancel()
			defer func(t0 time.Time) { temps.addModel(time.Since(t0)) }(time.Now())
			if !stream {
				out, _, err := s.client.ChatFO(cctx, s.cfg.PrimTarget(p, role), s.cfg.FallbackTarget(role), h, role.Temperature, role.MaxTokens, nil)
				return out, err
			}
			out, _, _, err := s.client.ChatStreamWithToolsFO(cctx, s.cfg.PrimTarget(p, role), s.cfg.FallbackTarget(role), h, role.Temperature, role.MaxTokens, nil, func(tok string) { emit("token", tok) }, nil, nil)
			return out, err
		},
		RunTool: func(ctx context.Context, name, argsJSON string) (string, []string, error) {
			defer func(t0 time.Time) { temps.addEina(time.Since(t0)) }(time.Now())
			dirs := append([]string{}, agent.SkillsDirsOf(ctx)...)
			dirs = append(dirs, s.userSkillsDirs()...)
			return agent.ExecCtx(agent.WithSkillsDirs(ctx, dirs...), s.id, s.cwd, name, argsJSON)
		},
		Decide: func(name, argsJSON string) (string, string) {
			// Els vents poden programar avisos (remind, eina validada); la
			// resta segueix el permís del mode inspect (només lectura).
			if name == "remind" {
				return "allow", "eina del vent"
			}
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
	res := exploracio{text: strings.TrimSpace(out), parcial: loop.Parcial, motiu: loop.Motiu, hist: loop.Hist}
	temps.mu.Lock()
	res.temps = tempsTorn{passos: temps.passos, model: temps.model, eines: temps.eines}
	temps.mu.Unlock()
	return res, err
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
