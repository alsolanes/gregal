package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gregal/internal/runs"
	"gregal/internal/tools"
)

// B3 — handoff durable per iniciar sessió. Proveïdor fals, eines falses i
// comprovació de sessió falsa: no hi ha cap navegador ni cap VM real.

const handoffSecret = "SECRET-TOKEN-1234"

// handoffModel demana un handoff quan la persona diu «ENTRA» i, amb el
// resultat de l'eina, respon amb el que diu. Desa tot el que rep.
func handoffModel(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		mu.Unlock()
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(raw, &req)
		w.Header().Set("Content-Type", "text/event-stream")
		last := req.Messages[len(req.Messages)-1]
		if last.Role == "tool" {
			b, _ := json.Marshal("Eina: " + fmt.Sprint(last.Content))
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", b)
			return
		}
		args, _ := json.Marshal(map[string]string{"action": "handoff", "text": "Inicia sessió a Exemple i toca Ja està"})
		wire, _ := json.Marshal(string(args))
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"h1\",\"function\":{\"name\":\"browser\",\"arguments\":%s}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n", wire)
	}))
	t.Cleanup(srv.Close)
	return srv, &bodies
}

type loginChecks struct {
	mu    sync.Mutex
	calls []string
	res   tools.LoginCheck
	err   error
}

func fakeLoginCheck(t *testing.T, res tools.LoginCheck) *loginChecks {
	t.Helper()
	lc := &loginChecks{res: res}
	old := windLoginCheck
	windLoginCheck = func(ctx context.Context, scope, page string) (tools.LoginCheck, error) {
		lc.mu.Lock()
		defer lc.mu.Unlock()
		lc.calls = append(lc.calls, page)
		return lc.res, lc.err
	}
	t.Cleanup(func() { windLoginCheck = old })
	return lc
}

func handoffPage(t *testing.T, raw string) {
	t.Helper()
	old := windPageURL
	windPageURL = func(string) string { return raw }
	t.Cleanup(func() { windPageURL = old })
}

func handoffServer(t *testing.T) (*Server, *[]string, *toolRecorder) {
	t.Helper()
	shortLive(t)
	resetOrdinadorControl()
	t.Cleanup(resetOrdinadorControl)
	rec := recordTools(t)
	model, bodies := handoffModel(t)
	s := decisionServer(t, model)
	t.Setenv("GREGAL_ORDINADOR_VISOR", "127.0.0.1:9") // visor local fals: només cal per prendre el control
	return s, bodies, rec
}

func openViewerFor(t *testing.T, s *Server, body string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleOrdinadorViewer(rec, httptest.NewRequest("POST", "/api/ordinador/viewer", strings.NewReader(body)))
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != 200 {
		t.Fatalf("obrir el visor: %d %s", rec.Code, rec.Body)
	}
	return out
}

