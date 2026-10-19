// Gregal 1.1 — panells de l'escriptori.
//
// index.html continua sent la conversa; aquí hi ha el que converteix la
// finestra en una eina de treball: pestanyes de sessió, revisió de canvis
// amb accepta/descarta per hunk, arbre de fitxers, terminal de processos
// llargs i un composer amb ordres / i mencions @.
//
// El pont amb la pàgina és window.gregal (api, refresh, sys, activityItem…).

import { office } from './office.js';
import { convs } from './convs.js';
import { prefs } from './prefs.js';
import { attach } from './attach.js';
import { aplica as aplicaIdioma, t as T, defineixPerDefecte, idioma } from './i18n.js';
import { flows } from './flows.js';
import { workshop } from './workshop.js';

const G = () => window.gregal;
const $ = id => document.getElementById(id);
const esc = s => String(s == null ? '' : s)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

async function j(url, opts) {
  const r = await G().api(url, opts);
  if (!r.ok) throw new Error((await r.text()) || ('HTTP ' + r.status));
  return r.json();
}

function renderSessionTranscript(transcript, busy) {
  const cards = new Map();
  let grouped = false, groupTools = 0, lastAssistantText = '', toolCount = 0, activeGroup = null;
  const closeGroup = failed => {
    if (grouped) G().endWorkGroup({ tools: groupTools, failed: !!failed });
    grouped = false; activeGroup = null;
    groupTools = 0;
  };
  (Array.isArray(transcript) ? transcript : []).forEach(m => {
    if (!m || typeof m !== 'object') return;
    if (m.role === 'user') {
      closeGroup(false);
      if ((m.content || '').trim()) G().add('u', '', G().md(m.content));
      return;
    }
    if (m.role === 'assistant') {
      const calls = Array.isArray(m.tool_calls) ? m.tool_calls : [];
      if (calls.length && !grouped && G().beginWorkGroup) { activeGroup = G().beginWorkGroup(); grouped = true; }
      if (!calls.length) closeGroup(false);
      if ((m.content || '').trim()) {
        lastAssistantText = m.content.trim();
        const el = G().add('a', '', G().md(m.content));
        if (calls.length && el) G().intoWorkGroup?.(el);
      }
      calls.forEach(call => {
        const fn = call && call.function || {};
        const card = G().toolCard?.(fn.name || '', fn.arguments || '');
        if (card) G().intoWorkGroup?.(card);
        if (call && call.id) cards.set(call.id, { card, name: fn.name || '' });
        groupTools++; toolCount++;
      });
      return;
    }
    if (m.role === 'tool') {
      const hit = cards.get(m.tool_call_id);
      if (hit) {
        G().toolCardDone?.(hit.card, hit.name || m.name || '', m.content || '');
        cards.delete(m.tool_call_id);
      } else {
        G().toolCardDone?.(null, m.name || 'tool', m.content || '');
      }
    }
  });
  const pendingCards = Array.from(cards.values()).map(hit => hit.card).filter(Boolean);
  if (busy) {
    if (!grouped && G().beginWorkGroup) { activeGroup = G().beginWorkGroup(); grouped = true; }
  } else {
    pendingCards.forEach(card => {
      card.classList?.remove('pending'); card.classList?.add('interrupted');
      const mark = card.querySelector?.('.tspin'); if (mark) mark.textContent = '■';
    });
    closeGroup(pendingCards.length > 0);
  }
  return { pendingCards, lastAssistantText, steps: toolCount, tools: toolCount, workGroup: activeGroup };
}

// Quan una sessió encara treballa, la conversa desada només té el missatge
// de l'usuari. Reconstruïm el tram visible a partir dels events durables del
// torn perquè canviar de pestanya no faci desaparèixer el progrés.
function renderActiveRunEvents(events) {
  const pending = new Map();
  let grouped = false, activeGroup = null, toolCount = 0, lastAssistantText = '';
  let tokenText = '';
  const flushTokens = () => {
    if (!tokenText) return;
    const text = tokenText; tokenText = '';
    const el = G().add('a', '', G().md(text));
    if (grouped && el) G().intoWorkGroup?.(el);
    lastAssistantText = text.trim();
  };
  const payload = e => {
    if (!e || !e.payload) return {};
    if (typeof e.payload === 'object') return e.payload;
    try { return JSON.parse(e.payload); } catch (x) { return {}; }
  };
  (Array.isArray(events) ? events : []).forEach(e => {
    if (!e || !e.kind) return;
    if (e.kind === 'progress' || e.kind === 'run_queued' || e.kind === 'run_started') return;
    const d = payload(e);
    if (e.kind === 'token') { tokenText += e.text || ''; return; }
    flushTokens();
    if (e.kind === 'thinking') {
      const text = String(d.text || e.text || '').replace(/^💭\s*/, '').trim();
      if (text) {
        const row = G().add('t', '', '<details class="tool"><summary>' + esc(T('ui.thinking')) + '</summary><div class="out">' + G().md(text.slice(0, 2000)) + '</div></details>');
        if (grouped && row) G().intoWorkGroup?.(row);
      }
    } else if (e.kind === 'tool_call') {
      if (!grouped && G().beginWorkGroup) { activeGroup = G().beginWorkGroup(); grouped = true; }
      const name = String(d.name || e.text || T('actk.tool'));
      const card = G().toolCard?.(name, d.args || '');
      if (card) G().intoWorkGroup?.(card);
      const list = pending.get(name) || []; list.push(card); pending.set(name, list);
      toolCount++;
    } else if (e.kind === 'tool_result') {
      const name = String(d.name || T('actk.tool'));
      const list = pending.get(name) || [];
      G().toolCardDone?.(list.shift() || null, name, d.output || e.text || '');
      if (list.length) pending.set(name, list); else pending.delete(name);
    } else if (e.kind === 'text') {
      const text = String(e.text || d.text || '').trim();
      if (text && text !== lastAssistantText) {
        const el = G().add('a', '', G().md(text));
        if (grouped && el) G().intoWorkGroup?.(el);
        lastAssistantText = text;
      }
    } else if (e.kind === 'error' || e.kind === 'blocked' || e.kind === 'review' || e.kind === 'activity') {
      const text = String(e.text || '').trim();
      if (text) {
        const row = G().add('t', '', '<div class="tool">' + esc(text) + '</div>');
        if (grouped && row) G().intoWorkGroup?.(row);
      }
    }
  });
  flushTokens();
  const pendingCards = Array.from(pending.values()).flat().filter(Boolean);
  return { pendingCards, lastAssistantText, steps: toolCount, tools: toolCount, workGroup: activeGroup };
}

async function activeRunEvents(id) {
  const headers = { 'X-Gregal-Session': id };
  const activeResp = await G().api('/api/active', { headers });
  if (!activeResp.ok) return null;
  const active = (await activeResp.json()).active;
  if (!active || active.task == null) return null;
  const events = [];
  let after = 0;
  // Cada petició té un màxim de 500 events; el cursor permet recuperar una
  // execució llarga sense truncar el seu inici.
  for (let page = 0; page < 20; page++) {
    const resp = await G().api('/api/events?session=' + encodeURIComponent(id) + '&after=' + after + '&limit=500', { headers });
    if (!resp.ok) return { active, events: [] };
    const data = await resp.json();
    const batch = Array.isArray(data.events) ? data.events : [];
    events.push(...batch.filter(e => Number(e.run_id) === Number(active.id)));
    const next = Number(data.next) || after;
    if (!batch.length || next <= after) break;
    after = next;
  }
  return { active, events };
}

// ---------------------------------------------------------------- sessions

