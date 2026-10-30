package cs2

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"gregal/internal/arena"
)

func play(seed int64, d [2]arena.Decide) (*Match, []arena.Event) {
	m := newMatch(seed, "en", [2]string{"Alpha", "Bravo"})
	ev := arena.Play(m, d)
	return m, ev
}

// ---- proves ----

func TestRegistered(t *testing.T) {
	g, ok := arena.Get("cs2")
	if !ok || g.Title("ca") != "Counter-Strike 2" {
		t.Fatal("cs2 not registered")
	}
	if len(g.Rules()) > 6000 || len(g.Rules()) < 500 {
		t.Fatalf("rules length %d", len(g.Rules()))
	}
	if !strings.Contains(g.Starter(), "#") {
		t.Fatal("starter is not markdown")
	}
	if _, err := json.Marshal(g.NewMatch(1, "ca", [2]string{"a", "b"}).Meta()); err != nil {
		t.Fatal(err)
	}
}

func TestRealism(t *testing.T) {
	const n = 120
	var rounds, ot, kills, hs, totalRounds, ctWins, planted, pistolSeen, eco, defuses, explode, timeouts int
	maxMoneySeen := 0
	type res struct {
		m  *Match
		ev []arena.Event
	}
	all := make([]res, n+1)
	parallel(n, func(i int) bool {
		m, ev := play(int64(i+1), [2]arena.Decide{empty, empty})
		all[i+1] = res{m, ev}
		return false
	})
	for s := int64(1); s <= n; s++ {
		m, ev := all[s].m, all[s].ev
		if !m.done || m.Winner() < 0 {
			t.Fatalf("seed %d: no winner", s)
		}
		sc := m.Score()
		w := m.Winner()
		if sc[w] <= sc[1-w] {
			t.Fatalf("seed %d: winner %d with score %v", s, w, sc)
		}
		if sc[w] != targetFor(m.round) || m.round > 24 && sc[1-w] < 12+3*((m.round-25)/6) {
			t.Fatalf("seed %d: bad final score %v after %d rounds", s, sc, m.round)
		}
		rounds += m.round
		if m.round > 24 {
			ot++
		}
		last := 0
		for _, e := range ev {
			if e.N <= last {
				t.Fatalf("event numbers not increasing")
			}
			last = e.N
			var d map[string]any
			_ = json.Unmarshal(e.Data, &d)
			switch e.Kind {
			case "kill":
				kills++
				if d["hs"] == true {
					hs++
				}
			case "round_end":
				totalRounds++
				if d["side"] == "CT" {
					ctWins++
				}
				switch d["reason"] {
				case "defuse":
					defuses++
				case "bomb":
					explode++
				case "time":
					timeouts++
				}
				if d["planted"] == true {
					planted++
				}
				for _, p := range d["players"].([]any) {
					mv := int(p.(map[string]any)["money"].(float64))
					if mv < 0 || mv > maxMoney {
						t.Fatalf("money out of bounds: %d", mv)
					}
					maxMoneySeen = max(maxMoneySeen, mv)
				}
			case "buy":
				if d["level"] == "eco" {
					eco++
				}
				if d["observed"] == "eco" {
					pistolSeen++
				}
			}
		}
	}
	avgR := float64(rounds) / n
	kpr := float64(kills) / float64(totalRounds)
	hsr := float64(hs) / float64(kills)
	t.Logf("rounds/match %.1f, OT %.1f%%, kills/round %.2f, HS %.1f%%, CT round wins %.1f%%, planted %.1f%%, defuses %.1f%%, explosions %.1f%%, time %.1f%%, eco buys %.1f%%, max money %d",
		avgR, 100*float64(ot)/n, kpr, 100*hsr, 100*float64(ctWins)/float64(totalRounds), 100*float64(planted)/float64(totalRounds),
		100*float64(defuses)/float64(totalRounds), 100*float64(explode)/float64(totalRounds), 100*float64(timeouts)/float64(totalRounds),
		100*float64(eco)/float64(2*totalRounds), maxMoneySeen)
	if avgR < 19 || avgR > 26 {
		t.Errorf("rounds per match %.1f out of range", avgR)
	}
	if kpr < 5.5 || kpr > 9 {
		t.Errorf("kills per round %.2f out of range", kpr)
	}
	if hsr < 0.38 || hsr > 0.52 {
		t.Errorf("headshot rate %.2f out of range", hsr)
	}
	if ot == 0 || float64(ot)/n > 0.3 {
		t.Errorf("overtime rate %d/%d", ot, n)
	}
	ctr := float64(ctWins) / float64(totalRounds)
	if ctr < 0.42 || ctr > 0.6 {
		t.Errorf("CT round win rate %.2f", ctr)
	}
}

