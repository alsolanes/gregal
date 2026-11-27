const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, 'index.html'), 'utf8').replace(/\r\n/g, '\n');

function slice(start, end) {
  const i = source.indexOf(start);
  assert.notEqual(i, -1, `missing start: ${start}`);
  const j = source.indexOf(end, i);
  assert.notEqual(j, -1, `missing end: ${end}`);
  return source.slice(i, j);
}

test('paintThinkTail renders structured thinking header and body without leading ellipsis', () => {
  const appended = [];
  const thinkEl = {
    className: '',
    innerHTML: '',
    querySelector(sel) {
      if (sel === '.wg-think-head span:last-child') return this._head;
      if (sel === '.wg-think-body') return this._body;
      return null;
    },
    _head: { textContent: '' },
    _body: { textContent: '' },
  };
  let currentThink = null;
  const body = {
    querySelector(sel) { return sel === ':scope > .wg-think' ? currentThink : null; },
    appendChild(el) { currentThink = el; appended.push(el); },
  };
  const turnGroup = {
    isConnected: true,
    querySelector(sel) { return sel === '.wg-body' ? body : null; },
  };
  const sandbox = {
    turnGroup,
    gregalT: k => (k === 'act.now.thinking' ? 'Pensant' : k),
    document: { createElement() { return thinkEl; } },
  };
  vm.runInNewContext(slice('function paintThinkTail(text)', 'function endWorkGroup('), sandbox);

  sandbox.paintThinkTail('  Analitzant els canvis recents  ');
  assert.equal(appended.length, 1);
  assert.equal(thinkEl.className, 'wg-think');
  assert.match(thinkEl.innerHTML, /wg-think-head/);
  assert.match(thinkEl.innerHTML, /wg-think-body/);
  assert.equal(thinkEl._head.textContent, 'Pensant');
  assert.equal(thinkEl._body.textContent, 'Analitzant els canvis recents');
});

test('pintaTodo renders progress bar percentage and accessible controls', () => {
  const todoBox = { hidden: true, innerHTML: '' };
  const cap = { onclick: null };
  const sandbox = {
    todoActual: [
      { marca: '✓', text: 'Pas 1' },
      { marca: '◌', text: 'Pas 2' },
      { marca: '☐', text: 'Pas 3' },
      { marca: '☐', text: 'Pas 4' },
    ],
    todoObert: true,
    $: id => (id === 'todoBox' ? todoBox : id === 'todoCap' ? cap : null),
    gregalT: k => ({ 'todo.title': 'Tasca', 'todo.allDone': 'tot fet' }[k] || k),
    escapeHTML: s => String(s),
    pintaTurnBar() {},
    localStorage: { setItem() {} },
  };
  vm.runInNewContext(slice('function pintaTodo()', '// En acabar el torn'), sandbox);

  sandbox.pintaTodo();
  assert.equal(todoBox.hidden, false);
  assert.match(todoBox.innerHTML, /<span class="todo-bar" aria-hidden="true"><i style="width:25%"><\/i><\/span>/);
  assert.match(todoBox.innerHTML, /<span class="todo-comptador">1\/4<\/span>/);
  assert.match(todoBox.innerHTML, /class="trow working"/);
});

test('stepChips and thinking blocks use structured classes in index.html and panels.js', () => {
  assert.match(source, /box = document\.createElement\('div'\);\s*\n\s*box\.className = 'step-chips';\s*\n\s*body\.appendChild\(box\);/);
  assert.match(source, /<details class="tool thinking"><summary><span class="tspin" aria-hidden="true"><\/span><b>'/);
  const panels = fs.readFileSync(path.join(__dirname, 'app', 'panels.js'), 'utf8');
  assert.match(panels, /<details class="tool thinking"><summary><span class="tspin" aria-hidden="true"><\/span><b>'/);
});
