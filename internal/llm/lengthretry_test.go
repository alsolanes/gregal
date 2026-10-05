package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// Servidor que falla el primer torn per length i respon el segon. Hauria de
// rebre el doble de pressupost al segon intent i el caller, la resposta.
func TestRetryLengthRecupera(t *testing.T) {
	var crides int64
	var mu sync.Mutex
	var budgets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MaxTokens int `json:"max_tokens"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &req)
		mu.Lock()
		budgets = append(budgets, req.MaxTokens)
		mu.Unlock()
		n := atomic.AddInt64(&crides, 1)
		if n == 1 {
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"","reasoning_content":"pensant molt"},"finish_reason":"length"}]}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"fet"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	out, err := New().Chat(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "hola"}}, 0.7, 1024)
	if err != nil || out != "fet" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if atomic.LoadInt64(&crides) != 2 {
		t.Fatalf("hauria de ser 1 intent + 1 reintent, han estat %d", crides)
	}
	if len(budgets) != 2 || budgets[0] != 1024 || budgets[1] != 2048 {
		t.Fatalf("pressupostos=%v, volia [1024 2048]", budgets)
	}
}

// Si ni escalant fins al sostre d'intents hi cap, l'error informa el
// pressupost DEL ROL (l'original), no el de l'últim reintent intern.
func TestRetryLengthEscalaTresCopsIDesisteix(t *testing.T) {
	var crides int64
	var mu sync.Mutex
	var budgets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MaxTokens int `json:"max_tokens"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &req)
		mu.Lock()
		budgets = append(budgets, req.MaxTokens)
		mu.Unlock()
		atomic.AddInt64(&crides, 1)
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"","reasoning_content":"pensant"},"finish_reason":"length"}]}`)
	}))
	defer srv.Close()

	_, err := New().Chat(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "hola"}}, 0.7, 1024)
	if err == nil {
		t.Fatal("hauria de fallar")
	}
	if atomic.LoadInt64(&crides) != 4 {
		t.Fatalf("1 intent + 3 reintents: %d crides", crides)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []int{1024, 2048, 4096, 8192}
	if len(budgets) != len(want) {
		t.Fatalf("pressupostos=%v, volia %v", budgets, want)
	}
	for i := range want {
		if budgets[i] != want[i] {
			t.Fatalf("pressupostos=%v, volia %v", budgets, want)
		}
	}
	le, ok := AsLengthError(err)
	if !ok {
		t.Fatalf("ha de ser LengthError: %T %v", err, err)
	}
	if le.MaxTokens != 1024 || le.Reasoned == 0 {
		t.Fatalf("ha d'informar l'original: %+v", le)
	}
	for _, want := range []string{"sense tokens", "max_tokens", "1024", "4096"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("l'error no diu %q: %v", want, err)
		}
	}
}

// Buit sense length (finish=stop) no és recuperable: ni un reintent.
func TestSenseRetrySiNoEsLength(t *testing.T) {
	var crides int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&crides, 1)
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":""},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	if _, err := New().Chat(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "hola"}}, 0.7, 512); err == nil {
		t.Fatal("hauria de fallar")
	}
	if atomic.LoadInt64(&crides) != 1 {
		t.Fatalf("sense reintent: %d crides", crides)
	}
}

// Al sostre o per sobre, tampoc es reintenta.
func TestSenseRetryAlSostre(t *testing.T) {
	var crides int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&crides, 1)
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":""},"finish_reason":"length"}]}`)
	}))
	defer srv.Close()

	if _, err := New().Chat(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "hola"}}, 0.7, MaxTokensCap); err == nil {
		t.Fatal("hauria de fallar")
	}
	if atomic.LoadInt64(&crides) != 1 {
		t.Fatalf("al sostre, sense reintent: %d crides", crides)
	}
}

// El flag «parcial» no depèn d'haver raonat, sinó d'haver ENSENYAT text de
// resposta. Un tall amb NOMÉS raonament no ha mostrat res: el torn es
// reintenta amb el doble de pressupost. Amb text visible (vegeu
// TestStreamTallatPeròAmbText) ja s'ha ensenyat i no es pot repetir.
func TestStreamLengthNomesRaonamentEsReintenta(t *testing.T) {
	var crides int64
	var mu sync.Mutex
	var budgets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			MaxTokens int `json:"max_tokens"`
		}
		_ = json.Unmarshal(raw, &req)
		mu.Lock()
		budgets = append(budgets, req.MaxTokens)
		mu.Unlock()
		n := atomic.AddInt64(&crides, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		// El pensament sempre hi és; el que canvia és si hi ha resposta.
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\""+strings.Repeat("p", 100)+"\"}}]}\n\n")
		if n == 1 {
			fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"length"}]}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"fet"}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	var tokens strings.Builder
	out, err := New().ChatStream(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "hola"}}, 0.7, 1024,
		func(tok string) { tokens.WriteString(tok) }, nil)
	if err != nil || out != "fet" {
		t.Fatalf("out=%q err=%v (hauria d'haver recuperat el torn)", out, err)
	}
	if tokens.String() != "fet" {
		t.Fatalf("tokens=%q: només s'ha d'haver emès la resposta bona", tokens.String())
	}
	if n := atomic.LoadInt64(&crides); n != 2 {
		t.Fatalf("1 intent + 1 reintent, han estat %d", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(budgets) != 2 || budgets[0] != 1024 || budgets[1] != 2048 {
		t.Fatalf("pressupostos=%v, volia [1024 2048]", budgets)
	}
}

// I el cas contrari: si el tall arriba després d'haver emès text, el flag
// «parcial» el fa no-reintentable (els tokens es duplicarien).
func TestLengthTallatMarcaSegonsTextVisible(t *testing.T) {
	if le := lengthTallat(500, 4096, 0); le.Partial {
		t.Fatalf("només raonament: ha de ser reintentable (no parcial): %+v", le)
	}
	if le := lengthTallat(500, 4096, 12); !le.Partial {
		t.Fatalf("amb text visible: ha de quedar marcat parcial: %+v", le)
	}
	// El motiu ha de continuar dient que ha estat el pensament.
	le := lengthTallat(13084, 4096, 0)
	for _, want := range []string{"raonant", "13084", "4096"} {
		if !strings.Contains(le.Error(), want) {
			t.Fatalf("l'error no diu %q: %v", want, le)
		}
	}
}

func TestChatWithToolsRetriesTruncatedToolCall(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, arguments, finish := "ok", `{"x":1}`, "tool_calls"
		if calls.Add(1) == 1 {
			id, arguments, finish = "bad", `{"x":`, "length"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{
			"message": map[string]any{"tool_calls": []any{map[string]any{
				"id": id, "type": "function", "function": map[string]string{"name": "run", "arguments": arguments},
			}}}, "finish_reason": finish,
		}}})
	}))
	defer srv.Close()

	_, got, err := New().ChatWithTools(context.Background(), srv.URL, "", "m", nil, 0, 512, nil)
	if err != nil || len(got) != 1 || got[0].ID != "ok" {
		t.Fatalf("calls=%+v err=%v", got, err)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected one bounded recovery retry, got %d calls", calls.Load())
	}
}

