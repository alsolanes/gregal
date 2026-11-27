const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

class El {
  constructor(tag) { this.tag = tag; this.children = []; this.parent = null; this.attrs = {}; this.className = ''; this.textContent = ''; }
  append(...c) { for (const x of c) { x.parent = this; this.children.push(x); } }
  remove() { if (this.parent) this.parent.children = this.parent.children.filter(x => x !== this); this.parent = null; }
  setAttribute(k, v) { this.attrs[k] = v; }
  focus() { this.focused = true; }
  select() { this.selected = true; }
  showModal() { this.open = true; }
  close() { this.open = false; }
}
const find = (el, tag) => el.tag === tag ? el : el.children.map(c => find(c, tag)).find(Boolean) || null;
const all = (el, tag) => [...(el.tag === tag ? [el] : []), ...el.children.flatMap(c => all(c, tag))];

function setup(t) {
  const body = new El('body');
  globalThis.document = { body, createElement: tag => new El(tag) };
  globalThis.gregalT = k => ({ 'ask.ok': "D'acord", 'ask.cancel': 'Cancel·la' })[k] || k;
  t.after(() => { delete globalThis.document; delete globalThis.gregalT; });
  delete require.cache[require.resolve('./ask.js')];
  return { body, ask: require('./ask.js').ask };
}

test('ask shows a modal with the label, the value, OK and Cancel, and resolves with the text on OK', async t => {
  const { body, ask } = setup(t);
  const answer = ask('Worktree name', 's-1124');
  const d = body.children[0];
  assert.equal(d.tag, 'dialog');
  assert.ok(d.open, 'the dialog is modal and open');
  const input = find(d, 'input');
  assert.equal(input.value, 's-1124');
  assert.ok(input.focused && input.selected);
  assert.match(find(d, 'label').textContent, /Worktree name/);
  const [no, ok] = all(d, 'button');
  assert.equal(ok.type, 'submit');
  assert.equal(ok.textContent, "D'acord");
  assert.equal(no.textContent, 'Cancel·la');
  input.value = 'proves';
  let prevented = false;
  find(d, 'form').onsubmit({ preventDefault: () => { prevented = true; } });
  assert.equal(await answer, 'proves');
  assert.ok(prevented, 'the page does not navigate');
  assert.equal(body.children.length, 0, 'the dialog is gone');
});

test('ask resolves with null on Cancel and on Escape, and starts empty without a value', async t => {
  const { body, ask } = setup(t);
  const cancelled = ask('Rename');
  assert.equal(find(body, 'input').value, '');
  all(body, 'button')[0].onclick();
  assert.equal(await cancelled, null);
  assert.equal(body.children.length, 0);

  const escaped = ask('Rename', 'x');
  let prevented = false;
  body.children[0].oncancel({ preventDefault: () => { prevented = true; } });
  assert.equal(await escaped, null);
  assert.ok(prevented);
  assert.equal(body.children.length, 0);
});

test('Escape in the ask dialog does not reach the page, so it does not also close the Kanban window under it', async t => {
  const { body, ask } = setup(t);
  const answer = ask('Rename', 'x');
  let stopped = false;
  body.children[0].onkeydown({ key: 'Escape', stopPropagation: () => { stopped = true; } });
  assert.ok(stopped);
  body.children[0].oncancel({ preventDefault() {} });
  assert.equal(await answer, null);
});

test('ask falls back to English OK and Cancel before the translations load', async t => {
  const { body, ask } = setup(t);
  delete globalThis.gregalT;
  const answer = ask('Rename', 'x');
  assert.deepEqual(all(body, 'button').map(b => b.textContent), ['Cancel', 'OK']);
  all(body, 'button')[0].onclick();
  assert.equal(await answer, null);
});

for (const file of ['attach.js', 'caps.js', 'kanban.js', 'panels.js']) {
  test(file + ' asks for text in the page, since prompt() does not exist in the desktop app', () => {
    const src = fs.readFileSync(path.join(__dirname, file), 'utf8');
    const calls = src.split(/\r?\n/).filter(l => !/^\s*\/\//.test(l) && /(^|[^\w.]|window\.)prompt\(/.test(l));
    assert.deepEqual(calls, []);
    assert.match(src, /gregalAsk\(/);
  });
}
