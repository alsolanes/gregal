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
    assert.ok(anim.contacts.length === 1, 'the run ends in a tackle');
    assert.equal(slowmo(anim, run, anim.contacts[0].tc + .02), .35);
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

test('seek lands on any play with the same state as playing straight through; seek(0) restarts', async () => {
  const { FootballView } = await import(pathToFileURL(path.join(__dirname, 'football-view.js')).href);
  const match = { teams: [team('home', '#1E5BD8'), team('away', '#C8102E')] };
  global.window = { addEventListener() {}, removeEventListener() {}, devicePixelRatio: 1 };
  global.requestAnimationFrame = () => 1;
  const A = { target: 'home-WR1', runner: 'home-RB0', tackler: 'away-LB1', defender: 'away-CB0', kicker: 'home-K0', returner: 'away-WR0' };
  const kinds = [
    { kind: 'kickoff', spot: 35, end: 25, detail: 'touchback' }, { kind: 'run', dir: 'left', spot: 25, end: 31, yards: 6 },
    { kind: 'pass', detail: 'deep', depth: 30, spot: 31, end: 70, yards: 39, complete: true, hl: true }, { kind: 'run', dir: 'middle', spot: 95, end: 100, yards: 5, td: true, hl: true },
    { kind: 'pat', spot: 98, end: 98, good: true }, { kind: 'sack', spot: 40, end: 33, yards: -7 }, { kind: 'punt', spot: 33, end: 75, yards: 42 },
  ];
  let sc = [0, 0];
  const plays = Array.from({ length: 21 }, (_, i) => {
    const p = { n: i + 1, q: 1 + Math.floor(i / 6), clock: 900 - i * 30, off: Math.floor(i / 7) % 2, down: 1, dist: 10, actors: A, text: 'play ' + (i + 1), ...kinds[i % kinds.length] };
    if (p.td) sc = sc.map((v, s) => s === p.off ? v + 6 : v);
    if (p.kind === 'pat') sc = sc.map((v, s) => s === p.off ? v + 1 : v);
    return { ...p, score: sc.slice() };
  });
  const canvas = { getContext: () => new Proxy({}, { get: () => () => ({ addColorStop() {} }), set: () => true }), getBoundingClientRect: () => ({ width: 960, height: 540 }), width: 0, height: 0 };
  const mk = (status, upto, onEvent) => new FootballView(canvas, { source: { load: async after => ({ ...match, status, plays: plays.slice(0, upto).filter(p => p.n > after) }) }, onEvent });
  const snap = v => JSON.stringify({ idx: v.idx, n: v.cur && v.cur.play.n, score: v.score(), phase: v.phase, pos: v.cur && v.cur.anim.tracks.map(tr => tr.path(.5)), ball: v.cur && v.cur.anim.ball(.7),
    frame: v.cur && v.cur.anim.frameAt(.93), pose: v.cur && v.cur.anim.tracks.map((_, i) => v.cur.anim.poseOf(i, .91)), loose: v.cur && v.cur.anim.ball(.99) });
  const straight = new Map();
  const a = mk('final', 99, p => straight.set(p.n, snap(a)));
  await new Promise(r => setTimeout(r, 10));
  for (let i = 0; i < 20000 && !a.isFinished(); i++) a.advance(.1);
  assert.equal(straight.size, plays.length);
  a.destroy();

  const seen = [], got = new Map();
  const v = mk('final', 99, p => { seen.push(p.n); got.set(p.n, snap(v)); });
  await new Promise(r => setTimeout(r, 10));
  for (let i = 0; i < 120; i++) v.advance(.1);
  for (const k of [15, 2, 3, 20, 9, 0]) {
    seen.length = 0;
    v.seek(k);
    assert.deepEqual(seen, []);
    assert.ok(!v.cur && !v.banner && v.particles.length === 0, 'nothing carried over');
    assert.equal(v.getProgress().index, k);
    v.advance(.1);
    assert.deepEqual(seen, [plays[k].n], 'onEvent once, for the play landed on');
    assert.equal(got.get(plays[k].n), straight.get(plays[k].n), 'same state at play ' + k);
  }
  // Restart: from the first play, 0-0.
  v.seek(0);
  assert.deepEqual(v.score(), [0, 0]); assert.equal(v.idx, 0);
  // Past the end: clamped, finished.
  v.seek(999);
  assert.ok(v.isFinished());
  const pr = v.getProgress();
  assert.deepEqual([pr.index, pr.total, pr.live], [plays.length, plays.length, false]);
  assert.ok(pr.marks.some(m => m.kind === 'score') && pr.marks.every(m => plays[m.i].hl || m.kind === 'score'));
  v.destroy();

  // Live: what has loaded grows; seeking past it waits at the edge.
  const w = mk('live', 8);
  await new Promise(r => setTimeout(r, 10));
  w.seek(50);
  assert.equal(w.idx, 8); assert.equal(w.getProgress().live, true);
  w.advance(.1); assert.ok(!w.cur);
  w.destroy();
});

