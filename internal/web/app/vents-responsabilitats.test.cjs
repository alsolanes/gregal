'use strict';
// B6: la pantalla de responsabilitats de la web (mateixes regles que
// ResponsabilitatsSheet d'Android) i el «per què ho has fet?».
const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

// DOM mínim: prou per construir nodes i llegir-ne el text.
function fakeDocument() {
  const node = tag => ({
    tag, className: '', textContent: '', children: [], dataset: {}, attrs: {},
    append(...c) { this.children.push(...c); },
    replaceChildren(...c) { this.children = c; },
    setAttribute(k, v) { this.attrs[k] = v; },
    addEventListener() {},
  });
  return { createElement: node };
}
const textOf = n => [n.textContent || '', ...(n.children || []).map(textOf)].join(' ');

const sample = {
  id: 'resp-1', wind: 'xaloc', windName: 'Xaloc', kind: 'watch', objective: 'Horaris del bus',
  sources: [{ kind: 'web', url: 'https://example.org/bus', label: 'https://example.org/bus' }],
  allowed: ['explore', 'prepare'], askFirst: [], budget: { prepsPerDay: 6, messagesPerDay: 3 },
  spent: { preps: 1, messages: 1 }, state: 'active', pending: 0,
  log: [{ kind: 'prepared', sources: ['https://example.org/bus'], changes: ['El bus A1 deixa de parar a la T2'], proposal: 'Reservar el bus A2', messageId: 'm-1' }],
};

async function withLang(lang, fn) {
  const prevStorage = global.localStorage, prevDoc = global.document;
  global.localStorage = { getItem: () => lang, setItem() {} };
  global.document = fakeDocument();
  try { return await fn(await import('./vents-responsabilitats.js')); }
  finally { global.localStorage = prevStorage; global.document = prevDoc; }
}

test('estat i accions: el cor es pausa però no s’acaba ni s’esborra', async () => {
  await withLang('ca', m => {
    assert.equal(m.stateKey(sample), 'active');
    assert.equal(m.stateKey({ ...sample, waitingComputer: true }), 'waitingComputer');
    assert.equal(m.stateKey({ ...sample, waitingComputer: true, state: 'paused' }), 'paused');
    assert.deepEqual(m.actionsFor(sample), ['pause', 'finish']);
    assert.deepEqual(m.actionsFor({ ...sample, state: 'paused' }), ['resume', 'finish']);
    assert.deepEqual(m.actionsFor({ ...sample, state: 'done' }), ['delete']);
    assert.deepEqual(m.actionsFor({ ...sample, kind: 'heartbeat' }), ['pause']);
    assert.deepEqual(m.actionsFor({ ...sample, kind: 'heartbeat', state: 'paused' }), ['resume']);
  });
});

test('pressupost, què fa sola i textos ca/en', async () => {
  await withLang('ca', m => {
    assert.equal(m.budgetLine(sample), 'Avui: 1/6 preparacions · 1/3 missatges');
    assert.equal(m.canDo(sample), 'consultar, preparar (comparar, resumir, esborrany)');
  });
  await withLang('en', m => {
    assert.equal(m.budgetLine(sample), 'Today: 1/6 preparations · 1/3 messages');
    assert.equal(m.canDo({ ...sample, allowed: [] }), 'look things up, prepare (compare, summarise, draft)');
  });
});

test('crear: cal objectiu i una font; el cos és el de l’API', async () => {
  await withLang('ca', m => {
    assert.equal(m.canCreate({ wind: 'xaloc', objective: 'x', url: '', agenda: false }), false);
    assert.equal(m.canCreate({ wind: 'xaloc', objective: '  ', url: 'https://example.org', agenda: false }), false);
    assert.equal(m.canCreate({ wind: 'xaloc', objective: 'x', url: '', agenda: true }), true);
    assert.deepEqual(m.createBody({ wind: 'xaloc', objective: ' Preus ', url: ' https://example.org ', agenda: false, every: '30' }),
      { action: 'create', wind: 'xaloc', objective: 'Preus', agenda: false, urls: ['https://example.org'], everyMinutes: 30 });
    // Mai no envia permisos: allowed només es posa a l'API amb intenció.
    assert.equal('allowed' in m.createBody({ wind: 'x', objective: 'y', url: 'https://e.org' }), false);
  });
});

test('«per què?» només als missatges proactius i amb el contingut sencer', async () => {
  await withLang('ca', m => {
    assert.equal(m.isProactive({ from: 'pet', relatedRunId: 412 }), true);
    assert.equal(m.isProactive({ from: 'pet', resultRefs: ['res-1'] }), true);
    assert.equal(m.isProactive({ from: 'pet', text: 'hola' }), false);
    assert.equal(m.isProactive({ from: 'user', relatedRunId: 3 }), false);
    const body = textOf(m.whyBody({ responsabilitat: sample, act: sample.log[0] }));
    for (const s of ['Objectiu', 'Horaris del bus', 'https://example.org/bus', 'deixa de parar a la T2', 'Reservar el bus A2', 'sempre et pregunta']) {
      assert.ok(body.includes(s), 'falta «' + s + '» a: ' + body);
    }
    assert.match(textOf(m.whyBody(null)), /no ve de cap responsabilitat/);
  });
});

test('i18n: totes les claus resp.* que fa servir la pantalla, en català i en anglès', async () => {
  const src = fs.readFileSync(path.join(__dirname, 'vents-responsabilitats.js'), 'utf8');
  const used = new Set([...src.matchAll(/t\('(resp\.[A-Za-z_.]+)'/g)].map(x => x[1]).filter(k => !k.endsWith('.')));
  for (const k of ['state.active', 'state.paused', 'state.done', 'state.waitingComputer', 'level.explore', 'level.prepare',
    'act.prepared', 'act.silent', 'act.deduplicated', 'act.deferred', 'act.error', 'act.computer_unavailable', 'pause', 'resume', 'finish', 'delete']) used.add('resp.' + k);
  const i18n = fs.readFileSync(path.join(__dirname, 'i18n.js'), 'utf8');
  const enStart = i18n.indexOf("'pg.winds.kicker': 'Companions that persist'");
  assert.ok(enStart > 0);
  const caPart = i18n.slice(0, enStart), enPart = i18n.slice(enStart);
  for (const k of used) {
    assert.ok(caPart.includes("'" + k + "':"), 'falta ' + k + ' en català');
    assert.ok(enPart.includes("'" + k + "':"), 'falta ' + k + ' en anglès');
  }
});
