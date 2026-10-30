package lol

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// ---- poder ----

func (m *match) teamCurve(i int, min float64) float64 {
	s := 0.0
	for _, c := range m.line[i] {
		s += curveAt(c.Curve, min)
	}
	return s / 5
}

func (m *match) buffed(until float64) bool { return m.evT < until }

// power: força d'un equip en una lluita (sense soroll ni enfrontament de
// postures). sMe = la seva postura ("" = sense postura).
func (m *match) power(i int, sMe string) float64 {
	j := 1 - i
	min := m.evT / 60
	p := m.teamCurve(i, min) * math.Exp(m.mod[i])
	gd := clampf(m.gold[i]-m.gold[j], -15000, 15000)
	p *= math.Exp(goldK * gd / 1000)
	p *= 1 + 0.01*float64(len(m.dragons[i]))
	if m.soul[i] {
		p *= 1.06
	}
	if m.buffed(m.baronUntil[i]) {
		p *= 1.08
	}
	if m.buffed(m.elderUntil[i]) {
		p *= 1.2
	}
	if sMe != "" {
		p *= math.Exp(fitK * (clampf(m.fit[i][sMe], 0.4, 1.6) - 1))
	}
	return p
}

const (
	goldK   = 0.025 // força per cada 1k d'or de diferència (log)
	fitK    = 0.12  // força per com encaixa la postura amb la composició
	stanceK = 0.22  // força de guanyar l'enfrontament de postures
)

// splitOK: l'equip i té un split de debò (un splitpusher) i no va per darrere.
func (m *match) splitOK(i int) bool {
	return m.fit[i]["split"] >= 1.1 && m.gold[i]-m.gold[1-i] >= -1500
}

// protectOK: l'equip i pot jugar protect contra engage: encaixa (fit ≥ 1),
// escala millor o té més primera línia.
func (m *match) protectOK(i int) bool {
	j := 1 - i
	if m.fit[i]["protect"] >= 1.0 {
		return true
	}
	if m.lateCurve(i) > m.lateCurve(j)+0.02 {
		return true
	}
	return sumTraits(m.line[i][:]).F >= sumTraits(m.line[j][:]).F+2
}

// lateCurve: el multiplicador de corba de l'equip al minut 35.
func (m *match) lateCurve(i int) float64 { return m.teamCurve(i, 35) }

// stanceWins: guanya la postura a de l'equip i contra la b del rival?
// Cicle base (engage > siege, pick; siege > split, protect; pick > siege,
// split; split > engage, protect; protect > engage, pick) amb dues
// condicions: split només guanya engage si splitOK, i protect només guanya
// engage si protectOK; si no, l'engage (que no vagi més de 1k per darrere)
// guanya ell.
func (m *match) stanceWins(i int, a, b string) bool {
	if a == "" || b == "" || a == b {
		return false
	}
	j := 1 - i
	notBehind := m.gold[i]-m.gold[j] >= -1000
	switch {
	case a == "split" && b == "engage":
		return m.splitOK(i)
	case a == "protect" && b == "engage":
		return m.protectOK(i)
	case a == "engage" && b == "split":
		return !m.splitOK(j) && notBehind
	case a == "engage" && b == "protect":
		return !m.protectOK(j) && notBehind
	}
	return versus(a, b) > 0
}

// stanceEdge: avantatge (log) de l'equip i per l'enfrontament de postures.
// Com més encaixa la postura guanyadora, més val (0.5..1.2 × stanceK).
func (m *match) stanceEdge(i int, a, b string) float64 {
	if !m.stanceWins(i, a, b) {
		return 0
	}
	return stanceK * clampf(m.fit[i][a], 0.5, 1.2)
}

// margin vist des de l'equip 0.
func (m *match) margin(s [2]string) float64 {
	return math.Log(m.power(0, s[0])/m.power(1, s[1])) + m.stanceEdge(0, s[0], s[1]) - m.stanceEdge(1, s[1], s[0]) + m.form
}

// pitMargin: baralla a un objectiu neutral. Qui és més fort controla la
// visió i el terreny del fossar: la diferència de força hi compta 1,5 vegades.
// Disputar un objectiu anant clarament per darrere surt car.
func (m *match) pitMargin(s [2]string) float64 {
	return m.margin(s) + pitK*math.Log(m.power(0, "")/m.power(1, ""))
}

const pitK = 0.5

// powerPct: força de l'equip i respecte del rival, sense postures ni forma
// (el que veu la Situation), en %.
func (m *match) powerPct(i int) float64 {
	return 100 * (m.power(i, "")/m.power(1-i, "") - 1)
}

// ---- morts ----

func (m *match) pickSlot(team int, alive [5]bool, w func(c *Champ) float64) int {
	tot := 0.0
	for r := 0; r < 5; r++ {
		if alive[r] {
			tot += w(m.line[team][r])
		}
	}
	if tot <= 0 {
		return -1
	}
	x := m.rng.Float64() * tot
	for r := 0; r < 5; r++ {
		if alive[r] {
			x -= w(m.line[team][r])
			if x <= 0 {
				return r
			}
		}
	}
	for r := 4; r >= 0; r-- {
		if alive[r] {
			return r
		}
	}
	return -1
}

func killerW(c *Champ) float64 { return 1 + float64(c.DPS) + 0.7*float64(c.Pick) }
func victimW(c *Champ) float64 {
	w := 1 + float64(3-c.Front)
	if c.Class == "marksman" || c.Class == "mage" || c.Class == "enchanter" {
		w += 0.7
	}
	return w
}

var multiName = []string{"", "", "DOUBLE KILL", "TRIPLE KILL", "QUADRA KILL", "PENTAKILL"}

func (m *match) champRef(team, slot int) map[string]any {
	return map[string]any{"side": team, "slot": slot, "champ": m.line[team][slot].Name, "handle": m.handles[team][slot]}
}

func (m *match) kill(kt, ks, vs int, x, y float64, assists []int, multi int, ace bool) {
	vt := 1 - kt
	bounty := 0.0
	if m.streak[vt][vs] >= 2 {
		bounty = math.Min(700, 150*float64(m.streak[vt][vs]-1))
	}
	g := 300 + bounty
	first := !m.firstBlood
	if first {
		g += 100
		m.firstBlood = true
	}
	if m.gold[kt] < m.gold[vt]-4000 {
		g += 150 // recompensa de remuntada
	}
	m.addGold(kt, ks, g)
	for _, a := range assists {
		m.addGold(kt, a, 150/float64(len(assists)))
		m.a[kt][a]++
	}
	m.k[kt][ks]++
	m.d[vt][vs]++
	m.streak[kt][ks]++
	m.streak[vt][vs] = 0
	m.kills[kt]++
	kn, vn := m.line[kt][ks].Name, m.line[vt][vs].Name
	en, ca := fmt.Sprintf("%s kills %s", kn, vn), fmt.Sprintf("%s mata %s", kn, vn)
	hl := false
	tag := ""
	if first {
		en, ca = en+" — FIRST BLOOD", ca+" — PRIMERA SANG"
		tag = "first_blood"
	}
	if multi >= 2 {
		en, ca = en+" — "+multiName[multi], ca+" — "+multiName[multi]
		hl = multi >= 4
	}
	if bounty >= 400 {
		en, ca = en+" (shutdown)", ca+" (shutdown)"
		hl = true
	}
	if ace {
		en, ca = en+" — ACE", ca+" — ACE"
		hl = true
	}
	as := []int{}
	as = append(as, assists...)
	m.emit("kill", en, ca, hl, map[string]any{"x": x, "y": y, "killer": m.champRef(kt, ks), "victim": m.champRef(vt, vs), "assists": as,
		"gold": round(g), "multi": multi, "ace": ace, "shutdown": bounty >= 400, "tag": tag, "side": kt})
}

