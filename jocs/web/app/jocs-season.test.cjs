// La temporada per persones (jocs-season.js): HTML escapat, el radar i els moments.
const test = require('node:test');
const assert = require('node:assert');
const path = require('path');
const { pathToFileURL } = require('url');

const load = () => import(pathToFileURL(path.join(__dirname, 'jocs-season.js')).href);

test('moment card: escapes, tidies table rows and links the play', async () => {
  const { momentCard } = await load();
  const html = momentCard({ owner: 'anna', display: '<Anna>', rival: 'Bot Rookie', score: [21, 14], rule: '| blitz | screen to rb |', what: 'TD', why: '', game: 'nfl', kind: 'official', stage: 'Final', match: 'm1', event: 42, lead: true, clutch: true, won: true }, true);
  assert.ok(html.includes('&lt;Anna&gt;') && !html.includes('<Anna>'));
  assert.ok(html.includes('blitz  ·  screen to rb'));
  assert.ok(html.includes('data-moment="m1"') && html.includes('data-at="42"'));
  assert.ok(html.includes('Go-ahead') && html.includes('Clutch') && html.includes('Final'));
});

test('radar: five axes, a shape only with scores', async () => {
  const { radarSVG, PRINCIPLES } = await load();
  assert.strictEqual(PRINCIPLES.length, 5);
  assert.ok(!radarSVG(null).includes('class="shape"'));
  const svg = radarSVG({ clarity: 5, specificity: 1, coverage: 3, adaptivity: 2, consistency: 4 });
  assert.ok(svg.includes('class="shape"') && svg.includes('Clarity') && svg.includes('5.0'));
  assert.ok(!/NaN/.test(svg));
});

test('standings and highlights: empty states and the season hero', async () => {
  const { standingsHTML, highlightsHTML, sparkSVG } = await load();
  assert.ok(standingsHTML({ players: [] }, 'me').includes('No players yet'));
  const p = { owner: 'me', display: 'Me', rank: 1, games: { nfl: { bots: ['rookie', 'aggro'] } }, official: [0, 0, 0], elo: 0, reviewed: 0, followed: 0, form: ['W'] };
  const t = standingsHTML({ players: [p] }, 'me');
  assert.ok(t.includes('practice ladder') && t.includes('YOU') && (t.match(/class="on"/g) || []).length === 2);
  assert.ok(highlightsHTML({ highlights: [] }).includes('No prompt moments'));
  const h = { owner: 'me', display: 'Me', rival: 'B', score: [1, 0], rule: 'r', what: 'w', game: 'nfl', kind: 'practice', match: 'x', event: 1 };
  assert.ok(highlightsHTML({ highlights: [h] }, 'all').includes('only you see'));
  assert.ok(highlightsHTML({ highlights: [{ ...h, kind: 'official' }] }, 'all').includes('Moment of the season'));
  assert.strictEqual(sparkSVG([]), '');
  assert.ok(sparkSVG([[10, 1516], [20, 1531]]).includes('class="jx-spark up"'));
});

test('estimate: same groups as the server, faster with more slots', async () => {
  const { estimateComp, plannerHTML } = await load();
  assert.deepStrictEqual(estimateComp(1, 1, 480), { matches: 0, seconds: 0, groups: 0 });
  assert.strictEqual(estimateComp(5, 1, 480).groups, 1);   // un grup de 5
  assert.strictEqual(estimateComp(7, 1, 480).groups, 2);   // 4 + 3
  assert.strictEqual(estimateComp(40, 1, 480).groups, 10);
  assert.ok(estimateComp(40, 4, 480).seconds < estimateComp(40, 1, 480).seconds / 2.5);
  const html = plannerHTML({ entries: { nfl: 3 }, bots: { nfl: 4, cs2: 4 }, match_secs: 480 }, 2, true, [{ id: 'nfl' }, { id: 'cs2' }]);
  assert.ok(html.includes('3</b> players + 4 bots') && html.includes('left out'));
});
