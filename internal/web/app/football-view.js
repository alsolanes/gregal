// Visor d'un partit de futbol americà entre els vents (internal/football):
// càmera de televisió des de la banda (perspectiva), camp amb franges, línies,
// números i zones de marca amb els colors de cada equip, jugadors amb el seu
// color, dorsal i casc, la pilota amb l'altura dels xuts i les passades, el
// marcador a dalt i el comentari a baix. Cada jugada es reconstrueix a partir
// del que ha passat (tipus, iardes, qui l'ha fet) amb formacions i rutes.
// A més de la càmera de banda hi ha càmeres 3D en perspectiva de debò: la de
// jugador (per sobre l'espatlla de qui porta la pilota), la skycam, la de la
// zona de marca, la del pal, la de carril i el dirigible. Un realitzador les
// tria per a cada jugada i fa repeticions a càmera lenta de les grosses. Tot
// passa dins d'un estadi amb graderies, públic animat i videomarcadors.
// La pilota té física (vol amb gir, bots irregulars, rodolar) i passa de mà en
// mà; els placatges tenen contacte, aturada curta, caiguda conjunta i terra.
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

// Les marques de la barra de progrés: jugades grosses (hl) i les que fan pujar
// el marcador. [{ i, kind: 'score'|'hl', text }], desades mentre la llista no creix.
const MARKS = new WeakMap();
export function progressMarks(list) {
  const c = MARKS.get(list);
  if (c && c.len === list.length) return c.marks;
  const marks = []; let prev = [0, 0];
  list.forEach((e, i) => {
    const sc = Array.isArray(e.score) ? e.score : null;
    const up = sc && (sc[0] > prev[0] || sc[1] > prev[1]);
    if (sc) prev = sc;
    if (up || e.hl) marks.push({ i, kind: up ? 'score' : 'hl', text: e.text || '' });
  });
  MARKS.set(list, { len: list.length, marks });
  return marks;
}

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

// buildRaw converteix una jugada en pistes d'animació: cada jugador té una
// funció de posició al llarg de t (0..1) i la pilota té x, y, z (altura).
function buildRaw(play, match) {
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
  // Moments i protagonistes de la jugada (per a les càmeres): qui la llança,
  // a qui va, quan surt i quan arriba la pilota.
  const marks = {};

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
      marks.returnerId = ret.id; marks.tCatch = .55;
      dur = 5.2;
    }
    return { tracks, ball, dur, carrierId, los, endX, dir, marks };
  }

  const offA = assign(off, offenseSet(kind, play.detail));
  const defA = assign(def, defenseSet());
  const qb = offA.find(a => a.p.pos === 'QB');
  marks.qbId = qb.p.id;
  const px = dx => los - dir * Math.abs(dx); // darrere la línia
  const dxF = dx => los + dir * dx; // davant

  if (kind === 'punt' || kind === 'fg' || kind === 'pat') {
    const k = off.players.find(p => p.id === A.kicker) || off.players.find(p => p.pos === 'K');
    const kickFrom = kind === 'punt' ? px(13) : px(7);
    marks.kickerId = k && k.id;
    offA.forEach(a => { if (a.p.pos === 'OL' || a.p.pos === 'TE') add(a.p, play.off, still(los - dir * 0.6, a.y)); else if (a.p.pos !== 'QB') add(a.p, play.off, towards(px(2), a.y, kind === 'punt' ? dxF(25) : px(2), a.y, .3, .95)); });
    if (kind !== 'punt') add(qb.p, play.off, still(px(7) + dir * .1, C + 1.2)); // el que aguanta
    add(k, play.off, towards(kickFrom - dir * 2, C - .6, kickFrom, C - .4, .1, .3));
    defA.forEach(a => add(a.p, 1 - play.off, towards(dxF(a.dx), a.y, dxF(a.dx * 0.4), lerp(a.y, C, .4), .2, .6)));
    if (kind === 'punt') {
      const land = endX;
      ball = t => { if (t < .12) return { x: lerp(los, kickFrom, t / .12), y: C, z: 1 }; const k2 = seg(t, .25, .8); return { x: lerp(kickFrom, land, k2), y: C + Math.sin(k2 * 3), z: 1 + 11 * Math.sin(Math.PI * k2) }; };
      const ret = def.players.find(p => p.id === A.returner);
      if (ret) { add(ret, 1 - play.off, still(land, C)); marks.returnerId = ret.id; }
      dur = 4.6;
    } else {
      const goalX = dir > 0 ? 120 : 0;
      const good = !!play.good;
      const wide = (R() < .5 ? -1 : 1) * 5;
      ball = t => { if (t < .2) return { x: lerp(los, kickFrom, t / .2), y: C, z: 1 }; const k2 = seg(t, .3, .85); return { x: lerp(kickFrom, goalX + dir * 3, k2), y: C + (good ? 0 : wide * k2), z: 1 + 3.5 * k2 + 4 * Math.sin(Math.PI * k2 * .8) }; };
      dur = 3.6;
    }
    return { tracks, ball, dur, carrierId, los, endX, dir, marks, goalPost: kind !== 'punt' };
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
      return { tracks, ball, dur: 1.8, carrierId: qb.p.id, los, endX, dir, marks };
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
    if (carrier !== qb) marks.tHand = .2;
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
    Object.assign(marks, { targetId: tgtA.p.id, tThrow, tCatch, deep: play.detail === 'deep' || depth >= 15 });
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
  return { tracks, ball, dur, carrierId, los, endX, dir, marks };
}



// buildPlay: les trajectòries de buildRaw més la física de la pilota, qui la
// porta i com, els contactes (placatges, línia, stiff-arm) i la separació.
export function buildPlay(play, match) { return enrich(buildRaw(play, match), play); }

// ---- física de la pilota, agafades i placatges ----
// La pilota passa de mà en mà (centre, quarterback, corredor o receptor), vola
// girant (en espiral a les passades i els punts, de punta a punta als xuts) i,
// quan cau, bota de manera irregular, rodola i s'atura. Els placatges tenen
// aproximació, contacte, una aturada curta, caiguda conjunta i terra; els de
// línia s'empenyen; ningú no travessa ningú. Tot surt de la llavor de la
// jugada: el mateix t dona sempre el mateix.
// ARM_REACH: fins on arriba una mà des de l'espatlla (iardes); la pilota a la mà mai queda més lluny.
export const ARM_REACH = 1.15;
const GRAV = 10.7, SEP = .62, BALL_R = .1;
const unit2 = (x, y) => { const n = Math.hypot(x, y); return n > 1e-9 ? [x / n, y / n] : [1, 0]; };
const unit3 = v => { const n = Math.hypot(v[0], v[1], v[2]); return n > 1e-9 ? [v[0] / n, v[1] / n, v[2] / n] : [1, 0, 0]; };

// Agafades: [endavant, costat (+ = dreta), alçada, mans] respecte del jugador.
const GRIPS = {
  snap: [.35, 0, .14, 2], carry: [.3, .05, 1.3, 2], cock: [-.22, .4, 1.95, 1], tuck: [.14, .34, 1.12, 1],
  field: [.42, 0, 1.3, 2], high: [.28, 0, 2.45, 2], low: [.7, 0, .7, 2], over: [.22, 0, 1.7, 2],
  ground: [.42, .25, .17, 1], clutch: [.3, 0, 1.08, 2], kneel: [.32, 0, .75, 2], hand: [.48, .12, 1.15, 2],
};

// Pilota solta: vol, bots irregulars que s'apaguen, rodolar i quedar quieta.
// p0, v0 en iardes i iardes/segon; el temps, en segons des que s'ha deixat anar.
export function looseBall(p0, v0, seed) {
  const R = rng(seed), segs = [];
  let p = [p0[0], p0[1], Math.max(BALL_R, p0[2])], v = v0.slice(), tau = 0, tumble = 8 + R() * 8, yaw = Math.atan2(v[1], v[0]), ph = R() * 6;
  for (let k = 0; k < 6; k++) {
    const z = p[2] - BALL_R, tf = Math.max(.001, (v[2] + Math.sqrt(v[2] * v[2] + 2 * GRAV * Math.max(0, z))) / GRAV);
    segs.push({ kind: 'air', tau, p: p.slice(), v: v.slice(), dur: tf, tumble, yaw, ph });
    p = [p[0] + v[0] * tf, p[1] + v[1] * tf, BALL_R]; tau += tf;
    const vzIn = Math.abs(v[2] - GRAV * tf);
    // La punta toca primer: el bot surt cap a un costat i amb un gir nou.
    const e = .38 + R() * .27, keep = .5 + R() * .35, turn = (R() - .5) * 1.5, c = Math.cos(turn), s = Math.sin(turn);
    v = [(v[0] * c - v[1] * s) * keep, (v[0] * s + v[1] * c) * keep, vzIn * e];
    tumble = (R() < .5 ? -1 : 1) * (10 + R() * 22); yaw = Math.atan2(v[1], v[0]); ph = R() * 6;
    if (v[2] < 1.1) break;
  }
  const sp = Math.hypot(v[0], v[1]), fr = 6, rd = Math.max(.05, sp / fr);
  segs.push({ kind: 'roll', tau, p: p.slice(), v: [v[0], v[1], 0], dur: rd, yaw: Math.atan2(v[1], v[0]) });
  tau += rd;
  segs.push({ kind: 'rest', tau, p: [p[0] + v[0] * rd / 2, p[1] + v[1] * rd / 2, BALL_R], dur: 1e9, yaw: Math.atan2(v[1], v[0]) + Math.PI / 2 + (R() - .5) });
  return segs;
}
// Altura màxima de cada vol (per comprovar que els bots s'apaguen).
export function bounceApexes(segs) { return segs.filter(s => s.kind === 'air').map(s => s.p[2] + Math.max(0, s.v[2]) ** 2 / (2 * GRAV)); }
export function looseAt(segs, tau) {
  let s = segs[segs.length - 1];
  for (const q of segs) if (tau < q.tau + q.dur) { s = q; break; }
  const dt = clamp(tau - s.tau, 0, s.dur);
  if (s.kind === 'air') {
    const phi = s.ph + s.tumble * dt, h = [Math.cos(s.yaw), Math.sin(s.yaw)];
    return { x: s.p[0] + s.v[0] * dt, y: s.p[1] + s.v[1] * dt, z: Math.max(BALL_R, s.p[2] + s.v[2] * dt - GRAV * dt * dt / 2), axis: [h[0] * Math.cos(phi), h[1] * Math.cos(phi), Math.sin(phi)], spin: phi };
  }
  if (s.kind === 'roll') {
    const f = dt - dt * dt / (2 * s.dur), wob = s.yaw + Math.PI / 2 + .35 * Math.sin(dt * 9);
    return { x: s.p[0] + s.v[0] * f, y: s.p[1] + s.v[1] * f, z: BALL_R, axis: [Math.cos(wob), Math.sin(wob), .08], spin: f * 14 };
  }
  return { x: s.p[0], y: s.p[1], z: BALL_R, axis: [Math.cos(s.yaw), Math.sin(s.yaw), .05], spin: 0 };
}

// Retalla t d'una jugada perquè el placatge faci una aturada curta (hit-stop)
// sense allargar la jugada: es para un moment i després recupera.
export function hitStop(anim, t) {
  const h = .02;
  for (const c of anim.contacts || []) {
    const u = t - c.tc;
    if (u > 0 && u < 3 * h) return c.tc + (u < h ? u * .15 : .15 * h + (u - h) * (2.85 * h) / (2 * h));
  }
  return t;
}

