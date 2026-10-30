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
  assert.ok(V.TEXT.ca.victory && V.TEXT.en.victory);
  assert.equal(typeof V.LolView, 'function');
  for (const k of ['isFinished', 'skipToEnd', 'togglePause', 'setSpeed', 'destroy']) assert.equal(typeof V.LolView.prototype[k], 'function', k);
});
