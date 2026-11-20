// Text en viu: mentre la resposta arriba, els blocs ja acabats no es tornen
// a pintar; només es refà el que encara creix.
const test = require('node:test');
const assert = require('node:assert');
global.window = global;
require('./md.js');
const md = global.gregalMD;

const DOC = [
  '# Resum', '',
  'Un paràgraf amb **negreta**', 'i una segona línia.', '',
  '- primer', '  - niat', '- segon', '',
  '1. un', '', '2. dos', '',
  '> una cita', '> de dues línies', '',
  '| a | b |', '|---|--:|', '| 1 | 2 |', '',
  'Abans ```inline``` després.', '',
  '```go', 'func f() {', '', '\treturn', '}', '```', '',
  'Final amb `codi` i [enllaç](https://example.com).', '',
  '```sh', 'echo sense tancar', '', 'encara',
].join('\n');

const pieces = (text) => {
  const cut = md.liveCut(text);
  return [md.render(text.slice(0, cut)), md.render(text.slice(cut))];
};

test('un paràgraf, una llista i un bloc de codi acabats no canvien quan arriba més text', () => {
  const start = 'Un paràgraf.\n\n- a\n- b\n\n```js\nlet a = 1;\n\nlet b = 2;\n```\n\n';
  const [done] = pieces(start + 'Ara');
  assert.ok(done.includes('<p>Un paràgraf.</p>') && done.includes('<li>b</li>') && done.includes('codeblock'), done);
  const [later] = pieces(start + 'Ara ve més text.\n\n## I un títol\n\nfi');
  assert.ok(later.startsWith(done), 'el que ja estava acabat ha canviat');
});

test('un bloc de codi sense tancar queda sencer a la part oberta', () => {
  const text = 'Mira:\n\n```py\nx = 1\n\ny = 2';
  const cut = md.liveCut(text);
  assert.equal(text.slice(cut), '```py\nx = 1\n\ny = 2');
});

test('un text sense cap línia en blanc no té res acabat', () => {
  assert.equal(md.liveCut('una línia\ni una altra'), 0);
  assert.equal(md.liveCut(''), 0);
});

test('a cada tros que arriba, acabat + obert és el mateix que pintar-ho tot', () => {
  let last = 0;
  for (let n = 0; n <= DOC.length; n++) {
    const text = DOC.slice(0, n);
    const cut = md.liveCut(text);
    assert.ok(cut >= last, 'el tall recula a ' + n);
    last = cut;
    assert.equal(pieces(text).filter(Boolean).join('\n'), md.render(text), 'difereix a ' + n);
  }
  assert.ok(last > DOC.indexOf('Final'), 'el tall no avança fins al final');
});

// Un cos de bombolla de mentida: cada tros d'HTML que s'hi posa és un node.
function fakeBody() {
  const node = (html) => ({
    html,
    set innerHTML(h) { this.html = h; },
    get innerHTML() { return this.html; },
    insertAdjacentHTML(pos, h) {
      assert.equal(pos, 'beforebegin');
      if (h) body.childNodes.splice(body.childNodes.indexOf(this), 0, node(h));
    },
  });
  const body = {
    childNodes: [],
    get lastChild() { return this.childNodes[this.childNodes.length - 1]; },
    set innerHTML(h) { this.childNodes = h ? [node(h)] : []; },
    insertAdjacentHTML(pos, h) { assert.equal(pos, 'beforeend'); this.childNodes.push(node(h)); },
    // El que es veu: els blocs acabats i, a dins del contenidor de la cua, el que creix.
    get shown() { return this.childNodes.map((n) => n.html).filter(Boolean).join('\n'); },
  };
  return body;
}

test('paintLive deixa el node d\'un bloc acabat i només refà el que creix', () => {
  const body = fakeBody();
  body.innerHTML = '<span class="spin">…</span>';
  md.paintLive(body, 'Primer paràgraf.\n\nSegon a mi');
  const first = body.childNodes[0];
  assert.equal(first.html, '<p>Primer paràgraf.</p>');
  md.paintLive(body, 'Primer paràgraf.\n\nSegon a mitges.\n\n- i una llista');
  assert.strictEqual(body.childNodes[0], first, 'el bloc acabat s\'ha tornat a crear');
  assert.equal(body.shown, md.render('Primer paràgraf.\n\nSegon a mitges.\n\n- i una llista'));
});

test('paintLive no esborra el que una altra part de la pàgina afegeix al costat d\'un bloc acabat', () => {
  // plots.js posa la targeta del gràfic just després del seu bloc, ja acabat.
  const body = fakeBody();
  md.paintLive(body, 'Mira:\n\n```chart\n{"type":"bar"}\n```\n\nParàgraf acabat.\n\ncua');
  const card = { html: '<div class="plot-card"></div>' };
  body.childNodes.splice(1, 0, card); // després dels blocs acabats, abans de la cua
  md.paintLive(body, 'Mira:\n\n```chart\n{"type":"bar"}\n```\n\nParàgraf acabat.\n\ncua que creix');
  assert.ok(body.childNodes.includes(card), 'la targeta afegida ha desaparegut');
  assert.ok(body.shown.includes('<p>Paràgraf acabat.</p>'), 'un paràgraf acabat ha desaparegut');
  assert.ok(body.shown.endsWith('<p>cua que creix</p>'), body.shown);
});
