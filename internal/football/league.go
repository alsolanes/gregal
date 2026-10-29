package football

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"strings"
)

// Wind és el que la lliga necessita d'un vent de l'usuari.
type Wind struct {
	ID, Name, Role, Specialty, Color, Style string
}

// League és la lliga dels vents: dues franquícies i els seus jugadors. Els
// jugadors que no són vents de l'usuari són «jugadors d'equip»: tenen nom,
// cara, dorsal i atributs, i es guarden aquí, no a la llista de vents.
type League struct {
	Teams  [2]*Team `json:"teams"`
	Season int      `json:"season"`
	Games  int      `json:"games"`
}

// Noms de vent per als jugadors d'equip: vents de tot el món.
var windNames = []string{
	"Siroco", "Mistral", "Bora", "Foehn", "Chinook", "Zonda", "Simún", "Harmattan", "Levante", "Etesi", "Khamsin", "Haboob",
	"Williwaw", "Santa Ana", "Brisa", "Oratge", "Terral", "Cierzo", "Solano", "Pampero", "Vendaval", "Torb", "Rufa", "Bufarut",
	"Tramontana", "Ostro", "Libeccio", "Grecale", "Scirocco", "Maestrale", "Ponente", "Galerna", "Sudestada", "Surazo",
	"Papagayo", "Tehuano", "Brickfielder", "Southerly", "Nor'wester", "Kona", "Elvegust", "Helm", "Pittarak", "Piteraq",
	"Sukhovei", "Buran", "Karaburan", "Shamal", "Barat", "Belat", "Koembang", "Datoo", "Leste", "Gharbi", "Marin",
	"Autan", "Tramontane", "Vent d'Aval", "Joran", "Bise", "Vardar", "Košava", "Gregale", "Jugo", "Burin", "Maistro",
	"Embat", "Cerç", "Llevantada", "Xaloquet", "Garbinada", "Migjorn", "Ventolina", "Oratjol", "Remolí", "Bufada",
}

// Perfils d'atributs per posició: la mitjana de cada atribut (la resta, ~55).
var posProfile = map[string]Attrs{
	QB: {Speed: 62, Strength: 58, Agility: 62, Arm: 82, Accuracy: 80, Hands: 58, Awareness: 78},
	RB: {Speed: 82, Strength: 74, Agility: 82, Hands: 66, Awareness: 66, Block: 55},
	WR: {Speed: 86, Strength: 58, Agility: 82, Hands: 78, Awareness: 68},
	TE: {Speed: 70, Strength: 74, Agility: 66, Hands: 72, Block: 70, Awareness: 66},
	OL: {Speed: 52, Strength: 82, Agility: 55, Block: 80, Awareness: 68},
	DL: {Speed: 70, Strength: 82, Agility: 64, Tackle: 76, Awareness: 64},
	LB: {Speed: 74, Strength: 72, Agility: 70, Tackle: 78, Coverage: 64, Awareness: 70},
	CB: {Speed: 88, Strength: 58, Agility: 84, Tackle: 62, Coverage: 78, Hands: 62, Awareness: 68},
	S:  {Speed: 82, Strength: 66, Agility: 76, Tackle: 72, Coverage: 74, Hands: 64, Awareness: 72},
	K:  {Speed: 50, Strength: 55, Agility: 52, Kick: 80, Awareness: 60},
}

func seedOf(s string) int64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return int64(h.Sum64() & 0x7fffffffffffffff)
}

func roll(r *rand.Rand, mean int, sd float64) int {
	if mean == 0 {
		mean = 55
	}
	v := int(float64(mean) + sd*r.NormFloat64())
	return clampi(v, 40, 99)
}

// makeAttrs genera els atributs d'un jugador per a una posició. star puja
// la mitjana (els vents de l'usuari són les estrelles).
func makeAttrs(pos string, r *rand.Rand, star bool) Attrs {
	p := posProfile[pos]
	bump := 0
	if star {
		bump = 6
	}
	f := func(v int) int {
		if v == 0 {
			return roll(r, 55, 6)
		}
		return roll(r, v+bump, 5)
	}
	return Attrs{Speed: f(p.Speed), Strength: f(p.Strength), Agility: f(p.Agility), Arm: f(p.Arm), Accuracy: f(p.Accuracy),
		Hands: f(p.Hands), Block: f(p.Block), Tackle: f(p.Tackle), Coverage: f(p.Coverage), Kick: f(p.Kick), Awareness: f(p.Awareness)}
}

