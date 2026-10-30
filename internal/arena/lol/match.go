// Package lol és el League of Legends de l'Arena: draft de torneig (bans i
// picks alternats) i una partida a la Clam de l'Invocador resolta per fases
// macro. Cada equip té els mateixos jugadors; només canvia l'estratègia (.md).
package lol

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"

	"gregal/internal/arena"
)

// Game és el joc "lol".
type Game struct{}

func init() { arena.Register(Game{}) }

func (Game) ID() string               { return "lol" }
func (Game) Title(lang string) string { return "League of Legends" }
func (Game) Rules() string            { return rulesText }
func (Game) Starter() string          { return starterText }

func (Game) NewMatch(seed int64, lang string, names [2]string) arena.Match {
	return newMatch(seed, lang, names)
}

// Passos del draft. side: 0 = blau, 1 = vermell.
type dstep struct {
	ban   bool
	sides []int
	label string
}

var draftPlan = []dstep{
	{true, []int{0, 1, 0, 1, 0, 1}, "ban phase 1"},
	{false, []int{0}, "B1"},
	{false, []int{1, 1}, "R1+R2"},
	{false, []int{0, 0}, "B2+B3"},
	{false, []int{1}, "R3"},
	{true, []int{1, 0, 1, 0}, "ban phase 2"},
	{false, []int{1}, "R4"},
	{false, []int{0, 0}, "B4+B5"},
	{false, []int{1}, "R5"},
}

const (
	stLaning = 9 + iota // len(draftPlan)
	stObjectives
	stPlates
	stMacro
)

var handlePool = []string{"Kairo", "Vexa", "Nimbus", "Talo", "Pyx", "Orrin", "Sable", "Juno", "Corvid", "Lumi",
	"Brisk", "Halcyon", "Myrr", "Quill", "Zeph", "Tamsin", "Okra", "Fenn", "Ivo", "Rook"}

var dragonTypes = []string{"infernal", "mountain", "ocean", "cloud", "hextech", "chemtech"}

type match struct {
	rng    *rand.Rand
	seed   int64
	lang   string
	names  [2]string
	blue   int // equip al costat blau
	dec    int
	stage  int
	n      int
	done   bool
	winner int
	out    []arena.Event
	evT    float64 // temps del darrer esdeveniment (s)
	t      float64 // inici de la fase actual (s)

	bans    [2][]int
	picks   [2][]int
	taken   map[int]bool
	line    [2][5]*Champ
	lineID  [2][5]int
	handles [2][5]string
	mod     [2]float64
	shallow bool
	form    float64 // forma del dia (equip 0 respecte de l'1), oculta
	fit     [2]map[string]float64

	gold            [2]float64
	pgold           [2][5]float64
	kills           [2]int
	k, d, a, streak [2][5]int
	firstBlood      bool

	towersLost      [2][3]int
	inhibUntil      [2][3]float64 // 0 = en peu; si no, quan reneix
	inhibsLost      [2]int
	nexusTowersLost [2]int

	dragons    [2][]string
	dragonSeq  []string
	nextDragon float64
	soul       [2]bool
	soulType   string
	elderNext  float64
	elderUntil [2]float64
	elders     [2]int
	grubs      [2]int
	herald     [2]int
	heraldDone bool
	baronNext  float64
	barons     [2]int
	baronUntil [2]float64
	steals     [2]int

	laneDiff   [3]float64 // vist des de l'equip 0
	hist       [2][]string
	laneChoice [2]laneAns
	stHist     [2][]string
	objHist    [2][]string
	plateHist  [2][2]string // focus, action
	graph      [][2]int
	gold15     [2]float64
	has15      bool
}

