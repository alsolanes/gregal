// L'assistent «Crea la meva estratègia» de l'Arena: tres preguntes amb
// targetes (en llenguatge planer, amb què guanya i què arrisca cada opció) i
// un .md complet a punt per jugar. Les taules de contrajoc surten de la guia
// del motor (games[].playbook): l'estil només desempata entre respostes igual
// de bones, de manera que tota estratègia generada llegeix i castiga el rival.

import { answers } from './arena-playbook.js';

// Les preguntes de cada joc. Cada opció: id, emoji, títol, el que fa, i el
// «pro / contra» que l'ensenya.
export const QUESTIONS = {
  nfl: [
    { id: 'off', title: 'How do you want to attack?', options: [
      { id: 'ground', icon: '🐂', title: 'Ground and pound', text: 'Run the ball, wear them down, then fake the run and throw.', pro: 'Few turnovers, eats the clock', con: 'Slow when you fall behind' },
      { id: 'balanced', icon: '⚖️', title: 'Balanced reader', text: 'No favourite play: whatever beats the defense they call most.', pro: 'Hardest to predict', con: 'Needs a few plays to read them', rec: true },
      { id: 'air', icon: '🚀', title: 'Air raid', text: 'Throw first: quick passes, medium routes and the odd deep shot.', pro: 'Big plays, fast comebacks', con: 'Sacks and interceptions vs blitzes' },
    ] },
    { id: 'def', title: 'And on defense?', options: [
      { id: 'attack', icon: '🔥', title: 'Bring the heat', text: 'Blitz and press their receivers man to man.', pro: 'Sacks, stops runs', con: 'Screens and deep shots burn you' },
      { id: 'read', icon: '🧠', title: 'Read and react', text: 'Pick the coverage that stops the play they call most.', pro: 'Adapts to any offense', con: 'Guesses until it has data', rec: true },
      { id: 'safe', icon: '🧱', title: "Bend, don't break", text: 'Keep everything in front with deep zones; give short yards, never the long one.', pro: 'No easy touchdowns', con: 'Runs eat you alive' },
    ] },
    { id: 'risk', title: 'On 4th down…', options: [
      { id: 'safe', icon: '🦺', title: 'Play it safe', text: 'Kick: field goal in range, otherwise punt.', pro: 'Never gives them a short field', con: 'Leaves points on the table' },
      { id: 'smart', icon: '📊', title: 'Smart aggression', text: 'Go for it on 4th & 1-2 past midfield, and late when behind.', pro: 'What the analytics say', con: 'A miss hurts', rec: true },
      { id: 'gambler', icon: '🎲', title: 'Gambler', text: 'Go for it on 4th & 3 or less in their half, and on 4th & 1 almost anywhere.', pro: 'Keeps drives alive', con: 'Hands them field position' },
    ] },
  ],
  cs2: [
    { id: 'eco', title: 'How do you handle money?', options: [
      { id: 'disciplined', icon: '💰', title: 'Disciplined', text: 'Save until the whole team can buy soakers; force only when almost there.', pro: 'Strong full-buy rounds', con: 'Gives away some rounds' },
      { id: 'balanced', icon: '⚖️', title: 'Balanced', text: 'Force when you have 60% of a full buy, save otherwise.', pro: 'What the pros do', con: 'Needs exact money reading', rec: true },
      { id: 'aggressive', icon: '💦', title: 'Always fight', text: 'Force-buy with 40% of a full buy: every round is winnable.', pro: 'Steals rounds', con: 'Broke when it fails' },
    ] },
    { id: 't', title: 'How do you attack as T?', options: [
      { id: 'fast', icon: '⚡', title: 'Fast and loud', text: 'Rush a site before they can rotate.', pro: 'Punishes empty sites and aggressive CTs', con: 'Walks into stacks' },
      { id: 'methodical', icon: '🧭', title: 'Methodical', text: 'Smokes and flashes: executes, splits and a slow default.', pro: 'Solid against everything', con: 'Predictable if repeated', rec: true },
      { id: 'tricky', icon: '🎭', title: 'Mind games', text: 'Fake one site, hit the other.', pro: 'Destroys stacks and early rotations', con: 'Weak vs a patient defense' },
    ] },
    { id: 'ct', title: 'How do you defend as CT?', options: [
      { id: 'solid', icon: '🛡️', title: 'Rock solid', text: 'Standard 2-1-2 until they show a pattern.', pro: 'Never loses badly', con: 'Fakes and splits beat it' },
      { id: 'read', icon: '🔎', title: 'Read and stack', text: 'Move players to the site they keep hitting.', pro: 'Punishes repetitive teams', con: 'Fakes punish you back', rec: true },
      { id: 'aggressive', icon: '🐺', title: 'Hunt mid', text: 'Push mid early for picks.', pro: 'Beats slow defaults and executes', con: 'Rushes run past it' },
    ] },
  ],
  lol: [
    { id: 'comp', title: 'What team do you draft?', options: [
      { id: 'early', icon: '⚔️', title: 'Early snowball', text: 'Engage and early-game champions: fight, win, end before 25 minutes.', pro: 'Ends games fast', con: 'Falls off late' },
      { id: 'scaling', icon: '🛡️', title: 'Protect the carry', text: 'Peel, frontline and a late carry: survive, then win the late teamfights.', pro: 'Unbeatable at 30+ minutes', con: 'Weak early, must not throw', rec: true },
      { id: 'pick', icon: '🗡️', title: 'Assassins', text: 'Catch isolated enemies and snowball from picks.', pro: 'Punishes sieges and split pushers', con: 'Dies to hard engage' },
      { id: 'siege', icon: '🏹', title: 'Poke and siege', text: 'Poke from range and take towers without fighting.', pro: 'Safe pressure, beats split', con: 'Gets dived by engage' },
    ] },
    { id: 'obj', title: 'What do you fight for?', options: [
      { id: 'dragons', icon: '🐉', title: 'Dragons', text: 'Stack drakes for the soul and the Elder.', pro: 'Permanent buffs', con: 'Gives them herald and towers' },
      { id: 'adaptive', icon: '📈', title: 'Whatever my power allows', text: 'Fight for objectives when even or ahead, trade towers when behind.', pro: 'Rarely throws', con: 'Less pressure early', rec: true },
      { id: 'baron', icon: '👑', title: 'Baron and towers', text: 'Push towers early and take Baron the moment you are ahead.', pro: 'Ends games', con: 'Ignores dragon soul' },
    ] },
    { id: 'lane', title: 'In lane (first 8 minutes)…', options: [
      { id: 'aggressive', icon: '🔥', title: 'Push every advantage', text: 'Aggressive where you are stronger, even elsewhere; the jungler ganks.', pro: 'Gold leads', con: 'Ganked when it goes wrong' },
      { id: 'read', icon: '🔎', title: 'Read the matchups', text: 'Aggressive only where stronger, passive where weaker.', pro: 'Safe and efficient', con: 'Rarely snowballs', rec: true },
      { id: 'passive', icon: '🌾', title: 'Farm and scale', text: 'Passive lanes, the jungler farms, nobody dies.', pro: 'No early disasters', con: 'Gives away lane gold' },
    ] },
  ],
};

