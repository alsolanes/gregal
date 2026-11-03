// Splash Strike (inspired by Counter-Strike 2) in first-person 3D (arena-3d.js adapter).
//
// Everything comes from the 2D viewer (arena-cs2-view.js, Cs2View): the round
// being played (view.R from buildRound), its play time view.p (or the splash-cam
// replay's game time), the score, the director's POV choice and the ticker.
// This file only turns that state into a place: the stylised Mirage of the
// tactical map and the raycast POV (the same zones, corridors, crates and plant
// spots, through the same occupancy grid, buildGrid) as sandstone walls with
// varied heights, painted floors, crates, arches and doors, site letters, sky
// and sun; ten players in their side's colours with toy water blasters, posed
// from the tracks (walking, crouching, aiming, soaked and sitting down); water
// streams and spray puffs, fog clouds, soap slicks, foam flashes, water
// balloons, the Mega Balloon (carried, planted, its light, the burst) and the
// first-person view: a blaster in your hands, a crosshair, the foam whiteout,
// the scope, health, money and the splash feed.
//
// Coordinates: the map is x right, y down (0..100 × 0..80), ~1 unit = 1 m. The
// world is Y-up with the map's centre at the origin: mapToWorld(x, y, h) =
// [x - 50, h, y - 40], so seen from above it is the radar, not mirrored. A map
// heading a (atan2(dy, dx)) is the world yaw -a (see arena-3d.js).

import {
  buildGrid, castRay, cellAt, posAt, povAim, utilState, bombState, shotsAt, feedAt, aliveAt, gameTimeAt, playTimeAt,
  hudStats, weaponClass, sees, fmtClock, money, EYE, CRATE_H, TEXT,
} from './arena-cs2-view.js';
import { clamp, lerp, rng, easeOut, esc } from './arena-view-base.js';
import { dampAngle, wrapAngle } from './arena-3d.js';

export const SIDE_COLOR = { CT: '#5aa9e6', T: '#e8a33d' };
const VEST = { CT: '#2f68a8', T: '#c27a2c' }, UNIFORM = { CT: '#3f4c5e', T: '#7f6c4b' };
const SLEEVE = { CT: '#33476a', T: '#7a6446' }, GLOVE = { CT: '#1b222d', T: '#3b3226' };
const WEAPON = { glock: 'Squirt-18', usp: 'Squirt-S', p250: 'Spritz 250', deagle: 'Desert Geyser', mac10: 'Drizzle-10', mp9: 'Mist-9', galil: 'Gale Soaker', famas: 'Fountain Soaker', ak47: 'Hydro-47', m4a1s: 'Hydro-M4', awp: 'AWP Aqua Jet' };
const TEX_M = 3.2; // metres covered by one tile of the wall texture
const HAZE = '#e2d2b2';
const SUN_ANG = -2.2; // the raycaster's sun (map angle)
const BOMB_LEFT = 40;

// ---------------------------------------------------------------------------
// Pure helpers (tested in arena-3d-cs2.test.cjs)
// ---------------------------------------------------------------------------
export const MAP_SIZE = [100, 80];
export function mapToWorld(x, y, h = 0, size = MAP_SIZE) { return [x - size[0] / 2, h, y - size[1] / 2]; }
export function worldToMap(p, size = MAP_SIZE) { return { x: p[0] + size[0] / 2, y: p[2] + size[1] / 2, h: p[1] }; }
export function mapYaw(a) { return wrapAngle(-a); }
const hashN = (a, b = 0) => { let h = (a * 374761393 + b * 668265263) | 0; h = (h ^ (h >>> 13)) * 1274126177 | 0; return ((h ^ (h >>> 16)) >>> 0) / 4294967296; };
const strHash = s => { let h = 7; for (const ch of String(s || '')) h = (h * 31 + ch.charCodeAt(0)) >>> 0; return h; };

// Wall height at a map point (0 on the floor and on crates).
export function heightAt(G, x, y) {
  const i = Math.floor(x / G.res), j = Math.floor(y / G.res);
  if (i < 0 || j < 0 || i >= G.gw || j >= G.gh) return 6;
  const k = j * G.gw + i;
  return G.cells[k] === 0 ? G.hgt[k] / 10 : 0;
}

// The nearest floor point (players hug walls, and the grid is coarse): the
// eye and the bodies are never inside a wall.
export function unstick(G, x, y) {
  if (cellAt(G, x, y) !== 0) return { x, y };
  for (let r = G.res; r <= 3; r += G.res / 2) {
    for (let k = 0; k < 16; k++) {
      const a = k / 16 * Math.PI * 2, px = x + Math.cos(a) * r, py = y + Math.sin(a) * r;
      if (cellAt(G, px, py) !== 0) return { x: px, y: py };
    }
  }
  return { x, y };
}

// Sandstone, tinted by the kind of zone a wall belongs to (as the raycaster).
function palOf(z) {
  if (!z) return [188, 162, 122];
  const j = (hashN(z.id.length * 31 + z.id.charCodeAt(0), 11) - 0.5) * 18;
  const base = z.kind === 'site' ? [216, 186, 140] : z.kind === 'spawn_t' ? [204, 160, 106] : z.kind === 'spawn_ct' ? [178, 172, 158] : [198, 170, 128];
  return base.map(v => clamp(v + j, 0, 255) / 255);
}

// The walls of the occupancy grid as a mesh (marching squares on the cell
// centres: diagonal corridors get diagonal walls, not stairs). Floor is open
// (the floor plane shows), walls are solid blocks with flat roofs whose height
// is the grid's (a varied skyline), with steps between roofs and an outer skirt.
// Returns plain arrays: { pos, nor, uv, col, faces: [{ ax, ay, bx, by, h, nx, ny }] }
// (faces: the walls that face the floor, for tests).
export function buildMapMesh(G) {
  const res = G.res, gw = G.gw, gh = G.gh, ox = G.w / 2, oy = G.h / 2;
  const PW = gw + 2, PH = gh + 2, SW = PW - 1, SH = PH - 1;
  const ci = (I, J) => (clamp(J - 1, 0, gh - 1)) * gw + clamp(I - 1, 0, gw - 1);
  const wallAt = (I, J) => { const i = I - 1, j = J - 1; if (i < 0 || j < 0 || i >= gw || j >= gh) return true; return G.cells[j * gw + i] === 0; };
  const cx = I => (I - 0.5) * res, cy = J => (J - 0.5) * res;
  const pal = new Map();
  const tintAt = (I, J) => { const zi = G.zone[ci(I, J)]; let p = pal.get(zi); if (!p) { p = palOf(zi >= 0 ? G.zones[zi] : null); pal.set(zi, p); } return p; };
  // Square cases and heights.
  const sqC = new Uint8Array(SW * SH), sqH = new Float32Array(SW * SH);
  const W = new Uint8Array(PW * PH);
  for (let J = 0; J < PH; J++) for (let I = 0; I < PW; I++) W[J * PW + I] = wallAt(I, J) ? 1 : 0;
  const w = (I, J) => W[J * PW + I];
  for (let J = 0; J < SH; J++) for (let I = 0; I < SW; I++) {
    const c = w(I, J) | w(I + 1, J) << 1 | w(I + 1, J + 1) << 2 | w(I, J + 1) << 3;
    let h = 0;
    if (c & 1) h = Math.max(h, G.hgt[ci(I, J)]);
    if (c & 2) h = Math.max(h, G.hgt[ci(I + 1, J)]);
    if (c & 4) h = Math.max(h, G.hgt[ci(I + 1, J + 1)]);
    if (c & 8) h = Math.max(h, G.hgt[ci(I, J + 1)]);
    sqC[J * SW + I] = c; sqH[J * SW + I] = h / 10;
  }
  const pos = [], nor = [], uv = [], col = [], faces = [];
  const vtx = (x, y, z, n, u, v, t, k) => { pos.push(x - ox, y, z - oy); nor.push(n[0], n[1], n[2]); uv.push(u, v); col.push(t[0] * k, t[1] * k, t[2] * k); };
  // A flat convex roof polygon (map points) at height h.
  const UP = [0, 1, 0];
  const roof = (poly, h, t) => {
    // Normal +Y: (b-a)×(c-a) must have y > 0, i.e. a negative shoelace area in map (x, y).
    let area = 0;
    for (let i = 0; i < poly.length; i++) { const a = poly[i], b = poly[(i + 1) % poly.length]; area += a[0] * b[1] - b[0] * a[1]; }
    const P = area > 0 ? poly.slice().reverse() : poly;
    for (let i = 1; i + 1 < P.length; i++) for (const p of [P[0], P[i], P[i + 1]]) vtx(p[0], h, p[1], UP, p[0] / TEX_M, p[1] / TEX_M, t, .97);
  };
  // A vertical quad on the map segment P-Q from y0 to y1, facing (nx, ny).
  const ao = y => .6 + .4 * clamp(y / 2.4, 0, 1);
  const wall = (P, Q, y0, y1, nx, ny, t, k0 = 1, k1 = 1) => {
    // Order the segment so that (Q-P) × up points along n: with x right, y
    // down, z up… in world terms the face (A,B,C) normal is (B-A)×(D-A).
    let a = P, b = Q;
    const dx = b[0] - a[0], dy = b[1] - a[1];
    // (B-A) = (dx, 0, dy), (D-A) = (0, 1, 0): cross = (-dy, 0, dx). Want it along (nx, 0, ny).
    if (-dy * nx + dx * ny < 0) { a = Q; b = P; }
    const L = Math.hypot(b[0] - a[0], b[1] - a[1]) || 1, ux = (b[0] - a[0]) / L, uy = (b[1] - a[1]) / L;
    const ua = (a[0] * ux + a[1] * uy) / TEX_M, ub = ua + L / TEX_M;
    const n = [nx, 0, ny];
    const v0 = y0 / TEX_M, v1 = y1 / TEX_M, c0 = ao(y0) * k0, c1 = ao(y1) * k1;
    vtx(a[0], y0, a[1], n, ua, v0, t, c0); vtx(b[0], y0, b[1], n, ub, v0, t, c0); vtx(b[0], y1, b[1], n, ub, v1, t, c1);
    vtx(a[0], y0, a[1], n, ua, v0, t, c0); vtx(b[0], y1, b[1], n, ub, v1, t, c1); vtx(a[0], y1, a[1], n, ua, v1, t, c1);
  };
  // A wall that faces the floor: plaster up to a lighter coping band.
  const facing = (P, Q, h, nx, ny, t) => {
    const band = Math.min(.32, h * .1);
    wall(P, Q, 0, h - band, nx, ny, t);
    wall(P, Q, h - band, h, nx, ny, t, 1.14 / ao(h - band), 1.14 / ao(h));
    faces.push({ ax: P[0], ay: P[1], bx: Q[0], by: Q[1], h, nx, ny });
  };
  const run = { on: false };
  const flushRun = () => { if (!run.on) return; roof([[run.x0, run.y0], [run.x1, run.y0], [run.x1, run.y1], [run.x0, run.y1]], run.h, run.t); run.on = false; };
  for (let J = 0; J < SH; J++) {
    for (let I = 0; I < SW; I++) {
      const c = sqC[J * SW + I];
      if (c === 0) { flushRun(); continue; }
      const h = sqH[J * SW + I];
      const t = tintAt(c & 1 ? I : c & 2 ? I + 1 : c & 4 ? I + 1 : I, c & 1 || c & 2 ? J : J + 1);
      const C = [[cx(I), cy(J)], [cx(I + 1), cy(J)], [cx(I + 1), cy(J + 1)], [cx(I), cy(J + 1)]];
      if (c === 15) {
        if (run.on && run.h === h && run.t === t && Math.abs(run.x1 - C[0][0]) < 1e-6) run.x1 = C[1][0];
        else { flushRun(); Object.assign(run, { on: true, x0: C[0][0], x1: C[1][0], y0: C[0][1], y1: C[2][1], h, t }); }
      } else {
        flushRun();
        const M = [0, 1, 2, 3].map(e => [(C[e][0] + C[(e + 1) % 4][0]) / 2, (C[e][1] + C[(e + 1) % 4][1]) / 2]);
        const mid = [(C[0][0] + C[2][0]) / 2, (C[0][1] + C[2][1]) / 2];
        const toward = (A, B, F) => {
          let nx = -(B[1] - A[1]), ny = B[0] - A[0];
          const L = Math.hypot(nx, ny) || 1; nx /= L; ny /= L;
          const mx = (A[0] + B[0]) / 2, my = (A[1] + B[1]) / 2;
          if (nx * (F[0] - mx) + ny * (F[1] - my) < 0) { nx = -nx; ny = -ny; }
          return [nx, ny];
        };
        if (c === 5 || c === 10) {
          // Saddle: two separate wall corners (the floor stays connected).
          const pairs = c === 5 ? [[0, M[0], M[3]], [2, M[1], M[2]]] : [[1, M[0], M[1]], [3, M[2], M[3]]];
          for (const [k, A, B] of pairs) {
            roof([C[k], A, B], h, t);
            const n = toward(A, B, mid);
            facing(A, B, h, n[0], n[1], t);
          }
        } else {
          const poly = [], cut = [];
          for (let k = 0; k < 4; k++) {
            const wk = (c >> k) & 1, wn = (c >> ((k + 1) % 4)) & 1;
            if (wk) poly.push(C[k]);
            if (wk !== wn) { poly.push(M[k]); cut.push(M[k]); }
          }
          roof(poly, h, t);
          let fx = 0, fy = 0, fn = 0;
          for (let k = 0; k < 4; k++) if (!((c >> k) & 1)) { fx += C[k][0]; fy += C[k][1]; fn++; }
          const n = toward(cut[0], cut[1], [fx / fn, fy / fn]);
          facing(cut[0], cut[1], h, n[0], n[1], t);
        }
      }
      // Steps between roofs: with the square on the right and the one below.
      const step = (I2, J2, ka, kb, me, nx, ny) => {
        if (I2 >= SW || J2 >= SH) return;
        const c2 = sqC[J2 * SW + I2]; if (!c2) return;
        const h2 = sqH[J2 * SW + I2]; if (Math.abs(h2 - h) < .01) return;
        const wa = (c >> ka) & 1, wb = (c >> kb) & 1; if (!wa && !wb) return;
        const Mm = [(C[ka][0] + C[kb][0]) / 2, (C[ka][1] + C[kb][1]) / 2];
        const P = wa ? C[ka] : Mm, Q = wb ? C[kb] : Mm;
        const lower = h2 < h ? 1 : -1;
        wall(P, Q, Math.min(h, h2), Math.max(h, h2), nx * lower, ny * lower, t, .85 / ao(Math.min(h, h2)), .95 / ao(Math.max(h, h2)));
        void me;
      };
      step(I + 1, J, 1, 2, 'r', 1, 0);
      step(I, J + 1, 3, 2, 'b', 0, 1);
      // The outer skirt round the padded border.
      if (I === 0 && (c & 1) && (c & 8)) wall(C[0], C[3], 0, h, -1, 0, t, .8, .9);
      if (I === SW - 1 && (c & 2) && (c & 4)) wall(C[1], C[2], 0, h, 1, 0, t, .8, .9);
      if (J === 0 && (c & 1) && (c & 2)) wall(C[0], C[1], 0, h, 0, -1, t, .8, .9);
      if (J === SH - 1 && (c & 4) && (c & 8)) wall(C[3], C[2], 0, h, 0, 1, t, .8, .9);
    }
    flushRun();
  }
  return { pos, nor, uv, col, faces };
}

