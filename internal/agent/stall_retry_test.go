package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/llm"
)

// A subagent step whose request never answers is cut by the inactivity
// limit and asked again once. Before, a child step had no limit at all
// apart from the HTTP client's 20 minutes, and the parent waited for it.
func TestDelegateChildStallIsCutAndRetried(t *testing.T) {
	var requests atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if requests.Add(1) == 1 {
			// Stuck upstream: headers never come.
			select {
			case <-r.Context().Done():
			case <-release:
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"done after retry"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()
	defer close(release)

	cfg := testCfg(server.URL)
	cfg.Agent.ModelTimeoutS = 1
	previousParallel := delegateParallelDefault.Load()
	SetupDelegate(cfg, llm.New())
	t.Cleanup(func() {
		delegateParallelDefault.Store(previousParallel)
		SetDelegateRunner(nil)
	})

	t0 := time.Now()
	out, _, err := ExecCtx(context.Background(), "session", t.TempDir(), "delegate", `{"tasks":[{"prompt":"look at something"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "done after retry") {
		t.Fatalf("child did not recover from the stall: %q", out)
	}
	if n := requests.Load(); n != 2 {
		t.Fatalf("requests=%d, want 2 (stalled one + retry)", n)
	}
	if d := time.Since(t0); d > 6*time.Second {
		t.Fatalf("stall took %v to detect", d)
	}
}
