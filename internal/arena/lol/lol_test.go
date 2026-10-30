package lol

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"testing"

	"gregal/internal/arena"
)

type policy func(m *match, side int, p arena.Prompt) json.RawMessage

func js(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

// play juga un partit sencer amb dues polítiques (amb accés a l'estat).
func play(seed int64, a, b policy) (*match, []arena.Event) {
	m := newMatch(seed, "en", [2]string{"Alpha", "Beta"})
	pol := [2]policy{a, b}
	ev := arena.Play(m, [2]arena.Decide{
		func(s int, p arena.Prompt) json.RawMessage {
			if p.Schema == "" {
				return nil
			}
			return pol[0](m, s, p)
		},
		func(s int, p arena.Prompt) json.RawMessage {
			if p.Schema == "" {
				return nil
			}
			return pol[1](m, s, p)
		},
	})
	return m, ev
}

func defaults(m *match, side int, p arena.Prompt) json.RawMessage { return nil }

// ---- política ingènua: primer de la llista, sempre igual ----

func naive(m *match, side int, p arena.Prompt) json.RawMessage {
	switch {
	case m.stage < len(draftPlan):
		return js(map[string]any{})
	case m.stage == stLaning:
		return js(map[string]string{"top": "aggressive", "mid": "aggressive", "bot": "aggressive", "jungle": "gank_bot"})
	case m.stage == stObjectives:
		return js(map[string]string{"objective": "dragon"})
	case m.stage == stPlates:
		return js(map[string]string{"focus": "bot", "action": "plates"})
	}
	obj := "towers"
	for _, o := range m.objOptions() {
		if o == "dragon" {
			obj = "dragon"
		}
	}
	return js(map[string]string{"objective": obj, "stance": "engage"})
}

// ---- política adaptativa: el que faria un bon .md ----

func champs(ids []int) []*Champ {
	out := []*Champ{}
	for _, id := range ids {
		out = append(out, &pool[id])
	}
	return out
}

// draftValue: estimació (logarítmica) de la força d'una composició parcial.
func draftValue(me, en []int) float64 {
	cs, es := champs(me), champs(en)
	k := float64(len(cs))
	rels := []float64{}
	for _, s := range stances {
		rels = append(rels, stanceRaw(cs, s)/(stanceAvg[s]*k/5))
	}
	sort.Float64s(rels)
	val := 0.12*(clampf(rels[4], 0.3, 1.7)-1) + 0.04*(clampf(rels[3], 0.3, 1.5)-1)
	t := sumTraits(cs)
	ap := 0
	for _, c := range cs {
		if c.Dmg == "AP" {
			ap++
		}
	}
	left := 5 - k
	if t.F+2*left < 3 {
		val -= 0.08
	} else if t.F+2*left < 5 {
		val -= 0.03
	}
	if float64(ap)+left < 2 || k-float64(ap)+left < 2 {
		val -= 0.05
	}
	if len(es) > 0 {
		e := sumTraits(es)
		val += clampf(0.012*(t.Pi*5/k-0.6*(e.P+e.F)*5/float64(len(es))), -0.05, 0.05) * k / 5
		// Corba: millor que la rival a 30 min.
		my, their := 0.0, 0.0
		for _, c := range cs {
			my += curveAt(c.Curve, 30) / k
		}
		for _, c := range es {
			their += curveAt(c.Curve, 30) / float64(len(es))
		}
		val += 0.3 * (my - their) * k / 5
	}
	if t.E*5/k >= 5 && t.D*5/k >= 9 {
		val += 0.02
	}
	if t.P*5/k >= 5 {
		val += 0.015
	}
	return val
}

// noise: soroll determinista per variar els drafts entre partits.
func noise(m *match, side, id int) float64 {
	h := uint64(m.seed)*0x9E3779B97F4A7C15 ^ uint64(m.dec*131+side*17+id)*0xBF58476D1CE4E5B9
	h ^= h >> 29
	h *= 0x94D049BB133111EB
	h ^= h >> 32
	return float64(h%1000)/1000*0.03 - 0.015
}

func adaptive(m *match, side int, p arena.Prompt) json.RawMessage {
	i, j := side, 1-side
	switch {
	case m.stage < len(draftPlan):
		st := draftPlan[m.stage]
		if st.ban {
			type cv struct {
				id int
				v  float64
			}
			cands := []cv{}
			for id := range pool {
				if !m.taken[id] {
					cands = append(cands, cv{id, draftValue(append(append([]int{}, m.picks[j]...), id), m.picks[i]) + noise(m, i, id)})
				}
			}
			sort.Slice(cands, func(a, b int) bool { return cands[a].v > cands[b].v })
			names := []string{}
			for k := 0; k < 6 && k < len(cands); k++ {
				names = append(names, pool[cands[k].id].Name)
			}
			return js(map[string]any{"bans": names})
		}
		picks := []string{}
		mine := append([]int{}, m.picks[i]...)
		taken := map[int]bool{}
		for range st.sides {
			best, bv := -1, -1e9
			for id := range pool {
				if m.taken[id] || taken[id] || assignRoles(append(append([]int{}, mine...), id)) == nil {
					continue
				}
				v := draftValue(append(append([]int{}, mine...), id), m.picks[j]) + noise(m, i, id)
				if v > bv {
					best, bv = id, v
				}
			}
			if best < 0 {
				break
			}
			taken[best] = true
			mine = append(mine, best)
			picks = append(picks, pool[best].Name)
		}
		return js(map[string]any{"picks": picks})
	case m.stage == stLaning:
		out := map[string]string{}
		bestL, bestD := 0, -9.0
		for l, k := range roleLane {
			d := m.laneStr(i, l) - m.laneStr(j, l)
			out[k] = "even"
			if d > 0.3 {
				out[k] = "aggressive"
			} else if d < -0.3 {
				out[k] = "passive"
			}
			if d > bestD {
				bestL, bestD = l, d
			}
		}
		out["jungle"] = "gank_" + roleLane[bestL]
		if laneScore(m.line[i][1])-laneScore(m.line[j][1]) >= 1 {
			out["jungle"] = "invade"
		}
		return js(out)
	case m.stage == stObjectives:
		if m.earlyPower(i) >= m.earlyPower(j) {
			return js(map[string]string{"objective": "dragon"})
		}
		return js(map[string]string{"objective": "grubs"})
	case m.stage == stPlates:
		// On és més fort l'enemic? Probablement hi pressionarà: hi rodo.
		bestL, bestV := 1, -1e9
		worstL, worstV := 1, 1e9
		for l := 0; l < 3; l++ {
			v := m.laneAdv(i, l)
			if v > bestV {
				bestL, bestV = l, v
			}
			if v < worstV {
				worstL, worstV = l, v
			}
		}
		if worstV < -300 && -worstV > bestV {
			return js(map[string]string{"focus": roleLane[worstL], "action": "roam"})
		}
		if bestV > 200 {
			return js(map[string]string{"focus": roleLane[bestL], "action": "plates"})
		}
		return js(map[string]string{"focus": "mid", "action": "farm"})
	}
	// Postura: contrarestar la més freqüent de l'enemic.
	pred := bestStance(m.fit[j])
	if h := m.stHist[j]; len(h) > 0 {
		cnt := map[string]int{}
		for k, s := range h {
			cnt[s] += 1 + k // les recents pesen més
		}
		bc := -1
		for _, s := range stances {
			if cnt[s] > bc {
				pred, bc = s, cnt[s]
			}
		}
	}
	stance, sv := "engage", -1e9
	for _, s := range stances {
		rel := m.fit[i][s]
		v := 0.15*(clampf(rel, 0.3, 1.7)-1) + 0.11*versus(s, pred)*clampf(rel, 0.3, 1)
		if v > sv {
			stance, sv = s, v
		}
	}
	opts := m.objOptions()
	has := func(o string) bool {
		for _, x := range opts {
			if x == o {
				return true
			}
		}
		return false
	}
	predObj := ""
	if h := m.objHist[j]; len(h) > 0 {
		cnt := map[string]int{}
		for _, o := range h {
			cnt[o]++
		}
		bc := 0
		for o, c := range cnt {
			if has(o) && c > bc {
				predObj, bc = o, c
			}
		}
	}
	predSt := pred
	var s2 [2]string
	s2[i], s2[j] = stance, predSt
	mg := m.margin(s2)
	if i == 1 {
		mg = -mg
	}
	if m.buffed(m.baronUntil[i]) || m.buffed(m.elderUntil[i]) {
		return js(map[string]string{"objective": "towers", "stance": stance})
	}
	prio := []string{"elder", "baron", "dragon", "herald"}
	if mg > 0.02 {
		if isNeutral(predObj) {
			return js(map[string]string{"objective": predObj, "stance": stance})
		}
		for _, o := range prio {
			if has(o) {
				return js(map[string]string{"objective": o, "stance": stance})
			}
		}
		return js(map[string]string{"objective": "towers", "stance": stance})
	}
	for _, o := range prio {
		if has(o) && o != predObj && !(o == "baron" && mg < -0.06) && !(o == "elder" && mg < -0.06) {
			return js(map[string]string{"objective": o, "stance": stance})
		}
	}
	if m.teamCurve(i, m.t/60+8)-m.teamCurve(i, m.t/60) > m.teamCurve(j, m.t/60+8)-m.teamCurve(j, m.t/60) {
		return js(map[string]string{"objective": "farm", "stance": stance})
	}
	return js(map[string]string{"objective": "towers", "stance": stance})
}

// ---- proves ----

type stats struct {
	n                                 int
	lens, kills, drag, barons, towers []float64
	comeback, behind15                int
}

func (s *stats) add(m *match, ev []arena.Event) {
	s.n++
	s.lens = append(s.lens, m.evT/60)
	s.kills = append(s.kills, float64(m.kills[0]+m.kills[1]))
	s.drag = append(s.drag, float64(len(m.dragons[0])+len(m.dragons[1])))
	s.barons = append(s.barons, float64(m.barons[0]+m.barons[1]))
	tw := 0
	for i := 0; i < 2; i++ {
		for l := 0; l < 3; l++ {
			tw += m.towersLost[i][l]
		}
		tw += m.nexusTowersLost[i]
	}
	s.towers = append(s.towers, float64(tw))
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

func mean(v []float64) float64 {
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}
func median(v []float64) float64 {
	c := append([]float64{}, v...)
	sort.Float64s(c)
	return c[len(c)/2]
}

func TestDefaultsRealistic(t *testing.T) {
	var s stats
	for seed := int64(1); seed <= 400; seed++ {
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
		s.add(m, ev)
	}
	t.Logf("defaults: length mean %.1f median %.1f min %.1f max %.1f | kills %.1f | dragons %.1f | barons %.2f | towers %.1f | comebacks %d/%d",
		mean(s.lens), median(s.lens), minF(s.lens), maxF(s.lens), mean(s.kills), mean(s.drag), mean(s.barons), mean(s.towers), s.comeback, s.behind15)
	checkRange(t, "median length", median(s.lens), 27, 35)
	checkRange(t, "mean kills", mean(s.kills), 16, 34)
	checkRange(t, "mean dragons", mean(s.drag), 2.5, 6.5)
	checkRange(t, "mean barons", mean(s.barons), 0.4, 2)
	checkRange(t, "mean towers", mean(s.towers), 8, 18)
}

func TestMixedRealism(t *testing.T) {
	var s stats
	for seed := int64(1); seed <= 400; seed++ {
		m, ev := play(seed, adaptive, adaptive)
		s.add(m, ev)
	}
	t.Logf("adaptive mirror: length mean %.1f median %.1f min %.1f max %.1f | kills %.1f | dragons %.1f | barons %.2f | towers %.1f | comebacks %d/%d",
		mean(s.lens), median(s.lens), minF(s.lens), maxF(s.lens), mean(s.kills), mean(s.drag), mean(s.barons), mean(s.towers), s.comeback, s.behind15)
	checkRange(t, "median length", median(s.lens), 26, 36)
	checkRange(t, "mean kills", mean(s.kills), 16, 34)
	cb := float64(s.comeback) / float64(s.behind15)
	checkRange(t, "comeback rate", cb, 0.1, 0.35)
}

func minF(v []float64) float64 {
	c := append([]float64{}, v...)
	sort.Float64s(c)
	return c[0]
}
func maxF(v []float64) float64 {
	c := append([]float64{}, v...)
	sort.Float64s(c)
	return c[len(c)-1]
}

func checkRange(t *testing.T, what string, v, lo, hi float64) {
	t.Helper()
	if v < lo || v > hi {
		t.Errorf("%s = %.2f, want [%.2f, %.2f]", what, v, lo, hi)
	}
}

func winRate(n int, a, b policy) float64 {
	wins := 0
	for seed := int64(1); seed <= int64(n); seed++ {
		// alterna qui és l'equip 0 (i el costat surt de la llavor)
		if seed%2 == 0 {
			m, _ := play(seed*7919, a, b)
			if m.winner == 0 {
				wins++
			}
		} else {
			m, _ := play(seed*7919, b, a)
			if m.winner == 1 {
				wins++
			}
		}
	}
	return float64(wins) / float64(n)
}

func TestPolicies(t *testing.T) {
	wr := winRate(300, adaptive, naive)
	t.Logf("adaptive vs naive: %.1f%%", wr*100)
	if wr < 0.65 {
		t.Errorf("adaptive should beat naive ≥65%%, got %.1f%%", wr*100)
	}
	if wr > 0.9 {
		t.Errorf("naive should still win >10%%, adaptive won %.1f%%", wr*100)
	}
	wd := winRate(300, adaptive, defaults)
	t.Logf("adaptive vs defaults: %.1f%%", wd*100)
	// Espill: per costat.
	blue := 0
	for seed := int64(1); seed <= 300; seed++ {
		m, _ := play(seed*31, adaptive, adaptive)
		if m.winner == m.blue {
			blue++
		}
	}
	t.Logf("adaptive mirror: blue side wins %.1f%%", float64(blue)/3)
	if blue < 110 || blue > 190 {
		t.Errorf("mirror blue side wins %d/300, want ≈50%%", blue)
	}
}

func TestDeterminism(t *testing.T) {
	_, a := play(42, adaptive, naive)
	_, b := play(42, adaptive, naive)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("same seed must give the same match")
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
		// Cap campió repetit.
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

func TestPromptsAndContract(t *testing.T) {
	g, ok := arena.Get("lol")
	if !ok {
		t.Fatal("not registered")
	}
	if len(g.Rules()) > 2600 {
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