// Crates: each prop of the map split into crate-sized boxes (the collision
// stays the prop's footprint at CRATE_H, like the raycaster).
export function cratesOf(meta) {
  const out = [];
  for (const p of (meta && meta.props) || []) {
    const nx = Math.max(1, Math.round(p.w / 1.3)), nz = Math.max(1, Math.round(p.h / 1.3));
    const bw = p.w / nx, bd = p.h / nz;
    for (let i = 0; i < nx; i++) for (let j = 0; j < nz; j++) {
      out.push({ x: p.x - p.w / 2 + bw * (i + .5), y: p.y - p.h / 2 + bd * (j + .5), w: bw - .05, d: bd - .05, h: CRATE_H, seed: strHash(`${p.x},${p.y},${i},${j}`) });
    }
  }
  return out;
}

// Where a corridor leaves the rectangle of zone z (from its centre towards d).
function exitParam(z, dx, dy) {
  const tx = dx ? (z.w / 2) / Math.abs(dx) : Infinity, ty = dy ? (z.h / 2) / Math.abs(dy) : Infinity;
  return Math.min(tx, ty);
}
// Arches (and some doors) where corridors enter the zones: a centre on the
// floor, both pillars inside walls. [{ x, y, ang, door }]
const DOORS = new Set(['palace', 'b_apps', 'market', 'window']);
export function archesOf(meta, G) {
  const byId = {}; for (const z of (meta && meta.zones) || []) byId[z.id] = z;
  const out = [];
  const ok = (x, y, ang) => {
    const lx = -Math.sin(ang), ly = Math.cos(ang);
    return cellAt(G, x, y) !== 0 && cellAt(G, x + lx * 3.05, y + ly * 3.05) === 0 && cellAt(G, x - lx * 3.05, y - ly * 3.05) === 0;
  };
  for (const [a, b] of (meta && meta.links) || []) {
    const za = byId[a], zb = byId[b];
    if (!za || !zb) continue;
    const dx = zb.x - za.x, dy = zb.y - za.y, L = Math.hypot(dx, dy);
    if (L < 1) continue;
    const ux = dx / L, uy = dy / L, ang = Math.atan2(dy, dx);
    const ta = exitParam(za, ux, uy), tb = L - exitParam(zb, -ux, -uy);
    if (tb - ta < 3) continue;
    const door = DOORS.has(a) || DOORS.has(b);
    const cands = [tb - 1.2];
    if (tb - ta > 14) cands.push(ta + 1.2);
    for (const t of cands) {
      const x = za.x + ux * t, y = za.y + uy * t;
      if (ok(x, y, ang)) { out.push({ x, y, ang, door, link: a + '-' + b }); continue; }
      // Corridors that bend into a wall: slide a little towards the middle.
      for (const d of [1, 2, 3]) {
        const t2 = t + (t > (ta + tb) / 2 ? -d : d), x2 = za.x + ux * t2, y2 = za.y + uy * t2;
        if (ok(x2, y2, ang)) { out.push({ x: x2, y: y2, ang, door, link: a + '-' + b }); break; }
      }
    }
  }
  return out;
}

// The painted site letters: on the wall nearest each plant spot (as the
// raycaster), flat on that wall, facing the site. [{ site, x, y, nx, ny }]
export function siteSignsOf(meta, G) {
  const out = [];
  for (const [site, pt] of Object.entries((meta && meta.plant) || {})) {
    let best = null;
    for (let i = 0; i < 48; i++) {
      const a = i / 48 * Math.PI * 2, dx = Math.cos(a), dy = Math.sin(a), h = castRay(G, pt[0], pt[1], dx, dy, 30);
      if (h.hit && h.d > 4 && (!best || h.d < best.d)) best = { d: h.d, h, dx, dy };
    }
    if (!best) continue;
    // The face crossed: side 0 = a wall across x, side 1 = across y.
    const nx = best.h.side === 0 ? -Math.sign(best.dx) : 0, ny = best.h.side === 1 ? -Math.sign(best.dy) : 0;
    out.push({ site, x: best.h.hx + nx * .04, y: best.h.hy + ny * .04, nx, ny, d: best.d });
  }
  return out;
}

// The round's times: game seconds (gt) and play seconds (p); in the kill-cam
// replay, the replay's own clock.
export function timesOf(v) {
  const R = v && v.R;
  if (!R || !R.tl) return null;
  const rp = v.phase === 'replay' && v.replay ? v.replay : null;
  const gt = rp ? rp.g : gameTimeAt(R.tl, v.p || 0);
  const p = rp ? playTimeAt(R.tl, rp.g) : (v.p || 0);
  return { R, gt, p, rp };
}

const DEAD_KEEP = 2.5; // game seconds a fallen player can still be watched
const aliveish = (R, id, gt, keep = 0) => { const q = R.players[id]; return !!q && gt < q.deathT + keep; };
// Who can be watched: both teams in the match's order, dry (or just soaked).
export function rosterOf(R, meta, gt) {
  const out = [], seen = new Set();
  const order = [];
  for (const t of (meta && meta.teams) || []) for (const p of t.players || []) order.push(p.id);
  for (const id of Object.keys(R.players)) if (!order.includes(id)) order.push(id);
  for (const id of order) {
    const q = R.players[id];
    if (!q || seen.has(id) || !aliveish(R, id, gt, DEAD_KEEP)) continue;
    seen.add(id);
    out.push({ id, name: q.handle, role: `${q.side}${q.role ? ' · ' + q.role : ''}`, team: q.team, color: SIDE_COLOR[q.side] || '#ccc' });
  }
  return out;
}

// The automatic subject: the kill-cam's killer; the 2D director's POV (the
// killer about to get a kill, the planter, the defuser, the clutch player);
// else whoever is defusing or planting, the bomb carrier, the player already
// followed, the CT nearest the planted bomb, then anybody alive.
export function autoPick(R, p, gt, { director = null, replay = null, prev = null } = {}) {
  if (replay && replay.k && R.players[replay.k.killer]) return replay.k.killer;
  if (director && director.mode === 'pov' && aliveish(R, director.id, gt, 1.5)) return director.id;
  const b = bombState(R, gt);
  if (b.defusing && aliveish(R, b.defusing.id, gt)) return b.defusing.id;
  if (b.planting && aliveish(R, b.planting.id, gt)) return b.planting.id;
  if (b.carrier && aliveish(R, b.carrier, gt)) return b.carrier;
  if (prev && aliveish(R, prev, gt, 1.5)) return prev;
  const alive = Object.values(R.players).filter(q => gt < q.deathT);
  if (b.planted && !b.exploded && !b.defused) {
    let best = null, bd = Infinity;
    for (const q of alive) { if (q.side !== 'CT') continue; const at = posAt(q.track, gt), d = Math.hypot(at.x - b.x, at.y - b.y); if (d < bd) { bd = d; best = q.id; } }
    if (best) return best;
  }
  const t = alive.find(q => q.side === 'T');
  return t ? t.id : alive.length ? alive[0].id : null;
}

// Cumulative distance walked along a track (seek-proof walk cycles).
const CUM = new WeakMap();
export function trackDist(track, t) {
  if (!track || track.length < 2) return 0;
  let c = CUM.get(track);
  if (!c || c.n !== track.length) {
    c = { n: track.length, d: [0] };
    for (let i = 1; i < track.length; i++) c.d.push(c.d[i - 1] + Math.hypot(track[i].x - track[i - 1].x, track[i].y - track[i - 1].y));
    CUM.set(track, c);
  }
  if (t <= track[0].t) return 0;
  for (let i = 1; i < track.length; i++) {
    const b = track[i];
    if (t <= b.t) { const a = track[i - 1], k = b.t > a.t ? (t - a.t) / (b.t - a.t) : 1; return c.d[i - 1] + (c.d[i] - c.d[i - 1]) * k; }
  }
  return c.d[c.d.length - 1];
}
export function speedAt(q, gt) {
  const t1 = Math.min(gt + .12, q.deathT), t0 = Math.min(Math.max(0, gt - .12), q.deathT);
  if (t1 - t0 < .01) return 0;
  const a = posAt(q.track, t0), b = posAt(q.track, t1);
  return Math.hypot(b.x - a.x, b.y - a.y) / (t1 - t0);
}
// How crouched a player is (0..1): planting and defusing; some hold their angle crouched in a duel.
export function crouchOf(R, id, gt, bomb = bombState(R, gt), speed = 0) {
  const ramp = (t0, t1) => clamp(Math.min((gt - t0) / .3 + 1, (t1 - gt) / .3 + 1), 0, 1);
  let c = 0;
  if (R.plant && R.plant.id === id) c = Math.max(c, ramp(R.plant.start, R.plant.t));
  if (R.defuse && R.defuse.id === id) c = Math.max(c, ramp(R.defuse.start, R.defuse.t));
  if (speed < .8) for (const k of R.kills) if (k.killer === id && hashN(k.n || 1, 77) < .4) c = Math.max(c, ramp(k.t - 1, k.t + .5) * .85);
  void bomb;
  return c;
}

// The first-person eye of player id at game time gt (play time p): at eye
// height (lower crouched, on the floor once dead), looking where povAim looks
// (pre-aiming the victim, at the killer, down at the bomb), pitched at the
// body being shot. { x, y, h, yaw, pitch, crouch, dead, deathK }
export function eyeOf(R, id, gt, G = null) {
  const q = R.players[id];
  if (!q) return null;
  const dead = gt >= q.deathT, deathK = dead ? clamp((gt - q.deathT) / .6, 0, 1) : 0;
  let at = dead ? (q.death || posAt(q.track, q.deathT)) : posAt(q.track, gt);
  if (G) at = unstick(G, at.x, at.y);
  const sp = dead ? 0 : speedAt(q, gt), cr = dead ? 0 : crouchOf(R, id, gt, undefined, sp);
  let h = lerp(EYE, 1.1, cr);
  h = lerp(h, .32, easeOut(deathK));
  const tAim = Math.min(gt, q.deathT);
  const aim = povAim(R, id, tAim) + deathK * .25;
  let pitch = 0;
  // Planting or draining: looking down at the balloon.
  if ((R.plant && R.plant.id === id && gt >= R.plant.start - .4 && gt <= R.plant.t + .3) || (R.defuse && R.defuse.id === id && gt >= R.defuse.start - .5 && gt <= R.defuse.t + .3)) pitch = -.62;
  else {
    // A duel: at the chest (or the head) of whoever is being shot.
    for (const k of R.kills) {
      const dt = k.t - tAim;
      let other = null, w = 0, tz = 1.3;
      if (k.killer === id && dt > -.8 && dt < 2.4) { other = k.victim; w = dt <= 0 ? 1 : clamp(1 - (dt - .35) / 2, 0, 1); tz = k.hs ? 1.6 : 1.3; }
      else if (k.victim === id && dt > -.5 && dt < .9) { other = k.killer; w = dt <= 0 ? 1 : clamp(1 - dt / .9, 0, 1); }
      if (!other || !w) continue;
      const o = R.players[other], op = o ? posAt(o.track, Math.min(tAim, o.deathT)) : { x: k.kx, y: k.ky };
      const d = Math.max(1, Math.hypot(op.x - at.x, op.y - at.y));
      const oc = o ? crouchOf(R, other, tAim) : 0;
      pitch = Math.max(-.5, Math.min(.4, Math.atan2(tz * (1 - oc * .3) - h, d))) * w;
      break;
    }
  }
  pitch = lerp(pitch, -.35, deathK);
  return { x: at.x, y: at.y, h, yaw: mapYaw(aim), pitch, crouch: cr, dead, deathK };
}

