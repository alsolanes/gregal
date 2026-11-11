package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/ordinador"
)

// cdpSa és el mínim d'un Chrome amb depuració: /json/version respon.
func cdpSa(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/version" {
			fmt.Fprint(w, `{"Browser":"Fake/1"}`)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

func ordinadorEnv(t *testing.T, cdp string) {
	t.Helper()
	t.Setenv("GREGAL_BROWSER_PROFILE", t.TempDir())
	t.Setenv("GREGAL_ORDINADOR_FILES", t.TempDir())
	t.Setenv("GREGAL_BROWSER_CDP", cdp)
	t.Setenv("GREGAL_ORDINADOR_OWNER", "anna")
	t.Setenv("GREGAL_ORDINADOR_ID", "pc-pilot-1")
	t.Setenv("GREGAL_ORDINADOR_VNC", "")
	t.Setenv("GREGAL_ORDINADOR_START_GRACE", "0")
	t.Setenv("GREGAL_ORDINADOR_PROFILE", "")
}

func ordinadorGet(t *testing.T, h http.Handler, user *User, query string) (int, map[string]any) {
	t.Helper()
	r := httptest.NewRequest("GET", "/api/ordinador"+query, nil)
	if user != nil {
		r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser{}, user))
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func checkOrdinadorContract(t *testing.T, out map[string]any) {
	t.Helper()
	for _, k := range []string{"id", "owner", "health", "capabilities", "controlOwner"} {
		if _, ok := out[k]; !ok {
			t.Fatalf("falta %q: %v", k, out)
		}
	}
	if h, _ := out["health"].(string); !slices.Contains(ordinadorHealth, h) {
		t.Fatalf("health fora del contracte: %v", out["health"])
	}
	if c, _ := out["controlOwner"].(string); !slices.Contains(ordinadorControl, c) {
		t.Fatalf("controlOwner fora del contracte: %v", out["controlOwner"])
	}
}

func caps(out map[string]any) []string {
	var c []string
	for _, v := range out["capabilities"].([]any) {
		c = append(c, v.(string))
	}
	return c
}

func TestOrdinadorHealthy(t *testing.T) {
	ordinadorEnv(t, cdpSa(t))
	_, h := knowledgeTestServer(t)
	code, out := ordinadorGet(t, h, &User{Name: "anna", Roots: []string{t.TempDir()}}, "")
	if code != 200 {
		t.Fatalf("codi %d: %v", code, out)
	}
	checkOrdinadorContract(t, out)
	if out["id"] != "pc-pilot-1" || out["owner"] != "anna" || out["health"] != "running" || out["controlOwner"] != "none" {
		t.Fatalf("ordinador sa: %v", out)
	}
	if c := caps(out); !slices.Contains(c, "browser") || !slices.Contains(c, "files") {
		t.Fatalf("capacitats: %v", c)
	}
	if out["lastSeenAt"] == nil {
		t.Fatal("un ordinador sa té lastSeenAt")
	}
	if code, _ := ordinadorGet(t, h, &User{Name: "anna"}, "?id=pc-pilot-1"); code != 200 {
		t.Fatalf("el seu id: %d", code)
	}
}

func TestOrdinadorBrowserStopped(t *testing.T) {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	dead := l.Addr().String()
	l.Close()
	ordinadorEnv(t, dead)
	_, h := knowledgeTestServer(t)
	code, out := ordinadorGet(t, h, &User{Name: "anna"}, "")
	if code != 200 {
		t.Fatalf("codi %d: %v", code, out)
	}
	checkOrdinadorContract(t, out)
	if out["health"] != "stopped" || slices.Contains(caps(out), "browser") {
		t.Fatalf("navegador aturat ha de dir stopped i sense browser: %v", out)
	}
	// CDP exposat fora de loopback: no es fa servir i es diu.
	t.Setenv("GREGAL_BROWSER_CDP", "192.0.2.1:9222")
	if _, out := ordinadorGet(t, h, &User{Name: "anna"}, ""); out["health"] != "unreachable" {
		t.Fatalf("CDP no loopback: %v", out)
	}
}

func TestOrdinadorOtherOwnerDenied(t *testing.T) {
	ordinadorEnv(t, cdpSa(t))
	_, h := knowledgeTestServer(t)
	code, out := ordinadorGet(t, h, &User{Name: "bernat"}, "")
	if code != http.StatusForbidden {
		t.Fatalf("un altre usuari: %d %v", code, out)
	}
	if _, ok := out["health"]; ok {
		t.Fatalf("la denegació no pot filtrar l'estat: %v", out)
	}
	if code, _ := ordinadorGet(t, h, &User{Name: "anna"}, "?id=pc-altre"); code != http.StatusNotFound {
		t.Fatalf("id d'un altre ordinador: %d", code)
	}
	if code, _ := (func() (int, map[string]any) {
		r := httptest.NewRequest("POST", "/api/ordinador", nil)
		r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser{}, &User{Name: "anna"}))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, nil
	})(); code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", code)
	}
}

