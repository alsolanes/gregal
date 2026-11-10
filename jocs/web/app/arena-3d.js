// La vista 3D en primera persona dels partits de l'Arena (three.js, vendor/).
// First-person 3D view for Arena matches, shared by every game.
//
// =====================================================================
// HOW TO WRITE A GAME ADAPTER (developer note)
// =====================================================================
// The 3D view never simulates anything. The game's existing 2D viewer
// (football-view.js, arena-cs2-view.js, arena-lol-view.js) keeps the one
// timeline: polling, seek, speed, pause, replays. While 3D is on, the 2D
// viewer is put in headless mode (ArenaViewBase.setHeadless): it stops its own
// loop and drawing, and this module calls view.tick(dt) once per 3D frame and
// then asks the adapter to pose the scene from the viewer's current state. So
// the scrubber, the commentary track, the TV page and the 3D view always show
// the same instant. Exiting 3D gives the loop back to the 2D viewer.
//
// 1. Register the adapter (one line, at the bottom of this file):
//      register3D('cs2', () => import('./arena-3d-cs2.js'));
//    The loader is only called when the user turns 3D on. The module must
//    export `adapter` (or a default export) with the shape below. A plain
//    adapter object is accepted too: register3D('x', { build, update, … }).
//
// 2. The adapter. Every method receives `ctx`; keep per-mount state in
//    ctx.state (a fresh {} for each mount), never in module globals.
//      build(ctx)            → create meshes in ctx.scene (may be async).
//      update(ctx, frame)    → read ctx.view (the 2D viewer; read only!) and
//                              pose everything for this frame.
//                              frame = { dt, clock, now }: dt real seconds
//                              since the last frame, clock seconds since mount.
//      entities(ctx), autoEntity(ctx) are called BEFORE update() in each
//      frame (after view.tick): read ctx.view, not what update() stored.
//      entities(ctx)         → who can be watched now, in cycling order:
//                              [{ id, name, role?, team: 0|1|-1, color? }]
//      autoEntity(ctx)       → the id to follow when nothing is locked (the
//                              ball carrier, the player with the bomb…) or null.
//      cameraFor(ctx, id)    → the first-person eye of that entity, or null:
//                              { pos: [x, y, z], yaw, pitch, fov?, overlay?, cut? }
//                              overlay: 'helmet' (facemask bars) | 'visor' | null.
//                              cut: true to jump there without smoothing
//                              (a new play, a replay, a seek).
//      focus(ctx)            → [x, y, z] the free camera orbits (the ball…).
//      hud?(ctx)             → { teams: [{ name, color, score, mark? }, …],
//                                clock, sub, tag, caption,
//                                banner: { id, text, color, color2 } | null }
//                              Strings only; the base updates the DOM when they change.
//      freeStart?(ctx)       → { yaw, pitch, dist } for the free camera.
//      dispose?(ctx)         → free textures you created outside ctx.scene.
//    ctx = { THREE, scene, camera, renderer, kit, view, game, lang, state,
//            watch: { id, mode: 'first'|'free' }, quality: { low } }
//    ctx.watch tells you who is being looked through (pose that body with
//    firstPerson: true so its head and arms are not drawn in your face).
//
// 3. World conventions. Y is up. yaw 0 looks along +X and grows
//    counter-clockwise seen from above (towards -Z); pitch > 0 looks up.
//    forwardOf(yaw, pitch) gives the unit vector. Characters made by the kit
//    face +X, so rotation.y = yaw. Units are the adapter's own (NFL: yards).
//
// 4. The kit (ctx.kit, built on ctx.THREE): shared materials and lights,
//    kit.humanoid(spec) – a low-poly rig (one mesh per limb, vertex colours,
//    ~12 draw calls) with h.pose({ yaw, phase, run, lean, crouch, jump, fall,
//    arms, kick, hideHead, firstPerson }) – and the presets kit.footballPlayer(),
//    kit.soldier() and kit.champion() (class silhouettes: tank, fighter,
//    mage, marksman, assassin, support), kit.crowd() (an instanced crowd,
//    one draw call, that bounces and does the wave on the GPU), kit.canvasTexture(), kit.blobShadows().
//
// The pure pieces (registry, camera rig maths, entity selection, two-bone IK)
// are exported for node tests: arena-3d.test.cjs.

import { clamp, lerp, viewDpr, lowPower, esc } from './arena-view-base.js';

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------
const REG = new Map();
const METHODS = ['build', 'update', 'entities', 'autoEntity', 'cameraFor', 'focus'];

// register3D(game, loader | adapter): loader() → Promise<module with `adapter` or default>.
export function register3D(game, def) {
  if (!game || !def || (typeof def !== 'function' && typeof def !== 'object')) throw new Error('register3D(game, loader | adapter)');
  REG.set(String(game), def);
}
export function unregister3D(game) { REG.delete(String(game)); }
export function has3D(game) { return REG.has(String(game)); }
export function games3D() { return [...REG.keys()]; }
// Checks an adapter has every required method (throws with the missing names).
export function checkAdapter(a, game = '') {
  const miss = METHODS.filter(k => !a || typeof a[k] !== 'function');
  if (miss.length) throw new Error(`3D adapter ${game}: missing ${miss.join(', ')}`);
  return a;
}
// The adapter of a game (loads it the first time), or null if none is registered.
export async function load3D(game) {
  const d = REG.get(String(game));
  if (!d) return null;
  if (typeof d !== 'function') return checkAdapter(d, game);
  const mod = await d();
  const a = (mod && (mod.adapter || mod.default)) || mod;
  checkAdapter(a, game);
  REG.set(String(game), a);
  return a;
}

// ---------------------------------------------------------------------------
// Pure maths: angles, camera rig, entity selection, two-bone IK
// ---------------------------------------------------------------------------
const TAU = Math.PI * 2;
export function wrapAngle(a) { a = ((a + Math.PI) % TAU + TAU) % TAU - Math.PI; return a === -Math.PI ? Math.PI : a; }
// Exponential smoothing towards b at rate k (per second): frame-rate independent.
export function damp(a, b, k, dt) { return lerp(a, b, 1 - Math.exp(-k * Math.max(0, dt))); }
export function dampAngle(a, b, k, dt) { return wrapAngle(a + wrapAngle(b - a) * (1 - Math.exp(-k * Math.max(0, dt)))); }
// Unit look vector for yaw (0 = +X, counter-clockwise from above) and pitch (up > 0).
export function forwardOf(yaw, pitch = 0) { const c = Math.cos(pitch); return [c * Math.cos(yaw), Math.sin(pitch), -c * Math.sin(yaw)]; }
// The yaw and pitch that look along a vector.
export function yawPitchOf(v) { return { yaw: Math.atan2(-v[2], v[0]), pitch: Math.atan2(v[1], Math.hypot(v[0], v[2])) }; }