func newMatch(seed int64, lang string, names [2]string) *match {
	m := &match{rng: rand.New(rand.NewSource(seed)), lang: lang, names: names, taken: map[int]bool{}, winner: -1}
	if lang != "ca" {
		m.lang = "en"
	}
	m.seed = seed
	m.blue = m.rng.Intn(2)
	perm := m.rng.Perm(len(handlePool))
	for i := 0; i < 2; i++ {
		for r := 0; r < 5; r++ {
			m.handles[i][r] = handlePool[perm[i*5+r]]
		}
	}
	// Dracs: els dos primers a l'atzar i diferents; el tercer marca la clam.
	dp := m.rng.Perm(len(dragonTypes))
	m.dragonSeq = []string{dragonTypes[dp[0]], dragonTypes[dp[1]]}
	m.soulType = dragonTypes[dp[2]]
	m.nextDragon = 300
	m.baronNext = 1200
	for i := 0; i < 2; i++ {
		m.gold[i] = 2500
		for r := 0; r < 5; r++ {
			m.pgold[i][r] = 500
		}
	}
	return m
}

func (m *match) teamOf(side int) int {
	if side == 0 {
		return m.blue
	}
	return 1 - m.blue
}
func (m *match) sideOf(team int) int {
	if team == m.blue {
		return 0
	}
	return 1
}
func (m *match) sideName(team int) string {
	if m.sideOf(team) == 0 {
		return "BLUE"
	}
	return "RED"
}

func (m *match) Score() [2]int { return m.kills }
func (m *match) Winner() int   { return m.winner }

func (m *match) tx(en, ca string) string {
	if m.lang == "ca" {
		return ca
	}
	return en
}

// ---- Next / Apply ----

func (m *match) Next() ([2]arena.Prompt, bool) {
	if m.done {
		return [2]arena.Prompt{}, true
	}
	var p [2]arena.Prompt
	for i := 0; i < 2; i++ {
		p[i] = m.prompt(i)
	}
	return p, false
}

func (m *match) Apply(ans [2]json.RawMessage) []arena.Event {
	if m.done {
		return nil
	}
	m.dec++
	m.out = nil
	switch {
	case m.stage < len(draftPlan):
		m.applyDraft(ans)
	case m.stage == stLaning:
		m.applyLaning(ans)
	case m.stage == stObjectives:
		m.applyObjectives(ans)
	case m.stage == stPlates:
		m.applyPlates(ans)
	default:
		m.applyMacro(ans)
	}
	m.stage++
	return m.out
}

// ---- respostes ----

func parseObj(raw json.RawMessage) map[string]any {
	var v map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &v) != nil {
		return map[string]any{}
	}
	// Tolera {"decision": {...}}.
	if d, ok := v["decision"].(map[string]any); ok {
		return d
	}
	return v
}

func getStr(o map[string]any, key string) string {
	for k, v := range o {
		if strings.EqualFold(k, key) {
			if s, ok := v.(string); ok {
				return strings.ToLower(strings.TrimSpace(s))
			}
		}
	}
	return ""
}

func getList(o map[string]any, keys ...string) []string {
	var out []string
	for k, v := range o {
		for _, key := range keys {
			if !strings.EqualFold(k, key) {
				continue
			}
			switch x := v.(type) {
			case string:
				out = append(out, x)
			case []any:
				for _, it := range x {
					switch y := it.(type) {
					case string:
						out = append(out, y)
					case map[string]any:
						for _, f := range []string{"champion", "name", "pick", "ban"} {
							if s, ok := y[f].(string); ok {
								out = append(out, s)
								break
							}
						}
					}
				}
			}
		}
	}
	return out
}

func oneOf(v string, opts []string, def string) string {
	for _, o := range opts {
		if v == o {
			return v
		}
	}
	return def
}

// ---- draft ----

func (m *match) canPick(team, id int) bool {
	if id < 0 || m.taken[id] {
		return false
	}
	return assignRoles(append(append([]int{}, m.picks[team]...), id)) != nil
}

func (m *match) defaultPick(team int) int {
	for i := range pool {
		if m.canPick(team, i) {
			return i
		}
	}
	return m.defaultBan() // cap encaixa: algú fora de rol
}

func (m *match) defaultBan() int {
	for i := range pool {
		if !m.taken[i] {
			return i
		}
	}
	return -1
}

