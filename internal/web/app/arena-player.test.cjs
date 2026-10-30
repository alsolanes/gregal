const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const load = f => import(pathToFileURL(path.join(__dirname, f)).href);

test('playback time: mm:ss, the event at a time, and the ⏪/⏩ targets', async () => {
  const S = await load('arena-scrubber.js');
  assert.equal(S.fmtTime(0), '0:00');
  assert.equal(S.fmtTime(75.9), '1:15');
  assert.equal(S.fmtTime(3725), '1:02:05');
  assert.equal(S.fmtTime(-3), '0:00');
  const times = [0, 4, 10, 10, 18, 30];
  assert.equal(S.indexAtTime(times, -1), 0);
  assert.equal(S.indexAtTime(times, 0), 0);
  assert.equal(S.indexAtTime(times, 9.9), 1);
  assert.equal(S.indexAtTime(times, 10), 3, 'two events at the same second: the later one');
  assert.equal(S.indexAtTime(times, 29), 4);
  assert.equal(S.indexAtTime(times, 99), 4, 'never past the last event');
  // With playback time: ±15 s like a video.
  const pr = { index: 5, total: 5, times, duration: 30, elapsed: 25 };
  assert.equal(S.backTarget(pr), 3, 'the event playing 15 s ago');
  assert.equal(S.forwardTarget(pr), 5, 'past the end: the end');
  assert.equal(S.forwardTarget({ ...pr, index: 1, elapsed: 2 }), 4, 'the first event at or after 17 s');
  assert.equal(S.backTarget({ ...pr, index: 1, elapsed: 2 }), 0);
  // A long event: ⏩ still moves on.
  assert.equal(S.forwardTarget({ index: 1, total: 2, times: [0, 100, 101], duration: 101, elapsed: 3 }), 1);
  // Without playback time: by chapters (rounds) or a few events.
  const ch = { index: 30, total: 60, chapters: [0, 20, 40] };
  assert.equal(S.backTarget(ch), 20);
  assert.equal(S.backTarget({ ...ch, index: 22 }), 0, 'right after a round start: the previous round');
  assert.equal(S.forwardTarget(ch), 40);
  assert.equal(S.forwardTarget({ ...ch, index: 50 }), 60);
  assert.equal(S.backTarget({ index: 12, total: 40, step: 5 }), 6);
  assert.equal(S.forwardTarget({ index: 12, total: 40, step: 5 }), 16);
  assert.equal(S.backTarget({ index: 0, total: 40 }), 0);
});

