package server

// Does a better prompt really win? The bots are the measuring stick: Bot Rookie
// plays a short, vague .md and Bot Analyst an expert one (arenaBotStyles), so
// every match already played against them is a data point. Nothing here calls
// the model: it is computed from the finished matches.
//
//   - Quality.VsVague: how often a structured prompt (any side that is not Bot
//     Rookie) beat the vague one. The headline for the event.
//   - Quality.Rungs: each bot as a step of the ladder, with its own record.
//   - arenaPlayer.VsBots: each person's record against each bot level.
//   - arenaPlayerGame.Versions: each person's .md versions in a game (by text),
//     with the record and coach score of each, for the before/after.

import (
	"crypto/sha256"
	"encoding/json"
	"math"
	"sort"
	"strings"
)

type arenaWL struct {
	Played int `json:"played"`
	Won    int `json:"won"`
}

func (w *arenaWL) add(won bool) {
	w.Played++
	if won {
		w.Won++
	}
}

type arenaRung struct {
	Owner   string  `json:"owner"`
	Name    string  `json:"name"`
	Level   string  `json:"level"` // rookie, solid, expert
	Overall arenaWL `json:"overall"`
	VsVague arenaWL `json:"vs_vague"` // against Bot Rookie (empty for Bot Rookie itself)
}

type arenaQuality struct {
	VsVague arenaWL     `json:"vs_vague"` // a structured prompt against the vague one
	People  arenaWL     `json:"people"`   // people against Bot Rookie
	Rungs   []arenaRung `json:"rungs"`
}

type arenaMDVersion struct {
	N      int     `json:"n"`     // 1 = the first .md that played
	At     int64   `json:"at"`    // first match with it
	Lines  int     `json:"lines"` // non-empty lines
	Played int     `json:"played"`
	Won    int     `json:"won"`
	Coach  float64 `json:"coach,omitempty"` // average coach score, 0-100
}

const (
	arenaVagueBot   = "bot:rookie"
	arenaMaxVersion = 12 // the latest versions kept per game
)

func arenaBotLevel(owner string) string {
	for _, b := range arenaBotStyles {
		if b.ID == owner {
			return b.Level
		}
	}
	return ""
}

// arenaQualityOf fills the per-player fields and returns the ladder. finals is
// in chronological order; coach is the coach total (5-25) per match and side.
func arenaQualityOf(finals []*arenaMatch, coach map[string][2]int, sigs map[string]arenaMDSigs, players map[string]*arenaPlayer) *arenaQuality {
	q := &arenaQuality{Rungs: []arenaRung{}}
	rung := map[string]*arenaRung{}
	for _, b := range arenaBotStyles {
		q.Rungs = append(q.Rungs, arenaRung{Owner: b.ID, Name: b.Name, Level: b.Level})
	}
	for i := range q.Rungs {
		rung[q.Rungs[i].Owner] = &q.Rungs[i]
	}
	type vkey struct{ owner, game string }
	type vacc struct {
		v      *arenaMDVersion
		coachT float64
		coachN int
	}
	versions := map[vkey][]*vacc{}
	byText := map[vkey]map[[32]byte]*vacc{}

	for _, m := range finals {
		for side := 0; side < 2; side++ {
			o, opp := m.Owners[side], m.Owners[1-side]
			won := m.Winner == side
			if r := rung[o]; r != nil {
				r.Overall.add(won)
				if opp == arenaVagueBot && o != arenaVagueBot {
					r.VsVague.add(won)
				}
			}
			// The headline: any non-vague side against Bot Rookie (demo matches aside).
			if opp == arenaVagueBot && o != arenaVagueBot && !strings.HasPrefix(o, "demo:") && o != "" {
				q.VsVague.add(won)
				if !isBotOwner(o) {
					q.People.add(won)
				}
			}
			p := players[o]
			if p == nil || isBotOwner(o) {
				continue
			}
			if lvl := arenaBotLevel(opp); lvl != "" {
				if p.VsBots == nil {
					p.VsBots = map[string]*arenaWL{}
				}
				if p.VsBots[lvl] == nil {
					p.VsBots[lvl] = &arenaWL{}
				}
				p.VsBots[lvl].add(won)
			}
			sig := sigs[m.ID][side]
			if sig.Lines == 0 {
				continue
			}
			k := vkey{o, m.Game}
			if byText[k] == nil {
				byText[k] = map[[32]byte]*vacc{}
			}
			a := byText[k][sig.Hash]
			if a == nil {
				a = &vacc{v: &arenaMDVersion{N: len(versions[k]) + 1, At: max(m.Started, m.Finished), Lines: sig.Lines}}
				byText[k][sig.Hash] = a
				versions[k] = append(versions[k], a)
			}
			a.v.Played++
			if won {
				a.v.Won++
			}
			if c := coach[m.ID][side]; c > 0 {
				a.coachT += float64(c) * 4 // 5-25 → 20-100
				a.coachN++
			}
		}
	}
	for k, list := range versions {
		p := players[k.owner]
		if p == nil {
			continue
		}
		g := p.Games[k.game]
		if g == nil {
			g = &arenaPlayerGame{Bots: []string{}}
			p.Games[k.game] = g
		}
		if len(list) > arenaMaxVersion {
			list = list[len(list)-arenaMaxVersion:]
		}
		g.Versions = make([]arenaMDVersion, 0, len(list))
		for _, a := range list {
			if a.coachN > 0 {
				a.v.Coach = math.Round(a.coachT / float64(a.coachN))
			}
			g.Versions = append(g.Versions, *a.v)
		}
	}
	return q
}

// arenaQualityNow: only the ladder, from the match summaries (no coach data,
// no players), cheap enough for the broadcast's polling.
func (s *Server) arenaQualityNow() *arenaQuality {
	sums := s.arenaSummaries()
	var finals []*arenaMatch
	for i := range sums {
		if sums[i].M.Status == "final" {
			finals = append(finals, &sums[i].M)
		}
	}
	at := func(m *arenaMatch) int64 { return max(m.Finished, m.Started) }
	sort.SliceStable(finals, func(i, j int) bool { return at(finals[i]) < at(finals[j]) })
	return arenaQualityOf(finals, nil, nil, nil)
}

// arenaMDSig: what the season needs of a .md (which version it is and how
// long), without keeping the text in the match summaries.
type arenaMDSig struct {
	Hash  [32]byte
	Lines int // 0 = no .md
}

type arenaMDSigs [2]arenaMDSig

func mdSig(md string) arenaMDSig {
	md = strings.TrimSpace(md)
	if md == "" {
		return arenaMDSig{}
	}
	lines := 0
	for _, l := range strings.Split(md, "\n") {
		if strings.TrimSpace(l) != "" {
			lines++
		}
	}
	return arenaMDSig{Hash: sha256.Sum256([]byte(md)), Lines: lines}
}

// UnmarshalJSON reads the "md" pair of a match file into signatures.
func (s *arenaMDSigs) UnmarshalJSON(b []byte) error {
	var md [2]string
	if json.Unmarshal(b, &md) != nil {
		return nil
	}
	for i := range md {
		s[i] = mdSig(md[i])
	}
	return nil
}
