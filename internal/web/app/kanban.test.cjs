const { test } = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

// kanban.js és un mòdul: es carrega amb un DOM mínim i s'hi proven les
// peces pures (codificació de tecles, ratolí i estat visible).
// El mòdul queda en memòria cau després del primer import: un sol entorn.
const store = {};
let loaded;
async function load() {
  if (loaded) return loaded;
  global.window = { gregalT: k => k };
  global.document = { querySelectorAll: () => [], getElementById: () => null, addEventListener: () => {}, hidden: false };
  global.localStorage = { getItem: k => store[k] ?? null, setItem: (k, v) => { store[k] = String(v); } };
  global.location = { search: '?kanban=1' };
  const mod = await import(pathToFileURL(path.join(__dirname, 'kanban.js')).href);
  loaded = { mod, store };
  return loaded;
}

const key = (k, o = {}) => Object.assign({ key: k, ctrlKey: false, altKey: false, shiftKey: false, metaKey: false }, o);

test('keys are encoded like xterm', async () => {
  const { mod } = await load();
  const enc = mod.keyToBytes;
  assert.equal(enc(key('a')), 'a');
  assert.equal(enc(key('Enter')), '\r');
  assert.equal(enc(key('Backspace')), '\x7f');
  assert.equal(enc(key('Escape')), '\x1b');
  assert.equal(enc(key('c', { ctrlKey: true })), '\x03');
  assert.equal(enc(key('x', { altKey: true })), '\x1bx');
  assert.equal(enc(key('Tab', { shiftKey: true })), '\x1b[Z');
  assert.equal(enc(key('ArrowUp')), '\x1b[A');
  assert.equal(enc(key('ArrowUp'), true), '\x1bOA', 'DECCKM');
  assert.equal(enc(key('ArrowRight', { ctrlKey: true })), '\x1b[1;5C');
  assert.equal(enc(key('PageDown')), '\x1b[6~');
  assert.equal(enc(key('Delete', { shiftKey: true })), '\x1b[3;2~');
  assert.equal(enc(key('Shift', { shiftKey: true })), null);
  assert.equal(enc(key('v', { metaKey: true })), null);
});

test('mouse events use SGR 1006 with 1-based cells', async () => {
  const { mod } = await load();
  assert.equal(mod.mouseBytes(64, 0, 0), '\x1b[<64;1;1M');
  assert.equal(mod.mouseBytes(0, 9, 4, true), '\x1b[<0;10;5m');
});

test('visible state prefers queue and stop over the TUI state', async () => {
  const { mod } = await load();
  assert.equal(mod.cardState({ queued: true, alive: false }), 'queued');
  assert.equal(mod.cardState({ alive: false, activity: 'working' }), 'stopped');
  assert.equal(mod.cardState({ alive: true, activity: '' }), 'starting');
  assert.equal(mod.cardState({ alive: true, activity: 'approval' }), 'approval');
});

test('?kanban=1 turns the experimental view on', async () => {
  const { mod, store } = await load();
  assert.equal(store.gregal_exp_kanban, '1');
  assert.equal(mod.kanbanEnabled(), true);
});
