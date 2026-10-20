package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/session"
)

// TestWindRoleRefSenseOverride: sense model propi, els vents parlen amb el
// mateix target que la feina (el rol base del servidor).
func TestWindRoleRefSenseOverride(t *testing.T) {
	s := goalTestServer(t)
	s.role = "code"
	p, r := s.windRoleRef()
	if p.BaseURL != s.cfg.Providers["p"].BaseURL || r.Model != "m" || r.Provider != "p" {
		t.Fatalf("sense override hauria de ser el rol base: %+v %+v", p, r)
	}
}

// TestWindRoleRefAplicaModelPropi: amb model per defecte dels vents, el
// provider/model canvien però la resta del rol (temperatura, tokens) es
// conserva.
func TestWindRoleRefAplicaModelPropi(t *testing.T) {
	s := goalTestServer(t)
	s.role = "code"
	s.cfg.Providers["q"] = config.Provider{BaseURL: "http://127.0.0.1:9"}
	s.cfg.Roles["code"] = config.Role{Provider: "p", Model: "m", Temperature: 0.7, MaxTokens: 111, ContextWindow: 32768}
	s.modelOverride["vents"] = "q/qm"
	p, r := s.windRoleRef()
	if p.BaseURL != "http://127.0.0.1:9" || r.Provider != "q" || r.Model != "qm" {
		t.Fatalf("hauria d'aplicar el model dels vents: %+v %+v", p, r)
	}
	if r.Temperature != 0.7 || r.MaxTokens != 111 {
		t.Fatalf("la resta del rol s'ha de conservar: %+v", r)
	}
}

// TestModelScopeVentsDesaPreferènciaPròpia: POST /api/model amb
// scope "vents" desa el model per defecte dels vents sense tocar el de la
// feina, i /api/models l'exposa.
func TestModelScopeVentsDesaPreferènciaPròpia(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"model-vent"}]}`))
	}))
	defer llm.Close()

	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"local": {BaseURL: llm.URL + "/v1"}}
	s.cfg.Roles["chat"] = config.Role{Provider: "local", Model: "model-feina"}
	s.role = "chat"

	rec := httptest.NewRecorder()
	s.handleModel(rec, httptest.NewRequest(http.MethodPost, "/api/model",
		bytes.NewBufferString(`{"model":"local/model-vent","scope":"vents"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("scope vents: %d %s", rec.Code, rec.Body.String())
	}
	if got := s.modelOverride["vents"]; got != "local/model-vent" {
		t.Fatalf("override vents: %q", got)
	}
	if got := s.modelOverride["chat"]; got != "" {
		t.Fatalf("el model de la feina no s'ha de tocar: %q", got)
	}
	defaults, err := session.LoadModelDefaults(s.usuari())
	if err != nil || defaults["vents"] != "local/model-vent" {
		t.Fatalf("no persisteix: %+v %v", defaults, err)
	}

	rec2 := httptest.NewRecorder()
	s.handleModels(rec2, httptest.NewRequest(http.MethodGet, "/api/models", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("models: %d %s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), `"vents_selected":"local/model-vent"`) {
		t.Fatalf("models hauria d'exposar el model dels vents: %s", rec2.Body.String())
	}
}

// TestModelScopeVentsRebutjaPeticionsMalformades: sense provider explícit o
// amb un model que el provider no anuncia, 400 i sense canvis.
func TestModelScopeVentsRebutjaPeticionsMalformades(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"model-vent"}]}`))
	}))
	defer llm.Close()

	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"local": {BaseURL: llm.URL + "/v1"}}
	s.cfg.Roles["chat"] = config.Role{Provider: "local", Model: "model-feina"}
	s.role = "chat"

	for _, body := range []string{
		`{"model":"model-vent","scope":"vents"}`,
		`{"model":"local/noexisteix","scope":"vents"}`,
	} {
		rec := httptest.NewRecorder()
		s.handleModel(rec, httptest.NewRequest(http.MethodPost, "/api/model", bytes.NewBufferString(body)))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s hauria de ser 400: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	if got := s.modelOverride["vents"]; got != "" {
		t.Fatalf("un error no pot desar res: %q", got)
	}
}

// TestWindRoleRefOverrideInvalid: un override que ja no existeix al config
// no tomba el vent: cau al rol base.
func TestWindRoleRefOverrideInvalid(t *testing.T) {
	s := goalTestServer(t)
	s.role = "code"
	s.modelOverride["vents"] = "fantasma/model-x"
	p, r := s.windRoleRef()
	if r.Provider != "p" || r.Model != "m" {
		t.Fatalf("amb override invàlid hauria de caure al rol base: %+v %+v", p, r)
	}
	_ = p
}
