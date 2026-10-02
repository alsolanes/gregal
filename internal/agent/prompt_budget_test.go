package agent

import (
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// TestPressupostPromptCode verifica que el system prompt REAL del rol code
// (base per defecte + fets d'entorn + regles de code) no superi el pressupost
// de 2500 tokens estimats. Si el prompt creix per sobre, el test obliga a
// aprimar-lo abans de fusionar.
func TestPressupostPromptCode(t *testing.T) {
	cfg := &config.Config{}
	sys := PromptFor(cfg.SystemPrompt(), ModeCode)
	n := llm.EstimateTokens([]llm.Message{{Role: "system", Content: sys}})
	t.Logf("system prompt code: %d caràcters, %d tokens estimats", len(sys), n)
	if n >= 2500 {
		t.Fatalf("system prompt code massa gros: %d tokens estimats (pressupost < 2500)", n)
	}
}
