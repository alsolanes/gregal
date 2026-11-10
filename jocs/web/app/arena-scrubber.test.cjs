const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const load = f => import(pathToFileURL(path.join(__dirname, f)).href);

test('timeline mapping: playback time when the viewer has it, events otherwise', async () => {
  const S = await load('arena-scrubber.js');
  const timed = { index: 2, total: 4, times: [0, 10, 40, 50, 100], duration: 100, elapsed: 25 };
  assert.equal(S.isTimed(timed), true);
  assert.equal(S.isTimed({ ...timed, times: [0, 1] }), false, 'times must cover every event');
  assert.equal(S.isTimed({ ...timed, duration: 0 }), false);
  assert.equal(S.fracOfIndex(timed, 0), 0);
  assert.equal(S.fracOfIndex(timed, 2), 0.4);
  assert.equal(S.fracOfIndex(timed, 2.5), 0.4, 'a mark sits where its event starts');
  assert.equal(S.fracOfIndex(timed, 4), 1);
  assert.equal(S.fracOfIndex(timed, 9), 1, 'clamped');
  assert.equal(S.fracNow(timed), 0.25, 'now = the playback second, not the event');
  assert.equal(S.indexAtFrac(timed, 0), 0);
  assert.equal(S.indexAtFrac(timed, 0.39), 1);
  assert.equal(S.indexAtFrac(timed, 0.4), 2);
  assert.equal(S.indexAtFrac(timed, 1), 3, 'never past the last event');
  assert.equal(S.indexAtFrac(timed, 7), 3);
  // Round trip: the index at an event's own position is that event.
  for (let i = 0; i < 4; i++) assert.equal(S.indexAtFrac(timed, S.fracOfIndex(timed, i)), i);
  const plain = { index: 3, total: 6 };
  assert.equal(S.fracOfIndex(plain, 3), 0.5);
  assert.equal(S.fracOfIndex(plain, 2.5), 2.5 / 6);
  assert.equal(S.fracNow(plain), 0.5);
  assert.equal(S.indexAtFrac(plain, 0.5), 3);
  assert.equal(S.indexAtFrac(plain, 1), 5);
  assert.equal(S.indexAtFrac(plain, -1), 0);
  for (let i = 0; i < 6; i++) assert.equal(S.indexAtFrac(plain, S.fracOfIndex(plain, i + 0.5)), i);
  // Nothing loaded yet.
  assert.equal(S.fracOfIndex({ total: 0 }, 3), 0);
  assert.equal(S.indexAtFrac({ total: 0 }, 0.5), 0);
  assert.equal(S.fracNow(null), 0);
});

test('labels: every control has an English default', async () => {
  const S = await load('arena-scrubber.js');
  for (const k of ['scrub.restart', 'scrub.back', 'scrub.play', 'scrub.pause', 'scrub.forward', 'scrub.end', 'scrub.live', 'scrub.speed', 'scrub.of', 'scrub.seek']) assert.ok(S.LABELS[k], k);
  assert.equal(S.JUMP_SECS, 15);
  assert.equal(S.fmtTime(59.99), '0:59');
  assert.equal(S.fmtTime('x'), '0:00');
});
