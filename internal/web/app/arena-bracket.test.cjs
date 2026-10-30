const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const load = () => import(pathToFileURL(path.join(__dirname, 'arena-bracket.js')).href);

// Una competició com la que torna /api/arena/competition: dos grups,
// semifinals creuades i final (al millor de 3).
function twoGroups() {
  const g = (stage, a, b, winner, score) => ({ game: 'lol', stage, phase: 'group', a, b, winner, score, done: true });
  return {
    title: 'Cup', games: ['lol'], status: 'running', current: 7,
    names: { p1: 'Anna', p2: 'Bernat', p3: 'Carla', p4: 'Dídac', p5: 'Eva', p6: 'Ferran' },
    tables: { lol: [{ stage: 'Group A', rows: [{ owner: 'p1', played: 2, won: 2, drawn: 0, lost: 0, pts: 6 }, { owner: 'p2', played: 2, won: 1, drawn: 0, lost: 1, pts: 3 }, { owner: 'p3', played: 2, won: 0, drawn: 0, lost: 2, pts: 0 }] },
      { stage: 'Group B', rows: [{ owner: 'p4', played: 2, won: 2, drawn: 0, lost: 0, pts: 6 }, { owner: 'p5', played: 2, won: 1, drawn: 0, lost: 1, pts: 3 }, { owner: 'p6', played: 2, won: 0, drawn: 0, lost: 2, pts: 0 }] }] },
    schedule: [
      g('Group A', 'p1', 'p2', 0, [1, 0]), g('Group A', 'p1', 'p3', 0, [1, 0]), g('Group A', 'p2', 'p3', 0, [1, 0]),
      g('Group B', 'p4', 'p5', 0, [1, 0]), g('Group B', 'p4', 'p6', 0, [1, 0]), g('Group B', 'p5', 'p6', 0, [1, 0]),
      { game: 'lol', stage: 'Semifinal 1', phase: 'playoff', a: 'p1', b: 'p5', from: ['A1', 'B2'], series: [2, 1], score: [2, 1], matches: ['m1', 'm2', 'm3'], winner: 0, done: true },
      { game: 'lol', stage: 'Semifinal 2', phase: 'playoff', a: 'p4', b: 'p2', from: ['B1', 'A2'], series: [1, 0], matches: ['m4'], match: 'm5', winner: -1, done: false },
      { game: 'lol', stage: 'Final', phase: 'playoff', a: 'p1', b: '', from: ['W:Semifinal 1', 'W:Semifinal 2'], pending: true, winner: -1, done: false },
    ],
  };
}

test('seed labels and short stage names', async () => {
  const B = await load();
  assert.equal(B.seedLabel('A1'), '1st Group A');
  assert.equal(B.seedLabel('B2'), '2nd Group B');
  assert.equal(B.seedLabel('W:Semifinal 1'), 'Winner SF1');
  assert.equal(B.seedLabel('W:Quarterfinal 3'), 'Winner QF3');
  assert.equal(B.seedLabel(''), 'TBD', 'a missing seed (a bye) is still to be decided');
  assert.equal(B.seedLabel('A1', 'ca'), '1r grup A');
  assert.equal(B.seedLabel('W:Final', 'ca'), 'Guanyador Final');
  assert.equal(B.shortStage('Final'), 'Final');
  assert.equal(B.roundOf('Semifinal 2'), 'Semifinal');
  assert.equal(B.roundOf('Group A'), '');
});

test('bracket layout: rounds in columns, each match between the two that feed it', async () => {
  const B = await load();
  const c = twoGroups();
  // Out of order on purpose: the layout sorts by round and number.
  c.schedule.push(c.schedule.splice(6, 1)[0]);
  c.current = 6; // Semifinal 2 has moved up one place
  const L = B.bracketLayout(c, 'lol', { cardW: 200, cardH: 60, gapX: 50, gapY: 20, top: 30 });
  assert.deepEqual(L.rounds.map(r => r.round), ['Semifinal', 'Final']);
  const [sf1, sf2] = L.rounds[0].cards, fin = L.rounds[1].cards[0];
  assert.deepEqual([sf1.stage, sf2.stage], ['Semifinal 1', 'Semifinal 2']);
  assert.equal(sf1.x, 0); assert.equal(fin.x, 250);
  assert.ok(sf2.y > sf1.y + 60, 'the semifinals do not overlap');
  assert.equal(fin.y, (sf1.y + sf2.y) / 2, 'the final sits halfway between its semifinals');
  assert.deepEqual(L.links.map(k => k.from + '>' + k.to), ['Semifinal 1>Final', 'Semifinal 2>Final']);
  assert.equal(L.links[0].x1, 200); assert.equal(L.links[0].x2, 250);
  assert.equal(L.height, 30 + 2 * 60 + 20);
  // A walkover (no rival, already decided) says so instead of «TBD».
  const bye = B.slotSides(c, { phase: 'playoff', a: 'p1', b: '', from: ['A1', ''], done: true, winner: 0 });
  assert.deepEqual(bye.map(x => x.name), ['Anna', 'Bye']);
  assert.ok(sf2.live && !sf1.live && !fin.live, 'the current match is the live one');
  assert.equal(L.champion, '');
});

