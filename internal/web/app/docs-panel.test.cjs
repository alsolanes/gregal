// El panell de documents (docs-panel.js) amb un DOM mínim i una API falsa:
// un document d'un vent («Els meus documents», scope mine) fa totes les
// crides amb scope=mine; un del projecte (Codi) no en fa cap.
const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { onDocChanged } = require('./doc-view.js');

class El {
  constructor(id = '') {
    this.id = id; this.hidden = false; this.attributes = {}; this.children = []; this.textContent = ''; this.href = ''; this.className = '';
    this.props = {};
    this.style = { setProperty: (k, v) => { this.props[k] = v; } };
    const classes = new Set();
    this.classList = { toggle: (c, on) => (on ? classes.add(c) : classes.delete(c)), contains: c => classes.has(c) };
  }
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
let box = null;
const THEME = { '--doc-page-w': '595.3', '--doc-body-font': '"Georgia"', '--doc-h1-color': '#123456' };
const json = data => ({ ok: true, status: 200, json: async () => data });

globalThis.window = globalThis;
const listeners = {};
const dispatched = [];
globalThis.document = { getElementById: byId, createElement: () => { const e = new El(); made.push(e); return e; }, head: new El(), addEventListener: (k, f) => { listeners[k] = f; }, dispatchEvent: e => dispatched.push(e) };
globalThis.localStorage = { getItem: () => 'en' };
globalThis.location = { href: 'http://127.0.0.1:8097/' };
globalThis.Option = function Option(text, value) { return { text, value }; };
globalThis.open = (url, name) => opened.push([url, name]);
globalThis.gregalMD = { render: s => s };
globalThis.gregalDocView = {
  onDocChanged,
  view: (b, doc, render, e) => { box = b; edit = e; return { focus() {}, apply() {} }; },
};
globalThis.gregal = {
  sys() {},
  api: async (url, opts = {}) => {
    calls.push(url);
    if (url.includes('/stream')) return { ok: true, body: { getReader: () => ({ read: () => new Promise(() => {}) }) } };
    if (/^\/api\/docs(\?|$)/.test(url)) return json({ items: [{ id: 'w1', title: 'Carta' }] });
    if (/^\/api\/docs\/theme(\?|$)/.test(url)) return json({ vars: THEME });
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
  assert.equal(win, '/app/doc/w1?session=default&lang=en&scope=mine', '«Open in a window» opens the personal document, in the app language');
  assert.equal(opened[0][1], 'gregal-doc-mine-w1', 'the personal document has its own window, apart from a project one with the same id');
});

test('a Codi document never asks for scope=mine', async () => {
  const { urls, window: win } = await urlsOf({ id: 'p1', title: 'Pla' });
  assert.ok(urls.includes('/api/docs/p1/stream') && urls.includes('/api/docs/p1/blocks'), urls.join(' '));
  for (const u of urls) assert.doesNotMatch(u, /scope=/, u);
  assert.equal(win, '/app/doc/p1?session=default&lang=en');
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

test('a sheet opens in the grid view, saves cells and new sheets, and downloads as Excel', async () => {
  await load;
  const sent = [];
  const api = window.gregal.api;
  let sheetEdit = null, write = null;
  window.gregalSheetView = { view: (box, doc, render, e) => { sheetEdit = e; return { open(...a) { write = a; }, apply() {} }; } };
  window.gregal.api = async (url, opts = {}) => {
    if (opts.body) sent.push([url, JSON.parse(opts.body)]);
    if (/^\/api\/docs\/f1(\?|$)/.test(url)) return json({ id: 'f1', title: 'Pressupost', kind: 'sheet', version: 1, blocks: [] });
    return api(url, opts);
  };
  try {
    edit = null;
    window.gregalDocs.open('f1', 'mine', true);
    await settle();
    assert.equal(edit, null, 'the text view is not used for a sheet');
    assert.equal(write, null, 'no text block is opened in a new sheet');
    await sheetEdit.put('s1', { cells: { B2: '=B1*2' } }, 2);
    await sheetEdit.post('s1', { name: 'Full2' });
    assert.deepEqual(sent, [
      ['/api/docs/f1/blocks/s1?scope=mine', { cells: { B2: '=B1*2' }, base_version: 2 }],
      ['/api/docs/f1/blocks?scope=mine', { after: 's1', name: 'Full2' }],
    ]);
    assert.equal(byId('docsDownload').href, '/api/docs/f1/export?format=xlsx&name=Pressupost.xlsx&scope=mine');
    assert.equal(byId('docsDownload').textContent, 'Download as Excel');
  } finally {
    window.gregal.api = api;
    byId('docsClose').click();
  }
});

// L'editor (doc-view.js) hi posa la barra i la pàgina .doc-editor.
test('a text document is drawn on a page with the export theme, under the editor toolbar', async () => {
  await load;
  box = null;
  window.gregalDocs.open('w1', 'mine');
  await settle();
  const body = byId('docsBody');
  assert.ok(body.classList.contains('doc-paper'), 'the body is the page background');
  assert.equal(box, body, 'the text editor draws its toolbar and page in the themed body');
  assert.deepEqual(body.props, THEME, 'the page reads the theme the Word export uses');

  // Una presentació no és una pàgina.
  const api = window.gregal.api;
  window.gregalDeckView = { view: b => { box = b; return { open() {}, apply() {} }; } };
  window.gregal.api = async (url, opts) => (/^\/api\/docs\/k1(\?|$)/.test(url) ? json({ id: 'k1', title: 'L', kind: 'deck', version: 1, blocks: [] }) : api(url, opts));
  try {
    window.gregalDocs.open('k1', 'mine');
    await settle();
    assert.equal(box, body);
    assert.ok(!body.classList.contains('doc-paper'));
  } finally {
    window.gregal.api = api;
    byId('docsClose').click();
  }
});

test('the page CSS takes every font, size and colour from the theme variables', async () => {
  await load;
  const css = document.head.children.map(s => s.textContent).join('');
  const rule = sel => {
    const m = [...css.matchAll(/([^{}]+)\{([^{}]*)\}/g)].find(r => r[1].split(',').map(s => s.trim()).includes(sel));
    assert.ok(m, 'no rule for ' + sel);
    return m[2];
  };
  assert.match(rule('#docsBody.doc-paper'), /container-type:inline-size/);
  assert.match(rule('#docsBody.doc-paper'), /background:color-mix\(in srgb,var\(--ink\) 10%,var\(--panel\)\)/, 'a neutral background around the page, in both themes');
  const page = rule('#docsBody .doc-editor');
  // La pàgina és clara en tots dos temes, com a Word, i fa l'amplada A4
  // del docx o s'encongeix fins a l'amplada del panell.
  assert.match(page, /background:#fff/);
  assert.match(page, /--u:min\(1pt,100cqw \/ var\(--doc-page-w\)\)/);
  assert.match(page, /width:calc\(var\(--doc-page-w\) \* var\(--u\)\)/);
  assert.match(page, /margin:0 auto/);
  assert.match(page, /padding:calc\(var\(--doc-margin-top\) \* var\(--u\)\) calc\(var\(--doc-margin-side\) \* var\(--u\)\)/);
  assert.match(page, /font-family:var\(--doc-body-font\)/);
  assert.match(page, /font-size:calc\(var\(--doc-body-size\) \* var\(--u\)\)/);
  assert.match(page, /color:var\(--doc-text\)/);
  for (const n of [1, 2, 3]) {
    const h = rule('#docsBody .doc-editor h' + n);
    assert.match(h, new RegExp('font-size:calc\\(var\\(--doc-h' + n + '-size\\) \\* var\\(--u\\)\\)'));
    assert.match(h, new RegExp('color:var\\(--doc-h' + n + '-color\\)'));
  }
  assert.match(rule('#docsBody .doc-editor h1'), /font-family:var\(--doc-heading-font\)/);
  assert.match(rule('#docsBody .doc-editor p'), /margin:0 0 calc\(var\(--doc-space-after\) \* var\(--u\)\)/);
  assert.match(rule('#docsBody .doc-editor blockquote'), /border-left:.*var\(--doc-accent\)/);
  assert.match(rule('#docsBody .doc-editor blockquote'), /color:var\(--doc-muted\)/);
  assert.match(rule('#docsBody .doc-editor pre'), /background:var\(--doc-light\)/);
  assert.match(rule('#docsBody .doc-editor pre'), /font-family:var\(--doc-code-font\)/);
  assert.match(rule('#docsBody .doc-editor code'), /font-family:var\(--doc-code-font\)/);
  assert.match(rule('#docsBody .doc-editor td'), /border:.*var\(--doc-table-border\)/);
  assert.match(rule('#docsBody .doc-editor th'), /background:var\(--doc-primary\)/);
  assert.match(rule('#docsBody .doc-editor a'), /color:var\(--doc-accent\)/);
  assert.match(rule('#docsBody .doc-editor img'), /max-width:100%/);
  // Cap color escrit a mà a la pàgina (llevat del blanc del paper): tots surten del tema.
  for (const [, sel, decl] of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    if (sel.includes('.doc-page')) assert.doesNotMatch(decl.replace(/#fff\b/gi, ''), /#[0-9a-f]{3,8}\b/i, sel);
  }
  const bar = rule('#docsBody .doc-toolbar');
  assert.match(bar, /position:sticky/);
  assert.match(bar, /top:0/);
});

// attach.js, que baixa els enllaços amb la clau, no és a la finestra pròpia
// (doc-page.js): el panell baixa el document ell mateix, en tots dos llocs.
test('the download fetches the export with the key and saves it under the document name', async () => {
  await load;
  const api = window.gregal.api;
  const fetched = [];
  window.gregal.api = async (url, opts) => {
    if (!url.includes('/export')) return api(url, opts);
    fetched.push(url);
    return { ok: true, blob: async () => new Blob(['docx']) };
  };
  const later = globalThis.setTimeout;
  try {
    window.gregalDocs.open('w1', 'mine');
    await settle();
    assert.doesNotMatch(byId('docsPanel').innerHTML, /data-gregal-doc/, 'attach.js must not download it a second time');
    let prevented = false;
    globalThis.setTimeout = () => {};
    await byId('docsDownload').onclick({ preventDefault() { prevented = true; } });
    globalThis.setTimeout = later;
    assert.ok(prevented, 'the link is not followed without the key');
    assert.deepEqual(fetched, ['/api/docs/w1/export?format=docx&name=Carta.docx&scope=mine']);
    const save = made.at(-1);
    assert.equal(save.download, 'Carta.docx');
    assert.match(save.href, /^blob:/);
  } finally {
    globalThis.setTimeout = later;
    window.gregal.api = api;
    byId('docsClose').click();
  }
});

test('switching session closes a project document, keeps a personal one and reloads the home', async () => {
  await load;
  window.gregalDocs.changed({ id: 'p1', title: 'Pla' });
  await settle();
  listeners['gregal:session']({ detail: { id: 'altra' } });
  assert.equal(byId('docsPanel').hidden, true, 'the other project’s document is closed');

  window.gregalDocs.changed({ id: 'w1', title: 'Carta', scope: 'mine' });
  await settle();
  calls.length = 0;
  listeners['gregal:session']({ detail: { id: 'altra' } });
  await settle();
  assert.equal(byId('docsPanel').hidden, false, 'a personal document belongs to no project');
  assert.deepEqual(calls, [], 'and it is not reopened');

  const lists = [];
  window.gregalDocHome = { home: (box, l) => lists.push(l) };
  await byId('docsHomeBtn').click();
  listeners['gregal:session']({ detail: { id: 'altra' } });
  await settle();
  assert.equal(lists.length, 2, 'the home is drawn again for the new session');
  byId('docsClose').click();
});

test('the home still lists this project when my documents fail to load', async () => {
  await load;
  const api = window.gregal.api, errors = [];
  const sys = window.gregal.sys;
  let lists = null;
  window.gregalDocHome = { home: (box, l) => { lists = l; } };
  window.gregalProduct = 'codi';
  window.gregal.state = { cwd: '/w/botiga', project: 'botiga' };
  window.gregal.sys = m => errors.push(m);
  window.gregal.api = async (url, opts) => (url === '/api/docs?scope=mine'
    ? { ok: false, status: 500, statusText: 'Internal Server Error', json: async () => ({ error: 'open /x: access denied' }) }
    : api(url, opts));
  try {
    await byId('docsHomeBtn').click();
    await settle();
    assert.equal(lists.mine, null, 'my documents could not be read');
    assert.deepEqual(lists.project, { name: 'botiga', items: [{ id: 'w1', title: 'Carta' }] });
    assert.deepEqual(errors, [], 'the home says it in its own list, not as a raw error');
  } finally {
    Object.assign(window.gregal, { api, sys, state: undefined });
    delete window.gregalProduct;
    byId('docsClose').click();
  }
});

// Un sol panell de feina al costat del xat (089): el document i l'ordinador dels vents
// (winds-workspace.js) s'avisen amb gregal:workpane i l'altre es tanca.
test('opening a document announces the docs pane, and the computer pane opening closes it', async () => {
  await load;
  dispatched.length = 0;
  window.gregalDocs.open('w1', 'mine');
  assert.ok(dispatched.some(e => e.type === 'gregal:workpane' && e.detail === 'docs'), 'the docs pane says it opened');
  assert.equal(byId('docsPanel').hidden, false);
  listeners['gregal:workpane']({ type: 'gregal:workpane', detail: 'docs' });
  assert.equal(byId('docsPanel').hidden, false, 'its own announcement does not close it');
  listeners['gregal:workpane']({ type: 'gregal:workpane', detail: 'computer' });
  assert.equal(byId('docsPanel').hidden, true, 'the computer pane takes its place');
});

// 126: el botó del lateral d'un xat de vent ha de saber que la persona l'ha
// tancat, i no confondre-ho amb l'ordinador que hi pren el lloc.
test('closing the pane with its × says the person closed it; the computer taking its place does not', async () => {
  await load;
  window.gregalDocs.open('w1', 'mine');
  dispatched.length = 0;
  listeners['gregal:workpane']({ type: 'gregal:workpane', detail: 'computer' });
  assert.equal(byId('docsPanel').hidden, true);
  assert.ok(!dispatched.some(e => e.type === 'gregal:docsclosed'), 'not the person');
  window.gregalDocs.open('w1', 'mine');
  dispatched.length = 0;
  byId('docsClose').click();
  assert.equal(byId('docsPanel').hidden, true);
  assert.ok(dispatched.some(e => e.type === 'gregal:docsclosed'), 'the person closed it');
});
