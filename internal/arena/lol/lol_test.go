package lol

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"gregal/internal/arena"
)

// play juga un partit sencer amb dues polítiques. Les polítiques de prova
// només llegeixen el Prompt (Situation + Schema), no l'estat intern.
func play(seed int64, a, b policy) (*match, []arena.Event) {
	m := newMatch(seed, "en", [2]string{"Alpha", "Beta"})
	pol := [2]policy{a, b}
	dec := func(k int) arena.Decide {
		return func(s int, p arena.Prompt) json.RawMessage {
			if p.Schema == "" {
				return nil
			}
			return pol[k](m, s, p)
		}
	}
	ev := arena.Play(m, [2]arena.Decide{dec(0), dec(1)})
	return m, ev
}

// playMany juga n partits en paral·lel (llavor = seed(k)).
func playMany(n int, seed func(k int) int64, a, b policy) []*match {
	out := make([]*match, n)
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	for k := 0; k < n; k++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(k int) {
			defer func() { <-sem; wg.Done() }()
			out[k], _ = play(seed(k+1), a, b)
		}(k)
	}
	wg.Wait()
	return out
}

// winRate: % de victòries de a contra b en n partits (a alterna d'equip i el
// costat blau/vermell surt de la llavor).
func winRate(n int, a, b policy) float64 {
	var wg sync.WaitGroup
	won := make([]bool, n+1)
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	for k := 1; k <= n; k++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(k int) {
			defer func() { <-sem; wg.Done() }()
			seed := int64(k)*7919 + 13
			if k%2 == 0 {
				m, _ := play(seed, a, b)
				won[k] = m.winner == 0
			} else {
				m, _ := play(seed, b, a)
				won[k] = m.winner == 1
			}
		}(k)
	}
	wg.Wait()
	wins := 0
	for _, w := range won {
		if w {
			wins++
		}
	}
	return float64(wins) / float64(n)
}

type stats struct {
	n                                 int
	lens, kills, drag, barons, towers []float64
	comeback, behind15                int
}

func (s *stats) add(m *match) {
	s.n++
	s.lens = append(s.lens, m.evT/60)
	s.kills = append(s.kills, float64(m.kills[0]+m.kills[1]))
	s.drag = append(s.drag, float64(len(m.dragons[0])+len(m.dragons[1])))
	s.barons = append(s.barons, float64(m.barons[0]+m.barons[1]))
	s.towers = append(s.towers, float64(m.towersBy(0)+m.towersBy(1)))
	if m.has15 && math.Abs(m.gold15[0]-m.gold15[1]) > 500 {
		s.behind15++
		lead := 0
		if m.gold15[1] > m.gold15[0] {
			lead = 1
		}
		if m.winner != lead {
			s.comeback++
		}
	}
}

func (s *stats) log(t *testing.T, what string) {
	t.Logf("%s: length mean %.1f median %.1f min %.1f max %.1f | kills %.1f | dragons %.1f | barons %.2f | towers %.1f | comebacks %d/%d",
		what, mean(s.lens), median(s.lens), minF(s.lens), maxF(s.lens), mean(s.kills), mean(s.drag), mean(s.barons), mean(s.towers), s.comeback, s.behind15)
}

func mean(v []float64) float64 {
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}
func sorted(v []float64) []float64 {
	c := append([]float64{}, v...)
	sort.Float64s(c)
	return c
}
func median(v []float64) float64 { return sorted(v)[len(v)/2] }
func minF(v []float64) float64   { return sorted(v)[0] }
func maxF(v []float64) float64   { return sorted(v)[len(v)-1] }

func checkRange(t *testing.T, what string, v, lo, hi float64) {
	t.Helper()
	if v < lo || v > hi {
		t.Errorf("%s = %.2f, want [%.2f, %.2f]", what, v, lo, hi)
	}
}

