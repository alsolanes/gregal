// Visor d'un partit de futbol americà entre els vents (internal/football):
// càmera de televisió des de la banda (perspectiva), camp amb franges, línies,
// números i zones de marca amb els colors de cada equip, jugadors amb el seu
// color, dorsal i casc, la pilota amb l'altura dels xuts i les passades, el
// marcador a dalt i el comentari a baix. Cada jugada es reconstrueix a partir
// del que ha passat (tipus, iardes, qui l'ha fet) amb formacions i rutes.
// A més de la càmera de banda hi ha una «skycam» darrere l'atac, en
// perspectiva de debò, i la realització alterna totes dues com la tele (amb
// repeticions de les jugades grosses i els gols de camp des del xutador).
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
const SKIN = ['#f1c7a5', '#d9a47e', '#b77b52', '#8d5a3b', '#6b4329', '#e8b893'];
function hashStr(v) { let h = 0; for (const c of String(v)) h = (h * 31 + c.charCodeAt(0)) >>> 0; return h; }
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
      const catchX = fieldX(3, dir), retY = C + (R() * 10 - 5);
      const retPath = towards(catchX, C, retEnd, retY, .55, .97, easeOut);
      add(ret, play.off, t => t < .55 ? { x: catchX, y: C } : retPath(t));
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
    const qbPath = towards(px(qb.dx), C, px(qb.dx + 2.5), C + (R() - .5), .1, .45);
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
    const routeIn = towards(px(tgtA.dx), tgtA.y, catchX, catchY, .08, tCatch, t => t);
    const afterCatch = play.complete ? towards(catchX, catchY, endX, catchY + (R() * 6 - 3), tCatch, .92, easeOut) : towards(catchX, catchY, catchX + dir * 2, catchY, tCatch, .8);
    const tgtPath = t => t < tCatch ? routeIn(t) : afterCatch(t);
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
      const jump = towards(dxF(da.dx), da.y, catchX, catchY, .1, tCatch);
      const back = towards(catchX, catchY, catchX - dir * (6 + R() * 10), catchY + (R() * 6 - 3), tCatch, .95, easeOut);
      add(defender, 1 - play.off, t => t < tCatch ? jump(t) : back(t), { carrier: true });
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
    const lag = isTackler ? 0 : 1.5 + R() * 3, side = (a.y < C ? -1 : 1) * (1.2 + R() * 2.5);
    add(a.p, 1 - play.off, t => {
      const b = ball(Math.max(0, t - .05)) || { x: los, y: C };
      const k = seg(t, .15, isTackler ? .9 : 1);
      const tx = b.x - dir * (isTackler ? .6 : lag), ty = b.y + (isTackler ? .4 : side);
      return { x: lerp(dxF(a.dx), tx, isTackler ? ease(k) : k * .85), y: lerp(a.y, ty, isTackler ? ease(k) : k * .8) };
    }, { tackle: isTackler });
  });
  offA.forEach(a => { if (!tracks.find(tr => tr.p.id === a.p.id)) add(a.p, play.off, still(px(a.dx), a.y)); });
  defA.forEach(a => { if (!tracks.find(tr => tr.p.id === a.p.id)) add(a.p, 1 - play.off, still(dxF(a.dx), a.y)); });
  if (kind === 'pass' && (play.detail === 'deep')) dur = 4.4;
  if (play.td) dur += .4;
  return { tracks, ball, dur, carrierId, los, endX, dir };
}


// ---- càmeres ----
// Totes dues càmeres són estenopeiques de debò: una posició P, un punt on
// miren L i una focal. La de televisió és a la banda de baix, quieta d'alçada,
// i només llisca seguint la pilota; la «skycam» va darrere l'atac, enlairada,
// mirant camp avall, i acompanya la jugada.
const NEAR = .35;

// Vista d'una càmera: base (dreta r, amunt u, endavant f) per projectar.
export function makeView(P, L, focal, cx, cy, roll = 0) {
  let fx = L[0] - P[0], fy = L[1] - P[1], fz = L[2] - P[2];
  const n = Math.hypot(fx, fy, fz) || 1; fx /= n; fy /= n; fz /= n;
  const hn = Math.hypot(fx, fy) || 1;
  const r = [-fy / hn, fx / hn, 0];
  const u = [fy * r[2] - fz * r[1], fz * r[0] - fx * r[2], fx * r[1] - fy * r[0]];
  return { P, f: [fx, fy, fz], r, u, focal, cx, cy, roll };
}
// Coordenades de càmera (X a la dreta, Y amunt, Z la profunditat).
export function camSpace(v, x, y, z = 0) {
  const dx = x - v.P[0], dy = y - v.P[1], dz = z - v.P[2];
  return [dx * v.r[0] + dy * v.r[1] + dz * v.r[2], dx * v.u[0] + dy * v.u[1] + dz * v.u[2], dx * v.f[0] + dy * v.f[1] + dz * v.f[2]];
}
// Punt de la pantalla; s són els píxels per iarda a aquella distància i d la
// profunditat (si d < NEAR és darrere la càmera i no s'ha de dibuixar).
export function project(v, x, y, z = 0) {
  const [X, Y, Z] = camSpace(v, x, y, z);
  const s = v.focal / Math.max(Z, 1e-3);
  return { x: v.cx + X * s, y: v.cy - Y * s, s, d: Z };
}
// La càmera de tele de sempre: a 30 iardes de la banda de baix, a l'alçada camH.
export function tvView(camX, focal, camH, W, horizon) {
  return makeView([camX, FIELD_W + 30, camH], [camX, 0, camH], focal, W / 2, horizon);
}

// On vol ser la skycam a l'instant t d'una jugada: darrere la pilota (o
// darrere el xutador en un gol de camp, seguint la pilota cap als pals).
export function skyShot(anim, kind, t) {
  const dir = anim.dir, u = v => v * dir;
  const b = (anim.ball && anim.ball(t)) || { x: anim.los, y: C, z: 1 };
  const bz = Math.min(b.z || 0, 9);
  if (kind === 'fg' || kind === 'pat') {
    const goalX = dir > 0 ? 120 : 0, kickFrom = anim.los - dir * 7;
    const start = u(kickFrom) - 12;
    const camU = Math.min(Math.max(start, u(b.x) - 18), Math.max(start, u(goalX) - 24));
    return { P: [camU * dir, C - 1.6, 7.5 + bz * .2], L: [goalX, C, 2.5 + bz * .2], zoom: 1.2 };
  }
  const kick = kind === 'kickoff' || kind === 'punt';
  const fx = (kick ? u(b.x) : Math.max(u(b.x), u(anim.los) - 4)) * dir;
  const ly = clamp(b.y, 2, FIELD_W - 2);
  return { P: [fx - dir * 17, lerp(C, ly, .55), 8 + bz * .25], L: [fx + dir * 14, lerp(C, ly, .8), bz * .3], zoom: 1 };
}

