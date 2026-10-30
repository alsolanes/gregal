package lol

const starterText = `# Strategy: "Know your window"

My team drafts ONE clear identity, counters what the enemy shows, and plays to the
moment its composition is strongest. When in doubt: coherent comp > single strong champion.

## 1. Draft

### Bans (phase 1)
- Ban flexible, high-impact champions first: strong engage tanks (engage3 + frontline3)
  and hyper-scaling carries (late curve + dps3).
- Always give 5-6 names: if one is already banned, the next is used.

### Bans (phase 2)
- Look at the enemy's 3 picks. Ban the champion that completes THEIR identity
  (e.g. their last good engage support, or a peel enchanter for their late carry).
- Ban the best remaining champion for the role I still need if the enemy could take it.

### Picks — choose an identity by my first two picks
| Identity | Need | Win condition |
|---|---|---|
| Engage | engage ≥ 7 total, frontline ≥ 6, dps ≥ 9 | force 5v5 fights at objectives |
| Protect | peel ≥ 5, 1-2 late dps3 carries, one tank | survive early, win late front-to-back |
| Siege/poke | poke ≥ 7, waveclear ≥ 9 | take towers without fighting |
| Pick | pick ≥ 8 (assassins, hooks) | catch isolated enemies, snowball early |
| Split | one splitpush3 + waveclear | 1-3-1 side lanes, avoid 5v5 |

Rules for every pick:
1. Never end the draft with frontline < 3: one tank/bruiser is mandatory, two is better.
2. Keep damage mixed: at least 2 AP and 2 AD.
3. First picks: flexible or strong-in-any-comp champions. Last picks (R4/R5, B4/B5): counterpicks.
4. Counters from the enemy draft:
   - Enemy has 2+ assassins / pick threats → pick peel (peel3 enchanter) and frontline.
   - Enemy is squishy (frontline ≤ 3, peel ≤ 2) → add an assassin (pick3).
   - Enemy is poke/siege → pick hard engage (engage3) to dive them.
   - Enemy is engage → pick protect/disengage (peel3) or a splitpusher.
   - Enemy has no waveclear (waveclear ≤ 6) → a splitpush3 champion punishes them.
5. Match power curves: don't mix three early champions with two late ones. Early comps must
   snowball; late comps must survive.

## 2. Laning (00:00)
- My lane is stronger (higher lane trait, early curve) → aggressive. Weaker → passive.
- Bot lane is aggressive only if both adc and support have lane ≥ 2.
- Jungle: gank the lane where MY laner is strongest and the enemy laner is likely aggressive.
  With a late jungler → farm. With an early jungler vs a late jungler → invade.

## 3. First objectives (08:00)
- Stronger early (lead in gold, more early champions, bot lane ahead) → dragon.
- Weaker early → grubs if the enemy always goes dragon (check their history), else farm.
- Siege/split comps love grubs (tower damage).

## 4. Plates (12:00)
- If the enemy history shows they pushed/dived one lane before → roam to that lane.
- Otherwise plates in the lane where I lead the most; dive only with a lane lead > 800.
- Late comp and behind → farm.

## 5. Mid and late game (every 4 minutes)
**Stance: read the enemy's stance history and counter their most frequent stance**,
choosing among the two stances that beat it the one with my better stance fit:
- they engage → protect (if fit ≥ 1.0) or split
- they siege → engage or pick
- they pick → engage or protect
- they split → siege or pick
- they protect → siege or split
If they have no history yet, use my best-fit stance. Never use a stance with fit < 0.8.

**Objective:**
- I am stronger now (gold lead, power multiplier ≥ enemy) → contest what they want
  (same objective = teamfight that I should win). Priority: elder > baron (if UP) > soul-point
  dragon > dragon > herald > towers.
- I am weaker now but scale better (my multiplier at +8min > enemy's) → do NOT fight: take a
  different objective from the one they usually choose, or farm.
- I am weaker and they scale better → force now: engage on their objective.
- Never start baron while clearly behind in gold: they will collapse on it.
- With baron/elder buff active → towers (siege).
- Enemy has 3 dragons → always contest the next dragon (deny soul).

## 6. Examples
- Enemy history "objective=dragon stance=engage" three times, my protect fit 1.3 →
  {"objective":"dragon","stance":"protect"} if I'm even or ahead; {"objective":"baron","stance":"split"} if behind.
- 26:00, I lead +4k, baron UP, enemy stance last two times "split" →
  {"objective":"baron","stance":"siege"}.
`
