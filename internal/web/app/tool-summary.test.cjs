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
    'tree -o out.txt src', 'find . -fprint0 out',
    // Shell syntax the splitter does not model: ANSI-C quoting, a comment that leaves a quote
    // open for the splitter but not for bash, expansions that run code, a stray \r.
    "cat $'a\\x27; rm -rf x'", "cat a #'\nrm -rf x'", 'printf -v x "%s" "$(id)"; echo ${x@P}', 'cat $[1+1]',
    'cat ${HOME}', 'ls 2>&1\r', 'tree -R -H . src']) {
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

test('detail has no box for a call without arguments', () => {
  const { detail } = require('./tool-summary.js');
  assert.equal(detail('remind_list', '{}'), '');
  assert.equal(detail('calendar', ''), '');
});

test('sentence describes a run of calls in the past tense, by its most telling call', () => {
  const { sentence } = require('./tool-summary.js');
  const c = (name, o) => ({ name, args: args(o || {}) });
  assert.equal(sentence([c('calendar', { action: 'today' })], 'ca'), 'Ha consultat el calendari');
  assert.equal(sentence([c('mail', { action: 'unread' })], 'en'), 'Checked the mail');
  assert.equal(sentence([c('remind_list')], 'ca'), 'Ha mirat els recordatoris');
  assert.equal(sentence([c('web_search', { query: 'x' }), c('web_fetch', { url: 'https://a.dev' })], 'ca'), 'Ha consultat la web');
  // Llegir i després editar és editar.
  assert.equal(sentence([c('read', { path: 'a.go' }), c('edit', { path: 'a.go', old_string: 'a', new_string: 'b' })], 'en'), 'Edited files');
  assert.equal(sentence([c('bash', { command: 'go test ./...' })], 'ca'), 'Ha passat els tests');
  assert.equal(sentence([c('bash', { command: 'npm run build' })], 'en'), 'Ran a command');
  assert.equal(sentence([c('bash', { command: 'ls src' }), c('grep', { pattern: 'x' })], 'en'), 'Searched the code');
  assert.equal(sentence([c('read', { path: 'a.go' })], 'ca'), 'Ha mirat els fitxers');
  assert.equal(sentence([c('mcp_github_create_issue', { title: 't' })], 'en'), 'Used github');
  assert.equal(sentence([c('profile', { action: 'get' })], 'ca'), 'Ha fet servir profile');
});

test('chip names what a call touched in a few words', () => {
  const { chip } = require('./tool-summary.js');
  assert.deepEqual(chip('edit', args({ path: 'C:/p/internal/login.go', old_string: 'a\nb', new_string: 'c' }), 'en'), { icon: 'edit', text: 'login.go +1 −2' });
  assert.deepEqual(chip('read', args({ path: 'src/app.ts' }), 'en'), { icon: 'read', text: 'app.ts' });
  assert.deepEqual(chip('read', args({ path: 'C:\\dev\\gregal\\internal\\web\\app\\tool-summary.js' }), 'en'), { icon: 'read', text: 'tool-summary.js' });
  assert.deepEqual(chip('write', args({ path: 'n.txt', content: 'a\nb\n' }), 'en'), { icon: 'edit', text: 'n.txt +2' });
  assert.deepEqual(chip('bash', args({ command: 'cd x && go test ./internal/...' }), 'en'), { icon: 'test', text: 'go test ./internal/...' });
  assert.deepEqual(chip('bash', args({ command: 'npm run build' }), 'en'), { icon: 'run', text: 'npm run build' });
  assert.deepEqual(chip('web_fetch', args({ url: 'https://pkg.go.dev/crypto/subtle' }), 'en'), { icon: 'web', text: 'pkg.go.dev' });
  assert.deepEqual(chip('calendar', args({ action: 'today' }), 'ca'), { icon: 'tool', text: 'calendar · today' });
  assert.deepEqual(chip('remind_list', '{}', 'ca'), { icon: 'tool', text: 'remind_list' });
  assert.ok(chip('bash', args({ command: 'x'.repeat(200) }), 'en').text.length <= 48);
});

test('outcome: errors fail, a read-only command that finds nothing is quiet, not failed', () => {
  const { outcome } = require('./tool-summary.js');
  const grep = args({ command: 'grep -rn foo src' });
  // El format real del backend: tools.go fa fmt.Errorf("exit: %v…") i el loop hi posa «ERROR: ».
  assert.deepEqual(outcome('bash', grep, 'ERROR: exit: exit status 1\n', 'ca'), { state: 'quiet', note: 'sense resultats' });
  assert.deepEqual(outcome('bash', args({ command: 'ls nope' }), 'ERROR: exit: exit status 2\nls: cannot access', 'en'), { state: 'quiet', note: 'nothing found' });
  // Una ordre que fa coses (o uns tests) que surt amb error sí que ha fallat.
  assert.deepEqual(outcome('bash', args({ command: 'npm run build' }), 'ERROR: exit: exit status 2\nTS2304', 'ca'), { state: 'failed', note: 'codi 2' });
  assert.deepEqual(outcome('bash', grep, 'ERROR: exit: exit status 127\n', 'en'), { state: 'failed', note: 'exit 127' });
  assert.deepEqual(outcome('read', args({ path: 'a' }), 'ERROR: no such file', 'en'), { state: 'failed', note: '' });
  assert.deepEqual(outcome('bash', args({ command: 'curl x' }), 'TOOL BLOCKED: patró bloquejat: curl', 'en'), { state: 'failed', note: '' });
  assert.deepEqual(outcome('calendar', '{}', 'No events.', 'en'), { state: 'done', note: '' });
});
