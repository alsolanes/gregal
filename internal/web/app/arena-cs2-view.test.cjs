const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const load = () => import(pathToFileURL(path.join(__dirname, 'arena-cs2-view.js')).href);

const meta = {
  game: 'cs2', map: 'Mirage', size: [100, 80], startT: 0, roundTime: 115, bombTime: 40,
  zones: [
    { id: 't_spawn', name: 'T Spawn', x: 90, y: 42, w: 14, h: 22, kind: 'spawn_t' },
    { id: 'a_main', name: 'A Main', x: 70, y: 64, w: 16, h: 9, kind: 'area', site: 'A' },
    { id: 'a_site', name: 'A Site', x: 40, y: 65, w: 20, h: 15, kind: 'site', site: 'A' },
    { id: 'ct_spawn', name: 'CT Spawn', x: 11, y: 50, w: 12, h: 14, kind: 'spawn_ct' },
    { id: 'b_site', name: 'B Site', x: 24, y: 15, w: 18, h: 13, kind: 'site', site: 'B' },
  ],
  links: [['t_spawn', 'a_main'], ['a_main', 'a_site'], ['a_site', 'ct_spawn'], ['ct_spawn', 'b_site']],
  props: [{ x: 36, y: 62, w: 3, h: 3 }],
  plant: { A: [41, 66], B: [24, 15] },
  teams: [0, 1].map(t => ({ name: ['Alpha', 'Bravo'][t], tag: ['ALP', 'BRA'][t], color: ['#2ec4b6', '#ff6b6b'][t], players: [0, 1, 2, 3, 4].map(i => ({ id: 'ab'[t] + i, handle: 'p' + t + i, role: ['IGL', 'AWPer', 'Entry', 'Support', 'Lurker'][i] })) })),
};

