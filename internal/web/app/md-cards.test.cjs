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
  assert.ok(h.includes('<span class="gc-pill">08:05</span>'));
  assert.ok(h.includes('42 €') && h.includes('gr-foot') && h.includes('gc-budget'));
  // Hi ha un detall amagat: el peu porta «Detalls».
  assert.ok(h.includes('data-gc-toggle'), h);
});

test('el pla llarg ensenya quatre files; les accions i el trajecte surten com al disseny', () => {
  const items = ['A', 'B', 'C', 'D', 'E', 'F'].map((t, k) => ({ time: '1' + k + ':00', title: t }));
  const h = md.render(block('gregal-plan', { title: 'Dia llarg', items, actions: [{ label: 'Una altra idea' }, { label: 'Prepara el pla', prompt: 'Prepara-me el pla amb reserves', primary: true }] }));
  assert.strictEqual((h.match(/class="gc-extra"/g) || []).length, 2, h);
  assert.ok(h.includes('data-gc-toggle'), h);
  const cta = h.indexOf('class="gc-cta"'), alt = h.indexOf('class="gc-alt"');
  assert.ok(cta > 0 && alt > cta && h.includes('data-gregal-say="Prepara-me el pla amb reserves"') && h.includes('data-gregal-say="Una altra idea"'), h);
  const r = md.render(block('gregal-route', { from: 'Barcelona', to: 'Girona', depart: '08:05', arrive: '08:45', from_stop: 'Sants', mode: 'train', service: 'Avant', duration: '40 min', badge: 'La millor opció', price: '17,40 €',
    alternatives: [{ time: '08:16', service: 'R11', price: '8,95 €', url: 'https://www.renfe.com/r11' }, { time: '09:32', service: '<b>x</b>' }] }));
  assert.ok(r.includes('class="gregal-card gregal-route"') && r.includes('<b>08:05</b><small>Sants</small>') && r.includes('class="gc-mode"') && r.includes('gc-badge') && r.includes('17,40 €'), r);
  assert.ok(r.includes('href="https://www.renfe.com/r11"') && !r.includes('<b>x</b>'), r);
  assert.strictEqual(md.render(block('gregal-route', { from: 'A', depart: '08:00' })).includes('class="gregal-card gregal-route"'), false);
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

test('la comparació posa les coses en columnes, amb foto, els trets en files i el veredicte', () => {
  const h = md.render(block('gregal-compare', {
    title: 'Guepard o lleopard',
    items: [
      { name: 'Guepard', image: 'https://upload.wikimedia.org/c.jpg', facts: { Velocitat: '110 km/h', Pes: '35-70 kg' } },
      { name: 'Lleopard', image: 'javascript:alert(1)', facts: [{ label: 'Velocitat', value: '58 km/h' }, { label: 'Caça', value: 'De nit' }], pick: true },
    ],
    verdict: 'El guepard corre; el **lleopard** s\'enfila.',
  }));
  assert.ok(h.includes('class="gregal-card gregal-compare"'), h);
  assert.ok(h.includes('--gx-n:2'));
  assert.ok(h.includes('class="gx-photo" src="https://upload.wikimedia.org/c.jpg"'), h);
  assert.ok(!h.includes('javascript:'), 'only http(s) photos');
  assert.strictEqual((h.match(/class="gx-key"/g) || []).length, 3, 'Velocitat, Pes, Caça once each');
  assert.ok(h.includes('<span class="gx-val">—</span>'), 'a missing value shows a dash');
  assert.ok(h.includes('gx-pick') && h.includes('class="gx-verdict"') && h.includes('<b>lleopard</b>'), h);
  // Amb una sola cosa no és una comparació: es queda com a codi.
  assert.ok(!md.render(block('gregal-compare', { title: 'x', items: [{ name: 'A' }] })).includes('gregal-compare"'));
});

test('les xifres clau són rajoles amb el número gran i què vol dir', () => {
  const h = md.render(block('gregal-facts', {
    title: 'El cor en xifres',
    facts: [{ value: '300 g', label: 'de pes' }, { value: '60-80', label: 'batecs per minut', note: 'en repòs' }, { value: '<b>5 L</b>', label: 'de sang per minut' }],
  }));
  assert.ok(h.includes('class="gregal-card gregal-facts"'), h);
  assert.strictEqual((h.match(/class="gf-tile"/g) || []).length, 3);
  assert.ok(h.includes('<b>300 g</b><span>de pes</span>') && h.includes('<small>en repòs</small>'), h);
  assert.ok(!h.includes('<b><b>5 L'), 'values are escaped');
});
