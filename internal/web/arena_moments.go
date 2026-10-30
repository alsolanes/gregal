package web

// Els «prompt moments» d'un partit: les jugades on una regla del .md d'un
// equip va pesar de debò. L'anàlisi del final (castAnalysis) en demana fins a
// sis al model; el servidor les comprova (la decisió existeix, la regla és al
// .md d'aquella banda, i hi ha una jugada que en surt) i, si el model falla o
// el partit és antic o de mostra, les tria ell: jugades que donen punts (o
// grans jugades) fetes amb una regla trobada al .md de qui en treu profit.
// La retransmissió les ensenya durant el partit, al resum final i al vídeo.

import (
	"fmt"
	"sort"
	"strings"
)

const arenaMaxMoments = 6

type arenaMoment struct {
	N     int    `json:"n"`    // la decisió
	Side  int    `json:"side"` // l'equip de la regla
	Rule  string `json:"rule"` // la línia del .md
	What  string `json:"what"` // què va passar (≤20 paraules)
	Why   string `json:"why"`  // per què hi va pesar la regla (≤25 paraules)
	Event int    `json:"event,omitempty"`
}

// clipWords deixa un text en una línia i com a molt n paraules.
func clipWords(t string, n int) string {
	w := strings.Fields(t)
	if len(w) <= n {
		return strings.Join(w, " ")
	}
	return strings.TrimRight(strings.Join(w[:n], " "), ",;:") + "…"
}

func arenaDecisionOf(m *arenaMatch, n, side int) *arenaDecision {
	for i := range m.Decisions {
		if d := &m.Decisions[i]; d.N == n && d.Side == side {
			return d
		}
	}
	return nil
}

// arenaGains: el que cada jugada fa pujar el marcador de cada banda.
func arenaGains(m *arenaMatch) [][2]int {
	out := make([][2]int, len(m.Events))
	var prev [2]int
	for i, e := range m.Events {
		for s := 0; s < 2; s++ {
			if g := e.Score[s] - prev[s]; g > 0 {
				out[i][s] = g
			}
		}
		prev = e.Score
	}
	return out
}

// arenaMomentEvent: la jugada que ensenya el moment (la jugada clau d'aquella
// decisió on guanya la banda; si no, la primera jugada clau; si no, la primera).
// Torna l'índex a m.Events, o -1.
func arenaMomentEvent(m *arenaMatch, gains [][2]int, n, side int) int {
	first, hl := -1, -1
	for i, e := range m.Events {
		if e.Decision != n {
			continue
		}
		if first < 0 {
			first = i
		}
		if gains[i][side] > 0 {
			return i
		}
		if e.Highlight && hl < 0 {
			hl = i
		}
	}
	if hl >= 0 {
		return hl
	}
	return first
}

// arenaCheckMoments valida els moments del model: decisió que existeix, regla
// que és de debò al .md d'aquella banda i una jugada que en surt.
func arenaCheckMoments(m *arenaMatch, in []arenaMoment) []arenaMoment {
	gains := arenaGains(m)
	seen := map[int]bool{}
	var out []arenaMoment
	for _, mo := range in {
		if mo.Side != 0 && mo.Side != 1 || seen[mo.N] || arenaDecisionOf(m, mo.N, mo.Side) == nil {
			continue
		}
		if !ruleInMD(mo.Rule, m.MD[mo.Side]) {
			continue
		}
		ev := arenaMomentEvent(m, gains, mo.N, mo.Side)
		if ev < 0 {
			continue
		}
		mo.Rule, mo.What, mo.Why = oneLine(mo.Rule, 300), clipWords(mo.What, 20), clipWords(mo.Why, 25)
		if mo.What == "" {
			mo.What = clipWords(m.Events[ev].Text, 20)
		}
		mo.Event = m.Events[ev].N
		seen[mo.N] = true
		out = append(out, mo)
		if len(out) == arenaMaxMoments {
			break
		}
	}
	return out
}

// arenaFallbackMoments: sense model, les jugades que donen més marcador
// (millor si són jugades clau) fetes amb una regla trobada al .md de qui en
// treu profit, repartides pel partit i, si es pot, dels dos equips.
func arenaFallbackMoments(m *arenaMatch, max int) []arenaMoment {
	gains := arenaGains(m)
	maxGain := 1
	for _, g := range gains {
		for s := 0; s < 2; s++ {
			if g[s] > maxGain {
				maxGain = g[s]
			}
		}
	}
	type cand struct {
		ev, side int
		w        float64
	}
	var cands []cand
	for i, e := range m.Events {
		if e.Decision <= 0 {
			continue
		}
		var sides []int
		for s := 0; s < 2; s++ {
			if gains[i][s] > 0 {
				sides = append(sides, s)
			}
		}
		// (Una gran jugada sense punts, com un sac, no diu de qui és el mèrit.)
		for _, s := range sides {
			d := arenaDecisionOf(m, e.Decision, s)
			if d == nil || !d.RuleFound || d.Fallback || strings.TrimSpace(d.Rule) == "" {
				continue
			}
			w := float64(gains[i][s]) / float64(maxGain)
			if e.Highlight {
				w += .5
			}
			if s == m.Winner {
				w += .25
			}
			cands = append(cands, cand{i, s, w})
		}
	}
	sort.SliceStable(cands, func(a, b int) bool { return cands[a].w > cands[b].w })
	// Repartits: primer amb una distància mínima entre moments i com a molt
	// max-2 d'un mateix equip, després sense.
	gap := len(m.Events) / (max * 2)
	usedN := map[int]bool{}
	var pick []cand
	var perSide [2]int
	for pass, minGap := range []int{gap, 0} {
		for _, c := range cands {
			if len(pick) >= max {
				break
			}
			n := m.Events[c.ev].Decision
			if usedN[n] || pass == 0 && perSide[c.side] >= max-2 {
				continue
			}
			near := false
			for _, p := range pick {
				if d := p.ev - c.ev; d < minGap && -d < minGap {
					near = true
					break
				}
			}
			if near {
				continue
			}
			usedN[n] = true
			perSide[c.side]++
			pick = append(pick, c)
		}
	}
	sort.Slice(pick, func(a, b int) bool { return pick[a].ev < pick[b].ev })
	out := make([]arenaMoment, 0, len(pick))
	for _, c := range pick {
		e := m.Events[c.ev]
		d := arenaDecisionOf(m, e.Decision, c.side)
		why := clipWords(d.Why, 25)
		if why == "" {
			why = fmt.Sprintf("%s followed this line of its .md.", m.Names[c.side])
		}
		out = append(out, arenaMoment{N: e.Decision, Side: c.side, Rule: oneLine(d.Rule, 300), What: clipWords(e.Text, 20), Why: why, Event: e.N})
	}
	return out
}

