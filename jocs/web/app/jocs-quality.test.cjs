const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const load = f => import(pathToFileURL(path.join(__dirname, f)).href);

const Q = {
  vs_vague: { played: 8, won: 6 }, people: { played: 4, won: 3 },
  rungs: [
    { owner: 'bot:rookie', name: 'Bot Rookie', level: 'rookie', overall: { played: 8, won: 2 } },
    { owner: 'bot:aggro', name: 'Bot Aggro', level: 'solid', overall: { played: 4, won: 2 } },
    { owner: 'bot:turtle', name: 'Bot Turtle', level: 'solid', overall: { played: 2, won: 2 } },
    { owner: 'bot:analyst', name: 'Bot Analyst', level: 'expert', overall: { played: 3, won: 3 } },
  ],
};
const PLAYERS = [
  { owner: 'ana', display: 'Ana', vs_bots: { rookie: { played: 2, won: 2 }, expert: { played: 2, won: 0 } } },
  { owner: 'joan', display: 'Joan', vs_bots: { rookie: { played: 3, won: 1 }, solid: { played: 1, won: 1 } } },
];

test('The headline needs enough matches', async () => {
  const Mq = await load('jocs-quality.js');
  assert.deepEqual(Mq.headline(Q), { pct: 75, played: 8, people: 75 });
  assert.equal(Mq.headline({ vs_vague: { played: 2, won: 2 } }), null);
  assert.equal(Mq.headline(null), null);
});

test('The ladder groups bots by level and lists who beat each', async () => {
  const Mq = await load('jocs-quality.js');
  const [expert, solid, rookie] = Mq.ladder(Q, PLAYERS);
  assert.equal(expert.winRate, 100);
  assert.deepEqual(solid.bots, ['Bot Aggro', 'Bot Turtle']);
  assert.equal(solid.winRate, 67);
  assert.deepEqual(rookie.beat.map(p => p.owner), ['ana', 'joan'], 'best record first');
  assert.equal(expert.beat.length, 0);
});

test('The section and the compact card render', async () => {
  const Mq = await load('jocs-quality.js');
  const html = Mq.qualityHTML(Q, PLAYERS, 'ana');
  assert.match(html, /75%/);
  assert.match(html, /Vague prompt/);
  assert.match(html, /class="jx-q-chip me"/);
  assert.match(Mq.qualityHTML(Q, PLAYERS, 'ana', { compact: true }), /Better prompts win/);
  assert.equal(Mq.qualityHTML({ vs_vague: { played: 1, won: 1 } }, [], '', { compact: true }), '');
});

test('Before/after compares the first and the latest version', async () => {
  const Mq = await load('jocs-quality.js');
  const p = { games: {
    nfl: { versions: [{ n: 1, played: 4, won: 1, coach: 40, lines: 6 }, { n: 2, played: 2, won: 1, lines: 20 }, { n: 3, played: 4, won: 3, coach: 76, lines: 32 }] },
    cs2: { versions: [{ n: 1, played: 1, won: 0, lines: 4 }] },
  } };
  assert.equal(Mq.bestVersions(p).game, 'nfl');
  const html = Mq.beforeAfterHTML(p, g => g.toUpperCase());
  assert.match(html, /Before \/ after · NFL/);
  assert.match(html, /\+50 pts/);
  assert.match(html, /\+36 coach/);
  assert.match(Mq.beforeAfterHTML({ games: {} }), /Change the \.md/);
});
