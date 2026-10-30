package cs2

import (
	"encoding/json"
	"math"
	"math/rand"
	"sort"
	"strings"

	"gregal/internal/arena"
)

const (
	roundTime  = 115
	bombTime   = 40
	runSpeed   = 4.0
	walkSpeed  = 2.7
	plantDur   = 3.2
	defuseKit  = 5.0
	defuseBare = 10.0
)

var formSigma = 0.55

// duelK: com pesa la diferència d'equip (arma, armilla) a cada duel.
const duelK = 3.2

// Contra un rival sense armilla / sense casc, cada bala val més.
const noArmorK, noHelmetK = 1.18, 1.08

// L'AWP: molt fort en duels llargs (sense fums), una mica millor que un rifle de prop.
const awpLong, awpShort = 0.18, 0.02

type wpt struct{ t, x, y float64 }

type unit struct {
	p      *Player
	side   int
	alive  bool
	saving bool
	wp     []wpt
	zone   string
	group  string // CT: A, B, mid, passive; T: hit, lurk, fake
	arrive float64
	stage  string
	kills  int
	idx    int
}

func (u *unit) pos(t float64) (float64, float64) {
	w := u.wp
	if len(w) == 0 {
		return 0, 0
	}
	if t <= w[0].t {
		return w[0].x, w[0].y
	}
	for i := 1; i < len(w); i++ {
		if t <= w[i].t {
			a, b := w[i-1], w[i]
			k := 0.0
			if b.t > a.t {
				k = (t - a.t) / (b.t - a.t)
			}
			return a.x + (b.x-a.x)*k, a.y + (b.y-a.y)*k
		}
	}
	l := w[len(w)-1]
	return l.x, l.y
}

func (u *unit) lastT() float64 { return u.wp[len(u.wp)-1].t }

// cut: talla la trajectòria a l'instant t (per a un canvi de pla).
func (u *unit) cut(t float64) {
	x, y := u.pos(t)
	var w []wpt
	for _, p := range u.wp {
		if p.t < t {
			w = append(w, p)
		}
	}
	u.wp = append(w, wpt{t, x, y})
}

type pev struct {
	t    float64
	seq  int
	kind string
	text string
	hl   bool
	data map[string]any
}

type rnd struct {
	m     *Match
	r     *rand.Rand
	num   int
	tTeam int
	td    tDec
	cd    ctDec
	T, CT []*unit
	evs   []pev
	seq   int
	tx    *texts

	site, fake          string
	hitStart            float64
	planted             bool
	plantT              float64
	planter, defuser    *unit
	carrier             *unit
	winSide             int
	reason              string
	endT                float64
	firstKill           bool
	clutch              [3]int // per banda: enemics vius quan s'ha quedat sol
	tSm, tFl, tMo, ctNd int
	U, CU               float64
	stackOn             bool
	lurkPick            bool
	midWins             int
	retake              bool
	posts               map[string]int
	tAWP, ctAWP         bool
	form                float64 // forma del dia: soroll comú a tots els duels de la ronda
	edge                float64 // avantatge del pla T contra la configuració CT (taula planEdge)
}

func (r *rnd) emit(t float64, kind, text string, hl bool, data map[string]any) {
	if r.m.quiet && kind != "defuse" {
		return
	}
	if data == nil {
		data = map[string]any{}
	}
	data["t"] = round1(t)
	r.seq++
	r.evs = append(r.evs, pev{t, r.seq, kind, text, hl, data})
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func (r *rnd) units() []*unit { return append(append([]*unit(nil), r.T...), r.CT...) }

func (r *rnd) teamOf(u *unit) int {
	if u.side == sideT {
		return r.tTeam
	}
	return 1 - r.tTeam
}

func spot(z string, rr *rand.Rand, spread float64) (float64, float64) {
	zz := zoneByID[z]
	return zz.X + (rr.Float64()-0.5)*zz.W*spread, zz.Y + (rr.Float64()-0.5)*zz.H*spread
}

// moveTo: mou la unitat per un camí de zones; torna l'hora d'arribada.
func (r *rnd) moveTo(u *unit, route []string, t0, speed float64, final *[2]float64) float64 {
	if !u.alive || len(route) == 0 {
		return t0
	}
	if t0 < u.lastT() {
		u.cut(t0)
	}
	x, y := u.pos(t0)
	pts := [][2]float64{{x, y}}
	for i, z := range route {
		if i == 0 && z == u.zone && len(route) > 1 {
			continue
		}
		zz := zoneByID[z]
		if i == len(route)-1 {
			if final != nil {
				pts = append(pts, *final)
			} else {
				sx, sy := spot(z, r.r, 0.7)
				pts = append(pts, [2]float64{sx, sy})
			}
		} else {
			pts = append(pts, [2]float64{zz.X + (r.r.Float64()-0.5)*3, zz.Y + (r.r.Float64()-0.5)*3})
		}
	}
	sp := speed * (0.94 + 0.06*u.p.Move)
	t := t0
	for i := 1; i < len(pts); i++ {
		t += dist(pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1]) / sp
		u.wp = append(u.wp, wpt{t, pts[i][0], pts[i][1]})
	}
	var pp [][2]float64
	for _, p := range pts {
		pp = append(pp, [2]float64{round1(p[0]), round1(p[1])})
	}
	to := route[len(route)-1]
	u.zone = to
	if r.m.quiet {
		return t
	}
	r.emit(t0, "move", r.tx.move(u.p.Handle, zoneByID[to].Name), false, map[string]any{
		"id": u.p.ID, "team": r.teamOf(u), "path": route, "pts": pp, "dur": round1(t - t0), "to": to,
	})
	return t
}

// ---- duels ----

func power(w weapon, armor int, long bool) float64 {
	p := w.Power
	if w.Class == "awp" {
		if long {
			p += awpLong
		} else {
			p += awpShort
		}
	}
	if armor == 0 && w.Class != "awp" {
		p *= noArmorK
	} else if armor == 1 && w.Class != "awp" {
		p *= noHelmetK
	}
	if armor == 2 && w.Class == "pistol" && w.ID != "deagle" {
		p *= 0.86
	}
	return p
}

func (r *rnd) aliveN(us []*unit) int {
	n := 0
	for _, u := range us {
		if u.alive {
			n++
		}
	}
	return n
}

func (r *rnd) active(us []*unit) []*unit {
	var o []*unit
	for _, u := range us {
		if u.alive && !u.saving {
			o = append(o, u)
		}
	}
	return o
}

func (r *rnd) strength(u *unit, foe *unit, long bool) float64 {
	s := u.p.Aim * power(u.p.weapon(u.side), foe.p.Armor, long)
	mine := r.T
	other := r.CT
	if u.side == sideCT {
		mine, other = r.CT, r.T
	}
	if r.aliveN(mine) == 1 && r.aliveN(other) >= 2 {
		s *= u.p.Clutch
	}
	return s
}

// duel: t (T) contra c (CT); mod a favor del T. Torna el guanyador.
func (r *rnd) duel(t, c *unit, mod float64, at float64, long bool, zoneID string, smoke bool) *unit {
	st, sc := r.strength(t, c, long), r.strength(c, t, long)
	lg := duelK*(st-sc) + mod + r.form + r.edge
	p := 1 / (1 + math.Exp(-lg))
	w, l := c, t
	if r.r.Float64() < p {
		w, l = t, c
	}
	r.kill(w, l, at, zoneID, smoke)
	return w
}

