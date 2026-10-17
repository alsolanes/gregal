package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gregal/internal/events"
	"gregal/internal/llm"
)

// Un torn que el procés anterior va deixar a mitges es tanca en arrencar:
// el mòbil el seguia pel registre i, sense run_failed, esperava per sempre.
func TestCloseOrphanRunsFailsUnfinishedRuns(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	h := s.Hub()
	for _, e := range []events.Event{
		{SessionID: "mobil", RunID: 7, Kind: "run_queued"},
		{SessionID: "mobil", RunID: 7, Kind: "run_started"},
		{SessionID: "mobil", RunID: 8, Kind: "run_queued"},
		{SessionID: "mobil", RunID: 8, Kind: "run_completed"},
	} {
		if _, err := h.eventStore.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	if n := s.CloseOrphanRuns(); n != 1 {
		t.Fatalf("closed %d runs, want 1", n)
	}
	got, err := h.eventStore.AfterScope(0, 100, "", "mobil")
	if err != nil {
		t.Fatal(err)
	}
	last := got[len(got)-1]
	if last.RunID != 7 || last.Kind != "run_failed" {
		t.Fatalf("last event: %+v", last)
	}
	if n := s.CloseOrphanRuns(); n != 0 {
		t.Fatalf("second call closed %d runs", n)
	}
}

// /api/sessions la serveix la sessió default, però «current» ha de ser la
// conversa de qui pregunta: l'app mòbil, amb la seva sessió, ho fa servir
// per reprendre-la després que el sistema li mati el backend.
func TestSessionsCurrentFollowsCallerSession(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	h := s.Hub()
	mobile := h.Session("mobil")
	for _, srv := range []*Server{s, mobile} {
		srv.mu.Lock()
		srv.convo = append(srv.convo, llm.Message{Role: "user", Content: "hola " + srv.id})
		srv.autosaveLocked()
		srv.mu.Unlock()
	}
	r := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	r.Header.Set("X-Gregal-Session", "mobil")
	w := httptest.NewRecorder()
	h.Handler().ServeHTTP(w, r)
	var rows []struct {
		Name    string `json:"name"`
		Current bool   `json:"current"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatalf("%v: %s", err, w.Body.String())
	}
	mobile.mu.Lock()
	want := mobile.convoFile
	mobile.mu.Unlock()
	cur := ""
	for _, row := range rows {
		if row.Current {
			cur = row.Name
		}
	}
	if cur != want || want == "" {
		t.Fatalf("current = %q, want the mobile conversation %q (rows %+v)", cur, want, rows)
	}
}