func (m *match) applyDraft(ans [2]json.RawMessage) {
	st := draftPlan[m.stage]
	if m.stage == 0 {
		m.emit("phase", "Draft begins", "Comença el draft", false, map[string]any{"phase": "draft", "blue": m.blue})
	}
	if st.ban {
		lists := [2][]string{}
		ptr := [2]int{}
		for i := 0; i < 2; i++ {
			lists[i] = getList(parseObj(ans[i]), "bans", "ban")
		}
		for _, side := range st.sides {
			team := m.teamOf(side)
			id := -1
			for ptr[team] < len(lists[team]) && id < 0 {
				c := champIndex(lists[team][ptr[team]])
				ptr[team]++
				if c >= 0 && !m.taken[c] {
					id = c
				}
			}
			if id < 0 {
				id = m.defaultBan()
			}
			m.taken[id] = true
			m.bans[team] = append(m.bans[team], id)
			m.emit("draft_ban", fmt.Sprintf("%s bans %s", m.names[team], pool[id].Name), fmt.Sprintf("%s veta %s", m.names[team], pool[id].Name), false,
				map[string]any{"side": team, "color": m.sideName(team), "champ": pool[id].Name, "order": len(m.bans[0]) + len(m.bans[1]), "slot": len(m.bans[team]) - 1, "step": st.label})
		}
		return
	}
	team := m.teamOf(st.sides[0])
	list := getList(parseObj(ans[team]), "picks", "pick", "champion", "champions")
	ptr := 0
	for range st.sides {
		id := -1
		for ptr < len(list) && id < 0 {
			c := champIndex(list[ptr])
			ptr++
			if m.canPick(team, c) {
				id = c
			}
		}
		if id < 0 {
			id = m.defaultPick(team)
		}
		m.taken[id] = true
		m.picks[team] = append(m.picks[team], id)
		roles := assignAny(m.picks[team])
		role := roleNames[roles[len(roles)-1]]
		slot := len(m.picks[team]) - 1
		m.emit("draft_pick", fmt.Sprintf("%s picks %s (%s)", m.names[team], pool[id].Name, role), fmt.Sprintf("%s tria %s (%s)", m.names[team], pool[id].Name, role), false,
			map[string]any{"side": team, "color": m.sideName(team), "champ": pool[id].Name, "role": role, "slot": slot, "order": len(m.picks[0]) + len(m.picks[1]), "step": st.label,
				"class": pool[id].Class, "roles": m.roleMap(team)})
	}
	if m.stage == len(draftPlan)-1 {
		m.finishDraft()
	}
}

// roleMap: campió → rol actual de cada equip (pot canviar amb els flex).
func (m *match) roleMap(team int) map[string]string {
	out := map[string]string{}
	roles := assignAny(m.picks[team])
	for i, id := range m.picks[team] {
		out[pool[id].Name] = roleNames[roles[i]]
	}
	return out
}

func (m *match) finishDraft() {
	for i := 0; i < 2; i++ {
		roles := assignAny(m.picks[i])
		for k, id := range m.picks[i] {
			m.line[i][roles[k]] = &pool[id]
			m.lineID[i][roles[k]] = id
		}
	}
	m.form = m.rng.NormFloat64() * 0.11
	for i := 0; i < 2; i++ {
		m.mod[i] = compMod(m.line[i][:], m.line[1-i][:])
		m.fit[i] = stanceFit(m.line[i][:])
	}
	m.emit("phase", "Draft complete", "Draft acabat", false, map[string]any{"phase": "draft_done", "lineups": m.lineups()})
}

func (m *match) lineups() [2][]map[string]any {
	var out [2][]map[string]any
	for i := 0; i < 2; i++ {
		for r := 0; r < 5; r++ {
			c := m.line[i][r]
			name := ""
			if c != nil {
				name = c.Name
			}
			out[i] = append(out[i], map[string]any{"role": roleNames[r], "handle": m.handles[i][r], "champ": name})
		}
	}
	return out
}

