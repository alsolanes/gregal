package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// registreProgres recull els avisos de StreamProgress (segur per a
// concurrència: el handler del test el consulta mentre el client llegeix).
type registreProgres struct {
	mu     sync.Mutex
	firsts int
	totals map[string]int
	ordre  []string
}

func (r *registreProgres) fn(kind string, n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.totals == nil {
		r.totals = map[string]int{}
	}
	if kind == ProgressFirst {
		r.firsts++
	}
	r.totals[kind] += n
	r.ordre = append(r.ordre, kind)
}

func (r *registreProgres) total(kind string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.totals[kind]
}

func (r *registreProgres) primers() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.firsts
}

func sseLinia(w http.ResponseWriter, payload string) {
	fmt.Fprint(w, "data: "+payload+"\n\n")
	if fl, ok := w.(http.Flusher); ok {
		fl.Flush()
	}
}

func TestWithStreamProgressNilNoCanviaElContext(t *testing.T) {
	ctx := context.Background()
	if got := WithStreamProgress(ctx, nil); got != ctx {
		t.Fatal("un observador nil no ha de crear un context nou")
	}
	if StreamProgressFrom(ctx) != nil {
		t.Fatal("sense observador, StreamProgressFrom ha de tornar nil")
	}
	// Un punter nil de progrés no fa res (els parsers el criden sempre).
	var p *progresResposta
	p.tros()
	p.delta(ProgressText, "hola")
	if nouProgres(ctx) != nil {
		t.Fatal("sense observador no s'ha de crear estat")
	}
}

// En streaming normal, cada delta avisa amb els seus caràcters (runes, no
// bytes) i el primer tros avisa una sola vegada.
func TestStreamProgressComptaDeltesEnDirecte(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sseLinia(w, `{"choices":[{"delta":{"reasoning_content":"penso"}}]}`)
		sseLinia(w, `{"choices":[{"delta":{"reasoning_content":" més"}}]}`)
		sseLinia(w, `{"choices":[{"delta":{"content":"això"}}]}`)
		sseLinia(w, `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"read","arguments":"{\"path\":"}}]}}]}`)
		sseLinia(w, `{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"a.go\"}"}}]},"finish_reason":"tool_calls"}]}`)
		sseLinia(w, `[DONE]`)
	}))
	defer srv.Close()
	reg := &registreProgres{}
	ctx := WithStreamProgress(context.Background(), reg.fn)
	text, calls, err := New().ChatStreamWithTools(ctx, srv.URL, "", "m", nil, 0, 512, nil, nil, nil)
	if err != nil || text != "això" || len(calls) != 1 {
		t.Fatalf("text=%q calls=%+v err=%v", text, calls, err)
	}
	if reg.primers() != 1 {
		t.Fatalf("first=%d, ha de ser 1 per resposta", reg.primers())
	}
	if got := reg.total(ProgressReasoning); got != len([]rune("penso més")) {
		t.Fatalf("reasoning=%d", got)
	}
	if got := reg.total(ProgressText); got != len([]rune("això")) {
		t.Fatalf("text=%d (ha de comptar caràcters, no bytes)", got)
	}
	if got := reg.total(ProgressTool); got != len(`{"path":"a.go"}`) {
		t.Fatalf("tool=%d", got)
	}
	if reg.ordre[0] != ProgressFirst {
		t.Fatalf("el primer avís ha de ser first: %v", reg.ordre)
	}
}

