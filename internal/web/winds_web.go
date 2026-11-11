package web

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/goal"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// Vents de Gregal: companions personals amb nom de vent, color, persona i
// xat amb context de projecte. La feina proactiva no la fan ells: es crea com
// un job del sistema (Feines), que ja dispara i notifica.
//
//	GET  /api/winds                → {winds:[{id,name,color,role}]}
//	GET  /api/winds/chat?wind=<id> → {wind,history:[{from,text}]}
//	POST /api/winds/chat           → {reply,wind,history}
//	POST /api/winds                → crea/actualitza un vent {name,color,role,persona}
//	DELETE /api/winds              → {id}

// windHistoryEntry conserva l'historial en memòria i el desa al magatzem
// durador després de cada resposta (mateix patró que les mascotes).
// gen compta els torns publicats: el commit d'un torn només desa si cap
// altre torn ha publicat entremig (evita que un torn lent trepitgi un de
// nou sense bloquejar el GET durant la crida al model).
type windHistoryEntry struct {
	deleted  bool
	name     string
	mu       sync.RWMutex
	loadOnce sync.Once
	messages []llm.Message
	gen      uint64
}

var windHistories sync.Map // conversationHistoryKey -> *windHistoryEntry

// Direct turns still share one cancellation and steering target per wind.
// Serializing them across threads prevents one client cancelling the other.
var windDirectTurns sync.Map

func windDirectTurn(s *Server, id string) *sync.Mutex {
	turn, _ := windDirectTurns.LoadOrStore(historyKey(s, "wind", id), &sync.Mutex{})
	return turn.(*sync.Mutex)
}

func windHistory(s *Server, windID string) *windHistoryEntry {
	return windHistoryFor(s, s.windSessionName(windID))
}

func windHistoryFor(s *Server, name string) *windHistoryEntry {
	h, _ := windHistories.LoadOrStore(historyKey(s, "wind", name), &windHistoryEntry{name: name})
	entry := h.(*windHistoryEntry)
	entry.loadOnce.Do(func() {
		if saved, err := session.Load(s.sessDir(), name); err == nil {
			entry.messages = append([]llm.Message(nil), saved.Convo...)
		}
	})
	return entry
}

func saveWindHistory(s *Server, w companion.Wind, history []llm.Message) error {
	return saveWindHistoryTo(s, w, s.windSessionName(w.ID), history)
}

func saveWindHistoryTo(s *Server, w companion.Wind, name string, history []llm.Message) error {
	title := "Company — " + w.Name
	if name != "company-"+w.ID {
		for _, msg := range history {
			if msg.Role == "user" && msg.Content != "" {
				title = oneLine(msg.Content, 70)
				break
			}
		}
	}
	_, err := session.SaveSession(s.sessDir(), session.Session{
		Name:  name,
		Title: title,
		Role:  "chat",
		Convo: append([]llm.Message(nil), podaHistorialVent(history)...),
	})
	return err
}

// commitWindTurn publishes only to the session and generation the turn read.
func (s *Server) commitWindTurn(wind companion.Wind, hist *windHistoryEntry, gen uint64, history []llm.Message) ([]llm.Message, error) {
	hist.mu.Lock()
	defer hist.mu.Unlock()
	if hist.gen != gen {
		return append([]llm.Message(nil), hist.messages...), nil
	}
	if err := saveWindHistoryTo(s, wind, hist.name, history); err != nil {
		return nil, err
	}
	hist.messages = podaHistorialVent(history)
	hist.gen++
	return history, nil
}

// clearWindHistory buida la conversa amb un vent (disc + memòria).
// El vent i el seu perfil es queden: només marxa el xat.
func (s *Server) clearWindHistory(wind companion.Wind, hist *windHistoryEntry) error {
	hist.mu.Lock()
	defer hist.mu.Unlock()
	if hist.deleted && hist.name != "company-"+wind.ID {
		return fmt.Errorf("session no longer exists")
	}
	if err := saveWindHistoryTo(s, wind, hist.name, nil); err != nil {
		return err
	}
	hist.messages = nil
	hist.gen++
	return nil
}

// windGoalTitles llegeix els títols dels objectius pendents del projecte.
func (s *Server) windGoalTitles() []string {
	list, err := goal.List(s.goalDir, s.projectName())
	if err != nil {
		return nil
	}
	var titles []string
	for _, g := range list {
		if strings.EqualFold(g.Status, "fet") {
			continue
		}
		titles = append(titles, g.Title)
		if len(titles) == 5 {
			break
		}
	}
	return titles
}