function enrich(anim, play) {
  anim.rawBall = anim.ball;
  const { tracks, dir, dur } = anim, m = anim.marks, A = play.actors || {}, kind = play.kind, N = tracks.length;
  const R = rng((play.n || 1) * 7919 + 3);
  const idx = id => id ? tracks.findIndex(tr => tr.p.id === id) : -1;
  const raw = (i, t) => tracks[i].path(t);
  const teamDir = i => tracks[i].team === play.off ? dir : -dir;
  const scrim = ['run', 'pass', 'sack', 'scramble', 'kneel', 'two'].includes(kind);
  const qbI = idx(m.qbId);
  const olI = tracks.map((_, i) => i).filter(i => tracks[i].team === play.off && tracks[i].p.pos === 'OL');
  const centerI = olI.length ? olI.reduce((a, i) => Math.abs(raw(i, 0).y - C) < Math.abs(raw(a, 0).y - C) ? i : a) : -1;
  const carrierI = idx(anim.carrierId), targetI = idx(m.targetId), defI = idx(A.defender);
  const passing = m.targetId && m.tThrow !== undefined;
  const nearestOpp = (i, t, excl = []) => {
    let best = -1, bd = 1e9; const p = raw(i, t);
    tracks.forEach((tr, j) => { if (tr.team === tracks[i].team || excl.includes(j)) return; const q = raw(j, t), d = Math.hypot(q.x - p.x, q.y - p.y); if (d < bd) { bd = d; best = j; } });
    return [best, bd];
  };
  const rawHead = (i, t) => { const a = raw(i, Math.max(0, t - .03)), b = raw(i, t); const n = Math.hypot(b.x - a.x, b.y - a.y); return n > .05 ? unit2(b.x - a.x, b.y - a.y) : [teamDir(i), 0]; };

  const overrides = [], allow = [], locks = [], falls = [], fx = [], contacts = [];
  // Línia: cada defensiu de línia s'enganxa al de davant i s'empenyen.
  const engaged = [];
  if (scrim && kind !== 'kneel') {
    const taken = new Set();
    tracks.forEach((tr, j) => {
      if (tr.team === play.off || tr.p.pos !== 'DL') return;
      let best = -1, bd = 1e9; const q = raw(j, .05);
      for (const o of olI) { if (taken.has(o)) continue; const p = raw(o, .05), d = Math.hypot(p.x - q.x, p.y - q.y); if (d < bd) { bd = d; best = o; } }
      if (best < 0) return;
      taken.add(best);
      engaged.push({ o: best, d: j, t1: .55 });
    });
  }
  // Qui porta la pilota (per no moure'l quan s'aparten els altres).
  let th = m.tHand;
  if (m.tHand !== undefined && qbI >= 0 && carrierI >= 0) {
    // L'entrega: el moment en què el corredor passa més a prop del quarterback.
    let bd = 1e9;
    for (let t = .08; t <= .3; t += .005) { const a = raw(qbI, t), b = raw(carrierI, t), d = Math.hypot(a.x - b.x, a.y - b.y); if (d < bd) { bd = d; th = t; } }
  }
  const tGot = passing ? m.tCatch : th !== undefined ? th : kind === 'kickoff' ? (m.tCatch || 1) : 0;
  const holderAt = t => {
    if (passing) return t < m.tThrow ? qbI : t >= m.tCatch ? carrierI : -1;
    if (th !== undefined) return t < th ? qbI : carrierI;
    if (kind === 'kickoff' || kind === 'punt') return t >= (m.tCatch || 1) ? carrierI : -1;
    return carrierI >= 0 ? carrierI : qbI;
  };

  // Placatge.
  const tackled = carrierI >= 0 && !play.td && !['kneel', 'pat', 'fg', 'punt'].includes(kind) && !(kind === 'kickoff' && play.detail === 'touchback');
  let tk = -1, tc = -1, gang = [], travel = [dir, 0], ap = [-dir, 0];
  if (tackled) {
    tk = idx(kind === 'sack' ? A.defender : A.tackler);
    if (tk < 0 || tracks[tk].team === tracks[carrierI].team) tk = nearestOpp(carrierI, .95)[0];
    if (kind === 'sack') tc = .58;
    else {
      for (let t = Math.max(tGot + .06, .3); t <= .95; t += .005) { const a = raw(carrierI, t), b = raw(tk, t); if (Math.hypot(a.x - b.x, a.y - b.y) < 1.5) { tc = t; break; } }
      if (tc < 0) tc = .9;
    }
    tc = clamp(tc, Math.min(.9, tGot + .08), .93);
    const c0 = raw(carrierI, tc - .03), k0 = raw(tk, tc - .03);
    ap = unit2(k0.x - c0.x, k0.y - c0.y);
    const vc = rawHead(carrierI, tc);
    travel = kind === 'sack' ? [-ap[0], -ap[1]] : unit2(vc[0] - ap[0] * .5, vc[1] - ap[1] * .5);
    // Placatge en grup: un o dos més s'hi tiren a sobre.
    if (kind !== 'sack' && (R() < .45 || play.hl)) {
      const cands = [];
      tracks.forEach((tr, j) => { if (tr.team === tracks[carrierI].team || j === tk) return; const a = raw(carrierI, tc), b = raw(j, tc), d = Math.hypot(a.x - b.x, a.y - b.y); if (d < 7.5) cands.push([d, j]); });
      cands.sort((a, b) => a[0] - b[0]);
      gang = cands.slice(0, R() < .5 ? 1 : 2).map(([, j], n) => ({ j, tj: tc + .015 * (n + 1), side: n % 2 ? -1 : 1 }));
    }
  }
  const pile = tackled ? [carrierI, tk, ...gang.map(g => g.j)] : [];
  const cPos0 = tackled ? raw(carrierI, tc) : null;
  const pileAt = t => { const k = 1.1 * easeOut(seg(t, tc, tc + .1)); return { x: cPos0.x + travel[0] * k, y: cPos0.y + travel[1] * k }; };

  // Stiff-arm a les curses llargues: un defensor arriba de costat i el corredor l'aparta.
  let stiff = null;
  const longRun = carrierI >= 0 && (kind === 'run' || kind === 'scramble' || kind === 'kickoff' || (kind === 'pass' && play.complete)) && Math.abs(play.yards || 0) >= 15;
  if (longRun) {
    const ts = Math.max(.55, tGot + .14), limit = tackled ? tc - .14 : .86;
    if (ts < limit) {
      const [sd, dd] = nearestOpp(carrierI, ts, [tk, ...gang.map(g => g.j)]);
      if (sd >= 0 && dd < 16) {
        const h = rawHead(carrierI, ts), cp = raw(carrierI, ts), q = raw(sd, ts);
        const side = (-h[1] * (q.x - cp.x) + h[0] * (q.y - cp.y)) >= 0 ? 1 : -1;
        stiff = { j: sd, ts, side, h, at: { x: cp.x - h[1] * side * .75 + h[0] * .25, y: cp.y + h[0] * side * .75 + h[1] * .25 } };
      }
    }
  }

  // Overrides (en ordre): línia, entrega, stiff-arm, placatge i grup.
  for (const e of engaged) overrides.push((t, P) => {
    const k = ease(seg(t, .03, .09)) * (1 - ease(seg(t, e.d === tk ? .28 : e.t1, (e.d === tk ? .28 : e.t1) + .1)));
    if (k <= 0) return;
    const o = P[e.o], q = P[e.d], tgt = { x: o.x + dir * .82, y: lerp(q.y, o.y, .75) };
    P[e.d] = { x: lerp(q.x, tgt.x, k), y: lerp(q.y, tgt.y, k) };
  });
  if (th !== undefined && qbI >= 0 && carrierI >= 0) {
    overrides.push((t, P) => {
      const k = Math.sin(Math.PI * seg(t, th - .07, th + .05));
      if (k <= 0) return;
      const r = P[carrierI], q = P[qbI], u = unit2(q.x - r.x, q.y - r.y), tgt = { x: r.x + u[0] * .7, y: r.y + u[1] * .7 };
      P[qbI] = { x: lerp(q.x, tgt.x, k), y: lerp(q.y, tgt.y, k) };
    });
    allow.push([qbI, carrierI, th - .08, th + .06]);
  }
  if (stiff) {
    const s = stiff, away = unit2(s.at.x - raw(carrierI, s.ts).x, s.at.y - raw(carrierI, s.ts).y);
    overrides.push((t, P) => {
      if (t < s.ts - .08) return;
      if (t < s.ts) { const k = ease(seg(t, s.ts - .08, s.ts)), c = P[carrierI], tgt = { x: c.x - s.h[1] * s.side * .75 + s.h[0] * .25, y: c.y + s.h[0] * s.side * .75 + s.h[1] * .25 }; P[s.j] = { x: lerp(P[s.j].x, tgt.x, k), y: lerp(P[s.j].y, tgt.y, k) }; return; }
      const k = 1.3 * easeOut(seg(t, s.ts, s.ts + .08));
      P[s.j] = { x: s.at.x + away[0] * k, y: s.at.y + away[1] * k };
    });
    allow.push([carrierI, s.j, s.ts - .03, s.ts + .03]);
    locks.push([s.j, s.ts - .08, 2]);
    falls.push({ i: s.j, t0: s.ts + .01, t1: s.ts + .08, dir: away });
    fx.push({ t: s.ts, kind: 'hit', at: s.at, big: false });
  }
  if (tackled) {
    overrides.push((t, P) => {
      if (t < tc - .08) return;
      const base = t >= tc ? pileAt(t) : P[carrierI];
      P[carrierI] = base;
      const fk = ease(seg(t, tc + .04, tc + .11));
      const att = { x: base.x + ap[0] * .55 + travel[0] * .3 * fk, y: base.y + ap[1] * .55 + travel[1] * .3 * fk };
      if (t < tc) { const k = ease(seg(t, tc - .08, tc)); P[tk] = { x: lerp(P[tk].x, att.x, k), y: lerp(P[tk].y, att.y, k) }; }
      else P[tk] = att;
      for (const g of gang) {
        if (t < g.tj - .08) continue;
        const q = P[g.j], a = unit2(q.x - base.x, q.y - base.y);
        const ang = g.side * .9, c = Math.cos(ang), s = Math.sin(ang);
        const off = t < g.tj ? a : unit2(ap[0] * c - ap[1] * s, ap[0] * s + ap[1] * c);
        const tgt = { x: base.x + off[0] * .62 + travel[0] * .2, y: base.y + off[1] * .62 + travel[1] * .2 };
        const k = t < g.tj ? ease(seg(t, g.tj - .08, g.tj)) : 1;
        P[g.j] = { x: lerp(q.x, tgt.x, k), y: lerp(q.y, tgt.y, k) };
      }
    });
    for (let a = 0; a < pile.length; a++) for (let b = a + 1; b < pile.length; b++) allow.push([pile[a], pile[b], tc - .04, 2]);
    for (const j of pile) locks.push([j, tc - .08, 2]);
    falls.push({ i: carrierI, t0: tc + .03, t1: tc + .1, dir: travel }, { i: tk, t0: tc + .04, t1: tc + .11, dir: travel });
    gang.forEach((g, n) => falls.push({ i: g.j, t0: g.tj + .03, t1: g.tj + .1, dir: unit2(travel[0] + (n ? -.3 : .3) * travel[1], travel[1] - (n ? -.3 : .3) * travel[0]) }));
    const big = kind === 'sack' || gang.length > 0 || !!play.hl;
    contacts.push({ tc, big, carrier: carrierI, tackler: tk, gang: gang.map(g => g.j), travel, ap });
    fx.push({ t: tc, kind: 'hit', at: null, big }, { t: tc + .1, kind: 'turf', at: null, big });
  }
  const lockedAt = (i, t) => locks.some(([j, t0]) => j === i && t >= t0);
  const allowed = (i, j, t) => allow.some(([a, b, t0, t1]) => ((a === i && b === j) || (a === j && b === i)) && t >= t0 && t <= t1);

  // Posicions resoltes per a cada instant (amb memòria curta).
  const cache = new Map();
  anim.frameAt = t0 => {
    // t arrodonit: el mateix instant dona exactament el mateix, sigui quin sigui l'ordre de les crides.
    const key = Math.round(t0 * 1e6), t = key / 1e6;
    let P = cache.get(key);
    if (P) return P;
    P = tracks.map(tr => { const q = tr.path(t); return { x: q.x, y: q.y }; });
    for (const f of overrides) f(t, P);
    const pin = holderAt(t);
    const w = tracks.map((_, i) => i === pin ? 0 : lockedAt(i, t) ? .15 : 1);
    for (let it = 0; it < 4; it++) for (let i = 0; i < N; i++) for (let j = i + 1; j < N; j++) {
      const dx = P[j].x - P[i].x, dy = P[j].y - P[i].y, d = Math.hypot(dx, dy);
      if (d >= SEP || allowed(i, j, t)) continue;
      const sum = w[i] + w[j];
      if (sum <= 0) continue;
      const u = d > 1e-6 ? [dx / d, dy / d] : [Math.cos(i * 2.4 + j), Math.sin(i * 2.4 + j)], push = SEP - d;
      P[i] = { x: P[i].x - u[0] * push * w[i] / sum, y: P[i].y - u[1] * push * w[i] / sum };
      P[j] = { x: P[j].x + u[0] * push * w[j] / sum, y: P[j].y + u[1] * push * w[j] / sum };
    }
    if (cache.size > 48) cache.clear();
    cache.set(key, P);
    return P;
  };
  tracks.forEach((tr, i) => { tr.pos = t => anim.frameAt(t)[i]; });
  // Cap on mira: on va (o el seu atac si està quiet o recula), el placador cap al corredor.
  anim.headingOf = (i, t) => {
    if (tackled && t >= tc - .08 && pile.includes(i) && i !== carrierI) { const a = anim.frameAt(t)[i], c = anim.frameAt(t)[carrierI]; return [...unit2(c.x - a.x, c.y - a.y), 0]; }
    for (const e of engaged) if ((e.o === i || e.d === i) && t > .04 && t < e.t1) return [teamDir(i), 0, 0];
    const a = anim.frameAt(Math.max(0, t - .012))[i], b = anim.frameAt(Math.min(1, t + .012))[i];
    const dx = b.x - a.x, dy = b.y - a.y, n = Math.hypot(dx, dy), td = teamDir(i);
    if (n < .06 || (dx * td < -.5 * n && t < .55)) return [td, 0, n];
    return [dx / n, dy / n, n];
  };
  const fallOf = (i, t) => { for (const f of falls) if (f.i === i && t >= f.t0) return { k: ease(seg(t, f.t0, f.t1)), dir: f.dir }; return null; };
  anim.fallOf = fallOf;

  // Tipus d'agafada del receptor.
  let catchType = 'field';
  if (passing) {
    if (m.deep || (play.depth || 0) >= 20) catchType = 'over';
    else if (play.hl && R() < .6) catchType = 'high';
    else if (play.complete && Math.abs((play.yards || 0) - (play.depth || 0)) < 1.5 && R() < .45) catchType = 'low';
    if (catchType === 'low' && targetI >= 0) falls.push({ i: targetI, t0: m.tCatch - .03, t1: m.tCatch + .04, dir: rawHead(targetI, m.tCatch) });
  }
  const catcher = passing ? (play.int && defI >= 0 ? defI : targetI) : -1;
  const jumpOf = (i, t) => (passing && i === catcher && catchType === 'high') ? .55 * Math.sin(Math.PI * seg(t, m.tCatch - .07, m.tCatch + .07)) : 0;
  anim.jumpOf = jumpOf;

  // On és una agafada a l'instant t (segueix el cos quan cau).
  const gripPt = (i, t, grip, side = 1, grip2, k = 0) => {
    const P = anim.frameAt(t)[i], h = anim.headingOf(i, t), r = [-h[1], h[0]];
    const g0 = GRIPS[grip], g = grip2 ? g0.map((v, n) => lerp(v, GRIPS[grip2][n], k)) : g0;
    let x = P.x + h[0] * g[0] + r[0] * g[1] * side, y = P.y + h[1] * g[0] + r[1] * g[1] * side, z = g[2] + jumpOf(i, t);
    const f = fallOf(i, t);
    if (f) { const a = f.k * Math.PI / 2 * .85; x += f.dir[0] * z * Math.sin(a); y += f.dir[1] * z * Math.sin(a); z = Math.max(BALL_R + .05, z * Math.cos(a)); }
    const up = grip === 'ground' ? .95 : grip === 'tuck' || grip2 === 'tuck' ? .45 : .2;
    return { x, y, z, axis: unit3([h[0], h[1], up]), hands: g[3] > 1.5 ? 2 : 1 };
  };
  // Tuck: al braç del costat contrari del defensor més proper.
  const tuckSide = (i, t) => {
    if (i < 0) return 1;
    const [j] = nearestOpp(i, t); if (j < 0) return 1;
    const P = raw(i, t), q = raw(j, t), h = rawHead(i, t);
    return (-h[1] * (q.x - P.x) + h[0] * (q.y - P.y)) > 0 ? -1 : 1;
  };

  // Línia de temps de la pilota.
  const tl = [];
  const hold = (t0, t1, who, grip, side, grip2) => { if (who >= 0 && t1 > t0) tl.push({ t0, t1, type: 'hold', who, grip, side: side || 1, grip2 }); };
  const give = (t0, t1, a, ga, b, gb, side) => { if (a >= 0 && b >= 0) tl.push({ t0, t1, type: 'pass', a, ga, b, gb, side: side || 1 }); };
  const fly = (t0, t1, p0, p1, peak, spin) => tl.push({ t0, t1, type: 'fly', p0, p1, peak, spin });
  const loose = (t0, p0, v0, n) => tl.push({ t0, t1: 99, type: 'loose', p0: [p0.x, p0.y, p0.z], segs: looseBall([p0.x, p0.y, p0.z], v0, (play.n || 1) * 131 + n) });
  const still = (t0, t1, p, axis) => tl.push({ t0, t1, type: 'still', p, axis });
  const flyVel = (f, t1) => { const T = Math.max(.05, (f.t1 - f.t0) * dur); return [(f.p1.x - f.p0.x) / T, (f.p1.y - f.p0.y) / T, (f.p1.z - f.p0.z - 4 * f.peak) / T]; };
  const pt = g => ({ x: g.x, y: g.y, z: g.z });
  // El snap: del centre al quarterback (a les mans si és a sota, volant si és en escopeta).
  const snapTo = (who, grip, tEnd) => {
    if (centerI < 0 || who < 0) return 0;
    const far = Math.hypot(raw(who, 0).x - raw(centerI, 0).x, raw(who, 0).y - raw(centerI, 0).y) > 1.6;
    hold(0, .03, centerI, 'snap');
    if (far) { const t1 = tEnd || .09; fly(.03, t1, pt(gripPt(centerI, .03, 'snap')), pt(gripPt(who, t1, grip)), .25, 'spiral'); return t1; }
    give(.03, .06, centerI, 'snap', who, grip); return .06;
  };
  const tuckAfter = (who, t0, from) => { const sd = tuckSide(who, t0); hold(t0, t0 + .06, who, from, sd, 'tuck'); return sd; };
  let fumbleAt = -1, tuck = 1;

  if (kind === 'kickoff') {
    const kx = fieldX(65, dir), tee = { x: kx, y: C, z: .25 };
    still(0, .05, tee, [0, 0, 1]);
    if (play.detail === 'touchback' || carrierI < 0) {
      const land = { x: fieldX(-2, dir) - dir * 2, y: C + (R() - .5) * 6, z: BALL_R };
      const f = { t0: .05, t1: .7, p0: tee, p1: land, peak: 12, spin: 'flip' };
      tl.push({ ...f, type: 'fly' });
      const v = flyVel(f); loose(.7, land, [v[0] * .55, v[1] * .55, v[2]], 1);
    } else {
      const tC = m.tCatch || .55;
      fly(.05, tC, tee, pt(gripPt(carrierI, tC, 'field')), 12, 'flip');
      tuck = tuckAfter(carrierI, tC, 'field'); hold(tC + .06, 1.01, carrierI, 'tuck', tuck);
    }
  } else if (kind === 'punt' || kind === 'fg' || kind === 'pat') {
    const kI = idx(m.kickerId);
    if (kind === 'punt') {
      const tS = .12;
      if (centerI >= 0 && kI >= 0) { hold(0, .02, centerI, 'snap'); fly(.02, tS, pt(gripPt(centerI, .02, 'snap')), pt(gripPt(kI, tS, 'field')), .4, 'spiral'); hold(tS, .25, kI, 'field', 1, 'carry'); }
      const from = kI >= 0 ? pt(gripPt(kI, .25, 'carry')) : { x: anim.los, y: C, z: 1 };
      const ret = idx(m.returnerId), caught = ret >= 0 && !['downed', 'touchback', 'out', 'blocked'].includes(play.detail);
      if (caught) { fly(.25, .8, from, pt(gripPt(ret, .8, 'field')), 11, 'spiral'); hold(.8, 1.01, ret, 'field'); }
      else { const land = { x: anim.endX, y: C + Math.sin(3), z: BALL_R }, f = { t0: .25, t1: .8, p0: from, p1: land, peak: 11 }; fly(.25, .8, from, land, 11, 'spiral'); const v = flyVel(f); loose(.8, land, [v[0] * .6, v[1] * .6, v[2]], 2); }
    } else {
      // Gol de camp / extra: snap baix al que l'aguanta, a terra, i el xut de punta a punta.
      const hI = qbI;
      if (centerI >= 0 && hI >= 0) { hold(0, .03, centerI, 'snap'); fly(.03, .17, pt(gripPt(centerI, .03, 'snap')), pt(gripPt(hI, .17, 'hand')), .15, 'spiral'); hold(.17, .3, hI, 'hand', 1, 'ground'); }
      const from = hI >= 0 ? pt(gripPt(hI, .3, 'ground')) : { x: anim.los - dir * 7, y: C, z: .2 };
      const end = anim.rawBall(.85);
      const f = { t0: .3, t1: .85, p0: from, p1: { x: end.x, y: end.y, z: end.z }, peak: 4.5 };
      fly(.3, .85, f.p0, f.p1, 4.5, 'flip');
      const v = flyVel(f); loose(.85, f.p1, [v[0] * .7, v[1] * .7, v[2]], 3);
    }
  } else if (passing) {
    const tS = snapTo(qbI, 'carry');
    hold(tS, m.tThrow - .08, qbI, 'carry');
    hold(m.tThrow - .08, m.tThrow, qbI, 'carry', 1, 'cock');
    const grip = catchType === 'high' ? 'high' : catchType === 'low' ? 'low' : catchType === 'over' ? 'over' : 'field';
    const p0 = pt(gripPt(qbI, m.tThrow, 'cock'));
    const who = catcher >= 0 ? catcher : targetI;
    const p1 = pt(gripPt(who, m.tCatch, grip));
    const peak = m.deep ? 6 : play.detail === 'medium' ? 3.5 : 1.8;
    const f = { t0: m.tThrow, t1: m.tCatch, p0, p1, peak };
    fly(m.tThrow, m.tCatch, p0, p1, peak, 'spiral');
    if (play.complete || play.int) {
      tuck = tuckAfter(who, m.tCatch, grip);
      hold(m.tCatch + .06, 1.01, who, 'tuck', tuck);
    } else {
      const v = flyVel(f), h = rawHead(targetI, m.tCatch);
      if (defI >= 0) loose(m.tCatch, p1, [v[0] * .15 - h[1] * 3.5, v[1] * .15 + h[0] * 3.5, 4.5], 4); // la desvia el defensor
      else loose(m.tCatch, p1, [v[0] * .3, v[1] * .3, 1.4], 5); // li rebota a les mans
    }
  } else if (qbI >= 0) {
    // Curses, scrambles, sacks i genolls a terra.
    const tS = snapTo(qbI, 'carry');
    if (th !== undefined && carrierI >= 0 && carrierI !== qbI) {
      hold(tS, th - .03, qbI, 'carry');
      give(th - .03, th + .02, qbI, 'carry', carrierI, 'hand');
      tuck = tuckAfter(carrierI, th + .02, 'hand');
      hold(th + .08, 1.01, carrierI, 'tuck', tuck);
    } else if (kind === 'sack') {
      hold(tS, tc - .04, qbI, 'carry'); hold(tc - .04, tc, qbI, 'carry', 1, 'clutch'); hold(tc, 1.01, qbI, 'clutch');
    } else if (kind === 'kneel') {
      hold(tS, .2, qbI, 'carry'); hold(.2, .35, qbI, 'carry', 1, 'kneel'); hold(.35, 1.01, qbI, 'kneel');
    } else {
      hold(tS, .4, qbI, 'carry'); tuck = tuckAfter(qbI, .4, 'carry'); hold(.46, 1.01, qbI, 'tuck', tuck);
    }
  }
  tl.sort((a, b) => a.t0 - b.t0);
  // Pilota perduda: surt de les mans en el placatge.
  if (play.fumble && tackled) {
    fumbleAt = tc + .02;
    const cut = tl.findIndex(s => s.t1 > fumbleAt);
    const holdSeg = tl[cut];
    if (holdSeg) {
      const p = (holdSeg.type === 'hold') ? pt(gripPt(holdSeg.who, fumbleAt, holdSeg.grip, holdSeg.side)) : { x: cPos0.x, y: cPos0.y, z: 1 };
      tl.splice(cut + 1);
      holdSeg.t1 = fumbleAt;
      loose(fumbleAt, p, [travel[0] * 3.5 + ap[1] * 2, travel[1] * 3.5 - ap[0] * 2, 3.2], 6);
    }
  }
  anim.timeline = tl;
  const ballCache = new Map();
  anim.ball = t0 => {
    const key = Math.round(t0 * 1e6), t = key / 1e6;
    const hit = ballCache.get(key);
    if (hit) return hit;
    let s = tl[tl.length - 1];
    for (const q of tl) if (t < q.t1) { s = q; break; }
    let out;
    if (!s) out = { x: anim.los, y: C, z: BALL_R, axis: [1, 0, 0], spin: 0, holder: -1, hands: 0, mode: 'still' };
    else if (t < s.t0 && s === tl[0] && s.type === 'hold') out = { ...gripPt(s.who, t, s.grip, s.side), holder: s.who, spin: 0, mode: 'held' };
    else if (s.type === 'hold') {
      const k = s.grip2 ? ease(seg(t, s.t0, s.t1)) : 0, g = gripPt(s.who, t, s.grip, s.side, s.grip2, k);
      out = { ...g, holder: s.who, spin: 0, mode: 'held', grip: k > .5 ? s.grip2 : s.grip };
    } else if (s.type === 'pass') {
      const k = ease(seg(t, s.t0, s.t1)), a = gripPt(s.a, t, s.ga), b = gripPt(s.b, t, s.gb, s.side);
      out = { x: lerp(a.x, b.x, k), y: lerp(a.y, b.y, k), z: lerp(a.z, b.z, k), axis: k < .5 ? a.axis : b.axis, hands: 2, holder: k < .5 ? s.a : s.b, giver: k < .5 ? s.b : s.a, spin: 0, mode: 'held' };
    } else if (s.type === 'fly') {
      const k = seg(t, s.t0, s.t1), tau = (t - s.t0) * dur;
      const x = lerp(s.p0.x, s.p1.x, k), y = lerp(s.p0.y, s.p1.y, k), z = lerp(s.p0.z, s.p1.z, k) + s.peak * 4 * k * (1 - k);
      const tan = unit3([s.p1.x - s.p0.x, s.p1.y - s.p0.y, s.p1.z - s.p0.z + s.peak * 4 * (1 - 2 * k)]);
      let axis = tan, spin = tau * 38;
      if (s.spin === 'flip') { const h = unit2(tan[0], tan[1]), phi = tau * Math.PI * 2 * 2.3; axis = [h[0] * Math.cos(phi), h[1] * Math.cos(phi), Math.sin(phi)]; spin = 0; }
      out = { x, y, z: Math.max(BALL_R, z), axis, spin, spiral: s.spin === 'spiral', holder: -1, hands: 0, mode: 'fly' };
    } else if (s.type === 'loose') {
      out = { ...looseAt(s.segs, Math.max(0, t - s.t0) * dur), holder: -1, hands: 0, mode: 'loose' };
    } else out = { x: s.p.x, y: s.p.y, z: s.p.z, axis: s.axis, spin: 0, holder: -1, hands: 0, mode: 'still' };
    if (ballCache.size > 48) ballCache.clear();
    ballCache.set(key, out);
    return out;
  };

  // Qui xuta d'inici: el de l'equip que xuta més a prop del tee.
  let kickerI = -1;
  if (kind === 'kickoff') { let bd = 1e9; const kx = fieldX(65, dir); tracks.forEach((tr, j) => { if (tr.team === play.off) return; const q = raw(j, .05), d = Math.hypot(q.x - kx, q.y - C); if (d < bd) { bd = d; kickerI = j; } }); }

  // Postura de cada jugador: inclinació, caiguda, salt, braços (punts on van
  // les mans, al món), xut i ajupit.
  anim.poseOf = (i, t) => {
    const pose = { lean: 0, fall: fallOf(i, t), jump: jumpOf(i, t), arms: null, kick: 0, crouch: 0 };
    const P = anim.frameAt(t)[i], b = anim.ball(t), bp = [b.x, b.y, b.z];
    const hd = anim.headingOf(i, t);
    pose.lean = b.holder === i && b.grip !== 'tuck' ? 0 : clamp(hd[2] * 1.2, 0, .35);
    if (b.holder === i || b.giver === i) pose.arms = b.hands === 2 || b.giver === i ? [bp, bp] : [bp, null];
    if (i === qbI && passing && t >= m.tThrow && t < m.tThrow + .08) {
      // Acaba el llançament: el braç endavant.
      pose.arms = [[P.x + hd[0] * .95, P.y + hd[1] * .95, 1.85 - seg(t, m.tThrow, m.tThrow + .08) * .6], [P.x + hd[0] * .5, P.y + hd[1] * .5, 1.2]];
      pose.lean = .4;
    }
    if (passing && i === (catcher >= 0 ? catcher : targetI) && t >= m.tCatch - .08 && t < m.tCatch) {
      const near = Math.hypot(b.x - P.x, b.y - P.y) < 8;
      const reach = near ? bp : [P.x + hd[0] * .6, P.y + hd[1] * .6, catchType === 'over' ? 1.8 : 1.5];
      pose.arms = [reach, reach]; pose.lean = 0;
    }
    if (passing && !play.complete && !play.int && i === defI && t >= m.tCatch - .05 && t < m.tCatch + .03) pose.arms = [bp, null], pose.lean = .3;
    for (const e of engaged) if ((e.o === i || e.d === i) && t > .05 && t < (e.d === tk ? .28 : e.t1)) {
      const o = anim.frameAt(t)[e.o === i ? e.d : e.o]; pose.arms = [[o.x, o.y, 1.15], [o.x, o.y, 1.25]]; pose.lean = .55;
    }
    if (stiff && t > stiff.ts - .05 && t < stiff.ts + .04) {
      if (i === carrierI) { const q = anim.frameAt(t)[stiff.j]; pose.arms = [pose.arms ? pose.arms[0] : bp, [q.x, q.y, 1.45]]; }
      if (i === stiff.j) pose.lean = .5;
    }
    if (tackled && t >= tc - .06 && (i === tk || gang.some(g => g.j === i))) {
      const c = anim.frameAt(t)[carrierI];
      pose.arms = [[c.x - travel[1] * .25, c.y + travel[0] * .25, 1.0], [c.x + travel[1] * .25, c.y - travel[0] * .25, 1.0]];
      pose.lean = .85;
    }
    if (kind === 'kickoff' && i === kickerI) pose.kick = Math.sin(Math.PI * seg(t, .02, .1));
    if ((kind === 'fg' || kind === 'pat') && i === idx(m.kickerId)) pose.kick = Math.sin(Math.PI * seg(t, .26, .36));
    if (kind === 'punt' && i === idx(m.kickerId)) pose.kick = Math.sin(Math.PI * seg(t, .21, .31));
    if ((kind === 'fg' || kind === 'pat') && i === qbI) pose.crouch = 1;
    if (i === centerI) pose.crouch = Math.max(pose.crouch, 1 - seg(t, .03, .1));
    if (kind === 'kneel' && i === qbI) pose.crouch = ease(seg(t, .25, .4));
    return pose;
  };
  anim.contacts = contacts;
  anim.fx = fx.map(f => ({ ...f, at: f.at || (tackled ? pileAt(f.t) : { x: anim.los, y: C }) }));
  anim.tHand = th;
  anim.tuckSide = tuck;
  anim.holderAt = holderAt;
  anim.inContact = allowed;
  // L'espatlla (on neix el braç) al món: amb la postura, el salt i la caiguda.
  anim.shoulderOf = (i, t) => {
    const P = anim.frameAt(t)[i], pose = anim.poseOf(i, t);
    let x = P.x, y = P.y, z = 1.45 - pose.lean * .24 - pose.crouch * .42 + pose.jump;
    if (pose.fall) { const a = pose.fall.k * Math.PI / 2 * .85; x += pose.fall.dir[0] * z * Math.sin(a); y += pose.fall.dir[1] * z * Math.sin(a); z *= Math.cos(a); }
    return { x, y, z };
  };
  return anim;
}

