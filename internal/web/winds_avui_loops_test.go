package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/jobs"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// loopThread desa una conversa amb el vent xaloc com si l'hagués deixat a l'hora at.
func loopThread(t *testing.T, s *Server, name string, at time.Time, asked, said string) {
	t.Helper()
	if _, err := session.SaveSession(s.sessDir(), session.Session{
		Name: name, Title: asked, Role: "chat", SavedAt: at,
		Convo: []llm.Message{{Role: "user", Content: asked, Ts: at.Unix()}, {Role: "assistant", Content: said, Ts: at.Unix()}},
	}); err != nil {
		t.Fatal(err)
	}
}

// stubLoopsLoop és el model fals de la mirada: respon reply i desa cada consigna.
func stubLoopsLoop(t *testing.T, reply *string) *[]string {
	t.Helper()
	var briefs []string
	old := loopsLoop
	loopsLoop = func(_ *Server, _ context.Context, brief string) (exploracio, error) {
		briefs = append(briefs, brief)
		return exploracio{text: *reply}, nil
	}
	t.Cleanup(func() { loopsLoop = old })
	return &briefs
}

func loopCards(t *testing.T, s *Server) []avuiCard {
	t.Helper()
	var out []avuiCard
	for _, c := range s.avuiView()[avuiFYI].([]avuiCard) {
		if c.Source == avuiLoopSource {
			out = append(out, c)
		}
	}
	return out
}

func TestAvuiLoopsJobRunsAnHourBeforeQuietTo(t *testing.T) {
	s := localTestServer(t)
	rec := httptest.NewRecorder()
	s.handleWindsAvui(rec, httptest.NewRequest(http.MethodGet, "/api/winds/avui", nil))
	if rec.Code != 200 {
		t.Fatalf("GET: %d %s", rec.Code, rec.Body)
	}
	job, ok := s.ownWindJob(loopsJobID(""))
	if !ok || job.Kind != "daily" || job.Time != "07:00" || !job.Enabled {
		t.Fatalf("with quiet_to 08:00 the open threads are looked at 07:00: %+v %v", job, ok)
	}
	avuiSettingsPost(t, s, `{"quiet_from":"23:00","quiet_to":"00:30"}`)
	if job, _ := s.ownWindJob(loopsJobID("")); job.Time != "23:30" {
		t.Fatalf("a new quiet_to moves it: %+v", job)
	}
}

func TestAvuiLoopsFollowUpOpenThreads(t *testing.T) {
	s := localTestServer(t)
	reply := "Ja hi ha vols a Roma el 12 de desembre des de 89 €. Surten de Girona."
	briefs := stubLoopsLoop(t, &reply)
	now := time.Now()
	loopThread(t, s, "company-xaloc", now.Add(-3*time.Hour), "Busca'm vols a Roma pel desembre",
		"Encara no hi ha preus per al 12 de desembre. Vols que ho torni a mirar demà?")
	loopThread(t, s, "company-xaloc-s1700000001", now.Add(-2*time.Hour), "Gràcies per la recepta", "De res!")
	loopThread(t, s, "company-xaloc-s1700000002", now.Add(-30*time.Hour), "Quin temps farà a Berlín?",
		"Encara no ho sé. Vols que ho miri?")

	if err := s.syncLoopsJob(); err != nil {
		t.Fatal(err)
	}
	job, ok := s.ownWindJob(loopsJobID(""))
	if !ok {
		t.Fatal("no loops job")
	}
	run, err := s.executeJob(context.Background(), job, runsZero())
	if err != nil || run.Status != "ok" {
		t.Fatalf("run: %+v %v", run, err)
	}
	if len(*briefs) != 1 || !strings.Contains((*briefs)[0], "Roma") {
		t.Fatalf("only the open thread of the last 24 h gets a turn: %d %q", len(*briefs), *briefs)
	}
	cards := loopCards(t, s)
	if len(cards) != 1 {
		t.Fatalf("one finding, one card: %+v", cards)
	}
	c := cards[0]
	if c.Section != avuiFYI || c.Wind != "xaloc" || !strings.Contains(c.Title, "89 €") || !strings.Contains(c.Why, "Busca'm vols a Roma") {
		t.Fatalf("card: %+v", c)
	}
	if len(c.Actions) != 1 || c.Actions[0].Thread != "company-xaloc" || c.Actions[0].Label == "" {
		t.Fatalf("the action opens the thread: %+v", c.Actions)
	}
}

