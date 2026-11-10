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

// cornerTestServer és el servidor dels jocs amb les dades en un directori
// temporal. No té cap vent: els jocs no en fan servir (la lliga té els seus
// membres, teams.go).
func cornerTestServer(t *testing.T) *Server {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	return s
}

func postJSON(t *testing.T, h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}
