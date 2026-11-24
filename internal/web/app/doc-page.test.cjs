// La finestra pròpia d'un document (doc-page.js) amb un DOM mínim: llegeix
// la sessió i l'scope de la URL, la clau del pont d'Android, i el panell
// (docs-panel.js) hi fa totes les crides amb tots tres, també la descàrrega.
const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

class El {
  constructor(id = '') {
    this.id = id; this.hidden = false; this.attributes = {}; this.children = []; this.textContent = ''; this.href = ''; this.className = '';
    this.style = { setProperty() {} };
    this.classList = { toggle() {}, contains: () => false };
  }
  setAttribute(k, v) { this.attributes[k] = String(v); }
  append(...n) { this.children.push(...n); }
  prepend(...n) { this.children.unshift(...n); }
  replaceChildren(...n) { this.children = n; }
  addEventListener() {}
  click() { return this.onclick?.({ target: this }); }
}

const els = new Map();
const byId = id => els.get(id) || els.set(id, new El(id)).get(id);
const calls = [];
const json = data => ({ ok: true, status: 200, json: async () => data });

globalThis.window = globalThis;
globalThis.document = { getElementById: byId, createElement: () => new El(), head: new El(), body: new El(), addEventListener() {}, title: '' };
globalThis.Option = function Option(text, value) { return { text, value }; };
globalThis.gregalMD = { render: s => s };
globalThis.gregalDocView = { onDocChanged: () => 'open', view: () => ({ focus() {}, apply() {} }) };
globalThis.fetch = async (url, opts = {}) => {
  calls.push([url, opts.headers]);
  if (url.includes('/stream')) return { ok: true, body: { getReader: () => ({ read: () => new Promise(() => {}) }) } };
  if (url.startsWith('/api/docs/theme')) return json({ vars: {} });
  if (url.includes('/export')) return { ok: true, blob: async () => new Blob(['docx']) };
  if (url.startsWith('/api/docs/w1')) return json({ id: 'w1', title: 'Carta', kind: 'doc', version: 1, blocks: [] });
  return json({ items: [] });
};

const settle = () => new Promise(r => setTimeout(r, 5));

test('the document window opens a personal document with the session, scope and Android key', async () => {
  globalThis.location = { search: '?session=tab-1&scope=mine', pathname: '/app/doc/w1', href: 'http://127.0.0.1:8097/app/doc/w1?session=tab-1&scope=mine' };
  globalThis.GregalAndroid = { token: () => 'clau-android' };
  globalThis.localStorage = { getItem: () => 'clau-del-navegador' };
  await import(pathToFileURL(path.join(__dirname, 'doc-page.js')).href);
  await settle();
  const urls = calls.map(c => c[0]);
  for (const want of ['/api/docs/w1/stream?scope=mine', '/api/docs/w1?scope=mine', '/api/docs?scope=mine']) {
    assert.ok(urls.includes(want), want + ' in ' + urls.join(' '));
  }
  for (const [url, headers] of calls) {
    assert.equal(headers['X-Gregal-Session'], 'tab-1', url);
    assert.equal(headers.Authorization, 'Bearer clau-android', url + ': the key comes from the bridge, never the URL');
  }
  assert.equal(document.title, 'Carta · Gregal');

  // attach.js no és a la finestra: la descàrrega la fa el panell, amb la clau.
  calls.length = 0;
  const later = globalThis.setTimeout;
  globalThis.setTimeout = () => {};
  try {
    await byId('docsDownload').onclick({ preventDefault() {} });
  } finally {
    globalThis.setTimeout = later;
  }
  assert.deepEqual(calls, [['/api/docs/w1/export?format=docx&name=Carta.docx&scope=mine', { 'X-Gregal-Session': 'tab-1', Authorization: 'Bearer clau-android' }]]);
});

test('an unknown scope opens the project document, with the browser key', async () => {
  calls.length = 0;
  globalThis.location = { search: '?scope=../x', pathname: '/app/doc/w1', href: 'http://127.0.0.1:8097/app/doc/w1?scope=../x' };
  delete globalThis.GregalAndroid;
  await import(pathToFileURL(path.join(__dirname, 'doc-page.js')).href + '?other');
  await settle();
  const urls = calls.map(c => c[0]);
  assert.ok(urls.includes('/api/docs/w1/stream'), urls.join(' '));
  for (const [url, headers] of calls) {
    assert.doesNotMatch(url, /scope=/);
    assert.deepEqual(headers, { 'X-Gregal-Session': 'default', Authorization: 'Bearer clau-del-navegador' }, url);
  }
});