// soloKill: una mort aïllada (gank, solo kill, pick).
func (m *match) soloKill(kt int, ks, vs int, x, y float64, assists []int) {
	if ks < 0 {
		ks = m.pickSlot(kt, [5]bool{true, true, true, true, true}, killerW)
	}
	if vs < 0 {
		vs = m.pickSlot(1-kt, [5]bool{true, true, true, true, true}, victimW)
	}
	m.kill(kt, ks, vs, x, y, assists, 1, false)
}

// fight resol una baralla. margin (equip 0) ja inclou postures; s'hi afegeix
// atzar. big = baralla d'equip; si no, escaramussa.
func (m *match) fight(x, y, margin float64, big bool, label string) (int, [2]int) {
	margin += m.rng.NormFloat64() * 0.3
	w := 0
	if margin < 0 {
		w = 1
	}
	am := math.Abs(margin)
	var wk, lk int
	if big {
		wk = 2 + int(am*5) + m.rng.Intn(3)
		lk = round(m.rng.NormFloat64()*1.0 + 1.6 - am*4)
	} else {
		wk = 1
		if m.rng.Float64() < 0.3+am {
			wk = 2
		}
		if m.rng.Float64() < 0.28-am*0.5 {
			lk = 1
		}
	}
	wk = clampi(wk, 1, 5)
	lk = clampi(lk, 0, wk-1)
	if lk > 4 {
		lk = 4
	}
	seq := make([]int, 0, wk+lk)
	for i := 0; i < wk; i++ {
		seq = append(seq, w)
	}
	for i := 0; i < lk; i++ {
		p := m.rng.Intn(len(seq)/2 + 1)
		seq = append(seq[:p], append([]int{1 - w}, seq[p:]...)...)
	}
	var alive [2][5]bool
	for i := 0; i < 2; i++ {
		for r := 0; r < 5; r++ {
			alive[i][r] = true
		}
	}
	kind := "skirmish"
	if big {
		kind = "teamfight"
	}
	m.emitT("teamfight", m.tx(fmt.Sprintf("Fight at %s", label), fmt.Sprintf("Baralla a %s", label)), false,
		map[string]any{"x": x, "y": y, "size": kind, "where": label, "winner": w, "side": w})
	multi := map[[2]int]int{}
	var kd [2]int
	for _, kt := range seq {
		vt := 1 - kt
		vs := m.pickSlot(vt, alive[vt], victimW)
		ks := m.pickSlot(kt, alive[kt], killerW)
		if vs < 0 || ks < 0 {
			continue
		}
		alive[vt][vs] = false
		as := []int{}
		for r := 0; r < 5; r++ {
			if r != ks && alive[kt][r] && m.rng.Float64() < 0.55 {
				as = append(as, r)
			}
		}
		multi[[2]int{kt, ks}]++
		ace := true
		for r := 0; r < 5; r++ {
			if alive[vt][r] {
				ace = false
			}
		}
		m.at(m.evT + 2 + m.rng.Float64()*3)
		m.kill(kt, ks, vs, x+m.rng.NormFloat64()*3, y+m.rng.NormFloat64()*3, as, multi[[2]int{kt, ks}], ace)
		kd[kt]++
	}
	m.wf = append(m.wf, fightRec{where: label, w: w, kd: kd, big: big})
	return w, kd
}

// ---- historial: què ha triat cada equip i com ha anat ----

type fightRec struct {
	where string
	w     int
	kd    [2]int
	big   bool
}

type planLog struct {
	t   string    // "14:00"
	ch  [2]string // tria de cada equip
	res [2]string // com ha anat, vist per cada equip
}

type snapT struct {
	kills                         [2]int
	tw, inh, drag, bar, her, grub [2]int
	eld                           [2]int
	gold                          [2]float64
}

func (m *match) towersBy(i int) int {
	j := 1 - i
	return m.towersLost[j][0] + m.towersLost[j][1] + m.towersLost[j][2] + m.nexusTowersLost[j]
}

func (m *match) snapshot() snapT {
	var s snapT
	for i := 0; i < 2; i++ {
		s.kills[i] = m.kills[i]
		s.tw[i] = m.towersBy(i)
		s.inh[i] = m.inhibsLost[1-i]
		s.drag[i] = len(m.dragons[i])
		s.bar[i] = m.barons[i]
		s.her[i] = m.herald[i]
		s.grub[i] = m.grubs[i]
		s.eld[i] = m.elders[i]
		s.gold[i] = m.gold[i]
	}
	return s
}

// outcome: com ha anat la finestra des de s0, vist per l'equip i.
func (m *match) outcome(s0 snapT, i int) string {
	j := 1 - i
	parts := []string{}
	sw, sl := 0, 0
	for _, f := range m.wf {
		if !f.big {
			if f.w == i {
				sw++
			} else {
				sl++
			}
			continue
		}
		if f.w == i {
			parts = append(parts, fmt.Sprintf("you WON the teamfight at %s (%d-%d)", f.where, f.kd[i], f.kd[j]))
		} else {
			parts = append(parts, fmt.Sprintf("you LOST the teamfight at %s (%d-%d)", f.where, f.kd[i], f.kd[j]))
		}
	}
	if sw+sl > 0 {
		parts = append(parts, fmt.Sprintf("skirmishes won %d lost %d", sw, sl))
	}
	parts = append(parts, fmt.Sprintf("kills %d-%d", m.kills[i]-s0.kills[i], m.kills[j]-s0.kills[j]))
	if d := [2]int{m.towersBy(i) - s0.tw[i], m.towersBy(j) - s0.tw[j]}; d[0]+d[1] > 0 {
		parts = append(parts, fmt.Sprintf("towers %d-%d", d[0], d[1]))
	}
	if d := [2]int{m.inhibsLost[j] - s0.inh[i], m.inhibsLost[i] - s0.inh[j]}; d[0]+d[1] > 0 {
		parts = append(parts, fmt.Sprintf("inhibitors %d-%d", d[0], d[1]))
	}
	for _, t := range []int{i, j} {
		who := "you"
		if t == j {
			who = "enemy"
		}
		took := []string{}
		if n := len(m.dragons[t]) - s0.drag[t]; n > 0 {
			took = append(took, plural(n, "drake"))
		}
		if n := m.elders[t] - s0.eld[t]; n > 0 {
			took = append(took, "ELDER")
		}
		if n := m.barons[t] - s0.bar[t]; n > 0 {
			took = append(took, "BARON")
		}
		if n := m.herald[t] - s0.her[t]; n > 0 {
			took = append(took, "herald")
		}
		if n := m.grubs[t] - s0.grub[t]; n > 0 {
			took = append(took, plural(n, "grub"))
		}
		if len(took) > 0 {
			parts = append(parts, who+" took "+strings.Join(took, "+"))
		}
	}
	gs := (m.gold[i] - m.gold[j]) - (s0.gold[i] - s0.gold[j])
	parts = append(parts, "gold swing "+signedK(gs))
	return strings.Join(parts, "; ")
}