func (r *rnd) kill(k, v *unit, at float64, zoneID string, smoke bool) {
	r.killWith(k, v, at, zoneID, smoke, k.p.weapon(k.side), nil)
}

// killWith: una mort amb l'arma que l'ha fet. given: el cap (mode en directe,
// on el dany també el porta el navegador); nil, a l'atzar com sempre.
func (r *rnd) killWith(k, v *unit, at float64, zoneID string, smoke bool, w weapon, given *bool) {
	v.alive = false
	v.cut(at)
	var hs bool
	if given != nil {
		hs = *given
	} else {
		hs = r.r.Float64() < w.HS*(0.95+0.3*(k.p.Aim-0.9))
		k.p.Dmg += 100
		v.p.Dmg += r.r.Intn(45)
	}
	k.kills++
	k.p.K++
	v.p.D++
	if hs {
		k.p.HS++
	}
	first := !r.firstKill
	if first {
		r.firstKill = true
		k.p.FK++
	}
	k.p.addMoney(w.Reward)
	kx, ky := k.pos(at)
	vx, vy := v.pos(at)
	kt, vt := r.teamOf(k), r.teamOf(v)
	hl := k.kills == 5
	if !r.m.quiet {
		r.emit(at, "kill", r.tx.kill(k.p.Handle, v.p.Handle, w.Name, hs), hl, map[string]any{
			"killer": k.p.ID, "victim": v.p.ID, "kteam": kt, "vteam": vt, "weapon": w.ID, "weaponName": w.Name, "hs": hs,
			"smoke": smoke, "first": first, "kx": round1(kx), "ky": round1(ky), "vx": round1(vx), "vy": round1(vy), "zone": zoneID,
			"multi": k.kills,
		})
	}
	// Clutch: qui es queda sol contra 2 o més.
	for _, s := range []int{sideT, sideCT} {
		mine, other := r.T, r.CT
		if s == sideCT {
			mine, other = r.CT, r.T
		}
		if r.clutch[s] == 0 && r.aliveN(mine) == 1 && r.aliveN(other) >= 2 {
			r.clutch[s] = r.aliveN(other)
		}
	}
}

// ---- utilitat ----

func (r *rnd) throw(u *unit, kind, zoneID string, at float64) {
	if !u.alive || !u.p.take(kind) {
		return
	}
	x, y := spot(zoneID, r.r, 0.5)
	if kind == "molotov" && (zoneID == "a_site" || zoneID == "b_site") {
		x, y = plantSpot[zoneByID[zoneID].Site][0]+(r.r.Float64()-0.5)*6, plantSpot[zoneByID[zoneID].Site][1]+(r.r.Float64()-0.5)*4
	}
	if r.m.quiet {
		return
	}
	ux, uy := u.pos(at)
	dur := map[string]float64{"smoke": 18, "molotov": 7, "flash": 1.5, "he": 1}[kind]
	r.emit(at, "utility", r.tx.util(u.p.Handle, kind, zoneByID[zoneID].Name), false, map[string]any{
		"id": u.p.ID, "team": r.teamOf(u), "type": kind, "zone": zoneID, "x": round1(x), "y": round1(y), "fx": round1(ux), "fy": round1(uy), "dur": dur,
	})
}

// ---- configuració CT i pla T ----

// planEdge: avantatge (logit per duel, a favor del T) de cada estratègia T
// contra cada configuració CT, a més del que ja fa la simulació (quants CT
// defensen el lloc, rotacions, utilitat). Calibrat perquè la taula de la
// rules (rondes guanyades amb compra completa) sigui la que dona el motor.
var planEdge = [9][6]float64{
	{-0.43, -0.03, -0.16, 0.07, -0.20, -0.68}, // rush_a
	{-0.50, -0.12, 0.02, 0.16, -0.63, -0.21},  // rush_b
	{0.17, -0.30, 0.07, -0.36, -0.07, 0.20},   // execute_a
	{0.11, 0.10, -0.30, -0.34, 0.20, -0.07},   // execute_b
	{-0.03, -0.38, -0.01, -0.60, -0.13, 0.10}, // split_a
	{-0.08, -0.05, -0.37, -0.61, 0.07, -0.12}, // split_b
	{-0.06, 0.37, 0.26, -0.42, 0.17, 0.19},    // default_mid
	{-0.01, 0.16, -0.39, -0.24, 0.11, -0.07},  // fake_a_go_b
	{0.05, -0.43, 0.21, -0.24, -0.08, 0.08},   // fake_b_go_a
}

func edgeOf(strat, setup string) float64 {
	i, j := indexOf(stratVals, strat), indexOf(setupVals, setup)
	if i < 0 || j < 0 {
		return 0
	}
	return planEdge[i][j]
}

func indexOf(xs []string, s string) int {
	for i, x := range xs {
		if x == s {
			return i
		}
	}
	return -1
}

type post struct{ zone, group string }

var setups = map[string][5]post{
	"standard_2_1_2": {{"a_site", "A"}, {"jungle", "A"}, {"window", "mid"}, {"b_site", "B"}, {"short", "B"}},
	"stack_a":        {{"a_site", "A"}, {"ct", "A"}, {"jungle", "A"}, {"a_site", "A"}, {"b_site", "B"}},
	"stack_b":        {{"b_site", "B"}, {"market", "B"}, {"short", "B"}, {"b_site", "B"}, {"a_site", "A"}},
	"aggressive_mid": {{"mid", "mid"}, {"short", "mid"}, {"window", "mid"}, {"a_site", "A"}, {"b_site", "B"}},
	"retake_b":       {{"a_site", "A"}, {"ct", "A"}, {"window", "mid"}, {"jungle", "A"}, {"market", "passive"}},
	"retake_a":       {{"b_site", "B"}, {"market", "B"}, {"window", "mid"}, {"short", "B"}, {"ct", "passive"}},
}

// Ordre dels rols CT als llocs: l'AWPer sempre al lloc 2.
var ctRoleOrder = [5]int{2, 3, 1, 0, 4} // índex de post -> slot (Entry, Support, AWPer, IGL, Lurker)

var tAvoid = map[string]bool{"ct": true, "ct_spawn": true, "market": true, "window": true}

// tPath: camí T precalculat (és una funció pura de from/to).
func tPath(from, to string) []string { return tPathTable[[2]string{from, to}] }

var tPathTable = func() map[[2]string][]string {
	out := map[[2]string][]string{}
	for _, a := range zones {
		for _, b := range zones {
			out[[2]string{a.ID, b.ID}] = tPathCalc(a.ID, b.ID)
		}
	}
	return out
}()

