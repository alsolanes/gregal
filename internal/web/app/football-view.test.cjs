const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const team = (id, color) => ({
  name: id, abbr: id.slice(0, 3).toUpperCase(), color, color2: '#ffffff',
  players: [['QB', 1], ['RB', 1], ['WR', 3], ['TE', 1], ['OL', 5], ['DL', 4], ['LB', 3], ['CB', 2], ['S', 2], ['K', 1]]
    .flatMap(([pos, n]) => Array.from({ length: n }, (_, i) => ({ id: `${id}-${pos}${i}`, name: `${pos}${i}`, pos, number: 10 + i }))),
});

test('every kind of play builds an animation with sane positions', async () => {
  const { buildPlay, attackDir, fieldX } = await import(pathToFileURL(path.join(__dirname, 'football-view.js')).href);
  const match = { teams: [team('home', '#1E5BD8'), team('away', '#C8102E')] };
  const A = { passer: 'home-QB0', target: 'home-WR1', runner: 'home-RB0', tackler: 'away-LB1', defender: 'away-CB0', kicker: 'home-K0', returner: 'away-WR0' };
  const plays = [
    { kind: 'run', dir: 'left', spot: 30, end: 36, yards: 6 },
    { kind: 'run', dir: 'middle', spot: 95, end: 100, yards: 5, td: true },
    { kind: 'pass', detail: 'short', depth: 6, spot: 40, end: 49, yards: 9, complete: true },
    { kind: 'pass', detail: 'deep', depth: 35, spot: 20, end: 20, yards: 0 },
    { kind: 'pass', detail: 'medium', depth: 14, spot: 50, end: 64, int: true },
    { kind: 'pass', detail: 'screen', depth: -1, spot: 25, end: 31, yards: 6, complete: true },
    { kind: 'sack', spot: 45, end: 38, yards: -7 },
    { kind: 'scramble', spot: 45, end: 52, yards: 7 },
    { kind: 'punt', spot: 30, end: 75, yards: 45 },
    { kind: 'fg', spot: 70, end: 70, good: true },
    { kind: 'fg', spot: 60, end: 60, good: false },
    { kind: 'pat', spot: 98, end: 98, good: true },
    { kind: 'kickoff', spot: 35, end: 30, detail: 'touchback' },
    { kind: 'kickoff', spot: 35, end: 27 },
    { kind: 'kneel', spot: 60, end: 59, yards: -1 },
  ];
  plays.forEach((p, i) => {
    for (const off of [0, 1]) for (const q of [1, 2, 4, 5]) {
      const play = { n: i + 1, q, off, down: 1, dist: 10, actors: A, ...p };
      const anim = buildPlay(play, match);
      assert.ok(anim.dur > 1 && anim.dur < 7, p.kind + ' dur');
      assert.ok(anim.tracks.length >= 11, p.kind + ' players: ' + anim.tracks.length);
      for (const t of [0, .25, .5, .75, 1]) {
        for (const tr of anim.tracks) {
          const pos = tr.path(t);
          assert.ok(Number.isFinite(pos.x) && Number.isFinite(pos.y), `${p.kind} ${tr.p.id} at ${t}`);
          assert.ok(pos.x > -20 && pos.x < 140 && pos.y > -15 && pos.y < 70, `${p.kind} ${tr.p.id} off the field at ${t}: ${pos.x},${pos.y}`);
        }
        const b = anim.ball(t);
        if (b) assert.ok(Number.isFinite(b.x) && Number.isFinite(b.z) && b.z >= 0, `${p.kind} ball at ${t}`);
        // El mateix instant dona sempre el mateix lloc: si no, els ninots tremolen.
        for (const tr of anim.tracks) assert.deepEqual(tr.path(t), tr.path(t), `${p.kind} ${tr.p.id} jitters at ${t}`);
        if (b) assert.deepEqual(anim.ball(t), b, `${p.kind} ball jitters at ${t}`);
      }
    }
  });
  // Els equips canvien de camp cada quart; la iarda 0 de l'atac és la seva línia de gol.
  assert.equal(attackDir(0, 1), -attackDir(0, 2));
  assert.equal(attackDir(0, 1), -attackDir(1, 1));
  assert.equal(fieldX(0, 1), 10);
  assert.equal(fieldX(0, -1), 110);
});

