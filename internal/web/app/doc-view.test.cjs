const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
global.window = global;
require('./md.js');
const { view, onDocChanged, keyAction } = require('./doc-view.js');

// Prou DOM per a la vista: fills en ordre, insertBefore, remove i innerHTML
// amb un comptador d'escriptures per saber què s'ha tornat a pintar.
class Node {
  constructor(doc) { this.ownerDocument = doc; this.children = []; this.parentNode = null; this.dataset = {}; this.writes = 0; this._html = ''; }
  set innerHTML(v) { this._html = v; this.children = []; this.writes++; }
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
  append(el) { this.insertBefore(el, null); }
  remove() { const s = this.parentNode.children; s.splice(s.indexOf(this), 1); }
  focus() {}
}
const fakeDocument = { createElement: tag => Object.assign(new Node(fakeDocument), { tag }) };

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
  assert.match(src, /tx\("L'agent ha canviat aquest bloc", 'The agent changed this block'\)/);
});

// fakeEdit fa de servidor de les rutes de blocs: apunta les crides i torna
// la resposta que toca, per defecte un 200.
function fakeEdit() {
  const e = {
    calls: [], next: null, errors: [],
    reply(r) { e.next = r; },
    async put(id, md, base) { e.calls.push(['put', id, md, base]); return e.take({ version: 9, block: { id, md, version: 9 } }); },
    async post(after, md) { e.calls.push(['post', after, md]); return e.take({ version: 9, blocks: [{ id: 'n', md, version: 9 }] }); },
    async del(id, base) { e.calls.push(['del', id, base]); return e.take({ version: 9 }); },
    take(body) { const r = e.next || { status: 200, body }; e.next = null; return r; },
    notice: () => 'The agent changed this block',
    error: m => e.errors.push(m),
  };
  return e;
}
const key = (ta, k, extra = {}) => ta.onkeydown({ key: k, preventDefault() {}, ...extra });
const tick = () => new Promise(r => setImmediate(r));

test('editing a block: open shows its markdown, Esc cancels, saving replaces, emptying deletes', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, DOC, window.gregalMD.render, ed);
  const b = box.children[1];
  v.open('b');
  const ta = v.editing.ta;
  assert.equal(ta.tag, 'textarea');
  assert.equal(ta.value, '- primer\n- segon');
  assert.deepEqual(b.children, [ta]);
  ta.value = 'Canviat';
  key(ta, 'Escape');
  assert.equal(v.editing, null);
  assert.match(b.innerHTML, /<li>primer<\/li>/, 'cancel shows the old text');
  assert.deepEqual(ed.calls, []);

  v.open('b');
  v.editing.ta.value = 'Llista nova';
  await v.save();
  assert.deepEqual(ed.calls, [['put', 'b', 'Llista nova', 2]]);
  assert.equal(b.text, 'Llista nova');
  assert.equal(v.editing, null);

  v.open('b');
  assert.equal(v.editing.base, 9, 'the next save starts from the saved version');
  key(v.editing.ta, 'Escape');
  v.open('c');
  v.editing.ta.value = '  ';
  await v.save();
  assert.deepEqual(ed.calls.at(-1), ['del', 'c', 3]);
  assert.deepEqual(box.children.map(e => e.dataset.id), ['a', 'b']);
});

test('Enter at the end of a block opens a new block below, saved with the id the server gives', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, DOC, window.gregalMD.render, ed);
  v.open('a');
  const ta = v.editing.ta;
  ta.selectionStart = ta.selectionEnd = 1;
  key(ta, 'Enter');
  assert.equal(v.editing.ta, ta, 'Enter in the middle of the text is a new line');
  ta.selectionStart = ta.selectionEnd = ta.value.length;
  key(ta, 'Enter');
  await tick();
  assert.equal(v.editing.id, null, 'a new block has no id until the server gives one');
  assert.deepEqual(box.children.map(e => e.dataset.id), ['a', undefined, 'b', 'c']);
  v.editing.ta.value = 'Nou';
  await v.save();
  assert.deepEqual(ed.calls, [['post', 'a', 'Nou']]);
  assert.deepEqual(box.children.map(e => e.dataset.id), ['a', 'n', 'b', 'c']);
  v.apply({ op: 'insert', block: { id: 'n', md: 'Nou' }, after: 'a', version: 9, author: 'user' });
  assert.deepEqual(box.children.map(e => e.dataset.id), ['a', 'n', 'b', 'c'], 'the stream event of the same insert is not doubled');

  v.open(null, 'c');
  await v.save();
  assert.equal(ed.calls.length, 1, 'an empty new block is dropped without a call');
  assert.deepEqual(box.children.map(e => e.dataset.id), ['a', 'n', 'b', 'c']);
});

