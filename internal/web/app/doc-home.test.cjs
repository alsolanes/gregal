const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { home, newDoc } = require('./doc-home.js');

// Prou DOM per a l'inici: fills en ordre, text i atributs.
class Node {
  constructor(doc, tag) { this.ownerDocument = doc; this.tag = tag; this.children = []; this.dataset = {}; this.attrs = {}; this.textContent = ''; this.className = ''; }
  append(...els) { this.children.push(...els); }
  replaceChildren(...els) { this.children = els; }
  setAttribute(k, v) { this.attrs[k] = v; }
  get text() { return this.textContent + this.children.map(c => c.text).join(' '); }
  all(cls) { return [...(this.className.split(' ').includes(cls) ? [this] : []), ...this.children.flatMap(c => c.all(cls))]; }
  find(tag) { return this.tag === tag ? this : this.children.map(c => c.find(tag)).find(Boolean); }
}
const fakeDocument = { createElement: tag => new Node(fakeDocument, tag) };
const en = { tx: (ca, e) => e, lang: 'en' };
const ca = { tx: c => c, lang: 'ca' };

const MINE = [
  { id: 'old', title: 'Carta', kind: 'doc', updated: '2026-11-01T09:00:00Z' },
  { id: 'new', title: 'Llançament', kind: 'deck', updated: '2026-11-20T09:00:00.5Z' },
];
const PROJECT = { name: 'botiga', items: [{ id: 'p1', title: 'Pla', kind: 'doc', updated: '2026-11-10T09:00:00Z' }] };

test('the Docs home lists my documents and the open project\'s, newest first, with kind and last change', () => {
  const box = new Node(fakeDocument, 'div'), opened = [];
  home(box, { mine: MINE, project: PROJECT }, { ...ca, open: (id, scope) => opened.push([id, scope]), create() {} });
  const sections = box.all('docs-home-section');
  assert.equal(sections.length, 2);
  assert.equal(sections[0].find('h3').textContent, 'Els meus documents');
  assert.equal(sections[1].find('h3').textContent, 'Aquest projecte · botiga');
  const items = sections[0].all('docs-home-item');
  assert.deepEqual(items.map(b => b.dataset.id), ['new', 'old']);
  assert.match(items[0].text, /Llançament.*Presentació · .*2026/);
  assert.match(items[1].text, /Carta.*Document · /);
  items[0].onclick();
  sections[1].all('docs-home-item')[0].onclick();
  assert.deepEqual(opened, [['new', 'mine'], ['p1', '']]);
});

test('without an open project the home has only my documents, and says when there are none', () => {
  const box = new Node(fakeDocument, 'div');
  home(box, { mine: [], project: null }, { ...en, open() {}, create() {} });
  const sections = box.all('docs-home-section');
  assert.equal(sections.length, 1);
  assert.equal(sections[0].find('h3').textContent, 'My documents');
  assert.match(sections[0].text, /No documents yet\./);
});

test('«New document» creates with the typed title, or an untitled one', () => {
  const box = new Node(fakeDocument, 'div'), created = [];
  home(box, { mine: [], project: null }, { ...en, open() {}, create: t => created.push(t) });
  const form = box.all('docs-home-new')[0];
  assert.equal(form.find('button').textContent, 'New document');
  form.find('input').value = '  Viatge  ';
  form.onsubmit({ preventDefault() {} });
  form.find('input').value = '';
  form.onsubmit({ preventDefault() {} });
  assert.deepEqual(created, ['Viatge', 'Untitled document']);
});

test('a new document is created in my documents and opens to write in', async () => {
  const calls = [], opened = [], errors = [];
  const api = async (url, opts) => {
    calls.push([url, opts.method, JSON.parse(opts.body)]);
    return { ok: true, status: 200, json: async () => ({ id: 'd9', title: 'Viatge', kind: 'doc', version: 1, blocks: [] }) };
  };
  await newDoc('Viatge', { api, open: (...a) => opened.push(a), error: m => errors.push(m) });
  assert.deepEqual(calls, [['/api/docs?scope=mine', 'POST', { title: 'Viatge', kind: 'doc' }]]);
  assert.deepEqual(opened, [['d9', 'mine', true]]);

  const refused = async () => ({ ok: false, status: 400, json: async () => ({ error: 'title required' }) });
  await newDoc('', { api: refused, open: (...a) => opened.push(a), error: m => errors.push(m) });
  assert.deepEqual(errors, ['title required']);
  assert.equal(opened.length, 1);

  const offline = async () => { throw new Error('offline'); };
  await newDoc('Viatge', { api: offline, open: (...a) => opened.push(a), error: m => errors.push(m) });
  assert.deepEqual(errors, ['title required', 'offline']);
});

