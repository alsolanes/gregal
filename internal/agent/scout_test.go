package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func TestScoutHeuristic(t *testing.T) {
	cases := []struct {
		task string
		want bool
	}{
		{"hola", false},
		{"thanks, that works", false},
		{"ok fes-ho ara mateix", false},
		{"fix typo in README", false},
		{"rename foo to bar please", false},
		{"inspect", false},
		{"add a describe() function to src/stats.py with tests", true},
		{"why does the login fail?", true},
		{"look at internal/web/serve.go handler", true},
		{"make the page load faster and explain what you changed in the end", true},
		{"posa-ho tot bé", false},
	}
	for _, c := range cases {
		if got := scoutTaskNeedsResearch(c.task); got != c.want {
			t.Errorf("scoutTaskNeedsResearch(%q) = %v, want %v", c.task, got, c.want)
		}
	}
}

func TestScoutWantedModesAndConfig(t *testing.T) {
	cfg := &config.Config{}
	task := "add a describe() function to src/stats.py with tests"
	for mode, want := range map[string]bool{ModeCode: true, ModeAutonomous: true, ModeDebug: true, ModeChat: false, ModeInspect: false, ModeGoal: false} {
		if got := ScoutWanted(cfg, mode, task); got != want {
			t.Errorf("mode %s: got %v want %v", mode, got, want)
		}
	}
	cfg.Agent.Scout = "off"
	if ScoutWanted(cfg, ModeCode, task) {
		t.Fatal("agent.scout: off must disable the scout")
	}
	if ScoutWanted(nil, ModeCode, task) {
		t.Fatal("nil config must not scout")
	}
}

type scoutEvents struct {
	mu  sync.Mutex
	evs []DelegateEvent
}

func (s *scoutEvents) hook(e DelegateEvent) { s.mu.Lock(); s.evs = append(s.evs, e); s.mu.Unlock() }
func (s *scoutEvents) statuses() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, e := range s.evs {
		out = append(out, e.Status)
	}
	return out
}

func TestRunScoutEmitsChildEventsAndReturnsBrief(t *testing.T) {
	var gotPrompt string
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		gotPrompt = prompt
		if def := SubagentFromContext(ctx); def == nil || def.Mode != ModeInspect || def.MaxSteps != scoutMaxSteps {
			t.Errorf("scout must run as a read-only 8-step subagent, got %+v", def)
		}
		return "src/stats.py: mean() at line 3; tests in tests/test_stats.py", nil
	})
	rec := &scoutEvents{}
	ctx := WithDelegateHook(context.Background(), rec.hook)
	res := RunScout(ctx, ScoutOptions{ID: "scout-1", Task: "add describe() to src/stats.py", Workspace: t.TempDir(), Session: "s"})
	if res.Err != nil || !strings.Contains(res.Brief, "src/stats.py") {
		t.Fatalf("unexpected result: %+v", res)
	}
	if !strings.Contains(gotPrompt, "add describe() to src/stats.py") || !strings.Contains(gotPrompt, "Do not change anything") {
		t.Fatalf("scout prompt misses the task or the read-only rule: %q", gotPrompt)
	}
	if got := strings.Join(rec.statuses(), ","); got != "working,completed" {
		t.Fatalf("events = %s", got)
	}
	for _, e := range rec.evs {
		if e.ID != "scout-1" || e.Agent != ScoutAgentName {
			t.Fatalf("event without scout identity: %+v", e)
		}
	}
}

func TestRunScoutFailureAndCancellation(t *testing.T) {
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		return "", errors.New("model down")
	})
	rec := &scoutEvents{}
	res := RunScout(WithDelegateHook(context.Background(), rec.hook), ScoutOptions{Task: "investigate the bug in main.go"})
	if res.Err == nil || res.Brief != "" {
		t.Fatalf("a failed scout must return no brief and the error: %+v", res)
	}
	if got := rec.statuses(); got[len(got)-1] != DelegateFailed {
		t.Fatalf("last event must be failed, got %v", got)
	}

	started := make(chan struct{})
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})
	rec = &scoutEvents{}
	ctx, cancel := context.WithCancel(WithDelegateHook(context.Background(), rec.hook))
	done := make(chan ScoutResult)
	go func() { done <- RunScout(ctx, ScoutOptions{Task: "investigate the bug in main.go"}) }()
	<-started
	cancel()
	res = <-done
	if !errors.Is(res.Err, context.Canceled) {
		t.Fatalf("cancelled turn must cancel the scout: %+v", res)
	}
	if got := rec.statuses(); got[len(got)-1] != DelegateCancelled {
		t.Fatalf("last event must be cancelled, got %v", got)
	}
}

func TestRunScoutWithoutRunner(t *testing.T) {
	SetDelegateRunner(nil)
	if res := RunScout(context.Background(), ScoutOptions{Task: "x y z w"}); res.Err == nil {
		t.Fatal("no runner must be an error, not a panic")
	}
}

// Amb un model fals real (buildDelegateRunner): el scout llegeix un fitxer
// i torna el resum; els tokens van al comptador del torn.
func TestRunScoutWithFakeModel(t *testing.T) {
	var n atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if n.Add(1) == 1 {
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"r1","type":"function","function":{"name":"read","arguments":"{\"path\":\"stats.py\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"stats.py defines mean(); add describe() next to it"},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":7}}`)
	}))
	defer server.Close()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "stats.py"), []byte("def mean(xs):\n    return sum(xs)/len(xs)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := testCfg(server.URL)
	meter := &llm.UsageMeter{}
	ctx := WithDelegateRunner(llm.WithUsage(context.Background(), meter), cfg, llm.New())
	rec := &scoutEvents{}
	ctx = WithDelegateHook(ctx, rec.hook)
	res := RunScout(ctx, ScoutOptions{Task: "add a describe() function to stats.py", Workspace: ws, Session: "s"})
	if res.Err != nil || !strings.Contains(res.Brief, "describe()") {
		t.Fatalf("unexpected result: %+v", res)
	}
	sawRead := false
	for _, e := range rec.evs {
		if e.Tool == "read" {
			sawRead = true
		}
	}
	if !sawRead {
		t.Fatal("the scout's tool calls must reach the room as delegate events")
	}
	if u, calls, _ := meter.Total(); calls != 2 || u.PromptTokens != 30 {
		t.Fatalf("scout tokens must count in the turn meter: %+v calls=%d", u, calls)
	}
}

func TestInjectScoutBrief(t *testing.T) {
	hist := []llm.Message{{Role: "user", Content: "old"}, {Role: "assistant", Content: "a"}, {Role: "user", Content: "task\n"}}
	out := InjectScoutBrief(hist, "brief text")
	if hist[2].Content != "task\n" {
		t.Fatal("the original history must not change")
	}
	if !strings.HasPrefix(out[2].Content, "task\n\nRESEARCHER BRIEF") || !strings.HasSuffix(out[2].Content, "brief text") {
		t.Fatalf("brief not appended to the task: %q", out[2].Content)
	}
	if len(out) != 3 || out[0].Content != "old" {
		t.Fatal("only the last user message changes")
	}
	if got := InjectScoutBrief(hist, "  "); len(got) != 3 || got[2].Content != "task\n" {
		t.Fatal("an empty brief injects nothing")
	}
	if got := InjectScoutBrief(nil, "b"); len(got) != 1 || got[0].Role != "user" {
		t.Fatal("without a user message the brief becomes one")
	}
}
