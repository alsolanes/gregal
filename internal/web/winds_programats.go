package web

import (
	"net/http"
	"strings"
	"time"
)

// La programació visible dels vents: què diran i quan (recordatoris), quin
// vent vigila què i quan tornarà a mirar (cors), i les estadístiques dels
// avisos proactius de cada vent: quants batecs ha fet, quants han acabat en
// missatge i quants en silenci. Tot surt de dades reals: els jobs i els runs
// desats, res d'inventat.

// ReminderInfo és un recordatori pendent d'un vent.
type ReminderInfo struct {
	ID       string    `json:"id"`
	Wind     string    `json:"wind"`
	WindName string    `json:"wind_name"`
	At       time.Time `json:"at"`
	Text     string    `json:"text"`
}

// HeartInfo és el cor d'un vent en marxa.
type HeartInfo struct {
	Wind       string     `json:"wind"`
	WindName   string     `json:"wind_name"`
	NextAt     *time.Time `json:"next_at,omitempty"`
	LastAt     *time.Time `json:"last_at,omitempty"`
	LastStatus string     `json:"last_status,omitempty"`
}

// WindStats són les estadístiques d'avisos proactius d'un vent: per quin
// canal ha parlat i quants cops ha decidit callar.
type WindStats struct {
	Wind      string     `json:"wind"`
	WindName  string     `json:"wind_name"`
	Beats     int        `json:"beats"`             // batecs del cor executats
	Spoke     int        `json:"spoke"`             // batecs que han acabat en missatge
	Silent    int        `json:"silent"`            // batecs en silenci (res que valgués la pena)
	Reminders int        `json:"reminders"`         // recordatoris disparats (missatges puntuals)
	LastAt    *time.Time `json:"last_at,omitempty"` // darrer avís de qualsevol mena
}

// handleWindsProgramats torna la programació i les estadístiques de tots els
// vents de la sessió. L'app la fa servir per a la vista «Programats».
func (s *Server) handleWindsProgramats(w http.ResponseWriter, r *http.Request) {
	if s.winds == nil {
		writeJSON(w, map[string]any{"reminders": []any{}, "hearts": []any{}, "stats": []any{}})
		return
	}
	winds, _ := s.winds.List()
	nameOf := func(id string) string {
		for _, w := range winds {
			if w.ID == id {
				return w.Name
			}
		}
		return ""
	}
	// Llistes buides, no null: els clients les recorren directament.
	reminders := []ReminderInfo{}
	hearts := []HeartInfo{}
	if s.jobs != nil {
		list, _ := s.jobs.List()
		for _, j := range list {
			if !j.Enabled {
				continue
			}
			switch {
			case strings.HasPrefix(j.ID, remindJobPrefix):
				id, ok := remindJobWind(j.ID)
				// Un avís d'un vent que ja no hi és no té a qui parlar: no es llista.
				if !ok || nameOf(id) == "" {
					continue
				}
				if j.NextAt == nil {
					continue
				}
				reminders = append(reminders, ReminderInfo{
					ID: j.ID, Wind: id, WindName: nameOf(id),
					At: *j.NextAt, Text: strings.TrimSpace(j.Prompt),
				})
			case j.Kind == "event" && strings.HasPrefix(j.ID, windJobPrefix):
				id, ok := windJobID(j.ID)
				if !ok || nameOf(id) == "" {
					continue
				}
				hearts = append(hearts, HeartInfo{
					Wind: id, WindName: nameOf(id),
					NextAt: j.NextAt, LastAt: j.LastAt, LastStatus: j.LastStatus,
				})
			}
		}
	}
	// Estadístiques des dels runs reals: cada run del cor (prefix wind-)
	// és un batec; output buit = silenci, amb text = parlat. Els runs remind
	// (prefix remind-) són avisos puntuals que sempre parlen.
	stats := map[string]*WindStats{}
	if s.jobs != nil {
		runs, _ := s.jobs.Runs("", 200)
		for _, run := range runs {
			wid := ""
			spoke := false
			if id, ok := windJobID(run.JobID); ok {
				wid = id
				spoke = strings.TrimSpace(run.Output) != ""
			} else if id, ok := remindJobWind(run.JobID); ok && run.Status == "ok" {
				wid = id
				spoke = true
			} else {
				continue
			}
			st := stats[wid]
			if st == nil {
				st = &WindStats{Wind: wid, WindName: nameOf(wid)}
				stats[wid] = st
			}
			if strings.HasPrefix(run.JobID, remindJobPrefix) {
				st.Reminders++
			} else {
				st.Beats++
				if spoke {
					st.Spoke++
				} else {
					st.Silent++
				}
			}
			at := run.FinishedAt
			if st.LastAt == nil || at.After(*st.LastAt) {
				st.LastAt = &at
			}
		}
	}
	// Tots els vents amb cor o recordatoris, ordenats per pròxim avís.
	out := make([]WindStats, 0, len(stats))
	for _, st := range stats {
		// Els runs d'un vent esborrat no tenen nom: fora de la vista.
		if st.WindName == "" {
			continue
		}
		out = append(out, *st)
	}
	writeJSON(w, map[string]any{
		"reminders": reminders,
		"hearts":    hearts,
		"stats":     out,
	})
}
