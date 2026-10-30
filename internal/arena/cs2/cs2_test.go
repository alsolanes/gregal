package cs2

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gregal/internal/arena"
)

// ---- polítiques de prova ----

func empty(int, arena.Prompt) json.RawMessage { return nil }

var reMoney = regexp.MustCompile(`Your money: \$([\d,]+) total`)
var reFull = regexp.MustCompile(`Full buy now costs your team about \$([\d,]+) \((affordable|NOT affordable)\)`)
var reLoss = regexp.MustCompile(`each player gets \$([\d,]+)`)
var reRound = regexp.MustCompile(`Round (\d+) `)

func num(s string) int { v, _ := strconv.Atoi(strings.ReplaceAll(s, ",", "")); return v }

type sit struct {
	ct          bool
	money, full int
	affordable  bool
	loss        int
	round       int
	pistol      bool
	lastOfHalf  bool
	enemyPoor   bool
	theyT       []string // línies d'historial del rival com a T
	theyCT      []string
}

func parseSit(s string) sit {
	var x sit
	x.ct = strings.Contains(s, "You are CT")
	if m := reMoney.FindStringSubmatch(s); m != nil {
		x.money = num(m[1])
	}
	if m := reFull.FindStringSubmatch(s); m != nil {
		x.full, x.affordable = num(m[1]), m[2] == "affordable"
	}
	if m := reLoss.FindStringSubmatch(s); m != nil {
		x.loss = num(m[1])
	}
	if m := reRound.FindStringSubmatch(s); m != nil {
		x.round = num(m[1])
	}
	x.pistol = strings.Contains(s, "PISTOL ROUND") || strings.Contains(s, "Overtime half start")
	x.lastOfHalf = strings.Contains(s, "Last round of the half")
	x.enemyPoor = strings.Contains(s, "probably cannot full buy")
	half := (x.round - 1) / 12
	for _, l := range strings.Split(s, "\n") {
		if !strings.HasPrefix(l, "- R") {
			continue
		}
		r := num(strings.TrimPrefix(strings.Fields(l)[1], "R"))
		if (r-1)/12 != half && x.round <= 24 {
			continue
		}
		if strings.Contains(l, "[they T]") {
			x.theyT = append(x.theyT, l)
		} else if strings.Contains(l, "[they CT]") {
			x.theyCT = append(x.theyCT, l)
		}
	}
	return x
}

func adaptiveBuy(x sit) string {
	switch {
	case x.pistol || x.affordable:
		return "full"
	case x.lastOfHalf:
		return "force"
	case x.money+5*x.loss >= x.full:
		if x.enemyPoor && x.money >= x.full*6/10 {
			return "half"
		}
		return "eco"
	case x.enemyPoor:
		return "force"
	case x.money >= x.full*55/100:
		return "force"
	}
	return "eco"
}

func adaptive(_ int, p arena.Prompt) json.RawMessage {
	x := parseSit(p.Situation)
	b := adaptiveBuy(x)
	var d map[string]any
	if x.ct {
		setup, util := "standard_2_1_2", "normal"
		recent := x.theyT
		if len(recent) > 4 {
			recent = recent[len(recent)-4:]
		}
		a, bb, rush, def := 0, 0, 0, 0
		for _, l := range recent {
			if strings.Contains(l, "hit A") || strings.Contains(l, "rushed A") || strings.Contains(l, "executed A") || strings.Contains(l, "split A") || strings.Contains(l, "then hit A") {
				a++
			}
			if strings.Contains(l, "hit B") || strings.Contains(l, "rushed B") || strings.Contains(l, "executed B") || strings.Contains(l, "split B") || strings.Contains(l, "then hit B") {
				bb++
			}
			if strings.Contains(l, "rushed") {
				rush++
			}
			if strings.Contains(l, "default/mid") {
				def++
			}
		}
		n := len(recent)
		switch {
		case n >= 2 && a*10 >= n*7:
			setup = "stack_a"
		case n >= 2 && bb*10 >= n*7:
			setup = "stack_b"
		case n >= 2 && def*2 >= n:
			setup = "aggressive_mid"
		}
		if rush >= 1 || b == "full" {
			util = "heavy"
		}
		d = map[string]any{"buy": b, "setup": setup, "utility": util, "awp": true}
	} else {
		strat, util := "", "smokes"
		if b == "full" {
			util = "full"
		}
		if n := len(x.theyCT); n > 0 {
			last := x.theyCT[n-1]
			switch {
			case strings.Contains(last, "stack_a"):
				strat = "execute_b"
				if x.round%2 == 0 {
					strat = "fake_a_go_b"
				}
			case strings.Contains(last, "stack_b"):
				strat = "execute_a"
				if x.round%2 == 0 {
					strat = "fake_b_go_a"
				}
			case strings.Contains(last, "aggressive_mid"):
				strat = "rush_b"
			case strings.Contains(last, "retake_b"):
				strat = "execute_b"
			case strings.Contains(last, "retake_a"):
				strat = "execute_a"
			}
		}
		if strat == "" {
			strat = []string{"execute_a", "execute_b", "default_mid", "split_b", "execute_b", "split_a"}[x.round%6]
		}
		if b == "eco" || b == "half" {
			strat = []string{"rush_a", "rush_b"}[x.round%2]
		}
		d = map[string]any{"buy": b, "strategy": strat, "utility": util, "pace": "default", "awp": true}
	}
	raw, _ := json.Marshal(d)
	return raw
}

