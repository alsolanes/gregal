// Targetes riques del xat: recepta, pla, opcions amb foto, galeria i
// checklists. Tot surt escapat; les fotes només http(s) o imatges dels vents.
const test = require('node:test');
const assert = require('node:assert');
global.window = global;
require('./md.js');
const md = global.gregalMD;
const block = (lang, o) => '```' + lang + '\n' + JSON.stringify(o) + '\n```';

test('la recepta té racions, ingredients per marcar, passos amb temps i consell', () => {
  const h = md.render('Un plat ràpid.\n\n' + block('gregal-recipe', {
    title: 'Cigrons amb espinacs', time: '20 min', level: 'Fàcil', servings: 2,
    ingredients: [{ name: 'Cigrons cuits', qty: '400 g' }, { name: 'All i oli', qty: 'Al gust' }],
    steps: [{ text: 'Daura l\'all.' }, { text: 'Afegeix el tomàquet.', minutes: 8 }],
    tip: 'Un toc de pebre vermell.',
  }));
  assert.ok(h.includes('class="gregal-card gregal-recipe" data-serves="2"'), h);
  assert.strictEqual((h.match(/class="gc-check"/g) || []).length, 2);
  assert.ok(h.includes('data-qty="400 g"'));
  assert.ok(h.includes('<em class="gc-timer">8 min</em>'));
  assert.ok(h.includes('class="gc-tip"'));
  assert.ok(h.includes('gc-more'), 'the servings stepper');
});

test('les quantitats s\'escalen amb les racions i el que no té número no canvia', () => {
  assert.strictEqual(md.scaleQty('400 g', 2, 3), '600 g');
  assert.strictEqual(md.scaleQty('1,5 l', 2, 4), '3 l');
  assert.strictEqual(md.scaleQty('1,5 l', 2, 3), '2,25 l');
  assert.strictEqual(md.scaleQty('½ ceba', 2, 4), '1 ceba');
  assert.strictEqual(md.scaleQty('Al gust', 2, 6), 'Al gust');
  assert.strictEqual(md.scaleQty('aprox. 400 g', 2, 1), 'aprox. 200 g');
  assert.strictEqual(md.scaleQty('2 cullerades', 2, 1), '1 cullerades');
  assert.strictEqual(md.scaleQty('1/2 c. cafè', 2, 4), '1 c. cafè');
});

test('el pla és una línia de temps amb foto, total i fonts', () => {
  const h = md.render(block('gregal-plan', {
    kicker: 'Dissabte · escapada', title: 'Girona, al teu ritme', image: 'https://example.com/girona.jpg',
    items: [{ time: '08:05', title: 'Tren des de Sants', detail: 'Arribada 08:45' }, { time: '13:30', title: 'Dinar al centre' }],
    total: { label: 'Pressupost orientatiu', value: '42 €' }, sources: [{ name: 'Renfe', url: 'https://www.renfe.com/x' }],
  }));
  assert.ok(h.includes('class="gregal-card gregal-plan"'), h);
  assert.ok(h.includes('<img class="gc-hero" src="https://example.com/girona.jpg"'));
  assert.ok(h.includes('<span class="gc-time">08:05</span>'));
  assert.ok(h.includes('42 €') && h.includes('gr-foot'));
});

test('una foto de targeta mai és javascript: ni HTML; la d\'un vent es baixa amb la clau', () => {
  const bad = md.render(block('gregal-plan', { title: 'X<b>', image: 'javascript:alert(1)', items: [{ title: '<img src=x onerror=1>' }] }));
  assert.ok(!bad.includes('javascript:') && !bad.includes('<img src=x') && !bad.includes('X<b>'), bad);
  const wind = md.render(block('gregal-result', { title: 'Tria', image: '/api/winds/image?id=0123456789abcdef01234567.png', numbered: true, rows: [{ lead: 'Model A', value: '699 €', pick: true }] }));
  assert.ok(wind.includes('data-gregal-img="/api/winds/image?id=0123456789abcdef01234567.png"'), wind);
  assert.ok(wind.includes('<span class="gr-rank">01</span>'));
});

test('una targeta sense el mínim surt com a codi', () => {
  assert.ok(md.render(block('gregal-recipe', { title: 'Res' })).includes('class="codeblock"'));
  assert.ok(md.render('```gregal-plan\nno és json\n```').includes('class="codeblock"'));
});

test('fotos seguides fan una galeria i les checklists es poden marcar', () => {
  const g = md.render('![a](https://e.com/a.jpg)\n![b](https://e.com/b.jpg)');
  assert.ok(g.startsWith('<div class="md-gallery md-gallery-2">'), g);
  const one = md.render('Mira:\n![a](https://e.com/a.jpg)');
  assert.ok(one.startsWith('<p>'), 'text and a photo stay a paragraph');
  const c = md.render('- [ ] Passaport\n- [x] Bitllets');
  assert.ok(c.includes('<li class="md-task"><label><input type="checkbox" class="md-check">'), c);
  assert.ok(!c.includes('disabled'));
  assert.ok(c.includes('class="md-check" checked'));
});

test('el JSON a la mateixa línia de l\'obertura també és una targeta', () => {
  const h = md.render('```gregal-recipe {"title":"Cigrons","ingredients":[{"name":"Cigrons","qty":"400 g"}]}\n```');
  assert.ok(h.includes('gregal-recipe'), h);
  const one = md.render('Mira:\n```gregal-plan {"title":"Girona","items":[{"time":"08:05","title":"Tren"}]}```\nFi.');
  assert.ok(one.includes('gregal-plan') && one.includes('Fi.'), one);
  assert.ok(md.render('```js title\nconst a = 1\n```').includes('const'), 'a normal code block keeps its body');
});

test('la foto pendent (image: consulta) no es veu mentre arriba la resposta', () => {
  const h = md.render('Gaudí.\n\n![Sagrada Família](image: Sagrada Familia Barcelona)\n\nVes-hi.\n\n![Park Güell](image: Park Gu');
  assert.ok(!h.includes('image:'), h);
  assert.ok(!h.includes('Sagrada Família') && !h.includes('Park'), h);
  assert.ok(h.includes('Ves-hi.'), h);
});
