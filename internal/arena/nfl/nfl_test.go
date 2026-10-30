package nfl

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gregal/internal/arena"
)

func pctOf(text, label, call string) int {
	i := strings.Index(text, label)
	if i < 0 {
		return 0
	}
	seg := text[i:]
	if j := strings.Index(seg, ";"); j >= 0 {
		seg = seg[:j]
	}
	m := regexp.MustCompile(call + ` (\d+)%`).FindStringSubmatch(seg)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func isOffense(p arena.Prompt) bool { return strings.Contains(p.Situation, "You are the OFFENSE") }

// naive: sempre la mateixa jugada i la mateixa defensa, i xuta a la 4a.
func naive(_ int, p arena.Prompt) json.RawMessage {
	if isOffense(p) {
		if strings.Contains(p.Short, " 4&") {
			return json.RawMessage(`{"play":"punt"}`)
		}
		return json.RawMessage(`{"play":"pass_deep","target":"wr1"}`)
	}
	return json.RawMessage(`{"call":"blitz"}`)
}

// adaptive llegeix les tendències del rival, com diu el .md d'exemple.
func adaptive(_ int, p arena.Prompt) json.RawMessage {
	s := p.Situation
	if isOffense(p) {
		fourth := strings.Contains(p.Short, " 4&")
		if fourth {
			if strings.Contains(s, "field goal from here would be") {
				re := regexp.MustCompile(`would be (\d+) yards \(your kicker's range is ~(\d+)\)`)
				if m := re.FindStringSubmatch(s); m != nil {
					d, _ := strconv.Atoi(m[1])
					r, _ := strconv.Atoi(m[2])
					if d <= r {
						return json.RawMessage(`{"play":"field_goal"}`)
					}
				}
			}
			return json.RawMessage(`{"play":"punt"}`)
		}
		blitz := pctOf(s, "Their DEFENSE so far: overall", "blitz")
		cover2 := pctOf(s, "Their DEFENSE so far: overall", "cover2") + pctOf(s, "Their DEFENSE so far: overall", "prevent")
		man := pctOf(s, "Their DEFENSE so far: overall", "man")
		switch {
		case blitz > 30:
			return json.RawMessage(`{"play":"screen","target":"rb"}`)
		case cover2 > 35:
			return json.RawMessage(`{"play":"run_inside"}`)
		case man > 35:
			return json.RawMessage(`{"play":"pass_deep","target":"wr1"}`)
		}
		return json.RawMessage(`{"play":"pass_short","target":"wr2"}`)
	}
	deep := pctOf(s, "Their OFFENSE so far: overall", "pass_deep")
	screen := pctOf(s, "Their OFFENSE so far: overall", "screen")
	runs := pctOf(s, "Their OFFENSE so far: overall", "run_inside") + pctOf(s, "Their OFFENSE so far: overall", "run_outside")
	switch {
	case deep > 30:
		return json.RawMessage(`{"call":"cover2"}`)
	case screen > 30:
		return json.RawMessage(`{"call":"man"}`)
	case runs > 50:
		return json.RawMessage(`{"call":"goal_line"}`)
	}
	return json.RawMessage(`{"call":"cover3"}`)
}

func TestAdaptiveStrategyBeatsNaive(t *testing.T) {
	g := Game{}
	wins, naiveWins, games := 0, 0, 200
	for i := 0; i < games; i++ {
		var d [2]arena.Decide
		ad := i % 2
		d[ad], d[1-ad] = adaptive, naive
		m := g.NewMatch(int64(i+1), "en", [2]string{"A", "B"})
		ev := arena.Play(m, d)
		if len(ev) < 60 {
			t.Fatalf("partit massa curt: %d esdeveniments", len(ev))
		}
		switch m.Winner() {
		case ad:
			wins++
		case 1 - ad:
			naiveWins++
		}
	}
	t.Logf("adaptativa %d, ingènua %d, de %d", wins, naiveWins, games)
	if wins < games*65/100 {
		t.Fatalf("l'estratègia adaptativa havia de guanyar ≥65%%: %d/%d", wins, games)
	}
	if naiveWins == 0 {
		t.Fatalf("la ingènua ha de guanyar algun partit")
	}
}

func TestMirrorFairness(t *testing.T) {
	g := Game{}
	w := [2]int{}
	for i := 0; i < 200; i++ {
		m := g.NewMatch(int64(1000+i), "en", [2]string{"A", "B"})
		arena.Play(m, [2]arena.Decide{adaptive, adaptive})
		if x := m.Winner(); x >= 0 {
			w[x]++
		}
	}
	if w[0] < 70 || w[1] < 70 {
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
	if !strings.Contains(Game{}.Starter(), "## Offense") || len(Game{}.Rules()) > 2500 {
		t.Fatal("plantilla o regles")
	}
}
