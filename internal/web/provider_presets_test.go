package web

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"gregal/internal/config"
)

func TestProviderPresetAssignsRolesWithoutPersistingSecrets(t *testing.T) {
	s := goalTestServer(t)
	t.Setenv("ZEN_API_KEY", "synthetic-preset-test-only")
	_, err := s.providerAction("preset", "zen", "", "", "", "", "", false, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for name, r := range s.cfg.Roles {
		if r.Provider != "zen" || r.Model != "glm-5.3-flash" || r.FallbackProvider != "" {
			t.Fatalf("incorrect preset for %s", name)
		}
	}
	if s.cfg.Providers["zen"].APIKey != "synthetic-preset-test-only" {
		t.Fatal("environment key not active")
	}
	bytes, err := os.ReadFile(s.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(bytes), "synthetic-preset-test-only") || !strings.Contains(string(bytes), "${ZEN_API_KEY}") {
		t.Fatal("key persistence regression")
	}
}

func TestHostedPresetWithoutKeyIsNotProbed(t *testing.T) {
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"openai": {BaseURL: "https://api.openai.com/v1"}}
	w := httptest.NewRecorder()
	s.handleModels(w, httptest.NewRequest("GET", "/api/models", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "API key required") {
		t.Fatal("missing credential state")
	}
}

func TestProviderPresetPreservesCustomEndpointsAndKeys(t *testing.T) {
	s := goalTestServer(t)
	s.cfg.Providers["zen"] = config.Provider{BaseURL: "https://custom.example.org/v1", APIKey: "synthetic-key"}
	_, err := s.providerAction("preset", "zen", "", "", "", "", "", false, 0, 0)
	if err == nil || s.cfg.Providers["zen"].BaseURL != "https://custom.example.org/v1" {
		t.Fatal("custom endpoint overwritten")
	}
	p, _ := config.ProveidorPerNom("zen")
	s.cfg.Providers["zen"] = config.Provider{BaseURL: p.BaseURL, APIKey: "synthetic-key"}
	_, err = s.providerAction("preset", "zen", "", "", "", "", "", false, 0, 0)
	if err != nil || s.cfg.Providers["zen"].APIKey != "synthetic-key" {
		t.Fatal("existing key lost")
	}
}
