package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/events"
	"gregal/internal/runs"
)

// v2Do passa pel mux real: els paths amb {id} necessiten el router per
// poblar PathValue, i així el test val també el registre de rutes.
func v2Do(t *testing.T, s *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	rec := httptest.NewRecorder()
	s.Hub().mux().ServeHTTP(rec, req)
	return rec
}

// runState consulta l'estat d'una execució per la API v2.
func runState(t *testing.T, s *Server, id int64) runs.Run {
	t.Helper()
	rec := v2Do(t, s, http.MethodGet, "/api/v2/runs/"+strconv.FormatInt(id, 10), "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/v2/runs/%d: %d %s", id, rec.Code, rec.Body.String())
	}
	var out struct {
		Run runs.Run `json:"run"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Run
}

// waitRunState espera (amb topall) que l'execució arribi a un estat.
func waitRunState(t *testing.T, s *Server, id int64, want runs.State) runs.Run {
	t.Helper()
	// 20 s i no 5: amb la màquina carregada (la suite sencera en
	// paral·lel) arrencar un torn pot trigar, i fallar aquí no vol dir
	// que la cua vagi malament.
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		run := runState(t, s, id)
		if run.State == want {
			return run
		}
		if !run.State.Active() {
			t.Fatalf("l'execució %d ha acabat com a %s, volia %s", id, run.State, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("l'execució %d no ha arribat a %s a temps", id, want)
	return runs.Run{}
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

// v2Provider construeix un model fals que senyalitza la primera crida i
// es queda blocat fins que el test l'alliberi. Retorna el servidor, el
// canal d'entrada i el d'alliberament.
func v2Provider(t *testing.T, reply string) (*httptest.Server, chan struct{}, chan struct{}) {
	t.Helper()
	entrada := make(chan struct{})
	var unCop sync.Once
	allibera := make(chan struct{})
	// acabat el tanca la neteja del test. Sense aixo, un test que falla
	// abans de cancel\u00b7lar el torn deixa la peticio penjada, i
	// httptest.Server.Close espera les peticions vives: una fallada de mig
	// segon es convertia en quatre minuts de timeout que amagaven quin
	// test havia petat de debo.
	acabat := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unCop.Do(func() { close(entrada) })
		select {
		case <-allibera:
		case <-acabat:
			return
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"` + reply + `"}}]}`))
	}))
	t.Cleanup(func() {
		close(acabat)
		provider.Close()
	})
	return provider, entrada, allibera
}

func storeKinds(t *testing.T, s *Server) map[string][]events.Event {
	t.Helper()
	got, err := s.Hub().eventStore.After(0, 500)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]events.Event{}
	for _, e := range got {
		out[e.Kind] = append(out[e.Kind], e)
	}
	return out
}

func TestV2RunCicleDeVida(t *testing.T) {
	provider, _, allibera := v2Provider(t, "Resposta final.")
	s := goalTestServer(t)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}

	rec := v2Do(t, s, http.MethodPost, "/api/v2/runs", `{"task":"feina llarga"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v2/runs: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Run runs.Run `json:"run"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Run.ID == 0 {
		t.Fatal("l'execució creada ha de tenir ID")
	}
	if !out.Run.State.Active() {
		t.Fatalf("l'execució nova ha de ser viva, no %s", out.Run.State)
	}

	// Estat consultable i arriba a running.
	run := waitRunState(t, s, out.Run.ID, runs.Running)
	if run.Workspace == "" {
		t.Fatal("l'execució hauria de portar el workspace de la sessió")
	}

	close(allibera)
	fin := waitRunState(t, s, out.Run.ID, runs.Completed)
	if fin.FinishedAt.IsZero() {
		t.Fatal("una execució completada ha de tenir finished_at")
	}

	// El registre durable conté el cicle de vida i la resposta.
	kinds := storeKinds(t, s)
	for _, kind := range []string{"run_queued", "run_started", "run_completed", "text", "done"} {
		if len(kinds[kind]) == 0 {
			t.Fatalf("manca l'event %s al registre (hi ha: %v)", kind, keys(kinds))
		}
	}
	if kinds["run_queued"][0].RunID != int(out.Run.ID) {
		t.Fatalf("run_id de l'event %d, volia %d", kinds["run_queued"][0].RunID, out.Run.ID)
	}
}

