package lol

import (
	"fmt"
	"strings"

	"gregal/internal/arena"
)

func (m *match) prompt(i int) arena.Prompt {
	if m.stage < len(draftPlan) {
		st := draftPlan[m.stage]
		if !st.ban && m.teamOf(st.sides[0]) != i {
			return arena.Prompt{
				Situation: fmt.Sprintf("Draft %s: the enemy is picking. No decision for you now.", st.label),
				Schema:    "",
				Short:     "waiting (" + st.label + ")",
			}
		}
		return m.draftPrompt(i, st)
	}
	sit := m.gameSituation(i)
	switch m.stage {
	case stLaning:
		return arena.Prompt{Situation: sit, Schema: schemaLaning, Short: "00:00 laning plan"}
	case stObjectives:
		return arena.Prompt{Situation: sit, Schema: schemaObjectives, Short: "08:00 first objectives"}
	case stPlates:
		return arena.Prompt{Situation: sit, Schema: schemaPlates, Short: "12:00 plates & roams"}
	}
	opts := m.objOptions()
	return arena.Prompt{Situation: sit, Schema: macroSchema(opts), Short: fmt.Sprintf("%s macro (%s)", fmtTime(m.t), strings.Join(opts[:len(opts)-2], ", "))}
}

func (m *match) draftPrompt(i int, st dstep) arena.Prompt {
	j := 1 - i
	var b strings.Builder
	fmt.Fprintf(&b, "LEAGUE OF LEGENDS — DRAFT. You are %q on %s side vs %q (%s).\n", m.names[i], m.sideName(i), m.names[j], m.sideName(j))
	b.WriteString("Order: bans B-R-B-R-B-R | picks B1, R1+R2, B2+B3, R3 | bans R-B-R-B | picks R4, B4+B5, R5.\n")
	if st.ban {
		fmt.Fprintf(&b, "NOW: %s — you ban %d (the enemy bans at the same time, alternating).\n", st.label, len(st.sides)/2)
	} else {
		fmt.Fprintf(&b, "NOW: your pick %s — choose %d champion(s).\n", st.label, len(st.sides))
	}
	fmt.Fprintf(&b, "Bans — you: %s; enemy: %s\n", m.nameList(m.bans[i]), m.nameList(m.bans[j]))
	for _, t := range []int{i, j} {
		who := "Your"
		if t == j {
			who = "Enemy"
		}
		fmt.Fprintf(&b, "%s picks: %s | open roles: %s\n", who, m.pickList(t), strings.Join(m.openRoles(t), ", "))
		if len(m.picks[t]) > 0 {
			cs := []*Champ{}
			for _, id := range m.picks[t] {
				cs = append(cs, &pool[id])
			}
			fmt.Fprintf(&b, "%s comp so far: %s\n", who, compSummary(cs))
			k := float64(len(cs))
			fs := []string{}
			for _, s := range stances {
				fs = append(fs, fmt.Sprintf("%s %.2f", s, stanceRaw(cs, s)/(stanceAvg[s]*k/5)))
			}
			fmt.Fprintf(&b, "%s stance fit so far (scaled to 5 champions, 1.00 = average): %s\n", who, strings.Join(fs, ", "))
		}
	}
	b.WriteString("Available champions (name | roles | class | damage | power curve | traits 0-3):\n")
	for id := range pool {
		if m.taken[id] {
			continue
		}
		c := &pool[id]
		mark := ""
		if !st.ban && !m.canPick(i, id) {
			mark = " (no open role for you)"
		}
		fmt.Fprintf(&b, "- %s | %s | %s | %s | %s | %s%s\n", c.Name, strings.Join(c.Roles, "/"), c.Class, c.Dmg, c.Curve, traitStr(c), mark)
	}
	if st.ban {
		n := len(st.sides) / 2
		return arena.Prompt{Situation: b.String(), Schema: fmt.Sprintf(schemaBan, n, n), Short: st.label}
	}
	return arena.Prompt{Situation: b.String(), Schema: fmt.Sprintf(schemaPick, len(st.sides)), Short: "pick " + st.label}
}

