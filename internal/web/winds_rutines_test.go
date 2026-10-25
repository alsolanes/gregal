package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/jobs"
)

// Les rutines de vent són jobs daily/interval amb Wind: quan corre el
// scheduler, el vent obre un torn real (LLM + eines) i parla al seu xat
// només si troba algo. Aquí provem l'API: crear, validar, llistar, esborrar.

func rutinaTestServer(t *testing.T) *Server {
	t.Helper()
	s, _ := companionTestServer(t)
	s.winds = companion.NewWindStore(t.TempDir())
	if _, err := s.winds.Save(companion.Wind{Name: "Xaloc", Color: "#EFA68F", Role: "Música i descobertes"}); err != nil {
		t.Fatal(err)
	}
	js, err := jobs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.jobs = js
	return s
}

func rutinaPost(t *testing.T, s *Server, url string, body map[string]any, v any) int {
	t.Helper()
	raw, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(string(raw)))
	if strings.HasSuffix(url, "/delete") {
		s.handleWindsRutinaDelete(rec, req)
	} else {
		s.handleWindsRutina(rec, req)
	}
	if v != nil && rec.Code == 200 {
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatal(err)
		}
	}
	return rec.Code
}

func TestRutinaDailyCreada(t *testing.T) {
	s := rutinaTestServer(t)
	w, _ := s.winds.ByRef("xaloc")

	var resp struct {
		Job struct {
			ID        string  `json:"id"`
			Wind      string  `json:"wind"`
			Kind      string  `json:"kind"`
			Time      string  `json:"time"`
			NextDue   string  `json:"next_due"`
			IntervalH float64 `json:"interval_h"`
		} `json:"job"`
	}
	if code := rutinaPost(t, s, "/api/winds/rutina", map[string]any{
		"wind": w.ID, "kind": "daily", "time": "09:00",
		"prompt": "Mira si hi ha concerts nous a Barcelona",
	}, &resp); code != http.StatusOK {
		t.Fatalf("crear: %d", code)
	}
	if resp.Job.ID == "" || !strings.HasPrefix(resp.Job.ID, "rutina-") {
		t.Fatalf("id de rutina dolent: %q", resp.Job.ID)
	}
	if resp.Job.Wind != w.ID || resp.Job.Kind != "daily" || resp.Job.Time != "09:00" {
		t.Fatalf("rutina mal desada: %+v", resp.Job)
	}
	if resp.Job.NextDue == "" {
		t.Fatal("una rutina daily ha de tenir pròxima franxa")
	}

	// La rutina surt a la llista.
	rec := httptest.NewRecorder()
	s.handleWindsRutines(rec, httptest.NewRequest(http.MethodGet, "/api/winds/rutines", nil))
	var llista struct {
		Rutines []struct {
			ID   string `json:"id"`
			Wind string `json:"wind"`
		} `json:"rutines"`
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("llista: %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &llista); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range llista.Rutines {
		if r.ID == resp.Job.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("la rutina %s no surt a la llista", resp.Job.ID)
	}

	// I es pot esborrar.
	var del map[string]bool
	if code := rutinaPost(t, s, "/api/winds/rutines/delete", map[string]any{"id": resp.Job.ID}, &del); code != http.StatusOK || !del["ok"] {
		t.Fatalf("esborrat: %d %v", code, del)
	}
	list, _ := s.jobs.List()
	for _, j := range list {
		if j.ID == resp.Job.ID {
			t.Fatal("la rutina no s'ha esborrat del store")
		}
	}
}

func TestRutinaIntervalValida(t *testing.T) {
	s := rutinaTestServer(t)
	w, _ := s.winds.ByRef("xaloc")

	var resp struct {
		Job struct {
			ID        string  `json:"id"`
			Kind      string  `json:"kind"`
			IntervalH float64 `json:"interval_h"`
		} `json:"job"`
	}
	if code := rutinaPost(t, s, "/api/winds/rutina", map[string]any{
		"wind": w.ID, "kind": "interval", "interval_h": 6,
		"prompt": "Vigila CVEs noves de PHP",
	}, &resp); code != http.StatusOK {
		t.Fatalf("crear interval: %d", code)
	}
	if resp.Job.Kind != "interval" || resp.Job.IntervalH != 6 {
		t.Fatalf("interval mal desat: %+v", resp.Job)
	}
}

func TestRutinaRebutja(t *testing.T) {
	s := rutinaTestServer(t)
	w, _ := s.winds.ByRef("xaloc")

	cases := []struct {
		name string
		body map[string]any
	}{
		{"vent inexistent", map[string]any{"wind": "vent-fantasma", "kind": "daily", "time": "09:00", "prompt": "x"}},
		{"hora dolenta", map[string]any{"wind": w.ID, "kind": "daily", "time": "9", "prompt": "x"}},
		{"prompt buit", map[string]any{"wind": w.ID, "kind": "daily", "time": "09:00", "prompt": "  "}},
		{"interval zero", map[string]any{"wind": w.ID, "kind": "interval", "interval_h": 0, "prompt": "x"}},
		{"kind desconegut", map[string]any{"wind": w.ID, "kind": "mai", "prompt": "x"}},
	}
	for _, c := range cases {
		if code := rutinaPost(t, s, "/api/winds/rutina", c.body, nil); code != http.StatusBadRequest {
			t.Fatalf("%s: %d, en vull 400", c.name, code)
		}
	}
}