test('eight teams: quarterfinals, semis and final line up', async () => {
  const B = await load();
  const seeds = ['A1', 'D2', 'C1', 'B2', 'B1', 'C2', 'D1', 'A2'];
  const schedule = [0, 1, 2, 3].map(i => ({ game: 'cs2', stage: `Quarterfinal ${i + 1}`, phase: 'playoff', a: '', b: '', from: [seeds[2 * i], seeds[2 * i + 1]], pending: true, winner: -1 }))
    .concat([{ game: 'cs2', stage: 'Semifinal 1', phase: 'playoff', from: ['W:Quarterfinal 1', 'W:Quarterfinal 2'], pending: true, winner: -1 },
      { game: 'cs2', stage: 'Semifinal 2', phase: 'playoff', from: ['W:Quarterfinal 3', 'W:Quarterfinal 4'], pending: true, winner: -1 },
      { game: 'cs2', stage: 'Final', phase: 'playoff', from: ['W:Semifinal 1', 'W:Semifinal 2'], pending: true, winner: -1 },
      { game: 'nfl', stage: 'Final', phase: 'playoff', from: ['A1', 'A2'], pending: true, winner: -1 }]);
  const c = { games: ['cs2', 'nfl'], status: 'ready', current: 0, schedule, names: {} };
  const L = B.bracketLayout(c, 'cs2');
  assert.deepEqual(L.rounds.map(r => r.cards.length), [4, 2, 1], 'only the cs2 slots');
  const [q1, q2, q3, q4] = L.rounds[0].cards, [s1, s2] = L.rounds[1].cards, f = L.rounds[2].cards[0];
  assert.ok(q1.y < q2.y && q2.y < q3.y && q3.y < q4.y);
  assert.equal(s1.y, (q1.y + q2.y) / 2); assert.equal(s2.y, (q3.y + q4.y) / 2); assert.equal(f.y, (s1.y + s2.y) / 2);
  assert.equal(L.links.length, 6);
  // Without rivals yet, the cards say where they come from.
  const sides = B.slotSides(c, schedule[0]);
  assert.deepEqual(sides.map(s => s.name), ['1st Group A', '2nd Group D']);
  assert.ok(sides.every(s => s.pending && s.score === null));
  const svg = B.bracketSVG(c, 'cs2');
  assert.match(svg, /^<svg class="abk-svg"/);
  assert.match(svg, /Winner QF1/);
  assert.match(svg, /Quarterfinals/);
  // A single group: just a final, centred.
  const one = B.bracketLayout(c, 'nfl');
  assert.equal(one.rounds.length, 1);
  assert.equal(B.bracketLayout(c, 'lol').rounds.length, 0, 'no playoffs, no bracket');
  assert.equal(B.bracketSVG(c, 'lol'), '');
});

test('series scores, winners and the champion', async () => {
  const B = await load();
  const c = twoGroups();
  const [a, b] = B.slotSides(c, c.schedule[6]);
  assert.deepEqual([a.name, a.score, a.won, b.name, b.score, b.lost], ['Anna', 2, true, 'Eva', 1, true]);
  const fin = B.slotSides(c, c.schedule[8]);
  assert.deepEqual(fin.map(s => s.name), ['Anna', 'Winner SF2'], 'one rival known, the other still to come');
  c.schedule[8] = { ...c.schedule[8], b: 'p4', pending: false, series: [2, 0], score: [2, 0], winner: 0, done: true };
  c.champion = { lol: 'p1' }; c.status = 'done';
  const L = B.bracketLayout(c, 'lol');
  assert.equal(L.champion, 'p1');
  assert.ok(L.width > 2 * 230 + 56, 'room for the champion');
  const svg = B.bracketSVG(c, 'lol');
  assert.match(svg, /🏆 Champion/);
  assert.match(svg, /class="abk-nm abk-won"[^>]*>Anna/);
  assert.ok(!/LIVE/.test(svg), 'nothing is live once it is done');
  // Names are escaped.
  c.names.p1 = '<b>Anna</b>';
  assert.ok(!B.bracketSVG(c, 'lol').includes('<b>Anna'));
});

