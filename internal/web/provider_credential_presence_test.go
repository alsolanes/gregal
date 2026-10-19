package web

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/config"
)

func TestProvidersExposeResolvedCredentialPresenceWithoutLeakingKeys(t *testing.T) {
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{
		"configured": {BaseURL: "https://model.example/v1", APIKey: "synthetic-private-credential-for-test"},
		"empty":      {BaseURL: "https://empty.example/v1"},
		"spaces":     {BaseURL: "https://spaces.example/v1", APIKey: "  \t "},
		"unresolved": {BaseURL: "https://missing.example/v1", APIKey: "${MISSING_TEST_CREDENTIAL}"},
		"local":      {BaseURL: "http://127.0.0.1:8080/v1"},
	}
	w := httptest.NewRecorder()
	s.handleProviders(w, httptest.NewRequest("GET", "/api/providers", nil))
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	var body struct {
		Providers []struct {
			Name      string `json:"name"`
			HasAPIKey *bool  `json:"has_api_key"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Providers) != len(s.cfg.Providers) {
		t.Fatal("metadata endpoint must preserve configured endpoints, not delete keyless ones")
	}
	for _, p := range body.Providers {
		if p.HasAPIKey == nil {
			t.Errorf("%s: missing authoritative has_api_key field", p.Name)
			continue
		}
		if *p.HasAPIKey != (p.Name == "configured") {
			t.Errorf("%s: credential presence = %v", p.Name, *p.HasAPIKey)
		}
	}
	if strings.Contains(w.Body.String(), "synthetic-private-credential-for-test") {
		t.Fatal("provider secret leaked")
	}
}
