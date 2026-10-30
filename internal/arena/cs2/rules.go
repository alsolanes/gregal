package cs2

import (
	"fmt"
	"strings"
)

// rulesText: les regles que llegeix el model. La taula de rondes la genera
// matchupWin, la mateixa que calibra el motor.
var rulesText = rulesHead + matchupTable() + rulesTail

const rulesHead = `COUNTER-STRIKE 2 — ARENA RULES (map: Mirage)
Two teams of 5 identical players; only the decisions differ.
FORMAT: MR12. First to 13 rounds wins. Sides swap after round 12. At 12-12, overtime: blocks of 6 rounds (first to 4 in the block), $10,000 each at every OT half, sides swap every 3 rounds. The starting side is random and both sides are equally strong over a match.
SIDES: T attacks and must plant the bomb at site A or B (it explodes 40s after the plant). CT wins by killing all Ts, by the clock (1:55) running out before a plant, or by defusing (5s with a kit, 10s without).
ECONOMY (per player): $800 at the start of each half. Win: $3,250 ($3,500 by bomb explosion / defuse). Loss bonus $1,400/1,900/2,400/2,900/3,400: one step up per loss, one step down per win (each half starts at $1,900). Losing Ts get +$800 each if they planted; Ts alive when time runs out get no loss money. Kill rewards: rifles and pistols $300, SMGs $600, AWP $100. Max $16,000. Survivors keep their weapons and armor; the dead lose everything.
PRICES: AK-47 $2,700, M4A1-S $2,900, Galil $1,800, FAMAS $2,050, MAC-10 $1,050, MP9 $1,250, AWP $4,750, kevlar $650, kevlar+helmet $1,000, defuse kit $400, smoke $300, flash $200, molotov $400 (CT $500), HE $300. A full buy from nothing costs about $21,000 (T) / $23,000 (CT) for the team.
DECISIONS: one per round per side, taken by both teams at the same time before the round. Answer one JSON object following the schema. Missing or invalid fields fall back to defaults: buy "full" if affordable else "eco"; T "default_mid", utility "smokes", pace "default"; CT "standard_2_1_2", utility "normal"; "awp" true.
HOW A ROUND IS DECIDED: the round is simulated as movements and duels. Each duel depends on weapons and armor, numbers, who holds the angle, utility, and on how the T plan matches the CT setup. Between equal buys the matchup is the biggest factor:
`

const rulesTail = `How to read it: stack the site they hit; hit the site they do not stack (default_mid finds the weak site by itself). aggressive_mid wins early mid duels and beats default_mid, splits and executes, but a rush walks into its empty sites. retake_x gives site x away: executes and default_mid into x win, rushes into x lose to the retake, and the 3 CTs on the other site hold it. Fakes beat standard rotations and a stack on the faked site. A rush into a standard setup loses. standard_2_1_2 is the safest CT setup (it never loses badly) but fakes and splits beat it.
EQUIPMENT (round win vs a full buy with the same plans, as T / as CT): eco under 2%; half (~$3,000 each) ~1% / ~4%; force with ~$2,500 each (Galil/FAMAS or SMG + armor) ~10% / ~20%; force with ~$3,400 each (T: AK + kevlar, no helmet or utility; CT: FAMAS + helmet) ~20% / ~30%; force when rich (rifles, helmets and utility but no AWP or kit) ~41% / ~47%; full ~47% / ~53%. Rifles beat SMGs beat pistols; no armor or no helmet makes you easier to kill. The AWP is the best gun in long, unsmoked duels (mid fights, CT holds) and a little better than a rifle up close: it adds ~3-7 points.
UTILITY: T utility none -> smokes adds ~5-9 points of round win (rushes ~1), smokes -> full ~2-5 more (executes, splits, fakes, default_mid and post-plants use it; rushes throw only flashes, so for them only the number of flashes counts). CT utility heavy instead of normal takes 1-13 points from the T (most against rushes and hits into a defended or stacked site, least against fakes and aggressive_mid fights); save instead of normal gives the T 2-9 points.
PACE: fast hits earlier (+2-5 points vs aggressive_mid, up to 5 fewer vs the other setups); slow plays more mid duels (+3-4 for default_mid vs stacks, 1-5 fewer vs aggressive_mid); default is the best general choice. Rushes ignore pace.
ECONOMY IN PRACTICE: a full buy beats a cheap force most of the time, and money compounds (winners keep their guns). Forcing every round loses to a team that saves when broke and full-buys after.
THE SITUATION shows each round: score, round and half, your exact money and the full-buy cost, both loss bonuses, what you would have after an eco, kept weapons, who has an AWP, the enemy's estimated money, the opponent's last rounds on their current side, their tendencies, and how your own last calls on this side went. A team that repeats itself can be read and punished: adapt, and change when they adapt to you.`

