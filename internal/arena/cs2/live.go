package cs2

// El mode en directe: una persona juga un dels jugadors del seu equip amb
// teclat i ratolí, i els altres nou són bots autònoms que es mouen i disparen
// al navegador (arena-cs2-live.js). El model continua fent el que fa sempre:
// abans de cada ronda llegeix el .md de cada equip i la situació i tria la
// compra i el pla; aquí, a més, pot donar un rol a cada bot del seu equip.
//
// El servidor és l'àrbitre de les regles: els diners, la bonificació per
// derrota, les recompenses, les meitats i l'historial que llegeix el model
// són els del partit normal. El navegador simula la ronda i en torna el
// registre (pistes, morts, utilitat, bomba); Finish el passa per les mateixes
// funcions del motor (kill, plant, defuse, finishRound), de manera que la
// ronda queda com qualsevol altra i el partit es pot tornar a mirar al visor.

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"gregal/internal/arena"
)

// Rols dels bots: com juguen la ronda dins del pla de l'equip.
// follow: es queda amb la persona (només l'equip on juga la persona).
var botRoles = []string{"entry", "support", "lurker", "awper", "anchor", "rifler", "follow"}

// El rol per defecte de cada posició, segons el bàndol.
func defaultBotRole(role string, side int) string {
	switch role {
	case "AWPer":
		return "awper"
	case "Entry":
		return "entry"
	case "Lurker":
		return "lurker"
	case "Support":
		if side == sideCT {
			return "anchor"
		}
		return "support"
	}
	if side == sideCT {
		return "anchor"
	}
	return "rifler"
}

// Live és un partit en directe.
type Live struct {
	m         *Match
	r         *rnd
	human     *Player
	roles     [2][5]string // rols fixats per a tot el partit ("" = el de la posició)
	notes     [2][5]string
	setup     *LiveSetup
	lastRoles map[string]string
}

// NewLive prepara un partit en directe. half: rondes per meitat (12 = MR12,
// 6 = el format curt); humanTeam/humanSlot: el jugador de la persona.
func NewLive(seed int64, lang string, names [2]string, half, humanTeam, humanSlot int) *Live {
	m := newMatch(seed, lang, names)
	if half >= 3 && half <= 12 {
		m.H = half
	}
	humanTeam = max(0, min(1, humanTeam))
	humanSlot = max(0, min(4, humanSlot))
	return &Live{m: m, human: m.players[humanTeam][humanSlot]}
}

// Match: el partit de sota (marcador, guanyador, meta).
func (l *Live) Match() arena.Match { return l.m }

// HumanID: l'id del jugador de la persona («a0»..«b4»).
func (l *Live) HumanID() string { return l.human.ID }

// HumanTeam: l'equip de la persona.
func (l *Live) HumanTeam() int { return l.human.Team }

// Done: el partit s'ha acabat.
func (l *Live) Done() bool { return l.m.done }

// SetRoles fixa el rol (i una nota) de cada bot d'un equip per a tot el partit.
func (l *Live) SetRoles(team int, roles, notes []string) {
	if team < 0 || team > 1 {
		return
	}
	for i := 0; i < 5; i++ {
		l.roles[team][i], l.notes[team][i] = "", ""
		if i < len(roles) {
			if r := pick(roles[i], botRoles, roleAlias); r != "" {
				l.roles[team][i] = r
			}
		}
		if i < len(notes) {
			l.notes[team][i] = oneLineCS(notes[i], 140)
			if r := NoteRole(notes[i]); r != "" && team == l.human.Team {
				l.roles[team][i] = r
			}
		}
	}
}

var roleAlias = map[string]string{"awp": "awper", "sniper": "awper", "entry_fragger": "entry", "lurk": "lurker", "flex": "rifler", "rotator": "rifler", "igl": "rifler", "anchor_site": "anchor", "util": "support", "utility": "support",
	"follow_me": "follow", "follower": "follow", "escort": "follow", "bodyguard": "follow", "guard": "follow", "cover_me": "follow", "stick_with_me": "follow", "segueix_me": "follow", "segueixme": "follow", "sigueme": "follow", "sígueme": "follow"}

