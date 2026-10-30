package cs2

const rulesText = `COUNTER-STRIKE 2 — ARENA RULES (map: Mirage)
Two teams of 5 identical players; only the strategy differs.
FORMAT: MR12. First to 13 rounds wins. Sides swap after round 12. At 12-12, overtime: blocks of 6 rounds (first to 4 in the block), $10,000 each at every OT half, sides swap every 3 rounds.
SIDES: T attacks and must plant the bomb at site A or B (bomb explodes 40s after plant). CT defends; wins by killing all Ts, by the clock (1:55) running out before a plant, or by defusing (5s with kit, 10s without).
ECONOMY (per player): start $800 each half. Win: $3,250 ($3,500 for bomb explosion / defuse). Loss bonus $1,400/1,900/2,400/2,900/3,400: goes up one step per loss and down one step per win (each half starts at step 1, $1,900). If the bomb was planted, losing Ts still get +$800 each. Ts alive when time runs out get no loss money. Kill rewards: rifles $300, SMGs $600, AWP $100, pistols $300. Max $16,000. Survivors keep their weapons and armor; the dead lose everything.
PRICES: AK-47 $2,700, M4A1-S $2,900, Galil $1,800, FAMAS $2,050, MAC-10 $1,050, MP9 $1,250, AWP $4,750, kevlar $650, kevlar+helmet $1,000, defuse kit $400, smoke $300, flash $200, molotov $400 (CT $500), HE $300.
DECISIONS: one per round per side, taken by both teams at the same time before the round. Answer one JSON object following the given schema. Missing/invalid fields fall back to defaults (full buy if affordable, else eco; T default_mid with smokes; CT standard_2_1_2 with normal utility; AWP on).
HOW ROUNDS ARE DECIDED: the round is simulated as movements and duels. Duels depend on weapons (rifle >> SMG > pistol; armor matters), numbers, position, utility and how your plan matches theirs:
- Stacking the site the Ts hit is very strong; a hit or fake on the other site beats a stack.
- Rushes beat thin sites, aggressive mid and slow CT setups, but die to stacks and heavy CT utility.
- T utility (smokes, flashes, molotovs) wins executes and post-plants; CT utility delays and punishes hits and helps retakes.
- Aggressive mid wins early duels against default/slow Ts but leaves sites thin.
- Retake setups give up a site and win it back together after the plant.
- AWPs shine on default/slow rounds and mid control, less in rushes.
- Eco vs full buy loses most of the time; force buys can upset. Money decisions compound over rounds.
The situation shows both economies (enemy money is an estimate) and the opponent's recent rounds. Read their tendencies, adapt, and avoid being predictable.`

const starterText = `# Strategy: "Read, then punish"

A complete Counter-Strike 2 game plan for Mirage. The model reads this file before every round, so it is written as clear priorities and if/then rules.

## 1. Principles (in order of priority)
1. **Economy first.** Never let both rounds in a row be weak buys. A good eco now buys two good rounds later.
2. **Read the opponent.** Use the last rounds in the situation. Repeat what works; stop what they have answered.
3. **Be unpredictable.** Never call the same T strategy or the same CT setup more than 2 rounds in a row.
4. **Utility wins rounds.** On full buys, buy utility: T "full" or "smokes", CT "normal" or "heavy".

## 2. Economy rules (both sides)
- Pistol round (or OT half start): buy "full".
- If the situation says the full buy is **affordable**: buy "full".
- Last round of a half: buy "force" (money resets anyway).
- If not affordable:
  - If (our money + 5 × our loss bonus) reaches the full buy cost: buy "eco" now, full buy next round.
  - If the enemy "probably cannot full buy" (they are poor too): buy "force" to win the cheap round.
  - If the loss bonus is already at $3,400 and we have at least 60% of a full buy: "force".
  - Otherwise: "eco".
- Right after winning the pistol round: "half" (SMGs farm money against their eco).
- AWP: "awp": true only when the full buy is comfortably affordable.

## 3. T side (attack)
### Default plan
- Mix "execute_a", "execute_b" and "default_mid"; use "split_a"/"split_b" now and then.
- Utility "full" on full buys, "smokes" on force buys, "none" on ecos. Pace "default".
### Adapting to the CTs (read "[they CT]" lines)
- If they **stacked A** last round (4 A) → hit B: "execute_b", or "fake_a_go_b" if they stacked A twice in a row.
- If they **stacked B** → "execute_a" or "fake_b_go_a".
- If they play **aggressive_mid** → "rush_a" or "rush_b" with pace "fast" (their sites are thin).
- If they use **retake** setups → hit the site they give up with utility "full" (molotovs win the post-plant).
- If they have been **standard** for 3+ rounds → "split_a"/"split_b" or a fake.
### Eco / force rounds as T
- Eco: "rush_b" or "rush_a" (whichever site had fewer CTs last time), pace "fast", utility "none".
- Force: "rush" or "execute" the weaker site with "flash_heavy".

## 4. CT side (defence)
### Default plan
- "standard_2_1_2", utility "normal" (use "heavy" on full buys), AWP on.
### Adapting to the Ts (read "[they T]" lines and their tendencies)
- If they hit the **same site** in 2 of their last 3 rounds → stack it ("stack_a"/"stack_b").
- If they **rushed** recently → utility "heavy" (molotovs stop rushes), keep standard or stack.
- If they play **default/slow** a lot → "aggressive_mid" to win early duels.
- If they just **faked** us → stay "standard_2_1_2" and do not over-rotate.
- If we are on an eco → "stack_a" or "stack_b" on their favourite site (gamble for a cheap win).

## 5. Examples
- R6, CT, they executed A in R4 and R5 → {"buy":"full","setup":"stack_a","utility":"heavy","awp":true}
- R8, T, they stacked A twice → {"buy":"full","strategy":"fake_a_go_b","utility":"full","pace":"default","awp":true}
- R3, T, lost pistol, money $5,200, cannot full buy, enemy rich → {"buy":"eco","strategy":"rush_b","utility":"none","pace":"fast","awp":false}
`
