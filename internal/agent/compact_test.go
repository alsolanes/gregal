package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/llm"
)

func TestNeedsCompact(t *testing.T) {
	if !NeedsCompact(25000, 32768) {
		t.Fatal("25K de 32K ha de compactar")
	}
	if NeedsCompact(10000, 32768) {
		t.Fatal("10K de 32K no ha de compactar")
	}
	if !NeedsCompact(25000, 0) {
		t.Fatal("finestra 0 usa el defecte 32K")
	}
	if WindowOf(0) != DefaultWindow || WindowOf(131072) != 131072 {
		t.Fatal("WindowOf")
	}
}

// La reserva de resposta descompta max_tokens de la finestra, però mai més
// d'un quart: un max_tokens desproporcionat no ha de deixar el prompt a zero.
func TestPromptBudget(t *testing.T) {
	if got := PromptBudget(32768, 0); got != 32768 {
		t.Fatalf("sense max_tokens el pressupost és la finestra: %d", got)
	}
	if got := PromptBudget(32768, 8192); got != 32768-8192 {
		t.Fatalf("ha de reservar 8192: %d", got)
	}
	// max_tokens enorme: la reserva es limita a window/4.
	if got := PromptBudget(32768, 999999); got != 32768-8192 {
		t.Fatalf("la reserva es limita a un quart: %d", got)
	}
	// Finestra no declarada → defecte sa.
	if got := PromptBudget(0, 1024); got != DefaultWindow-1024 {
		t.Fatalf("finestra 0 usa el defecte: %d", got)
	}
}

func TestTrimKeep(t *testing.T) {
	msgs := []llm.Message{{Role: "user", Content: "1"}, {Role: "assistant", Content: "2"}, {Role: "user", Content: "3"}}
	kept := TrimKeep(msgs, 2)
	if len(kept) != 2 || kept[0].Content != "2" || kept[1].Content != "3" {
		t.Fatalf("kept=%v", kept)
	}
	if len(TrimKeep(msgs, 9)) != 3 {
		t.Fatal("keep gran ho manté tot")
	}
	// No comparteix array (modificar kept no toca l'original).
	kept[0].Content = "X"
	if msgs[1].Content != "2" {
		t.Fatal("aliasing")
	}
}

func TestSummarizeAmbServidor(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Body.Read(buf)
			if n > 0 {
				b.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		gotBody = b.String()
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"RESUM FET"}}]}`))
	}))
	defer srv.Close()
	c := llm.New()
	convo := []llm.Message{
		{Role: "user", Content: "fes X"},
		{Role: "assistant", Content: "fet", ToolCalls: []llm.ToolCall{{ID: "1", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "write", Arguments: `{"path":"a"}`}}}},
	}
	out, err := Summarize(context.Background(), c, llm.Target{BaseURL: srv.URL, Model: "m"}, nil, convo)
	if err != nil {
		t.Fatal(err)
	}
	if out != "RESUM FET" {
		t.Fatalf("out=%q", out)
	}
	if !strings.Contains(gotBody, "fes X") || !strings.Contains(gotBody, "write") {
		t.Fatalf("el prompt ha de dur conversa + eines: %.120s", gotBody)
	}
}

func TestSummarizeBuitÉsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"  "}}]}`))
	}))
	defer srv.Close()
	_, err := Summarize(context.Background(), llm.New(), llm.Target{BaseURL: srv.URL, Model: "m"}, nil,
		[]llm.Message{{Role: "user", Content: "hola"}})
	if err == nil {
		t.Fatal("resum buit ha de fallar")
	}
}

func TestParseContextExceeded(t *testing.T) {
	cases := []struct {
		errStr   string
		exceeded bool
		tokens   int
	}{
		{`provider http://localhost:8089/v1: HTTP 400: {"error":{"code":400,"message":"request (36085 tokens) exceeds the available context size (32768 tokens), try increasing it","type":"exceed_context_size_error","n_prompt_tokens":36085,"n_ctx":32768}}`, true, 32768},
		{`HTTP 400: maximum context length is 8192 tokens. However, your request resulted in 9000 tokens`, true, 8192},
		{`prompt is too long: 40000 tokens > 32768 maximum`, true, 0},
		{`HTTP 500: internal server error`, false, 0},
		{`HTTP 404: model not found`, false, 0},
	}
	for _, tc := range cases {
		exc, tok := ParseContextExceeded(fmt.Errorf("%s", tc.errStr))
		if exc != tc.exceeded {
			t.Errorf("errStr %q: exc=%v esperat=%v", tc.errStr, exc, tc.exceeded)
		}
		if tc.tokens > 0 && tok != tc.tokens {
			t.Errorf("errStr %q: tok=%d esperat=%d", tc.errStr, tok, tc.tokens)
		}
	}
}
