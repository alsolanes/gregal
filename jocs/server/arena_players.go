package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gregal/jocs/arena"
)

// La temporada vista per jugadors: el perfil de cadascú (Elo i evolució,
// forma, principis de prompt, fidelitat al .md), la classificació general,
// els premis, els millors moments de tota la competició i l'informe d'IA de
// cada jugador.
//
// Què és públic (el mateix criteri que /api/arena/match): els números
// (resultats, Elo, notes del coach, % de decisions amb regla trobada) sí;
// el text d'un .md aliè, les regles citades i els moments que les citen, només
// d'un partit oficial acabat (o d'un bot). L'informe d'IA és un per jugador i
// el veu tothom, de manera que només es fa amb dades públiques.

// arenaDeep: el que cal d'un partit acabat i que la capçalera no porta. Es
// llegeix el fitxer sencer un cop (cache per mida i data, com arenaSums).
type arenaDeep struct {
	Followed  [2]float64
	Decisions [2]int
	Fallbacks [2]int
	Scores    [2]map[string]int
	Moments   []arenaHighlight
	MD        [2]string // només dels oficials (públics quan s'acaben)
}

// arenaHighlight: un moment del prompt amb el seu context, per a la galeria
// de millors moments i el perfil.
type arenaHighlight struct {
	Match   string  `json:"match"`
	Game    string  `json:"game"`
	Kind    string  `json:"kind"`
	Stage   string  `json:"stage,omitempty"`
	Side    int     `json:"side"`
	Owner   string  `json:"owner"`
	Display string  `json:"display"`
	Rival   string  `json:"rival"`
	Rule    string  `json:"rule"`
	What    string  `json:"what"`
	Why     string  `json:"why"`
	N       int     `json:"n"`
	Event   int     `json:"event"`
	Impact  float64 `json:"impact"`
	Clutch  bool    `json:"clutch,omitempty"`
	Lead    bool    `json:"lead,omitempty"` // la jugada posa la banda per davant
	Won     bool    `json:"won"`
	Score   [2]int  `json:"score"` // marcador final, de la banda del moment primer
	At      int64   `json:"at"`
}

type arenaDeepEntry struct {
	mod, size int64
	deep      *arenaDeep
}

var (
	arenaDeepMu    sync.Mutex
	arenaDeepCache = map[string]*arenaDeepEntry{}
)

// arenaDeepOf calcula el que cal d'un partit acabat.
func arenaDeepOf(m *arenaMatch) *arenaDeep {
	d := &arenaDeep{Followed: arenaFollowed(m)}
	for _, x := range m.Decisions {
		if x.Side == 0 || x.Side == 1 {
			d.Decisions[x.Side]++
			if x.Fallback {
				d.Fallbacks[x.Side]++
			}
		}
	}
	for i, f := range m.Feedback {
		if f != nil && len(f.Scores) > 0 {
			d.Scores[i] = f.Scores
		}
	}
	if m.Kind == "official" {
		d.MD = m.MD
	}
	arenaWithMoments(m)
	if m.Analysis == nil {
		return d
	}
	gains := arenaGains(m)
	maxGain := 1
	for _, g := range gains {
		maxGain = max(maxGain, g[0], g[1])
	}
	pos := map[int]int{} // número de jugada → índex
	for i, e := range m.Events {
		pos[e.N] = i
	}
	for _, mo := range m.Analysis.Moments {
		if mo.Side != 0 && mo.Side != 1 {
			continue
		}
		g, first, last := 0, -1, -1
		for i, e := range m.Events {
			if e.Decision == mo.N {
				g += gains[i][mo.Side]
				if first < 0 {
					first = i
				}
				last = i
			}
		}
		impact := math.Min(1.5, float64(g)/float64(maxGain))
		// Posar-se per davant val més que eixamplar una diferència.
		lead := false
		if first >= 0 {
			var before [2]int
			if first > 0 {
				before = m.Events[first-1].Score
			}
			after, me, them := m.Events[last].Score, mo.Side, 1-mo.Side
			if before[me] <= before[them] && after[me] > after[them] {
				lead = true
				impact += .4
			}
		}
		impact += arenaRivalWeight(m, mo.Side)
		won := m.Winner == mo.Side
		if won {
			impact += .4
		}
		clutch := false
		if i, ok := pos[mo.Event]; ok && len(m.Events) > 4 && i >= len(m.Events)*4/5 {
			clutch = true
			impact += .3
		}
		if m.Kind == "official" {
			impact += .2
		}
		d.Moments = append(d.Moments, arenaHighlight{
			Match: m.ID, Game: m.Game, Kind: m.Kind, Stage: m.Stage, Side: mo.Side,
			Owner: m.Owners[mo.Side], Display: m.Names[mo.Side], Rival: m.Names[1-mo.Side],
			Rule: mo.Rule, What: mo.What, Why: mo.Why, N: mo.N, Event: mo.Event,
			Impact: math.Round(impact*100) / 100, Clutch: clutch, Lead: lead, Won: won,
			Score: [2]int{m.Score[mo.Side], m.Score[1-mo.Side]}, At: max(m.Finished, m.Started),
		})
	}
	return d
}

