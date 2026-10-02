package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// El cas real: un model de raonament amb max_tokens curt es menja tot el
// pressupost pensant i la resposta ja no hi cap. Abans tornava ("", nil) i
// el TUI pintava una línia en blanc sense dir per què.
func TestStreamSenseRespostaPerTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\""+strings.Repeat("p", 3500)+"\"}}]}\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"length"}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	out, err := New().ChatStream(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "hola"}}, 0.7, 1024, nil, nil)
	if out != "" {
		t.Fatalf("no hi havia d'haver resposta: %q", out)
	}
	if err == nil {
		t.Fatal("un torn sense ni una lletra no pot passar per bo")
	}
	for _, want := range []string{"sense tokens", "3500", "max_tokens", "1024"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("l'error no diu %q: %v", want, err)
		}
	}
}

// Una resposta parcial no es pot presentar com a completa ni repetir-se
// després d'haver-ne emès els tokens.
func TestStreamTallatPeròAmbText(t *testing.T) {
	var crides int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		crides++
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"a mitges"}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"length"}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	out, err := New().ChatStream(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "hola"}}, 0.7, 1024, nil, nil)
	if out != "" || err == nil || !strings.Contains(err.Error(), "resposta s'ha tallat") {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if crides != 1 {
		t.Fatalf("un stream parcial no s'ha de repetir (tokens duplicats): %d crides", crides)
	}
}

func TestChatNoAcceptaRespostaParcialPerLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"a mitges"},"finish_reason":"length"}]}`)
	}))
	defer srv.Close()

	if out, err := New().Chat(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "hola"}}, 0.7, 512); err == nil || out != "" {
		t.Fatalf("out=%q err=%v: la resposta parcial ha de quedar marcada", out, err)
	}
}

func TestStreamSenseSenyalsFinalsNoPassaComplet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"resposta a mitges"}}]}`+"\n\n")
		// La connexió es tanca sense finish_reason ni [DONE].
	}))
	defer srv.Close()

	var tokens strings.Builder
	out, err := New().ChatStream(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "hola"}}, 0.7, 512,
		func(token string) { tokens.WriteString(token) }, nil)
	var interrupted *StreamInterruptedError
	if out != "" || !errors.As(err, &interrupted) || tokens.String() != "resposta a mitges" {
		t.Fatalf("out=%q tokens=%q err=%v", out, tokens.String(), err)
	}
	if IsRetryable(err) {
		t.Fatal("un stream parcial no es pot tractar com un error temporal del servidor")
	}
}

// La via no-streaming tenia el mateix punt cec.
func TestChatSenseContingut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"","reasoning_content":"pensant"},"finish_reason":"length"}]}`)
	}))
	defer srv.Close()

	if _, err := New().Chat(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "hola"}}, 0.7, 512); err == nil {
		t.Fatal("contingut buit no pot passar per resposta bona")
	}
}

// Però amb eines, contingut buit és perfectament normal: el torn és la crida.
func TestChatWithToolsContingutBuitAmbEinesÉsVàlid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","function":{"name":"read","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`)
	}))
	defer srv.Close()

	_, calls, err := New().ChatWithTools(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "hola"}}, 0.4, 512, nil)
	if err != nil {
		t.Fatalf("una crida d'eina sense text és vàlida: %v", err)
	}
	if len(calls) != 1 || calls[0].Type != "function" {
		t.Fatalf("calls=%+v", calls)
	}
}

// Només-espais amb finish=stop també és torn buit: abans passava com a
// resposta bona i el TUI pintava un GREGAL en blanc sense error.
func TestChatNomesEspaisEsBuit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"  \n  "},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	out, err := New().Chat(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "hola"}}, 0.7, 512)
	if err == nil || out != "" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
