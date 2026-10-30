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

var aliases = map[string]string{
	"full_buy": "full", "buy": "full", "force_buy": "force", "half_buy": "half", "save": "eco", "eco_buy": "eco", "light": "half",
	"standard": "standard_2_1_2", "2_1_2": "standard_2_1_2", "default": "default_mid", "stack_a_site": "stack_a", "stack_b_site": "stack_b",
	"fake_a": "fake_a_go_b", "fake_b": "fake_b_go_a", "fake_a_b": "fake_a_go_b", "fake_b_a": "fake_b_go_a",
	"execute_a_site": "execute_a", "execute_b_site": "execute_b", "aggressive": "aggressive_mid", "mid": "default_mid",
	"smoke": "smokes", "flashes": "flash_heavy", "heavy": "heavy", "normal": "normal", "medium": "default",
}

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
	d := tDec{Buy: pick(str(j, "buy"), buyVals, aliases), Strategy: pick(str(j, "strategy"), stratVals, aliases),
		Utility: pick(str(j, "utility"), tUtilVals, aliases), Pace: pick(str(j, "pace"), paceVals, aliases)}
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
	d := ctDec{Buy: pick(str(j, "buy"), buyVals, aliases), Setup: pick(str(j, "setup"), setupVals, aliases), Utility: pick(str(j, "utility"), ctUtilVal, aliases)}
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

const tSchema = `{"buy":"eco|half|force|full","strategy":"rush_a|rush_b|execute_a|execute_b|split_a|split_b|default_mid|fake_a_go_b|fake_b_go_a","utility":"none|smokes|flash_heavy|full","pace":"fast|default|slow","awp":true|false}
buy: eco = buy nothing, save money; half = cheap SMG/pistol + armor, keep ~half the money; force = spend everything on the best you can; full = rifles (AWP for the AWPer if awp=true), helmets, utility (in pistol rounds full = kevlar + pistol). Players who cannot afford a full buy get a dropped rifle from rich teammates when possible.
strategy: rush_x = all 5 run into site x fast (beats passive/aggressive-mid setups, dies to stacks and heavy CT utility); execute_x = take map control then hit x together with utility (~45s); split_x = hit x from two sides at once (crossfires, but the mid group can get picked); default_mid = spread out, fight for mid, hit late the site that looks weaker (good vs stacks, risks the clock); fake_x_go_y = show utility at x, then hit y (beats a stack or rotation to x, wastes utility).
utility: none ($0) | smokes (smoke+flash each, ~$500) | flash_heavy (2 flashes + smoke, ~$700) | full (smoke, 2 flashes, molotov, ~$1100). Utility strongly helps executes and post-plant; costs money.
pace: fast (less time for CTs to rotate, less utility value) | default | slow (more picks and info, AWP better, risks running out of time).
awp: true = your AWPer buys an AWP when affordable on full buys (strong on default/slow, weak in rushes).`

const ctSchema = `{"buy":"eco|half|force|full","setup":"standard_2_1_2|stack_a|stack_b|aggressive_mid|retake_b|retake_a","utility":"save|normal|heavy","awp":true|false}
buy: eco = buy nothing, save money; half = cheap SMG/pistol + armor, keep ~half; force = spend everything; full = rifles, helmets, kits, utility (pistol rounds: full = kevlar/kit).
setup: standard_2_1_2 = 2 A, 1 mid (window), 2 B, rotate on contact; stack_a / stack_b = 4 players on that site, 1 on the other (crushes a hit on the stacked site, loses to a hit or fake on the other); aggressive_mid = 3 push mid for early picks (beats slow/default Ts, a rush walks into near-empty sites); retake_b / retake_a = give that site up (1 hidden player), 3 on the other site, then retake the planted site together with bonus.
utility: save (none, saves ~$600/player) | normal (flash + smoke) | heavy (molotov, smoke, flash, HE: delays and punishes rushes and executes, helps retakes; ~$1300).
awp: true = your AWPer buys an AWP on full buys (strong vs default/slow pace and mid control).`

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

