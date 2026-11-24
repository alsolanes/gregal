'use strict';
// «Activitat» dels vents a la web (vents-activity.js): la línia de temps de
// /api/winds/activity agrupada per dia, amb la cara del vent, l'hora, el títol,
// l'estat i l'enllaç; en arribar al final demana els més vells amb `before`.
const { test } = require('node:test');
const assert = require('node:assert/strict');

// DOM mínim: prou per construir nodes, llegir-ne el text i prémer botons.
function fakeDocument() {
  const node = tag => ({
    tagName: tag.toUpperCase(), className: '', textContent: '', children: [], dataset: {}, attrs: {}, hidden: false,
    append(...c) { this.children.push(...c); },
    replaceChildren(...c) { this.children = c; },
    setAttribute(k, v) { this.attrs[k] = v; },
    getAttribute(k) { return this.attrs[k]; },
    addEventListener() {},
  });
  return { createElement: node };
}
const textOf = n => [n.textContent || '', ...(n.children || []).map(textOf)].join(' ');
const all = (n, pred) => [...(pred(n) ? [n] : []), ...(n.children || []).flatMap(c => all(c, pred))];
const byClass = (n, cls) => all(n, x => (' ' + x.className + ' ').includes(' ' + cls + ' '));
const buttons = (n, label) => all(n, x => x.tagName === 'BUTTON' && x.textContent === label);
const settle = () => new Promise(r => setTimeout(r, 0));

let observed = null;
global.window = { gregalWinds: { face: w => ({ tagName: 'SVG', className: 'face', textContent: '', children: [], dataset: { wind: w.id || w.name } }) } };
global.IntersectionObserver = class { constructor(cb) { this.cb = cb; observed = this; } observe(n) { this.target = n; } disconnect() { this.off = true; } };

async function withLang(lang, fn) {
  const prevStorage = global.localStorage, prevDoc = global.document;
  global.localStorage = { getItem: () => lang, setItem() {} };
  global.document = fakeDocument();
  try { return await fn(await import('./vents-activity.js')); }
  finally { global.localStorage = prevStorage; global.document = prevDoc; }
}

// Respostes de l'API per ruta; desa les crides.
function fakeApi(pages) {
  const calls = [];
  window.gregal = { api: async path => {
    calls.push(path);
    const body = pages(path);
    if (body instanceof Error) return { ok: false, status: 500, text: async () => body.message };
    return { ok: true, json: async () => body };
  } };
  return calls;
}

const local = (d, h, m) => new Date(2026, 10, d, h, m).toISOString();
const winds = [{ id: 'xaloc', name: 'Xaloc', color: '#efad85', style: 'pso' }, { id: 'ponent', name: 'Ponent', color: '#8eb4d6', style: 'psn' }];
const items = [
  { at: local(24, 9, 5), wind: 'xaloc', kind: 'task', title: 'Prepara la web del club', status: 'running', ref: 'task-1' },
  { at: local(24, 8, 0), wind: 'ponent', kind: 'news', title: 'Demà plourà', status: 'unread', ref: 'n-1' },
  { at: local(23, 18, 30), wind: 'xaloc', kind: 'routine', title: 'Concerts de la setmana', status: 'error', ref: 'job-1' },
  { at: local(20, 7, 0), wind: 'ponent', kind: 'heartbeat', title: 'El temps', status: 'ok', ref: 'wind-ponent' },
];

test('agrupa per dia, de més nou a més vell: avui, ahir i la data', async () => {
  await withLang('ca', m => {
    const groups = m.groupByDay(items, new Date(2026, 10, 24, 12, 0));
    assert.deepEqual(groups.map(g => g.items.length), [2, 1, 1]);
    assert.equal(groups[0].label, 'Avui');
    assert.equal(groups[1].label, 'Ahir');
    assert.match(groups[2].label, /20/);
  });
  await withLang('en', m => {
    const groups = m.groupByDay(items, new Date(2026, 10, 24, 12, 0));
    assert.deepEqual([groups[0].label, groups[1].label], ['Today', 'Yesterday']);
  });
});

test('l’estat és fet, ha fallat o espera, i l’enllaç diu què obre', async () => {
  await withLang('ca', m => {
    assert.deepEqual(m.statusChip({ status: 'done' }), { key: 'done', label: 'Fet' });
    assert.deepEqual(m.statusChip({ status: 'ok' }), { key: 'done', label: 'Fet' });
    assert.equal(m.statusChip({ status: 'error' }).key, 'failed');
    assert.equal(m.statusChip({ status: 'interrupted' }).key, 'failed');
    assert.equal(m.statusChip({ status: 'running' }).key, 'waiting');
    assert.equal(m.statusChip({ status: 'unread' }).key, 'waiting');
    assert.equal(m.statusChip({ status: '' }), null);
    assert.deepEqual(items.map(m.activityLink), ['task', 'chat', 'run', 'chat']);
  });
  await withLang('en', m => assert.deepEqual(m.statusChip({ status: 'failed' }), { key: 'failed', label: 'Failed' }));
});

