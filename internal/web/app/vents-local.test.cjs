'use strict';
// «A prop teu» a la web (vents-local.js): mateixes regles que LocalInfoCard
// d'Android i que /api/winds/local. Consentiment explícit, cap coordenada
// desada i el resum de la configuració en ca/en.
const { test } = require('node:test');
const assert = require('node:assert/strict');

async function withLang(lang, fn) {
  const prev = global.localStorage;
  global.localStorage = { getItem: () => lang, setItem() {} };
  try { return await fn(await import('./vents-local.js')); }
  finally { global.localStorage = prev; }
}

const topics = [{ id: 'concerts', label: 'Concerts i música' }, { id: 'news', label: 'Notícies locals' }];
const st = { enabled: true, consent: 1, place: { label: 'Gràcia, Barcelona', source: 'device' }, topics: ['concerts', 'news'], radius_km: 5, frequency: 'weekly', time: '18:30' };

test('el resum diu quan, on, a quina distància i què', async () => {
  await withLang('ca', m => {
    assert.equal(m.summaryLine(st, topics), 'Cada setmana a les 18:30 · Gràcia, Barcelona · a 5 km · Concerts i música, Notícies locals');
    assert.equal(m.summaryLine({ ...st, enabled: false }, topics), 'Desactivat.');
  });
  await withLang('en', m => {
    assert.equal(m.summaryLine({ ...st, frequency: '2days' }, topics), 'Every two days at 18:30 · Gràcia, Barcelona · within 5 km · Concerts i música, Notícies locals');
    assert.equal(m.frequencyLabel('daily', '09:00'), 'Every day at 09:00');
  });
});

test('no s’activa sense consentiment, sense lloc o sense temes', async () => {
  await withLang('ca', m => {
    assert.match(m.enableProblem({ consent: false, place: { label: 'Vic' }, topics: ['news'] }), /consentiment/);
    assert.match(m.enableProblem({ consent: true, place: null, topics: ['news'] }), /on ets/);
    assert.match(m.enableProblem({ consent: true, place: { label: '  ' }, topics: ['news'] }), /on ets/);
    assert.match(m.enableProblem({ consent: true, place: { label: 'Vic' }, topics: [] }), /tema/);
    assert.equal(m.enableProblem({ consent: true, place: { label: 'Vic' }, topics: ['news'] }), '');
  });
});

test('mai no surten coordenades exactes ni es desen', async () => {
  await withLang('ca', m => {
    assert.deepEqual(m.roundCoords(41.403712, 2.156789), { lat: 41.4, lon: 2.16 });
    const place = m.placeFromSearch({ name: 'Puigcerdà', where: 'Catalunya, Espanya', lat: 42.43, lon: 1.93 });
    assert.deepEqual(place, { label: 'Puigcerdà', region: 'Catalunya, Espanya', source: 'manual' });
    const body = m.payloadFrom({ enabled: true, consent: true, place: { ...place, lat: 42.43, lon: 1.93 }, topics: ['news'], radius_km: '25', frequency: 'bogus', time: '08:15', voice: true, wind: 'xaloc' });
    assert.deepEqual(body.place, { label: 'Puigcerdà', region: 'Catalunya, Espanya', country: '', source: 'manual' });
    assert.equal(JSON.stringify(body).includes('42.43'), false);
    assert.equal(body.radius_km, 25);
    assert.equal(body.frequency, 'daily');
    assert.equal(body.voice, true);
    assert.equal(body.wind, 'xaloc');
    assert.equal('place' in m.payloadFrom({ topics: [] }), false);
  });
});

test('«Prova-ho ara» no porta consentiment ni activa res, i diu què ha passat', async () => {
  await withLang('ca', m => {
    const body = m.payloadFrom({ ...st, enabled: true, consent: true, wind: 'ponent' });
    const p = m.tryPayload(body);
    assert.equal(p.action, 'try');
    assert.equal(p.place.label, 'Gràcia, Barcelona');
    assert.equal(p.wind, 'ponent');
    assert.ok(!('enabled' in p) && !('consent' in p) && !('lat' in p));
    assert.match(m.tryHeadline({ state: 'running', wind: 'Ponent', place: 'Vic', seconds: 12 }), /Ponent està mirant què hi ha a Vic… \(12 s\)/);
    assert.match(m.tryHeadline({ state: 'done', place: 'Vic', nothing: true, items: [] }), /No ha trobat res nou a Vic/);
    assert.match(m.tryHeadline({ state: 'done', place: 'Vic', seconds: 40, items: [{}, {}] }), /Ha trobat 2 coses a Vic en 40 s/);
  });
});
