package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/identity"
	"gregal/internal/ordinador"
	"gregal/internal/tools"
)

// B3 — visor autenticat i control humà. No hi ha cap VM real: el
// hipervisor i l'agent de l'ordinador són dobles (httptest); el noVNC és un
// servidor fals darrere del doble de l'agent.

type fakeViewerAgent struct {
	mu      sync.Mutex
	power   string
	verbs   []string
	auths   []string
	cookies []string
	// wakeHold, si no és nil, reté el «wake» del hipervisor fins que es
	// tanca: així el test veu l'estat mentre l'ordinador encara arrenca.
	wakeHold chan struct{}
}

func (f *fakeViewerAgent) seen() (auths, cookies []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.auths...), append([]string(nil), f.cookies...)
}

// ambVisorHost: la Carla i l'Aleix tenen cadascú el seu ordinador; el de la
// Carla és desat a disc i respon (agent + noVNC) quan s'encén.
func ambVisorHost(t *testing.T) *fakeViewerAgent {
	t.Helper()
	f := &fakeViewerAgent{power: "saved"}
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		up := f.power == "running"
		if strings.HasPrefix(r.URL.Path, "/v1/viewer/") {
			f.auths = append(f.auths, r.Header.Get("Authorization"))
			f.cookies = append(f.cookies, r.Header.Get("Cookie"))
		}
		f.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+ordTestToken {
			http.Error(w, "no", 401)
			return
		}
		if !up {
			http.Error(w, "down", 503)
			return
		}
		switch {
		case r.URL.Path == "/v1/health":
			json.NewEncoder(w).Encode(ordinador.Health{ID: "pc-carla", Owner: "carla", Health: "running", Capabilities: []string{"browser"}, ControlOwner: "none"})
		case r.URL.Path == "/v1/viewer/vnc.html":
			io.WriteString(w, "NOVNC-CARLA")
		case r.URL.Path == "/v1/viewer/websockify" && strings.EqualFold(r.Header.Get("Upgrade"), "websocket"):
			conn, buf, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}
			defer conn.Close()
			buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
			buf.Flush()
			line, _ := buf.ReadString('\n')
			buf.WriteString("ECO:" + line)
			buf.Flush()
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(agent.Close)
	tok := filepath.Join(t.TempDir(), "token")
	os.WriteFile(tok, []byte(ordTestToken), 0o600)
	m, err := ordinador.NewManager([]*ordinador.Computer{
		{ID: "pc-carla", Owner: "carla", AgentURL: agent.URL, TokenFile: tok, Control: []string{"hv", "carla"}},
		{ID: "pc-aleix", Owner: "aleix", AgentURL: "http://127.0.0.1:1", TokenFile: tok, Control: []string{"hv", "aleix"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	m.SetControlRunner(func(ctx context.Context, argv []string) (string, error) {
		verb := argv[len(argv)-1]
		f.mu.Lock()
		hold := f.wakeHold
		f.mu.Unlock()
		if verb == "wake" && argv[1] != "aleix" && hold != nil {
			select {
			case <-hold:
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.verbs = append(f.verbs, argv[1]+":"+verb)
		if argv[1] == "aleix" {
			return "saved", nil
		}
		switch verb {
		case "wake":
			f.power = "running"
		case "suspend":
			f.power = "saved"
		}
		return f.power, nil
	})
	oldMgr, oldErr := ordinadorHostMgr, ordinadorHostErr
	ordinadorHostOnce = sync.Once{}
	ordinadorHostOnce.Do(func() {})
	ordinadorHostMgr, ordinadorHostErr = m, nil
	resetOrdinadorControl()
	visorWaking.Range(func(k, _ any) bool { visorWaking.Delete(k); return true })
	t.Cleanup(func() {
		ordinadorHostOnce = sync.Once{}
		ordinadorHostOnce.Do(func() {})
		ordinadorHostMgr, ordinadorHostErr = oldMgr, oldErr
		resetOrdinadorControl()
	})
	return f
}

// resetOrdinadorControl oblida la lease en memòria (com un reinici): la
// propera crida la torna a llegir del disc.
func resetOrdinadorControl() {
	ordinadorCtlMu.Lock()
	ordinadorCtlLeases, ordinadorCtlPath = nil, ""
	ordinadorCtlMu.Unlock()
	tools.SetBrowserControlGuard(nil)
}

func apiAs(t *testing.T, h http.Handler, user *User, method, path, body string) (int, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if user != nil {
		r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser{}, user))
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// noRedirect és un client que no segueix redireccions (per veure la galeta).
func noRedirect() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func TestVisorSessionWakesAndProxiesOnlyWithSession(t *testing.T) {
	f := ambVisorHost(t)
	_, h := knowledgeTestServer(t)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	carla := &User{Name: "carla"}

	// Sense sessió del visor no hi ha res: ni la pàgina ni el pont.
	for _, p := range []string{visorPrefix, visorPrefix + "estat", visorPrefix + "vnc/vnc.html", visorPrefix + "entra?t=inventat"} {
		resp, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s sense sessió: %d", p, resp.StatusCode)
		}
	}
	if auths, _ := f.seen(); len(auths) != 0 {
		t.Fatalf("sense sessió no s'ha de parlar amb l'ordinador: %v", auths)
	}

	// Obrir el visor amb l'ordinador desat: el desperta (motiu viewer) i
	// diu starting, sense esperar-lo. El wake queda retingut fins després de
	// la resposta: si el handler l'esperés, aquí es penjaria.
	hold := make(chan struct{})
	f.mu.Lock()
	f.wakeHold = hold
	f.mu.Unlock()
	code, out := apiAs(t, h, carla, "POST", "/api/ordinador/viewer", `{}`)
	close(hold)
	if code != 200 || out["health"] != "starting" || !strings.HasPrefix(fmt.Sprint(out["url"]), visorPrefix+"entra?t=") || out["expiresAt"] == nil {
		t.Fatalf("obrir el visor: %d %v", code, out)
	}
	waitBackground()
	if _, st := apiAs(t, h, carla, "GET", "/api/ordinador", ""); st["health"] != "running" || st["usage"].(map[string]any)["lastWakeReason"] != "viewer" {
		t.Fatalf("despertat pel visor: %v", st)
	}

	// El testimoni es bescanvia una sola vegada per la galeta de sessió.
	resp, err := noRedirect().Get(srv.URL + fmt.Sprint(out["url"]))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	var ck *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == visorCookie {
			ck = c
		}
	}
	if resp.StatusCode != http.StatusSeeOther || ck == nil || !ck.HttpOnly || ck.Path != visorPrefix || strings.Contains(fmt.Sprint(out["url"]), ck.Value) {
		t.Fatalf("bescanvi: %d %+v", resp.StatusCode, ck)
	}
	if resp, _ := noRedirect().Get(srv.URL + fmt.Sprint(out["url"])); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("el testimoni és d'un sol ús: %d", resp.StatusCode)
	}

	get := func(p string) (int, string) {
		req, _ := http.NewRequest("GET", srv.URL+p, nil)
		req.AddCookie(ck)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if code, body := get(visorPrefix); code != 200 || !strings.Contains(body, "vnc/vnc.html?path=ordinador/visor/vnc/websockify") {
		t.Fatalf("pàgina del visor: %d", code)
	}
	if code, body := get(visorPrefix + "estat"); code != 200 || !strings.Contains(body, `"health":"running"`) || !strings.Contains(body, `"controlOwner":"none"`) {
		t.Fatalf("estat: %d %s", code, body)
	}
	if code, body := get(visorPrefix + "vnc/vnc.html"); code != 200 || body != "NOVNC-CARLA" {
		t.Fatalf("pont: %d %q", code, body)
	}
	auths, cookies := f.seen()
	if len(auths) == 0 || auths[len(auths)-1] != "Bearer "+ordTestToken || cookies[len(cookies)-1] != "" {
		t.Fatalf("el pont posa el testimoni de l'ordinador i no passa la galeta: %v %v", auths, cookies)
	}

	// El WebSocket del noVNC passa pel pont.
	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "GET %svnc/websockify HTTP/1.1\r\nHost: x\r\nCookie: %s=%s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n", visorPrefix, visorCookie, ck.Value)
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	br := bufio.NewReader(conn)
	status, _ := br.ReadString('\n')
	if !strings.Contains(status, "101") {
		t.Fatalf("upgrade: %q", status)
	}
	for {
		l, err := br.ReadString('\n')
		if err != nil || l == "\r\n" {
			break
		}
	}
	fmt.Fprint(conn, "rfb\n")
	if got, _ := br.ReadString('\n'); got != "ECO:rfb\n" {
		t.Fatalf("websocket: %q", got)
	}

	// L'Aleix no pot obrir ni controlar l'ordinador de la Carla.
	aleix := &User{Name: "aleix"}
	if code, out := apiAs(t, h, aleix, "POST", "/api/ordinador/viewer", `{"id":"pc-carla"}`); code != http.StatusForbidden || out["url"] != nil {
		t.Fatalf("l'Aleix obre el visor de la Carla: %d %v", code, out)
	}
	if code, _ := apiAs(t, h, aleix, "POST", "/api/ordinador/control", `{"id":"pc-carla","action":"take"}`); code != http.StatusForbidden {
		t.Fatalf("l'Aleix pren el control de la Carla: %d", code)
	}
	if code, _ := apiAs(t, h, &User{Name: "bernat"}, "POST", "/api/ordinador/viewer", `{}`); code != http.StatusNotFound {
		t.Fatalf("sense ordinador: %d", code)
	}
	// El seu sí (desat: el desperta).
	if code, out := apiAs(t, h, aleix, "POST", "/api/ordinador/viewer", `{}`); code != 200 || out["id"] != "pc-aleix" {
		t.Fatalf("el de l'Aleix: %d %v", code, out)
	}
	waitBackground()
}

func TestControlLeasePausesAgentBrowser(t *testing.T) {
	ambVisorHost(t)
	_, h := knowledgeTestServer(t)
	old := tools.BrowserControlWait
	tools.BrowserControlWait = 60 * time.Millisecond
	t.Cleanup(func() { tools.BrowserControlWait = old })
	carla, aleix := &User{Name: "carla"}, &User{Name: "aleix"}
	carlaScope := identity.Key("carla") + "-feina"
	aleixScope := identity.Key("aleix") + "-feina"

	code, out := apiAs(t, h, carla, "POST", "/api/ordinador/control", `{"action":"take"}`)
	if code != 200 || out["controlOwner"] != "user" || out["leaseExpiry"] == nil {
		t.Fatalf("prendre el control: %d %v", code, out)
	}
	if _, st := apiAs(t, h, carla, "GET", "/api/ordinador", ""); st["controlOwner"] != "user" || st["leaseExpiry"] == nil {
		t.Fatalf("la salut diu qui té el control: %v", st)
	}
	if !tools.BrowserControlHeld(carlaScope) || tools.BrowserControlHeld(aleixScope) {
		t.Fatal("la lease és només de l'ordinador de la Carla")
	}
	// L'agent de la Carla no actua ni captura: espera i, si no torna, ho diu.
	_, _, err := tools.BrowserActionFor(context.Background(), carlaScope, "screenshot", nil)
	if !errors.Is(err, tools.ErrBrowserUserControl) {
		t.Fatalf("captura amb la persona al control: %v", err)
	}
	if _, st := apiAs(t, h, aleix, "GET", "/api/ordinador", ""); st["controlOwner"] == "user" {
		t.Fatalf("l'Aleix no hereta la lease: %v", st)
	}

	// Reinici de l'amfitrió: la lease continua (fins que caduqui).
	resetOrdinadorControl()
	if _, st := apiAs(t, h, carla, "GET", "/api/ordinador/control", ""); st["controlOwner"] != "user" {
		t.Fatalf("després del reinici: %v", st)
	}
	if !tools.BrowserControlHeld(carlaScope) {
		t.Fatal("després del reinici la guarda torna a ser-hi")
	}

	// Retornar el control: l'agent pot tornar a actuar (l'espera s'acaba).
	done := make(chan error, 1)
	go func() {
		tools.BrowserControlWait = 5 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		// sense navegador real: només comprovem que deixa d'esperar el control
		done <- waitBrowserControlForTest(ctx, carlaScope)
	}()
	time.Sleep(100 * time.Millisecond)
	if code, out := apiAs(t, h, carla, "POST", "/api/ordinador/control", `{"action":"release"}`); code != 200 || out["controlOwner"] != "none" {
		t.Fatalf("retornar: %d %v", code, out)
	}
	if err := <-done; err != nil {
		t.Fatalf("en retornar el control l'agent continua: %v", err)
	}
	if code, _ := apiAs(t, h, carla, "POST", "/api/ordinador/control", `{"action":"esborra"}`); code != 400 {
		t.Fatalf("acció desconeguda: %d", code)
	}
}

// waitBrowserControlForTest espera que la guarda deixi passar.
func waitBrowserControlForTest(ctx context.Context, scope string) error {
	for tools.BrowserControlHeld(scope) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	return nil
}

func TestVisorLocalModeNeedsLoopbackViewer(t *testing.T) {
	resetOrdinadorControl()
	t.Cleanup(resetOrdinadorControl)
	_, h := knowledgeTestServer(t)
	t.Setenv("GREGAL_ORDINADOR_VISOR", "")
	if code, out := apiAs(t, h, nil, "POST", "/api/ordinador/viewer", `{}`); code != http.StatusServiceUnavailable || out["reason"] != "no_viewer" {
		t.Fatalf("sense visor: %d %v", code, out)
	}
	novnc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "LOCAL") }))
	t.Cleanup(novnc.Close)
	t.Setenv("GREGAL_ORDINADOR_VISOR", strings.TrimPrefix(novnc.URL, "http://"))
	code, out := apiAs(t, h, nil, "POST", "/api/ordinador/viewer", `{"control":true}`)
	if code != 200 || out["controlOwner"] != "user" {
		t.Fatalf("visor local: %d %v", code, out)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	resp, err := noRedirect().Get(srv.URL + fmt.Sprint(out["url"]))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	req, _ := http.NewRequest("GET", srv.URL+visorPrefix+"vnc/vnc.html", nil)
	for _, c := range resp.Cookies() {
		req.AddCookie(c)
	}
	r2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r2.Body)
	r2.Body.Close()
	if string(b) != "LOCAL" {
		t.Fatalf("pont local: %d %q", r2.StatusCode, b)
	}
	// Batec sense la capçalera pròpia (un formulari d'un altre lloc): no.
	post, _ := http.NewRequest("POST", srv.URL+visorPrefix+"control", strings.NewReader(`{"action":"release"}`))
	for _, c := range resp.Cookies() {
		post.AddCookie(c)
	}
	r3, _ := http.DefaultClient.Do(post)
	r3.Body.Close()
	if r3.StatusCode != http.StatusForbidden {
		t.Fatalf("POST sense X-Gregal-Visor: %d", r3.StatusCode)
	}
	post, _ = http.NewRequest("POST", srv.URL+visorPrefix+"control", strings.NewReader(`{"action":"release"}`))
	post.Header.Set("X-Gregal-Visor", "1")
	for _, c := range resp.Cookies() {
		post.AddCookie(c)
	}
	r4, _ := http.DefaultClient.Do(post)
	b, _ = io.ReadAll(r4.Body)
	r4.Body.Close()
	if r4.StatusCode != 200 || !strings.Contains(string(b), `"controlOwner":"none"`) {
		t.Fatalf("retornar des del visor: %d %s", r4.StatusCode, b)
	}
}
