// Counter-Strike 2 en primera persona 3D (arena-3d-cs2.js): el mapa del
// visor 2D convertit en parets, terra, caixes i arcs; qui es pot mirar i a qui
// segueix el mode automàtic; i l'ull a l'alçada bona (dret, ajupit, mort).
const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const fs = require('node:fs');
const { pathToFileURL } = require('node:url');

const load = f => import(pathToFileURL(path.join(__dirname, f)).href);
const near = (a, b, eps = 1e-6, msg) => assert.ok(Math.abs(a - b) <= eps, `${msg || ''} ${a} vs ${b}`);
const nearV = (a, b, eps = 1e-6, msg) => a.forEach((v, i) => near(v, b[i], eps, msg));
const demo = () => JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'arena_demo', 'cs2.json'), 'utf8'));

test('CS2 3D: registered with the shared view and a complete adapter', async () => {
  const A = await load('arena-3d.js');
  assert.ok(A.has3D('cs2'));
  const a = await A.load3D('cs2');
  assert.doesNotThrow(() => A.checkAdapter(a, 'cs2'));
  for (const k of ['hud', 'freeStart', 'dispose']) assert.equal(typeof a[k], 'function', k);
});

test('CS2 3D: map coordinates (Y up, map centre at the origin, the radar seen from above, not mirrored)', async () => {
  const C = await load('arena-3d-cs2.js'), A = await load('arena-3d.js');
  nearV(C.mapToWorld(50, 40, 0), [0, 0, 0]);
  nearV(C.mapToWorld(0, 0, 1.62), [-50, 1.62, -40]);
  for (const p of [[12, 70, 0], [90, 42, 3], [41, 66, 1.1]]) { const m = C.worldToMap(C.mapToWorld(...p)); nearV([m.x, m.y, m.h], p, 1e-9); }
  // A heading on the map and the camera's yaw agree (map y grows down = world +Z).
  for (const a of [0, Math.PI / 2, -2.2, 2.9, .4]) {
    const w0 = C.mapToWorld(30, 30), w1 = C.mapToWorld(30 + Math.cos(a), 30 + Math.sin(a));
    nearV(A.forwardOf(C.mapYaw(a), 0), [w1[0] - w0[0], 0, w1[2] - w0[2]], 1e-9, 'heading ' + a);
  }
});

