package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/events"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// Rutes de subagents definits, preguntes al marge (/btw), resums de sessió
// (/recap) i memòria automàtica amb higiene. Viuen aquí per no engreixar
// serve.go: mux() només crida extraRoutes.

func (h *Hub) extraRoutes(handle func(string, http.HandlerFunc)) {
	handle("/api/agents/defined", h.route((*Server).handleAgentsDefined))
	handle("/api/btw", h.route((*Server).handleBtw))
	handle("/api/recap", h.route((*Server).handleRecap))
	handle("/api/memory/suggest", h.route((*Server).handleMemorySuggest))
	handle("/api/memory/tidy", h.route((*Server).handleMemoryTidy))
	handle("/api/memory/tidy/apply", h.route((*Server).handleMemoryTidyApply))
}

// ---- traça viva del torn (per a /btw) ----

// liveTraces guarda, per sessió, una còpia de l'historial del torn en
// curs. El conductor la refresca a cada ordre; /btw la llegeix sense tocar
// el torn (que és d'una altra goroutine).
var liveTraces sync.Map // *Server -> []llm.Message

func (s *Server) setLiveTrace(hist []llm.Message) {
	liveTraces.Store(s, append([]llm.Message(nil), hist...))
}

func (s *Server) clearLiveTrace() { liveTraces.Delete(s) }

func (s *Server) liveTrace() []llm.Message {
	if v, ok := liveTraces.Load(s); ok {
		return v.([]llm.Message)
	}
	return nil
}

// sideRoleRef és el rol de les crides al marge (chat, o el que hi hagi),
// amb el model triat a la sessió si n'hi ha.
func (s *Server) sideRoleRef() (config.Provider, config.Role, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name, _, ok := agent.SideRole(s.cfg)
	if !ok {
		return config.Provider{}, config.Role{}, false
	}
	p, r := s.roleRefFor(name)
	if _, known := s.cfg.Providers[r.Provider]; !known {
		return config.Provider{}, config.Role{}, false
	}
	return p, r, true
}

func decodeSmall(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return false
	}
	return true
}

// ---- subagents definits ----

func (s *Server) handleAgentsDefined(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	cwd := s.cwd
	s.mu.Unlock()
	defs := agent.LoadSubagents(cwd)
	if defs == nil {
		defs = []agent.SubagentDef{}
	}
	writeJSON(w, map[string]any{
		"agents":   defs,
		"dirs":     agent.DirsSubagents(cwd),
		"isolated": agent.IsolatedWorktree(cwd),
	})
}

// ---- /btw ----

