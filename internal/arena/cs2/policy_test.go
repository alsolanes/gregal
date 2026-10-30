package cs2

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"gregal/internal/arena"
)

// Polítiques de prova escrites a mà. Totes llegeixen NOMÉS el text de la
// Situation (com ho faria un model amb el .md), mai l'estat intern.

var (
	reMoney    = regexp.MustCompile(`Your money: \$([\d,]+) total`)
	reFull     = regexp.MustCompile(`Full buy now costs your team about \$([\d,]+) \((affordable|NOT affordable)\)`)
	reLoss     = regexp.MustCompile(`each player gets \$([\d,]+) \(level (\d)/4`)
	reRound    = regexp.MustCompile(`^Map: Mirage\. Round (\d+) \([^,]*, round (\d+) of (\d+) of this half`)
	reScore    = regexp.MustCompile(`Score: you (\d+) - (\d+) them`)
	reEcoNext  = regexp.MustCompile(`If you eco now and lose, you will have about \$([\d,]+) next round \((enough|still NOT enough)`)
	reHistR    = regexp.MustCompile(`^- R(\d+) \[(they|we) (T|CT)\] (\w+) buy: (\w+)`)
	reHit      = regexp.MustCompile(`hit ([AB-])`)
	rePosts    = regexp.MustCompile(`\((\d) A, (\d) mid, (\d) B\)`)
	reVsTheirs = regexp.MustCompile(`vs their (\w+)`)
)

func num(s string) int { v, _ := strconv.Atoi(strings.ReplaceAll(s, ",", "")); return v }

type hline struct {
	round        int
	buy, call    string // compra observada i estratègia/configuració
	site         string // lloc atacat (A, B o -)
	won          bool   // l'hem guanyada nosaltres
	vs           string // (línies nostres) la crida del rival
	postA, postB int
}

type sit struct {
	ct                  bool
	round, pos, halfLen int
	us, them            int
	money, full         int
	affordable          bool
	loss, lossLvl       int
	pistol, last        bool
	ecoNote, ecoEnough  bool
	enemyPoor           bool
	their, ours         []hline
}

func parseSit(s string) sit {
	var x sit
	x.ct = strings.Contains(s, "You are CT")
	if m := reMoney.FindStringSubmatch(s); m != nil {
		x.money = num(m[1])
	}
	if m := reFull.FindStringSubmatch(s); m != nil {
		x.full, x.affordable = num(m[1]), m[2] == "affordable"
	}
	if m := reLoss.FindStringSubmatch(s); m != nil {
		x.loss, x.lossLvl = num(m[1]), num(m[2])
	}
	if m := reRound.FindStringSubmatch(s); m != nil {
		x.round, x.pos, x.halfLen = num(m[1]), num(m[2]), num(m[3])
	}
	if m := reScore.FindStringSubmatch(s); m != nil {
		x.us, x.them = num(m[1]), num(m[2])
	}
	if m := reEcoNext.FindStringSubmatch(s); m != nil {
		x.ecoNote, x.ecoEnough = true, m[2] == "enough"
	}
	x.pistol = strings.Contains(s, "PISTOL ROUND") || strings.Contains(s, "Overtime half start")
	x.last = strings.Contains(s, "Last round of the half")
	x.enemyPoor = strings.Contains(s, "probably cannot full buy")
	for _, l := range strings.Split(s, "\n") {
		if !strings.HasPrefix(l, "- R") {
			continue
		}
		m := reHistR.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		h := hline{round: num(m[1]), buy: m[4], call: m[5]}
		if hm := reHit.FindStringSubmatch(l); hm != nil {
			h.site = hm[1]
		}
		if pm := rePosts.FindStringSubmatch(l); pm != nil {
			h.postA, h.postB = num(pm[1]), num(pm[3])
		}
		if m[2] == "they" {
			h.won = strings.Contains(l, "-> we won")
			x.their = append(x.their, h)
		} else {
			h.won = strings.Contains(l, "-> we WON")
			if vm := reVsTheirs.FindStringSubmatch(l); vm != nil {
				h.vs = vm[1]
			}
			x.ours = append(x.ours, h)
		}
	}
	return x
}

func enc(d map[string]any) json.RawMessage { raw, _ := json.Marshal(d); return raw }

// ---- la política del .md inicial ("starter") ----

// starterBuy: les regles d'economia del .md inicial.
var nearFull = 60 // % del cost de la compra completa a partir del qual es força

// starterBuy: les regles d'economia del .md inicial.
func starterBuy(x sit) string {
	switch {
	case x.pistol || x.affordable:
		return "full"
	case x.last || x.money*100 >= x.full*nearFull:
		return "force"
	}
	return "eco"
}

// last n entries
func lastN(h []hline, n int) []hline {
	if len(h) > n {
		return h[len(h)-n:]
	}
	return h
}

// counterSetup: la millor resposta T a una configuració CT.
func counterSetup(setup string, round int) string {
	switch setup {
	case "stack_a":
		return "fake_a_go_b"
	case "stack_b":
		return "fake_b_go_a"
	case "aggressive_mid":
		return []string{"rush_a", "rush_b"}[round%2]
	case "retake_a":
		return "execute_a"
	case "retake_b":
		return "execute_b"
	case "standard_2_1_2":
		return []string{"fake_a_go_b", "fake_b_go_a"}[round%2]
	}
	return "default_mid"
}