test('CS2 3D: the 2D map becomes walls, floor, crates, arches and site letters', async () => {
  const C = await load('arena-3d-cs2.js'), V = await load('arena-cs2-view.js');
  const meta = demo().meta, G = V.buildGrid(meta, .5);
  const mm = C.buildMapMesh(G);
  const n = mm.pos.length / 3;
  assert.ok(n > 1000 && mm.pos.length % 9 === 0, 'triangles');
  assert.equal(mm.nor.length, mm.pos.length); assert.equal(mm.col.length, mm.pos.length); assert.equal(mm.uv.length / 2, n);
  assert.ok(mm.pos.length / 9 < 60000, 'small enough for integrated GPUs: ' + mm.pos.length / 9);
  // Inside the map (plus the padding ring), between the floor and the tallest wall.
  for (let i = 0; i < n; i++) {
    const x = mm.pos[i * 3], y = mm.pos[i * 3 + 1], z = mm.pos[i * 3 + 2];
    assert.ok(x >= -50.5 && x <= 50.5 && z >= -40.5 && z <= 40.5 && y >= 0 && y <= 9, `vertex ${i} ${x},${y},${z}`);
  }
  // Every triangle's winding matches its normal (faces are seen from the right side).
  for (let t = 0; t < n / 3; t += 7) {
    const P = k => [mm.pos[(t * 3 + k) * 3], mm.pos[(t * 3 + k) * 3 + 1], mm.pos[(t * 3 + k) * 3 + 2]];
    const a = P(0), b = P(1), c = P(2), u = b.map((v, i) => v - a[i]), w = c.map((v, i) => v - a[i]);
    const cr = [u[1] * w[2] - u[2] * w[1], u[2] * w[0] - u[0] * w[2], u[0] * w[1] - u[1] * w[0]];
    const nn = [mm.nor[t * 9], mm.nor[t * 9 + 1], mm.nor[t * 9 + 2]];
    if (Math.hypot(...cr) < 1e-9) continue;
    assert.ok(cr[0] * nn[0] + cr[1] * nn[1] + cr[2] * nn[2] > 0, 'winding of triangle ' + t);
  }
  // Walls facing the floor: the floor is in front of them, the wall behind, at the grid's height.
  let front = 0;
  for (const f of mm.faces) {
    const mx = (f.ax + f.bx) / 2, my = (f.ay + f.by) / 2;
    if (V.cellAt(G, mx + f.nx * .3, my + f.ny * .3) !== 0) front++;
    assert.ok(f.h >= 3.6 - 1e-4 && f.h <= 8.4 + 1e-4, 'wall height ' + f.h);
  }
  assert.ok(front / mm.faces.length > .97, 'walls face the floor: ' + front + '/' + mm.faces.length);
  // Where players walk there is floor; in the middle of nowhere, a wall.
  for (const z of meta.zones) assert.equal(C.heightAt(G, z.x, z.y), 0, z.id);
  for (const pt of Object.values(meta.plant)) assert.equal(C.heightAt(G, pt[0], pt[1]), 0);
  assert.ok(C.heightAt(G, 75, 20) >= 3.6, 'between B apartments and T spawn: a wall');
  // Crates: the props' footprints at crate height.
  const crates = C.cratesOf(meta);
  for (const p of meta.props) {
    const inside = crates.filter(c => Math.abs(c.x - p.x) < p.w / 2 && Math.abs(c.y - p.y) < p.h / 2);
    assert.ok(inside.length >= 1);
    near(inside.reduce((a, c) => a + (c.w + .05) * (c.d + .05), 0), p.w * p.h, 1e-6, 'footprint');
    assert.ok(inside.every(c => c.h === V.CRATE_H));
  }
  // Arches: on the floor, both pillars inside walls.
  const arches = C.archesOf(meta, G);
  assert.ok(arches.length >= 8, 'arches ' + arches.length);
  for (const a of arches) {
    const lx = -Math.sin(a.ang), ly = Math.cos(a.ang);
    assert.notEqual(V.cellAt(G, a.x, a.y), 0);
    assert.equal(V.cellAt(G, a.x + lx * 3.05, a.y + ly * 3.05), 0);
    assert.equal(V.cellAt(G, a.x - lx * 3.05, a.y - ly * 3.05), 0);
  }
  assert.ok(arches.some(a => a.door), 'some are doors');
  // Site letters: A and B, on a wall near their plant spot, facing it.
  const signs = C.siteSignsOf(meta, G);
  assert.deepEqual(signs.map(s => s.site).sort(), ['A', 'B']);
  for (const s of signs) {
    const pt = meta.plant[s.site];
    assert.ok(Math.hypot(s.x - pt[0], s.y - pt[1]) < 30);
    assert.equal(V.cellAt(G, s.x - s.nx * .2, s.y - s.ny * .2), 0, 'a wall behind the letter');
    assert.ok(s.nx * (pt[0] - s.x) + s.ny * (pt[1] - s.y) > 0, 'facing the site');
  }
  // Nobody is ever inside a wall.
  const u = C.unstick(G, 40, 73.4);
  assert.ok(V.cellAt(G, 40, 73.4) === 0 && V.cellAt(G, u.x, u.y) !== 0 && Math.hypot(u.x - 40, u.y - 73.4) <= 1.5, 'pushed out of the wall');
  assert.deepEqual(C.unstick(G, 40, 65), { x: 40, y: 65 });
});

function rounds() {
  return load('arena-cs2-view.js').then(V => {
    const m = demo();
    return { V, meta: m.meta, list: V.groupRounds(m.events).map(g => V.buildRound(g, m.meta)) };
  });
}