// ---------------------------------------------------------------------------
// Textures
// ---------------------------------------------------------------------------
function paintWall(g, W, H) {
  const R = rng(5);
  g.fillStyle = '#efe4cf'; g.fillRect(0, 0, W, H);
  // Large plaster blotches.
  for (let i = 0; i < 18; i++) {
    const x = R() * W, y = R() * H, r = 40 + R() * 110, gr = g.createRadialGradient(x, y, 0, x, y, r);
    const l = R() < .5;
    gr.addColorStop(0, l ? 'rgba(255,250,238,.3)' : 'rgba(150,120,80,.07)'); gr.addColorStop(1, 'rgba(0,0,0,0)');
    g.fillStyle = gr; g.fillRect(x - r, y - r, 2 * r, 2 * r);
  }
  // Blocks: 4 a row, 8 rows (0.8 × 0.4 m), staggered; some show through the plaster.
  const bw = W / 4, bh = H / 8;
  for (let r = 0; r < 8; r++) for (let c = -1; c < 5; c++) {
    const x = c * bw + (r & 1 ? bw / 2 : 0), y = r * bh, j = R();
    if (j < .55) { g.fillStyle = `rgba(${j < .25 ? '120,92,58' : '255,248,232'},${.06 + R() * .1})`; g.fillRect(x + 2, y + 2, bw - 4, bh - 4); }
    if (R() < .75) {
      g.fillStyle = 'rgba(96,72,44,.32)'; g.fillRect(x, y, bw, 3); g.fillRect(x, y, 3, bh);
      g.fillStyle = 'rgba(255,250,236,.22)'; g.fillRect(x + 3, y + 3, bw - 6, 2);
    }
  }
  // Grain and pits.
  for (let i = 0; i < 5000; i++) { g.fillStyle = R() < .5 ? `rgba(90,66,40,${.05 + R() * .08})` : `rgba(255,255,245,${.05 + R() * .07})`; g.fillRect(R() * W, R() * H, 1 + R() * 2, 1 + R() * 2); }
  // Grime at the bottom of the tile (the base of every wall).
  const gb = g.createLinearGradient(0, H, 0, H * .78); gb.addColorStop(0, 'rgba(80,60,36,.28)'); gb.addColorStop(1, 'rgba(80,60,36,0)');
  g.fillStyle = gb; g.fillRect(0, H * .78, W, H * .22);
}

function paintFloor(g, W, H, meta) {
  const size = meta.size || MAP_SIZE, k = W / size[0], X = x => x * k, Y = y => y * k;
  const R = rng(17);
  g.fillStyle = '#a99270'; g.fillRect(0, 0, W, H);
  const byId = {}; for (const z of meta.zones || []) byId[z.id] = z;
  // Corridors: packed sand.
  g.lineCap = 'round'; g.lineJoin = 'round';
  for (const [wd, c] of [[6.2, '#b9a17b'], [5.2, '#c2aa82']]) {
    g.strokeStyle = c; g.lineWidth = wd * k;
    for (const [a, b] of meta.links || []) { const za = byId[a], zb = byId[b]; if (!za || !zb) continue; g.beginPath(); g.moveTo(X(za.x), Y(za.y)); g.lineTo(X(zb.x), Y(zb.y)); g.stroke(); }
  }
  // Zones: stone paving by kind.
  for (const z of meta.zones || []) {
    const x = X(z.x - z.w / 2 - .4), y = Y(z.y - z.h / 2 - .4), w = (z.w + .8) * k, h = (z.h + .8) * k;
    g.fillStyle = z.kind === 'site' ? '#ccb48b' : z.kind === 'spawn_t' ? '#c19e70' : z.kind === 'spawn_ct' ? '#bdb5a4' : '#c5ad85';
    g.fillRect(x, y, w, h);
    g.save(); g.beginPath(); g.rect(x, y, w, h); g.clip();
    const tile = z.kind === 'site' ? 1.6 : z.kind.startsWith('spawn') ? 1.0 : 1.25;
    g.strokeStyle = 'rgba(92,70,42,.22)'; g.lineWidth = Math.max(1, .05 * k);
    for (let yy = Math.floor((z.y - z.h / 2) / tile) * tile; yy < z.y + z.h / 2 + tile; yy += tile) {
      g.beginPath(); g.moveTo(x, Y(yy)); g.lineTo(x + w, Y(yy)); g.stroke();
      const off = (Math.round(yy / tile) & 1) ? tile / 2 : 0;
      for (let xx = Math.floor((z.x - z.w / 2) / tile) * tile + off; xx < z.x + z.w / 2 + tile; xx += tile) { g.beginPath(); g.moveTo(X(xx), Y(yy)); g.lineTo(X(xx), Y(yy + tile)); g.stroke(); }
    }
    for (let i = 0; i < z.w * z.h * .4; i++) { g.fillStyle = `rgba(${R() < .5 ? '255,250,235' : '90,68,40'},${.05 + R() * .06})`; g.fillRect(x + R() * w, y + R() * h, tile * k * (.9 + R() * .1), tile * k * (.9 + R() * .1)); }
    g.restore();
  }
  // Dust, stains and pebbles everywhere.
  for (let i = 0; i < 22000; i++) { g.fillStyle = R() < .5 ? `rgba(80,60,36,${.04 + R() * .06})` : `rgba(255,252,240,${.03 + R() * .05})`; const s = 1 + R() * 2.2; g.fillRect(R() * W, R() * H, s, s); }
  for (let i = 0; i < 70; i++) { const x = R() * W, y = R() * H, r = (1 + R() * 3) * k, gr = g.createRadialGradient(x, y, 0, x, y, r); gr.addColorStop(0, 'rgba(70,52,30,.14)'); gr.addColorStop(1, 'rgba(70,52,30,0)'); g.fillStyle = gr; g.fillRect(x - r, y - r, 2 * r, 2 * r); }
  // Plant zones: a faded red paint ring.
  for (const pt of Object.values(meta.plant || {})) {
    g.save(); g.setLineDash([.7 * k, .45 * k]); g.strokeStyle = 'rgba(160,40,26,.55)'; g.lineWidth = .18 * k;
    g.beginPath(); g.arc(X(pt[0]), Y(pt[1]), 2.6 * k, 0, Math.PI * 2); g.stroke(); g.restore();
  }
}

function paintCrate(g, W, H) {
  g.fillStyle = '#8b6439'; g.fillRect(0, 0, W, H);
  const R = rng(3);
  for (let i = 0; i < 6; i++) { g.fillStyle = `rgba(${R() < .5 ? '60,38,18' : '170,130,80'},${.12 + R() * .1})`; g.fillRect(0, i * H / 6, W, H / 6 - 2); }
  g.fillStyle = 'rgba(40,24,10,.5)'; for (let i = 1; i < 6; i++) g.fillRect(0, i * H / 6 - 2, W, 2);
  // Frame and brace.
  g.fillStyle = '#6e4a26'; g.fillRect(0, 0, W, 12); g.fillRect(0, H - 12, W, 12); g.fillRect(0, 0, 12, H); g.fillRect(W - 12, 0, 12, H);
  g.strokeStyle = '#6e4a26'; g.lineWidth = 11; g.beginPath(); g.moveTo(10, H - 10); g.lineTo(W - 10, 10); g.stroke();
  g.fillStyle = 'rgba(20,12,4,.35)'; for (const [x, y] of [[6, 6], [W - 6, 6], [6, H - 6], [W - 6, H - 6]]) { g.beginPath(); g.arc(x, y, 3, 0, Math.PI * 2); g.fill(); }
  for (let i = 0; i < 900; i++) { g.fillStyle = `rgba(${R() < .5 ? '30,18,6' : '200,160,110'},${.06 + R() * .08})`; g.fillRect(R() * W, R() * H, 1 + R() * 3, 1); }
}

function paintSky(g, W, H) {
  const gr = g.createLinearGradient(0, 0, 0, H);
  gr.addColorStop(0, '#3a6fae'); gr.addColorStop(.35, '#6f9fcc'); gr.addColorStop(.47, '#b8cfdf'); gr.addColorStop(.5, '#ead8b4'); gr.addColorStop(1, '#d9c39c');
  g.fillStyle = gr; g.fillRect(0, 0, W, H);
  // A few thin high clouds.
  const R = rng(41);
  for (let i = 0; i < 30; i++) {
    const x = R() * W, y = H * (.12 + R() * .3), rw = 30 + R() * 90, rh = 3 + R() * 6;
    const c = g.createRadialGradient(x, y, 0, x, y, rw);
    c.addColorStop(0, 'rgba(255,255,255,.28)'); c.addColorStop(1, 'rgba(255,255,255,0)');
    g.save(); g.translate(x, y); g.scale(1, rh / rw); g.translate(-x, -y); g.fillStyle = c; g.fillRect(x - rw, y - rw, 2 * rw, 2 * rw); g.restore();
  }
}

function paintSign(g, W, H, site) {
  g.clearRect(0, 0, W, H);
  const cx = W / 2, cy = H / 2, r = W * .42;
  g.strokeStyle = 'rgba(150,30,20,.9)'; g.lineWidth = W * .055;
  g.beginPath(); g.arc(cx, cy, r, 0, Math.PI * 2); g.stroke();
  g.fillStyle = 'rgba(165,35,22,.92)'; g.font = `900 ${W * .6}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.textBaseline = 'middle';
  g.fillText(site, cx, cy + W * .03);
  // Weathering: knock some paint off.
  const R = rng(site.charCodeAt(0));
  g.globalCompositeOperation = 'destination-out';
  for (let i = 0; i < 260; i++) { g.fillStyle = `rgba(0,0,0,${.2 + R() * .6})`; g.fillRect(R() * W, R() * H, 1 + R() * 5, 1 + R() * 3); }
  g.globalCompositeOperation = 'source-over';
}

function paintPuff(g, W, H) {
  g.clearRect(0, 0, W, H);
  const R = rng(9);
  for (let i = 0; i < 14; i++) {
    const a = R() * Math.PI * 2, d = R() * W * .2, x = W / 2 + Math.cos(a) * d, y = H / 2 + Math.sin(a) * d, r = W * (.18 + R() * .2);
    const gr = g.createRadialGradient(x, y, 0, x, y, r);
    gr.addColorStop(0, 'rgba(255,255,255,.34)'); gr.addColorStop(.6, 'rgba(255,255,255,.16)'); gr.addColorStop(1, 'rgba(255,255,255,0)');
    g.fillStyle = gr; g.fillRect(0, 0, W, H);
  }
  // Fade the square's edges to nothing.
  g.globalCompositeOperation = 'destination-in';
  const m = g.createRadialGradient(W / 2, H / 2, W * .25, W / 2, H / 2, W / 2);
  m.addColorStop(0, 'rgba(0,0,0,1)'); m.addColorStop(1, 'rgba(0,0,0,0)');
  g.fillStyle = m; g.fillRect(0, 0, W, H);
  g.globalCompositeOperation = 'source-over';
}

function paintGlow(g, W, H) {
  const r = g.createRadialGradient(W / 2, H / 2, 0, W / 2, H / 2, W / 2);
  r.addColorStop(0, 'rgba(255,255,255,1)'); r.addColorStop(.2, 'rgba(255,255,255,.7)'); r.addColorStop(.5, 'rgba(255,255,255,.22)'); r.addColorStop(1, 'rgba(255,255,255,0)');
  g.fillStyle = r; g.fillRect(0, 0, W, H);
}

function paintTag(g, W, H, text, color) {
  g.clearRect(0, 0, W, H);
  g.font = `800 ${H * .6}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.textBaseline = 'middle';
  g.lineWidth = H * .14; g.strokeStyle = 'rgba(0,0,0,.75)'; g.strokeText(text, W / 2, H / 2, W - 8);
  g.fillStyle = color; g.fillText(text, W / 2, H / 2, W - 8);
}

// ---------------------------------------------------------------------------
// Instanced effects
// ---------------------------------------------------------------------------
// Camera-facing quads in one draw call; per instance: centre, size, roll, alpha, colour.
function billboards(THREE, n, map, { additive = false, fog = true } = {}) {
  const geo = new THREE.PlaneGeometry(1, 1);
  const bb = new THREE.InstancedBufferAttribute(new Float32Array(n * 2), 2);
  bb.setUsage(THREE.DynamicDrawUsage);
  geo.setAttribute('aBB', bb);
  const material = new THREE.MeshBasicMaterial({ map, transparent: true, depthWrite: false, blending: additive ? THREE.AdditiveBlending : THREE.NormalBlending, fog: fog && !additive });
  material.onBeforeCompile = sh => {
    sh.vertexShader = sh.vertexShader
      .replace('#include <common>', '#include <common>\nattribute vec2 aBB;\nvarying float vBBa;')
      .replace('#include <project_vertex>', `vBBa = aBB.y;
      vec4 mvPosition = modelViewMatrix * instanceMatrix * vec4(0.0, 0.0, 0.0, 1.0);
      vec2 bsc = vec2(length(instanceMatrix[0].xyz), length(instanceMatrix[1].xyz));
      float bc = cos(aBB.x), bs = sin(aBB.x);
      vec2 bp = position.xy * bsc;
      mvPosition.xy += vec2(bc * bp.x - bs * bp.y, bs * bp.x + bc * bp.y);
      gl_Position = projectionMatrix * mvPosition;`);
    sh.fragmentShader = sh.fragmentShader
      .replace('#include <common>', '#include <common>\nvarying float vBBa;')
      .replace('#include <alphamap_fragment>', '#include <alphamap_fragment>\ndiffuseColor.a *= vBBa;');
  };
  material.customProgramCacheKey = () => 'c3bb' + (additive ? 'a' : 'n') + (fog ? 'f' : '');
  const mesh = new THREE.InstancedMesh(geo, material, n);
  mesh.frustumCulled = false; mesh.count = 0;
  mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
  const white = new THREE.Color(1, 1, 1);
  for (let i = 0; i < n; i++) mesh.setColorAt(i, white);
  const items = [], m4 = new THREE.Matrix4(), q = new THREE.Quaternion(), v = new THREE.Vector3(), s = new THREE.Vector3(), c = new THREE.Color();
  return {
    mesh,
    add(x, y, z, sx, sy, rot, a, color) { if (a > .003 && items.length < n) items.push({ x, y, z, sx, sy, rot, a, color }); },
    flush(cam) {
      if (!additive && cam) { for (const it of items) it.d = (it.x - cam.x) ** 2 + (it.y - cam.y) ** 2 + (it.z - cam.z) ** 2; items.sort((a, b) => b.d - a.d); }
      items.forEach((it, i) => {
        m4.compose(v.set(it.x, it.y, it.z), q, s.set(it.sx, it.sy, 1)); mesh.setMatrixAt(i, m4);
        bb.array[i * 2] = it.rot; bb.array[i * 2 + 1] = it.a;
        mesh.setColorAt(i, c.set(it.color));
      });
      mesh.count = items.length;
      if (items.length) { mesh.instanceMatrix.needsUpdate = true; bb.needsUpdate = true; mesh.instanceColor.needsUpdate = true; }
      items.length = 0;
    },
  };
}

