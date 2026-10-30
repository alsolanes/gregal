// Visor d'un partit de Counter-Strike 2 de l'Arena (internal/arena/cs2): un
// minimapa tàctic com el de les retransmissions (Mirage estilitzat, zones,
// caixes, llocs A i B), jugadors amb el color de l'equip i la vora del bàndol,
// cons de visió, traçadores, fums, flaixos i molotovs, la bomba amb el seu
// rellotge, el marcador de dalt, el feed de morts, les plantilles de baix i
// els rètols de final de ronda, mitja part i final amb l'MVP.
//
// Les rondes arriben senceres (un punt de decisió = una ronda); cada ronda es
// reprodueix comprimint els temps morts perquè duri uns 8-15 s a 1×.

import { ArenaViewBase, lerp, clamp, easeOut, rng, hexToRgb as hexRgb, viewDpr, elapsedAt, progressMarks } from './arena-view-base.js';
export { elapsedAt, progressMarks };

// Progress-bar marks (progressMarks) and playback seconds (elapsedAt) live in arena-view-base.js.
const FONT = '"Segoe UI", system-ui, sans-serif';
const SIDE_COL = { T: '#e8a33d', CT: '#5aa9e6' };
const ROUND_TIME = 115, BOMB_TIME = 40;

const GREY = [140, 140, 140];
const hexToRgb = h => hexRgb(h, GREY);
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
    players[p.id] = { id: p.id, team: p.team, side: p.side, handle: (hm[p.id] || {}).handle || p.id, role: (hm[p.id] || {}).role || '', weapon: p.weapon, money: p.money, armor: p.armor, track: [{ t: 0, x: +p.x || 0, y: +p.y || 0 }], deathT: Infinity, death: null };
  }
  const R = { num: d.round || 0, tTeam: d.tTeam || 0, ctTeam: d.ctTeam != null ? d.ctTeam : 1 - (d.tTeam || 0), pistol: !!d.pistol, players, kills: [], utils: [], buys: [], plant: null, defuse: null, explode: null, bomb: d.bomb || null, money: d.money || [0, 0], calls: d.calls || {}, half: d.half || '', events: g.events.slice(), after: g.after || [], end: g.end ? dataOf(g.end) : null, endT: 0 };
  for (const e of R.events) {
    const x = dataOf(e); const t = Math.max(0, +x.t || 0);
    e._t = t;
    R.endT = Math.max(R.endT, t);
    switch (e.kind) {
      case 'buy': R.buys[x.team] = x; for (const it of x.items || []) if (players[it.id]) { players[it.id].weapon = it.weapon; players[it.id].money = it.money; if (it.armor != null) players[it.id].armor = it.armor; } break;
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

// ---- spectator cam: pure 3D helpers (tested without a DOM) ----
//
// The POV camera is a classic grid raycaster built from the same geometry as
// the tactical map: zones (rooms) and links (corridors) are floor, props are
// waist-high crates, everything else is wall. 1 map unit ~ 1 metre.

export const EYE = 1.62, CRATE_H = 1.1, BODY_H = 1.8;
const CORRIDOR = 2.5; // corridor half-width (the map draws them 4.6-5.2 wide)

export function angDiff(a, b) { let d = (a - b) % (Math.PI * 2); if (d > Math.PI) d -= Math.PI * 2; if (d < -Math.PI) d += Math.PI * 2; return d; }
function hash3(a, b, c) { let h = (a * 374761393 + b * 668265263 + c * 2147483647) | 0; h = (h ^ (h >>> 13)) * 1274126177 | 0; return ((h ^ (h >>> 16)) >>> 0) / 4294967296; }
function segDist(px, py, ax, ay, bx, by) {
  const dx = bx - ax, dy = by - ay, l2 = dx * dx + dy * dy;
  const t = l2 ? clamp(((px - ax) * dx + (py - ay) * dy) / l2, 0, 1) : 0;
  return Math.hypot(px - ax - t * dx, py - ay - t * dy);
}

// Occupancy grid: 0 wall, 1 floor, 2 crate. `zone` is the nearest zone of each
// cell (wall tint), `hgt` the wall height in decimetres (a varied skyline).
export function buildGrid(meta, res = 0.5) {
  const size = (meta && meta.size) || [100, 80];
  const gw = Math.ceil(size[0] / res), gh = Math.ceil(size[1] / res);
  const cells = new Uint8Array(gw * gh), zone = new Int8Array(gw * gh).fill(-1), hgt = new Uint8Array(gw * gh);
  const zs = (meta && meta.zones) || [], byId = {}; zs.forEach(z => { byId[z.id] = z; });
  const links = ((meta && meta.links) || []).map(([a, b]) => [byId[a], byId[b]]).filter(l => l[0] && l[1]);
  const props = (meta && meta.props) || [];
  // Plain loops and squared distances: this runs for every cell (~32k) and
  // used to take ~60 ms, a visible hitch on the first POV cut.
  const nz = zs.length, zx = new Float64Array(nz), zy = new Float64Array(nz), zw = new Float64Array(nz), zh = new Float64Array(nz);
  zs.forEach((z, zi) => { zx[zi] = z.x; zy[zi] = z.y; zw[zi] = z.w / 2; zh[zi] = z.h / 2; });
  const C2 = CORRIDOR * CORRIDOR;
  const segs = links.map(([a, b]) => { const dx = b.x - a.x, dy = b.y - a.y; return { ax: a.x, ay: a.y, dx, dy, l2: dx * dx + dy * dy, x0: Math.min(a.x, b.x) - CORRIDOR, x1: Math.max(a.x, b.x) + CORRIDOR, y0: Math.min(a.y, b.y) - CORRIDOR, y1: Math.max(a.y, b.y) + CORRIDOR }; });
  for (let j = 0; j < gh; j++) for (let i = 0; i < gw; i++) {
    const x = (i + 0.5) * res, y = (j + 0.5) * res, k = j * gw + i;
    let best = 1e18, bi = -1, floor = false;
    for (let zi = 0; zi < nz; zi++) {
      const ddx = Math.abs(x - zx[zi]) - zw[zi], ddy = Math.abs(y - zy[zi]) - zh[zi];
      const dx = ddx > 0 ? ddx : 0, dy = ddy > 0 ? ddy : 0, d2 = dx * dx + dy * dy;
      if (d2 < best) { best = d2; bi = zi; }
      if (d2 <= 0.09) floor = true;
    }
    if (!floor) for (const q of segs) {
      if (x < q.x0 || x > q.x1 || y < q.y0 || y > q.y1) continue;
      const t = q.l2 ? clamp(((x - q.ax) * q.dx + (y - q.ay) * q.dy) / q.l2, 0, 1) : 0;
      const ex = x - q.ax - t * q.dx, ey = y - q.ay - t * q.dy;
      if (ex * ex + ey * ey <= C2) { floor = true; break; }
    }
    let c = floor ? 1 : 0;
    if (floor) for (const p of props) if (Math.abs(x - p.x) <= p.w / 2 && Math.abs(y - p.y) <= p.h / 2) { c = 2; break; }
    cells[k] = c; zone[k] = bi;
    const kind = bi >= 0 ? zs[bi].kind : '';
    const block = hash3(Math.floor(x / 3.5), Math.floor(y / 3.5), 7);
    hgt[k] = Math.round(10 * ((kind === 'spawn_t' || kind === 'spawn_ct' ? 5.2 : 3.6) + block * 3.2));
  }
  return { gw, gh, res, cells, zone, hgt, zones: zs, w: size[0], h: size[1] };
}

export function cellAt(G, x, y) {
  const i = Math.floor(x / G.res), j = Math.floor(y / G.res);
  if (i < 0 || j < 0 || i >= G.gw || j >= G.gh) return 0;
  return G.cells[j * G.gw + i];
}

// DDA raycast. (dx,dy) need not be unit: the returned d is in units of the ray
// parameter, which with camera-plane rays is the perpendicular distance.
// A ray that starts inside a wall first walks out of it (players hug walls);
// crates crossed on the way are pushed onto `crates` as {d0, d1}.
export function castRay(G, ox, oy, dx, dy, maxD = 90, crates = null) {
  const r = G.res;
  let mx = Math.floor(ox / r), my = Math.floor(oy / r);
  const ddx = dx === 0 ? 1e30 : Math.abs(1 / dx) * r, ddy = dy === 0 ? 1e30 : Math.abs(1 / dy) * r;
  const stepX = dx < 0 ? -1 : 1, stepY = dy < 0 ? -1 : 1;
  let sdx = dx < 0 ? (ox / r - mx) * ddx : (mx + 1 - ox / r) * ddx;
  let sdy = dy < 0 ? (oy / r - my) * ddy : (my + 1 - oy / r) * ddy;
  const at = (i, j) => (i < 0 || j < 0 || i >= G.gw || j >= G.gh) ? -1 : j * G.gw + i;
  let k0 = at(mx, my);
  let escaping = k0 < 0 || G.cells[k0] === 0, inCrate = !escaping && G.cells[k0] === 2, crateD = -1;
  let d = 0, side = 0;
  for (let n = 0; n < 4000; n++) {
    if (sdx < sdy) { d = sdx; sdx += ddx; mx += stepX; side = 0; } else { d = sdy; sdy += ddy; my += stepY; side = 1; }
    if (d > maxD) break;
    const k = at(mx, my), c = k < 0 ? 0 : G.cells[k];
    if (escaping) { if (c !== 0 && k >= 0) escaping = false; continue; }
    if (c === 2) { if (!inCrate && crateD < 0) crateD = d; continue; }
    inCrate = false;
    if (crateD >= 0) { if (crates) crates.push({ d0: crateD, d1: d }); crateD = -1; }
    if (c === 0) {
      const hx = ox + dx * d, hy = oy + dy * d;
      const tex = side === 0 ? hy : hx;
      return { d, side, hx, hy, tex, k, hit: true };
    }
  }
  return { d: maxD, side, hx: ox + dx * maxD, hy: oy + dy * maxD, tex: 0, k: -1, hit: false };
}

// Mirage sandstone, tinted by the zone a wall belongs to.
const HAZE = [226, 210, 178];
// Both memoised: the raycaster asks for them for every column and brick row, every frame.
const PALS = new WeakMap(), NOPAL = [188, 162, 122], CRATE_TOP = [150, 116, 72], CRATE_SIDE = [118, 86, 50];
function wallPal(z) {
  if (!z) return NOPAL;
  let p = PALS.get(z);
  if (!p) { p = wallPal0(z); PALS.set(z, p); }
  return p;
}
function wallPal0(z) {
  const j = (hash3(z.id.length, z.id.charCodeAt(0), 11) - 0.5) * 18;
  const base = z.kind === 'site' ? [216, 186, 140] : z.kind === 'spawn_t' ? [204, 160, 106] : z.kind === 'spawn_ct' ? [178, 172, 158] : [198, 170, 128];
  return base.map(v => v + j);
}
const SHADECOLS = new WeakMap();
function shadeCol(pal, sh, fog) {
  // Quantised (1/128 of shade, 1/64 of fog): invisible steps, a few hundred strings instead of thousands per frame.
  const qs = Math.round(sh * 128), qf = Math.round(fog * 64), key = qs * 128 + qf;
  let m = SHADECOLS.get(pal);
  if (!m) { m = new Map(); SHADECOLS.set(pal, m); }
  let out = m.get(key);
  if (out === undefined) {
    const s2 = qs / 128, f2 = qf / 64, c = i => Math.round(clamp(pal[i] * s2 * (1 - f2) + HAZE[i] * f2, 0, 255));
    out = `rgb(${c(0)},${c(1)},${c(2)})`;
    if (m.size > 4096) m.clear();
    m.set(key, out);
  }
  return out;
}

// Line of sight between two points (true when no wall in between).
export function sees(G, ax, ay, bx, by) {
  const dist = Math.hypot(bx - ax, by - ay); if (dist < 0.01) return true;
  const h = castRay(G, ax, ay, (bx - ax) / dist, (by - ay) / dist, dist + 0.01);
  return !h.hit || h.d >= dist - 0.3;
}

// World point -> screen. cam: {x, y, ang, eye, focal, cx, horizon}. On the
// map y grows downwards, so the camera's right-hand vector is (-sin, cos).
export function projectPoint(cam, wx, wy, wz) {
  const rx = wx - cam.x, ry = wy - cam.y, c = Math.cos(cam.ang), s = Math.sin(cam.ang);
  const depth = rx * c + ry * s, lat = -rx * s + ry * c;
  if (depth <= 0.05) return { x: 0, y: 0, depth, behind: true };
  return { x: cam.cx + lat / depth * cam.focal, y: cam.horizon - (wz - cam.eye) / depth * cam.focal, depth, behind: false };
}

// Where the spectated player looks: along the path, pre-aiming at the victim of
// an upcoming kill, at the killer when about to die, down at the bomb.
export function povAim(R, id, t) {
  const pl = R.players[id]; if (!pl) return 0;
  const me = t < pl.deathT ? posAt(pl.track, t) : (pl.death || posAt(pl.track, t));
  let base = angleAt(R, id, t);
  const ahead = posAt(pl.track, Math.min(t + 0.7, pl.deathT));
  if (Math.hypot(ahead.x - me.x, ahead.y - me.y) > 0.35) base = Math.atan2(ahead.y - me.y, ahead.x - me.x);
  let target = null, w = 0;
  for (const k of R.kills) {
    const dt = k.t - t;
    if (k.killer === id && dt > -0.8 && dt < 2.4) {
      const v = R.players[k.victim];
      const vp = v && t < k.t ? posAt(v.track, t) : { x: k.vx, y: k.vy };
      const ww = dt <= 0 ? 1 : clamp(1 - (dt - 0.35) / 2, 0, 1);
      if (ww > w) { w = ww; target = Math.atan2(vp.y - me.y, vp.x - me.x); }
    } else if (k.victim === id && dt > -0.5 && dt < 0.9) {
      const ww = dt <= 0 ? 1 : clamp(1 - dt / 0.9, 0, 1);
      if (ww > w) { w = ww; target = Math.atan2(k.ky - k.vy, k.kx - k.vx); }
    }
  }
  const b = R.plant;
  if (b && w < 1) {
    const d = R.defuse;
    if (b.id === id && t >= b.start - 0.6 && t <= b.t + 0.3) { w = 1; target = Math.atan2(b.y - me.y, b.x - me.x); }
    else if (d && d.id === id && t >= d.start - 0.8 && t <= d.t + 0.3) { w = 1; target = Math.atan2(d.y - me.y, d.x - me.x); }
  }
  if (target == null || !isFinite(target)) return base;
  return base + angDiff(target, base) * w;
}

// Broadcast director: which camera the round wants at play-time p.
// {mode:'map'} or {mode:'pov', id, why, urgent}.
export function directorWant(R, p, gt) {
  const alive = id => { const q = R.players[id]; return q && gt < q.deathT; };
  if (R.defuse && alive(R.defuse.id)) {
    const s = playTimeAt(R.tl, R.defuse.start), e = playTimeAt(R.tl, R.defuse.t);
    if (p >= s - 0.6 && p <= e + 0.8) return { mode: 'pov', id: R.defuse.id, why: 'defuse', urgent: true };
  }
  let next = null, prev = null;
  for (const k of R.kills) {
    if (k._p >= p && k._p - p <= 1.5 && (!next || k._p < next._p)) next = k;
    if (k._p < p && p - k._p <= 0.8 && (!prev || k._p > prev._p)) prev = k;
  }
  const killerOk = k => { const q = R.players[k.killer]; return q && q.deathT >= k.t; };
  // Planting beats a duel that is already over, but not one about to happen.
  if (R.plant && !(next && next._p - p < 0.6 && next.killer !== R.plant.id)) {
    const s = playTimeAt(R.tl, R.plant.start), e = playTimeAt(R.tl, R.plant.t);
    if (p >= s - 0.7 && p <= e + 0.6) return { mode: 'pov', id: R.plant.id, why: 'plant', urgent: true };
  }
  if (next && killerOk(next) && (!prev || next._p - p < 0.6 || next.killer === prev.killer)) return { mode: 'pov', id: next.killer, why: 'kill', urgent: next._p - p < 0.6, k: next };
  if (prev && killerOk(prev)) return { mode: 'pov', id: prev.killer, why: 'kill', urgent: false, k: prev };
  if (next && killerOk(next)) return { mode: 'pov', id: next.killer, why: 'kill', urgent: false, k: next };
  // Clutch: a lone survivor against two or more (or the 1v1 that decides it).
  const n = aliveAt(R, gt);
  if (p < R.tl.total - 0.3 && (n[0] === 1 || n[1] === 1) && n[0] + n[1] >= 3 && n[0] > 0 && n[1] > 0) {
    const team = n[0] === 1 ? 0 : 1;
    const solo = Object.values(R.players).find(q => q.team === team && gt < q.deathT);
    if (solo) return { mode: 'pov', id: solo.id, why: 'clutch', urgent: false };
  }
  return { mode: 'map' };
}

// The kill worth a slow-motion kill-cam replay at the end of the round (or null).
export function pickReplay(R) {
  let best = null, bs = 0;
  const count = {};
  const last = R.kills.length ? R.kills[R.kills.length - 1] : null;
  for (const k of R.kills) {
    count[k.killer] = (count[k.killer] || 0) + 1;
    if (!R.players[k.killer]) continue;
    let s = 0;
    if (k.hl) s += 4;
    if (count[k.killer] >= 3) s += count[k.killer] - 1;
    if (k.weapon === 'awp') s += 1;
    if (k.hs) s += 1;
    if (k.smoke) s += 1.5;
    if (aliveAt(R, k.t - 0.01)[k.kteam] === 1) s += 2;
    if (k === last) s += 0.5;
    if (k.first) s += 0.5;
    if (s > bs) { bs = s; best = k; }
  }
  return bs >= 3 ? best : null;
}

// Shots in the air at play-time p: the killer fires a short burst, the victim
// answers once. Returns [{from, to, kill, age, hs}].
export function shotsAt(R, p) {
  const out = [];
  for (const k of R.kills) {
    const awp = k.weapon === 'awp';
    const burst = awp ? [0] : [-0.26, -0.13, 0];
    for (const off of burst) {
      const age = p - (k._p + off);
      if (age >= 0 && age < 0.16) out.push({ from: k.killer, to: k.victim, kill: off === 0, age, hs: k.hs && off === 0, k, ax: k.kx, ay: k.ky, bx: k.vx, by: k.vy, miss: off !== 0 ? (off * 7) : 0 });
    }
    const age2 = p - (k._p - 0.2);
    if (age2 >= 0 && age2 < 0.14) out.push({ from: k.victim, to: k.killer, kill: false, age: age2, hs: false, k, ax: k.vx, ay: k.vy, bx: k.kx, by: k.ky, miss: 0.9 });
  }
  return out;
}

const MAG = { glock: [20, 120], usp: [12, 24], p250: [13, 26], deagle: [7, 35], mac10: [30, 100], mp9: [30, 120], galil: [35, 90], famas: [25, 90], ak47: [30, 90], m4a1s: [20, 80], awp: [5, 30] };
// The muzzle in the gun's own drawing frame (barrel along -x, receiver at 0,0).
export function gunTip(wid) {
  const cls = weaponClass(wid);
  if (cls === 'pistol') return wid === 'usp' ? [-106, -8] : [-61, -8];
  if (cls === 'awp') return [-162, -6];
  if (cls === 'smg') return [-68, -6];
  return wid === 'm4a1s' ? [-164, -6] : [-120, -6];
}

// Where the first-person gun sits in a w×h view: the receiver anchored low on
// the right, rotated so the barrel's axis runs through the crosshair (the
// view centre) at any aspect ratio, and shortened along the barrel (fx, a
// cheap perspective) so the muzzle stops short of the crosshair. `right`: where
// the HUD box ends (the side rosters float over the rest of the view).
export function gunPose(w, h, wid, right = w) {
  const pistol = weaponClass(wid) === 'pistol';
  const S = Math.min(h, w * 0.75) / 235, tip = gunTip(wid);
  const ax = Math.min(w * (pistol ? 0.74 : 0.86), right - (pistol ? 70 : 40) * S), ay = h - (pistol ? 40 : 24) * S;
  const dx = w / 2 - ax, dy = h / 2 - ay, D = Math.hypot(dx, dy) || 1, off = S * tip[1];
  let rot = Math.atan2(dy, dx) - Math.PI + Math.asin(clamp(off / D, -1, 1));
  rot = Math.atan2(Math.sin(rot), Math.cos(rot));
  const L = Math.sqrt(Math.max(0, D * D - off * off));
  const fx = clamp(L * (pistol ? 0.5 : 0.62) / (S * -tip[0]), 0.4, 1.15);
  return { ax, ay, rot, S, fx, tip };
}

export function weaponClass(w) { return w === 'awp' ? 'awp' : ['glock', 'usp', 'p250', 'deagle'].includes(w) ? 'pistol' : ['mac10', 'mp9'].includes(w) ? 'smg' : w ? 'rifle' : 'pistol'; }

// HUD numbers for the spectated player (health is cosmetic: duels won cost a bit).
export function hudStats(R, id, t) {
  const pl = R.players[id] || {};
  let hp = 100, shots = 0;
  for (const k of R.kills) {
    if (k.t > t) continue;
    if (k.killer === id) { shots += k.weapon === 'awp' ? 1 : weaponClass(k.weapon) === 'pistol' ? 2 : 4; if (!k.hs) hp -= Math.floor(hash3(k.n || 1, 3, 9) * 48); }
  }
  if (t >= (pl.deathT == null ? Infinity : pl.deathT)) hp = 0;
  const [mag, res] = MAG[pl.weapon] || [30, 90];
  const used = shots % mag, reserve = Math.max(0, res - Math.floor(shots / mag) * mag);
  return { hp: clamp(hp, 0, 100) | 0, armor: pl.armor != null ? pl.armor : 1, ammo: mag - used, reserve, money: pl.money || 0 };
}

// ---- the arena studio: pure helpers (director between rounds, crowd) ----

// Rounds a team needs to win the match at round r (MR12, overtimes MR3).
// half: rounds per half (12 = MR12; the live mode can play MR6).
export function winTarget(r, half = 12) { r = +r || 1; return r <= 2 * half ? half + 1 : half + 4 + 3 * Math.floor((r - 2 * half - 1) / 6); }
const halfOf = meta => (meta && meta.half) || 12;
// Teams one round away from winning the match when round r starts.
export function matchPointTeams(score, r, half = 12) { const t = winTarget(r, half); return [0, 1].filter(i => ((score && score[i]) || 0) === t - 1); }

// How loud the crowd gets for an event (0 quiet … 1 roaring) and whose fans
// erupt (-1: everybody). Quiet during the buy, a roar for aces, clutches and
// the kill that wins the round.
export function crowdReaction(e, R) {
  if (!e) return { level: 0, team: -1 };
  const d = dataOf(e);
  let lv = 0, team = -1;
  switch (e.kind) {
    case 'round_start': case 'buy': return { level: 0.04, team: -1 };
    case 'kill': {
      team = d.kteam != null ? +d.kteam : -1; lv = 0.28;
      if (d.hs) lv += 0.05;
      if (d.first) lv += 0.07;
      if (d.weapon === 'awp') lv += 0.05;
      const ks = R ? R.kills : [], k = ks.find(q => q.n === e.n);
      if (k) {
        const n = ks.filter(q => q.killer === k.killer && q.t <= k.t).length;
        if (n >= 3) lv += 0.15 * (n - 2);
        const al = aliveAt(R, k.t - 0.01);
        if (al[k.kteam] === 1 && al[k.vteam] >= 2) lv += 0.15;
        if (k === ks[ks.length - 1] && R.end && R.end.reason === 'elim') lv += 0.3;
      }
      if (e.hl) lv += 0.15;
      break;
    }
    case 'plant': lv = 0.3; team = d.team != null ? +d.team : R ? R.tTeam : -1; break;
    case 'defuse': lv = d.ninja ? 1 : 0.8; team = d.team != null ? +d.team : R ? R.ctTeam : -1; break;
    case 'explode': lv = 0.7; team = R ? R.tTeam : -1; break;
    case 'round_end': {
      lv = 0.5; team = d.winner != null ? +d.winner : -1;
      const bonus = { ace: 0.4, clutch: 0.3, '4k': 0.15, eco_win: 0.15, last_second: 0.2, ninja: 0.25 };
      for (const t of d.tags || []) lv += bonus[t] || 0;
      if (e.hl) lv += 0.1;
      break;
    }
    case 'half': lv = 0.35; break;
    case 'match_end': lv = 1; team = d.winner != null ? +d.winner : -1; break;
    default: return { level: 0, team: -1 };
  }
  return { level: clamp(lv, 0, 1), team: team === 0 || team === 1 ? team : -1 };
}

// Excitement of one fan given both teams' crowd levels: your team's moment is
// yours, the neutrals join in at half voice, the rivals barely stir.
export function fanExcitement(lv, fanTeam) {
  const a = (lv && lv[0]) || 0, b = (lv && lv[1]) || 0;
  if (fanTeam !== 0 && fanTeam !== 1) return clamp(Math.max(a, b) * 0.55, 0, 1);
  const own = fanTeam === 0 ? a : b, other = fanTeam === 0 ? b : a;
  return clamp(own + other * 0.1, 0, 1);
}

// The studio director: the shots between rounds, in order, with durations
// (seconds at 1×). 'live' keeps the game view (map/POV + banner), 'tactical'
// is the map with the buy card; everything else is an arena camera.
export function studioPlan(kind, o = {}) {
  if (kind === 'intro') {
    const mp = o.matchPoint || [];
    const screen = mp.length ? 'matchpoint' : o.ot ? 'overtime' : o.pistol ? 'pistol' : 'round';
    const out = [];
    if (o.first) out.push({ shot: 'desk', screen: 'welcome', dur: 2.6 });
    if (screen !== 'round') out.push({ shot: 'screen', screen, team: mp.length === 1 ? mp[0] : -1, dur: 2.4 });
    else out.push({ shot: (o.round || 0) % 4 === 3 ? 'crowd' : 'arena', screen, team: (o.round || 0) % 2, dur: o.first ? 2.2 : 1.3 });
    out.push({ shot: 'tactical', dur: 1.6 });
    return out;
  }
  if (kind === 'roundEnd') {
    const lv = o.level || 0, w = o.winner === 0 || o.winner === 1 ? o.winner : -1;
    const out = [{ shot: 'live', dur: 1.5 }];
    if (w < 0) return out;
    if (lv >= 0.8) out.push({ shot: 'crowd', screen: 'win', team: w, dur: 1.4 });
    out.push({ shot: 'booth', screen: 'win', team: w, dur: 1.7 + lv * 0.8 });
    return out;
  }
  if (kind === 'half') return [{ shot: 'screen', screen: 'half', team: -1, dur: 2.2 }, { shot: 'desk', screen: 'half', dur: 3.2 }];
  if (kind === 'matchEnd') {
    const w = o.winner === 0 || o.winner === 1 ? o.winner : 0;
    return [{ shot: 'arena', screen: 'champions', team: w, trophy: true, dur: 2.8 }, { shot: 'trophy', screen: 'champions', team: w, dur: 3.4 }, { shot: 'booth', screen: 'champions', team: w, dur: 2.2 }];
  }
  return [];
}
export function planLength(plan) { return (plan || []).reduce((a, s) => a + s.dur, 0); }

// Seconds a kill-cam replay lasts on screen (the same slow motion as the viewer's replay phase).
export function replaySeconds(k) {
  if (!k) return 0;
  let g = Math.max(0, k.t - 1.5), s = 0;
  const end = k.t + 0.7, dt = 0.02;
  while (g < end) { const near = Math.abs(g - k.t); g += dt * (near < 0.35 ? 0.28 : near < 0.8 ? 0.6 : 1); s += dt; }
  return s;
}

// Playback time at 1x of every event (in arrival order) and the total: the
// studio intro, the round's compressed timeline, the round-end studio and the
// kill-cam replay, half-time and the trophy ceremony. Returns events.length + 1
// start times (the last one is the total), never going back.
export function matchTimes(events, meta) {
  const at = new Map();
  let t = 0, first = true, score = [0, 0];
  for (const g of groupRounds(events)) {
    const d = dataOf(g.start), round = d.round || 0;
    const intro = planLength(studioPlan('intro', { round, first, matchPoint: matchPointTeams(score, round, halfOf(meta)), ot: round > 2 * halfOf(meta), pistol: !!d.pistol }));
    first = false;
    if (!g.end) { for (const e of g.events) at.set(e, e === g.start ? t : t + intro); t += intro; continue; }
    const R = buildRound(g, meta || {});
    for (const e of R.events) at.set(e, (e.kind === 'round_start' || e.kind === 'buy') && e._p <= 0 ? t : t + intro + e._p);
    t += intro + R.tl.total;
    const end = R.events.find(e => e.kind === 'round_end');
    t += planLength(studioPlan('roundEnd', { winner: R.end && R.end.winner, level: crowdReaction(end, R).level })) + replaySeconds(pickReplay(R));
    if (end && Array.isArray(end.score)) score = end.score;
    for (const e of g.after) { at.set(e, t); t += planLength(studioPlan(e.kind === 'half' ? 'half' : 'matchEnd', { winner: dataOf(e).winner })); }
  }
  const out = []; let m = 0;
  for (const e of events) { m = Math.max(m, at.has(e) ? at.get(e) : m); out.push(m); }
  out.push(Math.max(m, t));
  return out;
}
// The step on air t seconds into a plan: {s, i, t (into the step), k (0..1)}.
export function shotAt(plan, t) {
  if (!plan || !plan.length) return null;
  let acc = 0;
  for (let i = 0; i < plan.length; i++) {
    const s = plan[i];
    if (t < acc + s.dur || i === plan.length - 1) { const lt = Math.max(0, t - acc); return { s, i, t: lt, k: clamp(lt / s.dur, 0, 1) }; }
    acc += s.dur;
  }
  return null;
}
// Camera move for an arena shot (virtual 1600×900 stage): focus and zoom.
export function shotCamera(s, k) {
  const e = k < 0.5 ? 2 * k * k : 1 - Math.pow(-2 * k + 2, 2) / 2;
  switch (s && s.shot) {
    case 'screen': return { fx: 800, fy: lerp(250, 430, e), z: lerp(1.75, 1.04, e) };
    case 'crowd': { const x = s.team === 1 ? 1170 : 430; return { fx: lerp(x, x + (s.team === 1 ? -70 : 70), e), fy: lerp(700, 680, e), z: lerp(1.95, 1.8, e) }; }
    case 'booth': return { fx: lerp(780, 820, e), fy: 450, z: lerp(1.08, 1.0, e) };
    case 'desk': return { fx: 800, fy: 450, z: lerp(1.0, 1.07, e) };
    case 'trophy': return { fx: 800, fy: lerp(430, 470, e), z: lerp(1.12, 1.0, e) };
    default: return s && s.trophy ? { fx: 800, fy: lerp(450, 470, e), z: lerp(1.0, 1.32, e) } : { fx: 800, fy: lerp(465, 440, e), z: lerp(1.0, 1.08, e) };
  }
}

// The crowd: rows of fans (virtual coordinates), back rows first. Side stands
// face the stage, the floor is seen from behind. Built once, drawn instanced.
export function buildCrowd(seed = 7) {
  const r = rng(seed), rows = [];
  let count = 0;
  const fan = (x, y, s, pTeam0, pTeam1) => {
    const q = r(), team = q < pTeam0 ? 0 : q < pTeam0 + pTeam1 ? 1 : -1;
    const pal = team < 0 ? 4 + Math.floor(r() * 3) : team * 2 + (r() < 0.6 ? 0 : 1);
    const pr = r();
    return { x, y, s, team, k: pal, ph: r() * 6.283, th: 0.22 + r() * 0.5, prop: pr < 0.42 ? 1 : pr < 0.46 ? 2 : pr < 0.54 ? 3 : 0, skin: Math.floor(r() * 3), _x: x, _y: y };
  };
  for (let side = 0; side < 2; side++) {
    for (let k = 0; k < 12; k++) {
      const yi = 252 + k * 27, yo = yi + 42 + k * 5, si = 5 + k * 0.32, fans = [];
      for (let t = 0; t <= 1;) {
        const s = lerp(si * 1.55, si, t), x = lerp(4, 296, t), y = lerp(yo, yi, t);
        fans.push(fan(side ? 1600 - x : x, y + (r() - 0.5) * s * 0.2, s, side ? 0.12 : 0.66, side ? 0.66 : 0.12));
        t += s * (1.05 + r() * 0.2) / 300;
      }
      fans.sort((a, b) => a.k - b.k);
      rows.push({ kind: 'side', fans, band: Math.min(3, k >> 2) });
      count += fans.length;
    }
  }
  const N = 15;
  for (let k = 0; k < N; k++) {
    const t = k / (N - 1), s = 7 + 21 * Math.pow(t, 1.5), y = 630 + 300 * Math.pow(t, 1.2), fans = [];
    for (let x = -20 + r() * s; x < 1620; x += s * (1.0 + r() * 0.22)) {
      const left = x < 800;
      fans.push(fan(x, y + (r() - 0.5) * s * 0.25, s * (0.92 + r() * 0.16), left ? 0.6 : 0.16, left ? 0.16 : 0.6));
    }
    fans.sort((a, b) => a.k - b.k);
    rows.push({ kind: 'floor', fans, band: Math.min(3, Math.floor(t * 4)) });
    count += fans.length;
  }
  return { rows, count };
}

// ---- el visor ----

export class Cs2View extends ArenaViewBase {
  constructor(canvas, opts = {}) {
    super(canvas, opts);
    this.lang = 'en'; // the games are always shown in English (TEXT.ca is kept for old callers)
    this.T = TEXT[this.lang];
    this.source = opts.source;
    this.onUpdate = opts.onUpdate || (() => {});
    this.onEvent = opts.onEvent || null;
    this.match = null; this.meta = null; this.events = []; this.rounds = []; this.lastN = 0; this.nIdx = new Map(); this.at = 0;
    this.ri = 0; this.R = null; this.phase = 'wait'; this.p = 0; this.hold = 0; this.fired = 0;
    this.score = [0, 0]; this.ticker = null; this.banner = null; this.overlay = null;
    this.mapCache = null;
    // Broadcast camera: 'map' (tactical) or 'pov' (spectator cam on one player).
    this.view = { mode: 'map', id: null, why: '' }; this.shotAge = 0; this.calm = 0; this.cutT = 0; this.cam = null;
    this.replay = null; this.replayed = false; this.grid = null; this.fdt = 0.016;
    // Arena studio: the shots between rounds, the crowd's mood per team, confetti.
    this.clock = 0; this.studio = null; this.halfE = null; this.party = null; this.seenFirst = false;
    this.cheer = [0.06, 0.06]; this.boothPulse = [-99, -99]; this.confetti = []; this.crowd = null; this.sCache = new Map();
    this.start(); // arena-view-base.js: size, frame loop, observers and the first poll
  }

  setSpeed(x) { super.setSpeed(+x || 1); }
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
    this.banner = null; this.overlay = null; this.ticker = null; this.replay = null; this.view = { mode: 'map', id: null, why: '' }; this.studio = null; this.halfE = null;
    if (this.R.after.some(e => e.kind === 'match_end') || this.terminal()) { this.ri = this.rounds.length; this.enterFinal(); }
    else { this.ri++; this.phase = 'wait'; }
    this.at = this.events.length;
    this.invalidate();
  }

  // Jump to event index (0 = the start), an index into the events as they
  // arrived. The round it belongs to is rebuilt (rounds are self-contained: the
  // start carries players, money and calls) with everything before the event
  // marked as played, the score taken from the last event before it, and the
  // studio, replays, banners and crowd reset. Then the event plays normally, so
  // onEvent fires once for it. A round start lands on the round's intro, a
  // half/match end on its studio segment. Past what has loaded: the last event.
  seek(index) {
    const n = this.events.length;
    if (!this.meta || !n) return false;
    this.invalidate();
    index = clamp(Math.round(+index || 0), 0, n);
    if (index >= n && this.terminal()) { this.skipToEnd(); this.onUpdate(this.match, this.current()); return true; }
    index = Math.min(index, n - 1);
    const e = this.events[index];
    let score = [0, 0];
    for (let i = 0; i < index; i++) if (Array.isArray(this.events[i].score)) score = this.events[i].score;
    this.score = score.slice();
    this.banner = null; this.overlay = null; this.ticker = null; this.replay = null; this.replayed = false; this.studio = null; this.halfE = null;
    this.party = null; this.confetti = []; this.cheer = [0.06, 0.06]; this.boothPulse = [-99, -99];
    this.view = { mode: 'map', id: null, why: '' }; this.cam = null; this.shotAge = 0; this.calm = 0; this.explodeT = 0; this.povBoomT = 0;
    this.seenFirst = index > 0; this.hold = 0; this.at = index;
    this.since = index > 0 ? Infinity : 0; // the bar: at the start of the event landed on
    let ri = this.rounds.findIndex(g => g.events.includes(e) || g.after.includes(e));
    if (ri < 0) ri = Math.max(0, this.rounds.findIndex(g => (this.nIdx.get(g.start.n) || 0) > index));
    const g = this.rounds[ri];
    this.ri = ri; this.R = null; this.p = 0; this.fired = 0; this.phase = 'wait';
    // A round start or buy: the round's intro. A round still being played (live): from its start, once it's in.
    if (!g || !g.end || e === g.start || (e.kind === 'buy' && g.events.includes(e))) { this.onUpdate(this.match, null); return true; }
    const R = this.R = buildRound(g, this.meta);
    if (g.after.includes(e)) {
      // The round is over: straight to the half-time or the trophy ceremony.
      this.p = R.tl.total; this.fired = R.events.length; this.replayed = true; R.after = g.after;
      this.fire(e);
      if (e.kind === 'match_end') {
        this.ri = ri + 1;
        const w = dataOf(e).winner; this.party = w === 0 || w === 1 ? w : null;
        if (this.party == null) this.enterFinal();
        else { const plan = studioPlan('matchEnd', { winner: this.party }); this.phase = 'ceremony'; this.studio = { plan, t: 0 }; this.hold = planLength(plan); }
      } else { this.phase = 'half'; this.halfE = e; const plan = studioPlan('half'); this.studio = { plan, t: 0 }; this.hold = planLength(plan); }
      this.onUpdate(this.match, this.current());
      return true;
    }
    const k = R.events.indexOf(e);
    this.fired = Math.max(0, k);
    this.p = Math.max(0, (e._p || 0) - 1e-3);
    this.phase = 'play';
    this.onUpdate(this.match, this.current());
    return true;
  }
  // Where playback is: index = events played, total = events loaded; the
  // chapters are the round starts (back/forward jump between rounds).
  getProgress() {
    const ch = this.chapters && this.chapters.len === this.events.length ? this.chapters : (this.chapters = { len: this.events.length, list: this.events.map((e, i) => e.kind === 'round_start' ? i : -1).filter(i => i >= 0) });
    const tc = this.timesC && this.timesC.len === this.events.length && this.timesC.meta === this.meta ? this.timesC : (this.timesC = { len: this.events.length, meta: this.meta, times: matchTimes(this.events, this.meta) });
    const index = this.phase === 'final' ? this.events.length : this.at;
    return { index, total: this.events.length, live: !!this.match && !this.terminal(), events: this.events, marks: progressMarks(this.events), chapters: ch.list, step: 8, ...elapsedAt(tc.times, index, this.since) };
  }

  async poll() {
    if (this.stopped) return;
    try {
      const m = await this.source.load(this.lastN);
      if (m) {
        if (m.meta && !this.meta) { this.meta = m.meta; this.mapCache = null; if (this.sCache) this.sCache.clear(); }
        const fresh = (m.events || []).filter(e => e.n > this.lastN);
        for (const e of fresh) { this.nIdx.set(e.n, this.events.length); this.events.push(e); this.lastN = Math.max(this.lastN, e.n); }
        if (fresh.length) this.rounds = groupRounds(this.events);
        this.match = { ...(this.match || {}), ...m, events: this.events, meta: this.meta };
        this.onUpdate(this.match, this.current());
        this.invalidate();
        // The first-person grid takes a while to build: do it now, not on the first POV cut.
        if (this.meta && !this.grid) this.idle(() => { if (!this.stopped) this.povGrid(); });
      }
    } catch (e) { /* torna-ho a provar */ }
    if (!this.terminal() && !this.stopped) this.schedulePoll(3000);
  }

  current() { return this.R && this.R.events[Math.max(0, this.fired - 1)] || null; }

  names() {
    const m = this.match || {}, mt = this.meta || {};
    return [0, 1].map(i => (m.names && m.names[i]) || (mt.teams && mt.teams[i] && mt.teams[i].name) || ['A', 'B'][i]);
  }
  teamColor(i) { const t = this.meta && this.meta.teams && this.meta.teams[i]; return (t && t.color) || ['#2ec4b6', '#ff6b6b'][i]; }

  resize() {
    const r = this.cv.getBoundingClientRect(), d = viewDpr();
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
    // Portrait: the map (wider than tall) would leave big empty bands above and
    // below, so it zooms in to fill the height and pans with the action.
    this.zoomMap = portrait && this.layout === 'bottom';
    this.pan = null; this.fitZ = null;
    this.mapCache = null;
    if (this.sCache) this.sCache.clear();
  }

  fitBase() { if (!this.fitR || this.fitR.box !== this.box || this.fitR.meta !== this.meta) this.fitR = { box: this.box, meta: this.meta, f: fitMap(this.meta && this.meta.size, this.box) }; return this.fitR.f; }
  fit() { return (this.zoomMap && this.fitZ) || this.fitBase(); }

  // Portrait framing: zoom the map up to the box height (at most 1.8×) and
  // follow the living players (and the planted bomb), easing towards them.
  framePortrait(R, gt, dt) {
    if (!this.zoomMap || !this.meta) { this.fitZ = null; return; }
    const base = this.fitBase(), b = this.box, size = this.meta.size || [100, 80];
    const z = clamp(b.h / (size[1] * base.s), 1, 1.8), sc = base.s * z, w = size[0] * sc, h = size[1] * sc;
    let tx = size[0] / 2, ty = size[1] / 2;
    if (R && this.phase !== 'intro') {
      let x0 = 1e9, x1 = -1e9, y0 = 1e9, y1 = -1e9;
      for (const q of Object.values(R.players)) {
        if (gt >= q.deathT) continue;
        const p = posAt(q.track, gt); x0 = Math.min(x0, p.x); x1 = Math.max(x1, p.x); y0 = Math.min(y0, p.y); y1 = Math.max(y1, p.y);
      }
      const bomb = bombState(R, gt);
      if (bomb.planted) { x0 = Math.min(x0, bomb.x); x1 = Math.max(x1, bomb.x); y0 = Math.min(y0, bomb.y); y1 = Math.max(y1, bomb.y); }
      if (x1 >= x0) { tx = (x0 + x1) / 2; ty = (y0 + y1) / 2; }
    }
    if (!this.pan || this.pan.z !== sc) this.pan = { x: tx, y: ty, z: sc };
    else { const k = 1 - Math.exp(-(dt || 0) * 1.6); this.pan.x += (tx - this.pan.x) * k; this.pan.y += (ty - this.pan.y) * k; }
    const ox = w <= b.w ? b.x + (b.w - w) / 2 : clamp(b.x + b.w / 2 - this.pan.x * sc, b.x + b.w - w, b.x);
    const oy = h <= b.h ? b.y + (b.h - h) / 2 : clamp(b.y + b.h / 2 - this.pan.y * sc, b.y + b.h - h, b.y);
    this.fitZ = { s: sc, ox, oy };
  }
  sx(x) { const f = this.fit(); return f.ox + x * f.s; }
  sy(y) { const f = this.fit(); return f.oy + y * f.s; }

  // One frame (the loop itself is in arena-view-base.js).
  onFrame(dt) {
    this.fdt = this.paused ? 0 : dt * this.speed;
    if (!this.paused) this.advance(dt * this.speed);
    try { this.draw(); } catch (e) { /* un fotograma dolent no ha d'aturar el visor */ }
  }

  // Run f when the browser is idle (or soon), off the frame loop.
  idle(f) {
    if (this.idleQ) return;
    this.idleQ = true;
    const run = () => { this.idleQ = false; f(); };
    if (typeof requestIdleCallback === 'function') requestIdleCallback(run, { timeout: 1500 }); else setTimeout(run, 50);
  }

  fire(e) {
    // Where playback is (events arrive in order; inside a round they play by time, so it only moves forward).
    const i = this.nIdx.get(e.n);
    if (i != null && i + 1 > this.at) { this.at = i + 1; this.since = 0; }
    if (e.score) this.score = e.score;
    if (['kill', 'plant', 'defuse', 'explode', 'round_end', 'buy', 'half', 'match_end'].includes(e.kind) && e.text) this.ticker = { text: e.text, t0: performance.now(), hl: !!e.hl };
    this.react(e);
    if (this.onEvent) { try { this.onEvent(e); } catch { /* res */ } }
  }

  // The crowd hears every event: a roar for the fans of whoever did it.
  react(e) {
    const r = crowdReaction(e, this.R);
    if (e.kind === 'round_start') this.cheer = this.cheer.map(v => Math.min(v, 0.2));
    if (r.team >= 0) { this.cheer[r.team] = Math.max(this.cheer[r.team], r.level); this.cheer[1 - r.team] = Math.max(this.cheer[1 - r.team], r.level * 0.15); }
    else if (r.level > 0) this.cheer = this.cheer.map(v => Math.max(v, r.level));
    if (e.kind === 'round_end') { const w = dataOf(e).winner; if (w === 0 || w === 1) this.boothPulse[w] = this.clock; }
  }

  matchWinner() { const me = [...this.events].reverse().find(e => e.kind === 'match_end'); const w = me ? dataOf(me).winner : null; return w === 0 || w === 1 ? w : null; }

  enterFinal() { this.phase = 'final'; this.studio = null; this.halfE = null; this.party = this.matchWinner(); this.overlay = { kind: 'final', t0: performance.now() }; }

  advance(dt) {
    this.since = (this.since || 0) + dt;
    this.tickStudio(dt);
    if (!this.meta) return;
    if (this.phase === 'final') return;
    if (this.phase === 'wait') {
      const g = this.rounds[this.ri];
      if (g && g.end) {
        this.R = buildRound(g, this.meta); this.p = 0; this.fired = 0; this.phase = 'intro'; this.hold = 1.7; this.banner = null; this.overlay = null;
        this.replay = null; this.replayed = false; this.view = { mode: 'map', id: null, why: '' }; this.cam = null;
        while (this.fired < this.R.events.length && this.R.events[this.fired]._p <= 0 && ['round_start', 'buy'].includes(this.R.events[this.fired].kind)) this.fire(this.R.events[this.fired++]);
        const plan = studioPlan('intro', { round: this.R.num, first: !this.seenFirst, matchPoint: matchPointTeams(this.score, this.R.num, halfOf(this.meta)), ot: this.R.num > 2 * halfOf(this.meta), pistol: this.R.pistol });
        this.seenFirst = true; this.studio = { plan, t: 0 }; this.hold = planLength(plan);
        this.onUpdate(this.match, this.current());
      } else if (this.terminal() && this.ri >= this.rounds.filter(r => r.end).length) {
        this.enterFinal();
      }
      return;
    }
    if (this.phase === 'intro') { this.hold -= dt; if (this.studio) this.studio.t += dt; if (this.hold <= 0) { this.phase = 'play'; this.studio = null; } return; }
    if (this.phase === 'play') {
      this.p += dt;
      const R = this.R;
      while (this.fired < R.events.length && R.events[this.fired]._p <= this.p && R.events[this.fired].kind !== 'round_end') this.fire(R.events[this.fired++]);
      this.direct(dt);
      if (this.p >= R.tl.total) {
        this.p = R.tl.total;
        while (this.fired < R.events.length) this.fire(R.events[this.fired++]);
        // Round over: the banner on the game view, then the crowd and the winners' booth.
        const lv = crowdReaction(R.events.find(e => e.kind === 'round_end'), R).level;
        const plan = studioPlan('roundEnd', { winner: R.end && R.end.winner, level: lv });
        this.phase = 'outro'; this.studio = { plan, t: 0 }; this.hold = planLength(plan);
        this.banner = { t0: performance.now() };
      }
      return;
    }
    if (this.phase === 'outro') {
      this.hold -= dt;
      if (this.studio) this.studio.t += dt;
      this.direct(dt);
      if (this.hold > 0) return;
      this.banner = null; this.studio = null;
      // Kill-cam replay of the round's best kill, from the killer's eyes.
      const rk = !this.replayed && pickReplay(this.R);
      this.replayed = true;
      if (rk) {
        this.phase = 'replay'; this.replay = { k: rk, g: Math.max(0, rk.t - 1.5), end: rk.t + 0.7 };
        this.cutTo('pov', rk.killer, 'replay');
        return;
      }
      // Els half/match_end poden arribar després.
      const g = this.rounds[this.ri];
      const after = (g && g.after) || [];
      this.R.after = after;
      const me = after.find(e => e.kind === 'match_end');
      if (me) {
        // Trophy ceremony on stage before the final table.
        this.fire(me); this.ri++;
        const w = dataOf(me).winner; this.party = w === 0 || w === 1 ? w : null;
        if (this.party == null) { this.enterFinal(); return; }
        const plan = studioPlan('matchEnd', { winner: this.party });
        this.phase = 'ceremony'; this.studio = { plan, t: 0 }; this.hold = planLength(plan);
        return;
      }
      const hf = after.find(e => e.kind === 'half');
      if (hf) { this.fire(hf); this.phase = 'half'; this.halfE = hf; const plan = studioPlan('half'); this.studio = { plan, t: 0 }; this.hold = planLength(plan); return; }
      this.ri++; this.phase = 'wait';
      return;
    }
    if (this.phase === 'replay') {
      const rp = this.replay;
      if (!rp) { this.phase = 'outro'; this.hold = 0; return; }
      const near = Math.abs(rp.g - rp.k.t);
      rp.g += dt * (near < 0.35 ? 0.28 : near < 0.8 ? 0.6 : 1);
      if (rp.g >= rp.end) { this.replay = null; this.phase = 'outro'; this.hold = 0; this.cutTo('map', null, ''); }
      return;
    }
    if (this.phase === 'half') { this.hold -= dt; if (this.studio) this.studio.t += dt; if (this.hold <= 0) { this.overlay = null; this.studio = null; this.halfE = null; this.ri++; this.phase = 'wait'; } }
    if (this.phase === 'ceremony') { this.hold -= dt; if (this.studio) this.studio.t += dt; if (this.hold <= 0) this.enterFinal(); }
  }

  // Director with hysteresis: shots last at least ~1 s, a kill about to happen
  // through another player's eyes cuts straight away.
  direct(dt) {
    this.shotAge += dt;
    const R = this.R;
    const want = R && this.phase === 'play' ? directorWant(R, this.p, gameTimeAt(R.tl, this.p)) : { mode: 'map' };
    const cur = this.view;
    if (want.mode === cur.mode && (want.mode === 'map' || want.id === cur.id)) { this.calm = 0; cur.why = want.why || ''; cur.k = want.k; return; }
    if (want.mode === 'map') { this.calm += dt; if (this.calm < 0.5 || this.shotAge < 1.1) return; }
    else if (!want.urgent && this.shotAge < 0.9) return;
    this.cutTo(want.mode, want.id, want.why);
    this.view.k = want.k;
  }

  cutTo(mode, id, why) {
    if (this.view.mode === mode && this.view.id === id) { this.view.why = why; return; }
    this.view = { mode, id, why }; this.shotAge = 0; this.calm = 0; this.cutT = performance.now();
    if (this.cam && this.cam.id !== id) this.cam = null;
  }

  povOn() { return !!this.R && this.view.mode === 'pov' && !!this.R.players[this.view.id] && ['play', 'outro', 'replay'].includes(this.phase); }

  // ---- dibuix ----

  draw() {
    const g = this.ctx, W = this.W, H = this.H;
    const bg = g.createRadialGradient(W / 2, H / 2, 0, W / 2, H / 2, Math.max(W, H) * 0.75);
    bg.addColorStop(0, '#131a26'); bg.addColorStop(1, '#070a10');
    g.fillStyle = bg; g.fillRect(0, 0, W, H);
    if (!this.meta) { this.centerText(this.match && this.match.status === 'failed' ? this.T.failed : this.T.pregame); return; }
    const R = this.R;
    const st = this.studio ? shotAt(this.studio.plan, this.studio.t) : null;
    const arena = st && st.s.shot !== 'tactical' && st.s.shot !== 'live' ? st : this.phase === 'final' && this.party != null ? this.finalShot() : null;
    this.onStudio = !!arena;
    if (arena) {
      this.drawStudio(arena);
      this.drawStudioLower(arena, R);
      if (this.phase !== 'final') this.studioTag(arena);
      this.drawTop(R, R ? gameTimeAt(R.tl, this.p) : 0);
      this.drawTicker();
      if (this.overlay) this.drawOverlay();
      return;
    }
    const pov = this.povOn();
    this.framePortrait(R, R ? gameTimeAt(R.tl, this.p) : 0, this.fdt);
    const clipMap = !pov && this.zoomMap;
    if (clipMap) { const b = this.box; g.save(); g.beginPath(); g.roundRect(b.x, b.y, b.w, b.h, 6 * this.u); g.clip(); }
    if (pov) {
      const rp = this.phase === 'replay' && this.replay;
      const pg = rp ? rp.g : gameTimeAt(R.tl, this.p);
      this.drawPov(R, pg, rp ? playTimeAt(R.tl, rp.g) : this.p, this.view.id, rp);
    } else this.drawMap();
    if (R) {
      const gt = gameTimeAt(R.tl, this.p);
      if (!pov) {
        this.drawUtil(R, gt, ['molotov']);
        this.drawBombGround(R, gt);
        this.drawPlayers(R, gt);
        this.drawTracers(R);
        this.drawUtil(R, gt, ['smoke', 'flash', 'he', 'flight']);
      }
      if (clipMap) g.restore();
      if (!pov && (this.phase === 'play' || this.phase === 'intro')) this.mapTag();
      this.drawCut();
      this.drawFeed(R);
      this.drawRosters(R, gt);
      this.drawTop(R, gt);
      this.drawTicker();
      if (this.phase === 'intro' && st) this.drawIntro(R, clamp(st.t / 0.3, 0, 1) * clamp((st.s.dur - st.t) / 0.3, 0, 1));
      if (this.banner) this.drawBanner(R, this.box.y + this.box.h * 0.42);
    } else {
      if (clipMap) g.restore();
      this.drawTop(null, 0);
      this.drawRosters(null, 0);
    }
    if (this.overlay) this.drawOverlay();
    if (!this.overlay && this.phase === 'wait' && !(this.terminal())) this.waitingNote();
  }

  // ---- spectator cam (pseudo-3D POV) ----

  // The 3D view bleeds across the whole stage (rosters float on top of it); the HUD stays in this.box.
  povRect() { const b = this.box, pad = 8 * this.u; return this.layout === 'side' ? { x: pad, y: b.y, w: this.W - 2 * pad, h: b.h } : b; }

  povGrid() {
    if (!this.grid || this.grid.meta !== this.meta) { this.grid = buildGrid(this.meta); this.grid.meta = this.meta; this.signs = null; }
    return this.grid;
  }

  // Painted site letters: on the wall nearest to each plant spot.
  siteSigns(G) {
    if (this.signs) return this.signs;
    const out = [];
    for (const [site, pt] of Object.entries(this.meta.plant || {})) {
      let best = null;
      for (let i = 0; i < 32; i++) {
        const a = i / 32 * Math.PI * 2, h = castRay(G, pt[0], pt[1], Math.cos(a), Math.sin(a), 30);
        if (h.hit && h.d > 4 && (!best || h.d < best.d)) best = { d: h.d, x: pt[0] + Math.cos(a) * (h.d - 0.1), y: pt[1] + Math.sin(a) * (h.d - 0.1) };
      }
      if (best) out.push({ site, x: best.x, y: best.y });
    }
    return (this.signs = out);
  }

  drawPov(R, gt, p, id, rp) {
    const g = this.ctx, vp = this.povRect(), u = this.u;
    const G = this.povGrid();
    const pl = R.players[id];
    const dead = gt >= pl.deathT;
    const pos = dead ? (pl.death || posAt(pl.track, pl.deathT)) : posAt(pl.track, gt);
    const bomb = bombState(R, gt);
    const planting = !!(bomb.planting && bomb.planting.id === id), defusing = !!(bomb.defusing && bomb.defusing.id === id);
    const dt = this.fdt || 0;
    const aim = povAim(R, id, gt);
    let cam = this.cam;
    if (!cam || cam.id !== id || cam.rp !== !!rp) cam = this.cam = { id, rp: !!rp, ang: aim, pitch: 0, bob: 0, lx: pos.x, ly: pos.y, spd: 0 };
    cam.ang += angDiff(aim, cam.ang) * Math.min(1, dt * (rp ? 4 : 7));
    const moved = Math.hypot(pos.x - cam.lx, pos.y - cam.ly); cam.lx = pos.x; cam.ly = pos.y;
    if (moved < 3) { cam.bob += moved * 1.7; if (dt > 0) cam.spd = lerp(cam.spd, clamp(moved / dt / 5, 0, 1), Math.min(1, dt * 5)); }
    cam.pitch = lerp(cam.pitch, planting || defusing ? 0.36 : 0, Math.min(1, dt * 5));
    const shots = shotsAt(R, p);
    let kick = 0, own = null;
    for (const s of shots) if (s.from === id) { const kk = 1 - s.age / 0.16; if (kk > kick) { kick = kk; own = s; } }
    const scoped = !dead && pl.weapon === 'awp' && !planting && !defusing && R.kills.some(k => k.killer === id && p >= k._p - 0.9 && p <= k._p + 0.1);
    const { x: x0, y: y0, w, h } = vp;
    const deathK = dead ? clamp((gt - pl.deathT) / 0.6, 0, 1) : 0;
    let shakeX = 0, shakeY = 0;
    if (bomb.exploded) {
      if (!this.povBoomT) this.povBoomT = performance.now();
      const a = (performance.now() - this.povBoomT) / 1000, near = clamp(1 - Math.hypot(bomb.x - pos.x, bomb.y - pos.y) / 40, 0, 1);
      if (a < 1.2) { const m = (1 - a / 1.2) * near * 9 * u; shakeX = (Math.random() - 0.5) * m; shakeY = (Math.random() - 0.5) * m; }
    } else this.povBoomT = 0;
    const C = {
      x: pos.x, y: pos.y, ang: cam.ang + (dead ? deathK * 0.25 : 0),
      eye: lerp(EYE + (cam.spd > 0.1 ? Math.sin(cam.bob) * 0.035 * cam.spd : 0), 0.35, easeOut(deathK)),
      focal: Math.max(w * 0.5, h * 0.72) * (scoped ? 2.6 : 1), cx: x0 + w / 2 + shakeX,
      horizon: y0 + h * 0.5 - cam.pitch * h + kick * h * 0.022 + shakeY,
    };
    g.save();
    g.beginPath(); g.roundRect(x0, y0, w, h, 6 * u); g.clip();
    this.povSky(C, x0, y0, w, h);
    const Z = this.povWalls(G, C, x0, y0, w, h);
    this.povSprites(R, G, C, Z, gt, p, id, shots, bomb);
    // Weapon (or the bomb in hands) and the shots fired from it.
    let muzzle = null;
    if (!dead && !scoped) muzzle = this.povWeapon(pl, cam, kick, own, planting, defusing, bomb, x0, y0, w, h);
    this.povTracers(C, shots, id, muzzle, x0, y0, w, h);
    if (scoped) this.povScope(x0, y0, w, h);
    this.povScreenFx(R, G, C, gt, p, pos, bomb, x0, y0, w, h);
    if (!dead) this.povCrosshair(x0 + w / 2, y0 + h / 2, own, scoped);
    if (dead) {
      g.fillStyle = `rgba(120,0,0,${0.35 * deathK})`; g.fillRect(x0, y0, w, h);
      const k = R.kills.find(q => q.victim === id);
      if (k && deathK > 0.3) {
        const kp = R.players[k.killer] || { handle: k.killer };
        g.fillStyle = 'rgba(0,0,0,.6)'; g.fillRect(x0, y0 + h * 0.42, w, 34 * u);
        g.fillStyle = '#fff'; g.font = `800 ${15 * u}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle';
        g.fillText(`KILLED BY ${kp.handle} · ${WEAPON[k.weapon] || k.weapon}${k.hs ? ' · HEADSHOT' : ''}`, x0 + w / 2, y0 + h * 0.42 + 17 * u, w - 20 * u);
      }
    }
    if (rp) {
      // Cinematic bars + a cool grade for the kill cam.
      g.fillStyle = 'rgba(20,40,80,.12)'; g.fillRect(x0, y0, w, h);
      g.fillStyle = '#000'; g.fillRect(x0, y0, w, h * 0.06); g.fillRect(x0, y0 + h * 0.94, w, h * 0.06);
    }
    this.povHud(R, gt, p, id, pl, bomb, planting, defusing, rp, C, x0, y0, w, h);
    g.restore();
  }

  povSky(C, x0, y0, w, h) {
    const g = this.ctx, hz = clamp(C.horizon, y0 - h, y0 + 2 * h);
    const sky = g.createLinearGradient(0, hz - h * 0.9, 0, hz);
    sky.addColorStop(0, '#4f82b8'); sky.addColorStop(0.6, '#9dbbd3'); sky.addColorStop(1, '#ecd9b2');
    g.fillStyle = sky; g.fillRect(x0, y0, w, Math.max(0, hz - y0));
    const sa = angDiff(-2.2, C.ang);
    if (Math.abs(sa) < 1.25) {
      const sx = C.cx + Math.tan(sa) * C.focal, sy = hz - h * 0.45, r = h * 0.45;
      const sun = g.createRadialGradient(sx, sy, 0, sx, sy, r);
      sun.addColorStop(0, 'rgba(255,252,235,.95)'); sun.addColorStop(0.08, 'rgba(255,240,200,.75)'); sun.addColorStop(1, 'rgba(255,230,180,0)');
      g.fillStyle = sun; g.fillRect(sx - r, sy - r, 2 * r, 2 * r);
    }
    const top = Math.max(y0, hz);
    const fl = g.createLinearGradient(0, top, 0, y0 + h);
    fl.addColorStop(0, '#c7b28c'); fl.addColorStop(0.3, '#a38e6a'); fl.addColorStop(1, '#5e5140');
    g.fillStyle = fl; g.fillRect(x0, top, w, y0 + h - top);
    // Paving lines that scroll as the player walks forward.
    const T = 1.6, fwd = C.x * Math.cos(C.ang) + C.y * Math.sin(C.ang), off = fwd / T - Math.floor(fwd / T);
    g.fillStyle = 'rgba(70,55,35,.16)';
    for (let i = 0; i < 40; i++) {
      const dd = (i + 1 - off) * T; if (dd < 0.5) continue;
      const y = hz + C.eye * C.focal / dd; if (y > y0 + h) continue; if (y < top + 1) break;
      g.fillRect(x0, y, w, Math.max(1, 0.05 * C.focal / dd));
    }
  }

  povWalls(G, C, x0, y0, w, h) {
    const g = this.ctx, cols = Math.max(80, Math.min(400, Math.round(w / (2.4 * this.dpr)))), cw = w / cols;
    if (!this.zbuf || this.zbuf.length !== cols) this.zbuf = new Float32Array(cols);
    const zb = this.zbuf, c = Math.cos(C.ang), s = Math.sin(C.ang), plen = (w / 2) / C.focal;
    const crates = [];
    for (let i = 0; i < cols; i++) {
      const camX = ((i + 0.5) / cols) * 2 - 1;
      crates.length = 0;
      const hit = castRay(G, C.x, C.y, c - s * plen * camX, s + c * plen * camX, 95, crates);
      const dist = Math.max(0.05, hit.d);
      zb[i] = dist;
      const x = x0 + i * cw, k = C.focal / dist, cwx = cw + 0.7;
      const wh = hit.k >= 0 ? G.hgt[hit.k] / 10 : 6;
      const yb = C.horizon + C.eye * k, yt = C.horizon - (wh - C.eye) * k;
      const pal = wallPal(hit.k >= 0 ? G.zones[G.zone[hit.k]] : null);
      const shade = hit.side ? 0.78 : 1, fog = clamp((dist - 10) / 75, 0, 0.7);
      if (dist > 22 || yb - yt < 28) { g.fillStyle = shadeCol(pal, shade, fog); g.fillRect(x, yt, cwx, yb - yt); }
      else {
        const BL = 0.75, rows = Math.ceil(wh / BL);
        for (let r = 0; r < rows; r++) {
          const z0 = r * BL, z1 = Math.min(wh, z0 + BL);
          const ya = C.horizon - (z1 - C.eye) * k, yb2 = C.horizon - (z0 - C.eye) * k;
          if (yb2 < y0 || ya > y0 + h) continue;
          const tx = hit.tex / 1.3 + (r & 1) * 0.5, joint = tx - Math.floor(tx) < 0.045;
          const j = hash3(Math.floor(tx), r, hit.k);
          const sh = shade * (0.88 + 0.18 * j) * (joint ? 0.7 : 1) * (r === 0 ? 0.78 : 1);
          g.fillStyle = shadeCol(pal, sh, fog); g.fillRect(x, ya, cwx, yb2 - ya);
          g.fillStyle = 'rgba(50,36,22,.25)'; g.fillRect(x, ya, cwx, Math.max(1, (yb2 - ya) * 0.07));
        }
      }
      g.fillStyle = `rgba(255,246,226,${0.3 * (1 - fog)})`; g.fillRect(x, yt, cwx, Math.max(1, 0.1 * k));
      for (let q = crates.length - 1; q >= 0; q--) {
        const cr = crates[q], k0 = C.focal / Math.max(0.15, cr.d0), k1 = C.focal / Math.max(0.15, cr.d1);
        const top0 = C.horizon - (CRATE_H - C.eye) * k0, bot0 = C.horizon + C.eye * k0, top1 = C.horizon - (CRATE_H - C.eye) * k1;
        const cf = clamp((cr.d0 - 10) / 75, 0, 0.7);
        g.fillStyle = shadeCol(CRATE_TOP, 1.05, cf); g.fillRect(x, top1, cwx, top0 - top1);
        g.fillStyle = shadeCol(CRATE_SIDE, 0.9, cf); g.fillRect(x, top0, cwx, bot0 - top0);
        g.fillStyle = 'rgba(40,25,10,.35)';
        g.fillRect(x, top0 + (bot0 - top0) * 0.48, cwx, Math.max(1, 0.06 * k0)); g.fillRect(x, top0, cwx, Math.max(1, 0.07 * k0));
      }
    }
    return { zb, cols, cw, x0 };
  }

  // Clip path of the screen columns where a sprite at `depth` is in front of the walls.
  clipRuns(Z, xa, xb, depth) {
    const g = this.ctx;
    const i0 = Math.max(0, Math.floor((xa - Z.x0) / Z.cw)), i1 = Math.min(Z.cols - 1, Math.floor((xb - Z.x0) / Z.cw));
    if (i1 < i0) return false;
    g.beginPath(); let run = -1, any = false;
    for (let i = i0; i <= i1 + 1; i++) {
      const vis = i <= i1 && Z.zb[i] > depth;
      if (vis && run < 0) run = i;
      if (!vis && run >= 0) { g.rect(Z.x0 + run * Z.cw, -1e4, (i - run) * Z.cw, 2e4); run = -1; any = true; }
    }
    return any;
  }

  povSprites(R, G, C, Z, gt, p, id, shots, bomb) {
    const g = this.ctx;
    const items = [];
    for (const q of Object.values(R.players)) {
      if (q.id === id) continue;
      const dq = gt >= q.deathT, qp = dq ? (q.death || posAt(q.track, q.deathT)) : posAt(q.track, gt);
      items.push({ kind: 'player', q, x: qp.x, y: qp.y, dead: dq });
    }
    for (const uu of R.utils) { const st = utilState(uu, gt, p); if (st) items.push({ kind: 'util', u: uu, st, x: st.x, y: st.y }); }
    if (bomb.planted || bomb.dropped) { const b = bomb.dropped || bomb; items.push({ kind: 'bomb', x: b.x, y: b.y, planted: bomb.planted, b: bomb }); }
    for (const s of this.siteSigns(G)) items.push({ kind: 'sign', site: s.site, x: s.x, y: s.y });
    const vis = [];
    for (const it of items) { it.pr = projectPoint(C, it.x, it.y, 0); if (!it.pr.behind || (it.kind === 'util' && it.st.kind === 'smoke')) vis.push(it); }
    vis.sort((a, b) => b.pr.depth - a.pr.depth);
    for (const it of vis) {
      if (it.kind === 'player') this.povFigure(R, C, Z, it, gt, shots);
      else if (it.kind === 'util') this.povUtil(C, Z, it);
      else if (it.kind === 'bomb') this.povBomb(C, Z, it);
      else if (it.kind === 'sign') {
        const pr = projectPoint(C, it.x, it.y, 2.6); if (pr.behind || pr.depth > 70) continue;
        const k = C.focal / pr.depth, fs = 2.2 * k;
        if (!this.clipRuns(Z, pr.x - fs, pr.x + fs, pr.depth - 0.6)) continue;
        g.save(); g.clip();
        g.globalAlpha = clamp(1 - pr.depth / 80, 0.3, 0.9);
        g.strokeStyle = 'rgba(150,30,20,.9)'; g.lineWidth = Math.max(1, 0.14 * k);
        g.beginPath(); g.arc(pr.x, pr.y, 1.35 * k, 0, Math.PI * 2); g.stroke();
        g.fillStyle = 'rgba(165,35,22,.92)'; g.font = `900 ${fs}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle';
        g.fillText(it.site, pr.x, pr.y + 0.08 * k);
        g.restore();
      }
    }
  }

  povFigure(R, C, Z, it, gt, shots) {
    const g = this.ctx, q = it.q, pr = it.pr, u = this.u;
    if (pr.depth > 75 || pr.depth < 0.9) return;
    const k = C.focal / pr.depth, H = BODY_H * k, foot = C.horizon + C.eye * k, x = pr.x;
    if (x < Z.x0 - H || x > Z.x0 + Z.cols * Z.cw + H) return;
    const col = this.teamColor(q.team);
    const ang = povAim(R, q.id, Math.min(gt, q.deathT));
    const lat = Math.cos(ang) * -Math.sin(C.ang) + Math.sin(ang) * Math.cos(C.ang);
    const ahead = posAt(q.track, gt + 0.3), cur = posAt(q.track, gt);
    const moving = !it.dead && Math.hypot(ahead.x - cur.x, ahead.y - cur.y) > 0.2;
    const walk = moving ? Math.sin(gt * 9 + q.id.length * 3 + (q.team ? 1.3 : 0)) : 0;
    const fall = it.dead ? clamp((gt - q.deathT) / 0.5, 0, 1) : 0;
    const fallDir = hash3(q.id.charCodeAt(0), q.id.length, 5) < 0.5 ? -1 : 1;
    // X-ray glow through walls, like a GOTV spectator.
    if (!it.dead && pr.depth < 45) { g.save(); g.globalAlpha = 0.6; this.figure(x, foot, H, q, lat, walk, 0, 0, col, true); g.restore(); }
    let tip = null;
    if (this.clipRuns(Z, x - H * 0.7, x + H * 0.7, pr.depth)) {
      g.save(); g.clip();
      if (it.dead && fall >= 1) g.globalAlpha = 0.85;
      tip = this.figure(x, foot, H, q, lat, walk, fall, fallDir, col, false);
      g.restore();
    }
    // Muzzle flash, head-shot mist, name tag.
    for (const s of shots) if (s.from === q.id && s.age < 0.07 && tip) this.flashStar(tip.x, tip.y, Math.max(6 * u, 0.5 * k), s.k.n);
    if (it.dead) {
      const kk = R.kills.find(z => z.victim === q.id);
      const age = gt - q.deathT;
      if (kk && age < 0.5) {
        const r = (0.25 + age) * k, a = 1 - age / 0.5;
        const mist = g.createRadialGradient(x, foot - (kk.hs ? 1.6 : 1.2) * k, 0, x, foot - (kk.hs ? 1.6 : 1.2) * k, r);
        mist.addColorStop(0, `rgba(190,20,20,${0.8 * a})`); mist.addColorStop(1, 'rgba(120,0,0,0)');
        g.fillStyle = mist; g.beginPath(); g.arc(x, foot - (kk.hs ? 1.6 : 1.2) * k, r, 0, Math.PI * 2); g.fill();
      }
      return;
    }
    if (pr.depth < 40) {
      const fs = clamp(0.32 * k, 9 * u, 13 * u), ty = foot - H - 0.25 * k - 2 * u;
      g.font = `800 ${fs}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'bottom';
      g.lineWidth = 3 * this.dpr; g.strokeStyle = 'rgba(0,0,0,.75)'; g.strokeText(q.handle, x, ty);
      g.fillStyle = col; g.fillText(q.handle, x, ty);
    }
  }

  // A procedural player model: jointed legs and boots, body armour with the
  // team stripe, arms holding the rifle, head with helmet (CT) or cap (T), and
  // a dark outline so it reads against the sandstone. Through walls (xray) the
  // same silhouette is a flat team-colour shape with a bright edge. Returns the gun tip.
  figure(x, foot, H, q, lat, walk, fall, fallDir, col, xray) {
    const g = this.ctx, s = H / BODY_H;
    g.save(); g.translate(x, foot);
    if (fall > 0) g.rotate(fallDir * easeOut(fall) * Math.PI * 0.48);
    const T = q.side === 'T';
    const cloth = xray ? col : T ? '#6f5c40' : '#2d3d56', dark = xray ? col : T ? '#4b3e2b' : '#1d283a';
    const vest = T ? '#57482f' : '#22324a', boot = '#16171a', skin = '#c49a74', line = 'rgba(8,8,10,.6)';
    const side = Math.abs(lat) > 0.3, dir = Math.sign(lat) || 1, ow = Math.max(1, 0.035 * s);
    g.lineCap = 'round'; g.lineJoin = 'round';
    if (xray) { g.fillStyle = rgba(col, 0.42); g.strokeStyle = rgba(col, 0.95); }
    // Legs: hip, knee, foot; the stride shows sideways, a lifted foot head-on.
    const legs = [];
    for (const k of [-1, 1]) {
      const st = walk * k, hx = k * 0.1 * s;
      const fxp = side ? hx + dir * st * 0.24 * s : hx * 1.15, fy = -(side ? Math.max(0, -st) * 0.06 : Math.max(0, st) * 0.1) * s;
      const kx = (hx + fxp) / 2 + (side ? dir * (0.05 + 0.07 * Math.max(0, st)) * s : 0), ky = -0.47 * s - (side ? 0 : Math.max(0, st) * 0.05 * s);
      legs.push([hx, -0.88 * s, kx, ky, fxp, fy]);
    }
    for (const [hx, hy, kx, ky, fx2, fy] of legs) {
      if (xray) { g.lineWidth = 0.17 * s; g.beginPath(); g.moveTo(hx, hy); g.lineTo(kx, ky); g.lineTo(fx2, fy - 0.04 * s); g.stroke(); continue; }
      g.strokeStyle = line; g.lineWidth = 0.19 * s + 2 * ow; g.beginPath(); g.moveTo(hx, hy); g.lineTo(kx, ky); g.lineTo(fx2, fy - 0.04 * s); g.stroke();
      g.strokeStyle = dark; g.lineWidth = 0.19 * s; g.beginPath(); g.moveTo(hx, hy); g.lineTo(kx, ky); g.stroke();
      g.lineWidth = 0.15 * s; g.beginPath(); g.moveTo(kx, ky); g.lineTo(fx2, fy - 0.04 * s); g.stroke();
      g.fillStyle = boot; g.beginPath(); g.ellipse(fx2 + (side ? dir * 0.05 * s : 0), fy - 0.03 * s, (side ? 0.13 : 0.09) * s, 0.055 * s, 0, 0, Math.PI * 2); g.fill();
    }
    // Torso: wider at the shoulders, armour vest, team stripe across the chest.
    const torso = () => { g.beginPath(); g.moveTo(-0.27 * s, -1.47 * s); g.lineTo(0.27 * s, -1.47 * s); g.quadraticCurveTo(0.3 * s, -1.2 * s, 0.21 * s, -0.84 * s); g.lineTo(-0.21 * s, -0.84 * s); g.quadraticCurveTo(-0.3 * s, -1.2 * s, -0.27 * s, -1.47 * s); g.closePath(); };
    torso();
    if (xray) { g.fill(); g.stroke(); }
    else {
      g.fillStyle = cloth; g.fill(); g.strokeStyle = line; g.lineWidth = ow * 1.5; g.stroke();
      g.fillStyle = vest; g.beginPath(); g.roundRect(-0.22 * s, -1.4 * s, 0.44 * s, 0.4 * s, 0.06 * s); g.fill();
      g.fillStyle = 'rgba(255,255,255,.08)'; g.fillRect(-0.22 * s, -1.4 * s, 0.44 * s, 0.05 * s);
      g.fillStyle = col; g.fillRect(-0.25 * s, -1.21 * s, 0.5 * s, 0.07 * s);
      g.fillStyle = dark; g.fillRect(-0.21 * s, -0.9 * s, 0.42 * s, 0.06 * s); // belt
    }
    // Head: neck, face, then the helmet (CT) or a cap and balaclava (T).
    if (!xray) { g.fillStyle = skin; g.fillRect(-0.05 * s, -1.55 * s, 0.1 * s, 0.1 * s); }
    g.beginPath(); g.arc(0, -1.63 * s, 0.135 * s, 0, Math.PI * 2);
    if (xray) { g.fill(); g.stroke(); }
    else {
      g.fillStyle = T ? '#2a2622' : skin; g.fill(); g.strokeStyle = line; g.lineWidth = ow; g.stroke();
      if (T) { g.fillStyle = skin; g.fillRect(side ? (dir > 0 ? 0.02 : -0.12) * s : -0.08 * s, -1.66 * s, (side ? 0.1 : 0.16) * s, 0.05 * s); }
      g.fillStyle = T ? '#1c1c1c' : '#3a4a62';
      g.beginPath(); g.arc(0, -1.65 * s, 0.155 * s, Math.PI * 1.02, Math.PI * 1.98); g.closePath(); g.fill();
      g.fillRect(-0.16 * s + (side ? dir * 0.03 * s : 0), -1.665 * s, 0.32 * s, 0.035 * s); // brim / rim
      if (!T) { g.fillStyle = 'rgba(255,255,255,.18)'; g.beginPath(); g.arc(-0.04 * s, -1.72 * s, 0.05 * s, 0, Math.PI * 2); g.fill(); }
    }
    // Gun: sideways when they face across the screen, foreshortened when aiming at us.
    const L = 0.62 * s * lat, gy = -1.16 * s;
    let tip;
    const gunCol = xray ? col : '#1a1a1c';
    if (side) {
      g.strokeStyle = gunCol; g.lineWidth = Math.max(1.5, 0.09 * s); g.beginPath(); g.moveTo(-0.08 * s * dir, gy); g.lineTo(L, gy - 0.03 * s); g.stroke();
      if (!xray) {
        g.lineWidth = Math.max(1.5, 0.13 * s); g.beginPath(); g.moveTo(-0.14 * s * dir, gy + 0.02 * s); g.lineTo(0.12 * s * dir, gy); g.stroke(); // receiver
        g.fillStyle = gunCol; g.fillRect(dir > 0 ? 0.04 * s : -0.1 * s, gy, 0.06 * s, 0.14 * s); // magazine
      }
      g.strokeStyle = xray ? col : cloth; g.lineWidth = Math.max(1.5, 0.1 * s);
      g.beginPath(); g.moveTo(-0.2 * dir * s, -1.38 * s); g.lineTo(0, gy + 0.05 * s); g.lineTo(L * 0.55, gy + 0.02 * s); g.stroke();
      if (!xray) { g.fillStyle = '#2a2420'; g.beginPath(); g.arc(L * 0.55, gy + 0.02 * s, 0.05 * s, 0, Math.PI * 2); g.arc(0, gy + 0.05 * s, 0.05 * s, 0, Math.PI * 2); g.fill(); }
      tip = { x: L, y: gy - 0.03 * s };
    } else {
      g.fillStyle = gunCol; g.beginPath(); g.arc(L * 0.3, gy, 0.08 * s, 0, Math.PI * 2); g.fill();
      if (!xray) { g.fillStyle = '#000'; g.beginPath(); g.arc(L * 0.3, gy, 0.035 * s, 0, Math.PI * 2); g.fill(); }
      g.strokeStyle = xray ? col : cloth; g.lineWidth = Math.max(1.5, 0.11 * s);
      g.beginPath(); g.moveTo(-0.24 * s, -1.4 * s); g.lineTo(L * 0.3 - 0.06 * s, gy + 0.03 * s); g.moveTo(0.24 * s, -1.4 * s); g.lineTo(L * 0.3 + 0.06 * s, gy + 0.03 * s); g.stroke();
      if (!xray) { g.fillStyle = '#2a2420'; g.beginPath(); g.arc(L * 0.3 - 0.07 * s, gy + 0.04 * s, 0.05 * s, 0, Math.PI * 2); g.arc(L * 0.3 + 0.07 * s, gy + 0.04 * s, 0.05 * s, 0, Math.PI * 2); g.fill(); }
      tip = { x: L * 0.3, y: gy };
    }
    g.restore();
    return { x: x + tip.x, y: foot + tip.y };
  }

  flashStar(x, y, r, seed) {
    const g = this.ctx, R2 = rng((seed || 1) * 31 + Math.floor(performance.now() / 40));
    const gl = g.createRadialGradient(x, y, 0, x, y, r * 1.6);
    gl.addColorStop(0, 'rgba(255,255,230,1)'); gl.addColorStop(0.3, 'rgba(255,210,110,.85)'); gl.addColorStop(1, 'rgba(255,140,40,0)');
    g.fillStyle = gl; g.beginPath();
    const n = 7, a0 = R2() * Math.PI;
    for (let i = 0; i < n * 2; i++) { const a = a0 + i * Math.PI / n, rr = (i & 1 ? 0.35 : 1 + R2() * 0.6) * r; g.lineTo(x + Math.cos(a) * rr, y + Math.sin(a) * rr); }
    g.closePath(); g.fill();
  }

  povUtil(C, Z, it) {
    const g = this.ctx, st = it.st, u0 = it.u;
    if (st.kind === 'smoke') {
      const R2 = rng(u0.n * 7 + 3);
      const puffs = [];
      for (let i = 0; i < 9; i++) {
        const a = R2() * Math.PI * 2, dd = i === 0 ? 0 : st.r * (0.2 + 0.45 * R2()), rr = st.r * (0.45 + 0.3 * R2()), hz = 0.9 + R2() * 1.8;
        const wob = Math.sin(performance.now() / 1500 + i) * 0.15;
        const pr = projectPoint(C, st.x + Math.cos(a) * dd + wob, st.y + Math.sin(a) * dd, hz);
        if (!pr.behind) puffs.push({ pr, rr });
      }
      puffs.sort((a, b) => b.pr.depth - a.pr.depth);
      for (const { pr, rr } of puffs) {
        const r = rr * C.focal / pr.depth; if (r > 6000) continue;
        if (!this.clipRuns(Z, pr.x - r, pr.x + r, pr.depth - rr)) continue;
        g.save(); g.clip();
        const gr = g.createRadialGradient(pr.x, pr.y, 0, pr.x, pr.y, r);
        gr.addColorStop(0, `rgba(214,218,224,${0.75 * st.a})`); gr.addColorStop(0.55, `rgba(188,194,204,${0.55 * st.a})`); gr.addColorStop(1, 'rgba(180,186,196,0)');
        g.fillStyle = gr; g.beginPath(); g.arc(pr.x, pr.y, r, 0, Math.PI * 2); g.fill();
        g.restore();
      }
      return;
    }
    const pr = it.pr, k = C.focal / pr.depth;
    if (pr.depth > 80) return;
    if (st.kind === 'molotov') {
      const r = st.r * k, fy = C.horizon + C.eye * k;
      if (!this.clipRuns(Z, pr.x - r * 1.2, pr.x + r * 1.2, pr.depth - st.r)) return;
      g.save(); g.clip();
      const gl = g.createRadialGradient(pr.x, fy, 0, pr.x, fy, r * 1.3);
      gl.addColorStop(0, `rgba(255,160,60,${0.55 * st.a})`); gl.addColorStop(1, 'rgba(255,80,20,0)');
      g.fillStyle = gl; g.fillRect(pr.x - r * 1.3, fy - r * 1.3, r * 2.6, r * 2.6);
      const R2 = rng(u0.n * 13 + Math.floor(performance.now() / 80));
      for (let i = 0; i < 16; i++) {
        const fx = st.x + (R2() - 0.5) * st.r * 1.6, fz = st.y + (R2() - 0.5) * st.r * 1.6;
        const fp = projectPoint(C, fx, fz, 0); if (fp.behind) continue;
        const kk = C.focal / fp.depth, fh = (0.5 + R2() * 0.8) * kk, fw = (0.25 + R2() * 0.25) * kk;
        const fg = g.createLinearGradient(0, fp.y, 0, fp.y - fh);
        fg.addColorStop(0, `rgba(255,220,120,${0.9 * st.a})`); fg.addColorStop(0.5, `rgba(255,120,30,${0.75 * st.a})`); fg.addColorStop(1, 'rgba(200,40,10,0)');
        g.fillStyle = fg; g.beginPath(); g.moveTo(fp.x - fw / 2, fp.y); g.quadraticCurveTo(fp.x - fw * 0.4, fp.y - fh * 0.6, fp.x, fp.y - fh); g.quadraticCurveTo(fp.x + fw * 0.4, fp.y - fh * 0.6, fp.x + fw / 2, fp.y); g.fill();
      }
      g.restore();
      return;
    }
    if (st.kind === 'flight') {
      const fp = projectPoint(C, st.x, st.y, 1.4 + st.h * 2.5); if (fp.behind) return;
      const r = Math.max(2 * this.dpr, 0.12 * C.focal / fp.depth);
      g.fillStyle = u0.type === 'molotov' ? '#ff8a3d' : u0.type === 'flash' ? '#f4f4f4' : u0.type === 'he' ? '#6d8f4a' : '#c9ced6';
      g.beginPath(); g.arc(fp.x, fp.y, r, 0, Math.PI * 2); g.fill();
      return;
    }
    // flash / HE burst
    const bp = projectPoint(C, st.x, st.y, 1.2); if (bp.behind) return;
    const r = st.r * 0.8 * C.focal / bp.depth;
    if (!this.clipRuns(Z, bp.x - r, bp.x + r, bp.depth - 2)) return;
    g.save(); g.clip();
    const col = st.kind === 'he' ? '255,170,80' : '255,255,255';
    const gr = g.createRadialGradient(bp.x, bp.y, 0, bp.x, bp.y, r);
    gr.addColorStop(0, `rgba(${col},${st.a})`); gr.addColorStop(1, `rgba(${col},0)`);
    g.fillStyle = gr; g.beginPath(); g.arc(bp.x, bp.y, r, 0, Math.PI * 2); g.fill();
    g.restore();
  }

  povBomb(C, Z, it) {
    const g = this.ctx, pr = it.pr; if (pr.depth > 60) return;
    const k = C.focal / pr.depth, y = C.horizon + C.eye * k, bw = 0.45 * k, bh = 0.2 * k;
    if (it.b.exploded) {
      const a = this.povBoomT ? (performance.now() - this.povBoomT) / 1000 : 0, kk = clamp(a / 1.4, 0, 1), r = (2 + 14 * easeOut(kk)) * k;
      const gr = g.createRadialGradient(pr.x, y - r * 0.4, 0, pr.x, y - r * 0.4, r);
      gr.addColorStop(0, `rgba(255,245,200,${1 - kk})`); gr.addColorStop(0.35, `rgba(255,130,30,${0.85 * (1 - kk)})`); gr.addColorStop(1, 'rgba(90,20,0,0)');
      g.fillStyle = gr; g.beginPath(); g.arc(pr.x, y - r * 0.4, r, 0, Math.PI * 2); g.fill();
      return;
    }
    if (!this.clipRuns(Z, pr.x - bw, pr.x + bw, pr.depth)) return;
    g.save(); g.clip();
    g.fillStyle = '#4d5232'; g.fillRect(pr.x - bw / 2, y - bh, bw, bh);
    g.fillStyle = '#23251a'; g.fillRect(pr.x - bw * 0.3, y - bh * 0.85, bw * 0.4, bh * 0.5);
    if (it.planted && !it.b.defused) {
      const rate = it.b.left < 10 ? 4 : it.b.left < 20 ? 2 : 1, on = Math.sin(performance.now() / 1000 * Math.PI * 2 * rate) > 0;
      const lx = pr.x + bw * 0.3, ly = y - bh * 0.7, lr = Math.max(2, 0.05 * k) * (on ? 1 : 0.6);
      const gl = g.createRadialGradient(lx, ly, 0, lx, ly, lr * 6);
      gl.addColorStop(0, on ? 'rgba(255,60,40,.95)' : 'rgba(120,20,10,.6)'); gl.addColorStop(1, 'rgba(255,40,20,0)');
      g.fillStyle = gl; g.beginPath(); g.arc(lx, ly, lr * 6, 0, Math.PI * 2); g.fill();
    }
    g.restore();
  }

  povTracers(C, shots, id, muzzle, x0, y0, w, h) {
    const g = this.ctx, d = this.dpr;
    for (const s of shots) {
      const a = 1 - s.age / 0.16;
      // aim a little off for the missed shots of a burst
      const dx = s.bx - s.ax, dy = s.by - s.ay, L = Math.hypot(dx, dy) || 1;
      const bx = s.bx + (-dy / L) * s.miss, by = s.by + (dx / L) * s.miss;
      let A = null;
      if (s.from === id) A = muzzle || { x: x0 + w / 2, y: y0 + h * 0.6 };
      else { const pa = projectPoint(C, s.ax, s.ay, 1.38); if (!pa.behind) A = pa; }
      let B = projectPoint(C, bx, by, s.hs ? 1.62 : 1.25);
      if (B.behind) {
        // clip the far end at the near plane
        const da = (s.ax - C.x) * Math.cos(C.ang) + (s.ay - C.y) * Math.sin(C.ang), db = B.depth;
        if (da <= 0.2) continue;
        const t = (da - 0.2) / (da - db);
        B = projectPoint(C, s.ax + (bx - s.ax) * t, s.ay + (by - s.ay) * t, 1.3);
        if (B.behind) continue;
      }
      if (!A) continue;
      const gr = g.createLinearGradient(A.x, A.y, B.x, B.y);
      gr.addColorStop(0, `rgba(255,240,180,${0.1 * a})`); gr.addColorStop(0.5, `rgba(255,226,140,${0.85 * a})`); gr.addColorStop(1, `rgba(255,250,220,${a})`);
      g.lineCap = 'round';
      g.strokeStyle = `rgba(255,200,90,${0.25 * a})`; g.lineWidth = 6 * d; g.beginPath(); g.moveTo(A.x, A.y); g.lineTo(B.x, B.y); g.stroke();
      g.strokeStyle = gr; g.lineWidth = 2 * d; g.beginPath(); g.moveTo(A.x, A.y); g.lineTo(B.x, B.y); g.stroke();
      if (s.kill) { g.fillStyle = `rgba(255,255,255,${a})`; g.beginPath(); g.arc(B.x, B.y, 3 * d, 0, Math.PI * 2); g.fill(); }
    }
  }

  // First-person weapon (or the C4 in hands while planting / defusing). Returns the muzzle point.
  povWeapon(pl, cam, kick, own, planting, defusing, bomb, x0, y0, w, h) {
    const g = this.ctx, T = pl.side === 'T';
    const sleeve = T ? '#7a6446' : '#33476a', glove = T ? '#3b3226' : '#1b222d';
    const S = Math.min(h, w * 0.75) / 235, bobX = Math.sin(cam.bob) * 7 * S * cam.spd, bobY = Math.abs(Math.cos(cam.bob)) * 6 * S * cam.spd;
    if (planting || defusing) {
      const prog = planting ? bomb.planting.prog : bomb.defusing.prog;
      g.save(); g.translate(x0 + w / 2, y0 + h * 0.86); g.scale(S, S * (defusing ? 0.62 : 1));
      g.fillStyle = '#4d5232'; g.beginPath(); g.roundRect(-58, -34, 116, 62, 6); g.fill();
      g.fillStyle = '#2e3220'; g.fillRect(-58, -6, 116, 6);
      for (let i = 0; i < 3; i++) { g.fillStyle = '#7b6a42'; g.fillRect(-52 + i * 36, -34, 22, 62); }
      g.fillStyle = '#151711'; g.fillRect(-26, -26, 52, 40);
      g.fillStyle = '#0b2410'; g.fillRect(-20, -22, 40, 12);
      g.fillStyle = '#5dff7a'; g.font = `800 10px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle';
      g.fillText(planting ? '7355608'.slice(0, Math.max(1, Math.ceil(prog * 7))) : '*******', 0, -16);
      for (let r = 0; r < 3; r++) for (let c = 0; c < 3; c++) { g.fillStyle = '#3a3d33'; g.fillRect(-16 + c * 12, -4 + r * 6, 9, 4); }
      const on = Math.sin(performance.now() / 120) > 0;
      g.fillStyle = on ? '#ff3b2e' : '#5a1510'; g.beginPath(); g.arc(44, -24, 4, 0, Math.PI * 2); g.fill();
      g.restore();
      // hands
      g.save(); g.translate(x0 + w / 2, y0 + h); g.scale(S, S);
      g.fillStyle = sleeve;
      g.beginPath(); g.moveTo(-150, 20); g.lineTo(-70, -40); g.lineTo(-48, -22); g.lineTo(-110, 30); g.fill();
      g.beginPath(); g.moveTo(150, 20); g.lineTo(70, -40 - (defusing ? Math.sin(performance.now() / 90) * 4 : 0)); g.lineTo(48, -22); g.lineTo(110, 30); g.fill();
      g.fillStyle = glove;
      g.beginPath(); g.ellipse(-60, -32, 15, 11, -0.5, 0, Math.PI * 2); g.fill();
      g.beginPath(); g.ellipse(60, -32, 15, 11, 0.5, 0, Math.PI * 2); g.fill();
      if (defusing) { g.strokeStyle = '#c0392b'; g.lineWidth = 4; g.beginPath(); g.moveTo(52, -38); g.lineTo(28, -52); g.moveTo(56, -30); g.lineTo(30, -42); g.stroke(); }
      g.restore();
      return null;
    }
    const cls = weaponClass(pl.weapon), wid = pl.weapon;
    const pistol = cls === 'pistol';
    // At rest the barrel points at the crosshair; bob and recoil only nudge it.
    const pose = gunPose(w, h, wid, this.box.x + this.box.w - x0), fx = pose.fx;
    const ax = x0 + pose.ax + bobX + kick * 10 * S, ay = y0 + pose.ay + bobY + kick * 12 * S;
    const rot = pose.rot + kick * (cls === 'awp' ? 0.16 : 0.07);
    g.save(); g.translate(ax, ay); g.rotate(rot); g.scale(S * fx, S);
    const tip = pose.tip;
    // The model is drawn in gun space (barrel towards -x, the muzzle at `tip`):
    // shaded metal and polymer, the magazine, sights and rails, gloved hands
    // gripping it and the sleeves running out of the frame.
    if (pistol) drawPistolFP(g, wid, sleeve, glove);
    else drawLongGunFP(g, cls, wid, sleeve, glove);
    g.restore();
    const c = Math.cos(rot), s = Math.sin(rot);
    const mx = ax + (tip[0] * fx * c - tip[1] * s) * S, my = ay + (tip[0] * fx * s + tip[1] * c) * S;
    if (own && own.age < 0.07) this.flashStar(mx, my, (cls === 'awp' ? 30 : 22) * S, own.k.n);
    return { x: mx, y: my };
  }

  povScope(x0, y0, w, h) {
    const g = this.ctx, cx = x0 + w / 2, cy = y0 + h / 2, r = Math.min(w, h) * 0.47;
    g.fillStyle = '#000'; g.beginPath(); g.rect(x0, y0, w, h); g.arc(cx, cy, r, 0, Math.PI * 2, true); g.fill('evenodd');
    const vg = g.createRadialGradient(cx, cy, r * 0.7, cx, cy, r);
    vg.addColorStop(0, 'rgba(0,0,0,0)'); vg.addColorStop(1, 'rgba(0,0,0,.7)');
    g.fillStyle = vg; g.beginPath(); g.arc(cx, cy, r, 0, Math.PI * 2); g.fill();
    g.strokeStyle = '#000'; g.lineWidth = 1.5 * this.dpr;
    g.beginPath(); g.moveTo(x0, cy); g.lineTo(x0 + w, cy); g.moveTo(cx, y0); g.lineTo(cx, y0 + h); g.stroke();
  }

  povCrosshair(cx, cy, own, scoped) {
    const g = this.ctx, u = this.u;
    if (!scoped) {
      const gap = 4 * u, len = 7 * u, th = 2 * u;
      for (const [dx, dy] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
        const x = dx ? cx + dx * (gap + len / 2) : cx, y = dy ? cy + dy * (gap + len / 2) : cy;
        const ww = dx ? len : th, hh = dx ? th : len;
        g.fillStyle = 'rgba(0,0,0,.7)'; g.fillRect(x - ww / 2 - u, y - hh / 2 - u, ww + 2 * u, hh + 2 * u);
        g.fillStyle = '#4dff6a'; g.fillRect(x - ww / 2, y - hh / 2, ww, hh);
      }
    }
    if (own && own.kill && own.age < 0.16) {
      const a = 1 - own.age / 0.16, r0 = 6 * u, r1 = 13 * u;
      g.strokeStyle = own.hs ? `rgba(255,70,60,${a})` : `rgba(255,255,255,${a})`; g.lineWidth = 2.2 * u;
      g.beginPath();
      for (const [dx, dy] of [[1, 1], [-1, 1], [1, -1], [-1, -1]]) { g.moveTo(cx + dx * r0, cy + dy * r0); g.lineTo(cx + dx * r1, cy + dy * r1); }
      g.stroke();
    }
  }

  // Smoke you stand in, flashbangs that catch your eyes, the bomb going off.
  povScreenFx(R, G, C, gt, p, pos, bomb, x0, y0, w, h) {
    const g = this.ctx;
    let white = 0, grey = 0;
    for (const uu of R.utils) {
      if (uu.type === 'flash') {
        const age = gt - uu.t; if (age < 0 || age > 2) continue;
        const dist = Math.hypot(uu.x - pos.x, uu.y - pos.y); if (dist > 24 || !sees(G, pos.x, pos.y, uu.x, uu.y)) continue;
        const facing = Math.cos(angDiff(Math.atan2(uu.y - pos.y, uu.x - pos.x), C.ang));
        white = Math.max(white, (1 - age / 2) * clamp(1.25 - dist / 24, 0, 1) * (0.3 + 0.7 * Math.max(0, facing)));
      } else if (uu.type === 'smoke') {
        const st = utilState(uu, gt, p); if (!st || st.kind !== 'smoke') continue;
        const dist = Math.hypot(st.x - pos.x, st.y - pos.y);
        if (dist < st.r) grey = Math.max(grey, st.a * clamp(1.2 - dist / st.r, 0, 1));
      }
    }
    if (grey > 0) { g.fillStyle = `rgba(196,200,208,${clamp(grey, 0, 0.92)})`; g.fillRect(x0, y0, w, h); }
    if (bomb.exploded && this.povBoomT) {
      const a = (performance.now() - this.povBoomT) / 1000, near = clamp(1 - Math.hypot(bomb.x - pos.x, bomb.y - pos.y) / 45, 0, 1);
      if (a < 1.6) { g.fillStyle = `rgba(255,170,90,${0.8 * near * (1 - a / 1.6)})`; g.fillRect(x0, y0, w, h); }
    }
    if (white > 0.02) { g.fillStyle = `rgba(255,255,255,${clamp(white * 1.1, 0, 0.97)})`; g.fillRect(x0, y0, w, h); }
  }

  povHud(R, gt, p, id, pl, bomb, planting, defusing, rp, C, x0, y0, w, h) {
    const g = this.ctx, u = this.u, col = this.teamColor(pl.team);
    const st = hudStats(R, id, gt);
    const bot = y0 + h - (rp ? h * 0.06 : 0) - 10 * u;
    const top = y0 + (rp ? h * 0.06 : 0) + 10 * u;
    const pill = (x, y, ww, hh, fill) => { g.fillStyle = fill; g.beginPath(); g.roundRect(x, y, ww, hh, hh / 2); g.fill(); };
    g.textBaseline = 'middle';
    // Corner tag: POV / REPLAY
    const why = this.view.why, k = this.view.k;
    const n = aliveAt(R, gt);
    const tagSub = rp ? 'KILL CAM' : why === 'kill' ? (k && k.first ? 'ENTRY' : 'DUEL') : why === 'clutch' ? `CLUTCH 1v${n[1 - pl.team]}` : why === 'plant' ? 'PLANT' : why === 'defuse' ? 'DEFUSE' : '';
    g.font = `900 ${11.5 * u}px ${FONT}`;
    const label = rp ? 'REPLAY' : `POV: ${pl.handle}`;
    const lw = g.measureText(label).width;
    g.font = `800 ${9.5 * u}px ${FONT}`;
    const sw2 = tagSub ? g.measureText(tagSub).width + 14 * u : 0;
    const tw = lw + sw2 + 34 * u;
    pill(x0 + 10 * u, top, tw, 24 * u, rp ? 'rgba(170,20,20,.9)' : 'rgba(6,9,14,.82)');
    const blink = Math.sin(performance.now() / 260) > -0.2;
    g.fillStyle = rp ? (blink ? '#fff' : 'rgba(255,255,255,.3)') : col; g.beginPath(); g.arc(x0 + 22 * u, top + 12 * u, 4.5 * u, 0, Math.PI * 2); g.fill();
    g.fillStyle = '#fff'; g.font = `900 ${11.5 * u}px ${FONT}`; g.textAlign = 'left'; g.fillText(label, x0 + 32 * u, top + 12.5 * u);
    if (tagSub) {
      g.fillStyle = rp ? 'rgba(255,255,255,.85)' : '#ffd34d'; g.font = `800 ${9.5 * u}px ${FONT}`;
      g.fillText(tagSub, x0 + 32 * u + lw + 10 * u, top + 12.5 * u);
    }
    // Mini radar under the tag.
    if (w > 420 * this.dpr) this.povRadar(R, gt, id, C, x0 + 10 * u, top + 30 * u, Math.min(w * 0.2, 170 * u));
    // Health / armour (bottom left)
    const bx = this.box.x, bw = this.box.w;
    const hpCol = st.hp < 30 ? '#ff5a4e' : '#fff';
    pill(bx + 10 * u, bot - 34 * u, 176 * u, 34 * u, 'rgba(6,9,14,.66)');
    g.fillStyle = hpCol; g.fillRect(bx + 22 * u, bot - 20 * u, 12 * u, 4 * u); g.fillRect(bx + 26 * u, bot - 24 * u, 4 * u, 12 * u);
    g.font = `900 ${19 * u}px ${FONT}`; g.textAlign = 'left'; g.fillText(String(st.hp), bx + 40 * u, bot - 16.5 * u);
    g.fillStyle = 'rgba(255,255,255,.85)';
    const shx = bx + 100 * u, shy = bot - 17 * u;
    g.beginPath(); g.moveTo(shx, shy - 8 * u); g.lineTo(shx + 7 * u, shy - 5 * u); g.lineTo(shx + 6 * u, shy + 3 * u); g.lineTo(shx, shy + 8 * u); g.lineTo(shx - 6 * u, shy + 3 * u); g.lineTo(shx - 7 * u, shy - 5 * u); g.closePath(); g.fill();
    g.font = `900 ${19 * u}px ${FONT}`; g.fillText(st.armor ? '100' : '0', shx + 12 * u, bot - 16.5 * u);

    if (!this.narrow) {
      // Ammo (bottom right)
      const cls = weaponClass(pl.weapon);
      pill(bx + bw - 150 * u, bot - 34 * u, 140 * u, 34 * u, 'rgba(6,9,14,.66)');
      g.textAlign = 'right'; g.fillStyle = st.ammo <= 5 && cls !== 'awp' ? '#ff5a4e' : '#fff'; g.font = `900 ${19 * u}px ${FONT}`;
      g.fillText(String(st.ammo), bx + bw - 74 * u, bot - 16.5 * u);
      g.fillStyle = 'rgba(255,255,255,.6)'; g.font = `700 ${12 * u}px ${FONT}`; g.textAlign = 'left';
      g.fillText(`/ ${st.reserve}`, bx + bw - 68 * u, bot - 15.5 * u);
      g.fillStyle = 'rgba(255,255,255,.75)'; g.font = `800 ${11 * u}px ${FONT}`; g.textAlign = 'right';
      g.fillText(planting || defusing ? 'C4' : (WEAPON[pl.weapon] || pl.weapon || '').toUpperCase(), bx + bw - 16 * u, bot - 46 * u);
    }
    // Player card (bottom left, above health: the weapon owns the bottom right)
    const cw = 176 * u, cx = bx + 10 * u, ch = 38 * u, cy = bot - 34 * u - 6 * u - ch;
    g.fillStyle = 'rgba(6,9,14,.8)'; g.beginPath(); g.roundRect(cx, cy, cw, ch, 6 * u); g.fill();
    g.fillStyle = col; g.fillRect(cx, cy, 5 * u, ch);
    g.fillStyle = SIDE_COL[pl.side] || '#ccc'; g.font = `900 ${9 * u}px ${FONT}`; g.textAlign = 'left';
    g.fillText(pl.side, cx + 13 * u, cy + 12 * u);
    g.fillStyle = '#fff'; g.font = `900 ${14 * u}px ${FONT}`; g.fillText(pl.handle, cx + 13 * u, cy + 26 * u, cw * 0.55);
    g.fillStyle = 'rgba(255,255,255,.55)'; g.font = `700 ${9 * u}px ${FONT}`; g.fillText((pl.role || '').toUpperCase(), cx + 32 * u, cy + 12 * u);
    const kills = R.kills.filter(q => q.killer === id && q.t <= gt).length;
    g.textAlign = 'right';
    g.fillStyle = 'rgba(150,255,150,.9)'; g.font = `800 ${10.5 * u}px ${FONT}`; g.fillText(money(st.money), cx + cw - 10 * u, cy + 12 * u);
    if (kills) { g.fillStyle = '#ffd34d'; g.font = `900 ${14 * u}px ${FONT}`; g.fillText(`${kills}K`, cx + cw - 10 * u, cy + 27 * u); }
    if (bomb.carrier === id) this.bombIcon(cx + cw - (kills ? 44 : 20) * u, cy + 26 * u, 6 * u, 1);
    // Plant / defuse progress
    const act = planting ? bomb.planting : defusing ? bomb.defusing : null;
    if (act) {
      const pw = Math.min(300 * u, bw * 0.6), px = bx + (bw - pw) / 2, by = y0 + h * 0.3;
      const pc = planting ? '#ff4b3e' : '#4fc3ff';
      const total = planting ? (R.plant.t - R.plant.start) : (R.defuse.t - R.defuse.start);
      g.fillStyle = 'rgba(6,9,14,.75)'; g.beginPath(); g.roundRect(px - 10 * u, by - 22 * u, pw + 20 * u, 44 * u, 8 * u); g.fill();
      g.fillStyle = '#fff'; g.font = `900 ${12 * u}px ${FONT}`; g.textAlign = 'left';
      g.fillText(planting ? 'PLANTING THE BOMB' : (R.defuse && R.defuse.kit ? 'DEFUSING · KIT' : 'DEFUSING · NO KIT'), px, by - 9 * u);
      g.textAlign = 'right'; g.fillStyle = pc; g.fillText(`${Math.max(0, total * (1 - act.prog)).toFixed(1)}s`, px + pw, by - 9 * u);
      g.fillStyle = 'rgba(255,255,255,.12)'; g.fillRect(px, by + 3 * u, pw, 9 * u);
      g.fillStyle = pc; g.fillRect(px, by + 3 * u, pw * act.prog, 9 * u);
    }
    // Kill-cam caption
    if (rp) {
      const kk = rp.k, kp = R.players[kk.killer] || { handle: kk.killer }, vp = R.players[kk.victim] || { handle: kk.victim };
      const txt = `${kp.handle}  ▸  ${WEAPON[kk.weapon] || kk.weapon}${kk.hs ? '  ·  HEADSHOT' : ''}${kk.smoke ? '  ·  THROUGH SMOKE' : ''}  ▸  ${vp.handle}`;
      g.font = `800 ${12 * u}px ${FONT}`; const tw2 = Math.min(g.measureText(txt).width + 30 * u, bw - 20 * u);
      g.fillStyle = 'rgba(6,9,14,.82)'; g.beginPath(); g.roundRect(bx + (bw - tw2) / 2, y0 + h * 0.06 + 40 * u, tw2, 26 * u, 13 * u); g.fill();
      g.fillStyle = '#fff'; g.textAlign = 'center'; g.fillText(txt, bx + bw / 2, y0 + h * 0.06 + 53 * u, tw2 - 20 * u);
    }
  }

  // The static map in its own canvas (rebuilt on resize). Zoomed (portrait), the
  // whole map is painted once at the zoomed scale and panned when drawn.
  ensureMapCache() {
    const f = this.fit();
    if (this.mapCache && this.mapCacheF && this.mapCacheF.s !== f.s) this.mapCache = null;
    if (!this.mapCache) {
      const c = document.createElement('canvas');
      let cf = f;
      if (this.zoomMap && this.fitZ) {
        const size = (this.meta && this.meta.size) || [100, 80];
        cf = { s: f.s, ox: 0, oy: 0 }; c.width = Math.ceil(size[0] * f.s) + 1; c.height = Math.ceil(size[1] * f.s) + 1;
        const saved = this.fitZ; this.fitZ = cf;
        try { this.renderMap(c.getContext('2d')); } finally { this.fitZ = saved; }
      } else { c.width = this.W; c.height = this.H; this.renderMap(c.getContext('2d')); }
      this.mapCache = c; this.mapCacheF = cf;
    }
    return this.mapCache;
  }

  povRadar(R, gt, id, C, x, y, wR) {
    const g = this.ctx, u = this.u, f = this.fit(), size = (this.meta && this.meta.size) || [100, 80];
    const hR = wR * size[1] / size[0], sc = wR / size[0];
    g.save();
    g.fillStyle = 'rgba(6,9,14,.75)'; g.beginPath(); g.roundRect(x - 3 * u, y - 3 * u, wR + 6 * u, hR + 6 * u, 6 * u); g.fill();
    g.beginPath(); g.roundRect(x, y, wR, hR, 4 * u); g.clip();
    const mc = this.ensureMapCache(), cf = this.mapCacheF || f;
    g.globalAlpha = 0.9; g.drawImage(mc, cf.ox, cf.oy, size[0] * cf.s, size[1] * cf.s, x, y, wR, hR); g.globalAlpha = 1;
    // view cone
    const me = R.players[id], mp = gt < me.deathT ? posAt(me.track, gt) : (me.death || posAt(me.track, gt));
    const mx = x + mp.x * sc, my = y + mp.y * sc;
    const cone = g.createRadialGradient(mx, my, 0, mx, my, 16 * sc);
    cone.addColorStop(0, 'rgba(255,255,255,.4)'); cone.addColorStop(1, 'rgba(255,255,255,0)');
    g.fillStyle = cone; g.beginPath(); g.moveTo(mx, my); g.arc(mx, my, 16 * sc, C.ang - 0.6, C.ang + 0.6); g.closePath(); g.fill();
    for (const q of Object.values(R.players)) {
      const alive = gt < q.deathT, qp = alive ? posAt(q.track, gt) : (q.death || posAt(q.track, gt));
      const px = x + qp.x * sc, py = y + qp.y * sc;
      if (!alive) { g.strokeStyle = rgba(this.teamColor(q.team), 0.7); g.lineWidth = 1.2 * u; g.beginPath(); g.moveTo(px - 2.5 * u, py - 2.5 * u); g.lineTo(px + 2.5 * u, py + 2.5 * u); g.moveTo(px + 2.5 * u, py - 2.5 * u); g.lineTo(px - 2.5 * u, py + 2.5 * u); g.stroke(); continue; }
      g.fillStyle = this.teamColor(q.team); g.beginPath(); g.arc(px, py, (q.id === id ? 4 : 3) * u, 0, Math.PI * 2); g.fill();
      if (q.id === id) { g.strokeStyle = '#fff'; g.lineWidth = 1.5 * u; g.stroke(); }
    }
    const b = bombState(R, gt);
    if (b.planted && !b.exploded) { g.fillStyle = Math.sin(performance.now() / 150) > 0 ? '#ff4b3e' : '#7a1d16'; g.beginPath(); g.arc(x + b.x * sc, y + b.y * sc, 3 * u, 0, Math.PI * 2); g.fill(); }
    g.restore();
  }

  mapTag() { this.camTag('TACTICAL', '', '#ffd34d'); }

  // Camera tag in the corner of the frame: ARENA / BOOTH / TACTICAL (POV and REPLAY draw their own).
  camTag(label, sub, dot) {
    const g = this.ctx, u = this.u, b = this.box;
    g.font = `900 ${10.5 * u}px ${FONT}`;
    const lw = g.measureText(label).width;
    g.font = `800 ${9 * u}px ${FONT}`;
    const sw = sub ? g.measureText(sub).width + 10 * u : 0;
    const tw = lw + sw + 30 * u;
    g.fillStyle = 'rgba(6,9,14,.75)'; g.beginPath(); g.roundRect(b.x + 8 * u, b.y + 8 * u, tw, 22 * u, 11 * u); g.fill();
    g.fillStyle = dot; g.beginPath(); g.arc(b.x + 19 * u, b.y + 19 * u, 3.5 * u, 0, Math.PI * 2); g.fill();
    g.fillStyle = 'rgba(255,255,255,.88)'; g.textAlign = 'left'; g.textBaseline = 'middle';
    g.font = `900 ${10.5 * u}px ${FONT}`; g.fillText(label, b.x + 27 * u, b.y + 19.5 * u);
    if (sub) { g.fillStyle = '#ffd34d'; g.font = `800 ${9 * u}px ${FONT}`; g.fillText(sub, b.x + 27 * u + lw + 10 * u, b.y + 19.5 * u); }
  }

  // Broadcast cut: a quick white pop and a team-coloured swipe across the frame.
  drawCut() {
    if (!this.cutT) return;
    const a = (performance.now() - this.cutT) / 1000; if (a > 0.32) return;
    const g = this.ctx, b = this.view.mode === 'pov' ? this.povRect() : this.box, k = a / 0.32;
    const pl = this.R && this.R.players[this.view.id];
    const col = pl ? this.teamColor(pl.team) : '#ffffff';
    g.save(); g.beginPath(); g.roundRect(b.x, b.y, b.w, b.h, 6 * this.u); g.clip();
    g.fillStyle = `rgba(255,255,255,${0.55 * (1 - k) * (1 - k)})`; g.fillRect(b.x, b.y, b.w, b.h);
    const bw = b.w * 0.22, sx = b.x - bw + (b.w + 2 * bw) * easeOut(k);
    g.fillStyle = rgba(col, 0.55 * (1 - k)); g.beginPath();
    g.moveTo(sx, b.y); g.lineTo(sx + bw, b.y); g.lineTo(sx + bw - b.h * 0.25, b.y + b.h); g.lineTo(sx - b.h * 0.25, b.y + b.h); g.closePath(); g.fill();
    g.restore();
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
    const c = this.ensureMapCache(), cf = this.mapCacheF, f = this.fit();
    if (cf && cf.ox === 0 && cf.oy === 0 && this.zoomMap) this.ctx.drawImage(c, Math.round(f.ox), Math.round(f.oy));
    else this.ctx.drawImage(c, 0, 0);
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
      g.fillText(R ? `${this.T.round} ${rn}${rn > 2 * halfOf(this.meta) ? ' · OT' : ''}` : this.T.live, cx, by + bh * 0.78);
    }
  }

  drawRosters(R, gt) {
    if (this.layout === 'none') return;
    // On the first-person cam the side panels would cover ~40% of the picture:
    // a slim strip under the scoreboard instead.
    if (this.layout === 'side' && R && this.povOn()) { this.drawRosterStrip(R, gt); return; }
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

  // The compact rosters: under the scoreboard, five chips per team (handle,
  // weapon, kills), dimmed and crossed out when dead, a bomb on the carrier.
  drawRosterStrip(R, gt) {
    const g = this.ctx, u = this.u, W = this.W;
    const teams = (this.meta && this.meta.teams) || [];
    const sw = Math.min(W - 16 * u, (this.narrow ? 560 : 760) * u), half = sw / 2 - 3 * u, x0 = (W - sw) / 2, y0 = this.topH - 2 * u, ch = 22 * u;
    g.textBaseline = 'middle';
    [0, 1].forEach(i => {
      const col = this.teamColor(i), ps = (teams[i] && teams[i].players) || [];
      const n = Math.max(1, ps.length), cw = (half - (n - 1) * 3 * u) / n;
      const bx = i === 0 ? x0 : x0 + sw - half;
      ps.forEach((mp, k) => {
        const pl = R.players[mp.id], alive = pl ? gt < pl.deathT : true;
        const cx = i === 0 ? bx + k * (cw + 3 * u) : bx + half - (k + 1) * cw - k * 3 * u;
        g.globalAlpha = alive ? 1 : 0.45;
        g.fillStyle = 'rgba(6,9,14,.82)'; g.beginPath(); g.roundRect(cx, y0, cw, ch, 4 * u); g.fill();
        g.fillStyle = alive ? col : 'rgba(255,255,255,.25)'; g.fillRect(i === 0 ? cx : cx + cw - 3 * u, y0 + 3 * u, 3 * u, ch - 6 * u);
        const kills = pl ? R.kills.filter(q => q.killer === mp.id && q.t <= gt).length : 0;
        g.textAlign = 'left'; g.fillStyle = '#fff'; g.font = `700 ${10 * u}px ${FONT}`;
        g.fillText(mp.handle, cx + 7 * u, y0 + ch * 0.38, cw - 14 * u);
        g.fillStyle = 'rgba(255,255,255,.6)'; g.font = `600 ${8 * u}px ${FONT}`;
        g.fillText(alive ? (pl ? WEAPON[pl.weapon] || pl.weapon || '' : '') : '✕', cx + 7 * u, y0 + ch * 0.76, cw - 26 * u);
        if (kills) { g.textAlign = 'right'; g.fillStyle = '#ffd34d'; g.font = `800 ${8.5 * u}px ${FONT}`; g.fillText('×' + kills, cx + cw - 6 * u, y0 + ch * 0.76); }
        if (pl && alive && bombState(R, gt).carrier === mp.id) this.bombIcon(cx + cw - 10 * u, y0 + ch * 0.36, 4 * u, 1);
      });
    });
    g.globalAlpha = 1;
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
    const y = this.onStudio ? this.H - (this.narrow ? 136 : 152) * u : this.povOn() ? this.box.y + (this.narrow ? 40 : this.layout === 'side' ? 32 : 10) * u : this.box.y + this.box.h - 30 * u, x = (this.W - tw) / 2;
    g.save(); g.globalAlpha = a;
    g.fillStyle = this.ticker.hl ? 'rgba(110,70,10,.85)' : 'rgba(6,9,14,.78)'; g.beginPath(); g.roundRect(x, y, tw, 24 * u, 12 * u); g.fill();
    g.fillStyle = this.ticker.hl ? '#ffd34d' : '#fff'; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText(text, this.W / 2, y + 12 * u, tw - 20 * u);
    g.restore();
  }

  drawIntro(R, k) {
    const g = this.ctx, u = this.u, W = this.W, T = this.T;
    const names = this.names();
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

  drawBanner(R, yc) {
    if (!R.end) return;
    const g = this.ctx, u = this.u, W = this.W, T = this.T;
    const age = (performance.now() - this.banner.t0) / 1000;
    const k = easeOut(clamp(age * 3, 0, 1));
    const win = R.end.winner, col = this.teamColor(win), names = this.names();
    const h = 74 * u, y = yc - h / 2;
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
    g.fillStyle = this.onStudio ? 'rgba(4,6,10,.8)' : 'rgba(4,6,10,.82)'; g.fillRect(0, 0, W, H);
    const names = this.names();
    if (o.kind === 'half') {
      const d = dataOf(o.e);
      const title = d.after === halfOf(this.meta) ? T.half : d.after === 2 * halfOf(this.meta) ? T.ot : T.otSwitch;
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

// ---- first-person weapons ----
// Drawn in gun space (units of the 235-px reference view, barrel towards -x).
// Colours are memoised: these run every frame.
const TONES = new Map();
function tone(a, b, k) { const key = a + b + k; let c = TONES.get(key); if (!c) { c = mix(a, b, k); TONES.set(key, c); } return c; }
function fpPoly(g, pts, fill) { g.fillStyle = fill; g.beginPath(); pts.forEach(([a, b], i) => i ? g.lineTo(a, b) : g.moveTo(a, b)); g.closePath(); g.fill(); }
function fpRect(g, x, y, w, h, fill, r = 0) { g.fillStyle = fill; if (r) { g.beginPath(); g.roundRect(x, y, w, h, r); g.fill(); } else g.fillRect(x, y, w, h); }

// A sleeve from the bottom of the frame up to the wrist at (wx, wy), with a cuff.
function fpSleeve(g, pts, cuff, col) {
  fpPoly(g, pts, col);
  // Fabric folds and the shadowed underside.
  g.strokeStyle = tone(col, '#000000', 0.28); g.lineWidth = 3; g.lineCap = 'round';
  g.beginPath(); const [a, b, c] = pts; g.moveTo(lerp(a[0], b[0], 0.35), lerp(a[1], b[1], 0.35)); g.lineTo(lerp(c[0], b[0], 0.3) + 6, lerp(c[1], b[1], 0.3) + 2); g.stroke();
  fpPoly(g, cuff, tone(col, '#000000', 0.35));
}

// A gloved hand wrapped round a grip: palm, four knuckles over the top, the
// thumb along the side, the knuckle pads lighter.
function fpHand(g, x, y, ang, sc, glove, fingersUp) {
  g.save(); g.translate(x, y); g.rotate(ang); g.scale(sc, sc);
  const hi = tone(glove, '#ffffff', 0.16), lo = tone(glove, '#000000', 0.35);
  fpRect(g, -13, -10, 26, 22, glove, 8);
  for (let i = 0; i < 4; i++) {
    const fy = -9 + i * 5.4;
    fpRect(g, fingersUp ? -17 : 8, fy, 11, 5, i % 2 ? glove : hi, 2.5);
    fpRect(g, fingersUp ? -18 : 15, fy + 1, 4, 3, lo, 1.5);
  }
  fpRect(g, -6, -15, 18, 7, hi, 3.5); // thumb
  fpRect(g, -13, 8, 26, 5, lo, 2.5); // wrist strap
  g.restore();
}

function drawLongGunFP(g, cls, wid, sleeve, glove) {
  const awp = cls === 'awp', smg = cls === 'smg', ak = wid === 'ak47';
  const metal = '#2a2c30', metal2 = '#4a4e57', dark = '#16171a';
  const body = awp ? '#3f5236' : ak ? metal : wid === 'galil' ? '#3b3a33' : '#25272b';
  const wood = ak ? '#7a4520' : body, woodHi = tone(wood, '#ffffff', 0.18), woodLo = tone(wood, '#000000', 0.35);
  const hgL = smg ? -46 : -66;
  const bL = awp ? -160 : smg ? -66 : wid === 'm4a1s' ? -112 : -118;
  // Rear arm (trigger hand) under everything.
  fpSleeve(g, [[150, 70], [40, -4], [16, 22], [100, 110]], [[46, 0], [34, -6], [14, 20], [24, 26]], sleeve);
  // Stock with a butt pad, highlight on top.
  fpPoly(g, [[30, -10], [84, -4], [88, 22], [30, 8]], awp ? body : wood);
  fpPoly(g, [[30, -10], [84, -4], [84, -1], [30, -6]], awp ? tone(body, '#ffffff', 0.15) : woodHi);
  fpPoly(g, [[80, -5], [88, -4], [92, 23], [84, 22]], dark);
  // Receiver: body, top highlight, bottom shadow, ejection port, bolt handle.
  fpRect(g, -24, -14, 58, 20, body, 2);
  fpRect(g, -24, -14, 58, 3, metal2);
  fpRect(g, -24, 3, 58, 3, tone(body, '#000000', 0.4));
  fpRect(g, 2, -9, 16, 5, dark, 1);
  fpRect(g, 12, -10, 8, 3, '#8b8f97', 1);
  // Magazine: the AK's curved banana, a straight box with ribs for the rest.
  if (ak) {
    g.fillStyle = '#8a4b1c'; g.beginPath(); g.moveTo(-10, 6); g.lineTo(8, 6); g.quadraticCurveTo(14, 26, 26, 46); g.lineTo(8, 52); g.quadraticCurveTo(-2, 30, -10, 6); g.fill();
    g.strokeStyle = '#5e3210'; g.lineWidth = 2; g.beginPath(); g.moveTo(-4, 16); g.lineTo(10, 14); g.moveTo(0, 28); g.lineTo(14, 25); g.moveTo(5, 39); g.lineTo(19, 36); g.stroke();
  } else if (awp) fpRect(g, -4, 6, 14, 20, '#1d1f1a', 2);
  else {
    fpPoly(g, [[-8, 6], [6, 6], [9, smg ? 40 : 44], [-5, smg ? 40 : 44]], '#1d1e21');
    g.strokeStyle = '#34363b'; g.lineWidth = 1.5; g.beginPath(); for (let k = 0; k < 3; k++) { const yy = 14 + k * 9; g.moveTo(-6, yy); g.lineTo(7, yy); } g.stroke();
  }
  // Pistol grip and trigger guard.
  fpPoly(g, [[22, 4], [32, 4], [36, 26], [26, 28]], dark);
  g.strokeStyle = metal; g.lineWidth = 2.5; g.beginPath(); g.moveTo(10, 6); g.quadraticCurveTo(14, 16, 24, 10); g.stroke();
  // Handguard: wood with grain (AK) or a polymer one with vent slots.
  fpRect(g, hgL, -12, -hgL - 24, 15, wood, 2);
  fpRect(g, hgL, -12, -hgL - 24, 3, woodHi);
  fpRect(g, hgL, 0, -hgL - 24, 3, woodLo);
  if (!ak && !awp) { g.fillStyle = dark; for (let x = hgL + 6; x < -30; x += 9) g.fillRect(x, -7, 5, 4); }
  // Barrel with a highlight, front sight post and a muzzle device.
  fpRect(g, bL, -9, hgL - bL, 6, metal);
  fpRect(g, bL, -9, hgL - bL, 1.6, '#62666f');
  fpRect(g, bL - 4, -11, 10, 10, dark, 2);
  if (wid === 'm4a1s') { fpRect(g, -162, -12, 52, 12, '#17181a', 4); fpRect(g, -162, -12, 52, 2.5, '#2e3034'); }
  if (!awp) {
    fpRect(g, bL + 6, -18, 4, 9, metal); // front sight
    if (!ak) { g.fillStyle = '#3a3d44'; for (let x = -22; x < 30; x += 6) g.fillRect(x, -17, 4, 3); } // top rail
    fpRect(g, 14, -21, 10, 7, metal, 1); // rear sight
  }
  if (awp) {
    fpRect(g, -44, -34, 66, 14, '#111214', 6);
    fpRect(g, -44, -34, 66, 3, '#2c2f35', 2);
    fpRect(g, -52, -37, 12, 20, '#26282c', 2); fpRect(g, 14, -36, 10, 18, '#26282c', 2);
    fpRect(g, -53, -34, 3, 14, 'rgba(120,180,255,.5)');
    fpRect(g, -10, -22, 8, 8, '#26282c'); fpRect(g, 4, -22, 8, 8, '#26282c'); // scope rings
    fpRect(g, 20, -2, 10, 4, metal2);
  }
  // Trigger hand round the grip, then the support arm and hand under the handguard.
  fpHand(g, 28, 14, 0.25, 0.95, glove, false);
  fpSleeve(g, [[hgL + 10, 6], [hgL + 34, 6], [hgL + 70, 120], [hgL - 30, 120]], [[hgL + 6, 8], [hgL + 36, 8], [hgL + 40, 20], [hgL + 2, 20]], sleeve);
  fpHand(g, hgL + 20, 2, -0.08, 0.95, glove, true);
}

function drawPistolFP(g, wid, sleeve, glove) {
  const metal = wid === 'deagle' ? '#8d9096' : '#2a2c30', hi = tone(metal, '#ffffff', 0.25), lo = tone(metal, '#000000', 0.45);
  fpSleeve(g, [[-14, 30], [18, 30], [70, 150], [-40, 150]], [[-16, 28], [20, 28], [24, 40], [-20, 40]], sleeve);
  // Frame and textured grip.
  fpPoly(g, [[-6, -2], [12, -2], [18, 36], [0, 36]], '#202124');
  g.strokeStyle = '#2f3134'; g.lineWidth = 1.2; g.beginPath(); for (let k = 0; k < 5; k++) { g.moveTo(1 + k * 0.6, 8 + k * 5); g.lineTo(14 + k * 0.6, 8 + k * 5); } g.stroke();
  g.strokeStyle = '#2a2b2e'; g.lineWidth = 2.5; g.beginPath(); g.moveTo(-14, 0); g.quadraticCurveTo(-12, 10, -2, 8); g.stroke(); // trigger guard
  // Slide: highlight, shadow, serrations, ejection port, sights.
  fpRect(g, -58, -16, 76, 16, metal, 3);
  fpRect(g, -58, -16, 76, 3, hi);
  fpRect(g, -58, -3, 76, 3, lo);
  g.fillStyle = lo; for (let x = 4; x < 16; x += 3) g.fillRect(x, -14, 1.5, 10);
  fpRect(g, -24, -14, 12, 5, '#111214', 1);
  fpRect(g, -56, -20, 4, 4, '#111'); fpRect(g, 10, -20, 6, 4, '#111');
  fpRect(g, -60, -12, 4, 8, '#111');
  if (wid === 'usp') { fpRect(g, -104, -14, 48, 12, '#18191b', 4); fpRect(g, -104, -14, 48, 2.5, '#2e3034'); }
  // Two hands: the shooting hand round the grip, the support thumb along the frame.
  fpHand(g, 6, 20, 0.15, 1, glove, false);
  fpHand(g, -6, 26, -0.2, 0.85, glove, true);
}

// ---- the arena studio: drawing ----
// Every arena camera draws a virtual 1600×900 set (covering the canvas). The
// static parts of each set are painted once into a cached canvas; the crowd,
// screens, lights, players and confetti are drawn live on top.

const VW = 1600, VH = 900;
function mix(a, b, k) {
  const x = hexToRgb(a), y = hexToRgb(b), h = v => Math.round(clamp(v, 0, 255)).toString(16).padStart(2, '0');
  return '#' + h(lerp(x[0], y[0], k)) + h(lerp(x[1], y[1], k)) + h(lerp(x[2], y[2], k));
}
const SKIN = ['#f0c8a4', '#c48f6a', '#8a5a3c'];
const HAIR = ['#17110e', '#3b2618', '#b8904e'];
const HEADBACK = ['#2b2420', '#211b17', '#18130f', '#100c0a'];
function strHash(s) { let h = 7; for (const ch of String(s || '')) h = (h * 31 + ch.charCodeAt(0)) >>> 0; return h; }

Object.assign(Cs2View.prototype, {
  // Crowd mood decays back to a murmur; confetti falls during the ceremony.
  tickStudio(dt) {
    this.clock += dt;
    const base = 0.06, f = Math.exp(-dt * 0.5);
    for (let i = 0; i < 2; i++) this.cheer[i] = base + (this.cheer[i] - base) * f;
    const party = this.party != null && (this.phase === 'ceremony' || this.phase === 'final');
    if (party && this.meta) {
      this.cheer[this.party] = Math.max(this.cheer[this.party], this.phase === 'ceremony' ? 0.95 : 0.7);
      const col = this.teamColor(this.party), cols = [col, '#ffd34d', '#ffffff', col, '#e9e9e9'];
      let n = (this.phase === 'ceremony' ? 110 : 30) * dt;
      while (n > 0 && this.confetti.length < 280) {
        if (n < 1 && Math.random() > n) break;
        n -= 1;
        this.confetti.push({ x: Math.random() * VW, y: -20 - Math.random() * 60, vx: (Math.random() - 0.5) * 60, vy: 90 + Math.random() * 90, r: Math.random() * 6.28, vr: (Math.random() - 0.5) * 10, c: cols[Math.floor(Math.random() * cols.length)], s: 5 + Math.random() * 6, ph: Math.random() * 6.28 });
      }
    }
    if (this.confetti.length) {
      let j = 0;
      for (const c of this.confetti) {
        c.x += (c.vx + Math.sin(this.clock * 2.2 + c.ph) * 45) * dt; c.y += c.vy * dt; c.r += c.vr * dt;
        if (c.y < 930) this.confetti[j++] = c;
      }
      this.confetti.length = j;
    }
  },

  pulseOf(team) { const a = this.clock - this.boothPulse[team]; if (a < 0 || a > 7) return 0; return Math.exp(-a * 0.45) * (0.55 + 0.45 * Math.sin(a * 14)); },
  tags() { const ts = (this.meta && this.meta.teams) || [], n = this.names(); return [0, 1].map(i => (ts[i] && ts[i].tag) || String(n[i]).slice(0, 4).toUpperCase()); },
  finalShot() { return { s: { shot: 'arena', screen: 'champions', team: this.party, trophy: true, dur: 1 }, i: 0, t: 9, k: 0.8 + 0.2 * Math.sin(this.clock * 0.15) }; },

  vbase() {
    // Portrait: crop in closer on the middle of the set instead of leaving big empty bands.
    const W = this.W, H = this.H, sc = Math.max(W / VW, Math.min(H / VH, W / (H > W * 1.25 ? 640 : 1000)));
    return { sc, ox: W / 2 - 800 * sc, oy: H / 2 - 450 * sc };
  },

  // A cached full-canvas layer painted in virtual coordinates (a few kept, least recently used dropped).
  studioLayer(key, paint) {
    const m = this.sCache;
    let c = m.get(key);
    if (c) { m.delete(key); m.set(key, c); return c; }
    c = document.createElement('canvas'); c.width = this.W; c.height = this.H;
    const g = c.getContext('2d'), V = this.vbase();
    g.setTransform(V.sc, 0, 0, V.sc, V.ox, V.oy);
    paint(g);
    m.set(key, c);
    while (m.size > 5) m.delete(m.keys().next().value);
    return c;
  },

  drawStudio(st) {
    const g = this.ctx, W = this.W, H = this.H, s = st.s;
    const cam = shotCamera(s, st.k), V = this.vbase(), z = cam.z, sc = V.sc * z;
    const fit = (v, h, L) => h * 2 >= L ? L / 2 : clamp(v, h, L - h);
    const fx = fit(cam.fx, W / 2 / sc, VW), fy = fit(cam.fy, H / 2 / sc, VH);
    const tx = W / 2 - fx * sc, ty = H / 2 - fy * sc;
    const scene = s.shot === 'booth' || s.shot === 'desk' || s.shot === 'trophy' ? s.shot : 'arena';
    const team = s.team === 1 ? 1 : 0;
    const key = scene === 'booth' || scene === 'trophy' ? scene + team : scene;
    const back = this.studioLayer(key + ':b', gg => scene === 'booth' ? this.paintBoothBack(gg, team) : scene === 'desk' ? this.paintDeskBack(gg) : scene === 'trophy' ? this.paintTrophyBack(gg, team) : this.paintArena(gg));
    const cached = () => g.setTransform(z, 0, 0, z, tx - V.ox * z, ty - V.oy * z);
    const live = () => g.setTransform(sc, 0, 0, sc, tx, ty);
    g.save();
    g.fillStyle = '#04060a'; g.fillRect(0, 0, W, H);
    cached(); g.drawImage(back, 0, 0);
    live();
    if (scene === 'arena') this.arenaScene(g, s, st);
    else if (scene === 'booth') {
      this.boothScene(g, s, st, team);
      cached(); g.drawImage(this.studioLayer(key + ':f', gg => this.paintBoothFront(gg, team)), 0, 0); live();
    } else if (scene === 'desk') {
      this.deskScene(g);
      cached(); g.drawImage(this.studioLayer('desk:f', gg => this.paintDeskFront(gg)), 0, 0); live();
    } else this.trophyScene(g, team);
    this.drawConfetti(g);
    g.setTransform(1, 0, 0, 1, 0, 0);
    const vg = g.createRadialGradient(W / 2, H / 2, Math.min(W, H) * 0.35, W / 2, H / 2, Math.max(W, H) * 0.75);
    vg.addColorStop(0, 'rgba(0,0,0,0)'); vg.addColorStop(1, 'rgba(0,0,0,.55)');
    g.fillStyle = vg; g.fillRect(0, 0, W, H);
    if (st.t < 0.16) { g.fillStyle = `rgba(255,255,255,${0.32 * (1 - st.t / 0.16)})`; g.fillRect(0, 0, W, H); }
    g.restore();
  },

  studioTag(st) {
    const s = st.s, red = '#ff3b3b';
    if (s.shot === 'booth') this.camTag('BOOTH', this.tags()[s.team === 1 ? 1 : 0], red);
    else this.camTag('ARENA', s.shot === 'crowd' ? 'CROWD CAM' : s.shot === 'desk' ? 'CASTER DESK' : s.shot === 'trophy' ? 'TROPHY' : '', red);
  },

  // Lower thirds on the arena cameras.
  drawStudioLower(st, R) {
    if (this.phase === 'final') return;
    const s = st.s, T = this.T, names = this.names(), sc = this.score || [0, 0];
    const yc = this.H - (this.narrow ? 64 : 80) * this.u;
    if (s.screen === 'win') { if (R && R.end && this.banner) this.drawBanner(R, yc); return; }
    if (s.screen === 'half' && this.halfE) {
      const d = dataOf(this.halfE), score = d.score || sc;
      const title = d.after === halfOf(this.meta) ? T.half : d.after === 2 * halfOf(this.meta) ? T.ot : T.otSwitch;
      this.lowerCard(yc, title, '#ffd34d', `${names[0]}   ${score[0] || 0} – ${score[1] || 0}   ${names[1]}`, d.money ? `${T.switchNote} ${money(d.money)}` : '', '#ffd34d');
    } else if (s.screen === 'welcome') {
      this.lowerCard(yc, 'LIVE FROM THE ARENA', '#ff5a4e', `${names[0]}  vs  ${names[1]}`, `MAP: ${String((this.meta && this.meta.map) || 'Mirage').toUpperCase()}${R ? ` · ${T.round} ${R.num}` : ''}`, '#ff5a4e');
    } else if (s.screen === 'champions') {
      const w = s.team === 1 ? 1 : 0;
      this.lowerCard(yc, 'CHAMPIONS', '#ffd34d', names[w], `${sc[0] || 0} – ${sc[1] || 0}`, this.teamColor(w));
    } else if (s.screen === 'matchpoint') {
      const mp = R ? matchPointTeams(sc, R.num, halfOf(this.meta)) : [];
      this.lowerCard(yc, 'MATCH POINT', '#ff5a4e', mp.length === 1 ? names[mp[0]] : `${names[0]}  ·  ${names[1]}`, `${T.round} ${R ? R.num : ''} · ${sc[0] || 0} – ${sc[1] || 0}`, mp.length === 1 ? this.teamColor(mp[0]) : '#ff5a4e');
    } else if (s.screen === 'overtime') {
      this.lowerCard(yc, T.ot, '#ffd34d', `${names[0]}   ${sc[0] || 0} – ${sc[1] || 0}   ${names[1]}`, `${T.round} ${R ? R.num : ''}`, '#ffd34d');
    } else if (s.screen === 'pistol') {
      this.lowerCard(yc, T.pistol, '#ffd34d', `${names[0]}  vs  ${names[1]}`, `${T.round} ${R ? R.num : ''}`, '#ffd34d');
    }
  },

  lowerCard(yc, title, tcol, line1, line2, accent) {
    const g = this.ctx, u = this.u, W = this.W;
    const w = Math.min(W - 24 * u, 600 * u), h = (line2 ? 70 : 54) * u, x = (W - w) / 2, y = yc - h / 2;
    g.save();
    g.fillStyle = 'rgba(6,9,14,.9)'; g.beginPath(); g.moveTo(x + 14 * u, y); g.lineTo(x + w, y); g.lineTo(x + w - 14 * u, y + h); g.lineTo(x, y + h); g.closePath(); g.fill();
    g.fillStyle = accent; g.fillRect(x + 14 * u, y, w - 14 * u, 3 * u);
    g.textAlign = 'center'; g.textBaseline = 'middle';
    g.fillStyle = tcol; g.font = `900 italic ${19 * u}px ${FONT}`; g.fillText(title, W / 2, y + 19 * u, w - 30 * u);
    g.fillStyle = '#fff'; g.font = `800 ${13 * u}px ${FONT}`; g.fillText(line1, W / 2, y + 40 * u, w - 30 * u);
    if (line2) { g.fillStyle = 'rgba(255,255,255,.65)'; g.font = `600 ${11 * u}px ${FONT}`; g.fillText(line2, W / 2, y + 58 * u, w - 30 * u); }
    g.restore();
  },

  // ---- wide shot of the arena ----

  paintArena(g) {
    const c0 = this.teamColor(0), c1 = this.teamColor(1), tags = this.tags();
    let gr = g.createLinearGradient(0, 0, 0, VH);
    gr.addColorStop(0, '#03050a'); gr.addColorStop(0.5, '#0a0f1d'); gr.addColorStop(1, '#05060b');
    g.fillStyle = gr; g.fillRect(-1200, -600, 4000, 2100);
    // LED mesh wall behind the stage
    for (let x = 180, i = 0; x < 1420; x += 36, i++) { g.fillStyle = i % 2 ? 'rgba(60,80,140,.07)' : 'rgba(60,80,140,.12)'; g.fillRect(x, 40, 34, 440); }
    for (const [x, c] of [[430, c0], [1170, c1]]) {
      const rg = g.createRadialGradient(x, 430, 0, x, 430, 440); rg.addColorStop(0, rgba(c, 0.26)); rg.addColorStop(1, rgba(c, 0));
      g.fillStyle = rg; g.fillRect(x - 440, 0, 880, 900);
    }
    const sg = g.createRadialGradient(800, 200, 0, 800, 200, 540); sg.addColorStop(0, 'rgba(120,150,255,.24)'); sg.addColorStop(1, 'rgba(120,150,255,0)');
    g.fillStyle = sg; g.fillRect(260, -340, 1080, 1080);
    // roof truss and light fixtures
    g.strokeStyle = '#2a2f3a'; g.lineWidth = 3;
    for (const y of [30, 56]) { g.beginPath(); g.moveTo(-300, y); g.lineTo(1900, y); g.stroke(); }
    g.lineWidth = 1.5; g.beginPath();
    for (let x = -300; x < 1900; x += 26) { g.moveTo(x, 30); g.lineTo(x + 13, 56); g.lineTo(x + 26, 30); }
    g.stroke();
    g.fillStyle = '#1b1f28';
    for (let i = 0; i < 10; i++) g.fillRect(70 + i * (1460 / 9) - 9, 54, 18, 12);
    g.strokeStyle = 'rgba(160,170,190,.25)'; g.beginPath();
    for (const [x, y] of [[600, 84], [1000, 84], [300, 146], [420, 146], [1180, 146], [1300, 146]]) { g.moveTo(x, 56); g.lineTo(x, y); }
    g.stroke();
    // giant screen and side screens (frames; the pictures are live)
    g.fillStyle = '#0c0e13'; g.fillRect(544, 80, 512, 236); g.strokeStyle = '#33373f'; g.lineWidth = 3; g.strokeRect(544, 80, 512, 236);
    for (const x of [250, 1130]) { g.fillStyle = '#0c0e13'; g.fillRect(x - 5, 145, 230, 128); }
    // side stands
    for (const side of [0, 1]) {
      const X = x => side ? VW - x : x;
      g.fillStyle = '#070910'; g.beginPath(); g.moveTo(X(-300), 236); g.lineTo(X(306), 236); g.lineTo(X(306), 590); g.lineTo(X(-300), 760); g.closePath(); g.fill();
      g.fillStyle = '#11141c'; g.beginPath(); g.moveTo(X(-300), 220); g.lineTo(X(310), 226); g.lineTo(X(310), 238); g.lineTo(X(-300), 240); g.closePath(); g.fill();
      // stairs between blocks
      g.strokeStyle = 'rgba(255,255,255,.04)'; g.lineWidth = 6; g.beginPath(); g.moveTo(X(150), 250); g.lineTo(X(120), 640); g.stroke();
    }
    // the stage
    gr = g.createLinearGradient(0, 470, 0, 592); gr.addColorStop(0, '#151b2a'); gr.addColorStop(1, '#090c14');
    g.fillStyle = gr; g.beginPath(); g.moveTo(262, 470); g.lineTo(1338, 470); g.lineTo(1372, 592); g.lineTo(228, 592); g.closePath(); g.fill();
    const rf = g.createRadialGradient(800, 560, 0, 800, 560, 380); rf.addColorStop(0, 'rgba(140,170,255,.13)'); rf.addColorStop(1, 'rgba(140,170,255,0)');
    g.fillStyle = rf; g.fillRect(420, 470, 760, 122);
    g.fillStyle = '#05060a'; g.fillRect(222, 592, 1156, 30);
    g.fillStyle = 'rgba(255,255,255,.08)'; g.fillRect(228, 591, 1144, 2);
    // the two team booths (shells; desks, players and glass are live)
    for (const team of [0, 1]) {
      const c = this.teamColor(team), x0 = team ? 880 : 300, x1 = x0 + 420, mx = x0 + 210;
      gr = g.createLinearGradient(0, 368, 0, 575); gr.addColorStop(0, mix(c, '#000000', 0.7)); gr.addColorStop(1, mix(c, '#000000', 0.9));
      g.fillStyle = gr; g.fillRect(x0, 368, 420, 207);
      for (let x = x0 + 18; x < x1 - 20; x += 50) { g.fillStyle = rgba(c, 0.09); g.fillRect(x, 378, 30, 96); }
      g.fillStyle = '#0b0d12'; g.fillRect(x0 - 8, 350, 436, 16); g.fillRect(x0 - 8, 568, 436, 14);
      g.fillStyle = '#12151c'; g.fillRect(x0 - 8, 350, 8, 232); g.fillRect(x1, 350, 8, 232);
      g.fillStyle = '#06070b'; g.fillRect(mx - 122, 310, 244, 38); g.strokeStyle = rgba(c, 0.85); g.lineWidth = 2; g.strokeRect(mx - 122, 310, 244, 38);
      g.fillStyle = c; g.font = `900 27px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText(tags[team], mx, 330, 224);
    }
    // the trophy pedestal between the booths
    gr = g.createLinearGradient(770, 0, 830, 0); gr.addColorStop(0, '#0c0d10'); gr.addColorStop(0.5, '#2c2f37'); gr.addColorStop(1, '#0c0d10');
    g.fillStyle = gr; g.fillRect(772, 520, 56, 72); g.fillStyle = '#c9a54a'; g.fillRect(768, 516, 64, 5);
    g.fillStyle = 'rgba(255,255,255,.55)'; g.font = `800 13px ${FONT}`; g.textAlign = 'center'; g.fillText('GREGAL ARENA', 800, 455);
  },

  arenaScene(g, s, st) {
    const ck = this.clock, lv = this.cheer, hype = Math.max(lv[0], lv[1]);
    if (!this.crowd) this.crowd = buildCrowd(7);
    // giant screen
    const mode = this.phase === 'final' ? 'champions' : s.screen && s.screen !== 'round' ? s.screen : this.phase === 'intro' ? 'round' : 'score';
    g.drawImage(this.boardImage(mode, s.team), 550, 86, 500, 224);
    g.globalCompositeOperation = 'lighter';
    const sg = g.createRadialGradient(800, 200, 60, 800, 200, 430); sg.addColorStop(0, 'rgba(130,160,255,.10)'); sg.addColorStop(1, 'rgba(130,160,255,0)');
    g.fillStyle = sg; g.fillRect(370, -230, 860, 860);
    g.globalCompositeOperation = 'source-over';
    this.sideScreens(g);
    // booths
    for (const t of [0, 1]) this.arenaBooth(g, t, this.party === t || (this.clock - this.boothPulse[t] >= 0 && this.clock - this.boothPulse[t] < 3.2));
    // trophy on its pedestal; the ceremony lifts it into a spotlight
    const tk = s.trophy ? easeOut(clamp(st.t / 1.6, 0, 1)) : 0;
    if (tk > 0) {
      g.globalCompositeOperation = 'lighter';
      const cg = g.createLinearGradient(800, 0, 800, 520); cg.addColorStop(0, `rgba(255,236,170,${0.02})`); cg.addColorStop(1, `rgba(255,236,170,${0.3 * tk})`);
      g.fillStyle = cg; g.beginPath(); g.moveTo(790, 56); g.lineTo(810, 56); g.lineTo(870, 525); g.lineTo(730, 525); g.closePath(); g.fill();
      g.globalCompositeOperation = 'source-over';
    }
    this.drawTrophy(g, 800, 516, 0.42 + tk * 0.16, tk);
    // LED ribbons: stage front and the stands' fascias
    this.ledLine(g, 222, 606, 1378, 606, 14, hype, null);
    // light show
    this.beams(g, hype, 10, 62, 760);
    // side stands (two tiers each side)
    const pal = this.crowdPal(), ex = [fanExcitement(lv, -1), fanExcitement(lv, 0), fanExcitement(lv, 1)];
    const rows = this.crowd.rows;
    for (const row of rows) {
      if (row.kind !== 'side') continue;
      this.sideRow(g, row, pal, ex);
      if (row.k === 5 || row.k === 11) {
        const side = row.fans.length && row.fans[0].x > 800 ? 1 : 0, X = x => side ? VW - x : x;
        const yi = 252 + row.k * 27 + 14, yo = yi + 42 + row.k * 5;
        g.fillStyle = '#06070b'; g.beginPath(); g.moveTo(X(-300), yo + 40); g.lineTo(X(300), yi); g.lineTo(X(300), yi + 16); g.lineTo(X(-300), yo + 70); g.closePath(); g.fill();
        this.ledLine(g, X(-40), yo + 9, X(300), yi + 4, 6, hype, this.teamColor(side));
      }
    }
    // haze over the stage
    const hz = g.createLinearGradient(0, 260, 0, 700);
    const ha = 0.06 + 0.02 * Math.sin(ck * 0.4);
    hz.addColorStop(0, 'rgba(160,180,240,0)'); hz.addColorStop(0.5, `rgba(160,180,240,${ha})`); hz.addColorStop(1, 'rgba(160,180,240,0)');
    g.fillStyle = hz; g.fillRect(-300, 260, 2200, 440);
    for (let i = 0; i < 3; i++) {
      const x = 800 + Math.sin(ck * 0.07 + i * 2.1) * 520, y = 470 + Math.sin(ck * 0.05 + i) * 60, r = 260;
      const bg = g.createRadialGradient(x, y, 0, x, y, r); bg.addColorStop(0, 'rgba(170,190,255,.05)'); bg.addColorStop(1, 'rgba(170,190,255,0)');
      g.fillStyle = bg; g.fillRect(x - r, y - r, 2 * r, 2 * r);
    }
    // pyro on big moments
    if (this.party != null || hype > 0.85) {
      const pk = this.pyroK(0), pk2 = this.pyroK(0.85);
      this.pyro(g, 250, 600, 260, pk); this.pyro(g, 1350, 600, 260, pk);
      this.pyro(g, 740, 590, 200, pk2); this.pyro(g, 860, 590, 200, pk2);
    }
    // the floor crowd, back rows first
    for (const row of rows) if (row.kind === 'floor') this.floorRow(g, row, pal, ex);
    this.camFlashes(g, rows, hype);
  },

  sideScreens(g) {
    const sc = this.score || [0, 0], tags = this.tags();
    g.textAlign = 'center'; g.textBaseline = 'middle';
    for (const t of [0, 1]) {
      const x = t ? 1130 : 250, c = this.teamColor(t), p = this.pulseOf(t);
      g.fillStyle = mix(c, '#000000', 0.62 - 0.4 * p); g.fillRect(x, 150, 220, 118);
      g.fillStyle = 'rgba(0,0,0,.25)'; g.fillRect(x, 150, 220, 40);
      g.fillStyle = '#fff'; g.font = `900 34px ${FONT}`; g.fillText(tags[t], x + 110, 171, 200);
      g.fillStyle = '#ffd34d'; g.font = `900 52px ${FONT}`; g.fillText(String(sc[t] || 0), x + 110, 232);
    }
  },

  // A team booth in the wide shot: five players at their monitors behind glass.
  arenaBooth(g, team, celebrate) {
    const c = this.teamColor(team), x0 = team ? 880 : 300, ck = this.clock;
    const jersey = mix(c, '#000000', 0.2);
    for (let i = 0; i < 5; i++) {
      const x = x0 + 50 + i * 80;
      let y = 474 + Math.sin(ck * 0.9 + i) * 0.8;
      if (celebrate) y -= 14 + Math.abs(Math.sin(ck * 7 + i * 1.3)) * 9;
      g.fillStyle = jersey; g.beginPath(); g.roundRect(x - 17, y + 10, 34, 44, 7); g.fill();
      if (celebrate) {
        g.strokeStyle = jersey; g.lineWidth = 6; g.lineCap = 'round'; g.beginPath();
        g.moveTo(x - 13, y + 15); g.lineTo(x - 22, y - 14 + Math.sin(ck * 9 + i) * 3);
        g.moveTo(x + 13, y + 15); g.lineTo(x + 22, y - 14 + Math.cos(ck * 9 + i) * 3); g.stroke();
      }
      g.fillStyle = SKIN[i % 3]; g.beginPath(); g.arc(x, y, 10, 0, 6.283); g.fill();
      g.fillStyle = '#121318'; g.beginPath(); g.arc(x, y - 2, 10.6, Math.PI, 0); g.fill();
      g.fillRect(x - 12, y - 3, 4, 8); g.fillRect(x + 8, y - 3, 4, 8);
    }
    for (let i = 0; i < 5; i++) {
      const x = x0 + 50 + i * 80;
      g.fillStyle = '#181b22'; g.fillRect(x - 28, 494, 56, 36);
      g.fillStyle = rgba(c, 0.55); g.fillRect(x - 28, 494, 56, 2);
    }
    g.fillStyle = '#0e1016'; g.fillRect(x0, 530, 420, 38); g.fillStyle = c; g.fillRect(x0, 530, 420, 3);
    // glass
    g.fillStyle = 'rgba(170,200,255,.06)'; g.fillRect(x0, 366, 420, 202);
    g.fillStyle = 'rgba(255,255,255,.05)'; g.beginPath(); g.moveTo(x0 + 60, 366); g.lineTo(x0 + 120, 366); g.lineTo(x0 + 40, 568); g.lineTo(x0, 568); g.closePath(); g.fill();
    g.fillStyle = 'rgba(255,255,255,.12)'; g.fillRect(x0 + 208, 366, 3, 202);
    // booth light: pulses after a round win
    const p = Math.max(this.pulseOf(team), this.party === team ? 0.7 + 0.3 * Math.sin(ck * 10) : 0);
    g.fillStyle = c; g.globalAlpha = 0.35 + 0.65 * p; g.fillRect(x0, 364, 420, 5); g.globalAlpha = 1;
    if (p > 0.02) {
      g.globalCompositeOperation = 'lighter';
      const lg = g.createLinearGradient(0, 366, 0, 470); lg.addColorStop(0, rgba(c, 0.4 * p)); lg.addColorStop(1, rgba(c, 0));
      g.fillStyle = lg; g.fillRect(x0, 366, 420, 104);
      const og = g.createRadialGradient(x0 + 210, 366, 0, x0 + 210, 366, 300); og.addColorStop(0, rgba(c, 0.25 * p)); og.addColorStop(1, rgba(c, 0));
      g.fillStyle = og; g.fillRect(x0 - 90, 66, 600, 600);
      g.globalCompositeOperation = 'source-over';
    }
  },

  // LED ribbon along a line: a chase in the team colours (left/right halves, or one colour).
  ledLine(g, x0, y0, x1, y1, h, hype, col) {
    const ck = this.clock, len = Math.hypot(x1 - x0, y1 - y0), n = Math.max(4, Math.round(len / 20));
    const win = this.party != null ? this.party : this.pulseOf(0) > 0.05 ? 0 : this.pulseOf(1) > 0.05 ? 1 : -1;
    const c0 = this.teamColor(0), c1 = this.teamColor(1);
    g.globalCompositeOperation = 'lighter';
    for (let i = 0; i < n; i++) {
      const a = i / n, b = (i + 0.78) / n;
      const ax = lerp(x0, x1, a), ay = lerp(y0, y1, a), bx = lerp(x0, x1, b), by = lerp(y0, y1, b);
      const br = 0.2 + 0.8 * Math.pow(Math.max(0, Math.sin(a * 16 - ck * (3 + hype * 7))), 2);
      const c = win >= 0 && (col == null || Math.floor(ck * 6) % 2) ? (win ? c1 : c0) : col || (a < 0.5 ? c0 : c1);
      g.fillStyle = rgba(c, 0.3 + 0.6 * br);
      g.beginPath(); g.moveTo(ax, ay - h / 2); g.lineTo(bx, by - h / 2); g.lineTo(bx, by + h / 2); g.lineTo(ax, ay + h / 2); g.closePath(); g.fill();
    }
    g.globalCompositeOperation = 'source-over';
  },

  // Moving-head beams from the roof truss; they speed up and strobe when the crowd roars.
  beams(g, hype, n, top, len) {
    const ck = this.clock, cols = [this.teamColor(0), '#ffffff', this.teamColor(1), '#9fb7ff'];
    const strobe = hype > 0.75 && Math.floor(ck * 9) % 2 === 0;
    g.globalCompositeOperation = 'lighter';
    for (let i = 0; i < n; i++) {
      const ox = 70 + i * (1460 / (n - 1)), oy = top;
      const ang = Math.PI / 2 + Math.sin(ck * (0.45 + hype * 0.9) + i * 1.9) * 0.5 + (i < n / 2 ? 0.14 : -0.14);
      const ex = ox + Math.cos(ang) * len, ey = oy + Math.sin(ang) * len, px = -Math.sin(ang), py = Math.cos(ang), w = 64;
      const col = strobe ? '#ffffff' : cols[i % 4], a = (0.09 + hype * 0.12) * (strobe ? 1.5 : 1);
      const gr = g.createLinearGradient(ox, oy, ex, ey); gr.addColorStop(0, rgba(col, a)); gr.addColorStop(1, rgba(col, 0));
      g.fillStyle = gr; g.beginPath(); g.moveTo(ox - px * 4, oy - py * 4); g.lineTo(ox + px * 4, oy + py * 4); g.lineTo(ex + px * w, ey + py * w); g.lineTo(ex - px * w, ey - py * w); g.closePath(); g.fill();
      g.fillStyle = rgba(col, 0.85); g.beginPath(); g.arc(ox, oy + 3, 5, 0, 6.283); g.fill();
    }
    g.globalCompositeOperation = 'source-over';
  },

  pyroK(off) { const t = (this.clock + off) % 1.7; return t < 0.8 ? Math.sin(t / 0.8 * Math.PI) : 0; },
  pyro(g, x, y, h, k) {
    if (k <= 0.02) return;
    g.globalCompositeOperation = 'lighter';
    for (let j = 0; j < 4; j++) {
      const yy = y - h * (0.15 + j * 0.22) * k, r = h * (0.16 + j * 0.05) * (0.6 + 0.4 * k);
      const gr = g.createRadialGradient(x, yy, 0, x, yy, r);
      gr.addColorStop(0, `rgba(255,240,190,${0.55 * k})`); gr.addColorStop(0.4, `rgba(255,140,40,${0.35 * k})`); gr.addColorStop(1, 'rgba(255,60,0,0)');
      g.fillStyle = gr; g.fillRect(x - r, yy - r, 2 * r, 2 * r);
    }
    g.globalCompositeOperation = 'source-over';
  },

  crowdPal() {
    const c0 = this.teamColor(0), c1 = this.teamColor(1), key = c0 + c1;
    if (this._pal && this._pal.key === key) return this._pal;
    const base = [c0, mix(c0, '#ffffff', 0.3), c1, mix(c1, '#ffffff', 0.3), '#2c313c', '#16181e', '#c9ced8'];
    const shade = b => base.map(c => mix(c, '#000000', 1 - b));
    return (this._pal = {
      key, floor: [0.55, 0.42, 0.32, 0.24].map(shade), side: [0.42, 0.46, 0.5, 0.54].map(shade),
      stick: [mix(c0, '#ffffff', 0.18), mix(c1, '#ffffff', 0.18), '#cfe6ff'], flag: [c0, c1, '#dddddd'],
      skin: SKIN.map(c => mix(c, '#000000', 0.45)),
    });
  },

  // Floor fans seen from behind: shirts, backs of heads, arms up and jumping when it's their moment.
  floorRow(g, row, pal, ex) {
    const ck = this.clock, F = row.fans, P = pal.floor[row.band];
    let k = -1;
    g.beginPath();
    for (const f of F) {
      if (f.k !== k) { if (k >= 0) { g.fillStyle = P[k]; g.fill(); g.beginPath(); } k = f.k; }
      const e = ex[f.team + 1], s = f.s, up = e > f.th;
      const hop = up ? e * Math.max(0, Math.sin(ck * 7.5 + f.ph)) * 0.32 * s : 0;
      const x = f.x + Math.sin(ck * 1.1 + f.ph) * 0.06 * s, y = f.y - hop;
      f._x = x; f._y = y; f._up = up;
      g.moveTo(x + 0.5 * s, y + 0.3 * s); g.ellipse(x, y + 0.3 * s, 0.5 * s, 0.3 * s, 0, 0, 6.283); g.rect(x - 0.5 * s, y + 0.3 * s, s, 1.3 * s);
      if (up) {
        const w = Math.sin(ck * 6 + f.ph) * 0.12 * s;
        g.rect(x - 0.6 * s + w, y - 0.95 * s, 0.18 * s, 1.05 * s); g.rect(x + 0.42 * s - w, y - 0.95 * s, 0.18 * s, 1.05 * s);
      }
    }
    if (k >= 0) { g.fillStyle = P[k]; g.fill(); }
    g.beginPath();
    for (const f of F) { const r = 0.31 * f.s, hy = f._y - 0.3 * f.s; g.moveTo(f._x + r, hy); g.arc(f._x, hy, r, 0, 6.283); }
    g.fillStyle = HEADBACK[row.band]; g.fill();
    this.crowdProps(g, F, pal, ex, F.length ? F[0].s : 8);
  },

  // Side-stand fans face the stage: seated in the quiet, on their feet for the big moments.
  sideRow(g, row, pal, ex) {
    const ck = this.clock, F = row.fans, P = pal.side[row.band];
    let k = -1;
    g.beginPath();
    for (const f of F) {
      if (f.k !== k) { if (k >= 0) { g.fillStyle = P[k]; g.fill(); g.beginPath(); } k = f.k; }
      const e = ex[f.team + 1], s = f.s, up = e > f.th;
      const hop = up ? e * Math.max(0, Math.sin(ck * 7 + f.ph)) * 0.25 * s : 0;
      const x = f.x, y = f.y + (up ? -0.35 * s - hop : 0.1 * s);
      f._x = x; f._y = y; f._up = up;
      g.moveTo(x + 0.45 * s, y + 0.25 * s); g.ellipse(x, y + 0.25 * s, 0.45 * s, 0.25 * s, 0, 0, 6.283); g.rect(x - 0.45 * s, y + 0.25 * s, 0.9 * s, 1.05 * s);
      if (up) g.rect(x - 0.62 * s, y - 0.85 * s, 0.17 * s, 0.95 * s), g.rect(x + 0.45 * s, y - 0.85 * s, 0.17 * s, 0.95 * s);
    }
    if (k >= 0) { g.fillStyle = P[k]; g.fill(); }
    for (let sk = 0; sk < 3; sk++) {
      g.beginPath();
      for (const f of F) { if (f.skin !== sk) continue; const r = 0.3 * f.s, hy = f._y - 0.28 * f.s; g.moveTo(f._x + r, hy); g.arc(f._x, hy, r, 0, 6.283); }
      g.fillStyle = pal.skin[sk]; g.fill();
    }
    this.crowdProps(g, F, pal, ex, F.length ? F[F.length >> 1].s : 6);
  },

  // Light sticks (glowing), flags and phone lights.
  crowdProps(g, F, pal, ex, s0) {
    const ck = this.clock;
    g.lineCap = 'round';
    for (let t = 0; t < 3; t++) {
      const team = t === 2 ? -1 : t;
      g.beginPath();
      let any = false;
      for (const f of F) {
        if (f.prop !== 1 || f.team !== team) continue;
        const s = f.s, sw = Math.sin(ck * (f._up ? 5 : 1.4) + f.ph);
        let hx, hy, tx, ty;
        if (f._up) { hx = f._x + 0.5 * s; hy = f._y - 0.95 * s; tx = hx + sw * 0.4 * s; ty = hy - 0.8 * s; }
        else { hx = f._x + 0.45 * s; hy = f._y + 0.05 * s; tx = hx + 0.1 * s + sw * 0.12 * s; ty = hy - 0.62 * s; }
        g.moveTo(hx, hy); g.lineTo(tx, ty); any = true;
      }
      if (!any) continue;
      const col = pal.stick[t];
      g.globalCompositeOperation = 'lighter';
      g.strokeStyle = rgba(col, 0.22); g.lineWidth = s0 * 0.42; g.stroke();
      g.strokeStyle = col; g.lineWidth = s0 * 0.13; g.stroke();
      g.globalCompositeOperation = 'source-over';
    }
    for (let t = 0; t < 2; t++) {
      g.beginPath();
      let any = false;
      for (const f of F) {
        if (f.prop !== 2 || f.team !== t) continue;
        const s = f.s, px = f._x + 0.4 * s, top = f._y - 2.4 * s, wv = Math.sin(ck * (f._up ? 7 : 2) + f.ph) * (f._up ? 0.35 : 0.12) * s;
        g.moveTo(px, top); g.lineTo(px + 1.5 * s, top + wv * 0.5); g.lineTo(px + 1.5 * s, top + 0.95 * s + wv); g.lineTo(px, top + 0.95 * s); g.closePath();
        any = true;
      }
      if (!any) continue;
      g.fillStyle = pal.flag[t]; g.fill();
      g.strokeStyle = 'rgba(200,200,200,.6)'; g.lineWidth = s0 * 0.06; g.beginPath();
      for (const f of F) if (f.prop === 2 && f.team === t) { g.moveTo(f._x + 0.4 * f.s, f._y + 0.2 * f.s); g.lineTo(f._x + 0.4 * f.s, f._y - 2.4 * f.s); }
      g.stroke();
    }
    g.beginPath();
    let any = false;
    for (const f of F) {
      if (f.prop !== 3 || !f._up) continue;
      const s = f.s; g.rect(f._x + 0.32 * s, f._y - 1.25 * s, 0.2 * s, 0.32 * s); any = true;
    }
    if (any) { g.globalCompositeOperation = 'lighter'; g.fillStyle = '#e8f4ff'; g.fill(); g.globalCompositeOperation = 'source-over'; }
  },

  camFlashes(g, rows, hype) {
    const n = Math.floor(hype * 8) + (Math.random() < 0.3 ? 1 : 0);
    if (!n) return;
    const img = this.sparkImg();
    for (let i = 0; i < n; i++) {
      const row = rows[Math.floor(Math.random() * rows.length)], f = row.fans[Math.floor(Math.random() * row.fans.length)];
      if (!f) continue;
      const r = f.s * 1.5;
      g.drawImage(img, f._x - r, f._y - f.s * 0.9 - r, 2 * r, 2 * r);
    }
  },
  sparkImg() {
    if (this._spark) return this._spark;
    const c = document.createElement('canvas'); c.width = c.height = 32;
    const g = c.getContext('2d'), gr = g.createRadialGradient(16, 16, 0, 16, 16, 16);
    gr.addColorStop(0, 'rgba(255,255,255,1)'); gr.addColorStop(0.25, 'rgba(255,255,255,.55)'); gr.addColorStop(1, 'rgba(255,255,255,0)');
    g.fillStyle = gr; g.fillRect(0, 0, 32, 32);
    return (this._spark = c);
  },

  drawConfetti(g) {
    for (const c of this.confetti) {
      const w = c.s * Math.abs(Math.cos(c.r)) + 1, h = c.s * 0.6;
      g.fillStyle = c.c; g.fillRect(c.x - w / 2, c.y - h / 2, w, h);
    }
  },

  // The trophy: (x, y) is the bottom of its base; tk>0 adds the ceremony glint.
  drawTrophy(g, x, y, s, tk = 0) {
    const S = v => v * s;
    if (tk > 0) {
      g.globalCompositeOperation = 'lighter';
      const gl = g.createRadialGradient(x, y - S(90), 0, x, y - S(90), S(160)); gl.addColorStop(0, `rgba(255,220,120,${0.45 * tk})`); gl.addColorStop(1, 'rgba(255,220,120,0)');
      g.fillStyle = gl; g.fillRect(x - S(160), y - S(250), S(320), S(320));
      g.globalCompositeOperation = 'source-over';
    }
    const gold = g.createLinearGradient(x - S(44), 0, x + S(44), 0);
    gold.addColorStop(0, '#6e5010'); gold.addColorStop(0.3, '#ffe38a'); gold.addColorStop(0.5, '#c6962a'); gold.addColorStop(0.72, '#fff2c2'); gold.addColorStop(1, '#7a5a14');
    g.fillStyle = '#121216'; g.beginPath(); g.moveTo(x - S(40), y); g.lineTo(x + S(40), y); g.lineTo(x + S(30), y - S(26)); g.lineTo(x - S(30), y - S(26)); g.closePath(); g.fill();
    g.fillStyle = '#d9b456'; g.fillRect(x - S(31), y - S(28), S(62), S(3));
    g.fillStyle = gold; g.fillRect(x - S(8), y - S(62), S(16), S(36));
    g.beginPath(); g.ellipse(x, y - S(46), S(14), S(5), 0, 0, 6.283); g.fill();
    g.beginPath(); g.moveTo(x - S(44), y - S(132));
    g.bezierCurveTo(x - S(44), y - S(84), x - S(14), y - S(64), x, y - S(62));
    g.bezierCurveTo(x + S(14), y - S(64), x + S(44), y - S(84), x + S(44), y - S(132)); g.closePath(); g.fill();
    g.strokeStyle = gold; g.lineWidth = S(6); g.beginPath();
    g.moveTo(x - S(42), y - S(122)); g.bezierCurveTo(x - S(74), y - S(122), x - S(70), y - S(84), x - S(30), y - S(80));
    g.moveTo(x + S(42), y - S(122)); g.bezierCurveTo(x + S(74), y - S(122), x + S(70), y - S(84), x + S(30), y - S(80)); g.stroke();
    g.fillStyle = '#fff4c8'; g.beginPath(); g.ellipse(x, y - S(132), S(44), S(7), 0, 0, 6.283); g.fill();
    g.fillStyle = '#8a6a1c'; g.beginPath(); g.ellipse(x, y - S(132), S(38), S(4.5), 0, 0, 6.283); g.fill();
    g.fillStyle = 'rgba(255,255,255,.55)'; g.fillRect(x - S(26), y - S(124), S(4), S(44));
    // glint
    const gk = Math.max(0, Math.sin(this.clock * 2.3)) ** 8;
    if (gk > 0.05) {
      g.globalCompositeOperation = 'lighter'; g.fillStyle = `rgba(255,255,255,${gk})`;
      const gx = x + S(24), gy = y - S(118), r = S(16) * gk;
      g.beginPath(); g.moveTo(gx - r, gy); g.lineTo(gx, gy - S(2)); g.lineTo(gx + r, gy); g.lineTo(gx, gy + S(2)); g.closePath(); g.fill();
      g.beginPath(); g.moveTo(gx, gy - r); g.lineTo(gx + S(2), gy); g.lineTo(gx, gy + r); g.lineTo(gx - S(2), gy); g.closePath(); g.fill();
      g.globalCompositeOperation = 'source-over';
    }
  },

  // The giant screen above the stage (redrawn only when what it shows changes).
  boardImage(mode, team) {
    const BW = 640, BH = 286, sc = this.score || [0, 0], names = this.names(), tags = this.tags(), rn = this.R ? this.R.num : 0;
    const flash = Math.floor(this.clock * 2.5) % 2, anim = mode === 'matchpoint' || mode === 'champions' || mode === 'win';
    const key = `${mode}|${team}|${sc[0]}|${sc[1]}|${rn}|${anim ? flash : 0}|${names[0]}|${names[1]}`;
    if (this.board && this.boardKey === key) return this.board;
    if (!this.board) { this.board = document.createElement('canvas'); this.board.width = BW; this.board.height = BH; }
    this.boardKey = key;
    const g = this.board.getContext('2d'), F = (w, s, it) => `${it ? 'italic ' : ''}${w} ${s}px ${FONT}`;
    const fg = c => luminance(c) > 0.6 ? '#0b0d12' : '#ffffff';
    g.setTransform(1, 0, 0, 1, 0, 0); g.globalAlpha = 1;
    g.fillStyle = '#04060c'; g.fillRect(0, 0, BW, BH);
    g.textAlign = 'center'; g.textBaseline = 'middle';
    const scoreRow = (y, size) => {
      g.font = F(900, size); g.fillStyle = '#fff';
      g.fillText(String(sc[0] || 0), BW / 2 - size * 0.85, y); g.fillText(String(sc[1] || 0), BW / 2 + size * 0.85, y);
      g.fillStyle = 'rgba(255,255,255,.45)'; g.fillText(':', BW / 2, y - size * 0.06);
    };
    const teamBars = () => [0, 1].forEach(i => {
      const c = this.teamColor(i), x = i ? BW - 210 : 0, cx = i ? BW - 98 : 98;
      const gr = g.createLinearGradient(i ? BW : 0, 0, i ? BW - 210 : 210, 0); gr.addColorStop(0, rgba(c, 0.95)); gr.addColorStop(1, rgba(c, 0));
      g.fillStyle = gr; g.fillRect(x, 0, 210, BH);
      g.fillStyle = '#fff'; g.font = F(900, 50); g.fillText(tags[i], cx, BH / 2 - 14, 180);
      g.font = F(700, 17); g.fillStyle = 'rgba(255,255,255,.85)'; g.fillText(String(names[i]).toUpperCase(), cx, BH / 2 + 30, 180);
    });
    const c = team === 0 || team === 1 ? this.teamColor(team) : '#ffd34d';
    switch (mode) {
      case 'round': case 'pistol':
        teamBars(); g.fillStyle = '#ffd34d'; g.font = F(800, 20); g.fillText(mode === 'pistol' ? 'PISTOL ROUND' : 'NEXT ROUND', BW / 2, 56);
        g.fillStyle = '#fff'; g.font = F(900, 60); g.fillText(`ROUND ${rn}`, BW / 2, 112, 230); scoreRow(206, 56); break;
      case 'matchpoint':
        g.fillStyle = flash ? c : '#04060c'; g.fillRect(0, 0, BW, BH);
        g.fillStyle = flash ? fg(c) : c; g.font = F(900, 90, true); g.fillText('MATCH POINT', BW / 2, 110, BW - 40);
        g.font = F(900, 30); g.fillText(team === 0 || team === 1 ? String(names[team]).toUpperCase() : 'BOTH TEAMS', BW / 2, 190, BW - 60);
        g.font = F(800, 28); g.fillText(`${sc[0] || 0} : ${sc[1] || 0}`, BW / 2, 240); break;
      case 'overtime':
        teamBars(); g.fillStyle = '#ffd34d'; g.font = F(900, 58, true); g.fillText('OVERTIME', BW / 2, 92, 230); scoreRow(196, 60); break;
      case 'half':
        teamBars(); g.fillStyle = '#ffd34d'; g.font = F(900, 54); g.fillText('HALF TIME', BW / 2, 92, 230); scoreRow(196, 64); break;
      case 'win':
        g.fillStyle = flash ? c : mix(c, '#000000', 0.6); g.fillRect(0, 0, BW, BH);
        g.fillStyle = flash ? fg(c) : '#fff'; g.font = F(900, 96, true); g.fillText(tags[team === 1 ? 1 : 0], BW / 2, 104, BW - 60);
        g.font = F(900, 40); g.fillText('WIN THE ROUND', BW / 2, 186, BW - 60);
        g.font = F(800, 26); g.fillText(`${sc[0] || 0} : ${sc[1] || 0}`, BW / 2, 244); break;
      case 'champions': {
        const gr = g.createRadialGradient(BW / 2, BH / 2, 10, BW / 2, BH / 2, BW / 1.6); gr.addColorStop(0, '#3a2a06'); gr.addColorStop(1, '#05060a');
        g.fillStyle = gr; g.fillRect(0, 0, BW, BH);
        g.fillStyle = flash ? '#ffd34d' : '#fff3b0'; g.font = F(900, 82, true); g.fillText('CHAMPIONS', BW / 2, 100, BW - 40);
        g.fillStyle = c; g.font = F(900, 40); g.fillText(String(names[team === 1 ? 1 : 0]).toUpperCase(), BW / 2, 180, BW - 60);
        g.fillStyle = '#fff'; g.font = F(800, 28); g.fillText(`${sc[0] || 0} : ${sc[1] || 0}`, BW / 2, 238); break;
      }
      case 'welcome':
        teamBars(); g.fillStyle = '#ff5a4e'; g.font = F(800, 20); g.fillText('● LIVE', BW / 2, 56);
        g.fillStyle = '#fff'; g.font = F(900, 80, true); g.fillText('VS', BW / 2, 140);
        g.fillStyle = 'rgba(255,255,255,.7)'; g.font = F(700, 20); g.fillText(String((this.meta && this.meta.map) || 'Mirage').toUpperCase(), BW / 2, 222); break;
      default:
        teamBars(); scoreRow(BH / 2 + 4, 92);
        g.fillStyle = 'rgba(255,255,255,.7)'; g.font = F(800, 20); g.fillText(`ROUND ${rn}`, BW / 2, 46);
        g.fillStyle = '#ff5a4e'; g.fillText('● LIVE', BW / 2, BH - 34);
    }
    g.fillStyle = 'rgba(0,0,0,.2)';
    for (let y = 0; y < BH; y += 4) g.fillRect(0, y, BW, 1);
    return this.board;
  },

  // ---- people (front view): players, casters ----

  person(g, x, y, s, o) {
    const len = o.len || 6;
    g.fillStyle = mix(o.skin, '#000000', 0.25); g.fillRect(x - 0.36 * s, y + 0.6 * s, 0.72 * s, 0.75 * s);
    const tg = g.createLinearGradient(0, y + 1.15 * s, 0, y + (1.15 + Math.min(len, 6)) * s);
    tg.addColorStop(0, mix(o.shirt, '#ffffff', 0.14)); tg.addColorStop(0.45, o.shirt); tg.addColorStop(1, mix(o.shirt, '#000000', 0.5));
    g.fillStyle = tg; g.beginPath(); g.roundRect(x - 1.85 * s, y + 1.15 * s, 3.7 * s, len * s, [0.95 * s, 0.95 * s, 0, 0]); g.fill();
    g.fillStyle = 'rgba(0,0,0,.22)'; g.fillRect(x - 1.85 * s, y + 1.9 * s, 0.45 * s, (len - 0.8) * s); g.fillRect(x + 1.4 * s, y + 1.9 * s, 0.45 * s, (len - 0.8) * s);
    if (o.suit) {
      g.fillStyle = o.trim; g.beginPath(); g.moveTo(x - 0.5 * s, y + 1.15 * s); g.lineTo(x + 0.5 * s, y + 1.15 * s); g.lineTo(x, y + 2.9 * s); g.closePath(); g.fill();
      g.fillStyle = o.tie; g.beginPath(); g.moveTo(x - 0.13 * s, y + 1.3 * s); g.lineTo(x + 0.13 * s, y + 1.3 * s); g.lineTo(x + 0.2 * s, y + 2.6 * s); g.lineTo(x, y + 2.85 * s); g.lineTo(x - 0.2 * s, y + 2.6 * s); g.closePath(); g.fill();
      g.strokeStyle = 'rgba(0,0,0,.35)'; g.lineWidth = 0.08 * s; g.beginPath(); g.moveTo(x - 0.5 * s, y + 1.15 * s); g.lineTo(x - 0.1 * s, y + 3.1 * s); g.moveTo(x + 0.5 * s, y + 1.15 * s); g.lineTo(x + 0.1 * s, y + 3.1 * s); g.stroke();
    } else {
      g.strokeStyle = o.trim; g.lineWidth = 0.14 * s; g.beginPath(); g.ellipse(x, y + 1.15 * s, 0.55 * s, 0.32 * s, 0, 0, Math.PI); g.stroke();
      g.fillStyle = o.trim; g.fillRect(x - 1.55 * s, y + 1.5 * s, 0.13 * s, 2.6 * s); g.fillRect(x + 1.42 * s, y + 1.5 * s, 0.13 * s, 2.6 * s);
      g.beginPath(); g.arc(x + 0.85 * s, y + 2.15 * s, 0.22 * s, 0, 6.283); g.fill();
      if (o.label) { g.fillStyle = o.trim; g.font = `900 ${0.42 * s}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText(o.label, x - 0.35 * s, y + 2.2 * s, 1.4 * s); }
    }
    this.arms(g, x, y, s, o);
    g.fillStyle = o.skin; g.beginPath(); g.ellipse(x, y, 0.8 * s, 0.98 * s, 0, 0, 6.283); g.fill();
    g.fillStyle = mix(o.skin, '#000000', 0.18); g.beginPath(); g.ellipse(x - 0.82 * s, y + 0.05 * s, 0.14 * s, 0.24 * s, 0, 0, 6.283); g.ellipse(x + 0.82 * s, y + 0.05 * s, 0.14 * s, 0.24 * s, 0, 0, 6.283); g.fill();
    g.fillStyle = o.hair; g.beginPath(); g.ellipse(x, y - 0.3 * s, 0.86 * s, 0.74 * s, 0, Math.PI, 0); g.fill();
    g.fillRect(x - 0.86 * s, y - 0.32 * s, 0.2 * s, 0.4 * s); g.fillRect(x + 0.66 * s, y - 0.32 * s, 0.2 * s, 0.4 * s);
    // face
    g.fillStyle = '#1a1210';
    if (o.mouth > 0.45) {
      g.strokeStyle = '#1a1210'; g.lineWidth = 0.08 * s; g.beginPath();
      g.arc(x - 0.3 * s, y + 0.06 * s, 0.11 * s, Math.PI * 1.1, Math.PI * 1.9); g.moveTo(x + 0.41 * s, y + 0.06 * s); g.arc(x + 0.3 * s, y + 0.06 * s, 0.11 * s, Math.PI * 1.1, Math.PI * 1.9); g.stroke();
    } else { g.beginPath(); g.ellipse(x - 0.3 * s, y + 0.02 * s, 0.085 * s, 0.1 * s, 0, 0, 6.283); g.ellipse(x + 0.3 * s, y + 0.02 * s, 0.085 * s, 0.1 * s, 0, 0, 6.283); g.fill(); }
    g.strokeStyle = mix(o.hair, '#000000', 0.2); g.lineWidth = 0.07 * s; g.beginPath(); g.moveTo(x - 0.45 * s, y - 0.2 * s); g.lineTo(x - 0.16 * s, y - 0.24 * s); g.moveTo(x + 0.45 * s, y - 0.2 * s); g.lineTo(x + 0.16 * s, y - 0.24 * s); g.stroke();
    if (o.mouth > 0.2) { g.fillStyle = '#4a1414'; g.beginPath(); g.ellipse(x, y + 0.5 * s, 0.22 * s + 0.06 * s * o.mouth, 0.05 * s + 0.2 * s * o.mouth, 0, 0, 6.283); g.fill(); }
    else { g.strokeStyle = '#5a2a22'; g.lineWidth = 0.06 * s; g.beginPath(); g.moveTo(x - 0.18 * s, y + 0.5 * s); g.quadraticCurveTo(x, y + 0.58 * s, x + 0.18 * s, y + 0.5 * s); g.stroke(); }
    if (o.headset) {
      const hs = o.headset === true ? '#141519' : o.headset;
      g.strokeStyle = '#141519'; g.lineWidth = 0.2 * s; g.beginPath(); g.ellipse(x, y - 0.05 * s, 0.94 * s, 1.1 * s, 0, Math.PI * 1.06, Math.PI * 1.94); g.stroke();
      g.fillStyle = '#141519'; g.beginPath(); g.roundRect(x - 1.08 * s, y - 0.32 * s, 0.38 * s, 0.68 * s, 0.14 * s); g.roundRect(x + 0.7 * s, y - 0.32 * s, 0.38 * s, 0.68 * s, 0.14 * s); g.fill();
      g.fillStyle = hs; g.fillRect(x - 1.06 * s, y - 0.1 * s, 0.08 * s, 0.24 * s); g.fillRect(x + 0.98 * s, y - 0.1 * s, 0.08 * s, 0.24 * s);
      g.strokeStyle = '#141519'; g.lineWidth = 0.07 * s; g.beginPath(); g.moveTo(x - 0.9 * s, y + 0.25 * s); g.quadraticCurveTo(x - 0.75 * s, y + 0.72 * s, x - 0.28 * s, y + 0.64 * s); g.stroke();
      g.fillStyle = '#141519'; g.beginPath(); g.arc(x - 0.26 * s, y + 0.64 * s, 0.08 * s, 0, 6.283); g.fill();
    }
  },

  arms(g, x, y, s, o) {
    const a = o.arms;
    if (!a || a === 'down') return;
    const ck = this.clock, i = o.i || 0, w = Math.sin(ck * 8 + i) * 0.22 * s;
    const sl = [x - 1.5 * s, y + 1.5 * s], sr = [x + 1.5 * s, y + 1.5 * s];
    const list = [];
    if (a === 'up') { list.push([sl, [x - 2.35 * s, y - 0.1 * s], [x - 2.0 * s + w, y - 2.2 * s]], [sr, [x + 2.35 * s, y - 0.1 * s], [x + 2.0 * s - w, y - 2.2 * s]]); }
    else if (a === 'pump') { const p = Math.abs(Math.sin(ck * 7 + i)); list.push([sr, [x + 2.5 * s, y + 0.5 * s], [x + 2.1 * s, y - 1.0 * s - p * 0.9 * s]], [sl, [x - 2.2 * s, y + 2.4 * s], [x - 1.0 * s, y + 3.0 * s]]); }
    else if (a === 'clap') { const p = Math.abs(Math.sin(ck * 10 + i)) * 0.45 * s; list.push([sl, [x - 2.1 * s, y + 2.6 * s], [x - 0.3 * s - p, y + 2.1 * s]], [sr, [x + 2.1 * s, y + 2.6 * s], [x + 0.3 * s + p, y + 2.1 * s]]); }
    else if (a === 'hold') { list.push([sl, [x - 2.1 * s, y - 0.5 * s], [x - 0.6 * s, y - 2.55 * s]], [sr, [x + 2.1 * s, y - 0.5 * s], [x + 0.6 * s, y - 2.55 * s]]); }
    else if (a === 'talk') { list.push([sr, [x + 2.0 * s, y + 3.2 * s], [x + 1.0 * s + Math.sin(ck * 3) * 0.35 * s, y + 2.1 * s + Math.sin(ck * 4.2) * 0.25 * s]]); }
    g.lineCap = 'round';
    for (const [sh, el, h] of list) {
      g.strokeStyle = o.shirt; g.lineWidth = 0.86 * s; g.beginPath(); g.moveTo(sh[0], sh[1]); g.lineTo(el[0], el[1]); g.stroke();
      g.strokeStyle = o.suit ? o.shirt : o.skin; g.lineWidth = 0.62 * s; g.beginPath(); g.moveTo(el[0], el[1]); g.lineTo(h[0], h[1]); g.stroke();
      g.fillStyle = o.skin; g.beginPath(); g.arc(h[0], h[1], 0.36 * s, 0, 6.283); g.fill();
    }
  },

  // ---- close shot of a team booth ----

  paintBoothBack(g, team) {
    const c = this.teamColor(team), tag = this.tags()[team];
    const gr = g.createLinearGradient(0, 0, 0, VH); gr.addColorStop(0, mix(c, '#000000', 0.76)); gr.addColorStop(1, '#05060a');
    g.fillStyle = gr; g.fillRect(-1200, -600, 4000, 2100);
    for (let x = -460, i = 0; x < 2060; x += 210, i++) { g.fillStyle = rgba(c, i % 2 ? 0.06 : 0.1); g.fillRect(x + 12, 80, 186, 900); g.fillStyle = rgba(c, 0.25); g.fillRect(x + 12, 80, 3, 900); }
    g.fillStyle = rgba(c, 0.13); g.font = `900 270px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText(tag, 800, 300, 1500);
    g.fillStyle = 'rgba(255,255,255,.18)'; g.font = `800 22px ${FONT}`; g.fillText('GREGAL ARENA', 800, 120);
    g.fillStyle = '#06070b'; g.fillRect(-1200, -600, 4000, 640);
    g.fillStyle = '#0d0f14'; g.fillRect(-1200, 36, 4000, 30);
  },

  paintBoothFront(g, team) {
    const c = this.teamColor(team), ps = ((this.meta.teams || [])[team] || {}).players || [];
    for (let i = 0; i < 5; i++) {
      const x = 240 + i * 280;
      const mg = g.createLinearGradient(0, 600, 0, 760); mg.addColorStop(0, '#23262e'); mg.addColorStop(1, '#101217');
      g.fillStyle = mg; g.beginPath(); g.roundRect(x - 132, 600, 264, 160, 12); g.fill();
      g.fillStyle = rgba(c, 0.5); g.fillRect(x - 128, 601, 256, 3);
      g.fillStyle = 'rgba(255,255,255,.07)'; g.beginPath(); g.arc(x, 680, 16, 0, 6.283); g.fill();
      g.fillStyle = '#0c0d11'; g.fillRect(x - 14, 760, 28, 40);
    }
    const dg = g.createLinearGradient(0, 790, 0, 900); dg.addColorStop(0, '#1d2028'); dg.addColorStop(1, '#07080b');
    g.fillStyle = dg; g.fillRect(-1200, 790, 4000, 800);
    g.fillStyle = '#30343e'; g.fillRect(-1200, 790, 4000, 4);
    g.fillStyle = c; g.fillRect(-1200, 812, 4000, 6);
    g.textAlign = 'center'; g.textBaseline = 'middle';
    for (let i = 0; i < 5; i++) {
      const x = 240 + i * 280;
      g.fillStyle = '#05060a'; g.beginPath(); g.roundRect(x - 92, 836, 184, 40, 8); g.fill();
      g.strokeStyle = rgba(c, 0.7); g.lineWidth = 2; g.stroke();
      g.fillStyle = '#fff'; g.font = `800 22px ${FONT}`; g.fillText(ps[i] ? ps[i].handle : '', x, 857, 170);
    }
    // the glass: frame, joints and reflections
    g.fillStyle = 'rgba(190,215,255,.05)'; g.fillRect(-1200, -600, 4000, 1390);
    g.fillStyle = 'rgba(255,255,255,.045)';
    g.beginPath(); g.moveTo(180, -10); g.lineTo(330, -10); g.lineTo(60, 800); g.lineTo(-90, 800); g.closePath(); g.fill();
    g.beginPath(); g.moveTo(1130, -10); g.lineTo(1175, -10); g.lineTo(1000, 800); g.lineTo(955, 800); g.closePath(); g.fill();
    g.fillStyle = 'rgba(200,220,255,.12)'; g.fillRect(797, 66, 6, 724); g.fillRect(-1200, 66, 4000, 5);
    const r = rng(5 + team);
    for (let i = 0; i < 9; i++) { const x = r() * 1600, y = 100 + r() * 500, rr = 14 + r() * 30; const bg = g.createRadialGradient(x, y, 0, x, y, rr); bg.addColorStop(0, 'rgba(255,255,255,.07)'); bg.addColorStop(1, 'rgba(255,255,255,0)'); g.fillStyle = bg; g.fillRect(x - rr, y - rr, 2 * rr, 2 * rr); }
  },

  boothScene(g, s, st, team) {
    const c = this.teamColor(team), ck = this.clock;
    const p = Math.max(this.pulseOf(team), s.screen === 'champions' ? 0.7 + 0.3 * Math.sin(ck * 10) : 0);
    g.fillStyle = c; g.globalAlpha = 0.4 + 0.6 * p; g.fillRect(-1200, 44, 4000, 14); g.globalAlpha = 1;
    g.globalCompositeOperation = 'lighter';
    const gr = g.createLinearGradient(0, 52, 0, 420); gr.addColorStop(0, rgba(c, 0.4 * (0.3 + p))); gr.addColorStop(1, rgba(c, 0));
    g.fillStyle = gr; g.fillRect(-1200, 52, 4000, 368);
    g.globalCompositeOperation = 'source-over';
    const ps = ((this.meta.teams || [])[team] || {}).players || [];
    const modes = ['up', 'pump', 'up', 'clap', 'pump'], trim = luminance(c) > 0.6 ? '#111318' : '#ffffff';
    for (let i = 0; i < 5; i++) {
      const id = ps[i] ? ps[i].id : team + ':' + i, h = strHash(id);
      const rise = easeOut(clamp((st.t - 0.12 - i * 0.07) / 0.4, 0, 1));
      const hop = rise * Math.abs(Math.sin(ck * 6.5 + i * 1.7)) * 34;
      const x = 240 + i * 280 + Math.sin(ck * 0.8 + i) * 5, y = 470 - rise * 125 - hop;
      this.person(g, x, y, 52, { shirt: c, trim, skin: SKIN[h % 3], hair: HAIR[(h >> 3) % 3], headset: rgba(c, 1), mouth: rise > 0.2 ? 0.6 + 0.4 * Math.sin(ck * 9 + i) : 0.1, arms: rise > 0.3 ? modes[i] : 'down', i, label: this.tags()[team] });
    }
  },

  // ---- caster desk ----

  paintDeskBack(g) {
    const c0 = this.teamColor(0), c1 = this.teamColor(1), r = rng(11);
    const gr = g.createLinearGradient(0, 0, 0, VH); gr.addColorStop(0, '#060912'); gr.addColorStop(0.55, '#0c1224'); gr.addColorStop(1, '#04050a');
    g.fillStyle = gr; g.fillRect(-1200, -600, 4000, 2100);
    const sg = g.createRadialGradient(800, 160, 0, 800, 160, 420); sg.addColorStop(0, 'rgba(120,150,255,.26)'); sg.addColorStop(1, 'rgba(120,150,255,0)');
    g.fillStyle = sg; g.fillRect(380, -260, 840, 840);
    g.fillStyle = 'rgba(140,170,255,.10)'; g.beginPath(); g.roundRect(560, 60, 480, 200, 30); g.fill();
    const cols = [c0, c1, '#ffffff', '#7aa2ff', '#ffd34d'];
    for (let i = 0; i < 52; i++) {
      const x = r() * 2000 - 200, y = 40 + r() * 560, rr = 16 + r() * 46, col = cols[Math.floor(r() * cols.length)], a = 0.06 + r() * 0.14;
      const bg = g.createRadialGradient(x, y, 0, x, y, rr); bg.addColorStop(0, rgba(col, a)); bg.addColorStop(0.7, rgba(col, a * 0.6)); bg.addColorStop(1, rgba(col, 0));
      g.fillStyle = bg; g.fillRect(x - rr, y - rr, 2 * rr, 2 * rr);
    }
    g.fillStyle = 'rgba(0,0,0,.45)';
    for (let i = 0; i < 160; i++) { const x = r() * 2200 - 300, y = 560 + r() * 80, rr = 13 + r() * 9; g.beginPath(); g.arc(x, y, rr, 0, 6.283); g.fill(); g.fillRect(x - rr, y, 2 * rr, 60); }
    g.fillStyle = 'rgba(255,255,255,.05)'; g.font = `900 150px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText('GREGAL ARENA', 800, 360, 1500);
  },

  paintDeskFront(g) {
    const c0 = this.teamColor(0), c1 = this.teamColor(1);
    g.beginPath(); g.moveTo(130, 668); g.quadraticCurveTo(800, 612, 1470, 668); g.lineTo(1560, 1300); g.lineTo(40, 1300); g.closePath();
    const gr = g.createLinearGradient(0, 620, 0, 900); gr.addColorStop(0, '#1e222c'); gr.addColorStop(1, '#07080c');
    g.fillStyle = gr; g.fill();
    const lg = g.createLinearGradient(130, 0, 1470, 0); lg.addColorStop(0, c0); lg.addColorStop(0.5, '#ffd34d'); lg.addColorStop(1, c1);
    g.strokeStyle = lg; g.lineWidth = 7; g.beginPath(); g.moveTo(130, 680); g.quadraticCurveTo(800, 624, 1470, 680); g.stroke();
    g.fillStyle = '#2a2e38'; g.beginPath(); g.moveTo(130, 668); g.quadraticCurveTo(800, 612, 1470, 668); g.lineTo(1474, 676); g.quadraticCurveTo(800, 620, 126, 676); g.closePath(); g.fill();
    for (const x of [650, 950]) {
      g.fillStyle = '#2b2f38'; g.beginPath(); g.moveTo(x - 60, 640); g.lineTo(x + 60, 640); g.lineTo(x + 50, 600); g.lineTo(x - 50, 600); g.closePath(); g.fill();
      g.fillStyle = 'rgba(150,190,255,.18)'; g.fillRect(x - 46, 603, 92, 4);
    }
    g.textAlign = 'center'; g.textBaseline = 'middle';
    g.fillStyle = 'rgba(255,255,255,.9)'; g.font = `900 46px ${FONT}`; g.fillText('ANALYST DESK', 800, 760);
    g.fillStyle = 'rgba(255,255,255,.45)'; g.font = `700 20px ${FONT}`; g.fillText('GREGAL ARENA  ·  LIVE', 800, 808);
  },

  deskScene(g) {
    const ck = this.clock, speaker = Math.floor(ck / 1.9) % 3;
    const cast = [[520, '#1c2433', this.teamColor(0), 0, 2], [800, '#2a2a31', '#ffd34d', 1, 0], [1080, '#33202a', this.teamColor(1), 2, 1]];
    cast.forEach(([x, jacket, tie, sk, hr], i) => {
      const talk = i === speaker, y = 392 + Math.sin(ck * (talk ? 3.1 : 1.2) + i) * (talk ? 4 : 1.5);
      this.person(g, x, y, 50, { shirt: jacket, trim: '#eef0f4', tie, suit: true, skin: SKIN[sk], hair: HAIR[hr], headset: '#ffd34d', mouth: talk ? 0.12 + 0.3 * Math.abs(Math.sin(ck * 11 + i)) : 0.05, arms: talk ? 'talk' : 'down', i, len: 7 });
    });
  },

  // ---- trophy lift on stage ----

  paintTrophyBack(g, team) {
    const c = this.teamColor(team);
    const gr = g.createLinearGradient(0, 0, 0, VH); gr.addColorStop(0, mix(c, '#000000', 0.72)); gr.addColorStop(1, '#05060a');
    g.fillStyle = gr; g.fillRect(-1200, -600, 4000, 2100);
    for (let x = -400, i = 0; x < 2000; x += 64, i++) { g.fillStyle = rgba(c, i % 3 ? 0.05 : 0.11); g.fillRect(x + 4, 60, 56, 640); }
    const gg = g.createRadialGradient(800, 300, 0, 800, 300, 560); gg.addColorStop(0, 'rgba(255,210,90,.2)'); gg.addColorStop(1, 'rgba(255,210,90,0)');
    g.fillStyle = gg; g.fillRect(240, -260, 1120, 1120);
    g.fillStyle = 'rgba(255,211,77,.10)'; g.font = `900 italic 190px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText('CHAMPIONS', 800, 250, 1500);
    g.strokeStyle = '#2a2f3a'; g.lineWidth = 3;
    for (const y of [16, 40]) { g.beginPath(); g.moveTo(-1200, y); g.lineTo(2800, y); g.stroke(); }
    const fg = g.createLinearGradient(0, 700, 0, 900); fg.addColorStop(0, '#111522'); fg.addColorStop(1, '#05060a');
    g.fillStyle = fg; g.fillRect(-1200, 700, 4000, 800);
    g.fillStyle = rgba(c, 0.6); g.fillRect(-1200, 700, 4000, 3);
  },

  trophyScene(g, team) {
    const c = this.teamColor(team), ck = this.clock, trim = luminance(c) > 0.6 ? '#111318' : '#ffffff';
    this.beams(g, 1, 8, 20, 900);
    const ps = ((this.meta.teams || [])[team] || {}).players || [];
    const xs = [360, 580, 800, 1020, 1240], modes = ['up', 'pump', 'hold', 'pump', 'up'];
    for (const i of [0, 4, 1, 3, 2]) {
      const id = ps[i] ? ps[i].id : team + ':' + i, h = strHash(id);
      const hop = Math.abs(Math.sin(ck * 6.2 + i * 1.4)) * (i === 2 ? 12 : 26);
      const y = 420 - hop + (i === 2 ? -10 : 0), s = i === 2 ? 48 : 44;
      this.person(g, xs[i], y, s, { shirt: c, trim, skin: SKIN[h % 3], hair: HAIR[(h >> 3) % 3], mouth: 0.7 + 0.3 * Math.sin(ck * 8 + i), arms: modes[i], i, len: 12, label: this.tags()[team] });
      if (i === 2) this.drawTrophy(g, xs[i], y - 2.55 * s + 6, 1.05, 1);
    }
    this.pyro(g, 120, 900, 460, this.pyroK(0)); this.pyro(g, 1480, 900, 460, this.pyroK(0));
    this.pyro(g, 300, 900, 300, this.pyroK(0.85)); this.pyro(g, 1300, 900, 300, this.pyroK(0.85));
  },
});
