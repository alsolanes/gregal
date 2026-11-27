const test = require('node:test');
const assert = require('node:assert/strict');
global.window = global;
require('./md.js');
const { view } = require('./deck-view.js');

// Prou DOM per a la vista: fills en ordre, insertBefore, remove i innerHTML
// amb un comptador d'escriptures per saber què s'ha tornat a pintar.
class Node {
  constructor(doc, tag) { this.ownerDocument = doc; this.tag = tag; this.children = []; this.parentNode = null; this.dataset = {}; this.writes = 0; this._html = ''; this.value = ''; this.className = ''; this.textContent = ''; }
  set innerHTML(v) { this._html = v; this.children = []; this.writes++; }
  get innerHTML() { return this._html; }
  get text() { return (this._html.replace(/<[^>]+>/g, ' ') + ' ' + this.textContent + ' ' + this.children.map(c => c.text).join(' ')).replace(/\s+/g, ' ').trim(); }
  get firstChild() { return this.children[0] || null; }
  get nextSibling() { const s = this.parentNode.children; return s[s.indexOf(this) + 1] || null; }
  insertBefore(el, ref) {
    el.parentNode = this;
    const i = ref ? this.children.indexOf(ref) : this.children.length;
    this.children.splice(i, 0, el);
  }
  replaceChildren(...els) { els.forEach(e => { e.parentNode = this; }); this.children = els; }
  append(...els) { els.forEach(e => this.insertBefore(e, null)); }
  remove() { const s = this.parentNode.children; s.splice(s.indexOf(this), 1); }
  setAttribute(k, v) { this[k] = v; }
  focus() { this.ownerDocument.activeElement = this; }
  closest(sel) { return sel.split(',').map(s => s.trim()).includes(this.tag) ? this : this.parentNode?.closest?.(sel) || null; }
  all(tag) { return [...(this.tag === tag ? [this] : []), ...this.children.flatMap(c => c.all(tag))]; }
}
const fakeDocument = { createElement: tag => new Node(fakeDocument, tag) };

const DECK = {
  id: 'p1', kind: 'deck', version: 3,
  blocks: [
    { id: 'a', md: '', slide: { layout: 'title', title: 'Llançament 2027', subtitle: 'Pla de llançament' }, version: 1 },
    { id: 'b', md: '', slide: { layout: 'content', title: 'Fases', bullets: ['Beta', 'Obert'], chart: { type: 'bar', categories: ['T1'], series: [{ name: 'Vendes', values: [3] }] } }, version: 2 },
    { id: 'c', md: '', slide: { layout: 'two_columns', title: 'Riscos', left: 'Temps', right: 'Cost' }, version: 3 },
  ],
};

function fakeEdit() {
  const e = {
    calls: [], next: null, errors: [],
    reply(r) { e.next = r; },
    async put(id, slide, base) { e.calls.push(['put', id, slide, base]); return e.take({ version: 9, block: { id, slide, version: 9 } }); },
    async post(after, slide) { e.calls.push(['post', after, slide]); return e.take({ version: 9, blocks: [{ id: 'n', slide, version: 9 }] }); },
    async del(id, base) { e.calls.push(['del', id, base]); return e.take({ version: 9 }); },
    take(body) { const r = e.next || { status: 200, body }; e.next = null; return r; },
    tx: (ca, en) => en,
    error: m => e.errors.push(m),
  };
  return e;
}
const slidesOf = box => box.children[0];
const ids = box => slidesOf(box).children.map(e => e.dataset.id);
const button = (el, label) => el.all('button').find(b => b.textContent === label);

test('a deck renders one 16:9 slide per block, drawn from its fields', () => {
  const box = new Node(fakeDocument);
  view(box, DECK, window.gregalMD.render, fakeEdit());
  assert.deepEqual(ids(box), ['a', 'b', 'c']);
  const [a, b, c] = slidesOf(box).children;
  assert.match(a.innerHTML, /class="slide slide-title"/);
  assert.match(a.text, /Llançament 2027 Pla de llançament/);
  assert.match(b.innerHTML, /<li>Beta<\/li><li>Obert<\/li>/);
  assert.match(c.innerHTML, /class="slide-cols"/);
  assert.match(c.text, /Riscos Temps Cost/);
});

