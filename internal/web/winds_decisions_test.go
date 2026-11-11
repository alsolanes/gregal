package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/events"
	"gregal/internal/runs"
)

// B4 — decisions durables: una aprovació que espera tota la nit no reté cap
// goroutine, sobreviu a un reinici, s'aplica una sola vegada, es pot
// cancel·lar i, si l'acció canvia, cal decidir la revisió nova.

// decisionModel és un proveïdor fals: demana esborrar el camí que diu el
// missatge de la persona (una acció que sempre pregunta) i, quan rep el
// resultat de l'eina, respon amb el que ha passat.
func decisionModel(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "text/event-stream")
		last := req.Messages[len(req.Messages)-1]
		if last.Role == "tool" {
			text := fmt.Sprint(last.Content)
			reply := "Fet: " + text
			if strings.Contains(text, "NOT RUN") || strings.Contains(text, "BLOCKED") {
				reply = "No ho he fet."
			}
			b, _ := json.Marshal(reply)
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", b)
			return
		}
		target := ""
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if m := req.Messages[i]; m.Role == "user" {
				if c := fmt.Sprint(m.Content); strings.Contains(c, "ESBORRA ") {
					target = strings.TrimSpace(c[strings.Index(c, "ESBORRA ")+len("ESBORRA "):])
					break
				}
			}
		}
		args, _ := json.Marshal(map[string]string{"command": "rm -rf " + target})
		wire, _ := json.Marshal(string(args))
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"function\":{\"name\":\"bash\",\"arguments\":%s}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n", wire)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// toolRecorder substitueix l'execució d'eines: compta cada ordre aplicada.
type toolRecorder struct {
	mu    sync.Mutex
	ran   []string
	block chan struct{} // si no és nil, l'eina espera fins que es cancel·la
	start chan struct{}
}

