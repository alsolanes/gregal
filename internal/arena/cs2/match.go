package cs2

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"gregal/internal/arena"
)

// Match és un partit en marxa.
type Match struct {
	seed    int64
	lang    string
	names   [2]string
	rng     *rand.Rand
	players [2][5]*Player

	startT   int
	tTeam    int
	round    int // rondes jugades
	score    [2]int
	lossLvl  [2]int
	hist     []roundRec
	decision int
	pending  bool
	done     bool
	winner   int
	evN      int
	stats    matchStats
	quiet    bool // proves: sense esdeveniments (simulacions ràpides)
}

type matchStats struct {
	OT int
}

// Decisions.
type tDec struct {
	Buy      string `json:"buy"`
	Strategy string `json:"strategy"`
	Utility  string `json:"utility"`
	Pace     string `json:"pace"`
	AWP      bool   `json:"awp"`
}

type ctDec struct {
	Buy     string `json:"buy"`
	Setup   string `json:"setup"`
	Utility string `json:"utility"`
	AWP     bool   `json:"awp"`
}

var (
	buyVals   = []string{"eco", "half", "force", "full"}
	stratVals = []string{"rush_a", "rush_b", "execute_a", "execute_b", "split_a", "split_b", "default_mid", "fake_a_go_b", "fake_b_go_a"}
	tUtilVals = []string{"none", "smokes", "flash_heavy", "full"}
	paceVals  = []string{"fast", "default", "slow"}
	setupVals = []string{"standard_2_1_2", "stack_a", "stack_b", "aggressive_mid", "retake_b", "retake_a"}
	ctUtilVal = []string{"save", "normal", "heavy"}
)

// roundRec: el que es recorda de cada ronda (per a l'historial).
type roundRec struct {
	Round    int
	TTeam    int
	T        tDec
	CT       ctDec
	TObs     string // compra observada
	CTObs    string
	Site     string
	Planted  bool
	Winner   int
	Reason   string
	TSmokes  int
	TFlashes int
	TMollies int
	CTNades  int
	CTPosts  map[string]int
	TAWP     bool
	CTAWP    bool
	Kills    [2]int
}

func norm(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer(" ", "_", "-", "_", "+", "_").Replace(s)
	return s
}

func pick(v string, allowed []string, alias map[string]string) string {
	v = norm(v)
	if a, ok := alias[v]; ok {
		v = a
	}
	for _, x := range allowed {
		if x == v {
			return v
		}
	}
	return ""
}

// Àlies per camp: cada camp té els seus ("save" és eco a "buy" però és un
// valor vàlid de la utilitat CT).
var (
	buyAlias   = map[string]string{"full_buy": "full", "buy": "full", "force_buy": "force", "half_buy": "half", "save": "eco", "eco_buy": "eco", "light": "half", "forced": "force"}
	stratAlias = map[string]string{"default": "default_mid", "mid": "default_mid", "fake_a": "fake_a_go_b", "fake_b": "fake_b_go_a", "fake_a_b": "fake_a_go_b", "fake_b_a": "fake_b_go_a",
		"execute_a_site": "execute_a", "execute_b_site": "execute_b", "exec_a": "execute_a", "exec_b": "execute_b", "a_rush": "rush_a", "b_rush": "rush_b"}
	tUtilAlias  = map[string]string{"smoke": "smokes", "flashes": "flash_heavy", "flash": "flash_heavy", "all": "full", "heavy": "full", "no": "none", "save": "none"}
	paceAlias   = map[string]string{"medium": "default", "normal": "default", "quick": "fast", "rush": "fast"}
	setupAlias  = map[string]string{"standard": "standard_2_1_2", "2_1_2": "standard_2_1_2", "default": "standard_2_1_2", "stack_a_site": "stack_a", "stack_b_site": "stack_b", "aggressive": "aggressive_mid", "mid": "aggressive_mid"}
	ctUtilAlias = map[string]string{"none": "save", "eco": "save", "default": "normal", "full": "heavy"}
)

func rawMap(raw json.RawMessage) map[string]any {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		if s := firstObj(string(raw)); s != "" {
			_ = json.Unmarshal([]byte(s), &m)
		}
	}
	if m == nil {
		return map[string]any{}
	}
	if d, ok := m["decision"].(map[string]any); ok {
		return d
	}
	return m
}