// handleBtw respon una pregunta al marge amb el rol de xat sobre una
// còpia del transcript i de la traça viva. No espera el torn, no l'atura i
// no afegeix res a l'historial.
func (s *Server) handleBtw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Question string `json:"question"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" || len(req.Question) > 4000 {
		http.Error(w, "question required (max 4000 characters)", http.StatusBadRequest)
		return
	}
	p, role, ok := s.sideRoleRef()
	if !ok {
		http.Error(w, "configure an active model first", http.StatusBadRequest)
		return
	}
	hist := s.liveTrace()
	busy := hist != nil
	s.mu.Lock()
	if hist == nil {
		hist = append([]llm.Message(nil), s.convo...)
	}
	compacted := s.compacted
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	transcript := agent.TranscriptDigest(hist, 24000)
	if strings.TrimSpace(compacted) != "" {
		transcript = "EARLIER (compacted summary): " + compacted + "\n\n" + transcript
	}
	answer, err := agent.AnswerSide(r.Context(), client, cfg, p, role, req.Question, transcript)
	if err != nil {
		log.Printf("btw error: %s", redactError(err))
		http.Error(w, "side question failed", http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"answer": answer, "model": role.Model, "busy": busy})
}

// ---- /recap ----

// recapSkipKinds no aporten res a un resum (text en viu, senyals de vida).
var recapSkipKinds = map[string]bool{"token": true, "progress": true, "ping": true}

// recapEventWindow és quants events globals es miren cap enrere: el
// registre és de tot el procés i llegir-lo sencer a cada recap no escala.
const recapEventWindow = 5000

func (s *Server) recapEvents(since time.Time) ([]events.Event, error) {
	if s.hub == nil || s.hub.eventError() != nil {
		return nil, fmt.Errorf("event log unavailable")
	}
	scope := s.hub.keyFor(s.user, s.id)
	cursor := s.hub.eventStore.Cursor()
	after := uint64(0)
	if cursor > recapEventWindow {
		after = cursor - recapEventWindow
	}
	var out []events.Event
	for pages := 0; pages < recapEventWindow/500+1; pages++ {
		got, err := s.hub.eventStore.AfterScope(after, 500, s.usuari(), scope)
		if err != nil {
			return nil, err
		}
		if len(got) == 0 {
			break
		}
		for _, e := range got {
			if !e.At.Before(since) && !recapSkipKinds[e.Kind] {
				out = append(out, e)
			}
		}
		after = got[len(got)-1].ID
		if len(got) < 500 {
			break
		}
	}
	return out, nil
}

func recapEventDigest(evs []events.Event, maxChars int) string {
	var b strings.Builder
	for _, e := range evs {
		text := strings.Join(strings.Fields(e.Text), " ")
		if r := []rune(text); len(r) > 400 {
			text = string(r[:400]) + "…"
		}
		fmt.Fprintf(&b, "[%s] %s: %s\n", e.At.Local().Format("15:04"), e.Kind, text)
	}
	out := b.String()
	if r := []rune(out); len(r) > maxChars {
		out = "…(earlier activity omitted)\n" + string(r[len(r)-maxChars:])
	}
	return out
}

// handleRecap resumeix què ha passat a la sessió des d'un moment (registre
// durable d'events) o en les últimes N peticions (transcript). Amb
// conversation, el transcript és el d'una conversa desada.
func (s *Server) handleRecap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Since        string `json:"since"`
		Turns        int    `json:"turns"`
		Conversation string `json:"conversation"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	if req.Turns < 0 || req.Turns > 50 {
		http.Error(w, "turns must be between 0 and 50", http.StatusBadRequest)
		return
	}
	p, role, ok := s.sideRoleRef()
	if !ok {
		http.Error(w, "configure an active model first", http.StatusBadRequest)
		return
	}
	var activity, source string
	count := 0
	switch {
	case strings.TrimSpace(req.Conversation) != "":
		sess, err := session.Load(s.sessDir(), strings.TrimSpace(req.Conversation))
		if err != nil {
			http.Error(w, "conversation not found", http.StatusNotFound)
			return
		}
		part := agent.RecapTurns(sess.Convo, req.Turns)
		activity, source, count = agent.TranscriptDigest(part, 24000), "conversation", len(part)
	case strings.TrimSpace(req.Since) != "":
		since, err := time.Parse(time.RFC3339, strings.TrimSpace(req.Since))
		if err != nil {
			http.Error(w, "since must be RFC3339", http.StatusBadRequest)
			return
		}
		evs, err := s.recapEvents(since)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		activity, source, count = recapEventDigest(evs, 24000), "events", len(evs)
	default:
		hist := s.liveTrace()
		s.mu.Lock()
		if hist == nil {
			hist = append([]llm.Message(nil), s.convo...)
		}
		s.mu.Unlock()
		part := agent.RecapTurns(hist, req.Turns)
		activity, source, count = agent.TranscriptDigest(part, 24000), "turns", len(part)
	}
	if count == 0 || strings.TrimSpace(activity) == "" {
		writeJSON(w, map[string]any{"summary": "", "empty": true, "source": source, "count": 0})
		return
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	summary, err := agent.Recap(r.Context(), client, cfg, p, role, activity)
	if err != nil {
		log.Printf("recap error: %s", redactError(err))
		http.Error(w, "recap failed", http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"summary": summary, "empty": false, "source": source, "count": count, "model": role.Model})
}

// ---- memòria automàtica ----

// memSuggestion és la proposta de notes de l'últim torn de codi d'una
// sessió. state: pending (el model hi pensa), ready (hi ha notes), none.
type memSuggestion struct {
	State     string    `json:"state"`
	Notes     []string  `json:"notes"`
	Workspace string    `json:"-"`
	At        time.Time `json:"at"`
}

var memSuggestions sync.Map // *Server -> memSuggestion

// memorySuggester és la crida al model; els tests la substitueixen.
var memorySuggester = agent.SuggestMemory

func (s *Server) queueMemorySuggestion(mode, workspace string, execs int, cutShort bool, hist []llm.Message) {
	if !agent.ShouldSuggestMemory(mode, execs, cutShort, hist) || strings.TrimSpace(workspace) == "" {
		return
	}
	p, role, ok := s.sideRoleRef()
	if !ok {
		return
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	started := time.Now().UTC()
	memSuggestions.Store(s, memSuggestion{State: "pending", Notes: []string{}, Workspace: workspace, At: started})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		notes, err := memorySuggester(ctx, client, cfg, p, role, hist, config.Notes(workspace, ""))
		if err != nil {
			log.Printf("memory suggestion error: %s", redactError(err))
		}
		state := "ready"
		if len(notes) == 0 {
			state, notes = "none", []string{}
		}
		// Un torn més nou ja n'ha encuat una altra: aquesta és vella.
		if cur, ok := memSuggestions.Load(s); ok && cur.(memSuggestion).At.After(started) {
			return
		}
		memSuggestions.Store(s, memSuggestion{State: state, Notes: notes, Workspace: workspace, At: started})
	}()
}