// arenaRivalWeight: un moment contra un rival fort val més (els bots per
// nivell; una persona, en un oficial, com un bot expert).
func arenaRivalWeight(m *arenaMatch, side int) float64 {
	switch o := m.Owners[1-side]; {
	case o == "bot:analyst" || !isBotOwner(o):
		return .3
	case o == "bot:turtle" || o == "bot:aggro":
		return .15
	}
	return 0
}

// arenaDeeps: el detall de cada partit acabat de sums (per id).
func (s *Server) arenaDeeps(sums []arenaSum) map[string]*arenaDeep {
	out := make(map[string]*arenaDeep, len(sums))
	for i := range sums {
		m := &sums[i].M
		if m.Status != "final" {
			continue
		}
		path := s.matchPath(m.ID)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		mod, size := info.ModTime().UnixNano(), info.Size()
		arenaDeepMu.Lock()
		c := arenaDeepCache[path]
		arenaDeepMu.Unlock()
		if c == nil || c.mod != mod || c.size != size {
			var full arenaMatch
			if !arenaRead(path, &full) || full.Status != "final" {
				continue
			}
			if renum := arenaRenumber(&full); renum != nil {
				for j, n := range renum {
					full.Decisions[j].N = n
				}
			}
			c = &arenaDeepEntry{mod: mod, size: size, deep: arenaDeepOf(&full)}
			arenaDeepMu.Lock()
			arenaDeepCache[path] = c
			arenaDeepMu.Unlock()
		}
		out[m.ID] = c.deep
	}
	return out
}

// ---- la temporada ----

type arenaPlayerGame struct {
	Elo     float64      `json:"elo"` // oficial; 0 = encara sense oficials
	Rank    int          `json:"rank,omitempty"`
	Played  int          `json:"played"`
	Won     int          `json:"won"`
	Lost    int          `json:"lost"`
	Drawn   int          `json:"drawn"`
	History [][2]float64 `json:"history,omitempty"` // [unix, Elo] després de cada oficial
	Bots    []string     `json:"bots"`              // bots guanyats (nivells)
	// Les versions del .md en aquest joc, amb el resultat de cadascuna (arena_quality.go).
	Versions []arenaMDVersion `json:"versions,omitempty"`
}

type arenaPlayer struct {
	Owner      string                      `json:"owner"`
	Display    string                      `json:"display"`
	Rank       int                         `json:"rank"`
	Games      map[string]*arenaPlayerGame `json:"games"`
	Played     int                         `json:"played"`
	Won        int                         `json:"won"`
	Lost       int                         `json:"lost"`
	Drawn      int                         `json:"drawn"`
	Official   [3]int                      `json:"official"` // guanyats, perduts, empatats
	Elo        float64                     `json:"elo"`      // mitjana de l'Elo oficial dels jocs on n'ha jugat
	Form       []string                    `json:"form"`     // els darrers 5: W, L, D (el més nou al final)
	Streak     int                         `json:"streak"`   // victòries seguides ara mateix
	BestStreak int                         `json:"best_streak"`
	BotsBeaten int                         `json:"bots_beaten"`
	Followed   float64                     `json:"followed"` // 0-1: decisions amb una regla que és al .md
	Principles map[string]float64          `json:"principles,omitempty"`
	Reviewed   int                         `json:"reviewed"`     // partits amb nota del coach
	Prompt     int                         `json:"prompt_score"` // 0-100: la mitjana de les notes del coach
	CoachFirst float64                     `json:"coach_first,omitempty"`
	CoachLast  float64                     `json:"coach_last,omitempty"`
	Moments    int                         `json:"moments"`
	Last       int64                       `json:"last"`
	// Contra cada nivell de bot (rookie, solid, expert): l'escala de qualitat.
	VsBots map[string]*arenaWL `json:"vs_bots,omitempty"`
	// De l'informe d'IA desat (si n'hi ha): el sobrenom per a les targetes.
	Archetype string `json:"archetype,omitempty"`
	Emoji     string `json:"emoji,omitempty"`
}