func recordTools(t *testing.T) *toolRecorder {
	t.Helper()
	rec := &toolRecorder{}
	old := windToolExec
	windToolExec = func(ctx context.Context, _, _, name, args string) (string, []string, error) {
		var a struct {
			Command string `json:"command"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		rec.mu.Lock()
		rec.ran = append(rec.ran, a.Command)
		block, start := rec.block, rec.start
		rec.mu.Unlock()
		if block != nil {
			close(start)
			select {
			case <-ctx.Done():
				return "", nil, ctx.Err()
			case <-block:
			}
		}
		return "esborrat " + a.Command, nil, nil
	}
	t.Cleanup(func() { windToolExec = old })
	return rec
}

func (r *toolRecorder) commands() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.ran...)
}

func shortLive(t *testing.T) {
	t.Helper()
	old := windAskLive
	windAskLive = 40 * time.Millisecond
	t.Cleanup(func() { windAskLive = old })
}

func decisionServer(t *testing.T, model *httptest.Server) *Server {
	t.Helper()
	s := remindTestServer(t)
	s.cfg.Providers["p"] = config.Provider{BaseURL: model.URL + "/v1"}
	s.cfg.Agent.MaxSteps = 6
	s.cwd = t.TempDir()
	// Els fils viuen al directori de sessions compartit dels tests.
	wind, _ := s.winds.ByID("xaloc")
	if err := s.clearWindHistory(wind, windHistory(s, "xaloc")); err != nil {
		t.Fatal(err)
	}
	return s
}

// restartServer simula un reinici: el mateix config i les mateixes carpetes,
// però cap estat en memòria (cua, esperes, historials, procés).
func restartServer(t *testing.T, old *Server) *Server {
	t.Helper()
	windHistories.Range(func(k, _ any) bool { windHistories.Delete(k); return true })
	windLiveAsks.Lock()
	windLiveAsks.m = map[string]chan windAnswer{}
	windLiveAsks.Unlock()
	windProc = newMessageID()
	s := New(old.cfg, old.cfgPath)
	s.winds, s.jobs, s.cwd = old.winds, old.jobs, old.cwd
	s.goalDir = old.goalDir
	return s
}

func windChat(t *testing.T, s *Server, msg string) string {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"wind": "xaloc", "message": msg})
	rec := httptest.NewRecorder()
	s.handleWindChatStream(rec, httptest.NewRequest("POST", "/api/winds/chat/stream", strings.NewReader(string(b))))
	return rec.Body.String()
}

func waitRun(t *testing.T, s *Server, id int64) runs.Run {
	t.Helper()
	select {
	case <-s.Hub().Queue().Done(id):
	case <-time.After(10 * time.Second):
		t.Fatalf("el run %d no acaba", id)
	}
	waitBackground()
	run, _ := s.Hub().Queue().Get(id)
	return run
}

func runEvents(t *testing.T, s *Server, runID int64) []events.Event {
	t.Helper()
	all, err := s.Hub().eventStore.AfterUser(0, 500, s.usuari())
	if err != nil {
		t.Fatal(err)
	}
	var out []events.Event
	for _, e := range all {
		if int64(e.RunID) == runID {
			out = append(out, e)
		}
	}
	return out
}

func threadText(s *Server) string {
	h := windHistory(s, "xaloc")
	h.mu.RLock()
	defer h.mu.RUnlock()
	var b strings.Builder
	for _, m := range h.messages {
		b.WriteString(m.Role + ": " + m.Content + "\n")
	}
	return b.String()
}

func TestWindDecisionSurvivesRestartAndAppliesOnce(t *testing.T) {
	shortLive(t)
	tools := recordTools(t)
	model := decisionModel(t)
	s := decisionServer(t, model)
	victim := filepath.Join(t.TempDir(), "vell")

	// La persona demana una acció que cal aprovar i no respon: el torn
	// s'aparca i la petició HTTP acaba (cap goroutine esperant).
	raw := windChat(t, s, "ESBORRA "+victim)
	if !strings.Contains(raw, "event: approve_request") || !strings.Contains(raw, `"waiting"`) {
		t.Fatalf("el torn s'havia d'aparcar:\n%s", raw)
	}
	pend := pendingWindAsks(s)
	if len(pend) != 1 {
		t.Fatalf("pendents: %+v", pend)
	}
	d := pend[0]
	if d.State != "pending" || d.Revision != 1 || d.RunID == 0 || d.ProposedEffect == nil || d.ProposedEffect.Tool != "bash" || d.ProposedEffect.Hash == "" || d.MessageID == "" || d.ExpiresAt == "" {
		t.Fatalf("decisió: %+v", d)
	}
	if run, _ := s.Hub().Queue().Get(d.RunID); run.State != runs.WaitingUser {
		t.Fatalf("el run ha d'esperar la persona: %+v", run)
	}
	if len(tools.commands()) != 0 {
		t.Fatal("res no s'ha d'executar sense resposta")
	}
	if !strings.Contains(threadText(s), "ESBORRA") || !strings.Contains(threadText(s), "assistant: "+s.tr("Espero el teu permís", "Waiting for your permission")) {
		t.Fatalf("el fil ha de dir que espera:\n%s", threadText(s))
	}

	// Reinici del servidor entre la pregunta i la resposta.
	s = restartServer(t, s)
	if n := s.CloseOrphanRuns(); n != 0 {
		t.Fatalf("un run aparcat no és orfe: %d tancats", n)
	}
	pend = pendingWindAsks(s)
	if len(pend) != 1 || pend[0].ID != d.ID || pend[0].RunID != d.RunID {
		t.Fatalf("després del reinici: %+v", pend)
	}

	// Respondre continua el torn com un run i aplica l'efecte.
	rec := answerWindAsk(t, s, `{"key":"`+d.ID+`","approve":true}`)
	if rec.Code != 200 {
		t.Fatalf("respondre: %d %s", rec.Code, rec.Body)
	}
	if run := waitRun(t, s, d.RunID); run.State != runs.Completed {
		t.Fatalf("run reprès: %+v", run)
	}
	if got := tools.commands(); len(got) != 1 || got[0] != "rm -rf "+victim {
		t.Fatalf("efecte aplicat: %q", got)
	}
	if !strings.Contains(threadText(s), "Fet: esborrat rm -rf "+victim) {
		t.Fatalf("el resultat ha d'arribar al mateix fil:\n%s", threadText(s))
	}
	if strings.Count(threadText(s), "ESBORRA") != 1 {
		t.Fatalf("el missatge de la persona no es pot duplicar:\n%s", threadText(s))
	}

	// Resposta duplicada: res més.
	rec = answerWindAsk(t, s, `{"key":"`+d.ID+`","approve":true}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"duplicate":true`) {
		t.Fatalf("duplicada: %d %s", rec.Code, rec.Body)
	}
	waitBackground()
	time.Sleep(100 * time.Millisecond)
	if got := tools.commands(); len(got) != 1 {
		t.Fatalf("la resposta duplicada ha tornat a executar: %q", got)
	}
	if len(pendingWindAsks(s)) != 0 {
		t.Fatal("ja no queda res pendent")
	}

	// El registre d'events de sempre explica tot el cicle.
	var kinds []string
	for _, e := range runEvents(t, s, d.RunID) {
		k := e.Kind
		if e.Kind == events.Decision {
			k += ":" + e.Text
		}
		kinds = append(kinds, k)
	}
	joined := strings.Join(kinds, ",")
	for _, want := range []string{events.RunWaitingUser, "decision:pending", "decision:approved", "run_queued", "run_started", "run_completed"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("falta %s als events del run: %s", want, joined)
		}
	}
	if strings.Count(joined, "decision:approved") != 1 {
		t.Fatalf("una sola transició a approved: %s", joined)
	}
}

func TestWindDecisionRejectedDoesNotRun(t *testing.T) {
	shortLive(t)
	tools := recordTools(t)
	s := decisionServer(t, decisionModel(t))
	windChat(t, s, "ESBORRA "+filepath.Join(t.TempDir(), "x"))
	d := pendingWindAsks(s)[0]
	if rec := answerWindAsk(t, s, `{"key":"`+d.ID+`","approve":false}`); rec.Code != 200 {
		t.Fatalf("rebutjar: %d", rec.Code)
	}
	waitRun(t, s, d.RunID)
	if got := tools.commands(); len(got) != 0 {
		t.Fatalf("rebutjat i executat: %q", got)
	}
	if !strings.Contains(threadText(s), "No ho he fet.") {
		t.Fatalf("el vent ha de saber que s'ha rebutjat:\n%s", threadText(s))
	}
}

// Una acció revisada (mateix fil, mateixa eina i tipus, arguments nous)
// substitueix la decisió vella: la revisió 1 ja no aplica res.
func TestWindDecisionRevisedNeedsNewRevision(t *testing.T) {
	shortLive(t)
	tools := recordTools(t)
	s := decisionServer(t, decisionModel(t))
	first := filepath.Join(t.TempDir(), "primer")
	second := filepath.Join(t.TempDir(), "segon")
	windChat(t, s, "ESBORRA "+first)
	d1 := pendingWindAsks(s)[0]
	windChat(t, s, "ESBORRA "+second)
	pend := pendingWindAsks(s)
	if len(pend) != 1 || pend[0].ID == d1.ID || pend[0].Revision != 2 {
		t.Fatalf("revisió nova: %+v", pend)
	}
	d2 := pend[0]
	// El run de la revisió vella s'ha aturat.
	if run, _ := s.Hub().Queue().Get(d1.RunID); run.State != runs.Cancelled {
		t.Fatalf("run vell: %+v", run)
	}
	rec := answerWindAsk(t, s, `{"key":"`+d1.ID+`","approve":true}`)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"superseded"`) || !strings.Contains(rec.Body.String(), d2.ID) {
		t.Fatalf("aprovar la revisió vella: %d %s", rec.Code, rec.Body)
	}
	// Una revisió explícita que no és l'actual tampoc val.
	if rec := answerWindAsk(t, s, `{"key":"`+d2.ID+`","approve":true,"revision":1}`); rec.Code != http.StatusConflict {
		t.Fatalf("revisió equivocada: %d", rec.Code)
	}
	if rec := answerWindAsk(t, s, `{"key":"`+d2.ID+`","approve":true,"revision":2}`); rec.Code != 200 {
		t.Fatalf("aprovar la revisió 2: %d %s", rec.Code, rec.Body)
	}
	waitRun(t, s, d2.RunID)
	if got := tools.commands(); len(got) != 1 || got[0] != "rm -rf "+second {
		t.Fatalf("només s'aplica la revisió aprovada: %q", got)
	}
}

// Cancel·lar un run aparcat l'atura sense executar res, i cancel·lar-lo
// mentre la represa treballa talla l'eina de debò.
func TestWindDecisionCancelStopsTheRun(t *testing.T) {
	shortLive(t)
	tools := recordTools(t)
	s := decisionServer(t, decisionModel(t))
	cancelRun := func(id int64) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v2/runs/%d/cancel", id), nil)
		req.SetPathValue("id", fmt.Sprint(id))
		rec := httptest.NewRecorder()
		s.handleV2RunCancel(rec, req)
		return rec
	}

	// 1) Aparcat: cancel·lar-lo després d'un reinici.
	windChat(t, s, "ESBORRA "+filepath.Join(t.TempDir(), "a"))
	d := pendingWindAsks(s)[0]
	s = restartServer(t, s)
	if rec := cancelRun(d.RunID); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"cancelled":true`) {
		t.Fatalf("cancel·lar aparcat: %d %s", rec.Code, rec.Body)
	}
	if len(pendingWindAsks(s)) != 0 {
		t.Fatal("la decisió d'un run cancel·lat no pot quedar pendent")
	}
	if rec := answerWindAsk(t, s, `{"key":"`+d.ID+`","approve":true}`); rec.Code != http.StatusConflict {
		t.Fatalf("respondre un cancel·lat: %d", rec.Code)
	}
	waitBackground()
	if len(tools.commands()) != 0 {
		t.Fatal("un run cancel·lat no executa res")
	}

	// 2) En marxa: la represa executa una eina que no acaba; cancel·lar-la
	// la talla (context) i el run acaba cancel·lat.
	tools.mu.Lock()
	tools.block, tools.start = make(chan struct{}), make(chan struct{})
	started := tools.start
	tools.mu.Unlock()
	windChat(t, s, "ESBORRA "+filepath.Join(t.TempDir(), "b"))
	d = pendingWindAsks(s)[0]
	answerWindAsk(t, s, `{"key":"`+d.ID+`","approve":true}`)
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("la represa no ha arribat a l'eina")
	}
	if rec := cancelRun(d.RunID); rec.Code != 200 {
		t.Fatalf("cancel·lar en marxa: %d", rec.Code)
	}
	if run := waitRun(t, s, d.RunID); run.State != runs.Cancelled {
		t.Fatalf("estat: %+v", run)
	}
}

// Dues identitats al mateix servidor: cap no veu, respon ni cancel·la les
// decisions de l'altra, ni en rep els events.
func TestWindDecisionsAreIsolatedPerUser(t *testing.T) {
	shortLive(t)
	base := remindTestServer(t)
	hub := base.Hub()
	anna := hub.SessionFor(&User{Name: "anna"}, "mobil")
	bernat := hub.SessionFor(&User{Name: "bernat"}, "mobil")

	spot := &windParkSpot{wind: "xaloc", thread: "company-xaloc", userMsg: "esborra"}
	ctx := withWindPark(context.Background(), spot)
	ok, _ := anna.windApproveCall(ctx, nil, "bash", `{"command":"rm -rf /srv/anna"}`, "delete", "rm -rf /srv/anna", "")
	if ok || spot.parkedRec() == nil {
		t.Fatal("la decisió de l'Anna s'havia d'aparcar")
	}
	d := pendingWindAsks(anna)
	if len(d) != 1 || d[0].Owner != "anna" {
		t.Fatalf("l'Anna veu la seva: %+v", d)
	}
	if got := pendingWindAsks(bernat); len(got) != 0 {
		t.Fatalf("en Bernat veu decisions de l'Anna: %+v", got)
	}
	if rec := answerWindAsk(t, bernat, `{"key":"`+d[0].ID+`","approve":true}`); rec.Code != http.StatusNotFound {
		t.Fatalf("en Bernat respon la de l'Anna: %d", rec.Code)
	}
	req := httptest.NewRequest("POST", "/", nil)
	req.SetPathValue("id", fmt.Sprint(d[0].RunID))
	rec := httptest.NewRecorder()
	bernat.handleV2RunCancel(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("en Bernat cancel·la el run de l'Anna: %d", rec.Code)
	}
	evs, _ := hub.eventStore.AfterUser(0, 500, "bernat")
	for _, e := range evs {
		if e.Kind == events.Decision || int64(e.RunID) == d[0].RunID {
			t.Fatalf("en Bernat rep un event de l'Anna: %+v", e)
		}
	}
	evs, _ = hub.eventStore.AfterUser(0, 500, "anna")
	seen := false
	for _, e := range evs {
		seen = seen || (e.Kind == events.Decision && strings.Contains(string(e.Payload), d[0].ID))
	}
	if !seen {
		t.Fatal("l'Anna ha de rebre l'event de la seva decisió")
	}
	if pend := pendingWindAsks(anna); len(pend) != 1 || pend[0].State != "pending" {
		t.Fatalf("la de l'Anna continua intacta: %+v", pend)
	}
}

// Una decisió sense punt de represa (rotllana, colla) que un altre procés
// esperava en directe caduca en reiniciar: no pot tenir efecte.
func TestWindLiveOnlyDecisionExpiresOnRestart(t *testing.T) {
	s := remindTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.windApprove(ctx, nil, "bash", "install", "apt-get install x", "")
	}()
	d := waitForWindAsk(t, s)
	windProc = newMessageID() // un altre procés
	if got := pendingWindAsks(s); len(got) != 0 {
		t.Fatalf("una espera d'un procés mort no és pendent: %+v", got)
	}
	if rec := answerWindAsk(t, s, `{"key":"`+d.Key+`","approve":true}`); rec.Code != http.StatusConflict {
		t.Fatalf("respondre-la: %d", rec.Code)
	}
	cancel()
	<-done
}
