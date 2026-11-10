package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/web"
)

// La web de Gregal Jocs: el nom de la competició el posa l'organitzador
// (admin) i es desa a la configuració; la portada rep el nom i les xifres.
func TestArenaEventIsSetByTheOrganizer(t *testing.T) {
	s := cornerTestServer(t)
	arenaStub(t)

	get := func() map[string]json.RawMessage {
		rec := httptest.NewRecorder()
		s.handleArena(rec, httptest.NewRequest(http.MethodGet, "/api/arena", nil))
		var out map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("GET /api/arena: %v", err)
		}
		return out
	}
	var ev arenaEventCfg
	json.Unmarshal(get()["event"], &ev)
	if ev.Name != arenaEventDefaultName || ev.Tagline == "" {
		t.Fatalf("sense configurar, el nom per defecte: %+v", ev)
	}
	if _, ok := get()["totals"]; !ok {
		t.Fatal("la portada necessita les xifres (totals)")
	}

	long := strings.Repeat("x", 200)
	rec := postJSON(t, s.handleArena, `{"action":"event","event":{"name":"  Spring Cup  ","tagline":"`+long+`","date":1800000000,"place":"Hall"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin: %d %s", rec.Code, rec.Body)
	}
	json.Unmarshal(get()["event"], &ev)
	if ev.Name != "Spring Cup" || ev.Date != 1800000000 || ev.Place != "Hall" || len([]rune(ev.Tagline)) != 140 {
		t.Fatalf("desat: %+v", ev)
	}

	// Un jugador que no és admin no el pot canviar.
	req := httptest.NewRequest(http.MethodPost, "/api/arena", strings.NewReader(`{"action":"event","event":{"name":"Mine"}}`))
	req = web.WithUser(req, &web.User{Name: "marta"})
	out := httptest.NewRecorder()
	s.handleArena(out, req)
	if out.Code != http.StatusForbidden {
		t.Fatalf("no admin: %d", out.Code)
	}
	if s.arenaEvent().Name != "Spring Cup" {
		t.Fatal("el nom ha canviat sense ser admin")
	}
}

func TestArenaTotalsCountPeopleNotBots(t *testing.T) {
	sums := []arenaSum{
		{M: arenaMatch{Kind: "official", Owners: [2]string{"aleix", "bot:rookie"}, Stats: &arenaStats{Decisions: 10}}},
		{M: arenaMatch{Kind: "practice", Owners: [2]string{"marta", "aleix"}, Stats: &arenaStats{Decisions: 5}}},
		{M: arenaMatch{Kind: "practice", Owners: [2]string{"bot:aggro", "bot:turtle"}}},
	}
	got := arenaTotals(sums)
	if got["matches"] != 3 || got["official"] != 1 || got["players"] != 2 || got["decisions"] != 15 {
		t.Fatalf("xifres: %v", got)
	}
}