export const DEFAULTS = game => Object.fromEntries((QUESTIONS[game] || []).map(q => [q.id, (q.options.find(o => o.rec) || q.options[0]).id]));

// La millor resposta del playbook contra cada opció del rival: primer la nota
// del motor; entre les de la millor nota (i, si el motor dona números, a 3
// punts o menys de la millor), la preferència de l'estil. Un estil no fa
// mai triar una resposta clarament pitjor. skip = ids que no van a la taula
// (crides de situació).
export const TIE = 3;
export function counters(sec, pref = {}, skip = []) {
  const out = {};
  sec.theirs.forEach((th, j) => {
    if (skip.includes(th.id)) return;
    const list = answers(sec, j).filter(a => !skip.includes(a.id));
    if (!list.length) return;
    const top = list[0], near = a => a.grade === top.grade && (a.value == null || top.value == null || top.value - a.value <= TIE);
    const order = list.slice().sort((a, b) => (near(b) - near(a)) || (near(a) ? (pref[b.id] || 0) - (pref[a.id] || 0) : 0) || b.grade - a.grade || (b.value ?? 0) - (a.value ?? 0) || a.i - b.i);
    out[th.id] = order.filter(a => a.grade > 0).map(a => a.id);
    // Quantes de les primeres són igual de bones (per alternar-les).
    Object.defineProperty(out[th.id], 'near', { value: order.filter(a => near(a) && a.grade > 0).length });
  });
  return out;
}

const section = (pb, id) => pb && pb.sections && pb.sections.find(s => s.id === id);
const pick = (q, game, id) => { const qs = (QUESTIONS[game] || []).find(x => x.id === q); return qs && qs.options.find(o => o.id === id); };
const NFL_TARGET = { pass_short: ' to wr2', pass_medium: ' to wr1', pass_deep: ' to wr1', screen: ' to rb', play_action: '' };
const nflCall = c => '**' + c + '**' + (NFL_TARGET[c] || '');

// El nom de l'estratègia, a partir de les respostes.
export function strategyName(game, a) {
  const t = Object.entries(a).map(([q, id]) => (pick(q, game, id) || {}).title).filter(Boolean);
  return t.slice(0, 2).join(' + ');
}