func tPathCalc(from, to string) []string {
	if from == to {
		return []string{to}
	}
	// Dijkstra petit evitant zones CT i l'altre lloc.
	other := "b_site"
	if to == "b_site" {
		other = "a_site"
	}
	best := map[string]float64{from: 0}
	prev := map[string]string{}
	done := map[string]bool{}
	for {
		cur, bd := "", math.Inf(1)
		for k, v := range best {
			if !done[k] && v < bd || !done[k] && v == bd && k < cur {
				cur, bd = k, v
			}
		}
		if cur == "" || cur == to {
			break
		}
		done[cur] = true
		a := zoneByID[cur]
		for _, l := range links {
			n := ""
			if l[0] == cur {
				n = l[1]
			} else if l[1] == cur {
				n = l[0]
			}
			if n == "" || tAvoid[n] || n == other || (n == "jungle" && to == "b_site") {
				continue
			}
			b := zoneByID[n]
			nd := bd + dist(a.X, a.Y, b.X, b.Y)
			if v, ok := best[n]; !ok || nd < v {
				best[n] = nd
				prev[n] = cur
			}
		}
	}
	var p []string
	for c := to; ; c = prev[c] {
		p = append([]string{c}, p...)
		if c == from {
			return p
		}
		if _, ok := prev[c]; !ok {
			return path(from, to)
		}
	}
}

var ctAvoid = map[string]bool{"t_spawn": true, "a_main": true, "palace": true, "b_apps": true, "top_mid": true}

func ctPath(from, to string) []string { return ctPathTable[[2]string{from, to}] }

var ctPathTable = func() map[[2]string][]string {
	out := map[[2]string][]string{}
	for _, a := range zones {
		for _, b := range zones {
			out[[2]string{a.ID, b.ID}] = ctPathCalc(a.ID, b.ID)
		}
	}
	return out
}()

func ctPathCalc(from, to string) []string {
	p := path(from, to)
	for _, z := range p[1:] {
		if ctAvoid[z] && z != to {
			// via CT spawn
			a := path(from, "ct_spawn")
			b := path("ct_spawn", to)
			if len(a) > 0 && len(b) > 0 {
				return append(append([]string(nil), a...), b[1:]...)
			}
		}
	}
	return p
}

// tStage: zona d'espera de cada T (per rol) segons l'estratègia.
func tStages(strat string) (site, fake string, stage [5]string, group [5]string) {
	group = [5]string{"hit", "hit", "hit", "hit", "hit"}
	switch strat {
	case "rush_a":
		return "A", "", [5]string{"a_main", "palace", "a_main", "a_main", "palace"}, group
	case "rush_b":
		return "B", "", [5]string{"b_apps", "b_apps", "b_apps", "b_apps", "b_apps"}, group
	case "execute_a":
		group[4] = "lurk"
		return "A", "", [5]string{"a_main", "mid", "a_main", "palace", "b_apps"}, group
	case "execute_b":
		group[4] = "lurk"
		return "B", "", [5]string{"b_apps", "mid", "b_apps", "b_apps", "palace"}, group
	case "split_a":
		return "A", "", [5]string{"mid", "mid", "a_main", "palace", "a_main"}, group
	case "split_b":
		return "B", "", [5]string{"mid", "mid", "b_apps", "b_apps", "mid"}, group
	case "fake_a_go_b":
		group[3], group[4] = "fake", "fake"
		return "B", "A", [5]string{"b_apps", "b_apps", "b_apps", "mid", "a_main"}, group
	case "fake_b_go_a":
		group[3], group[4] = "fake", "fake"
		return "A", "B", [5]string{"a_main", "palace", "a_main", "mid", "b_apps"}, group
	}
	// default_mid: lloc decidit tard.
	group[4] = "lurk"
	return "", "", [5]string{"top_mid", "mid", "a_main", "b_apps", "palace"}, group
}

func siteZone(s string) string {
	if s == "A" {
		return "a_site"
	}
	return "b_site"
}

func other(s string) string {
	if s == "A" {
		return "B"
	}
	return "A"
}

// ---- la ronda ----

func (m *Match) playRound(td tDec, cd ctDec) []arena.Event {
	r := &rnd{m: m, r: m.rng, num: m.nextRound(), tTeam: m.tTeam, td: td, cd: cd, tx: textsFor(m.lang), posts: map[string]int{}}
	ct := 1 - r.tTeam
	pistol := m.isPistol()
	tps, cps := m.teamPlayers(r.tTeam), m.teamPlayers(ct)

	// Compra.
	tBuy := buy(tps, sideT, td.Buy, td.Utility, td.AWP)
	cBuy := buy(cps, sideCT, cd.Buy, cd.Utility, cd.AWP)
	tObs, cObs := observeBuy(tps, sideT), observeBuy(cps, sideCT)

	// Unitats.
	for i, p := range tps {
		x, y := spot("t_spawn", r.r, 0.6)
		r.T = append(r.T, &unit{p: p, side: sideT, alive: true, zone: "t_spawn", wp: []wpt{{0, x, y}}, idx: i})
	}
	for i, p := range cps {
		x, y := spot("ct_spawn", r.r, 0.6)
		r.CT = append(r.CT, &unit{p: p, side: sideCT, alive: true, zone: "ct_spawn", wp: []wpt{{0, x, y}}, idx: i})
	}
	r.carrier = r.T[r.r.Intn(4)]
	if !m.quiet {
		r.startEvents(td, cd, tBuy, cBuy, tObs, cObs, pistol)
	}
	r.tAWP = hasAWP(r.T)
	r.ctAWP = hasAWP(r.CT)

	r.simulate()
	return m.finishRound(r, tObs, cObs)
}

// startEvents: els esdeveniments d'inici de ronda i de compra (no en mode quiet).
func (r *rnd) startEvents(td tDec, cd ctDec, tBuy, cBuy []buyItem, tObs, cObs string, pistol bool) {
	m := r.m
	ct := 1 - r.tTeam
	var pl []map[string]any
	for _, u := range r.units() {
		pl = append(pl, map[string]any{"id": u.p.ID, "team": r.teamOf(u), "side": sideName(u.side), "x": round1(u.wp[0].x), "y": round1(u.wp[0].y),
			"weapon": u.p.weapon(u.side).ID, "armor": u.p.Armor, "kit": u.p.Kit, "money": u.p.Money, "nades": u.p.Nades})
	}
	r.emit(0, "round_start", r.tx.roundStart(r.num, m.names[r.tTeam], m.names[ct]), false, map[string]any{
		"round": r.num, "half": m.halfName(r.num), "tTeam": r.tTeam, "ctTeam": ct, "pistol": pistol, "players": pl, "bomb": r.carrier.p.ID,
		"money": [2]int{teamMoney(m.teamPlayers(0)), teamMoney(m.teamPlayers(1))}, "target": m.target(),
		"calls": map[string]any{"t": td, "ct": cd},
	})
	for _, b := range []struct {
		team  int
		side  int
		level string
		items []buyItem
	}{{r.tTeam, sideT, td.Buy, tBuy}, {ct, sideCT, cd.Buy, cBuy}} {
		spent := 0
		for _, it := range b.items {
			spent += it.Spent
		}
		obs := tObs
		if b.side == sideCT {
			obs = cObs
		}
		r.emit(0, "buy", r.tx.buy(m.names[b.team], b.level, spent), false, map[string]any{
			"team": b.team, "side": sideName(b.side), "level": b.level, "observed": obs, "spent": spent, "items": b.items,
			"total": teamMoney(m.teamPlayers(b.team)),
		})
	}
}

