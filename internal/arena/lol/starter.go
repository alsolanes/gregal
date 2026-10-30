package lol

// starterText és el .md d'exemple. La política de prova "starter"
// (policies_test.go) en segueix la lògica llegint només la Situation: si
// canvies una regla aquí, canvia-la també allà.
const starterText = `# Strategy: "Read, counter, play your window"

How to use this file: every rule below names the exact line of the situation it
reads (POWER NOW, LANE MATCHUPS, CONDITIONAL COUNTERS NOW, ENEMY STANCES SO FAR...).
Go through the steps in order and stop at the first rule that applies. Quote that
rule as your "rule".

## 1. Draft — one clear identity, no holes

Bans
- Phase 1: ban engage tanks with engage3 + frontline3, then late carries with dps3.
  Always list 6 names: if one is gone, the next is used.
- Phase 2: read "Enemy stance fit so far" and their open roles. Ban the available
  champion that adds most to their best stance in a role they still need.

Picks — score every legal champion and take the best:
1. Identity: after the pick, my best stance fit should be high (aim ≥ 1.3) AND my
   second-best ≥ 1.0, so I have a counter-stance later. Use the stance-fit formulas
   in the rules.
2. Frontline: end with total frontline ≥ 5 (never < 3: that is −8% forever).
3. Damage: at least 2 AP and 2 AD.
4. Curve: don't mix — with 2+ early champions add no late ones, and vice versa.
5. Answer what the enemy shows ("Enemy comp so far"):
   - engage ≥ 6 (per 5 champions) → peel and frontline, so I can play protect;
   - frontline + peel ≤ 4 after 3 picks → an assassin (pick3);
   - poke ≥ 6 → hard engage (engage3).

## 2. Laning (00:00) — read LANE MATCHUPS
- STRONGER lane → aggressive. WEAKER → passive. even → even.
- Jungle: jungle STRONGER by ≥ 0.8 → invade. Otherwise gank the lane where I am
  most STRONGER (≥ 0.5). No stronger lane → farm.

## 3. First objective (08:00)
- POWER NOW ≥ −3% → dragon. Otherwise → grubs (they go dragon; take the other side).

## 4. Plates (12:00) — read "Lane gold leads"
- Best lane lead > 800 and POWER NOW ≥ +5% → dive that lane.
- Best lane lead > 0 → plates in that lane.
- No lead anywhere → farm.

## 5. Every 4 minutes: stance first, then objective

### Stance — counter their most likely stance
1. Predict: their most frequent stance in ENEMY STANCES SO FAR (the last ones count
   more). No history yet → the "best stance" in ENEMY COMP.
2. Counters (rules + CONDITIONAL COUNTERS NOW):
   - they engage → protect if "protect beats engage — you YES"; split if
     "split beats engage — you YES".
   - they siege → engage or pick. they pick → engage or protect.
   - they split → siege or pick; engage too if "split beats engage — enemy no"
     and GOLD DIFF ≥ −1000.
   - they protect → siege or split; engage too if "protect beats engage — enemy no"
     and GOLD DIFF ≥ −1000.
3. Among those counters, take the one with my highest stance fit, if it is ≥ 0.85.
4. No usable counter → my best-fit stance that does NOT lose to theirs.

### Objective — first rule that applies
1. I have BARON BUFF or ELDER BUFF → towers.
2. POWER NOW is EVEN or better (> −5%), or I counter their stance and am not
   CLEARLY BEHIND → fight for it: the neutral objective they usually take (ENEMY
   OBJECTIVES SO FAR) if it is available, else elder > baron (baron only if AHEAD)
   > dragon > herald, else towers.
3. SOUL POINT and the enemy has 3 drakes, and I am not CLEARLY BEHIND → dragon.
4. My comp is SCALING, "In 8 min" is better than now by 3+ points, and they are not
   going for a neutral objective → farm.
5. They scale better ("In 8 min" no better than now) and I am not CLEARLY BEHIND →
   take the neutral objective they do NOT usually take (dragon, then herald).
6. Otherwise (behind) → towers: take the other side of the map. Never start baron
   or elder while behind: they collapse on you and you lose the fight AND the buff.

## 6. Examples
- 18:00, POWER NOW −2% (EVEN), ENEMY STANCES SO FAR "engage, engage", "protect
  beats engage — you YES", my protect fit 1.25, they took dragon twice, dragon up →
  {"objective":"dragon","stance":"protect"}
- 22:00, POWER NOW −14% (CLEARLY BEHIND), baron UP, they keep sieging, my pick fit
  1.1 → {"objective":"towers","stance":"pick"}
- 26:00, POWER NOW +9% (AHEAD), BARON UP, they split every time →
  {"objective":"baron","stance":"siege"}
`