test('the commentary track: moments, scores, big plays and caster lines, with calls and rules', async () => {
  const P = await load('arena-player.js');
  const events = [
    { n: 1, kind: 'kickoff', text: 'Kickoff', score: [0, 0], decision: 1 },
    { n: 2, kind: 'run', text: 'Run for 4', score: [0, 0], decision: 1 },
    { n: 3, kind: 'pass', text: 'Deep pass, TOUCHDOWN', score: [6, 0], decision: 2, hl: true },
    { n: 4, kind: 'pass', text: 'Interception!', score: [6, 0], decision: 3, hl: true },
    { n: 5, kind: 'fg', text: 'Field goal is good', score: [6, 3], decision: 4 },
    { n: 6, kind: 'run', text: 'Kneel', score: [6, 3], decision: 5 },
  ];
  const times = [0, 5, 11, 20, 27, 33, 38];
  const decisions = [
    { n: 2, side: 0, decision: { play: 'pass_deep', target: 'wr1' }, rule: 'If they blitz, go deep to wr1.' },
    { n: 2, side: 1, decision: { call: 'blitz' }, rule: 'Blitz on 2nd down.' },
    { n: 4, side: 1, decision: { play: 'field_goal' }, rule: '' },
  ];
  const moments = new Map([[3, { n: 2, side: 0, rule: 'If they blitz, go deep to wr1.', what: 'Deep touchdown against the blitz', why: 'The rule read the blitz.', event: 3 }]]);
  const lines = new Map([[2, { n: 2, text: 'A short gain to start.' }], [4, { n: 4, text: 'Picked off!' }]]);
  const tr = P.buildTrack({ events, times, decisions, moments, lines });
  assert.deepEqual(tr.map(e => [e.i, e.kind, e.t]), [[1, 'line', 5], [2, 'moment', 11], [3, 'hl', 20], [4, 'score', 27]]);
  const mo = tr[1];
  assert.equal(mo.side, 0);
  assert.equal(mo.call, 'pass deep · wr1');
  assert.equal(mo.rule, 'If they blitz, go deep to wr1.');
  assert.equal(mo.what, 'Deep touchdown against the blitz');
  assert.deepEqual(mo.score, [6, 0]);
  assert.equal(tr[2].caster, 'Picked off!');
  assert.equal(tr[3].side, 1, 'the side that scored');
  assert.equal(tr[3].call, 'field goal');
  // Filters.
  assert.deepEqual(P.filterTrack(tr, 'prompt').map(e => e.i), [2]);
  assert.deepEqual(P.filterTrack(tr, 'key').map(e => e.i), [2, 3, 4]);
  assert.equal(P.filterTrack(tr, 'all').length, 4);
  // The line on screen: the last one that has started (index = events started).
  assert.equal(P.activeEntry(tr, 0), null);
  assert.equal(P.activeEntry(tr, 2).i, 1);
  assert.equal(P.activeEntry(tr, 3).i, 2);
  assert.equal(P.activeEntry(tr, 6).i, 4);
  // Captions: the caster's line, else what happened.
  assert.equal(P.captionOf(tr[0]), 'A short gain to start.');
  assert.equal(P.captionOf(tr[1]), 'Deep touchdown against the blitz');
  assert.equal(P.captionOf(tr[3]), 'Field goal is good');
  // HTML is escaped and carries the time.
  const h = P.trackItemHTML({ ...tr[1], text: '<b>x</b>' }, { teamName: () => 'A&B', sideColor: () => '#fff' });
  assert.ok(h.includes('0:11') && h.includes('A&amp;B') && !h.includes('<b>x</b>'));
});

test('the end-of-match summary shows the result, analysis, rule, moments and coach scores', async () => {
  const P = await load('arena-player.js');
  const m = {
    game: 'nfl', names: ['Alice', 'Bob'], score: [24, 17], winner: 0, followed: [0.8, 0.5],
    analysis: { headline: 'Deep shots won it', text: 'Alice kept going deep.', rule: 'Go deep vs blitz', side: 0, moments: [{ n: 2, side: 0, rule: 'Go deep vs blitz', what: 'TD', event: 3 }, { n: 9, side: 1, rule: 'Kick', what: 'FG' }] },
    feedback: [{ scores: { clarity: 4, coverage: 4, adaptivity: 3, specificity: 5, consistency: 4 } }, null],
  };
  const h = P.summaryHTML(m, { T: k => ({ 'ar.vp.wins': '{name} wins' })[k] || k, sideColor: s => ['#00f', '#f00'][s], momentIndex: mo => (mo.event === 3 ? 2 : null), times: [0, 5, 11] });
  for (const s of ['Alice', '24 - 17', 'Alice wins', 'Deep shots won it', 'Go deep vs blitz', '20/25', '80%', '50%', 'data-seek="2"', '0:11', 'data-end="again"', 'data-end="download"', 'data-end="close"']) assert.ok(h.includes(s), s);
  assert.ok(!h.includes('data-seek="null"'));
  // LoL with blue on the right: the score follows the map order.
  const lol = P.summaryHTML({ ...m, game: 'lol', meta: { blue: 1 }, score: [1, 0], winner: 0 }, {});
  assert.ok(lol.includes('Bob <b>0 - 1</b> Alice'));
});