export const PITCH_MAX = 1.35;
export function createRig(o = {}) {
  return {
    mode: o.mode === 'free' ? 'free' : 'first',
    pos: null, yaw: 0, pitch: 0, fov: o.fov || 72,
    lookYaw: 0, lookPitch: 0, lastInput: -1e9, dragging: false, recenter: o.recenter !== false,
    switchT: 0, watch: null,
    free: { yaw: o.freeYaw != null ? o.freeYaw : 2.4, pitch: o.freePitch != null ? o.freePitch : .45, dist: o.freeDist || 24, focus: null },
  };
}
// Drag to look: dx, dy in pixels. First person turns the head (offsets on top
// of where the entity looks); the free camera orbits its focus.
export function rigLook(rig, dx, dy, { sens = .0045, now = 0 } = {}) {
  if (rig.mode === 'free') {
    rig.free.yaw = wrapAngle(rig.free.yaw - dx * sens);
    rig.free.pitch = clamp(rig.free.pitch + dy * sens, .04, 1.45);
  } else {
    rig.lookYaw = wrapAngle(rig.lookYaw - dx * sens);
    rig.lookPitch = clamp(rig.lookPitch - dy * sens, -PITCH_MAX * 2, PITCH_MAX * 2);
  }
  rig.lastInput = now;
}
export function rigZoom(rig, factor) { rig.free.dist = clamp(rig.free.dist * factor, 4, 140); }
export function rigRecenter(rig) { rig.lookYaw = 0; rig.lookPitch = 0; }
// A new entity to look through: blend there in ~0.45 s (cut = jump at once).
export function rigWatch(rig, id, { cut = false } = {}) {
  if (id === rig.watch) return false;
  const first = rig.watch == null;
  rig.watch = id;
  rig.switchT = cut || first ? 0 : .45;
  if (cut) rig.pos = null;
  return true;
}
// One frame of first-person following. target = { pos, yaw, pitch, fov?, cut? }.
// The head follows the eye tightly (a body does not lag behind itself), the
// angles a little softer; a switch between entities blends more slowly.
export function rigFollow(rig, target, dt, now = 0) {
  rig.switchT = Math.max(0, rig.switchT - dt);
  if (!rig.pos || target.cut) {
    rig.pos = target.pos.slice(); rig.yaw = target.yaw; rig.pitch = target.pitch || 0;
  } else {
    const sw = rig.switchT > 0, kP = sw ? 7 : 30, kA = sw ? 6 : 9;
    for (let i = 0; i < 3; i++) rig.pos[i] = damp(rig.pos[i], target.pos[i], kP, dt);
    rig.yaw = dampAngle(rig.yaw, target.yaw, kA, dt);
    rig.pitch = damp(rig.pitch, target.pitch || 0, kA, dt);
  }
  if (target.fov) rig.fov = damp(rig.fov, target.fov, 4, dt);
  // Let go for a while: the head turns back to where the body looks.
  if (rig.recenter && !rig.dragging && now - rig.lastInput > 2500) {
    rig.lookYaw = damp(rig.lookYaw, 0, 1.4, dt); rig.lookPitch = damp(rig.lookPitch, 0, 1.4, dt);
  }
  return rigView(rig);
}
// One frame of the free camera: orbit focus at free.dist (the focus is smoothed).
export function rigFreeStep(rig, focus, dt) {
  const f = rig.free;
  if (!f.focus) f.focus = focus.slice();
  else for (let i = 0; i < 3; i++) f.focus[i] = damp(f.focus[i], focus[i], 4, dt);
  return rigView(rig);
}
// Where the camera is and what it looks at.
export function rigView(rig) {
  if (rig.mode === 'free') {
    const f = rig.free, fo = f.focus || [0, 0, 0], d = forwardOf(f.yaw, f.pitch);
    const pos = [fo[0] + d[0] * f.dist, fo[1] + d[1] * f.dist, fo[2] + d[2] * f.dist];
    const back = yawPitchOf([fo[0] - pos[0], fo[1] - pos[1], fo[2] - pos[2]]);
    return { pos, look: fo.slice(), yaw: back.yaw, pitch: back.pitch, fov: 55 };
  }
  const pos = rig.pos || [0, 2, 0];
  const yaw = wrapAngle(rig.yaw + rig.lookYaw), pitch = clamp(rig.pitch + rig.lookPitch, -PITCH_MAX, PITCH_MAX);
  const d = forwardOf(yaw, pitch);
  return { pos: pos.slice(), look: [pos[0] + d[0], pos[1] + d[1], pos[2] + d[2]], yaw, pitch, fov: rig.fov };
}

// Next/previous id in the list (wraps); from an id not in the list: the first/last.
export function cycleEntity(ids, current, step = 1) {
  if (!ids || !ids.length) return null;
  const i = ids.indexOf(current);
  if (i < 0) return ids[step > 0 ? 0 : ids.length - 1];
  const n = ids.length;
  return ids[(((i + step) % n) + n) % n];
}
// Who is watched: the locked id if it is on the field, else the automatic one.
// missing: a lock that is not in this play (kept: it comes back when they do).
export function resolveWatch(lock, ids, autoId) {
  const has = id => id != null && ids.includes(id);
  if (has(lock)) return { id: lock, auto: false, missing: false };
  const missing = lock != null;
  if (has(autoId)) return { id: autoId, auto: true, missing };
  return { id: ids.length ? ids[0] : null, auto: true, missing };
}
// Two-bone IK (law of cosines): upper L1, lower L2, distance D to the target.
// shoulder: angle between the upper bone and the line to the target;
// elbow: how much the joint bends (0 = straight). D is clamped to what reaches.
export function twoBoneIK(L1, L2, D) {
  const lo = Math.abs(L1 - L2) + 1e-4, hi = L1 + L2 - 1e-4, d = clamp(D, lo, hi);
  const c1 = clamp((L1 * L1 + d * d - L2 * L2) / (2 * L1 * d), -1, 1);
  const c2 = clamp((L1 * L1 + L2 * L2 - d * d) / (2 * L1 * L2), -1, 1);
  return { shoulder: Math.acos(c1), elbow: Math.PI - Math.acos(c2), reach: d };
}
// The picker line: "Watching: #11 Siroco · QB (auto) · Tab/Q-E to switch".
export function pickerText(e, { auto = false, missing = false, free = false, L = STR.en } = {}) {
  if (free) return L.free;
  if (!e) return L.nobody;
  const tail = auto ? ` (${missing ? L.lockedOff : L.auto})` : ' 🔒';
  return `${L.watching}: ${e.name}${e.role ? ' · ' + e.role : ''}${tail}`;
}
// WebGL2 available? (three.js r163+ needs WebGL 2.)
export function webglSupport(doc = typeof document !== 'undefined' ? document : null) {
  try {
    if (!doc || typeof doc.createElement !== 'function') return false;
    const c = doc.createElement('canvas');
    const gl = c.getContext && c.getContext('webgl2');
    if (!gl) return false;
    const lose = gl.getExtension && gl.getExtension('WEBGL_lose_context'); if (lose) lose.loseContext();
    return true;
  } catch (e) { return false; }
}

const STR = {
  en: { watching: 'Watching', auto: 'auto', lockedOff: 'your pick is off the field', nobody: 'Waiting for the next play…', free: '🎥 Free camera · drag to orbit, wheel to zoom',
    keys: 'Drag to look · Tab/Q-E switch player · A auto · F free camera · R recentre · Esc back to 2D', prev: 'Previous player (Q)', next: 'Next player (E / Tab)',
    autoBtn: 'Auto', freeBtn: '🎥 Free', firstBtn: '👁 First person', exit: '✕ 2D', exitHelp: 'Back to the 2D view (Esc)', noWebGL: 'First person 3D needs WebGL 2, which this browser or GPU does not offer. Staying on the 2D view.', lost: 'The 3D view lost its graphics context. Back to 2D.', loading: 'Loading 3D…', failed: 'The 3D view could not start: ' },
  ca: { watching: 'Mirant', auto: 'auto', lockedOff: 'el teu jugador no és al camp', nobody: 'Esperant la jugada següent…', free: '🎥 Càmera lliure · arrossega per girar, roda per apropar',
    keys: 'Arrossega per mirar · Tab/Q-E canvia de jugador · A auto · F càmera lliure · R centra · Esc torna al 2D', prev: 'Jugador anterior (Q)', next: 'Jugador següent (E / Tab)',
    autoBtn: 'Auto', freeBtn: '🎥 Lliure', firstBtn: '👁 Primera persona', exit: '✕ 2D', exitHelp: 'Torna a la vista 2D (Esc)', noWebGL: 'La primera persona en 3D necessita WebGL 2, i aquest navegador o targeta gràfica no el té. Es queda la vista 2D.', lost: 'La vista 3D ha perdut el context gràfic. Tornem al 2D.', loading: 'Carregant el 3D…', failed: 'La vista 3D no s\'ha pogut engegar: ' },
};
export const TEXT3D = STR;