test('playback time: the estimate matches playing at 1×, and elapsed only moves forward', async () => {
  const { FootballView, playTimes, playSeconds } = await import(pathToFileURL(path.join(__dirname, 'football-view.js')).href);
  const match = { teams: [team('home', '#1E5BD8'), team('away', '#C8102E')] };
  global.window = { addEventListener() {}, removeEventListener() {}, devicePixelRatio: 1 };
  global.requestAnimationFrame = () => 1;
  const A = { target: 'home-WR1', runner: 'home-RB0', tackler: 'away-LB1', defender: 'away-CB0', kicker: 'home-K0', returner: 'away-WR0' };
  const plays = [
    { kind: 'kickoff', spot: 35, end: 27 }, { kind: 'run', dir: 'left', spot: 27, end: 33, yards: 6 },
    { kind: 'pass', detail: 'deep', depth: 30, spot: 33, end: 95, yards: 62, complete: true, hl: true }, { kind: 'run', dir: 'middle', spot: 95, end: 100, yards: 5, td: true, hl: true, score: [6, 0] },
    { kind: 'pat', spot: 98, end: 98, good: true, score: [7, 0] }, { kind: 'sack', spot: 40, end: 33, yards: -7 },
  ].map((p, i) => ({ score: [0, 0], n: i + 1, q: 1, clock: 900 - i * 30, off: 0, down: 1, dist: 10, actors: A, text: 't', ...p }));
  for (const p of plays.slice(4)) p.score = [7, 0];
  const times = playTimes(plays, match);
  assert.equal(times.length, plays.length + 1);
  assert.ok(times.every((t, i) => i === 0 || t > times[i - 1]));
  assert.ok(playSeconds(plays[3], match) > playSeconds(plays[1], match), 'a touchdown (replays) lasts longer than a run');
  const canvas = { getContext: () => new Proxy({}, { get: () => () => ({ addColorStop() {} }), set: () => true }), getBoundingClientRect: () => ({ width: 960, height: 540 }), width: 0, height: 0 };
  let v, pr;
  // The director's cameras (replays, slow motion at the catch) and the fixed TV camera.
  for (const [cam, tol] of [['auto', .1], ['tv', .03]]) {
    v = new FootballView(canvas, { source: { load: async after => ({ ...match, status: 'final', plays: plays.filter(p => p.n > after) }) } });
    await new Promise(r => setTimeout(r, 10));
    if (cam !== 'auto') v.setCamera(cam);
    let t = 0, last = -1;
    for (let i = 0; i < 20000 && !v.isFinished(); i++) {
      v.advance(.02); t += .02;
      const p = v.getProgress();
      assert.ok(p.elapsed >= last - 1e-9, 'elapsed never goes back while playing');
      last = p.elapsed;
    }
    pr = v.getProgress();
    assert.equal(pr.elapsed, pr.duration);
    assert.ok(Math.abs(t - pr.duration) / pr.duration < tol, `${cam}: estimate ${pr.duration.toFixed(1)} s vs played ${t.toFixed(1)} s`);
    if (cam === 'auto') v.destroy();
  }
  v.seek(3);
  assert.equal(v.getProgress().elapsed, pr.times[3], 'after a seek: the start of the play landed on');
  v.destroy();
});

