package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/config"
)

// TestRequireToken: amb token, /api/* exigeix Bearer (la query ?token=
// ja no val); la pàgina no.
func TestRequireToken(t *testing.T) {
	s := goalTestServer(t)
	s.SetToken("secret")
	h := s.requireUser(http.HandlerFunc(s.handleState))

	cas := []struct {
		url, header string
		want        int
	}{
		{"/api/state", "", 401},
		{"/api/state", "Bearer malament", 401},
		{"/api/state", "Bearer secret", 200},
		{"/api/state?token=secret", "", 401},
		{"/", "", 200},
	}
	for _, c := range cas {
		req := httptest.NewRequest(http.MethodGet, c.url, nil)
		if c.header != "" {
			req.Header.Set("Authorization", c.header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s (%q): %d, volia %d", c.url, c.header, rec.Code, c.want)
		}
	}
}

// TestSenseTokenTotObert: sense token no hi ha barrera (comportament local).
func TestSenseTokenTotObert(t *testing.T) {
	s := goalTestServer(t)
	h := s.requireUser(http.HandlerFunc(s.handleState))
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("sense token hauria de passar: %d", rec.Code)
	}
}

// TestModelsLlistaProviders: /api/models retorna els ids vius per provider.
func TestModelsLlistaProviders(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/models") {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"data":[{"id":"model-b"},{"id":"model-a"}]}`))
	}))
	defer llm.Close()

	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{
		"local":  {BaseURL: llm.URL + "/v1"},
		"caigut": {BaseURL: "http://127.0.0.1:9"},
	}

	rec := httptest.NewRecorder()
	s.handleModels(rec, httptest.NewRequest(http.MethodGet, "/api/models", nil))
	if rec.Code != 200 {
		t.Fatalf("models: %d %s", rec.Code, rec.Body.String())
	}
	var v struct {
		Models           map[string][]string `json:"models"`
		Errors           map[string]string   `json:"errors"`
		Current          string              `json:"current"`
		Role             string              `json:"role"`
		SelectedOverride string              `json:"selected_override"`
		Fallback         string              `json:"fallback"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Models["local"]) != 2 || v.Models["local"][0] != "model-a" {
		t.Fatalf("llista inesperada: %+v", v.Models)
	}
	if _, ok := v.Errors["caigut"]; !ok {
		t.Fatalf("el provider caigut hauria de tenir error: %+v", v.Errors)
	}
	if !strings.Contains(v.Current, "p/m") {
		t.Fatalf("current inesperat: %q", v.Current)
	}
	if v.SelectedOverride != "" || v.Fallback != "p/m" {
		t.Fatalf("estat del model actual inesperat: override=%q fallback=%q", v.SelectedOverride, v.Fallback)
	}
}

// TestModelNomésAcceptaUnIDQueElProviderAnuncia evita overrides fantasmes.
func TestModelNomésAcceptaUnIDQueElProviderAnuncia(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/models") {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"data":[{"id":"model-real"}]}`))
	}))
	defer llm.Close()

	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"local": {BaseURL: llm.URL + "/v1"}}
	s.cfg.Roles["chat"] = config.Role{Provider: "local", Model: "model-real"}
	s.role = "chat"

	post := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		s.handleModel(rec, httptest.NewRequest(http.MethodPost, "/api/model", bytes.NewBufferString(body)))
		return rec
	}
	if rec := post(`{"model":"local/model-real"}`); rec.Code != http.StatusOK {
		t.Fatalf("model real: %d %s", rec.Code, rec.Body.String())
	}
	if got := s.modelOverride["chat"]; got != "local/model-real" {
		t.Fatalf("override: %q", got)
	}
	if rec := post(`{"model":"local/inventat"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("model inventat ha de fallar: %d %s", rec.Code, rec.Body.String())
	}
	if got := s.modelOverride["chat"]; got != "local/model-real" {
		t.Fatalf("un error no pot canviar l'override: %q", got)
	}
}

func TestRoleRefRespectaOverrideDelRolRutejat(t *testing.T) {
	s := goalTestServer(t)
	s.cfg.Providers["p2"] = config.Provider{BaseURL: "http://p2/v1"}
	s.modelOverride["code"] = "p2/model-code"
	s.role = "chat"
	p, r := s.roleRefFor("code")
	if p.BaseURL != "http://p2/v1" || r.Provider != "p2" || r.Model != "model-code" {
		t.Fatalf("override del rol rutejat ignorat: provider=%+v role=%+v", p, r)
	}
}

func TestRoleOverridePreservesReasoningAndFallback(t *testing.T) {
	s := goalTestServer(t)
	s.cfg.Providers["p2"] = config.Provider{BaseURL: "http://p2/v1"}
	original := s.cfg.Roles["code"]
	original.Think = "no"
	original.FallbackProvider, original.FallbackModel = "backup", "backup-model"
	original.MaxTokens = 8192
	s.cfg.Roles["code"] = original
	s.modelOverride["code"] = "p2/model-code"
	_, got := s.roleRefFor("code")
	want := original
	want.Provider, want.Model = "p2", "model-code"
	if got != want || s.cfg.Roles["code"] != original {
		t.Fatalf("override lost role settings or mutated config: got=%+v want=%+v", got, want)
	}
}

func TestModelsExposeFallbackWhenStoredOverrideIsUnavailable(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"model-live"}]}`))
	}))
	defer llm.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"local": {BaseURL: llm.URL + "/v1"}}
	s.cfg.Roles["chat"] = config.Role{Provider: "local", Model: "model-fallback"}
	s.role = "chat"
	s.modelOverride["chat"] = "local/model-removed"

	rec := httptest.NewRecorder()
	s.handleModels(rec, httptest.NewRequest(http.MethodGet, "/api/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("models: %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Current           string `json:"current"`
		SelectedOverride  string `json:"selected_override"`
		SelectedAvailable bool   `json:"selected_available"`
		Fallback          string `json:"fallback"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Current != "local/model-fallback" || response.Fallback != response.Current || response.SelectedAvailable {
		t.Fatalf("fallback metadata missing: %+v", response)
	}
	if response.SelectedOverride != "local/model-removed" {
		t.Fatalf("unavailable choice should remain visible: %+v", response)
	}
	p, role := s.roleRefFor("chat")
	if p.BaseURL != s.cfg.Providers["local"].BaseURL || role.Model != "model-fallback" {
		t.Fatalf("runtime did not use configured fallback: provider=%+v role=%+v", p, role)
	}
}

func TestExecutionRefreshFallsBackForPersistedUnavailableModel(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"model-live"}]}`))
	}))
	defer llm.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"local": {BaseURL: llm.URL + "/v1"}}
	s.cfg.Roles["chat"] = config.Role{Provider: "local", Model: "model-fallback"}
	s.modelOverride["chat"] = "local/model-removed"

	selection, unavailable := s.refreshModelAvailability("chat")
	if selection != "local/model-removed" || !unavailable {
		t.Fatalf("availability check: selection=%q unavailable=%v", selection, unavailable)
	}
	p, role := s.roleRefFor("chat")
	if p.BaseURL != llm.URL+"/v1" || role.Model != "model-fallback" {
		t.Fatalf("execution fallback: provider=%+v role=%+v", p, role)
	}
}
