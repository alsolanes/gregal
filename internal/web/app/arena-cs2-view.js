// Visor d'un partit de Counter-Strike 2 de l'Arena (internal/arena/cs2): un
// minimapa tàctic com el de les retransmissions (Mirage estilitzat, zones,
// caixes, llocs A i B), jugadors amb el color de l'equip i la vora del bàndol,
// cons de visió, traçadores, fums, flaixos i molotovs, la bomba amb el seu
// rellotge, el marcador de dalt, el feed de morts, les plantilles de baix i
// els rètols de final de ronda, mitja part i final amb l'MVP.
//
// Les rondes arriben senceres (un punt de decisió = una ronda); cada ronda es
// reprodueix comprimint els temps morts perquè duri uns 8-15 s a 1×.

const lerp = (a, b, t) => a + (b - a) * t;
const clamp = (v, a, b) => Math.max(a, Math.min(b, v));
const easeOut = t => 1 - Math.pow(1 - t, 3);
const FONT = '"Segoe UI", system-ui, sans-serif';
const SIDE_COL = { T: '#e8a33d', CT: '#5aa9e6' };
const ROUND_TIME = 115, BOMB_TIME = 40;

function rng(seed) { let s = seed >>> 0 || 1; return () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296); }
function hexToRgb(h) { const m = /^#?([0-9a-f]{6})$/i.exec(h || ''); if (!m) return [140, 140, 140]; const n = parseInt(m[1], 16); return [n >> 16, (n >> 8) & 255, n & 255]; }
function rgba(h, a) { const [r, g, b] = hexToRgb(h); return `rgba(${r},${g},${b},${a})`; }
function luminance(h) { const [r, g, b] = hexToRgb(h); return (0.299 * r + 0.587 * g + 0.114 * b) / 255; }

export const TEXT = {
  ca: {
    round: 'RONDA', half: 'MITJA PART', ot: 'PRÒRROGA', otSwitch: 'CANVI DE BANDA', final: 'FINAL', mvp: 'MVP', live: 'EN DIRECTE',
    waiting: 'Esperant la ronda següent…', pregame: 'Els equips preparen l\'estratègia…', stopped: 'Partit aturat', failed: 'El partit ha fallat',
    wins: 'guanya la ronda', winsMatch: 'GUANYA EL PARTIT', pistol: 'RONDA DE PISTOLES',
    reasons: { elim: 'Eliminació', bomb: 'La bomba ha explotat', defuse: 'Bomba desactivada', time: 'Temps esgotat' },
    buy: { eco: 'ECO', half: 'MITJA COMPRA', force: 'COMPRA FORÇADA', full: 'COMPRA COMPLETA' },
    tags: { pistol: 'pistoles', clutch: 'CLUTCH', ace: 'ACE!', '4k': '4 MORTS', eco_win: 'VICTÒRIA EN ECO', ninja: 'NINJA!', last_second: 'A L\'ÚLTIM SEGON' },
    cols: ['M', 'Mo', 'DPR', '%CAP'], switchNote: 'Canvi de banda, cada jugador comença amb', kills: 'morts', planted: 'BOMBA PLANTADA', defusing: 'DESACTIVANT', planting: 'PLANTANT',
  },
  en: {
    round: 'ROUND', half: 'HALF-TIME', ot: 'OVERTIME', otSwitch: 'SIDE SWITCH', final: 'FINAL', mvp: 'MVP', live: 'LIVE',
    waiting: 'Waiting for the next round…', pregame: 'The teams are preparing their strategy…', stopped: 'Match stopped', failed: 'Match failed',
    wins: 'win the round', winsMatch: 'WIN THE MATCH', pistol: 'PISTOL ROUND',
    reasons: { elim: 'Elimination', bomb: 'Bomb exploded', defuse: 'Bomb defused', time: 'Time ran out' },
    buy: { eco: 'ECO', half: 'HALF BUY', force: 'FORCE BUY', full: 'FULL BUY' },
    tags: { pistol: 'pistol', clutch: 'CLUTCH', ace: 'ACE!', '4k': '4K', eco_win: 'ECO WIN', ninja: 'NINJA!', last_second: 'LAST SECOND' },
    cols: ['K', 'D', 'ADR', 'HS%'], switchNote: 'Sides switch, every player starts with', kills: 'kills', planted: 'BOMB PLANTED', defusing: 'DEFUSING', planting: 'PLANTING',
  },
};

const WEAPON = { glock: 'Glock', usp: 'USP-S', p250: 'P250', deagle: 'Deagle', mac10: 'MAC-10', mp9: 'MP9', galil: 'Galil', famas: 'FAMAS', ak47: 'AK-47', m4a1s: 'M4A1-S', awp: 'AWP' };

// ---- ajudes pures (es proven sense DOM) ----

export function dataOf(e) {
  if (!e) return {};
  if (e._d) return e._d;
  let d = e.data;
  if (typeof d === 'string') { try { d = JSON.parse(d); } catch { d = {}; } }
  const out = d && typeof d === 'object' ? d : {};
  try { Object.defineProperty(e, '_d', { value: out, enumerable: false }); } catch { /* congelat */ }
  return out;
}

export function fmtClock(s) { s = Math.max(0, Math.ceil(s)); return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`; }
export function money(v) { return '$' + Math.round(v || 0).toLocaleString('en-US'); }
export function prettyCall(s) { return String(s || '').replace(/_go_/, ' → ').replace(/_2_1_2/, ' 2-1-2').replace(/_/g, ' ').toUpperCase(); }

// Agrupa els esdeveniments per rondes: round_start … round_end (+ half/match_end).
export function groupRounds(events) {
  const rounds = []; let cur = null;
  for (const e of events || []) {
    if (e.kind === 'round_start') { cur = { start: e, events: [e], end: null, after: [] }; rounds.push(cur); continue; }
    if (!cur) continue;
    if (e.kind === 'half' || e.kind === 'match_end' || cur.end) { cur.after.push(e); continue; }
    cur.events.push(e);
    if (e.kind === 'round_end') cur.end = e;
  }
  return rounds;
}

export function posAt(track, t) {
  if (!track || !track.length) return { x: 0, y: 0 };
  if (t <= track[0].t) return { x: track[0].x, y: track[0].y };
  for (let i = 1; i < track.length; i++) {
    const b = track[i];
    if (t <= b.t) { const a = track[i - 1]; const k = b.t > a.t ? (t - a.t) / (b.t - a.t) : 1; return { x: lerp(a.x, b.x, k), y: lerp(a.y, b.y, k) }; }
  }
  const l = track[track.length - 1]; return { x: l.x, y: l.y };
}

// Afegeix un moviment a una pista (talla el que hi hagués després de t0).
export function addMove(track, t0, pts, dur) {
  if (!track || !pts || !pts.length) return track;
  const p0 = posAt(track, t0);
  while (track.length && track[track.length - 1].t > t0) track.pop();
  track.push({ t: t0, x: p0.x, y: p0.y });
  const pp = [[p0.x, p0.y], ...pts.slice(1)];
  let len = 0; const seg = [];
  for (let i = 1; i < pp.length; i++) { const l = Math.hypot(pp[i][0] - pp[i - 1][0], pp[i][1] - pp[i - 1][1]); seg.push(l); len += l; }
  let t = t0; const D = Math.max(0.01, +dur || 0.01);
  for (let i = 1; i < pp.length; i++) { t += len > 0 ? D * seg[i - 1] / len : D / (pp.length - 1); track.push({ t, x: +pp[i][0], y: +pp[i][1] }); }
  return track;
}

// Línia de temps de reproducció: comprimeix els temps morts i s'atura un instant a cada mort.
export function buildTimeline(R, opt = {}) {
  const RATE = opt.rate || 5.5, CAP = opt.cap || 2.4;
  const beats = [];
  for (const k of R.kills) beats.push([k.t, 0.42]);
  if (R.plant) beats.push([R.plant.start, 0.1], [R.plant.t, 0.55]);
  if (R.defuse) beats.push([R.defuse.start, 0.15], [R.defuse.t, 0.6]);
  if (R.explode) beats.push([R.explode.t, 0.7]);
  for (const u of R.utils) beats.push([u.t, 0]);
  beats.push([R.endT, 0]);
  beats.sort((a, b) => a[0] - b[0]);
  const pts = [[0, 0]]; let p = 0, g = 0;
  for (const [t0, hold] of beats) {
    const t = Math.max(g, +t0 || 0);
    const gap = t - g;
    if (gap > 0) { p += Math.min(gap / RATE, CAP + gap / 40); g = t; pts.push([p, g]); }
    if (hold > 0) { p += hold; pts.push([p, g]); }
  }
  return { pts, total: p };
}

export function gameTimeAt(tl, p) {
  const P = tl.pts; if (p <= 0) return 0;
  for (let i = 1; i < P.length; i++) {
    if (p <= P[i][0]) { const a = P[i - 1], b = P[i]; if (b[0] === a[0] || b[1] === a[1]) return b[1] === a[1] ? a[1] : b[1]; return lerp(a[1], b[1], (p - a[0]) / (b[0] - a[0])); }
  }
  return P[P.length - 1][1];
}

export function playTimeAt(tl, g) {
  const P = tl.pts; if (g <= 0) return 0;
  for (let i = 1; i < P.length; i++) {
    const a = P[i - 1], b = P[i];
    if (g <= b[1] && b[1] > a[1]) return lerp(a[0], b[0], (g - a[1]) / (b[1] - a[1]));
    if (g === a[1]) return a[0];
  }
  return tl.total;
}

function handleMap(meta) {
  const m = {};
  ((meta && meta.teams) || []).forEach((t, ti) => (t.players || []).forEach(p => { m[p.id] = { handle: p.handle, role: p.role, team: ti }; }));
  return m;
}

// Reconstrueix una ronda: pistes de cada jugador, morts, utilitat, bomba.
export function buildRound(g, meta) {
  const d = dataOf(g.start);
  const hm = handleMap(meta);
  const players = {};
  for (const p of d.players || []) {
    players[p.id] = { id: p.id, team: p.team, side: p.side, handle: (hm[p.id] || {}).handle || p.id, role: (hm[p.id] || {}).role || '', weapon: p.weapon, money: p.money, track: [{ t: 0, x: +p.x || 0, y: +p.y || 0 }], deathT: Infinity, death: null };
  }
  const R = { num: d.round || 0, tTeam: d.tTeam || 0, ctTeam: d.ctTeam != null ? d.ctTeam : 1 - (d.tTeam || 0), pistol: !!d.pistol, players, kills: [], utils: [], buys: [], plant: null, defuse: null, explode: null, bomb: d.bomb || null, money: d.money || [0, 0], calls: d.calls || {}, half: d.half || '', events: g.events.slice(), after: g.after || [], end: g.end ? dataOf(g.end) : null, endT: 0 };
  for (const e of R.events) {
    const x = dataOf(e); const t = Math.max(0, +x.t || 0);
    e._t = t;
    R.endT = Math.max(R.endT, t);
    switch (e.kind) {
      case 'buy': R.buys[x.team] = x; for (const it of x.items || []) if (players[it.id]) { players[it.id].weapon = it.weapon; players[it.id].money = it.money; } break;
      case 'move': if (players[x.id]) addMove(players[x.id].track, t, x.pts, x.dur); break;
      case 'kill': {
        const k = { t, n: e.n, killer: x.killer, victim: x.victim, kteam: x.kteam, vteam: x.vteam, weapon: x.weapon, hs: !!x.hs, smoke: !!x.smoke, kx: +x.kx || 0, ky: +x.ky || 0, vx: +x.vx || 0, vy: +x.vy || 0, first: !!x.first, hl: !!e.hl };
        R.kills.push(k);
        const v = players[x.victim]; if (v) { v.deathT = Math.min(v.deathT, t); v.death = { x: k.vx, y: k.vy }; }
        break;
      }
      case 'utility': R.utils.push({ t, n: e.n, type: x.type, x: +x.x || 0, y: +x.y || 0, fx: +x.fx || 0, fy: +x.fy || 0, dur: +x.dur || 1, team: x.team, id: x.id }); break;
      case 'plant': R.plant = { t, start: x.start != null ? +x.start : t - 3.2, id: x.id, site: x.site, x: +x.x || 0, y: +x.y || 0, explodeAt: x.explodeAt != null ? +x.explodeAt : t + BOMB_TIME }; break;
      case 'defuse': R.defuse = { t, start: x.start != null ? +x.start : t - (+x.dur || 5), id: x.id, x: +x.x || 0, y: +x.y || 0, ninja: !!x.ninja, kit: !!x.kit }; break;
      case 'explode': R.explode = { t, x: +x.x || 0, y: +x.y || 0 }; break;
    }
  }
  // Els morts es queden on han caigut.
  for (const p of Object.values(players)) if (isFinite(p.deathT) && p.death) { while (p.track.length > 1 && p.track[p.track.length - 1].t > p.deathT) p.track.pop(); p.track.push({ t: p.deathT, x: p.death.x, y: p.death.y }); }
  R.tl = buildTimeline(R);
  for (const e of R.events) e._p = e.kind === 'round_start' || e.kind === 'buy' ? 0 : playTimeAt(R.tl, e._t);
  for (const k of R.kills) k._p = playTimeAt(R.tl, k.t);
  for (const u of R.utils) u._p = playTimeAt(R.tl, u.t);
  R.events.sort((a, b) => a._p - b._p || a.n - b.n);
  return R;
}

// Angle on mira el jugador a l'instant t.
export function angleAt(R, id, t) {
  const pl = R.players[id]; if (!pl) return 0;
  for (let i = R.kills.length - 1; i >= 0; i--) {
    const k = R.kills[i];
    if (k.t <= t && t - k.t < 1.2) {
      if (k.killer === id) return Math.atan2(k.vy - k.ky, k.vx - k.kx);
      if (k.victim === id) return Math.atan2(k.ky - k.vy, k.kx - k.vx);
    }
  }
  const tr = pl.track; let ang = pl.side === 'T' ? Math.PI : 0;
  for (let i = 1; i < tr.length; i++) {
    const a = tr[i - 1], b = tr[i];
    if (a.t > t) break;
    if (Math.hypot(b.x - a.x, b.y - a.y) > 0.05) ang = Math.atan2(b.y - a.y, b.x - a.x);
  }
  return ang;
}

// Estat d'una granada: núvol, foc, esclat o en vol.
export function utilState(u, g, p) {
  const dp = p - (u._p || 0);
  if (dp < 0) {
    if (dp > -0.45) { const k = 1 + dp / 0.45; return { kind: 'flight', x: lerp(u.fx, u.x, k), y: lerp(u.fy, u.y, k), h: Math.sin(Math.PI * k), r: 0, a: 1 }; }
    return null;
  }
  if (u.type === 'smoke') {
    const age = g - u.t; if (age > 18) return null;
    const grow = clamp(Math.max(age / 1.2, dp / 0.5), 0, 1), fade = clamp((18 - age) / 2.5, 0, 1);
    return { kind: 'smoke', x: u.x, y: u.y, r: 4.8 * (0.35 + 0.65 * easeOut(grow)), a: 0.9 * fade };
  }
  if (u.type === 'molotov') {
    const age = g - u.t; if (age > 7) return null;
    return { kind: 'molotov', x: u.x, y: u.y, r: 3.4 * clamp(Math.max(age / 0.6, dp / 0.3), 0.3, 1), a: clamp((7 - age) / 1.2, 0, 1) };
  }
  if (dp > 0.6) return null;
  const k = dp / 0.6;
  return { kind: u.type === 'he' ? 'he' : 'flash', x: u.x, y: u.y, r: 1.5 + 6 * easeOut(k), a: 1 - k };
}

// Estat de la bomba a l'instant de joc g.
export function bombState(R, g) {
  const s = { carrier: null, dropped: null, planting: null, planted: false, x: 0, y: 0, left: 0, defusing: null, exploded: false, defused: false };
  const pl = R.plant;
  if (!pl || g < pl.start) {
    const c = R.players[R.bomb];
    if (c) {
      if (g < c.deathT) s.carrier = c.id;
      else s.dropped = c.death ? { x: c.death.x, y: c.death.y } : posAt(c.track, c.deathT);
    }
    return s;
  }
  if (g < pl.t) { s.planting = { id: pl.id, prog: clamp((g - pl.start) / Math.max(0.1, pl.t - pl.start), 0, 1) }; return s; }
  s.planted = true; s.x = pl.x; s.y = pl.y; s.left = Math.max(0, pl.explodeAt - g);
  if (R.explode && g >= R.explode.t) { s.exploded = true; s.left = 0; }
  if (R.defuse) {
    if (g >= R.defuse.t) s.defused = true;
    else if (g >= R.defuse.start) s.defusing = { id: R.defuse.id, prog: clamp((g - R.defuse.start) / Math.max(0.1, R.defuse.t - R.defuse.start), 0, 1) };
  }
  return s;
}

// Feed de morts: les darreres (en temps de reproducció).
export function feedAt(R, p, keep = 6, max = 5) {
  return R.kills.filter(k => k._p <= p && p - k._p < keep).slice(-max);
}

export function aliveAt(R, g) {
  const n = [0, 0];
  for (const pl of Object.values(R.players)) if (g < pl.deathT) n[pl.team]++;
  return n;
}

export function fitMap(size, box) {
  const w = (size && size[0]) || 100, h = (size && size[1]) || 80;
  const s = Math.max(0.01, Math.min(box.w / w, box.h / h));
  return { s, ox: box.x + (box.w - w * s) / 2, oy: box.y + (box.h - h * s) / 2 };
}

// ---- el visor ----

export class Cs2View {
  constructor(canvas, opts = {}) {
    this.cv = canvas; this.ctx = canvas.getContext('2d');
    this.lang = opts.lang === 'ca' ? 'ca' : 'en';
    this.T = TEXT[this.lang];
    this.source = opts.source;
    this.onUpdate = opts.onUpdate || (() => {});
    this.onEvent = opts.onEvent || null;
    this.match = null; this.meta = null; this.events = []; this.rounds = []; this.lastN = 0;
    this.ri = 0; this.R = null; this.phase = 'wait'; this.p = 0; this.hold = 0; this.fired = 0;
    this.speed = 1; this.paused = false; this.stopped = false; this.score = [0, 0]; this.ticker = null; this.banner = null; this.overlay = null;
    this.last = performance.now(); this.mapCache = null;
    this.resize = this.resize.bind(this); window.addEventListener('resize', this.resize); this.resize();
    this.loop = this.loop.bind(this); requestAnimationFrame(this.loop);
    this.poll();
  }

  destroy() { this.stopped = true; window.removeEventListener('resize', this.resize); clearTimeout(this.pollTimer); }
  setSpeed(x) { this.speed = +x || 1; }
  togglePause() { this.paused = !this.paused; return this.paused; }
  terminal() { return !!this.match && ['final', 'stopped', 'failed'].includes(this.match.status); }
  isFinished() { return this.terminal() && this.phase === 'final'; }

  skipToEnd() {
    const done = this.rounds.filter(r => r.end);
    if (!done.length) { if (this.terminal()) this.phase = 'final'; return; }
    this.ri = done.length - 1;
    this.R = buildRound(done[this.ri], this.meta || {});
    this.p = this.R.tl.total; this.fired = this.R.events.length;
    const last = [...this.R.events, ...this.R.after].pop();
    if (last) this.score = last.score || this.score;
    this.banner = null; this.overlay = null; this.ticker = null;
    if (this.R.after.some(e => e.kind === 'match_end') || this.terminal()) { this.ri = this.rounds.length; this.enterFinal(); }
    else { this.ri++; this.phase = 'wait'; }
  }

  async poll() {
    if (this.stopped) return;
    try {
      const m = await this.source.load(this.lastN);
      if (m) {
        if (m.meta && !this.meta) { this.meta = m.meta; this.mapCache = null; }
        const fresh = (m.events || []).filter(e => e.n > this.lastN);
        for (const e of fresh) { this.events.push(e); this.lastN = Math.max(this.lastN, e.n); }
        if (fresh.length) this.rounds = groupRounds(this.events);
        this.match = { ...(this.match || {}), ...m, events: this.events, meta: this.meta };
        this.onUpdate(this.match, this.current());
      }
    } catch (e) { /* torna-ho a provar */ }
    if (!this.terminal()) this.pollTimer = setTimeout(() => this.poll(), 3000);
  }

  current() { return this.R && this.R.events[Math.max(0, this.fired - 1)] || null; }

  names() {
    const m = this.match || {}, mt = this.meta || {};
    return [0, 1].map(i => (m.names && m.names[i]) || (mt.teams && mt.teams[i] && mt.teams[i].name) || ['A', 'B'][i]);
  }
  teamColor(i) { const t = this.meta && this.meta.teams && this.meta.teams[i]; return (t && t.color) || ['#2ec4b6', '#ff6b6b'][i]; }

  resize() {
    const r = this.cv.getBoundingClientRect(), d = window.devicePixelRatio || 1;
    this.cv.width = Math.max(320, Math.round(r.width * d)); this.cv.height = Math.max(200, Math.round(r.height * d));
    this.W = this.cv.width; this.H = this.cv.height; this.dpr = d;
    const cssW = this.W / d, cssH = this.H / d;
    this.narrow = cssW < 640;
    this.u = d * clamp(Math.min(cssW / (this.narrow ? 500 : 960), cssH / 600), 0.66, 1.35);
    const u = this.u;
    this.topH = (this.narrow ? 46 : 58) * u;
    const pad = 8 * u;
    // Pantalla ampla: plantilles als costats (com a la tele); estreta: a sota o cap.
    const portrait = cssH > cssW * 1.25;
    this.layout = cssW / cssH >= 1.45 && cssW >= 860 ? 'side' : (!this.narrow && cssH > 430) || portrait ? 'bottom' : 'none';
    this.rosterW = this.layout === 'side' ? Math.min(230 * u, this.W * 0.2) : 0;
    this.rosterH = this.layout === 'bottom' ? (this.narrow ? 5 * 22 * u + 16 * u : 86 * u) : 0;
    const side = this.rosterW ? this.rosterW + pad : 0;
    const box = { x: pad + side, y: this.topH + pad, w: this.W - 2 * pad - 2 * side, h: this.H - this.topH - this.rosterH - 2 * pad };
    this.box = box;
    this.mapCache = null;
  }

  fit() { if (!this.fitR || this.fitR.box !== this.box || this.fitR.meta !== this.meta) this.fitR = { box: this.box, meta: this.meta, f: fitMap(this.meta && this.meta.size, this.box) }; return this.fitR.f; }
  sx(x) { const f = this.fit(); return f.ox + x * f.s; }
  sy(y) { const f = this.fit(); return f.oy + y * f.s; }

  loop(now) {
    if (this.stopped) return;
    const dt = Math.min(0.25, (now - this.last) / 1000); this.last = now;
    if (!this.paused) this.advance(dt * this.speed);
    try { this.draw(); } catch (e) { /* un fotograma dolent no ha d'aturar el visor */ }
    requestAnimationFrame(this.loop);
  }

  fire(e) {
    if (e.score) this.score = e.score;
    if (['kill', 'plant', 'defuse', 'explode', 'round_end', 'buy', 'half', 'match_end'].includes(e.kind) && e.text) this.ticker = { text: e.text, t0: performance.now(), hl: !!e.hl };
    if (this.onEvent) { try { this.onEvent(e); } catch { /* res */ } }
  }

  enterFinal() { this.phase = 'final'; this.overlay = { kind: 'final', t0: performance.now() }; }

  advance(dt) {
    if (!this.meta) return;
    if (this.phase === 'final') return;
    if (this.phase === 'wait') {
      const g = this.rounds[this.ri];
      if (g && g.end) {
        this.R = buildRound(g, this.meta); this.p = 0; this.fired = 0; this.phase = 'intro'; this.hold = 1.7; this.banner = null; this.overlay = null;
        while (this.fired < this.R.events.length && this.R.events[this.fired]._p <= 0 && ['round_start', 'buy'].includes(this.R.events[this.fired].kind)) this.fire(this.R.events[this.fired++]);
        this.onUpdate(this.match, this.current());
      } else if (this.terminal() && this.ri >= this.rounds.filter(r => r.end).length) {
        this.enterFinal();
      }
      return;
    }
    if (this.phase === 'intro') { this.hold -= dt; if (this.hold <= 0) this.phase = 'play'; return; }
    if (this.phase === 'play') {
      this.p += dt;
      const R = this.R;
      while (this.fired < R.events.length && R.events[this.fired]._p <= this.p && R.events[this.fired].kind !== 'round_end') this.fire(R.events[this.fired++]);
      if (this.p >= R.tl.total) {
        this.p = R.tl.total;
        while (this.fired < R.events.length) this.fire(R.events[this.fired++]);
        this.phase = 'outro'; this.hold = 3 + (R.events.some(e => e.kind === 'round_end' && e.hl) ? 0.8 : 0);
        this.banner = { t0: performance.now() };
      }
      return;
    }
    if (this.phase === 'outro') {
      this.hold -= dt;
      if (this.hold > 0) return;
      this.banner = null;
      // Els half/match_end poden arribar després.
      const g = this.rounds[this.ri];
      const after = (g && g.after) || [];
      this.R.after = after;
      const me = after.find(e => e.kind === 'match_end');
      if (me) { this.fire(me); this.ri++; this.enterFinal(); return; }
      const hf = after.find(e => e.kind === 'half');
      if (hf) { this.fire(hf); this.phase = 'half'; this.hold = 3.8; this.overlay = { kind: 'half', e: hf, t0: performance.now() }; return; }
      this.ri++; this.phase = 'wait';
      return;
    }
    if (this.phase === 'half') { this.hold -= dt; if (this.hold <= 0) { this.overlay = null; this.ri++; this.phase = 'wait'; } }
  }

  // ---- dibuix ----

  draw() {
    const g = this.ctx, W = this.W, H = this.H;
    const bg = g.createRadialGradient(W / 2, H / 2, 0, W / 2, H / 2, Math.max(W, H) * 0.75);
    bg.addColorStop(0, '#131a26'); bg.addColorStop(1, '#070a10');
    g.fillStyle = bg; g.fillRect(0, 0, W, H);
    if (!this.meta) { this.centerText(this.match && this.match.status === 'failed' ? this.T.failed : this.T.pregame); return; }
    this.drawMap();
    const R = this.R;
    if (R) {
      const gt = gameTimeAt(R.tl, this.p);
      this.drawUtil(R, gt, ['molotov']);
      this.drawBombGround(R, gt);
      this.drawPlayers(R, gt);
      this.drawTracers(R);
      this.drawUtil(R, gt, ['smoke', 'flash', 'he', 'flight']);
      this.drawFeed(R);
      this.drawRosters(R, gt);
      this.drawTop(R, gt);
      this.drawTicker();
      if (this.phase === 'intro') this.drawIntro(R);
      if (this.banner) this.drawBanner(R);
    } else {
      this.drawTop(null, 0);
      this.drawRosters(null, 0);
    }
    if (this.overlay) this.drawOverlay();
    if (!this.overlay && this.phase === 'wait' && !(this.terminal())) this.waitingNote();
  }

  centerText(text) {
    const g = this.ctx; g.fillStyle = 'rgba(255,255,255,.85)'; g.font = `600 ${15 * this.u}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText(text, this.W / 2, this.H / 2);
  }

  waitingNote() {
    const g = this.ctx, u = this.u, text = this.T.waiting;
    g.font = `600 ${12 * u}px ${FONT}`; const tw = g.measureText(text).width + 24 * u;
    const y = this.box.y + this.box.h - 30 * u;
    g.fillStyle = 'rgba(6,9,14,.78)'; g.beginPath(); g.roundRect((this.W - tw) / 2, y, tw, 24 * u, 12 * u); g.fill();
    g.fillStyle = 'rgba(255,255,255,.85)'; g.textAlign = 'center'; g.textBaseline = 'middle';
    const dots = '.'.repeat(1 + Math.floor(performance.now() / 400) % 3);
    g.fillText(text.replace(/…$/, '') + dots, this.W / 2, y + 12 * u);
  }

  // Mapa estàtic en una tela a part (es refà en canviar la mida).
  drawMap() {
    if (!this.mapCache) {
      const c = document.createElement('canvas'); c.width = this.W; c.height = this.H;
      this.renderMap(c.getContext('2d'));
      this.mapCache = c;
    }
    this.ctx.drawImage(this.mapCache, 0, 0);
  }

  renderMap(g) {
    const mt = this.meta, f = this.fit(), s = f.s;
    const X = x => f.ox + x * s, Y = y => f.oy + y * s;
    const zones = {}; (mt.zones || []).forEach(z => { zones[z.id] = z; });
    // Quadrícula tènue.
    g.save(); g.strokeStyle = 'rgba(120,150,190,.05)'; g.lineWidth = 1;
    const size = mt.size || [100, 80];
    for (let x = 0; x <= size[0]; x += 5) { g.beginPath(); g.moveTo(X(x), Y(0)); g.lineTo(X(x), Y(size[1])); g.stroke(); }
    for (let y = 0; y <= size[1]; y += 5) { g.beginPath(); g.moveTo(X(0), Y(y)); g.lineTo(X(size[0]), Y(y)); g.stroke(); }
    g.restore();
    // Passadissos: primer l'ombra (parets), després el terra.
    g.lineCap = 'round'; g.lineJoin = 'round';
    for (const [w, col] of [[6.4, '#05070b'], [5.2, '#3a4454'], [4.6, '#262e3b']]) {
      g.strokeStyle = col; g.lineWidth = w * s;
      for (const [a, b] of mt.links || []) { const za = zones[a], zb = zones[b]; if (!za || !zb) continue; g.beginPath(); g.moveTo(X(za.x), Y(za.y)); g.lineTo(X(zb.x), Y(zb.y)); g.stroke(); }
    }
    // Zones.
    for (const z of mt.zones || []) {
      const x = X(z.x - z.w / 2), y = Y(z.y - z.h / 2), w = z.w * s, h = z.h * s, rr = Math.min(1.6 * s, w / 3);
      g.fillStyle = '#05070b'; g.beginPath(); g.roundRect(x - 0.7 * s, y - 0.7 * s, w + 1.4 * s, h + 1.4 * s, rr + 0.7 * s); g.fill();
      const fl = g.createLinearGradient(x, y, x, y + h);
      if (z.kind === 'site') { fl.addColorStop(0, '#3a3f52'); fl.addColorStop(1, '#2d3243'); }
      else if (z.kind === 'spawn_t') { fl.addColorStop(0, '#3b3427'); fl.addColorStop(1, '#2c2820'); }
      else if (z.kind === 'spawn_ct') { fl.addColorStop(0, '#26364a'); fl.addColorStop(1, '#1f2b3b'); }
      else { fl.addColorStop(0, '#2f3846'); fl.addColorStop(1, '#272f3c'); }
      g.fillStyle = fl; g.beginPath(); g.roundRect(x, y, w, h, rr); g.fill();
      g.strokeStyle = z.kind === 'site' ? 'rgba(255,214,120,.35)' : 'rgba(150,170,200,.18)'; g.lineWidth = Math.max(1, 0.25 * s);
      if (z.kind === 'site') g.setLineDash([1.2 * s, 0.8 * s]);
      g.stroke(); g.setLineDash([]);
      if (z.kind === 'site') {
        g.fillStyle = 'rgba(255,214,120,.16)'; g.font = `900 ${Math.min(h, w) * 0.75}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle';
        g.fillText(z.site || '', X(z.x), Y(z.y));
      }
      const fs = Math.min(1.9 * s, 11 * this.dpr);
      if (fs > 6 * this.dpr) {
        g.fillStyle = 'rgba(190,205,225,.42)'; g.font = `700 ${fs}px ${FONT}`; g.textAlign = 'left'; g.textBaseline = 'top';
        g.fillText(z.name.toUpperCase(), x + 0.8 * s, y + 0.6 * s, w - 1.6 * s);
      }
    }
    // Caixes.
    for (const p of mt.props || []) {
      const x = X(p.x - p.w / 2), y = Y(p.y - p.h / 2);
      g.fillStyle = 'rgba(0,0,0,.45)'; g.fillRect(x + 0.4 * s, y + 0.5 * s, p.w * s, p.h * s);
      g.fillStyle = '#4a5466'; g.fillRect(x, y, p.w * s, p.h * s);
      g.strokeStyle = '#6b778c'; g.lineWidth = Math.max(1, 0.18 * s); g.strokeRect(x, y, p.w * s, p.h * s);
    }
    // Llocs de plantar.
    for (const [site, pt] of Object.entries(mt.plant || {})) {
      g.strokeStyle = 'rgba(255,90,70,.35)'; g.lineWidth = Math.max(1, 0.25 * s); g.setLineDash([0.8 * s, 0.6 * s]);
      g.beginPath(); g.arc(X(pt[0]), Y(pt[1]), 2.6 * s, 0, Math.PI * 2); g.stroke(); g.setLineDash([]);
      void site;
    }
  }

  drawUtil(R, gt, kinds) {
    const g = this.ctx, s = this.fit().s;
    for (const u of R.utils) {
      const st = utilState(u, gt, this.p);
      if (!st || !kinds.includes(st.kind)) continue;
      const x = this.sx(st.x), y = this.sy(st.y), r = st.r * s;
      if (st.kind === 'flight') {
        const col = { smoke: '#d6dbe3', flash: '#ffffff', molotov: '#ff8a3d', he: '#9be36b' }[u.type] || '#fff';
        g.fillStyle = col; g.beginPath(); g.arc(x, y - st.h * 2.2 * s, Math.max(2 * this.dpr, 0.45 * s), 0, Math.PI * 2); g.fill();
        g.fillStyle = 'rgba(0,0,0,.35)'; g.beginPath(); g.arc(x, y, Math.max(1.5 * this.dpr, 0.35 * s), 0, Math.PI * 2); g.fill();
        continue;
      }
      if (st.kind === 'smoke') {
        const R2 = rng(u.n * 7 + 3);
        for (let i = 0; i < 7; i++) {
          const a = R2() * Math.PI * 2, dd = i === 0 ? 0 : r * (0.25 + 0.35 * R2()), rr = r * (0.55 + 0.3 * R2());
          const wob = Math.sin(performance.now() / 1400 + i) * 0.04 * r;
          const cx = x + Math.cos(a) * dd + wob, cy = y + Math.sin(a) * dd;
          const gr = g.createRadialGradient(cx, cy, 0, cx, cy, rr);
          gr.addColorStop(0, `rgba(205,210,220,${0.5 * st.a})`); gr.addColorStop(0.6, `rgba(170,178,190,${0.35 * st.a})`); gr.addColorStop(1, 'rgba(160,168,180,0)');
          g.fillStyle = gr; g.beginPath(); g.arc(cx, cy, rr, 0, Math.PI * 2); g.fill();
        }
      } else if (st.kind === 'molotov') {
        const gr = g.createRadialGradient(x, y, 0, x, y, r);
        gr.addColorStop(0, `rgba(255,190,80,${0.55 * st.a})`); gr.addColorStop(0.6, `rgba(255,90,30,${0.38 * st.a})`); gr.addColorStop(1, 'rgba(200,40,10,0)');
        g.fillStyle = gr; g.beginPath(); g.arc(x, y, r, 0, Math.PI * 2); g.fill();
        const R2 = rng(u.n * 13 + Math.floor(performance.now() / 90));
        for (let i = 0; i < 9; i++) { const a = R2() * Math.PI * 2, dd = R2() * r * 0.8; g.fillStyle = `rgba(255,${150 + R2() * 90 | 0},60,${0.7 * st.a})`; g.beginPath(); g.arc(x + Math.cos(a) * dd, y + Math.sin(a) * dd, (0.25 + R2() * 0.35) * s, 0, Math.PI * 2); g.fill(); }
      } else {
        const col = st.kind === 'he' ? '255,170,80' : '255,255,255';
        const gr = g.createRadialGradient(x, y, 0, x, y, r);
        gr.addColorStop(0, `rgba(${col},${0.95 * st.a})`); gr.addColorStop(1, `rgba(${col},0)`);
        g.fillStyle = gr; g.beginPath(); g.arc(x, y, r, 0, Math.PI * 2); g.fill();
      }
    }
  }

  drawBombGround(R, gt) {
    const g = this.ctx, s = this.fit().s, d = this.dpr;
    const b = bombState(R, gt);
    if (b.dropped) { this.bombIcon(this.sx(b.dropped.x), this.sy(b.dropped.y), Math.max(4 * d, 0.9 * s), 0.9); }
    if (!b.planted) return;
    const x = this.sx(b.x), y = this.sy(b.y);
    if (b.exploded) {
      const age = (performance.now() - (this.explodeT || (this.explodeT = performance.now()))) / 1000;
      const k = clamp(age / 1.4, 0, 1), r = (3 + 16 * easeOut(k)) * s;
      const gr = g.createRadialGradient(x, y, 0, x, y, r);
      gr.addColorStop(0, `rgba(255,240,180,${0.9 * (1 - k)})`); gr.addColorStop(0.4, `rgba(255,120,30,${0.7 * (1 - k)})`); gr.addColorStop(1, 'rgba(120,20,0,0)');
      g.fillStyle = gr; g.beginPath(); g.arc(x, y, r, 0, Math.PI * 2); g.fill();
      g.fillStyle = 'rgba(20,10,5,.55)'; g.beginPath(); g.arc(x, y, 4 * s, 0, Math.PI * 2); g.fill();
      return;
    }
    this.explodeT = 0;
    const frac = b.left / BOMB_TIME;
    const rate = b.left < 10 ? 4 : b.left < 20 ? 2 : 1;
    const blink = 0.5 + 0.5 * Math.sin(performance.now() / 1000 * Math.PI * 2 * rate);
    const col = b.defused ? '#3ddc84' : '#ff4b3e';
    g.strokeStyle = 'rgba(0,0,0,.6)'; g.lineWidth = 0.9 * s; g.beginPath(); g.arc(x, y, 3 * s, 0, Math.PI * 2); g.stroke();
    g.strokeStyle = col; g.lineWidth = 0.6 * s; g.beginPath(); g.arc(x, y, 3 * s, -Math.PI / 2, -Math.PI / 2 + Math.PI * 2 * (b.defused ? 1 : frac)); g.stroke();
    const glow = g.createRadialGradient(x, y, 0, x, y, 2.4 * s);
    glow.addColorStop(0, rgba(col, b.defused ? 0.5 : 0.25 + 0.5 * blink)); glow.addColorStop(1, rgba(col, 0));
    g.fillStyle = glow; g.beginPath(); g.arc(x, y, 2.4 * s, 0, Math.PI * 2); g.fill();
    this.bombIcon(x, y, Math.max(4 * d, 0.9 * s), 1);
  }

  bombIcon(x, y, r, a) {
    const g = this.ctx;
    g.save(); g.globalAlpha = a;
    g.fillStyle = '#c0392b'; g.strokeStyle = '#1b0c0a'; g.lineWidth = Math.max(1, r * 0.2);
    g.beginPath(); g.roundRect(x - r, y - r * 0.7, 2 * r, 1.4 * r, r * 0.25); g.fill(); g.stroke();
    g.fillStyle = '#ffd34d'; g.fillRect(x - r * 0.55, y - r * 0.25, r * 1.1, r * 0.5);
    g.restore();
  }

  drawPlayers(R, gt) {
    const g = this.ctx, s = this.fit().s, d = this.dpr;
    const rad = Math.max(4.5 * d, 1.45 * s);
    const bomb = bombState(R, gt);
    const list = Object.values(R.players);
    // Morts primer (a sota).
    for (const pl of list) {
      if (gt < pl.deathT) continue;
      const p = pl.death || posAt(pl.track, pl.deathT), x = this.sx(p.x), y = this.sy(p.y), c = this.teamColor(pl.team);
      const age = gt - pl.deathT;
      g.save(); g.globalAlpha = age < 0.4 ? 1 : 0.55; g.strokeStyle = c; g.lineWidth = Math.max(1.5 * d, 0.35 * s);
      const k = rad * 0.7;
      g.beginPath(); g.moveTo(x - k, y - k); g.lineTo(x + k, y + k); g.moveTo(x + k, y - k); g.lineTo(x - k, y + k); g.stroke();
      g.restore();
    }
    for (const pl of list) {
      if (gt >= pl.deathT) continue;
      const p = posAt(pl.track, gt), x = this.sx(p.x), y = this.sy(p.y);
      const c = this.teamColor(pl.team), sc = SIDE_COL[pl.side] || '#ccc';
      const ang = angleAt(R, pl.id, gt);
      const awp = pl.weapon === 'awp';
      const cr = (awp ? 13 : 8.5) * s, half = awp ? 0.16 : 0.42;
      const cone = g.createRadialGradient(x, y, rad, x, y, cr);
      cone.addColorStop(0, rgba(sc, 0.26)); cone.addColorStop(1, rgba(sc, 0));
      g.fillStyle = cone; g.beginPath(); g.moveTo(x, y); g.arc(x, y, cr, ang - half, ang + half); g.closePath(); g.fill();
      // cos
      g.fillStyle = 'rgba(0,0,0,.45)'; g.beginPath(); g.arc(x + 0.15 * s, y + 0.25 * s, rad, 0, Math.PI * 2); g.fill();
      const body = g.createRadialGradient(x - rad * 0.35, y - rad * 0.35, rad * 0.1, x, y, rad);
      body.addColorStop(0, rgba(c, 1)); body.addColorStop(1, rgba(c, 0.78));
      g.fillStyle = body; g.beginPath(); g.arc(x, y, rad, 0, Math.PI * 2); g.fill();
      g.strokeStyle = sc; g.lineWidth = Math.max(1.5 * d, 0.32 * s); g.stroke();
      // punta de mira
      g.strokeStyle = 'rgba(255,255,255,.85)'; g.lineWidth = Math.max(1.2 * d, 0.22 * s);
      g.beginPath(); g.moveTo(x + Math.cos(ang) * rad * 0.35, y + Math.sin(ang) * rad * 0.35); g.lineTo(x + Math.cos(ang) * rad * 1.35, y + Math.sin(ang) * rad * 1.35); g.stroke();
      // trets
      for (const k of R.kills) {
        if (k.killer === pl.id && this.p - k._p >= 0 && this.p - k._p < 0.25) {
          g.fillStyle = 'rgba(255,230,140,.9)'; g.beginPath(); g.arc(x + Math.cos(ang) * rad * 1.5, y + Math.sin(ang) * rad * 1.5, rad * 0.55, 0, Math.PI * 2); g.fill();
        }
      }
      if (bomb.carrier === pl.id) this.bombIcon(x + rad * 1.1, y + rad * 0.9, rad * 0.5, 1);
      if (bomb.planting && bomb.planting.id === pl.id) this.progressRing(x, y, rad * 1.7, bomb.planting.prog, '#ff4b3e');
      if (bomb.defusing && bomb.defusing.id === pl.id) this.progressRing(x, y, rad * 1.7, bomb.defusing.prog, '#4fc3ff');
      // nom
      const fs = Math.max(9 * d, Math.min(1.7 * s, 13 * d));
      g.font = `700 ${fs}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'bottom';
      g.lineWidth = 3 * d; g.strokeStyle = 'rgba(0,0,0,.8)'; g.strokeText(pl.handle, x, y - rad - 2 * d);
      g.fillStyle = '#fff'; g.fillText(pl.handle, x, y - rad - 2 * d);
    }
  }

  progressRing(x, y, r, k, col) {
    const g = this.ctx, d = this.dpr;
    g.strokeStyle = 'rgba(0,0,0,.55)'; g.lineWidth = 3.5 * d; g.beginPath(); g.arc(x, y, r, 0, Math.PI * 2); g.stroke();
    g.strokeStyle = col; g.lineWidth = 2.5 * d; g.beginPath(); g.arc(x, y, r, -Math.PI / 2, -Math.PI / 2 + Math.PI * 2 * k); g.stroke();
  }

  drawTracers(R) {
    const g = this.ctx, s = this.fit().s, d = this.dpr;
    for (const k of R.kills) {
      const age = this.p - k._p;
      if (age < 0 || age > 0.7) continue;
      const a = 1 - age / 0.7;
      const x0 = this.sx(k.kx), y0 = this.sy(k.ky), x1 = this.sx(k.vx), y1 = this.sy(k.vy);
      const gr = g.createLinearGradient(x0, y0, x1, y1);
      gr.addColorStop(0, `rgba(255,236,160,${0.15 * a})`); gr.addColorStop(1, `rgba(255,236,160,${0.95 * a})`);
      g.strokeStyle = gr; g.lineWidth = Math.max(1.5 * d, (k.weapon === 'awp' ? 0.45 : 0.28) * s);
      g.beginPath(); g.moveTo(x0, y0); g.lineTo(x1, y1); g.stroke();
      const hr = (1 + 2.5 * (1 - a)) * s;
      g.strokeStyle = k.hs ? `rgba(255,70,60,${a})` : `rgba(255,255,255,${a})`; g.lineWidth = Math.max(1.5 * d, 0.3 * s);
      g.beginPath(); g.arc(x1, y1, hr, 0, Math.PI * 2); g.stroke();
    }
  }

  drawFeed(R) {
    const g = this.ctx, u = this.u;
    const rows = feedAt(R, this.p, 6, this.narrow ? 3 : 5);
    const rh = 22 * u, fs = 11.5 * u;
    const right = this.W - 10 * u;
    let y = this.topH + 8 * u;
    for (const k of rows) {
      const kp = R.players[k.killer] || { handle: k.killer }, vp = R.players[k.victim] || { handle: k.victim };
      const wname = WEAPON[k.weapon] || k.weapon;
      g.font = `700 ${fs}px ${FONT}`;
      const w1 = g.measureText(kp.handle).width, w3 = g.measureText(vp.handle).width;
      g.font = `600 ${fs * 0.85}px ${FONT}`;
      const w2 = g.measureText(wname).width;
      const extra = (k.hs ? 18 * u : 0) + (k.smoke ? 18 * u : 0);
      const w = w1 + w2 + w3 + extra + 34 * u;
      const x = right - w;
      const age = this.p - k._p;
      g.save(); g.globalAlpha = clamp((6 - age) / 0.6, 0, 1);
      g.fillStyle = k.hl ? 'rgba(120,20,20,.75)' : 'rgba(6,9,14,.72)'; g.beginPath(); g.roundRect(x, y, w, rh, 4 * u); g.fill();
      if (age < 0.5) { g.strokeStyle = `rgba(255,255,255,${0.6 * (1 - age / 0.5)})`; g.lineWidth = 1.5 * this.dpr; g.stroke(); }
      let cx = x + 8 * u; const cy = y + rh / 2;
      g.textBaseline = 'middle'; g.textAlign = 'left';
      g.font = `700 ${fs}px ${FONT}`; g.fillStyle = this.teamColor(k.kteam); g.fillText(kp.handle, cx, cy); cx += w1 + 8 * u;
      g.font = `600 ${fs * 0.85}px ${FONT}`; g.fillStyle = 'rgba(255,255,255,.85)'; g.fillText(wname, cx, cy); cx += w2 + 6 * u;
      if (k.smoke) { g.fillStyle = '#c9cfd8'; g.beginPath(); g.arc(cx + 5 * u, cy, 4.5 * u, 0, Math.PI * 2); g.arc(cx + 10 * u, cy - 1 * u, 4 * u, 0, Math.PI * 2); g.fill(); cx += 18 * u; }
      if (k.hs) { g.strokeStyle = '#ff5a4e'; g.lineWidth = 1.6 * this.dpr; g.beginPath(); g.arc(cx + 6 * u, cy, 4.5 * u, 0, Math.PI * 2); g.moveTo(cx + 6 * u, cy - 7 * u); g.lineTo(cx + 6 * u, cy + 7 * u); g.moveTo(cx - 1 * u, cy); g.lineTo(cx + 13 * u, cy); g.stroke(); cx += 18 * u; }
      g.font = `700 ${fs}px ${FONT}`; g.fillStyle = this.teamColor(k.vteam); g.fillText(vp.handle, cx + 2 * u, cy);
      g.restore();
      y += rh + 3 * u;
    }
  }

  sideOf(team, R) { if (!R) return ''; return team === R.tTeam ? 'T' : 'CT'; }

  drawTop(R, gt) {
    const g = this.ctx, u = this.u, W = this.W;
    const names = this.names();
    const bw = Math.min(W - 16 * u, (this.narrow ? 560 : 760) * u), bh = this.topH - 12 * u, bx = (W - bw) / 2, by = 6 * u;
    g.fillStyle = 'rgba(6,9,14,.88)'; g.beginPath(); g.roundRect(bx, by, bw, bh, 8 * u); g.fill();
    const mid = 118 * u * (this.narrow ? 0.8 : 1);
    const cell = (bw - mid) / 2;
    const tags = (this.meta && this.meta.teams || []).map(t => t.tag);
    const score = this.score || [0, 0];
    const cash = R ? (this.phase === 'outro' || this.phase === 'half' || this.phase === 'final') && R.end ? R.end.money : R.money : null;
    [0, 1].forEach(i => {
      const left = i === 0;
      const x = left ? bx : bx + cell + mid;
      const col = this.teamColor(i);
      // franja de color
      g.fillStyle = col; g.beginPath(); g.roundRect(left ? x : x + cell - 6 * u, by, 6 * u, bh, left ? [8 * u, 0, 0, 8 * u] : [0, 8 * u, 8 * u, 0]); g.fill();
      const grad = g.createLinearGradient(x, 0, x + cell, 0);
      grad.addColorStop(left ? 0 : 1, rgba(col, 0.32)); grad.addColorStop(left ? 1 : 0, rgba(col, 0));
      g.fillStyle = grad; g.fillRect(left ? x + 6 * u : x, by, cell - 6 * u, bh);
      // nom
      const side = R ? this.sideOf(i, R) : '';
      const name = this.narrow && names[i].length > 12 ? (tags[i] || names[i].slice(0, 10)) : names[i];
      g.fillStyle = '#fff'; g.font = `800 ${(this.narrow ? 13 : 16) * u}px ${FONT}`; g.textBaseline = 'middle';
      const nx = left ? x + 16 * u : x + cell - 16 * u;
      g.textAlign = left ? 'left' : 'right';
      const badgeW = side ? 26 * u : 0;
      g.fillText(name, left ? nx + badgeW : nx - badgeW, by + bh * (cash ? 0.38 : 0.5), cell - 90 * u - badgeW);
      if (side) {
        const bxx = left ? nx : nx - 22 * u;
        g.fillStyle = SIDE_COL[side]; g.beginPath(); g.roundRect(bxx, by + bh * 0.38 - 8 * u, 22 * u, 16 * u, 4 * u); g.fill();
        g.fillStyle = '#10131a'; g.font = `900 ${9.5 * u}px ${FONT}`; g.textAlign = 'center'; g.fillText(side, bxx + 11 * u, by + bh * 0.38 + 0.5 * u);
      }
      if (cash) {
        g.fillStyle = 'rgba(170,255,170,.85)'; g.font = `600 ${10.5 * u}px ${FONT}`; g.textAlign = left ? 'left' : 'right';
        g.fillText(money(cash[i]), nx, by + bh * 0.76);
      }
      // marcador
      g.fillStyle = '#fff'; g.font = `900 ${(this.narrow ? 22 : 28) * u}px ${FONT}`; g.textAlign = 'center';
      g.fillText(String(score[i] || 0), left ? x + cell - 30 * u : x + 30 * u, by + bh / 2 + 1 * u);
      if (R && R.end && this.phase !== 'play' && this.phase !== 'intro' && R.end.winner === i) {
        g.fillStyle = col; g.fillRect(left ? x + cell - 48 * u : x + 12 * u, by + bh - 4 * u, 36 * u, 3 * u);
      }
    });
    // centre: rellotge i ronda
    const cx = bx + cell + mid / 2;
    g.fillStyle = 'rgba(255,255,255,.06)'; g.fillRect(bx + cell, by, mid, bh);
    let clock = '', red = false;
    if (R) {
      const b = bombState(R, gt);
      if (b.planted && !b.defused && !b.exploded) { clock = fmtClock(b.left); red = true; }
      else if (this.phase === 'intro') clock = fmtClock(ROUND_TIME);
      else if (b.planted) clock = '0:00';
      else clock = fmtClock(ROUND_TIME - gt);
    }
    const final = this.phase === 'final';
    g.textAlign = 'center'; g.textBaseline = 'middle';
    if (final) { g.fillStyle = '#ffd34d'; g.font = `900 ${17 * u}px ${FONT}`; g.fillText(this.T.final, cx, by + bh * 0.5); }
    else {
      if (red) { this.bombIcon(cx - 34 * u, by + bh * 0.38, 6 * u, 0.6 + 0.4 * Math.sin(performance.now() / 160)); }
      g.fillStyle = red ? '#ff5a4e' : '#fff'; g.font = `800 ${(this.narrow ? 16 : 19) * u}px ${FONT}`;
      g.fillText(clock || '--:--', cx, by + bh * 0.4);
      g.fillStyle = 'rgba(255,255,255,.6)'; g.font = `700 ${9.5 * u}px ${FONT}`;
      const rn = R ? R.num : (this.rounds.length || 0);
      g.fillText(R ? `${this.T.round} ${rn}${rn > 24 ? ' · OT' : ''}` : this.T.live, cx, by + bh * 0.78);
    }
  }

  drawRosters(R, gt) {
    if (this.layout === 'none') return;
    const g = this.ctx, u = this.u, W = this.W, H = this.H, pad = 8 * u;
    const teams = (this.meta && this.meta.teams) || [];
    const names = this.names();
    const sideL = this.layout === 'side';
    [0, 1].forEach(i => {
      let x0, y0, w, h, rowH, head;
      if (sideL) {
        w = this.rosterW; head = 30 * u; rowH = 38 * u; h = head + 5 * rowH + 8 * u;
        x0 = i === 0 ? pad : W - pad - w; y0 = H - pad - h;
      } else {
        w = (W - 3 * pad) / 2; head = 0; h = this.rosterH - pad; rowH = (h - 8 * u) / 5;
        x0 = pad + i * (w + pad); y0 = H - this.rosterH;
      }
      const col = this.teamColor(i);
      g.fillStyle = 'rgba(6,9,14,.84)'; g.beginPath(); g.roundRect(x0, y0, w, h, 6 * u); g.fill();
      g.fillStyle = col; g.fillRect(i === 1 && sideL ? x0 + w - 4 * u : x0, y0, 4 * u, h);
      if (head) {
        const side = R ? this.sideOf(i, R) : '';
        g.textBaseline = 'middle'; g.textAlign = 'left';
        g.fillStyle = col; g.font = `800 ${12.5 * u}px ${FONT}`; g.fillText(names[i], x0 + 12 * u, y0 + head / 2 + 2 * u, w - 80 * u);
        if (side) { g.fillStyle = SIDE_COL[side]; g.font = `900 ${10 * u}px ${FONT}`; g.textAlign = 'right'; g.fillText(side, x0 + w - 12 * u, y0 + head / 2 + 2 * u); }
        if (R) { const al = aliveAt(R, gt)[i]; g.fillStyle = 'rgba(255,255,255,.6)'; g.font = `700 ${10 * u}px ${FONT}`; g.textAlign = 'right'; g.fillText(`${al}/5`, x0 + w - 36 * u, y0 + head / 2 + 2 * u); }
      }
      const ps = (teams[i] && teams[i].players) || [];
      ps.forEach((mp, k) => {
        const pl = R && R.players[mp.id];
        const alive = pl ? gt < pl.deathT : true;
        const y = y0 + head + 4 * u + k * rowH;
        const kills = pl ? R.kills.filter(k2 => k2.killer === mp.id && k2.t <= gt).length : 0;
        g.globalAlpha = alive ? 1 : 0.38;
        g.fillStyle = alive ? rgba(col, 0.15) : 'rgba(255,255,255,.03)'; g.fillRect(x0 + 8 * u, y + 1 * u, w - 16 * u, rowH - 2 * u);
        g.textBaseline = 'middle';
        if (sideL) {
          const fs = 12 * u;
          g.textAlign = 'left'; g.fillStyle = '#fff'; g.font = `700 ${fs}px ${FONT}`; g.fillText(mp.handle, x0 + 14 * u, y + rowH * 0.33, w * 0.5);
          g.fillStyle = 'rgba(255,255,255,.45)'; g.font = `600 ${fs * 0.78}px ${FONT}`; g.fillText(mp.role || '', x0 + 14 * u, y + rowH * 0.72);
          if (pl) {
            g.textAlign = 'right'; g.fillStyle = 'rgba(255,255,255,.92)'; g.font = `700 ${fs * 0.88}px ${FONT}`;
            g.fillText(alive ? (WEAPON[pl.weapon] || pl.weapon || '') : '✕', x0 + w - 14 * u, y + rowH * 0.33);
            g.fillStyle = 'rgba(170,255,170,.8)'; g.font = `600 ${fs * 0.78}px ${FONT}`; g.fillText(money(pl.money), x0 + w - 14 * u, y + rowH * 0.72);
            if (kills) { g.fillStyle = '#ffd34d'; g.font = `800 ${fs * 0.85}px ${FONT}`; g.fillText('×' + kills, x0 + w - 74 * u, y + rowH * 0.72); }
            if (R.bomb === mp.id && !R.plant) this.bombIcon(x0 + w * 0.56, y + rowH * 0.33, 5 * u, alive ? 1 : 0.5);
          }
        } else {
          const fs = Math.min(11.5 * u, rowH * 0.62);
          g.textAlign = 'left'; g.fillStyle = '#fff'; g.font = `700 ${fs}px ${FONT}`; g.fillText(mp.handle, x0 + 14 * u, y + rowH / 2, w * 0.4);
          if (this.narrow) {
            if (pl) {
              g.textAlign = 'right'; g.fillStyle = 'rgba(255,255,255,.9)'; g.font = `700 ${fs * 0.88}px ${FONT}`;
              g.fillText(alive ? (WEAPON[pl.weapon] || pl.weapon || '') : '✕', x0 + w - 30 * u, y + rowH / 2);
              if (kills) { g.fillStyle = '#ffd34d'; g.font = `800 ${fs * 0.9}px ${FONT}`; g.fillText('×' + kills, x0 + w - 8 * u, y + rowH / 2); }
            }
            g.globalAlpha = 1;
            return;
          }
          g.fillStyle = 'rgba(255,255,255,.45)'; g.font = `600 ${fs * 0.82}px ${FONT}`; g.fillText(mp.role || '', x0 + 14 * u + w * 0.22, y + rowH / 2);
          if (pl) {
            g.textAlign = 'right';
            g.fillStyle = 'rgba(255,255,255,.9)'; g.font = `700 ${fs * 0.9}px ${FONT}`;
            g.fillText(WEAPON[pl.weapon] || pl.weapon || '', x0 + w - 80 * u, y + rowH / 2);
            g.fillStyle = 'rgba(170,255,170,.8)'; g.font = `600 ${fs * 0.85}px ${FONT}`; g.fillText(money(pl.money), x0 + w - 34 * u, y + rowH / 2);
            if (kills) { g.fillStyle = '#ffd34d'; g.font = `800 ${fs * 0.9}px ${FONT}`; g.fillText('×' + kills, x0 + w - 10 * u, y + rowH / 2); }
            if (R.bomb === mp.id && !R.plant) this.bombIcon(x0 + w * 0.2, y + rowH / 2, 5 * u, alive ? 1 : 0.5);
          }
        }
        g.globalAlpha = 1;
      });
    });
  }

  drawTicker() {
    if (!this.ticker) return;
    const g = this.ctx, u = this.u;
    const age = (performance.now() - this.ticker.t0) / 1000;
    if (age > 4.5) return;
    const a = clamp((4.5 - age) / 0.6, 0, 1);
    g.font = `600 ${12.5 * u}px ${FONT}`;
    const text = this.ticker.text;
    const tw = Math.min(g.measureText(text).width + 28 * u, this.W - 20 * u);
    const y = this.box.y + this.box.h - 30 * u, x = (this.W - tw) / 2;
    g.save(); g.globalAlpha = a;
    g.fillStyle = this.ticker.hl ? 'rgba(110,70,10,.85)' : 'rgba(6,9,14,.78)'; g.beginPath(); g.roundRect(x, y, tw, 24 * u, 12 * u); g.fill();
    g.fillStyle = this.ticker.hl ? '#ffd34d' : '#fff'; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText(text, this.W / 2, y + 12 * u, tw - 20 * u);
    g.restore();
  }

  drawIntro(R) {
    const g = this.ctx, u = this.u, W = this.W, T = this.T;
    const names = this.names();
    const k = clamp((1.7 - this.hold) / 0.3, 0, 1) * clamp(this.hold / 0.3, 0, 1);
    const w = Math.min(W - 24 * u, 560 * u), h = 112 * u, x = (W - w) / 2, y = this.box.y + this.box.h * 0.32 - h / 2;
    g.save(); g.globalAlpha = k;
    g.fillStyle = 'rgba(6,9,14,.9)'; g.beginPath(); g.roundRect(x, y, w, h, 10 * u); g.fill();
    g.fillStyle = '#fff'; g.font = `900 ${20 * u}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle';
    g.fillText(`${T.round} ${R.num}`, W / 2, y + 22 * u);
    if (R.pistol) { g.fillStyle = '#ffd34d'; g.font = `800 ${10 * u}px ${FONT}`; g.fillText(T.pistol, W / 2, y + 40 * u); }
    [0, 1].forEach(i => {
      const cx = x + w * (i === 0 ? 0.25 : 0.75);
      const side = this.sideOf(i, R), b = R.buys[i] || {};
      const call = side === 'T' ? R.calls.t : R.calls.ct;
      g.fillStyle = this.teamColor(i); g.font = `800 ${13 * u}px ${FONT}`;
      g.fillText(`${names[i]}`, cx, y + 56 * u, w * 0.46);
      g.fillStyle = SIDE_COL[side]; g.font = `800 ${10.5 * u}px ${FONT}`;
      g.fillText(`${side} · ${T.buy[b.level] || ''} ${b.spent != null ? money(b.spent) : ''}`, cx, y + 74 * u, w * 0.46);
      if (call) {
        const label = side === 'T' ? `${prettyCall(call.strategy)} · ${prettyCall(call.pace)}` : prettyCall(call.setup);
        g.fillStyle = 'rgba(255,255,255,.7)'; g.font = `600 ${10 * u}px ${FONT}`; g.fillText(label, cx, y + 92 * u, w * 0.46);
      }
    });
    g.restore();
  }

  drawBanner(R) {
    if (!R.end) return;
    const g = this.ctx, u = this.u, W = this.W, T = this.T;
    const age = (performance.now() - this.banner.t0) / 1000;
    const k = easeOut(clamp(age * 3, 0, 1));
    const win = R.end.winner, col = this.teamColor(win), names = this.names();
    const h = 74 * u, y = this.box.y + this.box.h * 0.42 - h / 2;
    const w = Math.min(W - 20 * u, 620 * u) * k, x = (W - w) / 2;
    g.save();
    g.fillStyle = rgba(col, 0.92); g.beginPath(); g.moveTo(x + 14 * u, y); g.lineTo(x + w, y); g.lineTo(x + w - 14 * u, y + h); g.lineTo(x, y + h); g.closePath(); g.fill();
    if (k > 0.6) {
      const fg = luminance(col) > 0.6 ? '#0d1117' : '#fff';
      g.fillStyle = fg; g.textAlign = 'center'; g.textBaseline = 'middle';
      g.font = `900 italic ${20 * u}px ${FONT}`; g.fillText(`${names[win]} ${T.wins}`.toUpperCase(), W / 2, y + 22 * u, w - 30 * u);
      const tags = (R.end.tags || []).filter(t => t !== 'pistol').map(t => t === 'clutch' ? `CLUTCH 1v${R.end.clutch}` : (T.tags[t] || t));
      g.font = `700 ${12 * u}px ${FONT}`;
      g.fillText([T.reasons[R.end.reason] || R.end.reason, ...tags].join(' · '), W / 2, y + 44 * u, w - 30 * u);
      const mvp = R.players[R.end.mvp];
      if (mvp) {
        const kc = R.kills.filter(q => q.killer === mvp.id).length;
        g.font = `600 ${11 * u}px ${FONT}`; g.globalAlpha = 0.9;
        g.fillText(`${T.mvp}: ${mvp.handle} (${kc} ${T.kills})`, W / 2, y + 62 * u, w - 30 * u);
      }
    }
    g.restore();
  }

  drawOverlay() {
    const o = this.overlay, g = this.ctx, u = this.u, W = this.W, H = this.H, T = this.T;
    const age = (performance.now() - o.t0) / 1000;
    const a = clamp(age * 2.5, 0, 1);
    g.save(); g.globalAlpha = a;
    g.fillStyle = 'rgba(4,6,10,.82)'; g.fillRect(0, 0, W, H);
    const names = this.names();
    if (o.kind === 'half') {
      const d = dataOf(o.e);
      const title = d.after === 12 ? T.half : d.after === 24 ? T.ot : T.otSwitch;
      g.fillStyle = '#ffd34d'; g.font = `900 ${26 * u}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle';
      g.fillText(title, W / 2, H * 0.32);
      this.scoreLine(H * 0.48, names, d.score || this.score, 34);
      g.fillStyle = 'rgba(255,255,255,.7)'; g.font = `600 ${12 * u}px ${FONT}`;
      if (d.money) g.fillText(`${T.switchNote} ${money(d.money)}`, W / 2, H * 0.62);
      g.restore(); return;
    }
    // final
    const m = this.match || {};
    const me = [...this.events].reverse().find(e => e.kind === 'match_end');
    const d = me ? dataOf(me) : null;
    const score = (d && d.score) || m.score || this.score;
    g.textAlign = 'center'; g.textBaseline = 'middle';
    if (!d) {
      g.fillStyle = '#ff8a80'; g.font = `900 ${22 * u}px ${FONT}`;
      g.fillText(m.status === 'failed' ? T.failed : T.stopped, W / 2, H * 0.36);
      this.scoreLine(H * 0.52, names, score, 30);
      g.restore(); return;
    }
    const win = d.winner, col = this.teamColor(win);
    g.fillStyle = '#ffd34d'; g.font = `900 ${14 * u}px ${FONT}`; g.fillText(T.final, W / 2, H * 0.1);
    g.fillStyle = col; g.font = `900 ${(this.narrow ? 20 : 28) * u}px ${FONT}`;
    g.fillText(`${names[win]} ${T.winsMatch}`.toUpperCase(), W / 2, H * 0.18, W - 30 * u);
    this.scoreLine(H * 0.29, names, score, this.narrow ? 26 : 34);
    // MVP
    const ps = d.players || [];
    const mvp = ps.find(p => p.id === d.mvp);
    const cw = Math.min(W - 24 * u, 640 * u), cx = (W - cw) / 2;
    let y = H * 0.38;
    if (mvp) {
      g.fillStyle = rgba(this.teamColor(mvp.team), 0.25); g.beginPath(); g.roundRect(cx, y, cw, 46 * u, 8 * u); g.fill();
      g.fillStyle = '#ffd34d'; g.font = `900 ${12 * u}px ${FONT}`; g.textAlign = 'left'; g.fillText(T.mvp, cx + 14 * u, y + 23 * u);
      g.fillStyle = '#fff'; g.font = `900 ${18 * u}px ${FONT}`; g.fillText(mvp.handle, cx + 52 * u, y + 23 * u);
      g.textAlign = 'right'; g.font = `700 ${12 * u}px ${FONT}`;
      const hs = mvp.k ? Math.round(100 * mvp.hs / mvp.k) : 0;
      g.fillText(`${mvp.k}-${mvp.d} · ADR ${mvp.adr} · HS ${hs}% · ${mvp.rating}`, cx + cw - 14 * u, y + 23 * u);
      y += 56 * u;
    }
    // Taula: els dos equips.
    const cols = T.cols;
    const rowH = Math.min(20 * u, (H * 0.95 - y) / 13);
    [0, 1].forEach(ti => {
      const list = ps.filter(p => p.team === ti).sort((a2, b2) => b2.rating - a2.rating);
      g.fillStyle = this.teamColor(ti); g.font = `800 ${11.5 * u}px ${FONT}`; g.textAlign = 'left';
      g.fillText(names[ti], cx + 6 * u, y + rowH / 2, cw * 0.4);
      g.fillStyle = 'rgba(255,255,255,.5)'; g.font = `700 ${10 * u}px ${FONT}`; g.textAlign = 'right';
      cols.forEach((c, i) => g.fillText(c, cx + cw * (0.58 + i * 0.12), y + rowH / 2));
      y += rowH;
      list.forEach(p => {
        g.fillStyle = 'rgba(255,255,255,.05)'; g.fillRect(cx, y + 1, cw, rowH - 2);
        g.fillStyle = '#fff'; g.font = `700 ${10.5 * u}px ${FONT}`; g.textAlign = 'left'; g.fillText(p.handle, cx + 10 * u, y + rowH / 2);
        g.textAlign = 'right'; g.font = `600 ${10.5 * u}px ${FONT}`;
        const vals = [p.k, p.d, p.adr, p.k ? Math.round(100 * p.hs / p.k) + '%' : '0%'];
        vals.forEach((v, i) => g.fillText(String(v), cx + cw * (0.58 + i * 0.12), y + rowH / 2));
        y += rowH;
      });
      y += 6 * u;
    });
    g.restore();
  }

  scoreLine(y, names, score, size) {
    const g = this.ctx, u = this.u, W = this.W;
    g.textBaseline = 'middle';
    g.font = `900 ${size * u}px ${FONT}`; g.textAlign = 'center';
    g.fillStyle = '#fff'; g.fillText(`${score[0] || 0}  –  ${score[1] || 0}`, W / 2, y);
    g.font = `800 ${(size * 0.45) * u}px ${FONT}`;
    g.fillStyle = this.teamColor(0); g.textAlign = 'right'; g.fillText(names[0], W / 2 - size * 2.2 * u, y, W * 0.3);
    g.fillStyle = this.teamColor(1); g.textAlign = 'left'; g.fillText(names[1], W / 2 + size * 2.2 * u, y, W * 0.3);
  }
}
