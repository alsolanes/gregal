// La vista 3D del LoL (arena-3d-lol.js): el mapa al món, qui es pot mirar,
// a qui segueix la càmera sola i on es posa la càmera (darrere l'espatlla).
const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const fs = require('node:fs');
const { pathToFileURL } = require('node:url');

const load = f => import(pathToFileURL(path.join(__dirname, f)).href);
const near = (a, b, eps = 1e-6, msg) => assert.ok(Math.abs(a - b) <= eps, `${msg || ''} ${a} vs ${b}`);
const nearV = (a, b, eps = 1e-6, msg) => a.forEach((v, i) => near(v, b[i], eps, msg));

const meta = () => ({
  blue: 0,
  teams: [
    { name: 'Blue Team', abbr: 'BT', color: '#2f7df6', side: 'blue', players: ['top', 'jungle', 'mid', 'adc', 'support'].map((role, i) => ({ handle: 'B' + i, role, champ: 'Bc' + i })) },
    { name: 'Red Team', abbr: 'RT', color: '#e8414e', side: 'red', players: ['top', 'jungle', 'mid', 'adc', 'support'].map((role, i) => ({ handle: 'R' + i, role, champ: 'Rc' + i })) },
  ],
});
// A fake viewer: a map-mode state with everyone at given spots.
async function fakeView(spots = {}) {
  const V = await load('arena-lol-view.js');
  const st = V.createState(meta());
  st.mode = 'map';
  for (let t = 0; t < 2; t++) for (let r = 0; r < 5; r++) {
    const p = spots[`${t}:${r}`] || { x: t ? 80 : 20, y: t ? 20 : 80 };
    st.pos[t][r] = { x: p.x, y: p.y, alive: p.alive !== false };
  }
  return { state: st, anim: 10, clock: 300, hp: [[1, 1, 1, 1, 1], [1, 1, 1, 1, 1]], acts: [], dash: [[], []], air: [[], []], cur: null, shot: null, replay: null, camTw: null, killFeed: [] };
}

test('LoL 3D: the map maps to the world (Y up, centre at the origin, blue bottom-left, not mirrored)', async () => {
  const L = await load('arena-3d-lol.js'), A = await load('arena-3d.js');
  nearV(L.mapToWorld(50, 50), [0, 0, 0]);
  const blue = L.mapToWorld(5, 95), red = L.mapToWorld(95, 5);
  assert.ok(blue[0] < 0 && blue[2] > 0, 'the blue fountain is at -X, +Z (bottom-left seen from above)');
  nearV(red, [-blue[0], 0, -blue[2]], 1e-9, 'the red fountain mirrors it through the centre');
  nearV(L.mapToWorld(10, 20, 3), [-40 * L.K, 3, -30 * L.K]);
  for (const p of [[0, 0, 0], [33, 71, 2], [100, 100, 9]]) { const m = L.worldToMap(L.mapToWorld(...p)); nearV([m.x, m.y, m.h], p, 1e-9); }
  // Seen from above with the map's y going down the screen, x × y points into the ground.
  const o = L.mapToWorld(0, 0), ex = L.mapToWorld(1, 0).map((v, i) => v - o[i]), ey = L.mapToWorld(0, 1).map((v, i) => v - o[i]);
  const cross = [ex[1] * ey[2] - ex[2] * ey[1], ex[2] * ey[0] - ex[0] * ey[2], ex[0] * ey[1] - ex[1] * ey[0]];
  assert.ok(cross[1] < 0 && Math.abs(cross[0]) < 1e-9 && Math.abs(cross[2]) < 1e-9, 'not mirrored');
  // A heading on the map and the camera's yaw agree.
  for (const [dx, dy] of [[1, 0], [0, 1], [-.6, .8], [.3, -.95]]) {
    const n = Math.hypot(dx, dy), w = L.mapToWorld(50 + dx / n, 50 + dy / n).map(v => v / L.K);
    nearV(A.forwardOf(L.headingYaw(dx, dy), 0), w, 1e-9);
  }
});

