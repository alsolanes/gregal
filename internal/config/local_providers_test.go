package config

import (
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"gregal/internal/llm"
)

// Els presets locals parlen el seu protocol: Ollama natiu i llama.cpp amb slots.
func TestPresetsLocalsAmbTipus(t *testing.T) {
	c, _, err := Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for nom, want := range map[string]string{"ollama": "ollama", "llamacpp": "llamacpp", "lmstudio": ""} {
		pc, _ := ProveidorPerNom(nom)
		if pc.Type != want || c.Providers[nom].Type != want {
			t.Fatalf("%s: catàleg %q, config %q, volia %q", nom, pc.Type, c.Providers[nom].Type, want)
		}
	}
}

func TestOpcionsLocalsDelConfig(t *testing.T) {
	raw := `providers:
  ol:
    type: ollama
    base_url: http://ollama.example.test:11434/v1
    num_ctx: 65536
    keep_alive: "-1"
  lc:
    type: llama.cpp
    base_url: http://llama.example.test:8080/v1
    slot: 2
    side_slot: -1
roles:
  chat: {provider: ol, model: qwen3}
  code: {provider: lc, model: q}
  reviewer: {provider: ol, model: qwen3}
`
	c := &Config{}
	if err := yaml.Unmarshal([]byte(raw), c); err != nil {
		t.Fatal(err)
	}
	defer llm.RegisterProviderType("http://ollama.example.test:11434/v1", "")
	defer llm.RegisterProviderType("http://llama.example.test:8080/v1", "")
	defer llm.RegisterLocalOptions("http://ollama.example.test:11434/v1", llm.LocalOptions{})
	c.RegisterProviderTypes()
	if err := c.ValidaTipusProviders(); err != nil {
		t.Fatal(err)
	}
	p := c.Providers["ol"]
	if p.NumCtx != 65536 || p.KeepAlive != "-1" {
		t.Fatalf("ol=%+v", p)
	}
	if l := c.Providers["lc"]; l.Slot == nil || *l.Slot != 2 || l.SideSlot == nil || *l.SideSlot != -1 {
		t.Fatalf("lc=%+v", l)
	}
	if llm.ProviderTypeFor(p.BaseURL) != llm.ProviderOllama || llm.ProviderTypeFor("http://llama.example.test:8080/v1") != llm.ProviderLlamaCpp {
		t.Fatal("tipus locals no registrats")
	}
	if ka := llm.OllamaKeepAliveFor(p.BaseURL); ka != -1 {
		t.Fatalf("keep_alive=%v", ka)
	}
}
