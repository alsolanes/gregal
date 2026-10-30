// La simulació del Counter-Strike 2 en directe (internal/arena/cs2/live.go):
// una ronda de veritat, pas a pas (60 per segon), al navegador. Una persona
// porta un jugador amb teclat i ratolí; els altres nou són bots que segueixen
// el pla que el model ha triat per al seu equip (on esperen, quin lloc
// ataquen, quan entren, qui defensa on) i el rol que té cadascú (entry,
// support, lurker, AWPer, àncora, rifler): es mouen pel mateix mapa que
// dibuixa la vista en primera persona, veuen (parets, fums, flaixos) i
// disparen. El que passa es grava (pistes, morts, utilitat, bomba) i el
// servidor ho passa per les regles del motor: diners i rondes queden com en
// un partit normal.
//
// Aquí no hi ha DOM: el mòdul es prova a node (arena-cs2-live.test.cjs).

import { buildGrid, castRay, cellAt, angDiff, EYE, CRATE_H } from './arena-cs2-view.js';

const clamp = (v, a, b) => Math.max(a, Math.min(b, v));
const lerp = (a, b, t) => a + (b - a) * t;

export const TICK = 1 / 60;
export const ROUND_TIME = 115, BOMB_TIME = 40, FREEZE = 12, BUY_TIME = 20, PLANT_TIME = 3.2, KIT_TIME = 5, BARE_TIME = 10;
export const RUN = 4.8, WALK_K = 0.52, CROUCH_K = 0.36, RADIUS = 0.35, CROUCH_EYE = 1.15;

// Armes (valors de CS2 simplificats). rate: segons entre trets; spread: radians
// quiet; move: el que s'hi afegeix corrent; hs: probabilitat de cap d'un bot.
export const WEAPONS = {
  knife: { name: 'Knife', cls: 'knife', cost: 0, side: 0, dmg: 40, ap: 0.85, rate: 0.5, mag: 0, res: 0, reload: 0, spread: 0, move: 0, speed: 1.06, reward: 1500, hs: 0 },
  glock: { name: 'Glock-18', cls: 'pistol', cost: 0, side: 1, dmg: 30, ap: 0.47, rate: 0.15, mag: 20, res: 120, reload: 2.2, spread: 0.010, move: 0.035, speed: 1, reward: 300, hs: 0.32 },
  usp: { name: 'USP-S', cls: 'pistol', cost: 0, side: 2, dmg: 35, ap: 0.505, rate: 0.17, mag: 12, res: 24, reload: 2.2, spread: 0.007, move: 0.03, speed: 1, reward: 300, hs: 0.36 },
  p250: { name: 'P250', cls: 'pistol', cost: 300, side: 0, dmg: 38, ap: 0.64, rate: 0.15, mag: 13, res: 26, reload: 2.2, spread: 0.010, move: 0.035, speed: 1, reward: 300, hs: 0.34 },
  deagle: { name: 'Desert Eagle', cls: 'pistol', cost: 700, side: 0, dmg: 63, ap: 0.93, rate: 0.25, mag: 7, res: 35, reload: 2.2, spread: 0.012, move: 0.06, speed: 0.98, reward: 300, hs: 0.38 },
  mac10: { name: 'MAC-10', cls: 'smg', cost: 1050, side: 1, dmg: 29, ap: 0.575, rate: 0.075, mag: 30, res: 100, reload: 3.1, spread: 0.016, move: 0.03, speed: 0.96, reward: 600, hs: 0.22 },
  mp9: { name: 'MP9', cls: 'smg', cost: 1250, side: 2, dmg: 26, ap: 0.6, rate: 0.07, mag: 30, res: 120, reload: 2.1, spread: 0.015, move: 0.028, speed: 0.96, reward: 600, hs: 0.22 },
  galil: { name: 'Galil AR', cls: 'rifle', cost: 1800, side: 1, dmg: 30, ap: 0.775, rate: 0.09, mag: 35, res: 90, reload: 3.0, spread: 0.009, move: 0.06, speed: 0.9, reward: 300, hs: 0.28 },
  famas: { name: 'FAMAS', cls: 'rifle', cost: 2050, side: 2, dmg: 30, ap: 0.7, rate: 0.09, mag: 25, res: 90, reload: 3.3, spread: 0.009, move: 0.06, speed: 0.9, reward: 300, hs: 0.28 },
  ak47: { name: 'AK-47', cls: 'rifle', cost: 2700, side: 1, dmg: 36, ap: 0.775, rate: 0.1, mag: 30, res: 90, reload: 2.5, spread: 0.006, move: 0.07, speed: 0.86, reward: 300, hs: 0.32 },
  m4a1s: { name: 'M4A1-S', cls: 'rifle', cost: 2900, side: 2, dmg: 38, ap: 0.7, rate: 0.1, mag: 20, res: 80, reload: 3.1, spread: 0.005, move: 0.065, speed: 0.89, reward: 300, hs: 0.3 },
  awp: { name: 'AWP', cls: 'awp', cost: 4750, side: 0, dmg: 115, ap: 0.975, rate: 1.45, mag: 5, res: 30, reload: 3.7, spread: 0.07, scoped: 0.0012, move: 0.2, speed: 0.8, reward: 100, hs: 0.1 },
};
export const NADES = ['smoke', 'flash', 'molotov', 'he'];
export function nadeCost(n, side) { return n === 'flash' ? 200 : n === 'molotov' ? (side === 'CT' ? 500 : 400) : n === 'smoke' || n === 'he' ? 300 : 0; }
export const NADE_NAME = { smoke: 'Smoke', flash: 'Flashbang', molotov: 'Molotov', he: 'HE grenade' };

// Bot skill (the same for both teams: only the plans and the human differ).
export const SKILL = {
  easy: { react: 0.55, acc: 0.3, turn: 5, hs: 0.6 },
  normal: { react: 0.38, acc: 0.45, turn: 8, hs: 0.85 },
  hard: { react: 0.27, acc: 0.6, turn: 11, hs: 1.05 },
};

// Com juga cada rol: retard en sortir, reflexos, si empeny o aguanta.
export const ROLE = {
  entry: { delay: 0, react: 0.88, push: true },
  support: { delay: 1.4, react: 1, util: true },
  rifler: { delay: 0.8, react: 1 },
  awper: { delay: 2.6, react: 1.08, hold: true },
  lurker: { delay: 9, react: 0.95, lurk: true },
  anchor: { delay: 2, react: 0.95, anchor: true },
  // follow: va amb la persona (escorta): la segueix pel camí que fa, cobreix els
  // angles que no mira, entra darrere seu i intercanvia els seus duels.
  follow: { delay: 0.8, react: 0.95, follow: true },
};

// On fumen els T per tallar les rotacions CT de cada lloc.
export const SMOKES = { A: ['ct', 'jungle'], B: ['market', 'short'] };
// Per on arriben els T a cada lloc (els CT hi llancen molotovs).
export const T_ENTRY = { A: ['a_main', 'palace'], B: ['b_apps'] };

// ---- dany ----

// Un impacte: la vida i l'armilla que treu. zone: head | body | legs.
export function hitDamage(w, zone, armor, helmet) {
  let d = w.dmg * (zone === 'head' ? 4 : zone === 'legs' ? 0.75 : 1);
  if (armor > 0 && (zone !== 'head' || helmet) && zone !== 'legs') {
    const hp = d * w.ap, ar = (d - hp) * 0.5;
    if (ar > armor) return { hp: Math.round(hp + (ar - armor) * 2), armor };
    return { hp: Math.round(hp), armor: Math.round(ar) };
  }
  return { hp: Math.round(d), armor: 0 };
}

// Un raig des de l'ull (x, y, eye) en direcció ang amb pendent slope (tan del
// pitch): on toca un cos dret (o ajupit) a (tx, ty). Sense parets.
export function shotHit(sx, sy, eye, ang, slope, tx, ty, crouch) {
  const c = Math.cos(ang), s = Math.sin(ang), rx = tx - sx, ry = ty - sy;
  const depth = rx * c + ry * s; if (depth <= 0.15) return null;
  const lat = -rx * s + ry * c, z = eye + slope * depth, H = crouch ? 1.3 : 1.8;
  if (z < 0 || z > H + 0.02) return null;
  if (z >= H - 0.32) return Math.abs(lat) < 0.17 ? { d: depth, zone: 'head' } : null;
  return Math.abs(lat) < 0.3 ? { d: depth, zone: z < 0.8 ? 'legs' : 'body' } : null;
}

// ---- mapa: navegació ----

