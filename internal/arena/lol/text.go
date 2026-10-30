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
	return fmt.Sprintf("engage %.0f, peel %.0f, poke %.0f, waveclear %.0f, splitpush %.0f, pick %.0f, objective %.0f, frontline %.0f, dps %.0f | curve early %d/mid %d/late %d | damage AD %d/AP %d",
		t.E, t.P, t.Po, t.W, t.S, t.Pi, t.O, t.F, t.D, e, mi, l, len(cs)-ap, ap)
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

func (m *match) gameSituation(i int) string {
	j := 1 - i
	var b strings.Builder
	fmt.Fprintf(&b, "LEAGUE OF LEGENDS — game time %s. You are %q (%s side) vs %q (%s).\n", fmtTime(m.t), m.names[i], m.sideName(i), m.names[j], m.sideName(j))
	tw := m.hud()["tw"].([2]int)
	fmt.Fprintf(&b, "Kills %d-%d | Gold %s vs %s (you %s) | Towers destroyed: you %d, enemy %d | Inhibitors destroyed: you %d, enemy %d\n",
		m.kills[i], m.kills[j], kg(m.gold[i]), kg(m.gold[j]), signed(m.gold[i]-m.gold[j]), tw[i], tw[j], m.inhibsLost[j], m.inhibsLost[i])
	if m.stage > stLaning {
		fmt.Fprintf(&b, "Lane gold leads (you): top %s, mid %s, bot %s\n", signed(m.laneAdv(i, 0)), signed(m.laneAdv(i, 1)), signed(m.laneAdv(i, 2)))
		down := []string{}
		for l := 0; l < 3; l++ {
			down = append(down, fmt.Sprintf("%s %d/3", roleLane[l], m.towersLost[j][l]))
		}
		fmt.Fprintf(&b, "Enemy towers lost per lane: %s; enemy nexus towers lost %d. ", strings.Join(down, ", "), m.nexusTowersLost[j])
		down = down[:0]
		for l := 0; l < 3; l++ {
			down = append(down, fmt.Sprintf("%s %d/3", roleLane[l], m.towersLost[i][l]))
		}
		fmt.Fprintf(&b, "Your towers lost: %s; nexus towers %d.\n", strings.Join(down, ", "), m.nexusTowersLost[i])
	}
	soulNote := ""
	if !m.soul[0] && !m.soul[1] {
		soulNote = fmt.Sprintf(" Next drake: %s at %s. Soul (4 drakes) type: %s.", m.nextDragonType(i), fmtTime(m.nextDragon), m.soulType)
	} else if m.soul[i] {
		soulNote = " YOU have the soul. Elder dragon: " + m.elderInfo()
	} else {
		soulNote = " ENEMY has the soul. Elder dragon: " + m.elderInfo()
	}
	fmt.Fprintf(&b, "Dragons: you %d %v, enemy %d %v.%s\n", len(m.dragons[i]), m.dragons[i], len(m.dragons[j]), m.dragons[j], soulNote)
	herald := "not yet (spawns 14:00)"
	if m.heraldDone {
		herald = "taken"
	} else if m.t >= 1185 {
		herald = "despawned"
	} else if m.t >= 840 {
		herald = "available until 19:45"
	}
	baron := "spawns " + fmtTime(m.baronNext)
	if m.baronNext <= m.t {
		baron = "UP"
	}
	fmt.Fprintf(&b, "Void grubs: you %d, enemy %d. Herald: %s (you %d, enemy %d). Baron: %s (you %d, enemy %d).",
		m.grubs[i], m.grubs[j], herald, m.herald[i], m.herald[j], baron, m.barons[i], m.barons[j])
	for _, t := range []int{i, j} {
		if m.baronUntil[t] > m.t {
			fmt.Fprintf(&b, " %s has Baron buff until %s.", m.who(i, t), fmtTime(m.baronUntil[t]))
		}
		if m.elderUntil[t] > m.t {
			fmt.Fprintf(&b, " %s has Elder buff until %s.", m.who(i, t), fmtTime(m.elderUntil[t]))
		}
	}
	b.WriteString("\n")
	for _, t := range []int{i, j} {
		fmt.Fprintf(&b, "%s champions: ", m.who(i, t))
		s := []string{}
		for r := 0; r < 5; r++ {
			c := m.line[t][r]
			s = append(s, fmt.Sprintf("%s %s (%s %s, %s) %d/%d/%d %s", roleNames[r], c.Name, c.Class, c.Dmg, c.Curve, m.k[t][r], m.d[t][r], m.a[t][r], kg(m.pgold[t][r])))
		}
		b.WriteString(strings.Join(s, "; ") + "\n")
		fmt.Fprintf(&b, "%s comp: %s\n", m.who(i, t), compSummary(m.line[t][:]))
		fmt.Fprintf(&b, "%s stance fit (1.00 = average comp): %s\n", m.who(i, t), fitStr(m.fit[t]))
	}
	fmt.Fprintf(&b, "Team power multiplier by curve now/+8min: you %.2f→%.2f, enemy %.2f→%.2f\n",
		m.teamCurve(i, m.t/60), m.teamCurve(i, m.t/60+8), m.teamCurve(j, m.t/60), m.teamCurve(j, m.t/60+8))
	fmt.Fprintf(&b, "ENEMY history (oldest→newest): %s\n", lastN(m.hist[j], 8))
	fmt.Fprintf(&b, "Your history: %s\n", lastN(m.hist[i], 5))
	if len(m.stHist[j]) > 0 {
		fmt.Fprintf(&b, "Enemy stances so far: %s\n", strings.Join(m.stHist[j], ", "))
	}
	return b.String()
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
Priority list of at least %d champion names (add 2-3 backups). You need %d bans: they are applied in pro order alternating with the enemy, who bans at the same time; if one of yours is already gone, the next on your list is used. Invalid/missing → first available champion in pool order.`

const schemaPick = `{"picks": ["ChampionName", "..."]}
Exactly %d champion(s) from the available list, best first; extra names after them are backups used only if one is invalid. Every champion must fill a different open role of yours (roles are assigned automatically; flex champions may play any listed role). Invalid → first legal champion in pool order.`

const schemaLaning = `{"top":"aggressive|even|passive","mid":"aggressive|even|passive","bot":"aggressive|even|passive","jungle":"gank_top|gank_mid|gank_bot|farm|invade"}
Lanes (00:00-08:00): aggressive = trade hard, wins more gold if your lane is stronger, but enemy ganks succeed 62%. even = normal (ganks 38%). passive = give up some farm, ganks only 12%, halves the loss in a losing lane.
Jungle: gank_X = gank enemy lane X (works against aggressive/even laners; if both junglers go to the same lane they skirmish there). farm = +300 gold, safe. invade = steal camps (+350 / -250 enemy) unless they invade too (then skirmish); risky if their jungler is stronger early.`

const schemaObjectives = `{"objective":"dragon|grubs|farm"}
08:00-12:00. dragon = fight for the first drake (bot side of the map). grubs = take the 3 void grubs (top side; they help you take towers/plates). farm = +450 gold, no objective.
Both teams choose the same objective = skirmish there; the stronger early team (gold, early-curve champions, lane lead on that side) usually wins it. Different choices = each takes its own uncontested.`

const schemaPlates = `{"focus":"top|mid|bot","action":"plates|dive|roam|farm"}
12:00-14:00. plates = take turret plate gold in the focus lane (more if you lead that lane or have grubs; may take the outer tower). dive = tower-dive the focus lane for kills + outer tower (needs a lane lead or a stronger early comp; fails into a roam). roam = mid+jungle protect YOUR focus lane: crushes an enemy plates/dive there, otherwise wasted (-200 gold). farm = +350 gold, safe.`

func macroSchema(opts []string) string {
	return fmt.Sprintf(`{"objective":"%s","stance":"engage|siege|pick|split|protect"}
Next 4 minutes. objective (only these are available now): dragon/herald/baron/elder = take that neutral objective; if both teams choose the same one there is a 5v5 teamfight at the pit (winner takes it, small steal chance for the loser). Starting baron/elder while weaker invites a punishing collapse. towers = siege/push structures. farm = scale up (+gold, more if your comp is late-game) and cede the map. Unchosen dragons/heralds may be taken by whoever controls the map.
stance = how your team fights this window: engage (dive/hard engage), siege (poke + waveclear around towers), pick (hunt isolated targets), split (1-3-1 side-lane pressure, takes side towers), protect (front-to-back, peel the carries).
Counters: engage beats siege & pick; siege beats split & protect; pick beats siege & split; split beats engage & protect; protect beats engage & pick. A stance only works as well as your stance fit (shown in the situation).`, strings.Join(opts, "|"))
}

const rulesText = `LEAGUE OF LEGENDS ARENA — Summoner's Rift 5v5. Both teams have identical players; only your strategy differs. The same model reads your strategy and the situation and must answer ONLY the JSON in the schema.

DRAFT (pro order, blue side by coin flip): bans B-R-B-R-B-R (both teams submit a ban priority list at once) → picks B1, R1+R2, B2+B3, R3 → bans R-B-R-B → picks R4, B4+B5, R5. When it is not your turn you get no prompt. Each team needs top, jungle, mid, adc, support (roles auto-assigned; flex champions fit any listed role).
Champions have class, damage type (AD/AP), power curve (early/mid/late) and traits 0-3: engage, peel, poke, waveclear, splitpush, pick, objective, frontline, dps, lane.
What wins: a COHERENT composition (stance fit), frontline (<3 total frontline is a big penalty), mixed damage (4-5 of one type is penalised), assassins/pick threats punish squishy teams without peel+frontline, engage+dps, peel+late carries, poke+waveclear synergies. Early champions win lanes and fights before ~20 min, late champions dominate after ~30.

GAME (decisions at 00:00 laning, 08:00 first objectives, 12:00 plates, then every 4 minutes until a Nexus falls): both teams decide simultaneously without seeing the other's current choice, but you DO see the enemy's full history of choices. Predict them.
Fight strength = power curve now × composition × gold lead (≈3% per 1k) × dragons/soul/baron/elder buffs × stance fit × stance matchup, plus randomness.
Stances: engage beats siege & pick; siege beats split & protect; pick beats siege & split; split beats engage & protect; protect beats engage & pick. Countering the enemy's likely stance is worth ~20% fight strength, but only if your comp fits that stance.
Objectives: drakes (4 = soul, then Elder: +25% and game-ending), void grubs/herald (tower pressure), Baron (from 20:00: +14% and big pushes for 3 min). Same objective = teamfight there; different = trade. Big fight wins after 18:00 turn into towers, inhibitors and the Nexus. Behind teams get extra gold for kills/towers, so comebacks happen.
Invalid or missing answers fall back to a safe default.`
