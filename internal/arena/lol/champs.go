package lol

import (
	"math"
	"math/rand"
	"strings"
)

// Rols en l'ordre de la plantilla.
var roleNames = [5]string{"top", "jungle", "mid", "adc", "support"}

// Champ és un campió inventat amb el seu arquetip. Els trets van de 0 a 3.
type Champ struct {
	Name   string   `json:"name"`
	Roles  []string `json:"roles"`
	Class  string   `json:"class"`
	Dmg    string   `json:"dmg"`
	Curve  string   `json:"curve"`
	Engage int      `json:"engage"`
	Peel   int      `json:"peel"`
	Poke   int      `json:"poke"`
	Wave   int      `json:"waveclear"`
	Split  int      `json:"splitpush"`
	Pick   int      `json:"pick"`
	Obj    int      `json:"objective"`
	Front  int      `json:"frontline"`
	DPS    int      `json:"dps"`
	Lane   int      `json:"lane"`
}

func ch(name, roles, class, dmg, curve string, t ...int) Champ {
	return Champ{Name: name, Roles: strings.Split(roles, "/"), Class: class, Dmg: dmg, Curve: curve,
		Engage: t[0], Peel: t[1], Poke: t[2], Wave: t[3], Split: t[4], Pick: t[5], Obj: t[6], Front: t[7], DPS: t[8], Lane: t[9]}
}

