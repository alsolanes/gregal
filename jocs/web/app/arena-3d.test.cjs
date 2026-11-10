// La vista 3D en primera persona (arena-3d.js, arena-3d-nfl.js): les peces
// pures (registre d'adaptadors, càmera, selecció de jugador, IK, coordenades
// del camp) i el mode headless del visor 2D que la fa anar al mateix rellotge.
const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const load = f => import(pathToFileURL(path.join(__dirname, f)).href);
const near = (a, b, eps = 1e-6, msg) => assert.ok(Math.abs(a - b) <= eps, `${msg || ''} ${a} vs ${b}`);
const nearV = (a, b, eps = 1e-6, msg) => a.forEach((v, i) => near(v, b[i], eps, msg));

const team = (id, color) => ({
  name: id, abbr: id.slice(0, 3).toUpperCase(), color, color2: '#ffffff',
  players: [['QB', 1], ['RB', 1], ['WR', 3], ['TE', 1], ['OL', 5], ['DL', 4], ['LB', 3], ['CB', 2], ['S', 2], ['K', 1]]
    .flatMap(([pos, n]) => Array.from({ length: n }, (_, i) => ({ id: `${id}-${pos}${i}`, name: `${pos}${i}`, pos, number: 10 + i }))),
});
const ACT = { passer: 'home-QB0', target: 'home-WR1', runner: 'home-RB0', tackler: 'away-LB1', defender: 'away-CB0', kicker: 'home-K0', returner: 'away-WR0' };

test('adapter registry: one-line registration, lazy loading, required methods', async () => {
  const A = await load('arena-3d.js');
  assert.ok(A.has3D('nfl'), 'the NFL adapter is registered by default');
  assert.ok(A.has3D('lol'), 'the LoL adapter is registered too');
  const full = { build() {}, update() {}, entities: () => [], autoEntity: () => null, cameraFor: () => null, focus: () => [0, 0, 0] };
  let loads = 0;
  A.register3D('demo', async () => { loads++; return { adapter: full }; });
  assert.ok(A.has3D('demo'));
  assert.equal(await A.load3D('demo'), full);
  assert.equal(await A.load3D('demo'), full);
  assert.equal(loads, 1, 'the loader runs once');
  A.register3D('obj', full);
  assert.equal(await A.load3D('obj'), full, 'a plain object works too');
  A.register3D('broken', { build() {}, update() {} });
  await assert.rejects(A.load3D('broken'), /missing entities, autoEntity, cameraFor, focus/);
  assert.equal(await A.load3D('nope'), null);
  assert.throws(() => A.register3D('x', null));
  assert.ok(A.games3D().includes('nfl'));
  for (const g of ['demo', 'obj', 'broken']) A.unregister3D(g);
  assert.ok(!A.has3D('demo'));
  // The real NFL module loads in node (three.js is only imported by mount3D).
  const nfl = await A.load3D('nfl');
  assert.equal(typeof nfl.cameraFor, 'function');
  assert.doesNotThrow(() => A.checkAdapter(nfl, 'nfl'));
});

test('look vectors: yaw 0 is +X, counter-clockwise from above, pitch up; inverse matches', async () => {
  const A = await load('arena-3d.js');
  nearV(A.forwardOf(0, 0), [1, 0, 0]);
  nearV(A.forwardOf(Math.PI / 2, 0), [0, 0, -1]);
  nearV(A.forwardOf(0, Math.PI / 2), [0, 1, 0]);
  for (const [y, p] of [[.3, .2], [-2.9, -.4], [3, 1.1]]) {
    const r = A.yawPitchOf(A.forwardOf(y, p));
    near(r.yaw, y, 1e-9); near(r.pitch, p, 1e-9);
  }
  near(A.wrapAngle(3 * Math.PI), Math.PI, 1e-9);
  near(A.wrapAngle(-Math.PI / 2 - 2 * Math.PI), -Math.PI / 2, 1e-9);
  // Smoothing takes the short way round and does not depend on the frame rate.
  const a = A.dampAngle(3, -3, 5, .1);
  assert.ok(a > 3 || a < -3, 'from 3 to -3 rad goes through ±π, not through 0');
  const one = A.damp(0, 10, 4, .2), two = A.damp(A.damp(0, 10, 4, .1), 10, 4, .1);
  near(one, two, 1e-9, 'two half steps = one step');
});

