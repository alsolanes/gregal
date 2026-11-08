package web

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"gregal/internal/llm"
	"gregal/internal/session"
)

// Sessions per vent: cada vent tenia UNA conversa (company-<id>) i la
// persona només podia buidar-la. Amb sessions, un mateix vent pot tenir
// fils separats (un per tema, un per projecte) i canviar-hi, com fa el
// xat general amb /api/sessions.
//
// Nomenclatura al disc: company-<vent>-s<unix> (la conversa històrica,
// sense sufix, també és una sessió: la primera de la llista).

// WindSessionInfo és una fila de la llista de sessions d'un vent.
type WindSessionInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Msgs    int    `json:"msgs"`
	When    string `json:"when"`
	At      string `json:"at"`
	Current bool   `json:"current,omitempty"`
}

var windSessionSuffix = regexp.MustCompile(`^-s\d+$`)

// isWindSessionOf diu si name és una sessió (o la històrica) del vent.
func isWindSessionOf(wind, name string) bool {
	prefix := "company-" + wind
	return name == prefix || (strings.HasPrefix(name, prefix+"-s") && windSessionSuffix.MatchString(strings.TrimPrefix(name, prefix)))
}

// windSessionName és la sessió activa del vent: la marcada al registre o,
// per defecte, la conversa històrica.
func (s *Server) windSessionName(windID string) string {
	s.mu.Lock()
	active := s.windActiveSession[windID]
	s.mu.Unlock()
	if active != "" {
		return active
	}
	return "company-" + windID
}

// windSessionList torna les sessions del vent: l'activa primer, la resta
// per data (la més recent a dalt).
func (s *Server) windSessionList(windID string) []WindSessionInfo {
	infos, _ := session.List(s.sessDir())
	active := s.windSessionName(windID)
	out := []WindSessionInfo{}
	for _, in := range infos {
		if !isWindSessionOf(windID, in.Name) {
			continue
		}
		title := in.Title
		if title == "" {
			title = in.Name
		}
		out = append(out, WindSessionInfo{
			Name:    in.Name,
			Title:   title,
			Msgs:    in.Msgs,
			When:    in.SavedAt.Format("02-01 15:04"),
			At:      in.SavedAt.Format(time.RFC3339),
			Current: in.Name == active,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Current != out[j].Current {
			return out[i].Current
		}
		return out[i].At > out[j].At
	})
	return out
}

// handleWindSessions gestiona les sessions d'un vent:
//
//	GET    /api/winds/sessions?wind=<id> → {wind, sessions:[…]}
//	POST   /api/winds/sessions   {wind} → crea una sessió nova i l'activa
//	DELETE /api/winds/sessions   {wind, name} → esborra una sessió (no l'activa)
func (s *Server) handleWindSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		wind := strings.TrimSpace(r.URL.Query().Get("wind"))
		if wind == "" {
			http.Error(w, "cal wind", 400)
			return
		}
		if _, ok := s.winds.ByID(wind); !ok {
			http.Error(w, "unknown wind", http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"wind": wind, "sessions": s.windSessionList(wind)})
		return
	}
	var req struct {
		Wind string `json:"wind"`
		Name string `json:"name"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	wind := strings.TrimSpace(req.Wind)
	if _, ok := s.winds.ByID(wind); !ok {
		http.Error(w, "unknown wind", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPost:
		// Nova sessió: nom amb marca de temps, activada al registre i
		// desada buida al disc perquè surti a la llista des del primer
		// moment. La conversa històrica queda i es pot reobrir.
		name := fmt.Sprintf("company-%s-s%d", wind, time.Now().Unix())
		wd, _ := s.winds.ByID(wind)
		title := "Company — " + wind
		if wd.ID != "" {
			title = "Company — " + wd.Name
		}
		if _, err := session.SaveSession(s.sessDir(), session.Session{Name: name, Title: title, Role: "chat"}); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		s.mu.Lock()
		s.windActiveSession[wind] = name
		s.mu.Unlock()
		// La còpia en memòria és de l'altra sessió: fora, perquè el primer
		// GET de la nova no carregui res (comença de zero).
		windHistories.Delete(historyKey(s, "wind", wind))
		writeJSON(w, map[string]any{"wind": wind, "name": name, "sessions": s.windSessionList(wind)})
	case http.MethodDelete:
		name := strings.TrimSpace(req.Name)
		if name == "" {
			http.Error(w, "cal name", 400)
			return
		}
		if !isWindSessionOf(wind, name) {
			http.Error(w, "not a session of this wind", http.StatusBadRequest)
			return
		}
		if s.windSessionName(wind) == name {
			http.Error(w, "cannot delete the active session", http.StatusConflict)
			return
		}
		if err := session.Delete(s.sessDir(), name); err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	default:
		http.Error(w, "GET, POST or DELETE required", http.StatusMethodNotAllowed)
	}
}

// handleWindSessionOpen activa una sessió concreta del vent i n'exporta
// l'historial perquè el client el pinti de cop.
func (s *Server) handleWindSessionOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Wind string `json:"wind"`
		Name string `json:"name"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	wind := strings.TrimSpace(req.Wind)
	name := strings.TrimSpace(req.Name)
	if _, ok := s.winds.ByID(wind); !ok {
		http.Error(w, "unknown wind", http.StatusBadRequest)
		return
	}
	if !isWindSessionOf(wind, name) {
		http.Error(w, "not a session of this wind", http.StatusBadRequest)
		return
	}
	if _, err := session.Load(s.sessDir(), name); err != nil {
		http.Error(w, "no such session", http.StatusNotFound)
		return
	}
	s.mu.Lock()
	s.windActiveSession[wind] = name
	s.mu.Unlock()
	// La còpia en memòria és de l'altra sessió: fora, perquè el proper GET
	// carregui la que ara és activa.
	windHistories.Delete(historyKey(s, "wind", wind))
	// L'historial de la sessió oberta, per pintar-lo sense una segona crida.
	hist := windHistory(s, wind)
	hist.mu.RLock()
	messages := append([]llm.Message(nil), hist.messages...)
	hist.mu.RUnlock()
	writeJSON(w, map[string]any{"wind": wind, "name": name, "history": petHistoryJSON(messages), "sessions": s.windSessionList(wind)})
}