// Graella de pas: terra lliure amb una cel·la de marge (el cos no fa fregar les parets).
export function walkGrid(G) {
  const walk = new Uint8Array(G.gw * G.gh);
  for (let j = 1; j < G.gh - 1; j++) for (let i = 1; i < G.gw - 1; i++) {
    let ok = 1;
    for (let dj = -1; dj <= 1 && ok; dj++) for (let di = -1; di <= 1; di++) if (G.cells[(j + dj) * G.gw + i + di] !== 1) { ok = 0; break; }
    walk[j * G.gw + i] = ok;
  }
  return walk;
}

// Un cos (cercle) en (x, y) xoca amb paret o caixa?
export function blocked(G, x, y, r = RADIUS) {
  for (let a = 0; a < 8; a++) {
    const c = cellAt(G, x + Math.cos(a * Math.PI / 4) * r, y + Math.sin(a * Math.PI / 4) * r);
    if (c !== 1) return true;
  }
  return cellAt(G, x, y) !== 1;
}

// Mou un cos amb col·lisió (cada eix per separat: llisca per les parets).
export function moveBody(G, x, y, dx, dy, r = RADIUS) {
  let nx = x, ny = y;
  if (dx && !blocked(G, x + dx, y, r)) nx = x + dx;
  if (dy && !blocked(G, nx, y + dy, r)) ny = y + dy;
  return [nx, ny];
}

class Heap {
  constructor() { this.a = []; }
  push(k, p) { const a = this.a; a.push([p, k]); let i = a.length - 1; while (i > 0) { const j = (i - 1) >> 1; if (a[j][0] <= a[i][0]) break; [a[i], a[j]] = [a[j], a[i]]; i = j; } }
  pop() {
    const a = this.a, top = a[0], last = a.pop();
    if (a.length) { a[0] = last; let i = 0; for (;;) { const l = 2 * i + 1, r = l + 1; let m = i; if (l < a.length && a[l][0] < a[m][0]) m = l; if (r < a.length && a[r][0] < a[m][0]) m = r; if (m === i) break; [a[i], a[m]] = [a[m], a[i]]; i = m; } }
    return top[1];
  }
  get size() { return this.a.length; }
}

// La cel·la de pas més propera a un punt.
export function nearestWalk(N, x, y) {
  const { G, walk } = N, i0 = Math.floor(x / G.res), j0 = Math.floor(y / G.res);
  for (let r = 0; r < 30; r++) for (let dj = -r; dj <= r; dj++) for (let di = -r; di <= r; di++) {
    if (Math.max(Math.abs(di), Math.abs(dj)) !== r) continue;
    const i = i0 + di, j = j0 + dj;
    if (i >= 0 && j >= 0 && i < G.gw && j < G.gh && walk[j * G.gw + i]) return j * G.gw + i;
  }
  return -1;
}

// A* a la graella (8 veïns) i després estira el camí (línies rectes lliures).
export function findPath(N, ax, ay, bx, by) {
  const { G, walk } = N, W = G.gw, res = G.res;
  const s = nearestWalk(N, ax, ay), t = nearestWalk(N, bx, by);
  if (s < 0 || t < 0) return [[bx, by]];
  const g = N.g || (N.g = new Float64Array(W * G.gh)), from = N.from || (N.from = new Int32Array(W * G.gh)), stamp = N.stamp || (N.stamp = new Uint32Array(W * G.gh));
  N.run = (N.run || 0) + 1; const run = N.run;
  const tx = t % W, ty = (t / W) | 0, h = k => { const dx = Math.abs(k % W - tx), dy = Math.abs(((k / W) | 0) - ty); return Math.max(dx, dy) + 0.414 * Math.min(dx, dy); };
  const open = new Heap();
  g[s] = 0; from[s] = -1; stamp[s] = run; open.push(s, h(s));
  let found = false, n = 0;
  while (open.size && n++ < 250000) {
    const k = open.pop();
    if (k === t) { found = true; break; }
    const ki = k % W, kj = (k / W) | 0;
    for (let dj = -1; dj <= 1; dj++) for (let di = -1; di <= 1; di++) {
      if (!di && !dj) continue;
      const i = ki + di, j = kj + dj; if (i < 0 || j < 0 || i >= W || j >= G.gh) continue;
      const q = j * W + i; if (!walk[q]) continue;
      if (di && dj && (!walk[kj * W + i] || !walk[j * W + ki])) continue;
      const ng = g[k] + (di && dj ? 1.414 : 1);
      if (stamp[q] !== run || ng < g[q] - 1e-9) { stamp[q] = run; g[q] = ng; from[q] = k; open.push(q, ng + h(q)); }
    }
  }
  if (!found) return [[bx, by]];
  const cells = [];
  for (let k = t; k !== -1; k = from[k]) cells.push(k);
  cells.reverse();
  const pts = cells.map(k => [(k % W + 0.5) * res, (((k / W) | 0) + 0.5) * res]);
  pts[pts.length - 1] = walk[t] && !blocked(G, bx, by) ? [bx, by] : pts[pts.length - 1];
  // estira: de cada punt, el més llunyà que es veu en línia recta transitable
  const out = []; let i = 0, cx = ax, cy = ay;
  while (i < pts.length - 1) {
    let j = Math.min(pts.length - 1, i + 60);
    while (j > i + 1 && !clearLine(N, cx, cy, pts[j][0], pts[j][1])) j--;
    out.push(pts[j]); cx = pts[j][0]; cy = pts[j][1]; i = j;
  }
  if (!out.length) out.push(pts[pts.length - 1]);
  return out;
}

export function clearLine(N, ax, ay, bx, by) {
  const { G, walk } = N, d = Math.hypot(bx - ax, by - ay), n = Math.ceil(d / 0.25);
  for (let k = 1; k <= n; k++) {
    const x = ax + (bx - ax) * k / n, y = ay + (by - ay) * k / n;
    const i = Math.floor(x / G.res), j = Math.floor(y / G.res);
    if (!walk[j * G.gw + i]) return false;
  }
  return true;
}

// Zones: la zona d'un punt (o la més propera), camins de zona en zona.
export function zoneAt(meta, x, y) {
  let best = null, bd = 1e9;
  for (const z of meta.zones) {
    const dx = Math.max(0, Math.abs(x - z.x) - z.w / 2), dy = Math.max(0, Math.abs(y - z.y) - z.h / 2), d = Math.hypot(dx, dy);
    if (d < bd) { bd = d; best = z; }
  }
  return best;
}

export function inZone(z, x, y, pad = 0) { return z && Math.abs(x - z.x) <= z.w / 2 + pad && Math.abs(y - z.y) <= z.h / 2 + pad; }

export function zonePath(meta, from, to) {
  if (from === to) return [to];
  const adj = {}; for (const [a, b] of meta.links) { (adj[a] = adj[a] || []).push(b); (adj[b] = adj[b] || []).push(a); }
  const byId = {}; for (const z of meta.zones) byId[z.id] = z;
  const d = { [from]: 0 }, prev = {}, done = {};
  for (;;) {
    let cur = null, bd = Infinity;
    for (const k in d) if (!done[k] && d[k] < bd) { bd = d[k]; cur = k; }
    if (cur === null || cur === to) break;
    done[cur] = true;
    for (const n of adj[cur] || []) { const nd = bd + Math.hypot(byId[cur].x - byId[n].x, byId[cur].y - byId[n].y); if (d[n] === undefined || nd < d[n]) { d[n] = nd; prev[n] = cur; } }
  }
  if (d[to] === undefined) return [to];
  const p = [to]; while (p[0] !== from) p.unshift(prev[p[0]]);
  return p;
}

// Un punt lliure dins d'una zona (prop del centre, a l'atzar).
export function spotIn(N, z, rnd, spread = 0.6) {
  for (let i = 0; i < 30; i++) {
    const x = z.x + (rnd() - 0.5) * z.w * spread, y = z.y + (rnd() - 0.5) * z.h * spread;
    const k = Math.floor(y / N.G.res) * N.G.gw + Math.floor(x / N.G.res);
    if (N.walk[k]) return [x, y];
  }
  const k = nearestWalk(N, z.x, z.y);
  return [(k % N.G.gw + 0.5) * N.G.res, (((k / N.G.gw) | 0) + 0.5) * N.G.res];
}

// ---- la ronda ----

function rngFrom(seed) { let s = seed >>> 0 || 1; return () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296); }

export function mag(w) { return WEAPONS[w] ? WEAPONS[w].mag : 0; }

