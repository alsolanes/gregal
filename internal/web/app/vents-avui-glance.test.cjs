'use strict';
// «Avui» d'una ullada a la web (vents-avui-glance.js): el temps, el calendari i
// el que segueixes a dalt de tot, a partir del `glance` de /api/winds/avui, i
// el control «Segueix…» que desa el lloc i els seguiments a /api/winds/avui/settings.
const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

// DOM mínim: prou per construir nodes, llegir-ne el text i prémer botons.
function fakeDocument() {
  const node = tag => ({
    tagName: tag.toUpperCase(), className: '', textContent: '', innerHTML: '', children: [], dataset: {}, attrs: {}, hidden: false, value: '',
    style: { setProperty(k, v) { this[k] = v; } },
    append(...c) { this.children.push(...c); },
    replaceChildren(...c) { this.children = c; },
    setAttribute(k, v) { this.attrs[k] = String(v); },
    getAttribute(k) { return this.attrs[k]; },
    addEventListener() {},
    focus() { this.focused = true; },
  });
  return { createElement: node };
}
const textOf = n => [n.textContent || '', ...(n.children || []).map(textOf)].join(' ');
const all = (n, pred) => [...(pred(n) ? [n] : []), ...(n.children || []).flatMap(c => all(c, pred))];
const byClass = (n, cls) => all(n, x => (' ' + x.className + ' ').includes(' ' + cls + ' '));
const buttons = (n, label) => all(n, x => x.tagName === 'BUTTON' && (x.textContent === label || x.attrs['aria-label'] === label));

global.window = {};
let reduced = true;
global.matchMedia = () => ({ matches: reduced });

async function withLang(lang, fn) {
  const prevStorage = global.localStorage, prevDoc = global.document;
  global.localStorage = { getItem: () => lang, setItem() {} };
  global.document = fakeDocument();
  try { return await fn(await import('./vents-avui-glance.js')); }
  finally { global.localStorage = prevStorage; global.document = prevDoc; }
}

function fakeApi() {
  const calls = [];
  window.gregal = { api: async (p, o) => { calls.push({ path: p, body: o?.body ? JSON.parse(o.body) : null }); return { ok: true, json: async () => ({}) }; } };
  return calls;
}

// Dimarts 24 de novembre de 2026, 12:00 (hora local).
const now = new Date(2026, 10, 24, 12, 0);
const local = (d, h, m) => new Date(2026, 10, d, h, m).toISOString();
const glance = {
  weather: { place: 'Girona', now_c: 13.6, min_c: 6.2, max_c: 15.4, rain_mm: 3.2, rain_hours: ['15', '16', '17', '21'], code: 61, sunrise: '2026-11-24T07:52', sunset: '2026-11-24T17:24' },
  events: [
    { start: local(24, 9, 0), end: local(24, 10, 0), title: 'Ja ha passat', calendar: 'Feina' },
    { start: local(24, 16, 30), end: local(24, 17, 0), title: 'Dentista', calendar: 'Personal' },
    { start: local(25, 10, 0), title: 'Reunió de l’escola', calendar: 'Família' },
  ],
  follows: [
    { id: 'f1', kind: 'team', name: 'Barça', title: 'Barça – Madrid', when: local(28, 21, 0), detail: 'LaLiga, jornada 14', url: 'https://example.org/partit', updated: local(24, 7, 0) },
    { id: 'f2', kind: 'other', name: 'Llançament del telescopi' },
  ],
};
const settings = { quiet_from: '22:00', quiet_to: '08:00', level: 'normal', place: 'Girona', follows: [{ id: 'f1', kind: 'team', name: 'Barça' }, { id: 'f2', kind: 'other', name: 'Llançament del telescopi', wind: 'xaloc' }] };

test('el moment d’un seguiment es diu com ho diria una persona', async () => {
  await withLang('ca', m => {
    assert.equal(m.followWhen(local(24, 21, 0), now), 'avui a les 21:00');
    assert.equal(m.followWhen(local(25, 9, 30), now), 'demà a les 09:30');
    assert.equal(m.followWhen(local(28, 21, 0), now), 'dissabte a les 21:00');
    assert.match(m.followWhen(local(12 + 30, 21, 0), now), /a les 21:00$/);
    assert.equal(m.followWhen('', now), '');
  });
  await withLang('en', m => {
    assert.equal(m.followWhen(local(24, 21, 0), now), 'today at 21:00');
    assert.equal(m.followWhen(local(25, 9, 30), now), 'tomorrow at 09:30');
    assert.equal(m.followWhen(local(28, 21, 0), now), 'Saturday at 21:00');
  });
});

