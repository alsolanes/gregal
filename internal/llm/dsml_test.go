package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// El cas real, tal com va sortir al TUI (barres doblades): el model, sense
// esquema de tools a la petició, va escriure la crida com a text i l'usuari
// la va veure impresa.
const dsmlReal = "Va, deixa'm fer una ullada al projecte.\n\n" +
	"<｜｜DSML｜｜ calls> <｜｜DSML｜｜ invoke name=\"glob\"> <｜｜DSML｜｜ parameter name=\"pattern\" " +
	"string=\"true\">**/*</｜｜DSML｜｜ parameter> </｜｜DSML｜｜ invoke> </｜｜DSML｜｜ calls>"

func TestParseDSMLCasReal(t *testing.T) {
	clean, calls := parseDSML(dsmlReal)
	if clean != "Va, deixa'm fer una ullada al projecte." {
		t.Fatalf("text visible=%q", clean)
	}
	if len(calls) != 1 || calls[0].Function.Name != "glob" || calls[0].Type != "function" || calls[0].ID == "" {
		t.Fatalf("calls=%+v", calls)
	}
	if calls[0].Function.Arguments != `{"pattern":"**/*"}` {
		t.Fatalf("arguments=%s", calls[0].Function.Arguments)
	}
}

// Barra simple i paràmetres sense string="true": són JSON (números,
// booleans, llistes) i s'han de conservar com a tal.
func TestParseDSMLBarraSimpleIJSON(t *testing.T) {
	in := `<｜DSML｜calls><｜DSML｜invoke name="read"><｜DSML｜parameter name="path" string="true">a.go</｜DSML｜parameter>` +
		`<｜DSML｜parameter name="start">3</｜DSML｜parameter><｜DSML｜parameter name="tags">["x","y"]</｜DSML｜parameter></｜DSML｜invoke></｜DSML｜calls>`
	clean, calls := parseDSML(in)
	if clean != "" || len(calls) != 1 {
		t.Fatalf("clean=%q calls=%d", clean, len(calls))
	}
	if got := calls[0].Function.Arguments; got != `{"path":"a.go","start":3,"tags":["x","y"]}` {
		t.Fatalf("arguments=%s", got)
	}
}

// Dues crides en un bloc: dues ToolCall, ids diferents.
func TestParseDSMLDuesCrides(t *testing.T) {
	in := `<｜DSML｜calls><｜DSML｜invoke name="glob"><｜DSML｜parameter name="pattern" string="true">*.go</｜DSML｜parameter></｜DSML｜invoke>` +
		`<｜DSML｜invoke name="grep"><｜DSML｜parameter name="pattern" string="true">TODO</｜DSML｜parameter></｜DSML｜invoke></｜DSML｜calls>`
	_, calls := parseDSML(in)
	if len(calls) != 2 || calls[0].ID == calls[1].ID || calls[1].Function.Name != "grep" {
		t.Fatalf("calls=%+v", calls)
	}
}

// Sense DSML no es toca res: ni el text ni cap crida inventada.
func TestParseDSMLPassthrough(t *testing.T) {
	in := "Hola! Pots escriure <b>això</b> i ｜ una barra solta ｜ sense por."
	clean, calls := parseDSML(in)
	if clean != in || calls != nil {
		t.Fatalf("clean=%q calls=%v", clean, calls)
	}
}

// La via streaming no porta eines: el markup no s'imprimeix mai; es treu del
// text i l'error diu quina eina volia cridar.
func TestChatStreamDSMLNoSImprimeix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// El markup arriba a trossos, com passa de debò.
		for _, chunk := range []string{"Va, deixa'm fer una ullada al projecte.\n\n", "<｜｜DSML｜｜ calls> <｜｜DSML｜｜ invoke name=\"glob\">",
			" <｜｜DSML｜｜ parameter name=\"pattern\" string=\"true\">**/*</｜｜DSML｜｜ parameter>", " </｜｜DSML｜｜ invoke> </｜｜DSML｜｜ calls>"} {
			b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]string{"content": chunk}}}})
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	out, err := New().ChatStream(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "revisa"}}, 0.7, 4096, nil, nil)
	if strings.Contains(out, "DSML") {
		t.Fatalf("el markup no pot arribar a l'usuari: %q", out)
	}
	if out != "Va, deixa'm fer una ullada al projecte." {
		t.Fatalf("text visible=%q", out)
	}
	if err == nil || !strings.Contains(err.Error(), "glob") || !strings.Contains(err.Error(), "no porta eines") {
		t.Fatalf("l'error ha de dir quina eina i per què: %v", err)
	}
}

// Amb eines, un proxy que filtra DSML dins de content en comptes de
// tool_calls no ha de trencar el loop: es recupera com a crida normal.
func TestChatWithToolsRecuperaDSML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
			"message":       map[string]any{"role": "assistant", "content": dsmlReal},
			"finish_reason": "stop"}}})
		w.Write(b)
	}))
	defer srv.Close()

	content, calls, err := New().ChatWithTools(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "revisa"}}, 0.4, 4096, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(content, "DSML") || len(calls) != 1 || calls[0].Function.Name != "glob" {
		t.Fatalf("content=%q calls=%+v", content, calls)
	}
}
