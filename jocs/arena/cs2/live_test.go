package cs2

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
)

// fakeRound: el que tornaria el navegador si un bàndol guanyés per eliminació
// (o plantant abans).
func fakeRound(s *LiveSetup, winT bool, plant bool, rr *rand.Rand) LiveResult {
	res := LiveResult{Round: s.Round, Tracks: map[string][][3]float64{}, Dmg: map[string]int{}}
	var ts, cts []LivePlayer
	for _, p := range s.Players {
		res.Tracks[p.ID] = [][3]float64{{0, p.X, p.Y}, {2, p.X - 3, p.Y}, {4, p.X - 6, p.Y + 1}, {9, p.X - 6, p.Y + 1}}
		if p.Side == "T" {
			ts = append(ts, p)
		} else {
			cts = append(cts, p)
		}
	}
	t := 20.0
	if plant {
		res.Plant = &LivePlant{T: 30, Start: 26.8, ID: s.Bomb, X: 41, Y: 66}
		res.Explode = 70
		t = 35
	}
	win, lose := ts, cts
	if !winT {
		win, lose = cts, ts
	}
	for i, v := range lose {
		k := win[rr.Intn(len(win))]
		w := k.Primary
		if w == "" {
			w = k.Pistol
		}
		res.Kills = append(res.Kills, LiveKill{T: t + float64(i)*3, Killer: k.ID, Victim: v.ID, Weapon: w, HS: i%2 == 0})
		res.Dmg[k.ID] += 100
	}
	return res
}

func TestLiveShortMatchFollowsTheRules(t *testing.T) {
	l := NewLive(7, "en", [2]string{"Humans", "Bots"}, 6, 0, 2)
	if l.HumanID() != "a2" {
		t.Fatalf("human id %s", l.HumanID())
	}
	rr := rand.New(rand.NewSource(3))
	rounds := 0
	var all []string
	for !l.Done() {
		p, done := l.Prompts()
		if done {
			break
		}
		if !strings.Contains(p[0].Schema, `"roles"`) || !strings.Contains(p[0].Schema, "human") {
			t.Fatalf("live schema without roles/human: %s", p[0].Schema)
		}
		ans := [2]json.RawMessage{json.RawMessage(`{"buy":"full","strategy":"execute_a","utility":"full","setup":"stack_b","roles":{"` + l.m.players[0][0].Handle + `":"lurker"}}`), nil}
		s, err := l.Start(ans)
		if err != nil {
			t.Fatal(err)
		}
		if again, _ := l.Start(ans); again != s {
			t.Fatal("a second Start must return the same round")
		}
		if len(s.Players) != 10 || s.Human != "a2" || s.Bomb == "a2" || s.Bomb == "" {
			t.Fatalf("setup: %d players, human %s, bomb %s", len(s.Players), s.Human, s.Bomb)
		}
		for _, p := range s.Players {
			if p.Post == "" || len(p.Route) == 0 || p.Bot == "" {
				t.Fatalf("player without orders: %+v", p)
			}
			if p.ID == "a0" && p.Bot != "lurker" {
				t.Fatalf("the model's role for a0 was ignored: %s", p.Bot)
			}
		}
		res := fakeRound(s, rounds%3 != 0, rounds%2 == 0, rr)
		// La persona es compra un rifle amb armilla (si no hi arriba, res).
		res.Human = &LiveLoadout{Primary: map[bool]string{true: "ak47", false: "m4a1s"}[s.TTeam == 0], Armor: 2}
		before := l.m.score
		ev, err := l.Finish(res)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := l.Finish(res); err == nil {
			t.Fatal("a round can only finish once")
		}
		if l.m.score[0]+l.m.score[1] != before[0]+before[1]+1 {
			t.Fatalf("score %v -> %v", before, l.m.score)
		}
		kinds := map[string]int{}
		for _, e := range ev {
			kinds[e.Kind]++
			all = append(all, e.Kind)
		}
		if kinds["round_start"] != 1 || kinds["buy"] != 2 || kinds["round_end"] != 1 || kinds["move"] == 0 || kinds["kill"] == 0 {
			t.Fatalf("round %d events %v", s.Round, kinds)
		}
		for _, p := range l.m.teamPlayers(0) {
			if p.Money < 0 || p.Money > maxMoney {
				t.Fatalf("money out of range: %d", p.Money)
			}
		}
		rounds++
		if rounds > 30 {
			t.Fatal("never ends")
		}
	}
	sc := l.m.score
	if max(sc[0], sc[1]) < 7 {
		t.Fatalf("short format: first to 7, got %v after %d rounds", sc, rounds)
	}
	joined := strings.Join(all, ",")
	if !strings.Contains(joined, "half") || !strings.Contains(joined, "match_end") {
		t.Fatalf("missing half/match_end: %s", joined)
	}
	if mt := l.m.Meta().(meta); mt.Half != 6 || mt.Halves[1].From != 7 {
		t.Fatalf("meta half %d halves %v", mt.Half, mt.Halves)
	}
}

