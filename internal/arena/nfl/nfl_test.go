package nfl

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"gregal/internal/arena"
	"gregal/internal/football"
)

// ---- lectura de la Situation (només el que el model també veu) ----

var (
	reFG      = regexp.MustCompile(`would be (\d+) yards \(your kicker's range is ~(\d+)\)`)
	reShort   = regexp.MustCompile(`^Q(\d+) (\d+)&(\w+) @(\d+)$`)
	reClock   = regexp.MustCompile(`(\d+):(\d\d) left`)
	reScore   = regexp.MustCompile(`Score: .+? (\d+) - .+? (\d+)\.`)
	rePct     = regexp.MustCompile(`([a-z_0-9]+) (\d+)%`)
	reTotal   = regexp.MustCompile(`\((\d+) plays\)`)
	reResult  = regexp.MustCompile(`([a-z_0-9]+) (\d+) plays, (-?[\d.]+) yds/play, (\d+)% success`)
	reLastDef = regexp.MustCompile(`last plays: (.*)`)
)

type sit struct {
	offense             bool
	q, down, dist, spot int
	clock, lead         int
	s                   string
}

func read(p arena.Prompt) sit {
	x := sit{offense: strings.Contains(p.Situation, "You are the OFFENSE"), s: p.Situation}
	if m := reShort.FindStringSubmatch(p.Short); m != nil {
		x.q, _ = strconv.Atoi(m[1])
		x.down, _ = strconv.Atoi(m[2])
		x.spot, _ = strconv.Atoi(m[4])
		if m[3] == "goal" {
			x.dist = 100 - x.spot
		} else {
			x.dist, _ = strconv.Atoi(m[3])
		}
	}
	if m := reClock.FindStringSubmatch(p.Situation); m != nil {
		a, _ := strconv.Atoi(m[1])
		b, _ := strconv.Atoi(m[2])
		x.clock = a*60 + b
	}
	if m := reScore.FindStringSubmatch(p.Situation); m != nil {
		a, _ := strconv.Atoi(m[1])
		b, _ := strconv.Atoi(m[2])
		x.lead = a - b
	}
	return x
}

// line torna la línia de la Situation que comença per prefix.
func (x sit) line(prefix string) string {
	for _, l := range strings.Split(x.s, "\n") {
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}
	return ""
}

type freq struct {
	call string
	pct  int
}

// freqs llegeix «label: a 40%, b 20% (N plays)» d'una línia de tendències.
func freqs(line, label string) (out []freq, total int) {
	i := strings.Index(line, label)
	if i < 0 {
		return nil, 0
	}
	seg := line[i+len(label):]
	if j := strings.Index(seg, ";"); j >= 0 {
		seg = seg[:j]
	}
	for _, m := range rePct.FindAllStringSubmatch(seg, -1) {
		n, _ := strconv.Atoi(m[2])
		out = append(out, freq{m[1], n})
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].pct > out[b].pct })
	if m := reTotal.FindStringSubmatch(seg); m != nil {
		total, _ = strconv.Atoi(m[1])
	}
	return out, total
}

type res struct {
	n   int
	ypp float64
}

// results llegeix «by your call: x 3 plays, 4.0 yds/play, 50% success; …».
func results(line, label string) map[string]res {
	out := map[string]res{}
	i := strings.Index(line, label)
	if i < 0 {
		return out
	}
	seg := line[i+len(label):]
	if j := strings.Index(seg, ". "); j >= 0 {
		seg = seg[:j]
	}
	for _, m := range reResult.FindAllStringSubmatch(seg, -1) {
		n, _ := strconv.Atoi(m[2])
		y, _ := strconv.ParseFloat(m[3], 64)
		out[m[1]] = res{n, y}
	}
	return out
}

