package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func cornerTestServer(t *testing.T) *Server {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"#FF9F00","role":"jocs"}`)
	postJSON(t, s.handleWinds, `{"name":"Mestral","color":"#3E7BFA","role":"disseny"}`)
	return s
}

const cornerGoodPage = `<!doctype html><html><head><title>Racó</title></head><body><h1>El Racó</h1><button onclick="gregal.ask('xaloc','hola').then(t=>out.textContent=t)">Parla</button><p id="out"></p></body></html>`

func TestCornerSessionBuildsReviewsAndPublishes(t *testing.T) {
	s := cornerTestServer(t)
	var mu sync.Mutex
	var prompts []string
	calls := 0
	old := cornerModel
	cornerModel = func(_ *Server, _ context.Context, system, user string, _ int) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		prompts = append(prompts, system+"\n"+user)
		switch calls {
		case 1: // reunió
			return `{"action":"work","why":"El full és en blanc","builder":"xaloc","reviewer":"xaloc","plan":"Fem un joc per conèixer els vents."}`, nil
		case 2: // primer intent: carrega una font de fora → la comprovació l'atura
			return "SUMMARY: Primer joc\n```html\n<html><head><link href=\"https://fonts.example/x.css\" rel=stylesheet></head><body>x</body></html>\n```", nil
		case 3: // segon intent, net
			return "SUMMARY: Un joc per conèixer els vents\n```html\n" + cornerGoodPage + "\n```", nil
		case 4: // revisió
			return `{"approve":true,"notes":"Funciona i és amable.","fix":[]}`, nil
		}
		return "", errConst("too many calls")
	}
	t.Cleanup(func() { cornerModel = old })
	postJSON(t, s.handleCorner, `{"action":"comment","text":"Feu-hi un joc!"}`)

	s.cornerSession("test")

	st := s.cornerLoad()
	if st.Current != 1 || st.Sessions != 1 || st.Calls != 4 {
		t.Fatalf("estat: %+v", st)
	}
	vs := s.cornerVersions()
	if len(vs) != 1 || vs[0].Builder != "xaloc" || vs[0].Reviewer != "mestral" || !strings.Contains(vs[0].Summary, "conèixer") {
		t.Fatalf("versió: %+v", vs)
	}
	// La reunió va llegir el comentari, i el revisor no pot ser el constructor.
	if !strings.Contains(prompts[0], "Feu-hi un joc!") || !strings.Contains(prompts[0], "COMMENTS") {
		t.Errorf("la reunió no veia el comentari")
	}
	if !strings.Contains(prompts[2], "fonts.example") && !strings.Contains(prompts[2], "outside") {
		t.Errorf("el segon intent havia de portar el motiu de la comprovació:\n%s", prompts[2])
	}
	if st.SeenComment == 0 {
		t.Errorf("el comentari havia de quedar llegit")
	}
	kinds := []string{}
	for _, e := range s.cornerJournal(50) {
		kinds = append(kinds, e.Kind)
	}
	if got := strings.Join(kinds, ","); !strings.Contains(got, "meeting,rejected,built,review,published") {
		t.Errorf("diari: %s", got)
	}

	// La pàgina va emmarcada: CSP sense connexions i el pont.
	req := httptest.NewRequest(http.MethodGet, "/api/corner/page", nil)
	rec := httptest.NewRecorder()
	s.handleCornerPage(rec, req)
	var page struct {
		Doc  string         `json:"doc"`
		Data map[string]any `json:"data"`
	}
	json.Unmarshal(rec.Body.Bytes(), &page)
	if !strings.Contains(page.Doc, "connect-src 'none'") || !strings.Contains(page.Doc, "window.gregal") || !strings.Contains(page.Doc, "El Racó") {
		t.Fatalf("pàgina emmarcada: %s", page.Doc)
	}
	if strings.Index(page.Doc, "Content-Security-Policy") > strings.Index(page.Doc, "<title>") {
		t.Errorf("la CSP ha d'anar abans del contingut")
	}

	// Tornar enrere crea una versió nova igual a l'antiga.
	postJSON(t, s.handleCorner, `{"action":"revert","version":1}`)
	if st := s.cornerLoad(); st.Current != 2 || s.cornerHTML(2) != s.cornerHTML(1) {
		t.Fatalf("revert: %+v", st)
	}
}

func TestCornerRestAndRejectedTwiceKeepThePage(t *testing.T) {
	s := cornerTestServer(t)
	old := cornerModel
	t.Cleanup(func() { cornerModel = old })
	cornerModel = func(_ *Server, _ context.Context, _, _ string, _ int) (string, error) {
		return `{"action":"rest","why":"Avui no cal res."}`, nil
	}
	s.cornerSession("test")
	if st := s.cornerLoad(); st.Current != 0 || st.Calls != 1 {
		t.Fatalf("descans: %+v", st)
	}
	n := 0
	cornerModel = func(_ *Server, _ context.Context, _, _ string, _ int) (string, error) {
		n++
		switch {
		case n == 1:
			return `{"action":"work","why":"provem","builder":"mestral","reviewer":"xaloc","plan":"Una galeria."}`, nil
		case n%2 == 0:
			return "SUMMARY: galeria\n```html\n" + cornerGoodPage + "\n```", nil
		}
		return `{"approve":false,"notes":"Encara no.","fix":["més color"]}`, nil
	}
	s.cornerSession("test")
	if st := s.cornerLoad(); st.Current != 0 {
		t.Fatalf("dos rebuigs no publiquen res: %+v", st)
	}
	last := s.cornerJournal(1)
	if len(last) != 1 || last[0].Kind != "failed" {
		t.Fatalf("diari: %+v", last)
	}
}