// El cas que es va veure en viu: en mode recuperació (buffered) onToken i
// onReason no reben res fins al final, però l'observador sí que ha de veure
// els deltes mentre el model encara escriu.
func TestStreamProgressArribaEnModeRecuperacioAbansDelFinal(t *testing.T) {
	reg := &registreProgres{}
	vist := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sseLinia(w, `{"choices":[{"delta":{"reasoning_content":"raonant a poc a poc"}}]}`)
		// El servidor no acaba fins que el client ha avisat del raonament:
		// si el progrés anés retingut com el text, això faria timeout.
		select {
		case <-vist:
		case <-time.After(5 * time.Second):
			t.Error("el progrés del raonament no ha arribat abans del final de la resposta")
		}
		sseLinia(w, `{"choices":[{"delta":{"content":"fet"},"finish_reason":"stop"}]}`)
		sseLinia(w, `[DONE]`)
	}))
	defer srv.Close()
	var once sync.Once
	ctx := WithStreamProgress(WithRecoverTruncation(context.Background()), func(kind string, n int) {
		reg.fn(kind, n)
		if kind == ProgressReasoning {
			once.Do(func() { close(vist) })
		}
	})
	var tokens, raons strings.Builder
	text, _, err := New().ChatStreamWithTools(ctx, srv.URL, "", "m", nil, 0, 512, nil,
		func(s string) { tokens.WriteString(s) }, func(s string) { raons.WriteString(s) })
	if err != nil || text != "fet" {
		t.Fatalf("text=%q err=%v", text, err)
	}
	// El comportament buffered no canvia: els callbacks reben el total al final.
	if tokens.String() != "fet" || raons.String() != "raonant a poc a poc" {
		t.Fatalf("callbacks alterats: tokens=%q raons=%q", tokens.String(), raons.String())
	}
	if reg.primers() != 1 || reg.total(ProgressReasoning) != len([]rune("raonant a poc a poc")) || reg.total(ProgressText) != 3 {
		t.Fatalf("progrés incomplet: first=%d totals=%v", reg.primers(), reg.totals)
	}
}

// Cada resposta HTTP té el seu primer tros: un reintent de stream
// interromput (mode recuperació) torna a avisar.
func TestStreamProgressFirstPerRespostaAmbReintent(t *testing.T) {
	var mu sync.Mutex
	intents := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		intents++
		n := intents
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		if n == 1 {
			sseLinia(w, `{"choices":[{"delta":{"content":"tallat"}}]}`)
			return // sense [DONE] ni finish_reason: stream interromput
		}
		sseLinia(w, `{"choices":[{"delta":{"content":"sencer"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	reg := &registreProgres{}
	ctx := WithStreamProgress(WithRecoverTruncation(context.Background()), reg.fn)
	text, _, err := New().ChatStreamWithTools(ctx, srv.URL, "", "m", nil, 0, 512, nil, nil, nil)
	if err != nil || text != "sencer" {
		t.Fatalf("text=%q err=%v", text, err)
	}
	if reg.primers() != 2 {
		t.Fatalf("first=%d, n'hi ha d'haver un per resposta (2)", reg.primers())
	}
	if got := reg.total(ProgressText); got != len("tallat")+len("sencer") {
		t.Fatalf("text=%d: el progrés compta tot el que arriba, també el descartat", got)
	}
}

// Proxies que ignoren stream:true: la resposta JSON sencera també avisa.
func TestStreamProgressRespostaJSONSencera(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"hola","reasoning_content":"pensat","tool_calls":[{"id":"c1","function":{"name":"read","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`)
	}))
	defer srv.Close()
	reg := &registreProgres{}
	ctx := WithStreamProgress(context.Background(), reg.fn)
	if _, _, err := New().ChatStreamWithTools(ctx, srv.URL, "", "m", nil, 0, 512, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if reg.primers() != 1 || reg.total(ProgressText) != 4 || reg.total(ProgressReasoning) != 6 || reg.total(ProgressTool) != 2 {
		t.Fatalf("first=%d totals=%v", reg.primers(), reg.totals)
	}
}

// ChatStream (sense eines) fa servir el mateix observador.
func TestStreamProgressChatStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sseLinia(w, `{"choices":[{"delta":{"reasoning":"rumio"}}]}`)
		sseLinia(w, `{"choices":[{"delta":{"content":"resposta"},"finish_reason":"stop"}]}`)
		sseLinia(w, `[DONE]`)
	}))
	defer srv.Close()
	reg := &registreProgres{}
	ctx := WithStreamProgress(context.Background(), reg.fn)
	out, err := New().ChatStream(ctx, srv.URL, "", "m", nil, 0, 512, nil, nil)
	if err != nil || out != "resposta" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if reg.primers() != 1 || reg.total(ProgressReasoning) != 5 || reg.total(ProgressText) != 8 {
		t.Fatalf("first=%d totals=%v", reg.primers(), reg.totals)
	}
}