// ---- càmeres ----
// Totes les càmeres són estenopeiques de debò: una posició P, un punt on
// miren L i una focal. La de televisió és a la banda de baix i llisca seguint
// la pilota; les altres (jugador, skycam, zona de marca, pal, carril, dirigible)
// es calculen per a cada instant de la jugada i la càmera hi va suaument.
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

// Nom de cada càmera a la cantonada de la pantalla.
export const CAM_TAGS = { tv: '', sky: 'SKYCAM', player: 'PLAYER CAM', endzone: 'END ZONE', pylon: 'PYLON', rail: 'RAIL CAM', blimp: 'BLIMP' };

export function isBig(play) {
  const kick = ['kickoff', 'punt', 'fg', 'pat'].includes(play.kind);
  return !!(play.td || play.int || play.fumble || play.safety || play.hl || (!kick && Math.abs(play.yards || 0) >= 20));
}
const trackOf = (anim, id) => (id && anim.tracks.find(tr => tr.p.id === id)) || null;
const ballAt = (anim, t) => (anim.ball && anim.ball(t)) || { x: anim.los, y: C, z: 1 };
// Cap on corre qui acaba amb la pilota (una intercepció torna enrere).
function scoreDir(anim, play) { const c = trackOf(anim, anim.carrierId); return c && c.team !== play.off ? -anim.dir : anim.dir; }

// Skycam: darrere la pilota, enlairada (o darrere el xutador en un gol de
// camp, seguint la pilota cap als pals).
export function skyShot(anim, kind, t) {
  const dir = anim.dir, u = v => v * dir;
  const b = ballAt(anim, t);
  const bz = Math.min(b.z || 0, 9);
  if (kind === 'fg' || kind === 'pat') {
    const goalX = dir > 0 ? 120 : 0, kickFrom = anim.los - dir * 7;
    const start = u(kickFrom) - 12;
    const camU = Math.min(Math.max(start, u(b.x) - 18), Math.max(start, u(goalX) - 24));
    return { P: [camU * dir, C - 1.6, 7.5 + bz * .2], L: [goalX, C, 2.5 + bz * .2], zoom: 1.2, kP: 3.2, kL: 4.5 };
  }
  const kick = kind === 'kickoff' || kind === 'punt';
  const fx = (kick ? u(b.x) : Math.max(u(b.x), u(anim.los) - 4)) * dir;
  const ly = clamp(b.y, 2, FIELD_W - 2);
  return { P: [fx - dir * 17, lerp(C, ly, .55), 8 + bz * .25], L: [fx + dir * 14, lerp(C, ly, .8), bz * .3], zoom: 1, kP: 3.2, kL: 4.5 };
}

