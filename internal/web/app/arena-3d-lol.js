// League of Legends in 3D (arena-3d.js adapter): Summoner's Rift seen over a
// champion's shoulder (or through their eyes).
//
// Everything comes from the 2D viewer (arena-lol-view.js, LolView), read only:
// the map model (lanes, river, towers, inhibitors, nexus, pits, fountains), the
// match state (positions, deaths and respawns, kills, gold, towers, buffs,
// objective timers), the action cam's shot (the fight or objective it films,
// the monster or structure whose HP drains on camera), its procedural ability
// effects (skill shots, AoE circles, dashes, slams, shields, heals, tower shots,
// smites, damage numbers) and its replays. This file only turns that into a
// world: terrain painted from the map with lanes and bases, an animated river,
// instanced jungle (trees and rock walls), towers that lose HP and collapse
// into rubble, inhibitors and nexus crystals, the dragon (coloured by element),
// Baron, the Herald and the Void Grubs with HP bars, ten champions built from
// the kit's class silhouettes, and instanced minion waves walking the lanes.
//
// Coordinates: the map is 0..100 on both axes, blue base bottom-left (x right,
// y down, as drawn by the 2D viewer). The world is Y-up with the map centre at
// the origin and K world units per map unit: mapToWorld(x, y, h) =
// [(x - 50) * K, h, (y - 50) * K]. A heading (dx, dy) on the map is yaw
// atan2(-dy, dx) in the world (see arena-3d.js). Champions are ~2.4 tall.

import {
  lanePoint, waveFront, teamColor, mapSide, blueTeam, champInfo, fmtClock, fmtGold, ROLES, CLASS_COLORS, DRAGON_COLORS,
  drainHp, camAt, camTilt, easeInOut, FULL_CAM, TEXT, CAM_TAGS,
} from './arena-lol-view.js';
import { clamp, lerp, rng, easeOut, esc } from './arena-view-base.js';
import { dampAngle, damp, forwardOf, yawPitchOf, wrapAngle } from './arena-3d.js';

export const K = 2; // world units per map unit
const TAU = Math.PI * 2;
const FOUNTAIN = [[5, 95], [95, 5]];
const JUNGLE = [[30, 60], [70, 40]];
const MON_R = { baron: 7.5, drake: 6.5, elder: 7, herald: 6, grubs: 5.5 };
const KIT_CLASS = { tank: 'tank', engage: 'tank', bruiser: 'fighter', assassin: 'assassin', mage: 'mage', marksman: 'marksman', enchanter: 'support' };
const SKIN = ['#f1c7a5', '#d9a47e', '#b77b52', '#8d5a3b', '#e8b893', '#c7b3e6', '#a9cfd9', '#9fd3a8'];

// ---------------------------------------------------------------------------
// Pure pieces (tested in arena-3d-lol.test.cjs)
// ---------------------------------------------------------------------------
export function mapToWorld(x, y, h = 0) { return [(x - 50) * K, h, (y - 50) * K]; }
export function worldToMap(p) { return { x: p[0] / K + 50, y: p[2] / K + 50, h: p[1] }; }
export function headingYaw(dx, dy) { return Math.atan2(-dy, dx); }
export const champId = (t, r) => `${t}:${r}`;
export function parseId(id) { const m = /^([01]):([0-4])$/.exec(String(id)); return m ? { t: +m[1], r: +m[2] } : null; }
export const isAliveAt = (pos, t, r) => !!(pos && pos[t] && pos[t][r] && pos[t][r].alive !== false);

function segDist(px, py, a, b) {
  const dx = b[0] - a[0], dy = b[1] - a[1], l2 = dx * dx + dy * dy || 1;
  const t = clamp(((px - a[0]) * dx + (py - a[1]) * dy) / l2, 0, 1);
  return Math.hypot(px - a[0] - t * dx, py - a[1] - t * dy);
}
export function polyDist(px, py, pts) { let m = Infinity; for (let i = 1; i < (pts || []).length; i++) m = Math.min(m, segDist(px, py, pts[i - 1], pts[i])); return m; }

// Who can be watched: the ten champions, blue side first, top → support.
export function rosterOf(st) {
  if (!st || !st.line) return [];
  const b = blueTeam(st.meta), out = [], RS = TEXT.en.roleShort;
  for (const t of [b, 1 - b]) for (let r = 0; r < 5; r++) {
    const l = st.line[t][r] || {}, role = l.role || ROLES[r];
    out.push({ id: champId(t, r), name: l.champ ? `${l.champ} (${l.handle || role})` : (l.handle || role), role: RS[role] || String(role).toUpperCase(), team: t, color: teamColor(st.meta, t) });
  }
  return out;
}

// Where a champion stands this frame (map units): the viewer's position plus
// the same idle wobble, dash and fight spread the 2D action cam draws
// (LolView.champXY, with src instead of the viewer so replays work too).
export function champSpot(src, t, r) {
  const p = src.pos && src.pos[t] && src.pos[t][r];
  if (!p) return [50, 50];
  if (p.alive === false) return [p.x, p.y];
  const seed = t * 5 + r, a = src.camT || 0, amp = .6 + .9 * a, sp = 1 + 1.4 * a, an = src.anim || 0;
  let x = p.x + Math.sin(an * 1.3 * sp + seed * 1.7) * amp, y = p.y + Math.cos(an * 1.1 * sp + seed * 2.3) * amp;
  const ds = src.dash && src.dash[t] && src.dash[t][r];
  if (ds) {
    const age = an - ds.at, f = age < 0 ? 0 : age < .1 ? age / .1 : Math.max(0, 1 - (age - .1) / .8);
    x += ds.dx * easeOut(f); y += ds.dy * easeOut(f);
  }
  if (a > 0 && src.camFocus) {
    const [fx, fy] = src.camFocus, dx = x - fx, dy = y - fy;
    const d = Math.hypot(dx, dy), w = 1.1 * a * Math.max(0, 1 - d / 18);
    x = fx + dx * (1 + w); y = fy + dy * (1 + w);
    const sh = src.shot, m = sh && sh.mon;
    if (m && Number.isFinite(m.dieAt) && an < m.dieAt + .4) {
      const R0 = (MON_R[m.kind] || 4.5) * a, ox = x - sh.x, oy = y - sh.y, od = Math.hypot(ox, oy);
      if (od < R0) { const ang = od > .01 ? Math.atan2(oy, ox) : seed; x = sh.x + Math.cos(ang) * R0; y = sh.y + Math.sin(ang) * R0; }
    }
  }
  return [x, y];
}

// What the frame shows: the live match, or the replay's recorded frames
// (interpolated, like LolView.drawReplayMap).
export function frameSource(v) {
  const st = v.state, rp = v.replay;
  if (rp && rp.frames && rp.frames.length) {
    const fr = rp.frames, n = fr.length, p = clamp(rp.t * rp.rate / .05, 0, n - 1), i = Math.floor(p), f = p - i;
    const A = fr[i], B = fr[Math.min(n - 1, i + 1)];
    const pos = A.pos.map((a, t) => a.map((q, r) => { const o = B.pos[t][r]; return q.alive === o.alive ? { x: lerp(q.x, o.x, f), y: lerp(q.y, o.y, f), alive: q.alive } : q; }));
    const k = rp.len > 0 ? rp.t / rp.len : 1, z = rp.cam.z * (1 + .1 * easeInOut(k));
    return { replay: true, pos, hp: A.hp, acts: A.acts || [], flashes: A.flashes || [], particles: A.particles || [], popups: A.popups || [], skulls: A.skulls || [], anim: lerp(A.anim, B.anim, f), clock: A.clock, dash: A.dash, air: A.air, shot: rp.shot, camT: camTilt(z), camFocus: [rp.cam.x, rp.cam.y], seg: i };
  }
  const cam = st && st.mode === 'map' && v.camTw ? camAt(v.camTw, v.anim || 0) : FULL_CAM;
  return { replay: false, pos: st ? st.pos : [[], []], hp: v.hp, acts: v.acts || [], flashes: v.flashes || [], particles: v.particles || [], popups: v.popups || [], skulls: v.skulls || [], anim: v.anim || 0, clock: v.clock || 0, dash: v.dash, air: v.air, shot: v.shot, camT: camTilt(cam.z), camFocus: [cam.x, cam.y] };
}

// Who the camera follows on its own, like the action cam's director: the
// killer of a kill; in a filmed fight or objective, the champion nearest to
// what the camera films (the team taking the objective, the side pushing the
// structure); for a gank the jungler; otherwise whoever it followed, else the
// blue mid laner. memo { id, since } keeps a choice for at least 2 s so the
// camera doesn't flick between two champions of the same fight.
export function autoChampion(v, memo = {}) {
  const st = v && v.state;
  if (!st || !st.pos || !st.pos[0] || !st.pos[0].length || st.mode === 'draft') return null;
  const src = frameSource(v), now = src.anim, alive = (t, r) => isAliveAt(src.pos, t, r);
  const prev = parseId(memo.id), prevOk = prev && alive(prev.t, prev.r);
  const nearest = (x, y, team) => {
    let best = null, bd = Infinity;
    for (let t = 0; t < 2; t++) {
      if (team === 0 || team === 1) { if (t !== team) continue; }
      for (let r = 0; r < 5; r++) {
        if (!alive(t, r)) continue;
        const [cx, cy] = champSpot(src, t, r), d = Math.hypot(cx - x, cy - y);
        if (d < bd) { bd = d; best = { t, r, d }; }
      }
    }
    return best;
  };
  const choose = (c, force) => {
    if (!c) return null;
    const id = champId(c.t, c.r);
    if (!force && prevOk && memo.id !== id && now - (memo.since || -99) < 2) return memo.id;
    if (memo.id !== id) { memo.id = id; memo.since = now; }
    return id;
  };
  const ev = !src.replay && v.cur ? v.cur.ev : null, d = (ev && ev.data) || {};
  if (ev && ev.kind === 'kill' && d.killer && (d.killer.side === 0 || d.killer.side === 1) && alive(d.killer.side, d.killer.slot | 0)) return choose({ t: d.killer.side, r: d.killer.slot | 0 }, true);
  const sh = src.shot;
  if (sh && Number.isFinite(sh.x)) {
    let team = -1;
    if (sh.mon) team = sh.team;
    else if (sh.struct) team = mapSide(st.meta, 0) !== sh.struct.side ? 0 : 1;
    let c = nearest(sh.x, sh.y, team);
    if (!c || c.d > 22) c = nearest(sh.x, sh.y, -1);
    if (c && prevOk) {
      const [px, py] = champSpot(src, prev.t, prev.r);
      if (Math.hypot(px - sh.x, py - sh.y) < 10 && (team < 0 || prev.t === team)) return choose(prev, false);
    }
    if (c) return choose(c, false);
  }
  if (ev && Number.isFinite(d.x) && Number.isFinite(d.y)) {
    if (ev.kind === 'gank' && (d.side === 0 || d.side === 1) && alive(d.side, 1)) return choose({ t: d.side, r: 1 }, false);
    const c = nearest(d.x, d.y, d.side === 0 || d.side === 1 ? d.side : -1) || nearest(d.x, d.y, -1);
    if (c && c.d < 25) return choose(c, false);
  }
  if (prevOk) return memo.id;
  const b = blueTeam(st.meta);
  for (const [t, r] of [[b, 2], [b, 3], [1 - b, 2], [b, 0], [b, 1], [b, 4], [1 - b, 0], [1 - b, 1], [1 - b, 3], [1 - b, 4]]) if (alive(t, r)) return choose({ t, r }, true);
  return null;
}