type arenaAward struct {
	ID      string `json:"id"`
	Icon    string `json:"icon"`
	Title   string `json:"title"`
	Owner   string `json:"owner"`
	Display string `json:"display"`
	Value   string `json:"value"`
	Why     string `json:"why"`
	Match   string `json:"match,omitempty"`
}

type arenaSeasonView struct {
	Players    []*arenaPlayer   `json:"players"`
	Awards     []arenaAward     `json:"awards"`
	Highlights []arenaHighlight `json:"highlights"`
	Quality    *arenaQuality    `json:"quality,omitempty"`
}

// arenaPrinciples: els cinc criteris del coach, en l'ordre que es mostren.
var arenaPrinciples = []string{"clarity", "specificity", "coverage", "adaptivity", "consistency"}

const (
	arenaMinForAward     = 3
	arenaHighlightsMax   = 40
	arenaProfileMoments  = 8
	arenaMomentsPerMatch = 2
)

func isBotOwner(o string) bool {
	return strings.HasPrefix(o, "bot:") || strings.HasPrefix(o, "demo:")
}

// arenaVisibleSide: el que diu el .md d'aquesta banda (regles, moments) es pot
// ensenyar a me.
func arenaVisibleSide(m *arenaMatch, side int, me string) bool {
	o := m.Owners[side]
	return o == me || isBotOwner(o) || m.Kind == "official" && m.Status == "final"
}

// arenaEloReplay refà l'Elo oficial partit a partit (com arenaRecord): la
// corba de cada jugador i l'Elo de tots dos just abans de cada partit.
func arenaEloReplay(finals []*arenaMatch) (hist map[string]map[string][][2]float64, before map[string][2]float64) {
	hist = map[string]map[string][][2]float64{}
	before = map[string][2]float64{}
	elo := map[string]map[string]float64{}
	for _, m := range finals {
		if m.Kind != "official" {
			continue
		}
		g := elo[m.Game]
		if g == nil {
			g = map[string]float64{}
			elo[m.Game] = g
		}
		get := func(o string) float64 {
			if v, ok := g[o]; ok {
				return v
			}
			return 1500
		}
		a, b := get(m.Owners[0]), get(m.Owners[1])
		before[m.ID] = [2]float64{a, b}
		a, b = eloUpdate(a, b, m.Winner)
		g[m.Owners[0]], g[m.Owners[1]] = a, b
		at := float64(max(m.Finished, m.Started))
		if hist[m.Game] == nil {
			hist[m.Game] = map[string][][2]float64{}
		}
		for i, v := range []float64{a, b} {
			o := m.Owners[i]
			hist[m.Game][o] = append(hist[m.Game][o], [2]float64{at, math.Round(v)})
		}
	}
	return hist, before
}

