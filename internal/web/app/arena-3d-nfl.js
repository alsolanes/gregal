// American football in first-person 3D (arena-3d.js adapter).
//
// Everything comes from the 2D viewer (football-view.js): the play in progress
// (view.cur.play / view.cur.anim from buildPlay), its time view.t (with the
// tackle hit-stop), the phase (pre, run, post, replay), the score and the
// crowd's hype. This file only turns that state into a stadium: the field with
// its lines and numbers (a canvas texture), the end zones in team colours,
// goal posts, pylons, a bowl of stands with an instanced crowd, light towers
// and two videoboards; 22 low-poly players with their numbers, posed from the
// play tracks (running, catching, throwing, tackling, falling and getting up)
// and the ball with its flight, spiral, tumble and bounces.
//
// Coordinates: the play is in yards, x along the field (0..120 with the end
// zones), y across (0..53.33), z up. The world is Y-up with the field's centre
// at the origin: fieldToWorld(x, y, z) = [x - 60, z, 26.67 - y]. A heading
// (hx, hy) on the field is yaw atan2(hy, hx) in the world (see arena-3d.js).

import { hitStop, attackDir, fieldX, subjectOf, TEXT } from './football-view.js';
import { clamp, lerp, rng, hexToRgb } from './arena-view-base.js';
import { dampAngle } from './arena-3d.js';

export const FIELD_W = 53.33;
const C = FIELD_W / 2;
const ease = t => t < .5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
const seg = (t, a, b) => clamp((t - a) / (b - a), 0, 1);

// ---- pure mapping and choices (tested in arena-3d.test.cjs) ----
export function fieldToWorld(x, y, z = 0) { return [x - 60, z, C - y]; }
export function worldToField(p) { return { x: p[0] + 60, y: C - p[2], z: p[1] }; }
export function headingYaw(hx, hy) { return Math.atan2(hy, hx); }

const OFF_ORDER = ['QB', 'RB', 'WR', 'TE', 'OL', 'K', 'P'], DEF_ORDER = ['DL', 'LB', 'CB', 'S'];
// Who can be watched in this play, offence first (QB, RB, WR, TE, OL), then defence.
export function rosterOf(anim, play, teams) {
  const rank = tr => {
    const off = tr.team === play.off, order = off ? OFF_ORDER : DEF_ORDER;
    const k = order.indexOf(tr.p.pos);
    return (off ? 0 : 100) + (k < 0 ? 50 : k * 10);
  };
  return anim.tracks.map((tr, i) => ({ tr, i }))
    .sort((a, b) => rank(a.tr) - rank(b.tr) || (a.tr.p.number || 0) - (b.tr.p.number || 0) || a.i - b.i)
    .map(({ tr }) => ({ id: tr.p.id, name: `#${tr.p.number || '?'} ${tr.p.name || tr.p.pos}`, role: tr.p.pos, team: tr.team, color: (teams[tr.team] || {}).color || '#888' }));
}
// Who the camera follows on its own: the QB until the ball is on its way, then
// the receiver (and whoever runs with it); runners after the handoff; the
// returner on kicks; the kicker on field goals.
export function autoSubjectId(anim, play, t) {
  const m = anim.marks || {};
  const has = id => id && anim.tracks.some(tr => tr.p.id === id);
  let id = null;
  if (m.targetId && m.tThrow !== undefined) {
    const flip = m.tThrow + .65 * (m.tCatch - m.tThrow);
    id = t < flip ? m.qbId : (t >= m.tCatch && anim.carrierId ? anim.carrierId : m.targetId);
  } else if (play.kind === 'kickoff') id = m.returnerId || anim.carrierId;
  else if (play.kind === 'punt') id = t < .3 ? m.kickerId : (m.returnerId || m.kickerId);
  else if (play.kind === 'fg' || play.kind === 'pat') id = m.kickerId;
  if (!has(id)) id = subjectOf(anim, t, 'auto').p.id;
  return id;
}
// The first-person eye of track i at t: inside the helmet, looking where the
// player runs, at the ball while it flies (or bounces), at the carrier when
// chasing him, down at the turf when falling.
export function eyeOf(anim, play, i, t) {
  const sh = anim.shoulderOf(i, t), pose = anim.poseOf(i, t), hd = anim.headingOf(i, t), b = anim.ball(t);
  const fk = pose.fall ? pose.fall.k : 0, a = fk * Math.PI / 2 * .85;
  const hx = hd[0], hy = hd[1];
  const up = .36 * Math.cos(a), fwd = .1;
  const eye = { x: sh.x + hx * fwd + (pose.fall ? pose.fall.dir[0] * .36 * Math.sin(a) : 0), y: sh.y + hy * fwd + (pose.fall ? pose.fall.dir[1] * .36 * Math.sin(a) : 0), z: Math.max(.45, sh.z + up) };
  let yaw = headingYaw(hx, hy), pitch = -.07;
  const tr = anim.tracks[i];
  let w = 0, tx = b.x, ty = b.y, tz = b.z;
  if (b.mode === 'fly' || b.mode === 'loose') w = 1;
  else if (b.holder >= 0 && b.holder !== i) {
    const carrierTeam = anim.tracks[b.holder] ? anim.tracks[b.holder].team : -1;
    w = carrierTeam === tr.team ? .2 : .6; tz = 1.2;
  }
  const dx = tx - eye.x, dy = ty - eye.y, dz = tz - eye.z, dist = Math.hypot(dx, dy);
  if (w > 0 && dist > .3 && dist < 80) {
    const by = headingYaw(dx, dy), bp = Math.atan2(dz, dist);
    // Never snap the head round to look straight back unless the ball is in the air.
    const diff = Math.atan2(Math.sin(by - yaw), Math.cos(by - yaw));
    const k = b.mode === 'fly' || b.mode === 'loose' ? 1 : clamp(1 - (Math.abs(diff) - 1.6) / 1.2, 0, 1) * w;
    yaw += diff * k; pitch = lerp(pitch, clamp(bp, -1.1, 1.2), k);
  }
  // On the ground: lower, looking a little down, not into the turf.
  pitch -= fk * .35;
  return { eye, yaw, pitch, fall: fk };
}

