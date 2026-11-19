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

test('detail gives the whole bash command, not the clipped header', () => {
  const { detail } = require('./tool-summary.js');
  const command = 'cd /c/dev/gregal && ' + 'echo step && '.repeat(40) + 'go test ./...';
  assert.ok(summarize('bash', JSON.stringify({ command }), 'en').target.endsWith('…'));
  assert.equal(detail('bash', JSON.stringify({ command })), command);
  assert.equal(detail('bash', JSON.stringify({ command: 'ls', timeout: 30 })), 'ls\n\n# timeout: 30');
});

test('detail shows other tools as readable JSON and clips huge strings', () => {
  const { detail } = require('./tool-summary.js');
  const out = detail('write', JSON.stringify({ path: 'a.txt', content: 'y'.repeat(3000) }));
  assert.match(out, /"path": "a.txt"/);
  assert.match(out, /… \(\+1500\)/);
  assert.equal(detail('bash', 'not json'), 'not json');
});

test('read-only shell commands read as what they do, with paths instead of the raw command', () => {
  const bash = (command, lang = 'en') => summarize('bash', args({ command }), lang);
  assert.deepEqual(bash('ls internal/web desktop 2>/dev/null | head -50'), { label: 'List', target: 'internal/web, desktop', meta: '' });
  assert.deepEqual(bash('sed -n 1190,1205p internal/web/index.html'), { label: 'Read', target: 'internal/web/index.html', meta: 'lines 1190–1205' });
  assert.deepEqual(bash('cat a.js b.js'), { label: 'Read', target: 'a.js, b.js', meta: '' });
  assert.deepEqual(bash('grep -rn "work-group" internal/web/app | head -40'), { label: 'Search', target: '"work-group"', meta: 'in internal/web/app' });
  assert.deepEqual(bash('cd /c/dev/gregal && rg -n TODO'), { label: 'Search', target: '"TODO"', meta: '' });
  assert.equal(bash('ls src', 'ca').label, 'Llista');
  assert.equal(bash('sed -n 1,40p a.go', 'ca').meta, 'línies 1–40');
});

test('tests and git get their own verb; any other command keeps Run and the whole command', () => {
  const bash = command => summarize('bash', args({ command }), 'en');
  assert.deepEqual(bash('go test ./internal/web -run TestX'), { label: 'Test', target: 'go test ./internal/web -run TestX', meta: '' });
  assert.equal(bash('npm test').label, 'Test');
  assert.equal(bash('node --test internal/web/app/x.test.cjs').label, 'Test');
  assert.equal(bash('pytest -q tests').label, 'Test');
  assert.equal(bash('cargo test --lib').label, 'Test');
  assert.equal(bash('go test ./... 2>&1 | tail -20').label, 'Test');
  // Only a single command gets the verb: a chain may do something else after it.
  assert.deepEqual(bash('go test ./... && rm -rf x'), { label: 'Run', target: 'go test ./... && rm -rf x', meta: '' });
  assert.equal(bash('git status && rm -rf x').label, 'Run');
  assert.equal(bash('sed -n -E 1,5p a.go').label, 'Read');
  assert.deepEqual(bash('git status --short'), { label: 'Git', target: 'status --short', meta: '' });
  assert.deepEqual(bash('npm run build'), { label: 'Run', target: 'npm run build', meta: '' });
  assert.equal(summarize('bash', args({ command: 'go test ./...' }), 'ca').label, 'Prova');
});

test('explore lists what an exploring call looks at, and nothing for calls that change things', () => {
  const { explore } = require('./tool-summary.js');
  assert.deepEqual(explore('read', args({ path: 'a.go' })), [{ kind: 'read', target: 'a.go' }]);
  assert.deepEqual(explore('grep', args({ pattern: 'x' })), [{ kind: 'search', target: 'x' }]);
  assert.deepEqual(explore('glob', args({ pattern: '*.go' })), [{ kind: 'list', target: '*.go' }]);
  assert.deepEqual(explore('bash', args({ command: 'cat a.go; grep -n y b.go | head' })),
    [{ kind: 'read', target: 'a.go' }, { kind: 'search', target: 'y' }]);
  for (const command of ['cat a > b', 'sed -i s/a/b/ f', 'find . -name x -delete', 'find . -exec rm {} ;',
    'rm -rf out', 'ls; npm install', 'go test ./...', 'git status', 'cat a | tee b', 'cat $(rm x)',
    // sed: in-place hidden in combined or abbreviated flags, a second -e, or a script file.
    'sed -n -Ei 1p f', 'sed -n -ri 1,5p f', 'sed -n --in 1p f', 'sed -n -e "w /tmp/x" -e 1p f',
    'sed -n -ew/tmp/x 1p f', 'sed -n --expression=w/tmp/x 1p f', 'sed -n -f evil.sed -e 1p f', 'sed -n --file=evil.sed 1p f',
    // Filters after a pipe that write, and searchers or listers that run or write something.
    'ls | sort -o out', 'ls | sort --output=out', 'ls | sort --compress-program=sh', 'ls | uniq - out.txt',
    'rg --pre ./evil.sh x', 'rg --pre=sh x .', 'rg --hostname-bin=./evil x', 'grep -rn x . | rg --pre sh y',
    'tree -o out.txt src', 'find . -fprint0 out']) {
    assert.equal(explore('bash', args({ command })), null, command);
  }
  assert.equal(explore('edit', args({ path: 'a.go' })), null);
  assert.equal(explore('bash', 'not json'), null);
});

test('exploreSummary counts distinct files, searches and listings', () => {
  const { explore, exploreSummary } = require('./tool-summary.js');
  const calls = [
    explore('bash', args({ command: 'ls internal/web desktop' })),
    explore('read', args({ path: 'a.go' })),
    explore('bash', args({ command: 'sed -n 1,9p a.go; cat b.go' })),
    explore('grep', args({ pattern: 'x' })),
  ];
  assert.deepEqual(exploreSummary(calls, 'en'), { label: 'Explored', target: '2 files · 1 search · 1 listing' });
  assert.equal(exploreSummary(calls, 'ca').target, '2 fitxers · 1 cerca · 1 llistat');
  assert.equal(exploreSummary([explore('read', args({ path: 'a' }))], 'ca').target, '1 fitxer');
});
