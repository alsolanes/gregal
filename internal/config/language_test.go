package config

import "testing"

func TestPublicLanguageDefaults(t *testing.T) {
	t.Setenv("GREGAL_MEMORY", t.TempDir()+"/memory.md")
	for _, tc := range []struct{ language, want, prompt string }{
		{"", "en", defaultSystemEN},
		{"en", "en", defaultSystemEN},
		{"ca", "ca", defaultSystem},
		{"unsupported", "en", defaultSystemEN},
	} {
		c := &Config{Language: tc.language}
		if c.Lang() != tc.want || c.SystemPrompt() != tc.prompt {
			t.Errorf("language %q: got %q, %q", tc.language, c.Lang(), c.SystemPrompt())
		}
		c.System = "Custom instructions"
		if c.SystemPrompt() != c.System {
			t.Fatal("custom instructions must be preserved")
		}
	}
}