// Trets: engage, peel, poke, wave, split, pick, obj, front, dps, lane.
var pool = []Champ{
	ch("Ironwall", "top/support", "tank", "AP", "mid", 3, 2, 0, 1, 0, 1, 1, 3, 1, 1),
	ch("Gravelord", "top/jungle", "bruiser", "AD", "early", 1, 0, 0, 1, 2, 1, 2, 2, 2, 3),
	ch("Thornmantle", "top", "tank", "AP", "mid", 1, 3, 0, 1, 0, 0, 1, 3, 1, 2),
	ch("Bladewarden", "top", "bruiser", "AD", "late", 0, 0, 0, 1, 3, 1, 1, 1, 3, 1),
	ch("Rageborn", "top", "bruiser", "AD", "early", 2, 0, 0, 1, 2, 1, 1, 2, 2, 3),
	ch("Frostbulwark", "top/jungle", "tank", "AP", "mid", 3, 1, 0, 1, 0, 1, 2, 3, 1, 1),
	ch("Sandreaver", "top/mid", "mage", "AP", "mid", 0, 0, 3, 3, 1, 0, 1, 0, 2, 3),
	ch("Ashduelist", "top/mid", "bruiser", "AD", "mid", 1, 0, 0, 2, 3, 2, 0, 1, 2, 2),

	ch("Nightfang", "jungle", "assassin", "AD", "early", 1, 0, 0, 0, 1, 3, 1, 0, 1, 3),
	ch("Wildroar", "jungle", "bruiser", "AD", "early", 2, 0, 0, 1, 1, 2, 3, 2, 2, 3),
	ch("Stormhoof", "jungle/support", "tank", "AP", "mid", 3, 2, 0, 1, 0, 1, 2, 3, 1, 2),
	ch("Voidstalker", "jungle", "assassin", "AD", "late", 1, 0, 0, 1, 2, 3, 2, 0, 2, 1),
	ch("Briarqueen", "jungle/mid", "mage", "AP", "mid", 1, 1, 2, 3, 0, 1, 3, 0, 3, 2),
	ch("Ashclaw", "jungle", "bruiser", "AD", "mid", 2, 1, 0, 1, 2, 2, 2, 2, 2, 2),
	ch("Moonhunter", "jungle/adc", "marksman", "AD", "late", 0, 0, 1, 2, 1, 1, 3, 0, 3, 1),
	ch("Granitebeast", "jungle", "tank", "AP", "early", 3, 2, 0, 1, 0, 2, 2, 3, 1, 2),

	ch("Emberlance", "mid/support", "mage", "AP", "mid", 0, 0, 3, 3, 0, 1, 1, 0, 2, 2),
	ch("Shadowveil", "mid", "assassin", "AP", "early", 1, 0, 0, 1, 1, 3, 0, 0, 1, 3),
	ch("Starweaver", "mid", "mage", "AP", "late", 1, 1, 1, 3, 0, 0, 1, 0, 3, 1),
	ch("Galecutter", "mid/top", "bruiser", "AD", "late", 1, 0, 0, 2, 3, 2, 0, 1, 3, 1),
	ch("Hexwright", "mid", "mage", "AP", "late", 0, 0, 3, 2, 0, 1, 1, 0, 2, 1),
	ch("Rimeoracle", "mid/support", "mage", "AP", "mid", 2, 2, 1, 2, 0, 1, 1, 1, 2, 2),
	ch("Duskdancer", "mid", "assassin", "AD", "early", 1, 0, 0, 1, 2, 3, 0, 0, 1, 3),
	ch("Pyrelord", "mid", "mage", "AP", "early", 1, 0, 2, 3, 0, 1, 1, 0, 2, 3),

	ch("Swiftarrow", "adc", "marksman", "AD", "early", 0, 0, 1, 1, 0, 0, 1, 0, 2, 3),
	ch("Longshot", "adc", "marksman", "AD", "mid", 0, 0, 3, 2, 0, 1, 2, 0, 2, 2),
	ch("Ironsights", "adc", "marksman", "AD", "late", 0, 0, 0, 2, 1, 0, 2, 0, 3, 1),
	ch("Venomdart", "adc/top", "marksman", "AP", "mid", 0, 0, 2, 2, 1, 0, 2, 0, 2, 2),
	ch("Stormcaller", "adc", "marksman", "AD", "late", 1, 0, 0, 2, 0, 0, 1, 0, 3, 1),
	ch("Quickdraw", "adc", "marksman", "AD", "early", 1, 0, 0, 1, 0, 1, 1, 0, 2, 3),
	ch("Sunspear", "adc/mid", "marksman", "AD", "mid", 0, 0, 2, 3, 1, 0, 2, 0, 2, 2),
	ch("Mirrorshot", "adc", "marksman", "AP", "late", 0, 1, 1, 2, 0, 0, 2, 0, 3, 1),

	ch("Chainwarden", "support", "engage", "AP", "mid", 2, 1, 0, 0, 0, 3, 0, 1, 0, 2),
	ch("Lumenshield", "support", "enchanter", "AP", "late", 0, 3, 1, 0, 0, 0, 0, 0, 1, 1),
	ch("Boulderheart", "support", "engage", "AP", "mid", 3, 2, 0, 0, 0, 1, 0, 3, 0, 1),
	ch("Tidesinger", "support", "enchanter", "AP", "mid", 0, 2, 2, 1, 0, 0, 0, 0, 1, 2),
	ch("Ravenmaw", "support/mid", "mage", "AP", "early", 0, 0, 3, 2, 0, 1, 0, 0, 2, 3),
	ch("Dawnkeeper", "support", "enchanter", "AP", "late", 0, 3, 0, 0, 0, 0, 0, 0, 1, 1),
	ch("Ironhorn", "support", "engage", "AD", "early", 3, 1, 0, 0, 0, 2, 0, 2, 0, 3),
	ch("Mistweaver", "support", "enchanter", "AP", "mid", 0, 3, 1, 1, 0, 0, 0, 0, 1, 2),
}

func champIndex(name string) int {
	k := normName(name)
	if k == "" {
		return -1
	}
	for i, c := range pool {
		if normName(c.Name) == k {
			return i
		}
	}
	return -1
}

func normName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (c *Champ) hasRole(r string) bool {
	for _, x := range c.Roles {
		if x == r {
			return true
		}
	}
	return false
}

