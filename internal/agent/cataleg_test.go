package agent

import (
	"testing"

	"gregal/internal/catalog"
	"gregal/internal/config"
)

// Amb catàleg, un model que el proveïdor no declara a /models pren la
// finestra i el preu de models.dev; el config continua manant.
func TestFinestraIPreuDelCataleg(t *testing.T) {
	d, err := catalog.Parse([]byte(`{"opencode-go": {"api": "https://opencode.ai/inference/go/openai/v1", "models": {
	  "deepseek-v4.1-flash": {"tool_call": true, "limit": {"context": 1000000, "output": 384000}, "cost": {"input": 0.15, "output": 0.6}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	catalog.Set(d)
	t.Cleanup(catalog.Reset)
	ResetWindows()
	cfg := &config.Config{
		Providers: map[string]config.Provider{"zen": {BaseURL: "https://opencode.ai/zen/go/v1"}, "local": {BaseURL: "http://localhost:8080/v1"}},
		Roles: map[string]config.Role{
			"code":  {Provider: "zen", Model: "deepseek-v4.1-flash"},
			"chat":  {Provider: "local", Model: "deepseek-v4.1-flash"},
			"think": {Provider: "zen", Model: "deepseek-v4.1-flash", ContextWindow: 64000},
		},
	}
	if n, src := WindowSource(cfg, cfg.Roles["code"]); n != 1000000 || src != "cataleg" {
		t.Fatalf("code: %d %s", n, src)
	}
	if _, src := WindowSource(cfg, cfg.Roles["chat"]); src == "cataleg" {
		t.Fatal("un servidor local té la finestra que s'hi ha configurat, no la del catàleg")
	}
	if n, src := WindowSource(cfg, cfg.Roles["think"]); n != 64000 || src != "config" {
		t.Fatalf("context_window del rol mana: %d %s", n, src)
	}
	SetPrices(nil)
	PreusDelCataleg(cfg)
	usd, ok := CostUSD("zen", "deepseek-v4.1-flash", 1_000_000, 1_000_000)
	if !ok || usd < 0.749 || usd > 0.751 {
		t.Fatalf("cost: %v %v", usd, ok)
	}
	// Un preu del config no es trepitja.
	SetPrices(map[string]Price{"deepseek-v4.1-flash": {In: 1, Out: 1}})
	PreusDelCataleg(cfg)
	if usd, _ = CostUSD("zen", "deepseek-v4.1-flash", 1_000_000, 0); usd != 1 {
		t.Fatalf("el config mana: %v", usd)
	}
	SetPrices(nil)
}
