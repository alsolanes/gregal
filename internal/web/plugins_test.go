package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/config"
)

// fakePlugin és un plugin de prova amb una ruta i un bucle que compta
// quantes vegades ha arrencat i si encara corre.
func fakePlugin(starts, running *int32) *Plugin {
	return &Plugin{
		ID: "fake", Name: "Fake", NameCA: "Fals", Description: "d", DescriptionCA: "d", Icon: "*", View: "fake", DefaultOn: true,
		Routes: func(h *Hub, handle func(string, http.HandlerFunc)) {
			handle("/api/fake", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]bool{"ok": true}) })
		},
		Start: func(ctx context.Context, s *Server) {
			atomic.AddInt32(starts, 1)
			atomic.AddInt32(running, 1)
			<-ctx.Done()
			atomic.AddInt32(running, -1)
		},
	}
}

// pluginServer és hubTestServer amb un config que es pot desar.
func pluginServer(t *testing.T) *Server {
	return goalTestServer(t)
}

func pluginTestHub(t *testing.T) (*Hub, *int32, *int32) {
	t.Helper()
	var starts, running int32
	h := pluginServer(t).Hub()
	h.plug.list = []*Plugin{fakePlugin(&starts, &running)}
	return h, &starts, &running
}

func pluginWaitCount(t *testing.T, v *int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(v) != want {
		if time.Now().After(deadline) {
			t.Fatalf("comptador = %d, volia %d", atomic.LoadInt32(v), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func pluginReq(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

func TestPluginsRegistreTeElsTres(t *testing.T) {
	ids := map[string]bool{}
	for _, p := range registeredPlugins() {
		ids[p.ID] = true
		if !p.DefaultOn || p.View == "" || p.Icon == "" || p.Name == "" || p.NameCA == "" || p.Routes == nil {
			t.Errorf("plugin %s incomplet", p.ID)
		}
	}
	for _, id := range []string{"arena", "games", "corner"} {
		if !ids[id] {
			t.Errorf("falta el plugin %s al registre", id)
		}
	}
}

func TestPluginApagatRespon404INoArrenca(t *testing.T) {
	h, starts, running := pluginTestHub(t)
	mux := h.mux()
	if w := pluginReq(mux, "GET", "/api/fake", ""); w.Code != 200 {
		t.Fatalf("encès: %d", w.Code)
	}
	if w := pluginReq(mux, "POST", "/api/plugins", `{"id":"fake","enabled":false}`); w.Code != 200 {
		t.Fatalf("apagar: %d %s", w.Code, w.Body.String())
	}
	w := pluginReq(mux, "GET", "/api/fake", "")
	if w.Code != 404 || !strings.Contains(w.Body.String(), "plugin disabled") {
		t.Fatalf("apagat: %d %s", w.Code, w.Body.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.startPlugins(ctx)
	time.Sleep(30 * time.Millisecond)
	if atomic.LoadInt32(starts) != 0 || h.pluginRunning("fake") {
		t.Fatal("un plugin apagat no ha d'arrencar el bucle")
	}
	// Encendre'l en calent l'arrenca; apagar-lo l'atura.
	if w := pluginReq(mux, "POST", "/api/plugins", `{"id":"fake","enabled":true}`); w.Code != 200 {
		t.Fatalf("encendre: %d", w.Code)
	}
	pluginWaitCount(t, running, 1)
	if w := pluginReq(mux, "GET", "/api/fake", ""); w.Code != 200 {
		t.Fatalf("tornat a encendre: %d", w.Code)
	}
	if w := pluginReq(mux, "POST", "/api/plugins", `{"id":"fake","enabled":false}`); w.Code != 200 {
		t.Fatalf("apagar: %d", w.Code)
	}
	pluginWaitCount(t, running, 0)
	if atomic.LoadInt32(starts) != 1 {
		t.Fatalf("arrencades = %d", atomic.LoadInt32(starts))
	}
}

func TestPluginEstatEsDesaAlConfig(t *testing.T) {
	h, _, _ := pluginTestHub(t)
	if err := h.SetPluginEnabled("fake", false); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := config.Load(h.def.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if on, ok := cfg.Plugins["fake"]; !ok || on {
		t.Fatalf("config.plugins = %v", cfg.Plugins)
	}
	// Un servidor nou amb aquest config el troba apagat.
	h2 := New(cfg, h.def.cfgPath).Hub()
	h2.plug.list = h.plug.list
	if h2.PluginEnabled("fake") {
		t.Fatal("l'estat no ha sobreviscut el reinici")
	}
	if err := h.SetPluginEnabled("nope", true); err != errUnknownPlugin {
		t.Fatalf("plugin desconegut: %v", err)
	}
}

func TestAPIPluginsLlistaIAdmin(t *testing.T) {
	h, _, _ := pluginTestHub(t)
	mux := h.mux()
	w := pluginReq(mux, "GET", "/api/plugins", "")
	var got struct {
		Plugins []struct {
			ID, Name, Description, Icon, View string
			Enabled                           bool
		} `json:"plugins"`
		Admin bool `json:"admin"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err, w.Body.String())
	}
	if len(got.Plugins) != 1 || got.Plugins[0].ID != "fake" || !got.Plugins[0].Enabled || got.Plugins[0].View != "fake" || !got.Admin {
		t.Fatalf("resposta: %+v", got)
	}
	// Qui no és admin la pot llegir però no canviar.
	req := httptest.NewRequest("POST", "/api/plugins", strings.NewReader(`{"id":"fake","enabled":false}`))
	req = req.WithContext(context.WithValue(req.Context(), ctxKeyUser{}, &User{Name: "anna"}))
	rec := httptest.NewRecorder()
	h.handlePlugins(rec, req)
	if rec.Code != http.StatusForbidden || !h.PluginEnabled("fake") {
		t.Fatalf("no admin: %d", rec.Code)
	}
	if w := pluginReq(mux, "POST", "/api/plugins", `{"id":"fake"}`); w.Code != 400 {
		t.Fatalf("sense enabled: %d", w.Code)
	}
	if w := pluginReq(mux, "POST", "/api/plugins", `{"id":"nope","enabled":true}`); w.Code != 404 {
		t.Fatalf("desconegut: %d", w.Code)
	}
}

// Els plugins de debò: apagar l'Arena treu les seves rutes.
func TestPluginArenaApagadaRutes404(t *testing.T) {
	h := pluginServer(t).Hub()
	mux := h.mux()
	if err := h.SetPluginEnabled("arena", false); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/api/arena", "/api/arena/stream", "/api/arena/match?id=x"} {
		if w := pluginReq(mux, "GET", p, ""); w.Code != 404 || !strings.Contains(w.Body.String(), "plugin disabled") {
			t.Errorf("%s: %d %s", p, w.Code, w.Body.String())
		}
	}
	if !h.PluginEnabled("games") || !h.PluginEnabled("corner") {
		t.Fatal("apagar l'Arena no ha de tocar els altres")
	}
}