func TestDefaultsRealistic(t *testing.T) {
	var s stats
	for seed := int64(1); seed <= 300; seed++ {
		m, ev := play(seed, defaults, defaults)
		if m.winner < 0 || !m.done {
			t.Fatalf("seed %d: no winner", seed)
		}
		if ev[len(ev)-1].Kind != "match_end" {
			t.Fatalf("seed %d: last event %s", seed, ev[len(ev)-1].Kind)
		}
		for k, e := range ev {
			if e.N != k+1 || e.Decision < 1 {
				t.Fatalf("seed %d: bad event numbering %+v", seed, e)
			}
		}
		s.add(m)
	}
	s.log(t, "defaults")
	checkRange(t, "median length", median(s.lens), 26, 36)
	checkRange(t, "mean kills", mean(s.kills), 16, 36)
	checkRange(t, "mean dragons", mean(s.drag), 2.5, 6.5)
	checkRange(t, "mean barons", mean(s.barons), 0.3, 2)
	checkRange(t, "mean towers", mean(s.towers), 8, 20)
}

func TestStarterMirrorRealism(t *testing.T) {
	var s stats
	blue := 0
	for _, m := range playMany(300, func(k int) int64 { return int64(k) * 31 }, starter, starter) {
		s.add(m)
		if m.winner == m.blue {
			blue++
		}
	}
	s.log(t, "starter mirror")
	t.Logf("starter mirror: blue side wins %d/300", blue)
	checkRange(t, "median length", median(s.lens), 24, 36)
	checkRange(t, "mean kills", mean(s.kills), 14, 36)
	checkRange(t, "comeback rate", float64(s.comeback)/float64(s.behind15), 0.08, 0.45)
	checkRange(t, "mirror blue wins", float64(blue)/300, 0.4, 0.6)
}

// Objectiu 4: per defecte (resposta invàlida o buida) les dues bandes reben
// un draft igual de bo; el blau no guanya per l'ordre del pool.
func TestFairDefaults(t *testing.T) {
	blue := 0
	const n = 800
	for _, m := range playMany(n, func(k int) int64 { return int64(k) }, defaults, defaults) {
		if m.winner == m.blue {
			blue++
		}
	}
	t.Logf("default vs default: blue wins %.1f%%", 100*float64(blue)/n)
	checkRange(t, "default blue win rate", float64(blue)/n, 0.45, 0.55)
}

// Objectius 1-3: un .md que llegeix la Situation guanya els constants, i
// "sempre engage + sempre disputar" no és dominant.
func TestStrategyMatters(t *testing.T) {
	n := 60
	if testing.Short() {
		n = 40
	}
	cs := append(constantPolicies(), named{"beginner (good draft, always engage+contest)", beginner})
	for _, c := range cs {
		ws := winRate(n, starter, c.p)
		we := winRate(n, exploiter, c.p)
		t.Logf("%-46s starter %5.1f%%  exploiter %5.1f%%", c.name, 100*ws, 100*we)
		min := 0.5
		if c.name == "no-answer" || strings.Contains(c.name, "engage+contest") {
			min = 0.6
		}
		if ws < min { // els valors reals són ≥ 85%: el marge cobreix el soroll
			t.Errorf("starter vs %s = %.1f%%, want ≥ %.0f%%", c.name, 100*ws, 100*min)
		}
		if we < 0.7 {
			t.Errorf("exploiter vs %s = %.1f%%, want ≥ 70%%", c.name, 100*we)
		}
	}
}