// ---- esdeveniments ----

func (m *match) at(t float64) {
	if t < m.evT {
		t = m.evT
	}
	if m.evT > 0 || m.stage >= stLaning {
		// Ingressos passius de tots dos equips (or/minut d'equip).
		dt := t - m.evT
		rate := 1750.0 / 60
		if t < 120 {
			rate = 600.0 / 60
		}
		for i := 0; i < 2; i++ {
			m.gold[i] += rate * dt
			for r := 0; r < 5; r++ {
				share := []float64{.21, .18, .22, .23, .16}[r]
				m.pgold[i][r] += rate * dt * share
			}
		}
	}
	m.evT = t
	if !m.has15 && t >= 900 {
		m.has15 = true
		m.gold15 = m.gold
	}
	if len(m.graph) == 0 || int(t)-m.graph[len(m.graph)-1][0] >= 60 {
		m.graph = append(m.graph, [2]int{int(t), int(m.gold[0] - m.gold[1])})
	}
}

func (m *match) addGold(team, slot int, g float64) {
	m.gold[team] += g
	m.pgold[team][slot] += g
}

func (m *match) teamGold(team int, g float64) {
	m.gold[team] += g
	for r := 0; r < 5; r++ {
		m.pgold[team][r] += g / 5
	}
}

func (m *match) hud() map[string]any {
	tw := [2]int{}
	for i := 0; i < 2; i++ {
		j := 1 - i
		for l := 0; l < 3; l++ {
			tw[i] += m.towersLost[j][l]
		}
		tw[i] += m.nexusTowersLost[j]
	}
	return map[string]any{
		"k":  m.kills,
		"tw": tw,
		"dr": [2]int{len(m.dragons[0]), len(m.dragons[1])},
		"g":  [2]int{int(m.gold[0]), int(m.gold[1])},
		"bn": m.barons,
		"gr": m.grubs,
		"in": m.inhibsLost,
	}
}

// emitT: com emit però amb el text ja traduït.
func (m *match) emitT(kind, text string, hl bool, data map[string]any) {
	m.emit(kind, text, text, hl, data)
}

func (m *match) emit(kind, en, ca string, hl bool, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["t"] = int(m.evT)
	if m.stage >= stLaning {
		data["hud"] = m.hud()
	}
	raw, _ := json.Marshal(data)
	m.n++
	m.out = append(m.out, arena.Event{N: m.n, Kind: kind, Text: m.tx(en, ca), Highlight: hl, Score: m.kills, Decision: m.dec, Data: raw})
}

// ---- Meta ----

func (m *match) Meta() any {
	teams := []map[string]any{}
	for i := 0; i < 2; i++ {
		side := m.sideOf(i)
		color, color2 := "#2f7df6", "#9cc8ff"
		if side == 1 {
			color, color2 = "#e8414e", "#ffb3b8"
		}
		pl := []map[string]any{}
		for r := 0; r < 5; r++ {
			p := map[string]any{"handle": m.handles[i][r], "role": roleNames[r], "mechanics": 80, "laning": 80, "macro": 80}
			if m.line[i][r] != nil {
				p["champ"] = m.line[i][r].Name
			}
			pl = append(pl, p)
		}
		teams = append(teams, map[string]any{"name": m.names[i], "side": []string{"blue", "red"}[side], "color": color, "color2": color2, "abbr": abbr(m.names[i]), "players": pl})
	}
	champs := []map[string]any{}
	for _, c := range pool {
		champs = append(champs, map[string]any{"name": c.Name, "roles": c.Roles, "class": c.Class, "dmg": c.Dmg, "curve": c.Curve, "traits": traitList(&c)})
	}
	return map[string]any{"game": "lol", "blue": m.blue, "teams": teams, "champions": champs, "map": mapMeta()}
}