func plural(n int, w string) string {
	if n == 1 {
		return "1 " + w
	}
	return fmt.Sprintf("%d %ss", n, w)
}

func signedK(v float64) string {
	if v >= 0 {
		return fmt.Sprintf("+%.1fk", v/1000)
	}
	return fmt.Sprintf("%.1fk", v/1000)
}

// logPlan desa la tria dels dos equips i com ha anat la finestra.
func (m *match) logPlan(t string, ch [2]string, s0 snapT) {
	m.log = append(m.log, planLog{t: t, ch: ch, res: [2]string{m.outcome(s0, 0), m.outcome(s0, 1)}})
	m.wf = nil
}

// ---- estructures ----

func (m *match) inhibDown(team, l int) bool { return m.inhibUntil[team][l] > m.evT }
func (m *match) anyInhibDown(team int) bool {
	for l := 0; l < 3; l++ {
		if m.inhibDown(team, l) {
			return true
		}
	}
	return false
}

func (m *match) maxTier() int {
	switch {
	case m.evT < 840:
		return 1
	case m.evT < 1140:
		return 2
	}
	return 4
}

// pushS: pressió de les onades: només torres, mai inhibidors ni el nexe.
func (m *match) pushS(att, n, lane int) int {
	m.shallow = true
	defer func() { m.shallow = false }()
	return m.push(att, n, lane)
}

// push: l'equip att tomba fins a n estructures (lane -1 = tria). Torna quantes.
func (m *match) push(att, n, lane int) int {
	def := 1 - att
	done := 0
	for k := 0; k < n && !m.done; k++ {
		m.at(m.evT + 15 + m.rng.Float64()*20)
		if !m.shallow && m.anyInhibDown(def) && m.evT >= 1200 {
			if m.nexusTowersLost[def] < 2 {
				m.nexusTowersLost[def]++
				p := blueNexusTowers[m.nexusTowersLost[def]-1]
				if m.sideOf(def) == 1 {
					p = mirror(p)
				}
				m.teamGold(att, 250)
				m.emitT("tower", m.tx(fmt.Sprintf("%s destroy a nexus tower", m.names[att]), fmt.Sprintf("%s tomben una torre del nexe", m.names[att])), false,
					map[string]any{"side": att, "owner": def, "lane": "nexus", "tier": 3 + m.nexusTowersLost[def], "x": p[0], "y": p[1]})
				done++
				continue
			}
			m.finish(att)
			return done + 1
		}
		l := lane
		if l < 0 || m.laneDone(def, l) || (m.shallow && m.towersLost[def][l] >= 3) {
			l = m.chooseLane(def)
		}
		if l < 0 {
			break
		}
		tier := m.towersLost[def][l]
		if tier >= m.maxTier() || (m.shallow && tier >= 3) {
			break
		}
		side := m.sideOf(def)
		if tier < 3 {
			m.towersLost[def][l]++
			p := structXY(side, l, tier)
			g := 550.0
			if m.gold[att] < m.gold[def]-4000 {
				g += 300
			}
			m.teamGold(att, g)
			names := []string{"outer", "inner", "inhibitor"}
			namesCa := []string{"exterior", "interior", "de l'inhibidor"}
			m.emitT("tower", m.tx(fmt.Sprintf("%s destroy the %s %s tower", m.names[att], roleLane[l], names[tier]),
				fmt.Sprintf("%s tomben la torre %s de %s", m.names[att], namesCa[tier], roleLane[l])), tier == 2,
				map[string]any{"side": att, "owner": def, "lane": roleLane[l], "tier": tier + 1, "x": p[0], "y": p[1]})
		} else {
			m.inhibUntil[def][l] = m.evT + 300
			m.inhibsLost[def]++
			p := structXY(side, l, 3)
			m.teamGold(att, 300)
			m.emitT("inhibitor", m.tx(fmt.Sprintf("%s destroy the %s inhibitor!", m.names[att], roleLane[l]), fmt.Sprintf("%s destrueixen l'inhibidor de %s!", m.names[att], roleLane[l])), true,
				map[string]any{"side": att, "owner": def, "lane": roleLane[l], "x": p[0], "y": p[1]})
		}
		done++
	}
	return done
}

func (m *match) laneDone(def, l int) bool { return m.towersLost[def][l] >= 3 && m.inhibDown(def, l) }

// chooseLane: a partir dels 22 min, el carril més avançat; abans, a l'atzar.
func (m *match) chooseLane(def int) int {
	best, bestV := -1, -1.0
	for l := 0; l < 3; l++ {
		if m.laneDone(def, l) || (m.shallow && m.towersLost[def][l] >= 3) {
			continue
		}
		v := m.rng.Float64() * 2
		if m.evT >= 1500 {
			v += float64(m.towersLost[def][l])
		}
		if v > bestV {
			best, bestV = l, v
		}
	}
	return best
}

func (m *match) respawnInhibs() {
	for i := 0; i < 2; i++ {
		for l := 0; l < 3; l++ {
			if m.inhibUntil[i][l] > 0 && m.inhibUntil[i][l] <= m.evT {
				m.inhibUntil[i][l] = 0
				p := structXY(m.sideOf(i), l, 3)
				m.emitT("inhibitor", m.tx(fmt.Sprintf("%s %s inhibitor respawns", m.names[i], roleLane[l]), fmt.Sprintf("L'inhibidor de %s de %s reneix", roleLane[l], m.names[i])), false,
					map[string]any{"side": i, "owner": i, "lane": roleLane[l], "respawn": true, "x": p[0], "y": p[1]})
			}
		}
	}
}

