package football

// Jugades cridades de fora (l'Arena): l'atac i la defensa decideixen cada
// jugada i el resultat depèn de com s'enfronten les dues crides, com al futbol
// de debò. Sense crides externes (External=false), el motor juga sol com
// sempre i el calibratge no canvia.

import (
	"fmt"
	"strings"
)

// Crides d'atac.
var OffensePlays = []string{"run_inside", "run_outside", "qb_sneak", "pass_short", "pass_medium", "pass_deep", "screen", "play_action", "punt", "field_goal", "kneel", "spike"}

// Crides de defensa.
var DefenseCalls = []string{"base", "blitz", "cover2", "cover3", "man", "goal_line", "prevent"}

// OffCall és la crida de l'atac.
type OffCall struct {
	Play   string `json:"play"`
	Target string `json:"target"` // wr1, wr2, wr3, te, rb o "" (el QB tria)
	Tempo  string `json:"tempo"`  // normal, hurry, milk
}

// DefCall és la crida de la defensa.
type DefCall struct {
	Call string `json:"call"`
}

// ext són els ajustos d'una jugada cridada.
type ext struct {
	off OffCall
	def DefCall
	// Cursa
	runAdj, stuffAdj float64
	// Passada
	compAdj, sackAdj, intAdj, yacAdj float64
	kind                             string
	playAction                       bool
}

// matchup aplica la matriu crida contra crida (iardes, completades, sacks…).
func matchup(o OffCall, d DefCall) ext {
	e := ext{off: o, def: d}
	dc := d.Call
	switch o.Play {
	case "run_inside", "qb_sneak":
		e.runAdj = map[string]float64{"base": 0, "blitz": .4, "cover2": .9, "cover3": -.2, "man": .4, "goal_line": -1.8, "prevent": 2}[dc]
		e.stuffAdj = map[string]float64{"blitz": .06, "goal_line": .12, "cover2": -.04, "prevent": -.08}[dc]
	case "run_outside":
		e.runAdj = map[string]float64{"base": 0, "blitz": .9, "cover2": .3, "cover3": -.7, "man": .2, "goal_line": -.4, "prevent": 1.6}[dc]
		e.stuffAdj = map[string]float64{"blitz": .05, "cover3": .05, "goal_line": .05, "prevent": -.06}[dc]
	case "pass_short":
		e.kind = "short"
		e.compAdj = map[string]float64{"blitz": .02, "cover2": .03, "cover3": .05, "man": -.06, "goal_line": .05, "prevent": .1}[dc]
		e.sackAdj = map[string]float64{"blitz": .02}[dc]
		e.yacAdj = map[string]float64{"man": -1, "prevent": 1.5, "cover3": .5}[dc]
	case "pass_medium":
		e.kind = "medium"
		e.compAdj = map[string]float64{"blitz": -.05, "cover2": .06, "cover3": -.03, "man": -.03, "goal_line": .14, "prevent": -.03}[dc]
		e.sackAdj = map[string]float64{"blitz": .05}[dc]
		e.intAdj = map[string]float64{"cover3": .008, "blitz": .01}[dc]
	case "pass_deep":
		e.kind = "deep"
		e.compAdj = map[string]float64{"blitz": .08, "cover2": -.13, "cover3": -.06, "man": .07, "goal_line": .16, "prevent": -.16}[dc]
		e.sackAdj = map[string]float64{"blitz": .07, "cover2": -.01, "prevent": -.02}[dc]
		e.intAdj = map[string]float64{"cover2": .02, "prevent": .025, "blitz": .012}[dc]
	case "screen":
		e.kind = "screen"
		e.compAdj = map[string]float64{"blitz": .08, "man": -.04, "goal_line": .05, "prevent": .04}[dc]
		e.yacAdj = map[string]float64{"blitz": 4, "goal_line": 2, "prevent": 2, "base": 0, "cover3": -1, "man": -1.5}[dc]
		e.sackAdj = -.05
	case "play_action":
		e.kind = "medium"
		e.playAction = true
		e.compAdj = map[string]float64{"base": .05, "goal_line": .18, "cover2": .02, "cover3": -.02, "man": .03, "blitz": -.03, "prevent": -.04}[dc]
		e.sackAdj = map[string]float64{"blitz": .08, "base": .01}[dc]
	}
	return e
}

// NormalizeOff deixa una crida d'atac vàlida (els valors desconeguts → per defecte).
func NormalizeOff(c OffCall) OffCall {
	c.Play = strings.ToLower(strings.TrimSpace(c.Play))
	ok := false
	for _, p := range OffensePlays {
		if p == c.Play {
			ok = true
		}
	}
	if !ok {
		c.Play = "pass_short"
	}
	c.Target = strings.ToLower(strings.TrimSpace(c.Target))
	switch c.Tempo {
	case "normal", "hurry", "milk":
	default:
		c.Tempo = "normal"
	}
	return c
}