test('el compte enrere diu quant falta i calla si ja ha passat', async () => {
  await withLang('ca', m => {
    assert.equal(m.countdown(local(24, 12, 25), now), 'd’aquí a 25 min');
    assert.equal(m.countdown(local(24, 14, 0), now), 'd’aquí a 2 h');
    assert.equal(m.countdown(local(26, 12, 0), now), 'd’aquí a 2 dies');
    assert.equal(m.countdown(local(25, 11, 0), now), 'd’aquí a 23 h');
    assert.equal(m.countdown(local(24, 11, 0), now), '');
  });
  await withLang('en', m => {
    assert.equal(m.countdown(local(24, 14, 0), now), 'in 2 h');
    assert.equal(m.countdown(local(26, 12, 0), now), 'in 2 days');
  });
});

test('les hores de pluja s’agrupen en franges', async () => {
  await withLang('ca', m => {
    assert.equal(m.rainText(['15', '16', '17', '21']), 'Pluja: 15–18 h, 21 h');
    assert.equal(m.rainText([]), 'Sense pluja');
  });
  await withLang('en', m => assert.equal(m.rainText(['07', '08']), 'Rain: 07–09 h'));
});

test('la targeta del temps: icona del codi, ara/mínima/màxima, hores de pluja i sol', async () => {
  await withLang('ca', m => {
    const box = document.createElement('div');
    m.renderGlance(box, { glance, settings, now });
    const wx = byClass(box, 'vw-glance-wx')[0];
    assert.ok(wx, 'hi ha targeta del temps');
    assert.equal(wx.dataset.kind, 'rain');
    assert.match(byClass(wx, 'vw-glance-icon')[0].innerHTML, /vw-gi-drops/);
    const text = textOf(wx);
    assert.match(text, /Girona/);
    assert.match(text, /14°/);
    assert.match(text, /6°/);
    assert.match(text, /15°/);
    assert.match(text, /Pluja: 15–18 h, 21 h/);
    assert.match(text, /07:52/);
    assert.match(text, /17:24/);
    const hours = byClass(wx, 'vw-glance-hour');
    assert.equal(hours.length, 24);
    assert.deepEqual(hours.filter(h => h.className.includes('is-rain')).map(h => h.dataset.h), ['15', '16', '17', '21']);
    assert.ok(hours[12].className.includes('is-now'));
  });
});

test('el calendari ensenya el que ve, i el de demà ho diu', async () => {
  await withLang('ca', m => {
    const box = document.createElement('div');
    m.renderGlance(box, { glance, settings, now });
    const cal = byClass(box, 'vw-glance-cal')[0];
    const rows = byClass(cal, 'vw-glance-event');
    assert.equal(rows.length, 2);
    assert.match(textOf(rows[0]), /16:30/);
    assert.match(textOf(rows[0]), /Dentista/);
    assert.match(textOf(rows[0]), /Personal/);
    assert.match(textOf(rows[1]), /demà 10:00/);
    assert.doesNotMatch(textOf(cal), /Ja ha passat/);
  });
});

test('una targeta per seguiment: títol, quan, detall, enllaç i compte enrere', async () => {
  await withLang('ca', m => {
    const box = document.createElement('div');
    m.renderGlance(box, { glance, settings, now });
    const cards = byClass(box, 'vw-glance-follow');
    assert.equal(cards.length, 2);
    const [match, other] = cards;
    assert.equal(match.dataset.kind, 'team');
    const text = textOf(match);
    assert.match(text, /Barça – Madrid/);
    assert.match(text, /dissabte a les 21:00/);
    assert.match(text, /LaLiga, jornada 14/);
    assert.match(text, /d’aquí a 4 dies/);
    const link = all(match, n => n.tagName === 'A')[0];
    assert.equal(link.href, 'https://example.org/partit');
    assert.equal(link.target, '_blank');
    assert.match(link.rel, /noopener/);
    assert.match(textOf(other), /Llançament del telescopi/);
    assert.match(textOf(other), /ho buscarà/);
  });
  await withLang('en', m => {
    const box = document.createElement('div');
    m.renderGlance(box, { glance, settings, now });
    assert.match(textOf(byClass(box, 'vw-glance-follow')[0]), /Saturday at 21:00/);
  });
});

