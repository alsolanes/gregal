package agent

import (
	"context"
	"encoding/json"
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
	"gregal/internal/llm"
	"gregal/internal/tools"
)

func functionalCall(t *testing.T, id, name string, args any) llm.ToolCall {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	return llm.ToolCall{
		ID: id, Type: "function",
		Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: name, Arguments: string(raw)},
	}
}

func functionalReply(w http.ResponseWriter, content string, calls ...llm.ToolCall) {
	w.Header().Set("Content-Type", "application/json")
	message := map[string]any{"role": "assistant", "content": content}
	finish := "stop"
	if len(calls) > 0 {
		message["tool_calls"] = calls
		finish = "tool_calls"
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"choices": []any{map[string]any{"message": message, "finish_reason": finish}},
	})
}

func functionalConfig(url string) *config.Config {
	cfg := testCfg(url)
	role := cfg.Roles["code"]
	role.ContextWindow = 32768
	cfg.Roles["code"] = role
	cfg.Router.Mode = "off"
	return cfg
}

func functionalInputName(i int) string {
	return "input-" + string(rune('a'+i/26)) + string(rune('a'+i%26)) + ".txt"
}

func TestFunctionalCodeReadEditAndTestCycle(t *testing.T) {
	t.Setenv("GOWORK", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOSUMDB", "off")
	dir := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":         "module smoke\n\ngo 1.27.1\n",
		"answer.go":      "package smoke\n\nfunc answer() int { return 41 }\n",
		"answer_test.go": "package smoke\n\nimport \"testing\"\n\nfunc TestAnswer(t *testing.T) { if answer() != 42 { t.Fatalf(\"answer = %d\", answer()) } }\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		switch requests.Add(1) {
		case 1:
			functionalReply(w, "", functionalCall(t, "read-1", "read", map[string]any{"path": "answer.go"}))
		case 2:
			functionalReply(w, "", functionalCall(t, "edit-1", "edit", map[string]any{
				"path": "answer.go", "old_string": "return 41", "new_string": "return 42",
			}))
		case 3:
			functionalReply(w, "", functionalCall(t, "test-1", "bash", map[string]any{"command": "go test ./..."}))
		default:
			functionalReply(w, "implemented and verified")
		}
	}))
	defer server.Close()

	tools.TodoClear()
	t.Cleanup(tools.TodoClear)
	res, err := RunNonInteractiveExIn(context.Background(), llm.New(), functionalConfig(server.URL), "Fix answer and run its tests", ModeCode, 8, false, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Answer != "implemented and verified" {
		t.Fatalf("answer=%q", res.Answer)
	}
	if got := strings.Join(res.Tools, ","); got != "read,edit,bash" {
		t.Fatalf("tools=%s", got)
	}
	for _, call := range res.Trace {
		if call.Failed {
			t.Errorf("tool %s failed: %s", call.Tool, call.Summary)
		}
	}
	sawPassingGoTest := false
	for _, call := range res.Trace {
		if call.Tool == "bash" && strings.Contains(call.Summary, "ok smoke") {
			sawPassingGoTest = true
		}
	}
	if !sawPassingGoTest {
		t.Fatalf("run did not capture successful go test output; trace=%+v", res.Trace)
	}
	updated, err := os.ReadFile(filepath.Join(dir, "answer.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "return 42") {
		t.Fatalf("edit did not reach workspace: %s", updated)
	}
}

func TestFunctionalAutonomousRunStopsAtToolBudget(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		name := functionalInputName(i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("marker-"+name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var toolCalls atomic.Int32
	var nonToolCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var request struct {
			Tools []json.RawMessage `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if len(request.Tools) > 0 {
			n := toolCalls.Add(1)
			if n > 20 {
				t.Errorf("agent exceeded tool budget: call %d", n)
				functionalReply(w, "unexpected extra tool call")
				return
			}
			path := functionalInputName(int(n - 1))
			functionalReply(w, "", functionalCall(t, "read-"+functionalInputName(int(n)), "read", map[string]any{"path": path}))
			return
		}
		if nonToolCalls.Add(1) == 1 {
			functionalReply(w, "CONTINUA")
			return
		}
		functionalReply(w, "bounded autonomous summary")
	}))
	defer server.Close()

	cfg := functionalConfig(server.URL)
	cfg.Agent.Autonomous = config.AutonomousCfg{
		CheckpointEvery: 5,
		ReviewEvery:     50,
		MaxMinutes:      10,
		MaxToolSteps:    20,
	}
	tools.TodoClear()
	t.Cleanup(tools.TodoClear)
	res, err := RunNonInteractiveExIn(context.Background(), llm.New(), cfg, "Read all the inputs and summarize them", ModeAutonomous, 4, true, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := toolCalls.Load(); got != 20 {
		t.Fatalf("read calls=%d, want the configured 20-call budget (non-tool calls=%d, steps=%d, tools=%v, answer=%q, trace=%+v)", got, nonToolCalls.Load(), res.Steps, res.Tools, res.Answer, res.Trace)
	}
	if got := nonToolCalls.Load(); got != 2 {
		t.Fatalf("non-tool model calls=%d, want one continue check and one bounded summary", got)
	}
	for _, call := range res.Trace {
		if call.Failed {
			t.Errorf("bounded run failed %s: %s", call.Tool, call.Summary)
		}
	}
	if res.Answer != "bounded autonomous summary" {
		t.Fatalf("answer=%q", res.Answer)
	}
}

func TestFunctionalHeadlessCancelPropagatesToActiveDelegate(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	stopRunner := func() { releaseOnce.Do(func() { close(release) }) }
	SetDelegateRunner(func(ctx context.Context, _ string) (string, error) {
		started <- struct{}{}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-release:
			return "", context.Canceled
		}
	})
	t.Cleanup(func() {
		stopRunner()
		SetDelegateRunner(nil)
	})

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if requests.Add(1) == 1 {
			functionalReply(w, "", functionalCall(t, "delegate-1", "delegate", map[string]any{
				"tasks": []map[string]string{{"prompt": "wait for cancellation"}},
			}))
			return
		}
		functionalReply(w, "stopped")
	}))
	defer server.Close()

	cfg := functionalConfig(server.URL)
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	type result struct {
		runErr error
	}
	done := make(chan result, 1)
	go func() {
		_, err := RunNonInteractiveExIn(ctx, llm.New(), cfg, "Investigate in parallel", ModeCode, 8, false, dir, nil)
		done <- result{runErr: err}
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("delegate did not start")
	}
	cancel()

	select {
	case got := <-done:
		if got.runErr == nil {
			t.Fatal("canceled run returned successfully")
		}
	case <-time.After(time.Second):
		stopRunner()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("run did not unwind after releasing the test delegate")
		}
		t.Fatal("cancellation did not reach the active delegate promptly")
	}
}
