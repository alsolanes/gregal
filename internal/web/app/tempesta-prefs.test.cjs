// «A fons» a Preferències (tempesta-prefs.js) i, al xat, què fa cada agent
// de la Tempesta (winds-workspace.js).
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function node(tag) {
  return {
    tagName: tag, className: '', textContent: '', title: '', value: '', children: [], dataset: {}, attrs: {},
    classList: { add() {}, remove() {}, toggle() {} },
    append(...n) { this.children.push(...n); },
    replaceChildren(...n) { this.children = n; },
    setAttribute(k, v) { this.attrs[k] = v; }, style: { setProperty() {} },
  };
}
const all = n => [n, ...(n.children || []).flatMap(all)];

function loadPrefs(lang, responses) {
  const calls = [];
  const window = {
    api: async (url, opts = {}) => {
      calls.push({ url, method: opts.method || 'GET', body: opts.body && JSON.parse(opts.body) });
      const r = responses(url, opts);
      return { ok: r.ok !== false, json: async () => r.body, text: async () => r.text || '' };
    },
  };
  const document = { createElement: node };
  let src = fs.readFileSync(path.join(__dirname, 'tempesta-prefs.js'), 'utf8');
  src = src.replace(/^import .*$/m, 'const idioma = () => LANG;').replace(/^export const /m, 'const ') + '\nthis.tempestaPrefs = tempestaPrefs;';
  const sandbox = { window, document, LANG: lang };
  vm.runInNewContext(src, sandbox, { filename: 'tempesta-prefs.js' });
  return { prefs: sandbox.tempestaPrefs, calls };
}

const models = { models: { q: ['fort', 'rapid'], p: ['m'] } };

test('the Deep section has a model picker with Automatic for each agent and the synthesis', async () => {
  const { prefs } = loadPrefs('en', url => url === '/api/models' ? { body: models } : { body: { models: { direct: '', sources: 'q/fort', skeptic: '', synth: '' } } });
  assert.match(prefs.section(), /Deep/);
  const box = node('div');
  await prefs.bind({ querySelector: sel => (sel === '#prefsTempesta' ? box : null) });
  const selects = all(box).filter(n => n.tagName === 'select');
  assert.deepEqual(selects.map(s => s.attrs['aria-label']), ['Direct', 'Sources', 'Devil’s advocate', 'Synthesis']);
  for (const s of selects) {
    assert.equal(s.children[0].value, '');
    assert.equal(s.children[0].textContent, 'Automatic');
    assert.deepEqual(s.children.slice(1).map(o => o.value), ['p/m', 'q/fort', 'q/rapid']);
  }
  assert.equal(selects[0].value, '');
  assert.equal(selects[1].value, 'q/fort', 'shows what is saved');
});

test('choosing a model saves that slot, and Automatic sends an empty model', async () => {
  const { prefs, calls } = loadPrefs('ca', (url, opts) => {
    if (url === '/api/models') return { body: models };
    const b = opts.body ? JSON.parse(opts.body) : null;
    return { body: { models: { direct: '', sources: '', skeptic: '', synth: b ? b.model : '' } } };
  });
  const box = node('div');
  await prefs.bind({ querySelector: () => box });
  const synth = all(box).filter(n => n.tagName === 'select')[3];
  assert.equal(synth.attrs['aria-label'], 'Síntesi');
  synth.value = 'q/rapid';
  await synth.onchange();
  synth.value = '';
  await synth.onchange();
  const posts = calls.filter(c => c.method === 'POST');
  assert.deepEqual(posts.map(c => [c.url, c.body]), [
    ['/api/winds/tempesta/models', { slot: 'synth', model: 'q/rapid' }],
    ['/api/winds/tempesta/models', { slot: 'synth', model: '' }],
  ]);
  assert.ok(all(box).some(n => n.textContent === 'Desat'));
});

test('a rejected model puts the picker back and says why', async () => {
  const { prefs } = loadPrefs('en', (url, opts) => {
    if (url === '/api/models') return { body: models };
    if (opts.method === 'POST') return { ok: false, text: 'model no disponible al provider' };
    return { body: { models: { direct: 'q/fort', sources: '', skeptic: '', synth: '' } } };
  });
  const box = node('div');
  await prefs.bind({ querySelector: () => box });
  const direct = all(box).filter(n => n.tagName === 'select')[0];
  direct.value = 'q/rapid';
  await direct.onchange();
  assert.equal(direct.value, 'q/fort');
  assert.ok(all(box).some(n => /Could not save: model no disponible/.test(n.textContent)));
});

function loadChat(lang) {
  const window = { gregalWinds: {} };
  const document = { documentElement: { lang }, addEventListener() {}, createElement: node };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'winds-workspace.js'), 'utf8'), {
    window, document, getComputedStyle: () => ({ getPropertyValue: () => '' }),
  });
  return window.gregalWindChat.tempestaAgentRow;
}

test('each Tempesta agent in the chat shows what it does, in the app language', () => {
  const agent = { index: 2, angle: 'skeptic', label: 'Advocat del diable', ca: 'Advocat del diable', en: 'Devil\'s advocate',
    desc_ca: 'Busca el parany.', desc_en: 'Looks for the trap.', model: 'm-sceptic', state: 'working' };
  for (const [lang, want] of [['en', 'Looks for the trap.'], ['ca', 'Busca el parany.']]) {
    const row = loadChat(lang)(agent);
    const desc = all(row).find(n => n.className === 'vw-tempesta-desc');
    assert.ok(desc, lang + ': the description is shown');
    assert.equal(desc.textContent, want);
    assert.equal(row.title, want);
  }
});

test('a rotllana voice without a description shows no empty line', () => {
  const row = loadChat('en')({ index: 0, angle: 'xaloc', label: 'Xaloc', ca: 'Xaloc', en: 'Xaloc', state: 'done' });
  assert.ok(!all(row).some(n => n.className === 'vw-tempesta-desc'));
  assert.equal(row.title, '');
});