func TestOptInStreamRecoveryDoesNotDuplicatePartialText(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if attempts.Add(1) == 1 {
			fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"partial"}}]}`+"\n\n")
			fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"length"}]}`+"\n\n")
		} else {
			fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"complete"}}]}`+"\n\n")
			fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	var emitted strings.Builder
	out, _, err := New().ChatStreamWithTools(WithRecoverTruncation(context.Background()), srv.URL, "", "m", nil, 0, 512, nil,
		func(s string) { emitted.WriteString(s) }, nil)
	if err != nil || out != "complete" || emitted.String() != "complete" {
		t.Fatalf("out=%q emitted=%q err=%v", out, emitted.String(), err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected one recovery retry, got %d calls", attempts.Load())
	}
}

// L'escalada recupera al tercer intent: el raonament és no-determinista i
// amb un sol doblatge un segon pensament llarg tornava a tallar.
func TestRetryLengthRecuperaAlTercerIntent(t *testing.T) {
	var crides int64
	var mu sync.Mutex
	var budgets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MaxTokens int `json:"max_tokens"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &req)
		mu.Lock()
		budgets = append(budgets, req.MaxTokens)
		mu.Unlock()
		n := atomic.AddInt64(&crides, 1)
		if n < 3 {
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"","reasoning_content":"pensant molt"},"finish_reason":"length"}]}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"fet"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	out, err := New().Chat(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "hola"}}, 0.7, 1024)
	if err != nil || out != "fet" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if atomic.LoadInt64(&crides) != 3 {
		t.Fatalf("hauria de recuperar al tercer intent, han estat %d crides", crides)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []int{1024, 2048, 4096}
	if len(budgets) != len(want) {
		t.Fatalf("pressupostos=%v, volia %v", budgets, want)
	}
	for i := range want {
		if budgets[i] != want[i] {
			t.Fatalf("pressupostos=%v, volia %v", budgets, want)
		}
	}
}

// Quan el pensament observat ja es menja el pressupost (aquí ~13.000
// caràcters contra 1024), l'escalada salta a cobrir-lo més marge en comptes
// de cremar intents doblant a cegues: és la lògica d'opencode (max_tokens =
// resposta + pressupost de pensament).
func TestRetryLengthSaltaACobrirElPensament(t *testing.T) {
	var crides int64
	var mu sync.Mutex
	var budgets []int
	llarg := strings.Repeat("pensament ", 1309) // ~13.090 caràcters
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MaxTokens int `json:"max_tokens"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &req)
		mu.Lock()
		budgets = append(budgets, req.MaxTokens)
		mu.Unlock()
		n := atomic.AddInt64(&crides, 1)
		if n == 1 {
			fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"","reasoning_content":%q},"finish_reason":"length"}]}`, llarg)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"fet"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	out, err := New().Chat(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "hola"}}, 0.7, 1024)
	if err != nil || out != "fet" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []int{1024, 16384}
	if len(budgets) != len(want) {
		t.Fatalf("pressupostos=%v, volia %v", budgets, want)
	}
	for i := range want {
		if budgets[i] != want[i] {
			t.Fatalf("pressupostos=%v, volia %v", budgets, want)
		}
	}
}

// El suggeriment creix amb el raonament observat: 6145 caràcters de
// pensament (el cas real) demanen 8192, no un genèric «puja-ho».

func TestSuggeritSegonsRaonament(t *testing.T) {
	casos := map[int]int{
		0:     4096,
		8:     4096,
		6145:  8192,
		13084: 16384,
	}
	for raonat, want := range casos {
		if got := suggerit(raonat, 2048); got != want {
			t.Fatalf("suggerit(%d, 2048)=%d, volia %d", raonat, got, want)
		}
	}
	if got := suggerit(100000, 2048); got != MaxTokensCap {
		t.Fatalf("el suggeriment no passa del sostre: %d", got)
	}
}
