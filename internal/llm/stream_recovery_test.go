package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBufferedStreamRecoversInterruptedToolCall(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if attempts.Add(1) == 1 {
			fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"discard this","reasoning_content":"discard reasoning","tool_calls":[{"index":0,"id":"bad","function":{"name":"write","arguments":"{\"path\":"}}]}}]}`+"\n\n")
			return
		}
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"recovered","reasoning_content":"fresh reasoning","tool_calls":[{"index":0,"id":"good","function":{"name":"read","arguments":"{\"path\":\"main.go\"}"}}]},"finish_reason":"tool_calls"}]}`+"\n\n")
	}))
	defer srv.Close()
	var text, reason strings.Builder
	out, calls, err := New().ChatStreamWithTools(WithRecoverTruncation(context.Background()), srv.URL, "", "m", nil, 0, 512, nil,
		func(s string) { text.WriteString(s) }, func(s string) { reason.WriteString(s) })
	if err != nil || out != "recovered" || text.String() != "recovered" || reason.String() != "fresh reasoning" || attempts.Load() != 2 {
		t.Fatalf("out=%q text=%q reason=%q attempts=%d err=%v", out, text.String(), reason.String(), attempts.Load(), err)
	}
	if len(calls) != 1 || calls[0].ID != "good" || calls[0].Function.Name != "read" {
		t.Fatalf("partial tool calls escaped: %+v", calls)
	}
}

func TestInterruptedStreamRecoveryIsBounded(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"partial"}}]}`+"\n\n")
	}))
	defer srv.Close()
	for _, buffered := range []bool{false, true} {
		attempts.Store(0)
		ctx := context.Background()
		if buffered {
			ctx = WithRecoverTruncation(ctx)
		}
		var emitted strings.Builder
		out, calls, err := New().ChatStreamWithTools(ctx, srv.URL, "", "m", nil, 0, 512, nil, func(s string) { emitted.WriteString(s) }, nil)
		wantAttempts, wantText := int32(1), "partial"
		if buffered {
			wantAttempts, wantText = 3, ""
		}
		var interrupted *StreamInterruptedError
		if !errors.As(err, &interrupted) || attempts.Load() != wantAttempts || out != "" || len(calls) != 0 || emitted.String() != wantText {
			t.Fatalf("buffered=%v attempts=%d emitted=%q out=%q calls=%+v err=%v", buffered, attempts.Load(), emitted.String(), out, calls, err)
		}
	}
}

func TestInterruptedStreamRecoveryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hooks, attempts := 0, 0
	ctx = WithRetryHook(ctx, func(attempt, total int, wait time.Duration, err error) {
		hooks++
		if attempt != 1 || total != 3 {
			t.Errorf("unexpected retry notification %d/%d", attempt, total)
		}
		cancel()
	})
	err := retryInterruptedStream(ctx, true, func() error {
		attempts++
		return &StreamInterruptedError{}
	})
	if !errors.Is(err, context.Canceled) || attempts != 1 || hooks != 1 {
		t.Fatalf("attempts=%d hooks=%d err=%v", attempts, hooks, err)
	}
}

func TestInterruptedStreamRecoveryDoesNotRetryOtherErrors(t *testing.T) {
	want := errors.New("invalid model")
	attempts := 0
	err := retryInterruptedStream(context.Background(), true, func() error {
		attempts++
		return want
	})
	if !errors.Is(err, want) || attempts != 1 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

func TestLiveStreamCancellationDoesNotRetryOrFailOver(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"visible partial"}}]}`+"\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var emitted strings.Builder
	target := Target{BaseURL: srv.URL, Model: "m"}
	out, calls, fallback, err := New().ChatStreamWithToolsFO(ctx, target, &target, nil, 0, 512, nil, func(s string) {
		emitted.WriteString(s)
		cancel()
	}, nil, nil)
	if !errors.Is(err, context.Canceled) || IsRetryable(err) || fallback || attempts.Load() != 1 || emitted.String() != "visible partial" || out != "" || len(calls) != 0 {
		t.Fatalf("err=%v retryable=%v fallback=%v attempts=%d emitted=%q", err, IsRetryable(err), fallback, attempts.Load(), emitted.String())
	}
}
