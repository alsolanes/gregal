package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"
)

// registerTestProduct registra un producte de proves i el treu (amb els seus
// plugins) en acabar.
func registerTestProduct(t *testing.T, p *Product) {
	t.Helper()
	RegisterProduct(p)
	t.Cleanup(func() {
		productMu.Lock()
		for i, q := range productReg {
			if q == p {
				productReg = append(productReg[:i], productReg[i+1:]...)
				break
			}
		}
		productMu.Unlock()
		pluginRegMu.Lock()
		for _, pl := range p.Plugins {
			for i, q := range pluginReg {
				if q == pl {
					pluginReg = append(pluginReg[:i], pluginReg[i+1:]...)
					break
				}
			}
		}
		pluginRegMu.Unlock()
	})
}

// Un producte de fora (com Gregal Jocs) hi posa la seva pàgina, els seus
// fitxers a /app/, rutes amb la mateixa autenticació i convidats.
func TestProductExtensionPoint(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	registerTestProduct(t, &Product{
		ID: "demo", Path: "/demo",
		Title: `<title>Demo</title>`,
		Slots: map[string]string{
			"css": `<link rel="stylesheet" href="/app/demo.css">`,
			"end": `<script type="module" src="/app/demo.js"></script>`,
		},
		Assets: fstest.MapFS{
			"app/demo.js":  {Data: []byte("import './demo-dep.js';\n")},
			"app/demo.css": {Data: []byte("body{}")},
		},
		Plugins: []*Plugin{{
			ID: "demoplug", Name: "Demo", NameCA: "Demo", Description: "d", DescriptionCA: "d", Icon: "*", View: "demo", DefaultOn: true, Order: 99,
			Routes: func(h *Hub, handle func(string, http.HandlerFunc)) {
				handle("/api/demo", h.Global(func(s *Server, w http.ResponseWriter, r *http.Request) {
					writeJSON(w, map[string]any{"guest": IsGuest(r), "lang": s.Tr("ca", "en")})
				}))
			},
		}},
		Guest: func(s *Server, r *http.Request) bool {
			return r.URL.Path == "/api/demo" && r.Header.Get("X-Demo-Guest") == "ok"
		},
	})
	s := goalTestServer(t)
	s.SetToken("secret")
	h := s.Handler()
	do := func(method, path string, hdr map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	page := do("GET", "/demo/", nil).Body.String()
	if !strings.Contains(page, `data-product="demo"`) || !strings.Contains(page, "<title>Demo</title>") {
		t.Fatalf("pàgina del producte: %.300s", page)
	}
	if !strings.Contains(page, `href="/app/demo.css?fgv=`) || !strings.Contains(page, `src="/app/demo.js?fgv=`) {
		t.Fatal("les marques han de portar l'HTML del producte, amb la versió")
	}
	if root := do("GET", "/", nil).Body.String(); strings.Contains(root, "demo.js") || strings.Contains(root, "<!--product:") {
		t.Fatal("l'HTML del producte només va a la seva pàgina")
	}
	if w := do("GET", "/app/demo.js", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "./demo-dep.js?fgv=") || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("fitxer del producte: %d %q %s", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
	if w := do("GET", "/app/nope.js", nil); w.Code != 404 {
		t.Fatalf("fitxer que no hi és: %d", w.Code)
	}
	if w := do("GET", "/api/demo", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("sense token ha de ser 401: %d", w.Code)
	}
	if w := do("GET", "/api/demo", map[string]string{"Authorization": "Bearer secret"}); w.Code != 200 || !strings.Contains(w.Body.String(), `"guest":false`) {
		t.Fatalf("amb token: %d %s", w.Code, w.Body.String())
	}
	if w := do("GET", "/api/demo", map[string]string{"X-Demo-Guest": "ok"}); w.Code != 200 || !strings.Contains(w.Body.String(), `"guest":true`) {
		t.Fatalf("convidat: %d %s", w.Code, w.Body.String())
	}
	if err := s.Hub().SetPluginEnabled("demoplug", false); err != nil {
		t.Fatal(err)
	}
	if w := do("GET", "/api/demo", map[string]string{"Authorization": "Bearer secret"}); w.Code != 404 {
		t.Fatalf("plugin apagat: %d", w.Code)
	}
	if w := do("GET", "/api/no-existeix", map[string]string{"Authorization": "Bearer secret"}); w.Code != 404 || !strings.Contains(w.Body.String(), "not found") {
		t.Fatalf("una /api/ desconeguda ha de ser 404: %d", w.Code)
	}
}

// Al servidor d'un producte (Gregal Jocs al costat de Gregal), la feina de
// fons de Gregal no arrenca; la dels plugins del producte, sí.
func TestSenseFeinaDeFonsDeGregal(t *testing.T) {
	SetGregalBackground(false)
	t.Cleanup(func() { SetGregalBackground(true) })
	var coreStarts, coreRunning, prodStarts, prodRunning int32
	core := fakePlugin(&coreStarts, &coreRunning)
	prod := fakePlugin(&prodStarts, &prodRunning)
	prod.ID, prod.product = "fakeprod", "demo"
	h := pluginServer(t).Hub()
	h.plug.list = []*Plugin{core, prod}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.startPlugins(ctx)
	pluginWaitCount(t, &prodRunning, 1)
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&coreStarts) != 0 {
		t.Fatal("el plugin de Gregal no ha d'arrencar al servidor d'un producte")
	}
	cancel()
	pluginWaitCount(t, &prodRunning, 0)
}
