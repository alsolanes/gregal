package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/jobs"
)

// failingProvider és un proveïdor de model que sempre falla i compta les
// crides: si un recordatori simple el toca, el test ho veu.
func failingProvider(t *testing.T, s *Server) *atomic.Int64 {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "model down", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	s.mu.Lock()
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: srv.URL}}
	s.mu.Unlock()
	return &hits
}

func b8Post(t *testing.T, h http.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw)))
	return rec
}

// Acceptació B8: un recordatori simple es dispara pel camí real del
// programador (runJob → cua → executeWindRemind) amb el model caigut, i no fa
// cap crida al model, ni tan sols en segon pla per aprendre'n.
func TestB8SimpleReminderFiresWithoutLLM(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	hits := failingProvider(t, s)
	j, err := saveWindReminder(s, "xaloc", "+1h", "Truca a l'àvia")
	if err != nil {
		t.Fatal(err)
	}
	// Arriba l'hora.
	past := time.Now().Add(-time.Second)
	if err := s.jobs.SetNextAt(j.ID, past); err != nil {
		t.Fatal(err)
	}
	run, err := s.runJob(j.ID, false)
	if err != nil || run.Status != "ok" {
		t.Fatalf("run = %+v err=%v", run, err)
	}
	waitBackground()
	if n := hits.Load(); n != 0 {
		t.Fatalf("un recordatori simple ha cridat el model %d cops", n)
	}
	found := false
	for _, m := range windHistory(s, "xaloc").messages {
		if m.Role == "assistant" && m.Content == "Truca a l'àvia" && m.ID != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("el text desat no ha entrat al fil del vent")
	}
	// Sense model configurat (client absent) també sona.
	j2, err := saveWindReminder(s, "xaloc", "+1h", "Rega")
	if err != nil {
		t.Fatal(err)
	}
	_ = s.jobs.SetNextAt(j2.ID, past)
	s.mu.Lock()
	s.client = nil
	s.mu.Unlock()
	if run, err := s.runJob(j2.ID, false); err != nil || run.Status != "ok" {
		t.Fatalf("sense client: %+v %v", run, err)
	}
	waitBackground()
}

