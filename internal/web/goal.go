package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gregal/internal/goal"
)

// goalDirFor retorna on es desen els objectius (al costat del config).
func goalDirFor(cfgPath string) string {
	if cfgPath != "" {
		if dir := filepath.Dir(cfgPath); dir != "" && dir != "." {
			return dir
		}
	}
	if base, err := os.UserConfigDir(); err == nil {
		return filepath.Join(base, "gregal")
	}
	return "."
}

// projectName és el nom del projecte actual.
func (s *Server) projectName() string {
	return filepath.Base(strings.TrimRight(s.cwd, string(filepath.Separator)))
}

// recordGoal desa l'objectiu d'una resposta de l'agent (mode objectiu).
func (s *Server) recordGoal(reply string) (goal.Goal, bool) {
	g, ok := goal.Parse(reply, s.cwd)
	if !ok {
		return goal.Goal{}, false
	}
	if err := goal.Save(s.goalDir, g); err != nil {
		return goal.Goal{}, false
	}
	s.mu.Lock()
	s.lastGoalID = g.ID
	s.mu.Unlock()
	return g, true
}

// emitGoal desa (si cal) l'objectiu de la resposta i l'envia a la UI.
func (s *Server) emitGoal(emit func(string, any), reply string) {
	g, ok := s.recordGoal(reply)
	if !ok {
		return
	}
	emit("goal", map[string]any{
		"id": g.ID, "title": g.Title, "body": g.Body,
		"status": g.Status, "project": g.Project,
	})
}

// handleGoal llista, prepara i esborra objectius.
func (s *Server) handleGoal(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.mu.Lock()
		últim := s.lastGoalID
		s.mu.Unlock()
		llista, err := goal.List(s.goalDir, s.projectName())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if llista == nil {
			llista = []goal.Goal{}
		}
		writeJSON(w, map[string]any{"goals": llista, "last": últim, "project": s.projectName()})
		return
	}
	var req struct {
		Action string `json:"action"` // task|delete|last
		ID     string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "json invàlid", 400)
		return
	}
	g, err := s.resolveGoal(req.ID)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	switch req.Action {
	case "task":
		// La UI engega l'agent amb aquesta tasca en mode code.
		g.Status = goal.StatusFet
		if err := goal.Save(s.goalDir, g); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		s.mu.Lock()
		s.lastGoalID = g.ID
		s.mu.Unlock()
		writeJSON(w, map[string]string{"id": g.ID, "title": g.Title, "task": goal.Task(g)})
	case "delete":
		if err := goal.Delete(s.goalDir, g.ID); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		s.mu.Lock()
		if s.lastGoalID == g.ID {
			s.lastGoalID = ""
		}
		s.mu.Unlock()
		writeJSON(w, map[string]string{"deleted": g.ID})
	default:
		http.Error(w, "acció desconeguda (task|delete)", 400)
	}
}

// resolveGoal troba l'objectiu per id o el més recent del projecte.
func (s *Server) resolveGoal(id string) (goal.Goal, error) {
	if id == "" {
		s.mu.Lock()
		id = s.lastGoalID
		s.mu.Unlock()
	}
	if id != "" {
		if g, err := goal.Get(s.goalDir, id); err == nil {
			return g, nil
		}
	}
	llista, err := goal.List(s.goalDir, s.projectName())
	if err != nil {
		return goal.Goal{}, err
	}
	if len(llista) == 0 {
		return goal.Goal{}, os.ErrNotExist
	}
	return llista[0], nil
}