// followNote: una instrucció que demana que el bot vagi amb la persona
// («follow me», «stick with me», «segueix-me», «sígueme»…).
var followNote = regexp.MustCompile(`(?i)\b(follow|stick|stay|cover|escort|protect|guard)\b.{0,12}\b(me|my|human|player)\b|segueix[- ]?me|acompanya[- ]?m|cobreix[- ]?me|v[eé]s amb mi|s[ií]gueme|sigue ?me|c[uú]breme|qu[eé]date conmigo`)

// NoteRole: el rol que una instrucció demana sense dubte ("" si cap).
func NoteRole(note string) string {
	if followNote.MatchString(note) {
		return "follow"
	}
	return ""
}

func oneLineCS(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		s = string(r[:n])
	}
	return s
}

// Prompts: les dues decisions de la propera ronda (el mateix que Next, amb
// el camp opcional «roles» i qui juga la persona).
func (l *Live) Prompts() (p [2]arena.Prompt, done bool) {
	p, done = l.m.Next()
	if done {
		return p, true
	}
	for t := 0; t < 2; t++ {
		side := sideOf(l.m, t)
		var names []string
		for _, pl := range l.m.players[t] {
			if pl == l.human {
				continue
			}
			names = append(names, fmt.Sprintf("%q (%s, now %s)", pl.Handle, pl.Role, l.roleOf(pl, side, nil)))
		}
		extra := "\nLIVE MODE: the round is played in real time by bots"
		if t == l.human.Team {
			extra += fmt.Sprintf(" and by a human, who plays %s (%s) and follows the plan as he sees fit", l.human.Handle, l.human.Role)
		}
		roles := "entry|support|lurker|awper|anchor|rifler"
		follow := ""
		if t == l.human.Team {
			roles += "|follow"
			follow = "; follow = stays with your human player, covers them and trades their duels (use it when the human's instructions or your strategy say \"follow me\", \"stick with me\", \"cover me\", \"segueix-me\", \"sígueme\" or similar)"
		}
		extra += `. Optional: "roles": {"<handle>": "` + roles + `"} to change how your bots play this round (entry = first in, peeks; support = follows the entry, throws utility, trades; lurker = plays apart, flanks late / CT: holds a lonely spot; awper = holds long angles; anchor = holds the site and never rotates early; rifler = standard` + follow + `). Your bots: ` + strings.Join(names, ", ") + "."
		p[t].Schema += extra
		if t == l.human.Team {
			var ns []string
			for i, pl := range l.m.players[t] {
				if n := l.notes[t][i]; n != "" && pl != l.human {
					ns = append(ns, pl.Handle+": "+n)
				}
			}
			if len(ns) > 0 {
				p[t].Situation += "Your human player's instructions for the bots: " + strings.Join(ns, " | ") + "\n"
			}
		}
	}
	return p, false
}

func (l *Live) roleOf(p *Player, side int, over map[string]string) string {
	// seguir la persona només té sentit al seu equip
	ok := func(r string) bool { return r != "" && (r != "follow" || p.Team == l.human.Team) }
	if r := over[p.Handle]; ok(r) {
		return r
	}
	if r := l.roles[p.Team][p.Slot]; ok(r) {
		return r
	}
	return defaultBotRole(p.Role, side)
}