// Editar, ajornar, reprogramar, pausar i cancel·lar treballen sobre el mateix
// id; ajornar un recordatori que ja ha sonat el torna a deixar pendent.
func TestB8RemindEditKeepsID(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	j, err := saveWindReminder(s, "xaloc", "+2h", "Compra pa")
	if err != nil {
		t.Fatal(err)
	}
	get := func() jobs.Job {
		list, _ := s.jobs.List()
		for _, x := range list {
			if x.ID == j.ID {
				return x
			}
		}
		t.Fatalf("el job %s ha desaparegut", j.ID)
		return jobs.Job{}
	}
	rec := b8Post(t, s.handleWindsRemindEdit, map[string]any{"id": j.ID, "op": "edit", "text": "Compra pa i llet"})
	if rec.Code != 200 {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		ID   string         `json:"id"`
		Item AutomationItem `json:"item"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ID != j.ID || resp.Item.ID != j.ID || resp.Item.Text != "Compra pa i llet" {
		t.Fatalf("edit resposta: %s", rec.Body)
	}
	if got := get(); got.Prompt != "Compra pa i llet" || !got.NextAt.Equal(*j.NextAt) {
		t.Fatalf("edit de text ha tocat l'hora o no ha desat: %+v", got)
	}
	// Hora absoluta a la zona de l'usuari: «09:00» a Tòquio.
	rec = b8Post(t, s.handleWindsRemindEdit, map[string]any{"id": j.ID, "op": "reschedule", "at": "09:00", "tz": "Asia/Tokyo"})
	if rec.Code != 200 {
		t.Fatalf("reschedule: %d %s", rec.Code, rec.Body)
	}
	tokyo, _ := time.LoadLocation("Asia/Tokyo")
	if at := get().NextAt.In(tokyo); at.Hour() != 9 || at.Minute() != 0 {
		t.Fatalf("09:00 a Tòquio ha quedat %s", at)
	}
	// Dispara'l (simulat) i ajorna'l: torna pendent amb el mateix id.
	fired := get()
	fired.Enabled = false
	_, _ = s.jobs.Save(fired)
	_ = s.jobs.MarkRun(j.ID, time.Now(), "ok")
	rec = b8Post(t, s.handleWindsRemindEdit, map[string]any{"id": j.ID, "op": "snooze", "minutes": 15})
	if rec.Code != 200 {
		t.Fatalf("snooze: %d %s", rec.Code, rec.Body)
	}
	got := get()
	if !got.Enabled || got.NextAt.Sub(time.Now()) < 14*time.Minute || got.NextAt.Sub(time.Now()) > 16*time.Minute {
		t.Fatalf("snooze: %+v", got)
	}
	if jobs.NextDue(got, time.Now()).IsZero() {
		t.Fatal("ajornat però el programador no el tornarà a disparar")
	}
	// Errors clars i cap canvi.
	for _, bad := range []map[string]any{
		{"id": j.ID, "op": "reschedule", "at": "2020-01-01T00:00:00Z"},
		{"id": j.ID, "op": "edit"},
		{"id": j.ID, "op": "snooze", "minutes": 99999},
		{"id": j.ID, "op": "explode"},
	} {
		if rec := b8Post(t, s.handleWindsRemindEdit, bad); rec.Code != 400 {
			t.Fatalf("%v: %d %s", bad, rec.Code, rec.Body)
		}
	}
	for _, op := range []string{"pause", "resume"} {
		if rec := b8Post(t, s.handleWindsRemindEdit, map[string]any{"id": j.ID, "op": op}); rec.Code != 200 {
			t.Fatalf("%s: %d", op, rec.Code)
		}
	}
	if rec := b8Post(t, s.handleWindsRemindEdit, map[string]any{"id": j.ID, "op": "cancel"}); rec.Code != 200 {
		t.Fatalf("cancel: %d", rec.Code)
	}
	if list, _ := s.jobs.List(); len(list) != 0 {
		t.Fatalf("cancel no ha esborrat: %+v", list)
	}
}

// b8Identity és una sessió d'un altre compte sobre la mateixa cua de jobs.
func b8Identity(t *testing.T, base *Server, name string) *Server {
	t.Helper()
	sess := remindTestServer(t)
	sess.jobs = base.jobs
	sess.user = &User{Name: name}
	sess.winds = companion.NewWindStore(t.TempDir())
	if _, err := sess.winds.Save(companion.Wind{Name: "Xaloc", Color: "#EFA68F"}); err != nil {
		t.Fatal(err)
	}
	return sess
}

// Segona identitat: amb el mateix vent (mateix id) i la mateixa cua, ningú no
// pot tocar els recordatoris ni les rutines d'un altre compte.
func TestB8SecondIdentityCannotTouch(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	base := remindTestServer(t)
	compteA := b8Identity(t, base, "usuari-a")
	compteB := b8Identity(t, base, "usuari-b")
	rem, err := saveWindReminder(compteA, "xaloc", "+3h", "Vacuna del gat")
	if err != nil {
		t.Fatal(err)
	}
	rec := b8Post(t, compteA.handleWindsRutina, map[string]any{"wind": "xaloc", "kind": "daily", "time": "08:00", "prompt": "Preus de vols"})
	if rec.Code != 200 {
		t.Fatalf("rutina: %d %s", rec.Code, rec.Body)
	}
	var created struct{ Job RutinaInfo }
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	for _, op := range []map[string]any{
		{"id": rem.ID, "op": "edit", "text": "hackejat"},
		{"id": rem.ID, "op": "snooze"},
		{"id": rem.ID, "op": "cancel"},
	} {
		if rec := b8Post(t, compteB.handleWindsRemindEdit, op); rec.Code != 404 {
			t.Fatalf("compteB %v: %d %s", op, rec.Code, rec.Body)
		}
	}
	if _, err := compteB.remindEdit(rem.ID, "cancel", ""); err == nil {
		t.Fatal("l'eina del vent de compteB ha cancel·lat el recordatori d'compteA")
	}
	if rec := b8Post(t, compteB.handleWindsRutinaTeach, map[string]any{"id": created.Job.ID, "instruction": "x"}); rec.Code != 404 {
		t.Fatalf("teach aliè: %d", rec.Code)
	}
	if rec := b8Post(t, compteB.handleWindsRutinaEdit, map[string]any{"id": created.Job.ID, "prompt": "x"}); rec.Code != 404 {
		t.Fatalf("edit aliè: %d", rec.Code)
	}
	if _, err := compteB.rutinaTeachTool("xaloc", "", "fes-ho curt", false); err == nil {
		t.Fatal("l'eina de compteB ha trobat la rutina d'compteA")
	}
	list, _ := base.jobs.List()
	for _, j := range list {
		if j.ID == rem.ID && j.Prompt != "Vacuna del gat" {
			t.Fatalf("recordatori alterat: %+v", j)
		}
		if j.ID == created.Job.ID && (j.Prompt != "Preus de vols" || j.Howto != "") {
			t.Fatalf("rutina alterada: %+v", j)
		}
	}
}

// Acceptació B8: una rutina pausada no corre — ni pel programador, ni per
// runJob desatès, ni pel botó «fes-ho ara».
func TestB8PausedRoutineDoesNotRun(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	hits := failingProvider(t, s)
	old := time.Now().Add(-48 * time.Hour)
	j, err := s.jobs.Save(jobs.Job{ID: "rutina-xaloc-1", Name: "Vols", Prompt: "Mira vols", Kind: "interval",
		IntervalH: 1, Enabled: false, Wind: "xaloc", LastAt: &old})
	if err != nil {
		t.Fatal(err)
	}
	if jobs.Due(j, time.Now()) {
		t.Fatal("Due diu que una pausada toca")
	}
	s.runDueJobs()
	if _, err := s.runJob(j.ID, false); err == nil {
		t.Fatal("runJob desatès ha corregut una rutina pausada")
	}
	if rec := b8Post(t, s.handleWindsRutinaRun, map[string]any{"id": j.ID}); rec.Code != http.StatusConflict {
		t.Fatalf("fes-ho ara amb pausa: %d %s", rec.Code, rec.Body)
	}
	time.Sleep(50 * time.Millisecond)
	waitBackground()
	if runs, _ := s.jobs.Runs(j.ID, 5); len(runs) != 0 {
		t.Fatalf("hi ha runs d'una rutina pausada: %+v", runs)
	}
	if hits.Load() != 0 {
		t.Fatal("una rutina pausada ha cridat el model")
	}
}

// «Fes-ho així la propera vegada»: instrucció + resultat d'exemple desats a la
// mateixa rutina, visibles, editables i dins la consigna de la rutina.
func TestB8TeachRoutineIsReviewable(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	rec := b8Post(t, s.handleWindsRutina, map[string]any{"wind": "xaloc", "kind": "daily", "time": "08:00", "prompt": "Resum de vols"})
	var created struct{ Job RutinaInfo }
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	id := created.Job.ID
	_ = s.jobs.AddRun(jobs.Run{ID: "run-1", JobID: id, Status: "ok", Output: "Vol BCN-LIS 89 €", StartedAt: time.Now(), FinishedAt: time.Now()})

	rec = b8Post(t, s.handleWindsRutinaTeach, map[string]any{"id": id, "instruction": "Posa només els tres més barats, en una taula", "ref_last": true})
	if rec.Code != 200 {
		t.Fatalf("teach: %d %s", rec.Code, rec.Body)
	}
	// Repetir la mateixa instrucció no la duplica.
	_ = b8Post(t, s.handleWindsRutinaTeach, map[string]any{"id": id, "instruction": "Posa només els tres més barats, en una taula"})
	var taught struct{ Job RutinaInfo }
	rec = b8Post(t, s.handleWindsRutinaTeach, map[string]any{"id": id, "instruction": "Sense emojis"})
	_ = json.Unmarshal(rec.Body.Bytes(), &taught)
	if taught.Job.ID != id || taught.Job.Howto != "- Posa només els tres més barats, en una taula\n- Sense emojis" || len(taught.Job.Refs) != 1 || taught.Job.Refs[0].Excerpt != "Vol BCN-LIS 89 €" {
		t.Fatalf("teach: %+v", taught.Job)
	}
	j, _ := s.ownWindRutina(id)
	brief := rutinaHowtoBrief(j)
	if !strings.Contains(brief, "Sense emojis") || !strings.Contains(brief, "Vol BCN-LIS 89 €") {
		t.Fatalf("la consigna no porta el que s'ha ensenyat: %q", brief)
	}
	// Visible a la llista d'automatitzacions (sencer, no retallat).
	items, _ := automationsGet(t, s, "")
	if len(items) != 1 || items[0].Howto == "" || items[0].Refs != 1 || items[0].Text != "Resum de vols" {
		t.Fatalf("automations: %+v", items)
	}
	// Editar: mateix id, instruccions corregides, exemple tret, nova hora.
	rec = b8Post(t, s.handleWindsRutinaEdit, map[string]any{"id": id, "howto": "- Sense emojis", "keep_refs": []string{}, "time": "07:30"})
	if rec.Code != 200 {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body)
	}
	j, _ = s.ownWindRutina(id)
	if j.ID != id || j.Howto != "- Sense emojis" || len(j.Refs) != 0 || j.Time != "07:30" || j.Prompt != "Resum de vols" {
		t.Fatalf("edit: %+v", j)
	}
	if rec := b8Post(t, s.handleWindsRutinaEdit, map[string]any{"id": id, "time": "25:99"}); rec.Code != 400 {
		t.Fatalf("hora dolenta: %d", rec.Code)
	}
	// Eina del vent: amb una sola rutina la troba sola i desa el darrer missatge.
	postWindMessage(s, mustWind(t, s), "chat", "Aquí tens els vols d'avui")
	waitBackground()
	msg, err := s.rutinaTeachTool("Xaloc", "", "Afegeix l'hora de sortida", true)
	if err != nil || !strings.Contains(msg, "Desat") {
		t.Fatalf("eina: %q %v", msg, err)
	}
	j, _ = s.ownWindRutina(id)
	if !strings.Contains(j.Howto, "hora de sortida") || len(j.Refs) != 1 || j.Refs[0].Kind != "message" {
		t.Fatalf("eina: %+v", j)
	}
	// Amb dues, demana quina.
	time.Sleep(1100 * time.Millisecond) // els ids de rutina són per segon
	_ = b8Post(t, s.handleWindsRutina, map[string]any{"wind": "xaloc", "kind": "interval", "interval_h": 6, "prompt": "Altres"})
	if _, err := s.rutinaTeachTool("xaloc", "", "x", false); err == nil || !strings.Contains(err.Error(), "routine") {
		t.Fatalf("amb dues rutines havia de demanar quina: %v", err)
	}
}

func mustWind(t *testing.T, s *Server) companion.Wind {
	t.Helper()
	w, ok := s.winds.ByRef("xaloc")
	if !ok {
		t.Fatal("cap vent")
	}
	return w
}

// Editar la memòria (preferències) d'un vent no toca credencials ni connexions.
func TestB8EditMemoryLeavesCredentials(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	auth := filepath.Join(t.TempDir(), "auth.json")
	t.Setenv("GREGAL_AUTH", auth)
	conn := s.connectionsPath()
	_ = os.MkdirAll(filepath.Dir(conn), 0o700)
	want := map[string][]byte{
		conn: []byte(`{"imap":{"host":"mail.example","user":"a","password":"secret"}}`),
		auth: []byte(`{"users":[{"name":"a","hash":"x"}]}`),
	}
	for p, b := range want {
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := json.Marshal(map[string]string{"wind": "xaloc", "memoria": "- Vol els informes curts\n- Sense emojis"})
	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	s.handleWindMemoria(rec, req)
	if rec.Code != 200 {
		t.Fatalf("memoria: %d %s", rec.Code, rec.Body)
	}
	for p, b := range want {
		got, err := os.ReadFile(p)
		if err != nil || !bytes.Equal(got, b) {
			t.Fatalf("%s ha canviat en editar la memòria", p)
		}
	}
	// I la preferència corregida arriba als encàrrecs del vent, no només al xat.
	if !strings.Contains(s.windMemorySystem("xaloc"), "Sense emojis") {
		t.Fatal("la memòria no entra a la consigna dels encàrrecs")
	}
}

// Dades existents: un jobs.json d'abans (sense howto/refs) es llegeix igual i
// desar-hi una altra cosa no en perd cap camp.
func TestB8LegacyJobsPreserved(t *testing.T) {
	dir := t.TempDir()
	legacy := `[{"id":"remind-xaloc-1","name":"Recordatori Xaloc","prompt":"ITV","kind":"remind","time":"","interval_h":0,"enabled":true,"notify":false,"created_at":"2026-01-01T00:00:00Z","next_at":"2099-01-01T09:00:00Z","wind":"xaloc"},
{"id":"rutina-xaloc-2","name":"Vols","prompt":"Mira vols","kind":"daily","time":"08:00","interval_h":0,"enabled":false,"notify":false,"created_at":"2026-01-01T00:00:00Z","wind":"xaloc","wind_owner":"usuari-a"}]`
	if err := os.WriteFile(filepath.Join(dir, "jobs.json"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := jobs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Save(jobs.Job{ID: "rutina-xaloc-3", Name: "Nova", Prompt: "p", Kind: "daily", Time: "09:00", Wind: "xaloc", Howto: "- curt"}); err != nil {
		t.Fatal(err)
	}
	list, _ := st.List()
	if len(list) != 3 {
		t.Fatalf("jobs: %+v", list)
	}
	for _, j := range list {
		switch j.ID {
		case "remind-xaloc-1":
			if j.Prompt != "ITV" || j.NextAt == nil || !j.Enabled {
				t.Fatalf("recordatori antic: %+v", j)
			}
		case "rutina-xaloc-2":
			if j.Enabled || j.WindOwner != "usuari-a" || j.Time != "08:00" {
				t.Fatalf("rutina antiga: %+v", j)
			}
		}
	}
}