// LiveRound: una ronda. setup ve del servidor (LiveSetup); opts: { skill,
// humanBot (la persona també la porta un bot: proves i «pilot automàtic»), seed }.
export class LiveRound {
  constructor(meta, setup, opts = {}) {
    this.meta = meta; this.setup = setup; this.opts = opts;
    this.G = opts.grid || buildGrid(meta);
    this.N = opts.nav || { G: this.G, walk: walkGrid(this.G) };
    this.byZone = {}; for (const z of meta.zones) this.byZone[z.id] = z;
    this.rnd = rngFrom(opts.seed || (setup.round * 7919 + 13));
    this.skill = SKILL[opts.skill] || SKILL.normal;
    this.t = -FREEZE; // temps de ronda: negatiu durant la congelació
    this.over = false; this.winner = ''; this.reason = ''; this.endT = 0;
    this.kills = []; this.utils = []; this.flying = []; this.shots = []; this.feed = []; this.events = [];
    this.bomb = { carrier: setup.bomb, x: 0, y: 0, dropped: false, planted: false, plantT: 0, plantStart: 0, planting: null, defusing: null, defused: false, exploded: false, site: '' };
    this.plant = null; this.defuse = null;
    this.spotted = [{}, {}];
    this.calls = {}; // per equip: el lloc on els CT han vist els T
    this.players = setup.players.map(p => this.makePlayer(p));
    this.byId = {}; for (const p of this.players) this.byId[p.id] = p;
    this.human = this.byId[setup.human];
    this.sampleT = 0;
    this.later = [];
    // per on ha passat la persona (els escortes la segueixen pel mateix camí)
    this.trail = this.human ? [[this.human.x, this.human.y]] : [];
    // l'ordre de l'equip de la persona: null (cada bot el seu rol), 'all' (tots la segueixen), 'plan' (cap)
    this.teamOrder = opts.teamOrder || null;
  }

  // Una acció més tard, en temps de ronda (va amb els passos, no amb el rellotge).
  defer(delay, fn) { this.later.push({ t: this.t + delay, fn }); }

  makePlayer(p) {
    const side = p.side, w = WEAPONS[p.primary] ? p.primary : '';
    const pl = {
      id: p.id, handle: p.handle, role: p.role, team: p.team, side, human: !!p.human, bot: p.bot || 'rifler', note: p.note || '', group: p.group, post: p.post, route: p.route || [], start: p.start || 0,
      x: p.x, y: p.y, ang: side === 'T' ? Math.PI : 0, pitch: 0, vx: 0, vy: 0, crouch: false, walking: false,
      hp: 100, armor: p.armor > 0 ? 100 : 0, helmet: p.armor === 2, kit: !!p.kit, money: p.money, aim: p.aim || 1,
      primary: w, pistol: p.pistol || (side === 'CT' ? 'usp' : 'glock'), nades: (p.nades || []).slice(),
      slot: w ? 'primary' : 'pistol', ammo: {}, alive: true, deathT: 0, kills: 0, dmg: 0,
      nextShot: 0, reloadUntil: 0, heat: 0, flashUntil: -99, flashK: 0, scoped: false, punch: 0, lastShotT: -9,
      track: [[0, p.x, p.y]], ai: { mode: 'wait', path: [], goal: null, target: null, reactAt: 0, seenT: -9, lastSeen: null, burst: 0, pauseUntil: 0, think: 0, stuck: 0, utilDone: {}, heard: null, hold: null, stage: 'start' },
    };
    for (const k of [pl.primary, pl.pistol]) if (k) pl.ammo[k] = [mag(k), WEAPONS[k].res];
    if (side === 'T') pl.ang = Math.PI; // de dreta a esquerra
    return pl;
  }

  // ---- utilitats ----
  weapon(p) { return WEAPONS[p.slot === 'primary' ? p.primary : p.slot === 'pistol' ? p.pistol : 'knife'] || WEAPONS.knife; }
  weaponId(p) { return p.slot === 'primary' ? p.primary : p.slot === 'pistol' ? p.pistol : 'knife'; }
  enemies(p) { return this.players.filter(q => q.alive && q.team !== p.team); }
  mates(p) { return this.players.filter(q => q.alive && q.team === p.team && q !== p); }
  alive(side) { return this.players.filter(q => q.alive && q.side === side).length; }
  eyeOf(p) { return p.crouch ? CROUCH_EYE : EYE; }
  zoneOf(p) { return zoneAt(this.meta, p.x, p.y); }
  siteZone(s) { return this.byZone[s === 'A' ? 'a_site' : 'b_site']; }
  inSite(x, y) { for (const s of ['A', 'B']) if (inZone(this.siteZone(s), x, y, 0.5)) return s; return ''; }
  log(kind, data) { this.events.push({ kind, t: this.t, ...data }); }

  smokes(now = this.t) { return this.utils.filter(u => u.type === 'smoke' && now >= u.t && now < u.t + 18); }
  smokeR(u, now = this.t) { return 4.3 * clamp((now - u.t) / 1.2, 0.3, 1); }

  // La línia entre dos punts passa per un fum?
  smoked(ax, ay, bx, by) {
    for (const u of this.smokes()) {
      const r = this.smokeR(u), dx = bx - ax, dy = by - ay, l2 = dx * dx + dy * dy || 1;
      const k = clamp(((u.x - ax) * dx + (u.y - ay) * dy) / l2, 0, 1);
      if (Math.hypot(ax + dx * k - u.x, ay + dy * k - u.y) < r * 0.85) return true;
    }
    return false;
  }

  los(ax, ay, bx, by) {
    const d = Math.hypot(bx - ax, by - ay); if (d < 0.01) return true;
    const h = castRay(this.G, ax, ay, (bx - ax) / d, (by - ay) / d, d + 0.01);
    return !h.hit || h.d >= d - 0.3;
  }

  // p veu q? (camp de visió, parets, fums, flaix)
  sees(p, q) {
    if (this.t < p.flashUntil - 0.4) return false;
    const dx = q.x - p.x, dy = q.y - p.y, d = Math.hypot(dx, dy);
    if (d > 70) return false;
    if (d > 2.5 && Math.abs(angDiff(Math.atan2(dy, dx), p.ang)) > 1.05) return false;
    return this.los(p.x, p.y, q.x, q.y) && !this.smoked(p.x, p.y, q.x, q.y);
  }

  // ---- el pas ----

  // input: { fwd, side, walk, crouch, fire, scope, use, reload, slot, throw, dAng, dPitch, buy } de la persona.
  step(dt, input = {}) {
    if (this.over) { this.t += dt; return; }
    this.t += dt;
    const live = this.t >= 0;
    for (const p of this.players) {
      if (!p.alive) continue;
      p.heat = Math.max(0, p.heat - dt * (p.human ? 5 : 3.5));
      p.punch = Math.max(0, p.punch - dt * 0.9);
      if (p.reloadUntil && this.t >= p.reloadUntil) this.finishReload(p);
      if (p.human && !this.opts.humanBot) this.humanStep(p, dt, input, live);
      else if (live) this.botStep(p, dt);
    }
    if (!live) return;
    if (this.later.length) { const due = this.later.filter(x => this.t >= x.t); this.later = this.later.filter(x => this.t < x.t); due.forEach(x => x.fn()); }
    this.utilStep(dt);
    this.bombStep(dt);
    const hu = this.human;
    if (hu && hu.alive) { const l = this.trail[this.trail.length - 1]; if (!l || Math.hypot(hu.x - l[0], hu.y - l[1]) > 0.5) { this.trail.push([hu.x, hu.y]); if (this.trail.length > 400) this.trail.shift(); } }
    if (this.t - this.sampleT >= 0.25) { this.sampleT = this.t; for (const p of this.players) if (p.alive) p.track.push([+this.t.toFixed(2), +p.x.toFixed(2), +p.y.toFixed(2)]); }
    this.shots = this.shots.filter(s => this.t - s.t < 0.2);
    this.checkEnd();
  }

  // ---- la persona ----
  humanStep(p, dt, inp, live) {
    p.ang += inp.dAng || 0; p.pitch = clamp(p.pitch + (inp.dPitch || 0), -0.55, 0.55);
    if (inp.slot && inp.slot !== p.slot) this.switchTo(p, inp.slot);
    p.crouch = !!inp.crouch; p.walking = !!inp.walk;
    const busy = this.bomb.planting === p || this.bomb.defusing === p;
    let mx = 0, my = 0;
    if (live && !busy) {
      const c = Math.cos(p.ang), s = Math.sin(p.ang);
      mx = c * (inp.fwd || 0) - s * (inp.side || 0); my = s * (inp.fwd || 0) + c * (inp.side || 0);
      const l = Math.hypot(mx, my); if (l > 1) { mx /= l; my /= l; }
    }
    const sp = RUN * this.weapon(p).speed * (p.scoped ? 0.55 : 1) * (p.crouch ? CROUCH_K : p.walking ? WALK_K : 1);
    const k = Math.min(1, dt * 14);
    p.vx = lerp(p.vx, mx * sp, k); p.vy = lerp(p.vy, my * sp, k);
    [p.x, p.y] = moveBody(this.G, p.x, p.y, p.vx * dt, p.vy * dt);
    if (!live) return;
    if (inp.reload) this.startReload(p);
    if (inp.scope && this.weaponId(p) === 'awp' && !p.reloadUntil) p.scoped = !p.scoped;
    if (this.weaponId(p) !== 'awp') p.scoped = false;
    if (inp.throw) this.throwNade(p, inp.throw, p.ang, p.pitch);
    if (inp.fire) this.humanFire(p);
    this.useStep(p, dt, !!inp.use);
  }

