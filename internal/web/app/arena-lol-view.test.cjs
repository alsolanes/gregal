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

test('arena director: crowd reaction intensity', async () => {
  const V = await load();
  const st = V.createState(meta, ['A', 'B']);
  V.applyEvent(st, { n: 1, kind: 'phase', data: { phase: 'laning', t: 0 } });
  const R = (kind, data) => V.crowdReaction(st, { kind, data });
  const penta = R('kill', { killer: ref(1, 3), victim: ref(0, 0), multi: 5, side: 1 });
  assert.deepEqual([penta.level, penta.team, penta.label], [1, 1, 'PENTAKILL'], 'the killer\'s fans roar');
  assert.equal(R('kill', { killer: ref(0, 2), victim: ref(1, 0), ace: true, multi: 2, side: 0 }).label, 'ACE');
  const fb = R('kill', { killer: ref(0, 1), victim: ref(1, 0), tag: 'first_blood', side: 0 });
  const plain = R('kill', { killer: ref(0, 1), victim: ref(1, 0), side: 0 });
  assert.ok(fb.level > plain.level && fb.label === 'FIRST BLOOD');
  const order = [
    R('kill', { killer: ref(0, 1), victim: ref(1, 0), side: 0 }), R('dragon', { side: 0, type: 'ocean' }), R('inhibitor', { side: 0, lane: 'mid' }),
    R('kill', { killer: ref(0, 1), victim: ref(1, 0), tag: 'first_blood', side: 0 }), R('dragon', { side: 0, type: 'ocean', soul: true }),
    R('baron', { side: 1 }), R('dragon', { side: 1, type: 'elder' }), R('nexus', { side: 0, owner: 1 }),
  ];
  for (let i = 1; i < order.length; i++) assert.ok(order[i].level >= order[i - 1].level, `${order[i].label} ≥ ${order[i - 1].label}`);
  assert.ok(R('baron', { side: 1, stolen: true }).level > R('baron', { side: 1 }).level, 'a steal is louder');
  assert.equal(R('inhibitor', { side: 0, respawn: true }).level, 0);
  assert.equal(R('lane', { lane: 'top', diff: 300 }).level, 0);
  assert.equal(R('weird', {}).level, 0);
  for (const r of order) assert.ok(r.level >= 0 && r.level <= 1);
  // Over a scene the noise rises fast, peaks at the level and settles, never above it.
  assert.equal(V.crowdHype(.8, 0, 2), 0);
  assert.ok(Math.abs(V.crowdHype(.8, .35, 2) - .8) < 1e-9);
  assert.ok(V.crowdHype(.8, 2, 2) < .8 && V.crowdHype(.8, 2, 2) > .4, 'settles but still loud');
  for (let t = 0; t < 4; t += .1) assert.ok(V.crowdHype(.8, t, 2) <= .8 + 1e-9);
  assert.equal(V.crowdHype(0, 1, 2), 0);
  // Each fan: their team's moment, everyone's moment, the rival's.
  assert.equal(V.fanHype(.9, 1, 1), .9);
  assert.ok(V.fanHype(.9, 0, 1) < .2, 'rival fans groan');
  assert.ok(V.fanHype(.9, -1, 1) > V.fanHype(.9, 0, 1) && V.fanHype(.9, -1, 1) < .9);
  assert.ok(V.fanHype(.9, 0, -1) > .7, 'a shared moment: everyone');
  assert.equal(V.fanHype(0, 1, 1), 0);
});