// NormalizeDef deixa una crida de defensa vàlida.
func NormalizeDef(c DefCall) DefCall {
	c.Call = strings.ToLower(strings.TrimSpace(c.Call))
	for _, d := range DefenseCalls {
		if d == c.Call {
			return c
		}
	}
	c.Call = "base"
	return c
}

// NeedsCall diu si el motor espera les crides de la propera jugada.
func (g *Game) NeedsCall() bool { return g.needCall }

// SetCalls posa les crides de la propera jugada (mode extern).
func (g *Game) SetCalls(o OffCall, d DefCall) {
	g.callOff, g.callDef = NormalizeOff(o), NormalizeDef(d)
	g.haveCall, g.needCall = true, false
}

// Step juga un pas (una jugada, un xut, una conversió). En mode extern s'atura
// abans de cada jugada de línia fins que hi ha crides.
func (g *Game) Step() { g.step() }

// scrimExternal juga la jugada amb les crides rebudes.
func (g *Game) scrimExternal() {
	o, d := g.off(), g.def()
	oc, dc := g.callOff, g.callDef
	g.offHist[g.Poss] = append(g.offHist[g.Poss], callRecord{Down: g.Down, Dist: g.Dist, Call: oc.Play})
	g.defHist[1-g.Poss] = append(g.defHist[1-g.Poss], callRecord{Down: g.Down, Dist: g.Dist, Call: dc.Call})
	g.tempo = oc.Tempo
	defer func() { g.tempo = "" }()
	e := matchup(oc, dc)
	blitz := dc.Call == "blitz"
	switch oc.Play {
	case "punt":
		g.punt()
	case "field_goal":
		g.fieldGoal()
	case "kneel":
		g.kneel()
	case "spike":
		g.spike()
	case "run_inside", "run_outside", "qb_sneak":
		g.ext = &e
		g.run(o, d, blitz)
	default:
		g.ext = &e
		g.pass(o, d, g.Plans[g.Poss], blitz)
	}
	g.ext = nil
}

func (g *Game) spike() {
	qb := g.off().at(QB, 0)
	p := Play{Kind: "spike", Down: g.Down, Dist: g.Dist, Spot: g.Spot, Clock: g.Clock, EndSpot: g.Spot, Actors: Actors{Passer: id(qb)}}
	p.Text = g.t(nameOf(qb)+" spikes the ball to stop the clock.", nameOf(qb)+" clava la pilota a terra per aturar el rellotge.")
	p.Score = g.Score
	g.push(p)
	g.advance(0, false, false, &g.Plays[len(g.Plays)-1])
	g.runClock(1, false)
}

// targetFor tria el receptor que l'atac ha demanat (si n'ha demanat).
func (g *Game) targetFor(o *Team) *Player {
	if g.ext == nil {
		return nil
	}
	switch g.ext.off.Target {
	case "wr1":
		return o.at(WR, 0)
	case "wr2":
		return o.at(WR, 1)
	case "wr3":
		return o.at(WR, 2)
	case "te":
		return o.at(TE, 0)
	case "rb":
		return o.at(RB, 0)
	}
	return nil
}

type callRecord struct {
	Down, Dist int
	Call       string
}

// Tendencies resumeix el que un equip ha cridat fins ara (el que el rival ha
// pogut observar): percentatges globals, en 3a i llarga, i les últimes crides.
func (g *Game) Tendencies(team int, offense bool) string {
	hist := g.defHist[team]
	if offense {
		hist = g.offHist[team]
	}
	if len(hist) == 0 {
		return "no plays observed yet"
	}
	count := func(f func(callRecord) bool) map[string]int {
		m := map[string]int{}
		for _, h := range hist {
			if f(h) {
				m[h.Call]++
			}
		}
		return m
	}
	pct := func(m map[string]int) string {
		total := 0
		for _, v := range m {
			total += v
		}
		if total == 0 {
			return "none"
		}
		var parts []string
		for _, k := range append(append([]string{}, OffensePlays...), DefenseCalls...) {
			if m[k] > 0 {
				parts = append(parts, fmt.Sprintf("%s %d%%", k, m[k]*100/total))
			}
		}
		return fmt.Sprintf("%s (%d plays)", strings.Join(parts, ", "), total)
	}
	all := pct(count(func(callRecord) bool { return true }))
	third := pct(count(func(h callRecord) bool { return h.Down == 3 && h.Dist >= 5 }))
	early := pct(count(func(h callRecord) bool { return h.Down == 1 }))
	last := hist
	if len(last) > 6 {
		last = last[len(last)-6:]
	}
	var recent []string
	for _, h := range last {
		recent = append(recent, fmt.Sprintf("%d&%d %s", h.Down, h.Dist, h.Call))
	}
	return fmt.Sprintf("overall: %s; on 1st down: %s; on 3rd & 5+: %s; last plays: %s", all, early, third, strings.Join(recent, " | "))
}

