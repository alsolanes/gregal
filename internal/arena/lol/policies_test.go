package lol

// Polítiques de prova que juguen NOMÉS amb el text de la Situation (i les
// regles publicades), com ho faria un model llegint un .md:
//   - starter: segueix la lògica del .md d'exemple (starter.go);
//   - exploiter: el mateix, però compta exactament el que el rival ha fet
//     abans i el contraresta (contra polítiques constants, guanya fàcil);
//   - constants: draft amb una llista fixa, línies fixes, postura i
//     objectiu fixos (com un .md de principiant: "sempre engage").

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gregal/internal/arena"
)

type policy func(m *match, side int, p arena.Prompt) json.RawMessage

func js(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

// ---- lectura de la Situation ----

var (
	reNow      = regexp.MustCompile(`NOW: (ban phase \d) — you ban (\d+)|NOW: your pick \S+ — choose (\d+)`)
	reAvail    = regexp.MustCompile(`(?m)^- (\w+) \| ([a-z/]+) \| (\w+) \| (AD|AP) \| (\w+) \| ([a-z0-9 ]*?)( \(no open role for you\))?$`)
	reTrait    = regexp.MustCompile(`([a-z]+)(\d)`)
	reSoFar    = regexp.MustCompile(`(?m)^(Your|Enemy) comp so far: (.*)$`)
	reOpen     = regexp.MustCompile(`(?m)^(Your|Enemy) picks: .* \| open roles: (.*)$`)
	reNum      = regexp.MustCompile(`([A-Za-z ]+?) (\d+)`)
	rePower    = regexp.MustCompile(`POWER NOW: you ([+-]\d+)%.*In 8 min at the same gold: you ([+-]\d+)%`)
	reTime     = regexp.MustCompile(`game time (\d+):`)
	reGold     = regexp.MustCompile(`GOLD DIFF ([+-]?\d+)`)
	reOpts     = regexp.MustCompile(`OBJECTIVES YOU CAN CHOOSE NOW: ([a-z, ]+)\.`)
	reComp     = regexp.MustCompile(`(?m)^(YOUR|ENEMY) COMP: (\w+), best stance (\w+) \| stance fit: engage ([\d.]+), siege ([\d.]+), pick ([\d.]+), split ([\d.]+), protect ([\d.]+) \| frontline (\d+), peel (\d+), engage (\d+), poke (\d+), waveclear (\d+), pick (\d+)`)
	reCond     = regexp.MustCompile(`protect beats engage — you (YES|no), enemy (YES|no) \| split beats engage — you (YES|no), enemy (YES|no)`)
	reDragons  = regexp.MustCompile(`Dragons: you (\d+) \[[^\]]*\], enemy (\d+)`)
	reLaneM    = regexp.MustCompile(`(top|mid|bot|jungle) you ([\d.]+) vs enemy ([\d.]+)`)
	reLaneGold = regexp.MustCompile(`Lane gold leads \(you\): top ([+-]\d+), mid ([+-]\d+), bot ([+-]\d+)`)
)

type sit struct {
	power, later float64
	min          float64 // minut de joc
	gold         float64
	opts         []string
	myCurve      string
	enCurve      string
	enBest       string
	myFit, enFit map[string]float64
	enT          map[string]float64 // trets de l'enemic: frontline, peel, engage, poke, waveclear, pick
	protOK       [2]bool            // jo, enemic
	splitOK      [2]bool
	enStances    []string
	enObjs       []string
	drakes       [2]int
	soulPoint    bool
	myBuff       bool
	enBuff       bool
	laneD        map[string]float64
	laneGold     [3]float64
}

func atof(s string) float64 { v, _ := strconv.ParseFloat(strings.TrimPrefix(s, "+"), 64); return v }

func parseSit(t string) sit {
	var s sit
	s.laneD = map[string]float64{}
	for _, ln := range strings.Split(t, "\n") {
		switch {
		case strings.HasPrefix(ln, "LEAGUE OF LEGENDS"):
			if x := reTime.FindStringSubmatch(ln); x != nil {
				s.min = atof(x[1])
			}
		case strings.HasPrefix(ln, "SCORE:"):
			if x := reGold.FindStringSubmatch(ln); x != nil {
				s.gold = atof(x[1])
			}
		case strings.HasPrefix(ln, "POWER NOW"):
			if x := rePower.FindStringSubmatch(ln); x != nil {
				s.power, s.later = atof(x[1]), atof(x[2])
			}
		case strings.HasPrefix(ln, "OBJECTIVES"):
			if x := reOpts.FindStringSubmatch(ln); x != nil {
				s.opts = strings.Split(x[1], ", ")
			}
			if x := reDragons.FindStringSubmatch(ln); x != nil {
				s.drakes = [2]int{int(atof(x[1])), int(atof(x[2]))}
			}
			s.soulPoint = strings.Contains(ln, "SOUL POINT")
		case strings.HasPrefix(ln, "Void grubs"):
			s.myBuff = strings.Contains(ln, "Your has BARON BUFF") || strings.Contains(ln, "Your has ELDER BUFF")
			s.enBuff = strings.Contains(ln, "Enemy has BARON BUFF") || strings.Contains(ln, "Enemy has ELDER BUFF")
		case strings.HasPrefix(ln, "Lane gold leads"):
			if x := reLaneGold.FindStringSubmatch(ln); x != nil {
				s.laneGold = [3]float64{atof(x[1]), atof(x[2]), atof(x[3])}
			}
		case strings.HasPrefix(ln, "YOUR COMP:") || strings.HasPrefix(ln, "ENEMY COMP:"):
			x := reComp.FindStringSubmatch(ln)
			if x == nil {
				continue
			}
			fit := map[string]float64{}
			for k, st := range stances {
				fit[st] = atof(x[4+k])
			}
			if x[1] == "YOUR" {
				s.myCurve, s.myFit = strings.ToLower(x[2]), fit
			} else {
				s.enCurve, s.enBest, s.enFit = strings.ToLower(x[2]), x[3], fit
				s.enT = map[string]float64{"frontline": atof(x[9]), "peel": atof(x[10]), "engage": atof(x[11]), "poke": atof(x[12]), "waveclear": atof(x[13]), "pick": atof(x[14])}
			}
		case strings.HasPrefix(ln, "LANE MATCHUPS"):
			for _, x := range reLaneM.FindAllStringSubmatch(ln, -1) {
				s.laneD[x[1]] = atof(x[2]) - atof(x[3])
			}
		case strings.HasPrefix(ln, "CONDITIONAL COUNTERS"):
			if x := reCond.FindStringSubmatch(ln); x != nil {
				s.protOK = [2]bool{x[1] == "YES", x[2] == "YES"}
				s.splitOK = [2]bool{x[3] == "YES", x[4] == "YES"}
			}
		case strings.HasPrefix(ln, "ENEMY STANCES SO FAR: "):
			s.enStances = strings.Split(strings.TrimPrefix(ln, "ENEMY STANCES SO FAR: "), ", ")
		case strings.HasPrefix(ln, "ENEMY OBJECTIVES SO FAR: "):
			if v := strings.TrimPrefix(ln, "ENEMY OBJECTIVES SO FAR: "); v != "" {
				s.enObjs = strings.Split(v, ", ")
			}
		}
	}
	return s
}

func (s sit) has(o string) bool {
	for _, x := range s.opts {
		if x == o {
			return true
		}
	}
	return false
}

// ---- draft a partir del text ----

type dchamp struct {
	name, roles, class, dmg, curve string
	t                              map[string]float64
	legal                          bool
}

type dsit struct {
	ban    bool
	n      int
	avail  []dchamp
	my, en map[string]float64 // trets acumulats (comp so far), "ap", "early", "late", "count"
	open   [2][]string
}

func parseDraft(t string) dsit {
	var d dsit
	if x := reNow.FindStringSubmatch(t); x != nil {
		if x[1] != "" {
			d.ban, d.n = true, int(atof(x[2]))
		} else {
			d.n = int(atof(x[3]))
		}
	}
	for _, ln := range strings.Split(t, "\n") {
		if !strings.HasPrefix(ln, "- ") {
			continue
		}
		x := reAvail.FindStringSubmatch(ln)
		if x == nil {
			continue
		}
		c := dchamp{name: x[1], roles: x[2], class: x[3], dmg: x[4], curve: x[5], t: map[string]float64{}, legal: x[7] == ""}
		for _, y := range reTrait.FindAllStringSubmatch(x[6], -1) {
			c.t[y[1]] = atof(y[2])
		}
		d.avail = append(d.avail, c)
	}
	d.my, d.en = map[string]float64{}, map[string]float64{}
	for _, x := range reSoFar.FindAllStringSubmatch(t, -1) {
		mp := d.my
		if x[1] == "Enemy" {
			mp = d.en
		}
		for _, y := range reNum.FindAllStringSubmatch(x[2], -1) {
			k := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(y[1]), "|"))
			k = strings.TrimSpace(strings.TrimPrefix(k, "curve"))
			k = strings.TrimSpace(strings.TrimPrefix(k, "damage"))
			mp[k] = atof(y[2])
		}
		mp["count"] = mp["early"] + mp["mid"] + mp["late"]
	}
	for _, x := range reOpen.FindAllStringSubmatch(t, -1) {
		k := 0
		if x[1] == "Enemy" {
			k = 1
		}
		d.open[k] = strings.Split(x[2], ", ")
	}
	return d
}