function nfl(a, pb) {
  const off = section(pb, 'offense'), def = section(pb, 'defense');
  const offPref = { ground: { run_inside: 3, run_outside: 3, play_action: 2 }, air: { pass_medium: 3, pass_short: 3, pass_deep: 2, screen: 2 }, balanced: {} }[a.off] || {};
  const defPref = { attack: { blitz: 3, man: 2 }, safe: { cover2: 3, cover3: 2, base: 1 }, read: {} }[a.def] || {};
  const oc = off ? counters(off, offPref, ['qb_sneak']) : {};
  const dc = def ? counters(def, defPref, ['goal_line', 'prevent', 'qb_sneak']) : {};
  const open = { ground: ['run_inside', 'run_outside', 'pass_short'], air: ['pass_short', 'pass_medium', 'pass_medium'], balanced: ['run_inside', 'pass_short', 'pass_medium'] }[a.off];
  const dOpen = { attack: ['blitz', 'man', 'man'], safe: ['base', 'cover2', 'cover3'], read: ['base', 'cover2', 'man'] }[a.def];
  const idOff = { ground: 'We run the ball to set up everything else: runs first, play-action once the run is established.', air: 'We throw to win: quick and medium passes are our base, the deep ball when they press man to man.', balanced: 'We have no favourite play: we call whatever beats the defense they show most.' }[a.off];
  const idDef = { attack: 'On defense we attack the quarterback, but never with the same call three plays in a row.', safe: 'On defense nothing gets behind us: we give short yards and take away the big play.', read: 'On defense we read their most frequent play and call the coverage that stops it.' }[a.def];
  const fourth = {
    safe: ['- "A field goal from here would be X yards" and X is at most our kicker\'s range → **field_goal**.', '- Q4, under 2:00, behind by 4 or more → go for it with our counter call.', '- Anything else → **punt**.'],
    smart: ['- "A field goal from here would be X yards" and X is at most our kicker\'s range → **field_goal**.', '- 4th & 1 or 2 at "the defense\'s" 45 or closer → go for it: **qb_sneak** on 4th & 1, otherwise our counter call below.', '- Q4, under 5:00, behind by 4 or more → go for it with our counter call.', '- Anything else → **punt**.'],
    gambler: ['- 4th & 1 anywhere past our own 40 → go for it: **qb_sneak**.', '- 4th & 3 or less at "the defense\'s" 50 or closer → go for it with our counter call.', '- "A field goal from here would be X yards" and X is at most our kicker\'s range → **field_goal**.', '- Q4, under 6:00, behind → go for it with our counter call.', '- Anything else → **punt**.'],
  }[a.risk];
  const offRows = (off ? off.theirs.map(t => t.id) : []).filter(id => oc[id] && oc[id].length).map(id => {
    const [A, B] = oc[id];
    const note = c => c === 'play_action' ? ' (only once runs are 40%+ of our plays)' : '';
    return `| ${id} | ${nflCall(A)}${note(A)} | ${B ? nflCall(B) + note(B) : nflCall(A)} |`;
  });
  const defRows = (def ? def.theirs.map(t => t.id) : []).filter(id => dc[id] && dc[id].length).map(id => `| ${id} | **${dc[id][0]}** |`);
  const exDef = off && off.theirs.find(t => oc[t.id] && oc[t.id].length);
  const exOff = def && def.theirs.find(t => dc[t.id] && dc[t.id].length);
  return [
    '## Identity', `${idOff} ${idDef} We counter what the opponent calls most, keep calling what gains yards and drop what fails. We never let them key on one call of ours.`, '',
    '## Priorities (go down the list; the first rule that applies decides)', '1. The situation: 4th down, the clock, short yardage.', '2. The RESULTS line: what has worked and what has failed in this game.', '3. Their tendencies: counter their most frequent call (tables below).', '4. Our style, and no single call above 40% of our plays.', '',
    '## Offense', '### 4th down (decide this first)', ...fourth, '',
    '### Clock', `- Q4, under 3:00, ahead: ${a.off === 'air' ? 'pass_short or run_inside' : 'run (run_inside, or run_outside if their top defense is man, blitz or cover3)'}, tempo **milk**.`, '- Q4 under 5:00 behind, or Q2 under 2:00: tempo **hurry** and pass (pick the pass from the table below).', '',
    '### Counter their defense', 'Look at "Their DEFENSE so far: overall" and take their most-called defense. On 3rd & 5+, use their "on 3rd & 5+" numbers instead.',
    '| Their top defense | Option A | Option B (every 3rd play, or when A is failing) |', '|---|---|---|', ...offRows,
    '- On 3rd & 7+, prefer a pass from that row.',
    ...{ ground: ['- Leading by 8+ in the second half: run_inside or run_outside (whichever the table allows), tempo **milk**.', '- Runs under 40% of our plays after the 1st quarter: on 1st down pick a run from the table even if it is Option B.'],
      air: ['- 3rd & 7+: pass_medium to wr1, unless their 3rd-down blitz is 35%+ (then screen to rb).', '- Ahead by 3+ scores: still pass, but pass_short only (no deep shots).'],
      balanced: ['- Keep the mix: if runs or passes are under 35% of our plays, pick that kind from the table on the next 1st down.'] }[a.off],
    `- No data yet: 1st down ${open[0]}, 2nd down ${open[1]}, 3rd down ${open[2]}.`, '',
    '### Use the RESULTS', '- In "RESULTS of your offense ... by your call": if the call we are about to make has 3+ plays and under 4.0 yds/play, switch to the other option.', '- A call at 7+ yds/play and 60%+ success: keep calling it (still respecting the 40% cap).', '',
    '## Defense', '### Counter their offense', 'Look at "Their OFFENSE so far: overall" and take their most-called play (on 3rd & 5+, their "on 3rd & 5+" numbers).',
    '| Their top play | Our call |', '|---|---|', ...defRows,
    '- In "RESULTS of their offense ... by your defense": a defense that allows 6+ yds/play over 4+ plays is broken; use a different one.', '',
    '### Situations', '- 3rd or 4th & 1-2, or goal to go from our 3: **goal_line**.',
    ...{ attack: ['- 3rd & 7+: **blitz**, unless they called screen on 20%+ of their plays (then **man**).'], safe: ['- 3rd & 7+: **cover2** (no deep balls), unless they ran on 50%+ of their 3rd downs (then **cover3**).', '- A defense from the table that allows 6+ yds/play: fall back to **base**, never to blitz.'], read: ['- 3rd & 7+: use their "on 3rd & 5+" numbers with the table above.'] }[a.def], `- No data yet: ${dOpen[0]} on 1st down, ${dOpen[1]} on 2nd, ${dOpen[2]} on 3rd.`,
    `- Q2 or Q4 under 1:00 and we are ahead: **${a.def === 'attack' ? 'cover2' : 'prevent'}**.`, '- Never blitz on more than 3 plays in a row: a screen kills it.', '',
    '## Examples',
    exDef ? `- 3rd & 8 at the defense's 42; Their DEFENSE so far: overall: ${exDef.id} 45%, ... → {"play":"${oc[exDef.id][0]}","target":"${(NFL_TARGET[oc[exDef.id][0]] || '').replace(' to ', '')}","tempo":"normal"}` : '',
    exOff ? `- Their OFFENSE so far: overall: ${exOff.id} 50%, ...; 1st & 10 → {"call":"${dc[exOff.id][0]}"}` : '',
  ];
}

