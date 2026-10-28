// convs: la llista de converses de l'esquerra, com la de ChatGPT o Claude.
//
// Abans era una filera de botons amb el nom del fitxer («sessio-20260915…»)
// i només hi apareixia el que s'havia desat clicant «Nova sessió». Ara el
// servidor desa cada conversa sola en acabar el torn i li posa títol, i
// aquí es veuen agrupades per data, es poden cercar, obrir i esborrar.
const $ = id => document.getElementById(id);
const G = () => window.gregal;
const esc = s => String(s == null ? '' : s)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

const gregalT = k => (typeof window.gregalT === 'function' ? window.gregalT(k) : '');
const resumedSessionsKey = 'gregal_resumed_conversations';

function resumedSessions() {
  try {
    const value = JSON.parse(localStorage.getItem(resumedSessionsKey) || '{}');
    return value && typeof value === 'object' && !Array.isArray(value) ? value : {};
  } catch (e) { return {}; }
}

function saveResumedSession(name, id) {
  try {
    const map = resumedSessions();
    map[name] = id;
    localStorage.setItem(resumedSessionsKey, JSON.stringify(map));
  } catch (e) {}
}

// grup diu a quin bloc va una conversa segons quan es va tocar per últim
// cop. Els mateixos trams que fan servir les altres apps: el que importa és
// trobar de seguida la d'aquest matí.
function grup(at) {
  const d = new Date(at);
  if (isNaN(d)) return { ordre: 9, nom: gregalT('conv.group.older') || 'Older' };
  const avui = new Date(); avui.setHours(0, 0, 0, 0);
  const dies = Math.floor((avui - new Date(d).setHours(0, 0, 0, 0)) / 86400000);
  if (dies <= 0) return { ordre: 0, nom: gregalT('conv.group.today') || 'Today' };
  if (dies === 1) return { ordre: 1, nom: gregalT('conv.group.yesterday') || 'Yesterday' };
  if (dies < 7) return { ordre: 2, nom: gregalT('conv.group.thisWeek') || 'This week' };
  if (dies < 30) return { ordre: 3, nom: gregalT('conv.group.thisMonth') || 'This month' };
  return { ordre: 4, nom: gregalT('conv.group.older') || 'Older' };
}

// xipProjecte diu de quin projecte és la conversa quan no és el d'ara:
// la mateixa llista barreja feina de repositoris diferents i, sense això,
// obrir-ne una canviava el workspace sense que se sabés cap a on.
function xipProjecte(c) {
  const ws = c.workspace || '';
  if (!ws) return '';
  const nom = ws.replace(/[\\/]+$/, '').split(/[\\/]/).pop() || ws;
  const actual = (window.gregalCwd || '').replace(/[\\/]+$/, '');
  if (actual && actual.toLowerCase() === ws.replace(/[\\/]+$/, '').toLowerCase()) return '';
  return '<span class="conv-p" title="' + esc(ws) + '">' + esc(nom) + '</span>';
}

function titolVisible(c) {
  const title = String(c.title || c.name || '');
  if (!/^sessio-\d{8}/i.test(title)) return title;
  const d = new Date(c.at);
  if (isNaN(d)) return gregalT('conv.previousSession') || 'Previous session';
  return (gregalT('inspector.session') || 'Session') + ' · ' + new Intl.DateTimeFormat(undefined, {
    day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit'
  }).format(d);
}