// handleWinds gestiona el catàleg de vents.
func (s *Server) handleWinds(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		windLibraryMu.Lock()
		defer windLibraryMu.Unlock()
		var req companion.Wind
		if !decodeSmall(w, r, &req) {
			return
		}
		var saved companion.Wind
		var err error
		if id, fresh := s.freshWindID(req); fresh {
			saved, err = s.winds.SaveNew(req, id)
		} else {
			saved, err = s.winds.Save(req)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"wind": saved})
		return
	}
	if r.Method == http.MethodDelete {
		var req struct {
			ID string `json:"id"`
		}
		if !decodeSmall(w, r, &req) || strings.TrimSpace(req.ID) == "" {
			http.Error(w, "id obligatori", http.StatusBadRequest)
			return
		}
		if err := s.archiveWind(req.ID); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "GET, POST or DELETE required", http.StatusMethodNotAllowed)
		return
	}
	// Gregal, l'assistent principal (B14), hi és sempre.
	s.ensureGregalWind()
	list, err := s.winds.List()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, w := range list {
		out = append(out, map[string]any{"id": w.ID, "name": w.Name, "color": w.Color, "role": w.Role, "style": w.Style, "squad": w.Squad, "specialty": w.Specialty, "kit": kitOf(w), "followups": followupsLabel(w)})
	}
	writeJSON(w, map[string]any{"winds": out})
}

// windLoop és la via de cerca del vent; els tests la substitueixen. Per
// defecte, el loop d'inspecció amb eines web (web_search, web_fetch), amb el
// model per defecte dels vents (windRoleRef). Torna l'exploració sencera:
// el text i si ha quedat parcial (pressupost exhaurit) perquè el client
// pugui oferir «continua».
var windLoop = func(s *Server, ctx context.Context, brief string, routed ...*routedRole) (exploracio, error) {
	p, role := s.windRoleRef()
	if len(routed) > 0 && routed[0] != nil {
		p, role = routed[0].p, routed[0].role
	}
	return s.runReadOnlyLoopForRole(ctx, brief, windTurnSteps, p, role)
}

// routedRole és el resultat del router per a un torn de vent: qui l'ha calculat
// ja n'ha fet log; el loop només necessita el proveïdor i el rol finals.
type routedRole struct {
	p    config.Provider
	role config.Role
}

// handleWindChat és el xat amb un vent: persona + context de projecte.
func (s *Server) handleWindChat(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		id := strings.TrimSpace(r.URL.Query().Get("wind"))
		wind, ok := s.winds.ByID(id)
		if !ok {
			http.Error(w, "unknown wind", http.StatusBadRequest)
			return
		}
		hist, ok := s.requestedWindHistory(w, wind.ID, r.URL.Query().Get("session"))
		if !ok {
			return
		}
		hist.mu.RLock()
		messages := append([]llm.Message(nil), hist.messages...)
		hist.mu.RUnlock()
		writeJSON(w, map[string]any{"wind": wind.ID, "session": hist.name, "history": petHistoryJSON(messages)})
		return
	}
	// DELETE buida la conversa amb el vent (el vent es queda).
	if r.Method == http.MethodDelete {
		var req struct {
			ID      string `json:"id"`
			Wind    string `json:"wind"`
			Session string `json:"session"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		id := strings.TrimSpace(req.Wind)
		if id == "" {
			id = strings.TrimSpace(req.ID)
		}
		wind, ok := s.winds.ByID(id)
		if !ok {
			http.Error(w, "unknown wind", http.StatusBadRequest)
			return
		}
		hist, ok := s.requestedWindHistory(w, id, firstNonEmpty(req.Session, r.URL.Query().Get("session")))
		if !ok {
			return
		}
		if err := s.clearWindHistory(wind, hist); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "GET, POST or DELETE required", http.StatusMethodNotAllowed)
		return
	}
	s.handleWindTurn(w, r, false)
}

// freshWindID: un vent nou amb el nom d'un vent arxivat no pot heretar-ne
// l'historial ni la memòria (abans, crear «Llevant» després d'arxivar-ne un
// altre feia que el nou recordés converses que mai havia tingut). Si l'id que
// li tocaria encara té rastre al disc, en tria un de lliure («llevant-2»).
func (s *Server) freshWindID(req companion.Wind) (string, bool) {
	if strings.TrimSpace(req.ID) != "" || strings.TrimSpace(req.Name) == "" {
		return "", false
	}
	base := companion.Slug(req.Name)
	if _, exists := s.winds.ByID(base); exists || !s.windHasResidue(base) {
		return "", false
	}
	for n := 2; n < 1000; n++ {
		id := fmt.Sprintf("%s-%d", base, n)
		if _, exists := s.winds.ByID(id); !exists && !s.windHasResidue(id) {
			return id, true
		}
	}
	return "", false
}

// windHasResidue diu si un id de vent encara té historial o estat al disc.
func (s *Server) windHasResidue(id string) bool {
	// Un vent arxivat amb aquest id (la biblioteca el pot restaurar amb el seu
	// historial): el nou no el pot trepitjar.
	if items, err := s.archivedWinds(); err == nil {
		for _, it := range items {
			if it.Wind.ID == id {
				return true
			}
		}
	}
	for _, sub := range []string{"wind-memory", "wind-knowledge", "wind-skills", "wind-documents", "wind-tasks"} {
		matches, _ := filepath.Glob(filepath.Join(s.windStateDir(), sub, id+"*"))
		for _, m := range matches {
			base := filepath.Base(m)
			if base == id || strings.HasPrefix(base, id+".") {
				return true
			}
		}
	}
	return false
}

// followupsLabel és el valor que veu el client ("on"/"off"): una cadena, perquè
// un client que torni a enviar el vent tal qual no trenqui el desat.
func followupsLabel(w companion.Wind) string {
	if w.FollowupsOn() {
		return "on"
	}
	return "off"
}