func (m *match) finish(w int) {
	if m.done {
		return
	}
	m.done = true
	m.winner = w
	p := nexusXY(m.sideOf(1 - w))
	m.emitT("nexus", m.tx(fmt.Sprintf("%s destroy the Nexus!", m.names[w]), fmt.Sprintf("%s destrueixen el Nexe!", m.names[w])), true,
		map[string]any{"side": w, "owner": 1 - w, "x": p[0], "y": p[1]})
	// MVP: millor KDA i or de l'equip guanyador.
	best, bestV := 0, -1.0
	players := [2][]map[string]any{}
	for i := 0; i < 2; i++ {
		for r := 0; r < 5; r++ {
			players[i] = append(players[i], map[string]any{"handle": m.handles[i][r], "champ": m.line[i][r].Name, "role": roleNames[r],
				"k": m.k[i][r], "d": m.d[i][r], "a": m.a[i][r], "gold": round(m.pgold[i][r])})
			if i == w {
				v := float64(2*m.k[i][r]+m.a[i][r])/float64(1+m.d[i][r]) + m.pgold[i][r]/8000
				if v > bestV {
					best, bestV = r, v
				}
			}
		}
	}
	comeback := m.has15 && m.gold15[w] < m.gold15[1-w]-1000
	res := m.tx(fmt.Sprintf("%s WIN — Nexus destroyed at %s (kills %d-%d)", m.names[w], fmtTime(m.evT), m.kills[w], m.kills[1-w]),
		fmt.Sprintf("%s GUANYEN — Nexe destruït al %s (morts %d-%d)", m.names[w], fmtTime(m.evT), m.kills[w], m.kills[1-w]))
	m.emitT("match_end", res, true,
		map[string]any{"winner": w, "winner_name": m.names[w], "result": res, "kills": m.kills, "duration": int(m.evT), "mvp": map[string]any{"side": w, "slot": best, "handle": m.handles[w][best], "champ": m.line[w][best].Name,
			"k": m.k[w][best], "d": m.d[w][best], "a": m.a[w][best]},
			"players": players, "graph": m.graph, "comeback": comeback,
			"stats": map[string]any{"kills": m.kills, "dragons": [2]int{len(m.dragons[0]), len(m.dragons[1])}, "barons": m.barons, "heralds": m.herald, "grubs": m.grubs,
				"elders": m.elders, "soul": m.soul, "steals": m.steals, "gold": [2]int{int(m.gold[0]), int(m.gold[1])}}})
}

// ---- objectius neutrals ----

func (m *match) dragonUp(end float64) bool {
	return !m.soul[0] && !m.soul[1] && m.nextDragon <= end
}
func (m *match) elderUp(end float64) bool {
	return (m.soul[0] || m.soul[1]) && m.elderNext > 0 && m.elderNext <= end
}
func (m *match) heraldUp(start float64) bool { return !m.heraldDone && start >= 840 && start < 1185 }
func (m *match) baronUp(start float64) bool  { return m.baronNext <= start+150 }

func (m *match) nextDragonType(i int) string {
	n := len(m.dragons[0]) + len(m.dragons[1])
	if n < 2 {
		return m.dragonSeq[n]
	}
	return m.soulType
}

func (m *match) takeDragon(i int, stolen bool) {
	m.at(math.Max(m.evT, m.nextDragon) + 5)
	typ := m.nextDragonType(i)
	m.dragons[i] = append(m.dragons[i], typ)
	m.nextDragon = m.evT + 300
	m.teamGold(i, 250)
	hl := stolen
	en := fmt.Sprintf("%s take the %s drake", m.names[i], typ)
	ca := fmt.Sprintf("%s maten el drac %s", m.names[i], typ)
	if stolen {
		m.steals[i]++
		en, ca = fmt.Sprintf("DRAGON STOLEN! %s snatch the %s drake", m.names[i], typ), fmt.Sprintf("DRAC ROBAT! %s s'emporten el drac %s", m.names[i], typ)
	}
	soul := false
	if len(m.dragons[i]) == 4 {
		m.soul[i] = true
		soul = true
		m.elderNext = m.evT + 360
		en += fmt.Sprintf(" — %s SOUL!", strings.ToUpper(typ))
		ca += fmt.Sprintf(" — ÀNIMA %s!", strings.ToUpper(typ))
		hl = true
	}
	m.emit("dragon", en, ca, hl, map[string]any{"side": i, "type": typ, "stolen": stolen, "count": len(m.dragons[i]), "soul": soul, "x": dragonPit[0], "y": dragonPit[1]})
}

func (m *match) takeElder(i int, stolen bool) {
	m.at(math.Max(m.evT, m.elderNext) + 5)
	m.elders[i]++
	m.elderUntil[i] = m.evT + 150
	m.elderNext = m.evT + 360
	m.teamGold(i, 250)
	en, ca := fmt.Sprintf("%s slay the ELDER DRAGON", m.names[i]), fmt.Sprintf("%s maten el DRAC ANCESTRAL", m.names[i])
	if stolen {
		m.steals[i]++
		en, ca = "ELDER STOLEN! "+en, "ANCESTRAL ROBAT! "+ca
	}
	m.emit("dragon", en, ca, true, map[string]any{"side": i, "type": "elder", "stolen": stolen, "count": len(m.dragons[i]), "x": dragonPit[0], "y": dragonPit[1]})
}

func (m *match) takeBaron(i int, stolen bool) {
	m.at(math.Max(m.evT, m.baronNext) + 5)
	m.barons[i]++
	m.baronUntil[i] = m.evT + 180
	m.baronNext = m.evT + 360
	m.teamGold(i, 1500)
	en, ca := fmt.Sprintf("%s secure BARON NASHOR", m.names[i]), fmt.Sprintf("%s maten el BARÓ NASHOR", m.names[i])
	if stolen {
		m.steals[i]++
		en, ca = fmt.Sprintf("BARON STOLEN by %s!", m.names[i]), fmt.Sprintf("BARÓ ROBAT per %s!", m.names[i])
	}
	m.emit("baron", en, ca, true, map[string]any{"side": i, "stolen": stolen, "x": baronPit[0], "y": baronPit[1]})
}

func (m *match) takeHerald(i int, stolen bool) {
	m.at(math.Max(m.evT, 840) + 5)
	m.heraldDone = true
	m.herald[i]++
	m.teamGold(i, 200)
	en, ca := fmt.Sprintf("%s take the Rift Herald", m.names[i]), fmt.Sprintf("%s s'emporten l'Herald", m.names[i])
	if stolen {
		m.steals[i]++
		en, ca = "HERALD STOLEN! "+en, "HERALD ROBAT! "+ca
	}
	m.emit("herald", en, ca, stolen, map[string]any{"side": i, "obj": "herald", "stolen": stolen, "x": baronPit[0], "y": baronPit[1]})
}

func (m *match) takeGrubs(i, n int) {
	m.grubs[i] += n
	m.teamGold(i, 60*float64(n))
	m.emitT("herald", m.tx(fmt.Sprintf("%s take %d void grubs", m.names[i], n), fmt.Sprintf("%s maten %d larves del buit", m.names[i], n)), false,
		map[string]any{"side": i, "obj": "grubs", "count": n, "x": baronPit[0], "y": baronPit[1]})
}

func (m *match) takeObjective(i int, obj string, stolen bool) {
	switch obj {
	case "dragon":
		m.takeDragon(i, stolen)
	case "elder":
		m.takeElder(i, stolen)
	case "baron":
		m.takeBaron(i, stolen)
	case "herald":
		m.takeHerald(i, stolen)
	}
}

func objPit(obj string) [2]float64 {
	if obj == "dragon" || obj == "elder" {
		return dragonPit
	}
	return baronPit
}

// ---- fases ----

type laneAns struct {
	Lane   [3]string
	Jungle string
}

var aggrOpts = []string{"aggressive", "even", "passive"}
var jungleOpts = []string{"gank_top", "gank_mid", "gank_bot", "farm", "invade"}

func aggrV(s string) float64 {
	switch s {
	case "aggressive":
		return 1
	case "passive":
		return -1
	}
	return 0
}