test('the TV camera keeps the old sideline projection', async () => {
  const { tvView, project } = await import(pathToFileURL(path.join(__dirname, 'football-view.js')).href);
  const W = 1280, focal = 61.65 * W / 44, camH = 12, horizon = 200, camX = 47, FW = 53.33;
  const v = tvView(camX, focal, camH, W, horizon);
  for (const [x, y, z] of [[30, 10, 0], [60, 40, 2], [100, 0, 11]]) {
    const sc = focal / (30 + (FW - y)), p = project(v, x, y, z);
    assert.ok(Math.abs(p.x - (W / 2 + (x - camX) * sc)) < 1e-6);
    assert.ok(Math.abs(p.y - (horizon + camH * sc - z * sc)) < 1e-6);
    assert.ok(Math.abs(p.s - sc) < 1e-9);
  }
});

test('the skycam looks downfield from behind the offense', async () => {
  const { buildPlay, skyShot, makeView, project } = await import(pathToFileURL(path.join(__dirname, 'football-view.js')).href);
  const match = { teams: [team('home', '#1E5BD8'), team('away', '#C8102E')] };
  const A = { target: 'home-WR1', runner: 'home-RB0', defender: 'away-CB0', kicker: 'home-K0' };
  for (const off of [0, 1]) for (const q of [1, 2]) {
    const play = { n: 3, q, off, kind: 'pass', detail: 'deep', depth: 30, spot: 40, end: 70, yards: 30, complete: true, actors: A, dist: 10, down: 1 };
    const anim = buildPlay(play, match);
    for (const t of [0, .5, 1]) {
      const shot = skyShot(anim, play.kind, t);
      // Darrere la pilota (en el sentit de l'atac) i enlairada.
      assert.ok((shot.L[0] - shot.P[0]) * anim.dir > 10, 'looks along the attack');
      assert.ok(shot.P[2] > 5, 'raised');
      const b = anim.ball(t);
      assert.ok((b.x - shot.P[0]) * anim.dir > 5, 'camera behind the ball');
      const v = makeView(shot.P, shot.L, 900, 640, 360);
      const near = project(v, shot.P[0] + anim.dir * 12, 26.6), far = project(v, shot.P[0] + anim.dir * 40, 26.6);
      assert.ok(far.y < near.y, 'downfield is higher on screen');
      assert.ok(far.s < near.s, 'nearer is larger');
      // Esquerra i dreta: la banda y=53 queda a la dreta quan s'ataca cap a +x.
      const r = project(v, shot.P[0] + anim.dir * 20, 50), l = project(v, shot.P[0] + anim.dir * 20, 3);
      assert.ok(anim.dir > 0 ? r.x > l.x : r.x < l.x);
      assert.ok(project(v, shot.P[0] - anim.dir * 5, 26.6).d < 0, 'behind the camera');
    }
  }
  // Gol de camp: darrere el xutador mirant els pals.
  const fg = buildPlay({ n: 5, q: 1, off: 0, kind: 'fg', spot: 70, end: 70, good: true, actors: A }, match);
  const s0 = skyShot(fg, 'fg', 0), goalX = fg.dir > 0 ? 120 : 0;
  assert.equal(s0.L[0], goalX);
  assert.ok((fg.los - s0.P[0]) * fg.dir > 7);
});

test('the director picks cameras like a broadcast, player cam first', async () => {
  const { buildPlay, directPlay, liveSeg, CAM_TAGS } = await import(pathToFileURL(path.join(__dirname, 'football-view.js')).href);
  const match = { teams: [team('home', '#1E5BD8'), team('away', '#C8102E')] };
  const A = { target: 'home-WR1', runner: 'home-RB0', tackler: 'away-LB1', defender: 'away-CB0', kicker: 'home-K0', returner: 'away-WR0' };
  const mk = p => { const play = { q: 1, off: 0, down: 1, dist: 10, actors: A, ...p }; return [play, buildPlay(play, match)]; };
  let player = 0, total = 0;
  for (let n = 1; n <= 40; n++) {
    for (const p of [{ kind: 'run', spot: 30, end: 35, yards: 5 }, { kind: 'pass', detail: 'short', depth: 6, spot: 40, end: 47, yards: 7, complete: true }]) {
      const [play, anim] = mk({ n, ...p });
      const plan = directPlay(play, anim);
      assert.ok(plan.live.length >= 1 && plan.live[0].at === 0);
      for (const s of plan.live) assert.ok(s.cam in CAM_TAGS, s.cam);
      total++; if (plan.live.some(s => s.cam === 'player')) player++;
      assert.deepEqual(plan.replays, []);
    }
  }
  assert.ok(player / total > .6, 'player cam is the main live camera: ' + player + '/' + total);
  // Passada: darrere el quarterback i, quan arriba la pilota, darrere el receptor.
  const [pass, pa] = mk({ n: 2, kind: 'pass', detail: 'short', depth: 6, spot: 40, end: 47, yards: 7, complete: true });
  const pp = directPlay(pass, pa);
  assert.equal(pp.live[0].who, 'qb');
  const last = pp.live[liveSeg(pp, pa.marks.tCatch)];
  assert.equal(last.cam, 'player'); assert.equal(last.who, 'target');
  // Passada llarga: la skycam mentre vola la pilota.
  const [deep, da] = mk({ n: 2, kind: 'pass', detail: 'deep', depth: 35, spot: 20, end: 55, yards: 35, complete: true });
  assert.equal(directPlay(deep, da).live[liveSeg(directPlay(deep, da), (da.marks.tThrow + da.marks.tCatch) / 2)].cam, 'sky');
  // Touchdown: repeticions lentes des de la zona de marca i el pal.
  const [td, ta] = mk({ n: 7, kind: 'run', dir: 'middle', spot: 95, end: 100, yards: 5, td: true });
  const tp = directPlay(td, ta);
  assert.deepEqual(tp.replays.map(r => r.cam), ['endzone', 'pylon']);
  assert.ok(tp.replays.every(r => r.rate < 1 && r.from >= 0 && r.from < 1));
  // Retorn de xut inicial: dirigible i després el retornador.
  const [ko, ka] = mk({ n: 1, kind: 'kickoff', spot: 35, end: 27 });
  const kp = directPlay(ko, ka);
  assert.equal(kp.live[0].cam, 'blimp');
  assert.equal(kp.live[kp.live.length - 1].cam, 'player');
  assert.equal(kp.live[kp.live.length - 1].who, 'returner');
  const [fg, fa] = mk({ n: 5, kind: 'fg', spot: 70, end: 70, good: true });
  assert.equal(directPlay(fg, fa).live[0].cam, 'sky');
  // Càmera fixada.
  assert.deepEqual(directPlay(td, ta, 'tv'), { live: [{ cam: 'tv', at: 0, who: 'auto' }], replays: [] });
});

