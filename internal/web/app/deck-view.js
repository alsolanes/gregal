// Vista d'una presentació viva (gdocs kind deck): una diapositiva 16:9 per
// bloc, pintada a partir dels seus camps, i els events `block` aplicats al
// seu lloc. Un clic en una diapositiva n'obre el formulari del layout; les
// regles d'edició i de conflicte són les de doc-view.js. La referència és el
// pptx exportat: aquí només cal reconèixer-hi el layout.
// UMD: window.gregalDeckView al navegador, require() als tests de Node.
(function (root, factory) {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.gregalDeckView = api;
})(typeof self !== 'undefined' ? self : this, function () {
  'use strict';

  const LAYOUTS = {
    title: ['Portada', 'Title'], section: ['Secció', 'Section'], content: ['Contingut', 'Content'],
    two_columns: ['Dues columnes', 'Two columns'], comparison: ['Comparació', 'Comparison'],
    big_number: ['Xifra gran', 'Big number'], image: ['Imatge', 'Image'], title_only: ['Només títol', 'Title only'],
    blank: ['En blanc', 'Blank'], closing: ['Tancament', 'Closing'],
  };
  const COLS = ['title', 'left_title', 'left', 'right_title', 'right'];
  // Els camps que el formulari ensenya per a cada layout, com els fa servir
  // el constructor (ooxml.buildSlide); les notes hi són sempre.
  const FIELDS = {
    title: ['title', 'subtitle'], section: ['title', 'subtitle'], closing: ['title', 'subtitle'],
    content: ['title', 'body', 'bullets'], two_columns: COLS, comparison: COLS,
    big_number: ['title', 'number', 'caption'], image: ['title', 'image', 'body'], title_only: ['title'], blank: [],
  };
  const LABELS = {
    title: ['Títol', 'Title'], subtitle: ['Subtítol', 'Subtitle'], body: ['Text', 'Text'],
    bullets: ['Punts, un per línia', 'Bullets, one per line'], left_title: ["Títol de l'esquerra", 'Left heading'],
    left: ['Columna esquerra', 'Left column'], right_title: ['Títol de la dreta', 'Right heading'],
    right: ['Columna dreta', 'Right column'], number: ['Xifra', 'Number'], caption: ['Peu', 'Caption'],
    image: ['URL de la imatge', 'Image URL'], notes: ['Notes del presentador', 'Speaker notes'],
  };
  const LONG = ['body', 'bullets', 'left', 'right', 'notes'];

  const esc = s => String(s ?? '').replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]);

  // slideHTML dibuixa una diapositiva; render passa markdown a HTML.
  function slideHTML(s, render) {
    const md = x => (x ? render(x) : '');
    const title = s.title ? '<h2>' + esc(s.title) + '</h2>' : '';
    const body = () => md(s.body) + (s.bullets?.length ? '<ul>' + s.bullets.map(b => '<li>' + esc(b.trim()) + '</li>').join('') + '</ul>' : '');
    const col = (h, x) => '<div>' + (h ? '<h3>' + esc(h) + '</h3>' : '') + md(x) + '</div>';
    let inner;
    switch (s.layout) {
      case 'title': case 'section': case 'closing':
        inner = title + (s.subtitle || s.body ? '<p>' + esc(s.subtitle || s.body) + '</p>' : '');
        break;
      case 'two_columns': case 'comparison':
        inner = title + '<div class="slide-cols">' + col(s.left_title, s.left) + col(s.right_title, s.right) + '</div>';
        break;
      case 'big_number':
        inner = title + '<div class="slide-number">' + esc(s.number) + '</div><p>' + esc(s.caption) + '</p>';
        break;
      case 'image': {
        // Una ruta del projecte no es pot carregar aquí: se'n diu el nom.
        const img = /^https?:\/\//.test(s.image || '') ? '<img alt="" src="' + esc(s.image) + '">' : '<p>' + esc(s.image) + '</p>';
        inner = title + '<div class="slide-cols"><div>' + body() + '</div><div>' + img + '</div></div>';
        break;
      }
      default:
        inner = title + body();
    }
    if (s.chart) inner += '<p class="slide-chart">' + esc(s.chart.title || s.chart.type) + '</p>';
    return '<div class="slide slide-' + esc(s.layout) + '">' + inner + '</div>';
  }

  // view pinta la presentació doc dins box. edit desa els canvis com a
  // doc-view.js: put(id, slide, base), post(after, slide) i del(id, base)
  // tornen {status, body}; tx(ca, en) tria l'idioma i error(msg) diu els
  // errors que no són un 409.
  function view(box, doc, render, edit) {
    const D = box.ownerDocument, tx = edit.tx;
    const el = (tag, cls, text) => {
      const e = D.createElement(tag);
      if (cls) e.className = cls;
      if (text !== undefined) e.textContent = text;
      return e;
    };
    const list = el('div', 'deck-slides');
    const els = new Map();
    const slides = new Map(); // id → {slide, version}: l'últim que se'n sap
    const base = doc.version;
    let version = doc.version, editing = null, current = '';
    const paint = (e, id) => { e.innerHTML = slideHTML(slides.get(id).slide, render); };
    const make = b => {
      const e = el('div', 'doc-block deck-slide');
      e.dataset.id = b.id;
      slides.set(b.id, { slide: b.slide || {}, version: b.version });
      paint(e, b.id);
      // El clic d'un botó del formulari (Cancel·la) arriba aquí quan ja s'ha
      // tancat: no l'ha de tornar a obrir. Un enllaç tampoc.
      e.onclick = ev => {
        if (editing?.el === e || ev.target?.closest?.('form, a')) return;
        if (!editing) return open(b.id);
        return leave().then(ok => { if (ok) open(b.id); });
      };
      els.set(b.id, e);
      return e;
    };
    const empty = el('p', 'deck-empty', tx('Encara no hi ha cap diapositiva.', 'No slides yet.'));
    let emptyShown = false;
    const syncEmpty = () => {
      if (!els.size === emptyShown) return;
      emptyShown = !els.size;
      if (emptyShown) list.append(empty);
      else empty.remove();
    };
    list.replaceChildren(...doc.blocks.map(make));
    syncEmpty();

    // La barra de «Diapositiva nova»: el layout es tria d'una llista.
    const bar = el('div', 'deck-add');
    const pick = el('select');
    pick.setAttribute('aria-label', tx('Layout de la diapositiva nova', 'Layout of the new slide'));
    for (const [k, [ca, en]] of Object.entries(LAYOUTS)) {
      const o = el('option', '', tx(ca, en));
      o.value = k;
      pick.append(o);
    }
    pick.value = 'content';
    const add = el('button', '', tx('Diapositiva nova', 'New slide'));
    add.type = 'button';
    add.onclick = () => insert(pick.value);
    bar.append(pick, add);
    box.replaceChildren(list, bar);

    // read és la diapositiva del formulari: els camps que no s'hi veuen
    // (un gràfic, una taula) es queden com eren.
    function read(me) {
      const s = { ...me.slide, layout: me.layout };
      for (const [k, f] of Object.entries(me.fields)) {
        const lines = f.value.split('\n').map(x => x.trimEnd()).filter(x => x.trim());
        const v = f.value.trim();
        if (k === 'bullets' ? lines.length : v) s[k] = k === 'bullets' ? lines : v;
        else delete s[k];
      }
      return s;
    }

    function open(id) {
      if (editing && !editing.saving) close(editing);
      current = id;
      const e = els.get(id), slide = slides.get(id).slide;
      const me = { id, el: e, slide, layout: slide.layout, base: slides.get(id).version, fields: {}, conflict: false, saving: false };
      const form = el('form', 'deck-form');
      const layout = el('select');
      for (const [k, [ca, en]] of Object.entries(LAYOUTS)) {
        const o = el('option', '', tx(ca, en));
        o.value = k;
        layout.append(o);
      }
      layout.value = slide.layout;
      const pickLabel = el('label', '', tx('Layout', 'Layout'));
      pickLabel.append(layout);
      const btn = (text, fn) => { const b = el('button', '', text); b.type = 'button'; b.onclick = fn; return b; };
      const row = el('div', 'deck-form-buttons');
      row.append(btn(tx('Desa', 'Save'), () => save(me)), btn(tx('Cancel·la', 'Cancel'), () => close(me)),
        btn(tx('Esborra la diapositiva', 'Delete slide'), () => remove(me)));
      form.append(pickLabel, row);
      // fill posa els camps del layout triat; el que s'hi ha escrit es
      // queda en els camps que el nou layout també té.
      const fill = () => {
        const old = me.fields;
        for (const f of Object.values(old)) f.parentNode.remove();
        me.fields = {};
        for (const k of [...(FIELDS[me.layout] || []), 'notes']) {
          const label = el('label', '', tx(...LABELS[k]));
          const f = el(LONG.includes(k) ? 'textarea' : 'input');
          f.value = old[k] ? old[k].value : k === 'bullets' ? (me.slide.bullets || []).join('\n') : me.slide[k] || '';
          if (LONG.includes(k)) f.rows = f.value.split('\n').length + 1;
          label.append(f);
          form.insertBefore(label, row);
          me.fields[k] = f;
        }
      };
      layout.onchange = () => { me.layout = layout.value; fill(); };
      fill();
      form.onsubmit = ev => { ev.preventDefault(); save(me); };
      me.form = form;
      me.opened = JSON.stringify(read(me));
      e.innerHTML = '';
      e.append(form);
      editing = me;
      Object.values(me.fields)[0]?.focus();
      return me;
    }

    function close(me) {
      if (editing === me) editing = null;
      if (slides.has(me.id)) paint(me.el, me.id);
      else { me.el.remove(); els.delete(me.id); }
      syncEmpty();
    }

    // send fa una crida de l'editor; done rep la resposta d'un 200, i
    // failed, si hi és, encapçala els errors.
    async function send(me, call, done, failed) {
      if (me.saving) return;
      me.saving = true;
      const fail = m => edit.error(failed ? failed + ': ' + m : m);
      let r;
      try {
        r = await call();
      } catch (err) {
        // El formulari es queda com està per tornar-ho a provar.
        fail(err.message || String(err));
        return;
      } finally {
        me.saving = false;
      }
      if (r.status === 409) { conflict(me, r.body.block); return; }
      if (r.status !== 200) { fail(r.body.error || String(r.status)); return; }
      done(r.body);
      close(me);
    }

    function save(me) {
      const s = read(me);
      // Sense tocar no es desa: el que s'ha obert pot ser més vell que el de l'agent.
      if (!me.conflict && JSON.stringify(s) === me.opened) { close(me); return Promise.resolve(); }
      return send(me, () => edit.put(me.id, s, me.base), b => slides.set(me.id, { slide: b.block.slide, version: b.block.version }));
    }

    function remove(me) {
      return send(me, () => edit.del(me.id, me.base), () => { slides.delete(me.id); if (current === me.id) current = ''; },
        tx("No s'ha pogut esborrar la diapositiva", 'Could not delete the slide'));
    }

    // leave deixa el formulari obert per anar a una altra diapositiva: el
    // que s'hi ha escrit es desa abans. Després d'un 409 s'hi queda, perquè
    // la persona triï entre desar-lo (trepitja l'agent) i cancel·lar-lo.
    async function leave() {
      const me = editing;
      if (!me || me.saving) return true;
      if (me.conflict) { Object.values(me.fields)[0]?.focus(); return false; }
      await save(me);
      return editing !== me;
    }

    // conflict ensenya la diapositiva d'ara a sobre del formulari, que es
    // queda amb el que la persona hi ha escrit; desar-lo la trepitja.
    function conflict(me, b) {
      slides.set(me.id, { slide: b.slide, version: b.version });
      me.slide = b.slide;
      me.base = b.version;
      me.conflict = true;
      me.notice ||= el('div', 'doc-conflict');
      me.notice.innerHTML = '<p>' + esc(tx("L'agent ha canviat aquesta diapositiva", 'The agent changed this slide')) + '</p>' + slideHTML(b.slide, render);
      me.form.insertBefore(me.notice, me.form.firstChild);
    }

    // insert posa una diapositiva buida del layout després de l'actual (la
    // darrera oberta, o la darrera de totes) i n'obre el formulari.
    async function insert(layout) {
      if (!(await leave())) return;
      const after = els.has(current) ? current : [...els.keys()].at(-1) || '';
      let r;
      try {
        r = await edit.post(after, { layout });
      } catch (err) {
        edit.error(err.message || String(err));
        return;
      }
      if (r.status !== 200) { edit.error(r.body.error || String(r.status)); return; }
      let at = after;
      for (const b of r.body.blocks) {
        ctl.apply({ op: 'insert', block: b, after: at, version: r.body.version });
        at = b.id;
      }
      open(at);
    }

    const ctl = {
      get version() { return version; },
      get editing() { return editing; },
      open,
      save: () => editing && save(editing),
      cancel: () => editing && close(editing),
      // apply aplica un event `block` com doc-view.js: la diapositiva que la
      // persona edita no es repinta, el formulari es queda com està.
      apply(c) {
        if (c.version <= base) return false;
        version = Math.max(version, c.version);
        const id = c.block.id, e = els.get(id);
        const mine = editing?.id === id;
        if (c.op === 'insert' && !e) {
          const ref = c.after ? els.get(c.after)?.nextSibling : list.firstChild;
          list.insertBefore(make(c.block), ref || null);
        } else if (c.op === 'replace' && e) {
          slides.set(id, { slide: c.block.slide || {}, version: c.block.version });
          if (!mine) paint(e, id);
        } else if (c.op === 'delete' && e) {
          slides.delete(id);
          if (!mine) { e.remove(); els.delete(id); }
        }
        syncEmpty();
        return true;
      },
    };
    return ctl;
  }

  return { view, slideHTML };
});
