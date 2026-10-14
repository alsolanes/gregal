package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/llm"
)

func TestCanceledToolBatchDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := make([]llm.ToolCall, 2)
	for i := range calls {
		calls[i].Function.Name = "write"
		calls[i].Function.Arguments = `{"path":"file.txt","content":"changed"}`
	}
	outs := RunCalls(calls, nil, func(i int, c llm.ToolCall) error {
		if i == 0 {
			cancel()
		}
		_, _, err := ExecCtx(ctx, "", dir, c.Function.Name, c.Function.Arguments)
		return err
	})
	for _, err := range outs {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("canceled tool created a file: %v", err)
	}
}

func TestHeadlessCancellationBetweenModelAndTools(t *testing.T) {
	dir := t.TempDir()
	srv := fakeAgentServer(t, filepath.Join(dir, "missing.txt"))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := RunNonInteractiveExIn(ctx, llm.New(), testCfg(srv.URL), "read file", "code", 5, false, dir,
		func(step int, tools []string) {
			if step > 0 {
				cancel()
			}
		})
	if !errors.Is(err, context.Canceled) || res.Answer != "" || res.Steps != 1 {
		t.Fatalf("result=%+v err=%v", res, err)
	}
	for _, trace := range res.Trace {
		if trace.Summary != "" {
			t.Fatalf("tool executed after cancellation: %+v", trace)
		}
	}
}

func TestHeadlessCancellationDuringToolLeavesNoFakeResults(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		inner, _ := json.Marshal(map[string]string{"command": "sleep 20"})
		argsField, _ := json.Marshal(string(inner))
		fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"bash","arguments":%s}}]},"finish_reason":"tool_calls"}]}`, argsField)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := RunNonInteractiveExIn(ctx, llm.New(), testCfg(srv.URL), "sleep a while", "code", 5, true, dir,
		func(step int, tools []string) {
			if step > 0 {
				go func() {
					time.Sleep(300 * time.Millisecond)
					cancel()
				}()
			}
		})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("result=%+v err=%v", res, err)
	}
	for _, trace := range res.Trace {
		if strings.Contains(trace.Summary, "cancel") || strings.Contains(trace.Summary, "ERROR:") {
			t.Fatalf("canceled tool polluted history and trace: %+v", trace)
		}
	}
}
