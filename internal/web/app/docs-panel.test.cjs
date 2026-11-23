// El panell de documents (docs-panel.js) amb un DOM mínim i una API falsa:
// un document d'un vent («Els meus documents», scope mine) fa totes les
// crides amb scope=mine; un del projecte (Codi) no en fa cap.
const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { onDocChanged } = require('./doc-view.js');

class El {
  constructor(id = '') { this.id = id; this.hidden = false; this.attributes = {}; this.children = []; this.textContent = ''; this.href = ''; }
  setAttribute(k, v) { this.attributes[k] = String(v); }
  append(...n) { this.children.push(...n); }
  replaceChildren(...n) { this.children = n; }
  addEventListener(k, f) { if (k === 'click') this.onclick = f; }
  click() { return this.onclick?.({ target: this }); }
}

const els = new Map();
const made = [];
const byId = id => made.find(e => e.id === id) || els.get(id) || els.set(id, new El(id)).get(id);
const calls = [];
const opened = [];
let edit = null;
const json = data => ({ ok: true, status: 200, json: async () => data });

globalThis.window = globalThis;
globalThis.document = { getElementById: byId, createElement: () => { const e = new El(); made.push(e); return e; }, head: new El() };
globalThis.localStorage = { getItem: () => 'en' };
globalThis.Option = function Option(text, value) { return { text, value }; };
globalThis.open = (url, name) => opened.push([url, name]);
globalThis.gregalMD = { render: s => s };
globalThis.gregalDocView = {
  onDocChanged,
  view: (box, doc, render, e) => { edit = e; return { open() {}, apply() {} }; },
};
globalThis.gregal = {
  sys() {},
  api: async (url, opts = {}) => {
    calls.push(url);
    if (url.includes('/stream')) return { ok: true, body: { getReader: () => ({ read: () => new Promise(() => {}) }) } };
    if (/^\/api\/docs(\?|$)/.test(url)) return json({ items: [{ id: 'w1', title: 'Carta' }] });
    if (url.includes('/blocks')) return json({});
    return json({ id: 'w1', title: 'Carta', version: 1, blocks: [] });
  },
};

const settle = () => new Promise(r => setTimeout(r, 5));
const load = import(pathToFileURL(path.join(__dirname, 'docs-panel.js')).href);

// Obre un document com ho fa un doc_changed i n'apunta totes les URLs: llista,
// stream, document, descàrrega, edició de blocs i la finestra pròpia.
async function urlsOf(event) {
  await load;
  calls.length = 0; opened.length = 0;
  window.gregalDocs.changed(event);
  await settle();
  await edit.put('b1', 'x', 1);
  await edit.post('', 'y');
  await edit.del('b1', 1);
  const urls = [...calls, byId('docsDownload').href];
  await byId('docsWindow').click();
  return { urls, window: opened[0]?.[0] };
}

test('a wind document opens in the panel and every call carries scope=mine', async () => {
  const { urls, window: win } = await urlsOf({ id: 'w1', title: 'Carta', scope: 'mine' });
  for (const want of ['/api/docs?scope=mine', '/api/docs/w1/stream?scope=mine', '/api/docs/w1?scope=mine',
    '/api/docs/w1/blocks/b1?scope=mine', '/api/docs/w1/blocks?scope=mine', '/api/docs/w1/blocks/b1?base_version=1&scope=mine']) {
    assert.ok(urls.includes(want), want + ' in ' + urls.join(' '));
  }
  assert.match(urls.at(-1), /^\/api\/docs\/w1\/export\?format=docx&name=Carta\.docx&scope=mine$/);
  for (const u of urls) assert.match(u, /[?&]scope=mine(&|$)/, u);
  assert.equal(win, '/app/doc/w1?session=default&scope=mine', '«Open in a window» opens the personal document');
});

test('a Codi document never asks for scope=mine', async () => {
  const { urls, window: win } = await urlsOf({ id: 'p1', title: 'Pla' });
  assert.ok(urls.includes('/api/docs/p1/stream') && urls.includes('/api/docs/p1/blocks'), urls.join(' '));
  for (const u of urls) assert.doesNotMatch(u, /scope=/, u);
  assert.equal(win, '/app/doc/p1?session=default');
});

test('the same wind document already open is only brought forward', async () => {
  await load;
  window.gregalDocs.changed({ id: 'w1', title: 'Carta', scope: 'mine' });
  await settle();
  calls.length = 0;
  window.gregalDocs.changed({ id: 'w1', title: 'Carta', scope: 'mine' });
  await settle();
  assert.ok(!calls.some(u => u.includes('/stream')), 'no new stream: ' + calls.join(' '));
  assert.equal(byId('docsPanel').hidden, false);
  assert.equal(byId('docsList').attributes['aria-label'], 'My documents', 'the list is the person’s own documents');
  byId('docsClose').click();
});

test('on the desktop, «Open in a window» asks for the personal document window', async () => {
  await load;
  const asked = [];
  window.gregalDesktop = { openDocWindow: (...a) => asked.push(a) };
  try {
    window.gregalDocs.changed({ id: 'w1', title: 'Carta', scope: 'mine' });
    await settle();
    assert.equal(byId('docsWindow').hidden, false, 'the button shows for a personal document');
    await byId('docsWindow').click();
    window.gregalDocs.changed({ id: 'p1', title: 'Pla' });
    await settle();
    await byId('docsWindow').click();
  } finally {
    delete window.gregalDesktop;
  }
  assert.deepEqual(asked, [['w1', 'default', 'mine'], ['p1', 'default', '']]);
});

test('a deck opens in the slide view, saves slides and downloads as PowerPoint', async () => {
  await load;
  const sent = [];
  const api = window.gregal.api;
  let deckEdit = null;
  window.gregalDeckView = { view: (box, doc, render, e) => { deckEdit = e; return { open() {}, apply() {} }; } };
  window.gregal.api = async (url, opts = {}) => {
    if (opts.body) sent.push([url, JSON.parse(opts.body)]);
    if (/^\/api\/docs\/k1(\?|$)/.test(url)) return json({ id: 'k1', title: 'Llançament', kind: 'deck', version: 1, blocks: [] });
    return api(url, opts);
  };
  try {
    edit = null;
    window.gregalDocs.open('k1', 'mine', true);
    await settle();
    assert.equal(edit, null, 'the text view is not used for a deck');
    assert.equal(deckEdit.tx('Diapositiva', 'Slide'), 'Slide');
    await deckEdit.put('s1', { layout: 'content', title: 'Fases' }, 2);
    await deckEdit.post('s1', { layout: 'title' });
    assert.deepEqual(sent, [
      ['/api/docs/k1/blocks/s1?scope=mine', { slide: { layout: 'content', title: 'Fases' }, base_version: 2 }],
      ['/api/docs/k1/blocks?scope=mine', { after: 's1', slide: { layout: 'title' } }],
    ]);
    assert.equal(byId('docsDownload').href, '/api/docs/k1/export?format=pptx&name=Llan%C3%A7ament.pptx&scope=mine');
    assert.equal(byId('docsDownload').textContent, 'Download as PowerPoint');
  } finally {
    window.gregal.api = api;
    byId('docsClose').click();
  }
});