func hasAWP(us []*unit) bool {
	for _, u := range us {
		if u.p.Primary == "awp" {
			return true
		}
	}
	return false
}

func sideName(s int) string {
	if s == sideCT {
		return "CT"
	}
	return "T"
}

func observeBuy(ps []*Player, side int) string {
	v := 0
	for _, p := range ps {
		v += equipValue(p, side)
	}
	v /= len(ps)
	switch {
	case v >= 3300:
		return "full"
	case v >= 1900:
		return "force"
	case v >= 800:
		return "half"
	}
	return "eco"
}

func (r *rnd) rng() float64 { return r.r.Float64() }

func (r *rnd) simulate() {
	r.form = r.r.NormFloat64() * formSigma
	r.edge = edgeOf(r.td.Strategy, r.cd.Setup)
	td, cd := r.td, r.cd
	speedT := runSpeed
	if td.Pace == "slow" {
		speedT = walkSpeed
	}
	// CT: cap als seus llocs.
	sp := setups[cd.Setup]
	for pi, slot := range ctRoleOrder {
		u := r.CT[slot]
		u.group = sp[pi].group
		start := 0.3 + r.rng()*1.5
		u.arrive = r.moveTo(u, ctPath("ct_spawn", sp[pi].zone), start, runSpeed, nil)
		g := u.group
		if g == "passive" {
			g = map[string]string{"market": "B", "ct": "A"}[sp[pi].zone]
		}
		r.posts[g]++
	}
	// T: cap a les zones d'espera.
	site, fake, stage, group := tStages(td.Strategy)
	r.site, r.fake = site, fake
	rush := strings.HasPrefix(td.Strategy, "rush")
	for i, u := range r.T {
		u.group = group[i]
		u.stage = stage[i]
		if rush {
			continue
		}
		r.moveTo(u, tPath("t_spawn", stage[i]), 0.5+r.rng()*1.5, speedT, nil)
	}
	paceK := map[string]float64{"fast": 0.72, "default": 1, "slow": 1.3}[td.Pace]
	jit := func(s float64) float64 { return (r.rng() - 0.5) * 2 * s }
	switch {
	case rush:
		r.hitStart = 0.5
	case strings.HasPrefix(td.Strategy, "execute"):
		r.hitStart = 30*paceK + jit(6)
	case strings.HasPrefix(td.Strategy, "split"):
		r.hitStart = 35*paceK + jit(6)
	case strings.HasPrefix(td.Strategy, "fake"):
		r.hitStart = 34*paceK + jit(5)
	default:
		r.hitStart = 48*paceK + jit(8)
	}
	if r.hitStart > 92 {
		r.hitStart = 92
	}

	// Fase inicial: duels pel mig.
	if !rush {
		r.midFights()
	} else if cd.Setup == "aggressive_mid" && r.rng() < 0.3 {
		// la rush es topa amb un CT avançat
		tu, cu := r.pickT(func(u *unit) bool { return true }), r.pickCT(func(u *unit) bool { return u.group == "mid" })
		if tu != nil && cu != nil {
			r.duel(tu, cu, 0.1, 9+r.rng()*4, false, "top_mid", false)
		}
	}
	if r.over() {
		return
	}

	// default_mid: tria el lloc amb menys CT (amb informació imperfecta).
	if site == "" {
		a, b := 0, 0
		for _, u := range r.active(r.CT) {
			switch u.group {
			case "A":
				a++
			case "B":
				b++
			}
		}
		acc := map[string]float64{"fast": 0.6, "default": 0.7, "slow": 0.8}[td.Pace]
		if a < b || a == b && r.rng() < 0.5 {
			site = "A"
		} else {
			site = "B"
		}
		if r.rng() > acc {
			site = other(site)
		}
		r.site = site
	}
	sz := siteZone(site)

	// Finta: utilitat a l'altre lloc i reacció dels CT.
	if fake != "" {
		ft := r.hitStart - 14*paceK
		fz := map[string]string{"A": "a_main", "B": "b_apps"}[fake]
		fakers := 0
		for _, u := range r.active(r.T) {
			if u.group == "fake" {
				target := map[string]string{"A": "jungle", "B": "short"}[fake]
				if u.stage != "mid" {
					target = map[string]string{"A": "a_site", "B": "b_site"}[fake]
				}
				if u.p.count("smoke") > 0 {
					r.throw(u, "smoke", target, ft+r.rng()*2)
					fakers += 2
				}
				if u.p.count("flash") > 0 {
					r.throw(u, "flash", siteZone(fake), ft+2+r.rng()*2)
					fakers++
				}
				if u.p.count("molotov") > 0 {
					r.throw(u, "molotov", siteZone(fake), ft+3)
					fakers++
				}
			}
		}
		_ = fz
		sell := 0.45 + 0.1*float64(fakers)
		if sell > 0.95 {
			sell = 0.95
		}
		// CT de l'altre lloc i del mig que roten cap a la finta.
		for _, u := range r.active(r.CT) {
			p := 0.0
			switch {
			case u.group == "mid":
				p = 0.55
			case u.group == site && cd.Setup == "standard_2_1_2":
				p = 0.45
			case u.group == site:
				p = 0.12
			}
			if r.rng() < p*sell {
				u.group = "rot_" + fake
				r.moveTo(u, ctPath(u.zone, siteZone(fake)), ft+2+r.rng()*3, runSpeed, nil)
			}
		}
		// els fakers van cap al lloc real
		for _, u := range r.active(r.T) {
			if u.group == "fake" {
				u.group = "hit"
			}
		}
	}

	// Execució: arribada de cada T al lloc.
	paceUtil := map[string]float64{"fast": 0.75, "default": 1, "slow": 1.1}[td.Pace]
	if rush {
		paceUtil = 1 // les rushes sempre van a tota velocitat: el ritme no hi compta
	}
	stratK := 1.0
	switch {
	case rush:
		stratK = 0.5
	case strings.HasPrefix(td.Strategy, "split"):
		stratK = 0.95
	case td.Strategy == "default_mid":
		stratK = 0.8
	case fake != "":
		stratK = 0.75
	}
	// utilitat CT al lloc atacat (frena i castiga)
	delay := 0.0
	var defenders []*unit
	for _, u := range r.active(r.CT) {
		if u.group == site {
			defenders = append(defenders, u)
		}
	}
	r.stackOn = len(defenders) >= 3
	entry := map[string]string{"A": "a_main", "B": "b_apps"}[site]
	firstArr := r.hitStart + map[bool]float64{true: 15, false: 9}[rush]
	cu := 0.0
	for _, u := range defenders {
		for _, n := range []string{"molotov", "he", "flash", "smoke"} {
			if u.p.count(n) > 0 && r.rng() < 0.85 {
				at := firstArr - 2 - r.rng()*3
				if at < 3 {
					at = 3
				}
				r.throw(u, n, entry, at)
				r.ctNd++
				cu += map[string]float64{"molotov": 0.14, "he": 0.07, "flash": 0.06, "smoke": 0.06}[n]
				if n == "molotov" {
					delay += 2
				}
			}
		}
	}
	if rush {
		cu *= 1.7
	}
	r.CU = math.Min(cu, 0.75)

	// T: llença la utilitat i entra.
	var hitters []*unit
	for _, u := range r.active(r.T) {
		if u.group == "hit" {
			hitters = append(hitters, u)
		}
	}
	u := 0.0
	ua := 0.0
	smokeTargets := map[string][]string{"A": {"ct", "jungle", "window"}, "B": {"short", "market", "market"}}[site]
	si := 0
	for _, h := range hitters {
		ua += h.p.Util
		throwAt := r.hitStart + 4 + r.rng()*4
		if rush {
			throwAt = firstArr - 2
		}
		for h.p.count("smoke") > 0 && !rush {
			r.throw(h, "smoke", smokeTargets[si%len(smokeTargets)], throwAt)
			si++
			r.tSm++
			u += 0.11
		}
		for h.p.count("flash") > 0 {
			r.throw(h, "flash", sz, throwAt+1.5+r.rng())
			r.tFl++
			u += 0.05
		}
		if h.p.count("molotov") > 0 && !rush {
			r.throw(h, "molotov", sz, throwAt+1)
			r.tMo++
			u += 0.08
		}
	}
	if len(hitters) > 0 {
		u *= ua / float64(len(hitters))
	}
	r.U = 0.55 * (1 - math.Exp(-u*paceUtil*stratK/0.6))
	// Arribada sincronitzada: cadascú surt a temps d'entrar alhora.
	routes := map[*unit][]string{}
	travel := map[*unit]float64{}
	maxTr := 0.0
	for _, h := range hitters {
		if rush {
			routes[h] = append(append([]string(nil), tPath("t_spawn", h.stage)...), tPath(h.stage, sz)[1:]...)
		} else {
			routes[h] = tPath(h.zone, sz)
		}
		l := 0.0
		rt := routes[h]
		for i := 1; i < len(rt); i++ {
			a, b := zoneByID[rt[i-1]], zoneByID[rt[i]]
			l += dist(a.X, a.Y, b.X, b.Y)
		}
		travel[h] = l / (runSpeed * (0.94 + 0.06*h.p.Move))
		maxTr = math.Max(maxTr, travel[h])
	}
	for _, h := range hitters {
		route := routes[h]
		start := r.hitStart + math.Min(maxTr, 9) - travel[h] + r.rng()*2
		if rush {
			start = r.hitStart + maxTr - travel[h] + r.rng()*2
		}
		start = math.Max(start, 1)
		if !rush {
			start += 3
		}
		if h.p.Role == "Entry" {
			start -= 1
		}
		if len(route) > 2 && delay > 0 {
			mid := r.moveTo(h, route[:len(route)-1], start, runSpeed, nil)
			h.arrive = r.moveTo(h, route[len(route)-2:], mid+delay, runSpeed, nil)
		} else {
			h.arrive = r.moveTo(h, route, start, runSpeed, nil)
		}
	}
	contact := math.Inf(1)
	for _, h := range hitters {
		contact = math.Min(contact, h.arrive)
	}
	if math.IsInf(contact, 1) {
		contact = r.hitStart + 10
	}

	// Rotacions dels CT.
	retake := (cd.Setup == "retake_a" && site == "A") || (cd.Setup == "retake_b" && site == "B")
	r.retake = retake
	farGo := 0
	for _, c := range r.active(r.CT) {
		switch {
		case c.group == site:
			continue
		case c.group == "passive" || retake:
			c.arrive = math.Inf(1)
		case c.group == "mid" || strings.HasPrefix(c.group, "rot_"):
			c.arrive = r.moveTo(c, ctPath(c.zone, sz), contact-3+r.rng()*3, runSpeed, nil)
		default:
			if !r.stackOn {
				c.arrive = r.moveTo(c, ctPath(c.zone, sz), contact+1+4*float64(farGo)+r.rng()*3, runSpeed, nil)
				farGo++
			} else {
				c.arrive = math.Inf(1)
			}
		}
	}
	// Lurker: talla una rotació o arriba tard pel flanc.
	for _, l := range r.active(r.T) {
		if l.group != "lurk" {
			continue
		}
		if zoneByID[l.stage].Site != site && td.Pace != "fast" && r.rng() < 0.45 {
			if c := r.pickCT(func(c *unit) bool { return c.group == other(site) }); c != nil {
				at := contact + 2 + r.rng()*3
				lz := l.zone
				w := r.duel(l, c, 0.4, at, false, lz, false)
				if w == l {
					r.lurkPick = true
				}
			}
		}
		if l.alive {
			l.group = "hit"
			l.arrive = r.moveTo(l, tPath(l.zone, sz), contact+3+r.rng()*4, runSpeed, nil)
		}
	}
	if r.over() {
		return
	}
	r.siteFight(sz, contact)
}