func TestCornerWants(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	recent := cornerState{LastSession: now.Add(-20 * time.Minute).Unix()}
	old := cornerState{LastSession: now.Add(-5 * time.Hour).Unix()}
	cases := []struct {
		name      string
		st        cornerState
		winds     int
		comments  int
		talks     int
		running   bool
		rnd       float64
		want      bool
	}{
		{"first time", cornerState{}, 2, 0, 0, false, 0.99, true},
		{"one wind is not enough", cornerState{}, 1, 0, 0, false, 0, false},
		{"paused", cornerState{Paused: true}, 2, 1, 0, false, 0, false},
		{"already working", cornerState{}, 2, 0, 0, true, 0, false},
		{"comment after 10 min", recent, 2, 1, 0, false, 0.99, true},
		{"nothing new, too soon", recent, 2, 0, 0, false, 0, false},
		{"a while ago, lucky", old, 2, 0, 0, false, 0.1, true},
		{"a while ago, not now", old, 2, 0, 0, false, 0.5, false},
		{"visitors talked", old, 2, 0, 4, false, 0.4, true},
		{"budget used", cornerState{Sessions: cornerMaxSessions}, 2, 1, 0, false, 0, false},
	}
	for _, c := range cases {
		if got, _ := cornerWants(c.st, c.winds, c.comments, c.talks, c.running, now, c.rnd); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

func TestCornerProblems(t *testing.T) {
	if p := cornerProblems(cornerGoodPage); len(p) != 0 {
		t.Fatalf("pàgina bona: %v", p)
	}
	for _, bad := range []string{
		`<html><body><img src="https://x.cat/a.png"></body></html>`,
		`<html><body><script src="//cdn.x/y.js"></script></body></html>`,
		`<html><body><style>@import url(x.css)</style></body></html>`,
		`<html><body><script>fetch('/api/state')</script></body></html>`,
		`<html><head><meta http-equiv="refresh" content="0;url=x"></head><body></body></html>`,
		`<p>sense document</p>`,
	} {
		if len(cornerProblems(bad)) == 0 {
			t.Errorf("s'havia de rebutjar: %s", bad)
		}
	}
	sum, html := cornerExtract("SUMMARY: hola\n```html\n<html><body>x</body></html>\n```")
	if sum != "hola" || html != "<html><body>x</body></html>" {
		t.Errorf("extract: %q %q", sum, html)
	}
}

func TestCornerAskHasADailyLimit(t *testing.T) {
	s := cornerTestServer(t)
	saved := petChatter
	petChatter = func(context.Context, *llm.Client, *config.Config, config.Provider, config.Role, []llm.Message) (string, error) {
		return "Hola, visitant!", nil
	}
	t.Cleanup(func() { waitBackground(); petChatter = saved })
	rec := postJSON(t, s.handleCornerAsk, `{"wind":"xaloc","text":"qui ets?"}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Hola, visitant!") {
		t.Fatalf("ask: %d %s", rec.Code, rec.Body.String())
	}
	if j := s.cornerJournal(5); len(j) == 0 || j[len(j)-1].Kind != "talk" {
		t.Fatalf("la conversa s'havia d'apuntar: %+v", j)
	}
	st := s.cornerLoad()
	st.Talks = cornerMaxTalks
	s.cornerSave(st)
	if rec := postJSON(t, s.handleCornerAsk, `{"wind":"xaloc","text":"i ara?"}`); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("límit: %d", rec.Code)
	}
	if rec := postJSON(t, s.handleCornerAsk, `{"wind":"ningu","text":"hola"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("vent desconegut: %d", rec.Code)
	}
}

func TestCornerStartsPausedAndStopCutsTheSession(t *testing.T) {
	s := cornerTestServer(t)
	if !s.cornerLoad().Paused {
		t.Fatal("un Racó nou ha de començar aturat (no gasta res fins que l'usuari ho vol)")
	}
	postJSON(t, s.handleCorner, `{"action":"resume"}`)
	if s.cornerLoad().Paused {
		t.Fatal("resume")
	}
	started := make(chan struct{})
	old := cornerModel
	cornerModel = func(_ *Server, ctx context.Context, _, _ string, _ int) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}
	t.Cleanup(func() { cornerModel = old })
	done := make(chan struct{})
	go func() { s.cornerSession("test"); close(done) }()
	<-started
	postJSON(t, s.handleCorner, `{"action":"stop"}`)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stop no ha tallat la sessió")
	}
	st := s.cornerLoad()
	if !st.Paused || st.Current != 0 {
		t.Fatalf("després d'aturar: %+v", st)
	}
	var kinds []string
	for _, e := range s.cornerJournal(10) {
		kinds = append(kinds, e.Kind)
	}
	if got := strings.Join(kinds, ","); !strings.Contains(got, "stopped") || strings.Contains(got, "failed") {
		t.Fatalf("diari: %s", got)
	}
}
