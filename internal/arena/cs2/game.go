// Package cs2 és el Counter-Strike 2 de l'Arena: dos equips idèntics a
// Mirage, MR12 amb pròrrogues MR3, economia real de CS2 i una decisió per
// ronda i banda (compra + estratègia). El motor simula la ronda com una
// seqüència de moviments i duels per zones.
package cs2

import (
	"math/rand"

	"gregal/internal/arena"
)

func init() { arena.Register(Game{}) }

// Game és el joc "cs2".
type Game struct{}

func (Game) ID() string               { return "cs2" }
func (Game) Title(lang string) string { return "Counter-Strike 2" }
func (Game) Rules() string            { return rulesText }
func (Game) Starter() string          { return starterText }

// NewMatch prepara un partit amb la llavor.
func (Game) NewMatch(seed int64, lang string, names [2]string) arena.Match {
	return newMatch(seed, lang, names)
}

var roles = [5]string{"IGL", "AWPer", "Entry", "Support", "Lurker"}

var handlePool = []string{
	"vortex", "kryo", "nexa", "dazed", "frostbyte", "hazz", "pyke", "zenith", "ruin", "miro",
	"sparx", "ghostly", "flick", "kavu", "rift", "oxide", "juno", "brisk", "vyper", "tecno",
	"lumo", "snaz", "drift", "halo", "kez", "nyx", "bolt", "crane", "taz", "omen",
}

var teamColors = [2]string{"#2ec4b6", "#ff6b6b"}

func newMatch(seed int64, lang string, names [2]string) *Match {
	r := rand.New(rand.NewSource(seed))
	m := &Match{seed: seed, lang: lang, names: names, rng: r, H: 12}
	for i := range names {
		if m.names[i] == "" {
			m.names[i] = []string{"Team A", "Team B"}[i]
		}
	}
	// Atributs per posició, iguals als dos equips (mirall).
	type attrs struct{ aim, util, move, clutch float64 }
	var base [5]attrs
	for i, role := range roles {
		a := attrs{1, 1, 1, 1}
		switch role {
		case "IGL":
			a.aim, a.util = 0.94, 1.12
		case "AWPer":
			a.aim, a.clutch = 1.07, 1.02
		case "Entry":
			a.aim, a.move = 1.05, 1.1
		case "Support":
			a.aim, a.util = 0.97, 1.1
		case "Lurker":
			a.aim, a.clutch = 1.0, 1.12
		}
		j := func() float64 { return (r.Float64() - 0.5) * 0.06 }
		a.aim += j()
		a.util += j()
		a.move += j()
		a.clutch += j()
		base[i] = a
	}
	perm := r.Perm(len(handlePool))
	for t := 0; t < 2; t++ {
		for i := 0; i < 5; i++ {
			a := base[i]
			m.players[t][i] = &Player{
				ID: string(rune('a'+t)) + string(rune('0'+i)), Handle: handlePool[perm[t*5+i]], Role: roles[i], Team: t, Slot: i,
				Aim: a.aim, Util: a.util, Move: a.move, Clutch: a.clutch,
			}
		}
	}
	m.startT = r.Intn(2)
	m.tTeam = m.startT
	m.resetHalf(startMoney)
	return m
}
