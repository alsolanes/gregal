// Pàgina «Knowledge»: col·leccions de coneixement (globals, del projecte i
// de la sessió) i les memòries que ja existien (memòria d'usuari, notes del
// projecte, AGENTS.md). Script clàssic: index.html hi delega des de setView
// (window.gregalKnowledge.load) i fa servir les eines globals de la pàgina
// (api, gregalT). Les funcions pures es carreguen també des dels tests de
// node (knowledge.test.cjs).
(function (root) {
  'use strict';

  // ---------------------------------------------------------------- pures

  function fmt(s, vars) {
    return String(s == null ? '' : s).replace(/\{(\w+)\}/g, (m, k) => (vars && k in vars ? String(vars[k]) : m));
  }

  function esc(s) {
    return String(s == null ? '' : s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  // parseGlobs: «*.md, docs/**» → ['*.md', 'docs/**'] (comes, punt i coma
  // o salts de línia).
  function parseGlobs(s) {
    return String(s || '').split(/[,;\n]+/).map(x => x.trim()).filter(Boolean);
  }

  // groupCollections reparteix per abast en l'ordre de la pàgina.
  function groupCollections(list) {
    const g = { global: [], project: [], session: [] };
    for (const c of list || []) (g[c.scope] || (g[c.scope] = [])).push(c);
    return g;
  }

  // relTime: «just now», «5 min ago», «3 h ago», «2 d ago» o la data.
  function relTime(iso, now, lang) {
    const t = Date.parse(iso || '');
    if (!t || t < 86400000) return '';
    const s = Math.max(0, Math.round(((now || Date.now()) - t) / 1000));
    const ca = lang === 'ca';
    if (s < 60) return ca ? 'ara mateix' : 'just now';
    if (s < 3600) return ca ? 'fa ' + Math.floor(s / 60) + ' min' : Math.floor(s / 60) + ' min ago';
    if (s < 86400) return ca ? 'fa ' + Math.floor(s / 3600) + ' h' : Math.floor(s / 3600) + ' h ago';
    if (s < 86400 * 14) return ca ? 'fa ' + Math.floor(s / 86400) + ' d' : Math.floor(s / 86400) + ' d ago';
    return new Date(t).toISOString().slice(0, 10);
  }

  // statusCounts compta documents per estat (per al resum i el filtre).
  function statusCounts(docs) {
    const out = { indexed: 0, empty: 0, skipped: 0, unsupported: 0, error: 0 };
    for (const d of docs || []) out[d.status] = (out[d.status] || 0) + 1;
    return out;
  }

  // queryWords: paraules de la consulta, plegades (sense accents), per
  // ressaltar-les al fragment. Mateix criteri que el servidor.
  function fold(s) {
    return String(s || '').toLowerCase().normalize('NFD').replace(/[\u0300-\u036f]/g, '');
  }
  function queryWords(q) {
    const seen = new Set();
    return fold(q).split(/[^\p{L}\p{N}]+/u).filter(w => w.length > 1 && !seen.has(w) && seen.add(w));
  }

  // highlight escapa el fragment i hi marca les paraules de la consulta.
  function highlight(snippet, q) {
    const words = queryWords(q);
    const text = String(snippet || '');
    if (!words.length) return esc(text);
    const folded = fold(text);
    // El plegat pot canviar la llargada (NFD + treure diacrítics deixa la
    // mateixa quantitat de caràcters base): es marca per posicions.
    if (folded.length !== text.length) return esc(text);
    const marks = new Array(text.length).fill(false);
    for (const w of words) {
      let i = folded.indexOf(w);
      while (i >= 0) {
        for (let k = i; k < i + w.length; k++) marks[k] = true;
        i = folded.indexOf(w, i + w.length);
      }
    }
    let out = '', open = false;
    for (let i = 0; i < text.length; i++) {
      if (marks[i] && !open) { out += '<mark>'; open = true; }
      if (!marks[i] && open) { out += '</mark>'; open = false; }
      out += esc(text[i]);
    }
    return out + (open ? '</mark>' : '');
  }

  // statsLine resumeix una col·lecció per a la llista.
  function statsLine(c, T, lang, now) {
    const p = c.progress || {};
    if (p.running) return fmt(T('kb.indexing', 'Indexing {done}/{total}'), { done: p.done || 0, total: p.total || 0 });
    const st = c.stats || {};
    if (!st.indexed) return (c.sources || []).length || (c.notes || []).length ? T('kb.notIndexed', 'Not indexed yet') : T('kb.emptyCollection', 'Empty');
    const parts = [fmt(T('kb.docs', '{n} documents'), { n: st.docs || 0 })];
    if (st.errors) parts.push(fmt(T('kb.errorsN', '{n} errors'), { n: st.errors }));
    const when = relTime(st.indexed, now, lang);
    if (when) parts.push(when);
    return parts.join(' · ');
  }

  // ------------------------------------------------------------------ UI

  const state = { data: null, memories: [], sel: null, detail: null, docFilter: false, poll: null, lastQuery: '' };

  function T(key, fallback) {
    const v = typeof root.gregalT === 'function' ? root.gregalT(key) : '';
    return v && v !== key ? v : (fallback || key);
  }
  function lang() {
    try { return (document.documentElement.lang || 'en').slice(0, 2); } catch (e) { return 'en'; }
  }
  async function call(url, method, body) {
    const opts = { method: method || 'GET' };
    if (body !== undefined) { opts.body = JSON.stringify(body); opts.headers = { 'Content-Type': 'application/json' }; }
    const r = await root.api(url, opts);
    let d = null;
    try { d = await r.json(); } catch (e) { d = null; }
    if (!r.ok) {
      const err = new Error((d && d.error) || ('HTTP ' + r.status));
      err.status = r.status; err.data = d;
      throw err;
    }
    return d;
  }

  function rootEl() { return document.getElementById('knowledgeRoot'); }

  function shell() {
    const box = rootEl();
    if (!box) return box;
    // Es torna a pintar si canvia l'idioma (els textos fixos són aquí).
    const sig = lang() + '|' + T('nav.knowledge', 'Knowledge');
    if (box.dataset.ready === sig) return box;
    box.dataset.ready = sig;
    box.innerHTML =
      '<header class="page-head">' +
        '<div><h1>' + esc(T('nav.knowledge', 'Knowledge')) + '</h1>' +
        '<p class="page-copy">' + esc(T('kb.lead', '')) + '</p></div>' +
        '<button class="primary" type="button" data-kb="new">' + esc(T('kb.new', 'New collection')) + '</button>' +
        '<button class="ghost" type="button" data-kb="refresh">' + esc(T('kb.refresh', 'Refresh')) + '</button>' +
      '</header>' +
      '<section class="page-card kb-search">' +
        '<h2>' + esc(T('kb.search.title', 'Test retrieval')) + '</h2>' +
        '<form class="kb-search-row" data-kb-form="search">' +
          '<input name="q" autocomplete="off" placeholder="' + esc(T('kb.search.ph', '')) + '" aria-label="' + esc(T('kb.search.title', 'Search')) + '">' +
          '<select name="collection" aria-label="' + esc(T('kb.form.scope', 'Collection')) + '"></select>' +
          '<button class="ghost" type="submit">' + esc(T('kb.search.btn', 'Search')) + '</button>' +
        '</form>' +
        '<div class="kb-mode" id="kbMode"></div>' +
        '<div class="kb-results" id="kbResults" aria-live="polite"></div>' +
      '</section>' +
      '<div class="kb-grid">' +
        '<nav class="kb-list" id="kbList" aria-label="' + esc(T('nav.knowledge', 'Knowledge')) + '"></nav>' +
        '<section class="page-card kb-detail" id="kbDetail"></section>' +
      '</div>';
    if (!box.dataset.bound) {
      box.dataset.bound = '1';
      box.addEventListener('click', onClick);
      box.addEventListener('submit', onSubmit);
      box.addEventListener('change', onChange);
    }
    return box;
  }

  async function load() {
    const box = shell();
    if (!box) return;
    try {
      const [d, m] = await Promise.all([call('/api/knowledge'), call('/api/knowledge/memory')]);
      state.data = d;
      state.memories = (m && m.memories) || [];
    } catch (e) {
      document.getElementById('kbList').innerHTML = '<div class="activity-empty">' + esc(T('kb.failed', 'Failed: ') + e.message) + '</div>';
      return;
    }
    renderList();
    renderSearchControls();
    if (state.sel && state.sel.type === 'col') {
      const still = (state.data.collections || []).some(c => c.id === state.sel.id);
      if (still) await openCollection(state.sel.id, true); else { state.sel = null; renderDetail(); }
    } else renderDetail();
    schedulePoll();
  }

  function schedulePoll() {
    clearTimeout(state.poll);
    const busy = ((state.data && state.data.collections) || []).some(c => c.progress && c.progress.running);
    if (!busy) return;
    state.poll = setTimeout(() => {
      const page = document.getElementById('knowledgePage');
      if (page && !page.hidden) load();
    }, 900);
  }

  function renderSearchControls() {
    const sel = rootEl().querySelector('select[name=collection]');
    const cols = (state.data && state.data.collections) || [];
    const cur = sel.value;
    sel.innerHTML = '<option value="">' + esc(T('kb.search.all', 'All enabled collections')) + '</option>' +
      cols.map(c => '<option value="' + esc(c.id) + '">' + esc(c.name) + ' · ' + esc(scopeLabel(c.scope)) + '</option>').join('');
    sel.value = cols.some(c => c.id === cur) ? cur : '';
    const mode = document.getElementById('kbMode');
    mode.textContent = state.data && state.data.embeddings
      ? fmt(T('kb.embeddings', 'Hybrid search: keywords + {model}'), { model: state.data.embed_model })
      : T('kb.bm25', 'Keyword search (BM25), offline');
  }

  function scopeLabel(s) {
    return T('kb.scope.' + s, s);
  }

  function renderList() {
    const list = document.getElementById('kbList');
    const d = state.data || {};
    const g = groupCollections(d.collections);
    const lg = lang(), now = Date.now();
    const item = c => {
      const active = state.sel && state.sel.type === 'col' && state.sel.id === c.id;
      const run = c.progress && c.progress.running;
      return '<button type="button" class="kb-item' + (active ? ' active' : '') + (c.enabled ? '' : ' off') + '" data-kb-col="' + esc(c.id) + '"' + (active ? ' aria-current="true"' : '') + '>' +
        '<span class="kb-item-name">' + (run ? '<span class="kb-dot busy" aria-hidden="true"></span>' : '') + esc(c.name) + (c.enabled ? '' : ' <small>· ' + esc(T('kb.disabled', 'Disabled')) + '</small>') + '</span>' +
        (c.description ? '<span class="kb-item-desc">' + esc(c.description) + '</span>' : '') +
        '<span class="kb-item-meta">' + esc(statsLine(c, T, lg, now)) + '</span></button>';
    };
    const group = (scope, hint, items, extra) =>
      '<div class="kb-group"><div class="kb-group-head"><b>' + esc(scopeLabel(scope)) + '</b><small>' + esc(hint) + '</small></div>' +
      (items.length ? items.map(item).join('') : '<div class="kb-none">' + esc(extra || T('kb.emptyScope', 'None yet.')) + '</div>') + '</div>';
    let html = '';
    html += group('global', T('kb.scope.globalHint', 'All projects'), g.global);
    html += group('project', d.project ? T('kb.scope.projectHint', '') : '', g.project, d.project ? '' : T('kb.noProject', ''));
    html += group('session', T('kb.scope.sessionHint', ''), g.session);
    const mems = state.memories.map(m => {
      const active = state.sel && state.sel.type === 'mem' && state.sel.id === m.kind;
      const meta = m.exists ? (m.text.trim() ? fmt(T('kb.mem.lines', '{n} lines'), { n: m.text.trim().split('\n').length }) : T('kb.mem.empty', '(empty)')) : (m.kind === 'rules' ? T('kb.mem.none', '') : T('kb.mem.empty', '(empty)'));
      return '<button type="button" class="kb-item' + (active ? ' active' : '') + '" data-kb-mem="' + esc(m.kind) + '">' +
        '<span class="kb-item-name">' + esc(T('kb.mem.' + m.kind, m.kind)) + '</span>' +
        '<span class="kb-item-desc">' + esc(T('kb.mem.' + m.kind + 'Hint', '')) + '</span>' +
        '<span class="kb-item-meta">' + esc(meta) + '</span></button>';
    }).join('');
    html += '<div class="kb-group"><div class="kb-group-head"><b>' + esc(T('kb.builtin', 'Built-in memory')) + '</b></div>' + mems + '</div>';
    list.innerHTML = html;
  }

  function renderDetail() {
    const box = document.getElementById('kbDetail');
    if (!box) return;
    if (!state.sel) {
      const empty = !((state.data && state.data.collections) || []).length;
      box.innerHTML = '<div class="activity-empty">' + esc(empty ? T('kb.empty', '') : T('kb.selectHint', '')) + '</div>' +
        (empty ? '<p><button class="primary" type="button" data-kb="new">' + esc(T('kb.new', 'New collection')) + '</button></p>' : '');
      return;
    }
    if (state.sel.type === 'new') return renderNewForm(box);
    if (state.sel.type === 'mem') return renderMemory(box);
    if (state.sel.type === 'col' && state.detail) return renderCollection(box, state.detail);
  }

  function folderField(name, value) {
    return '<div class="kb-path"><input name="' + name + '" class="mono" spellcheck="false" value="' + esc(value || '') + '" placeholder="' + esc(T('kb.form.folderPh', 'C:\\docs or ./docs')) + '">' +
      '<button class="ghost" type="button" data-kb="browse" data-kb-target="' + name + '">' + esc(T('kb.form.browse', 'Browse…')) + '</button></div>';
  }

  function renderNewForm(box) {
    const d = state.data || {};
    const opt = (v, dis) => '<option value="' + v + '"' + (dis ? ' disabled' : '') + '>' + esc(scopeLabel(v)) + '</option>';
    box.innerHTML =
      '<h2>' + esc(T('kb.new', 'New collection')) + '</h2>' +
      '<form class="kb-form" data-kb-form="create">' +
        '<label>' + esc(T('kb.form.name', 'Name')) + '<input name="name" required maxlength="80"></label>' +
        '<label>' + esc(T('kb.form.desc', 'Description')) + '<input name="description" maxlength="300"></label>' +
        '<label>' + esc(T('kb.form.scope', 'Scope')) + '<select name="scope">' +
          opt('project', !d.project) + opt('global', d.can_global === false) + opt('session') + '</select></label>' +
        '<label>' + esc(T('kb.form.folder', 'Folder or file (optional)')) + folderField('path', '') + '</label>' +
        '<label>' + esc(T('kb.form.include', 'Include')) + '<input name="include" class="mono" placeholder="*.md, *.pdf, docs/**"></label>' +
        '<div class="kb-actions"><button class="primary" type="submit">' + esc(T('kb.form.create', 'Create')) + '</button>' +
        '<button class="ghost" type="button" data-kb="cancel">' + esc(T('kb.form.cancel', 'Cancel')) + '</button></div>' +
        '<div class="kb-msg" role="status"></div>' +
      '</form>';
    const scope = box.querySelector('select[name=scope]');
    scope.value = d.project ? 'project' : 'session';
    box.querySelector('input[name=name]').focus();
  }

  function renderMemory(box) {
    const m = state.memories.find(x => x.kind === state.sel.id);
    if (!m) { box.innerHTML = ''; return; }
    const title = T('kb.mem.' + m.kind, m.kind);
    if (m.kind === 'rules' && !m.path) {
      box.innerHTML = '<h2>' + esc(title) + '</h2><p class="page-copy">' + esc(T('kb.mem.none', '')) + '</p>';
      return;
    }
    box.innerHTML =
      '<div class="kb-detail-head"><div><h2>' + esc(title) + '</h2><code class="kb-path-label">' + esc(m.path) + '</code></div></div>' +
      '<p class="page-copy">' + esc(T('kb.mem.' + m.kind + 'Hint', '')) + '</p>' +
      '<form class="kb-form" data-kb-form="memory">' +
        '<textarea name="text" rows="14" class="mono"' + (m.editable ? '' : ' readonly') + '>' + esc(m.text) + '</textarea>' +
        (m.editable
          ? '<div class="kb-actions"><button class="primary" type="submit">' + esc(T('kb.mem.save', 'Save')) + '</button>' +
            '<button class="ghost" type="button" data-kb="mem-reload">' + esc(T('kb.mem.reload', 'Reload')) + '</button></div>'
          : '<p class="kb-none">' + esc(T('kb.mem.readonly', '')) + '</p>') +
        '<div class="kb-msg" role="status"></div>' +
      '</form>';
  }

  function docRow(d, lg, now) {
    const st = d.status || 'indexed';
    return '<tr class="kb-doc ' + esc(st) + '"><td class="mono" title="' + esc(d.key) + '">' + esc(d.path) + '</td>' +
      '<td>' + esc(d.kind || '') + '</td>' +
      '<td><span class="kb-status ' + esc(st) + '">' + esc(T('kb.status.' + st, st)) + '</span>' + (d.error ? '<small class="kb-err">' + esc(d.error) + '</small>' : '') + '</td>' +
      '<td class="num">' + (d.chunks || 0) + '</td><td>' + esc(relTime(d.indexed, now, lg)) + '</td></tr>';
  }

  function renderCollection(box, c) {
    const lg = lang(), now = Date.now();
    const p = c.progress || {};
    const st = c.stats || {};
    const canEdit = c.scope !== 'global' || !(state.data && state.data.can_global === false);
    const missing = new Set(c.missing_sources || []);
    const sources = (c.sources || []).map(s =>
      '<li><div><code class="kb-path-label">' + esc((c.source_paths || {})[s.id] || s.path) + '</code>' +
      (missing.has(s.id) ? ' <span class="kb-status error">' + esc(T('kb.sources.missing', 'missing')) + '</span>' : '') +
      '<small>' + esc(s.include && s.include.length ? s.include.join(', ') : T('kb.sources.all', 'all readable files')) +
      (s.exclude && s.exclude.length ? ' · − ' + esc(s.exclude.join(', ')) : '') + '</small></div>' +
      (canEdit ? '<button class="ghost tiny" type="button" data-kb="source-remove" data-id="' + esc(s.id) + '">' + esc(T('kb.sources.remove', 'Remove')) + '</button>' : '') + '</li>').join('');
    const notes = (c.notes || []).slice().reverse().map(n =>
      '<li><div><b>' + esc(n.title || relTime(n.created, now, lg)) + '</b>' +
      (n.author ? ' <small>' + esc(fmt(T('kb.notes.by', 'by {who}'), { who: n.author })) + '</small>' : '') +
      '<p>' + esc(n.text) + '</p></div>' +
      (canEdit ? '<button class="ghost tiny" type="button" data-kb="note-remove" data-id="' + esc(n.id) + '">' + esc(T('kb.sources.remove', 'Remove')) + '</button>' : '') + '</li>').join('');
    const docs = (c.docs || []).filter(d => !state.docFilter || (d.status !== 'indexed' && d.status !== 'empty'));
    const counts = statusCounts(c.docs);
    const problems = counts.error + counts.unsupported + counts.skipped;
    const progress = p.running
      ? '<div class="kb-progress" role="progressbar" aria-valuemin="0" aria-valuemax="' + (p.total || 0) + '" aria-valuenow="' + (p.done || 0) + '"><span style="width:' + (p.total ? Math.round(100 * (p.done || 0) / p.total) : 5) + '%"></span></div>' +
        '<small class="kb-none">' + esc(fmt(T('kb.indexing', 'Indexing {done}/{total}'), { done: p.done || 0, total: p.total || 0 })) + (p.current ? ' · ' + esc(p.current) : '') + '</small>'
      : (p.error ? '<p class="kb-err">' + esc(T('kb.failed', 'Failed: ') + p.error) + '</p>' : '');
    const warn = [];
    if (st.truncated) warn.push(fmt(T('kb.truncated', 'Limit reached ({limit}).'), { limit: st.limit || '' }));
    if (st.embed_error) warn.push(fmt(T('kb.embedError', 'Embeddings failed: {error}'), { error: st.embed_error }));
    if (c.index_error) warn.push(c.index_error);
    box.innerHTML =
      '<div class="kb-detail-head">' +
        '<div><h2>' + esc(c.name) + ' <span class="kb-badge">' + esc(scopeLabel(c.scope)) + '</span></h2>' +
        (c.description ? '<p class="page-copy">' + esc(c.description) + '</p>' : '') +
        '<small class="kb-none">' + esc([fmt(T('kb.docs', '{n} documents'), { n: st.docs || 0 }), fmt(T('kb.chunks', '{n} chunks'), { n: st.chunks || 0 }), st.indexed ? fmt(T('kb.indexed', 'indexed {when}'), { when: relTime(st.indexed, now, lg) }) : T('kb.notIndexed', 'Not indexed yet')].join(' · ')) + '</small></div>' +
        (canEdit ? '<div class="kb-actions">' +
          '<label><input type="checkbox" data-kb="toggle"' + (c.enabled ? ' checked' : '') + '> ' + esc(T('kb.enabled', 'Enabled')) + '</label>' +
          '<button class="ghost" type="button" data-kb="edit">' + esc(T('kb.edit', 'Edit')) + '</button>' +
          '<button class="ghost" type="button" data-kb="reindex"' + (p.running ? ' disabled' : '') + ' title="' + esc(T('kb.reindexHint', '')) + '">' + esc(T('kb.reindex', 'Reindex')) + '</button>' +
          '<button class="ghost" type="button" data-kb="reindex-full"' + (p.running ? ' disabled' : '') + '>' + esc(T('kb.reindexFull', 'Full reindex')) + '</button>' +
          '<button class="danger" type="button" data-kb="delete"' + (p.running ? ' disabled' : '') + '>' + esc(T('kb.delete', 'Delete')) + '</button></div>'
          : '<p class="kb-none">' + esc(T('kb.globalAdmin', '')) + '</p>') +
      '</div>' + progress +
      (warn.length ? '<p class="kb-warn">' + warn.map(esc).join('<br>') + '</p>' : '') +
      '<div class="kb-msg" role="status"></div>' +
      '<form class="kb-form kb-edit" data-kb-form="edit" hidden>' +
        '<label>' + esc(T('kb.form.name', 'Name')) + '<input name="name" required maxlength="80" value="' + esc(c.name) + '"></label>' +
        '<label>' + esc(T('kb.form.desc', 'Description')) + '<input name="description" maxlength="300" value="' + esc(c.description || '') + '"></label>' +
        '<div class="kb-actions"><button class="primary" type="submit">' + esc(T('kb.save', 'Save')) + '</button><button class="ghost" type="button" data-kb="edit-cancel">' + esc(T('kb.form.cancel', 'Cancel')) + '</button></div>' +
      '</form>' +
      '<h3>' + esc(T('kb.sources', 'Sources')) + '</h3>' +
      (sources ? '<ul class="kb-rows">' + sources + '</ul>' : '<p class="kb-none">' + esc(T('kb.sources.none', '')) + '</p>') +
      (canEdit ? '<form class="kb-form kb-inline" data-kb-form="source">' + folderField('path', '') +
        '<input name="include" class="mono" placeholder="' + esc(T('kb.form.includeShort', 'Include: *.md, docs/**')) + '">' +
        '<input name="exclude" class="mono" placeholder="' + esc(T('kb.form.excludeShort', 'Exclude: drafts/**')) + '">' +
        '<button class="ghost" type="submit">' + esc(T('kb.sources.add', 'Add source')) + '</button></form>' : '') +
      '<h3>' + esc(T('kb.notes', 'Notes')) + '</h3>' +
      (canEdit ? '<form class="kb-form" data-kb-form="note">' +
        '<input name="title" maxlength="120" placeholder="' + esc(T('kb.notes.title', 'Title (optional)')) + '">' +
        '<textarea name="text" rows="3" maxlength="8000" placeholder="' + esc(T('kb.notes.text', 'Write a note…')) + '"></textarea>' +
        '<div class="kb-actions"><button class="ghost" type="submit">' + esc(T('kb.notes.add', 'Add note')) + '</button></div></form>' : '') +
      (notes ? '<ul class="kb-rows kb-notes">' + notes + '</ul>' : '<p class="kb-none">' + esc(T('kb.notes.none', 'No notes.')) + '</p>') +
      '<div class="kb-docs-head"><h3>' + esc(T('kb.documents', 'Documents')) + '</h3>' +
      (problems ? '<label><input type="checkbox" data-kb="doc-filter"' + (state.docFilter ? ' checked' : '') + '> ' + esc(fmt(T('kb.documents.problems', 'Only problems ({n})'), { n: problems })) + '</label>' : '') + '</div>' +
      (docs.length
        ? '<div class="kb-table-wrap"><table class="kb-table"><thead><tr><th>' + esc(T('kb.col.path', 'Path')) + '</th><th>' + esc(T('kb.col.kind', 'Type')) + '</th><th>' + esc(T('kb.col.status', 'Status')) + '</th><th class="num">' + esc(T('kb.col.chunks', 'Chunks')) + '</th><th>' + esc(T('kb.col.indexed', 'Indexed')) + '</th></tr></thead><tbody>' +
          docs.slice(0, 500).map(d => docRow(d, lg, now)).join('') + '</tbody></table></div>' +
          (docs.length > 500 ? '<p class="kb-none">' + esc(fmt(T('kb.documents.more', '…and {n} more'), { n: docs.length - 500 })) + '</p>' : '')
        : '<p class="kb-none">' + esc(T('kb.documents.none', 'Nothing indexed yet.')) + '</p>');
  }

  function msg(text, bad) {
    const el = document.querySelector('#kbDetail .kb-msg');
    if (!el) return;
    el.textContent = text || '';
    el.classList.toggle('bad', !!bad);
  }

  async function openCollection(id, quiet) {
    state.sel = { type: 'col', id };
    try {
      state.detail = await call('/api/knowledge/' + encodeURIComponent(id));
    } catch (e) {
      if (!quiet) msg(T('kb.failed', 'Failed: ') + e.message, true);
      return;
    }
    renderList();
    renderDetail();
  }

  async function act(fn) {
    try { await fn(); }
    catch (e) { msg(T('kb.failed', 'Failed: ') + e.message, true); return false; }
    return true;
  }

  async function browse(target) {
    const input = document.querySelector('#kbDetail input[name="' + target + '"]');
    const put = p => { if (input && p) { input.value = p; input.focus(); } };
    if (root.gregalDesktop && root.gregalDesktop.chooseFolder) {
      put(await root.gregalDesktop.chooseFolder());
      return;
    }
    const ws = root.gregalPanels && root.gregalPanels.workspaces;
    if (ws && ws.pick) {
      await ws.pick({ title: T('kb.chooseFolder', 'Folder for this collection'), chooseLabel: T('kb.chooseThis', 'Use this folder'), onChoose: p => { put(p); return true; } });
    }
  }

  async function onClick(e) {
    const colBtn = e.target.closest('[data-kb-col]');
    if (colBtn) { state.docFilter = false; return openCollection(colBtn.dataset.kbCol); }
    const memBtn = e.target.closest('[data-kb-mem]');
    if (memBtn) { state.sel = { type: 'mem', id: memBtn.dataset.kbMem }; renderList(); return renderDetail(); }
    const b = e.target.closest('[data-kb]');
    if (!b || b.tagName === 'INPUT') return;
    const c = state.detail;
    switch (b.dataset.kb) {
      case 'new': state.sel = { type: 'new' }; renderList(); return renderDetail();
      case 'refresh': return load();
      case 'cancel': state.sel = null; renderList(); return renderDetail();
      case 'browse': return browse(b.dataset.kbTarget);
      case 'edit': { const f = document.querySelector('[data-kb-form=edit]'); f.hidden = !f.hidden; if (!f.hidden) f.querySelector('input').focus(); return; }
      case 'edit-cancel': document.querySelector('[data-kb-form=edit]').hidden = true; return;
      case 'reindex': case 'reindex-full':
        return act(async () => { await call('/api/knowledge/' + c.id + '/reindex', 'POST', { force: b.dataset.kb === 'reindex-full' }); await load(); });
      case 'delete':
        if (!confirm(fmt(T('kb.confirmDelete', 'Delete "{name}"?'), { name: c.name }))) return;
        return act(async () => { await call('/api/knowledge/' + c.id, 'DELETE'); state.sel = null; state.detail = null; await load(); });
      case 'source-remove':
        return act(async () => { await call('/api/knowledge/' + c.id + '/sources', 'DELETE', { id: b.dataset.id }); await load(); });
      case 'note-remove':
        return act(async () => { await call('/api/knowledge/' + c.id + '/notes', 'DELETE', { id: b.dataset.id }); await load(); });
      case 'mem-reload': return load();
    }
  }

  async function onChange(e) {
    const t = e.target;
    if (t.dataset.kb === 'toggle' && state.detail) {
      const id = state.detail.id;
      await act(async () => { await call('/api/knowledge/' + id, 'PATCH', { enabled: t.checked }); await load(); });
    }
    if (t.dataset.kb === 'doc-filter') { state.docFilter = t.checked; renderDetail(); }
  }

  async function onSubmit(e) {
    const f = e.target.closest('[data-kb-form]');
    if (!f) return;
    e.preventDefault();
    const v = name => (f.elements[name] ? f.elements[name].value.trim() : '');
    const c = state.detail;
    switch (f.dataset.kbForm) {
      case 'search': return search(v('q'), v('collection'));
      case 'create':
        return act(async () => {
          const d = await call('/api/knowledge', 'POST', { name: v('name'), description: v('description'), scope: v('scope'), path: v('path'), include: parseGlobs(v('include')) });
          await load();
          await openCollection(d.id);
        });
      case 'edit':
        return act(async () => { await call('/api/knowledge/' + c.id, 'PATCH', { name: v('name'), description: v('description') }); await load(); });
      case 'source':
        if (!v('path')) return;
        return act(async () => { await call('/api/knowledge/' + c.id + '/sources', 'POST', { path: v('path'), include: parseGlobs(v('include')), exclude: parseGlobs(v('exclude')) }); await load(); });
      case 'note':
        if (!v('text')) return;
        return act(async () => { await call('/api/knowledge/' + c.id + '/notes', 'POST', { title: v('title'), text: f.elements.text.value }); await load(); });
      case 'memory': {
        const m = state.memories.find(x => x.kind === state.sel.id);
        try {
          const d = await call('/api/knowledge/memory', 'PUT', { kind: m.kind, text: f.elements.text.value, hash: m.hash });
          state.memories = d.memories || [];
          renderList(); renderDetail();
          msg(T('kb.mem.saved', 'Saved.'));
        } catch (err) {
          if (err.status === 409 && err.data && err.data.current) {
            // Algú l'ha canviada: es carrega l'actual i es diu, sense perdre res
            // (el text de l'usuari queda al porta-retalls del missatge).
            const mine = f.elements.text.value;
            const i = state.memories.findIndex(x => x.kind === m.kind);
            if (i >= 0) state.memories[i] = Object.assign({}, m, err.data.current, { editable: m.editable });
            renderDetail();
            msg(T('kb.mem.conflict', 'This memory changed since you opened it.'), true);
            const box = document.querySelector('#kbDetail .kb-msg');
            if (box) { const pre = document.createElement('pre'); pre.className = 'kb-mine'; pre.textContent = mine; box.appendChild(pre); }
          } else msg(T('kb.failed', 'Failed: ') + err.message, true);
        }
        return;
      }
    }
  }

  async function search(q, collection) {
    const out = document.getElementById('kbResults');
    if (!q) { out.innerHTML = ''; return; }
    state.lastQuery = q;
    out.innerHTML = '<div class="activity-empty">…</div>';
    let d;
    try {
      d = await call('/api/knowledge/search?q=' + encodeURIComponent(q) + '&k=8' + (collection ? '&collection=' + encodeURIComponent(collection) : ''));
    } catch (e) {
      out.innerHTML = '<div class="kb-err">' + esc(T('kb.failed', 'Failed: ') + e.message) + '</div>';
      return;
    }
    out.innerHTML = renderHits(d.hits || [], q, T);
  }

  // renderHits és pur (el test el crida).
  function renderHits(hits, q, tr) {
    const t = tr || ((k, f) => f || k);
    if (!hits.length) return '<div class="activity-empty">' + esc(t('kb.search.none', 'No passages found.')) + '</div>';
    return '<ol class="kb-hits">' + hits.map(h =>
      '<li><div class="kb-hit-head"><span class="mono">' + esc(h.path) + (h.loc ? ' · ' + esc(h.loc) : '') + '</span>' +
      '<span class="kb-badge">' + esc(h.collection) + '</span><small>' + esc(String(h.score)) + '</small></div>' +
      '<p>' + highlight(h.snippet, q) + '</p></li>').join('') + '</ol>';
  }

  const api = { load, parseGlobs, groupCollections, relTime, statusCounts, highlight, queryWords, statsLine, renderHits, fmt };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  if (typeof window !== 'undefined') window.gregalKnowledge = api;
})(typeof window !== 'undefined' ? window : globalThis);