// Objectius 1 i 5: cada decisió compta. Contrarestar la postura, no disputar
// anant clarament per darrere, i jugar segons la corba de la composició.
func TestDecisionsMatter(t *testing.T) {
	n := 240
	if testing.Short() {
		n = 100
	}
	check := func(what string, v, lo, hi float64) {
		t.Helper()
		t.Logf("%-58s %5.1f%%", what, 100*v)
		checkRange(t, what, v, lo, hi)
	}
	check("starter that always contests vs starter", winRate(n, starterWith(alwaysContestMacro), starter), 0, 0.45)
	check("starter that never counters stances vs starter", winRate(n, starterWith(noCounterMacro), starter), 0, 0.35)
	check("exploiter vs starter", winRate(n, exploiter, starter), 0.42, 0.75)
	ss, sf := fixedComp(scalingComp, scaleMacro), fixedComp(scalingComp, forceMacro)
	est, es := fixedComp(earlyComp, starterMacro), fixedComp(earlyComp, scaleMacro)
	// Una composició scaling que es baralla d'hora perd; si espera, aguanta.
	check("scaling comp waiting vs early comp (starter)", winRate(n, ss, est), 0.2, 0.6)
	check("scaling comp forcing early vs early comp (starter)", winRate(n, sf, est), 0, 0.3)
	// Una composició early que no força perd la seva finestra.
	check("early comp forcing (starter) vs scaling waiting", winRate(n, est, ss), 0.55, 1)
	check("early comp waiting vs scaling waiting", winRate(n, es, ss), 0, 0.5)
}

// Objectiu 1: les condicions de l'enfrontament de postures.
func TestStanceConditions(t *testing.T) {
	m, _ := play(3, defaults, defaults)
	// Mateixa alineació a les dues bandes: només hi compten l'encaix i l'or.
	m.line[1] = m.line[0]
	set := func(fit0, fit1 map[string]float64, gold0, gold1 float64) {
		m.fit[0], m.fit[1] = fit0, fit1
		m.gold = [2]float64{gold0, gold1}
	}
	avg := func(kv ...any) map[string]float64 {
		f := map[string]float64{"engage": 1, "siege": 1, "pick": 1, "split": 1, "protect": 1}
		for k := 0; k < len(kv); k += 2 {
			f[kv[k].(string)] = kv[k+1].(float64)
		}
		return f
	}
	// Cicle base.
	set(avg(), avg(), 10000, 10000)
	for _, c := range [][2]string{{"engage", "siege"}, {"engage", "pick"}, {"siege", "split"}, {"siege", "protect"}, {"pick", "siege"}, {"pick", "split"}, {"split", "protect"}, {"protect", "pick"}} {
		if !m.stanceWins(0, c[0], c[1]) || m.stanceWins(1, c[1], c[0]) {
			t.Errorf("%s should beat %s", c[0], c[1])
		}
	}
	// protect contra engage: només si encaixa.
	set(avg("protect", 1.2), avg(), 10000, 10000)
	if !m.stanceWins(0, "protect", "engage") || m.stanceWins(1, "engage", "protect") {
		t.Error("protect (fit 1.2) should beat engage")
	}
	set(avg("protect", 0.7), avg(), 10000, 10000)
	if m.protectOK(0) || !m.stanceWins(1, "engage", "protect") {
		t.Error("engage should beat a protect that does not qualify")
	}
	set(avg("protect", 0.7), avg(), 12000, 10000)
	if m.stanceWins(1, "engage", "protect") {
		t.Error("an engage team 2k behind should not beat protect")
	}
	// split contra engage: cal un splitpusher i no anar per darrere.
	set(avg("split", 1.3), avg(), 10000, 10000)
	if !m.stanceWins(0, "split", "engage") {
		t.Error("split (fit 1.3) should beat engage when even")
	}
	set(avg("split", 1.3), avg(), 10000, 12000)
	if m.stanceWins(0, "split", "engage") || !m.stanceWins(1, "engage", "split") {
		t.Error("engage should beat split when the splitter is 2k behind")
	}
	// L'enfrontament pesa: guanyar-lo amb bon encaix ≈ +22%.
	set(avg("protect", 1.2), avg(), 10000, 10000)
	if e := m.stanceEdge(0, "protect", "engage"); e < 0.2 {
		t.Errorf("stance edge %.2f", e)
	}
	if formSD > 0.05 {
		t.Error("hidden form must stay small")
	}
}