func (m *match) laneSlots(l int) []int {
	switch l {
	case 0:
		return []int{0}
	case 1:
		return []int{2}
	}
	return []int{3, 4}
}

func (m *match) laneStr(i, l int) float64 {
	s := 0.0
	sl := m.laneSlots(l)
	for _, r := range sl {
		s += laneScore(m.line[i][r])
	}
	return s / float64(len(sl))
}

var laneSpot = [3][2]float64{{11, 18}, {50, 50}, {82, 89}}

func (m *match) jungleSpot(team int) [2]float64 {
	p := [2]float64{30, 60}
	if m.sideOf(team) == 1 {
		return mirror(p)
	}
	return p
}

func (m *match) applyLaning(ans [2]json.RawMessage) {
	m.t = 0
	m.evT = 0
	var la [2]laneAns
	for i := 0; i < 2; i++ {
		o := parseObj(ans[i])
		for l, k := range roleLane {
			la[i].Lane[l] = oneOf(getStr(o, k), aggrOpts, "even")
		}
		la[i].Jungle = oneOf(getStr(o, "jungle"), jungleOpts, "farm")
		m.laneChoice[i] = la[i]
	}
	s0 := m.snapshot()
	m.emit("phase", "Minions have spawned — laning phase", "Han sortit els súbdits — fase de línies", false,
		map[string]any{"phase": "laning", "lineups": m.lineups(), "choices": [2]laneAns{la[0], la[1]}})
	jp := [2]float64{laneScore(m.line[0][1]), laneScore(m.line[1][1])}
	m.at(90)
	// Invasions.
	for i := 0; i < 2; i++ {
		j := 1 - i
		if la[i].Jungle != "invade" {
			continue
		}
		sp := m.jungleSpot(j)
		if la[j].Jungle == "invade" {
			if i == 0 {
				m.fight(50, 50, 0.08*(jp[0]-jp[1]), false, m.tx("the river", "el riu"))
			}
			continue
		}
		m.at(m.evT + 20)
		m.addGold(i, 1, 350)
		m.addGold(j, 1, -250)
		m.emitT("gank", m.tx(fmt.Sprintf("%s invade and steal camps", m.names[i]), fmt.Sprintf("%s envaeixen i roben camps", m.names[i])), false,
			map[string]any{"side": i, "lane": "jungle", "x": sp[0], "y": sp[1], "success": true, "invade": true})
		if jp[j]-jp[i] >= 0.8 && m.rng.Float64() < 0.4 {
			m.soloKill(j, 1, 1, sp[0], sp[1], []int{2})
		}
	}
	for i := 0; i < 2; i++ {
		if la[i].Jungle == "farm" {
			m.addGold(i, 1, 300)
		}
	}
	// Carrils.
	for l := 0; l < 3; l++ {
		m.at(150 + float64(l)*80 + m.rng.Float64()*40)
		a0, a1 := aggrV(la[0].Lane[l]), aggrV(la[1].Lane[l])
		d := m.laneStr(0, l) - m.laneStr(1, l)
		g := 150 * d * (1 + 0.5*math.Max(a0, a1))
		// Ser agressiu paga si no ets més feble; si ho ets, et castiguen.
		for i, a := range [2]float64{a0, a1} {
			dd := d
			if i == 1 {
				dd = -d
			}
			if a > 0 {
				v := 100.0
				if dd < -0.3 {
					v = -200
				}
				if i == 0 {
					g += v
				} else {
					g -= v
				}
			}
		}
		if (d < 0 && a0 < 0) || (d > 0 && a1 < 0) {
			g *= 0.55
		}
		g += m.rng.NormFloat64() * 130
		sp := laneSpot[l]
		// Solo kill.
		if a0 > 0 || a1 > 0 {
			pk := 0.12 + 0.15*math.Abs(d)
			if m.rng.Float64() < pk {
				kt := 0
				if d+m.rng.NormFloat64()*0.7 < 0 {
					kt = 1
				}
				if [2]float64{a0, a1}[1-kt] >= 0 || m.rng.Float64() < 0.4 {
					sl := m.laneSlots(l)
					m.soloKill(kt, sl[0], sl[m.rng.Intn(len(sl))], sp[0], sp[1], nil)
					if kt == 0 {
						g += 250
					} else {
						g -= 250
					}
				}
			}
		}
		// Ganks.
		gk := "gank_" + roleLane[l]
		for i := 0; i < 2; i++ {
			j := 1 - i
			if la[i].Jungle != gk {
				continue
			}
			m.at(m.evT + 25)
			sign := 1.0
			if i == 1 {
				sign = -1
			}
			if la[j].Jungle == gk {
				if i == 0 {
					mg := 0.1*(jp[0]-jp[1]) + 0.05*d
					w, _ := m.fight(sp[0], sp[1], mg, false, roleLane[l])
					if w == 0 {
						g += 200
					} else {
						g -= 200
					}
				}
				continue
			}
			ps := map[string]float64{"aggressive": 0.62, "even": 0.38, "passive": 0.12}[la[j].Lane[l]]
			ps += 0.06 * (jp[i] - jp[j])
			if la[j].Jungle == "invade" {
				ps -= 0.1
			}
			success := m.rng.Float64() < ps
			m.emitT("gank", m.tx(fmt.Sprintf("%s gank %s", m.line[i][1].Name, roleLane[l]), fmt.Sprintf("%s fa un gank a %s", m.line[i][1].Name, roleLane[l])), false,
				map[string]any{"side": i, "lane": roleLane[l], "x": sp[0], "y": sp[1], "success": success, "jungler": m.champRef(i, 1)})
			sl := m.laneSlots(l)
			if success {
				m.at(m.evT + 6)
				v1 := sl[m.rng.Intn(len(sl))]
				m.soloKill(i, -1, v1, sp[0], sp[1], []int{1})
				if len(sl) == 2 && m.rng.Float64() < 0.3 {
					m.at(m.evT + 3)
					m.soloKill(i, -1, sl[0]+sl[1]-v1, sp[0], sp[1], []int{1})
				}
				g += sign * 250
			} else {
				m.addGold(i, 1, -150)
				if la[j].Lane[l] != "passive" && m.laneStr(j, l) >= m.laneStr(i, l) && m.rng.Float64() < 0.25 {
					m.at(m.evT + 5)
					m.soloKill(j, sl[0], 1, sp[0], sp[1], nil)
				}
			}
		}
		// Diferència de carril.
		m.laneDiff[l] += g
		sl := m.laneSlots(l)
		win := 0
		if g < 0 {
			win = 1
		}
		for _, r := range sl {
			m.addGold(win, r, math.Abs(g)/float64(len(sl)))
		}
		m.at(m.evT + 10)
		m.emitT("lane", m.tx(fmt.Sprintf("%s lane: %s +%d gold", capital(roleLane[l]), m.names[win], round(math.Abs(m.laneDiff[l]))),
			fmt.Sprintf("Carril %s: %s +%d d'or", roleLane[l], m.names[win], round(math.Abs(m.laneDiff[l])))), false,
			map[string]any{"lane": roleLane[l], "diff": round(m.laneDiff[l]), "x": sp[0], "y": sp[1], "side": win})
	}
	m.at(480)
	var ch [2]string
	for i := 0; i < 2; i++ {
		ch[i] = fmt.Sprintf("laning top=%s mid=%s bot=%s jungle=%s", la[i].Lane[0], la[i].Lane[1], la[i].Lane[2], la[i].Jungle)
	}
	m.logPlan("00:00", ch, s0)
	for i := 0; i < 2; i++ {
		m.log[len(m.log)-1].res[i] += fmt.Sprintf("; lane gold leads top %s mid %s bot %s", signed(m.laneAdv(i, 0)), signed(m.laneAdv(i, 1)), signed(m.laneAdv(i, 2)))
	}
	m.t = 480
}

