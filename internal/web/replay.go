package web

// Repetició (timelapse) d'un torn acabat a la sala d'agents. El servidor
// només llista torns i lliura els events durables del que es demana; la
// reconstrucció dels estats de la sala és pura i viu al client
// (app/replay-core.js), igual que el directe.

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"gregal/internal/events"
)

type replayRun struct {
	RunID     int       `json:"run_id"`
	State     string    `json:"state"` // completed | failed | cancelled | running
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	Events    int       `json:"events"`
	Tools     int       `json:"tools"`
	Agents    int       `json:"agents"`
}

// replayMaxEvents talla torns gegants: n'hi ha prou per a una repetició i
// la resposta no creix sense límit.
const replayMaxEvents = 4000

// replaySkip: el text en viu (token), la velocitat del model (speed) i el
// canal #retro no canvien la sala.
func replaySkip(kind string) bool {
	return kind == "token" || kind == "speed" || strings.HasPrefix(kind, "retro_")
}

// replayRuns resumeix els torns d'una conversa, del més recent al més antic.
func replayRuns(evs []events.Event) []replayRun {
	by := map[int]*replayRun{}
	agents := map[int]map[string]bool{}
	for _, e := range evs {
		if e.RunID <= 0 || replaySkip(e.Kind) {
			continue
		}
		r := by[e.RunID]
		if r == nil {
			r = &replayRun{RunID: e.RunID, State: "running", StartedAt: e.At}
			by[e.RunID] = r
			agents[e.RunID] = map[string]bool{}
		}
		r.Events++
		switch e.Kind {
		case "run_started":
			r.StartedAt = e.At
		case "tool_call":
			r.Tools++
		case "delegate":
			if p := retroArgs(e.Payload); p != nil {
				if id, _ := p["id"].(string); id != "" {
					agents[e.RunID][id] = true
				}
			}
		case "run_completed":
			r.State, r.EndedAt = "completed", e.At
		case "run_failed":
			r.State, r.EndedAt = "failed", e.At
		case "run_cancelled":
			r.State, r.EndedAt = "cancelled", e.At
		}
	}
	out := make([]replayRun, 0, len(by))
	for id, r := range by {
		r.Agents = len(agents[id])
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RunID > out[j].RunID })
	if len(out) > 50 {
		out = out[:50]
	}
	return out
}

// handleReplay: GET /api/replay (torns de la conversa) o
// GET /api/replay?run_id=N (events d'aquell torn, en ordre).
func (s *Server) handleReplay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "només GET", http.StatusMethodNotAllowed)
		return
	}
	if s.Hub().eventError() != nil {
		http.Error(w, "registre d'events no disponible", http.StatusServiceUnavailable)
		return
	}
	evs := s.retroSessionEvents()
	raw := strings.TrimSpace(r.URL.Query().Get("run_id"))
	if raw == "" {
		writeJSON(w, map[string]any{"runs": replayRuns(evs)})
		return
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		http.Error(w, "run_id invàlid", http.StatusBadRequest)
		return
	}
	out := make([]events.Event, 0, 256)
	truncated := false
	for _, e := range evs {
		if e.RunID != id || replaySkip(e.Kind) {
			continue
		}
		if len(out) == replayMaxEvents {
			truncated = true
			break
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		http.Error(w, "torn desconegut en aquesta conversa", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"run_id": id, "events": out, "truncated": truncated})
}
