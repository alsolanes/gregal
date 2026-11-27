'use strict';
// El prestatge d'un vent a la web (vents-shelf.js): /api/winds/outputs en
// targetes amb la icona del tipus, el títol, la data, obrir i compartir.
const { test } = require('node:test');
const assert = require('node:assert/strict');

function fakeDocument() {
  const node = tag => ({
    tagName: tag.toUpperCase(), className: '', textContent: '', children: [], dataset: {}, attrs: {}, style: {},
    append(...c) { this.children.push(...c); },
    replaceChildren(...c) { this.children = c; },
    setAttribute(k, v) { this.attrs[k] = v; },
    getAttribute(k) { return this.attrs[k]; },
  });
  return { createElement: node };
}
const textOf = n => [n.textContent || '', ...(n.children || []).map(textOf)].join(' ');
const all = (n, pred) => [...(pred(n) ? [n] : []), ...(n.children || []).flatMap(c => all(c, pred))];
const byClass = (n, cls) => all(n, x => (' ' + x.className + ' ').includes(' ' + cls + ' '));

global.window = {};
global.location = { origin: 'https://gregal.example' };

async function withLang(lang, fn) {
  global.localStorage = { getItem: () => lang, setItem() {} };
  global.document = fakeDocument();
  return fn(await import('./vents-shelf.js'));
}

function fakeApi(body) {
  const calls = [];
  window.gregal = { api: async path => { calls.push(path); return body instanceof Error ? { ok: false, status: 500, text: async () => body.message } : { ok: true, json: async () => body }; } };
  return calls;
}

const today = new Date(); today.setHours(9, 5, 0, 0);
const items = [
  { at: today.toISOString(), kind: 'file', title: 'Pressupost.pdf', url: '/api/winds/doc?name=Pressupost.pdf' },
  { at: today.toISOString(), kind: 'site', title: 'Web del club', url: '/share/' + 'a'.repeat(40) + '/' },
  { at: '2026-11-20T10:00:00Z', kind: 'doc', title: 'Informe', url: '/api/docs/d1a2/export?scope=mine&format=docx' },
];

test('una targeta per peça: icona, títol, data i com obrir-la', async () => {
  await withLang('ca', async m => {
    const calls = fakeApi({ items });
    const box = document.createElement('div');
    await m.renderShelf(box, { wind: 'xaloc' });
    assert.deepEqual(calls, ['/api/winds/outputs?wind=xaloc']);
    const cards = byClass(box, 'shelf-card');
    assert.equal(cards.length, 3);
    assert.match(textOf(cards[0]), /📎 .*Pressupost\.pdf/);
    assert.match(textOf(cards[0]), /avui 09:05/);
    const link = c => all(c, x => x.tagName === 'A')[0];
    // El fitxer es baixa amb la clau (attach.js), la web s'obre, el document va a la seva pàgina.
    assert.equal(link(cards[0]).dataset.gregalDoc, '1');
    assert.equal(link(cards[1]).href, items[1].url);
    assert.equal(link(cards[2]).href, '/app/doc/d1a2?scope=mine');
    assert.match(textOf(cards[2]), /📄 .*Informe/);
    // Només el que s'obre sense clau es comparteix.
    const share = c => all(c, x => x.tagName === 'BUTTON' && x.textContent === 'Comparteix');
    assert.equal(share(cards[1]).length, 1);
    assert.equal(share(cards[0]).length + share(cards[2]).length, 0);
  });
});

test('compartir dona l’adreça sencera', async () => {
  await withLang('en', async m => {
    let copied = '';
    Object.defineProperty(global, 'navigator', { configurable: true, value: { clipboard: { writeText: async t => { copied = t; } } } });
    fakeApi({ items: [items[1]] });
    const box = document.createElement('div');
    await m.renderShelf(box, { wind: 'xaloc' });
    const b = all(box, x => x.tagName === 'BUTTON' && x.textContent === 'Share')[0];
    await b.onclick();
    assert.equal(copied, 'https://gregal.example/share/' + 'a'.repeat(40) + '/');
    assert.match(textOf(box), /Link copied/);
  });
});

test('prestatge buit i error, en el teu idioma', async () => {
  await withLang('en', async m => {
    fakeApi({ items: [] });
    const box = document.createElement('div');
    await m.renderShelf(box, { wind: 'xaloc' });
    assert.match(textOf(box), /hasn't made anything you can open yet/);
    fakeApi(new Error('boom'));
    await m.renderShelf(box, { wind: 'xaloc' });
    assert.match(textOf(box), /boom/);
  });
});