// Una ronda sintètica amb totes les menes d'esdeveniment.
function roundEvents(n0, round, score, opts = {}) {
  let n = n0;
  const ev = (kind, data, extra = {}) => ({ n: n++, kind, text: kind + ' text', score: extra.score || score, decision: round, data, ...(extra.hl ? { hl: true } : {}) });
  const players = [];
  for (let t = 0; t < 2; t++) for (let i = 0; i < 5; i++) players.push({ id: 'ab'[t] + i, team: t, side: t === 0 ? 'T' : 'CT', x: t ? 11 : 90, y: t ? 50 : 42, weapon: t ? 'm4a1s' : 'ak47', armor: 2, money: 1000 });
  const out = [
    ev('round_start', { t: 0, round, tTeam: 0, ctTeam: 1, pistol: round === 1, players, bomb: 'a3', money: [5000, 6000], calls: { t: { buy: 'full', strategy: 'execute_a', utility: 'full', pace: 'default', awp: true }, ct: { buy: 'full', setup: 'stack_b', utility: 'normal', awp: true } } }),
    ev('buy', { t: 0, team: 0, side: 'T', level: 'full', spent: 18000, items: players.filter(p => p.team === 0).map(p => ({ id: p.id, weapon: p.id === 'a1' ? 'awp' : 'ak47', armor: 2, money: 300, spent: 3700 })) }),
    ev('buy', { t: 0, team: 1, side: 'CT', level: 'force', spent: 12000, items: players.filter(p => p.team === 1).map(p => ({ id: p.id, weapon: 'famas', armor: 1, kit: true, money: 100, spent: 2700 })) }),
  ];
  for (let i = 0; i < 5; i++) out.push(ev('move', { t: 1 + i * 0.2, id: 'a' + i, team: 0, path: ['t_spawn', 'a_main', 'a_site'], pts: [[90, 42], [70, 64], [40 + i, 65]], dur: 15, to: 'a_site' }));
  for (let i = 0; i < 5; i++) out.push(ev('move', { t: 0.5, id: 'b' + i, team: 1, path: ['ct_spawn', 'a_site'], pts: [[11, 50], [38, 62 + i]], dur: 8, to: 'a_site' }));
  out.push(ev('utility', { t: 14, id: 'a0', team: 0, type: 'smoke', zone: 'a_site', x: 30, y: 70, fx: 70, fy: 64, dur: 18 }));
  out.push(ev('utility', { t: 15, id: 'a1', team: 0, type: 'flash', zone: 'a_site', x: 40, y: 65, fx: 70, fy: 64, dur: 1.5 }));
  out.push(ev('utility', { t: 15.5, id: 'a2', team: 0, type: 'molotov', zone: 'a_site', x: 41, y: 66, fx: 70, fy: 64, dur: 7 }));
  out.push(ev('utility', { t: 16, id: 'b2', team: 1, type: 'he', zone: 'a_main', x: 68, y: 64, fx: 38, fy: 62, dur: 1 }));
  out.push(ev('kill', { t: 17, killer: 'a2', victim: 'b0', kteam: 0, vteam: 1, weapon: 'ak47', hs: true, smoke: false, first: true, kx: 45, ky: 65, vx: 38, vy: 62, zone: 'a_site' }));
  out.push(ev('kill', { t: 18.5, killer: 'b1', victim: 'a3', kteam: 1, vteam: 0, weapon: 'famas', hs: false, smoke: true, kx: 38, ky: 63, vx: 44, vy: 65, zone: 'a_site' }));
  for (let i = 2; i < 5; i++) out.push(ev('kill', { t: 18 + i, killer: 'a1', victim: 'b' + i, kteam: 0, vteam: 1, weapon: 'awp', hs: false, kx: 50, ky: 66, vx: 38, vy: 62 + i, zone: 'a_site' }, { hl: i === 4 }));
  if (opts.defuse) {
    out.push(ev('plant', { t: 26, id: 'a0', team: 0, site: 'A', x: 41, y: 66, start: 22.8, explodeAt: 66 }));
    out.push(ev('kill', { t: 40, killer: 'b1', victim: 'a0', kteam: 1, vteam: 0, weapon: 'famas', hs: true, kx: 30, ky: 70, vx: 41, vy: 66 }));
    out.push(ev('kill', { t: 41, killer: 'b1', victim: 'a1', kteam: 1, vteam: 0, weapon: 'famas', hs: true, kx: 30, ky: 70, vx: 50, vy: 66 }));
    out.push(ev('kill', { t: 42, killer: 'b1', victim: 'a2', kteam: 1, vteam: 0, weapon: 'famas', hs: false, kx: 30, ky: 70, vx: 45, vy: 65 }));
    out.push(ev('kill', { t: 43, killer: 'b1', victim: 'a4', kteam: 1, vteam: 0, weapon: 'famas', hs: false, kx: 30, ky: 70, vx: 44, vy: 64 }));
    out.push(ev('defuse', { t: 49, id: 'b1', team: 1, start: 44, dur: 5, kit: true, ninja: false, x: 42, y: 67, left: 17 }, { hl: true }));
    out.push(ev('round_end', { t: 49.01, round, winner: 1, side: 'CT', reason: 'defuse', mvp: 'b1', tags: ['clutch'], clutch: 4, score: [score[0], score[1] + 1], money: [9000, 12000], players: [] }, { score: [score[0], score[1] + 1], hl: true }));
  } else {
    out.push(ev('plant', { t: 26, id: 'a0', team: 0, site: 'A', x: 41, y: 66, start: 22.8, explodeAt: 66 }));
    out.push(ev('kill', { t: 30, killer: 'a0', victim: 'b1', kteam: 0, vteam: 1, weapon: 'ak47', hs: true, kx: 41, ky: 66, vx: 30, vy: 70 }));
    out.push(ev('explode', { t: 66, site: 'A', x: 41, y: 66 }));
    out.push(ev('round_end', { t: 66.01, round, winner: 0, side: 'T', reason: 'bomb', mvp: 'a1', tags: ['pistol', 'ace'], clutch: 0, score: [score[0] + 1, score[1]], money: [16000, 9000], players: [] }, { score: [score[0] + 1, score[1]], hl: true }));
  }
  return out;
}

