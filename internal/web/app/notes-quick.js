// Notes a mà des de qualsevol lloc (Codi o Vents): un botó a la capçalera i
// Ctrl+Alt+N obren un full gran amb la nota a la dreta i la resta de notes a
// l'esquerra. Cada nota es pot associar a un projecte, s'escriu en markdown
// (amb barra de format) i es llegeix ben formatada. Les accions d'IA
// (netejar, organitzar, resumir, extreure tasques o el que demanis) proposen
// un text nou que l'usuari aplica o descarta: res no es desa sol.
// Servidor: notes.go (/api/notes) i notes_ai.go (/api/notes/ai).
(function (root) {
  'use strict';

  const $ = id => document.getElementById(id);
  const ca = () => (document.documentElement.lang || 'ca') !== 'en';
  const t = (c, e) => (ca() ? c : e);
  const LAST = 'gregal_qn_last';
  const icon = (name, fallback) => root.gregalIcon?.(name) || fallback || '';
  const NOTE_SVG = '<svg class="ui-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 3h9l4 4v14H6z M14 3v5h5 M9 12h7 M9 16h5"/></svg>';

  const S = {
    open: false, list: [], projects: [], noteProjects: [], filter: '', q: '',
    note: null, dirty: false, saving: null, timer: 0, mode: 'read', proposal: null, busy: false,
  };

  function el(tag, cls, text) {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }
  function btn(cls, label, onclick, html) {
    const b = el('button', cls);
    b.type = 'button';
    if (html) b.innerHTML = html; else b.textContent = label;
    if (label) { b.title = label; b.setAttribute('aria-label', label); }
    b.onclick = onclick;
    return b;
  }
  async function call(url, body) {
    const opts = body === undefined ? {} : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) };
    const r = await root.api(url, opts);
    if (!r.ok) {
      const err = new Error(((await r.text()) || '').trim() || 'HTTP ' + r.status);
      err.status = r.status;
      throw err;
    }
    return r.json();
  }
  const base = p => String(p || '').split(/[\\/]/).filter(Boolean).pop() || p;
  const day = iso => root.gregalNotes?.dayText?.(iso, ca() ? 'ca' : 'en') || '';
  function say(text, bad) {
    const s = $('qnStatus');
    if (!s) return;
    s.textContent = text || '';
    s.classList.toggle('bad', !!bad);
  }

  // ------------------------------------------------------------ dades

  async function loadProjects() {
    try {
      const d = await call('/api/workspaces');
      S.projects = (d.workspaces || []).filter(w => !w.missing).map(w => ({ path: w.path, name: w.name || base(w.path) }));
      S.current = d.current || '';
    } catch { S.projects = []; }
  }
  async function loadList() {
    const qs = new URLSearchParams();
    if (S.q) qs.set('q', S.q);
    if (S.filter) qs.set('project', S.filter);
    const d = await call('/api/notes' + (qs.toString() ? '?' + qs : ''));
    S.list = d.notes || [];
    S.noteProjects = d.projects || [];
    paintList();
  }
  async function openNote(id) {
    await flush();
    const d = await call('/api/notes?id=' + encodeURIComponent(id));
    S.note = d.note; S.dirty = false; S.proposal = null;
    S.mode = (S.note.body || '').trim() ? 'read' : 'edit';
    try { localStorage.setItem(LAST, id); } catch {}
    paintList(); paintNote();
  }
  async function newNote() {
    await flush();
    // Des de Codi, la nota nova va al projecte obert (es pot canviar).
    const product = document.documentElement.dataset.product || 'codi';
    const project = S.filter && S.filter !== '-' ? S.filter : (product === 'codi' ? S.current || '' : '');
    // Esborrany local: el servidor no desa notes buides, així que es crea
    // amb la primera lletra.
    S.note = { id: '', title: '', body: '', project, pinned: false, updated: '' };
    S.dirty = false; S.proposal = null; S.mode = 'edit';
    paintList(); paintNote();
    setTimeout(() => $('qnTitle')?.focus(), 0);
  }

  // Desa el que hi ha a l'editor (amb la versió de base per detectar
  // conflictes amb un vent o una altra finestra).
  async function save() {
    if (!S.note || !S.dirty) return;
    const n = S.note;
    S.dirty = false;
    if (!n.id && !(n.title || '').trim() && !(n.body || '').trim()) { say(''); return; }
    say(t('Desant…', 'Saving…'));
    try {
      const req = { action: 'save', title: n.title, body: n.body, project: n.project || '' };
      if (n.id) { req.id = n.id; req.base = n.updated; }
      const out = await call('/api/notes', req);
      const created = !n.id;
      n.id = out.id; n.updated = out.updated;
      if (created) {
        try { localStorage.setItem(LAST, out.id); } catch {}
        await loadList();
        // No es repinta la nota: l'usuari hi està escrivint. Només s'activen
        // fixar i esborrar, que ja tenen id.
        document.querySelectorAll('#qnMain .qn-top .qn-icon:disabled').forEach(b => { b.disabled = false; });
      } else {
        const item = S.list.find(x => x.id === out.id);
        if (item) Object.assign(item, { title: out.title, excerpt: out.excerpt, updated: out.updated, project: out.project });
        paintList();
      }
      say(t('Desat', 'Saved'));
    } catch (e) {
      if (e.status === 409) say(t('Aquesta nota ha canviat en un altre lloc: torna-la a obrir.', 'This note changed elsewhere: reopen it.'), true);
      else { S.dirty = true; say(t('No s\'ha pogut desar: ', 'Could not save: ') + e.message, true); }
    }
  }
  function touch() {
    S.dirty = true;
    say(t('Sense desar', 'Unsaved'));
    clearTimeout(S.timer);
    S.timer = setTimeout(() => { S.saving = save(); }, 700);
  }
  async function flush() {
    clearTimeout(S.timer);
    if (S.dirty) await save();
    else if (S.saving) await S.saving;
  }

  // ------------------------------------------------------------ pintar

  function shell() {
    let box = $('qnPanel');
    if (box) return box;
    box = el('div', 'qn-backdrop');
    box.id = 'qnPanel';
    box.hidden = true;
    box.addEventListener('mousedown', e => { if (e.target === box) close(); });
    const sheet = el('div', 'qn-sheet');
    sheet.setAttribute('role', 'dialog');
    sheet.setAttribute('aria-modal', 'true');
    sheet.setAttribute('aria-label', t('Notes', 'Notes'));
    const side = el('aside', 'qn-side');
    const head = el('div', 'qn-side-head');
    head.append(el('b', '', t('Notes', 'Notes')), btn('qn-new', t('Nota nova', 'New note'), () => newNote().catch(e => say(e.message, true)), icon('plus', '+')));
    const search = el('input', 'qn-search');
    search.id = 'qnSearch';
    search.type = 'search';
    search.placeholder = t('Cerca a les notes…', 'Search notes…');
    let st = 0;
    search.oninput = () => { clearTimeout(st); st = setTimeout(() => { S.q = search.value.trim(); loadList().catch(() => {}); }, 180); };
    const filter = el('select', 'qn-filter');
    filter.id = 'qnFilter';
    filter.setAttribute('aria-label', t('Projecte', 'Project'));
    filter.onchange = () => { S.filter = filter.value; loadList().catch(() => {}); };
    const list = el('div', 'qn-list');
    list.id = 'qnList';
    side.append(head, search, filter, list);
    const main = el('section', 'qn-main');
    main.id = 'qnMain';
    sheet.append(side, main);
    box.append(sheet);
    document.body.append(box);
    box.addEventListener('keydown', e => {
      if (e.key === 'Escape') { e.preventDefault(); if (S.proposal) { S.proposal = null; paintNote(); } else close(); }
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'e') { e.preventDefault(); setMode(S.mode === 'edit' ? 'read' : 'edit'); }
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') { e.preventDefault(); flush(); }
    });
    return box;
  }

  function projectOptions(sel, value, withAll) {
    sel.textContent = '';
    const add = (v, label) => { const o = el('option', '', label); o.value = v; sel.append(o); };
    if (withAll) { add('', t('Totes les notes', 'All notes')); add('-', t('Sense projecte', 'No project')); }
    else add('', t('Sense projecte', 'No project'));
    const seen = new Set();
    const known = [...S.projects, ...S.noteProjects.map(p => ({ path: p.name, name: base(p.name) }))];
    for (const p of known) {
      if (!p.path || seen.has(p.path)) continue;
      seen.add(p.path);
      add(p.path, p.name);
    }
    sel.value = value || '';
    if (sel.value !== (value || '')) { add(value, base(value)); sel.value = value; }
  }

  function paintList() {
    const list = $('qnList');
    if (!list) return;
    const filter = $('qnFilter');
    if (filter && document.activeElement !== filter) projectOptions(filter, S.filter, true);
    list.textContent = '';
    if (!S.list.length) {
      list.append(el('p', 'qn-empty', S.q ? t('Cap nota no hi coincideix.', 'No matching notes.') : t('Encara no hi ha notes aquí.', 'No notes here yet.')));
      return;
    }
    for (const n of S.list) {
      const item = btn('qn-item' + (S.note && S.note.id === n.id ? ' on' : ''), '', () => openNote(n.id).catch(e => say(e.message, true)));
      item.removeAttribute('aria-label');
      const top = el('span', 'qn-item-top');
      top.append(el('b', '', n.title || t('Sense títol', 'Untitled')));
      if (n.pinned) top.append(el('span', 'qn-pin', '★'));
      item.append(top);
      const meta = el('span', 'qn-item-meta');
      meta.append(el('span', '', day(n.updated)));
      if (n.project) meta.append(el('span', 'qn-proj', base(n.project)));
      if (n.tasks_open) meta.append(el('span', '', '☐ ' + n.tasks_open));
      item.append(meta);
      if (n.excerpt) item.append(el('span', 'qn-item-ex', n.excerpt));
      list.append(item);
    }
  }

  function setMode(m) {
    if (!S.note) return;
    S.mode = m;
    paintNote();
    if (m === 'edit') setTimeout(() => $('qnBody')?.focus(), 0);
  }

  // Barra de format: embolcalla la selecció o posa el prefix a la línia.
  function format(kind) {
    const ta = $('qnBody');
    if (!ta) return;
    const { selectionStart: a, selectionEnd: b, value } = ta;
    const sel = value.slice(a, b);
    const wrap = (l, r, ph) => { const inner = sel || ph; ta.setRangeText(l + inner + r, a, b, 'end'); if (!sel) { ta.selectionStart = a + l.length; ta.selectionEnd = a + l.length + inner.length; } };
    const line = prefix => {
      const start = value.lastIndexOf('\n', a - 1) + 1;
      const end = b > a ? b : a;
      const block = value.slice(start, end);
      const out = block.split('\n').map(l => (l.startsWith(prefix) ? l.slice(prefix.length) : prefix + l)).join('\n');
      ta.setRangeText(out, start, end, 'end');
    };
    switch (kind) {
      case 'b': wrap('**', '**', t('negreta', 'bold')); break;
      case 'i': wrap('_', '_', t('cursiva', 'italic')); break;
      case 'h': line('## '); break;
      case 'ul': line('- '); break;
      case 'task': line('- [ ] '); break;
      case 'quote': line('> '); break;
      case 'code': sel.includes('\n') ? wrap('```\n', '\n```', '') : wrap('`', '`', 'codi'); break;
      case 'link': wrap('[', '](https://)', t('text', 'text')); break;
    }
    ta.focus();
    S.note.body = ta.value;
    touch();
  }

  function render(body) {
    const html = root.gregalMD ? root.gregalMD.render(body || '') : String(body || '').replace(/[&<>]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' })[c]);
    return root.gregalNotes?.linkNotes ? root.gregalNotes.linkNotes(html) : html;
  }

  function paintNote() {
    const main = $('qnMain');
    if (!main) return;
    main.textContent = '';
    const top = el('div', 'qn-top');
    const close_ = btn('qn-icon', t('Tanca (Esc)', 'Close (Esc)'), close, icon('close', '×'));
    if (!S.note) {
      top.append(el('span', 'qn-spacer'), close_);
      const empty = el('div', 'qn-blank');
      empty.append(el('p', '', t('Tria una nota o comença\'n una de nova.', 'Pick a note or start a new one.')),
        btn('qn-primary', t('Nota nova', 'New note'), () => newNote().catch(e => say(e.message, true))));
      main.append(top, empty, el('p', 'qn-status'));
      main.lastChild.id = 'qnStatus';
      return;
    }
    const n = S.note;
    const proj = el('select', 'qn-project');
    proj.setAttribute('aria-label', t('Projecte de la nota', 'Note project'));
    projectOptions(proj, n.project || '', false);
    proj.onchange = () => { n.project = proj.value; touch(); };
    const mode = el('div', 'qn-seg');
    for (const [m, label] of [['read', t('Llegeix', 'Read')], ['edit', t('Edita', 'Edit')]]) {
      const b = btn(S.mode === m ? 'on' : '', '', () => setMode(m));
      b.textContent = label;
      b.title = label + ' (Ctrl+E)';
      mode.append(b);
    }
    const ai = el('details', 'qn-ai');
    const sum = el('summary', 'qn-ai-btn');
    sum.innerHTML = icon('sparkles', '✦') + '<span>' + t('IA', 'AI') + '</span>';
    ai.append(sum);
    const menu = el('div', 'qn-ai-menu');
    for (const [a, label, help] of [
      ['tidy', t('Neteja', 'Clean up'), t('Ortografia, format i línies repetides', 'Spelling, formatting and duplicates')],
      ['organize', t('Organitza', 'Organize'), t('Seccions i tasques agrupades', 'Sections and grouped tasks')],
      ['summarize', t('Resumeix', 'Summarize'), t('Un resum curt a dalt', 'A short summary on top')],
      ['tasks', t('Extreu tasques', 'Extract tasks'), t('Totes les accions en una llista', 'Every action item in one list')],
    ]) {
      const b = btn('qn-ai-item', '', () => { ai.open = false; runAI(a); });
      b.append(el('b', '', label), el('small', '', help));
      menu.append(b);
    }
    const ask = el('form', 'qn-ai-ask');
    const inp = el('input');
    inp.placeholder = t('Demana-li el que vulguis…', 'Ask for anything…');
    inp.setAttribute('aria-label', t('Instrucció per a la IA', 'Instruction for the AI'));
    const go = btn('qn-ai-go', t('Fes-ho', 'Go'), null, '↵');
    go.type = 'submit';
    ask.append(inp, go);
    ask.onsubmit = e => { e.preventDefault(); if (!inp.value.trim()) return; ai.open = false; runAI('custom', inp.value.trim()); inp.value = ''; };
    menu.append(ask);
    ai.append(menu);
    const pin = btn('qn-icon' + (n.pinned ? ' on' : ''), n.pinned ? t('Desfixa', 'Unpin') : t('Fixa a dalt', 'Pin'), async () => {
      const out = await call('/api/notes', { action: 'pin', id: n.id, pinned: !n.pinned }).catch(e => { say(e.message, true); return null; });
      if (out) { n.pinned = out.pinned; n.updated = out.updated; await loadList(); paintNote(); }
    }, '★');
    const del = btn('qn-icon', t('Esborra la nota', 'Delete note'), async () => {
      if (!confirm(t('Vols esborrar aquesta nota?', 'Delete this note?'))) return;
      clearTimeout(S.timer); S.dirty = false;
      await call('/api/notes', { action: 'delete', id: n.id }).catch(e => say(e.message, true));
      S.note = null;
      await loadList();
      if (S.list[0]) await openNote(S.list[0].id); else paintNote();
    }, '<svg class="ui-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 7h16 M9 7V4h6v3 M6 7l1 13h10l1-13 M10 11v6 M14 11v6"/></svg>');
    if (!n.id) { pin.disabled = true; del.disabled = true; }
    top.append(proj, el('span', 'qn-spacer'), mode, ai, pin, del, close_);

    const title = el('input', 'qn-title');
    title.id = 'qnTitle';
    title.value = n.title || '';
    title.placeholder = t('Títol', 'Title');
    title.oninput = () => { n.title = title.value; touch(); };
    title.onkeydown = e => { if (e.key === 'Enter') { e.preventDefault(); setMode('edit'); } };

    const body = el('div', 'qn-body');
    if (S.proposal != null) {
      const p = el('div', 'qn-proposal');
      const bar = el('div', 'qn-proposal-bar');
      bar.append(el('span', '', t('Proposta de la IA', 'AI suggestion')),
        btn('qn-ghost', t('Descarta', 'Discard'), () => { S.proposal = null; paintNote(); }),
        btn('qn-primary', t('Aplica', 'Apply'), () => { n.body = S.proposal; S.proposal = null; S.mode = 'read'; touch(); paintNote(); }));
      const view = el('div', 'qn-md md');
      view.innerHTML = render(S.proposal);
      p.append(bar, view);
      body.append(p);
    } else if (S.mode === 'edit') {
      const tools = el('div', 'qn-format');
      // Les mateixes icones de traç que la resta de l'app (abans: glifs i un emoji).
      const sv = d => '<svg class="ui-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="' + d + '"/></svg>';
      for (const [k, label, glyph] of [
        ['b', t('Negreta', 'Bold'), sv('M7 5h6a3.5 3.5 0 0 1 0 7H7z M7 12h7a3.5 3.5 0 0 1 0 7H7z')],
        ['i', t('Cursiva', 'Italic'), sv('M14 5h-4 M14 19h-4 M14 5l-4 14')],
        ['h', t('Títol', 'Heading'), sv('M6 5v14 M18 5v14 M6 12h12')],
        ['ul', t('Llista', 'List'), sv('M9 6h11 M9 12h11 M9 18h11 M4.5 6h.01 M4.5 12h.01 M4.5 18h.01')],
        ['task', t('Tasca', 'Task'), sv('M4 5h6v6H4z M5.5 8l1.5 1.5L9.5 6 M13 8h7 M4 15h6v6H4z M13 18h7')],
        ['quote', t('Cita', 'Quote'), sv('M6 6v12 M10 8h9 M10 12h9 M10 16h6')],
        ['code', t('Codi', 'Code'), sv('M9 7l-5 5 5 5 M15 7l5 5-5 5')],
        ['link', t('Enllaç', 'Link'), sv('M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1 M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1')],
      ]) {
        const b = btn('qn-fmt', label, () => format(k), glyph);
        b.onmousedown = e => e.preventDefault();
        tools.append(b);
      }
      const ta = el('textarea', 'qn-editor');
      ta.id = 'qnBody';
      ta.value = n.body || '';
      ta.placeholder = t('Escriu en markdown: ## títols, - llistes, - [ ] tasques, [[Una altra nota]]…', 'Write in markdown: ## headings, - lists, - [ ] tasks, [[Another note]]…');
      ta.spellcheck = true;
      ta.oninput = () => { n.body = ta.value; touch(); };
      body.append(tools, ta);
    } else {
      const view = el('div', 'qn-md md');
      view.innerHTML = (n.body || '').trim() ? render(n.body) : '<p class="qn-placeholder">' + t('Nota buida. Fes doble clic per escriure.', 'Empty note. Double-click to write.') + '</p>';
      // Les tasques es marquen amb un clic; doble clic per editar.
      view.querySelectorAll('input[type=checkbox]').forEach((c, i) => {
        c.disabled = false;
        c.onclick = e => { e.preventDefault(); if (!root.gregalNotes?.toggleTask) return; n.body = root.gregalNotes.toggleTask(n.body, i); touch(); paintNote(); };
      });
      view.querySelectorAll('a.note-link').forEach(a => {
        a.onclick = e => { e.preventDefault(); const hit = S.list.find(x => (x.title || '').toLowerCase() === a.dataset.note.toLowerCase()); if (hit) openNote(hit.id); };
      });
      view.ondblclick = () => setMode('edit');
      body.append(view);
    }
    if (S.busy) {
      const veil = el('div', 'qn-busy');
      veil.append(el('span', 'qn-spin'), el('span', '', t('La IA hi està treballant…', 'The AI is working on it…')));
      body.append(veil);
    }
    const foot = el('div', 'qn-foot');
    const st = el('p', 'qn-status');
    st.id = 'qnStatus';
    foot.append(st, el('span', 'qn-meta', n.updated ? t('Editada ', 'Edited ') + day(n.updated) : ''));
    main.append(top, title, body, foot);
  }

  async function runAI(action, instruction) {
    const n = S.note;
    if (!n || S.busy) return;
    if (!(n.body || '').trim() && action !== 'custom') { say(t('Escriu alguna cosa primer.', 'Write something first.'), true); return; }
    S.busy = true;
    paintNote();
    try {
      const out = await call('/api/notes/ai', { action, instruction: instruction || '', title: n.title || '', body: n.body || '' });
      S.proposal = out.changed ? out.body : null;
      if (!out.changed) say(t('La IA no hi veu res a canviar.', 'Nothing to change.'));
    } catch (e) {
      say(e.status === 400 && /model/.test(e.message) ? t('Configura un model a Preferències per fer servir la IA.', 'Set up a model in Preferences to use AI.') : e.message, true);
    } finally {
      S.busy = false;
      paintNote();
    }
  }

  // ------------------------------------------------------------ obrir

  async function open(opts = {}) {
    const box = shell();
    if (!S.open) {
      S.open = true;
      box.hidden = false;
      document.body.classList.add('qn-on');
      S.returnFocus = document.activeElement;
    }
    await loadProjects();
    await loadList().catch(e => say(e.message, true));
    if (opts.new) return newNote();
    if (S.note) { paintNote(); return; }
    let last = '';
    try { last = localStorage.getItem(LAST) || ''; } catch {}
    const target = S.list.find(n => n.id === last) || S.list[0];
    if (target) await openNote(target.id);
    else await newNote();
  }
  async function close() {
    await flush();
    const box = $('qnPanel');
    if (box) box.hidden = true;
    S.open = false;
    document.body.classList.remove('qn-on');
    if (S.returnFocus?.isConnected) S.returnFocus.focus();
  }
  function toggle() { return S.open ? close() : open().catch(e => say(e.message, true)); }

  // ------------------------------------------------------------ botons

  function makeButton(cls) {
    const b = btn('icon-btn qn-launch ' + (cls || ''), t('Notes (Ctrl+Alt+N)', 'Notes (Ctrl+Alt+N)'), toggle, NOTE_SVG);
    return b;
  }
  function placeButtons() {
    // Codi: a la capçalera, al costat de les altres eines.
    const meta = document.querySelector('#top .top-meta');
    if (meta && !meta.querySelector('.qn-launch')) meta.prepend(makeButton());
    // Vents: la capçalera es torna a pintar a cada vista.
    const vw = document.querySelector('.vw-top');
    if (vw && !vw.querySelector('.qn-launch')) vw.append(makeButton('qn-launch-vw'));
  }
  placeButtons();
  // Només s'observa Vents (on la capçalera es repinta): mirar tot el body
  // despertaria l'observador a cada token d'una resposta.
  const vents = $('ventsPage');
  if (vents) new MutationObserver(placeButtons).observe(vents, { childList: true, subtree: true });

  document.addEventListener('keydown', e => {
    if ((e.ctrlKey || e.metaKey) && e.altKey && !e.shiftKey && (e.key || '').toLowerCase() === 'n') {
      e.preventDefault();
      if (S.open) newNote().catch(err => say(err.message, true)); else open({ new: true }).catch(err => say(err.message, true));
    }
  });
  document.addEventListener('gregal:idioma', () => { if (S.open) { paintList(); paintNote(); } });

  root.gregalQuickNotes = { open, close, toggle, newNote: () => open({ new: true }) };
})(window);