// posFor tria la posició que escau a un vent pel seu rol o especialitat.
func posFor(w Wind, free map[string]int) string {
	text := strings.ToLower(w.Role + " " + w.Specialty + " " + w.Name)
	pref := []struct {
		pos   string
		words []string
	}{
		{QB, []string{"organ", "lead", "líder", "coord", "estrat", "strateg", "plan", "cap", "director", "gestió", "manag"}},
		{WR, []string{"music", "músic", "art", "disseny", "design", "creat", "front", "fotos", "cine", "viat", "travel"}},
		{RB, []string{"esport", "sport", "salut", "health", "fitness", "cuina", "cook", "aventura"}},
		{TE, []string{"product", "dades", "data", "anàl", "analy", "finan", "econ"}},
		{LB, []string{"segur", "secur", "revis", "review", "qualitat", "quality", "test", "back"}},
		{CB, []string{"notíc", "news", "inform", "investig", "research", "llibre", "book", "cultur"}},
		{S, []string{"accessib", "dret", "law", "legal", "ciència", "scien", "histò", "histor"}},
		{K, []string{"poes", "poet", "escri", "writ", "idiom", "langu"}},
	}
	for _, pr := range pref {
		if free[pr.pos] == 0 {
			continue
		}
		for _, w := range pr.words {
			if strings.Contains(text, w) {
				return pr.pos
			}
		}
	}
	// Sense pista: la posició més vistosa que quedi lliure.
	for _, pos := range []string{QB, WR, RB, TE, CB, S, LB, DL, OL, K} {
		if free[pos] > 0 {
			return pos
		}
	}
	return ""
}

var teamPalette = [][2]string{{"#1E5BD8", "#F2B705"}, {"#C8102E", "#1D1D1F"}, {"#0B6E4F", "#E4E4E4"}, {"#5B2A86", "#F5A623"}, {"#E35205", "#002244"}}

