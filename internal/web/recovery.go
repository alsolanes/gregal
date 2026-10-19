package web

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"gregal/internal/agent"
	"gregal/internal/events"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// persistRunInput makes the accepted task discoverable before scouts,
// providers, or tools can take time or change files. The model conversation
// is untouched; runTurn adds its enriched user message when preparation ends.
func (s *Server) persistRunInput(runID int64, task string, images []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// The queue can start the executor before the HTTP handler returns from
	// Submit. Once either path has checkpointed this run, the other must not
	// overwrite a newer transcript snapshot with the pre-run copy.
	if s.currentRunID == runID {
		return nil
	}
	name := s.convoFile
	if name == "" {
		name = session.NewName()
	}
	convo := append([]llm.Message(nil), s.convo...)
	convo = append(convo, llm.Message{Role: "user", Content: agent.SenseMapa(task), Images: append([]string(nil), images...)})
	path, err := session.SaveSession(s.sessDir(), session.Session{
		Name: name, SessionID: s.id, LastRunID: runID, Role: s.role,
		Pinned: s.pinned, Workspace: s.cwd, Compacted: s.compacted,
		ModelOverrides: cloneModelOverrides(s.modelOverride), Convo: convo,
	})
	if err == nil {
		s.convoFile = strings.TrimSuffix(filepath.Base(path), ".json")
		s.currentRunID = runID
	}
	return err
}

// recoverInterruptedSession folds the durable event log back into a saved
// conversation when its last persisted turn did not reach a transcript save.
// It never starts the task again: tools may have produced external side
// effects, so the recovered transcript marks an unfinished tool explicitly.
func (s *Server) recoverInterruptedSession(saved *session.Session) bool {
	if saved == nil || saved.SessionID == "" || saved.LastRunID <= 0 || s.hub == nil || s.hub.eventStore == nil {
		return false
	}
	scope := s.hub.keyFor(s.user, saved.SessionID)
	runEvents, err := s.hub.eventStore.RunEvents(int(saved.LastRunID), s.usuari(), scope)
	if err != nil {
		return false
	}
	if len(runEvents) == 0 {
		return false
	}
	status := ""
	for _, ev := range runEvents {
		switch ev.Kind {
		case "run_completed":
			status = "completed"
		case "run_failed":
			status = "failed"
		case "run_cancelled":
			status = "cancelled"
		}
	}
	if status == "" { // A live run must never be rewritten as a recovered one.
		return false
	}
	lastUser := -1
	for i := range saved.Convo {
		if saved.Convo[i].Role == "user" {
			lastUser = i
		}
	}
	for i := lastUser + 1; i < len(saved.Convo); i++ {
		if saved.Convo[i].Role == "assistant" && strings.TrimSpace(saved.Convo[i].Content) != "" && !esNotaDeTorn(saved.Convo[i]) {
			return false // The completed reply was already saved.
		}
	}

	task := ""
	for _, ev := range runEvents {
		if ev.Kind != "run_started" || len(ev.Payload) == 0 {
			continue
		}
		var p struct {
			Task string `json:"task"`
		}
		if json.Unmarshal(ev.Payload, &p) == nil {
			task = strings.TrimSpace(p.Task)
		}
		break
	}
	if lastUser < 0 && task != "" {
		saved.Convo = append(saved.Convo, llm.Message{Role: "user", Content: task})
	}
	for len(saved.Convo) > 0 && esNotaDeTorn(saved.Convo[len(saved.Convo)-1]) {
		saved.Convo = saved.Convo[:len(saved.Convo)-1]
	}
	messages := recoverRunMessages(runEvents, status)
	if len(messages) == 0 && status == "completed" {
		return false
	}
	saved.Convo = append(saved.Convo, messages...)
	saved.SessionID = scope
	_, err = session.SaveSession(s.sessDir(), *saved)
	return err == nil
}

func (s *Server) recoverLiveSessionRun(runID int64) {
	s.mu.Lock()
	name := s.convoFile
	current := s.currentRunID
	s.mu.Unlock()
	if name == "" || current != runID {
		return
	}
	saved, err := session.Load(s.sessDir(), name)
	if err != nil || saved.LastRunID != runID || !s.recoverInterruptedSession(&saved) {
		return
	}
	s.mu.Lock()
	if s.convoFile == name && s.currentRunID == runID {
		s.convo = agent.AmbResum(saved.Convo, saved.Compacted)
	}
	s.mu.Unlock()
}

func recoverRunMessages(runEvents []events.Event, status string) []llm.Message {
	var out []llm.Message
	var tokens strings.Builder
	flushTokens := func() {
		if text := strings.TrimSpace(tokens.String()); text != "" {
			out = append(out, llm.Message{Role: "assistant", Content: tokens.String()})
		}
		tokens.Reset()
	}
	payload := func(ev events.Event) map[string]any {
		var fields map[string]any
		_ = json.Unmarshal(ev.Payload, &fields)
		return fields
	}
	str := func(fields map[string]any, key string) string {
		v, _ := fields[key].(string)
		return v
	}
	pending := map[string]bool{}
	var finalText string
	for _, ev := range runEvents {
		fields := payload(ev)
		switch ev.Kind {
		case "token":
			tokens.WriteString(ev.Text)
		case "tool_call":
			flushTokens()
			call := llm.ToolCall{ID: str(fields, "id"), Type: "function"}
			call.Function.Name = str(fields, "name")
			call.Function.Arguments = str(fields, "args")
			out = append(out, llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{call}})
			if call.ID != "" {
				pending[call.ID] = true
			}
		case "tool_result":
			flushTokens()
			id := str(fields, "id")
			name := str(fields, "name")
			output := str(fields, "output")
			if id != "" {
				delete(pending, id)
			}
			out = append(out, llm.Message{Role: "tool", ToolCallID: id, Name: name, Content: output})
		case "text":
			text := strings.TrimSpace(ev.Text)
			partial := strings.TrimSpace(tokens.String())
			if text != "" {
				if partial != "" && (strings.HasPrefix(text, partial) || strings.HasPrefix(partial, text)) {
					tokens.Reset()
					out = append(out, llm.Message{Role: "assistant", Content: text})
				} else {
					flushTokens()
					if text != finalText {
						out = append(out, llm.Message{Role: "assistant", Content: text})
					}
				}
				finalText = text
			}
		case "done":
			reply := strings.TrimSpace(ev.Text)
			if reply != "" && reply != finalText {
				partial := strings.TrimSpace(tokens.String())
				if partial != "" && (strings.HasPrefix(reply, partial) || strings.HasPrefix(partial, reply)) {
					tokens.Reset()
				}
				flushTokens()
				out = append(out, llm.Message{Role: "assistant", Content: reply})
				finalText = reply
			}
		}
	}
	flushTokens()
	for id := range pending {
		out = append(out, llm.Message{Role: "tool", ToolCallID: id, Content: "Torn interromput abans de rebre el resultat; comprova l'estat del projecte abans de repetir aquesta eina."})
	}
	if status == "failed" {
		out = append(out, llm.Message{Role: "assistant", Content: turnFailedNote})
	} else if status == "cancelled" {
		out = append(out, llm.Message{Role: "assistant", Content: turnStoppedNote})
	}
	return out
}