func TestDeterminism(t *testing.T) {
	_, a := play(42, starter, constant("engage", "contest", "aggressive"))
	_, b := play(42, starter, constant("engage", "contest", "aggressive"))
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("same seed must give the same match")
	}
}

// Objectiu 7: el marcador final que veu l'app no pot fer semblar perdedor el
// guanyador; l'esdeveniment final diu clarament qui guanya.
func TestScoreShowsWinner(t *testing.T) {
	for seed := int64(1); seed <= 100; seed++ {
		m, ev := play(seed, starter, constant("engage", "contest", "aggressive"))
		sc := m.Score()
		if sc[m.winner] != 1 || sc[1-m.winner] != 0 {
			t.Fatalf("seed %d: final score %v, winner %d", seed, sc, m.winner)
		}
		end := ev[len(ev)-1]
		var d map[string]any
		_ = json.Unmarshal(end.Data, &d)
		if int(d["winner"].(float64)) != m.winner || !strings.Contains(end.Text, m.names[m.winner]+" WIN") || d["result"] == nil {
			t.Fatalf("seed %d: bad match_end %s %s", seed, end.Text, end.Data)
		}
		// El marcador dels esdeveniments segueix sent de morts (el visor).
		if end.Score != m.kills {
			t.Fatalf("event score must stay kills")
		}
	}
}

func TestInvalidAnswers(t *testing.T) {
	junk := func(m *match, side int, p arena.Prompt) json.RawMessage {
		switch m.dec % 4 {
		case 0:
			return json.RawMessage(`{"picks":["Nobody","Ironwall","Ironwall"],"bans":[3,"x"],"objective":"nexus","stance":"yolo","top":"crazy"}`)
		case 1:
			return json.RawMessage(`not json`)
		case 2:
			return json.RawMessage(`{"decision":{"objective":"baron","stance":"split"}}`)
		}
		return nil
	}
	for seed := int64(1); seed <= 50; seed++ {
		m, _ := play(seed, junk, defaults)
		if !m.done || m.winner < 0 {
			t.Fatal("junk answers must still finish the game")
		}
		for i := 0; i < 2; i++ {
			for r := 0; r < 5; r++ {
				if m.line[i][r] == nil {
					t.Fatal("incomplete lineup")
				}
			}
		}
		seen := map[string]bool{}
		for i := 0; i < 2; i++ {
			for _, id := range append(append([]int{}, m.picks[i]...), m.bans[i]...) {
				if seen[pool[id].Name] {
					t.Fatalf("duplicate %s", pool[id].Name)
				}
				seen[pool[id].Name] = true
			}
		}
	}
}

// Objectiu 6: la Situation porta el que un bon .md necessita.
func TestSituationContents(t *testing.T) {
	m := newMatch(9, "en", [2]string{"A", "B"})
	pol := [2]policy{starter, constant("engage", "contest", "aggressive")}
	sawMacro := 0
	for {
		p, done := m.Next()
		if done {
			break
		}
		var ans [2]json.RawMessage
		for i := 0; i < 2; i++ {
			if p[i].Schema != "" {
				ans[i] = pol[i](m, i, p[i])
			}
		}
		if m.stage >= stMacro {
			sawMacro++
			s := p[0].Situation
			for _, want := range []string{"GOLD DIFF", "kills you", "towers destroyed", "POWER NOW", "In 8 min", "OBJECTIVES YOU CAN CHOOSE NOW",
				"Dragons: you", "YOUR COMP: ", "ENEMY COMP: ", "stance fit", "CONDITIONAL COUNTERS NOW", "HISTORY", "→ "} {
				if !strings.Contains(s, want) {
					t.Fatalf("macro situation lacks %q:\n%s", want, s)
				}
			}
			if sawMacro >= 2 && !strings.Contains(s, "ENEMY STANCES SO FAR: engage") {
				t.Fatalf("macro situation lacks the enemy's stances:\n%s", s)
			}
			if sawMacro == 3 {
				ps := parseSit(s)
				if len(ps.enStances) != 2 || ps.enStances[0] != "engage" || len(ps.opts) < 2 || ps.myFit == nil || ps.enT == nil {
					t.Fatalf("situation not parseable: %+v\n%s", ps, s)
				}
			}
		}
		m.Apply(ans)
	}
	if sawMacro < 2 {
		t.Fatal("no macro windows")
	}
}