test('«New presentation» creates a deck in my documents', async () => {
  const box = new Node(fakeDocument, 'div'), created = [];
  home(box, { mine: [], project: null }, { ...en, open() {}, create: (...a) => created.push(a) });
  const form = box.all('docs-home-new')[0];
  const deck = form.children.find(c => c.textContent === 'New presentation');
  form.find('input').value = 'Llançament';
  deck.onclick();
  form.find('input').value = '';
  deck.onclick();
  assert.deepEqual(created, [['Llançament', 'deck'], ['Untitled presentation', 'deck']]);

  const calls = [], opened = [];
  const api = async (url, opts) => {
    calls.push([url, JSON.parse(opts.body)]);
    return { ok: true, status: 200, json: async () => ({ id: 'k9', kind: 'deck', version: 1, blocks: [] }) };
  };
  await newDoc('Llançament', { api, open: (...a) => opened.push(a), error() {}, kind: 'deck' });
  assert.deepEqual(calls, [['/api/docs?scope=mine', { title: 'Llançament', kind: 'deck' }]]);
  assert.deepEqual(opened, [['k9', 'mine', true]]);
});

test('the slide view is loaded by the app and by the document window', () => {
  const html = fs.readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');
  assert.match(html, /<script defer src="\/app\/deck-view.js"><\/script>/);
  const page = fs.readFileSync(path.join(__dirname, '..', 'docs_api.go'), 'utf8');
  assert.match(page, /<script src="\/app\/deck-view.js"><\/script>/);
});

test('«New spreadsheet» creates a sheet in my documents, listed as a spreadsheet', () => {
  const box = new Node(fakeDocument, 'div'), created = [];
  home(box, { mine: [{ id: 'f1', title: 'Pressupost', kind: 'sheet', updated: '2026-11-20T09:00:00Z' }], project: null }, { ...ca, open() {}, create: (...a) => created.push(a) });
  assert.match(box.all('docs-home-item')[0].text, /Full de càlcul · /);
  const form = box.all('docs-home-new')[0];
  const sheet = form.children.find(c => c.textContent === 'Full de càlcul nou');
  form.find('input').value = 'Pressupost';
  sheet.onclick();
  form.find('input').value = '';
  sheet.onclick();
  assert.deepEqual(created, [['Pressupost', 'sheet'], ['Full de càlcul sense títol', 'sheet']]);
});

test('the grid view is loaded by the app and by the document window', () => {
  const html = fs.readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');
  assert.match(html, /<script defer src="\/app\/sheet-view.js"><\/script>/);
  const page = fs.readFileSync(path.join(__dirname, '..', 'docs_api.go'), 'utf8');
  assert.match(page, /<script src="\/app\/sheet-view.js"><\/script>/);
});

test('the app has a Gregal Docs button in the sidebar rail, shared by Codi and Vents', () => {
  const html = fs.readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');
  const rail = html.slice(html.indexOf('<div class="rail-main">'), html.indexOf('<details id="sideMore"'));
  assert.match(rail, /id="docsHomeBtn"[^>]*>.*Gregal Docs/);
  assert.match(html, /<script defer src="\/app\/doc-home.js"><\/script>/);
  const panel = fs.readFileSync(path.join(__dirname, 'docs-panel.js'), 'utf8');
  assert.match(panel, /\$\('docsHomeBtn'\)/);
});

test('a list that could not be read says so, and the other list is still shown', () => {
  const box = new Node(fakeDocument, 'div');
  home(box, { mine: null, project: PROJECT }, { ...en, open() {}, create() {} });
  const sections = box.all('docs-home-section');
  assert.equal(sections.length, 2);
  assert.match(sections[0].text, /Couldn’t load these documents\./);
  assert.doesNotMatch(sections[0].text, /No documents yet/);
  assert.equal(sections[1].all('docs-home-item').length, 1);

  const cat = new Node(fakeDocument, 'div');
  home(cat, { mine: [], project: { name: 'botiga', items: null } }, { ...ca, open() {}, create() {} });
  assert.match(cat.all('docs-home-section')[1].text, /No s’han pogut carregar aquests documents\./);
});

test('a document of an unknown kind is listed as a text document, not by its raw kind', () => {
  const box = new Node(fakeDocument, 'div');
  home(box, { mine: [{ id: 'x1', title: 'Nota', kind: 'whiteboard', updated: '2026-11-20T09:00:00Z' }], project: null }, { ...en, open() {}, create() {} });
  const item = box.all('docs-home-item')[0];
  assert.match(item.text, /Nota.*Document · /);
  assert.doesNotMatch(item.text, /whiteboard/);
  assert.equal(item.children[0].className, 'docs-kind docs-kind-doc');
});