func TestDeterminism(t *testing.T) {
	for s := int64(1); s < 6; s++ {
		ma, a := play(s, [2]arena.Decide{starter, empty})
		_, b := play(s, [2]arena.Decide{starter, empty})
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("seed %d: not deterministic", s)
		}
		// el mode ràpid de les proves juga exactament el mateix partit
		q := newMatch(s, "en", [2]string{"Alpha", "Bravo"})
		q.quiet = true
		arena.Play(q, [2]arena.Decide{starter, empty})
		if !reflect.DeepEqual(q.hist, ma.hist) || q.score != ma.score {
			t.Fatalf("seed %d: quiet mode plays a different match", s)
		}
	}
}

func TestInvalidAnswers(t *testing.T) {
	junk := []string{``, `null`, `[]`, `"full"`, `{"buy":42}`, `{"buy":"FULL BUY","strategy":"Execute A","utility":"???","pace":7,"awp":"yes"}`,
		`{"decision":{"buy":"force","setup":"stack b"}}`, `{"setup":"stack_c","strategy":"teleport"}`, `not json at all {"buy":"eco"} trailing`, `{"awp":null}`}
	for s := int64(0); s < 30; s++ {
		i := 0
		d := func(int, arena.Prompt) json.RawMessage { i++; return json.RawMessage(junk[(i+int(s))%len(junk)]) }
		m, ev := play(s, [2]arena.Decide{d, d})
		if !m.done || len(ev) == 0 {
			t.Fatal("match did not finish")
		}
	}
	m := newMatch(3, "ca", [2]string{"", ""})
	m.Next()
	d := m.parseT(json.RawMessage(`{"buy":"FULL BUY","strategy":"Execute A","awp":"no"}`), 0)
	if d.Buy != "full" || d.Strategy != "execute_a" || d.AWP || d.Pace != "default" || d.Utility != "smokes" {
		t.Fatalf("parse: %+v", d)
	}
	c := m.parseCT(json.RawMessage(`garbage`), 1)
	if c.Setup != "standard_2_1_2" || c.Buy != "full" || c.Utility != "normal" || !c.AWP {
		t.Fatalf("ct default: %+v", c)
	}
	// Cada camp té els seus àlies: "save" és eco a la compra però un valor vàlid de la utilitat CT.
	c = m.parseCT(json.RawMessage(`{"buy":"save","setup":"standard","utility":"save"}`), 1)
	if c.Buy != "eco" || c.Setup != "standard_2_1_2" || c.Utility != "save" {
		t.Fatalf("ct aliases: %+v", c)
	}
	c = m.parseCT(json.RawMessage(`{"buy":"force buy","setup":"Stack-A","utility":"HEAVY"}`), 1)
	if c.Buy != "force" || c.Setup != "stack_a" || c.Utility != "heavy" {
		t.Fatalf("ct parse: %+v", c)
	}
	d = m.parseT(json.RawMessage(`{"buy":"save","strategy":"fake a","utility":"smoke","pace":"normal"}`), 0)
	if d.Buy != "eco" || d.Strategy != "fake_a_go_b" || d.Utility != "smokes" || d.Pace != "default" {
		t.Fatalf("t aliases: %+v", d)
	}
}