// ---------------------------------------------------------------------------
// The kit: shared materials, lights, characters, crowd (needs THREE)
// ---------------------------------------------------------------------------
export function makeKit(THREE) {
  const colorCache = new Map();
  const col = c => { const k = String(c); let v = colorCache.get(k); if (!v) { v = new THREE.Color(c); colorCache.set(k, v); } return v; };
  const mats = {
    vc: new THREE.MeshLambertMaterial({ vertexColors: true }),
    basic: new THREE.MeshBasicMaterial({ vertexColors: true, fog: false }),
  };
  const lambert = new Map();
  const mat = (c, o = {}) => {
    const k = c + JSON.stringify(o);
    let m = lambert.get(k);
    if (!m) { m = new THREE.MeshLambertMaterial({ color: col(c), ...o }); lambert.set(k, m); }
    return m;
  };
  const owned = [];
  const own = x => { owned.push(x); return x; };

  // Merge geometries into one with a colour per part (one draw call per limb).
  // parts: [{ g: BufferGeometry, c: colour, m?: Matrix4 }]
  function merge(parts) {
    const pos = [], nor = [], colr = [], idx = [];
    let base = 0;
    for (const p of parts) {
      const g = p.g.index ? p.g.clone() : p.g.clone();
      if (p.m) g.applyMatrix4(p.m);
      const P = g.attributes.position, N = g.attributes.normal, c = col(p.c);
      for (let i = 0; i < P.count; i++) {
        pos.push(P.getX(i), P.getY(i), P.getZ(i));
        if (N) nor.push(N.getX(i), N.getY(i), N.getZ(i)); else nor.push(0, 1, 0);
        colr.push(c.r, c.g, c.b);
      }
      if (g.index) for (let i = 0; i < g.index.count; i++) idx.push(base + g.index.getX(i));
      else for (let i = 0; i < P.count; i++) idx.push(base + i);
      base += P.count;
      g.dispose();
    }
    const out = new THREE.BufferGeometry();
    out.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
    out.setAttribute('normal', new THREE.Float32BufferAttribute(nor, 3));
    out.setAttribute('color', new THREE.Float32BufferAttribute(colr, 3));
    out.setIndex(idx);
    out.computeBoundingSphere();
    return own(out);
  }
  const M = (x = 0, y = 0, z = 0, sx = 1, sy = 1, sz = 1, rx = 0, ry = 0, rz = 0) => {
    const m = new THREE.Matrix4();
    m.compose(new THREE.Vector3(x, y, z), new THREE.Quaternion().setFromEuler(new THREE.Euler(rx, ry, rz)), new THREE.Vector3(sx, sy, sz));
    return m;
  };
  const G = {
    box: new THREE.BoxGeometry(1, 1, 1),
    cyl: new THREE.CylinderGeometry(.5, .5, 1, 7, 1),
    cone: new THREE.CylinderGeometry(.18, .5, 1, 8, 1, true),
    sph: new THREE.SphereGeometry(.5, 10, 7),
    hemi: new THREE.SphereGeometry(.5, 10, 5, 0, Math.PI * 2, 0, Math.PI / 2),
    cap: new THREE.CylinderGeometry(.5, .5, 1, 9, 1),
  };

  function lights(scene, { sky = '#b9c6ff', ground = '#2b3a2b', hemi = 1.25, sun = '#fff4dc', sunI = 1.7, dir = [-40, 90, 30] } = {}) {
    const h = new THREE.HemisphereLight(col(sky), col(ground), hemi);
    const d = new THREE.DirectionalLight(col(sun), sunI); d.position.set(...dir);
    scene.add(h, d);
    return { hemi: h, sun: d };
  }

  // A canvas texture (sRGB, mipmapped, anisotropic). draw(g, w, h) paints it.
  function canvasTexture(w, h, draw, { aniso = 4, repeat = false } = {}) {
    const c = document.createElement('canvas'); c.width = w; c.height = h;
    const g = c.getContext('2d');
    draw(g, w, h);
    const t = new THREE.CanvasTexture(c);
    t.colorSpace = THREE.SRGBColorSpace; t.anisotropy = aniso;
    if (repeat) t.wrapS = t.wrapT = THREE.RepeatWrapping;
    t.redraw = fn => { fn(g, w, h); t.needsUpdate = true; };
    return own(t);
  }

  // ---- the humanoid rig ----
  // Model space: feet at y = 0, faces +X, right hand on +Z. Total height 2
  // (scaled by spec.height). Parts are vertex-coloured meshes sharing one
  // material, except the torso when it carries a texture (numbers).
  // spec: { height, build: { shoulders, chest, hips, bulk, slim, robe },
  //   colors: { torso, torso2, sleeve, arm, hand, pelvis, thigh, shin, shoe, skin },
  //   head: (h) => Mesh|Group placed at the neck (y = head centre), torsoMap: Texture }
  function humanoid(spec = {}) {
    const S = spec.height ? spec.height / 2 : 1, b = spec.build || {}, c = spec.colors || {};
    const bulk = b.bulk || 1, slim = b.slim || 1;
    const D = {
      hip: 1.0, thigh: .48, shin: .44, upper: .3, lower: .27,
      shoulderY: .55, shoulderZ: .25 * (b.shoulders || 1), headY: .8, legZ: .1 * bulk,
    };
    const skin = c.skin || '#c48d66';
    const root = new THREE.Group(), tilt = new THREE.Group(), yawG = new THREE.Group(), hips = new THREE.Group(), spine = new THREE.Group();
    root.add(tilt); tilt.add(yawG); yawG.add(hips); hips.add(spine);
    yawG.scale.setScalar(S);
    hips.position.y = D.hip;
    const mk = (geo, material = mats.vc) => { const m = new THREE.Mesh(geo, material); m.matrixAutoUpdate = true; return m; };
    // Torso: a tapered box (wide shoulders, narrow waist), optional texture.
    const tw = .44 * (b.chest || 1) * bulk * slim, td = .27 * bulk;
    let torso;
    if (spec.torsoMap) {
      const g = new THREE.BoxGeometry(td, .58, tw);
      // Back (-X) shows the left half of the map, the chest (+X) the right
      // half, the other faces a plain corner of it (the jersey colour).
      const uv = g.attributes.uv;
      for (let f = 0; f < 6; f++) for (let k = 0; k < 4; k++) {
        const i = f * 4 + k, u = uv.getX(i), v = uv.getY(i);
        if (f === 0) uv.setXY(i, .5 + u * .5, v);
        else if (f === 1) uv.setXY(i, u * .5, v);
        else uv.setXY(i, .005, .5);
      }
      const P = g.attributes.position; for (let i = 0; i < P.count; i++) if (P.getY(i) < 0) P.setZ(i, P.getZ(i) * .78);
      g.computeVertexNormals();
      own(g);
      torso = mk(g, own(new THREE.MeshLambertMaterial({ map: spec.torsoMap })));
    } else {
      const g = merge([{ g: G.box, c: c.torso || '#666', m: M(0, 0, 0, td, .58, tw) }]);
      const P = g.attributes.position; for (let i = 0; i < P.count; i++) if (P.getY(i) < 0) P.setZ(i, P.getZ(i) * .78);
      g.computeVertexNormals();
      torso = mk(g);
    }
    torso.position.y = .33; spine.add(torso);
    // Shoulders (pads or a yoke) and neck.
    const sh = merge([
      { g: G.sph, c: c.torso2 || c.torso || '#666', m: M(0, .58, 0, td * 1.12, .2, (2 * D.shoulderZ + .16) * slim) },
      { g: G.cyl, c: skin, m: M(0, .68, 0, .12, .14, .12) },
    ]);
    spine.add(mk(sh));
    // Pelvis.
    const pel = merge([{ g: G.box, c: c.pelvis || c.thigh || '#444', m: M(0, -.02, 0, td * .9, .2, tw * .82) }]);
    hips.add(mk(pel));
    // Head (game-specific headwear), at the neck.
    const head = new THREE.Group(); head.position.y = D.headY; spine.add(head);
    const hm = spec.head ? spec.head(skin) : mk(merge([{ g: G.sph, c: skin, m: M(0, 0, 0, .25, .28, .24) }]));
    head.add(hm);
    // Robe (mages, supports): a cone from the waist to the ankles; hides the legs.
    let robe = null;
    if (b.robe) { robe = mk(merge([{ g: G.cone, c: c.robe || c.torso || '#555', m: M(0, -.48, 0, .62 * bulk, .98, .62 * bulk) }])); hips.add(robe); }
    // Limbs: a group at the joint, the mesh hanging down (-Y).
    const limb = (len, w, parts) => { const grp = new THREE.Group(); grp.add(mk(merge(parts.map(p => ({ g: p.g || G.cyl, c: p.c, m: M(p.x || 0, -len * (p.y0 + p.y1) / 2, 0, w * (p.w || 1), len * (p.y1 - p.y0), w * (p.w || 1)) }))))); return grp; };
    const arm = side => {
      const up = limb(D.upper, .13 * bulk, [{ c: c.sleeve || c.torso || '#666', y0: 0, y1: .55, w: 1.15 }, { c: c.arm || skin, y0: .55, y1: 1 }]);
      up.position.set(0, D.shoulderY, side * D.shoulderZ);
      const lo = limb(D.lower, .11 * bulk, [{ c: c.arm || skin, y0: 0, y1: .8 }, { g: G.box, c: c.hand || skin, y0: .8, y1: 1.2, w: .9 }]);
      lo.position.y = -D.upper; up.add(lo);
      spine.add(up);
      return { up, lo, side };
    };
    const leg = side => {
      const th = limb(D.thigh, .19 * bulk, [{ c: c.thigh || '#444', y0: 0, y1: 1 }]);
      th.position.set(0, -.05, side * D.legZ);
      const sn = limb(D.shin, .14 * bulk, [{ c: c.shin || c.thigh || '#444', y0: 0, y1: .86 }, { g: G.box, c: c.shoe || '#18181c', y0: .86, y1: 1.12, w: 1.2, x: .05 }]);
      sn.position.y = -D.thigh; th.add(sn);
      hips.add(th);
      if (robe) th.visible = false;
      return { th, sn, side };
    };
    const armL = arm(-1), armR = arm(1), legL = leg(-1), legR = leg(1);
    const h = { root, tilt, yawG, hips, spine, torso, head, armL, armR, legL, legR, D, S, robe };
    h.pose = p => poseHumanoid(h, p);
    h.handR = armR.lo; h.handL = armL.lo; // attach props here (y = -D.lower)
    return h;
  }

  const _v = new THREE.Vector3(), _w = new THREE.Vector3(), _e = new THREE.Vector3(), _d = new THREE.Vector3(), _q = new THREE.Quaternion(), _q2 = new THREE.Quaternion(), _down = new THREE.Vector3(0, -1, 0), _ax = new THREE.Vector3();
  // p: { yaw, phase, run (0..1), lean, crouch, jump, fall: { k, dir: [x, z] } | null,
  //      arms: [left, right] world points (Vector3 or [x,y,z]) or null, kick,
//      hideHead, firstPerson (hides the upper body of the one we look through) }
  function poseHumanoid(h, p) {
    const D = h.D, run = clamp(p.run || 0, 0, 1), ph = p.phase || 0, cr = clamp(p.crouch || 0, 0, 1);
    const sw = Math.sin(ph) * run, bob = Math.abs(Math.cos(ph)) * .05 * run;
    // Fall: tip over towards dir, pivoting on the feet.
    const f = p.fall;
    if (f && f.k > 0) {
      const a = f.k * Math.PI / 2 * .85, n = Math.hypot(f.dir[0], f.dir[1]) || 1;
      _ax.set(f.dir[1] / n, 0, -f.dir[0] / n);
      h.tilt.quaternion.setFromAxisAngle(_ax, a);
    } else h.tilt.quaternion.identity();
    h.yawG.rotation.y = p.yaw || 0;
    h.hips.position.y = D.hip * (1 - cr * .34) + bob - (p.kneel ? .3 : 0);
    h.spine.rotation.z = -((p.lean || 0) * .62 + cr * .55 + run * .1);
    // Legs: swing while running; crouch bends hips and knees; kick swings the right one high.
    const legPose = (L, s) => {
      let t = s * .78 + cr * .9, k = -(.12 + Math.max(0, -Math.sin(ph) * s) * 1.1 * run + cr * 1.5);
      if (L === h.legR && p.kick) { t = lerp(t, 1.5, p.kick); k = lerp(k, -.1, p.kick); }
      L.th.rotation.z = t; L.sn.rotation.z = k;
    };
    legPose(h.legL, sw); legPose(h.legR, -sw);
    // Looking through this body: no head, shoulders or arms (they would fill the screen).
    h.head.visible = !p.hideHead && !p.firstPerson;
    h.spine.visible = !p.firstPerson;
    // Arms: swing, or reach for a world point (two-bone IK in spine space).
    const arms = p.arms || [null, null];
    const anyIK = arms[0] || arms[1];
    const swingArm = (A, s) => { A.up.quaternion.setFromAxisAngle(_ax.set(0, 0, 1), -s * .8 + (run ? .1 : 0)); A.up.rotateX(A.side * -.12); A.lo.quaternion.setFromAxisAngle(_ax.set(0, 0, 1), .25 + run * .9); };
    swingArm(h.armL, sw); swingArm(h.armR, -sw);
    if (anyIK && !p.firstPerson) {
      h.root.updateMatrixWorld(true);
      [h.armL, h.armR].forEach((A, i) => {
        const tgt = arms[i];
        if (!tgt) return;
        if (Array.isArray(tgt)) _v.set(tgt[0], tgt[1], tgt[2]); else _v.copy(tgt);
        h.spine.worldToLocal(_v);
        _w.copy(A.up.position);
        _d.subVectors(_v, _w);
        const L1 = D.upper, L2 = D.lower + .04, dist = _d.length();
        if (dist < 1e-4) return;
        _d.divideScalar(dist);
        const ik = twoBoneIK(L1, L2, dist);
        // Elbow bends down and out.
        _e.set(0, -1, A.side * .7).normalize();
        _e.addScaledVector(_d, -_e.dot(_d));
        if (_e.lengthSq() < 1e-6) _e.set(0, 0, A.side); else _e.normalize();
        const elbowDir = _ax.copy(_d).multiplyScalar(Math.cos(ik.shoulder)).addScaledVector(_e, Math.sin(ik.shoulder));
        A.up.quaternion.setFromUnitVectors(_down, elbowDir);
        // Lower bone towards the target (from the elbow), in the upper bone's frame.
        const elbow = _w.addScaledVector(elbowDir, L1);
        _d.copy(_v).sub(elbow);
        if (_d.lengthSq() < 1e-8) _d.copy(elbowDir); else _d.normalize();
        _q.setFromUnitVectors(_down, _d);
        _q2.copy(A.up.quaternion).invert();
        A.lo.quaternion.copy(_q2.multiply(_q));
      });
    }
  }

  // ---- presets ----
  // American football player: helmet with stripe and facemask, shoulder pads,
  // numbers on the chest and back.
  function footballPlayer({ jersey = '#1E5BD8', trim = '#ffffff', pants = '#e8e8e8', helmet, mask = '#c9ccd2', skin = '#c48d66', number = '', bulk = 1, height = 2 } = {}) {
    const hcol = helmet || jersey;
    const tex = number !== '' && typeof document !== 'undefined' ? canvasTexture(256, 128, (g, w, h) => {
      g.fillStyle = jersey; g.fillRect(0, 0, w, h);
      g.fillStyle = trim; g.fillRect(0, 0, w, 10); g.fillRect(0, h - 8, w, 8);
      const num = (x, size) => { g.font = `900 ${size}px "Segoe UI", system-ui, sans-serif`; g.textAlign = 'center'; g.textBaseline = 'middle'; g.lineWidth = size * .09; g.strokeStyle = 'rgba(0,0,0,.45)'; g.strokeText(String(number), x, h * .55); g.fillStyle = trim; g.fillText(String(number), x, h * .55); };
      num(w * .25, 92); num(w * .75, 58);
    }, { aniso: 2 }) : null;
    return humanoid({
      height, torsoMap: tex,
      build: { shoulders: 1.12, chest: 1.05, bulk },
      colors: { torso: jersey, torso2: jersey, sleeve: jersey, arm: skin, hand: '#202024', pelvis: pants, thigh: pants, shin: jersey, shoe: '#16161a', skin },
      head: () => new THREE.Mesh(merge([
        { g: G.sph, c: hcol, m: M(-.01, .02, 0, .34, .33, .3) },
        { g: G.sph, c: trim, m: M(-.01, .025, 0, .345, .34, .07) },
        { g: G.box, c: mask, m: M(.165, -.02, 0, .03, .025, .24) },
        { g: G.box, c: mask, m: M(.17, -.08, 0, .03, .025, .2) },
        { g: G.box, c: mask, m: M(.17, -.06, 0, .03, .12, .025) },
        { g: G.sph, c: skin, m: M(.08, -.04, 0, .16, .18, .17) },
      ]), mats.vc),
    });
  }
  // Counter-Strike soldier: helmet, vest, gloves. side 'ct' | 't'.
  function soldier({ side = 'ct', skin = '#c48d66', height = 2, uniform, vest } = {}) {
    const u = uniform || (side === 't' ? '#8a7a55' : '#4b5a6b'), v = vest || (side === 't' ? '#5d5440' : '#2f3a46');
    return humanoid({
      height, build: { shoulders: 1, chest: 1.08, bulk: 1 },
      colors: { torso: v, torso2: v, sleeve: u, arm: u, hand: '#1d1d20', pelvis: u, thigh: u, shin: u, shoe: '#2a2520', skin },
      head: () => new THREE.Mesh(merge([
        { g: G.sph, c: skin, m: M(.02, -.03, 0, .23, .26, .22) },
        { g: G.hemi, c: side === 't' ? '#4a4535' : '#33404d', m: M(0, .02, 0, .3, .26, .29) },
        { g: G.box, c: '#151515', m: M(.11, .0, 0, .04, .04, .2) },
      ]), mats.vc),
    });
  }
  // League of Legends champion with a class silhouette.
  // cls: tank | fighter | mage | marksman | assassin | support
  function champion({ cls = 'fighter', color = '#3e7bfa', color2 = '#ffd400', skin = '#c48d66', height = 2 } = {}) {
    const B = {
      tank: { shoulders: 1.45, chest: 1.3, bulk: 1.3 }, fighter: { shoulders: 1.15, chest: 1.1, bulk: 1.05 },
      mage: { shoulders: .9, chest: .9, bulk: .95, robe: true }, support: { shoulders: .95, chest: .95, bulk: 1, robe: true },
      marksman: { shoulders: .95, chest: .9, bulk: .9, slim: .9 }, assassin: { shoulders: .9, chest: .85, bulk: .88, slim: .85 },
    }[cls] || { shoulders: 1, chest: 1, bulk: 1 };
    const heads = {
      tank: [{ g: G.sph, c: color, m: M(0, 0, 0, .34, .32, .32) }, { g: G.cone, c: color2, m: M(0, .2, .17, .07, .26, .07, .6, 0, 0) }, { g: G.cone, c: color2, m: M(0, .2, -.17, .07, .26, .07, -.6, 0, 0) }],
      fighter: [{ g: G.sph, c: color, m: M(0, 0, 0, .3, .3, .28) }, { g: G.box, c: color2, m: M(-.04, .2, 0, .3, .12, .04) }],
      mage: [{ g: G.sph, c: skin, m: M(.02, -.02, 0, .24, .27, .23) }, { g: G.cone, c: color, m: M(0, .22, 0, .4, .5, .4, 0, 0, 0) }],
      support: [{ g: G.sph, c: skin, m: M(.02, -.02, 0, .24, .27, .23) }, { g: G.cyl, c: color2, m: M(0, .24, 0, .34, .02, .34) }],
      marksman: [{ g: G.sph, c: skin, m: M(.02, -.02, 0, .23, .26, .22) }, { g: G.hemi, c: color, m: M(-.03, -.01, 0, .3, .34, .29) }],
      assassin: [{ g: G.sph, c: '#20222b', m: M(0, 0, 0, .26, .28, .25) }, { g: G.box, c: color2, m: M(.12, .02, 0, .03, .03, .18) }],
    }[cls] || [{ g: G.sph, c: skin, m: M(0, 0, 0, .25, .28, .24) }];
    return humanoid({
      height, build: B,
      colors: { torso: color, torso2: cls === 'tank' ? color2 : color, sleeve: color, arm: cls === 'tank' ? color : skin, hand: cls === 'tank' ? color2 : skin, pelvis: '#2a2c36', thigh: '#2a2c36', shin: color, shoe: '#1a1a20', robe: color, skin },
      head: () => new THREE.Mesh(merge(heads), mats.vc),
    });
  }

  // ---- crowd: one instanced mesh (body and head, 16 triangles a person) ----
  // people: [{ x, y, z, yaw, color, team (0|1|-1), ph, th }] (th: angle round the bowl, for the wave)
  // Everyone faces +X before yaw (towards the field): no back or bottom faces.
  // update(clock, hype, hypeTeam) moves them on the GPU (no per-frame CPU work).
  function crowd(people, { body = [.46, .62, .34], head = .24, skinColor = '#c99f7e' } = {}) {
    const n = people.length;
    const uni = { uTime: { value: 0 }, uHype: { value: 0 }, uTeam: { value: -1 }, uSkin: { value: col(skinColor).clone() } };
    // A box without its back (-X) and bottom (-Y) faces, translated.
    const part = (sx, sy, sz, y, isHead) => {
      const g = new THREE.BoxGeometry(sx, sy, sz); g.translate(0, y, 0);
      const keep = [0, 2, 4, 5], idx = [], src = g.index.array;
      for (const f of keep) for (let k = 0; k < 6; k++) idx.push(src[f * 6 + k]);
      g.setIndex(idx);
      g.setAttribute('aHead', new THREE.Float32BufferAttribute(new Float32Array(g.attributes.position.count).fill(isHead ? 1 : 0), 1));
      return g;
    };
    const parts = [part(body[0], body[1], body[2], body[1] / 2, false), part(head, head, head, body[1] + head * .6, true)];
    const geo = new THREE.BufferGeometry(), P = [], N = [], H = [], I = [];
    let base = 0;
    for (const g of parts) {
      P.push(...g.attributes.position.array); N.push(...g.attributes.normal.array); H.push(...g.attributes.aHead.array);
      for (const i of g.index.array) I.push(base + i);
      base += g.attributes.position.count; g.dispose();
    }
    geo.setAttribute('position', new THREE.Float32BufferAttribute(P, 3));
    geo.setAttribute('normal', new THREE.Float32BufferAttribute(N, 3));
    geo.setAttribute('aHead', new THREE.Float32BufferAttribute(H, 1));
    geo.setIndex(I);
    const attr = new Float32Array(n * 3);
    people.forEach((p, i) => { attr[i * 3] = p.ph || 0; attr[i * 3 + 1] = p.team == null ? -1 : p.team; attr[i * 3 + 2] = p.th || 0; });
    geo.setAttribute('aCrowd', new THREE.InstancedBufferAttribute(attr, 3));
    own(geo);
    const material = own(new THREE.MeshLambertMaterial({ color: 0xffffff }));
    material.onBeforeCompile = sh => {
      Object.assign(sh.uniforms, uni);
      sh.vertexShader = sh.vertexShader
        .replace('#include <common>', `#include <common>
        attribute vec3 aCrowd;
        attribute float aHead;
        uniform float uTime;
        uniform float uHype;
        uniform float uTeam;
        uniform vec3 uSkin;`)
        .replace('#include <color_vertex>', `#include <color_vertex>
        #ifdef USE_INSTANCING_COLOR
        vColor.xyz = mix(vColor.xyz, uSkin, aHead);
        #endif`)
        .replace('#include <begin_vertex>', `#include <begin_vertex>
        float mine = aCrowd.y < -0.5 ? 0.4 : (abs(aCrowd.y - uTeam) < 0.5 ? 1.0 : 0.08);
        float ex = uHype * mine;
        float up = sin(uTime * 1.6 + aCrowd.x) * 0.05 + ex * abs(sin(uTime * 8.5 + aCrowd.x * 3.0)) * 0.55;
        float wt = mod(uTime, 40.0);
        if (wt < 13.0) { float wth = wt / 13.0 * 9.4248; float dth = abs(mod(aCrowd.z - wth + 9.4248, 6.2832) - 3.1416); up += 0.7 * exp(-dth * dth * 30.0); }
        transformed.y += up;`);
    };
    material.customProgramCacheKey = () => 'arena3d-crowd';
    const mesh = new THREE.InstancedMesh(geo, material, n);
    const m4 = new THREE.Matrix4(), q = new THREE.Quaternion(), one = new THREE.Vector3(1, 1, 1), up = new THREE.Vector3(0, 1, 0), v = new THREE.Vector3();
    people.forEach((p, i) => {
      q.setFromAxisAngle(up, p.yaw || 0); m4.compose(v.set(p.x, p.y, p.z), q, one);
      mesh.setMatrixAt(i, m4);
      mesh.setColorAt(i, col(p.color || '#888'));
    });
    mesh.instanceMatrix.needsUpdate = true;
    if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true;
    mesh.frustumCulled = false; // the bounce moves them past the bounds; one draw call anyway
    const group = new THREE.Group(); group.add(mesh);
    return { group, mesh, update(clock, hype = 0, team = -1) { uni.uTime.value = clock; uni.uHype.value = hype; uni.uTeam.value = team; }, count: n };
  }

  // Soft round shadows on the ground for n things (one draw call). set(i, x, z, r, alpha).
  function blobShadows(n, y = .02) {
    const tex = canvasTexture(64, 64, (g, w, h) => { const r = g.createRadialGradient(w / 2, h / 2, 0, w / 2, h / 2, w / 2); r.addColorStop(0, 'rgba(0,0,0,.55)'); r.addColorStop(1, 'rgba(0,0,0,0)'); g.fillStyle = r; g.fillRect(0, 0, w, h); }, { aniso: 1 });
    const geo = own(new THREE.PlaneGeometry(1, 1)); geo.rotateX(-Math.PI / 2);
    const mat = own(new THREE.MeshBasicMaterial({ map: tex, transparent: true, depthWrite: false, polygonOffset: true, polygonOffsetFactor: -4, polygonOffsetUnits: -4 }));
    const mesh = new THREE.InstancedMesh(geo, mat, n);
    mesh.frustumCulled = false; mesh.renderOrder = 1;
    const m4 = new THREE.Matrix4(), q = new THREE.Quaternion(), s = new THREE.Vector3(), p = new THREE.Vector3();
    return {
      mesh,
      set(i, x, z, r, a = 1) { m4.compose(p.set(x, y, z), q, s.set(2 * r * a, 1, 2 * r * a)); mesh.setMatrixAt(i, m4); },
      hide(i) { m4.makeScale(0, 0, 0); mesh.setMatrixAt(i, m4); },
      commit() { mesh.instanceMatrix.needsUpdate = true; },
    };
  }

  function dispose() {
    for (const x of owned) try { x.dispose(); } catch (e) { /* res */ }
    for (const m of lambert.values()) m.dispose();
    for (const k in mats) mats[k].dispose();
    for (const k in G) G[k].dispose();
  }

  return { THREE, col, mat, mats, merge, M, G, lights, canvasTexture, humanoid, poseHumanoid, footballPlayer, soldier, champion, crowd, blobShadows, own, dispose };
}

