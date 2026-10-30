// Tria el visor de cada joc de l'Arena i li dona les dades del partit
// (/api/arena/match). El futbol americà fa servir el visor de football-view.js
// amb les jugades que porten els esdeveniments; CS2 i LoL tenen el seu.

// source(fetchMatch): fetchMatch(after) → Promise del JSON del partit.
export async function makeView(game, canvas, { lang, fetchMatch, onUpdate, onEvent }) {
  if (game === 'nfl') {
    const { FootballView } = await import('./football-view.js');
    let teams = null;
    const source = {
      load: async after => {
        const m = await fetchMatch(after);
        if (m.meta && m.meta.teams) teams = m.meta.teams.map((t, i) => ({ ...t, name: (m.names && m.names[i]) || t.name }));
        const plays = (m.events || []).map(e => ({ ...(e.data || {}), n: e.n, decision: e.decision, comment: '' }));
        return { ...m, teams, plays, commentator: '' };
      },
    };
    return new FootballView(canvas, { lang, source, onUpdate, onEvent });
  }
  const opts = { lang, source: { load: fetchMatch }, onUpdate, onEvent };
  if (game === 'cs2') { const { Cs2View } = await import('./arena-cs2-view.js'); return new Cs2View(canvas, opts); }
  if (game === 'lol') { const { LolView } = await import('./arena-lol-view.js'); return new LolView(canvas, opts); }
  throw new Error('unknown game ' + game);
}

// Lliçons per escriure un bon .md (es mostren al costat de l'editor).
export const LESSONS = {
  ca: [
    ['Comença per la identitat', 'Una frase que digui quin estil de joc vols. El model la fa servir quan cap regla no encaixa.'],
    ['Regles si/llavors', '«Si el rival fa X més del 30% de cops, fes Y.» Les regles concretes guanyen a les intencions («juga intel·ligent»).'],
    ['Llegeix el rival', 'La situació porta el que el rival ha fet fins ara. Digues què cal mirar i com respondre-hi: és on es guanyen els partits.'],
    ['Prioritats clares', 'Quan dues regles xoquen, quina mana? Escriu l’ordre. Les contradiccions fan que el model triï a l’atzar.'],
    ['Casos especials', 'Final de partit, anar per davant o per darrere, recursos baixos: són els moments que decideixen.'],
    ['Exemples', 'Un o dos exemples de situació → decisió ensenyen més que un paràgraf.'],
    ['Curt i escanejable', 'Títols, llistes i frases curtes. El model llegeix el .md a cada decisió: el que és fàcil de trobar es fa servir.'],
    ['Itera amb dades', 'Juga contra els bots, llegeix l’informe i les decisions amb la línia que les va causar, i canvia una cosa cada cop.'],
  ],
  en: [
    ['Start with an identity', 'One sentence saying what style you want. The model falls back on it when no rule fits.'],
    ['If/then rules', '“If the opponent does X more than 30% of the time, do Y.” Concrete rules beat intentions (“play smart”).'],
    ['Read the opponent', 'The situation includes what the opponent has done so far. Say what to look at and how to answer it: that is where games are won.'],
    ['Clear priorities', 'When two rules clash, which one wins? Write the order. Contradictions make the model choose at random.'],
    ['Special cases', 'End of the game, leading or trailing, low resources: these moments decide matches.'],
    ['Examples', 'One or two situation → decision examples teach more than a paragraph.'],
    ['Short and scannable', 'Headings, lists and short sentences. The model reads the .md at every decision: what is easy to find gets used.'],
    ['Iterate with data', 'Play the bots, read the report and the decisions with the line that caused them, and change one thing at a time.'],
  ],
};