// counterStrat: la millor resposta CT a una estratègia T (i al lloc que ataca).
func counterStrat(strat, site string) string {
	switch {
	case strat == "default_mid":
		return "aggressive_mid"
	case site == "A":
		return "stack_a"
	case site == "B":
		return "stack_b"
	}
	return "standard_2_1_2"
}

func mostCommon(h []hline, key func(hline) string) (string, int) {
	cnt := map[string]int{}
	best, bn := "", 0
	for i := len(h) - 1; i >= 0; i-- { // empat: el més recent
		k := key(h[i])
		cnt[k]++
	}
	for i := len(h) - 1; i >= 0; i-- {
		k := key(h[i])
		if cnt[k] > bn {
			best, bn = k, cnt[k]
		}
	}
	return best, bn
}

// lostTwiceWith: les nostres dues últimes rondes en aquesta banda, perdudes amb aquesta crida.
func lostTwiceWith(ours []hline, call string) bool {
	if len(ours) < 2 {
		return false
	}
	l := ours[len(ours)-2:]
	return l[0].call == call && l[1].call == call && !l[0].won && !l[1].won
}

func starter(_ int, p arena.Prompt) json.RawMessage {
	x := parseSit(p.Situation)
	b := starterBuy(x)
	recent := lastN(x.their, 3)
	if x.ct {
		setup, util := "standard_2_1_2", "normal"
		if len(recent) >= 2 {
			st, sn := mostCommon(recent, func(h hline) string { return h.call })
			site, hn := mostCommon(recent, func(h hline) string { return h.site })
			switch {
			case st == "default_mid" && sn >= 2:
				setup = "aggressive_mid"
			case hn >= 2 && site != "-":
				setup = counterStrat("", site)
			}
		}
		if lostTwiceWith(x.ours, setup) {
			setup = "standard_2_1_2"
		}
		switch b {
		case "full":
			util = "heavy"
		case "eco":
			util = "save"
			if site, _ := mostCommon(x.their, func(h hline) string { return h.site }); site == "A" || site == "B" {
				setup = counterStrat("", site)
			}
		}
		return enc(map[string]any{"buy": b, "setup": setup, "utility": util, "awp": b == "full"})
	}
	strat, util, pace := "default_mid", "full", "default"
	if len(recent) > 0 {
		st, _ := mostCommon(recent, func(h hline) string { return h.call })
		strat = counterSetup(st, x.round)
		if lostTwiceWith(x.ours, strat) {
			strat = counterSetup(recent[len(recent)-1].call, x.round+1)
			if lostTwiceWith(x.ours, strat) {
				strat = "default_mid"
			}
		}
	}
	switch b {
	case "force":
		util = "smokes"
	case "eco":
		util = "none"
		strat = "rush_b"
		if n := len(x.their); n > 0 && x.their[n-1].postA < x.their[n-1].postB {
			strat = "rush_a"
		}
	}
	if strings.HasPrefix(strat, "rush") {
		pace = "fast"
	}
	return enc(map[string]any{"buy": b, "strategy": strat, "utility": util, "pace": pace, "awp": b == "full"})
}

// exploit: compta TOT l'historial del rival a la seva banda i contraresta el
// que més fa.
func exploit(_ int, p arena.Prompt) json.RawMessage {
	x := parseSit(p.Situation)
	b := starterBuy(x)
	if x.ct {
		setup, util := "standard_2_1_2", "heavy"
		if len(x.their) > 0 {
			st, sn := mostCommon(x.their, func(h hline) string { return h.call })
			site, _ := mostCommon(x.their, func(h hline) string { return h.site })
			if st == "default_mid" && sn*2 >= len(x.their) {
				setup = "aggressive_mid"
			} else {
				setup = counterStrat("", site)
			}
		}
		if b != "full" {
			util = "normal"
		}
		return enc(map[string]any{"buy": b, "setup": setup, "utility": util, "awp": true})
	}
	strat, util, pace := "default_mid", "full", "default"
	if len(x.their) > 0 {
		st, _ := mostCommon(x.their, func(h hline) string { return h.call })
		strat = counterSetup(st, x.round)
	}
	if b != "full" {
		util = "smokes"
	}
	if strings.HasPrefix(strat, "rush") {
		pace = "fast"
	}
	return enc(map[string]any{"buy": b, "strategy": strat, "utility": util, "pace": pace, "awp": true})
}

// constant: sempre la mateixa crida ("" = no dir-ho, el motor posa el defecte).
func constant(buy, strat, setup string) arena.Decide {
	return func(_ int, p arena.Prompt) json.RawMessage {
		d := map[string]any{}
		if buy != "" {
			d["buy"] = buy
		}
		if strings.Contains(p.Situation, "You are CT") {
			if setup != "" {
				d["setup"] = setup
			}
		} else if strat != "" {
			d["strategy"] = strat
		}
		return enc(d)
	}
}

// withBuy: la mateixa política amb una altra regla de compra.
func withBuy(base arena.Decide, rule func(sit) string) arena.Decide {
	return func(side int, p arena.Prompt) json.RawMessage {
		var d map[string]any
		_ = json.Unmarshal(base(side, p), &d)
		d["buy"] = rule(parseSit(p.Situation))
		return enc(d)
	}
}

func empty(int, arena.Prompt) json.RawMessage { return nil }