// Fórmules de postura publicades a les regles.
var pubAvg = map[string]float64{"engage": 19.1, "siege": 19.3, "pick": 15.0, "split": 13.7, "protect": 17.5}

func pubRaw(t map[string]float64, st string) float64 {
	switch st {
	case "engage":
		return 2*t["engage"] + t["frontline"] + 0.5*t["dps"]
	case "siege":
		return 2*t["poke"] + t["waveclear"] + 0.3*t["dps"]
	case "pick":
		return 2*t["pick"] + 0.7*t["engage"] + 0.3*t["poke"]
	case "split":
		return 3*t["best splitpusher"] + t["splitpush"] + 0.5*t["waveclear"]
	case "protect":
		return 2*t["peel"] + t["dps"] + 0.3*t["frontline"]
	}
	return 0
}

func pubFit(t map[string]float64, k float64) map[string]float64 {
	out := map[string]float64{}
	for _, st := range stances {
		out[st] = pubRaw(t, st) / (pubAvg[st] * k / 5)
	}
	return out
}

// addChamp: trets de la composició amb un campió més.
func addChamp(t map[string]float64, c dchamp) map[string]float64 {
	out := map[string]float64{}
	for k, v := range t {
		out[k] = v
	}
	for _, k := range []string{"engage", "peel", "poke", "waveclear", "splitpush", "pick", "objective", "frontline", "dps"} {
		out[k] += c.t[k]
	}
	if c.t["splitpush"] > out["best splitpusher"] {
		out["best splitpusher"] = c.t["splitpush"]
	}
	if c.dmg == "AP" {
		out["AP"]++
	} else {
		out["AD"]++
	}
	out[c.curve]++
	out["count"]++
	return out
}