func TestLiveBombAndDefuse(t *testing.T) {
	l := NewLive(11, "en", [2]string{"A", "B"}, 6, 1, 0)
	l.Prompts()
	s, _ := l.Start([2]json.RawMessage{nil, nil})
	// Planten a B, maten un CT i un altre desactiva: guanya el CT per desactivació.
	var cts []LivePlayer
	var tt LivePlayer
	for _, p := range s.Players {
		if p.Side == "CT" {
			cts = append(cts, p)
		} else if p.ID != s.Bomb {
			tt = p
		}
	}
	res := LiveResult{Round: s.Round, Plant: &LivePlant{T: 40, Start: 36.8, ID: s.Bomb, X: 24, Y: 15},
		Defuse: &LiveDefuse{T: 60, Start: 55, ID: cts[0].ID}, Kills: []LiveKill{{T: 45, Killer: tt.ID, Victim: cts[1].ID, Weapon: "glock"}}}
	ev, err := l.Finish(res)
	if err != nil {
		t.Fatal(err)
	}
	var end map[string]any
	planted, defused := false, false
	for _, e := range ev {
		switch e.Kind {
		case "round_end":
			json.Unmarshal(e.Data, &end)
		case "plant":
			planted = true
		case "defuse":
			defused = true
		}
	}
	if !planted || !defused || end["reason"] != "defuse" || end["side"] != "CT" || end["site"] != "B" {
		t.Fatalf("planted %v defused %v end %v", planted, defused, end)
	}
	// Ronda 2: planten a A i ningú no desactiva: explota.
	l.Prompts()
	s, _ = l.Start([2]json.RawMessage{nil, nil})
	ev, _ = l.Finish(LiveResult{Plant: &LivePlant{T: 50, Start: 46.8, ID: s.Bomb, X: 41, Y: 66}})
	for _, e := range ev {
		if e.Kind == "round_end" {
			json.Unmarshal(e.Data, &end)
		}
	}
	if end["reason"] != "bomb" || end["side"] != "T" || end["site"] != "A" {
		t.Fatalf("explosion: %v", end)
	}
	// Ronda 3: no passa res: guanya el CT pel temps.
	l.Prompts()
	l.Start([2]json.RawMessage{nil, nil})
	ev, _ = l.Finish(LiveResult{})
	for _, e := range ev {
		if e.Kind == "round_end" {
			json.Unmarshal(e.Data, &end)
		}
	}
	if end["reason"] != "time" || end["side"] != "CT" {
		t.Fatalf("time: %v", end)
	}
}

func TestLiveHumanBuyIsPaid(t *testing.T) {
	l := NewLive(5, "en", [2]string{"A", "B"}, 12, 0, 1)
	l.Prompts()
	s, _ := l.Start([2]json.RawMessage{nil, nil})
	side := sideCT
	if s.TTeam == 0 {
		side = sideT
	}
	h := l.human
	if h.Money != 800 {
		t.Fatalf("pistol round money %d", h.Money)
	}
	// Massa car: no canvia res.
	l.applyHuman(&LiveLoadout{Primary: "awp", Armor: 2}, side)
	if h.Money != 800 || h.Primary != "" {
		t.Fatalf("unaffordable buy went through: %d %s", h.Money, h.Primary)
	}
	// P250 + dos flaixos (el tercer no hi cap).
	l.applyHuman(&LiveLoadout{Pistol: "p250", Nades: []string{"flash", "flash", "flash"}}, side)
	if h.Money != 100 || h.Pistol != "p250" || len(h.Nades) != 2 {
		t.Fatalf("buy not applied right: %+v", h)
	}
}