func abbr(s string) string {
	s = strings.TrimSpace(s)
	var b []rune
	for _, w := range strings.Fields(s) {
		b = append(b, []rune(strings.ToUpper(w))[0])
	}
	if len(b) < 2 {
		r := []rune(strings.ToUpper(s))
		if len(r) > 3 {
			r = r[:3]
		}
		return string(r)
	}
	if len(b) > 4 {
		b = b[:4]
	}
	return string(b)
}

func traitList(c *Champ) map[string]int {
	return map[string]int{"engage": c.Engage, "peel": c.Peel, "poke": c.Poke, "waveclear": c.Wave, "splitpush": c.Split,
		"pick": c.Pick, "objective": c.Obj, "frontline": c.Front, "dps": c.DPS, "lane": c.Lane}
}

// ---- mapa (0..100, blau a baix a l'esquerra) ----

// Torres blaves per carril: exterior, interior, de base, inhibidor.
var blueStruct = [3][4][2]float64{
	{{9, 30}, {11, 52}, {10, 70}, {10, 77}},
	{{40, 60}, {34, 66}, {26, 74}, {21, 79}},
	{{70, 91}, {48, 89}, {30, 90}, {23, 90}},
}
var blueNexusTowers = [2][2]float64{{15, 83}, {17, 85}}
var blueNexus = [2]float64{12, 88}

var (
	baronPit  = [2]float64{31, 29}
	dragonPit = [2]float64{69, 71}
)

// structXY: side 0 blau, 1 vermell; idx 0..2 torres, 3 inhibidor.
func structXY(side, lane, idx int) [2]float64 {
	if side == 0 {
		return blueStruct[lane][idx]
	}
	p := blueStruct[2-lane][idx]
	return [2]float64{100 - p[0], 100 - p[1]}
}

func mirror(p [2]float64) [2]float64 { return [2]float64{100 - p[0], 100 - p[1]} }

func nexusXY(side int) [2]float64 {
	if side == 0 {
		return blueNexus
	}
	return mirror(blueNexus)
}

func mapMeta() map[string]any {
	towers := []map[string]any{}
	inhibs := []map[string]any{}
	nexus := []map[string]any{}
	for side := 0; side < 2; side++ {
		for l := 0; l < 3; l++ {
			for k := 0; k < 3; k++ {
				p := structXY(side, l, k)
				towers = append(towers, map[string]any{"side": side, "lane": roleLane[l], "tier": k + 1, "x": p[0], "y": p[1]})
			}
			p := structXY(side, l, 3)
			inhibs = append(inhibs, map[string]any{"side": side, "lane": roleLane[l], "x": p[0], "y": p[1]})
		}
		for k := 0; k < 2; k++ {
			p := blueNexusTowers[k]
			if side == 1 {
				p = mirror(p)
			}
			towers = append(towers, map[string]any{"side": side, "lane": "nexus", "tier": 4 + k, "x": p[0], "y": p[1]})
		}
		p := nexusXY(side)
		nexus = append(nexus, map[string]any{"side": side, "x": p[0], "y": p[1]})
	}
	return map[string]any{
		"towers": towers, "inhibitors": inhibs, "nexus": nexus,
		"dragon": map[string]float64{"x": dragonPit[0], "y": dragonPit[1]},
		"baron":  map[string]float64{"x": baronPit[0], "y": baronPit[1]},
		"lanes": map[string][][2]float64{
			"top": {{10, 82}, {9, 14}, {14, 9}, {82, 10}},
			"mid": {{18, 82}, {82, 18}},
			"bot": {{18, 90}, {86, 91}, {91, 86}, {90, 18}},
		},
		"river": [][2]float64{{12, 12}, {31, 33}, {50, 50}, {69, 67}, {88, 88}},
		"bases": []map[string]float64{{"side": 0, "x": 12, "y": 88, "r": 15}, {"side": 1, "x": 88, "y": 12, "r": 15}},
	}
}

var roleLane = [3]string{"top", "mid", "bot"}

func fmtTime(sec float64) string {
	s := int(sec)
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

func round(v float64) int { return int(math.Round(v)) }
