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
  for (let j = 0; j < gh; j++) for (let i = 0; i < gw; i++) {
    const x = (i + 0.5) * res, y = (j + 0.5) * res, k = j * gw + i;
    let best = 1e9, bi = -1, floor = false;
    zs.forEach((z, zi) => {
      const dx = Math.max(0, Math.abs(x - z.x) - z.w / 2), dy = Math.max(0, Math.abs(y - z.y) - z.h / 2), d = Math.hypot(dx, dy);
      if (d < best) { best = d; bi = zi; }
      if (d <= 0.3) floor = true;
    });
    if (!floor) for (const [a, b] of links) if (segDist(x, y, a.x, a.y, b.x, b.y) <= CORRIDOR) { floor = true; break; }
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
function wallPal(z) {
  if (!z) return [188, 162, 122];
  const j = (hash3(z.id.length, z.id.charCodeAt(0), 11) - 0.5) * 18;
  const base = z.kind === 'site' ? [216, 186, 140] : z.kind === 'spawn_t' ? [204, 160, 106] : z.kind === 'spawn_ct' ? [178, 172, 158] : [198, 170, 128];
  return base.map(v => v + j);
}
function shadeCol(pal, sh, fog) {
  const c = i => Math.round(clamp(pal[i] * sh * (1 - fog) + HAZE[i] * fog, 0, 255));
  return `rgb(${c(0)},${c(1)},${c(2)})`;
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

// ---- el visor ----

export class Cs2View {
  constructor(canvas, opts = {}) {
    this.cv = canvas; this.ctx = canvas.getContext('2d');
    this.lang = 'en'; // the games are always shown in English (TEXT.ca is kept for old callers)
    this.T = TEXT[this.lang];
    this.source = opts.source;
    this.onUpdate = opts.onUpdate || (() => {});
    this.onEvent = opts.onEvent || null;
    this.match = null; this.meta = null; this.events = []; this.rounds = []; this.lastN = 0;
    this.ri = 0; this.R = null; this.phase = 'wait'; this.p = 0; this.hold = 0; this.fired = 0;
    this.speed = 1; this.paused = false; this.stopped = false; this.score = [0, 0]; this.ticker = null; this.banner = null; this.overlay = null;
    this.last = performance.now(); this.mapCache = null;
    // Broadcast camera: 'map' (tactical) or 'pov' (spectator cam on one player).
    this.view = { mode: 'map', id: null, why: '' }; this.shotAge = 0; this.calm = 0; this.cutT = 0; this.cam = null;
    this.replay = null; this.replayed = false; this.grid = null; this.fdt = 0.016;
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
    this.banner = null; this.overlay = null; this.ticker = null; this.replay = null; this.view = { mode: 'map', id: null, why: '' };
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
    this.fdt = this.paused ? 0 : dt * this.speed;
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
        this.replay = null; this.replayed = false; this.view = { mode: 'map', id: null, why: '' }; this.cam = null;
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
      this.direct(dt);
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
      this.direct(dt);
      if (this.hold > 0) return;
      this.banner = null;
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
      if (me) { this.fire(me); this.ri++; this.enterFinal(); return; }
      const hf = after.find(e => e.kind === 'half');
      if (hf) { this.fire(hf); this.phase = 'half'; this.hold = 3.8; this.overlay = { kind: 'half', e: hf, t0: performance.now() }; return; }
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
    if (this.phase === 'half') { this.hold -= dt; if (this.hold <= 0) { this.overlay = null; this.ri++; this.phase = 'wait'; } }
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
    const pov = this.povOn();
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
      if (!pov && this.phase === 'play') this.mapTag();
      this.drawCut();
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
        g.fillStyle = shadeCol([150, 116, 72], 1.05, cf); g.fillRect(x, top1, cwx, top0 - top1);
        g.fillStyle = shadeCol([118, 86, 50], 0.9, cf); g.fillRect(x, top0, cwx, bot0 - top0);
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
    if (!it.dead && pr.depth < 45) { g.save(); g.globalAlpha = 0.3; this.figure(x, foot, H, q, lat, walk, 0, 0, col, true); g.restore(); }
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

  // A procedural player model: legs, body armour, head, rifle. Returns the gun tip.
  figure(x, foot, H, q, lat, walk, fall, fallDir, col, xray) {
    const g = this.ctx, s = H / BODY_H;
    g.save(); g.translate(x, foot);
    if (fall > 0) g.rotate(fallDir * easeOut(fall) * Math.PI * 0.48);
    const T = q.side === 'T';
    const cloth = xray ? col : T ? '#6f5c40' : '#2d3d56', dark = xray ? col : T ? '#4b3e2b' : '#1d283a';
    const sw = walk * 0.13 * s;
    g.fillStyle = dark;
    g.beginPath(); g.roundRect(-0.2 * s + sw, -0.86 * s, 0.17 * s, 0.86 * s, 0.05 * s); g.fill();
    g.beginPath(); g.roundRect(0.03 * s - sw, -0.86 * s, 0.17 * s, 0.86 * s, 0.05 * s); g.fill();
    g.fillStyle = cloth;
    g.beginPath(); g.roundRect(-0.26 * s, -1.46 * s, 0.52 * s, 0.68 * s, 0.1 * s); g.fill();
    if (!xray) {
      g.fillStyle = T ? '#57482f' : '#22324a'; g.fillRect(-0.24 * s, -1.38 * s, 0.48 * s, 0.34 * s); // vest
      g.fillStyle = col; g.fillRect(-0.24 * s, -1.2 * s, 0.48 * s, 0.06 * s); // team stripe
      g.strokeStyle = rgba(col, 0.9); g.lineWidth = Math.max(1, 0.035 * s);
      g.beginPath(); g.roundRect(-0.26 * s, -1.46 * s, 0.52 * s, 0.68 * s, 0.1 * s); g.stroke();
    }
    g.fillStyle = xray ? col : '#c49a74'; g.beginPath(); g.arc(0, -1.62 * s, 0.14 * s, 0, Math.PI * 2); g.fill();
    if (!xray) {
      g.fillStyle = T ? '#1c1c1c' : '#28374e';
      g.beginPath(); g.arc(0, -1.64 * s, 0.15 * s, Math.PI, Math.PI * 2); g.fill();
      if (T) g.fillRect(-0.14 * s, -1.6 * s, 0.28 * s, 0.07 * s);
    }
    // Gun: sideways when they face across the screen, foreshortened when aiming at us.
    const L = 0.62 * s * lat, gy = -1.16 * s;
    let tip;
    g.strokeStyle = xray ? col : '#1a1a1c'; g.lineCap = 'round';
    if (Math.abs(lat) > 0.3) {
      g.lineWidth = Math.max(1.5, 0.09 * s); g.beginPath(); g.moveTo(-0.08 * s * Math.sign(lat), gy); g.lineTo(L, gy - 0.03 * s); g.stroke();
      g.strokeStyle = xray ? col : cloth; g.lineWidth = Math.max(1.5, 0.1 * s); g.beginPath(); g.moveTo(-0.2 * Math.sign(lat) * s, -1.38 * s); g.lineTo(L * 0.55, gy + 0.02 * s); g.stroke();
      tip = { x: L, y: gy - 0.03 * s };
    } else {
      g.fillStyle = xray ? col : '#141416'; g.beginPath(); g.arc(L * 0.3, gy, 0.08 * s, 0, Math.PI * 2); g.fill();
      g.strokeStyle = xray ? col : cloth; g.lineWidth = Math.max(1.5, 0.1 * s);
      g.beginPath(); g.moveTo(-0.22 * s, -1.38 * s); g.lineTo(L * 0.3, gy); g.lineTo(0.22 * s, -1.38 * s); g.stroke();
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
    const ax = x0 + w * (pistol ? 0.6 : 0.66) + bobX + kick * 10 * S, ay = y0 + h - (pistol ? 46 : 34) * S + bobY + kick * 12 * S;
    const rot = (pistol ? 0.42 : 0.36) + kick * (cls === 'awp' ? 0.16 : 0.07);
    g.save(); g.translate(ax, ay); g.rotate(rot); g.scale(S, S);
    const metal = '#2a2c30', metal2 = '#45484f';
    let tip;
    const poly = (pts, fill) => { g.fillStyle = fill; g.beginPath(); pts.forEach(([a, b], i) => i ? g.lineTo(a, b) : g.moveTo(a, b)); g.closePath(); g.fill(); };
    if (pistol) {
      poly([[-14, 26], [18, 26], [70, 150], [-40, 150]], sleeve);
      poly([[-6, -2], [12, -2], [18, 36], [0, 36]], '#202124');
      g.fillStyle = wid === 'deagle' ? '#8d9096' : metal; g.beginPath(); g.roundRect(-58, -16, 76, 16, 3); g.fill();
      g.fillStyle = metal2; g.fillRect(-58, -16, 76, 3);
      g.fillStyle = '#111'; g.fillRect(-60, -12, 4, 8);
      if (wid === 'usp') { g.fillStyle = '#18191b'; g.beginPath(); g.roundRect(-104, -14, 48, 12, 4); g.fill(); tip = [-106, -8]; } else tip = [-61, -8];
      g.fillStyle = glove; g.beginPath(); g.ellipse(6, 18, 17, 15, 0, 0, Math.PI * 2); g.fill();
      g.beginPath(); g.ellipse(-6, 24, 14, 12, 0, 0, Math.PI * 2); g.fill();
    } else {
      const awp = cls === 'awp', smg = cls === 'smg';
      const body = awp ? '#3f5236' : wid === 'ak47' ? metal : wid === 'galil' ? '#3b3a33' : '#25272b';
      const wood = wid === 'ak47' ? '#7a4520' : body;
      poly([[150, 70], [40, -4], [16, 22], [100, 110]], sleeve);
      poly([[30, -10], [84, -4], [88, 22], [30, 8]], awp ? body : wood);
      g.fillStyle = body; g.fillRect(-24, -14, 58, 20);
      g.fillStyle = metal2; g.fillRect(-24, -14, 58, 3);
      if (wid === 'ak47') poly([[-8, 6], [8, 6], [18, 46], [2, 50]], '#8a4b1c');
      else if (awp) poly([[-4, 6], [10, 6], [10, 26], [-4, 26]], '#1d1f1a');
      else poly([[-8, 6], [6, 6], [8, smg ? 40 : 44], [-6, smg ? 40 : 44]], '#1d1e21');
      const hgL = smg ? -46 : -66;
      g.fillStyle = wood; g.fillRect(hgL, -12, -hgL - 24, 15);
      const bL = awp ? -160 : smg ? -66 : wid === 'm4a1s' ? -112 : -118;
      g.fillStyle = metal; g.fillRect(bL, -9, hgL - bL, 6);
      if (wid === 'm4a1s') { g.fillStyle = '#17181a'; g.beginPath(); g.roundRect(-162, -12, 52, 12, 4); g.fill(); tip = [-164, -6]; }
      else tip = [bL - 2, -6];
      if (!awp) { g.fillStyle = metal; g.fillRect(bL + 6, -17, 4, 8); g.fillRect(14, -20, 10, 6); }
      if (awp) {
        g.fillStyle = '#111214'; g.beginPath(); g.roundRect(-44, -34, 66, 14, 6); g.fill();
        g.fillStyle = '#26282c'; g.fillRect(-52, -37, 12, 20); g.fillRect(14, -36, 10, 18);
        g.fillStyle = 'rgba(120,180,255,.5)'; g.fillRect(-53, -34, 3, 14);
        g.fillStyle = metal2; g.fillRect(20, -2, 10, 4);
      }
      g.fillStyle = glove; g.beginPath(); g.ellipse(24, 12, 14, 11, 0.3, 0, Math.PI * 2); g.fill();
      poly([[hgL + 10, 2], [hgL + 34, 2], [hgL + 70, 120], [hgL - 30, 120]], sleeve);
      g.fillStyle = glove; g.beginPath(); g.ellipse(hgL + 20, 4, 15, 10, 0, 0, Math.PI * 2); g.fill();
    }
    g.restore();
    const c = Math.cos(rot), s = Math.sin(rot);
    const mx = ax + (tip[0] * c - tip[1] * s) * S, my = ay + (tip[0] * s + tip[1] * c) * S;
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

  ensureMapCache() {
    if (!this.mapCache) { const c = document.createElement('canvas'); c.width = this.W; c.height = this.H; this.renderMap(c.getContext('2d')); this.mapCache = c; }
    return this.mapCache;
  }

  povRadar(R, gt, id, C, x, y, wR) {
    const g = this.ctx, u = this.u, f = this.fit(), size = (this.meta && this.meta.size) || [100, 80];
    const hR = wR * size[1] / size[0], sc = wR / size[0];
    g.save();
    g.fillStyle = 'rgba(6,9,14,.75)'; g.beginPath(); g.roundRect(x - 3 * u, y - 3 * u, wR + 6 * u, hR + 6 * u, 6 * u); g.fill();
    g.beginPath(); g.roundRect(x, y, wR, hR, 4 * u); g.clip();
    g.globalAlpha = 0.9; g.drawImage(this.ensureMapCache(), f.ox, f.oy, size[0] * f.s, size[1] * f.s, x, y, wR, hR); g.globalAlpha = 1;
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

  mapTag() {
    const g = this.ctx, u = this.u, b = this.box;
    g.font = `900 ${10.5 * u}px ${FONT}`;
    const t = 'TACTICAL VIEW', tw = g.measureText(t).width + 30 * u;
    g.fillStyle = 'rgba(6,9,14,.75)'; g.beginPath(); g.roundRect(b.x + 8 * u, b.y + 8 * u, tw, 22 * u, 11 * u); g.fill();
    g.fillStyle = '#ffd34d'; g.beginPath(); g.arc(b.x + 19 * u, b.y + 19 * u, 3.5 * u, 0, Math.PI * 2); g.fill();
    g.fillStyle = 'rgba(255,255,255,.85)'; g.textAlign = 'left'; g.textBaseline = 'middle'; g.fillText(t, b.x + 27 * u, b.y + 19.5 * u);
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
    const y = this.povOn() ? this.box.y + (this.narrow ? 40 : 10) * u : this.box.y + this.box.h - 30 * u, x = (this.W - tw) / 2;
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
