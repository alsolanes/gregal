package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/jocs/arena"
)

// seasonMatch és un partit acabat de mentida, amb els moments ja fets.
func seasonMatch(id, kind string, a, b string, winner int, at int64, moments ...arenaHighlight) (arenaSum, *arenaDeep) {
	m := arenaMatch{ID: id, Game: "nfl", Kind: kind, Owners: [2]string{a, b}, Names: [2]string{strings.ToUpper(a), strings.ToUpper(b)},
		Status: "final", Started: at, Finished: at + 60, Winner: winner, Score: [2]int{21, 14}}
	if winner == 1 {
		m.Score = [2]int{14, 21}
	}
	d := &arenaDeep{Followed: [2]float64{.9, .5}, Decisions: [2]int{10, 10}}
	for _, h := range moments {
		h.Match, h.Game, h.Kind, h.Owner = id, "nfl", kind, m.Owners[h.Side]
		d.Moments = append(d.Moments, h)
	}
	return arenaSum{M: m, Coach: [2]int{15, 10}}, d
}

func TestArenaSeasonStandingsAwardsAndPrivacy(t *testing.T) {
	var sums []arenaSum
	deeps := map[string]*arenaDeep{}
	add := func(s arenaSum, d *arenaDeep) { sums = append(sums, s); deeps[s.M.ID] = d }
	// Anna guanya tres pràctiques seguides als bots (moment privat a la segona)
	// i dos oficials; Bernat perd els oficials però guanya al rookie.
	add(seasonMatch("p1", "practice", "anna", "bot:rookie", 0, 100))
	add(seasonMatch("p2", "practice", "anna", "bot:aggro", 0, 200, arenaHighlight{Side: 0, Rule: "secret rule", Impact: 2}))
	add(seasonMatch("p3", "practice", "anna", "bot:analyst", 0, 300))
	add(seasonMatch("p4", "practice", "bernat", "bot:rookie", 0, 150))
	add(seasonMatch("o1", "official", "anna", "bernat", 0, 400, arenaHighlight{Side: 0, Rule: "Blitz on 3rd and long", Impact: 1.2}))
	add(seasonMatch("o2", "official", "bernat", "anna", 1, 500, arenaHighlight{Side: 0, Rule: "Run late", Impact: .4}))
	deeps["o1"].Scores = [2]map[string]int{{"clarity": 5, "specificity": 4, "coverage": 4, "adaptivity": 3, "consistency": 5}, nil}
	ratings := map[string]map[string]*arenaRating{"nfl": {"anna": {Elo: 1531, Played: 2, Won: 2}, "bernat": {Elo: 1469, Played: 2, Lost: 2}}}

	v := arenaSeason(sums, deeps, ratings, map[string]string{"anna": "Anna (BCN)", "carla": "Carla"}, "bernat")
	if len(v.Players) != 3 || v.Players[0].Owner != "anna" || v.Players[0].Rank != 1 || v.Players[2].Owner != "carla" {
		t.Fatalf("classificació: %+v", v.Players)
	}
	anna := v.Players[0]
	if anna.Display != "Anna (BCN)" || anna.Played != 5 || anna.Won != 5 || anna.Official != [3]int{2, 0, 0} || anna.Streak != 5 || anna.BotsBeaten != 3 {
		t.Fatalf("anna: %+v", anna)
	}
	if g := anna.Games["nfl"]; g == nil || g.Elo != 1531 || g.Rank != 1 || len(g.History) != 2 || g.History[1][1] <= 1500 {
		t.Fatalf("anna nfl: %+v", anna.Games["nfl"])
	}
	if anna.Principles["clarity"] != 5 || anna.Reviewed != 1 || anna.Prompt != 56 || strings.Join(anna.Form, "") != "WWWWW" {
		t.Fatalf("principis/forma: %+v", anna)
	}
	// Bernat no veu el moment de la pràctica d'Anna (cita el seu .md), sí els oficials.
	for _, h := range v.Highlights {
		if h.Rule == "secret rule" {
			t.Fatalf("un moment d'una pràctica aliena no s'ha de veure: %+v", h)
		}
	}
	if len(v.Highlights) != 2 || v.Highlights[0].Rule != "Blitz on 3rd and long" || v.Highlights[0].Display != "Anna (BCN)" {
		t.Fatalf("moments: %+v", v.Highlights)
	}
	// Anna sí que veu el seu.
	if own := arenaSeason(sums, deeps, ratings, nil, "anna"); len(own.Highlights) != 3 || own.Highlights[0].Rule != "secret rule" {
		t.Fatalf("els meus moments: %+v", own.Highlights)
	}
	ids := map[string]string{}
	for _, a := range v.Awards {
		ids[a.ID] = a.Owner
	}
	if ids["leader"] != "anna" || ids["on_fire"] != "anna" || ids["by_the_book"] != "anna" {
		t.Fatalf("premis: %+v", v.Awards)
	}
	if _, ok := ids["architect"]; ok {
		t.Fatalf("sense prou notes no hi ha arquitecte: %+v", v.Awards)
	}
}