function cs2(a, pb) {
  const t = section(pb, 't'), ct = section(pb, 'ct');
  const tPref = { fast: { rush_a: 3, rush_b: 3 }, methodical: { execute_a: 3, execute_b: 3, split_a: 2, split_b: 2, default_mid: 2 }, tricky: { fake_a_go_b: 3, fake_b_go_a: 3 } }[a.t] || {};
  const ctPref = { solid: { standard_2_1_2: 3 }, read: { stack_a: 2, stack_b: 2 }, aggressive: { aggressive_mid: 3 } }[a.ct] || {};
  const tc = t ? counters(t, tPref) : {}, cc = ct ? counters(ct, ctPref) : {};
  const force = { disciplined: 80, balanced: 60, aggressive: 40 }[a.eco];
  const tOpen = { fast: 'rush_b', methodical: 'default_mid', tricky: 'fake_a_go_b' }[a.t];
  const ctOpen = { solid: 'standard_2_1_2', read: 'standard_2_1_2', aggressive: 'aggressive_mid' }[a.ct];
  // Dues respostes igual de bones: s'alternen (així no ens llegeixen).
  const tRows = (t ? t.theirs.map(x => x.id) : []).filter(id => tc[id] && tc[id].length).map(id => {
    const best = tc[id].slice(0, Math.min(2, tc[id].near));
    return `| ${id} | ${best.length > 1 ? `${best[0]}, or ${best[1]} if our last T round was ${best[0]}` : best[0]} |`;
  });
  const ctRows = (ct ? ct.theirs.map(x => x.id) : []).filter(id => cc[id] && cc[id].length).map(id => `| ${id} | ${cc[id][0]} |`);
  // Un exemple de cada taula (la fila que més s'aprofita: la segona, si n'hi ha).
  const exOf = (sec, c) => { const ids = (sec ? sec.theirs.map(x => x.id) : []).filter(k => c[k] && c[k].length); return ids[1] || ids[0]; };
  const exT = exOf(t, tc), exCT = exOf(ct, cc);
  const id = {
    disciplined: 'We play for full buys: we save together and hit with soakers and utility.',
    balanced: 'We buy with the team: full when we can, force when we are close, save otherwise.',
    aggressive: 'We fight every round: if we can buy blasters, we buy them.',
  }[a.eco] + ' ' + { fast: 'As T we hit fast before they rotate.', methodical: 'As T we take sites with smokes and flashes, and change the plan when they read us.', tricky: 'As T we show one site and hit the other.' }[a.t] + ' ' + { solid: 'As CT we hold a standard setup until they show a pattern.', read: 'As CT we stack the site they keep hitting.', aggressive: 'As CT we fight for mid early.' }[a.ct];
  return [
    '## Identity', id + ' Above all, we read what the opponent repeats and punish it.', '',
    '## 0. What to read in the Situation', '- "PISTOL ROUND" / "Overtime half start" / "Last round of the half".', '- "Your money: $X total" and "Full buy now costs your team about $Y (affordable | NOT affordable)".', '- "Opponent\'s last rounds as CT/T" (one line per round, oldest first) — their setup or strategy and the site they hit.', '- "Your last rounds as T/CT" — what we called and whether "we WON" or "we LOST".', '',
    '## 1. Economy (both sides, first match wins)', '1. Pistol round or overtime half start → "buy":"full".', '2. Full buy **affordable** → "buy":"full", "awp":true.', '3. Last round of the half → "buy":"force" (money resets anyway).',
    `4. Our money is at least **${force}%** of the full-buy cost → "buy":"force".`, '5. Otherwise → "buy":"eco": saving now buys a full buy next round. Never "half".', '- "awp" is true only on full buys.', '',
    '## 2. T side',
    '**First: never repeat a losing call.** Read "Your last rounds as T". If our last T round LOST, do not call that strategy again this round; if it was a fake or a hit on one site, go to the other site (fake_a_go_b → fake_b_go_a, rush_b → rush_a, execute_a → execute_b, …).',
    `**Then pick the counter.** Look at the opponent's CT setups in their last 3 rounds and take the one they used most (tie: the most recent). No CT rounds seen yet → "${tOpen}".`,
    '| Their setup | Our strategy |', '|---|---|', ...tRows, '',
    '**They adapted?** If our last two rounds as T both used the same strategy and both say "we LOST", counter only their most recent setup instead. If that also lost twice in a row → "default_mid".',
    ...{ fast: ['- Speed is our secret: if their last round showed 1 or fewer CTs on a site (the "(n A, n mid, n B)" numbers), rush that site instead of the table, "pace":"fast".'],
      methodical: ['- On full buys: "utility":"full" and "pace":"default" always; never rush on a full buy unless they played aggressive_mid.'],
      tricky: ['- Never hit the same site 3 rounds in a row: if the table would make it 3, pick the fake toward the other site (fake_a_go_b goes B, fake_b_go_a goes A).'] }[a.t], '',
    '**Buy-dependent details.**', '- Full buy: "utility":"full". Force: "utility":"smokes".', '- Eco: ignore the table → rush the site that had fewer CTs in their last round (rush_a only if A had fewer, else rush_b), "utility":"none".', '- "pace":"fast" for rushes, "default" for everything else.', '',
    '## 3. CT side',
    '**First: never repeat a losing setup.** If our last CT round LOST with stack_a or stack_b, do not stack the same site again this round.',
    { solid: '**Read their last 3 T rounds.** Only when they used the same strategy in **all 3** (or 2 in a row that both WON), answer it from this table:', read: '**Read their last 3 T rounds** and take the strategy they used most (2 of 3 is enough; tie: the most recent):', aggressive: '**Read their last 3 T rounds** and take the strategy they used most (2 of 3; tie: the most recent):' }[a.ct],
    '| Their strategy | Our setup |', '|---|---|', ...ctRows,
    `- Not enough to read yet (see above) → "${ctOpen}".`,
    ...{ solid: ['- A stack is a bet: after a lost round with stack_a or stack_b, go back to "standard_2_1_2" for the next round.'], read: ['- They hit the same site 2 rounds in a row, whatever the strategy name → stack that site.'], aggressive: ['- Pistol rounds and our full buys with an AWP: "aggressive_mid" unless they rushed in 2 of their last 3 rounds (then stack the site they rushed).'] }[a.ct],
    '**They adapted?** If our last two rounds as CT both used the same setup and both say "we LOST" → "standard_2_1_2".', '',
    '**Buy-dependent details.**', '- Utility: "heavy" on a full buy, "normal" on a force, "save" on an eco.', '',
    '## 4. Examples',
    exT ? `- R8, T, full buy affordable, they played ${exT} twice → {"buy":"full","strategy":"${tc[exT][0]}","utility":"full","pace":"default","awp":true}` : '',
    exCT ? `- R7, CT, full buy affordable, they played ${exCT} in R5 and R6 → {"buy":"full","setup":"${cc[exCT][0]}","utility":"heavy","awp":true}` : '',
    `- R3, T, $9,000 vs a $21,000 full buy (43%) → ${force <= 43 ? '{"buy":"force","strategy":"rush_b","utility":"smokes","pace":"fast","awp":false}' : '{"buy":"eco","strategy":"rush_b","utility":"none","pace":"fast","awp":false}'}`,
  ];
}