func TestWindHandoffSurvivesRestartAndChecksLogin(t *testing.T) {
	s, bodies, toolsRan := handoffServer(t)
	handoffPage(t, "https://exemple.cat/entra?next=/compte&token="+handoffSecret+"#x")
	checks := fakeLoginCheck(t, tools.LoginCheck{Page: "https://exemple.cat/compte", Title: "El meu compte"})

	// El vent necessita que la persona iniciï sessió: el torn s'aparca, no
	// espera al navegador.
	raw := windChat(t, s, "ENTRA al meu compte")
	if !strings.Contains(raw, "event: approve_request") || !strings.Contains(raw, `"type":"handoff"`) || !strings.Contains(raw, `"waiting"`) {
		t.Fatalf("el torn s'havia d'aparcar amb un handoff:\n%s", raw)
	}
	pend := pendingWindAsks(s)
	if len(pend) != 1 {
		t.Fatalf("pendents: %+v", pend)
	}
	d := pend[0]
	if d.Type != "handoff" || d.State != "pending" || d.ProposedEffect == nil || d.ProposedEffect.Type != "handoff" || d.Page != "https://exemple.cat/entra" || d.Site != "exemple.cat" || d.RunID == 0 {
		t.Fatalf("decisió de handoff: %+v", d)
	}
	if run, _ := s.Hub().Queue().Get(d.RunID); run.State != runs.WaitingUser {
		t.Fatalf("la feina ha d'esperar la persona: %+v", run)
	}
	if !strings.Contains(threadText(s), s.tr("Et toca a tu a l'ordinador", "Your turn on the computer")) {
		t.Fatalf("el fil diu què ha de fer:\n%s", threadText(s))
	}

	// La persona obre l'ordinador des de la targeta: pren el control.
	out := openViewerFor(t, s, `{"handoff":"`+d.ID+`"}`)
	if out["controlOwner"] != "user" || out["handoff"] != d.ID {
		t.Fatalf("el visor del handoff dona el control: %v", out)
	}
	if !tools.BrowserControlHeld(s.id) {
		t.Fatal("mentre la persona inicia sessió l'agent no pot tocar el navegador")
	}

	// Reinici del servidor (i l'app tancada) entre el handoff i la represa.
	s = restartServer(t, s)
	resetOrdinadorControl()
	if n := s.CloseOrphanRuns(); n != 0 {
		t.Fatalf("un handoff aparcat no és orfe: %d", n)
	}
	if p := pendingWindAsks(s); len(p) != 1 || p[0].ID != d.ID || p[0].Type != "handoff" {
		t.Fatalf("el punt d'espera es conserva: %+v", p)
	}
	if !s.ordinadorLeases().HeldByUser(tools.BrowserOwnerKey(s.id)) {
		t.Fatal("la lease també sobreviu al reinici")
	}

	// «Ja està»: el control torna a l'agent, el torn es reprèn i comprova
	// la sessió sense llegir cap camp.
	if rec := answerWindAsk(t, s, `{"key":"`+d.ID+`","approve":true}`); rec.Code != 200 {
		t.Fatalf("ja està: %d %s", rec.Code, rec.Body)
	}
	if tools.BrowserControlHeld(s.id) {
		t.Fatal("en dir «ja està» el control torna a l'agent")
	}
	if run := waitRun(t, s, d.RunID); run.State != runs.Completed {
		t.Fatalf("encàrrec reprès: %+v", run)
	}
	checks.mu.Lock()
	if len(checks.calls) != 1 || checks.calls[0] != "https://exemple.cat/entra" {
		t.Fatalf("una comprovació, a la pàgina del login (sense query): %v", checks.calls)
	}
	checks.mu.Unlock()
	if got := toolsRan.commands(); len(got) != 0 {
		t.Fatalf("el handoff no passa per l'eina del navegador: %q", got)
	}
	thread := threadText(s)
	if !strings.Contains(thread, "session looks active") || strings.Count(thread, "ENTRA al meu compte") != 1 {
		t.Fatalf("l'encàrrec continua al mateix fil:\n%s", thread)
	}

	// Cap testimoni de la pàgina a la decisió, als events, al fil ni al model.
	files, _ := filepath.Glob(filepath.Join(s.decisionsPath(), "*.json"))
	for _, f := range files {
		b, _ := os.ReadFile(f)
		if strings.Contains(string(b), handoffSecret) {
			t.Fatalf("secret a la decisió %s", f)
		}
	}
	evs, _ := s.Hub().eventStore.AfterUser(0, 1000, s.usuari())
	for _, e := range evs {
		if strings.Contains(e.Text+string(e.Payload), handoffSecret) {
			t.Fatalf("secret a un event: %+v", e)
		}
	}
	if strings.Contains(thread, handoffSecret) {
		t.Fatal("secret al fil")
	}
	for _, b := range *bodies {
		if strings.Contains(b, handoffSecret) {
			t.Fatal("secret enviat al model")
		}
	}
	if p := pendingWindAsks(s); len(p) != 0 {
		t.Fatalf("ja no queda res pendent: %+v", p)
	}
}