// ---- colours ----
const SKIN = ['#f1c7a5', '#d9a47e', '#b77b52', '#8d5a3b', '#6b4329', '#e8b893'];
const hashStr = v => { let h = 0; for (const c of String(v)) h = (h * 31 + c.charCodeAt(0)) >>> 0; return h; };
const lum = h => { const [r, g, b] = hexToRgb(h); return (.299 * r + .587 * g + .114 * b) / 255; };
function shade(h, f) {
  const [r, g, b] = hexToRgb(h, [120, 120, 120]);
  const k = v => clamp(Math.round(f < 0 ? v * (1 + f) : v + (255 - v) * f), 0, 255);
  return '#' + [k(r), k(g), k(b)].map(v => v.toString(16).padStart(2, '0')).join('');
}
const teamLook = t => {
  const jersey = t.color || '#1E5BD8', trim = t.color2 || '#ffffff';
  return { jersey, trim, pants: lum(trim) > .55 ? trim : shade(jersey, .7), helmet: shade(jersey, -.08) };
};

// ---- the bowl (same shape as the 2D stadium) ----
const BOWL_A = 76, BOWL_B = 44;
function ring(th, o) {
  const c = Math.cos(th), s = Math.sin(th);
  return [60 + (BOWL_A + o) * Math.sign(c) * Math.sqrt(Math.abs(c)), C + (BOWL_B + o) * Math.sign(s) * Math.sqrt(Math.abs(s))];
}
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
// [offset and height at the bottom, offset and height at the top, colour, kind]
const TIERS = [
  [0, 0, 0, 1.4, null, 'wall'],
  [0, 1.4, 22, 12, '#283252', 'seats'],
  [22, 12, 22, 15.5, '#0c1120', 'suite'],
  [22, 13.2, 22, 14.4, '#ffe096', 'glass'],
  [22, 15.5, 40, 26, '#232c4a', 'seats'],
  [40, 26, 40, 29.5, '#0c1120', 'suite'],
  [40, 27.2, 40, 28.4, '#f5d28a', 'glass'],
  [40, 29.5, 58, 41, '#1f2843', 'seats'],
  [58, 41, 58, 47, '#11172a', 'rim'],
  [58, 47, 46, 48.5, '#1a2138', 'roof'],
  [46, 48.4, 46, 48.9, '#fffae1', 'glass'],
];
const SEAT_ROWS = [[1, 9], [4, 8], [7, 7]];
const TOWERS = [.62, Math.PI - .62, Math.PI + .62, -.62];
const BOWL_N = 72;