func off(play, target, tempo string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"play":%q,"target":%q,"tempo":%q}`, play, target, tempo))
}

func def(call string) json.RawMessage { return json.RawMessage(fmt.Sprintf(`{"call":%q}`, call)) }

func inFGRange(x sit) bool {
	if m := reFG.FindStringSubmatch(x.s); m != nil {
		d, _ := strconv.Atoi(m[1])
		r, _ := strconv.Atoi(m[2])
		return d <= r
	}
	return false
}

func kick4th(x sit) json.RawMessage {
	if inFGRange(x) {
		return off("field_goal", "", "normal")
	}
	return off("punt", "", "normal")
}

func isRunCall(c string) bool { return c == "run_inside" || c == "run_outside" || c == "qb_sneak" }

// ---- la plantilla, escrita com a codi (segueix starterMD regla per regla) ----

var optA = map[string]string{"base": "pass_medium", "blitz": "screen", "cover2": "run_inside", "cover3": "pass_short", "man": "run_outside", "goal_line": "play_action", "prevent": "run_inside"}
var optB = map[string]string{"base": "play_action", "blitz": "pass_short", "cover2": "pass_medium", "cover3": "screen", "man": "pass_deep", "goal_line": "pass_medium", "prevent": "run_outside"}
var targets = map[string]string{"pass_medium": "wr1", "pass_deep": "wr1", "screen": "rb", "pass_short": "wr2", "play_action": "wr1"}
var defCounter = map[string]string{"run_inside": "blitz", "run_outside": "cover3", "qb_sneak": "goal_line", "pass_short": "man", "pass_medium": "blitz", "pass_deep": "cover2", "screen": "man", "play_action": "blitz"}

func starter(_ int, p arena.Prompt) json.RawMessage {
	x := read(p)
	if x.offense {
		return starterOffense(x)
	}
	return starterDefense(x)
}

func starterOffense(x sit) json.RawMessage {
	dl := x.line("Their DEFENSE so far:")
	theirs, _ := freqs(dl, "overall:")
	if x.down == 3 && x.dist >= 5 {
		if t, _ := freqs(dl, "on 3rd & 5+:"); len(t) > 0 {
			theirs = t
		}
	}
	top := ""
	if len(theirs) > 0 {
		top = theirs[0].call
	}
	mine, myTotal := freqs(x.line("Your offense so far:"), "overall:")
	myPct := map[string]int{}
	runShare := 0
	for _, f := range mine {
		myPct[f.call] = f.pct
		if isRunCall(f.call) {
			runShare += f.pct
		}
	}
	counter := func() string {
		if top == "" {
			switch x.down {
			case 1:
				return "run_inside"
			case 2:
				return "pass_short"
			}
			return "pass_medium"
		}
		a, b := optA[top], optB[top]
		if b == "play_action" && runShare < 40 {
			b = a
		}
		choice := a
		if myTotal%3 == 2 {
			choice = b
		}
		other := a
		if choice == a {
			other = b
		}
		if x.down == 3 && x.dist >= 7 && isRunCall(choice) && !isRunCall(other) {
			choice, other = other, choice
		}
		if r := results(x.line("RESULTS of your offense"), "by your call:")[choice]; r.n >= 3 && r.ypp < 4 {
			choice, other = other, choice
		}
		if myTotal >= 10 && myPct[choice] > 40 {
			choice = other
		}
		return choice
	}
	// 4a.
	if x.down == 4 {
		if inFGRange(x) {
			return off("field_goal", "", "normal")
		}
		desperate := x.q >= 4 && x.clock < 300 && x.lead <= -4
		if !(x.dist <= 2 && x.spot >= 55) && !desperate {
			return off("punt", "", "normal")
		}
		if x.dist <= 1 && !desperate {
			if top == "goal_line" {
				return off("play_action", "te", "normal")
			}
			return off("qb_sneak", "", "normal")
		}
		c := counter()
		return off(c, targets[c], "normal")
	}
	// Rellotge.
	if x.q >= 4 && x.clock < 180 && x.lead > 0 {
		run := "run_inside"
		if top == "man" || top == "blitz" || top == "cover3" {
			run = "run_outside"
		}
		return off(run, "", "milk")
	}
	tempo := "normal"
	if (x.q >= 4 && x.clock < 300 && x.lead < 0) || (x.q == 2 && x.clock < 120) {
		tempo = "hurry"
		c := counter()
		if isRunCall(c) {
			c = optB[top]
			if top == "" || isRunCall(c) {
				c = "pass_short"
			}
		}
		return off(c, targets[c], tempo)
	}
	c := counter()
	return off(c, targets[c], tempo)
}

// bestDef és la defensa amb la suma de notes més baixa contra plays.
func bestDef(plays []string, exclude string) string {
	best, bs, bt := "", 99, 99
	for _, d := range football.DefenseCalls {
		if d == exclude {
			continue
		}
		s := 0
		for _, p := range plays {
			s += football.Matchup[p][d]
		}
		t := football.Matchup[plays[0]][d]
		if s < bs || s == bs && t < bt {
			best, bs, bt = d, s, t
		}
	}
	return best
}

func starterDefense(x sit) json.RawMessage {
	if (x.down >= 3 && x.dist <= 2) || x.spot >= 97 {
		return def("goal_line")
	}
	if (x.q == 2 || x.q == 4) && x.clock < 60 && x.lead > 0 {
		return def("prevent")
	}
	ol := x.line("Their OFFENSE so far:")
	theirs, _ := freqs(ol, "overall:")
	if x.down == 3 && x.dist >= 5 {
		if t, _ := freqs(ol, "on 3rd & 5+:"); len(t) > 0 {
			theirs = t
		}
	}
	if len(theirs) == 0 {
		return def([]string{"base", "base", "cover2", "man", "man"}[x.down])
	}
	top := theirs[0].call
	call := defCounter[top]
	if len(theirs) > 1 && theirs[1].pct >= 25 && football.Matchup[theirs[1].call][call] > 0 {
		call = bestDef([]string{top, theirs[1].call}, "")
	}
	if r := results(x.line("RESULTS of their offense"), "by your defense:")[call]; r.n >= 4 && r.ypp >= 6 {
		call = bestDef([]string{top}, call)
	}
	if call == "blitz" {
		if m := reLastDef.FindStringSubmatch(x.line("Your defense so far:")); m != nil {
			last := strings.Split(m[1], " | ")
			if len(last) >= 3 && strings.HasSuffix(last[len(last)-1], "blitz") && strings.HasSuffix(last[len(last)-2], "blitz") && strings.HasSuffix(last[len(last)-3], "blitz") {
				call = bestDef([]string{top}, "blitz")
			}
		}
	}
	return def(call)
}

// exploit: només compta el que el rival crida més i hi respon (sense cap
// altra lògica que xutar a la 4a).
func exploit(_ int, p arena.Prompt) json.RawMessage {
	x := read(p)
	if x.offense {
		if x.down == 4 {
			return kick4th(x)
		}
		theirs, _ := freqs(x.line("Their DEFENSE so far:"), "overall:")
		if len(theirs) == 0 {
			return nil
		}
		c := optA[theirs[0].call]
		return off(c, targets[c], "normal")
	}
	theirs, _ := freqs(x.line("Their OFFENSE so far:"), "overall:")
	if len(theirs) == 0 {
		return nil
	}
	return def(defCounter[theirs[0].call])
}

// constant: sempre la mateixa jugada i la mateixa defensa ("" = no respondre
// res, el valor per defecte del motor); a la 4a xuta, si kick.
func constant(play, call string, kick bool) arena.Decide {
	return func(_ int, p arena.Prompt) json.RawMessage {
		x := read(p)
		if x.offense {
			if kick && x.down == 4 {
				return kick4th(x)
			}
			if play == "" {
				return nil
			}
			return off(play, targets[play], "normal")
		}
		if call == "" {
			return nil
		}
		return def(call)
	}
}

// winRate juga n partits (alternant bandes, llavors fixes) i torna la part
// que guanya a (els empats compten mig).
func winRate(a, b arena.Decide, n int, seed int64) float64 {
	var mu sync.WaitGroup
	score := make([]float64, n)
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	for i := 0; i < n; i++ {
		mu.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer func() { <-sem; mu.Done() }()
			var d [2]arena.Decide
			s := i % 2
			d[s], d[1-s] = a, b
			m := Game{}.NewMatch(seed+int64(i), "en", [2]string{"A", "B"})
			arena.Play(m, d)
			switch m.Winner() {
			case s:
				score[i] = 1
			case -1:
				score[i] = .5
			}
		}(i)
	}
	mu.Wait()
	w := 0.0
	for _, v := range score {
		w += v
	}
	return w / float64(n)
}

var scrimCalls = []string{"run_inside", "run_outside", "pass_short", "pass_medium", "pass_deep", "screen", "play_action"}

type opponent struct {
	name string
	d    arena.Decide
}

// constants són les polítiques constants que cap estratègia que llegeixi el
// rival hauria de perdre: no respondre res, i cada jugada (amb la defensa per
// defecte) i cada defensa (amb l'atac per defecte).
func constants() []opponent {
	out := []opponent{{"no answer", constant("", "", false)}}
	for _, o := range scrimCalls {
		out = append(out, opponent{"always " + o, constant(o, "", true)})
	}
	for _, d := range football.DefenseCalls {
		out = append(out, opponent{"always " + d, constant("", d, true)})
	}
	return out
}

// La plantilla guanya totes les polítiques constants, i de llarg la de no
// respondre res.
func TestStarterBeatsConstantPolicies(t *testing.T) {
	const n = 100
	for _, c := range constants() {
		r := winRate(starter, c.d, n, 1)
		t.Logf("starter vs %-18s %.1f%%", c.name, r*100)
		min := .5
		if c.name == "no answer" {
			min = .6
		}
		if r < min {
			t.Errorf("starter vs %s: %.1f%%, ha de ser ≥ %.0f%%", c.name, r*100, min*100)
		}
	}
}

// Qui només compta la crida més freqüent del rival i hi respon guanya
// qualsevol política constant.
func TestExploiterBeatsConstantPolicies(t *testing.T) {
	const n = 100
	for _, c := range constants() {
		r := winRate(exploit, c.d, n, 5000)
		t.Logf("exploit vs %-18s %.1f%%", c.name, r*100)
		if r < .7 {
			t.Errorf("exploit vs %s: %.1f%%, ha de ser ≥ 70%%", c.name, r*100)
		}
	}
}

// Totes les parelles jugada × defensa constants (lent: NFL_FULL=1).
func TestAllConstantPairs(t *testing.T) {
	if os.Getenv("NFL_FULL") == "" {
		t.Skip("NFL_FULL=1 per jugar totes les parelles")
	}
	for _, o := range scrimCalls {
		for _, d := range football.DefenseCalls {
			c := constant(o, d, true)
			rs, re := winRate(starter, c, 200, 9000), winRate(exploit, c, 200, 9000)
			t.Logf("%-12s + %-9s starter %.1f%%  exploit %.1f%%", o, d, rs*100, re*100)
			if rs < .5 || re < .7 {
				t.Errorf("%s + %s: starter %.1f%%, exploit %.1f%%", o, d, rs*100, re*100)
			}
		}
	}
}

func TestMirrorFairness(t *testing.T) {
	w := [2]int{}
	for i := 0; i < 200; i++ {
		m := Game{}.NewMatch(int64(1000+i), "en", [2]string{"A", "B"})
		arena.Play(m, [2]arena.Decide{starter, starter})
		if x := m.Winner(); x >= 0 {
			w[x]++
		}
	}
	t.Logf("starter vs starter: %v", w)
	if w[0] < 80 || w[1] < 80 {
		t.Fatalf("dues estratègies iguals han de quedar ~50%%: %v", w)
	}
}

func TestInvalidAnswersFallBack(t *testing.T) {
	m := Game{}.NewMatch(3, "ca", [2]string{"A", "B"})
	junk := func(int, arena.Prompt) json.RawMessage { return json.RawMessage(`{"play":"teleport","call":"???"}`) }
	ev := arena.Play(m, [2]arena.Decide{junk, func(int, arena.Prompt) json.RawMessage { return nil }})
	if len(ev) < 60 || m.Winner() < -1 {
		t.Fatalf("partit amb respostes invàlides: %d", len(ev))
	}
	for _, e := range ev {
		var p football.Play
		_ = json.Unmarshal(e.Data, &p)
		if p.Down == 4 && (p.Kind == "run" || p.Kind == "pass" || p.Kind == "sack") {
			t.Fatalf("sense resposta a la 4a s'ha de xutar: %s", e.Text)
		}
	}
	if !strings.Contains(Game{}.Starter(), "## Offense") || len(Game{}.Rules()) > 2600 {
		t.Fatalf("plantilla o regles (%d caràcters)", len(Game{}.Rules()))
	}
}

// Les regles porten la taula tal com és.
func TestRulesShowTheTable(t *testing.T) {
	r := Game{}.Rules()
	for p := range football.Matchup {
		if !strings.Contains(r, "- "+p+": "+football.MatchupLine(p)) {
			t.Errorf("les regles no diuen la fila de %s", p)
		}
	}
}

// La plantilla respon cada defensa amb una crida que la taula dona com a
// avantatge, i cada jugada amb una defensa que la taula dona com a aturada.
func TestStarterTablesFollowTheMatchupTable(t *testing.T) {
	for d, o := range optA {
		if football.Matchup[o][d] < 1 {
			t.Errorf("opció A contra %s (%s) no guanya a la taula", d, o)
		}
		if b := optB[d]; football.Matchup[b][d] < 1 {
			t.Errorf("opció B contra %s (%s) no guanya a la taula", d, b)
		}
		row := "| " + d + " | " + o
		if !strings.Contains(starterMD, row) {
			t.Errorf("la plantilla no diu %q", row)
		}
	}
	for o, d := range defCounter {
		if football.Matchup[o][d] > -1 {
			t.Errorf("la defensa contra %s (%s) no l'atura", o, d)
		}
		if o != "qb_sneak" && !strings.Contains(starterMD, "| "+o+" | "+d+" |") {
			t.Errorf("la plantilla no diu %s → %s", o, d)
		}
	}
}