func (r *rnd) pickT(f func(*unit) bool) *unit  { return r.pick(r.T, f) }
func (r *rnd) pickCT(f func(*unit) bool) *unit { return r.pick(r.CT, f) }
func (r *rnd) pick(us []*unit, f func(*unit) bool) *unit {
	var c []*unit
	for _, u := range us {
		if u.alive && !u.saving && f(u) {
			c = append(c, u)
		}
	}
	if len(c) == 0 {
		return nil
	}
	return c[r.r.Intn(len(c))]
}

// midFights: contactes pel control del mig abans de l'execució.
func (r *rnd) midFights() {
	td, cd := r.td, r.cd
	inMid := func(u *unit) bool { return u.stage == "mid" || u.stage == "top_mid" }
	ctMid := func(u *unit) bool { return u.group == "mid" }
	tries := 2
	if cd.Setup == "aggressive_mid" {
		tries = 2
	}
	if td.Pace == "slow" {
		tries++
	}
	pc := map[string]float64{"fast": 0.35, "default": 0.65, "slow": 0.8}[td.Pace]
	if td.Strategy == "default_mid" {
		pc += 0.15
	}
	smoked := false
	for i := 0; i < tries; i++ {
		if r.rng() > pc {
			continue
		}
		tu := r.pickT(func(u *unit) bool { return inMid(u) && u.p.Primary == "awp" })
		if tu == nil || r.rng() < 0.4 {
			if x := r.pickT(inMid); x != nil {
				tu = x
			}
		}
		cu := r.pickCT(func(u *unit) bool { return ctMid(u) && u.p.Primary == "awp" })
		if cu == nil || r.rng() < 0.4 {
			if x := r.pickCT(ctMid); x != nil {
				cu = x
			}
		}
		if tu == nil || cu == nil {
			return
		}
		at := 14 + r.rng()*math.Max(2, r.hitStart-20)
		mod := 0.0
		if cd.Setup == "aggressive_mid" && td.Pace != "fast" {
			mod -= 0.6
		}
		if !smoked && td.Utility != "none" {
			if s := r.pickT(func(u *unit) bool { return inMid(u) && u.p.count("smoke") > 0 }); s != nil {
				r.throw(s, "smoke", "window", at-3)
				r.tSm++
				smoked = true
			}
		}
		long := !smoked
		if smoked {
			mod += 0.25
		}
		if tu.p.Primary == "awp" && td.Pace != "fast" {
			mod += 0.2
		}
		zz := "mid"
		if cu.zone == "short" || cu.zone == "window" {
			zz = cu.zone
		}
		if w := r.duel(tu, cu, mod, at, long, zz, smoked && r.rng() < 0.15); w == tu {
			r.midWins++
		} else {
			r.midWins--
		}
		if r.over() {
			return
		}
	}
}

