const test = require('node:test');
const assert = require('node:assert/strict');
const { view, column } = require('./sheet-view.js');

// Prou DOM per a la vista: fills en ordre, text, atributs i focus.
class Node {
  constructor(doc, tag) { this.ownerDocument = doc; this.tag = tag; this.children = []; this.dataset = {}; this.style = {}; this.value = ''; this.className = ''; this.textContent = ''; this.hidden = false; this.scrollTop = 0; this.clientHeight = 0; }
  replaceChildren(...els) { this.children = els; }
  append(...els) { this.children.push(...els); }
  setAttribute(k, v) { this[k] = v; }
  focus() { this.ownerDocument.activeElement = this; }
  scrollIntoView(opts) { this.ownerDocument.scrolled.push([this.dataset.cell, opts]); }
  all(tag) { return [...(this.tag === tag ? [this] : []), ...this.children.flatMap(c => c.all(tag))]; }
}
const fakeDocument = { scrolled: [], createElement: tag => new Node(fakeDocument, tag) };

const SHEET = {
  id: 'f1', kind: 'sheet', version: 3,
  blocks: [
    { id: 's1', name: 'Vendes', cells: { A1: '10', A2: '20', A3: '=SUM(A1:A2)', B1: 'Total' }, values: { A1: '10', A2: '20', A3: '30', B1: 'Total' }, version: 3 },
    { id: 's2', name: 'Costos', cells: { A1: '=Vendes!A3*2' }, values: { A1: '60' }, version: 2 },
  ],
};

function fakeEdit() {
  const e = {
    calls: [], next: null, errors: [],
    reply(r) { e.next = r; },
    async put(id, body, base) {
      e.calls.push(['put', id, body, base]);
      const [[k, v]] = Object.entries(body.cells);
      return e.take({ version: 9, block: { ...SHEET.blocks[0], cells: { ...SHEET.blocks[0].cells, [k]: v }, values: { ...SHEET.blocks[0].values, [k]: v.startsWith('=') ? '99' : v }, version: 9 } });
    },
    async post(after, body) { e.calls.push(['post', after, body]); return e.take({ version: 9, blocks: [{ id: 'n', name: body.name, cells: {}, values: {}, version: 9 }] }); },
    take(body) { const r = e.next || { status: 200, body }; e.next = null; return r; },
    tx: (ca, en) => en,
    error: m => e.errors.push(m),
  };
  return e;
}
const cell = (box, k) => box.all('td').find(td => td.dataset.cell === k);
const selected = box => box.all('td').filter(td => td.className.split(' ').includes('sel')).map(td => td.dataset.cell);
const tabs = box => box.children[0].children.map(b => b.textContent);
const key = (el, k, extra = {}) => el.onkeydown({ key: k, preventDefault() {}, ...extra });

test('a sheet is a grid with column letters, row numbers, one tab per sheet and the computed values', () => {
  const box = new Node(fakeDocument);
  view(box, SHEET, null, fakeEdit());
  assert.deepEqual(tabs(box), ['Vendes', 'Costos', 'New sheet']);
  assert.deepEqual(box.all('thead')[0].all('th').map(t => t.textContent), ['', 'A', 'B', 'C', 'D', 'E', 'F']);
  assert.deepEqual(box.all('tbody')[0].children.slice(0, 3).map(r => r.children[0].textContent), ['1', '2', '3']);
  assert.equal(box.all('tbody')[0].children.length, 20);
  assert.equal(cell(box, 'A3').textContent, '30', 'the value, not the formula');
  assert.equal(cell(box, 'B1').textContent, 'Total');
  assert.equal(column(26), 'AA');
});

test('block events are applied in place without moving the selection or the input being typed', () => {
  const box = new Node(fakeDocument);
  const v = view(box, SHEET, null, fakeEdit());
  cell(box, 'B2').onclick();
  v.apply({ op: 'replace', block: { ...SHEET.blocks[0], cells: { ...SHEET.blocks[0].cells, A1: '15' }, values: { ...SHEET.blocks[0].values, A1: '15', A3: '35' }, version: 4 }, version: 4, author: 'agent' });
  assert.equal(cell(box, 'A3').textContent, '35');
  assert.deepEqual(selected(box), ['B2']);
  v.bar.value = 'Meu';
  v.bar.oninput();
  v.apply({ op: 'replace', block: { ...SHEET.blocks[0], cells: { B2: 'Agent' }, values: { B2: 'Agent' }, version: 5 }, version: 5, author: 'agent' });
  assert.equal(v.bar.value, 'Meu', 'what the person types stays');
  assert.equal(cell(box, 'B2').textContent, 'Agent');
  v.apply({ op: 'insert', block: { id: 's3', name: 'Resum', cells: {}, values: {}, version: 6 }, after: 's1', version: 6, author: 'agent' });
  assert.deepEqual(tabs(box), ['Vendes', 'Resum', 'Costos', 'New sheet']);
  v.apply({ op: 'delete', block: { id: 's2' }, version: 7, author: 'agent' });
  assert.deepEqual(tabs(box), ['Vendes', 'Resum', 'New sheet']);
  assert.equal(v.apply({ op: 'delete', block: { id: 's1' }, version: 3, author: '' }), false, 'events the loaded sheet has are ignored');
  assert.deepEqual(selected(box), ['B2']);
});

