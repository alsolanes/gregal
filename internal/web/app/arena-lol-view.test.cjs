const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const load = () => import(pathToFileURL(path.join(__dirname, 'arena-lol-view.js')).href);

const ROLES = ['top', 'jungle', 'mid', 'adc', 'support'];
const champs = ['Ironwall', 'Nightfang', 'Emberlance', 'Swiftarrow', 'Chainwarden', 'Rageborn', 'Wildroar', 'Pyrelord', 'Longshot', 'Ironhorn'];
const meta = {
  game: 'lol', blue: 1,
  teams: [0, 1].map(t => ({ name: `Team ${t}`, side: t === 1 ? 'blue' : 'red', color: t === 1 ? '#2f7df6' : '#e8414e', players: ROLES.map((r, i) => ({ handle: `P${t}${i}`, role: r })) })),
  champions: champs.map((n, i) => ({ name: n, roles: [ROLES[i % 5]], class: ['tank', 'assassin', 'mage', 'marksman', 'engage'][i % 5], dmg: 'AD', curve: 'mid' })),
};

const hud = (k = [0, 0]) => ({ k, tw: [0, 0], dr: [0, 0], g: [10000, 9500], bn: [0, 0], gr: [0, 0] });
const ref = (side, slot) => ({ side, slot, champ: champs[side * 5 + slot], handle: 'x' });

function events() {
  const ev = [];
  let n = 0;
  const add = (kind, data, hl = false) => ev.push({ n: ++n, kind, text: kind, hl, score: [0, 0], decision: 1, data });
  add('phase', { phase: 'draft', t: 0 });
  add('draft_ban', { side: 0, champ: 'Rageborn', order: 1, slot: 0, step: 'ban phase 1', t: 0 });
  champs.forEach((c, i) => add('draft_pick', { side: i < 5 ? 0 : 1, champ: c, role: ROLES[i % 5], slot: i % 5, class: 'mage', roles: { [c]: ROLES[i % 5] }, t: 0 }));
  add('phase', { phase: 'draft_done', t: 0, lineups: [0, 1].map(t => ROLES.map((r, i) => ({ role: r, handle: `P${t}${i}`, champ: champs[t * 5 + i] }))) });
  add('phase', { phase: 'laning', t: 0, hud: hud() });
  add('gank', { side: 1, lane: 'bot', x: 82, y: 89, success: true, t: 200, hud: hud() });
  add('kill', { x: 82, y: 89, killer: ref(1, 1), victim: ref(0, 3), assists: [3], multi: 1, tag: 'first_blood', side: 1, t: 206, hud: hud([0, 1]) }, false);
  add('lane', { lane: 'top', diff: 400, x: 11, y: 18, side: 0, t: 300, hud: hud([0, 1]) });
  add('phase', { phase: 'macro', t: 840, choices: [{ objective: 'dragon', stance: 'engage' }, { objective: 'towers', stance: 'split' }], hud: hud([0, 1]) });
  add('teamfight', { x: 69, y: 71, size: 'teamfight', winner: 0, side: 0, t: 900, hud: hud([0, 1]) });
  for (let s = 0; s < 5; s++) add('kill', { x: 69, y: 71, killer: ref(0, 2), victim: ref(1, s), assists: [0, 1], multi: s + 1, ace: s === 4, side: 0, t: 903 + s, hud: hud([s + 1, 1]) }, s >= 3);
  add('dragon', { side: 0, type: 'infernal', count: 1, x: 69, y: 71, t: 920, hud: hud([5, 1]) });
  add('dragon', { side: 0, type: 'infernal', count: 4, soul: true, x: 69, y: 71, t: 1900, hud: hud([5, 1]) }, true);
  add('dragon', { side: 1, type: 'elder', stolen: true, x: 69, y: 71, t: 2300, hud: hud([5, 1]) }, true);
  add('herald', { side: 1, obj: 'grubs', count: 3, x: 31, y: 29, t: 500, hud: hud([5, 1]) });
  add('herald', { side: 0, obj: 'herald', x: 31, y: 29, t: 960, hud: hud([5, 1]) });
  add('baron', { side: 1, stolen: true, x: 31, y: 29, t: 1500, hud: hud([5, 1]) }, true);
  add('tower', { side: 0, owner: 1, lane: 'mid', tier: 1, x: 60, y: 40, t: 1600, hud: hud([5, 1]) });
  add('tower', { side: 0, owner: 1, lane: 'nexus', tier: 4, x: 85, y: 17, t: 1700, hud: hud([5, 1]) });
  add('inhibitor', { side: 0, owner: 1, lane: 'mid', x: 79, y: 21, t: 1750, hud: hud([5, 1]) }, true);
  add('inhibitor', { side: 1, owner: 1, lane: 'mid', x: 79, y: 21, respawn: true, t: 2050, hud: hud([5, 1]) });
  add('nexus', { side: 0, owner: 1, x: 88, y: 12, t: 2100, hud: hud([5, 1]) }, true);
  add('match_end', { winner: 0, duration: 2100, mvp: { side: 0, slot: 2, handle: 'P02', champ: 'Emberlance', k: 5, d: 0, a: 2 },
    players: [0, 1].map(t => ROLES.map((r, i) => ({ handle: `P${t}${i}`, champ: champs[t * 5 + i], role: r, k: 1, d: 1, a: 1, gold: 9000 }))),
    graph: [[0, 0], [600, 400], [1200, -800], [2100, 5000]], t: 2100, hud: hud([5, 1]) }, true);
  add('weird_future_kind', { t: 2100 });
  return ev;
}

