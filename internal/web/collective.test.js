// Com es diuen els ajudants a la web: el nom triat substitueix «vent/vents»
// (i wind/winds) als textos, menys la marca «Gregal Vents»; i el singular es
// dedueix del plural com a l'Android.
import test from 'node:test';
import assert from 'node:assert/strict';

globalThis.window = globalThis.window || {};
globalThis.localStorage = { getItem: () => null, setItem() {} };
globalThis.document = { documentElement: { lang: 'ca' } };
const { apply, singularOf, options } = await import('./app/collective.js');

test('el nom triat substitueix vent i vents conservant la majúscula', () => {
  const c = { singular: 'fullet', plural: 'fullets' };
  assert.equal(apply('Els teus vents', c), 'Els teus fullets');
  assert.equal(apply('Nou vent', c), 'Nou fullet');
  assert.equal(apply('Vents', c), 'Fullets');
  assert.equal(apply('Obre Gregal Vents', c), 'Obre Gregal Vents');
  assert.equal(apply('Your winds and a wind', { singular: 'pirate', plural: 'pirates' }), 'Your pirates and a pirate');
  assert.equal(apply('Windows i ventilador', c), 'Windows i ventilador');
  assert.equal(apply('Els teus vents', { singular: '', plural: '' }), 'Els teus vents');
});

test('un a partir de molts', () => {
  assert.equal(singularOf('pirates', 'ca'), 'pirata');
  assert.equal(singularOf('minairons', 'ca'), 'minairó');
  assert.equal(singularOf('pirates', 'en'), 'pirate');
  assert.equal(singularOf('witches', 'en'), 'witch');
  assert.equal(options('ca')[0][1], 'Vents');
  assert.equal(options('en')[0][1], 'Winds');
});

test('els textos que expliquen el canvi no es tradueixen al nom triat', async () => {
  const src = (await import('node:fs')).readFileSync(new URL('./app/i18n.js', import.meta.url), 'utf8');
  assert.match(src, /clau\.startsWith\('collective\.'\)/);
});
