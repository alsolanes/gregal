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
What wins: matching your call to theirs. Every call beats some calls and loses to others (rock-paper-scissors), so no single call is safe: a team that always calls the same thing gets countered.
` + football.MatchupText() + `play_action also needs an established run: it gains more once 40%+ of your plays have been runs, and less if you never run.
Each prompt shows the opponent's tendencies (overall, on 1st down, on 3rd & 5+, last plays) and the RESULTS so far (yards per play and success rate by your call and by their call): counter what they call most, keep calling what works, drop what fails, and avoid becoming predictable yourself.
Situational football matters: 4th-down decisions by field position (if you give no valid answer on 4th down, you kick), the clock (hurry when behind late, milk/kneel when ahead), field goal range, two-minute drills, protecting a lead.
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
	// hints: el format de decisió porta, per a cada crida, contra què guanya
	// i perd (la taula de les regles ja mastegada). Per defecte sí; les
	// competicions oficials ho treuen (SetHints(false)).
	hints bool
}

// SetHints encén o apaga les ajudes del format de decisió.
func (m *Match) SetHints(on bool) { m.hints = on }

func (Game) NewMatch(seed int64, lang string, names [2]string) arena.Match {
	teams := football.MirroredTeams(seed, names)
	g := football.NewGame(teams[0], teams[1], seed, lang)
	g.External = true
	return &Match{g: g, names: names, lang: lang, hints: true}
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
		switch {
		case t == off && m.hints:
			p[t].Schema = football.OffenseSchema
		case t == off:
			p[t].Schema = football.OffenseSchemaBare
		case m.hints:
			p[t].Schema = football.DefenseSchema
		default:
			p[t].Schema = football.DefenseSchemaBare
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

const starterMD = `# Strategy: "Read, counter, adapt"

## Identity
We win the chess match, not the highlight reel. We find what the opponent calls most, answer it with the call the MATCHUP TABLE says beats it, keep doing what gains yards and drop what fails. We never let them key on one call of ours.

## Priorities (go down the list; the first rule that applies decides)
1. The situation: 4th down, the clock, short yardage.
2. The RESULTS line: what has worked and what has failed in this game.
3. Their tendencies: counter their most frequent call.
4. Our own tendencies: no single call above 40% of our plays.

## Offense
### 4th down (decide this first)
- "A field goal from here would be X yards" and X is at most our kicker's range → **field_goal**.
- 4th & 1 or 2 at "the defense's" 45 or closer → go for it: **qb_sneak** on 4th & 1 (play_action if their top defense is goal_line), otherwise our counter call below.
- Q4, under 5:00, behind by 4 or more → go for it with our counter call.
- Anything else → **punt**.

### Clock
- Q4, under 3:00, ahead: run (run_inside, or run_outside if their top defense is man, blitz or cover3), tempo **milk**.
- Q4 under 5:00 behind, or Q2 under 2:00: tempo **hurry** and pass (pick the pass option from the table below).

### Counter their defense
Look at "Their DEFENSE so far: overall" and take their most-called defense. On 3rd & 5+, use their "on 3rd & 5+" numbers instead.
| Their top defense | Option A | Option B (every 3rd play, or when A is failing) |
| base | pass_medium to wr1 | play_action (only once runs are 40%+ of our plays) |
| blitz | screen to rb | pass_short to wr2 |
| cover2 | run_inside | pass_medium to wr1 |
| cover3 | pass_short to wr2 | screen to rb |
| man | run_outside | pass_deep to wr1 |
| goal_line | play_action | pass_medium to te |
| prevent | run_inside | run_outside |
- On 3rd & 7+, prefer a pass from that row.
- No data yet: 1st down run_inside, 2nd down pass_short, 3rd down pass_medium.

### Use the RESULTS
- In "RESULTS of your offense ... by your call": if the call we are about to make has 3+ plays and under 4.0 yds/play, switch to the other option.
- A call at 7+ yds/play and 60%+ success: keep calling it (still respecting the 40% cap).

## Defense
### Counter their offense
Look at "Their OFFENSE so far: overall" and take their most-called play (on 3rd & 5+, their "on 3rd & 5+" numbers).
| Their top play | Our call |
| run_inside | blitz |
| run_outside | cover3 |
| pass_short | man |
| pass_medium | blitz |
| pass_deep | cover2 |
| screen | man |
| play_action | blitz |
- If their second play is 25%+ and the MATCHUP TABLE gives it + or ++ against that call, pick instead the defense that is best against both together (e.g. run_inside + screen → base; pass_short + pass_deep → cover2).
- In "RESULTS of their offense ... by your defense": a defense that allows 6+ yds/play over 4+ plays is broken; use a different one.

### Situations
- 3rd or 4th & 1-2, or goal to go from our 3: **goal_line**.
- No data yet: base on 1st down, cover2 on 2nd, man on 3rd.
- Q2 or Q4 under 1:00 and we are ahead: **prevent**.
- Never blitz on more than 3 plays in a row: a screen kills it.

## Examples
- 3rd & 8 at the defense's 42; Their DEFENSE so far: overall: blitz 45%, base 30%, ... → {"play":"screen","target":"rb","tempo":"normal"}
- Their OFFENSE so far: overall: run_outside 50%, pass_short 20%, ...; 1st & 10 → {"call":"cover3"}

## Writing tips (for whoever edits this file)
Quote the exact words of the Situation ("Their DEFENSE so far", "RESULTS", "on 3rd & 5+") in your rules, give each rule one clear action, and keep the order of priorities explicit: the model follows short if/then rules best.
`