// The over-the-shoulder camera: behind the champion at `pos` (world, feet)
// looking along yaw, a little to the right and above, aimed just ahead.
// eye: the champion's own eyes instead.
export function shoulderCam(pos, yaw, { eye = false, eyeH = 2.18, dist = 6.2, height = 3.6, side = 1.1, ahead = 7, look = 1.3 } = {}) {
  const f = forwardOf(yaw, 0), right = [Math.sin(yaw), 0, Math.cos(yaw)];
  if (eye) {
    const p = [pos[0] + f[0] * .28, pos[1] + eyeH, pos[2] + f[2] * .28];
    return { pos: p, yaw, pitch: -.1, fov: 80 };
  }
  const p = [pos[0] - f[0] * dist + right[0] * side, pos[1] + height, pos[2] - f[2] * dist + right[2] * side];
  const tgt = [pos[0] + f[0] * ahead + right[0] * side * .35, pos[1] + look, pos[2] + f[2] * ahead + right[2] * side * .35];
  const yp = yawPitchOf([tgt[0] - p[0], tgt[1] - p[1], tgt[2] - p[2]]);
  return { pos: p, yaw: yp.yaw, pitch: yp.pitch, fov: 68 };
}

// ---------------------------------------------------------------------------
// Painting
// ---------------------------------------------------------------------------
function shadeHex(h, f) {
  const m = /^#?([0-9a-f]{6})$/i.exec(h || ''); const n = m ? parseInt(m[1], 16) : 0x808080;
  const k = v => clamp(Math.round(f < 0 ? v * (1 + f) : v + (255 - v) * f), 0, 255);
  return '#' + [k(n >> 16), k((n >> 8) & 255), k(n & 255)].map(v => v.toString(16).padStart(2, '0')).join('');
}
function paintRift(g, W, H, map, sideCols) {
  const S = W / 100, P = p => [p[0] * S, p[1] * S];
  g.fillStyle = '#26401f'; g.fillRect(0, 0, W, H);
  const R = rng(7);
  for (let i = 0; i < 7000; i++) { g.fillStyle = R() < .5 ? 'rgba(0,0,0,.07)' : 'rgba(150,190,90,.05)'; const r = 2 + R() * 9; g.beginPath(); g.arc(R() * W, R() * H, r, 0, TAU); g.fill(); }
  const stroke = (pts, w, col, cap = 'round') => { if (!pts || pts.length < 2) return; g.strokeStyle = col; g.lineWidth = w * S; g.lineCap = cap; g.lineJoin = 'round'; g.beginPath(); pts.forEach((p, i) => { const q = P(p); if (i) g.lineTo(q[0], q[1]); else g.moveTo(q[0], q[1]); }); g.stroke(); };
  // Bases: stone plazas in team colours.
  (map.nexus || []).forEach(nx => {
    const c = sideCols[nx.side], [x, y] = P([nx.x, nx.y]);
    const gr = g.createRadialGradient(x, y, 0, x, y, 24 * S); gr.addColorStop(0, '#7b776d'); gr.addColorStop(.72, '#5f5c55'); gr.addColorStop(1, 'rgba(70,68,62,0)');
    g.fillStyle = gr; g.beginPath(); g.arc(x, y, 24 * S, 0, TAU); g.fill();
    g.strokeStyle = shadeHex(c, -.2); g.globalAlpha = .55; g.lineWidth = .8 * S; for (const r of [9, 15]) { g.beginPath(); g.arc(x, y, r * S, 0, TAU); g.stroke(); } g.globalAlpha = 1;
    for (let i = 0; i < 16; i++) { const a = i / 16 * TAU; g.fillStyle = 'rgba(0,0,0,.12)'; g.fillRect(x + Math.cos(a) * 12 * S - 3, y + Math.sin(a) * 12 * S - 3, 6, 6); }
  });
  FOUNTAIN.forEach((f, side) => { const [x, y] = P(f); const gr = g.createRadialGradient(x, y, 0, x, y, 7 * S); gr.addColorStop(0, shadeHex(sideCols[side], .45)); gr.addColorStop(1, 'rgba(0,0,0,0)'); g.fillStyle = gr; g.beginPath(); g.arc(x, y, 7 * S, 0, TAU); g.fill(); });
  // River bed, then the lanes (darker edge, dirt, a lighter centre).
  stroke(map.river, 9, '#1d3532'); stroke(map.river, 6.5, '#29504c');
  const L = map.lanes || {};
  for (const k of ['top', 'mid', 'bot']) stroke(L[k], 8.4, '#4c4535');
  for (const k of ['top', 'mid', 'bot']) stroke(L[k], 6.8, '#8a7a5b');
  for (const k of ['top', 'mid', 'bot']) stroke(L[k], 3.2, 'rgba(176,160,122,.55)');
  // Pits.
  for (const o of [map.dragon, map.baron]) { if (!o) continue; const [x, y] = P([o.x, o.y]); g.fillStyle = '#3a342b'; g.beginPath(); g.arc(x, y, 7.5 * S, 0, TAU); g.fill(); g.strokeStyle = '#5b5141'; g.lineWidth = .9 * S; g.stroke(); }
  // Jungle clearings.
  for (const j of JUNGLE.concat([[24, 46], [76, 54], [47, 74], [53, 26]])) { const [x, y] = P(j); g.fillStyle = 'rgba(110,120,70,.28)'; g.beginPath(); g.arc(x, y, 4.5 * S, 0, TAU); g.fill(); }
}
function paintWater(g, w, h) {
  g.fillStyle = '#2a7f95'; g.fillRect(0, 0, w, h);
  const R = rng(3);
  for (let i = 0; i < 240; i++) { g.strokeStyle = `rgba(200,245,255,${.08 + R() * .18})`; g.lineWidth = 1 + R() * 2; const x = R() * w, y = R() * h, l = 8 + R() * 26; g.beginPath(); g.moveTo(x, y); g.quadraticCurveTo(x + l / 2, y + 4, x + l, y); g.stroke(); }
}

// Value noise for the jungle walls.
function noise2(seed) {
  const R = rng(seed), N = 64, tab = Array.from({ length: N * N }, () => R());
  const at = (i, j) => tab[((i % N + N) % N) * N + ((j % N + N) % N)];
  const sm = t => t * t * (3 - 2 * t);
  return (x, y) => { const i = Math.floor(x), j = Math.floor(y), fx = sm(x - i), fy = sm(y - j); return lerp(lerp(at(i, j), at(i + 1, j), fx), lerp(at(i, j + 1), at(i + 1, j + 1), fx), fy); };
}

// The jungle: where trees and rock walls stand (map units), away from lanes,
// the river, the bases, the pits and the camps; thick round the edge.
export function jungleSpots(map, { step = 2.4, seed = 11 } = {}) {
  const R = rng(seed), n1 = noise2(seed + 1), out = [];
  const L = map.lanes || {}, lanes = ['top', 'mid', 'bot'].map(k => L[k]).filter(Boolean);
  const nex = map.nexus && map.nexus.length ? map.nexus : [{ x: 12, y: 88 }, { x: 88, y: 12 }];
  const pits = [map.dragon, map.baron].filter(Boolean);
  const camps = JUNGLE.concat([[24, 46], [76, 54], [47, 74], [53, 26]]);
  for (let gx = -14; gx <= 114; gx += step) for (let gy = -14; gy <= 114; gy += step) {
    const x = gx + (R() - .5) * step * .8, y = gy + (R() - .5) * step * .8;
    const outside = x < 1 || x > 99 || y < 1 || y > 99;
    const dl = Math.min(...lanes.map(l => polyDist(x, y, l)));
    if (dl < 5.4) continue;
    if (polyDist(x, y, map.river || []) < 6) continue;
    if (nex.some(n => Math.hypot(n.x - x, n.y - y) < 23)) continue;
    if (pits.some(p => Math.hypot(p.x - x, p.y - y) < 9.5)) continue;
    if (camps.some(c => Math.hypot(c[0] - x, c[1] - y) < 5)) continue;
    const edge = outside || x < 4 || x > 96 || y < 4 || y > 96;
    if (!edge && n1(x * .13, y * .13) < .4) continue;
    if (outside && R() < .35) continue;
    out.push({ x, y, rock: !edge && R() < .3, s: .8 + R() * .6, rot: R() * TAU, tint: .82 + R() * .3 });
  }
  return out;
}

// ---------------------------------------------------------------------------
// Small render helpers (instanced pools, billboards, text)
// ---------------------------------------------------------------------------
function instPool(THREE, kit, geo, material, n) {
  const mesh = new THREE.InstancedMesh(geo, material, n);
  mesh.frustumCulled = false;
  const white = new THREE.Color(1, 1, 1);
  for (let i = 0; i < n; i++) mesh.setColorAt(i, white);
  mesh.count = 0;
  const M4 = new THREE.Matrix4(), V = new THREE.Vector3(), S = new THREE.Vector3(), C = new THREE.Color(), Q0 = new THREE.Quaternion();
  let k = 0;
  return {
    mesh, n,
    begin() { k = 0; },
    cam: null, // set: skip shapes the camera is inside (they would fill the screen)
    add(x, y, z, q, sx, sy, sz, color, f = 1) {
      if (k >= n) return;
      const c = this.cam;
      if (c && Math.hypot(x - c.x, y - c.y, z - c.z) < Math.max(sx, sy, sz) * .5 + .6) return;
      M4.compose(V.set(x, y, z), q || Q0, S.set(sx, sy, sz)); mesh.setMatrixAt(k, M4);
      C.copy(kit.col(color)).multiplyScalar(f); mesh.setColorAt(k, C); k++;
    },
    end() { mesh.count = k; mesh.instanceMatrix.needsUpdate = true; if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true; },
  };
}