func (r *rnd) over() bool { return r.winSide != 0 }

func (r *rnd) end(side int, reason string, t float64) {
	if r.winSide == 0 {
		r.winSide, r.reason, r.endT = side, reason, t
	}
}

// siteFight: el combat al lloc (abans i després de plantar) fins al final.
func (r *rnd) siteFight(sz string, t float64) {
	site := zoneByID[sz].Site
	bx, by := plantSpot[site][0], plantSpot[site][1]
	var lastKiller *unit
	nDuel := 0
	holderAdv := -0.6
	if r.stackOn {
		holderAdv += 0.15 // angles repetits: cada jugador de més val menys
	}
	if r.td.Strategy == "default_mid" {
		holderAdv -= 0.1
	}
	ctNadesLeft := func() int {
		n := 0
		for _, c := range r.active(r.CT) {
			n += len(c.p.Nades)
		}
		return n
	}
	tMolly := func() int {
		n := 0
		for _, u := range r.active(r.T) {
			n += u.p.count("molotov")
		}
		return n
	}
	for guard := 0; guard < 80; guard++ {
		ts, cs := r.active(r.T), r.active(r.CT)
		if r.aliveN(r.T) == 0 {
			if r.planted {
				r.defuse(t, bx, by)
			} else {
				r.end(sideCT, "elim", t)
			}
			return
		}
		if r.aliveN(r.CT) == 0 {
			r.end(sideT, "elim", t)
			return
		}
		deadline := float64(roundTime)
		if r.planted {
			deadline = r.plantT + bombTime
		}
		if t >= deadline {
			if r.planted {
				r.explode(deadline, bx, by)
			} else {
				r.end(sideCT, "time", deadline)
			}
			return
		}
		if len(ts) == 0 { // tots els T salven
			r.end(sideCT, "time", roundTime)
			return
		}
		if len(cs) == 0 { // els CT salven
			r.explode(deadline, bx, by)
			return
		}
		var tp, cp []*unit
		nextT, nextC := math.Inf(1), math.Inf(1)
		for _, u := range ts {
			if u.arrive <= t {
				tp = append(tp, u)
			} else {
				nextT = math.Min(nextT, u.arrive)
			}
		}
		for _, u := range cs {
			if u.arrive <= t {
				cp = append(cp, u)
			} else {
				nextC = math.Min(nextC, u.arrive)
			}
		}
		if r.planted && len(tp) == 0 {
			if len(cp) == 0 {
				t = math.Min(nextT, nextC)
				continue
			}
			// cap T al lloc: desactivar abans que arribin
			r.tryDefuse(t, nextT, bx, by)
			if r.over() {
				return
			}
			t = nextT
			continue
		}
		if !r.planted && len(cp) == 0 && len(tp) > 0 {
			if t+plantDur+1 > roundTime {
				r.end(sideCT, "time", roundTime)
				return
			}
			if nextC > t+plantDur+1 {
				r.plant(tp, t, sz, bx, by)
				for _, c := range r.active(r.CT) {
					if math.IsInf(c.arrive, 1) {
						from := c.zone
						c.arrive = r.moveTo(c, ctPath(from, sz), t+plantDur+0.5+r.rng()*2, runSpeed, nil)
					}
				}
				if r.retake {
					// arriben junts
					mx := 0.0
					for _, c := range r.active(r.CT) {
						mx = math.Max(mx, c.arrive)
					}
					for _, c := range r.active(r.CT) {
						c.arrive = mx
					}
				}
				t = t + plantDur
				r.ctSave(t)
				continue
			}
			t = nextC
			continue
		}
		if len(tp) == 0 || len(cp) == 0 {
			nx := math.Min(nextT, nextC)
			if math.IsInf(nx, 1) {
				if r.planted {
					r.explode(deadline, bx, by)
				} else {
					r.end(sideCT, "time", deadline)
				}
				return
			}
			t = math.Max(t, nx)
			continue
		}
		// Duel.
		var a, c *unit
		if lastKiller != nil && lastKiller.alive && r.rng() < 0.6 {
			if lastKiller.side == sideT {
				a = lastKiller
			} else {
				c = lastKiller
			}
		}
		if a == nil {
			a = tp[r.r.Intn(len(tp))]
			if nDuel == 0 {
				for _, x := range tp {
					if x.p.Role == "Entry" {
						a = x
					}
				}
			}
		}
		if c == nil {
			c = cp[r.r.Intn(len(cp))]
		}
		mod := 0.08 * float64(len(tp)-len(cp))
		mod = math.Max(-0.5, math.Min(0.5, mod))
		if lastKiller != nil && lastKiller.alive {
			if lastKiller == a {
				mod -= 0.5
			} else if lastKiller == c {
				mod += 0.5
			}
		}
		long := false
		if !r.planted {
			holding := c.arrive <= r.firstContact(tp)
			uk := 1.0
			if nDuel >= 2 {
				uk = 0.5
			}
			if holding {
				mod += holderAdv + (r.U-r.CU)*uk
				long = r.tSm == 0
				if strings.HasPrefix(r.td.Strategy, "rush") && !r.stackOn && r.cd.Utility != "heavy" {
					mod += 0.3
				}
				if strings.HasPrefix(r.td.Strategy, "split") {
					mod += 0.2
				}
			} else {
				mod += 0.1 + r.U*0.3
			}
			if r.td.Strategy == "default_mid" {
				mod += 0.1 * float64(r.midWins)
			}
		} else {
			mod += 0.45 + math.Min(0.3, 0.1*float64(tMolly())) - math.Min(0.2, 0.025*float64(ctNadesLeft()))
			if r.retake {
				mod -= 0.6
			}
		}
		at := t + 0.6 + r.rng()*2.4
		if nDuel == 0 {
			at = t + r.rng()*1.2
		}
		smoke := r.tSm > 0 && !r.planted && r.rng() < 0.07
		lastKiller = r.duel(a, c, mod, at, long, sz, smoke)
		nDuel++
		t = at
		if !r.planted {
			r.tSave(t)
		} else {
			r.ctSave(t)
		}
	}
	r.end(sideCT, "time", roundTime)
}

func (r *rnd) firstContact(tp []*unit) float64 {
	f := math.Inf(1)
	for _, u := range r.T {
		if u.group == "hit" && u.arrive > 0 {
			f = math.Min(f, u.arrive)
		}
	}
	return f
}

func (r *rnd) plant(tp []*unit, t float64, sz string, bx, by float64) {
	p := tp[0]
	for _, u := range tp {
		if u == r.carrier {
			p = u
		}
	}
	fin := [2]float64{bx + (r.rng()-0.5)*2, by + (r.rng()-0.5)*2}
	r.moveTo(p, []string{sz}, t, runSpeed, &fin)
	r.planted, r.plantT, r.planter = true, t+plantDur, p
	p.p.Plants++
	p.p.addMoney(plantReward)
	r.emit(r.plantT, "plant", r.tx.plant(p.p.Handle, zoneByID[sz].Site), false, map[string]any{
		"id": p.p.ID, "team": r.tTeam, "site": zoneByID[sz].Site, "x": round1(fin[0]), "y": round1(fin[1]), "start": round1(t), "explodeAt": round1(r.plantT + bombTime),
	})
}

