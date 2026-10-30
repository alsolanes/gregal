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