// Sprites with canvas text, reused frame to frame by key.
function textPool(THREE, kit, scene, n, w = 256, h = 80) {
  const items = [];
  for (let i = 0; i < n; i++) {
    const tex = kit.canvasTexture(w, h, () => {}, { aniso: 1 });
    const sp = new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, transparent: true, depthTest: false, depthWrite: false, fog: false, sizeAttenuation: false }));
    sp.visible = false; sp.renderOrder = 20; scene.add(sp);
    items.push({ sp, tex, key: null, sig: '' });
  }
  let used = new Map(), last = new Map(), taken = new Set();
  return {
    begin() { last = used; used = new Map(); taken = new Set(); },
    put(key, text, color, x, y, z, { alpha = 1, size = 1, outline = 'rgba(0,0,0,.85)', weight = 900 } = {}) {
      let it = last.get(key);
      if (!it || taken.has(it)) { const prev = new Set(last.values()); it = items.find(o => !taken.has(o) && !prev.has(o)) || items.find(o => !taken.has(o)); }
      if (!it) return;
      used.set(key, it); taken.add(it);
      const sig = text + '|' + color + '|' + weight;
      if (it.sig !== sig) {
        it.sig = sig;
        it.tex.redraw((g, W, H) => {
          g.clearRect(0, 0, W, H); g.font = `${weight} ${Math.round(H * .62)}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.textBaseline = 'middle';
          g.lineJoin = 'round'; g.lineWidth = H * .14; g.strokeStyle = outline; g.strokeText(text, W / 2, H / 2, W - 8); g.fillStyle = color; g.fillText(text, W / 2, H / 2, W - 8);
        });
      }
      // size 1 ≈ 4% of the view's height, whatever the distance (like the game's UI).
      it.sp.visible = true; it.sp.position.set(x, y, z); it.sp.material.opacity = alpha; it.sp.scale.set(size * .055 * w / h, size * .055, 1);
    },
    end() { for (const it of items) if (!taken.has(it)) it.sp.visible = false; },
  };
}

// HP bars: two instanced quads (back, fill) turned to the camera.
function barPool(THREE, kit, scene, n) {
  const geo = new THREE.PlaneGeometry(1, 1);
  const back = instPool(THREE, kit, geo, new THREE.MeshBasicMaterial({ color: 0xffffff, transparent: true, opacity: .8, depthTest: false, depthWrite: false, fog: false }), n);
  const fill = instPool(THREE, kit, geo, new THREE.MeshBasicMaterial({ color: 0xffffff, transparent: true, depthTest: false, depthWrite: false, fog: false }), n);
  back.mesh.renderOrder = 18; fill.mesh.renderOrder = 19;
  scene.add(back.mesh, fill.mesh);
  const R = new THREE.Vector3(), C = new THREE.Vector3();
  let q = null;
  return {
    begin(camera) { back.begin(); fill.begin(); q = camera.quaternion; C.copy(camera.position); R.set(1, 0, 0).applyQuaternion(q); },
    add(x, y, z, w, hp, color, h = .22) {
      hp = clamp(hp, 0, 1);
      // About the same size on screen near and far.
      const dist = Math.hypot(x - C.x, y - C.y, z - C.z);
      if (dist < 5) return;
      const k = clamp(dist / 16, .45, 4); w *= k; h *= k;
      back.add(x, y, z, q, w + .08, h + .08, 1, '#05060a');
      const off = -w / 2 + w * hp / 2;
      if (hp > 0) fill.add(x + R.x * off, y + R.y * off, z + R.z * off, q, w * hp, h, 1, color);
    },
    end() { back.end(); fill.end(); },
  };
}

const riftKey = st => [0, 1].map(t => teamColor(st.meta, t) + mapSide(st.meta, t)).join() + JSON.stringify(st.map.lanes || {}) + (st.map.towers || []).length;

// ---------------------------------------------------------------------------
// The adapter
// ---------------------------------------------------------------------------
const CSS = `.a3-lol-card { position:absolute; left:8px; top:44px; background:rgba(8,10,20,.82); border-radius:8px; padding:6px 10px; font-size:12px; line-height:1.35; border-left:4px solid var(--tc,#ffd400); min-width:150px; }
.a3-lol-card:empty, .a3-lol-feed:empty { display:none; }
.a3-lol-card b { font-size:13px; } .a3-lol-card .k { color:#ffd400; font-weight:800; } .a3-lol-card .dead { color:#ff8080; font-weight:800; }
.a3-lol-boss { position:absolute; left:50%; top:96px; transform:translateX(-50%); width:min(380px, 60%); font-size:11px; font-weight:800; text-shadow:0 1px 2px #000; }
.a3-lol-boss:empty { display:none; }
.a3-lol-boss .bar { height:10px; margin-top:2px; background:rgba(5,5,10,.85); border:1px solid var(--mc,#9b59ff); border-radius:3px; overflow:hidden; }
.a3-lol-boss .bar i { display:block; height:100%; background:var(--mc,#9b59ff); }
.a3-lol-feed { position:absolute; right:8px; bottom:50px; display:flex; flex-direction:column; gap:3px; align-items:flex-end; }
.a3-lol-feed div { background:rgba(8,10,20,.82); border-radius:6px; padding:3px 8px; font-size:12px; font-weight:700; white-space:nowrap; }
.a3-lol-feed i { font-style:normal; opacity:.8; margin:0 5px; }
@media (max-width:760px) { .a3-lol-feed { display:none; } .a3-lol-card { font-size:11px; padding:4px 8px; min-width:0; } }`;

export const adapter = {
  label: 'League of Legends',

  build(ctx) {
    const { THREE, scene, kit } = ctx, s = ctx.state;
    s.memo = {}; s.face = new Map(); s.camYaw = null; s.models = new Map(); s.prevSpot = new Map(); s.phase = new Map(); s.corpse = new Map();
    s.eye = false; s.cut = true; s.lastAnim = 0; s.towerFall = new Map(); s.structAlive = new Map(); s.dt = 1 / 60; s.monDie = {};
    scene.background = new THREE.Color('#9fb7c9');
    scene.fog = new THREE.Fog(0x9fb7c9, 140, 360);
    kit.lights(scene, { sky: '#dfe9ff', ground: '#3b4a2c', hemi: 1.35, sun: '#fff1d6', sunI: 1.6, dir: [-60, 110, 50] });
    const low = !!(ctx.quality && ctx.quality.low);
    s.low = low;
    // Ground beyond the map, then the Rift itself (painted once the map is known).
    const far = new THREE.Mesh(new THREE.PlaneGeometry(900, 900), kit.mat('#1f3319')); far.rotation.x = -Math.PI / 2; far.position.y = -.05; scene.add(far);
    s.groundTex = kit.canvasTexture(low ? 1024 : 2048, low ? 1024 : 2048, (g, w, h) => { g.fillStyle = '#26401f'; g.fillRect(0, 0, w, h); }, { aniso: Math.min(8, ctx.renderer.capabilities.getMaxAnisotropy()) });
    const ground = new THREE.Mesh(new THREE.PlaneGeometry(100 * K, 100 * K), new THREE.MeshLambertMaterial({ map: s.groundTex }));
    ground.rotation.x = -Math.PI / 2; scene.add(ground);
    s.waterTex = kit.canvasTexture(256, 256, paintWater, { aniso: 2, repeat: true });
    // Pools: shadows, minions, effects, bars, text.
    s.shadows = kit.blobShadows(100); scene.add(s.shadows.mesh);
    const add = new THREE.MeshBasicMaterial({ color: 0xffffff, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false, fog: false, side: THREE.DoubleSide });
    const ringG = new THREE.RingGeometry(.84, 1, 40); ringG.rotateX(-Math.PI / 2);
    const discG = new THREE.CircleGeometry(1, 32); discG.rotateX(-Math.PI / 2);
    const arcG = new THREE.RingGeometry(.78, 1, 14, 1, -1.1, 2.2);
    const beamG = new THREE.CylinderGeometry(.5, .5, 1, 6, 1, true); beamG.translate(0, .5, 0);
    s.fx = {
      orb: instPool(THREE, kit, new THREE.SphereGeometry(.5, 10, 7), add, 160),
      ring: instPool(THREE, kit, ringG, add, 96),
      disc: instPool(THREE, kit, discG, add, 48),
      arc: instPool(THREE, kit, arcG, add, 32),
      beam: instPool(THREE, kit, beamG, add, 64),
      chunk: instPool(THREE, kit, new THREE.BoxGeometry(1, 1, 1), kit.mat('#ffffff'), 160),
    };
    for (const k in s.fx) { s.fx[k].mesh.renderOrder = k === 'chunk' ? 0 : 5; scene.add(s.fx[k].mesh); }
    s.fx.orb.cam = ctx.camera.position;
    s.bars = barPool(THREE, kit, scene, 64);
    s.text = textPool(THREE, kit, scene, 26);
    s.labels = textPool(THREE, kit, scene, 12, 320, 72);
    // Minions: one instanced mesh, tinted per side.
    const minionG = kit.merge([
      { g: kit.G.box, c: '#d8dbe2', m: kit.M(0, .55, 0, .55, .6, .5) },
      { g: kit.G.sph, c: '#f2f2f2', m: kit.M(.05, 1.02, 0, .42, .4, .42) },
      { g: kit.G.box, c: '#9aa0ad', m: kit.M(.32, .62, .2, .35, .1, .1) },
      { g: kit.G.cyl, c: '#5a5f6b', m: kit.M(0, .15, .14, .16, .32, .16) },
      { g: kit.G.cyl, c: '#5a5f6b', m: kit.M(0, .15, -.14, .16, .32, .16) },
    ]);
    s.minions = new THREE.InstancedMesh(minionG, kit.mats.vc, 96); s.minions.frustumCulled = false; s.minions.count = 0;
    for (let i = 0; i < 96; i++) s.minions.setColorAt(i, kit.col('#ffffff'));
    scene.add(s.minions);
    s.M4 = new THREE.Matrix4(); s.V = new THREE.Vector3(); s.Q = new THREE.Quaternion(); s.S = new THREE.Vector3(); s.UP = new THREE.Vector3(0, 1, 0); s.AX = new THREE.Vector3(); s.QX = new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(1, 0, 0), -Math.PI / 2);
    // The view toggle (over the shoulder / eyes) and the DOM bits of the HUD.
    if (typeof document !== 'undefined' && ctx.root) {
      if (!document.getElementById('a3LolCSS')) { const st = document.createElement('style'); st.id = 'a3LolCSS'; st.textContent = CSS; document.head.append(st); }
      const hud = ctx.root.querySelector('.a3-hud') || ctx.root;
      s.cardEl = document.createElement('div'); s.cardEl.className = 'a3-lol-card'; hud.append(s.cardEl);
      s.feedEl = document.createElement('div'); s.feedEl.className = 'a3-lol-feed'; hud.append(s.feedEl);
      s.bossEl = document.createElement('div'); s.bossEl.className = 'a3-lol-boss'; hud.append(s.bossEl);
      const bar = ctx.root.querySelector('.a3-bar');
      if (bar) {
        const b = document.createElement('button'); b.type = 'button'; b.className = 'a3-lol-eye';
        const paint = () => { b.textContent = s.eye ? (ctx.lang === 'ca' ? '🎯 Darrere' : '🎯 Shoulder') : (ctx.lang === 'ca' ? '👁 Ulls' : '👁 Eyes'); b.title = ctx.lang === 'ca' ? 'Càmera darrere l’espatlla o als ulls del campió (V)' : 'Camera over the shoulder or through the champion’s eyes (V)'; };
        s.toggleEye = () => { s.eye = !s.eye; s.cut = true; paint(); };
        b.onclick = () => s.toggleEye();
        paint();
        const ex = bar.querySelector('[data-a="exit"]'); bar.insertBefore(b, ex || null);
        s.onKey = e => { if ((e.key === 'v' || e.key === 'V') && !e.ctrlKey && !e.metaKey && !e.altKey && !(e.target && /^(INPUT|TEXTAREA|SELECT)$/.test(e.target.tagName)) && ctx.root.isConnected) { s.toggleEye(); e.preventDefault(); } };
        window.addEventListener('keydown', s.onKey);
      }
    }
  },

  // The Rift: built when the map and the team colours are known.
  rift(ctx, st) {
    const { THREE, scene, kit } = ctx, s = ctx.state, map = st.map;
    if (s.rift) { scene.remove(s.rift); s.rift.traverse(o => { if (o.geometry && !Object.values(kit.G).includes(o.geometry)) o.geometry.dispose(); }); }
    const g = new THREE.Group(); s.rift = g;
    s.hideU = s.hideU || { uFocus: { value: new THREE.Vector3() }, uHide: { value: 0 } };
    // Stone that would hide the champion from the camera behind it is cut away.
    const clearMat = (m, key) => {
      m.onBeforeCompile = sh => {
        Object.assign(sh.uniforms, s.hideU);
        sh.vertexShader = sh.vertexShader.replace('#include <common>', '#include <common>\nvarying vec3 vA3W;').replace('#include <fog_vertex>', `#include <fog_vertex>
        vec4 a3w = vec4(transformed, 1.0);
        #ifdef USE_INSTANCING
        a3w = instanceMatrix * a3w;
        #endif
        vA3W = (modelMatrix * a3w).xyz;`);
        sh.fragmentShader = sh.fragmentShader.replace('#include <common>', '#include <common>\nvarying vec3 vA3W; uniform vec3 uFocus; uniform float uHide;').replace('void main() {', `void main() {
        if (uHide > 0.5) { vec2 c2 = cameraPosition.xz, d2 = uFocus.xz - c2; float tt = clamp(dot(vA3W.xz - c2, d2) / max(dot(d2, d2), 1e-4), 0.0, 1.0);
          if (tt < 0.82 && distance(vA3W.xz, c2 + d2 * tt) < 2.6) discard; }`);
      };
      m.customProgramCacheKey = () => key;
      return m;
    };
    const sideTeam = side => (mapSide(st.meta, 0) === side ? 0 : 1);
    const sideCols = [teamColor(st.meta, sideTeam(0)), teamColor(st.meta, sideTeam(1))];
    s.sideCols = sideCols;
    s.groundTex.redraw((c, w, h) => paintRift(c, w, h, map, sideCols));
    // River: a ribbon of scrolling water along the river line.
    const river = (map.river || []).map(p => new THREE.Vector3(...mapToWorld(p[0], p[1], .06)));
    if (river.length > 1) {
      // It narrows into streams where it meets the side lanes.
      const curve = new THREE.CatmullRomCurve3(river), N = 80, W = 3.3 * K, P = [], UV = [], I = [];
      for (let i = 0; i <= N; i++) {
        const u = .06 + .88 * i / N, p = curve.getPointAt(u), tg = curve.getTangentAt(u), nx = -tg.z, nz = tg.x, l = Math.hypot(nx, nz) || 1;
        const w = W * (.35 + .65 * Math.sin(Math.PI * u)) * (1 - .15 * Math.cos(u * TAU * 2));
        P.push(p.x + nx / l * w, .06, p.z + nz / l * w, p.x - nx / l * w, .06, p.z - nz / l * w);
        UV.push(0, u * 12, 1, u * 12);
        if (i) { const a = (i - 1) * 2; I.push(a, a + 1, a + 2, a + 1, a + 3, a + 2); }
      }
      const geo = new THREE.BufferGeometry(); geo.setAttribute('position', new THREE.Float32BufferAttribute(P, 3)); geo.setAttribute('uv', new THREE.Float32BufferAttribute(UV, 2)); geo.setIndex(I); geo.computeVertexNormals();
      const wm = new THREE.MeshLambertMaterial({ map: s.waterTex, transparent: true, opacity: .85, color: 0xbfefff, side: THREE.DoubleSide, polygonOffset: true, polygonOffsetFactor: -2, polygonOffsetUnits: -2 });
      g.add(new THREE.Mesh(geo, wm));
    }
    // The jungle: trees and rock walls (two instanced meshes) that step aside
    // for the camera (they shrink away near it and between it and the champion).
    const spots = jungleSpots(map, { step: s.low ? 3.1 : 2.4 });
    const treeG = kit.merge([
      { g: kit.G.cyl, c: '#5a3d26', m: kit.M(0, .8, 0, .45, 1.6, .45) },
      { g: kit.G.cone, c: '#1f5a2c', m: kit.M(0, 2.4, 0, 2.6, 2.4, 2.6) },
      { g: kit.G.cone, c: '#2a7038', m: kit.M(0, 3.7, 0, 2.0, 2.0, 2.0) },
      { g: kit.G.cone, c: '#378443', m: kit.M(0, 4.8, 0, 1.3, 1.5, 1.3) },
    ]);
    const rockG = (() => { const d = new THREE.DodecahedronGeometry(1, 0); const out = kit.merge([{ g: d, c: '#6c6f63', m: kit.M(0, .5, 0, 1.6, 1.25, 1.3) }, { g: d, c: '#55584e', m: kit.M(.7, .3, .5, .9, .8, .8) }]); d.dispose(); return out; })();
    const hideMat = () => {
      const m = new THREE.MeshLambertMaterial({ vertexColors: true });
      m.onBeforeCompile = sh => {
        Object.assign(sh.uniforms, s.hideU);
        sh.vertexShader = sh.vertexShader.replace('#include <common>', `#include <common>
        uniform vec3 uFocus; uniform float uHide;`).replace('#include <begin_vertex>', `#include <begin_vertex>
        #ifdef USE_INSTANCING
        vec3 ip = (modelMatrix * instanceMatrix * vec4(0.0, 0.0, 0.0, 1.0)).xyz;
        vec2 c2 = cameraPosition.xz, f2 = uFocus.xz, d2 = f2 - c2;
        float tt = clamp(dot(ip.xz - c2, d2) / max(dot(d2, d2), 1e-4), 0.0, 1.0);
        float dd = min(distance(ip.xz, c2 + d2 * tt), distance(ip.xz, c2));
        float keep = mix(1.0, smoothstep(2.2, 5.0, dd), uHide);
        transformed *= keep;
        #endif`);
      };
      m.customProgramCacheKey = () => 'a3lol-jungle';
      return m;
    };
    const trees = spots.filter(p => !p.rock), rocks = spots.filter(p => p.rock);
    const inst = (geo, list, mat, sc) => {
      const im = new THREE.InstancedMesh(geo, mat, Math.max(1, list.length)); const C = new THREE.Color();
      list.forEach((p, i) => { const w = mapToWorld(p.x, p.y); s.M4.compose(s.V.set(w[0], 0, w[2]), s.Q.setFromAxisAngle(s.UP, p.rot), s.S.setScalar(p.s * sc)); im.setMatrixAt(i, s.M4); im.setColorAt(i, C.setScalar(p.tint)); });
      im.count = list.length; im.computeBoundingSphere(); g.add(im); return im;
    };
    inst(treeG, trees, hideMat(), 1.45); inst(rockG, rocks, hideMat(), 1.7);
    // Static stone: base walls, the fountains, the pits' rims (one merged mesh).
    const parts = [];
    (map.nexus || []).forEach(nx => {
      const c = sideCols[nx.side];
      for (let i = 0; i < 26; i++) {
        const a = i / 26 * TAU, x = nx.x + Math.cos(a) * 21, y = nx.y + Math.sin(a) * 21;
        // Gaps where the lanes come in.
        const L = map.lanes || {};
        if (['top', 'mid', 'bot'].some(k => polyDist(x, y, L[k] || []) < 6.5)) continue;
        if (x < 0 || x > 100 || y < 0 || y > 100) continue;
        const w = mapToWorld(x, y);
        parts.push({ g: kit.G.box, c: '#6d6a60', m: kit.M(w[0], 1.1, w[2], 4.6, 2.2, 3.4, 0, -a, 0) });
        if (i % 3 === 0) parts.push({ g: kit.G.cyl, c, m: kit.M(w[0], 3.1, w[2], .5, 1.8, .5) });
      }
    });
    FOUNTAIN.forEach((f, side) => {
      const w = mapToWorld(f[0], f[1]), c = sideCols[side];
      parts.push({ g: kit.G.cap, c: '#8d8a80', m: kit.M(w[0], .2, w[2], 13, .4, 13) });
      parts.push({ g: kit.G.cap, c: shadeHex(c, .35), m: kit.M(w[0], .45, w[2], 8, .2, 8) });
      for (let i = 0; i < 6; i++) { const a = i / 6 * TAU; parts.push({ g: kit.G.box, c: '#b9b4a4', m: kit.M(w[0] + Math.cos(a) * 5.6, 2.2, w[2] + Math.sin(a) * 5.6, .8, 4.4, .8) }); }
    });
    for (const o of [map.dragon, map.baron]) {
      if (!o) continue;
      for (let i = 0; i < 28; i++) {
        const a = i / 28 * TAU; if (Math.cos(a - (o === map.dragon ? -2.36 : .79)) > .55) continue; // the entrance faces the river
        const w = mapToWorld(o.x + Math.cos(a) * 7.8, o.y + Math.sin(a) * 7.8);
        parts.push({ g: kit.G.box, c: '#5b5a50', m: kit.M(w[0], .3, w[2], 1.7, .6 + (i % 3) * .2, 1.2, 0, -a, 0) });
      }
    }
    g.add(new THREE.Mesh(kit.merge(parts), clearMat(new THREE.MeshLambertMaterial({ vertexColors: true }), 'a3lol-stone')));
    // Fountain glow.
    s.fountainGlow = FOUNTAIN.map((f, side) => { const w = mapToWorld(f[0], f[1]); const m = new THREE.Mesh(new THREE.CylinderGeometry(4, 4, 6, 24, 1, true), new THREE.MeshBasicMaterial({ color: kit.col(shadeHex(sideCols[side], .3)), transparent: true, opacity: .22, blending: THREE.AdditiveBlending, depthWrite: false, side: THREE.DoubleSide, fog: false })); m.position.set(w[0], 3, w[2]); g.add(m); return m; });
    // Structures: towers (instanced body and crystal), inhibitors and the nexus.
    const towers = map.towers || [];
    s.towers = towers;
    const bodyG = kit.merge([
      { g: kit.G.cap, c: '#5f6170', m: kit.M(0, .5, 0, 3.2, 1, 3.2) },
      { g: kit.G.cyl, c: '#8b8f9c', m: kit.M(0, 3.2, 0, 1.9, 4.6, 1.9) },
      { g: kit.G.cyl, c: '#a7abb7', m: kit.M(0, 5.7, 0, 2.6, .5, 2.6) },
      { g: kit.G.box, c: '#6f7280', m: kit.M(.9, 6.4, 0, .35, 1.2, .35) }, { g: kit.G.box, c: '#6f7280', m: kit.M(-.9, 6.4, 0, .35, 1.2, .35) },
      { g: kit.G.box, c: '#6f7280', m: kit.M(0, 6.4, .9, .35, 1.2, .35) }, { g: kit.G.box, c: '#6f7280', m: kit.M(0, 6.4, -.9, .35, 1.2, .35) },
    ]);
    const crystalG = kit.merge([
      { g: kit.G.cyl, c: '#ffffff', m: kit.M(0, 4.6, 0, 1.95, .5, 1.95) },
      { g: new THREE.OctahedronGeometry(.8, 0), c: '#ffffff', m: kit.M(0, 7.3, 0, 1, 1.5, 1) },
    ]);
    s.towerBody = new THREE.InstancedMesh(bodyG, clearMat(new THREE.MeshLambertMaterial({ vertexColors: true }), 'a3lol-tower'), Math.max(1, towers.length)); s.towerBody.frustumCulled = false;
    s.towerCrys = new THREE.InstancedMesh(crystalG, new THREE.MeshBasicMaterial({ color: 0xffffff }), Math.max(1, towers.length)); s.towerCrys.frustumCulled = false;
    towers.forEach((tw, i) => s.towerCrys.setColorAt(i, kit.col(shadeHex(sideCols[tw.side], .25))));
    g.add(s.towerBody, s.towerCrys);
    // Rubble chunks of every structure (shown once it falls).
    const rub = [], R = rng(5);
    const addRubble = (x, y, n, sz) => { const out = []; for (let k = 0; k < n; k++) { const a = R() * TAU, d = R() * sz; const w = mapToWorld(x, y); out.push({ x: w[0] + Math.cos(a) * d, z: w[2] + Math.sin(a) * d, s: .5 + R() * .9, r: R() * 3, h: R() }); } rub.push(...out); return out; };
    s.towerRubble = towers.map(tw => addRubble(tw.x, tw.y, 7, 2.6));
    s.inhibs = (map.inhibitors || []).map(ih => ({ o: ih, rubble: addRubble(ih.x, ih.y, 5, 2) }));
    s.nexus = (map.nexus || []).map(nx => ({ o: nx, rubble: addRubble(nx.x, nx.y, 9, 3.5) }));
    s.rubbleMesh = new THREE.InstancedMesh(new THREE.DodecahedronGeometry(.6, 0), kit.mat('#5d5a54'), Math.max(1, rub.length)); s.rubbleMesh.frustumCulled = false; s.rubbleMesh.count = 0; g.add(s.rubbleMesh);
    const crysMat = side => new THREE.MeshBasicMaterial({ color: kit.col(shadeHex(sideCols[side], .2)) });
    for (const it of s.inhibs) {
      const w = mapToWorld(it.o.x, it.o.y);
      const base = new THREE.Mesh(kit.merge([{ g: kit.G.cap, c: '#5d6170', m: kit.M(0, .3, 0, 3.6, .6, 3.6) }, { g: kit.G.cap, c: '#3f4250', m: kit.M(0, .7, 0, 2.6, .3, 2.6) }]), kit.mats.vc);
      base.position.set(w[0], 0, w[2]); g.add(base);
      it.crys = new THREE.Mesh(new THREE.OctahedronGeometry(1, 0), crysMat(it.o.side)); it.crys.scale.set(1, 1.6, 1); it.crys.position.set(w[0], 2.6, w[2]); g.add(it.crys);
      it.w = w;
    }
    for (const it of s.nexus) {
      const w = mapToWorld(it.o.x, it.o.y);
      const base = new THREE.Mesh(kit.merge([{ g: kit.G.cap, c: '#5b5f6d', m: kit.M(0, .5, 0, 7.5, 1, 7.5) }, { g: kit.G.cap, c: '#454856', m: kit.M(0, 1.2, 0, 5.2, .6, 5.2) }]), kit.mats.vc);
      base.position.set(w[0], 0, w[2]); g.add(base);
      it.crys = new THREE.Mesh(new THREE.OctahedronGeometry(1.8, 0), crysMat(it.o.side)); it.crys.scale.set(1, 1.7, 1); it.crys.position.set(w[0], 5.2, w[2]); g.add(it.crys);
      it.ring = new THREE.Mesh(new THREE.TorusGeometry(3.4, .14, 6, 40), new THREE.MeshBasicMaterial({ color: kit.col(shadeHex(sideCols[it.o.side], .5)) })); it.ring.position.set(w[0], 5.2, w[2]); g.add(it.ring);
      it.w = w;
    }
    // Monsters (built once, posed per frame).
    s.mon = this.monsters(ctx, g, map);
    scene.add(g);
    s.riftKey = riftKey(st);
  },

  monsters(ctx, g, map) {
    const { THREE, kit } = ctx, M = kit.M, G = kit.G;
    const mk = parts => new THREE.Mesh(kit.merge(parts), kit.mats.vc);
    const out = {};
    // Dragon (geometry is white-ish: the material's colour gives the element).
    const dragon = new THREE.Group(), dmat = new THREE.MeshLambertMaterial({ vertexColors: true, color: 0xff8a3d });
    const body = new THREE.Mesh(kit.merge([
      { g: G.sph, c: '#ffffff', m: M(0, 2.2, 0, 4.4, 2.2, 2.3) },
      { g: G.sph, c: '#d9d2c0', m: M(.3, 1.9, 0, 3.4, 1.6, 1.8) },
      { g: G.cyl, c: '#ffffff', m: M(2.2, 3.3, 0, .9, 2.4, .9, 0, 0, -.75) },
      { g: G.sph, c: '#ffffff', m: M(3.2, 4.3, 0, 1.6, .9, 1) },
      { g: G.box, c: '#f2e7c9', m: M(3.95, 4.05, 0, .9, .3, .7) },
      { g: G.cone, c: '#f2e7c9', m: M(2.9, 4.9, .3, .25, .8, .25, -.3, 0, .5) }, { g: G.cone, c: '#f2e7c9', m: M(2.9, 4.9, -.3, .25, .8, .25, .3, 0, .5) },
      { g: G.cone, c: '#ffffff', m: M(-3.4, 1.9, 0, .9, 3.2, .9, 0, 0, 1.35) },
      ...[[1.2, .8], [1.2, -.8], [-1.2, .8], [-1.2, -.8]].map(([x, z]) => ({ g: G.cyl, c: '#bdb6a6', m: M(x, .6, z, .55, 1.4, .55) })),
    ]), dmat);
    dragon.add(body);
    const wing = side => { const w = new THREE.Group(); w.position.set(.2, 3.2, side * 1); const m = new THREE.Mesh(kit.merge([{ g: G.box, c: '#ffffff', m: M(0, 0, side * 2.2, 2.6, .12, 4.4) }, { g: G.box, c: '#d0c8b4', m: M(-.6, -.1, side * 3.4, 1.6, .1, 2.2) }]), dmat); w.add(m); dragon.add(w); return w; };
    dragon.wings = [wing(1), wing(-1)];
    out.dragon = { g: dragon, mat: dmat };
    // Baron Nashor: a purple serpent rising from the pit.
    const baron = new THREE.Group();
    const segs = [];
    for (let i = 0; i < 6; i++) { const k = i / 5; segs.push({ g: G.sph, c: i % 2 ? '#4b2273' : '#5b2a86', m: M(-1.6 + k * 2.4, 1 + k * 5.4, 0, 3.4 - k * 1.1, 2.6, 3.2 - k * 1) }); }
    baron.add(mk([
      ...segs,
      { g: G.sph, c: '#6d33a1', m: M(1.6, 7.6, 0, 3.2, 2, 2.4) },
      { g: G.box, c: '#2b123f', m: M(2.9, 7, 0, 1.4, .6, 1.8) },
      { g: G.sph, c: '#ffe05a', m: M(2.6, 8.1, .75, .45, .35, .35) }, { g: G.sph, c: '#ffe05a', m: M(2.6, 8.1, -.75, .45, .35, .35) },
      ...[-1, 1].map(z => ({ g: G.cone, c: '#d8b7ff', m: M(.6, 9, z * 1, .5, 2.2, .5, z * .5, 0, .6) })),
      ...[0, 1, 2, 3, 4].map(i => ({ g: G.cone, c: '#b77bff', m: M(-2.4 + i * .9, 2.2 + i * 1.4, 0, .45, 1.6, .45, 0, 0, .7) })),
    ]));
    out.baron = { g: baron };
    // Rift Herald: a shelled crab with one big eye.
    const herald = new THREE.Group();
    herald.add(mk([
      { g: G.hemi, c: '#5b2f87', m: M(0, .9, 0, 4, 3.2, 3.6) },
      { g: G.sph, c: '#3c1d5c', m: M(0, .9, 0, 4.2, .7, 3.8) },
      { g: G.sph, c: '#f3ecff', m: M(1.6, 1.9, 0, 1.3, 1.3, 1.3) }, { g: G.sph, c: '#b04dff', m: M(2.15, 1.95, 0, .5, .7, .7) },
      ...[-1, 1].flatMap(z => [-1, 0, 1].map(x => ({ g: G.cyl, c: '#2d1745', m: M(x * 1.1, .4, z * 1.9, .3, 1.1, .3, z * .5, 0, 0) }))),
    ]));
    out.herald = { g: herald };
    // Void Grubs: three little bugs.
    const grubs = new THREE.Group(), grubG = kit.merge([{ g: G.sph, c: '#6a3aa0', m: M(0, .7, 0, 1.8, 1.2, 1.3) }, { g: G.sph, c: '#d6b5ff', m: M(.85, .85, 0, .6, .5, .6) }, ...[0, 1, 2].map(i => ({ g: G.cone, c: '#b884ff', m: M(-.5 + i * .5, 1.35, 0, .2, .55, .2) }))]);
    out.grubList = [[-1.6, -1.2], [1.4, -1.6], [0, 1.8]].map(([x, z]) => { const m = new THREE.Mesh(grubG, kit.mats.vc); m.position.set(x, 0, z); grubs.add(m); return m; });
    out.grubs = { g: grubs };
    for (const k of ['dragon', 'baron', 'herald', 'grubs']) { out[k].g.visible = false; g.add(out[k].g); }
    out.dragonAt = mapToWorld(map.dragon.x, map.dragon.y); out.baronAt = mapToWorld(map.baron.x, map.baron.y);
    return out;
  },

  // A champion model (rebuilt if the pick changes, e.g. after a seek into the draft).
  model(ctx, st, t, r) {
    const s = ctx.state, l = st.line[t][r] || {}, key = `${t}:${r}:${l.champ || ''}:${teamColor(st.meta, t)}`;
    let m = s.models.get(champId(t, r));
    if (m && m.key === key) return m;
    if (m) ctx.scene.remove(m.h.root);
    const info = champInfo(st.meta, l.champ), cls = KIT_CLASS[info.class] || 'fighter';
    const seed = [...String(l.champ || l.handle || r)].reduce((a, c) => a * 31 + c.charCodeAt(0) >>> 0, 7);
    const h = ctx.kit.champion({ cls, color: teamColor(st.meta, t), color2: CLASS_COLORS[info.class] || '#ffd400', skin: SKIN[seed % SKIN.length], height: cls === 'tank' ? 2.7 : cls === 'assassin' || cls === 'marksman' ? 2.3 : 2.45 });
    ctx.scene.add(h.root);
    m = { h, key, cls: info.class, dmg: info.dmg };
    s.models.set(champId(t, r), m);
    return m;
  },

  update(ctx, frame) {
    const s = ctx.state, v = ctx.view, st = v.state, { THREE, kit, camera } = ctx;
    s.dt = frame.dt;
    if (!st || !st.map) { s.ready = false; return; }
    if (s.riftKey !== riftKey(st)) this.rift(ctx, st);
    s.ready = true;
    const src = frameSource(v);
    // A seek (a new state), a replay starting or ending, time going back: snap, no smoothing.
    const cut = s.stateRef !== st || s.wasReplay !== src.replay || src.anim < s.lastAnim - 1e-6;
    if (cut) { s.cut = true; s.corpse.clear(); s.towerFall.clear(); s.structAlive.clear(); s.face.clear(); s.prevSpot.clear(); }
    s.stateRef = st; s.wasReplay = src.replay;
    const dA = cut ? 0 : Math.max(0, src.anim - s.lastAnim);
    s.lastAnim = src.anim; s.src = src;
    const watch = ctx.watch || {}, me = parseId(watch.id);
    s.waterTex.offset.y -= frame.dt * .05; s.waterTex.offset.x = Math.sin(frame.clock * .3) * .02;
    for (const m of s.fountainGlow || []) m.material.opacity = .16 + .08 * Math.sin(frame.clock * 2);
    const fx = s.fx; for (const k in fx) fx[k].begin();
    s.bars.begin(camera); s.text.begin(); s.labels.begin();
    const camPos = camera.position, near = (w, r = 95) => Math.hypot(w[0] - camPos.x, w[2] - camPos.z) < r;
    let sh = 0;

    // ---- champions ----
    s.spots = {};
    const held = src.shot && src.shot.struct;
    for (let t = 0; t < 2; t++) for (let r = 0; r < 5; r++) {
      const id = champId(t, r), m = this.model(ctx, st, t, r), h = m.h, p = src.pos[t] && src.pos[t][r];
      if (!p) { h.root.visible = false; continue; }
      const alive = p.alive !== false, [mx, my] = champSpot(src, t, r);
      // Dying: the body falls where it stood and stays a few seconds, then sinks.
      let corpse = s.corpse.get(id);
      if (!alive && !corpse && !cut && s.prevSpot.has(id)) { corpse = { at: src.anim, x: s.prevSpot.get(id)[0], y: s.prevSpot.get(id)[1], yaw: s.face.get(id) || 0 }; s.corpse.set(id, corpse); }
      if (alive && corpse) { s.corpse.delete(id); corpse = null; }
      if (alive) s.prevSpot.set(id, [mx, my]);
      const respawn = Math.max(0, Math.ceil((st.dead[t][r] || 0) - src.clock));
      if (!alive) {
        const age = corpse ? src.anim - corpse.at : 99;
        const fw = corpse ? mapToWorld(corpse.x, corpse.y) : mapToWorld(p.x, p.y);
        s.spots[id] = { w: fw, yaw: corpse ? corpse.yaw : 0, dead: true, corpse: age < 4.6, respawn, fountain: mapToWorld(...FOUNTAIN[mapSide(st.meta, t)]) };
        if (age < 4.6) {
          h.root.visible = true;
          h.root.position.set(fw[0], -clamp((age - 3.6) * 1.6, 0, 1.6), fw[2]);
          h.pose({ yaw: corpse.yaw, fall: { k: clamp(age / .5, 0, 1), dir: [-Math.cos(corpse.yaw), Math.sin(corpse.yaw)] }, firstPerson: watch.mode === 'first' && s.eye && me && me.t === t && me.r === r });
          s.shadows.set(sh++, fw[0], fw[2], 1.1, 1);
        } else h.root.visible = false;
        const lw = age < 4.6 ? fw : mapToWorld(p.x, p.y);
        if (near(lw, 70)) s.labels.put(id, `${(st.line[t][r] || {}).champ || ''} ☠ ${respawn}`, '#ff9a9a', lw[0], 1.6, lw[2], { size: .6, alpha: .9 });
        continue;
      }
      h.root.visible = true;
      const w = mapToWorld(mx, my);
      // Facing: the fight it is in, the nearest enemy, else where it walks.
      let tgtYaw = null;
      const tg = st.target[t][r];
      if (src.shot && Math.hypot(mx - src.shot.x, my - src.shot.y) < 20 && Math.hypot(mx - src.shot.x, my - src.shot.y) > 1.5) tgtYaw = headingYaw(src.shot.x - mx, src.shot.y - my);
      if (tgtYaw == null && tg && Math.hypot(tg.x - p.x, tg.y - p.y) > 1.2) tgtYaw = headingYaw(tg.x - p.x, tg.y - p.y);
      if (tgtYaw == null) {
        let bd = 9, bx = 0, by = 0;
        for (let r2 = 0; r2 < 5; r2++) { if (!isAliveAt(src.pos, 1 - t, r2)) continue; const [ex, ey] = champSpot(src, 1 - t, r2), d = Math.hypot(ex - mx, ey - my); if (d < bd) { bd = d; bx = ex; by = ey; } }
        if (bd < 9) tgtYaw = headingYaw(bx - mx, by - my);
      }
      const prevYaw = s.face.get(id);
      const yaw = prevYaw == null ? (tgtYaw != null ? tgtYaw : headingYaw(50 - mx, 50 - my)) : tgtYaw == null ? prevYaw : dampAngle(prevYaw, tgtYaw, 7, Math.max(dA, frame.dt * .25));
      s.face.set(id, yaw);
      // Walking speed from how far the spot moved in match time.
      const last = s.lastW && s.lastW.get(id);
      const dist = last && dA > 0 ? Math.hypot(w[0] - last[0], w[2] - last[2]) : 0;
      (s.lastW || (s.lastW = new Map())).set(id, w);
      const speed = dA > 0 ? dist / dA : 0, run = clamp(speed / 6, 0, 1) * (dA > 0 ? 1 : 0);
      const ph = (s.phase.get(id) || r * 1.3) + dist * 1.5; s.phase.set(id, ph);
      // Knocked up.
      const airT = src.anim - ((src.air && src.air[t] && src.air[t][r]) || -9), lift = airT >= 0 && airT < .6 ? Math.sin(airT / .6 * Math.PI) * 1.6 : 0;
      // Casting: arms reach towards the target of the ability just thrown.
      let arms = null;
      for (const a of src.acts) {
        if (a.age < 0 || a.age > .35) continue;
        if ((a.k === 'proj' && Math.hypot(a.x0 - mx, a.y0 - my) < 2.2) || (a.k === 'dash' && a.t === t && a.r === r)) {
          const tx = a.x1, ty = a.y1, aw = mapToWorld(tx, ty, 1.5);
          arms = [m.cls === 'marksman' || m.cls === 'mage' ? aw : null, aw];
          break;
        }
      }
      h.root.position.set(w[0], lift, w[2]);
      const mine = watch.mode === 'first' && me && me.t === t && me.r === r;
      // Someone right in front of the lens (not the one we follow) steps out of the picture.
      if (!mine && watch.mode === 'first' && Math.hypot(w[0] - camPos.x, w[2] - camPos.z) < 2.4) h.root.visible = false;
      h.pose({ yaw, phase: ph, run, lean: run * .3, arms, jump: lift, firstPerson: mine && s.eye });
      s.shadows.set(sh++, w[0], w[2], .9, 1);
      s.spots[id] = { w: [w[0], 0, w[2]], yaw, map: [mx, my], eyeH: h.S * 2 * .9 };
      if (!near(w)) continue;
      const top = h.S * 2 + .35 + lift;
      const col = teamColor(st.meta, t), hp = src.hp ? src.hp[t][r] : 1;
      if (!(mine && s.eye)) {
        if (!mine) s.bars.add(w[0], top + .25, w[2], 1.9, hp, hp > .5 ? col : hp > .25 ? '#f0b13a' : '#ef3b3b', .2);
        if (!mine) s.labels.put(id, (st.line[t][r] || {}).champ || (st.line[t][r] || {}).handle || '', '#ffffff', w[0], top + .25, w[2], { size: .62, alpha: .95 });
      }
      // Buff auras.
      if (st.buff[t].baron > src.clock) fx.ring.add(w[0], .08, w[2], null, 1.5, 1, 1.5, '#a25bff', .7 + .3 * Math.sin(frame.clock * 5));
      if (st.buff[t].elder > src.clock) fx.ring.add(w[0], .1, w[2], null, 1.8, 1, 1.8, '#f5e17a', .8);
      fx.ring.add(w[0], .05, w[2], null, 1.05, 1, 1.05, col, mine ? .9 : .45);
    }

    // ---- minion waves (the 2D viewer's formula) ----
    let mi = 0;
    if (st.mode === 'map') {
      const L = st.map.lanes || {}, Cm = new THREE.Color();
      for (const lane of ['top', 'mid', 'bot']) {
        const path = L[lane]; if (!path) continue;
        const f = waveFront(st, lane);
        for (let side = 0; side < 2; side++) {
          const col = s.sideCols[side], sup = st.inhibs[`${1 - side}-${lane}`] === false;
          for (let wv = 0; wv < 3; wv++) {
            const ph = ((src.anim * .09 + wv / 3 + (lane === 'mid' ? .1 : lane === 'top' ? .2 : 0)) % 1);
            const sPos = side === 0 ? ph * f : 1 - ph * (1 - f);
            const fade = ph > .85 ? (1 - ph) / .15 : ph < .08 ? ph / .08 : 1;
            for (let i = 0; i < 4 && mi < 96; i++) {
              const off = (side === 0 ? -1 : 1) * i * .012, p = lanePoint(path, sPos + off), p2 = lanePoint(path, sPos + off + (side === 0 ? .01 : -.01));
              const fighting = ph > .85, jx = fighting ? Math.sin(src.anim * 20 + i) * .5 : 0;
              const mx = p[0] + (i % 2 ? .8 : -.8) + jx, my = p[1] + (i % 2 ? -.8 : .8), w = mapToWorld(mx, my);
              if (!near(w, 120)) continue;
              const big = sup && i === 0 ? 1.7 : 1, sc = big * clamp(fade * 1.4, .05, 1);
              const yawM = fighting ? headingYaw(side === 0 ? 1 : -1, side === 0 ? -1 : 1) : headingYaw(p2[0] - p[0], p2[1] - p[1]);
              s.M4.compose(s.V.set(w[0], Math.abs(Math.sin(src.anim * 9 + i + wv)) * .12, w[2]), s.Q.setFromAxisAngle(s.UP, yawM), s.S.setScalar(sc));
              s.minions.setMatrixAt(mi, s.M4); s.minions.setColorAt(mi, Cm.copy(kit.col(col)).lerp(kit.col('#ffffff'), .25)); mi++;
              if (sh < 99) s.shadows.set(sh++, w[0], w[2], .55 * big, fade);
            }
          }
        }
      }
    }
    s.minions.count = mi; s.minions.instanceMatrix.needsUpdate = true; if (s.minions.instanceColor) s.minions.instanceColor.needsUpdate = true;

    // ---- structures ----
    const isHeld = (kind, o) => !!held && Number.isFinite(held.dieAt) && src.anim < held.dieAt && held.kind === kind && held.side === o.side && (kind === 'nexus' || held.lane === o.lane) && (kind !== 'tower' || held.tier === o.tier);
    let ri = 0;
    const rubble = (list, k) => { for (const c of list) { if (ri >= s.rubbleMesh.instanceMatrix.count) break; s.M4.compose(s.V.set(c.x, .15 + c.h * .3 * k, c.z), s.Q.setFromAxisAngle(s.UP, c.r), s.S.setScalar(c.s * k)); s.rubbleMesh.setMatrixAt(ri++, s.M4); } };
    const fall = (key, up) => {
      // Was standing, now down: it collapses over 1.2 s of match time (not after a seek).
      const was = s.structAlive.get(key);
      s.structAlive.set(key, up);
      if (was === true && !up && !cut) s.towerFall.set(key, src.anim);
      if (up) { s.towerFall.delete(key); return -1; }
      const t0 = s.towerFall.get(key);
      return t0 == null ? 1 : clamp((src.anim - t0) / 1.2, 0, 1);
    };
    (s.towers || []).forEach((tw, i) => {
      const hold = isHeld('tower', tw), up = hold || st.towers[`${tw.side}-${tw.lane}-${tw.tier}`] !== false;
      const k = fall('t' + i, up), w = mapToWorld(tw.x, tw.y);
      if (k < 0) {
        s.M4.compose(s.V.set(w[0], 0, w[2]), s.Q.identity(), s.S.setScalar(1));
        const hp = hold ? drainHp(held, src.anim) : (v.towerHp ? v.towerHp(tw) : 1);
        if (near(w, 110)) s.bars.add(w[0], 9.2, w[2], 3.4, hp, hp > .6 ? s.sideCols[tw.side] : hp > .4 ? '#f0b13a' : '#ef5350', .32);
      } else if (k < 1) {
        // Collapse: tilt away from the lane, sink, throw dust.
        s.AX.set(Math.cos(i * 2.1), 0, Math.sin(i * 2.1));
        s.M4.compose(s.V.set(w[0], -k * k * 5, w[2]), s.Q.setFromAxisAngle(s.AX, k * k * 1.2), s.S.set(1, 1 - k * .3, 1));
        for (let d = 0; d < 6; d++) { const a = d / 6 * TAU + i; fx.chunk.add(w[0] + Math.cos(a) * (1 + k * 4), .4 + Math.sin(k * Math.PI) * (2 + d % 3), w[2] + Math.sin(a) * (1 + k * 4), null, .5, .5, .5, '#7a746a'); }
        fx.disc.add(w[0], .12, w[2], null, 3 + k * 5, 1, 3 + k * 5, '#d8c9a8', .35 * (1 - k));
        rubble(s.towerRubble[i], k);
      } else { s.M4.makeScale(0, 0, 0); rubble(s.towerRubble[i], 1); }
      s.towerBody.setMatrixAt(i, s.M4); s.towerCrys.setMatrixAt(i, s.M4);
    });
    if (s.towerBody) { s.towerBody.instanceMatrix.needsUpdate = true; s.towerCrys.instanceMatrix.needsUpdate = true; }
    for (const it of s.inhibs || []) {
      const hold = isHeld('inhib', it.o), up = hold || st.inhibs[`${it.o.side}-${it.o.lane}`] !== false;
      const k = fall('i' + it.o.side + it.o.lane, up);
      it.crys.visible = k < 1; it.crys.rotation.y = frame.clock * .8; it.crys.position.y = 2.6 + Math.sin(frame.clock * 2 + it.o.x) * .25 - (k > 0 ? k * 2.4 : 0);
      if (k < 0 && hold && near(it.w)) s.bars.add(it.w[0], 5.2, it.w[2], 3, drainHp(held, src.anim), s.sideCols[it.o.side], .3);
      if (k >= 0) rubble(it.rubble, Math.max(k, .01));
    }
    for (const it of s.nexus || []) {
      const hold = isHeld('nexus', it.o), up = hold || st.nexus[it.o.side];
      const k = fall('n' + it.o.side, up);
      it.crys.visible = it.ring.visible = k < 1; it.crys.rotation.y = frame.clock * .4; it.ring.rotation.x = Math.PI / 2 + Math.sin(frame.clock) * .3; it.ring.rotation.y = frame.clock;
      if (k > 0) { it.crys.scale.set(1 + k, 1.7 * (1 - k), 1 + k); fx.orb.add(it.w[0], 5, it.w[2], null, 4 + k * 20, 4 + k * 20, 4 + k * 20, '#ffffff', 1 - k); }
      if (k < 0 && hold && near(it.w)) s.bars.add(it.w[0], 10, it.w[2], 5, drainHp(held, src.anim), s.sideCols[it.o.side], .38);
      if (k >= 0) rubble(it.rubble, Math.max(k, .01));
    }
    if (s.rubbleMesh) { s.rubbleMesh.count = ri; s.rubbleMesh.instanceMatrix.needsUpdate = true; }

    // ---- monsters ----
    this.poseMonsters(ctx, st, src, frame, near);

    // ---- ability effects, flashes, debris, numbers ----
    this.effects(ctx, st, src, frame, near);

    for (let i = sh; i < 100; i++) s.shadows.hide(i);
    s.shadows.commit();
    for (const k in fx) fx[k].end();
    s.bars.end(); s.text.end(); s.labels.end();

    // The jungle steps aside for the over-the-shoulder camera.
    const ws = me && s.spots[watch.id];
    if (s.hideU) { s.hideU.uHide.value = watch.mode === 'first' && ws ? 1 : 0; if (ws) s.hideU.uFocus.value.set(ws.w[0], 0, ws.w[2]); }
    // The free camera orbits the fight, else the watched champion, else the map centre.
    if (src.shot && Number.isFinite(src.shot.x)) s.focus = mapToWorld(src.shot.x, src.shot.y, 1.2);
    else if (ws) s.focus = [ws.w[0], 1.2, ws.w[2]];
    else s.focus = [0, 1, 0];
  },

  poseMonsters(ctx, st, src, frame, near) {
    const s = ctx.state, M = s.mon, fx = s.fx, c = src.clock, o = st.obj || {}, sm = src.shot && src.shot.mon;
    if (!M) return;
    const heldKind = sm && Number.isFinite(sm.dieAt) && src.anim < sm.dieAt + .9 ? sm.kind : null;
    const dk = heldKind ? clamp((src.anim - sm.dieAt) / .9, 0, 1) : 0;
    // Which monsters are up (the 2D pit badges' rules), plus the one filmed dying.
    const soul = st.soul && (st.soul[0] || st.soul[1]);
    const drakeUp = soul ? c >= o.elderNext && Number.isFinite(o.elderNext) && o.elderNext > 0 : Number.isFinite(o.dragonNext) && c >= o.dragonNext;
    const grubsUp = c >= 360 && c < 840 && !o.grubsTaken, heraldUp = c >= 840 && c < 1185 && !o.heraldTaken && c < o.baronNext, baronUp = c >= o.baronNext;
    const pitHeld = heldKind === 'baron' || heldKind === 'herald' || heldKind === 'grubs';
    const show = { dragon: heldKind === 'drake' || heldKind === 'elder' || drakeUp, baron: pitHeld ? heldKind === 'baron' : baronUp, herald: pitHeld ? heldKind === 'herald' : heraldUp, grubs: pitHeld ? heldKind === 'grubs' : grubsUp };
    const filmed = k => heldKind && ((k === 'dragon' && (heldKind === 'drake' || heldKind === 'elder')) || k === heldKind);
    const t = frame.clock;
    // The dragon: its element, breathing, wings beating.
    {
      const D = M.dragon, on = show.dragon && st.mode !== 'draft';
      D.g.visible = on;
      if (on) {
        const types = st.dragons ? st.dragons[0].concat(st.dragons[1]) : [];
        const col = filmed('dragon') ? sm.col : soul ? DRAGON_COLORS.elder : (types.length >= 3 ? DRAGON_COLORS[types[types.length - 1]] : null) || '#ff8a3d';
        if (D.col !== col) { D.col = col; D.mat.color.copy(ctx.kit.col(col)); }
        const k = filmed('dragon') ? dk : 0, w = M.dragonAt;
        D.g.position.set(w[0], Math.sin(t * 1.4) * .15 - k * 3, w[2]);
        D.g.rotation.y = headingYaw(-1, -1) + Math.sin(t * .4) * .25; D.g.scale.setScalar((soul ? 1.25 : 1) * (1 - .3 * k));
        D.g.wings[0].rotation.x = -.3 + Math.sin(t * 3.2) * .55; D.g.wings[1].rotation.x = .3 - Math.sin(t * 3.2) * .55;
        if (near(w) && !filmed('dragon')) s.bars.add(w[0], 7.2, w[2], 5, 1, col, .36);
        if (filmed('dragon') && k > 0) fx.orb.add(w[0], 3, w[2], null, 6 + k * 10, 6 + k * 10, 6 + k * 10, '#ffffff', .8 * (1 - k));
      }
    }
    const pose = (key, g, at, y, hpCol, barH) => {
      const on = show[key] && st.mode !== 'draft';
      g.visible = on; if (!on) return;
      const k = filmed(key) ? dk : 0;
      g.position.set(at[0], y - k * 4, at[2]); g.scale.setScalar(1 - .3 * k);
      if (near(at) && !filmed(key)) s.bars.add(at[0], barH, at[2], key === 'baron' ? 6 : 4, 1, hpCol, .36); // the filmed one has the boss bar
      if (filmed(key) && k > 0) fx.orb.add(at[0], 3, at[2], null, 6 + k * 10, 6 + k * 10, 6 + k * 10, '#ffffff', .8 * (1 - k));
    };
    pose('baron', M.baron.g, M.baronAt, 0, '#9b59ff', 10.4);
    M.baron.g.rotation.y = headingYaw(1, 1) + Math.sin(t * .6) * .2; M.baron.g.rotation.z = Math.sin(t * .9) * .05;
    pose('herald', M.herald.g, M.baronAt, 0, '#c48bff', 5.6);
    M.herald.g.rotation.y = headingYaw(1, 1) + Math.sin(t * .8) * .3;
    pose('grubs', M.grubs.g, M.baronAt, 0, '#a78bfa', 4);
    M.grubList.forEach((g, i) => { g.position.y = Math.abs(Math.sin(t * 3 + i * 2)) * .3; g.rotation.y = t * .5 + i * 2; });
  },

  effects(ctx, st, src, frame, near) {
    const s = ctx.state, fx = s.fx, H = 1.4;
    const W = (x, y, h = 0) => mapToWorld(x, y, h);
    const spotOf = (t, r) => { const p = s.spots[champId(t, r)]; return p && !p.dead ? p.w : null; };
    const beam = (a, b, w, col, f) => {
      const dx = b[0] - a[0], dy = b[1] - a[1], dz = b[2] - a[2], len = Math.hypot(dx, dy, dz);
      if (len < 1e-3) return;
      s.AX.set(dx / len, dy / len, dz / len); s.Q.setFromUnitVectors(s.UP, s.AX);
      fx.beam.add(a[0], a[1], a[2], s.Q, w, len, w, col, f);
    };
    const nums = [];
    for (const a of src.acts) {
      if (a.age < 0) continue;
      const k = clamp(a.age / a.life, 0, 1), fade = 1 - k;
      switch (a.k) {
        case 'aoe': {
          const w = W(a.x, a.y), r = a.r * K;
          if (a.age < a.hitAt) { const g = a.age / a.hitAt; fx.ring.add(w[0], .1, w[2], null, r, 1, r, a.team, .9); fx.disc.add(w[0], .08, w[2], null, r, 1, r, a.team, .18); fx.disc.add(w[0], .09, w[2], null, r * g, 1, r * g, a.team, .3); }
          else { const g = (a.age - a.hitAt) / (a.life - a.hitAt); fx.disc.add(w[0], .1, w[2], null, r * (1 + .3 * g), 1, r * (1 + .3 * g), a.team, .9 * (1 - g)); fx.orb.add(w[0], .6, w[2], null, r * .8, 1.6 * (1 - g) + .2, r * .8, '#ffffff', .5 * (1 - g)); }
          break;
        }
        case 'slam': {
          const w = W(a.x, a.y), r = a.r * K * (.3 + 1.1 * easeOut(k));
          fx.ring.add(w[0], .12, w[2], null, r, 1, r, a.team, fade);
          if (k < .3) fx.disc.add(w[0], .1, w[2], null, r * .5, 1, r * .5, '#ffffff', (.3 - k) * 3);
          break;
        }
        case 'spin': {
          const w = W(a.x, a.y), r = a.r * K;
          for (let i = 0; i < 3; i++) { s.Q.setFromAxisAngle(s.UP, k * TAU * 1.5 + i * TAU / 3).multiply(s.QX); fx.arc.add(w[0], .9 + i * .25, w[2], s.Q, r, r, 1, i ? a.team : '#ffffff', fade); }
          break;
        }
        case 'proj': {
          const px = lerp(a.x0, a.x1, k), py = lerp(a.y0, a.y1, k), w = W(px, py, H);
          if (!near(w)) break;
          if (a.style === 'arrow') { const kk = Math.max(0, k - .25); beam(W(lerp(a.x0, a.x1, kk), lerp(a.y0, a.y1, kk), H), w, .06, '#fff2b0', .9); fx.orb.add(w[0], w[1], w[2], null, .2, .2, .2, '#ffffff', 1); }
          else if (a.style === 'hook') { beam(W(a.x0, a.y0, H), w, .08, '#c9ccd6', .9); fx.orb.add(w[0], w[1], w[2], null, .7, .7, .7, '#eef0f6', .9); }
          else {
            const r = a.big ? 1.1 : .8;
            for (let i = 4; i >= 1; i--) { const kk = Math.max(0, k - i * .06), tw = W(lerp(a.x0, a.x1, kk), lerp(a.y0, a.y1, kk), H); fx.orb.add(tw[0], tw[1], tw[2], null, r * (1 - i * .15), r * (1 - i * .15), r * (1 - i * .15), a.team, .25 * (5 - i) / 4); }
            fx.orb.add(w[0], w[1], w[2], null, r, r, r, a.col || '#fff2c0', 1); fx.orb.add(w[0], w[1], w[2], null, r * .5, r * .5, r * .5, '#ffffff', 1);
          }
          break;
        }
        case 'slash': {
          const w = W(a.x, a.y), r = (a.big ? 2.6 : 1.9) * (.7 + .5 * easeOut(k));
          s.Q.setFromAxisAngle(s.UP, a.ang + k * 1.2);
          fx.arc.add(w[0], 1.3, w[2], s.Q, r, r, 1, '#ffffff', fade); fx.arc.add(w[0], 1.3, w[2], s.Q, r * .8, r * .8, 1, a.team, fade);
          break;
        }
        case 'dash': {
          const a0 = W(a.x0, a.y0, 1.1), a1 = W(a.x1, a.y1, 1.1);
          beam(a0, a1, .22 * fade + .05, a.col, .6 * fade);
          for (let i = 1; i <= 3; i++) { const f = i / 4, p = W(lerp(a.x0, a.x1, f), lerp(a.y0, a.y1, f), 1.1); fx.orb.add(p[0], p[1], p[2], null, .7, 1.3, .7, a.col, .2 * fade * f); }
          break;
        }
        case 'shield': case 'heal': {
          const p = spotOf(a.t, a.r); if (!p) break;
          const al = Math.min(1, a.age * 6) * Math.min(1, (a.life - a.age) * 3);
          if (a.k === 'shield') { fx.orb.add(p[0], 1.3, p[2], null, 3, 3.4, 3, '#ffe08a', .12 * al); fx.ring.add(p[0], 1.3 + Math.sin(frame.clock * 4) * .5, p[2], null, 1.6, 1, 1.6, '#fff1b8', .8 * al); }
          else for (let i = 0; i < 5; i++) { const ph = (k + i / 5) % 1; fx.orb.add(p[0] + Math.sin(i * 2.3) * .9, .3 + ph * 2.8, p[2] + Math.cos(i * 2.3) * .9, null, .25, .25, .25, '#62ff96', al); }
          break;
        }
        case 'beam': {
          const p = spotOf(a.t, a.r); if (!p) break;
          const o = W(a.x0, a.y0, 7.3); beam(o, [p[0], 1.4, p[2]], .35 * fade + .12, a.col, fade); beam(o, [p[0], 1.4, p[2]], .1, '#ffffff', fade);
          fx.orb.add(o[0], o[1], o[2], null, 1.2, 1.2, 1.2, a.col, fade);
          break;
        }
        case 'spark': {
          const w = W(a.x, a.y, H), r = .6 + 1.6 * k;
          for (let i = 0; i < 6; i++) { const an = i / 6 * TAU + a.x; fx.orb.add(w[0] + Math.cos(an) * r, w[1] + Math.sin(an) * r * .6, w[2] + Math.sin(an) * r * .4, null, .18, .18, .18, '#fff6d0', fade); }
          break;
        }
        case 'smite': {
          const w = W(a.x, a.y); beam([w[0], 30, w[2]], [w[0], 1, w[2]], .9 * fade + .2, '#ffd23f', fade); fx.disc.add(w[0], .15, w[2], null, 4 * (1 + k), 1, 4 * (1 + k), '#ffe58a', fade);
          break;
        }
        case 'num': nums.push(a); break;
      }
    }
    // Damage numbers and gold popups (text sprites).
    for (const a of nums.slice(-14)) {
      const k = clamp(a.age / a.life, 0, 1), w = W(a.x + (a.dx || 0) * .6, a.y, 3.6 + k * 2.2);
      if (!near(w, 70)) continue;
      s.text.put(a, a.text, a.col || '#fff', w[0], w[1], w[2], { alpha: clamp((1 - k) * 3, 0, 1), size: a.big ? 1 : .7 });
    }
    for (const p of src.popups) { const k = p.age / p.life, w = W(p.x + 2, p.y, 5 + k * 3); if (near(w, 90)) s.text.put(p, p.text, p.color, w[0], w[1], w[2], { alpha: clamp((1 - k) * 2, 0, 1), size: .85 }); }
    for (const q of src.skulls) { const a = clamp(Math.min(q.age * 4, (q.life - q.age) / .6), 0, 1), w = W(q.x, q.y, 4.2); if (near(w, 90)) s.text.put(q, '☠', q.color, w[0], w[1], w[2], { alpha: a, size: 1, outline: 'rgba(0,0,0,.9)' }); }
    // Flashes: a ring that spreads on the ground; debris and sparks fly.
    for (const f of src.flashes) {
      const k = f.age / f.life, w = W(f.x, f.y), r = K * (f.big ? 11 : 6) * easeOut(k);
      fx.ring.add(w[0], .15, w[2], null, r, 1, r, f.color && f.color.startsWith('#') ? f.color : '#ffffff', 1 - k);
      if (k < .3) fx.orb.add(w[0], 1.5, w[2], null, (f.big ? 7 : 4), (f.big ? 7 : 4), (f.big ? 7 : 4), '#fff8e0', (.3 - k) * 2);
    }
    for (const q of src.particles) {
      const w = W(q.x, q.y, .4 + (1.3 - q.life) * 2.2);
      fx.chunk.add(w[0], Math.max(.2, w[1]), w[2], null, .35, .35, .35, q.c || '#8a7a66');
    }
  },

  // Called before update() each frame: read the view, not the last update.
  entities(ctx) {
    const st = ctx.view.state, s = ctx.state;
    if (!st || !st.line) return [];
    const key = st.line.map(l => l.map(p => (p && p.champ) || '').join()).join('|') + st.meta.teams?.map(t => t.color).join();
    if (s.rosterKey !== key) { s.rosterKey = key; s.roster = rosterOf(st); }
    return s.roster;
  },

  autoEntity(ctx) { return autoChampion(ctx.view, ctx.state.memo || (ctx.state.memo = {})); },

  cameraFor(ctx, id) {
    const s = ctx.state, p = s.spots && s.spots[id];
    if (!p) return null;
    const cut = s.cut; s.cut = false;
    if (p.dead && !p.corpse) {
      // Dead: up above the fountain, looking out over the base.
      const f = p.fountain, yaw = headingYaw(-f[0], -f[2]);
      const c = shoulderCam(f, yaw, { dist: 12, height: 10, ahead: 18, look: 0 });
      s.camYaw = yaw;
      const first = s.lastDeadCam !== id; s.lastDeadCam = id;
      return { ...c, overlay: 'visor', cut: cut || first };
    }
    s.lastDeadCam = null;
    // The camera turns towards the fight more slowly than the body.
    const id2 = s.camFor;
    if (s.camYaw == null || cut || id2 !== id) s.camYaw = p.yaw;
    else s.camYaw = dampAngle(s.camYaw, p.yaw, p.dead ? 1 : 2.4, s.dt || 1 / 60);
    s.camFor = id;
    const c = shoulderCam(p.w, s.camYaw, p.dead ? { dist: 7, height: 5 } : { eye: s.eye, eyeH: p.eyeH });
    return { ...c, overlay: s.eye ? 'visor' : null, cut };
  },

  focus(ctx) { return ctx.state.focus || [0, 1, 0]; },

  freeStart() { return { yaw: Math.PI * .75, pitch: .75, dist: 48 }; },

  hud(ctx) {
    const v = ctx.view, st = v.state, s = ctx.state;
    if (!st) return { caption: TEXT.en.pregame };
    const src = s.src || frameSource(v), b = blueTeam(st.meta), order = [b, 1 - b];
    const names = v.names ? v.names() : ['Blue', 'Red'];
    const abbr = t => (st.meta.teams && st.meta.teams[t] && st.meta.teams[t].abbr) || names[t];
    const teams = order.map(t => ({ name: abbr(t), color: teamColor(st.meta, t), score: st.kills[t] || 0, mark: !!(st.buff[t] && (st.buff[t].baron > src.clock || st.buff[t].elder > src.clock)) }));
    const clock = st.mode === 'victory' ? TEXT.en.final : st.mode === 'draft' ? TEXT.en.draft : fmtClock(src.clock);
    const sub = st.mode === 'draft' ? '' : `${fmtGold(st.gold[order[0]])} ⛁ ${fmtGold(st.gold[order[1]])} · 🗼 ${st.towersK[order[0]] || 0}–${st.towersK[order[1]] || 0}`;
    let tag = v.replay ? 'REPLAY · SLOW-MO' : v.scene ? `⏸ ${v.scene.label || CAM_TAGS[v.scene.kind] || ''}` : v.shot ? `ACTION · ${v.shot.label || ''}` : '';
    const caption = st.ticker || '';
    let banner = null;
    const bn = v.banner;
    if (bn && !v.replay) { const age = (v.anim || 0) - bn.t0, life = bn.big ? 2.6 : 1.8; if (age >= 0 && age < life) banner = { id: bn.t0, text: bn.text, color: bn.color, color2: '#ffffff' }; }
    if (st.mode === 'victory' && st.end) { const w = st.end.winner === 1 ? 1 : 0; banner = { id: 'victory', text: `${TEXT.en.victory} · ${names[w]}`, color: teamColor(st.meta, w), color2: '#ffffff' }; }
    this.paintDom(ctx, st, src);
    return { teams, clock, sub, tag, caption, banner };
  },

  // The watched champion's card and the kill feed (DOM, only when the text changes).
  paintDom(ctx, st, src) {
    const s = ctx.state, v = ctx.view;
    if (!s.cardEl) return;
    const me = parseId(ctx.watch && ctx.watch.mode === 'first' ? ctx.watch.id : null);
    let card = '';
    if (me && st.line) {
      const l = st.line[me.t][me.r] || {}, kda = st.kda[me.t][me.r] || { k: 0, d: 0, a: 0 }, RS = TEXT.en.roleShort;
      const alive = isAliveAt(src.pos, me.t, me.r), hp = src.hp ? Math.round(src.hp[me.t][me.r] * 100) : 100;
      const left = Math.max(0, Math.ceil((st.dead[me.t][me.r] || 0) - src.clock));
      card = `<b>${esc(l.champ || l.handle || '')}</b> · ${esc(l.handle || '')} · ${esc(RS[l.role] || l.role || '')}<br><span class="k">${kda.k} / ${kda.d} / ${kda.a}</span> · ${esc(fmtGold(st.pgold[me.t][me.r]))} gold · ${alive ? `HP ${hp}%` : `<span class="dead">☠ ${left}s</span>`}`;
      const tc = teamColor(st.meta, me.t);
      if (s.cardTc !== tc) { s.cardTc = tc; s.cardEl.style.setProperty('--tc', tc); }
    }
    if (s.cardHtml !== card) { s.cardHtml = card; s.cardEl.innerHTML = card; }
    const feed = (v.killFeed || []).filter(f => (v.anim || 0) - f.born < 14).slice(0, 3).map(f => {
      const kc = teamColor(st.meta, f.killer.side === 1 ? 1 : 0), vc = teamColor(st.meta, f.victim.side === 1 ? 1 : 0);
      return `<div><span style="color:${esc(kc)}">${esc(f.killer.champ || '?')}</span><i>${f.ace ? 'ACE' : f.multi >= 2 ? '⚔⚔' : '⚔'}</i><span style="color:${esc(vc)}">${esc(f.victim.champ || '?')}</span></div>`;
    }).join('');
    if (s.feedHtml !== feed) { s.feedHtml = feed; s.feedEl.innerHTML = feed; }
    // The objective dying on camera: a boss bar, like the 2D action cam's.
    const sm = src.shot && src.shot.mon;
    let boss = '';
    if (sm && Number.isFinite(sm.dieAt) && src.anim < sm.dieAt + .3) {
      const hp = drainHp(sm, src.anim), max = sm.kind === 'baron' ? 12000 : sm.kind === 'elder' ? 9000 : 5500;
      boss = `<span style="color:${esc(sm.col)}">${esc(sm.name || '')}</span> · ${Math.ceil(hp * max)}<div class="bar" style="--mc:${esc(sm.col)}"><i style="width:${(hp * 100).toFixed(1)}%"></i></div>`;
    }
    if (s.bossHtml !== boss) { s.bossHtml = boss; s.bossEl.innerHTML = boss; }
  },

  dispose(ctx) {
    const s = ctx.state;
    if (s.onKey) window.removeEventListener('keydown', s.onKey);
    for (const m of s.models ? s.models.values() : []) ctx.scene.remove(m.h.root);
  },
};

export default adapter;