func firstObj(s string) string {
	a := arena.ParseAnswer(s)
	return string(a.Decision)
}

func str(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func boolean(m map[string]any, k string) (bool, bool) {
	switch v := m[k].(type) {
	case bool:
		return v, true
	case string:
		switch norm(v) {
		case "true", "yes", "1":
			return true, true
		case "false", "no", "0":
			return false, true
		}
	case float64:
		return v != 0, true
	}
	return false, false
}

func (m *Match) defaultBuy(team, side int) string {
	if m.isPistol() || canFullBuy(m.players[team][:], side) {
		return "full"
	}
	return "eco"
}

func (m *Match) parseT(raw json.RawMessage, team int) tDec {
	j := rawMap(raw)
	d := tDec{Buy: pick(str(j, "buy"), buyVals, buyAlias), Strategy: pick(str(j, "strategy"), stratVals, stratAlias),
		Utility: pick(str(j, "utility"), tUtilVals, tUtilAlias), Pace: pick(str(j, "pace"), paceVals, paceAlias)}
	if d.Buy == "" {
		d.Buy = m.defaultBuy(team, sideT)
	}
	if d.Strategy == "" {
		d.Strategy = "default_mid"
	}
	if d.Utility == "" {
		d.Utility = "smokes"
	}
	if d.Pace == "" {
		d.Pace = "default"
	}
	if v, ok := boolean(j, "awp"); ok {
		d.AWP = v
	} else {
		d.AWP = true
	}
	return d
}

func (m *Match) parseCT(raw json.RawMessage, team int) ctDec {
	j := rawMap(raw)
	d := ctDec{Buy: pick(str(j, "buy"), buyVals, buyAlias), Setup: pick(str(j, "setup"), setupVals, setupAlias), Utility: pick(str(j, "utility"), ctUtilVal, ctUtilAlias)}
	if d.Buy == "" {
		d.Buy = m.defaultBuy(team, sideCT)
	}
	if d.Setup == "" {
		d.Setup = "standard_2_1_2"
	}
	if d.Utility == "" {
		d.Utility = "normal"
	}
	if v, ok := boolean(j, "awp"); ok {
		d.AWP = v
	} else {
		d.AWP = true
	}
	return d
}

// ---- rondes, meitats, pròrrogues ----

func (m *Match) nextRound() int { return m.round + 1 }

func (m *Match) isPistol() bool { r := m.nextRound(); return r == 1 || r == 13 }

// tTeamFor: qui fa de T a la ronda r. A cada pròrroga es torna a canviar.
func (m *Match) tTeamFor(r int) int {
	switch {
	case r <= 12:
		return m.startT
	case r <= 24:
		return 1 - m.startT
	}
	if ((r-25)/3)%2 == 0 {
		return m.startT
	}
	return 1 - m.startT
}

func (m *Match) halfName(r int) string {
	switch {
	case r <= 12:
		return "first half"
	case r <= 24:
		return "second half"
	}
	return fmt.Sprintf("overtime %d", (r-25)/6+1)
}

// targetFor: rondes que cal per guanyar a la ronda r (MR12, pròrrogues MR3).
func targetFor(r int) int {
	if r <= 24 {
		return 13
	}
	return 16 + 3*((r-25)/6)
}

func (m *Match) target() int { return targetFor(m.nextRound()) }

func (m *Match) resetHalf(cash int) {
	for t := 0; t < 2; t++ {
		for _, p := range m.players[t] {
			p.strip()
			p.Money = cash
		}
		m.lossLvl[t] = 1
	}
}

func (m *Match) teamPlayers(t int) []*Player { return m.players[t][:] }

func sideOf(m *Match, team int) int {
	if team == m.tTeam {
		return sideT
	}
	return sideCT
}

// ---- contracte ----

func (m *Match) Next() (p [2]arena.Prompt, done bool) {
	if m.done {
		return p, true
	}
	if !m.pending {
		m.decision++
		m.pending = true
		m.tTeam = m.tTeamFor(m.nextRound())
	}
	for t := 0; t < 2; t++ {
		p[t] = m.prompt(t)
	}
	return p, false
}

func (m *Match) Apply(ans [2]json.RawMessage) []arena.Event {
	if m.done {
		return nil
	}
	if !m.pending {
		m.decision++
		m.tTeam = m.tTeamFor(m.nextRound())
	}
	m.pending = false
	tt := m.tTeam
	td := m.parseT(ans[tt], tt)
	cd := m.parseCT(ans[1-tt], 1-tt)
	return m.playRound(td, cd)
}

func (m *Match) Score() [2]int { return m.score }

func (m *Match) Winner() int {
	if !m.done {
		return -1
	}
	return m.winner
}

// ---- prompts ----

const buyHelp = `buy: eco = buy nothing and save; half = each player spends at most 45% of his money (max $2,100) on an SMG or P250, kevlar and a flash/smoke; force = spend everything on the best you can (AK/M4 if you can pay it plus kevlar, else Galil/FAMAS, else SMG, else Deagle/P250), then helmet, then your utility; full = rifles (the AWPer buys an AWP if "awp":true and he can pay it plus kevlar), helmets, kits (CT) and your utility; on a full buy rich players drop rifles to teammates who cannot afford one. In pistol rounds ($800) full and force both mean kevlar + pistol.`

const tSchema = `{"buy":"eco|half|force|full","strategy":"rush_a|rush_b|execute_a|execute_b|split_a|split_b|default_mid|fake_a_go_b|fake_b_go_a","utility":"none|smokes|flash_heavy|full","pace":"fast|default|slow","awp":true|false}
` + buyHelp + `
strategy (see the round-win table in the rules): rush_x = all 5 run into site x at once (beats aggressive_mid and a stack on the other site; loses to a stack on x, to standard and to retakes, especially vs heavy CT utility); execute_x = take control, then hit x together with utility (beats retake_x and standard a little; loses to a stack on x and to aggressive_mid); split_x = hit x from two sides (beats standard; loses to a stack on x and to aggressive_mid); default_mid = spread, fight for mid, hit late the site with fewer CTs (beats stacks and retakes; loses to aggressive_mid); fake_x_go_y = utility at x, then hit y (beats standard and a stack on x; loses to a stack on y).
utility: none ($0) | smokes (smoke + flash each, ~$500) | flash_heavy (2 flashes + smoke, ~$700) | full (smoke, 2 flashes, molotov, ~$1,100). More utility wins more executes, splits, fakes and post-plants; rushes only use the flashes.
pace: fast (hit earlier: a little better vs aggressive_mid, a little worse vs the rest) | default (best in general) | slow (more mid duels: a little better for default_mid vs stacks, worse vs aggressive_mid). Rushes ignore pace.
awp: true = on a full buy your AWPer buys an AWP if he can afford it (+3-7 points: best in long, unsmoked duels).`

const ctSchema = `{"buy":"eco|half|force|full","setup":"standard_2_1_2|stack_a|stack_b|aggressive_mid|retake_b|retake_a","utility":"save|normal|heavy","awp":true|false}
` + buyHelp + `
setup (see the round-win table in the rules): standard_2_1_2 = 2 A, 1 window, 2 B, rotate on contact (safest: beats rushes, loses to fakes and splits); stack_a / stack_b = 4 on that site, 1 on the other (crushes any hit on the stacked site, loses to hits on the other site, fakes of the stacked site and default_mid); aggressive_mid = 3 push mid for early picks, 1 per site (beats default_mid, splits and executes; rushes walk into its empty sites); retake_a / retake_b = give that site up (1 hidden player), 3 hold the other site, then retake together after the plant (beats rushes and executes into the held site, and rushes into the given site; loses to executes and default_mid into the given site).
utility: save (none, $0) | normal (flash + smoke each, ~$500) | heavy (molotov, smoke, flash, HE each, ~$1,300: takes 1-13 points from the T, most vs rushes and hits into a defended site).
awp: true = on a full buy your AWPer buys an AWP if he can afford it (+3-7 points: best in long, unsmoked holds).`

func (m *Match) prompt(team int) arena.Prompt {
	side := sideOf(m, team)
	r := m.nextRound()
	sideName := "T"
	sch := tSchema
	if side == sideCT {
		sideName = "CT"
		sch = ctSchema
	}
	short := fmt.Sprintf("R%d %s %d-%d %s", r, sideName, m.score[team], m.score[1-team], money(teamMoney(m.teamPlayers(team))))
	return arena.Prompt{Situation: m.situation(team), Schema: sch, Short: short}
}

func equipValue(p *Player, side int) int {
	v := 0
	if p.Primary != "" {
		v += weapons[p.Primary].Cost
	} else if p.Pistol != "" {
		v += weapons[p.Pistol].Cost
	}
	if p.Armor == 1 {
		v += kevlarCost
	} else if p.Armor == 2 {
		v += helmetCost
	}
	return v
}

// hashFloat: un nombre pseudoaleatori estable dins [0,1) (splitmix64), per
// al soroll de l'estimació dels diners del rival sense tocar l'atzar del partit.
func hashFloat(x uint64) float64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	x ^= x >> 31
	return float64(x>>11) / float64(1<<53)
}

