package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
)

func TestCLIMessageCatalogHasEnglishAndCatalan(t *testing.T) {
	for key, message := range cliMessages {
		if message.en == "" || message.ca == "" {
			t.Errorf("message %q must define both languages", key)
		}
	}
	if got := cliText("en", "config.missing", "settings.yaml"); got != "does not exist (to create one: gregal init --config=settings.yaml)" {
		t.Fatalf("English message = %q", got)
	}
	if got := cliText("ca", "config.missing", "settings.yaml"); got != "no existeix (per crear-ne un: gregal init --config=settings.yaml)" {
		t.Fatalf("Catalan message = %q", got)
	}
	if got := cliText("unsupported", "config.key_label"); got != "key" {
		t.Fatalf("unsupported language should default to English, got %q", got)
	}
}

func TestConfigLanguage(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, contents, want string
	}{
		{"catalan", "lang: ca\nroles: invalid\n", "ca"},
		{"english", "lang: en\n", "en"},
		{"missing", "roles: {}\n", "en"},
		{"unsupported", "lang: fr\n", "en"},
		{"malformed", "lang: [\n", "en"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".yaml")
			if err := os.WriteFile(path, []byte(tc.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if got := configLanguage(path); got != tc.want {
				t.Fatalf("configLanguage() = %q, want %q", got, tc.want)
			}
		})
	}
	if got := configLanguage(filepath.Join(dir, "missing.yaml")); got != "en" {
		t.Fatalf("missing config language = %q, want en", got)
	}
}

func TestConfigListUsesConfiguredLanguage(t *testing.T) {
	for _, tc := range []struct{ language, label string }{
		{"en", "key:"},
		{"ca", "clau:"},
	} {
		t.Run(tc.language, func(t *testing.T) {
			cfg := &config.Config{
				Language: tc.language,
				Providers: map[string]config.Provider{
					"local": {BaseURL: "http://localhost:8089/v1"},
				},
			}
			got := configList(cfg)
			if !strings.Contains(got, tc.label) {
				t.Fatalf("configList() = %q, want label %q", got, tc.label)
			}
		})
	}
}