func (m *match) nameList(ids []int) string {
	if len(ids) == 0 {
		return "none"
	}
	s := []string{}
	for _, id := range ids {
		s = append(s, pool[id].Name)
	}
	return strings.Join(s, ", ")
}

func (m *match) pickList(t int) string {
	if len(m.picks[t]) == 0 {
		return "none"
	}
	roles := assignAny(m.picks[t])
	s := []string{}
	for k, id := range m.picks[t] {
		c := &pool[id]
		s = append(s, fmt.Sprintf("%s (%s, %s, %s, %s)", c.Name, roleNames[roles[k]], c.Class, c.Dmg, c.Curve))
	}
	return strings.Join(s, "; ")
}

func (m *match) openRoles(t int) []string {
	used := [5]bool{}
	for _, r := range assignAny(m.picks[t]) {
		used[r] = true
	}
	out := []string{}
	for r := 0; r < 5; r++ {
		if !used[r] {
			out = append(out, roleNames[r])
		}
	}
	if len(out) == 0 {
		return []string{"none"}
	}
	return out
}

func traitStr(c *Champ) string {
	parts := []string{}
	add := func(k string, v int) {
		if v > 0 {
			parts = append(parts, fmt.Sprintf("%s%d", k, v))
		}
	}
	add("engage", c.Engage)
	add("peel", c.Peel)
	add("poke", c.Poke)
	add("waveclear", c.Wave)
	add("splitpush", c.Split)
	add("pick", c.Pick)
	add("objective", c.Obj)
	add("frontline", c.Front)
	add("dps", c.DPS)
	add("lane", c.Lane)
	return strings.Join(parts, " ")
}

func compSummary(cs []*Champ) string {
	t := sumTraits(cs)
	e, mi, l := curveCounts(cs)
	ap := 0
	for _, c := range cs {
		if c.Dmg == "AP" {
			ap++
		}
	}
	return fmt.Sprintf("engage %.0f, peel %.0f, poke %.0f, waveclear %.0f, splitpush %.0f, pick %.0f, objective %.0f, frontline %.0f, dps %.0f, best splitpusher %.0f | curve early %d/mid %d/late %d | damage AD %d/AP %d",
		t.E, t.P, t.Po, t.W, t.S, t.Pi, t.O, t.F, t.D, t.MaxS, e, mi, l, len(cs)-ap, ap)
}

func fitStr(fit map[string]float64) string {
	s := []string{}
	for _, st := range stances {
		s = append(s, fmt.Sprintf("%s %.2f", st, fit[st]))
	}
	return strings.Join(s, ", ")
}

func kg(v float64) string { return fmt.Sprintf("%.1fk", v/1000) }

func signed(v float64) string {
	if v >= 0 {
		return fmt.Sprintf("+%d", round(v))
	}
	return fmt.Sprintf("%d", round(v))
}

// statusWord: com va l'equip segons la força (%).
func statusWord(pct float64) string {
	switch {
	case pct >= 12:
		return "CLEARLY AHEAD"
	case pct >= 5:
		return "AHEAD"
	case pct <= -12:
		return "CLEARLY BEHIND"
	case pct <= -5:
		return "BEHIND"
	}
	return "EVEN"
}

func pct(v float64) string {
	if v >= 0 {
		return fmt.Sprintf("+%.0f%%", v)
	}
	return fmt.Sprintf("%.0f%%", v)
}

func yesNo(b bool) string {
	if b {
		return "YES"
	}
	return "no"
}

// compLine: identitat d'una composició tal com la veu la Situation.
func (m *match) compLine(t int) string {
	cs := m.line[t][:]
	tr := sumTraits(cs)
	ap := 0
	for _, c := range cs {
		if c.Dmg == "AP" {
			ap++
		}
	}
	return fmt.Sprintf("%s, best stance %s | stance fit: %s | frontline %.0f, peel %.0f, engage %.0f, poke %.0f, waveclear %.0f, pick %.0f, splitpush %.0f, dps %.0f | damage AD %d/AP %d",
		strings.ToUpper(compCurve(cs)), bestStance(m.fit[t]), fitStr(m.fit[t]), tr.F, tr.P, tr.E, tr.Po, tr.W, tr.Pi, tr.S, tr.D, 5-ap, ap)
}