test('block events change the right slide: insert in its position, replace only that slide, delete removes it', () => {
  const box = new Node(fakeDocument);
  const v = view(box, DECK, window.gregalMD.render, fakeEdit());
  const [a, b] = slidesOf(box).children;
  v.apply({ op: 'insert', block: { id: 'x', slide: { layout: 'section', title: 'Part dos' } }, after: 'a', version: 4, author: 'agent' });
  v.apply({ op: 'insert', block: { id: 'y', slide: { layout: 'title', title: 'Portada' } }, version: 4, author: 'agent' });
  v.apply({ op: 'replace', block: { id: 'b', slide: { layout: 'big_number', title: 'Usuaris', number: '12.000' } }, version: 5, author: 'agent' });
  v.apply({ op: 'delete', block: { id: 'c' }, version: 6, author: 'agent' });
  assert.deepEqual(ids(box), ['y', 'a', 'x', 'b']);
  assert.match(slidesOf(box).children[2].text, /Part dos/);
  assert.match(b.text, /Usuaris 12\.000/);
  assert.equal(a.writes, 1, 'the title slide was not painted again');
  assert.equal(b.writes, 2, 'the replaced slide was painted once more');
  assert.equal(v.version, 6);
  assert.equal(v.apply({ op: 'delete', block: { id: 'a' }, version: 3, author: '' }), false, 'events the loaded deck has are ignored');
});

test('clicking a slide opens the form of its layout; saving sends the whole slide with its base_version', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, DECK, window.gregalMD.render, ed);
  const b = slidesOf(box).children[1];
  b.onclick({ target: {} });
  assert.deepEqual(Object.keys(v.editing.fields), ['title', 'body', 'bullets', 'notes']);
  assert.equal(v.editing.fields.bullets.value, 'Beta\nObert');
  assert.match(b.text, /Bullets, one per line/);
  await button(b, 'Save').onclick();
  assert.deepEqual(ed.calls, [], 'an untouched slide is not saved');
  assert.equal(v.editing, null);

  v.open('b');
  v.editing.fields.title.value = 'Fases noves';
  v.editing.fields.bullets.value = 'Beta\n\n  Obert\nTancament';
  v.editing.fields.notes.value = 'Parla poc';
  await v.save();
  assert.deepEqual(ed.calls, [['put', 'b', { layout: 'content', title: 'Fases noves', bullets: ['Beta', '  Obert', 'Tancament'], notes: 'Parla poc', chart: DECK.blocks[1].slide.chart }, 2]],
    'the fields the form does not show, like the chart, stay');
  assert.match(b.text, /Fases noves/);
  assert.equal(v.editing, null);

  v.open('c');
  assert.deepEqual(Object.keys(v.editing.fields), ['title', 'left_title', 'left', 'right_title', 'right', 'notes']);
  v.cancel();
  v.open('a');
  assert.deepEqual(Object.keys(v.editing.fields), ['title', 'subtitle', 'notes']);
});

test('a click on a form button that reaches the slide does not open the form again', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, DECK, window.gregalMD.render, ed);
  const b = slidesOf(box).children[1];
  b.onclick({ target: {} });
  // El navegador fa arribar el clic del botó a la diapositiva després del seu onclick.
  const cancel = button(b, 'Cancel');
  cancel.onclick();
  b.onclick({ target: cancel });
  assert.equal(v.editing, null, 'Cancel leaves the form closed');
  v.open('b');
  const save = button(b, 'Save');
  await save.onclick();
  b.onclick({ target: save });
  assert.equal(v.editing, null, 'an untouched Save leaves the form closed');
});