// ---- the field texture ----
function paintField(g, W, H, teams, q) {
  const S = W / 120, py = y => (FIELD_W - y) * S, px = x => x * S;
  g.fillStyle = '#357d38'; g.fillRect(0, 0, W, H);
  for (let x = 10; x < 110; x += 5) { g.fillStyle = (x / 5) % 2 ? '#3d8a3f' : '#357d38'; g.fillRect(px(x), 0, px(5) + 1, H); }
  // A little grain so the grass is not flat at the players' feet.
  const R = rng(99);
  for (let k = 0; k < 9000; k++) { g.fillStyle = R() < .5 ? 'rgba(0,0,0,.05)' : 'rgba(255,255,255,.035)'; g.fillRect(R() * W, R() * H, 2 + R() * 3, 1 + R() * 2); }
  const left = teams[attackDir(0, q) > 0 ? 0 : 1], right = teams[attackDir(0, q) > 0 ? 1 : 0];
  for (const [x0, tm, rot] of [[0, left, -Math.PI / 2], [110, right, Math.PI / 2]]) {
    g.fillStyle = shade(tm.color || '#444', -.15); g.fillRect(px(x0), 0, px(10), H);
    g.save(); g.beginPath(); g.rect(px(x0), 0, px(10), H); g.clip();
    g.strokeStyle = shade(tm.color || '#444', -.3); g.lineWidth = S * 1.1;
    for (let y = -10; y < FIELD_W + 10; y += 6) { g.beginPath(); g.moveTo(px(x0), py(y)); g.lineTo(px(x0 + 10), py(y + 6)); g.stroke(); }
    g.restore();
    g.save(); g.translate(px(x0 + 5), py(C)); g.rotate(rot);
    g.font = `900 ${Math.round(S * 5)}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.textBaseline = 'middle';
    g.fillStyle = tm.color2 || '#fff'; g.strokeStyle = 'rgba(0,0,0,.35)'; g.lineWidth = S * .25;
    const name = String(tm.name || '').toUpperCase().slice(0, 14);
    g.strokeText(name, 0, 0, px(FIELD_W - 8)); g.fillText(name, 0, 0, px(FIELD_W - 8));
    g.restore();
  }
  const line = (x0, y0, x1, y1, w, c = 'rgba(255,255,255,.92)') => { g.strokeStyle = c; g.lineWidth = w * S; g.beginPath(); g.moveTo(px(x0), py(y0)); g.lineTo(px(x1), py(y1)); g.stroke(); };
  for (let x = 10; x <= 110; x += 5) line(x, 0, x, FIELD_W, x === 10 || x === 110 ? .22 : .14);
  g.fillStyle = 'rgba(255,255,255,.9)';
  for (let x = 11; x < 110; x++) { if (x % 5 === 0) continue; for (const [y0, y1] of [[22.9, 23.6], [29.7, 30.4], [1, 1.6], [FIELD_W - 1.6, FIELD_W - 1]]) g.fillRect(px(x) - .06 * S, py(y1), .12 * S, (y1 - y0) * S); }
  // Border: sidelines and end lines.
  g.strokeStyle = '#fff'; g.lineWidth = .3 * S; g.strokeRect(.15 * S, .15 * S, W - .3 * S, H - .3 * S);
  // Numbers, bottom towards the nearest sideline.
  g.font = `700 ${Math.round(S * 2.4)}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillStyle = 'rgba(255,255,255,.92)';
  for (let x = 20; x <= 100; x += 10) {
    const n = String(x <= 60 ? x - 10 : 110 - x);
    g.fillText(n, px(x), py(9));
    g.save(); g.translate(px(x), py(FIELD_W - 9)); g.rotate(Math.PI); g.fillText(n, 0, 0); g.restore();
  }
  // Midfield logo.
  g.beginPath(); g.arc(px(60), py(C), 5.2 * S, 0, Math.PI * 2); g.fillStyle = 'rgba(255,255,255,.55)'; g.fill();
  g.beginPath(); g.arc(px(60), py(C), 4.7 * S, 0, Math.PI * 2); g.fillStyle = shade(teams[0].color || '#444', -.2); g.fill();
  g.beginPath(); g.arc(px(60), py(C), 4.7 * S, -Math.PI / 2, Math.PI / 2); g.fillStyle = shade(teams[1].color || '#444', -.2); g.fill();
  g.font = `800 ${Math.round(S * 2.2)}px "Segoe UI", system-ui, sans-serif`; g.fillStyle = 'rgba(255,255,255,.88)'; g.fillText('VENTS', px(60), py(C));
}

// The videoboard: score and clock, or the big moment (TOUCHDOWN…).
function paintBoard(g, w, h, d) {
  g.fillStyle = '#04060c'; g.fillRect(0, 0, w, h);
  g.textAlign = 'center'; g.textBaseline = 'middle';
  if (d.banner) {
    g.fillStyle = d.flash ? d.banner.color : (d.banner.color2 || '#fff'); g.fillRect(6, 6, w - 12, h - 12);
    g.fillStyle = d.flash ? (d.banner.color2 || '#fff') : d.banner.color; g.font = '900 italic 54px "Segoe UI", system-ui, sans-serif';
    g.fillText(d.banner.text.replace(/!$/, ''), w / 2, h / 2, w - 30);
    return;
  }
  [0, 1].forEach(i => {
    const t = d.teams[i], x = i ? w * .75 : w * .25;
    g.fillStyle = t.color; g.fillRect(i ? w / 2 + 6 : 6, 6, w / 2 - 12, 34);
    g.fillStyle = lum(t.color) > .6 ? '#111' : '#fff'; g.font = '800 24px "Segoe UI", system-ui, sans-serif'; g.fillText(t.abbr || '', x, 24);
    g.fillStyle = '#ffd400'; g.font = '900 72px "Segoe UI", system-ui, sans-serif'; g.fillText(String(d.score[i]), x, 96);
  });
  g.fillStyle = '#fff'; g.font = '700 20px "Segoe UI", system-ui, sans-serif'; g.fillText(d.clock, w / 2, 150);
}

// ---------------------------------------------------------------------------
// The adapter
// ---------------------------------------------------------------------------
export const adapter = {
  label: 'American football',

  build(ctx) {
    const { THREE, scene, kit } = ctx, s = ctx.state;
    s.models = new Map(); s.phase = new Map(); s.yaw = new Map(); s.playKey = null; s.lastT = 0; s.cut = true; s.trail = [];
    scene.background = new THREE.Color('#070b18');
    scene.fog = new THREE.Fog(0x0a0f22, 170, 560);
    kit.lights(scene, { hemi: 1.35, sunI: 1.5, dir: [-30, 100, 40] });
    // Ground beyond the field and the sideline apron.
    const ground = new THREE.Mesh(new THREE.PlaneGeometry(520, 380), kit.mat('#14301c'));
    ground.rotation.x = -Math.PI / 2; ground.position.y = -.03; scene.add(ground);
    const apron = new THREE.Mesh(new THREE.PlaneGeometry(146, 74), kit.mat('#2f6b33', { polygonOffset: true, polygonOffsetFactor: -1, polygonOffsetUnits: -1 }));
    apron.rotation.x = -Math.PI / 2; scene.add(apron);
    // The field (texture painted once the teams are known).
    s.fieldTex = kit.canvasTexture(2560, Math.round(2560 * FIELD_W / 120), (g, w, h) => { g.fillStyle = '#357d38'; g.fillRect(0, 0, w, h); }, { aniso: Math.min(8, ctx.renderer.capabilities.getMaxAnisotropy()) });
    const field = new THREE.Mesh(new THREE.PlaneGeometry(120, FIELD_W), new THREE.MeshLambertMaterial({ map: s.fieldTex, polygonOffset: true, polygonOffsetFactor: -2, polygonOffsetUnits: -2 }));
    field.rotation.x = -Math.PI / 2; scene.add(field);
    // Line of scrimmage (blue) and first down (yellow, with a glow).
    const strip = (w, color, opacity, f) => { const m = new THREE.Mesh(new THREE.PlaneGeometry(w, FIELD_W), new THREE.MeshBasicMaterial({ color, transparent: opacity < 1, opacity, depthWrite: false, polygonOffset: true, polygonOffsetFactor: f, polygonOffsetUnits: f })); m.rotation.x = -Math.PI / 2; m.renderOrder = 2; m.visible = false; scene.add(m); return m; };
    s.los = strip(.22, 0x4690ff, .95, -3);
    s.fdGlow = strip(1.1, 0xffd400, .22, -3);
    s.fd = strip(.26, 0xffd400, 1, -4);
    // Goal posts and pylons.
    const yellow = kit.mat('#f4d03f'), orange = kit.mat('#ff7a12');
    for (const x of [0, 120]) {
      const sgn = x ? 1 : -1, parts = [];
      const cyl = (x0, y0, z0, x1, y1, z1, r) => { const a = new THREE.Vector3(...fieldToWorld(x0, y0, z0)), b = new THREE.Vector3(...fieldToWorld(x1, y1, z1)); const len = a.distanceTo(b); const m = new THREE.Mesh(kit.G.cyl, yellow); m.scale.set(r * 2, len, r * 2); m.position.copy(a).add(b).multiplyScalar(.5); m.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), b.clone().sub(a).normalize()); parts.push(m); };
      cyl(x + sgn * 2, C, 0, x + sgn * 2, C, 2.7, .12);
      cyl(x + sgn * 2, C, 2.7, x, C, 2.95, .1);
      cyl(x, C, 2.95, x, C, 3.33, .1);
      cyl(x, C - 3.08, 3.33, x, C + 3.08, 3.33, .09);
      cyl(x, C - 3.08, 3.33, x, C - 3.08, 14, .07);
      cyl(x, C + 3.08, 3.33, x, C + 3.08, 14, .07);
      parts.forEach(p => scene.add(p));
    }
    for (const x of [0, 10, 110, 120]) for (const y of [0, FIELD_W]) { const p = new THREE.Mesh(kit.G.box, orange); p.scale.set(.13, .5, .13); p.position.set(...fieldToWorld(x, y, .25)); scene.add(p); }
    // Stars.
    const R = rng(23), sp = [];
    for (let i = 0; i < 260; i++) { const az = R() * 6.28, el = .15 + R() * 1.2; sp.push(Math.cos(az) * Math.cos(el) * 600, Math.sin(el) * 600, Math.sin(az) * Math.cos(el) * 600); }
    const sg = new THREE.BufferGeometry(); sg.setAttribute('position', new THREE.Float32BufferAttribute(sp, 3));
    scene.add(new THREE.Points(sg, new THREE.PointsMaterial({ color: 0xffffff, size: 1.6, sizeAttenuation: false, fog: false, transparent: true, opacity: .75 })));
    // The ball: an oval with white stripes near the tips and laces.
    const ballGeo = kit.merge([
      { g: kit.G.sph, c: '#7a3e14', m: kit.M(0, 0, 0, .31, .19, .19) },
      { g: kit.G.cyl, c: '#f2f2f2', m: kit.M(.095, 0, 0, .162, .012, .162, 0, 0, Math.PI / 2) },
      { g: kit.G.cyl, c: '#f2f2f2', m: kit.M(-.095, 0, 0, .162, .012, .162, 0, 0, Math.PI / 2) },
      { g: kit.G.box, c: '#ffffff', m: kit.M(0, .092, 0, .12, .012, .016) },
    ]);
    s.ball = new THREE.Mesh(ballGeo, kit.mats.vc); s.ball.visible = false; scene.add(s.ball);
    s.trailGeo = new THREE.BufferGeometry(); s.trailGeo.setAttribute('position', new THREE.Float32BufferAttribute(new Float32Array(16 * 3), 3));
    s.trailLine = new THREE.Line(s.trailGeo, new THREE.LineBasicMaterial({ color: 0xffffff, transparent: true, opacity: .45 }));
    s.trailLine.frustumCulled = false; s.trailLine.visible = false; scene.add(s.trailLine);
    // Shadows: 48 slots (players, the ball).
    s.shadows = kit.blobShadows(48); scene.add(s.shadows.mesh);
    // Contact flash.
    const flashTex = kit.canvasTexture(64, 64, (g, w, h) => { const r = g.createRadialGradient(w / 2, h / 2, 0, w / 2, h / 2, w / 2); r.addColorStop(0, 'rgba(255,255,240,1)'); r.addColorStop(.35, 'rgba(255,250,220,.5)'); r.addColorStop(1, 'rgba(255,250,220,0)'); g.fillStyle = r; g.fillRect(0, 0, w, h); });
    s.flash = new THREE.Sprite(new THREE.SpriteMaterial({ map: flashTex, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending, fog: false }));
    s.flash.visible = false; scene.add(s.flash);
    s.glowTex = flashTex;
    s.tmpQ = new THREE.Quaternion(); s.tmpQ2 = new THREE.Quaternion(); s.tmpV = new THREE.Vector3(); s.xAxis = new THREE.Vector3(1, 0, 0);
  },

  // The stadium needs the team colours: built when the match is known (and again if they change).
  stadium(ctx, teams) {
    const { THREE, scene, kit } = ctx, s = ctx.state;
    if (s.bowl) { scene.remove(s.bowl); s.bowl.traverse(o => { if (o.geometry && o.geometry !== kit.G.box && o.geometry !== kit.G.cyl) o.geometry.dispose(); }); }
    const bowl = new THREE.Group(); s.bowl = bowl;
    // Stands: one strip per tier round the bowl, flat colour per segment.
    const solid = { pos: [], col: [] }, glow = { pos: [], col: [] };
    const tmp = new THREE.Color();
    for (let i = 0; i < BOWL_N; i++) {
      const t0 = ringTh(i / BOWL_N), t1 = i + 1 === BOWL_N ? Math.PI * 2 : ringTh((i + 1) / BOWL_N);
      for (const [o0, z0, o1, z1, fill, kind] of TIERS) {
        let c = fill;
        if (kind === 'wall') c = shade((teams[i % 2] || {}).color || '#333', -.35);
        else if (kind === 'seats') {
          const th = ringTh((i + .5) / BOWL_N), side = Math.abs(Math.cos(th)) > .82 ? -1 : (Math.sin(th) < 0 ? 0 : 1);
          c = side < 0 ? (i % 4 ? '#2b3150' : '#343b5c') : shade((teams[side] || {}).color || '#333', i % 4 ? -.62 : -.55);
        }
        const a = ring(t0, o0), b = ring(t1, o0), cc = ring(t1, o1), d = ring(t0, o1);
        const P = [fieldToWorld(a[0], a[1], z0), fieldToWorld(b[0], b[1], z0), fieldToWorld(cc[0], cc[1], z1), fieldToWorld(d[0], d[1], z1)];
        const out = kind === 'glass' ? glow : solid;
        tmp.set(c);
        for (const k of [0, 1, 2, 0, 2, 3]) { out.pos.push(...P[k]); out.col.push(tmp.r, tmp.g, tmp.b); }
      }
    }
    const mesh = (o, material) => { const g = new THREE.BufferGeometry(); g.setAttribute('position', new THREE.Float32BufferAttribute(o.pos, 3)); g.setAttribute('color', new THREE.Float32BufferAttribute(o.col, 3)); g.computeVertexNormals(); return new THREE.Mesh(g, material); };
    s.standMat = s.standMat || new THREE.MeshLambertMaterial({ vertexColors: true, side: THREE.DoubleSide });
    s.glowMat = s.glowMat || new THREE.MeshBasicMaterial({ vertexColors: true, side: THREE.DoubleSide });
    bowl.add(mesh(solid, s.standMat), mesh(glow, s.glowMat));
    // The crowd: the 2D stadium's seats, team colours by side, facing the field.
    const R = rng(23), pal = [teams[0].color, teams[0].color2 || '#fff', teams[1].color, teams[1].color2 || '#fff', '#c9c4b8', '#5b6170', '#8a6f55', '#2e3240'];
    const people = [];
    for (const [ti, rows] of SEAT_ROWS) {
      const [o0, z0, o1, z1] = TIERS[ti];
      for (let r = 0; r < rows; r++) {
        const k = (r + .5) / rows, o = lerp(o0, o1, k), z = lerp(z0, z1, k) + .05;
        const per = Math.round((380 + o * 6.3) / 1.6);
        for (let j = 0; j < per; j++) {
          if (R() < .06) continue;
          const th = ringTh((j + R() * .6) / per), [x, y] = ring(th, o);
          const side = Math.abs(Math.cos(th)) > .82 ? -1 : (y < C ? 0 : 1);
          const q = R(), team = side < 0 ? (R() < .5 ? 0 : 1) : (q < .82 ? side : 1 - side);
          const ci = q < .9 ? team * 2 + (R() < .72 ? 0 : 1) : 4 + Math.floor(R() * 4);
          const w = fieldToWorld(x, y, z);
          people.push({ x: w[0], y: w[1], z: w[2], yaw: headingYaw(60 - x, C - y), color: pal[ci], team: q < .9 ? team : -1, ph: R() * 6.28, th });
        }
      }
    }
    s.crowd = kit.crowd(people, { body: [.5, .7, .42], head: .27 });
    bowl.add(s.crowd.group);
    // Light towers with a glow.
    for (const th of TOWERS) {
      const [x, y] = ring(th, 64), base = fieldToWorld(x, y, 0);
      const pole = new THREE.Mesh(kit.G.cyl, kit.mat('#0b0f1c')); pole.scale.set(1.2, 56, 1.2); pole.position.set(base[0], 28, base[2]); bowl.add(pole);
      const bank = new THREE.Mesh(kit.G.box, new THREE.MeshBasicMaterial({ color: 0xfffbe8, fog: false })); bank.scale.set(1, 8, 14); bank.position.set(base[0], 58, base[2]); bank.rotation.y = headingYaw(60 - x, C - y); bowl.add(bank);
      const glowS = new THREE.Sprite(new THREE.SpriteMaterial({ map: s.glowTex, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending, fog: false, opacity: .7 }));
      glowS.scale.set(46, 46, 1); glowS.position.set(base[0], 58, base[2]); bowl.add(glowS);
    }
    // Videoboards over both end zones, facing the field.
    s.boardTex = s.boardTex || kit.canvasTexture(384, 176, (g, w, h) => { g.fillStyle = '#04060c'; g.fillRect(0, 0, w, h); }, { aniso: 2 });
    s.boardKey = '';
    for (const [x, ry] of [[-20, Math.PI / 2], [140, -Math.PI / 2]]) {
      const w = fieldToWorld(x, C, 19.75);
      const scr = new THREE.Mesh(new THREE.PlaneGeometry(30, 13.5), new THREE.MeshBasicMaterial({ map: s.boardTex, fog: false }));
      scr.position.set(w[0], w[1], w[2]); scr.rotation.y = ry; bowl.add(scr);
      const frame = new THREE.Mesh(kit.G.box, kit.mat('#1c2233')); frame.scale.set(.8, 15, 31.6); frame.position.set(w[0] + Math.sign(w[0]) * .45, w[1], w[2]); bowl.add(frame);
      for (const dy of [-12, 12]) { const leg = new THREE.Mesh(kit.G.cyl, kit.mat('#3a4258')); leg.scale.set(.6, 13, .6); leg.position.set(w[0] + Math.sign(w[0]) * .6, 6.5, w[2] + dy); bowl.add(leg); }
    }
    scene.add(bowl);
    s.teamKey = teams[0].color + teams[1].color;
  },

  model(ctx, tr, teams) {
    const s = ctx.state;
    let h = s.models.get(tr.p.id);
    if (h) return h;
    const look = teamLook(teams[tr.team] || {});
    const pos = tr.p.pos, bulk = pos === 'OL' || pos === 'DL' ? 1.16 : pos === 'TE' || pos === 'LB' ? 1.06 : pos === 'WR' || pos === 'CB' || pos === 'K' ? .93 : 1;
    h = ctx.kit.footballPlayer({ ...look, number: tr.p.number != null ? tr.p.number : '', skin: SKIN[hashStr(tr.p.id || tr.p.name || '') % SKIN.length], bulk });
    h.team = tr.team;
    ctx.scene.add(h.root);
    s.models.set(tr.p.id, h);
    return h;
  },

  update(ctx, frame) {
    const s = ctx.state, v = ctx.view, m = v.match;
    if (!m || !m.teams) return;
    const teams = m.teams;
    if (s.teamKey !== teams[0].color + teams[1].color) this.stadium(ctx, teams);
    s.crowd.update(v.clock || 0, v.hype || 0, v.hypeTeam == null ? -1 : v.hypeTeam);
    const c = v.cur;
    // End zones swap with the quarter.
    const q = (c && c.play.q) || m.quarter || 1, fk = teams.map(t => t.color + t.name).join() + attackDir(0, q);
    if (s.fieldKey !== fk) { s.fieldKey = fk; s.fieldTex.redraw((g, w, h) => paintField(g, w, h, teams, q)); }
    this.board(ctx);
    if (!c) {
      for (const h of s.models.values()) h.root.visible = false;
      s.ball.visible = false; s.trailLine.visible = false; s.los.visible = s.fd.visible = s.fdGlow.visible = false;
      for (let i = 0; i < 48; i++) s.shadows.hide(i);
      s.shadows.commit(); s.flash.visible = false;
      s.playKey = null; s.cur = null;
      return;
    }
    const a = c.anim, play = c.play, t = hitStop(a, v.t);
    const key = play.n + ':' + (v.phase === 'replay' ? 'r' + c.rep : 'l');
    // A new play, a replay, or a jump back: no smoothing (the camera cuts).
    if (key !== s.playKey || t < s.lastT - 1e-6) s.cut = true;
    const dTau = key === s.playKey && t >= s.lastT ? (t - s.lastT) * a.dur : 0;
    if (play.n !== s.playN) {
      s.playN = play.n; s.trail = [];
      const ids = new Set(a.tracks.map(tr => tr.p.id));
      for (const [id, h] of s.models) h.root.visible = ids.has(id);
      // Lines: scrimmage and first down.
      const lines = !['kickoff', 'pat', 'two'].includes(play.kind);
      s.los.visible = lines; s.los.position.set(a.los - 60, 0, 0);
      const fd = lines && play.dist && play.spot + play.dist < 100;
      s.fd.visible = s.fdGlow.visible = !!fd;
      if (fd) { const fx = fieldX(play.spot + play.dist, a.dir) - 60; s.fd.position.x = s.fdGlow.position.x = fx; }
    }
    s.playKey = key; s.lastT = t; s.cur = { a, play, t };
    const moving = v.phase === 'run' || v.phase === 'replay';
    const b = a.ball(t);
    const watch = ctx.watch || {};
    const P = a.frameAt(t);
    const getUp = v.phase === 'post' && v.t >= 1 ? 1 - ease(seg(v.postAge || 0, 1, 1.6)) : 1;
    a.tracks.forEach((tr, i) => {
      const h = this.model(ctx, tr, teams), id = tr.p.id;
      h.root.visible = true;
      const pose = a.poseOf(i, t), hd = a.headingOf(i, t), pos = P[i];
      const speed = hd[2] / (.024 * a.dur);
      const run = moving ? clamp(speed / 7.5, 0, 1) : 0;
      const ph = (s.phase.get(id) || (i * 1.7)) + dTau * speed * 1.35;
      s.phase.set(id, ph);
      const yawT = headingYaw(hd[0], hd[1]);
      const prev = s.yaw.get(id);
      const yaw = prev == null || s.cut ? yawT : dampAngle(prev, yawT, 16, Math.max(dTau, moving ? 0 : frame.dt));
      s.yaw.set(id, yaw);
      const fall = pose.fall ? { k: pose.fall.k * getUp, dir: [pose.fall.dir[0], -pose.fall.dir[1]] } : null;
      const arms = pose.arms ? pose.arms.map(p => p && fieldToWorld(p[0], p[1], p[2])) : null;
      const w = fieldToWorld(pos.x, pos.y, pose.jump || 0);
      h.root.position.set(w[0], w[1], w[2]);
      h.pose({ yaw, phase: ph, run, lean: pose.lean, crouch: pose.crouch, jump: pose.jump, fall, arms, kick: pose.kick, firstPerson: watch.mode === 'first' && watch.id === id });
      s.shadows.set(i, w[0], w[2], .55 + (fall ? fall.k * .5 : 0), 1);
    });
    for (let i = a.tracks.length; i < 47; i++) s.shadows.hide(i);
    // The ball: along its axis, spinning round it.
    const bw = fieldToWorld(b.x, b.y, b.z);
    // Held by the player we look through: right under the eye, it would fill the
    // screen (and be cut by the near plane), so it is not drawn.
    const mine = watch.mode === 'first' && b.holder >= 0 && a.tracks[b.holder] && a.tracks[b.holder].p.id === watch.id;
    s.ball.visible = !mine; s.ball.position.set(bw[0], bw[1], bw[2]);
    const ax = b.axis || [1, 0, 0], aw = s.tmpV.set(ax[0], ax[2], -ax[1]).normalize();
    s.tmpQ.setFromUnitVectors(s.xAxis, aw); s.tmpQ2.setFromAxisAngle(aw, b.spin || 0);
    s.ball.quaternion.copy(s.tmpQ2.multiply(s.tmpQ));
    s.shadows.set(47, bw[0], bw[2], clamp(.32 - b.z * .015, .12, .32), 1);
    s.shadows.commit();
    // A faint trail while it flies.
    if (b.mode === 'fly' && moving) { s.trail.push(bw); if (s.trail.length > 16) s.trail.shift(); } else if (s.trail.length) s.trail = [];
    if (s.trail.length > 1) {
      const arr = s.trailGeo.attributes.position.array;
      for (let k = 0; k < 16; k++) { const p = s.trail[Math.max(0, s.trail.length - 16 + k)] || s.trail[0]; arr[k * 3] = p[0]; arr[k * 3 + 1] = p[1]; arr[k * 3 + 2] = p[2]; }
      s.trailGeo.attributes.position.needsUpdate = true; s.trailLine.visible = true;
    } else s.trailLine.visible = false;
    // Contact flash (from the play: the same in a replay).
    let fl = null;
    for (const f of a.fx || []) { const age = (t - f.t) * a.dur; if (f.kind === 'hit' && age >= 0 && age < .16) fl = { f, age }; }
    if (fl) { const p = fieldToWorld(fl.f.at.x, fl.f.at.y, 1.2), sz = (fl.f.big ? 2.6 : 1.8) * (.6 + fl.age * 3); s.flash.visible = true; s.flash.position.set(p[0], p[1], p[2]); s.flash.scale.set(sz, sz, 1); s.flash.material.opacity = (1 - fl.age / .16) * (fl.f.big ? .9 : .65); }
    else s.flash.visible = false;
  },

  board(ctx) {
    const s = ctx.state, v = ctx.view, m = v.match;
    const p = v.cur ? v.cur.play : (v.shown ? v.shown() : null), sc = v.score ? v.score() || [0, 0] : [0, 0], T = v.T || TEXT.en;
    const bn = v.banner && (performance.now() - v.banner.t0) < 4000 ? v.banner : null;
    const flash = bn ? Math.floor(performance.now() / 280) % 2 : 0;
    const clock = p ? `${T.q[Math.min(4, p.q - 1)]}  ${Math.floor(p.clock / 60)}:${String(p.clock % 60).padStart(2, '0')}` : '';
    const key = bn ? 'B' + bn.text + flash : [sc, clock].join('|');
    if (key === s.boardKey) return;
    s.boardKey = key;
    s.boardTex.redraw((g, w, h) => paintBoard(g, w, h, { teams: m.teams, score: sc, clock, banner: bn, flash }));
  },

  // Called before update() each frame: read the view, not the last update.
  entities(ctx) {
    const c = ctx.view.cur, s = ctx.state;
    if (!c || !ctx.view.match) return [];
    if (s.rosterN !== c.play.n) { s.rosterN = c.play.n; s.roster = rosterOf(c.anim, c.play, ctx.view.match.teams); }
    return s.roster;
  },

  autoEntity(ctx) { const c = ctx.view.cur; return c ? autoSubjectId(c.anim, c.play, hitStop(c.anim, ctx.view.t)) : null; },

  cameraFor(ctx, id) {
    const s = ctx.state, c = s.cur;
    if (!c) return null;
    const i = c.a.tracks.findIndex(tr => tr.p.id === id);
    if (i < 0) return null;
    const e = eyeOf(c.a, c.play, i, c.t);
    const h = s.models.get(id);
    // A little head bob when running.
    const bob = h ? Math.abs(Math.cos(s.phase.get(id) || 0)) * .05 * clamp((c.a.headingOf(i, c.t)[2] / (.024 * c.a.dur)) / 7.5, 0, 1) : 0;
    const pos = fieldToWorld(e.eye.x, e.eye.y, e.eye.z + bob);
    const cut = s.cut; s.cut = false;
    return { pos, yaw: e.yaw, pitch: e.pitch, fov: 76, overlay: 'helmet', cut };
  },

  focus(ctx) {
    const c = ctx.state.cur;
    if (!c) return [0, 1, 0];
    const b = c.a.ball(c.t);
    return fieldToWorld(clamp(b.x, -5, 125), clamp(b.y, -3, FIELD_W + 3), clamp(b.z, .5, 8));
  },

  freeStart(ctx) {
    const c = ctx.view.cur;
    const dir = c ? (c.anim.dir || 1) : 1;
    // Behind the offence, a little to the side, looking downfield.
    return { yaw: (dir > 0 ? Math.PI : 0) + .45 * dir, pitch: .42, dist: 26 };
  },

  hud(ctx) {
    const v = ctx.view, m = v.match;
    if (!m || !m.teams) return {};
    const T = v.T || TEXT.en, c = v.cur, p = c ? c.play : (v.shown ? v.shown() : null), sc = v.score() || [0, 0];
    const final = m.status === 'final' && v.idx >= v.queue.length && !c;
    const clock = final ? T.final : p ? `${T.q[Math.min(4, p.q - 1)]} ${Math.floor(p.clock / 60)}:${String(p.clock % 60).padStart(2, '0')}` : T.live;
    const sub = p && p.down && !final ? `${T.down[p.down]} ${T.and} ${p.spot + p.dist >= 100 ? T.goal : p.dist}` : '';
    const teams = m.teams.map((t, i) => ({ name: t.abbr || t.name, color: t.color, score: sc[i], mark: !!(p && p.off === i && !['pat', 'two', 'kickoff'].includes(p.kind)) }));
    let tag = v.phase === 'replay' ? 'REPLAY' : '';
    if (v.slow < 1 && (v.phase === 'replay' || v.phase === 'run')) tag += (tag ? ' · ' : '') + 'SLOW-MO';
    const caption = p && (!c || (v.phase === 'post' && c.replaysDone)) ? p.text || '' : (!c && m.status !== 'final' ? (m.status === 'pregame' ? T.pregame : T.waiting) : '');
    const bn = v.banner && (performance.now() - v.banner.t0) < 3000 ? { id: v.banner.t0, text: v.banner.text, color: v.banner.color, color2: v.banner.color2 } : null;
    return { teams, clock, sub, tag, caption, banner: bn };
  },
};

export default adapter;
