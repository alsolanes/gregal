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
