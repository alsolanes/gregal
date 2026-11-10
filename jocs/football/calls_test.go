package football

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
)

// scrimPlays són les crides d'atac de jugada de línia (sense xuts ni rellotge).
var scrimPlays = []string{"run_inside", "run_outside", "pass_short", "pass_medium", "pass_deep", "screen", "play_action"}

var evTeams = func() [][2]*Team {
	var out [][2]*Team
	for i := 0; i < 24; i++ {
		out = append(out, MirroredTeams(int64(31*i+5), [2]string{"A", "B"}))
	}
	return out
}()

// callEV mesura les iardes esperades d'una crida contra una defensa des d'una
// situació fixa (una pèrdua de pilota compta com -35 iardes) i la taxa
// d'èxit (40% de la distància al 1r down, 60% al 2n, tota al 3r i 4t).
func callEV(off, def string, n int, spot, down, dist int) (ev, success float64) {
	const workers = 8
	type acc struct {
		sum float64
		ok  int
	}
	res := make(chan acc, workers)
	for w := 0; w < workers; w++ {
		go func(w int) {
			var a acc
			for i := w; i < n; i += workers {
				teams := evTeams[i%len(evTeams)]
				g := NewGame(teams[0], teams[1], int64(7919*i+17), "en")
				g.External = true
				g.kickNext = false
				g.Poss, g.Spot, g.Down, g.Dist = i%2, spot, down, dist
				g.SetCalls(OffCall{Play: off}, DefCall{Call: def})
				before := len(g.Plays)
				g.Step()
				p := g.Plays[before]
				y := float64(p.Yards)
				if p.Intercept || p.Fumble {
					y = -35
				}
				a.sum += y
				if playSuccess(p) {
					a.ok++
				}
			}
			res <- a
		}(w)
	}
	var t acc
	for w := 0; w < workers; w++ {
		a := <-res
		t.sum += a.sum
		t.ok += a.ok
	}
	return t.sum / float64(n), float64(t.ok) / float64(n)
}

func evMatrix(n int) map[string]map[string]float64 {
	m := map[string]map[string]float64{}
	for _, o := range scrimPlays {
		m[o] = map[string]float64{}
		for _, d := range DefenseCalls {
			ev, sr := callEV(o, d, n, 30, 1, 10)
			m[o][d] = ev
			m[o]["sr_"+d] = sr
		}
	}
	return m
}

