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
  // Cada web és un punt amb el nom al títol (com a la maqueta), i el peu en diu el nombre.
  assert.strictEqual((h.match(/title="idealo"/g) || []).length, 1);
  assert.ok(h.includes('title="amazon"'));
  assert.ok(h.includes('>2 sources<'));
  assert.ok(h.includes('class="gr-lead"'));
});

test('stamp afegeix quan es va mirar al peu de la targeta', () => {
  const html = card({ title: 'Trens', rows: [{ lead: '08:25', text: 'Directe', value: '59 €' }], sources: [{ name: 'Omio', url: 'https://www.omio.es/x' }] });
  const age = { textContent: '', dataset: {} };
  const m = /<span class="gr-age">([^<]*)<\/span>/.exec(html);
  assert.ok(m, 'hi ha el recompte de fonts');
  age.textContent = m[1];
  const node = { querySelectorAll: () => [age] };
  md.stamp(node, Math.round(Date.now() / 1000) - 120);
  assert.strictEqual(age.textContent, '1 source · 2 min ago');
});
