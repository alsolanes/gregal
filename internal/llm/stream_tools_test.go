package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// sseToolsServer emet cada payload com un event `data:` i tanca amb [DONE].
func sseToolsServer(t *testing.T, payloads ...any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		var req chatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Errorf("la petició ha d'anar amb stream:true")
		}
		if len(req.Tools) == 0 {
			t.Errorf("la petició ha de portar tools: és tot el sentit d'aquesta via")
		}
		for _, p := range payloads {
			b, _ := json.Marshal(p)
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

func delta(d map[string]any, finish string) map[string]any {
	ch := map[string]any{"delta": d}
	if finish != "" {
		ch["finish_reason"] = finish
	}
	return map[string]any{"choices": []any{ch}}
}

func tcDelta(index int, id, name, args string) map[string]any {
	fn := map[string]any{}
	if name != "" {
		fn["name"] = name
	}
	if args != "" {
		fn["arguments"] = args
	}
	tc := map[string]any{"index": index, "function": fn}
	if id != "" {
		tc["id"] = id
		tc["type"] = "function"
	}
	return map[string]any{"tool_calls": []any{tc}}
}

var specsProva = []ToolSpec{{Name: "glob", Description: "x", Parameters: map[string]any{"type": "object"}}}

// El cas de debò: text en directe seguit d'una crida que arriba a trossos
// (id+nom primer, arguments en tres bocins), i una segona crida per l'índex 1
// intercalada. S'han de reconstruir senceres i en ordre.
func TestChatStreamWithToolsReconstrueixLesCrides(t *testing.T) {
	srv := sseToolsServer(t,
		delta(map[string]any{"content": "Deixa'm "}, ""),
		delta(map[string]any{"content": "mirar."}, ""),
		delta(tcDelta(0, "c1", "glob", ""), ""),
		delta(tcDelta(0, "", "", `{"pat`), ""),
		delta(tcDelta(1, "c2", "grep", `{"pattern":"TODO"}`), ""),
		delta(tcDelta(0, "", "", `tern":"*`), ""),
		delta(tcDelta(0, "", "", `.go"}`), ""),
		delta(map[string]any{}, "tool_calls"),
	)
	defer srv.Close()

	var tokens []string
	content, calls, err := New().ChatStreamWithTools(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "revisa"}}, 0.4, 4096, specsProva,
		func(s string) { tokens = append(tokens, s) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if content != "Deixa'm mirar." || strings.Join(tokens, "") != content {
		t.Fatalf("content=%q tokens=%q", content, tokens)
	}
	if len(calls) != 2 {
		t.Fatalf("volia 2 crides, tinc %d: %+v", len(calls), calls)
	}
	if calls[0].ID != "c1" || calls[0].Function.Name != "glob" || calls[0].Function.Arguments != `{"pattern":"*.go"}` || calls[0].Type != "function" {
		t.Fatalf("crida 0 mal reconstruïda: %+v", calls[0])
	}
	if calls[1].ID != "c2" || calls[1].Function.Name != "grep" || calls[1].Function.Arguments != `{"pattern":"TODO"}` {
		t.Fatalf("crida 1 mal reconstruïda: %+v", calls[1])
	}
}

// Només text: cap crida, i el text arriba token a token.
func TestChatStreamWithToolsNomesText(t *testing.T) {
	srv := sseToolsServer(t,
		delta(map[string]any{"content": "Hola"}, ""),
		delta(map[string]any{"content": "!"}, "stop"),
	)
	defer srv.Close()
	n := 0
	content, calls, err := New().ChatStreamWithTools(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "hola"}}, 0.7, 4096, specsProva, func(string) { n++ }, nil)
	if err != nil || content != "Hola!" || len(calls) != 0 || n != 2 {
		t.Fatalf("content=%q calls=%d n=%d err=%v", content, len(calls), n, err)
	}
}