// handleMemorySuggest: GET torna la proposta; POST {action, note} accepta
// (escriu a .gregal/memory.md) o descarta una nota de la proposta. Res no
// s'escriu sense aquest POST.
func (s *Server) handleMemorySuggest(w http.ResponseWriter, r *http.Request) {
	cur := memSuggestion{State: "none", Notes: []string{}}
	if v, ok := memSuggestions.Load(s); ok {
		cur = v.(memSuggestion)
	}
	if r.Method == http.MethodGet {
		writeJSON(w, cur)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "GET or POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Action string `json:"action"`
		Note   string `json:"note"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	if req.Action != "accept" && req.Action != "dismiss" {
		http.Error(w, "action must be accept or dismiss", http.StatusBadRequest)
		return
	}
	note := strings.TrimSpace(req.Note)
	idx := -1
	for i, n := range cur.Notes {
		if n == note {
			idx = i
		}
	}
	if req.Action == "dismiss" && note == "" {
		memSuggestions.Store(s, memSuggestion{State: "none", Notes: []string{}, At: cur.At})
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	if idx < 0 {
		http.Error(w, "note is not in the current suggestion", http.StatusNotFound)
		return
	}
	if req.Action == "accept" {
		if err := config.Note(cur.Workspace, note); err != nil && !config.IsDuplicateNote(err) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	rest := append(append([]string{}, cur.Notes[:idx]...), cur.Notes[idx+1:]...)
	state := cur.State
	if len(rest) == 0 {
		state = "none"
	}
	memSuggestions.Store(s, memSuggestion{State: state, Notes: rest, Workspace: cur.Workspace, At: cur.At})
	writeJSON(w, map[string]any{"ok": true, "saved": req.Action == "accept", "path": config.ProjectMemoryPath(cur.Workspace)})
}

// memoryTidier és la crida al model de la consolidació (substituïble).
var memoryTidier = agent.TidyMemory

// handleMemoryTidy: GET torna les notes actuals i l'empremta; POST demana
// la proposta consolidada i en torna el diff. No escriu res.
func (s *Server) handleMemoryTidy(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	cwd := s.cwd
	s.mu.Unlock()
	before := config.Notes(cwd, "")
	if before == nil {
		before = []string{}
	}
	hash := config.NotesHash(cwd)
	if r.Method == http.MethodGet {
		writeJSON(w, map[string]any{"notes": before, "hash": hash, "path": config.ProjectMemoryPath(cwd)})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "GET or POST required", http.StatusMethodNotAllowed)
		return
	}
	if len(before) == 0 {
		writeJSON(w, map[string]any{"before": before, "after": before, "diff": "", "hash": hash, "changed": false})
		return
	}
	p, role, ok := s.sideRoleRef()
	if !ok {
		http.Error(w, "configure an active model first", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	after, err := memoryTidier(r.Context(), client, cfg, p, role, before)
	if err != nil {
		log.Printf("memory tidy error: %s", redactError(err))
		http.Error(w, "memory tidy failed", http.StatusBadGateway)
		return
	}
	if after == nil {
		after = []string{}
	}
	diff := agent.NotesDiff(before, after)
	writeJSON(w, map[string]any{"before": before, "after": after, "diff": diff, "hash": hash, "changed": strings.Join(before, "\n") != strings.Join(after, "\n")})
}

// handleMemoryTidyApply escriu la llista que l'usuari ha vist al diff,
// només si el fitxer no ha canviat des de la proposta (hash).
func (s *Server) handleMemoryTidyApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		After []string `json:"after"`
		Hash  string   `json:"hash"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	s.mu.Lock()
	cwd := s.cwd
	s.mu.Unlock()
	if config.NotesHash(cwd) != req.Hash {
		http.Error(w, "notes changed since the proposal; tidy again", http.StatusConflict)
		return
	}
	before := config.Notes(cwd, "")
	if len(req.After) > len(before) {
		http.Error(w, "tidy can merge and drop notes, not add them", http.StatusBadRequest)
		return
	}
	if err := config.ReplaceNotes(cwd, req.After); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "count": len(req.After), "hash": config.NotesHash(cwd), "kept": strconv.Itoa(len(req.After)) + "/" + strconv.Itoa(len(before))})
}
