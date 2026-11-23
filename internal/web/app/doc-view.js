// Vista d'un document viu (internal/gdocs): un element per bloc, pintat amb
// md.js, i els events `block` de /api/docs/{id}/stream aplicats al seu lloc
// sense tornar a pintar la resta. Un clic en un bloc l'obre per editar-ne el
// markdown. El panell lateral (docs-panel.js) la fa servir; és a part
// perquè la finestra pròpia la pugui reusar.
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

  // Què vol dir una tecla dins l'editor d'un bloc. Enter amb el cursor al
  // final del text obre un bloc nou a sota; Maj+Enter és un salt de línia.
  function keyAction(e, ta) {
    if (e.key === 'Escape') return 'cancel';
    if (e.key !== 'Enter' || e.shiftKey) return null;
    if (e.ctrlKey || e.metaKey) return 'save';
    const end = ta.value.length;
    return ta.selectionStart === end && ta.selectionEnd === end ? 'insert' : null;
  }

  // view pinta doc dins box; render passa markdown a HTML (gregalMD.render).
  // edit desa els canvis de la persona: put(id, md, base), post(after, md) i
  // del(id, base) tornen {status, body} de les rutes de blocs; notice() és
  // l'avís d'un 409, empty() el text del bloc buit d'un document sense blocs
  // i error(msg) diu la resta d'errors.
  function view(box, doc, render, edit) {
    const D = box.ownerDocument;
    const els = new Map();
    const blocks = new Map(); // id → {md, version}: l'últim que se'n sap
    const base = doc.version;
    let version = doc.version;
    let editing = null;
    const paint = (el, id) => { el.innerHTML = render(blocks.get(id).md); };
    const make = b => {
      const el = D.createElement('div');
      el.className = 'doc-block';
      el.dataset.id = b.id;
      blocks.set(b.id, { md: b.md, version: b.version });
      paint(el, b.id);
      el.onclick = e => {
        if (editing?.el !== el && !e.target?.closest?.('a')) open(b.id);
      };
      els.set(b.id, el);
      return el;
    };
    box.replaceChildren(...doc.blocks.map(make));
    // Un document sense blocs també s'ha de poder escriure: un bloc buit on
    // clicar obre l'editor d'un bloc nou al principi. Una presentació no
    // s'edita en markdown.
    const writable = !!edit && doc.kind !== 'deck';
    let empty = null;
    function syncEmpty() {
      const want = writable && !els.size && !(editing && !editing.id);
      if (want && !empty) {
        empty = D.createElement('div');
        empty.className = 'doc-block doc-empty';
        empty.innerHTML = '<p>' + edit.empty() + '</p>';
        empty.onclick = () => open(null, '');
        box.append(empty);
      } else if (!want && empty) {
        empty.remove();
        empty = null;
      }
    }
    syncEmpty();

    // open posa un editor al bloc id, o un de bloc nou després d'after (buit:
    // al principi).
    function open(id, after) {
      if (editing && !editing.saving) close(editing);
      let el = els.get(id);
      if (!id) {
        el = D.createElement('div');
        el.className = 'doc-block';
        box.insertBefore(el, after ? els.get(after).nextSibling || null : box.firstChild);
      }
      const ta = D.createElement('textarea');
      ta.value = id ? blocks.get(id).md : '';
      ta.rows = ta.value.split('\n').length + 1;
      const me = { id, after, el, ta, opened: ta.value.trim(), base: id ? blocks.get(id).version : 0, conflict: false, saving: false };
      ta.onkeydown = e => {
        const a = keyAction(e, ta);
        if (!a) return;
        e.preventDefault();
        if (a === 'cancel') close(me);
        else if (a === 'save') save(me);
        else insertBelow(me);
      };
      // Després d'un 409 sortir no desa: només Ctrl+Enter trepitja l'altre.
      ta.onblur = () => { if (editing === me && !me.conflict) save(me); };
      el.innerHTML = '';
      el.append(ta);
      editing = me;
      syncEmpty();
      ta.focus();
      return me;
    }

    function close(me) {
      if (editing === me) editing = null;
      if (!me.id) me.el.remove();
      else if (blocks.has(me.id)) paint(me.el, me.id);
      else { me.el.remove(); els.delete(me.id); }
      syncEmpty();
    }

    // save desa l'editor i torna l'id del bloc després del qual aniria un de
    // nou, o null si no hi ha bloc (esborrat, buit o error).
    async function save(me) {
      if (me.saving) return null;
      const md = me.ta.value.trim();
      if (!me.id && !md) { close(me); return null; }
      // Sense tocar no es desa: el text obert pot ser més vell que el de l'agent.
      if (me.id && !me.conflict && md === me.opened) { close(me); return me.id; }
      me.saving = true;
      let r;
      try {
        if (!me.id) r = await edit.post(me.after, md);
        else if (md) r = await edit.put(me.id, md, me.base);
        else r = await edit.del(me.id, me.base);
      } catch (e) {
        // El text es queda a l'editor per tornar-ho a provar.
        edit.error(e.message || String(e));
        return null;
      } finally {
        me.saving = false;
      }
      if (r.status === 409) { conflict(me, r.body.block); return null; }
      if (r.status !== 200) { edit.error(r.body.error || String(r.status)); return null; }
      let at = me.id;
      if (!me.id) {
        at = me.after;
        for (const b of r.body.blocks) {
          ctl.apply({ op: 'insert', block: b, after: at, version: r.body.version });
          at = b.id;
        }
      } else if (md) {
        blocks.set(me.id, { md: r.body.block.md, version: r.body.block.version });
      } else {
        blocks.delete(me.id);
        at = null;
      }
      close(me);
      return at;
    }

    // conflict ensenya el text d'ara a sobre del de la persona, que es
    // queda a l'editor per copiar-lo o desar-lo igualment.
    function conflict(me, b) {
      blocks.set(me.id, { md: b.md, version: b.version });
      me.base = b.version;
      me.conflict = true;
      me.notice ||= D.createElement('div');
      me.notice.className = 'doc-conflict';
      me.notice.innerHTML = '<p>' + edit.notice() + '</p>' + render(b.md);
      me.el.insertBefore(me.notice, me.ta);
    }

    async function insertBelow(me) {
      const at = await save(me);
      if (at && els.has(at)) open(null, at);
    }

    const ctl = {
      get version() { return version; },
      get editing() { return editing; },
      open,
      save: () => editing && save(editing),
      cancel: () => editing && close(editing),
      insertBelow: () => editing && insertBelow(editing),
      // apply aplica un event `block`; els que el document llegit ja tenia
      // (la mateixa versió o una de més vella) no compten. El bloc que la
      // persona edita no es repinta: el seu text queda a l'editor.
      apply(c) {
        if (c.version <= base) return false;
        version = Math.max(version, c.version);
        const id = c.block.id, el = els.get(id);
        const mine = editing?.id === id;
        if (c.op === 'insert' && !el) {
          const ref = c.after ? els.get(c.after)?.nextSibling : box.firstChild;
          box.insertBefore(make(c.block), ref || null);
        } else if (c.op === 'replace' && el) {
          blocks.set(id, { md: c.block.md, version: c.block.version });
          if (!mine) paint(el, id);
        } else if (c.op === 'delete' && el) {
          blocks.delete(id);
          if (!mine) { el.remove(); els.delete(id); }
        }
        syncEmpty();
        return true;
      },
    };
    return ctl;
  }

  return { view, onDocChanged, keyAction };
});