// arenaSeason: perfils, classificació, premis i millors moments, vistos per me.
// ratings és l'Elo oficial desat (el que mana); displays, el nom de cadascú.
func arenaSeason(sums []arenaSum, deeps map[string]*arenaDeep, ratings map[string]map[string]*arenaRating, displays map[string]string, me string) arenaSeasonView {
	var finals []*arenaMatch
	coach := map[string][2]int{}
	sigs := map[string]arenaMDSigs{}
	for i := range sums {
		m := &sums[i].M
		if m.Status == "final" {
			finals = append(finals, m)
			coach[m.ID] = sums[i].Coach
			sigs[m.ID] = sums[i].MD
		}
	}
	at := func(m *arenaMatch) int64 { return max(m.Finished, m.Started) }
	sort.SliceStable(finals, func(i, j int) bool { return at(finals[i]) < at(finals[j]) })
	hist, before := arenaEloReplay(finals)

	players := map[string]*arenaPlayer{}
	get := func(o string) *arenaPlayer {
		if p := players[o]; p != nil {
			return p
		}
		d := displays[o]
		if d == "" {
			d = o
		}
		p := &arenaPlayer{Owner: o, Display: d, Games: map[string]*arenaPlayerGame{}, Form: []string{}}
		players[o] = p
		return p
	}
	game := func(p *arenaPlayer, g string) *arenaPlayerGame {
		if x := p.Games[g]; x != nil {
			return x
		}
		x := &arenaPlayerGame{Bots: []string{}}
		p.Games[g] = x
		return x
	}
	type acc struct {
		followed, followedN float64
		princ               map[string]float64
		coach               []float64
		results             []string
	}
	accs := map[string]*acc{}
	accOf := func(o string) *acc {
		if a := accs[o]; a != nil {
			return a
		}
		a := &acc{princ: map[string]float64{}}
		accs[o] = a
		return a
	}
	var giant struct {
		owner, match string
		gap          float64
	}
	for _, m := range finals {
		deep := deeps[m.ID]
		for side := 0; side < 2; side++ {
			o := m.Owners[side]
			if o == "" || isBotOwner(o) {
				continue
			}
			p, a := get(o), accOf(o)
			if displays[o] == "" && m.Names[side] != "" {
				p.Display = m.Names[side]
			}
			res := "D"
			switch m.Winner {
			case side:
				res = "W"
			case 1 - side:
				res = "L"
			}
			p.Played++
			switch res {
			case "W":
				p.Won++
			case "L":
				p.Lost++
			default:
				p.Drawn++
			}
			a.results = append(a.results, res)
			p.Last = max(p.Last, at(m))
			if m.Kind == "official" {
				switch res {
				case "W":
					p.Official[0]++
				case "L":
					p.Official[1]++
				default:
					p.Official[2]++
				}
				if b, ok := before[m.ID]; ok && res == "W" {
					if gap := b[1-side] - b[side]; gap > giant.gap {
						giant.owner, giant.match, giant.gap = o, m.ID, gap
					}
				}
			}
			g := game(p, m.Game)
			if res == "W" && isBotOwner(m.Owners[1-side]) {
				lvl := strings.TrimPrefix(m.Owners[1-side], "bot:")
				seen := false
				for _, b := range g.Bots {
					seen = seen || b == lvl
				}
				if !seen {
					g.Bots = append(g.Bots, lvl)
					p.BotsBeaten++
				}
			}
			if deep != nil && deep.Decisions[side] > 0 {
				a.followed += deep.Followed[side]
				a.followedN++
			}
			if deep != nil && deep.Scores[side] != nil {
				for _, k := range arenaPrinciples {
					a.princ[k] += float64(deep.Scores[side][k])
				}
				p.Reviewed++
			}
			if c := coach[m.ID][side]; c > 0 {
				a.coach = append(a.coach, float64(c))
			}
		}
	}
	// L'Elo oficial (el desat) i la corba.
	for g, byOwner := range ratings {
		for o, r := range byOwner {
			if isBotOwner(o) {
				continue
			}
			p := get(o)
			x := game(p, g)
			x.Elo, x.Played, x.Won, x.Lost, x.Drawn = math.Round(r.Elo), r.Played, r.Won, r.Lost, r.Drawn
			x.History = hist[g][o]
		}
		// La posició al joc, comptant-hi els bots (com el rànquing de la web).
		type row struct {
			o   string
			elo float64
		}
		var rows []row
		for o, r := range byOwner {
			rows = append(rows, row{o, r.Elo})
		}
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].elo > rows[j].elo || rows[i].elo == rows[j].elo && rows[i].o < rows[j].o
		})
		for i, r := range rows {
			if p := players[r.o]; p != nil && p.Games[g] != nil {
				p.Games[g].Rank = i + 1
			}
		}
	}
	// Els que han desat un .md però encara no han jugat també hi són.
	for o := range displays {
		if !isBotOwner(o) && o != "" {
			get(o)
		}
	}
	for o, p := range players {
		a := accOf(o)
		n, sum := 0, 0.0
		for _, x := range p.Games {
			if x.Played > 0 && x.Elo > 0 {
				n++
				sum += x.Elo
			}
		}
		if n > 0 {
			p.Elo = math.Round(sum / float64(n))
		}
		if a.followedN > 0 {
			p.Followed = math.Round(1000*a.followed/a.followedN) / 1000
		}
		if p.Reviewed > 0 {
			p.Principles = map[string]float64{}
			for _, k := range arenaPrinciples {
				p.Principles[k] = math.Round(10*a.princ[k]/float64(p.Reviewed)) / 10
			}
		}
		if len(a.coach) > 0 {
			t := 0.0
			for _, c := range a.coach {
				t += c
			}
			p.Prompt = int(math.Round(100 * t / float64(len(a.coach)) / 25))
			p.CoachFirst, p.CoachLast = a.coach[0], a.coach[len(a.coach)-1]
		}
		run := 0
		for _, r := range a.results {
			if r == "W" {
				run++
				p.BestStreak = max(p.BestStreak, run)
			} else {
				run = 0
			}
		}
		p.Streak = run
		if k := len(a.results); k > 5 {
			p.Form = append(p.Form, a.results[k-5:]...)
		} else {
			p.Form = append(p.Form, a.results...)
		}
	}

	// Els millors moments que me pot veure (de persones, no de bots).
	var hl []arenaHighlight
	for _, m := range finals {
		deep := deeps[m.ID]
		if deep == nil {
			continue
		}
		for _, h := range deep.Moments {
			if isBotOwner(h.Owner) {
				continue
			}
			// El recompte (premis) és de tothom igual; el moment, només si es pot veure.
			p := players[h.Owner]
			if p != nil {
				p.Moments++
				h.Display = p.Display
			}
			if arenaVisibleSide(m, h.Side, me) {
				hl = append(hl, h)
			}
		}
	}
	sort.SliceStable(hl, func(i, j int) bool {
		if hl[i].Impact != hl[j].Impact {
			return hl[i].Impact > hl[j].Impact
		}
		return hl[i].At > hl[j].At
	})
	// Com a molt dos moments per partit: la galeria és de tota la temporada.
	perMatch := map[string]int{}
	kept := []arenaHighlight{}
	for _, h := range hl {
		if perMatch[h.Match] < arenaMomentsPerMatch {
			perMatch[h.Match]++
			kept = append(kept, h)
		}
	}
	hl = kept

	list := make([]*arenaPlayer, 0, len(players))
	for _, p := range players {
		list = append(list, p)
	}
	// La classificació general: primer qui té Elo oficial (més alt primer),
	// després l'escala de bots, la nota de prompt i les victòries.
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.Elo != b.Elo {
			return a.Elo > b.Elo
		}
		if a.BotsBeaten != b.BotsBeaten {
			return a.BotsBeaten > b.BotsBeaten
		}
		if a.Prompt != b.Prompt {
			return a.Prompt > b.Prompt
		}
		if a.Won != b.Won {
			return a.Won > b.Won
		}
		return a.Display < b.Display
	})
	for i, p := range list {
		p.Rank = i + 1
	}
	quality := arenaQualityOf(finals, coach, sigs, players)
	return arenaSeasonView{Players: list, Awards: arenaAwards(list, giant.owner, giant.match, giant.gap), Highlights: hl, Quality: quality}
}