// draftScore: el que diu el .md d'exemple (identitat clara, primera línia,
// dany mixt, corba coherent, contrarestar el que ensenya l'enemic).
func draftScore(d dsit, c dchamp) float64 {
	after := addChamp(d.my, c)
	k := after["count"]
	fit := pubFit(after, k)
	fs := []float64{}
	for _, st := range stances {
		fs = append(fs, fit[st])
	}
	sort.Float64s(fs)
	// Una identitat clara (fins a 1.4) i flexibilitat per contrarestar.
	v := math.Min(fs[4], 1.4) + 0.5*math.Min(fs[3], 1.2) + 0.25*math.Min(fs[2], 1.1)
	left := 5 - k
	if after["frontline"]+2.5*left < 3 {
		v -= 0.6
	} else if left == 0 && after["frontline"] < 5 {
		v -= 0.15
	}
	if after["AP"]+left < 2 || after["AD"]+left < 2 {
		v -= 0.5
	}
	if after["early"] >= 2 && c.curve == "late" || after["late"] >= 2 && c.curve == "early" {
		v -= 0.08
	}
	if d.en["count"] > 0 {
		// L'enemic fa engage → peel i primera línia (per poder jugar protect).
		if d.en["engage"]*5/d.en["count"] >= 6 {
			v += 0.05*c.t["peel"] + 0.03*c.t["frontline"]
		}
		// Enemic fràgil → un assassí.
		if d.en["count"] >= 3 && d.en["frontline"]+d.en["peel"] <= 4 {
			v += 0.05 * c.t["pick"]
		}
		// Enemic de poke → engage dur.
		if d.en["poke"]*5/d.en["count"] >= 6 {
			v += 0.05 * c.t["engage"]
		}
	}
	return v + 0.02*c.t["lane"]
}

