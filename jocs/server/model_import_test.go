package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/profile"
	"gregal/internal/web"
)

// jocsProfile posa un home temporal amb el perfil de Gregal (config.yaml i
// auth.yaml) i fa que aquest procés sigui Gregal Jocs.
func jocsProfile(t *testing.T, gregalYAML, gregalAuth string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GREGAL_AUTH", filepath.Join(home, "jocs-auth.yaml"))
	prev := profile.Product()
	profile.SetProduct("jocs")
	t.Cleanup(func() { profile.SetProduct(prev) })
	g := filepath.Join(home, ".config", "gregal")
	if err := os.MkdirAll(g, 0o700); err != nil {
		t.Fatal(err)
	}
	if gregalYAML != "" {
		os.WriteFile(filepath.Join(g, "config.yaml"), []byte(gregalYAML), 0o600)
	}
	if gregalAuth != "" {
		os.WriteFile(filepath.Join(g, "auth.yaml"), []byte(gregalAuth), 0o600)
	}
	return home
}

// importTestServer és un Gregal Jocs sense cap model que respongui (el
// proveïdor de prova no té clau i no és local).
func importTestServer(t *testing.T) (*Server, http.Handler, string) {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := &config.Config{
		Mode: "code",
		Roles: map[string]config.Role{
			"chat":     {Provider: "p", Model: "m"},
			"think":    {Provider: "p", Model: "m"},
			"code":     {Provider: "p", Model: "m"},
			"reviewer": {Provider: "p", Model: "m"},
		},
		Providers:   map[string]config.Provider{"p": {BaseURL: "https://api.example.com/v1"}},
		Permissions: config.PermissionsCfg{Tools: map[string]string{}},
		Verify:      config.VerifyCfg{Mode: "off"},
		Agent:       config.AgentCfg{MaxSteps: 3},
	}
	ws := web.New(cfg, cfgPath)
	return New(ws), ws.Handler(), cfgPath
}

const gregalConfig = `mode: code
providers:
  openrouter:
    base_url: https://openrouter.ai/api/v1
    api_key: ""
roles:
  chat: {provider: openrouter, model: deepseek/deepseek-chat}
  code: {provider: openrouter, model: deepseek/deepseek-chat, context_window: 65536}
  reviewer: {provider: openrouter, model: deepseek/deepseek-chat}
`

func importState(t *testing.T, h http.Handler) map[string]any {
	t.Helper()
	w := get(t, h, "/api/jocs/model-import")
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	return out
}

func postImport(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/jocs/model-import", strings.NewReader(body)))
	return w
}

// La primera vegada, Gregal Jocs ofereix el model de Gregal; importar-lo
// en copia el proveïdor, el model i la clau (al seu auth.yaml, no al
// config), no toca cap fitxer de Gregal i l'oferta no torna a sortir.
func TestImportaElModelDeGregalUnaVegada(t *testing.T) {
	home := jocsProfile(t, gregalConfig, "keys:\n  openrouter: sk-or-secret\n")
	gregalDir := filepath.Join(home, ".config", "gregal")
	before := map[string]string{}
	for _, f := range []string{"config.yaml", "auth.yaml"} {
		raw, _ := os.ReadFile(filepath.Join(gregalDir, f))
		before[f] = string(raw)
	}
	s, h, cfgPath := importTestServer(t)

	st := importState(t, h)
	if st["offer"] != true || st["provider"] != "openrouter" || st["model"] != "deepseek/deepseek-chat" {
		t.Fatalf("oferta: %v", st)
	}
	if w := postImport(t, h, `{"action":"import"}`); w.Code != 200 {
		t.Fatalf("import: %d %s", w.Code, w.Body.String())
	}
	if !s.host.ModelReady() {
		t.Fatal("després d'importar, Gregal Jocs ha de tenir un model que respongui")
	}
	cfg, client, p, role := s.host.ChatModel()
	_ = client
	if role.Provider != "openrouter" || role.Model != "deepseek/deepseek-chat" || p.APIKey != "sk-or-secret" || cfg.Roles["code"].ContextWindow != 65536 {
		t.Fatalf("model adoptat: %+v %+v", role, p)
	}
	saved, _ := os.ReadFile(cfgPath)
	if strings.Contains(string(saved), "sk-or-secret") || !strings.Contains(string(saved), "openrouter") {
		t.Fatalf("el config de Jocs no ha de portar la clau:\n%s", saved)
	}
	if raw, _ := os.ReadFile(os.Getenv("GREGAL_AUTH")); !strings.Contains(string(raw), "sk-or-secret") {
		t.Fatalf("la clau ha d'anar a l'auth.yaml de Jocs: %s", raw)
	}
	for f, want := range before {
		raw, _ := os.ReadFile(filepath.Join(gregalDir, f))
		if string(raw) != want {
			t.Fatalf("no s'ha de tocar %s de Gregal", f)
		}
	}
	if ents, _ := os.ReadDir(gregalDir); len(ents) != 2 {
		t.Fatalf("la carpeta de Gregal no ha de tenir res de nou: %d fitxers", len(ents))
	}
	if st := importState(t, h); st["offer"] != false || st["state"] != "imported" {
		t.Fatalf("l'oferta només surt una vegada: %v", st)
	}
	if w := postImport(t, h, `{"action":"import"}`); w.Code != http.StatusConflict {
		t.Fatalf("una segona importació s'ha de refusar: %d", w.Code)
	}
}