test('a 409 keeps the person\'s fields with the notice and the slide as it is now; saving again overwrites it', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, DECK, window.gregalMD.render, ed);
  v.open('c');
  const me = v.editing;
  me.fields.left.value = 'Temps i gent';
  ed.reply({ status: 409, body: { error: 'changed', block: { id: 'c', slide: { layout: 'two_columns', title: 'Riscos grans', left: 'Temps', right: 'Cost' }, version: 7 } } });
  await v.save();
  assert.equal(v.editing, me);
  assert.equal(me.fields.left.value, 'Temps i gent');
  assert.match(me.notice.innerHTML, /The agent changed this slide/);
  assert.match(me.notice.innerHTML, /Riscos grans/);
  await v.save();
  assert.deepEqual(ed.calls.at(-1), ['put', 'c', { layout: 'two_columns', title: 'Riscos', left: 'Temps i gent', right: 'Cost' }, 7], 'saved on top of the version now shown');
  assert.equal(v.editing, null);
});

test('a save that fails on the network is reported and keeps the form', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  ed.put = async () => { throw new Error('offline'); };
  const v = view(box, DECK, window.gregalMD.render, ed);
  v.open('a');
  v.editing.fields.title.value = 'No arriba';
  await v.save();
  assert.deepEqual(ed.errors, ['offline']);
  assert.equal(v.editing.fields.title.value, 'No arriba');
});

test('«New slide» adds one after the current slide with the chosen layout and opens its form', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, DECK, window.gregalMD.render, ed);
  const bar = box.children[1];
  const select = bar.all('select')[0];
  assert.equal(select.children.length, 10);
  assert.ok(select.children.some(o => o.value === 'big_number' && o.textContent === 'Big number'));
  v.open('a');
  select.value = 'big_number';
  await button(bar, 'New slide').onclick();
  assert.deepEqual(ed.calls, [['post', 'a', { layout: 'big_number' }]]);
  assert.deepEqual(ids(box), ['a', 'n', 'b', 'c']);
  assert.equal(v.editing.id, 'n');
  assert.deepEqual(Object.keys(v.editing.fields), ['title', 'number', 'caption', 'notes']);
  v.apply({ op: 'insert', block: { id: 'n', slide: { layout: 'big_number' } }, after: 'a', version: 9, author: 'user' });
  assert.deepEqual(ids(box), ['a', 'n', 'b', 'c'], 'the stream event of the same insert is not doubled');

  v.cancel();
  select.value = 'content';
  await button(bar, 'New slide').onclick();
  assert.deepEqual(ed.calls.at(-1), ['post', 'n', { layout: 'content' }], 'after the last slide opened');
});

test('an empty deck says so, and its first slide goes at the start', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  view(box, { id: 'p2', kind: 'deck', version: 1, blocks: [] }, window.gregalMD.render, ed);
  assert.match(slidesOf(box).text, /No slides yet/);
  const bar = box.children[1];
  bar.all('select')[0].value = 'title';
  await button(bar, 'New slide').onclick();
  assert.deepEqual(ed.calls, [['post', '', { layout: 'title' }]]);
  assert.deepEqual(ids(box), ['n']);
});

test('a slide can be deleted from its form; a 409 keeps it with the notice', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, DECK, window.gregalMD.render, ed);
  v.open('b');
  ed.reply({ status: 409, body: { error: 'changed', block: { id: 'b', slide: { layout: 'content', title: 'Fases (agent)' }, version: 8 } } });
  await button(slidesOf(box).children[1], 'Delete slide').onclick();
  assert.deepEqual(ids(box), ['a', 'b', 'c']);
  assert.match(v.editing.notice.innerHTML, /Fases \(agent\)/);
  await button(slidesOf(box).children[1], 'Delete slide').onclick();
  assert.deepEqual(ed.calls, [['del', 'b', 2], ['del', 'b', 8]]);
  assert.deepEqual(ids(box), ['a', 'c']);
  assert.equal(v.editing, null);
});

