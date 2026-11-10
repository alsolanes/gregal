const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const load = f => import(pathToFileURL(path.join(__dirname, f)).href);

const match = () => ({
  game: 'cs2', names: ['Blue', 'Red'], score: [13, 9],
  events: [
    { n: 1, kind: 'round_start', decision: 1 },
    { n: 2, kind: 'kill', decision: 1, hl: true, text: 'ace!' },
    { n: 3, kind: 'kill', decision: 2 },
    { n: 4, kind: 'plant', decision: 2 },
    { n: 5, kind: 'round_end', decision: 2, score: [1, 0] },
    { n: 6, kind: 'round_start', decision: 3, hl: true, text: 'eco' },
  ],
  lines: [{ n: 4, text: 'They planted it!', audio: 'v4' }, { n: 2, text: 'no audio' }],
  analysis: { audio: 'sum', moments: [{ n: 2, side: 0, rule: 'stack B', what: 'B plant', why: 'read', event: 4 }] },
});

test('where the video slows down: the plays of the moment decision', async () => {
  const X = await load('arena-export.js');
  const m = match();
  assert.deepEqual([...X.slowStarts(m.events, X.momentsByEvent(m))], [3], 'from the first play of decision 2');
  // The moment is the first play of its decision: one play earlier.
  const lone = new Map([[3, {}]]);
  assert.deepEqual([...X.slowStarts(m.events, lone)], [2]);
  // At most 8 plays back.
  const long = Array.from({ length: 20 }, (_, i) => ({ n: i + 1, decision: 7 }));
  assert.deepEqual([...X.slowStarts(long, new Map([[20, {}]]))], [12]);
});

test('the director: fast between moments, slow on them, with the sounds', async () => {
  const X = await load('arena-export.js');
  const m = match(), d = new X.Director(m);
  assert.equal(d.fast, 12); assert.equal(d.slow, 1.5);
  // Before the match starts, events only move the score.
  d.event(m.events[0], 0);
  assert.equal(d.cues.length, 0);
  d.begin();
  assert.equal(d.tick(5), 12);
  d.event(m.events[1], 5);
  assert.deepEqual(d.cues, [], 'a big play at full speed: no roar');
  d.event(m.events[2], 6); // slowFrom
  assert.equal(d.speed, 1.5); assert.equal(d.slowUntil, Infinity);
  assert.equal(d.tick(50), 1.5, 'stays slow until the moment');
  d.event(m.events[3], 10); // the moment
  assert.equal(d.card.until, 10 + X.CARD_SECS);
  assert.equal(d.cap.text, 'They planted it!');
  assert.deepEqual(d.cues, [{ t: 10, kind: 'sting' }, { t: 10, kind: 'voice', key: 'v4' }]);
  d.event(m.events[4], 11);
  assert.deepEqual(d.score, [1, 0]);
  d.event(m.events[5], 12);
  assert.deepEqual(d.cues[2], { t: 12, kind: 'roar', big: false }, 'a big play while slow: a roar');
  assert.equal(d.tick(16.4), 1.5);
  assert.equal(d.tick(16.6), 12, 'back to fast after the card');
  d.summary(30);
  assert.deepEqual(d.cues.slice(-2), [{ t: 30, kind: 'sting' }, { t: 30, kind: 'voice', key: 'sum' }]);
  assert.equal(d.shown, 6);
  // NFL: the scoreboard shows the score before the play.
  const n = new X.Director({ game: 'nfl', names: ['a', 'b'], events: [] });
  n.event({ n: 1, score: [7, 0] }, 0); assert.deepEqual(n.score, [0, 0]);
  n.event({ n: 2, score: [7, 3] }, 1); assert.deepEqual(n.score, [7, 0]);
});

test('audio placement: voices cut each other, nothing past the end', async () => {
  const X = await load('arena-export.js');
  const cues = [
    { t: 10, kind: 'voice', key: 'b' },
    { t: 2, kind: 'voice', key: 'a' },
    { t: 2, kind: 'sting' },
    { t: 4, kind: 'voice', key: 'missing' },
    { t: 19, kind: 'roar', big: true },
    { t: 25, kind: 'voice', key: 'a' },
  ];
  const p = X.placeAudio(cues, new Map([['a', 12], ['b', 3]]), 20);
  assert.deepEqual(p, [
    { kind: 'voice', key: 'a', at: 2, end: 10 },
    { kind: 'sting', at: 2, end: 2 + X.FX_SECS.sting },
    { kind: 'voice', key: 'b', at: 10, end: 13 },
    { kind: 'roar', at: 19, end: 20, big: true },
  ]);
  assert.deepEqual(X.placeAudio([{ t: 1, kind: 'voice', key: 'a' }], { a: 2 }, 10), [{ kind: 'voice', key: 'a', at: 1, end: 3 }], 'durations as an object too');
  assert.equal(X.summarySecs(0), X.SUMMARY_MIN);
  assert.equal(X.summarySecs(14.2), 14.2);
  assert.equal(X.summarySecs(99), X.SUMMARY_MAX);
});

test('frame schedule, progress and ETA', async () => {
  const X = await load('arena-export.js');
  assert.equal(X.frameStamp(0), 0);
  assert.equal(X.frameStamp(1), 33333);
  assert.equal(X.frameStamp(30), 1000000, 'exact on every second: no drift');
  assert.equal(X.frameStamp(3 * 60 * 30 + 1) - X.frameStamp(3 * 60 * 30), 33333);
  // Every frame lasts 33 or 34 µs-rounded steps, never a gap.
  for (let i = 0; i < 300; i++) { const d = X.frameStamp(i + 1) - X.frameStamp(i); assert.ok(d === 33333 || d === 33334); }
  assert.equal(X.exportFrac('intro', null, 0.5), 0.025);
  assert.equal(X.exportFrac('play', { elapsed: 50, duration: 100 }), 0.05 + 0.85 * 0.5);
  assert.equal(X.exportFrac('play', { index: 3, total: 6 }), 0.05 + 0.85 * 0.5, 'by events without playback time');
  assert.equal(X.exportFrac('summary', null, 1), 1);
  assert.equal(X.etaSecs(0.01, 5), null, 'too early to say');
  assert.equal(X.etaSecs(0.25, 10), 30);
  assert.equal(X.etaSecs(1, 10), 0);
  assert.equal(X.videoName({ game: 'lol', names: ['Équipe Ça', 'B team!'] }, 'mp4'), 'gregal-arena-lol-equipe-ca-vs-b-team.mp4');
});
