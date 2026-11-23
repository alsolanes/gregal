// Inici de Gregal Docs: el botó de la barra lateral obre el panell de
// documents (docs-panel.js) amb «Els meus documents» (?scope=mine) i, a Codi,
// els del projecte obert; i «Document nou» i «Presentació nova», que la
// persona escriu sense l'agent, i «Full de càlcul nou».
// UMD: window.gregalDocHome al navegador, require() als tests de Node.
(function (root, factory) {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.gregalDocHome = api;
})(typeof self !== 'undefined' ? self : this, function () {
  'use strict';

  const KINDS = { doc: ['Document', 'Document'], deck: ['Presentació', 'Presentation'], sheet: ['Full de càlcul', 'Spreadsheet'] };

  // home pinta l'inici dins box. lists = {mine, project}: mine són els
  // items de GET /api/docs?scope=mine; project, {name, items} del projecte
  // obert, o null si no n'hi ha. act = {open(id, scope), create(title, kind), tx, lang}.
  function home(box, lists, act) {
    const D = box.ownerDocument, tx = act.tx;
    const el = (tag, cls, text) => {
      const e = D.createElement(tag);
      if (cls) e.className = cls;
      if (text !== undefined) e.textContent = text;
      return e;
    };
    const when = iso => new Date(iso).toLocaleString(act.lang === 'en' ? 'en-GB' : 'ca', { dateStyle: 'medium', timeStyle: 'short' });
    const section = (title, items, scope) => {
      const s = el('section', 'docs-home-section');
      s.append(el('h3', '', title));
      if (!items.length) s.append(el('p', 'docs-home-empty', tx('Encara no hi ha cap document.', 'No documents yet.')));
      const newest = [...items].sort((a, b) => Date.parse(b.updated) - Date.parse(a.updated));
      for (const d of newest) {
        const b = el('button', 'docs-home-item');
        b.type = 'button';
        b.dataset.id = d.id;
        const k = KINDS[d.kind];
        // La icona del tipus la pinta el CSS del panell (docs-panel.js).
        b.append(el('span', 'docs-kind docs-kind-' + d.kind), el('span', 'docs-home-title', d.title), el('span', 'docs-home-meta', (k ? tx(k[0], k[1]) : d.kind) + ' · ' + when(d.updated)));
        b.onclick = () => act.open(d.id, scope);
        s.append(b);
      }
      return s;
    };
    const form = el('form', 'docs-home-new');
    const title = el('input');
    title.placeholder = tx('Títol del document', 'Document title');
    title.setAttribute('aria-label', tx('Títol del document', 'Document title'));
    const create = el('button', 'docs-kind-doc', tx('Document nou', 'New document'));
    create.type = 'submit';
    const deck = el('button', 'docs-kind-deck', tx('Presentació nova', 'New presentation'));
    deck.type = 'button';
    deck.onclick = () => act.create(title.value.trim() || tx('Presentació sense títol', 'Untitled presentation'), 'deck');
    const sheet = el('button', 'docs-kind-sheet', tx('Full de càlcul nou', 'New spreadsheet'));
    sheet.type = 'button';
    sheet.onclick = () => act.create(title.value.trim() || tx('Full de càlcul sense títol', 'Untitled spreadsheet'), 'sheet');
    form.append(title, create, deck, sheet);
    form.onsubmit = e => {
      e.preventDefault();
      act.create(title.value.trim() || tx('Document sense títol', 'Untitled document'));
    };
    const parts = [el('h2', '', 'Gregal Docs'), form, section(tx('Els meus documents', 'My documents'), lists.mine, 'mine')];
    if (lists.project) parts.push(section(tx('Aquest projecte', 'This project') + ' · ' + lists.project.name, lists.project.items, ''));
    const page = el('div', 'docs-home');
    page.append(...parts);
    box.replaceChildren(page);
  }

  // newDoc crea un document buit a «Els meus documents», de text o una
  // presentació (kind 'deck') o un full de càlcul (kind 'sheet'), i l'obre per escriure-hi (open amb write).
  // api és window.gregal.api.
  async function newDoc(title, { api, open, error, kind = 'doc' }) {
    let r;
    try {
      r = await api('/api/docs?scope=mine', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ title, kind }) });
    } catch (e) {
      error(e.message || String(e));
      return;
    }
    const d = await r.json().catch(() => ({ error: r.statusText }));
    if (!r.ok) { error(d.error || String(r.status)); return; }
    open(d.id, 'mine', true);
  }

  return { home, newDoc };
});
