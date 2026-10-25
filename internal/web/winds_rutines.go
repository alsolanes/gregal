package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gregal/internal/jobs"
)

// Les rutines d'un vent: feines programades (daily/interval) que, quan toca,
// obren un torn real del vent — LLM amb la seva persona, memòria i eines
// (web_search, web_fetch, web_images…) — i parlen al seu xat només si el
// que han trobat val la pena. A diferència dels recordatoris (remind), que
// només repeteixen un text ja escrit, aquí el vent decideix cada cop: pot
// buscar, comparar, fer comptes i callar si no hi ha res.

const rutinaMaxPrompt = 2000

// RutinaInfo és una rutina tal com la veu l'app a «Programats».
type RutinaInfo struct {
	ID         string     `json:"id"`
	Wind       string     `json:"wind"`
	WindName   string     `json:"wind_name"`
	Name       string     `json:"name"`
	Kind       string     `json:"kind"` // "daily" | "interval"
	Time       string     `json:"time,omitempty"`
	IntervalH  float64    `json:"interval_h,omitempty"`
	Prompt     string     `json:"prompt"`
	Enabled    bool       `json:"enabled"`
	NextDue    string     `json:"next_due,omitempty"`
	NextDueAt  string     `json:"next_due_at,omitempty"`
	LastAt     *time.Time `json:"last_at,omitempty"`
	LastStatus string     `json:"last_status,omitempty"`
}

// handleWindsRutina crea una rutina de vent des de l'app. POST amb
// {wind, kind: "daily"|"interval", time: "HH:MM" | interval_h: hores,
//
//	prompt, name?}. La rutina corre el vent de veres: LLM + eines.
func (s *Server) handleWindsRutina(w http.ResponseWriter, r *http.Request) {
	if s.winds == nil || s.jobs == nil {
		http.Error(w, "rutines no disponibles", 500)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var req struct {
		Wind      string  `json:"wind"`
		Kind      string  `json:"kind"`
		Time      string  `json:"time"`
		IntervalH float64 `json:"interval_h"`
		Prompt    string  `json:"prompt"`
		Name      string  `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "petició il·legible", 400)
		return
	}
	wind, ok := windByRef(s, strings.TrimSpace(req.Wind))
	if !ok {
		http.Error(w, "vent inexistent", 400)
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		http.Error(w, "cal dir què ha de vigilar o fer", 400)
		return
	}
	if len([]rune(prompt)) > rutinaMaxPrompt {
		http.Error(w, "descripció massa llarga (màxim 2000)", 400)
		return
	}
	kind := req.Kind
	if kind == "" {
		kind = "daily"
	}
	timeStr := strings.TrimSpace(req.Time)
	name := strings.TrimSpace(req.Name)
	switch kind {
	case "daily":
		if _, err := time.Parse("15:04", timeStr); err != nil {
			http.Error(w, "hora amb format HH:MM", 400)
			return
		}
		if name == "" {
			name = "Rutina " + wind.Name
		}
	case "interval":
		if req.IntervalH <= 0 || req.IntervalH > 24*30 {
			http.Error(w, "interval entre 0 i 720 hores", 400)
			return
		}
		if name == "" {
			name = "Rutina " + wind.Name
		}
	default:
		http.Error(w, "kind ha de ser daily o interval", 400)
		return
	}
	// WindOwner: la rutina pertany a l'usuari que la crea (multitasca:
	// qui la veu i qui la corre), com fa handleJobSave.
	owner := ""
	if s.user != nil && !s.user.Tothom {
		owner = s.user.Name
	}
	id := fmt.Sprintf("rutina-%s-%d", wind.ID, time.Now().Unix())
	j, err := s.jobs.Save(jobs.Job{
		ID: id, Name: name, Prompt: prompt, Kind: kind,
		Time: timeStr, IntervalH: req.IntervalH,
		Enabled: true,
		// Notify:false — l'avís arriba com a novetat del vent al seu xat
		// (notificació local de l'app); per Telegram hi hauria un duplicat.
		Notify: false,
		Wind:   wind.ID, WindOwner: owner,
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"job": rutinaFromJob(s, j)})
}

// rutinaFromJob transforma un Job en la vista de l'app.
func rutinaFromJob(s *Server, j jobs.Job) RutinaInfo {
	nameOf := ""
	if s.winds != nil {
		if w, ok := s.winds.ByID(j.Wind); ok {
			nameOf = w.Name
		}
	}
	out := RutinaInfo{
		ID: j.ID, Wind: j.Wind, WindName: nameOf,
		Name: j.Name, Kind: j.Kind, Time: j.Time, IntervalH: j.IntervalH,
		Prompt: j.Prompt, Enabled: j.Enabled,
		LastAt: j.LastAt, LastStatus: j.LastStatus,
	}
	if nd := jobs.NextDue(j, time.Now()); !nd.IsZero() {
		out.NextDue = nd.Format("02-01 15:04")
		out.NextDueAt = nd.Format(time.RFC3339)
	}
	return out
}

// handleWindsRutines llista les rutines dels vents visibles per a l'usuari.
func (s *Server) handleWindsRutines(w http.ResponseWriter, r *http.Request) {
	if s.winds == nil || s.jobs == nil {
		writeJSON(w, map[string]any{"rutines": []any{}})
		return
	}
	list, err := s.jobs.List()
	if err != nil {
		http.Error(w, "could not read routines", 500)
		return
	}
	out := make([]RutinaInfo, 0, 4)
	for _, j := range list {
		// Només rutines de vent (kind daily/interval amb Wind), no cors ni
		// recordatoris: els dos primers tenen el seu lloc a Programats.
		if j.Wind == "" || (j.Kind != "daily" && j.Kind != "interval") {
			continue
		}
		if !s.canSeeWindRoutine(j) {
			continue
		}
		out = append(out, rutinaFromJob(s, j))
	}
	writeJSON(w, map[string]any{"rutines": out})
}

// handleWindsRutinaDelete esborra una rutina. POST amb {"id": ...}.
func (s *Server) handleWindsRutinaDelete(w http.ResponseWriter, r *http.Request) {
	if s.jobs == nil {
		http.Error(w, "rutines no disponibles", 500)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "id obligatori", 400)
		return
	}
	if !s.windRoutineVisibleID(req.ID) {
		http.Error(w, "unknown routine", 404)
		return
	}
	if err := s.jobs.Delete(req.ID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
