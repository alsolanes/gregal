package config

import (
	"strings"
	"testing"
)

func TestPublicLanguageDefaults(t *testing.T) {
	t.Setenv("GREGAL_MEMORY", t.TempDir()+"/memory.md")
	for _, tc := range []struct{ language, want, prompt string }{
		{"", "en", defaultSystemEN},
		{"en", "en", defaultSystemEN},
		{"ca", "ca", defaultSystem},
		{"unsupported", "en", defaultSystemEN},
	} {
		c := &Config{Language: tc.language}
		want := tc.prompt + "\n\n" + LanguageDirective(tc.want)
		if c.Lang() != tc.want || c.SystemPrompt() != want {
			t.Errorf("language %q: got %q, %q", tc.language, c.Lang(), c.SystemPrompt())
		}
		c.System = "Custom instructions"
		if c.SystemPrompt() != c.System+"\n\n"+LanguageDirective(tc.want) {
			t.Fatal("custom instructions must be preserved")
		}
	}
}

// The language directive closes the prompt and can be split off again, so
// callers that append blocks keep it last.
func TestLanguageDirectiveSplit(t *testing.T) {
	t.Setenv("GREGAL_MEMORY", t.TempDir()+"/memory.md")
	for _, lang := range []string{"en", "ca"} {
		c := &Config{Language: lang}
		body, d := SplitLanguageDirective(c.SystemPrompt())
		if d != LanguageDirective(lang) || strings.Contains(body, "LANGUAGE:") {
			t.Errorf("%s: split = %q / %q", lang, body, d)
		}
		if LangOfPrompt(c.SystemPrompt()) != lang {
			t.Errorf("%s: LangOfPrompt = %q", lang, LangOfPrompt(c.SystemPrompt()))
		}
	}
	if body, d := SplitLanguageDirective("plain"); body != "plain" || d != "" {
		t.Errorf("no directive: %q / %q", body, d)
	}
	if !strings.Contains(LanguageDirective("en"), "English") || !strings.Contains(LanguageDirective("ca"), "català") {
		t.Error("each directive names its language")
	}
}
