// Proves de la simulació del CS2 en directe (arena-cs2-live-sim.js).
// Les rondes surten de internal/web/testdata/cs2-live-fixture.json, que
// escriu el motor (CS2_LIVE_FIXTURE=… go test ./internal/arena/cs2 -run LiveFixture).
const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const fs = require('node:fs');
const { pathToFileURL } = require('node:url');

const load = () => import(pathToFileURL(path.join(__dirname, 'arena-cs2-live-sim.js')).href);
const fx = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'testdata', 'cs2-live-fixture.json'), 'utf8'));

test('damage: armor, helmet and headshots', async () => {
  const S = await load(), ak = S.WEAPONS.ak47;
  assert.equal(S.hitDamage(ak, 'head', 0, false).hp, 144);
  assert.ok(S.hitDamage(ak, 'head', 100, true).hp >= 100, 'an AK head shot kills through a helmet');
  assert.equal(S.hitDamage(ak, 'body', 100, true).hp, 28);
  assert.equal(S.hitDamage(ak, 'body', 0, false).hp, 36);
  assert.ok(S.hitDamage(S.WEAPONS.awp, 'body', 100, true).hp >= 100, 'the AWP kills with one body shot');
  assert.ok(S.hitDamage(S.WEAPONS.glock, 'head', 100, true).hp < 100, 'a Glock head shot does not kill a helmet');
});

test('shot geometry: head, body, legs and misses', async () => {
  const S = await load();
  assert.equal(S.shotHit(0, 0, 1.62, 0, 0, 10, 0, false).zone, 'head');
  assert.equal(S.shotHit(0, 0, 1.62, 0, -0.06, 10, 0, false).zone, 'body');
  assert.equal(S.shotHit(0, 0, 1.62, 0, -0.12, 10, 0, false).zone, 'legs');
  assert.equal(S.shotHit(0, 0, 1.62, 0.1, 0, 10, 0, false), null, 'a metre to the side misses');
  assert.equal(S.shotHit(0, 0, 1.62, Math.PI, 0, 10, 0, false), null, 'behind');
  assert.equal(S.shotHit(0, 0, 1.62, 0, 0, 10, 0, true), null, 'over a crouching head');
  assert.equal(S.shotHit(0, 0, 1.62, 0, -0.04, 10, 0, true).zone, 'head');
});

test('bodies never walk into walls, and paths reach both sites', async () => {
  const S = await load(), V = await import(pathToFileURL(path.join(__dirname, 'arena-cs2-view.js')).href);
  const G = V.buildGrid(fx.meta), N = { G, walk: S.walkGrid(G) };
  let x = 90, y = 42, seed = 3;
  const rnd = () => ((seed = (seed * 1664525 + 1013904223) >>> 0) / 4294967296);
  let a = 0;
  for (let i = 0; i < 20000; i++) {
    if (i % 40 === 0) a = rnd() * Math.PI * 2;
    [x, y] = S.moveBody(G, x, y, Math.cos(a) * 0.08, Math.sin(a) * 0.08);
    assert.ok(!S.blocked(G, x, y), `inside a wall at ${x.toFixed(2)},${y.toFixed(2)}`);
  }
  for (const [from, to] of [[[90, 42], [41, 66]], [[90, 42], [24, 15]], [[11, 50], [41, 66]], [[11, 50], [24, 15]], [[56, 13], [70, 64]]]) {
    const p = S.findPath(N, from[0], from[1], to[0], to[1]);
    const end = p[p.length - 1];
    assert.ok(Math.hypot(end[0] - to[0], end[1] - to[1]) < 1, `path ${from} -> ${to} ends at ${end}`);
    let cx = from[0], cy = from[1];
    for (const q of p) { assert.ok(S.clearLine(N, cx, cy, q[0], q[1]) || Math.hypot(q[0] - cx, q[1] - cy) < 1, 'every leg is walkable'); cx = q[0]; cy = q[1]; }
  }
});

function playRound(S, meta, setup, opts) {
  const R = new S.LiveRound(meta, setup, { humanBot: true, ...opts });
  let n = 0;
  while (!R.over && n++ < 60 * 200) R.step(S.TICK);
  return R;
}

