// Un encàrrec d'un altre vent amb l'eina wind (by a /api/winds/chat) es dibuixa com
// una fila d'encàrrec amb la cara de qui el demana, no com una bombolla teva (winds-workspace.js).
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function node(tag) {
  return {
    tagName: tag, className: '', textContent: '', children: [], dataset: {},
    append(...n) { this.children.push(...n); },
    setAttribute() {}, style: { setProperty() {} },
  };
}
const all = n => [n, ...(n.children || []).flatMap(all)];

function load(lang) {
  const faces = [];
  const window = {
    gregalWinds: { face(w) { faces.push(w); const f = node('svg'); f.className = 'winds-face'; f.dataset.wind = w.id; return f; } },
  };
  const document = { documentElement: { lang }, addEventListener() {}, createElement: node };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'winds-workspace.js'), 'utf8'), {
    window, document, getComputedStyle: () => ({ getPropertyValue: () => '' }),
  });
  return { handoffRow: window.gregalWindChat.handoffRow, faces };
}

test('a message handed over by another wind shows its face and «Task from X», not a user bubble', () => {
  const { handoffRow, faces } = load('en');
  const row = handoffRow({ from: 'user', text: 'Task from Xaloc: Make a snake game', by: { id: 'xaloc', name: 'Xaloc' } });
  const nodes = all(row);
  assert.ok(!/vw-user/.test(row.className), 'not drawn as your bubble');
  assert.equal(faces.length, 1);
  assert.equal(faces[0].id, 'xaloc');
  assert.ok(nodes.some(n => n.className === 'winds-face' && n.dataset.wind === 'xaloc'), 'the asker’s face is in the row');
  assert.ok(nodes.some(n => n.textContent === 'Task from Xaloc'));
  // The task text keeps no second copy of who asked.
  assert.ok(nodes.some(n => n.textContent === 'Make a snake game'));
});

test('in Catalan the row says «Encàrrec de X»', () => {
  const { handoffRow } = load('ca');
  const nodes = all(handoffRow({ from: 'user', text: 'Encàrrec de Gregal: Fes un joc', by: { id: 'gregal', name: 'Gregal' } }));
  assert.ok(nodes.some(n => n.textContent === 'Encàrrec de Gregal'));
  assert.ok(nodes.some(n => n.textContent === 'Fes un joc'));
});
