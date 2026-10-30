package cs2

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
)

// Eines de calibratge (no s'executen per defecte).
//
//	CS2_CAL=1 go test -run TestTuneCalibrate -v ./internal/arena/cs2/
//	  recalibra planEdge perquè el motor doni la taula matchupWin de les
//	  regles i n'imprimeix el codi per enganxar-lo a round.go.
//	CS2_TUNE=1 go test -run TestTuneReport -v ./internal/arena/cs2/
//	  imprimeix les xifres d'equip, utilitat i ritme que citen les regles.

func logit(p float64) float64 {
	p = math.Max(0.01, math.Min(0.99, p))
	return math.Log(p / (1 - p))
}

func TestTuneCalibrate(t *testing.T) {
	if os.Getenv("CS2_CAL") == "" {
		t.Skip("CS2_CAL not set")
	}
	n := 3000
	for it := 0; it < 7; it++ {
		m := measureMatrix(n)
		worst := 0.0
		for i := range m {
			for j := range m[i] {
				tg := float64(matchupWin[i][j]) / 100
				worst = math.Max(worst, math.Abs(m[i][j]-tg))
				planEdge[i][j] += 0.6 * (logit(tg) - logit(m[i][j]))
			}
		}
		t.Logf("iter %d: worst error %.1f%%", it, 100*worst)
	}
	var b strings.Builder
	b.WriteString("var planEdge = [9][6]float64{\n")
	for i := range planEdge {
		b.WriteString("\t{")
		for j := range planEdge[i] {
			if j > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%.2f", planEdge[i][j])
		}
		fmt.Fprintf(&b, "}, // %s\n", stratVals[i])
	}
	b.WriteString("}\n")
	t.Log("\n" + b.String())
}

func TestTuneReport(t *testing.T) {
	if os.Getenv("CS2_TUNE") == "" {
		t.Skip("CS2_TUNE not set")
	}
	n := 3000
	full := func(st, u, p string) tDec { return tDec{"full", st, u, p, true} }
	ct := func(se, u string) ctDec { return ctDec{"full", se, u, true} }
	for _, mny := range []int{2500, 3000, 3400} {
		a := roundRate(tDec{"force", "default_mid", "full", "default", false}, ct("standard_2_1_2", "normal"), mny, 10000, n)
		b := roundRate(full("default_mid", "full", "default"), ctDec{"force", "standard_2_1_2", "normal", false}, 10000, mny, n)
		h := roundRate(tDec{"half", "default_mid", "full", "default", false}, ct("standard_2_1_2", "normal"), mny, 10000, n)
		h2 := roundRate(full("default_mid", "full", "default"), ctDec{"half", "standard_2_1_2", "normal", false}, 10000, mny, n)
		t.Logf("$%d each vs a full buy: force T %.0f%% CT %.0f%%; half T %.0f%% CT %.0f%%", mny, 100*a, 100*(1-b), 100*h, 100*(1-h2))
	}
	e := roundRate(full("default_mid", "full", "default"), ct("standard_2_1_2", "normal"), 10000, 10000, n)
	eT := roundRate(tDec{"eco", "default_mid", "none", "default", false}, ct("standard_2_1_2", "normal"), 10000, 10000, n)
	eC := roundRate(full("default_mid", "full", "default"), ctDec{"eco", "standard_2_1_2", "save", false}, 10000, 10000, n)
	rf := roundRate(tDec{"force", "default_mid", "full", "default", true}, ct("standard_2_1_2", "normal"), 10000, 10000, n)
	rc := roundRate(full("default_mid", "full", "default"), ctDec{"force", "standard_2_1_2", "normal", true}, 10000, 10000, n)
	aT := roundRate(tDec{"full", "default_mid", "full", "default", false}, ct("standard_2_1_2", "normal"), 10000, 10000, n)
	aC := roundRate(full("default_mid", "full", "default"), ctDec{"full", "standard_2_1_2", "normal", false}, 10000, 10000, n)
	t.Logf("full vs full: T %.0f%%; eco vs full: T %.0f%% CT %.0f%%; rich force vs full: T %.0f%% CT %.0f%%; without AWP: T %.0f%% CT %.0f%%",
		100*e, 100*eT, 100*(1-eC), 100*rf, 100*(1-rc), 100*aT, 100*(1-aC))
	for _, st := range []string{"rush_b", "execute_a", "split_b", "default_mid", "fake_a_go_b"} {
		s := fmt.Sprintf("%-12s T utility:", st)
		for _, u := range tUtilVals {
			s += fmt.Sprintf(" %s %.0f", u, 100*roundRate(full(st, u, calPace(st)), ct("standard_2_1_2", "normal"), 10000, 10000, n))
		}
		for _, se := range []string{"standard_2_1_2", "stack_a", "aggressive_mid"} {
			s += " | vs " + se + ":"
			for _, u := range ctUtilVal {
				s += fmt.Sprintf(" %s %.0f", u, 100*roundRate(full(st, "full", calPace(st)), ct(se, u), 10000, 10000, n))
			}
			for _, p := range paceVals {
				s += fmt.Sprintf(" %s %.0f", p, 100*roundRate(full(st, "full", p), ct(se, "normal"), 10000, 10000, n))
			}
		}
		t.Log(s)
	}
}