// jitter: variació determinista per partit (un model tampoc no fa sempre el
// mateix draft). No porta cap informació de la partida.
func jitter(m *match, side int, name string) float64 {
	h := uint64(m.seed)*0x9E3779B97F4A7C15 ^ uint64(side*7919+m.dec*131)
	for _, c := range name {
		h = (h ^ uint64(c)) * 0x100000001B3
	}
	h ^= h >> 29
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 32
	return float64(h%1000)/1000*0.12 - 0.06
}

func starterDraft(m *match, side int, p arena.Prompt) json.RawMessage {
	d := parseDraft(p.Situation)
	if d.ban {
		type cv struct {
			n string
			v float64
		}
		cs := []cv{}
		for _, c := range d.avail {
			v := c.t["engage"] + c.t["frontline"] + c.t["dps"] + c.t["peel"]
			if c.t["engage"] == 3 && c.t["frontline"] == 3 {
				v += 4
			}
			if c.curve == "late" && c.t["dps"] == 3 {
				v += 3
			}
			// Fase 2: el que completa la identitat de l'enemic (rols que li falten).
			if d.en["count"] > 0 {
				ef := pubFit(d.en, d.en["count"])
				best := bestStance(ef)
				v = pubRaw(addChamp(d.en, c), best) - pubRaw(d.en, best)
				fits := false
				for _, r := range strings.Split(c.roles, "/") {
					for _, o := range d.open[1] {
						if r == o {
							fits = true
						}
					}
				}
				if !fits {
					v -= 10
				}
			}
			cs = append(cs, cv{c.name, v + 3*jitter(m, side, c.name)})
		}
		sort.SliceStable(cs, func(a, b int) bool { return cs[a].v > cs[b].v })
		out := []string{}
		for k := 0; k < len(cs) && k < 6; k++ {
			out = append(out, cs[k].n)
		}
		return js(map[string]any{"bans": out})
	}
	out := []string{}
	used := map[string]bool{}
	for k := 0; k < d.n; k++ {
		best, bv := -1, -1e9
		for idx, c := range d.avail {
			if !c.legal || used[c.name] || !roleFree(d, c) {
				continue
			}
			if v := draftScore(d, c) + jitter(m, side, c.name); v > bv {
				best, bv = idx, v
			}
		}
		if best < 0 {
			break
		}
		c := d.avail[best]
		used[c.name] = true
		out = append(out, c.name)
		d.my = addChamp(d.my, c)
		d.open[0] = takeRole(d.open[0], c)
	}
	return js(map[string]any{"picks": out})
}

func roleFree(d dsit, c dchamp) bool {
	for _, r := range strings.Split(c.roles, "/") {
		for _, o := range d.open[0] {
			if r == o {
				return true
			}
		}
	}
	return false
}

func takeRole(open []string, c dchamp) []string {
	for _, r := range strings.Split(c.roles, "/") {
		for i, o := range open {
			if r == o {
				return append(append([]string{}, open[:i]...), open[i+1:]...)
			}
		}
	}
	return open
}

// ---- fases de partida (starter) ----

func starterLaning(s sit) json.RawMessage {
	out := map[string]string{}
	bestL, bestD := "", 0.4
	for _, l := range roleLane {
		d := s.laneD[l]
		out[l] = "even"
		if d >= 0.5 {
			out[l] = "aggressive"
		} else if d <= -0.5 {
			out[l] = "passive"
		}
		if d > bestD {
			bestL, bestD = l, d
		}
	}
	out["jungle"] = "farm"
	switch {
	case s.laneD["jungle"] >= 0.8:
		out["jungle"] = "invade"
	case bestL != "":
		out["jungle"] = "gank_" + bestL
	}
	return js(out)
}

func starterObjective(s sit) json.RawMessage {
	if s.power >= -3 {
		return js(map[string]string{"objective": "dragon"})
	}
	return js(map[string]string{"objective": "grubs"})
}

func starterPlates(s sit) json.RawMessage {
	bl, bv := 1, -1e9
	for l := 0; l < 3; l++ {
		if s.laneGold[l] > bv {
			bl, bv = l, s.laneGold[l]
		}
	}
	switch {
	case bv > 800 && s.power >= 5:
		return js(map[string]string{"focus": roleLane[bl], "action": "dive"})
	case bv > 0:
		return js(map[string]string{"focus": roleLane[bl], "action": "plates"})
	}
	return js(map[string]string{"focus": "mid", "action": "farm"})
}

