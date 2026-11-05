package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func countCrewAuditAnswer(messages []llm.Message, answer string) int {
	count := 0
	for _, message := range messages {
		if message.Role == "user" && message.Content == answer {
			count++
		}
	}
	return count
}

func installCrewAuditChatCounter(t *testing.T) *atomic.Int32 {
	t.Helper()
	old := petChatter
	var calls atomic.Int32
	petChatter = func(context.Context, *llm.Client, *config.Config, config.Provider, config.Role, []llm.Message) (string, error) {
		calls.Add(1)
		return "unexpected normal chat fallback", nil
	}
	t.Cleanup(func() { waitBackground(); petChatter = old })
	return &calls
}

func TestCrewResumePersistenceFailureRoutesFailClosed(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "sync", true: "sse"}[stream], func(t *testing.T) {
			s, id := rotllanaChatter(t, "unused")
			rot, ok := s.rotllanaByID(id)
			if !ok {
				t.Fatal("missing test rotllana")
			}
			if err := saveCrewState(s, id, &crewState{Task: "await answer", RunID: "audit-resume", Waiting: true, Started: time.Now()}); err != nil {
				t.Fatal(err)
			}
			job := &crewJob{path: crewJobPath(s, id)}
			if err := job.set("waiting"); err != nil {
				t.Fatal(err)
			}
			job.beforeReplace = func(step string) error {
				if step == "working" {
					return errors.New("injected job status persistence failure")
				}
				return nil
			}
			key := crewRegistryKey(s, id)
			crewJobs.Store(key, job)
			t.Cleanup(func() { crewJobs.Delete(key) })
			calls := installCrewAuditChatCounter(t)

			answer := "Build a Python app"
			body := `{"rotllana":"` + id + `","message":"` + answer + `"}`
			var rec *httptest.ResponseRecorder
			if stream {
				rec = postJSON(t, s.handleRotllanaChatStream, body)
			} else {
				rec = postJSON(t, s.handleRotllanaChat, body)
			}
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("resume persistence failure must return 500, got %d: %s", rec.Code, rec.Body.String())
			}
			if got := countCrewAuditAnswer(s.crewHistory(rot), answer); got != 1 {
				t.Fatalf("saved answer count = %d, want exactly one", got)
			}
			if got := calls.Load(); got != 0 {
				t.Fatalf("ordinary chat ran after failed crew resume (%d model calls)", got)
			}
		})
	}
}

func TestCrewUnreadableResumeStateRoutesFailClosed(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "sync", true: "sse"}[stream], func(t *testing.T) {
			s, id := rotllanaChatter(t, "unused")
			rot, ok := s.rotllanaByID(id)
			if !ok {
				t.Fatal("missing test rotllana")
			}
			statePath := crewStatePath(s, id)
			if err := os.MkdirAll(filepath.Dir(statePath), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(statePath, 0o700); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Remove(statePath) })
			calls := installCrewAuditChatCounter(t)

			answer := "Build a Python app"
			body := `{"rotllana":"` + id + `","message":"` + answer + `"}`
			var rec *httptest.ResponseRecorder
			if stream {
				rec = postJSON(t, s.handleRotllanaChatStream, body)
			} else {
				rec = postJSON(t, s.handleRotllanaChat, body)
			}
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("unreadable resume state must return 500, got %d: %s", rec.Code, rec.Body.String())
			}
			if got := countCrewAuditAnswer(s.crewHistory(rot), answer); got != 0 {
				t.Fatalf("answer was saved although resume state was unreadable: count=%d", got)
			}
			if got := calls.Load(); got != 0 {
				t.Fatalf("ordinary chat ran after unreadable resume state (%d model calls)", got)
			}
		})
	}
}

func TestCrewTerminalRecoveryDoesNotRepublishAfterLaterChat(t *testing.T) {
	s, id := rotllanaChatter(t, "unused")
	rot, ok := s.rotllanaByID(id)
	if !ok {
		t.Fatal("missing test rotllana")
	}
	members := s.rotMembersOf(rot)
	if len(members) == 0 {
		t.Fatal("missing lead wind")
	}
	st := &crewState{Task: "terminal task", RunID: "audit-terminal", Started: time.Now()}
	result := "terminal result to publish once"
	if err := s.publishCrewStateResult(rot, st, members[0], "done", result); err != nil {
		t.Fatal(err)
	}
	calls := installCrewAuditChatCounter(t)
	rec := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+id+`","message":"thanks"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("pending terminal result must block fallthrough, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("ordinary chat ran while terminal result was pending (%d calls)", got)
	}

	// Simulate a later durable history entry from before recovery. Terminal
	// deduplication must use its run identity, not require the result to remain
	// the last message in the transcript.
	hist := rotllanaHistory(s, id)
	hist.mu.Lock()
	next := append(append([]llm.Message(nil), hist.messages...),
		llm.Message{Role: "user", Content: "follow up after cleanup failure", Ts: time.Now().Unix()},
		llm.Message{Role: "assistant", Content: "ordinary reply", Ts: time.Now().Unix()})
	if err := saveRotllanaHistory(s, id, next); err != nil {
		hist.mu.Unlock()
		t.Fatal(err)
	}
	hist.messages = next
	hist.mu.Unlock()

	s.crewRecoverInterruptedSession(s)
	count := 0
	for _, message := range s.crewHistory(rot) {
		if message.Role == "assistant" && message.Kind == "result" && strings.Contains(message.Content, result) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("terminal result appeared %d times after recovery, want once", count)
	}
}

func TestCrewNewRunDoesNotInheritOldRunWriteList(t *testing.T) {
	s, id := rotllanaChatter(t, "unused")
	if err := saveCrewState(s, id, &crewState{Task: "old", RunID: "old-run", Files: []string{"old.txt"}}); err != nil {
		t.Fatal(err)
	}
	if err := saveCrewState(s, id, &crewState{Task: "new", RunID: "new-run", Files: []string{"new.txt"}}); err != nil {
		t.Fatal(err)
	}
	st, ok := loadCrewState(s, id)
	if !ok || st.RunID != "new-run" || len(st.Files) != 1 || st.Files[0] != "new.txt" {
		t.Fatalf("new run inherited stale prior state: %+v, ok=%v", st, ok)
	}
}