function matchEvents() {
  let ev = roundEvents(1, 1, [0, 0]);
  ev = ev.concat(roundEvents(ev.length + 1, 2, [1, 0], { defuse: true }));
  let n = ev.length + 1;
  ev.push({ n: n++, kind: 'half', text: 'Half', score: [1, 1], decision: 2, data: { after: 12, score: [1, 1], tTeam: 1, ot: false, money: 800 } });
  ev = ev.concat(roundEvents(n, 3, [1, 1]));
  n = ev.length + 1;
  ev.push({ n, kind: 'match_end', text: 'Alpha win', hl: true, score: [2, 1], decision: 3, data: { winner: 0, score: [2, 1], rounds: 3, mvp: 'a1', ot: 0, players: meta.teams.flatMap((t, ti) => t.players.map(p => ({ id: p.id, handle: p.handle, team: ti, k: 3, d: 2, hs: 1, fk: 0, adr: 80.5, clutches: 0, plants: 0, defuses: 0, rating: 1.1 }))) } });
  return ev;
}

const finite = (...v) => v.every(x => Number.isFinite(x));

test('pure helpers produce finite positions, timelines and states for every event kind', async () => {
  const V = await load();
  const events = matchEvents();
  const rounds = V.groupRounds(events);
  assert.equal(rounds.length, 3);
  assert.ok(rounds.every(r => r.end));
  assert.equal(rounds[1].after[0].kind, 'half');
  assert.equal(rounds[2].after[0].kind, 'match_end');
  const kinds = new Set(events.map(e => e.kind));
  for (const k of ['round_start', 'buy', 'move', 'utility', 'kill', 'plant', 'defuse', 'explode', 'round_end', 'half', 'match_end']) assert.ok(kinds.has(k), k);

  for (const g of rounds) {
    const R = V.buildRound(g, meta);
    assert.ok(R.tl.total > 3 && R.tl.total < 30, 'round length ' + R.tl.total);
    assert.equal(Object.keys(R.players).length, 10);
    for (let p = 0; p <= R.tl.total + 0.5; p += 0.1) {
      const gt = V.gameTimeAt(R.tl, p);
      assert.ok(finite(gt) && gt >= 0 && gt <= R.endT + 1e-6);
      for (const id of Object.keys(R.players)) {
        const pos = V.posAt(R.players[id].track, gt);
        assert.ok(finite(pos.x, pos.y), `pos ${id} @${gt}`);
        assert.ok(pos.x >= 0 && pos.x <= 100 && pos.y >= 0 && pos.y <= 80);
        assert.ok(finite(V.angleAt(R, id, gt)));
      }
      for (const u of R.utils) {
        const st = V.utilState(u, gt, p);
        if (st) assert.ok(finite(st.x, st.y, st.r, st.a), u.type);
      }
      const b = V.bombState(R, gt);
      assert.ok(finite(b.left, b.x, b.y));
      if (b.planting) assert.ok(b.planting.prog >= 0 && b.planting.prog <= 1);
      if (b.defusing) assert.ok(b.defusing.prog >= 0 && b.defusing.prog <= 1);
      const alive = V.aliveAt(R, gt);
      assert.ok(alive[0] <= 5 && alive[1] <= 5);
      assert.ok(V.feedAt(R, p).length <= 5);
    }
    // gameTime i playTime són inverses (fora de les pauses)
    for (const k of R.kills) assert.ok(Math.abs(V.gameTimeAt(R.tl, k._p) - k.t) < 1e-6);
    // els esdeveniments queden ordenats per temps de reproducció
    for (let i = 1; i < R.events.length; i++) assert.ok(R.events[i]._p >= R.events[i - 1]._p);
  }
  const R0 = V.buildRound(rounds[0], meta);
  assert.equal(V.bombState(R0, 5).carrier, 'a3');
  assert.ok(V.bombState(R0, 19).dropped, 'bomb dropped when carrier dies');
  assert.ok(V.bombState(R0, 24).planting);
  assert.ok(V.bombState(R0, 30).planted);
  assert.ok(V.bombState(R0, 70).exploded);
  const R1 = V.buildRound(rounds[1], meta);
  assert.ok(V.bombState(R1, 46).defusing);
  assert.ok(V.bombState(R1, 49.5).defused);
  assert.deepEqual(V.aliveAt(R1, 49), [0, 1]);
  const f = V.fitMap([100, 80], { x: 10, y: 50, w: 400, h: 200 });
  assert.ok(finite(f.s, f.ox, f.oy) && f.s === 2.5);
  assert.equal(V.fmtClock(65.2), '1:06');
  assert.equal(V.prettyCall('fake_a_go_b'), 'FAKE A → B');
  assert.ok(V.TEXT.ca.reasons.defuse && V.TEXT.en.reasons.bomb);
  // dades en text i esdeveniments incomplets
  const odd = V.groupRounds([{ n: 1, kind: 'round_start', data: JSON.stringify({ t: 0, round: 1, players: [] }) }, { n: 2, kind: 'kill', data: '{bad' }]);
  assert.equal(odd.length, 1); assert.equal(odd[0].end, null);
  const Ro = V.buildRound(odd[0], {});
  assert.ok(finite(Ro.tl.total));
});

