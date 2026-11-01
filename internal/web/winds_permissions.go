package web

import (
	"context"

	"gregal/internal/agent"
)

// Only intentional HTTP wind/group turns carry this marker. Jobs, plans,
// heartbeat and crew/routine calls to the same loop must not inherit Actua.
// Capture the session scope once so a folder/access change cannot make the
// permission decision refer to a different workspace than tool execution.
type interactiveWindKey struct{}
type windTurnScope struct {
	mode      string
	workspace string
	policy    *agent.Policy
}

func (s *Server) windLoopScope() windTurnScope {
	s.mu.Lock()
	ws, repo := s.ventWorkspaceLocked()
	pol := agent.PolicyFromConfig(s.cfg, ws)
	if s.policy != nil {
		cp := *s.policy
		cp.ProjectDir = ws
		// Els projectes viuen fora del repo: si el vent treballa a
		// ~/gregal/projectes, el repo segueix essent escrivible (una feina
		// sobre el Gregal mateix no s'ha de trencar per això).
		if repo != "" && repo != ws {
			cp.AlsoDirs = append(append([]string{}, cp.AlsoDirs...), repo)
		}
		cp.AskEdits = s.cfg.Permissions.AskEdits()
		pol = &cp
	}
	s.mu.Unlock()
	return windTurnScope{mode: s.mode, workspace: ws, policy: pol}
}

func (s *Server) interactiveWindCtx(ctx context.Context) context.Context {
	scope := s.windLoopScope()
	// The full/autonomous policy upgrades pending asks to allow. Wind chat
	// has no approval UI: retain code permissions, never that upgrade.
	if scope.mode == agent.ModeAutonomous {
		scope.mode = agent.ModeCode
	}
	if !agent.ValidMode(scope.mode) {
		scope.mode = agent.ModeInspect
	}
	return context.WithValue(ctx, interactiveWindKey{}, scope)
}

const windRequiresApproval = "REQUIRES APPROVAL: this action was not run. Wind chat cannot request approval yet; use the main agent chat to review and approve it. No permission was granted."