// ---------------------------------------------------------------------------
// The view: mount3D(host…) puts the 3D layer over the 2D canvas
// ---------------------------------------------------------------------------
let cssDone = false;
function css() {
  if (cssDone || typeof document === 'undefined' || document.getElementById('a3CSS')) return;
  cssDone = true;
  const st = document.createElement('style'); st.id = 'a3CSS';
  st.textContent = `
.a3-root { position:absolute; z-index:4; overflow:hidden; background:#05070f; font:13px/1.35 "Segoe UI", system-ui, sans-serif; color:#f3f5fb; user-select:none; -webkit-user-select:none; }
.a3-root canvas.a3-cv { position:absolute; inset:0; width:100%; height:100%; max-width:none; max-height:none; border-radius:0; box-shadow:none; display:block; touch-action:none; cursor:grab; outline:none; }
.a3-root canvas.a3-cv.drag { cursor:grabbing; }
.a3-mask { position:absolute; inset:0; pointer-events:none; opacity:0; transition:opacity .35s; }
.a3-mask.on { opacity:1; }
.a3-mask svg { position:absolute; left:0; bottom:0; width:100%; height:100%; }
.a3-hud { position:absolute; inset:0; pointer-events:none; }
.a3-score { position:absolute; top:8px; left:50%; transform:translateX(-50%); display:flex; align-items:stretch; background:rgba(8,10,20,.82); border-radius:10px; overflow:hidden; font-weight:800; box-shadow:0 6px 18px rgba(0,0,0,.35); }
.a3-score .t { display:flex; align-items:center; gap:10px; padding:4px 12px; min-width:92px; justify-content:space-between; }
.a3-score .t i { font-style:normal; font-size:18px; color:#ffd400; }
.a3-score .t .mk { width:7px; height:7px; border-radius:50%; background:#ffd400; display:inline-block; }
.a3-score .mid { display:flex; flex-direction:column; align-items:center; justify-content:center; padding:3px 12px; font-size:12px; min-width:84px; }
.a3-score .mid small { color:#ffd400; font-size:11px; }
.a3-tag { position:absolute; top:10px; left:10px; background:rgba(8,10,20,.78); border-radius:6px; padding:3px 10px; font-weight:800; font-size:11px; letter-spacing:.06em; }
.a3-tag:empty { display:none; }
.a3-tag.rep::before { content:''; display:inline-block; width:8px; height:8px; border-radius:50%; background:#ff3b3b; margin-right:6px; animation:a3blink 1s infinite; }
@keyframes a3blink { 50% { opacity:.25; } }
.a3-cap { position:absolute; top:58px; left:50%; transform:translateX(-50%); max-width:min(92%, 720px); text-align:center; background:rgba(8,10,20,.72); border-radius:8px; padding:4px 12px; font-size:12px; }
.a3-cap:empty { display:none; }
.a3-banner { position:absolute; left:50%; top:40%; transform:translate(-50%,-50%) skewX(-8deg); font:900 italic clamp(22px, 6vw, 54px)/1.1 "Segoe UI", system-ui, sans-serif; padding:6px 34px; opacity:0; transition:opacity .25s; white-space:nowrap; }
.a3-banner.on { opacity:1; animation:a3pop .35s ease-out; }
@keyframes a3pop { from { transform:translate(-50%,-50%) skewX(-8deg) scale(.6); } }
.a3-bar { position:absolute; left:8px; bottom:8px; display:flex; gap:6px; align-items:center; flex-wrap:wrap; max-width:calc(100% - 16px); pointer-events:auto; }
.a3-bar .who { background:rgba(8,10,20,.82); border-radius:8px; padding:5px 10px; font-weight:700; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; max-width:min(60vw, 420px); border-left:4px solid var(--tc, #ffd400); }
.a3-bar button { font:inherit; font-weight:700; border:1px solid rgba(255,255,255,.22); background:rgba(8,10,20,.82); color:#f3f5fb; border-radius:8px; padding:5px 9px; cursor:pointer; }
.a3-bar button:hover { background:rgba(40,46,70,.92); }
.a3-bar button.on { background:#ffd400; color:#111; border-color:#ffd400; }
.a3-help { position:absolute; right:8px; top:8px; background:rgba(8,10,20,.7); border-radius:8px; padding:4px 10px; font-size:11px; opacity:.95; transition:opacity .8s; max-width:calc(50% - 175px); text-align:right; }
.a3-help.gone { opacity:0; }
.a3-cut { position:absolute; inset:0; background:#fff; opacity:0; pointer-events:none; }
.a3-cut.go { animation:a3cut .22s ease-out; }
@keyframes a3cut { from { opacity:.55; } to { opacity:0; } }
.a3-msg { position:absolute; inset:0; display:flex; align-items:center; justify-content:center; text-align:center; padding:20px; font-weight:700; background:rgba(5,7,15,.88); }
.a3-msg[hidden] { display:none; }
.a3-toast { position:absolute; left:50%; top:50%; transform:translate(-50%,-50%); z-index:8; max-width:80%; background:rgba(8,10,20,.92); color:#fff; border:1px solid rgba(255,255,255,.2); border-radius:10px; padding:12px 16px; font:600 14px/1.4 "Segoe UI", system-ui, sans-serif; text-align:center; }
@media (max-width:640px) { .a3-help { display:none; } .a3-score .t { min-width:70px; padding:3px 8px; } }`;
  document.head.append(st);
}