// matchupTable: la taula de matchupWin en text.
func matchupTable() string {
	var b strings.Builder
	b.WriteString("T ROUND WIN % (both full buy, T utility full, CT utility normal, default pace):\n")
	fmt.Fprintf(&b, "%-12s", "T vs CT")
	for _, s := range []string{"standard", "stack_a", "stack_b", "agg_mid", "retake_b", "retake_a"} {
		fmt.Fprintf(&b, " %8s", s)
	}
	b.WriteString("\n")
	for i, st := range stratVals {
		fmt.Fprintf(&b, "%-12s", st)
		for _, v := range matchupWin[i] {
			fmt.Fprintf(&b, " %8d", v)
		}
		b.WriteString("\n")
	}
	return b.String()
}

const starterText = `# Strategy: "Read, then punish"

A complete Counter-Strike 2 plan. The coach model reads this file and the round's Situation before every round, so it is written as numbered rules: **the first rule that matches wins**, every rule names the exact Situation line it reads and the exact JSON values to answer. Copy that style when you improve it: concrete conditions, exact values, one default for every case.

## 0. What to read in the Situation
- "PISTOL ROUND" / "Overtime half start" / "Last round of the half".
- "Your money: $X total" and "Full buy now costs your team about $Y (affordable | NOT affordable)".
- "Opponent's last rounds as CT/T" (one line per round, oldest first) — their setup or strategy and the site they hit.
- "Your last rounds as T/CT" — what we called and whether "we WON" or "we LOST".

## 1. Economy (both sides, first match wins)
1. Pistol round or overtime half start → "buy":"full".
2. Full buy **affordable** → "buy":"full", "awp":true.
3. Last round of the half → "buy":"force" (money resets anyway).
4. Our money is at least **60%** of the full-buy cost → "buy":"force" (most players still get rifles).
5. Otherwise → "buy":"eco": saving now buys a full buy next round. Never "half".
- "awp" is true only on full buys.

## 2. T side
**Pick the counter.** Look at the opponent's CT setups in their last 3 rounds and take the one they used most (tie: the most recent). No CT rounds seen yet → "default_mid".
| Their setup | Our strategy |
|---|---|
| stack_a | fake_a_go_b |
| stack_b | fake_b_go_a |
| aggressive_mid | rush_a on even rounds, rush_b on odd rounds |
| retake_a | execute_a |
| retake_b | execute_b |
| standard_2_1_2 | fake_a_go_b on even rounds, fake_b_go_a on odd rounds |

**They adapted?** If our last two rounds as T both used that same strategy and both say "we LOST", counter only their most recent setup instead (and for rushes/fakes take the other site from the table). If that one also lost twice in a row → "default_mid".

**Buy-dependent details.**
- Full buy: "utility":"full". Force: "utility":"smokes".
- Eco: ignore the table → rush the site that had fewer CTs in their last round (the "(n A, n mid, n B)" numbers; rush_a only if A had fewer, else rush_b), "utility":"none".
- "pace":"fast" for rushes, "default" for everything else.

## 3. CT side
**Read their last 3 T rounds.**
1. "default_mid" in 2 or more of them → "aggressive_mid".
2. Otherwise, they hit the same site ("hit A" / "hit B") in 2 or more of them → "stack_a" / "stack_b".
3. Otherwise (or fewer than 2 rounds seen) → "standard_2_1_2".

**They adapted?** If our last two rounds as CT both used that same setup and both say "we LOST" → "standard_2_1_2".

**Buy-dependent details.**
- Utility: "heavy" on a full buy, "normal" on a force, "save" on an eco.
- Eco: stack the site they hit most often in all the rounds shown (tie: the most recent; keep the choice above if none).

## 4. Examples
- R7, CT, full buy affordable, they executed A in R5 and R6 → {"buy":"full","setup":"stack_a","utility":"heavy","awp":true}
- R8, T, full buy affordable, they played stack_a twice → {"buy":"full","strategy":"fake_a_go_b","utility":"full","pace":"default","awp":true}
- R3, T, $9,000 vs a $21,000 full buy (43%) → {"buy":"eco","strategy":"rush_b","utility":"none","pace":"fast","awp":false}
- R4, T, $16,000 vs $21,000 (76%), they played aggressive_mid → {"buy":"force","strategy":"rush_a","utility":"smokes","pace":"fast","awp":false}
`

// matchupWin: % de rondes que guanya el T amb cada estratègia (files, ordre
// de stratVals) contra cada configuració CT (columnes, ordre de setupVals),
// tots dos amb compra completa, utilitat T "full" i CT "normal", ritme
// "default". És la taula que publiquen les regles; el motor hi està calibrat
// (planEdge) i les proves ho comproven.
var matchupWin = [9][6]int{
	//  std st_a st_b agg rt_b rt_a
	{40, 25, 70, 70, 35, 40}, // rush_a
	{40, 70, 25, 70, 40, 35}, // rush_b
	{55, 25, 70, 40, 45, 65}, // execute_a
	{55, 70, 25, 40, 65, 45}, // execute_b
	{58, 30, 70, 35, 50, 55}, // split_a
	{58, 70, 30, 35, 55, 50}, // split_b
	{47, 68, 68, 35, 60, 60}, // default_mid
	{62, 75, 25, 50, 55, 45}, // fake_a_go_b
	{62, 25, 75, 50, 45, 55}, // fake_b_go_a
}