// LivePlayer: un jugador al començar la ronda, amb les ordres del seu bot.
type LivePlayer struct {
	ID      string   `json:"id"`
	Handle  string   `json:"handle"`
	Role    string   `json:"role"`
	Team    int      `json:"team"`
	Side    string   `json:"side"`
	Human   bool     `json:"human,omitempty"`
	X       float64  `json:"x"`
	Y       float64  `json:"y"`
	Primary string   `json:"primary"`
	Pistol  string   `json:"pistol"`
	Armor   int      `json:"armor"`
	Kit     bool     `json:"kit,omitempty"`
	Nades   []string `json:"nades"`
	Money   int      `json:"money"`
	Aim     float64  `json:"aim"`
	// Ordres: el rol del bot, el grup del pla, la zona on espera (T) o
	// defensa (CT), el camí fins allà i quan surt.
	Bot   string   `json:"bot"`
	Note  string   `json:"note,omitempty"`
	Group string   `json:"group"`
	Post  string   `json:"post"`
	Route []string `json:"route"`
	Start float64  `json:"start"`
}

// LiveSetup: la ronda que el navegador ha de jugar.
type LiveSetup struct {
	Round   int          `json:"round"`
	Half    string       `json:"half"`
	TTeam   int          `json:"tTeam"`
	Pistol  bool         `json:"pistol"`
	Score   [2]int       `json:"score"`
	Target  int          `json:"target"`
	T       tDec         `json:"t"`
	CT      ctDec        `json:"ct"`
	Site    string       `json:"site"`  // el lloc que ataquen els T
	Fake    string       `json:"fake"`  // on fan la finta ("" = cap)
	HitAt   float64      `json:"hitAt"` // segons des de l'inici fins que entren
	FakeAt  float64      `json:"fakeAt,omitempty"`
	Bomb    string       `json:"bomb"`
	Human   string       `json:"human"`
	Players []LivePlayer `json:"players"`
	// T-side tactical routes into each site from the stage zones, and the
	// zones the CTs must cross to rotate (from the engine's path tables).
	Routes map[string][]string `json:"routes"`
}

