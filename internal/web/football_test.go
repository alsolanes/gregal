package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
)

func TestFootballMatchPlaysToTheEnd(t *testing.T) {
	s := cornerTestServer(t)
	var calls int32
	old := footballModel
	nRe := regexp.MustCompile(`(?m)^(\d+) \|`)
	footballModel = func(_ *Server, _ context.Context, system, user string, _ int) (string, error) {
		atomic.AddInt32(&calls, 1)
		switch {
		case strings.Contains(system, "HEAD COACH"):
			return `{"pass_rate":0.6,"deep_shots":0.2,"aggression":0.5,"blitz":0.3,"tempo":0.2,"focus":"","note":"Anem a per ells!"}`, nil
		case strings.Contains(system, "PLAY-BY-PLAY"):
			var lines []string
			for _, m := range nRe.FindAllStringSubmatch(user, -1) {
				lines = append(lines, `{"n":`+m[1]+`,"text":"Quina jugada!"}`)
			}
			return `{"lines":[` + strings.Join(lines, ",") + `]}`, nil
		default:
			return `{"recap":"Un partidàs.","mvp":""}`, nil
		}
	}
	t.Cleanup(func() { footballModel = old })

	rec := postJSON(t, s.handleFootball, `{"action":"play"}`)
	var started struct{ ID string }
	json.Unmarshal(rec.Body.Bytes(), &started)
	if rec.Code != 200 || started.ID == "" {
		t.Fatalf("play: %d %s", rec.Code, rec.Body.String())
	}
	waitBackground()

	req := httptest.NewRequest(http.MethodGet, "/api/football/match?id="+started.ID, nil)
	out := httptest.NewRecorder()
	s.handleFootballMatch(out, req)
	var m fbMatch
	json.Unmarshal(out.Body.Bytes(), &m)
	if m.Status != "final" || len(m.Plays) < 80 || m.Summary != "Un partidàs." {
		t.Fatalf("partit: %s, %d jugades, %q", m.Status, len(m.Plays), m.Summary)
	}
	if len(m.Teams[0].Players) != 23 || len(m.Teams[1].Players) != 23 {
		t.Fatalf("equips de 23")
	}
	commented := 0
	for _, p := range m.Plays {
		if p.Comment != "" {
			commented++
		}
	}
	if commented == 0 || len(m.Plans) < 8 {
		t.Fatalf("comentaris %d, plans %d", commented, len(m.Plans))
	}
	// El mínim garantit són les 8 decisions dels entrenadors (2 equips × 4
	// quarts). El comentarista va per quart i, si un quart no té cap jugada
	// clau, fbCommentary torna sense cridar el model — per això 12 també és
	// un partit bo i el terra no pot ser 13. Que el comentari funciona ja ho
	// comprova `commented` aquí a sobre, i el resum, `m.Summary`.
	if c := atomic.LoadInt32(&calls); c < 8 || c > 30 {
		t.Fatalf("crides al model: %d", c)
	}
	// Les jugades noves, a partir d'una.
	req = httptest.NewRequest(http.MethodGet, "/api/football/match?id="+started.ID+"&after=10", nil)
	out = httptest.NewRecorder()
	s.handleFootballMatch(out, req)
	var tail fbMatch
	json.Unmarshal(out.Body.Bytes(), &tail)
	if len(tail.Plays) != len(m.Plays)-10 || tail.Plays[0].N != 11 || tail.Teams[0].Players != nil {
		t.Fatalf("after=10: %d jugades", len(tail.Plays))
	}
	// La lliga apunta el resultat; els jugadors d'equip no són vents.
	lg := s.fbLoadLeague()
	if lg.Games != 1 || lg.Teams[0].Wins+lg.Teams[0].Losses+lg.Teams[0].Ties != 1 {
		t.Fatalf("lliga: %+v", lg)
	}
	if ws, _ := s.winds.List(); len(ws) != 2 {
		t.Fatalf("els jugadors d'equip no han d'aparèixer com a vents: %d", len(ws))
	}
	// Tres partits al dia.
	postJSON(t, s.handleFootball, `{"action":"play"}`)
	waitBackground()
	postJSON(t, s.handleFootball, `{"action":"play"}`)
	waitBackground()
	if rec := postJSON(t, s.handleFootball, `{"action":"play"}`); rec.Code != http.StatusConflict {
		t.Fatalf("el quart partit del dia s'havia de refusar: %d", rec.Code)
	}
}

func TestFootballStop(t *testing.T) {
	s := cornerTestServer(t)
	old := footballModel
	gate := make(chan struct{})
	footballModel = func(_ *Server, ctx context.Context, _, _ string, _ int) (string, error) {
		select {
		case <-gate:
		case <-ctx.Done():
		}
		return "", ctx.Err()
	}
	t.Cleanup(func() { footballModel = old })
	rec := postJSON(t, s.handleFootball, `{"action":"play"}`)
	var started struct{ ID string }
	json.Unmarshal(rec.Body.Bytes(), &started)
	if rec := postJSON(t, s.handleFootball, `{"action":"play"}`); rec.Code != http.StatusConflict {
		t.Fatalf("un segon partit alhora s'havia de refusar: %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/football/match?id="+started.ID, strings.NewReader(`{"action":"stop"}`))
	s.handleFootballMatch(httptest.NewRecorder(), req)
	close(gate)
	waitBackground()
	if m := s.fbLoad(started.ID); m == nil || m.Status != "stopped" {
		t.Fatalf("aturat: %+v", m)
	}
}