  speedOf(p) { return Math.hypot(p.vx, p.vy); }

  // Imprecisió ara: l'arma, si corre o camina, si està ajupit, el retrocés.
  spreadOf(p) {
    const w = this.weapon(p), id = this.weaponId(p);
    let s = id === 'awp' ? (p.scoped ? w.scoped : w.spread) : w.spread;
    s += w.move * clamp(this.speedOf(p) / (RUN * 0.6), 0, 1);
    s += p.heat * (w.cls === 'pistol' ? 0.006 : w.cls === 'awp' ? 0 : 0.0045);
    return s * (p.crouch ? 0.75 : 1);
  }

  humanFire(p) {
    const id = this.weaponId(p), w = WEAPONS[id] || WEAPONS.knife;
    if (this.t < p.nextShot || p.reloadUntil) return;
    if (w.cls === 'knife') { p.nextShot = this.t + w.rate; this.knife(p); return; }
    const am = p.ammo[id]; if (!am) return;
    if (am[0] <= 0) { this.startReload(p); return; }
    am[0]--; p.nextShot = this.t + w.rate; p.lastShotT = this.t;
    const sp = this.spreadOf(p), r = this.rnd;
    const ang = p.ang + (r() - 0.5) * 2 * sp, slope = Math.tan(p.pitch + p.punch * 0.5) + (r() - 0.5) * 2 * sp;
    p.heat = Math.min(p.heat + 1, 12);
    p.punch = Math.min(0.12, p.punch + (w.cls === 'awp' ? 0.05 : w.cls === 'pistol' ? 0.012 : 0.008));
    if (w.cls === 'awp') p.scoped = false;
    this.fireRay(p, id, ang, slope);
    if (am[0] === 0) this.startReload(p);
  }

  // Un tret de la persona: el primer cos que toca abans de la paret.
  fireRay(p, id, ang, slope) {
    const w = WEAPONS[id], c = Math.cos(ang), s = Math.sin(ang), eye = this.eyeOf(p);
    const crates = [], wall = castRay(this.G, p.x, p.y, c, s, 95, crates);
    let wallD = wall.d;
    for (const cr of crates) { if (eye + slope * cr.d0 < CRATE_H) { wallD = Math.min(wallD, cr.d0); break; } }
    let best = null;
    for (const q of this.enemies(p)) {
      const h = shotHit(p.x, p.y, eye, ang, slope, q.x, q.y, q.crouch);
      if (h && h.d < wallD && (!best || h.d < best.h.d)) best = { q, h };
    }
    const endD = best ? best.h.d : wallD;
    this.shots.push({ t: this.t, from: p.id, ax: p.x, ay: p.y, bx: p.x + c * endD, by: p.y + s * endD, bz: eye + slope * endD, hit: !!best, hs: best && best.h.zone === 'head' });
    this.noise(p, 32);
    if (best) this.damage(best.q, p, id, best.h.zone, w);
  }

  knife(p) {
    for (const q of this.enemies(p)) {
      const d = Math.hypot(q.x - p.x, q.y - p.y);
      if (d < 1.7 && Math.abs(angDiff(Math.atan2(q.y - p.y, q.x - p.x), p.ang)) < 0.6) { this.damage(q, p, 'knife', 'body', WEAPONS.knife); return; }
    }
  }

  switchTo(p, slot) {
    if (slot === 'primary' && !p.primary) return;
    if (slot === p.slot) return;
    p.slot = slot; p.scoped = false; p.reloadUntil = 0; p.nextShot = Math.max(p.nextShot, this.t + 0.4);
  }

  startReload(p) {
    const id = this.weaponId(p), w = WEAPONS[id], am = p.ammo[id];
    if (!w || !am || p.reloadUntil || am[0] >= w.mag || am[1] <= 0) return;
    p.reloadUntil = this.t + w.reload; p.scoped = false;
  }

  finishReload(p) {
    const id = this.weaponId(p), w = WEAPONS[id], am = p.ammo[id]; p.reloadUntil = 0;
    if (!w || !am) return;
    const n = Math.min(w.mag - am[0], am[1]); am[0] += n; am[1] -= n;
  }

  // Plantar i desactivar (mantenint E, quiet).
  useStep(p, dt, use) {
    const b = this.bomb;
    if (p.side === 'T' && b.carrier === p.id && !b.planted) {
      const site = this.inSite(p.x, p.y);
      if (use && site && this.speedOf(p) < 0.6) {
        if (b.planting !== p) { b.planting = p; b.plantStart = this.t; }
        if (this.t - b.plantStart >= PLANT_TIME) this.doPlant(p, site);
      } else if (b.planting === p) b.planting = null;
    }
    if (p.side === 'CT' && b.planted && !b.defused && !b.exploded) {
      const near = Math.hypot(p.x - b.x, p.y - b.y) < 1.8;
      if (use && near && this.speedOf(p) < 0.6) {
        if (b.defusing !== p) { b.defusing = p; b.defuseStart = this.t; }
        if (this.t - b.defuseStart >= (p.kit ? KIT_TIME : BARE_TIME)) this.doDefuse(p);
      } else if (b.defusing === p) b.defusing = null;
    }
  }

  doPlant(p, site) {
    const b = this.bomb;
    Object.assign(b, { planted: true, plantT: this.t, x: p.x, y: p.y, site, planting: null, carrier: null });
    this.plant = { t: +this.t.toFixed(2), start: +b.plantStart.toFixed(2), id: p.id, x: +p.x.toFixed(2), y: +p.y.toFixed(2) };
    this.feed.push({ t: this.t, kind: 'plant', text: `${p.handle} planted the bomb at ${site}` });
    this.log('plant', { id: p.id, site });
    p.money += 300;
  }

  doDefuse(p) {
    const b = this.bomb;
    b.defused = true; b.defusing = null;
    this.defuse = { t: +this.t.toFixed(2), start: +b.defuseStart.toFixed(2), id: p.id };
    this.feed.push({ t: this.t, kind: 'defuse', text: `${p.handle} defused the bomb` });
    this.log('defuse', { id: p.id });
    this.end('CT', 'defuse');
  }

  // ---- dany i morts ----
  damage(q, from, wid, zone, w) {
    if (!q.alive) return;
    const h = hitDamage(w, zone, q.armor, q.helmet);
    q.armor = Math.max(0, q.armor - h.armor);
    const dealt = Math.min(q.hp, h.hp);
    q.hp -= h.hp;
    if (from && from.team !== q.team) from.dmg += dealt;
    q.ai.hitFrom = from ? { x: from.x, y: from.y, t: this.t } : null;
    if (q.hp <= 0) this.die(q, from, wid, zone === 'head');
  }

  die(q, k, wid, hs) {
    q.alive = false; q.hp = 0; q.deathT = this.t; q.scoped = false;
    q.track.push([+this.t.toFixed(2), +q.x.toFixed(2), +q.y.toFixed(2)]);
    const smoke = !!(k && this.smoked(k.x, k.y, q.x, q.y));
    const kill = { t: +this.t.toFixed(2), killer: k ? k.id : q.id, victim: q.id, weapon: wid, hs: !!hs, smoke };
    this.kills.push(kill);
    if (k && k !== q && k.team !== q.team) { k.kills++; k.money += (WEAPONS[wid] || { reward: 300 }).reward; }
    this.feed.push({ t: this.t, kind: 'kill', killer: k ? k.id : q.id, victim: q.id, weapon: wid, hs: !!hs });
    this.log('kill', kill);
    if (this.bomb.carrier === q.id) Object.assign(this.bomb, { carrier: null, dropped: true, x: q.x, y: q.y });
    if (this.bomb.planting === q) this.bomb.planting = null;
    if (this.bomb.defusing === q) this.bomb.defusing = null;
    q.killedBy = k && k !== q ? k.id : null;
    for (const m of this.mates(q)) m.ai.mateDied = { x: q.x, y: q.y, t: this.t, killer: q.killedBy };
  }

  end(side, reason) {
    if (this.over) return;
    this.over = true; this.winner = side; this.reason = reason; this.endT = this.t;
    this.log('end', { side, reason });
  }

  checkEnd() {
    if (this.over) return;
    const t = this.alive('T'), ct = this.alive('CT');
    if (ct === 0) return this.end('T', 'elim');
    if (t === 0 && !this.bomb.planted) return this.end('CT', 'elim');
    if (!this.bomb.planted && this.t >= ROUND_TIME) return this.end('CT', 'time');
  }

