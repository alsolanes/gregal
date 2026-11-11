package ordinador

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/identity"
)

const testToken = "0123456789abcdef0123456789abcdef0123456789abcdef"

func testAgent(t *testing.T) (http.Handler, string, *[]ExecRequest) {
	t.Helper()
	t.Setenv("GREGAL_BROWSER_PROFILE", t.TempDir())
	t.Setenv("GREGAL_BROWSER_CDP", "")
	t.Setenv("GREGAL_ORDINADOR_VNC", "")
	t.Setenv("GREGAL_ORDINADOR_PROFILE", "")
	files := t.TempDir()
	var mu sync.Mutex
	var calls []ExecRequest
	old := execTool
	execTool = func(ctx context.Context, session, dir, name, args string) (string, []string, error) {
		mu.Lock()
		calls = append(calls, ExecRequest{Session: session, Tool: name, Args: args, Dir: dir})
		mu.Unlock()
		return "fet:" + name, nil, nil
	}
	t.Cleanup(func() { execTool = old })
	h, err := NewAgent(AgentConfig{ID: "pc-carla", Owner: "carla", FilesDir: files, Token: testToken})
	if err != nil {
		t.Fatal(err)
	}
	return h, files, &calls
}

func call(t *testing.T, h http.Handler, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var rd *strings.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = strings.NewReader(string(b))
	} else {
		rd = strings.NewReader("")
	}
	r := httptest.NewRequest(method, path, rd)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestAgentAuth(t *testing.T) {
	h, _, calls := testAgent(t)
	for _, tok := range []string{"", "dolent", testToken + "x"} {
		if code, _ := call(t, h, "GET", "/v1/health", tok, nil); code != 401 {
			t.Fatalf("testimoni %q: %d", tok, code)
		}
		if code, _ := call(t, h, "POST", "/v1/exec", tok, ExecRequest{Owner: "carla", Tool: "bash"}); code != 401 {
			t.Fatalf("exec sense testimoni: %d", code)
		}
	}
	if len(*calls) != 0 {
		t.Fatal("cap eina sense autenticar")
	}
	if _, err := NewAgent(AgentConfig{ID: "x", Owner: "o", FilesDir: t.TempDir(), Token: "curt"}); err == nil {
		t.Fatal("un testimoni curt no s'accepta")
	}
}

func TestAgentHealth(t *testing.T) {
	h, _, _ := testAgent(t)
	code, out := call(t, h, "GET", "/v1/health", testToken, nil)
	if code != 200 || out["id"] != "pc-carla" || out["owner"] != "carla" {
		t.Fatalf("salut: %d %v", code, out)
	}
	if _, ok := out["health"].(string); !ok {
		t.Fatalf("sense health: %v", out)
	}
}

func TestAgentExecOwnerAndScope(t *testing.T) {
	h, files, calls := testAgent(t)
	carla := identity.Key("carla") + "-default"
	aleix := identity.Key("aleix") + "-default"
	if code, _ := call(t, h, "POST", "/v1/exec", testToken, ExecRequest{Owner: "aleix", Session: aleix, Tool: "bash"}); code != 403 {
		t.Fatalf("un altre compte: %d", code)
	}
	if code, _ := call(t, h, "POST", "/v1/exec", testToken, ExecRequest{Owner: "carla", Session: aleix, Tool: "bash"}); code != 403 {
		t.Fatalf("sessió d'un altre compte: %d", code)
	}
	if code, _ := call(t, h, "POST", "/v1/exec", testToken, ExecRequest{Owner: "carla", Session: carla, Tool: "publish"}); code != 400 {
		t.Fatalf("eina de l'amfitrió: %d", code)
	}
	for _, d := range []string{"../fora", "/etc", "a/../../fora"} {
		if code, _ := call(t, h, "POST", "/v1/exec", testToken, ExecRequest{Owner: "carla", Session: carla, Tool: "bash", Dir: d}); code != 400 {
			t.Fatalf("dir %q: %d", d, code)
		}
	}
	if len(*calls) != 0 {
		t.Fatalf("no s'havia d'executar res: %v", *calls)
	}
	code, out := call(t, h, "POST", "/v1/exec", testToken, ExecRequest{Owner: "carla", Session: carla, Tool: "browser", Args: `{"action":"where"}`, Dir: "web"})
	if code != 200 || out["output"] != "fet:browser" {
		t.Fatalf("exec: %d %v", code, out)
	}
	got := (*calls)[0]
	if got.Session != carla || got.Dir != filepath.Join(files, "web") || got.Tool != "browser" {
		t.Fatalf("crida: %+v", got)
	}
}