// Start tria la compra i el pla amb les respostes del model i prepara la
// ronda. Fins que no arriba Finish no hi ha esdeveniments.
func (l *Live) Start(ans [2]json.RawMessage) (*LiveSetup, error) {
	m := l.m
	if m.done {
		return nil, fmt.Errorf("match over")
	}
	if l.setup != nil {
		return l.setup, nil // la mateixa ronda (un segon clic, una recàrrega)
	}
	if !m.pending {
		m.Next()
	}
	m.pending = false
	tt := m.tTeam
	td := m.parseT(ans[tt], tt)
	cd := m.parseCT(ans[1-tt], 1-tt)
	over := map[string]string{}
	for t := 0; t < 2; t++ {
		j := rawMap(ans[t])
		if rm, ok := j["roles"].(map[string]any); ok {
			for h, v := range rm {
				if s, ok := v.(string); ok {
					if r := pick(s, botRoles, roleAlias); r != "" {
						over[h] = r
					}
				}
			}
		}
	}
	l.lastRoles = over
	r := &rnd{m: m, r: m.rng, num: m.nextRound(), tTeam: tt, td: td, cd: cd, tx: textsFor(m.lang), posts: map[string]int{}}
	ct := 1 - tt
	tps, cps := m.teamPlayers(tt), m.teamPlayers(ct)
	// La persona compra ella mateixa (al menú de compra); els bots, segons el pla.
	without := func(ps []*Player) []*Player {
		var o []*Player
		for _, p := range ps {
			if p != l.human {
				o = append(o, p)
			}
		}
		return o
	}
	buy(without(tps), sideT, td.Buy, td.Utility, td.AWP)
	buy(without(cps), sideCT, cd.Buy, cd.Utility, cd.AWP)
	for i, p := range tps {
		x, y := spot("t_spawn", r.r, 0.6)
		r.T = append(r.T, &unit{p: p, side: sideT, alive: true, zone: "t_spawn", wp: []wpt{{0, x, y}}, idx: i})
	}
	for i, p := range cps {
		x, y := spot("ct_spawn", r.r, 0.6)
		r.CT = append(r.CT, &unit{p: p, side: sideCT, alive: true, zone: "ct_spawn", wp: []wpt{{0, x, y}}, idx: i})
	}
	// La bomba: a un bot T (la persona la pot recollir del terra).
	var carriers []*unit
	for _, u := range r.T {
		if u.p != l.human {
			carriers = append(carriers, u)
		}
	}
	r.carrier = carriers[r.r.Intn(len(carriers))]
	l.r = r

	s := &LiveSetup{Round: r.num, Half: m.halfName(r.num), TTeam: tt, Pistol: m.isPistol(), Score: m.score, Target: m.target(),
		T: td, CT: cd, Bomb: r.carrier.p.ID, Human: l.human.ID, Routes: map[string][]string{}}
	// El pla T, com al motor: zones d'espera, lloc, finta i hora d'entrar.
	site, fake, stage, group := tStages(td.Strategy)
	paceK := map[string]float64{"fast": 0.72, "default": 1, "slow": 1.3}[td.Pace]
	jit := func(v float64) float64 { return (r.rng() - 0.5) * 2 * v }
	switch {
	case strings.HasPrefix(td.Strategy, "rush"):
		s.HitAt = 1
	case strings.HasPrefix(td.Strategy, "execute"):
		s.HitAt = 30*paceK + jit(6)
	case strings.HasPrefix(td.Strategy, "split"):
		s.HitAt = 35*paceK + jit(6)
	case strings.HasPrefix(td.Strategy, "fake"):
		s.HitAt = 34*paceK + jit(5)
		s.FakeAt = s.HitAt - 14*paceK
	default:
		s.HitAt = 48*paceK + jit(8)
	}
	s.HitAt = math.Min(92, round1(s.HitAt))
	s.FakeAt = round1(s.FakeAt)
	// CT: cada posició del muntatge.
	sp := setups[cd.Setup]
	ctPost := map[int]post{}
	for pi, slot := range ctRoleOrder {
		ctPost[slot] = sp[pi]
		g := sp[pi].group
		if g == "passive" {
			g = map[string]string{"market": "B", "ct": "A"}[sp[pi].zone]
		}
		r.posts[g]++
	}
	if site == "" {
		// default_mid: el lloc amb menys CT, amb informació imperfecta.
		acc := map[string]float64{"fast": 0.6, "default": 0.7, "slow": 0.8}[td.Pace]
		if r.posts["A"] < r.posts["B"] || r.posts["A"] == r.posts["B"] && r.rng() < 0.5 {
			site = "A"
		} else {
			site = "B"
		}
		if r.rng() > acc {
			site = other(site)
		}
	}
	r.site, r.fake = site, fake
	s.Site, s.Fake = site, fake
	for _, st := range append(stage[:], "t_spawn") {
		for _, sz := range []string{"a_site", "b_site"} {
			s.Routes[st+">"+sz] = tPath(st, sz)
		}
	}
	for _, from := range []string{"a_site", "b_site", "mid", "window", "short", "jungle", "ct", "market", "ct_spawn"} {
		for _, sz := range []string{"a_site", "b_site"} {
			s.Routes[from+">"+sz] = ctPath(from, sz)
		}
	}
	for _, u := range r.units() {
		p := u.p
		lp := LivePlayer{ID: p.ID, Handle: p.Handle, Role: p.Role, Team: p.Team, Side: sideName(u.side), Human: p == l.human,
			X: round1(u.wp[0].x), Y: round1(u.wp[0].y), Primary: p.Primary, Pistol: p.weapon(u.side).ID, Armor: p.Armor, Kit: p.Kit,
			Nades: append([]string{}, p.Nades...), Money: p.Money, Aim: math.Round(p.Aim*1000) / 1000, Bot: l.roleOf(p, u.side, over), Note: l.notes[p.Team][p.Slot]}
		if p.Pistol != "" {
			lp.Pistol = p.Pistol
		} else {
			lp.Pistol = sidePistol(u.side)
		}
		if u.side == sideT {
			lp.Group, lp.Post = group[u.idx], stage[u.idx]
			if strings.HasPrefix(td.Strategy, "rush") {
				lp.Post = siteZone(site)
			}
			lp.Route = tPath("t_spawn", lp.Post)
			lp.Start = round1(0.3 + r.rng()*1.2)
		} else {
			pp := ctPost[u.idx]
			lp.Group, lp.Post = pp.group, pp.zone
			lp.Route = ctPath("ct_spawn", pp.zone)
			lp.Start = round1(0.2 + r.rng()*1.2)
		}
		s.Players = append(s.Players, lp)
	}
	l.setup = s
	return s, nil
}

