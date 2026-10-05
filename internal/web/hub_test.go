package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/session"
)

func hubTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		Providers: map[string]config.Provider{"p": {BaseURL: "http://x", APIKey: "k"}},
		Roles: map[string]config.Role{
			"chat": {Provider: "p", Model: "m"}, "code": {Provider: "p", Model: "m"},
			"reviewer": {Provider: "p", Model: "m"},
		},
		Mode: "code",
	}
	s := New(cfg, filepath.Join(dir, "config.yaml"))
	return s
}

// Sense id de sessió tot va a la default: els clients antics no noten G1.
func TestSessionPerDefecte(t *testing.T) {
	h := hubTestServer(t).Hub()
	r := httptest.NewRequest("GET", "/api/state", nil)
	if got := h.resolve(r); got != h.def {
		t.Fatal("sense id s'ha d'usar la sessió default")
	}
	if sessionID(r) != DefaultSession {
		t.Fatalf("id=%q", sessionID(r))
	}
}

func TestHealthIdentificaProtocolICapacitats(t *testing.T) {
	s := hubTestServer(t)
	w := httptest.NewRecorder()
	s.handleHealth(w, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("health: %d %s", w.Code, w.Body.String())
	}
	var got struct {
		Status       string          `json:"status"`
		Protocol     string          `json:"protocol"`
		Instance     string          `json:"instance_id"`
		Capabilities map[string]bool `json:"capabilities"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "ok" || got.Protocol != APIProtocol || got.Instance == "" {
		t.Fatalf("health incomplet: %+v", got)
	}
	_, streamAdvertised := got.Capabilities["event_stream"]
	_, interactiveAdvertised := got.Capabilities["interactive_events"]
	if !got.Capabilities["sessions"] || !got.Capabilities["runs"] ||
		!got.Capabilities["events"] || !streamAdvertised || !interactiveAdvertised {
		t.Fatalf("capacitats incompletes: %+v", got.Capabilities)
	}
}

func TestHealthNoAnunciaEventsInteractiusSenseStore(t *testing.T) {
	s := hubTestServer(t)
	h := s.Hub()
	h.eventStore = nil
	w := httptest.NewRecorder()
	s.handleHealth(w, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("health: %d %s", w.Code, w.Body.String())
	}
	var got struct {
		Capabilities map[string]bool `json:"capabilities"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Capabilities["interactive_events"] || got.Capabilities["durable_events"] {
		t.Fatalf("no s'han d'anunciar events durables sense store: %+v", got.Capabilities)
	}
}

func TestEventsDurablesAmbCursor(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	s.Hub()
	s.mu.Lock()
	s.active = &activeAgent{ID: 7}
	s.mu.Unlock()
	s.recordActive("thinking", map[string]string{"text": "pas 1"})
	s.recordActive("tool_call", map[string]string{"name": "shell"})
	w := httptest.NewRecorder()
	s.handleEvents(w, httptest.NewRequest(http.MethodGet, "/api/v2/events?after=0&limit=10", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("events: %d %s", w.Code, w.Body.String())
	}
	var got struct {
		Events []struct {
			ID   uint64 `json:"id"`
			Text string `json:"text"`
		} `json:"events"`
		Next uint64 `json:"next"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Events) != 2 || got.Events[0].ID == 0 || got.Next != got.Events[1].ID {
		t.Fatalf("events incomplets: %+v", got)
	}
	w2 := httptest.NewRecorder()
	s.handleEvents(w2, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/events?after=%d", got.Next), nil))
	if w2.Code != http.StatusOK || strings.Contains(w2.Body.String(), "pas 1") {
		t.Fatalf("cursor no avança: %d %s", w2.Code, w2.Body.String())
	}
}

func TestRespostaDurableNoEsTrunca(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	s.Hub()
	s.mu.Lock()
	s.active = &activeAgent{ID: 8}
	s.mu.Unlock()
	full := strings.Repeat("Resposta llarga. ", 100) + "Fi."
	s.recordActive("assistant", map[string]string{"text": full})
	s.recordActive("done", map[string]string{"reply": full})
	w := httptest.NewRecorder()
	s.handleEvents(w, httptest.NewRequest(http.MethodGet, "/api/v2/events?after=0&limit=10", nil))
	var got struct {
		Events []struct {
			Text string `json:"text"`
		} `json:"events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Events) != 2 || got.Events[0].Text != full || got.Events[1].Text != full {
		t.Fatalf("resposta truncada: %d events", len(got.Events))
	}
}

func TestEventsInteractiusConservenPayloadSegur(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	h := s.Hub()
	s.mu.Lock()
	s.active = &activeAgent{ID: 11}
	s.mu.Unlock()
	s.recordActive("approve_request", map[string]any{
		"key": "7", "call_id": "call-1", "name": "shell", "args": "ls",
		"secret": "no-desar",
	})
	s.recordActive("question_request", map[string]any{
		"key": "q2", "call_id": "call-2", "query": "Què faig?",
		"options": []map[string]string{{"label": "sí", "description": "continua"}},
		"secret":  "no-desar",
	})
	got, err := h.eventStore.AfterScope(0, 10, "", s.id)
	if err != nil || len(got) != 2 {
		t.Fatalf("events interactius: %+v %v", got, err)
	}
	for _, e := range got {
		if len(e.Payload) == 0 {
			t.Fatalf("manca payload a %s: %+v", e.Kind, e)
		}
		var payload map[string]any
		if err := json.Unmarshal(e.Payload, &payload); err != nil {
			t.Fatalf("payload %s: %v", e.Kind, err)
		}
		if _, found := payload["secret"]; found {
			t.Fatalf("s'ha persistit un camp no autoritzat: %s", e.Payload)
		}
		if e.Kind == "approve_request" {
			for _, key := range []string{"key", "call_id", "name", "args"} {
				if _, ok := payload[key]; !ok {
					t.Fatalf("approve sense %s: %s", key, e.Payload)
				}
			}
		} else if e.Kind == "question_request" {
			for _, key := range []string{"key", "call_id", "query", "options"} {
				if _, ok := payload[key]; !ok {
					t.Fatalf("question sense %s: %s", key, e.Payload)
				}
			}
		}
	}
}

func TestCheckpointDurableReprodueixPayloadEstructurat(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	s.Hub()
	s.mu.Lock()
	s.active = &activeAgent{ID: 12}
	s.mu.Unlock()

	longOutput := strings.Repeat("x", 1200)
	checkpoint := map[string]any{
		"number": 3,
		"checks": []map[string]any{
			{"command": "go test ./...", "code": 0, "output": longOutput},
			{"command": "go vet ./...", "code": 1, "output": "vet: issue"},
		},
		"review": "CAL REVISAR: revisar error",
		"secret": "no-desar",
	}
	s.recordActive("autonomous_checkpoint", checkpoint)

	w := httptest.NewRecorder()
	s.handleEvents(w, httptest.NewRequest(http.MethodGet, "/api/v2/events?after=0&limit=10", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("events replay: %d %s", w.Code, w.Body.String())
	}
	var got struct {
		Events []struct {
			Kind    string          `json:"kind"`
			Text    string          `json:"text"`
			Payload json.RawMessage `json:"payload"`
		} `json:"events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := `{"checks":[{"code":0,"command":"go test ./...","output":"` + longOutput + `"},{"code":1,"command":"go vet ./...","output":"vet: issue"}],"number":3,"review":"CAL REVISAR: revisar error"}`
	if len(got.Events) != 1 || got.Events[0].Kind != "autonomous_checkpoint" {
		t.Fatalf("checkpoint absent del replay: %+v", got.Events)
	}
	if string(got.Events[0].Payload) != want {
		t.Fatalf("payload de checkpoint perdut o alterat:\n got %s\nwant %s", got.Events[0].Payload, want)
	}
	if len([]rune(got.Events[0].Text)) > 901 {
		t.Fatalf("el text resum no s'ha retallat: %d runes", len([]rune(got.Events[0].Text)))
	}
}

// La capçalera i el query param trien sessió; ids invàlids cauen a default.
func TestSessionID(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/state", nil)
	r.Header.Set("X-Gregal-Session", "feina-2")
	if got := sessionID(r); got != "feina-2" {
		t.Fatalf("capçalera: %q", got)
	}
	r2 := httptest.NewRequest("GET", "/api/state?session=s123", nil)
	if got := sessionID(r2); got != "s123" {
		t.Fatalf("query: %q", got)
	}
	r3 := httptest.NewRequest("GET", "/api/state?session=../etc", nil)
	if got := sessionID(r3); got != DefaultSession {
		t.Fatalf("id invàlid ha de caure a default: %q", got)
	}
}

// Dues sessions tenen converses independents i no es veuen entre elles.
func TestSessionsIndependents(t *testing.T) {
	h := hubTestServer(t).Hub()
	a := h.Session("a")
	b := h.Session("b")
	if a == b {
		t.Fatal("dues sessions diferents no poden ser el mateix objecte")
	}
	a.convo = append(a.convo, llm.Message{Role: "user", Content: "hola"})
	if len(b.convo) != 0 {
		t.Fatalf("la conversa de b s'ha contaminat: %d", len(b.convo))
	}
	// La configuració (providers, models) sí que és compartida.
	if a.cfg != b.cfg {
		t.Fatal("el config ha de ser compartit entre sessions")
	}
	// I la sessió es reutilitza pel mateix id.
	if h.Session("a") != a {
		t.Fatal("el mateix id ha de tornar la mateixa sessió")
	}
}

// Un agent ocupat a una sessió no bloqueja l'altra: això és el 409 que G1
// venia a treure.
func TestAgentBusyNoBloquejaAltresSessions(t *testing.T) {
	h := hubTestServer(t).Hub()
	a := h.Session("a")
	a.mu.Lock()
	a.agentBusy = true
	a.mu.Unlock()
	b := h.Session("b")
	b.mu.Lock()
	busy := b.agentBusy
	b.mu.Unlock()
	if busy {
		t.Fatal("b no hauria d'estar ocupada")
	}
}

// Obrir, llistar i tancar sessions per l'API.
func TestObrirLlistarTancarSessions(t *testing.T) {
	h := hubTestServer(t).Hub()
	w := httptest.NewRecorder()
	h.handleOpenSession(w, httptest.NewRequest("POST", "/api/sessions/open", strings.NewReader(`{"id":"feina","title":"Feina"}`)))
	if w.Code != 200 {
		t.Fatalf("open: %d %s", w.Code, w.Body.String())
	}
	var open map[string]any
	json.Unmarshal(w.Body.Bytes(), &open)
	if open["id"] != "feina" || open["title"] != "Feina" {
		t.Fatalf("open: %v", open)
	}

	w2 := httptest.NewRecorder()
	h.handleLiveSessions(w2, httptest.NewRequest("GET", "/api/sessions/live", nil))
	var live struct {
		Sessions []map[string]any `json:"sessions"`
		Current  string           `json:"current"`
	}
	json.Unmarshal(w2.Body.Bytes(), &live)
	if len(live.Sessions) != 2 || live.Current != DefaultSession {
		t.Fatalf("live: %+v", live)
	}

	w3 := httptest.NewRecorder()
	h.handleCloseSession(w3, httptest.NewRequest("POST", "/api/sessions/close", strings.NewReader(`{"id":"feina"}`)))
	if w3.Code != 200 {
		t.Fatalf("close: %d", w3.Code)
	}
	if len(h.list()) != 1 {
		t.Fatalf("hauria de quedar només la default: %v", h.list())
	}
	// La default no es tanca mai.
	w4 := httptest.NewRecorder()
	h.handleCloseSession(w4, httptest.NewRequest("POST", "/api/sessions/close", strings.NewReader(`{"id":"default"}`)))
	if w4.Code != 400 {
		t.Fatalf("la default no s'ha de poder tancar: %d", w4.Code)
	}
}

func TestPinSessionNoClonaLaConversa(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	if _, err := session.SaveSession(s.sessDir(), session.Session{
		Name: "feina", Role: "code", Workspace: s.cwd,
		Convo: []llm.Message{{Role: "user", Content: "revisa el projecte"}},
	}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handlePinSession(rec, httptest.NewRequest("POST", "/api/sessions/pin", strings.NewReader(`{"name":"feina","pinned":true}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("pin: %d %s", rec.Code, rec.Body.String())
	}
	loaded, err := session.Load(s.sessDir(), "feina")
	if err != nil || !loaded.Pinned {
		t.Fatalf("pin no persistit: err=%v sess=%+v", err, loaded)
	}
	infos, err := session.List(s.sessDir())
	if err != nil || len(infos) != 1 || !infos[0].Pinned {
		t.Fatalf("llistat després del pin: err=%v infos=%+v", err, infos)
	}
}

func TestPinSessionActualitzaPestanyaViva(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	h := s.Hub()
	clone := h.Session("feina")
	if _, err := session.SaveSession(s.sessDir(), session.Session{
		Name: "feina", Role: "code", Workspace: s.cwd,
		Convo: []llm.Message{{Role: "user", Content: "continua"}},
	}); err != nil {
		t.Fatal(err)
	}
	clone.mu.Lock()
	clone.convoFile = "feina"
	clone.pinned = false
	clone.mu.Unlock()
	rec := httptest.NewRecorder()
	s.handlePinSession(rec, httptest.NewRequest("POST", "/api/sessions/pin", strings.NewReader(`{"name":"feina","pinned":true}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("pin: %d %s", rec.Code, rec.Body.String())
	}
	clone.mu.Lock()
	pinned := clone.pinned
	clone.mu.Unlock()
	if !pinned {
		t.Fatal("la pestanya viva conserva el pin antic i el pròxim autosave el perdria")
	}
}

func TestPinSessionConcurrentNoBloqueja(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	if _, err := session.SaveSession(s.sessDir(), session.Session{
		Name: "feina", Role: "code", Workspace: s.cwd,
		Convo: []llm.Message{{Role: "user", Content: "concurrent"}},
	}); err != nil {
		t.Fatal(err)
	}
	clone := s.Hub().Session("feina")
	clone.mu.Lock()
	clone.convoFile = "feina"
	clone.mu.Unlock()
	done := make(chan struct{}, 2)
	for _, pinned := range []bool{true, false} {
		go func(pinned bool) {
			body := fmt.Sprintf(`{"name":"feina","pinned":%t}`, pinned)
			s.handlePinSession(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/sessions/pin", strings.NewReader(body)))
			done <- struct{}{}
		}(pinned)
	}
	deadline := time.After(2 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-deadline:
			t.Fatal("pin concurrent bloquejat")
		}
	}
}

func TestModelDefaultsAreInheritedPerUserAndConversationOverridesResume(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	user := &User{Name: "alice"}
	h := s.Hub()
	first := h.SessionFor(user, "first")
	first.modelOverride["chat"] = "p/model-first"
	if err := session.SaveModelDefault(user.Name, "chat", "p/model-first"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.SaveSession(first.sessDir(), session.Session{
		Name: "saved", Role: "chat",
		ModelOverrides: map[string]string{"chat": "p/model-first"},
		Convo:          []llm.Message{{Role: "user", Content: "keep this model"}},
	}); err != nil {
		t.Fatal(err)
	}

	second := h.SessionFor(user, "second")
	if got := second.modelOverride["chat"]; got != "p/model-first" {
		t.Fatalf("new conversation did not inherit user default: %q", got)
	}
	if err := session.SaveModelDefault(user.Name, "chat", "p/model-latest"); err != nil {
		t.Fatal(err)
	}
	third := h.SessionFor(user, "third")
	if got := third.modelOverride["chat"]; got != "p/model-latest" {
		t.Fatalf("later conversation did not inherit latest model: %q", got)
	}
	if got := second.modelOverride["chat"]; got != "p/model-first" {
		t.Fatalf("existing conversation changed with the default: %q", got)
	}
	restarted := New(s.cfg, s.cfgPath)
	if got := restarted.Hub().SessionFor(user, "after-restart").modelOverride["chat"]; got != "p/model-latest" {
		t.Fatalf("model default did not survive backend restart: %q", got)
	}

	rec := httptest.NewRecorder()
	first.handleResume(rec, httptest.NewRequest(http.MethodPost, "/api/resume", strings.NewReader(`{"name":"saved"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("resume: %d %s", rec.Code, rec.Body.String())
	}
	if got := first.modelOverride["chat"]; got != "p/model-first" {
		t.Fatalf("resume did not restore conversation's model: %q", got)
	}
}

func TestResumeNoCanviaConversaSiWorkspaceNoExisteix(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	if _, err := session.SaveSession(s.sessDir(), session.Session{
		Name: "trencada", Role: "code", Workspace: filepath.Join(t.TempDir(), "ja-no-hi-es"),
		Convo: []llm.Message{{Role: "user", Content: "antiga"}},
	}); err != nil {
		t.Fatal(err)
	}
	s.convo = []llm.Message{{Role: "user", Content: "actual"}}
	rec := httptest.NewRecorder()
	s.handleResume(rec, httptest.NewRequest("POST", "/api/resume", strings.NewReader(`{"name":"trencada"}`)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("resume hauria de refusar workspace absent: %d %s", rec.Code, rec.Body.String())
	}
	if len(s.convo) != 1 || s.convo[0].Content != "actual" {
		t.Fatalf("la conversa activa ha canviat després d'un resume refusat: %+v", s.convo)
	}
}

// Cancel·lar sense res en marxa no és un error; amb torn actiu, el talla.
func TestCancelTorn(t *testing.T) {
	s := hubTestServer(t)
	w := httptest.NewRecorder()
	s.handleCancel(w, httptest.NewRequest("POST", "/api/agent/cancel", nil))
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	if out["cancelled"] != false {
		t.Fatalf("sense torn: %v", out)
	}
	ctx, end := s.runContext()
	defer end()
	w2 := httptest.NewRecorder()
	s.handleCancel(w2, httptest.NewRequest("POST", "/api/agent/cancel", nil))
	json.Unmarshal(w2.Body.Bytes(), &out)
	if out["cancelled"] != true {
		t.Fatalf("amb torn: %v", out)
	}
	if ctx.Err() == nil {
		t.Fatal("el context del torn s'havia de cancel·lar")
	}
}

// El workspace es pot canviar per sessió i els permisos el segueixen.
func TestWorkspacePerSessio(t *testing.T) {
	setHomeTest(t, t.TempDir())
	h := hubTestServer(t).Hub()
	a, b := h.Session("a"), h.Session("b")
	dirA, dirB := t.TempDir(), t.TempDir()
	if err := a.setWorkspace(dirA); err != nil {
		t.Fatal(err)
	}
	if err := b.setWorkspace(dirB); err != nil {
		t.Fatal(err)
	}
	if a.cwd == b.cwd {
		t.Fatal("cada sessió ha de tenir el seu workspace")
	}
	if a.policy.ProjectDir != a.cwd {
		t.Fatalf("els permisos no segueixen el workspace: %s vs %s", a.policy.ProjectDir, a.cwd)
	}
	if err := a.setWorkspace(filepath.Join(dirA, "no-existeix")); err == nil {
		t.Fatal("un directori inexistent ha de fallar")
	}
	// El fitxer de recents s'ha escrit i conté els dos.
	raw, err := os.ReadFile(workspacesPath())
	if err != nil {
		t.Fatalf("recents: %v", err)
	}
	if !strings.Contains(string(raw), filepath.Base(dirB)) {
		t.Fatalf("recents sense dirB: %s", raw)
	}
}

// inWorkspace només accepta el que penja del directori de la sessió.
func TestInWorkspace(t *testing.T) {
	setHomeTest(t, t.TempDir())
	s := hubTestServer(t)
	dir := t.TempDir()
	if err := s.setWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	if !s.inWorkspace(filepath.Join(dir, "sub", "x.go")) {
		t.Fatal("un fitxer de dins hauria de comptar")
	}
	if s.inWorkspace(filepath.Join(filepath.Dir(dir), "fora.go")) {
		t.Fatal("un fitxer de fora no hauria de comptar")
	}
}

// L'API de workspaces llista el recent i el canvia.
func TestHandleWorkspaces(t *testing.T) {
	setHomeTest(t, t.TempDir())
	s := hubTestServer(t)
	dir := t.TempDir()
	w := httptest.NewRecorder()
	s.handleWorkspaces(w, httptest.NewRequest("POST", "/api/workspaces", strings.NewReader(`{"path":"`+strings.ReplaceAll(dir, `\`, `\\`)+`"}`)))
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var out struct {
		Current    string `json:"current"`
		Workspaces []struct {
			Path string `json:"path"`
		} `json:"workspaces"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Current != dir || len(out.Workspaces) == 0 {
		t.Fatalf("%+v", out)
	}
	w2 := httptest.NewRecorder()
	s.handleWorkspaces(w2, httptest.NewRequest("POST", "/api/workspaces", strings.NewReader(`{"path":"/no/existeix/ai"}`)))
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("directori inexistent: %d", w2.Code)
	}
}
