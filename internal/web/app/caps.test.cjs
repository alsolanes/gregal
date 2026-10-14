const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

// caps.js és un script clàssic: es carrega en un context mínim i s'hi
// proven les peces pures (crític, prompt del pla, xip d'aposta).
function load() {
  const window = { gregalT: k => k };
  const document = { readyState: 'complete', addEventListener: () => {}, getElementById: () => null, createElement: () => ({ className: '', textContent: '', title: '' }) };
  const ctx = vm.createContext({ window, document, setTimeout, console });
  vm.runInContext(fs.readFileSync(path.join(__dirname, 'caps.js'), 'utf8'), ctx);
  return window.gregalCaps;
}

test('critic block counts concerns and escapes text; none and skipped are quiet', () => {
  const caps = load();
  const html = caps.criticHTML({ issues: [{ kind: 'missing', text: 'tests <b>' }, { kind: 'risky', text: 'x' }] });
  assert.match(html, /<details class="plan-critic">/);
  assert.match(html, /Critic: 2 concerns/);
  assert.match(html, /tests &lt;b&gt;/);
  assert.match(caps.criticHTML({ issues: [] }), /Critic: no concerns/);
  assert.equal(caps.criticHTML({ issues: [], skipped: 'no reviewer role' }), '');
  assert.equal(caps.criticHTML(null), '');
});

test('critic prompt asks for one revision in autonomous mode only', () => {
  const caps = load();
  const c = { issues: [{ kind: 'assumption', text: 'API returns JSON' }] };
  assert.match(caps.criticPrompt(c, true), /\[assumption\] API returns JSON[\s\S]*Revise the plan ONCE/);
  assert.doesNotMatch(caps.criticPrompt(c, false), /Revise the plan ONCE/);
  assert.equal(caps.criticPrompt({ issues: [] }, true), '');
});

test('bet chip lands on the tool summary with the outcome mark', () => {
  const caps = load();
  const appended = [];
  const card = { querySelector: sel => (sel === 'summary' ? { appendChild: el => appended.push(el) } : null) };
  caps.betChip(card, { p: '80', passed: 'true', command: 'go test ./...' });
  caps.betChip(card, { p: '30', passed: 'false' });
  caps.betChip(null, { p: '50', passed: 'true' });
  assert.equal(appended.length, 2);
  assert.equal(appended[0].textContent, 'bet 80% ✓');
  assert.match(appended[0].className, /bet-ok/);
  assert.equal(appended[1].textContent, 'bet 30% ✗');
});

test('/debug switches to debug mode and sends only the task', async () => {
  const caps = load();
  assert.equal(caps.interceptDebug('fix the bug'), false);
  assert.equal(caps.interceptDebug('/debugger'), false);
});
