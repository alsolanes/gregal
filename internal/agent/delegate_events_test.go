package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func TestDelegateEventsConcurrentLifecycleAndOrder(t *testing.T) {
	var mu sync.Mutex
	events := []DelegateEvent{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"done"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()
	cfg := testCfg(server.URL)
	ctx := WithDelegateHook(context.Background(), func(event DelegateEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})
	ctx = WithDelegateRunner(ctx, cfg, llm.New())
	out, err := delegateExec(ctx, `{"tasks":[{"prompt":"first"},{"prompt":"second"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(out, "== subagent 1:") < 0 || strings.Index(out, "== subagent 2:") < strings.Index(out, "== subagent 1:") {
		t.Fatalf("output order changed: %s", out)
	}
	mu.Lock()
	defer mu.Unlock()
	byID := map[string][]DelegateEvent{}
	for _, event := range events {
		if event.ID == "" || event.Task == "" || event.StartedAt.IsZero() || event.UpdatedAt.IsZero() {
			t.Errorf("event missing identity or timestamps: %+v", event)
		}
		byID[event.ID] = append(byID[event.ID], event)
	}
	if len(byID) != 2 {
		t.Fatalf("want two unique child IDs, got %d events=%+v", len(byID), events)
	}
	for id, childEvents := range byID {
		if childEvents[0].Status != DelegateWorking || childEvents[len(childEvents)-1].Status != DelegateCompleted {
			t.Errorf("child %s lifecycle=%+v", id, childEvents)
		}
		foundModel := false
		for _, event := range childEvents {
			if event.Model == "m" {
				foundModel = true
			}
		}
		if !foundModel {
			t.Errorf("child %s did not report selected model: %+v", id, childEvents)
		}
	}
}

func TestDelegateScopedRunnerSelectsDelegateRole(t *testing.T) {
	type requestInfo struct {
		model string
		think any
	}
	models := make(chan requestInfo, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		_ = json.NewDecoder(r.Body).Decode(&request)
		models <- requestInfo{model: request["model"].(string), think: request["enable_thinking"]}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()
	cfg := testCfg(server.URL)
	cfg.Roles["delegate"] = config.Role{Provider: "t", Model: "delegate-model", Temperature: 0.1, MaxTokens: 99, Think: "no"}
	ctx := WithDelegateRunner(context.Background(), cfg, llm.New())
	if _, err := delegateExec(ctx, `{"tasks":[{"prompt":"inspect"}]}`); err != nil {
		t.Fatal(err)
	}
	select {
	case request := <-models:
		if request.model != "delegate-model" {
			t.Fatalf("model=%q, want delegate-model", request.model)
		}
		if request.think != false {
			t.Fatalf("Think setting not applied: enable_thinking=%v", request.think)
		}
	case <-time.After(time.Second):
		t.Fatal("no scoped runner request")
	}
}

func TestDelegateScopedRunnerReportsInvalidExplicitRole(t *testing.T) {
	cfg := testCfg("http://unused")
	cfg.Roles["delegate"] = config.Role{Provider: "missing", Model: "m"}
	out, err := delegateExec(WithDelegateRunner(context.Background(), cfg, llm.New()), `{"tasks":[{"prompt":"inspect"}]}`)
	if err != nil || !strings.Contains(out, `provider "missing" del rol "delegate"`) {
		t.Fatalf("expected clear invalid delegate provider error, got output=%q err=%v", out, err)
	}
}

func TestDelegateScopedRunnerCapsConcurrentCalls(t *testing.T) {
	ctx := WithDelegateRunner(context.Background(), testCfg("http://unused"), llm.New())
	started := make(chan struct{}, 8)
	release := make(chan struct{})
	active := atomic.Int32{}
	var maxActive atomic.Int32
	runner := func(ctx context.Context, prompt string) (string, error) {
		current := active.Add(1)
		for old := maxActive.Load(); current > old && !maxActive.CompareAndSwap(old, current); old = maxActive.Load() {
		}
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			active.Add(-1)
			return "", ctx.Err()
		}
		active.Add(-1)
		return "ok", nil
	}
	ctx = context.WithValue(ctx, delegateRunnerContextKey{}, delegateRunnerContext{run: runner, semaphore: ctx.Value(delegateRunnerContextKey{}).(delegateRunnerContext).semaphore})
	args := `{"tasks":[{"prompt":"1"},{"prompt":"2"},{"prompt":"3"},{"prompt":"4"}]}`
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = delegateExec(ctx, args)
		}()
	}
	for i := 0; i < MaxDelegateTasks; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("workers did not start")
		}
	}
	select {
	case <-started:
		close(release)
		t.Fatal("scoped runner exceeded concurrency cap")
	case <-time.After(50 * time.Millisecond):
	}
	if got := maxActive.Load(); got > MaxDelegateTasks {
		close(release)
		t.Fatalf("active=%d, cap=%d", got, MaxDelegateTasks)
	}
	close(release)
	wg.Wait()
}

func TestDelegateEventsFailureCancellationAndPreview(t *testing.T) {
	var mu sync.Mutex
	var events []DelegateEvent
	ctx := WithDelegateHook(context.Background(), func(event DelegateEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})
	SetDelegateRunner(func(ctx context.Context, prompt string) (string, error) {
		if prompt == "fail" {
			return "", fmt.Errorf("failed")
		}
		<-ctx.Done()
		return "", ctx.Err()
	})
	t.Cleanup(func() { SetDelegateRunner(nil) })
	cancelCtx, cancel := context.WithCancel(ctx)
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, err := delegateExec(cancelCtx, `{"tasks":[{"prompt":"fail"},{"prompt":"cancel"}]}`)
	if err != context.Canceled {
		t.Fatalf("delegate err=%v, want parent cancellation", err)
	}
	mu.Lock()
	defer mu.Unlock()
	statuses := map[string]string{}
	for _, event := range events {
		if event.Status == DelegateFailed || event.Status == DelegateCancelled {
			statuses[event.Task] = event.Status
		}
	}
	if statuses["fail"] != DelegateFailed || statuses["cancel"] != DelegateCancelled {
		t.Fatalf("terminal statuses=%v events=%+v", statuses, events)
	}
	if got := delegatePreview(strings.Repeat("x", 2005)); len([]rune(got)) != 2001 || !strings.HasSuffix(got, "…") {
		t.Fatalf("preview not bounded with marker: chars=%d suffix=%q", len([]rune(got)), got[len(got)-3:])
	}
}

func TestDelegateTimeoutEventIsFailed(t *testing.T) {
	var terminal string
	ctx := WithDelegateHook(context.Background(), func(event DelegateEvent) {
		if event.Status == DelegateFailed || event.Status == DelegateCancelled {
			terminal = event.Status
		}
	})
	SetDelegateRunner(func(context.Context, string) (string, error) { return "", context.DeadlineExceeded })
	t.Cleanup(func() { SetDelegateRunner(nil) })
	_, err := delegateExec(ctx, `{"tasks":[{"prompt":"timeout"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if terminal != DelegateFailed {
		t.Fatalf("deadline should be failed, got %q", terminal)
	}
}

func TestDelegateRunnerPanicBecomesFailedChild(t *testing.T) {
	var mu sync.Mutex
	var events []DelegateEvent
	ctx := WithDelegateHook(context.Background(), func(event DelegateEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})
	ctx = WithDelegateRunner(ctx, nil, nil)
	scoped := ctx.Value(delegateRunnerContextKey{}).(delegateRunnerContext)
	scoped.run = func(_ context.Context, prompt string) (string, error) {
		if prompt == "panic" {
			panic("broken runner")
		}
		return "still alive", nil
	}
	ctx = context.WithValue(ctx, delegateRunnerContextKey{}, scoped)
	out, err := delegateExec(ctx, `{"tasks":[{"prompt":"panic"},{"prompt":"ok"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "subagent panic: broken runner") || !strings.Contains(out, "still alive") {
		t.Fatalf("unexpected child results: %s", out)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, event := range events {
		if event.Task == "panic" && event.Status == DelegateFailed {
			return
		}
	}
	t.Fatalf("panic child was not marked failed: %+v", events)
}

func TestDelegateEventsCarryChildUsageTotals(t *testing.T) {
	usageBody := func(up, down int) string {
		if up < 0 {
			return `{"choices":[{"message":{"role":"assistant","content":"done"},"finish_reason":"stop"}]}`
		}
		return fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":"done"},"finish_reason":"stop"}],"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d}}`, up, down, up+down)
	}
	run := func(t *testing.T, up, down int) []DelegateEvent {
		t.Helper()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, usageBody(up, down))
		}))
		defer server.Close()
		var mu sync.Mutex
		var events []DelegateEvent
		ctx := WithDelegateHook(context.Background(), func(event DelegateEvent) {
			mu.Lock()
			events = append(events, event)
			mu.Unlock()
		})
		ctx = WithDelegateRunner(ctx, testCfg(server.URL), llm.New())
		if _, err := delegateExec(ctx, `{"tasks":[{"prompt":"count me"}]}`); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		defer mu.Unlock()
		return append([]DelegateEvent(nil), events...)
	}

	withUsage := run(t, 100, 50)
	var done *DelegateEvent
	sawPostCall := false
	for i, event := range withUsage {
		if event.Status == DelegateCompleted {
			done = &withUsage[i]
		}
		if event.Calls > 1 || event.UpTokens > 100 || event.DownTokens > 50 {
			t.Fatalf("child event exceeds the single real call: %+v", event)
		}
		if event.Calls == 1 {
			sawPostCall = true
			if !event.UsageKnown || event.UpTokens != 100 || event.DownTokens != 50 {
				t.Fatalf("post-call child event missing usage totals: %+v", event)
			}
		}
	}
	if done == nil || !sawPostCall {
		t.Fatalf("no completed child event with usage: %+v", withUsage)
	}
	if done.UpTokens != 100 || done.DownTokens != 50 || done.Calls != 1 || !done.UsageKnown {
		t.Fatalf("completed child missing usage totals: %+v", done)
	}

	withoutUsage := run(t, -1, -1)
	sawTerminal := false
	for _, event := range withoutUsage {
		if event.Status == DelegateCompleted {
			sawTerminal = true
		}
		// One real call happened, but the provider declared nothing:
		// calls show, tokens stay silent instead of inventing zeros.
		if event.Calls > 1 || event.UsageKnown || event.UpTokens != 0 || event.DownTokens != 0 {
			t.Fatalf("calls without declared usage must stay silent: %+v", event)
		}
	}
	if !sawTerminal {
		t.Fatalf("no completed child event: %+v", withoutUsage)
	}
	for _, event := range withoutUsage {
		if event.Status == DelegateCompleted && event.Calls != 1 {
			t.Fatalf("completed child must count its call: %+v", event)
		}
	}
}