  bombStep() {
    const b = this.bomb;
    if (b.dropped && !b.planted) {
      for (const p of this.players) if (p.alive && p.side === 'T' && Math.hypot(p.x - b.x, p.y - b.y) < 1.1) { b.carrier = p.id; b.dropped = false; break; }
    }
    if (b.planted && !b.defused && !b.exploded && this.t >= b.plantT + BOMB_TIME) {
      b.exploded = true;
      for (const p of this.players) {
        if (!p.alive) continue;
        const d = Math.hypot(p.x - b.x, p.y - b.y);
        const dmg = d < 7 ? 500 : d < 24 ? 120 * (1 - (d - 7) / 17) * (p.armor ? 0.7 : 1) : 0;
        if (dmg > 0) { p.hp -= dmg; if (p.hp <= 0) this.die(p, null, 'c4', false); }
      }
      this.log('explode', {});
      this.end('T', 'bomb');
    }
  }

  // ---- utilitat ----
  // Llança una granada: on mira (fins a una paret, com a molt a 26 m).
  throwNade(p, type, ang, pitch) {
    const i = p.nades.indexOf(type); if (i < 0 || this.t < (p.nadeT || 0)) return false;
    const c = Math.cos(ang), s = Math.sin(ang), range = clamp(14 + pitch * 30, 4, 26);
    const hit = castRay(this.G, p.x, p.y, c, s, range);
    const d = Math.max(1, hit.d - 0.6);
    return this.throwTo(p, type, p.x + c * d, p.y + s * d);
  }

  throwTo(p, type, x, y) {
    const i = p.nades.indexOf(type); if (i < 0) return false;
    p.nades.splice(i, 1); p.nadeT = this.t + 1;
    const d = Math.hypot(x - p.x, y - p.y), fl = clamp(0.4 + d / 16, 0.5, 1.8);
    this.flying.push({ type, id: p.id, team: p.team, side: p.side, fx: p.x, fy: p.y, x, y, t0: this.t, t: this.t + fl });
    this.noise(p, 10);
    return true;
  }

  utilStep(dt) {
    for (const f of this.flying.slice()) {
      if (this.t < f.t) continue;
      this.flying.splice(this.flying.indexOf(f), 1);
      const u = { ...f, t: +f.t.toFixed(2), x: +f.x.toFixed(2), y: +f.y.toFixed(2), fx: +f.fx.toFixed(2), fy: +f.fy.toFixed(2) };
      this.utils.push(u);
      this.log('utility', u);
      if (u.type === 'flash') this.flashBang(u);
      if (u.type === 'he') this.heBlast(u);
    }
    // molotovs: cremen el que hi ha a sota
    for (const u of this.utils) {
      if (u.type !== 'molotov' || this.t < u.t || this.t > u.t + 7) continue;
      if (this.smokes().some(s => Math.hypot(s.x - u.x, s.y - u.y) < 3.5)) { u.out = true; continue; }
      if (u.out) continue;
      for (const p of this.players) {
        if (!p.alive || Math.hypot(p.x - u.x, p.y - u.y) > 2.6) continue;
        p.burn = (p.burn || 0) + dt * 38;
        if (p.burn >= 1) { const n = Math.floor(p.burn); p.burn -= n; p.hp -= n; const th = this.byId[u.id]; if (th && th.team !== p.team) th.dmg += n; if (p.hp <= 0) this.die(p, th && th.team !== p.team ? th : null, 'molotov', false); }
      }
    }
  }

  flashBang(u) {
    for (const p of this.players) {
      if (!p.alive) continue;
      const dx = u.x - p.x, dy = u.y - p.y, d = Math.hypot(dx, dy);
      if (d > 24 || !this.los(p.x, p.y, u.x, u.y) || this.smoked(p.x, p.y, u.x, u.y)) continue;
      const facing = Math.cos(angDiff(Math.atan2(dy, dx), p.ang));
      const k = clamp(1.25 - d / 24, 0, 1) * (0.25 + 0.75 * Math.max(0, facing));
      const dur = 0.4 + 3.2 * k;
      if (this.t + dur > p.flashUntil) { p.flashUntil = this.t + dur; p.flashK = k; }
    }
  }

  heBlast(u) {
    const th = this.byId[u.id];
    for (const p of this.players) {
      if (!p.alive) continue;
      const d = Math.hypot(p.x - u.x, p.y - u.y); if (d > 6 || !this.los(u.x, u.y, p.x, p.y)) continue;
      const dmg = Math.round(98 * (1 - d / 6) * (p.armor ? 0.57 : 1));
      p.hp -= dmg; if (th && th.team !== p.team) th.dmg += dmg;
      if (p.hp <= 0) this.die(p, th && th.team !== p.team ? th : null, 'he', false);
    }
  }

  // Soroll: els enemics a prop el senten (els bots es giren cap allà).
  noise(p, r) {
    for (const q of this.players) if (q.alive && q.team !== p.team && Math.hypot(q.x - p.x, q.y - p.y) < r) q.ai.heard = { x: p.x, y: p.y, t: this.t };
  }

  // ---- els bots ----
  botStep(p, dt) {
    const ai = p.ai, role = ROLE[p.bot] || ROLE.rifler;
    if (this.t >= ai.think) { ai.think = this.t + 0.1; this.perceive(p, role); this.plan(p, role); }
    // apuntar
    let want = ai.face != null ? ai.face : p.ang;
    const tg = ai.target && this.byId[ai.target];
    if (tg && tg.alive) want = Math.atan2(tg.y - p.y, tg.x - p.x);
    const turn = this.skill.turn * dt * (p.bot === 'awper' ? 0.8 : 1);
    const da = angDiff(want, p.ang); p.ang += clamp(da, -turn, turn);
    // moure's
    let mx = 0, my = 0;
    const fighting = tg && tg.alive && this.t >= ai.reactAt;
    const stop = fighting && !(role.push && this.speedOf(p) > 0 && this.rnd() < 0.3) || this.bomb.planting === p || this.bomb.defusing === p;
    if (!stop && ai.path.length) {
      const [wx, wy] = ai.path[0], dx = wx - p.x, dy = wy - p.y, d = Math.hypot(dx, dy);
      if (d < 0.35) { ai.path.shift(); if (!ai.path.length) ai.arrived = this.t; }
      else { mx = dx / d; my = dy / d; if (!tg && ai.face == null) p.ang += clamp(angDiff(Math.atan2(dy, dx), p.ang), -turn, turn); }
    }
    // no posar-se dins d'un company: una mica d'espai entre cossos
    for (const q of this.players) { if (q === p || !q.alive || q.team !== p.team) continue; const dx = p.x - q.x, dy = p.y - q.y, d = Math.hypot(dx, dy); if (d < 1 && d > 1e-3) { mx += dx / d * (1 - d) * 1.5; my += dy / d * (1 - d) * 1.5; } }
    { const l = Math.hypot(mx, my); if (l > 1) { mx /= l; my /= l; } }
    // l'escorta que s'ha quedat enrere corre una mica més per tornar al seu lloc
    const lag = ai.following && this.human.alive ? Math.hypot(this.human.x - p.x, this.human.y - p.y) : 0;
    const w = this.weapon(p), sp = RUN * w.speed * (ai.walk ? WALK_K : 1) * (lag > 4 ? 1.2 : 1);
    const k = Math.min(1, dt * 10);
    p.vx = lerp(p.vx, mx * sp, k); p.vy = lerp(p.vy, my * sp, k);
    const ox = p.x, oy = p.y;
    [p.x, p.y] = moveBody(this.G, p.x, p.y, p.vx * dt, p.vy * dt);
    if (mx || my) {
      if (Math.hypot(p.x - ox, p.y - oy) < sp * dt * 0.2) ai.stuck += dt; else ai.stuck = Math.max(0, ai.stuck - dt);
      if (ai.stuck > 0.6) { ai.stuck = 0; const g = ai.path[ai.path.length - 1]; if (g) ai.path = findPath(this.N, p.x, p.y, g[0], g[1]); }
    }
    if (this.speedOf(p) > RUN * 0.75 && this.rnd() < dt * 4) this.noise(p, 13);
    // disparar
    if (fighting && Math.abs(angDiff(want, p.ang)) < 0.09 + 0.6 / Math.max(2, Math.hypot(tg.x - p.x, tg.y - p.y))) this.botFire(p, tg);
    else if (!tg && p.reloadUntil === 0) { const am = p.ammo[this.weaponId(p)]; if (am && am[0] < mag(this.weaponId(p)) * 0.4 && this.t - ai.seenT > 2) this.startReload(p); }
    this.useStep(p, dt, ai.use);
  }