// tSave: els T salven si ja no poden guanyar abans del temps.
func (r *rnd) tSave(t float64) {
	n, c := len(r.active(r.T)), len(r.active(r.CT))
	if n == 0 || r.planted {
		return
	}
	if (roundTime-t < 15 && n < c) || (n == 1 && c >= 3 && t < 80 && r.rng() < 0.25) {
		savers := r.active(r.T)
		for _, u := range savers {
			u.saving = true
			r.moveTo(u, tPath(u.zone, "t_spawn"), t+0.5, runSpeed, nil)
		}
		r.hunt(savers, r.CT, t, roundTime-1, -0.2)
	}
}

// hunt: els que salven poden caure mentre fugen (exit frags).
func (r *rnd) hunt(savers, hunters []*unit, t, limit, mod float64) {
	for _, s := range savers {
		if r.rng() > 0.35 {
			continue
		}
		h := r.pick(hunters, func(*unit) bool { return true })
		at := t + 2 + r.rng()*6
		if h == nil || at >= limit {
			continue
		}
		zz := s.zone
		if s.side == sideT {
			r.duel(s, h, mod, at, true, zz, false)
		} else {
			r.duel(h, s, mod, at, true, zz, false)
		}
	}
}

// ctSave: després de plantar, els CT salven si la recuperació és impossible.
func (r *rnd) ctSave(t float64) {
	cs := r.active(r.CT)
	n, c := len(r.active(r.T)), len(cs)
	if c == 0 || n == 0 {
		return
	}
	kit := false
	for _, u := range cs {
		kit = kit || u.p.Kit
	}
	need := defuseBare
	if kit {
		need = defuseKit
	}
	left := r.plantT + bombTime - t
	save := c <= n-3 || (c <= n-2 && r.rng() < 0.15) || left < need+2+2*float64(n)
	if c == 1 && n == 2 && !kit && r.rng() < 0.5 {
		save = true
	}
	if r.retake && c >= n-2 && left >= need+6 {
		save = false
	}
	if save {
		for _, u := range cs {
			u.saving = true
			r.moveTo(u, ctPath(u.zone, "ct_spawn"), t+0.5, runSpeed, nil)
		}
		r.hunt(cs, r.T, t, r.plantT+bombTime-1, 0.2)
	}
}

func (r *rnd) tryDefuse(t, nextT, bx, by float64) {
	cs := r.active(r.CT)
	var d *unit
	for _, u := range cs {
		if u.arrive <= t && (d == nil || u.p.Kit && !d.p.Kit) {
			d = u
		}
	}
	if d == nil {
		return
	}
	dur := defuseBare
	if d.p.Kit {
		dur = defuseKit
	}
	start := t + 1
	if start+dur > r.plantT+bombTime {
		return
	}
	if start+dur <= nextT {
		ninja := r.aliveN(r.T) > 0
		r.finishDefuse(d, start, dur, bx, by, ninja)
		return
	}
	// el T arriba mentre desactiva
	fin := [2]float64{bx + 1, by + 1}
	r.moveTo(d, []string{zoneByID[siteZone(r.site)].ID}, t, runSpeed, &fin)
	for _, u := range r.active(r.T) {
		if u.arrive == nextT {
			w := r.duel(u, d, 0.6, nextT, false, siteZone(r.site), false)
			if w == u {
				return
			}
		}
	}
}

func (r *rnd) defuse(t, bx, by float64) {
	var d *unit
	for _, u := range r.active(r.CT) {
		if d == nil || u.p.Kit && !d.p.Kit {
			d = u
		}
	}
	if d == nil {
		r.explode(r.plantT+bombTime, bx, by)
		return
	}
	dur := defuseBare
	if d.p.Kit {
		dur = defuseKit
	}
	x, y := d.pos(t)
	walk := dist(x, y, bx, by)/runSpeed + 0.5
	start := math.Max(t, d.arrive) + walk
	if start+dur > r.plantT+bombTime {
		r.explode(r.plantT+bombTime, bx, by)
		return
	}
	r.finishDefuse(d, start, dur, bx, by, false)
}

func (r *rnd) finishDefuse(d *unit, start, dur, bx, by float64, ninja bool) {
	fin := [2]float64{bx + 1, by + 1}
	if d.lastT() > start-0.5 {
		d.cut(start - 0.5)
	}
	r.moveTo(d, []string{siteZone(r.site)}, start-0.5, runSpeed*3, &fin)
	d.p.Defuses++
	d.p.addMoney(defuseReward)
	r.defuser = d
	r.emit(start+dur, "defuse", r.tx.defuse(d.p.Handle, ninja), ninja || r.plantT+bombTime-(start+dur) < 1, map[string]any{
		"id": d.p.ID, "team": 1 - r.tTeam, "start": round1(start), "dur": dur, "kit": d.p.Kit, "ninja": ninja, "x": round1(fin[0]), "y": round1(fin[1]),
		"left": round1(r.plantT + bombTime - (start + dur)),
	})
	r.end(sideCT, "defuse", start+dur)
}

func (r *rnd) explode(t, bx, by float64) {
	r.emit(t, "explode", r.tx.explode(), false, map[string]any{"site": r.site, "x": round1(bx), "y": round1(by)})
	r.end(sideT, "bomb", t)
}

// ---- tancament de la ronda ----

