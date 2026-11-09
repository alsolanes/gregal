// Notes a la web de Vents: el quadern de l'usuari, compartit amb els vents.
// Llista amb cerca, quaderns i etiquetes a l'esquerra; la nota (llegida en
// markdown o en edició) a la dreta. [[Títol]] obre una altra nota (o la crea
// si encara no existeix) i les tasques «- [ ]» es marquen amb un clic.
// Servidor: notes.go (GET/POST /api/notes). winds-workspace.js crida view().
// Tot el text de l'usuari va per textContent o pel markdown segur (md.js).
(function (root) {
  'use strict';

  // ---------------------------------------------------------------- pures

  const LINK = /\[\[([^\[\]\n|]{1,120})(?:\|([^\[\]\n]{0,120}))?\]\]/g;
  const fold = s => String(s || '').normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().trim();

  /** Posa enllaços de nota a l'HTML de md.js, fora del codi. El text ja
   *  arriba escapat (md.js no deixa passar cap < > " &), així que va tal qual
   *  a l'atribut i el navegador el torna a llegir com el títol original. */
  function linkNotes(html) {
    return String(html).split(/(<pre[\s\S]*?<\/pre>|<code>[\s\S]*?<\/code>)/).map((part, i) => i % 2 ? part : part.replace(LINK, (m, title, label) =>
      '<a href="#" class="note-link" data-note="' + title.trim() + '">' + (label || title).trim() + '</a>')).join('');
  }

  /** Marca o desmarca la tasca número i (0 = la primera) del text. */
  function toggleTask(body, index) {
    let n = -1, inFence = false;
    return String(body).split('\n').map(line => {
      if (/^\s*```/.test(line)) inFence = !inFence;
      if (inFence) return line;
      const m = /^(\s*[-*+] \[)([ xX])(\].*)$/.exec(line);
      if (!m) return line;
      n++;
      return n === index ? m[1] + (m[2] === ' ' ? 'x' : ' ') + m[3] : line;
    }).join('\n');
  }

  /** «avui», «ahir» o la data curta. */
  function dayText(iso, lang, now = new Date()) {
    const d = new Date(iso);
    if (!iso || isNaN(d)) return '';
    const day = x => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
    const diff = Math.round((day(now) - day(d)) / 86400000);
    if (diff === 0) return d.toLocaleTimeString(lang === 'ca' ? 'ca-ES' : 'en-GB', { hour: '2-digit', minute: '2-digit' });
    if (diff === 1) return lang === 'ca' ? 'ahir' : 'yesterday';
    return d.toLocaleDateString(lang === 'ca' ? 'ca-ES' : 'en-GB', { day: 'numeric', month: 'short', ...(d.getFullYear() !== now.getFullYear() ? { year: 'numeric' } : {}) });
  }

  /** «música, #pendents , música» → ['música','pendents']. */
  function parseTags(s) {
    const seen = new Set(), out = [];
    for (let x of String(s || '').split(/[,;\n]+/)) {
      x = x.trim().replace(/^#/, '').trim().split(/\s+/).join('-');
      if (x && !seen.has(fold(x))) { seen.add(fold(x)); out.push(x); }
    }
    return out;
  }

  const isoDay = d => d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0');

  /** Reparteix l'agenda: vençudes, avui, aquesta setmana (7 dies) i més endavant. */
  function agendaGroups(items, now = new Date()) {
    const today = isoDay(now), week = isoDay(new Date(now.getFullYear(), now.getMonth(), now.getDate() + 7));
    const g = { overdue: [], today: [], week: [], later: [] };
    for (const it of items || []) {
      if (!it.due) continue;
      (it.due < today ? g.overdue : it.due === today ? g.today : it.due <= week ? g.week : g.later).push(it);
    }
    return g;
  }

  /** Plantilla d'una reunió: data d'avui i una tasca amb termini d'aquí a una setmana. */
  function meetingTemplate(lang, now = new Date()) {
    const isCa = lang === 'ca', next = isoDay(new Date(now.getFullYear(), now.getMonth(), now.getDate() + 7));
    const title = (isCa ? 'Reunió ' : 'Meeting ') + now.toLocaleDateString(isCa ? 'ca-ES' : 'en-GB', { day: 'numeric', month: 'short' });
    const body = isCa
      ? '**Data:** ' + isoDay(now) + ' · **Amb:** \n\n## Notes\n- \n\n## Decisions\n- \n\n## Tasques\n- [ ]  @' + next + '\n'
      : '**Date:** ' + isoDay(now) + ' · **With:** \n\n## Notes\n- \n\n## Decisions\n- \n\n## Action items\n- [ ]  @' + next + '\n';
    return { title, body, notebook: isCa ? 'Reunions' : 'Meetings' };
  }

  const pure = { linkNotes, toggleTask, dayText, parseTags, fold, agendaGroups, meetingTemplate, isoDay };
  if (typeof module !== 'undefined' && module.exports) { module.exports = pure; return; }

  // ---------------------------------------------------------------- vista

  const ca = () => document.documentElement.lang === 'ca';
  // {vent}/{vents} com a Android: aquí es tornen «vent(s)»/«wind(s)» i
  // gregalCollective hi posa el nom que hagi triat l'usuari.
  const t = (a, b) => {
    const v = (ca() ? a : b).replace(/\{vents\}/g, ca() ? 'vents' : 'winds').replace(/\{vent\}/g, ca() ? 'vent' : 'wind');
    return root.gregalCollective ? root.gregalCollective.apply(v) : v;
  };
  const el = (tag, cls, text) => { const n = document.createElement(tag); if (cls) n.className = cls; if (text !== undefined) n.textContent = text; return n; };
  const btn = (label, action, cls = 'vw-button') => { const n = el('button', cls, label); n.type = 'button'; n.onclick = () => Promise.resolve().then(action).catch(fail); return n; };
  // Les icones de traç de Vents (winds-workspace.js); sense elles, només el text.
  const ic = name => (root.gregalWindIcon ? root.gregalWindIcon(name) : document.createTextNode(''));
  const ibtn = (name, label, action, cls) => { const n = btn('', action, cls); n.append(ic(name), el('span', '', label)); return n; };
  async function call(url, method = 'GET', body) {
    const r = await root.api(url, { method, ...(body === undefined ? {} : { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }) });
    const text = await r.text();
    let data = null; try { data = JSON.parse(text); } catch { }
    if (!r.ok) { const e = new Error((data && data.error) || text.trim() || ('HTTP ' + r.status)); e.status = r.status; e.data = data; throw e; }
    return data;
  }

  // Marca petita (fixada, privada, bloquejada, amb avís): icona amb nom per al lector de pantalla.
  const flag = (name, label) => { const f = el('span', 'notes-flag'); f.append(ic(name)); f.title = label; f.setAttribute('role', 'img'); f.setAttribute('aria-label', label); return f; };

  const S = { q: '', tag: '', notebook: '', pinned: false, list: null, open: null, backlinks: [], editing: false, draft: null, box: null, opts: {}, epoch: 0, timer: 0 };
  let status = null;
  function fail(e) { if (status) { status.textContent = t('No s’ha pogut fer: ', 'Couldn’t do it: ') + (e.message || e); status.setAttribute('role', 'alert'); } }
  function say(text) { if (status) { status.textContent = text; status.setAttribute('role', 'status'); } }

  async function loadList() {
    const p = new URLSearchParams();
    if (S.q.trim()) p.set('q', S.q.trim());
    if (S.tag) p.set('tag', S.tag);
    if (S.notebook) p.set('notebook', S.notebook);
    const epoch = ++S.epoch;
    const data = await call('/api/notes' + (p.toString() ? '?' + p : ''));
    if (epoch !== S.epoch) return;
    S.list = data;
    paintList();
    if (!S.open && !S.editing) paintNote();
  }

  async function openNote(id) {
    const data = await call('/api/notes?id=' + encodeURIComponent(id));
    S.open = data.note; S.backlinks = data.backlinks || []; S.editing = false;
    paintNote();
    S.box?.classList.add('has-note');
  }

  function newNote(title = '', tpl = null) {
    S.open = null; S.backlinks = [];
    S.draft = { title, body: '', notebook: S.notebook || '', tags: S.tag ? [S.tag] : [], private: false, locked: false, ...(tpl || {}) };
    S.editing = true; paintNote(); S.box?.classList.add('has-note');
    S.box?.querySelector('.notes-edit-body')?.focus();
  }

  function closeNote() { S.open = null; S.editing = false; S.box?.classList.remove('has-note'); paintNote(); paintList(); }

  async function openByTitle(title) {
    const all = await call('/api/notes?q=' + encodeURIComponent(title));
    const hit = (all.notes || []).find(n => fold(n.title) === fold(title));
    if (hit) return openNote(hit.id);
    newNote(title);
  }

  function chips(parent) {
    const d = S.list || {};
    const row = el('div', 'notes-chips');
    const chip = (label, on, fn) => { const c = btn(label, fn, 'act-chip' + (on ? ' on' : '')); c.setAttribute('aria-pressed', String(on)); row.append(c); };
    chip(t('Totes', 'All'), !S.tag && !S.notebook && !S.pinned, () => { S.tag = ''; S.notebook = ''; S.pinned = false; loadList(); });
    if ((d.notes || []).some(n => n.pinned) || S.pinned) { chip(t('Fixades', 'Pinned'), S.pinned, () => { S.pinned = !S.pinned; paintList(); }); row.lastChild.prepend(ic('pin')); }
    for (const nb of (d.notebooks || []).slice(0, 12)) { chip(nb.name, S.notebook === nb.name, () => { S.notebook = S.notebook === nb.name ? '' : nb.name; loadList(); }); row.lastChild.prepend(ic('notebook')); }
    for (const tg of (d.tags || []).slice(0, 16)) chip('#' + tg.name, fold(S.tag) === fold(tg.name), () => { S.tag = fold(S.tag) === fold(tg.name) ? '' : tg.name; loadList(); });
    if (row.children.length > 1) parent.append(row);
  }

  function noteMeta(n) {
    const lang = ca() ? 'ca' : 'en';
    const bits = [];
    if (n.notebook) bits.push(n.notebook);
    if (n.tasks_total) bits.push((n.tasks_total - n.tasks_open) + '/' + n.tasks_total + ' ' + t('tasques', 'tasks'));
    bits.push(dayText(n.updated, lang));
    if (n.edited_by) bits.push(t('per ', 'by ') + n.edited_by);
    return bits.filter(Boolean).join(' · ');
  }

  function paintList() {
    const side = S.box?.querySelector('.notes-side');
    if (!side) return;
    const list = side.querySelector('.notes-list');
    side.querySelector('.notes-chips')?.remove();
    chips(side.querySelector('.notes-filters'));
    list.replaceChildren();
    if (!S.list) { list.append(el('p', 'vw-muted', t('Carregant…', 'Loading…'))); return; }
    if (!S.q.trim() && !S.tag && !S.notebook && !S.pinned) agenda(list);
    let notes = S.list.notes || [];
    if (S.pinned) notes = notes.filter(n => n.pinned);
    if (!notes.length) {
      const empty = el('div', 'notes-empty');
      if (!S.list.total) {
        empty.append(el('strong', '', t('Encara no tens cap nota', 'No notes yet')),
          el('p', '', t('Escriu idees, llistes o el que vulguis recordar. Els {vents} les poden llegir i, si els ho demanes, apuntar-hi coses: «apunta-ho a la nota de la compra».',
            'Write down ideas, lists or anything you want to remember. Your {vents} can read them and, when you ask, add to them: “put it on my shopping list note”.')),
          ibtn('plus', t('Primera nota', 'First note'), () => newNote(), 'vw-primary'));
      } else empty.append(el('p', 'vw-muted', t('Cap nota no hi coincideix.', 'No notes match.')));
      list.append(empty);
      return;
    }
    for (const n of notes) {
      const row = el('button', 'notes-row');
      row.type = 'button';
      row.setAttribute('aria-current', String(S.open?.id === n.id));
      const title = el('strong', '', n.title || t('Sense títol', 'Untitled'));
      const head = el('span', 'notes-row-head');
      if (n.pinned) head.append(flag('pin', t('Fixada', 'Pinned')));
      head.append(title);
      if (n.private) head.append(flag('eyeOff', t('Només per a tu', 'Only you')));
      if (n.locked) head.append(flag('lock', t('Bloquejada', 'Locked')));
      row.append(head);
      const text = n.snippet || n.excerpt;
      if (text) row.append(el('span', 'notes-excerpt', text));
      const tags = [...(n.tags || []), ...(n.hashtags || [])];
      const meta = el('small', '', noteMeta(n) + (tags.length ? ' · #' + [...new Set(tags)].slice(0, 4).join(' #') : ''));
      row.append(meta);
      row.onclick = () => openNote(n.id).catch(fail);
      list.append(row);
    }
  }

  /** L'agenda: les tasques amb data de totes les notes, marcables aquí mateix. */
  function agenda(parent) {
    const g = agendaGroups(S.list?.agenda);
    const groups = [['overdue', t('Vençudes', 'Overdue')], ['today', t('Avui', 'Today')], ['week', t('Aquesta setmana', 'This week')], ['later', t('Més endavant', 'Later')]];
    if (!groups.some(([k]) => g[k].length)) return;
    const box = el('div', 'notes-agenda');
    box.append(el('h3', '', t('Agenda', 'Agenda')));
    let shown = 0;
    for (const [k, label] of groups) {
      if (!g[k].length || shown >= 10) continue;
      box.append(el('p', 'notes-agenda-group notes-agenda-' + k, label));
      for (const it of g[k].slice(0, 10 - shown)) {
        shown++;
        const row = el('div', 'notes-agenda-row');
        const cb = el('input'); cb.type = 'checkbox'; cb.setAttribute('aria-label', t('Fet', 'Done') + ': ' + it.text);
        cb.onchange = () => tickAgenda(it).catch(fail);
        const txt = btn('', () => openNote(it.note_id), 'notes-agenda-text');
        const when = k === 'today' ? '' : dayText(it.due + 'T12:00:00', ca() ? 'ca' : 'en');
        const sub = el('small', '', [when, it.time, it.note_title].filter(Boolean).join(' · '));
        if (it.remind_at) { const bell = flag('bell', t('Amb avís', 'With a reminder')); sub.append(bell); }
        txt.append(el('span', '', it.text || it.note_title), sub);
        row.append(cb, txt);
        box.append(row);
      }
    }
    parent.append(box);
  }

  async function tickAgenda(it) {
    const data = await call('/api/notes?id=' + encodeURIComponent(it.note_id));
    const n = data.note;
    await call('/api/notes', 'POST', { action: 'save', id: n.id, body: toggleTask(n.body, it.index), base: n.updated });
    say(t('Fet ✓', 'Done ✓'));
    if (S.open?.id === n.id) await openNote(n.id);
    await loadList();
  }

  /** Sense cap nota oberta: el resum, capturar, reunió nova i ordenar amb un vent. */
  function dashboard(main) {
    const d = el('div', 'notes-dash');
    const g = agendaGroups(S.list?.agenda);
    const stats = el('div', 'notes-stats');
    const stat = (n, label) => { const c = el('div', 'notes-stat'); c.append(el('strong', '', String(n)), el('span', '', label)); stats.append(c); };
    stat(S.list?.total ?? 0, t('notes', 'notes'));
    stat(g.overdue.length, t('vençudes', 'overdue'));
    stat(g.today.length, t('per avui', 'due today'));
    stat(g.week.length, t('aquesta setmana', 'this week'));
    d.append(stats);
    const acts = el('div', 'notes-actions notes-dash-actions');
    acts.append(ibtn('calendar', t('Reunió', 'Meeting'), () => newNote('', meetingTemplate(ca() ? 'ca' : 'en')), 'vw-button'));
    if (S.opts.ask) {
      acts.append(
        ibtn('sparkles', t('Organitza amb un {vent}', 'Organise with a {vent}'), () => S.opts.ask(t(
          'Mira les meves notes (eina notes) i ajuda’m a organitzar-les: proposa quaderns i etiquetes, junta les repetides, treu les tasques de les notes de reunions amb data (@AAAA-MM-DD) i digue’m què tinc pendent aquesta setmana. Abans de canviar res, ensenya’m el pla.',
          'Look at my notes (notes tool) and help me organise them: suggest notebooks and tags, merge duplicates, pull dated action items (@YYYY-MM-DD) out of meeting notes and tell me what is due this week. Show me the plan before changing anything.')), 'vw-button'),
        ibtn('sun', t('Què tinc avui?', 'What’s on today?'), () => S.opts.ask(t(
          'Mira la meva agenda de notes (eina notes, acció agenda) i fes-me un resum curt del que tinc vençut, per avui i aquesta setmana. Ofereix-te a recordar-me les tasques amb hora.',
          'Check my notes agenda (notes tool, agenda action) and give me a short summary of what is overdue, due today and this week. Offer to remind me of the tasks that have a time.')), 'vw-button'));
    }
    d.append(acts);
    d.append(el('p', 'notes-tip', t('Consell: escriu «- [ ] envia el pressupost @2026-11-12 10:00» i la tasca sortirà a l’agenda; un {vent} t’avisarà a aquella hora (a les 9:00 si no en té).',
      'Tip: write “- [ ] send the budget @2026-11-12 10:00” and the task shows up in the agenda; a {vent} reminds you at that time (at 9:00 if there’s no time).')));
    main.append(d);
  }

  function paintNote() {
    const main = S.box?.querySelector('.notes-main');
    if (!main) return;
    main.replaceChildren();
    if (S.editing) return paintEditor(main);
    const n = S.open;
    if (!n) {
      dashboard(main);
      return;
    }
    const bar = el('div', 'notes-bar');
    bar.append(ibtn('back', t('Notes', 'Notes'), closeNote, 'vw-button notes-back'));
    const actions = el('div', 'notes-actions');
    actions.append(
      btn(t('Edita', 'Edit'), () => { S.draft = { ...n, tags: [...(n.tags || [])] }; S.editing = true; paintNote(); }, 'vw-primary'),
      btn(n.pinned ? t('Desfixa', 'Unpin') : t('Fixa', 'Pin'), async () => { await call('/api/notes', 'POST', { action: 'pin', id: n.id, pinned: !n.pinned }); n.pinned = !n.pinned; paintNote(); loadList(); }),
    );
    if (!n.private && S.opts.ask) actions.append(btn(t('Pregunta a un {vent}', 'Ask a {vent}'), () => S.opts.ask(t('Llegeix la meva nota «' + n.title + '» i ', 'Read my note “' + n.title + '” and '))));
    actions.append(btn(t('Esborra', 'Delete'), async () => {
      if (!confirm(t('Esborrar «' + n.title + '»?', 'Delete “' + n.title + '”?'))) return;
      await call('/api/notes', 'POST', { action: 'delete', id: n.id });
      say(t('Nota esborrada.', 'Note deleted.')); closeNote(); loadList();
    }, 'vw-button vw-archive'));
    bar.append(actions);
    main.append(bar);
    main.append(el('h2', 'notes-title', n.title || t('Sense títol', 'Untitled')));
    const meta = el('p', 'notes-meta');
    const bits = [noteMeta(n)];
    if (n.by && n.by !== n.edited_by) bits.push(t('creada per ', 'created by ') + n.by);
    if (n.private) bits.push(t('només per a tu: els {vents} no la veuen', 'only you: the {vents} can’t see it'));
    if (n.locked) bits.push(t('els {vents} no la poden canviar', 'the {vents} can’t change it'));
    meta.textContent = bits.join(' · ');
    main.append(meta);
    const tags = [...new Set([...(n.tags || []), ...(n.hashtags || [])])];
    if (tags.length) {
      const row = el('div', 'notes-tags');
      for (const tg of tags) row.append(btn('#' + tg, () => { S.tag = tg; S.notebook = ''; loadList(); }, 'notes-tag'));
      main.append(row);
    }
    const body = el('div', 'vw-result-text notes-body');
    if (root.gregalMD) body.innerHTML = linkNotes(root.gregalMD.render(n.body || ''));
    else body.textContent = n.body || '';
    // Les tasques es marquen aquí mateix; els enllaços [[…]] obren la nota.
    body.querySelectorAll('input[type=checkbox]').forEach((box, i) => {
      box.disabled = false;
      box.onchange = () => save({ id: n.id, body: toggleTask(n.body, i), base: n.updated }, true).catch(fail);
    });
    body.addEventListener('click', e => {
      const a = e.target.closest('a.note-link');
      if (!a) return;
      e.preventDefault();
      openByTitle(a.dataset.note).catch(fail);
    });
    main.append(body);
    if (!n.body?.trim()) body.append(el('p', 'vw-muted', t('Nota buida.', 'Empty note.')));
    if (S.backlinks.length) {
      const back = el('div', 'notes-backlinks');
      back.append(el('h3', '', t('L’enllacen', 'Linked from')));
      for (const r of S.backlinks) back.append(btn(r.title, () => openNote(r.id), 'notes-tag'));
      main.append(back);
    }
  }

  function paintEditor(main) {
    const d = S.draft;
    const bar = el('div', 'notes-bar');
    bar.append(ibtn('back', t('Cancel·la', 'Cancel'), () => { S.editing = false; if (!S.open) closeNote(); else paintNote(); }, 'vw-button notes-back notes-cancel'));
    const form = el('div', 'notes-editor');
    const title = el('input', 'notes-edit-title'); title.value = d.title || ''; title.placeholder = t('Títol', 'Title'); title.setAttribute('aria-label', t('Títol', 'Title'));
    const body = el('textarea', 'notes-edit-body'); body.value = d.body || ''; body.placeholder = t('Escriu en markdown… #etiqueta, [[Una altra nota]], - [ ] tasca', 'Write in markdown… #tag, [[Another note]], - [ ] task');
    body.setAttribute('aria-label', t('Text de la nota', 'Note text'));
    const row = el('div', 'notes-edit-row');
    const nbWrap = el('label', 'vw-field', t('Quadern', 'Notebook')), nb = el('input'); nb.value = d.notebook || '';
    const dl = el('datalist'); dl.id = 'notesNotebooks'; for (const x of S.list?.notebooks || []) { const o = el('option'); o.value = x.name; dl.append(o); }
    nb.setAttribute('list', dl.id); nbWrap.append(nb, dl);
    const tgWrap = el('label', 'vw-field', t('Etiquetes', 'Tags')), tg = el('input'); tg.value = (d.tags || []).join(', '); tg.placeholder = t('feina, idees', 'work, ideas'); tgWrap.append(tg);
    row.append(nbWrap, tgWrap);
    const check = (label, on) => { const l = el('label', 'vw-check'), c = el('input'); c.type = 'checkbox'; c.checked = !!on; l.append(c, el('span', '', label)); return [l, c]; };
    const [privL, priv] = check(t('Només per a mi: els {vents} no la veuen', 'Only me: the {vents} can’t see it'), d.private);
    const [lockL, lock] = check(t('Bloquejada: els {vents} la poden llegir però no canviar', 'Locked: the {vents} can read it but not change it'), d.locked);
    const [remL, rem] = check(t('Avisa’m de les tasques amb data (a l’hora, o a les 9:00)', 'Remind me of dated tasks (at their time, or at 9:00)'), !d.no_remind);
    const doSave = force => save({
      id: S.open?.id || '', title: title.value, body: body.value, notebook: nb.value, tags: parseTags(tg.value),
      private: priv.checked, locked: lock.checked, no_remind: !rem.checked, base: force ? '' : (S.open?.updated || ''),
    });
    const actions = el('div', 'notes-actions');
    actions.append(btn(t('Desa', 'Save'), () => doSave(false), 'vw-primary'));
    bar.append(actions);
    form.append(title, body, row, remL, privL, lockL);
    form.addEventListener('keydown', e => { if ((e.ctrlKey || e.metaKey) && e.key === 's') { e.preventDefault(); doSave(false).catch(fail); } });
    S.forceSave = () => doSave(true);
    main.append(bar, form);
  }

  async function save(payload, quiet) {
    let n;
    try { n = await call('/api/notes', 'POST', { action: 'save', ...payload }); }
    catch (e) {
      if (e.status !== 409) throw e;
      // Un {vent} (o una altra pestanya) l'ha tocada mentre l'editaves.
      if (quiet || !confirm(t('Aquesta nota ha canviat mentre l’editaves (potser un {vent} hi ha afegit alguna cosa). Vols desar la teva versió igualment? Cancel·la per veure la versió nova.',
        'This note changed while you were editing it (maybe a {vent} added something). Save your version anyway? Cancel to see the new version.'))) {
        S.open = e.data?.note || S.open; S.editing = false; paintNote(); loadList();
        say(t('La nota ha canviat: aquesta és la versió nova.', 'The note changed: this is the new version.'));
        return;
      }
      return S.forceSave?.();
    }
    S.open = n; S.editing = false;
    if (!quiet) say(t('Desat', 'Saved'));
    const data = await call('/api/notes?id=' + encodeURIComponent(n.id)).catch(() => null);
    if (data) { S.open = data.note; S.backlinks = data.backlinks || []; }
    paintNote(); loadList();
  }

  /** Pinta la secció de notes dins de content. opts.ask(text) obre el xat amb text. */
  function view(content, opts = {}) {
    S.opts = opts;
    const head = el('div', 'vw-heading');
    head.append(el('h1', '', t('Notes', 'Notes')),
      el('p', '', t('El teu quadern, compartit amb els {vents}: et poden llegir les notes i, si els ho demanes, crear-ne o afegir-hi coses.',
        'Your notebook, shared with your {vents}: they can read your notes and, when you ask, create them or add to them.')));
    content.append(head);
    const box = el('div', 'notes-wrap' + (S.open || S.editing ? ' has-note' : ''));
    const side = el('section', 'notes-side');
    const filters = el('div', 'notes-filters');
    const top = el('div', 'notes-top');
    const search = el('input', 'vw-search'); search.type = 'search'; search.value = S.q;
    search.placeholder = t('Cerca a les notes', 'Search notes'); search.setAttribute('aria-label', search.placeholder);
    search.oninput = () => { S.q = search.value; clearTimeout(S.timer); S.timer = setTimeout(() => loadList().catch(fail), 200); };
    top.append(search, ibtn('plus', t('Nota', 'Note'), () => newNote(), 'vw-primary notes-new'));
    filters.append(top);
    // Captura ràpida: escrius i Enter, sense obrir res. La primera línia fa de títol.
    const quick = el('input', 'vw-search notes-quick');
    quick.placeholder = t('Apunta ràpid… (Enter desa)', 'Quick note… (Enter saves)');
    quick.setAttribute('aria-label', quick.placeholder);
    quick.onkeydown = e => {
      if (e.key !== 'Enter' || !quick.value.trim()) return;
      e.preventDefault();
      const text = quick.value.trim(); quick.value = '';
      call('/api/notes', 'POST', { action: 'save', body: text, notebook: S.notebook || '', tags: S.tag ? [S.tag] : [] })
        .then(() => { say(t('Apuntat ✓', 'Noted ✓')); return loadList(); })
        .catch(err => { quick.value = text; fail(err); });
    };
    filters.prepend(quick);
    side.append(filters, el('div', 'notes-list'));
    const main = el('section', 'notes-main');
    box.append(side, main);
    status = el('p', 'vw-status notes-status');
    content.append(box, status);
    S.box = box;
    paintList(); paintNote();
    loadList().catch(fail);
    if (opts.open) openNote(opts.open).catch(fail);
  }

  /** Posa el cursor a la captura ràpida (drecera Ctrl+Alt+N de l'escriptori). */
  function focusCapture() {
    const q = S.box?.querySelector('.notes-quick');
    if (!q) return;
    if (S.open || S.editing) closeNote();
    q.focus();
  }

  root.gregalNotes = { view, newNote: () => newNote(), focusCapture, ...pure };
})(typeof globalThis !== 'undefined' ? globalThis : this);