function lol(a, pb) {
  const st = section(pb, 'stance');
  const home = { early: 'engage', scaling: 'protect', pick: 'pick', siege: 'siege' }[a.comp];
  const sc = st ? counters(st, { [home]: 3 }) : {};
  const draft = {
    early: ['- Bans: late carries with dps3 first, then peel supports.', '- Picks: engage3 and early-curve champions. End with engage ≥ 5 and dps ≥ 9 (that combo gives +3%).', '- Curve: at least 3 early champions, no late ones.'],
    scaling: ['- Bans: engage tanks with engage3 + frontline3, then assassins with pick3.', '- Picks: a late dps3 carry first, then peel and frontline: end with peel ≥ 5 and the carry (+3%).', '- Curve: at least 2 late champions, no early ones.'],
    pick: ['- Bans: peel supports and engage tanks.', '- Picks: champions with pick3 (assassins, catch supports), plus engage for follow-up.', '- Curve: early or mid champions; do not mix with late ones.'],
    siege: ['- Bans: engage tanks with engage3 first, then assassins with pick3.', '- Picks: poke and waveclear: end with poke ≥ 7 and waveclear ≥ 9 (that combo gives +3%).', '- Curve: mid or late champions.'],
  }[a.comp];
  const lanes = {
    aggressive: ['- STRONGER or even lane → aggressive. WEAKER → even.', '- Jungle: gank the lane where I am most STRONGER; no stronger lane → gank_mid.'],
    read: ['- STRONGER lane → aggressive. WEAKER → passive. even → even.', '- Jungle: jungle STRONGER by ≥ 0.8 → invade. Otherwise gank the lane where I am most STRONGER (≥ 0.5). No stronger lane → farm.'],
    passive: ['- STRONGER lane → even. Everything else → passive.', '- Jungle: farm, unless a lane is STRONGER by ≥ 1.0 → gank that lane.'],
  }[a.lane];
  const obj = {
    dragons: ['1. I have BARON BUFF or ELDER BUFF → towers.', '2. Dragon or elder available and POWER NOW is EVEN or better (> −5%) → dragon (elder when it is up).', '3. SOUL POINT for either team and I am not CLEARLY BEHIND → dragon.', '4. Baron available and I am AHEAD → baron.', '5. Otherwise (behind) → towers on the other side of the map. Never start baron or elder while behind.'],
    adaptive: ['1. I have BARON BUFF or ELDER BUFF → towers.', '2. POWER NOW is EVEN or better (> −5%), or I counter their stance and am not CLEARLY BEHIND → the neutral objective they usually take (ENEMY OBJECTIVES SO FAR) if available, else elder > baron (baron only if AHEAD) > dragon > herald, else towers.', '3. SOUL POINT and the enemy has 3 drakes, and I am not CLEARLY BEHIND → dragon.', `4. ${a.comp === 'scaling' ? 'My comp is SCALING and "In 8 min" is better than now by 3+ points → farm.' : 'They scale better ("In 8 min" no better than now) and I am not CLEARLY BEHIND → take the neutral objective they do NOT usually take.'}`, '5. Otherwise (behind) → towers on the other side of the map. Never start baron or elder while behind.'],
    baron: ['1. I have BARON BUFF or ELDER BUFF → towers.', '2. Baron available and POWER NOW is AHEAD (≥ +5%) → baron.', '3. Before 20:00, or baron not up: herald if available, else towers.', '4. SOUL POINT for the enemy and I am not CLEARLY BEHIND → dragon.', '5. Otherwise → towers. Never start baron or elder while behind.'],
  }[a.obj];
  const rows = (st ? st.theirs.map(x => x.id) : []).filter(id => sc[id] && sc[id].length).map(id => `- they ${id} → ${sc[id].join(' or ')}`);
  const idText = { early: 'We draft engage and early champions and end the game before 25 minutes.', scaling: 'We draft a late carry with peel and frontline, survive the early game and win the late teamfights.', pick: 'We draft assassins and catch whoever walks alone.', siege: 'We draft poke and waveclear and take towers without fighting fair.' }[a.comp];
  return [
    '## Identity', `${idText} Our home stance is **${home}**. We still counter what the enemy does most.`, '', 'How to use this file: every rule names the line of the situation it reads (POWER NOW, LANE MATCHUPS, ENEMY STANCES SO FAR...). Go through the steps in order and stop at the first rule that applies. Quote that rule as your "rule".', '',
    '## 1. Draft', ...draft, '- Frontline: end with total frontline ≥ 5 (never < 3: that is −8% forever).', '- Damage: at least 2 AP and 2 AD.', '- Always list 6 names for bans: if one is gone, the next is used.', '',
    '## 2. Laning (00:00) — read LANE MATCHUPS', ...lanes, '',
    '## 3. First objective (08:00)', a.obj === 'baron' ? '- POWER NOW ≥ −3% → grubs (towers come faster). Otherwise → dragon.' : '- POWER NOW ≥ −3% → dragon. Otherwise → grubs (they go dragon; take the other side).', '',
    '## 4. Plates (12:00) — read "Lane gold leads"', '- Best lane lead > 800 and POWER NOW ≥ +5% → dive that lane.', '- Best lane lead > 0 → plates in that lane.', '- No lead anywhere → farm.', '',
    '## 5. Every 4 minutes: stance first, then objective', '### Stance — counter their most likely stance', '1. Predict: their most frequent stance in ENEMY STANCES SO FAR (the last ones count more). No history yet → the "best stance" in ENEMY COMP.', '2. Counters:', ...rows,
    '3. Among those counters, take the one with my highest stance fit, if it is ≥ 0.85.', `4. No usable counter → **${home}**, unless it loses to their stance; then my best-fit stance that does not.`, '',
    '### Objective — first rule that applies', ...obj,
  ];
}

