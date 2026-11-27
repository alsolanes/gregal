// El pati dels vents (winds-pati.js): on comença a mirar el pati.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const window = {};
vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'winds-pati.js'), 'utf8'), { window, document: {} });
const { startLeft } = window.gregalPati;

// The 2026-11-24 review, at 1280 px: two columns of corners, the square, one more column; 1117 units in view.
const edges = [0, 360, 720, 1190];
const plaza = { x: 720, w: 470 };

test('looking at the square, the left edge falls where a corner starts, so no wind there is cut in half', () => {
  const centred = plaza.x + plaza.w / 2 - 1117 / 2;
  assert.ok(centred > 360 && centred < 360 + 50, 'centring alone cut Mestral’s corner 37 units in');
  const left = startLeft(edges, plaza, 1117);
  assert.equal(left, 360, 'the corner on the left is whole');
  assert.ok(left + 1117 >= plaza.x + plaza.w, 'and the square is still whole');
});

test('when no corner start keeps the square whole (a phone), the square is centred', () => {
  assert.equal(startLeft(edges, plaza, 400), plaza.x + plaza.w / 2 - 200);
});

test('a corner chosen in the bar is shown whole from a corner start too', () => {
  const left = startLeft(edges, { x: 360, w: 360 }, 900);
  assert.equal(left, 0);
});
