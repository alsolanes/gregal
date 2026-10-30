package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"gregal/internal/arena/cs2"
)

// El CS2 en directe: començar, demanar el pla de cada ronda (el model), tornar
// el resultat i acabar; el partit queda desat com «live» i no toca l'Elo.
func TestArenaLiveMatch(t *testing.T) {
	s := cornerTestServer(t)
	old := arenaModel
	calls := 0
	arenaModel = func(_ *Server, _ context.Context, system, user string, _ int) (string, error) {
		calls++
		if strings.Contains(system, "Give each bot a role") {
			return `{"players":[{"slot":0,"role":"entry","note":"Go first"},{"slot":1,"role":"awper","note":"Hold mid"},{"slot":3,"role":"support","note":"Flash me"},{"slot":4,"role":"lurker","note":"Flank"}]}`, nil
		}
		if !strings.Contains(user, "LIVE MODE") {
			t.Errorf("the live prompt must say it is live: %s", user)
		}
		if strings.Contains(user, "You are T") {
			return `{"decision":{"buy":"full","strategy":"execute_a","utility":"full","pace":"default","awp":true},"why":"hit A","rule":""}`, nil
		}
		return `{"decision":{"buy":"full","setup":"stack_b","utility":"normal","awp":true},"why":"stack","rule":""}`, nil
	}
	t.Cleanup(func() { arenaModel = old })

	rec := postJSON(t, s.handleArenaLive, `{"action":"roles","slot":2}`)
	var roles struct{ Roles, Notes []string }
	json.Unmarshal(rec.Body.Bytes(), &roles)
	if rec.Code != 200 || roles.Roles[0] != "entry" || roles.Roles[2] != "" || roles.Notes[3] != "Flash me" {
		t.Fatalf("roles: %d %s", rec.Code, rec.Body.String())
	}

	rec = postJSON(t, s.handleArenaLive, `{"action":"start","format":"short","slot":2,"roles":["entry","awper","","support","lurker"],"notes":["Go first","","","",""]}`)
	var st struct {
		ID    string
		Human string
		Half  int
	}
	json.Unmarshal(rec.Body.Bytes(), &st)
	if rec.Code != 200 || st.ID == "" || st.Human != "a2" || st.Half != 6 {
		t.Fatalf("start: %d %s", rec.Code, rec.Body.String())
	}
	for round := 1; round <= 40; round++ {
		rec = postJSON(t, s.handleArenaLive, fmt.Sprintf(`{"action":"plan","id":%q}`, st.ID))
		var plan struct {
			Setup     cs2.LiveSetup
			Decisions []map[string]any
		}
		json.Unmarshal(rec.Body.Bytes(), &plan)
		if rec.Code != 200 || plan.Setup.Round != round || len(plan.Setup.Players) != 10 {
			t.Fatalf("plan %d: %d %s", round, rec.Code, rec.Body.String())
		}
		if len(plan.Decisions) != 1 || plan.Decisions[0]["side"].(float64) != 0 {
			t.Fatalf("only my team's call is shown: %+v", plan.Decisions)
		}
		// Un segon clic no torna a cridar el model.
		before := calls
		postJSON(t, s.handleArenaLive, fmt.Sprintf(`{"action":"plan","id":%q}`, st.ID))
		if calls != before {
			t.Fatal("a second plan request must not call the model again")
		}
		// Guanya sempre el meu equip, per eliminació.
		var kills []cs2.LiveKill
		var killer string
		for _, p := range plan.Setup.Players {
			if p.Team == 0 {
				killer = p.ID
			}
		}
		for _, p := range plan.Setup.Players {
			if p.Team == 1 {
				kills = append(kills, cs2.LiveKill{T: 20 + float64(len(kills)), Killer: killer, Victim: p.ID, Weapon: "ak47"})
			}
		}
		res, _ := json.Marshal(cs2.LiveResult{Round: round, Kills: kills})
		rec = postJSON(t, s.handleArenaLive, fmt.Sprintf(`{"action":"result","id":%q,"result":%s}`, st.ID, res))
		var out struct {
			Score  [2]int
			Done   bool
			Winner int
		}
		json.Unmarshal(rec.Body.Bytes(), &out)
		if rec.Code != 200 || out.Score[0] != round {
			t.Fatalf("result %d: %d %s", round, rec.Code, rec.Body.String())
		}
		if out.Done {
			if out.Winner != 0 || round != 7 {
				t.Fatalf("short format ends 7-0: round %d winner %d", round, out.Winner)
			}
			break
		}
	}
	m := s.arenaLoadMatch(st.ID)
	if m == nil || m.Status != "final" || m.Kind != "live" || m.Winner != 0 || len(m.Decisions) != 14 || len(m.Events) < 50 {
		t.Fatalf("saved match: %+v", m)
	}
	if r := s.arenaRatings()["cs2"]; len(r) != 0 {
		t.Fatalf("a live match must not be ranked: %+v", r)
	}
	if rec := postJSON(t, s.handleArenaLive, fmt.Sprintf(`{"action":"plan","id":%q}`, st.ID)); rec.Code != http.StatusNotFound {
		t.Fatalf("a finished live match takes no more plans: %d", rec.Code)
	}
}