// Plain instanced pieces (tracers, grenades, floor glows): set(i, matrix, colour).
function instanced(THREE, geo, material, n) {
  const mesh = new THREE.InstancedMesh(geo, material, n);
  mesh.frustumCulled = false; mesh.count = 0;
  mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
  const white = new THREE.Color(1, 1, 1);
  for (let i = 0; i < n; i++) mesh.setColorAt(i, white);
  let k = 0;
  const c = new THREE.Color();
  return {
    mesh,
    add(m4, color) { if (k >= n) return; mesh.setMatrixAt(k, m4); mesh.setColorAt(k, c.set(color)); k++; },
    flush() { mesh.count = k; if (k) { mesh.instanceMatrix.needsUpdate = true; mesh.instanceColor.needsUpdate = true; } k = 0; },
  };
}

// ---------------------------------------------------------------------------
// Weapons: toy water blasters (third person and first person share the
// blaster; +X is the nozzle, the origin at the trigger, units of the humanoid:
// ~metres). Rounded bright plastic, a water tank on top, a nozzle at the tip.
// ---------------------------------------------------------------------------
const TIPS = { rifle: .8, m4a1s: .95, awp: 1.02, smg: .5, pistol: .17, usp: .27, deagle: .22 };
export function tipOf(wid) {
  const cls = weaponClass(wid);
  return TIPS[wid] != null ? TIPS[wid] : TIPS[cls] || .8;
}
// [body, accent] per blaster.
const ORANGE = '#ff8a1f', BLUE = '#2f8fe8', LIME = '#8fdc2a', YELLOW = '#ffd23a', SKY = '#29b6f6';
const PLASTIC = {
  glock: [ORANGE, BLUE], usp: [BLUE, YELLOW], p250: [LIME, ORANGE], deagle: [YELLOW, BLUE],
  mac10: [LIME, BLUE], mp9: [BLUE, ORANGE], galil: [LIME, ORANGE], famas: [SKY, YELLOW],
  ak47: [ORANGE, BLUE], m4a1s: [BLUE, LIME], awp: [LIME, ORANGE],
};
const WATER = '#8fe3ff', CAP = '#ffffff';
function gunParts(kit, wid) {
  const { G, M } = kit, cls = weaponClass(wid), P = [];
  const [body, acc] = PLASTIC[wid] || [ORANGE, BLUE];
  const add = (g, c, m) => P.push({ g, c, m });
  const bar = (x0, x1, y, r, c) => add(G.cyl, c, M((x0 + x1) / 2, y, 0, r * 2, x1 - x0, r * 2, 0, 0, Math.PI / 2));
  // A rounded capsule along X (ry tall, rz wide), with ball ends.
  const pill = (x0, x1, y, ry, rz, c) => {
    add(G.cyl, c, M((x0 + x1) / 2, y, 0, ry * 2, x1 - x0, rz * 2, 0, 0, Math.PI / 2));
    add(G.sph, c, M(x0, y, 0, ry * 1.6, ry * 2, rz * 2));
    add(G.sph, c, M(x1, y, 0, ry * 1.6, ry * 2, rz * 2));
  };
  const grip = (x, y, h, rz, c = acc) => add(G.sph, c, M(x, y, 0, .052, h, .04, 0, 0, rz));
  if (cls === 'pistol') {
    const L = wid === 'deagle' ? .22 : .17;
    pill(-.02, L - .04, .035, .03, .022, body);
    grip(-.01, -.04, .11, .28);
    add(G.box, CAP, M(.03, -.005, 0, .05, .02, .012));
    add(G.sph, WATER, M(.04, .078, 0, .075, .05, .036));
    add(G.cyl, CAP, M(.04, .104, 0, .018, .012, .018));
    bar(L - .05, L, .035, .013, acc);
    if (wid === 'usp') bar(L - .03, .27, .035, .016, acc);
    return P;
  }
  if (cls === 'awp') {
    // The Aqua Jet: long body, a big tank where the scope was, a long nozzle.
    pill(-.1, .35, .03, .055, .032, body);
    pill(-.46, -.2, .0, .055, .028, acc);
    add(G.box, body, M(-.15, .015, 0, .12, .06, .04));
    grip(.02, -.07, .11, -.1);
    add(G.box, CAP, M(-.06, -.03, 0, .05, .02, .012));
    pill(-.12, .28, .14, .06, .06, WATER);
    add(G.cyl, CAP, M(-.12, .14, 0, .124, .02, .124, 0, 0, Math.PI / 2));
    add(G.cyl, CAP, M(.28, .14, 0, .124, .02, .124, 0, 0, Math.PI / 2));
    add(G.cyl, CAP, M(.1, .205, 0, .036, .022, .036));
    add(G.box, CAP, M(.3, .085, 0, .02, .07, .02));
    pill(.4, .56, -.005, .03, .03, acc);
    bar(.35, .96, .045, .02, CAP);
    bar(.93, 1.02, .045, .028, acc);
    return P;
  }
  if (cls === 'smg') {
    pill(-.06, .24, .03, .045, .028, body);
    pill(-.25, -.1, .02, .025, .016, acc);
    grip(-.01, -.06, .11, .25);
    add(G.box, CAP, M(.03, -.02, 0, .05, .02, .012));
    add(G.cyl, WATER, M(.08, .105, 0, .08, .09, .08));
    add(G.cyl, CAP, M(.08, .155, 0, .03, .016, .03));
    pill(.16, .27, -.02, .022, .022, acc);
    bar(.24, .48, .04, .013, CAP);
    add(G.sph, acc, M(.49, .04, 0, .036, .036, .036));
    return P;
  }
  // Soakers (rifles): a pump under the nozzle, a long tank on top.
  const L = wid === 'm4a1s' ? .95 : .8;
  pill(-.05, .3, .035, .052, .03, body);
  pill(-.42, -.18, .01, .045, .025, acc);
  add(G.box, body, M(-.12, .02, 0, .1, .05, .04));
  grip(-.01, -.06, .12, .27);
  add(G.box, CAP, M(.03, -.025, 0, .05, .02, .012));
  pill(-.04, .2, .125, .045, .045, WATER);
  add(G.cyl, CAP, M(.08, .175, 0, .03, .02, .03));
  add(G.box, CAP, M(.22, .09, 0, .02, .05, .02));
  pill(.28, .44, -.005, .03, .03, acc);
  bar(.3, L - .05, .045, wid === 'm4a1s' ? .02 : .017, CAP);
  bar(L - .07, L, .045, .026, acc);
  return P;
}

// The Mega Balloon: a big round water balloon with its knot (the light sits on the knot).
function balloonParts(kit) {
  const { G, M } = kit;
  return [
    { g: G.sph, c: '#2f8fe8', m: M(0, .13, 0, .3, .27, .3) },
    { g: G.sph, c: '#9fe0ff', m: M(-.07, .19, .07, .08, .06, .06) },
    { g: G.sph, c: '#1f6fc0', m: M(0, .27, 0, .06, .045, .06) },
    { g: G.cyl, c: '#1f6fc0', m: M(0, .3, 0, .03, .035, .03) },
  ];
}

// From a to b (3-vectors), a box w × h thick.
function segM(THREE, a, b, w, h) {
  const A = new THREE.Vector3(...a), B = new THREE.Vector3(...b), d = B.clone().sub(A), L = d.length();
  const m = new THREE.Matrix4();
  m.compose(A.add(B).multiplyScalar(.5), new THREE.Quaternion().setFromUnitVectors(new THREE.Vector3(1, 0, 0), d.normalize()), new THREE.Vector3(L, w, h));
  return m;
}

// First-person models: the blaster with gloved hands and sleeves running out
// of the frame (or the Mega Balloon held low). In gun space (+X barrel, +Z right, +Y up).
function viewParts(kit, THREE, wid, side, c4 = false) {
  const { G, M } = kit, sl = SLEEVE[side] || SLEEVE.CT, gl = GLOVE[side] || GLOVE.CT;
  const P = c4 ? balloonParts(kit).map(p => ({ ...p, m: new THREE.Matrix4().makeTranslation(.12, -.15, 0).multiply(p.m) })) : gunParts(kit, wid);
  const seg = (a, b, w, c) => P.push({ g: G.box, c, m: segM(THREE, a, b, w, w) });
  const hand = (x, y, z, c = gl) => P.push({ g: G.box, c, m: M(x, y, z, .1, .085, .085) });
  const pistol = !c4 && weaponClass(wid) === 'pistol';
  if (c4) {
    hand(.12, -.02, .14); hand(.12, -.02, -.14);
    seg([.1, -.04, .16], [-.35, -.3, .32], .085, sl);
    seg([.1, -.04, -.16], [-.35, -.3, -.32], .085, sl);
  } else if (pistol) {
    hand(-.005, -.045, .015); hand(.0, -.06, -.04);
    seg([-.04, -.07, .03], [-.42, -.24, .16], .08, sl);
    seg([-.03, -.08, -.06], [-.4, -.28, -.26], .08, sl);
  } else {
    const fx = weaponClass(wid) === 'smg' ? .2 : .33;
    hand(-.01, -.07, .01);
    hand(fx, -.035, -.005);
    seg([-.05, -.09, .03], [-.45, -.25, .17], .085, sl);
    seg([fx - .03, -.06, -.03], [.02, -.32, -.3], .08, sl);
  }
  return P;
}

// ---------------------------------------------------------------------------
// HUD overlay (DOM, over the 3D canvas and under the shared head-up display)
// ---------------------------------------------------------------------------
let cssDone = false;
function css() {
  if (cssDone || typeof document === 'undefined' || document.getElementById('c3CSS')) return;
  cssDone = true;
  const st = document.createElement('style'); st.id = 'c3CSS';
  st.textContent = `
.c3 { position:absolute; inset:0; pointer-events:none; overflow:hidden; font:600 12px/1.3 "Segoe UI", system-ui, sans-serif; color:#fff; }
.c3 > div { position:absolute; }
.c3 .fx { inset:0; opacity:0; }
.c3 .smk { background:rgb(196,200,208); }
.c3 .boom { background:rgb(150,215,255); }
.c3 .dead { background:radial-gradient(ellipse at center, rgba(40,120,200,.18), rgba(20,80,160,.55)); }
.c3 .wht { background:#fff; }
.c3 .scope { inset:0; display:none; background:radial-gradient(circle at center, rgba(0,0,0,0) 0, rgba(0,0,0,0) calc(min(46vw, 46vh, 47%) - 2px), rgba(0,0,0,.65) calc(min(46vw, 46vh, 47%)), #000 calc(min(46vw, 46vh, 47%) + 2px)); }
.c3 .scope::before, .c3 .scope::after { content:''; position:absolute; background:#000; }
.c3 .scope::before { left:0; right:0; top:50%; height:1.5px; }
.c3 .scope::after { top:0; bottom:0; left:50%; width:1.5px; }
.c3 .x { left:50%; top:50%; width:0; height:0; }
.c3 .x i { position:absolute; background:#4dff6a; box-shadow:0 0 0 1px rgba(0,0,0,.7); }
.c3 .x i:nth-child(1) { left:4px; top:-1px; width:7px; height:2px; } .c3 .x i:nth-child(2) { left:-11px; top:-1px; width:7px; height:2px; }
.c3 .x i:nth-child(3) { left:-1px; top:4px; width:2px; height:7px; } .c3 .x i:nth-child(4) { left:-1px; top:-11px; width:2px; height:7px; }
.c3 .hit { left:50%; top:50%; width:30px; height:30px; margin:-15px 0 0 -15px; opacity:0; background:
 linear-gradient(45deg, transparent 46%, currentColor 46%, currentColor 54%, transparent 54%) , linear-gradient(-45deg, transparent 46%, currentColor 46%, currentColor 54%, transparent 54%);
 -webkit-mask:radial-gradient(circle, transparent 7px, #000 7.5px); mask:radial-gradient(circle, transparent 7px, #000 7.5px); }
.c3 .feed { right:8px; top:46px; display:flex; flex-direction:column; align-items:flex-end; gap:3px; }
.c3 .feed div { background:rgba(6,9,14,.74); border-radius:4px; padding:3px 9px; font-weight:700; white-space:nowrap; display:flex; gap:7px; align-items:center; }
.c3 .feed div.hl { background:rgba(20,80,140,.78); }
.c3 .feed div.mine { box-shadow:inset 0 0 0 1.5px #4fc3ff; }
.c3 .feed small { font-weight:600; opacity:.85; font-size:11px; }
.c3 .feed .hs { color:#7fe0ff; font-size:11px; }
.c3 .card { right:8px; bottom:8px; display:flex; gap:6px; align-items:stretch; }
.c3 .card > span { background:rgba(6,9,14,.72); border-radius:8px; padding:4px 11px; display:flex; flex-direction:column; justify-content:center; }
.c3 .card b { font-size:20px; font-weight:900; line-height:1.05; }
.c3 .card small { font-size:10px; opacity:.75; letter-spacing:.04em; }
.c3 .card .who { border-left:4px solid var(--sc, #fff); min-width:96px; }
.c3 .card .who b { font-size:14px; }
.c3 .card .cash { color:#a8ffb0; font-weight:800; font-size:12px; }
.c3 .card .low { color:#ffb14e; }
.c3 .act { left:50%; top:30%; transform:translateX(-50%); width:min(320px, 70%); background:rgba(6,9,14,.75); border-radius:8px; padding:6px 10px 8px; display:none; }
.c3 .act .t { display:flex; justify-content:space-between; font-weight:900; font-size:12px; margin-bottom:4px; }
.c3 .act .bar { height:8px; background:rgba(255,255,255,.14); border-radius:4px; overflow:hidden; }
.c3 .act .bar i { display:block; height:100%; width:0; }
.c3 .kb { left:0; right:0; top:44%; text-align:center; background:rgba(0,0,0,.6); padding:8px; font-weight:800; font-size:14px; display:none; }
@media (max-width:640px) { .c3 .feed { top:40px; font-size:11px; } .c3 .card b { font-size:16px; } .c3 .card .who { display:none; } }`;
  document.head.append(st);
}