func (m *match) earlyPower(i int) float64 {
	f := m.form / 2
	if i == 1 {
		f = -f
	}
	return m.teamCurve(i, m.evT/60) * math.Exp(m.mod[i]+f) * math.Exp(goldK*(m.gold[i]-m.gold[1-i])/1000)
}

func (m *match) applyObjectives(ans [2]json.RawMessage) {
	m.respawnInhibs()
	var o [2]string
	for i := 0; i < 2; i++ {
		o[i] = oneOf(getStr(parseObj(ans[i]), "objective"), []string{"dragon", "grubs", "farm"}, "dragon")
		m.objHist[i] = append(m.objHist[i], o[i])
	}
	s0 := m.snapshot()
	m.emit("phase", "08:00 — first neutral objectives", "08:00 — primers objectius neutrals", false,
		map[string]any{"phase": "objectives", "choices": o})
	m.at(500)
	side := func(obj string, i int) float64 { // prioritat de carril
		l := 2
		if obj == "grubs" {
			l = 0
		}
		v := m.laneDiff[l]
		if i == 1 {
			v = -v
		}
		return v
	}
	mg := math.Log(m.earlyPower(0) / m.earlyPower(1))
	takeObj := func(i int, obj string, stolen bool) {
		if obj == "dragon" {
			m.takeDragon(i, stolen)
		} else if obj == "grubs" {
			m.at(m.evT + 20)
			m.takeGrubs(i, 3)
		}
	}
	tookGrubs := false
	if o[0] == o[1] && o[0] != "farm" {
		pit := objPit(o[0])
		w, kd := m.fight(pit[0], pit[1], mg+0.00012*(side(o[0], 0)), false, m.tx("the "+o[0], o[0]))
		_ = kd
		stolen := m.rng.Float64() < 0.05+0.02*float64(m.line[1-w][1].Obj)
		if stolen {
			w = 1 - w
		}
		takeObj(w, o[0], stolen)
		tookGrubs = o[0] == "grubs"
	} else {
		for i := 0; i < 2; i++ {
			if o[i] == "farm" {
				m.teamGold(i, 450)
				continue
			}
			takeObj(i, o[i], false)
			if o[i] == "grubs" {
				tookGrubs = true
			}
		}
	}
	if !tookGrubs && m.rng.Float64() < 0.6 {
		i := 0
		if m.laneDiff[0]+m.rng.NormFloat64()*300 < 0 {
			i = 1
		}
		m.at(m.evT + 40)
		m.takeGrubs(i, 3)
	}
	// Una escaramussa més al voltant del minut 10.
	if m.rng.Float64() < 0.6 {
		m.at(m.evT + 40)
		m.fight(50, 50, mg, false, m.tx("mid river", "el riu"))
	}
	m.at(720)
	m.logPlan("08:00", [2]string{"objective=" + o[0], "objective=" + o[1]}, s0)
	m.t = 720
}

var plateActs = []string{"plates", "dive", "roam", "farm"}

func (m *match) laneAdv(i, l int) float64 {
	if i == 0 {
		return m.laneDiff[l]
	}
	return -m.laneDiff[l]
}

func (m *match) applyPlates(ans [2]json.RawMessage) {
	m.respawnInhibs()
	var foc [2]int
	var act [2]string
	for i := 0; i < 2; i++ {
		o := parseObj(ans[i])
		f := getStr(o, "focus")
		foc[i] = 1
		for l, k := range roleLane {
			if f == k {
				foc[i] = l
			}
		}
		act[i] = oneOf(getStr(o, "action"), plateActs, "farm")
		m.plateHist[i] = [2]string{roleLane[foc[i]], act[i]}
	}
	s0 := m.snapshot()
	m.emit("phase", "12:00 — turret plates and roams", "12:00 — plaques i rotacions", false,
		map[string]any{"phase": "plates", "choices": [2]map[string]string{{"focus": roleLane[foc[0]], "action": act[0]}, {"focus": roleLane[foc[1]], "action": act[1]}}})
	m.at(740)
	for i := 0; i < 2; i++ {
		j := 1 - i
		l := foc[i]
		sp := laneSpot[l]
		roamed := act[j] == "roam" && foc[j] == l
		sign := 1.0
		if i == 1 {
			sign = -1
		}
		mg := sign * math.Log(m.earlyPower(0)/m.earlyPower(1))
		switch act[i] {
		case "plates":
			if roamed {
				w, _ := m.fight(sp[0], sp[1], sign*(mg-0.35), false, roleLane[l])
				_ = w
				continue
			}
			g := clampf(300+0.25*m.laneAdv(i, l)+100*float64(m.grubs[i]), 150, 800)
			m.teamGold(i, g)
			m.at(m.evT + 20)
			m.emitT("lane", m.tx(fmt.Sprintf("%s take %d gold of plates %s", m.names[i], round(g), roleLane[l]), fmt.Sprintf("%s s'emporten %d d'or en plaques a %s", m.names[i], round(g), roleLane[l])), false,
				map[string]any{"lane": roleLane[l], "side": i, "plates": round(g), "x": sp[0], "y": sp[1]})
			if (m.laneAdv(i, l) > 500 || m.grubs[i] >= 2) && m.rng.Float64() < 0.55 {
				m.pushS(i, 1, l)
			}
		case "dive":
			ps := 1 / (1 + math.Exp(-(m.laneAdv(i, l)/500 + 3*mg)))
			if roamed {
				ps *= 0.3
			}
			m.at(m.evT + 20)
			if m.rng.Float64() < ps {
				sl := m.laneSlots(l)
				m.soloKill(i, 1, sl[0], sp[0], sp[1], []int{2})
				if m.rng.Float64() < 0.4 {
					m.soloKill(i, -1, -1, sp[0], sp[1], []int{1})
				}
				m.pushS(i, 1, l)
			} else {
				m.soloKill(j, -1, 1, sp[0], sp[1], nil)
				if m.rng.Float64() < 0.5 {
					m.soloKill(j, -1, -1, sp[0], sp[1], nil)
				}
			}
		case "roam":
			if act[j] == "plates" || act[j] == "dive" {
				if foc[j] == l {
					continue // ja resolt a la banda rival
				}
			}
			m.addGold(i, 2, -200)
			if m.rng.Float64() < 0.3 {
				m.at(m.evT + 15)
				sl := m.laneSlots(l)
				m.soloKill(i, 2, sl[0], sp[0], sp[1], []int{1})
			}
		case "farm":
			m.teamGold(i, 350)
		}
	}
	if m.grubs[0]+m.grubs[1] == 0 {
		i := 0
		if m.laneDiff[0] < 0 {
			i = 1
		}
		m.at(m.evT + 20)
		m.takeGrubs(i, 3)
	}
	m.at(840)
	m.logPlan("12:00", [2]string{"plates focus=" + roleLane[foc[0]] + " action=" + act[0], "plates focus=" + roleLane[foc[1]] + " action=" + act[1]}, s0)
	m.t = 840
}