test('bots play whole rounds to a result the server can replay', async () => {
  const S = await load();
  const reasons = {};
  for (const [i, setup] of [...fx.setups, ...fx.full.filter((_, k) => k % 9 === 0)].entries()) {
    const R = playRound(S, fx.meta, setup, { seed: 7 + i });
    assert.ok(R.over, `round ${i} ended`);
    assert.ok(['T', 'CT'].includes(R.winner));
    reasons[R.reason] = (reasons[R.reason] || 0) + 1;
    const res = R.result(null);
    assert.equal(Object.keys(res.tracks).length, 10);
    const dead = new Set();
    for (const k of res.kills) {
      assert.ok(!dead.has(k.victim), 'nobody dies twice'); dead.add(k.victim);
      assert.ok(R.byId[k.victim] && R.byId[k.killer]);
      if (k.killer !== k.victim) assert.notEqual(R.byId[k.killer].team, R.byId[k.victim].team, 'no team kills');
    }
    if (R.reason === 'elim') assert.ok(R.alive(R.winner === 'T' ? 'CT' : 'T') === 0);
    if (res.plant) assert.ok(R.inSite(res.plant.x, res.plant.y), 'planted inside a site');
    if (R.reason === 'defuse') assert.ok(res.defuse && res.plant);
    if (R.reason === 'bomb') assert.ok(res.explode > res.plant.t);
    for (const p of R.players) assert.ok(p.track.length >= 2 || !p.alive, 'tracks are sampled');
    // the T bots got off their spawn
    const moved = R.players.filter(p => p.side === 'T' && Math.hypot(p.track[p.track.length - 1][1] - p.track[0][1], p.track[p.track.length - 1][2] - p.track[0][2]) > 8).length;
    assert.ok(moved >= 3, `round ${i}: only ${moved} T bots left spawn`);
  }
  assert.ok(Object.keys(reasons).length >= 2, `rounds end in different ways: ${JSON.stringify(reasons)}`);
});

test('the human: walking, shooting an enemy in front, planting with E', async () => {
  const S = await load();
  const setup = JSON.parse(JSON.stringify(fx.full[0]));
  const R = new S.LiveRound(fx.meta, setup, { seed: 5 });
  const me = R.human;
  assert.ok(me && me.human);
  // freeze time: nobody moves
  const x0 = me.x;
  R.step(S.TICK, { fwd: 1 });
  assert.equal(me.x, x0, 'frozen during the freeze time');
  while (R.t < 0) R.step(S.TICK, {});
  // put an enemy 8 m in front of us, in the open, and fire
  const foe = R.players.find(p => p.team !== me.team);
  for (const p of R.players) if (p !== me && p !== foe) { p.x = 5; p.y = 5; p.alive = false; }
  me.x = 41; me.y = 66; me.ang = 0; me.pitch = 0; me.slot = me.primary ? 'primary' : 'pistol';
  foe.x = 49; foe.y = 66; foe.ai.think = 1e9; foe.ang = 0; // looking away
  for (let i = 0; i < 120 && foe.alive; i++) R.step(S.TICK, { fire: true });
  assert.ok(!foe.alive, 'the enemy in front of the crosshair dies');
  assert.equal(R.kills[R.kills.length - 1].killer, me.id);
  assert.ok(R.over, 'last enemy dead: round over');

  // planting: a T human with the bomb inside site A
  const s2 = JSON.parse(JSON.stringify(fx.full[1]));
  const tHuman = s2.players.find(p => p.side === 'T' && !p.human);
  s2.players.forEach(p => { p.human = p.id === tHuman.id; });
  s2.human = tHuman.id; s2.bomb = tHuman.id;
  const R2 = new S.LiveRound(fx.meta, s2, { seed: 9 });
  while (R2.t < 0) R2.step(S.TICK, {});
  const t = R2.human;
  for (const p of R2.players) if (p.side === 'CT') { p.x = 11; p.y = 50; p.ai.think = 1e9; }
  t.x = 41; t.y = 66;
  for (let i = 0; i < 60 * 3.4; i++) R2.step(S.TICK, { use: true });
  assert.ok(R2.bomb.planted && R2.bomb.site === 'A', 'planted at A');
  assert.ok(R2.plant && R2.plant.id === t.id);
});

test('grenades: a smoke blocks sight, a flash blinds who looks at it', async () => {
  const S = await load();
  const R = new S.LiveRound(fx.meta, JSON.parse(JSON.stringify(fx.full[2])), { seed: 4, humanBot: true });
  while (R.t < 0) R.step(S.TICK);
  const a = R.players.find(p => p.side === 'T'), b = R.players.find(p => p.side === 'CT');
  a.x = 41; a.y = 66; a.ang = 0; b.x = 50; b.y = 66; b.ang = Math.PI;
  assert.ok(R.sees(a, b));
  a.nades = ['smoke', 'flash'];
  R.throwTo(a, 'smoke', 45.5, 66);
  for (let i = 0; i < 120; i++) R.utilStep(S.TICK), R.t += S.TICK;
  assert.ok(!R.sees(a, b) && !R.sees(b, a), 'nobody sees through the smoke');
  R.utils = [];
  R.throwTo(a, 'flash', 47, 66);
  for (let i = 0; i < 90; i++) R.utilStep(S.TICK), R.t += S.TICK;
  assert.ok(b.flashUntil > R.t + 1, 'the CT facing the flash is blind');
});