test('«Segueix…» afegeix un equip o una altra cosa i desa el lloc', async () => {
  await withLang('ca', async m => {
    const calls = fakeApi();
    let changed = 0;
    const box = document.createElement('div');
    m.renderGlance(box, { glance, settings, now, onChange: () => changed++ });
    const form = byClass(box, 'vw-glance-form')[0];
    assert.equal(form.hidden, true);
    buttons(box, 'Segueix…')[0].onclick();
    assert.equal(form.hidden, false);
    const name = byClass(form, 'vw-glance-name')[0];
    name.value = '  Girona FC ';
    await byClass(form, 'vw-glance-add')[0].onsubmit({ preventDefault() {} });
    assert.equal(calls[0].path, '/api/winds/avui/settings');
    assert.deepEqual(calls[0].body.follows, [...settings.follows, { kind: 'team', name: 'Girona FC' }]);
    // «Una altra cosa».
    buttons(form, 'Una altra cosa')[0].onclick();
    name.value = 'Eclipsi';
    await byClass(form, 'vw-glance-add')[0].onsubmit({ preventDefault() {} });
    assert.deepEqual(calls[1].body.follows.at(-1), { kind: 'other', name: 'Eclipsi' });
    // Un nom buit no envia res.
    name.value = '   ';
    await byClass(form, 'vw-glance-add')[0].onsubmit({ preventDefault() {} });
    assert.equal(calls.length, 2);
    const place = byClass(form, 'vw-glance-place-in')[0];
    assert.equal(place.value, 'Girona');
    place.value = 'Sóller';
    await byClass(form, 'vw-glance-placeform')[0].onsubmit({ preventDefault() {} });
    assert.deepEqual(calls[2].body, { place: 'Sóller' });
    assert.equal(changed, 3);
  });
});

test('cada seguiment es pot treure', async () => {
  await withLang('ca', async m => {
    const calls = fakeApi();
    const box = document.createElement('div');
    m.renderGlance(box, { glance, settings, now, onChange() {} });
    await buttons(box, 'Deixa de seguir Barça')[0].onclick();
    assert.deepEqual(calls[0].body, { follows: [settings.follows[1]] });
  });
  await withLang('en', m => {
    const box = document.createElement('div');
    m.renderGlance(box, { glance, settings, now });
    assert.equal(buttons(box, 'Stop following Barça').length, 1);
  });
});

test('sense lloc ni seguiments, les targetes conviden a afegir-los', async () => {
  await withLang('ca', m => {
    const box = document.createElement('div');
    m.renderGlance(box, { glance: { events: [], follows: [] }, settings: { follows: [] }, now });
    assert.equal(byClass(box, 'vw-glance-wx').length, 0);
    assert.equal(byClass(box, 'vw-glance-cal').length, 0);
    const invites = byClass(box, 'vw-glance-invite');
    assert.equal(invites.length, 2);
    assert.match(textOf(invites[0]), /on ets/);
    assert.match(textOf(invites[1]), /equip/);
    const form = byClass(box, 'vw-glance-form')[0];
    buttons(invites[0], 'Digues on ets')[0].onclick();
    assert.equal(form.hidden, false);
    assert.equal(byClass(form, 'vw-glance-place-in')[0].focused, true);
  });
  await withLang('en', m => {
    const box = document.createElement('div');
    m.renderGlance(box, { glance: { events: [], follows: [] }, settings: {}, now });
    assert.match(textOf(box), /Follow…/);
    assert.match(textOf(box), /Tell me where you are/);
  });
});

test('els comptes enrere corren sols, però no amb moviment reduït', async () => {
  await withLang('ca', m => {
    const started = [];
    const prev = global.setInterval;
    global.setInterval = (fn, ms) => { started.push({ fn, ms }); return 1; };
    try {
      reduced = true;
      m.renderGlance(document.createElement('div'), { glance, settings, now });
      assert.equal(started.length, 0);
      reduced = false;
      const box = document.createElement('div');
      box.isConnected = true;
      m.renderGlance(box, { glance: { ...glance, follows: [{ ...glance.follows[0], when: local(24, 14, 0) }] }, settings, now });
      assert.equal(started.length, 1);
      const pill = byClass(box, 'vw-glance-count')[0];
      assert.equal(pill.textContent, 'd’aquí a 2 h');
      m.tickCountdowns(box, new Date(2026, 10, 24, 13, 40));
      assert.equal(pill.textContent, 'd’aquí a 20 min');
    } finally { global.setInterval = prev; reduced = true; }
  });
});

test('les animacions són de transform/opacity i s’apaguen amb moviment reduït', () => {
  const css = fs.readFileSync(path.join(__dirname, 'winds-workspace.css'), 'utf8');
  const block = css.slice(css.indexOf('/* Avui d’una ullada'), css.indexOf('/* fi Avui d’una ullada */'));
  assert.ok(block.length > 100, 'hi ha el bloc de CSS de la ullada');
  const frames = [...block.matchAll(/@keyframes[^{]+\{((?:[^{}]*\{[^}]*\})*)\s*\}/g)].map(m => m[1]);
  assert.ok(frames.length >= 3, 'entrada, sol i pluja');
  for (const f of frames) for (const [, prop] of f.matchAll(/([a-z-]+)\s*:/g)) assert.ok(['transform', 'opacity'].includes(prop), 'anima ' + prop);
  assert.match(block, /@media\s*\(prefers-reduced-motion:\s*no-preference\)/);
  const outside = block.replace(/@media\s*\(prefers-reduced-motion:\s*no-preference\)\s*\{(?:[^{}]*\{[^}]*\})*\s*\}/g, '');
  assert.doesNotMatch(outside, /animation\s*:/, 'cap animació fora de no-preference');
});
