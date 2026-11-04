package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"gregal/internal/config"
	"gregal/internal/session"
)

// thinkScope és la clau (dins modelOverride, que ja es desa per usuari) del
// nivell de raonament triat a la UI. No és un rol: se superposa al think del
// rol actiu. «default» o res = el que digui el config del rol.
const thinkScope = "~think"

// thinkLevels són els nivells que la UI ofereix, en ordre. «auto» raona on
// aporta (agent.ThinkPerPas) i «no» l'apaga.
var thinkLevels = []string{"default", "auto", "no", "low", "medium", "high", "max"}

func validThinkLevel(l string) bool {
	for _, x := range thinkLevels {
		if x == l {
			return true
		}
	}
	return false
}

// thinkLevel és el nivell triat (buit = el del config). Cal s.mu agafat.
func (s *Server) thinkLevel() string {
	l := s.modelOverride[thinkScope]
	if l == "default" || !validThinkLevel(l) {
		return ""
	}
	return l
}

// withThinkLevel aplica el nivell triat a un rol. El revisor no el rep: el seu
// think és explícit i curt (veredicte d'una línia).
func (s *Server) withThinkLevel(name string, r config.Role) config.Role {
	if name == "reviewer" {
		return r
	}
	if l := s.thinkLevel(); l != "" {
		r.Think = l
	}
	return r
}

// POST /api/think {level}: canvia el nivell de raonament i el desa per usuari.
func (s *Server) handleThink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "mètode no permès", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Level string `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "petició il·legible", http.StatusBadRequest)
		return
	}
	level := strings.ToLower(strings.TrimSpace(req.Level))
	if !validThinkLevel(level) {
		http.Error(w, "nivell desconegut", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := session.SaveModelDefault(s.usuari(), thinkScope, level); err != nil {
		http.Error(w, "no puc desar el nivell: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.modelOverride[thinkScope] = level
	s.autosaveLocked()
	writeJSON(w, map[string]any{"think": level, "levels": thinkLevels})
}
