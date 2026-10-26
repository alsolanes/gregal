package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/companion"
)

// TestColllesSenseNomsRepetits: les plantilles no repeteixen cap nom i, si
// un vent creat a mà ja en fa servir un, el de la colla en pren un altre.
func TestColllesSenseNomsRepetits(t *testing.T) {
	vistos := map[string]string{}
	for _, sq := range windSquadTemplates {
		for _, m := range sq.Members {
			if prev, ok := vistos[strings.ToLower(m.Name)]; ok {
				t.Fatalf("%s surt a %s i a %s", m.Name, prev, sq.ID)
			}
			vistos[strings.ToLower(m.Name)] = sq.ID
		}
	}
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	s.winds.Save(companion.Wind{Name: "Cerç", Role: "a mà"})
	rec := httptest.NewRecorder()
	s.handleWindSquads(rec, httptest.NewRequest(http.MethodPost, "/api/winds/squads", bytes.NewBufferString(`{"id":"data","lang":"ca"}`)))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	list, _ := s.winds.List()
	noms := map[string]int{}
	for _, w := range list {
		noms[strings.ToLower(w.Name)]++
	}
	for n, c := range noms {
		if c > 1 {
			t.Fatalf("%s repetit %d vegades", n, c)
		}
	}
}
