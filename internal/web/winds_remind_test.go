package web

import (
	"context"
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
func TestParseRemindAtLenient(t *testing.T) {
	now := time.Now()
	cases := []string{
		now.Add(2 * time.Hour).Format(time.RFC3339),
		now.Add(2 * time.Hour).Format("2006-01-02 15:04"),
		now.Add(2 * time.Hour).Format("02/01/2006 15:04"),
		"+2h",
		"+30m",
	}
	for _, c := range cases {
		at, err := parseRemindAt(c, now)
		if err != nil {
			t.Fatalf("parseRemindAt(%q): %v", c, err)
		}
		if at.Before(now.Add(time.Minute)) || at.After(now.Add(30*24*time.Hour)) {
			t.Fatalf("parseRemindAt(%q) = %v, fora de rang", c, at)
		}
	}
	if _, err := parseRemindAt("demà a les 9", now); err == nil {
		t.Fatal("havia de rebutjar text lliure")
	}
	// Una data explícita del passat es parseja (el rebuig és feina de
	// saveWindReminder, que mira el rellotge real).
	if at, err := parseRemindAt(now.Add(-time.Hour).Format(time.RFC3339), now); err != nil || !at.Before(now) {
		t.Fatalf("parse del passat = %v, %v", at, err)
	}
}

func TestRemindActionsSince(t *testing.T) {
	t0 := time.Now()
	recordRemindEvent("", "remind-vent-prova-1", false, "vent-prova", t0.Add(time.Hour), "Rega les plantes")
	got := remindActionsSince("", "vent-prova", t0)
	if len(got) != 1 || got[0]["at"] == "" || got[0]["text"] != "Rega les plantes" {
		t.Fatalf("accions = %v", got)
	}
	if len(remindActionsSince("", "un-altre", t0)) != 0 {
		t.Fatal("no ha de barrejar vents")
	}
	if len(remindActionsSince("", "vent-prova", t0.Add(time.Hour))) != 0 {
		t.Fatal("no ha de tornar esdeveniments vells")
	}
}

// Dos comptes amb un vent del mateix nom no es veuen els avisos, i l'app rep
// el moment exacte per programar la passada.
func TestRemindActionsAreScopedToTheAccount(t *testing.T) {
	t0 := time.Now()
	at := t0.Add(2 * time.Minute).Truncate(time.Second)
	recordRemindEvent("marta", "remind-xaloc-comu-1", false, "xaloc-comu", at, "Apaga el foc")
	if len(remindActionsSince("aleix", "xaloc-comu", t0)) != 0 {
		t.Fatal("un altre compte veu l'avís")
	}
	got := remindActionsSince("marta", "xaloc-comu", t0)
	if len(got) != 1 {
		t.Fatalf("accions = %v", got)
	}
	if iso, err := time.Parse(time.RFC3339, got[0]["at_iso"]); err != nil || !iso.Equal(at) {
		t.Fatalf("at_iso = %q (%v), want %v", got[0]["at_iso"], err, at)
	}
}

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
	run, err := s.executeWindRemind(context.Background(), j)
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

// TestRemindListReturnsPendingReminders: l'eina recorda els pendents.
// El vent ha de poder respondre «quins recordatoris tinc?» — abans no
// tenia cap eina de consulta i deia que no podia.
func TestRemindListReturnsPendingReminders(t *testing.T) {
	s := remindTestServer(t)
	at := time.Now().Add(3 * time.Hour)
	if _, err := saveWindReminder(s, "xaloc", at.Format(time.RFC3339), "Rega les plantes"); err != nil {
		t.Fatal(err)
	}
	items, err := s.remindListItems("")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 pending reminder, got %d", len(items))
	}
	if items[0]["titol"] != "Rega les plantes" {
		t.Fatalf("titol = %q", items[0]["titol"])
	}
	if items[0]["vent"] != "Xaloc" {
		t.Fatalf("vent = %q", items[0]["vent"])
	}
	if items[0]["estat"] != "actiu" {
		t.Fatalf("estat = %q", items[0]["estat"])
	}
	// Filtre per vent: només els seus.
	if items, err := s.remindListItems("xaloc"); err != nil || len(items) != 1 {
		t.Fatalf("per xaloc: %v %d", err, len(items))
	}
	if items, err := s.remindListItems("mestral"); err != nil || len(items) != 0 {
		t.Fatalf("per mestral: %v %d", err, len(items))
	}
	// Un recordatori ja sonat i apagat és història: no surt a la llista.
	j, err := s.jobs.List()
	if err != nil || len(j) != 1 {
		t.Fatal(err)
	}
	gone := j[0]
	gone.Enabled = false
	la := time.Now()
	gone.LastAt = &la
	if _, err := s.jobs.Save(gone); err != nil {
		t.Fatal(err)
	}
	if items, err := s.remindListItems(""); err != nil || len(items) != 0 {
		t.Fatalf("sonat encara surt: %v %d", err, len(items))
	}
}

// TestRemindListEmptyIsNotAnError: sense recordatoris, llista buida amb
// missatge clar (el model no ha de dir «no tinc l'eina»).
func TestRemindListEmptyIsNotAnError(t *testing.T) {
	s := remindTestServer(t)
	items, err := s.remindListItems("")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0, got %d", len(items))
	}
}
