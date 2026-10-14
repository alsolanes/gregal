package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/events"
	"gregal/internal/tools"
)

func retroPayload(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// retroTraceEvents és un torn sintètic però amb la forma exacta del registre
// durable: crides i resultats d'eina, un delegat, un checkpoint i el final.
func retroTraceEvents(t *testing.T, sess string, run int) []events.Event {
	p := func(v any) json.RawMessage { return retroPayload(t, v) }
	return []events.Event{
		{SessionID: sess, RunID: run, Kind: "run_started", Text: "torn iniciat"},
		{SessionID: sess, RunID: run, Kind: "tool_call", Payload: p(map[string]string{"id": "c1", "name": "read", "args": `{"path":"main.go"}`})},
		{SessionID: sess, RunID: run, Kind: "tool_result", Payload: p(map[string]string{"id": "c1", "name": "read", "output": "package main"})},
		{SessionID: sess, RunID: run, Kind: "tool_call", Payload: p(map[string]string{"id": "c2", "name": "edit", "args": `{"path":"calc.go","old":"a","new":"b"}`})},
		{SessionID: sess, RunID: run, Kind: "tool_result", Payload: p(map[string]string{"id": "c2", "name": "edit", "output": "edit aplicat a calc.go"})},
		{SessionID: sess, RunID: run, Kind: "tool_call", Payload: p(map[string]string{"id": "c3", "name": "bash", "args": `{"command":"go test ./..."}`})},
		{SessionID: sess, RunID: run, Kind: "tool_result", Payload: p(map[string]string{"id": "c3", "name": "bash", "output": "ok  \tcalc\t0.01s"})},
		{SessionID: sess, RunID: run, Kind: "tool_call", Payload: p(map[string]string{"id": "c4", "name": "bash", "args": `{"command":"golint ./..."}`})},
		{SessionID: sess, RunID: run, Kind: "tool_result", Payload: p(map[string]string{"id": "c4", "name": "bash", "output": "ERROR: exit: exit status 1\nnot found"})},
		{SessionID: sess, RunID: run, Kind: "delegate", Payload: p(map[string]any{"id": "d1", "task": "implement the parser helper", "status": "working", "model": "m"})},
		{SessionID: sess, RunID: run, Kind: "delegate", Payload: p(map[string]any{"id": "d1", "status": "working", "tool": "write", "output": "escrit parser.go"})},
		{SessionID: sess, RunID: run, Kind: "delegate", Payload: p(map[string]any{"id": "d1", "status": "completed", "output": "parser helper written"})},
		{SessionID: sess, RunID: run, Kind: "autonomous_checkpoint", Payload: p(map[string]any{"number": 1, "checks": []map[string]any{{"command": "go vet ./...", "code": 1, "output": "vet: bad printf"}}, "review": "CAL REVISAR: printf"})},
		{SessionID: sess, RunID: run, Kind: "done", Text: "Fixed calc and added tests."},
		{SessionID: sess, RunID: run, Kind: "run_completed", Text: "torn completat"},
		// Un altre torn no ha de contaminar el paquet.
		{SessionID: sess, RunID: run - 1, Kind: "tool_call", Payload: p(map[string]string{"id": "x", "name": "bash", "args": `{"command":"rm -rf other"}`})},
	}
}

func TestRetroEvidencePack(t *testing.T) {
	diff := []tools.DiffFile{{Path: "calc.go", Status: "modified", Added: 2, Removed: 1, Hunks: []tools.DiffHunk{{Patch: "@@ -1 +1 @@\n-a\n+b\n"}}}}
	pack := buildRetroPack(retroTraceEvents(t, "s", 5), 5, diff, "")
	ids := map[string]bool{}
	for _, a := range pack.Agents {
		ids[a.ID] = true
	}
	if !ids["coordinator"] || !ids["builder"] || !ids["reviewer"] || len(pack.Agents) != 3 {
		t.Fatalf("agents=%+v", pack.Agents)
	}
	find := func(sub string) *retroEvidence {
		for i := range pack.Evidence {
			if strings.Contains(pack.Evidence[i].Title, sub) {
				return &pack.Evidence[i]
			}
		}
		return nil
	}
	if f := find("calc.go (+2 −1)"); f == nil || !strings.Contains(f.Detail, "+b") || f.Kind != "file" {
		t.Fatalf("file evidence with hunk missing: %+v", pack.Evidence)
	}
	if e := find("go test ./..."); e == nil || e.Kind != "test" || e.OK == nil || !*e.OK {
		t.Fatalf("passing test missing: %+v", e)
	}
	if e := find("golint"); e == nil || e.OK == nil || *e.OK {
		t.Fatalf("failed command must be marked failed: %+v", e)
	}
	if e := find("go vet"); e == nil || e.Agent != "reviewer" || *e.OK {
		t.Fatalf("checkpoint check: %+v", e)
	}
	if e := find("read ×1"); e == nil {
		t.Fatal("read aggregate missing")
	}
	if e := find("result handed back"); e == nil || e.Agent != "builder" {
		t.Fatalf("delegate summary: %+v", e)
	}
	if find("rm -rf") != nil {
		t.Fatal("events from another run leaked into the pack")
	}
	text := renderRetroEvidence(pack)
	if !strings.Contains(text, "e1 by coordinator") || !strings.Contains(text, "[FAILED]") {
		t.Fatalf("rendered evidence:\n%s", text)
	}
}

func TestRetroMentions(t *testing.T) {
	agents := []retroAgent{{ID: "coordinator", Name: "Coordinator"}, {ID: "builder", Name: "Builder"}, {ID: "builder-2", Name: "Builder 2"}, {ID: "reviewer", Name: "Reviewer"}}
	got := parseRetroMentions("@Builder why? cc @reviewer, @main and @builder-2. mail me at x@builder.com @nobody", agents)
	want := []string{"builder", "reviewer", "coordinator", "builder-2"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("mentions=%v want %v", got, want)
	}
	if len(parseRetroMentions("no mentions here", agents)) != 0 {
		t.Fatal("phantom mention")
	}
}

type retroFake struct {
	mu      sync.Mutex
	bodies  []string
	calls   atomic.Int32
	garbage bool
}

func (f *retroFake) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		f.calls.Add(1)
		f.mu.Lock()
		f.bodies = append(f.bodies, string(raw))
		f.mu.Unlock()
		var req struct {
			Stream   bool `json:"stream"`
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(raw, &req)
		sys := ""
		if len(req.Messages) > 0 {
			sys = req.Messages[0].Content
		}
		content := `{"replies":[{"agent":"coordinator","text":"Tests passed, see the receipt.","receipts":["e3"]}]}`
		switch {
		case f.garbage:
			content = "sorry, I can't produce JSON"
		case strings.Contains(sys, "opening stand-up"):
			content = "```json\n" + `{"messages":[{"agent":"coordinator","text":"I edited calc.go and go test passed.","receipts":["e1","e2","e999"]},{"agent":"builder","text":"I wrote the parser helper, fully tested!","receipts":["e6"]},{"agent":"reviewer","text":"go vet failed at the checkpoint.","receipts":["e8"]},{"agent":"ghost","text":"boo"}],"reactions":[{"agent":"reviewer","message":1,"emoji":"🤔"},{"agent":"builder","message":0,"emoji":"🎉"},{"agent":"coordinator","message":0,"emoji":"💣"}]}` + "\n```"
		case strings.Contains(sys, "reacted 🤔"):
			content = `{"replies":[{"agent":"builder","text":"touché — the evidence only shows I wrote parser.go, no tests.","receipts":["e6"],"concede":true,"fix_suggestion":"Add unit tests for the parser helper and run go test ./..."}]}`
		case strings.Contains(sys, "reacted 👎"):
			content = `{"replies":[{"agent":"builder","text":"Fair, noted.","preference_note":"Prefer adding tests together with new helpers"}]}`
		case strings.Contains(sys, "Only these agents may reply: coordinator."):
		case strings.Contains(sys, "#retro channel replying"):
			content = `{"replies":[{"agent":"builder","text":"touché, I have no test receipts.","receipts":[],"concede":false,"fix_suggestion":"Write parser tests"}]}`
		}
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			chunk, _ := json.Marshal(map[string]any{"choices": []map[string]any{{"delta": map[string]string{"content": "done"}}}})
			w.Write([]byte("data: " + string(chunk) + "\n\ndata: [DONE]\n\n"))
			return
		}
		out, _ := json.Marshal(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": content}}}})
		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	}
}

