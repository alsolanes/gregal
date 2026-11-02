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
	return windTurnScope{mode: s.mode, workspace: ws, policy: pol}
}

func (s *Server) interactiveWindCtx(ctx context.Context) context.Context {
	scope := s.windLoopScope()
	scope.mode = windTurnMode(scope.mode)
	return context.WithValue(ctx, interactiveWindKey{}, scope)
}

// windTurnMode és el mode d'un torn que la persona ha demanat a un vent.
//
// El mode del servidor és el del xat principal: «Pregunta» (chat) vol dir que
// l'agent principal no toca res, però un vent és un especialista a qui la
// persona encarrega feina («fes-me un joc»): heretar-ne el només-lectura feia
// que el vent no pogués ni crear la carpeta del projecte. Per això els torns
// de vent treballen en mode code (escriure i executar dins del projecte, amb
// WindAutoAllow) llevat que s'hagi triat «Planifica» (plan), que demana
// explícitament no actuar. L'autònom tampoc no s'hereta: el seu «tot ask és
// allow» no té lloc en un xat sense aprovacions.
func windTurnMode(mode string) string {
	switch mode {
	case agent.ModePlan:
		return agent.ModePlan
	case agent.ModeDebug:
		return agent.ModeDebug
	}
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