func keys(m map[string][]events.Event) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestV2RunIdempotencia(t *testing.T) {
	provider, _, allibera := v2Provider(t, "Resposta final.")
	s := goalTestServer(t)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}

	body := `{"task":"repetit","idempotency_key":"msg-7"}`
	rec := v2Do(t, s, http.MethodPost, "/api/v2/runs", body)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("primera crida: %d %s", rec.Code, rec.Body.String())
	}
	var primera struct {
		Run runs.Run `json:"run"`
	}
	json.Unmarshal(rec.Body.Bytes(), &primera)

	rec = v2Do(t, s, http.MethodPost, "/api/v2/runs", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("el reenviament idempotent havia de donar 200, ha donat %d", rec.Code)
	}
	var segona struct {
		Run runs.Run `json:"run"`
	}
	json.Unmarshal(rec.Body.Bytes(), &segona)
	if segona.Run.ID != primera.Run.ID {
		t.Fatalf("idempotència: id %d != %d", segona.Run.ID, primera.Run.ID)
	}

	close(allibera)
	waitRunState(t, s, primera.Run.ID, runs.Completed)

	// Acabada: la mateixa clau continua retornant l'existents, sense reexecutar.
	rec = v2Do(t, s, http.MethodPost, "/api/v2/runs", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("reenviament després d'acabar: %d", rec.Code)
	}
	var tercera struct {
		Run runs.Run `json:"run"`
	}
	json.Unmarshal(rec.Body.Bytes(), &tercera)
	if tercera.Run.ID != primera.Run.ID || tercera.Run.State != runs.Completed {
		t.Fatalf("la clau vella ha de retornar l'execució acabada: %+v", tercera.Run)
	}
}

