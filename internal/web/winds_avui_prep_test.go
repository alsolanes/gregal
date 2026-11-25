package web

import (
	"context"
	"strings"
	"testing"
	"time"
)

// stubPrepLoop fa de model: torna reply i apunta les consignes.
func stubPrepLoop(t *testing.T, reply string) *[]string {
	t.Helper()
	var briefs []string
	old := prepLoop
	prepLoop = func(_ *Server, _ context.Context, brief string) (exploracio, error) {
		briefs = append(briefs, brief)
		return exploracio{text: reply}, nil
	}
	t.Cleanup(func() { prepLoop = old })
	return &briefs
}

func icsAllDay(uid, summary string, day time.Time) string {
	return "BEGIN:VEVENT\r\nUID:" + uid + "\r\nSUMMARY:" + summary + "\r\nDTSTART;VALUE=DATE:" + day.Format("20060102") +
		"\r\nDTEND;VALUE=DATE:" + day.AddDate(0, 0, 1).Format("20060102") + "\r\nEND:VEVENT\r\n"
}

// prepServer connecta un calendari amb events i torna la feina de preparació,
// creada en mirar «Avui» per primer cop.
func prepServer(t *testing.T, events ...string) (*Server, func() []avuiCard) {
	t.Helper()
	s := localTestServer(t)
	fakeICSServer(t, icsWith(events...))
	if w := connPost(t, s, `{"add_calendar":{"name":"Feina","url":"https://calendar.example.org/private-prep.ics"}}`); w.Code != 200 {
		t.Fatalf("add calendar: %s", w.Body.String())
	}
	getGlance(t, s)
	job, ok := s.ownWindJob(prepJobID(""))
	if !ok || job.Kind != "interval" || !job.Enabled || job.IntervalH <= 0 || job.IntervalH > 1 {
		t.Fatalf("a connected calendar gets a prep job: %+v %v", job, ok)
	}
	run := func() []avuiCard {
		t.Helper()
		if r, err := s.executeJob(context.Background(), job, runsZero()); err != nil || r.Status != "ok" {
			t.Fatalf("prep run: %+v %v", r, err)
		}
		var out []avuiCard
		for _, c := range s.avuiView()["now"].([]avuiCard) {
			if c.Source == "prep" {
				out = append(out, c)
			}
		}
		return out
	}
	return s, run
}

func TestAvuiPrepCardForTheNextMeeting(t *testing.T) {
	now := time.Now().Truncate(time.Minute)
	s, run := prepServer(t,
		icsEvent("a@x", "Revisió de pressupost", now.Add(60*time.Minute)),
		icsEvent("b@x", "Massa aviat", now.Add(30*time.Minute)),
		icsEvent("c@x", "Massa tard", now.Add(150*time.Minute)),
		icsEvent("d@x", "", now.Add(70*time.Minute)),
		icsAllDay("e@x", "Vacances", now.Add(60*time.Minute).In(time.Local)),
	)
	briefs := stubPrepLoop(t, "La Marta va enviar el full de càlcul dilluns.\nPorta les xifres del Q3.")
	cards := run()
	hhmm := now.Add(60 * time.Minute).In(s.loadConnections().zone()).Format("15:04")
	if len(cards) != 1 || cards[0].Title != "Prep: Revisió de pressupost · "+hhmm || cards[0].Section != avuiNow {
		t.Fatalf("one now card for the meeting in an hour: %+v", cards)
	}
	if !strings.Contains(cards[0].Body, "full de càlcul") || len(cards[0].Actions) == 0 || cards[0].Held {
		t.Fatalf("the body is the read-only turn's: %+v", cards[0])
	}
	if len(*briefs) != 1 || !strings.Contains((*briefs)[0], "Revisió de pressupost") {
		t.Fatalf("one model turn, about that meeting: %q", *briefs)
	}
	n := loadNovetats(s)
	if len(n) != 1 || n[0].Source != "prep" || n[0].Quiet {
		t.Fatalf("outside quiet hours it rings: %+v", n)
	}
}

func TestAvuiPrepOncePerEventThreePerDayAndNotUsefulStopsIt(t *testing.T) {
	now := time.Now().Truncate(time.Minute)
	_, run := prepServer(t,
		icsEvent("a@x", "Un a un amb la Joana", now.Add(50*time.Minute)),
		icsEvent("b@x", "Comitè", now.Add(60*time.Minute)),
		icsEvent("c@x", "Client", now.Add(70*time.Minute)),
		icsEvent("d@x", "Equip", now.Add(80*time.Minute)),
	)
	briefs := stubPrepLoop(t, "Notes de preparació.")
	if cards := run(); len(cards) != 3 || len(*briefs) != 3 {
		t.Fatalf("at most 3 per day: %d cards, %d turns", len(cards), len(*briefs))
	}
	if cards := run(); len(cards) != 3 || len(*briefs) != 3 {
		t.Fatalf("a second check repeats nothing: %d cards, %d turns", len(cards), len(*briefs))
	}

	// «No m'interessa» atura la preparació d'events amb el mateix títol.
	s2, run2 := prepServer(t, icsEvent("a@x", "Un a un amb la Joana", now.Add(60*time.Minute)))
	briefs = stubPrepLoop(t, "Notes.")
	cards := run2()
	if len(cards) != 1 {
		t.Fatalf("first: %+v", cards)
	}
	avuiPost(t, s2, cards[0].ID, "notuseful")
	// La setmana vinent, el mateix un a un.
	avuiMu.Lock()
	list := loadAvui(s2)
	for i := range list {
		list[i].Created -= 7 * 24 * 3600
	}
	_ = saveAvui(s2, list)
	avuiMu.Unlock()
	if cards := run2(); len(cards) != 0 || len(*briefs) != 1 {
		t.Fatalf("notuseful stops prep for that title: %d cards, %d turns", len(cards), len(*briefs))
	}
}

func TestAvuiPrepDuringQuietHoursIsHeldAndSilent(t *testing.T) {
	now := time.Now().Truncate(time.Minute)
	s, run := prepServer(t, icsEvent("a@x", "Trucada amb Lisboa", now.Add(90*time.Minute)))
	local := time.Now().In(s.loadConnections().zone())
	avuiSettingsPost(t, s, `{"quiet_from":"`+local.Add(-time.Hour).Format("15:04")+`","quiet_to":"`+local.Add(time.Hour).Format("15:04")+`"}`)
	stubPrepLoop(t, "Res de nou per correu.")
	cards := run()
	if len(cards) != 1 || !cards[0].Held {
		t.Fatalf("held in quiet hours: %+v", cards)
	}
	if n := loadNovetats(s); len(n) != 1 || !n[0].Quiet {
		t.Fatalf("it does not ring: %+v", n)
	}
}

func TestAvuiPrepNovetatCarriesItsCard(t *testing.T) {
	now := time.Now().Truncate(time.Minute)
	s, run := prepServer(t, icsEvent("a@x", "Revisió de pressupost", now.Add(60*time.Minute)))
	stubPrepLoop(t, "La Marta va enviar el full de càlcul dilluns.")
	cards := run()
	n := loadNovetats(s)
	if len(cards) != 1 || len(n) != 1 || n[0].Card != cards[0].ID {
		t.Fatalf("the prep novetat carries its Today card: %+v %+v", cards, n)
	}
	avuiPost(t, s, cards[0].ID, "done")
	if n := loadNovetats(s); !n[0].Read {
		t.Fatalf("deciding the card marks the novetat read: %+v", n)
	}
}