function makeHud(root) {
  css();
  const el = document.createElement('div'); el.className = 'c3';
  el.innerHTML = `<div class="fx smk"></div><div class="fx boom"></div><div class="fx dead"></div><div class="scope"></div>
<div class="x"><i></i><i></i><i></i><i></i></div><div class="hit"></div><div class="feed"></div><div class="act"><div class="t"><span></span><span></span></div><div class="bar"><i></i></div></div>
<div class="kb"></div><div class="card"><span class="who"><small></small><b></b><span class="cash"></span></span><span class="hp"><small>HP</small><b></b></span><span class="ar"><small>RAINCOAT</small><b></b></span><span class="am"><small></small><b></b></span></div><div class="fx wht"></div>`;
  const before = root && root.querySelector('.a3-hud');
  if (root) root.insertBefore(el, before || null);
  const q = s => el.querySelector(s);
  const cache = new Map();
  const set = (k, fn, v) => { if (cache.get(k) !== v) { cache.set(k, v); fn(v); } };
  const H = {
    el, smk: q('.smk'), boom: q('.boom'), dead: q('.dead'), wht: q('.wht'), scope: q('.scope'), x: q('.x'), hit: q('.hit'), feed: q('.feed'),
    act: q('.act'), actL: q('.act .t span:first-child'), actR: q('.act .t span:last-child'), actBar: q('.act .bar i'), kb: q('.kb'),
    card: q('.card'), who: q('.card .who'), whoS: q('.who small'), whoB: q('.who b'), cash: q('.cash'), hp: q('.hp b'), ar: q('.ar b'), amS: q('.am small'), am: q('.am b'),
    op(k, e, v) { set(k, x => { e.style.opacity = x; }, v.toFixed(2)); },
    show(k, e, on, d = 'block') { set(k, x => { e.style.display = x; }, on ? d : 'none'); },
    text(k, e, v) { set(k, x => { e.textContent = x; }, v); },
    html(k, e, v) { set(k, x => { e.innerHTML = x; }, v); },
    style(k, e, p, v) { set(k, x => { e.style.setProperty(p, x); }, v); },
    cls(k, e, c, on) { set(k, x => { e.classList.toggle(c, x === '1'); }, on ? '1' : '0'); },
  };
  return H;
}

