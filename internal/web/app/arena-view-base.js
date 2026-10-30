// La base comuna dels visors de partits de l'Arena (football-view.js,
// arena-cs2-view.js, arena-lol-view.js): el bucle de fotogrames, la consulta
// periòdica del partit, la mida del canvas i el progrés, més uns quants
// ajudants petits que els tres feien servir repetits.
//
// El bucle s'atura del tot (fotogrames i consultes) quan el visor no es veu:
// la pestanya amagada (visibilitychange) o el canvas fora de la pantalla o dins
// d'una pàgina amagada (IntersectionObserver). En pausa només es redibuixa
// quan alguna cosa canvia (invalidate()). El canvas es torna a mesurar quan
// canvia el seu contenidor (ResizeObserver), no només la finestra, i la
// densitat de píxels té sostre (2, o 1.5 en pantalles tàctils): un mòbil de
// 3× ja no pinta 1170×2532 píxels a cada fotograma. Als mòbils (o si els
// fotogrames surten cars) el visor va a 30 fps.
//
// Una subclasse posa els seus camps, crida this.start() al final del
// constructor i implementa resize(), draw(), onFrame(dt, now) i poll().

export const lerp = (a, b, t) => a + (b - a) * t;
export const clamp = (v, a, b) => Math.max(a, Math.min(b, v));
export const easeOut = t => 1 - Math.pow(1 - t, 3);
export const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
// Generador d'atzar amb llavor (el mateix partit es veu igual cada cop).
export function rng(seed) { let s = seed >>> 0 || 1; return () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296); }
export function hexToRgb(h, fb = [128, 128, 128]) { const m = /^#?([0-9a-f]{6})$/i.exec(h || ''); if (!m) return fb; const n = parseInt(m[1], 16); return [n >> 16, (n >> 8) & 255, n & 255]; }

// Playback seconds now: when the current event started plus how long it has been on (capped at its length).
export function elapsedAt(times, index, since) {
  const n = times.length - 1, i = Math.max(0, Math.min(n, index)), t0 = times[Math.max(0, i - 1)], t1 = times[i];
  return { times, duration: times[n], elapsed: i >= n && index > 0 ? times[n] : i === 0 ? Math.min(times[0], Math.max(0, since || 0)) : Math.min(t1, t0 + Math.max(0, since || 0)) };
}

// Progress-bar marks: big moments (hl) and, when scores=true, events that move
// the score up. [{ i, kind: 'score'|'hl', text }], cached while the list doesn't grow.
const MARKS = new WeakMap();
export function progressMarks(list, scores = true) {
  const c = MARKS.get(list);
  if (c && c.len === list.length && c.scores === scores) return c.marks;
  const marks = []; let prev = [0, 0];
  list.forEach((e, i) => {
    const sc = Array.isArray(e.score) ? e.score : null;
    const up = scores && sc && (sc[0] > prev[0] || sc[1] > prev[1]);
    if (sc) prev = sc;
    if (up || e.hl) marks.push({ i, kind: up ? 'score' : 'hl', text: e.text || '' });
  });
  MARKS.set(list, { len: list.length, scores, marks });
  return marks;
}

const hasWin = () => typeof window !== 'undefined';
const mq = q => { try { return hasWin() && typeof window.matchMedia === 'function' && window.matchMedia(q).matches; } catch (e) { return false; } };
// Pantalla tàctil (mòbil, tauleta): el dit és el punter principal.
export function isTouch() { return mq('(pointer: coarse)'); }
// La densitat de píxels del canvas: la de la pantalla amb sostre (2; 1.5 al tàctil).
export function viewDpr() {
  const d = (hasWin() && window.devicePixelRatio) || 1;
  return Math.min(d, isTouch() ? 1.5 : 2);
}
// Mòbil o estalvi de dades: n'hi ha prou amb 30 fps.
export function lowPower() {
  const nav = typeof navigator !== 'undefined' ? navigator : null;
  return isTouch() || !!(nav && nav.connection && nav.connection.saveData) || mq('(prefers-reduced-motion: reduce)');
}

const raf = f => (typeof requestAnimationFrame === 'function' ? requestAnimationFrame(f) : 0);
const caf = id => { if (id && typeof cancelAnimationFrame === 'function') cancelAnimationFrame(id); };
const now = () => (typeof performance !== 'undefined' ? performance.now() : Date.now());

export class ArenaViewBase {
  constructor(canvas, opts = {}) {
    this.cv = canvas; this.ctx = canvas.getContext('2d');
    this.speed = 1; this.paused = false; this.stopped = false;
    this.last = now();
    this.dirty = true; this.rafId = 0; this.lastDraw = -1e9;
    this.vis = { page: true, screen: true };
    this.pollTimer = 0; this.pollDue = null;
    // fps: 60 o 30; 'auto' baixa a 30 als mòbils o quan els fotogrames costen massa.
    this.fpsPref = opts.fps || 'auto';
    this.fps = this.fpsPref === 'auto' ? (lowPower() ? 30 : 60) : +this.fpsPref || 60;
    this.cost = 0;
    this.frame = this.frame.bind(this);
  }

  // Engega el visor: mida, observadors, el primer fotograma i la primera consulta.
  start() {
    // resize() es pot cridar de fora (pantalla completa): després cal repintar.
    const resize = this.resize.bind(this);
    this.resize = () => { resize(); this.invalidate(); };
    this.onWinResize = () => this.remeasure();
    if (hasWin() && window.addEventListener) window.addEventListener('resize', this.onWinResize);
    this.remeasure(true);
    this.observe();
    this.wake();
    this.poll();
  }

  // Torna a mesurar el canvas si la seva mida a la pàgina ha canviat (o si force).
  remeasure(force) {
    const r = this.cv.getBoundingClientRect ? this.cv.getBoundingClientRect() : { width: this.cv.width, height: this.cv.height };
    // Amagat (display: none): mida zero; es mesura quan torni a sortir.
    if (!force && (!r.width || !r.height)) return;
    const key = Math.round(r.width) + 'x' + Math.round(r.height) + '@' + viewDpr();
    if (!force && key === this.sizeKey) return;
    this.sizeKey = key;
    this.resize();
  }

  observe() {
    if (!hasWin()) return;
    const cv = this.cv;
    if (typeof ResizeObserver === 'function' && cv && cv.nodeType === 1) {
      this.ro = new ResizeObserver(() => this.remeasure());
      this.ro.observe(cv);
      if (cv.parentElement) this.ro.observe(cv.parentElement);
    }
    if (typeof IntersectionObserver === 'function' && cv && cv.nodeType === 1) {
      this.io = new IntersectionObserver(es => { for (const e of es) this.setShown('screen', e.isIntersecting); });
      this.io.observe(cv);
    }
    if (typeof document !== 'undefined' && document.addEventListener) {
      this.onVis = () => this.setShown('page', !document.hidden);
      document.addEventListener('visibilitychange', this.onVis);
      this.vis.page = !document.hidden;
    }
  }

  // Es veu? (pestanya visible i canvas a la pantalla)
  active() { return !this.stopped && this.vis.page && this.vis.screen; }

  setShown(what, on) {
    if (this.vis[what] === on) return;
    this.vis[what] = on;
    if (this.active()) {
      // Torna: un fotograma nou (sense salt de temps) i la consulta que s'havia aturat.
      this.last = now(); this.invalidate(); this.wake();
      if (this.pollDue != null) this.schedulePoll(Math.max(0, this.pollDue - now()));
    } else {
      caf(this.rafId); this.rafId = 0;
      clearTimeout(this.pollTimer); this.pollTimer = 0;
    }
  }

  // Cal tornar a pintar (en pausa no es pinta si no ha canviat res).
  invalidate() { this.dirty = true; if (this.active()) this.wake(); }
  wake() { if (!this.rafId && this.active()) this.rafId = raf(this.frame) || 0; }

  // La propera consulta d'aquí a ms (s'atura mentre el visor no es veu).
  schedulePoll(ms) {
    clearTimeout(this.pollTimer); this.pollTimer = 0;
    this.pollDue = now() + ms;
    if (!this.active()) return;
    this.pollTimer = setTimeout(() => { this.pollTimer = 0; this.pollDue = null; this.poll(); }, ms);
  }

  // Compatibilitat: abans cada visor tenia el seu loop(now).
  loop(t) { this.frame(t); }

  frame(t) {
    this.rafId = 0;
    if (!this.active()) return;
    if (t == null) t = now();
    // En pausa i sense canvis: no es pinta (ni es demana cap fotograma més fins a invalidate()).
    if (this.paused && !this.dirty) { this.last = t; return; }
    this.rafId = raf(this.frame) || 0;
    if (this.fps < 60 && t - this.lastDraw < 1000 / this.fps - 2) return;
    this.lastDraw = t;
    const dt = Math.min(0.25, Math.max(0, (t - this.last) / 1000)); this.last = t;
    this.dirty = false;
    const t0 = now();
    this.onFrame(dt, t);
    this.adapt(now() - t0);
  }

  // Si els fotogrames surten cars de manera sostinguda, a 30 fps.
  // (Els primers fotogrames, que omplen les memòries cau, no compten.)
  adapt(ms) {
    this.nDrawn = (this.nDrawn || 0) + 1;
    if (this.nDrawn < 90) return;
    this.cost = this.cost ? this.cost * 0.97 + ms * 0.03 : ms;
    if (this.fpsPref === 'auto' && this.fps === 60 && this.nDrawn > 240 && this.cost > 14) this.fps = 30;
  }

  setSpeed(x) { this.speed = x; this.invalidate(); }
  togglePause() { this.paused = !this.paused; this.invalidate(); return this.paused; }

  destroy() {
    this.stopped = true;
    caf(this.rafId); this.rafId = 0;
    clearTimeout(this.pollTimer); this.pollTimer = 0;
    if (hasWin() && window.removeEventListener) window.removeEventListener('resize', this.onWinResize);
    if (this.ro) this.ro.disconnect();
    if (this.io) this.io.disconnect();
    if (this.onVis && typeof document !== 'undefined') document.removeEventListener('visibilitychange', this.onVis);
  }

  // A implementar.
  resize() {}
  onFrame() {}
  poll() {}
}