test('the form changes the slide\'s layout: the fields follow it and Save sends the new layout', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, DECK, window.gregalMD.render, ed);
  const b = slidesOf(box).children[1];
  v.open('b');
  const layout = b.all('select')[0];
  assert.equal(layout.value, 'content');
  assert.equal(layout.children.length, 10);
  v.editing.fields.title.value = 'Usuaris';
  layout.value = 'big_number';
  layout.onchange();
  assert.deepEqual(Object.keys(v.editing.fields), ['title', 'number', 'caption', 'notes']);
  assert.equal(v.editing.fields.title.value, 'Usuaris', 'what the person typed stays');
  assert.equal(b.all('input').filter(f => f.value === 'Usuaris').length, 1, 'the old fields are gone from the form');
  v.editing.fields.number.value = '12.000';
  await button(b, 'Save').onclick();
  assert.deepEqual(ed.calls, [['put', 'b', { ...DECK.blocks[1].slide, layout: 'big_number', title: 'Usuaris', number: '12.000' }, 2]]);
  assert.match(b.innerHTML, /slide-big_number/);
});

test('after a 409, clicking another slide or «New slide» keeps the form with the person\'s fields', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, DECK, window.gregalMD.render, ed);
  const [a, , c] = slidesOf(box).children;
  v.open('c');
  const me = v.editing;
  me.fields.left.value = 'Temps i gent';
  ed.reply({ status: 409, body: { error: 'changed', block: { id: 'c', slide: { layout: 'two_columns', title: 'Riscos grans' }, version: 7 } } });
  await v.save();
  await a.onclick({ target: {} });
  assert.equal(v.editing, me);
  assert.equal(me.fields.left.value, 'Temps i gent');
  assert.ok(c.all('form').length, 'the form is still there');
  await button(box.children[1], 'New slide').onclick();
  assert.equal(v.editing, me);
  assert.deepEqual(ed.calls.map(x => x[0]), ['put'], 'nothing else was sent');
  button(c, 'Cancel').onclick();
  await a.onclick({ target: {} });
  assert.equal(v.editing.id, 'a', 'Cancel lets the person leave');
});

test('clicking another slide saves the open form first when it was changed', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, DECK, window.gregalMD.render, ed);
  const [a, b] = slidesOf(box).children;
  v.open('a');
  v.editing.fields.title.value = 'Llançament 2028';
  await b.onclick({ target: {} });
  assert.deepEqual(ed.calls, [['put', 'a', { layout: 'title', title: 'Llançament 2028', subtitle: 'Pla de llançament' }, 1]]);
  assert.match(a.text, /Llançament 2028/);
  assert.equal(v.editing.id, 'b');
});

test('a failed delete says the slide was not deleted, not that it was not saved', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  ed.tx = ca => ca;
  const v = view(box, DECK, window.gregalMD.render, ed);
  v.open('b');
  ed.reply({ status: 404, body: { error: 'block not found' } });
  await button(slidesOf(box).children[1], 'Esborra la diapositiva').onclick();
  ed.del = async () => { throw new Error('offline'); };
  await button(slidesOf(box).children[1], 'Esborra la diapositiva').onclick();
  assert.deepEqual(ed.errors, ["No s'ha pogut esborrar la diapositiva: block not found", "No s'ha pogut esborrar la diapositiva: offline"]);
  assert.deepEqual(ids(box), ['a', 'b', 'c']);
  assert.equal(v.editing.id, 'b', 'the form stays to try again');

  const en = fakeEdit();
  en.reply({ status: 500, body: { error: 'disk full' } });
  const v2 = view(new Node(fakeDocument), DECK, window.gregalMD.render, en);
  v2.open('a');
  await button(v2.editing.form, 'Delete slide').onclick();
  assert.deepEqual(en.errors, ['Could not delete the slide: disk full']);
});

test('Enter in a field submits the form', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, DECK, window.gregalMD.render, ed);
  v.open('a');
  v.editing.fields.subtitle.value = 'Pla nou';
  let prevented = false;
  await v.editing.form.onsubmit({ preventDefault() { prevented = true; } });
  assert.ok(prevented);
  assert.deepEqual(ed.calls, [['put', 'a', { layout: 'title', title: 'Llançament 2027', subtitle: 'Pla nou' }, 1]]);
  assert.equal(v.editing, null);
});