func (m *match) gameSituation(i int) string {
	j := 1 - i
	var b strings.Builder
	next := ""
	switch m.stage {
	case stLaning:
		next = "This decision: the laning plan for 00:00-08:00."
	case stObjectives:
		next = "This decision: the first objective for 08:00-12:00."
	case stPlates:
		next = "This decision: plates and roams for 12:00-14:00."
	default:
		next = fmt.Sprintf("This decision covers %s-%s.", fmtTime(m.t), fmtTime(m.t+240))
	}
	fmt.Fprintf(&b, "LEAGUE OF LEGENDS — game time %s. You are %q (%s side) vs %q (%s side). %s\n", fmtTime(m.t), m.names[i], m.sideName(i), m.names[j], m.sideName(j), next)
	fmt.Fprintf(&b, "SCORE: kills you %d - enemy %d | gold you %s vs enemy %s | GOLD DIFF %s | towers destroyed: you %d, enemy %d | inhibitors destroyed: you %d, enemy %d\n",
		m.kills[i], m.kills[j], kg(m.gold[i]), kg(m.gold[j]), signed(m.gold[i]-m.gold[j]), m.towersBy(i), m.towersBy(j), m.inhibsLost[j], m.inhibsLost[i])
	p := m.powerPct(i)
	// A +8 min amb el mateix or: només corbes, composició i bufs permanents.
	cNow := m.teamCurve(i, m.t/60) / m.teamCurve(j, m.t/60)
	cLater := m.teamCurve(i, m.t/60+8) / m.teamCurve(j, m.t/60+8)
	later := 100 * ((1+p/100)*cLater/cNow - 1)
	fmt.Fprintf(&b, "POWER NOW: you %s → %s (team-fight strength from gold, comps, power curves, drakes and buffs, before stances; ±5%% = EVEN, ±12%% = CLEARLY). In 8 min at the same gold: you %s (%s).\n",
		pct(p), statusWord(p), pct(later), statusWord(later))
	// Objectius.
	if m.stage >= stMacro {
		opts := m.objOptions()
		fmt.Fprintf(&b, "OBJECTIVES YOU CAN CHOOSE NOW: %s.", strings.Join(opts, ", "))
	} else {
		b.WriteString("OBJECTIVES:")
	}
	if !m.soul[0] && !m.soul[1] {
		n := len(m.dragons[0]) + len(m.dragons[1])
		fmt.Fprintf(&b, " Next drake: %s (drake #%d) at %s. Soul = first team to 4 drakes, soul type %s.", m.nextDragonType(i), n+1, fmtTime(m.nextDragon), m.soulType)
		if len(m.dragons[i]) == 3 || len(m.dragons[j]) == 3 {
			b.WriteString(" SOUL POINT: the next drake gives someone the soul!")
		}
	} else {
		owner := "YOU have"
		if m.soul[j] {
			owner = "ENEMY has"
		}
		fmt.Fprintf(&b, " %s the %s soul. Elder dragon: %s.", owner, m.soulType, m.elderInfo())
	}
	fmt.Fprintf(&b, " Dragons: you %d %v, enemy %d %v.\n", len(m.dragons[i]), m.dragons[i], len(m.dragons[j]), m.dragons[j])
	herald := "spawns 14:00"
	if m.heraldDone {
		herald = "taken"
	} else if m.t >= 1185 {
		herald = "despawned"
	} else if m.t >= 840 {
		herald = "UP until 19:45"
	}
	baron := "spawns " + fmtTime(m.baronNext)
	if m.baronNext <= m.t+150 {
		baron = "UP"
	}
	fmt.Fprintf(&b, "Void grubs: you %d, enemy %d. Herald: %s (you %d, enemy %d). Baron: %s (you %d, enemy %d).",
		m.grubs[i], m.grubs[j], herald, m.herald[i], m.herald[j], baron, m.barons[i], m.barons[j])
	for _, t := range []int{i, j} {
		if m.baronUntil[t] > m.t {
			fmt.Fprintf(&b, " %s has BARON BUFF until %s.", m.who(i, t), fmtTime(m.baronUntil[t]))
		}
		if m.elderUntil[t] > m.t {
			fmt.Fprintf(&b, " %s has ELDER BUFF until %s.", m.who(i, t), fmtTime(m.elderUntil[t]))
		}
	}
	b.WriteString("\n")
	if m.stage > stLaning {
		fmt.Fprintf(&b, "Lane gold leads (you): top %s, mid %s, bot %s. ", signed(m.laneAdv(i, 0)), signed(m.laneAdv(i, 1)), signed(m.laneAdv(i, 2)))
		down := []string{}
		for l := 0; l < 3; l++ {
			down = append(down, fmt.Sprintf("%s %d/3", roleLane[l], m.towersLost[j][l]))
		}
		fmt.Fprintf(&b, "Enemy towers lost: %s, nexus %d/2. ", strings.Join(down, ", "), m.nexusTowersLost[j])
		down = down[:0]
		for l := 0; l < 3; l++ {
			down = append(down, fmt.Sprintf("%s %d/3", roleLane[l], m.towersLost[i][l]))
		}
		fmt.Fprintf(&b, "Your towers lost: %s, nexus %d/2.\n", strings.Join(down, ", "), m.nexusTowersLost[i])
	}
	// Composicions.
	for _, t := range []int{i, j} {
		up := "YOUR"
		if t == j {
			up = "ENEMY"
		}
		fmt.Fprintf(&b, "%s COMP: %s\n", up, m.compLine(t))
		s := []string{}
		for r := 0; r < 5; r++ {
			c := m.line[t][r]
			s = append(s, fmt.Sprintf("%s %s (%s %s %s, lane %d) %d/%d/%d %s", roleNames[r], c.Name, c.Class, c.Dmg, c.Curve, c.Lane, m.k[t][r], m.d[t][r], m.a[t][r], kg(m.pgold[t][r])))
		}
		fmt.Fprintf(&b, "%s champions: %s\n", m.who(i, t), strings.Join(s, "; "))
	}
	if m.stage == stLaning {
		b.WriteString("LANE MATCHUPS (lane strength = lane trait, +0.6 early / -0.5 late champion; bot = adc+support average):")
		for l := 0; l < 3; l++ {
			d := m.laneStr(i, l) - m.laneStr(j, l)
			fmt.Fprintf(&b, " %s you %.1f vs enemy %.1f (%s);", roleLane[l], m.laneStr(i, l), m.laneStr(j, l), laneWord(d))
		}
		ji, jj := laneScore(m.line[i][1]), laneScore(m.line[j][1])
		fmt.Fprintf(&b, " jungle you %.1f vs enemy %.1f (%s).\n", ji, jj, laneWord(ji-jj))
	}
	if m.stage >= stMacro {
		fmt.Fprintf(&b, "CONDITIONAL COUNTERS NOW: protect beats engage — you %s, enemy %s | split beats engage — you %s, enemy %s | where it is \"no\", engage beats that stance instead unless the engaging team is >1k gold behind.\n",
			yesNo(m.protectOK(i)), yesNo(m.protectOK(j)), yesNo(m.splitOK(i)), yesNo(m.splitOK(j)))
	}
	// Historial.
	if len(m.log) == 0 {
		b.WriteString("HISTORY: none yet.\n")
	} else {
		b.WriteString("HISTORY (both teams' choices and how each window went for YOU, oldest first):\n")
		from := 0
		if len(m.log) > 8 {
			from = len(m.log) - 8
		}
		for _, l := range m.log[from:] {
			fmt.Fprintf(&b, "- %s you: %s | enemy: %s → %s\n", l.t, l.ch[i], l.ch[j], l.res[i])
		}
	}
	if len(m.stHist[j]) > 0 {
		fmt.Fprintf(&b, "ENEMY STANCES SO FAR: %s\n", strings.Join(m.stHist[j], ", "))
		fmt.Fprintf(&b, "ENEMY OBJECTIVES SO FAR: %s\n", strings.Join(m.objHist[j][1:], ", "))
		fmt.Fprintf(&b, "YOUR STANCES SO FAR: %s\n", strings.Join(m.stHist[i], ", "))
	}
	return b.String()
}