// assignAny: com assignRoles, però si no hi ha manera (pool esgotat) deixa
// jugar algú fora de rol.
func assignAny(ids []int) []int {
	if r := assignRoles(ids); r != nil {
		return r
	}
	out := make([]int, len(ids))
	used := [5]bool{}
	for i := range ids {
		out[i] = -1
		for _, ro := range pool[ids[i]].Roles {
			if ri := roleIdx(ro); !used[ri] {
				out[i], used[ri] = ri, true
				break
			}
		}
	}
	for i := range ids {
		if out[i] < 0 {
			for r := 0; r < 5; r++ {
				if !used[r] {
					out[i], used[r] = r, true
					break
				}
			}
		}
	}
	return out
}

// assignRoles reparteix els campions en rols diferents (preferint l'ordre
// de cada campió). Torna nil si no és possible.
func assignRoles(ids []int) []int {
	out := make([]int, len(ids))
	used := [5]bool{}
	var rec func(i int) bool
	rec = func(i int) bool {
		if i == len(ids) {
			return true
		}
		for _, r := range pool[ids[i]].Roles {
			ri := roleIdx(r)
			if ri < 0 || used[ri] {
				continue
			}
			used[ri] = true
			out[i] = ri
			if rec(i + 1) {
				return true
			}
			used[ri] = false
		}
		return false
	}
	if len(ids) > 5 || !rec(0) {
		return nil
	}
	return out
}

func roleIdx(r string) int {
	for i, x := range roleNames {
		if x == r {
			return i
		}
	}
	return -1
}

// ---- corba de poder ----

var curvePts = map[string][][2]float64{
	"early": {{0, 1.12}, {14, 1.1}, {22, 0.99}, {30, 0.91}, {40, 0.86}, {60, 0.84}},
	"mid":   {{0, 0.98}, {14, 1.04}, {22, 1.08}, {30, 1.0}, {40, 0.95}, {60, 0.93}},
	"late":  {{0, 0.9}, {14, 0.92}, {22, 0.99}, {30, 1.1}, {40, 1.17}, {60, 1.19}},
}

func curveAt(c string, min float64) float64 {
	pts := curvePts[c]
	if min <= pts[0][0] {
		return pts[0][1]
	}
	for i := 1; i < len(pts); i++ {
		if min <= pts[i][0] {
			a, b := pts[i-1], pts[i]
			k := (min - a[0]) / (b[0] - a[0])
			return a[1] + (b[1]-a[1])*k
		}
	}
	return pts[len(pts)-1][1]
}

// ---- postures de lluita ----

var stances = []string{"engage", "siege", "pick", "split", "protect"}

var beats = map[string][2]string{
	"engage":  {"siege", "pick"},
	"siege":   {"split", "protect"},
	"pick":    {"siege", "split"},
	"split":   {"engage", "protect"},
	"protect": {"engage", "pick"},
}

// versus: +1 si a guanya b, -1 si perd, 0 si és neutre.
func versus(a, b string) float64 {
	if x, ok := beats[a]; ok && (x[0] == b || x[1] == b) {
		return 1
	}
	if x, ok := beats[b]; ok && (x[0] == a || x[1] == a) {
		return -1
	}
	return 0
}

type traitSum struct{ E, P, Po, W, S, Pi, O, F, D, MaxS float64 }

func sumTraits(cs []*Champ) traitSum {
	var t traitSum
	for _, c := range cs {
		t.E += float64(c.Engage)
		t.P += float64(c.Peel)
		t.Po += float64(c.Poke)
		t.W += float64(c.Wave)
		t.S += float64(c.Split)
		t.Pi += float64(c.Pick)
		t.O += float64(c.Obj)
		t.F += float64(c.Front)
		t.D += float64(c.DPS)
		t.MaxS = math.Max(t.MaxS, float64(c.Split))
	}
	return t
}

func stanceRaw(cs []*Champ, s string) float64 {
	t := sumTraits(cs)
	switch s {
	case "engage":
		return 2*t.E + t.F + 0.5*t.D
	case "siege":
		return 2*t.Po + t.W + 0.3*t.D
	case "pick":
		return 2*t.Pi + 0.7*t.E + 0.3*t.Po
	case "split":
		return 3*t.MaxS + t.S + 0.5*t.W
	case "protect":
		return 2*t.P + t.D + 0.3*t.F
	}
	return 0
}

