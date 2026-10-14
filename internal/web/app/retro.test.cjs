const { test } = require('node:test');
const assert = require('node:assert/strict');
const core = require('./retro-core.js');

const agents = [
  { id: 'coordinator', name: 'Coordinator' },
  { id: 'builder', name: 'Builder' },
  { id: 'builder-2', name: 'Builder 2' },
  { id: 'reviewer', name: 'Reviewer' },
];

test('mentions match ids and names like the server, ignoring emails and unknown names', () => {
  assert.deepEqual(core.parseMentions('@Builder why? cc @reviewer, @main and @builder-2. x@builder.com @nobody', agents),
    ['builder', 'reviewer', 'coordinator', 'builder-2']);
  assert.deepEqual(core.parseMentions('no mentions', agents), []);
  assert.deepEqual(core.parseMentions('@builder @builder', agents), ['builder']);
});

test('mention autocomplete finds the token under the caret and inserts the id', () => {
  assert.deepEqual(core.mentionQuery('hey @bu', 7), { start: 4, query: 'bu' });
  assert.equal(core.mentionQuery('mail a@bu', 9), null);
  assert.deepEqual(core.mentionMatches('bu', agents).map(a => a.id), ['builder', 'builder-2']);
  assert.equal(core.mentionMatches('', agents).length, 4);
  const out = core.insertMention('hey @bu tell me', 7, 'builder');
  assert.equal(out.text, 'hey @builder  tell me');
  assert.equal(out.caret, 13);
});

test('messages group by consecutive author within the gap and threads stay apart', () => {
  const t = s => new Date(Date.UTC(2026, 0, 1, 10, 0, s)).toISOString();
  const msgs = [
    { id: 'm1', author: 'coordinator', at: t(0) },
    { id: 'm2', author: 'coordinator', at: t(10) },
    { id: 'm3', author: 'builder', at: t(20) },
    { id: 'm4', author: 'user', at: t(30), thread_id: 'm3' },
    { id: 'm5', author: 'builder', at: t(40), thread_id: 'm3' },
    { id: 'm6', author: 'builder', at: t(50) },
  ];
  const groups = core.groupMessages(msgs);
  assert.deepEqual(groups.map(g => [g.author, g.items.map(m => m.id)]), [['coordinator', ['m1', 'm2']], ['builder', ['m3', 'm6']]]);
  assert.equal(core.groupMessages(msgs, 5000).length, 4);
  assert.deepEqual(core.replyCounts(msgs), { m3: 2 });
  assert.deepEqual(core.threadMessages(msgs, 'm3').map(m => m.id), ['m3', 'm4', 'm5']);
});

test('live events upsert messages, replace reactions and ignore other channels', () => {
  const ch = { key: 'run-5', messages: [{ id: 'm1', author: 'builder', text: 'a' }] };
  assert.equal(core.applyEvent(ch, 'retro_message', { key: 'run-6', message: { id: 'm2' } }), false);
  assert.equal(core.applyEvent(ch, 'retro_message', { key: 'run-5', message: { id: 'm2', author: 'reviewer', text: 'b' } }), true);
  assert.equal(core.applyEvent(ch, 'retro_message', { key: 'run-5', message: { id: 'm2', author: 'reviewer', text: 'b2' } }), true);
  assert.equal(ch.messages.length, 2);
  assert.equal(ch.messages[1].text, 'b2');
  core.applyEvent(ch, 'retro_reaction', { key: 'run-5', message_id: 'm1', reactions: [{ emoji: '🤔', by: ['user'] }] });
  assert.deepEqual(ch.messages[0].reactions, [{ emoji: '🤔', by: ['user'] }]);
});

test('optimistic reaction toggle and typing names', () => {
  let r = core.toggleReaction([], '👍', 'user');
  assert.deepEqual(r, [{ emoji: '👍', by: ['user'] }]);
  r = core.toggleReaction([{ emoji: '👍', by: ['user', 'builder'] }], '👍', 'user');
  assert.deepEqual(r, [{ emoji: '👍', by: ['builder'] }]);
  assert.deepEqual(core.toggleReaction(r, '👍', 'builder'), []);
  assert.equal(core.typingNames(['Builder']), 'Builder');
  assert.equal(core.typingNames(['A', 'B', 'C'], ' i '), 'A, B i C');
});