func laneWord(d float64) string {
	switch {
	case d >= 0.5:
		return fmt.Sprintf("you +%.1f, STRONGER", d)
	case d <= -0.5:
		return fmt.Sprintf("you %.1f, WEAKER", d)
	}
	return fmt.Sprintf("you %+.1f, even", d)
}

func (m *match) elderInfo() string {
	if m.elderNext <= m.t {
		return "UP"
	}
	return "spawns " + fmtTime(m.elderNext)
}

func (m *match) who(me, t int) string {
	if me == t {
		return "Your"
	}
	return "Enemy"
}

func lastN(h []string, n int) string {
	if len(h) == 0 {
		return "none yet"
	}
	if len(h) > n {
		h = h[len(h)-n:]
	}
	return strings.Join(h, " | ")
}

const schemaBan = `{"bans": ["ChampionName", "..."]}
Priority list of at least %d champion names (add 2-3 backups). You need %d bans: they are applied in pro order alternating with the enemy, who bans at the same time; if one of yours is already gone, the next on your list is used. Invalid/missing → a random available champion.`

const schemaPick = `{"picks": ["ChampionName", "..."]}
Exactly %d champion(s) from the available list, best first; extra names after them are backups used only if one is invalid. Every champion must fill a different open role of yours (roles are assigned automatically; flex champions may play any listed role). Invalid/missing → a random legal champion (same rule for both teams).`

