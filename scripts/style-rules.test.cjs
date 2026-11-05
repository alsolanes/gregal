const test = require('node:test');
const assert = require('node:assert/strict');
const r = require('./style-rules.cjs');

const has = (list, re) => list.some(p => re.test(p));

test('una resposta curta, càlida i en català passa', () => {
  const t = "Bon dia! Avui tens el calendari lliure, així que és un bon dia per avançar amb la web. Vols que en preparem l'esquelet?";
  assert.deepEqual(r.checkReply(t, { speaker: 'Marinada', others: ['Ponent'], lang: 'ca', kind: 'casual' }), []);
});

test('el seu propi nom al principi, també amb negreta', () => {
  assert.ok(has(r.checkReply('Marinada: Hola, com va?', { speaker: 'Marinada' }), /propi nom/));
  assert.ok(has(r.checkReply('**Marinada:** Hola, com va?', { speaker: 'Marinada' }), /propi nom/));
});

test('parlar per un altre vent dins la pròpia veu', () => {
  const t = 'Jo faria una truita.\nPonent: I jo una amanida!';
  assert.ok(has(r.checkReply(t, { speaker: 'Marinada', others: ['Ponent'] }), /com si fos Ponent/));
});

test('marcadors interns filtrats', () => {
  assert.ok(has(r.checkReply('El partit és dissabte.\nNEXT: 2026-11-08T21:00:00Z'), /NEXT/));
  assert.ok(has(r.checkReply('<tool_call>web_search</tool_call>'), /eina/));
  assert.ok(has(r.checkReply('ASSISTANT Ponent: hola'), /rol del transcript/));
});

test('format de document en una xerrada', () => {
  const t = '## Opcions\n- Truita\n- Amanida\n- Sopa';
  const p = r.checkReply(t, { kind: 'casual' });
  assert.ok(has(p, /títols/));
  assert.ok(has(p, /punts de llista/));
  // En una explicació sí que pot haver-hi estructura.
  assert.deepEqual(r.checkReply(t, { kind: 'explain' }).filter(x => /títols|llista/.test(x)), []);
});

test('massa llarg per a una xerrada', () => {
  const t = 'U. Dos. Tres. Quatre. Cinc. Sis.';
  assert.ok(has(r.checkReply(t, { kind: 'casual' }), /frases/));
});

test('idioma equivocat', () => {
  const t = 'Today the calendar is free, so it is a good day to work on the site and you can also rest.';
  assert.ok(has(r.checkReply(t, { lang: 'ca' }), /idioma en/));
  const es = 'Hoy tienes el calendario libre, así que puedes avanzar con la web y también descansar un poco.';
  assert.equal(r.guessLang(es), 'es');
  assert.equal(r.guessLang("Avui tens el calendari lliure, així que pots avançar amb la web i també descansar."), 'ca');
});

test('emojis, preguntes i farciment', () => {
  assert.ok(has(r.checkReply('Fet! 🎉🎉🔥'), /emojis/));
  assert.ok(has(r.checkReply('Què vols? I quan? I on?'), /preguntes/));
  assert.ok(has(r.checkReply('Aquí ho tens. Espero que això t\'ajudi!'), /farciment/));
  assert.ok(has(r.checkReply('*somriu* Hola!'), /asteriscs/));
  assert.deepEqual(r.checkReply('Això és **important** per a tu.').filter(x => /asteriscs/.test(x)), []);
});

test('massa enllaços en una veu de grup', () => {
  const t = 'Mira [a](https://a.cat), [b](https://b.cat) i [c](https://c.cat).';
  assert.ok(has(r.checkReply(t, { kind: 'group-voice' }), /enllaços/));
});

test('torn de grup: veus que repeteixen el mateix consell', () => {
  const v = [
    { speaker: 'Marinada', text: 'Per sopar ràpid, fes una truita de patates amb ceba i una amanida verda.' },
    { speaker: 'Ponent', text: 'Una truita de patates amb ceba i una amanida verda és el sopar ràpid ideal.' },
  ];
  assert.ok(has(r.checkGroupTurn(v, { members: ['Marinada', 'Ponent'] }), /gairebé el mateix/));
});

test('torn de grup: menció a un sol vent', () => {
  const v = [{ speaker: 'Marinada', text: 'Jo crec que sí.' }, { speaker: 'Ponent', text: 'I jo que no.' }];
  assert.ok(has(r.checkGroupTurn(v, { members: ['Marinada', 'Ponent'], mention: 'Marinada' }), /menció/));
});

test('torn de grup bo: veus que es responen sense repetir-se', () => {
  const v = [
    { speaker: 'Marinada', text: 'Jo faria una truita de patates, que en vint minuts està llesta.' },
    { speaker: 'Ponent', text: 'Bona idea, Marinada; jo hi afegiria pa amb tomàquet i deixaria les postres per demà.' },
  ];
  assert.deepEqual(r.checkGroupTurn(v, { members: ['Marinada', 'Ponent'], lang: 'ca' }), []);
});

test('informe de colla', () => {
  assert.ok(has(r.checkCrewReport('{"status":"done","files":["a"]}'), /JSON/));
  assert.ok(has(r.checkCrewReport('He fet la web. Tot funciona i els tests passen.'), /proves/));
  assert.deepEqual(r.checkCrewReport('He fet la web del comptador a `index.html`. Hem provat `node test.js`: 3 tests passen.', { lang: 'ca' }), []);
});

test('títols en castellà dins una resposta catalana no canvien l\'idioma', () => {
  const t = "Per un cap de setmana: **La plaça del Diamant**, de Rodoreda, o *Cien años de soledad* si vols una cosa més llarga. Jo començaria divendres a la nit i dissabte ja no el deixaries.";
  assert.equal(r.guessLang(t), 'ca');
});

test('respostes ràpides en un altre idioma', () => {
  const t = "12 minutes, zero equipment: squats, push-ups and a plank. You will be done before lunch.\n\n```choices\nFes-ho més suau, per a dia fluix\nVull una versió amb mancuernes i més temps\n```";
  assert.ok(has(r.checkReply(t, { lang: 'en' }), /respostes ràpides en ca/));
});

test('torn de grup: el mateix vent dues vegades (A, B, A, B)', () => {
  const v = [{ speaker: 'A', text: 'Menja un plàtan abans.' }, { speaker: 'B', text: 'En dejú, si és suau.' }, { speaker: 'A', text: 'Jo hi afegiria mel.' }];
  assert.ok(has(r.checkGroupTurn(v, { members: ['A', 'B'] }), /A parla 2 vegades/));
});

test('bloc choices en una sola línia', () => {
  assert.ok(has(r.checkReply('Fet.\n\n```choices Força al gimnàs Córrer fluix ```'), /una sola línia/));
});

test('una paraula en cursiva enmig de la frase no és una acció de rol', () => {
  assert.deepEqual(r.checkReply('Entrenar en dejú crema més greix *durant* la sessió, però no al cap dels dies.').filter(x => /asteriscs/.test(x)), []);
});
