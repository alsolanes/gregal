package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/jobs"
)

// Un run manual d'una rutina esborrada (runs antics encara visibles) s'ha de
// rebutjar amb 404: si no, el goroutine moriria en silenci i l'app quedaria
// esperant una execució que mai arriba.
func TestRunDeRutinaEsborrada(t *testing.T) {
	s := remindTestServer(t)
	// Un job que JA NO ÉS a la llista (esborrat), amb runs antics encara al disc.
	if _, err := s.jobs.Save(jobs.Job{ID: "wind-fantasma", Name: "Cor fantasma", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.jobs.AddRun(jobs.Run{JobID: "wind-fantasma", JobName: "Cor fantasma", Status: "error", Error: "context deadline exceeded"}); err != nil {
		t.Fatal(err)
	}
	if err := s.jobs.Delete("wind-fantasma"); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleJobRun(rec, httptest.NewRequest("POST", "/api/jobs/run", strings.NewReader(`{"id":"wind-fantasma"}`)))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("volia 404, té %d: %s", rec.Code, rec.Body.String())
	}
	// ...però els runs antics es poden seguir VEIENT.
	rec2 := httptest.NewRecorder()
	s.handleJobRuns(rec2, httptest.NewRequest("GET", "/api/jobs/runs?job=wind-fantasma", nil))
	var got struct {
		Runs []map[string]any `json:"runs"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Runs) == 0 {
		t.Fatal("els runs antics s'haurien de poder veure")
	}
}
