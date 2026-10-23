package llm

import (
	"strings"
	"testing"
)

// tc és una crida d'eina de test amb el Function inline del ToolCall real.
func tc(id, name, args string) ToolCall {
	c := ToolCall{ID: id, Type: "function"}
	c.Function.Name = name
	c.Function.Arguments = args
	return c
}

// El cas real: un assistant amb tool_calls la resposta del qual mai no
// va arribar (torn mort per OOM/cancel·lació) enverina la sessió —
// l'endpoint estricte el rebutja amb 400 a cada missatge posterior.
func TestSanitizeHistAplanaCridesPenjants(t *testing.T) {
	broket := []Message{
		{Role: "user", Content: "escriu hola a /tmp/x.txt"},
		{Role: "assistant", ToolCalls: []ToolCall{
			tc("call_1", "write", `{"path":"/tmp/x.txt","content":"hola"}`),
		}},
		{Role: "user", Content: "Segueix"},
	}
	net := SanitizeHist(broket)
	for i, m := range net {
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			t.Fatalf("missatge %d: assistant amb tool_calls ha quedat aplanat", i)
		}
		if m.Role == "tool" {
			t.Fatalf("missatge %d: resposta d'eina sense crida", i)
		}
	}
	if len(net) != 3 {
		t.Fatalf("llargada inesperada: %d", len(net))
	}
	if !strings.Contains(net[1].Content, "es va quedar sense resposta") {
		t.Errorf("la nota de la crida penjant falta: %q", net[1].Content)
	}
	if !strings.Contains(net[1].Content, "write(") {
		t.Errorf("el nom de l'eina falta: %q", net[1].Content)
	}
}

// Un històric sa es torna idèntic.
func TestSanitizeHistDeixaSaElSa(t *testing.T) {
	sa := []Message{
		{Role: "user", Content: "escriu hola"},
		{Role: "assistant", ToolCalls: []ToolCall{
			tc("call_1", "write", `{"path":"x","content":"hola"}`),
		}},
		{Role: "tool", ToolCallID: "call_1", Content: "5 bytes"},
		{Role: "assistant", Content: "fet"},
	}
	net := SanitizeHist(sa)
	if len(net) != len(sa) {
		t.Fatalf("llargada canviada: %d vs %d", len(net), len(sa))
	}
	for i := range sa {
		if net[i].Role != sa[i].Role || net[i].Content != sa[i].Content {
			t.Errorf("missatge %d canviat sense necessitat", i)
		}
	}
}

// Diverses crides on només una té resposta: l'assistant s'aplan a i la
// resposta que sí que hi és no queda penjada d'una crida que ja no existeix.
func TestSanitizeHistCridaParcial(t *testing.T) {
	broket := []Message{
		{Role: "user", Content: "fes dues coses"},
		{Role: "assistant", ToolCalls: []ToolCall{
			tc("a", "read", `{"path":"1"}`),
			tc("b", "read", `{"path":"2"}`),
		}},
		{Role: "tool", ToolCallID: "a", Content: "contingut 1"},
	}
	net := SanitizeHist(broket)
	for i, m := range net {
		if m.Role == "tool" || (m.Role == "assistant" && len(m.ToolCalls) > 0) {
			t.Errorf("missatge %d (%s) hauria de ser aplanat: %+v", i, m.Role, m)
		}
	}
	if !strings.Contains(net[1].Content, "es va quedar sense resposta") {
		t.Errorf("la nota falta: %q", net[1].Content)
	}
}
