package web

import (
	"context"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/jobs"
	"gregal/internal/runs"
)

// TestWindHeartbeatSilenceWithNext (F01): el prompt demana «(res)» i, a més,
// la línia NEXT. Aquella resposta és silenci: cap missatge al xat i cap
// sortida de job (que és el que porta la notificació). Abans es comparava
// el text sencer amb «(res)» i la línia NEXT feia que es publiqués.
func TestWindHeartbeatSilenceWithNext(t *testing.T) {
	next := time.Now().Add(6 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339)
	replies := []string{
		"(res)\nNEXT: " + next,
		"(res)\n\nnext: " + next,
		"`(res)`\n`NEXT: " + next + "`",
		"(Res).\n  NEXT: " + next,
		"NEXT: " + next,
	}
	for _, reply := range replies {
		t.Run(reply, func(t *testing.T) {
			t.Setenv("GREGAL_DATA_DIR", t.TempDir())
			s, _ := companionTestServer(t)
			postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"#FF9F00","role":"futbol"}`)
			if _, err := s.jobs.Save(jobs.Job{ID: "wind-xaloc", Name: "Cor de Xaloc", Prompt: "vigila", Kind: "event", Enabled: true}); err != nil {
				t.Fatal(err)
			}
			old := windHeartbeatLoop
			windHeartbeatLoop = func(_ *Server, _ context.Context, _ string) (exploracio, error) {
				return exploracio{text: reply}, nil
			}
			t.Cleanup(func() { waitBackground(); windHeartbeatLoop = old })

			hist := windHistory(s, "xaloc")
			hist.mu.Lock()
			before := len(hist.messages)
			hist.mu.Unlock()

			run, err := s.executeWindHeartbeat(context.Background(), jobs.Job{ID: "wind-xaloc", Name: "Cor de Xaloc"}, runs.Run{},
				companion.Wind{ID: "xaloc", Name: "Xaloc", Role: "futbol", Persona: "ets en Xaloc"})
			if err != nil {
				t.Fatal(err)
			}
			if run.Output != "" || run.Status != "ok" {
				t.Fatalf("silenci amb sortida (notificaria): %+v", run)
			}
			hist.mu.Lock()
			after := len(hist.messages)
			hist.mu.Unlock()
			if after != before {
				t.Fatalf("el silenci ha entrat al xat: %d → %d missatges", before, after)
			}
			// Silenci no vol dir perdre la programació: la propera mirada es desa.
			list, _ := s.jobs.List()
			if len(list) != 1 || list[0].NextAt == nil {
				t.Fatalf("el cor en silenci no s'ha reprogramat: %+v", list)
			}
		})
	}
}

// TestWindHeartbeatStripNextVariants: la línia NEXT surt del xat amb el
// mateix criteri amb què es llegeix (minúscules, cometes, espais).
func TestWindHeartbeatStripNextVariants(t *testing.T) {
	for _, in := range []string{
		"Hi ha partit.\nnext: 2026-12-01T10:00:00Z",
		"Hi ha partit.\n`NEXT: 2026-12-01T10:00:00Z`",
		"Hi ha partit.\n   NEXT:2026-12-01T10:00:00Z",
	} {
		if got := windStripNext(in); got != "Hi ha partit." {
			t.Errorf("windStripNext(%q) = %q", in, got)
		}
	}
	// Una frase que parla de «next» no és la programació.
	if got := windStripNext("The next match is Sunday."); got != "The next match is Sunday." {
		t.Errorf("ha tret text normal: %q", got)
	}
}

// TestWindClampNext (F02): una data passada, massa propera o d'aquí a anys
// no pot fer córrer el cor en bucle ni deixar-lo mut indefinidament.
func TestWindClampNext(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		at   time.Time
		ok   bool
		want time.Time
	}{
		{"sense NEXT", time.Time{}, false, now.Add(windNextDefault)},
		{"passat", now.Add(-48 * time.Hour), true, now.Add(windNextMin)},
		{"ara mateix", now, true, now.Add(windNextMin)},
		{"d'aquí a 5 min", now.Add(5 * time.Minute), true, now.Add(windNextMin)},
		{"raonable", now.Add(36 * time.Hour), true, now.Add(36 * time.Hour)},
		{"d'aquí a anys", now.AddDate(3, 0, 0), true, now.Add(windNextMax)},
	}
	for _, c := range cases {
		if got := windClampNext(c.at, c.ok, now); !got.Equal(c.want) {
			t.Errorf("%s: windClampNext = %v; want %v", c.name, got, c.want)
		}
	}
}