func TestPrintMatrix(t *testing.T) {
	if os.Getenv("PRINT_MATRIX") == "" {
		t.Skip("PRINT_MATRIX=1 per veure la taula d'iardes esperades")
	}
	m := evMatrix(6000)
	var b strings.Builder
	fmt.Fprintf(&b, "\n%-12s", "")
	for _, d := range DefenseCalls {
		fmt.Fprintf(&b, "%9s", d)
	}
	b.WriteString("     avg\n")
	col := map[string]float64{}
	for _, o := range scrimPlays {
		fmt.Fprintf(&b, "%-12s", o)
		s := 0.0
		for _, d := range DefenseCalls {
			fmt.Fprintf(&b, "%9.2f", m[o][d])
			s += m[o][d]
			col[d] += m[o][d]
		}
		fmt.Fprintf(&b, "%8.2f   success%%:", s/float64(len(DefenseCalls)))
		for _, d := range DefenseCalls {
			fmt.Fprintf(&b, " %3.0f", m[o]["sr_"+d]*100)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "%-12s", "avg")
	for _, d := range DefenseCalls {
		fmt.Fprintf(&b, "%9.2f", col[d]/float64(len(scrimPlays)))
	}
	t.Log(b.String())
}

// TestMatchupCalibration busca, per a cada jugada i nota, la força x que dona
// les iardes esperades 5,2 + 1,3·nota. Només s'executa amb CALIBRATE=1.
func TestMatchupCalibration(t *testing.T) {
	if os.Getenv("CALIBRATE") == "" {
		t.Skip()
	}
	saved := map[string][5]float64{}
	for k, v := range matchLevel {
		saved[k] = v
	}
	defer func() { matchLevel = saved }()
	for _, p := range scrimPlays {
		var out [5]float64
		for r := -2; r <= 2; r++ {
			target := evCenter + evSlope*float64(r)
			lo, hi := -6.0, 6.0
			for it := 0; it < 14; it++ {
				x := (lo + hi) / 2
				matchLevel[p] = [5]float64{x, x, x, x, x}
				ev, _ := callEV(p, "base", 8000, 30, 1, 10)
				if ev < target {
					lo = x
				} else {
					hi = x
				}
			}
			out[r+2] = (lo + hi) / 2
		}
		t.Logf("%q: {%.2f, %.2f, %.2f, %.2f, %.2f},", p, out[0], out[1], out[2], out[3], out[4])
	}
}

// Iardes esperades d'una nota r de la taula: evCenter + evSlope·r.
const evCenter, evSlope = 5.0, 1.2

// La taula és pedra-paper-tisora: cada jugada té una defensa que l'atura i
// cada defensa una jugada que la castiga, i les iardes que dona el motor
// segueixen les notes de la taula (que és el que diuen les regles).
func TestMatchupTableIsRockPaperScissors(t *testing.T) {
	for _, o := range append(scrimPlays, "qb_sneak") {
		min := 9
		for _, d := range DefenseCalls {
			if r := Matchup[o][d]; r < min {
				min = r
			}
		}
		if min > -2 {
			t.Errorf("%s no té cap defensa que l'aturi (--)", o)
		}
	}
	for _, d := range DefenseCalls {
		max := -9
		for _, o := range scrimPlays {
			if r := Matchup[o][d]; r > max {
				max = r
			}
		}
		if max < 2 {
			t.Errorf("%s no té cap jugada que la castigui (++)", d)
		}
	}
	m := evMatrix(3000)
	for _, o := range scrimPlays {
		lo, hi := 99.0, -99.0
		for _, d := range DefenseCalls {
			ev, want := m[o][d], evCenter+evSlope*float64(Matchup[o][d])
			if ev < want-1 || ev > want+1 {
				t.Errorf("%s vs %s: %.2f iardes, la taula (%+d) en diu %.2f", o, d, ev, Matchup[o][d], want)
			}
			for _, d2 := range DefenseCalls {
				if Matchup[o][d]-Matchup[o][d2] >= 2 && ev <= m[o][d2] {
					t.Errorf("%s: vs %s (%+d) %.2f no supera vs %s (%+d) %.2f", o, d, Matchup[o][d], ev, d2, Matchup[o][d2], m[o][d2])
				}
			}
			lo, hi = math.Min(lo, ev), math.Max(hi, ev)
		}
		if lo > evCenter-1.5 || hi < evCenter+1.5 {
			t.Errorf("%s: de %.2f a %.2f, sense prou contrast", o, lo, hi)
		}
	}
	for _, d := range DefenseCalls {
		best := -99.0
		for _, o := range scrimPlays {
			best = math.Max(best, m[o][d])
		}
		if best < evCenter+1.5 {
			t.Errorf("cap jugada castiga %s: com a màxim %.2f", d, best)
		}
	}
	// El qb_sneak és per a 3a i 1: goal_line l'atura, prevent el regala.
	gl, _ := callEV("qb_sneak", "goal_line", 3000, 50, 3, 1)
	_, slGL := callEV("qb_sneak", "goal_line", 3000, 50, 3, 1)
	_, slPrev := callEV("qb_sneak", "prevent", 3000, 50, 3, 1)
	_, slBase := callEV("qb_sneak", "base", 3000, 50, 3, 1)
	if !(slGL < slBase && slBase < slPrev) {
		t.Errorf("qb_sneak a 3a i 1 (%.2f iardes vs goal_line): èxit goal_line %.0f%%, base %.0f%%, prevent %.0f%%", gl, slGL*100, slBase*100, slPrev*100)
	}
}

// Les ajudes de l'esquema diuen exactament la taula.
func TestSchemasFollowTheTable(t *testing.T) {
	for _, o := range append(scrimPlays, "qb_sneak") {
		if !strings.Contains(OffenseSchema, "- "+o+": ") || !strings.Contains(OffenseSchema, "vs defenses: "+MatchupLine(o)+".") {
			t.Errorf("l'esquema d'atac no diu la fila de %s", o)
		}
	}
	for _, d := range DefenseCalls {
		if !strings.Contains(DefenseSchema, DefenseLine(d)) {
			t.Errorf("l'esquema de defensa no diu la columna de %s", d)
		}
		for _, o := range append(scrimPlays, "qb_sneak") {
			r := Matchup[o][d]
			in := strings.Contains(DefenseLine(d), o+" ("+ratingSign[r]+")")
			if (r != 0) != in {
				t.Errorf("%s / %s: nota %+d, la línia de defensa diu %q", o, d, r, DefenseLine(d))
			}
		}
	}
}

// Sense resposta vàlida: pass_short i base; a la 4a, xutar (gol de camp si
// és a l'abast), mai jugar-se-la.
func TestFallbackCalls(t *testing.T) {
	teams := MirroredTeams(1, [2]string{"A", "B"})
	for _, c := range []struct {
		down, spot int
		play       string
	}{{1, 30, "pass_short"}, {3, 70, "pass_short"}, {4, 30, "punt"}, {4, 55, "punt"}, {4, 75, "field_goal"}, {4, 95, "field_goal"}} {
		g := NewGame(teams[0], teams[1], 1, "en")
		g.External = true
		g.Down, g.Dist, g.Spot = c.down, 3, c.spot
		g.SetCalls(OffCall{Play: "go for it!!"}, DefCall{})
		if g.callOff.Play != c.play || g.callDef.Call != "base" {
			t.Errorf("%da a la %d: %s / %s, calia %s / base", c.down, c.spot, g.callOff.Play, g.callDef.Call, c.play)
		}
	}
}

// Les RESULTS de la Situation diuen com ha anat cada crida.
func TestSituationShowsResults(t *testing.T) {
	teams := MirroredTeams(2, [2]string{"A", "B"})
	g := NewGame(teams[0], teams[1], 2, "en")
	g.External = true
	for i := 0; i < 400 && !g.Over; i++ {
		if g.NeedsCall() {
			g.SetCalls(OffCall{Play: []string{"run_inside", "pass_medium"}[i%2]}, DefCall{Call: "cover2"})
		}
		g.Step()
		if len(g.offHist[0]) >= 6 && len(g.offHist[1]) >= 6 && g.NeedsCall() {
			break
		}
	}
	off := g.Situation(g.Poss)
	def := g.Situation(1 - g.Poss)
	for _, want := range []string{"RESULTS of your offense", "by your call: ", "vs cover2 ", " yds/play, ", "% success", "Runs vs passes: runs "} {
		if !strings.Contains(off, want) {
			t.Errorf("la Situation d'atac no diu %q:\n%s", want, off)
		}
	}
	for _, want := range []string{"RESULTS of their offense against you", "by their call: ", "By your defense: cover2 ", "Their runs vs passes: "} {
		if !strings.Contains(def, want) {
			t.Errorf("la Situation de defensa no diu %q:\n%s", want, def)
		}
	}
}
