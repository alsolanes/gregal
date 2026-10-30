// Visor d'un partit de futbol americà entre els vents (internal/football):
// càmera de televisió des de la banda (perspectiva), camp amb franges, línies,
// números i zones de marca amb els colors de cada equip, jugadors amb el seu
// color, dorsal i casc, la pilota amb l'altura dels xuts i les passades, el
// marcador a dalt i el comentari a baix. Cada jugada es reconstrueix a partir
// del que ha passat (tipus, iardes, qui l'ha fet) amb formacions i rutes.
//
// El fa servir la pàgina «Partits» de l'app (football.js) i, al mòbil, la
// pàgina autònoma football-viewer.html dins d'un WebView.

const FIELD_W = 53.33;
const lerp = (a, b, t) => a + (b - a) * t;
const clamp = (v, a, b) => Math.max(a, Math.min(b, v));
const ease = t => t < .5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
const easeOut = t => 1 - Math.pow(1 - t, 3);
const seg = (t, a, b) => clamp((t - a) / (b - a), 0, 1);

// Generador d'atzar amb llavor (la mateixa jugada es veu igual cada cop).
function rng(seed) { let s = seed >>> 0 || 1; return () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296); }

function hexToRgb(h) { const m = /^#?([0-9a-f]{6})$/i.exec(h || ''); if (!m) return [120, 120, 120]; const n = parseInt(m[1], 16); return [n >> 16, (n >> 8) & 255, n & 255]; }
function shade(h, f) { const [r, g, b] = hexToRgb(h); const k = v => clamp(Math.round(f < 0 ? v * (1 + f) : v + (255 - v) * f), 0, 255); return `rgb(${k(r)},${k(g)},${k(b)})`; }
function luminance(h) { const [r, g, b] = hexToRgb(h); return (0.299 * r + 0.587 * g + 0.114 * b) / 255; }

export const TEXT = {
  ca: { q: ['1r', '2n', '3r', '4t', 'Pr.'], down: ['', '1a', '2a', '3a', '4a'], and: 'i', goal: 'gol', td: 'TOUCHDOWN!', fg: 'GOL DE CAMP!', int: 'INTERCEPCIÓ!', fumble: 'PILOTA PERDUDA!', safety: 'SAFETY!', sack: 'SACK!', final: 'FINAL', half: 'DESCANS', pregame: 'Els entrenadors preparen el partit…', live: 'EN DIRECTE', waiting: 'Esperant la jugada següent…', mvp: 'MVP' },
  en: { q: ['1st', '2nd', '3rd', '4th', 'OT'], down: ['', '1st', '2nd', '3rd', '4th'], and: '&', goal: 'Goal', td: 'TOUCHDOWN!', fg: 'IT\'S GOOD!', int: 'INTERCEPTED!', fumble: 'FUMBLE!', safety: 'SAFETY!', sack: 'SACK!', final: 'FINAL', half: 'HALFTIME', pregame: 'The coaches are preparing the game…', live: 'LIVE', waiting: 'Waiting for the next play…', mvp: 'MVP' },
};

// ---- formacions i rutes ----

// Direcció d'atac: els equips canvien de camp cada quart.
export function attackDir(team, q) { return (team === 0) === (q % 2 === 1) ? 1 : -1; }
// Iarda del camp (0..120, amb les zones de marca) d'un punt de l'atac.
export function fieldX(spot, dir) { return dir > 0 ? 10 + spot : 110 - spot; }

const C = FIELD_W / 2;
function offenseSet(kind, detail) {
  const s = [];
  // [pos, dx (enrere és negatiu), y]
  [-4, -2, 0, 2, 4].forEach(y => s.push(['OL', -0.6, C + y * 0.55]));
  const shotgun = kind === 'pass' || kind === 'sack' || kind === 'scramble' || detail === 'screen';
  s.push(['QB', shotgun ? -5 : -1.4, C]);
  s.push(['RB', shotgun ? -5.2 : -6.5, C + (shotgun ? 1.8 : 0)]);
  s.push(['WR', -0.8, C - 19], ['WR', -0.8, C + 19], ['WR', -1.6, C + 12]);
  s.push(['TE', -0.8, C - 4.2]);
  return s;
}
function defenseSet() {
  const s = [];
  [-4.5, -1.5, 1.5, 4.5].forEach(y => s.push(['DL', 1.1, C + y * 0.62]));
  [-5.5, 0, 5.5].forEach(y => s.push(['LB', 5, C + y]));
  s.push(['CB', 6.5, C - 18.5], ['CB', 6.5, C + 18.5]);
  s.push(['S', 13, C - 8], ['S', 13.5, C + 8]);
  return s;
}

// Reparteix els jugadors d'un equip en una formació (per posició, en ordre).
function assign(team, set) {
  const byPos = {};
  team.players.forEach(p => { (byPos[p.pos] = byPos[p.pos] || []).push(p); });
  const used = {};
  return set.map(([pos, dx, y]) => {
    const i = used[pos] = (used[pos] || 0) + 1;
    const p = (byPos[pos] || [])[i - 1] || { id: pos + i, name: pos, number: 0, pos };
    return { p, dx, y };
  });
}

// buildPlay converteix una jugada en pistes d'animació: cada jugador té una
// funció de posició al llarg de t (0..1) i la pilota té x, y, z (altura).
export function buildPlay(play, match) {
  const R = rng(play.n * 9973 + 17);
  const off = match.teams[play.off], def = match.teams[1 - play.off];
  const dir = attackDir(play.off, play.q);
  const los = fieldX(play.spot, dir);
  const endX = fieldX(clamp(play.end, -5, 105), dir);
  const A = play.actors || {};
  const kind = play.kind;
  const tracks = [];
  const add = (p, team, path, extra = {}) => tracks.push({ p, team, path, ...extra });
  const still = (x, y) => () => ({ x, y });
  const towards = (x0, y0, x1, y1, a, b, f = ease) => t => { const k = f(seg(t, a, b)); return { x: lerp(x0, x1, k), y: lerp(y0, y1, k) }; };
  let ball = () => ({ x: los, y: C, z: 0.3 });
  let dur = 3.4, carrierId = '';

  const isKick = kind === 'kickoff' || kind === 'punt';
  if (kind === 'kickoff') {
    // Atac = qui rep; xuta l'altre des de la seva 35.
    const kx = fieldX(65, dir); // la 35 del rival vista des de qui rep
    const recvX = fieldX(clamp(play.end < 30 && play.detail === 'touchback' ? -2 : Math.min(play.end, 8), -5, 100), dir);
    const retEnd = endX;
    assign(def, defenseSet().concat(offenseSet('run')).slice(0, 11)).forEach((a, i) => {
      const y = 4 + i * 4.5;
      const x0 = kx + dir * 1.5, x1 = lerp(kx, retEnd, .85) + dir * (R() * 4 - 2);
      add(a.p, 1 - play.off, towards(x0, y, x1, lerp(y, C, .5), .05, .95, t => t));
    });
    const ret = (off.players.find(p => p.id === A.returner)) || off.players[0];
    assign(off, offenseSet('run')).forEach((a, i) => {
      if (a.p.id === ret.id) return;
      const y = 6 + i * 4;
      add(a.p, play.off, towards(fieldX(30, dir), y, lerp(fieldX(30, dir), retEnd, .7), lerp(y, C, .3), .3, .95));
    });
    if (play.detail === 'touchback') {
      add(ret, play.off, still(recvX, C));
      ball = t => { const k = seg(t, .05, .75); return { x: lerp(kx, recvX - dir * 2, k), y: C, z: 0.5 + 11 * Math.sin(Math.PI * k) }; };
      dur = 3.2;
    } else {
      const catchX = fieldX(3, dir);
      add(ret, play.off, t => t < .55 ? { x: catchX, y: C } : towards(catchX, C, retEnd, C + (R() * 10 - 5), .55, .97, easeOut)(t));
      ball = t => { if (t < .55) { const k = seg(t, .05, .55); return { x: lerp(kx, catchX, k), y: C, z: 0.5 + 12 * Math.sin(Math.PI * k) }; } const q = tracks.find(tr => tr.p.id === ret.id).path(t); return { x: q.x, y: q.y, z: 1.1 }; };
      carrierId = ret.id;
      dur = 5.2;
    }
    return { tracks, ball, dur, carrierId, los, endX, dir };
  }

  const offA = assign(off, offenseSet(kind, play.detail));
  const defA = assign(def, defenseSet());
  const qb = offA.find(a => a.p.pos === 'QB');
  const px = dx => los - dir * Math.abs(dx); // darrere la línia
  const dxF = dx => los + dir * dx; // davant

  if (kind === 'punt' || kind === 'fg' || kind === 'pat') {
    const k = off.players.find(p => p.id === A.kicker) || off.players.find(p => p.pos === 'K');
    const kickFrom = kind === 'punt' ? px(13) : px(7);
    offA.forEach(a => { if (a.p.pos === 'OL' || a.p.pos === 'TE') add(a.p, play.off, still(los - dir * 0.6, a.y)); else if (a.p.pos !== 'QB') add(a.p, play.off, towards(px(2), a.y, kind === 'punt' ? dxF(25) : px(2), a.y, .3, .95)); });
    if (kind !== 'punt') add(qb.p, play.off, still(px(7) + dir * .1, C + 1.2)); // el que aguanta
    add(k, play.off, towards(kickFrom - dir * 2, C - .6, kickFrom, C - .4, .1, .3));
    defA.forEach(a => add(a.p, 1 - play.off, towards(dxF(a.dx), a.y, dxF(a.dx * 0.4), lerp(a.y, C, .4), .2, .6)));
    if (kind === 'punt') {
      const land = endX;
      ball = t => { if (t < .12) return { x: lerp(los, kickFrom, t / .12), y: C, z: 1 }; const k2 = seg(t, .25, .8); return { x: lerp(kickFrom, land, k2), y: C + Math.sin(k2 * 3), z: 1 + 11 * Math.sin(Math.PI * k2) }; };
      const ret = def.players.find(p => p.id === A.returner);
      if (ret) add(ret, 1 - play.off, still(land, C));
      dur = 4.6;
    } else {
      const goalX = dir > 0 ? 120 : 0;
      const good = !!play.good;
      const wide = (R() < .5 ? -1 : 1) * 5;
      ball = t => { if (t < .2) return { x: lerp(los, kickFrom, t / .2), y: C, z: 1 }; const k2 = seg(t, .3, .85); return { x: lerp(kickFrom, goalX + dir * 3, k2), y: C + (good ? 0 : wide * k2), z: 1 + 3.5 * k2 + 4 * Math.sin(Math.PI * k2 * .8) }; };
      dur = 3.6;
    }
    return { tracks, ball, dur, carrierId, los, endX, dir, goalPost: kind !== 'punt' };
  }

  // Línia: l'atac bloqueja i la defensa empeny.
  offA.filter(a => a.p.pos === 'OL').forEach(a => add(a.p, play.off, t => ({ x: px(a.dx) + dir * .4 * Math.sin(t * 8 + a.y), y: a.y + Math.sin(t * 6 + a.y) * .2 })));
  const dlPath = (a, target) => t => { const k = seg(t, .12, .9); return { x: lerp(dxF(a.dx), target ? target(t).x : los + dir * .3, k * (target ? 1 : .4)), y: lerp(a.y, target ? target(t).y : a.y, k * (target ? 1 : .2)) }; };

  const runner = off.players.find(p => p.id === A.runner);
  const target = off.players.find(p => p.id === A.target);
  const tackler = def.players.find(p => p.id === A.tackler);
  const defender = def.players.find(p => p.id === A.defender);
  let carrierPath = null;

  if (kind === 'run' || kind === 'scramble' || kind === 'kneel' || (kind === 'two' && !A.target)) {
    const lane = play.dir === 'left' ? C - 4 : play.dir === 'right' ? C + 4 : C;
    const rb = offA.find(a => a.p.id === (runner || {}).id) || offA.find(a => a.p.pos === 'RB');
    if (kind === 'kneel') {
      add(qb.p, play.off, t => ({ x: px(qb.dx) - dir * seg(t, .2, .4), y: C }));
      ball = t => ({ x: px(qb.dx) - dir * seg(t, .2, .4), y: C, z: .6 - .3 * seg(t, .2, .5) });
      offA.filter(a => a.p.pos !== 'OL' && a.p.pos !== 'QB').forEach(a => add(a.p, play.off, still(px(a.dx), a.y)));
      defA.forEach(a => add(a.p, 1 - play.off, still(dxF(a.dx), a.y)));
      return { tracks, ball, dur: 1.8, carrierId: qb.p.id, los, endX, dir };
    }
    const carrier = kind === 'scramble' ? qb : rb;
    const wob = R() * 6 - 3;
    carrierPath = t => {
      if (kind === 'scramble') {
        if (t < .4) return towards(px(qb.dx), C, px(qb.dx + 2), C + wob, .1, .4)(t);
        return towards(px(qb.dx + 2), C + wob, endX, C + wob * 2, .4, .9, easeOut)(t);
      }
      if (t < .32) return towards(px(carrier.dx), carrier.y, los - dir * .5, lane, .12, .32)(t);
      return towards(los - dir * .5, lane, endX, lane + wob, .32, .9, easeOut)(t);
    };
    add(carrier.p, play.off, carrierPath, { carrier: true });
    carrierId = carrier.p.id;
    if (carrier !== qb) add(qb.p, play.off, t => ({ x: px(qb.dx) + dir * .5 * seg(t, .1, .2), y: lerp(C, lane, seg(t, .1, .2) * .3) }));
    ball = t => { const c = t < .2 && carrier !== qb ? { x: lerp(px(qb.dx), carrierPath(t).x, seg(t, .1, .2)), y: lerp(C, carrierPath(t).y, seg(t, .1, .2)) } : carrierPath(t); return { x: c.x, y: c.y, z: 1.1 }; };
    offA.filter(a => !['OL', 'QB'].includes(a.p.pos) && a.p.id !== carrier.p.id).forEach(a => add(a.p, play.off, towards(px(a.dx), a.y, dxF(5 + R() * 4), a.y + (C - a.y) * .2, .12, .8)));
  } else if (kind === 'sack') {
    const qbPath = t => towards(px(qb.dx), C, px(qb.dx + 2.5), C + (R() - .5), .1, .45)(t);
    add(qb.p, play.off, qbPath, { downAt: .62, carrier: true });
    carrierId = qb.p.id;
    ball = t => { const q = qbPath(t); return { x: q.x, y: q.y, z: t > .62 ? .4 : 1.3 }; };
    offA.filter(a => !['OL', 'QB'].includes(a.p.pos)).forEach(a => add(a.p, play.off, towards(px(a.dx), a.y, dxF(10 + R() * 8), a.y, .12, .9)));
    if (defender) { const da = defA.find(a => a.p.id === defender.id) || defA[0]; add(defender, 1 - play.off, towards(dxF(da.dx), da.y, qbPath(.62).x + dir * .4, qbPath(.62).y + .5, .12, .6)); }
  } else {
    // Passada (també el dos punts per l'aire i el spike).
    const tgtA = offA.find(a => a.p.id === (target || {}).id) || offA.find(a => a.p.pos === 'WR');
    const depth = play.detail === 'screen' ? -2 : Math.max(2, play.depth || 8);
    const side = tgtA.y < C ? -1 : 1;
    const catchY = clamp(tgtA.y + (play.detail === 'deep' ? -side * 4 : play.detail === 'medium' ? -side * 7 : -side * 3), 3, FIELD_W - 3);
    const catchX = los + dir * depth;
    const dropX = px(qb.dx + (play.detail === 'deep' ? 3 : 1.5));
    add(qb.p, play.off, towards(px(qb.dx), C, dropX, C, .08, .35));
    const tThrow = play.detail === 'deep' ? .5 : .4, tCatch = play.detail === 'deep' ? .75 : .58;
    const tgtPath = t => {
      if (t < tCatch) return towards(px(tgtA.dx), tgtA.y, catchX, catchY, .08, tCatch, t => t)(t);
      if (play.complete) return towards(catchX, catchY, endX, catchY + (R() * 6 - 3), tCatch, .92, easeOut)(t);
      return towards(catchX, catchY, catchX + dir * 2, catchY, tCatch, .8)(t);
    };
    add(tgtA.p, play.off, tgtPath, { carrier: !!play.complete });
    if (play.complete) carrierId = tgtA.p.id;
    offA.filter(a => !['OL', 'QB'].includes(a.p.pos) && a.p.id !== tgtA.p.id).forEach(a => {
      const route = R();
      add(a.p, play.off, t => { const k = seg(t, .08, .8); const fx = px(a.dx) + dir * (route < .4 ? 22 : 10) * k; const fy = a.y + (route > .7 ? (a.y < C ? 5 : -5) * seg(k, .5, 1) : 0); return { x: fx, y: fy }; });
    });
    const throwFrom = () => ({ x: dropX, y: C });
    ball = t => {
      if (t < tThrow) { const q = towards(px(qb.dx), C, dropX, C, .08, .35)(t); return { x: q.x, y: q.y, z: 1.6 }; }
      if (t < tCatch) { const k = seg(t, tThrow, tCatch), f = throwFrom(); const peak = play.detail === 'deep' ? 6 : play.detail === 'medium' ? 3.5 : 1.8; return { x: lerp(f.x, catchX, k), y: lerp(f.y, catchY, k), z: 1.6 + peak * Math.sin(Math.PI * k) }; }
      if (play.int) { const d = defender ? tracks.find(tr => tr.p.id === defender.id) : null; const q = d ? d.path(t) : { x: catchX, y: catchY }; return { x: q.x, y: q.y, z: 1.1 }; }
      if (!play.complete) { const k = seg(t, tCatch, tCatch + .15); return { x: catchX + dir * 2 * k, y: catchY, z: Math.max(.1, 1.6 * (1 - k)) }; }
      const q = tgtPath(t); return { x: q.x, y: q.y, z: 1.1 };
    };
    if (defender && play.int) {
      const da = defA.find(a => a.p.id === defender.id) || defA[defA.length - 1];
      add(defender, 1 - play.off, t => t < tCatch ? towards(dxF(da.dx), da.y, catchX, catchY, .1, tCatch)(t) : towards(catchX, catchY, catchX - dir * (6 + R() * 10), catchY + (R() * 6 - 3), tCatch, .95, easeOut)(t), { carrier: true });
      carrierId = defender.id;
    } else if (defender) {
      const da = defA.find(a => a.p.id === defender.id) || defA[0];
      add(defender, 1 - play.off, towards(dxF(da.dx), da.y, catchX + dir * .8, catchY + .6, .1, tCatch));
    }
    carrierPath = tgtPath;
  }

  // La resta de la defensa: persegueix la pilota; el placador hi arriba.
  const used = new Set(tracks.map(tr => tr.p.id));
  defA.forEach(a => {
    if (used.has(a.p.id)) return;
    const isTackler = tackler && a.p.id === tackler.id;
    if (a.p.pos === 'DL' && !isTackler) { add(a.p, 1 - play.off, dlPath(a)); return; }
    const lag = isTackler ? 0 : 1.5 + R() * 3;
    add(a.p, 1 - play.off, t => {
      const b = ball(Math.max(0, t - .05)) || { x: los, y: C };
      const k = seg(t, .15, isTackler ? .9 : 1);
      const tx = b.x - dir * (isTackler ? .6 : lag), ty = b.y + (isTackler ? .4 : (a.y - b.y) * .25);
      return { x: lerp(dxF(a.dx), tx, isTackler ? ease(k) : k * .85), y: lerp(a.y, ty, isTackler ? ease(k) : k * .8) };
    }, { tackle: isTackler });
  });
  offA.forEach(a => { if (!tracks.find(tr => tr.p.id === a.p.id)) add(a.p, play.off, still(px(a.dx), a.y)); });
  defA.forEach(a => { if (!tracks.find(tr => tr.p.id === a.p.id)) add(a.p, 1 - play.off, still(dxF(a.dx), a.y)); });
  if (kind === 'pass' && (play.detail === 'deep')) dur = 4.4;
  if (play.td) dur += .4;
  return { tracks, ball, dur, carrierId, los, endX, dir };
}

// ---- dibuix ----

export class FootballView {
  constructor(canvas, opts = {}) {
    this.cv = canvas; this.ctx = canvas.getContext('2d');
    this.lang = opts.lang === 'ca' ? 'ca' : 'en';
    this.T = TEXT[this.lang];
    this.source = opts.source; // { load(after) → Promise<match> }
    this.onUpdate = opts.onUpdate || (() => {});
    this.onEvent = opts.onEvent || (() => {});
    this.match = null; this.queue = []; this.idx = 0; this.speed = 1; this.paused = false;
    this.cur = null; this.t = 0; this.phase = 'wait'; this.hold = 0; this.camX = 60; this.particles = [];
    this.banner = null; this.lastN = 0; this.stopped = false; this.last = performance.now();
    this.resize = this.resize.bind(this); window.addEventListener('resize', this.resize); this.resize();
    this.loop = this.loop.bind(this); requestAnimationFrame(this.loop);
    this.poll();
  }

  destroy() { this.stopped = true; window.removeEventListener('resize', this.resize); clearTimeout(this.pollTimer); }
  setSpeed(x) { this.speed = x; }
  togglePause() { this.paused = !this.paused; return this.paused; }
  skipToEnd() { this.idx = this.queue.length; this.cur = null; this.phase = 'wait'; this.banner = null; }
  // Ja s'ha vist tot (la retransmissió passa al partit següent).
  isFinished() { return !!this.match && ['final', 'stopped', 'failed'].includes(this.match.status) && this.idx >= this.queue.length && !this.cur; }

  async poll() {
    if (this.stopped) return;
    try {
      const m = await this.source.load(this.lastN);
      if (!this.match) this.match = m; else { Object.assign(this.match, { ...m, teams: this.match.teams, plays: this.match.plays }); }
      const fresh = m.plays || [];
      if (!this.match.plays || this.match.plays === fresh) this.match.plays = [];
      for (const p of fresh) { if (p.n > this.lastN) { this.queue.push(p); this.match.plays.push(p); this.lastN = p.n; } }
      this.onUpdate(this.match, this.shown());
    } catch (e) { /* torna-ho a provar */ }
    const done = this.match && ['final', 'stopped', 'failed'].includes(this.match.status);
    if (!done) this.pollTimer = setTimeout(() => this.poll(), 3000);
  }

  shown() { return this.queue[Math.min(this.idx, this.queue.length) - 1] || null; }

  resize() {
    const r = this.cv.getBoundingClientRect(), d = window.devicePixelRatio || 1;
    this.cv.width = Math.max(320, r.width * d); this.cv.height = Math.max(200, r.height * d);
    this.W = this.cv.width; this.H = this.cv.height; this.dpr = d;
    // Càmera de tele: des de la banda de baix, una mica alta.
    const yards = this.W / this.H > 1.4 ? 44 : 30;
    this.focal = 61.65 * this.W / yards;
    const near = .96 * this.H, far = .3 * this.H;
    this.camH = (near - far) / (this.focal * (1 / 30 - 1 / (30 + FIELD_W)));
    this.horizon = near - this.camH * this.focal / 30;
  }

  proj(x, y, z = 0) {
    const depth = 30 + (FIELD_W - y);
    const sc = this.focal / depth;
    return { x: this.W / 2 + (x - this.camX) * sc, y: this.horizon + this.camH * sc - z * sc, s: sc };
  }

  loop(now) {
    if (this.stopped) return;
    const dt = Math.min(.25, (now - this.last) / 1000); this.last = now;
    if (!this.paused) this.advance(dt * this.speed);
    this.draw();
    requestAnimationFrame(this.loop);
  }

  advance(dt) {
    if (!this.match) return;
    if (!this.cur) {
      if (this.idx < this.queue.length) {
        const p = this.queue[this.idx++];
        this.cur = { play: p, anim: buildPlay(p, this.match) };
        this.t = 0; this.phase = 'pre'; this.hold = .7;
        this.onUpdate(this.match, p);
        this.onEvent(p);
      }
      return;
    }
    if (this.phase === 'pre') { this.hold -= dt; if (this.hold <= 0) this.phase = 'run'; return; }
    if (this.phase === 'run') {
      this.t += dt / this.cur.anim.dur;
      if (this.t >= 1) {
        this.t = 1; this.phase = 'post';
        const p = this.cur.play;
        this.hold = (p.comment ? 3.2 : 1.6) + (p.hl ? 1 : 0);
        this.celebrate(p);
      }
      return;
    }
    if (this.phase === 'post') { this.hold -= dt; if (this.hold <= 0) { this.cur = null; this.banner = null; } }
  }

  celebrate(p) {
    const T = this.T;
    const team = this.match.teams[p.td && p.int ? 1 - p.off : p.off];
    let text = '';
    if (p.td) text = T.td; else if ((p.kind === 'fg') && p.good) text = T.fg; else if (p.int) text = T.int; else if (p.fumble) text = T.fumble; else if (p.safety) text = T.safety; else if (p.kind === 'sack' && p.hl) text = T.sack;
    if (!text) return;
    const scorer = p.td ? this.match.teams[(p.int ? 1 - p.off : p.off)] : (p.int || p.fumble || p.safety ? this.match.teams[1 - p.off] : team);
    this.banner = { text, color: scorer.color, color2: scorer.color2, t0: performance.now() };
    if (p.td || (p.kind === 'fg' && p.good)) {
      const b = this.cur.anim.ball(1) || { x: this.cur.anim.endX, y: C, z: 1 };
      for (let i = 0; i < 160; i++) this.particles.push({ x: b.x, y: b.y, z: 2, vx: (Math.random() - .5) * 18, vy: (Math.random() - .5) * 18, vz: 8 + Math.random() * 14, c: Math.random() < .5 ? scorer.color : scorer.color2, life: 2 + Math.random() });
    }
  }

  draw() {
    const g = this.ctx, W = this.W, H = this.H;
    // Cel i graderia.
    const sky = g.createLinearGradient(0, 0, 0, H * .35);
    sky.addColorStop(0, '#0b1020'); sky.addColorStop(1, '#1b2440');
    g.fillStyle = sky; g.fillRect(0, 0, W, H);
    if (!this.match) { this.drawCenterText(this.T.pregame); return; }
    const a = this.cur && this.cur.anim;
    const b = a && a.ball(this.t);
    const focusX = b ? b.x : (this.cur ? this.cur.anim.los : this.camX);
    this.camX = lerp(this.camX, clamp(focusX, 14, 106), .06);
    this.drawStands();
    this.drawField();
    if (a) this.drawPlay(a, b);
    this.drawParticles();
    this.drawHUD();
  }

  drawStands() {
    const g = this.ctx, W = this.W;
    const top = this.proj(this.camX, FIELD_W + 6).y;
    const grad = g.createLinearGradient(0, 0, 0, top);
    grad.addColorStop(0, '#141a2e'); grad.addColorStop(1, '#2a3354');
    g.fillStyle = grad; g.fillRect(0, 0, W, top);
    // Públic: punts de colors.
    const R = rng(7);
    for (let i = 0; i < 380; i++) { const x = R() * W, y = R() * top * .92; g.fillStyle = `hsla(${R() * 360},45%,${45 + R() * 30}%,.55)`; g.fillRect(x, y, 2 * this.dpr, 2 * this.dpr); }
    // Llums de l'estadi.
    for (let i = 0; i < 6; i++) { const x = (i + .5) * W / 6; const lg = g.createRadialGradient(x, top * .1, 0, x, top * .1, W * .12); lg.addColorStop(0, 'rgba(255,255,240,.35)'); lg.addColorStop(1, 'rgba(255,255,240,0)'); g.fillStyle = lg; g.fillRect(x - W * .12, 0, W * .24, top * .5); }
  }

  quad(x0, y0, x1, y1, fill) {
    const g = this.ctx, a = this.proj(x0, y0), b = this.proj(x1, y0), c = this.proj(x1, y1), d = this.proj(x0, y1);
    g.beginPath(); g.moveTo(a.x, a.y); g.lineTo(b.x, b.y); g.lineTo(c.x, c.y); g.lineTo(d.x, d.y); g.closePath(); g.fillStyle = fill; g.fill();
  }

  line(x0, y0, x1, y1, color, w) {
    const g = this.ctx, a = this.proj(x0, y0), b = this.proj(x1, y1);
    g.strokeStyle = color; g.lineWidth = w * a.s * .12; g.beginPath(); g.moveTo(a.x, a.y); g.lineTo(b.x, b.y); g.stroke();
  }

  groundText(text, x, y, size, color, flip) {
    const g = this.ctx, p = this.proj(x, y);
    const fs = size * p.s;
    if (fs < 4) return;
    g.save(); g.translate(p.x, p.y); g.scale(1, .45); if (flip) g.rotate(Math.PI);
    g.fillStyle = color; g.font = `700 ${fs}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.textBaseline = 'middle';
    g.fillText(text, 0, 0); g.restore();
  }

  drawField() {
    const m = this.match;
    const q = (this.cur && this.cur.play.q) || m.quarter || 1;
    this.quad(-12, -5, 132, FIELD_W + 5, '#2f6b33');
    for (let x = 10; x < 110; x += 5) this.quad(x, 0, x + 5, FIELD_W, (x / 5) % 2 ? '#3d8a3f' : '#357d38');
    // Zones de marca amb els colors de qui les defensa.
    const leftTeam = m.teams[attackDir(0, q) > 0 ? 0 : 1];
    const rightTeam = m.teams[attackDir(0, q) > 0 ? 1 : 0];
    this.quad(0, 0, 10, FIELD_W, shade(leftTeam.color, -.15));
    this.quad(110, 0, 120, FIELD_W, shade(rightTeam.color, -.15));
    this.groundText(leftTeam.name.toUpperCase(), 5, C, 3.2, leftTeam.color2 || '#fff');
    this.groundText(rightTeam.name.toUpperCase(), 115, C, 3.2, rightTeam.color2 || '#fff');
    // Línies.
    for (let x = 10; x <= 110; x += 5) this.line(x, 0, x, FIELD_W, 'rgba(255,255,255,.85)', x % 10 === 0 ? 1.4 : 1);
    this.line(0, 0, 120, 0, '#fff', 2); this.line(0, FIELD_W, 120, FIELD_W, '#fff', 2);
    this.line(0, 0, 0, FIELD_W, '#fff', 2); this.line(120, 0, 120, FIELD_W, '#fff', 2);
    for (let x = 11; x < 110; x++) { this.line(x, 22.9, x, 23.6, 'rgba(255,255,255,.7)', .8); this.line(x, 29.7, x, 30.4, 'rgba(255,255,255,.7)', .8); this.line(x, 1, x, 1.6, 'rgba(255,255,255,.7)', .8); this.line(x, FIELD_W - 1.6, x, FIELD_W - 1, 'rgba(255,255,255,.7)', .8); }
    for (let x = 20; x <= 100; x += 10) { const n = String(x <= 60 ? x - 10 : 110 - x); this.groundText(n, x, 9, 2.6, 'rgba(255,255,255,.9)', true); this.groundText(n, x, FIELD_W - 9, 2.6, 'rgba(255,255,255,.9)'); }
    // Logo al mig.
    this.groundText('VENTS', 60, C, 4.2, 'rgba(255,255,255,.18)');
    // Pals.
    this.drawPosts(0); this.drawPosts(120);
    // Línia de scrimmage i del primer down.
    if (this.cur && !['kickoff', 'pat', 'two'].includes(this.cur.play.kind)) {
      const p = this.cur.play, a = this.cur.anim;
      this.line(a.los, 0, a.los, FIELD_W, 'rgba(70,140,255,.95)', 2.2);
      if (p.dist && p.spot + p.dist < 100) { const fx = fieldX(p.spot + p.dist, a.dir); const g = this.ctx; g.save(); g.shadowColor = '#ffd400'; g.shadowBlur = 12; this.line(fx, 0, fx, FIELD_W, '#ffd400', 2.4); g.restore(); }
    }
  }

  drawPosts(x) {
    const g = this.ctx;
    const base = this.proj(x, C), bar = this.proj(x, C, 3.3);
    const l = this.proj(x, C - 3.08, 3.3), r = this.proj(x, C + 3.08, 3.3), lt = this.proj(x, C - 3.08, 11), rt = this.proj(x, C + 3.08, 11);
    g.strokeStyle = '#f4d03f'; g.lineCap = 'round';
    g.lineWidth = .35 * base.s; g.beginPath(); g.moveTo(base.x, base.y); g.lineTo(bar.x, bar.y); g.stroke();
    g.lineWidth = .25 * base.s; g.beginPath(); g.moveTo(l.x, l.y); g.lineTo(r.x, r.y); g.moveTo(l.x, l.y); g.lineTo(lt.x, lt.y); g.moveTo(r.x, r.y); g.lineTo(rt.x, rt.y); g.stroke();
  }

  drawPlay(a, b) {
    const g = this.ctx, t = this.t;
    const items = a.tracks.map(tr => ({ tr, pos: tr.path(t) }));
    items.sort((u, v) => v.pos.y - u.pos.y); // lluny primer
    for (const it of items) this.drawPlayer(it.tr, it.pos, a);
    if (b) {
      const sh = this.proj(b.x, b.y), p = this.proj(b.x, b.y, b.z);
      g.fillStyle = 'rgba(0,0,0,.35)'; g.beginPath(); g.ellipse(sh.x, sh.y, .45 * sh.s, .18 * sh.s, 0, 0, Math.PI * 2); g.fill();
      g.save(); g.translate(p.x, p.y); g.rotate(-.3 + t * 6);
      g.fillStyle = '#7a3e14'; g.beginPath(); g.ellipse(0, 0, .42 * p.s, .26 * p.s, 0, 0, Math.PI * 2); g.fill();
      g.strokeStyle = '#fff'; g.lineWidth = .05 * p.s; g.beginPath(); g.moveTo(-.15 * p.s, 0); g.lineTo(.15 * p.s, 0); g.stroke();
      g.restore();
    }
  }

  drawPlayer(tr, pos, a) {
    const g = this.ctx, team = this.match.teams[tr.team];
    const down = tr.downAt && this.t > tr.downAt;
    const tackled = tr.carrier && this.t > .9 && !this.cur.play.td && this.cur.play.kind !== 'pat';
    const p = this.proj(pos.x, pos.y), s = p.s;
    const h = down || tackled ? .7 : 1.9;
    // Ombra.
    g.fillStyle = 'rgba(0,0,0,.3)'; g.beginPath(); g.ellipse(p.x, p.y, .55 * s, .2 * s, 0, 0, Math.PI * 2); g.fill();
    // Cos.
    const top = this.proj(pos.x, pos.y, h);
    const w = .62 * s;
    const body = g.createLinearGradient(p.x - w, 0, p.x + w, 0);
    body.addColorStop(0, shade(team.color, -.25)); body.addColorStop(.5, team.color); body.addColorStop(1, shade(team.color, -.35));
    g.fillStyle = body;
    g.beginPath(); g.roundRect(p.x - w / 2, top.y, w, p.y - top.y, w * .35); g.fill();
    // Dorsal.
    const pl = tr.p;
    if (s > 7 && h > 1) { g.fillStyle = team.color2 || '#fff'; g.font = `800 ${.38 * s}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText(String(pl.number || ''), p.x, lerp(top.y, p.y, .45)); }
    // Casc.
    const hy = top.y - .18 * s;
    g.fillStyle = luminance(team.color2) > .6 ? shade(team.color2, -.05) : team.color2 || '#ddd';
    g.beginPath(); g.arc(p.x, hy, .27 * s, 0, Math.PI * 2); g.fill();
    g.strokeStyle = team.color; g.lineWidth = .06 * s; g.beginPath(); g.moveTo(p.x - .24 * s, hy); g.lineTo(p.x + .24 * s, hy); g.stroke();
    // Els vents: el seu color al casc i el nom a sobre.
    if (pl.wind) {
      g.fillStyle = pl.color || '#fff'; g.beginPath(); g.arc(p.x, hy - .02 * s, .11 * s, 0, Math.PI * 2); g.fill();
    }
    if ((tr.carrier && this.cur && this.cur.anim.carrierId === pl.id) || (pl.wind && s > 9)) {
      const label = pl.name + (tr.carrier ? '' : '');
      g.font = `700 ${Math.max(10 * this.dpr, .34 * s)}px "Segoe UI", system-ui, sans-serif`;
      const tw = g.measureText(label).width + 8 * this.dpr;
      const ly = hy - .55 * s;
      g.fillStyle = tr.carrier ? 'rgba(0,0,0,.75)' : 'rgba(0,0,0,.45)';
      g.beginPath(); g.roundRect(p.x - tw / 2, ly - 9 * this.dpr, tw, 18 * this.dpr, 9 * this.dpr); g.fill();
      g.fillStyle = tr.carrier ? '#ffd400' : '#fff'; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText(label, p.x, ly);
    }
  }

  drawParticles() {
    const g = this.ctx, dt = 1 / 60;
    this.particles = this.particles.filter(q => (q.life -= dt) > 0);
    for (const q of this.particles) {
      q.vz -= 20 * dt; q.x += q.vx * dt; q.y += q.vy * dt; q.z = Math.max(0, q.z + q.vz * dt);
      const p = this.proj(q.x, clamp(q.y, -3, FIELD_W + 3), q.z);
      g.fillStyle = q.c; g.globalAlpha = Math.min(1, q.life); g.fillRect(p.x, p.y, .25 * p.s, .4 * p.s); g.globalAlpha = 1;
    }
  }

  drawCenterText(text) {
    const g = this.ctx; g.fillStyle = 'rgba(255,255,255,.85)'; g.font = `600 ${16 * this.dpr}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.fillText(text, this.W / 2, this.H / 2);
  }

  drawHUD() {
    const g = this.ctx, W = this.W, d = this.dpr, m = this.match, T = this.T;
    const p = this.cur ? this.cur.play : this.shown();
    const score = this.phase === 'post' || !this.cur ? (p ? p.score : [0, 0]) : (this.queue[this.idx - 2] || { score: [0, 0] }).score;
    // Marcador.
    const bw = Math.min(W - 20 * d, 560 * d), bh = 40 * d, bx = (W - bw) / 2, by = 10 * d;
    g.fillStyle = 'rgba(8,10,20,.82)'; g.beginPath(); g.roundRect(bx, by, bw, bh, 10 * d); g.fill();
    const cell = bw * .3;
    [0, 1].forEach(i => {
      const t = m.teams[i], x = bx + (i === 0 ? 0 : cell + bw * .4);
      g.fillStyle = t.color; g.beginPath(); g.roundRect(x + 4 * d, by + 4 * d, cell - 8 * d, bh - 8 * d, 7 * d); g.fill();
      g.fillStyle = luminance(t.color) > .6 ? '#111' : '#fff'; g.font = `800 ${15 * d}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'left'; g.textBaseline = 'middle';
      g.fillText(t.abbr, x + 12 * d, by + bh / 2);
      g.textAlign = 'right'; g.font = `800 ${20 * d}px "Segoe UI", system-ui, sans-serif`; g.fillText(String(score ? score[i] : 0), x + cell - 12 * d, by + bh / 2);
      if (p && p.off === i && !['pat', 'two', 'kickoff'].includes(p.kind)) { g.fillStyle = '#ffd400'; g.beginPath(); g.arc(x + cell - 46 * d, by + bh / 2, 4 * d, 0, Math.PI * 2); g.fill(); }
    });
    const mid = bx + cell + bw * .2;
    g.fillStyle = '#fff'; g.textAlign = 'center'; g.font = `700 ${12 * d}px "Segoe UI", system-ui, sans-serif`;
    const final = m.status === 'final' && this.idx >= this.queue.length && !this.cur;
    const qText = final ? T.final : p ? `${T.q[Math.min(4, p.q - 1)]} ${Math.floor(p.clock / 60)}:${String(p.clock % 60).padStart(2, '0')}` : T.live;
    g.fillText(qText, mid, by + bh * .36);
    if (p && p.down && !final) { g.fillStyle = '#ffd400'; g.fillText(`${T.down[p.down]} ${T.and} ${p.spot + p.dist >= 100 ? T.goal : p.dist}`, mid, by + bh * .72); }
    // Rètol.
    if (this.banner) {
      const age = (performance.now() - this.banner.t0) / 1000;
      const k = Math.min(1, age * 4);
      const fs = 46 * d * (.6 + .4 * easeOut(k));
      g.save(); g.globalAlpha = Math.min(1, 3 - age);
      g.font = `900 italic ${fs}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.textBaseline = 'middle';
      const tw = g.measureText(this.banner.text).width + 60 * d;
      const y = this.H * .42;
      g.fillStyle = this.banner.color; g.beginPath(); g.moveTo(W / 2 - tw / 2 + 20 * d, y - fs * .7); g.lineTo(W / 2 + tw / 2 + 20 * d, y - fs * .7); g.lineTo(W / 2 + tw / 2 - 20 * d, y + fs * .7); g.lineTo(W / 2 - tw / 2 - 20 * d, y + fs * .7); g.closePath(); g.fill();
      g.fillStyle = this.banner.color2 || '#fff'; g.fillText(this.banner.text, W / 2, y + 2 * d);
      g.restore();
    }
    // Text de la jugada i comentari.
    if (p && (this.phase === 'post' || !this.cur)) {
      const lines = [p.text]; if (p.comment) lines.push('🎙 ' + (m.commentator ? m.commentator + ': ' : '') + p.comment);
      g.font = `600 ${13 * d}px "Segoe UI", system-ui, sans-serif`;
      const maxW = Math.min(W - 24 * d, 760 * d);
      const wrapped = [];
      lines.forEach((l, i) => { const words = l.split(' '); let cur = ''; words.forEach(w => { const test = cur ? cur + ' ' + w : w; if (g.measureText(test).width > maxW - 24 * d) { wrapped.push([cur, i]); cur = w; } else cur = test; }); if (cur) wrapped.push([cur, i]); });
      const lh = 18 * d, hgt = wrapped.length * lh + 14 * d, y0 = this.H - hgt - 12 * d;
      g.fillStyle = 'rgba(8,10,20,.82)'; g.beginPath(); g.roundRect((W - maxW) / 2, y0, maxW, hgt, 10 * d); g.fill();
      g.textAlign = 'left'; g.textBaseline = 'top';
      wrapped.forEach(([l, i], k) => { g.fillStyle = i === 0 ? '#fff' : '#ffd98a'; g.fillText(l, (W - maxW) / 2 + 12 * d, y0 + 7 * d + k * lh); });
    } else if (!this.cur && m.status !== 'final') {
      this.drawCenterText(m.status === 'pregame' ? T.pregame : T.waiting);
    }
  }
}
