package web

import (
	"context"
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/jobs"
	"gregal/internal/runs"
)

// TestWindHeartbeatReschedulesItself: el cor del vent es reprograma sol.
// Quan troba el proper esdeveniment (p. ex. un partit), la línia NEXT del
// final de la resposta programa la propera mirada al job (kind event) —
// l'endemà a dos quarts d'onze, no l'endemà al matí.
func TestWindHeartbeatReschedulesItself(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"#FF9F00","role":"futbol: partits del Barça"}`)
	// Creem el job del cor com fa l'app (kind event).
	saved, err := s.jobs.Save(jobs.Job{ID: "wind-xaloc", Name: "Cor de Xaloc", Prompt: "vigila", Kind: "event", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Kind != "event" {
		t.Fatalf("kind event no acceptat: %q", saved.Kind)
	}

	matchEnd := time.Now().Add(36 * time.Hour).UTC().Truncate(time.Second)
	reply := "Partit acabat: el Barça ha guanyat 2-0. [mundodeportivo.com](https://mundodeportivo.com)\nNEXT: " + matchEnd.Format(time.RFC3339)
	old := windHeartbeatLoop
	windHeartbeatLoop = func(_ *Server, _ context.Context, _ string) (exploracio, error) {
		return exploracio{text: reply}, nil
	}
	t.Cleanup(func() { windHeartbeatLoop = old })

	if _, err := s.executeWindHeartbeat(context.Background(), jobs.Job{ID: "wind-xaloc", Name: "Cor de Xaloc"}, runs.Run{}, companion.Wind{ID: "xaloc", Name: "Xaloc", Role: "futbol", Persona: "ets en Xaloc"}); err != nil {
		t.Fatal(err)
	}
	list, err := s.jobs.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].NextAt == nil || !list[0].NextAt.Equal(matchEnd) {
		t.Fatalf("el cor no s'ha reprogramat al final del partit: %+v", list)
	}
	// I la línia NEXT no ha entrat al xat: és programació, no parla.
	hist := windHistory(s, "xaloc")
	hist.mu.Lock()
	last := hist.messages[len(hist.messages)-1]
	hist.mu.Unlock()
	if strings.Contains(last.Content, "NEXT:") {
		t.Fatalf("la línia NEXT ha entrat al xat: %q", last.Content)
	}
	if !strings.Contains(last.Content, "Barça ha guanyat") {
		t.Fatalf("el missatge del partit no ha entrat al xat: %q", last.Content)
	}
}

// TestWindHeartbeatNextParse: la línia NEXT del model es llegeix bé — amb
// zona, amb cometes, i sense línia no reprograma res d'estranqui.
func TestWindHeartbeatNextParse(t *testing.T) {
	ok, good := time.Date(2026, 10, 21, 20, 50, 0, 0, time.UTC), true
	cases := []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"NEXT: 2026-10-21T20:50:00Z", ok, good},
		{"NEXT: 2026-10-21T22:50:00+02:00", ok, good},
		{"`NEXT: 2026-10-21T20:50:00Z`", ok, good},
		{"next: 2026-10-21T20:50:00Z", ok, good},
		{"NEXT: demà a dos quarts", time.Time{}, false},
		{"", time.Time{}, false},
	}
	for _, c := range cases {
		got, gotOk := windNextAt(c.in)
		if gotOk != c.ok || (gotOk && !got.Equal(c.want)) {
			t.Errorf("windNextAt(%q) = %v,%v; want %v,%v", c.in, got, gotOk, c.want, c.ok)
		}
	}
	// I strip: només treu les línies NEXT.
	if got := windStripNext("a\nNEXT: 2026-10-21T20:50:00Z\nb"); got != "a\nb" {
		t.Errorf("windStripNext = %q", got)
	}
	if got := windStripNext("a\nb"); got != "a\nb" {
		t.Errorf("windStripNext sense NEXT = %q", got)
	}
}