test('player cam rides low behind the ball carrier', async () => {
  const { buildPlay, playerShot, subjectOf, makeView, project, slowmo, shotFor } = await import(pathToFileURL(path.join(__dirname, 'football-view.js')).href);
  const match = { teams: [team('home', '#1E5BD8'), team('away', '#C8102E')] };
  const A = { target: 'home-WR1', runner: 'home-RB0', tackler: 'away-LB1', defender: 'away-CB0', kicker: 'home-K0', returner: 'away-WR0' };
  for (const off of [0, 1]) for (const q of [1, 2]) {
    const run = { n: 3, q, off, kind: 'run', dir: 'left', spot: 30, end: 42, yards: 12, actors: A, down: 1, dist: 10 };
    const anim = buildPlay(run, match);
    for (const t of [0, .1, .5, .8]) {
      const shot = playerShot(anim, run, t);
      const sub = subjectOf(anim, t).path(t);
      const back = Math.hypot(shot.P[0] - sub.x, shot.P[1] - sub.y);
      assert.ok(back > 3 && back < 5.5, 'distance behind: ' + back);
      assert.ok(shot.P[2] >= 1.8 && shot.P[2] <= 2.6, 'helmet height');
      // Mira endavant, cap on ataca qui porta la pilota.
      assert.ok((shot.L[0] - shot.P[0]) * anim.dir > 5);
      const v = makeView(shot.P, shot.L, 700, 640, 360);
      const p = project(v, sub.x, sub.y, 1);
      assert.ok(p.d > 3 && p.x > 300 && p.x < 980, 'subject in frame');
    }
    assert.equal(subjectOf(anim, .05).p.pos, 'QB');
    assert.equal(subjectOf(anim, .5).p.id, 'home-RB0'.replace('home', off ? 'away' : 'home'));
    assert.equal(slowmo(anim, run, .92), .35);
    assert.equal(slowmo(anim, run, .5), 1);
    // Totes les càmeres donen plans finits.
    for (const cam of ['player', 'sky', 'endzone', 'pylon', 'rail', 'blimp']) for (const t of [0, .5, 1]) {
      const s = shotFor(cam, anim, run, t, 'auto');
      assert.ok([...s.P, ...s.L, s.zoom].every(Number.isFinite), cam);
      assert.ok(Math.hypot(s.L[0] - s.P[0], s.L[1] - s.P[1], s.L[2] - s.P[2]) > 1, cam + ' looks somewhere');
    }
  }
  // Recepció: càmera lenta just quan arriba la pilota.
  const pass = { n: 4, q: 1, off: 0, kind: 'pass', detail: 'short', depth: 6, spot: 40, end: 47, yards: 7, complete: true, actors: A };
  const pa = buildPlay(pass, match);
  assert.equal(slowmo(pa, pass, pa.marks.tCatch), .3);
  assert.equal(subjectOf(pa, pa.marks.tCatch - .1, 'target').p.id, 'home-WR1');
});
