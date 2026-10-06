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
import { aplica as aplicaIdioma, t as T, defineixPerDefecte } from './i18n.js';
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

// ---------------------------------------------------------------- sessions

export const sessions = {
  current: 'default',
  _error: '',
  _lastList: [],
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
    await this.render();
  },
  async list() {
    try {
      const list = (await j('/api/sessions/live')).sessions || [];
      this._lastList = list;
      this._error = '';
      return list;
    } catch (e) {
      this._error = 'No s’han pogut carregar les sessions: ' + String(e.message || e);
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
    const firma = list.map(s => s.id + ':' + (s.busy ? 1 : 0) + ':' + (pending?.has(s.id) ? 1 : 0) + ':' + (s.title || '') + ':' + (s.project || '')).join('|') + '#' + this.current + '#' + this._error;
    if (firma === this._firma) return;
    this._firma = firma;
    bar.innerHTML = '';
    list.forEach(s => {
      const tab = document.createElement('button');
      tab.className = 'session-tab' + (s.id === this.current ? ' active' : '') + (s.busy ? ' busy' : '') + (pending?.has(s.id) ? ' needs-input' : '');
      tab.title = s.cwd + (pending?.has(s.id) ? ' · cal respondre' : s.busy ? ' · treballant' : '');
      const name = s.title || (s.id === 'default' ? s.project || 'principal' : s.id);
      tab.innerHTML = '<span class="dot"></span><span class="nm">' + esc(name) + '</span>' +
        (s.id === 'default' || s.busy ? '' : '<span class="x" title="Tanca">×</span>');
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
    plus.title = 'Sessió nova (Ctrl+T): una altra conversa, en paral·lel';
    plus.onclick = () => this.open();
    bar.appendChild(plus);
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
    trans.forEach(m => {
      if (m.role === 'user') G().add('u', '', G().md(m.content));
      else if (m.role === 'assistant' && (m.content || '').trim()) G().add('a', '', G().md(m.content));
    });
    G().sys('Sessió · ' + id + (trans.length ? ' · ' + trans.length + ' missatges' : ''));
    // La barra de pestanyes i els panells, després i sense esperar-los: el
    // que volies era canviar de conversa, no mirar el diff.
    this.render();
    const vista = G().state && G().viewActual ? G().viewActual() : '';
    if (vista === 'canvis') changes.load();
    if (vista === 'fitxers') files.load();
    return st;
  },
  async close(id) {
    try {
      await j('/api/sessions/close', { method: 'POST', body: JSON.stringify({ id }) });
    } catch (e) {
      this.setError('No s’ha pogut tancar la sessió: ' + String(e.message || e));
      G().sys('No s’ha pogut tancar la sessió ' + id + ': ' + String(e.message || e));
      return;
    }
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
      G().sys('Projecte creat · ' + dir);
    }
    catch (e) { G().sys('No s’ha pogut crear el projecte: ' + e.message); }
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
      box.innerHTML = '<div class="activity-empty">No s\'ha pogut llegir el diff: ' + esc(e.message) + '</div>';
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
        ? d.files.length + ' fitxer(s) · +' + d.added + ' −' + d.removed + (d.branch ? ' · ' + d.branch : '')
        : 'Sense canvis al working tree.';
    }
    if (!d.files.length) {
      box.innerHTML = '<div class="activity-empty">Res per revisar. Quan l\'agent toqui fitxers, els canvis apareixeran aquí.</div>';
      return;
    }
    box.innerHTML = '';
    d.files.forEach(f => box.appendChild(this.fileCard(f)));
  },
  fileCard(f) {
    const wrap = document.createElement('details');
    wrap.className = 'diff-file';
    wrap.open = f.hunks && f.hunks.length <= 3;
    const badge = { added: 'nou', deleted: 'esborrat', renamed: 'mogut' }[f.status] || 'modificat';
    const head = document.createElement('summary');
    head.innerHTML = '<span class="dfname">' + esc(f.path) + '</span>' +
      '<span class="dfbadge ' + esc(f.status) + '">' + badge + '</span>' +
      '<span class="dfstat"><b class="add">+' + f.added + '</b> <b class="del">−' + f.removed + '</b></span>';
    const kill = document.createElement('button');
    kill.className = 'ghost tiny';
    kill.textContent = 'Descarta el fitxer';
    kill.onclick = async e => {
      e.preventDefault(); e.stopPropagation();
      if (!confirm('Descartar tots els canvis de ' + f.path + '?')) return;
      await this.discard({ path: f.path });
    };
    head.appendChild(kill);
    const open = document.createElement('button');
    open.className = 'ghost tiny diff-open';
    open.textContent = 'Obre';
    open.title = 'Obre el fitxer a la subfinestra lateral';
    open.onclick = e => { e.preventDefault(); e.stopPropagation(); files.view(f.path); };
    head.appendChild(open);
    wrap.appendChild(head);
    if (f.binary) {
      const b = document.createElement('div');
      b.className = 'activity-empty';
      b.textContent = 'Fitxer binari: no es pot mostrar el diff.';
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
    drop.textContent = 'Descarta';
    drop.title = 'Torna aquest bloc a com estava';
    drop.onclick = () => this.discard({ path: f.path, hunk: i });
    const keep = document.createElement('button');
    keep.className = 'ghost tiny ok';
    keep.textContent = 'Accepta';
    keep.title = 'Deixa el canvi tal com està i marca el bloc com a revisat';
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
      const r = await j('/api/diff/discard', { method: 'POST', body: JSON.stringify(payload) });
      G().activityItem('canvis', 'Descartat', r.summary || payload.path, 'ok');
    } catch (e) {
      G().activityItem('canvis', 'No s\'ha pogut descartar', String(e.message || e), 'bad');
      alert('No s\'ha pogut descartar: ' + e.message);
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
    pane.innerHTML = '<div class="activity-empty">carregant ' + esc(path) + '…</div>';
    try {
      const r = await j('/api/file?path=' + encodeURIComponent(path));
      if (r.binary) { pane.innerHTML = '<div class="activity-empty">Fitxer binari (' + r.size + " bytes).</div>"; return; }
      if (r.too_big) { pane.innerHTML = '<div class="activity-empty">Massa gran per mostrar (' + r.size + ' bytes).</div>'; return; }
      const lines = String(r.content).split('\n');
      pane.innerHTML = '<div class="fv-head"><b>' + esc(path) + '</b>' +
        '<button class="ghost tiny" id="fvMention">Posa-ho al missatge</button></div>' +
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
      if (!v) { G().sys('Adreça web no vàlida.'); return; }
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
      G().setComposer('Revisa aquesta pàgina web: ' + v);
    };
    if (desktop) {
      view.addEventListener('did-navigate', e => { url.value = e.url; });
      view.addEventListener('did-navigate-in-page', e => { url.value = e.url; });
      view.addEventListener('did-fail-load', e => { if (e.errorCode !== -3) G().sys('Navegador: ' + e.errorDescription); });
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
      box.innerHTML = '<div class="activity-empty">Cap procés. Engega\'n un a dalt: els llargs (servidor, build, tests) viuen aquí i no bloquegen el torn.</div>';
      return;
    }
    this.procs.forEach(p => {
      const row = document.createElement('div');
      row.className = 'proc-row' + (p.running ? ' running' : '');
      row.innerHTML = '<span class="dot"></span><code>' + esc(p.cmd) + '</code>' +
        '<span class="pstate">' + (p.running ? 'en marxa' : 'codi ' + (p.exit == null ? '?' : p.exit)) + '</span>';
      const open = document.createElement('button');
      open.className = 'ghost tiny';
      open.textContent = 'Sortida';
      open.onclick = () => this.watch(p.id);
      row.appendChild(open);
      if (p.running) {
        const kill = document.createElement('button');
        kill.className = 'ghost tiny';
        kill.textContent = 'Atura';
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
      G().activityItem('procés', cmd.slice(0, 60), 'engegat', 'warn');
      await this.load();
      this.watch(r.id);
    } catch (e) {
      $('procOut').textContent = 'No s\'ha pogut engegar: ' + e.message;
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

const COMMANDS = [
  { cmd: '/mode', desc: T('keys.modeCycle'), run: () => G().setMode(G().nextMode()) },
  { cmd: '/model', desc: 'tria proveïdor i model', run: () => models.open() },
  { cmd: '/canvis', desc: 'revisa el diff del workspace', run: () => G().setView('canvis') },
  { cmd: '/fitxers', desc: 'arbre del projecte', run: () => G().setView('fitxers') },
  { cmd: '/terminal', desc: 'processos en segon pla', run: () => G().setView('terminal') },
  { cmd: '/nova', desc: 'sessió nova en paral·lel', run: () => sessions.open() },
  { cmd: '/atura', desc: 'atura el torn en curs', run: () => j('/api/agent/cancel', { method: 'POST', body: '{}' }) },
  { cmd: '/desfes', desc: 'desfés els canvis de la sessió', run: () => $('rewindBtn').click() },
  { cmd: '/compara', desc: 'pregunta a diversos models alhora', run: () => $('parallelBtn').click() },
  { cmd: '/pla', desc: 'explora i proposa un pla', run: () => $('planBtn').click() },
  { cmd: '/objectius', desc: 'objectius del projecte', run: () => G().setView('objectius') },
  { cmd: '/activitat', desc: 'registre d\'aquesta sessió', run: () => G().setView('activitat') },
  { cmd: '/permissiu', desc: 'l\'agent tira sense demanar', run: () => $('permBtn').click() },
];

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
      const selected = this.data.selected_override || 'el model seleccionat';
      const fallback = this.data.fallback;
      footHTML += '<div class="mp-err">El model seleccionat «' + esc(selected) + '» no està disponible.' +
        (fallback ? ' S’utilitza «' + esc(fallback) + '» temporalment.' : ' No hi ha cap model alternatiu configurat.') +
        '</div>';
    }
    if (errs.length) {
      const summaryText = errs.length === 1
        ? '1 proveïdor desconnectat (' + esc(errs[0][0]) + ')'
        : errs.length + ' proveïdors desconnectats (' + errs.map(e => esc(e[0])).join(', ') + ')';
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
      this.items = COMMANDS.filter(c => c.cmd.startsWith('/' + t.word)).slice(0, 8);
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
      const cmd = COMMANDS.find(c => c.cmd === it.cmd);
      if (cmd) cmd.run();
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
    sideCompactBtn.title = compact ? 'Amplia la barra lateral' : 'Compacta la barra lateral';
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
  // El nom del projecte a la capçalera també obre el selector: és on
  // mires per saber on ets, i on esperes poder canviar-ho.
  const hdrProj = $('project');
  if (hdrProj) { hdrProj.style.cursor = 'pointer'; hdrProj.title = (hdrProj.title || '') + ' · clic per canviar de projecte'; hdrProj.onclick = () => workspaces.pick(); }
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
  let ticsOffline = 0;
  setInterval(async () => {
    if (window.gregalOffline && (++ticsOffline % 3) !== 0) return;
    try {
      const hadError = !!sessions._error;
      const list = await sessions.list();
      if (G()?.viewActual() === 'workshop') await workshop.update(list);
      if (list.some(s => s.busy) || sessions._error || hadError) await sessions.render(list);
    } catch (e) {}
  }, 5000);
  const retry = document.getElementById('offlineRetry');
  if (retry) retry.onclick = () => { G().refresh(); };
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
      '<h3 id="wsTitle">' + esc(options.create ? 'Nou projecte' : (t('ws.titol') || 'Projecte d’aquesta sessió')) + '<button class="x" aria-label="Tanca">×</button></h3>' +
      '<div class="ws-sec">' + esc(t('ws.recents') || 'Recents') + '</div><div class="ws-list" id="wsRecents"><div class="ws-item"><span>' + esc(t('ws.carregant') || 'Carregant…') + '</span></div></div>' +
      '<div class="ws-sec">' + esc(t('ws.navega') || 'Navega') + '</div>' +
      '<div class="ws-path"><input id="wsPath" spellcheck="false" placeholder="' + esc(t('ws.ruta') || 'Ruta de la carpeta (Enter per anar-hi)') + '"/>' +
      (window.gregalDesktop && window.gregalDesktop.chooseFolder ? '<button class="ws-btn" id="wsNatiu">' + esc(t('ws.dialeg') || 'Diàleg del sistema…') + '</button>' : '') + '</div>' +
      '<div class="ws-crumbs" id="wsCrumbs"></div><div class="ws-dirs" id="wsDirs"></div>' +
      '<div class="ws-foot"><span class="info" id="wsInfo"></span>' +
      '<button class="ws-btn" id="wsCancel">' + esc(t('ws.cancella') || 'Cancel·la') + '</button>' +
      '<button class="ws-btn primari" id="wsTria">' + esc(options.create ? 'Crea projecte' : (t('ws.tria') || 'Tria aquesta carpeta')) + '</button></div></div>';
    document.body.appendChild(ov);
    const q = id => ov.querySelector('#' + id);
    let actual = '';
    const tanca = () => ov.remove();
    const tria = async path => { if (await this.set(path, options)) tanca(); };
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
      (w.path === current ? '<span class="git">' + esc(t('ws.actual') || 'actual') + '</span>' : '') + '</div>').join('')
      : '<div class="ws-item"><span>' + esc(t('ws.capRecent') || 'Cap projecte recent.') + '</span></div>';
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
      q('wsInfo').textContent = (d.git ? '⎇ ' : '') + d.path + (d.hidden ? ' · ' + d.hidden + ' ' + (t('ws.amagades') || 'amagades') : '');
      q('wsInfo').title = q('wsInfo').textContent;
      const parts = [];
      if (d.roots && d.roots.length > 1) parts.push('<button data-p="' + esc(d.roots[0]) + '" title="' + esc(d.roots.join(' · ')) + '">' + esc(t('ws.unitats') || 'Unitats') + '</button>');
      if (d.home) parts.push('<button data-p="' + esc(d.home) + '">~</button>');
      if (d.parent) parts.push('<button data-p="' + esc(d.parent) + '">↑ ' + esc(t('ws.amunt') || 'amunt') + '</button>');
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
      q('wsDirs').innerHTML = llista || '<div class="ws-dir" style="color:var(--faint)">' + esc(t('ws.buida') || 'Sense subcarpetes.') + '</div>';
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
      if (!options.quiet && conversa && conversa.classList.contains('on')) G().sys('Projecte · ' + path);
      return true;
    } catch (e) {
      alert('No s\'ha pogut obrir: ' + e.message);
      return false;
    }
  },
};