// «Ara no» també compta: l'oferta ja no torna. Sense model a Gregal (o
// sense clau) no hi ha oferta.
func TestImportDescartadaISenseModel(t *testing.T) {
	jocsProfile(t, gregalConfig, "")
	_, h, _ := importTestServer(t)
	if st := importState(t, h); st["offer"] != false {
		t.Fatalf("Gregal sense clau no s'ofereix: %v", st)
	}
	if w := postImport(t, h, `{"action":"import"}`); w.Code != http.StatusConflict {
		t.Fatalf("sense model de Gregal: %d", w.Code)
	}

	jocsProfile(t, strings.Replace(gregalConfig, `api_key: ""`, `api_key: ${GREGAL_TEST_OR_KEY}`, 1), "")
	t.Setenv("GREGAL_TEST_OR_KEY", "sk-env")
	s, h, cfgPath := importTestServer(t)
	if st := importState(t, h); st["offer"] != true {
		t.Fatalf("una ${VAR} amb valor s'ofereix: %v", st)
	}
	if w := postImport(t, h, `{"action":"dismiss"}`); w.Code != 200 {
		t.Fatalf("dismiss: %d", w.Code)
	}
	if st := importState(t, h); st["offer"] != false || st["state"] != "dismissed" {
		t.Fatalf("descartada: %v", st)
	}
	// Importar-la igualment (des de Preferències) copia la referència.
	if w := postImport(t, h, `{"action":"import"}`); w.Code != 200 {
		t.Fatalf("import després de dismiss: %d %s", w.Code, w.Body.String())
	}
	saved, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(saved), "${GREGAL_TEST_OR_KEY}") || strings.Contains(string(saved), "sk-env") {
		t.Fatalf("la ${VAR} es copia com a referència:\n%s", saved)
	}
	if !s.host.ModelReady() {
		t.Fatal("ha de respondre amb la variable")
	}
}

// Un compte que no és administrador no veu l'oferta ni pot importar.
func TestImportNomesAdministrador(t *testing.T) {
	jocsProfile(t, gregalConfig, "keys:\n  openrouter: sk\n")
	s, _, _ := importTestServer(t)
	r := web.WithUser(httptest.NewRequest(http.MethodGet, "/api/jocs/model-import", nil), &web.User{Name: "anna"})
	w := httptest.NewRecorder()
	s.handleModelImport(w, r)
	if !strings.Contains(w.Body.String(), `"offer":false`) {
		t.Fatalf("no admin: %s", w.Body.String())
	}
	r = web.WithUser(httptest.NewRequest(http.MethodPost, "/api/jocs/model-import", strings.NewReader(`{"action":"import"}`)), &web.User{Name: "anna"})
	w = httptest.NewRecorder()
	s.handleModelImport(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("no admin: %d", w.Code)
	}
}
