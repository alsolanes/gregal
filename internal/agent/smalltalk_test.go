package agent

import (
	"strings"
	"testing"
)

func TestSmallTalkSalutacions(t *testing.T) {
	casos := map[string]string{
		"hola": "Hola", "  Hola! ": "Hola", "HOLA": "Hola",
		"bon dia": "Hola", "bona tarda": "Hola", "què tal?": "Hola",
		"gràcies": "De res", "merci": "De res",
		"adeu": "Fins ara", "fins ara": "Fins ara",
		"ok": "Perfecte", "vale": "Perfecte", "d'acord": "Perfecte",
		"hello": "Hi!", "thanks": "welcome", "bye": "See you",
		"buenas": "¡Hola", "gracias": "nada",
		"adios": "luego",
	}
	for text, want := range casos {
		reply, ok := SmallTalk(text)
		if !ok {
			t.Fatalf("%q hauria de ser cortesia", text)
		}
		if !strings.Contains(reply, want) {
			t.Fatalf("%q → %q (esperava %q)", text, reply, want)
		}
	}
}

func TestSmallTalkNoTocaFeina(t *testing.T) {
	// Tot això TÉ feina a fer: ha d'anar a l'agent, mai a la via ràpida.
	feina := []string{
		"", "   ", "hola, resumeix el document", "hola i adeu",
		"hol", "holaaa", "fes una web amb three.js",
		"què diu el document?", "llegeix main.go",
		"gràcies per res, ara esborra-ho tot",
		"ok, executa el pla", "bon dia, com puc compilar?",
		strings.Repeat("hola ", 10),
		"hola\nadeu",
	}
	for _, text := range feina {
		if _, ok := SmallTalk(text); ok {
			t.Fatalf("%q té feina: no pot ser cortesia", text)
		}
	}
}