// Cerca local als camps que ja proporciona /api/sessions, inclosa la ruta
// completa per distingir projectes amb noms de carpeta iguals.
export function matchesConversation(c, query) {
  const q = String(query == null ? '' : query).trim().toLowerCase();
  if (!q) return true;
  if (!c || typeof c !== 'object') return false;
  return [c.title, c.name, c.project, c.workspace]
    .some(value => typeof value === 'string' && value.toLowerCase().includes(q));
}
export const convs = {
  items: [],
  projects: [],
  _liveSessions: [],
  _liveSignature: '',
  expandedProjects: new Set(),
  collapsedProjects: new Set(),
  filtre: '',
  loading: false,
  loadError: false,
  init() {
    const cerca = $('convSearch');
    if (cerca) {
      cerca.addEventListener('input', () => { this.filtre = cerca.value.trim().toLowerCase(); this.draw(); });
      cerca.addEventListener('keydown', e => {
        if (e.key === 'Escape') { cerca.value = ''; this.filtre = ''; this.draw(); cerca.blur(); }
      });
    }
    // La llista es construeix aquí: en canviar d'idioma es torna a pintar.
    document.addEventListener('gregal:idioma', () => this.draw());
    this.load();
  },
  async load() {
    this.loading = true;
    this.loadError = false;
    this.draw();
    try {
      const [r, projects] = await Promise.all([G().api('/api/sessions'), G().api('/api/workspaces').then(async r => r.ok ? (await r.json()).workspaces || [] : []).catch(() => [])]);
      this.projects = projects;
      if (r.ok) {
        this.items = (await r.json()) || [];
      } else {
        this.items = [];
        this.loadError = true;
      }
    } catch (e) {
      this.items = [];
      this.loadError = true;
    } finally {
      this.loading = false;
      this.draw();
    }
  },
  updateLiveSessions(list) {
    this._liveSessions = Array.isArray(list) ? list : [];
    const signature = this._liveSessions.map(s => s.id + ':' + (s.busy ? 1 : 0) + ':' + (window.gregalPendingInteractions?.has(s.id) ? 1 : 0)).join('|');
    if (signature !== this._liveSignature) { this._liveSignature = signature; this.draw(); }
  },
  forgetSession(id) {
    const map = resumedSessions();
    let changed = false;
    Object.keys(map).forEach(name => { if (map[name] === id) { delete map[name]; changed = true; } });
    if (changed) { try { localStorage.setItem(resumedSessionsKey, JSON.stringify(map)); } catch (e) {} }
    this.draw();
  },
  draw() {
    const box = $('convs');
    if (!box) return;
    if (this.loading && !this.items.length) {
      box.innerHTML = '<div class="conv-empty">' + esc(gregalT('conv.loading') || 'Loading conversations…') + '</div>';
      return;
    }
    if (this.loadError && !this.items.length) {
      box.innerHTML = '<div class="conv-empty">' + esc(gregalT('conv.error.load') || 'Could not load conversations.') + '</div>';
      return;
    }
    const q = this.filtre;
    const llista = q ? this.items.filter(c => matchesConversation(c, q)) : this.items;
    if (!llista.length) {
      box.innerHTML = '<div class="conv-empty">' +
        esc(q ? (gregalT('conv.empty.search') || 'No conversations match.') : (gregalT('conv.empty.none') || 'No conversations yet.')) + '</div>';
      this.drawProjects(new Map());
      return;
    }
    // Projects have their own section; general conversations stay grouped by date.
    const pins = llista.filter(c => c.pinned && !c.workspace);
    const projectes = new Map();
    const recents = [];
    // Converses d'una carpeta que ja no existeix: no es poden obrir, així que
    // van a un sol grup apartat amb l'opció d'esborrar-les.
    const missing = llista.filter(c => c.missing);
    llista.filter(c => (!c.pinned || c.workspace) && !c.missing).forEach(c => {
      const ws = c.workspace || '';
      if (!ws) { recents.push(c); return; }
      // La ruta completa és la clau: dos repositoris poden acabar amb el
      // mateix nom de carpeta (C:\\a\\app i C:\\b\\app).
      if (!projectes.has(ws)) projectes.set(ws, []);
      projectes.get(ws).push(c);
    });
    const msgsWord = gregalT('conv.messages') || 'messages';
    const delTitle = gregalT('conv.delete.title') || 'Delete this conversation';
    const delAria = gregalT('conv.delete.aria') || 'Delete';
    const pinLabel = gregalT('conv.pin') || 'Pin', unpinLabel = gregalT('conv.unpin') || 'Unpin';
    const expLabel = gregalT('conv.export') || 'Export or share';
    const liveById = new Map(this._liveSessions.map(s => [s.id, s]));
    const resumed = resumedSessions();
    const fila = (c, inProject = false) => {
      const sessionId = c.live_session || resumed[c.name] || (c.current ? window.gregalSession : '');
      const live = sessionId ? liveById.get(sessionId) : null;
      const busy = live ? !!live.busy : !!c.live_busy;
      const pending = !!(sessionId && window.gregalPendingInteractions?.has(sessionId));
      const status = pending ? '<span class="conv-live-state waiting">' + esc(gregalT('sessions.needsInputLabel') || 'Needs input') + '</span>'
        : busy ? '<span class="conv-live-state busy">' + esc(gregalT('sessions.workingLabel') || 'Working') + '</span>' : '';
      return '<div class="conv' + (c.current ? ' current' : '') + '" data-name="' + esc(c.name) + '" title="' +
      esc(titolVisible(c) + ' · ' + c.msgs + ' ' + msgsWord + (c.when ? ' · ' + c.when : '')) + '">' +
      '<span class="conv-t">' + esc(titolVisible(c)) + '</span>' + status + (inProject ? '' : xipProjecte(c)) +
      '<button class="conv-pin' + (c.pinned ? ' pinned' : '') + '" title="' + esc(c.pinned ? unpinLabel : pinLabel) + '" aria-label="' + esc(c.pinned ? unpinLabel : pinLabel) + '">★</button>' +
      '<button class="conv-exp" title="' + esc(expLabel) + '" aria-label="' + esc(expLabel) + '" aria-haspopup="menu">⤓</button>' +
      '<button class="conv-x" title="' + esc(delTitle) + '" aria-label="' + esc(delAria) + '">×</button></div>';
    };
    let html = '';
    if (pins.length) html += '<div class="conv-grup">' + esc(gregalT('conv.pinned') || 'Pinned') + '</div>' + pins.map(fila).join('');
    this.drawProjects(projectes, fila, missing);
    if (recents.length) {
      const grups = new Map();
      recents.forEach(c => { const g = grup(c.at); if (!grups.has(g.nom)) grups.set(g.nom, []); grups.get(g.nom).push(c); });
      for (const [nom, files] of grups) html += '<div class="conv-grup">' + esc(nom) + '</div>' + files.map(fila).join('');
    }
    box.innerHTML = html;
    [...box.querySelectorAll('.conv-more'), ...($('projectList')?.querySelectorAll('.conv-more') || [])].forEach(el => {
      el.onclick = e => {
        e.stopPropagation();
        const path = el.dataset.projectMore;
        if (this.allRows.has(path)) this.allRows.delete(path); else this.allRows.add(path);
        this.draw();
      };
    });
    [...box.querySelectorAll('.conv'), ...($('projectList')?.querySelectorAll('.conv') || [])].forEach(el => {
      el.onclick = e => {
        if (e.target.classList.contains('conv-x')) { this.esborra(el.dataset.name); return; }
        if (e.target.classList.contains('conv-exp')) { window.gregalExport?.openMenu(e.target, el.dataset.name); return; }
        if (e.target.classList.contains('conv-pin')) { this.fixa(el.dataset.name, !e.target.classList.contains('pinned')); return; }
        this.obre(el.dataset.name);
      };
    });
  },
  // Una carpeta amb moltes converses en mostra les més recents: la resta,
  // a un clic. Sense això, un projecte amb 40 sessions omple tota la barra.
  allRows: new Set(),
  projectRows(path, files, row) {
    const limit = 6;
    const all = this.allRows.has(path) || files.length <= limit + 1;
    const shown = all ? files : files.slice(0, limit);
    let html = shown.map(c => row(c, true)).join('');
    if (files.length > limit + 1) {
      const label = all ? (gregalT('conv.less') || 'Show fewer')
        : (gregalT('conv.more') || 'Show {n} more').replace('{n}', files.length - limit);
      html += '<button class="conv-more" data-project-more="' + esc(path) + '">' + esc(label) + '</button>';
    }
    return html;
  },
  drawProjects(grouped, row = () => '', missing = []) {
    const box = $('projectList');
    if (!box) return;
    // Carpetes registrades que ja no existeixen: van amb les converses perdudes.
    const goneFolders = this.projects.filter(p => p.missing && !grouped.has(p.path));
    const projects = new Map(this.projects.filter(p => !p.missing || grouped.has(p.path)).map(p => [p.path, p]));
    for (const path of grouped.keys()) if (!projects.has(path)) projects.set(path, {path});
    const cwd = G().state?.cwd || window.gregalCwd || '';
    const norm = path => path.replace(/[\\/]+$/, '').toLowerCase();
    const sameName = new Map();
    for (const p of projects.values()) { const n = (p.name || p.path.replace(/[\\/]+$/, '').split(/[\\/]/).pop()).toLowerCase(); sameName.set(n, (sameName.get(n) || 0) + 1); }
    // Carpetes temporals (sandboxes, proves): fora de la llista si no són
    // la carpeta actual; la cerca les continua trobant.
    const temp = path => /[\\/]AppData[\\/]Local[\\/]Temp[\\/]|^\/tmp\//i.test(path);
    box.innerHTML = [...projects.values()].filter(p => this.filtre ? (matchesConversation({workspace:p.path, title:p.name}, this.filtre) || grouped.has(p.path)) : (!temp(p.path) || norm(cwd) === norm(p.path))).map(p => {
      const name = p.name || p.path.replace(/[\\/]+$/, '').split(/[\\/]/).pop();
      // Dues carpetes amb el mateix nom (tres «ws»): la carpeta de sobre les distingeix.
      const parent = sameName.get(name.toLowerCase()) > 1 ? p.path.replace(/[\\/]+$/, '').split(/[\\/]/).slice(-2, -1)[0] || '' : '';
      const expanded = (this.expandedProjects.has(p.path) || norm(cwd) === norm(p.path)) && !this.collapsedProjects.has(p.path);
      const files = grouped.get(p.path) || [];
      return '<section class="sidebar-project' + (norm(cwd) === norm(p.path) ? ' selected' : '') + '"><div class="sidebar-project-row"><button class="project-open" data-path="'+esc(p.path)+'" aria-expanded="'+expanded+'" title="'+esc(p.path)+'"><span class="project-chevron" aria-hidden="true">'+(expanded?'⌄':'›')+'</span><svg class="project-folder-ico" viewBox="0 0 16 16" aria-hidden="true" focusable="false"><path d="M1.75 4.5a1 1 0 0 1 1-1h3.1l1.45 1.55h5.95a1 1 0 0 1 1 1v5.45a1 1 0 0 1-1 1h-10.5a1 1 0 0 1-1-1z" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/></svg><span>'+esc(name)+'</span>'+(parent?'<span class="project-parent">'+esc(parent)+'</span>':'')+'</button><button class="project-new" data-path="'+esc(p.path)+'" title="'+esc(gregalT('shell.newProjectChat'))+'" aria-label="'+esc(gregalT('shell.newProjectChat')+' · '+name)+'">＋</button></div>' +
        (expanded ? '<div class="project-conversations"><div class="project-folder" title="'+esc(gregalT('shell.projectFolder'))+'">'+esc(p.path)+'</div>'+this.projectRows(p.path, files, row)+'</div>' : '')+'</section>';
    }).join('') || '<div class="conv-empty">'+esc(gregalT('shell.noProjects'))+'</div>';
    if (missing.length || goneFolders.length) {
      const open = this.allRows.has(':missing');
      box.insertAdjacentHTML('beforeend', '<section class="sidebar-missing"><button class="conv-more missing-toggle" data-project-more=":missing" aria-expanded="' + open + '">' +
        esc((gregalT('conv.missing.title') || 'Folders that no longer exist · {n}').replace('{n}', missing.length + goneFolders.length)) + '</button>' +
        (open ? '<p class="missing-help">' + esc(gregalT('conv.missing.help') || 'These conversations belong to folders that are gone, so they cannot be opened.') + '</p>' +
          '<button class="missing-clean">' + esc(gregalT('conv.missing.clean') || 'Delete them') + '</button>' +
          goneFolders.map(p => '<div class="conv missing folder" title="' + esc(p.path) + '"><span class="conv-t">' + esc(p.name || p.path) + '</span></div>').join('') +
          missing.map(c => '<div class="conv missing" data-name="' + esc(c.name) + '" title="' + esc(c.workspace || '') + '"><span class="conv-t">' + esc(titolVisible(c)) + '</span><span class="conv-ws">' + esc((c.workspace || '').split(/[\\/]/).pop()) + '</span><button class="conv-x" title="' + esc(gregalT('conv.delete.title') || 'Delete this conversation') + '" aria-label="' + esc(gregalT('conv.delete.aria') || 'Delete') + '">×</button></div>').join('') : '') +
        '</section>');
      const clean = box.querySelector('.missing-clean');
      if (clean) clean.onclick = () => this.esborraMissing(missing, goneFolders);
    }
    box.querySelectorAll('.project-open').forEach(b => b.onclick = async () => {
      const path = b.dataset.path;
      if (b.getAttribute('aria-expanded') === 'true') { this.collapsedProjects.add(path); this.draw(); return; }
      this.collapsedProjects.delete(path);
      this.expandedProjects.add(path);
      if (norm(cwd) === norm(path)) { this.draw(); return; }
      const latest = (grouped.get(path) || [])[0];
      if (latest) await this.obre(latest.name);
      else await this.newProjectConversation(path);
    });
    box.querySelectorAll('.project-new').forEach(b => b.onclick = () => this.newProjectConversation(b.dataset.path));
  },
  async newProjectConversation(path) {
    try {
      await window.gregalPanels.sessions.open(path);
      this.expandedProjects.add(path);
      this.collapsedProjects.delete(path);
      G().setView('agent');
      await this.load();
      $('in')?.focus();
    } catch (e) { G().sys(String(e.message || e)); }
  },
  async fixa(name, pinned) {
    try {
      const r = await G().api('/api/sessions/pin', { method: 'POST', body: JSON.stringify({ name, pinned }) });
      if (!r.ok) throw new Error(await r.text());
      await this.load();
    } catch (e) { G().sys((gregalT('conv.pin.error') || 'Could not update the pin: ') + (e.message || e)); }
  },
  _openVersion: 0,
  async obre(name) {
    const version = ++this._openVersion;
    const target = this.items.find(item => item.name === name);
    if (target?.missing) {
      G().sys((gregalT('conv.missing.open') || 'The folder of this conversation no longer exists: {path}').replace('{path}', target.workspace || ''));
      return;
    }
    try {
      const tabs = window.gregalPanels?.sessions;
      const cached = this.items.find(item => item.name === name);
      const known = cached?.live_session || resumedSessions()[name];
      const tab = tabs?._lastList?.find(item => item.id === known && (cached?.live_session === known || !Object.prototype.hasOwnProperty.call(item, 'conversation') || item.conversation === name));
      if (tab) {
        await tabs.switchTo(tab.id);
        return;
      }
      // Refresh the association at click time. The conversation list may have
      // been rendered before the agent autosaved the first user message.
      await this.load();
      if (version !== this._openVersion) return;
      const sessions = window.gregalPanels?.sessions;
      const reopened = resumedSessions();
      const previous = Object.prototype.hasOwnProperty.call(reopened, name) ? reopened[name] : '';
      if (sessions && typeof sessions.list === 'function' && typeof sessions.switchTo === 'function') {
        const live = await sessions.list();
        if (version !== this._openVersion) return;
        // The backend owns this association, so conversations started in a
        // live tab return to that exact tab even before the user has reopened
        // them from the saved list once.
        const linked = this.items.find(item => item.name === name)?.live_session;
        const linkedSession = linked && live.find(s => s && s.id === linked);
        const previousSession = previous && live.find(s => s && s.id === previous);
        const target = linkedSession ? linked
          : previousSession && (!Object.prototype.hasOwnProperty.call(previousSession, 'conversation') || previousSession.conversation === name) ? previous : '';
        if (target) {
          saveResumedSession(name, target);
          await sessions.switchTo(target);
          G().setView('agent');
          return;
        }
        if (previous) {
          delete reopened[name];
          try { localStorage.setItem(resumedSessionsKey, JSON.stringify(reopened)); } catch (e) {}
        }
      }
      // Reprendre sobre una sessió ocupada canviaria el context del worker.
      await sessions.open(G().state.cwd);
      if (version !== this._openVersion) return;
      const targetSession = window.gregalSession || 'default';
      const r = await G().api('/api/resume', { method: 'POST', body: JSON.stringify({ name }), headers: { 'X-Gregal-Session': targetSession } });
      if (version !== this._openVersion) return;
      if (!r.ok) { G().sys((gregalT('conv.open.error') || 'Could not open: ') + await r.text()); return; }
      const d = await r.json();
      if (version !== this._openVersion) return;
      saveResumedSession(name, window.gregalSession || 'default');
      const main = $('main');
      if (main) main.innerHTML = '';
      // Tool calls come back as compact, already finished rows (paired with
      // their result by id), so a resumed conversation reads like it did
      // live instead of jumping from the question to the final answer.
      // Each turn's intermediate work folds into one closed group, as live.
      const cards = new Map();
      let grouped = false, groupTools = 0;
      const closeGroup = () => { if (grouped) G().endWorkGroup({ tools: groupTools }); grouped = false; groupTools = 0; };
      (d.transcript || []).forEach(m => {
        if (m.role === 'user') { closeGroup(); G().add('u', '', G().md(m.content)); return; }
        if (m.role === 'assistant') {
          const calls = Array.isArray(m.tool_calls) ? m.tool_calls : [];
          if (calls.length && G().beginWorkGroup && !grouped) { G().beginWorkGroup(); grouped = true; }
          if (!calls.length) closeGroup();
          if ((m.content || '').trim()) {
            const el = G().add('a', '', G().md(m.content));
            if (calls.length && el) { el.classList.add('working'); G().intoWorkGroup?.(el); }
          }
          groupTools += calls.filter(c => !['todowrite', 'todoread'].includes((c.function || {}).name)).length;
          if (G().toolCard) calls.forEach(c => {
            const fn = c.function || {};
            if (fn.name === 'todowrite' || fn.name === 'todoread') return;
            cards.set(c.id, { card: G().toolCard(fn.name || '', fn.arguments || ''), name: fn.name || '' });
          });
          return;
        }
        if (m.role === 'tool' && G().toolCardDone) {
          const hit = cards.get(m.tool_call_id);
          if (hit) { G().toolCardDone(hit.card, hit.name, m.content || ''); cards.delete(m.tool_call_id); }
        }
      });
      closeGroup();
      // A call without a stored result (turn stopped mid-tool) must not keep spinning.
      cards.forEach(({ card }) => {
        card.classList.remove('pending'); card.classList.add('interrupted');
        const mark = card.querySelector('.tspin'); if (mark) mark.textContent = '■';
      });
      const msgsWord = gregalT('conv.messages') || 'messages';
      G().sys((d.title || name) + ' · ' + d.msgs + ' ' + msgsWord);
      if (d.recovered) G().sys(gregalT('sessions.recoveredRun'));
      G().activityItem('conversa', d.title || name, d.msgs + ' ' + msgsWord, 'ok');
      G().setView('agent');
      await this.load();
      G().refresh();
    } catch (e) { G().sys(String(e.message || e)); }
  },
  async esborraMissing(list, folders = []) {
    const tpl = gregalT('conv.missing.confirm') || 'Delete {n} conversations whose folders no longer exist? This cannot be undone.';
    if (!confirm(tpl.replace('{n}', list.length))) return;
    const failed = [];
    // Les carpetes només s'obliden de la llista (el servidor ho refusa si encara existeixen).
    for (const p of folders) {
      try { await G().api('/api/workspaces?path=' + encodeURIComponent(p.path), { method: 'DELETE' }); } catch (e) {}
    }
    for (const c of list) {
      try {
        const r = await G().api('/api/sessions?name=' + encodeURIComponent(c.name), { method: 'DELETE' });
        if (!r.ok) failed.push(c.name);
      } catch (e) { failed.push(c.name); }
    }
    if (failed.length) G().sys((gregalT('conv.delete.error') || 'Could not delete: ') + failed.length);
    await this.load();
  },
  async esborra(name) {
    const c = this.items.find(x => x.name === name);
    const title = c ? (c.title || c.name) : name;
    const confirmTpl = gregalT('conv.delete.confirm') || 'Delete “{title}”? This cannot be undone.';
    if (!confirm(confirmTpl.replace('{title}', title))) return;
    try {
      const r = await G().api('/api/sessions?name=' + encodeURIComponent(name), { method: 'DELETE' });
      if (!r.ok) { G().sys((gregalT('conv.delete.error') || 'Could not delete: ') + await r.text()); return; }
    } catch (e) { G().sys(String(e.message || e)); return; }
    await this.load();
  },
};