test('leaving a block or Ctrl+Enter saves it; leaving an untouched block sends nothing', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, DOC, window.gregalMD.render, ed);
  v.open('b');
  v.editing.ta.value = 'En sortir';
  v.editing.ta.onblur();
  await tick();
  assert.deepEqual(ed.calls, [['put', 'b', 'En sortir', 2]]);
  assert.equal(box.children[1].text, 'En sortir');

  v.open('c');
  v.editing.ta.value = 'Amb Ctrl+Enter';
  key(v.editing.ta, 'Enter', { ctrlKey: true });
  await tick();
  assert.deepEqual(ed.calls.at(-1), ['put', 'c', 'Amb Ctrl+Enter', 3]);

  v.open('a');
  const ta = v.editing.ta;
  v.apply({ op: 'replace', block: { id: 'a', md: 'Ho canvia l\'agent', version: 10 }, version: 10, author: 'agent' });
  ta.onblur();
  await tick();
  assert.equal(ed.calls.length, 2, 'an untouched block is not saved over the agent\'s change');
});

test('a save that fails on the network is reported and keeps the text', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  ed.put = async () => { throw new Error('offline'); };
  const v = view(box, DOC, window.gregalMD.render, ed);
  v.open('b');
  v.editing.ta.value = 'No arriba';
  await v.save();
  assert.deepEqual(ed.errors, ['offline']);
  assert.equal(v.editing.ta.value, 'No arriba');
});

test('a 409 shows the current text and a notice, and keeps the person\'s text', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, DOC, window.gregalMD.render, ed);
  v.open('b');
  const ta = v.editing.ta;
  ta.value = 'El meu text';
  ed.reply({ status: 409, body: { error: 'changed', block: { id: 'b', md: 'Text de l\'agent', version: 7 } } });
  await v.save();
  const [notice, kept] = box.children[1].children;
  assert.match(notice.innerHTML, /The agent changed this block/);
  assert.match(notice.innerHTML, /Text de l.agent/);
  assert.equal(kept, ta);
  assert.equal(ta.value, 'El meu text');
  ta.onblur();
  assert.equal(ed.calls.length, 1, 'leaving after a conflict does not overwrite');
  key(ta, 'Enter', { ctrlKey: true });
  await tick();
  assert.deepEqual(ed.calls.at(-1), ['put', 'b', 'El meu text', 7], 'Ctrl+Enter saves on top of the version now shown');
  assert.equal(box.children[1].text, 'El meu text');
});

test('while a block is edited its events do not touch the text area; other blocks still update', () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, DOC, window.gregalMD.render, ed);
  v.open('b');
  const ta = v.editing.ta;
  ta.value = 'A mig escriure';
  v.apply({ op: 'replace', block: { id: 'b', md: 'Agent b', version: 4 }, version: 4, author: 'agent' });
  v.apply({ op: 'replace', block: { id: 'a', md: '# Pla nou', version: 5 }, version: 5, author: 'agent' });
  assert.deepEqual(box.children[1].children, [ta]);
  assert.equal(ta.value, 'A mig escriure');
  assert.equal(v.editing.base, 2, 'saving is still checked against the version it opened');
  assert.match(box.children[0].innerHTML, /Pla nou/);
  v.cancel();
  assert.equal(box.children[1].text, 'Agent b', 'cancel shows what the agent wrote meanwhile');
});

test('keys in the block editor', () => {
  const ta = { value: 'abc', selectionStart: 3, selectionEnd: 3 };
  assert.equal(keyAction({ key: 'Escape' }, ta), 'cancel');
  assert.equal(keyAction({ key: 'Enter', ctrlKey: true }, ta), 'save');
  assert.equal(keyAction({ key: 'Enter', metaKey: true }, ta), 'save');
  assert.equal(keyAction({ key: 'Enter' }, ta), 'insert');
  assert.equal(keyAction({ key: 'Enter', shiftKey: true }, ta), null);
  assert.equal(keyAction({ key: 'Enter' }, { ...ta, selectionStart: 1, selectionEnd: 1 }), null);
  assert.equal(keyAction({ key: 'a' }, ta), null);
});
