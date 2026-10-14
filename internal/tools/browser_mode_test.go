package tools

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestChooseBrowserMode(t *testing.T) {
	cases := []struct {
		cfg           string
		force, bridge bool
		want          string
	}{
		{"", false, false, BrowserModeHeadless},
		{"auto", false, false, BrowserModeHeadless},
		{"auto", false, true, BrowserModeApp},
		{"", false, true, BrowserModeApp},
		{"app", false, false, BrowserModeApp}, // openBrowser fa el recanvi amb nota
		{"headless", false, true, BrowserModeHeadless},
		{"window", false, true, BrowserModeWindow},
		{"WINDOW", false, false, BrowserModeWindow},
		{"window", true, false, BrowserModeHeadless}, // GREGAL_BROWSER_HEADLESS=1 mana
		{"app", true, true, BrowserModeHeadless},
		{"auto", true, true, BrowserModeHeadless},
	}
	for _, c := range cases {
		if got := chooseBrowserMode(c.cfg, c.force, c.bridge); got != c.want {
			t.Errorf("chooseBrowserMode(%q, %v, %v) = %q, want %q", c.cfg, c.force, c.bridge, got, c.want)
		}
	}
	for _, ok := range []string{"", "auto", "app", "headless", "window", " Window "} {
		if !ValidBrowserMode(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"visible", "chrome", "on"} {
		if ValidBrowserMode(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

func TestEffectiveBrowserModeEnvOverride(t *testing.T) {
	ambModeNet(t, "window", "")
	if m := EffectiveBrowserMode(); m != BrowserModeWindow {
		t.Fatalf("window: %q", m)
	}
	t.Setenv("GREGAL_BROWSER_HEADLESS", "1")
	if m := EffectiveBrowserMode(); m != BrowserModeHeadless {
		t.Fatalf("env override: %q", m)
	}
}

// ambModeNet deixa el procés sense navegador, amb el mode i el pont donats,
// i restaura l'estat en acabar.
func ambModeNet(t *testing.T, mode, bridge string) {
	t.Helper()
	CloseBrowser()
	oldAttach, oldLaunch, oldBridge := browserAttach, launchChrome, browserAppBridge()
	browserModeMu.Lock()
	oldMode := browserModeCfg
	browserModeMu.Unlock()
	browserAttach = ""
	t.Setenv("GREGAL_BROWSER_HEADLESS", "")
	t.Setenv("GREGAL_BROWSER_CDP", "")
	SetBrowserMode(mode)
	setBrowserAppBridge(bridge)
	t.Cleanup(func() {
		CloseBrowser()
		browserAttach, launchChrome = oldAttach, oldLaunch
		setBrowserAppBridge(oldBridge)
		SetBrowserMode(oldMode)
	})
}

// llançadorFals substitueix l'arrencada de Chrome: es connecta al CDP fals
// i apunta si s'ha demanat headless.
func llançadorFals(t *testing.T) (*fakeCDP, *[]bool) {
	t.Helper()
	f := newFakeCDP(t)
	t.Cleanup(f.srv.Close)
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(f.srv.URL, "http://"))
	var mu sync.Mutex
	var crides []bool
	launchChrome = func(headless bool) (*Browser, error) {
		mu.Lock()
		crides = append(crides, headless)
		mu.Unlock()
		b, err := attachCDP("127.0.0.1:" + port)
		if b != nil {
			b.mode = map[bool]string{true: BrowserModeHeadless, false: BrowserModeWindow}[headless]
		}
		return b, err
	}
	return f, &crides
}

func TestBrowserModeHeadlessByDefault(t *testing.T) {
	ambModeNet(t, "auto", "")
	_, crides := llançadorFals(t)
	out, _, err := BrowserAction(nil, "open", map[string]any{"url": "http://exemple.cat/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(*crides) != 1 || !(*crides)[0] {
		t.Fatalf("auto without bridge must launch headless: %v", *crides)
	}
	if strings.Contains(out, "Note:") {
		t.Fatalf("no note expected: %s", out)
	}
}

func TestBrowserModeWindow(t *testing.T) {
	ambModeNet(t, "window", "")
	_, crides := llançadorFals(t)
	if _, _, err := BrowserAction(nil, "where", nil); err != nil {
		t.Fatal(err)
	}
	if len(*crides) != 1 || (*crides)[0] {
		t.Fatalf("window must launch a visible browser: %v", *crides)
	}
	// Canvi de config en calent: la propera acció reobre en headless.
	SetBrowserMode("headless")
	if _, _, err := BrowserAction(nil, "where", nil); err != nil {
		t.Fatal(err)
	}
	if len(*crides) != 2 || !(*crides)[1] {
		t.Fatalf("mode change must relaunch headless: %v", *crides)
	}
}

func TestBrowserModeAppWithoutBridgeFallsBack(t *testing.T) {
	ambModeNet(t, "app", "")
	_, crides := llançadorFals(t)
	out, _, err := BrowserAction(nil, "open", map[string]any{"url": "http://exemple.cat/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(*crides) != 1 || !(*crides)[0] {
		t.Fatalf("fallback must be headless: %v", *crides)
	}
	if !strings.HasPrefix(out, "Note: browser.mode is app but no desktop app browser bridge") {
		t.Fatalf("missing fallback note: %s", out)
	}
	// La nota surt un sol cop.
	out2, _, _ := BrowserAction(nil, "read", nil)
	if strings.Contains(out2, "Note:") {
		t.Fatalf("note repeated: %s", out2)
	}
}

func TestBrowserModeAutoBridgeDownFallsBack(t *testing.T) {
	pont := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "the app window is not ready"})
	}))
	defer pont.Close()
	ambModeNet(t, "auto", pont.URL+"/?token=t0k")
	_, crides := llançadorFals(t)
	out, _, err := BrowserAction(nil, "open", map[string]any{"url": "http://exemple.cat/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(*crides) != 1 || !(*crides)[0] {
		t.Fatalf("fallback must be headless: %v", *crides)
	}
	if !strings.Contains(out, "desktop app browser is not available") || !strings.Contains(out, "not ready") {
		t.Fatalf("missing fallback note: %s", out)
	}
}

// pontFals és el pont HTTP de l'app d'escriptori sobre la pàgina falsa.
type pontFals struct {
	*fakeCDP
	srv      *httptest.Server
	mu       sync.Mutex
	attach   int
	release  int
	noAuth   int
	gone     bool
	ordresOK []string
}

func newPontFals(t *testing.T, token string) *pontFals {
	t.Helper()
	p := &pontFals{fakeCDP: newFakeCDP(t)}
	t.Cleanup(p.fakeCDP.srv.Close)
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer "+token {
			p.mu.Lock()
			p.noAuth++
			p.mu.Unlock()
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		p.mu.Lock()
		gone := p.gone
		p.mu.Unlock()
		if gone {
			http.Error(w, `{"error":"browser view closed"}`, http.StatusGone)
			return
		}
		switch r.URL.Path {
		case "/attach":
			p.mu.Lock()
			p.attach++
			p.mu.Unlock()
			w.Write([]byte(`{"ok":true}`))
		case "/release":
			p.mu.Lock()
			p.release++
			p.mu.Unlock()
			w.Write([]byte(`{"ok":true}`))
		case "/cdp":
			var req struct {
				Method string         `json:"method"`
				Params map[string]any `json:"params"`
			}
			if json.NewDecoder(r.Body).Decode(&req) != nil {
				http.Error(w, `{"error":"bad json"}`, http.StatusBadRequest)
				return
			}
			if req.Method == "Browser.close" {
				json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "method not allowed"}})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"result": p.respon(req.Method, req.Params)})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(p.srv.Close)
	return p
}

