package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"gregal/internal/companion"
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
	s := remindTestServer(t)
	w := companion.Wind{ID: "xaloc", Name: "Xaloc"}
	for i := 0; i < maxNovetats+20; i++ {
		recordNovetat(s, w, "remind", "text")
	}
	if n := len(loadNovetats(s)); n != maxNovetats {
		t.Fatalf("registre = %d, calia %d", n, maxNovetats)
	}
}