export const sessions = {
  current: 'default',
  _error: '',
  _lastList: [],
  _lastOkAt: 0,
  _drafts: null,
  _draftKey: 'gregal_draft_sessions',
  drafts() {
    if (this._drafts) return this._drafts;
    try {
      const saved = JSON.parse(localStorage.getItem(this._draftKey) || '{}');
      this._drafts = saved && typeof saved === 'object' && !Array.isArray(saved) ? saved : {};
    } catch (e) { this._drafts = {}; }
    return this._drafts;
  },
  saveDraft() {
    const input = $('in');
    if (!input) return;
    const drafts = this.drafts();
    if (input.value) drafts[this.current] = input.value;
    else delete drafts[this.current];
    try { localStorage.setItem(this._draftKey, JSON.stringify(drafts)); } catch (e) {}
  },
  restoreDraft(id) {
    const input = $('in');
    if (!input) return;
    input.value = this.drafts()[id] || '';
    input.dispatchEvent(new Event('input'));
  },
  setError(message) {
    this._error = message || '';
    this._firma = null;
    this.render(this._lastList);
  },
  async init() {
    try { this.current = localStorage.getItem('gregal_session') || 'default'; } catch (e) {}
    window.gregalSession = this.current;
    const input = $('in');
    if (input) input.addEventListener('input', () => this.saveDraft());
    this.restoreDraft(this.current);
    const save = () => this.saveDraft();
    window.addEventListener('pagehide', save);
    window.addEventListener('beforeunload', save);
    // El primer refresh() de la pàgina surt abans que això i demana la
    // sessió 'default'; si la desada és una altra, la resposta es descarta
    // i la capçalera es quedava sense projecte ni carpeta. La demanem ara.
    if (this.current !== 'default') { try { G()?.refresh?.()?.catch?.(() => {}); } catch (e) {} }
    await this.render();
  },
  async list() {
    try {
      const list = (await j('/api/sessions/live')).sessions || [];
      this._lastList = list;
      this._lastOkAt = Date.now();
      this._error = '';
      return list;
    } catch (e) {
      this._error = T('sessions.loadFailed') + String(e.message || e);
      return this._lastList;
    }
  },
  // Accepta la llista ja demanada: el sondeig de cada 5s la demanava, i
  // després render() la tornava a demanar. Dues peticions cada cinc segons
  // per pintar la mateixa fila.
  async render(llista) {
    const bar = $('sessionTabs');
    if (!bar) return;
    const list = llista || await this.list();
    // Si res no ha canviat, no toquem el DOM: repintar la barra mentre
    // arrossegues o cliques és part del que es notava.
    const pending = window.gregalPendingInteractions;
    // El projecte hi ha de ser: la pestanya principal en duu el nom, i sense
    // ell canviar de carpeta no la repintava (deia l'antic).
    const firma = list.map(s => s.id + ':' + (s.busy ? 1 : 0) + ':' + (pending?.has(s.id) ? 1 : 0) + ':' + (s.title || '') + ':' + (s.project || '') + ':' + (s.worktree || '')).join('|') + '#' + this.current + '#' + this._error;
    if (firma === this._firma) return;
    this._firma = firma;
    bar.innerHTML = '';
    list.forEach(s => {
      const tab = document.createElement('button');
      tab.className = 'session-tab' + (s.id === this.current ? ' active' : '') + (s.busy ? ' busy' : '') + (pending?.has(s.id) ? ' needs-input' : '');
      const name = s.title || (s.id === 'default' ? s.project || T('sessions.main') : s.id);
      // El tooltip diu on treballa: nom, ruta sencera i, si n'és, el worktree.
      tab.title = name + '\n' + (s.cwd || '') + (s.worktree ? '\n' + T('sessions.worktreeBadge') + s.worktree : '') +
        (pending?.has(s.id) ? T('sessions.needsInput') : s.busy ? T('sessions.working') : '');
      // Sessió dins d'un git worktree aïllat: la pestanya ho diu.
      const wt = s.worktree ? '<span class="wt-badge" title="' + esc(T('sessions.worktreeBadge') + s.worktree) + '">⎇ ' + esc(s.worktree) + '</span>' : '';
      const status = pending?.has(s.id)
        ? '<span class="session-state needs-input">' + esc(T('sessions.needsInputLabel')) + '</span>'
        : s.busy ? '<span class="session-state working">' + esc(T('sessions.workingLabel')) + '</span>' : '';
      tab.innerHTML = '<span class="dot"></span><span class="nm">' + esc(name) + '</span>' + wt +
        status + (s.id === 'default' ? '' : '<span class="x" title="' + esc(s.busy ? T('sessions.closeBusy') : T('btn.close')) + '" aria-label="' + esc(s.busy ? T('sessions.closeBusy') : T('btn.close')) + '">×</span>');
      tab.onclick = e => {
        if (e.target.classList.contains('x')) { this.close(s.id); return; }
        this.switchTo(s.id);
      };
      bar.appendChild(tab);
    });
    if (this._error) {
      const error = document.createElement('span');
      error.className = 'session-error';
      error.textContent = this._error;
      error.title = this._error;
      error.style.cssText = 'align-self:center;color:var(--red);font-size:11px;max-width:260px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap';
      bar.appendChild(error);
    }
    const plus = document.createElement('button');
    plus.className = 'session-tab new';
    plus.textContent = '＋';
    plus.title = T('sessions.newTitle');
    plus.onclick = () => this.open();
    bar.appendChild(plus);
    const iso = document.createElement('button');
    iso.className = 'session-tab new worktree';
    iso.textContent = '⎇＋';
    iso.title = T('sessions.newWorktree');
    iso.setAttribute('aria-label', T('sessions.newWorktree'));
    iso.onclick = () => this.openWorktree();
    bar.appendChild(iso);
  },
  // Sessió aïllada: worktree nou (branca gregal/<nom>) i una pestanya que hi
  // treballa. El repo principal no es toca fins a «Porta al repo principal».
  async openWorktree() {
    const suggested = 's-' + new Date().toISOString().slice(5, 16).replace(/[-T:]/g, '');
    const name = prompt(T('sessions.worktreeName'), suggested);
    if (name == null) return;
    try {
      const r = await j('/api/worktrees', { method: 'POST', body: JSON.stringify({ name: name.trim() }) });
      if (r.session && r.session.id) await this.switchTo(r.session.id);
    } catch (e) {
      this.setError(T('sessions.worktreeFailed') + String(e.message || e));
    }
  },
  async open(cwd) {
    const r = await j('/api/sessions/open', { method: 'POST', body: JSON.stringify({ cwd: cwd || '' }) });
    await this.switchTo(r.id);
  },
  // Canviar de pestanya feia CINC peticions en sèrie: sessions vives,
  // /api/state, /api/state UN ALTRE COP (per al transcript, que ja venia a
  // la primera), el git diff i l'arbre de fitxers. Amb dues sessions
  // treballant, el diff i l'arbre triguen —el disc està ocupat— i el canvi
  // es quedava penjat segons. Ara són dues, i els panells cars només es
  // carreguen si els estàs mirant.
  async switchTo(id) {
    this.saveDraft();
    this.current = id;
    window.gregalSession = id;
    this.restoreDraft(id);
    G().onSessionSwitch(id);
    try { localStorage.setItem('gregal_session', id); } catch (e) {}
    const main = $('main');
    if (main) main.innerHTML = '';
    const st = await G().refresh();
    if (this.current !== id) return st;
    G().onSessionSwitch(id, st);
    const trans = (st && st.transcript) || [];
    let restored = renderSessionTranscript(trans, !!(st && st.agent_busy));
    if (st && st.agent_busy) {
      try {
        const replay = await activeRunEvents(id);
        if (this.current !== id) return st;
        if (replay && replay.active) {
          const runState = renderActiveRunEvents(replay.events);
          restored = Object.assign(restored, {
            pendingCards: runState.pendingCards,
            lastAssistantText: runState.lastAssistantText || restored.lastAssistantText,
            steps: restored.steps + runState.steps,
            tools: restored.tools + runState.tools,
            workGroup: runState.workGroup || restored.workGroup,
          });
        }
      } catch (e) { /* el transcript desat i l'estat ocupat continuen visibles */ }
    }
    const runHandler = window.gregalRunEventHandlers?.get(id);
    if (runHandler && typeof runHandler.restoreTranscript === 'function') runHandler.restoreTranscript(restored);
    G().sys(T('sessions.opened') + id + (trans.length ? ' · ' + trans.length + ' ' + T('sessions.messages') : ''));
    // En tornar d'una altra vista, els esdeveniments en directe no es
    // reprodueixen al fil. Fer explícit que el servidor encara treballa
    // evita que una sessió ocupada sembli aturada mentre arriba la resposta.
    if (st && st.agent_busy) G().sys(T('sessions.stillWorking'));
    // La barra de pestanyes i els panells, després i sense esperar-los: el
    // que volies era canviar de conversa, no mirar el diff.
    this.render();
    const vista = G().state && G().viewActual ? G().viewActual() : '';
    if (vista === 'canvis') changes.load();
    if (vista === 'fitxers') files.load();
    return st;
  },
  async close(id) {
    const session = this._lastList.find(s => s.id === id);
    if (session && session.busy && !confirm(T('sessions.closeBusyConfirm'))) return;
    try {
      await j('/api/sessions/close', { method: 'POST', body: JSON.stringify({ id }) });
    } catch (e) {
      this.setError(T('sessions.closeFailed') + String(e.message || e));
      G().sys(T('sessions.closeFailedWithID') + id + ': ' + String(e.message || e));
      return;
    }
    convs.forgetSession(id);
    if (this.current === id) {
      await this.switchTo('default');
      delete this.drafts()[id];
      try { localStorage.setItem(this._draftKey, JSON.stringify(this.drafts())); } catch (e) {}
      return;
    }
    delete this.drafts()[id];
    try { localStorage.setItem(this._draftKey, JSON.stringify(this.drafts())); } catch (e) {}
    await this.render();
  },
};