test('every event kind gives finite positions and a sane state', async () => {
  const V = await load();
  const st = V.createState(meta, ['Alice', 'Bob']);
  const kinds = new Set();
  for (const ev of events()) {
    const fx = V.applyEvent(st, ev);
    kinds.add(ev.kind);
    assert.ok(fx.dur > 0 && fx.dur < 5, `${ev.kind} duration`);
    for (let t = 0; t < 2; t++) for (let r = 0; r < 5; r++) {
      const p = st.target[t][r];
      assert.ok(Number.isFinite(p.x) && Number.isFinite(p.y), `${ev.kind} ${t}/${r}`);
      assert.ok(p.x >= 0 && p.x <= 100 && p.y >= 0 && p.y <= 100, `${ev.kind} ${t}/${r} off map: ${p.x},${p.y}`);
    }
    for (const f of fx.flashes) assert.ok(Number.isFinite(f.x) && Number.isFinite(f.y), `${ev.kind} flash`);
    assert.ok(Number.isFinite(st.t) && st.t >= 0);
  }
  for (const k of ['phase', 'draft_ban', 'draft_pick', 'lane', 'gank', 'kill', 'teamfight', 'dragon', 'herald', 'baron', 'tower', 'inhibitor', 'nexus', 'match_end']) assert.ok(kinds.has(k), k);
  assert.equal(st.draft.picks[0].length, 5);
  assert.equal(st.draft.picks[1].length, 5);
  assert.equal(st.draft.bans[0][0], 'Rageborn');
  assert.equal(st.line[1][2].champ, 'Pyrelord');
  assert.equal(st.mode, 'map');
  assert.deepEqual(st.dragons[0], ['infernal', 'infernal']);
  // Team 1 is on the blue side (map side 0): its towers/inhib/nexus use side 0 keys.
  assert.equal(st.towers['0-mid-1'], false);
  assert.equal(st.towers['0-nexus-4'], false);
  assert.equal(st.towers['1-mid-1'], true);
  assert.equal(st.inhibs['0-mid'], true);
  assert.equal(st.nexus[0], false);
  assert.ok(st.end && st.end.mvp.champ === 'Emberlance');
  assert.ok(st.goldPts.length >= 2 && st.goldPts.every(p => p.every(Number.isFinite)));
  assert.ok(st.feed.length > 0 && st.feed.length <= 6);
});

test('dead champions wait at the fountain and come back', async () => {
  const V = await load();
  const st = V.createState(meta, ['A', 'B']);
  V.applyEvent(st, { n: 1, kind: 'phase', data: { phase: 'laning', t: 0 } });
  V.applyEvent(st, { n: 2, kind: 'kill', data: { t: 600, x: 50, y: 50, killer: ref(0, 2), victim: ref(1, 2), assists: [] } });
  assert.equal(V.isAlive(st, 1, 2), false);
  assert.equal(st.target[1][2].alive, false);
  V.applyEvent(st, { n: 3, kind: 'lane', data: { t: 600 + V.respawnTime(600) + 1, x: 11, y: 18, lane: 'top' } });
  assert.equal(V.isAlive(st, 1, 2), true);
});

