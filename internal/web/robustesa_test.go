package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// A single executed tool call must produce exactly one tool_result event.
// The engine's result events carry the call id now (headless trace), and the
// web driver, which already emits tool_result when it executes, painted it
// a second time (orphan duplicate card in the UI).
func TestUnaCridaExecutadaUnSolToolResult(t *testing.T) {
	var n atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if n.Add(1) == 1 {
			w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"call-1","type":"function","function":{"name":"read","arguments":"{\"path\":\"a.txt\"}"}}]},"finish_reason":"tool_calls"}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"read it"},"finish_reason":"stop"}]}`))
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL + "/v1"}}
	s.cwd = t.TempDir()
	if err := os.WriteFile(filepath.Join(s.cwd, "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleAgent(rec, httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(`{"task":"read a.txt","mode":"code"}`)))
	body := rec.Body.String()
	if got := strings.Count(body, "event: tool_result"); got != 1 {
		t.Fatalf("tool_result events = %d, want 1:\n%s", got, body)
	}
	if got := strings.Count(body, "event: tool_call"); got != 1 {
		t.Fatalf("tool_call events = %d, want 1:\n%s", got, body)
	}
}

// B8: an «ok» answering the agent's question goes to the model; it is not
// swallowed by the small-talk shortcut.
func TestOkAMitjaConversaVaAlModel(t *testing.T) {
	var hits atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"Applying the change now."},"finish_reason":"stop"}]}`))
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL + "/v1"}}
	s.cwd = t.TempDir()
	s.convo = []llm.Message{
		{Role: "user", Content: "add Total(), but ask me before editing"},
		{Role: "assistant", Content: "Here is the code. Shall I apply it?"},
	}
	rec := httptest.NewRecorder()
	s.handleAgent(rec, httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(`{"task":"ok","mode":"code"}`)))
	if hits.Load() == 0 {
		t.Fatalf("the confirmation never reached the model:\n%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "digues-m'hi") {
		t.Fatalf("small-talk reply instead of the agent:\n%s", rec.Body.String())
	}
}

// B8: the shortcut still works at the start, in the session language.
func TestSmallTalkWebEnLaLlenguaDeLaSessio(t *testing.T) {
	s := goalTestServer(t)
	s.cfg.Language = "en"
	rec := httptest.NewRecorder()
	s.handleAgent(rec, httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(`{"task":"ok","mode":"code"}`)))
	if !strings.Contains(rec.Body.String(), "Got it") {
		t.Fatalf("expected the English acknowledgement:\n%s", rec.Body.String())
	}
}

// Q3 (P2, append-only history): earlier turns are never rewritten — the
// conversation after turn k is a byte-identical prefix of the one after
// turn k+1, so a local server keeps its KV cache — and each new turn only
// carries what changed in the project since the previous map (a delta).
func TestNomesElMissatgeNouPortaMapa(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"done"},"finish_reason":"stop"}]}`))
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL + "/v1"}}
	s.cwd = t.TempDir()
	// Enough files that a delta is shorter than the full list.
	for i := 0; i < 20; i++ {
		if err := os.WriteFile(filepath.Join(s.cwd, fmt.Sprintf("seed-%02d.txt", i)), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var abans []byte
	var sysAbans string
	for i := 1; i <= 3; i++ {
		// The workspace changes between turns, so each turn has a new map.
		if err := os.WriteFile(filepath.Join(s.cwd, fmt.Sprintf("f%d.txt", i)), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		s.handleAgent(rec, httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(fmt.Sprintf(`{"task":"look at the project %d","mode":"code"}`, i))))
		ara, _ := json.Marshal(s.convo)
		if abans != nil && !strings.HasPrefix(string(ara), strings.TrimSuffix(string(abans), "]")) {
			t.Fatalf("turn %d rewrote earlier messages:\nbefore %s\nafter  %s", i, abans, ara)
		}
		abans = ara
		sys := s.sysPromptAmb(agent.ModeCode, s.cwd)
		if sysAbans != "" && sys != sysAbans {
			t.Fatalf("turn %d changed the system prompt", i)
		}
		sysAbans = sys
	}
	var users []llm.Message
	for _, m := range s.convo {
		if m.Role == "user" {
			users = append(users, m)
		}
	}
	if len(users) != 3 {
		raw, _ := json.Marshal(s.convo)
		t.Fatalf("expected 3 user messages: %s", raw)
	}
	if !strings.Contains(users[0].Content, agent.MarcaContext) || !strings.Contains(users[0].Content, "f1.txt") {
		t.Fatalf("the first message must carry the full map: %q", users[0].Content)
	}
	for i, m := range users[1:] {
		nou := fmt.Sprintf("f%d.txt", i+2)
		if !strings.Contains(m.Content, " delta] ") || !strings.Contains(m.Content, "Added:\n"+nou) || strings.Contains(m.Content, "seed-00.txt") {
			t.Fatalf("message %d must carry only the delta (%s added): %q", i+1, nou, m.Content)
		}
	}
}