// halfPos: posició de la ronda r dins la seva meitat (1..len) i la llargada.
func halfPos(r int) (int, int) {
	if r <= 24 {
		return (r-1)%12 + 1, 12
	}
	return (r-25)%3 + 1, 3
}

// freshFullCost: el que costa la compra completa d'un equip sense res.
func freshFullCost(side int) int {
	c := weapons[sideRifle(side)].Cost + helmetCost + 500
	if side == sideCT {
		c += kitCost / 2
	}
	return 5 * c
}

func (m *Match) situation(team int) string {
	var b strings.Builder
	side := sideOf(m, team)
	opp := 1 - team
	r := m.nextRound()
	me, them, sideName, oppName := "T (attack: plant the bomb at A or B)", "CT", "T", "CT"
	if side == sideCT {
		me, them, sideName, oppName = "CT (defend A and B, defuse)", "T", "CT", "T"
	}
	pos, hl := halfPos(r)
	fmt.Fprintf(&b, "Map: Mirage. Round %d (%s, round %d of %d of this half; first to %d wins). You are %s. Score: you %d - %d them.\n",
		r, m.halfName(r), pos, hl, m.target(), me, m.score[team], m.score[opp])
	if m.isPistol() || r > 24 && (r-25)%3 == 0 {
		if r <= 24 {
			fmt.Fprintf(&b, "PISTOL ROUND: everyone has $800.\n")
		} else {
			fmt.Fprintf(&b, "Overtime half start: everyone has $10,000.\n")
		}
	}
	last := pos == hl
	if last {
		fmt.Fprintf(&b, "Last round of the half: money resets after it, so spend everything.\n")
	}
	ps := m.teamPlayers(team)
	var pm []string
	for _, p := range ps {
		pm = append(pm, fmt.Sprintf("%s (%s) %s", p.Handle, p.Role, money(p.Money)))
	}
	fmt.Fprintf(&b, "Your money: %s total — %s.\n", money(teamMoney(ps)), strings.Join(pm, ", "))
	fmt.Fprintf(&b, "Full buy now costs your team about %s (%s).\n", money(teamFullCost(ps, side)), map[bool]string{true: "affordable", false: "NOT affordable"}[canFullBuy(ps, side)])
	fmt.Fprintf(&b, "Loss bonus: if you lose this round each player gets %s (level %d/4; each loss raises it one level, each win lowers it one level).\n", money(lossBonus(m.lossLvl[team])), m.lossLvl[team])
	if !last && !canFullBuy(ps, side) {
		next := teamMoney(ps) + 5*lossBonus(m.lossLvl[team])
		ok := "enough for a full buy next round"
		if next < freshFullCost(side)-400 {
			ok = "still NOT enough for a full buy next round"
		}
		fmt.Fprintf(&b, "If you eco now and lose, you will have about %s next round (%s; a full buy from nothing costs about %s).\n", money(next), ok, money(freshFullCost(side)))
	}
	var kept []string
	myAWP := false
	for _, p := range ps {
		if p.Primary != "" || p.Armor > 0 {
			kept = append(kept, p.Handle+": "+gearLine(p, side))
		}
		myAWP = myAWP || p.Primary == "awp"
	}
	if len(kept) == 0 {
		fmt.Fprintf(&b, "Kept from last round: nothing.\n")
	} else {
		fmt.Fprintf(&b, "Kept from last round: %s.\n", strings.Join(kept, "; "))
	}
	// Diners del rival, estimats.
	ops := m.teamPlayers(opp)
	om := teamMoney(ops)
	est := float64(om) * (0.92 + 0.16*hashFloat(uint64(m.seed*7919+int64(r)*31+int64(team))))
	lo, hi := int(est*0.9)/500*500, (int(est*1.1)/500+1)*500
	okeep, theirAWP := 0, false
	for _, p := range ops {
		if p.Primary != "" && weapons[p.Primary].Class != "smg" {
			okeep++
		}
		theirAWP = theirAWP || p.Primary == "awp"
	}
	can := "probably cannot full buy"
	if est >= float64(teamFullCost(ops, 3-side))-400 {
		can = "can probably full buy"
	}
	fmt.Fprintf(&b, "Enemy (%s) estimated money: %s-%s (%s); they kept %d rifle(s)/AWP; their next loss pays %s each (level %d/4).\n", them, money(lo), money(hi), can, okeep, money(lossBonus(m.lossLvl[opp])), m.lossLvl[opp])
	fmt.Fprintf(&b, "AWP: you %s; the enemy %s.\n",
		map[bool]string{true: "kept an AWP", false: "have no AWP kept (a full buy with \"awp\":true buys one if the AWPer can afford it)"}[myAWP],
		map[bool]string{true: "kept an AWP", false: "kept no AWP"}[theirAWP])
	// Historial: el rival a la banda que juga ara, i les nostres crides a la nostra banda.
	var theirs, ours []roundRec
	for _, h := range m.hist {
		if (h.TTeam == opp) == (side == sideCT) {
			theirs = append(theirs, h)
			ours = append(ours, h)
		}
	}
	if len(theirs) == 0 {
		fmt.Fprintf(&b, "Opponent as %s: not seen yet on that side.\n", oppName)
	} else {
		if len(theirs) > 6 {
			theirs = theirs[len(theirs)-6:]
		}
		fmt.Fprintf(&b, "Opponent's last rounds as %s (oldest first):\n", oppName)
		for _, h := range theirs {
			b.WriteString("- " + m.histLine(h, team) + "\n")
		}
		b.WriteString(m.tendencies(team))
	}
	if len(ours) > 0 {
		if len(ours) > 5 {
			ours = ours[len(ours)-5:]
		}
		fmt.Fprintf(&b, "Your last rounds as %s (oldest first):\n", sideName)
		for _, h := range ours {
			b.WriteString("- " + m.ownLine(h, team) + "\n")
		}
	}
	return b.String()
}