// Mitjana de cada postura en composicions a l'atzar (per normalitzar).
var stanceAvg = func() map[string]float64 {
	r := rand.New(rand.NewSource(7))
	byRole := [5][]int{}
	for i, c := range pool {
		for _, ro := range c.Roles {
			byRole[roleIdx(ro)] = append(byRole[roleIdx(ro)], i)
		}
	}
	sum := map[string]float64{}
	const n = 3000
	for k := 0; k < n; k++ {
		cs := []*Champ{}
		seen := map[int]bool{}
		for ri := 0; ri < 5; ri++ {
			for {
				id := byRole[ri][r.Intn(len(byRole[ri]))]
				if !seen[id] {
					seen[id] = true
					cs = append(cs, &pool[id])
					break
				}
			}
		}
		for _, s := range stances {
			sum[s] += stanceRaw(cs, s)
		}
	}
	for s := range sum {
		sum[s] /= n
	}
	return sum
}()

// stanceFit: com de bé executa la composició cada postura (1 = mitjana).
func stanceFit(cs []*Champ) map[string]float64 {
	out := map[string]float64{}
	for _, s := range stances {
		out[s] = stanceRaw(cs, s) / stanceAvg[s]
	}
	return out
}

func bestStance(fit map[string]float64) string {
	best := "engage"
	for _, s := range stances {
		if fit[s] > fit[best] {
			best = s
		}
	}
	return best
}

// compMod: modificador (logarítmic) de la composició contra la rival:
// primera línia, barreja de dany, amenaça d'assassins i sinergies.
func compMod(me, en []*Champ) float64 {
	t, e := sumTraits(me), sumTraits(en)
	mod := 0.0
	switch {
	case t.F < 3:
		mod -= 0.08
	case t.F < 5:
		mod -= 0.03
	}
	ap := 0
	for _, c := range me {
		if c.Dmg == "AP" {
			ap++
		}
	}
	switch ap {
	case 0, 5:
		mod -= 0.09
	case 1, 4:
		mod -= 0.035
	}
	// Assassins contra una rereguarda sense protecció.
	mod += clampf(0.012*(t.Pi-0.6*(e.P+e.F)), -0.05, 0.05)
	late := 0
	for _, c := range me {
		if c.Curve == "late" && c.DPS >= 3 {
			late++
		}
	}
	if t.E >= 5 && t.D >= 9 {
		mod += 0.03
	}
	if t.P >= 5 && late >= 1 {
		mod += 0.03
	}
	if t.Po >= 7 && t.W >= 9 {
		mod += 0.03
	}
	return mod
}

func laneScore(c *Champ) float64 {
	s := float64(c.Lane)
	switch c.Curve {
	case "early":
		s += 0.6
	case "late":
		s -= 0.5
	}
	return s
}

func curveCounts(cs []*Champ) (e, m, l int) {
	for _, c := range cs {
		switch c.Curve {
		case "early":
			e++
		case "mid":
			m++
		default:
			l++
		}
	}
	return
}

func clampf(v, a, b float64) float64 { return math.Max(a, math.Min(b, v)) }
func clampi(v, a, b int) int {
	if v < a {
		return a
	}
	if v > b {
		return b
	}
	return v
}

// compCurve: identitat de la corba de la composició, comparant la força
// mitjana al minut 15 i al 35: "early" (força ara, cal tancar la partida),
// "scaling" (millora tard, cal sobreviure) o "balanced".
func compCurve(cs []*Champ) string {
	a, b := 0.0, 0.0
	for _, c := range cs {
		a += curveAt(c.Curve, 15)
		b += curveAt(c.Curve, 35)
	}
	d := (b - a) / float64(len(cs))
	switch {
	case d >= 0.02:
		return "scaling"
	case d <= -0.06:
		return "early"
	}
	return "balanced"
}
