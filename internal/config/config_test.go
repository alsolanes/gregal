package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const goodYAML = `
providers:
  local: {base_url: http://localhost:8089/v1, api_key: ""}
  cloud: {base_url: https://example.test/v1, api_key: secret}
roles:
  chat: {provider: local, model: m1, temperature: 0.7, max_tokens: 10}
  code: {provider: local, model: m1, temperature: 0.4, max_tokens: 10}
  reviewer: {provider: cloud, model: m2, temperature: 0.2, max_tokens: 10}
verify: {mode: both}
`

func TestLoadGood(t *testing.T) {
	c, _, err := Load(writeTemp(t, goodYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Roles["chat"].Model != "m1" {
		t.Fatalf("rol chat inesperat: %+v", c.Roles["chat"])
	}
	if got := c.RoleNames(); got != "chat, code, reviewer" {
		t.Fatalf("RoleNames=%q", got)
	}
}

func TestLoadPromptCacheOptions(t *testing.T) {
	c, _, err := Load(writeTemp(t, `
providers:
  local: {base_url: http://localhost:8089/v1, api_key: "", prompt_cache_key: gregal, prompt_cache_retention: 24h}
  cloud: {base_url: https://example.test/v1, api_key: secret}
roles:
  chat: {provider: local, model: m1}
  code: {provider: local, model: m1}
  reviewer: {provider: cloud, model: m2}
verify: {mode: both}
`))
	if err != nil {
		t.Fatal(err)
	}
	p := c.Providers["local"]
	if p.PromptCacheKey != "gregal" || p.PromptCacheRetention != "24h" {
		t.Fatalf("opcions de cache=%+v", p)
	}
}

// El ratolí ve activat (roda) i `mouse: off` el desactiva (selecció
// nativa); la tria sobreviu reinicis perquè /mouse la desa.
func TestMousePersistencia(t *testing.T) {
	c, _, err := Load(writeTemp(t, goodYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !c.MouseOn() {
		t.Fatal("per defecte el ratolí ha d'estar activat")
	}
	c.Mouse = "off"
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := c.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	c2, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load 2: %v", err)
	}
	if c2.MouseOn() {
		t.Fatal("off s'ha de conservar en desar i carregar")
	}
	c2.Mouse = "bogus"
	if err := c2.Validate(); err == nil {
		t.Fatal("mouse invàlid ha de fallar la validació")
	}
}

func TestInitialRoleFollowsMode(t *testing.T) {
	c := &Config{Mode: "code", Roles: map[string]Role{
		"chat": {Model: "chat"}, "think": {Model: "think"}, "code": {Model: "code"},
	}}
	if got := c.InitialRole(); got != "code" {
		t.Fatalf("code inicial = %q", got)
	}
	c.Mode = "inspect"
	if got := c.InitialRole(); got != "think" {
		t.Fatalf("inspect inicial = %q", got)
	}
	delete(c.Roles, "code")
	delete(c.Roles, "think")
	if got := c.InitialRole(); got != "chat" {
		t.Fatalf("fallback inicial = %q", got)
	}
}

func TestBadVerifyMode(t *testing.T) {
	bad := goodYAML + ""
	_ = bad
	y := `
providers:
  local: {base_url: http://x/v1, api_key: ""}
roles:
  chat: {provider: local, model: m1, temperature: 0.7, max_tokens: 10}
  code: {provider: local, model: m1, temperature: 0.4, max_tokens: 10}
  reviewer: {provider: local, model: m1, temperature: 0.2, max_tokens: 10}
verify: {mode: sempre}
`
	if _, _, err := Load(writeTemp(t, y)); err == nil {
		t.Fatal("esperava error amb verify.mode invàlid")
	}
}

func TestMissingRole(t *testing.T) {
	y := `
providers:
  local: {base_url: http://x/v1, api_key: ""}
roles:
  chat: {provider: local, model: m1, temperature: 0.7, max_tokens: 10}
verify: {mode: manual}
`
	if _, _, err := Load(writeTemp(t, y)); err == nil {
		t.Fatal("esperava error per rol code inexistent")
	}
}

func TestAgentDefault(t *testing.T) {
	c, _, err := Load(writeTemp(t, goodYAML))
	if err != nil {
		t.Fatal(err)
	}
	// 40, no 10: una tasca de codi real (llegir, fer grep, editar, verificar)
	// es menjava deu passos i el torn s'acabava a mitges.
	if c.Agent.MaxSteps != 40 {
		t.Fatalf("MaxSteps=%d", c.Agent.MaxSteps)
	}
}

func TestHooksParse(t *testing.T) {
	c, _, err := Load(writeTemp(t, goodYAML+"\nhooks:\n  post_edit: \"gofmt -w {file}\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Hooks.PostEdit != "gofmt -w {file}" {
		t.Fatalf("PostEdit=%q", c.Hooks.PostEdit)
	}
	c2, _, err := Load(writeTemp(t, goodYAML))
	if err != nil {
		t.Fatal(err)
	}
	if c2.Hooks.PostEdit != "" {
		t.Fatalf("per defecte desactivat, trobat %q", c2.Hooks.PostEdit)
	}
}

func TestCostParse(t *testing.T) {
	c, _, err := Load(writeTemp(t, goodYAML+"\ncost:\n  kimi-k2.7-code: {in: 0.6, out: 2.5}\n"))
	if err != nil {
		t.Fatal(err)
	}
	p, ok := c.Cost["kimi-k2.7-code"]
	if !ok || p.In != 0.6 || p.Out != 2.5 {
		t.Fatalf("cost=%v", c.Cost)
	}
}

func TestBadPermissions(t *testing.T) {
	y := `
providers:
  local: {base_url: http://x/v1, api_key: ""}
roles:
  chat: {provider: local, model: m1, temperature: 0.7, max_tokens: 10}
  code: {provider: local, model: m1, temperature: 0.4, max_tokens: 10}
  reviewer: {provider: local, model: m1, temperature: 0.2, max_tokens: 10}
verify: {mode: manual}
agent: {max_steps: 10}
permissions: {tools: {evil: allow}}
`
	if _, _, err := Load(writeTemp(t, y)); err == nil {
		t.Fatal("esperava error amb eina desconeguda")
	}
	y2 := `
providers:
  local: {base_url: http://x/v1, api_key: ""}
roles:
  chat: {provider: local, model: m1, temperature: 0.7, max_tokens: 10}
  code: {provider: local, model: m1, temperature: 0.4, max_tokens: 10}
  reviewer: {provider: local, model: m1, temperature: 0.2, max_tokens: 10}
verify: {mode: manual}
agent: {max_steps: 10}
permissions: {tools: {write: maybe}}
`
	if _, _, err := Load(writeTemp(t, y2)); err == nil {
		t.Fatal("esperava error amb valor invàlid")
	}
	y3 := goodYAML + `
permissions:
  tools:
    patch: allow
    delegate: allow
    read_image: allow
`
	if _, _, err := Load(writeTemp(t, y3)); err != nil {
		t.Fatalf("les eines natives del loop haurien de ser configurables: %v", err)
	}
}

func TestFallbackTarget(t *testing.T) {
	c, _, err := Load(writeTemp(t, goodYAML))
	if err != nil {
		t.Fatal(err)
	}
	// Sense fallback al goodYAML → nil.
	if fb := c.FallbackTarget(c.Roles["code"]); fb != nil {
		t.Fatalf("sense config ha de ser nil: %+v", fb)
	}
	r := Role{Provider: "local", Model: "m", FallbackProvider: "cloud", FallbackModel: "fb"}
	c.Providers = map[string]Provider{
		"local": {BaseURL: "http://l/v1"},
		"cloud": {BaseURL: "http://c/v1", APIKey: "k"},
	}
	fb := c.FallbackTarget(r)
	if fb == nil || fb.BaseURL != "http://c/v1" || fb.APIKey != "k" || fb.Model != "fb" {
		t.Fatalf("fb=%+v", fb)
	}
	// Provider inexistent → nil (sense failover, sense error).
	r.FallbackProvider = "noexisteix"
	if fb := c.FallbackTarget(r); fb != nil {
		t.Fatalf("provider inexistent ha de ser nil: %+v", fb)
	}
	prim := c.PrimTarget(c.Providers["local"], r)
	if prim.BaseURL != "http://l/v1" || prim.Model != "m" {
		t.Fatalf("prim=%+v", prim)
	}
}

// SetRawKey + Save + Load: la clau persisteix (camí de `gregal config`).
func TestRawKeyPersisteix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cfg.Providers["p1"] = Provider{BaseURL: "http://127.0.0.1:9"}
	cfg.SetRawKey("p1", "sk-persist-1")
	if err := cfg.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	cfg2, _, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if cfg2.Providers["p1"].APIKey != "sk-persist-1" {
		t.Fatalf("clau perduda: %q", cfg2.Providers["p1"].MaskedKey())
	}
	// Sense SetRawKey, el literal no es congela al disc.
	cfg2.Providers["p1"] = Provider{BaseURL: "http://127.0.0.1:9", APIKey: "sk-memoria"}
	if err := cfg2.Save(path); err != nil {
		t.Fatalf("save2: %v", err)
	}
	cfg3, _, _ := Load(path)
	if cfg3.Providers["p1"].APIKey != "sk-persist-1" {
		t.Fatalf("el literal hauria de quedar fora del disc: %q", cfg3.Providers["p1"].APIKey)
	}
}
