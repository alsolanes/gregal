// Imatges que fan els vents: ruta del servidor amb clau, l'app la baixa
// (attach.js); res més que aquesta ruta exacta es converteix en imatge.
const test = require('node:test');
const assert = require('node:assert');
global.window = global;
require('./md.js');
const md = global.gregalMD;

test('una imatge d\'un vent queda marcada per baixar-la amb la clau', () => {
  const h = md.render('Aquí el tens:\n\n![Cartell \"festa\"](/api/winds/image?id=0123456789abcdef01234567.jpg)');
  assert.ok(h.includes('data-gregal-img="/api/winds/image?id=0123456789abcdef01234567.jpg"'), h);
  assert.ok(!/<img[^>]* src=/.test(h), 'no src: it would go out without the key');
  assert.ok(!h.includes('"festa"'), 'alt escaped');
});

test('altres rutes relatives no es converteixen en imatge', () => {
  for (const u of ['/api/winds/image?id=../x.jpg', '/api/config', 'javascript:alert(1)']) {
    assert.ok(!md.render('![x](' + u + ')').includes('<img'), u);
  }
});

test('the Word link the doc tool gives is a link the app downloads with its key, not raw markdown', () => {
  const h = md.render("Aquí el tens: [El quadern](/api/docs/zjq2kr3jf3k2/export?format=docx)");
  assert.ok(h.includes('href="/api/docs/zjq2kr3jf3k2/export?format=docx"') && h.includes('data-gregal-doc="1"'), h);
  assert.ok(!h.includes(']('), h);
});

test('a link with a made-up scheme shows its text only: no raw markdown and no live link', () => {
  for (const u of ['gregal://docs/zjq2kr3jf3k2', 'javascript:alert(1)']) {
    const h = md.render("[Obre el document per veure'l](" + u + ')');
    assert.ok(!h.includes('](') && !h.includes('<a') && !h.includes(u), u + ' → ' + h);
    assert.ok(h.includes("Obre el document per veure"), h);
  }
});
