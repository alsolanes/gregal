package football

// Jugades cridades de fora (l'Arena): l'atac i la defensa decideixen cada
// jugada i el resultat depèn de com s'enfronten les dues crides, com al futbol
// de debò. Sense crides externes (External=false), el motor juga sol com
// sempre i el calibratge no canvia.

import (
	"fmt"
	"sort"
	"strconv"
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

// Matchup és la taula pedra-paper-tisora de crida contra crida, vista des de
// l'atac: +2 l'atac guanya clarament, +1 avantatge, 0 igualat, -1 avantatge de
// la defensa, -2 la defensa guanya clarament. Les regles i les ajudes de
// l'esquema es generen d'aquí (MatchupText), i les proves comproven que les
// iardes esperades segueixen la taula.
var Matchup = map[string]map[string]int{
	//             base       blitz       cover2       cover3       man       goal_line       prevent
	"run_inside":  {"base": -1, "blitz": -2, "cover2": 2, "cover3": -1, "man": 0, "goal_line": -2, "prevent": 2},
	"run_outside": {"base": -1, "blitz": 1, "cover2": 0, "cover3": -2, "man": 2, "goal_line": -1, "prevent": 2},
	"qb_sneak":    {"base": 0, "blitz": -1, "cover2": 1, "cover3": 0, "man": 1, "goal_line": -2, "prevent": 2},
	"pass_short":  {"base": 0, "blitz": 1, "cover2": -1, "cover3": 2, "man": -2, "goal_line": 1, "prevent": 1},
	"pass_medium": {"base": 1, "blitz": -2, "cover2": 2, "cover3": 1, "man": 0, "goal_line": 2, "prevent": -1},
	"pass_deep":   {"base": 0, "blitz": 1, "cover2": -2, "cover3": -1, "man": 2, "goal_line": 2, "prevent": -2},
	"screen":      {"base": -1, "blitz": 2, "cover2": 0, "cover3": 1, "man": -2, "goal_line": 1, "prevent": 1},
	"play_action": {"base": 2, "blitz": -2, "cover2": 0, "cover3": 0, "man": 0, "goal_line": 2, "prevent": -1},
}

// matchLevel és, per a cada jugada, la força x (vegeu adjFor) que dona a cada
// nota de la taula (-2..+2) les iardes esperades 5,0 + 1,2·nota (1r i 10, una
// pèrdua de pilota compta com -35). Calibrat amb TestMatchupCalibration.
var matchLevel = map[string][5]float64{
	"run_inside":  {-1.36, -0.13, 0.94, 1.92, 2.82},
	"run_outside": {-1.39, -0.17, 0.92, 1.88, 2.80},
	"qb_sneak":    {-1.8, -0.9, 0, 0.9, 1.8},
	"pass_short":  {-3.73, -2.00, -0.15, 2.00, 4.00},
	"pass_medium": {-4.67, -3.43, -2.05, -0.97, 0.52},
	"pass_deep":   {-4.00, -3.17, -2.18, -1.46, -0.66},
	"screen":      {-1.67, -0.69, 1.67, 2.50, 3.33},
	"play_action": {-4.67, -3.43, -2.05, -0.97, 0.52},
}

// adjFor tradueix una força x en els ajustos de la jugada (x > 0 ajuda l'atac).
func adjFor(play string, x float64) ext {
	var e ext
	neg := 0.0
	if x < 0 {
		neg = -x
	}
	switch play {
	case "run_inside", "run_outside", "qb_sneak":
		e.runAdj = x
		e.stuffAdj = -.03 * x
	case "screen":
		e.kind = "screen"
		e.compAdj = .03 * x
		e.yacAdj = 1.2 * x
		e.sackAdj = -.05
	default:
		e.kind = map[string]string{"pass_short": "short", "pass_medium": "medium", "pass_deep": "deep", "play_action": "medium"}[play]
		e.playAction = play == "play_action"
		e.compAdj = .035 * x
		e.yacAdj = .5 * x
		e.sackAdj = .012 * neg
		e.intAdj = map[string]float64{"short": .003, "medium": .005, "deep": .007}[e.kind] * neg
	}
	return e
}

// matchup aplica la taula: crida d'atac contra crida de defensa.
func matchup(o OffCall, d DefCall) ext {
	r := Matchup[o.Play][d.Call]
	lv, ok := matchLevel[o.Play]
	if !ok {
		return ext{off: o, def: d}
	}
	e := adjFor(o.Play, lv[r+2])
	e.off, e.def = o, d
	return e
}

// NormalizeOff deixa una crida d'atac vàlida (els valors desconeguts → per defecte).
func NormalizeOff(c OffCall) OffCall {
	c.Play = strings.ToLower(strings.TrimSpace(c.Play))
	if !validPlay(c.Play) {
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

func validPlay(p string) bool {
	for _, x := range OffensePlays {
		if x == p {
			return true
		}
	}
	return false
}

// isScrimmage: jugades de línia de veritat (no xuts ni jugades de rellotge).
func isScrimmage(p string) bool {
	_, ok := Matchup[p]
	return ok
}

func isRun(p string) bool { return p == "run_inside" || p == "run_outside" || p == "qb_sneak" }

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

// fgRange és la distància màxima raonable del xutador de l'atac.
func (g *Game) fgRange() int { return 52 + (g.off().at(K, 0).Attrs.Kick-75)/5 }

// NeedsCall diu si el motor espera les crides de la propera jugada.
func (g *Game) NeedsCall() bool { return g.needCall }

// SetCalls posa les crides de la propera jugada (mode extern). Si l'atac no
// ha cridat res de vàlid, juga pass_short; a la 4a, xuta (gol de camp si és a
// l'abast, punt si no) en lloc d'arriscar-s'hi.
func (g *Game) SetCalls(o OffCall, d DefCall) {
	raw := strings.ToLower(strings.TrimSpace(o.Play))
	o = NormalizeOff(o)
	if !validPlay(raw) && g.Down == 4 {
		o.Play = "punt"
		if 100-g.Spot+17 <= g.fgRange() {
			o.Play = "field_goal"
		}
	}
	g.callOff, g.callDef = o, NormalizeDef(d)
	g.haveCall, g.needCall = true, false
}

// Step juga un pas (una jugada, un xut, una conversió). En mode extern s'atura
// abans de cada jugada de línia fins que hi ha crides.
func (g *Game) Step() { g.step() }

// scrimExternal juga la jugada amb les crides rebudes. La taula Matchup ja
// inclou l'efecte de cada defensa (també el del blitz), així que el motor no
// hi suma res més.
func (g *Game) scrimExternal() {
	o, d := g.off(), g.def()
	oc, dc := g.callOff, g.callDef
	poss := g.Poss
	scrim := isScrimmage(oc.Play)
	if scrim {
		g.offHist[poss] = append(g.offHist[poss], callRecord{Down: g.Down, Dist: g.Dist, Call: oc.Play, Opp: dc.Call})
		g.defHist[1-poss] = append(g.defHist[1-poss], callRecord{Down: g.Down, Dist: g.Dist, Call: dc.Call, Opp: oc.Play})
	}
	g.tempo = oc.Tempo
	defer func() { g.tempo = "" }()
	e := matchup(oc, dc)
	before := len(g.Plays)
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
		g.run(o, d, false)
	default:
		g.ext = &e
		g.pass(o, d, g.Plans[poss], false)
	}
	g.ext = nil
	if scrim && len(g.Plays) > before {
		p := g.Plays[before]
		for _, h := range []*callRecord{&g.offHist[poss][len(g.offHist[poss])-1], &g.defHist[1-poss][len(g.defHist[1-poss])-1]} {
			h.Done, h.Yards, h.Success, h.Turnover = true, p.Yards, playSuccess(p), p.Intercept || p.Fumble
		}
	}
}

// playSuccess: una jugada «bona» (definició habitual de la NFL): el 40% de la
// distància al 1r down, el 60% al 2n i el primer down al 3r i 4t.
func playSuccess(p Play) bool {
	switch {
	case p.Intercept || p.Fumble:
		return false
	case p.TD || p.FirstDown:
		return true
	case p.Down == 1:
		return float64(p.Yards) >= .4*float64(p.Dist)
	case p.Down == 2:
		return float64(p.Yards) >= .6*float64(p.Dist)
	}
	return p.Yards >= p.Dist
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

// callRecord és una jugada de línia vista per un equip: la seva crida, la del
// rival i com ha anat (iardes, èxit, pèrdua de pilota).
type callRecord struct {
	Down, Dist int
	Call, Opp  string
	Done       bool
	Yards      int
	Success    bool
	Turnover   bool
}

var allCalls = append(append([]string{}, OffensePlays...), DefenseCalls...)

var callIdx = func() map[string]int {
	m := map[string]int{}
	for i, c := range allCalls {
		m[c] = i
	}
	return m
}()

// Tendencies resumeix el que un equip ha cridat fins ara a les jugades de
// línia (el que el rival ha pogut observar): percentatges globals, al 1r down,
// en 3a i llarga, i les últimes crides.
func (g *Game) Tendencies(team int, offense bool) string {
	hist := g.defHist[team]
	if offense {
		hist = g.offHist[team]
	}
	if len(hist) == 0 {
		return "no plays observed yet"
	}
	b := make([]byte, 0, 512)
	pct := func(f func(callRecord) bool) {
		var n [32]int
		total := 0
		for _, h := range hist {
			if f(h) {
				n[callIdx[h.Call]]++
				total++
			}
		}
		if total == 0 {
			b = append(b, "none"...)
			return
		}
		first := true
		for i, k := range allCalls {
			if n[i] > 0 {
				if !first {
					b = append(b, ", "...)
				}
				first = false
				b = append(b, k...)
				b = append(b, ' ')
				b = strconv.AppendInt(b, int64(n[i]*100/total), 10)
				b = append(b, '%')
			}
		}
		b = append(b, " ("...)
		b = strconv.AppendInt(b, int64(total), 10)
		b = append(b, " plays)"...)
	}
	b = append(b, "overall: "...)
	pct(func(callRecord) bool { return true })
	b = append(b, "; on 1st down: "...)
	pct(func(h callRecord) bool { return h.Down == 1 })
	b = append(b, "; on 3rd & 5+: "...)
	pct(func(h callRecord) bool { return h.Down == 3 && h.Dist >= 5 })
	b = append(b, "; last plays: "...)
	last := hist
	if len(last) > 6 {
		last = last[len(last)-6:]
	}
	for i, h := range last {
		if i > 0 {
			b = append(b, " | "...)
		}
		b = strconv.AppendInt(b, int64(h.Down), 10)
		b = append(b, '&')
		b = strconv.AppendInt(b, int64(h.Dist), 10)
		b = append(b, ' ')
		b = append(b, h.Call...)
	}
	return string(b)
}

// results resumeix com han anat les jugades agrupades per key (un índex dins
// order): iardes per jugada, % d'èxit i pèrdues de pilota, de la més cridada a
// la que menys.
func results(hist []callRecord, key func(callRecord) int, order []string, prefix string) string {
	type agg struct {
		i, n, yds, ok, to int
	}
	var m [16]agg
	any := false
	for _, h := range hist {
		if !h.Done {
			continue
		}
		k := key(h)
		if k < 0 || k >= len(order) {
			continue
		}
		a := &m[k]
		a.n++
		a.yds += h.Yards
		if h.Success {
			a.ok++
		}
		if h.Turnover {
			a.to++
		}
		any = true
	}
	if !any {
		return "none yet"
	}
	aggs := m[:len(order)]
	for i := range aggs {
		aggs[i].i = i
	}
	sort.SliceStable(aggs, func(i, j int) bool { return aggs[i].n > aggs[j].n })
	b := make([]byte, 0, 256)
	for _, a := range aggs {
		if a.n == 0 {
			continue
		}
		if len(b) > 0 {
			b = append(b, "; "...)
		}
		b = append(b, prefix...)
		b = append(b, order[a.i]...)
		b = append(b, ' ')
		b = strconv.AppendInt(b, int64(a.n), 10)
		b = append(b, " plays, "...)
		b = strconv.AppendFloat(b, float64(a.yds)/float64(a.n), 'f', 1, 64)
		b = append(b, " yds/play, "...)
		b = strconv.AppendInt(b, int64(a.ok*100/a.n), 10)
		b = append(b, "% success"...)
		if a.to > 0 {
			b = append(b, ", "...)
			b = strconv.AppendInt(b, int64(a.to), 10)
			b = append(b, " turnovers"...)
		}
	}
	return string(b)
}

var (
	scrimOrder = scrimmageOrder()
	scrimIdx   = indexOf(scrimOrder)
	defIdx     = indexOf(DefenseCalls)
	runPass    = []string{"runs", "passes"}
)

func indexOf(list []string) map[string]int {
	m := map[string]int{}
	for i, s := range list {
		m[s] = i
	}
	return m
}

func runOrPass(p string) int {
	if isRun(p) {
		return 0
	}
	return 1
}

// Results diu com han anat les jugades de línia d'un equip: en atac (offense)
// les seves pròpies jugades; en defensa, les del rival contra ell.
func (g *Game) Results(team int, offense bool) string {
	if offense {
		h := g.offHist[team]
		return "by your call: " + results(h, func(c callRecord) int { return scrimIdx[c.Call] }, scrimOrder, "") +
			". By their defense: " + results(h, func(c callRecord) int { return defIdx[c.Opp] }, DefenseCalls, "vs ") +
			". Runs vs passes: " + results(h, func(c callRecord) int { return runOrPass(c.Call) }, runPass, "")
	}
	h := g.defHist[team]
	return "by their call: " + results(h, func(c callRecord) int { return scrimIdx[c.Opp] }, scrimOrder, "") +
		". By your defense: " + results(h, func(c callRecord) int { return defIdx[c.Call] }, DefenseCalls, "") +
		". Their runs vs passes: " + results(h, func(c callRecord) int { return runOrPass(c.Opp) }, runPass, "")
}

func scrimmageOrder() []string {
	var out []string
	for _, p := range OffensePlays {
		if isScrimmage(p) {
			out = append(out, p)
		}
	}
	return out
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
		fmt.Fprintf(&b, "A field goal from here would be %d yards (your kicker's range is ~%d).\n", 100-g.Spot+17, g.fgRange())
		fmt.Fprintf(&b, "Their DEFENSE so far: %s.\n", g.Tendencies(1-team, false))
		fmt.Fprintf(&b, "Your offense so far: %s.\n", g.Tendencies(team, true))
		fmt.Fprintf(&b, "RESULTS of your offense so far (success = 40%% of the distance on 1st down, 60%% on 2nd, a first down on 3rd/4th): %s.\n", g.Results(team, true))
	} else {
		fmt.Fprintf(&b, "Their OFFENSE so far: %s.\n", g.Tendencies(1-team, true))
		fmt.Fprintf(&b, "Your defense so far: %s.\n", g.Tendencies(team, false))
		fmt.Fprintf(&b, "RESULTS of their offense against you so far (success = 40%% of the distance on 1st down, 60%% on 2nd, a first down on 3rd/4th): %s.\n", g.Results(team, false))
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

var ratingSign = map[int]string{2: "++", 1: "+", 0: "=", -1: "-", -2: "--"}

// MatchupLine resumeix la fila d'una crida d'atac de la taula: contra quines
// defenses guanya (++, +), empata (=) o perd (-, --).
func MatchupLine(play string) string {
	row, ok := Matchup[play]
	if !ok {
		return ""
	}
	var parts []string
	for _, r := range []int{2, 1, 0, -1, -2} {
		var ds []string
		for _, d := range DefenseCalls {
			if row[d] == r {
				ds = append(ds, d)
			}
		}
		if len(ds) > 0 {
			parts = append(parts, ratingSign[r]+" "+strings.Join(ds, ", "))
		}
	}
	return strings.Join(parts, "; ")
}

// DefenseLine és la columna d'una defensa: què atura i què la castiga.
func DefenseLine(call string) string {
	var stops, beaten []string
	for _, p := range scrimmageOrder() {
		r := Matchup[p][call]
		switch {
		case r < 0:
			stops = append(stops, p+" ("+ratingSign[r]+")")
		case r > 0:
			beaten = append(beaten, p+" ("+ratingSign[r]+")")
		}
	}
	return "stops " + strings.Join(stops, ", ") + "; beaten by " + strings.Join(beaten, ", ")
}

// MatchupText és la taula sencera, per a les regles.
func MatchupText() string {
	var b strings.Builder
	b.WriteString("MATCHUP TABLE (offense call: result vs each defense call; ++ big edge for the offense, about +2.5 yards/play; + edge; = even; - edge for the defense; -- big edge for the defense, about -2.5 yards/play and more sacks/turnovers):\n")
	for _, p := range scrimmageOrder() {
		fmt.Fprintf(&b, "- %s: %s\n", p, MatchupLine(p))
	}
	return b.String()
}

var offDesc = map[string]string{
	"run_inside":  "power run between the tackles",
	"run_outside": "outside zone run to the edge",
	"qb_sneak":    "QB push for 1-2 yards (3rd/4th & 1 only)",
	"pass_short":  "quick throw, 3-9 yards",
	"pass_medium": "intermediate throw, 10-18 yards",
	"pass_deep":   "deep shot, 20+ yards (interceptions when it loses)",
	"screen":      "screen to the RB behind the rush",
	"play_action": "fake run, then a medium throw (better once 40%+ of your plays have been runs, worse if you never run)",
}

var defDesc = map[string]string{
	"base":      "balanced 4-3",
	"blitz":     "extra rushers",
	"cover2":    "two deep safeties, light box",
	"cover3":    "three deep, eight in the box",
	"man":       "tight man coverage",
	"goal_line": "everyone in the box (short yardage)",
	"prevent":   "soft deep zone (end of halves)",
}

// OffenseSchema i DefenseSchema són el JSON que cal respondre; les ajudes de
// cada crida surten de la taula Matchup.
var OffenseSchema = func() string {
	var b strings.Builder
	b.WriteString(`{"play":"run_inside|run_outside|qb_sneak|pass_short|pass_medium|pass_deep|screen|play_action|punt|field_goal|kneel|spike","target":"wr1|wr2|wr3|te|rb|","tempo":"normal|hurry|milk"}` + "\n")
	for _, p := range scrimmageOrder() {
		fmt.Fprintf(&b, "- %s: %s. vs defenses: %s.\n", p, offDesc[p], MatchupLine(p))
	}
	b.WriteString(`- punt / field_goal: kick (choose them yourself on 4th down; a field goal beyond your kicker's range usually misses). Without a valid answer on 4th down you kick.
- kneel: run out the clock (when ahead late). spike: stop the clock (costs a down).
- target: preferred receiver for passes (wr1 is the best receiver; "" lets the QB choose).
- tempo: hurry = no huddle, ~12 s between plays (when behind late); milk = use the whole play clock (when ahead late); normal otherwise.`)
	return b.String()
}()

var DefenseSchema = func() string {
	var b strings.Builder
	b.WriteString(`{"call":"base|blitz|cover2|cover3|man|goal_line|prevent"}` + "\n")
	for _, d := range DefenseCalls {
		fmt.Fprintf(&b, "- %s: %s; %s.\n", d, defDesc[d], DefenseLine(d))
	}
	return strings.TrimSpace(b.String())
}()

// LastPlays torna les jugades a partir de la posició after (el visor les necessita).
func (g *Game) LastPlays(after int) []Play {
	if after >= len(g.Plays) {
		return nil
	}
	return g.Plays[after:]
}