// arenaAwards: els premis de la temporada, calculats (cap model no hi vota).
// Un premi sense prou dades no surt.
func arenaAwards(list []*arenaPlayer, giant, giantMatch string, gap float64) []arenaAward {
	var out []arenaAward
	by := map[string]*arenaPlayer{}
	for _, p := range list {
		by[p.Owner] = p
	}
	best := func(ok func(*arenaPlayer) bool, val func(*arenaPlayer) float64) *arenaPlayer {
		var b *arenaPlayer
		for _, p := range list {
			if !ok(p) {
				continue
			}
			if b == nil || val(p) > val(b) {
				b = p
			}
		}
		return b
	}
	add := func(p *arenaPlayer, id, icon, title, value, why string) {
		if p != nil {
			out = append(out, arenaAward{ID: id, Icon: icon, Title: title, Owner: p.Owner, Display: p.Display, Value: value, Why: why})
		}
	}
	if len(list) > 0 && (list[0].Elo > 0 || list[0].BotsBeaten > 0) {
		p := list[0]
		v := fmt.Sprintf("%d bots beaten", p.BotsBeaten)
		if p.Elo > 0 {
			v = fmt.Sprintf("%.0f Elo", p.Elo)
		}
		add(p, "leader", "👑", "Season leader", v, "Top of the overall standings.")
	}
	if p := best(func(p *arenaPlayer) bool { return p.Reviewed >= arenaMinForAward }, func(p *arenaPlayer) float64 { return float64(p.Prompt) }); p != nil {
		add(p, "architect", "🧠", "Prompt architect", fmt.Sprintf("%d/100", p.Prompt), "Best average coach score on the five prompting principles.")
	}
	if p := best(func(p *arenaPlayer) bool { return p.Played >= arenaMinForAward && p.Followed > 0 }, func(p *arenaPlayer) float64 { return p.Followed }); p != nil {
		add(p, "by_the_book", "📜", "By the book", fmt.Sprintf("%.0f%% of calls", 100*p.Followed), "The model found a real rule of their .md behind the most decisions.")
	}
	if p := by[giant]; p != nil && gap >= 50 {
		out = append(out, arenaAward{ID: "giant_killer", Icon: "🗡️", Title: "Giant killer", Owner: p.Owner, Display: p.Display, Value: fmt.Sprintf("+%.0f Elo gap", gap), Why: "Biggest upset in an official match.", Match: giantMatch})
	}
	if p := best(func(p *arenaPlayer) bool { return p.BestStreak >= arenaMinForAward }, func(p *arenaPlayer) float64 { return float64(p.BestStreak) }); p != nil {
		add(p, "on_fire", "🔥", "On fire", fmt.Sprintf("%d wins in a row", p.BestStreak), "Longest winning streak of the season.")
	}
	if p := best(func(p *arenaPlayer) bool { return p.Reviewed >= arenaMinForAward && p.CoachLast > p.CoachFirst }, func(p *arenaPlayer) float64 { return p.CoachLast - p.CoachFirst }); p != nil {
		add(p, "most_improved", "📈", "Most improved", fmt.Sprintf("+%.0f coach points", p.CoachLast-p.CoachFirst), "Biggest jump between the first and the latest coach score.")
	}
	moments := map[string]int{}
	for _, p := range list {
		moments[p.Owner] = p.Moments
	}
	if p := best(func(p *arenaPlayer) bool { return moments[p.Owner] >= 2 }, func(p *arenaPlayer) float64 { return float64(moments[p.Owner]) }); p != nil {
		add(p, "playmaker", "🎬", "Playmaker", fmt.Sprintf("%d prompt moments", moments[p.Owner]), "Most moments where a line of the .md swung a match.")
	}
	if out == nil {
		out = []arenaAward{}
	}
	return out
}