const BUILD = { nfl, cs2, lol };

// El .md sencer per a unes respostes (a = {pregunta: opció}).
export function buildMD(game, a, pb, name) {
  const f = BUILD[game]; if (!f) return '';
  const ans = { ...DEFAULTS(game), ...(a || {}) };
  const title = (name || '').trim() || strategyName(game, ans);
  const body = f(ans, pb).filter(l => l !== undefined);
  // Fora les línies d'exemple que han quedat buides i els buits dobles.
  const out = [`# Strategy: "${title.replace(/"/g, "'")}"`, ''];
  for (const l of body) { if (l === '' && out[out.length - 1] === '') continue; if (l === '' || l) out.push(l); }
  while (out[out.length - 1] === '') out.pop();
  return out.join('\n') + '\n';
}

// El diàleg de l'assistent (el mostra arena.js).
export function builderHTML(game, a, title) {
  const qs = QUESTIONS[game] || [];
  return `<form method="dialog" class="bd">
    <header><div class="bd-kicker">Strategy builder</div><h3>${title}</h3>
      <p class="ar-muted">Three choices and you have a complete strategy. It always reads the opponent and counters what they repeat; your style decides how. You can edit every line afterwards.</p></header>
    ${qs.map((q, n) => `<fieldset class="bd-q" style="--d:${n * 80}ms"><legend><span class="bd-n">${n + 1}</span>${q.title}</legend><div class="bd-opts">${q.options.map(o => `
      <label class="bd-opt${a[q.id] === o.id ? ' on' : ''}"><input type="radio" name="${q.id}" value="${o.id}"${a[q.id] === o.id ? ' checked' : ''}>
        <span class="bd-ic">${o.icon}</span><b>${o.title}${o.rec ? ' <i>recommended</i>' : ''}</b><span class="bd-t">${o.text}</span>
        <span class="bd-pc"><span class="pro">▲ ${o.pro}</span><span class="con">▼ ${o.con}</span></span></label>`).join('')}</div></fieldset>`).join('')}
    <div class="bd-name"><label>Name <input name="name" maxlength="60" placeholder="${strategyName(game, a).replace(/"/g, '&quot;')}"></label></div>
    <details class="bd-preview"><summary>Preview the .md</summary><pre id="bdPreview"></pre></details>
    <footer><button type="button" value="cancel" class="bd-cancel">Cancel</button><button type="submit" value="ok" class="primary" autofocus>✨ Use this strategy</button></footer>
  </form>`;
}