func TestPromptsAndContract(t *testing.T) {
	g, ok := arena.Get("lol")
	if !ok {
		t.Fatal("not registered")
	}
	if len(g.Rules()) > 3400 {
		t.Errorf("rules too long: %d", len(g.Rules()))
	}
	if !strings.Contains(g.Starter(), "Draft") {
		t.Error("starter")
	}
	m := g.NewMatch(5, "ca", [2]string{"A", "B"})
	if _, err := json.Marshal(m.Meta()); err != nil {
		t.Fatal(err)
	}
	decisions := 0
	for {
		p, done := m.Next()
		if done {
			break
		}
		decisions++
		empty := 0
		for i := 0; i < 2; i++ {
			if p[i].Schema == "" {
				empty++
			} else if p[i].Situation == "" || p[i].Short == "" {
				t.Fatal("empty prompt")
			}
		}
		if empty == 2 {
			t.Fatal("nobody decides")
		}
		evs := m.Apply([2]json.RawMessage{nil, nil})
		for _, e := range evs {
			if e.Text == "" || !json.Valid(e.Data) {
				t.Fatalf("bad event %+v", e)
			}
		}
	}
	if decisions < 14 || decisions > 30 {
		t.Errorf("decisions = %d", decisions)
	}
	if m.Winner() < 0 {
		t.Error("no winner")
	}
}

// TestTuneMatrix (LOL_TUNE=1): la taula completa per afinar el motor.
func TestTuneMatrix(t *testing.T) {
	if os.Getenv("LOL_TUNE") == "" {
		t.Skip("LOL_TUNE=1 to run")
	}
	n := 300
	cs := constantPolicies()
	for _, c := range cs {
		t.Logf("%-24s starter %5.1f%%  exploiter %5.1f%%", c.name, 100*winRate(n, starter, c.p), 100*winRate(n, exploiter, c.p))
	}
	t.Logf("exploiter vs starter %5.1f%%", 100*winRate(n, exploiter, starter))
	t.Logf("starter-always-contest vs starter %5.1f%%", 100*winRate(n, starterWith(alwaysContestMacro), starter))
	t.Logf("starter-no-counter vs starter %5.1f%%", 100*winRate(n, starterWith(noCounterMacro), starter))
	ef, es := fixedComp(earlyComp, forceMacro), fixedComp(earlyComp, scaleMacro)
	sf, ss := fixedComp(scalingComp, forceMacro), fixedComp(scalingComp, scaleMacro)
	t.Logf("early-force vs scaling-scale %5.1f%% | early-scale vs scaling-scale %5.1f%% | early-force vs scaling-force %5.1f%% | early-scale vs scaling-force %5.1f%%",
		100*winRate(n, ef, ss), 100*winRate(n, es, ss), 100*winRate(n, ef, sf), 100*winRate(n, es, sf))
	t.Logf("early-starter vs scaling-starter %5.1f%%", 100*winRate(n, fixedComp(earlyComp, starterMacro), fixedComp(scalingComp, starterMacro)))
	// Constants entre ells: cap n'hauria de ser dominant.
	for a := 1; a < len(cs); a += 3 {
		row := ""
		for b := 1; b < len(cs); b += 3 {
			row += fmt.Sprintf(" %s=%.0f%%", strings.Fields(cs[b].name)[1], 100*winRate(100, cs[a].p, cs[b].p))
		}
		t.Logf("%-22s%s", cs[a].name, row)
	}
}
