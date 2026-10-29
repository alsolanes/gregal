// Package football és el motor d'un partit de futbol americà (regles de la
// NFL) entre els vents. Tot el joc és codi: downs, iardes, rellotge, quatre
// quarts i pròrroga, curses i passades, sacks, intercepcions, fumbles, gols
// de camp, punts, xuts d'inici, penals i el final de cada part. El que fa
// que una jugada surti bé o malament són els atributs dels jugadors (velocitat,
// força, braç, mans, placatge…) contra els del rival, amb atzar.
//
// El model només hi entra des de fora: els entrenadors (dos vents) decideixen
// el pla de joc de cada quart (Plan) i un comentarista escriu els moments
// clau. Un partit té un sol generador d'atzar amb llavor, de manera que el
// mateix partit amb els mateixos plans es pot tornar a jugar igual.
//
// Calibratge (mitjanes NFL recents, partit entre equips iguals): ~22 punts i
// ~62 jugades d'atac per equip, 4,3 iardes per cursa, ~64 % de passades
// completades a ~7 iardes per intent, ~2,5 sacks, ~0,8 intercepcions i ~4
// punts (xuts) per equip. engine_test.go ho comprova sobre centenars de
// partits.
package football

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

// Posicions (11 d'atac, 11 de defensa i el xutador).
const (
	QB = "QB"
	RB = "RB"
	WR = "WR"
	TE = "TE"
	OL = "OL"
	DL = "DL"
	LB = "LB"
	CB = "CB"
	S  = "S"
	K  = "K"
)

// RosterShape és quants jugadors de cada posició té un equip.
var RosterShape = []struct {
	Pos string
	N   int
}{{QB, 1}, {RB, 1}, {WR, 3}, {TE, 1}, {OL, 5}, {DL, 4}, {LB, 3}, {CB, 2}, {S, 2}, {K, 1}}

// RosterSize és el total de jugadors d'un equip (23).
func RosterSize() int {
	n := 0
	for _, r := range RosterShape {
		n += r.N
	}
	return n
}

// Attrs són els atributs d'un jugador, de 40 a 99.
type Attrs struct {
	Speed     int `json:"spd"`
	Strength  int `json:"str"`
	Agility   int `json:"agi"`
	Arm       int `json:"arm"`
	Accuracy  int `json:"acc"`
	Hands     int `json:"hands"`
	Block     int `json:"block"`
	Tackle    int `json:"tackle"`
	Coverage  int `json:"cover"`
	Kick      int `json:"kick"`
	Awareness int `json:"awr"`
}

// Player és un jugador: un vent de l'usuari (WindID) o un de l'equip.
type Player struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Pos    string `json:"pos"`
	Number int    `json:"number"`
	Color  string `json:"color,omitempty"`
	Style  string `json:"style,omitempty"`
	WindID string `json:"wind,omitempty"`
	Attrs  Attrs  `json:"attrs"`
}

// Overall és la nota d'un jugador per a la seva posició.
func (p Player) Overall() int {
	a := p.Attrs
	var v float64
	switch p.Pos {
	case QB:
		v = .35*float64(a.Accuracy) + .3*float64(a.Arm) + .25*float64(a.Awareness) + .1*float64(a.Speed)
	case RB:
		v = .3*float64(a.Speed) + .3*float64(a.Agility) + .25*float64(a.Strength) + .15*float64(a.Hands)
	case WR:
		v = .35*float64(a.Speed) + .35*float64(a.Hands) + .3*float64(a.Agility)
	case TE:
		v = .3*float64(a.Hands) + .3*float64(a.Block) + .2*float64(a.Strength) + .2*float64(a.Speed)
	case OL:
		v = .6*float64(a.Block) + .4*float64(a.Strength)
	case DL:
		v = .4*float64(a.Strength) + .3*float64(a.Tackle) + .3*float64(a.Speed)
	case LB:
		v = .4*float64(a.Tackle) + .3*float64(a.Speed) + .3*float64(a.Coverage)
	case CB, S:
		v = .45*float64(a.Coverage) + .35*float64(a.Speed) + .2*float64(a.Tackle)
	case K:
		v = float64(a.Kick)
	}
	return int(math.Round(v))
}

// Team és una franquícia.
type Team struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Abbr    string   `json:"abbr"`
	Color   string   `json:"color"`
	Color2  string   `json:"color2"`
	Coach   string   `json:"coach"` // id del vent entrenador
	Players []Player `json:"players"`
	Wins    int      `json:"wins"`
	Losses  int      `json:"losses"`
	Ties    int      `json:"ties"`
}

func (t *Team) at(pos string, i int) *Player {
	n := 0
	for k := range t.Players {
		if t.Players[k].Pos == pos {
			if n == i {
				return &t.Players[k]
			}
			n++
		}
	}
	return nil
}

func (t *Team) all(pos string) []*Player {
	var out []*Player
	for k := range t.Players {
		if t.Players[k].Pos == pos {
			out = append(out, &t.Players[k])
		}
	}
	return out
}

func avg(ps []*Player, f func(Attrs) int) float64 {
	if len(ps) == 0 {
		return 60
	}
	s := 0
	for _, p := range ps {
		s += f(p.Attrs)
	}
	return float64(s) / float64(len(ps))
}

// Plan és el pla de joc d'un entrenador per a un quart.
type Plan struct {
	PassRate   float64 `json:"pass_rate"`   // 0..1, quant passa en situació neutra (NFL ~0.58)
	DeepShots  float64 `json:"deep_shots"`  // 0..1, part de passades llargues (~0.18)
	Aggression float64 `json:"aggression"`  // 0..1, anar a per la 4a i el 2 punts
	Blitz      float64 `json:"blitz"`       // 0..1, quant envia pressió extra (~0.25)
	Tempo      float64 `json:"tempo"`       // 0..1, atac sense pausa
	Focus      string  `json:"focus"`       // id d'un jugador a qui buscar més
	Note       string  `json:"note"`        // la frase de l'entrenador
}

// DefaultPlan és un pla neutre (el que fa un entrenador sense model).
func DefaultPlan() Plan {
	return Plan{PassRate: .58, DeepShots: .18, Aggression: .35, Blitz: .25, Tempo: .3}
}

