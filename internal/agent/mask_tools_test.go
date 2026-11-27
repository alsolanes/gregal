package agent

import (
	"strings"
	"testing"

	"gregal/internal/llm"
)

func TestMaskOldToolOutputs_KeepsRecentAndSmallIntact(t *testing.T) {
	big := strings.Repeat("línia de sortida llarga per provar l'emmascarament\n", 100)
	small := "resultat curt"

	h := []llm.Message{
		{Role: "user", Content: "fes alguna cosa"},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "1"}}},
		{Role: "tool", ToolCallID: "1", Content: big},   // antiga i gran -> emmascarada
		{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "2"}}},
		{Role: "tool", ToolCallID: "2", Content: small}, // antiga però curta -> intacta
		{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "3"}, {ID: "4"}, {ID: "5"}, {ID: "6"}}},
		{Role: "tool", ToolCallID: "3", Content: big},   // dins les 4 recents -> intacta
		{Role: "tool", ToolCallID: "4", Content: big},   // dins les 4 recents -> intacta
		{Role: "tool", ToolCallID: "5", Content: big},   // dins les 4 recents -> intacta
		{Role: "tool", ToolCallID: "6", Content: big},   // dins les 4 recents -> intacta
		{Role: "assistant", Content: "fet"},
	}

	got := MaskOldToolOutputs(h)
	if &got[0] == &h[0] {
		t.Fatal("esperava una còpia de l'historial quan s'emmascara algun missatge")
	}
	if !strings.Contains(got[2].Content, maskArchiveMarker) {
		t.Fatalf("esperava marcador d'arxiu a la sortida antiga gran, tinc: %q", got[2].Content)
	}
	if len(got[2].Content) >= len(big) {
		t.Fatalf("la sortida emmascarada ha de ser molt més curta (%d vs %d)", len(got[2].Content), len(big))
	}
	if got[4].Content != small {
		t.Fatalf("la sortida curta no s'ha de tocar: %q", got[4].Content)
	}
	for _, idx := range []int{6, 7, 8, 9} {
		if got[idx].Content != big {
			t.Fatalf("la sortida recent [%d] s'ha de mantenir intacta", idx)
		}
	}
	// Original intacte
	if h[2].Content != big {
		t.Fatal("MaskOldToolOutputs no ha de mutar el slice original")
	}
}

func TestMaskOldToolOutputs_NoCopyWhenNothingMasked(t *testing.T) {
	h := []llm.Message{
		{Role: "user", Content: "hola"},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "1"}}},
		{Role: "tool", ToolCallID: "1", Content: "curt"},
	}
	got := MaskOldToolOutputs(h)
	if &got[0] != &h[0] {
		t.Fatal("no cal copiar el slice si no s'emmascara res")
	}
}