func TestWindHandoffCancelAndTwoFactorAreHonest(t *testing.T) {
	s, _, _ := handoffServer(t)
	handoffPage(t, "https://exemple.cat/entra")
	checks := fakeLoginCheck(t, tools.LoginCheck{Page: "https://exemple.cat/2fa", CodeField: true})

	// 2FA pendent: la persona diu «ja està» però el codi encara hi és.
	windChat(t, s, "ENTRA")
	d := pendingWindAsks(s)[0]
	answerWindAsk(t, s, `{"key":"`+d.ID+`","approve":true}`)
	waitRun(t, s, d.RunID)
	if !strings.Contains(threadText(s), "two-step verification pending") {
		t.Fatalf("2FA pendent ha de dir-ho:\n%s", threadText(s))
	}

	// Cancel·la: rebutjada, el model ho sap i no es comprova res.
	checks.mu.Lock()
	checks.calls = nil
	checks.mu.Unlock()
	windChat(t, s, "ENTRA un altre cop")
	d = pendingWindAsks(s)[0]
	openViewerFor(t, s, `{"handoff":"`+d.ID+`"}`)
	if rec := answerWindAsk(t, s, `{"key":"`+d.ID+`","approve":false}`); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"rejected"`) {
		t.Fatalf("cancel·lar: %d %s", rec.Code, rec.Body)
	}
	if tools.BrowserControlHeld(s.id) {
		t.Fatal("cancel·lar també torna el control")
	}
	waitRun(t, s, d.RunID)
	if !strings.Contains(threadText(s), "cancelled the handoff") {
		t.Fatalf("cancel·lat:\n%s", threadText(s))
	}
	if len(checks.calls) != 0 {
		t.Fatal("cancel·lat: cap comprovació")
	}

	// Ordinador o xarxa caiguts: no es dona la sessió per bona.
	checks.mu.Lock()
	checks.err = fmt.Errorf("browser: connection refused")
	checks.mu.Unlock()
	windChat(t, s, "ENTRA de nou")
	d = pendingWindAsks(s)[0]
	answerWindAsk(t, s, `{"key":"`+d.ID+`","approve":true}`)
	waitRun(t, s, d.RunID)
	if !strings.Contains(threadText(s), "Do not assume the sign-in worked") {
		t.Fatalf("xarxa perduda:\n%s", threadText(s))
	}
}

func TestWindHandoffExpiresAndCancelsTheRun(t *testing.T) {
	s, _, _ := handoffServer(t)
	handoffPage(t, "")
	fakeLoginCheck(t, tools.LoginCheck{})
	old := windHandoffTTL
	windHandoffTTL = 1 // ja caducat en llegir-lo
	t.Cleanup(func() { windHandoffTTL = old })
	windChat(t, s, "ENTRA")
	if p := pendingWindAsks(s); len(p) != 0 {
		t.Fatalf("caducat no és pendent: %+v", p)
	}
	waitBackground()
	var found bool
	_ = s.withDecisions(func(st *decisionDir) error {
		for _, rec := range st.list() {
			if rec.Type == "handoff" {
				found = true
				if rec.State != "expired" || rec.StateReason != "timeout" {
					t.Fatalf("handoff caducat: %+v", rec.windAskReq)
				}
				if run, _ := s.Hub().Queue().Get(rec.RunID); run.State != runs.Cancelled {
					t.Fatalf("el run del handoff caducat s'atura: %+v", run)
				}
			}
		}
		return nil
	})
	if !found {
		t.Fatal("no hi ha cap handoff desat")
	}
}

func TestWindHandoffIsolatedPerUser(t *testing.T) {
	shortLive(t)
	resetOrdinadorControl()
	t.Cleanup(resetOrdinadorControl)
	t.Setenv("GREGAL_ORDINADOR_VISOR", "127.0.0.1:9")
	handoffPage(t, "https://exemple.cat/entra")
	base := remindTestServer(t)
	hub := base.Hub()
	anna := hub.SessionFor(&User{Name: "anna"}, "mobil")
	bernat := hub.SessionFor(&User{Name: "bernat"}, "mobil")

	spot := &windParkSpot{wind: "xaloc", thread: "company-xaloc", userMsg: "entra"}
	ctx := withWindPark(context.Background(), spot)
	if _, parked := anna.windHandoffCall(ctx, nil, `{"action":"handoff","text":"Inicia sessió"}`); !parked {
		t.Fatal("el handoff de l'Anna s'havia d'aparcar")
	}
	d := pendingWindAsks(anna)
	if len(d) != 1 || d[0].Owner != "anna" || d[0].Type != "handoff" {
		t.Fatalf("l'Anna veu el seu: %+v", d)
	}
	if got := pendingWindAsks(bernat); len(got) != 0 {
		t.Fatalf("en Bernat veu el handoff de l'Anna: %+v", got)
	}
	if bernat.ownsPendingHandoff(d[0].ID) || !anna.ownsPendingHandoff(d[0].ID) {
		t.Fatal("el handoff és només de l'Anna")
	}
	// En Bernat obre el seu visor amb l'id de l'Anna: no s'hi lliga.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/ordinador/viewer", strings.NewReader(`{"handoff":"`+d[0].ID+`"}`))
	req = req.WithContext(context.WithValue(req.Context(), ctxKeyUser{}, &User{Name: "bernat"}))
	bernat.handleOrdinadorViewer(rec, req)
	if strings.Contains(rec.Body.String(), d[0].ID) {
		t.Fatalf("en Bernat s'ha lligat al handoff de l'Anna: %s", rec.Body)
	}
	if rec := answerWindAsk(t, bernat, `{"key":"`+d[0].ID+`","approve":true}`); rec.Code != http.StatusNotFound {
		t.Fatalf("en Bernat respon el de l'Anna: %d", rec.Code)
	}
	if p := pendingWindAsks(anna); len(p) != 1 || p[0].State != "pending" {
		t.Fatalf("el de l'Anna continua: %+v", p)
	}
}

// Sense ordinador, sense visor i amb el navegador invisible: no es crea cap
// handoff que ningú no podria fer; l'eina de sempre respon (i diu que cal un
// navegador visible) i res no peta.
func TestWindHandoffWithoutAnyComputerFallsBack(t *testing.T) {
	s, _, toolsRan := handoffServer(t)
	t.Setenv("GREGAL_ORDINADOR_VISOR", "")
	old := windBrowserVisible
	windBrowserVisible = func() bool { return false }
	t.Cleanup(func() { windBrowserVisible = old })
	handoffPage(t, "")
	fakeLoginCheck(t, tools.LoginCheck{})
	windChat(t, s, "ENTRA")
	if p := pendingWindAsks(s); len(p) != 0 {
		t.Fatalf("sense ordinador no hi ha d'haver handoff pendent: %+v", p)
	}
	if got := toolsRan.commands(); len(got) != 1 {
		t.Fatalf("l'eina de sempre respon: %q", got)
	}
	rec := httptest.NewRecorder()
	s.handleOrdinadorViewer(rec, httptest.NewRequest("POST", "/api/ordinador/viewer", strings.NewReader(`{}`)))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "no_viewer") {
		t.Fatalf("sense visor ho diu clar: %d %s", rec.Code, rec.Body)
	}
}
