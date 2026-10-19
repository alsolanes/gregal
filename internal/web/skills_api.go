package web

import (
	"encoding/json"
	"net/http"

	"gregal/internal/skills"
)

type skillPreference struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.skillsResponse())
	case http.MethodPut:
		var req struct {
			Enabled []string `json:"enabled"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Enabled == nil {
			http.Error(w, "expected an enabled array of specialist skill names", http.StatusBadRequest)
			return
		}
		if err := skills.ValidaHabilitades(req.Enabled); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		var old []string
		if s.cfg.Skills.Enabled != nil {
			old = append([]string{}, s.cfg.Skills.Enabled...)
		}
		s.cfg.Skills.Enabled = append([]string{}, req.Enabled...)
		if err := s.cfg.Save(s.cfgPath); err != nil {
			s.cfg.Skills.Enabled = old
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := skills.DefineixHabilitades(req.Enabled); err != nil {
			s.cfg.Skills.Enabled = old
			_ = s.cfg.Save(s.cfgPath)
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Unlock()
		writeJSON(w, s.skillsResponse())
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) skillsResponse() map[string]any {
	on := make(map[string]bool)
	for _, name := range skills.Habilitades() {
		on[name] = true
	}
	items := make([]skillPreference, 0)
	for _, item := range skills.Curades() {
		items = append(items, skillPreference{
			Name: item.Nom, Title: item.Titol, Description: item.Descripcio, Enabled: on[item.Nom],
		})
	}
	return map[string]any{"skills": items}
}
