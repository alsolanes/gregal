package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/web"
)

// Les eines de prova que els jocs feien servir d'internal/web (goal_api_test,
// companion_test, corner_test), refetes amb el que Gregal exporta.

// goalTestServer és un servidor de Gregal de proves, amb un proveïdor fals.
func goalTestServer(t testing.TB) *web.Server {
	t.Helper()
	cfg := &config.Config{
		Mode: "code",
		Roles: map[string]config.Role{
			"chat":     {Provider: "p", Model: "m", ContextWindow: 32768},
			"think":    {Provider: "p", Model: "m", ContextWindow: 32768},
			"code":     {Provider: "p", Model: "m", ContextWindow: 32768},
			"reviewer": {Provider: "p", Model: "m", ContextWindow: 32768},
		},
		Providers:   map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1"}},
		Permissions: config.PermissionsCfg{Tools: map[string]string{}},
		Verify:      config.VerifyCfg{Mode: "off"},
		Agent:       config.AgentCfg{MaxSteps: 3},
	}
	return web.New(cfg, filepath.Join(t.TempDir(), "config.yaml"))
}

// companionTestServer és el servidor dels jocs damunt d'un de proves, amb la
// memòria de l'usuari en un directori temporal.
func companionTestServer(t testing.TB) (*Server, string) {
	t.Helper()
	ws := goalTestServer(t)
	home := t.TempDir()
	t.Setenv("GREGAL_MEMORY", filepath.Join(home, ".config", "gregal", "memory.md"))
	return New(ws), home
}

// cornerTestServer té dos vents, Xaloc i Mestral (entrenadors de la lliga),
// creats per la ruta de debò.
func cornerTestServer(t *testing.T) *Server {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	h := s.host.(*web.Server).Handler()
	for _, body := range []string{`{"name":"Xaloc","color":"#FF9F00","role":"jocs"}`, `{"name":"Mestral","color":"#3E7BFA","role":"disseny"}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/winds", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("vent de prova: %d %s", rec.Code, rec.Body.String())
		}
	}
	return s
}

func postJSON(t *testing.T, h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}