// A qui segueix la càmera de jugador: el quarterback, el receptor (i, quan
// l'ha agafada, qui porta la pilota), el retornador o, per defecte, qui la porta.
export function subjectOf(anim, t, who = 'auto') {
  const m = anim.marks || {};
  let id;
  if (who === 'qb') id = m.qbId;
  else if (who === 'target') id = (t >= (m.tCatch === undefined ? 0 : m.tCatch) && anim.carrierId) || m.targetId;
  else if (who === 'returner') id = m.returnerId || anim.carrierId;
  else id = (m.tHand !== undefined && t < m.tHand) ? m.qbId : (anim.carrierId || m.qbId);
  return trackOf(anim, id) || trackOf(anim, m.qbId) || anim.tracks[0];
}

// Càmera de jugador: baixa, per sobre l'espatlla de qui segueix, 4 iardes
// darrere i a l'alçada del casc, mirant cap on corre. Mentre la pilota vola,
// la del quarterback la segueix amb la mirada.
export function playerShot(anim, play, t, who = 'auto') {
  const m = anim.marks || {};
  const tr = subjectOf(anim, t, who);
  const at = tr.pos || tr.path, pos = at(t), a = at(Math.max(0, t - .03)), b = at(Math.min(1, t + .03));
  const own = tr.team === play.off ? anim.dir : -anim.dir;
  let hx = b.x - a.x, hy = b.y - a.y;
  const sp = Math.hypot(hx, hy);
  if (tr.p.id === m.qbId || sp < .3) { hx = own; hy = 0; }
  else { hx = hx / sp + own * .5; hy /= sp; const n = Math.hypot(hx, hy) || 1; hx /= n; hy /= n; }
  const rx = -hy, ry = hx;
  const P = [pos.x - hx * 4.3 + rx * .8, pos.y - hy * 4.3 + ry * .8, 2.15];
  let L = [pos.x + hx * 10, pos.y + hy * 10, 1.4];
  if (who === 'qb' && m.tThrow !== undefined && t > m.tThrow) {
    const bl = ballAt(anim, t), k = ease(seg(t, m.tThrow, m.tThrow + .05));
    L = [lerp(L[0], bl.x, k), lerp(L[1], bl.y, k), lerp(L[2], bl.z, k)];
  }
  return { P, L, zoom: 1, kP: 6, kL: 8, speed: sp / (.06 * anim.dur), subject: tr.p.id };
}

// Des de darrere els pals de la zona de marca on s'ataca, mirant la pilota.
export function endzoneShot(anim, play, t) {
  const sd = scoreDir(anim, play), gx = sd > 0 ? 120 : 0, b = ballAt(anim, t);
  const P = [gx + sd * 8, lerp(C, b.y, .3), 6.5];
  return { P, L: [b.x, b.y, 1.2], zoom: clamp(Math.abs(P[0] - b.x) / 20, 1, 2.6), kP: 3, kL: 6 };
}
// Arran de gespa, al pal (pylon) de la línia de gol per on entra la pilota.
export function pylonShot(anim, play, t) {
  const sd = scoreDir(anim, play), gl = sd > 0 ? 110 : 10, b = ballAt(anim, t), end = ballAt(anim, 1);
  const P = [gl + sd * .4, end.y < C ? -.6 : FIELD_W + .6, .3];
  return { P, L: [b.x, b.y, .9], zoom: clamp(Math.hypot(b.x - P[0], b.y - P[1]) / 9, 1, 3.2), kP: 8, kL: 7 };
}
// Càmera de carril a la banda, baixa, corrent amb la pilota.
export function railShot(anim, play, t) {
  const sd = scoreDir(anim, play), b = ballAt(anim, t);
  return { P: [b.x - sd * 2.5, FIELD_W + 6, 2.3], L: [b.x + sd * 3, b.y, 1], zoom: 1.25, kP: 4, kL: 5, speed: 0 };
}
// Dirigible: ben amunt, girant a poc a poc al voltant de la jugada.
export function blimpShot(anim, play, t) {
  const b = ballAt(anim, t), a = .9 + t * .5;
  return { P: [lerp(60, b.x, .6) - 55 * Math.cos(a) * anim.dir, C + 55 * Math.sin(a), 95], L: [lerp(60, b.x, .7), b.y, 0], zoom: 1.05, kP: 1.5, kL: 2.5 };
}
export function shotFor(mode, anim, play, t, who) {
  if (mode === 'player') return playerShot(anim, play, t, who);
  if (mode === 'endzone') return endzoneShot(anim, play, t);
  if (mode === 'pylon') return pylonShot(anim, play, t);
  if (mode === 'rail') return railShot(anim, play, t);
  if (mode === 'blimp') return blimpShot(anim, play, t);
  return skyShot(anim, play.kind, t);
}

// El realitzador: quina càmera va en directe a cada moment de la jugada
// (llista de trams {cam, at, who}) i quines repeticions es fan després. La
// càmera de jugador és la principal en curses, recepcions i retorns; la tele
// i la skycam donen context; les jugades grosses tenen repeticions a càmera
// lenta des de la zona de marca, el pal o el jugador.
export function directPlay(play, anim, pref = 'auto') {
  const m = anim.marks || {}, k = play.kind, n = play.n || 0, big = isBig(play);
  const S = (cam, at = 0, who = 'auto') => ({ cam, at, who });
  const passing = !!m.targetId && m.tThrow !== undefined;
  const flip = passing ? m.tThrow + .65 * (m.tCatch - m.tThrow) : 0;
  if (pref !== 'auto') {
    if (pref === 'player' && passing) return { live: [S('player', 0, 'qb'), S('player', flip, 'target')], replays: [] };
    return { live: [S(pref, 0, k === 'kickoff' ? 'returner' : 'auto')], replays: [] };
  }
  let live, replays = [];
  if (k === 'kickoff') live = m.tCatch ? [S('blimp'), S('player', m.tCatch - .12, 'returner')] : [S('blimp')];
  else if (k === 'punt' || k === 'fg') live = [S('sky')];
  else if (k === 'pat') live = [S(n % 2 ? 'endzone' : 'sky')];
  else if (k === 'kneel') live = [S('tv')];
  else if (passing) {
    if (n % 4 === 3) live = [S('tv')];
    else if (m.deep) live = [S('player', 0, 'qb'), S('sky', m.tThrow + .03), S('player', m.tCatch - .06, 'target')];
    else live = [S('player', 0, 'qb'), S('player', flip, 'target')];
  } else if ((play.spot || 0) >= 95 && n % 2 === 0) live = [S('endzone')];
  else if (n % 5 === 2) live = [S('player', 0, 'qb'), S('rail', .3)];
  else live = [S('player')];
  if (big && !['fg', 'pat', 'kneel'].includes(k)) {
    const from = passing ? Math.max(0, m.tThrow - .12) : k === 'kickoff' ? .45 : k === 'punt' ? .2 : .1;
    if (play.td) replays = [{ cam: 'endzone', from, rate: .6 }, { cam: 'pylon', from: .72, rate: .35 }];
    else if (k === 'kickoff') replays = [{ cam: 'player', who: 'returner', from, rate: .55 }];
    else if (k === 'punt') replays = [{ cam: 'blimp', from, rate: .7 }];
    else replays = [{ cam: live.some(s => s.cam === 'sky') ? 'rail' : 'sky', from, rate: .65 }, { cam: 'player', who: passing ? 'target' : 'auto', from: passing ? m.tCatch - .08 : .3, rate: .4 }];
  }
  return { live, replays };
}
// Tram del directe que toca a l'instant t.
export function liveSeg(plan, t) { let i = 0; plan.live.forEach((s, k) => { if (s.at <= t) i = k; }); return i; }

// Càmera lenta en directe: quan arriba la pilota al receptor i en el placatge.
export function slowmo(anim, play, t) {
  const m = anim.marks || {};
  if (m.tCatch !== undefined && (play.complete || play.int) && Math.abs(t - m.tCatch) < .025) return .3;
  const c = (anim.contacts || [])[0];
  if (c && Math.abs(t - (c.tc + .02)) < .025) return .35;
  return 1;
}
// Sacsejada de la càmera en el contacte (0..1).
function contact(anim, play, t) {
  let v = 0;
  for (const c of anim.contacts || []) if (t > c.tc && t < c.tc + .1) v = Math.max(v, (c.big ? 1 : .6) * (1 - seg(t, c.tc, c.tc + .1)));
  return v;
}
// Només els cops grossos sacsegen les càmeres de lluny.
function bigHit(anim, t) { let v = 0; for (const c of anim.contacts || []) if (c.big && t > c.tc && t < c.tc + .06) v = Math.max(v, 1 - seg(t, c.tc, c.tc + .06)); return v; }

// ---- l'estadi ----
// Una grada en forma de bol (superel·lipse) al voltant del camp, amb tres
// pisos, llotges il·luminades, el ràfec del sostre i quatre torres de llum.
const BOWL_A = 76, BOWL_B = 44, BOWL_N = 56;
function ring(th, o) {
  const c = Math.cos(th), s = Math.sin(th);
  return [60 + (BOWL_A + o) * Math.sign(c) * Math.sqrt(Math.abs(c)), C + (BOWL_B + o) * Math.sign(s) * Math.sqrt(Math.abs(s))];
}
// Angle per a una fracció u (0..1) del perímetre: així els trams i la gent
// queden repartits per igual (amb l'angle tal qual, els fons quedarien buits).
const RING_U = (() => {
  const n = 2048, th = [], acc = [0];
  for (let i = 0; i <= n; i++) th.push(i / n * Math.PI * 2);
  for (let i = 1; i <= n; i++) { const a = ring(th[i - 1], 20), b = ring(th[i], 20); acc.push(acc[i - 1] + Math.hypot(b[0] - a[0], b[1] - a[1])); }
  const out = new Float64Array(1025);
  let j = 0;
  for (let k = 0; k <= 1024; k++) { const L = k / 1024 * acc[n]; while (j < n && acc[j + 1] < L) j++; const f = (L - acc[j]) / ((acc[j + 1] - acc[j]) || 1); out[k] = lerp(th[j], th[Math.min(n, j + 1)], clamp(f, 0, 1)); }
  return out;
})();
function ringTh(u) { u = ((u % 1) + 1) % 1; const k = u * 1024, i = Math.floor(k); return lerp(RING_U[i], RING_U[Math.min(1024, i + 1)], k - i); }
// [desplaçament i alçada de baix, desplaçament i alçada de dalt, color, tipus]
const TIERS = [
  [0, 0, 0, 1.4, null, 'wall'],
  [0, 1.4, 22, 12, '#283252', 'seats'],
  [22, 12, 22, 15.5, '#0c1120', 'suite'],
  [22, 13.2, 22, 14.4, 'rgba(255,224,150,.55)', 'glass'],
  [22, 15.5, 40, 26, '#232c4a', 'seats'],
  [40, 26, 40, 29.5, '#0c1120', 'suite'],
  [40, 27.2, 40, 28.4, 'rgba(255,224,150,.45)', 'glass'],
  [40, 29.5, 58, 41, '#1f2843', 'seats'],
  [58, 41, 58, 47, '#11172a', 'rim'],
  [58, 47, 46, 48.5, '#1a2138', 'roof'],
  [46, 48.4, 46, 48.9, 'rgba(255,250,225,.9)', 'glass'],
];
const SEAT_ROWS = [[1, 9], [4, 8], [7, 7]]; // [índex del pis a TIERS, files]
const TOWERS = [.62, Math.PI - .62, Math.PI + .62, -.62];

function makeStadium(teams) {
  const polys = [];
  for (let i = 0; i < BOWL_N; i++) {
    const t0 = ringTh(i / BOWL_N), t1 = i + 1 === BOWL_N ? Math.PI * 2 : ringTh((i + 1) / BOWL_N);
    TIERS.forEach(([o0, z0, o1, z1, fill, kind], ti) => {
      const a = ring(t0, o0), b = ring(t1, o0), c = ring(t1, o1), d = ring(t0, o1);
      let col = fill;
      if (kind === 'wall') col = i % 2 ? shade(teams[0].color, -.35) : shade(teams[1].color, -.35);
      else if (kind === 'seats') {
        // Les grades agafen un to fosc de l'equip d'aquella banda (fa de públic llunyà).
        const th = ringTh((i + .5) / BOWL_N), side = Math.abs(Math.cos(th)) > .82 ? -1 : (Math.sin(th) < 0 ? 0 : 1);
        col = side < 0 ? (i % 4 ? '#2b3150' : '#343b5c') : shade(teams[side].color, i % 4 ? -.62 : -.55);
      }
      const pts = [[a[0], a[1], z0], [b[0], b[1], z0], [c[0], c[1], z1], [d[0], d[1], z1]];
      const cen = [(a[0] + c[0]) / 2, (a[1] + c[1]) / 2, (z0 + z1) / 2];
      polys.push({ pts, fill: col, cen, kind, bias: kind === 'glass' ? -.2 : -ti * .01 });
    });
  }
  // Públic: files de seients a cada pis, amb els colors de l'equip de cada banda.
  const R = rng(23), people = [];
  const pal = [teams[0].color, teams[0].color2 || '#fff', teams[1].color, teams[1].color2 || '#fff', '#c9c4b8', '#5b6170', '#8a6f55', '#2e3240'];
  for (const [ti, rows] of SEAT_ROWS) {
    const [o0, z0, o1, z1] = TIERS[ti];
    for (let r = 0; r < rows; r++) {
      const k = (r + .5) / rows, o = lerp(o0, o1, k), z = lerp(z0, z1, k) + .3;
      const per = Math.round((380 + o * 6.3) / 1.15);
      for (let j = 0; j < per; j++) {
        if (R() < .07) continue; // seients buits
        const th = ringTh((j + R() * .6) / per), [x, y] = ring(th, o);
        const side = Math.abs(Math.cos(th)) > .82 ? -1 : (y < C ? 0 : 1); // fons: barrejat
        const q = R();
        const team = side < 0 ? (R() < .5 ? 0 : 1) : (q < .82 ? side : 1 - side);
        const ci = q < .9 ? team * 2 + (R() < .72 ? 0 : 1) : 4 + Math.floor(R() * 4);
        people.push({ x, y, z, th, team: q < .9 ? team : -1, ci, ph: R() * 6.28, flag: R() < .025, lod: R() < .5 });
      }
    }
  }
  people.sort((a, b) => a.ci - b.ci);
  // Estrelles: direccions fixes del cel.
  const stars = Array.from({ length: 170 }, () => { const az = R() * 6.28, el = .12 + R() * 1.2; return [Math.cos(az) * Math.cos(el), Math.sin(az) * Math.cos(el), Math.sin(el), .3 + R() * .7]; });
  // Banquetes: cada equip a una banda, amb els jugadors de peu mirant el camp.
  const bench = [];
  [0, 1].forEach(team => {
    const y = team === 0 ? -9.5 : FIELD_W + 9.5;
    for (let i = 0; i < 18; i++) bench.push({ team, x: 38 + i * 2.5 + R() * 1.2, y: y + (team === 0 ? 1 : -1) * (1.2 + R() * 1.5), num: 2 + Math.floor(R() * 97) });
  });
  return { polys, people, pal, stars, bench, key: teams[0].color + teams[1].color };
}