// Projecte = workspace persistent de la sessió nova. A l'escriptori usa el
// selector natiu; al navegador recau en el selector de workspaces existent.
async function nouProjecte() {
  if (window.gregalDesktop && window.gregalDesktop.chooseFolder) {
    const dir = await window.gregalDesktop.chooseFolder();
    if (!dir) return;
    try {
      // Desa el projecte al registre de workspaces i obre una sessió lligada
      // a la mateixa carpeta. Així el projecte reapareix a la sidebar després
      // de reiniciar el desktop, i la conversa conserva el seu context.
      await sessions.open(dir);
      G().setView('agent');
      G().sys(T('sessions.projectCreated') + dir);
    }
    catch (e) { G().sys(T('sessions.projectCreateFailed') + e.message); }
    return;
  }
  await workspaces.pick({ create: true });
}

// ------------------------------------------------------------------ canvis

export const changes = {
  data: null,
  async load() {
    const box = $('changesBody');
    if (!box) return;
    try {
      this.data = await j('/api/diff');
    } catch (e) {
      box.innerHTML = '<div class="activity-empty">' + esc(T('changes.loadFailed')) + esc(e.message) + '</div>';
      return;
    }
    this.render();
  },
  render() {
    const box = $('changesBody');
    const sum = $('changesSummary');
    const d = this.data || { files: [] };
    // Sense canvis, el servidor pot enviar files: null (una llista buida de
    // Go surt així en JSON) i aquí petava en llegir-ne la mida.
    if (!Array.isArray(d.files)) d.files = [];
    if (sum) {
      sum.textContent = d.files.length
        ? d.files.length + ' ' + T('changes.fileCount') + ' · +' + d.added + ' −' + d.removed + (d.branch ? ' · ' + d.branch : '')
        : T('changes.noChanges');
    }
    if (!d.files.length) {
      box.innerHTML = '<div class="activity-empty">' + esc(T('changes.empty')) + '</div>';
      return;
    }
    box.innerHTML = '';
    d.files.forEach(f => box.appendChild(this.fileCard(f)));
  },
  fileCard(f) {
    const wrap = document.createElement('details');
    wrap.className = 'diff-file';
    wrap.open = f.hunks && f.hunks.length <= 3;
    const badge = esc(T({ added: 'changes.added', deleted: 'changes.deleted', renamed: 'changes.renamed' }[f.status] || 'changes.modified'));
    const head = document.createElement('summary');
    head.innerHTML = '<span class="dfname">' + esc(f.path) + '</span>' +
      '<span class="dfbadge ' + esc(f.status) + '">' + badge + '</span>' +
      '<span class="dfstat"><b class="add">+' + f.added + '</b> <b class="del">−' + f.removed + '</b></span>';
    const kill = document.createElement('button');
    kill.className = 'ghost tiny';
    kill.textContent = T('changes.discardFile');
    kill.onclick = async e => {
      e.preventDefault(); e.stopPropagation();
      if (!confirm(T('changes.confirmDiscard') + f.path + '?')) return;
      await this.discard({ path: f.path });
    };
    head.appendChild(kill);
    const open = document.createElement('button');
    open.className = 'ghost tiny diff-open';
    open.textContent = T('changes.openFile');
    open.title = T('changes.openFileTitle');
    open.onclick = e => { e.preventDefault(); e.stopPropagation(); files.view(f.path); };
    head.appendChild(open);
    wrap.appendChild(head);
    if (f.binary) {
      const b = document.createElement('div');
      b.className = 'activity-empty';
      b.textContent = T('changes.binary');
      wrap.appendChild(b);
      return wrap;
    }
    (f.hunks || []).forEach((h, i) => wrap.appendChild(this.hunkBlock(f, h, i)));
    return wrap;
  },
  hunkBlock(f, h, i) {
    const box = document.createElement('div');
    box.className = 'diff-hunk';
    const bar = document.createElement('div');
    bar.className = 'hunk-bar';
    bar.innerHTML = '<code>' + esc(h.header) + '</code>';
    const drop = document.createElement('button');
    drop.className = 'ghost tiny';
    drop.textContent = T('changes.discardHunk');
    drop.title = T('changes.restoreHunkTitle');
    drop.onclick = () => this.discard({ path: f.path, hunk: i });
    const keep = document.createElement('button');
    keep.className = 'ghost tiny ok';
    keep.textContent = T('changes.acceptHunk');
    keep.title = T('changes.acceptHunkTitle');
    keep.onclick = () => { box.classList.add('accepted'); keep.disabled = true; };
    bar.append(keep, drop);
    box.appendChild(bar);
    const pre = document.createElement('div');
    pre.className = 'diff-lines';
    pre.innerHTML = (h.lines || []).map(l => {
      const sign = l.kind === 'add' ? '+' : l.kind === 'del' ? '−' : ' ';
      const num = l.kind === 'add' ? (l.new || '') : (l.old || '');
      return '<div class="dl ' + l.kind + '"><span class="ln">' + num + '</span>' +
        '<span class="sg">' + sign + '</span><span class="tx">' + esc(l.text) + '</span></div>';
    }).join('');
    box.appendChild(pre);
    return box;
  },
  async discard(payload) {
    try {
      await j('/api/diff/discard', { method: 'POST', body: JSON.stringify(payload) });
      // El resum del servidor és en català; el detall es compon aquí.
      G().activityItem('canvis', T('changes.discarded'), payload.path + (payload.hunk != null ? ' (hunk ' + (payload.hunk + 1) + ')' : ''), 'ok');
    } catch (e) {
      G().activityItem('canvis', T('changes.discardFailed').replace(/:\s*$/, ''), String(e.message || e), 'bad');
      alert(T('changes.discardFailed') + e.message);
    }
    await this.load();
    files.load();
  },
};

// ----------------------------------------------------------------- fitxers