// Quina càmera fa cada jugada, com una retransmissió: les jugades normals
// alternen tele i skycam; les grosses (touchdown, pèrdua, guany llarg) tenen
// repetició amb l'altra càmera; els gols de camp, des de darrere el xutador.
export function chooseCameras(play, pref = 'auto') {
  if (pref === 'tv' || pref === 'sky') return { live: pref, replay: null, from: 0 };
  const k = play.kind;
  if (k === 'fg' || k === 'pat') return { live: 'sky', replay: null, from: 0 };
  if (k === 'kneel') return { live: 'tv', replay: null, from: 0 };
  const big = !!(play.td || play.int || play.fumble || play.safety || play.hl || Math.abs(play.yards || 0) >= 20);
  if (k === 'kickoff' || k === 'punt') return { live: 'tv', replay: big ? 'sky' : null, from: k === 'kickoff' ? .45 : .2 };
  const live = (play.n || 0) % 2 === 0 ? 'sky' : 'tv';
  return { live, replay: big ? (live === 'sky' ? 'tv' : 'sky') : null, from: k === 'pass' || k === 'two' ? .28 : .12 };
}

// Públic de les graderies de la skycam (sempre el mateix).
function makeCrowd() {
  const R = rng(11), out = [];
  const stands = [
    (a, h) => [lerp(-40, 160, a), -14 - 32 * h, 1.5 + 28 * h],
    (a, h) => [lerp(-40, 160, a), FIELD_W + 14 + 32 * h, 1.5 + 28 * h],
    (a, h) => [-22 - 32 * h, lerp(-14, FIELD_W + 14, a), 1.5 + 28 * h],
    (a, h) => [142 + 32 * h, lerp(-14, FIELD_W + 14, a), 1.5 + 28 * h],
  ];
  // Pocs colors i agrupats: es pinta un camí per color (molt més ràpid).
  const colors = Array.from({ length: 28 }, () => `hsl(${Math.floor(R() * 360)},${20 + R() * 30 | 0}%,${32 + R() * 30 | 0}%)`);
  stands.forEach((f, i) => { const n = i < 2 ? 2600 : 1100; for (let k = 0; k < n; k++) { const [x, y, z] = f(R(), R()); out.push({ x, y, z, ci: Math.floor(R() * colors.length) }); } });
  out.sort((a, b) => a.ci - b.ci);
  out.colors = colors;
  return out;
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
    this.camPref = 'auto'; this.camMode = 'tv'; this.sky = null; this.snap = true; this.fx = null; this.simDt = 0;
    this.crowd = makeCrowd();
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
  // Càmera: 'auto' (alterna com la tele), 'tv' (sempre de banda) o 'sky'.
  setCamera(mode) {
    this.camPref = ['tv', 'sky'].includes(mode) ? mode : 'auto';
    if (this.cur) { this.cur.shots = chooseCameras(this.cur.play, this.camPref); if (this.phase !== 'replay') this.setCam(this.cur.shots.live); }
    else if (this.camPref !== 'auto') this.setCam(this.camPref);
  }

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
    this.view = tvView(this.camX, this.focal, this.camH, this.W, this.horizon);
  }

  proj(x, y, z = 0) { return project(this.view, x, y, z); }

  loop(now) {
    if (this.stopped) return;
    const dt = Math.min(.25, (now - this.last) / 1000); this.last = now;
    this.simDt = this.paused ? 0 : dt * this.speed;
    if (!this.paused) this.advance(this.simDt);
    this.draw();
    requestAnimationFrame(this.loop);
  }

  // Canvi de càmera: tall sec amb un flaix (o la cortina de la repetició).
  setCam(mode, kind = 'cut') {
    const changed = mode !== this.camMode;
    if (changed || kind === 'replay') this.fx = { t0: performance.now(), kind };
    // La tele llisca d'una jugada a l'altra; si canvia de càmera, tall sec.
    if (changed || mode === 'sky' || kind === 'replay') this.snap = true;
    this.camMode = mode;
  }

  advance(dt) {
    if (!this.match) return;
    if (!this.cur) {
      if (this.idx < this.queue.length) {
        const p = this.queue[this.idx++];
        this.cur = { play: p, anim: buildPlay(p, this.match), shots: chooseCameras(p, this.camPref), replayed: false };
        this.t = 0; this.phase = 'pre'; this.hold = .7;
        this.setCam(this.cur.shots.live);
        this.onUpdate(this.match, p);
        this.onEvent(p);
      }
      return;
    }
    const p = this.cur.play;
    const restHold = () => (p.comment ? 3.2 : 1.6) + (p.hl ? 1 : 0);
    if (this.phase === 'pre') { this.hold -= dt; if (this.hold <= 0) this.phase = 'run'; return; }
    if (this.phase === 'run') {
      this.t += dt / this.cur.anim.dur;
      if (this.t >= 1) {
        this.t = 1; this.phase = 'post';
        this.hold = this.cur.shots.replay ? 1.5 : restHold();
        this.celebrate(p);
      }
      return;
    }
    if (this.phase === 'replay') {
      this.t += dt * .7 / this.cur.anim.dur;
      if (this.t >= 1) { this.t = 1; this.phase = 'post'; this.hold = restHold(); }
      return;
    }
    if (this.phase === 'post') {
      this.hold -= dt;
      if (this.hold > 0) return;
      if (this.cur.shots.replay && !this.cur.replayed) {
        this.cur.replayed = true; this.phase = 'replay'; this.t = this.cur.shots.from;
        this.setCam(this.cur.shots.replay, 'replay');
        return;
      }
      this.cur = null; this.banner = null;
    }
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

  isSky() { return this.camMode === 'sky' && !!this.sky; }

  // Mou les càmeres: la de tele llisca amb la pilota; la skycam va cap a on
  // diu skyShot amb una mica de retard, i s'inclina quan es desplaça de costat.
  updateCamera(a, b) {
    const W = this.W, H = this.H, dt = this.simDt;
    const focusX = b ? b.x : (this.cur ? this.cur.anim.los : this.camX);
    if (this.camMode === 'tv' && this.snap) this.camX = clamp(focusX, 14, 106);
    else this.camX = lerp(this.camX, clamp(focusX, 14, 106), .06);
    if (this.camMode === 'sky' && a) {
      const shot = skyShot(a, this.cur.play.kind, this.t);
      if (this.snap || !this.sky) this.sky = { P: shot.P.slice(), L: shot.L.slice(), zoom: shot.zoom, roll: 0 };
      else if (dt > 0) {
        const s = this.sky, kP = 1 - Math.exp(-dt * 3.2), kL = 1 - Math.exp(-dt * 4.5);
        const old = s.P.slice();
        for (let i = 0; i < 3; i++) { s.P[i] = lerp(s.P[i], shot.P[i], kP); s.L[i] = lerp(s.L[i], shot.L[i], kL); }
        s.zoom = lerp(s.zoom, shot.zoom, kP);
        const v = this.view && this.view.r ? this.view : makeView(s.P, s.L, 1, 0, 0);
        const side = ((s.P[0] - old[0]) * v.r[0] + (s.P[1] - old[1]) * v.r[1]) / dt;
        s.roll = lerp(s.roll, clamp(side * .01, -.045, .045), 1 - Math.exp(-dt * 2.5));
      }
    }
    // Partit acabat o esperant jugades: tornem a la tele.
    if (!this.cur && this.idx >= this.queue.length) this.camMode = 'tv';
    this.snap = false;
    if (this.isSky()) {
      const s = this.sky;
      // En vertical (mòbil) el punt de fuga puja perquè es vegi més camp.
      this.view = makeView(s.P, s.L, Math.max(.68 * W, .5 * H) * s.zoom, W / 2, H * (W < H ? .36 : .5), s.roll);
    } else this.view = tvView(this.camX, this.focal, this.camH, W, this.horizon);
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
    this.updateCamera(a, b);
    const v = this.view;
    g.save();
    if (v.roll) { g.translate(v.cx, v.cy); g.rotate(v.roll); g.translate(-v.cx, -v.cy); }
    if (this.isSky()) this.drawSkyStands(); else this.drawStands();
    this.drawField();
    if (a) this.drawPlay(a, b);
    this.drawParticles();
    g.restore();
    if (this.isSky()) {
      const vg = g.createRadialGradient(W / 2, H * .55, Math.min(W, H) * .35, W / 2, H * .55, Math.max(W, H) * .75);
      vg.addColorStop(0, 'rgba(0,0,0,0)'); vg.addColorStop(1, 'rgba(0,0,0,.42)');
      g.fillStyle = vg; g.fillRect(0, 0, W, H);
    }
    this.drawCutFx();
    this.drawHUD();
    this.drawCamTag();
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

  // L'estadi vist des de dins: terra, graderies en talús amb públic, la tanca
  // del camp i les torres de llum.
  drawSkyStands() {
    const g = this.ctx, d = this.dpr, W = this.W, H = this.H;
    const FW = FIELD_W;
    this.poly([[-80, -60, 0], [200, -60, 0], [200, FW + 60, 0], [-80, FW + 60, 0]], '#1d3a22');
    const planes = [
      [[-40, -14, 0], [160, -14, 0], [160, -46, 30], [-40, -46, 30]],
      [[-40, FW + 14, 0], [160, FW + 14, 0], [160, FW + 46, 30], [-40, FW + 46, 30]],
      [[-22, -14, 0], [-22, FW + 14, 0], [-54, FW + 14, 30], [-54, -14, 30]],
      [[142, -14, 0], [142, FW + 14, 0], [174, FW + 14, 30], [174, -14, 30]],
    ].map(q => ({ q, dd: camSpace(this.view, (q[0][0] + q[2][0]) / 2, (q[0][1] + q[2][1]) / 2, 15)[2] }))
      .sort((u, v) => v.dd - u.dd);
    for (const { q } of planes) {
      this.poly(q, '#202842');
      // Grades: franges una mica més clares.
      for (let h = .1; h < 1; h += .15) {
        const at = (k) => [0, 1, 2].map(i => lerp(q[0][i], q[3][i], k)), at2 = (k) => [0, 1, 2].map(i => lerp(q[1][i], q[2][i], k));
        this.poly([at(h), at2(h), at2(h + .035), at(h + .035)], '#2b3558');
      }
    }
    let ci = -1;
    for (const c of this.crowd) {
      const p = this.proj(c.x, c.y, c.z);
      if (p.d < NEAR || p.x < -4 || p.x > W + 4 || p.y < -4 || p.y > H + 4) continue;
      if (c.ci !== ci) { ci = c.ci; g.fillStyle = this.crowd.colors[ci]; }
      const sz = clamp(.26 * p.s, 1 * d, 3.6 * d);
      g.fillRect(p.x - sz / 2, p.y - sz, sz, sz * 1.3);
    }
    // Tanca de publicitat al voltant del camp.
    const wall = [[-14, -6, -14, FW + 6], [134, -6, 134, FW + 6], [-14, -6, 134, -6], [-14, FW + 6, 134, FW + 6]];
    for (const [x0, y0, x1, y1] of wall) {
      this.poly([[x0, y0, 0], [x1, y1, 0], [x1, y1, 1.3], [x0, y0, 1.3]], '#0f1a36');
      this.poly([[x0, y0, 1.05], [x1, y1, 1.05], [x1, y1, 1.3], [x0, y0, 1.3]], '#ffd400');
    }
    // Torres de llum.
    for (const [x, y] of [[-34, -40], [154, -40], [-34, FW + 40], [154, FW + 40], [60, -50], [60, FW + 50]]) {
      const p = this.proj(x, y, 44);
      if (p.d < NEAR) continue;
      const r = clamp(16 * p.s, 20 * d, W * .3);
      const lg = g.createRadialGradient(p.x, p.y, 0, p.x, p.y, r);
      lg.addColorStop(0, 'rgba(255,255,235,.9)'); lg.addColorStop(.08, 'rgba(255,255,235,.45)'); lg.addColorStop(1, 'rgba(255,255,235,0)');
      g.fillStyle = lg; g.fillRect(p.x - r, p.y - r, 2 * r, 2 * r);
    }
  }

  // Polígon 3D retallat pel pla de davant de la càmera (si no, el que queda
  // darrere la skycam es dibuixaria del revés).
  // Amb fill null només afegeix el polígon al camí actual (per pintar-ne molts d'un cop).
  poly(pts, fill) {
    const v = this.view, cam = pts.map(p => camSpace(v, p[0], p[1], p[2] || 0)), out = [];
    for (let i = 0; i < cam.length; i++) {
      const a = cam[i], b = cam[(i + 1) % cam.length], ia = a[2] >= NEAR, ib = b[2] >= NEAR;
      if (ia) out.push(a);
      if (ia !== ib) { const k = (NEAR - a[2]) / (b[2] - a[2]); out.push([lerp(a[0], b[0], k), lerp(a[1], b[1], k), NEAR]); }
    }
    if (out.length < 3) return;
    const g = this.ctx;
    if (fill !== null) g.beginPath();
    out.forEach(([X, Y, Z], i) => { const s = v.focal / Z, x = v.cx + X * s, y = v.cy - Y * s; if (i) g.lineTo(x, y); else g.moveTo(x, y); });
    g.closePath();
    if (fill !== null) { g.fillStyle = fill; g.fill(); }
  }

  quad(x0, y0, x1, y1, fill) { this.poly([[x0, y0], [x1, y0], [x1, y1], [x0, y1]], fill); }

  line(x0, y0, x1, y1, color, w) {
    const g = this.ctx;
    if (this.isSky()) {
      // A la skycam les ratlles són tires de gespa pintada (fan perspectiva).
      const L = Math.hypot(x1 - x0, y1 - y0) || 1, hw = w * .06, nx = -(y1 - y0) / L * hw, ny = (x1 - x0) / L * hw;
      this.poly([[x0 + nx, y0 + ny], [x1 + nx, y1 + ny], [x1 - nx, y1 - ny], [x0 - nx, y0 - ny]], color);
      return;
    }
    const a = this.proj(x0, y0), b = this.proj(x1, y1);
    g.strokeStyle = color; g.lineWidth = w * a.s * .12; g.beginPath(); g.moveTo(a.x, a.y); g.lineTo(b.x, b.y); g.stroke();
  }

  // Text pintat a la gespa. rot és la direcció de lectura al terra (per
  // defecte al llarg del camp; flip el gira mitja volta).
  groundText(text, x, y, size, color, flip, rot) {
    const g = this.ctx;
    if (this.isSky()) {
      const [rx, ry] = rot || (flip ? [-1, 0] : [1, 0]), ux = ry, uy = -rx;
      const o = this.proj(x, y), A = this.proj(x + rx, y + ry), B = this.proj(x - ux, y - uy);
      // Prou lluny perquè tot el text quedi davant la càmera.
      if (o.d < 3 + size * text.length * .35 || size * o.s < 3) return;
      g.save(); g.transform(A.x - o.x, A.y - o.y, B.x - o.x, B.y - o.y, o.x, o.y); g.scale(size / 100, size / 100);
      g.fillStyle = color; g.font = '700 100px "Segoe UI", system-ui, sans-serif'; g.textAlign = 'center'; g.textBaseline = 'middle';
      g.fillText(text, 0, 0); g.restore();
      return;
    }
    const p = this.proj(x, y);
    const fs = size * p.s;
    if (fs < 4) return;
    g.save(); g.translate(p.x, p.y); g.scale(1, .45); if (flip) g.rotate(Math.PI);
    g.fillStyle = color; g.font = `700 ${fs}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.textBaseline = 'middle';
    g.fillText(text, 0, 0); g.restore();
  }

  drawField() {
    const m = this.match, sky = this.isSky();
    const q = (this.cur && this.cur.play.q) || m.quarter || 1;
    this.quad(-12, -5, 132, FIELD_W + 5, '#2f6b33');
    for (let x = 10; x < 110; x += 5) this.quad(x, 0, x + 5, FIELD_W, (x / 5) % 2 ? '#3d8a3f' : '#357d38');
    // Zones de marca amb els colors de qui les defensa.
    const leftTeam = m.teams[attackDir(0, q) > 0 ? 0 : 1];
    const rightTeam = m.teams[attackDir(0, q) > 0 ? 1 : 0];
    this.quad(0, 0, 10, FIELD_W, shade(leftTeam.color, -.15));
    this.quad(110, 0, 120, FIELD_W, shade(rightTeam.color, -.15));
    // A la skycam el nom es llegeix de cara des del camp, d'una banda a l'altra.
    this.groundText(leftTeam.name.toUpperCase(), 5, C, sky ? 4.4 : 3.2, leftTeam.color2 || '#fff', false, [0, -1]);
    this.groundText(rightTeam.name.toUpperCase(), 115, C, sky ? 4.4 : 3.2, rightTeam.color2 || '#fff', false, [0, 1]);
    // Línies.
    for (let x = 10; x <= 110; x += 5) this.line(x, 0, x, FIELD_W, 'rgba(255,255,255,.85)', x % 10 === 0 ? 1.4 : 1);
    this.line(0, 0, 120, 0, '#fff', 2); this.line(0, FIELD_W, 120, FIELD_W, '#fff', 2);
    this.line(0, 0, 0, FIELD_W, '#fff', 2); this.line(120, 0, 120, FIELD_W, '#fff', 2);
    if (sky) {
      // Marques de iarda: totes en un sol camí.
      const g = this.ctx; g.beginPath();
      for (let x = 11; x < 110; x++) { if (x % 5 === 0) continue; for (const [y0, y1] of [[22.9, 23.6], [29.7, 30.4], [1, 1.6], [FIELD_W - 1.6, FIELD_W - 1]]) this.poly([[x - .05, y0], [x + .05, y0], [x + .05, y1], [x - .05, y1]], null); }
      g.fillStyle = 'rgba(255,255,255,.7)'; g.fill();
    } else for (let x = 11; x < 110; x++) { if (x % 5 === 0) continue; this.line(x, 22.9, x, 23.6, 'rgba(255,255,255,.7)', .8); this.line(x, 29.7, x, 30.4, 'rgba(255,255,255,.7)', .8); this.line(x, 1, x, 1.6, 'rgba(255,255,255,.7)', .8); this.line(x, FIELD_W - 1.6, x, FIELD_W - 1, 'rgba(255,255,255,.7)', .8); }
    for (let x = 20; x <= 100; x += 10) { const n = String(x <= 60 ? x - 10 : 110 - x); this.groundText(n, x, 9, 2.6, 'rgba(255,255,255,.9)', true); this.groundText(n, x, FIELD_W - 9, 2.6, 'rgba(255,255,255,.9)'); }
    // Logo al mig.
    this.groundText('VENTS', 60, C, 4.2, 'rgba(255,255,255,.18)');
    // Línia de scrimmage i del primer down.
    if (this.cur && !['kickoff', 'pat', 'two'].includes(this.cur.play.kind)) {
      const p = this.cur.play, a = this.cur.anim;
      this.line(a.los, 0, a.los, FIELD_W, 'rgba(70,140,255,.95)', 2.2);
      if (p.dist && p.spot + p.dist < 100) { const fx = fieldX(p.spot + p.dist, a.dir); const g = this.ctx; g.save(); g.shadowColor = '#ffd400'; g.shadowBlur = 12; this.line(fx, 0, fx, FIELD_W, '#ffd400', 2.4); g.restore(); }
    }
    // Pals (el de lluny primer).
    const posts = [0, 120].sort((u, v) => camSpace(this.view, v, C)[2] - camSpace(this.view, u, C)[2]);
    if (!sky) posts.forEach(x => this.drawPosts(x));
    else this.skyPosts = posts;
  }

  drawPosts(x) {
    const g = this.ctx;
    const base = this.proj(x, C), bar = this.proj(x, C, 3.3);
    const l = this.proj(x, C - 3.08, 3.3), r = this.proj(x, C + 3.08, 3.3), lt = this.proj(x, C - 3.08, 11), rt = this.proj(x, C + 3.08, 11);
    if (Math.min(base.d, l.d, r.d) < NEAR + .5) return;
    g.strokeStyle = '#f4d03f'; g.lineCap = 'round';
    g.lineWidth = .35 * base.s; g.beginPath(); g.moveTo(base.x, base.y); g.lineTo(bar.x, bar.y); g.stroke();
    g.lineWidth = .25 * base.s; g.beginPath(); g.moveTo(l.x, l.y); g.lineTo(r.x, r.y); g.moveTo(l.x, l.y); g.lineTo(lt.x, lt.y); g.moveTo(r.x, r.y); g.lineTo(rt.x, rt.y); g.stroke();
  }

  drawBall(b, t) {
    const g = this.ctx;
    const sh = this.proj(b.x, b.y), p = this.proj(b.x, b.y, b.z);
    if (p.d < NEAR + .5) return;
    const flat = this.isSky() ? clamp(this.view.P[2] / Math.max(1, sh.d), .15, .8) : .4;
    g.fillStyle = 'rgba(0,0,0,.35)'; g.beginPath(); g.ellipse(sh.x, sh.y, .45 * sh.s, .45 * flat * sh.s, 0, 0, Math.PI * 2); g.fill();
    // Mida mínima perquè la pilota es vegi encara que vagi lluny.
    const s = Math.max(p.s, 9 * this.dpr);
    g.save(); g.translate(p.x, p.y); g.rotate(-.3 + t * 6);
    g.fillStyle = '#7a3e14'; g.beginPath(); g.ellipse(0, 0, .42 * s, .26 * s, 0, 0, Math.PI * 2); g.fill();
    g.strokeStyle = '#fff'; g.lineWidth = .05 * s; g.beginPath(); g.moveTo(-.15 * s, 0); g.lineTo(.15 * s, 0); g.stroke();
    g.restore();
  }

  drawPlay(a, b) {
    const t = this.t, sky = this.isSky();
    const items = a.tracks.map((tr, i) => { const pos = tr.path(t); return { tr, i, pos, d: camSpace(this.view, pos.x, pos.y, 1)[2] }; });
    // Lluny primer; en cas d'empat, sempre el mateix ordre, que si no
    // parpellegen quan s'ajunten.
    items.sort((u, v) => (v.d - u.d) || (u.i - v.i));
    // A la skycam la pilota i els pals entren a l'ordre de profunditat.
    const extra = [];
    // A la skycam, la pilota que porta sota el braç un jugador vist de cara o
    // d'esquena ja la dibuixa el ninot.
    let held = false;
    if (sky && b && b.z < 1.4 && t > .2) {
      const c = items.find(it => it.tr.carrier && it.tr.p.id === a.carrierId);
      held = !!c && Math.hypot(c.pos.x - b.x, c.pos.y - b.y) < .5 && this.facing(c.tr, c.pos, a).front !== null;
    }
    if (b && !held) extra.push({ d: camSpace(this.view, b.x, b.y, b.z)[2] + (sky && b.z < 2 ? .3 : 0), draw: () => this.drawBall(b, t) });
    if (sky && this.skyPosts) for (const x of this.skyPosts) extra.push({ d: camSpace(this.view, x, C, 0)[2], draw: () => this.drawPosts(x) });
    if (!sky) {
      for (const it of items) this.drawPlayer(it.tr, it.pos, a);
      extra.forEach(e => e.draw());
      return;
    }
    const all = items.map(it => ({ d: it.d, draw: () => { if (it.d > 3) this.drawPlayer(it.tr, it.pos, a); } })).concat(extra);
    // La pilota per l'aire va per sobre dels que té a prop.
    all.sort((u, v) => v.d - u.d);
    all.forEach(e => e.draw());
  }

  // Cap on mira un jugador respecte de la càmera: face és el costat (±1) i
  // front, a la skycam, si es veu de cara (true), d'esquena (false) o de costat (null).
  facing(tr, pos, a) {
    const ahead = tr.path(Math.min(1, this.t + .015));
    const vx = ahead.x - pos.x, vy = ahead.y - pos.y;
    const moving = (this.phase === 'run' || this.phase === 'replay') && Math.hypot(vx, vy) > .004;
    const homeDir = tr.team === this.cur.play.off ? a.dir : -a.dir;
    if (!this.isSky()) return { face: Math.abs(vx) > .002 ? Math.sign(vx) : homeDir, front: null, moving };
    const sp = Math.hypot(vx, vy), v = this.view;
    const hx = sp > .08 ? vx / sp : homeDir, hy = sp > .08 ? vy / sp : 0;
    const fr = hx * v.r[0] + hy * v.r[1], ff = hx * v.f[0] + hy * v.f[1];
    return { face: fr >= 0 ? 1 : -1, front: Math.abs(fr) < 1.5 * Math.abs(ff) ? ff < 0 : null, moving };
  }

  // Un jugador de futbol americà: cames que corren segons el que es mou,
  // espatlleres, braços, casc amb reixa i el dorsal. A la tele es veu de
  // costat; a la skycam, de costat, d'esquena (dorsal gros) o de cara (reixa).
  drawPlayer(tr, pos, a) {
    const g = this.ctx, team = this.match.teams[tr.team], pl = tr.p;
    const down = tr.downAt && this.t > tr.downAt;
    const tackled = tr.carrier && this.t > .9 && !this.cur.play.td && this.cur.play.kind !== 'pat';
    const p = this.proj(pos.x, pos.y), s = p.s;
    const jersey = team.color, trim = team.color2 || '#fff';
    const pants = luminance(trim) > .55 ? trim : shade(jersey, .7);
    const helmet = shade(jersey, -.08), helmetEdge = shade(jersey, -.3);
    const skin = SKIN[hashStr(pl.id || pl.name || '') % SKIN.length];
    const { face, front, moving } = this.facing(tr, pos, a);
    const sky = this.isSky();
    g.lineCap = 'round'; g.lineJoin = 'round';
    // Ombra (a la skycam el terra s'aixafa segons l'alçada de la càmera).
    const flat = sky ? clamp(this.view.P[2] / Math.max(1, p.d), .15, .45) : .36;
    const shw = (down || tackled ? 1 : .55) * s;
    g.fillStyle = 'rgba(0,0,0,.32)'; g.beginPath(); g.ellipse(p.x, p.y, shw, Math.max(.2 * s * flat / .36, 1), 0, 0, Math.PI * 2); g.fill();
    if (s < 3) { g.fillStyle = jersey; g.fillRect(p.x - s * .3, p.y - s * 1.8, s * .6, s * 1.8); return; }
    if (down || tackled) {
      // Estirat a terra: el cos de costat i el casc al davant.
      g.save(); g.translate(p.x, p.y - .22 * s); g.scale(face, 1);
      g.strokeStyle = pants; g.lineWidth = .24 * s; g.beginPath(); g.moveTo(-.2 * s, 0); g.lineTo(-.95 * s, .05 * s); g.stroke();
      g.fillStyle = jersey; g.beginPath(); g.roundRect(-.3 * s, -.24 * s, .85 * s, .44 * s, .2 * s); g.fill();
      g.fillStyle = helmet; g.beginPath(); g.arc(.72 * s, -.02 * s, .24 * s, 0, Math.PI * 2); g.fill();
      g.restore();
      this.drawLabel(tr, pl, p, s, p.y - .7 * s);
      return;
    }
    const phase = (pos.x * 1.9 + pos.y * 1.3);
    const sw = moving ? Math.sin(phase) : 0, bob = moving ? Math.abs(Math.cos(phase)) * .06 * s : 0;
    const hipY = p.y - .9 * s - bob, shY = p.y - 1.55 * s - bob, headY = p.y - 1.86 * s - bob;
    const carrying = tr.carrier && this.cur && this.cur.anim.carrierId === pl.id && this.t > .2;
    if (front !== null) {
      this.drawPlayerFacing(p, s, { front, face, sw, moving, hipY, shY, headY, jersey, trim, pants, helmet, helmetEdge, skin, carrying, pl });
      this.drawLabel(tr, pl, p, s, headY - .5 * s);
      return;
    }
    const lean = moving ? face * .1 * s : 0;
    // Cames: cuixa amb el pantaló, mitjó del color de l'equip, bota.
    for (const k of [-1, 1]) {
      const st = sw * k;
      const hip = { x: p.x + k * .1 * s, y: hipY };
      const foot = { x: p.x + k * .1 * s + face * st * .36 * s, y: p.y - Math.max(0, -st) * .14 * s };
      const knee = { x: lerp(hip.x, foot.x, .5) + face * (.08 + Math.max(0, st) * .1) * s, y: lerp(hip.y, foot.y, .5) };
      g.strokeStyle = pants; g.lineWidth = .2 * s; g.beginPath(); g.moveTo(hip.x, hip.y); g.lineTo(knee.x, knee.y); g.stroke();
      g.strokeStyle = shade(jersey, -.2); g.lineWidth = .15 * s; g.beginPath(); g.moveTo(knee.x, knee.y); g.lineTo(foot.x, foot.y); g.stroke();
      g.fillStyle = '#16161a'; g.beginPath(); g.ellipse(foot.x + face * .06 * s, foot.y, .13 * s, .06 * s, 0, 0, Math.PI * 2); g.fill();
    }
    // Braç del darrere.
    const arm = (k, carry) => {
      const sh = { x: p.x + lean + k * .32 * s, y: shY + .1 * s };
      const hand = carry ? { x: p.x + lean + face * .28 * s, y: shY + .38 * s } : { x: sh.x - face * sw * k * .3 * s, y: shY + .6 * s };
      const elbow = carry ? { x: sh.x + face * .05 * s, y: shY + .42 * s } : { x: lerp(sh.x, hand.x, .5) - face * .05 * s, y: lerp(sh.y, hand.y, .5) };
      g.strokeStyle = shade(jersey, -.1); g.lineWidth = .16 * s; g.beginPath(); g.moveTo(sh.x, sh.y); g.lineTo(elbow.x, elbow.y); g.stroke();
      g.strokeStyle = skin; g.lineWidth = .12 * s; g.beginPath(); g.moveTo(elbow.x, elbow.y); g.lineTo(hand.x, hand.y); g.stroke();
    };
    arm(-face, false);
    // Tors: samarreta amb espatlleres, més ample a dalt.
    const tw = .82 * s, ww = .5 * s;
    const body = g.createLinearGradient(p.x - tw / 2, 0, p.x + tw / 2, 0);
    body.addColorStop(0, shade(jersey, -.3)); body.addColorStop(.45, jersey); body.addColorStop(1, shade(jersey, -.4));
    g.fillStyle = body; g.strokeStyle = shade(jersey, -.5); g.lineWidth = .03 * s;
    g.beginPath();
    g.moveTo(p.x + lean - tw / 2, shY); g.lineTo(p.x + lean + tw / 2, shY);
    g.lineTo(p.x + ww / 2, hipY + .05 * s); g.lineTo(p.x - ww / 2, hipY + .05 * s); g.closePath(); g.fill(); g.stroke();
    // Espatlleres.
    g.fillStyle = shade(jersey, .12); g.beginPath(); g.ellipse(p.x + lean, shY + .02 * s, tw / 2 + .04 * s, .13 * s, 0, 0, Math.PI * 2); g.fill();
    // Ratlla de la màniga i dorsal.
    g.fillStyle = trim; g.fillRect(p.x + lean - tw / 2 - .02 * s, shY + .05 * s, .1 * s, .06 * s); g.fillRect(p.x + lean + tw / 2 - .08 * s, shY + .05 * s, .1 * s, .06 * s);
    if (s > 7) {
      g.font = `900 ${.36 * s}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.textBaseline = 'middle';
      g.lineWidth = .06 * s; g.strokeStyle = shade(jersey, -.55); g.strokeText(String(pl.number || ''), p.x + lean * .6, lerp(shY, hipY, .5));
      g.fillStyle = trim; g.fillText(String(pl.number || ''), p.x + lean * .6, lerp(shY, hipY, .5));
    }
    arm(face, carrying);
    // Casc: closca, ratlla, reixa al costat on mira, i el color del vent.
    const hx = p.x + lean * 1.2;
    g.fillStyle = helmet; g.strokeStyle = helmetEdge; g.lineWidth = .03 * s; g.beginPath(); g.ellipse(hx, headY, .26 * s, .25 * s, 0, 0, Math.PI * 2); g.fill(); g.stroke();
    const shine = g.createRadialGradient(hx - .08 * s, headY - .1 * s, 0, hx, headY, .28 * s);
    shine.addColorStop(0, 'rgba(255,255,255,.55)'); shine.addColorStop(.4, 'rgba(255,255,255,0)');
    g.fillStyle = shine; g.beginPath(); g.ellipse(hx, headY, .26 * s, .25 * s, 0, 0, Math.PI * 2); g.fill();
    g.strokeStyle = trim; g.lineWidth = .07 * s; g.beginPath(); g.ellipse(hx, headY, .25 * s, .24 * s, 0, Math.PI * 1.2, Math.PI * 1.8); g.stroke();
    g.strokeStyle = '#c9ccd2'; g.lineWidth = .04 * s; g.beginPath();
    g.moveTo(hx + face * .14 * s, headY - .02 * s); g.lineTo(hx + face * .3 * s, headY - .02 * s);
    g.moveTo(hx + face * .14 * s, headY + .09 * s); g.lineTo(hx + face * .29 * s, headY + .09 * s);
    g.moveTo(hx + face * .3 * s, headY - .06 * s); g.lineTo(hx + face * .27 * s, headY + .16 * s); g.stroke();
    if (pl.wind) { g.fillStyle = pl.color || '#fff'; g.beginPath(); g.arc(hx - face * .08 * s, headY - .06 * s, .08 * s, 0, Math.PI * 2); g.fill(); }
    this.drawLabel(tr, pl, p, s, headY - .5 * s);
  }

  // El mateix ninot vist de cara (reixa del casc, dorsal al pit) o d'esquena
  // (dorsal gros a l'esquena, ratlla del casc pel mig).
  drawPlayerFacing(p, s, o) {
    const g = this.ctx, { front, sw, moving, hipY, shY, headY, jersey, trim, pants, helmet, helmetEdge, skin, carrying, pl } = o;
    // Cames: la que puja s'escurça (el peu s'aixeca cap a la càmera o lluny).
    for (const k of [-1, 1]) {
      const lift = moving ? Math.max(0, sw * k) : 0;
      const hip = { x: p.x + k * .13 * s, y: hipY };
      const foot = { x: p.x + k * .17 * s, y: p.y - lift * .2 * s };
      const knee = { x: p.x + k * .17 * s, y: lerp(hipY, foot.y, .48) - lift * .1 * s };
      g.strokeStyle = pants; g.lineWidth = .22 * s; g.beginPath(); g.moveTo(hip.x, hip.y); g.lineTo(knee.x, knee.y); g.stroke();
      g.strokeStyle = shade(jersey, -.2); g.lineWidth = .16 * s; g.beginPath(); g.moveTo(knee.x, knee.y); g.lineTo(foot.x, foot.y); g.stroke();
      g.fillStyle = lift > .3 && !front ? '#3a3a40' : '#16161a'; g.beginPath(); g.ellipse(foot.x, foot.y, .1 * s, .07 * s, 0, 0, Math.PI * 2); g.fill();
    }
    const tw = 1.0 * s, ww = .58 * s;
    // Braços a banda i banda (el que porta la pilota, plegat).
    const arm = (k, carry) => {
      const sh = { x: p.x + k * (tw / 2 - .04 * s), y: shY + .1 * s };
      const swing = moving ? sw * k : 0;
      const hand = carry ? { x: p.x + k * .2 * s, y: shY + .45 * s } : { x: p.x + k * (.5 + Math.max(0, swing) * .06) * s, y: shY + (.62 - Math.max(0, swing) * .2) * s };
      const elbow = carry ? { x: p.x + k * .52 * s, y: shY + .42 * s } : { x: p.x + k * .55 * s, y: lerp(sh.y, hand.y, .5) };
      g.strokeStyle = shade(jersey, -.15); g.lineWidth = .17 * s; g.beginPath(); g.moveTo(sh.x, sh.y); g.lineTo(elbow.x, elbow.y); g.stroke();
      g.strokeStyle = skin; g.lineWidth = .12 * s; g.beginPath(); g.moveTo(elbow.x, elbow.y); g.lineTo(hand.x, hand.y); g.stroke();
    };
    // Tors.
    const body = g.createLinearGradient(p.x - tw / 2, 0, p.x + tw / 2, 0);
    body.addColorStop(0, shade(jersey, -.35)); body.addColorStop(.5, jersey); body.addColorStop(1, shade(jersey, -.35));
    g.fillStyle = body; g.strokeStyle = shade(jersey, -.5); g.lineWidth = .03 * s;
    g.beginPath(); g.moveTo(p.x - tw / 2, shY); g.lineTo(p.x + tw / 2, shY); g.lineTo(p.x + ww / 2, hipY + .05 * s); g.lineTo(p.x - ww / 2, hipY + .05 * s); g.closePath(); g.fill(); g.stroke();
    g.fillStyle = shade(jersey, .12); g.beginPath(); g.ellipse(p.x, shY + .02 * s, tw / 2 + .05 * s, .14 * s, 0, 0, Math.PI * 2); g.fill();
    g.fillStyle = trim; g.fillRect(p.x - tw / 2 - .04 * s, shY + .02 * s, .12 * s, .07 * s); g.fillRect(p.x + tw / 2 - .08 * s, shY + .02 * s, .12 * s, .07 * s);
    if (s > 6) {
      const big = !front, ny = lerp(shY, hipY, big ? .52 : .45);
      g.font = `900 ${(big ? .46 : .3) * s}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.textBaseline = 'middle';
      g.lineWidth = .06 * s; g.strokeStyle = shade(jersey, -.55); g.strokeText(String(pl.number || ''), p.x, ny);
      g.fillStyle = trim; g.fillText(String(pl.number || ''), p.x, ny);
      // D'esquena, el nom a sobre del dorsal.
      if (big && s > 22) { g.font = `800 ${.12 * s}px "Segoe UI", system-ui, sans-serif`; g.fillText(String(pl.name || '').toUpperCase().slice(0, 12), p.x, shY + .2 * s); }
    }
    if (carrying) {
      // La pilota sota el braç.
      g.fillStyle = '#7a3e14'; g.beginPath(); g.ellipse(p.x + .22 * s, shY + .42 * s, .2 * s, .13 * s, -.4, 0, Math.PI * 2); g.fill();
    }
    arm(-1, false); arm(1, carrying);
    // Coll i casc.
    g.fillStyle = front ? skin : shade(skin, -.15); g.fillRect(p.x - .09 * s, headY + .12 * s, .18 * s, .16 * s);
    g.fillStyle = helmet; g.strokeStyle = helmetEdge; g.lineWidth = .03 * s; g.beginPath(); g.ellipse(p.x, headY, .27 * s, .26 * s, 0, 0, Math.PI * 2); g.fill(); g.stroke();
    const shine = g.createRadialGradient(p.x - .09 * s, headY - .11 * s, 0, p.x, headY, .3 * s);
    shine.addColorStop(0, 'rgba(255,255,255,.55)'); shine.addColorStop(.4, 'rgba(255,255,255,0)');
    g.fillStyle = shine; g.beginPath(); g.ellipse(p.x, headY, .27 * s, .26 * s, 0, 0, Math.PI * 2); g.fill();
    if (front) {
      // Obertura de la cara i reixa.
      g.fillStyle = '#1a1310'; g.beginPath(); g.ellipse(p.x, headY + .07 * s, .17 * s, .13 * s, 0, 0, Math.PI * 2); g.fill();
      g.fillStyle = skin; g.beginPath(); g.ellipse(p.x, headY + .09 * s, .1 * s, .08 * s, 0, 0, Math.PI * 2); g.fill();
      g.strokeStyle = '#c9ccd2'; g.lineWidth = .035 * s; g.beginPath();
      g.moveTo(p.x - .19 * s, headY + .03 * s); g.lineTo(p.x + .19 * s, headY + .03 * s);
      g.moveTo(p.x - .16 * s, headY + .13 * s); g.lineTo(p.x + .16 * s, headY + .13 * s);
      g.moveTo(p.x, headY + .03 * s); g.lineTo(p.x, headY + .2 * s);
      g.moveTo(p.x - .19 * s, headY + .03 * s); g.lineTo(p.x - .12 * s, headY + .19 * s);
      g.moveTo(p.x + .19 * s, headY + .03 * s); g.lineTo(p.x + .12 * s, headY + .19 * s); g.stroke();
      g.strokeStyle = trim; g.lineWidth = .06 * s; g.beginPath(); g.moveTo(p.x, headY - .25 * s); g.lineTo(p.x, headY - .1 * s); g.stroke();
    } else {
      g.strokeStyle = trim; g.lineWidth = .07 * s; g.beginPath(); g.moveTo(p.x, headY - .25 * s); g.lineTo(p.x, headY + .2 * s); g.stroke();
    }
    if (pl.wind) { g.fillStyle = pl.color || '#fff'; g.beginPath(); g.arc(p.x + .17 * s, headY - .04 * s, .07 * s, 0, Math.PI * 2); g.fill(); }
  }

  drawLabel(tr, pl, p, s, ly) {
    const g = this.ctx;
    const isCarrier = tr.carrier && this.cur && this.cur.anim.carrierId === pl.id;
    if (!isCarrier && !(pl.wind && s > 9)) return;
    g.font = `700 ${Math.max(10 * this.dpr, Math.min(.34 * s, 16 * this.dpr))}px "Segoe UI", system-ui, sans-serif`;
    const tw = g.measureText(pl.name).width + 8 * this.dpr;
    g.fillStyle = isCarrier ? 'rgba(0,0,0,.75)' : 'rgba(0,0,0,.45)';
    g.beginPath(); g.roundRect(p.x - tw / 2, ly - 9 * this.dpr, tw, 18 * this.dpr, 9 * this.dpr); g.fill();
    g.fillStyle = isCarrier ? '#ffd400' : '#fff'; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText(pl.name, p.x, ly);
  }

  drawParticles() {
    const g = this.ctx, dt = 1 / 60;
    this.particles = this.particles.filter(q => (q.life -= dt) > 0);
    for (const q of this.particles) {
      q.vz -= 20 * dt; q.x += q.vx * dt; q.y += q.vy * dt; q.z = Math.max(0, q.z + q.vz * dt);
      const p = this.proj(q.x, clamp(q.y, -3, FIELD_W + 3), q.z);
      if (p.d < NEAR + .5) continue;
      g.fillStyle = q.c; g.globalAlpha = Math.min(1, q.life); g.fillRect(p.x, p.y, .25 * p.s, .4 * p.s); g.globalAlpha = 1;
    }
  }

  // Tall de càmera: flaix blanc; repetició: cortina en diagonal amb «REPLAY».
  drawCutFx() {
    if (!this.fx) return;
    const g = this.ctx, W = this.W, H = this.H, d = this.dpr;
    const age = (performance.now() - this.fx.t0) / 1000;
    if (this.fx.kind === 'replay') {
      const k = age / .6;
      if (k >= 1) { this.fx = null; return; }
      const team = this.cur ? this.match.teams[this.cur.play.off] : null;
      const cx = lerp(-.4 * W, 1.4 * W, ease(k)), bw = W * .45, sl = H * .35;
      g.save();
      g.fillStyle = team ? team.color : '#1E5BD8';
      g.beginPath(); g.moveTo(cx - bw / 2 + sl, 0); g.lineTo(cx + bw / 2 + sl, 0); g.lineTo(cx + bw / 2 - sl, H); g.lineTo(cx - bw / 2 - sl, H); g.closePath(); g.fill();
      g.fillStyle = 'rgba(255,255,255,.9)';
      g.beginPath(); g.moveTo(cx + bw / 2 + sl, 0); g.lineTo(cx + bw / 2 + sl + 10 * d, 0); g.lineTo(cx + bw / 2 - sl + 10 * d, H); g.lineTo(cx + bw / 2 - sl, H); g.closePath(); g.fill();
      g.fillStyle = (team && team.color2) || '#fff'; g.font = `900 italic ${Math.min(64 * d, W * .08)}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.textBaseline = 'middle';
      g.fillText('REPLAY', cx, H / 2);
      g.restore();
      return;
    }
    const k = age / .28;
    if (k >= 1) { this.fx = null; return; }
    g.fillStyle = `rgba(255,255,255,${(.75 * (1 - k)).toFixed(3)})`; g.fillRect(0, 0, W, H);
  }

  // Etiqueta de la càmera a la cantonada (SKYCAM / REPLAY).
  drawCamTag() {
    const replay = this.phase === 'replay';
    if (!replay && !this.isSky()) return;
    const g = this.ctx, d = this.dpr;
    const text = replay ? (this.isSky() ? 'REPLAY · SKYCAM' : 'REPLAY') : 'SKYCAM';
    g.font = `800 ${11 * d}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'left'; g.textBaseline = 'middle';
    const w = g.measureText(text).width + 30 * d, x = 12 * d, y = 58 * d, h = 22 * d;
    g.fillStyle = 'rgba(8,10,20,.78)'; g.beginPath(); g.roundRect(x, y, w, h, 6 * d); g.fill();
    const on = !replay || (performance.now() / 500 | 0) % 2 === 0;
    g.fillStyle = replay ? (on ? '#ff3b3b' : 'rgba(255,59,59,.3)') : '#ffd400'; g.beginPath(); g.arc(x + 11 * d, y + h / 2, 4 * d, 0, Math.PI * 2); g.fill();
    g.fillStyle = '#fff'; g.fillText(text, x + 20 * d, y + h / 2 + .5 * d);
  }

  drawCenterText(text) {
    const g = this.ctx; g.fillStyle = 'rgba(255,255,255,.85)'; g.font = `600 ${16 * this.dpr}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.fillText(text, this.W / 2, this.H / 2);
  }

  drawHUD() {
    const g = this.ctx, W = this.W, d = this.dpr, m = this.match, T = this.T;
    const p = this.cur ? this.cur.play : this.shown();
    const after = this.phase === 'post' || this.phase === 'replay' || !this.cur;
    const score = after ? (p ? p.score : [0, 0]) : (this.queue[this.idx - 2] || { score: [0, 0] }).score;
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
      g.save(); g.globalAlpha = Math.max(0, Math.min(1, 3 - age));
      g.font = `900 italic ${fs}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.textBaseline = 'middle';
      const tw = g.measureText(this.banner.text).width + 60 * d;
      const y = this.H * .42;
      g.fillStyle = this.banner.color; g.beginPath(); g.moveTo(W / 2 - tw / 2 + 20 * d, y - fs * .7); g.lineTo(W / 2 + tw / 2 + 20 * d, y - fs * .7); g.lineTo(W / 2 + tw / 2 - 20 * d, y + fs * .7); g.lineTo(W / 2 - tw / 2 - 20 * d, y + fs * .7); g.closePath(); g.fill();
      g.fillStyle = this.banner.color2 || '#fff'; g.fillText(this.banner.text, W / 2, y + 2 * d);
      g.restore();
    }
    // Text de la jugada i comentari (no tapa la repetició ni el moment d'abans).
    const replayPending = this.cur && this.cur.shots.replay && !this.cur.replayed;
    if (p && ((this.phase === 'post' && !replayPending) || !this.cur)) {
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