test('clicking a cell shows its formula; Enter saves and moves down, Tab saves and moves right', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, SHEET, null, ed);
  cell(box, 'A3').onclick();
  assert.equal(v.bar.value, '=SUM(A1:A2)');
  assert.equal(box.all('span')[0].textContent, 'A3');
  await key(v.bar, 'Enter');
  assert.deepEqual(ed.calls, [], 'an untouched cell is not saved');
  assert.equal(v.selected, 'A4');

  v.bar.value = '=A3*2';
  v.bar.oninput();
  await key(v.bar, 'Enter');
  assert.deepEqual(ed.calls, [['put', 's1', { cells: { A4: '=A3*2' } }, 3]]);
  assert.equal(v.selected, 'A5');
  assert.equal(cell(box, 'A4').textContent, '99', 'the value the server computed');

  cell(box, 'C1').onclick();
  key(v.wrap, '7');
  assert.equal(v.bar.value, '7', 'typing on a cell replaces it');
  await key(v.bar, 'Tab');
  assert.deepEqual(ed.calls.at(-1), ['put', 's1', { cells: { C1: '7' } }, 9]);
  assert.equal(v.selected, 'D1');
});

test('arrow keys move the selection and Esc cancels the input', () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, SHEET, null, ed);
  key(v.wrap, 'ArrowDown');
  key(v.wrap, 'ArrowRight');
  key(v.wrap, 'ArrowRight');
  key(v.wrap, 'ArrowUp');
  key(v.wrap, 'ArrowUp');
  key(v.wrap, 'ArrowLeft');
  assert.equal(v.selected, 'B1', 'never above row 1');
  assert.deepEqual(selected(box), ['B1']);
  key(v.wrap, 'Enter');
  assert.equal(fakeDocument.activeElement, v.bar, 'Enter edits the cell');
  v.bar.value = 'Brut';
  v.bar.oninput();
  key(v.bar, 'Escape');
  assert.equal(v.bar.value, 'Total');
  assert.equal(fakeDocument.activeElement, v.wrap);
  assert.deepEqual(ed.calls, []);
});

test('a 409 keeps the person\'s input with the notice; Enter again saves on top of the sheet now shown', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, SHEET, null, ed);
  cell(box, 'A1').onclick();
  v.bar.value = '12';
  v.bar.oninput();
  ed.reply({ status: 409, body: { error: 'changed', block: { ...SHEET.blocks[0], cells: { ...SHEET.blocks[0].cells, A1: '11' }, values: { ...SHEET.blocks[0].values, A1: '11' }, version: 7 } } });
  await key(v.bar, 'Enter');
  assert.equal(v.selected, 'A1', 'it does not move');
  assert.equal(v.bar.value, '12');
  assert.equal(box.children[2].hidden, false);
  assert.match(box.children[2].textContent, /The agent changed this sheet · A1: 11/);
  assert.equal(cell(box, 'A1').textContent, '11');
  await key(v.bar, 'Enter');
  assert.deepEqual(ed.calls.at(-1), ['put', 's1', { cells: { A1: '12' } }, 7]);
  assert.equal(v.selected, 'A2');
  assert.equal(box.children[2].hidden, true);

  ed.put = async () => { throw new Error('offline'); };
  cell(box, 'B1').onclick();
  v.bar.value = 'Suma';
  v.bar.oninput();
  await key(v.bar, 'Enter');
  assert.deepEqual(ed.errors, ['offline']);
  assert.equal(v.bar.value, 'Suma');
  assert.equal(v.selected, 'B1');
});

test('tabs switch sheets and «New sheet» adds one after the last with a free name', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, SHEET, null, ed);
  box.children[0].children[1].onclick();
  assert.equal(v.sheet, 's2');
  assert.equal(cell(box, 'A1').textContent, '60');
  await box.children[0].children.at(-1).onclick();
  assert.deepEqual(ed.calls, [['post', 's2', { name: 'Sheet3' }]]);
  assert.deepEqual(tabs(box), ['Vendes', 'Costos', 'Sheet3', 'New sheet']);
  assert.equal(v.sheet, 'n');
  assert.equal(box.children[0].children[2].className, 'sheet-tab on');
});

