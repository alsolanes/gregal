package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func chatOK(w http.ResponseWriter) {
	fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"hola"}}]}`)
}

// 500, 500, 200: el retry ho salva (3 crides).
func TestRetrySupera500(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) < 3 {
			http.Error(w, "caigut", 500)
			return
		}
		chatOK(w)
	}))
	defer srv.Close()
	c := New()
	got, err := c.Chat(context.Background(), srv.URL, "", "m", nil, 0, 0)
	if err != nil || got != "hola" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if n.Load() != 3 {
		t.Fatalf("crides=%d, volia 3", n.Load())
	}
}

// 400: definitiu, una sola crida.
func TestRetryNoToca400(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		http.Error(w, "petició malament", 400)
	}))
	defer srv.Close()
	c := New()
	bad := Message{Role: "assistant", ToolCalls: []ToolCall{{ID: "c1", Type: "function"}}}
	bad.ToolCalls[0].Function.Arguments = `{"command":"python - <<'PY`
	if _, err := c.Chat(context.Background(), srv.URL, "", "m", []Message{bad}, 0, 0); err == nil {
		t.Fatal("el 400 ha de fallar")
	}
	if n.Load() != 1 {
		t.Fatalf("crides=%d, volia 1", n.Load())
	}
}

// Alguns proxies retornen 500 per arguments de tool_calls mal formats. És un
// error determinista i no s'ha de repetir sis vegades.
func TestRetryNoTocaMalformedToolArguments(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		http.Error(w, `{"error":{"message":"Failed to parse tool call arguments as JSON: parse_error.101: missing closing quote"}}`, 500)
	}))
	defer srv.Close()
	c := New()
	start := time.Now()
	bad := Message{Role: "assistant", ToolCalls: []ToolCall{{ID: "c1", Type: "function"}}}
	bad.ToolCalls[0].Function.Arguments = `{"command":"python - <<'PY`
	if _, err := c.Chat(context.Background(), srv.URL, "", "m", []Message{bad}, 0, 0); err == nil {
		t.Fatal("hauria de fallar")
	}
	if n.Load() != 1 {
		t.Fatalf("crides=%d, volia 1", n.Load())
	}
	if time.Since(start) > time.Second {
		t.Fatal("un error de parseig no ha d'esperar el backoff")
	}
}

func TestRetryMantéUnParseigNoRelacionatComTransient(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			http.Error(w, "parse error while decoding provider response", 500)
			return
		}
		chatOK(w)
	}))
	defer srv.Close()
	c := New()
	got, err := c.Chat(context.Background(), srv.URL, "", "m", nil, 0, 0)
	if err != nil || got != "hola" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if n.Load() != 2 {
		t.Fatalf("crides=%d, volia 2", n.Load())
	}
}

// 500 sempre: 3 intents i error (ràpid: backoff curt en test).
func TestRetryEsgota(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		http.Error(w, "caigut", 500)
	}))
	defer srv.Close()
	// Esperes curtes: el que es prova és el recompte, no el rellotge.
	base, max, jit := RetryBase, RetryMax, RetryJitter
	RetryBase, RetryMax, RetryJitter = 5*time.Millisecond, 20*time.Millisecond, time.Millisecond
	t.Cleanup(func() { RetryBase, RetryMax, RetryJitter = base, max, jit })
	c := New()
	t0 := time.Now()
	if _, err := c.Chat(context.Background(), srv.URL, "", "m", nil, 0, 0); err == nil {
		t.Fatal("hauria de fallar")
	}
	if n.Load() != RetryAttempts {
		t.Fatalf("crides=%d, volia %d", n.Load(), RetryAttempts)
	}
	if time.Since(t0) > 20*time.Second {
		t.Fatal("reintents massa lents")
	}
}