// arenaSeasonFor junta les dades del servidor per a me.
func (s *Server) arenaSeasonFor(me string) arenaSeasonView {
	sums := s.arenaSummaries()
	v := arenaSeason(sums, s.arenaDeeps(sums), s.arenaRatings(), s.arenaDisplays(), me)
	s.arenaTagPlayers(v.Players)
	return v
}

// arenaTagPlayers posa a cada jugador el sobrenom del seu informe desat.
func (s *Server) arenaTagPlayers(ps []*arenaPlayer) {
	for _, p := range ps {
		if in := s.arenaInsightSaved(p.Owner); in != nil {
			p.Archetype, p.Emoji = in.Archetype, in.Emoji
		}
	}
}

// arenaDisplays: el nom de cada jugador, el del .md desat més recentment.
func (s *Server) arenaDisplays() map[string]string {
	displays := map[string]string{}
	newest := map[string]int64{}
	for _, g := range arena.Games() {
		for _, e := range s.arenaEntries(g.ID()) {
			if e.Updated >= newest[e.Owner] && e.Display != "" {
				displays[e.Owner], newest[e.Owner] = e.Display, e.Updated
			}
		}
	}
	return displays
}

// ---- l'informe d'IA de cada jugador ----

type arenaInsight struct {
	Archetype string `json:"archetype"`
	Emoji     string `json:"emoji"`
	Tagline   string `json:"tagline"`
	Summary   string `json:"summary"`
	Signature string `json:"signature"`
	Strength  string `json:"strength"`
	Weakness  string `json:"weakness"`
	Tip       string `json:"tip"`
	Principle string `json:"principle"` // el principi del tip
	Rival     string `json:"rival"`     // com se li guanya
	At        int64  `json:"at"`
	Hash      string `json:"hash"`
	Matches   int    `json:"matches"`
}

func (s *Server) insightPath(owner string) string {
	return filepath.Join(s.arenaDir(), "insights", safeName(owner)+".json")
}

var (
	arenaInsightMu   sync.Mutex
	arenaInsightBusy = map[string]bool{}
)