export const files = {
  open: new Set(),
  async load(path) {
    const box = $('treeBody');
    if (!box) return;
    try {
      const r = await j('/api/tree?depth=2&path=' + encodeURIComponent(path || ''));
      box.innerHTML = '';
      box.appendChild(this.nodeList(r.entries || []));
    } catch (e) {
      box.innerHTML = '<div class="activity-empty">' + esc(e.message) + '</div>';
    }
  },
  nodeList(entries) {
    const ul = document.createElement('div');
    ul.className = 'tree-list';
    entries.forEach(e => ul.appendChild(this.node(e)));
    return ul;
  },
  node(e) {
    const row = document.createElement('div');
    row.className = 'tree-node' + (e.dir ? ' dir' : '');
    const btn = document.createElement('button');
    btn.className = 'tree-row';
    btn.innerHTML = '<span class="ti">' + (e.dir ? '▸' : '·') + '</span><span class="tn">' + esc(e.name) + '</span>' +
      (e.status ? '<span class="tst ' + esc(e.status) + '">' + esc(e.status) + '</span>' : '');
    const kids = document.createElement('div');
    kids.className = 'tree-kids';
    kids.hidden = true;
    if (e.dir) {
      if (e.children && e.children.length) kids.appendChild(this.nodeList(e.children));
      btn.onclick = async () => {
        kids.hidden = !kids.hidden;
        btn.querySelector('.ti').textContent = kids.hidden ? '▸' : '▾';
        if (!kids.hidden && !kids.childElementCount) {
          const r = await j('/api/tree?depth=2&path=' + encodeURIComponent(e.path));
          kids.appendChild(this.nodeList(r.entries || []));
        }
      };
    } else {
      btn.onclick = () => this.view(e.path);
      btn.ondblclick = () => G().mention(e.path);
    }
    row.append(btn, kids);
    return row;
  },
  async view(path) {
    // La previsualització viu a la subfinestra lateral perquè el xat i el
    // fitxer es puguin veure alhora. El panell Fitxers continua servint per
    // navegar l'arbre.
    if (office && office.openTextFile) { await office.openTextFile(path); return; }
    const pane = $('fileView');
    if (!pane) return;
    pane.innerHTML = '<div class="activity-empty">' + esc(T('files.loading')) + esc(path) + '…</div>';
    try {
      const r = await j('/api/file?path=' + encodeURIComponent(path));
      if (r.binary) { pane.innerHTML = '<div class="activity-empty">' + esc(T('files.binary')) + r.size + esc(T('files.bytes')) + '</div>'; return; }
      if (r.too_big) { pane.innerHTML = '<div class="activity-empty">' + esc(T('files.tooLarge')) + r.size + esc(T('files.bytes')) + '</div>'; return; }
      const lines = String(r.content).split('\n');
      pane.innerHTML = '<div class="fv-head"><b>' + esc(path) + '</b>' +
        '<button class="ghost tiny" id="fvMention">' + esc(T('files.addToMessage')) + '</button></div>' +
        '<div class="fv-body">' + lines.map((l, i) =>
          '<div class="fl"><span class="ln">' + (i + 1) + '</span><span class="tx">' + esc(l) + '</span></div>').join('') + '</div>';
      $('fvMention').onclick = () => G().mention(path);
    } catch (e) {
      pane.innerHTML = '<div class="activity-empty">' + esc(e.message) + '</div>';
    }
  },
  // Llista plana per a l'autocompleció de @.
  cache: null,
  async paths() {
    if (this.cache) return this.cache;
    try {
      const r = await j('/api/tree?depth=5');
      const out = [];
      const walk = ns => ns.forEach(n => { if (n.dir) walk(n.children || []); else out.push(n.path); });
      walk(r.entries || []);
      this.cache = out;
      setTimeout(() => { this.cache = null; }, 30000);
      return out;
    } catch (e) { return []; }
  },
};

// ---------------------------------------------------------------- navegador

export const browser = {
  ready: false,
  init() {
    const url = $('browserUrl'), go = $('browserGo');
    if (!url || !go) return;
    const view = $('browserView'), frame = $('browserFrame');
    this.ready = true;
    const desktop = !!window.gregalDesktop && view && typeof view.loadURL === 'function';
    if (!desktop && view) view.style.display = 'none';
    if (!desktop && frame) frame.style.display = 'block';
    const normalitza = raw => {
      let v = String(raw || '').trim();
      if (!v) return '';
      if (!/^https?:\/\//i.test(v)) v = 'https://' + v;
      try { const u = new URL(v); return /^https?:$/.test(u.protocol) ? u.href : ''; } catch (e) { return ''; }
    };
    const navigate = () => {
      const v = normalitza(url.value);
      if (!v) { G().sys(T('browser.invalidAddress')); return; }
      url.value = v;
      $('browserEmpty').hidden = true;
      if (desktop) view.loadURL(v); else frame.src = v;
    };
    go.onclick = navigate;
    url.onkeydown = e => { if (e.key === 'Enter') { e.preventDefault(); navigate(); } };
    $('browserBack').onclick = () => desktop ? view.goBack() : history.back();
    $('browserForward').onclick = () => desktop ? view.goForward() : history.forward();
    $('browserReload').onclick = () => desktop ? view.reload() : (frame.src = frame.src);
    $('browserAttach').onclick = () => {
      const v = normalitza(url.value); if (!v) return;
      G().setComposer(T('browser.reviewPrompt') + v);
    };
    if (desktop) {
      view.addEventListener('did-navigate', e => { url.value = e.url; });
      view.addEventListener('did-navigate-in-page', e => { url.value = e.url; });
      view.addEventListener('did-fail-load', e => { if (e.errorCode !== -3) G().sys(T('nav.browser') + ': ' + e.errorDescription); });
    }
  },
};

// ---------------------------------------------------------------- terminal

export const term = {
  procs: [],
  async load() {
    const box = $('procList');
    if (!box) return;
    try {
      const r = await j('/api/exec');
      this.procs = r.procs || [];
    } catch (e) { this.procs = []; }
    box.innerHTML = '';
    if (!this.procs.length) {
      box.innerHTML = '<div class="activity-empty">' + esc(T('terminal.empty')) + '</div>';
      return;
    }
    this.procs.forEach(p => {
      const row = document.createElement('div');
      row.className = 'proc-row' + (p.running ? ' running' : '');
      row.innerHTML = '<span class="dot"></span><code>' + esc(p.cmd) + '</code>' +
        '<span class="pstate">' + esc(p.running ? T('terminal.running') : T('terminal.exitCode') + (p.exit == null ? '?' : p.exit)) + '</span>';
      const open = document.createElement('button');
      open.className = 'ghost tiny';
      open.textContent = T('terminal.output');
      open.onclick = () => this.watch(p.id);
      row.appendChild(open);
      if (p.running) {
        const kill = document.createElement('button');
        kill.className = 'ghost tiny';
        kill.textContent = T('terminal.stop');
        kill.onclick = async () => { await j('/api/exec/kill', { method: 'POST', body: JSON.stringify({ id: p.id }) }); this.load(); };
        row.appendChild(kill);
      }
      box.appendChild(row);
    });
  },
  async run(cmd) {
    if (!cmd.trim()) return;
    try {
      const r = await j('/api/exec', { method: 'POST', body: JSON.stringify({ cmd }) });
      G().activityItem('procés', cmd.slice(0, 60), T('terminal.started'), 'warn');
      await this.load();
      this.watch(r.id);
    } catch (e) {
      $('procOut').textContent = T('terminal.startFailed') + e.message;
    }
  },
  timer: null,
  async watch(id) {
    clearInterval(this.timer);
    const out = $('procOut');
    out.textContent = '';
    let from = 0;
    const tick = async () => {
      try {
        const r = await j('/api/exec/output?id=' + encodeURIComponent(id) + '&from=' + from);
        (r.lines || []).forEach(l => {
          from = l.n;
          const d = document.createElement('div');
          d.className = 'pl ' + l.stream;
          d.textContent = l.text;
          out.appendChild(d);
        });
        out.scrollTop = out.scrollHeight;
        if (r.done) { clearInterval(this.timer); this.load(); }
      } catch (e) { clearInterval(this.timer); }
    };
    await tick();
    this.timer = setInterval(tick, 500);
  },
};

// ---------------------------------------------------------------- composer

// Cada ordre té el nom català de sempre i un d'anglès: a la llista surt el
// de l'idioma de la UI, però s'accepten tots dos. La descripció és una clau
// perquè la llista es construeix en carregar el mòdul, abans de l'idioma.
const COMMANDS = [
  { cmd: '/mode', en: '/mode', desc: 'keys.modeCycle', run: () => G().setMode(G().nextMode()) },
  { cmd: '/model', en: '/model', desc: 'cmd.model', run: () => models.open() },
  { cmd: '/canvis', en: '/changes', desc: 'cmd.changes', run: () => G().setView('canvis') },
  { cmd: '/fitxers', en: '/files', desc: 'cmd.files', run: () => G().setView('fitxers') },
  { cmd: '/coneixement', en: '/knowledge', desc: 'cmd.knowledge', run: () => G().setView('knowledge') },
  { cmd: '/terminal', en: '/terminal', desc: 'cmd.terminal', run: () => G().setView('terminal') },
  { cmd: '/nova', en: '/new', desc: 'cmd.newSession', run: () => sessions.open() },
  { cmd: '/atura', en: '/stop', desc: 'cmd.stop', run: () => j('/api/agent/cancel', { method: 'POST', body: '{}' }) },
  { cmd: '/desfes', en: '/undo', desc: 'cmd.undo', run: () => $('rewindBtn').click() },
  { cmd: '/compara', en: '/compare', desc: 'cmd.compare', run: () => $('parallelBtn').click() },
  { cmd: '/pla', en: '/plan', desc: 'cmd.plan', run: () => $('planBtn').click() },
  // /debug <tasca>: mode depuració per a aquest torn (app/caps.js l'intercepta en enviar).
  { cmd: '/debug', en: '/debug', desc: 'cmd.debug', run: () => { const i = $('in'); i.value = '/debug ' + i.value.trimStart(); } },
  { cmd: '/objectiu', en: '/goal', desc: 'cmd.goalMode', run: () => G().setMode('goal') },
  { cmd: '/autonom', en: '/auto', desc: 'cmd.autoMode', run: () => G().setMode('autonomous') },
  { cmd: '/objectius', en: '/goals', desc: 'cmd.goals', run: () => G().setView('objectius') },
  { cmd: '/activitat', en: '/activity', desc: 'cmd.activity', run: () => G().setView('activitat') },
  { cmd: '/permissiu', en: '/permissive', desc: 'cmd.permissive', run: () => $('permBtn').click() },
];

// mcpPrompts: els prompts MCP per a la paleta (memòria de 30 s).
let mcpPromptCache = { at: 0, list: [] };
async function mcpPrompts() {
  if (Date.now() - mcpPromptCache.at < 30000) return mcpPromptCache.list;
  try {
    const d = await j('/api/mcp');
    mcpPromptCache = { at: Date.now(), list: Array.isArray(d.prompts) ? d.prompts : [] };
  } catch (e) { mcpPromptCache = { at: Date.now(), list: [] }; }
  return mcpPromptCache.list;
}

// runMcpPrompt demana els arguments, expandeix el prompt al servidor i el
// deixa al compositor.
async function runMcpPrompt(p) {
  const args = {};
  for (const a of (p.arguments || [])) {
    const name = String(a).replace(/\*$/, '');
    const v = window.prompt(p.command + ' · ' + name + (String(a).endsWith('*') ? ' *' : ''), '');
    if (v === null) return;
    if (v.trim()) args[name] = v.trim();
  }
  try {
    const d = await j('/api/mcp/prompt', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ text: p.command, args }) });

    const input = $('in');
    input.value = d.text || '';
    input.dispatchEvent(new Event('input'));
    input.focus();
  } catch (e) { window.alert(e.message || String(e)); }
}