  perceive(p, role) {
    const ai = p.ai;
    let best = null, bd = 1e9;
    for (const q of this.enemies(p)) {
      if (!this.sees(p, q)) continue;
      const d = Math.hypot(q.x - p.x, q.y - p.y);
      if (d < bd) { bd = d; best = q; }
      this.spotted[p.team][q.id] = { x: q.x, y: q.y, t: this.t, zone: (this.zoneOf(q) || {}).id };
    }
    // qui ha matat (o està matant) la persona, primer
    if (ai.priority && this.t < ai.priority.until) { const q = this.byId[ai.priority.id]; if (q && q.alive && this.sees(p, q)) { best = q; bd = Math.hypot(q.x - p.x, q.y - p.y); } }
    if (best) {
      if (ai.target !== best.id) {
        // de lluny costa més adonar-se'n (i el que es mou es veu abans)
        const notice = bd > 18 ? (bd - 18) / 30 * (this.speedOf(best) > 1 ? 0.6 : 1) : 0;
        const react = (this.skill.react * role.react * (0.8 + this.rnd() * 0.45) * (1.15 - 0.4 * (p.aim - 0.95)) + notice) * (this.t < p.flashUntil ? 2 : 1);
        ai.reactAt = this.t + (ai.target && this.t - ai.seenT < 0.5 ? react * 0.5 : react);
        ai.target = best.id;
      }
      ai.seenT = this.t; ai.lastSeen = { x: best.x, y: best.y, t: this.t };
    } else if (ai.target && this.t - ai.seenT > 0.35) {
      ai.target = null;
    }
    if (!ai.target) {
      // girar-se cap al que se sent o cap a qui ens ha tocat
      const cue = ai.hitFrom && this.t - ai.hitFrom.t < 1.5 ? ai.hitFrom : ai.heard && this.t - ai.heard.t < 1.2 ? ai.heard : null;
      if (cue) ai.face = Math.atan2(cue.y - p.y, cue.x - p.x);
      else if (ai.lastSeen && this.t - ai.lastSeen.t < 2.5) ai.face = Math.atan2(ai.lastSeen.y - p.y, ai.lastSeen.x - p.x);
      else ai.face = ai.hold != null ? ai.hold : null;
    }
  }

  botFire(p, q) {
    const id = this.weaponId(p), w = WEAPONS[id], ai = p.ai;
    if (this.t < p.nextShot || p.reloadUntil || this.t < ai.pauseUntil) return;
    const am = p.ammo[id];
    if (w.cls === 'knife' || !am) { if (p.primary) this.switchTo(p, 'primary'); else this.switchTo(p, 'pistol'); return; }
    if (am[0] <= 0) { if (am[1] > 0) this.startReload(p); else this.switchTo(p, p.slot === 'primary' ? 'pistol' : 'primary'); return; }
    if (id === 'awp' && this.speedOf(p) > 1) return; // l'AWPer s'atura per disparar
    am[0]--; p.nextShot = this.t + w.rate; p.lastShotT = this.t;
    const d = Math.hypot(q.x - p.x, q.y - p.y);
    const reach = w.cls === 'pistol' ? 40 : w.cls === 'smg' ? 45 : w.cls === 'awp' ? 140 : 80;
    let acc = this.skill.acc * p.aim * clamp(1.08 - d / reach, 0.12, 1);
    if (id === 'awp') acc = Math.min(0.92, acc * 1.55);
    acc *= 1 - Math.min(0.65, w.move * 7 * clamp(this.speedOf(p) / RUN, 0, 1));
    acc /= 1 + p.heat * (w.cls === 'pistol' ? 0.2 : 0.1);
    if (this.speedOf(q) > RUN * 0.7) acc *= 0.82;
    if (q.crouch) acc *= 0.9;
    if (this.t < p.flashUntil) acc *= 0.2;
    p.heat = Math.min(p.heat + 1, 12);
    ai.burst++;
    if (ai.burst >= (d > 25 ? 3 : d > 12 ? 6 : 12)) { ai.burst = 0; ai.pauseUntil = this.t + 0.22 + this.rnd() * 0.15; }
    const hit = this.rnd() < acc;
    const hsP = w.hs * this.skill.hs * (q.crouch ? 0.8 : 1) * clamp(1.25 - d / reach, 0.12, 1);
    const zone = hit ? (this.rnd() < hsP ? 'head' : this.rnd() < 0.12 ? 'legs' : 'body') : '';
    const miss = hit ? 0 : (this.rnd() - 0.5) * 1.6;
    const bz = zone === 'head' ? (q.crouch ? 1.15 : 1.62) : hit ? 1.15 : 0.6 + this.rnd() * 1.4;
    const c = Math.cos(p.ang), s = Math.sin(p.ang);
    this.shots.push({ t: this.t, from: p.id, ax: p.x, ay: p.y, bx: q.x - s * miss, by: q.y + c * miss, bz, hit, hs: zone === 'head' });
    this.noise(p, 32);
    if (hit) this.damage(q, p, id, zone, w);
    if (am[0] === 0) this.startReload(p);
  }

  goTo(p, x, y) { p.ai.path = findPath(this.N, p.x, p.y, x, y); p.ai.goal = [x, y]; p.ai.arrived = 0; }

  // Seguir una ruta de zones (la del pla) fins a un punt final.
  goRoute(p, zones, final) {
    const pts = [];
    let cx = p.x, cy = p.y;
    const start = (this.zoneOf(p) || {}).id;
    for (const zid of zones) {
      if (zid === start) continue;
      const z = this.byZone[zid]; if (!z) continue;
      if (zid === zones[zones.length - 1]) break;
      const k = nearestWalk(this.N, z.x, z.y), nx = (k % this.G.gw + 0.5) * this.G.res, ny = (((k / this.G.gw) | 0) + 0.5) * this.G.res;
      pts.push(...findPath(this.N, cx, cy, nx, ny)); cx = nx; cy = ny;
    }
    const fz = this.byZone[zones[zones.length - 1]];
    const end = final || (fz ? spotIn(this.N, fz, this.rnd, 0.6) : [cx, cy]);
    pts.push(...findPath(this.N, cx, cy, end[0], end[1]));
    p.ai.path = pts; p.ai.goal = end; p.ai.arrived = 0;
  }

  facingFrom(zid, toward) {
    // de cara a la zona veïna per on arriba el rival
    const zp = zonePath(this.meta, zid, toward), n = this.byZone[zp[1] || zp[0]], z = this.byZone[zid];
    return n && z ? Math.atan2(n.y - z.y, n.x - z.x) : null;
  }

  // El pla de cada bot (4 cops per segon n'hi ha prou, però va amb el think).
  plan(p, role) {
    const ai = p.ai, S = this.setup, b = this.bomb;
    ai.use = false;
    const follow = this.follows(p);
    if (follow) { ai.following = true; return this.planFollow(p, ai, b); }
    if (ai.following) { ai.following = false; this.rejoin(p); }
    if (p.bot === 'follow' || this.teamOrder === 'all') role = ROLE.support; // sense ningú a seguir: support
    if (p.side === 'T') return this.planT(p, role, ai, S, b);
    return this.planCT(p, role, ai, S, b);
  }

  // Aquest bot escorta la persona ara?
  follows(p) {
    const h = this.human;
    if (!h || p === h || p.team !== h.team || p.ai.unfollow) return false;
    if (!h.alive) { if (this.t - h.deathT > 5) p.ai.unfollow = true; return this.t - h.deathT <= 5 && (p.bot === 'follow' || this.teamOrder === 'all'); }
    if (this.teamOrder === 'plan') return false;
    return this.teamOrder === 'all' || p.bot === 'follow';
  }

  followers() { return this.players.filter(q => q.alive && this.follows(q)).sort((a, c) => a.id < c.id ? -1 : 1); }

  // Un punt del camí de la persona, d metres enrere.
  trailBack(d) {
    const tr = this.trail, h = this.human;
    let px = h.x, py = h.y, acc = 0;
    for (let i = tr.length - 1; i >= 0; i--) {
      const [x, y] = tr[i], l = Math.hypot(x - px, y - py);
      if (acc + l >= d) { const k = (d - acc) / (l || 1); return [px + (x - px) * k, py + (y - py) * k]; }
      acc += l; px = x; py = y;
    }
    return [px, py];
  }

