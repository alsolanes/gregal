package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/events"
	"gregal/internal/llm"
	"gregal/internal/session"
)

func TestResumeRecoversInterruptedRunFromDurableEvents(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	h := s.Hub()
	const runID = 17
	saved := session.Session{
		Name: "important-task", SessionID: s.id, LastRunID: runID,
		Role: "code", Convo: []llm.Message{{Role: "user", Content: "important task"}},
	}
	if _, err := session.SaveSession(s.sessDir(), saved); err != nil {
		t.Fatal(err)
	}
	for _, ev := range []events.Event{
		{SessionID: s.id, RunID: runID, Kind: "run_started", Payload: json.RawMessage(`{"task":"important task"}`)},
		{SessionID: s.id, RunID: runID, Kind: "token", Text: "Partial visible result"},
		{SessionID: s.id, RunID: runID, Kind: "tool_call", Payload: json.RawMessage(`{"id":"call-1","name":"write","args":"{\"path\":\"result.txt\"}"}`)},
		{SessionID: s.id, RunID: runID, Kind: "tool_result", Payload: json.RawMessage(`{"id":"call-1","name":"write","output":"written result.txt"}`)},
		{SessionID: s.id, RunID: runID, Kind: "run_failed", Text: "server restarted"},
	} {
		if _, err := h.eventStore.Append(ev); err != nil {
			t.Fatal(err)
		}
	}
	request := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/resume", strings.NewReader(`{"name":"important-task"}`))
		w := httptest.NewRecorder()
		s.handleResume(w, r)
		return w
	}
	w := request()
	if w.Code != http.StatusOK {
		t.Fatalf("resume status %d: %s", w.Code, w.Body.String())
	}
	var response struct {
		Recovered  bool          `json:"recovered"`
		Transcript []llm.Message `json:"transcript"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Recovered {
		t.Fatalf("response did not report recovery: %s", w.Body.String())
	}
	if len(response.Transcript) < 4 {
		t.Fatalf("partial task history missing: %+v", response.Transcript)
	}
	if response.Transcript[1].Content != "Partial visible result" || response.Transcript[2].ToolCalls[0].Function.Name != "write" || response.Transcript[3].Content != "written result.txt" {
		t.Fatalf("unexpected recovered transcript: %+v", response.Transcript)
	}
	if again := request(); again.Code != http.StatusOK {
		t.Fatalf("second resume status %d: %s", again.Code, again.Body.String())
	}
	loaded, err := session.Load(s.sessDir(), "important-task")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Convo) != len(response.Transcript)+1 { // response omits the private interrupted-turn marker
		t.Fatalf("recovery was duplicated or not persisted: got %d messages, response had %d", len(loaded.Convo), len(response.Transcript))
	}
}

func TestPersistRunInputBeforeWork(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	s.Hub()
	if err := s.persistRunInput(23, "important task before model work", nil); err != nil {
		t.Fatal(err)
	}
	if s.convoFile == "" {
		t.Fatal("task was not assigned a durable conversation")
	}
	saved, err := session.Load(s.sessDir(), s.convoFile)
	if err != nil {
		t.Fatal(err)
	}
	if saved.LastRunID != 23 || saved.SessionID != s.id || len(saved.Convo) != 1 || saved.Convo[0].Content != "important task before model work" {
		t.Fatalf("task checkpoint not saved before work: %+v", saved)
	}
	s.mu.Lock()
	s.convo = []llm.Message{
		{Role: "user", Content: "important task before model work"},
		{Role: "assistant", Content: "new reply"},
	}
	s.mu.Unlock()
	s.autosave()
	if err := s.persistRunInput(23, "important task before model work", nil); err != nil {
		t.Fatal(err)
	}
	saved, err = session.Load(s.sessDir(), s.convoFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Convo) != 2 || saved.Convo[1].Content != "new reply" {
		t.Fatalf("duplicate checkpoint overwrote newer transcript: %+v", saved.Convo)
	}
}
