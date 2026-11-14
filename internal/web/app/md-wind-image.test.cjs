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
