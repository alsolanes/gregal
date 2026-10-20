package web

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// Mode company (estil Dot): memòria personal + check-ins.
//
// La memòria és la d'usuari existent (config/memory.go: ~/.config/gregal/
// memory.md via GREGAL_MEMORY), que ja s'injecta a cada system prompt. Aquí
// només hi afegim la proposta de fets (com automem per al codi) i el check-in.

// companionSuggestion és la proposta de fets personals de l'últim torn de
// xat. state: pending (el model hi pensa), ready (hi ha fets), none.
type companionSuggestion struct {
	State string    `json:"state"`
	Facts []string  `json:"facts"`
	At    time.Time `json:"at"`
}

var companionSuggests sync.Map // *Server -> companionSuggestion

// companionExtractorVar és la crida de parse; els tests la substitueixen.
var companionExtractorVar = companion.ParseExtract

// queueCompanionSuggestion encola l'extracció de fets personals després
// d'un torn de xat amb substància. Mai no retarda el torn ni escriu sola:
// la UI ha d'acceptar cada fet amb POST /api/companion/memory.
func (s *Server) queueCompanionSuggestion(mode string, hist []llm.Message) {
	// Només torns de xat (el company no treballa en codi) amb contingut.
	if mode != agent.ModeChat && mode != "company" {
		return
	}
	if !companion.ShouldExtract(hist) {
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
	companionSuggests.Store(s, companionSuggestion{State: "pending", Facts: []string{}, At: started})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		digest := agent.TranscriptDigest(hist, 16000)
		msgs := companion.ExtractMessage(digest, strings.Join(s.personalNotes(), "\n"))
		out, err := agent.SideCallLateral(ctx, client, cfg, p, role, msgs)
		if err != nil {
			log.Printf("companion extract error: %s", redactError(err))
			return
		}
		facts := companionExtractorVar(out, s.personalNotes())
		state := "ready"
		if len(facts) == 0 {
			state, facts = "none", []string{}
		}
		// Un torn més nou ja n'ha encuat una altra: aquesta és vella.
		if cur, ok := companionSuggests.Load(s); ok && cur.(companionSuggestion).At.After(started) {
			return
		}
		companionSuggests.Store(s, companionSuggestion{State: state, Facts: facts, At: started})
	}()
}

// handleCompanionMemory gestiona la memòria personal.
//
//	GET                          → {state, facts} de la proposta + notes actuals
//	POST {action:accept, fact}   → desa el fat a la memòria d'usuari (config.Remember)
//	POST {action:dismiss}        → descarta la proposta
//	POST {action:forget, term}   → esborra notes que contenen el terme
func (s *Server) handleCompanionMemory(w http.ResponseWriter, r *http.Request) {
	cur := companionSuggestion{State: "none", Facts: []string{}}
	if v, ok := companionSuggests.Load(s); ok {
		cur = v.(companionSuggestion)
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, struct {
			State string   `json:"state"`
			Facts []string `json:"facts"`
			Notes []string `json:"notes"`
		}{State: cur.State, Facts: cur.Facts, Notes: s.personalNotes()})
		return
	case http.MethodPost:
	default:
		http.Error(w, "GET or POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Action string `json:"action"`
		Fact   string `json:"fact"`
		Term   string `json:"term"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	switch req.Action {
	case "dismiss":
		companionSuggests.Store(s, companionSuggestion{State: "none", Facts: []string{}, At: cur.At})
		writeJSON(w, map[string]any{"ok": true})
		return
	case "forget":
		n, err := s.forgetPersonal(strings.TrimSpace(req.Term))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "forgotten": n, "notes": s.personalNotes()})
		return
	case "accept":
		fact := strings.TrimSpace(req.Fact)
		idx := -1
		for i, f := range cur.Facts {
			if f == fact {
				idx = i
			}
		}
		if idx < 0 {
			http.Error(w, "fact is not in the current suggestion", http.StatusNotFound)
			return
		}
		if err := s.rememberPersonal(fact); err != nil {
			// Duplicat: no és error per a l'usuari, la memòria ja ho té.
			if strings.Contains(err.Error(), "duplicat") || strings.Contains(err.Error(), "apuntat") {
				consumeCompanionFact(s, fact)
				writeJSON(w, map[string]any{"ok": true, "total": len(s.personalNotes()), "notes": s.personalNotes(), "already": true})
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		consumeCompanionFact(s, fact)
		writeJSON(w, map[string]any{"ok": true, "total": len(s.personalNotes()), "notes": s.personalNotes()})
		return
	default:
		http.Error(w, "action must be accept, dismiss or forget", http.StatusBadRequest)
	}
}

// consumeCompanionFact treu un fat acceptat de la proposta pendents: un
// acceptat no es pot tornar a acceptar (la UI no l'ha de tornar a ensenyar).
func consumeCompanionFact(s *Server, fact string) {
	if v, ok := companionSuggests.Load(s); ok {
		cur := v.(companionSuggestion)
		var rest []string
		for _, f := range cur.Facts {
			if f != fact {
				rest = append(rest, f)
			}
		}
		if len(rest) == 0 {
			rest = []string{}
			cur.State = "none"
		}
		cur.Facts = rest
		companionSuggests.Store(s, cur)
	}
}

// handleCompanionCheckin genera un check-in a partir de la memòria i la
// darrera sessió. POST per disparar-lo manualment (la UI Android també el
// programa via jobs daily); GET torna el darrer generat.
var companionCheckiner = defaultCheckin

func defaultCheckin(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, memory, digest string) (string, error) {
	msgs := []llm.Message{
		{Role: "system", Content: companion.CheckinSystem()},
		{Role: "user", Content: companion.CheckinPrompt(memory, digest)},
	}
	out, err := agent.SideCallLateral(ctx, client, cfg, p, role, msgs)
	if err != nil {
		return "", err
	}
	return companion.TruncateCheckin(out), nil
}

// lastSessionDigest torna un digest curt de la conversa viva (la sessió
// d'aquest servidor: per al company, la seva sessió persistida). Sense
// res recent torna "" i el prompt del check-in ja ho tracta.
func (s *Server) lastSessionDigest() string {
	s.mu.Lock()
	convo := append([]llm.Message(nil), s.convo...)
	s.mu.Unlock()
	if len(convo) == 0 {
		return ""
	}
	return agent.TranscriptDigest(convo, 4000)
}

func (s *Server) handleCompanionCheckin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	p, role, ok := s.sideRoleRef()
	if !ok {
		http.Error(w, "no side role available", http.StatusServiceUnavailable)
		return
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	memory := s.personalMemory()
	digest := s.lastSessionDigest()
	out, err := companionCheckiner(ctx, client, cfg, p, role, memory, digest)
	if err != nil {
		http.Error(w, redactError(err), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"checkin": out, "at": time.Now().UTC().Format(time.RFC3339)})
}
