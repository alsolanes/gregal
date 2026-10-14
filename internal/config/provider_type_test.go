package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/llm"
)

func cfgAmbProviders(provs map[string]Provider, rolProv string) *Config {
	roles := map[string]Role{}
	for _, n := range []string{"chat", "code", "reviewer"} {
		roles[n] = Role{Provider: rolProv, Model: "m"}
	}
	return &Config{Providers: provs, Roles: roles}
}

func TestProviderTypeValidationAndRegistration(t *testing.T) {
	c := cfgAmbProviders(map[string]Provider{
		"claude":     {Type: "anthropic", BaseURL: "https://anthropic.example.test/v1"},
		"openai":     {BaseURL: "https://openai.example.test/v1"},
		"openai-rsp": {Type: "openai-responses", BaseURL: "https://openai.example.test/v1/"},
	}, "openai")
	if err := c.ValidaTipusProviders(); err != nil {
		t.Fatal(err)
	}
	defer llm.RegisterProviderType("https://anthropic.example.test/v1", "")
	defer llm.RegisterProviderType("https://openai.example.test/v1", "")
	c.RegisterProviderTypes()
	if llm.ProviderTypeFor("https://anthropic.example.test/v1") != llm.ProviderAnthropic {
		t.Fatal("anthropic not registered")
	}
	// La URL compartida la decideix el provider que fan servir els rols.
	if llm.ProviderTypeFor("https://openai.example.test/v1") != "" {
		t.Fatal("shared URL must follow the provider used by roles")
	}
	for n, r := range c.Roles {
		r.Provider = "openai-rsp"
		c.Roles[n] = r
	}
	c.RegisterProviderTypes()
	if llm.ProviderTypeFor("https://openai.example.test/v1") != llm.ProviderOpenAIResponses {
		t.Fatal("responses provider should win when roles use it")
	}
	r := c.Roles["chat"]
	r.FallbackProvider, r.FallbackModel = "openai", "m"
	c.Roles["chat"] = r
	if err := c.ValidaTipusProviders(); err == nil || !strings.Contains(err.Error(), "base_url") {
		t.Fatalf("conflict not detected: %v", err)
	}
	bad := cfgAmbProviders(map[string]Provider{"x": {Type: "gemini", BaseURL: "https://x.test"}}, "x")
	if err := bad.Validate(); err == nil || !strings.Contains(err.Error(), "gemini") {
		t.Fatalf("unknown type accepted: %v", err)
	}
}

func TestProviderTypeLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `providers:
  claude:
    type: anthropic
    base_url: https://roundtrip.anthropic.test/v1
    api_key: ${GREGAL_TEST_NO_SUCH_KEY}
roles:
  chat: {provider: claude, model: claude-opus-5-5}
  code: {provider: claude, model: claude-opus-5-5}
  reviewer: {provider: claude, model: claude-opus-5-5}
verify:
  mode: both
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	defer llm.RegisterProviderType("https://roundtrip.anthropic.test/v1", "")
	c, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Providers["claude"].Type != "anthropic" || llm.ProviderTypeFor("https://roundtrip.anthropic.test/v1") != llm.ProviderAnthropic {
		t.Fatal("type not loaded/registered")
	}
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "type: anthropic") {
		t.Fatalf("type not persisted:\n%s", raw)
	}
}

func TestCatalogNativePresets(t *testing.T) {
	a, ok := ProveidorPerNom("anthropic")
	if !ok || a.Type != "anthropic" || a.EnvVar != "ANTHROPIC_API_KEY" || a.DefaultModel == "" {
		t.Fatalf("anthropic preset %+v", a)
	}
	r, ok := ProveidorPerNom("openai-responses")
	if !ok || r.Type != "openai-responses" || r.EnvVar != "OPENAI_API_KEY" {
		t.Fatalf("responses preset %+v", r)
	}
}
