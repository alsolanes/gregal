package web

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"gregal/internal/team"
)

// TestResumDeFeinesPerAlMobil: la llista lleugera diu l'estat (també si va
// quedar a mitges), els fitxers que són el resultat i la resposta final.
func TestResumDeFeinesPerAlMobil(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	ws := t.TempDir()
	s.cwd = ws
	emit, err := s.recordWindTask("Fes una web", ws, "rot-9", &team.FlowSpec{Nodes: []team.FlowNode{{ID: "builder", Label: "Tramuntana"}}})
	if err != nil {
		t.Fatal(err)
	}
	emit(team.Event{Type: "tool_result", Agent: "builder", Path: "index.html"})
	emit(team.Event{Type: "tool_result", Agent: "builder", Path: "_harness.html"})
	emit(team.Event{Type: "tool_result", Agent: "builder", Path: "index.html"})

	llista := func() []windTaskSummary {
		rec := httptest.NewRecorder()
		s.handleWindTasks(rec, httptest.NewRequest("GET", "/api/winds/tasks?summary=1", nil))
		var out struct {
			Tasks []windTaskSummary `json:"tasks"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Tasks) != 1 {
			t.Fatalf("resposta: %s", rec.Body.String())
		}
		return out.Tasks
	}
	got := llista()[0]
	if got.Status != "running" || got.Thread != "rot-9" || len(got.Files) != 1 || got.Files[0] != "index.html" {
		t.Fatalf("en marxa: %+v", got)
	}

	// El servidor es reinicia a mitja feina: ja no és viva.
	liveWindTasks.Delete(got.ID)
	if st := llista()[0].Status; st != "interrupted" {
		t.Fatalf("una feina a mitges després d'un reinici: %q", st)
	}

	liveWindTasks.Store(got.ID, true)
	emit(team.Event{Type: "done", Output: "He fet index.html amb la portada."})
	got = llista()[0]
	if got.Status != "done" || got.Result == "" {
		t.Fatalf("acabada: %+v", got)
	}
}
