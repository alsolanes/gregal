// Mode Kanban (experimental): un tauler on cada targeta és un TUI de Gregal
// viu (internal/web/kanban.go). Les targetes minimitzades només pinten un
// resum (estat, intenció i les últimes línies de la pantalla, amb colors);
// en clicar-ne una s'obre gran i rep la pantalla sencera per SSE. Les tecles
// es codifiquen aquí com ho faria un terminal i van a /api/kanban/input.
const $ = id => document.getElementById(id);
const T = (key, vars) => {
  let s = (window.gregalT ? window.gregalT(key) : '') || key;
  if (vars) for (const k in vars) s = s.replace('{' + k + '}', vars[k]);
  return s;
};
const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const PREF = 'gregal_exp_kanban';
const COLUMNS = ['definition', 'ready', 'progress', 'review', 'done'];

const api = (path, opts) => window.api(path, opts).then(async r => {
  const j = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(j.error || ('HTTP ' + r.status));
  return j;
});
const post = (path, body) => api(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });

export function kanbanEnabled() {
  try { return localStorage.getItem(PREF) === '1'; } catch (e) { return false; }
}

// Estat visible d'una targeta: el del TUI, o el del procés si no en té.
export function cardState(c) {
  if (c.queued) return 'queued';
  if (!c.alive) return 'stopped';
  return c.activity || 'starting';
}

const base = p => String(p || '').split(/[\\/]/).filter(Boolean).pop() || p || '';

// --- Codificació de tecles (xterm) ---

const SPECIAL = {
  Enter: '\r', Backspace: '\x7f', Tab: '\t', Escape: '\x1b',
  Home: '\x1b[H', End: '\x1b[F', Insert: '\x1b[2~', Delete: '\x1b[3~', PageUp: '\x1b[5~', PageDown: '\x1b[6~',
  F1: '\x1bOP', F2: '\x1bOQ', F3: '\x1bOR', F4: '\x1bOS', F5: '\x1b[15~', F6: '\x1b[17~', F7: '\x1b[18~',
  F8: '\x1b[19~', F9: '\x1b[20~', F10: '\x1b[21~', F11: '\x1b[23~', F12: '\x1b[24~',
};
const ARROWS = { ArrowUp: 'A', ArrowDown: 'B', ArrowRight: 'C', ArrowLeft: 'D' };