// Explicacions de cada joc per a la retransmissió (el primer cop que surt).
export const EXPLAIN = {
  nfl: {
    ca: ['Futbol americà, regles de la NFL', 'A cada jugada, el model llegeix el .md i tria la jugada d’atac (córrer, passar curt o llarg, screen, play-action, xutar) i la defensa (blitz, cobertures).', 'Guanya qui encerta el duel: un screen contra un blitz, córrer contra una defensa tirada enrere, tancar la llarga quan el rival hi va sempre.', 'Mireu la línia groga (el primer down) i com cada equip llegeix el que l’altre ha fet fins ara.'],
    en: ['American football, NFL rules', 'Every play, the model reads the .md and picks the offensive play (run, short or deep pass, screen, play-action, kick) and the defense (blitz, coverages).', 'Whoever wins the matchup wins: a screen against a blitz, a run against a soft defense, taking away the deep ball from a team that always throws it.', 'Watch the yellow line (first down) and how each team reads what the other has done so far.'],
  },
  cs2: {
    ca: ['Counter-Strike 2, competitiu (primer a 13)', 'Cada ronda, el model llegeix el .md i decideix la compra (eco, force, full), l’estratègia (rush, execució, fake, split) i la utilitat; els CT, com defensen els punts A i B.', 'L’economia és la clau: saber quan estalviar, i llegir on ataca el rival per apilar-hi defensors o enganyar-lo amb un fake.', 'Mireu els diners de cada equip i el que cada .md fa després de perdre una ronda.'],
    en: ['Counter-Strike 2, competitive (first to 13)', 'Each round, the model reads the .md and decides the buy (eco, force, full), the strategy (rush, execute, fake, split) and utility; the CTs decide how to hold A and B.', 'Economy is everything: knowing when to save, and reading where the opponent attacks to stack it or fool them with a fake.', 'Watch each team’s money and what each .md does after losing a round.'],
  },
  lol: {
    ca: ['League of Legends, Summoner’s Rift 5 contra 5', 'Primer el draft: el model tria i prohibeix campions seguint el .md. Després, a cada fase, decideix l’objectiu (drac, herald, baró, torres) i la postura (iniciar, assetjar, dividir, protegir).', 'Guanya qui fa un equip que encaixa i el juga al seu moment: fort al principi, ataca; fort al final, aguanta i escala.', 'Mireu el gràfic d’or i qui pren els objectius grans.'],
    en: ['League of Legends, Summoner’s Rift 5v5', 'First the draft: the model picks and bans champions following the .md. Then, each phase, it chooses the objective (dragon, herald, baron, towers) and the stance (engage, siege, split, protect).', 'The winner builds a team that fits and plays it at the right time: strong early, attack; strong late, stall and scale.', 'Watch the gold graph and who takes the big objectives.'],
  },
};

// Guia de cada joc per a qui escriu el .md: què decideix el model per tu (amb
// les opcions exactes del motor), què veu a cada decisió i una regla d'exemple.
export const GUIDE = {
  nfl: {
    sees: 'Down, distance, field position, score, clock, timeouts, and what the other team has called so far (their run/pass mix, favourite defences).',
    decides: [
      ['Every offensive snap', 'play: run_inside, run_outside, qb_sneak, pass_short, pass_medium, pass_deep, screen, play_action, punt, field_goal, kneel, spike · target: wr1, wr2, wr3, te, rb · tempo: normal, hurry, milk'],
      ['Every defensive snap', 'call: base, blitz, cover2, cover3, man, goal_line, prevent'],
    ],
    example: '- If they run more than 55% of plays: call goal_line on 1st down, cover3 otherwise.\n- 3rd & 7+: pass_medium to the open side; vs blitz, screen to the rb.\n- Leading by 8+ in the 4th quarter: run_inside, tempo milk.',
  },
  cs2: {
    sees: 'Round, score, side (T or CT), your money and the enemy’s likely money, who has the AWP, and where the other team attacked or stacked in past rounds.',
    decides: [
      ['Each round as T', 'buy: eco, half, force, full · strategy: rush_a/b, execute_a/b, split_a/b, default_mid, fake_a_go_b, fake_b_go_a · utility: none, smokes, flash_heavy, full · pace · awp'],
      ['Each round as CT', 'buy: eco, half, force, full · setup: standard_2_1_2, stack_a, stack_b, aggressive_mid, retake_a, retake_b · utility: save, normal, heavy · awp'],
    ],
    example: '- If we have under $3,500 each after a loss: eco, unless it is the last round of the half.\n- If they hit B twice in a row: stack_b as CT.\n- When they stack a site: fake it and go to the other.',
  },
  lol: {
    sees: 'Draft so far (picks, bans, roles), gold and kills, towers and dragons, which objectives are up, and what the other team chose in earlier phases.',
    decides: [
      ['Draft', 'bans and picks: the champion names on screen (tanks, engage, poke, scaling carries…)'],
      ['00:00 laning', 'top/mid/bot: aggressive, even, passive · jungle: gank_top, gank_mid, gank_bot, farm, invade'],
      ['08:00 & 12:00', 'objective: dragon, grubs, farm · then focus lane + plates, dive, roam, farm'],
      ['Every 4 minutes after', 'objective: dragon, herald, baron, elder, towers, farm · stance: engage, siege, pick, split, protect'],
    ],
    example: '- Draft: if they pick two early-game champions, ban their jungler and pick scaling carries.\n- Behind by 3k+ gold: farm safe lanes and pick off overextenders; never start baron.\n- Ahead with soul point: take dragon with stance engage.',
  },
};
