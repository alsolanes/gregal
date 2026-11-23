// Vista d'un full de càlcul viu (gdocs kind sheet): una pestanya per full
// (un bloc), una barra de fórmula i una graella amb els valors que calcula
// el servidor. Es tria una cel·la amb un clic o les fletxes; s'hi escriu a
// la barra, i Enter o Tab desen la cel·la (PUT {cells, base_version}) i
// baixen o van a la dreta; Esc ho desfà. Un 409 es queda amb el que la
// persona ha escrit, com a doc-view.js.
// UMD: window.gregalSheetView al navegador, require() als tests de Node.
(function (root, factory) {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.gregalSheetView = api;
})(typeof self !== 'undefined' ? self : this, function () {
  'use strict';

  const ROW_H = 24; // px, la mateixa alçada que .sheet-grid td al CSS del panell
  const BIG = 300; // amb més files, només es pinten les que es veuen

  // column(0) és A, column(26) és AA.
  const column = i => (i >= 26 ? column(Math.floor(i / 26) - 1) : '') + String.fromCharCode(65 + (i % 26));
  const parse = ref => {
    const m = /^([A-Z]+)(\d+)$/.exec(ref);
    if (!m) return null;
    let c = 0;
    for (const ch of m[1]) c = c * 26 + ch.charCodeAt(0) - 64;
    return { c: c - 1, r: Number(m[2]) };
  };

  // view pinta el full de càlcul doc dins box. edit desa com a deck-view.js:
  // put(sheet, {cells}, base) i post(after, {name}) tornen {status, body};
  // tx(ca, en) tria l'idioma i error(msg) diu els errors que no són un 409.
  function view(box, doc, render, edit) {
    const D = box.ownerDocument, tx = edit.tx;
    const el = (tag, cls, text) => {
      const e = D.createElement(tag);
      if (cls) e.className = cls;
      if (text !== undefined) e.textContent = text;
      return e;
    };
    const sheets = new Map(); // id → bloc del full, l'últim que se'n sap
    let order = [];
    for (const b of doc.blocks) { sheets.set(b.id, b); order.push(b.id); }
    const seen = doc.version;
    let version = doc.version, current = order[0] || '';
    // sel és la cel·la triada; base, la versió del full que es veia en
    // triar-la; typed, si la persona ja hi ha escrit.
    let sel = { c: 0, r: 1 }, base = 0, typed = false, saving = false, big = false, selTd = null;

    const tabs = el('div', 'sheet-tabs');
    const ref = el('span', 'sheet-ref');
    const bar = el('input', 'sheet-formula');
    bar.setAttribute('aria-label', tx('Contingut de la cel·la', 'Cell content'));
    const head = el('div', 'sheet-bar');
    head.append(ref, bar);
    const notice = el('div', 'doc-conflict');
    notice.hidden = true;
    const wrap = el('div', 'sheet-grid');
    wrap.tabIndex = 0;
    box.replaceChildren(tabs, head, notice, wrap);

    const name = p => column(p.c) + p.r;
    const sheet = () => sheets.get(current) || { cells: {}, values: {} };
    const raw = () => sheet().cells?.[name(sel)] || '';

    function paintTabs() {
      const bs = order.map(id => {
        const b = el('button', 'sheet-tab' + (id === current ? ' on' : ''), sheets.get(id).name);
        b.type = 'button';
        b.onclick = () => show(id);
        return b;
      });
      const add = el('button', 'sheet-add', tx('Full nou', 'New sheet'));
      add.type = 'button';
      add.onclick = addSheet;
      tabs.replaceChildren(...bs, add);
    }

    // paint dibuixa la graella: una columna i cinc files més enllà de
    // l'última cel·la amb dades (o la triada), almenys 6 × 20.
    function paint() {
      const s = sheet();
      let cols = 6, rows = 20;
      for (const k of Object.keys(s.cells || {})) {
        const p = parse(k);
        if (p) { cols = Math.max(cols, p.c + 2); rows = Math.max(rows, p.r + 5); }
      }
      cols = Math.max(cols, sel.c + 1);
      rows = Math.max(rows, sel.r);
      let first = 1, last = rows;
      big = rows > BIG;
      if (big) {
        first = Math.max(1, Math.floor(wrap.scrollTop / ROW_H) - 10);
        last = Math.min(rows, first + Math.ceil((wrap.clientHeight || 600) / ROW_H) + 20);
      }
      const tr = cells => { const r = el('tr'); r.append(...cells); return r; };
      const spacer = n => { const r = el('tr', 'sheet-spacer'); r.style.height = n * ROW_H + 'px'; return r; };
      const top = [el('th')];
      for (let c = 0; c < cols; c++) top.push(el('th', '', column(c)));
      const body = [];
      selTd = null;
      if (first > 1) body.push(spacer(first - 1));
      for (let r = first; r <= last; r++) {
        const row = [el('th', '', String(r))];
        for (let c = 0; c < cols; c++) {
          const k = column(c) + r, v = s.values?.[k] ?? s.cells?.[k] ?? '';
          const td = el('td', (r === sel.r && c === sel.c ? 'sel' : '') + (v !== '' && !isNaN(v) ? ' num' : ''), v);
          td.dataset.cell = k;
          if (r === sel.r && c === sel.c) selTd = td;
          td.onclick = () => select({ c, r });
          td.ondblclick = () => bar.focus();
          row.push(td);
        }
        body.push(tr(row));
      }
      if (last < rows) body.push(spacer(rows - last));
      const thead = el('thead'), tbody = el('tbody'), table = el('table');
      thead.append(tr(top));
      tbody.append(...body);
      table.append(thead, tbody);
      wrap.replaceChildren(table);
      wrap.dataset.rows = first + '-' + last;
      ref.textContent = name(sel);
    }

    function select(p) {
      sel = { c: Math.max(0, p.c), r: Math.max(1, p.r) };
      bar.value = raw();
      base = sheet().version;
      typed = false;
      notice.hidden = true;
      // La fila triada ha de quedar sota la capçalera fixa: en una graella
      // llarga, perquè es pinti; la columna la posa el navegador
      // (scroll-padding al CSS del panell, per la columna dels números).
      const top = (sel.r - 1) * ROW_H, h = (wrap.clientHeight || 600) - ROW_H;
      if (top < wrap.scrollTop) wrap.scrollTop = top;
      else if (top + ROW_H > wrap.scrollTop + h) wrap.scrollTop = top + ROW_H - h;
      paint();
      selTd?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    }

    function show(id) {
      current = id;
      paintTabs();
      select({ c: 0, r: 1 });
    }

    // commit desa la cel·la si ha canviat i, si s'ha desat, es mou (dc, dr).
    async function commit(dc, dr) {
      // Mentre es desa la persona pot canviar de full o de cel·la: la
      // resposta és del full id, i només es mou si encara és on era.
      const p = sel, id = current, input = bar.value;
      const here = () => current === id && sel === p;
      if (typed && (input !== raw() || !notice.hidden)) {
        if (saving) return;
        saving = true;
        let r;
        try {
          r = await edit.put(id, { cells: { [name(p)]: input } }, base);
        } catch (err) {
          // El que ha escrit es queda a la barra per tornar-ho a provar.
          edit.error(err.message || String(err));
          return;
        } finally {
          saving = false;
        }
        if (r.status === 409) {
          // Ara es veu el full d'avui; el pròxim Enter el trepitja.
          sheets.set(id, r.body.block);
          if (!here()) return;
          base = r.body.block.version;
          notice.hidden = false;
          notice.textContent = tx("L'agent ha canviat aquest full", 'The agent changed this sheet') + ' · ' + name(p) + ': ' + (raw() || '—');
          paint();
          return;
        }
        if (r.status !== 200) { edit.error(r.body.error || String(r.status)); return; }
        sheets.set(id, r.body.block);
        if (!here()) return;
      }
      select({ c: p.c + dc, r: p.r + dr });
      wrap.focus();
    }

    function cancel() {
      select(sel);
      wrap.focus();
    }

    const MOVES = { ArrowUp: [0, -1], ArrowDown: [0, 1], ArrowLeft: [-1, 0], ArrowRight: [1, 0] };
    wrap.onkeydown = e => {
      if (MOVES[e.key]) {
        e.preventDefault();
        select({ c: sel.c + MOVES[e.key][0], r: sel.r + MOVES[e.key][1] });
      } else if (e.key === 'Enter' || e.key === 'F2') {
        e.preventDefault();
        bar.focus();
      } else if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
        // Escriure sobre una cel·la la reemplaça, com a Excel.
        e.preventDefault();
        bar.value = e.key;
        typed = true;
        bar.focus();
      }
    };
    wrap.onscroll = () => { if (big) paint(); };
    bar.oninput = () => { typed = true; };
    bar.onkeydown = e => {
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault();
        return commit(e.key === 'Tab' ? 1 : 0, e.key === 'Enter' ? 1 : 0);
      }
      if (e.key === 'Escape') { e.preventDefault(); cancel(); }
    };

    // addSheet afegeix un full buit al final, amb el primer nom lliure.
    async function addSheet() {
      const used = new Set([...sheets.values()].map(b => b.name.toLowerCase()));
      let n = order.length + 1;
      while (used.has((tx('Full', 'Sheet') + n).toLowerCase())) n++;
      let r;
      try {
        r = await edit.post(order.at(-1) || '', { name: tx('Full', 'Sheet') + n });
      } catch (err) {
        edit.error(err.message || String(err));
        return;
      }
      if (r.status !== 200) { edit.error(r.body.error || String(r.status)); return; }
      for (const b of r.body.blocks) ctl.apply({ op: 'insert', block: b, after: order.at(-1), version: r.body.version });
      show(r.body.blocks.at(-1).id);
    }

    const ctl = {
      get version() { return version; },
      get selected() { return name(sel); },
      get sheet() { return current; },
      bar, wrap, show, commit, cancel,
      // apply aplica un event `block`: la cel·la triada i el que la
      // persona escriu a la barra no es mouen.
      apply(c) {
        if (c.version <= seen) return false;
        version = Math.max(version, c.version);
        const id = c.block.id;
        if (c.op === 'insert' && !sheets.has(id)) {
          const i = c.after ? order.indexOf(c.after) + 1 : 0;
          order.splice(i, 0, id);
          sheets.set(id, c.block);
        } else if (c.op === 'replace' && sheets.has(id)) {
          sheets.set(id, c.block);
          if (id === current && !typed) { bar.value = raw(); base = c.block.version; }
        } else if (c.op === 'delete' && sheets.has(id)) {
          sheets.delete(id);
          order = order.filter(x => x !== id);
          if (id === current) { show(order[0] || ''); return true; }
        }
        paintTabs();
        if (id === current || !current) paint();
        return true;
      },
    };
    paintTabs();
    select(sel);
    return ctl;
  }

  return { view, column };
});