test('helpers', async () => {
  const V = await load();
  assert.equal(V.mapSide(meta, 1), 0);
  assert.equal(V.mapSide(meta, 0), 1);
  assert.equal(V.mapSide({ blue: 0 }, 0), 0);
  assert.equal(V.initials('Emberlance'), 'Em');
  assert.equal(V.fmtClock(1865), '31:05');
  const m = V.defaultMap();
  assert.equal(m.towers.length, 22);
  assert.equal(m.inhibitors.length, 6);
  for (const s of [0, 1]) for (let r = 0; r < 5; r++) { const p = V.homePos(s, r); assert.ok(p.every(Number.isFinite)); }
  assert.ok(V.TEXT.en.victory);
  assert.equal(V.TEXT.ca, undefined, 'the viewer is English only');
  assert.equal(typeof V.LolView, 'function');
  for (const k of ['isFinished', 'skipToEnd', 'togglePause', 'setSpeed', 'destroy']) assert.equal(typeof V.LolView.prototype[k], 'function', k);
});

test('captions are English and built from the data, never from ev.text', async () => {
  const V = await load();
  const st = V.createState(meta, ['Alice', 'Bob']);
  for (const ev of events()) {
    ev.text = 'Text en català que no surt';
    V.applyEvent(st, ev);
    const c = V.caption(st, ev);
    assert.ok(!/català/.test(c), ev.kind + ': ' + c);
    if (ev.kind !== 'weird_future_kind') assert.ok(c.length > 3, ev.kind + ' has a caption');
    assert.equal(st.ticker, c);
  }
  const kill = { kind: 'kill', data: { killer: ref(0, 0), victim: ref(1, 2), gold: 400, tag: 'first_blood', t: 100 } };
  assert.equal(V.caption(st, kill), 'Ironwall kills Pyrelord — First Blood (+400g)');
  assert.equal(V.caption(st, { kind: 'tower', data: { side: 1, owner: 0, lane: 'top', tier: 2 } }), 'Bob destroy the top inner tower');
  assert.equal(V.caption(st, { kind: 'dragon', data: { side: 0, type: 'ocean', soul: true } }), 'Alice claim the Ocean Soul!');
});

test('plans read in plain English and point somewhere on the map', async () => {
  const V = await load();
  assert.equal(V.planText(V.normalizePlan('macro', { objective: 'dragon', stance: 'engage' })), 'Dragon · Engage');
  assert.equal(V.planText(V.normalizePlan('macro', { objective: 'towers', stance: 'split' })), 'Push towers · Split push');
  assert.equal(V.planText(V.normalizePlan('objectives', 'grubs')), 'Take Void Grubs');
  assert.equal(V.planText(V.normalizePlan('plates', { focus: 'top', action: 'dive' })), 'Tower dive top');
  assert.equal(V.planText(V.normalizePlan('laning', { Lane: ['aggressive', 'even', 'even'], Jungle: 'gank_top' })), 'Jungler ganks top · aggressive top');
  const st = V.createState(meta, ['A', 'B']);
  V.applyEvent(st, { n: 1, kind: 'phase', data: { phase: 'laning', t: 0, choices: [{ Lane: ['even', 'even', 'even'], Jungle: 'gank_bot' }, { Lane: ['even', 'even', 'even'], Jungle: 'farm' }] } });
  assert.equal(st.mode, 'map');
  const tg = V.planTarget(st, 0);
  assert.ok(tg && tg.slot === 1 && tg.y > 80, 'jungler heads for bot lane');
  assert.equal(V.planTarget(st, 1), null);
  V.applyEvent(st, { n: 2, kind: 'phase', data: { phase: 'macro', t: 840, choices: [{ objective: 'dragon', stance: 'engage' }, { objective: 'towers', stance: 'siege' }] } });
  assert.deepEqual([V.planTarget(st, 0).x, V.planTarget(st, 0).y], [69, 71]);
  const t1 = V.planTarget(st, 1);
  assert.ok(Number.isFinite(t1.x) && Number.isFinite(t1.y));
  // Team 1 is blue, so it pushes red (side 1) structures.
  assert.ok(st.map.towers.some(t => t.side === 1 && t.x === t1.x && t.y === t1.y));
  // Team 0 groups near the dragon pit.
  for (let r = 0; r < 5; r++) assert.ok(Math.hypot(st.target[0][r].x - 69, st.target[0][r].y - 71) < 10, 'grouped at dragon');
});