func TestCheckPrivateAddr(t *testing.T) {
	ok := []string{"127.0.0.1:8098", "10.66.0.2:8098", "192.168.1.5:1", "172.16.0.1:2", "100.101.102.103:8098", "[fd7a:115c::1]:8098", "localhost:8098"}
	bad := []string{"0.0.0.0:8098", "8.8.8.8:8098", "[::]:8098", "vm.exemple.cat:8098", "10.0.0.1"}
	for _, a := range ok {
		if err := CheckPrivateAddr(a); err != nil {
			t.Errorf("%s: %v", a, err)
		}
	}
	for _, a := range bad {
		if CheckPrivateAddr(a) == nil {
			t.Errorf("%s s'havia de rebutjar", a)
		}
	}
}

// fakeVM: hipervisor i agent falsos per al gestor de l'amfitrió.
type fakeVM struct {
	mu       sync.Mutex
	power    string
	verbs    []string
	agentUp  bool
	control  string
	hang     bool
	srv      *httptest.Server
	healthes int
}

func newFakeVM(t *testing.T) (*fakeVM, *Manager, *Computer) {
	t.Helper()
	f := &fakeVM{power: "stopped", control: "none"}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		up, hang, ctl := f.agentUp, f.hang, f.control
		f.healthes++
		f.mu.Unlock()
		if hang {
			select {
			case <-r.Context().Done():
			case <-time.After(10 * time.Second):
			}
			return
		}
		if !up {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+testToken {
			http.Error(w, "no", 401)
			return
		}
		switch r.URL.Path {
		case "/v1/health":
			json.NewEncoder(w).Encode(Health{ID: "pc-carla", Owner: "carla", Health: "running", Capabilities: []string{"browser", "files"}, ControlOwner: ctl})
		case "/v1/exec":
			var req ExecRequest
			json.NewDecoder(r.Body).Decode(&req)
			json.NewEncoder(w).Encode(ExecResponse{Output: req.Owner + ":" + req.Tool})
		}
	}))
	t.Cleanup(f.srv.Close)
	tok := filepath.Join(t.TempDir(), "token")
	os.WriteFile(tok, []byte(testToken+"\n"), 0o600)
	m, err := NewManager([]*Computer{{ID: "pc-carla", Owner: "carla", AgentURL: f.srv.URL, TokenFile: tok, Control: []string{"hv", "gregal-carla"}, IdleMinutes: 10, WakeTimeoutSeconds: 2}})
	if err != nil {
		t.Fatal(err)
	}
	m.SetControlRunner(func(ctx context.Context, argv []string) (string, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		verb := argv[len(argv)-1]
		f.verbs = append(f.verbs, verb)
		switch verb {
		case "status":
			return f.power + "\n", nil
		case "wake":
			f.power, f.agentUp = "running", true
		case "suspend":
			f.power, f.agentUp = "saved", false
		case "stop":
			f.power, f.agentUp = "stopped", false
		}
		return "", nil
	})
	m.SetMeterFile(filepath.Join(t.TempDir(), "uso.jsonl"))
	c, _ := m.For("carla")
	return f, m, c
}

func TestHostStatusVMOff(t *testing.T) {
	f, m, c := newFakeVM(t)
	f.hang = true // si es parlés amb l'agent, penjaria
	start := time.Now()
	r := m.Status(context.Background(), c)
	if r.Health != "stopped" || r.Power != "stopped" || len(r.Capabilities) != 0 || r.ControlOwner != "none" {
		t.Fatalf("VM apagada: %+v", r)
	}
	if time.Since(start) > time.Second || f.healthes != 0 {
		t.Fatal("amb la VM apagada no es parla amb l'agent")
	}
}

