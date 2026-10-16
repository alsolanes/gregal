package agent

import (
	"context"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/config"
)

func TestScoutFocuses(t *testing.T) {
	cases := []struct {
		name, task string
		want       []string
	}{
		{"inline numbered", "Review three areas: (1) internal/team, (2) the web room in workshop.js and (3) delegate.go. One paragraph each.",
			[]string{"internal/team", "the web room in workshop.js", "delegate.go"}},
		{"numbered lines", "Please check:\n1. the login flow\n2. the billing page\n",
			[]string{"the login flow", "the billing page"}},
		{"bullets", "Investigate\n- flaky tests in api/\n- slow build\n- docs links",
			[]string{"flaky tests in api/", "slow build", "docs links"}},
		{"extra items merge into the last area", "(1) a1 (2) b2 (3) c3 (4) d4",
			[]string{"a1", "b2", "c3; d4"}},
		{"paths in different folders", "Why does internal/web/app/workshop.js disagree with internal/agent/delegate.go?",
			[]string{"internal/web/app/workshop.js", "internal/agent/delegate.go"}},
		{"one folder is one area", "Fix internal/agent/scout.go and internal/agent/delegate.go", nil},
		{"urls are not areas", "Read https://example.com/a/b and fix main.go", nil},
		{"plain task", "Add a describe() function to the stats module", nil},
		{"list inside a question", "Without changing anything: how does each surface (the TUI, the web server and the Telegram bot) enforce the step budget?",
			[]string{"the TUI", "the web server", "the Telegram bot"}},
		{"catalan list", "Compara com gestionen els errors el client, el servidor i el bot.",
			[]string{"el client", "el servidor", "el bot"}},
		{"steps of one job are not areas", "Fix the bug, run the tests and commit", nil},
		{"two items are too common", "Explain how read and write differ", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ScoutFocuses(c.task); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("ScoutFocuses(%q)\n got %q\nwant %q", c.task, got, c.want)
			}
		})
	}
}

func TestParallelSubagentsLimits(t *testing.T) {
	remote := &config.Config{
		Providers: map[string]config.Provider{"zen": {BaseURL: "https://opencode.ai/zen/go/v1"}},
		Roles:     map[string]config.Role{"code": {Provider: "zen", Model: "m"}},
	}
	if got := ParallelSubagents(nil); got != MaxDelegateTasks {
		t.Fatalf("nil config = %d", got)
	}
	if got := ParallelSubagents(remote); got != MaxDelegateTasks {
		t.Fatalf("remote provider = %d, want %d", got, MaxDelegateTasks)
	}
	local := &config.Config{
		Providers: map[string]config.Provider{"llama": {BaseURL: "http://localhost:8080/v1"}},
		Roles:     map[string]config.Role{"code": {Provider: "llama", Model: "m"}},
	}
	if got := ParallelSubagents(local); got != 1 {
		t.Fatalf("a one-slot local server must run subagents one at a time, got %d", got)
	}
	ollama := &config.Config{
		Providers: map[string]config.Provider{"o": {Type: "ollama", BaseURL: "http://gpu-box:11434/v1"}},
		Roles:     map[string]config.Role{"code": {Provider: "o", Model: "m"}},
	}
	if got := ParallelSubagents(ollama); got != 1 {
		t.Fatalf("ollama = %d", got)
	}
	remote.Agent.ParallelSubagents = 2
	if got := ParallelSubagents(remote); got != 2 {
		t.Fatalf("explicit limit = %d", got)
	}
	remote.Agent.ParallelSubagents = 99
	if got := ParallelSubagents(remote); got != MaxDelegateLiveTotal {
		t.Fatalf("explicit limit is capped by the process limit, got %d", got)
	}
}