// ownLine: com ha anat una crida nostra.
func (m *Match) ownLine(h roundRec, team int) string {
	res := "we LOST"
	if h.Winner == team {
		res = "we WON"
	}
	reason := map[string]string{"elim": "elimination", "bomb": "bomb exploded", "defuse": "defused", "time": "time ran out"}[h.Reason]
	if h.TTeam == team {
		return fmt.Sprintf("R%d [we T] %s buy: %s, utility %s, pace %s vs their %s (%s buy) -> %s (%s)",
			h.Round, h.TObs, h.T.Strategy, h.T.Utility, h.T.Pace, h.CT.Setup, h.CTObs, res, reason)
	}
	return fmt.Sprintf("R%d [we CT] %s buy: %s, utility %s vs their %s (%s buy, hit %s) -> %s (%s)",
		h.Round, h.CTObs, h.CT.Setup, h.CT.Utility, h.T.Strategy, h.TObs, orDash(h.Site), res, reason)
}

func setupDesc(s string, posts map[string]int) string {
	return fmt.Sprintf("%s (%d A, %d mid, %d B)", s, posts["A"], posts["mid"], posts["B"])
}

func (m *Match) histLine(h roundRec, team int) string {
	res := "they won"
	if h.Winner == team {
		res = "we won"
	}
	reason := map[string]string{"elim": "elimination", "bomb": "bomb exploded", "defuse": "defused", "time": "time ran out"}[h.Reason]
	if h.TTeam != team {
		awp := ""
		if h.TAWP {
			awp = ", AWP"
		}
		fake := ""
		if strings.HasPrefix(h.T.Strategy, "fake") {
			fake = ", faked " + strings.ToUpper(h.T.Strategy[5:6])
		}
		planted := "no plant"
		if h.Planted {
			planted = "planted"
		}
		return fmt.Sprintf("R%d [they T] %s buy: %s, hit %s%s, pace %s, threw %d smokes %d flashes %d molotovs%s; %s -> %s (%s)",
			h.Round, h.TObs, h.T.Strategy, orDash(h.Site), fake, h.T.Pace, h.TSmokes, h.TFlashes, h.TMollies, awp, planted, res, reason)
	}
	awp := ""
	if h.CTAWP {
		awp = ", AWP"
	}
	return fmt.Sprintf("R%d [they CT] %s buy: %s, threw %d utility%s; we hit %s -> %s (%s)",
		h.Round, h.CTObs, setupDesc(h.CT.Setup, h.CTPosts), h.CTNades, awp, orDash(h.Site), res, reason)
}

