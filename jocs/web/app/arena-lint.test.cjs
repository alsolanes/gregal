const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const load = f => import(pathToFileURL(path.join(__dirname, f)).href);

const GOOD = [
  '# Strategy', 'We read the opponent and counter what they repeat.', '',
  '## Priorities (the first rule that applies wins)',
  '1. 4th down, under 2 yards: run_inside.',
  '2. 3rd & 7+: pass_medium to wr1; if they blitz, screen to rb.',
  '3. Q4, under 3:00, ahead by 7: run_inside, tempo milk.',
  '4. If they key on the run more than 60%: pass_short.',
  '5. In the red zone when behind: pass_deep.',
  '6. Against cover2: pass_medium.',
  '7. Otherwise: base formation, run_outside.',
  '', 'Keep it simple. When they adapt, we adapt. ' + 'x'.repeat(120),
].join('\n');

test('A concrete, complete .md passes every check', async () => {
  const { lintMD } = await load('arena-lint.js');
  const r = lintMD(GOOD);
  assert.equal(r.ok, r.total, JSON.stringify(r.checks.filter(c => !c.ok)));
  assert.equal(r.score, 100);
});

test('Vague wording is flagged with the offending line', async () => {
  const { lintMD } = await load('arena-lint.js');
  const r = lintMD('# Plan\nBe smart with the clock.\nTry to pass more.');
  const c = r.checks.find(x => x.id === 'vague');
  assert.equal(c.ok, false);
  assert.match(c.tip, /Be smart with the clock/);
  assert.equal(r.tip.length > 0, true);
});

test('Missing default, opponent and order are each reported', async () => {
  const { lintMD } = await load('arena-lint.js');
  const r = lintMD('x'.repeat(400));
  for (const id of ['default', 'opponent', 'order', 'rules', 'numbers']) assert.equal(r.checks.find(c => c.id === id).ok, false, id);
});

test('Repeated rules and the length limit are caught', async () => {
  const { lintMD } = await load('arena-lint.js');
  const dup = lintMD('- On 3rd and long pass to wr1 every time.\n- On 3rd and long pass to wr1 every time.');
  assert.equal(dup.checks.find(c => c.id === 'dups').ok, false);
  assert.equal(lintMD('a'.repeat(50), 40).checks.find(c => c.id === 'size').ok, false);
  assert.equal(lintMD('').checks.find(c => c.id === 'size').ok, false);
});

test('The HTML escapes the quoted line', async () => {
  const { lintMD, lintHTML } = await load('arena-lint.js');
  const html = lintHTML(lintMD('<script>alert(1)</script> be smart'));
  assert.equal(html.includes('<script>'), false);
});
