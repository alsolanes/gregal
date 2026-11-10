package football

import (
	"testing"
)

func testWinds(n int) []Wind {
	names := []string{"Tramuntana", "Gregal", "Llevant", "Xaloc", "Migjorn", "Garbí", "Ponent", "Mestral"}
	var out []Wind
	for i := 0; i < n; i++ {
		out = append(out, Wind{ID: "w" + names[i%len(names)], Name: names[i%len(names)], Role: []string{"music", "organisation", "sports", "news"}[i%4]})
	}
	return out
}

func TestLeagueCompletesTwoFullTeams(t *testing.T) {
	lg := NewLeague(testWinds(5), "ca", 7)
	seen := map[string]bool{}
	winds := 0
	for _, team := range lg.Teams {
		if len(team.Players) != RosterSize() || team.Name == "" || team.Coach == "" {
			t.Fatalf("equip incomplet: %+v", team)
		}
		count := map[string]int{}
		nums := map[int]bool{}
		for _, p := range team.Players {
			count[p.Pos]++
			if seen[p.ID] || nums[p.Number] {
				t.Fatalf("id o dorsal repetit: %+v", p)
			}
			seen[p.ID], nums[p.Number] = true, true
			if p.WindID != "" {
				winds++
			}
		}
		for _, rs := range RosterShape {
			if count[rs.Pos] != rs.N {
				t.Fatalf("%s: %d %s, en calen %d", team.Name, count[rs.Pos], rs.Pos, rs.N)
			}
		}
	}
	if winds != 5 {
		t.Fatalf("els 5 vents havien de jugar: %d", winds)
	}
}

// Calibratge: centenars de partits entre equips iguals han de donar números
// de la NFL.
func TestEngineLooksLikeTheNFL(t *testing.T) {
	const games = 400
	var pts, plays, runs, runYds, att, comp, passYds, sacks, ints, punts, fumbles, ties, ot, fgA, fgM int
	for i := 0; i < games; i++ {
		lg := NewLeague(testWinds(6), "en", int64(i))
		g := NewGame(lg.Teams[0], lg.Teams[1], int64(1000+i), "en")
		g.SimAll()
		if !g.Over || len(g.Plays) < 80 {
			t.Fatalf("partit %d sense acabar bé: %d jugades", i, len(g.Plays))
		}
		pts += g.Score[0] + g.Score[1]
		if g.Score[0] == g.Score[1] {
			ties++
		}
		for _, p := range g.Plays {
			if p.Quarter >= 5 {
				ot++
				break
			}
		}
		for _, p := range g.Plays {
			switch p.Kind {
			case "run", "pass", "sack", "scramble":
				plays++
			}
			switch p.Kind {
			case "run":
				runs++
				runYds += p.Yards
				if p.Fumble {
					fumbles++
				}
			case "pass":
				if p.Penalty == "" {
					att++
				}
				if p.Complete {
					comp++
					passYds += p.Yards
				}
				if p.Intercept {
					ints++
				}
			case "sack":
				sacks++
			case "punt":
				punts++
			case "fg":
				fgA++
				if p.Good {
					fgM++
				}
			}
		}
	}
	per := func(v int) float64 { return float64(v) / float64(games*2) }
	ypc := float64(runYds) / float64(runs)
	compPct := float64(comp) / float64(att)
	ypa := float64(passYds) / float64(att)
	t.Logf("per equip: %.1f punts, %.1f jugades, %.1f curses a %.2f, %.1f passades %.1f%% a %.2f ia/intent, %.2f sacks, %.2f int, %.2f fumbles, %.1f punts; FG %.0f%%; empats %d, pròrrogues %d",
		per(pts), per(plays), per(runs), ypc, per(att), compPct*100, ypa, per(sacks), per(ints), per(fumbles), per(punts), float64(fgM)/float64(fgA)*100, ties, ot)
	check := func(name string, v, lo, hi float64) {
		if v < lo || v > hi {
			t.Errorf("%s = %.2f, fora de [%.2f, %.2f]", name, v, lo, hi)
		}
	}
	check("punts per equip", per(pts), 19, 25.5)
	check("jugades per equip", per(plays), 55, 72)
	check("iardes per cursa", ypc, 3.8, 4.8)
	check("% completades", compPct*100, 58, 70)
	check("iardes per intent", ypa, 6.2, 8)
	check("sacks per equip", per(sacks), 1.5, 3.5)
	check("int per equip", per(ints), .5, 1.3)
	check("punts xutats per equip", per(punts), 3, 6)
	check("% gols de camp", float64(fgM)/float64(fgA)*100, 78, 92)
}

func TestSameSeedSameGame(t *testing.T) {
	lg := NewLeague(testWinds(4), "en", 3)
	a := NewGame(lg.Teams[0], lg.Teams[1], 99, "en")
	a.SimAll()
	lg2 := NewLeague(testWinds(4), "en", 3)
	b := NewGame(lg2.Teams[0], lg2.Teams[1], 99, "en")
	b.SimAll()
	if a.Score != b.Score || len(a.Plays) != len(b.Plays) {
		t.Fatalf("mateixa llavor, partit diferent: %v %v", a.Score, b.Score)
	}
}

func TestQuarterBoundaries(t *testing.T) {
	lg := NewLeague(testWinds(4), "en", 11)
	g := NewGame(lg.Teams[0], lg.Teams[1], 5, "en")
	q1 := g.SimQuarter()
	if len(q1) == 0 || q1[len(q1)-1].Quarter != 1 || g.Quarter != 2 {
		t.Fatalf("Q1: %d jugades, ara Q%d", len(q1), g.Quarter)
	}
	for !g.Over {
		g.SimQuarter()
	}
	last := g.Plays[len(g.Plays)-1]
	if last.Score != g.Score {
		t.Fatalf("el marcador de l'última jugada no quadra: %v vs %v", last.Score, g.Score)
	}
}