func TestBrowserAppTransportDrivesTheAppView(t *testing.T) {
	p := newPontFals(t, "s3cret")
	ambModeNet(t, "auto", p.srv.URL+"/?token=s3cret")
	launchChrome = func(bool) (*Browser, error) {
		t.Fatal("app mode must not launch Chrome")
		return nil, nil
	}
	out, _, err := BrowserAction(nil, "open", map[string]any{"url": "http://exemple.cat/inici"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Benvingut") || !strings.Contains(out, `[3] button "Envia"`) {
		t.Fatalf("read via app: %s", out)
	}
	b, _ := GetBrowser()
	if b.Mode() != BrowserModeApp {
		t.Fatalf("mode: %q", b.Mode())
	}
	if out, _, err := BrowserAction(nil, "click", map[string]any{"target": "3"}); err != nil || !strings.Contains(out, `clic a button "Envia"`) {
		t.Fatalf("click via app: %v %q", err, out)
	}
	if _, imgs, err := BrowserAction(nil, "screenshot", nil); err != nil || len(imgs) != 1 {
		t.Fatalf("screenshot via app: %v", err)
	}
	p.mu.Lock()
	navegat, clics, attach := p.url, len(p.clics), p.attach
	calls := strings.Join(p.calls, ",")
	p.mu.Unlock()
	if navegat != "http://exemple.cat/inici" || clics != 1 || attach != 1 {
		t.Fatalf("url=%q clics=%d attach=%d", navegat, clics, attach)
	}
	if !strings.HasPrefix(calls, "Page.enable,Runtime.enable,Page.navigate") {
		t.Fatalf("calls: %s", calls)
	}
	// Els errors CDP del pont arriben com a errors de l'acció.
	if _, err := b.call("Browser.close", nil); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("CDP error: %v", err)
	}
	// close allibera el depurador (el panell queda a l'usuari).
	if out, _, _ := BrowserAction(nil, "close", nil); !strings.Contains(out, "tancat") {
		t.Fatal("close")
	}
	p.mu.Lock()
	release := p.release
	p.mu.Unlock()
	if release != 1 {
		t.Fatalf("release: %d", release)
	}
}