test('ball physics, catches and tackles: deterministic, attached to the hands, bouncing, no overlaps', async () => {
  const { buildPlay, looseBall, looseAt, bounceApexes, hitStop, ARM_REACH, makeView, project, skyShot } = await import(pathToFileURL(path.join(__dirname, 'football-view.js')).href);
  const match = { teams: [team('home', '#1E5BD8'), team('away', '#C8102E')] };
  const A = { passer: 'home-QB0', target: 'home-WR1', runner: 'home-RB0', tackler: 'away-LB1', defender: 'away-CB0', kicker: 'home-K0', returner: 'away-WR0' };
  const plays = [
    { kind: 'run', dir: 'left', spot: 30, end: 36, yards: 6 },
    { kind: 'run', dir: 'right', spot: 20, end: 60, yards: 40, hl: true },
    { kind: 'run', dir: 'middle', spot: 40, end: 43, yards: 3, fumble: true },
    { kind: 'run', dir: 'middle', spot: 95, end: 100, yards: 5, td: true },
    { kind: 'pass', detail: 'short', depth: 6, spot: 40, end: 49, yards: 9, complete: true },
    { kind: 'pass', detail: 'deep', depth: 35, spot: 20, end: 20, yards: 0 },
    { kind: 'pass', detail: 'medium', depth: 14, spot: 50, end: 64, int: true },
    { kind: 'pass', detail: 'short', depth: 5, spot: 30, end: 30, yards: 0, hl: true },
    { kind: 'sack', spot: 45, end: 38, yards: -7, hl: true },
    { kind: 'scramble', spot: 45, end: 52, yards: 7 },
    { kind: 'punt', spot: 30, end: 75, yards: 45 },
    { kind: 'fg', spot: 70, end: 70, good: true },
    { kind: 'kickoff', spot: 35, end: 30, detail: 'touchback' },
    { kind: 'kickoff', spot: 35, end: 27 },
    { kind: 'kneel', spot: 60, end: 59, yards: -1 },
  ];
  let held = 0, worstSep = 9, worstPx = 0;
  plays.forEach((p, k) => {
    for (const off of [0, 1]) {
      const play = { n: k + 3, q: 1 + off, off, down: 1, dist: 10, actors: A, ...p };
      const a = buildPlay(play, match), b = buildPlay(play, match);
      const shot = skyShot(a, play.kind, .5), view = makeView(shot.P, shot.L, 900, 640, 360);
      for (let t = 0; t <= 1.0001; t += .02) {
        // Determinisme: la mateixa jugada dona la mateixa pilota, posicions i postures.
        assert.deepEqual(a.ball(t), b.ball(t), `${p.kind} ball at ${t}`);
        assert.deepEqual(a.frameAt(t), b.frameAt(t), `${p.kind} frame at ${t}`);
        a.tracks.forEach((_, i) => assert.deepEqual(a.poseOf(i, t), b.poseOf(i, t), `${p.kind} pose ${i} at ${t}`));
        const ball = a.ball(t);
        assert.ok([ball.x, ball.y, ball.z, ...ball.axis].every(Number.isFinite) && ball.z >= 0, `${p.kind} ball finite at ${t}`);
        // A la mà: mai més lluny que on arriba el braç (al món i a la pantalla).
        if (ball.holder >= 0) {
          held++;
          const sh = a.shoulderOf(ball.holder, t), d = Math.hypot(ball.x - sh.x, ball.y - sh.y, ball.z - sh.z);
          assert.ok(d <= ARM_REACH + 1e-9, `${p.kind}: ball ${d.toFixed(2)} yd from the hand of its carrier at ${t}`);
          const ps = project(view, sh.x, sh.y, sh.z), pb = project(view, ball.x, ball.y, ball.z);
          if (ps.d > 2 && pb.d > 2) { const px = Math.hypot(ps.x - pb.x, ps.y - pb.y) / (ARM_REACH * ps.s); worstPx = Math.max(worstPx, px); assert.ok(px <= 1.08, `${p.kind}: on screen the ball is ${px.toFixed(2)} arm lengths away at ${t}`); }
        }
        // Ningú no travessa ningú, llevat del punt de contacte.
        const P = a.frameAt(t);
        for (let i = 0; i < P.length; i++) for (let j = i + 1; j < P.length; j++) {
          if (a.inContact(i, j, t)) continue;
          const dd = Math.hypot(P[i].x - P[j].x, P[i].y - P[j].y);
          worstSep = Math.min(worstSep, dd);
          assert.ok(dd > .45, `${p.kind}: players ${i},${j} overlap (${dd.toFixed(2)} yd) at ${t}`);
        }
      }
      // Hit-stop: no fa enrere i no allarga la jugada.
      let prev = -1;
      for (let t = 0; t <= 1; t += .001) { const h = hitStop(a, t); assert.ok(h >= prev - 1e-9 && Math.abs(h - t) < .03); prev = h; }
      assert.equal(hitStop(a, 1), 1);
    }
  });
  assert.ok(held > 300 && worstSep > .45);

  // Bots: cada vol és més baix que l'anterior, i al final queda quieta a terra.
  for (const seed of [1, 2, 3, 99]) {
    const segs = looseBall([10, 20, 2.5], [7, 2, 3], seed), ap = bounceApexes(segs);
    assert.ok(ap.length >= 2, 'it bounces');
    for (let i = 2; i < ap.length; i++) assert.ok(ap[i] < ap[i - 1], 'bounce heights decay: ' + ap.map(v => v.toFixed(2)));
    const end = looseAt(segs, 60), later = looseAt(segs, 90);
    assert.deepEqual(end, later); assert.ok(end.z <= .11);
  }
  // Passada incompleta: rebota, bota i es para.
  const inc = buildPlay({ n: 9, q: 1, off: 0, down: 1, dist: 10, actors: A, kind: 'pass', detail: 'deep', depth: 35, spot: 20, end: 20, yards: 0 }, match);
  const lo = inc.timeline.find(s => s.type === 'loose');
  assert.ok(lo && Math.abs(lo.t0 - inc.marks.tCatch) < 1e-9, 'it comes loose at the catch point');
  const ap = bounceApexes(lo.segs);
  assert.ok(ap.length >= 2 && ap.slice(1).every((v, i) => v < ap[i]), 'it pops up, then the bounces decay: ' + ap.map(v => v.toFixed(2)));
  const zs = []; for (let t = inc.marks.tCatch; t <= 1; t += .002) zs.push(inc.ball(t).z);
  assert.ok(Math.max(...zs) > zs[0], 'the ball pops off the hands');
  assert.equal(inc.ball(1).mode, 'loose');
  // Entrega en mà, i el llançament amb el braç enrere.
  const run = buildPlay({ n: 4, q: 1, off: 0, down: 1, dist: 10, actors: A, kind: 'run', dir: 'left', spot: 30, end: 36, yards: 6 }, match);
  let hand = false;
  for (let t = 0; t <= .4; t += .002) { const bl = run.ball(t); if (bl.giver >= 0 && bl.holder >= 0) { const q1 = run.frameAt(t)[bl.giver], q2 = run.frameAt(t)[bl.holder]; if (Math.hypot(q1.x - q2.x, q1.y - q2.y) < 1) hand = true; } }
  assert.ok(hand, 'hand-to-hand handoff');
  const pass = buildPlay({ n: 5, q: 1, off: 0, down: 1, dist: 10, actors: A, kind: 'pass', detail: 'short', depth: 6, spot: 40, end: 49, yards: 9, complete: true }, match);
  assert.ok(pass.ball(pass.marks.tThrow - .001).z > 1.8, 'the arm is cocked at the release');
  assert.equal(pass.ball(pass.marks.tCatch + .1).grip, 'tuck');
  // Placatge: contacte, caiguda i a terra.
  assert.equal(run.contacts.length, 1);
  const c = run.contacts[0];
  assert.ok(c.tc > .3 && c.tc < .95);
  assert.equal(run.fallOf(c.carrier, c.tc + .15).k, 1);
  assert.equal(run.fallOf(c.tackler, c.tc + .15).k, 1);
});
