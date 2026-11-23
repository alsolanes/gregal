const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
global.window = global;
require('./md.js');
const { view, onDocChanged } = require('./doc-view.js');

// Prou DOM per a la vista: fills en ordre, insertBefore, remove i innerHTML
// amb un comptador d'escriptures per saber què s'ha tornat a pintar.
class Node {
  constructor(doc) { this.ownerDocument = doc; this.children = []; this.parentNode = null; this.dataset = {}; this.writes = 0; this._html = ''; }
  set innerHTML(v) { this._html = v; this.writes++; }
  get innerHTML() { return this._html; }
  get text() { return this._html.replace(/<[^>]+>/g, '').trim(); }
  get firstChild() { return this.children[0] || null; }
  get nextSibling() { const s = this.parentNode.children; return s[s.indexOf(this) + 1] || null; }
  insertBefore(el, ref) {
    el.parentNode = this;
    const i = ref ? this.children.indexOf(ref) : this.children.length;
    this.children.splice(i, 0, el);
  }
  replaceChildren(...els) { els.forEach(e => { e.parentNode = this; }); this.children = els; }
  remove() { const s = this.parentNode.children; s.splice(s.indexOf(this), 1); }
}
const fakeDocument = { createElement: () => new Node(fakeDocument) };

const DOC = {
  id: 'd1', version: 3,
  blocks: [
    { id: 'a', md: '# Pla', version: 1 },
    { id: 'b', md: '- primer\n- segon', version: 2 },
    { id: 'c', md: '| Fase | Data |\n|---|---|\n| Beta | Març |', version: 3 },
  ],
};

test('the document renders one block per block, through md.js', () => {
  const box = new Node(fakeDocument);
  view(box, DOC, window.gregalMD.render);
  assert.deepEqual(box.children.map(e => e.dataset.id), ['a', 'b', 'c']);
  assert.match(box.children[0].innerHTML, /<h1[^>]*>Pla<\/h1>/);
  assert.match(box.children[1].innerHTML, /<li>primer<\/li>/);
  assert.match(box.children[2].innerHTML, /<table/);
});

test('block events apply in place: insert in its position, replace only that block, delete removes it', () => {
  const box = new Node(fakeDocument);
  const v = view(box, DOC, window.gregalMD.render);
  const [a, b, c] = box.children;
  v.apply({ op: 'insert', block: { id: 'x', md: 'Nou paràgraf' }, after: 'a', version: 4, author: 'agent' });
  v.apply({ op: 'insert', block: { id: 'y', md: 'Al principi' }, version: 4, author: 'agent' });
  v.apply({ op: 'replace', block: { id: 'b', md: 'Llista canviada' }, version: 5, author: 'agent' });
  v.apply({ op: 'delete', block: { id: 'c' }, version: 6, author: '' });
  assert.deepEqual(box.children.map(e => e.dataset.id), ['y', 'a', 'x', 'b']);
  assert.deepEqual(box.children.map(e => e.text), ['Al principi', 'Pla', 'Nou paràgraf', 'Llista canviada']);
  assert.equal(a.writes, 1, 'the heading was not painted again');
  assert.equal(b.writes, 2, 'the replaced block was painted once more');
  assert.equal(c.parentNode.children.includes(c), false);
  assert.equal(v.version, 6);
});

test('events the loaded document already has are ignored', () => {
  const box = new Node(fakeDocument);
  const v = view(box, DOC, window.gregalMD.render);
  assert.equal(v.apply({ op: 'delete', block: { id: 'a' }, version: 3, author: '' }), false);
  assert.deepEqual(box.children.map(e => e.dataset.id), ['a', 'b', 'c']);
});

test('doc_changed opens a closed document and only brings forward the one already open', () => {
  assert.equal(onDocChanged({ id: '', visible: false }, { id: 'd1' }), 'open');
  assert.equal(onDocChanged({ id: 'd1', visible: false }, { id: 'd1' }), 'open');
  assert.equal(onDocChanged({ id: 'd2', visible: true }, { id: 'd1' }), 'open');
  assert.equal(onDocChanged({ id: 'd1', visible: true }, { id: 'd1' }), 'front');
});

test('the panel has its strings in Catalan and English and downloads through the export route', () => {
  const src = fs.readFileSync(path.join(__dirname, 'docs-panel.js'), 'utf8');
  assert.match(src, /tx\('Descarrega en Word', 'Download as Word'\)/);
  assert.match(src, /\/export\?format=docx/);
  assert.match(src, /'\/api\/docs'/);
});
