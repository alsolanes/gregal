package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/jobs"
)

// avuiRespCards posa n targetes d'una font a «Avui» com ho fa una veu del vent.
func avuiRespCards(t *testing.T, s *Server, source, job string, n int) []string {
	t.Helper()
	w, ok := s.winds.ByID("xaloc")
	if !ok {
		t.Fatal("xaloc")
	}
	var ids []string
	for i := 0; i < n; i++ {
		id, _ := s.avuiFromVoice(w, source, job, "Han canviat els horaris del bus "+string(rune('A'+i))+".")
		if id == "" {
			t.Fatal("no card")
		}
		ids = append(ids, id)
	}
	return ids
}

func avuiPost(t *testing.T, s *Server, id, action string) {
	t.Helper()
	if rec := b8Post(t, s.handleWindsAvui, map[string]string{"id": id, "action": action}); rec.Code != 200 {
		t.Fatalf("%s %s: %d %s", action, id, rec.Code, rec.Body)
	}
}

// respListed llegeix una responsabilitat per GET /api/winds/responsabilitats.
func respListed(t *testing.T, s *Server, id string) respPublic {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleWindsResponsabilitats(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	var list struct{ Responsabilitats []respPublic }
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	for _, r := range list.Responsabilitats {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("%s no surt a la llista: %s", id, rec.Body)
	return respPublic{}
}

func respLogNotes(r respPublic, kind string) []string {
	var out []string
	for _, a := range r.Log {
		if a.Kind == kind {
			out = append(out, a.Note)
		}
	}
	return out
}

func TestAvuiThreeNoesHalveTheWatchAndFivePauseIt(t *testing.T) {
	s := respTestServer(t)
	page := newRespPage(t, "Horari: 8:00")
	id := respCreateWeb(t, s, page.srv.URL)
	cards := avuiRespCards(t, s, "responsabilitat", id, 6)

	avuiPost(t, s, cards[0], "dismiss")
	avuiPost(t, s, cards[1], "notuseful")
	if r := respListed(t, s, id); r.Budget.MessagesPerDay != 3 || len(respLogNotes(r, "budget")) != 0 {
		t.Fatalf("two noes must not slow it down yet: %+v", r)
	}
	avuiPost(t, s, cards[2], "dismiss")
	r := respListed(t, s, id)
	if r.Budget.MessagesPerDay != 1 || r.State != respActive {
		t.Fatalf("three noes halve the attention budget: %+v", r)
	}
	if notes := respLogNotes(r, "budget"); len(notes) != 1 || !strings.Contains(notes[0], "3") {
		t.Fatalf("the slowdown must be in the log: %+v", r.Log)
	}
	if n := len(s.avuiView()["now"].([]avuiCard)); n != 0 {
		t.Fatalf("no card before the pause: %d", n)
	}

	avuiPost(t, s, cards[3], "dismiss")
	avuiPost(t, s, cards[4], "dismiss")
	r = respListed(t, s, id)
	if r.State != respPaused || len(respLogNotes(r, "pause")) != 1 {
		t.Fatalf("five noes pause the watch: %+v", r)
	}
	if j, _ := s.respFindJob(id); j.Enabled {
		t.Fatal("a paused watch must not run")
	}
	now := s.avuiView()["now"].([]avuiCard)
	if len(now) != 1 || !strings.Contains(now[0].Title, "horaris del bus") || len(now[0].Actions) != 1 || now[0].Actions[0].Label == "" {
		t.Fatalf("one card tells what stopped: %+v", now)
	}
	// Una sisena negativa no en fa cap altra.
	avuiPost(t, s, cards[5], "dismiss")
	if n := len(s.avuiView()["now"].([]avuiCard)); n != 1 {
		t.Fatalf("only one pause card: %d", n)
	}

	avuiPost(t, s, now[0].ID, "resume")
	if r := respListed(t, s, id); r.State != respActive {
		t.Fatalf("the card's action resumes the watch: %+v", r)
	}
	if j, _ := s.respFindJob(id); !j.Enabled {
		t.Fatal("a resumed watch runs again")
	}
	if n := len(s.avuiView()["now"].([]avuiCard)); n != 0 {
		t.Fatalf("the pause card leaves once resumed: %d", n)
	}
	if rec := b8Post(t, s.handleWindsAvui, map[string]string{"id": cards[0], "action": "resume"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("only a pause card resumes: %d", rec.Code)
	}
}

func TestAvuiUsefulResetsTheNoes(t *testing.T) {
	s := respTestServer(t)
	page := newRespPage(t, "Horari: 8:00")
	id := respCreateWeb(t, s, page.srv.URL)
	cards := avuiRespCards(t, s, "responsabilitat", id, 6)

	avuiPost(t, s, cards[0], "dismiss")
	avuiPost(t, s, cards[1], "dismiss")
	avuiPost(t, s, cards[2], "useful")
	avuiPost(t, s, cards[3], "dismiss")
	avuiPost(t, s, cards[4], "notuseful")
	if r := respListed(t, s, id); r.Budget.MessagesPerDay != 3 {
		t.Fatalf("useful on a later card starts the count again: %+v", r.Budget)
	}
	avuiPost(t, s, cards[5], "dismiss")
	if r := respListed(t, s, id); r.Budget.MessagesPerDay != 1 {
		t.Fatalf("three noes after the useful one halve it: %+v", r.Budget)
	}
}

func TestAvuiHeartbeatIsOnlySlowedNeverPaused(t *testing.T) {
	s := respTestServer(t)
	if _, err := s.jobs.Save(jobs.Job{ID: "wind-xaloc", Name: "Cor de Xaloc", Prompt: "vigila", Kind: "event", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	respListed(t, s, "cor-xaloc")
	cards := avuiRespCards(t, s, "heartbeat", "", 5)
	for _, c := range cards {
		avuiPost(t, s, c, "dismiss")
	}
	r := respListed(t, s, "cor-xaloc")
	if r.State != respActive || r.Budget.MessagesPerDay != 1 || len(respLogNotes(r, "budget")) != 2 {
		t.Fatalf("the heartbeat is slowed twice, never paused: %+v", r)
	}
	if j, _ := s.respFindJob("wind-xaloc"); !j.Enabled {
		t.Fatal("the heartbeat's job keeps running")
	}
	if n := len(s.avuiView()["now"].([]avuiCard)); n != 0 {
		t.Fatalf("no pause card for the heartbeat: %d", n)
	}
}

func TestAvuiRemindersWatchIsOnlySlowedNeverPaused(t *testing.T) {
	s := respTestServer(t)
	code, out := respCall(t, s, map[string]any{"action": "create", "wind": "xaloc", "objective": "Avisa'm dels meus recordatoris", "agenda": true})
	if code != 200 {
		t.Fatalf("create: %d %v", code, out)
	}
	id := out["responsabilitat"].(map[string]any)["id"].(string)
	for _, c := range avuiRespCards(t, s, "responsabilitat", id, 5) {
		avuiPost(t, s, c, "notuseful")
	}
	r := respListed(t, s, id)
	if r.State != respActive || r.Budget.MessagesPerDay != 1 || len(respLogNotes(r, "budget")) != 2 {
		t.Fatalf("a reminders watch is slowed, never paused: %+v", r)
	}
}