func naive(_ int, p arena.Prompt) json.RawMessage {
	x := parseSit(p.Situation)
	b := "eco"
	if x.pistol || x.affordable {
		b = "full"
	}
	if x.ct {
		return json.RawMessage(fmt.Sprintf(`{"buy":%q,"setup":"standard_2_1_2","utility":"normal","awp":true}`, b))
	}
	return json.RawMessage(fmt.Sprintf(`{"buy":%q,"strategy":"execute_a","utility":"smokes","pace":"default","awp":true}`, b))
}

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
	if len(g.Rules()) > 2600 || len(g.Rules()) < 500 {
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
	const n = 400
	var rounds, ot, kills, hs, totalRounds, ctWins, planted, pistolSeen, eco, defuses, explode, timeouts int
	maxMoneySeen := 0
	for s := int64(1); s <= n; s++ {
		m, ev := play(s, [2]arena.Decide{empty, empty})
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

func winRate(t *testing.T, a, b arena.Decide, n int) float64 {
	wins := 0
	for s := int64(0); s < int64(n); s++ {
		// el jugador A alterna d'índex per neutralitzar el costat
		if s%2 == 0 {
			m, _ := play(1000+s, [2]arena.Decide{a, b})
			if m.Winner() == 0 {
				wins++
			}
		} else {
			m, _ := play(1000+s, [2]arena.Decide{b, a})
			if m.Winner() == 1 {
				wins++
			}
		}
	}
	return float64(wins) / float64(n)
}

func TestPolicies(t *testing.T) {
	ad := winRate(t, adaptive, naive, 300)
	t.Logf("adaptive vs naive: %.1f%%", ad*100)
	if ad < 0.65 {
		t.Errorf("adaptive should beat naive >=65%%, got %.1f%%", ad*100)
	}
	if ad > 0.9 {
		t.Errorf("naive should still win >10%%, adaptive got %.1f%%", ad*100)
	}
	aa := winRate(t, adaptive, adaptive, 300)
	t.Logf("adaptive vs adaptive (slot 0 in even seeds): %.1f%%", aa*100)
	if aa < 0.4 || aa > 0.6 {
		t.Errorf("mirror match should be ~50%%, got %.1f%%", aa*100)
	}
	nn := winRate(t, naive, empty, 200)
	t.Logf("naive vs defaults: %.1f%%", nn*100)
}

func TestDeterminism(t *testing.T) {
	for s := int64(1); s < 6; s++ {
		_, a := play(s, [2]arena.Decide{adaptive, naive})
		_, b := play(s, [2]arena.Decide{adaptive, naive})
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("seed %d: not deterministic", s)
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
	if c.Setup != "standard_2_1_2" || c.Buy != "full" {
		t.Fatalf("ct default: %+v", c)
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
		_, ev := play(s, [2]arena.Decide{adaptive, adaptive})
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