// ---------------------------------------------------------------------------
// The adapter
// ---------------------------------------------------------------------------
export const adapter = {
  label: 'Splash Strike',

  build(ctx) {
    const { THREE, scene, kit, camera } = ctx, s = ctx.state, low = !!(ctx.quality && ctx.quality.low);
    s.low = low;
    s.models = new Map(); s.yaw = new Map(); s.cut = true; s.R = null; s.gt = 0; s.rp = false; s.boom = null; s.eye = null;
    scene.background = new THREE.Color(HAZE);
    scene.fog = new THREE.Fog(new THREE.Color(HAZE), 45, 175);
    const sun = [Math.cos(SUN_ANG) * 60, 75, Math.sin(SUN_ANG) * 60];
    s.lights = kit.lights(scene, { sky: '#d6e4f4', ground: '#b48d5c', hemi: 1.05, sun: '#fff0d4', sunI: 2.1, dir: sun });
    // Sky dome and the sun.
    const skyTex = kit.canvasTexture(64, 512, paintSky, { aniso: 1 });
    const sky = new THREE.Mesh(new THREE.SphereGeometry(420, 24, 16), new THREE.MeshBasicMaterial({ map: skyTex, side: THREE.BackSide, fog: false, depthWrite: false }));
    sky.renderOrder = -1; scene.add(sky);
    s.glowTex = kit.canvasTexture(64, 64, paintGlow, { aniso: 1 });
    const sunS = new THREE.Sprite(new THREE.SpriteMaterial({ map: s.glowTex, color: 0xfff3d8, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending, fog: false }));
    const sd = new THREE.Vector3(...sun).normalize().multiplyScalar(380);
    sunS.position.copy(sd); sunS.scale.set(90, 90, 1); scene.add(sunS);
    // Ground round the map.
    const ground = new THREE.Mesh(new THREE.PlaneGeometry(900, 900), kit.mat('#b39a74'));
    ground.rotation.x = -Math.PI / 2; ground.position.y = -.03; scene.add(ground);
    // Effects (built now so their shaders compile with the scene).
    s.puffTex = kit.canvasTexture(128, 128, paintPuff, { aniso: 1 });
    s.soft = billboards(THREE, 300, s.puffTex); s.soft.mesh.renderOrder = 2; scene.add(s.soft.mesh);
    s.glow = billboards(THREE, 160, s.glowTex, { additive: true }); s.glow.mesh.renderOrder = 3; scene.add(s.glow.mesh);
    const floorGlowGeo = new THREE.PlaneGeometry(1, 1); floorGlowGeo.rotateX(-Math.PI / 2);
    s.decal = instanced(THREE, floorGlowGeo, new THREE.MeshBasicMaterial({ map: s.glowTex, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending, fog: false, polygonOffset: true, polygonOffsetFactor: -4, polygonOffsetUnits: -4 }), 12);
    scene.add(s.decal.mesh);
    // Puddles (soaked players, soap slicks, the burst): soft blue discs, normally blended.
    s.puddle = instanced(THREE, floorGlowGeo, new THREE.MeshBasicMaterial({ map: s.glowTex, transparent: true, opacity: .55, depthWrite: false, fog: true, polygonOffset: true, polygonOffsetFactor: -3, polygonOffsetUnits: -3 }), 20);
    s.puddle.mesh.renderOrder = 1; scene.add(s.puddle.mesh);
    s.tracer = instanced(THREE, new THREE.BoxGeometry(1, 1, 1), new THREE.MeshBasicMaterial({ transparent: true, depthWrite: false, blending: THREE.AdditiveBlending, fog: false }), 24);
    scene.add(s.tracer.mesh);
    s.nade = instanced(THREE, new THREE.SphereGeometry(.07, 8, 6), new THREE.MeshLambertMaterial(), 10);
    scene.add(s.nade.mesh);
    s.shadows = kit.blobShadows(16); scene.add(s.shadows.mesh);
    // The Mega Balloon: on the ground (dropped, planted) and on the carrier's back.
    s.bomb = new THREE.Group();
    s.bomb.add(new THREE.Mesh(kit.merge(balloonParts(kit)), kit.mats.vc));
    s.led = new THREE.Mesh(new THREE.SphereGeometry(.022, 8, 6), new THREE.MeshBasicMaterial({ color: 0x7fe8ff }));
    s.led.position.set(0, .325, 0); s.bomb.add(s.led);
    s.bomb.visible = false; scene.add(s.bomb);
    s.bombBack = new THREE.Mesh(kit.merge(balloonParts(kit)), kit.mats.vc);
    s.bombBack.rotation.set(0, Math.PI / 2, Math.PI / 2); s.bombBack.position.set(-.2, .32, 0); s.bombBack.scale.setScalar(.8); s.bombBack.visible = false;
    // The first-person view model: in front of the camera, drawn last over everything.
    scene.add(camera);
    s.vm = new THREE.Group(); camera.add(s.vm);
    s.vmModels = new Map();
    s.vmFlash = new THREE.Sprite(new THREE.SpriteMaterial({ map: s.glowTex, color: 0xd8f2ff, transparent: true, depthTest: false, depthWrite: false, blending: THREE.AdditiveBlending, fog: false }));
    s.vmFlash.renderOrder = 1001; s.vmFlash.visible = false; s.vmFlash.scale.set(.2, .2, 1); camera.add(s.vmFlash);
    s.vmMat = new THREE.MeshLambertMaterial({ vertexColors: true, transparent: true, opacity: 1, depthWrite: true, fog: false });
    // The HUD.
    s.hud = typeof document !== 'undefined' ? makeHud(ctx.root) : null;
    s.tmp = { m4: new THREE.Matrix4(), q: new THREE.Quaternion(), v: new THREE.Vector3(), v2: new THREE.Vector3(), x: new THREE.Vector3(1, 0, 0), s: new THREE.Vector3(), e: new THREE.Euler() };
    s.guns = new Map(); s.tags = new Map();
  },

  // The map: built once the match tells which one (meta).
  map(ctx, meta) {
    const { THREE, scene, kit } = ctx, s = ctx.state, low = s.low;
    if (s.mapG) { scene.remove(s.mapG); s.mapG.traverse(o => { if (o.geometry) o.geometry.dispose(); }); }
    const grp = new THREE.Group(); s.mapG = grp;
    const G = s.G = buildGrid(meta, .5);
    const size = s.size = meta.size || MAP_SIZE;
    const aniso = Math.min(8, ctx.renderer.capabilities.getMaxAnisotropy());
    // Floor.
    const fw = low ? 1000 : 1600;
    s.floorTex = s.floorTex || kit.canvasTexture(fw, Math.round(fw * size[1] / size[0]), () => {}, { aniso });
    s.floorTex.redraw((g, w, h) => paintFloor(g, w, h, meta));
    const floor = new THREE.Mesh(new THREE.PlaneGeometry(size[0], size[1]), new THREE.MeshLambertMaterial({ map: s.floorTex }));
    floor.rotation.x = -Math.PI / 2; grp.add(floor);
    // Walls.
    s.wallTex = s.wallTex || kit.canvasTexture(low ? 256 : 512, low ? 256 : 512, paintWall, { aniso, repeat: true });
    const mm = buildMapMesh(G);
    const wg = new THREE.BufferGeometry();
    wg.setAttribute('position', new THREE.Float32BufferAttribute(mm.pos, 3));
    wg.setAttribute('normal', new THREE.Float32BufferAttribute(mm.nor, 3));
    wg.setAttribute('uv', new THREE.Float32BufferAttribute(mm.uv, 2));
    wg.setAttribute('color', new THREE.Float32BufferAttribute(mm.col, 3));
    wg.computeBoundingSphere();
    grp.add(new THREE.Mesh(wg, new THREE.MeshLambertMaterial({ map: s.wallTex, vertexColors: true })));
    s.wallTris = mm.pos.length / 9;
    // Crates (one mesh, with uvs).
    s.crateTex = s.crateTex || kit.canvasTexture(128, 128, paintCrate, { aniso });
    const crates = cratesOf(meta), cp = [], cn = [], cu = [], cc = [];
    const box = new THREE.BoxGeometry(1, 1, 1), tc = new THREE.Color();
    for (const c of crates) {
      const g = box.clone(), w = mapToWorld(c.x, c.y, c.h / 2, size);
      g.scale(c.w, c.h, c.d); g.translate(w[0], w[1], w[2]);
      const k = .82 + hashN(c.seed, 3) * .3;
      tc.setRGB(k, k * .97, k * .92);
      const P = g.attributes.position, N = g.attributes.normal, U = g.attributes.uv, I = g.index;
      for (let i = 0; i < I.count; i++) { const j = I.getX(i); cp.push(P.getX(j), P.getY(j), P.getZ(j)); cn.push(N.getX(j), N.getY(j), N.getZ(j)); cu.push(U.getX(j), U.getY(j)); cc.push(tc.r, tc.g, tc.b); }
      g.dispose();
    }
    box.dispose();
    if (cp.length) {
      const cg = new THREE.BufferGeometry();
      cg.setAttribute('position', new THREE.Float32BufferAttribute(cp, 3)); cg.setAttribute('normal', new THREE.Float32BufferAttribute(cn, 3));
      cg.setAttribute('uv', new THREE.Float32BufferAttribute(cu, 2)); cg.setAttribute('color', new THREE.Float32BufferAttribute(cc, 3));
      grp.add(new THREE.Mesh(cg, new THREE.MeshLambertMaterial({ map: s.crateTex, vertexColors: true })));
    }
    // Arches, doors, awnings: plain vertex-coloured boxes, merged.
    const parts = [], { G: KG, M } = kit;
    for (const a of archesOf(meta, G)) {
      const yaw = mapYaw(a.ang), c = Math.cos(a.ang), sn = Math.sin(a.ang);
      const at = (dd, l, y) => mapToWorld(a.x + c * dd - sn * l, a.y + sn * dd + c * l, y, size);
      const bx = (dd, l, y, sx, sy, sz, col) => { const p = at(dd, l, y); parts.push({ g: KG.box, c: col, m: M(p[0], p[1], p[2], sx, sy, sz, 0, yaw, 0) }); };
      const stone = '#d8c39b', stone2 = '#c9b189';
      for (const l of [-2.85, 2.85]) { bx(0, l, 1.55, .9, 3.1, .75, stone2); bx(0, l, .15, 1.0, .3, .85, '#b49b74'); bx(0, l, 3.05, 1.0, .12, .85, '#e6d3ae'); }
      bx(0, 0, 3.55, .9, .9, 6.5, stone);
      bx(0, 0, 4.06, 1.25, .14, 6.9, '#9c5a3c');
      // The arch under the lintel: slices following a flat curve.
      const N = 12;
      for (let k = 0; k < N; k++) {
        const l = -2.5 + (k + .5) * 5 / N, yb = 2.55 + .55 * Math.sqrt(Math.max(0, 1 - (l / 2.5) ** 2));
        bx(0, l, (yb + 3.12) / 2, .8, 3.12 - yb, 5 / N + .01, stone);
      }
      if (a.door) for (const side of [-1, 1]) {
        // An open wooden door leaf against each pillar.
        const p = at(.55, side * 2.4, 1.2);
        const leaf = M(p[0], p[1], p[2], 1.15, 2.35, .07, 0, yaw + side * 1.25, 0);
        parts.push({ g: KG.box, c: '#3f6a62', m: leaf });
        const p2 = at(.55, side * 2.4, 2.1);
        parts.push({ g: KG.box, c: '#2c4a44', m: M(p2[0], p2[1], p2[2], 1.17, .08, .09, 0, yaw + side * 1.25, 0) });
      }
    }
    // Awnings and planks over a few zones (shade cloth, Mirage style).
    const R = rng(29);
    for (const z of meta.zones || []) {
      if (z.kind === 'site' || R() < .45) continue;
      const side = R() < .5 ? -1 : 1, along = z.w > z.h;
      const ax = along ? z.x + (R() - .5) * z.w * .4 : z.x + side * (z.w / 2 - .6), ay = along ? z.y + side * (z.h / 2 - .6) : z.y + (R() - .5) * z.h * .4;
      const p = mapToWorld(ax, ay, 3.1, size), col = ['#b8453a', '#3f6fa0', '#d7b25a', '#5f8a55'][Math.floor(R() * 4)];
      parts.push({ g: KG.box, c: col, m: M(p[0], p[1], p[2], along ? 4.2 : 1.4, .06, along ? 1.4 : 4.2, along ? side * .25 : 0, 0, along ? 0 : -side * .25) });
    }
    // Barrels and planters along some walls.
    for (const f of mmFaces(mm, 70, R)) {
      const p = mapToWorld(f.x + f.nx * .55, f.y + f.ny * .55, 0, size);
      if (cellAt(G, f.x + f.nx * .55, f.y + f.ny * .55) === 0) continue;
      if (R() < .5) parts.push({ g: KG.cyl, c: R() < .5 ? '#5f6b4a' : '#6c4b33', m: M(p[0], .5, p[2], .62, 1.0, .62) });
      else { parts.push({ g: KG.box, c: '#a0663f', m: M(p[0], .35, p[2], .8, .7, .8) }); parts.push({ g: KG.sph, c: '#4f7d3a', m: M(p[0], .9, p[2], .9, .7, .9) }); }
    }
    if (parts.length) grp.add(new THREE.Mesh(kit.merge(parts), kit.mats.vc));
    // Site letters on the walls.
    for (const sg of siteSignsOf(meta, G)) {
      const tex = kit.canvasTexture(256, 256, (g, w, h) => paintSign(g, w, h, sg.site), { aniso });
      const pl = new THREE.Mesh(new THREE.PlaneGeometry(2.4, 2.4), new THREE.MeshLambertMaterial({ map: tex, transparent: true, depthWrite: false, polygonOffset: true, polygonOffsetFactor: -2, polygonOffsetUnits: -2 }));
      const p = mapToWorld(sg.x, sg.y, Math.min(2.5, heightAt(G, sg.x - sg.nx * .3, sg.y - sg.ny * .3) - 1.3), size);
      pl.position.set(p[0], p[1], p[2]);
      // The plane faces +Z: turn it to the wall's normal (map (nx, ny) = world (nx, 0, ny)).
      pl.rotation.y = Math.atan2(sg.nx, sg.ny);
      grp.add(pl);
    }
    scene.add(grp);
    s.meta = meta;
  },

  // A player for q (rebuilt when they change side).
  model(ctx, q) {
    const s = ctx.state, { kit, THREE } = ctx;
    let m = s.models.get(q.id);
    if (m && m.side === q.side) return m;
    if (m) { ctx.scene.remove(m.h.root); m.h.root.traverse(o => { if (o.isSprite) o.material.dispose(); }); }
    const skin = ['#f0c8a4', '#d9a47e', '#b77b52', '#8d5a3b'][strHash(q.id) % 4];
    const h = kit.soldier({ side: q.side === 'T' ? 't' : 'ct', skin, height: 1.8, uniform: UNIFORM[q.side], vest: VEST[q.side] });
    const gun = new THREE.Mesh(this.gunGeo(ctx, q.weapon), kit.mats.vc);
    h.spine.add(gun);
    // The name over the head.
    let tag = s.tags.get(q.id);
    if (!tag) { tag = kit.canvasTexture(256, 48, () => {}, { aniso: 1 }); s.tags.set(q.id, tag); }
    tag.redraw((g, w, hh) => paintTag(g, w, hh, q.handle, SIDE_COLOR[q.side] || '#fff'));
    const name = new THREE.Sprite(new THREE.SpriteMaterial({ map: tag, transparent: true, depthWrite: false, fog: false }));
    name.scale.set(1.5, .28, 1); name.position.y = 2.2; h.root.add(name);
    m = { h, gun, wid: q.weapon, side: q.side, name };
    ctx.scene.add(h.root);
    s.models.set(q.id, m);
    return m;
  },
  gunGeo(ctx, wid) {
    const s = ctx.state, k = wid || 'glock';
    let g = s.guns.get(k);
    if (!g) { g = ctx.kit.merge(gunParts(ctx.kit, k)); s.guns.set(k, g); }
    return g;
  },
  viewModel(ctx, key, build) {
    const s = ctx.state;
    let m = s.vmModels.get(key);
    if (!m) {
      const { THREE, kit } = ctx;
      m = new THREE.Mesh(kit.merge(build()), s.vmMat);
      m.renderOrder = 1000;
      // Drawn last, over a cleared depth buffer: never inside a wall.
      m.onBeforeRender = r => { r.state.buffers.depth.setMask(true); r.clearDepth(); };
      m.frustumCulled = false; m.visible = false;
      s.vm.add(m); s.vmModels.set(key, m);
    }
    return m;
  },

  update(ctx, frame) {
    const s = ctx.state, v = ctx.view, { THREE } = ctx;
    if (!v.meta) { this.hideAll(ctx); return; }
    if (s.meta !== v.meta) this.map(ctx, v.meta);
    const T = timesOf(v);
    if (!T) { this.hideAll(ctx); return; }
    const { R, gt, p, rp } = T, size = s.size, G = s.G, tmp = s.tmp, speed = Math.max(1, v.speed || 1);
    // A new round, a jump back or the splash cam: no smoothing.
    if (R !== s.R || gt < s.gt - .05 || !!rp !== s.rp) { s.cut = true; s.yaw.clear(); s.boom = null; }
    s.R = R; s.gt = gt; s.p = p; s.rp = !!rp;
    const bomb = bombState(R, gt), shots = shotsAt(R, p);
    const watch = ctx.watch || {}, first = watch.mode === 'first', camPos = ctx.camera.position;
    const cam = { x: camPos.x, y: camPos.y, z: camPos.z };
    s.bombState = bomb;
    // ---- players ----
    const seen = new Set();
    let sh = 0;
    const tips = new Map();
    Object.values(R.players).forEach(q => {
      const m = this.model(ctx, q), h = m.h;
      seen.add(q.id);
      if (m.wid !== q.weapon) { m.gun.geometry = this.gunGeo(ctx, q.weapon); m.wid = q.weapon; }
      const dead = gt >= q.deathT;
      let at = dead ? (q.death || posAt(q.track, q.deathT)) : posAt(q.track, gt);
      at = unstick(G, at.x, at.y);
      const sp = dead ? 0 : speedAt(q, gt), run = clamp(sp / 4.5, 0, 1);
      const cr = dead ? 0 : crouchOf(R, q.id, gt, bomb, sp);
      const yawT = mapYaw(povAim(R, q.id, Math.min(gt, q.deathT)));
      const prev = s.yaw.get(q.id);
      const yaw = prev == null || s.cut ? yawT : dampAngle(prev, yawT, 14, frame.dt * speed);
      s.yaw.set(q.id, yaw);
      const w = mapToWorld(at.x, at.y, 0, size);
      h.root.visible = true; h.root.position.set(w[0], 0, w[2]);
      let fall = null;
      if (dead) {
        const k = clamp((gt - q.deathT) / .5, 0, 1), kill = R.kills.find(z => z.victim === q.id);
        const dx = kill ? kill.vx - kill.kx : Math.cos(yaw), dy = kill ? kill.vy - kill.ky : 1;
        fall = { k: easeOut(k) * 1.06, dir: [dx, dy] };
        // Soaked: a puddle spreading under them (a little towards where they sit down).
        const dl = Math.hypot(dx, dy) || 1, pk = clamp((gt - q.deathT) / 1.2, 0, 1), pr = .9 + 1.1 * easeOut(pk);
        tmp.m4.compose(tmp.v.set(w[0] + dx / dl * .45, .03, w[2] + dy / dl * .45), tmp.q.identity(), tmp.s.set(pr, 1, pr));
        s.puddle.add(tmp.m4, '#3a8fd6');
      }
      const mine = first && watch.id === q.id;
      const pitch = mine && s.eye ? s.eye.pitch : 0;
      const pose = { yaw, phase: trackDist(q.track, Math.min(gt, q.deathT)) * 2.6, run, lean: dead ? 0 : .12, crouch: cr, fall, firstPerson: mine };
      h.pose(pose);
      // The gun level with the aim, at the right shoulder; hands on it.
      const pistol = weaponClass(q.weapon) === 'pistol';
      m.gun.position.set(pistol ? .42 : .2, pistol ? .44 : .5, pistol ? .04 : .14);
      m.gun.rotation.set(0, 0, -h.spine.rotation.z + pitch + (dead ? -.4 : 0));
      if (!dead && !mine) {
        h.root.updateMatrixWorld(true);
        const gr = m.gun.localToWorld(tmp.v.set(-.01, -.07, .01)).toArray();
        const fx = pistol ? .0 : weaponClass(q.weapon) === 'smg' ? .2 : .33;
        const gl = m.gun.localToWorld(tmp.v.set(fx, pistol ? -.06 : -.035, pistol ? -.03 : 0)).toArray();
        h.pose({ ...pose, arms: [gl, gr] });
      }
      // Third-person muzzle.
      h.root.updateMatrixWorld(true);
      tips.set(q.id, m.gun.localToWorld(tmp.v.set(tipOf(q.weapon), .045, 0)).toArray());
      m.name.visible = !dead && !mine;
      m.gun.visible = !dead || gt - q.deathT < .25; // dropped as they sit down
      if (!mine && sh < 15) s.shadows.set(sh++, w[0], w[2], dead ? .7 : .42, 1);
    });
    for (const [id, m] of s.models) if (!seen.has(id)) m.h.root.visible = false;
    for (let i = sh; i < 16; i++) s.shadows.hide(i);
    s.shadows.commit();
    // ---- the watched eye (cameraFor reads it) ----
    s.eye = watch.id && R.players[watch.id] ? eyeOf(R, watch.id, gt, G) : null;
    if (s.eye) {
      const q = R.players[watch.id];
      const bob = s.eye.dead ? 0 : Math.sin(trackDist(q.track, gt) * 2.6 * 2) * .03 * clamp(speedAt(q, gt) / 4.5, 0, 1);
      s.eye.h += bob;
    }
    // ---- the Mega Balloon ----
    const carrierM = bomb.carrier && s.models.get(bomb.carrier);
    if (carrierM) { if (s.bombBack.parent !== carrierM.h.spine) carrierM.h.spine.add(s.bombBack); s.bombBack.visible = true; } else s.bombBack.visible = false;
    let bp = null;
    if (bomb.dropped) bp = bomb.dropped;
    else if (bomb.planted) bp = { x: bomb.x, y: bomb.y };
    else if (bomb.planting && bomb.planting.prog > .15 && R.plant) bp = { x: R.plant.x, y: R.plant.y };
    s.bomb.visible = !!bp && !bomb.exploded;
    if (bp) { const w = mapToWorld(bp.x, bp.y, .01, size); s.bomb.position.set(w[0], w[1], w[2]); s.bomb.rotation.y = .6; }
    s.bomb.scale.set(1, bomb.defused ? .55 : 1, 1); // drained: it sags
    if (bomb.planted && !bomb.exploded) {
      const rate = bomb.left < 10 ? 4 : bomb.left < 20 ? 2 : 1, on = bomb.defused || Math.sin(gt * Math.PI * 2 * rate) > 0;
      s.led.material.color.set(bomb.defused ? '#3ddc84' : on ? '#7fe8ff' : '#1a4a66');
      if (on) { const w = s.bomb.localToWorld(tmp.v.copy(s.led.position)); s.glow.add(w.x, w.y, w.z, .35, .35, 0, bomb.defused ? .5 : .9, bomb.defused ? '#3ddc84' : '#6fe0ff'); }
    } else s.led.material.color.set('#1a4a66');
    // The burst (real time from the moment it is seen: the round's clock stops at its end):
    // a flash of water, a wave rolling out over the floor, spray and falling drops, a big puddle.
    s.boomFx = 0; s.boomShake = 0;
    if (bomb.exploded) {
      if (!s.boom) s.boom = { t0: frame.clock, x: bomb.x, y: bomb.y };
      const a = (frame.clock - s.boom.t0) * Math.min(2, speed), w = mapToWorld(s.boom.x, s.boom.y, 0, size);
      if (s.eye) {
        const near = clamp(1 - Math.hypot(s.boom.x - s.eye.x, s.boom.y - s.eye.y) / 45, 0, 1);
        s.boomFx = a < 1.6 ? .8 * near * (1 - a / 1.6) : 0;
        s.boomShake = a < 1.2 ? (1 - a / 1.2) * near * .22 : 0;
      }
      if (a < 1.6) {
        const k = clamp(a / 1.4, 0, 1), r = 2 + 15 * easeOut(k);
        for (let i = 0; i < 7; i++) {
          const an = i / 7 * Math.PI * 2 + 1, d = r * .35 * (i ? 1 : 0);
          s.glow.add(w[0] + Math.cos(an) * d, 1 + r * .3 + (i % 3) * .5, w[2] + Math.sin(an) * d, r * 1.1, r * 1.1, an, (1 - k) * .7, i ? '#5cc8ff' : '#e8fbff');
        }
      }
      if (a < 2.2) for (let i = 0; i < 14; i++) {
        // The wave: a ring of water rolling out.
        const an = i / 14 * Math.PI * 2 + hashN(i, 4) * .3, k = clamp(a / 2.2, 0, 1), d = 2 + 16 * easeOut(k);
        s.soft.add(w[0] + Math.cos(an) * d, .5 + Math.sin(k * Math.PI) * .8, w[2] + Math.sin(an) * d, 3 + k * 2, 2 + k * 1.5, an, .8 * (1 - k), i & 1 ? '#7fcfff' : '#d6f2ff');
      }
      if (a < 2.6) for (let i = 0; i < 12; i++) {
        // Drops thrown up and falling back.
        const an = hashN(i, 9) * Math.PI * 2, sp = 4 + hashN(i, 3) * 6, up = 7 + hashN(i, 6) * 5, y = Math.max(0, up * a - 6 * a * a);
        s.soft.add(w[0] + Math.cos(an) * sp * a, .3 + y, w[2] + Math.sin(an) * sp * a, .7, .7, an, .9 * (1 - a / 2.6), '#a8e0ff');
      }
      if (a < 5) for (let i = 0; i < 9; i++) {
        const an = i / 9 * Math.PI * 2, k = clamp(a / 5, 0, 1), d = 3 + 9 * easeOut(k) * (.5 + hashN(i, 5) * .5);
        s.soft.add(w[0] + Math.cos(an) * d, 1 + k * 4 + hashN(i, 7) * 2, w[2] + Math.sin(an) * d, 6 + k * 8, 6 + k * 8, an, .5 * (1 - k), '#e2f4ff');
      }
      const pr = 3 + 15 * easeOut(clamp(a / 1.5, 0, 1));
      tmp.m4.compose(tmp.v.set(w[0], .03, w[2]), tmp.q.identity(), tmp.s.set(pr, 1, pr));
      s.puddle.add(tmp.m4, '#3a8fd6');
    }
    // ---- utility: fog, soap slicks, foam flashes, water balloons ----
    const puffs = s.low ? 9 : 15;
    let nd = 0;
    for (const u of R.utils) {
      const st = utilState(u, gt, p);
      if (!st) continue;
      if (st.kind === 'flight') {
        const w = mapToWorld(st.x, st.y, 1.4 + st.h * 2.5, size);
        tmp.m4.makeTranslation(w[0], w[1], w[2]);
        s.nade.add(tmp.m4, u.type === 'molotov' ? '#bfeaff' : u.type === 'flash' ? '#f4f4f4' : u.type === 'he' ? '#2f8fe8' : '#9aa3ad');
        nd++;
        continue;
      }
      const w = mapToWorld(st.x, st.y, 0, size);
      if (st.kind === 'smoke') {
        const R2 = rng(u.n * 7 + 3);
        for (let i = 0; i < puffs; i++) {
          const an = R2() * Math.PI * 2, dd = i === 0 ? 0 : st.r * (.15 + .55 * R2()), hz = .5 + R2() * 2.6, rr = st.r * (.62 + .35 * R2());
          const wob = Math.sin(frame.clock * .5 + i) * .25;
          const g = Math.round(178 + hz * 12 + R2() * 10);
          s.soft.add(w[0] + Math.cos(an) * dd + wob, hz * (.55 + .45 * st.r / 4.8), w[2] + Math.sin(an) * dd, rr * 1.5, rr * 1.5, an + frame.clock * .06 * (i & 1 ? 1 : -1), st.a * .78, `rgb(${g},${g + 4},${g + 10})`);
        }
      } else if (st.kind === 'molotov') {
        // The soap slick: a pale blue sheen with suds and bubbles drifting up.
        const R2 = rng(u.n * 13 + 1), sr = st.r * 3 * (.6 + .4 * st.a);
        tmp.m4.compose(tmp.v.set(w[0], .03, w[2]), tmp.q.identity(), tmp.s.set(sr, 1, sr));
        s.puddle.add(tmp.m4, '#cdeeff');
        tmp.m4.compose(tmp.v.set(w[0], .04, w[2]), tmp.q.identity(), tmp.s.set(st.r * 2.4, 1, st.r * 2.4));
        s.decal.add(tmp.m4, new THREE.Color('#9fdcff').multiplyScalar(st.a * .5));
        for (let i = 0; i < (s.low ? 10 : 18); i++) {
          const an = R2() * Math.PI * 2, dd = Math.sqrt(R2()) * st.r * .85, ph = R2() * 6.28;
          const bob = .5 + .5 * Math.sin(frame.clock * (2 + i % 3) + ph), sz = .22 + R2() * .28;
          s.soft.add(w[0] + Math.cos(an) * dd, .12 + bob * .12, w[2] + Math.sin(an) * dd, sz * 1.6, sz, 0, st.a * (.6 + .3 * bob), i & 1 ? '#ffffff' : '#d4f0ff');
        }
        for (let i = 0; i < 5; i++) {
          const k = (frame.clock * .35 + i / 5) % 1, an = i * 1.9 + u.n;
          s.soft.add(w[0] + Math.cos(an) * st.r * .4, .2 + k * 1.8, w[2] + Math.sin(an) * st.r * .4, .18 + k * .1, .18 + k * .1, i, st.a * .7 * (1 - k), '#f2fbff');
        }
      } else {
        // Foam flash: a white pop and foam; water balloon: a small blue burst.
        const he = st.kind === 'he';
        s.glow.add(w[0], 1.3, w[2], st.r * 1.6, st.r * 1.6, 0, st.a * (he ? .7 : 1), he ? '#6fd0ff' : '#ffffff');
        for (let i = 0; i < 6; i++) {
          const an = i / 6 * Math.PI * 2 + u.n, d = st.r * .45 * (1.2 - st.a);
          s.soft.add(w[0] + Math.cos(an) * d, 1 + (i & 1) * .4, w[2] + Math.sin(an) * d, st.r * .5, st.r * .5, an, st.a * .6, he ? '#9fdcff' : '#ffffff');
        }
      }
    }
    s.nade.flush();
    // ---- shots: water streams, spray puffs, splashes ----
    const eyeW = s.eye ? mapToWorld(s.eye.x, s.eye.y, s.eye.h, size) : null;
    for (const sht of shots) {
      const mineShot = first && sht.from === watch.id;
      let A = mineShot && eyeW ? null : tips.get(sht.from);
      if (mineShot && eyeW) {
        // From the view model's muzzle: a little right and below the eye.
        const f = tmp.v2.set(0, 0, -1).applyQuaternion(ctx.camera.quaternion), r = tmp.v.set(1, 0, 0).applyQuaternion(ctx.camera.quaternion);
        A = [camPos.x + f.x * .9 + r.x * .14, camPos.y + f.y * .9 - .12, camPos.z + f.z * .9 + r.z * .14];
      }
      if (!A) continue;
      const dx = sht.bx - sht.ax, dy = sht.by - sht.ay, L = Math.hypot(dx, dy) || 1;
      const tgt = R.players[sht.to];
      const tc = tgt ? crouchOf(R, sht.to, gt) : 0;
      const bx = sht.bx + (-dy / L) * sht.miss + (sht.miss ? dx / L * 6 : 0), by = sht.by + (dx / L) * sht.miss + (sht.miss ? dy / L * 6 : 0);
      const B = mapToWorld(bx, by, (sht.hs ? 1.6 : 1.25) * (1 - tc * .3), size);
      const d = tmp.v2.set(B[0] - A[0], B[1] - A[1], B[2] - A[2]), len = d.length();
      if (len < .2) continue;
      d.divideScalar(len);
      const k = clamp(sht.age / .06, 0, 1), head = len * Math.min(1, k + .15), tail = Math.max(0, head - Math.min(4, len * .6));
      const mid = tmp.v.set(A[0] + d.x * (head + tail) / 2, A[1] + d.y * (head + tail) / 2, A[2] + d.z * (head + tail) / 2);
      tmp.q.setFromUnitVectors(tmp.x, d);
      tmp.m4.compose(mid, tmp.q, tmp.s.set(head - tail, .026, .026));
      const a = (1 - sht.age / .16) * .6;
      s.tracer.add(tmp.m4, new THREE.Color(.45, .78, 1).multiplyScalar(a));
      // Droplets along the stream, sagging a little (a slight arc).
      for (let j = 0; j < 3; j++) {
        const t = tail + (head - tail) * (j + .5) / 3, sag = Math.sin((t / len) * Math.PI) * Math.min(.25, len * .02);
        s.soft.add(A[0] + d.x * t, A[1] + d.y * t - sag, A[2] + d.z * t, .1, .1, 0, a * 1.2, '#cfeeff');
      }
      if (sht.age < .07 && !mineShot) {
        s.glow.add(A[0], A[1], A[2], .45, .45, sht.k.n, (1 - sht.age / .07) * .8, '#dff4ff');
        s.soft.add(A[0], A[1], A[2], .35, .35, sht.k.n, (1 - sht.age / .07) * .7, '#eaf8ff');
      }
    }
    s.tracer.flush();
    // Splashes: a burst of water off whoever gets soaked.
    for (const k of R.kills) {
      const age = gt - k.t;
      if (age < 0 || age > .6) continue;
      const w = mapToWorld(k.vx, k.vy, k.hs ? 1.55 : 1.2, size), f = 1 - age / .6;
      s.soft.add(w[0], w[1], w[2], .35 + age * 1.8, .35 + age * 1.8, k.n, .75 * f, '#bfe6ff');
      for (let j = 0; j < 5; j++) {
        const an = j / 5 * Math.PI * 2 + k.n, dd = age * 2.2;
        s.soft.add(w[0] + Math.cos(an) * dd, w[1] + age * 1.5 - age * age * 6, w[2] + Math.sin(an) * dd, .16 + age * .4, .16 + age * .4, an, .85 * f, j & 1 ? '#8fd8ff' : '#e6f7ff');
      }
    }
    s.decal.flush(); s.puddle.flush();
    s.soft.flush(cam); s.glow.flush(null);
    // ---- first person: the view model and the HUD ----
    this.viewUpdate(ctx, frame, R, gt, p, bomb, shots);
    this.hudUpdate(ctx, R, gt, p, rp, bomb, shots);
    void nd;
  },

  viewUpdate(ctx, frame, R, gt, p, bomb, shots) {
    const s = ctx.state, watch = ctx.watch || {}, first = watch.mode === 'first';
    const q = watch.id && R.players[watch.id];
    for (const m of s.vmModels.values()) m.visible = false;
    s.vmFlash.visible = false;
    s.scoped = false;
    if (!first || !q || !s.eye || s.eye.dead) return;
    const planting = !!(bomb.planting && bomb.planting.id === q.id), defusing = !!(bomb.defusing && bomb.defusing.id === q.id);
    s.scoped = !planting && !defusing && q.weapon === 'awp' && R.kills.some(k => k.killer === q.id && p >= k._p - .9 && p <= k._p + .1);
    if (s.scoped) return;
    const c4 = planting || defusing;
    const key = c4 ? 'c4|' + q.side : q.weapon + '|' + q.side;
    const m = this.viewModel(ctx, key, () => viewParts(ctx.kit, ctx.THREE, q.weapon, q.side, c4));
    m.visible = true;
    let kick = 0, own = null;
    for (const sh of shots) if (sh.from === q.id) { const kk = 1 - sh.age / .16; if (kk > kick) { kick = kk; own = sh; } }
    const run = clamp(speedAt(q, gt) / 4.5, 0, 1), ph = trackDist(q.track, gt) * 2.6;
    const bx = Math.sin(ph) * .012 * run, by = -Math.abs(Math.cos(ph)) * .01 * run;
    const pistol = weaponClass(q.weapon) === 'pistol', cls = weaponClass(q.weapon);
    if (c4) {
      m.position.set(.02 + bx, -.25 + by + (defusing ? Math.sin(frame.clock * 9) * .004 : 0), -.5);
      m.rotation.set(0, Math.PI / 2, .35); m.scale.setScalar(.62);
      return;
    }
    m.scale.setScalar(.78);
    m.position.set((pistol ? .13 : .16) + bx, (pistol ? -.15 : -.175) + by - kick * .01, (pistol ? -.3 : -.27) + kick * (cls === 'awp' ? .05 : .025));
    m.rotation.set(0, Math.PI / 2 + (pistol ? .07 : .09), .05 + kick * (cls === 'awp' ? .12 : .05));
    if (own && own.age < .07) {
      m.updateMatrix();
      const tip = new ctx.THREE.Vector3(tipOf(q.weapon), .045, 0).applyMatrix4(m.matrix);
      s.vmFlash.position.copy(tip); s.vmFlash.visible = true;
      const sz = (cls === 'awp' ? .26 : .18) * (1.2 - own.age / .07 * .4);
      s.vmFlash.scale.set(sz, sz, 1); s.vmFlash.material.rotation = own.k.n;
    }
  },

  hudUpdate(ctx, R, gt, p, rp, bomb, shots) {
    const s = ctx.state, H = s.hud, v = ctx.view, watch = ctx.watch || {};
    if (!H) return;
    const first = watch.mode === 'first', q = watch.id && R.players[watch.id];
    const eye = first && q ? s.eye : null;
    // The splash feed (team side colours, the watched player's splashes ringed).
    const feed = feedAt(R, p, 6, 5).map(k => {
      const kp = R.players[k.killer] || { handle: k.killer, side: '' }, vp = R.players[k.victim] || { handle: k.victim, side: '' };
      return `<div class="${k.hl ? 'hl' : ''}${q && (k.killer === q.id || k.victim === q.id) ? ' mine' : ''}"><b style="color:${SIDE_COLOR[kp.side] || '#fff'}">${esc(kp.handle)}</b><small>${esc(WEAPON[k.weapon] || k.weapon || '')}</small>${k.smoke ? '<small>☁</small>' : ''}${k.hs ? '<span class="hs">◎</span>' : ''}<b style="color:${SIDE_COLOR[vp.side] || '#fff'}">${esc(vp.handle)}</b></div>`;
    }).join('');
    H.html('feed', H.feed, feed);
    // Screen effects for the eye: fog you stand in, foam flashes, the burst, getting soaked.
    let white = 0, grey = 0, boom = 0;
    if (eye) {
      const yawMap = -eye.yaw;
      for (const u of R.utils) {
        if (u.type === 'flash') {
          const age = gt - u.t; if (age < 0 || age > 2) continue;
          const dist = Math.hypot(u.x - eye.x, u.y - eye.y); if (dist > 24 || !sees(s.G, eye.x, eye.y, u.x, u.y)) continue;
          const facing = Math.cos(Math.atan2(u.y - eye.y, u.x - eye.x) - yawMap);
          white = Math.max(white, (1 - age / 2) * clamp(1.25 - dist / 24, 0, 1) * (.3 + .7 * Math.max(0, facing)));
        } else if (u.type === 'smoke') {
          const st = utilState(u, gt, p); if (!st || st.kind !== 'smoke') continue;
          const dist = Math.hypot(st.x - eye.x, st.y - eye.y);
          if (dist < st.r) grey = Math.max(grey, st.a * clamp(1.2 - dist / st.r, 0, 1));
        }
      }
    }
    if (eye) boom = s.boomFx || 0;
    H.op('smk', H.smk, clamp(grey, 0, .92));
    H.op('wht', H.wht, clamp(white * 1.1, 0, .97));
    H.op('boom', H.boom, boom);
    H.op('dead', H.dead, eye && eye.dead ? eye.deathK : 0);
    H.show('scope', H.scope, !!(eye && s.scoped));
    H.show('x', H.x, !!(eye && !eye.dead && !s.scoped));
    let hit = 0, hs = false;
    if (eye) for (const sh of shots) if (sh.from === q.id && sh.kill && sh.age < .16) { hit = 1 - sh.age / .16; hs = sh.hs; }
    H.op('hit', H.hit, hit);
    H.style('hitc', H.hit, 'color', hs ? '#5cd2ff' : '#ffffff');
    // Splashed by…
    let kb = '';
    if (eye && eye.dead && eye.deathK > .3) {
      const k = R.kills.find(z => z.victim === q.id);
      if (k) { const kp = R.players[k.killer] || { handle: k.killer }; kb = `SPLASHED BY ${kp.handle} · ${WEAPON[k.weapon] || k.weapon}${k.hs ? ' · FACE SPLASH' : ''}`; }
    }
    H.show('kb', H.kb, !!kb); H.text('kbT', H.kb, kb);
    // Planting / draining the balloon (whoever it is).
    const act = bomb.planting ? { id: bomb.planting.id, prog: bomb.planting.prog, plant: true } : bomb.defusing ? { id: bomb.defusing.id, prog: bomb.defusing.prog, plant: false } : null;
    H.show('act', H.act, !!act);
    if (act) {
      const ap = R.players[act.id] || { handle: act.id };
      const total = act.plant ? R.plant.t - R.plant.start : R.defuse.t - R.defuse.start;
      H.text('actL', H.actL, act.plant ? `PLANTING THE BALLOON · ${ap.handle}` : `DRAINING · ${ap.handle}${R.defuse && R.defuse.kit ? ' · DRAIN KIT' : ''}`);
      H.text('actR', H.actR, `${Math.max(0, total * (1 - act.prog)).toFixed(1)}s`);
      H.style('actB', H.actBar, 'width', (act.prog * 100).toFixed(1) + '%');
      H.style('actC', H.actBar, 'background', act.plant ? '#ffa03a' : '#4fc3ff');
      H.style('actRC', H.actR, 'color', act.plant ? '#ffa03a' : '#4fc3ff');
    }
    // The player card: name, money, health, raincoat, water left.
    H.show('card', H.card, !!(first && q), 'flex');
    if (first && q) {
      const st = hudStats(R, q.id, gt), c4 = !!(act && act.id === q.id);
      H.style('sc', H.who, '--sc', SIDE_COLOR[q.side] || '#fff');
      H.text('whoS', H.whoS, `${q.side}${q.role ? ' · ' + q.role.toUpperCase() : ''}`);
      H.text('whoB', H.whoB, q.handle);
      H.text('cash', H.cash, money(st.money) + (bomb.carrier === q.id ? '  🎈' : ''));
      H.text('hp', H.hp, String(st.hp)); H.cls('hpl', H.hp, 'low', st.hp < 30);
      H.text('ar', H.ar, st.armor ? '100' : '0');
      H.text('amS', H.amS, c4 ? 'MEGA BALLOON' : (WEAPON[q.weapon] || q.weapon || '').toUpperCase());
      H.text('am', H.am, c4 ? '—' : `${st.ammo} / ${st.reserve}`);
      H.cls('aml', H.am, 'low', !c4 && st.ammo <= 5 && q.weapon !== 'awp');
    }
    void v;
  },

  hideAll(ctx) {
    const s = ctx.state;
    for (const m of s.models.values()) m.h.root.visible = false;
    if (s.bomb) s.bomb.visible = false;
    if (s.bombBack) s.bombBack.visible = false;
    if (s.vmModels) for (const m of s.vmModels.values()) m.visible = false;
    if (s.vmFlash) s.vmFlash.visible = false;
    for (const k of ['soft', 'glow', 'tracer', 'nade', 'decal', 'puddle']) if (s[k]) s[k].flush(null);
    s.eye = null; s.R = null;
    const H = s.hud;
    if (H) { H.html('feed', H.feed, ''); H.show('card', H.card, false); H.show('x', H.x, false); H.show('act', H.act, false); H.show('kb', H.kb, false); H.show('scope', H.scope, false); for (const k of ['smk', 'wht', 'boom', 'dead', 'hit']) H.op(k, H[k], 0); }
  },

  // Called before update(): read the view.
  entities(ctx) {
    const v = ctx.view, T = timesOf(v);
    if (!T) return [];
    return rosterOf(T.R, v.meta, T.gt);
  },

  autoEntity(ctx) {
    const v = ctx.view, T = timesOf(v), s = ctx.state;
    if (!T) return null;
    const director = v.phase === 'play' || v.phase === 'outro' ? v.view : null;
    const id = autoPick(T.R, T.p, T.gt, { director, replay: T.rp, prev: s.autoPrev && s.autoR === T.R ? s.autoPrev : null });
    s.autoPrev = id; s.autoR = T.R;
    return id;
  },

  cameraFor(ctx, id) {
    const s = ctx.state, R = s.R;
    if (!R || !R.players[id]) return null;
    const e = s.eye && ctx.watch && ctx.watch.id === id ? s.eye : eyeOf(R, id, s.gt, s.G);
    if (!e) return null;
    const pos = mapToWorld(e.x, e.y, e.h, s.size);
    // The balloon bursting nearby shakes the view.
    if (s.boom && s.boomShake > 0) { pos[0] += (Math.random() - .5) * s.boomShake; pos[1] += (Math.random() - .5) * s.boomShake * .5; pos[2] += (Math.random() - .5) * s.boomShake; }
    const cut = s.cut; s.cut = false;
    return { pos, yaw: e.yaw, pitch: e.pitch, fov: s.scoped ? 30 : 74, overlay: null, cut };
  },

  focus(ctx) {
    const s = ctx.state, R = s.R;
    if (!R) return [0, 1, 0];
    const w = ctx.watch && ctx.watch.id && R.players[ctx.watch.id];
    if (w) { const at = posAt(w.track, Math.min(s.gt, w.deathT)); return mapToWorld(at.x, at.y, 1, s.size); }
    let x = 0, y = 0, n = 0;
    for (const q of Object.values(R.players)) if (s.gt < q.deathT) { const at = posAt(q.track, s.gt); x += at.x; y += at.y; n++; }
    return n ? mapToWorld(x / n, y / n, 1, s.size) : [0, 1, 0];
  },

  freeStart() { return { yaw: 2.2, pitch: .8, dist: 42 }; },

  hud(ctx) {
    const v = ctx.view, s = ctx.state;
    if (!v.meta) return {};
    const T = v.T || TEXT.en, names = v.names ? v.names() : ['A', 'B'], score = v.score || [0, 0];
    const tags = ((v.meta && v.meta.teams) || []).map(t => t.tag);
    const t = timesOf(v), R = t ? t.R : null;
    const sideOf = i => (R ? (i === R.tTeam ? 'T' : 'CT') : '');
    const teams = [0, 1].map(i => ({ name: `${tags[i] || names[i]}${R ? ' · ' + sideOf(i) : ''}`, color: v.teamColor ? v.teamColor(i) : '#888', score: score[i] || 0, mark: !!R && sideOf(i) === 'T' }));
    const final = v.phase === 'final';
    let clock = '--:--', sub = '';
    if (final) clock = T.final;
    else if (R) {
      const b = bombState(R, t.gt), n = aliveAt(R, t.gt);
      if (b.planted && !b.defused && !b.exploded) clock = '🎈 ' + fmtClock(b.left);
      else if (v.phase === 'intro') clock = fmtClock(115);
      else if (b.planted) clock = '0:00';
      else clock = fmtClock(115 - t.gt);
      sub = `${T.round} ${R.num}${R.num > 2 * ((v.meta && v.meta.half) || 12) ? ' · OT' : ''} · ${n[0]}v${n[1]}`;
    } else sub = T.live;
    let tag = '';
    if (t && t.rp) tag = 'REPLAY · SPLASH CAM';
    else if (v.view && v.view.mode === 'pov' && ctx.watch && ctx.watch.id === v.view.id && v.phase === 'play') {
      const why = v.view.why;
      tag = why === 'kill' ? (v.view.k && v.view.k.first ? 'ENTRY' : 'DUEL') : why === 'plant' ? 'PLANT' : why === 'defuse' ? 'DRAIN' : why === 'clutch' ? 'CLUTCH' : '';
    }
    let caption = '';
    if (v.phase === 'intro' && R) {
      const call = c => (c ? String(c).replace(/_go_/, ' → ').replace(/_2_1_2/, ' 2-1-2').replace(/_/g, ' ').toUpperCase() : '');
      const tc = R.calls && R.calls.t, cc = R.calls && R.calls.ct;
      caption = `${T.round} ${R.num}${R.pistol ? ' · ' + T.pistol : ''}${tc ? ' · T: ' + call(tc.strategy) : ''}${cc ? ' · CT: ' + call(cc.setup) : ''}`;
    } else if (v.phase === 'wait' || !R) caption = v.match && v.match.status === 'final' ? '' : (v.rounds && v.rounds.length ? T.waiting : T.pregame);
    else if (v.phase === 'half') caption = T.half;
    else if (final) caption = `${T.final} · ${names[0]} ${score[0]} – ${score[1]} ${names[1]}`;
    else if (v.ticker && performance.now() - v.ticker.t0 < 4500) caption = v.ticker.text;
    let banner = null;
    if (R && R.end && v.banner && (v.phase === 'outro') && performance.now() - v.banner.t0 < 3200) {
      const w = R.end.winner;
      banner = { id: 'r' + R.num, text: `${names[w] || ''} ${T.wins}`.toUpperCase(), color: v.teamColor ? v.teamColor(w) : '#ffd400', color2: '#fff' };
    }
    void s;
    return { teams, clock, sub, tag, caption, banner };
  },

  dispose(ctx) {
    const s = ctx.state;
    if (s.hud && s.hud.el) s.hud.el.remove();
    if (s.bombBack && s.bombBack.parent) s.bombBack.parent.remove(s.bombBack);
    if (s.bombBack) s.bombBack.geometry.dispose();
  },
};

// Some wall faces spread round the map (for barrels and planters).
function mmFaces(mm, n, R) {
  const out = [], F = mm.faces;
  if (!F.length) return out;
  for (let i = 0; i < n; i++) {
    const f = F[Math.floor(R() * F.length)];
    const L = Math.hypot(f.bx - f.ax, f.by - f.ay);
    if (L < .3) continue;
    out.push({ x: (f.ax + f.bx) / 2, y: (f.ay + f.by) / 2, nx: f.nx, ny: f.ny });
  }
  return out;
}

export default adapter;
