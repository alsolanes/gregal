package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFallbackPreservesCancellation(t *testing.T) {
	prim := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer prim.Close()
	for _, mode := range []string{"chat", "tools", "stream"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				cancel()
				http.Error(w, "canceled", http.StatusServiceUnavailable)
			}))
			defer fb.Close()
			primary := Target{BaseURL: prim.URL, Model: "primary"}
			fallback := &Target{BaseURL: fb.URL, Model: "fallback"}
			client := New()
			var err error
			switch mode {
			case "chat":
				_, _, err = client.ChatFO(ctx, primary, fallback, nil, 0, 100, nil)
			case "tools":
				_, _, _, err = client.ChatWithToolsFO(ctx, primary, fallback, nil, 0, 100, nil, nil)
			case "stream":
				_, _, _, err = client.ChatStreamWithToolsFO(ctx, primary, fallback, nil, 0, 100, nil, nil, nil, nil)
			}
			if !errors.Is(err, context.Canceled) || IsRetryable(err) {
				t.Fatalf("cancellation became retryable: %v", err)
			}
		})
	}
}