func TestCompressTrack(t *testing.T) {
	var tr [][3]float64
	// Camina recte 4 s, s'atura 2 s, gira i camina 2 s més.
	for i := 0; i <= 16; i++ {
		tr = append(tr, [3]float64{float64(i) * 0.25, float64(i), 0})
	}
	for i := 1; i <= 8; i++ {
		tr = append(tr, [3]float64{4 + float64(i)*0.25, 16, 0})
	}
	for i := 1; i <= 8; i++ {
		tr = append(tr, [3]float64{6 + float64(i)*0.25, 16, float64(i)})
	}
	ch := compressTrack(tr)
	if len(ch) != 2 {
		t.Fatalf("chunks %d: %+v", len(ch), ch)
	}
	if len(ch[0].pts) != 2 || ch[0].t0 != 0 || ch[0].t1 != 4 {
		t.Fatalf("straight line kept as %+v", ch[0])
	}
	if ch[1].t0 != 6 || ch[1].pts[len(ch[1].pts)-1] != [2]float64{16, 8} {
		t.Fatalf("second chunk %+v", ch[1])
	}
}

func TestTargetForH(t *testing.T) {
	for _, c := range [][3]int{{1, 12, 13}, {24, 12, 13}, {25, 12, 16}, {31, 12, 19}, {12, 6, 7}, {13, 6, 10}, {19, 6, 13}} {
		if got := targetForH(c[0], c[1]); got != c[2] {
			t.Errorf("targetForH(%d,%d) = %d, want %d", c[0], c[1], got, c[2])
		}
	}
}

func TestLiveFollowRole(t *testing.T) {
	for _, c := range [][2]string{{"Follow me all the time", "follow"}, {"stick with me on B", "follow"}, {"cover me when I peek", "follow"},
		{"Segueix-me sempre", "follow"}, {"sígueme", "follow"}, {"hold mid angles with AWP", ""}, {"follow the entry", ""}} {
		if got := NoteRole(c[0]); got != c[1] {
			t.Errorf("NoteRole(%q) = %q, want %q", c[0], got, c[1])
		}
	}
	for _, a := range []string{"follow", "Follow me", "escort", "bodyguard", "follow-me"} {
		if pick(a, botRoles, roleAlias) != "follow" {
			t.Errorf("alias %q is not follow", a)
		}
	}
	l := NewLive(3, "en", [2]string{"A", "B"}, 6, 0, 2)
	// a0 té rol explícit, a1 una nota «segueix-me» (guanya la nota), a3 res.
	l.SetRoles(0, []string{"entry", "awper", "", "rifler", ""}, []string{"", "segueix-me", "", "", ""})
	p, _ := l.Prompts()
	if !strings.Contains(p[0].Schema, "follow = stays with your human player") || strings.Contains(p[1].Schema, "|follow") {
		t.Fatalf("follow is offered only to the human's team: %q / %q", p[0].Schema, p[1].Schema)
	}
	opp := l.m.players[1][0].Handle
	s, _ := l.Start([2]json.RawMessage{json.RawMessage(`{"roles":{"` + l.m.players[0][3].Handle + `":"follow_me"}}`), json.RawMessage(`{"roles":{"` + opp + `":"follow"}}`)})
	want := map[string]string{"a0": "entry", "a1": "follow", "a3": "follow"}
	for _, pl := range s.Players {
		if w := want[pl.ID]; w != "" && pl.Bot != w {
			t.Errorf("%s: role %s, want %s", pl.ID, pl.Bot, w)
		}
		if pl.Team == 1 && pl.Bot == "follow" {
			t.Errorf("the bot team has no human to follow: %s", pl.ID)
		}
	}
}
