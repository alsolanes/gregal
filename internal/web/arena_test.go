package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

// arenaStub fa de model: decideix, escriu bots i dona informes.
func arenaStub(t *testing.T) {
	t.Helper()
	old := arenaModel
	arenaModel = func(_ *Server, _ context.Context, system, user string, _ int) (string, error) {
		switch {
		case strings.Contains(system, "GREGAL ARENA") && strings.Contains(user, "preview the clash"):
			return `{"text":"Dos estils oposats: compte amb el blitz."}`, nil
		case strings.Contains(system, "GREGAL ARENA") && strings.Contains(user, "CASTER"):
			var lines []string
			for _, m := range regexp.MustCompile(`(?m)^(\d+) \|`).FindAllStringSubmatch(user, -1) {
				lines = append(lines, `{"n":`+m[1]+`,"text":"Quina jugada!"}`)
			}
			return `{"lines":[` + strings.Join(lines, ",") + `]}`, nil
		case strings.Contains(system, "GREGAL ARENA"):
			return `{"headline":"La regla del blitz","text":"Ho va decidir una regla.","side":0,"rule":"Run on first down."}`, nil
		case strings.Contains(system, "You write strategy .md files"):
			return "# Bot strategy\n\n- Run on first down.\n- Cover2 on 3rd and long.", nil
		case strings.Contains(system, "prompt-engineering coach"):
			return `{"scores":{"clarity":4,"coverage":2,"adaptivity":2,"specificity":3,"consistency":5},"summary":"Bona base.","strengths":["clar"],"mistakes":[{"moment":"d3","what":"x","md_issue":"y","fix":"z"}],"suggestions":["Afegeix si/llavors."]}`, nil
		case strings.Contains(user, "You are the OFFENSE"):
			if strings.Contains(system, "ALWAYS DEEP") {
				return `{"decision":{"play":"pass_deep","target":"wr1"},"why":"sempre llarg","rule":"ALWAYS DEEP"}`, nil
			}
			return `{"decision":{"play":"run_inside"},"why":"llegeixo","rule":"Run on first down."}`, nil
		default:
			return `{"decision":{"call":"cover2"},"why":"profund","rule":"Cover2 on 3rd and long."}`, nil
		}
	}
	t.Cleanup(func() { arenaModel = old })
}