func TestDelegateHookPanicDoesNotBreakExecution(t *testing.T) {
	ctx := WithDelegateHook(context.Background(), func(DelegateEvent) { panic("observer") })
	SetDelegateRunner(func(context.Context, string) (string, error) { return "ok", nil })
	t.Cleanup(func() { SetDelegateRunner(nil) })
	if _, err := delegateExec(ctx, `{"tasks":[{"prompt":"inspect"}]}`); err != nil {
		t.Fatalf("observer panic affected execution: %v", err)
	}
}

func TestDelegateGlobalLiveCapBoundsProductionRunners(t *testing.T) {
	// Via global sense semàfor per torn (TUI, Telegram, headless): 3 crides
	// de 4 en paral·lel no poden passar de MaxDelegateLiveTotal vius.
	release := make(chan struct{})
	t.Cleanup(func() { select { case <-release: default: close(release) } })
	active := atomic.Int32{}
	var maxActive atomic.Int32
	SetDelegateRunner(func(ctx context.Context, _ string) (string, error) {
		current := active.Add(1)
		for old := maxActive.Load(); current > old && !maxActive.CompareAndSwap(old, current); old = maxActive.Load() {
		}
		select {
		case <-release:
		case <-ctx.Done():
			active.Add(-1)
			return "", ctx.Err()
		}
		active.Add(-1)
		return "ok", nil
	})
	t.Cleanup(func() { SetDelegateRunner(nil) })
	args := `{"tasks":[{"prompt":"1"},{"prompt":"2"},{"prompt":"3"},{"prompt":"4"}]}`
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = delegateExec(context.Background(), args)
		}()
	}
	deadline := time.Now().Add(5 * time.Second)
	for maxActive.Load() < MaxDelegateLiveTotal && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := maxActive.Load(); got != MaxDelegateLiveTotal {
		close(release)
		wg.Wait()
		t.Fatalf("live children=%d, want exactly the global cap %d", got, MaxDelegateLiveTotal)
	}
	close(release)
	wg.Wait()
	if got := maxActive.Load(); got > MaxDelegateLiveTotal {
		t.Fatalf("live children exceeded the global cap: %d", got)
	}
}

func TestDelegateChildSummaryIsCappedWithMark(t *testing.T) {
	long := strings.Repeat("dada ", 2000) // 10000 caràcters
	SetDelegateRunner(func(context.Context, string) (string, error) { return long, nil })
	t.Cleanup(func() { SetDelegateRunner(nil) })
	out, err := delegateExec(context.Background(), `{"tasks":[{"prompt":"resumeix molt"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "retallat a 6000 de 9999 caràcters") {
		t.Fatalf("long child result passed uncapped: %d chars", len([]rune(out)))
	}
	if n := len([]rune(out)); n > 7000 {
		t.Fatalf("capped child result still too long: %d runes", n)
	}
	short := "breu"
	SetDelegateRunner(func(context.Context, string) (string, error) { return short, nil })
	out, err = delegateExec(context.Background(), `{"tasks":[{"prompt":"resumeix poc"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "== subagent 1:\nbreu") || strings.Contains(out, "retallat") {
		t.Fatalf("short child result must pass untouched: %q", out)
	}
}