// Segons que una jugada ocupa la pantalla a 1×: la pausa d'abans, la jugada,
// les repeticions lentes i la pausa de després (com a advance()).
export function playSeconds(play, match, pref = 'auto') {
  let anim;
  try { anim = buildPlay(play, match); } catch (e) { return 6; }
  const plan = directPlay(play, anim, pref);
  const rest = (play.comment ? 3.2 : 1.6) + (play.hl ? 1 : 0);
  let s = .7 + anim.dur;
  if (plan.replays.length) s += 1.4 + plan.replays.reduce((a, r) => a + (1 - r.from) * anim.dur / r.rate, 0);
  return s + rest;
}
// L'inici de cada jugada a la reproducció i el total (s'allarga quan n'arriben més).
const TIMES = new WeakMap();
export function playTimes(list, match, pref = 'auto') {
  let c = TIMES.get(list);
  if (!c || c.match !== match || c.pref !== pref || c.out.length - 1 > list.length) { c = { match, pref, out: [0] }; TIMES.set(list, c); }
  while (c.out.length - 1 < list.length) c.out.push(c.out[c.out.length - 1] + playSeconds(list[c.out.length - 1], match, pref));
  return c.out;
}
// Playback seconds now: when the current event started plus how long it has been on (capped at its length).
export function elapsedAt(times, index, since) {
  const n = times.length - 1, i = Math.max(0, Math.min(n, index)), t0 = times[Math.max(0, i - 1)], t1 = times[i];
  return { times, duration: times[n], elapsed: i >= n && index > 0 ? times[n] : i === 0 ? Math.min(times[0], Math.max(0, since || 0)) : Math.min(t1, t0 + Math.max(0, since || 0)) };
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
    this.camPref = 'auto'; this.camMode = 'tv'; this.camWho = 'auto'; this.cam3 = null; this.snap = true; this.fx = null;
    this.simDt = 0; this.clock = 0; this.hype = 0; this.hypeTeam = -1; this.slow = 1; this.shake = 0;
    this.stadium = null; this.cache = null; this.cacheKey = ''; this.lastKey = ''; this.board = null; this.boardKey = '';
    this.resize = this.resize.bind(this); window.addEventListener('resize', this.resize); this.resize();
    this.loop = this.loop.bind(this); requestAnimationFrame(this.loop);
    this.poll();
  }

  destroy() { this.stopped = true; window.removeEventListener('resize', this.resize); clearTimeout(this.pollTimer); }
  setSpeed(x) { this.speed = x; }
  togglePause() { this.paused = !this.paused; return this.paused; }
  skipToEnd() { this.idx = this.queue.length; this.cur = null; this.phase = 'wait'; this.banner = null; }
  // Salta a la jugada index (0 = des del principi): cada jugada porta el seu
  // marcador i la seva posició, així que n'hi ha prou de netejar el que queda de
  // l'anterior (celebració, repeticions, càmera) i deixar que es jugui aquesta.
  // Fora del que ja ha arribat, es queda a la vora (en directe, esperant).
  seek(index) {
    if (!this.match) return false;
    this.idx = clamp(Math.round(+index || 0), 0, this.queue.length);
    this.cur = null; this.phase = 'wait'; this.t = 0; this.hold = 0; this.banner = null; this.particles = [];
    this.since = this.idx > 0 ? Infinity : 0; // la barra: a l'inici de la jugada on s'aterra
    this.hype = 0; this.hypeTeam = -1; this.slow = 1; this.shake = 0; this.fx = null; this.cam3 = null; this.snap = true;
    if (this.camPref === 'auto') { this.camMode = 'tv'; this.camWho = 'auto'; }
    this.onUpdate(this.match, this.shown());
    return true;
  }
  // On som: index = jugades començades, total = les que han arribat.
  getProgress() {
    const done = !!this.match && ['final', 'stopped', 'failed'].includes(this.match.status);
    const times = this.match && this.match.teams ? playTimes(this.queue, this.match, this.camPref) : this.queue.map((_, i) => i * 6).concat(this.queue.length * 6);
    return { index: this.idx, total: this.queue.length, live: !!this.match && !done, events: this.queue, marks: progressMarks(this.queue), chapters: [], step: 3, ...elapsedAt(times, this.idx, this.since) };
  }
  // Ja s'ha vist tot (la retransmissió passa al partit següent).
  isFinished() { return !!this.match && ['final', 'stopped', 'failed'].includes(this.match.status) && this.idx >= this.queue.length && !this.cur; }
  // Càmera: 'auto' (el realitzador), o fixa: 'tv', 'sky', 'player', 'endzone', 'pylon', 'rail', 'blimp'.
  setCamera(mode) {
    this.camPref = mode in CAM_TAGS ? mode : 'auto';
    if (this.cur) {
      this.cur.plan = directPlay(this.cur.play, this.cur.anim, this.camPref); this.cur.seg = -1;
      if (this.phase !== 'replay') this.followLive();
    } else if (this.camPref !== 'auto') this.setCam(this.camPref);
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
    this.cacheKey = '';
  }

  proj(x, y, z = 0) { return project(this.view, x, y, z); }

  loop(now) {
    if (this.stopped) return;
    const dt = Math.min(.25, (now - this.last) / 1000); this.last = now;
    this.clock += dt; this.hype = Math.max(0, this.hype - dt * .16);
    this.simDt = this.paused ? 0 : dt * this.speed;
    if (!this.paused) this.advance(this.simDt);
    this.draw();
    requestAnimationFrame(this.loop);
  }

  // Canvi de càmera: tall sec amb un flaix, un escombrat ràpid (whip) o la
  // cortina de la repetició.
  setCam(mode, kind = 'cut', who = 'auto') {
    const changed = mode !== this.camMode || who !== this.camWho;
    if (changed || kind === 'replay') this.fx = { t0: performance.now(), kind: mode === this.camMode && kind === 'cut' ? 'whip' : kind };
    // La tele llisca d'una jugada a l'altra; la resta, tall sec.
    if (changed || mode !== 'tv' || kind === 'replay') this.snap = true;
    this.camMode = mode; this.camWho = who;
  }
  // Posa la càmera del tram de directe que toca.
  followLive() {
    const c = this.cur, i = liveSeg(c.plan, this.t);
    if (i === c.seg) return;
    const first = c.seg < 0;
    c.seg = i;
    const s = c.plan.live[i];
    if (first && s.cam === this.camMode && s.who === this.camWho) { this.snap = true; return; }
    this.setCam(s.cam, 'cut', s.who);
  }

  advance(dt) {
    if (!this.match) return;
    this.since = (this.since || 0) + dt;
    if (!this.cur) {
      if (this.idx < this.queue.length) {
        const p = this.queue[this.idx++];
        const anim = buildPlay(p, this.match);
        this.cur = { play: p, anim, plan: directPlay(p, anim, this.camPref), seg: -1, rep: -1, replaysDone: false }; this.since = 0;
        this.t = 0; this.phase = 'pre'; this.hold = .7;
        this.followLive();
        this.onUpdate(this.match, p);
        this.onEvent(p);
      }
      return;
    }
    const c = this.cur, p = c.play;
    const restHold = () => (p.comment ? 3.2 : 1.6) + (p.hl ? 1 : 0);
    this.slow = 1; this.shake = 0;
    if (this.phase === 'pre') { this.hold -= dt; if (this.hold <= 0) this.phase = 'run'; return; }
    if (this.phase === 'run') {
      const close = this.camMode === 'player' || this.camMode === 'rail';
      this.slow = close ? slowmo(c.anim, p, this.t) : 1;
      this.shake = close ? contact(c.anim, p, this.t) : bigHit(c.anim, this.t) * .45;
      this.t += dt * this.slow / c.anim.dur;
      if (this.t >= 1) {
        this.t = 1; this.phase = 'post'; this.postAge = 0;
        this.hold = c.plan.replays.length ? 1.4 : restHold();
        if (!c.plan.replays.length) c.replaysDone = true;
        this.celebrate(p);
      } else this.followLive();
      return;
    }
    if (this.phase === 'replay') {
      const r = c.plan.replays[c.rep];
      this.slow = r.rate; this.shake = r.cam === 'player' ? contact(c.anim, p, this.t) * .6 : bigHit(c.anim, this.t) * .3;
      this.t += dt * r.rate / c.anim.dur;
      if (this.t >= 1) {
        this.t = 1;
        if (c.rep + 1 < c.plan.replays.length) this.startReplay(c.rep + 1);
        else { this.phase = 'post'; this.postAge = 0; this.hold = restHold(); c.replaysDone = true; }
      }
      return;
    }
    if (this.phase === 'post') {
      this.hold -= dt; this.postAge = (this.postAge || 0) + dt;
      if (this.hold > 0) return;
      if (!c.replaysDone) { this.startReplay(0); return; }
      this.cur = null; this.banner = null;
    }
  }

  startReplay(i) {
    const c = this.cur, r = c.plan.replays[i];
    c.rep = i; this.phase = 'replay'; this.t = r.from;
    this.setCam(r.cam, 'replay', r.who || 'auto');
  }

  celebrate(p) {
    const T = this.T;
    const team = this.match.teams[p.td && p.int ? 1 - p.off : p.off];
    // L'afició: salta la de l'equip que celebra.
    if (p.td) { this.hype = 1; this.hypeTeam = p.int ? 1 - p.off : p.off; }
    else if (p.int || p.fumble || p.safety) { this.hype = .85; this.hypeTeam = 1 - p.off; }
    else if (p.kind === 'fg' && p.good) { this.hype = .7; this.hypeTeam = p.off; }
    else if (isBig(p)) { this.hype = .55; this.hypeTeam = p.kind === 'sack' ? 1 - p.off : p.off; }
    let text = '';
    if (p.td) text = T.td; else if ((p.kind === 'fg') && p.good) text = T.fg; else if (p.int) text = T.int; else if (p.fumble) text = T.fumble; else if (p.safety) text = T.safety; else if (p.kind === 'sack' && p.hl) text = T.sack;
    if (!text) return;
    const scorer = p.td ? this.match.teams[(p.int ? 1 - p.off : p.off)] : (p.int || p.fumble || p.safety ? this.match.teams[1 - p.off] : team);
    this.banner = { text, color: scorer.color, color2: scorer.color2, t0: performance.now(), td: !!p.td };
    if (p.td || (p.kind === 'fg' && p.good)) {
      const b = this.cur.anim.ball(1) || { x: this.cur.anim.endX, y: C, z: 1 };
      for (let i = 0; i < 160; i++) this.particles.push({ x: b.x, y: b.y, z: 2, vx: (Math.random() - .5) * 18, vy: (Math.random() - .5) * 18, vz: 8 + Math.random() * 14, c: Math.random() < .5 ? scorer.color : scorer.color2, life: 2 + Math.random() });
    }
  }

  // Càmera 3D (qualsevol menys la tele de banda).
  is3D() { return this.camMode !== 'tv' && !!this.cam3; }
  isSky() { return this.camMode === 'sky' && !!this.cam3; }

  // Mou les càmeres: la de tele llisca amb la pilota; la resta va cap al pla
  // que toca amb una mica de retard (més viva la de jugador), amb balanceig
  // del cap quan corre i la skycam inclinant-se quan es desplaça de costat.
  updateCamera(a, b) {
    const W = this.W, H = this.H, dt = this.simDt;
    const focusX = b ? b.x : (this.cur ? this.cur.anim.los : this.camX);
    if (this.camMode === 'tv' && this.snap) this.camX = clamp(focusX, 14, 106);
    else this.camX = lerp(this.camX, clamp(focusX, 14, 106), .06);
    if (this.camMode !== 'tv' && a) {
      const shot = shotFor(this.camMode, a, this.cur.play, this.t, this.camWho);
      if (this.snap || !this.cam3) this.cam3 = { P: shot.P.slice(), L: shot.L.slice(), zoom: shot.zoom, roll: 0, speed: shot.speed || 0 };
      else if (dt > 0) {
        const s = this.cam3, kP = 1 - Math.exp(-dt * shot.kP), kL = 1 - Math.exp(-dt * shot.kL);
        const old = s.P.slice();
        for (let i = 0; i < 3; i++) { s.P[i] = lerp(s.P[i], shot.P[i], kP); s.L[i] = lerp(s.L[i], shot.L[i], kL); }
        s.zoom = lerp(s.zoom, shot.zoom, kP);
        s.speed = lerp(s.speed, shot.speed || 0, kP);
        let rollTo = 0;
        if (this.camMode === 'sky') { const v = this.view && this.view.r ? this.view : makeView(s.P, s.L, 1, 0, 0); rollTo = clamp(((s.P[0] - old[0]) * v.r[0] + (s.P[1] - old[1]) * v.r[1]) / dt * .01, -.045, .045); }
        s.roll = lerp(s.roll, rollTo, 1 - Math.exp(-dt * 2.5));
      }
    }
    // Partit acabat o esperant jugades: tornem a la tele.
    if (!this.cur && this.idx >= this.queue.length) this.camMode = 'tv';
    this.snap = false;
    if (this.is3D()) {
      const s = this.cam3, P = s.P.slice();
      // Balanceig del cap a la càmera de jugador quan corre.
      if (this.camMode === 'player') P[2] += Math.sin(this.clock * 13) * .07 * clamp(s.speed / 7, 0, 1);
      const base = this.camMode === 'player' ? Math.max(.5 * W, .42 * H) : Math.max(.68 * W, .5 * H);
      const cy = this.camMode === 'player' ? H * (W < H ? .42 : .5) : H * (W < H ? .36 : .5);
      this.view = makeView(P, s.L, base * s.zoom, W / 2, cy, s.roll);
    } else this.view = tvView(this.camX, this.focal, this.camH, W, this.horizon);
  }

  draw() {
    const g = this.ctx, W = this.W, H = this.H, d = this.dpr;
    if (!this.match) { g.fillStyle = '#05070f'; g.fillRect(0, 0, W, H); this.drawCenterText(this.T.pregame); return; }
    if (!this.stadium || this.stadium.key !== this.match.teams[0].color + this.match.teams[1].color) { this.stadium = makeStadium(this.match.teams); this.cacheKey = ''; }
    const a = this.cur && this.cur.anim;
    // Temps que es veu: amb l'aturada curta (hit-stop) dels placatges.
    this.tt = a ? hitStop(a, this.t) : this.t;
    const b = a && a.ball(this.tt);
    this.updateCamera(a, b);
    const v = this.view;
    g.save();
    if (this.shake) g.translate(Math.sin(this.clock * 61) * this.shake * 9 * d, Math.cos(this.clock * 47) * this.shake * 6 * d);
    this.drawStatic();
    g.save();
    if (v.roll) { g.translate(v.cx, v.cy); g.rotate(v.roll); g.translate(-v.cx, -v.cy); }
    this.drawCrowd();
    if (this.camMode !== 'tv') this.drawBoards();
    this.drawPlay(a, b);
    this.drawParticles();
    g.restore();
    g.restore();
    if (this.is3D()) {
      // Vinyeta: es pinta un cop per mida i després només es copia.
      if (!this.vig || this.vig.width !== W || this.vig.height !== H) {
        this.vig = document.createElement('canvas'); this.vig.width = W; this.vig.height = H;
        const vc = this.vig.getContext('2d'), vg = vc.createRadialGradient(W / 2, H * .55, Math.min(W, H) * .35, W / 2, H * .55, Math.max(W, H) * .75);
        vg.addColorStop(0, 'rgba(0,0,0,0)'); vg.addColorStop(1, 'rgba(0,0,0,.42)');
        vc.fillStyle = vg; vc.fillRect(0, 0, W, H);
      }
      g.drawImage(this.vig, 0, 0);
    }
    this.drawSpeed();
    this.drawCutFx();
    if (this.slow < 1) { g.fillStyle = 'rgba(0,0,0,.55)'; g.fillRect(0, H - 10 * d, W, 10 * d); }
    this.drawHUD();
    this.drawCamTag();
  }

  // Capa quieta (cel, estadi, camp): si la càmera no s'ha mogut des de l'últim
  // fotograma, es fa servir la còpia desada en lloc de tornar-la a pintar.
  drawStatic() {
    const v = this.view, W = this.W, H = this.H;
    const r3 = x => x.toFixed(3);
    const key = [this.camMode, v.P.map(r3), v.f.map(r3), r3(v.focal), r3(v.roll), W, H, this.cur ? this.cur.play.n : 0].join('|');
    const still = key === this.lastKey;
    this.lastKey = key;
    const g = this.ctx;
    if (still && this.cacheKey === key) { g.drawImage(this.cache, 0, 0); return; }
    let target = g;
    if (still && typeof document !== 'undefined') {
      if (!this.cache || this.cache.width !== W || this.cache.height !== H) { this.cache = document.createElement('canvas'); this.cache.width = W; this.cache.height = H; }
      target = this.cache.getContext('2d');
      target.setTransform(1, 0, 0, 1, 0, 0);
    }
    this.ctx = target;
    target.save();
    if (v.roll) { target.translate(v.cx, v.cy); target.rotate(v.roll); target.translate(-v.cx, -v.cy); }
    this.drawSky();
    this.drawStadium();
    this.drawField();
    target.restore();
    this.ctx = g;
    if (target !== g) { this.cacheKey = key; g.drawImage(this.cache, 0, 0); }
  }

  // Cel de nit amb estrelles i una boirina clara a l'horitzó.
  drawSky() {
    const g = this.ctx, v = this.view, W = this.W, H = this.H, d = this.dpr;
    // L'horitzó (on van a parar els punts llunyans a terra).
    const hz = project(v, v.P[0] + v.f[0] * 1e4, v.P[1] + v.f[1] * 1e4, v.P[2]).y;
    const sky = g.createLinearGradient(0, Math.min(hz, H) - H, 0, Math.min(hz, H));
    sky.addColorStop(0, '#03050c'); sky.addColorStop(.75, '#0d1430'); sky.addColorStop(1, '#2a3156');
    g.fillStyle = sky; g.fillRect(-W * .1, -H * .1, W * 1.2, H * 1.2);
    g.fillStyle = 'rgba(255,255,255,.6)';
    for (const [x, y, z] of this.stadium.stars) {
      const X = x * v.r[0] + y * v.r[1] + z * v.r[2], Y = x * v.u[0] + y * v.u[1] + z * v.u[2], Z = x * v.f[0] + y * v.f[1] + z * v.f[2];
      if (Z < .05) continue;
      const sx = v.cx + v.focal * X / Z, sy = v.cy - v.focal * Y / Z;
      if (sx < 0 || sx > W || sy < 0 || sy > H) continue;
      g.fillRect(sx, sy, 1.4 * d, 1.4 * d);
    }
  }

  // El bol de graderies, les torres de llum i el terra de fora del camp.
  drawStadium() {
    const g = this.ctx, v = this.view, W = this.W, tv = this.camMode === 'tv';
    this.poly([[-90, -70, 0], [210, -70, 0], [210, FIELD_W + 70, 0], [-90, FIELD_W + 70, 0]], '#14301c');
    // Torres (darrere de tot): pal i focus.
    const glows = [];
    for (const th of TOWERS) {
      const [x, y] = ring(th, 64), [nx, ny] = [Math.cos(th), Math.sin(th)];
      if (tv && y > FIELD_W) continue;
      const tx = -ny, ty = nx;
      this.poly([[x - tx * .7, y - ty * .7, 0], [x + tx * .7, y + ty * .7, 0], [x + tx * .5, y + ty * .5, 56], [x - tx * .5, y - ty * .5, 56]], '#0b0f1c');
      this.poly([[x - tx * 7, y - ty * 7, 54], [x + tx * 7, y + ty * 7, 54], [x + tx * 7, y + ty * 7, 62], [x - tx * 7, y - ty * 7, 62]], '#fffbe8');
      const p = this.proj(x, y, 58);
      if (p.d > NEAR) glows.push(p);
    }
    const list = [];
    for (const q of this.stadium.polys) {
      if (tv && q.cen[1] > FIELD_W + 8) continue;
      const dd = camSpace(v, q.cen[0], q.cen[1], q.cen[2])[2];
      if (dd < -40) continue;
      list.push([dd + q.bias, q]);
    }
    list.sort((p, q) => q[0] - p[0]);
    for (const [, q] of list) this.poly(q.pts, q.fill);
    // Resplendor dels focus i boirina.
    for (const p of glows) {
      const r = clamp(30 * p.s, 30 * this.dpr, W * .25);
      const lg = g.createRadialGradient(p.x, p.y, 0, p.x, p.y, r);
      lg.addColorStop(0, 'rgba(255,252,230,.95)'); lg.addColorStop(.06, 'rgba(255,250,225,.5)'); lg.addColorStop(.35, 'rgba(200,210,255,.12)'); lg.addColorStop(1, 'rgba(200,210,255,0)');
      g.fillStyle = lg; g.fillRect(p.x - r, p.y - r, 2 * r, 2 * r);
    }
  }

  // El públic: punts i ninots amb els colors dels equips, que es balancegen,
  // fan l'onada de tant en tant i salten, aixequen els braços, onegen
  // banderes i disparen flaixos quan el seu equip marca.
  drawCrowd() {
    const g = this.ctx, v = this.view, W = this.W, H = this.H, d = this.dpr, st = this.stadium;
    const tv = this.camMode === 'tv', ck = this.clock, hype = this.hype, ht = this.hypeTeam;
    const Px = v.P[0], Py = v.P[1], Pz = v.P[2], r = v.r, u = v.u, f = v.f, F = v.focal, cx = v.cx, cy = v.cy;
    const wt = ck % 40, wave = wt < 13, wth = wt / 13 * Math.PI * 3;
    const heads = [], flags = [];
    let ci = -1;
    for (const c of st.people) {
      if (tv && c.y > FIELD_W + 8) continue;
      const ex = hype * (c.team === ht ? 1 : c.team < 0 ? .4 : .08);
      let up = Math.sin(ck * 1.6 + c.ph) * .05 + ex * Math.abs(Math.sin(ck * 8.5 + c.ph * 3)) * .55, arms = ex > .3;
      if (wave) { const dth = Math.abs((((c.th - wth) % 6.2832) + 9.4248) % 6.2832 - 3.1416); const w = Math.exp(-dth * dth * 30); up += w * .7; if (w > .35) arms = true; }
      const dx = c.x - Px, dy = c.y - Py, dz = c.z + up - Pz;
      const Z = dx * f[0] + dy * f[1] + dz * f[2];
      if (Z < 2) continue;
      const s = F / Z;
      // De lluny n'hi ha prou amb la meitat (la grada ja té el color de l'afició).
      if (c.lod && s < 4 * d) continue;
      const x = cx + (dx * r[0] + dy * r[1]) * s, y = cy - (dx * u[0] + dy * u[1] + dz * u[2]) * s;
      if (x < -20 || x > W + 20 || y < -20 || y > H + 20) continue;
      if (c.ci !== ci) { ci = c.ci; g.fillStyle = st.pal[ci]; }
      const bw = Math.max(.5 * s, 1.1 * d), bh = Math.max(.66 * s, 1.3 * d);
      g.fillRect(x - bw / 2, y - bh, bw, bh);
      if (bw > 3 * d) {
        heads.push(x, y - bh - .14 * s, .26 * s);
        // De prop, els veïns de seient (més gent sense més càlculs).
        const sx = .58 * s, sy = (Math.sin(c.ph * 7) * .08) * s;
        g.fillRect(x - sx - bw / 2, y - bh + sy, bw, bh); g.fillRect(x + sx - bw / 2, y - bh - sy, bw, bh);
        heads.push(x - sx, y - bh - .14 * s + sy, .26 * s, x + sx, y - bh - .14 * s - sy, .26 * s);
        if (arms) { g.fillRect(x - bw / 2 - .05 * s, y - bh - .42 * s, .1 * s, .45 * s); g.fillRect(x + bw / 2 - .05 * s, y - bh - .42 * s, .1 * s, .45 * s); }
      }
      if (c.flag && bw > 1.5 * d) flags.push(x, y - bh, s, c.ph, c.team);
    }
    g.fillStyle = '#c99f7e';
    for (let i = 0; i < heads.length; i += 3) g.fillRect(heads[i] - heads[i + 2] / 2, heads[i + 1] - heads[i + 2] / 2, heads[i + 2], heads[i + 2]);
    const teams = this.match.teams;
    for (let i = 0; i < flags.length; i += 5) {
      const [x, y, s, ph, tm] = [flags[i], flags[i + 1], flags[i + 2], flags[i + 3], flags[i + 4]];
      const team = teams[tm < 0 ? 0 : tm], top = y - 1.3 * s, wav = Math.sin(ck * 6 + ph) * (.15 + hype * .3) * s;
      g.fillStyle = '#ddd'; g.fillRect(x, top, Math.max(1, .05 * s), 1.3 * s);
      g.fillStyle = team.color; g.beginPath(); g.moveTo(x, top); g.lineTo(x + .8 * s, top + wav * .5); g.lineTo(x + .8 * s, top + .5 * s + wav); g.lineTo(x, top + .5 * s); g.closePath(); g.fill();
    }
    // Flaixos de càmeres: sempre algun, i una pluja quan hi ha gresca.
    const n = Math.floor(hype * 18) + (Math.random() < .25 ? 1 : 0);
    for (let k = 0; k < n; k++) {
      const c = st.people[Math.floor(Math.random() * st.people.length)];
      if (tv && c.y > FIELD_W + 8) continue;
      const p = this.proj(c.x, c.y, c.z + .6);
      if (p.d < 2 || p.x < 0 || p.x > W || p.y < 0 || p.y > H) continue;
      const rr = clamp(.9 * p.s, 3 * d, 18 * d);
      const lg = g.createRadialGradient(p.x, p.y, 0, p.x, p.y, rr);
      lg.addColorStop(0, 'rgba(255,255,255,1)'); lg.addColorStop(1, 'rgba(255,255,255,0)');
      g.fillStyle = lg; g.fillRect(p.x - rr, p.y - rr, 2 * rr, 2 * rr);
    }
  }

  // Videomarcadors penjats sobre les dues zones de marca.
  boardCanvas() {
    const m = this.match, T = this.T, p = this.cur ? this.cur.play : this.shown(), sc = this.score();
    const bn = this.banner, flash = bn ? Math.floor(performance.now() / 280) % 2 : 0;
    const key = bn ? 'B' + bn.text + flash : [sc, p && p.q, p && p.clock, p && p.down, p && p.dist].join(',');
    if (key === this.boardKey && this.board) return this.board;
    this.boardKey = key;
    if (!this.board) { this.board = document.createElement('canvas'); this.board.width = 384; this.board.height = 176; }
    const g = this.board.getContext('2d'), w = 384, h = 176;
    g.fillStyle = '#04060c'; g.fillRect(0, 0, w, h);
    g.textAlign = 'center'; g.textBaseline = 'middle';
    if (bn) {
      g.fillStyle = flash ? bn.color : (bn.color2 || '#fff'); g.fillRect(6, 6, w - 12, h - 12);
      g.fillStyle = flash ? (bn.color2 || '#fff') : bn.color; g.font = '900 italic 54px "Segoe UI", system-ui, sans-serif';
      g.fillText(bn.text.replace(/!$/, ''), w / 2, h / 2, w - 30);
    } else {
      [0, 1].forEach(i => {
        const t = m.teams[i], x = i ? w * .75 : w * .25;
        g.fillStyle = t.color; g.fillRect(i ? w / 2 + 6 : 6, 6, w / 2 - 12, 34);
        g.fillStyle = luminance(t.color) > .6 ? '#111' : '#fff'; g.font = '800 24px "Segoe UI", system-ui, sans-serif'; g.fillText(t.abbr, x, 24);
        g.fillStyle = '#ffd400'; g.font = '900 72px "Segoe UI", system-ui, sans-serif'; g.fillText(String(sc ? sc[i] : 0), x, 96);
      });
      g.fillStyle = '#fff'; g.font = '700 20px "Segoe UI", system-ui, sans-serif';
      const q = p ? `${T.q[Math.min(4, p.q - 1)]}  ${Math.floor(p.clock / 60)}:${String(p.clock % 60).padStart(2, '0')}` : '';
      g.fillText(q, w / 2, 150);
    }
    return this.board;
  }

  drawBoards() {
    const g = this.ctx, img = this.boardCanvas();
    for (const [x, sgn] of [[-20, 1], [140, -1]]) {
      // Qui el mira des del camp té la dreta cap a -y (pantalla a x<0) o +y.
      const y0 = sgn > 0 ? C + 15 : C - 15, y1 = sgn > 0 ? C - 15 : C + 15;
      const zb = 13, zt = 26.5;
      const TL = this.proj(x, y0, zt), TR = this.proj(x, y1, zt), BL = this.proj(x, y0, zb), BR = this.proj(x, y1, zb);
      if (Math.min(TL.d, TR.d, BL.d, BR.d) < 3) continue;
      // Que es vegi de cara (no d'esquena).
      if ((TR.x - TL.x) * (BL.y - TL.y) - (TR.y - TL.y) * (BL.x - TL.x) <= 0) continue;
      this.poly([[x, y0 + sgn * .8, zb - .8], [x, y1 - sgn * .8, zb - .8], [x, y1 - sgn * .8, zt + .8], [x, y0 + sgn * .8, zt + .8]], '#1c2233');
      g.strokeStyle = '#3a4258'; g.lineWidth = Math.max(1, .15 * TL.s);
      g.beginPath(); g.moveTo(TL.x, TL.y); const c1 = this.proj(x, y0, 47); g.lineTo(c1.x, c1.y); g.moveTo(TR.x, TR.y); const c2 = this.proj(x, y1, 47); g.lineTo(c2.x, c2.y); g.stroke();
      g.save(); g.transform((TR.x - TL.x) / img.width, (TR.y - TL.y) / img.width, (BL.x - TL.x) / img.height, (BL.y - TL.y) / img.height, TL.x, TL.y);
      g.drawImage(img, 0, 0); g.restore();
    }
  }

  // Polígon 3D retallat pel pla de davant de la càmera (si no, el que queda
  // darrere es dibuixaria del revés). Amb fill null només afegeix el polígon
  // al camí actual (per pintar-ne molts d'un cop).
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
    if (this.is3D()) {
      // A les càmeres 3D les ratlles són tires de gespa pintada (fan perspectiva).
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
    if (this.is3D()) {
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

  circle(x, y, r, fill) { const pts = []; for (let i = 0; i < 28; i++) { const a = i / 28 * Math.PI * 2; pts.push([x + Math.cos(a) * r, y + Math.sin(a) * r]); } this.poly(pts, fill); }

  drawField() {
    const m = this.match, s3 = this.is3D();
    const q = (this.cur && this.cur.play.q) || m.quarter || 1;
    this.quad(-12, -5, 132, FIELD_W + 5, '#2f6b33');
    for (let x = 10; x < 110; x += 5) this.quad(x, 0, x + 5, FIELD_W, (x / 5) % 2 ? '#3d8a3f' : '#357d38');
    // Zones de marca amb els colors de qui les defensa.
    const leftTeam = m.teams[attackDir(0, q) > 0 ? 0 : 1];
    const rightTeam = m.teams[attackDir(0, q) > 0 ? 1 : 0];
    this.quad(0, 0, 10, FIELD_W, shade(leftTeam.color, -.15));
    this.quad(110, 0, 120, FIELD_W, shade(rightTeam.color, -.15));
    // Ratlles en diagonal a les zones de marca.
    for (const [x0, tm] of [[0, leftTeam], [110, rightTeam]]) {
      const g = this.ctx; g.beginPath();
      for (let y = 2; y < FIELD_W - 6; y += 6) this.poly([[x0 + .6, y], [x0 + .6, y + 1.2], [x0 + 9.4, y + 5.2], [x0 + 9.4, y + 4]], null);
      g.fillStyle = shade(tm.color, -.28); g.fill();
    }
    // A les càmeres 3D el nom es llegeix de cara des del camp, d'una banda a l'altra.
    this.groundText(leftTeam.name.toUpperCase(), 5, C, s3 ? 4.4 : 3.2, leftTeam.color2 || '#fff', false, [0, -1]);
    this.groundText(rightTeam.name.toUpperCase(), 115, C, s3 ? 4.4 : 3.2, rightTeam.color2 || '#fff', false, [0, 1]);
    // Línies.
    for (let x = 10; x <= 110; x += 5) this.line(x, 0, x, FIELD_W, 'rgba(255,255,255,.85)', x % 10 === 0 ? 1.4 : 1);
    this.line(0, 0, 120, 0, '#fff', 2); this.line(0, FIELD_W, 120, FIELD_W, '#fff', 2);
    this.line(0, 0, 0, FIELD_W, '#fff', 2); this.line(120, 0, 120, FIELD_W, '#fff', 2);
    if (s3) {
      // Marques de iarda: totes en un sol camí.
      const g = this.ctx; g.beginPath();
      for (let x = 11; x < 110; x++) { if (x % 5 === 0) continue; for (const [y0, y1] of [[22.9, 23.6], [29.7, 30.4], [1, 1.6], [FIELD_W - 1.6, FIELD_W - 1]]) this.poly([[x - .05, y0], [x + .05, y0], [x + .05, y1], [x - .05, y1]], null); }
      g.fillStyle = 'rgba(255,255,255,.7)'; g.fill();
    } else for (let x = 11; x < 110; x++) { if (x % 5 === 0) continue; this.line(x, 22.9, x, 23.6, 'rgba(255,255,255,.7)', .8); this.line(x, 29.7, x, 30.4, 'rgba(255,255,255,.7)', .8); this.line(x, 1, x, 1.6, 'rgba(255,255,255,.7)', .8); this.line(x, FIELD_W - 1.6, x, FIELD_W - 1, 'rgba(255,255,255,.7)', .8); }
    for (let x = 20; x <= 100; x += 10) { const n = String(x <= 60 ? x - 10 : 110 - x); this.groundText(n, x, 9, 2.6, 'rgba(255,255,255,.9)', true); this.groundText(n, x, FIELD_W - 9, 2.6, 'rgba(255,255,255,.9)'); }
    // Logo del mig del camp: cercle amb els colors dels dos equips.
    this.circle(60, C, 5.2, 'rgba(255,255,255,.55)');
    this.circle(60, C, 4.7, shade(m.teams[0].color, -.2));
    this.poly(Array.from({ length: 15 }, (_, i) => { const a = -Math.PI / 2 + i / 14 * Math.PI; return [60 + Math.cos(a) * 4.7, C + Math.sin(a) * 4.7]; }), shade(m.teams[1].color, -.2));
    this.groundText('VENTS', 60, C, 2.6, 'rgba(255,255,255,.85)');
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
    if (Math.min(base.d, l.d, r.d, lt.d, rt.d) < NEAR + .5) return;
    g.strokeStyle = '#f4d03f'; g.lineCap = 'round';
    g.lineWidth = .35 * base.s; g.beginPath(); g.moveTo(base.x, base.y); g.lineTo(bar.x, bar.y); g.stroke();
    g.lineWidth = .25 * base.s; g.beginPath(); g.moveTo(l.x, l.y); g.lineTo(r.x, r.y); g.moveTo(l.x, l.y); g.lineTo(lt.x, lt.y); g.moveTo(r.x, r.y); g.lineTo(rt.x, rt.y); g.stroke();
  }

  // Pal taronja (pylon) a les cantonades de les zones de marca.
  drawPylon(x, y) {
    const g = this.ctx, b = this.proj(x, y), t = this.proj(x, y, 1.2);
    if (b.d < NEAR + .3) return;
    const w = Math.max(1.5, .14 * b.s);
    g.fillStyle = '#ff7a12'; g.fillRect(b.x - w / 2, t.y, w, b.y - t.y);
    g.fillStyle = '#fff'; g.fillRect(b.x - w / 2, lerp(t.y, b.y, .45), w, Math.max(1, (b.y - t.y) * .08));
  }

  drawBall(b) {
    const g = this.ctx, d = this.dpr;
    const sh = this.proj(b.x, b.y, 0), p = this.proj(b.x, b.y, b.z);
    if (p.d < NEAR + .3) return;
    // Ombra a terra: més petita i clara com més amunt va la pilota.
    const flat = this.is3D() ? clamp(this.view.P[2] / Math.max(1, sh.d), .15, .8) : .4, hk = clamp(1 - b.z / 14, .35, 1);
    if (sh.d > NEAR) { g.fillStyle = `rgba(0,0,0,${(.35 * hk).toFixed(3)})`; g.beginPath(); g.ellipse(sh.x, sh.y, .32 * sh.s * hk, .32 * flat * sh.s * hk, 0, 0, Math.PI * 2); g.fill(); }
    // Mida mínima perquè es vegi de lluny; orientada segons el seu eix (el vol, el gir, el bot).
    const up = Math.max(1, 6 * d / Math.max(p.s * .19, .01)), L = .3 * up, ax = b.axis || [1, 0, 0];
    const t1 = this.proj(b.x + ax[0] * L, b.y + ax[1] * L, b.z + ax[2] * L), t2 = this.proj(b.x - ax[0] * L, b.y - ax[1] * L, b.z - ax[2] * L);
    const minor = .19 * p.s * up, major = Math.max(minor * 1.06, Math.hypot(t1.x - t2.x, t1.y - t2.y) / 2), ang = Math.atan2(t1.y - t2.y, t1.x - t2.x);
    g.save(); g.translate(p.x, p.y); g.rotate(ang);
    g.fillStyle = '#7a3e14'; g.strokeStyle = '#4a240b'; g.lineWidth = Math.max(1, .02 * p.s * up);
    g.beginPath(); g.ellipse(0, 0, major, minor, 0, 0, Math.PI * 2); g.fill(); g.stroke();
    g.fillStyle = 'rgba(255,220,180,.25)'; g.beginPath(); g.ellipse(-major * .1, -minor * .35, major * .6, minor * .3, 0, 0, Math.PI * 2); g.fill();
    if (major > minor * 1.35) {
      // Ratlles blanques a prop de les puntes i les costures, que giren amb l'espiral.
      g.strokeStyle = 'rgba(255,255,255,.85)'; g.lineWidth = Math.max(1, .035 * p.s * up);
      const sx = major * .62, sy = minor * .82;
      g.beginPath(); g.moveTo(sx, -sy); g.lineTo(sx, sy); g.moveTo(-sx, -sy); g.lineTo(-sx, sy); g.stroke();
      const sp = b.spin || 0, off = Math.sin(sp) * minor * .5;
      if (Math.cos(sp) > -.2) { g.beginPath(); g.moveTo(-major * .3, off); g.lineTo(major * .3, off); g.stroke(); }
    }
    g.restore();
  }

  // Jugadors, banquetes, pals, pylons i pilota, ordenats de lluny a prop.
  drawPlay(a, b) {
    const t = this.tt, s3 = this.is3D(), v = this.view;
    const depth = (x, y, z = 1) => camSpace(v, x, y, z)[2];
    const all = [];
    const near = ['player', 'pylon', 'rail'].includes(this.camMode) ? .8 : 2;
    if (a) {
      const items = a.tracks.map((tr, i) => { const pos = tr.pos(t); return { tr, i, pos, d: depth(pos.x, pos.y) }; });
      for (const it of items) all.push({ d: it.d, i: it.i, draw: () => { if (it.d > near) this.drawPlayer(it.tr, it.pos, a, it.i, b); } });
      if (b) {
        let bd = s3 ? depth(b.x, b.y, b.z) : FIELD_W + 30 - b.y;
        // A la mà: just per sobre de qui la porta si es veu de costat; si no, per profunditat.
        if (b.holder >= 0) { const h = items[b.holder], f = this.facingOf(b.holder, a.headingOf(b.holder, t)); if (!s3 || f.front === null) bd = h.d - 1e-3; }
        all.push({ d: bd, i: 1e4, draw: () => this.drawBall(b) });
      }
    }
    for (const x of [0, 120]) all.push({ d: depth(x, C, 0), i: -1, draw: () => this.drawPosts(x) });
    for (const x of [0, 10, 110, 120]) for (const y of [0, FIELD_W]) all.push({ d: depth(x, y, 0), i: -2, draw: () => this.drawPylon(x, y) });
    for (const bp of this.stadium.bench) { const dd = depth(bp.x, bp.y); if (dd > 2) all.push({ d: dd, i: -3, draw: () => this.drawBencher(bp) }); }
    // Les banquetes: un banc llarg de cada equip.
    for (const team of [0, 1]) { const y = team === 0 ? -10.2 : FIELD_W + 10.2; all.push({ d: depth(60, y, .4), i: -4, draw: () => this.drawBench(team, y) }); }
    all.sort((p, q) => (q.d - p.d) || (p.i - q.i));
    all.forEach(e => e.draw());
    if (a) this.drawContactFx(a, t);
  }

  // Cop i gespa: un flaix al contacte i terra que salta quan cauen. Surt de la
  // jugada (llavor i t), així es veu igual a la repetició i a càmera lenta.
  drawContactFx(a, t) {
    const g = this.ctx, dur = a.dur;
    for (const f of a.fx) {
      const age = (t - f.t) * dur;
      if (age < 0 || age > .7) continue;
      if (f.kind === 'hit' && age < .16) {
        const p = this.proj(f.at.x, f.at.y, 1.2);
        if (p.d < NEAR + .5) continue;
        const r = Math.max(6 * this.dpr, (f.big ? 1.6 : 1.1) * p.s) * (.6 + age * 3), al = (1 - age / .16) * (f.big ? .85 : .6);
        const lg = g.createRadialGradient(p.x, p.y, 0, p.x, p.y, r);
        lg.addColorStop(0, `rgba(255,255,240,${al.toFixed(3)})`); lg.addColorStop(1, 'rgba(255,255,240,0)');
        g.fillStyle = lg; g.fillRect(p.x - r, p.y - r, 2 * r, 2 * r);
        // Rajos curts.
        g.strokeStyle = `rgba(255,255,255,${(al * .8).toFixed(3)})`; g.lineWidth = Math.max(1, .05 * p.s);
        g.beginPath();
        for (let k = 0; k < 7; k++) { const an = k * .9 + f.t * 50, r0 = r * .45, r1 = r * (.8 + (k % 3) * .15); g.moveTo(p.x + Math.cos(an) * r0, p.y + Math.sin(an) * r0); g.lineTo(p.x + Math.cos(an) * r1, p.y + Math.sin(an) * r1); }
        g.stroke();
      }
      if (f.kind === 'turf') {
        const R = rng(Math.round(f.t * 1e5) + 7), n = f.big ? 22 : 13;
        for (let k = 0; k < n; k++) {
          const ang = R() * Math.PI * 2, sp = 1 + R() * 3.5, vz = 2 + R() * 3.5, sz = .06 + R() * .1, grass = R() < .55;
          const z = vz * age - GRAV * age * age / 2;
          if (z < 0) continue;
          const p = this.proj(f.at.x + Math.cos(ang) * sp * age, f.at.y + Math.sin(ang) * sp * age, z + .05);
          if (p.d < NEAR + .5) continue;
          g.fillStyle = grass ? '#4f9a3c' : '#6b4a2b'; g.globalAlpha = clamp(1 - age / .7, 0, 1);
          const w = Math.max(1.2 * this.dpr, sz * p.s); g.fillRect(p.x - w / 2, p.y - w / 2, w, grass ? w * .5 : w);
        }
        g.globalAlpha = 1;
        // Polsim.
        const p = this.proj(f.at.x, f.at.y, .3);
        if (p.d > NEAR + .5 && age < .5) { const r = (1.2 + age * 3) * p.s, lg = g.createRadialGradient(p.x, p.y, 0, p.x, p.y, r); lg.addColorStop(0, `rgba(150,130,100,${(.35 * (1 - age / .5)).toFixed(3)})`); lg.addColorStop(1, 'rgba(150,130,100,0)'); g.fillStyle = lg; g.fillRect(p.x - r, p.y - r, 2 * r, 2 * r); }
      }
    }
  }

  drawBench(team, y) {
    const col = this.match.teams[team].color;
    this.poly([[37, y - .5, 0], [83, y - .5, 0], [83, y - .5, .55], [37, y - .5, .55]], shade(col, -.3));
    this.poly([[37, y - .5, .55], [83, y - .5, .55], [83, y + .5, .55], [37, y + .5, .55]], shade(col, -.1));
  }

  // Un reserva de peu a la banqueta, mirant el camp.
  drawBencher(bp) {
    const p = this.proj(bp.x, bp.y), s = p.s, team = this.match.teams[bp.team];
    if (p.d < 2 || p.x < -s * 2 || p.x > this.W + s * 2) return;
    const jersey = team.color, trim = team.color2 || '#fff';
    if (s < 4) { this.ctx.fillStyle = jersey; this.ctx.fillRect(p.x - s * .3, p.y - s * 1.8, s * .6, s * 1.8); return; }
    const toField = bp.team === 0 ? 1 : -1, front = toField * this.view.f[1] < 0;
    const hipY = p.y - .9 * s, shY = p.y - 1.55 * s, headY = p.y - 1.86 * s;
    this.drawPlayerFacing(p, s, { front, face: 1, sw: 0, moving: false, hipY, shY, headY, jersey, trim, pants: luminance(trim) > .55 ? trim : shade(jersey, .7), helmet: shade(jersey, -.08), helmetEdge: shade(jersey, -.3), skin: SKIN[bp.num % SKIN.length], pl: { number: bp.num, name: '' } });
  }

  // Com es veu un jugador segons cap on mira (hd: direcció al terra i velocitat):
  // face és el costat (±1) i front, a les càmeres 3D, de cara (true), d'esquena (false) o de costat (null).
  facingOf(i, hd) {
    const moving = (this.phase === 'run' || this.phase === 'replay') && hd[2] > .02;
    if (!this.is3D()) return { face: Math.abs(hd[0]) > .15 ? Math.sign(hd[0]) : (hd[0] >= 0 ? 1 : -1), front: null, moving };
    const v = this.view, fr = hd[0] * v.r[0] + hd[1] * v.r[1], ff = hd[0] * v.f[0] + hd[1] * v.f[1];
    return { face: fr >= 0 ? 1 : -1, front: Math.abs(fr) < 1.5 * Math.abs(ff) ? ff < 0 : null, moving };
  }

  // Un braç fins a un punt (estirat del tot si cal, per no deixar mai la
  // pilota penjant), amb el colze doblegat cap a fora.
  limb(S, T, s, upper, skin, cx) {
    const g = this.ctx, A = .62 * s, R = ARM_REACH * s;
    let dx = T.x - S.x, dy = T.y - S.y, d = Math.hypot(dx, dy) || 1e-6;
    if (d > R) { dx *= R / d; dy *= R / d; d = R; }
    const H = { x: S.x + dx, y: S.y + dy }, half = A * .5, bend = Math.sqrt(Math.max(0, half * half - (d / 2) ** 2)) * .7;
    let nx = -dy / d, ny = dx / d;
    if (cx !== undefined ? nx * (S.x - cx) < 0 : ny < 0) { nx = -nx; ny = -ny; }
    const E = { x: S.x + dx / 2 + nx * bend, y: S.y + dy / 2 + ny * bend };
    g.strokeStyle = upper; g.lineWidth = .16 * s; g.beginPath(); g.moveTo(S.x, S.y); g.lineTo(E.x, E.y); g.stroke();
    g.strokeStyle = skin; g.lineWidth = .12 * s; g.beginPath(); g.moveTo(E.x, E.y); g.lineTo(H.x, H.y); g.stroke();
    g.fillStyle = '#2a2a30'; g.beginPath(); g.arc(H.x, H.y, .075 * s, 0, Math.PI * 2); g.fill();
    return H;
  }

  // Un jugador de futbol americà segons la seva postura: corre, bloqueja,
  // llança, estira els braços cap a la pilota, la porta sota el braç, placa
  // (espatlla avall i braços al voltant) o cau amb el placatge i queda a terra.
  // A la tele es veu de costat; a les càmeres 3D, de costat, d'esquena o de cara.
  drawPlayer(tr, pos, a, i, ball) {
    const g = this.ctx, team = this.match.teams[tr.team], pl = tr.p, t = this.tt;
    const pose = a.poseOf(i, t), hd = a.headingOf(i, t);
    const ground = this.proj(pos.x, pos.y, 0), p = pose.jump ? this.proj(pos.x, pos.y, pose.jump) : ground, s = p.s;
    const jersey = team.color, trim = team.color2 || '#fff';
    const pants = luminance(trim) > .55 ? trim : shade(jersey, .7);
    const helmet = shade(jersey, -.08), helmetEdge = shade(jersey, -.3);
    const skin = SKIN[hashStr(pl.id || pl.name || '') % SKIN.length];
    const { face, front, moving } = this.facingOf(i, hd);
    const sky = this.is3D();
    g.lineCap = 'round'; g.lineJoin = 'round';
    // Caiguda (i, un cop acabada la jugada, s'aixequen).
    let fall = pose.fall ? pose.fall.k : 0;
    if (fall && this.phase === 'post' && this.t >= 1) fall *= 1 - ease(seg(this.postAge || 0, 1, 1.6));
    let ang = 0, squash = 1;
    if (fall > 0) {
      const q = this.proj(pos.x + pose.fall.dir[0], pos.y + pose.fall.dir[1], 0), sx = q.x - ground.x, sy = q.y - ground.y, n = Math.hypot(sx, sy) || 1;
      ang = fall * 1.45 * (sx / n); squash = 1 - fall * .72 * Math.abs(sy / n);
    }
    // Ombra (a les càmeres 3D el terra s'aixafa segons l'alçada de la càmera).
    const flat = sky ? clamp(this.view.P[2] / Math.max(1, ground.d), .15, .45) : .36;
    const shw = (.55 + fall * .5) * s;
    g.fillStyle = 'rgba(0,0,0,.32)'; g.beginPath(); g.ellipse(ground.x, ground.y, shw, Math.max(.2 * s * flat / .36, 1), 0, 0, Math.PI * 2); g.fill();
    if (s < 3) { g.fillStyle = jersey; g.fillRect(p.x - s * .3, p.y - s * 1.8 * (1 - fall * .7), s * .6, s * 1.8 * (1 - fall * .7)); return; }
    const isCarrier = ball && ball.holder === i;
    // Punts on van les mans, a la pantalla i en coordenades del ninot (si cau, girat).
    const ca = Math.cos(-ang), sa = Math.sin(-ang);
    const bodyD = camSpace(this.view, pos.x, pos.y, 1.3)[2];
    const local = pt => { const q = this.proj(pt[0], pt[1], pt[2]); const dx = q.x - p.x, dy = q.y - p.y; return { x: p.x + dx * ca - dy * sa, y: p.y + (dx * sa + dy * ca) / squash, near: q.d < bodyD }; };
    const targets = pose.arms ? pose.arms.map(pt => pt && local(pt)) : null;
    g.save();
    if (fall > 0) { g.translate(p.x, p.y); g.rotate(ang); g.scale(1, squash); g.translate(-p.x, -p.y); }
    const phase = (pos.x * 1.9 + pos.y * 1.3);
    const sw = moving && fall < .2 ? Math.sin(phase) : 0, bob = moving ? Math.abs(Math.cos(phase)) * .06 * s : 0;
    const bend = pose.lean * .24 * s + pose.crouch * .42 * s;
    const hipY = p.y - .9 * s - bob + pose.crouch * .32 * s, shY = p.y - 1.55 * s - bob + bend, headY = p.y - 1.86 * s - bob + bend * 1.05;
    if (front !== null) {
      this.drawPlayerFacing(p, s, { front, face, sw, moving, hipY, shY, headY, jersey, trim, pants, helmet, helmetEdge, skin, pl, targets, kick: pose.kick, crouch: pose.crouch });
      g.restore();
      this.drawLabel(pl, p, s, p.y - (2.36 - fall * 1.7) * s, isCarrier);
      return;
    }
    const lean = face * (pose.lean * .38 + (moving ? .1 : 0)) * s;
    // Cames: cuixa amb el pantaló, mitjó del color de l'equip, bota.
    for (const k of [-1, 1]) {
      const st = sw * k;
      const hip = { x: p.x + k * .1 * s, y: hipY };
      let foot = { x: p.x + k * .1 * s + face * st * .36 * s - (pose.crouch ? face * .25 * s * k : 0), y: p.y - Math.max(0, -st) * .14 * s };
      if (pose.kick && k === 1) foot = { x: hip.x + face * .75 * s * pose.kick, y: lerp(p.y, hipY - .1 * s, pose.kick) };
      const knee = { x: lerp(hip.x, foot.x, .5) + face * (.08 + Math.max(0, st) * .1 + pose.crouch * .2) * s, y: lerp(hip.y, foot.y, .5) };
      g.strokeStyle = pants; g.lineWidth = .2 * s; g.beginPath(); g.moveTo(hip.x, hip.y); g.lineTo(knee.x, knee.y); g.stroke();
      g.strokeStyle = shade(jersey, -.2); g.lineWidth = .15 * s; g.beginPath(); g.moveTo(knee.x, knee.y); g.lineTo(foot.x, foot.y); g.stroke();
      g.fillStyle = '#16161a'; g.beginPath(); g.ellipse(foot.x + face * .06 * s, foot.y, .13 * s, .06 * s, 0, 0, Math.PI * 2); g.fill();
    }
    // Braços: el del darrere abans del cos, el del davant després.
    const swing = k => { const sh = { x: p.x + lean + k * .32 * s, y: shY + .1 * s }; const hand = { x: sh.x - face * sw * k * .3 * s, y: shY + .68 * s }; this.limb(sh, hand, s, shade(jersey, -.1), skin); };
    const back = -face, frontArm = face;
    const tBack = targets ? (targets[1] || null) : null, tFront = targets ? (targets[0] || targets[1]) : null;
    const shoulder = k => ({ x: p.x + lean + k * .32 * s, y: shY + .1 * s });
    if (tBack) this.limb(shoulder(back), tBack, s, shade(jersey, -.25), shade(skin, -.12)); else swing(back);
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
    if (tFront) this.limb(shoulder(frontArm), tFront, s, shade(jersey, -.1), skin); else swing(frontArm);
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
    g.restore();
    this.drawLabel(pl, p, s, p.y - (2.36 - fall * 1.7) * s, isCarrier);
  }

  // El mateix ninot vist de cara (reixa del casc, dorsal al pit) o d'esquena
  // (dorsal gros a l'esquena, ratlla del casc pel mig). targets: on van les mans.
  drawPlayerFacing(p, s, o) {
    const g = this.ctx, { front, sw, moving, hipY, shY, headY, jersey, trim, pants, helmet, helmetEdge, skin, pl, targets, kick = 0 } = o;
    // Cames: la que puja s'escurça (el peu s'aixeca cap a la càmera o lluny).
    for (const k of [-1, 1]) {
      const lift = moving ? Math.max(0, sw * k) : 0, kk = kick && k === 1 ? kick : 0;
      const hip = { x: p.x + k * .13 * s, y: hipY };
      const foot = { x: p.x + k * .17 * s, y: p.y - lift * .2 * s - kk * .55 * s };
      const knee = { x: p.x + k * .17 * s, y: lerp(hipY, foot.y, .48) - lift * .1 * s - kk * .15 * s };
      g.strokeStyle = pants; g.lineWidth = .22 * s; g.beginPath(); g.moveTo(hip.x, hip.y); g.lineTo(knee.x, knee.y); g.stroke();
      g.strokeStyle = shade(jersey, -.2); g.lineWidth = .16 * s; g.beginPath(); g.moveTo(knee.x, knee.y); g.lineTo(foot.x, foot.y); g.stroke();
      g.fillStyle = lift > .3 && !front ? '#3a3a40' : '#16161a'; g.beginPath(); g.ellipse(foot.x, foot.y, .1 * s, .07 * s, 0, 0, Math.PI * 2); g.fill();
    }
    const tw = 1.0 * s, ww = .58 * s;
    const shoulder = k => ({ x: p.x + k * (tw / 2 - .04 * s), y: shY + .1 * s });
    // Braços: cap on toquen (la pilota, el rival) o balancejant-se.
    const swing = k => {
      const sh = shoulder(k), sw2 = moving ? sw * k : 0;
      this.limb(sh, { x: p.x + k * (.52 + Math.max(0, sw2) * .06) * s, y: shY + (.7 - Math.max(0, sw2) * .2) * s }, s, shade(jersey, -.15), skin, p.x);
    };
    let tl = null, tr = null;
    if (targets) {
      const [a, b] = targets;
      if (a && b) { if (a.x <= b.x) { tl = a; tr = b; } else { tl = b; tr = a; } }
      else { const one = a || b; if (one) { if (one.x < p.x) tl = one; else tr = one; } }
    }
    // Els braços que van cap a un punt més lluny que el cos (d'esquena, endavant)
    // queden darrere; els que venen cap a la càmera, davant.
    const reach = (k, T) => this.limb(shoulder(k), T, s, shade(jersey, -.15), skin, p.x);
    if (tl && !tl.near) reach(-1, tl);
    if (tr && !tr.near) reach(1, tr);
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
      if (big && s > 22) { g.font = `800 ${.1 * s}px "Segoe UI", system-ui, sans-serif`; g.fillText(String(pl.name || '').toUpperCase().slice(0, 12), p.x, shY + .2 * s); }
    }
    if (tl && tl.near) reach(-1, tl); else if (!tl) swing(-1);
    if (tr && tr.near) reach(1, tr); else if (!tr) swing(1);
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

  drawLabel(pl, p, s, ly, isCarrier) {
    const g = this.ctx;
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


  // Línies de velocitat a la càmera de jugador quan corre de pressa.
  drawSpeed() {
    if (!this.is3D() || this.camMode !== 'player' || this.phase === 'pre') return;
    const k = clamp((this.cam3.speed - 4.5) / 5, 0, 1);
    if (k <= 0) return;
    const g = this.ctx, W = this.W, H = this.H, d = this.dpr, cx = W / 2, cy = this.view.cy, R = Math.hypot(W, H) / 2;
    g.strokeStyle = `rgba(255,255,255,${(.22 * k).toFixed(3)})`; g.lineWidth = 1.6 * d; g.beginPath();
    for (let i = 0; i < 34; i++) {
      const a = Math.random() * Math.PI * 2, r0 = R * (.55 + Math.random() * .3), r1 = r0 + R * (.15 + .3 * k);
      g.moveTo(cx + Math.cos(a) * r0, cy + Math.sin(a) * r0); g.lineTo(cx + Math.cos(a) * r1, cy + Math.sin(a) * r1);
    }
    g.stroke();
  }

  // Tall de càmera: flaix blanc; escombrat ràpid (whip) d'un jugador a l'altre
  // amb el fotograma esborronat; repetició: cortina en diagonal amb «REPLAY».
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
    if (this.fx.kind === 'whip') {
      const k = age / .3;
      if (k >= 1) { this.fx = null; return; }
      g.save(); g.globalAlpha = .45 * (1 - k);
      for (let i = 1; i <= 3; i++) { g.drawImage(this.cv, i * 26 * d * (1 - k), 0); g.drawImage(this.cv, -i * 26 * d * (1 - k), 0); }
      g.restore();
      g.fillStyle = `rgba(255,255,255,${(.18 * (1 - k)).toFixed(3)})`;
      for (let i = 0; i < 14; i++) g.fillRect(0, Math.random() * H, W, (1 + Math.random() * 3) * d);
      return;
    }
    const k = age / .28;
    if (k >= 1) { this.fx = null; return; }
    g.fillStyle = `rgba(255,255,255,${(.75 * (1 - k)).toFixed(3)})`; g.fillRect(0, 0, W, H);
  }

  // Etiqueta de la càmera a la cantonada (PLAYER CAM, SKYCAM, REPLAY…).
  drawCamTag() {
    const replay = this.phase === 'replay';
    const name = this.is3D() ? CAM_TAGS[this.camMode] : '';
    if (!replay && !name) return;
    const g = this.ctx, d = this.dpr;
    let text = replay ? (name ? 'REPLAY · ' + name : 'REPLAY') : name;
    if (this.slow < 1) text += ' · SLOW-MO';
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

  // El marcador que toca ara (abans d'acabar la jugada, el d'abans).
  score() {
    const p = this.cur ? this.cur.play : this.shown();
    const after = this.phase === 'post' || this.phase === 'replay' || !this.cur;
    return after ? (p ? p.score : [0, 0]) : (this.queue[this.idx - 2] || { score: [0, 0] }).score;
  }

  drawHUD() {
    const g = this.ctx, W = this.W, d = this.dpr, m = this.match, T = this.T;
    const p = this.cur ? this.cur.play : this.shown();
    const score = this.score();
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
    // Text de la jugada i comentari (no tapa les repeticions ni el moment d'abans).
    if (p && (!this.cur || (this.phase === 'post' && this.cur.replaysDone))) {
      const lines = [p.text]; if (p.comment) lines.push('🎙 ' + (m.commentator ? m.commentator + ': ' : '') + p.comment);
      g.font = `600 ${13 * d}px "Segoe UI", system-ui, sans-serif`;
      const maxW = Math.min(W - 24 * d, 760 * d);
      const wrapped = [];
      lines.forEach((l, i) => { const words = String(l || '').split(' '); let cur = ''; words.forEach(w => { const test = cur ? cur + ' ' + w : w; if (g.measureText(test).width > maxW - 24 * d) { wrapped.push([cur, i]); cur = w; } else cur = test; }); if (cur) wrapped.push([cur, i]); });
      const lh = 18 * d, hgt = wrapped.length * lh + 14 * d, y0 = this.H - hgt - 12 * d;
      g.fillStyle = 'rgba(8,10,20,.82)'; g.beginPath(); g.roundRect((W - maxW) / 2, y0, maxW, hgt, 10 * d); g.fill();
      g.textAlign = 'left'; g.textBaseline = 'top';
      wrapped.forEach(([l, i], k) => { g.fillStyle = i === 0 ? '#fff' : '#ffd98a'; g.fillText(l, (W - maxW) / 2 + 12 * d, y0 + 7 * d + k * lh); });
    } else if (!this.cur && m.status !== 'final') {
      this.drawCenterText(m.status === 'pregame' ? T.pregame : T.waiting);
    }
  }
}