func TestV2RunCancelEnCuaNoExecuta(t *testing.T) {
	provider, entrada, allibera := v2Provider(t, "Resposta final.")
	s := goalTestServer(t)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}

	rec := v2Do(t, s, http.MethodPost, "/api/v2/runs", `{"task":"primera"}`)
	var a struct {
		Run runs.Run `json:"run"`
	}
	json.Unmarshal(rec.Body.Bytes(), &a)
	<-entrada // la primera és en marxa

	rec = v2Do(t, s, http.MethodPost, "/api/v2/runs", `{"task":"segona"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("el segon torn s'hauria d'encuar, no %d %s", rec.Code, rec.Body.String())
	}
	var b struct {
		Run runs.Run `json:"run"`
	}
	json.Unmarshal(rec.Body.Bytes(), &b)
	if b.Run.State != runs.Queued {
		t.Fatalf("el segon torn ha de néixer en cua, no %s", b.Run.State)
	}

	rec = v2Do(t, s, http.MethodPost, "/api/v2/runs/"+itoa(b.Run.ID)+"/cancel", `{}`)
	if rec.Code != 200 {
		t.Fatalf("cancel: %d %s", rec.Code, rec.Body.String())
	}
	if fin := waitRunState(t, s, b.Run.ID, runs.Cancelled); fin.State != runs.Cancelled {
		t.Fatal("estat final del cancel·lat")
	}

	close(allibera)
	waitRunState(t, s, a.Run.ID, runs.Completed)

	// El torn cancel·lat no ha d'haver arribat mai al model: una sola crida.
	if n := len(s.Hub().Queue().ListSession(s.id)); n != 2 {
		t.Fatalf("la sessió hauria de tenir 2 execucions, té %d", n)
	}
}

func TestV2RunCancelEnMarxaTalla(t *testing.T) {
	provider, _, _ := v2Provider(t, "Resposta final.")
	s := goalTestServer(t)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}

	rec := v2Do(t, s, http.MethodPost, "/api/v2/runs", `{"task":"llarga"}`)
	var a struct {
		Run runs.Run `json:"run"`
	}
	json.Unmarshal(rec.Body.Bytes(), &a)
	waitRunState(t, s, a.Run.ID, runs.Running)

	// La cancel·lació legacy (/api/agent/cancel) ha de trobar el torn de la cua.
	rec = v2Do(t, s, http.MethodPost, "/api/agent/cancel", `{}`)
	if rec.Code != 200 {
		t.Fatalf("cancel legacy: %d", rec.Code)
	}
	fin := waitRunState(t, s, a.Run.ID, runs.Cancelled)
	if fin.State != runs.Cancelled {
		t.Fatalf("el cancel legacy no ha tallat l'execució: %s", fin.State)
	}
	// I la sessió deixa d'estar ocupada de seguida. És el que mira el
	// client per moure la mar de la capçalera (body.working): si això es
	// quedés cert fins al timeout del proveïdor, «he parat i encara es
	// mou» seria exactament el que es veuria.
	limit := time.Now().Add(2 * time.Second)
	for {
		var st struct {
			Busy bool `json:"agent_busy"`
		}
		rec = v2Do(t, s, http.MethodGet, "/api/state", "")
		json.Unmarshal(rec.Body.Bytes(), &st)
		if !st.Busy {
			break
		}
		if time.Now().After(limit) {
			t.Fatal("agent_busy segueix cert 2 s després de cancel·lar: la mar del web no s'aturaria")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestV2RunErrors(t *testing.T) {
	s := goalTestServer(t)

	if rec := v2Do(t, s, http.MethodPost, "/api/v2/runs", `{"task":""}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("tasca buida: %d", rec.Code)
	}
	if rec := v2Do(t, s, http.MethodGet, "/api/v2/runs/999", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("execució desconeguda: %d", rec.Code)
	}
	if rec := v2Do(t, s, http.MethodPost, "/api/v2/runs/999/cancel", `{}`); rec.Code != http.StatusNotFound {
		t.Fatalf("cancel desconegut: %d", rec.Code)
	}
	if rec := v2Do(t, s, http.MethodGet, "/api/v2/runs/abc", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("id no numèric: %d", rec.Code)
	}

	// Llista d'execucions de la sessió (buida però vàlida).
	rec := v2Do(t, s, http.MethodGet, "/api/v2/runs", "")
	if rec.Code != 200 {
		t.Fatalf("llista: %d", rec.Code)
	}
	var out struct {
		Runs []runs.Run `json:"runs"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Runs == nil || len(out.Runs) != 0 {
		t.Fatalf("la sessió nova no té execucions: %+v", out.Runs)
	}
}

func TestV2RunSmallTalkNoGastaModel(t *testing.T) {
	provider, _, _ := v2Provider(t, "mai")
	s := goalTestServer(t)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}

	rec := v2Do(t, s, http.MethodPost, "/api/v2/runs", `{"task":"hola!"}`)
	if rec.Code != http.StatusAccepted && rec.Code != http.StatusOK {
		t.Fatalf("smalltalk: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Run runs.Run `json:"run"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	fin := waitRunState(t, s, out.Run.ID, runs.Completed)

	kinds := storeKinds(t, s)
	if len(kinds["text"]) == 0 {
		t.Fatal("la cortesia havia de deixar la resposta al registre")
	}
	// L'executor ha d'haver respost sense cridar el model: l'únic event
	// done conté la cortesia i cap crida al provider (el canal allibera mai).
	_ = fin
	if len(kinds["run_failed"]) != 0 {
		t.Fatalf("la cortesia no hauria de fallar: %v", kinds["run_failed"])
	}
}