  planFollow(p, ai, b) {
    const h = this.human, fs = this.followers(), k = Math.max(0, fs.indexOf(p)), n = fs.length;
    // la persona ha mort: anar a intercanviar-la (primer el seu assassí), i després el pla
    if (!h.alive) {
      if (!ai.traded) { ai.traded = true; if (h.killedBy) ai.priority = { id: h.killedBy, until: this.t + 5 }; const kk = this.byId[h.killedBy]; this.goTo(p, kk && kk.alive ? kk.x : h.x, kk && kk.alive ? kk.y : h.y); }
      return;
    }
    // si li disparen, cap a qui li dispara
    const hit = h.ai.hitFrom;
    if (hit && this.t - hit.t < 1.5 && !ai.target) {
      const shooter = this.players.find(q => q.alive && q.team !== h.team && Math.hypot(q.x - hit.x, q.y - hit.y) < 3);
      if (shooter) ai.priority = { id: shooter.id, until: this.t + 3 };
      ai.face = Math.atan2(hit.y - p.y, hit.x - p.x);
      if (!ai.path.length || Math.hypot(ai.goal[0] - hit.x, ai.goal[1] - hit.y) > 3) this.goTo(p, (h.x + hit.x) / 2, (h.y + hit.y) / 2);
      return;
    }
    // cobrir els angles que la persona no mira
    const off = [Math.PI, Math.PI / 2, -Math.PI / 2, Math.PI * 0.75, -Math.PI * 0.75][k % 5];
    ai.hold = h.ang + (n === 1 ? Math.PI * 0.7 : off);
    // on posar-se: pel camí que ha fet, a 3-8 m (cadascú a la seva distància)
    const moving = this.speedOf(h) > 1;
    let tx, ty;
    if (moving) [tx, ty] = this.trailBack(2.4 + 1.3 * k);
    else { const a = h.ang + Math.PI + (k - (n - 1) / 2) * 0.9, r = 2.6 + 0.6 * (k % 2); tx = h.x + Math.cos(a) * r; ty = h.y + Math.sin(a) * r; }
    const cell = nearestWalk(this.N, tx, ty);
    if (cell >= 0) { tx = (cell % this.G.gw + 0.5) * this.G.res; ty = (((cell / this.G.gw) | 0) + 0.5) * this.G.res; }
    const dh = Math.hypot(h.x - p.x, h.y - p.y), dg = ai.goal ? Math.hypot(ai.goal[0] - tx, ai.goal[1] - ty) : 99;
    if (dh < 1.3 && !moving) ai.path = [];
    else if (Math.hypot(tx - p.x, ty - p.y) > 0.8 && clearLine(this.N, p.x, p.y, tx, ty)) { ai.path = [[tx, ty]]; ai.goal = [tx, ty]; }
    else if (dg > 1.2 || !ai.path.length && Math.hypot(tx - p.x, ty - p.y) > 0.8) this.goTo(p, tx, ty);
    ai.walk = h.walking && moving && dh < 8;
    // la utilitat per a la persona: quan entra en un lloc (T) o quan l'equip veu rivals a prop (CT)
    if (p.side === 'T') {
      const site = ['A', 'B'].find(s0 => { const z = this.siteZone(s0); return Math.hypot(z.x - h.x, z.y - h.y) < Math.max(z.w, z.h) / 2 + 8; });
      if (site && moving) {
        const sz = this.siteZone(site);
        if (!ai.utilDone.fsmoke && p.nades.includes('smoke')) { ai.utilDone.fsmoke = true; const z = this.byZone[SMOKES[site][k % 2]]; this.throwTo(p, 'smoke', z.x, z.y); }
        else if (!ai.utilDone.fflash && p.nades.includes('flash')) { ai.utilDone.fflash = true; this.throwTo(p, 'flash', sz.x, sz.y); }
      }
      // la bomba: si la porta l'escorta, planta quan són al lloc
      if (b.carrier === p.id && this.inSite(p.x, p.y) && !ai.target && this.inSite(h.x, h.y)) { ai.use = true; ai.path = []; }
    } else {
      const seen = Object.values(this.spotted[p.team]).filter(s0 => this.t - s0.t < 2 && Math.hypot(s0.x - h.x, s0.y - h.y) < 22);
      if (seen.length && !ai.utilDone.fnade) {
        const nade = p.nades.includes('molotov') ? 'molotov' : p.nades.includes('he') ? 'he' : '';
        if (nade) { ai.utilDone.fnade = true; this.throwTo(p, nade, seen[0].x, seen[0].y); }
      }
      if (b.planted && !b.defused && !ai.target && Math.hypot(p.x - b.x, p.y - b.y) < 1.6 && (!b.defusing || b.defusing === p) && b.plantT + BOMB_TIME - this.t > (p.kit ? KIT_TIME : BARE_TIME)) { ai.use = true; ai.path = []; }
    }
  }

  // Deixa d'escortar: torna al pla des d'on és.
  rejoin(p) {
    const ai = p.ai, from = (this.zoneOf(p) || {}).id;
    ai.path = []; ai.hold = null; ai.priority = null;
    if (p.side === 'T') { ai.stage = this.bomb.planted ? 'start' : 'stage'; if (!this.bomb.planted) this.goRoute(p, zonePath(this.meta, from, p.post)); }
    else { ai.stage = 'post'; this.goRoute(p, zonePath(this.meta, from, p.post)); ai.hold = this.facingFrom(p.post, 't_spawn'); }
  }

  planT(p, role, ai, S, b) {
    const site = S.site, sz = this.siteZone(site);
    const hitAt = S.hitAt + role.delay * (role.lurk ? 1 : 0.6) + (p.group === 'lurk' ? 8 : 0);
    // la bomba a terra: el més proper la va a buscar
    if (b.dropped && !b.planted) {
      const mates = this.players.filter(q => q.alive && q.side === 'T' && !(q.human && !this.opts.humanBot));
      const near = mates.sort((a, c) => Math.hypot(a.x - b.x, a.y - b.y) - Math.hypot(c.x - b.x, c.y - b.y))[0];
      if (near === p) { if (!ai.goal || Math.hypot(ai.goal[0] - b.x, ai.goal[1] - b.y) > 0.5) this.goTo(p, b.x, b.y); ai.stage = 'pickup'; return; }
    }
    if (b.planted) {
      // poc abans d'explotar, lluny de la bomba
      if (b.plantT + BOMB_TIME - this.t < 6 && ai.stage !== 'flee') { ai.stage = 'flee'; this.goRoute(p, zonePath(this.meta, (this.zoneOf(p) || {}).id, 't_spawn')); return; }
      if (ai.stage === 'flee') return;
      if (ai.stage !== 'post') {
        ai.stage = 'post';
        const a = this.rnd() * Math.PI * 2, r = 3 + this.rnd() * 5;
        const k = nearestWalk(this.N, b.x + Math.cos(a) * r, b.y + Math.sin(a) * r);
        this.goTo(p, (k % this.G.gw + 0.5) * this.G.res, (((k / this.G.gw) | 0) + 0.5) * this.G.res);
        ai.hold = this.facingFrom(sz.id, 'ct_spawn');
      }
      return;
    }
    const carrier = b.carrier === p.id;
    if (ai.stage === 'start' && this.t >= p.start) {
      ai.stage = 'stage';
      if (S.t.strategy.startsWith('rush')) { ai.stage = 'hit'; this.goRoute(p, S.routes['t_spawn>' + sz.id] || p.route, carrier ? this.plantSpot(site) : null); return; }
      this.goRoute(p, p.route); ai.walk = S.t.pace === 'slow';
      ai.hold = this.facingFrom(p.post, sz.id);
      return;
    }
    // finta: utilitat a l'altre lloc
    if (p.group === 'fake' && S.fake && ai.stage === 'stage' && S.fakeAt && this.t >= S.fakeAt && !ai.utilDone.fake) {
      ai.utilDone.fake = true;
      const fz = this.siteZone(S.fake);
      for (const t of SMOKES[S.fake]) if (p.nades.includes('smoke')) { const z = this.byZone[t]; this.throwTo(p, 'smoke', z.x, z.y); break; }
      if (p.nades.includes('flash')) this.defer(1.5, () => p.alive && this.throwTo(p, 'flash', fz.x, fz.y));
      if (p.nades.includes('molotov')) this.defer(2.5, () => p.alive && this.throwTo(p, 'molotov', fz.x, fz.y));
    }
    const lateHit = ROUND_TIME - this.t < 30 && !b.planted;
    if ((ai.stage === 'stage' || ai.stage === 'pickup') && (this.t >= hitAt || lateHit)) {
      ai.stage = 'hit'; ai.walk = false;
      const from = (this.zoneOf(p) || {}).id;
      const route = S.routes[from + '>' + sz.id] || zonePath(this.meta, from, sz.id);
      this.goRoute(p, route, carrier ? this.plantSpot(site) : null);
      // la utilitat de l'execució: fums a les rotacions CT, flaix a sobre del lloc
      if (!S.t.strategy.startsWith('rush') && S.t.utility !== 'none') {
        const targets = SMOKES[site];
        const mates = this.players.filter(q => q.alive && q.side === 'T');
        const i = mates.indexOf(p);
        if (p.nades.includes('smoke') && i >= 0) { const z = this.byZone[targets[i % targets.length]]; this.throwTo(p, 'smoke', z.x + (this.rnd() - 0.5) * 2, z.y + (this.rnd() - 0.5) * 2); }
      }
    }
    if (ai.stage === 'hit') {
      // el support (o qui tingui flaixos) en fa un a sobre del lloc quan hi arriba l'entry
      const dz = Math.hypot(sz.x - p.x, sz.y - p.y);
      if (!ai.utilDone.flash && p.nades.includes('flash') && dz < Math.max(sz.w, sz.h) * 0.5 + 9 && (role.util || p.bot === 'rifler')) { ai.utilDone.flash = true; this.throwTo(p, 'flash', sz.x, sz.y); }
      if (!ai.utilDone.molly && p.nades.includes('molotov') && dz < Math.max(sz.w, sz.h) * 0.5 + 6) { ai.utilDone.molly = true; const ps = this.plantSpot(site); this.throwTo(p, 'molotov', ps[0] - 5, ps[1] + (this.rnd() - 0.5) * 4); }
      if (carrier && this.inSite(p.x, p.y) === site && !ai.target) {
        if (!ai.path.length || ai.arrived) { ai.use = true; ai.hold = null; }
      } else if (carrier && !ai.path.length && !ai.target) this.goTo(p, ...this.plantSpot(site));
      if (!ai.path.length && !carrier && !ai.target) {
        // al lloc: mirar cap a les entrades dels CT
        ai.hold = this.facingFrom(sz.id, 'ct_spawn');
        if (role.push && ai.lastSeen && this.t - ai.lastSeen.t < 4 && this.t - ai.lastSeen.t > 1) this.goTo(p, ai.lastSeen.x, ai.lastSeen.y);
      }
    }
  }

