package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultProviderPresets(t *testing.T) {
	for _, p := range Cataleg() {
		if p.EnvVar != "" {
			t.Setenv(p.EnvVar, "")
		}
	}
	c, _, err := Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, preset := range Cataleg() {
		p, ok := c.Providers[preset.Nom]
		if !ok || p.BaseURL != preset.BaseURL || p.APIKey != "" {
			t.Fatalf("incorrect or credential-bearing preset %s", preset.Nom)
		}
		if !preset.Local && preset.DefaultModel == "" {
			t.Fatalf("model missing for %s", preset.Nom)
		}
	}
	for name, role := range c.Roles {
		if role.Provider != "openai" || role.Model != "gpt-4.1-mini" {
			t.Fatalf("unprepared role %s", name)
		}
	}
	if strings.Contains(DefaultYAML(), "api.example.org") {
		t.Fatal("placeholder endpoint retained")
	}
}
