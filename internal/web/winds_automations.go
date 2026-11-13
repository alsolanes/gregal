package web

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/jobs"
)

// Automatitzacions dels vents en una sola llista: tot el que un vent farà pel
// seu compte (recordatoris, seguiments, vigilàncies del tema i rutines), amb
// el proper moment i les accions que admet. Abans n'hi havia quatre vistes que
// es trepitjaven; aquesta és la font de totes.
//
//	GET /api/winds/automations[?wind=<id>] → {items:[…], counts:{…}}
//
// Les accions fan servir les rutes de sempre (rutines/run, toggle, delete i
// runs), que ara accepten qualsevol automatització pròpia.

// AutomationItem és una cosa que un vent farà sol.
type AutomationItem struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"` // reminder | followup | task | watch | routine
	Wind       string     `json:"wind"`
	WindName   string     `json:"wind_name"`
	WindColor  string     `json:"wind_color,omitempty"`
	WindStyle  string     `json:"wind_style,omitempty"`
	Title      string     `json:"title"`
	Detail     string     `json:"detail,omitempty"`
	Repeat     string     `json:"repeat"` // once | daily | interval | watch
	Time       string     `json:"time,omitempty"`
	IntervalH  float64    `json:"interval_h,omitempty"`
	Enabled    bool       `json:"enabled"`
	NextAt     *time.Time `json:"next_at,omitempty"`
	LastAt     *time.Time `json:"last_at,omitempty"`
	LastStatus string     `json:"last_status,omitempty"`
	CanPause   bool       `json:"can_pause"`
	// Text sencer i editable (Detail i Title es retallen per a la llista):
	// el text d'un recordatori o la consigna d'una rutina.
	Text string `json:"text,omitempty"`
	// El que l'usuari ha ensenyat a una rutina i quants exemples té.
	Howto string `json:"howto,omitempty"`
	Refs  int    `json:"refs,omitempty"`
}

// AutomationCounts és el resum de dalt de la pantalla.
type AutomationCounts struct {
	Total  int `json:"total"`
	Today  int `json:"today"`
	Week   int `json:"week"`
	Paused int `json:"paused"`
}

// automationWind diu de quin vent és un job, si n'és d'algun.
func automationWind(j jobs.Job) string {
	if j.Wind != "" {
		return j.Wind
	}
	if id, ok := remindJobWind(j.ID); ok && strings.HasPrefix(j.ID, remindJobPrefix) {
		return id
	}
	if id, ok := windJobID(j.ID); ok && j.Kind == "event" {
		return id
	}
	return ""
}