export function builderCSS() {
  if (document.getElementById('bdCSS')) return;
  const st = document.createElement('style'); st.id = 'bdCSS';
  st.textContent = `
.bd-dialog{position:fixed;inset:0;margin:auto;height:fit-content;max-width:min(920px,95vw);width:920px;border:1px solid var(--line-hi,transparent);background:var(--surface,#fff);color:var(--ink,#111);border-radius:16px;padding:0;box-shadow:0 30px 90px rgba(0,0,0,.4);animation:bdPop .25s ease-out}
.bd-dialog::backdrop{background:rgba(10,14,30,.55);backdrop-filter:blur(3px)}
.bd{padding:20px 22px 16px;max-height:88vh;overflow:auto}
.bd header h3{margin:2px 0 4px;font-size:20px}
.bd-kicker{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--accent,#4f5bd5)}
.bd-q{border:0;padding:0;margin:16px 0 0;animation:bdIn .35s ease-out both;animation-delay:var(--d,0ms)}
.bd-q legend{font-weight:700;font-size:14.5px;display:flex;align-items:center;gap:8px;margin-bottom:8px}
.bd-n{display:inline-grid;place-items:center;width:22px;height:22px;border-radius:50%;background:var(--accent,#4f5bd5);color:#fff;font-size:12px}
.bd-opts{display:grid;grid-template-columns:repeat(auto-fit,minmax(190px,1fr));gap:8px}
.bd-opt{position:relative;display:flex;flex-direction:column;gap:4px;padding:12px 12px 10px;border:1.5px solid var(--line,#dde1ea);border-radius:12px;cursor:pointer;transition:border-color .15s,transform .15s,box-shadow .15s,background .15s}
.bd-opt:hover{transform:translateY(-2px);box-shadow:0 6px 18px rgba(20,30,60,.12)}
.bd-opt.on{border-color:var(--accent,#4f5bd5);background:color-mix(in srgb,var(--accent,#4f5bd5) 7%,transparent);box-shadow:0 0 0 3px color-mix(in srgb,var(--accent,#4f5bd5) 18%,transparent)}
.bd-opt input{position:absolute;opacity:0;pointer-events:none}
.bd-opt:has(input:focus-visible){outline:2px solid var(--accent,#4f5bd5);outline-offset:2px}
.bd-ic{font-size:24px;line-height:1}
.bd-opt b{font-size:13.5px}
.bd-opt b i{font-style:normal;font-size:10px;font-weight:700;text-transform:uppercase;letter-spacing:.06em;color:#1f9d5a;margin-left:4px}
.bd-t{font-size:12.5px;line-height:1.4;opacity:.85}
.bd-pc{display:flex;flex-direction:column;gap:1px;font-size:11.5px;margin-top:auto;padding-top:4px}
.bd-pc .pro{color:#1f9d5a}.bd-pc .con{color:#d9453c}
.bd-name{margin-top:14px}.bd-name input{width:100%;max-width:360px;margin-left:6px}
.bd-preview{margin-top:10px}.bd-preview pre{max-height:260px;overflow:auto;font-size:11.5px;background:var(--code-bg,rgba(127,127,127,.08));padding:10px;border-radius:8px;white-space:pre-wrap}
.bd footer button{font:inherit;font-size:14px;padding:9px 16px;border-radius:10px;border:1px solid var(--line,#d6dae4);background:transparent;color:inherit;cursor:pointer}
.bd footer button.primary{background:var(--accent,#4f5bd5);border-color:var(--accent,#4f5bd5);color:#fff;font-weight:700;box-shadow:0 6px 18px color-mix(in srgb,var(--accent,#4f5bd5) 35%,transparent)}
.bd footer button.primary:hover{filter:brightness(1.08)}
.bd-name label{font-size:13px;font-weight:600;display:flex;align-items:center;gap:8px}
.bd-name input{font:inherit;font-weight:400;padding:7px 10px;border-radius:8px;border:1px solid var(--line,#d6dae4);background:transparent;color:inherit;margin-left:0!important}
.bd-preview summary{cursor:pointer;font-size:13px;color:var(--muted,#667)}
.bd footer{display:flex;justify-content:flex-end;gap:8px;margin-top:14px;position:sticky;bottom:-16px;padding:10px 0 4px;background:var(--surface,#fff)}
@keyframes bdPop{from{opacity:0;transform:translateY(10px) scale(.98)}to{opacity:1;transform:none}}
@keyframes bdIn{from{opacity:0;transform:translateY(6px)}to{opacity:1;transform:none}}
@media (prefers-reduced-motion:reduce){.bd-dialog,.bd-q{animation:none}.bd-opt{transition:none}}
`;
  document.head.append(st);
}