test('LoL 3D: the jungle keeps off the lanes, the river, the bases and the pits', async () => {
  const L = await load('arena-3d-lol.js'), V = await load('arena-lol-view.js');
  const map = V.defaultMap(), spots = L.jungleSpots(map);
  assert.ok(spots.length > 400, 'a real jungle: ' + spots.length);
  for (const p of spots) {
    for (const k of ['top', 'mid', 'bot']) assert.ok(L.polyDist(p.x, p.y, map.lanes[k]) >= 5.4, `tree on the ${k} lane at ${p.x},${p.y}`);
    assert.ok(L.polyDist(p.x, p.y, map.river) >= 6, 'tree in the river');
    for (const n of map.nexus) assert.ok(Math.hypot(n.x - p.x, n.y - p.y) >= 23, 'tree in a base');
    for (const o of [map.dragon, map.baron]) assert.ok(Math.hypot(o.x - p.x, o.y - p.y) >= 9.5, 'tree in a pit');
  }
  assert.deepEqual(L.jungleSpots(map), spots, 'the same jungle every time');
  assert.ok(L.jungleSpots(map, { step: 3.1 }).length < spots.length * .75, 'fewer trees on low-power machines');
});

test('LoL 3D: ten champions to watch, blue side first, top to support', async () => {
  const L = await load('arena-3d-lol.js'), V = await load('arena-lol-view.js');
  const st = V.createState(meta());
  const list = L.rosterOf(st);
  assert.equal(list.length, 10);
  assert.equal(new Set(list.map(e => e.id)).size, 10);
  assert.deepEqual(list.slice(0, 5).map(e => e.team), [0, 0, 0, 0, 0]);
  assert.deepEqual(list.map(e => e.role).slice(0, 5), ['TOP', 'JNG', 'MID', 'BOT', 'SUP']);
  assert.equal(list[2].name, 'Bc2 (B2)');
  assert.equal(list[0].color, '#2f7df6'); assert.equal(list[5].color, '#e8414e');
  // Red on the blue side: red's champions come first.
  const m = meta(); m.blue = 1; m.teams[0].side = 'red'; m.teams[1].side = 'blue';
  assert.deepEqual(L.rosterOf(V.createState(m)).slice(0, 5).map(e => e.team), [1, 1, 1, 1, 1]);
  assert.deepEqual(L.parseId(list[7].id), { t: 1, r: 2 });
  assert.equal(L.parseId('nope'), null);
  // The adapter's entities() reads the same from the view.
  const ctx = { view: { state: st }, state: {} };
  assert.deepEqual(L.adapter.entities(ctx).map(e => e.id), list.map(e => e.id));
});

test('LoL 3D: the automatic camera follows the fight, the killer, the objective', async () => {
  const L = await load('arena-3d-lol.js');
  // A fight near the dragon pit: red mid (1:2) is right in it.
  const v = await fakeView({ '0:3': { x: 60, y: 76 }, '1:2': { x: 68, y: 70 }, '1:1': { x: 74, y: 66 }, '0:4': { x: 62, y: 78 } });
  const memo = {};
  v.shot = { x: 69, y: 71, team: 1, kind: 'fight', mon: null, struct: null };
  assert.equal(L.autoChampion(v, memo), '1:2', 'nearest to what the action cam films');
  // An objective: the team on it, even if an enemy stands nearer.
  v.shot = { x: 61, y: 77, team: 1, kind: 'objective', mon: { kind: 'drake' } };
  const m2 = {};
  assert.equal(L.autoChampion(v, m2), '1:2', 'the team taking the dragon');
  // A kill: the killer, at once.
  v.cur = { ev: { kind: 'kill', data: { killer: { side: 0, slot: 4 }, victim: { side: 1, slot: 2 }, x: 62, y: 78 } } };
  assert.equal(L.autoChampion(v, memo), '0:4');
  // Hysteresis: a new nearest champion does not steal the camera within 2 s…
  v.cur = null; v.shot = { x: 61, y: 77, team: 0, kind: 'fight' };
  v.anim = 10.5;
  const keep = { id: '1:1', since: 10 };
  assert.equal(L.autoChampion(v, keep), '1:1', 'kept for a moment');
  v.anim = 13;
  assert.equal(L.autoChampion(v, keep), '0:4', 'then the fight wins');
  // The dead are never followed.
  v.state.pos[0][4].alive = false;
  assert.notEqual(L.autoChampion(v, {}), '0:4');
  // Nothing happening: whoever it followed, else blue mid.
  v.shot = null;
  assert.equal(L.autoChampion(v, { id: '1:1', since: 0 }), '1:1');
  assert.equal(L.autoChampion(v, {}), '0:2');
  // A gank: the jungler.
  v.cur = { ev: { kind: 'gank', data: { side: 1, x: 40, y: 50 } } };
  assert.equal(L.autoChampion(v, {}), '1:1');
  // The draft: nobody (the free camera).
  v.state.mode = 'draft';
  assert.equal(L.autoChampion(v, {}), null);
});

