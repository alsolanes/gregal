const assert = require('node:assert/strict');
const { test } = require('node:test');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

function scan(files, env = {}) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'gregal-audit-'));
  try {
    for (const [name, content] of Object.entries(files)) {
      const target = path.join(root, name);
      fs.mkdirSync(path.dirname(target), { recursive: true });
      fs.writeFileSync(target, content);
    }
    return spawnSync(process.execPath, [path.join(__dirname, 'audit-public.cjs'), root], { encoding: 'utf8', env: { ...process.env, GREGAL_PRIVATE_MARKERS: '[]', ...env } });
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
}

test('scripts receive credential checks without printing the credential', () => {
  const credential = 'sk-' + 'synthetic'.repeat(4);
  const result = scan({ 'scripts/deploy.cjs': `const key = '${credential}';` });
  assert.equal(result.status, 1);
  assert.match(result.stderr, /scripts\/deploy.cjs: provider credential/);
  assert.ok(!result.stderr.includes(credential));
});

test('runtime data is rejected and neutral examples pass', () => {
  assert.equal(scan({ 'memory.md': 'local context' }).status, 1);
  assert.equal(scan({ '.env': Buffer.from([0, 1, 2]) }).status, 1);
  assert.equal(scan({ '.gregal/config.yaml': 'local settings' }).status, 1);
  assert.equal(scan({ 'android/local.properties': 'sdk.dir=/private/path' }).status, 1);
  assert.equal(scan({ '.env.example': 'TOKEN=replace-me', 'README.md': 'Neutral source' }).status, 0);
});

test('audit utilities retain credential checks', () => {
  const result = scan({ 'scripts/prepare-public.cjs': 'sk-' + 'synthetic'.repeat(4) });
  assert.equal(result.status, 1);
  assert.equal(scan({ LICENSE: 'sk-' + 'synthetic'.repeat(4) }).status, 1);
});

test('Telegram credentials with short bot identifiers are rejected without disclosure', () => {
  for (const id of ['123', '1234567', '1234567890']) {
    const credential = id + ':' + 'synthetic_'.repeat(4) + '-';
    const result = scan({ 'config.yaml': 'telegram:\n  token: ' + credential });
    assert.equal(result.status, 1);
    assert.match(result.stderr, /config.yaml: Telegram credential/);
    assert.ok(!result.stderr.includes(credential));
  }
});

test('configured markers detect private values without printing them', () => {
  const marker = 'private-fixture-42';
  const result = scan({ 'notes.md': 'reviewed by ' + marker }, { GREGAL_PRIVATE_MARKERS: JSON.stringify([marker]) });
  assert.equal(result.status, 1);
  assert.ok(!result.stderr.includes(marker));
  assert.equal(scan({ 'notes.md': 'a.b' }, { GREGAL_PRIVATE_MARKERS: '["a.b"]' }).status, 1);
  assert.equal(scan({ 'notes.md': 'axb' }, { GREGAL_PRIVATE_MARKERS: '["a.b"]' }).status, 0);
  assert.notEqual(scan({}, { GREGAL_PRIVATE_MARKERS: '[""]' }).status, 0);
  assert.equal(scan({ 'notes.md': 'reviewed by usera and userb (ids 1234567)' }).status, 0);
  assert.equal(scan({ 'docs.md': 'see https://releases.example.org/x' }).status, 0);
});