func retroTestServer(t *testing.T, fake *retroFake) *Server {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	provider := httptest.NewServer(fake.handler(t))
	t.Cleanup(provider.Close)
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL + "/v1"}}
	s.cwd = t.TempDir()
	s.Hub()
	for _, e := range retroTraceEvents(t, s.id, 5) {
		if _, err := s.Hub().eventStore.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func retroPost(t *testing.T, s *Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return rec
}

func retroChannelFrom(t *testing.T, rec *httptest.ResponseRecorder) *retroChannel {
	t.Helper()
	var out struct {
		Channel *retroChannel `json:"channel"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Channel == nil {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	return out.Channel
}

func TestRetroStandupGroundedAndPersisted(t *testing.T) {
	fake := &retroFake{}
	s := retroTestServer(t, fake)
	rec := retroPost(t, s, "/api/retro/start", `{}`)
	c := retroChannelFrom(t, rec)
	if c.RunID != 5 || c.Degraded || len(c.Messages) != 3 {
		t.Fatalf("channel=%+v", c)
	}
	if c.Messages[0].Author != "coordinator" || strings.Join(c.Messages[0].Receipts, ",") != "e1,e2" {
		t.Fatalf("unknown receipts must be dropped: %+v", c.Messages[0])
	}
	if !retroHas(c.Messages[1].Reactions, "🤔", "reviewer") || !retroHas(c.Messages[0].Reactions, "🎉", "builder") || len(c.Messages[0].Reactions) != 1 {
		t.Fatalf("agent reactions: %+v / %+v", c.Messages[0].Reactions, c.Messages[1].Reactions)
	}
	fake.mu.Lock()
	prompt := fake.bodies[0]
	fake.mu.Unlock()
	for _, want := range []string{"STRICT GROUNDING", "go test ./...", "e1 by coordinator", "calc.go", "implement the parser helper", "Always reply to the user in English"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("grounding prompt lacks %q", want)
		}
	}
	// Idempotent: a second start does not call the model again.
	before := fake.calls.Load()
	retroChannelFrom(t, retroPost(t, s, "/api/retro/start", `{"run_id":5}`))
	if fake.calls.Load() != before {
		t.Fatal("start must be idempotent")
	}
	// Live events with allowlisted payloads.
	got, _ := s.Hub().eventStore.AfterScope(0, 500, "", s.id)
	kinds := map[string]int{}
	for _, e := range got {
		if strings.HasPrefix(e.Kind, "retro_") {
			kinds[e.Kind]++
			if !strings.Contains(string(e.Payload), `"key":"run-5"`) {
				t.Fatalf("payload without key: %s", e.Payload)
			}
		}
	}
	if kinds["retro_message"] != 3 || kinds["retro_typing"] != 2 {
		t.Fatalf("retro events: %v", kinds)
	}
	// Persistence: a fresh server for the same conversation sees the channel.
	s2 := goalTestServer(t)
	s2.Hub()
	s2.id, s2.convoFile = s.id, s.convoFile
	if again := s2.retroGet("run-5"); again == nil || len(again.Messages) != 3 {
		t.Fatalf("channel not persisted: %+v", again)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/retro", nil))
	if !strings.Contains(w.Body.String(), `"key":"run-5"`) || retroChannelFrom(t, w).Key != "run-5" {
		t.Fatalf("GET: %s", w.Body.String())
	}
}

func TestRetroMessageMentionsAndConcede(t *testing.T) {
	fake := &retroFake{}
	s := retroTestServer(t, fake)
	retroChannelFrom(t, retroPost(t, s, "/api/retro/start", `{}`))
	rec := retroPost(t, s, "/api/retro/message", `{"text":"@builder did you really test it?"}`)
	var out struct {
		Message  retroMessage   `json:"message"`
		Replies  []retroMessage `json:"replies"`
		Mentions []string       `json:"mentions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Replies) != 1 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	r := out.Replies[0]
	if r.Author != "builder" || r.Thread != out.Message.ID || !r.Concede || r.FixSuggestion != "Write parser tests" {
		t.Fatalf("reply=%+v", r)
	}
	if strings.Join(out.Mentions, ",") != "builder" {
		t.Fatalf("mentions=%v", out.Mentions)
	}
	// Without mentions the coordinator answers.
	rec = retroPost(t, s, "/api/retro/message", `{"text":"so what changed?"}`)
	out.Replies = nil
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Replies) != 1 || out.Replies[0].Author != "coordinator" || out.Replies[0].FixSuggestion != "" || out.Replies[0].Concede {
		t.Fatalf("default responder: %s", rec.Body.String())
	}
}

func TestRetroReactionSemantics(t *testing.T) {
	fake := &retroFake{}
	s := retroTestServer(t, fake)
	c := retroChannelFrom(t, retroPost(t, s, "/api/retro/start", `{}`))
	builderMsg := c.Messages[1].ID
	// 👍 is only recorded.
	before := fake.calls.Load()
	rec := retroPost(t, s, "/api/retro/react", `{"message_id":"`+builderMsg+`","emoji":"👍","action":"add"}`)
	if rec.Code != 200 || fake.calls.Load() != before || !strings.Contains(rec.Body.String(), `"by":["user"]`) {
		t.Fatalf("👍: %d %s", rec.Code, rec.Body.String())
	}
	rec = retroPost(t, s, "/api/retro/react", `{"message_id":"`+builderMsg+`","emoji":"👍","action":"remove"}`)
	if strings.Contains(rec.Body.String(), `"emoji":"👍"`) {
		t.Fatalf("remove: %s", rec.Body.String())
	}
	// 🤔 → justification in the author's thread, here a concession with a fix.
	rec = retroPost(t, s, "/api/retro/react", `{"message_id":"`+builderMsg+`","emoji":"🤔","action":"add"}`)
	var out struct {
		Replies []retroMessage `json:"replies"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Replies) != 1 || out.Replies[0].Author != "builder" || out.Replies[0].Thread != builderMsg || out.Replies[0].FixSuggestion == "" {
		t.Fatalf("🤔: %s", rec.Body.String())
	}
	fixMsg := out.Replies[0].ID
	// 👎 → preference stored in the project team notes.
	rec = retroPost(t, s, "/api/retro/react", `{"message_id":"`+builderMsg+`","emoji":"👎","action":"add"}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Replies) != 1 || !strings.HasPrefix(out.Replies[0].Note, "Prefer") {
		t.Fatalf("👎: %s", rec.Body.String())
	}
	notes, _ := os.ReadFile(filepath.Join(s.cwd, ".gregal", "memory.md"))
	if !strings.Contains(string(notes), "[retro] Prefer adding tests together with new helpers") {
		t.Fatalf("memory.md=%q", notes)
	}
	// Unsupported emoji and unknown message are rejected.
	if rec := retroPost(t, s, "/api/retro/react", `{"message_id":"`+builderMsg+`","emoji":"💣"}`); rec.Code != 400 {
		t.Fatalf("emoji: %d", rec.Code)
	}
	if rec := retroPost(t, s, "/api/retro/react", `{"message_id":"m999","emoji":"👍"}`); rec.Code != 404 {
		t.Fatalf("unknown message: %d", rec.Code)
	}
	// Fix it: enqueues a normal turn through the v2 run queue.
	rec = retroPost(t, s, "/api/retro/fix", `{"message_id":"`+fixMsg+`"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("fix: %d %s", rec.Code, rec.Body.String())
	}
	var fix struct {
		Run struct {
			ID int64 `json:"id"`
		} `json:"run"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &fix)
	if fix.Run.ID == 0 {
		t.Fatalf("fix run: %s", rec.Body.String())
	}
	select {
	case <-s.Hub().Queue().Done(fix.Run.ID):
	case <-time.After(30 * time.Second):
		t.Fatal("fix run did not finish")
	}
	if ch := s.retroGet("run-5"); ch.message(fixMsg).FixRun != fix.Run.ID {
		t.Fatal("fix run not linked to the message")
	}
	if rec := retroPost(t, s, "/api/retro/fix", `{"message_id":"`+builderMsg+`"}`); rec.Code != 400 {
		t.Fatalf("fix without suggestion: %d", rec.Code)
	}
}

func TestRetroDegradesAfterRepairRetry(t *testing.T) {
	fake := &retroFake{garbage: true}
	s := retroTestServer(t, fake)
	c := retroChannelFrom(t, retroPost(t, s, "/api/retro/start", `{}`))
	// Els especialistes no parlen al resum automàtic: no tenen evidència pròpia.
	doers := 0
	for i := range c.Agents {
		if !retroIsSpecialist(&c.Agents[i]) {
			doers++
		}
	}
	if !c.Degraded || len(c.Messages) != doers || fake.calls.Load() != 2 {
		t.Fatalf("degraded=%v msgs=%d calls=%d", c.Degraded, len(c.Messages), fake.calls.Load())
	}
	for _, m := range c.Messages {
		if len(m.Receipts) == 0 {
			t.Fatalf("fallback message without receipts: %+v", m)
		}
	}
	fake.mu.Lock()
	repair := fake.bodies[1]
	fake.mu.Unlock()
	if !strings.Contains(repair, "not valid") {
		t.Fatal("repair retry must explain the validation error")
	}
}

func TestRetroWebhookAndValidation(t *testing.T) {
	got := make(chan string, 1)
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got <- string(raw)
	}))
	defer hook.Close()
	fake := &retroFake{}
	s := retroTestServer(t, fake)
	if err := os.WriteFile(s.cfgPath, []byte("retro:\n  webhook_url: "+hook.URL+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	retroChannelFrom(t, retroPost(t, s, "/api/retro/start", `{}`))
	select {
	case body := <-got:
		var msg map[string]string
		if json.Unmarshal([]byte(body), &msg) != nil || !strings.HasPrefix(msg["text"], "#retro · ") || !strings.Contains(msg["text"], "*Coordinator*") {
			t.Fatalf("webhook body=%s", body)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("webhook not called")
	}
	if rec := retroPost(t, s, "/api/retro/start", `{"run_id":77}`); rec.Code != 404 {
		t.Fatalf("unknown run: %d", rec.Code)
	}
	if rec := retroPost(t, s, "/api/retro/message", `{"text":""}`); rec.Code != 400 {
		t.Fatalf("empty message: %d", rec.Code)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/retro/start", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET start: %d", w.Code)
	}
}