test('a sheet with many rows draws only the rows in view', () => {
  const box = new Node(fakeDocument);
  const v = view(box, { id: 'f2', kind: 'sheet', version: 1, blocks: [{ id: 's1', name: 'Llarg', cells: { A2000: 'fi' }, values: { A2000: 'fi' }, version: 1 }] }, null, fakeEdit());
  v.wrap.clientHeight = 240;
  v.wrap.onscroll();
  const rows = () => box.all('tbody')[0].children.filter(r => r.className !== 'sheet-spacer');
  assert.ok(rows().length < 60, rows().length + ' rows drawn');
  assert.equal(rows()[0].children[0].textContent, '1');
  v.wrap.scrollTop = 1000 * 24;
  v.wrap.onscroll();
  assert.ok(rows().some(r => r.children[0].textContent === '1000'));
  assert.ok(!rows().some(r => r.children[0].textContent === '1'));
  const spacers = box.all('tbody')[0].children.filter(r => r.className === 'sheet-spacer');
  assert.equal(spacers.length, 2, 'spacers keep the scroll height');
});

test('switching tab while a cell is saving keeps the saved cells on their own sheet', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  let resolve;
  ed.put = (id, body, base) => { ed.calls.push(['put', id, body, base]); return new Promise(r => { resolve = r; }); };
  const v = view(box, SHEET, null, ed);
  cell(box, 'B2').onclick();
  v.bar.value = 'Meu';
  v.bar.oninput();
  const saving = key(v.bar, 'Enter');
  box.children[0].children[1].onclick();
  resolve({ status: 200, body: { version: 9, block: { ...SHEET.blocks[0], cells: { ...SHEET.blocks[0].cells, B2: 'Meu' }, values: { ...SHEET.blocks[0].values, B2: 'Meu' }, version: 9 } } });
  await saving;
  assert.deepEqual(tabs(box), ['Vendes', 'Costos', 'New sheet']);
  assert.equal(v.sheet, 's2');
  assert.equal(v.selected, 'A1', 'the selection on the other sheet does not move');
  assert.equal(cell(box, 'A1').textContent, '60', 'the other sheet is untouched');
  box.children[0].children[0].onclick();
  assert.equal(cell(box, 'B2').textContent, 'Meu', 'the saved sheet has the cell');
});

test('a 409 that arrives after switching tab does not land on the other sheet', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  let resolve;
  ed.put = () => new Promise(r => { resolve = r; });
  const v = view(box, SHEET, null, ed);
  cell(box, 'A1').onclick();
  v.bar.value = '12';
  v.bar.oninput();
  const saving = key(v.bar, 'Enter');
  box.children[0].children[1].onclick();
  resolve({ status: 409, body: { error: 'changed', block: { ...SHEET.blocks[0], cells: { ...SHEET.blocks[0].cells, A1: '11' }, values: { ...SHEET.blocks[0].values, A1: '11' }, version: 7 } } });
  await saving;
  assert.deepEqual(tabs(box), ['Vendes', 'Costos', 'New sheet']);
  assert.equal(cell(box, 'A1').textContent, '60');
  assert.equal(box.children[2].hidden, true);
  box.children[0].children[0].onclick();
  assert.equal(cell(box, 'A1').textContent, '11', 'the first sheet is now as the server has it');
});

test('moving the selection scrolls it into view under the sticky header, sideways too', () => {
  const box = new Node(fakeDocument);
  const v = view(box, SHEET, null, fakeEdit());
  v.wrap.clientHeight = 240; // the header and nine rows
  for (let i = 0; i < 8; i++) key(v.wrap, 'ArrowDown');
  assert.equal(v.selected, 'A9');
  assert.equal(v.wrap.scrollTop, 0, 'row 9 is still in view');
  key(v.wrap, 'ArrowDown');
  assert.equal(v.wrap.scrollTop, 24, 'row 10 is not left under the bottom edge');
  fakeDocument.scrolled.length = 0;
  key(v.wrap, 'ArrowRight');
  assert.deepEqual(fakeDocument.scrolled, [['B10', { block: 'nearest', inline: 'nearest' }]]);
});

test('an event while the person types keeps the base, so Enter gets a 409 instead of overwriting', async () => {
  const box = new Node(fakeDocument), ed = fakeEdit();
  const v = view(box, SHEET, null, ed);
  cell(box, 'A1').onclick();
  v.bar.value = '12';
  v.bar.oninput();
  v.apply({ op: 'replace', block: { ...SHEET.blocks[0], cells: { ...SHEET.blocks[0].cells, A1: '11' }, version: 4 }, version: 4, author: 'agent' });
  await key(v.bar, 'Enter');
  assert.deepEqual(ed.calls, [['put', 's1', { cells: { A1: '12' } }, 3]]);
});