func TestRunScoutsInvestigatesAreasInParallel(t *testing.T) {
	var live, peak, calls atomic.Int32
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		calls.Add(1)
		n := live.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(150 * time.Millisecond)
		live.Add(-1)
		area := prompt[strings.Index(prompt, "YOUR AREA: ")+len("YOUR AREA: "):]
		return "brief for " + area[:strings.Index(area, "\n")], nil
	})
	rec := &scoutEvents{}
	task := "Risk review of (1) internal/team, (2) internal/web/app/workshop.js and (3) internal/agent/delegate.go"
	res := RunScouts(WithDelegateHook(context.Background(), rec.hook), ScoutOptions{ID: "first", Task: task, Workspace: t.TempDir(), Session: "s"})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if calls.Load() != 3 || peak.Load() != 3 {
		t.Fatalf("want 3 scouts at once, got %d calls with peak %d", calls.Load(), peak.Load())
	}
	if len(res.IDs) != 3 || res.ID != "first" || res.IDs[0] != "first" {
		t.Fatalf("ids = %v (first %q)", res.IDs, res.ID)
	}
	for i, area := range []string{"internal/team", "internal/web/app/workshop.js", "internal/agent/delegate.go"} {
		head := "AREA " + string(rune('1'+i)) + ": " + area + "\nbrief for " + area
		if !strings.Contains(res.Brief, head) {
			t.Fatalf("brief misses %q:\n%s", head, res.Brief)
		}
	}
	// The room labels each child with its area, not the boilerplate prompt.
	tasks := map[string]bool{}
	for _, e := range rec.evs {
		tasks[e.Task] = true
		if strings.Contains(e.Task, "Investigate") {
			t.Fatalf("child label is the prompt: %q", e.Task)
		}
	}
	if !tasks["internal/team"] || !tasks["internal/agent/delegate.go"] {
		t.Fatalf("labels = %v", tasks)
	}
}

func TestRunScoutsKeepsOneScoutOnLocalModels(t *testing.T) {
	var calls atomic.Int32
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		calls.Add(1)
		if strings.Contains(prompt, "YOUR AREA") {
			t.Errorf("a local model gets the whole task, not one area")
		}
		return "brief", nil
	})
	cfg := &config.Config{
		Providers: map[string]config.Provider{"llama": {BaseURL: "http://127.0.0.1:8080/v1"}},
		Roles:     map[string]config.Role{"code": {Provider: "llama", Model: "m"}},
	}
	res := RunScouts(context.Background(), ScoutOptions{Cfg: cfg, Task: "(1) internal/team (2) internal/web", Workspace: t.TempDir()})
	if res.Err != nil || res.Brief != "brief" || calls.Load() != 1 || len(res.IDs) != 1 {
		t.Fatalf("res=%+v calls=%d", res, calls.Load())
	}
}

func TestRunScoutsReportsAreasThatFailed(t *testing.T) {
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		if strings.Contains(prompt, "YOUR AREA: beta") {
			return "", context.DeadlineExceeded
		}
		return "alpha facts", nil
	})
	res := RunScouts(context.Background(), ScoutOptions{Task: "(1) alpha (2) beta", Workspace: t.TempDir()})
	if res.Err != nil || !strings.Contains(res.Brief, "AREA 1: alpha\nalpha facts") || !strings.Contains(res.Brief, "AREA 2: beta\n(no brief") {
		t.Fatalf("res = %+v", res)
	}
}

// With a cap of one, every child still shows up at once: the ones that
// wait say "queued" instead of being invisible until a slot frees.
func TestDelegateQueuesChildrenOverTheCap(t *testing.T) {
	old := delegateParallelDefault.Load()
	delegateParallelDefault.Store(1)
	t.Cleanup(func() { delegateParallelDefault.Store(old) })
	var live, peak atomic.Int32
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		n := live.Add(1)
		if n > peak.Load() {
			peak.Store(n)
		}
		time.Sleep(80 * time.Millisecond)
		live.Add(-1)
		return "ok " + prompt, nil
	})
	rec := &scoutEvents{}
	out, err := delegateExec(WithDelegateHook(context.Background(), rec.hook), `{"tasks":[{"prompt":"a"},{"prompt":"b"},{"prompt":"c"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if peak.Load() != 1 {
		t.Fatalf("cap 1 ran %d at once", peak.Load())
	}
	for _, p := range []string{"ok a", "ok b", "ok c"} {
		if !strings.Contains(out, p) {
			t.Fatalf("output misses %q: %s", p, out)
		}
	}
	st := rec.statuses()
	queued, firstDone := 0, -1
	for i, s := range st {
		if s == DelegateQueued {
			queued++
		}
		if s == DelegateCompleted && firstDone < 0 {
			firstDone = i
		}
	}
	if queued != 2 {
		t.Fatalf("want 2 queued children, statuses %v", st)
	}
	// Both queued events come before the first child finishes.
	for i, s := range st {
		if s == DelegateQueued && i > firstDone {
			t.Fatalf("a child appeared only after another finished: %v", st)
		}
	}
}
