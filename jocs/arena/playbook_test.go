package arena_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gregal/jocs/arena"
	_ "gregal/jocs/arena/cs2"
	_ "gregal/jocs/arena/lol"
	_ "gregal/jocs/arena/nfl"
)

// Cada joc té una guia sencera: la graella quadra amb les opcions, cada opció
// té nom i explicació, els ids surten a les regles del motor (els que el model
// pot respondre) i contra cada opció del rival hi ha almenys una resposta que
// guanya.
func TestPlaybooks(t *testing.T) {
	for _, g := range arena.Games() {
		pb, ok := g.(arena.Playbooker)
		if !ok {
			t.Fatalf("%s: no playbook", g.ID())
		}
		secs := pb.Playbook().Sections
		if len(secs) == 0 {
			t.Fatalf("%s: no sections", g.ID())
		}
		for _, s := range secs {
			name := g.ID() + "/" + s.ID
			if s.Title == "" || s.Intro == "" || !strings.Contains(s.Rule, "{mine}") || !strings.Contains(s.Rule, "{theirs}") {
				t.Errorf("%s: incomplete section %+v", name, s)
			}
			if len(s.Grid) != len(s.Mine) {
				t.Fatalf("%s: %d rows for %d options", name, len(s.Grid), len(s.Mine))
			}
			for _, o := range append(append([]arena.PlayOption{}, s.Mine...), s.Theirs...) {
				if o.ID == "" || o.Label == "" || len(o.Plain) < 20 {
					t.Errorf("%s: option %+v needs id, label and a plain explanation", name, o)
				}
				if !strings.Contains(g.Rules()+g.Starter(), o.ID) {
					t.Errorf("%s: %q is not an engine value", name, o.ID)
				}
			}
			for i, row := range s.Grid {
				if len(row) != len(s.Theirs) {
					t.Fatalf("%s: row %d has %d cells for %d columns", name, i, len(row), len(s.Theirs))
				}
				for _, v := range row {
					if v < -2 || v > 2 {
						t.Errorf("%s: cell %d out of range", name, v)
					}
				}
			}
			if s.Values != nil && len(s.Values) != len(s.Grid) {
				t.Errorf("%s: values do not match the grid", name)
			}
			for j, th := range s.Theirs {
				best := -3
				for i := range s.Mine {
					best = max(best, s.Grid[i][j])
				}
				if best <= 0 {
					t.Errorf("%s: nothing beats %s", name, th.ID)
				}
			}
		}
	}
}

func TestGrade(t *testing.T) {
	th := [4]int{33, 45, 55, 64}
	for v, want := range map[int]int{25: -2, 35: -1, 45: 0, 50: 0, 55: 0, 58: 1, 64: 1, 65: 2, 75: 2} {
		if got := arena.Grade(v, th); got != want {
			t.Errorf("Grade(%d) = %d, want %d", v, got, want)
		}
	}
}

// Els duels i el cicle que la tele ensenya a «How it is played»
// (jocs/web/app/arena-views.js, COUNTERS) són certs al motor: cada
// guanyador guanya clarament (+2) al rival, i al LoL cada postura guanya les
// dues següents del cicle.
func TestExplainCounters(t *testing.T) {
	raw, err := os.ReadFile("../web/app/arena-views.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	src = src[strings.Index(src, "export const COUNTERS"):]
	grade := func(game, a, b string) (int, bool) {
		g, _ := arena.Get(game)
		for _, s := range g.(arena.Playbooker).Playbook().Sections {
			i, j := -1, -1
			for k, o := range s.Mine {
				if o.ID == a {
					i = k
				}
			}
			for k, o := range s.Theirs {
				if o.ID == b {
					j = k
				}
			}
			if i >= 0 && j >= 0 {
				return s.Grid[i][j], true
			}
		}
		return 0, false
	}
	duel := regexp.MustCompile(`\['(\w+)', '[^']+', '(\w+)', '[^']+', '[^']+'\]`)
	for _, game := range []string{"nfl", "cs2"} {
		line := src[strings.Index(src, game+": {"):]
		line = line[:strings.Index(line, "\n")]
		ms := duel.FindAllStringSubmatch(line, -1)
		if len(ms) != 3 {
			t.Fatalf("%s: %d duels", game, len(ms))
		}
		for _, m := range ms {
			if g, ok := grade(game, m[1], m[2]); !ok || g != 2 {
				t.Errorf("%s: %s vs %s is %d (found %v), want +2", game, m[1], m[2], g, ok)
			}
		}
	}
	line := src[strings.Index(src, "lol: {"):]
	line = line[:strings.Index(line, "\n")]
	var cyc []string
	for _, m := range regexp.MustCompile(`\['(\w+)', '[^']+'\]`).FindAllStringSubmatch(line, -1) {
		cyc = append(cyc, m[1])
	}
	if len(cyc) != 5 {
		t.Fatalf("lol cycle %v", cyc)
	}
	for i, a := range cyc {
		for k := 1; k <= 4; k++ {
			g, ok := grade("lol", a, cyc[(i+k)%5])
			if want := map[bool]int{true: 2, false: -2}[k <= 2]; !ok || g != want {
				t.Errorf("lol: %s vs %s is %d, want %d", a, cyc[(i+k)%5], g, want)
			}
		}
	}
}