test('LoL 3D: the camera sits over the shoulder, behind and above, looking ahead; or at eye level', async () => {
  const L = await load('arena-3d-lol.js'), A = await load('arena-3d.js');
  for (const yaw of [0, 1.2, -2.5, Math.PI]) {
    const at = [10, 0, -20], c = L.shoulderCam(at, yaw), f = A.forwardOf(yaw, 0);
    const back = [at[0] - c.pos[0], at[2] - c.pos[2]], along = back[0] * f[0] + back[1] * f[2];
    assert.ok(along > 5 && along < 7.5, 'behind the champion: ' + along);
    assert.ok(c.pos[1] > 3 && c.pos[1] < 4.5, 'above its head');
    const right = [Math.sin(yaw), Math.cos(yaw)], side = -(back[0] * right[0] + back[1] * right[1]);
    assert.ok(side > .5 && side < 1.6, 'a little to the right: ' + side);
    assert.ok(Math.cos(A.wrapAngle(c.yaw - yaw)) > .97, 'looking the way the champion faces');
    assert.ok(c.pitch < -.1 && c.pitch > -.6, 'down onto the champion and the ground ahead');
    // The champion itself is in view, near the centre.
    const d = A.forwardOf(c.yaw, c.pitch), to = [at[0] - c.pos[0], at[1] + 1.2 - c.pos[1], at[2] - c.pos[2]], n = Math.hypot(...to);
    assert.ok((d[0] * to[0] + d[1] * to[1] + d[2] * to[2]) / n > Math.cos(.5), 'the champion is in the picture');
    const e = L.shoulderCam(at, yaw, { eye: true, eyeH: 2.1 });
    near(e.pos[1], 2.1, 1e-9, 'eye height'); near(e.yaw, yaw, 1e-9);
    assert.ok(Math.hypot(e.pos[0] - at[0], e.pos[2] - at[2]) < .4, 'inside the head');
  }
  // cameraFor: from the spot update() stored, a cut on the first frame, then smooth.
  const ctx = { state: { spots: { '0:2': { w: [4, 0, 4], yaw: .5, eyeH: 2.1 } }, cut: true, dt: 1 / 60 } };
  const c1 = L.adapter.cameraFor(ctx, '0:2');
  assert.equal(c1.cut, true); assert.equal(c1.overlay, null);
  ctx.state.spots['0:2'].yaw = 2;
  const c2 = L.adapter.cameraFor(ctx, '0:2');
  assert.equal(c2.cut, false);
  assert.ok(Math.abs(A.wrapAngle(c2.yaw - .5)) < .3, 'the camera turns slowly, not with every step');
  // Dead (after the body is gone): above the fountain, the visor's vignette.
  ctx.state.spots['0:2'] = { dead: true, corpse: false, fountain: L.mapToWorld(5, 95), w: [0, 0, 0] };
  const c3 = L.adapter.cameraFor(ctx, '0:2');
  assert.equal(c3.overlay, 'visor'); assert.ok(c3.pos[1] > 8 && c3.cut);
  assert.equal(L.adapter.cameraFor(ctx, 'nobody'), null);
});