// models: selector de proveïdor i model per al rol actiu, com el /model del

// TUI. /api/models llista el que cada proveïdor anuncia ara mateix i
// /api/model valida abans de fixar l'override. Abans a la web
// només es podia escriure el nom a mà als proveïdors.
export const models = {
  box: null, data: null, all: [], filtered: [], idx: 0, busy: false,
  recentKey: 'gregal_recent_models',
  recents() {
    try {
      const value = JSON.parse(localStorage.getItem(this.recentKey) || '[]');
      return Array.isArray(value) ? [...new Set(value.filter(x => typeof x === 'string'))].slice(0, 8) : [];
    } catch (e) { return []; }
  },
  remember(model) {
    const list = [model, ...this.recents().filter(x => x !== model)].slice(0, 8);
    try { localStorage.setItem(this.recentKey, JSON.stringify(list)); } catch (e) {}
    return list;
  },
  init() {
    const pill = $('modelPill');
    if (!pill) return;
    pill.addEventListener('click', e => { e.stopPropagation(); this.box && !this.box.hidden ? this.close() : this.open(); });
    document.addEventListener('click', e => { if (this.box && !this.box.hidden && !this.box.contains(e.target)) this.close(); });
    document.addEventListener('keydown', e => { if (e.key === 'Escape' && this.box && !this.box.hidden) this.close(); });
  },
  ensure() {
    if (this.box) return this.box;
    this.box = document.createElement('div');
    this.box.id = 'mpick';
    this.box.hidden = true;
    this.box.innerHTML =
      '<div class="mp-tab-page" data-page="models">' +
        '<div class="mp-subhead">' +
          '<span>' + T('models.taskModel') + '</span>' +
        '</div>' +
        '<div class="mp-search-wrap">' +
          '<span class="mp-search-icon">🔍</span>' +
          '<input class="mp-search" placeholder="' + T('models.filterPlaceholder') + '" aria-label="' + T('models.filterLabel') + '" autocomplete="off">' +
        '</div>' +
        '<div class="mp-list" role="listbox"></div>' +
      '</div>' +
      '<details class="mp-advanced-roles">' +
        '<summary>' + T('models.advancedRoles') + '</summary>' +
        '<div class="mp-advanced-body">' +
          '<div class="mp-subhead"><span>' + T('models.rolesHelp') + '</span></div>' +
          '<div class="mp-rols"></div>' +
        '</div>' +
      '</details>' +
      '<div class="mp-foot"></div>';
    document.body.appendChild(this.box);

    const s = this.box.querySelector('.mp-search');
    s.addEventListener('input', () => { this.idx = 0; this.draw(); });
    s.addEventListener('keydown', e => this.key(e));
    return this.box;
  },
  place() {
    const pill = $('modelPill');
    const r = pill.getBoundingClientRect();
    const w = Math.min(440, window.innerWidth - 16);
    this.box.style.top = (r.bottom + 6) + 'px';
    this.box.style.left = Math.max(8, Math.min(r.left, window.innerWidth - w - 8)) + 'px';
    this.box.style.width = w + 'px';
  },
  async open() {
    this.ensure();
    this.place();
    this.box.hidden = false;
    this.box.querySelector('.mp-list').innerHTML = '<div class="mp-empty">' + T('models.fetching') + '</div>';
    this.box.querySelector('.mp-foot').textContent = '';
    const s = this.box.querySelector('.mp-search'); s.value = ''; s.focus();
    try {
      this.data = await j('/api/models');
    } catch (e) {
      this.box.querySelector('.mp-list').innerHTML = '<div class="mp-empty mp-err">' + esc(e.message || e) + '</div>';
      return;
    }
    const cur = String(this.data.current || '').replace(/ \(override\)$/, '');
    this.all = [];
    Object.keys(this.data.models || {}).sort().forEach(p => {
      (this.data.models[p] || []).forEach(m => this.all.push({ prov: p, id: m, sel: p + '/' + m, cur: p + '/' + m === cur }));
    });
    const recents = this.recents();
    this.all.forEach(m => { m.recent = recents.indexOf(m.sel); });
    this.all.sort((a, b) => {
      const ar = a.recent < 0 ? Infinity : a.recent, br = b.recent < 0 ? Infinity : b.recent;
      return ar - br || a.prov.localeCompare(b.prov) || a.id.localeCompare(b.id);
    });
    // Proveïdors desconnectats en un acordió plegat discret per no carregar el menú:
    const errs = Object.entries(this.data.errors || {});
    let footHTML = '';
    if (this.data.selected_available === false) {
      const selected = this.data.selected_override || T('models.selectedFallback');
      const fallback = this.data.fallback;
      footHTML += '<div class="mp-err">' + esc(T('models.selectedUnavailable')) + esc(selected) + esc(T('models.selectedUnavailableSuffix')) +
        (fallback ? esc(T('models.usingTemporarily')) + esc(fallback) + esc(T('models.usingTemporarilySuffix')) : esc(T('models.noAlternative'))) +
        '</div>';
    }
    if (errs.length) {
      const summaryText = errs.length === 1
        ? esc(T('models.providerOfflineOne')) + esc(errs[0][0]) + ')'
        : errs.length + esc(T('models.providerOfflineMany')) + errs.map(e => esc(e[0])).join(', ') + ')';
      footHTML +=
        '<details class="mp-offline">' +
          '<summary class="mp-offline-sum">' +
            '<span class="mp-offline-dot"></span>' +
            '<span>' + summaryText + '</span>' +
            '<span class="mp-offline-caret">▾</span>' +
          '</summary>' +
          '<div class="mp-offline-body">' +
            errs.map(([p, m]) => '<div class="mp-offline-row"><b>' + esc(p) + ':</b> ' + esc(m) + '</div>').join('') +
          '</div>' +
        '</details>';
    }
    this.box.querySelector('.mp-foot').innerHTML = footHTML;

    this.idx = Math.max(0, this.all.findIndex(m => m.cur));
    await this.dibuixaRols();
    this.draw();
  },

  // NOMS tradueix el rol del config a què vol dir per a qui el fa servir.
  // «think» o «reviewer» són noms nostres, no del qui mira la pantalla.
  nomRole(role) {
    const claus = { chat: 'models.role.chat', think: 'models.role.think', code: 'models.role.code', reviewer: 'models.role.reviewer' };
    return T(claus[role] || role || 'models.automatic');
  },

  async dibuixaRols() {
    const caixa = this.box.querySelector('.mp-rols');
    if (!caixa) return;
    const st = G().state || {};
    let rols = {};
    try { rols = (await j('/api/providers')).roles || {}; } catch (e) {}
    const fixat = !!st.role_pinned;
    const fila = (val, titol, sub, marcat) =>
      '<button class="mp-rol' + (marcat ? ' on' : '') + '" data-rol="' + esc(val) + '">' +
      '<b>' + esc(titol) + '</b><span>' + esc(sub) + '</span></button>';
    let html = fila('auto', T('models.automatic'),
      T('models.roleAutomaticHelp') + (!fixat && st.current_role ? ' · ' + T('models.currentRole') + ': ' + this.nomRole(st.current_role) : ''),
      !fixat);
    (st.role || []).forEach(n => {
      const r = rols[n] || {};
      const model = r.model ? r.provider + '/' + r.model : T('models.noModelConfigured');
      html += fila(n, this.nomRole(n), model, fixat && n === st.current_role);
    });
    caixa.innerHTML = html;
    caixa.querySelectorAll('[data-rol]').forEach(b => b.onclick = async () => {
      try {
        await G().api('/api/role', { method: 'POST', body: JSON.stringify({ role: b.dataset.rol }) });
        await G().refresh();
        this.close();
      } catch (e) {}
    });
  },
  close() {
    if (!this.box) return;
    const dins = this.box.contains(document.activeElement);
    this.box.hidden = true;
    // En seleccionar un model el focus quedava al cercador ocult del picker;
    // l'usuari escrivia però el missatge anava a un input invisible.
    if (dins) $('in')?.focus();
  },
  draw() {
    const q = this.box.querySelector('.mp-search').value.trim().toLowerCase();
    this.filtered = q ? this.all.filter(m => m.sel.toLowerCase().includes(q)) : this.all.slice();
    const list = this.box.querySelector('.mp-list');
    if (!this.filtered.length) {
      const errs = Object.keys(this.data?.errors || {});
      const empty = this.all.length
        ? T('models.noMatch')
        : errs.length
          ? T('models.noneAvailable')
          : T('models.noneListed');
      list.innerHTML = '<div class="mp-empty">' + esc(empty) + '</div>';
      return;
    }
    if (this.idx >= this.filtered.length) this.idx = 0;
    let html = '';
    const itemHTML = (m, i) =>
      '<div class="mp-item' + (i === this.idx ? ' on' : '') + (m.cur ? ' cur' : '') + '" data-i="' + i + '" role="option" aria-selected="' + (m.cur) + '">' +
        '<span class="mp-mark">' + (m.cur ? '✓' : '') + '</span>' +
        '<span class="mp-id">' + esc(m.id) + '</span>' +
      '</div>';
    const recent = this.filtered.filter(m => m.recent >= 0);
    if (recent.length) {
      html += '<div class="mp-prov">' + T('models.recent') + '</div>';
      recent.forEach(m => html += itemHTML(m, this.filtered.indexOf(m)));
    }
    const rest = this.filtered.filter(m => m.recent < 0);
    let lastProv = '';
    rest.forEach(m => {
      const i = this.filtered.indexOf(m);
      if (m.prov !== lastProv) { html += '<div class="mp-prov">' + esc(m.prov) + '</div>'; lastProv = m.prov; }
      html += itemHTML(m, i);
    });
    list.innerHTML = html;
    list.querySelectorAll('.mp-item').forEach(el => {
      el.onclick = () => { this.idx = +el.dataset.i; this.pick(); };
    });
    const on = list.querySelector('.mp-item.on'); if (on) on.scrollIntoView({ block: 'nearest' });
  },
  key(e) {
    if (!this.filtered.length) return;
    if (e.key === 'ArrowDown') { e.preventDefault(); this.idx = (this.idx + 1) % this.filtered.length; this.draw(); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); this.idx = (this.idx - 1 + this.filtered.length) % this.filtered.length; this.draw(); }
    else if (e.key === 'Enter') { e.preventDefault(); this.pick(); }
  },
  async pick() {
    const m = this.filtered[this.idx];
    if (!m || this.busy) return;
    this.busy = true;
    const foot = this.box.querySelector('.mp-foot');
    foot.innerHTML = '<div class="mp-subhead">' + T('models.validating') + esc(m.sel) + '…</div>';
    try {
      // G().api posa el token Bearer si n'hi ha; el servidor valida que el
      // proveïdor anunciï el model abans de fixar res.
      const res = await G().api('/api/model', { method: 'POST', body: JSON.stringify({ model: m.sel }) });
      if (!res.ok) { foot.innerHTML = '<div class="mp-err">' + esc(await res.text()) + '</div>'; return; }
      this.remember(m.sel);
      G().sys(T('models.changed') + m.sel);
      G().activityItem('model', m.sel, T('models.roleActivity') + (this.data.role || ''), 'ok');
      this.close();
      G().refresh();
    } catch (e) {
      foot.innerHTML = '<div class="mp-err">' + esc(e.message || e) + '</div>';
    } finally { this.busy = false; }
  },
};

