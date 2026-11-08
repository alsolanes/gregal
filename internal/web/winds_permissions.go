package web

import (
	"context"
	"strings"

	"gregal/internal/agent"
	"gregal/internal/llm"
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
	mode := s.mode
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
	// Poders de servidor (server_powers.go): només l'administrador.
	if s.hasServerPowers() {
		cp := *pol
		cp.ServerPowers, cp.ServerHome = true, serviceHome()
		pol = &cp
	}
	s.mu.Unlock()
	return windTurnScope{mode: mode, workspace: ws, policy: pol}
}

// interactiveWindCtx és el context dels torns interactius dels vents (xat
// 1:1, rotllanes): porta l'abast del torn. steerKey hi afegeix la clau de
// la bústia de missatges escrits mentre el torn treballa (winds_steer.go).
type windSteerKey struct{}

func (s *Server) withWindSteerKey(ctx context.Context, kind, id string) context.Context {
	return context.WithValue(ctx, windSteerKey{}, turnKey{scope: s.sessDir(), kind: kind, id: id})
}

func windSteerKeyOf(ctx context.Context) (turnKey, bool) {
	k, ok := ctx.Value(windSteerKey{}).(turnKey)
	return k, ok
}

func (s *Server) interactiveWindCtx(ctx context.Context) context.Context {
	scope := s.windLoopScope()
	scope.mode = windTurnMode(scope.mode)
	return context.WithValue(ctx, interactiveWindKey{}, scope)
}

func (s *Server) readOnlyWindCtx(ctx context.Context) context.Context {
	scope, ok := ctx.Value(interactiveWindKey{}).(windTurnScope)
	if !ok {
		scope = s.windLoopScope()
	}
	scope.mode = agent.ModeInspect
	return context.WithValue(ctx, interactiveWindKey{}, scope)
}

// windTurnMode keeps read-only server modes read-only for ordinary wind turns.
// A request to a specialist does not grant permission to write when the
// session itself is in question, plan, or inspect mode.
func windTurnMode(mode string) string {
	if agent.ReadOnlyMode(mode) || mode == agent.ModeDebug {
		return mode
	}
	// Autonomous permission is not inherited by a wind turn without an
	// interactive approval surface.
	return agent.ModeCode
}

// windStudyCtx és el context d'un estudi («aprèn X»): el vent pot escriure i
// executar només a la seva carpeta de pràctiques, per provar el que aprèn.
func (s *Server) windStudyCtx(ctx context.Context, dir string) context.Context {
	scope := s.windLoopScope()
	pol := *scope.policy
	pol.ProjectDir = dir
	pol.AlsoDirs = nil
	pol.AskEdits = false
	scope.policy, scope.workspace, scope.mode = &pol, dir, agent.ModeCode
	return context.WithValue(ctx, interactiveWindKey{}, scope)
}

// windSpecs són les eines dels torns de vent: totes menys `wind` (un vent no
// n'invoca un altre: ho fan les colles i l'agent principal).
func windSpecs() []llm.ToolSpec {
	all := agent.SpecsAll()
	out := make([]llm.ToolSpec, 0, len(all))
	for _, t := range all {
		if t.Name != "wind" {
			out = append(out, t)
		}
	}
	return out
}

// windNotRun és el que rep el model quan una acció necessita una aprovació
// que el xat del vent no pot demanar. Ha de seguir treballant per una altra
// via, no aturar-se ni demanar a la persona que vagi al xat principal.
const windNotRun = "NOT RUN: this needs the user's explicit approval and wind chat cannot ask for it. Do not retry it and do not stop: find another way inside the project folder (servers: the app tool; stopping something you started: bash_kill or app stop; downloads: web_fetch or a CDN link; files: only inside the project)."

// windSkippedNote és la línia que va al final de la resposta quan alguna
// acció no s'ha fet.
func windSkippedNote(reasons []string) string {
	seen := map[string]bool{}
	var uniq []string
	for _, r := range reasons {
		// El motiu pot portar l'ordre sencera (un heredoc de 50 línies):
		// a la nota, només la primera línia i curta.
		r, _, _ = strings.Cut(strings.TrimSpace(r), "\n")
		if rr := []rune(r); len(rr) > 90 {
			r = string(rr[:90]) + "…"
		}
		if r != "" && !seen[r] {
			seen[r] = true
			uniq = append(uniq, r)
		}
	}
	note := "(Not run — needs your approval in the main chat"
	if len(uniq) > 0 {
		if len(uniq) > 3 {
			uniq = uniq[:3]
		}
		note += ": " + strings.Join(uniq, "; ")
	}
	return note + ".)"
}