// NewLeague reparteix els vents en dues franquícies (els millors, alternats
// perquè quedin equilibrades) i completa cada equip amb jugadors d'equip.
// Els entrenadors són els dos primers vents (o el que n'hi hagi).
func NewLeague(winds []Wind, lang string, seed int64) *League {
	r := rand.New(rand.NewSource(seed))
	sorted := append([]Wind(nil), winds...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	r.Shuffle(len(sorted), func(i, j int) { sorted[i], sorted[j] = sorted[j], sorted[i] })
	lg := &League{Season: 1}
	used := map[string]bool{}
	for _, w := range winds {
		used[w.Name] = true
	}
	nameN := 0
	nextName := func() string {
		for i := 0; i < len(windNames); i++ {
			n := windNames[(nameN+i)%len(windNames)]
			if !used[n] {
				used[n] = true
				nameN += i + 1
				return n
			}
		}
		nameN++
		return fmt.Sprintf("Ventada %d", nameN)
	}
	for t := 0; t < 2; t++ {
		pal := teamPalette[(int(seed)+t*2)%len(teamPalette)]
		if t == 1 && pal == teamPalette[int(seed)%len(teamPalette)] {
			pal = teamPalette[(int(seed)+1)%len(teamPalette)]
		}
		lg.Teams[t] = &Team{ID: fmt.Sprintf("t%d", t+1), Color: pal[0], Color2: pal[1]}
	}
	// Els vents de l'usuari, alternats entre els dos equips.
	free := [2]map[string]int{{}, {}}
	for t := 0; t < 2; t++ {
		for _, rs := range RosterShape {
			free[t][rs.Pos] = rs.N
		}
	}
	numbers := [2]map[int]bool{{}, {}}
	number := func(t int, pos string) int {
		ranges := map[string][2]int{QB: {1, 19}, RB: {20, 49}, WR: {10, 19}, TE: {80, 89}, OL: {60, 79}, DL: {90, 99}, LB: {50, 59}, CB: {20, 39}, S: {20, 49}, K: {1, 9}}
		rg := ranges[pos]
		for i := 0; i < 200; i++ {
			n := rg[0] + r.Intn(rg[1]-rg[0]+1)
			if !numbers[t][n] {
				numbers[t][n] = true
				return n
			}
		}
		for n := 1; n < 100; n++ {
			if !numbers[t][n] {
				numbers[t][n] = true
				return n
			}
		}
		return 0
	}
	for i, w := range sorted {
		t := i % 2
		if len(lg.Teams[t].Players) >= RosterSize() {
			t = 1 - t
		}
		if len(lg.Teams[t].Players) >= RosterSize() {
			break
		}
		if lg.Teams[t].Coach == "" {
			// El primer vent de cada equip també n'és l'entrenador.
			lg.Teams[t].Coach = w.ID
		}
		pos := posFor(w, free[t])
		if pos == "" {
			continue
		}
		free[t][pos]--
		pr := rand.New(rand.NewSource(seedOf(w.ID)))
		lg.Teams[t].Players = append(lg.Teams[t].Players, Player{ID: "w:" + w.ID, Name: w.Name, Pos: pos, Number: number(t, pos),
			Color: w.Color, Style: w.Style, WindID: w.ID, Attrs: makeAttrs(pos, pr, true)})
	}
	// Els que falten: jugadors d'equip.
	for t := 0; t < 2; t++ {
		team := lg.Teams[t]
		for _, rs := range RosterShape {
			for free[t][rs.Pos] > 0 {
				free[t][rs.Pos]--
				name := nextName()
				pid := fmt.Sprintf("p:%s:%s", team.ID, strings.ToLower(strings.ReplaceAll(name, " ", "-")))
				team.Players = append(team.Players, Player{ID: pid, Name: name, Pos: rs.Pos, Number: number(t, rs.Pos),
					Style: faceStyle(r), Attrs: makeAttrs(rs.Pos, r, false)})
			}
		}
		sort.SliceStable(team.Players, func(i, j int) bool { return posOrder(team.Players[i].Pos) < posOrder(team.Players[j].Pos) })
	}
	lg.nameTeams(winds, lang)
	return lg
}

func posOrder(p string) int {
	for i, rs := range RosterShape {
		if rs.Pos == p {
			return i
		}
	}
	return 99
}

// faceStyle és una cara de vent a l'atzar (ulls, boca, forma).
func faceStyle(r *rand.Rand) string {
	const eyes, mouths, shapes = "pdlsc", "osfwt", "orsb"
	return string([]byte{eyes[r.Intn(len(eyes))], mouths[r.Intn(len(mouths))], shapes[r.Intn(len(shapes))]})
}

// nameTeams posa nom a les franquícies a partir del seu entrenador.
func (lg *League) nameTeams(winds []Wind, lang string) {
	nick := []string{"Storm", "Gusts", "Gales", "Squalls", "Cyclones", "Breakers"}
	nickCA := []string{"Tempestes", "Ratxes", "Ventades", "Torbs", "Cicló", "Galernes"}
	usedNick := -1
	for t, team := range lg.Teams {
		coach := ""
		for _, w := range winds {
			if w.ID == team.Coach {
				coach = w.Name
			}
		}
		if coach == "" {
			coach = []string{"Gregal", "Xaloc"}[t]
		}
		i := (int(seedOf(coach)) + t) % len(nick)
		if i == usedNick {
			i = (i + 1) % len(nick) // dos equips, dos sobrenoms
		}
		usedNick = i
		if lang == "ca" {
			team.Name = nickCA[i] + " de " + coach
		} else {
			team.Name = coach + " " + nick[i]
		}
		abbr := strings.ToUpper(coach)
		if len([]rune(abbr)) > 3 {
			abbr = string([]rune(abbr)[:3])
		}
		team.Abbr = abbr
	}
	if lg.Teams[0].Abbr == lg.Teams[1].Abbr {
		lg.Teams[1].Abbr = lg.Teams[1].Abbr[:2] + "2"
	}
}

// Ensure torna una lliga vàlida per a aquests vents: la que hi ha, amb els
// vents nous afegits si hi caben, o una de nova si encara no n'hi ha.
func Ensure(lg *League, winds []Wind, lang string, seed int64) *League {
	if lg == nil || lg.Teams[0] == nil || lg.Teams[1] == nil || len(lg.Teams[0].Players) != RosterSize() || len(lg.Teams[1].Players) != RosterSize() {
		return NewLeague(winds, lang, seed)
	}
	return lg
}
