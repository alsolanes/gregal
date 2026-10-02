package tools

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"golang.org/x/net/websocket"
)

// fakeCDP simula el port de depuració d'un Chromium: /json/version,
// /json/list i un websocket que respon les ordres que fa servir el
// navegador de l'agent. La «pàgina» és un HTML fix i un estat mínim
// (URL actual, text escrit, clics) per comprovar el circuit sencer sense
// cap navegador.
type fakeCDP struct {
	srv    *httptest.Server
	mu     sync.Mutex
	url    string
	html   string
	clics  []string
	escrit string
	calls  []string
}

func newFakeCDP(t *testing.T) *fakeCDP {
	t.Helper()
	f := &fakeCDP{url: "about:blank"}
	f.html = `<html><head><title>Prova</title></head><body><nav><a href="/x">menu</a></nav>
<main><h1>Benvingut</h1><p>Paràgraf amb prou text per ser contingut principal, amb comes i punts, que el lector reconegui com a cos.</p>
<a href="/docs">Documentació</a> <button id="ok">Envia</button> <input id="q" placeholder="Cerca">
<p>Un segon paràgraf també llarg, perquè hi hagi text de sobres i la puntuació pugi com toca.</p></main></body></html>`
	mux := http.NewServeMux()
	mux.HandleFunc("/json/version", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"Browser":"Fake/1","webSocketDebuggerUrl":"ws://x/devtools/browser/1"}`)
	})
	mux.HandleFunc("/json/list", func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		fmt.Fprintf(w, `[{"type":"page","url":"about:blank","webSocketDebuggerUrl":"ws://%s/devtools/page/1"}]`, host)
	})
	mux.Handle("/devtools/page/1", websocket.Handler(func(ws *websocket.Conn) {
		for {
			var raw []byte
			if err := websocket.Message.Receive(ws, &raw); err != nil {
				return
			}
			var req struct {
				ID     int            `json:"id"`
				Method string         `json:"method"`
				Params map[string]any `json:"params"`
			}
			if json.Unmarshal(raw, &req) != nil {
				continue
			}
			f.mu.Lock()
			f.calls = append(f.calls, req.Method)
			f.mu.Unlock()
			resp := map[string]any{"id": req.ID, "result": map[string]any{}}
			switch req.Method {
			case "Page.navigate":
				f.mu.Lock()
				f.url, _ = req.Params["url"].(string)
				f.mu.Unlock()
			case "Runtime.evaluate":
				expr, _ := req.Params["expression"].(string)
				resp["result"] = map[string]any{"result": map[string]any{"value": f.avalua(expr)}}
			case "Page.captureScreenshot":
				resp["result"] = map[string]any{"data": "/9j/4AAQSkZJRg=="}
			case "Input.insertText":
				f.mu.Lock()
				f.escrit, _ = req.Params["text"].(string)
				f.mu.Unlock()
			case "Input.dispatchMouseEvent":
				if tp, _ := req.Params["type"].(string); tp == "mouseReleased" {
					f.mu.Lock()
					f.clics = append(f.clics, "mouse")
					f.mu.Unlock()
				}
			}
			out, _ := json.Marshal(resp)
			websocket.Message.Send(ws, string(out))
		}
	}))
	f.srv = httptest.NewServer(mux)
	return f
}

// avalua respon les expressions JavaScript conegudes amb l'estat fals.
func (f *fakeCDP) avalua(expr string) any {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case expr == "document.readyState":
		return "complete"
	case expr == "location.href":
		return f.url
	case expr == "document.title":
		return "Prova"
	case strings.Contains(expr, "data-gregal-ref', String(n)"):
		snap := map[string]any{"title": "Prova", "url": f.url, "html": f.html,
			"items": []string{`[1] a "menu  → http://x/x"`, `[2] a "Documentació  → http://x/docs"`, `[3] button "Envia"`, `[4] input:text "Cerca"`}}
		raw, _ := json.Marshal(snap)
		return string(raw)
	case strings.Contains(expr, "document.querySelector(") && strings.Contains(expr, `? "1" : "0"`):
		if strings.Contains(expr, "#ok") || strings.Contains(expr, "data-gregal-ref") {
			return "1"
		}
		return "0"
	case strings.Contains(expr, "best.setAttribute('data-gregal-ref', 'text')"):
		if strings.Contains(strings.ToLower(expr), `"envia"`) {
			return "1"
		}
		return ""
	case strings.Contains(expr, "getBoundingClientRect") && strings.Contains(expr, "JSON.stringify({x:"):
		return `{"x":10,"y":20,"tag":"BUTTON","text":"Envia"}`
	case strings.Contains(expr, "e.focus()"):
		return true
	case strings.Contains(expr, "history.back()"):
		return nil
	}
	return "1+1=2"
}

func ambNavegadorFals(t *testing.T) *fakeCDP {
	t.Helper()
	CloseBrowser()
	f := newFakeCDP(t)
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(f.srv.URL, "http://"))
	old := browserAttach
	browserAttach = "127.0.0.1:" + port
	t.Cleanup(func() {
		CloseBrowser()
		browserAttach = old
		f.srv.Close()
	})
	return f
}

func TestBrowserObreILlegeix(t *testing.T) {
	f := ambNavegadorFals(t)
	out, imgs, err := BrowserAction(nil, "open", map[string]any{"url": "http://exemple.cat/inici"})
	if err != nil {
		t.Fatal(err)
	}
	if imgs != nil {
		t.Fatal("open no adjunta imatges")
	}
	for _, vol := range []string{"# Prova", "http://exemple.cat/inici", "Benvingut", "Paràgraf amb prou text", "Elements interactius", "[3] button \"Envia\"", "[4] input:text"} {
		if !strings.Contains(out, vol) {
			t.Errorf("falta %q a:\n%s", vol, out)
		}
	}
	if strings.Contains(out, "menu\n") && strings.Contains(out, "# menu") {
		t.Error("la navegació no és contingut principal")
	}
	f.mu.Lock()
	navegat := f.url
	f.mu.Unlock()
	if navegat != "http://exemple.cat/inici" {
		t.Fatalf("no ha navegat: %q", navegat)
	}
	if _, _, err := BrowserAction(nil, "open", map[string]any{"url": "file:///etc/passwd"}); err == nil {
		t.Fatal("només http(s)")
	}
}

func TestBrowserClicPerRefIPerText(t *testing.T) {
	f := ambNavegadorFals(t)
	if _, _, err := BrowserAction(nil, "open", map[string]any{"url": "http://exemple.cat/"}); err != nil {
		t.Fatal(err)
	}
	out, _, err := BrowserAction(nil, "click", map[string]any{"target": "3"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `clic a button "Envia"`) {
		t.Fatalf("clic: %q", out)
	}
	if _, _, err := BrowserAction(nil, "click", map[string]any{"target": "Envia"}); err != nil {
		t.Fatalf("clic per text: %v", err)
	}
	if _, _, err := BrowserAction(nil, "click", map[string]any{"target": "#ok"}); err != nil {
		t.Fatalf("clic per selector: %v", err)
	}
	if _, _, err := BrowserAction(nil, "click", map[string]any{"target": "99"}); err == nil || !strings.Contains(err.Error(), "[99]") {
		t.Fatalf("ref inexistent: %v", err)
	}
	f.mu.Lock()
	n := len(f.clics)
	f.mu.Unlock()
	if n != 3 {
		t.Fatalf("clics reals: %d", n)
	}
}

func TestBrowserEscriuCapturaIPolitica(t *testing.T) {
	f := ambNavegadorFals(t)
	if _, _, err := BrowserAction(nil, "open", map[string]any{"url": "http://exemple.cat/"}); err != nil {
		t.Fatal(err)
	}
	out, _, err := BrowserAction(nil, "type", map[string]any{"target": "4", "text": "hola món", "enter": true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hola món") || !strings.Contains(out, "Enter") {
		t.Fatalf("type: %q", out)
	}
	f.mu.Lock()
	escrit := f.escrit
	calls := strings.Join(f.calls, ",")
	f.mu.Unlock()
	if escrit != "hola món" || !strings.Contains(calls, "Input.dispatchKeyEvent") {
		t.Fatalf("escrit=%q calls=%s", escrit, calls)
	}
	txt, imgs, err := BrowserAction(nil, "screenshot", nil)
	if err != nil || len(imgs) != 1 || !strings.HasPrefix(imgs[0], "data:image/jpeg;base64,") || !strings.Contains(txt, "captura") {
		t.Fatalf("screenshot: %v %v %q", err, imgs, txt)
	}
	if _, _, err := BrowserAction(nil, "press", map[string]any{"key": "F13"}); err == nil {
		t.Fatal("tecla desconeguda ha de fallar")
	}
	if _, _, err := BrowserAction(nil, "dansa", nil); err == nil {
		t.Fatal("acció desconeguda ha de fallar")
	}
	for _, a := range []string{"open", "read", "screenshot", "back", "close"} {
		if !BrowserActionSegura(a) {
			t.Errorf("%s hauria de ser segura", a)
		}
	}
	for _, a := range []string{"click", "type", "press", "eval"} {
		if BrowserActionSegura(a) {
			t.Errorf("%s actua sobre la pàgina", a)
		}
	}
	if out, _, _ := BrowserAction(nil, "close", nil); !strings.Contains(out, "tancat") {
		t.Fatal("close")
	}
}
