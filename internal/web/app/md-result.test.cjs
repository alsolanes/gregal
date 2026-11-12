// Targeta de resultat de Gregal (```gregal-result```) al markdown de la web.
const test = require('node:test');
const assert = require('node:assert');
global.window = global;
require('./md.js');
const md = global.gregalMD;
const card = (o) => md.render('Els millors:\n```gregal-result\n' + JSON.stringify(o) + '\n```');

test('pinta la targeta amb la fila recomanada', () => {
  const h = card({ title: 'BCN → Girona', rows: [{ lead: '08:05', text: 'Avant', value: '17,40 €', pick: true, tag: 'Recomanat' }],
    sources: [{ name: 'Renfe', url: 'https://www.renfe.com' }], more: { label: 'Veure els 11', url: 'https://www.renfe.com' } });
  assert.ok(h.includes('class="gregal-result"'));
  assert.ok(h.includes('gr-pick'));
  assert.ok(h.includes('Veure els 11'));
});

test('no deixa passar enllaços que no són http(s) ni HTML', () => {
  const h = card({ title: '<b>x</b>', rows: [{ text: '<img src=x onerror=1>', url: 'javascript:alert(1)' }], sources: [{ name: 'x', url: 'data:text/html,1' }] });
  assert.ok(!h.includes('javascript:'));
  assert.ok(!h.includes('data:text'));
  assert.ok(!h.includes('<img'));
  assert.ok(!h.includes('<b>x'));
});

test('un bloc mal fet surt com a codi', () => {
  assert.ok(md.render('```gregal-result\n{trencat\n```').includes('codeblock'));
  assert.ok(md.render('```gregal-result\n{"title":"sense files","rows":[]}\n```').includes('codeblock'));
});

test('cada web surt una sola vegada a les fonts', () => {
  const h = card({ title: 'Auriculars', rows: [{ lead: 'QCY H3', text: 'ANC', value: '25 €' }],
    sources: [{ name: 'idealo · QCY H3', url: 'https://www.idealo.es/a' }, { name: 'idealo · Sony', url: 'https://www.idealo.es/b' }, { name: 'Amazon', url: 'https://www.amazon.es/x' }] });
  assert.strictEqual((h.match(/>idealo</g) || []).length, 1);
  assert.ok(h.includes('>amazon<'));
  assert.ok(h.includes('class="gr-lead"'));
});
