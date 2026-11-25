// La conversa d'un vent agafa el seu color i en diu l'estat (winds-workspace.js):
// el botó Permet amb el color del vent i l'etiqueta de la capçalera, com a funcions pures.
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
// Les dues tintes del tema per al botó Permet: --ink i --bg (una clara i una fosca).
const themes = {
  dark: ['#F2F4F7', '#131518'],
  light: ['#10202A', '#F4F6F3'],
};

test('the Allow button takes the wind’s colour and its text stays at 4.5:1 or more in dark and light', () => {
  const { allow } = load();
  for (const [theme, inks] of Object.entries(themes)) {
    for (const color of winds) {
      const { fill, ink } = allow(color, inks);
      assert.ok(inks.includes(ink), `${theme} ${color}: the text is one of the theme's inks`);
      assert.ok(contrast(fill, ink) >= 4.5, `${theme} ${color}: ${ink} on ${fill} is ${contrast(fill, ink).toFixed(2)}`);
    }
    // Els vents pastel queden amb el seu color tal qual.
    assert.equal(allow('#EFC69A', inks).fill, '#efc69a', theme);
  }
  // El blau de Gregal es llegeix amb la tinta fosca del tema fosc; amb les del clar no arriba a 4.5:1 i cedeix una mica.
  assert.equal(allow('#3D7BFF', themes.dark).fill, '#3d7bff');
  const blue = allow('#3D7BFF', themes.light).fill;
  assert.notEqual(blue, '#3d7bff');
  assert.ok(contrast(blue, '#3D7BFF') < 1.6, `${blue} is still Gregal's blue`);
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
