package tools

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

// fakeExtension fa d'extensió de Chrome: es connecta al backend, saluda i
// respon les ordres amb la pàgina falsa de browser_test.go.
type fakeExtension struct {
	mu   sync.Mutex
	ops  []string
	conn *websocket.Conn
}

func (fe *fakeExtension) send(v any) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	_ = websocket.JSON.Send(fe.conn, v)
}

func connectFakeExtension(t *testing.T, f *fakeCDP, origin string) (*fakeExtension, func()) {
	t.Helper()
	srv := httptest.NewServer(websocket.Server{Handler: func(ws *websocket.Conn) { ServeBrowserExtension(ws, nil) }})
	cfg, _ := websocket.NewConfig(strings.Replace(srv.URL, "http", "ws", 1)+"/", origin)
	conn, err := websocket.DialConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	fe := &fakeExtension{conn: conn}
	fe.send(map[string]string{"type": "hello", "version": "test"})
	var ready map[string]string
	if err := websocket.JSON.Receive(conn, &ready); err != nil || ready["type"] != "ready" {
		t.Fatalf("hello: %v %v", err, ready)
	}
	go func() {
		for {
			var m struct {
				ID     int            `json:"id"`
				Op     string         `json:"op"`
				Tab    int            `json:"tab"`
				Method string         `json:"method"`
				Params map[string]any `json:"params"`
			}
			if err := websocket.JSON.Receive(conn, &m); err != nil {
				return
			}
			fe.mu.Lock()
			fe.ops = append(fe.ops, m.Op)
			fe.mu.Unlock()
			var res any = map[string]any{}
			switch m.Op {
			case "open":
				res = map[string]any{"tab": 7}
			case "tabs":
				res = []map[string]any{{"id": 7, "title": "Prova", "url": "http://exemple.cat/"}}
			case "cdp":
				res = f.respon(m.Method, m.Params)
			}
			raw, _ := json.Marshal(res)
			fe.send(map[string]any{"id": m.ID, "result": json.RawMessage(raw)})
		}
	}()
	waitFor(t, func() bool { return BrowserExtensionStatus()["connected"] == true })
	return fe, func() { conn.Close(); srv.Close(); waitFor(t, func() bool { return BrowserExtensionStatus()["connected"] == false }) }
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if ok() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timeout")
}

func TestBrowserUsesTheUsersChromeThroughTheExtension(t *testing.T) {
	CloseBrowser()
	f := newFakeCDP(t)
	defer f.srv.Close()
	const scope = "ext-session"
	defer ForgetBrowserSites(scope)

	// Sense extensió, chrome:true explica què ha de fer l'usuari.
	if _, _, err := BrowserActionFor(context.Background(), scope, "open", map[string]any{"url": "http://exemple.cat/", "chrome": true}); err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("sense extensió: %v", err)
	}

	fe, stop := connectFakeExtension(t, f, "chrome-extension://abc")
	defer stop()
	defer CloseBrowser()

	// Al Chrome de l'usuari, fins i tot llegir un lloc nou demana permís (forçat).
	d, reason, force := BrowserGate(scope, map[string]any{"action": "open", "url": "http://exemple.cat/", "chrome": true}, true)
	if d != "ask" || !force || !strings.Contains(reason, "exemple.cat") {
		t.Fatalf("gate: %s %q %v", d, reason, force)
	}
	if _, _, err := BrowserActionFor(context.Background(), scope, "open", map[string]any{"url": "http://exemple.cat/", "chrome": true}); err != nil {
		t.Fatal(err)
	}
	b := sessionBrowser(scope)
	if b == nil || b.Mode() != BrowserModeChrome {
		t.Fatalf("mode: %+v", b)
	}
	// Executat = aprovat: el lloc queda permès, i la sessió es queda al Chrome.
	if d, why, _ := BrowserGate(scope, map[string]any{"action": "click", "target": "Documentació"}, false); d != "allow" {
		t.Log(why)
		t.Fatalf("lloc ja permès: %s", d)
	}
	// Però enviar torna a preguntar sempre.
	if d, _, force := BrowserGate(scope, map[string]any{"action": "click", "target": "Envia"}, true); d != "ask" || !force {
		t.Fatalf("irreversible: %s %v", d, force)
	}
	if out, _, err := BrowserActionFor(context.Background(), scope, "tabs", map[string]any{}); err != nil || !strings.Contains(out, "exemple.cat") {
		t.Fatalf("tabs: %v %q", err, out)
	}
	fe.mu.Lock()
	ops := strings.Join(fe.ops, ",")
	fe.mu.Unlock()
	if !strings.HasPrefix(ops, "open,cdp") || !strings.Contains(ops, "tabs") {
		t.Fatalf("ops: %s", ops)
	}

	// L'usuari prem Atura: l'agent no s'hi torna a enganxar sol.
	fe.send(map[string]any{"event": "stop"})
	waitFor(t, func() bool { return BrowserExtensionStatus()["stopped"] == true })
	if _, _, err := BrowserActionFor(context.Background(), scope, "read", map[string]any{}); err == nil || !strings.Contains(err.Error(), "stopped") {
		t.Fatalf("després d'aturar: %v", err)
	}
}

func TestBrowserGateGregalProfile(t *testing.T) {
	ambNavegadorFals(t)
	const scope = ""
	defer ForgetBrowserSites(scope)
	// Llegir amb el perfil propi no demana res.
	if d, _, _ := BrowserGate(scope, map[string]any{"action": "open", "url": "https://botiga.cat/"}, false); d != "allow" {
		t.Fatalf("open: %s", d)
	}
	// Actuar en un lloc nou sí, però no forçat; en autònom passa.
	lot := map[string]any{"action": "batch", "steps": []any{
		map[string]any{"action": "open", "url": "https://botiga.cat/"},
		map[string]any{"action": "type", "target": "e3", "text": "x"},
	}}
	if d, reason, force := BrowserGate(scope, lot, false); d != "ask" || force || !strings.Contains(reason, "botiga.cat") {
		t.Fatalf("lot: %s %q %v", d, reason, force)
	}
	if d, _, _ := BrowserGate(scope, lot, true); d != "allow" {
		t.Fatalf("autònom: %s", d)
	}
	// localhost no demana permís de lloc.
	if d, _, _ := BrowserGate(scope, map[string]any{"action": "batch", "steps": []any{
		map[string]any{"action": "open", "url": "http://localhost:3000/"},
		map[string]any{"action": "click", "target": "e1"},
	}}, false); d != "allow" {
		t.Fatalf("localhost: %s", d)
	}
	for _, name := range []string{"Envia la comanda", "Pay now", "Esborra el compte", "Confirmar compra", "Delete"} {
		if !IrreversibleName(name) {
			t.Errorf("%q és irreversible", name)
		}
	}
	for _, name := range []string{"Inici", "Orders", "Postres", "Sign in", "Cerca", "Següent"} {
		if IrreversibleName(name) {
			t.Errorf("%q no és irreversible", name)
		}
	}
}
