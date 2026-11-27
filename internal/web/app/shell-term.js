// Terminal interactiu de la pestanya Terminal del panell lateral
// (internal/web/terminal.go): un shell de debò de la sessió. La pantalla
// arriba per SSE amb el mateix format que les targetes del Kanban, i les
// tecles es codifiquen igual (kanban.js). Les línies que surten per dalt
// queden a l'historial, a sobre de la pantalla, i es llegeixen fent scroll.
import { keyToBytes, mouseBytes } from './kanban.js';

const $ = id => document.getElementById(id);
const T = k => (window.gregalT && window.gregalT(k)) || k;
const post = (path, body) => window.api(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
  .then(async r => {
    const j = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(j.error || ('HTTP ' + r.status));
    return j;
  });
const HISTORY_MAX = 2000; // com historyMax a internal/vterm

export const shellTerm = {
  session: null, abort: null, rows: [], modes: {}, queue: '', sending: false, cell: null, size: '', wired: false,

  // open: s'ensenya la pestanya. Engega (o recupera) el shell de la sessió.
  open() {
    if (!$('shellTerm')) return;
    this.wire();
    const session = window.gregalSession || 'default';
    if (this.session === session && this.abort) { this.fit(); this.focus(); return; }
    this.stop();
    this.session = session;
    const cwd = window.gregalCwd || '', where = $('shellTermCwd');
    where.textContent = cwd.split(/[\\/]/).filter(Boolean).pop() || '';
    where.title = cwd;
    this.start();
  },

  async start() {
    $('shellHist').innerHTML = ''; $('shellScreen').innerHTML = ''; this.rows = [];
    this.state(T('shellTerm.connecting'), false, 'connecting');
    const d = this.dims();
    try { await post('/api/terminal', d); } catch (e) { this.state(T('shellTerm.failed') + e.message, true, 'exited'); return; }
    this.size = d.cols + 'x' + d.rows;
    this.stream();
    this.focus();
  },

  stop() { if (this.abort) { this.abort.abort(); this.abort = null; } },
  focus() { const inp = $('shellInput'); if (inp && !$('terminalPage').hidden) inp.focus(); },

  // state: el text de sota la pantalla i l'estat de la capçalera
  // (connecting, live, exited), que pinta el punt i atenua la pantalla.
  state(text, restart, kind) {
    const st = $('shellTermState');
    $('shellTermWrap').dataset.state = kind;
    st.textContent = text;
    if (restart) {
      const b = document.createElement('button');
      b.type = 'button'; b.className = 'ghost tiny'; b.textContent = T('shellTerm.restart');
      b.onclick = () => this.start();
      st.append(' ', b);
    }
    st.hidden = !text;
  },

  wire() {
    if (this.wired) return;
    this.wired = true;
    const box = $('shellTerm'), inp = $('shellInput');
    // Com al Kanban: keydown per a les tecles especials i «input» per al
    // text (també el d'un IME o un teclat de mòbil).
    inp.onkeydown = e => this.key(e);
    $('shellCopy').onclick = () => this.copy();
    // Ctrl+L: el shell (i el programa que hi corri) redibuixa la pantalla
    // neta; l'historial d'aquí es buida a mà.
    $('shellClear').onclick = () => { $('shellHist').innerHTML = ''; this.send('\x0c'); this.focus(); };
    inp.oninput = e => {
      if (e.isComposing) return;
      const v = inp.value; inp.value = '';
      if (v) this.send(v.replace(/\n/g, '\r'));
    };
    inp.oncompositionend = () => { const v = inp.value; inp.value = ''; if (v) this.send(v); };
    inp.onpaste = e => {
      const text = (e.clipboardData || window.clipboardData).getData('text');
      e.preventDefault();
      if (text) this.send(this.modes.paste ? '\x1b[200~' + text + '\x1b[201~' : text);
    };
    // La roda fa scroll de l'historial, tret que el programa hagi demanat el ratolí.
    box.onwheel = e => {
      if (!this.modes.mouse) return;
      e.preventDefault();
      const [x, y] = this.cellAt(e);
      this.send(mouseBytes(e.deltaY < 0 ? 64 : 65, x, y));
    };
    // Un clic sense arrossegar posa el focus a l'entrada; seleccionar text per copiar-lo continua funcionant.
    box.onmouseup = () => { if (!String(window.getSelection())) this.focus(); };
    let t = 0;
    new ResizeObserver(() => { clearTimeout(t); t = setTimeout(() => this.fit(), 150); }).observe(box);
    document.addEventListener('gregal:session', () => {
      this.stop(); this.session = null;
      if (!$('terminalPage').hidden) this.open();
    });
  },

  measure() {
    if (this.cell) return this.cell;
    const box = $('shellTerm'), probe = document.createElement('span');
    probe.className = 'kb-probe';
    probe.textContent = 'MMMMMMMMMM';
    box.appendChild(probe);
    const r = probe.getBoundingClientRect();
    probe.remove();
    const lh = parseFloat(getComputedStyle(box).lineHeight) || r.height;
    if (r.width > 0) this.cell = { w: r.width / 10, h: lh };
    return this.cell || { w: 8, h: 17 };
  },

  dims() {
    const box = $('shellTerm'), c = this.measure(), cs = getComputedStyle(box);
    const w = box.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight);
    const h = box.clientHeight - parseFloat(cs.paddingTop) - parseFloat(cs.paddingBottom);
    return {
      cols: Math.max(20, Math.min(400, Math.floor(w / c.w) || 80)),
      rows: Math.max(5, Math.min(200, Math.floor(h / c.h) || 24)),
    };
  },

  cellAt(e) {
    const scr = $('shellScreen').getBoundingClientRect(), c = this.measure();
    return [Math.max(0, Math.floor((e.clientX - scr.left) / c.w)), Math.max(0, Math.floor((e.clientY - scr.top) / c.h))];
  },

  // fit ajusta el pseudoterminal a l'espai del panell (el mateix POST que l'obre).
  fit() {
    if (!this.abort || $('terminalPage').hidden) return;
    const d = this.dims(), size = d.cols + 'x' + d.rows;
    if (size === this.size) return;
    this.size = size;
    post('/api/terminal', d).catch(() => {});
  },

  async stream() {
    const ctrl = new AbortController(), session = this.session;
    this.abort = ctrl;
    let exited = false;
    try {
      const r = await window.api('/api/terminal/stream', { signal: ctrl.signal });
      // El servidor ha respost: un shell que ha mort de seguida (un perfil que
      // peta, un $SHELL trencat) no es torna a engegar sol en bucle.
      if (!r.ok) {
        exited = true;
        this.state(T(r.status === 409 ? 'shellTerm.gone' : 'shellTerm.failed') + (r.status === 409 ? '' : 'HTTP ' + r.status), true, 'exited');
        throw new Error('HTTP ' + r.status);
      }
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
          if (!data || this.abort !== ctrl) continue;
          let d; try { d = JSON.parse(data); } catch (e) { continue; }
          if (ev === 'frame') { if (!this.rows.length) this.state('', false, 'live'); this.paint(d); }
          else if (ev === 'history') this.history(d.lines || []);
          else if (ev === 'exited') { exited = true; this.state(T('shellTerm.exited') + d.exit + '.', true, 'exited'); }
        }
      }
    } catch (e) { if (ctrl.signal.aborted) return; }
    if (this.abort !== ctrl) return;
    this.abort = null;
    // Connexió tallada (servidor reiniciat, xarxa): torna a obrir el shell de la sessió.
    if (!exited) setTimeout(() => { if (this.session === session && !this.abort) this.start(); }, 1500);
  },

  // copy copia el text seleccionat o, si no n'hi ha, tota la sortida.
  async copy() {
    const btn = $('shellCopy');
    const text = String(window.getSelection()) || ($('shellHist').innerText + '\n' + $('shellScreen').innerText).replace(/\s+$/, '');
    try { await navigator.clipboard.writeText(text); } catch (e) { btn.title = T('shellTerm.copy') + ': ' + e.message; return; }
    btn.classList.add('done');
    setTimeout(() => btn.classList.remove('done'), 1400);
  },

  atBottom() { const b = $('shellTerm'); return b.scrollHeight - b.scrollTop - b.clientHeight < 4; },

  history(lines) {
    const stick = this.atBottom(), hist = $('shellHist');
    for (const l of lines) {
      const d = document.createElement('div');
      d.className = 'kb-row';
      d.innerHTML = l || '&nbsp;';
      hist.appendChild(d);
    }
    while (hist.childElementCount > HISTORY_MAX) hist.firstElementChild.remove();
    if (stick) $('shellTerm').scrollTop = $('shellTerm').scrollHeight;
  },

  paint(f) {
    const stick = this.atBottom(), scr = $('shellScreen');
    this.modes = { app: f.app_cursor, paste: f.paste, mouse: f.mouse };
    if (f.full || this.rows.length !== f.rows) {
      scr.innerHTML = '';
      this.rows = [];
      for (let i = 0; i < f.rows; i++) {
        const d = document.createElement('div');
        d.className = 'kb-row';
        scr.appendChild(d);
        this.rows.push(d);
      }
    }
    for (const k in f.lines) { const d = this.rows[+k]; if (d) d.innerHTML = f.lines[k]; }
    if (stick) $('shellTerm').scrollTop = $('shellTerm').scrollHeight;
  },

  key(e) {
    if (e.isComposing) return;
    if (e.key.length === 1 && !e.ctrlKey && !e.altKey && !e.metaKey) return;
    if ((e.ctrlKey || e.metaKey) && (e.key === 'v' || e.key === 'V')) return;
    if ((e.ctrlKey || e.metaKey) && (e.key === 'c' || e.key === 'C') && String(window.getSelection())) return;
    if (e.metaKey) return;
    const b = keyToBytes(e, this.modes.app);
    if (b == null) return;
    e.preventDefault();
    $('shellTerm').scrollTop = $('shellTerm').scrollHeight;
    this.send(b);
  },

  // send encua i envia en ordre; el que s'escriu mentre una petició és en
  // vol s'ajunta a la següent.
  send(data) {
    if (!this.abort) return;
    this.queue += data;
    if (!this.sending) this.flush();
  },

  async flush() {
    this.sending = true;
    while (this.queue && this.abort) {
      const data = this.queue;
      this.queue = '';
      try { await post('/api/terminal/input', { data }); } catch (e) { this.queue = ''; break; }
    }
    this.sending = false;
  },
};