// arenaInsightBasis: el que el model llegeix d'un jugador, només públic: els
// números de la temporada, els moments dels oficials i el darrer .md oficial.
func arenaInsightBasis(p *arenaPlayer, finals []*arenaMatch, deeps map[string]*arenaDeep) string {
	var b strings.Builder
	fmt.Fprintf(&b, "PLAYER: %s\n", p.Display)
	fmt.Fprintf(&b, "Overall: %d played, %d won, %d lost, %d drawn. Official: %d-%d-%d. Current streak: %d wins (best %d). Bots beaten: %d.\n",
		p.Played, p.Won, p.Lost, p.Drawn, p.Official[0], p.Official[1], p.Official[2], p.Streak, p.BestStreak, p.BotsBeaten)
	games := make([]string, 0, len(p.Games))
	for g := range p.Games {
		games = append(games, g)
	}
	sort.Strings(games)
	for _, g := range games {
		x := p.Games[g]
		elo := "no official matches yet"
		if x.Elo > 0 {
			elo = fmt.Sprintf("official Elo %.0f (rank %d)", x.Elo, x.Rank)
		}
		fmt.Fprintf(&b, "- Game %s: %s, bots beaten: %s\n", g, elo, strings.Join(x.Bots, ", "))
	}
	if p.Followed > 0 {
		fmt.Fprintf(&b, "Rule adherence: the model found a real line of their .md behind %.0f%% of decisions.\n", 100*p.Followed)
	}
	if p.Principles != nil {
		var parts []string
		for _, k := range arenaPrinciples {
			parts = append(parts, fmt.Sprintf("%s %.1f", k, p.Principles[k]))
		}
		fmt.Fprintf(&b, "Coach scores (1-5, average of %d matches): %s.\n", p.Reviewed, strings.Join(parts, ", "))
	}
	if p.CoachLast > 0 {
		fmt.Fprintf(&b, "Coach total first match %.0f/25, latest %.0f/25.\n", p.CoachFirst, p.CoachLast)
	}
	// Moments i .md: només dels oficials (públics).
	var mo []string
	md := ""
	for i := len(finals) - 1; i >= 0; i-- {
		m := finals[i]
		if m.Kind != "official" {
			continue
		}
		d := deeps[m.ID]
		if d == nil {
			continue
		}
		for side := 0; side < 2; side++ {
			if m.Owners[side] != p.Owner {
				continue
			}
			if md == "" && strings.TrimSpace(d.MD[side]) != "" {
				md = d.MD[side]
			}
			for _, h := range d.Moments {
				if h.Side == side && len(mo) < 6 {
					mo = append(mo, fmt.Sprintf("- %s vs %s: %s Rule: «%s»", m.Game, h.Rival, h.What, oneLine(h.Rule, 160)))
				}
			}
		}
	}
	if len(mo) > 0 {
		b.WriteString("Key moments in official matches (a line of their .md that swung the game):\n" + strings.Join(mo, "\n") + "\n")
	}
	if md != "" {
		if r := []rune(md); len(r) > 3500 {
			md = string(r[:3500]) + "\n[…]"
		}
		b.WriteString("Their strategy .md from the latest official match:\n<<<\n" + md + "\n>>>\n")
	} else {
		b.WriteString("Their .md is still secret (no official match yet): judge only from the numbers, do not invent rules.\n")
	}
	return b.String()
}

func arenaInsightHash(basis string) string {
	h := sha256.Sum256([]byte(basis))
	return hex.EncodeToString(h[:8])
}

func (s *Server) arenaInsightSaved(owner string) *arenaInsight {
	var in arenaInsight
	if !arenaRead(s.insightPath(owner), &in) {
		return nil
	}
	return &in
}

// arenaMakeInsight escriu (o torna, si les dades no han canviat) l'informe
// d'un jugador.
func (s *Server) arenaMakeInsight(ctx context.Context, p *arenaPlayer, basis string) (*arenaInsight, error) {
	hash := arenaInsightHash(basis)
	if old := s.arenaInsightSaved(p.Owner); old != nil && old.Hash == hash {
		return old, nil
	}
	arenaInsightMu.Lock()
	if arenaInsightBusy[p.Owner] {
		arenaInsightMu.Unlock()
		return nil, errConst("already writing this report")
	}
	arenaInsightBusy[p.Owner] = true
	arenaInsightMu.Unlock()
	defer func() {
		arenaInsightMu.Lock()
		delete(arenaInsightBusy, p.Owner)
		arenaInsightMu.Unlock()
	}()
	system := "You are the lead analyst of an internal prompt-engineering competition at a company: colleagues write strategy .md files and the same AI model plays every match following them. You write short, fun, insightful scouting reports about each player, like a sports magazine, to help everyone learn to write better prompts. Be warm and witty, never mean. Base every claim on the data; never invent rules that are not quoted. Reply with JSON only."
	user := basis + "\nWrite the scouting report in English. archetype = a catchy 2-4 word nickname for their style (e.g. \"The Clock Whisperer\"). emoji = one emoji. tagline = max 12 words. summary = 2-3 sentences on how they play and why it works or not. signature = their defining habit or rule (quote a rule if you have one). strength and weakness = one sentence each. tip = one concrete prompt-writing tip for their next version, tied to one principle; principle = that principle (clarity, specificity, coverage, adaptivity or consistency). rival = one sentence on how an opponent could beat them.\n" +
		`JSON: {"archetype":"…","emoji":"…","tagline":"…","summary":"…","signature":"…","strength":"…","weakness":"…","tip":"…","principle":"…","rival":"…"}`
	raw, err := arenaModel(s, ctx, system, user, 900)
	if err != nil {
		return nil, err
	}
	var in arenaInsight
	if obj := firstJSONObject(raw); obj == "" || json.Unmarshal([]byte(obj), &in) != nil || strings.TrimSpace(in.Archetype) == "" {
		return nil, errConst("the analyst did not answer in JSON")
	}
	in.Archetype, in.Emoji, in.Tagline = oneLine(in.Archetype, 40), oneLine(in.Emoji, 8), oneLine(in.Tagline, 120)
	in.Summary, in.Signature = oneLine(in.Summary, 500), oneLine(in.Signature, 300)
	in.Strength, in.Weakness, in.Tip, in.Rival = oneLine(in.Strength, 240), oneLine(in.Weakness, 240), oneLine(in.Tip, 300), oneLine(in.Rival, 240)
	in.Principle = strings.ToLower(oneLine(in.Principle, 20))
	in.At, in.Hash, in.Matches = time.Now().Unix(), hash, p.Played
	arenaMu.Lock()
	arenaWrite(s.insightPath(p.Owner), in)
	arenaMu.Unlock()
	return &in, nil
}