// Objectius disponibles a l'inici d'una fase macro.
func (m *match) objOptions() []string {
	s := m.t
	out := []string{}
	if m.elderUp(s + 180) {
		out = append(out, "elder")
	}
	if m.baronUp(s) {
		out = append(out, "baron")
	}
	if m.dragonUp(s + 180) {
		out = append(out, "dragon")
	}
	if m.heraldUp(s) {
		out = append(out, "herald")
	}
	return append(out, "towers", "farm")
}

func (m *match) defaultObj() string {
	opts := m.objOptions()
	for _, o := range []string{"elder", "baron", "dragon", "herald"} {
		if o == "baron" && m.t < 1440 {
			continue
		}
		for _, x := range opts {
			if x == o {
				return o
			}
		}
	}
	return "towers"
}

func isNeutral(o string) bool { return o == "dragon" || o == "baron" || o == "elder" || o == "herald" }

func sgn(i int) float64 {
	if i == 0 {
		return 1
	}
	return -1
}

// sidePlay: el que fa la postura de l'equip i fora de la gran baralla.
func (m *match) sidePlay(i int, s [2]string) {
	j := 1 - i
	q := clampf(m.fit[i][s[i]], 0.5, 1.2)
	switch s[i] {
	case "split":
		l := 0
		if m.towersLost[j][2] < m.towersLost[j][0] || (m.towersLost[j][2] == m.towersLost[j][0] && m.rng.Intn(2) == 0) {
			l = 2
		}
		sp := laneSpot[l]
		switch {
		case s[j] == "pick" || (s[j] == "engage" && m.stanceWins(j, "engage", "split")):
			// L'agafen sol a la banda.
			if m.rng.Float64() < 0.6 {
				best := 0
				for r := 0; r < 5; r++ {
					if m.line[i][r].Split > m.line[i][best].Split {
						best = r
					}
				}
				m.at(m.evT + 30)
				m.soloKill(j, -1, best, sp[0], sp[1], []int{1})
			}
		case s[j] == "siege":
			// El setge de 5 avança més ràpid que el split: res per al split.
		default:
			if m.rng.Float64() < 0.4+0.4*(q-1)+0.2*float64(boolI(m.herald[i] > 0 && m.t < 1500)) {
				m.pushS(i, 1, l)
			}
			if (s[j] == "protect" || (s[j] == "engage" && m.splitOK(i))) && m.rng.Float64() < 0.5*q {
				m.pushS(i, 1, l)
			}
		}
	case "pick":
		switch s[j] {
		case "siege", "split":
			if m.rng.Float64() < 0.6*q {
				sp := m.jungleSpot(j)
				m.at(m.evT + 25)
				m.soloKill(i, -1, -1, sp[0], sp[1], []int{1})
				if m.rng.Float64() < 0.3 {
					m.soloKill(i, -1, -1, sp[0], sp[1], []int{2})
				}
			}
		case "pick":
			if m.rng.Float64() < 0.3 {
				sp := m.jungleSpot(j)
				m.at(m.evT + 25)
				m.soloKill(i, -1, -1, sp[0], sp[1], []int{1})
			}
		case "engage":
			// El grup de 5 enxampa qui va sol a caçar.
			if m.rng.Float64() < 0.35 {
				sp := m.jungleSpot(i)
				m.at(m.evT + 25)
				m.soloKill(j, -1, -1, sp[0], sp[1], []int{0})
			}
		}
	case "siege":
		switch s[j] {
		case "engage", "pick":
			// Els fan dive / els enxampen posant guardes.
			if m.rng.Float64() < 0.4 {
				m.at(m.evT + 20)
				m.soloKill(j, -1, -1, 50, 50, []int{1})
			}
		case "protect", "split":
			m.pushS(i, 1, 1)
			if m.rng.Float64() < 0.5*q {
				m.pushS(i, 1, -1)
			}
		default:
			if m.rng.Float64() < 0.5 {
				m.pushS(i, 1, 1)
			}
		}
	}
}

// collapseP: probabilitat que l'equip j (que no hi anava) caigui sobre el
// rival que fa un objectiu neutral. adv = avantatge de j a la baralla.
func (m *match) collapseP(obj, oj, sj string, adv float64) float64 {
	p := map[string]float64{"engage": 0.5, "pick": 0.4, "protect": 0.25, "siege": 0.2, "split": 0.1}[sj]
	if obj == "baron" || obj == "elder" {
		p += 0.15
	}
	if oj == "farm" {
		p -= 0.15
	}
	p += 3 * adv
	return clampf(p, 0.05, 0.95)
}

// farmGold: or de farmejar 4 minuts (més si la composició escala).
func (m *match) farmGold(i int) float64 {
	r := m.teamCurve(i, m.t/60+8) / m.teamCurve(i, m.t/60)
	g := 650 * r * r
	if compCurve(m.line[i][:]) == "scaling" {
		g += 300
	}
	return g
}

