package web

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/jobs"
)

// TestPetHistoryPortaHora: la història que veu l'app porta l'hora (ts) dels
// missatges nous; els antics continuen sense ts (0 = desconegut).
func TestPetHistoryPortaHora(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	w, _ := s.winds.ByID("xaloc")
	saved := windLoop
	windLoop = func(_ *Server, _ context.Context, _ string) (exploracio, error) {
		return exploracio{text: "bon dia"}, nil
	}
	t.Cleanup(func() { waitBackground(); windLoop = saved })
	rec := postJSON(t, s.handleWindChat, `{"wind":"`+w.ID+`","message":"hola"}`)
	if rec.Code != 200 {
		t.Fatalf("esperava 200, va %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		History []map[string]any `json:"history"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.History) < 2 {
		t.Fatalf("historial curt: %d", len(out.History))
	}
	for i, m := range out.History {
		if ts, _ := m["ts"].(float64); ts <= 0 {
			t.Errorf("missatge %d sense ts: %v", i, m)
		}
	}
}

// TestJobDeVentEscriuAlXat: qualsevol job d'un vent (per exemple el butlletí
// d'una feina seva) entra al seu xat i queda com a novetat perquè el mòbil
// pugui avisar — no només al registre de runs.
func TestJobDeVentEscriuAlXat(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	w, _ := s.winds.ByID("xaloc")
	job, err := s.jobs.Save(jobs.Job{ID: "feina-" + w.ID, Name: "Butlletí", Prompt: "escriu una frase", Wind: w.ID, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	provider, entered, release := v2Provider(t, "# BUTLLETÍ - Bon dia, mar en calma.")
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
	type result struct {
		run jobs.Run
		err error
	}
	ch := make(chan result, 1)
	go func() {
		run, runErr := s.runJob(job.ID, true)
		ch <- result{run: run, err: runErr}
	}()
	<-entered
	close(release)
	got := <-ch
	if got.err != nil || got.run.Status != "ok" {
		t.Fatalf("job: run=%+v err=%v", got.run, got.err)
	}
	hist := windHistory(s, w.ID)
	var ultim string
	for _, m := range hist.messages {
		if m.Role == "assistant" {
			ultim = m.Content
		}
	}
	if !strings.Contains(ultim, "BUTLLETÍ") {
		t.Fatalf("el butlletí no ha entrat al xat del vent: %q", ultim)
	}
	nov := loadNovetats(s)
	found := false
	for _, n := range nov {
		if n.Wind == w.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("cap novetat registrada per al job del vent %s", w.ID)
	}
}
