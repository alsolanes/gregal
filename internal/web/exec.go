package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/tools"
)

// G8 — processos en segon pla des dels clients. El terminal de
// l'escriptori i l'eina bash_background comparteixen el mateix magatzem:
// el que engega l'agent es veu al panell i a l'inrevés.

// handleExec llista (GET) o engega (POST) processos de la sessió.
func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, map[string]any{"procs": agent.Procs().List(s.id)})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "només GET o POST", 405)
		return
	}
	var req struct {
		Cmd string `json:"cmd"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Cmd) == "" {
		http.Error(w, "cal una ordre", 400)
		return
	}
	// Mateixa política que el bash de l'agent: el que està denegat a
	// l'agent tampoc s'engega des del terminal.
	if d, why := tools.ClassifyWith(req.Cmd, s.policyBashAllow(), s.policyBashDeny()); d == "deny" {
		http.Error(w, "ordre bloquejada: "+why, 403)
		return
	}
	s.mu.Lock()
	dir := s.cwd
	s.mu.Unlock()
	p, err := agent.Procs().Start(s.id, dir, req.Cmd)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"id": p.ID, "cmd": p.Cmd, "dir": p.Dir, "running": p.IsRunning()})
}

func (s *Server) policyBashAllow() []string {
	if s.policy == nil {
		return nil
	}
	return s.policy.BashAllow
}

func (s *Server) policyBashDeny() []string {
	if s.policy == nil {
		return nil
	}
	return s.policy.BashDeny
}

// handleExecOutput retorna les línies noves (from = última vista).
func (s *Server) handleExecOutput(w http.ResponseWriter, r *http.Request) {
	p := agent.Procs().Get(r.URL.Query().Get("id"))
	if p == nil {
		http.Error(w, "procés desconegut", 404)
		return
	}
	from, _ := strconv.Atoi(r.URL.Query().Get("from"))
	lines, done, exit := p.Output(from)
	writeJSON(w, map[string]any{"lines": lines, "done": done, "exit": exit, "running": p.IsRunning()})
}

// handleExecStream envia la sortida en viu per SSE (events "line" i "end").
func (s *Server) handleExecStream(w http.ResponseWriter, r *http.Request) {
	p := agent.Procs().Get(r.URL.Query().Get("id"))
	if p == nil {
		http.Error(w, "procés desconegut", 404)
		return
	}
	emit := sse(w)
	from, _ := strconv.Atoi(r.URL.Query().Get("from"))
	ctx := r.Context()
	for {
		lines, done, exit := p.Output(from)
		for _, l := range lines {
			emit("line", l)
			from = l.N
		}
		if done {
			code := 0
			if exit != nil {
				code = *exit
			}
			emit("end", map[string]any{"exit": code})
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(300 * time.Millisecond):
		}
	}
}

// handleExecKill atura un procés.
func (s *Server) handleExecKill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.ID == "" {
		http.Error(w, "cal un id", 400)
		return
	}
	if !agent.Procs().Kill(req.ID) {
		http.Error(w, "procés desconegut", 404)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "killed": req.ID})
}