test('camera rig: follows the eye, drag turns the head, recentres, blends switches, orbits when free', async () => {
  const A = await load('arena-3d.js');
  const rig = A.createRig();
  // First frame snaps.
  let v = A.rigFollow(rig, { pos: [10, 2, 0], yaw: 0, pitch: 0 }, 1 / 60, 0);
  nearV(v.pos, [10, 2, 0]); nearV(v.look, [11, 2, 0]);
  // Then it follows tightly, and converges.
  for (let i = 0; i < 30; i++) v = A.rigFollow(rig, { pos: [12, 2, 0], yaw: .5, pitch: -.1 }, 1 / 60, i * 16);
  near(v.pos[0], 12, .01, 'position converges in half a second');
  near(v.yaw, .5, .02); near(v.pitch, -.1, .01);
  // Drag right looks right (yaw goes down), drag up looks up; clamped.
  A.rigLook(rig, 100, 0, { now: 1000 });
  v = A.rigView(rig);
  assert.ok(v.yaw < .5, 'dragging right turns right');
  A.rigLook(rig, 0, -100000, { now: 1000 });
  near(A.rigView(rig).pitch, A.PITCH_MAX, 1e-9, 'pitch is clamped');
  // Right after the drag the offset stays; long after, it goes back.
  rig.dragging = false;
  A.rigFollow(rig, { pos: [12, 2, 0], yaw: .5, pitch: 0 }, 1 / 60, 1500);
  assert.ok(Math.abs(rig.lookYaw) > .3, 'kept while the user is still looking around');
  for (let i = 0; i < 400; i++) A.rigFollow(rig, { pos: [12, 2, 0], yaw: .5, pitch: 0 }, 1 / 60, 5000 + i * 16);
  near(rig.lookYaw, 0, .01, 'recentred'); near(rig.lookPitch, 0, .02);
  // Switching entity: a slower blend (not a snap)…
  assert.ok(A.rigWatch(rig, 'b'));
  A.rigWatch(rig, 'c');
  assert.ok(rig.switchT > 0);
  const before = rig.pos.slice();
  A.rigFollow(rig, { pos: [30, 2, 0], yaw: 0, pitch: 0 }, 1 / 60, 9000);
  assert.ok(rig.pos[0] > before[0] && rig.pos[0] < 20, 'blends towards the new eye');
  // …or a cut.
  A.rigWatch(rig, 'd', { cut: true });
  v = A.rigFollow(rig, { pos: [50, 2, 5], yaw: 1, pitch: 0 }, 1 / 60, 9100);
  nearV(v.pos, [50, 2, 5]);
  assert.equal(A.rigWatch(rig, 'd'), false, 'same entity: nothing to do');
  // Free camera: orbits the focus at its distance and looks at it.
  rig.mode = 'free'; rig.free.dist = 20; rig.free.yaw = 0; rig.free.pitch = 0;
  v = A.rigFreeStep(rig, [5, 1, 5], 1 / 60);
  nearV(v.pos, [25, 1, 5]); nearV(v.look, [5, 1, 5]);
  near(Math.cos(v.yaw), -1, 1e-9, 'looking back at the focus');
  A.rigLook(rig, 0, 50);
  assert.ok(rig.free.pitch > 0, 'drag down raises the orbit');
  A.rigZoom(rig, 100); assert.equal(rig.free.dist, 140); A.rigZoom(rig, .0001); assert.equal(rig.free.dist, 4);
});