// keyToBytes tradueix un keydown al que enviaria un terminal (null = no és
// una tecla per al TUI). appCursor és el mode DECCKM.
export function keyToBytes(e, appCursor) {
  const mod = (e.shiftKey ? 1 : 0) + (e.altKey ? 2 : 0) + (e.ctrlKey ? 4 : 0);
  if (ARROWS[e.key]) {
    const k = ARROWS[e.key];
    if (mod) return '\x1b[1;' + (mod + 1) + k;
    return (appCursor ? '\x1bO' : '\x1b[') + k;
  }
  if (e.key === 'Tab' && e.shiftKey) return '\x1b[Z';
  if (e.key === 'Enter' && e.altKey) return '\x1b\r';
  if (SPECIAL[e.key]) {
    if (mod && /^\x1b\[\d+~$/.test(SPECIAL[e.key])) return SPECIAL[e.key].replace('~', ';' + (mod + 1) + '~');
    if (mod && (e.key === 'Home' || e.key === 'End')) return '\x1b[1;' + (mod + 1) + (e.key === 'Home' ? 'H' : 'F');
    return (e.altKey && e.key.length > 1 && e.key !== 'Escape' ? '\x1b' : '') + SPECIAL[e.key];
  }
  if (e.key.length !== 1 || e.metaKey) return null;
  if (e.ctrlKey && !e.altKey) {
    const c = e.key.toLowerCase();
    if (c >= 'a' && c <= 'z') return String.fromCharCode(c.charCodeAt(0) - 96);
    const map = { ' ': '\x00', '@': '\x00', '[': '\x1b', '\\': '\x1c', ']': '\x1d', '^': '\x1e', '_': '\x1f', '/': '\x1f' };
    return map[e.key] || null;
  }
  return (e.altKey ? '\x1b' : '') + e.key;
}

// Roda i clics en SGR (1006) si el TUI ha demanat el ratolí.
export function mouseBytes(button, x, y, release) {
  return '\x1b[<' + button + ';' + (x + 1) + ';' + (y + 1) + (release ? 'm' : 'M');
}

// --- Tauler ---

const kb = {
  board: null, timer: 0, busy: false, openId: '', drag: '',
  els: new Map(), // id → element de la targeta

  syncNav() {
    document.querySelectorAll('[data-view="kanban"]').forEach(b => { b.hidden = !kanbanEnabled(); });
  },

  async load() {
    const page = $('kanbanPage');
    if (!page || typeof window.api !== 'function') return;
    if (!page.dataset.ready) this.build(page);
    await this.refresh();
  },

  build(page) {
    page.dataset.ready = '1';
    page.innerHTML = `<div class="kb">
      <header class="kb-head">
        <div><div class="page-kicker"><span class="kb-exp">${esc(T('kanban.kicker'))}</span></div>
          <h1 class="page-title">${esc(T('kanban.title'))}</h1>
          <p class="page-copy">${esc(T('kanban.lead'))}</p></div>
        <div class="kb-actions"><span id="kbLive" class="kb-live" role="status"></span>
          <button type="button" id="kbNew" class="primary">${esc(T('kanban.new'))}</button></div>
      </header>
      <form id="kbForm" class="kb-form" hidden>
        <label><span>${esc(T('kanban.form.task'))}</span><textarea id="kbPrompt" rows="2" maxlength="8000" placeholder="${esc(T('kanban.form.taskPh'))}"></textarea></label>
        <div class="kb-form-row">
          <label><span>${esc(T('kanban.form.title'))}</span><input id="kbTitle" maxlength="120" placeholder="${esc(T('kanban.form.titlePh'))}"></label>
          <label><span>${esc(T('kanban.form.dir'))}</span><input id="kbDir" placeholder="${esc(T('kanban.form.dirPh'))}" spellcheck="false"></label>
        </div>
        <div class="kb-form-foot"><span id="kbFormErr" class="kb-err" role="alert"></span>
          <button type="button" id="kbCancel">${esc(T('kanban.form.cancel'))}</button>
          <button type="submit" class="primary">${esc(T('kanban.form.create'))}</button></div>
      </form>
      <div id="kbBoard" class="kb-board">${COLUMNS.map(c => `
        <section class="kb-col" data-col="${c}">
          <header class="kb-col-head"><span class="kb-dot"></span><b>${esc(T('kanban.col.' + c))}</b><span class="kb-count">0</span>
            <span class="kb-col-hint">${esc(T('kanban.col.' + c + '.hint'))}</span></header>
          <div class="kb-cards" data-col="${c}"></div>
        </section>`).join('')}
      </div>
      <div id="kbEmpty" class="kb-empty" hidden>${esc(T('kanban.empty'))}</div>
    </div>`;
    $('kbNew').onclick = () => this.toggleForm(true);
    $('kbCancel').onclick = () => this.toggleForm(false);
    $('kbForm').onsubmit = e => { e.preventDefault(); this.create(); };
    $('kbPrompt').onkeydown = e => { if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); this.create(); } };
    page.querySelectorAll('.kb-cards').forEach(lane => {
      lane.ondragover = e => { if (!this.drag) return; e.preventDefault(); lane.parentElement.classList.add('drop'); };
      lane.ondragleave = () => lane.parentElement.classList.remove('drop');
      lane.ondrop = e => {
        e.preventDefault();
        lane.parentElement.classList.remove('drop');
        const id = this.drag;
        this.drag = '';
        if (id) this.action(id, 'move', { column: lane.dataset.col });
      };
    });
  },

  toggleForm(on) {
    $('kbForm').hidden = !on;
    $('kbFormErr').textContent = '';
    if (on) $('kbPrompt').focus();
  },

  async create() {
    const body = { prompt: $('kbPrompt').value.trim(), title: $('kbTitle').value.trim(), dir: $('kbDir').value.trim() };
    try {
      await post('/api/kanban', body);
      $('kbPrompt').value = ''; $('kbTitle').value = '';
      this.toggleForm(false);
      await this.refresh();
    } catch (e) { $('kbFormErr').textContent = e.message; }
  },

  async action(id, action, extra) {
    try { await post('/api/kanban/card', Object.assign({ id, action }, extra || {})); }
    catch (e) { window.gregalToast ? window.gregalToast(e.message) : alert(e.message); }
    await this.refresh();
  },

  // refresh demana el tauler i, mentre la vista és oberta, es torna a
  // programar. Amb la pestanya amagada no es fa cap petició.
  async refresh() {
    clearTimeout(this.timer);
    const page = $('kanbanPage');
    if (!page || page.hidden) return;
    if (!this.busy && !document.hidden) {
      this.busy = true;
      try { this.board = await api('/api/kanban'); this.paint(); }
      catch (e) { const l = $('kbLive'); if (l) l.textContent = e.message; }
      finally { this.busy = false; }
    }
    this.timer = setTimeout(() => this.refresh(), 1200);
  },

  paint() {
    const b = this.board;
    if (!b) return;
    $('kbLive').textContent = T('kanban.live', { n: b.live, max: b.max_live });
    $('kbEmpty').hidden = b.cards.length > 0;
    const seen = new Set();
    const lanes = {};
    COLUMNS.forEach(c => { lanes[c] = []; });
    b.cards.forEach(c => { (lanes[c.column] || lanes.ready).push(c); });
    COLUMNS.forEach(col => {
      const lane = document.querySelector('.kb-cards[data-col="' + col + '"]');
      lane.parentElement.querySelector('.kb-count').textContent = lanes[col].length;
      lanes[col].forEach((c, i) => {
        seen.add(c.id);
        let el = this.els.get(c.id);
        if (!el) { el = this.cardEl(c.id); this.els.set(c.id, el); }
        this.fill(el, c);
        if (lane.children[i] !== el) lane.insertBefore(el, lane.children[i] || null);
      });
    });
    for (const [id, el] of this.els) if (!seen.has(id)) { el.remove(); this.els.delete(id); }
    if (this.openId) {
      const c = b.cards.find(x => x.id === this.openId);
      if (c) term.header(c); else term.close();
    }
  },

  cardEl(id) {
    const el = document.createElement('article');
    el.className = 'kb-card';
    el.draggable = true;
    el.tabIndex = 0;
    el.dataset.id = id;
    el.innerHTML = `<div class="kb-card-top"><span class="kb-pulse"></span><b class="kb-card-title"></b><span class="kb-chip"></span></div>
      <div class="kb-card-meta"></div><div class="kb-card-intent"></div><div class="kb-mini"></div>
      <div class="kb-card-tools"><button type="button" data-act="restart" class="ghost tiny"></button><button type="button" data-act="stop" class="ghost tiny"></button><button type="button" data-act="delete" class="ghost tiny kb-del"></button></div>`;
    el.ondragstart = e => { this.drag = id; el.classList.add('dragging'); try { e.dataTransfer.setData('text/plain', id); e.dataTransfer.effectAllowed = 'move'; } catch (x) {} };
    el.ondragend = () => { this.drag = ''; el.classList.remove('dragging'); };
    el.onclick = e => {
      const btn = e.target.closest('button[data-act]');
      if (btn) {
        e.stopPropagation();
        const act = btn.dataset.act;
        if (act === 'delete' && !confirm(T('kanban.deleteConfirm'))) return;
        this.action(id, act);
        return;
      }
      term.open(id);
    };
    el.onkeydown = e => { if (e.key === 'Enter' && e.target === el) term.open(id); };
    return el;
  },

  fill(el, c) {
    const st = cardState(c);
    const sig = JSON.stringify([c.title, st, c.column, c.detail, c.intent, c.task, c.turns, c.dir, c.exit, c.manual, c.tail]);
    el.classList.toggle('active', c.alive && c.idle_ms != null && c.idle_ms < 1500);
    if (el.dataset.sig === sig) return;
    el.dataset.sig = sig;
    el.dataset.state = st;
    el.dataset.col = c.column;
    el.classList.toggle('stopped', !c.alive);
    el.querySelector('.kb-card-title').textContent = c.title;
    el.querySelector('.kb-card-title').title = c.prompt || c.title;
    const chip = el.querySelector('.kb-chip');
    chip.textContent = T('kanban.state.' + st) + (st === 'stopped' && c.exit != null && c.exit >= 0 ? ' · ' + T('kanban.exit', { code: c.exit }) : '');
    const meta = [base(c.dir)];
    if (c.turns) meta.push(T('kanban.turns', { n: c.turns }));
    if (c.manual) meta.push(T('kanban.manual'));
    el.querySelector('.kb-card-meta').textContent = meta.join(' · ');
    el.querySelector('.kb-card-meta').title = c.dir;
    el.querySelector('.kb-card-intent').textContent = c.intent || c.task || c.detail || c.prompt || '';
    // Les línies ja venen pintades i escapades del servidor (vterm.LineHTML).
    el.querySelector('.kb-mini').innerHTML = (c.tail || []).map(l => '<div>' + (l || '&nbsp;') + '</div>').join('');
    const [restart, stop, del] = el.querySelectorAll('.kb-card-tools button');
    restart.textContent = '↻ ' + T('kanban.restart'); restart.hidden = c.alive || c.queued;
    stop.textContent = '■ ' + T('kanban.stop'); stop.hidden = !c.alive && !c.queued;
    del.textContent = '✕'; del.title = T('kanban.delete'); del.setAttribute('aria-label', T('kanban.delete'));
  },
};

