package web

import (
	"strings"
	"testing"

	"gregal/internal/config"
)

func TestKeylessHint(t *testing.T) {
	s := &Server{}
	raw := `agent: error: provider https://api.openai.com/v1: HTTP 401: { "error": { "message": "You didn't provide an API key." } }`
	remote := config.Provider{BaseURL: "https://api.openai.com/v1"}

	got := s.keylessHint(remote, "openai", raw)
	if got == raw || !strings.Contains(got, "openai") || strings.Contains(got, "{") || !strings.Contains(got, "no API key") {
		t.Fatalf("a remote provider without a key should get a plain hint, got %q", got)
	}
	// Amb clau, el proveïdor l'ha rebutjada: també en clar, però diferent.
	withKey := remote
	withKey.APIKey = "sk-test"
	if got := s.keylessHint(withKey, "openai", raw); got == raw || !strings.Contains(got, "does not accept") {
		t.Fatalf("a rejected key should say so, got %q", got)
	}
	// Un servidor local no necessita clau: l'error és un altre.
	local := config.Provider{BaseURL: "http://localhost:11434/v1"}
	if got := s.keylessHint(local, "ollama", raw); got != raw {
		t.Fatalf("local providers keep the raw error, got %q", got)
	}
	// Errors que no són d'autenticació no es toquen.
	other := "provider https://api.openai.com/v1: HTTP 500: boom"
	if got := s.keylessHint(remote, "openai", other); got != other {
		t.Fatalf("non-auth errors must stay, got %q", got)
	}
	// Amb clau, parlar d'una «api key» (d'una eina, posem) no és un rebuig.
	tool := "search tool: missing api key for the web search"
	if got := s.keylessHint(withKey, "openai", tool); got != tool {
		t.Fatalf("with a key, a loose mention of an api key must stay, got %q", got)
	}
}
