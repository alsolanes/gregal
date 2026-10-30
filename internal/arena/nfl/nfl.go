// Package nfl és el futbol americà de l'Arena: cada jugada de línia la
// criden les dues estratègies (.md) a través del model. L'atac tria la jugada
// (cursa per dins o per fora, passades curtes, mitjanes o llargues, screen,
// play-action, xuts, ajupir-se…) i la defensa la cobertura; el resultat depèn
// de com s'enfronten, i els equips són idèntics.
package nfl

import (
	"encoding/json"

	"gregal/internal/arena"
	"gregal/internal/football"
)

func init() { arena.Register(Game{}) }

// Game és el futbol americà de l'Arena.
type Game struct{}

func (Game) ID() string { return "nfl" }

func (Game) Title(lang string) string {
	if lang == "ca" {
		return "Futbol americà (NFL)"
	}
	return "American football (NFL)"
}

func (Game) Rules() string {
	return `AMERICAN FOOTBALL, NFL rules. Two identical teams; the only difference is each team's strategy. Four 15-minute quarters, overtime if tied (both teams get the ball; then next score wins).
You call EVERY scrimmage play for your side. On offense you pick the play (and on 4th down whether to punt, kick a field goal or go for it); on defense you pick the coverage. Both calls are made at the same time, without seeing the other's.
What wins: matching your call to theirs. Runs beat soft/pass coverages, passes beat stacked boxes, screens and deep shots punish blitzes, cover2/prevent kill deep balls, play-action works after you have established the run. You see the opponent's tendencies so far (overall, on 1st down, on 3rd & long, last plays): exploit them, and avoid becoming predictable yourself.
Situational football matters: 4th-down decisions by field position, the clock (hurry when behind late, milk/kneel when ahead), field goal range, two-minute drills, protecting a lead.
Answer only with the JSON asked for.`
}

func (Game) Starter() string { return starterMD }

// Match és un partit de l'Arena.
type Match struct {
	g        *football.Game
	names    [2]string
	sent     int // jugades ja enviades com a esdeveniments
	decision int
	lang     string
}

func (Game) NewMatch(seed int64, lang string, names [2]string) arena.Match {
	teams := football.MirroredTeams(seed, names)
	g := football.NewGame(teams[0], teams[1], seed, lang)
	g.External = true
	return &Match{g: g, names: names, lang: lang}
}

// advance juga fins que el motor demana crides o s'acaba.
func (m *Match) advance() {
	for !m.g.Over && !m.g.NeedsCall() {
		m.g.Step()
	}
}

func (m *Match) Next() ([2]arena.Prompt, bool) {
	m.advance()
	if m.g.Over {
		return [2]arena.Prompt{}, true
	}
	off := m.g.Poss
	var p [2]arena.Prompt
	for t := 0; t < 2; t++ {
		p[t].Situation = m.g.Situation(t)
		if t == off {
			p[t].Schema = football.OffenseSchema
		} else {
			p[t].Schema = football.DefenseSchema
		}
		dist := "goal"
		if m.g.Spot+m.g.Dist < 100 {
			dist = itoa(m.g.Dist)
		}
		p[t].Short = "Q" + itoa(m.g.Quarter) + " " + itoa(m.g.Down) + "&" + dist + " @" + itoa(m.g.Spot)
	}
	return p, false
}

func (m *Match) Apply(ans [2]json.RawMessage) []arena.Event {
	off := m.g.Poss
	var oc football.OffCall
	var dc football.DefCall
	_ = json.Unmarshal(ans[off], &oc)
	_ = json.Unmarshal(ans[1-off], &dc)
	m.decision++
	m.g.SetCalls(oc, dc)
	m.g.Step()
	m.advance()
	return m.events()
}

// events converteix les jugades noves en esdeveniments de l'Arena.
func (m *Match) events() []arena.Event {
	plays := m.g.LastPlays(m.sent)
	out := make([]arena.Event, 0, len(plays))
	for _, p := range plays {
		m.sent++
		raw, _ := json.Marshal(p)
		dec := 0
		switch p.Kind {
		case "run", "pass", "sack", "scramble", "punt", "fg", "kneel", "spike":
			dec = m.decision
		}
		out = append(out, arena.Event{N: m.sent, Kind: p.Kind, Text: p.Text, Highlight: p.Highlight, Score: p.Score, Decision: dec, Data: raw})
	}
	return out
}

func (m *Match) Score() [2]int { return m.g.Score }

func (m *Match) Winner() int {
	switch {
	case m.g.Score[0] > m.g.Score[1]:
		return 0
	case m.g.Score[1] > m.g.Score[0]:
		return 1
	}
	return -1
}

// Meta: els equips tal com els vol el visor de futbol.
func (m *Match) Meta() any {
	type team struct {
		Name    string            `json:"name"`
		Abbr    string            `json:"abbr"`
		Color   string            `json:"color"`
		Color2  string            `json:"color2"`
		Players []football.Player `json:"players"`
	}
	var t [2]team
	for i, x := range m.g.Teams {
		t[i] = team{Name: x.Name, Abbr: x.Abbr, Color: x.Color, Color2: x.Color2, Players: x.Players}
	}
	return map[string]any{"teams": t}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

const starterMD = `# Strategy: "Take what they give"

## Identity
We are a balanced team that reads the defense and takes the easy yards. We never force deep balls into two-deep coverage and we never let the opponent guess our play.

## Offense: reading the defense (most important)
Look at "Their DEFENSE so far" before every call.
- If they blitz on more than 30% of plays: call **screen** (to the RB) or **run_outside**. On 3rd & long vs a blitz-happy team: screen.
- If they play **cover2** or **prevent** a lot: run_inside, and pass_medium to the middle. Never pass_deep against them.
- If they play **man** a lot: pass_deep to wr1, run_outside.
- If they play **cover3** a lot: pass_short (target wr2/te), run_inside.
- If they stack the box with **goal_line**: play_action or pass_short to the te.
- No data yet (first drive): run_inside on 1st down, pass_short on 2nd & long, pass_medium on 3rd & 5+.

## Staying unpredictable
Check "Your offense so far". If one play type is above 40% of our calls, pick something else this down unless the situation demands it. After 5+ runs, play_action is our best shot.

## Situations
- 3rd/4th & 1: qb_sneak, unless they are in goal_line (then play_action).
- 4th down: inside their 35 with 1-2 yards to go → go for it. In field goal range (kick distance ≤ our range) → field_goal. Otherwise punt. Down by 9+ in the 4th quarter → go for it from anywhere past midfield.
- Behind in the last 4 minutes: tempo hurry, prefer passes to the sideline (pass_short/pass_medium), spike only to save the last seconds.
- Ahead in the last 4 minutes: run_inside with tempo milk; kneel when the clock can run out.
- Red zone (inside their 20): no pass_deep. Mix run_inside and play_action; vs goal_line → pass_short to the te.

## Defense: reading their offense
Look at "Their OFFENSE so far".
- They run more than 55% (or on 1st down): base or goal_line on 1st down, cover3 otherwise.
- They throw deep a lot: cover2; late in halves: prevent.
- They love short passes: man.
- 3rd & 7+: cover2 or prevent (make them throw short of the sticks); 3rd & short: goal_line or blitz.
- Blitz at most on 1 play out of 4, and never twice in a row if they have run screens.
- Last 2 minutes of a half while ahead: prevent.

## Example
3rd & 8 at their 42, they blitzed 4 of the last 6 plays → {"play":"screen","target":"rb","tempo":"normal"}.
`