// --- Terminal gran ---

const term = {
  id: '', abort: null, rows: [], modes: {}, queue: '', sending: false, cell: null, resizeT: 0,

  ensure() {
    let ov = $('kbOverlay');
    if (ov) return ov;
    ov = document.createElement('div');
    ov.id = 'kbOverlay';
    ov.className = 'kb-overlay';
    ov.hidden = true;
    ov.innerHTML = `<div class="kb-win" role="dialog" aria-modal="true" aria-labelledby="kbWinTitle">
      <header class="kb-win-head"><span class="kb-pulse"></span><b id="kbWinTitle" class="kb-win-title" title="${esc(T('kanban.rename'))}"></b>
        <span id="kbWinChip" class="kb-chip"></span><span id="kbWinDir" class="kb-win-dir"></span>
        <span class="kb-win-tools"><button type="button" id="kbWinRestart" class="ghost tiny"></button><button type="button" id="kbWinStop" class="ghost tiny"></button>
        <button type="button" id="kbWinClose" class="kb-close" aria-label="${esc(T('kanban.collapse'))}" title="${esc(T('kanban.collapse'))}">⤡</button></span></header>
      <div id="kbTerm" class="kb-term" aria-label="Terminal"></div>
      <textarea id="kbInput" class="kb-input" autocomplete="off" autocapitalize="off" spellcheck="false" aria-label="Terminal input"></textarea>
      <footer class="kb-win-foot">${esc(T('kanban.termHint'))}</footer>
    </div>`;
    document.body.appendChild(ov);
    ov.onmousedown = e => { if (e.target === ov) this.close(); };
    $('kbWinClose').onclick = () => this.close();
    $('kbWinStop').onclick = () => kb.action(this.id, 'stop');
    $('kbWinRestart').onclick = () => kb.action(this.id, 'restart');
    $('kbWinTitle').ondblclick = () => {
      const t = prompt(T('kanban.rename'), $('kbWinTitle').textContent);
      if (t && t.trim()) kb.action(this.id, 'rename', { title: t.trim() });
    };
    const box = $('kbTerm'), inp = $('kbInput');
    // Les tecles van a un textarea amagat: keydown per a les especials i
    // «input» per al text (també el d'un IME o un teclat de mòbil, que no
    // passa per keydown).
    inp.onkeydown = e => this.key(e);
    inp.oninput = e => {
      if (e.isComposing) return;
      const v = inp.value;
      inp.value = '';
      if (v) this.send(v.replace(/\n/g, '\r'));
    };
    inp.oncompositionend = () => { const v = inp.value; inp.value = ''; if (v) this.send(v); };
    inp.onpaste = e => {
      const text = (e.clipboardData || window.clipboardData).getData('text');
      e.preventDefault();
      if (text) this.send(this.modes.paste ? '\x1b[200~' + text + '\x1b[201~' : text);
    };
    box.onwheel = e => {
      e.preventDefault();
      const [x, y] = this.cellAt(e);
      const up = e.deltaY < 0;
      if (this.modes.mouse) this.send(mouseBytes(up ? 64 : 65, x, y));
      else this.send(up ? '\x1b[5~' : '\x1b[6~');
    };
    box.onmousedown = e => {
      setTimeout(() => inp.focus(), 0);
      if (!this.modes.mouse || e.button !== 0) return;
      const [x, y] = this.cellAt(e);
      this.send(mouseBytes(0, x, y));
    };
    box.onmouseup = e => {
      if (!this.modes.mouse || e.button !== 0) return;
      const [x, y] = this.cellAt(e);
      this.send(mouseBytes(0, x, y, true));
    };
    if (!this.wired) {
      this.wired = true;
      window.addEventListener('resize', () => { clearTimeout(this.resizeT); this.resizeT = setTimeout(() => this.fit(), 200); });
      // Esc fora del terminal tanca; dins, és una tecla per al TUI.
      document.addEventListener('keydown', e => {
        const o = $('kbOverlay');
        if (o && !o.hidden && e.key === 'Escape' && document.activeElement !== $('kbInput')) this.close();
      });
    }
    return ov;
  },

  open(id) {
    const ov = this.ensure();
    if (this.id !== id) this.stopStream();
    this.id = id;
    kb.openId = id;
    ov.hidden = false;
    const c = kb.board && kb.board.cards.find(x => x.id === id);
    if (c) this.header(c);
    $('kbTerm').innerHTML = '';
    this.rows = [];
    this.fit();
    this.stream();
    $('kbInput').focus();
  },

  close() {
    const ov = $('kbOverlay');
    if (ov) ov.hidden = true;
    this.stopStream();
    const id = this.id;
    this.id = ''; kb.openId = '';
    const el = kb.els.get(id);
    if (el) el.focus();
  },

  header(c) {
    const st = cardState(c);
    $('kbWinTitle').textContent = c.title;
    const chip = $('kbWinChip');
    chip.textContent = T('kanban.state.' + st);
    $('kbOverlay').dataset.state = st;
    $('kbOverlay').dataset.col = c.column;
    $('kbOverlay').classList.toggle('active', c.alive && c.idle_ms != null && c.idle_ms < 1500);
    $('kbWinDir').textContent = c.dir;
    $('kbWinRestart').textContent = '↻ ' + T('kanban.restart'); $('kbWinRestart').hidden = c.alive || c.queued;
    $('kbWinStop').textContent = '■ ' + T('kanban.stop'); $('kbWinStop').hidden = !c.alive && !c.queued;
  },

  stopStream() { if (this.abort) { this.abort.abort(); this.abort = null; } },

  // stream llegeix l'SSE de la pantalla (fetch: EventSource no deixa posar
  // el token a les capçaleres).
  async stream() {
    const ctrl = new AbortController();
    this.abort = ctrl;
    const id = this.id;
    try {
      const r = await window.api('/api/kanban/stream?id=' + encodeURIComponent(id), { signal: ctrl.signal });
      if (!r.ok) throw new Error('HTTP ' + r.status);
      const rd = r.body.getReader(), dec = new TextDecoder();
      let buf = '';
      for (;;) {
        const { done, value } = await rd.read();
        if (done) break;
        buf += dec.decode(value, { stream: true });
        let i;
        while ((i = buf.indexOf('\n\n')) >= 0) {
          const frame = buf.slice(0, i); buf = buf.slice(i + 2);
          let ev = '', data = '';
          frame.split('\n').forEach(l => { if (l.startsWith('event:')) ev = l.slice(6).trim(); else if (l.startsWith('data:')) data += l.slice(5).trim(); });
          if (!data || id !== this.id) continue;
          let d; try { d = JSON.parse(data); } catch (e) { continue; }
          if (ev === 'frame') this.paint(d);
          else if (ev === 'stopped') { this.header(d); this.paintTail(d.tail); }
          else if (ev === 'gone') { this.close(); return; }
        }
      }
    } catch (e) { if (ctrl.signal.aborted) return; }
    // Connexió tallada (servidor reiniciat, xarxa): torna-hi si encara és oberta.
    if (this.id === id && this.abort === ctrl) setTimeout(() => { if (this.id === id && this.abort === ctrl) this.stream(); }, 1500);
  },

  paint(f) {
    const box = $('kbTerm');
    this.modes = { app: f.app_cursor, paste: f.paste, mouse: f.mouse, sgr: f.mouse_sgr };
    if (f.full || this.rows.length !== f.rows) {
      box.innerHTML = '';
      this.rows = [];
      for (let i = 0; i < f.rows; i++) {
        const d = document.createElement('div');
        d.className = 'kb-row';
        box.appendChild(d);
        this.rows.push(d);
      }
    }
    for (const k in f.lines) { const d = this.rows[+k]; if (d) d.innerHTML = f.lines[k]; }
    box.style.setProperty('--kb-cols', f.cols);
  },

  paintTail(tail) {
    const box = $('kbTerm');
    box.innerHTML = '<div class="kb-term-stopped">' + (tail || []).map(l => '<div class="kb-row">' + (l || '&nbsp;') + '</div>').join('') + '</div>';
    this.rows = [];
  },

  measure() {
    if (this.cell) return this.cell;
    const box = $('kbTerm');
    const probe = document.createElement('span');
    probe.className = 'kb-probe';
    probe.textContent = 'MMMMMMMMMM';
    box.appendChild(probe);
    const r = probe.getBoundingClientRect();
    probe.remove();
    const lh = parseFloat(getComputedStyle(box).lineHeight) || r.height;
    if (r.width > 0) this.cell = { w: r.width / 10, h: lh };
    return this.cell || { w: 8, h: 17 };
  },

  cellAt(e) {
    const box = $('kbTerm'), r = box.getBoundingClientRect(), c = this.measure();
    const cs = getComputedStyle(box);
    return [Math.max(0, Math.floor((e.clientX - r.left - parseFloat(cs.paddingLeft)) / c.w)), Math.max(0, Math.floor((e.clientY - r.top - parseFloat(cs.paddingTop)) / c.h))];
  },

  // fit ajusta el pseudoterminal a l'espai de la finestra gran.
  fit() {
    if (!this.id || $('kbOverlay').hidden) return;
    const box = $('kbTerm'), c = this.measure(), cs = getComputedStyle(box);
    const w = box.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight);
    const h = box.clientHeight - parseFloat(cs.paddingTop) - parseFloat(cs.paddingBottom);
    const cols = Math.max(40, Math.min(300, Math.floor(w / c.w)));
    const rows = Math.max(10, Math.min(120, Math.floor(h / c.h)));
    post('/api/kanban/resize', { id: this.id, cols, rows }).catch(() => {});
  },

  key(e) {
    if (e.isComposing) return;
    // Text normal: arriba per l'event input.
    if (e.key.length === 1 && !e.ctrlKey && !e.altKey && !e.metaKey) return;
    // Copiar amb text seleccionat i enganxar van pel navegador.
    if ((e.ctrlKey || e.metaKey) && (e.key === 'v' || e.key === 'V')) return;
    if ((e.ctrlKey || e.metaKey) && (e.key === 'c' || e.key === 'C') && String(window.getSelection())) return;
    if (e.metaKey) return;
    const b = keyToBytes(e, this.modes.app);
    if (b == null) return;
    e.preventDefault();
    this.send(b);
  },

  // send encua i envia en ordre; el que s'escriu mentre una petició és en
  // vol s'ajunta a la següent.
  send(data) {
    if (!this.id) return;
    this.queue += data;
    if (this.sending) return;
    this.flush();
  },

  async flush() {
    this.sending = true;
    while (this.queue && this.id) {
      const data = this.queue;
      this.queue = '';
      try { await post('/api/kanban/input', { id: this.id, data }); } catch (e) { this.queue = ''; break; }
    }
    this.sending = false;
  },
};

window.gregalKanban = { load: () => kb.load(), syncNav: () => kb.syncNav(), enabled: kanbanEnabled };

// ?kanban=1 a l'adreça l'activa (i ?kanban=0 l'apaga) sense passar per
// Preferències.
try {
  const q = new URLSearchParams(location.search).get('kanban');
  if (q === '1' || q === '0') localStorage.setItem(PREF, q);
} catch (e) {}
kb.syncNav();
// Si la pàgina ja s'ha obert a la vista Kanban abans que carregués aquest mòdul.
if ($('kanbanPage') && !$('kanbanPage').hidden) kb.load();
document.addEventListener('gregal:fun-prefs', () => kb.syncNav());
document.addEventListener('visibilitychange', () => { if (!document.hidden) kb.refresh(); });
// Canvi d'idioma: el tauler es torna a construir amb els textos nous.
document.addEventListener('gregal:idioma', () => {
  const page = $('kanbanPage');
  if (!page || !page.dataset.ready) return;
  delete page.dataset.ready;
  kb.els.clear();
  $('kbOverlay')?.remove();
  term.id = ''; kb.openId = ''; term.stopStream(); term.cell = null;
  if (!page.hidden) kb.load();
});