func (m *Match) finishRound(r *rnd, tObs, cObs string) []arena.Event {
	if r.winSide == 0 {
		r.end(sideCT, "time", roundTime)
	}
	ct := 1 - r.tTeam
	win := r.tTeam
	if r.winSide == sideCT {
		win = ct
	}
	lose := 1 - win
	m.round++
	m.score[win]++
	// Diners.
	reward := winElim
	if r.reason == "bomb" || r.reason == "defuse" {
		reward = winBomb
	}
	units := map[string]*unit{}
	for _, u := range r.units() {
		units[u.p.ID] = u
	}
	for _, p := range m.teamPlayers(win) {
		p.addMoney(reward)
	}
	lb := lossBonus(m.lossLvl[lose])
	for _, p := range m.teamPlayers(lose) {
		u := units[p.ID]
		if lose == r.tTeam && r.reason == "time" && u.alive {
			continue // T que sobreviuen al temps: res
		}
		v := lb
		if lose == r.tTeam && r.planted {
			v += plantBonus
		}
		p.addMoney(v)
	}
	m.lossLvl[lose] = min(m.lossLvl[lose]+1, 4)
	m.lossLvl[win] = max(m.lossLvl[win]-1, 0)
	// Equip: els morts ho perden tot; els vius ho conserven (menys la utilitat).
	var mvp *unit
	for _, u := range r.units() {
		if !u.alive {
			u.p.strip()
		}
		if u.kills >= 4 {
			u.p.MultiK++
		}
		if r.teamOf(u) == win && (mvp == nil || u.kills > mvp.kills) {
			mvp = u
		}
	}
	if r.defuser != nil && r.defuser.kills >= mvp.kills-1 {
		mvp = r.defuser
	} else if r.reason == "bomb" && r.planter != nil && r.planter.kills >= mvp.kills-1 {
		mvp = r.planter
	}
	// Destacats.
	hl := false
	var tags []string
	if m.pistolRound(r.num) {
		hl = true
		tags = append(tags, "pistol")
	}
	winSide := r.winSide
	if r.clutch[winSide] >= 2 {
		hl = true
		tags = append(tags, "clutch")
		for _, u := range r.units() {
			if u.alive && r.teamOf(u) == win {
				u.p.Clutches++
				mvp = u
			}
		}
	}
	for _, u := range r.units() {
		if u.kills == 5 {
			hl = true
			tags = append(tags, "ace")
		} else if u.kills == 4 {
			hl = true
			tags = append(tags, "4k")
		}
	}
	if (win == r.tTeam && tObs == "eco" && cObs == "full") || (win == ct && cObs == "eco" && tObs == "full") {
		hl = true
		tags = append(tags, "eco_win")
	}
	if r.defuser != nil && r.plantT+bombTime-r.endT < 1 {
		tags = append(tags, "last_second")
		hl = true
	}
	var ninjaTag bool
	for _, e := range r.evs {
		if e.kind == "defuse" && e.data["ninja"] == true {
			ninjaTag = true
		}
	}
	if ninjaTag {
		tags = append(tags, "ninja")
		hl = true
	}
	if !m.quiet {
		var money []map[string]any
		for _, u := range r.units() {
			money = append(money, map[string]any{"id": u.p.ID, "money": u.p.Money, "alive": u.alive, "kills": u.kills})
		}
		clutchN := r.clutch[winSide]
		r.emit(r.endT+0.01, "round_end", r.tx.roundEnd(m.names[win], r.reason, tags, clutchN), hl, map[string]any{
			"round": r.num, "winner": win, "side": sideName(winSide), "reason": r.reason, "mvp": mvp.p.ID, "tags": tags,
			"score": m.score, "players": money, "money": [2]int{teamMoney(m.teamPlayers(0)), teamMoney(m.teamPlayers(1))},
			"site": r.site, "planted": r.planted, "clutch": clutchN, "lossBonus": [2]int{lossBonus(m.lossLvl[0]), lossBonus(m.lossLvl[1])},
		})
	}
	// Historial.
	posts := map[string]int{}
	for k, v := range r.posts {
		posts[k] = v
	}
	site := r.site
	if r.hitStart > r.endT && !r.planted {
		site = ""
	}
	kills := [2]int{}
	for _, u := range r.units() {
		kills[r.teamOf(u)] += u.kills
	}
	if m.pistolRound(r.num) {
		tObs, cObs = "pistol", "pistol"
	}
	m.hist = append(m.hist, roundRec{Round: r.num, TTeam: r.tTeam, T: r.td, CT: r.cd, TObs: tObs, CTObs: cObs, Site: site, Planted: r.planted,
		Winner: win, Reason: r.reason, TSmokes: r.tSm, TFlashes: r.tFl, TMollies: r.tMo, CTNades: r.ctNd, CTPosts: posts, TAWP: r.tAWP, CTAWP: r.ctAWP, Kills: kills})

	// Mitja part, pròrroga, final.
	n := m.round
	endT := r.endT + 1
	if m.score[win] >= targetForH(n, m.H) {
		m.done, m.winner = true, win
		r.emit(endT, "match_end", r.tx.matchEnd(m.names[win], m.score), true, m.endData())
	} else if n == m.H || n == 2*m.H || n > 2*m.H && (n-2*m.H)%3 == 0 {
		cash := startMoney
		if n >= 2*m.H {
			cash = otMoney
		}
		m.resetHalf(cash)
		if n >= 2*m.H && (n-2*m.H)%6 == 0 {
			m.stats.OT++
		}
		nt := m.tTeamFor(n + 1)
		r.emit(endT, "half", r.tx.half(n, m.H, m.score), false, map[string]any{"after": n, "score": m.score, "tTeam": nt, "ot": n >= 2*m.H, "money": cash})
	}
	return m.toEvents(r)
}

func (m *Match) endData() map[string]any {
	type ps struct {
		ID      string  `json:"id"`
		Handle  string  `json:"handle"`
		Team    int     `json:"team"`
		K       int     `json:"k"`
		D       int     `json:"d"`
		HS      int     `json:"hs"`
		FK      int     `json:"fk"`
		ADR     float64 `json:"adr"`
		Clutch  int     `json:"clutches"`
		Plants  int     `json:"plants"`
		Defuses int     `json:"defuses"`
		Rating  float64 `json:"rating"`
	}
	var all []ps
	var best *ps
	for t := 0; t < 2; t++ {
		for _, p := range m.players[t] {
			rt := (float64(p.K)*1.0 - float64(p.D)*0.6 + float64(p.FK)*0.4 + float64(p.Clutches)*1.2 + float64(p.Plants+p.Defuses)*0.3) / float64(max(m.round, 1))
			x := ps{p.ID, p.Handle, t, p.K, p.D, p.HS, p.FK, math.Round(float64(p.Dmg)/float64(max(m.round, 1))*10) / 10, p.Clutches, p.Plants, p.Defuses, math.Round((1+rt)*100) / 100}
			all = append(all, x)
		}
	}
	for i := range all {
		if all[i].Team == m.winner && (best == nil || all[i].Rating > best.Rating) {
			best = &all[i]
		}
	}
	mvp := best.ID
	sort.SliceStable(all, func(i, j int) bool { return all[i].Rating > all[j].Rating })
	return map[string]any{"winner": m.winner, "score": m.score, "rounds": m.round, "mvp": mvp, "players": all, "ot": m.stats.OT}
}

func (m *Match) toEvents(r *rnd) []arena.Event {
	if m.quiet {
		return nil
	}
	kept := r.evs[:0]
	for _, e := range r.evs {
		if e.t <= r.endT+0.02 || e.kind == "half" || e.kind == "match_end" {
			kept = append(kept, e)
		}
	}
	r.evs = kept
	sort.SliceStable(r.evs, func(i, j int) bool {
		if r.evs[i].t != r.evs[j].t {
			return r.evs[i].t < r.evs[j].t
		}
		return r.evs[i].seq < r.evs[j].seq
	})
	before := m.score
	win := -1
	for _, e := range r.evs {
		if e.kind == "round_end" {
			win = e.data["winner"].(int)
		}
	}
	if win >= 0 {
		before[win]--
	}
	out := make([]arena.Event, 0, len(r.evs))
	ended := false
	for _, e := range r.evs {
		if e.kind == "round_end" {
			ended = true
		}
		sc := before
		if ended {
			sc = m.score
		}
		m.evN++
		raw, _ := json.Marshal(e.data)
		out = append(out, arena.Event{N: m.evN, Kind: e.kind, Text: e.text, Highlight: e.hl, Score: sc, Decision: m.decision, Data: raw})
	}
	return out
}