func yesno(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// tendencies: resum del rival a la banda que farà ara.
func (m *Match) tendencies(team int) string {
	opp := 1 - team
	oppSide := sideT
	if m.tTeam == opp {
		oppSide = sideT
	} else {
		oppSide = sideCT
	}
	cnt := map[string]int{}
	n := 0
	var keys []string
	for _, h := range m.hist {
		if (oppSide == sideT) != (h.TTeam == opp) {
			continue
		}
		n++
		k := h.CT.Setup
		if oppSide == sideT {
			k = h.T.Strategy
			if h.Site != "" {
				cnt["site "+h.Site]++
			}
		}
		cnt[k]++
		if !contains(keys, k) {
			keys = append(keys, k)
		}
	}
	if n == 0 {
		if oppSide == sideT {
			return "Their T-side tendencies: not seen yet as T.\n"
		}
		return "Their CT-side tendencies: not seen yet as CT.\n"
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s x%d", k, cnt[k]))
	}
	if oppSide == sideT {
		return fmt.Sprintf("Their T-side tendencies this match (%d rounds): %s; hit A x%d, hit B x%d.\n", n, strings.Join(parts, ", "), cnt["site A"], cnt["site B"])
	}
	return fmt.Sprintf("Their CT-side tendencies this match (%d rounds): %s.\n", n, strings.Join(parts, ", "))
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// ---- meta ----

type metaPlayer struct {
	ID     string `json:"id"`
	Handle string `json:"handle"`
	Role   string `json:"role"`
}

type metaTeam struct {
	Name    string       `json:"name"`
	Tag     string       `json:"tag"`
	Color   string       `json:"color"`
	Players []metaPlayer `json:"players"`
}

type metaHalf struct {
	From int `json:"from"`
	To   int `json:"to"`
	T    int `json:"t"`
}

type meta struct {
	Game      string                `json:"game"`
	Map       string                `json:"map"`
	Size      [2]float64            `json:"size"`
	Zones     []zone                `json:"zones"`
	Links     [][2]string           `json:"links"`
	Props     []prop                `json:"props"`
	Plant     map[string][2]float64 `json:"plant"`
	Teams     [2]metaTeam           `json:"teams"`
	StartT    int                   `json:"startT"`
	Halves    []metaHalf            `json:"halves"`
	RoundTime int                   `json:"roundTime"`
	BombTime  int                   `json:"bombTime"`
}

func tag(name string) string {
	f := strings.Fields(strings.ToUpper(name))
	s := ""
	if len(f) > 1 {
		for _, w := range f {
			s += string([]rune(w)[:1])
		}
	} else if len(f) == 1 {
		s = string([]rune(f[0]))
	}
	r := []rune(s)
	if len(r) > 4 {
		r = r[:4]
	}
	return string(r)
}

func (m *Match) Meta() any {
	mt := meta{Game: "cs2", Map: "Mirage", Size: [2]float64{mapW, mapH}, Zones: zones, Links: links, Props: props, Plant: plantSpot,
		StartT: m.startT, RoundTime: roundTime, BombTime: bombTime,
		Halves: []metaHalf{{1, 12, m.startT}, {13, 24, 1 - m.startT}, {25, 27, m.startT}, {28, 30, 1 - m.startT}}}
	for t := 0; t < 2; t++ {
		mt.Teams[t] = metaTeam{Name: m.names[t], Tag: tag(m.names[t]), Color: teamColors[t]}
		for _, p := range m.players[t] {
			mt.Teams[t].Players = append(mt.Teams[t].Players, metaPlayer{p.ID, p.Handle, p.Role})
		}
	}
	return mt
}