// Un proxy que filtra DSML pel canal de text, també en streaming: es
// recupera com a crida i el markup no surt.
func TestChatStreamWithToolsRecuperaDSML(t *testing.T) {
	srv := sseToolsServer(t,
		delta(map[string]any{"content": "Va, deixa'm fer una ullada.\n\n"}, ""),
		delta(map[string]any{"content": `<｜DSML｜calls><｜DSML｜invoke name="glob"><｜DSML｜parameter name="pattern" string="true">**/*</｜DSML｜parameter></｜DSML｜invoke></｜DSML｜calls>`}, "stop"),
	)
	defer srv.Close()
	content, calls, err := New().ChatStreamWithTools(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "revisa"}}, 0.4, 4096, specsProva, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(content, "DSML") || len(calls) != 1 || calls[0].Function.Name != "glob" {
		t.Fatalf("content=%q calls=%+v", content, calls)
	}
}

// Ni text ni crides: no passa per bo (mateix criteri que la resta de vies).
func TestChatStreamWithToolsBuitÉsError(t *testing.T) {
	srv := sseToolsServer(t, delta(map[string]any{"reasoning_content": strings.Repeat("p", 500)}, "length"))
	defer srv.Close()
	_, _, err := New().ChatStreamWithTools(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "x"}}, 0.4, 256, specsProva, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "sense tokens") {
		t.Fatalf("err=%v", err)
	}
}

func TestChatStreamWithToolsNoExecutaCridaIncompleta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"write","arguments":"{\"path\":"}}]}}]}`+"\n\n")
		// Simula una connexió tallada mentre arribaven els arguments de l'eina.
	}))
	defer srv.Close()

	_, calls, err := New().ChatStreamWithTools(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "fes-ho"}}, 0.5, 100, nil, nil, nil)
	var interrupted *StreamInterruptedError
	if !errors.As(err, &interrupted) || len(calls) != 0 {
		t.Fatalf("calls=%+v err=%v: no s'ha d'executar cap crida de tool parcial", calls, err)
	}
}

// Failover: si el primari ja ha emès tokens, no es reintenta (no es poden
// retirar); si falla abans d'emetre res, sí.
func TestChatStreamWithToolsFONoRepeteixTokens(t *testing.T) {
	fails := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", 503)
	}))
	defer fails.Close()
	ok := sseToolsServer(t, delta(map[string]any{"content": "del fallback"}, "stop"))
	defer ok.Close()

	notified := ""
	content, _, usedFb, err := New().ChatStreamWithToolsFO(context.Background(),
		Target{BaseURL: fails.URL, Model: "prim"}, &Target{BaseURL: ok.URL, Model: "fb"},
		[]Message{{Role: "user", Content: "x"}}, 0.4, 256, specsProva, nil, nil, func(m string) { notified = m })
	if err != nil || !usedFb || content != "del fallback" || notified != "fb" {
		t.Fatalf("content=%q usedFb=%v notified=%q err=%v", content, usedFb, notified, err)
	}
}

// Proxy que ignora stream:true i torna el JSON sencer (i els fakes dels
// tests de la web, que fan el mateix): s'ha de llegir igualment, amb el text
// arribant a onToken d'un sol cop i les tool_calls ja muntades.
func TestChatStreamWithToolsAcceptaJSONSenseSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"miro","tool_calls":[{"id":"c1","function":{"name":"glob","arguments":"{\"pattern\":\"*.go\"}"}}]},"finish_reason":"tool_calls"}]}`)
	}))
	defer srv.Close()
	var tokens []string
	content, calls, err := New().ChatStreamWithTools(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "revisa"}}, 0.4, 4096, specsProva, func(s string) { tokens = append(tokens, s) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if content != "miro" || len(tokens) != 1 || tokens[0] != "miro" {
		t.Fatalf("content=%q tokens=%q", content, tokens)
	}
	if len(calls) != 1 || calls[0].Function.Name != "glob" || calls[0].Type != "function" {
		t.Fatalf("calls=%+v", calls)
	}
}
