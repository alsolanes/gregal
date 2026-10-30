package cs2

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"gregal/internal/arena"
)

// Proves d'equilibri: simulacions amb llavor i polítiques escrites a mà
// (policy_test.go) que només llegeixen la Situation.

// parallel: executa f(0..n-1) en paral·lel i compta quantes tornen true.
func parallel(n int, f func(i int) bool) int {
	var mu sync.Mutex
	wins := 0
	ch := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := 0
			for i := range ch {
				if f(i) {
					c++
				}
			}
			mu.Lock()
			wins += c
			mu.Unlock()
		}()
	}
	for i := 0; i < n; i++ {
		ch <- i
	}
	close(ch)
	wg.Wait()
	return wins
}

// matchRate: % de partits que guanya a contra b. Alterna l'índex dels
// equips; el costat inicial surt de la llavor.
func matchRate(a, b arena.Decide, n int, base int64) float64 {
	w := parallel(n, func(i int) bool {
		m := newMatch(base+int64(i), "en", [2]string{"a", "b"})
		m.quiet = true
		if i%2 == 0 {
			arena.Play(m, [2]arena.Decide{a, b})
			return m.Winner() == 0
		}
		arena.Play(m, [2]arena.Decide{b, a})
		return m.Winner() == 1
	})
	return float64(w) / float64(n)
}

// tFirstRate: % de partits que guanya l'equip que comença de T.
func tFirstRate(a, b arena.Decide, n int, base int64) float64 {
	w := parallel(n, func(i int) bool {
		m := newMatch(base+int64(i), "en", [2]string{"a", "b"})
		m.quiet = true
		d := [2]arena.Decide{a, b}
		if i%2 == 1 {
			d = [2]arena.Decide{b, a}
		}
		arena.Play(m, d)
		return m.Winner() == m.startT
	})
	return float64(w) / float64(n)
}

// roundRate: % de rondes que guanya el T amb aquestes decisions i diners
// (ronda normal, no de pistoles).
func roundRate(td tDec, cd ctDec, tMoney, ctMoney int, n int) float64 {
	w := parallel(n, func(i int) bool {
		m := newMatch(int64(7000+i), "en", [2]string{"a", "b"})
		m.quiet = true
		m.round = 2
		m.tTeam = 0
		for _, p := range m.players[0] {
			p.Money = tMoney
		}
		for _, p := range m.players[1] {
			p.Money = ctMoney
		}
		m.playRound(td, cd)
		return m.score[0] == 1
	})
	return float64(w) / float64(n)
}

func calPace(st string) string {
	if strings.HasPrefix(st, "rush") {
		return "fast"
	}
	return "default"
}

func measureMatrix(n int) [9][6]float64 {
	var out [9][6]float64
	for i, st := range stratVals {
		for j, se := range setupVals {
			out[i][j] = roundRate(tDec{"full", st, "full", calPace(st), true}, ctDec{"full", se, "normal", true}, 10000, 10000, n)
		}
	}
	return out
}

// Objectiu 1a: la taula de les regles és la del motor i és pedra-paper-tisora.
func TestMatchupTable(t *testing.T) {
	if !strings.Contains(rulesText, matchupTable()) {
		t.Fatal("rules do not contain the matchup table")
	}
	// La taula publicada és pedra-paper-tisora.
	for i, row := range matchupWin {
		lo := 100
		for _, v := range row {
			lo = min(lo, v)
		}
		if lo > 35 {
			t.Errorf("rules: no CT setup beats %s (best CT result %d%%)", stratVals[i], 100-lo)
		}
	}
	for j := range setupVals {
		hi := 0
		for i := range matchupWin {
			hi = max(hi, matchupWin[i][j])
		}
		if hi < 60 {
			t.Errorf("rules: no T strategy beats %s (best T result %d%%)", setupVals[j], hi)
		}
	}
	// I el motor dona aquesta taula.
	m := measureMatrix(600)
	var b strings.Builder
	for i := range m {
		fmt.Fprintf(&b, "%-12s", stratVals[i])
		for j := range m[i] {
			fmt.Fprintf(&b, " %3.0f(%2d)", 100*m[i][j], matchupWin[i][j])
			if d := 100*m[i][j] - float64(matchupWin[i][j]); d > 6 || d < -6 {
				t.Errorf("%s vs %s: engine %.0f%%, rules say %d%%", stratVals[i], setupVals[j], 100*m[i][j], matchupWin[i][j])
			}
		}
		b.WriteString("\n")
	}
	t.Log("engine (rules) T win %:\n" + b.String())
}