// The facemask seen from inside the helmet: bars at the bottom, a soft vignette.
const HELMET_SVG = `<svg viewBox="0 0 1600 900" preserveAspectRatio="none" xmlns="http://www.w3.org/2000/svg"><defs>
<radialGradient id="a3v" cx="50%" cy="46%" r="75%"><stop offset="62%" stop-color="#000" stop-opacity="0"/><stop offset="100%" stop-color="#000" stop-opacity=".62"/></radialGradient>
<linearGradient id="a3b" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#aeb3bb"/><stop offset=".5" stop-color="#6f747d"/><stop offset="1" stop-color="#3d4148"/></linearGradient>
<filter id="a3f"><feGaussianBlur stdDeviation="5"/></filter></defs>
<rect width="1600" height="900" fill="url(#a3v)"/>
<g filter="url(#a3f)" opacity=".55" fill="none" stroke="url(#a3b)" stroke-linecap="round">
<path d="M-60 770 Q800 640 1660 770" stroke-width="40"/><path d="M-60 900 Q800 800 1660 900" stroke-width="36"/>
<path d="M800 660 L800 920" stroke-width="34"/><path d="M360 700 L250 930" stroke-width="30"/><path d="M1240 700 L1350 930" stroke-width="30"/></g>
<path d="M0 0 L0 900 L-30 900Z M1600 0 L1630 900 L1600 900Z" fill="#000" opacity=".5"/></svg>`;
const VISOR_SVG = `<svg viewBox="0 0 1600 900" preserveAspectRatio="none" xmlns="http://www.w3.org/2000/svg"><defs><radialGradient id="a3w" cx="50%" cy="48%" r="72%"><stop offset="66%" stop-color="#000" stop-opacity="0"/><stop offset="100%" stop-color="#000" stop-opacity=".55"/></radialGradient></defs><rect width="1600" height="900" fill="url(#a3w)"/></svg>`;