test('arena director: which cut, when, and the show around the match', async () => {
  const V = await load();
  const r = (level, label, team = 0) => ({ level, label, team });
  // Quick reaction cuts: only big moments, not too often, never at high speed.
  assert.equal(V.reactionCut(r(.3, 'DRAGON')), null, 'small moments: no cut');
  assert.equal(V.reactionCut(null), null);
  assert.equal(V.reactionCut(r(1, 'NEXUS')), null, 'the nexus has its own finale');
  assert.equal(V.reactionCut(r(.7, 'FIRST BLOOD'), { speed: 4 }), null, 'fast-forward: no cuts');
  assert.equal(V.reactionCut(r(.7, 'FIRST BLOOD'), { sinceCut: 10 }), null, 'not right after another cut');
  assert.ok(V.reactionCut(r(.95, 'ELDER DRAGON'), { sinceCut: 10 }), 'the biggest moments come back sooner');
  const fb = V.reactionCut(r(.7, 'FIRST BLOOD', 1), { sinceCut: 60 });
  assert.deepEqual([fb.kind, fb.team], ['crowd', 1]);
  assert.ok(fb.dur >= 1.5 && fb.dur <= 3, 'short: ' + fb.dur);
  assert.equal(V.reactionCut(r(.9, 'BARON NASHOR', 1)).kind, 'booth', 'big objective: the team celebrates in the booth');
  assert.equal(V.reactionCut(r(.85, 'DRAGON SOUL', 0)).kind, 'booth');
  assert.equal(V.reactionCut(r(.9, 'ACE', 0)).kind, 'arena', 'an ACE: the whole arena');
  for (const lv of [.5, .7, .9, 1]) { const c = V.reactionCut(r(lv, 'PENTAKILL')); assert.ok(c.dur >= 1.5 && c.dur <= 3); }
  // The cut waits for the action cam and the replay, and goes stale.
  const p = { at: 10 };
  assert.equal(V.cutReady(p, { now: 12, mode: 'map' }), 'now');
  assert.equal(V.cutReady(p, { now: 10.5, mode: 'map' }), 'wait', 'let the moment land first');
  assert.equal(V.cutReady(p, { now: 12, mode: 'map', shot: {} }), 'wait', 'never cut away from a fight on camera');
  assert.equal(V.cutReady(p, { now: 12, mode: 'map', replay: {} }), 'wait');
  assert.equal(V.cutReady(p, { now: 19, mode: 'map', shot: {} }), 'drop', 'too late');
  assert.equal(V.cutReady(p, { now: 12, mode: 'victory' }), 'drop');
  assert.equal(V.cutReady(null, { now: 12, mode: 'map' }), 'drop');
  // Draft: casters and booths between the phases.
  const ban = { kind: 'draft_ban', data: { side: 1 } }, pick = { kind: 'draft_pick', data: { side: 1 } };
  assert.equal(V.draftCut(null, ban), null, 'the first event: no cut');
  assert.equal(V.draftCut('draft_ban', ban), null, 'same phase: no cut');
  assert.deepEqual([V.draftCut('draft_ban', pick, 0).kind, V.draftCut('draft_ban', pick, 0).team, V.draftCut('draft_ban', pick, 0).label], ['booth', 1, 'PICK PHASE 1']);
  assert.equal(V.draftCut('draft_pick', ban, 1).kind, 'casters');
  assert.equal(V.draftCut('draft_ban', pick, 2).label, 'PICK PHASE 2');
  assert.equal(V.draftCut('draft_pick', { kind: 'kill', data: {} }), null);
  // Opening, ready, finale.
  const st = V.createState(meta, ['A', 'B']);
  assert.deepEqual(V.openingScenes(st).map(s => s.kind), ['arena', 'casters']);
  V.applyEvent(st, { n: 1, kind: 'phase', data: { phase: 'laning', t: 0 } });
  assert.deepEqual(V.openingScenes(st).map(s => s.kind), ['arena'], 'joining mid-game: a quick look only');
  assert.equal(V.readyScenes()[0].kind, 'arena');
  const fin = V.finaleScenes(1);
  assert.deepEqual(fin.map(s => s.kind), ['arena', 'booth', 'trophy']);
  assert.ok(fin.every(s => s.team === 1 && s.confetti && s.dur > 0));
  // Scene cameras stay inside the arena.
  for (const sc of [...V.openingScenes(V.createState(meta)), ...fin, { kind: 'arena', dur: 2 }, { kind: 'crowd', dur: 2 }])
    for (const pos of [-1, 0, 1]) for (let t = 0; t <= (sc.dur || 2); t += .25) {
      const c = V.sceneCam(sc, t, pos);
      assert.ok(c.z >= 1 && c.x - 800 / c.z >= -1 && c.x + 800 / c.z <= 1601 && c.y - 450 / c.z >= -1 && c.y + 450 / c.z <= 901, `${sc.kind} ${t} ${JSON.stringify(c)}`);
    }
  assert.ok(V.sceneCam(fin[2], 6).z > 1.5, 'the trophy: close on the stage');
  assert.equal(V.CAM_TAGS.booth, 'BOOTH');
  assert.equal(V.CAM_TAGS.casters, 'CASTERS');
  assert.equal(V.CAM_TAGS.map, 'MAP');
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
  let maxZ = 1, replays = 0, wasReplay = false, lastN = 0, overlap = 0;
  const labels = new Set(), mons = new Set(), scenes = [];
  for (let i = 0; i < 60 * 200 && !v.isFinished(); i++) {
    v.advance(1 / 30);
    try { v.draw(); } catch (e) { errors.push(e.stack); if (errors.length > 3) break; }
    maxZ = Math.max(maxZ, v.camC.z);
    if (v.shot) { labels.add(v.shot.label); if (v.shot.mon) mons.add(v.shot.mon.kind); }
    if (v.replay && !wasReplay) replays++;
    wasReplay = !!v.replay;
    if (v.scene && v.scene.n !== lastN) { lastN = v.scene.n; scenes.push(`${v.scene.kind}:${v.scene.label}`); if (v.shot || v.replay) overlap++; }
  }
  assert.deepEqual(errors, []);
  // The show: opening, casters and booths in the draft, reaction cuts, the finale.
  assert.deepEqual(scenes.slice(0, 2), ['arena:GRAND FINAL', 'casters:CHAMPION DRAFT'], scenes.join(' | '));
  assert.ok(scenes.includes('arena:GAME ON'), scenes.join(' | '));
  assert.ok(scenes.some(s => /^(crowd|arena|booth):(FIRST BLOOD|ACE|PENTAKILL|BARON|ELDER|DRAGON)/.test(s)), 'a reaction cut: ' + scenes.join(' | '));
  assert.deepEqual(scenes.slice(-3), ['arena:VICTORY', 'booth:VICTORY', 'trophy:CHAMPIONS']);
  assert.equal(overlap, 0, 'a scene never covers the action cam or a replay');
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

test('seek rebuilds the same state as playing straight through, and seek(0) restarts', async () => {
  const V = await load();
  global.requestAnimationFrame = () => 0;
  const evs = events().filter(e => e.kind !== 'weird_future_kind');
  const match = { status: 'final', meta, names: ['Alice', 'Bob'], events: evs };
  const canvas = { width: 1280, height: 720, getContext: () => stubCtx() };
  // The state right after each event is applied, playing straight through.
  const strip = st => JSON.stringify({ ...st, pos: undefined, meta: undefined, map: undefined });
  const straight = [];
  const a = new V.LolView(canvas, { source: { load: async () => match } });
  await new Promise(r => setTimeout(r, 10));
  a.setSpeed(8); // no arena scenes: straight through
  for (let i = 0; i < 20000 && !a.isFinished(); i++) { const before = a.idx; a.advance(1 / 30); if (a.idx > before) straight[a.idx - 1] = strip(a.state); }
  assert.equal(straight.length, evs.length);
  a.destroy();

  const seen = [];
  const v = new V.LolView(canvas, { source: { load: async () => match }, onEvent: e => seen.push(e.n) });
  await new Promise(r => setTimeout(r, 10));
  for (let i = 0; i < 400; i++) v.advance(1 / 30); // play a bit first, then jump around
  for (const k of [20, 5, 27, evs.length - 2, 12, 0]) {
    seen.length = 0;
    assert.equal(v.seek(k), true);
    assert.deepEqual(seen, [], 'no onEvent while rebuilding');
    assert.ok(!v.shot && !v.replay && !v.scene && !v.banner && v.acts.length === 0, 'no effects carried over');
    assert.equal(v.getProgress().index, k);
    // Everyone is snapped into place.
    for (let t = 0; t < 2; t++) for (let r = 0; r < 5; r++) { const p = v.state.pos[t][r], q = v.state.target[t][r]; assert.ok(p.x === q.x && p.y === q.y, 'snapped'); }
    v.setSpeed(8);
    for (let i = 0; i < 200 && v.idx === k; i++) v.advance(1 / 30);
    assert.equal(seen[0], evs[k].n, 'onEvent fires for the event landed on');
    assert.equal(seen.length, 1);
    assert.equal(strip(v.state), straight[k], 'same state as straight through after event ' + k);
    v.setSpeed(1);
  }
  // seek(0) is a restart: fresh state and the opening show again.
  v.seek(0);
  assert.equal(v.idx, 0); assert.equal(v.state.mode, 'draft'); assert.deepEqual(v.state.kills, [0, 0]); assert.equal(v.opened, false);
  // Past the end: clamped, finished.
  v.seek(9999);
  assert.equal(v.idx, evs.length); assert.ok(v.isFinished());
  const pr = v.getProgress();
  assert.equal(pr.total, evs.length); assert.equal(pr.live, false);
  assert.ok(pr.marks.length > 0 && pr.marks.every(m => evs[m.i].hl), 'hl marks');
  v.destroy();
});

test('seek while live: total grows and seeking past it waits at the edge', async () => {
  const V = await load();
  global.requestAnimationFrame = () => 0;
  const evs = events().filter(e => e.kind !== 'weird_future_kind');
  let upto = 15;
  const src = { load: async after => ({ status: 'live', meta, names: ['A', 'B'], events: evs.slice(0, upto).filter(e => e.n > after) }) };
  const canvas = { width: 1280, height: 720, getContext: () => stubCtx() };
  const v = new V.LolView(canvas, { source: src });
  await new Promise(r => setTimeout(r, 10));
  assert.deepEqual([v.getProgress().total, v.getProgress().live], [15, true]);
  v.seek(40);
  assert.equal(v.idx, 15, 'clamped to what has loaded');
  for (let i = 0; i < 60; i++) v.advance(1 / 30);
  assert.ok(!v.isFinished());
  upto = 25; await v.poll(); clearTimeout(v.pollTimer);
  assert.equal(v.getProgress().total, 25);
  v.setSpeed(8);
  for (let i = 0; i < 300; i++) v.advance(1 / 30);
  assert.ok(v.idx > 15, 'carries on with the new events');
  v.destroy();
});

test('playback time: event times, and elapsed only moves forward to the total', async () => {
  const V = await load();
  global.requestAnimationFrame = () => 0;
  const evs = events().filter(e => e.kind !== 'weird_future_kind');
  const times = V.eventTimes(evs);
  assert.equal(times.length, evs.length + 1);
  assert.ok(times.every((t, i) => i === 0 || t > times[i - 1]));
  assert.equal(V.eventSeconds({ kind: 'kill', hl: true }), 1.7 + .6);
  const canvas = { width: 1280, height: 720, getContext: () => stubCtx() };
  const v = new V.LolView(canvas, { source: { load: async () => ({ status: 'final', meta, names: ['A', 'B'], events: evs }) } });
  await new Promise(r => setTimeout(r, 10));
  let t = 0, last = -1;
  for (let i = 0; i < 60 * 300 && !v.isFinished(); i++) {
    v.advance(1 / 30); t += 1 / 30;
    const p = v.getProgress();
    assert.ok(p.elapsed >= last - 1e-9, 'elapsed never goes back while playing');
    last = p.elapsed;
  }
  const pr = v.getProgress();
  assert.equal(pr.elapsed, pr.duration);
  // The arena show (opening, draft cuts, reaction cuts, finale) is in the estimate; action-cam replays aren't.
  assert.ok(Math.abs(t - pr.duration) / pr.duration < .15, `estimate ${pr.duration.toFixed(1)} s vs played ${t.toFixed(1)} s`);
  assert.equal(pr.times[0] > 0, true, 'the opening comes before the first event');
  v.destroy();
});