// handleArenaSeason: GET → {players, awards, highlights} vistos per qui ho demana.
func (s *Server) handleArenaSeason(w http.ResponseWriter, r *http.Request) {
	me, _ := s.arenaUser(r)
	v := s.arenaSeasonFor(me)
	if len(v.Highlights) > arenaHighlightsMax {
		v.Highlights = v.Highlights[:arenaHighlightsMax]
	}
	writeJSON(w, v)
}

// handleArenaPlayer: GET ?owner= → {player, highlights, matches, insight,
// insight_stale}; POST {action:"insight", owner} escriu l'informe d'IA (si
// les dades han canviat; si no, torna el desat).
func (s *Server) handleArenaPlayer(w http.ResponseWriter, r *http.Request) {
	me, _ := s.arenaUser(r)
	owner := r.URL.Query().Get("owner")
	if r.Method == http.MethodPost {
		var req struct{ Action, Owner string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil || req.Action != "insight" {
			http.Error(w, "unknown action", http.StatusBadRequest)
			return
		}
		owner = req.Owner
	}
	if owner == "" {
		owner = me
	}
	if isBotOwner(owner) {
		http.Error(w, "bots have no profile", http.StatusNotFound)
		return
	}
	sums := s.arenaSummaries()
	deeps := s.arenaDeeps(sums)
	v := arenaSeason(sums, deeps, s.arenaRatings(), s.arenaDisplays(), me)
	s.arenaTagPlayers(v.Players)
	var p *arenaPlayer
	for _, x := range v.Players {
		if x.Owner == owner {
			p = x
		}
	}
	if p == nil {
		http.Error(w, "no such player", http.StatusNotFound)
		return
	}
	var finals []*arenaMatch
	for i := range sums {
		if sums[i].M.Status == "final" {
			finals = append(finals, &sums[i].M)
		}
	}
	sort.SliceStable(finals, func(i, j int) bool {
		return max(finals[i].Finished, finals[i].Started) < max(finals[j].Finished, finals[j].Started)
	})
	basis := arenaInsightBasis(p, finals, deeps)
	if r.Method == http.MethodPost {
		if p.Played == 0 {
			http.Error(w, "no matches yet", http.StatusConflict)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		in, err := s.arenaMakeInsight(ctx, p, basis)
		if err != nil {
			writeJSONStatus(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"insight": in, "insight_stale": false})
		return
	}
	hl := []arenaHighlight{}
	for _, h := range v.Highlights {
		if h.Owner == owner && len(hl) < arenaProfileMoments {
			hl = append(hl, h)
		}
	}
	// Els partits del jugador que me pot obrir: els oficials i, si és ell, tots.
	matches := []map[string]any{}
	for i := len(finals) - 1; i >= 0 && len(matches) < 12; i-- {
		m := finals[i]
		side := -1
		for k, o := range m.Owners {
			if o == owner {
				side = k
			}
		}
		if side < 0 || m.Kind != "official" && owner != me {
			continue
		}
		res := "D"
		if m.Winner == side {
			res = "W"
		} else if m.Winner == 1-side {
			res = "L"
		}
		matches = append(matches, map[string]any{"id": m.ID, "game": m.Game, "kind": m.Kind, "stage": m.Stage, "rival": m.Names[1-side],
			"result": res, "score": [2]int{m.Score[side], m.Score[1-side]}, "at": max(m.Finished, m.Started)})
	}
	in := s.arenaInsightSaved(owner)
	writeJSON(w, map[string]any{"player": p, "highlights": hl, "matches": matches, "insight": in,
		"insight_stale": p.Played > 0 && (in == nil || in.Hash != arenaInsightHash(basis))})
}
