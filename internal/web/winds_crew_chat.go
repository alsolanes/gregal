package web

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/llm"
	"gregal/internal/team"
)

// Deliberately conservative: conversational questions and general requests must
// not unexpectedly start an autonomous code-writing pipeline.
var crewCodeAction = regexp.MustCompile(`(?i)(?:^|[^\pL])(implementa|implementar|implement|implementi|programa|programar|program|build|desenvolupa|desenvolupar|develop|crea|crear|create|creeu|afegeix|afegiu|añade|add|arregla|fix|modifica|modify|refactoritza|refactor|fes|feu|haz|haced|make)(?:$|[^\pL])`)
var crewCodeObject = regexp.MustCompile(`(?i)(?:^|[^\pL])(codi|código|codigo|code|software|app|aplicaci[oó]|aplicación|aplicacion|application|programa|program|python|kotlin|golang|website|web|p[aà]gina|page|joc|juego|game|projecte|proyecto|project|fitxer|archivo|file|script|api|servidor|server|bug|error|test|prova|funci[oó]|función|funcion|function|component|bot|repo|repositori|repository)(?:$|[^\pL])`)
var crewCodeNegation = regexp.MustCompile(`(?i)(?:^|[^\pL])(?:(?:no\s+(?:vull|volem|quiero|queremos)|(?:do\s+not|don't|dont)\s+ever)(?:$|[^\pL])|(?:no|not|never|don't|dont|evita|evitar)\s+(?:cre\pL*|crear|escri\pL*|escrib\pL*|write|writ\pL*|modifi\pL*|modify|modific\pL*|canvi\pL*|edit\pL*|implement\pL*|program\pL*|build|constru\pL*|gener\pL*|afegeix\pL*|afeg\pL*|añad\pL*|add|facis|feu|fes)(?:$|[^\pL]))`)
var crewQuestion = regexp.MustCompile(`(?i)^(com|cómo|como|how|què|que|qué|what|why|per què|por qué|when|when|which|explain|explica|describe|descriu|planifica|plan)(?:$|[^\pL])`)
var crewStartMu sync.Mutex

func crewChatBusy(id string) bool {
	if v, ok := crewJobs.Load(id); ok {
		switch v.(*crewJob).get() {
		case "discussing", "working", "waiting":
			return true
		}
	}
	return false
}

// crewBusyMessage és el text del 409 quan la colla ja treballa: la UI del
// mòbil mostra el text del servidor tal qual, així que surt en l'idioma del
// servidor, no en anglès.
func (s *Server) crewBusyMessage() string {
	if s.cfg.Lang() == "ca" {
		return "La colla ja està treballant en aquest fil; espera que acabi o atura-la."
	}
	return "The crew is already working in this thread; wait for it to finish or stop it."
}

func (s *Server) crewMayResume() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mode != agent.ModePlan
}

func (s *Server) crewChatAllowed(msg string) bool {
	s.mu.Lock()
	mode := s.mode
	s.mu.Unlock()
	m := strings.TrimLeft(strings.ToLower(strings.TrimSpace(msg)), "¿")
	if mode == agent.ModePlan || strings.HasPrefix(m, "?") || crewQuestion.MatchString(m) || crewCodeNegation.MatchString(m) || strings.HasPrefix(m, "can you explain") {
		return false
	}
	return crewCodeAction.MatchString(m) && crewCodeObject.MatchString(m)
}

// startCrewChat persists the user message once before launching. A single
// lock protects job reservation and the initial history append from duplicates.
func (s *Server) startCrewChat(rot Rotllana, msg string) (*crewJob, int, error) {
	crewStartMu.Lock()
	defer crewStartMu.Unlock()
	if v, ok := crewJobs.Load(rot.ID); ok {
		switch v.(*crewJob).get() {
		case "discussing", "working", "waiting":
			return nil, 0, errConst("crew already running")
		}
	}
	hist := rotllanaHistory(s, rot.ID)
	hist.mu.Lock()
	start := len(hist.messages)
	messages := append(append([]llm.Message(nil), hist.messages...), llm.Message{Role: "user", Content: msg, Ts: time.Now().Unix()})
	err := saveRotllanaHistory(s, rot.ID, messages)
	if err == nil {
		hist.messages = messages
	}
	hist.mu.Unlock()
	if err != nil {
		return nil, 0, err
	}
	job := &crewJob{path: crewJobPath(s, rot.ID)}
	job.set("discussing")
	crewJobs.Store(rot.ID, job)
	ctx, cancel := context.WithTimeout(context.Background(), teamRunTimeout+15*time.Minute)
	ctx, doneTurn := s.beginWindTurn("rotllana", rot.ID, ctx)
	goBackground(func() {
		defer cancel()
		defer doneTurn()
		ca := s.cfg.Lang() == "ca"
		s.runCrewContext(ctx, rot, msg, ca, job)
	})
	return job, start + 1, nil
}

// Publish only a real completed node's text, attributed to its actual wind.
// Review output is the deliverable; only Message contains reviewer feedback.
func (s *Server) crewPublishNode(rot Rotllana, flow team.FlowSpec, e team.Event) {
	if e.Type != "completed" {
		return
	}
	for _, node := range flow.Nodes {
		if node.ID != e.Agent || node.Wind == "" {
			continue
		}
		for _, wind := range s.rotMembersOf(rot) {
			if wind.ID != node.Wind {
				continue
			}
			text := strings.TrimSpace(e.Output)
			if node.Mode == "review" {
				text = strings.TrimSpace(e.Message)
				// An invalid verdict is not the reviewer's words: do not publish
				// it as a review message; crewWork reports the incomplete review.
				if e.ReviewInvalid {
					return
				}
				// Do not expose a structured verdict as a chat message.
				if firstJSONObject(text) != "" {
					return
				}
			}
			if text == "" {
				return
			}
			s.crewSayKind(rot, wind, "debate", oneLineKeepBreaks(text, 3000))
			return
		}
	}
}

// Serve actual saved crew messages, not synthesized speaker responses. The
// history mutex is held only while taking snapshots, never during the run.
func (s *Server) crewChat(w http.ResponseWriter, r *http.Request, rot Rotllana, msg string, stream bool) {
	job, next, err := s.startCrewChat(rot, msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	var emit func(string, any)
	if stream {
		var stop func()
		emit, stop = windSSE(w, r.Context(), 15*time.Second)
		defer stop()
		emit("phase", map[string]string{"phase": "thinking"})
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		step := job.get()
		history := s.crewHistory(rot)
		if emit != nil {
			for next < len(history) {
				m := history[next]
				next++
				if m.Role != "assistant" || m.SpeakerID == "" {
					continue
				}
				text := strings.TrimPrefix(m.Content, m.SpeakerName+": ")
				emit("phase", map[string]string{"phase": "speaker:" + m.SpeakerID})
				emit("token", map[string]string{"token": text})
			}
		}
		switch step {
		case "done", "failed", "cancelled", "waiting":
			history = s.crewHistory(rot)
			result := map[string]any{"reply": map[string]string{"wind": "", "text": ""}, "silenci": true, "history": s.rotllanaHistoryJSON(history)}
			if emit != nil {
				emit("done", result)
			} else {
				writeJSON(w, result)
			}
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
