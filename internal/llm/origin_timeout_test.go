package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Behind Cloudflare, a non-streamed answer that takes over ~100 s comes
// back as 524, every time. The client must not retry the identical request
// six times: it switches that endpoint to streaming at once and keeps it.
func TestOriginTimeoutSwitchesToStreaming(t *testing.T) {
	oblidaStreamObligat()
	t.Cleanup(oblidaStreamObligat)
	t.Setenv("HOME", t.TempDir()) // els dumps de peticions fallides (client.go)
	var plain, streamed atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Stream bool `json:"stream"`
		}
		_ = json.Unmarshal(raw, &req)
		if !req.Stream {
			plain.Add(1)
			w.WriteHeader(524)
			fmt.Fprint(w, "error code: 524")
			return
		}
		streamed.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hola\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := New()
	out, err := c.Chat(context.Background(), srv.URL, "k", "m", []Message{{Role: "user", Content: "hi"}}, 0, 100)
	if err != nil || out != "hola" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if plain.Load() != 1 || streamed.Load() != 1 {
		t.Fatalf("plain=%d streamed=%d: the 524 must not be retried", plain.Load(), streamed.Load())
	}
	// The endpoint is remembered: no more doomed non-streamed requests.
	text, _, err := c.ChatWithTools(context.Background(), srv.URL, "k", "m", []Message{{Role: "user", Content: "hi"}}, 0, 100, nil)
	if err != nil || !strings.Contains(text, "hola") {
		t.Fatalf("text=%q err=%v", text, err)
	}
	if plain.Load() != 1 || streamed.Load() != 2 {
		t.Fatalf("plain=%d streamed=%d after the switch", plain.Load(), streamed.Load())
	}
}

// A 524 on a streamed request (prefill longer than the proxy waits) is not
// retried by the client either; it still reads as a server error so the
// engine can decide.
func TestOriginTimeoutOnStreamIsNotRetried(t *testing.T) {
	oblidaStreamObligat()
	t.Cleanup(oblidaStreamObligat)
	t.Setenv("HOME", t.TempDir()) // els dumps de peticions fallides (client.go)
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(524)
	}))
	defer srv.Close()
	_, err := New().ChatStream(context.Background(), srv.URL, "k", "m", []Message{{Role: "user", Content: "hi"}}, 0, 100, nil, nil)
	if err == nil || !IsServerError(err) || n.Load() != 1 {
		t.Fatalf("err=%v requests=%d", err, n.Load())
	}
}
