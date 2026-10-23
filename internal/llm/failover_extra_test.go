package llm

import (
	"errors"
	"testing"
)

// El 400 buit de zen ({"model":"x"} sense error) en un torn amb històric
// gran és context excedit: s'ha de poder detectar per recuperar.
func TestBuit400(t *testing.T) {
	casos := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New(`provider https://opencode.ai/zen/go/v1: HTTP 400: {"model":"deepseek-v4.1-flash"}`), true},
		{errors.New(`provider x: HTTP 400: `), true},
		{errors.New(`provider x: HTTP 400: {"error":{"message":"up"}}`), false},
		{errors.New(`provider x: HTTP 400: {"message":"up"}`), false},
		{errors.New(`provider x: HTTP 500: {"model":"x"}`), false},
		{errors.New(`provider x: HTTP 400: context molt gran`), false},
	}
	for _, tc := range casos {
		if got := Buit400(tc.err); got != tc.want {
			t.Errorf("Buit400(%v)=%v volia %v", tc.err, got, tc.want)
		}
	}
}

// Raonaments antics fora, l'últim es manté: zen només exigeix el de
// l'últim pas; els anteriors inflen el context.
func TestSanitizeHistRetallaRaonamentsAntics(t *testing.T) {
	hist := []Message{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b", ReasoningContent: "pensament vell"},
		{Role: "user", Content: "c"},
		{Role: "assistant", Content: "d", ReasoningContent: "pensament nou"},
	}
	net := SanitizeHist(hist)
	if net[1].ReasoningContent != "" {
		t.Errorf("el raonament antic s'havia de treure: %q", net[1].ReasoningContent)
	}
	if net[3].ReasoningContent != "pensament nou" {
		t.Errorf("l'últim raonament s'ha de mantenir: %q", net[3].ReasoningContent)
	}
	// l'original no es toca
	if hist[1].ReasoningContent != "pensament vell" {
		t.Errorf("l'original s'ha modificat")
	}
}