// Setup: la ronda preparada que espera el resultat (nil si no n'hi ha cap).
func (l *Live) Setup() *LiveSetup { return l.setup }

// LiveLoadout: el que porta la persona després del menú de compra.
type LiveLoadout struct {
	Primary string   `json:"primary"`
	Pistol  string   `json:"pistol"`
	Armor   int      `json:"armor"`
	Kit     bool     `json:"kit"`
	Nades   []string `json:"nades"`
}

// LiveKill, LiveUtil…: el registre que torna el navegador.
type LiveKill struct {
	T      float64 `json:"t"`
	Killer string  `json:"killer"`
	Victim string  `json:"victim"`
	Weapon string  `json:"weapon"`
	HS     bool    `json:"hs"`
	Smoke  bool    `json:"smoke"`
	Wall   bool    `json:"wall,omitempty"` // molotov, HE, la bomba
}

type LiveUtil struct {
	T    float64 `json:"t"`
	ID   string  `json:"id"`
	Type string  `json:"type"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	FX   float64 `json:"fx"`
	FY   float64 `json:"fy"`
}

type LivePlant struct {
	T     float64 `json:"t"`
	Start float64 `json:"start"`
	ID    string  `json:"id"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}

type LiveDefuse struct {
	T     float64 `json:"t"`
	Start float64 `json:"start"`
	ID    string  `json:"id"`
}

// LiveResult: la ronda jugada.
type LiveResult struct {
	Round   int                     `json:"round"`
	Human   *LiveLoadout            `json:"human"`
	Tracks  map[string][][3]float64 `json:"tracks"` // id -> [[t, x, y], …]
	Kills   []LiveKill              `json:"kills"`
	Utils   []LiveUtil              `json:"utils"`
	Plant   *LivePlant              `json:"plant"`
	Defuse  *LiveDefuse             `json:"defuse"`
	Explode float64                 `json:"explode"` // hora de l'explosió (0 = no)
	EndT    float64                 `json:"endT"`
	Dmg     map[string]int          `json:"dmg"`
}

// loadoutCost: el que val un equipament comprat.
func loadoutCost(lo *LiveLoadout, side int) int {
	c := 0
	if w, ok := weapons[lo.Primary]; ok {
		c += w.Cost
	}
	if w, ok := weapons[lo.Pistol]; ok && w.ID != sidePistol(side) {
		c += w.Cost
	}
	switch lo.Armor {
	case 1:
		c += kevlarCost
	case 2:
		c += helmetCost
	}
	if lo.Kit {
		c += kitCost
	}
	for _, n := range lo.Nades {
		c += nadeCost(n, side)
	}
	return c
}