func TestPromptsAndEvents(t *testing.T) {
	m := newMatch(42, "ca", [2]string{"Llamps", "Trons"})
	p, done := m.Next()
	if done || !strings.Contains(p[0].Situation, "PISTOL ROUND") || !strings.Contains(p[0].Schema, "buy") {
		t.Fatalf("prompt: %+v", p[0])
	}
	ev := m.Apply([2]json.RawMessage{nil, nil})
	kinds := map[string]bool{}
	for _, e := range ev {
		kinds[e.Kind] = true
		if e.Decision != 1 {
			t.Fatalf("decision index %d", e.Decision)
		}
		if e.Text == "" {
			t.Fatalf("empty text for %s", e.Kind)
		}
	}
	for _, k := range []string{"round_start", "buy", "move", "round_end"} {
		if !kinds[k] {
			t.Fatalf("missing %s", k)
		}
	}
	p, _ = m.Next()
	if !strings.Contains(p[1].Situation, "Opponent's last rounds") {
		t.Fatalf("no history: %s", p[1].Situation)
	}
	// totes les menes d'esdeveniment apareixen en uns quants partits
	all := map[string]bool{}
	hl := 0
	for s := int64(1); s < 40; s++ {
		_, ev := play(s, [2]arena.Decide{starter, starter})
		for _, e := range ev {
			all[e.Kind] = true
			if e.Highlight {
				hl++
			}
		}
	}
	for _, k := range []string{"round_start", "buy", "move", "utility", "kill", "plant", "defuse", "explode", "round_end", "half", "match_end"} {
		if !all[k] {
			t.Errorf("event kind %s never produced", k)
		}
	}
	if hl == 0 {
		t.Error("no highlights")
	}
}

// Objectiu 4: la Situation porta el que necessita un bon .md.
func TestSituationInfo(t *testing.T) {
	m := newMatch(9, "en", [2]string{"A", "B"})
	m.quiet = true
	var p [2]arena.Prompt
	for i := 0; i < 7; i++ {
		p, _ = m.Next()
		m.Apply([2]json.RawMessage{starter(0, p[0]), empty(1, p[1])})
	}
	p, _ = m.Next()
	for team := 0; team < 2; team++ {
		s := p[team].Situation
		for _, want := range []string{
			"Round 8 (first half, round 8 of 12 of this half; first to 13 wins)", "Score: you ",
			"Your money: $", "Full buy now costs your team about $", "Loss bonus: if you lose this round each player gets $",
			"Enemy (", "estimated money: $", "their next loss pays $", "AWP: you ", "the enemy ",
			"Opponent's last rounds as ", "tendencies this match", "Your last rounds as ", "-> we ", "Kept from last round",
		} {
			if !strings.Contains(s, want) {
				t.Errorf("team %d situation lacks %q:\n%s", team, want, s)
			}
		}
		x := parseSit(s)
		if len(x.their) != 6 || len(x.ours) != 5 || x.round != 8 || x.pos != 8 || x.halfLen != 12 || x.money == 0 || x.full == 0 || x.loss == 0 {
			t.Errorf("team %d: parsed %+v", team, x)
		}
		for _, h := range x.ours {
			if h.call == "" || h.vs == "" {
				t.Errorf("own line without calls: %+v", h)
			}
		}
	}
	both := p[0].Situation + p[1].Situation
	if !strings.Contains(both, "-> we LOST") || !strings.Contains(both, "-> we WON") {
		t.Error("own results not reported")
	}
}

// El .md inicial diu el mateix que la política "starter" de les proves.
func TestStarterText(t *testing.T) {
	for _, want := range []string{"60%", "Never \"half\"", "fake_a_go_b", "fake_b_go_a", "aggressive_mid", "execute_a", "execute_b",
		"rush_a on even rounds", "\"default_mid\"", "stack_a", "\"heavy\" on a full buy", "\"save\" on an eco", "we LOST", "Last round of the half"} {
		if !strings.Contains(starterText, want) {
			t.Errorf("starter lacks %q", want)
		}
	}
	for _, v := range append(append(append([]string{}, stratVals...), setupVals...), tUtilVals...) {
		if !strings.Contains(rulesText+tSchema+ctSchema, v) {
			t.Errorf("rules/schema never mention %q", v)
		}
	}
}
