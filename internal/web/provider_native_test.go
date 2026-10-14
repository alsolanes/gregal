package web

import (
	"os"
	"strings"
	"testing"

	"gregal/internal/llm"
)

// El preset d'Anthropic deixa el provider amb type: anthropic, el registra
// al client (el cable natiu) i el desa al config sense la clau.
func TestAnthropicPresetSetsNativeType(t *testing.T) {
	s := goalTestServer(t)
	t.Setenv("ANTHROPIC_API_KEY", "synthetic-anthropic-test-only")
	defer llm.RegisterProviderType("https://api.anthropic.com/v1", "")
	if _, err := s.providerAction("preset", "anthropic", "", "", "", "", "", false, 0, 0); err != nil {
		t.Fatal(err)
	}
	p := s.cfg.Providers["anthropic"]
	if p.Type != "anthropic" || p.BaseURL != "https://api.anthropic.com/v1" {
		t.Fatalf("provider %+v", p)
	}
	if llm.ProviderTypeFor(p.BaseURL) != llm.ProviderAnthropic {
		t.Fatal("native type not registered after save")
	}
	raw, err := os.ReadFile(s.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "type: anthropic") || strings.Contains(string(raw), "synthetic-anthropic-test-only") {
		t.Fatal("type/key persistence regression")
	}
}