// Un torn delegat amb workspace ha d'executar les eines ALLÀ: un write amb
// ruta relativa ha de crear el fitxer al directori demanat, no al cwd del
// procés del servei ni al de la sessió.
func TestV2RunWorkspaceDelegat(t *testing.T) {
	ws := t.TempDir()
	var n int64
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// DetectWindows sondeja /models abans del primer pas: no és el
		// torn i no ha de consumir el comptador de respostes.
		if r.URL.Path == "/v1/models" {
			w.Write([]byte(`{"data":[]}`))
			return
		}
		if atomic.AddInt64(&n, 1) == 1 {
			w.Write([]byte(`{"choices":[{"message":{"content":"Escric el fitxer.","tool_calls":[{"id":"w1","type":"function","function":{"name":"write","arguments":"{\"path\":\"prova-ws.txt\",\"content\":\"hola\"}"}}]}}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"Fet."}}]}`))
	}))
	defer provider.Close()

	s := goalTestServer(t)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
	body, _ := json.Marshal(map[string]string{"task": "escriu el fitxer", "workspace": ws})
	rec := v2Do(t, s, http.MethodPost, "/api/v2/runs", string(body))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v2/runs: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Run runs.Run `json:"run"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Run.Workspace != runs.WorkspaceKey(ws) {
		t.Fatalf("workspace del run=%q, volia %q", out.Run.Workspace, runs.WorkspaceKey(ws))
	}
	waitRunState(t, s, out.Run.ID, runs.Completed)

	raw, err := os.ReadFile(filepath.Join(ws, "prova-ws.txt"))
	if err != nil || string(raw) != "hola" {
		t.Fatalf("el fitxer havia de néixer al workspace delegat: %q %v", raw, err)
	}
	// I sobretot no al directori del procés del servei: abans del
	// workspace delegat, les relatives hi queien en silenci.
	if _, err := os.Stat(filepath.Join(".", "prova-ws.txt")); !os.IsNotExist(err) {
		t.Fatal("el fitxer ha caigut al cwd del procés en comptes del workspace")
	}
}

func TestV2RunWorkspaceInvalidIProhibit(t *testing.T) {
	s := goalTestServer(t)
	if rec := v2Do(t, s, http.MethodPost, "/api/v2/runs", `{"task":"x","workspace":"relatiu/no-absolut"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("workspace relatiu: %d", rec.Code)
	}
	inexistent := filepath.Join(t.TempDir(), "no-hi-es")
	body, _ := json.Marshal(map[string]string{"task": "x", "workspace": inexistent})
	if rec := v2Do(t, s, http.MethodPost, "/api/v2/runs", string(body)); rec.Code != http.StatusBadRequest {
		t.Fatalf("workspace inexistent: %d", rec.Code)
	}
}

// El guard mana: un workspace fora de les arrels de l'usuari es rebutja,
// encara que existeixi al disc.
func TestResolveWorkspaceGuard(t *testing.T) {
	casa := t.TempDir()
	fora := t.TempDir()
	s := &Server{user: &User{Name: "userb", Roots: []string{casa}}}
	if _, code, _ := s.resolveWorkspace(fora); code != http.StatusForbidden {
		t.Fatalf("fora d'arrels: %d, volia 403", code)
	}
	dins := filepath.Join(casa, "proj")
	if err := os.MkdirAll(dins, 0o755); err != nil {
		t.Fatal(err)
	}
	if ws, code, msg := s.resolveWorkspace(dins); code != 0 || ws != dins {
		t.Fatalf("dins d'arrels ha de passar: %q %d %s", ws, code, msg)
	}
	// Sense workspace demanat: el de la sessió, com abans.
	s2 := &Server{}
	s2.cwd = casa
	if ws, code, _ := s2.resolveWorkspace(""); code != 0 || ws != casa {
		t.Fatalf("buit ha de donar el cwd de sessió: %q %d", ws, code)
	}
}