// automationOf converteix un job en element de la llista; false si no és
// una automatització d'un vent d'aquest compte.
func (s *Server) automationOf(j jobs.Job, winds map[string]companion.Wind, allWinds bool, now time.Time) (AutomationItem, bool) {
	wid := automationWind(j)
	if wid == "" || (j.Wind != "" && !s.canSeeWindRoutine(j)) {
		return AutomationItem{}, false
	}
	w, ok := winds[wid]
	if !ok {
		// La rutina d'un vent arxivat continua sent seva: ha de poder-la veure
		// i esborrar. Sense Wind (el cor antic) no se'n pot saber l'amo.
		if j.Wind == "" || len(winds) == 0 || !allWinds {
			return AutomationItem{}, false
		}
		w = companion.Wind{ID: wid, Name: wid}
	}
	it := AutomationItem{
		ID: j.ID, Wind: w.ID, WindName: w.Name, WindColor: w.Color, WindStyle: w.Style,
		// «Responsabilitat · …» és el nom intern de la feina programada d'una
		// responsabilitat (vigilar un preu…): a la llista només es diu què fa.
		Title: strings.TrimPrefix(strings.TrimSpace(j.Name), "Responsabilitat · "), Enabled: j.Enabled,
		LastAt: j.LastAt, LastStatus: j.LastStatus,
	}
	switch {
	case strings.HasPrefix(j.ID, remindJobPrefix):
		// Un recordatori que ja ha sonat s'apaga sol: és història, no pendent.
		if !j.Enabled && j.LastAt != nil {
			return AutomationItem{}, false
		}
		it.Kind, it.Repeat = "reminder", "once"
		it.Text = strings.TrimSpace(j.Prompt)
		it.Title = firstNonEmpty(strings.TrimSpace(j.Prompt), strings.TrimSpace(j.Task))
		if strings.TrimSpace(j.Task) != "" {
			it.Kind = "task"
			it.Detail = strings.TrimSpace(j.Task)
		}
		if j.Followup {
			it.Kind = "followup"
		}
		it.NextAt = j.NextAt
	case strings.HasPrefix(j.ID, respJobPrefix):
		// Una responsabilitat (B6): vigilància incremental de fonts.
		it.Kind, it.Repeat, it.CanPause = "watch", "watch", true
		it.Detail = strings.TrimSpace(j.Prompt)
		if nd := jobs.NextDue(j, now); !nd.IsZero() {
			it.NextAt = &nd
		}
	case j.Kind == "event":
		it.Kind, it.Repeat, it.CanPause = "watch", "watch", true
		it.Title = strings.TrimSpace(w.Role)
		it.Detail = strings.TrimSpace(j.Prompt)
		it.NextAt = j.NextAt
	case j.Kind == "daily" || j.Kind == "interval":
		it.Kind, it.Repeat, it.CanPause = "routine", j.Kind, true
		it.Time, it.IntervalH = j.Time, j.IntervalH
		it.Detail = strings.TrimSpace(j.Prompt)
		it.Text = strings.TrimSpace(j.Prompt)
		it.Howto, it.Refs = strings.TrimSpace(j.Howto), len(j.Refs)
		if nd := jobs.NextDue(j, now); !nd.IsZero() {
			it.NextAt = &nd
		}
	default:
		return AutomationItem{}, false
	}
	if it.Title == "" {
		it.Title = w.Name
	}
	if len([]rune(it.Detail)) > 400 {
		it.Detail = string([]rune(it.Detail)[:400]) + "…"
	}
	return it, true
}

// sortAutomations: primer les actives pel proper moment (les que no en tenen,
// al final de les actives), després les pausades.
func sortAutomations(items []AutomationItem) {
	sort.SliceStable(items, func(a, b int) bool {
		x, y := items[a], items[b]
		if x.Enabled != y.Enabled {
			return x.Enabled
		}
		if (x.NextAt == nil) != (y.NextAt == nil) {
			return x.NextAt != nil
		}
		if x.NextAt != nil && !x.NextAt.Equal(*y.NextAt) {
			return x.NextAt.Before(*y.NextAt)
		}
		return x.WindName < y.WindName
	})
}

func countAutomations(items []AutomationItem, now time.Time) AutomationCounts {
	c := AutomationCounts{Total: len(items)}
	y, m, d := now.Date()
	endOfDay := time.Date(y, m, d, 23, 59, 59, 0, now.Location())
	week := now.Add(7 * 24 * time.Hour)
	for _, it := range items {
		if !it.Enabled {
			c.Paused++
			continue
		}
		if it.NextAt == nil {
			continue
		}
		if !it.NextAt.After(endOfDay) {
			c.Today++
		}
		if it.NextAt.Before(week) {
			c.Week++
		}
	}
	return c
}

func (s *Server) handleWindsAutomations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	items := []AutomationItem{}
	now := time.Now()
	if s.winds != nil && s.jobs != nil {
		list, err := s.winds.List()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		only := strings.TrimSpace(r.URL.Query().Get("wind"))
		byID := map[string]companion.Wind{}
		for _, wd := range list {
			if only == "" || wd.ID == only {
				byID[wd.ID] = wd
			}
		}
		all, err := s.jobs.List()
		if err != nil {
			http.Error(w, "could not read automations", 500)
			return
		}
		for _, j := range all {
			if it, ok := s.automationOf(j, byID, only == "", now); ok {
				items = append(items, it)
			}
		}
	}
	sortAutomations(items)
	writeJSON(w, map[string]any{"items": items, "counts": countAutomations(items, now)})
}