test('lanes, wave fronts, kda and per-player gold', async () => {
  const V = await load();
  const m = V.defaultMap();
  assert.deepEqual(V.lanePoint(m.lanes.mid, 0), [18, 82]);
  assert.deepEqual(V.lanePoint(m.lanes.mid, 1), [82, 18]);
  assert.deepEqual(V.lanePoint(m.lanes.mid, .5), [50, 50]);
  const st = V.createState(meta, ['A', 'B']);
  assert.equal(V.waveFront(st, 'mid'), .5);
  st.towers['0-mid-1'] = false;
  assert.ok(V.waveFront(st, 'mid') < .5, 'blue lost a tower: the wave pushes towards blue');
  let n = 0;
  for (const ev of events()) { V.applyEvent(st, ev); n++; }
  // match_end carries the real stats.
  assert.deepEqual(st.kda[0][0], { k: 1, d: 1, a: 1 });
  assert.equal(st.pgold[1][4], 9000);
  assert.ok(st.marks.some(m => m.kind === 'dragon') && st.marks.some(m => m.kind === 'baron') && st.marks.some(m => m.kind === 'ace'));
  assert.equal(st.soul[0], 'infernal');
  const st2 = V.createState(meta, ['A', 'B']);
  V.applyEvent(st2, { n: 1, kind: 'phase', data: { phase: 'laning', t: 0, hud: { k: [0, 0], g: [2500, 2500] } } });
  V.applyEvent(st2, { n: 2, kind: 'kill', data: { t: 100, x: 50, y: 50, killer: ref(0, 2), victim: ref(1, 2), assists: [1], gold: 400, hud: { k: [1, 0], g: [3200, 2800] } } });
  assert.deepEqual(st2.kda[0][2], { k: 1, d: 0, a: 0 });
  assert.deepEqual(st2.kda[0][1], { k: 0, d: 0, a: 1 });
  assert.deepEqual(st2.kda[1][2], { k: 0, d: 1, a: 0 });
  const sum = st2.pgold[0].reduce((a, b) => a + b, 0);
  assert.ok(Math.abs(sum - 3200) < 1, 'player gold adds up to team gold: ' + sum);
  assert.ok(st2.pgold[0][2] > st2.pgold[0][0] + 300, 'the killer got the bounty');
});

test('action cam: easing, tweens and the tilted projection', async () => {
  const V = await load();
  assert.equal(V.easeInOut(0), 0);
  assert.equal(V.easeInOut(1), 1);
  assert.equal(V.easeInOut(.5), .5);
  assert.ok(V.easeInOut(.1) < .1 && V.easeInOut(.9) > .9, 'slow at both ends');
  assert.deepEqual(V.camAt(null, 3), V.FULL_CAM);
  const tw = { from: { x: 50, y: 50, z: 1 }, to: { x: 70, y: 70, z: 2.6 }, t0: 10, dur: 1 };
  assert.deepEqual(V.camAt(tw, 9), { x: 50, y: 50, z: 1 });
  assert.deepEqual(V.camAt(tw, 12), { x: 70, y: 70, z: 2.6 });
  const mid = V.camAt(tw, 10.5);
  assert.ok(mid.z > 1 && mid.z < 2.6 && Math.abs(mid.x - 60) < 1e-9);
  assert.equal(V.camTilt(1), 0);
  assert.equal(V.camTilt(4), 1);
  const view = { cx: 400, cy: 300, ax: 420, ay: 340, h: 600 };
  // z = 1 looking at the map centre is the flat broadcast view, untouched.
  for (const [X, Y] of [[0, 0], [400, 300], [812, 77], [33, 590]]) {
    const [x, y, k] = V.camProject({ fX: 400, fY: 300, z: 1 }, view, X, Y);
    assert.ok(Math.abs(x - X) < 1e-9 && Math.abs(y - Y) < 1e-9 && k === 1, `${X},${Y}`);
  }
  // Zoomed: the focus lands on the anchor, scaled by z, nearer = bigger, order kept.
  const cam = { fX: 250, fY: 200, z: 2.6 };
  const f = V.camProject(cam, view, 250, 200);
  assert.ok(Math.abs(f[0] - 420) < 1e-9 && Math.abs(f[1] - 340) < 1e-9 && Math.abs(f[2] - 2.6) < 1e-9);
  const near = V.camProject(cam, view, 250, 260), far = V.camProject(cam, view, 250, 140);
  assert.ok(near[2] > f[2] && far[2] < f[2], 'perspective: lower on screen is bigger');
  let prev = -Infinity;
  for (let Y = -400; Y <= 900; Y += 25) { const y = V.camProject(cam, view, 250, Y)[1]; assert.ok(y > prev, 'monotonic at ' + Y); prev = y; }
  const [hx] = V.camProject(cam, view, 260, 200);
  assert.ok(Math.abs(hx - 420 - 26) < 1e-9, 'horizontal scale = z at the focus');
});

