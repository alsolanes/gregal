package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// fakeChatProvider respon sempre el mateix text i desa cada petició.
func fakeChatProvider(t *testing.T, answer string) (*httptest.Server, func() []map[string]any) {
	var mu sync.Mutex
	var reqs []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		reqs = append(reqs, body)
		mu.Unlock()
		raw, _ := json.Marshal(answer)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":` + string(raw) + `}}]}`))
	}))
	return srv, func() []map[string]any {
		mu.Lock()
		defer mu.Unlock()
		return append([]map[string]any(nil), reqs...)
	}
}

func useProvider(s *Server, url string) {
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: url + "/v1"}}
	for name, r := range s.cfg.Roles {
		r.Provider, r.ContextWindow = "p", 32768
		s.cfg.Roles[name] = r
	}
}

func toolNames(req map[string]any) []string {
	var out []string
	tools, _ := req["tools"].([]any)
	for _, t := range tools {
		if m, ok := t.(map[string]any); ok {
			if f, ok := m["function"].(map[string]any); ok {
				out = append(out, f["name"].(string))
			}
		}
	}
	return out
}

func systemOf(req map[string]any) string {
	msgs, _ := req["messages"].([]any)
	if len(msgs) == 0 {
		return ""
	}
	m, _ := msgs[0].(map[string]any)
	s, _ := m["content"].(string)
	return s
}

func TestTeamRunWithToolsGivesEachRoleItsTools(t *testing.T) {
	srv, reqs := fakeChatProvider(t, "result")
	defer srv.Close()
	s := goalTestServer(t)
	useProvider(s, srv.URL)
	s.cwd = t.TempDir()
	w := httptest.NewRecorder()
	s.handleTeamRun(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(`{"task":"report","tools":true}`)))
	if !strings.Contains(w.Body.String(), `"type":"done"`) {
		t.Fatalf("body=%s", w.Body.String())
	}
	byRole := map[string][]string{}
	for _, req := range reqs() {
		sys := systemOf(req)
		for _, role := range []string{"researcher", "reviewer", "builder"} {
			if strings.Contains(sys, "TEAM ROLE "+role) {
				byRole[role] = toolNames(req)
			}
		}
		if strings.Contains(sys, "You are the coordinator") && len(toolNames(req)) != 0 {
			t.Error("the coordinator stays tool-free")
		}
	}
	has := func(list []string, name string) bool {
		for _, n := range list {
			if n == name {
				return true
			}
		}
		return false
	}
	if !has(byRole["researcher"], "read") || has(byRole["researcher"], "write") || has(byRole["researcher"], "bash") {
		t.Fatalf("researcher tools=%v", byRole["researcher"])
	}
	if !has(byRole["reviewer"], "bash") || has(byRole["reviewer"], "write") {
		t.Fatalf("reviewer tools=%v", byRole["reviewer"])
	}
	if !has(byRole["builder"], "write") || has(byRole["builder"], "delegate") {
		t.Fatalf("builder tools=%v", byRole["builder"])
	}
}

func TestTeamFlowUnknownSubagentIs400(t *testing.T) {
	s := goalTestServer(t)
	s.cwd = t.TempDir()
	body := `{"task":"x","flow":{"nodes":[{"id":"a","role":"researcher","agent":"ghost"}],"edges":[],"output":"a"}}`
	w := httptest.NewRecorder()
	s.handleTeamRun(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(body)))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "ghost") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if s.flowRunning {
		t.Fatal("busy guard leaked")
	}
}