func TestAvuiLoopsNothingFoundNoCardAndDismissedNeverComesBack(t *testing.T) {
	s := localTestServer(t)
	reply := "(res)"
	briefs := stubLoopsLoop(t, &reply)
	loc := s.loadConnections().zone()
	n := time.Now().In(loc)
	night := time.Date(n.Year(), n.Month(), n.Day(), 7, 0, 0, 0, loc)
	loopThread(t, s, "company-xaloc", night.Add(-30*time.Minute), "Mira si el Liceu treu entrades noves",
		"Encara no n'hi ha. Ho tinc pendent.")
	if _, err := s.avuiLoops(context.Background(), night); err != nil {
		t.Fatal(err)
	}
	if len(*briefs) != 1 || len(loopCards(t, s)) != 0 {
		t.Fatalf("nothing found, no card: %d %+v", len(*briefs), loopCards(t, s))
	}

	loopThread(t, s, "company-xaloc-s1700000001", night.Add(-20*time.Minute), "I el concert de jazz?",
		"No hi ha data. Vols que hi torni?")
	reply = "El concert de jazz és el 3 de desembre al Jamboree."
	if _, err := s.avuiLoops(context.Background(), night); err != nil {
		t.Fatal(err)
	}
	cards := loopCards(t, s)
	if len(*briefs) != 2 || len(cards) != 1 {
		t.Fatalf("the jazz thread is a finding and the Liceu one is not looked at again: %d %+v", len(*briefs), cards)
	}
	avuiPost(t, s, cards[0].ID, "dismiss")

	// L'endemà al vespre els dos fils encara són de les últimes 24 h.
	if _, err := s.avuiLoops(context.Background(), night.Add(23*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(*briefs) != 2 || len(loopCards(t, s)) != 0 {
		t.Fatalf("a dismissed loop never comes back: %d %+v", len(*briefs), loopCards(t, s))
	}
}

func TestAvuiLoopsAtMostThreeTurnsPerNight(t *testing.T) {
	s := localTestServer(t)
	reply := "Hi ha novetats sobre això d'ahir."
	briefs := stubLoopsLoop(t, &reply)
	now := time.Now()
	for i := 0; i < 5; i++ {
		loopThread(t, s, "company-xaloc-s170000000"+strconv.Itoa(i), now.Add(-time.Duration(i+1)*time.Hour),
			"Pregunta "+strconv.Itoa(i), "Encara no ho sé. Vols que ho torni a mirar?")
	}
	for range 2 {
		if _, err := s.avuiLoops(context.Background(), now); err != nil {
			t.Fatal(err)
		}
	}
	if len(*briefs) != 3 || len(loopCards(t, s)) != 3 {
		t.Fatalf("at most 3 model turns per night: %d turns, %d cards", len(*briefs), len(loopCards(t, s)))
	}
}

// TestAvuiNightJobsAreThereOnServerStart: un compte amb vents té les feines de
// la nit en engegar el servidor, sense obrir «Avui»; una d'apagada no s'encén.
func TestAvuiNightJobsAreThereOnServerStart(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	js, err := jobs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.jobs = js
	s.users = NewUsers(&config.Config{Users: map[string]config.UserCfg{
		"anna": {Password: HashPassword("clau-anna-01")},
		"bert": {Password: HashPassword("clau-bert-01")},
	}}, "")
	s.users.path = filepath.Join(t.TempDir(), "tokens.json")
	anna := s.Hub().SessionFor(s.users.Usuari("anna"), "t")
	bert := s.Hub().SessionFor(s.users.Usuari("bert"), "t")
	if _, err := anna.winds.Save(companion.Wind{Name: "Xaloc"}); err != nil {
		t.Fatal(err)
	}
	fakeICSServer(t, icsWith())
	if w := connPost(t, anna, `{"add_calendar":{"name":"Feina","url":"https://calendar.example.org/private-anna.ics"}}`); w.Code != 200 {
		t.Fatalf("add calendar: %s", w.Body.String())
	}
	start := func() {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		done := s.StartBackground(ctx)
		cancel()
		<-done
	}
	start()
	job, ok := anna.ownWindJob(loopsJobID("anna"))
	if !ok || !job.Enabled || job.Wind != "xaloc" || job.Kind != "daily" {
		t.Fatalf("an account with winds gets its open-threads job on start: %+v %v", job, ok)
	}
	if prep, ok := anna.ownWindJob(prepJobID("anna")); !ok || !prep.Enabled {
		t.Fatalf("with a calendar it also gets the prep job: %+v %v", prep, ok)
	}
	if _, ok := bert.ownWindJob(loopsJobID("bert")); ok {
		t.Fatal("an account without winds gets no job")
	}

	job.Enabled = false
	if _, err := s.jobs.Save(job); err != nil {
		t.Fatal(err)
	}
	avuiSettingsPost(t, anna, `{"quiet_from":"23:00","quiet_to":"09:30"}`)
	start()
	if job, _ := anna.ownWindJob(loopsJobID("anna")); job.Enabled {
		t.Fatalf("a job turned off stays off: %+v", job)
	}
}
