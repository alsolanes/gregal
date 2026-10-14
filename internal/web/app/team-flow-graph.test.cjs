const { test } = require('node:test');
const assert = require('node:assert/strict');

// Boxes are 156x34 centred on the node position.
const inside = (x, y, [cx, cy]) => Math.abs(x - cx) < 78 && Math.abs(y - cy) < 17;
const ends = d => {
  const n = d.match(/-?\d+(\.\d+)?/g).map(Number);
  return { start: [n[0], n[1]], end: [n[n.length - 2], n[n.length - 1]] };
};

test('revision edges start and end on box borders, never inside a node', async () => {
  const { revisionPath } = await import('./team-flow-graph.js');
  const cases = [
    [[512, 200], [840, 120]], // reviewer below-left of builder (default pipeline)
    [[840, 200], [512, 120]], // mirrored
    [[512, 220], [520, 60]],  // vertically aligned: loops out on the right
  ];
  for (const [from, to] of cases) {
    const { start, end } = ends(revisionPath(from[0], from[1], to[0], to[1]));
    assert.ok(!inside(start[0], start[1], from), `start inside source: ${start}`);
    assert.ok(!inside(end[0], end[1], to), `end inside target: ${end}`);
    assert.ok(Math.max(start[0], end[0]) < 1024, 'stays within the 1024 viewBox');
  }
});