test('action cam: which events get a shot, and how shots chain', async () => {
  const V = await load();
  const st = V.createState(meta, ['A', 'B']);
  assert.equal(V.shotFor(st, { kind: 'teamfight', data: { x: 69, y: 71, size: 'teamfight', t: 900 } }), null, 'no shots during the draft');
  V.applyEvent(st, { n: 1, kind: 'phase', data: { phase: 'laning', t: 0 } });
  const tf = V.shotFor(st, { kind: 'teamfight', data: { x: 69, y: 71, size: 'teamfight', side: 0, t: 900 } });
  assert.ok(tf && tf.prio === 3 && tf.zoom >= 2 && tf.zoom <= 3 && tf.label === 'TEAMFIGHT' && tf.where === 'the dragon pit');
  const ace = V.shotFor(st, { kind: 'kill', data: { x: 69, y: 71, killer: ref(0, 2), victim: ref(1, 0), ace: true, multi: 2, side: 0 } });
  assert.ok(ace.replay && ace.label === 'ACE' && ace.prio === 4);
  const baron = V.shotFor(st, { kind: 'baron', data: { x: 31, y: 29, side: 1, stolen: true } });
  assert.ok(baron.replay && baron.prio === 5 && baron.mon.kind === 'baron' && baron.kind === 'objective' && /STEAL/.test(baron.label));
  const drake = V.shotFor(st, { kind: 'dragon', data: { x: 69, y: 71, side: 0, type: 'ocean' } });
  assert.ok(drake.mon.kind === 'drake' && drake.mon.col === V.DRAGON_COLORS.ocean && !drake.replay);
  assert.equal(V.shotFor(st, { kind: 'dragon', data: { x: 69, y: 71, side: 0, type: 'elder' } }).mon.kind, 'elder');
  assert.equal(V.shotFor(st, { kind: 'herald', data: { x: 31, y: 29, side: 0, obj: 'grubs', count: 3 } }).mon.kind, 'grubs');
  const tw = V.shotFor(st, { kind: 'tower', data: { x: 60, y: 40, side: 0, owner: 1, lane: 'mid', tier: 1 } });
  assert.deepEqual(tw.struct, { kind: 'tower', side: V.mapSide(meta, 1), lane: 'mid', tier: 1 });
  assert.equal(tw.prio, 1);
  assert.equal(V.shotFor(st, { kind: 'nexus', data: { x: 88, y: 12, side: 0, owner: 1 } }).prio, 5);
  assert.equal(V.shotFor(st, { kind: 'inhibitor', data: { x: 79, y: 21, owner: 1, lane: 'mid', respawn: true } }), null);
  assert.equal(V.shotFor(st, { kind: 'lane', data: { x: 11, y: 18, lane: 'top', diff: 300 } }), null);
  assert.equal(V.shotFor(st, { kind: 'gank', data: { x: 82, y: 89, side: 1, success: false } }), null);
  assert.equal(V.shotFor(st, { kind: 'kill', data: { killer: ref(0, 2), victim: ref(1, 0) } }), null, 'no position, no shot');
  // A kill under the victim's own standing tower is a dive.
  const vt = st.map.towers.find(t => t.side === V.mapSide(meta, 1) && t.lane === 'mid' && t.tier === 1);
  assert.equal(V.shotFor(st, { kind: 'kill', data: { x: vt.x + 1, y: vt.y, killer: ref(0, 2), victim: ref(1, 2), multi: 1, side: 0 } }).label, 'TOWER DIVE');
  // Chaining.
  assert.equal(V.shotAction(null, null, 0), 'none');
  assert.equal(V.shotAction(null, tf, 0), 'start');
  assert.equal(V.shotAction(null, { ...tf, prio: 1 }, 2), 'skip', 'minor shots wait for a quiet spell');
  assert.equal(V.shotAction(null, { ...tf, prio: 1 }, 20), 'start');
  assert.equal(V.shotAction(tf, ace, 0), 'merge', 'same place: keep filming');
  assert.equal(V.shotAction(tf, baron, 0), 'cut', 'far away and more important: cut');
  assert.equal(V.shotAction(tf, { ...baron, x: 60, y: 52 }, 0), 'pan');
  assert.equal(V.shotAction(baron, { ...tf, x: 80, y: 80 }, 0), 'skip');
  // HP drain of a monster/structure filmed dying.
  const o = { t0: 10, dieAt: 12, hp0: .5 };
  assert.equal(V.drainHp(o, 10), .5);
  assert.equal(V.drainHp(o, 12), 0);
  assert.ok(V.drainHp(o, 11) < .5 && V.drainHp(o, 11) > 0);
  assert.equal(V.drainHp(null, 3), 1);
});