func TestHostStatusAgentHangsDoesNotHang(t *testing.T) {
	f, m, c := newFakeVM(t)
	f.power, f.hang = "running", true
	start := time.Now()
	r := m.Status(context.Background(), c)
	if el := time.Since(start); el > healthTimeout+2*time.Second {
		t.Fatalf("la salut ha trigat %v", el)
	}
	if r.Health != "unreachable" {
		t.Fatalf("encesa i sense agent: %+v", r)
	}
}

func TestHostWakeAndRunning(t *testing.T) {
	f, m, c := newFakeVM(t)
	if err := m.EnsureRunning(context.Background(), c, "task:7 browser", 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	r := m.Status(context.Background(), c)
	if r.Health != "running" || r.LastSeenAt == nil || r.Power != "running" {
		t.Fatalf("encesa: %+v", r)
	}
	if !strings.Contains(strings.Join(f.verbs, ","), "wake") {
		t.Fatalf("s'havia d'encendre: %v", f.verbs)
	}
	out, err := m.Exec(context.Background(), "carla", ExecRequest{Tool: "bash", Owner: "aleix"})
	if err != nil || out.Output != "carla:bash" {
		t.Fatalf("exec: %v %+v (l'owner el posa l'amfitrió)", err, out)
	}
	if _, err := m.Exec(context.Background(), "aleix", ExecRequest{Tool: "bash"}); !errors.Is(err, ErrNoComputer) {
		t.Fatalf("aleix no té ordinador aquí: %v", err)
	}
}

func TestHostWakingThenStarting(t *testing.T) {
	f, m, c := newFakeVM(t)
	m.SetControlRunner(func(ctx context.Context, argv []string) (string, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if argv[len(argv)-1] == "status" {
			return f.power, nil
		}
		f.power = "running" // encesa però l'agent encara no respon
		return "", nil
	})
	if err := m.Wake(context.Background(), c, "task:1"); err != nil {
		t.Fatal(err)
	}
	if r := m.Status(context.Background(), c); r.Health != "starting" {
		t.Fatalf("acabada d'encendre: %+v", r)
	}
	err := m.EnsureRunning(context.Background(), c, "task:8", 50*time.Millisecond)
	if !errors.Is(err, ErrComputerUnavailable) {
		t.Fatalf("si no arrenca, waiting_computer: %v", err)
	}
	if r := m.Status(context.Background(), c); r.Health != "unreachable" {
		t.Fatalf("passat el topall d'arrencada: %+v", r)
	}
}

func TestHostParkIdleSavesToDisk(t *testing.T) {
	f, m, c := newFakeVM(t)
	clock := time.Now()
	m.now = func() time.Time { return clock }
	m.Wake(context.Background(), c, "task:1")
	if got := m.ParkIdle(context.Background()); len(got) != 0 {
		t.Fatalf("acabada d'usar: %v", got)
	}
	clock = clock.Add(9 * time.Minute)
	if got := m.ParkIdle(context.Background()); len(got) != 0 {
		t.Fatalf("abans dels 10 minuts: %v", got)
	}
	clock = clock.Add(2 * time.Minute)
	f.control = "user" // visor obert / la persona té el control
	if got := m.ParkIdle(context.Background()); len(got) != 0 {
		t.Fatalf("amb el visor obert no es desa: %v", got)
	}
	f.control = "none"
	release := m.Hold(c)
	if got := m.ParkIdle(context.Background()); len(got) != 0 {
		t.Fatalf("amb feina en curs no es desa: %v", got)
	}
	release()
	clock = clock.Add(11 * time.Minute)
	if got := m.ParkIdle(context.Background()); len(got) != 1 || f.power != "saved" {
		t.Fatalf("inactiva: %v %s %v", got, f.power, f.verbs)
	}
	if !strings.Contains(strings.Join(f.verbs, ","), "suspend") || strings.Contains(strings.Join(f.verbs, ","), "stop") {
		t.Fatalf("per defecte es desa a disc, no s'atura: %v", f.verbs)
	}
	r := m.Status(context.Background(), c)
	if r.Health != "stopped" || r.Power != "saved" || r.Usage.OnSince != nil || r.Usage.Wakes != 1 || r.Usage.LastWakeReason != "task:1" {
		t.Fatalf("desada: %+v", r)
	}
	if r.Usage.OnSeconds != int64((22*time.Minute)/time.Second) {
		t.Fatalf("hores enceses: %d", r.Usage.OnSeconds)
	}
}

// La salut no desperta mai l'ordinador desat.
func TestHostStatusSavedNeverWakes(t *testing.T) {
	f, m, c := newFakeVM(t)
	f.power, f.hang = "saved", true
	for i := 0; i < 3; i++ {
		if r := m.Status(context.Background(), c); r.Health != "stopped" || r.Power != "saved" {
			t.Fatalf("desat: %+v", r)
		}
	}
	for _, v := range f.verbs {
		if v != "status" {
			t.Fatalf("la salut només pot preguntar l'estat: %v", f.verbs)
		}
	}
	if f.healthes != 0 {
		t.Fatal("amb l'ordinador desat no es parla amb l'agent")
	}
	if err := m.Wake(context.Background(), c, " "); err == nil {
		t.Fatal("cap encesa sense motiu")
	}
}

// El registre d'ús sobreviu a reiniciar l'amfitrió.
func TestHostMeterPersists(t *testing.T) {
	_, m, c := newFakeVM(t)
	clock := time.Now()
	m.now = func() time.Time { return clock }
	meter := filepath.Join(t.TempDir(), "uso.jsonl")
	m.SetMeterFile(meter)
	m.Wake(context.Background(), c, "viewer")
	clock = clock.Add(90 * time.Second)
	if err := m.Park(context.Background(), c, "user"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(meter)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"reason":"viewer"`) || !strings.Contains(lines[1], `"on_seconds":90`) {
		t.Fatalf("registre: %s", raw)
	}
	_, m2, c2 := newFakeVM(t)
	m2.SetMeterFile(meter)
	if u := m2.Status(context.Background(), c2).Usage; u.OnSeconds != 90 || u.Wakes != 1 || u.LastWakeReason != "viewer" {
		t.Fatalf("totals recuperats: %+v", u)
	}
}

func TestRegistryValidation(t *testing.T) {
	tok := filepath.Join(t.TempDir(), "t")
	os.WriteFile(tok, []byte(testToken), 0o600)
	short := filepath.Join(t.TempDir(), "s")
	os.WriteFile(short, []byte("curt"), 0o600)
	cases := map[string][]*Computer{
		"idle_action":   {{ID: "a", Owner: "a", AgentURL: "http://10.0.0.1:8098", TokenFile: tok, IdleAction: "hibernate-ish"}},
		"pública":       {{ID: "a", Owner: "a", AgentURL: "http://8.8.8.8:8098", TokenFile: tok}},
		"sense port":    {{ID: "a", Owner: "a", AgentURL: "http://10.0.0.1", TokenFile: tok}},
		"0.0.0.0":       {{ID: "a", Owner: "a", AgentURL: "http://0.0.0.0:8098", TokenFile: tok}},
		"testimoni":     {{ID: "a", Owner: "a", AgentURL: "http://10.0.0.1:8098", TokenFile: short}},
		"owner repetit": {{ID: "a", Owner: "a", AgentURL: "http://10.0.0.1:8098", TokenFile: tok}, {ID: "b", Owner: "a", AgentURL: "http://10.0.0.2:8098", TokenFile: tok}},
	}
	for name, list := range cases {
		if _, err := NewManager(list); err == nil {
			t.Errorf("%s: s'havia de rebutjar", name)
		}
	}
	reg := filepath.Join(t.TempDir(), "ordinadors.json")
	os.WriteFile(reg, []byte(`{"computers":[{"id":"pc-carla","owner":"carla","agent_url":"http://10.66.0.2:8098","token_file":`+jsonStr(tok)+`},{"id":"pc-aleix","owner":"aleix","agent_url":"http://10.66.0.3:8098","token_file":`+jsonStr(tok)+`}]}`), 0o600)
	m, err := LoadRegistry(reg)
	if err != nil {
		t.Fatal(err)
	}
	if c, _ := m.For("aleix"); c == nil || c.ID != "pc-aleix" || m.ByID("pc-carla").Owner != "carla" {
		t.Fatal("registre mal llegit")
	}
	// Sense ordre de hipervisor: estat desconegut i salut de l'agent.
	if r := m.Status(context.Background(), m.ByID("pc-carla")); r.Power != "unknown" {
		t.Fatalf("sense control: %+v", r)
	}
}

func jsonStr(s string) string { b, _ := json.Marshal(s); return string(b) }
