// Vista d'un document viu (internal/gdocs): un element per bloc, pintat amb
// md.js, i els events `block` de /api/docs/{id}/stream aplicats al seu lloc
// sense tornar a pintar la resta. El panell lateral (docs-panel.js) la fa
// servir; és a part perquè l'editor i la finestra pròpia la puguin reusar.
// UMD: window.gregalDocView al navegador, require() als tests de Node.
(function (root, factory) {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.gregalDocView = api;
})(typeof self !== 'undefined' ? self : this, function () {
  'use strict';

  // Què fa el panell amb un doc_changed: obrir el document, o només portar-lo
  // al davant si ja és el que s'hi veu.
  function onDocChanged(state, ev) {
    return state.visible && state.id === ev.id ? 'front' : 'open';
  }

  // view pinta doc dins box; render passa markdown a HTML (gregalMD.render).
  function view(box, doc, render) {
    const els = new Map();
    const base = doc.version;
    let version = doc.version;
    const make = b => {
      const el = box.ownerDocument.createElement('div');
      el.className = 'doc-block';
      el.dataset.id = b.id;
      el.innerHTML = render(b.md);
      els.set(b.id, el);
      return el;
    };
    box.replaceChildren(...doc.blocks.map(make));
    return {
      get version() { return version; },
      // apply aplica un event `block`; els que el document llegit ja tenia
      // (la mateixa versió o una de més vella) no compten.
      apply(c) {
        if (c.version <= base) return false;
        version = c.version;
        const el = els.get(c.block.id);
        if (c.op === 'insert') {
          const ref = c.after ? els.get(c.after)?.nextSibling : box.firstChild;
          box.insertBefore(make(c.block), ref || null);
        } else if (c.op === 'replace' && el) {
          el.innerHTML = render(c.block.md);
        } else if (c.op === 'delete' && el) {
          el.remove();
          els.delete(c.block.id);
        }
        return true;
      },
    };
  }

  return { view, onDocChanged };
});