test('match_end uses the new engine fields (winner_name, kills)', async () => {
  const V = await load();
  const st = V.createState(meta, ['', 'Bob']);
  V.applyEvent(st, { n: 1, kind: 'phase', data: { phase: 'laning', t: 0, hud: { k: [3, 4] } } });
  const ev = { n: 2, kind: 'match_end', score: [1, 0], data: { winner: 0, winner_name: 'Alice', result: 'Alice GUANYEN', kills: [21, 9], duration: 1800, t: 1800 } };
  V.applyEvent(st, ev);
  assert.deepEqual(st.kills, [21, 9], 'kills, not the 1-0 match score');
  assert.equal(st.names[0], 'Alice');
  assert.equal(V.caption(st, ev), 'Alice win in 30:00 — kills 21–9');
  assert.deepEqual(V.finalKills({ stats: { kills: [2, 5] } }), [2, 5]);
  assert.equal(V.finalKills({}), null);
});

// A stub 2D context: every method is a no-op, so the real drawing code runs.
function stubCtx() {
  const grad = { addColorStop() {} };
  const fn = () => {};
  return new Proxy({}, {
    get(t, k) {
      if (k in t) return t[k];
      if (k === 'measureText') return s => ({ width: String(s).length * 7 });
      if (k === 'createLinearGradient' || k === 'createRadialGradient') return () => grad;
      return fn;
    },
    set(t, k, v) { t[k] = v; return true; },
  });
}

test('the viewer plays a whole match with the action cam and a replay, headless', async () => {
  const V = await load();
  global.requestAnimationFrame = () => 0;
  const evs = events().filter(e => e.kind !== 'weird_future_kind');
  const match = { status: 'final', meta, names: ['Alice', 'Bob'], events: evs };
  const canvas = { width: 1280, height: 720, getContext: () => stubCtx() };
  const v = new V.LolView(canvas, { source: { load: async () => match } });
  await new Promise(r => setTimeout(r, 10));
  assert.ok(v.state, 'loaded');
  const errors = [];
  let maxZ = 1, replays = 0, wasReplay = false;
  const labels = new Set(), mons = new Set();
  for (let i = 0; i < 60 * 200 && !v.isFinished(); i++) {
    v.advance(1 / 30);
    try { v.draw(); } catch (e) { errors.push(e.stack); if (errors.length > 3) break; }
    maxZ = Math.max(maxZ, v.camC.z);
    if (v.shot) { labels.add(v.shot.label); if (v.shot.mon) mons.add(v.shot.mon.kind); }
    if (v.replay && !wasReplay) replays++;
    wasReplay = !!v.replay;
  }
  assert.deepEqual(errors, []);
  assert.ok(v.isFinished(), 'reached the victory screen');
  assert.equal(v.state.mode, 'victory');
  assert.ok(maxZ > 2, 'zoomed in: ' + maxZ);
  assert.ok(labels.has('TEAMFIGHT') || labels.has('ACE'), [...labels].join(','));
  assert.ok(mons.has('baron') || mons.has('drake'), [...mons].join(','));
  assert.ok(replays >= 1, 'a replay of the ACE / steal');
  assert.equal(v.camC.z, 1, 'back to the full map at the end');
  v.destroy();
  // skipToEnd clears any shot/replay.
  const w = new V.LolView(canvas, { source: { load: async () => match } });
  await new Promise(r => setTimeout(r, 10));
  for (let i = 0; i < 900; i++) w.advance(1 / 30);
  w.skipToEnd();
  assert.ok(!w.shot && !w.replay && w.isFinished());
  w.draw();
  w.destroy();
});