// arenaMergeMoments: els del model (comprovats) i, si en surten menys de tres,
// els del servidor fins a sis; en ordre de partit.
func arenaMergeMoments(m *arenaMatch, model []arenaMoment) []arenaMoment {
	out := arenaCheckMoments(m, model)
	if len(out) < 3 {
		used := map[int]bool{}
		for _, mo := range out {
			used[mo.N] = true
		}
		for _, mo := range arenaFallbackMoments(m, arenaMaxMoments) {
			if len(out) >= arenaMaxMoments {
				break
			}
			if !used[mo.N] {
				used[mo.N] = true
				out = append(out, mo)
			}
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Event < out[b].Event })
	if out == nil {
		out = []arenaMoment{}
	}
	return out
}

// arenaMomentCandidates: el que el model veu per triar els moments: les
// jugades clau o que fan marcador amb la decisió de cada banda i la seva regla.
func arenaMomentCandidates(m *arenaMatch) string {
	gains := arenaGains(m)
	var lines []string
	seen := map[[2]int]bool{}
	for i, e := range m.Events {
		if e.Decision <= 0 || !e.Highlight && gains[i] == [2]int{} {
			continue
		}
		for s := 0; s < 2; s++ {
			d := arenaDecisionOf(m, e.Decision, s)
			if d == nil || !d.RuleFound || seen[[2]int{d.N, s}] {
				continue
			}
			seen[[2]int{d.N, s}] = true
			lines = append(lines, fmt.Sprintf("d%d side%d %s | play: %s | chose %s because %s | rule: %s",
				d.N, s, m.Names[s], oneLine(e.Text, 100), oneLine(string(d.Decision), 70), oneLine(d.Why, 110), d.Rule))
		}
	}
	// Com a molt 40, repartides pel partit.
	if n := len(lines); n > 40 {
		var keep []string
		for i := 0; i < 40; i++ {
			keep = append(keep, lines[i*n/40])
		}
		lines = keep
	}
	return strings.Join(lines, "\n")
}

// arenaOnlyMoments: sense l'anàlisi del model, almenys els moments del servidor.
func arenaOnlyMoments(m *arenaMatch) *arenaAnalysis {
	return &arenaAnalysis{Side: -1, Moments: arenaMergeMoments(m, nil)}
}

// arenaWithMoments: la mateixa anàlisi amb els moments calculats si no en té
// (partits antics i de mostra); no toca l'original.
func arenaWithMoments(m *arenaMatch) {
	if m.Status != "final" || m.Analysis != nil && m.Analysis.Moments != nil {
		return
	}
	a := arenaAnalysis{Side: -1}
	if m.Analysis != nil {
		a = *m.Analysis
	}
	a.Moments = arenaMergeMoments(m, nil)
	m.Analysis = &a
}

// arenaHideMoments treu els moments de la banda el .md de la qual no es pot veure.
func arenaHideMoments(m *arenaMatch, side int) {
	if m.Analysis == nil || len(m.Analysis.Moments) == 0 {
		return
	}
	a := *m.Analysis
	a.Moments = nil
	for _, mo := range m.Analysis.Moments {
		if mo.Side != side {
			a.Moments = append(a.Moments, mo)
		}
	}
	if a.Moments == nil {
		a.Moments = []arenaMoment{}
	}
	m.Analysis = &a
}

// arenaRenumber: als partits desats abans que cada decisió portés el número de
// la jugada (al draft del LoL de vegades només decideix un equip i el número
// es desquadrava), es refà el pas de cada decisió: un pas nou comença quan
// torna a decidir la mateixa banda o canvia el moment (short). Torna els
// números nous (en l'ordre de m.Decisions) o nil si no cal o no quadra.
func arenaRenumber(m *arenaMatch) []int {
	maxEv, maxN := 0, 0
	for _, e := range m.Events {
		if e.Decision > maxEv {
			maxEv = e.Decision
		}
	}
	for _, d := range m.Decisions {
		if d.N > maxN {
			maxN = d.N
		}
	}
	if maxN >= maxEv || len(m.Decisions) == 0 {
		return nil
	}
	out := make([]int, len(m.Decisions))
	step, oldN, short := 0, -1, ""
	var sides [2]bool
	for i, d := range m.Decisions {
		if step == 0 || d.N != oldN || d.Short != short || d.Side < 0 || d.Side > 1 || sides[d.Side] {
			step++
			oldN, short, sides = d.N, d.Short, [2]bool{}
		}
		if d.Side == 0 || d.Side == 1 {
			sides[d.Side] = true
		}
		out[i] = step
	}
	if step != maxEv {
		return nil
	}
	return out
}