test('cada element porta la cara del vent, l’hora, el títol, l’estat i l’enllaç', async () => {
  await withLang('ca', async m => {
    fakeApi(() => ({ items }));
    const opened = [];
    const box = document.createElement('div');
    await m.renderActivity(box, { winds, onOpen: (it, kind) => opened.push([it.ref, kind]) });
    const rows = byClass(box, 'act-item');
    assert.equal(rows.length, 4);
    const first = rows[0];
    assert.equal(byClass(first, 'face')[0].dataset.wind, 'xaloc');
    assert.match(textOf(first), /09:05/);
    assert.match(textOf(first), /Prepara la web del club/);
    assert.match(textOf(first), /Xaloc/);
    assert.equal(byClass(first, 'act-state')[0].className, 'act-state is-waiting');
    assert.equal(byClass(rows[2], 'act-state')[0].className, 'act-state is-failed');
    buttons(first, 'Obre la feina')[0].onclick();
    buttons(rows[1], 'Obre el xat')[0].onclick();
    assert.deepEqual(opened, [['task-1', 'task'], ['n-1', 'chat']]);
    assert.equal(buttons(rows[2], 'Mostra el resultat').length, 1);
  });
});

test('un vent sol demana només les seves; el run d’una rutina s’obre al lloc', async () => {
  await withLang('ca', async m => {
    const calls = fakeApi(path => path.startsWith('/api/winds/rutines/runs')
      ? { runs: [{ finished_at: items[2].at, status: 'error', error: 'sense xarxa' }] }
      : { items: [items[2]] });
    const box = document.createElement('div');
    await m.renderActivity(box, { wind: 'xaloc', winds });
    assert.equal(calls[0], '/api/winds/activity?wind=xaloc&limit=50');
    await buttons(box, 'Mostra el resultat')[0].onclick();
    assert.equal(calls[1], '/api/winds/rutines/runs?id=job-1');
    assert.match(textOf(box), /sense xarxa/);
  });
});

test('en arribar al final carrega els més vells amb before, fins que no n’hi ha més', async () => {
  await withLang('ca', async m => {
    const page1 = Array.from({ length: 50 }, (_, i) => ({ at: new Date(Date.UTC(2026, 10, 24, 12, 0) - i * 60000).toISOString().replace('Z', '+01:00'), wind: 'xaloc', kind: 'news', title: 'n' + i, status: 'read', ref: 'n-' + i }));
    const older = [{ at: '2026-11-20T08:00:00+01:00', wind: 'xaloc', kind: 'news', title: 'La més vella', status: 'read', ref: 'n-old' }];
    const calls = fakeApi(path => (path.includes('before=') ? { items: older } : { items: page1 }));
    const box = document.createElement('div');
    await m.renderActivity(box, { winds });
    assert.equal(byClass(box, 'act-item').length, 50);
    observed.cb([{ isIntersecting: true }]);
    await settle();
    assert.equal(calls[1], '/api/winds/activity?before=' + encodeURIComponent(page1[49].at) + '&limit=50');
    assert.equal(byClass(box, 'act-item').length, 51);
    assert.match(textOf(box), /La més vella/);
    assert.equal(observed.off, true);
  });
});

test('una llista buida s’explica i una càrrega fallida es pot tornar a provar', async () => {
  await withLang('ca', async m => {
    fakeApi(() => ({ items: [] }));
    const box = document.createElement('div');
    await m.renderActivity(box, { wind: 'xaloc', winds });
    assert.match(textOf(box), /Aquest vent encara no ha fet res pel seu compte/);
    let fail = true;
    fakeApi(() => (fail ? new Error('caigut') : { items }));
    await m.renderActivity(box, { winds });
    assert.match(textOf(box), /No s’ha pogut carregar l’activitat/);
    fail = false;
    await buttons(box, 'Torna-ho a provar')[0].onclick();
    assert.equal(byClass(box, 'act-item').length, 4);
  });
  await withLang('en', async m => {
    fakeApi(() => ({ items: [] }));
    const box = document.createElement('div');
    await m.renderActivity(box, { winds });
    assert.match(textOf(box), /Nothing yet/);
  });
});
