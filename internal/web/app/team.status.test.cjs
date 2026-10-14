const { test } = require('node:test');
const assert = require('node:assert/strict');

test('team progress counts only the current real colleague states in both languages', async () => {
  const { progressSummary, replaceAgentButtons } = await import('./team-status.js');
  const roles = ['coordinator', 'researcher', 'builder', 'reviewer'];
  const states = { coordinator: 'completed', researcher: 'working', builder: 'waiting', reviewer: 'idle' };

  assert.equal(progressSummary(roles, states, 'en'), '1 working · 1 waiting for a teammate · 1/4 completed');
  assert.equal(progressSummary(roles, states, 'ca'), '1 treballant · 1 esperant un company · 1/4 completats');
  assert.equal(progressSummary(roles, { ...states, researcher: 'idle', builder: 'idle' }, 'en'), '1/4 completed');

  const focused = { dataset: { teamRole: 'builder' }, options: null, focus(options) { this.options = options; } };
  const list = { children: [], replaceChildren(...children) { this.children = children; } };
  replaceAgentButtons(list, [{ dataset: { teamRole: 'coordinator' } }, focused], 'builder');
  assert.equal(list.children[1], focused);
  assert.deepEqual(focused.options, { preventScroll: true });
});

test('attention queue lists agents awaiting a decision and cycles through them', async () => {
  const { attentionQueue, nextAttention } = await import('./team-status.js');
  const roles = ['coordinator', 'researcher', 'builder', 'reviewer'];

  assert.deepEqual(
    attentionQueue(roles, { coordinator: 'working', researcher: 'waiting', builder: 'failed', reviewer: 'completed' }),
    ['builder'],
  );
  assert.deepEqual(
    attentionQueue(roles, { coordinator: 'working', researcher: 'completed', builder: 'completed', reviewer: 'idle' }),
    [],
  );
  const states = { coordinator: 'cancelled', researcher: 'waiting', builder: 'working', reviewer: 'waiting' };
  assert.equal(nextAttention(roles, states, 'researcher'), 'coordinator');
  assert.equal(nextAttention(roles, states, 'reviewer'), 'coordinator');
  assert.equal(nextAttention(roles, states, 'builder'), 'coordinator');
  assert.equal(nextAttention(roles, { coordinator: 'working', researcher: 'completed', builder: 'idle', reviewer: 'idle' }, 'coordinator'), null);
});

test('flow topology mirrors the engine pipeline: plan, parallel pair, review, revision', async () => {
  const { teamFlowTopology } = await import('./team-status.js');
  const { nodes, edges } = teamFlowTopology();
  assert.deepEqual(nodes, ['coordinator', 'researcher', 'builder', 'reviewer']);
  assert.deepEqual(edges, [
    { from: 'coordinator', to: 'researcher', kind: 'split' },
    { from: 'coordinator', to: 'builder', kind: 'split' },
    { from: 'researcher', to: 'reviewer', kind: 'merge' },
    { from: 'builder', to: 'reviewer', kind: 'merge' },
    { from: 'reviewer', to: 'builder', kind: 'revision' },
  ]);
  for (const edge of edges) {
    assert.ok(nodes.includes(edge.from) && nodes.includes(edge.to), `dangling edge ${edge.from}>${edge.to}`);
  }
});

test('layoutFlow layers any spec by data depth and tolerates cycles', async () => {
  const { layoutFlow } = await import('./team-status.js');
  const nodes = [{ id: 'a' }, { id: 'b' }, { id: 'c' }, { id: 'd' }];
  const edges = [
    { from: 'a', to: 'b', kind: 'data' },
    { from: 'a', to: 'c', kind: 'data' },
    { from: 'b', to: 'd', kind: 'data' },
    { from: 'c', to: 'd', kind: 'data' },
  ];
  const { pos, height } = layoutFlow(nodes, edges);
  assert.equal(pos.a[1], 36);
  assert.equal(pos.b[1], 100);
  assert.equal(pos.c[1], 100);
  assert.equal(pos.d[1], 164);
  assert.ok(pos.b[0] < 512 && pos.c[0] > 512, 'parallel pair spreads around the center');
  assert.equal(height, 196);
  const cyclic = layoutFlow(nodes, [...edges, { from: 'd', to: 'a', kind: 'data' }]);
  assert.ok(cyclic.height <= 36 + 5 * 64 + 32, 'cycles stay drawable');
  assert.ok(Object.values(cyclic.pos).every(([x, y]) => Number.isFinite(x) && Number.isFinite(y)));
});

test('checkFlow catches structural mistakes, the server owns deep rules', async () => {
  const { checkFlow } = await import('./team-status.js');
  const good = { nodes: [{ id: 'a', role: 'builder' }, { id: 'b', role: 'reviewer', mode: 'review' }], edges: [{ from: 'a', to: 'b', kind: 'data' }], output: 'b', max_rounds: 1 };
  assert.equal(checkFlow(good), null);
  assert.equal(checkFlow(good, 'ca'), null);
  assert.match(checkFlow(null), /agent/i);
  assert.match(checkFlow({ ...good, nodes: [{ id: 'a', role: 'builder' }, { id: 'a', role: 'builder' }] }), /id/i);
  assert.match(checkFlow({ ...good, nodes: [{ id: 'a', role: 'oracle' }, good.nodes[1]] }, 'ca'), /rol/i);
  assert.ok(checkFlow({ ...good, output: 'ghost' }));
  assert.ok(checkFlow({ ...good, edges: [{ from: 'a', to: 'a', kind: 'data' }] }));
  assert.ok(checkFlow({ ...good, edges: [{ from: 'a', to: 'b', kind: 'telepathy' }] }));
  assert.ok(checkFlow({ ...good, max_rounds: -1 }));
});

test('flow storage degrades on blocked or corrupt storage', async () => {
  const { readJSON, writeJSON } = await import('./team-status.js');
  const mem = new Map();
  const storage = { getItem: k => (mem.has(k) ? mem.get(k) : null), setItem: (k, v) => void mem.set(k, v) };
  assert.equal(readJSON(storage, 'missing'), null);
  assert.equal(writeJSON(storage, 'k', { a: 1 }), true);
  assert.deepEqual(readJSON(storage, 'k'), { a: 1 });
  mem.set('k', '{invalid');
  assert.equal(readJSON(storage, 'k'), null);
  assert.equal(writeJSON({ setItem() { throw new Error('denied'); } }, 'k', 1), false);
  assert.equal(readJSON(null, 'k'), null);
});
