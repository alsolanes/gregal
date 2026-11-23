// Editor d'un document viu de text (internal/gdocs): un sol editor WYSIWYG
// (ProseMirror, vendor/prosemirror.min.js) on cada node de primer nivell és
// un bloc. Cada node es llegeix del markdown del seu bloc i s'hi torna a
// escriure, i els canvis es desen bloc a bloc amb base_version; els events
// `block` de /api/docs/{id}/stream s'apliquen al seu node. El panell lateral
// (docs-panel.js) i la finestra pròpia (doc-page.js) la fan servir.
// UMD: window.gregalDocView al navegador, require() als tests de Node.
(function (root, factory) {
  const api = factory(root.gregalPM || require('./vendor/prosemirror.min.js'));
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.gregalDocView = api;
})(typeof self !== 'undefined' ? self : this, function (pm) {
  'use strict';

  // Què fa el panell amb un doc_changed: obrir el document, o només portar-lo
  // al davant si ja és el que s'hi veu (mateix id i mateixa carpeta, scope).
  function onDocChanged(state, ev) {
    return state.visible && state.id === ev.id && (state.scope || '') === (ev.scope || '') ? 'front' : 'open';
  }

  // L'esquema de prosemirror-markdown amb l'id del bloc als nodes que poden
  // anar a primer nivell, l'alineació dels paràgrafs i títols, el subratllat
  // (++u++, 078), el ratllat (~~s~~) i l'estil [text]{color=… bg=… font="…" size=N} (078).
  let nodes = pm.markdownSchema.spec.nodes;
  for (const n of ['paragraph', 'blockquote', 'horizontal_rule', 'heading', 'code_block', 'ordered_list', 'bullet_list']) {
    const s = nodes.get(n);
    nodes = nodes.update(n, { ...s, attrs: { ...s.attrs, id: { default: null } } });
  }
  for (const n of ['paragraph', 'heading']) {
    const s = nodes.get(n);
    nodes = nodes.update(n, {
      ...s, attrs: { ...s.attrs, align: { default: null } },
      toDOM: node => { const d = s.toDOM(node); return node.attrs.align ? [d[0], { style: 'text-align:' + node.attrs.align }, ...d.slice(1)] : d; },
    });
  }
  const SPAN = 'color bg font size'.split(' ');
  const CSS = { color: v => 'color:' + v, bg: v => 'background-color:' + v, font: v => 'font-family:"' + v + '"', size: v => 'font-size:' + v + 'pt' };
  // Una taula de markdown (081): la primera fila és la capçalera, i les
  // cel·les només tenen text, com a markdown.
  const cellAlign = el => ({ align: el.style?.textAlign || null });
  nodes = nodes
    .addToEnd('table', { content: 'table_row+', group: 'block', isolating: true, attrs: { id: { default: null } },
      parseDOM: [{ tag: 'table' }], toDOM: () => ['table', ['tbody', 0]] })
    .addToEnd('table_row', { content: 'table_cell+', parseDOM: [{ tag: 'tr' }], toDOM: () => ['tr', 0] })
    .addToEnd('table_cell', { content: 'inline*', isolating: true, attrs: { align: { default: null } },
      parseDOM: [{ tag: 'td', getAttrs: cellAlign }, { tag: 'th', getAttrs: cellAlign }],
      toDOM: n => ['td', n.attrs.align ? { style: 'text-align:' + n.attrs.align } : {}, 0] });
  const schema = new pm.Schema({
    nodes,
    marks: pm.markdownSchema.spec.marks
      // Primer, perquè l'estil quedi per fora de la negreta, la cursiva o l'enllaç.
      .addToStart('span', {
        attrs: { color: { default: null }, bg: { default: null }, font: { default: null }, size: { default: null } },
        toDOM: m => ['span', { style: SPAN.filter(k => m.attrs[k]).map(k => CSS[k](m.attrs[k])).join(';') }, 0],
      })
      .addToEnd('underline', { parseDOM: [{ tag: 'u' }], toDOM: () => ['u', 0] })
      .addToEnd('strike', { parseDOM: [{ tag: 's' }, { tag: 'del' }], toDOM: () => ['s', 0] }),
  });

  // spanAttrs llegeix els atributs d'un [text]{…} amb les mateixes regles que
  // internal/ooxml: cada clau un cop, i res que no sigui vàlid.
  const ATTR = /^([a-z]+)=("[^"\n]*"|[^\s"]*)(\s+|$)/;
  function spanAttrs(s) {
    s = s.trim();
    if (!s) return null;
    const a = {};
    for (let m; s; s = s.slice(m[0].length)) {
      m = ATTR.exec(s);
      if (!m || m[1] in a) return null;
      const [, k, v] = m;
      if ((k === 'color' || k === 'bg') && /^#[0-9a-fA-F]{6}$/.test(v)) a[k] = v;
      else if (k === 'font' && v.length > 2 && v[0] === '"') a[k] = v.slice(1, -1);
      else if (k === 'size' && /^\d+(\.\d+)?$/.test(v) && +v > 0 && +v <= 1638) a[k] = +v;
      else return null;
    }
    return a;
  }
  // extended afegeix a markdown-it els [text]{…} i la línia {align=…} just
  // després d'un paràgraf o d'un títol de primer nivell.
  const ALIGN = /^\{align=(left|center|right|justify)\}$/;
  function extended(md) {
    md.inline.ruler.before('link', 'span', (state, silent) => {
      if (state.src.charCodeAt(state.pos) !== 0x5B) return false;
      const end = state.md.helpers.parseLinkLabel(state, state.pos, false);
      const m = end >= 0 && /^\]\{([^{}\n]*)\}/.exec(state.src.slice(end));
      const own = m && spanAttrs(m[1]);
      if (!own) return false;
      if (!silent) {
        // Un span dins d'un altre en pren el que no diu, com fa l'exportació.
        const outer = state.env.spans || (state.env.spans = []), attrs = { ...outer[outer.length - 1], ...own };
        state.push('span_open', 'span', 1).meta = attrs;
        outer.push(attrs);
        const max = state.posMax;
        state.pos++;
        state.posMax = end;
        state.md.inline.tokenize(state);
        state.posMax = max;
        outer.pop();
        state.push('span_close', 'span', -1);
        if (outer.length) state.push('span_open', 'span', 1).meta = outer[outer.length - 1];
      }
      state.pos = end + m[0].length;
      return true;
    });
    md.core.ruler.before('inline', 'align', state => {
      const lines = state.src.split('\n'), t = state.tokens;
      for (let i = 0; i < t.length; i++) {
        const o = t[i];
        if (o.level !== 0) continue;
        if (o.type === 'paragraph_open' && o.map[1] - o.map[0] > 1) {
          const m = ALIGN.exec(lines[o.map[1] - 1]);
          if (m) { o.attrSet('align', m[1]); t[i + 1].content = t[i + 1].content.replace(/\n[^\n]*$/, ''); }
        } else if (o.type === 'heading_open' && t[i + 3]?.type === 'paragraph_open' && t[i + 3].map[0] === o.map[1] && t[i + 3].map[1] === o.map[1] + 1) {
          const m = ALIGN.exec(lines[o.map[1]]);
          if (m) { o.attrSet('align', m[1]); t.splice(i + 3, 3); }
        }
      }
    });
  }

  const cell = { block: 'table_cell', getAttrs: t => ({ align: /text-align:(\w+)/.exec(t.attrGet('style') || '')?.[1] || null }) };
  const parser = new pm.MarkdownParser(schema, pm.MarkdownIt('commonmark', { html: false }).enable(['strikethrough', 'table']).use(pm.ins).use(extended), {
    ...pm.defaultMarkdownParser.tokens, s: { mark: 'strike' }, ins: { mark: 'underline' },
    paragraph: { block: 'paragraph', getAttrs: t => ({ align: t.attrGet('align') }) },
    heading: { block: 'heading', getAttrs: t => ({ level: +t.tag.slice(1), align: t.attrGet('align') }) },
    span: { mark: 'span', getAttrs: t => t.meta },
    table: { block: 'table' }, thead: { ignore: true }, tbody: { ignore: true }, tr: { block: 'table_row' }, th: cell, td: cell,
  });
  const D = pm.defaultMarkdownSerializer;
  const wrap = m => ({ open: m, close: m, mixable: true, expelEnclosingWhitespace: true });
  const DELIM = { left: ':---', center: ':---:', right: '---:' };
  // Una cel·la s'escriu com un paràgraf d'una sola línia, amb les | escapades.
  const cellMd = c => toMd([schema.nodes.paragraph.create(null, c.content)]).replace(/\|/g, '\\|').replace(/\n/g, ' ');
  function tableMd(s, n) {
    const rows = [];
    n.forEach(r => { const cells = []; r.forEach(c => cells.push(cellMd(c))); rows.push(cells); });
    const delim = [];
    n.firstChild.forEach(c => delim.push(DELIM[c.attrs.align] || '---'));
    for (const cells of [rows[0], delim, ...rows.slice(1)]) { s.write('| ' + cells.join(' | ') + ' |'); s.ensureNewLine(); }
    s.closeBlock(n);
  }
  // La línia {align=…} només a primer nivell: dins d'una llista o d'una cita
  // aniria sagnada i l'exportació no la llegiria.
  const alignLine = (s, n) => { if (n.attrs.align && !s.delim) { s.ensureNewLine(); s.write('{align=' + n.attrs.align + '}'); } };
  const serializer = new pm.MarkdownSerializer({
    ...D.nodes, bullet_list: (s, n) => s.renderList(n, '  ', () => '- '), table: tableMd,
    paragraph: (s, n) => { s.renderInline(n); alignLine(s, n); s.closeBlock(n); },
    heading: (s, n) => { s.write('#'.repeat(n.attrs.level) + ' '); s.renderInline(n, false); alignLine(s, n); s.closeBlock(n); },
  }, {
    ...D.marks, underline: wrap('++'), strike: wrap('~~'),
    span: {
      open: '[', mixable: true, expelEnclosingWhitespace: true,
      close: (s, m) => ']{' + SPAN.filter(k => m.attrs[k]).map(k => k + '=' + (k === 'font' ? '"' + m.attrs.font + '"' : m.attrs[k])).join(' ') + '}',
    },
  });

  const fromMd = (md, id) => parser.parse(md).content.content.map(n => n.type.create({ ...n.attrs, id }, n.content, n.marks));
  const toMd = list => serializer.serialize(schema.topNodeType.create(null, list)).trim();
  const toDoc = blocks => schema.topNodeType.createAndFill(null, blocks.flatMap(b => fromMd(b.md, b.id)));

  // runs agrupa els nodes de primer nivell seguits amb el mateix id: un bloc
  // del magatzem pot fer més d'un node (un paràgraf i una llista sense línia en blanc).
  function runs(doc) {
    const out = [];
    doc.forEach((n, pos) => {
      const last = out[out.length - 1];
      if (n.attrs.id && last?.id === n.attrs.id) { last.nodes.push(n); last.to += n.nodeSize; }
      else out.push({ id: n.attrs.id, nodes: [n], from: pos, to: pos + n.nodeSize });
    });
    return out;
  }

  const S = schema.nodes, M = schema.marks;
  const commands = {
    paragraph: pm.setBlockType(S.paragraph),
    h1: pm.setBlockType(S.heading, { level: 1 }),
    h2: pm.setBlockType(S.heading, { level: 2 }),
    h3: pm.setBlockType(S.heading, { level: 3 }),
    bold: pm.toggleMark(M.strong),
    italic: pm.toggleMark(M.em),
    underline: pm.toggleMark(M.underline),
    strike: pm.toggleMark(M.strike),
    code: pm.toggleMark(M.code),
    bullets: pm.wrapInList(S.bullet_list),
    numbers: pm.wrapInList(S.ordered_list),
    quote: pm.wrapIn(S.blockquote),
    rule: (st, d) => { if (d) d(st.tr.replaceSelectionWith(S.horizontal_rule.create())); return true; },
    undo: pm.undo,
    redo: pm.redo,
  };
  const link = href => pm.toggleMark(M.link, { href });
  // style posa (o treu, amb un valor buit) una clau de l'estil a la selecció
  // i deixa les altres claus de cada tros com eren.
  const style = (key, value) => (st, d) => {
    const merge = m => {
      const a = { ...m?.attrs, [key]: value || null };
      return SPAN.some(k => a[k]) ? M.span.create(a) : null;
    };
    const { from, to, empty, $from } = st.selection;
    if (!d) return true;
    if (empty) {
      const marks = st.storedMarks || $from.marks(), m = merge(M.span.isInSet(marks)), rest = M.span.removeFromSet(marks);
      d(st.tr.setStoredMarks(m ? m.addToSet(rest) : rest));
      return true;
    }
    const tr = st.tr;
    st.doc.nodesBetween(from, to, (n, pos) => {
      if (!n.isText) return;
      const a = Math.max(pos, from), b = Math.min(pos + n.nodeSize, to), m = merge(M.span.isInSet(n.marks));
      tr.removeMark(a, b, M.span);
      if (m) tr.addMark(a, b, m);
    });
    d(tr);
    return true;
  };
  // align alinea els paràgrafs i títols de primer nivell de la selecció;
  // a l'esquerra és el valor per defecte i no escriu cap línia.
  const align = value => (st, d) => {
    const tr = st.tr, { from, to } = st.selection;
    st.doc.nodesBetween(from, to, (n, pos) => {
      if (n.type.spec.attrs?.align) tr.setNodeAttribute(pos, 'align', value === 'left' ? null : value);
      return false;
    });
    if (!tr.docChanged) return false;
    if (d) d(tr);
    return true;
  };
  // current diu l'estil de la selecció per a la barra: el valor si tota la
  // selecció el comparteix, o '' si no en té o en barreja.
  function current(st) {
    const { from, to, empty, $from } = st.selection, spans = [];
    if (empty) spans.push(M.span.isInSet(st.storedMarks || $from.marks()));
    else st.doc.nodesBetween(from, to, n => { if (n.isText) spans.push(M.span.isInSet(n.marks)); });
    const out = { align: st.doc.maybeChild($from.index(0))?.attrs.align || 'left' };
    for (const k of SPAN) {
      const vs = new Set(spans.map(m => String(m?.attrs[k] || '')));
      out[k] = vs.size === 1 ? [...vs][0] : '';
    }
    return out;
  }

  // inTable diu on és el cursor dins d'una taula: la taula, on comença, la fila i la columna.
  function inTable(st) {
    const $h = st.selection.$head, d = $h.depth - 2;
    if ($h.parent.type !== S.table_cell) return null;
    return { table: $h.node(d), pos: $h.before(d), r: $h.index(d), c: $h.index(d + 1) };
  }
  // cellPos és la posició de l'inici del text de la cel·la r, c d'una taula que comença a pos.
  function cellPos(table, pos, r, c) {
    let at = pos + 1;
    for (let i = 0; i < r; i++) at += table.child(i).nodeSize;
    at += 1;
    for (let j = 0; j < c; j++) at += table.child(r).child(j).nodeSize;
    return at + 1;
  }
  // reshape refà la taula del cursor amb fn(files de cel·les, r, c), que
  // torna la cel·la on va el cursor. Sense files o columnes, la taula se'n va.
  const reshape = fn => (st, d) => {
    const at = inTable(st);
    if (!at) return false;
    if (!d) return true;
    const rows = [];
    at.table.forEach(row => { const cells = []; row.forEach(c => cells.push(c)); rows.push(cells); });
    const [r, c] = fn(rows, at.r, at.c);
    const tr = st.tr, end = at.pos + at.table.nodeSize;
    if (!rows.length || !rows[0].length) tr.delete(at.pos, end);
    else {
      const t = S.table.create(at.table.attrs, rows.map(cells => S.table_row.create(null, cells)));
      tr.replaceWith(at.pos, end, t).setSelection(pm.TextSelection.create(tr.doc, cellPos(t, at.pos, r, c)));
    }
    d(tr.scrollIntoView());
    return true;
  };
  const newRow = n => Array.from({ length: n }, () => S.table_cell.create());
  Object.assign(commands, {
    rowAdd: reshape((rows, r, c) => { rows.splice(r + 1, 0, newRow(rows[0].length)); return [r + 1, c]; }),
    rowDelete: reshape((rows, r, c) => { rows.splice(r, 1); return [Math.min(r, rows.length - 1), c]; }),
    colAdd: reshape((rows, r, c) => { for (const cells of rows) cells.splice(c + 1, 0, S.table_cell.create()); return [r, c + 1]; }),
    colDelete: reshape((rows, r, c) => { for (const cells of rows) cells.splice(c, 1); return [r, Math.min(c, rows[0].length - 1)]; }),
  });
  // Tab passa a la cel·la següent (i, després de l'última, fa una fila nova); Shift-Tab, a l'anterior.
  const rowAtEnd = reshape(rows => { rows.push(newRow(rows[0].length)); return [rows.length - 1, 0]; });
  const nextCell = dir => (st, d) => {
    const at = inTable(st);
    if (!at) return false;
    const cols = at.table.child(at.r).childCount, i = at.r * cols + at.c + dir;
    if (i >= at.table.childCount * cols) return rowAtEnd(st, d);
    if (d && i >= 0) d(st.tr.setSelection(pm.TextSelection.create(st.doc, cellPos(at.table, at.pos, Math.floor(i / cols), i % cols))));
    return true;
  };
  // Dins d'una cel·la, Enter no fa res, i esborrar a la vora no l'ajunta amb la del costat.
  const inCell = st => st.selection.$head.parent.type === S.table_cell;
  const cellEdge = back => st => {
    const { empty, $head } = st.selection;
    return empty && inCell(st) && $head.parentOffset === (back ? 0 : $head.parent.content.size);
  };

  // insertAfter posa node darrere del bloc del cursor, i el cursor a dins.
  const insertAfter = (node, inside) => (st, d) => {
    if (d) {
      const head = st.selection.$head.pos, at = runs(st.doc).find(r => head <= r.to)?.to ?? st.doc.content.size;
      const tr = st.tr.insert(at, node);
      d(tr.setSelection(pm.TextSelection.near(tr.doc.resolve(at + inside))).scrollIntoView());
    }
    return true;
  };
  const table = (rows, cols) => insertAfter(S.table.create(null, Array.from({ length: rows }, () => S.table_row.create(null, newRow(cols)))), 3);
  const image = (src, alt) => insertAfter(S.paragraph.create(null, S.image.create({ src, alt })), 1);
  // imageName torna el nom d'una imatge de la carpeta del document
  // (images/<nom>), o null: cap altra ruta no es mostra.
  const imageName = src => /^images\/([^/\\:]+)$/.exec(src)?.[1].replace(/^\.\.?$/, '') || null;
  // Enter al mig d'un bloc el parteix en dos: la part nova és un bloc nou, sense id.
  const split = pm.splitBlockAs((n, atEnd) => (atEnd ? null : { type: n.type, attrs: { ...n.attrs, id: null } }));
  const keys = {
    'Mod-b': commands.bold, 'Mod-i': commands.italic, 'Mod-u': commands.underline, 'Mod-`': commands.code,
    'Mod-Alt-0': commands.paragraph, 'Mod-Alt-1': commands.h1, 'Mod-Alt-2': commands.h2, 'Mod-Alt-3': commands.h3,
    'Mod-z': pm.undo, 'Mod-y': pm.redo, 'Shift-Mod-z': pm.redo,
    Enter: pm.chainCommands(inCell, pm.newlineInCode, pm.splitListItem(S.list_item), pm.createParagraphNear, pm.liftEmptyBlock, split),
    Tab: pm.chainCommands(nextCell(1), pm.sinkListItem(S.list_item)), 'Shift-Tab': pm.chainCommands(nextCell(-1), pm.liftListItem(S.list_item)),
    Backspace: cellEdge(true), Delete: cellEdge(false),
  };
  // Esborrar una selecció que va d'una cel·la a una altra les ajunta: les
  // files que queden curtes es tornen a omplir fins a l'amplada de la taula.
  const fillRows = new pm.Plugin({
    appendTransaction(trs, old, st) {
      if (!trs.some(t => t.docChanged)) return null;
      let tr = null;
      st.doc.descendants((n, pos) => {
        if (n.type !== S.table) return n.isBlock && !n.isTextblock;
        let width = 0;
        n.forEach(r => { width = Math.max(width, r.childCount); });
        n.forEach((r, off) => {
          if (r.childCount < width) { tr ||= st.tr; tr.insert(tr.mapping.map(pos + off + r.nodeSize), newRow(width - r.childCount)); }
        });
        return false;
      });
      return tr;
    },
  });

  const REMOTE = 'gregal-remote';
  // Un node que una ordre ha refet (paràgraf → títol, dins d'una llista o
  // d'una cita) perd l'id: si ocupa el lloc d'un bloc que ja no hi és, n'és el mateix bloc.
  const keepIds = new pm.Plugin({
    appendTransaction(trs, old, st) {
      if (!trs.some(t => t.docChanged) || trs.some(t => t.getMeta(REMOTE))) return null;
      const back = new pm.Mapping(trs.flatMap(t => t.mapping.maps)).invert();
      const have = new Set();
      st.doc.forEach(n => have.add(n.attrs.id));
      let tr = null;
      st.doc.forEach((n, pos) => {
        const was = !n.attrs.id && old.doc.maybeChild(old.doc.resolve(back.map(pos + 1)).index(0))?.attrs.id;
        if (was && !have.has(was)) { have.add(was); (tr ||= st.tr).setNodeAttribute(pos, 'id', was); }
      });
      return tr;
    },
  });

  // editor és el document sense DOM: l'estat, els desaments i els events.
  // edit = {put(id, md, base), post(after, md), del(id, base)} tornen
  // {status, body} de les rutes de blocs; error(msg) diu els errors.
  function editor(doc, edit) {
    const known = new Map(); // id → {md, version}: el que el magatzem en té
    for (const b of doc.blocks) known.set(b.id, { md: toMd(fromMd(b.md, b.id)), version: b.version });
    const conflicts = new Map(); // id → el bloc com és ara, després d'un 409
    const waiting = []; // events que esperen: un desament en curs o el bloc que s'edita
    const base = doc.version;
    let version = doc.version, timer = null, busy = false, temp = 0, focused = false;
    let state = pm.EditorState.create({ schema, doc: toDoc(doc.blocks), plugins: [pm.history(), pm.keymap(keys), pm.keymap(pm.baseKeymap), keepIds, fillRows] });

    // S'edita el bloc on és el cursor mentre l'editor té el focus.
    const editingId = () => focused && state.doc.maybeChild(state.selection.$head.index(0))?.attrs.id;
    const find = id => runs(state.doc).find(r => r.id === id);
    const dirty = id => { const r = find(id); return !!r && toMd(r.nodes) !== known.get(id)?.md; };
    const quiet = () => state.tr.setMeta(REMOTE, true).setMeta('addToHistory', false);
    function retag(from, to) {
      const tr = quiet();
      state.doc.forEach((n, pos) => { if (n.attrs.id === from) tr.setNodeAttribute(pos, 'id', to); });
      ctl.dispatch(tr);
    }

    // plan torna el primer desament que cal, en ordre del document.
    function plan() {
      const seen = new Set();
      let prev = '';
      for (const r of runs(state.doc)) {
        const md = toMd(r.nodes);
        if (r.id && !seen.has(r.id) && known.has(r.id)) {
          seen.add(r.id);
          if (md !== known.get(r.id).md && !conflicts.has(r.id)) return { id: r.id, md };
          prev = r.id;
        } else if (md) return { run: r, after: prev, md };
      }
      for (const id of known.keys()) if (!seen.has(id) && !conflicts.has(id)) return { id, md: '' };
      return null;
    }

    async function save(op) {
      let r;
      try {
        if (op.run) {
          const tmp = 'new:' + ++temp;
          const tr = quiet();
          state.doc.nodesBetween(op.run.from, op.run.to, (n, pos) => { tr.setNodeAttribute(pos, 'id', tmp); return false; });
          ctl.dispatch(tr);
          r = await edit.post(op.after, op.md);
          if (r.status === 200) {
            const b = r.body.blocks[0];
            known.set(b.id, { md: op.md, version: b.version });
            retag(tmp, b.id);
          } else retag(tmp, null);
        } else if (op.md) {
          r = await edit.put(op.id, op.md, known.get(op.id).version);
          if (r.status === 200) known.set(op.id, { md: op.md, version: r.body.block.version });
        } else {
          r = await edit.del(op.id, known.get(op.id).version);
          if (r.status === 200) { known.delete(op.id); if (find(op.id)) retag(op.id, null); }
        }
      } catch (e) {
        edit.error(e.message || String(e));
        return false;
      }
      if (r.status === 409) { conflicts.set(op.id, r.body.block); ctl.onConflict(); return true; }
      if (r.status !== 200) { edit.error(r.body.error || String(r.status)); return false; }
      version = Math.max(version, r.body.version || 0);
      return true;
    }

    async function sync() {
      clearTimeout(timer);
      if (busy) return;
      busy = true;
      try {
        for (let op; (op = plan());) if (!(await save(op))) break;
      } finally {
        busy = false;
        drain();
      }
    }

    // remote aplica un event al seu node amb una transacció que no va a
    // l'historial; el cursor de la persona es mou amb el text.
    function remote(c) {
      const id = c.block.id, r = find(id), tr = quiet();
      if (c.op === 'delete') {
        known.delete(id);
        if (r) tr.delete(r.from, r.to);
      } else {
        const list = fromMd(c.block.md, id);
        known.set(id, { md: toMd(list), version: c.block.version });
        if (r) tr.replaceWith(r.from, r.to, list);
        else tr.insert((c.after && find(c.after)?.to) || 0, list);
      }
      ctl.dispatch(tr);
    }

    // drain aplica els events que ja poden anar: no durant un desament, ni al
    // bloc on és el cursor o que té canvis per desar.
    function drain() {
      for (const c of waiting.splice(0)) {
        const id = c.block.id, k = known.get(id);
        if (c.op === 'insert' ? k : !k || c.block.version <= k.version) continue; // ja hi és: és el nostre
        if (conflicts.has(id)) { conflicts.set(id, c.block); ctl.onConflict(); continue; }
        if (busy || (c.op !== 'insert' && (editingId() === id || dirty(id)))) waiting.push(c);
        else remote(c);
      }
    }

    const ctl = {
      get state() { return state; },
      get version() { return version; },
      set focused(v) { focused = v; if (!v && !busy) drain(); },
      conflicts,
      onChange: () => {},
      onConflict: () => {},
      dispatch(tr) {
        state = state.apply(tr);
        ctl.onChange(state);
        if (tr.docChanged && !tr.getMeta(REMOTE)) { clearTimeout(timer); timer = setTimeout(sync, 700); }
        if (tr.selectionSet && waiting.length && !busy) drain();
      },
      run: (cmd, ...args) => cmd(state, ctl.dispatch, ...args),
      flush: sync,
      // apply rep un event `block`; els que el document llegit ja tenia no compten.
      apply(c) {
        if (c.version <= base) return false;
        version = Math.max(version, c.version);
        waiting.push(c);
        if (!busy) drain();
        return true;
      },
      // keepMine desa el text de la persona damunt de la versió que l'avís ensenya.
      keepMine(id) {
        const b = conflicts.get(id);
        conflicts.delete(id);
        if (known.has(id)) known.get(id).version = b.version;
        ctl.onConflict();
        return sync();
      },
      useTheirs(id) {
        const b = conflicts.get(id);
        conflicts.delete(id);
        remote({ op: 'replace', block: b });
        ctl.onConflict();
      },
    };
    return ctl;
  }

  const TOOLS = [
    ['paragraph', '¶', 'Paràgraf', 'Paragraph'], ['h1', 'H1', 'Títol 1', 'Heading 1'], ['h2', 'H2', 'Títol 2', 'Heading 2'],
    ['h3', 'H3', 'Títol 3', 'Heading 3'], ['bold', 'B', 'Negreta', 'Bold'], ['italic', 'I', 'Cursiva', 'Italic'],
    ['underline', 'U', 'Subratllat', 'Underline'], ['strike', 'S', 'Ratllat', 'Strikethrough'],
    ['bullets', '•', 'Llista de pics', 'Bulleted list'], ['numbers', '1.', 'Llista numerada', 'Numbered list'],
    ['quote', '❝', 'Cita', 'Quote'], ['link', '🔗', 'Enllaç', 'Link'], ['code', '</>', 'Codi', 'Code'],
    ['rule', '—', 'Línia horitzontal', 'Horizontal line'], ['undo', '↶', 'Desfés', 'Undo'], ['redo', '↷', 'Refés', 'Redo'],
  ];
  // Els desplegables d'estil (082): [clau, títol ca, en, opcions [valor, ca, en]].
  // Les fonts són de les que porta el Word tant a Windows com a macOS.
  const CUSTOM = ['custom', 'Color personalitzat…', 'Custom colour…'];
  const STYLES = [
    ['color', 'Color del text', 'Text colour', [['', 'Color automàtic', 'Automatic colour'],
      ['#000000', 'Negre', 'Black'], ['#7f7f7f', 'Gris', 'Grey'], ['#c0392b', 'Vermell', 'Red'], ['#e67e22', 'Taronja', 'Orange'],
      ['#27ae60', 'Verd', 'Green'], ['#2e75b6', 'Blau', 'Blue'], ['#8e44ad', 'Lila', 'Purple'], CUSTOM]],
    ['bg', 'Ressaltat', 'Highlight', [['', 'Sense ressaltat', 'No highlight'],
      ['#fff3a3', 'Groc', 'Yellow'], ['#c6efce', 'Verd', 'Green'], ['#cfe2f3', 'Blau', 'Blue'], ['#f4cccc', 'Rosa', 'Pink'],
      ['#e7e6e6', 'Gris', 'Grey'], CUSTOM]],
    ['font', 'Tipus de lletra', 'Font', [['', 'Lletra per defecte', 'Default font'],
      ...['Arial', 'Calibri', 'Cambria', 'Courier New', 'Georgia', 'Times New Roman', 'Verdana'].map(f => [f, f, f])]],
    ['size', 'Mida (pt)', 'Size (pt)', [['', 'Mida per defecte', 'Default size'],
      ...[8, 9, 10, 11, 12, 14, 16, 18, 20, 24, 28, 36, 48, 72].map(n => [String(n), String(n), String(n)])]],
    ['align', 'Alineació', 'Alignment', [['left', "Alinea a l'esquerra", 'Align left'], ['center', 'Centra', 'Centre'],
      ['right', 'Alinea a la dreta', 'Align right'], ['justify', 'Justifica', 'Justify']]],
  ];
  // El grup de taules i imatges de la barra (081).
  const INSERT = [
    ['table', '▦', 'Insereix una taula', 'Insert a table'], ['rowAdd', '+⇣', 'Afegeix una fila', 'Add a row'],
    ['rowDelete', '−⇣', 'Treu la fila', 'Remove the row'], ['colAdd', '+⇢', 'Afegeix una columna', 'Add a column'],
    ['colDelete', '−⇢', 'Treu la columna', 'Remove the column'], ['image', '🖼', 'Insereix una imatge', 'Insert an image'],
  ];

  // view pinta doc dins box amb la barra d'eines; render passa markdown a
  // HTML (gregalMD.render) per a l'avís d'un 409. A més d'editor(), edit té
  // notice() (l'avís), empty() (el text d'un document buit), tx(ca, en),
  // upload(file) (puja una imatge: {status, body: {path}}) i image(nom) (la
  // promesa de l'URL d'una imatge de la carpeta del document).
  // La pàgina que embolcalla el text és .doc-editor.
  function view(box, doc, render, edit) {
    const Doc = box.ownerDocument, ed = editor(doc, edit);
    const button = (label, title, fn) => {
      const b = Doc.createElement('button');
      Object.assign(b, { type: 'button', textContent: label, title });
      b.setAttribute('aria-label', title);
      b.onmousedown = e => e.preventDefault();
      b.onclick = fn;
      return b;
    };
    const bar = Doc.createElement('div'), notes = Doc.createElement('div'), page = Doc.createElement('div');
    bar.className = 'doc-toolbar';
    bar.setAttribute('role', 'toolbar');
    page.className = 'doc-editor';
    for (const [name, label, ca, en] of TOOLS) {
      bar.append(button(label, edit.tx(ca, en), () => {
        const href = name === 'link' && prompt(edit.tx("Adreça de l'enllaç", 'Link address'));
        pv.focus();
        if (name !== 'link') ed.run(commands[name]);
        else if (href) ed.run(link(href));
      }));
    }
    // L'estil va en un grup propi de la barra: cada desplegable ensenya el
    // valor de la selecció, i un valor que no és a la llista (un color
    // personalitzat, una mida de l'agent) hi surt com a opció de més.
    const group = Doc.createElement('div'), pickers = [];
    group.className = 'doc-style';
    group.setAttribute('role', 'group');
    group.setAttribute('aria-label', edit.tx('Estil del text', 'Text style'));
    for (const [key, ca, en, opts] of STYLES) {
      const sel = Doc.createElement('select'), extra = Doc.createElement('option');
      sel.title = edit.tx(ca, en);
      sel.setAttribute('aria-label', sel.title);
      for (const [v, oca, oen] of opts) sel.append(Object.assign(Doc.createElement('option'), { value: v, textContent: edit.tx(oca, oen) }));
      extra.hidden = true;
      sel.append(extra);
      const pick = Doc.createElement('input');
      Object.assign(pick, { type: 'color', hidden: true });
      const set = v => { pv.focus(); ed.run(key === 'align' ? align(v) : style(key, v)); };
      pick.onchange = () => set(pick.value);
      sel.onchange = () => {
        if (sel.value !== 'custom') return set(sel.value);
        show(ed.state);
        pick.value = /^#/.test(sel.value) ? sel.value : '#000000';
        pick.click();
      };
      pickers.push(v => {
        const known = opts.some(o => o[0] === v);
        Object.assign(extra, { value: v, textContent: v, hidden: known });
        sel.value = v;
      });
      group.append(sel, pick);
    }
    const show = st => { const c = current(st); STYLES.forEach(([key], i) => pickers[i](c[key])); };
    const insertGroup = Doc.createElement('span'), file = Doc.createElement('input');
    insertGroup.className = 'doc-toolbar-group';
    insertGroup.setAttribute('role', 'group');
    insertGroup.setAttribute('aria-label', edit.tx('Taules i imatges', 'Tables and images'));
    Object.assign(file, { type: 'file', accept: 'image/png,image/jpeg,image/gif', hidden: true });
    file.onchange = async () => {
      const f = file.files[0];
      file.value = '';
      const caption = f && prompt(edit.tx('Peu de la imatge', 'Image caption'), f.name.replace(/\.\w+$/, ''));
      if (caption == null) return;
      const r = await edit.upload(f);
      if (r.status !== 200) return edit.error(r.body.error || String(r.status));
      pv.focus();
      ed.run(image(r.body.path, caption));
    };
    const cellTools = [];
    for (const [name, label, ca, en] of INSERT) {
      const b = button(label, edit.tx(ca, en), () => {
        if (name === 'image') return file.click();
        const size = name === 'table' && /(\d+)\D+(\d+)/.exec(prompt(edit.tx('Files × columnes', 'Rows × columns'), '3 × 3') || '');
        pv.focus();
        if (name !== 'table') ed.run(commands[name]);
        else if (size) ed.run(table(Math.min(+size[1] || 1, 50), Math.min(+size[2] || 1, 20)));
      });
      if (commands[name]) cellTools.push([b, commands[name]]);
      insertGroup.append(b);
    }
    insertGroup.append(file);
    bar.append(group, insertGroup);
    box.replaceChildren(bar, notes, page);
    const empty = st => {
      const first = st.doc.firstChild;
      if (st.doc.childCount > 1 || first.content.size) return null;
      return pm.DecorationSet.create(st.doc, [pm.Decoration.node(0, first.nodeSize, { class: 'doc-empty', 'data-placeholder': edit.empty() })]);
    };
    const pv = new pm.EditorView(page, {
      state: ed.state, dispatchTransaction: tr => ed.dispatch(tr), decorations: empty,
      handleDOMEvents: { focus: () => { ed.focused = true; }, blur: () => { ed.focused = false; } },
      // Una imatge només es carrega de la carpeta del document; qualsevol altra ruta es queda en el peu.
      nodeViews: {
        image: n => {
          const img = Doc.createElement('img'), name = imageName(n.attrs.src);
          Object.assign(img, { alt: n.attrs.alt || '', title: n.attrs.alt || '' });
          if (name) edit.image(name).then(url => { img.src = url; }, e => edit.error(e.message || String(e)));
          return { dom: img };
        },
      },
    });
    const tools = st => { for (const [b, cmd] of cellTools) b.disabled = !cmd(st); };
    ed.onChange = st => { pv.updateState(st); show(st); tools(st); };
    show(ed.state);
    tools(ed.state);
    ed.onConflict = () => notes.replaceChildren(...[...ed.conflicts].map(([id, b]) => {
      const el = Doc.createElement('div');
      el.className = 'doc-conflict';
      el.innerHTML = '<p>' + edit.notice() + '</p>' + render(b.md || '');
      el.append(button(edit.tx('Desa el meu text', 'Keep my text'), () => ed.keepMine(id)),
        button(edit.tx("Fes servir el de l'agent", "Use the agent's"), () => ed.useTheirs(id)));
      return el;
    }));
    return { apply: ed.apply, focus: () => pv.focus(), get version() { return ed.version; } };
  }

  return { view, editor, onDocChanged, commands, keys, link, style, align, current, table, image, imageName, toDoc, toMd };
});