func (m *Match) situation(team int) string {
	var b strings.Builder
	side := sideOf(m, team)
	opp := 1 - team
	r := m.nextRound()
	me, them := "T (attack: plant the bomb at A or B)", "CT"
	if side == sideCT {
		me, them = "CT (defend A and B, defuse)", "T"
	}
	fmt.Fprintf(&b, "Map: Mirage. Round %d (%s; first to %d wins). You are %s. Score: you %d - %d them.\n", r, m.halfName(r), m.target(), me, m.score[team], m.score[opp])
	if m.isPistol() || r > 24 && (r-25)%3 == 0 {
		if r <= 24 {
			fmt.Fprintf(&b, "PISTOL ROUND: everyone has $800.\n")
		} else {
			fmt.Fprintf(&b, "Overtime half start: everyone has $10,000.\n")
		}
	}
	if r == 12 || r == 24 || (r > 24 && (r-24)%3 == 0) {
		fmt.Fprintf(&b, "Last round of the half: money resets after it, so spend everything.\n")
	}
	ps := m.teamPlayers(team)
	var pm []string
	for _, p := range ps {
		pm = append(pm, fmt.Sprintf("%s (%s) %s", p.Handle, p.Role, money(p.Money)))
	}
	fmt.Fprintf(&b, "Your money: %s total — %s.\n", money(teamMoney(ps)), strings.Join(pm, ", "))
	fmt.Fprintf(&b, "Full buy now costs your team about %s (%s).\n", money(teamFullCost(ps, side)), map[bool]string{true: "affordable", false: "NOT affordable"}[canFullBuy(ps, side)])
	fmt.Fprintf(&b, "Loss bonus: if you lose this round each player gets %s (level %d/4).\n", money(lossBonus(m.lossLvl[team])), m.lossLvl[team])
	var kept []string
	for _, p := range ps {
		if p.Primary != "" || p.Armor > 0 {
			kept = append(kept, p.Handle+": "+gearLine(p, side))
		}
	}
	if len(kept) == 0 {
		fmt.Fprintf(&b, "Kept from last round: nothing.\n")
	} else {
		fmt.Fprintf(&b, "Kept from last round: %s.\n", strings.Join(kept, "; "))
	}
	// Diners del rival, estimats.
	om := teamMoney(m.teamPlayers(opp))
	nr := rand.New(rand.NewSource(m.seed*7919 + int64(r)*31 + int64(team)))
	est := float64(om) * (0.92 + 0.16*nr.Float64())
	lo, hi := int(est*0.9)/500*500, (int(est*1.1)/500+1)*500
	okeep := 0
	for _, p := range m.teamPlayers(opp) {
		if p.Primary != "" && weapons[p.Primary].Class != "smg" {
			okeep++
		}
	}
	can := "probably cannot full buy"
	if est >= float64(teamFullCost(m.teamPlayers(opp), 3-side))-400 {
		can = "can probably full buy"
	}
	fmt.Fprintf(&b, "Enemy (%s) estimated money: %s-%s (%s); they kept %d rifle(s)/AWP; their next loss pays %s.\n", them, money(lo), money(hi), can, okeep, money(lossBonus(m.lossLvl[opp])))
	// Historial.
	if len(m.hist) == 0 {
		b.WriteString("Opponent history: none yet (first round).\n")
	} else {
		b.WriteString("Opponent's last rounds (oldest first):\n")
		from := len(m.hist) - 6
		if from < 0 {
			from = 0
		}
		for _, h := range m.hist[from:] {
			b.WriteString("- " + m.histLine(h, team) + "\n")
		}
		b.WriteString(m.tendencies(team))
	}
	return b.String()
}

func setupDesc(s string, posts map[string]int) string {
	return fmt.Sprintf("%s (%d A, %d mid, %d B)", s, posts["A"], posts["mid"], posts["B"])
}

func (m *Match) histLine(h roundRec, team int) string {
	opp := 1 - team
	res := "they won"
	if h.Winner == team {
		res = "we won"
	}
	reason := map[string]string{"elim": "elimination", "bomb": "bomb exploded", "defuse": "defused", "time": "time ran out"}[h.Reason]
	if h.TTeam == opp {
		awp := ""
		if h.TAWP {
			awp = ", AWP"
		}
		what := ""
		switch {
		case strings.HasPrefix(h.T.Strategy, "rush"):
			what = "rushed " + h.Site
		case strings.HasPrefix(h.T.Strategy, "execute"):
			what = "executed " + h.Site
		case strings.HasPrefix(h.T.Strategy, "split"):
			what = "split " + h.Site
		case h.T.Strategy == "default_mid":
			what = "played default/mid control, late hit " + h.Site
		case strings.HasPrefix(h.T.Strategy, "fake"):
			what = fmt.Sprintf("faked %s then hit %s", strings.ToUpper(h.T.Strategy[5:6]), h.Site)
		}
		return fmt.Sprintf("R%d [they T] %s buy; %s (%s), %s pace; utility %d smokes %d flashes %d molotovs%s; planted: %v -> %s (%s)",
			h.Round, h.TObs, what, h.T.Strategy, h.T.Pace, h.TSmokes, h.TFlashes, h.TMollies, awp, yesno(h.Planted), res, reason)
	}
	awp := ""
	if h.CTAWP {
		awp = ", AWP"
	}
	return fmt.Sprintf("R%d [they CT] %s buy; setup %s; %d utility thrown%s; we hit %s -> %s (%s)",
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
			cnt["site "+h.Site]++
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
