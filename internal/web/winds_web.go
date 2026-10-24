package web

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"gregal/internal/companion"

	"gregal/internal/agent"
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
type windHistoryEntry struct {
	mu       sync.Mutex
	loadOnce sync.Once
	messages []llm.Message
}

var windHistories sync.Map // conversationHistoryKey -> *windHistoryEntry

func windHistory(s *Server, windID string) *windHistoryEntry {
	h, _ := windHistories.LoadOrStore(historyKey(s, "wind", windID), &windHistoryEntry{})
	entry := h.(*windHistoryEntry)
	entry.loadOnce.Do(func() {
		if saved, err := session.Load(s.sessDir(), "company-"+windID); err == nil {
			entry.messages = append([]llm.Message(nil), saved.Convo...)
		}
	})
	return entry
}

func saveWindHistory(s *Server, w companion.Wind, history []llm.Message) error {
	_, err := session.SaveSession(s.sessDir(), session.Session{
		Name:  "company-" + w.ID,
		Title: "Company — " + w.Name,
		Role:  "chat",
		Convo: append([]llm.Message(nil), history...),
	})
	return err
}

// clearWindHistory buida la conversa amb un vent (disc + memòria).
// El vent i el seu perfil es queden: només marxa el xat.
func (s *Server) clearWindHistory(windID string) error {
	windHistories.Delete(historyKey(s, "wind", windID))
	if err := session.Delete(s.sessDir(), "company-"+windID); err != nil {
		// Sense historial previ no hi ha res a netejar: no és error.
		if !strings.Contains(err.Error(), "no trobada") {
			return err
		}
	}
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
		saved, err := s.winds.Save(req)
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
	list, err := s.winds.List()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, w := range list {
		out = append(out, map[string]any{"id": w.ID, "name": w.Name, "color": w.Color, "role": w.Role, "style": w.Style, "squad": w.Squad, "specialty": w.Specialty})
	}
	writeJSON(w, map[string]any{"winds": out})
}

// windLoop és la via de cerca del vent; els tests la substitueixen. Per
// defecte, el loop d'inspecció amb eines web (web_search, web_fetch), amb el
// model per defecte dels vents (windRoleRef). Torna l'exploració sencera:
// el text i si ha quedat parcial (pressupost exhaurit) perquè el client
// pugui oferir «continua».
var windLoop = func(s *Server, ctx context.Context, brief string) (exploracio, error) {
	s.mu.Lock()
	p, role := s.windRoleRef()
	s.mu.Unlock()
	return s.runReadOnlyLoopForRole(ctx, brief, 6, p, role)
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
		hist := windHistory(s, wind.ID)
		hist.mu.Lock()
		messages := append([]llm.Message(nil), hist.messages...)
		hist.mu.Unlock()
		writeJSON(w, map[string]any{"wind": wind.ID, "history": petHistoryJSON(messages)})
		return
	}
	// DELETE buida la conversa amb el vent (el vent es queda).
	if r.Method == http.MethodDelete {
		var req struct {
			ID   string `json:"id"`
			Wind string `json:"wind"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		id := strings.TrimSpace(req.Wind)
		if id == "" {
			id = strings.TrimSpace(req.ID)
		}
		if _, ok := s.winds.ByID(id); !ok {
			http.Error(w, "unknown wind", http.StatusBadRequest)
			return
		}
		if err := s.clearWindHistory(id); err != nil {
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
	var req struct {
		Wind     string `json:"wind"`
		Message  string `json:"message"`
		Continue bool   `json:"continue"` // reprèn l'última resposta parcial
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	wind, ok := s.winds.ByID(strings.TrimSpace(req.Wind))
	if !ok {
		http.Error(w, "unknown wind", http.StatusBadRequest)
		return
	}
	msg := strings.TrimSpace(req.Message)
	hist := windHistory(s, wind.ID)
	hist.mu.Lock()
	defer hist.mu.Unlock() // serialize context, model call, persistence and publication
	previous := append([]llm.Message(nil), hist.messages...)
	if len(previous) > 40 {
		previous = previous[len(previous)-40:]
	}
	// «Continua» sense historial o sense text nou no té què reprendre.
	if msg == "" && !(req.Continue && len(previous) > 0) {
		http.Error(w, "missatge buit", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// El vent busca al web abans de respondre: és un company de cerques.
	// runReadOnlyLoop li dona web_search/web_fetch (mode inspect: cap eina
	// d'escriptura) i l'obliguem a respondre curt amb Finalize.
	turnStart := time.Now()
	isContinue := req.Continue && len(previous) > 0
	var res exploracio
	var err error
	if isContinue {
		// Reprèn: sense nou missatge d'usuari, el loop afegeix el
		// PromptContinuaExploracio sobre l'historial (amb la parcial).
		s.mu.Lock()
		p, role := s.windRoleRef()
		s.mu.Unlock()
		res, err = s.runReadOnlyLoopForRole(ctx, "", 6, p, role, previous)
	} else {
		res, err = windLoop(s, agent.WithSkillsDirs(ctx, s.windSkillsDir(wind.ID)), windChatBrief(s, wind, previous, msg))
	}
	if err != nil {
		http.Error(w, redactError(err), http.StatusBadGateway)
		return
	}
	reply := stripQuoteTrim(res.text)
	// Un torn mort a mitges no és una resposta del vent (vegeu el mateix
	// criteri a winds_stream.go): mai el resum mecànic a la bombolla.
	if res.parcial && esRespostaMecanica(reply) {
		reply = fraseTornAturat(res.motiu, s.cfg.Lang())
	}
	history := append([]llm.Message(nil), hist.messages...)
	if !isContinue {
		history = append(history, llm.Message{Role: "user", Content: msg, Ts: time.Now().Unix()})
	}
	history = append(history, llm.Message{Role: "assistant", Content: reply, Ts: time.Now().Unix()})
	if err := saveWindHistory(s, wind, history); err != nil {
		http.Error(w, "no s'ha pogut desar la conversa", http.StatusInternalServerError)
		return
	}
	hist.messages = history // failed model/save calls never publish a partial turn
	// Aprèn: els fets duradors de la resposta (amb les fonts) van al cervell.
	s.startWindLearn(wind, msg, reply)
	s.startWindMemoryLearn(wind, msg, reply)
	writeJSON(w, map[string]any{"reply": reply, "wind": wind.ID, "parcial": res.parcial, "history": petHistoryJSON(history), "actions": remindActionsSince(wind.ID, turnStart)})
}