test('LoL 3D: champions stand where the 2D action cam draws them, live and in replays', async () => {
  const L = await load('arena-3d-lol.js'), V = await load('arena-lol-view.js');
  const v = await fakeView({ '0:1': { x: 40, y: 52 } });
  v.dash[0][1] = { dx: 2, dy: -1, at: 9.9 };
  v.camT = .8; v.camFocus = [42, 50];
  v.shot = { x: 42, y: 50, mon: { kind: 'drake', dieAt: 12 } };
  const fake = { ...v, state: v.state };
  const want = V.LolView.prototype.champXY.call(fake, 0, 1);
  const got = L.champSpot({ pos: v.state.pos, anim: v.anim, dash: v.dash, camT: .8, camFocus: [42, 50], shot: v.shot }, 0, 1);
  nearV(got, want, 1e-9);
  // A replay: positions come from the recorded frames, not the live state.
  const fr = (x, anim) => ({ anim, clock: 300, pos: [[0, 1, 2, 3, 4].map(() => ({ x, y: 50, alive: true })), [0, 1, 2, 3, 4].map(() => ({ x: 70, y: 30, alive: true }))], hp: [[.5, 1, 1, 1, 1], [1, 1, 1, 1, 1]], acts: [], dash: [[], []], air: [[], []] });
  v.replay = { frames: [fr(10, 1), fr(20, 1.05)], t: .5 * .05 / .6, rate: .6, len: .05 / .6, cam: { x: 50, y: 50, z: 2.6 }, shot: { x: 50, y: 50 } };
  const src = L.frameSource(v);
  assert.equal(src.replay, true);
  near(src.pos[0][0].x, 15, 1e-9, 'half way between the two frames');
  near(src.anim, 1.025, 1e-9);
  assert.equal(src.hp[0][0], .5);
  assert.ok(src.camT > .9);
  v.replay = null;
  assert.equal(L.frameSource(v).replay, false);
});

test('LoL 3D: a real match (the demo) headless: entities, automatic choice, camera framing synced', async () => {
  const L = await load('arena-3d-lol.js'), { LolView } = await load('arena-lol-view.js');
  const demo = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'arena_demo', 'lol.json'), 'utf8'));
  global.window = { devicePixelRatio: 1, addEventListener() {}, removeEventListener() {}, matchMedia: () => ({ matches: false }) };
  global.document = { hidden: false, addEventListener() {}, removeEventListener() {} };
  global.requestAnimationFrame = () => 1; global.cancelAnimationFrame = () => {};
  global.IntersectionObserver = class { observe() {} disconnect() {} };
  const canvas = { nodeType: 1, getContext: () => new Proxy({}, { get: () => () => ({ addColorStop() {} }), set: () => true }), getBoundingClientRect: () => ({ width: 1280, height: 720 }), width: 0, height: 0 };
  const v = new LolView(canvas, { source: { load: async after => ({ ...demo, status: 'final', events: demo.events.filter(e => e.n > after) }) } });
  await new Promise(r => setTimeout(r, 20));
  v.setHeadless(true);
  const ctx = { view: v, state: {} };
  let sawShot = 0, synced = 0, autos = new Set(), checked = 0;
  for (let i = 0; i < 60 * 260 && !v.isFinished(); i++) {
    v.tick(1 / 30);
    const ents = L.adapter.entities(ctx);
    if (v.state && v.state.mode === 'map') {
      assert.equal(ents.length, 10);
      const id = L.adapter.autoEntity(ctx);
      if (id) { autos.add(id); assert.ok(ents.some(e => e.id === id)); const p = L.parseId(id); assert.ok(L.isAliveAt(L.frameSource(v).pos, p.t, p.r), 'auto never follows the dead'); checked++; }
      if (v.shot && !v.replay && !v.scene) {
        sawShot++;
        // The viewer's framing keeps following its tween while headless.
        const src = L.frameSource(v);
        if (Math.abs(src.camT - v.camT) < 1e-9) synced++;
      }
    }
  }
  assert.ok(checked > 100, 'the map was played');
  assert.ok(autos.size >= 4, 'the camera moves round the champions: ' + [...autos].join(' '));
  assert.ok(sawShot > 30, 'the action cam filmed fights or objectives');
  assert.equal(synced, sawShot, 'camT in step with the action cam');
  v.destroy();
});