// Teles falses perquè la classe pugui córrer sense DOM.
function fakeCtx() {
  const grad = { addColorStop() {} };
  return new Proxy({}, {
    get(t, k) {
      if (k in t) return t[k];
      if (k === 'measureText') return s => ({ width: String(s).length * 6 });
      if (k.startsWith('create')) return () => grad;
      return () => {};
    },
    set(t, k, v) { t[k] = v; return true; },
  });
}

test('the view plays a whole match, reports events and finishes', async () => {
  const V = await load();
  let frame = null;
  global.window = { addEventListener() {}, removeEventListener() {}, devicePixelRatio: 1 };
  global.requestAnimationFrame = f => { frame = f; return 1; };
  global.document = { createElement: () => ({ width: 0, height: 0, getContext: () => fakeCtx() }) };
  const events = matchEvents();
  const canvas = { getContext: () => fakeCtx(), getBoundingClientRect: () => ({ width: 960, height: 540 }), width: 0, height: 0 };
  const source = { load: async after => ({ status: 'final', names: ['Prompt A', 'Prompt B'], meta, events: events.filter(e => e.n > after), score: [2, 1], decisions: [] }) };
  const seen = [];
  const view = new V.Cs2View(canvas, { lang: 'ca', source, onEvent: e => seen.push(e.n) });
  await new Promise(r => setTimeout(r, 10));
  assert.equal(view.names()[0], 'Prompt A');
  assert.equal(view.isFinished(), false);
  let now = 0;
  const modes = new Set(), phases = new Set();
  for (let i = 0; i < 4000 && !view.isFinished(); i++) { now += 100; view.advance(0.1); view.draw(); modes.add(view.povOn() ? 'pov' : 'map'); phases.add(view.phase); }
  assert.ok(view.isFinished(), 'finished after playback');
  assert.ok(modes.has('pov') && modes.has('map'), 'the director alternates map and POV');
  assert.ok(phases.has('replay'), 'a kill-cam replay was shown');
  assert.equal(view.T, V.TEXT.en, 'always English');
  assert.equal(new Set(seen).size, events.length, 'every event reported once');
  assert.deepEqual(view.score, [2, 1]);
  view.draw();
  // pausa i velocitat
  assert.equal(view.togglePause(), true);
  assert.equal(view.togglePause(), false);
  view.setSpeed(4);
  view.destroy();
  void frame; void now;

  // skipToEnd mostra el final de seguida, també en un visor estret
  const narrowCanvas = { ...canvas, getBoundingClientRect: () => ({ width: 380, height: 640 }) };
  const v2 = new V.Cs2View(narrowCanvas, { lang: 'en', source });
  await new Promise(r => setTimeout(r, 10));
  v2.skipToEnd();
  assert.ok(v2.isFinished());
  v2.draw();
  v2.destroy();

  // en directe: sense estat final no s'acaba mai
  const live = { load: async after => ({ status: 'live', names: ['A', 'B'], meta, events: events.filter(e => e.n > after && e.n < 30), score: [1, 0] }) };
  const v3 = new V.Cs2View(canvas, { lang: 'en', source: live });
  await new Promise(r => setTimeout(r, 10));
  for (let i = 0; i < 600; i++) { v3.advance(0.1); v3.draw(); }
  assert.equal(v3.isFinished(), false);
  assert.equal(v3.phase, 'wait');
  v3.destroy();
});

