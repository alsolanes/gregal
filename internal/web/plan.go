package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/llm"
)

// planSystem obliga exploració read-only i tanca amb pla numerat + verificació
// (mateix contracte que el /plan del TUI).
const planSystem = agent.PlanSystem

// runReadOnlyLoop fa un loop d'exploració (decisions inspect: escriure queda
// registrat com a no disponible, mai demana res). Nucli compartit de jobs i /api/plan.
func (s *Server) runReadOnlyLoop(ctx context.Context, brief string, maxSteps int) (string, error) {
	s.mu.Lock()
	p, role := s.roleRef()
	s.mu.Unlock()
	loop := &agent.Loop{
		MaxSteps: maxSteps,
		Step: func(ctx context.Context, h []llm.Message) (string, []llm.ToolCall, error) {
			cctx, cancel := context.WithTimeout(ctx, 180*time.Second)
			defer cancel()
			content, calls, _, err := s.client.ChatWithToolsFO(cctx, s.cfg.PrimTarget(p, role), s.cfg.FallbackTarget(role), h, role.Temperature, role.MaxTokens, agent.SpecsAll(), nil)
			return content, calls, err
		},
		RunTool: func(ctx context.Context, name, argsJSON string) (string, []string, error) {
			return agent.ExecIn(s.id, s.cwd, name, argsJSON)
		},
		Decide: func(name, argsJSON string) (string, string) {
			return s.policy.Decide(agent.ModeInspect, name, argsJSON)
		},
	}
	out, _, err := loop.Run(ctx, brief)
	return strings.TrimSpace(out), err
}

// planBrief munta la consigna d'exploració amb memòria del projecte.
func (s *Server) planBrief(task string) string {
	return agent.PlanBrief(s.cwd, task)
}

// handlePlan explora en read-only i torna el pla (sense actuar).
func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var req struct {
		Task string `json:"task"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Task) == "" {
		http.Error(w, "tasca buida", 400)
		return
	}
	s.mu.Lock()
	if s.agentBusy || s.jobRunning || s.planRunning {
		s.mu.Unlock()
		http.Error(w, "agent ocupat: torna-ho a provar", 409)
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
	plan, err := s.runReadOnlyLoop(ctx, s.planBrief(strings.TrimSpace(req.Task)), s.cfg.Agent.MaxSteps)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	if plan == "" {
		http.Error(w, "exploració buida", 502)
		return
	}
	writeJSON(w, map[string]string{"plan": plan})
}