const schemaLaning = `{"top":"aggressive|even|passive","mid":"aggressive|even|passive","bot":"aggressive|even|passive","jungle":"gank_top|gank_mid|gank_bot|farm|invade"}
Lanes (00:00-08:00), see LANE MATCHUPS: the stronger lane earns gold (≈150 per point of lane strength, +50% if either side is aggressive). aggressive = +100 gold if you are not the weaker lane, -200 if you are, more solo kills, and enemy ganks succeed 62%. even = ganks 38%. passive = ganks only 12% and the gold you lose in a weaker lane is almost halved.
Jungle: gank_X = gank enemy lane X (works against aggressive/even laners; if both junglers go to the same lane they skirmish there; a failed gank costs gold). farm = +300 gold, safe. invade = steal camps (+350 you / -250 them) unless they invade too (then a skirmish); their jungler may kill yours if it is much stronger. Missing → even lanes, farm.`

const schemaObjectives = `{"objective":"dragon|grubs|farm"}
08:00-12:00. dragon = first drake (bot side). grubs = 3 void grubs (top side; +gold, they boost your 12:00 plates). farm = +450 gold, no objective.
Same objective = skirmish there, won by the stronger early team (POWER NOW, lane lead on that side). Different choices = each takes its own uncontested. Missing → dragon.`

const schemaPlates = `{"focus":"top|mid|bot","action":"plates|dive|roam|farm"}
12:00-14:00. plates = turret plate gold in the focus lane (more with a lane lead or grubs; may take the outer tower). dive = tower-dive the focus lane for kills + the outer tower (needs a lane lead > 500 or POWER AHEAD; otherwise you lose kills). roam = mid+jungle protect YOUR focus lane: crushes an enemy plates/dive there, otherwise wasted (-200 gold). farm = +350 gold, safe. Missing → farm mid.`