test('CS2 3D: the ten players to watch and who auto follows', async () => {
  const C = await load('arena-3d-cs2.js');
  const { V, meta, list } = await rounds();
  const R = list[0];
  const r0 = C.rosterOf(R, meta, 0);
  assert.equal(r0.length, 10);
  assert.equal(new Set(r0.map(e => e.id)).size, 10);
  assert.deepEqual(r0.map(e => e.id), meta.teams.flatMap(t => t.players.map(p => p.id)), 'in the match order');
  for (const e of r0) {
    const q = R.players[e.id];
    assert.equal(e.name, q.handle); assert.equal(e.team, q.team);
    assert.equal(e.color, C.SIDE_COLOR[q.side], 'CT blue, T orange');
    assert.match(e.role, /^(CT|T)( · .+)?$/);
  }
  // The fallen leave the list a moment after dying (their fall can still be watched).
  const k = R.kills[0];
  assert.ok(C.rosterOf(R, meta, k.t + .5).some(e => e.id === k.victim), 'just killed: still there');
  assert.ok(!C.rosterOf(R, meta, k.t + 3).some(e => e.id === k.victim), 'later: gone');
  // Auto: the kill cam's killer; the 2D director's POV; the bomb carrier; the defuser.
  assert.equal(C.autoPick(R, 0, 0, { replay: { k } }), k.killer);
  const want = V.directorWant(R, k._p - .3, k.t - .5);
  assert.equal(want.mode, 'pov');
  assert.equal(C.autoPick(R, k._p - .3, k.t - .5, { director: want }), want.id);
  assert.equal(want.id, k.killer, 'the killer about to get the kill');
  assert.equal(C.autoPick(R, 0, 0, { director: { mode: 'map' } }), R.bomb, 'the bomb carrier at the start');
  assert.equal(C.autoPick(R, 0, 0, { director: null, prev: 'a0' }), R.bomb, 'the carrier beats the last one followed');
  const D = list.find(r => r.defuse);
  assert.ok(D, 'the demo has a defuse');
  const gd = (D.defuse.start + D.defuse.t) / 2;
  assert.equal(C.autoPick(D, V.playTimeAt(D.tl, gd), gd, { director: { mode: 'map' } }), D.defuse.id);
  const P = list.find(r => r.plant);
  const gp = (P.plant.start + P.plant.t) / 2;
  assert.equal(C.autoPick(P, V.playTimeAt(P.tl, gp), gp, {}), P.plant.id);
  // Planted, nobody special: a CT near the bomb.
  const gq = P.plant.t + 1, alive = Object.values(P.players).filter(q => gq < q.deathT && q.side === 'CT');
  if (alive.length) assert.equal(P.players[C.autoPick(P, V.playTimeAt(P.tl, gq), gq, {})].side, 'CT');
  // timesOf: the replay runs on its own clock.
  const v = { R, p: 3, phase: 'play' };
  near(C.timesOf(v).gt, V.gameTimeAt(R.tl, 3), 1e-9);
  const rp = { R, p: R.tl.total, phase: 'replay', replay: { k, g: k.t - .4 } };
  near(C.timesOf(rp).gt, k.t - .4, 1e-9); assert.equal(C.timesOf(rp).rp, rp.replay);
  assert.equal(C.timesOf({ R: null }), null);
});

test('CS2 3D: the eye at standing, crouched and fallen height, never in a wall, looking where the POV looks', async () => {
  const C = await load('arena-3d-cs2.js');
  const { V, meta, list } = await rounds();
  const G = V.buildGrid(meta, .5);
  const R = list[0], id = Object.keys(R.players)[0];
  const e = C.eyeOf(R, id, 1, G);
  near(e.h, V.EYE, 1e-9, 'standing: the 2D POV eye height');
  assert.ok(!e.dead && e.crouch === 0);
  near(Math.cos(e.yaw - C.mapYaw(V.povAim(R, id, 1))), 1, 1e-9, 'yaw = the POV aim');
  // Planting: crouched, looking down at the bomb.
  const P = list.find(r => r.plant);
  const ep = C.eyeOf(P, P.plant.id, (P.plant.start + P.plant.t) / 2, G);
  assert.ok(ep.h < 1.3 && ep.h > .9, 'crouched eye ' + ep.h);
  assert.ok(ep.pitch < -.4, 'looking down');
  // Killed: down on the floor once the fall is over.
  const k = R.kills[0];
  const ed = C.eyeOf(R, k.victim, k.t + 1, G);
  assert.ok(ed.dead && ed.h < .5 && ed.deathK === 1, 'fallen eye ' + ed.h);
  const half = C.eyeOf(R, k.victim, k.t + .2, G);
  assert.ok(half.h < V.EYE && half.h > ed.h, 'falling');
  // Every eye of every round is on the floor.
  for (const r of list.slice(0, 6)) for (const q of Object.values(r.players)) for (const t of [0, 10, 25, 40]) {
    const ee = C.eyeOf(r, q.id, t, G);
    assert.notEqual(V.cellAt(G, ee.x, ee.y), 0, `round ${r.num} ${q.id} t=${t}`);
  }
  // The killer pitches at the victim (a body at about chest height, not the sky or the floor).
  const ek = C.eyeOf(R, k.killer, k.t - .1, G);
  assert.ok(Math.abs(ek.pitch) < .4, 'duel pitch ' + ek.pitch);
  assert.equal(C.eyeOf(R, 'nobody', 1, G), null);
  // Walk cycles follow the distance walked (so they survive seeks).
  const tr = R.players[id].track;
  assert.equal(C.trackDist(tr, -1), 0);
  assert.ok(C.trackDist(tr, 30) >= C.trackDist(tr, 10));
});
