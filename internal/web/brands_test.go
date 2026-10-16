package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/brand"
	"gregal/internal/config"
	"gregal/internal/ooxml"
)

func brandCall(t *testing.T, s *Server, fn func(*Server, http.ResponseWriter, *http.Request), method, path, body string) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	fn(s, w, httptest.NewRequest(method, path, strings.NewReader(body)))
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestBrandsPreviewInstallActivateDelete(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	defer ooxml.SetDefaultTheme("")
	// Un config de debò (la plantilla per defecte): s'ha de poder desar.
	cfg, path, err := config.Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, path)
	v1 := `{"name":"Acme","version":"1.0","colors":{"primary":"#112233","accent":"#FF8800"}}`
	v2 := `{"name":"Acme","version":"1.2","colors":{"primary":"#445566"}}`

	code, out := brandCall(t, s, (*Server).handleBrandPreview, "POST", "/api/brands/preview", v1)
	if code != 200 || out["status"] != "new" {
		t.Fatalf("preview new: %d %v", code, out)
	}
	sw := out["brand"].(map[string]any)["swatches"].([]any)
	if len(sw) != 2 {
		t.Fatalf("swatches = %v", sw)
	}
	if code, out = brandCall(t, s, (*Server).handleBrandPreview, "POST", "/api/brands/preview", `{"name":"x"}`); code != 400 || out["error"] == "" {
		t.Fatalf("invalid preview: %d %v", code, out)
	}

	if code, out = brandCall(t, s, (*Server).handleBrands, "POST", "/api/brands", v1); code != 200 || out["installed"] != "acme" {
		t.Fatalf("install: %d %v", code, out)
	}
	if _, out = brandCall(t, s, (*Server).handleBrandPreview, "POST", "/api/brands/preview", v2); out["status"] != "update" || out["installed_version"] != "1.0" {
		t.Fatalf("preview update: %v", out)
	}
	if _, out = brandCall(t, s, (*Server).handleBrandPreview, "POST", "/api/brands/preview", v1); out["status"] != "same" {
		t.Fatalf("preview same: %v", out)
	}

	if code, out = brandCall(t, s, (*Server).handleBrandActive, "POST", "/api/brands/active", `{"id":"acme"}`); code != 200 || out["active"] != "acme" {
		t.Fatalf("activate: %d %v", code, out)
	}
	cfg, _, err = config.Load(s.cfgPath)
	if err != nil || cfg.Brand != "acme" {
		t.Fatalf("config not saved: %v %+v", err, cfg)
	}
	if ooxml.GetTheme("").Primary != "112233" {
		t.Fatal("documents do not follow the active branding")
	}
	if code, _ = brandCall(t, s, (*Server).handleBrandActive, "POST", "/api/brands/active", `{"id":"nope"}`); code != 404 {
		t.Fatalf("activating a missing branding: %d", code)
	}

	w := httptest.NewRecorder()
	s.handleBrandItem(w, httptest.NewRequest("GET", "/api/brands/acme/file", nil))
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Disposition"), "acme.gregal-brand.json") {
		t.Fatalf("download: %d %v", w.Code, w.Header())
	}
	if p, err := brand.Parse(w.Body.Bytes()); err != nil || p.ID != "acme" {
		t.Fatalf("downloaded file does not parse back: %v", err)
	}

	if code, out = brandCall(t, s, (*Server).handleBrandItem, "DELETE", "/api/brands/acme", ""); code != 200 || out["active"] != "" {
		t.Fatalf("delete: %d %v", code, out)
	}
	if ooxml.GetTheme("").Name != "default" {
		t.Fatal("deleted active branding still drives documents")
	}
	if code, _ = brandCall(t, s, (*Server).handleBrandItem, "DELETE", "/api/brands/acme", ""); code != 404 {
		t.Fatalf("second delete: %d", code)
	}
}