func (p Plan) clamp() Plan {
	c := func(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
	p.PassRate = c(p.PassRate, .3, .8)
	p.DeepShots = c(p.DeepShots, .05, .4)
	p.Aggression = c(p.Aggression, 0, 1)
	p.Blitz = c(p.Blitz, .05, .6)
	p.Tempo = c(p.Tempo, 0, 1)
	return p
}

// Actors són els jugadors que surten a una jugada (ids).
type Actors struct {
	Passer   string `json:"passer,omitempty"`
	Target   string `json:"target,omitempty"`
	Runner   string `json:"runner,omitempty"`
	Tackler  string `json:"tackler,omitempty"`
	Defender string `json:"defender,omitempty"`
	Kicker   string `json:"kicker,omitempty"`
	Returner string `json:"returner,omitempty"`
}

// Play és una jugada: el que ha passat i prou per animar-la.
type Play struct {
	N         int    `json:"n"`
	Quarter   int    `json:"q"`
	Clock     int    `json:"clock"` // segons que queden al quart en començar
	Offense   int    `json:"off"`   // 0 local, 1 visitant
	Down      int    `json:"down"`
	Dist      int    `json:"dist"`
	Spot      int    `json:"spot"` // iardes des de la pròpia línia de gol de l'atac (1..99)
	Kind      string `json:"kind"` // run, pass, sack, scramble, punt, fg, kickoff, pat, two, kneel, spike, penalty
	Detail    string `json:"detail,omitempty"`
	Dir       string `json:"dir,omitempty"`   // left, middle, right
	Depth     int    `json:"depth,omitempty"` // iardes d'aire d'una passada
	Yards     int    `json:"yards"`
	EndSpot   int    `json:"end"` // on acaba la pilota (mateixa referència que Spot)
	Complete  bool   `json:"complete,omitempty"`
	Intercept bool   `json:"int,omitempty"`
	Fumble    bool   `json:"fumble,omitempty"`
	TD        bool   `json:"td,omitempty"`
	Safety    bool   `json:"safety,omitempty"`
	Good      bool   `json:"good,omitempty"` // gol de camp / PAT / 2 punts
	FirstDown bool   `json:"first,omitempty"`
	Turnover  bool   `json:"turnover,omitempty"`
	Penalty   string `json:"penalty,omitempty"`
	Actors    Actors `json:"actors"`
	Score     [2]int `json:"score"`
	Highlight bool   `json:"hl,omitempty"`
	Text      string `json:"text"`
	Comment   string `json:"comment,omitempty"`
	Drive     int    `json:"drive"`
}

// Stat és la línia d'estadística d'un jugador en un partit.
type Stat struct {
	PassAtt, PassComp, PassYds, PassTD, Ints int
	RushAtt, RushYds, RushTD                 int
	Rec, RecYds, RecTD                       int
	Tackles, Sacks, DefInt                   int
	FGM, FGA                                 int
}

// Game és un partit en marxa.
type Game struct {
	Teams     [2]*Team
	Plans     [2]Plan
	Plays     []Play
	Stats     map[string]*Stat
	Score     [2]int
	Quarter   int
	Clock     int
	Poss      int
	Spot      int
	Down      int
	Dist      int
	Timeouts  [2]int
	Drive     int
	Over      bool
	rng       *rand.Rand
	recv2nd   int  // qui rep a la segona part
	warned    bool // avís dels dos minuts ja donat en aquesta part
	otPoss    [2]bool
	Lang      string
	kickNext  bool // toca xut d'inici (després de puntuar o començar part)
	afterTD   bool // toca PAT / 2 punts
}

// NewGame prepara un partit: llançament de moneda i xut d'inici.
func NewGame(home, away *Team, seed int64, lang string) *Game {
	g := &Game{Teams: [2]*Team{home, away}, Plans: [2]Plan{DefaultPlan(), DefaultPlan()}, Stats: map[string]*Stat{},
		Quarter: 1, Clock: 900, Timeouts: [2]int{3, 3}, rng: rand.New(rand.NewSource(seed)), Lang: lang}
	// Qui guanya la moneda difereix: l'altre rep ara i ell a la segona part.
	recv := g.rng.Intn(2)
	g.Poss, g.recv2nd = recv, 1-recv
	g.kickNext = true
	return g
}

func (g *Game) st(id string) *Stat {
	if id == "" {
		return &Stat{}
	}
	s := g.Stats[id]
	if s == nil {
		s = &Stat{}
		g.Stats[id] = s
	}
	return s
}

func (g *Game) off() *Team { return g.Teams[g.Poss] }
func (g *Game) def() *Team { return g.Teams[1-g.Poss] }

func (g *Game) norm(mean, sd float64) float64 { return mean + sd*g.rng.NormFloat64() }

func clampi(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// pick tria un jugador ponderat per una funció (sempre en torna un si n'hi ha).
func (g *Game) pick(ps []*Player, w func(*Player) float64) *Player {
	if len(ps) == 0 {
		return nil
	}
	total := 0.0
	ws := make([]float64, len(ps))
	for i, p := range ps {
		ws[i] = math.Max(.01, w(p))
		total += ws[i]
	}
	r := g.rng.Float64() * total
	for i, p := range ps {
		r -= ws[i]
		if r <= 0 {
			return p
		}
	}
	return ps[len(ps)-1]
}

func id(p *Player) string {
	if p == nil {
		return ""
	}
	return p.ID
}

// tackler tria qui placa segons on acaba la jugada.
func (g *Game) tackler(yards int, deep bool) *Player {
	d := g.def()
	var pool []*Player
	switch {
	case deep || yards > 15:
		pool = append(d.all(S), d.all(CB)...)
	case yards > 5:
		pool = append(append(d.all(LB), d.all(S)...), d.all(CB)...)
	default:
		pool = append(d.all(DL), d.all(LB)...)
	}
	return g.pick(pool, func(p *Player) float64 { return float64(p.Attrs.Tackle) })
}

// situation: segons que queden a la part, i qui va per davant.
func (g *Game) halfLeft() int {
	if g.Quarter == 2 || g.Quarter == 4 {
		return g.Clock
	}
	if g.Quarter == 1 || g.Quarter == 3 {
		return g.Clock + 900
	}
	return g.Clock
}

func (g *Game) lead() int { return g.Score[g.Poss] - g.Score[1-g.Poss] }

// Over diu si el partit s'ha acabat.
func (g *Game) Finished() bool { return g.Over }

// SimQuarter juga fins al final del quart actual (o del partit).
func (g *Game) SimQuarter() []Play {
	start := len(g.Plays)
	q := g.Quarter
	for !g.Over && g.Quarter == q {
		g.step()
	}
	return g.Plays[start:]
}

// SimAll juga el partit sencer (proves i simulacions ràpides).
func (g *Game) SimAll() {
	for !g.Over {
		g.step()
	}
}

func (g *Game) push(p Play) *Play {
	p.N = len(g.Plays) + 1
	p.Quarter, p.Offense, p.Drive = g.Quarter, g.Poss, g.Drive
	g.Plays = append(g.Plays, p)
	return &g.Plays[len(g.Plays)-1]
}

// step és una jugada (o un xut) i el que en ve: rellotge, downs, canvis.
func (g *Game) step() {
	switch {
	case g.afterTD:
		g.conversion()
		return
	case g.kickNext:
		g.kickoff()
		return
	}
	g.scrimmage()
}

// ---- xut d'inici ----

func (g *Game) kickoff() {
	g.kickNext = false
	kicking := g.def() // Poss és qui rep
	k := kicking.at(K, 0)
	ret := g.pick(append(g.off().all(WR), g.off().all(RB)...), func(p *Player) float64 { return float64(p.Attrs.Speed) })
	clock := g.Clock
	p := Play{Kind: "kickoff", Spot: 35, Clock: clock, Actors: Actors{Kicker: id(k), Returner: id(ret)}}
	tb := .62 + float64(k.Attrs.Kick-70)/250
	end := 30
	used := 0
	if g.rng.Float64() < tb {
		p.Detail = "touchback"
		end = 30
	} else {
		retSkill := float64(ret.Attrs.Speed+ret.Attrs.Agility) / 2
		end = clampi(int(g.norm(26+(retSkill-75)/5, 7)), 8, 60)
		used = 6
		if g.rng.Float64() < .006+(retSkill-75)/4000 {
			p.TD, end = true, 100
		}
		p.Actors.Tackler = id(g.pick(append(kicking.all(LB), kicking.all(S)...), func(p *Player) float64 { return float64(p.Attrs.Tackle) }))
	}
	p.EndSpot, p.Yards = end, end
	g.Clock -= used
	g.Drive++
	if p.TD {
		p.Highlight = true
		p.Actors.Tackler = ""
		g.Score[g.Poss] += 6
		p.Score = g.Score
		p.Text = g.t(fmt.Sprintf("%s returns the kickoff all the way for a TOUCHDOWN!", nameOf(ret)), fmt.Sprintf("%s retorna el xut d'inici fins a la zona de marca: TOUCHDOWN!", nameOf(ret)))
		g.push(p)
		g.afterTD = true
		return
	}
	g.Spot, g.Down, g.Dist = end, 1, 10
	p.Score = g.Score
	if p.Detail == "touchback" {
		p.Text = g.t(fmt.Sprintf("%s kicks it deep: touchback, %s start at their %d.", nameOf(k), g.off().Name, end), fmt.Sprintf("%s xuta llarg: touchback, %s comencen a la seva %d.", nameOf(k), g.off().Name, end))
	} else {
		p.Text = g.t(fmt.Sprintf("%s returns the kickoff to the %s.", nameOf(ret), yardText(end, true)), fmt.Sprintf("%s retorna el xut fins a la %s.", nameOf(ret), yardText(end, false)))
	}
	g.push(p)
	g.endPlayClock(true)
}

// ---- conversió després de touchdown ----

func (g *Game) conversion() {
	g.afterTD = false
	pl := g.Plans[g.Poss]
	lead := g.lead() // ja inclou els 6
	two := false
	late := g.Quarter >= 4 && g.Clock < 600
	if late && (lead == -2 || lead == 1 || lead == 5 || lead == -10 || lead == -5) {
		two = true
	} else if g.rng.Float64() < .06*pl.Aggression {
		two = true
	}
	k := g.off().at(K, 0)
	p := Play{Spot: 98, Clock: g.Clock, Actors: Actors{Kicker: id(k)}}
	if two {
		p.Kind = "two"
		p.Spot = 98
		ok := g.rng.Float64() < .47
		p.Good = ok
		qb := g.off().at(QB, 0)
		p.Actors.Passer = id(qb)
		if ok {
			g.Score[g.Poss] += 2
			p.Text = g.t("Two-point conversion: GOOD!", "Conversió de dos punts: BONA!")
			p.Highlight = true
		} else {
			p.Text = g.t("Two-point try fails.", "La conversió de dos punts no entra.")
		}
	} else {
		p.Kind = "pat"
		ok := g.rng.Float64() < .935+float64(k.Attrs.Kick-75)/400
		p.Good = ok
		if ok {
			g.Score[g.Poss]++
			p.Text = g.t("Extra point is good.", "L'extra point entra.")
		} else {
			p.Text = g.t(nameOf(k)+" misses the extra point!", nameOf(k)+" falla l'extra point!")
			p.Highlight = true
		}
	}
	p.Score = g.Score
	g.push(p)
	g.changeAfterScore()
}

// changeAfterScore: qui ha marcat xuta.
func (g *Game) changeAfterScore() {
	if g.checkOTEnd() {
		return
	}
	g.Poss = 1 - g.Poss
	g.kickNext = true
}

// ---- jugada des de la línia ----

func (g *Game) scrimmage() {
	if g.Quarter >= 5 {
		g.otPoss[g.Poss] = true
	}
	o, d := g.off(), g.def()
	pl, dp := g.Plans[g.Poss], g.Plans[1-g.Poss]
	half := g.halfLeft()
	lead := g.lead()
	two := g.Quarter == 2 || g.Quarter >= 4

	// Ajupir-se per tancar el partit.
	if g.Quarter >= 4 && lead > 0 && g.Clock <= 120 && g.Down < 4 && float64(g.Clock) <= float64(4-g.Down)*40+5 && g.Timeouts[1-g.Poss] <= 1 {
		g.kneel()
		return
	}

	// Quarta: xutar, gol de camp o anar-hi.
	if g.Down == 4 {
		if g.fourthDown(pl, lead, half, two) {
			return
		}
	}

	// Gol de camp al final de la part.
	if two && g.Clock <= 8 && g.Spot >= 55 && (g.Quarter == 2 || lead >= -3) {
		g.fieldGoal()
		return
	}

	passRate := pl.PassRate
	switch {
	case g.Down == 3 && g.Dist >= 7:
		passRate = .88
	case g.Down == 3 && g.Dist <= 2, g.Down == 4 && g.Dist <= 2:
		passRate = .35
	case g.Dist >= 12:
		passRate += .2
	}
	if g.Quarter >= 4 && lead < 0 && g.Clock < 300 {
		passRate = math.Max(passRate, .82)
	}
	if g.Quarter >= 4 && lead > 7 && g.Clock < 480 {
		passRate = math.Min(passRate, .3)
	}
	if g.Quarter == 2 && g.Clock < 120 {
		passRate = math.Max(passRate, .75)
	}
	if g.Spot >= 98 {
		passRate = math.Min(passRate, .45)
	}
	blitz := g.rng.Float64() < dp.Blitz
	if g.rng.Float64() < passRate {
		g.pass(o, d, pl, blitz)
	} else {
		g.run(o, d, blitz)
	}
}

// fourthDown decideix la quarta; torna true si ja ha jugat (xut o gol).
func (g *Game) fourthDown(pl Plan, lead, half int, two bool) bool {
	fgDist := 100 - g.Spot + 17
	k := g.off().at(K, 0)
	maxFG := 52 + (k.Attrs.Kick-75)/5
	desperate := g.Quarter >= 4 && lead < 0 && (g.Clock < 240 || (lead < -8 && g.Clock < 420))
	goFor := false
	switch {
	case desperate:
		goFor = !(lead >= -3 && fgDist <= maxFG)
	case g.Dist <= 1 && g.Spot >= 45:
		goFor = g.rng.Float64() < .45+.5*pl.Aggression
	case g.Dist <= 2 && g.Spot >= 60:
		goFor = g.rng.Float64() < .25+.5*pl.Aggression
	case g.Dist <= 4 && g.Spot >= 65 && g.Spot < 70:
		goFor = g.rng.Float64() < .3*pl.Aggression
	}
	if g.Quarter >= 4 && lead > 0 && g.Clock < 120 && g.Spot < 60 {
		goFor = false
	}
	if goFor {
		return false
	}
	if fgDist <= maxFG && g.Spot >= 55 {
		g.fieldGoal()
		return true
	}
	g.punt()
	return true
}

func (g *Game) kneel() {
	qb := g.off().at(QB, 0)
	p := Play{Kind: "kneel", Down: g.Down, Dist: g.Dist, Spot: g.Spot, Clock: g.Clock, Yards: -1, EndSpot: g.Spot - 1, Actors: Actors{Passer: id(qb)}}
	p.Text = g.t(nameOf(qb)+" takes a knee.", nameOf(qb)+" s'ajup amb la pilota.")
	p.Score = g.Score
	g.push(p)
	g.advance(-1, false, false, &g.Plays[len(g.Plays)-1])
	g.runClock(2, true)
}

// run és una cursa.
func (g *Game) run(o, d *Team, blitz bool) {
	rb := o.at(RB, 0)
	qb := o.at(QB, 0)
	runner := rb
	if g.Dist <= 1 && g.rng.Float64() < .25 {
		runner = qb // QB sneak
	}
	dirs := []string{"left", "middle", "right"}
	dir := dirs[g.rng.Intn(3)]
	runBlock := avg(o.all(OL), func(a Attrs) int { return (a.Block*2 + a.Strength) / 3 })
	runDef := (avg(d.all(DL), func(a Attrs) int { return (a.Strength + a.Tackle) / 2 })*2 + avg(d.all(LB), func(a Attrs) int { return a.Tackle })) / 3
	skill := float64(runner.Attrs.Speed+runner.Attrs.Agility+runner.Attrs.Strength) / 3
	edge := (runBlock-runDef)/9 + (skill-75)/12
	if blitz {
		edge += .3 // un blitz deixa forats darrere… o atrapa
	}
	if g.Spot >= 80 {
		edge -= 1.2 // a prop de la zona de marca hi ha onze defensors en poc espai
	}
	var yds int
	r := g.rng.Float64()
	switch {
	case r < .19-edge/40:
		yds = -3 + g.rng.Intn(4) // aturat
	case r < .965:
		yds = int(math.Round(g.norm(4.2+edge*.9, 2.9)))
		if yds < 0 {
			yds = 0
		}
	default:
		yds = 12 + int(math.Abs(g.norm(0, 18))) + int(edge*2)
	}
	if runner == qb && g.Dist <= 1 {
		yds = int(math.Round(g.norm(1.6, 1.2)))
	}
	if g.Spot >= 99 {
		yds = int(math.Round(g.norm(.6, 1.4)))
	}
	p := Play{Kind: "run", Dir: dir, Down: g.Down, Dist: g.Dist, Spot: g.Spot, Clock: g.Clock, Actors: Actors{Runner: id(runner)}}
	if runner == qb {
		p.Detail = "sneak"
	}
	// Fumble.
	fumble := g.rng.Float64() < .011-float64(runner.Attrs.Hands-75)/12000
	g.finishCarry(&p, runner, yds, fumble, false)
}

// finishCarry acaba una jugada on algú porta la pilota (cursa o captura).
func (g *Game) finishCarry(p *Play, carrier *Player, yds int, fumble bool, isPass bool) {
	end := g.Spot + yds
	if end >= 100 {
		yds = 100 - g.Spot
		end = 100
		p.TD = true
	}
	if end <= 0 {
		end = 0
		p.Safety = true
	}
	p.Yards, p.EndSpot = yds, end
	st := g.st(carrier.ID)
	if isPass {
		st.Rec++
		st.RecYds += yds
	} else {
		st.RushAtt++
		st.RushYds += yds
	}
	if !p.TD && !p.Safety {
		t := g.tackler(yds, false)
		p.Actors.Tackler = id(t)
		g.st(id(t)).Tackles++
	}
	if fumble && !p.TD {
		p.Fumble, p.Turnover = true, true
		p.Highlight = true
		p.Text = g.t(fmt.Sprintf("%s FUMBLES! %s recover at the %s.", nameOf(carrier), g.def().Name, yardText(end, true)), fmt.Sprintf("%s perd la pilota! %s la recuperen a la %s.", nameOf(carrier), g.def().Name, yardText(end, false)))
		p.Score = g.Score
		g.push(*p)
		g.turnover(end)
		g.runClock(6, false)
		return
	}
	if p.TD {
		g.Score[g.Poss] += 6
		p.Highlight = true
		if isPass {
			st.RecTD++
			g.st(p.Actors.Passer).PassTD++
		} else {
			st.RushTD++
		}
	}
	if p.Safety {
		g.Score[1-g.Poss] += 2
		p.Highlight = true
	}
	p.Score = g.Score
	if p.Text == "" {
		p.Text = g.describe(p, carrier, isPass)
	}
	g.push(*p)
	last := &g.Plays[len(g.Plays)-1]
	if p.TD {
		g.runClock(6, false)
		g.afterTD = true
		return
	}
	if p.Safety {
		g.runClock(6, false)
		g.safetyKick()
		return
	}
	outOfBounds := g.rng.Float64() < .14
	g.advance(yds, outOfBounds, false, last)
	g.runClock(6, !outOfBounds || !(g.Quarter == 4 && g.Clock < 300 || g.Quarter == 2 && g.Clock < 120))
}

// pass és una passada (amb el sack, l'escapada i la intercepció).
func (g *Game) pass(o, d *Team, pl Plan, blitz bool) {
	qb := o.at(QB, 0)
	passBlock := avg(o.all(OL), func(a Attrs) int { return a.Block })
	rush := avg(d.all(DL), func(a Attrs) int { return (a.Speed + a.Strength) / 2 })
	sackP := .072 + (rush-passBlock)/220 - float64(qb.Attrs.Awareness-75)/700
	if g.Down == 3 && g.Dist >= 5 {
		sackP += .02
	}
	if blitz {
		sackP += .035
	}
	p := Play{Down: g.Down, Dist: g.Dist, Spot: g.Spot, Clock: g.Clock, Actors: Actors{Passer: id(qb)}}
	if g.rng.Float64() < sackP {
		dl := g.pick(append(d.all(DL), d.all(LB)...), func(p *Player) float64 { return float64(p.Attrs.Speed + p.Attrs.Strength) })
		loss := -(4 + g.rng.Intn(6))
		p.Kind = "sack"
		p.Actors.Defender = id(dl)
		g.st(dl.ID).Sacks++
		g.st(dl.ID).Tackles++
		end := g.Spot + loss
		p.Yards, p.EndSpot = loss, end
		fumble := g.rng.Float64() < .09
		if end <= 0 {
			p.EndSpot, p.Safety = 0, true
			g.Score[1-g.Poss] += 2
			p.Highlight = true
			p.Text = g.t(fmt.Sprintf("%s sacks %s in the end zone: SAFETY!", nameOf(dl), nameOf(qb)), fmt.Sprintf("%s fa un sack a %s dins la zona de marca: SAFETY!", nameOf(dl), nameOf(qb)))
			p.Score = g.Score
			g.push(p)
			g.runClock(6, false)
			g.safetyKick()
			return
		}
		if fumble {
			p.Fumble, p.Turnover, p.Highlight = true, true, true
			p.Text = g.t(fmt.Sprintf("Strip-sack by %s! %s recover.", nameOf(dl), d.Name), fmt.Sprintf("Sack de %s i la pilota cau! La recuperen %s.", nameOf(dl), d.Name))
			p.Score = g.Score
			g.push(p)
			g.turnover(end)
			g.runClock(6, false)
			return
		}
		p.Text = g.t(fmt.Sprintf("SACK! %s brings down %s for a loss of %d.", nameOf(dl), nameOf(qb), -loss), fmt.Sprintf("SACK! %s tomba %s i perden %d iardes.", nameOf(dl), nameOf(qb), -loss))
		if -loss >= 8 {
			p.Highlight = true
		}
		p.Score = g.Score
		g.push(p)
		g.advance(loss, false, false, &g.Plays[len(g.Plays)-1])
		g.runClock(6, true)
		return
	}
	// Escapada del QB.
	if g.rng.Float64() < .035+float64(qb.Attrs.Speed-70)/600 {
		p.Kind = "scramble"
		yds := int(math.Round(g.norm(5+float64(qb.Attrs.Speed-70)/6, 4)))
		g.finishCarry(&p, qb, yds, false, false)
		return
	}
	// A qui i com de lluny.
	targets := append(append(o.all(WR), o.all(TE)...), o.all(RB)...)
	tgt := g.pick(targets, func(t *Player) float64 {
		w := float64(t.Attrs.Hands+t.Attrs.Speed) / 2
		switch t.Pos {
		case WR:
			w *= 1.25
		case RB:
			w *= .55
		}
		if pl.Focus != "" && t.ID == pl.Focus {
			w *= 2.2
		}
		return w
	})
	r := g.rng.Float64()
	var depth int
	kind := "short"
	switch {
	case tgt.Pos == RB && r < .6, r < .07:
		kind, depth = "screen", -2+g.rng.Intn(3)
	case r < 1-pl.DeepShots-.25:
		kind, depth = "short", 3+g.rng.Intn(7)
	case r < 1-pl.DeepShots:
		kind, depth = "medium", 10+g.rng.Intn(9)
	default:
		kind, depth = "deep", 20+g.rng.Intn(18)
	}
	if g.Spot+depth >= 100 {
		depth = 99 - g.Spot
		if depth < 1 {
			depth = 1
		}
		kind = "short"
	}
	cov := g.pick(append(d.all(CB), d.all(S)...), func(c *Player) float64 { return float64(c.Attrs.Coverage) })
	sep := float64(tgt.Attrs.Speed+tgt.Attrs.Agility)/2 - float64(cov.Attrs.Coverage+cov.Attrs.Speed)/2
	base := map[string]float64{"screen": .8, "short": .7, "medium": .52, "deep": .33}[kind]
	compP := base + float64(qb.Attrs.Accuracy-75)/220 + sep/260 + float64(tgt.Attrs.Hands-75)/350
	if blitz {
		compP -= .05
	}
	// 3a i llarga: la defensa cobreix el que cal i s'apreta.
	if g.Down == 3 && g.Dist >= 5 {
		compP -= .1
	} else if g.Down == 3 && g.Dist >= 3 {
		compP -= .05
	}
	// Zona vermella: el camp s'escurça i les finestres són petites.
	if g.Spot >= 80 {
		compP -= .1
	}
	intP := map[string]float64{"screen": .006, "short": .014, "medium": .026, "deep": .052}[kind] - float64(qb.Attrs.Awareness-75)/2500 + float64(cov.Attrs.Hands-60)/4000
	p.Kind, p.Detail, p.Depth = "pass", kind, depth
	p.Dir = []string{"left", "middle", "right"}[g.rng.Intn(3)]
	p.Actors.Target = id(tgt)
	st := g.st(qb.ID)
	st.PassAtt++
	if g.rng.Float64() < intP {
		p.Intercept, p.Turnover, p.Highlight = true, true, true
		p.Actors.Defender = id(cov)
		st.Ints++
		g.st(cov.ID).DefInt++
		ret := clampi(int(math.Abs(g.norm(8, 12))), 0, 80)
		catchSpot := clampi(g.Spot+depth, 1, 99)
		newSpot := 100 - catchSpot + ret // per a la defensa
		if newSpot >= 100 {
			// Pick six.
			p.Yards, p.EndSpot = depth, catchSpot
			p.Text = g.t(fmt.Sprintf("INTERCEPTED by %s and he takes it back to the house! PICK SIX!", nameOf(cov)), fmt.Sprintf("INTERCEPCIÓ de %s, que se'n va fins a la zona de marca! PICK SIX!", nameOf(cov)))
			g.Poss = 1 - g.Poss
			g.Score[g.Poss] += 6
			p.TD = true
			p.Score = g.Score
			g.push(p)
			g.runClock(8, false)
			g.Drive++
			g.afterTD = true
			return
		}
		p.Yards, p.EndSpot = depth, catchSpot
		p.Text = g.t(fmt.Sprintf("INTERCEPTED! %s jumps the route on %s's throw for %s.", nameOf(cov), nameOf(qb), nameOf(tgt)), fmt.Sprintf("INTERCEPCIÓ! %s s'avança a la passada de %s per a %s.", nameOf(cov), nameOf(qb), nameOf(tgt)))
		p.Score = g.Score
		g.push(p)
		g.Poss = 1 - g.Poss
		g.Spot, g.Down, g.Dist = clampi(newSpot, 1, 99), 1, 10
		g.Drive++
		g.runClock(7, false)
		return
	}
	if g.rng.Float64() < compP {
		p.Complete = true
		st.PassComp++
		yac := 0
		switch kind {
		case "screen":
			yac = int(math.Round(g.norm(6, 5)))
		case "short":
			yac = int(math.Round(g.norm(2.8, 2.6)))
		case "medium":
			yac = int(math.Round(g.norm(2.2, 2.6)))
		case "deep":
			yac = int(math.Round(g.norm(2.5, 4)))
		}
		if yac < 0 {
			yac = 0
		}
		if g.rng.Float64() < .022+sep/2000 {
			yac += 8 + int(math.Abs(g.norm(0, 16)))
		}
		yds := depth + yac
		st.PassYds += yds
		if yds >= 25 {
			p.Highlight = true
		}
		fumble := g.rng.Float64() < .008
		g.finishCarry(&p, tgt, yds, fumble, true)
		return
	}
	// Incompleta (de vegades amb penal de la defensa).
	p.Yards, p.EndSpot = 0, g.Spot
	if (kind == "deep" || kind == "medium") && g.rng.Float64() < .045 {
		gain := depth
		if gain > 40 {
			gain = 40
		}
		if g.Spot+gain >= 100 {
			gain = 99 - g.Spot
		}
		p.Penalty = "dpi"
		p.Yards, p.EndSpot = gain, g.Spot+gain
		p.Text = g.t(fmt.Sprintf("Flag! Pass interference on %s: %d yards and a first down.", nameOf(cov), gain), fmt.Sprintf("Mocador! Interferència de %s: %d iardes i primer down.", nameOf(cov), gain))
		p.Score = g.Score
		g.push(p)
		g.advance(gain, false, true, &g.Plays[len(g.Plays)-1])
		g.runClock(5, false)
		return
	}
	p.Text = g.t(fmt.Sprintf("Incomplete: %s's pass for %s falls to the turf.", nameOf(qb), nameOf(tgt)), fmt.Sprintf("Incompleta: la passada de %s per a %s cau a terra.", nameOf(qb), nameOf(tgt)))
	p.Actors.Defender = id(cov)
	p.Score = g.Score
	g.push(p)
	g.advance(0, false, false, &g.Plays[len(g.Plays)-1])
	g.runClock(5, false)
}

// advance mou la pilota i els downs després d'una jugada normal.
func (g *Game) advance(yds int, oob bool, autoFirst bool, last *Play) {
	g.Spot = clampi(g.Spot+yds, 1, 99)
	if autoFirst || yds >= g.Dist {
		g.Down, g.Dist = 1, 10
		if 100-g.Spot < 10 {
			g.Dist = 100 - g.Spot
		}
		if last != nil {
			last.FirstDown = true
		}
		return
	}
	g.Dist -= yds
	g.Down++
	if g.Down > 4 {
		if last != nil {
			last.Turnover = true
			last.Text += g.t(" Turnover on downs.", " Perden la pilota per downs.")
		}
		g.turnover(g.Spot)
	}
}

// turnover dona la pilota a l'altre equip on és ara.
func (g *Game) turnover(spot int) {
	g.Poss = 1 - g.Poss
	g.Spot = clampi(100-spot, 1, 99)
	g.Down, g.Dist = 1, 10
	if 100-g.Spot < 10 {
		g.Dist = 100 - g.Spot
	}
	g.Drive++
	g.checkOTEnd()
}

func (g *Game) punt() {
	k := g.off().at(K, 0)
	ret := g.pick(g.def().all(WR), func(p *Player) float64 { return float64(p.Attrs.Speed) })
	gross := int(math.Round(g.norm(46+float64(k.Attrs.Kick-75)/4, 6)))
	land := g.Spot + gross
	p := Play{Kind: "punt", Down: g.Down, Dist: g.Dist, Spot: g.Spot, Clock: g.Clock, Actors: Actors{Kicker: id(k), Returner: id(ret)}}
	var newSpot int
	switch {
	case land >= 100:
		newSpot = 20
		p.Detail = "touchback"
		p.EndSpot = 100
		p.Text = g.t(nameOf(k)+" punts into the end zone: touchback.", nameOf(k)+" xuta fins a la zona de marca: touchback.")
	case land >= 90 || g.rng.Float64() < .3:
		newSpot = 100 - land
		p.Detail = "fair"
		p.EndSpot = land
		p.Text = g.t(fmt.Sprintf("%s punts %d yards; %s fair catch at the %s.", nameOf(k), gross, nameOf(ret), yardText(100-land, true)), fmt.Sprintf("%s xuta %d iardes; %s la para a la %s.", nameOf(k), gross, nameOf(ret), yardText(100-land, false)))
	default:
		r := clampi(int(math.Round(g.norm(9, 6))), 0, 60)
		if g.rng.Float64() < .006 {
			r = land // retorn fins a marcar
		}
		newSpot = 100 - land + r
		p.EndSpot = land
		if newSpot >= 100 {
			p.TD, p.Highlight = true, true
			p.Text = g.t(fmt.Sprintf("PUNT RETURN TOUCHDOWN by %s!", nameOf(ret)), fmt.Sprintf("%s retorna el punt fins a marcar: TOUCHDOWN!", nameOf(ret)))
		} else {
			p.Text = g.t(fmt.Sprintf("%s punts %d yards, returned %d by %s.", nameOf(k), gross, r, nameOf(ret)), fmt.Sprintf("%s xuta %d iardes i %s en retorna %d.", nameOf(k), gross, nameOf(ret), r))
		}
	}
	p.Yards = gross
	g.Poss = 1 - g.Poss
	g.Drive++
	if p.TD {
		g.Score[g.Poss] += 6
		p.Score = g.Score
		g.push(p)
		g.runClock(10, false)
		g.afterTD = true
		return
	}
	g.Spot, g.Down, g.Dist = clampi(newSpot, 1, 99), 1, 10
	p.Score = g.Score
	g.push(p)
	g.runClock(10, false)
	g.checkOTEnd()
}

func (g *Game) fieldGoal() {
	k := g.off().at(K, 0)
	dist := 100 - g.Spot + 17
	var pr float64
	switch {
	case dist < 30:
		pr = .975
	case dist < 40:
		pr = .92
	case dist < 50:
		pr = .82
	case dist < 55:
		pr = .68
	default:
		pr = .48 - float64(dist-55)*.04
	}
	pr += float64(k.Attrs.Kick-75) / 300
	ok := g.rng.Float64() < pr
	p := Play{Kind: "fg", Down: g.Down, Dist: g.Dist, Spot: g.Spot, Clock: g.Clock, Depth: dist, Good: ok, Actors: Actors{Kicker: id(k)}}
	st := g.st(k.ID)
	st.FGA++
	if ok {
		st.FGM++
		g.Score[g.Poss] += 3
		p.Score = g.Score
		p.Highlight = dist >= 50 || (g.Quarter >= 4 && g.Clock < 30)
		p.Text = g.t(fmt.Sprintf("%s's %d-yard field goal is GOOD.", nameOf(k), dist), fmt.Sprintf("El gol de camp de %d iardes de %s entra.", dist, nameOf(k)))
		g.push(p)
		g.runClock(5, false)
		g.changeAfterScore()
		return
	}
	p.Score = g.Score
	p.Highlight = true
	p.Text = g.t(fmt.Sprintf("%s's %d-yard attempt is NO GOOD.", nameOf(k), dist), fmt.Sprintf("%s falla el gol de camp de %d iardes.", nameOf(k), dist))
	g.push(p)
	g.runClock(5, false)
	spot := g.Spot - 7
	if spot < 20 {
		spot = 20
	}
	g.turnover(spot)
}

// safetyKick: després d'un safety, xuta qui l'ha patit (des de la seva 20).
func (g *Game) safetyKick() {
	if g.checkOTEnd() {
		return
	}
	g.Poss = 1 - g.Poss
	g.kickNext = true
}

// ---- rellotge ----

// runClock gasta el temps d'una jugada i, si el rellotge segueix corrent,
// el temps fins a la següent. Gestiona l'avís dels dos minuts, els temps
// morts i el final dels quarts.
func (g *Game) runClock(playSecs int, running bool) {
	g.Clock -= playSecs
	if running && g.Clock > 0 {
		gap := 36
		pl := g.Plans[g.Poss]
		gap -= int(pl.Tempo * 12)
		lead := g.lead()
		late := g.Quarter == 4 || g.Quarter == 2
		if late && g.Clock < 150 && lead < 0 {
			gap = 14 // atac sense pausa
			if g.Timeouts[g.Poss] > 0 && g.Clock < 120 {
				g.Timeouts[g.Poss]--
				gap = 0
			}
		}
		if g.Quarter == 4 && lead > 0 && g.Clock < 180 {
			gap = 40
			if g.Timeouts[1-g.Poss] > 0 {
				g.Timeouts[1-g.Poss]--
				gap = 0
			}
		}
		if (g.Quarter == 2 || g.Quarter == 4) && !g.warned && g.Clock > 120 && g.Clock-gap <= 120 {
			gap = g.Clock - 120
			g.warned = true
		}
		g.Clock -= gap
	}
	if (g.Quarter == 2 || g.Quarter == 4) && !g.warned && g.Clock <= 120 && g.Clock > 0 {
		g.warned = true
	}
	if g.Clock <= 0 {
		g.Clock = 0
		g.endQuarter()
	}
}

func (g *Game) endPlayClock(_ bool) {
	if g.Clock <= 0 {
		g.endQuarter()
	}
}

func (g *Game) endQuarter() {
	if g.afterTD {
		// El PAT es xuta igualment, amb el rellotge a zero.
		g.conversion()
	}
	switch {
	case g.Quarter == 2:
		g.Quarter, g.Clock = 3, 900
		g.Timeouts = [2]int{3, 3}
		g.warned = false
		g.Poss = g.recv2nd
		g.kickNext = true
		g.Drive++
	case g.Quarter == 4:
		if g.Score[0] != g.Score[1] {
			g.Over = true
			return
		}
		g.Quarter, g.Clock = 5, 600
		g.Timeouts = [2]int{2, 2}
		g.Poss = g.rng.Intn(2)
		g.kickNext = true
		g.Drive++
	case g.Quarter >= 5:
		g.Over = true // empat després de la pròrroga
	default:
		g.Quarter++
		g.Clock = 900
	}
}

// checkOTEnd aplica la pròrroga (regla NFL actual): tots dos equips tenen
// la pilota com a mínim un cop; després, qui marqui primer guanya. Es mira
// després de cada punt i de cada canvi de possessió.
func (g *Game) checkOTEnd() bool {
	if g.Quarter < 5 || g.Over {
		return false
	}
	if g.Score[0] != g.Score[1] && g.otPoss[0] && g.otPoss[1] {
		g.Over = true
		return true
	}
	return false
}

// ---- text ----

func (g *Game) t(en, ca string) string {
	if g.Lang == "ca" {
		return ca
	}
	return en
}

func nameOf(p *Player) string {
	if p == nil {
		return "?"
	}
	return p.Name
}

// yardText: «own 25», «opponent 30», «50».
func yardText(spot int, en bool) string {
	switch {
	case spot == 50:
		if en {
			return "50"
		}
		return "50"
	case spot < 50:
		if en {
			return fmt.Sprintf("own %d", spot)
		}
		return fmt.Sprintf("seva %d", spot)
	default:
		if en {
			return fmt.Sprintf("opponent %d", 100-spot)
		}
		return fmt.Sprintf("%d rival", 100-spot)
	}
}

func (g *Game) describe(p *Play, carrier *Player, isPass bool) string {
	y := p.Yards
	switch {
	case p.TD && isPass:
		return g.t(fmt.Sprintf("TOUCHDOWN! %s finds %s for %d yards!", nameOf(g.player(p.Actors.Passer)), nameOf(carrier), y), fmt.Sprintf("TOUCHDOWN! %s troba %s per %d iardes!", nameOf(g.player(p.Actors.Passer)), nameOf(carrier), y))
	case p.TD:
		return g.t(fmt.Sprintf("TOUCHDOWN! %s runs it in from %d yards!", nameOf(carrier), y), fmt.Sprintf("TOUCHDOWN! %s entra corrent des de %d iardes!", nameOf(carrier), y))
	case p.Safety:
		return g.t(fmt.Sprintf("%s is tackled in the end zone: SAFETY!", nameOf(carrier)), fmt.Sprintf("%s cau dins la seva zona de marca: SAFETY!", nameOf(carrier)))
	case isPass:
		return g.t(fmt.Sprintf("%s to %s for %d yards.", nameOf(g.player(p.Actors.Passer)), nameOf(carrier), y), fmt.Sprintf("%s per a %s, %d iardes.", nameOf(g.player(p.Actors.Passer)), nameOf(carrier), y))
	case p.Kind == "scramble":
		return g.t(fmt.Sprintf("%s scrambles for %d.", nameOf(carrier), y), fmt.Sprintf("%s s'escapa corrent: %d iardes.", nameOf(carrier), y))
	case y < 0:
		return g.t(fmt.Sprintf("%s is stopped for a loss of %d.", nameOf(carrier), -y), fmt.Sprintf("%s queda aturat i perd %d.", nameOf(carrier), -y))
	case y >= 15:
		return g.t(fmt.Sprintf("%s breaks free up the %s for %d yards!", nameOf(carrier), p.Dir, y), fmt.Sprintf("%s s'escapa per la %s: %d iardes!", nameOf(carrier), dirCA(p.Dir), y))
	default:
		return g.t(fmt.Sprintf("%s runs %s for %d.", nameOf(carrier), p.Dir, y), fmt.Sprintf("%s corre per la %s: %d.", nameOf(carrier), dirCA(p.Dir), y))
	}
}

func dirCA(d string) string {
	switch d {
	case "left":
		return "esquerra"
	case "right":
		return "dreta"
	}
	return "banda del mig"
}

func (g *Game) player(pid string) *Player {
	for _, t := range g.Teams {
		for i := range t.Players {
			if t.Players[i].ID == pid {
				return &t.Players[i]
			}
		}
	}
	return nil
}

// Highlights torna les jugades destacades d'un tram.
func Highlights(plays []Play) []Play {
	var out []Play
	for _, p := range plays {
		if p.Highlight {
			out = append(out, p)
		}
	}
	return out
}

// Leaders són els millors de cada apartat d'un partit (per al resum).
type Leaders struct {
	Passer, Rusher, Receiver, Defender string
}

// BoxLeaders troba els líders d'un equip.
func (g *Game) BoxLeaders(team int) Leaders {
	t := g.Teams[team]
	var l Leaders
	type kv struct {
		id string
		v  int
	}
	best := func(f func(*Stat) int) string {
		var all []kv
		for _, p := range t.Players {
			if s, ok := g.Stats[p.ID]; ok {
				all = append(all, kv{p.ID, f(s)})
			}
		}
		sort.Slice(all, func(i, j int) bool { return all[i].v > all[j].v })
		if len(all) == 0 || all[0].v <= 0 {
			return ""
		}
		return all[0].id
	}
	l.Passer = best(func(s *Stat) int { return s.PassYds })
	l.Rusher = best(func(s *Stat) int { return s.RushYds })
	l.Receiver = best(func(s *Stat) int { return s.RecYds })
	l.Defender = best(func(s *Stat) int { return s.Sacks*3 + s.DefInt*4 + s.Tackles })
	return l
}

// SetPlan posa el pla d'un entrenador (net de valors absurds).
func (g *Game) SetPlan(team int, p Plan) { g.Plans[team] = p.clamp() }