// Objectiu 1b: l'economia compta.
func TestEconomyMatters(t *testing.T) {
	full := roundRate(tDec{"full", "default_mid", "full", "default", true}, ctDec{"full", "standard_2_1_2", "normal", true}, 10000, 10000, 1000)
	f34 := roundRate(tDec{"force", "default_mid", "full", "default", false}, ctDec{"full", "standard_2_1_2", "normal", true}, 3400, 10000, 1000)
	f25 := roundRate(tDec{"force", "default_mid", "full", "default", false}, ctDec{"full", "standard_2_1_2", "normal", true}, 2500, 10000, 1000)
	eco := roundRate(tDec{"eco", "default_mid", "none", "default", false}, ctDec{"full", "standard_2_1_2", "normal", true}, 10000, 10000, 1000)
	t.Logf("T vs a CT full buy: full %.0f%%, force $3,400 %.0f%%, force $2,500 %.0f%%, eco %.0f%%", 100*full, 100*f34, 100*f25, 100*eco)
	if !(full > f34+0.12 && f34 > f25+0.05 && f25 > eco+0.04 && eco < 0.05) {
		t.Error("equipment should matter: full >> force($3,400) > force($2,500) > eco")
	}
	plain := constant("", "default_mid", "standard_2_1_2")
	alwaysForce := func(sit) string { return "force" }
	r1 := matchRate(withBuy(plain, starterBuy), withBuy(plain, alwaysForce), 1200, 100)
	r2 := matchRate(starter, withBuy(starter, alwaysForce), 1200, 200)
	t.Logf("sound economy vs forcing every round: same fixed plan %.1f%%, starter plan %.1f%%", 100*r1, 100*r2)
	if r1 < 0.56 || r2 < 0.52 {
		t.Error("forcing every round should lose to a sound economy")
	}
}

// constants: totes les polítiques fixes (compra + estratègia + configuració;
// "" = no ho diu).
func constants() map[string]arena.Decide {
	out := map[string]arena.Decide{}
	for _, b := range []string{"", "eco", "half", "force", "full"} {
		for _, st := range stratVals {
			for _, se := range setupVals {
				out[fmt.Sprintf("%s/%s/%s", or(b, "default"), st, se)] = constant(b, st, se)
			}
		}
	}
	return out
}

func or(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// againstConstants: cribratge ràpid de totes les constants i repetició
// amb més partits per a les més difícils. Torna el pitjor resultat.
func againstConstants(t *testing.T, p arena.Decide, quick, deep, worstN int) (float64, string) {
	type res struct {
		name string
		r    float64
	}
	var rs []res
	for name, c := range constants() {
		rs = append(rs, res{name, matchRate(p, c, quick, 300)})
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].r < rs[j].r || rs[i].r == rs[j].r && rs[i].name < rs[j].name })
	worst, wn := 1.0, ""
	for _, x := range rs[:worstN] {
		r := matchRate(p, constants()[x.name], deep, 900)
		t.Logf("  vs %-36s quick %.0f%%, deep %.1f%%", x.name, 100*x.r, 100*r)
		if r < worst {
			worst, wn = r, x.name
		}
	}
	return worst, wn
}

// Objectiu 2: cap política fixa guanya la del .md inicial.
func TestStarterBeatsConstants(t *testing.T) {
	na := matchRate(starter, empty, 600, 50)
	t.Logf("starter vs no answer: %.1f%%", 100*na)
	if na < 0.6 {
		t.Errorf("starter should beat the no-answer defaults >= 60%%, got %.1f%%", 100*na)
	}
	worst, wn := againstConstants(t, starter, 24, 150, 6)
	t.Logf("starter worst vs a constant policy: %.1f%% (%s)", 100*worst, wn)
	if worst < 0.5 {
		t.Errorf("constant %s beats the starter: %.1f%%", wn, 100*(1-worst))
	}
}

// Objectiu 3: qui llegeix l'historial i contraresta guanya les constants.
func TestExploitBeatsConstants(t *testing.T) {
	worst, wn := againstConstants(t, exploit, 24, 150, 6)
	t.Logf("exploit worst vs a constant policy: %.1f%% (%s)", 100*worst, wn)
	if worst < 0.7 {
		t.Errorf("exploit should beat every constant >= 70%%, %s holds it to %.1f%%", wn, 100*worst)
	}
}

// Objectiu 5: començar de T o de CT és igual.
func TestSidesFair(t *testing.T) {
	for _, c := range []struct {
		name string
		a, b arena.Decide
	}{{"no answer mirror", empty, empty}, {"starter mirror", starter, starter}, {"starter vs no answer", starter, empty}} {
		r := tFirstRate(c.a, c.b, 800, 70)
		t.Logf("%s: the team that starts T wins %.1f%%", c.name, 100*r)
		if r < 0.45 || r > 0.55 {
			t.Errorf("%s: T-first wins %.1f%%, should be ~50%%", c.name, 100*r)
		}
	}
	if r := matchRate(starter, starter, 600, 80); r < 0.45 || r > 0.55 {
		t.Errorf("starter mirror: %.1f%%", 100*r)
	}
}