export const composer = {
  box: null, items: [], idx: 0, kind: null,
  init() {
    const input = $('in');
    if (!input) return;
    this.box = document.createElement('div');
    this.box.id = 'suggest';
    this.box.hidden = true;
    $('box').appendChild(this.box);
    input.addEventListener('input', () => this.update());
    input.addEventListener('keydown', e => this.key(e), true);
    document.addEventListener('click', e => { if (!this.box.contains(e.target)) this.hide(); });
  },
  hide() { this.box.hidden = true; this.items = []; this.kind = null; },
  token() {
    const input = $('in');
    const upto = input.value.slice(0, input.selectionStart);
    const m = upto.match(/(^|\s)([/@])([^\s]*)$/);
    return m ? { sign: m[2], word: m[3], start: upto.length - m[3].length - 1 } : null;
  },
  async update() {
    const t = this.token();
    if (!t) { this.hide(); return; }
    if (t.sign === '/') {
      this.kind = '/';
      const ca = idioma() === 'ca';
      this.items = COMMANDS.filter(c => c.cmd.startsWith('/' + t.word) || c.en.startsWith('/' + t.word)).slice(0, 8)
        .map(c => ({ cmd: ca ? c.cmd : c.en, desc: T(c.desc), run: c.run }));
      // Prompts dels servidors MCP (/mcp__servidor__prompt): s'expandeixen
      // al compositor perquè la persona els revisi abans d'enviar.
      const prompts = (await mcpPrompts()).filter(p => p.command.startsWith('/' + t.word));
      this.items = this.items.concat(prompts.slice(0, Math.max(0, 8 - this.items.length))
        .map(p => ({ cmd: p.command, desc: p.description || T('cmd.mcpPrompt'), run: () => runMcpPrompt(p) })));
    } else {
      this.kind = '@';
      const all = await files.paths();
      const q = t.word.toLowerCase();
      this.items = all.filter(p => p.toLowerCase().includes(q)).slice(0, 8).map(p => ({ cmd: p, desc: '' }));
    }
    this.idx = 0;
    this.draw();
  },
  draw() {
    if (!this.items.length) { this.hide(); return; }
    this.box.innerHTML = this.items.map((it, i) =>
      '<div class="sg-item' + (i === this.idx ? ' on' : '') + '" data-i="' + i + '"><b>' + esc(it.cmd) + '</b>' +
      (it.desc ? '<span>' + esc(it.desc) + '</span>' : '') + '</div>').join('');
    this.box.hidden = false;
    this.box.querySelectorAll('.sg-item').forEach(el =>
      el.onclick = () => { this.idx = +el.dataset.i; this.pick(); });
  },
  key(e) {
    if (this.box.hidden || !this.items.length) return;
    if (e.key === 'ArrowDown') { e.preventDefault(); this.idx = (this.idx + 1) % this.items.length; this.draw(); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); this.idx = (this.idx - 1 + this.items.length) % this.items.length; this.draw(); }
    else if (e.key === 'Enter' || e.key === 'Tab') { e.preventDefault(); e.stopPropagation(); this.pick(); }
    else if (e.key === 'Escape') { this.hide(); }
  },
  pick() {
    const it = this.items[this.idx];
    const t = this.token();
    if (!it || !t) return;
    const input = $('in');
    if (this.kind === '/') {
      input.value = input.value.slice(0, t.start) + input.value.slice(input.selectionStart);
      this.hide();
      if (it.run) it.run();
      input.focus();
      return;
    }
    input.value = input.value.slice(0, t.start) + '@' + it.cmd + ' ' + input.value.slice(input.selectionStart);
    this.hide();
    input.focus();
  },
};