// applyHuman: el que la persona ha comprat (es paga la diferència amb el
// que ja portava; si no hi arriba, es queda com estava).
func (l *Live) applyHuman(lo *LiveLoadout, side int) {
	p := l.human
	if lo == nil {
		return
	}
	n := LiveLoadout{Primary: lo.Primary, Pistol: lo.Pistol, Armor: max(0, min(2, lo.Armor)), Kit: lo.Kit && side == sideCT}
	if w, ok := weapons[n.Primary]; !ok || w.Class == "pistol" || (w.Side != 0 && w.Side != side) {
		n.Primary = ""
	}
	if w, ok := weapons[n.Pistol]; !ok || w.Class != "pistol" || (w.Side != 0 && w.Side != side) {
		n.Pistol = sidePistol(side)
	}
	cnt := map[string]int{}
	for _, x := range lo.Nades {
		if nadeCost(x, side) > 0 && len(n.Nades) < 4 && (cnt[x] < 1 || x == "flash" && cnt[x] < 2) {
			cnt[x]++
			n.Nades = append(n.Nades, x)
		}
	}
	had := LiveLoadout{Primary: p.Primary, Pistol: p.Pistol, Armor: p.Armor, Kit: p.Kit, Nades: p.Nades}
	if had.Pistol == "" {
		had.Pistol = sidePistol(side)
	}
	cost := loadoutCost(&n, side) - loadoutCost(&had, side)
	if cost > p.Money {
		return
	}
	p.Money -= max(0, cost)
	p.Primary, p.Pistol, p.Armor, p.Kit, p.Nades = n.Primary, n.Pistol, n.Armor, n.Kit, n.Nades
	if p.Pistol == sidePistol(side) {
		p.Pistol = ""
	}
}

// zoneAt: la zona del mapa més propera a un punt.
func zoneAt(x, y float64) string {
	best, bd := "mid", math.Inf(1)
	for _, z := range zones {
		dx := math.Max(0, math.Abs(x-z.X)-z.W/2)
		dy := math.Max(0, math.Abs(y-z.Y)-z.H/2)
		if d := math.Hypot(dx, dy); d < bd {
			best, bd = z.ID, d
		}
	}
	return best
}

// moveChunk: un tram de moviment seguit (sense aturades), per a l'esdeveniment «move».
type moveChunk struct {
	t0, t1 float64
	pts    [][2]float64
}

// compressTrack parteix una pista mostrejada en trams de moviment: talla a
// cada aturada (els trams del visor van a velocitat constant) i com a molt
// cada 6 s, i es queda els punts on la direcció canvia.
func compressTrack(tr [][3]float64) []moveChunk {
	var out []moveChunk
	var cur *moveChunk
	flush := func() {
		if cur != nil && len(cur.pts) >= 2 {
			out = append(out, *cur)
		}
		cur = nil
	}
	for i := 1; i < len(tr); i++ {
		a, b := tr[i-1], tr[i]
		dt := b[0] - a[0]
		if dt <= 0 {
			continue
		}
		moving := math.Hypot(b[1]-a[1], b[2]-a[2])/dt > 0.6
		if !moving {
			flush()
			continue
		}
		if cur != nil && b[0]-cur.t0 > 6 {
			flush()
		}
		if cur == nil {
			cur = &moveChunk{t0: a[0], pts: [][2]float64{{a[1], a[2]}}}
		}
		// el punt d'abans sobra si està en línia recta
		if n := len(cur.pts); n >= 2 {
			p, q := cur.pts[n-2], cur.pts[n-1]
			ux, uy := q[0]-p[0], q[1]-p[1]
			vx, vy := b[1]-q[0], b[2]-q[1]
			if math.Abs(ux*vy-uy*vx) < 0.05*math.Hypot(ux, uy)*math.Hypot(vx, vy) && ux*vx+uy*vy > 0 {
				cur.pts = cur.pts[:n-1]
			}
		}
		cur.pts = append(cur.pts, [2]float64{b[1], b[2]})
		cur.t1 = b[0]
	}
	flush()
	return out
}

