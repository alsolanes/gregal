package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Un stream que el proveïdor tanca sense res (finish_reason "") ja no acaba
// el torn: es torna a demanar i la tasca continua. Era el cas real d'una
// tasca llarga que es quedava a mitges amb «resposta buida».
func TestRespostaBuidaEsTornaADemanar(t *testing.T) {
	var crides int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&crides, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		if n <= 2 {
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"fet"}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	var avisos int32
	ctx := WithRetryHook(context.Background(), func(attempt, total int, wait time.Duration, err error) { atomic.AddInt32(&avisos, 1) })
	var tokens strings.Builder
	out, err := New().ChatStream(ctx, srv.URL, "", "m", []Message{{Role: "user", Content: "hola"}}, 0.7, 1024, func(s string) { tokens.WriteString(s) }, nil)
	if err != nil || out != "fet" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if tokens.String() != "fet" {
		t.Fatalf("la pantalla ha de coincidir amb el retorn: %q", tokens.String())
	}
	if c := atomic.LoadInt32(&crides); c != 3 {
		t.Fatalf("crides=%d, volia 3", c)
	}
	if a := atomic.LoadInt32(&avisos); a != 2 {
		t.Fatalf("cada reintent s'ha d'avisar: %d", a)
	}
}

// Si continua buida, es rendeix amb el mateix error de sempre després dels
// reintents (i no en fa cap més).
func TestRespostaBuidaPersistentFalla(t *testing.T) {
	var crides int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&crides, 1)
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":""},"finish_reason":""}]}`)
	}))
	defer srv.Close()

	_, _, err := New().ChatWithTools(context.Background(), srv.URL, "", "m", []Message{{Role: "user", Content: "hola"}}, 0.7, 1024, nil)
	if err == nil || !strings.Contains(err.Error(), "resposta buida") {
		t.Fatalf("err=%v", err)
	}
	if c := atomic.LoadInt32(&crides); c != int32(MaxEmptyRetries+1) {
		t.Fatalf("crides=%d, volia %d", c, MaxEmptyRetries+1)
	}
}