// Sense propietari declarat (servidor multiusuari): cadascú veu el seu, amb
// ids diferents.
func TestOrdinadorPerUserWithoutDeclaredOwner(t *testing.T) {
	ordinadorEnv(t, cdpSa(t))
	t.Setenv("GREGAL_ORDINADOR_OWNER", "")
	t.Setenv("GREGAL_ORDINADOR_ID", "")
	_, h := knowledgeTestServer(t)
	_, a := ordinadorGet(t, h, &User{Name: "anna"}, "")
	_, b := ordinadorGet(t, h, &User{Name: "bernat"}, "")
	if a["id"] == b["id"] || a["owner"] != "anna" || b["owner"] != "bernat" {
		t.Fatalf("ordinadors barrejats: %v / %v", a, b)
	}
	if code, _ := ordinadorGet(t, h, &User{Name: "bernat"}, "?id="+a["id"].(string)); code != http.StatusNotFound {
		t.Fatalf("Bernat no pot demanar l'ordinador d'Anna: %d", code)
	}
}

// ---- mode amfitrió (GREGAL_ORDINADORS): la VM de cada persona ----

const ordTestToken = "0123456789abcdef0123456789abcdef0123456789abcdef"

// ambOrdinadorsHost: Carla i Aleix amb una VM cadascú; la de la Carla
// apagada al començament. Torna l'estat d'encesa i els verbs rebuts.
func ambOrdinadorsHost(t *testing.T) (*string, *[]string) {
	t.Helper()
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+ordTestToken {
			http.Error(w, "no", 401)
			return
		}
		json.NewEncoder(w).Encode(ordinador.Health{ID: "pc-carla", Owner: "carla", Health: "running", Capabilities: []string{"browser"}, ControlOwner: "none"})
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
	power := "stopped"
	var verbs []string
	var mu sync.Mutex
	m.SetControlRunner(func(ctx context.Context, argv []string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		verb := argv[len(argv)-1]
		verbs = append(verbs, argv[1]+":"+verb)
		if argv[1] == "aleix" {
			return "stopped", nil
		}
		switch verb {
		case "wake":
			power = "running"
		case "suspend":
			power = "saved"
		case "stop":
			power = "stopped"
		}
		return power, nil
	})
	oldMgr, oldErr := ordinadorHostMgr, ordinadorHostErr
	ordinadorHostOnce = sync.Once{}
	ordinadorHostOnce.Do(func() {})
	ordinadorHostMgr, ordinadorHostErr = m, nil
	t.Cleanup(func() {
		ordinadorHostOnce = sync.Once{}
		ordinadorHostOnce.Do(func() {})
		ordinadorHostMgr, ordinadorHostErr = oldMgr, oldErr
	})
	return &power, &verbs
}