// Una ronda on la persona camina cap a un lloc i els seus bots tenen el rol «follow».
function escortRound(S, setupIdx, opts = {}) {
  const setup = JSON.parse(JSON.stringify(fx.full[setupIdx]));
  const human = setup.players.find(p => p.human);
  for (const p of setup.players) if (p.team === human.team && !p.human) p.bot = 'follow';
  const R = new S.LiveRound(fx.meta, setup, { seed: 11, ...opts });
  const me = R.human;
  // els rivals no es mouen ni disparen: només mirem com el segueixen
  for (const p of R.players) if (p.team !== me.team) { p.ai.think = 1e9; p.x = me.side === 'T' ? 11 : 90; p.y = me.side === 'T' ? 50 : 42; }
  return R;
}

test('escorts: "follow" bots stay with the human along his path, spaced out', async () => {
  const S = await load();
  const R = escortRound(S, 0, { noEnemies: true });
  const me = R.human;
  while (R.t < 0) R.step(S.TICK, {});
  const goal = me.side === 'T' ? [41, 66] : [56, 42];
  const path = S.findPath(R.N, me.x, me.y, goal[0], goal[1]);
  let worst = 0, sum = 0, n = 0, minGap = 99;
  for (let i = 0; i < 60 * 30 && path.length; i++) {
    const [wx, wy] = path[0];
    if (Math.hypot(wx - me.x, wy - me.y) < 0.6) { path.shift(); continue; }
    me.ang = Math.atan2(wy - me.y, wx - me.x);
    R.step(S.TICK, { fwd: 1 });
    if (R.t > 4) {
      const fs = R.players.filter(p => p.team === me.team && !p.human && p.alive);
      for (const f of fs) { const d = Math.hypot(f.x - me.x, f.y - me.y); worst = Math.max(worst, d); sum += d; n++; }
      for (let a = 0; a < fs.length; a++) for (let b = a + 1; b < fs.length; b++) minGap = Math.min(minGap, Math.hypot(fs[a].x - fs[b].x, fs[a].y - fs[b].y));
    }
  }
  assert.ok(!path.length, 'the human got there');
  // standing still for a while: they gather around without stacking
  for (let i = 0; i < 60 * 4; i++) R.step(S.TICK, {});
  const fs = R.players.filter(p => p.team === me.team && !p.human);
  for (const f of fs) assert.ok(Math.hypot(f.x - me.x, f.y - me.y) < 7, `${f.id} is ${Math.hypot(f.x - me.x, f.y - me.y).toFixed(1)} m away after the human stopped`);
  assert.ok(sum / n < 8, `average distance ${(sum / n).toFixed(1)} m`);
  assert.ok(worst < 16, `worst distance ${worst.toFixed(1)} m`);
  assert.ok(minGap > 0.4, 'followers do not stand inside each other');
  assert.ok(fs.every(f => R.doing(f) === 'covering you'), fs.map(f => R.doing(f)).join(','));
});

test('escorts trade the human\'s killer, then go back to the plan', async () => {
  const S = await load();
  const R = escortRound(S, 0);
  const me = R.human;
  while (R.t < 0) R.step(S.TICK, {});
  for (let i = 0; i < 60 * 3; i++) R.step(S.TICK, {});
  const foe = R.players.find(p => p.team !== me.team);
  for (const p of R.players) if (p.team !== me.team && p !== foe) p.alive = false;
  foe.x = me.x + 6; foe.y = me.y; foe.ai.think = 1e9;
  if (S.blocked(R.G, foe.x, foe.y)) { foe.x = me.x - 6; }
  R.die(me, foe, 'ak47', false);
  R.step(S.TICK, {}); for (let i = 0; i < 12; i++) R.step(S.TICK, {});
  const fs = R.players.filter(p => p.team === me.team && !p.human && p.alive);
  assert.ok(fs.some(f => f.ai.priority && f.ai.priority.id === foe.id), 'the killer is the escorts\' first target');
  for (let i = 0; i < 60 * 8 && foe.alive; i++) R.step(S.TICK, {});
  assert.ok(!foe.alive, 'the escorts traded the killer');
  assert.ok(R.kills.some(k => k.victim === foe.id && k.killer !== me.id));
  // ...and back to the plan (no human to follow any more)
  const s2 = escortRound(S, 0);
  while (s2.t < 0) s2.step(S.TICK, {});
  s2.die(s2.human, null, 'c4', false);
  for (let i = 0; i < 60 * 7; i++) s2.step(S.TICK, {});
  const f2 = s2.players.filter(p => p.team === s2.human.team && !p.human && p.alive);
  assert.ok(f2.length && f2.every(f => !f.ai.following && f.ai.unfollow), 'after the human dies the escorts play the plan');
  assert.ok(f2.every(f => R.doing(f) !== 'covering you'));
});

test('team order: "all" makes every bot follow, "plan" none', async () => {
  const S = await load();
  const R = new S.LiveRound(fx.meta, JSON.parse(JSON.stringify(fx.full[3])), { seed: 2, teamOrder: 'all' });
  const mates = R.players.filter(p => p.team === R.human.team && !p.human);
  assert.ok(mates.every(p => R.follows(p)));
  R.teamOrder = 'plan';
  assert.ok(mates.every(p => !R.follows(p)));
});