// counters: les postures que guanyen b (segons les regles i la línia de
// CONDITIONAL COUNTERS). me = jo, en = enemic.
func counters(s sit, b string) []string {
	out := []string{}
	for _, a := range stances {
		if a == b {
			continue
		}
		win := versus(a, b) > 0
		switch {
		case a == "split" && b == "engage":
			win = s.splitOK[0]
		case a == "protect" && b == "engage":
			win = s.protOK[0]
		case a == "engage" && b == "split":
			win = !s.splitOK[1] && s.gold >= -1000
		case a == "engage" && b == "protect":
			win = !s.protOK[1] && s.gold >= -1000
		}
		if win {
			out = append(out, a)
		}
	}
	return out
}

// predStance: la postura més probable de l'enemic (la més freqüent, les
// recents pesen més; sense historial, la seva millor postura).
func predStance(s sit) string {
	if len(s.enStances) == 0 {
		return s.enBest
	}
	cnt := map[string]int{}
	for k, st := range s.enStances {
		cnt[st] += 1 + k
	}
	best, bc := s.enBest, -1
	for _, st := range stances {
		if cnt[st] > bc {
			best, bc = st, cnt[st]
		}
	}
	return best
}

func predObj(s sit) string {
	cnt := map[string]int{}
	for _, o := range s.enObjs {
		cnt[o]++
	}
	best, bc := "", 0
	for _, o := range []string{"elder", "baron", "dragon", "herald", "towers", "farm"} {
		if cnt[o] > bc && s.has(o) {
			best, bc = o, cnt[o]
		}
	}
	return best
}

func chooseStance(s sit, pred string) string {
	st, sv := "", -1e9
	for _, a := range counters(s, pred) {
		if s.myFit[a] >= 0.85 && s.myFit[a] > sv {
			st, sv = a, s.myFit[a]
		}
	}
	if st != "" {
		return st
	}
	// Cap contra que encaixi: la millor postura que no perdi contra la seva.
	for _, a := range stances {
		loses := false
		for _, c := range counters(sit{splitOK: [2]bool{s.splitOK[1], s.splitOK[0]}, protOK: [2]bool{s.protOK[1], s.protOK[0]}, gold: -s.gold}, a) {
			if c == pred {
				loses = true
			}
		}
		if !loses && s.myFit[a] > sv {
			st, sv = a, s.myFit[a]
		}
	}
	if st == "" {
		st = pred
	}
	return st
}

func firstNeutral(s sit, skipBaron bool) string {
	for _, o := range []string{"elder", "baron", "dragon", "herald"} {
		if o == "baron" && skipBaron {
			continue
		}
		if s.has(o) {
			return o
		}
	}
	return ""
}

func starterMacro(s sit) json.RawMessage {
	pred := predStance(s)
	st := chooseStance(s, pred)
	countering := false
	for _, c := range counters(s, pred) {
		if c == st {
			countering = true
		}
	}
	ans := func(o string) json.RawMessage { return js(map[string]string{"objective": o, "stance": st}) }
	if s.myBuff {
		return ans("towers")
	}
	po := predObj(s)
	strong := s.power > -5 || (s.power > -12 && countering)
	switch {
	case strong:
		if isNeutral(po) {
			return ans(po)
		}
		if o := firstNeutral(s, s.power < 5); o != "" {
			return ans(o)
		}
		return ans("towers")
	case s.soulPoint && s.drakes[1] == 3 && s.power > -12 && s.has("dragon"):
		return ans("dragon") // denegar l'ànima
	case s.later > s.power+3 && s.myCurve == "scaling" && !isNeutral(po):
		// Escalo millor i ells no van a cap objectiu: farmejar.
		return ans("farm")
	case s.power > -12 && s.later <= s.power:
		// Ells escalen millor: forçar ara, però on no ens esperen.
		for _, o := range []string{"dragon", "herald"} {
			if s.has(o) && o != po {
				return ans(o)
			}
		}
	}
	// Per darrere: l'altra banda del mapa.
	return ans("towers")
}

