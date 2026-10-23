package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"gregal/internal/companion"
)

func (s *Server) writeWindProfile(w http.ResponseWriter, wind companion.Wind) {
	writeJSON(w, map[string]any{
		"wind":            wind,
		"knowledge":       s.windBrain(wind.ID),
		"personal_memory": s.personalMemory(),
		"memory":          companion.LoadWindMemory(s.windStateDir(), wind.ID),
		"skills":          s.windSkills(wind.ID),
	})
}

func (s *Server) handleWindProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	wind, ok := s.winds.ByID(strings.TrimSpace(r.URL.Query().Get("wind")))
	if !ok {
		http.Error(w, "unknown wind", http.StatusBadRequest)
		return
	}
	s.writeWindProfile(w, wind)
}

func (s *Server) handleWindMemory(w http.ResponseWriter, r *http.Request) {
	windLibraryMu.Lock()
	defer windLibraryMu.Unlock()
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Wind string  `json:"wind"`
		Text *string `json:"text"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := d.Decode(new(any)); err != io.EOF {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	wind, ok := s.winds.ByID(strings.TrimSpace(req.Wind))
	if !ok || req.Text == nil || len(*req.Text) > companion.MaxWindKnowledgeBytes {
		http.Error(w, "unknown wind or invalid text (max 24576 bytes)", http.StatusBadRequest)
		return
	}
	if err := companion.ReplaceWindBrain(s.windStateDir(), wind.ID, *req.Text); err != nil {
		http.Error(w, "no s'ha pogut desar el coneixement", http.StatusInternalServerError)
		return
	}
	s.writeWindProfile(w, wind)
}
