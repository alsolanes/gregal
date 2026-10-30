const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

// plugins.js amb un DOM de joguina: botons de la barra lateral, una vista
// actual i una API falsa per a /api/plugins.
async function load(responses) {
  const buttons = ['raco', 'partits', 'arena', 'vents'].map(view => ({ view, hidden: false }));
  const events = [];
  const calls = [];
  let current = 'arena';
  const window = {
    gregal: {
      viewActual: () => current,
      setView: v => { current = v; },
      api: async (url, opts) => {
        calls.push({ url, opts });
        const r = responses.shift();
        return { ok: r.status === 200, status: r.status, json: async () => r.body };
      },
    },
  };
  const document = {
    querySelectorAll: sel => {
      const m = /data-view="([^"]+)"/.exec(sel);
      return buttons.filter(b => m && b.view === m[1]);
    },
    addEventListener: () => {},
    dispatchEvent: e => events.push(e),
  };
  class CustomEvent { constructor(type, init) { this.type = type; this.detail = init && init.detail; } }
  const context = vm.createContext({ window, document, CustomEvent, JSON });
  const mod = new vm.SourceTextModule(fs.readFileSync(path.join(__dirname, 'plugins.js'), 'utf8'), { context, identifier: 'plugins.js' });
  await mod.link(() => { throw new Error('plugins.js no importa res'); });
  await mod.evaluate();
  return { plugins: mod.namespace.plugins, buttons, events, calls, window, current: () => current };
}

const list = (arena, games, corner) => ({
  plugins: [
    { id: 'corner', view: 'raco', enabled: corner },
    { id: 'games', view: 'partits', enabled: games },
    { id: 'arena', view: 'arena', enabled: arena },
  ],
  admin: true,
});

test('sense dades tot es mostra; amb dades, els apagats s\'amaguen', async () => {
  const { plugins, buttons, events, current, window } = await load([{ status: 200, body: list(false, true, true) }]);
  assert.equal(window.gregalPlugins, plugins);
  assert.equal(plugins.viewAllowed('arena'), true, 'abans de saber-ho, tot val');
  assert.equal(await plugins.load(), true);
  assert.equal(plugins.isAdmin(), true);
  assert.equal(plugins.enabled('arena'), false);
  assert.equal(plugins.viewAllowed('arena'), false);
  assert.equal(plugins.viewAllowed('vents'), true, 'les vistes del nucli no depenen dels plugins');
  assert.deepEqual(buttons.map(b => [b.view, b.hidden]), [['raco', false], ['partits', false], ['arena', true], ['vents', false]]);
  assert.equal(current(), 'agent', 'si mires un plugin que s\'apaga, tornes al xat');
  assert.equal(events.at(-1).type, 'gregal:plugins');
});

test('set envia {id, enabled} i torna a mostrar el plugin', async () => {
  const { plugins, buttons, calls } = await load([{ status: 200, body: list(false, true, true) }, { status: 200, body: list(true, true, true) }]);
  await plugins.load();
  await plugins.set('arena', true);
  assert.equal(calls[1].url, '/api/plugins');
  assert.deepEqual(JSON.parse(calls[1].opts.body), { id: 'arena', enabled: true });
  assert.equal(buttons.find(b => b.view === 'arena').hidden, false);
});

test('un error del servidor (403) es propaga i no canvia res', async () => {
  const { plugins } = await load([{ status: 200, body: list(true, true, true) }, { status: 403, body: { error: 'només per a l\'administrador' } }]);
  await plugins.load();
  await assert.rejects(plugins.set('arena', false), /administrador/);
  assert.equal(plugins.enabled('arena'), true);
});