// --------------------------------------------------------------------- HUD

export const hud = {
  cost: 0,
  set(text) { const el = $('hudCost'); if (el) el.textContent = text; },
  addCost(usd) {
    this.cost += usd || 0;
    if (this.cost > 0) this.set('$' + this.cost.toFixed(4));
  },
};

// ------------------------------------------------------------------ arrencada

export function boot() {
  // L'idioma, primer de tot: si no, la finestra surt en català i salta a
  // l'anglès un cop carregada, i es veu el salt.
  window.gregalT = T;
  // Si aquest navegador no ha triat idioma, mana el config del servidor:
  // refresh() ens porta el valor i crida aquesta funcio.
  window.gregalLang = defineixPerDefecte;
  aplicaIdioma();
  composer.init();
  sessions.init();
  workshop.init();
  const runBtn = $('procRun'), cmdIn = $('procCmd');
  if (runBtn) runBtn.onclick = () => { term.run(cmdIn.value); cmdIn.value = ''; };
  if (cmdIn) cmdIn.onkeydown = e => { if (e.key === 'Enter') { term.run(cmdIn.value); cmdIn.value = ''; } };
  const refreshBtn = $('changesRefresh');
  if (refreshBtn) refreshBtn.onclick = () => changes.load();
  const treeRefresh = $('treeRefresh');
  if (treeRefresh) treeRefresh.onclick = () => { files.cache = null; files.load(); };
  const wsBtn = $('wsBtn');
  if (wsBtn) wsBtn.onclick = () => workspaces.pick();
  const newProject = $('newproject');
  if (newProject) newProject.onclick = () => nouProjecte();
  const sideCompactBtn = $('sideCompact');
  if (sideCompactBtn) sideCompactBtn.onclick = () => {
    const side = $('side');
    if (!side || mobile.matches) return;
    const compact = !side.classList.contains('compact');
    side.classList.toggle('compact', compact);
    sideCompactBtn.textContent = compact ? '›' : '‹';
    sideCompactBtn.dataset.i18nTitle = compact ? 'ui.sidebarExpand' : 'ui.sidebarCompact';
    sideCompactBtn.title = T(sideCompactBtn.dataset.i18nTitle);
    sideCompactBtn.setAttribute('aria-pressed', compact ? 'true' : 'false');
    try { localStorage.setItem('gregal_side_compact', compact ? '1' : '0'); } catch (e) {}
  };
  document.querySelectorAll('[data-inspector]').forEach(b => b.onclick = () => {
    const view = b.dataset.inspector || 'agent';
    if (view === 'agent') { G().setView('agent'); $('in')?.focus(); return; }
    G().setView(view);
  });
  const convToggle = $('convToggle');
  if (convToggle) convToggle.onclick = () => {
    const search = $('convSearch'), list = $('convs');
    const hidden = list && !list.hidden;
    if (search) search.hidden = hidden;
    if (list) list.hidden = hidden;
    convToggle.setAttribute('aria-expanded', hidden ? 'false' : 'true');
    convToggle.textContent = hidden ? '⌄' : '⌃';
  };
  // El nom del projecte a la capçalera és dins del xip de la carpeta
  // (app/folder.js), que ja obre el selector.
  const workspaceButton = $('activeWorkspace');
  if (workspaceButton) workspaceButton.onclick = () => workspaces.pick();
  document.addEventListener('keydown', e => {
    if (!(e.ctrlKey || e.metaKey)) return;
    if (e.key === 't') { e.preventDefault(); sessions.open(); }
    else if (e.key === 'd') { e.preventDefault(); G().setView('canvis'); changes.load(); }
    else if (e.key === 'e') { e.preventDefault(); G().setView('fitxers'); files.load(); }
    else if (e.key === 'g') { e.preventDefault(); G().setView('grafs'); }
  });
  // Dreceres del menú natiu de l'escriptori.
  if (window.gregalDesktop && window.gregalDesktop.onCommand) {
    window.gregalDesktop.onCommand(name => {
      if (name === 'open-project') workspaces.pick();
      else if (name === 'new-session') sessions.open();
      else if (name === 'changes') { G().setView('canvis'); changes.load(); }
      else if (name === 'files') { G().setView('fitxers'); files.load(); }
    });
  }
  models.init();
  office.init();
  browser.init();
  convs.init();
  prefs.init();
  attach.init();
  window.gregalPanels = { sessions, changes, files, browser, term, composer, hud, workspaces, models, office, convs, prefs, attach, flows, workshop };
  // Les pestanyes alienes no s'actualitzaven mentre treballaven (render només
  // a boot i a done). Polling lleuger: si alguna sessió està busy, repinta.
  // Fora de línia el sondeig s'espaia (un cop cada 15 s) i, quan torna,
  // repinta tot: és api() qui marca l'estat, aquí només es respecta.
  let nextSessionPoll = 0, pollingSessions = false;
  setInterval(async () => {
    if (document.hidden || pollingSessions || Date.now() < nextSessionPoll) return;
    pollingSessions = true;
    const delay = window.gregalOffline ? 15000 : G()?.viewActual() === 'workshop' ? 1500 : 5000;
    nextSessionPoll = Date.now() + delay;
    try {
      const list = await sessions.list();
      convs.updateLiveSessions(list);
      if (G()?.viewActual() === 'workshop') await workshop.update(list);
      await sessions.render(list);
    } catch (e) {} finally { pollingSessions = false; }
  }, 1000);
  const retry = document.getElementById('offlineRetry');
  if (retry) retry.onclick = () => { G().refresh(); };
  // El que es pinta des d'aquí no porta data-i18n: en canviar d'idioma es
  // torna a pintar el que es veu, i el selector de models es refà en obrir-lo.
  document.addEventListener('gregal:idioma', () => {
    if (sessions._lastList.length) { sessions._firma = null; sessions.render(sessions._lastList); }
    if (changes.data) changes.render();
    if (G()?.viewActual?.() === 'terminal') term.load();
    if (models.box && models.box.hidden) { models.box.remove(); models.box = null; }
  });
}

// ------------------------------------------------------------- workspaces