  plantSpot(site) {
    const pt = (this.meta.plant || {})[site] || [this.siteZone(site).x, this.siteZone(site).y];
    const k = nearestWalk(this.N, pt[0] + (this.rnd() - 0.5) * 2, pt[1] + (this.rnd() - 0.5) * 2);
    return [(k % this.G.gw + 0.5) * this.G.res, (((k / this.G.gw) | 0) + 0.5) * this.G.res];
  }

  planCT(p, role, ai, S, b) {
    if (ai.stage === 'start' && this.t >= p.start) {
      ai.stage = 'post';
      this.goRoute(p, p.route);
      ai.hold = this.facingFrom(p.post, 't_spawn');
      return;
    }
    if (ai.stage === 'start') return;
    // bomba plantada: tothom cap al lloc; qui hi arriba sense ningú a la vista desactiva
    if (b.planted) {
      const sz = this.siteZone(b.site);
      if (ai.stage !== 'retake' && ai.stage !== 'defuse') { ai.stage = 'retake'; const from = (this.zoneOf(p) || {}).id; this.goRoute(p, S.routes[from + '>' + sz.id] || zonePath(this.meta, from, sz.id), [b.x + (this.rnd() - 0.5) * 3, b.y + (this.rnd() - 0.5) * 3]); }
      const d = Math.hypot(p.x - b.x, p.y - b.y);
      const others = this.players.filter(q => q.alive && q.side === 'CT' && q !== p && (b.defusing === q || Math.hypot(q.x - b.x, q.y - b.y) < d));
      const left = b.plantT + BOMB_TIME - this.t, need = p.kit ? KIT_TIME : BARE_TIME;
      // ja no hi arriba: salvar-se
      if (b.defusing !== p && left < need + Math.max(0, d - 1.6) / RUN && ai.stage !== 'flee') { ai.stage = 'flee'; this.goRoute(p, zonePath(this.meta, (this.zoneOf(p) || {}).id, 'ct_spawn')); return; }
      if (ai.stage === 'flee') return;
      if (d < 1.6 && !ai.target && (!b.defusing || b.defusing === p) && left > need - 0.2 && (others.length === 0 || b.defusing === p)) { ai.stage = 'defuse'; ai.use = true; ai.path = []; }
      else if (ai.stage === 'defuse' && ai.target) ai.stage = 'retake';
      else if (!ai.path.length && d > 1.6 && !ai.target) this.goTo(p, b.x + 0.6, b.y + 0.6);
      return;
    }
    // rotar: on els companys veuen (o senten) T
    const seen = Object.values(this.spotted[p.team]).filter(s => this.t - s.t < 3);
    for (const site of ['A', 'B']) {
      const sz = this.siteZone(site);
      const near = seen.filter(s => { const z = this.byZone[s.zone]; return z && (z.site === site || z.id === sz.id); }).length;
      const util = this.utils.some(u => this.t - u.t < 3 && u.side === 'T' && inZone(sz, u.x, u.y, 3));
      const call = near >= 2 || near >= 1 && util;
      if (call && !this.calls[site]) this.calls[site] = this.t;
      // una finta també fa rotar (més els del mig), com al motor
      if (util && !near && !this.calls['fake' + site] && this.rnd() < 0.5) this.calls['fake' + site] = this.t;
    }
    const callSite = ['A', 'B'].filter(s => this.calls[s] != null).sort((a, c) => this.calls[c] - this.calls[a])[0] ||
      ['A', 'B'].find(s => this.calls['fake' + s] != null && (p.group === 'mid' || this.rnd() < 0.02));
    if (callSite && p.group !== callSite && ai.rotated !== callSite && !role.anchor && !role.lurk) {
      const since = this.t - (this.calls[callSite] != null ? this.calls[callSite] : this.calls['fake' + callSite]);
      if (since >= role.delay * 0.6 + (p.group === 'mid' ? 0 : 1)) {
        ai.rotated = callSite; ai.stage = 'rotate';
        const sz = this.siteZone(callSite), from = (this.zoneOf(p) || {}).id;
        this.goRoute(p, S.routes[from + '>' + sz.id] || zonePath(this.meta, from, sz.id));
        ai.hold = this.facingFrom(sz.id, 't_spawn');
      }
    }
    // utilitat: molotov o HE a l'entrada per on arriben
    if (!ai.utilDone.stall) {
      const post = this.byZone[p.post], site = post && post.site;
      if (site) {
        for (const ez of T_ENTRY[site]) {
          if (seen.some(s => s.zone === ez)) {
            const nade = p.nades.includes('molotov') ? 'molotov' : p.nades.includes('he') ? 'he' : '';
            const z = this.byZone[ez];
            if (nade && Math.hypot(z.x - p.x, z.y - p.y) < 28) { ai.utilDone.stall = true; this.throwTo(p, nade, z.x + (this.rnd() - 0.5) * 3, z.y + (this.rnd() - 0.5) * 3); }
            break;
          }
        }
      }
    }
    // l'entry CT (o qui empeny) va a buscar el darrer T vist, si fa poc
    if (role.push && !ai.target && ai.lastSeen && this.t - ai.lastSeen.t > 1 && this.t - ai.lastSeen.t < 4 && !ai.path.length) this.goTo(p, ai.lastSeen.x, ai.lastSeen.y);
    else if (!ai.target && !ai.path.length && ai.goal && Math.hypot(ai.goal[0] - p.x, ai.goal[1] - p.y) > 3 && ai.stage === 'post') this.goTo(p, ...ai.goal);
  }

  // Què fa cada bot (per al HUD de l'equip).
  doing(p) {
    if (!p.alive) return 'dead';
    const ai = p.ai, b = this.bomb;
    if (b.planting === p) return 'planting';
    if (b.defusing === p) return 'defusing';
    if (ai.target && this.t >= ai.reactAt) return 'fighting';
    const z = ai.goal ? zoneAt(this.meta, ai.goal[0], ai.goal[1]) : this.zoneOf(p);
    const zn = z ? z.name : '';
    if (ai.following) return this.human.alive ? (ai.target ? 'fighting' : 'covering you') : 'trading you';
    if (ai.stage === 'pickup') return 'getting the bomb';
    if (ai.stage === 'post') return p.side === 'T' ? 'post-plant' : (ai.path.length ? '→ ' : 'holding ') + zn;
    if (ai.stage === 'retake') return 'retaking ' + (b.site || '');
    if (ai.stage === 'rotate') return 'rotating → ' + zn;
    if (ai.stage === 'hit') return 'hitting ' + this.setup.site;
    if (ai.stage === 'stage') return (ai.path.length ? '→ ' : 'waiting at ') + zn;
    return '';
  }

  // El registre per al servidor (LiveResult).
  result(humanLoadout) {
    const tracks = {}, dmg = {};
    for (const p of this.players) { tracks[p.id] = p.track; dmg[p.id] = Math.round(p.dmg); }
    return {
      round: this.setup.round, human: humanLoadout, tracks, dmg, kills: this.kills,
      utils: this.utils.map(u => ({ t: u.t, id: u.id, type: u.type, x: u.x, y: u.y, fx: u.fx, fy: u.fy })),
      plant: this.plant, defuse: this.defuse, explode: this.bomb.exploded ? +(this.bomb.plantT + BOMB_TIME).toFixed(2) : 0, endT: +this.endT.toFixed(2),
    };
  }
}
