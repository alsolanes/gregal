const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

async function loadConvs() {
  const stored = new Map();
  const calls = { opens: 0, resumes: 0, switches: [], views: [] };
  const window = { gregalSession: 'default' };
  const sessions = {
    async open() { calls.opens++; window.gregalSession = 'resumed-1'; },
    async list() { return calls.opens ? [{ id: 'resumed-1' }] : [{ id: 'default' }]; },
    async switchTo(id) { calls.switches.push(id); window.gregalSession = id; },
  };
  window.gregalPanels = { sessions };
  window.gregal = {
    isBusy() { return true; }, state: { cwd: 'C:/project' },
    async api(url) {
      if (url === '/api/resume') { calls.resumes++; return { ok: true, async json() { return { title: 'Old task', msgs: 2, transcript: [] }; } }; }
      return { ok: true, async json() { return []; } };
    },
    sys() {}, activityItem() {}, setView(view) { calls.views.push(view); }, refresh() {},
  };
  const context = vm.createContext({
    window, localStorage: {
      getItem(key) { return stored.get(key) || null; },
      setItem(key, value) { stored.set(key, String(value)); },
    },
    document: { getElementById() { return null; } },
  });
  const source = fs.readFileSync(path.join(__dirname, 'convs.js'), 'utf8');
  const mod = new vm.SourceTextModule(source, { context, identifier: 'convs.js' });
  await mod.link(() => { throw new Error('convs.js no hauria de tenir imports'); });
  await mod.evaluate();
  return { convs: mod.namespace.convs, calls, window };
}

test('reopening a sidebar conversation reuses its tab and returns to the chat', async () => {
  const { convs, calls } = await loadConvs();
  await convs.obre('sessio-20261019-old');
  await convs.obre('sessio-20261019-old');

  assert.equal(calls.opens, 1);
  assert.equal(calls.resumes, 1);
  assert.deepEqual(calls.switches, ['resumed-1']);
  assert.deepEqual(calls.views, ['agent', 'agent']);
});