func starter(m *match, side int, p arena.Prompt) json.RawMessage {
	switch {
	case strings.Contains(p.Schema, `"bans"`) || strings.Contains(p.Schema, `"picks"`):
		return starterDraft(m, side, p)
	case strings.Contains(p.Schema, `"jungle"`):
		return starterLaning(parseSit(p.Situation))
	case strings.Contains(p.Schema, `"objective":"dragon|grubs|farm"`):
		return starterObjective(parseSit(p.Situation))
	case strings.Contains(p.Schema, `"focus"`):
		return starterPlates(parseSit(p.Situation))
	}
	return starterMacro(parseSit(p.Situation))
}

// ---- exploiter: compta què ha fet el rival i ho castiga ----

func exploitMacro(s sit) json.RawMessage {
	pred := s.enBest
	if n := len(s.enStances); n > 0 {
		pred = s.enStances[n-1]
	}
	// La millor postura contra la prevista: encaix + enfrontament.
	st, sv := "", -1e9
	cs := counters(s, pred)
	for _, a := range stances {
		v := 0.12 * (s.myFit[a] - 1)
		for _, c := range cs {
			if c == a {
				v += 0.22 * clampf(s.myFit[a], 0.5, 1.2)
			}
		}
		if v > sv {
			st, sv = a, v
		}
	}
	// Avantatge esperat a la baralla: força + postures.
	edge := s.power/100 + sv - 0.12*(s.enFit[pred]-1)
	po := ""
	if n := len(s.enObjs); n > 0 {
		po = s.enObjs[n-1]
	}
	ans := func(o string) json.RawMessage { return js(map[string]string{"objective": o, "stance": st}) }
	if s.myBuff {
		return ans("towers")
	}
	if edge > 0.03 {
		if isNeutral(po) && s.has(po) {
			return ans(po)
		}
		if o := firstNeutral(s, edge < 0.1); o != "" {
			return ans(o)
		}
		return ans("towers")
	}
	if s.soulPoint && s.drakes[1] == 3 && edge > -0.1 && s.has("dragon") {
		return ans("dragon")
	}
	if isNeutral(po) {
		return ans("towers")
	}
	for _, o := range []string{"dragon", "herald"} {
		if s.has(o) && o != po {
			return ans(o)
		}
	}
	if s.later > s.power {
		return ans("farm")
	}
	return ans("towers")
}

func exploiter(m *match, side int, p arena.Prompt) json.RawMessage {
	if strings.Contains(p.Schema, `"stance"`) {
		return exploitMacro(parseSit(p.Situation))
	}
	return starter(m, side, p)
}

// ---- polítiques constants ----

// prefList: una llista fixa de campions per a una postura (els que més hi
// sumen sols), com un .md que diu "tria sempre aquests".
func prefList(st string) []string {
	ids := make([]int, len(pool))
	for i := range ids {
		ids[i] = i
	}
	sort.SliceStable(ids, func(a, b int) bool {
		return stanceRaw([]*Champ{&pool[ids[a]]}, st) > stanceRaw([]*Champ{&pool[ids[b]]}, st)
	})
	out := []string{}
	for _, id := range ids {
		out = append(out, pool[id].Name)
	}
	return out
}

// constant: draft amb llista fixa, línies fixes, postura i objectiu fixos.
// obj: "contest" (el primer neutral disponible), "towers" o "farm".
func constant(st, obj, lanes string) policy {
	list := prefList(st)
	return func(m *match, side int, p arena.Prompt) json.RawMessage {
		switch {
		case strings.Contains(p.Schema, `"bans"`):
			// Veta el que més li faria mal: els millors de la mateixa llista.
			return js(map[string]any{"bans": list[len(list)-8:]})
		case strings.Contains(p.Schema, `"picks"`):
			return js(map[string]any{"picks": list})
		case strings.Contains(p.Schema, `"jungle"`):
			return js(map[string]string{"top": lanes, "mid": lanes, "bot": lanes, "jungle": "gank_bot"})
		case strings.Contains(p.Schema, `"objective":"dragon|grubs|farm"`):
			if obj == "contest" {
				return js(map[string]string{"objective": "dragon"})
			}
			return js(map[string]string{"objective": "farm"})
		case strings.Contains(p.Schema, `"focus"`):
			return js(map[string]string{"focus": "bot", "action": "plates"})
		}
		o := obj
		if obj == "contest" {
			o = "towers"
			s := parseSit(p.Situation)
			if n := firstNeutral(s, false); n != "" {
				o = n
			}
		}
		return js(map[string]string{"objective": o, "stance": st})
	}
}

