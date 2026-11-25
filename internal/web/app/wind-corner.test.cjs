// 138: el vent de la cantonada del xat (winds-workspace.js) no salta entre la cantonada i la capçalera
// a la mínima, i canvia de cara amb calma quan les eines es van succeint.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function load() {
  const window = { document: { createElementNS() {}, createElement() {} }, performance: { now: () => 0 } };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'winds.js'), 'utf8'), { window });
  // Els rellotges del context són els del test: amb mock.timers activat, els falsos.
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'winds-workspace.js'), 'utf8'), { window, setTimeout, clearTimeout, Date, document: { documentElement: { lang: 'en' }, addEventListener() {} } });
  return window.gregalWindChat;
}

test('scrolling back and forth around one threshold does not move the wind between the corner and the header', () => {
  const { cornerAt } = load();
  // A la cantonada, en pujar a poc a poc: es queda fins que t'has allunyat clarament (més de 240 px).
  let at = 'corner';
  for (const gap of [100, 150, 170, 140, 200, 160, 230, 180]) {
    at = cornerAt(at, gap, true);
    assert.equal(at, 'corner', `${gap} px from the end`);
  }
  at = cornerAt(at, 260, true);
  assert.equal(at, 'top');
  // A dalt, en baixar: no torna fins que gairebé ets al final (menys de 48 px).
  for (const gap of [230, 170, 150, 90, 60, 160, 50]) {
    at = cornerAt(at, gap, false);
    assert.equal(at, 'top', `${gap} px from the end`);
  }
  assert.equal(cornerAt(at, 30, false), 'corner');
});

test('the thread growing under a wind that streams does not fly it up: only scrolling up does', () => {
  const { cornerAt } = load();
  assert.equal(cornerAt('corner', 600, false), 'corner');
  assert.equal(cornerAt('corner', 600, true), 'top');
});

test('the tool that is running says what kind of work the corner wind shows, over the talk around it', () => {
  const { cornerMood } = load();
  const chat = 'company-xaloc-s1', wind = 'xaloc', at = (steps, text = '') => cornerMood({ pending: { thread: chat, wind, phase: 'working', text, steps } }, chat, wind);
  assert.equal(at([{ tool: 'read', done: false }]), 'reading');
  assert.equal(at([{ tool: 'read', done: true }, { tool: 'web_search', done: false }], 'Ho miro'), 'searching');
  assert.equal(at([{ tool: 'edit', done: false }]), 'writing');
  assert.equal(at([{ tool: 'bash', done: false }]), 'working');
  // Cap eina en marxa: torna a dir-ho la fase o el text.
  assert.equal(at([{ tool: 'read', done: true }], 'Ja ho tinc'), 'writing');
  assert.equal(at([{ tool: 'read', done: true }]), 'working');
});

test('a burst of tool and talk events in 2 s changes the corner face at most twice, and then shows the latest work', t => {
  t.mock.timers.enable({ apis: ['setTimeout', 'Date'], now: 0 });
  const { cornerMood, heldMood } = load();
  const chat = 'company-xaloc-s1', wind = 'xaloc', state = { asks: [], pending: null };
  // Com paintCorner: només es torna a dibuixar (i es reinicia l'animació) quan la cara que es veu canvia.
  const shown = [];
  const paint = () => { const mood = heldMood(wind, cornerMood(state, chat, wind), paint); if (shown[shown.length - 1] !== mood) shown.push(mood); };
  // Pensa, busca, xerra, treballa, busca, xerra, treballa, busca: una cosa nova cada 150-400 ms.
  const events = [
    [0, { phase: 'thinking', text: '' }], [150, { phase: 'searching', text: '' }], [400, { phase: 'working', text: 'Mirant-ho' }],
    [700, { phase: 'working', text: '' }], [900, { phase: 'searching', text: '' }], [1300, { phase: 'working', text: 'Ja ho' }],
    [1600, { phase: 'working', text: '' }], [1900, { phase: 'searching', text: '' }],
  ];
  let now = 0;
  for (const [at, pending] of events) {
    t.mock.timers.tick(at - now); now = at;
    state.pending = { thread: chat, wind, ...pending };
    paint();
  }
  t.mock.timers.tick(2000 - now);
  assert.ok(shown.length - 1 <= 2, shown.join(' → '));
  assert.equal(shown[0], 'thinking');
  // Passada l'estona mínima, la cara arriba a la feina d'ara: no s'hi queda encallada.
  t.mock.timers.tick(1000);
  assert.equal(shown[shown.length - 1], 'searching', shown.join(' → '));
});

test('the end of the turn and a question for you show at once, without waiting for the hold', t => {
  t.mock.timers.enable({ apis: ['setTimeout', 'Date'], now: 0 });
  const { cornerMood, heldMood } = load();
  const chat = 'company-xaloc-s1', wind = 'xaloc', none = () => {};
  assert.equal(heldMood(wind, cornerMood({ pending: { thread: chat, wind, phase: 'searching' } }, chat, wind), none), 'searching');
  t.mock.timers.tick(200);
  assert.equal(heldMood(wind, cornerMood({ pending: { thread: chat, wind, phase: 'searching' }, asks: [{ key: 'k', wind }] }, chat, wind), none), 'waiting');
  assert.equal(heldMood(wind, cornerMood({ pending: { thread: chat, wind, phase: 'working' } }, chat, wind), none), 'working');
  t.mock.timers.tick(200);
  assert.equal(heldMood(wind, cornerMood({ asks: [], done: chat }, chat, wind), none), 'done');
});
