const { test } = require('node:test');
const assert = require('node:assert/strict');
const { summarize } = require('./tool-summary.js');

const args = o => JSON.stringify(o);

test('common tools read as an action and a target, not raw JSON', () => {
  assert.deepEqual(summarize('read', args({ path: 'src/stats.py' }), 'en'), { label: 'Read', target: 'src/stats.py', meta: '' });
  assert.equal(summarize('read', args({ path: 'a.go', offset: 10, limit: 31 }), 'en').meta, 'lines 10–40');
  assert.deepEqual(summarize('edit', args({ path: 'a.py', old_string: 'x\ny', new_string: 'z' }), 'en'),
    { label: 'Edit', target: 'a.py', meta: '+1 −2' });
  assert.equal(summarize('bash', args({ command: 'python -m unittest\n  discover' }), 'en').target, 'python -m unittest discover');
  assert.equal(summarize('grep', args({ pattern: 'TODO', dir: 'internal' }), 'en').meta, 'in internal');
  assert.equal(summarize('delegate', args({ tasks: [{ prompt: 'find tests' }, { prompt: 'read docs' }] }), 'en').meta, '2 tasks');
  assert.equal(summarize('write', args({ path: 'n.txt', content: 'a\nb\n' }), 'ca').meta, '2 línies');
});

test('knowledge tools read as a search and a note', () => {
  assert.deepEqual(summarize('knowledge_search', args({ query: 'pump torque', collection: 'Manuals' }), 'en'),
    { label: 'Search knowledge', target: '"pump torque"', meta: 'Manuals' });
  assert.equal(summarize('knowledge_search', args({ query: 'x' }), 'ca').label, 'Cerca al coneixement');
  assert.deepEqual(summarize('knowledge_note', args({ text: 'Deploys on Thursday' }), 'en'),
    { label: 'Save note', target: 'Deploys on Thursday', meta: '' });
});

test('unknown and MCP tools show key/value pairs; broken JSON is shown as text', () => {
  const mcp = summarize('mcp_notes_create', args({ title: 'x', tags: ['a'] }), 'en');
  assert.equal(mcp.label, 'MCP · notes › create');
  assert.equal(mcp.target, 'title: x · tags: ["a"]');
  assert.deepEqual(summarize('read', '{"path": "trunc', 'en'), { label: 'Read', target: '{"path": "trunc', meta: '' });
});
