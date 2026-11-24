// La conversa d'un vent agafa el seu color i en diu l'estat (winds-workspace.js):
// la barreja de la bombolla i l'etiqueta de la capçalera, com a funcions pures.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function load() {
  const window = {};
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'winds-workspace.js'), 'utf8'), { window, document: { documentElement: { lang: 'en' }, addEventListener() {} } });
  return window.gregalWindChat;
}

const luminance = hex => {
  const c = i => { const v = parseInt(hex.slice(i, i + 2), 16) / 255; return v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4; };
  return .2126 * c(1) + .7152 * c(3) + .0722 * c(5);
};
const contrast = (a, b) => { const [x, y] = [luminance(a), luminance(b)].sort((p, q) => q - p); return (x + .05) / (y + .05); };

// Els vuit vents de la colla (presetDefaults a winds.js) i el blau de Gregal (winds_gregal.go).
const winds = ['#EFC69A', '#EFA68F', '#A8CDE2', '#BFCFA4', '#E8AEC2', '#C1B4DE', '#EACF87', '#9ED3C5', '#3D7BFF'];
// --surface-hi (el fons de la bombolla) i les tintes que s'hi escriuen: --text, --ink i --muted (el nom).
const themes = {
  dark: { base: '#24282D', inks: ['#DEE2E7', '#F2F4F7', '#A7AEB8'] },
  light: { base: '#DEE5DE', inks: ['#1C2D35', '#10202A', '#4C5F5C'] },
};

test('a wind bubble takes a tint of the wind’s colour and its text stays at 4.5:1 or more in dark and light', () => {
  const { mix, share } = load();
  for (const [theme, { base, inks }] of Object.entries(themes)) {
    for (const color of winds) {
      const amount = share(color, base, inks);
      assert.ok(amount > 0, `${theme} ${color}: the bubble is tinted`);
      const bubble = mix(color, base, amount / 100);
      assert.notEqual(bubble.toLowerCase(), base.toLowerCase(), `${theme} ${color}`);
      for (const ink of inks) assert.ok(contrast(bubble, ink) >= 4.5, `${theme} ${color} on ${bubble} with ${ink}: ${contrast(bubble, ink).toFixed(2)}`);
    }
  }
});

test('the mix goes from the base to the colour as CSS color-mix in srgb does', () => {
  const { mix } = load();
  assert.equal(mix('#FF0000', '#000000', 0), '#000000');
  assert.equal(mix('#FF0000', '#000000', 1), '#ff0000');
  assert.equal(mix('#FFFFFF', '#000000', .5), '#808080');
});

test('the header says what the wind is doing: working, waiting for you, done or nothing', () => {
  const { mark } = load();
  const chat = 'company-xaloc-s1', wind = 'xaloc';
  assert.equal(mark({ pending: { thread: chat, wind } }, chat, wind), 'working');
  assert.equal(mark({ pending: null, asks: [{ key: 'k', wind }] }, chat, wind), 'waiting');
  assert.equal(mark({ asks: [], done: chat }, chat, wind), 'done');
  assert.equal(mark({ asks: [] }, chat, wind), '');
  // Una pregunta d'un altre vent, un torn d'una altra conversa o un «fet» d'una altra no compten.
  assert.equal(mark({ pending: { thread: 'other', wind }, asks: [{ key: 'k', wind: 'tramuntana' }], done: 'other' }, chat, wind), '');
  // Amb el torn aturat esperant la teva resposta, el vent t'espera.
  assert.equal(mark({ pending: { thread: chat, wind }, asks: [{ key: 'k', wind }] }, chat, wind), 'waiting');
});

// El vent present a la cantonada del xat (087) fa cara del mateix estat: les cares de feina
// de moodOf mentre el torn corre, la postura d'espera amb una pregunta seva, «fet» en acabar.
test('the corner wind’s mood follows the state: working moods, waiting with an ask, done after the turn', () => {
  const window = { document: { createElementNS() {}, createElement() {} }, performance: { now: () => 0 } };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'winds.js'), 'utf8'), { window });
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'winds-workspace.js'), 'utf8'), { window, document: { documentElement: { lang: 'en' }, addEventListener() {} } });
  const { cornerMood } = window.gregalWindChat;
  const chat = 'company-xaloc-s1', wind = 'xaloc';
  assert.equal(cornerMood({ pending: { thread: chat, wind, phase: 'thinking' } }, chat, wind), 'thinking');
  assert.equal(cornerMood({ pending: { thread: chat, wind, phase: 'searching' } }, chat, wind), 'searching');
  assert.equal(cornerMood({ pending: { thread: chat, wind, phase: 'working', text: 'Hi ha' } }, chat, wind), 'writing');
  assert.equal(cornerMood({ pending: { thread: chat, wind, phase: 'working' }, asks: [{ key: 'k', wind }] }, chat, wind), 'waiting');
  assert.equal(cornerMood({ asks: [], done: chat }, chat, wind), 'done');
  assert.equal(cornerMood({ asks: [] }, chat, wind), 'idle');
});
