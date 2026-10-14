const { test } = require('node:test');
const assert = require('node:assert/strict');
const kb = require('./knowledge.js');

test('globs accept commas, semicolons and new lines', () => {
  assert.deepEqual(kb.parseGlobs('*.md, docs/** ;\n**/*.pdf,,'), ['*.md', 'docs/**', '**/*.pdf']);
  assert.deepEqual(kb.parseGlobs(''), []);
});

test('collections are grouped by scope in page order', () => {
  const g = kb.groupCollections([{ id: 'p-1', scope: 'project' }, { id: 'g-1', scope: 'global' }, { id: 's-1', scope: 'session' }, { id: 'g-2', scope: 'global' }]);
  assert.deepEqual(g.global.map(c => c.id), ['g-1', 'g-2']);
  assert.deepEqual(g.project.map(c => c.id), ['p-1']);
  assert.deepEqual(g.session.map(c => c.id), ['s-1']);
});

test('relative times read naturally in both languages', () => {
  const now = Date.parse('2026-10-13T12:00:00Z');
  assert.equal(kb.relTime('2026-10-13T11:59:30Z', now, 'en'), 'just now');
  assert.equal(kb.relTime('2026-10-13T11:55:00Z', now, 'en'), '5 min ago');
  assert.equal(kb.relTime('2026-10-13T09:00:00Z', now, 'ca'), 'fa 3 h');
  assert.equal(kb.relTime('2026-09-01T09:00:00Z', now, 'en'), '2026-09-01');
  assert.equal(kb.relTime('0001-01-01T00:00:00Z', now, 'en'), '');
});

test('status counts feed the problems filter', () => {
  assert.deepEqual(kb.statusCounts([{ status: 'indexed' }, { status: 'error' }, { status: 'indexed' }, { status: 'unsupported' }]),
    { indexed: 2, empty: 0, skipped: 0, unsupported: 1, error: 1 });
});

test('highlight escapes HTML and marks query words, accent-insensitive', () => {
  const html = kb.highlight('Índex <b>de</b> la bomba centrífuga', 'indice centrifuga');
  assert.ok(!html.includes('<b>'), 'raw HTML must be escaped');
  assert.ok(html.includes('&lt;b&gt;'));
  assert.ok(html.includes('<mark>centrífuga</mark>'), html);
  assert.equal(kb.highlight('plain text', ''), 'plain text');
});

test('stats line covers running, never indexed and indexed collections', () => {
  const T = (k, f) => f;
  const now = Date.parse('2026-10-13T12:00:00Z');
  assert.equal(kb.statsLine({ progress: { running: true, done: 3, total: 9 } }, T, 'en', now), 'Indexing 3/9');
  assert.equal(kb.statsLine({ sources: [{}], stats: {} }, T, 'en', now), 'Not indexed yet');
  assert.equal(kb.statsLine({ stats: {} }, T, 'en', now), 'Empty');
  assert.equal(kb.statsLine({ stats: { docs: 4, errors: 1, indexed: '2026-10-13T11:00:00Z' } }, T, 'en', now), '4 documents · 1 errors · 1 h ago');
});

test('search hits render path, location, collection and escaped snippet', () => {
  const html = kb.renderHits([{ path: 'docs/a.pdf', loc: 'page 2', collection: 'Manuals', score: 3.1, snippet: 'torque <is> 35 Nm' }], 'torque');
  assert.ok(html.includes('docs/a.pdf · page 2'));
  assert.ok(html.includes('Manuals'));
  assert.ok(html.includes('<mark>torque</mark> &lt;is&gt;'));
  assert.ok(kb.renderHits([], 'x').includes('No passages'));
});