func TestArenaEloReplayMatchesRecord(t *testing.T) {
	ms := []*arenaMatch{
		{ID: "a", Game: "nfl", Kind: "official", Owners: [2]string{"x", "y"}, Winner: 0, Started: 1},
		{ID: "b", Game: "nfl", Kind: "practice", Owners: [2]string{"x", "bot:rookie"}, Winner: 0, Started: 2},
		{ID: "c", Game: "nfl", Kind: "official", Owners: [2]string{"y", "x"}, Winner: 0, Started: 3},
	}
	hist, before := arenaEloReplay(ms)
	x1, y1 := eloUpdate(1500, 1500, 0)
	y2, x2 := eloUpdate(y1, x1, 0)
	if len(hist["nfl"]["x"]) != 2 || hist["nfl"]["x"][1][1] != float64(int(x2+.5)) || hist["nfl"]["y"][1][1] != float64(int(y2+.5)) {
		t.Fatalf("corba: %+v", hist)
	}
	if before["c"] != [2]float64{y1, x1} || before["b"] != [2]float64{} {
		t.Fatalf("Elo d'abans: %+v", before)
	}
}

func TestArenaPlayerInsight(t *testing.T) {
	s := cornerTestServer(t)
	arenaStub(t)
	calls := 0
	old := arenaModel
	arenaModel = func(s *Server, ctx context.Context, system, user string, n int) (string, error) {
		if strings.Contains(system, "scouting reports") {
			calls++
			if !strings.Contains(user, "Aleix") || !strings.Contains(user, "still secret") {
				t.Errorf("el que llegeix l'analista: %s", user)
			}
			return `{"archetype":"The Grinder","emoji":"⚙️","tagline":"Slow and steady.","summary":"Runs a lot.","signature":"Run on first down.","strength":"Patient.","weakness":"Predictable.","tip":"Add if/then rules.","principle":"Specificity","rival":"Stack the box."}`, nil
		}
		return old(s, ctx, system, user, n)
	}
	postJSON(t, s.handleArena, `{"action":"save","game":"nfl","display":"Aleix","md":"# Balanced\nRun on first down."}`)
	rec := postJSON(t, s.handleArena, `{"action":"practice","game":"nfl","opponent":"bot:rookie"}`)
	var started struct{ ID string }
	json.Unmarshal(rec.Body.Bytes(), &started)
	waitMatch(t, s, started.ID)

	get := func() map[string]json.RawMessage {
		rec := httptest.NewRecorder()
		s.handleArenaPlayer(rec, httptest.NewRequest(http.MethodGet, "/api/arena/player?owner=local", nil))
		if rec.Code != 200 {
			t.Fatalf("perfil: %d %s", rec.Code, rec.Body.String())
		}
		var out map[string]json.RawMessage
		json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	p := get()
	if string(p["insight_stale"]) != "true" || string(p["insight"]) != "null" || !strings.Contains(string(p["matches"]), started.ID) {
		t.Fatalf("abans de l'informe: %s / %s / %s", p["insight_stale"], p["insight"], p["matches"])
	}
	for i := 0; i < 2; i++ {
		if rec := postJSON(t, s.handleArenaPlayer, `{"action":"insight","owner":"local"}`); rec.Code != 200 || !strings.Contains(rec.Body.String(), "The Grinder") {
			t.Fatalf("informe: %d %s", rec.Code, rec.Body.String())
		}
	}
	if calls != 1 {
		t.Fatalf("amb les mateixes dades l'informe no es torna a escriure: %d crides", calls)
	}
	if p = get(); string(p["insight_stale"]) != "false" || !strings.Contains(string(p["insight"]), `"principle":"specificity"`) {
		t.Fatalf("després: %s %s", p["insight_stale"], p["insight"])
	}
	if rec := postJSON(t, s.handleArenaPlayer, `{"action":"insight","owner":"bot:rookie"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("els bots no tenen perfil: %d", rec.Code)
	}
	// La temporada: el jugador, amb el bot guanyat o no, i sense bots a la llista.
	rec = httptest.NewRecorder()
	s.handleArenaSeason(rec, httptest.NewRequest(http.MethodGet, "/api/arena/season", nil))
	var v arenaSeasonView
	json.Unmarshal(rec.Body.Bytes(), &v)
	if len(v.Players) != 1 || v.Players[0].Owner != "local" || v.Players[0].Played != 1 || v.Players[0].Reviewed != 1 {
		t.Fatalf("temporada: %s", rec.Body.String())
	}
	_ = arena.Games
}
