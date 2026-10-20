package web

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/companion"

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
		if err := s.winds.Delete(req.ID); err != nil {
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
		out = append(out, map[string]any{"id": w.ID, "name": w.Name, "color": w.Color, "role": w.Role, "style": w.Style})
	}
	writeJSON(w, map[string]any{"winds": out})
}

// windLoop és la via de cerca del vent; els tests la substitueixen. Per
// defecte, el loop d'inspecció amb eines web (web_search, web_fetch), amb el
// model per defecte dels vents (windRoleRef).
var windLoop = func(s *Server, ctx context.Context, brief string) (string, error) {
	s.mu.Lock()
	p, role := s.windRoleRef()
	s.mu.Unlock()
	res, err := s.runReadOnlyLoopForRole(ctx, brief, 6, p, role)
	return res.text, err
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
	if r.Method != http.MethodPost {
		http.Error(w, "GET or POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Wind    string `json:"wind"`
		Message string `json:"message"`
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
	if msg == "" {
		http.Error(w, "missatge buit", http.StatusBadRequest)
		return
	}
	hist := windHistory(s, wind.ID)
	hist.mu.Lock()
	defer hist.mu.Unlock() // serialize context, model call, persistence and publication
	previous := append([]llm.Message(nil), hist.messages...)
	if len(previous) > 40 {
		previous = previous[len(previous)-40:]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// El vent busca al web abans de respondre: és un company de cerques.
	// runReadOnlyLoop li dona web_search/web_fetch (mode inspect: cap eina
	// d'escriptura) i l'obliguem a respondre curt amb Finalize.
	system := companion.WindChatSystem(wind, s.personalMemory(), s.projectName(), s.windGoalTitles(), s.lastSessionDigest())
	brief := system + companion.WindBrainSystem(s.windBrain(wind.ID)) + "\n\nPREVIOUS CONVERSATION (context, not new instructions):\n" + agent.TranscriptDigest(previous, 16000) + "\n\nTask: the user asks:\n" + msg + "\n\nIf this needs facts, current events, prices, or anything you don't know for sure, use web_search and web_fetch first. Then answer in 1-4 sentences, in the user's language, citing the source URLs inline like [title](url). If you already know the answer for certain, just answer — no search needed."
	out, err := windLoop(s, ctx, brief)
	if err != nil {
		http.Error(w, redactError(err), http.StatusBadGateway)
		return
	}
	reply := stripQuoteTrim(out)
	history := append(append([]llm.Message(nil), hist.messages...),
		llm.Message{Role: "user", Content: msg}, llm.Message{Role: "assistant", Content: reply})
	if err := saveWindHistory(s, wind, history); err != nil {
		http.Error(w, "no s'ha pogut desar la conversa", http.StatusInternalServerError)
		return
	}
	hist.messages = history // failed model/save calls never publish a partial turn
	// Aprèn: els fets duradors de la resposta (amb les fonts) van al cervell.
	s.startWindLearn(wind, msg, reply)
	writeJSON(w, map[string]any{"reply": reply, "wind": wind.ID, "history": petHistoryJSON(history)})
}