func ordinadorPost(t *testing.T, h http.Handler, user *User, body string) (int, map[string]any) {
	t.Helper()
	r := httptest.NewRequest("POST", "/api/ordinador", strings.NewReader(body))
	r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser{}, user))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestOrdinadorHostVMOffIsStopped(t *testing.T) {
	ambOrdinadorsHost(t)
	_, h := knowledgeTestServer(t)
	start := time.Now()
	code, out := ordinadorGet(t, h, &User{Name: "carla"}, "")
	if code != 200 || out["health"] != "stopped" || out["power"] != "stopped" || out["id"] != "pc-carla" {
		t.Fatalf("VM apagada: %d %v", code, out)
	}
	checkOrdinadorContract(t, out)
	if time.Since(start) > 2*time.Second {
		t.Fatal("amb la VM apagada la resposta ha de ser immediata")
	}
}

func TestOrdinadorHostOtherOwnerDenied(t *testing.T) {
	ambOrdinadorsHost(t)
	_, h := knowledgeTestServer(t)
	if code, out := ordinadorGet(t, h, &User{Name: "aleix"}, "?id=pc-carla"); code != http.StatusForbidden || out["health"] != nil {
		t.Fatalf("l'Aleix no veu l'ordinador de la Carla: %d %v", code, out)
	}
	if code, _ := ordinadorPost(t, h, &User{Name: "aleix"}, `{"action":"wake"}`+"\n"); code != 200 && code != 202 {
		// L'Aleix sí que pot encendre el seu (sense ?id és el seu).
		t.Fatalf("el seu: %d", code)
	}
	r := httptest.NewRequest("POST", "/api/ordinador?id=pc-carla", strings.NewReader(`{"action":"stop"}`))
	r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser{}, &User{Name: "aleix"}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("l'Aleix no pot aturar l'ordinador de la Carla: %d", w.Code)
	}
	if code, _ := ordinadorGet(t, h, &User{Name: "bernat"}, ""); code != http.StatusNotFound {
		t.Fatalf("sense ordinador: %d", code)
	}
	if code, _ := ordinadorGet(t, h, &User{Name: "carla"}, "?id=pc-nou"); code != http.StatusNotFound {
		t.Fatalf("id desconegut: %d", code)
	}
}

func TestOrdinadorHostWakeAndStop(t *testing.T) {
	power, verbs := ambOrdinadorsHost(t)
	_, h := knowledgeTestServer(t)
	code, out := ordinadorPost(t, h, &User{Name: "carla"}, `{"action":"wake"}`)
	if code != http.StatusAccepted || *power != "running" || out["health"] != "running" {
		t.Fatalf("encendre: %d %v %v", code, out, *verbs)
	}
	usage, _ := out["usage"].(map[string]any)
	if usage["lastWakeReason"] != "user" || usage["onSince"] == nil {
		t.Fatalf("mesura d'ús: %v", out["usage"])
	}
	code, out = ordinadorPost(t, h, &User{Name: "carla"}, `{"action":"suspend"}`)
	if code != http.StatusAccepted || *power != "saved" || out["health"] != "stopped" || out["power"] != "saved" {
		t.Fatalf("desar: %d %v", code, out)
	}
	ordinadorGet(t, h, &User{Name: "carla"}, "")
	if strings.Contains(strings.Join(*verbs, ","), "carla:wake,carla:status,carla:suspend,carla:status,carla:wake") {
		t.Fatalf("la consulta de salut ha encès l'ordinador: %v", *verbs)
	}
	code, out = ordinadorPost(t, h, &User{Name: "carla"}, `{"action":"stop"}`)
	if code != http.StatusAccepted || *power != "stopped" || out["health"] != "stopped" {
		t.Fatalf("aturar: %d %v", code, out)
	}
	if code, _ := ordinadorPost(t, h, &User{Name: "carla"}, `{"action":"esborra"}`); code != 400 {
		t.Fatalf("acció desconeguda: %d", code)
	}
}

// Registre trencat: 503, mai les comprovacions locals d'una màquina que no
// és l'ordinador de ningú.
func TestOrdinadorHostBadRegistry(t *testing.T) {
	ambOrdinadorsHost(t)
	ordinadorHostMgr, ordinadorHostErr = nil, errors.New("trencat")
	_, h := knowledgeTestServer(t)
	if code, _ := ordinadorGet(t, h, &User{Name: "carla"}, ""); code != http.StatusServiceUnavailable {
		t.Fatalf("registre trencat: %d", code)
	}
}
