package web

import (
	"net/http"
	"sort"
)

// windThread és una conversa a soles amb un vent tal com surt a la llista de
// converses: amb el títol de la primera cosa que li vas demanar.
type windThread struct {
	Wind  string `json:"wind"`
	Name  string `json:"name"`
	Title string `json:"title"`
	At    string `json:"at"`
	Msgs  int    `json:"msgs"`
}

// maxWindThreads són les converses a soles que surten a la barra lateral.
const maxWindThreads = 12

// recentWindThreads ajunta les converses a soles de tots els vents, de la més
// recent a la més vella. Per defecte una conversa nova la porta un sol vent
// («Automàtic»): sense aquesta llista no es podia tornar a trobar.
func (s *Server) recentWindThreads() []windThread {
	winds, _ := s.winds.List()
	var out []windThread
	for _, w := range winds {
		for _, in := range s.windSessionList(w.ID) {
			if in.Msgs == 0 {
				continue
			}
			out = append(out, windThread{Wind: w.ID, Name: in.Name, Title: in.Title, At: in.At, Msgs: in.Msgs})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At > out[j].At })
	if len(out) > maxWindThreads {
		out = out[:maxWindThreads]
	}
	for i := range out {
		if first := threadTitle(s, out[i].Name); first != "" {
			out[i].Title = first
		}
	}
	return out
}

// threadTitle és el primer missatge de la persona, en una línia curta.
func threadTitle(s *Server, session string) string {
	h := windHistoryFor(s, session)
	h.mu.RLock()
	msgs := h.messages
	h.mu.RUnlock()
	return oneLine(firstUserLine(msgs), 75)
}

// handleWindThreads: GET /api/winds/threads → {threads:[{wind,name,title,at,msgs}]}
func (s *Server) handleWindThreads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]any{"threads": s.recentWindThreads()})
}