func defaults(m *match, side int, p arena.Prompt) json.RawMessage { return nil }

type named struct {
	name string
	p    policy
}

func constantPolicies() []named {
	out := []named{{"no-answer", defaults}}
	for _, st := range stances {
		for _, o := range []string{"contest", "towers", "farm"} {
			lanes := "even"
			if st == "engage" || st == "pick" {
				lanes = "aggressive"
			}
			out = append(out, named{fmt.Sprintf("always %s+%s", st, o), constant(st, o, lanes)})
		}
	}
	return out
}

// ---- variants del starter (per mesurar què val cada decisió) ----

// starterWith: el starter amb una altra fase macro.
func starterWith(macro func(s sit) json.RawMessage) policy {
	return func(m *match, side int, p arena.Prompt) json.RawMessage {
		if strings.Contains(p.Schema, `"stance"`) {
			return macro(parseSit(p.Situation))
		}
		return starter(m, side, p)
	}
}

// alwaysContestMacro: postures del starter, però sempre disputa un neutral,
// encara que vagi clarament per darrere.
func alwaysContestMacro(s sit) json.RawMessage {
	o := firstNeutral(s, false)
	if o == "" {
		o = "towers"
	}
	return js(map[string]string{"objective": o, "stance": chooseStance(s, predStance(s))})
}

// noCounterMacro: objectius del starter, però sempre la seva millor postura.
func noCounterMacro(s sit) json.RawMessage {
	var a map[string]string
	_ = json.Unmarshal(starterMacro(s), &a)
	a["stance"] = bestStance(s.myFit)
	return js(a)
}

// ---- composicions fixes: early contra scaling ----

var (
	earlyComp   = []string{"Rageborn", "Granitebeast", "Pyrelord", "Quickdraw", "Ironhorn"}
	scalingComp = []string{"Thornmantle", "Voidstalker", "Starweaver", "Ironsights", "Dawnkeeper"}
	neutralBans = []string{"Ironwall", "Frostbulwark", "Stormhoof", "Boulderheart", "Chainwarden", "Emberlance", "Longshot", "Sunspear"}
)

// forceMacro: disputa cada objectiu (amb la postura del starter).
func forceMacro(s sit) json.RawMessage { return alwaysContestMacro(s) }

// scaleMacro: fins al 26:00 no es baralla (l'altra banda del mapa o farm);
// després, el starter.
func scaleMacro(s sit) json.RawMessage {
	if s.min >= 26 {
		return starterMacro(s)
	}
	o := "farm"
	if isNeutral(predObj(s)) {
		o = "towers"
	}
	return js(map[string]string{"objective": o, "stance": chooseStance(s, predStance(s))})
}

func fixedComp(list []string, macro func(s sit) json.RawMessage) policy {
	return func(m *match, side int, p arena.Prompt) json.RawMessage {
		switch {
		case strings.Contains(p.Schema, `"bans"`):
			return js(map[string]any{"bans": neutralBans})
		case strings.Contains(p.Schema, `"picks"`):
			return js(map[string]any{"picks": list})
		case strings.Contains(p.Schema, `"stance"`):
			return macro(parseSit(p.Situation))
		}
		return starter(m, side, p)
	}
}

// beginner: el .md del principiant de la prova en directe: un draft decent
// (el model tria bé els campions), però "juga agressiu i fes engage a cada
// objectiu".
func beginner(m *match, side int, p arena.Prompt) json.RawMessage {
	switch {
	case strings.Contains(p.Schema, `"bans"`) || strings.Contains(p.Schema, `"picks"`):
		return starterDraft(m, side, p)
	case strings.Contains(p.Schema, `"jungle"`):
		return js(map[string]string{"top": "aggressive", "mid": "aggressive", "bot": "aggressive", "jungle": "invade"})
	case strings.Contains(p.Schema, `"objective":"dragon|grubs|farm"`):
		return js(map[string]string{"objective": "dragon"})
	case strings.Contains(p.Schema, `"focus"`):
		return js(map[string]string{"focus": "bot", "action": "dive"})
	}
	o := firstNeutral(parseSit(p.Situation), false)
	if o == "" {
		o = "towers"
	}
	return js(map[string]string{"objective": o, "stance": "engage"})
}