// Finish rep la ronda jugada al navegador i la tanca amb les regles del
// motor: torna els esdeveniments de la ronda (com una ronda simulada).
func (l *Live) Finish(res LiveResult) ([]arena.Event, error) {
	m, r, s := l.m, l.r, l.setup
	if r == nil || s == nil {
		return nil, fmt.Errorf("no round in play")
	}
	if res.Round != 0 && res.Round != s.Round {
		return nil, fmt.Errorf("round %d is not in play (round %d is)", res.Round, s.Round)
	}
	l.setup, l.r = nil, nil
	hs := sideCT
	if l.human.Team == r.tTeam {
		hs = sideT
	}
	l.applyHuman(res.Human, hs)
	units := map[string]*unit{}
	for _, u := range r.units() {
		units[u.p.ID] = u
	}
	clampT := func(t float64) float64 { return math.Max(0, math.Min(t, roundTime+bombTime+5)) }
	// Pistes: el punt de sortida i els trams de moviment.
	for id, tr := range res.Tracks {
		u := units[id]
		if u == nil || len(tr) == 0 {
			continue
		}
		sort.SliceStable(tr, func(i, j int) bool { return tr[i][0] < tr[j][0] })
		u.wp = []wpt{{0, tr[0][1], tr[0][2]}}
		for _, p := range tr[1:] {
			u.wp = append(u.wp, wpt{clampT(p[0]), p[1], p[2]})
		}
	}
	tObs, cObs := observeBuy(m.teamPlayers(r.tTeam), sideT), observeBuy(m.teamPlayers(1-r.tTeam), sideCT)
	if !m.quiet {
		var tBuy, cBuy []buyItem
		for _, u := range r.units() {
			it := buyItem{ID: u.p.ID, Weapon: u.p.weapon(u.side).ID, Armor: u.p.Armor, Kit: u.p.Kit, Nades: append([]string(nil), u.p.Nades...), Money: u.p.Money}
			if u.side == sideT {
				tBuy = append(tBuy, it)
			} else {
				cBuy = append(cBuy, it)
			}
		}
		r.startEvents(r.td, r.cd, tBuy, cBuy, tObs, cObs, m.isPistol())
		for _, u := range r.units() {
			for _, c := range compressTrack(res.Tracks[u.p.ID]) {
				var pp [][2]float64
				for _, p := range c.pts {
					pp = append(pp, [2]float64{round1(p[0]), round1(p[1])})
				}
				to := zoneAt(c.pts[len(c.pts)-1][0], c.pts[len(c.pts)-1][1])
				r.emit(clampT(c.t0), "move", r.tx.move(u.p.Handle, zoneByID[to].Name), false, map[string]any{
					"id": u.p.ID, "team": r.teamOf(u), "path": []string{to}, "pts": pp, "dur": round1(c.t1 - c.t0), "to": to,
				})
			}
		}
	}
	r.tAWP, r.ctAWP = hasAWP(r.T), hasAWP(r.CT)
	// Utilitat (gastada: els vius no la tornen a tenir la ronda vinent).
	sort.SliceStable(res.Utils, func(i, j int) bool { return res.Utils[i].T < res.Utils[j].T })
	for _, ut := range res.Utils {
		u := units[ut.ID]
		if u == nil || nadeCost(ut.Type, u.side) == 0 || !u.p.take(ut.Type) {
			continue
		}
		switch {
		case u.side == sideT && ut.Type == "smoke":
			r.tSm++
		case u.side == sideT && ut.Type == "flash":
			r.tFl++
		case u.side == sideT && ut.Type == "molotov":
			r.tMo++
		case u.side == sideCT:
			r.ctNd++
		}
		z := zoneAt(ut.X, ut.Y)
		if !m.quiet {
			dur := map[string]float64{"smoke": 18, "molotov": 7, "flash": 1.5, "he": 1}[ut.Type]
			r.emit(clampT(ut.T), "utility", r.tx.util(u.p.Handle, ut.Type, zoneByID[z].Name), false, map[string]any{
				"id": u.p.ID, "team": r.teamOf(u), "type": ut.Type, "zone": z, "x": round1(ut.X), "y": round1(ut.Y), "fx": round1(ut.FX), "fy": round1(ut.FY), "dur": dur,
			})
		}
	}
	// Morts, plantada, desactivació i explosió, per ordre.
	sort.SliceStable(res.Kills, func(i, j int) bool { return res.Kills[i].T < res.Kills[j].T })
	plantT := math.Inf(1)
	if res.Plant != nil && units[res.Plant.ID] != nil && units[res.Plant.ID].side == sideT {
		plantT = clampT(res.Plant.T)
	}
	planted := false
	doPlant := func() {
		p := units[res.Plant.ID]
		site := siteOf(zoneAt(res.Plant.X, res.Plant.Y))
		if site == "" {
			site = r.site
		}
		r.site = site
		r.planted, r.plantT, r.planter = true, plantT, p
		p.p.Plants++
		p.p.addMoney(plantReward)
		r.emit(plantT, "plant", r.tx.plant(p.p.Handle, site), false, map[string]any{
			"id": p.p.ID, "team": r.tTeam, "site": site, "x": round1(res.Plant.X), "y": round1(res.Plant.Y), "start": round1(math.Max(0, res.Plant.Start)), "explodeAt": round1(plantT + bombTime),
		})
		planted = true
	}
	for _, k := range res.Kills {
		t := clampT(k.T)
		if r.over() {
			break
		}
		if !planted && t >= plantT {
			doPlant()
		}
		kk, vv := units[k.Killer], units[k.Victim]
		if vv == nil || !vv.alive {
			continue
		}
		if kk == nil || kk == vv || !kk.alive && kk.side == vv.side {
			// suïcidi (molotov, la bomba): mor sense que ningú en cobri res
			vv.alive = false
			vv.cut(t)
			vv.p.D++
			if !m.quiet {
				vx, vy := vv.pos(t)
				r.emit(t, "kill", r.tx.kill(vv.p.Handle, vv.p.Handle, "world", false), false, map[string]any{
					"killer": vv.p.ID, "victim": vv.p.ID, "kteam": r.teamOf(vv), "vteam": r.teamOf(vv), "weapon": "world", "weaponName": "World", "hs": false,
					"kx": round1(vx), "ky": round1(vy), "vx": round1(vx), "vy": round1(vy), "zone": zoneAt(vx, vy), "multi": 0,
				})
			}
		} else {
			w, ok := weapons[k.Weapon]
			if !ok {
				w = kk.p.weapon(kk.side)
			}
			vx, vy := vv.pos(t)
			h := k.HS
			r.killWith(kk, vv, t, zoneAt(vx, vy), k.Smoke, w, &h)
		}
		tA, cA := r.aliveN(r.T), r.aliveN(r.CT)
		switch {
		case cA == 0:
			r.end(sideT, "elim", t)
		case tA == 0 && !planted && t < plantT:
			r.end(sideCT, "elim", t)
		}
	}
	if !r.over() && !planted && plantT < math.Inf(1) && plantT <= roundTime {
		doPlant()
	}
	if !r.over() && planted {
		if d := res.Defuse; d != nil && units[d.ID] != nil && units[d.ID].alive && units[d.ID].side == sideCT && d.T <= plantT+bombTime {
			du := units[d.ID]
			du.p.Defuses++
			du.p.addMoney(defuseReward)
			r.defuser = du
			t := clampT(d.T)
			ninja := r.aliveN(r.T) > 0
			dur := defuseBare
			if du.p.Kit {
				dur = defuseKit
			}
			x, y := du.pos(t)
			r.emit(t, "defuse", r.tx.defuse(du.p.Handle, ninja), ninja || plantT+bombTime-t < 1, map[string]any{
				"id": du.p.ID, "team": 1 - r.tTeam, "start": round1(math.Max(plantT, d.Start)), "dur": dur, "kit": du.p.Kit, "ninja": ninja, "x": round1(x), "y": round1(y),
				"left": round1(plantT + bombTime - t),
			})
			r.end(sideCT, "defuse", t)
		} else {
			bx, by := res.Plant.X, res.Plant.Y
			r.explode(plantT+bombTime, bx, by)
		}
	}
	if !r.over() {
		r.end(sideCT, "time", roundTime)
	}
	r.hitStart = s.HitAt
	for id, d := range res.Dmg {
		if u := units[id]; u != nil && d > 0 {
			u.p.Dmg += min(d, 500)
		}
	}
	return m.finishRound(r, tObs, cObs), nil
}
