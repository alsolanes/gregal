package web

import (
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/jobs"
)

func remindTestServer(t *testing.T) *Server {
	t.Helper()
	s, _ := companionTestServer(t)
	s.winds = companion.NewWindStore(t.TempDir())
	if _, err := s.winds.Save(companion.Wind{Name: "Xaloc", Color: "#EFA68F"}); err != nil {
		t.Fatal(err)
	}
	js, err := jobs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.jobs = js
	return s
}

// TestSaveWindReminderValid: el vent desa un avís futur amb el seu text.
func TestSaveWindReminderValid(t *testing.T) {
	s := remindTestServer(t)
	at := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	j, err := saveWindReminder(s, "xaloc", at, "Passa la ITV")
	if err != nil {
		t.Fatal(err)
	}
	if j.Kind != "remind" || !j.Enabled || j.NextAt == nil {
		t.Fatalf("job mal desat: %+v", j)
	}
	if j.Prompt != "Passa la ITV" {
		t.Fatalf("prompt = %q", j.Prompt)
	}
	list, _ := s.jobs.List()
	if len(list) != 1 {
		t.Fatalf("%d jobs, en vull 1", len(list))
	}
}

// TestSaveWindReminderRebutja: vent inexistent, data passada, text buit o
// massa llarg no desen res.
func TestSaveWindReminderRebutja(t *testing.T) {
	s := remindTestServer(t)
	futur := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	passat := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	cases := []struct {
		name string
		wind string
		at   string
		text string
	}{
		{"vent inexistent", "llevant", futur, "Hola"},
		{"data passada", "xaloc", passat, "Hola"},
		{"data il·legible", "xaloc", "demà a les nou", "Hola"},
		{"text buit", "xaloc", futur, "   "},
		{"text massa llarg", "xaloc", futur, strings.Repeat("a", 501)},
	}
	for _, c := range cases {
		if _, err := saveWindReminder(s, c.wind, c.at, c.text); err == nil {
			t.Fatalf("%s: hauria de fallar", c.name)
		}
	}
	if list, _ := s.jobs.List(); len(list) != 0 {
		t.Fatalf("%d jobs desats, en vull 0", len(list))
	}
}

// TestExecuteRemindPostsAndDisables: quan toca, el text entra al xat del vent
// com si l'hagués dit ell, la sortida (notificació) el porta i el job
// s'apaga: d'una sola execució, mai es repeteix.
func TestExecuteRemindPostsAndDisables(t *testing.T) {
	s := remindTestServer(t)
	at := time.Now().Add(-time.Minute).UTC()
	j, err := s.jobs.Save(jobs.Job{
		ID: "remind-xaloc-1", Name: "Recordatori Xaloc", Kind: "remind",
		Prompt: "Passa la ITV", Enabled: true, Notify: true, NextAt: &at,
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.executeWindRemind(j)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "ok" {
		t.Fatalf("status = %q (%s)", run.Status, run.Error)
	}
	if run.Output != "Xaloc: Passa la ITV" {
		t.Fatalf("output = %q", run.Output)
	}
	found := false
	for _, m := range windHistory(s, "xaloc").messages {
		if m.Role == "assistant" && m.Content == "Passa la ITV" {
			found = true
		}
	}
	if !found {
		t.Fatal("el text no ha entrat al xat del vent")
	}
	list, _ := s.jobs.List()
	if len(list) != 1 || list[0].Enabled {
		t.Fatal("el recordatori s'hauria d'haver apagat després de disparar")
	}
}
