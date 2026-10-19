package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"gregal/internal/config"
)

func TestConfiguredModelListingSkipsKeylessEndpointsWithoutChangingSelection(t *testing.T) {
	var keyedCalls, keylessCalls atomic.Int32
	keyed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keyedCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"shared-model"}]}`))
	}))
	defer keyed.Close()
	keyless := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keylessCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"keyless-model"}]}`))
	}))
	defer keyless.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{
		"ready":   {BaseURL: keyed.URL, APIKey: "synthetic-listing-key"},
		"keyless": {BaseURL: keyless.URL},
	}
	s.modelOverride = map[string]string{s.role: "keyless/keyless-model"}
	w := httptest.NewRecorder()
	s.handleModels(w, httptest.NewRequest("GET", "/api/models?configured_only=1", nil))
	var body struct {
		Models            map[string][]string `json:"models"`
		SelectedAvailable *bool               `json:"selected_available"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if keyedCalls.Load() != 1 || keylessCalls.Load() != 0 {
		t.Errorf("catalogue queries keyed=%d keyless=%d", keyedCalls.Load(), keylessCalls.Load())
	}
	if len(body.Models) != 1 || len(body.Models["ready"]) != 1 {
		t.Errorf("unexpected filtered catalogue: %#v", body.Models)
	}
	if body.SelectedAvailable != nil {
		t.Error("a filtered-out selection must remain unprobed, not be marked unavailable")
	}
	if s.unavailableModels[s.role] != "" {
		t.Fatal("picker filtering changed actual active model availability")
	}
}
