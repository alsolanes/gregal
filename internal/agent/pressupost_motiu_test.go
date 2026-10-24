package agent

import (
	"strings"
	"testing"

	"gregal/internal/llm"
)

// El motiu d'interrupció arriba cru del client («provider https://…: dial
// tcp: lookup …»). A la conversa de l'usuari hi ha d'arribar una frase
// humana, sense URL ni la paraula «provider https».
func TestMotiuDNSNoFiltraURL(t *testing.T) {
	cru := `provider https://opencode.ai/zen/go/v1: Post "https://opencode.ai/zen/go/v1/chat/completions": dial tcp: lookup opencode.ai: no such host (host not found: check the provider base_url); provider https://opencode.ai/zen/go/v1: Post "https://opencode.ai/zen/go/v1/chat/completions": dial tcp: lookup opencode.ai: no such host (host not found: check the provider base_url)`
	for _, lang := range []string{"ca", "en"} {
		out := ResumDeFeina([]llm.Message{{Role: "user", Content: "hola"}}, cru, lang)
		if strings.Contains(strings.ToLower(out), "https://") {
			t.Errorf("lang %s: la URL del proveïdor ha filtrat al resum: %s", lang, out)
		}
		if strings.Contains(out, "dial tcp") || strings.Contains(out, "provider https") {
			t.Errorf("lang %s: error cru visible: %s", lang, out)
		}
		if !strings.Contains(out, "proveïdor") && !strings.Contains(out, "provider") {
			t.Errorf("lang %s: el resum no diu res del proveïdor: %s", lang, out)
		}
	}
}

func TestMotiuAmableCadaErrorTeLaSevaFrase(t *testing.T) {
	casos := map[string]string{
		"dial tcp: lookup x.invalid: no such host": "DNS",
		"provider http://localhost:8089: connection refused": "refusada",
		"context deadline exceeded":                         "temps",
		"provider https://x: HTTP 429: massa peticions":    "tràfec",
		"provider https://x: HTTP 500: boom":                "incidència",
		"provider https://x: HTTP 401: unauthorized":        "clau",
	}
	for cru := range casos {
		for _, lang := range []string{"ca", "en"} {
			out := ResumDeFeina(nil, cru, lang)
			if strings.Contains(strings.ToLower(out), "http://") || strings.Contains(strings.ToLower(out), "https://") {
				t.Errorf("lang %s (%s): URL filtrada: %s", lang, cru, out)
			}
		}
	}
	// El motiu buit o «passos» no ha d'afegir res:
	if strings.Contains(ResumDeFeina(nil, "", "ca"), "(") {
		t.Error("el motiu buit afegeix un parèntesi")
	}
}