test('entity selection: cycling wraps, a lock wins while on the field, auto otherwise', async () => {
  const A = await load('arena-3d.js');
  const ids = ['qb', 'rb', 'wr'];
  assert.equal(A.cycleEntity(ids, 'qb', 1), 'rb');
  assert.equal(A.cycleEntity(ids, 'wr', 1), 'qb');
  assert.equal(A.cycleEntity(ids, 'qb', -1), 'wr');
  assert.equal(A.cycleEntity(ids, 'gone', 1), 'qb');
  assert.equal(A.cycleEntity(ids, 'gone', -1), 'wr');
  assert.equal(A.cycleEntity([], 'qb', 1), null);
  assert.deepEqual(A.resolveWatch('rb', ids, 'qb'), { id: 'rb', auto: false, missing: false });
  assert.deepEqual(A.resolveWatch(null, ids, 'qb'), { id: 'qb', auto: true, missing: false });
  assert.deepEqual(A.resolveWatch('kicker', ids, 'wr'), { id: 'wr', auto: true, missing: true }, 'a locked player off the field: auto, lock kept');
  assert.deepEqual(A.resolveWatch(null, ids, 'nobody'), { id: 'qb', auto: true, missing: false });
  assert.deepEqual(A.resolveWatch(null, [], null), { id: null, auto: true, missing: false });
  const L = A.TEXT3D.en;
  assert.match(A.pickerText({ name: '#11 Siroco', role: 'QB' }, { auto: true, L }), /Watching: #11 Siroco · QB \(auto\)/);
  assert.match(A.pickerText({ name: '#11 Siroco', role: 'QB' }, { auto: false, L }), /🔒/);
  assert.match(A.pickerText(null, { free: true, L }), /Free camera/);
});

test('two-bone IK: the hand lands on the target when it can reach, straight when it cannot', async () => {
  const A = await load('arena-3d.js');
  const L1 = .3, L2 = .31;
  for (const D of [.1, .3, .45, .6]) {
    const ik = A.twoBoneIK(L1, L2, D);
    // Shoulder at the origin, target on +X; elbow bent towards +Y.
    const E = [Math.cos(ik.shoulder) * L1, Math.sin(ik.shoulder) * L1];
    near(Math.hypot(D - E[0], -E[1]), L2, 1e-6, 'forearm length at D=' + D);
    near(ik.elbow, Math.PI - Math.acos((L1 * L1 + L2 * L2 - D * D) / (2 * L1 * L2)), 1e-9);
  }
  const far = A.twoBoneIK(L1, L2, 5);
  assert.ok(far.elbow < .05 && far.shoulder < .05, 'out of reach: the arm points straight at it');
  near(far.reach, L1 + L2, 1e-3);
});

test('NFL: field coordinates map to the world (Y up, field centre at the origin, not mirrored)', async () => {
  const N = await load('arena-3d-nfl.js'), A = await load('arena-3d.js');
  const C = N.FIELD_W / 2;
  nearV(N.fieldToWorld(60, C, 0), [0, 0, 0]);
  nearV(N.fieldToWorld(10, 0, 2), [-50, 2, C], 1e-9, 'left goal line, near sideline, 2 yards up');
  nearV(N.fieldToWorld(120, N.FIELD_W), [60, 0, -C]);
  for (const p of [[33, 12, 1.5], [0, 0, 0], [117, 40, 9]]) {
    const f = N.worldToField(N.fieldToWorld(...p));
    nearV([f.x, f.y, f.z], p, 1e-9);
  }
  // Right-handed both sides: field x × y = up.
  const o = N.fieldToWorld(0, 0, 0), ex = N.fieldToWorld(1, 0, 0).map((v, i) => v - o[i]), ey = N.fieldToWorld(0, 1, 0).map((v, i) => v - o[i]);
  const cross = [ex[1] * ey[2] - ex[2] * ey[1], ex[2] * ey[0] - ex[0] * ey[2], ex[0] * ey[1] - ex[1] * ey[0]];
  nearV(cross, [0, 1, 0], 1e-9, 'x × y is world up');
  // A heading on the field and the camera's yaw agree.
  for (const [hx, hy] of [[1, 0], [0, 1], [-.6, .8], [.3, -.95]]) {
    const n = Math.hypot(hx, hy), w = N.fieldToWorld(60 + hx / n, C + hy / n, 0);
    nearV(A.forwardOf(N.headingYaw(hx, hy), 0), w, 1e-9);
  }
});

test('NFL: roster order, automatic subject and the eye inside the helmet', async () => {
  const N = await load('arena-3d-nfl.js');
  const { buildPlay } = await load('football-view.js');
  const match = { teams: [team('home', '#1E5BD8'), team('away', '#C8102E')] };
  const pass = { n: 7, q: 1, off: 0, kind: 'pass', detail: 'medium', depth: 14, spot: 40, end: 60, yards: 20, complete: true, actors: ACT };
  const a = buildPlay(pass, match), m = a.marks;
  const roster = N.rosterOf(a, pass, match.teams);
  assert.equal(roster.length, 22);
  assert.equal(new Set(roster.map(e => e.id)).size, 22);
  assert.equal(roster[0].role, 'QB', 'the QB first');
  assert.ok(roster.slice(0, 11).every(e => e.team === 0) && roster.slice(11).every(e => e.team === 1), 'offence, then defence');
  assert.match(roster[0].name, /^#\d+ /);
  assert.equal(roster[0].color, '#1E5BD8');
  // Auto: QB until the ball is well on its way, then the receiver, who keeps it after the catch.
  assert.equal(N.autoSubjectId(a, pass, .1), m.qbId);
  assert.equal(N.autoSubjectId(a, pass, m.tThrow + .01), m.qbId);
  assert.equal(N.autoSubjectId(a, pass, m.tCatch - .01), m.targetId);
  assert.equal(N.autoSubjectId(a, pass, .95), 'home-WR1');
  const run = { n: 8, q: 1, off: 0, kind: 'run', dir: 'left', spot: 30, end: 36, yards: 6, actors: ACT };
  const ar = buildPlay(run, match);
  assert.equal(N.autoSubjectId(ar, run, .05), ar.marks.qbId, 'the QB before the handoff');
  assert.equal(N.autoSubjectId(ar, run, .6), 'home-RB0', 'the runner after it');
  const kick = { n: 9, q: 1, off: 1, kind: 'kickoff', spot: 35, end: 30, actors: { ...ACT, returner: 'away-WR0' } };
  const ak = buildPlay(kick, match);
  assert.equal(N.autoSubjectId(ak, kick, .3), ak.marks.returnerId || ak.carrierId);
  // The eye: helmet height when standing, looking downfield before the snap, at the ball while it flies.
  const qb = a.tracks.findIndex(tr => tr.p.id === m.qbId);
  const e0 = N.eyeOf(a, pass, qb, 0);
  assert.ok(e0.eye.z > 1.6 && e0.eye.z < 2.1, 'eye height ' + e0.eye.z);
  near(Math.cos(e0.yaw), a.dir, .05, 'facing the way the offence attacks');
  const wr = a.tracks.findIndex(tr => tr.p.id === m.targetId), tf = (m.tThrow + m.tCatch) / 2;
  const ew = N.eyeOf(a, pass, wr, tf), b = a.ball(tf);
  const want = Math.atan2(b.y - ew.eye.y, b.x - ew.eye.x);
  near(Math.cos(ew.yaw - want), 1, .002, 'the receiver watches the ball in the air');
  assert.ok(ew.pitch > 0, 'it is above the eye');
  // Falling in the tackle: the eye drops and looks at the turf.
  const c = (a.contacts || [])[0];
  if (c) {
    const ec = N.eyeOf(a, pass, c.carrier, Math.min(1, c.tc + .15));
    assert.ok(ec.eye.z < e0.eye.z - .4 && ec.pitch < -.3, 'down on the ground: ' + ec.eye.z.toFixed(2));
  }
});

// A fake browser: rAF by hand, visibility and observers controllable.
function fakeBrowser() {
  const frames = [], ios = [];
  global.window = { devicePixelRatio: 1, addEventListener() {}, removeEventListener() {}, matchMedia: () => ({ matches: false }) };
  global.document = { hidden: false, addEventListener() {}, removeEventListener() {} };
  global.requestAnimationFrame = f => { frames.push(f); return frames.length; };
  global.cancelAnimationFrame = () => {};
  global.IntersectionObserver = class { constructor(cb) { this.cb = cb; ios.push(this); } observe() {} disconnect() {} };
  let now = 0;
  const step = (ms = 16) => { now += ms; const list = frames.splice(0); for (const f of list) f(now); return list.length; };
  return { frames, ios, step };
}

test('headless 2D viewer: no own loop, tick() advances it, polling goes on while its canvas is hidden', async () => {
  const B = await load('arena-view-base.js');
  const br = fakeBrowser();
  let frames = 0, polls = 0, lastDt = 0;
  class V extends B.ArenaViewBase {
    constructor(cv) { super(cv, { fps: 60 }); this.start(); }
    onFrame(dt) { frames++; lastDt = dt; }
    poll() { polls++; if (!this.stopped) this.schedulePoll(15); }
  }
  const cv = { nodeType: 1, width: 0, height: 0, getContext: () => ({}), getBoundingClientRect: () => ({ width: 400, height: 300 }) };
  const v = new V(cv);
  br.step(); br.step();
  assert.equal(frames, 2);
  v.setHeadless(true);
  for (let i = 0; i < 5; i++) br.step();
  assert.equal(frames, 2, 'headless: its own loop stops');
  assert.equal(br.frames.length, 0, 'and asks for no frames');
  v.tick(.05); v.tick(1);
  assert.equal(frames, 4, 'the 3D view drives it');
  assert.equal(lastDt, .25, 'one tick is capped like a frame');
  v.invalidate(); v.togglePause();
  assert.equal(br.frames.length, 0, 'invalidate does not restart its loop');
  // The 2D canvas covered or hidden: polling continues for the 3D view.
  br.ios[0].cb([{ isIntersecting: false }]);
  const p0 = polls;
  await new Promise(r => setTimeout(r, 70));
  assert.ok(polls > p0, 'still polling while headless');
  // The 3D view goes off screen: it says so, and polling stops.
  v.setShown('driver', false);
  const p1 = polls;
  await new Promise(r => setTimeout(r, 70));
  assert.equal(polls, p1, 'driver hidden: no polls');
  v.setShown('driver', true);
  await new Promise(r => setTimeout(r, 40));
  assert.ok(polls > p1, 'driver shown again: polling resumes');
  br.ios[0].cb([{ isIntersecting: true }]);
  v.togglePause();
  v.setHeadless(false);
  br.step();
  assert.equal(frames, 5, 'back to 2D: its loop resumes');
  v.destroy();
});

test('headless football: the same timeline as the 2D view, and nothing drawn', async () => {
  const { FootballView } = await load('football-view.js');
  fakeBrowser();
  const match = { teams: [team('home', '#1E5BD8'), team('away', '#C8102E')] };
  const plays = [
    { kind: 'kickoff', spot: 35, end: 25, detail: 'touchback' }, { kind: 'run', dir: 'left', spot: 25, end: 31, yards: 6 },
    { kind: 'pass', detail: 'deep', depth: 30, spot: 31, end: 70, yards: 39, complete: true, hl: true }, { kind: 'sack', spot: 40, end: 33, yards: -7 },
  ].map((p, i) => ({ n: i + 1, q: 1, clock: 900 - i * 30, off: 0, down: 1, dist: 10, actors: ACT, text: 'p', score: [0, 0], ...p }));
  const canvas = () => ({ nodeType: 1, getContext: () => new Proxy({}, { get: () => () => ({ addColorStop() {} }), set: () => true }), getBoundingClientRect: () => ({ width: 960, height: 540 }), width: 0, height: 0 });
  const mk = () => new FootballView(canvas(), { source: { load: async after => ({ ...match, status: 'final', plays: plays.filter(p => p.n > after) }) } });
  const a = mk(), b = mk();
  await new Promise(r => setTimeout(r, 10));
  let draws = 0;
  a.draw = () => {}; // the 2D reference: its picture does not matter here, only its clock
  b.draw = () => { draws++; };
  b.setHeadless(true);
  a.setSpeed(2); b.setSpeed(2);
  const snap = v => [v.idx, v.phase, v.cur && v.cur.play.n, +v.t.toFixed(9), v.getProgress().elapsed.toFixed(6)].join('|');
  for (let i = 0; i < 900; i++) {
    a.onFrame(1 / 60, i * 16); b.tick(1 / 60);
    if (i === 300) { a.seek(2); b.seek(2); }
    if (i % 50 === 0) assert.equal(snap(b), snap(a), 'frame ' + i);
  }
  assert.equal(draws, 0, 'headless: no 2D drawing');
  a.destroy(); b.destroy();
});