const now = () => (typeof performance !== 'undefined' ? performance.now() : Date.now());

// A short message over the 2D view (e.g. no WebGL).
export function toast3D(host, text, ms = 4500) {
  if (!host || typeof document === 'undefined') return;
  css();
  const t = document.createElement('div'); t.className = 'a3-toast'; t.textContent = text; t.setAttribute('role', 'status');
  host.append(t);
  setTimeout(() => t.remove(), ms);
}

// mount3D(cover, opts) → Promise<handle | null>
//   cover: the 2D viewer's canvas; the 3D layer is laid exactly over it (in
//          its parent, which must be positioned).
//   opts: { game, view, lang = 'en', onExit(reason), onFail(message),
//           lock (an entity id to start on), free (start on the free camera) }
//   handle: { destroy(), stats(), bench(n), setLock(id), toggleFree(), el, ctx }
// Returns null (after showing a message over the 2D view and calling onFail)
// when there is no adapter, no WebGL 2 or the scene cannot be built.
export async function mount3D(cover, opts = {}) {
  const L = STR[opts.lang === 'ca' ? 'ca' : 'en'];
  const host = cover && cover.parentElement;
  const fail = msg => { toast3D(host, msg); if (opts.onFail) opts.onFail(msg); return null; };
  if (!host || !opts.view) return fail(L.failed + 'no viewer');
  if (!has3D(opts.game)) return fail(L.failed + 'no 3D view for ' + opts.game);
  if (!webglSupport()) return fail(L.noWebGL);
  css();
  let THREE, adapter;
  try {
    [THREE, adapter] = await Promise.all([import('./vendor/three.module.min.js'), load3D(opts.game)]);
  } catch (e) { return fail(L.failed + (e && e.message || e)); }

  // ---- DOM ----
  const root = document.createElement('div'); root.className = 'a3-root';
  root.innerHTML = `<canvas class="a3-cv" tabindex="0" aria-label="3D view"></canvas><div class="a3-mask"></div><div class="a3-cut"></div>
<div class="a3-hud"><div class="a3-score" hidden></div><div class="a3-tag"></div><div class="a3-cap"></div><div class="a3-banner"></div>
<div class="a3-bar"><button type="button" data-a="prev" title="${esc(L.prev)}">◀</button><span class="who"></span><button type="button" data-a="next" title="${esc(L.next)}">▶</button><button type="button" data-a="auto">${esc(L.autoBtn)}</button><button type="button" data-a="free">${esc(L.freeBtn)}</button><button type="button" data-a="exit" title="${esc(L.exitHelp)}">${esc(L.exit)}</button></div>
<div class="a3-help">${esc(L.keys)}</div></div><div class="a3-msg" hidden></div>`;
  const q = s => root.querySelector(s);
  const cv = q('canvas'), maskEl = q('.a3-mask'), scoreEl = q('.a3-score'), tagEl = q('.a3-tag'), capEl = q('.a3-cap'), banEl = q('.a3-banner'), whoEl = q('.who'), msgEl = q('.a3-msg'), helpEl = q('.a3-help'), cutEl = q('.a3-cut');
  const place = () => {
    root.style.left = cover.offsetLeft + 'px'; root.style.top = cover.offsetTop + 'px';
    root.style.width = cover.offsetWidth + 'px'; root.style.height = cover.offsetHeight + 'px';
    try { root.style.borderRadius = getComputedStyle(cover).borderRadius; } catch (e) { /* res */ }
  };
  place();
  host.append(root);

  let renderer;
  try {
    renderer = new THREE.WebGLRenderer({ canvas: cv, antialias: true, powerPreference: 'high-performance', alpha: false, stencil: false });
  } catch (e) { root.remove(); return fail(L.noWebGL); }
  renderer.setClearColor(0x05070f, 1);
  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(72, 16 / 9, .05, 900);
  const kit = makeKit(THREE);
  const ctx = { THREE, scene, camera, renderer, kit, view: opts.view, game: opts.game, lang: opts.lang === 'ca' ? 'ca' : 'en', state: {}, watch: { id: null, mode: 'first' }, quality: { low: lowPower() }, root };
  try { await adapter.build(ctx); } catch (e) {
    console.error('arena 3D: build failed', e);
    try { renderer.dispose(); kit.dispose(); } catch (x) { /* res */ }
    root.remove(); return fail(L.failed + (e && e.message || e));
  }

  // ---- state ----
  const rig = createRig({ mode: opts.free ? 'free' : 'first' });
  const fs = adapter.freeStart ? adapter.freeStart(ctx) : null;
  if (fs) Object.assign(rig.free, fs);
  let lock = opts.lock != null ? opts.lock : null, stopped = false, rafId = 0, last = now(), clock = 0;
  const vis = { page: typeof document === 'undefined' || !document.hidden, screen: true };
  let pr = Math.min(viewDpr(), 1.5), sizeKey = '';
  const perf = { ms: 0, gap: 0, n: 0, slow: 0, lastHud: '' };

  const resize = () => {
    place();
    const w = Math.max(2, root.clientWidth), h = Math.max(2, root.clientHeight), key = w + 'x' + h + '@' + pr;
    if (key === sizeKey) return;
    sizeKey = key;
    renderer.setPixelRatio(pr); renderer.setSize(w, h, false);
    camera.aspect = w / h; camera.updateProjectionMatrix();
  };
  resize();

  // ---- the head-up display (DOM only touched when a string changes) ----
  const hudKeys = {};
  const setHTML = (el, k, html) => { if (hudKeys[k] !== html) { hudKeys[k] = html; el.innerHTML = html; } };
  const setText = (el, k, text) => { if (hudKeys[k] !== text) { hudKeys[k] = text; el.textContent = text; } };
  function paintHud(entities, w) {
    const h = adapter.hud ? adapter.hud(ctx) || {} : {};
    if (h.teams && h.teams.length) {
      scoreEl.hidden = false;
      const team = (t, i) => `<span class="t" style="border-${i ? 'right' : 'left'}:5px solid ${esc(t.color || '#888')}">${i ? `<i>${esc(t.score)}</i>${t.mark ? '<span class="mk"></span>' : ''}<b>${esc(t.name)}</b>` : `<b>${esc(t.name)}</b>${t.mark ? '<span class="mk"></span>' : ''}<i>${esc(t.score)}</i>`}</span>`;
      setHTML(scoreEl, 'score', team(h.teams[0], 0) + `<span class="mid"><b>${esc(h.clock || '')}</b>${h.sub ? `<small>${esc(h.sub)}</small>` : ''}</span>` + team(h.teams[1], 1));
    }
    setText(tagEl, 'tag', h.tag || '');
    tagEl.classList.toggle('rep', /REPLAY|REPETICI/i.test(h.tag || ''));
    setText(capEl, 'cap', h.caption || '');
    const b = h.banner;
    const bk = b ? b.id + '|' + b.text : '';
    if (hudKeys.ban !== bk) {
      hudKeys.ban = bk;
      if (b) { banEl.textContent = b.text; banEl.style.background = b.color || '#ffd400'; banEl.style.color = b.color2 || '#fff'; banEl.classList.remove('on'); void banEl.offsetWidth; banEl.classList.add('on'); }
      else banEl.classList.remove('on');
    }
    const e = entities.find(x => x.id === w.id);
    setText(whoEl, 'who', pickerText(e, { auto: w.auto, missing: w.missing, free: rig.mode === 'free', L }));
    const tc = rig.mode === 'free' ? '#ffd400' : (e && e.color) || '#ffd400';
    if (hudKeys.tc !== tc) { hudKeys.tc = tc; whoEl.style.setProperty('--tc', tc); }
    q('[data-a="auto"]').classList.toggle('on', lock == null && rig.mode !== 'free');
    q('[data-a="free"]').classList.toggle('on', rig.mode === 'free');
  }

  // ---- one frame ----
  let lastWatch = null, curOverlay = '';
  function step(dt) {
    const t0 = now();
    clock += dt;
    // The shared timeline: the 2D viewer advances (without drawing).
    try { opts.view.tick(dt); } catch (e) { console.warn('arena 3D: view tick', e); }
    const entities = adapter.entities(ctx) || [];
    const ids = entities.map(e => e.id);
    const w = resolveWatch(lock, ids, adapter.autoEntity(ctx));
    ctx.watch = { id: w.id, mode: rig.mode };
    try { adapter.update(ctx, { dt, clock, now: t0 }); } catch (e) { console.warn('arena 3D: update', e); }
    let view, overlay = '';
    if (rig.mode === 'free' || w.id == null) {
      const f = adapter.focus(ctx) || [0, 0, 0];
      const was = rig.mode; rig.mode = 'free';
      view = rigFreeStep(rig, f, dt);
      rig.mode = was;
    } else {
      const cam = adapter.cameraFor(ctx, w.id);
      if (cam) {
        // A jump far away (an automatic switch, a new play) is a cut, not a flight.
        let cut = !!cam.cut;
        if (w.id !== lastWatch) {
          const far = rig.pos && Math.hypot(cam.pos[0] - rig.pos[0], cam.pos[2] - rig.pos[2]) > 9;
          rigWatch(rig, w.id, { cut: cut || (far && w.auto) || !rig.pos });
          if (lastWatch != null && (cut || (far && w.auto))) { cutEl.classList.remove('go'); void cutEl.offsetWidth; cutEl.classList.add('go'); }
          lastWatch = w.id;
        }
        view = rigFollow(rig, { ...cam, cut }, dt, t0);
        overlay = cam.overlay || '';
      } else view = rigFreeStep(rig, adapter.focus(ctx) || [0, 0, 0], dt);
    }
    if (overlay !== curOverlay) {
      curOverlay = overlay;
      maskEl.innerHTML = overlay === 'helmet' ? HELMET_SVG : overlay === 'visor' ? VISOR_SVG : '';
      maskEl.classList.toggle('on', !!overlay);
    }
    camera.position.set(view.pos[0], view.pos[1], view.pos[2]);
    camera.lookAt(view.look[0], view.look[1], view.look[2]);
    if (Math.abs(camera.fov - view.fov) > .01) { camera.fov = view.fov; camera.updateProjectionMatrix(); }
    if ((perf.hudN = (perf.hudN || 0) + 1) % 3 === 0 || dt === 0) paintHud(entities, w);
    renderer.render(scene, camera);
    const ms = now() - t0;
    perf.ms = perf.n ? perf.ms * .95 + ms * .05 : ms; perf.n++;
    return ms;
  }

  function frame(t) {
    rafId = 0;
    if (stopped || !vis.page || !vis.screen) return;
    rafId = requestAnimationFrame(frame);
    const gap = t - last;
    const dt = Math.min(.25, Math.max(0, gap / 1000)); last = t;
    if (perf.n > 30) perf.gap = perf.gap ? perf.gap * .95 + gap * .05 : gap;
    step(dt);
    // Sustained slow frames: fewer pixels (never below 0.75).
    if (perf.n > 120 && perf.gap > 26) { perf.slow++; if (perf.slow > 90 && pr > .75) { pr = Math.max(.75, pr - .25); perf.slow = 0; perf.gap = 0; perf.n = 31; resize(); } } else perf.slow = 0;
  }
  const wake = () => { if (!rafId && !stopped && vis.page && vis.screen) { last = now(); rafId = requestAnimationFrame(frame); } };
  const setShown = (k, on) => {
    vis[k] = on;
    // The 2D viewer polls while we are seen (its own canvas may be hidden under us).
    try { opts.view.setShown('driver', vis.page && vis.screen); } catch (e) { /* res */ }
    if (on) wake(); else if (rafId) { cancelAnimationFrame(rafId); rafId = 0; }
  };

  // ---- input ----
  let drag = null;
  const onDown = e => {
    if (e.button != null && e.button !== 0) return;
    drag = { x: e.clientX, y: e.clientY, id: e.pointerId, moved: 0 };
    rig.dragging = true; cv.classList.add('drag');
    try { cv.setPointerCapture(e.pointerId); } catch (x) { /* res */ }
    cv.focus({ preventScroll: true });
  };
  const onMove = e => {
    if (!drag || e.pointerId !== drag.id) return;
    const dx = e.clientX - drag.x, dy = e.clientY - drag.y; drag.x = e.clientX; drag.y = e.clientY; drag.moved += Math.abs(dx) + Math.abs(dy);
    rigLook(rig, dx, dy, { now: now(), sens: e.pointerType === 'touch' ? .007 : .0045 });
  };
  const onUp = e => { if (!drag || (e && e.pointerId !== drag.id)) return; drag = null; rig.dragging = false; rig.lastInput = now(); cv.classList.remove('drag'); };
  const onWheel = e => { if (rig.mode !== 'free') return; e.preventDefault(); rigZoom(rig, Math.exp(e.deltaY * .0012)); };
  cv.addEventListener('pointerdown', onDown); cv.addEventListener('pointermove', onMove);
  cv.addEventListener('pointerup', onUp); cv.addEventListener('pointercancel', onUp);
  cv.addEventListener('wheel', onWheel, { passive: false });
  cv.addEventListener('dblclick', () => api.toggleFree());

  const idsNow = () => (adapter.entities(ctx) || []).map(e => e.id);
  const api = {
    el: root, ctx, rig,
    next(step = 1) { const ids = idsNow(); const cur = lock != null && ids.includes(lock) ? lock : ctx.watch.id; lock = cycleEntity(ids, cur, step); rig.mode = 'first'; rigRecenter(rig); },
    setLock(id) { lock = id; if (id != null) rig.mode = 'first'; rigRecenter(rig); },
    auto() { lock = null; rig.mode = 'first'; rigRecenter(rig); },
    toggleFree() {
      if (rig.mode === 'free') { rig.mode = 'first'; rig.pos = null; lastWatch = null; return; }
      rig.mode = 'free';
      rig.free.focus = null;
      // Start the orbit behind where we were looking.
      const v = rigView({ ...rig, mode: 'first' });
      if (rig.pos) { rig.free.yaw = wrapAngle(v.yaw + Math.PI); }
    },
    recenter() { rigRecenter(rig); },
    stats() { const info = renderer.info.render; return { ms: +perf.ms.toFixed(2), interval: +perf.gap.toFixed(2), fps: perf.gap ? +(1000 / perf.gap).toFixed(1) : 0, calls: info.calls, triangles: info.triangles, pixelRatio: pr, width: renderer.domElement.width, height: renderer.domElement.height, watching: ctx.watch.id, mode: rig.mode }; },
    // n frames rendered back to back, each waited for on the GPU (readPixels):
    // the real cost of a frame (CPU + GPU) in ms, without the vsync wait.
    bench(n = 60) {
      const gl = renderer.getContext(), px = new Uint8Array(4), out = [];
      for (let i = 0; i < n; i++) { const t0 = now(); step(1 / 60); gl.readPixels(0, 0, 1, 1, gl.RGBA, gl.UNSIGNED_BYTE, px); out.push(now() - t0); }
      out.sort((a, b) => a - b);
      return { avg: +(out.reduce((a, b) => a + b, 0) / n).toFixed(2), p50: +out[n >> 1].toFixed(2), p95: +out[Math.floor(n * .95)].toFixed(2), calls: renderer.info.render.calls, triangles: renderer.info.render.triangles };
    },
    destroy(reason) {
      if (stopped) return;
      stopped = true;
      if (rafId) cancelAnimationFrame(rafId); rafId = 0;
      window.removeEventListener('keydown', onKey, true);
      document.removeEventListener('visibilitychange', onVis);
      if (ro) ro.disconnect(); if (io) io.disconnect();
      clearTimeout(helpT);
      try { if (adapter.dispose) adapter.dispose(ctx); } catch (e) { /* res */ }
      scene.traverse(o => { if (o.isMesh || o.isInstancedMesh || o.isPoints || o.isLine) { if (o.geometry) o.geometry.dispose(); const m = o.material; (Array.isArray(m) ? m : [m]).forEach(x => { if (x) { if (x.map) x.map.dispose(); x.dispose(); } }); } });
      kit.dispose();
      renderer.dispose();
      root.remove();
      // The 2D viewer takes its loop back.
      try { opts.view.setHeadless(false); } catch (e) { /* res */ }
      if (opts.onExit) opts.onExit(reason || 'exit');
    },
  };
  root.querySelector('.a3-bar').addEventListener('click', e => {
    const b = e.target.closest('button'); if (!b) return;
    const a = b.dataset.a;
    if (a === 'prev') api.next(-1); else if (a === 'next') api.next(1); else if (a === 'auto') api.auto(); else if (a === 'free') api.toggleFree(); else if (a === 'exit') api.destroy('button');
  });
  const editable = t => t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName));
  const onKey = e => {
    if (stopped || e.ctrlKey || e.metaKey || e.altKey || editable(e.target) || !root.isConnected || !root.getClientRects().length) return;
    const k = e.key;
    let used = true;
    if (k === 'Tab') api.next(e.shiftKey ? -1 : 1);
    else if (k === 'e' || k === 'E' || k === ']') api.next(1);
    else if (k === 'q' || k === 'Q' || k === '[') api.next(-1);
    else if (k === 'a' || k === 'A' || k === '0') api.auto();
    else if (k === 'f' || k === 'F' || k === 'c' || k === 'C') api.toggleFree();
    else if (k === 'r' || k === 'R') api.recenter();
    else if (k === 'Escape') { api.destroy('escape'); e.stopImmediatePropagation(); }
    else used = false;
    if (used) e.preventDefault();
  };
  window.addEventListener('keydown', onKey, true);
  const onVis = () => setShown('page', !document.hidden);
  document.addEventListener('visibilitychange', onVis);
  const ro = typeof ResizeObserver === 'function' ? new ResizeObserver(() => resize()) : null;
  if (ro) { ro.observe(cover); ro.observe(host); }
  const io = typeof IntersectionObserver === 'function' ? new IntersectionObserver(es => { for (const e of es) setShown('screen', e.isIntersecting); }) : null;
  if (io) io.observe(root);
  cv.addEventListener('webglcontextlost', e => { e.preventDefault(); msgEl.hidden = false; msgEl.textContent = L.lost; setTimeout(() => api.destroy('contextlost'), 1500); });
  const helpT = setTimeout(() => helpEl.classList.add('gone'), 7000);

  root.a3 = api; // for the console and tests: document.querySelector('.a3-root').a3.stats()
  // The 2D viewer stops its own loop: from now on its clock is ours.
  opts.view.setHeadless(true);
  wake();
  return api;
}

// ---------------------------------------------------------------------------
// Adapters: one line per game. Add yours here.
// ---------------------------------------------------------------------------
register3D('nfl', () => import('./arena-3d-nfl.js'));
register3D('cs2', () => import('./arena-3d-cs2.js'));
register3D('lol', () => import('./arena-3d-lol.js'));