func TestAgentsDefinedEndpoint(t *testing.T) {
	s := goalTestServer(t)
	s.cwd = t.TempDir()
	dir := filepath.Join(s.cwd, ".gregal", "agents")
	os.MkdirAll(dir, 0o700)
	os.WriteFile(filepath.Join(dir, "scout.md"), []byte("---\nname: zz-web-scout\ndescription: Finds things\ntools: [read]\n---\nLook."), 0o600)
	w := httptest.NewRecorder()
	s.handleAgentsDefined(w, httptest.NewRequest("GET", "/api/agents/defined", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"name":"zz-web-scout"`) || !strings.Contains(w.Body.String(), `"isolated":false`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestBtwAnswersWithoutTouchingHistory(t *testing.T) {
	srv, reqs := fakeChatProvider(t, "It is editing main.go")
	defer srv.Close()
	s := goalTestServer(t)
	useProvider(s, srv.URL)
	s.convo = []llm.Message{{Role: "user", Content: "old request"}, {Role: "assistant", Content: "old answer"}}
	s.setLiveTrace(append(append([]llm.Message(nil), s.convo...), llm.Message{Role: "user", Content: "LIVE TASK"}))
	defer s.clearLiveTrace()
	s.agentBusy = true
	w := postJSON(t, s.handleBtw, `{"question":"what is it doing?"}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "editing main.go") || !strings.Contains(w.Body.String(), `"busy":true`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(s.convo) != 2 {
		t.Fatalf("btw must not touch the history: %d messages", len(s.convo))
	}
	got := reqs()
	if len(got) != 1 || len(toolNames(got[0])) != 0 {
		t.Fatalf("one tool-free call expected: %v", got)
	}
	raw, _ := json.Marshal(got[0]["messages"])
	if !strings.Contains(string(raw), "LIVE TASK") || !strings.Contains(string(raw), "what is it doing?") {
		t.Fatalf("live trace missing: %s", raw)
	}
	if w := postJSON(t, s.handleBtw, `{"question":"  "}`); w.Code != 400 {
		t.Fatalf("empty question status=%d", w.Code)
	}
}

func TestRecapTurnsAndValidation(t *testing.T) {
	srv, _ := fakeChatProvider(t, "- did things")
	defer srv.Close()
	s := goalTestServer(t)
	useProvider(s, srv.URL)
	if w := postJSON(t, s.handleRecap, `{}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"empty":true`) {
		t.Fatalf("empty session: %d %s", w.Code, w.Body.String())
	}
	s.convo = []llm.Message{{Role: "user", Content: "a"}, {Role: "assistant", Content: "b"}}
	if w := postJSON(t, s.handleRecap, `{"turns":2}`); w.Code != 200 || !strings.Contains(w.Body.String(), "did things") || !strings.Contains(w.Body.String(), `"source":"turns"`) {
		t.Fatalf("turns recap: %d %s", w.Code, w.Body.String())
	}
	for _, bad := range []string{`{"turns":99}`, `{"since":"yesterday"}`, `{"nope":1}`} {
		if w := postJSON(t, s.handleRecap, bad); w.Code != 400 {
			t.Fatalf("%s status=%d", bad, w.Code)
		}
	}
}

func TestMemorySuggestionNeverWritesWithoutAccept(t *testing.T) {
	s := goalTestServer(t)
	s.cwd = t.TempDir()
	old := memorySuggester
	memorySuggester = func(context.Context, *llm.Client, *config.Config, config.Provider, config.Role, []llm.Message, []string) ([]string, error) {
		return []string{"Run go vet before tests", "Keep i18n keys identical"}, nil
	}
	defer func() { memorySuggester = old }()
	hist := []llm.Message{{Role: "tool", Name: "write", Content: "ok"}}
	s.queueMemorySuggestion("code", s.cwd, 4, false, hist)
	var got memSuggestion
	for i := 0; i < 200; i++ {
		w := httptest.NewRecorder()
		s.handleMemorySuggest(w, httptest.NewRequest("GET", "/", nil))
		json.Unmarshal(w.Body.Bytes(), &got)
		if got.State == "ready" {
			break
		}
		<-timeAfter()
	}
	if got.State != "ready" || len(got.Notes) != 2 {
		t.Fatalf("suggestion=%+v", got)
	}
	if _, err := os.Stat(config.ProjectMemoryPath(s.cwd)); err == nil {
		t.Fatal("a suggestion must never write by itself")
	}
	if w := postJSON(t, s.handleMemorySuggest, `{"action":"accept","note":"invented"}`); w.Code != 404 {
		t.Fatalf("only suggested notes can be accepted: %d", w.Code)
	}
	if w := postJSON(t, s.handleMemorySuggest, `{"action":"accept","note":"Run go vet before tests"}`); w.Code != 200 {
		t.Fatalf("accept: %d %s", w.Code, w.Body.String())
	}
	if w := postJSON(t, s.handleMemorySuggest, `{"action":"dismiss","note":"Keep i18n keys identical"}`); w.Code != 200 {
		t.Fatalf("dismiss: %d", w.Code)
	}
	notes := config.Notes(s.cwd, "")
	if len(notes) != 1 || !strings.Contains(notes[0], "Run go vet before tests") {
		t.Fatalf("notes=%q", notes)
	}
	// Un torn sense edicions no proposa res.
	memSuggestions.Delete(s)
	s.queueMemorySuggestion("code", s.cwd, 4, false, nil)
	if _, ok := memSuggestions.Load(s); ok {
		t.Fatal("no applied edit, no suggestion")
	}
}

func TestMemoryTidyShowsDiffAndAppliesWithHash(t *testing.T) {
	s := goalTestServer(t)
	s.cwd = t.TempDir()
	srv, _ := fakeChatProvider(t, "x")
	defer srv.Close()
	useProvider(s, srv.URL)
	for _, n := range []string{"use gofmt", "use gofmt always", "old stale note"} {
		if err := config.Note(s.cwd, n); err != nil {
			t.Fatal(err)
		}
	}
	old := memoryTidier
	memoryTidier = func(_ context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, _ config.Role, lines []string) ([]string, error) {
		return []string{lines[1]}, nil
	}
	defer func() { memoryTidier = old }()
	w := postJSON(t, s.handleMemoryTidy, `{}`)
	var prop struct {
		After   []string `json:"after"`
		Diff    string   `json:"diff"`
		Hash    string   `json:"hash"`
		Changed bool     `json:"changed"`
	}
	json.Unmarshal(w.Body.Bytes(), &prop)
	if !prop.Changed || len(prop.After) != 1 || !strings.Contains(prop.Diff, "- ") || len(config.Notes(s.cwd, "")) != 3 {
		t.Fatalf("proposal must not write: %+v notes=%v", prop, config.Notes(s.cwd, ""))
	}
	body, _ := json.Marshal(map[string]any{"after": prop.After, "hash": "stale"})
	if w := postJSON(t, s.handleMemoryTidyApply, string(body)); w.Code != 409 {
		t.Fatalf("stale hash status=%d", w.Code)
	}
	body, _ = json.Marshal(map[string]any{"after": prop.After, "hash": prop.Hash})
	if w := postJSON(t, s.handleMemoryTidyApply, string(body)); w.Code != 200 {
		t.Fatalf("apply status=%d %s", w.Code, w.Body.String())
	}
	if notes := config.Notes(s.cwd, ""); len(notes) != 1 || !strings.Contains(notes[0], "use gofmt always") {
		t.Fatalf("notes=%q", notes)
	}
}

func timeAfter() <-chan time.Time { return time.After(10 * time.Millisecond) }