export const workspaces = {
  // El selector és el mateix a l'escriptori i al navegador: recents,
  // navegador de carpetes del servidor i ruta a mà. A l'escriptori, a
  // més, el diàleg natiu del sistema. Abans al navegador era un prompt()
  // amb una llista numerada, i al Windows escriure una ruta sense
  // equivocar-se era la part més lenta d'obrir un projecte.
  async pick(options = {}) {
    if (document.querySelector('.ws-overlay')) return;
    const t = k => (typeof window.gregalT === 'function' && window.gregalT(k)) || '';
    const esc = x => String(x == null ? '' : x).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    const ov = document.createElement('div');
    ov.className = 'ws-overlay';
    ov.innerHTML =
      '<div class="ws-box" role="dialog" aria-modal="true" aria-labelledby="wsTitle">' +
      '<h3 id="wsTitle">' + esc(options.title || (options.create ? (t('ws.nouProjecte') || 'New project') : (t('ws.titol') || 'Project for this session'))) + '<button class="x" aria-label="' + esc(t('btn.close') || 'Close') + '">×</button></h3>' +
      '<div class="ws-sec">' + esc(t('ws.recents') || 'Recent') + '</div><div class="ws-list" id="wsRecents"><div class="ws-item"><span>' + esc(t('ui.loading') || 'Loading…') + '</span></div></div>' +
      '<div class="ws-sec">' + esc(t('ws.navega') || 'Browse') + '</div>' +
      '<div class="ws-path"><input id="wsPath" spellcheck="false" placeholder="' + esc(t('ws.ruta') || 'Folder path (Enter to go)') + '"/>' +
      (window.gregalDesktop && window.gregalDesktop.chooseFolder ? '<button class="ws-btn" id="wsNatiu">' + esc(t('ws.dialeg') || 'System dialog…') + '</button>' : '') + '</div>' +
      '<div class="ws-crumbs" id="wsCrumbs"></div><div class="ws-dirs" id="wsDirs"></div>' +
      '<div class="ws-foot"><span class="info" id="wsInfo"></span>' +
      '<button class="ws-btn" id="wsCancel">' + esc(t('ws.cancella') || 'Cancel') + '</button>' +
      '<button class="ws-btn primari" id="wsTria">' + esc(options.chooseLabel || (options.create ? (t('ws.creaProjecte') || 'Create project') : (t('ws.tria') || 'Choose this folder'))) + '</button></div></div>';
    document.body.appendChild(ov);
    const q = id => ov.querySelector('#' + id);
    let actual = '';
    const tanca = () => ov.remove();
    // onChoose: qui obre el selector només vol la carpeta (p. ex. una font
    // de coneixement) i no canviar el projecte de la sessió.
    const tria = async path => { if (await (options.onChoose ? options.onChoose(path) : this.set(path, options))) tanca(); };
    ov.querySelector('.x').onclick = tanca;
    q('wsCancel').onclick = tanca;
    ov.addEventListener('click', e => { if (e.target === ov) tanca(); });
    const onKey = e => { if (e.key === 'Escape') { tanca(); document.removeEventListener('keydown', onKey); } };
    document.addEventListener('keydown', onKey);
    // El camp es pot fer servir abans que arribin les dades: si l'usuari hi
    // escriu, la càrrega inicial no li ha de trepitjar el text.
    let escrit = false;
    q('wsPath').addEventListener('input', () => { escrit = true; });
    q('wsPath').focus();

    // Recents
    let recents = [], current = '';
    try { const r = await j('/api/workspaces'); recents = r.workspaces || []; current = r.current || ''; } catch (e) {}
    const rl = q('wsRecents');
    rl.innerHTML = recents.length ? recents.slice(0, 8).map(w =>
      '<div class="ws-item' + (w.path === current ? ' cur' : '') + '" data-p="' + esc(w.path) + '"><b>' + esc(w.name || w.path) + '</b><span>' + esc(w.path) + '</span>' +
      (w.path === current ? '<span class="git">' + esc(t('ws.actual') || 'current') + '</span>' : '') + '</div>').join('')
      : '<div class="ws-item"><span>' + esc(t('ws.capRecent') || 'No recent projects.') + '</span></div>';
    rl.querySelectorAll('.ws-item[data-p]').forEach(el => { el.onclick = () => tria(el.dataset.p); });

    // Navegador
    // Cada navegació té número: una resposta que arriba tard (la inicial,
    // p. ex.) no pot pintar per sobre d'una de més nova.
    let torn = 0;
    const navega = async (path, inicial) => {
      const meu = ++torn;
      if (!inicial) escrit = false;
      let d;
      try { d = await j('/api/dirs' + (path ? '?path=' + encodeURIComponent(path) : '')); }
      catch (e) { if (meu === torn) { q('wsInfo').textContent = String(e.message || e); q('wsInfo').title = q('wsInfo').textContent; } return; }
      if (meu !== torn) return;
      actual = d.path;
      if (!(inicial && escrit)) q('wsPath').value = d.path;
      q('wsInfo').textContent = (d.git ? '⎇ ' : '') + d.path + (d.hidden ? ' · ' + d.hidden + ' ' + (t('ws.amagades') || 'hidden') : '');
      q('wsInfo').title = q('wsInfo').textContent;
      const parts = [];
      if (d.roots && d.roots.length > 1) parts.push('<button data-p="' + esc(d.roots[0]) + '" title="' + esc(d.roots.join(' · ')) + '">' + esc(t('ws.unitats') || 'Drives') + '</button>');
      if (d.home) parts.push('<button data-p="' + esc(d.home) + '">~</button>');
      if (d.parent) parts.push('<button data-p="' + esc(d.parent) + '">↑ ' + esc(t('ws.amunt') || 'up') + '</button>');
      // Engrunes: cada tros de la ruta és clicable.
      const sep = d.path.includes('\\') ? '\\' : '/';
      const trossos = d.path.split(/[\\/]+/).filter(Boolean);
      let acum = d.path.startsWith('/') ? '' : '';
      const crumbs = trossos.map((tr, i) => {
        acum = i === 0 ? (d.path.startsWith('/') ? '/' + tr : tr + (tr.endsWith(':') ? sep : '')) : acum.replace(/[\\/]$/, '') + sep + tr;
        return '<button data-p="' + esc(acum) + '">' + esc(tr.replace(/[\\/]$/, '')) + '</button>';
      });
      q('wsCrumbs').innerHTML = parts.join('') + (parts.length ? '<span>│</span>' : '') + crumbs.join('<span>›</span>');
      q('wsCrumbs').querySelectorAll('button').forEach(b => { b.onclick = () => navega(b.dataset.p); });
      let unitats = null;
      if (d.roots && d.roots.length > 1) {
        unitats = d.roots.map(r => '<button class="ws-dir" data-p="' + esc(r) + '">🖴 ' + esc(r) + '</button>').join('');
      }
      const llista = (d.dirs || []).map(e =>
        '<button class="ws-dir" data-p="' + esc(e.path) + '" title="' + esc(e.path) + '">📁 <span>' + esc(e.name) + '</span>' + (e.git ? '<span class="git">git</span>' : '') + '</button>').join('');
      q('wsDirs').innerHTML = llista || '<div class="ws-dir" style="color:var(--faint)">' + esc(t('ws.buida') || 'No subfolders.') + '</div>';
      q('wsDirs').querySelectorAll('.ws-dir[data-p]').forEach(b => {
        b.onclick = () => navega(b.dataset.p);
        b.ondblclick = () => tria(b.dataset.p);
      });
      q('wsCrumbs').dataset.unitats = unitats || '';
      const unitatsBtn = q('wsCrumbs').querySelector('button[title]');
      if (unitatsBtn && unitats) unitatsBtn.onclick = () => { q('wsDirs').innerHTML = unitats; q('wsDirs').querySelectorAll('.ws-dir').forEach(b => { b.onclick = () => navega(b.dataset.p); }); };
    };
    q('wsPath').addEventListener('keydown', e => { if (e.key === 'Enter') { e.preventDefault(); navega(q('wsPath').value.trim()); } });
    q('wsTria').onclick = () => tria(q('wsPath').value.trim() || actual);
    if (q('wsNatiu')) q('wsNatiu').onclick = async () => {
      const dir = await window.gregalDesktop.chooseFolder();
      if (dir) tria(dir);
    };
    await navega(current || '', true);
  },
  async set(path, options = {}) {
    try {
      if (options.create) {
        await sessions.open(path);
        G().setView('agent');
        return true;
      }
      await j('/api/workspaces', { method: 'POST', body: JSON.stringify({ path }) });
      files.cache = null;
      await G().refresh();
      files.load(); changes.load();
      if (window.gregalPanels && window.gregalPanels.convs) window.gregalPanels.convs.draw();
      // La pestanya duu el nom del projecte: sense repintar-la, deia
      // l'antic fins al missatge següent.
      if (window.gregalPanels && window.gregalPanels.sessions) window.gregalPanels.sessions.render();
      // Amb la conversa buida, l'avís deixava la pantalla inicial com una
      // sola línia «Projecte · …»: el canvi ja es veu a la capçalera.
      const conversa = document.getElementById('main');
      if (!options.quiet && conversa && conversa.classList.contains('on')) G().sys(T('ui.project') + ' · ' + path);
      return true;
    } catch (e) {
      alert(T('conv.open.error') + e.message);
      return false;
    }
  },
};
