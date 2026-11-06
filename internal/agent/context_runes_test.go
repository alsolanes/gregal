package agent

import (
	"strings"
	"testing"
	"unicode/utf8"

	"gregal/internal/llm"
)

// Retallar el context no toca mai el system ni la tasca (abans, 600 bytes: el
// que demanava la persona es perdia) i escurça les sortides antigues en
// runes, sense partir cap accent.
func TestContextRetallatKeepsTheTaskAndValidUTF8(t *testing.T) {
	task := strings.Repeat("à", 9000)
	hist := []llm.Message{{Role: "system", Content: "regles"}, {Role: "user", Content: task}}
	for i := 0; i < 30; i++ {
		hist = append(hist, llm.Message{Role: "assistant", Content: "pas"}, llm.Message{Role: "tool", Content: strings.Repeat("é", 500)})
	}
	for vegada := 1; vegada <= 2; vegada++ {
		got := contextRetallat(hist, vegada)
		if got[0].Content != "regles" || got[1].Content != task {
			t.Fatalf("vegada %d: el system i la tasca es conserven sencers", vegada)
		}
		if !utf8.ValidString(got[3].Content) || got[3].Content == hist[3].Content {
			t.Fatalf("vegada %d: sortida antiga no escurçada o UTF-8 trencat: %q", vegada, got[3].Content)
		}
		if got[len(got)-1].Content != hist[len(hist)-1].Content {
			t.Fatalf("vegada %d: la cua queda sencera", vegada)
		}
	}
	if got := contextRetallat(hist, 1); utf8.RuneCountInString(got[3].Content) != 400+1+utf8.RuneCountInString(Retallat) {
		t.Fatalf("retall en runes: %d", utf8.RuneCountInString(got[3].Content))
	}
}