func TestBrowserAppTransportReattachesAfterViewGone(t *testing.T) {
	p := newPontFals(t, "tok")
	ambModeNet(t, "app", p.srv.URL+"/?token=tok")
	if _, _, err := BrowserAction(nil, "open", map[string]any{"url": "http://exemple.cat/"}); err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	p.gone = true
	p.mu.Unlock()
	if _, _, err := BrowserAction(nil, "read", nil); err == nil {
		t.Fatal("a closed view must fail the action")
	}
	p.mu.Lock()
	p.gone = false
	p.mu.Unlock()
	if _, _, err := BrowserAction(nil, "read", nil); err != nil {
		t.Fatalf("must reattach: %v", err)
	}
	p.mu.Lock()
	attach := p.attach
	p.mu.Unlock()
	if attach != 2 {
		t.Fatalf("attach calls: %d", attach)
	}
}

func TestParseAppBridge(t *testing.T) {
	if base, tok, err := parseAppBridge("http://127.0.0.1:4567/?token=abc"); err != nil || base != "http://127.0.0.1:4567" || tok != "abc" {
		t.Fatalf("%q %q %v", base, tok, err)
	}
	for _, bad := range []string{
		"http://127.0.0.1:4567/",              // sense token
		"http://192.168.1.10:4567/?token=abc", // fora de loopback
		"https://example.com/?token=abc",      // remot
		"ws://127.0.0.1:1/?token=abc",         // esquema
		"",
	} {
		if _, _, err := parseAppBridge(bad); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
	// Token equivocat: el pont diu 401 i no s'obre el navegador de l'app.
	p := newPontFals(t, "right")
	if _, err := openAppBrowser(p.srv.URL + "/?token=wrong"); err == nil {
		t.Fatal("wrong token must fail")
	}
	p.mu.Lock()
	noAuth := p.noAuth
	p.mu.Unlock()
	if noAuth == 0 {
		t.Fatal("bridge must have rejected the request")
	}
}