func (m *match) applyMacro(ans [2]json.RawMessage) {
	m.evT = m.t
	m.respawnInhibs()
	opts := m.objOptions()
	var o, s [2]string
	for i := 0; i < 2; i++ {
		a := parseObj(ans[i])
		o[i] = oneOf(getStr(a, "objective"), opts, m.defaultObj())
		s[i] = oneOf(getStr(a, "stance"), stances, bestStance(m.fit[i]))
		m.stHist[i] = append(m.stHist[i], s[i])
		m.objHist[i] = append(m.objHist[i], o[i])
	}
	m.emitT("phase", m.tx(fmt.Sprintf("%s — %s: %s/%s · %s: %s/%s", fmtTime(m.t), m.names[0], o[0], s[0], m.names[1], o[1], s[1]),
		fmt.Sprintf("%s — %s: %s/%s · %s: %s/%s", fmtTime(m.t), m.names[0], o[0], s[0], m.names[1], o[1], s[1])), false,
		map[string]any{"phase": "macro", "choices": [2]map[string]string{{"objective": o[0], "stance": s[0]}, {"objective": o[1], "stance": s[1]}}})
	s0 := m.snapshot()
	m.wf = nil
	start := m.t
	end := m.t + 240
	m.at(m.t + 20)
	// 1. Joc de bandes segons les postures.
	for i := 0; i < 2 && !m.done; i++ {
		m.sidePlay(i, s)
	}
	mg := m.margin(s)
	// 2. Escaramussa a la jungla mentre es mou la visió.
	if !m.done && m.rng.Float64() < 0.45 {
		m.at(m.evT + 20)
		w := 0
		if mg+m.rng.NormFloat64()*0.3 < 0 {
			w = 1
		}
		sp := m.jungleSpot(1 - w)
		m.fight(sp[0], sp[1], mg, false, m.tx("the jungle", "la jungla"))
		mg = m.margin(s)
	}
	fought := false
	fightWin := -1
	var fightKD [2]int
	doFight := func(x, y, mm float64, label string) int {
		w, kd := m.fight(x, y, mm, true, label)
		fought = true
		fightWin = w
		fightKD = kd
		return w
	}
	// 3. Objectius.
	switch {
	case m.done:
	case o[0] == o[1] && isNeutral(o[0]):
		pit := objPit(o[0])
		m.at(math.Max(m.evT+30, m.spawnOf(o[0])))
		w := doFight(pit[0], pit[1], m.pitMargin(s), m.tx("the "+o[0], o[0]))
		lose := 1 - w
		pst := 0.04 + 0.025*float64(m.line[lose][1].Obj) - 0.01*float64(m.line[w][1].Obj)
		if fightKD[w]-fightKD[lose] >= 3 {
			pst *= 0.3
		}
		if m.rng.Float64() < pst {
			m.takeObjective(lose, o[0], true)
		} else {
			m.takeObjective(w, o[0], false)
		}
	case o[0] == "towers" && o[1] == "towers":
		m.at(m.evT + 40)
		w := doFight(50, 50, mg, m.tx("mid lane", "el carril central"))
		m.push(w, 1+boolI(fightKD[w]-fightKD[1-w] >= 3), 1)
	default:
		busy := [2]bool{}
		for i := 0; i < 2 && !m.done; i++ {
			if !isNeutral(o[i]) {
				continue
			}
			j := 1 - i
			pit := objPit(o[i])
			m.at(math.Max(m.evT+30, m.spawnOf(o[i])))
			// Pot caure-hi a sobre qui no fa cap neutral; i al baró / ancestral
			// (lents), també qui ha fet un altre neutral abans.
			slow := o[i] == "baron" || o[i] == "elder"
			can := !busy[j] && o[j] != o[i] && (!isNeutral(o[j]) || slow)
			pc := m.collapseP(o[i], o[j], s[j], sgn(j)*mg)
			if isNeutral(o[j]) {
				pc -= 0.15
			}
			if can && m.rng.Float64() < pc {
				// El rival hi cau a sobre mentre el fan.
				busy[j] = true
				w := doFight(pit[0], pit[1], m.pitMargin(s)+sgn(j)*0.05, m.tx("the "+o[i], o[i]))
				if w == i {
					m.takeObjective(i, o[i], false)
				} else if m.rng.Float64() < 0.75 {
					m.takeObjective(j, o[i], false)
				}
				continue
			}
			m.takeObjective(i, o[i], false)
		}
		for i := 0; i < 2 && !m.done; i++ {
			if busy[i] {
				continue
			}
			switch o[i] {
			case "towers":
				// Si el rival és a un objectiu neutral, el mapa és teu: una torre més.
				n := 1 + boolI(isNeutral(o[1-i])) + boolI(s[i] == "siege" || s[i] == "split") + boolI(m.buffed(m.baronUntil[i]))
				if m.herald[i] > 0 && m.t < 1500 {
					n++
				}
				m.push(i, n, -1)
			case "farm":
				m.teamGold(i, m.farmGold(i))
			}
		}
		// Els equips es troben igualment.
		pf := 0.35
		if s[0] == "engage" || s[1] == "engage" {
			pf += 0.3
		}
		if s[0] == "pick" || s[1] == "pick" {
			pf += 0.1
		}
		// Qui farmeja evita les baralles.
		pf -= 0.15 * float64(boolI(o[0] == "farm")+boolI(o[1] == "farm"))
		if !fought && !m.done && m.rng.Float64() < pf {
			m.at(m.evT + 30)
			loc := [2]float64{50 + m.rng.NormFloat64()*12, 50 + m.rng.NormFloat64()*12}
			doFight(loc[0], loc[1], m.margin(s), m.tx("mid river", "el riu central"))
		}
	}
	mg = m.margin(s)
	// Pressió de les onades: torres exteriors i interiors.
	for i := 0; i < 2 && !m.done; i++ {
		if m.rng.Float64() < 0.3+0.8*sgn(i)*mg {
			m.pushS(i, 1, -1)
		}
	}
	// Convertir la baralla en estructures.
	if fought && !m.done && m.evT >= 1080 {
		w := fightWin
		diff := fightKD[w] - fightKD[1-w]
		if diff >= 2 {
			n := 1 + boolI(diff >= 4) + boolI(fightKD[w] == 5) + boolI(m.buffed(m.baronUntil[w])) + 2*boolI(m.buffed(m.elderUntil[w]))
			m.push(w, n, -1)
		}
	}
	// Pressió del baró o d'un gran avantatge.
	for i := 0; i < 2 && !m.done; i++ {
		if m.buffed(m.baronUntil[i]) && !(fought && fightWin == i) {
			m.push(i, 1+m.rng.Intn(2), -1)
		}
		if !m.done && m.gold[i]-m.gold[1-i] > 7000 && m.t >= 1500 {
			m.push(i, 1, -1)
		}
	}
	// Objectius que ningú ha triat.
	if !m.done && m.dragonUp(end-30) && o[0] != "dragon" && o[1] != "dragon" && m.rng.Float64() < 0.35 {
		i := 0
		if mg+m.rng.NormFloat64()*0.2 < 0 {
			i = 1
		}
		m.takeDragon(i, false)
	}
	if !m.done && m.heraldUp(m.t) && o[0] != "herald" && o[1] != "herald" && m.rng.Float64() < 0.25 {
		i := 0
		if mg+m.rng.NormFloat64()*0.2 < 0 {
			i = 1
		}
		m.takeHerald(i, false)
	}
	// Massa llarg: la partida es decideix.
	if !m.done && m.t >= 2880 {
		w := 0
		if mg+m.rng.NormFloat64()*0.15 < 0 {
			w = 1
		}
		m.at(m.evT + 20)
		doFight(50, 50, sgn(w)*0.3, m.tx("the final fight", "la baralla final"))
		for !m.done {
			m.push(w, 3, -1)
			if !m.done && m.chooseLane(1-w) < 0 && !m.anyInhibDown(1-w) {
				m.finish(w)
			}
			m.evT += 10
		}
	}
	m.logPlan(fmtTime(start), [2]string{"objective=" + o[0] + " stance=" + s[0], "objective=" + o[1] + " stance=" + s[1]}, s0)
	if !m.done {
		m.at(math.Max(m.evT, end))
		m.t = math.Max(end, m.evT)
		m.t = float64(int(m.t/60)) * 60
		if m.t < end {
			m.t = end
		}
	}
}

func (m *match) spawnOf(o string) float64 {
	switch o {
	case "dragon":
		return m.nextDragon
	case "baron":
		return m.baronNext
	case "elder":
		return m.elderNext
	case "herald":
		return 840
	}
	return 0
}

func boolI(b bool) int {
	if b {
		return 1
	}
	return 0
}

func capital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