test('spectator cam: grid, raycast, projection and director', async () => {
  const V = await load();
  const G = V.buildGrid(meta);
  assert.equal(G.gw, 200); assert.equal(G.gh, 160);
  assert.equal(V.cellAt(G, 40, 65), 1, 'A site is floor');
  assert.equal(V.cellAt(G, 36, 62), 2, 'a crate on A');
  assert.equal(V.cellAt(G, 2, 2), 0, 'outside the map is wall');
  assert.equal(V.cellAt(G, 80, 53), 1, 'corridor between T spawn and A main');
  // From the middle of A site (20 wide, x 30..50) looking east and west the walls are ~10 away
  // (the A main corridor opens east, so look west only), north/south ~7.5.
  const west = V.castRay(G, 40, 72, -1, 0);
  assert.ok(west.hit && west.d > 9 && west.d < 11.5, 'west wall ' + west.d);
  const south = V.castRay(G, 44, 65, 0, 1);
  assert.ok(south.hit && south.d > 6.5 && south.d < 9, 'south wall ' + south.d);
  // A ray through the crate reports it and still finds the wall behind.
  const crates = [];
  const h = V.castRay(G, 40, 62.5, -1, 0, 90, crates);
  assert.ok(crates.length === 1 && crates[0].d0 > 2 && crates[0].d1 > crates[0].d0 && h.d > crates[0].d1);
  // Starting inside a wall walks out of it instead of returning 0.
  const out = V.castRay(G, 29.8, 65, 1, 0);
  assert.ok(out.d > 5, 'escapes the wall it starts in');
  assert.equal(V.castRay(G, 40, 65, 0, 0).hit, false, 'zero direction does not loop');
  assert.ok(V.sees(G, 35, 65, 45, 65));
  assert.ok(!V.sees(G, 40, 65, 24, 15), 'A cannot see B');
  // Projection: straight ahead is screen centre, the right-hand side (y down) is to the right.
  const cam = { x: 0, y: 0, ang: 0, eye: 1.6, focal: 500, cx: 400, horizon: 300 };
  const c = V.projectPoint(cam, 10, 0, 1.6);
  assert.equal(c.x, 400); assert.equal(c.y, 300); assert.equal(c.depth, 10);
  assert.ok(V.projectPoint(cam, 10, 2, 1.6).x > 400, 'right');
  assert.ok(V.projectPoint(cam, 10, 0, 0).y > 300, 'floor below horizon');
  assert.ok(V.projectPoint(cam, -5, 0, 0).behind);
  assert.ok(Math.abs(V.projectPoint({ ...cam, ang: Math.PI / 2 }, 0, 10, 1.6).x - 400) < 1e-9);
  assert.ok(Math.abs(V.angDiff(0.1, Math.PI * 2 - 0.1) - 0.2) < 1e-9);

  const R = V.buildRound(V.groupRounds(matchEvents())[0], meta);
  const first = R.kills[0];
  // Just before the first kill the director is on the killer, pre-aiming at the victim.
  const w = V.directorWant(R, first._p - 0.3, V.gameTimeAt(R.tl, first._p - 0.3));
  assert.equal(w.mode, 'pov'); assert.equal(w.id, first.killer); assert.equal(w.why, 'kill');
  assert.equal(V.directorWant(R, 0.2, V.gameTimeAt(R.tl, 0.2)).mode, 'map', 'map during the setup');
  const aim = V.povAim(R, first.killer, first.t);
  const want = Math.atan2(first.vy - first.ky, first.vx - first.kx);
  assert.ok(Math.abs(V.angDiff(aim, want)) < 0.6, 'aims at the victim');
  // planting shows the planter
  const pp = V.playTimeAt(R.tl, (R.plant.start + R.plant.t) / 2);
  const wp = V.directorWant(R, pp, V.gameTimeAt(R.tl, pp));
  assert.ok(wp.mode === 'pov');
  const rk = V.pickReplay(R);
  assert.ok(rk && rk.killer === 'a1', 'replay the AWP multi-kill');
  assert.ok(V.shotsAt(R, first._p).some(s => s.kill && s.from === first.killer));
  const hs = V.hudStats(R, 'b0', first.t + 1);
  assert.equal(hs.hp, 0);
  const ha = V.hudStats(R, 'a1', 0);
  assert.equal(ha.hp, 100); assert.equal(ha.ammo, 5);
  assert.equal(V.weaponClass('usp'), 'pistol'); assert.equal(V.weaponClass('awp'), 'awp');
});