func macroSchema(opts []string) string {
	return fmt.Sprintf(`{"objective":"%s","stance":"engage|siege|pick|split|protect"}
Next 4 minutes. objective (only these are available now):
- dragon/herald/baron/elder = take it. Both teams on the same one = 5v5 teamfight at the pit (the stronger team's power edge counts 1.5x there; the winner takes it, rare steals). If only you go, the enemy may collapse on you: likely if they are stronger or on engage/pick, unlikely if they farm. Baron/elder are slow: even a team that just took another objective can collapse on them.
- towers = push structures on the side of the map the enemy is not on: 1, +1 if the enemy is at a neutral objective, +1 with siege/split, +1 with baron buff, +1 with herald before 25:00. Both teams on towers = fight in mid lane.
- farm = +650 gold (more if your comp is still growing, +300 if SCALING), fewer fights; you cede the map.
stance = how you fight this window. Counters: engage beats siege & pick; siege beats split & protect; pick beats siege & split; split beats protect; protect beats pick. Engage vs split / protect is CONDITIONAL (see CONDITIONAL COUNTERS NOW in the situation): protect beats engage only if that team qualifies (protect fit ≥ 1.00, better scaling or +2 frontline), split beats engage only with a real splitpusher (split fit ≥ 1.10) and not >1.5k gold behind; otherwise engage beats them unless the engaging team is >1k gold behind.
Winning the stance matchup ≈ +22%% fight strength × your fit (0.5-1.2); fit also adds 12%% × (fit-1). Missing → your best-fit stance and the first neutral objective (baron only from 24:00), else towers.`, strings.Join(opts, "|"))
}

const rulesText = `LEAGUE OF LEGENDS ARENA — Summoner's Rift 5v5. Both teams have identical players; only the strategy (.md) differs. The same model reads your strategy and the situation and answers ONLY the JSON in the schema. You win by destroying the enemy Nexus; kills are only the TV scoreboard (the final score is 1-0 for the Nexus winner).

DRAFT (blue side by coin flip): bans B-R-B-R-B-R (both send a ban list at once) → picks B1, R1+R2, B2+B3, R3 → bans R-B-R-B → picks R4, B4+B5, R5. Roles top/jungle/mid/adc/support are auto-assigned. Invalid answers get a random legal champion.
Champions: class, AD/AP, power curve, traits 0-3 (engage, peel, poke, waveclear, splitpush, pick, objective, frontline, dps, lane).
Composition (permanent fight strength): total frontline <3 −8%, <5 −3%; damage all-AD/all-AP −9%, 4-1 −3.5%; pick threat vs a squishy enemy up to ±5% (1.2% × (your pick − 0.6 × their peel+frontline)); +3% each: engage≥5 & dps≥9, peel≥5 & a late dps3 carry, poke≥7 & waveclear≥9.
Power curves (×strength): early 1.12 at 0' → 1.10 at 14' → 0.99 at 22' → 0.91 at 30' → 0.86 at 40'; mid 0.98 → 1.04 → 1.08 → 1.00 → 0.95; late 0.90 → 0.92 → 0.99 → 1.10 → 1.17. Comps are EARLY (must win before ~25'), SCALING (must survive to 30') or BALANCED.
Stance fit (1.00 = average comp): engage = 2·engage+frontline+0.5·dps (avg 19.1); siege = 2·poke+waveclear+0.3·dps (19.3); pick = 2·pick+0.7·engage+0.3·poke (15.0); split = 3·best splitpusher+splitpush+0.5·waveclear (13.7); protect = 2·peel+dps+0.3·frontline (17.5).

GAME: decisions at 00:00 (laning), 08:00 (first objective), 12:00 (plates), then every 4 minutes until a Nexus falls. Both decide at once; you see the enemy's earlier choices and how each window went (HISTORY), so predict and counter them.
Fight strength = curve × comp × gold (+2.5% per 1k lead) × drakes (+1% each, soul +6%) × baron buff (+8%, 3 min) × elder (+20%, 2.5 min) × stance fit and stance matchup, plus dice in every fight. POWER NOW in the situation is this strength before stances (±5% = EVEN). At objective pits the power difference counts 1.5x.
Stances off the main fight: split takes side towers (none vs siege; the splitter gets caught by pick, and by engage when engage beats split); pick catches isolated enemies vs siege/split but gets caught by engage; siege takes mid towers vs protect/split but gets dived by engage/pick; engage makes big fights more likely; protect only fights.
Objectives: 4 drakes = soul, then Elder; Baron from 20:00; herald/grubs help towers; a drake or herald nobody picks may go to the stronger team. Outer towers only before 14:00, inner before 19:00. Big fight wins after 18:00 turn into towers/inhibitors; losing teams get bounty gold, so comebacks happen. Contesting while clearly behind usually loses the fight AND the objective: trade with towers instead.`
