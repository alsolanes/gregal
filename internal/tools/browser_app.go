package tools

// Navegador de l'app d'escriptori. Quan l'app engega el backend, el seu
// procés principal (desktop/browser-bridge.js) obre un pont HTTP només a
// loopback amb un token aleatori i el passa per l'entorn:
//
//	GREGAL_APP_BROWSER=http://127.0.0.1:<port>/?token=<hex>
//
// El backend el llegeix en arrencar i l'esborra de l'entorn (les ordres de
// shell que llança l'agent no l'hereten). Contracte del pont, sempre amb
// Authorization: Bearer <token>:
//
//	POST /attach   mostra el panell de l'agent a l'app i hi enganxa el
//	               depurador (webContents.debugger). 200 {"ok":true}
//	POST /cdp      {"method","params"} → {"result":…} o {"error":{"message"}}
//	POST /release  desenganxa el depurador (l'usuari torna a manar).
//
// Un 410 vol dir que el panell ha desaparegut (recàrrega de la finestra):
// el transport queda mort i la propera acció torna a fer /attach.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	appBridgeMu  sync.Mutex
	appBridgeURL string
)

func init() {
	// Es captura un cop i s'esborra: el token només és per al backend.
	if v := strings.TrimSpace(os.Getenv("GREGAL_APP_BROWSER")); v != "" {
		appBridgeURL = v
		_ = os.Unsetenv("GREGAL_APP_BROWSER")
	}
}

// browserAppBridge és l'adreça del pont de l'app ("" si no n'hi ha).
func browserAppBridge() string {
	appBridgeMu.Lock()
	defer appBridgeMu.Unlock()
	return appBridgeURL
}

// setBrowserAppBridge canvia el pont (tests).
func setBrowserAppBridge(v string) {
	appBridgeMu.Lock()
	appBridgeURL = v
	appBridgeMu.Unlock()
}

// BrowserAppBridgeAvailable diu si aquest backend té el pont de l'app.
func BrowserAppBridgeAvailable() bool { return browserAppBridge() != "" }

// appBridgeAttachTimeout és el marge perquè l'app mostri el panell i
// carregui la vista (la primera vegada crea el webview).
var appBridgeAttachTimeout = 15 * time.Second

// appTransport parla CDP amb el pont HTTP de l'app d'escriptori.
type appTransport struct {
	scope  string
	base   string // http://127.0.0.1:port
	token  string
	client *http.Client
	mu     sync.Mutex
	dead   bool
}

// parseAppBridge valida l'adreça del pont: només http a loopback i amb token.
func parseAppBridge(raw string) (base, token string, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "http" || u.Host == "" {
		return "", "", fmt.Errorf("invalid bridge address")
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return "", "", fmt.Errorf("bridge must be on loopback, got %q", host)
	}
	token = u.Query().Get("token")
	if token == "" {
		return "", "", fmt.Errorf("bridge address without token")
	}
	return "http://" + u.Host, token, nil
}

// openAppBrowser es connecta al pont i demana el panell de l'agent.
func openAppBrowser(raw string, scopes ...string) (*Browser, error) {
	base, token, err := parseAppBridge(raw)
	if err != nil {
		return nil, err
	}
	t := &appTransport{base: base, token: token, client: &http.Client{}}
	if len(scopes) > 0 {
		t.scope = scopes[0]
	}
	ctx, cancel := context.WithTimeout(context.Background(), appBridgeAttachTimeout)
	defer cancel()
	if _, err := t.post(ctx, "/attach", map[string]any{}); err != nil {
		return nil, err
	}
	b := &Browser{refs: map[int]string{}, t: t, mode: BrowserModeApp}
	if err := b.prepara(); err != nil {
		t.Close()
		return nil, err
	}
	return b, nil
}

// post fa una petició autenticada al pont i torna el cos JSON.
func (t *appTransport) post(ctx context.Context, path string, body any) ([]byte, error) {
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.base+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+t.token)
	req.Header.Set("Content-Type", "application/json")
	if t.scope != "" {
		req.Header.Set("X-Gregal-Session", t.scope)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		if ctx.Err() == nil {
			t.markDead()
		}
		return nil, fmt.Errorf("app browser bridge: %v", err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusUnauthorized {
		t.markDead()
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		msg := strings.TrimSpace(string(out))
		if json.Unmarshal(out, &e) == nil && e.Error != "" {
			msg = e.Error
		}
		return nil, fmt.Errorf("app browser bridge: %s (HTTP %d)", truncateInline(msg, 200), resp.StatusCode)
	}
	return out, nil
}

func (t *appTransport) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if params == nil {
		params = map[string]any{}
	}
	out, err := t.post(ctx, "/cdp", map[string]any{"method": method, "params": params})
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("browser: %s: %v", method, err)
	}
	var m cdpMsg
	if err := json.Unmarshal(out, &m); err != nil {
		return nil, fmt.Errorf("browser: %s: invalid bridge reply", method)
	}
	if m.Error != nil {
		return nil, fmt.Errorf("browser: %s: %s", method, m.Error.Message)
	}
	if len(m.Result) == 0 {
		m.Result = json.RawMessage(`{}`)
	}
	return m.Result, nil
}

func (t *appTransport) markDead() {
	t.mu.Lock()
	t.dead = true
	t.mu.Unlock()
}

func (t *appTransport) Alive() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.dead
}

// Close desenganxa el depurador: el panell queda a l'usuari.
func (t *appTransport) Close() {
	t.mu.Lock()
	was := t.dead
	t.dead = true
	t.mu.Unlock()
	if was {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = t.post(ctx, "/release", map[string]any{})
}