test('schedule: played, live and next', async () => {
  const B = await load();
  const c = twoGroups();
  const s = B.scheduleList(c, { max: 3 });
  assert.deepEqual(s.live.map(x => x.stage), ['Semifinal 2']);
  assert.equal(s.live[0].games, 2, 'game 2 of the series');
  assert.deepEqual(s.live[0].sides.map(x => x.score), [1, 0]);
  assert.deepEqual(s.next.map(x => x.stage), ['Final']);
  assert.equal(s.past.length, 3, 'only the last few');
  assert.equal(s.past[2].stage, 'Semifinal 1');
  const html = B.scheduleHTML(c);
  assert.match(html, /Live now/);
  assert.match(html, /Up next/);
  assert.ok(html.indexOf('Semifinal') < 0 || /SF2/.test(html));
  // Group tables: the top two go through.
  const g = B.groupsHTML(c);
  assert.equal((g.match(/class="abk-group"/g) || []).length, 2);
  assert.equal((g.match(/<tr class="q">/g) || []).length, 4);
  assert.match(B.groupsHTML(c, { stage: 'Group B' }), /Dídac/);
  assert.ok(!B.groupsHTML(c, { stage: 'Group B' }).includes('Anna'));
  assert.match(B.groupsHTML(c, { lang: 'ca' }), /Grup A/);
  assert.ok(B.playoffsStarted(c, 'lol'));
  assert.equal(B.currentGame(c), 'lol');
  // Groups finished but the playoffs not drawn yet: still playoff time.
  const c2 = twoGroups();
  c2.schedule.slice(6).forEach(x => { x.pending = true; x.a = x.b = ''; x.done = false; x.matches = []; delete x.match; x.series = undefined; });
  assert.ok(B.playoffsStarted(c2, 'lol'));
  c2.schedule[0].done = false;
  assert.ok(!B.playoffsStarted(c2, 'lol'), 'groups still running');
  assert.match(B.scheduleHTML({ schedule: [] }), /Nothing yet/);
});

test('a broadcast running behind the server does not spoil results', async () => {
  const B = await load();
  const c = twoGroups();
  // Server: SF2 done 2-1 for p4 (m4, m5, m6), the final p1 vs p4 done, champion p1.
  Object.assign(c.schedule[7], { series: [2, 1], score: [2, 1], matches: ['m4', 'm5', 'm6'], winner: 0, done: true });
  delete c.schedule[7].match;
  Object.assign(c.schedule[8], { b: 'p4', pending: false, series: [2, 0], score: [2, 0], matches: ['m7', 'm8'], winner: 0, done: true });
  c.champion = { lol: 'p1' }; c.status = 'done'; c.current = 8;
  assert.equal(B.maskUnseen(c, new Set()), c, 'nothing unseen: the same object');
  // The TV has shown SF2 game 1 (p4 won) only.
  const winners = new Map([['m5', 'p2'], ['m6', 'p4'], ['m7', 'p1'], ['m8', 'p1']]);
  const m = B.maskUnseen(c, new Set(['m5', 'm6', 'm7', 'm8']), winners);
  assert.notEqual(m, c);
  const sf2 = m.schedule[7], fin = m.schedule[8];
  assert.deepEqual(sf2.series, [1, 0], 'only the game already shown counts');
  assert.ok(!sf2.done && sf2.winner === -1);
  assert.equal(fin.b, '', 'the SF2 winner is not in the final yet');
  assert.ok(fin.pending && !fin.done && fin.series[0] === 0);
  assert.equal(m.champion.lol, undefined, 'no champion yet');
  assert.equal(m.current, 7); assert.equal(m.status, 'running');
  assert.ok(B.isLive(m, 7));
  assert.deepEqual(sf2.matches, ['m4']); assert.equal(sf2.match, 'm5');
  assert.equal(B.scheduleList(m).live[0].games, 2, 'game 2 is the next one to watch');
  assert.match(B.bracketSVG(m, 'lol'), /SF2 · Game 2/);
  assert.deepEqual(B.slotSides(m, fin).map(s => s.name), ['Anna', 'Winner SF2']);
  // The original is untouched.
  assert.equal(c.champion.lol, 'p1'); assert.deepEqual(c.schedule[7].series, [2, 1]);
  // Only the final's last game unseen: the final is 1-0 and live, the semis stay decided.
  const m2 = B.maskUnseen(c, new Set(['m8']), winners);
  assert.deepEqual(m2.schedule[8].series, [1, 0]);
  assert.equal(m2.schedule[8].b, 'p4');
  assert.ok(m2.schedule[7].done);
  assert.equal(m2.champion.lol, undefined);
});
