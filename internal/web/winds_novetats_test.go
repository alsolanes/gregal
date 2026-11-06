package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/jobs"
)

func getNovetats(t *testing.T, s *Server, desDe int64) []windNovetat {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/winds/novetats?des_de="+strconv.FormatInt(desDe, 10), nil)
	rec := httptest.NewRecorder()
	s.handleWindNovetats(rec, req)
	if rec.Code != 200 {
		t.Fatalf("novetats rebutjades: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Novetats []windNovetat `json:"novetats"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out.Novetats == nil {
		t.Fatal("novetats:null, calia []")
	}
	return out.Novetats
}

// TestNovetatsRegistreFiltre: el que parla sol queda apuntat i des_de filtra.
func TestNovetatsRegistreFiltre(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	w := companion.Wind{ID: "xaloc", Name: "Xaloc"}
	recordNovetat(s, w, "remind", "primera")
	recordNovetat(s, w, "heartbeat", "segona")
	recordNovetat(s, w, "remind", "   ")
	got := getNovetats(t, s, 0)
	if len(got) != 2 {
		t.Fatalf("novetats = %d, calia 2 (el buit no compta)", len(got))
	}
	if got[0].Text != "primera" || got[0].Source != "remind" || got[0].Name != "Xaloc" || got[0].Wind != "xaloc" {
		t.Fatalf("primera = %+v", got[0])
	}
	if got[0].Ts <= 0 || got[1].Ts < got[0].Ts {
		t.Fatalf("ts desordenats: %+v", got)
	}
	if rest := getNovetats(t, s, got[1].Ts); len(rest) != 0 {
		t.Fatalf("des_de=últim ha de tornar buit: %+v", rest)
	}
}

// TestNovetatsTallen: el registre no creix sense fre (màxim 100).
func TestNovetatsTallen(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	w := companion.Wind{ID: "xaloc", Name: "Xaloc"}
	for i := 0; i < maxNovetats+20; i++ {
		recordNovetat(s, w, "remind", "text")
	}
	if n := len(loadNovetats(s)); n != maxNovetats {
		t.Fatalf("registre = %d, calia %d", n, maxNovetats)
	}
}

// TestNovetatRecordatoriPortaElJob: la veu d'un recordatori porta l'id del
// job perquè l'app no avisi dos cops si ja l'havia mostrat sense connexió.
// Les altres veus (cor) no en porten.
func TestNovetatRecordatoriPortaElJob(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	at := time.Now().Add(-time.Minute).UTC()
	j, err := s.jobs.Save(jobs.Job{
		ID: "remind-xaloc-7", Name: "Recordatori Xaloc", Kind: "remind",
		Prompt: "Truca al dentista", Enabled: true, Notify: true, NextAt: &at,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.executeWindRemind(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	recordNovetat(s, companion.Wind{ID: "xaloc", Name: "Xaloc"}, "heartbeat", "hola")
	got := getNovetats(t, s, 0)
	if len(got) != 2 || got[0].Job != "remind-xaloc-7" || got[1].Job != "" {
		t.Fatalf("novetats = %+v", got)
	}
}