// Situation descriu la jugada per a una banda (anglès, per al model).
func (g *Game) Situation(team int) string {
	o := g.Poss
	you, them := g.Teams[team], g.Teams[1-team]
	side := "DEFENSE"
	if team == o {
		side = "OFFENSE"
	}
	q := fmt.Sprintf("Q%d", g.Quarter)
	if g.Quarter >= 5 {
		q = "OVERTIME"
	}
	spot := g.Spot
	where := fmt.Sprintf("the offense's own %d", spot)
	if spot > 50 {
		where = fmt.Sprintf("the defense's %d (%d yards to the goal line)", 100-spot, 100-spot)
	} else if spot == 50 {
		where = "midfield"
	}
	dist := fmt.Sprintf("%d", g.Dist)
	if g.Spot+g.Dist >= 100 {
		dist = "goal"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "You are the %s of %s. %s %d:%02d left in the quarter. Score: %s %d - %s %d.\n", side, you.Name, q, g.Clock/60, g.Clock%60, you.Name, g.Score[team], them.Name, g.Score[1-team])
	fmt.Fprintf(&b, "Down: %d & %s at %s. Timeouts: you %d, them %d.\n", g.Down, dist, where, g.Timeouts[team], g.Timeouts[1-team])
	if team == o {
		k := you.at(K, 0)
		fmt.Fprintf(&b, "A field goal from here would be %d yards (your kicker's range is ~%d).\n", 100-g.Spot+17, 52+(k.Attrs.Kick-75)/5)
		fmt.Fprintf(&b, "Their DEFENSE so far: %s.\n", g.Tendencies(1-team, false))
		fmt.Fprintf(&b, "Your offense so far: %s.\n", g.Tendencies(team, true))
	} else {
		fmt.Fprintf(&b, "Their OFFENSE so far: %s.\n", g.Tendencies(1-team, true))
		fmt.Fprintf(&b, "Your defense so far: %s.\n", g.Tendencies(team, false))
	}
	if n := len(g.Plays); n > 0 {
		var last []string
		start := n - 3
		if start < 0 {
			start = 0
		}
		for _, p := range g.Plays[start:] {
			last = append(last, p.Text)
		}
		fmt.Fprintf(&b, "Last plays: %s\n", strings.Join(last, " / "))
	}
	return b.String()
}

// OffenseSchema i DefenseSchema són el JSON que cal respondre.
const OffenseSchema = `{"play":"run_inside|run_outside|qb_sneak|pass_short|pass_medium|pass_deep|screen|play_action|punt|field_goal|kneel|spike","target":"wr1|wr2|wr3|te|rb|","tempo":"normal|hurry|milk"}
- run_inside: power run between the tackles (good vs light boxes: cover2, prevent; bad vs goal_line).
- run_outside: outside zone (good vs blitz and prevent; bad vs cover3's force defender).
- qb_sneak: 1 yard push, for 3rd/4th & 1.
- pass_short (3-9 air yards): beats soft zones (cover3, prevent); hurt by tight man.
- pass_medium (10-18): attacks the hole in cover2; risky vs blitz.
- pass_deep (20+): beats man and blitz if protected; terrible vs cover2/prevent (interceptions).
- screen: punishes the blitz and goal_line; weak vs man.
- play_action: fake run then medium shot: great after you have run a lot and vs goal_line/base; very risky vs blitz.
- punt / field_goal: kick (choose them yourself on 4th down; a field goal beyond range usually misses).
- kneel: run out the clock (when ahead late). spike: stop the clock (costs a down).
- target: preferred receiver for passes (wr1 is the best receiver; "" lets the QB choose).
- tempo: hurry = no huddle, ~12 s between plays (when behind late); milk = use the whole play clock (when ahead late); normal otherwise.`

const DefenseSchema = `{"call":"base|blitz|cover2|cover3|man|goal_line|prevent"}
- base: balanced 4-3, no strong weakness.
- blitz: extra rushers: sacks and stuffed runs, but screens and deep shots beat it.
- cover2: two deep safeties: stops deep passes, weaker vs runs and the medium middle.
- cover3: three deep: good vs outside runs and deep, gives up short passes.
- man: tight man coverage: kills short timing throws, beatable deep.
- goal_line: everyone in the box: stops runs and sneaks, very weak vs play_action and passes.
- prevent: soft deep zone: gives short yards, prevents big plays (end of halves).`

// HasTD diu si l'última jugada va acabar en touchdown (el visor ho necessita).
func (g *Game) LastPlays(after int) []Play {
	if after >= len(g.Plays) {
		return nil
	}
	return g.Plays[after:]
}