func waitMatch(t *testing.T, s *Server, id string) *arenaMatch {
	t.Helper()
	for i := 0; i < 600; i++ {
		if m := s.arenaLoadMatch(id); m != nil && (m.Status == "final" || m.Status == "failed" || m.Status == "stopped") {
			waitBackground()
			return s.arenaLoadMatch(id)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("el partit %s no acaba", id)
	return nil
}

func TestArenaPracticeAndCompetition(t *testing.T) {
	s := cornerTestServer(t)
	arenaStub(t)

	// Bots escrits pel model.
	postJSON(t, s.handleArena, `{"action":"seed_bots","games":["nfl"]}`)
	waitBackground()
	if n := len(s.arenaEntries("nfl")); n != len(arenaBotStyles) {
		t.Fatalf("bots: %d", n)
	}
	// El meu .md, amb historial de versions.
	postJSON(t, s.handleArena, `{"action":"save","game":"nfl","display":"Aleix","md":"# ALWAYS DEEP\nThrow deep."}`)
	postJSON(t, s.handleArena, `{"action":"save","game":"nfl","display":"Aleix","md":"# Balanced\nRun on first down."}`)
	if e := s.arenaEntry("nfl", "local"); e == nil || len(e.Versions) != 1 || !strings.Contains(e.MD, "Balanced") {
		t.Fatalf("entrada: %+v", e)
	}
	if rec := postJSON(t, s.handleArena, `{"action":"save","game":"nfl","md":"`+strings.Repeat("x", arenaMaxMD+1)+`"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("un .md massa llarg s'havia de refusar")
	}

	// Partit de prova contra un bot.
	rec := postJSON(t, s.handleArena, `{"action":"practice","game":"nfl","opponent":"bot:rookie"}`)
	var started struct{ ID string }
	json.Unmarshal(rec.Body.Bytes(), &started)
	if started.ID == "" {
		t.Fatalf("practice: %d %s", rec.Code, rec.Body.String())
	}
	m := waitMatch(t, s, started.ID)
	if m.Status != "final" || len(m.Events) < 60 || len(m.Decisions) < 100 {
		t.Fatalf("partit: %s, %d esdeveniments, %d decisions", m.Status, len(m.Events), len(m.Decisions))
	}
	if m.Decisions[0].Why == "" || m.Decisions[0].Rule == "" || m.Feedback[0] == nil || m.Feedback[0].Scores["clarity"] != 4 {
		t.Fatalf("decisions/informe: %+v %+v", m.Decisions[0], m.Feedback[0])
	}
	// La retransmissió: prèvia, comentaris lligats a jugades clau i anàlisi.
	if m.Preview == "" || len(m.Lines) == 0 || m.Analysis == nil || m.Analysis.Rule == "" {
		t.Fatalf("retransmissió: prèvia %q, %d comentaris, anàlisi %+v", m.Preview, len(m.Lines), m.Analysis)
	}
	hl := map[int]bool{}
	for _, e := range m.Events {
		if e.Highlight {
			hl[e.N] = true
		}
	}
	for _, l := range m.Lines {
		if !hl[l.N] {
			t.Fatalf("un comentari ha d'anar lligat a una jugada clau: %+v", l)
		}
	}
	// El visor rep les jugades noves i el meta només al principi.
	req := httptest.NewRequest(http.MethodGet, "/api/arena/match?id="+m.ID+"&after=5", nil)
	out := httptest.NewRecorder()
	s.handleArenaMatch(out, req)
	var tail arenaMatch
	json.Unmarshal(out.Body.Bytes(), &tail)
	if len(tail.Events) != len(m.Events)-5 || tail.Meta != nil {
		t.Fatalf("after=5: %d esdeveniments", len(tail.Events))
	}
	// Les pràctiques no mouen l'Elo.
	if len(s.arenaRatings()["nfl"]) != 0 {
		t.Fatal("una pràctica no ha de comptar per al rànquing")
	}

	// Competició: 4 bots + jo = un grup de 5 i final.
	rec = postJSON(t, s.handleArenaComp, `{"action":"create","title":"Prova","games":["nfl"],"bots":true}`)
	var comp struct{ ID string }
	json.Unmarshal(rec.Body.Bytes(), &comp)
	c := s.arenaComp(comp.ID)
	if c == nil || len(c.Groups["nfl"]) != 1 || len(c.Schedule) != 11 {
		t.Fatalf("competició: %+v", c)
	}
	postJSON(t, s.handleArenaComp, `{"action":"start","id":"`+comp.ID+`"}`)
	for i := 0; i < 3000; i++ {
		arenaMu.Lock()
		c = s.arenaComp(comp.ID)
		arenaMu.Unlock()
		if c != nil && c.Status == "done" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	waitBackground()
	if c.Status != "done" || c.Champion["nfl"] == "" {
		t.Fatalf("la competició havia d'acabar amb campió: %s %+v", c.Status, c.Champion)
	}
	final := c.Schedule[len(c.Schedule)-1]
	if final.Stage != "Final" || !final.Done || final.A == "" || final.B == "" {
		t.Fatalf("final: %+v", final)
	}
	board := s.arenaRatings()["nfl"]
	if len(board) != 5 {
		t.Fatalf("rànquing: %d jugadors", len(board))
	}
	played := 0
	for _, r := range board {
		played += r.Played
	}
	if played != 22 {
		t.Fatalf("partits al rànquing: %d (11 partits × 2)", played)
	}
}

func TestArenaGroupsAndBracket(t *testing.T) {
	elo := map[string]float64{}
	owners := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	for i, o := range owners {
		elo[o] = float64(2000 - i*10)
	}
	groups := arenaGroups(append([]string(nil), owners...), elo)
	if len(groups) != 3 {
		t.Fatalf("9 jugadors → 3 grups: %v", groups)
	}
	sched := arenaBuildSchedule("nfl", groups)
	stages := map[string]int{}
	for _, sl := range sched {
		stages[sl.Stage]++
	}
	if stages["Group A"] != 3 || stages["Quarterfinal 1"] != 1 || stages["Final"] != 1 {
		t.Fatalf("calendari: %v", stages)
	}
	if a, b := eloUpdate(1500, 1500, 0); a != 1516 || b != 1484 {
		t.Fatalf("elo: %v %v", a, b)
	}
}

func TestArenaSpectatorLinkOnlyWatches(t *testing.T) {
	s := cornerTestServer(t)
	s.Token = "secret"
	h := s.Hub().Handler()
	key := s.arenaSpectatorKey(false)
	do := func(method, path, spect string) int {
		req := httptest.NewRequest(method, path, strings.NewReader("{}"))
		if spect != "" {
			req.Header.Set("X-Arena-Spectator", spect)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := do("GET", "/api/arena/stream", key); c != 200 {
		t.Fatalf("l'espectador ha de poder mirar la retransmissió: %d", c)
	}
	if c := do("GET", "/api/arena/stream", "dolenta"); c != http.StatusUnauthorized {
		t.Fatalf("clau dolenta: %d", c)
	}
	for _, p := range []string{"/api/arena", "/api/state", "/api/winds"} {
		if c := do("GET", p, key); c != http.StatusUnauthorized {
			t.Fatalf("l'espectador no pot entrar a %s: %d", p, c)
		}
	}
	if c := do("POST", "/api/arena/match?id=x", key); c != http.StatusUnauthorized {
		t.Fatalf("l'espectador no pot aturar partits: %d", c)
	}
	if k2 := s.arenaSpectatorKey(true); k2 == key || do("GET", "/api/arena/stream", key) != http.StatusUnauthorized {
		t.Fatal("renovar la clau ha d'invalidar l'enllaç antic")
	}
}

// Un reinici del servidor no pot deixar una competició penjada.
func TestArenaResumeAfterRestart(t *testing.T) {
	s := cornerTestServer(t)
	arenaStub(t)
	postJSON(t, s.handleArena, `{"action":"seed_bots","games":["nfl"]}`)
	waitBackground()
	rec := postJSON(t, s.handleArenaComp, `{"action":"create","title":"Reinici","games":["nfl"],"bots":true}`)
	var comp struct{ ID string }
	json.Unmarshal(rec.Body.Bytes(), &comp)
	// Com si el servidor hagués caigut a mig partit: competició en marxa i un
	// partit a mitges sense ningú que el jugui.
	arenaMu.Lock()
	c := s.arenaComp(comp.ID)
	c.Status = "running"
	orphan := &arenaMatch{ID: "orfe", Game: "nfl", Kind: "official", Comp: c.ID, Status: "live", Winner: -1}
	c.Schedule[0].Match = orphan.ID
	arenaWrite(s.compPath(c.ID), c)
	arenaWrite(s.matchPath(orphan.ID), orphan)
	arenaMu.Unlock()

	s.arenaResume(context.Background())
	for i := 0; i < 3000; i++ {
		arenaMu.Lock()
		c = s.arenaComp(comp.ID)
		arenaMu.Unlock()
		if c != nil && c.Status == "done" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	waitBackground()
	if c.Status != "done" || c.Champion["nfl"] == "" {
		t.Fatalf("la competició havia de continuar fins al final: %s", c.Status)
	}
	if m := s.arenaLoadMatch("orfe"); m == nil || m.Status != "stopped" {
		t.Fatalf("el partit a mitges s'ha de donar per aturat: %+v", m)
	}
	if c.Schedule[0].Match == "orfe" || !c.Schedule[0].Done {
		t.Fatalf("el partit que tocava es torna a jugar: %+v", c.Schedule[0])
	}
}
