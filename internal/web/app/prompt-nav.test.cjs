const { test } = require('node:test');
const assert = require('node:assert/strict');
const { promptStates, activeIndex, pitch, clip } = require('./prompt-nav.js');

test('each prompt takes the state of its own turn', () => {
  assert.deepEqual(promptStates(['u', 'answer', 'u', 'fail', 'u', 'run']), ['done', 'failed', 'running']);
  // A running group wins over an answer already painted in the same turn.
  assert.deepEqual(promptStates(['u', 'answer', 'run']), ['running']);
  assert.deepEqual(promptStates(['u', 'fail', 'answer']), ['failed']);
  // A turn you stopped still painted its work group: the stop line wins.
  assert.deepEqual(promptStates(['u', 'answer', 'stop', 'u', 'stop', 'fail']), ['stopped', 'stopped']);
  // Marks before the first prompt belong to nobody.
  assert.deepEqual(promptStates(['answer', 'run', 'u', 'answer']), ['done']);
  assert.deepEqual(promptStates([]), []);
});

test('a prompt with nothing below is running only while the session is busy, and only if last', () => {
  assert.deepEqual(promptStates(['u', 'u'], false), ['empty', 'empty']);
  assert.deepEqual(promptStates(['u', 'u'], true), ['empty', 'running']);
  assert.deepEqual(promptStates(['u', 'answer', 'u'], true), ['done', 'running']);
});

test('the prompt in view is the last one that reached the top of the viewport', () => {
  const tops = [0, 400, 900, 1500];
  assert.equal(activeIndex(tops, 0, 600, false), 0);
  assert.equal(activeIndex([16, 130], 0, 600, false), 0); // a short answer: the next prompt is still lower
  assert.equal(activeIndex(tops, 250, 600, false), 0); // line at 330
  assert.equal(activeIndex(tops, 330, 600, false), 1); // line at 410
  assert.equal(activeIndex([0, 50], 0, 120, false), 0); // short view: the line is at a quarter (30), not 80

  assert.equal(activeIndex(tops, 830, 600, false), 2); // line at 910
  assert.equal(activeIndex(tops, 800, 600, true), 3); // scrolled to the bottom
  assert.equal(activeIndex([100], 0, 30, false), 0); // nothing above the line yet: the first
  assert.equal(activeIndex([], 0, 600, false), -1);
});

test('spacing compresses with many prompts but never below the minimum', () => {
  assert.equal(pitch(3, 600), 14);
  assert.equal(pitch(60, 600), 10);
  assert.equal(pitch(500, 600), 5);
  assert.equal(pitch(0, 600), 14);
  assert.equal(pitch(10, 0), 14);
});

test('prompt text is collapsed and truncated for the tooltip', () => {
  assert.equal(clip('  fes  un\n test '), 'fes un test');
  assert.equal(clip('abcdefghij', 6), 'abcde…');
  assert.equal(clip(null), '');
});
