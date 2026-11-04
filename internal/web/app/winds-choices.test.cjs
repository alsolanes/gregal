const test = require('node:test');
const assert = require('node:assert/strict');
const { extract, strip } = require('./winds-choices.js');

test('the choices block becomes quick replies and leaves the text', () => {
  const md = 'Ho tinc a punt.\n\n```choices\n- Publica-la\n2. Canvia el títol\nDeixa-ho\n```';
  assert.deepEqual(extract(md), ['Publica-la', 'Canvia el títol', 'Deixa-ho']);
  assert.equal(strip(md), 'Ho tinc a punt.');
});

test('an unfinished block (still streaming) is hidden too', () => {
  assert.equal(strip('Ho tinc a punt.\n```choices\n- Pub'), 'Ho tinc a punt.');
});

test('no block: the text is untouched and there are no options', () => {
  assert.equal(strip('Hola'), 'Hola');
  assert.deepEqual(extract('Hola'), []);
  assert.deepEqual(extract(null), []);
  assert.equal(strip(undefined), '');
});

test('at most 4 options, no duplicates, none longer than 60 characters', () => {
  assert.equal(extract('```choices\na\nb\nc\nd\ne\nf\n```').length, 4);
  assert.deepEqual(extract('```choices\nsí\nsí\nno\n```'), ['sí', 'no']);
  assert.deepEqual(extract('```choices\n' + 'x'.repeat(61) + '\nok\n```'), ['ok']);
});

test('text after the closing fence is kept', () => {
  assert.equal(strip('A\n```choices\nx\ny\n```\nB'), 'A\nB');
});
