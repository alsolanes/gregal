// League of Legends viewer for the Arena (internal/arena/lol), drawn like an
// esports broadcast: a punchy draft, then an isometric Summoner's Rift with
// champion tokens, minion waves, fights, objectives and each team's plan, a
// broadcast top bar, a stats panel (rosters, objectives, gold graph) and a
// victory screen. Everything on screen is English, whatever the page language.
//
// Pure helpers (createState, applyEvent, layout, caption…) don't touch the DOM
// and are tested with node (arena-lol-view.test.cjs).

import { ArenaViewBase, lerp, clamp, easeOut, rng, hexToRgb as hexRgb, viewDpr, elapsedAt, progressMarks } from './arena-view-base.js';
export { elapsedAt, progressMarks };

// Progress-bar marks (progressMarks) and playback seconds (elapsedAt) live in arena-view-base.js.
const TAU = Math.PI * 2;
const FONT = '"Segoe UI", "Helvetica Neue", Arial, system-ui, sans-serif';
const SQ = 0.64; // vertical squash of the tilted map

export const TEXT = {
  en: {
    draft: 'CHAMPION DRAFT', pregame: 'Teams are preparing the draft…', waiting: 'Waiting for the next decision…',
    live: 'LIVE', final: 'FINAL', victory: 'VICTORY', mvp: 'MVP', duration: 'Duration', kills: 'Kills', towers: 'Towers', dragons: 'Dragons',
    gold: 'Gold', barons: 'Barons', heralds: 'Heralds', grubs: 'Grubs', firstBlood: 'FIRST BLOOD', ace: 'ACE', shutdown: 'SHUTDOWN',
    dragonStolen: 'DRAGON STOLEN!', baronStolen: 'BARON STOLEN!', heraldStolen: 'HERALD STOLEN!',
    soul: t => `${t.toUpperCase()} SOUL`, drake: t => `${t.toUpperCase()} DRAKE`, elder: 'ELDER DRAGON', baron: 'BARON NASHOR', herald: 'RIFT HERALD', grubsBanner: 'VOID GRUBS',
    inhib: 'INHIBITOR DESTROYED', nexus: 'NEXUS DESTROYED', comeback: 'COMEBACK!', golddiff: 'GOLD DIFFERENCE', ban: 'BAN', pick: 'PICK', laning: 'LANING PHASE',
    roles: { top: 'Top', jungle: 'Jungle', mid: 'Mid', adc: 'Bot', support: 'Support' },
    roleShort: { top: 'TOP', jungle: 'JNG', mid: 'MID', adc: 'BOT', support: 'SUP' },
    side: ['BLUE SIDE', 'RED SIDE'], objectives: 'OBJECTIVES', lockedIn: 'LOCKED IN', plan: 'PLAN',
    actionCam: 'ACTION CAM', replay: 'REPLAY', slowMo: 'SLOW MOTION', steals: 'Steals', grubsStat: 'Void Grubs', elders: 'Elders',
    mapTag: 'MAP', arena: 'GREGAL ARENA', grandFinal: 'GRAND FINAL', champions: 'CHAMPIONS',
  },
};
const T = TEXT.en;

const OBJ_NAMES = { dragon: 'Dragon', elder: 'Elder Dragon', baron: 'Baron', herald: 'Rift Herald', grubs: 'Void Grubs', towers: 'Push towers', farm: 'Farm & scale' };
const STANCE_NAMES = { engage: 'Engage', siege: 'Siege', pick: 'Pick-offs', split: 'Split push', protect: 'Protect carry' };
const TIER_NAMES = { 1: 'outer', 2: 'inner', 3: 'base', 4: 'nexus', 5: 'nexus' };
const cap = s => String(s || '').charAt(0).toUpperCase() + String(s || '').slice(1);

export const ROLES = ['top', 'jungle', 'mid', 'adc', 'support'];
const ROLE_LANE = ['top', null, 'mid', 'bot', 'bot'];
const GOLD_SHARE = [0.21, 0.17, 0.22, 0.25, 0.15];
export const CLASS_COLORS = { tank: '#7d8ea3', bruiser: '#c8692c', assassin: '#8e44ad', mage: '#5161d6', marksman: '#d8a020', enchanter: '#27ae78', engage: '#13958a' };
export const DRAGON_COLORS = { infernal: '#ff5a36', mountain: '#b0874f', ocean: '#2ec4c4', cloud: '#d6e4f0', hextech: '#4fd8ff', chemtech: '#8ef05a', elder: '#f5e17a' };
const SIDE_COLORS = [['#2f7df6', '#9cc8ff'], ['#e8414e', '#ffb3b8']];
const FOUNTAIN = [[5, 95], [95, 5]];
const JUNGLE = [[30, 60], [70, 40]];
const GOLDC = '#f2c55c';

// Default map (same as Meta().map in the engine).
export function defaultMap() {
  const blue = [
    [[9, 30], [11, 52], [10, 70], [10, 77]],
    [[40, 60], [34, 66], [26, 74], [21, 79]],
    [[70, 91], [48, 89], [30, 90], [23, 90]],
  ];
  const lanes = ['top', 'mid', 'bot'];
  const towers = [], inhibitors = [], nexus = [];
  for (let side = 0; side < 2; side++) {
    for (let l = 0; l < 3; l++) {
      const src = side === 0 ? blue[l] : blue[2 - l].map(p => [100 - p[0], 100 - p[1]]);
      for (let k = 0; k < 3; k++) towers.push({ side, lane: lanes[l], tier: k + 1, x: src[k][0], y: src[k][1] });
      inhibitors.push({ side, lane: lanes[l], x: src[3][0], y: src[3][1] });
    }
    [[15, 83], [17, 85]].forEach((p, k) => { const q = side === 0 ? p : [100 - p[0], 100 - p[1]]; towers.push({ side, lane: 'nexus', tier: 4 + k, x: q[0], y: q[1] }); });
    nexus.push(side === 0 ? { side, x: 12, y: 88 } : { side, x: 88, y: 12 });
  }
  return {
    towers, inhibitors, nexus, dragon: { x: 69, y: 71 }, baron: { x: 31, y: 29 },
    lanes: { top: [[10, 82], [9, 14], [14, 9], [82, 10]], mid: [[18, 82], [82, 18]], bot: [[18, 90], [86, 91], [91, 86], [90, 18]] },
    river: [[12, 12], [31, 33], [50, 50], [69, 67], [88, 88]],
  };
}

// ---- pure helpers ----

export function initials(name) {
  const s = String(name || '?').replace(/[^A-Za-z]/g, '');
  if (!s) return '?';
  const caps = s.match(/[A-Z]/g) || [];
  return (caps.length >= 2 ? caps[0] + caps[1] : s.slice(0, 2)).replace(/^(.)(.)$/, (m, a, b) => a.toUpperCase() + b.toLowerCase());
}

function hash(s) { let h = 2166136261; for (const c of String(s)) { h ^= c.charCodeAt(0); h = Math.imul(h, 16777619); } return h >>> 0; }
const hexToRgb = h => hexRgb(h);
function shade(h, f) { const [r, g, b] = hexToRgb(h); const k = v => clamp(Math.round(f < 0 ? v * (1 + f) : v + (255 - v) * f), 0, 255); return `rgb(${k(r)},${k(g)},${k(b)})`; }
function rgba(h, a) { const [r, g, b] = hexToRgb(h); return `rgba(${r},${g},${b},${a})`; }
function segDist(px, py, a, b) {
  const dx = b[0] - a[0], dy = b[1] - a[1], l2 = dx * dx + dy * dy || 1;
  const t = clamp(((px - a[0]) * dx + (py - a[1]) * dy) / l2, 0, 1);
  return Math.hypot(px - a[0] - t * dx, py - a[1] - t * dy);
}
function polyDist(px, py, pts) { let m = Infinity; for (let i = 1; i < pts.length; i++) m = Math.min(m, segDist(px, py, pts[i - 1], pts[i])); return m; }

// Team (0/1) → map side (0 blue, 1 red).
export function mapSide(meta, team) {
  const t = meta && meta.teams && meta.teams[team];
  if (t && (t.side === 'blue' || t.side === 'red')) return t.side === 'blue' ? 0 : 1;
  if (meta && typeof meta.blue === 'number') return meta.blue === team ? 0 : 1;
  return team === 0 ? 0 : 1;
}
export function teamColor(meta, team) {
  const t = meta && meta.teams && meta.teams[team];
  return (t && t.color) || SIDE_COLORS[mapSide(meta, team)][0];
}
export function blueTeam(meta) { return mapSide(meta, 0) === 0 ? 0 : 1; }

export function champInfo(meta, name) {
  const list = (meta && meta.champions) || [];
  return list.find(c => c.name === name) || { name: name || '?', class: 'bruiser', roles: [], curve: 'mid', dmg: 'AD' };
}

export function fmtClock(sec) { const s = Math.max(0, Math.floor(sec || 0)); return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`; }
export function fmtGold(g) { return `${(Math.max(0, g || 0) / 1000).toFixed(1)}k`; }

// Point at fraction s (0 = blue base, 1 = red base) along a lane polyline.
export function lanePoint(path, s) {
  if (!path || !path.length) return [50, 50];
  if (path.length === 1) return path[0].slice();
  const segs = []; let total = 0;
  for (let i = 1; i < path.length; i++) { const l = Math.hypot(path[i][0] - path[i - 1][0], path[i][1] - path[i - 1][1]); segs.push(l); total += l; }
  let d = clamp(s, 0, 1) * total;
  for (let i = 0; i < segs.length; i++) {
    if (d <= segs[i] || i === segs.length - 1) { const k = segs[i] ? clamp(d / segs[i], 0, 1) : 0; return [lerp(path[i][0], path[i + 1][0], k), lerp(path[i][1], path[i + 1][1], k)]; }
    d -= segs[i];
  }
  return path[path.length - 1].slice();
}

function lanes(state) { return (state.map && state.map.lanes) || defaultMap().lanes; }

// Where the minion waves meet in a lane (0 = at the blue base, 1 = at the red base).
export function waveFront(state, lane) {
  const lost = side => (state.map.towers || []).filter(t => t.side === side && t.lane === lane && state.towers[`${t.side}-${t.lane}-${t.tier}`] === false).length;
  let f = 0.5 - lost(0) * 0.11 + lost(1) * 0.11;
  f += clamp((state.laneLead[lane] || 0) / 4000, -0.1, 0.1);
  if (state.inhibs[`0-${lane}`] === false) f -= 0.08;
  if (state.inhibs[`1-${lane}`] === false) f += 0.08;
  return clamp(f, 0.12, 0.88);
}

// Position of each role in the laning phase (blue side); red is the transpose.
const LANE_POS = [[10, 27], [27, 60], [43, 57], [74, 91], [71, 88]];
const tr = p => [p[1], p[0]];
export function homePos(side, role) { const p = LANE_POS[role] || [50, 50]; return side === 0 ? p.slice() : tr(p); }

function lanerPos(state, side, r) {
  const lane = ROLE_LANE[r];
  if (!lane) return JUNGLE[side].slice();
  const f = waveFront(state, lane), dir = side === 0 ? -1 : 1;
  const p = lanePoint(lanes(state)[lane], f + dir * (0.045 + (r === 4 ? 0.025 : 0)));
  return r === 4 ? [p[0] + 1.5, p[1] - 1.5] : p;
}

export function respawnTime(t) { return 8 + (t / 60) * 1.15; }

export function eventDuration(ev) {
  const d = ev && ev.data || {};
  switch (ev && ev.kind) {
    case 'phase': return d.phase === 'macro' ? 2.6 : d.phase === 'draft' ? 0.6 : d.phase === 'draft_done' ? 1.2 : d.phase === 'laning' ? 2.4 : 2.0;
    // The draft moves briskly at 1× (it used to drag on before the map).
    case 'draft_ban': return 0.55;
    case 'draft_pick': return 0.85;
    case 'lane': return 1.0;
    case 'gank': return 1.3;
    case 'kill': return ev.hl ? 1.7 : 1.0;
    case 'teamfight': return 1.4;
    case 'dragon': case 'baron': case 'herald': return ev.hl ? 2.6 : 1.8;
    case 'tower': return ev.hl ? 1.5 : 1.2;
    case 'inhibitor': return d.respawn ? 1 : 2.0;
    case 'nexus': return 3.0;
    case 'match_end': return 1.5;
  }
  return 1;
}

// Seconds an event holds the screen at 1x (its duration, plus the action-cam
// cut on the map).
const SHOT_KINDS = new Set(['gank', 'kill', 'teamfight', 'dragon', 'baron', 'herald', 'tower', 'inhibitor', 'nexus']);
export function eventSeconds(ev) { return eventDuration(ev) + (SHOT_KINDS.has(ev && ev.kind) ? .6 : 0); }
const sceneSecs = list => (list || []).reduce((a, s) => a + (s.dur || 0), 0);
// The playback start time at 1x of every event, plus the total: each event's
// own time and the arena show around it as the viewer stages it (the opening,
// the cuts between draft phases, the teams ready, the crowd's reaction cuts and
// the finale; with meta, the state is replayed to know the mode). Replays of
// the action cam aren't counted (they depend on what the camera filmed).
export function eventTimes(list, meta, names) {
  list = list || [];
  const out = [];
  const st = meta ? createState(meta, names) : null;
  let t = 0, prevDraft = null, cuts = 0, ready = false, lastCut = -99;
  list.forEach((ev, i) => {
    if (st && i === 0 && st.mode === 'draft') t += sceneSecs(openingScenes(st));
    if (st && st.mode === 'draft') {
      if (ev.kind === 'draft_ban' || ev.kind === 'draft_pick') { const c = draftCut(prevDraft, ev, cuts); prevDraft = ev.kind; if (c) { cuts++; t += c.dur; } }
      else if (st.draft.done && !ready && !(ev.kind === 'phase' && ['draft', 'draft_done'].includes((ev.data || {}).phase))) { ready = true; t += sceneSecs(readyScenes(st)); }
    }
    out.push(t);
    t += eventSeconds(ev);
    if (!st) return;
    applyEvent(st, ev);
    if (st.mode === 'map') {
      const c = reactionCut(crowdReaction(st, ev), { sinceCut: t - lastCut, speed: 1 });
      if (c) { t += c.dur; lastCut = t; }
    }
    if (ev.kind === 'match_end') t += sceneSecs(finaleScenes((ev.data || {}).winner));
  });
  out.push(t);
  return out;
}

export function createState(meta, names) {
  meta = meta || {};
  const map = meta.map || defaultMap();
  if (!map.lanes) map.lanes = defaultMap().lanes;
  const state = {
    meta, map, names: names || (meta.teams || []).map(t => t.name) || ['', ''],
    mode: 'draft', t: 0, phase: '', clockFrom: 0, clockTo: 0,
    draft: { bans: [[], []], picks: [[], []], step: '', active: null, done: false },
    line: [[], []], kills: [0, 0], towersK: [0, 0], gold: [2500, 2500], goldPts: [[0, 0]],
    dragons: [[], []], barons: [0, 0], grubs: [0, 0], heralds: [0, 0], elders: [0, 0], soul: [null, null],
    towers: {}, inhibs: {}, inhibDown: {}, nexus: [true, true], laneLead: {},
    dead: [[0, 0, 0, 0, 0], [0, 0, 0, 0, 0]], feed: [], plan: [null, null], planT: 0, buff: [{}, {}],
    kda: [0, 1].map(() => ROLES.map(() => ({ k: 0, d: 0, a: 0 }))), pgold: [[500, 500, 500, 500, 500], [500, 500, 500, 500, 500]],
    marks: [], obj: { dragonNext: 300, elderNext: 0, baronNext: 1200, heraldTaken: false, grubsTaken: false, last: {} },
    pos: [[], []], target: [[], []], end: null, ticker: '',
  };
  for (const tw of map.towers || []) state.towers[`${tw.side}-${tw.lane}-${tw.tier}`] = true;
  for (const ih of map.inhibitors || []) state.inhibs[`${ih.side}-${ih.lane}`] = true;
  for (let team = 0; team < 2; team++) {
    const side = mapSide(meta, team);
    const pl = (meta.teams && meta.teams[team] && meta.teams[team].players) || [];
    for (let r = 0; r < 5; r++) {
      const p = pl[r] || {};
      state.line[team][r] = { role: ROLES[r], handle: p.handle || ROLES[r], champ: p.champ || '' };
      const f = FOUNTAIN[side];
      state.pos[team][r] = { x: f[0], y: f[1] };
      state.target[team][r] = { x: f[0], y: f[1], alive: true };
    }
  }
  return state;
}

export function isAlive(state, team, slot) { return (state.dead[team][slot] || 0) <= state.t; }

function teamName(state, team) { const n = state.names || []; return n[team] || (state.meta.teams && state.meta.teams[team] && state.meta.teams[team].name) || (mapSide(state.meta, team) === 0 ? 'Blue' : 'Red'); }

// Normalise each phase's per-team choice into a plan object.
export function normalizePlan(phase, c) {
  if (c == null) return null;
  switch (phase) {
    case 'laning': return { kind: 'laning', lanes: Array.isArray(c.Lane || c.lane) ? (c.Lane || c.lane) : [], jungle: c.Jungle || c.jungle || 'farm' };
    case 'objectives': return { kind: 'objectives', objective: typeof c === 'string' ? c : c.objective || 'farm' };
    case 'plates': return { kind: 'plates', focus: c.focus || 'mid', action: c.action || 'farm' };
    case 'macro': return { kind: 'macro', objective: c.objective || 'towers', stance: c.stance || 'engage' };
  }
  return null;
}

// A plan in plain English.
export function planText(plan) {
  if (!plan) return '';
  switch (plan.kind) {
    case 'laning': {
      const j = plan.jungle || 'farm';
      const jt = j.startsWith('gank_') ? `Jungler ganks ${j.slice(5)}` : j === 'invade' ? 'Invade enemy jungle' : 'Jungler farms';
      const aggro = ['top', 'mid', 'bot'].filter((l, i) => plan.lanes[i] === 'aggressive');
      return aggro.length ? `${jt} · aggressive ${aggro.join('+')}` : `Laning · ${jt}`;
    }
    case 'objectives': return plan.objective === 'farm' ? 'Farm lanes' : `Take ${OBJ_NAMES[plan.objective] || cap(plan.objective)}`;
    case 'plates': return { plates: `Turret plates ${plan.focus}`, dive: `Tower dive ${plan.focus}`, roam: `Roam to ${plan.focus}`, farm: 'Farm & scale' }[plan.action] || cap(plan.action);
    case 'macro': return `${OBJ_NAMES[plan.objective] || cap(plan.objective)} · ${STANCE_NAMES[plan.stance] || cap(plan.stance)}`;
  }
  return '';
}

// Next standing enemy structure in a lane (towers, then inhibitor, then nexus).
function nextStructure(state, side, lane) {
  const m = state.map;
  const tw = (m.towers || []).filter(t => t.side === side && t.lane === lane && state.towers[`${t.side}-${t.lane}-${t.tier}`] !== false).sort((a, b) => a.tier - b.tier)[0];
  if (tw) return { x: tw.x, y: tw.y, lane };
  const ih = (m.inhibitors || []).find(i => i.side === side && i.lane === lane);
  if (ih && state.inhibs[`${side}-${lane}`] !== false) return { x: ih.x, y: ih.y, lane };
  const nt = (m.towers || []).filter(t => t.side === side && t.lane === 'nexus' && state.towers[`${t.side}-nexus-${t.tier}`] !== false)[0];
  if (nt) return { x: nt.x, y: nt.y, lane };
  const nx = (m.nexus || [])[side] || { x: side ? 88 : 12, y: side ? 12 : 88 };
  return { x: nx.x, y: nx.y, lane };
}

// Where a team is heading for its current plan (null when it just farms).
export function planTarget(state, team) {
  const plan = state.plan[team]; if (!plan) return null;
  const side = mapSide(state.meta, team), enemy = 1 - side, m = state.map;
  const pit = o => (o === 'dragon' || o === 'elder') ? { x: m.dragon.x, y: m.dragon.y } : (o === 'baron' || o === 'herald' || o === 'grubs') ? { x: m.baron.x, y: m.baron.y } : null;
  switch (plan.kind) {
    case 'laning': {
      const j = plan.jungle || 'farm';
      if (j.startsWith('gank_')) { const lane = j.slice(5); if (!lanes(state)[lane]) return null; const p = lanePoint(lanes(state)[lane], waveFront(state, lane)); return { x: p[0], y: p[1], slot: 1 }; }
      if (j === 'invade') return { x: JUNGLE[enemy][0], y: JUNGLE[enemy][1], slot: 1 };
      return null;
    }
    case 'objectives': return pit(plan.objective);
    case 'plates': return plan.action === 'farm' ? null : { ...nextStructure(state, enemy, plan.focus || 'mid'), slot: plan.action === 'roam' ? 2 : undefined };
    case 'macro': {
      const p = pit(plan.objective);
      if (p) return p;
      if (plan.objective === 'towers') {
        let best = null;
        for (const lane of ['mid', 'bot', 'top']) {
          const n = (state.map.towers || []).filter(t => t.side === enemy && t.lane === lane && state.towers[`${t.side}-${t.lane}-${t.tier}`] !== false).length;
          if (!best || n < best.n) best = { lane, n };
        }
        return nextStructure(state, enemy, best.lane);
      }
      return null;
    }
  }
  return null;
}

const RING = [[0, 0], [2.6, -1.4], [-2.6, 1.4], [1.6, 2.4], [-1.6, -2.4]];

// layout: where each champion should be for this event.
export function layout(state, ev) {
  const d = (ev && ev.data) || {};
  const out = [[], []];
  for (let team = 0; team < 2; team++) {
    const side = mapSide(state.meta, team), plan = state.plan[team], tgt = planTarget(state, team);
    const f = FOUNTAIN[side];
    const toward = (p, k) => { const dx = f[0] - p.x, dy = f[1] - p.y, l = Math.hypot(dx, dy) || 1; return [p.x + dx / l * k, p.y + dy / l * k]; };
    for (let r = 0; r < 5; r++) {
      let p = state.mode === 'draft' ? f.slice() : lanerPos(state, side, r);
      if (plan && tgt && state.mode !== 'draft') {
        const grp = (k, sc = 1) => { const q = toward(tgt, k); return [q[0] + RING[r][0] * sc, q[1] + RING[r][1] * sc]; };
        switch (plan.kind) {
          case 'laning': if (r === tgt.slot) p = grp(2.5, .3); break;
          case 'objectives': if (r === 1 || r === 2 || r === 4) p = grp(3.5); break;
          case 'plates':
            if (plan.action === 'roam') { if (r === 2) p = grp(4, .4); }
            else if (r === 1 || r === 4 || ROLE_LANE[r] === plan.focus) p = grp(plan.action === 'dive' ? 1.5 : 4.5, .8);
            break;
          case 'macro': {
            const st = plan.stance;
            if (st === 'split' && r === 0) {
              const lane = tgt.y < 50 && tgt.x < 50 ? 'bot' : 'top';
              p = lanePoint(lanes(state)[lane], clamp(waveFront(state, lane) + (side === 0 ? 0.12 : -0.12), 0.05, 0.95));
            } else p = grp(st === 'siege' ? 5 : 3.5, st === 'pick' ? 2.2 : st === 'protect' ? .7 : 1);
          }
        }
      }
      out[team][r] = { x: p[0], y: p[1], alive: true };
    }
  }
  const near = (team, r, x, y, k = 3.2) => {
    const side = mapSide(state.meta, team), f = FOUNTAIN[side];
    const dx = f[0] - x, dy = f[1] - y, len = Math.hypot(dx, dy) || 1;
    out[team][r] = { x: x + dx / len * k + RING[r][0] * .9, y: y + dy / len * k + RING[r][1] * .9, alive: true };
  };
  const x = Number.isFinite(d.x) ? d.x : null, y = Number.isFinite(d.y) ? d.y : null;
  if (x !== null && y !== null && ev) {
    switch (ev.kind) {
      case 'teamfight': for (let t = 0; t < 2; t++) for (let r = 0; r < 5; r++) near(t, r, x, y, 3.5); break;
      case 'kill': {
        const k = d.killer || {}, v = d.victim || {};
        if (k.side === 0 || k.side === 1) { near(k.side, k.slot | 0, x, y, 2); (d.assists || []).forEach(a => near(k.side, a | 0, x, y, 4)); }
        if (v.side === 0 || v.side === 1) near(v.side, v.slot | 0, x, y, 0.5);
        break;
      }
      case 'dragon': case 'baron': case 'herald': case 'tower': case 'inhibitor': case 'nexus': {
        const team = d.side === 1 ? 1 : 0;
        if (!d.respawn) for (let r = 0; r < 5; r++) near(team, r, x, y, ev.kind === 'tower' || ev.kind === 'inhibitor' || ev.kind === 'nexus' ? 4.5 : 3.5);
        if (d.stolen) near(1 - team, 1, x, y, 2);
        break;
      }
      case 'gank': {
        const team = d.side === 1 ? 1 : 0;
        near(team, 1, x, y, 1.5);
        break;
      }
    }
  }
  // The dead wait at the fountain.
  for (let team = 0; team < 2; team++) {
    const f = FOUNTAIN[mapSide(state.meta, team)];
    for (let r = 0; r < 5; r++) {
      if (!isAlive(state, team, r)) out[team][r] = { x: f[0] + RING[r][0] * .6, y: f[1] + RING[r][1] * .6, alive: false };
      out[team][r].x = clamp(out[team][r].x, 1, 99); out[team][r].y = clamp(out[team][r].y, 1, 99);
    }
  }
  return out;
}

// Where something happened, in words.
export function placeName(state, x, y) {
  const m = state.map;
  if (Math.hypot(x - m.dragon.x, y - m.dragon.y) < 9) return 'the dragon pit';
  if (Math.hypot(x - m.baron.x, y - m.baron.y) < 9) return 'the baron pit';
  for (const lane of ['top', 'mid', 'bot']) if (polyDist(x, y, lanes(state)[lane]) < 4) return `${lane} lane`;
  if (polyDist(x, y, m.river || []) < 6) return 'the river';
  return 'the jungle';
}

// Final kill count of a match_end event (the match score is 1-0 now; kills travel apart).
export function finalKills(d) {
  const k = d && (Array.isArray(d.kills) ? d.kills : d.stats && Array.isArray(d.stats.kills) ? d.stats.kills : null);
  return k && k.length === 2 && k.every(Number.isFinite) ? k.slice() : null;
}

const MULTI = { 2: 'DOUBLE KILL', 3: 'TRIPLE KILL', 4: 'QUADRA KILL', 5: 'PENTAKILL' };

// English caption for any event (the server text may be in another language).
export function caption(state, ev) {
  const d = (ev && ev.data) || {};
  const nm = t => teamName(state, t === 1 ? 1 : 0);
  const team = d.side === 1 ? 1 : 0;
  switch (ev && ev.kind) {
    case 'phase':
      switch (d.phase) {
        case 'draft': return 'The draft begins';
        case 'draft_done': return 'Draft complete — teams locked in';
        case 'laning': return 'Minions have spawned — laning phase';
        case 'objectives': return '08:00 — first neutral objectives';
        case 'plates': return '12:00 — turret plates and roams';
        case 'macro': return `${fmtClock(d.t)} — new plans: ${[0, 1].map(t => `${nm(t)} → ${planText(state.plan[t]) || '?'}`).join('  |  ')}`;
      }
      return ev.text || '';
    case 'draft_ban': return `${nm(team)} ban ${d.champ}`;
    case 'draft_pick': return `${nm(team)} pick ${d.champ}${d.role ? ` (${T.roles[d.role] || d.role})` : ''}`;
    case 'kill': {
      const k = d.killer || {}, v = d.victim || {};
      let s = `${k.champ || '?'} kills ${v.champ || '?'}`;
      if (d.tag === 'first_blood') s += ' — First Blood';
      if (d.multi >= 2) s += ` — ${cap((MULTI[Math.min(5, d.multi)] || '').toLowerCase())}`;
      if (d.shutdown) s += ' — Shutdown';
      if (d.ace) s += ' — ACE';
      return d.gold ? `${s} (+${d.gold}g)` : s;
    }
    case 'gank':
      if (d.invade) return `${nm(team)} invade and steal enemy camps`;
      return `${(d.jungler && d.jungler.champ) || nm(team)} ganks ${d.lane || ''}${d.success === false ? ' — escaped' : ''}`;
    case 'lane':
      if (Number.isFinite(d.plates)) return `${nm(team)} take ${d.plates}g of turret plates ${d.lane || ''}`;
      return `${cap(d.lane)} lane: ${nm(team)} ahead by ${Math.abs(d.diff || 0)}g`;
    case 'teamfight': {
      const where = Number.isFinite(d.x) ? placeName(state, d.x, d.y) : 'the map';
      return `${d.size === 'teamfight' ? 'Teamfight' : 'Skirmish'} at ${where} — ${nm(d.winner)} win it`;
    }
    case 'dragon':
      if (d.type === 'elder') return `${nm(team)} ${d.stolen ? 'steal' : 'slay'} the Elder Dragon`;
      if (d.soul) return `${nm(team)} claim the ${cap(d.type)} Soul!`;
      return `${nm(team)} ${d.stolen ? 'steal' : 'slay'} the ${cap(d.type)} Drake (${d.count || 1})`;
    case 'baron': return `${nm(team)} ${d.stolen ? 'steal' : 'slay'} Baron Nashor`;
    case 'herald': return d.obj === 'grubs' ? `${nm(team)} take ${d.count || 3} Void Grubs` : `${nm(team)} ${d.stolen ? 'steal' : 'take'} the Rift Herald`;
    case 'tower': return d.lane === 'nexus' ? `${nm(team)} destroy a nexus tower` : `${nm(team)} destroy the ${d.lane} ${TIER_NAMES[d.tier] || ''} tower`;
    case 'inhibitor': return d.respawn ? `${nm(d.owner)} ${d.lane} inhibitor respawns` : `${nm(team)} destroy the ${d.lane} inhibitor!`;
    case 'nexus': return `${nm(team)} destroy the Nexus!`;
    case 'match_end': {
      const w = d.winner === 1 ? 1 : 0, k = finalKills(d);
      return `${d.winner_name || nm(w)} win in ${fmtClock(d.duration || state.t)}${k ? ` — kills ${k[w]}–${k[1 - w]}` : ''}`;
    }
  }
  return '';
}

// applyEvent: applies an event to the state and returns what to animate.
export function applyEvent(state, ev) {
  const d = (ev && ev.data) || {};
  const fx = { banner: null, flashes: [], popups: [], skulls: [], hits: [], clash: null, focus: null, dur: eventDuration(ev), planChange: false };
  state.clockFrom = state.t;
  if (Number.isFinite(d.t) && d.t >= state.t) state.t = d.t;
  state.clockTo = state.t;
  const ref = r => r && (r.side === 0 || r.side === 1) && r.slot >= 0 && r.slot < 5;
  // Gold explicitly earned in this event (kills), the rest of the team income goes by role.
  const extra = [[0, 0, 0, 0, 0], [0, 0, 0, 0, 0]];
  if (ev.kind === 'kill' && ref(d.killer)) {
    extra[d.killer.side][d.killer.slot] += d.gold || 300;
    const as = (d.assists || []).filter(a => a >= 0 && a < 5);
    as.forEach(a => { extra[d.killer.side][a] += 150 / as.length; });
  }
  if (d.hud) {
    if (Array.isArray(d.hud.k)) state.kills = d.hud.k.slice();
    if (Array.isArray(d.hud.tw)) state.towersK = d.hud.tw.slice();
    if (Array.isArray(d.hud.g)) {
      for (let t = 0; t < 2; t++) {
        const delta = (d.hud.g[t] || 0) - (state.gold[t] || 0);
        if (delta <= 0) continue;
        const ex = extra[t].reduce((a, b) => a + b, 0), scale = ex > delta ? delta / ex : 1, rest = Math.max(0, delta - ex);
        for (let r = 0; r < 5; r++) state.pgold[t][r] += extra[t][r] * scale + rest * GOLD_SHARE[r];
      }
      state.gold = d.hud.g.slice();
      const last = state.goldPts[state.goldPts.length - 1]; const pt = [state.t, state.gold[0] - state.gold[1]];
      if (!last || last[0] !== pt[0]) state.goldPts.push(pt); else last[1] = pt[1];
    }
    if (Array.isArray(d.hud.bn)) state.barons = d.hud.bn.slice();
    if (Array.isArray(d.hud.gr)) state.grubs = d.hud.gr.slice();
  } else if (Array.isArray(ev.score) && ev.kind !== 'match_end') state.kills = ev.score.slice();
  const team = d.side === 1 ? 1 : 0;
  const color = teamColor(state.meta, team);
  const banner = (text, c = color, big = true, sub = '') => { fx.banner = { text, color: c, big, sub }; };
  const mark = (kind, c) => state.marks.push({ t: state.t, kind, team, color: c || color });
  if (state.mode === 'draft' && !['phase', 'draft_ban', 'draft_pick'].includes(ev.kind)) state.mode = 'map';
  switch (ev.kind) {
    case 'phase':
      state.phase = d.phase || '';
      if (d.phase === 'draft') state.mode = 'draft';
      if (d.phase === 'draft_done') state.draft.done = true;
      if (Array.isArray(d.lineups)) d.lineups.forEach((l, t) => (l || []).forEach((p, r) => { if (state.line[t] && p) state.line[t][r] = { role: p.role, handle: p.handle, champ: p.champ }; }));
      if (Array.isArray(d.choices) && ['laning', 'objectives', 'plates', 'macro'].includes(d.phase)) {
        state.plan = [0, 1].map(t => normalizePlan(d.phase, d.choices[t]));
        state.planT = state.t; fx.planChange = true;
      }
      if (d.phase === 'laning') { state.mode = 'map'; banner(T.laning, '#d8b25a', true, 'Minions have spawned'); }
      if (d.phase === 'objectives') banner('FIRST OBJECTIVES', '#d8b25a', false);
      if (d.phase === 'plates') banner('TURRET PLATES', '#d8b25a', false);
      if (d.phase === 'macro') banner('NEW GAME PLANS', '#d8b25a', false);
      break;
    case 'draft_ban':
      state.mode = 'draft';
      state.draft.bans[team].push(d.champ);
      state.draft.step = d.step || ''; state.draft.active = { team, kind: 'ban', slot: state.draft.bans[team].length - 1 };
      break;
    case 'draft_pick': {
      state.mode = 'draft';
      state.draft.picks[team].push({ champ: d.champ, role: d.role, cls: d.class || champInfo(state.meta, d.champ).class });
      if (d.roles) state.draft.picks[team].forEach(p => { if (d.roles[p.champ]) p.role = d.roles[p.champ]; });
      state.draft.step = d.step || ''; state.draft.active = { team, kind: 'pick', slot: state.draft.picks[team].length - 1 };
      break;
    }
    case 'kill': {
      const v = d.victim || {}, k = d.killer || {};
      if (ref(v)) { state.dead[v.side][v.slot] = state.t + respawnTime(state.t); state.kda[v.side][v.slot].d++; fx.hits.push({ team: v.side, slot: v.slot, hp: 0 }); }
      if (ref(k)) {
        state.kda[k.side][k.slot].k++; fx.hits.push({ team: k.side, slot: k.slot, dmg: .45 });
        (d.assists || []).forEach(a => { if (a >= 0 && a < 5) { state.kda[k.side][a].a++; fx.hits.push({ team: k.side, slot: a, dmg: .3 }); } });
      }
      state.feed.unshift({ killer: k, victim: v, t: state.t, multi: d.multi || 0, ace: !!d.ace });
      state.feed.length = Math.min(state.feed.length, 6);
      const kc = teamColor(state.meta, k.side === 1 ? 1 : 0);
      if (Number.isFinite(d.x) && Number.isFinite(d.y)) {
        fx.flashes.push({ x: d.x, y: d.y, color: kc, big: false });
        fx.popups.push({ x: d.x, y: d.y, text: `+${d.gold || 300}g`, color: GOLDC });
        fx.skulls.push({ x: d.x, y: d.y, color: teamColor(state.meta, v.side === 1 ? 1 : 0) });
        fx.clash = { x: d.x, y: d.y };
      }
      const sub = `${k.champ || ''} ▸ ${v.champ || ''}`;
      if (d.ace) mark('ace', kc);
      if (d.multi >= 5) banner('PENTAKILL', kc, true, sub);
      else if (d.ace) banner(T.ace, kc, true, `${teamName(state, k.side === 1 ? 1 : 0)} wipe the enemy team`);
      else if (d.multi === 4) banner('QUADRA KILL', kc, true, sub);
      else if (d.multi === 3) banner('TRIPLE KILL', kc, true, sub);
      else if (d.tag === 'first_blood') { banner(T.firstBlood, kc, true, sub); mark('kill', kc); }
      else if (d.multi === 2) banner('DOUBLE KILL', kc, false, sub);
      else if (d.shutdown) banner(T.shutdown, kc, false, sub);
      break;
    }
    case 'teamfight':
      if (Number.isFinite(d.x)) { fx.flashes.push({ x: d.x, y: d.y, color: '#fff2c0', big: true }); fx.clash = { x: d.x, y: d.y, big: true }; }
      for (let t = 0; t < 2; t++) for (let r = 0; r < 5; r++) fx.hits.push({ team: t, slot: r, dmg: .25 });
      break;
    case 'dragon': {
      const dc = DRAGON_COLORS[d.type] || '#ffb347';
      if (d.type === 'elder') {
        state.buff[team].elder = state.t + 150; state.elders[team]++; state.obj.elderNext = state.t + 360;
        banner(d.stolen ? T.dragonStolen : T.elder, DRAGON_COLORS.elder, true, teamName(state, team));
      } else {
        state.dragons[team].push(d.type);
        state.obj.dragonNext = state.t + 300;
        if (d.soul) { state.soul[team] = d.type; state.obj.dragonNext = Infinity; state.obj.elderNext = state.t + 360; }
        if (d.stolen) banner(T.dragonStolen, dc, true, teamName(state, team));
        else if (d.soul) banner(T.soul(d.type || ''), dc, true, `${teamName(state, team)} claim the soul`);
        else banner(T.drake(d.type || ''), dc, false, teamName(state, team));
      }
      state.obj.last.dragon = { team, t: state.t };
      mark('dragon', dc);
      fx.flashes.push({ x: d.x, y: d.y, color: dc, big: true });
      break;
    }
    case 'baron':
      state.buff[team].baron = state.t + 180; state.obj.baronNext = state.t + 360; state.obj.last.baron = { team, t: state.t };
      banner(d.stolen ? T.baronStolen : T.baron, '#9b59ff', true, `${teamName(state, team)} empowered`);
      mark('baron', '#b77bff');
      fx.flashes.push({ x: d.x, y: d.y, color: '#b77bff', big: true });
      break;
    case 'herald':
      if (d.obj === 'grubs') { state.obj.grubsTaken = true; state.obj.last.grubs = { team, t: state.t }; banner(T.grubsBanner, '#a78bfa', false, teamName(state, team)); }
      else { state.heralds[team]++; state.obj.heraldTaken = true; state.obj.last.herald = { team, t: state.t }; banner(d.stolen ? T.heraldStolen : T.herald, '#a78bfa', !!d.stolen, teamName(state, team)); mark('herald', '#a78bfa'); }
      fx.flashes.push({ x: d.x, y: d.y, color: '#a78bfa', big: false });
      break;
    case 'tower': {
      const owner = d.owner === 1 ? 1 : 0;
      state.towers[`${mapSide(state.meta, owner)}-${d.lane}-${d.tier}`] = false;
      fx.flashes.push({ x: d.x, y: d.y, color: '#ffcf6b', big: false, debris: true });
      if (Number.isFinite(d.x)) fx.popups.push({ x: d.x, y: d.y, text: 'TOWER DOWN', color: color });
      break;
    }
    case 'inhibitor': {
      const owner = d.owner === 1 ? 1 : 0;
      const key = `${mapSide(state.meta, owner)}-${d.lane}`;
      state.inhibs[key] = !!d.respawn;
      if (d.respawn) delete state.inhibDown[key];
      else { state.inhibDown[key] = state.t; banner(T.inhib, color, true, `${teamName(state, team)} · ${d.lane} lane`); mark('inhib', color); fx.flashes.push({ x: d.x, y: d.y, color: '#ffcf6b', big: true, debris: true }); }
      break;
    }
    case 'nexus':
      state.nexus[mapSide(state.meta, d.owner === 1 ? 1 : 0)] = false;
      banner(T.nexus, color, true, `${teamName(state, team)} win!`);
      mark('nexus', color);
      fx.flashes.push({ x: d.x, y: d.y, color: '#ffffff', big: true, debris: true });
      break;
    case 'lane': {
      if (d.lane && Number.isFinite(d.diff)) state.laneLead[d.lane] = (mapSide(state.meta, team) === 0 ? 1 : -1) * Math.abs(d.diff);
      if (Number.isFinite(d.plates) && Number.isFinite(d.x)) fx.popups.push({ x: d.x, y: d.y, text: `+${d.plates}g`, color: GOLDC });
      break;
    }
    case 'gank':
      if (Number.isFinite(d.x)) fx.flashes.push({ x: d.x, y: d.y, color, big: false });
      break;
    case 'match_end': {
      state.end = d;
      const fk = finalKills(d);
      if (fk) state.kills = fk;
      const wt = d.winner === 1 ? 1 : 0;
      if (d.winner_name && Array.isArray(state.names) && !state.names[wt]) state.names[wt] = d.winner_name;
      if (Array.isArray(d.graph) && d.graph.length > 1) state.goldPts = d.graph.map(p => [p[0], p[1]]);
      if (Array.isArray(d.players)) d.players.forEach((pl, t) => (pl || []).forEach((p, r) => { if (t < 2 && r < 5 && p) { state.kda[t][r] = { k: p.k | 0, d: p.d | 0, a: p.a | 0 }; if (p.gold) state.pgold[t][r] = p.gold; } }));
      break;
    }
  }
  state.ticker = caption(state, ev) || '';
  if (Number.isFinite(d.x) && Number.isFinite(d.y)) fx.focus = { x: d.x, y: d.y };
  state.target = layout(state, ev);
  return fx;
}

// ---- action cam (pure) ----
//
// The camera is {x, y, z}: the map point it looks at (0..100) and the zoom (1 =
// the whole map). Zoomed in, the view also tilts: a bit more vertical squash and
// a gentle perspective, so things nearer the bottom of the screen look bigger.

export const FULL_CAM = { x: 50, y: 50, z: 1 };
// Room each filmed monster takes (map units): champions stand around it.
const MON_R = { baron: 7.5, drake: 6.5, elder: 7, herald: 6, grubs: 5.5 };
const MON_SCALE = { baron: 1.35, drake: 1.15, elder: 1.25, herald: 1.3, grubs: 1.2 };
const PERSP = .26;
export const easeInOut = t => { t = clamp(t, 0, 1); return t < .5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2; };

// How tilted the camera is at zoom z (0 = top-down full map, 1 = full action cam).
export function camTilt(z) { return clamp((z - 1) / 1.6, 0, 1); }

// Camera at time t of a tween {from, to, t0, dur} (eased in and out).
export function camAt(tw, t) {
  if (!tw) return { ...FULL_CAM };
  const k = tw.dur > 0 ? easeInOut((t - tw.t0) / tw.dur) : 1;
  return { x: lerp(tw.from.x, tw.to.x, k), y: lerp(tw.from.y, tw.to.y, k), z: lerp(tw.from.z, tw.to.z, k) };
}

// Projects a point of the flat (z = 1) broadcast view through the camera.
// cam: {fX, fY: the focus in flat screen coords, z}; view: {cx, cy: where the
// flat view puts the map centre, ax, ay: where the zoomed focus sits, h: height}.
// Returns [X, Y, scale] (scale = how much bigger things are drawn there).
export function camProject(cam, view, X, Y) {
  const z = cam.z, t = camTilt(z), sq = 1 - .2 * t, p = PERSP * t;
  const ax = lerp(view.cx, view.ax, t), ay = lerp(view.cy, view.ay, t);
  const y1 = (Y - cam.fY) * z * sq / view.h, yc = Math.max(y1, -1.5);
  const k = Math.max(.3, 1 + p * yc);
  return [ax + (X - cam.fX) * z * k, ay + view.h * (y1 + p * yc * yc / 2), z * k];
}

// Camera for a shot: its zoom, framed so it doesn't film too much void past the map edge.
export function shotCam(s) { return { x: clamp(s.x, 14, 86), y: clamp(s.y, 14, 86), z: s.zoom }; }

// What the action cam should show for an event (null = stay on the full map).
// prio: 1 minor (only after a quiet spell) … 5 must-see. replay: worth a replay.
export function shotFor(state, ev) {
  const d = (ev && ev.data) || {};
  if (!ev || !Number.isFinite(d.x) || !Number.isFinite(d.y) || !state || state.mode !== 'map') return null;
  const team = d.side === 1 ? 1 : 0;
  const shot = { x: d.x, y: d.y, team, kind: 'fight', zoom: 2.6, prio: 1, hold: 2.4, label: '', where: placeName(state, d.x, d.y), replay: false, mon: null, struct: null };
  const S = o => Object.assign(shot, o);
  const owner = d.owner === 1 ? 1 : 0;
  switch (ev.kind) {
    case 'teamfight': return d.size === 'teamfight' ? S({ zoom: 2.7, prio: 3, hold: 2.8, label: 'TEAMFIGHT' }) : S({ zoom: 2.5, prio: 2, hold: 2.2, label: 'SKIRMISH' });
    case 'kill': {
      const multi = d.multi | 0, v = d.victim || {};
      if (d.ace) return S({ prio: 4, label: 'ACE', replay: true });
      if (multi >= 4) return S({ prio: 4, label: multi >= 5 ? 'PENTAKILL' : 'QUADRA KILL', replay: true });
      if (multi === 3) return S({ prio: 3, label: 'TRIPLE KILL' });
      const vs = v.side === 0 || v.side === 1 ? mapSide(state.meta, v.side) : -1;
      const dive = vs >= 0 && (state.map.towers || []).some(t => t.side === vs && state.towers[`${t.side}-${t.lane}-${t.tier}`] !== false && Math.hypot(t.x - d.x, t.y - d.y) < 7);
      if (d.tag === 'first_blood') return S({ prio: 3, label: 'FIRST BLOOD' });
      if (dive) return S({ prio: 2, zoom: 2.5, label: 'TOWER DIVE' });
      if (d.shutdown) return S({ prio: 2, label: 'SHUTDOWN' });
      if (multi === 2) return S({ prio: 2, label: 'DOUBLE KILL' });
      return S({ hold: 1.8, label: 'PICK-OFF' });
    }
    case 'gank': return d.invade || d.success === false ? null : S({ zoom: 2.5, hold: 1.8, label: 'GANK' });
    case 'dragon': {
      const elder = d.type === 'elder';
      return S({ kind: 'objective', zoom: 2.8, prio: elder ? 5 : d.soul ? 4 : 3, hold: 2.6, replay: !!d.stolen,
        label: d.stolen ? (elder ? 'ELDER STEAL' : 'DRAGON STEAL') : elder ? 'ELDER DRAGON' : d.soul ? 'DRAGON SOUL' : 'DRAGON',
        mon: { kind: elder ? 'elder' : 'drake', col: DRAGON_COLORS[d.type] || '#ff8a3d', name: elder ? 'ELDER DRAGON' : `${String(d.type || '').toUpperCase()} DRAKE`.trim(), stolen: !!d.stolen } });
    }
    case 'baron': return S({ kind: 'objective', zoom: 2.6, prio: 5, hold: 2.8, replay: !!d.stolen, label: d.stolen ? 'BARON STEAL' : 'BARON NASHOR', mon: { kind: 'baron', col: '#9b59ff', name: 'BARON NASHOR', stolen: !!d.stolen } });
    case 'herald':
      if (d.obj === 'grubs') return S({ kind: 'objective', zoom: 2.7, prio: 2, hold: 2.2, label: 'VOID GRUBS', mon: { kind: 'grubs', col: '#a78bfa', name: 'VOID GRUBS', stolen: false } });
      return S({ kind: 'objective', zoom: 2.7, prio: 3, hold: 2.4, replay: !!d.stolen, label: d.stolen ? 'HERALD STEAL' : 'RIFT HERALD', mon: { kind: 'herald', col: '#c48bff', name: 'RIFT HERALD', stolen: !!d.stolen } });
    case 'tower':
      return S({ kind: 'struct', zoom: 2.4, prio: d.lane === 'nexus' || d.tier >= 2 ? 2 : 1, hold: 2.0, label: d.lane === 'nexus' ? 'NEXUS TOWER' : 'TOWER DOWN',
        struct: { kind: 'tower', side: mapSide(state.meta, owner), lane: d.lane, tier: d.tier } });
    case 'inhibitor':
      if (d.respawn) return null;
      return S({ kind: 'struct', zoom: 2.4, prio: 3, hold: 2.2, label: 'INHIBITOR', struct: { kind: 'inhib', side: mapSide(state.meta, owner), lane: d.lane } });
    case 'nexus': return S({ kind: 'struct', zoom: 2.2, prio: 5, hold: 3.2, label: 'NEXUS', struct: { kind: 'nexus', side: mapSide(state.meta, owner) } });
  }
  return null;
}

// What to do with a new shot given the current one: 'start' | 'merge' (same
// place: keep filming) | 'pan' (move there) | 'cut' (jump there, with a flash) |
// 'skip' | 'none'. sinceLast: seconds since the last shot ended.
export function shotAction(cur, shot, sinceLast) {
  if (!shot) return 'none';
  if (!cur) return shot.prio >= 2 || sinceLast > 8 ? 'start' : 'skip';
  const dist = Math.hypot(shot.x - cur.x, shot.y - cur.y);
  if (dist < 14) return 'merge';
  if (shot.prio >= cur.prio) return dist < 30 ? 'pan' : 'cut';
  return 'skip';
}

// Health of an objective monster / structure while the action cam watches it
// die: from hp0 down to 0 at dieAt.
export function drainHp(o, t) {
  if (!o || !Number.isFinite(o.t0) || !Number.isFinite(o.dieAt)) return 1;
  const k = clamp((t - o.t0) / Math.max(.01, o.dieAt - o.t0), 0, 1);
  return (o.hp0 == null ? 1 : o.hp0) * (1 - k * k);
}

// ---- arena director (pure) ----
//
// Between map shots the broadcast cuts to the arena: a wide shot of the stage,
// the crowd, the team booths and the caster desk. These scenes pause the match
// (like a replay), so they never hide a fight.

export const CAM_TAGS = { arena: 'ARENA', crowd: 'ARENA', booth: 'BOOTH', casters: 'CASTERS', trophy: 'ARENA', map: 'MAP', action: 'ACTION CAM', replay: 'REPLAY' };
const BOOTH_LABELS = ['BARON NASHOR', 'BARON STEAL', 'ELDER DRAGON', 'ELDER STEAL', 'DRAGON SOUL'];
const WIDE_LABELS = ['ACE', 'PENTAKILL', 'QUADRA KILL'];

// How loud the arena gets for an event: {level 0..1, team (whose fans roar, -1 =
// everyone), label}. level 0 = the crowd doesn't care.
export function crowdReaction(state, ev) {
  const d = (ev && ev.data) || {};
  let team = d.side === 1 ? 1 : d.side === 0 ? 0 : -1;
  const R = (level, label) => ({ level, team, label });
  switch (ev && ev.kind) {
    case 'kill': {
      const m = d.multi | 0, k = d.killer || {};
      if (k.side === 0 || k.side === 1) team = k.side;
      if (m >= 5) return R(1, 'PENTAKILL');
      if (d.ace) return R(.9, 'ACE');
      if (m === 4) return R(.85, 'QUADRA KILL');
      if (d.tag === 'first_blood') return R(.7, 'FIRST BLOOD');
      if (m === 3) return R(.6, 'TRIPLE KILL');
      if (d.shutdown) return R(.35, 'SHUTDOWN');
      return R(m === 2 ? .3 : .15, m === 2 ? 'DOUBLE KILL' : 'KILL');
    }
    case 'dragon':
      if (d.type === 'elder') return R(.95, d.stolen ? 'ELDER STEAL' : 'ELDER DRAGON');
      if (d.soul) return R(.85, 'DRAGON SOUL');
      return d.stolen ? R(.8, 'DRAGON STEAL') : R(.3, 'DRAGON');
    case 'baron': return R(d.stolen ? .95 : .9, d.stolen ? 'BARON STEAL' : 'BARON NASHOR');
    case 'herald':
      if (d.obj === 'grubs') return R(.2, 'VOID GRUBS');
      return d.stolen ? R(.6, 'HERALD STEAL') : R(.3, 'RIFT HERALD');
    case 'inhibitor': return d.respawn ? R(0, '') : R(.5, 'INHIBITOR');
    case 'tower': return d.lane === 'nexus' ? R(.45, 'NEXUS TOWER') : R(.15, 'TOWER');
    case 'teamfight': return R(d.size === 'teamfight' ? .4 : .25, 'TEAMFIGHT');
    case 'nexus': return R(1, 'NEXUS');
    case 'match_end': team = d.winner === 1 ? 1 : 0; return R(1, 'VICTORY');
  }
  return { level: 0, team: -1, label: '' };
}

// The reaction cut a big moment earns: null, or {kind: 'crowd' (fans close-up) |
// 'arena' (wide, the whole arena on its feet) | 'booth' (the team celebrating),
// team, level, label, dur}. ctx: {sinceCut: seconds since the last cut, speed}.
// The nexus has its own finale, so it never gets a quick cut.
export function reactionCut(r, ctx = {}) {
  if (!r || !(r.level >= .5) || r.label === 'NEXUS' || r.label === 'VICTORY' || (ctx.speed || 1) > 3.01) return null;
  const since = ctx.sinceCut == null ? Infinity : ctx.sinceCut;
  if (since < (r.level >= .9 ? 9 : 20)) return null;
  const kind = BOOTH_LABELS.includes(r.label) && r.team >= 0 ? 'booth' : WIDE_LABELS.includes(r.label) ? 'arena' : 'crowd';
  return { kind, team: r.team, level: r.level, label: r.label, dur: clamp(1.5 + 1.5 * r.level, 1.5, 3), mood: kind === 'booth' ? 'win' : '' };
}

// May a pending reaction cut roll now? 'now' | 'wait' | 'drop'. Never while the
// action cam films or a replay runs (the cut waits for them), never too stale.
// ctx: {now, shot, replay, mode}.
export function cutReady(p, ctx = {}) {
  if (!p || ctx.mode !== 'map') return 'drop';
  const age = (ctx.now || 0) - p.at;
  if (age > 8) return 'drop';
  if (ctx.shot || ctx.replay || age < 1.2) return 'wait';
  return 'now';
}

// A cut between draft phases (prev: the kind of the last draft event, n: cuts so
// far): ban → pick goes to the booth of the team about to pick, pick → ban to the
// caster desk.
export function draftCut(prev, ev, n = 0) {
  const k = ev && ev.kind;
  if ((k !== 'draft_ban' && k !== 'draft_pick') || !prev || prev === k) return null;
  const d = ev.data || {}, team = d.side === 1 ? 1 : 0;
  if (k === 'draft_pick') return { kind: 'booth', team, dur: 1.8, label: n < 1 ? 'PICK PHASE 1' : 'PICK PHASE 2', mood: 'focus' };
  return { kind: 'casters', team: -1, dur: 2.2, label: 'BAN PHASE 2', sub: 'bans2' };
}

// The show's opening (joining mid-game: just a quick look at the arena).
export function openingScenes(state) {
  if (!state || state.mode !== 'draft') return [{ kind: 'arena', sub: 'opening', team: -1, level: .45, dur: 3, label: 'LIVE FROM THE ARENA' }];
  return [
    { kind: 'arena', sub: 'opening', team: -1, level: .55, dur: 3.4, label: 'GRAND FINAL' },
    { kind: 'casters', sub: 'intro', team: -1, dur: 2.4, label: 'CHAMPION DRAFT' },
  ];
}

// Draft locked in → teams ready → the map.
export function readyScenes() { return [{ kind: 'arena', sub: 'ready', team: -1, level: .6, dur: 3, label: 'GAME ON' }]; }

// Nexus down: the arena erupts, the booth celebrates, the trophy is lifted.
export function finaleScenes(winner) {
  const team = winner === 1 ? 1 : 0;
  return [
    { kind: 'arena', sub: 'finale', team, level: 1, dur: 3.2, label: 'VICTORY', confetti: true, fin: true },
    { kind: 'booth', team, level: 1, dur: 2.6, label: 'VICTORY', mood: 'win', confetti: true, fin: true },
    { kind: 'trophy', team, level: 1, dur: 6, label: 'CHAMPIONS', confetti: true, fin: true },
  ];
}

// The crowd's noise during a scene: rises fast, then slowly settles (0..level).
export function crowdHype(level, t, dur) {
  if (!(level > 0) || !(t >= 0)) return 0;
  const rise = easeOut(clamp(t / .35, 0, 1)), settle = 1 - .35 * clamp((t - .35) / Math.max(.5, dur || 3), 0, 1);
  return clamp(level * rise * settle, 0, level);
}

// How much one fan feels it: their team's moment (full), a shared one (most of
// it), the rival's (a groan). fanTeam / team: -1 = neutral / everyone.
export function fanHype(h, fanTeam, team) {
  if (!(h > 0)) return 0;
  if (team < 0) return h * .85;
  if (fanTeam < 0) return h * .5;
  return fanTeam === team ? h : h * .1;
}

// World camera of an arena scene ({x, y, z} in the 1600×900 arena; pos: the
// cheering team's side on screen, 0 left / 1 right / -1 none).
export function sceneCam(sc, t, pos = -1) {
  const k = sc && sc.dur > 0 ? clamp(t / sc.dur, 0, 1) : 0, e = easeInOut(k), dir = pos === 0 ? -1 : pos === 1 ? 1 : 0;
  if (!sc) return { x: 800, y: 450, z: 1 };
  if (sc.kind === 'trophy') return { x: 800, y: lerp(425, 440, e), z: lerp(1.85, 2.2, e) };
  if (sc.sub === 'opening') return { x: 800, y: lerp(450, 430, e), z: lerp(1, 1.25, e) };
  if (sc.sub === 'finale') return { x: 800 + dir * 40 * (1 - e), y: 450, z: lerp(1.12, 1, e) };
  return { x: 800 + dir * 40 * e, y: lerp(455, 445, e), z: lerp(1.08, 1.16, e) };
}

// ---- drawing primitives ----

const CLASS_GLYPH = {
  tank(g, r) { g.beginPath(); g.moveTo(0, -r); g.lineTo(r * .85, -r * .55); g.lineTo(r * .7, r * .45); g.lineTo(0, r); g.lineTo(-r * .7, r * .45); g.lineTo(-r * .85, -r * .55); g.closePath(); g.fill(); },
  bruiser(g, r) { g.beginPath(); g.moveTo(-r * .2, r); g.lineTo(-r * .2, -r * .5); g.lineTo(-r * .9, -r * .9); g.lineTo(r * .9, -r * .9); g.lineTo(r * .2, -r * .5); g.lineTo(r * .2, r); g.closePath(); g.fill(); },
  assassin(g, r) { g.beginPath(); g.moveTo(0, -r); g.lineTo(r * .3, r * .3); g.lineTo(0, r); g.lineTo(-r * .3, r * .3); g.closePath(); g.fill(); },
  mage(g, r) { g.beginPath(); for (let i = 0; i < 10; i++) { const a = -Math.PI / 2 + i * Math.PI / 5, rr = i % 2 ? r * .42 : r; g.lineTo(Math.cos(a) * rr, Math.sin(a) * rr); } g.closePath(); g.fill(); },
  marksman(g, r) { g.lineWidth = r * .22; g.beginPath(); g.moveTo(-r * .8, r * .8); g.lineTo(r * .6, -r * .6); g.stroke(); g.beginPath(); g.moveTo(r * .95, -r * .95); g.lineTo(r * .2, -r * .7); g.lineTo(r * .7, -r * .2); g.closePath(); g.fill(); },
  enchanter(g, r) { g.beginPath(); g.moveTo(0, r * .9); g.bezierCurveTo(-r * 1.2, 0, -r * .5, -r * 1.1, 0, -r * .4); g.bezierCurveTo(r * .5, -r * 1.1, r * 1.2, 0, 0, r * .9); g.fill(); },
  engage(g, r) { g.lineWidth = r * .26; g.beginPath(); g.moveTo(-r * .8, -r * .1); g.lineTo(0, r * .7); g.lineTo(r * .8, -r * .1); g.stroke(); g.beginPath(); g.moveTo(-r * .8, -r * .8); g.lineTo(0, 0); g.lineTo(r * .8, -r * .8); g.stroke(); },
};

// Procedural champion portraits: a bust in the team's armour on the class
// colour, with headgear built from the class shape (a tank's helm, a
// bruiser's horns, an assassin's hood, a mage's star crown, a marksman's bow,
// an enchanter's charm, an engage crest) and the class emblem behind. Every
// detail (skin, hair, eyes, tilt) comes from the champion's name, so the same
// champion always looks the same. Painted once per size into a cache.
const PORTRAIT_SKINS = ['#f1c7a5', '#d9a47e', '#b77b52', '#8d5a3b', '#e8b893', '#c7b3e6', '#a9cfd9', '#9fd3a8'];
const PORTRAIT_HAIRS = ['#1d1612', '#4a2e1c', '#c9a05a', '#e8e2d6', '#7a1f2b', '#2d3f7a', '#3a6b46', '#9b4fd0'];
const PORTRAITS = new Map();

function paintPortrait(g, r, info, ring, dead) {
  const cls = info.class || 'bruiser', h = hash(info.name || '?');
  const base = dead ? '#555a63' : (CLASS_COLORS[cls] || '#888');
  const hue = (h % 40) - 20, team = dead ? '#6a6f78' : (/^#[0-9a-f]{6}$/i.test(ring) ? ring : '#7a8494');
  const skin = dead ? '#9a9da3' : PORTRAIT_SKINS[(h >>> 3) % PORTRAIT_SKINS.length], hair = dead ? '#55585e' : PORTRAIT_HAIRS[(h >>> 7) % PORTRAIT_HAIRS.length];
  const armor = shade(team, -.25), trim = shade(team, .4), eye = dead ? '#444' : shade(base, .55);
  const tilt = ((h >>> 11) % 7 - 3) * .025;
  g.save();
  g.beginPath(); g.arc(0, 0, r, 0, TAU); g.clip();
  // Backdrop: the class colour, a diagonal light band and the class emblem.
  const grd = g.createRadialGradient(-r * .3, -r * .4, r * .1, 0, 0, r);
  grd.addColorStop(0, shade(base, .35 + hue / 200)); grd.addColorStop(.7, base); grd.addColorStop(1, shade(base, -.45));
  g.fillStyle = grd; g.fillRect(-r, -r, 2 * r, 2 * r);
  g.save(); g.globalAlpha = .14; g.fillStyle = '#fff'; g.rotate((h % 6) * .5); g.fillRect(-r, -r * .25, r * 2, r * .3); g.restore();
  g.save(); g.globalAlpha = .2; g.fillStyle = '#fff'; g.strokeStyle = '#fff'; g.translate(0, -r * .12); (CLASS_GLYPH[cls] || CLASS_GLYPH.bruiser)(g, r * .9); g.restore();
  g.rotate(tilt);
  // Marksman: the bow behind the shoulder.
  if (cls === 'marksman') { g.strokeStyle = shade('#8a5a2b', dead ? -.3 : 0); g.lineWidth = r * .07; g.beginPath(); g.arc(r * .55, r * .2, r * .62, -Math.PI * .75, Math.PI * .35); g.stroke(); g.strokeStyle = 'rgba(255,255,255,.6)'; g.lineWidth = r * .02; g.beginPath(); g.moveTo(r * .55 + Math.cos(-Math.PI * .75) * r * .62, r * .2 + Math.sin(-Math.PI * .75) * r * .62); g.lineTo(r * .55 + Math.cos(Math.PI * .35) * r * .62, r * .2 + Math.sin(Math.PI * .35) * r * .62); g.stroke(); }
  // Long hair behind the head (mages, enchanters, marksmen).
  if (cls === 'mage' || cls === 'enchanter' || cls === 'marksman') { g.fillStyle = hair; g.beginPath(); g.ellipse(0, r * .05, r * .4, r * .5, 0, 0, TAU); g.fill(); }
  // Shoulders: team armour with a trim and the class emblem on the chest.
  g.fillStyle = armor; g.beginPath(); g.moveTo(-r * 1.05, r * 1.05); g.quadraticCurveTo(-r * .95, r * .42, -r * .3, r * .4); g.lineTo(r * .3, r * .4); g.quadraticCurveTo(r * .95, r * .42, r * 1.05, r * 1.05); g.closePath(); g.fill();
  g.strokeStyle = trim; g.lineWidth = r * .06; g.beginPath(); g.moveTo(-r * .85, r * .62); g.quadraticCurveTo(0, r * .38, r * .85, r * .62); g.stroke();
  if (cls === 'tank' || cls === 'bruiser' || cls === 'engage') { g.fillStyle = shade(team, -.05); for (const k of [-1, 1]) { g.beginPath(); g.ellipse(k * r * .66, r * .55, r * .3, r * .17, k * .35, 0, TAU); g.fill(); } }
  g.save(); g.translate(0, r * .78); g.fillStyle = trim; g.strokeStyle = trim; (CLASS_GLYPH[cls] || CLASS_GLYPH.bruiser)(g, r * .16); g.restore();
  // Neck and face.
  g.fillStyle = shade(skin, -.2); g.fillRect(-r * .11, r * .12, r * .22, r * .3);
  g.fillStyle = skin; g.beginPath(); g.ellipse(0, -r * .08, r * .27, r * .33, 0, 0, TAU); g.fill();
  g.fillStyle = shade(skin, -.18); g.beginPath(); g.ellipse(r * .06, r * .06, r * .2, r * .17, 0, 0, Math.PI); g.fill();
  // Eyes (glowing in the class colour under a helm or a hood).
  const covered = cls === 'tank' || cls === 'assassin' || cls === 'engage';
  g.fillStyle = covered ? eye : '#1b1b22';
  for (const k of [-1, 1]) { g.beginPath(); g.ellipse(k * r * .1, -r * .08, r * (covered ? .065 : .045), r * (covered ? .03 : .04), 0, 0, TAU); g.fill(); }
  if (!covered) { g.strokeStyle = shade(hair, -.2); g.lineWidth = r * .03; for (const k of [-1, 1]) { g.beginPath(); g.moveTo(k * r * .04, -r * .17); g.lineTo(k * r * .17, -r * .19 + (cls === 'assassin' ? r * .03 : 0)); g.stroke(); } }
  // Headgear from the class shape.
  const metal = dead ? '#777' : shade('#9aa6b8', hue / 100);
  if (cls === 'tank') {
    g.fillStyle = metal; g.beginPath(); g.moveTo(0, -r * .58); g.lineTo(r * .34, -r * .4); g.lineTo(r * .33, -r * .02); g.lineTo(r * .2, r * .2); g.lineTo(-r * .2, r * .2); g.lineTo(-r * .33, -r * .02); g.lineTo(-r * .34, -r * .4); g.closePath(); g.fill();
    g.fillStyle = '#15161b'; g.fillRect(-r * .25, -r * .12, r * .5, r * .08);
    g.fillStyle = eye; g.fillRect(-r * .2, -r * .105, r * .14, r * .05); g.fillRect(r * .06, -r * .105, r * .14, r * .05);
    g.fillStyle = 'rgba(255,255,255,.3)'; g.fillRect(-r * .02, -r * .55, r * .04, r * .4);
  } else if (cls === 'bruiser') {
    g.fillStyle = metal; g.beginPath(); g.arc(0, -r * .17, r * .3, Math.PI, TAU); g.lineTo(r * .3, -r * .1); g.lineTo(-r * .3, -r * .1); g.fill();
    g.fillStyle = dead ? '#888' : '#efe6cf';
    for (const k of [-1, 1]) { g.beginPath(); g.moveTo(k * r * .26, -r * .3); g.quadraticCurveTo(k * r * .62, -r * .38, k * r * .58, -r * .78); g.quadraticCurveTo(k * r * .48, -r * .45, k * r * .2, -r * .4); g.fill(); }
    g.fillStyle = '#15161b'; g.fillRect(-r * .2, -r * .14, r * .4, r * .05);
  } else if (cls === 'assassin') {
    g.fillStyle = shade(base, -.45); g.beginPath(); g.moveTo(0, -r * .72); g.quadraticCurveTo(r * .42, -r * .4, r * .36, r * .25); g.lineTo(r * .2, r * .05); g.quadraticCurveTo(r * .22, -r * .32, 0, -r * .36); g.quadraticCurveTo(-r * .22, -r * .32, -r * .2, r * .05); g.lineTo(-r * .36, r * .25); g.quadraticCurveTo(-r * .42, -r * .4, 0, -r * .72); g.fill();
    g.fillStyle = shade(base, -.6); g.fillRect(-r * .2, r * .0, r * .4, r * .17); // mask
    g.fillStyle = 'rgba(0,0,0,.35)'; g.beginPath(); g.ellipse(0, -r * .2, r * .22, r * .1, 0, 0, TAU); g.fill();
  } else if (cls === 'mage') {
    g.fillStyle = hair; g.beginPath(); g.arc(0, -r * .17, r * .29, Math.PI * 1.05, Math.PI * 1.95); g.fill();
    g.save(); g.translate(0, -r * .5); g.fillStyle = dead ? '#999' : '#ffe27a'; CLASS_GLYPH.mage(g, r * .2); g.restore();
    g.fillStyle = dead ? '#777' : shade(base, .6); g.beginPath(); g.arc(0, -r * .26, r * .035, 0, TAU); g.fill();
  } else if (cls === 'marksman') {
    g.fillStyle = hair; g.beginPath(); g.arc(0, -r * .17, r * .29, Math.PI * 1.05, Math.PI * 1.95); g.fill();
    g.fillStyle = trim; g.fillRect(-r * .29, -r * .26, r * .58, r * .06);
  } else if (cls === 'enchanter') {
    g.fillStyle = hair; g.beginPath(); g.arc(0, -r * .17, r * .29, Math.PI * 1.02, Math.PI * 1.98); g.fill();
    g.save(); g.translate(r * .2, -r * .38); g.rotate(.4); g.fillStyle = dead ? '#999' : shade(base, .5); CLASS_GLYPH.enchanter(g, r * .16); g.restore();
  } else if (cls === 'engage') {
    g.fillStyle = metal; g.beginPath(); g.arc(0, -r * .15, r * .31, Math.PI, TAU); g.lineTo(r * .31, r * .05); g.lineTo(r * .18, r * .08); g.lineTo(r * .16, -r * .12); g.lineTo(-r * .16, -r * .12); g.lineTo(-r * .18, r * .08); g.lineTo(-r * .31, r * .05); g.closePath(); g.fill();
    g.fillStyle = trim; g.beginPath(); g.moveTo(-r * .05, -r * .46); g.quadraticCurveTo(r * .1, -r * .8, r * .42, -r * .7); g.quadraticCurveTo(r * .2, -r * .58, r * .05, -r * .44); g.fill();
  }
  g.restore();
  // The ring in the team colour.
  g.lineWidth = Math.max(1.5, r * .17); g.strokeStyle = dead ? '#777' : ring; g.beginPath(); g.arc(0, 0, r, 0, TAU); g.stroke();
}

export function drawPortrait(g, x, y, r, info, ring, opts = {}) {
  info = info || {};
  if (opts.glow) {
    // A soft halo in layered strokes (shadowBlur is slow).
    g.save(); g.strokeStyle = rgba(/^#[0-9a-f]{6}$/i.test(ring) ? ring : '#ffffff', .16);
    for (const k of [.55, .32]) { g.lineWidth = r * k; g.beginPath(); g.arc(x, y, r * (1 + k / 2), 0, TAU); g.stroke(); }
    g.restore();
  }
  const dead = !!opts.dead;
  if (typeof document !== 'undefined' && r >= 3) {
    const R = Math.pow(2, Math.ceil(Math.log2(r) * 4) / 4), pad = Math.ceil(R * .12) + 2;
    const key = (info.name || '') + '|' + (info.class || '') + '|' + ring + '|' + (dead ? 1 : 0) + '|' + R;
    let c = PORTRAITS.get(key);
    if (!c) {
      if (PORTRAITS.size > 400) PORTRAITS.clear();
      c = document.createElement('canvas'); c.width = c.height = Math.ceil(2 * (R + pad));
      const cg = c.getContext('2d'); cg.translate(R + pad, R + pad); paintPortrait(cg, R, info, ring, dead);
      PORTRAITS.set(key, c);
    }
    const k = r / R, half = c.width / 2 * k;
    g.drawImage(c, x - half, y - half, half * 2, half * 2);
  } else { g.save(); g.translate(x, y); paintPortrait(g, r, info, ring, dead); g.restore(); }
  if (opts.banned) {
    g.save(); g.translate(x, y);
    g.fillStyle = 'rgba(20,20,24,.55)'; g.beginPath(); g.arc(0, 0, r, 0, TAU); g.fill();
    const k = opts.slash == null ? 1 : opts.slash;
    g.strokeStyle = '#ff3b4b'; g.lineWidth = r * .2; g.lineCap = 'round'; g.beginPath(); g.moveTo(-r * .7, -r * .7); g.lineTo(-r * .7 + r * 1.4 * k, -r * .7 + r * 1.4 * k); g.stroke();
    g.restore();
  }
}

function drawDrake(g, r, col) {
  g.fillStyle = col;
  g.beginPath(); g.moveTo(0, -r * .05); g.quadraticCurveTo(-r * .8, -r * .95, -r * 1.05, r * .1); g.quadraticCurveTo(-r * .5, -r * .1, 0, r * .25);
  g.quadraticCurveTo(r * .5, -r * .1, r * 1.05, r * .1); g.quadraticCurveTo(r * .8, -r * .95, 0, -r * .05); g.fill();
  g.beginPath(); g.ellipse(0, r * .2, r * .2, r * .48, 0, 0, TAU); g.fill();
  g.beginPath(); g.moveTo(-r * .2, -r * .2); g.lineTo(0, -r * .7); g.lineTo(r * .2, -r * .2); g.fill();
}
function drawBaronGlyph(g, r, col) {
  g.strokeStyle = col; g.lineWidth = r * .3; g.lineCap = 'round';
  g.beginPath(); g.arc(-r * .1, r * .35, r * .62, Math.PI * 1.0, Math.PI * 1.85); g.stroke();
  g.fillStyle = col; g.beginPath(); g.ellipse(r * .42, -r * .2, r * .38, r * .26, -.6, 0, TAU); g.fill();
  g.lineWidth = r * .1; for (let i = 0; i < 3; i++) { const a = Math.PI * (1.15 + i * .22); g.beginPath(); g.moveTo(-r * .1 + Math.cos(a) * r * .7, r * .35 + Math.sin(a) * r * .7); g.lineTo(-r * .1 + Math.cos(a) * r * 1.0, r * .35 + Math.sin(a) * r * 1.0); g.stroke(); }
  g.fillStyle = '#fff'; g.beginPath(); g.arc(r * .5, -r * .25, r * .09, 0, TAU); g.fill();
}
function drawEyeGlyph(g, r, col) {
  g.fillStyle = col; g.beginPath(); g.moveTo(-r * .9, 0); g.quadraticCurveTo(0, -r * .8, r * .9, 0); g.quadraticCurveTo(0, r * .8, -r * .9, 0); g.fill();
  g.fillStyle = '#16102a'; g.beginPath(); g.arc(0, 0, r * .3, 0, TAU); g.fill();
  g.fillStyle = '#fff'; g.beginPath(); g.arc(r * .08, -r * .08, r * .09, 0, TAU); g.fill();
}
function drawGrubsGlyph(g, r, col) {
  g.fillStyle = col;
  for (const [x, y] of [[-r * .45, r * .2], [r * .45, r * .2], [0, -r * .3]]) { g.beginPath(); g.ellipse(x, y, r * .32, r * .26, 0, 0, TAU); g.fill(); }
}
function drawSkull(g, x, y, r, col) {
  g.fillStyle = col || '#eee'; g.beginPath(); g.arc(x, y - r * .15, r * .75, 0, TAU); g.fill();
  g.fillRect(x - r * .42, y + r * .25, r * .84, r * .45);
  g.fillStyle = '#111'; g.beginPath(); g.arc(x - r * .3, y - r * .15, r * .2, 0, TAU); g.arc(x + r * .3, y - r * .15, r * .2, 0, TAU); g.fill();
  g.fillRect(x - r * .06, y + r * .3, r * .12, r * .35);
}
function drawSwords(g, x, y, r, col) {
  g.strokeStyle = col; g.lineCap = 'round';
  for (const s of [-1, 1]) {
    g.lineWidth = r * .2; g.beginPath(); g.moveTo(x - s * r, y + r); g.lineTo(x + s * r * .8, y - r * .8); g.stroke();
    g.lineWidth = r * .16; g.beginPath(); g.moveTo(x - s * r * .75 - r * .25, y + r * .45 - s * s * r * .05); g.lineTo(x - s * r * .45 + r * .05, y + r * .75); g.stroke();
  }
}
function drawTowerIcon(g, x, y, r, col) {
  g.fillStyle = col; g.beginPath(); g.moveTo(x - r * .5, y + r); g.lineTo(x - r * .35, y - r * .4); g.lineTo(x + r * .35, y - r * .4); g.lineTo(x + r * .5, y + r); g.closePath(); g.fill();
  g.beginPath(); g.moveTo(x, y - r); g.lineTo(x + r * .5, y - r * .35); g.lineTo(x - r * .5, y - r * .35); g.closePath(); g.fill();
}
function drawCoin(g, x, y, r) {
  g.fillStyle = GOLDC; g.beginPath(); g.arc(x, y, r, 0, TAU); g.fill();
  g.strokeStyle = '#8a6420'; g.lineWidth = r * .22; g.beginPath(); g.arc(x, y, r * .6, 0, TAU); g.stroke();
}
function drawDiamond(g, x, y, r, col, filled = true) {
  g.beginPath(); g.moveTo(x, y - r); g.lineTo(x + r * .75, y); g.lineTo(x, y + r); g.lineTo(x - r * .75, y); g.closePath();
  if (filled) { g.fillStyle = col; g.fill(); } else { g.strokeStyle = col; g.lineWidth = Math.max(1, r * .2); g.stroke(); }
}

// ---- big monsters for the action cam (origin = the ground under them, S = px per map unit) ----

function groundGlow(g, S, col, r, a) {
  const gr = g.createRadialGradient(0, 0, 0, 0, 0, S * r); gr.addColorStop(0, rgba(col, a)); gr.addColorStop(1, rgba(col, 0));
  g.fillStyle = gr; g.beginPath(); g.ellipse(0, 0, S * r, S * r * .45, 0, 0, TAU); g.fill();
}

function paintDrake(g, S, col, t, elder) {
  const flap = Math.sin(t * 5.5), hov = -S * (2.5 + Math.sin(t * 2) * .2);
  groundGlow(g, S, col, 4.2, .35);
  g.fillStyle = 'rgba(0,0,0,.45)'; g.beginPath(); g.ellipse(0, 0, S * 2.6, S * .9, 0, 0, TAU); g.fill();
  const ag = g.createRadialGradient(0, hov, 0, 0, hov, S * 5); ag.addColorStop(0, rgba(col, elder ? .55 : .4)); ag.addColorStop(1, rgba(col, 0));
  g.fillStyle = ag; g.beginPath(); g.arc(0, hov, S * 5, 0, TAU); g.fill();
  g.save(); g.translate(0, hov);
  const dark = shade(col, -.6), mid = shade(col, -.25);
  g.lineCap = 'round'; g.lineJoin = 'round';
  // Tail.
  g.strokeStyle = dark; g.lineWidth = S * .5; g.beginPath(); g.moveTo(0, S * .9); g.quadraticCurveTo(S * 1.4 * Math.sin(t * 1.6), S * 2.3, S * 2.2, S * 1.8); g.stroke();
  // Wings.
  for (const sd of [-1, 1]) {
    const tx = sd * S * 4.3, ty = -S * 1.5 - flap * S * 1.1;
    g.fillStyle = dark;
    g.beginPath(); g.moveTo(sd * S * .4, -S * .7); g.lineTo(tx * .55, ty - S * .5); g.lineTo(tx, ty);
    g.quadraticCurveTo(sd * S * 3.4, -S * .1 + flap * S * .4, sd * S * 2.6, S * .5);
    g.quadraticCurveTo(sd * S * 1.8, S * .05, sd * S * 1.3, S * .55);
    g.quadraticCurveTo(sd * S * .9, S * .1, sd * S * .4, S * .3); g.closePath(); g.fill();
    g.strokeStyle = col; g.lineWidth = Math.max(1, S * .14); g.stroke();
    g.strokeStyle = mid; g.lineWidth = Math.max(1, S * .1);
    for (const [ex, ey] of [[sd * S * 2.6, S * .5], [sd * S * 1.3, S * .55]]) { g.beginPath(); g.moveTo(tx * .55, ty - S * .5); g.lineTo(ex, ey); g.stroke(); }
  }
  // Body.
  const bg = g.createRadialGradient(-S * .3, -S * .4, S * .1, 0, 0, S * 1.5); bg.addColorStop(0, shade(col, .35)); bg.addColorStop(.6, col); bg.addColorStop(1, dark);
  g.fillStyle = bg; g.beginPath(); g.ellipse(0, 0, S * .95, S * 1.35, 0, 0, TAU); g.fill();
  g.strokeStyle = 'rgba(255,255,255,.25)'; g.lineWidth = Math.max(1, S * .07);
  for (let i = 0; i < 3; i++) { g.beginPath(); g.arc(0, S * (-.2 + i * .4), S * .5, .3, Math.PI - .3); g.stroke(); }
  // Neck and head.
  g.fillStyle = mid; g.beginPath(); g.ellipse(0, -S * 1.25, S * .45, S * .55, 0, 0, TAU); g.fill();
  g.fillStyle = col; g.beginPath(); g.ellipse(0, -S * 1.85, S * .72, S * .52, 0, 0, TAU); g.fill();
  g.beginPath(); g.ellipse(0, -S * 1.45, S * .38, S * .42, 0, 0, TAU); g.fill();
  g.fillStyle = dark;
  for (const sd of [-1, 1]) { g.beginPath(); g.moveTo(sd * S * .35, -S * 2.2); g.lineTo(sd * S * .85, -S * 2.95); g.lineTo(sd * S * .6, -S * 2.05); g.closePath(); g.fill(); }
  g.shadowColor = elder ? '#fff3a0' : '#fff7c0'; g.shadowBlur = S * .6; g.fillStyle = elder ? '#fff3a0' : '#fffbe0';
  for (const sd of [-1, 1]) { g.beginPath(); g.ellipse(sd * S * .3, -S * 1.92, S * .13, S * .08, sd * .3, 0, TAU); g.fill(); }
  g.shadowBlur = 0;
  if (elder) {
    for (let i = 0; i < 9; i++) {
      const a = i / 9 * TAU + t * 1.3, fl = .6 + .4 * Math.sin(t * 9 + i * 2);
      g.fillStyle = i % 2 ? 'rgba(190,120,255,.55)' : 'rgba(255,225,120,.6)';
      const x = Math.cos(a) * S * 1.9, y = Math.sin(a) * S * 1.3;
      g.beginPath(); g.moveTo(x - S * .2, y); g.quadraticCurveTo(x, y - S * .9 * fl, x + S * .2, y); g.closePath(); g.fill();
    }
  }
  g.restore();
}

function paintBaron(g, S, t) {
  const col = '#9b59ff', dark = '#22103a', mid = '#552a86';
  groundGlow(g, S, col, 5, .4);
  g.fillStyle = 'rgba(70,30,110,.65)'; g.beginPath(); g.ellipse(0, 0, S * 3.3, S * 1.25, 0, 0, TAU); g.fill();
  g.lineCap = 'round';
  for (let i = 0; i < 5; i++) {
    const a = (i / 5) * Math.PI + .3, sw = Math.sin(t * 2 + i) * S * .6;
    g.strokeStyle = mid; g.lineWidth = S * .38;
    const x0 = Math.cos(a) * S * 2.6 * (i % 2 ? 1 : -1), y0 = Math.sin(a) * S * .6;
    g.beginPath(); g.moveTo(x0 * .5, y0 * .5); g.quadraticCurveTo(x0 + sw, y0 - S * 1.6, x0 * 1.25 + sw, y0 - S * 2.2); g.stroke();
  }
  const segs = [];
  for (let i = 0; i <= 8; i++) { const f = i / 8; segs.push([Math.sin(f * 2.4 + t * .9) * S * .9 * (1 - f * .4), -f * S * 4.8, S * (1.6 - f * .65)]); }
  for (const [x, y, r] of segs) {
    const gr = g.createRadialGradient(x - r * .3, y - r * .4, r * .1, x, y, r); gr.addColorStop(0, '#8a55c8'); gr.addColorStop(.6, mid); gr.addColorStop(1, dark);
    g.fillStyle = gr; g.beginPath(); g.arc(x, y, r, 0, TAU); g.fill();
    g.fillStyle = '#c9a6ff'; g.beginPath(); g.moveTo(x + r * .6, y - r * .5); g.lineTo(x + r * 1.25, y - r * 1.0); g.lineTo(x + r * .85, y - r * .1); g.closePath(); g.fill();
  }
  const [hx, hy] = segs[segs.length - 1];
  g.save(); g.translate(hx, hy - S * .3); g.rotate(Math.sin(t * 1.2) * .12);
  g.fillStyle = mid; g.beginPath(); g.ellipse(0, 0, S * 1.5, S * 1.05, 0, 0, TAU); g.fill();
  g.strokeStyle = '#d7b8ff'; g.lineWidth = S * .22;
  for (const sd of [-1, 1]) { g.beginPath(); g.moveTo(sd * S * .8, -S * .7); g.quadraticCurveTo(sd * S * 1.8, -S * 1.6, sd * S * 1.3, -S * 2.4); g.stroke(); }
  const open = .35 + .25 * Math.sin(t * 3);
  g.fillStyle = '#12051f'; g.beginPath(); g.ellipse(0, S * .35, S * .9, S * open, 0, 0, TAU); g.fill();
  g.fillStyle = '#f2e8ff';
  for (let i = -3; i <= 3; i++) { g.beginPath(); g.moveTo(i * S * .22 - S * .08, S * .35 - S * open * .8); g.lineTo(i * S * .22, S * .35 - S * open * .2); g.lineTo(i * S * .22 + S * .08, S * .35 - S * open * .8); g.fill(); }
  g.shadowColor = '#e0b3ff'; g.shadowBlur = S * .8; g.fillStyle = '#f4dcff';
  for (const sd of [-1, 1]) { g.beginPath(); g.ellipse(sd * S * .62, -S * .3, S * .2, S * .12, sd * .4, 0, TAU); g.fill(); }
  g.shadowBlur = 0; g.restore();
}

function paintHerald(g, S, t) {
  const col = '#c48bff', bob = Math.sin(t * 3) * S * .15;
  groundGlow(g, S, col, 3.6, .35);
  g.fillStyle = 'rgba(0,0,0,.45)'; g.beginPath(); g.ellipse(0, 0, S * 2.4, S * .8, 0, 0, TAU); g.fill();
  g.strokeStyle = '#2b1842'; g.lineWidth = S * .28; g.lineCap = 'round';
  for (const sd of [-1, 1]) for (let i = 0; i < 3; i++) {
    const st = Math.sin(t * 6 + i * 2 + sd) * S * .2;
    g.beginPath(); g.moveTo(sd * S * .8, -S * (1 + i * .3)); g.lineTo(sd * S * (1.9 + i * .2), -S * (1.3 + i * .25) + st); g.lineTo(sd * S * (2.2 + i * .15), S * (.1 - i * .2)); g.stroke();
  }
  const by = -S * 1.4 + bob;
  const gr = g.createRadialGradient(-S * .5, by - S * .5, S * .2, 0, by, S * 2.2); gr.addColorStop(0, '#6a43a0'); gr.addColorStop(.7, '#3b2457'); gr.addColorStop(1, '#1b0f2a');
  g.fillStyle = gr; g.beginPath(); g.ellipse(0, by, S * 2.1, S * 1.35, 0, 0, TAU); g.fill();
  g.strokeStyle = 'rgba(200,160,255,.35)'; g.lineWidth = Math.max(1, S * .08);
  for (let i = 0; i < 3; i++) { g.beginPath(); g.ellipse(0, by, S * (2.1 - i * .45), S * (1.35 - i * .3), 0, Math.PI * 1.1, Math.PI * 1.9); g.stroke(); }
  g.save(); g.translate(0, by - S * .2); g.shadowColor = col; g.shadowBlur = S; drawEyeGlyph(g, S * 1.05, col); g.restore();
  g.fillStyle = '#d9b8ff';
  for (let i = -2; i <= 2; i++) { g.beginPath(); g.moveTo(i * S * .4 - S * .12, by - S * 1.2); g.lineTo(i * S * .4, by - S * (1.85 - Math.abs(i) * .2)); g.lineTo(i * S * .4 + S * .12, by - S * 1.2); g.fill(); }
}

function paintGrubs(g, S, t, alive = 3) {
  const col = '#8f6be0';
  groundGlow(g, S, '#a78bfa', 3.4, .3);
  [[-1.7, .5], [1.6, .3], [0, -.9]].slice(0, alive).forEach(([x, y], i) => {
    const ph = t * 4 + i * 2;
    g.save(); g.translate(x * S, y * S);
    g.fillStyle = 'rgba(0,0,0,.4)'; g.beginPath(); g.ellipse(0, 0, S * .9, S * .3, 0, 0, TAU); g.fill();
    for (let k = 2; k >= 0; k--) {
      const cx = (k - 1) * S * .45, cy = -S * .45 - Math.abs(Math.sin(ph + k)) * S * .15, r = S * (.42 - k * .05);
      g.fillStyle = k ? shade(col, -.2) : col; g.beginPath(); g.arc(cx, cy, r, 0, TAU); g.fill();
    }
    g.fillStyle = '#f5e8ff'; g.beginPath(); g.arc(-S * .55, -S * .55, S * .08, 0, TAU); g.arc(-S * .35, -S * .58, S * .08, 0, TAU); g.fill();
    g.restore();
  });
}

// ---- arena: people and props ----

const AW = 1600, AH = 900; // the arena "world" (scenes are framed in it, cover-fit)
const SKINS = ['#f3cfb0', '#e6b48f', '#c99068', '#a8704a', '#7d4e31', '#f7dcc6', '#5e3a24'];
const HAIRS = ['#16110e', '#2e1d12', '#5a3a1e', '#b88a50', '#26262b', '#7a7a80', '#a2462f', '#0d0d10'];
const LED_GOLD = '#f2c55c';

// One person, front (or back) view. (x, y): the waist; S: the torso height.
// o: {shirt, trim, skin, hair, hs (hair style), pants, legs, back, mouth (0..1),
// headset, suit, arms: [[a1, a2] left, [a1, a2] right] (radians from straight
// down, positive = outwards; a1 upper arm, a2 forearm), lean}. Returns the hands.
function drawPerson(g, x, y, S, o = {}) {
  const sh = o.shirt || '#333a48', skin = o.skin || SKINS[0], hair = o.hair || HAIRS[0];
  const lean = (o.lean || 0) * S;
  if (o.legs) {
    g.fillStyle = o.pants || '#1b1e28';
    g.fillRect(x - .3 * S, y - .02 * S, .25 * S, .95 * S); g.fillRect(x + .05 * S, y - .02 * S, .25 * S, .95 * S);
    g.fillStyle = '#0b0c10'; g.fillRect(x - .34 * S, y + .88 * S, .31 * S, .1 * S); g.fillRect(x + .03 * S, y + .88 * S, .31 * S, .1 * S);
  }
  const top = y - S, hx = x + lean, hy = top - .3 * S, hr = .27 * S;
  // Torso.
  g.fillStyle = sh; g.beginPath();
  g.moveTo(x - .33 * S, y); g.lineTo(x - .42 * S + lean, top + .12 * S); g.quadraticCurveTo(x - .42 * S + lean, top, x - .28 * S + lean, top);
  g.lineTo(x + .28 * S + lean, top); g.quadraticCurveTo(x + .42 * S + lean, top, x + .42 * S + lean, top + .12 * S); g.lineTo(x + .33 * S, y); g.closePath(); g.fill();
  if (o.suit && !o.back) {
    g.fillStyle = '#eef0f4'; g.beginPath(); g.moveTo(hx - .14 * S, top); g.lineTo(hx + .14 * S, top); g.lineTo(hx, top + .42 * S); g.closePath(); g.fill();
    g.fillStyle = o.trim || '#8a1c2c'; g.beginPath(); g.moveTo(hx - .035 * S, top + .05 * S); g.lineTo(hx + .035 * S, top + .05 * S); g.lineTo(hx + .05 * S, top + .38 * S); g.lineTo(hx, top + .45 * S); g.lineTo(hx - .05 * S, top + .38 * S); g.closePath(); g.fill();
  } else if (o.trim) {
    g.fillStyle = o.trim; g.fillRect(x - .41 * S + lean * .8, top + .3 * S, .82 * S, .07 * S);
    if (!o.back) { g.beginPath(); g.moveTo(hx - .12 * S, top); g.lineTo(hx + .12 * S, top); g.lineTo(hx, top + .14 * S); g.closePath(); g.fill(); }
  }
  // Neck and head.
  g.fillStyle = o.back ? shade(skin, -.25) : skin; g.fillRect(hx - .09 * S, top - .1 * S, .18 * S, .13 * S);
  g.beginPath(); g.arc(hx, hy, hr, 0, TAU); g.fill();
  g.fillStyle = hair;
  if (o.back) { g.beginPath(); g.arc(hx, hy - .02 * S, hr * 1.02, 0, TAU); g.fill(); }
  else {
    const hs = o.hs | 0;
    g.beginPath(); g.arc(hx, hy - .02 * S, hr * 1.04, Math.PI * (hs === 2 ? 1.12 : 1.0), Math.PI * (hs === 2 ? 1.88 : 2.0)); g.closePath(); g.fill();
    if (hs === 1) { g.fillRect(hx - hr * 1.04, hy - .04 * S, hr * .32, hr * 1.2); g.fillRect(hx + hr * .72, hy - .04 * S, hr * .32, hr * 1.2); }
    if (hs === 3) { g.beginPath(); g.arc(hx, hy - hr * .9, hr * .42, 0, TAU); g.fill(); }
    // Face.
    g.fillStyle = '#17110e';
    g.beginPath(); g.arc(hx - .095 * S, hy + .01 * S, .03 * S, 0, TAU); g.arc(hx + .095 * S, hy + .01 * S, .03 * S, 0, TAU); g.fill();
    const m = clamp(o.mouth || 0, 0, 1);
    if (m > .08) { g.fillStyle = '#4b1717'; g.beginPath(); g.ellipse(hx, hy + .14 * S, .065 * S + .02 * S * m, .02 * S + .07 * S * m, 0, 0, TAU); g.fill(); }
    else { g.fillStyle = shade(skin, -.4); g.fillRect(hx - .05 * S, hy + .13 * S, .1 * S, .02 * S); }
  }
  if (o.headset) {
    g.strokeStyle = '#0e0f14'; g.lineWidth = .06 * S; g.beginPath(); g.arc(hx, hy, hr * 1.12, Math.PI * 1.08, Math.PI * 1.92); g.stroke();
    g.fillStyle = '#16181f'; g.beginPath(); g.ellipse(hx - hr * 1.05, hy + .02 * S, .07 * S, .1 * S, 0, 0, TAU); g.ellipse(hx + hr * 1.05, hy + .02 * S, .07 * S, .1 * S, 0, 0, TAU); g.fill();
    if (o.trim) { g.fillStyle = o.trim; g.fillRect(hx - hr * 1.12, hy - .01 * S, .05 * S, .06 * S); g.fillRect(hx + hr * 1.07, hy - .01 * S, .05 * S, .06 * S); }
    if (!o.back) { g.strokeStyle = '#0e0f14'; g.lineWidth = .025 * S; g.beginPath(); g.moveTo(hx - hr * 1.05, hy + .08 * S); g.quadraticCurveTo(hx - hr * .9, hy + .2 * S, hx - .08 * S, hy + .17 * S); g.stroke(); }
  }
  // Arms.
  const arms = o.arms || [[.12, .1], [.12, .1]], hands = [];
  g.lineCap = 'round'; g.lineJoin = 'round';
  for (let k = 0; k < 2; k++) {
    const dir = k ? 1 : -1, [a1, a2] = arms[k];
    const sx = x + lean + dir * .36 * S, sy = top + .1 * S;
    const ex = sx + dir * Math.sin(a1) * .5 * S, ey = sy + Math.cos(a1) * .5 * S;
    const wx = ex + dir * Math.sin(a2) * .46 * S, wy = ey + Math.cos(a2) * .46 * S;
    g.strokeStyle = sh; g.lineWidth = .17 * S; g.beginPath(); g.moveTo(sx, sy); g.lineTo(ex, ey); g.lineTo(wx, wy); g.stroke();
    g.fillStyle = skin; g.beginPath(); g.arc(wx, wy, .085 * S, 0, TAU); g.fill();
    hands.push([wx, wy]);
  }
  g.lineCap = 'butt';
  return { hands, head: [hx, hy] };
}

// Pose helpers (see drawPerson's arms).
const POSE = {
  down: [[.12, .1], [.12, .1]],
  type: [[.38, -1.2], [.38, -1.2]],
  up: [[2.55, 2.85], [2.55, 2.85]],
  fist: [[.25, 2.9], [2.6, 2.9]],
  clap: [[.35, 2.25], [.35, 2.25]],
  mic: [[.15, .3], [.3, -2.6]],
  point: [[.15, .3], [1.45, 1.6]],
};
function wavePose(ph, t, h) {
  const w = Math.sin(t * 8 + ph) * .25 * h;
  return [[2.3 + w, 2.7 - w], [2.3 - w, 2.7 + w]];
}

// The cup: silver, two handles, a dark plinth.
function drawCup(g, x, y, r, glow) {
  if (glow) { const gl = g.createRadialGradient(x, y - r, 0, x, y - r, r * 3.2); gl.addColorStop(0, 'rgba(255,240,190,.55)'); gl.addColorStop(1, 'rgba(255,240,190,0)'); g.fillStyle = gl; g.fillRect(x - r * 3.2, y - r * 4.2, r * 6.4, r * 6.4); }
  const m = g.createLinearGradient(x - r, 0, x + r, 0);
  m.addColorStop(0, '#8d939e'); m.addColorStop(.35, '#ffffff'); m.addColorStop(.6, '#c9ced8'); m.addColorStop(1, '#6d727c');
  g.fillStyle = '#2a2420'; g.fillRect(x - r * .55, y - r * .25, r * 1.1, r * .3);
  g.fillStyle = m;
  g.beginPath(); g.moveTo(x - r * .35, y - r * .25); g.lineTo(x - r * .15, y - r * .8); g.lineTo(x + r * .15, y - r * .8); g.lineTo(x + r * .35, y - r * .25); g.closePath(); g.fill();
  g.beginPath(); g.moveTo(x - r, y - r * 2.1); g.quadraticCurveTo(x - r * .9, y - r * .9, x, y - r * .82); g.quadraticCurveTo(x + r * .9, y - r * .9, x + r, y - r * 2.1); g.closePath(); g.fill();
  g.strokeStyle = m; g.lineWidth = r * .16;
  g.beginPath(); g.arc(x - r * 1.02, y - r * 1.6, r * .38, Math.PI * .5, Math.PI * 1.5); g.stroke();
  g.beginPath(); g.arc(x + r * 1.02, y - r * 1.6, r * .38, -Math.PI * .5, Math.PI * .5); g.stroke();
  g.fillStyle = 'rgba(255,255,255,.75)'; g.fillRect(x - r * .55, y - r * 2, r * .14, r * .9);
}

export class LolView extends ArenaViewBase {
  constructor(canvas, opts = {}) {
    super(canvas, opts);
    this.lang = 'en'; this.T = T;
    this.source = opts.source;
    this.onUpdate = opts.onUpdate || (() => {});
    this.onEvent = opts.onEvent || (() => {});
    this.match = null; this.state = null; this.queue = []; this.idx = 0;
    this.cur = null; this.prog = 0; this.lastN = 0;
    this.flashes = []; this.particles = []; this.popups = []; this.skulls = []; this.clashes = []; this.killFeed = [];
    this.banner = null; this.mapCache = null; this.now = 0; this.anim = 0; this.clock = 0;
    this.hp = [[1, 1, 1, 1, 1], [1, 1, 1, 1, 1]]; this.planFlash = -10; this.draftAnim = { t0: 0 };
    // Action cam: the current shot, the camera tween, ability effects, a replay.
    this.shot = null; this.camTw = null; this.acts = []; this.pending = []; this.replay = null;
    this.lastShotEnd = -99; this.lastReplay = -999; this.cutFlash = -10; this.hiCache = null; this.hiTimer = null;
    this.dash = [[], []]; this.air = [[], []];
    this.camC = { fX: 0, fY: 0, z: 1 }; this.camT = 0; this.camFocus = [50, 50]; this.zoomed = false;
    // Arena director: the scene on air (arena / crowd / booth / casters / trophy), the ones queued, a pending reaction cut.
    this.scene = null; this.sceneQ = []; this.sceneThen = null; this.pendingCut = null; this.lastCut = -99; this.stinger = -10;
    this.opened = false; this.readyDone = false; this.finaleDone = false; this.draftKind = null; this.draftCuts = 0; this.sceneN = 0;
    this.confetti = []; this.arena = null;
    this.start(); // arena-view-base.js: size, frame loop, observers and the first poll
  }

  destroy() { super.destroy(); clearTimeout(this.hiTimer); }
  setSpeed(x) { super.setSpeed(Math.max(.1, +x || 1)); }
  skipToEnd() {
    if (!this.state) return;
    this.invalidate();
    if (this.cur) this.cur = null;
    while (this.idx < this.queue.length) { const ev = this.queue[this.idx++]; applyEvent(this.state, ev); this.onEvent(ev); }
    this.snapPositions(); this.banner = null; this.flashes = []; this.popups = []; this.clashes = [];
    this.shot = null; this.replay = null; this.camTw = null; this.acts = []; this.pending = [];
    this.scene = null; this.sceneQ = []; this.sceneThen = null; this.pendingCut = null; this.confetti = [];
    this.opened = this.readyDone = this.finaleDone = true;
    this.clock = this.state.t;
    if (this.state.end) this.state.mode = 'victory';
  }
  // Jump to event index (0 = the start): rebuild the state from scratch with
  // events 0..index-1, silently (no banners, shots, replays, arena scenes or
  // onEvent), snap everyone into place and let event index play normally (so
  // onEvent fires once, for the event landed on). Past what has loaded, it
  // stays at the edge (live: waiting for the next event).
  seek(index) {
    const m = this.match;
    if (!m || !m.meta) return false;
    this.invalidate();
    index = clamp(Math.round(+index || 0), 0, this.queue.length);
    const st = createState(m.meta, m.names || (this.state && this.state.names));
    for (let i = 0; i < index; i++) applyEvent(st, this.queue[i]);
    this.state = st; this.idx = index; this.cur = null; this.prog = 0;
    this.since = index > 0 ? Infinity : 0; // the bar: at the start of the event landed on
    this.snapPositions();
    for (let t = 0; t < 2; t++) for (let r = 0; r < 5; r++) this.hp[t][r] = st.target[t][r].alive === false ? 0 : 1;
    this.flashes = []; this.particles = []; this.popups = []; this.skulls = []; this.clashes = []; this.killFeed = [];
    this.banner = null; this.planFlash = -10; this.draftAnim = { t0: -10 };
    this.shot = null; this.camTw = null; this.acts = []; this.pending = []; this.replay = null;
    this.lastShotEnd = -99; this.lastReplay = -999; this.dash = [[], []]; this.air = [[], []];
    this.camC = { fX: 0, fY: 0, z: 1 }; this.camT = 0; this.camFocus = [50, 50]; this.zoomed = false;
    this.scene = null; this.sceneQ = []; this.sceneThen = null; this.pendingCut = null; this.lastCut = -99; this.confetti = [];
    // From the very start the opening plays again; anywhere else the arena cuts stay off until the next moment.
    this.opened = index > 0; this.readyDone = index > 0 && (st.mode !== 'draft' || st.draft.done); this.finaleDone = false;
    this.draftKind = index > 0 ? this.queue[index - 1].kind : null; this.draftCuts = 0;
    this.clock = st.t;
    if (st.end && index >= this.queue.length) st.mode = 'victory';
    this.onUpdate(m, this.queue[index - 1] || null);
    return true;
  }
  // Where playback is: index = events started, total = events loaded.
  getProgress() {
    const done = !!this.match && ['final', 'stopped', 'failed'].includes(this.match.status);
    const tc = this.timesC && this.timesC.len === this.queue.length ? this.timesC : (this.timesC = { len: this.queue.length, times: eventTimes(this.queue, this.match && this.match.meta, this.match && this.match.names) });
    return { index: this.idx, total: this.queue.length, live: !!this.match && !done, events: this.queue, marks: progressMarks(this.queue, false), chapters: [], step: 6, ...elapsedAt(tc.times, this.idx, this.since) };
  }
  isFinished() {
    const m = this.match;
    if (!m || !['final', 'stopped', 'failed'].includes(m.status)) return false;
    if (this.cur || this.replay || this.scene || this.sceneQ.length || this.idx < this.queue.length) return false;
    return m.status !== 'final' || !!(this.state && this.state.mode === 'victory');
  }

  async poll() {
    if (this.stopped) return;
    try {
      const m = await this.source.load(this.lastN);
      if (m) {
        if (!this.match) this.match = { ...m }; else Object.assign(this.match, { ...m, meta: m.meta || this.match.meta });
        if (!this.state && this.match.meta) { this.state = createState(this.match.meta, this.match.names); this.mapCache = null; }
        else if (this.state && Array.isArray(m.names)) this.state.names = m.names;
        for (const e of m.events || []) if (e.n > this.lastN) { this.queue.push(e); this.lastN = e.n; }
        this.onUpdate(this.match, this.queue[this.idx - 1] || null);
        this.invalidate();
      }
    } catch (e) { /* retry */ }
    const done = this.match && ['final', 'stopped', 'failed'].includes(this.match.status);
    if (!done && !this.stopped) this.schedulePoll(3000);
  }

  resize() {
    const r = this.cv.getBoundingClientRect ? this.cv.getBoundingClientRect() : { width: this.cv.width, height: this.cv.height };
    const d = viewDpr();
    this.cv.width = Math.max(320, Math.round(r.width * d)); this.cv.height = Math.max(240, Math.round(r.height * d));
    this.W = this.cv.width; this.H = this.cv.height; this.dpr = d;
    this.mapCache = null; this.hiCache = null; clearTimeout(this.hiTimer); this.hiTimer = null; this.arena = null;
    this.computeLayout();
  }

  computeLayout() {
    const W = this.W, H = this.H;
    this.wide = W / H >= 1.25;
    let s;
    if (this.wide) {
      s = this.s = Math.min(W / 1280, H / 720);
      const top = 62 * s, bottom = 38 * s, pad = 10 * s, mw = W * .68;
      this.topH = top;
      this.mapR = { x: pad, y: top + 6 * s, w: mw - pad, h: H - top - 6 * s - bottom };
      this.tick = { x: pad, y: H - bottom + 2 * s, w: mw - pad, h: bottom - 10 * s };
      this.side = { x: mw + 10 * s, y: top + 8 * s, w: W - mw - 10 * s - pad, h: H - top - 8 * s - pad };
    } else {
      s = this.s = Math.min(W / 640, H / 760);
      const top = 62 * s;
      this.topH = top;
      const mh = Math.min(H * .5, W * .72);
      this.mapR = { x: 6 * s, y: top + 4 * s, w: W - 12 * s, h: mh };
      this.tick = { x: 6 * s, y: this.mapR.y + mh + 2 * s, w: W - 12 * s, h: 28 * s };
      this.side = { x: 6 * s, y: this.tick.y + this.tick.h + 6 * s, w: W - 12 * s, h: H - (this.tick.y + this.tick.h + 12 * s) };
    }
    const R = this.mapR, depth = 12 * s;
    const k = Math.min(R.w / (100 * Math.SQRT2), (R.h - depth) / (100 * Math.SQRT2 * SQ));
    this.mapG = { cx: R.x + R.w / 2, cy: R.y + (R.h - depth) / 2, k, depth };
    this.camView = { cx: this.mapG.cx, cy: this.mapG.cy, ax: R.x + R.w / 2, ay: R.y + R.h * .56, h: R.h };
  }

  // Map coords (0..100, blue bottom-left) → flat screen, tilted 45° so blue is on the left.
  base(x, y) { const G = this.mapG; return [G.cx + (x - y) * G.k * Math.SQRT1_2, G.cy + (x + y - 100) * G.k * SQ * Math.SQRT1_2]; }
  // Map coords → screen through the camera, plus the local scale ([X, Y, scale]).
  PS(x, y) { const b = this.base(x, y); return this.zoomed ? camProject(this.camC, this.camView, b[0], b[1]) : [b[0], b[1], 1]; }
  P(x, y) { const b = this.base(x, y); if (!this.zoomed) return b; const r = camProject(this.camC, this.camView, b[0], b[1]); return [r[0], r[1]]; }
  get u() { return this.mapG.k * (this.zoomed ? this.camC.z : 1); }
  // Pixel size of one map unit at a map point (bigger near the camera).
  uAt(x, y) { return this.mapG.k * this.PS(x, y)[2]; }

  setCam(cam) {
    const f = this.base(cam.x, cam.y);
    this.camC = { fX: f[0], fY: f[1], z: cam.z };
    this.zoomed = cam.z > 1.004; this.camT = camTilt(cam.z); this.camFocus = [cam.x, cam.y];
  }
  camNow() { return camAt(this.camTw, this.anim); }

  // Where a champion is drawn: its position, a little idle wobble, fight
  // footwork and dashes while the action cam watches, spread out around the focus.
  champXY(t, r) {
    const p = this.state.pos[t][r];
    if (!p) return [50, 50];
    if (p.alive === false) return [p.x, p.y];
    const seed = t * 5 + r, a = this.camT, amp = .6 + .9 * a, sp = 1 + 1.4 * a;
    let x = p.x + Math.sin(this.anim * 1.3 * sp + seed * 1.7) * amp, y = p.y + Math.cos(this.anim * 1.1 * sp + seed * 2.3) * amp;
    const ds = this.dash[t][r];
    if (ds) {
      const age = this.anim - ds.at, f = age < 0 ? 0 : age < .1 ? age / .1 : Math.max(0, 1 - (age - .1) / .8);
      x += ds.dx * easeOut(f); y += ds.dy * easeOut(f);
    }
    if (a > 0) {
      const [fx, fy] = this.camFocus, dx = x - fx, dy = y - fy;
      const d = Math.hypot(dx, dy), w = 1.1 * a * Math.max(0, 1 - d / 18);
      x = fx + dx * (1 + w); y = fy + dy * (1 + w);
      // Keep clear of the objective monster's body so it stays visible.
      const sh = this.shot, m = sh && sh.mon;
      if (m && Number.isFinite(m.dieAt) && this.anim < m.dieAt + .4) {
        const R0 = (MON_R[m.kind] || 4.5) * a, ox = x - sh.x, oy = y - sh.y, od = Math.hypot(ox, oy);
        if (od < R0) { const an = od > .01 ? Math.atan2(oy, ox) : seed; x = sh.x + Math.cos(an) * R0; y = sh.y + Math.sin(an) * R0; }
      }
    }
    return [x, y];
  }

  // One frame (the loop itself is in arena-view-base.js).
  onFrame(dt, now) {
    this.now = now / 1000;
    if (!this.paused) this.advance(dt * this.speed, dt);
    try { this.draw(); } catch (e) { /* one bad frame doesn't stop the viewer */ }
  }

  advance(dt) {
    if (!this.state) return;
    // A replay pauses the match: only the replay clock runs.
    this.since = (this.since || 0) + dt;
    if (this.replay) { this.replay.t += dt; if (this.replay.t >= this.replay.len) this.finishReplay(); return; }
    // So does an arena scene: only its own clock (and the confetti) runs.
    if (this.scene) { this.scene.t += dt; this.updateConfetti(dt); if (this.scene.t >= this.scene.dur) this.nextScene(); return; }
    if (this.directArena()) return;
    this.anim += dt;
    if (this.pending.length) this.pending = this.pending.filter(p => this.anim < p.at || (p.fn(), false));
    if (!this.cur && this.idx < this.queue.length && this.state.mode !== 'victory') {
      const ev = this.queue[this.idx++];
      const wasDraft = this.state.mode === 'draft';
      const fx = applyEvent(this.state, ev);
      if (wasDraft && this.state.mode === 'map') this.snapPositions();
      this.cur = { ev, fx }; this.prog = 0; this.since = 0;
      if (fx.planChange) this.planFlash = this.anim;
      const delay = this.direct(ev, fx);
      const show = () => {
        if (fx.banner) this.banner = { ...fx.banner, t0: this.anim };
        for (const f of fx.flashes) this.addFlash(f);
        for (const p of fx.popups) this.popups.push({ ...p, age: 0, life: 1.6 });
        for (const k of fx.skulls) this.skulls.push({ ...k, age: 0, life: 3.5 });
        if (fx.clash) this.clashes.push({ ...fx.clash, age: 0, life: 1.1 });
      };
      if (delay > 0) this.pending.push({ at: this.anim + delay, fn: show }); else show();
      for (const h of fx.hits) { if (h.hp === 0) this.hp[h.team][h.slot] = 0; else this.hp[h.team][h.slot] = clamp(this.hp[h.team][h.slot] - h.dmg * (.5 + Math.random()), .08, 1); }
      if (ev.kind === 'kill') { const f = this.state.feed[0]; if (f) { this.killFeed.unshift({ ...f, born: this.anim }); this.killFeed.length = Math.min(this.killFeed.length, 5); } }
      if (ev.kind === 'draft_ban' || ev.kind === 'draft_pick') this.draftAnim = { t0: this.anim };
      this.react(ev);
      this.onEvent(ev);
      this.onUpdate(this.match, ev);
    }
    if (this.cur) {
      this.prog += dt / this.cur.fx.dur;
      if (this.prog >= 1) {
        const ev = this.cur.ev; this.cur = null;
        if (ev.kind === 'match_end') this.finale(ev);
      }
    }
    const st = this.state;
    const kc = this.cur ? clamp(this.prog * 1.4, 0, 1) : 1;
    this.clock = Math.max(this.clock, lerp(st.clockFrom, st.clockTo, kc));
    // Smooth movement towards the target positions.
    const k = 1 - Math.exp(-dt * 2.6);
    for (let t = 0; t < 2; t++) for (let r = 0; r < 5; r++) {
      const p = st.pos[t][r], q = st.target[t][r];
      if (!p || !q) continue;
      if (q.alive === false && p.alive !== false) {
        if (this.camT > .2) { const [dx, dy] = this.champXY(t, r); this.addAct({ k: 'death', x: dx, y: dy, t, r, life: 1.6 }); }
        p.x = q.x; p.y = q.y;
      }
      if (q.alive !== false && p.alive === false) { const f = FOUNTAIN[mapSide(st.meta, t)]; p.x = f[0]; p.y = f[1]; this.hp[t][r] = 1; }
      p.x = lerp(p.x, q.x, k); p.y = lerp(p.y, q.y, k); p.alive = q.alive;
      if (q.alive !== false) this.hp[t][r] = Math.min(1, this.hp[t][r] + dt * .05);
    }
    const age = arr => arr.filter(f => (f.age += dt) < f.life);
    this.flashes = age(this.flashes); this.popups = age(this.popups); this.skulls = age(this.skulls); this.clashes = age(this.clashes);
    this.particles = this.particles.filter(q => (q.life -= dt) > 0);
    for (const q of this.particles) { q.x += q.vx * dt; q.y += q.vy * dt; q.vx *= .95; q.vy *= .95; }
    for (const a of this.acts) { a.age += dt; if (a.hit && !a.done && a.age >= a.hitAt) { a.done = true; this.impact(a); } }
    this.acts = this.acts.filter(a => a.age < a.life);
    this.updateShot(dt);
    this.rollCut();
  }

  // A pending reaction cut goes on air as soon as the action cam and the replay are done.
  rollCut() {
    if (!this.pendingCut) return false;
    const r = cutReady(this.pendingCut, { now: this.anim, shot: this.shot, replay: this.replay, mode: this.state.mode });
    if (r === 'drop') this.pendingCut = null;
    if (r !== 'now') return false;
    const c = this.pendingCut; this.pendingCut = null; this.lastCut = this.anim; this.playScenes([c]);
    return true;
  }

  // ---- arena director ----

  // Before the next event: the opening, a cut between draft phases, the teams
  // ready before the map. Returns true when a scene took the air.
  directArena() {
    const st = this.state, fast = this.speed > 3.01;
    if (!this.opened) { this.opened = true; if (!fast && !this.cur && st.mode !== 'victory') { this.playScenes(openingScenes(st)); return true; } }
    // Before the next event can start a new shot, the moment that just ended gets its reaction.
    if (!this.cur && this.rollCut()) return true;
    const nx = !this.cur && this.queue[this.idx];
    if (!nx || st.mode !== 'draft') return false;
    if (nx.kind === 'draft_ban' || nx.kind === 'draft_pick') {
      const c = fast ? null : draftCut(this.draftKind, nx, this.draftCuts);
      this.draftKind = nx.kind;
      if (c) { this.draftCuts++; this.playScenes([c]); return true; }
      return false;
    }
    const isDraft = nx.kind === 'phase' && ['draft', 'draft_done'].includes((nx.data || {}).phase);
    if (st.draft.done && !isDraft && !this.readyDone) { this.readyDone = true; if (!fast) { this.playScenes(readyScenes(st)); return true; } }
    return false;
  }

  // A big moment: the crowd reacts (a quick cut once the action cam is done).
  react(ev) {
    if (this.state.mode !== 'map') return;
    const r = crowdReaction(this.state, ev);
    const c = reactionCut(r, { sinceCut: this.anim - this.lastCut, speed: this.speed });
    if (c && (!this.pendingCut || c.level >= this.pendingCut.level)) this.pendingCut = { ...c, at: this.anim };
  }

  // The nexus fell and the match is over: eruption, booth, trophy, then the stats.
  finale(ev) {
    if (this.finaleDone || this.speed > 3.01) { this.state.mode = 'victory'; return; }
    this.finaleDone = true;
    this.shot = null; this.replay = null; this.camTw = null; this.acts = []; this.pendingCut = null;
    const d = (ev && ev.data) || this.state.end || {};
    this.playScenes(finaleScenes(d.winner), () => { this.state.mode = 'victory'; this.confetti = []; });
  }

  playScenes(list, then) {
    this.sceneQ.push(...list);
    if (then) this.sceneThen = then;
    if (!this.scene) this.nextScene();
  }

  nextScene() {
    const sc = this.sceneQ.shift() || null;
    this.stinger = this.now;
    if (sc) {
      const order = this.order();
      this.scene = { ...sc, t: 0, n: ++this.sceneN, pos: sc.team >= 0 ? order.indexOf(sc.team) : -1 };
      return;
    }
    this.scene = null;
    const then = this.sceneThen; this.sceneThen = null;
    if (then) then();
  }

  updateConfetti(dt) {
    const sc = this.scene;
    if (sc && sc.confetti && this.confetti.length < 420) {
      const cols = [teamColor(this.state.meta, sc.team), LED_GOLD, '#ffffff', shade(teamColor(this.state.meta, sc.team), .4)];
      for (let i = 0; i < 9; i++) this.confetti.push({ x: Math.random() * AW, y: -20 - Math.random() * 60, vx: (Math.random() - .5) * 40, vy: 70 + Math.random() * 90, r: Math.random() * TAU, vr: (Math.random() - .5) * 10, c: cols[i % cols.length], s: 4 + Math.random() * 5 });
    }
    for (const q of this.confetti) { q.x += (q.vx + Math.sin(q.r) * 25) * dt; q.y += q.vy * dt; q.r += q.vr * dt; }
    this.confetti = this.confetti.filter(q => q.y < AH + 30);
  }

  // ---- action cam director ----

  // Decides the camera for a new event; returns how long to hold back the
  // event's banner and flashes (an objective dies on camera first).
  direct(ev, fx) {
    const st = this.state;
    if (st.mode !== 'map' || this.speed > 3.01) return 0;
    const shot = shotFor(st, ev), now = this.anim, cur = this.shot, d = ev.data || {};
    const act = shotAction(cur, shot, now - this.lastShotEnd);
    if (act === 'none' || act === 'skip') {
      // Something happens far away: a shot that has had its moment ends early.
      if (cur && Number.isFinite(d.x) && Math.hypot(d.x - cur.x, d.y - cur.y) > 22 && now - cur.t0 > 2.2) cur.until = Math.min(cur.until, now + .35);
      return 0;
    }
    let s = shot;
    if (act === 'merge') {
      s = cur;
      s.until = Math.max(s.until, now + shot.hold);
      if (shot.prio >= s.prio) { s.label = shot.label; s.prio = shot.prio; s.kind = shot.kind === 'fight' ? s.kind : shot.kind; }
      s.replay = s.replay || shot.replay;
      if (shot.replay) s.replayAt = now;
      if (shot.mon && !s.mon) { s.mon = shot.mon; s.x = shot.x; s.y = shot.y; s.team = shot.team; }
      if (shot.struct && !s.struct) s.struct = shot.struct;
      if (shot.mon && s.mon === shot.mon) this.armDrain(s, s.mon, 1.5, .55);
      if (shot.struct && s.struct === shot.struct) { s.struct.x = shot.x; s.struct.y = shot.y; this.armDrain(s, s.struct, 1.3, .5); }
    } else {
      if (act === 'pan' || act === 'cut') this.endShot(false);
      this.startShot(s, act === 'cut');
    }
    if (ev.kind === 'kill') this.killingBlow(d);
    const target = s.mon && shot.mon === s.mon ? s.mon : s.struct && shot.struct === s.struct ? s.struct : null;
    if (act !== 'merge') this.cur.fx.dur += .6;
    if (target) { const left = target.dieAt - now; this.cur.fx.dur = Math.max(this.cur.fx.dur, left + 1); return left; }
    return 0;
  }

  armDrain(s, o, wait, hp0) {
    o.t0 = this.anim; o.dieAt = this.anim + wait; o.hp0 = hp0;
    s.until = Math.max(s.until, o.dieAt + 1.4);
    if (o === s.mon) this.pending.push({ at: o.dieAt, fn: () => {
      this.addAct({ k: 'smite', x: s.x, y: s.y, life: .55 });
      this.addAct({ k: 'num', x: s.x, y: s.y, text: o.stolen ? 'STOLEN!' : 'SMITE', col: o.stolen ? teamColor(this.state.meta, s.team) : '#ffd23f', big: true, life: 1.4, dx: 0 });
    } });
  }

  startShot(s, cut) {
    const now = this.anim, to = shotCam(s);
    s.t0 = now; s.until = now + .75 + s.hold; if (s.replay) s.replayAt = now + 1.2; s.frames = []; s.acc = 0; s.beam = 0; s.monAcc = 0;
    if (cut) { this.camTw = { from: to, to, t0: now, dur: 0 }; this.cutFlash = this.now; }
    else this.camTw = { from: this.camNow(), to, t0: now, dur: .75 };
    this.shot = s;
    if (s.mon) this.armDrain(s, s.mon, 1.6, .55);
    if (s.struct) { s.struct.x = s.x; s.struct.y = s.y; this.armDrain(s, s.struct, 1.4, .5); }
  }

  // The shot is over: a replay if it earned one, else back to the full map.
  endShot(allowReplay) {
    const s = this.shot;
    if (!s) return;
    this.shot = null; this.lastShotEnd = this.anim;
    const cam = shotCam(s);
    // The replay shows the moment that earned it: ~3 s before, ~1 s after.
    const at = Number.isFinite(s.replayAt) ? s.replayAt : this.anim;
    const frames = (s.frames || []).filter(f => f.anim >= at - 3.2 && f.anim <= at + 1.3);
    if (allowReplay && s.replay && frames.length > 24 && this.anim - this.lastReplay > 25) {
      this.replay = { frames, t: 0, rate: .6, len: (frames.length - 1) * .05 / .6, cam, shot: s, label: s.label, where: s.where };
      this.lastReplay = this.anim; this.cutFlash = this.now;
      return;
    }
    this.camTw = { from: this.camNow(), to: { ...FULL_CAM }, t0: this.anim, dur: .9 };
  }

  finishReplay() {
    const r = this.replay;
    this.replay = null; this.cutFlash = this.now;
    this.camTw = { from: r.cam, to: { ...FULL_CAM }, t0: this.anim, dur: 1 };
  }

  updateShot(dt) {
    const s = this.shot;
    if (this.state.mode !== 'map') { if (s || this.camTw) { this.shot = null; this.camTw = null; this.acts = []; } return; }
    if (!s) return;
    if (this.anim >= s.until) { this.endShot(true); return; }
    if (this.anim - s.t0 > .6 && this.anim - (s.rec || 0) >= .05) {
      s.rec = this.anim; s.frames.push(this.snapshot());
      if (s.frames.length > 200) s.frames.shift();
    }
    if (this.anim - s.t0 < .3) return;
    const rate = s.label === 'TEAMFIGHT' || s.prio >= 4 ? 7 : s.kind === 'objective' ? 5 : s.kind === 'struct' ? 4 : 3.5;
    s.acc += dt * rate;
    while (s.acc >= 1) { s.acc -= .8 + Math.random() * .4; this.castAbility(s); }
    // Towers shoot back; big monsters slam.
    const now = this.anim;
    if (s.struct && s.struct.kind === 'tower' && now < s.struct.dieAt && (s.beam += dt) > .7) { s.beam = 0; this.towerShot(s); }
    if (s.mon && now < s.mon.dieAt && s.mon.kind !== 'grubs' && (s.monAcc += dt) > .9) { s.monAcc = 0; this.monsterSlam(s); }
  }

  snapshot() {
    const cp = a => a.map(o => ({ ...o }));
    return {
      anim: this.anim, clock: this.clock,
      pos: this.state.pos.map(a => a.map(p => ({ x: p.x, y: p.y, alive: p.alive }))),
      hp: this.hp.map(a => a.slice()), acts: cp(this.acts), popups: cp(this.popups), flashes: cp(this.flashes),
      skulls: cp(this.skulls), clashes: cp(this.clashes), particles: cp(this.particles),
      dash: this.dash.map(a => a.map(o => o && { ...o })), air: this.air.map(a => a.slice()),
    };
  }

  addAct(a) {
    a.age = -(a.delay || 0);
    this.acts.push(a);
    if (this.acts.length > 180) this.acts.splice(0, this.acts.length - 180);
  }

  // Champions in the shot: [{t, r, x, y}] (alive, near the focus).
  fighters(s) {
    const st = this.state, out = [];
    for (let t = 0; t < 2; t++) for (let r = 0; r < 5; r++) {
      const p = st.pos[t][r];
      if (!p || p.alive === false) continue;
      const [x, y] = this.champXY(t, r);
      if (Math.hypot(x - s.x, y - s.y) < 17) out.push({ t, r, x, y });
    }
    return out;
  }

  // One procedural ability, by champion class: skill shots, AoE circles,
  // dashes, slams, hooks, shields and heals.
  castAbility(s) {
    const st = this.state, all = this.fighters(s);
    if (!all.length) return;
    const pick = a => a[Math.floor(Math.random() * a.length)];
    const c = pick(all), now = this.anim;
    const info = champInfo(st.meta, (st.line[c.t][c.r] || {}).champ), cls = info.class || 'bruiser';
    const col = shade(CLASS_COLORS[cls] || '#cccccc', .3), team = teamColor(st.meta, c.t);
    const ap = /AP/i.test(info.dmg || '') || cls === 'mage' || cls === 'enchanter';
    const dcol = ap ? '#8ec5ff' : '#ffb35c';
    const foes = all.filter(e => e.t !== c.t), friends = all.filter(e => e.t === c.t);
    const R = (lo, hi) => Math.round(lo + Math.random() * (hi - lo));
    let tg = null;
    if (s.mon && now < s.mon.dieAt && c.t === s.team && (!foes.length || Math.random() < .75)) tg = { x: s.x, y: s.y, mon: true };
    else if (s.struct && now < s.struct.dieAt && mapSide(st.meta, c.t) !== s.struct.side && (!foes.length || Math.random() < .6)) tg = { x: s.struct.x, y: s.struct.y, struct: true };
    else if (foes.length) { const f = pick(foes); tg = { x: f.x, y: f.y, t: f.t, r: f.r }; }
    const hitOf = (dmg, crit) => tg.mon || tg.struct ? { obj: true, dmg, crit } : { targets: [{ t: tg.t, r: tg.r }], dmg, crit };
    const area = (r, dmg) => tg.mon || tg.struct ? { obj: true, dmg } : { area: r, foeOf: c.t, dmg };
    const shield = who => { this.addAct({ k: 'shield', t: who.t, r: who.r, col: '#ffe08a', life: 1.6 }); this.addAct({ k: 'num', x: who.x, y: who.y, text: `+${R(150, 320)}`, col: '#ffe08a', life: .9, dx: 0 }); };
    if (!tg) { if (cls === 'enchanter' || cls === 'tank') shield(c); return; }
    const proj = (style, dmg, crit, extra = {}) => {
      const life = .14 + Math.hypot(tg.x - c.x, tg.y - c.y) * .03;
      this.addAct({ k: 'proj', style, x0: c.x, y0: c.y, x1: tg.x, y1: tg.y, col, team, life, hitAt: life, hit: hitOf(dmg, crit), dcol, ...extra });
    };
    const dashTo = frac => {
      const dx = (tg.x - c.x) * frac, dy = (tg.y - c.y) * frac;
      this.dash[c.t][c.r] = { dx, dy, at: now };
      this.addAct({ k: 'dash', x0: c.x, y0: c.y, x1: c.x + dx, y1: c.y + dy, t: c.t, r: c.r, col: team, life: .4 });
    };
    switch (cls) {
      case 'mage':
        if (Math.random() < .45) this.addAct({ k: 'aoe', x: tg.x, y: tg.y, r: 2.6, col, team, life: 1.05, hitAt: .55, hit: area(2.6, R(220, 420)), dcol });
        else proj('orb', R(180, 380), false);
        break;
      case 'marksman':
        for (let i = 0; i < 3; i++) { const crit = Math.random() < .3; proj('arrow', crit ? R(330, 540) : R(110, 210), crit, { delay: i * .16 }); }
        break;
      case 'assassin':
        dashTo(.8);
        this.addAct({ k: 'slash', x: tg.x, y: tg.y, col: '#ffffff', team, life: .45, hitAt: .12, hit: hitOf(R(260, 520), Math.random() < .35), dcol, ang: Math.random() * TAU });
        break;
      case 'bruiser':
        dashTo(.55);
        this.addAct({ k: 'spin', x: c.x + (tg.x - c.x) * .55, y: c.y + (tg.y - c.y) * .55, r: 2.3, col, team, life: .55, hitAt: .15, hit: area(2.4, R(160, 320)), dcol });
        break;
      case 'engage':
        if (Math.random() < .5 && !tg.mon && !tg.struct) proj('hook', R(120, 220), false, { knock: true });
        else { dashTo(.7); this.addAct({ k: 'slam', x: tg.x, y: tg.y, r: 3, col, team, life: .75, hitAt: .12, hit: { ...area(3, R(140, 260)), knock: true }, dcol }); }
        break;
      case 'tank':
        if (Math.random() < .35) shield(c);
        else this.addAct({ k: 'slam', x: tg.x, y: tg.y, r: 3, col, team, life: .75, hitAt: .1, hit: { ...area(3, R(120, 240)), knock: true }, dcol });
        break;
      case 'enchanter':
        if (friends.length && Math.random() < .55) {
          const f = pick(friends);
          if (Math.random() < .5) shield(f);
          else { this.addAct({ k: 'heal', t: f.t, r: f.r, life: 1 }); this.hp[f.t][f.r] = Math.min(1, this.hp[f.t][f.r] + .12); this.addAct({ k: 'num', x: f.x, y: f.y, text: `+${R(140, 280)}`, col: '#62ff96', life: .9, dx: 0 }); }
        } else proj('orb', R(90, 200), false);
        break;
      default: proj('orb', R(150, 300), false);
    }
  }

  // An ability lands: damage numbers, HP bars and knock-ups.
  impact(a) {
    const h = a.hit, st = this.state;
    let x = a.x1 != null ? a.x1 : a.x, y = a.y1 != null ? a.y1 : a.y;
    if (a.k === 'beam') [x, y] = this.champXY(a.t, a.r);
    const num = (px, py, v, col, big) => this.addAct({ k: 'num', x: px, y: py, text: String(v), col, big, life: .95, dx: (Math.random() - .5) * 2 });
    this.addAct({ k: 'spark', x, y, col: a.col || '#fff', life: .35 });
    if (h.obj) { num(x, y, `${h.dmg}${h.crit ? '!' : ''}`, h.crit ? '#ffd23f' : '#ffffff', h.crit); return; }
    let targets = h.targets || [];
    if (h.area) {
      targets = [];
      for (let r = 0; r < 5; r++) {
        const t = 1 - h.foeOf, p = st.pos[t][r];
        if (!p || p.alive === false) continue;
        const [px, py] = this.champXY(t, r);
        if (Math.hypot(px - x, py - y) < h.area + 1) targets.push({ t, r });
      }
    }
    for (const tg of targets) {
      const p = st.pos[tg.t][tg.r];
      if (!p || p.alive === false) continue;
      const [px, py] = this.champXY(tg.t, tg.r);
      this.hp[tg.t][tg.r] = Math.max(.1, this.hp[tg.t][tg.r] - h.dmg / 2400);
      num(px, py, `${h.dmg}${h.crit ? '!' : ''}`, h.crit ? '#ffd23f' : a.dcol || '#ffb35c', h.crit);
      if (h.knock || a.knock) this.air[tg.t][tg.r] = this.anim;
    }
  }

  // The killing blow of a kill event, filmed: a fast strike killer → victim.
  killingBlow(d) {
    const k = d.killer || {}, v = d.victim || {};
    if (!(k.side === 0 || k.side === 1) || !(v.side === 0 || v.side === 1)) return;
    const st = this.state, kp = st.pos[k.side][k.slot | 0], vp = st.pos[v.side][v.slot | 0];
    if (!kp || !vp || kp.alive === false) return;
    const [x0, y0] = this.champXY(k.side, k.slot | 0), [x1, y1] = vp.alive === false ? [vp.x, vp.y] : this.champXY(v.side, v.slot | 0);
    const team = teamColor(st.meta, k.side);
    this.addAct({ k: 'proj', style: 'orb', x0, y0, x1, y1, col: '#fff2c0', team, life: .18, big: true });
    this.addAct({ k: 'slash', x: x1, y: y1, col: '#ffffff', team, life: .55, ang: Math.random() * TAU, big: true });
    this.addAct({ k: 'num', x: x1, y: y1, text: `${Math.round(450 + Math.random() * 500)}!`, col: '#ff4d4d', big: true, life: 1.1, dx: 0 });
  }

  towerShot(s) {
    const st = this.state, foes = this.fighters(s).filter(f => mapSide(st.meta, f.t) !== s.struct.side);
    if (!foes.length) return;
    const f = foes[Math.floor(Math.random() * foes.length)];
    this.addAct({ k: 'beam', x0: s.struct.x, y0: s.struct.y, t: f.t, r: f.r, col: SIDE_COLORS[s.struct.side][1], life: .32, hitAt: .06, hit: { targets: [{ t: f.t, r: f.r }], dmg: Math.round(250 + Math.random() * 200) }, dcol: '#ffffff' });
  }

  monsterSlam(s) {
    const foes = this.fighters(s);
    if (!foes.length) return;
    const f = foes[Math.floor(Math.random() * foes.length)];
    this.addAct({ k: 'slam', x: f.x, y: f.y, r: 3.4, col: s.mon.col, team: s.mon.col, life: .8, hitAt: .3, hit: { area: 3.4, foeOf: 1 - f.t, dmg: Math.round(200 + Math.random() * 250) }, dcol: '#d9b8ff' });
  }

  snapPositions() {
    for (let t = 0; t < 2; t++) for (let r = 0; r < 5; r++) { const q = this.state.target[t][r]; this.state.pos[t][r] = { x: q.x, y: q.y, alive: q.alive }; }
  }

  addFlash(f) {
    if (!Number.isFinite(f.x) || !Number.isFinite(f.y)) return;
    this.flashes.push({ ...f, age: 0, life: f.big ? 1.3 : .9 });
    const n = f.debris ? 30 : f.big ? 20 : 10;
    for (let i = 0; i < n; i++) {
      const a = Math.random() * TAU, v = (f.big ? 10 : 6) * (.4 + Math.random());
      this.particles.push({ x: f.x, y: f.y, vx: Math.cos(a) * v, vy: Math.sin(a) * v, life: .5 + Math.random() * .8, c: f.debris && Math.random() < .5 ? '#8a7a66' : f.color });
    }
  }

  // ---- text helpers ----

  font(size, weight = 700, italic = false) { return `${italic ? 'italic ' : ''}${weight} ${Math.max(1, size)}px ${FONT}`; }
  txt(str, x, y, size, color, align = 'left', weight = 700, base = 'middle') {
    const g = this.ctx; g.font = this.font(size, weight); g.fillStyle = color; g.textAlign = align; g.textBaseline = base; g.fillText(str, x, y);
  }
  fit(text, w, size, weight) { const g = this.ctx; g.font = this.font(size, weight); let s = String(text == null ? '' : text); if (g.measureText(s).width <= w) return s; while (s.length > 2 && g.measureText(s + '…').width > w) s = s.slice(0, -1); return s + '…'; }
  panel(x, y, w, h, r, fill = 'rgba(9,13,24,.88)', stroke = 'rgba(255,255,255,.07)') {
    const g = this.ctx; g.fillStyle = fill; g.beginPath(); g.roundRect(x, y, w, h, r); g.fill();
    if (stroke) { g.strokeStyle = stroke; g.lineWidth = Math.max(1, this.s); g.stroke(); }
  }

  names() {
    const m = this.match || {}, meta = this.state.meta;
    const n = m.names || this.state.names || [];
    return [0, 1].map(i => n[i] || (meta.teams && meta.teams[i] && meta.teams[i].name) || `Team ${i + 1}`);
  }
  order() { const b = blueTeam(this.state.meta); return [b, 1 - b]; }

  // ---- main draw ----

  draw() {
    const g = this.ctx, W = this.W, H = this.H;
    const bg = g.createLinearGradient(0, 0, 0, H);
    bg.addColorStop(0, '#0b1120'); bg.addColorStop(1, '#04060b');
    g.fillStyle = bg; g.fillRect(0, 0, W, H);
    if (!this.state) { this.txt(T.pregame, W / 2, H / 2, 18 * Math.min(W / 1280, H / 720) + 6, 'rgba(255,255,255,.8)', 'center', 600); return; }
    if (this.scene) this.drawScene();
    else if (this.state.mode === 'draft') this.drawDraft();
    else {
      const rp = this.replay;
      if (this.state.mode !== 'map') this.setCam(FULL_CAM);
      else if (rp) { const k = rp.len > 0 ? rp.t / rp.len : 1; this.setCam({ ...rp.cam, z: rp.cam.z * (1 + .1 * easeInOut(k)) }); }
      else this.setCam(this.camNow());
      if (rp) this.drawReplayMap(); else this.drawMap();
      this.drawCamOverlay();
      this.drawMapOverlays();
      this.drawTicker();
      this.drawSidePanel();
      this.drawTopBar();
      this.drawBanner();
      if (this.state.mode === 'victory') this.drawVictory();
    }
    this.drawStinger();
    if (!this.scene && !this.cur && this.idx >= this.queue.length && this.match && this.match.status !== 'final' && this.state.mode !== 'victory') {
      const s = this.s;
      this.txt(this.match.status === 'pregame' ? T.pregame : T.waiting, W - 12 * s, H - 8 * s, 12 * s, 'rgba(255,255,255,.6)', 'right', 600, 'bottom');
    }
  }

  // ---- draft ----

  drawDraft() {
    const g = this.ctx, W = this.W, H = this.H, st = this.state;
    const s = Math.min(W / 1280, H / 720) || 1;
    const order = this.order(), names = this.names();
    const narrow = W / H < 1.2;
    // Background: team light on each side + diagonal stripes.
    for (let k = 0; k < 2; k++) {
      const c = teamColor(st.meta, order[k]);
      const rg = g.createRadialGradient(k ? W : 0, H * .55, 0, k ? W : 0, H * .55, W * .62);
      rg.addColorStop(0, rgba(c, .32)); rg.addColorStop(1, rgba(c, 0));
      g.fillStyle = rg; g.fillRect(0, 0, W, H);
    }
    g.save(); g.globalAlpha = .05; g.strokeStyle = '#fff'; g.lineWidth = 2 * s;
    for (let x = -H; x < W; x += 34 * s) { g.beginPath(); g.moveTo(x, H); g.lineTo(x + H, 0); g.stroke(); }
    g.restore();
    const fs = narrow ? Math.min(W / 30, H / 40) : 16 * s;
    const act = st.draft.active, ac = act ? teamColor(st.meta, act.team) : '#e8d9a8';
    const da = clamp((this.anim - this.draftAnim.t0) / .45, 0, 1);
    // Header.
    this.txt(T.draft, W / 2, fs * 1.6, fs * 1.5, '#f2d98d', 'center', 900);
    const step = st.draft.done ? 'ALL PICKS LOCKED' : act ? `${mapSide(st.meta, act.team) === 0 ? 'BLUE' : 'RED'} ${act.kind === 'ban' ? 'BANS' : 'PICKS'}${st.draft.step ? ' · ' + st.draft.step.toUpperCase() : ''}` : 'GET READY';
    g.font = this.font(fs * .8, 800); const sw = g.measureText(step).width + fs * 1.6;
    this.panel(W / 2 - sw / 2, fs * 2.6, sw, fs * 1.5, fs * .75, rgba(ac, .25), ac);
    this.txt(step, W / 2, fs * 3.38, fs * .8, '#fff', 'center', 800);
    const colW = narrow ? W * .47 : W * .35;
    const yTop = narrow ? fs * 9 : fs * 9.4;
    const slotH = Math.min((H - yTop - fs * 1.2) / 5.25, colW * .26);
    for (let k = 0; k < 2; k++) {
      const team = order[k], c = teamColor(st.meta, team), left = k === 0;
      const x0 = left ? fs * 1.2 : W - fs * 1.2 - colW, ax = left ? x0 : x0 + colW, al = left ? 'left' : 'right';
      this.txt(this.fit(names[team], colW, fs * 1.7, 900), ax, fs * 5.1, fs * 1.7, c, al, 900);
      this.txt(T.side[k], ax, fs * 6.4, fs * .7, 'rgba(255,255,255,.55)', al, 800);
      // Bans.
      const bans = st.draft.bans[team], br = Math.min(fs * 1.2, colW / 12.5);
      const bx0 = left ? x0 + colW - br : x0 + br;
      this.txt('BANS', left ? bx0 - br * 9.6 - br * 1.4 : bx0 + br * 9.6 + br * 1.4, fs * 5.75, fs * .6, 'rgba(255,255,255,.45)', left ? 'right' : 'left', 800);
      for (let i = 0; i < 5; i++) {
        const bx = left ? bx0 - (4 - i) * br * 2.4 : bx0 + (4 - i) * br * 2.4, by = fs * 5.75;
        const isAct = this.isActive(team, 'ban', i);
        if (bans[i]) drawPortrait(g, bx, by + (isAct ? (1 - easeOut(da)) * -br * 2 : 0), br * (isAct ? 1 + .3 * (1 - da) : 1), champInfo(st.meta, bans[i]), '#555', { banned: true, glow: isAct, slash: isAct ? da : 1 });
        else { g.strokeStyle = 'rgba(255,255,255,.18)'; g.lineWidth = 1.5 * s; g.beginPath(); g.arc(bx, by, br, 0, TAU); g.stroke(); }
      }
      // Picks.
      const pl = (st.meta.teams && st.meta.teams[team] && st.meta.teams[team].players) || [];
      const nextSlot = st.draft.picks[team].length;
      for (let i = 0; i < 5; i++) {
        const p = st.draft.picks[team][i], active = this.isActive(team, 'pick', i);
        const y = yTop + i * slotH * 1.05;
        const slide = active ? (1 - easeOut(da)) * colW * .4 * (left ? -1 : 1) : 0;
        g.save(); g.translate(slide, 0); g.globalAlpha = active ? .35 + .65 * da : 1;
        const grd = g.createLinearGradient(x0, 0, x0 + colW, 0);
        grd.addColorStop(left ? 0 : 1, rgba(c, p ? (active ? .65 : .32) : .1)); grd.addColorStop(left ? 1 : 0, 'rgba(255,255,255,.02)');
        g.fillStyle = grd; g.beginPath(); g.roundRect(x0, y, colW, slotH, 8 * s); g.fill();
        g.fillStyle = c; g.fillRect(left ? x0 : x0 + colW - 5 * s, y, 5 * s, slotH);
        const waiting = !p && !st.draft.done && act && act.team === team && i === nextSlot && act.kind === 'pick';
        if (active || waiting) { g.strokeStyle = c; g.lineWidth = 2 * s; g.globalAlpha = waiting ? .4 + .4 * Math.sin(this.now * 6) : 1; g.beginPath(); g.roundRect(x0, y, colW, slotH, 8 * s); g.stroke(); g.globalAlpha = 1; }
        const pr = slotH * .4, px = left ? x0 + slotH * .62 : x0 + colW - slotH * .62, cy = y + slotH / 2;
        if (p) {
          drawPortrait(g, px, cy, pr * (active ? 1 + .25 * (1 - da) : 1), champInfo(st.meta, p.champ), c, { glow: active });
          const tx = left ? px + pr * 1.5 : px - pr * 1.5;
          const nameW = colW - slotH * .62 - pr * 1.5 - slotH * 1.05 - 16 * s;
          this.txt(this.fit(p.champ.toUpperCase(), nameW, slotH * .25, 900), tx, y + slotH * .36, slotH * .25, '#fff', al, 900);
          const roleIdx = ROLES.indexOf(p.role), handle = roleIdx >= 0 && pl[roleIdx] ? pl[roleIdx].handle : '';
          this.txt(`${handle}${handle ? ' · ' : ''}${cap(p.cls || '')}`, tx, y + slotH * .7, slotH * .19, 'rgba(255,255,255,.65)', al, 600);
          // Role chip.
          const rl = T.roleShort[p.role] || (p.role || '').toUpperCase();
          g.font = this.font(slotH * .2, 900); const rw = g.measureText(rl).width + slotH * .3;
          const rx = left ? x0 + colW - rw - 10 * s : x0 + 10 * s;
          this.panel(rx, y + slotH * .3, rw, slotH * .36, slotH * .1, rgba(c, .35), rgba(c, .9));
          this.txt(rl, rx + rw / 2, y + slotH * .48, slotH * .2, '#fff', 'center', 900);
          if (active && da < 1) { g.fillStyle = `rgba(255,255,255,${(1 - da) * .5})`; g.beginPath(); g.roundRect(x0, y, colW, slotH, 8 * s); g.fill(); }
        } else {
          g.strokeStyle = 'rgba(255,255,255,.22)'; g.lineWidth = 1.5 * s; g.setLineDash([4 * s, 4 * s]);
          g.beginPath(); g.arc(px, cy, pr, 0, TAU); g.stroke(); g.setLineDash([]);
          this.txt(waiting ? 'PICKING…' : `PICK ${i + 1}`, left ? px + pr * 1.5 : px - pr * 1.5, cy, slotH * .22, waiting ? c : 'rgba(255,255,255,.35)', al, 800);
        }
        g.restore();
      }
    }
    // Centre: turn timer and current action.
    const cx = W / 2, cy = narrow ? H - fs * 3.5 : H * .55, cr = narrow ? fs * 2.2 : Math.min(fs * 3.4, W * .08);
    const left = this.cur ? 1 - this.prog : 0;
    g.strokeStyle = 'rgba(255,255,255,.12)'; g.lineWidth = 6 * s; g.beginPath(); g.arc(cx, cy, cr, 0, TAU); g.stroke();
    g.strokeStyle = ac; g.lineCap = 'round'; g.beginPath(); g.arc(cx, cy, cr, -Math.PI / 2, -Math.PI / 2 + TAU * left); g.stroke(); g.lineCap = 'butt';
    if (st.draft.done) this.txt('GO', cx, cy, cr * .6, '#f2d98d', 'center', 900);
    else {
      this.txt(String(Math.ceil(27 * left) + (this.cur ? 3 : 0)), cx, cy - cr * .1, cr * .62, '#fff', 'center', 900);
      if (act) this.txt(act.kind === 'ban' ? T.ban : T.pick, cx, cy + cr * .45, cr * .26, ac, 'center', 900);
    }
    if (!narrow) {
      this.txt('VS', cx, cy - cr * 1.75, fs * 2.2, 'rgba(242,217,141,.9)', 'center', 900);
      if (this.cur) {
        const cap0 = caption(st, this.cur.ev);
        const lines = this.wrap(cap0, colW * .75, fs * .85, 700);
        lines.slice(0, 3).forEach((l, i) => this.txt(l, cx, cy + cr * 1.6 + i * fs * 1.1, fs * .85, 'rgba(255,255,255,.85)', 'center', 700));
      }
    }
    if (st.draft.done && this.cur) {
      const k = easeOut(clamp(this.prog * 2, 0, 1));
      g.save(); g.globalAlpha = Math.min(1, (1 - this.prog) * 3);
      g.fillStyle = 'rgba(4,6,12,.6)'; g.fillRect(0, H * .42, W, H * .16);
      this.txt(T.lockedIn, cx, H * .5, fs * 3 * (.8 + .2 * k), '#f2d98d', 'center', 900);
      g.restore();
    }
  }

  wrap(text, w, size, weight) {
    const g = this.ctx; g.font = this.font(size, weight);
    const out = []; let line = '';
    for (const word of String(text || '').split(' ')) { const t = line ? line + ' ' + word : word; if (g.measureText(t).width > w && line) { out.push(line); line = word; } else line = t; }
    if (line) out.push(line);
    return out;
  }

  isActive(team, kind, slot) { const a = this.state.draft.active; return !!(this.cur && a && a.team === team && a.kind === kind && a.slot === slot); }

  // ---- map ----

  // The map texture; scale > 1 builds the sharper one the action cam zooms into.
  buildMapCache(scale = .9) {
    if (typeof document === 'undefined') return null;
    const N = Math.round(clamp(this.mapG.k * 100 * scale, 300, scale > 1 ? 2048 : 1400));
    const c = document.createElement('canvas'); c.width = N; c.height = N;
    const g = c.getContext('2d'), m = this.state.map, P = v => v / 100 * N;
    const R = rng(42), L = m.lanes || defaultMap().lanes;
    // Ground.
    const bg = g.createLinearGradient(0, N, N, 0);
    bg.addColorStop(0, '#1c3d27'); bg.addColorStop(.5, '#16331f'); bg.addColorStop(1, '#2a2a22');
    g.fillStyle = bg; g.fillRect(0, 0, N, N);
    for (let i = 0; i < 420; i++) { g.fillStyle = `rgba(${R() < .5 ? '10,32,18' : '48,84,46'},${.18 + R() * .25})`; g.beginPath(); g.arc(R() * N, R() * N, N * (.008 + R() * .03), 0, TAU); g.fill(); }
    // River.
    const poly = (pts, w, col) => { g.strokeStyle = col; g.lineWidth = w; g.lineCap = 'round'; g.lineJoin = 'round'; g.beginPath(); pts.forEach((p, i) => i ? g.lineTo(P(p[0]), P(p[1])) : g.moveTo(P(p[0]), P(p[1]))); g.stroke(); };
    poly(m.river, N * .12, '#14394a'); poly(m.river, N * .09, '#1b5a72'); poly(m.river, N * .05, '#2a7894'); poly(m.river, N * .012, 'rgba(160,230,255,.35)');
    for (let i = 0; i < 60; i++) { const t = R(), p = lanePoint(m.river, t); g.strokeStyle = 'rgba(200,240,255,.25)'; g.lineWidth = N * .002; g.beginPath(); g.moveTo(P(p[0] - 1 + R() * 2), P(p[1] + R() * 3)); g.lineTo(P(p[0] + 1 + R() * 2), P(p[1] + R() * 3)); g.stroke(); }
    // Jungle trees: canopy clusters away from lanes, river, bases and pits.
    const bases = [[12, 88], [88, 12]];
    const blocked = (x, y) => polyDist(x, y, L.top) < 5 || polyDist(x, y, L.mid) < 5 || polyDist(x, y, L.bot) < 5 || polyDist(x, y, m.river) < 6.5 ||
      bases.some(b => Math.hypot(x - b[0], y - b[1]) < 19) || Math.hypot(x - m.dragon.x, y - m.dragon.y) < 7.5 || Math.hypot(x - m.baron.x, y - m.baron.y) < 7.5 ||
      JUNGLE.some(j => Math.hypot(x - j[0], y - j[1]) < 3.5) || [[50, 22], [22, 50], [78, 50], [50, 78]].some(j => Math.hypot(x - j[0], y - j[1]) < 3.2);
    const trees = [];
    for (let i = 0; i < 2400 && trees.length < 1100; i++) { const x = R() * 100, y = R() * 100; if (x < 2 || y < 2 || x > 98 || y > 98 || blocked(x, y)) continue; trees.push([x, y, 1.1 + R() * 1.6]); }
    trees.sort((a, b) => a[1] - b[1]);
    for (const [x, y, r] of trees) { g.fillStyle = 'rgba(3,12,6,.55)'; g.beginPath(); g.arc(P(x + .4), P(y + .5), P(r), 0, TAU); g.fill(); }
    for (const [x, y, r] of trees) {
      const gr = g.createRadialGradient(P(x - r * .3), P(y - r * .3), 0, P(x), P(y), P(r));
      gr.addColorStop(0, '#3f7a3a'); gr.addColorStop(.7, '#1f4a24'); gr.addColorStop(1, '#123018');
      g.fillStyle = gr; g.beginPath(); g.arc(P(x), P(y), P(r), 0, TAU); g.fill();
    }
    // Jungle camps (clearings).
    for (const [x, y] of [...JUNGLE, [50, 22], [22, 50], [78, 50], [50, 78]]) { g.fillStyle = 'rgba(110,100,70,.45)'; g.beginPath(); g.arc(P(x), P(y), P(2.4), 0, TAU); g.fill(); g.fillStyle = 'rgba(255,220,140,.35)'; g.beginPath(); g.arc(P(x), P(y), P(.6), 0, TAU); g.fill(); }
    // Lanes.
    for (const k of ['top', 'mid', 'bot']) { poly(L[k], N * .06, '#3a3424'); poly(L[k], N * .046, '#7b6a47'); poly(L[k], N * .03, '#8f7c55'); }
    for (let i = 0; i < 500; i++) { const k = ['top', 'mid', 'bot'][i % 3], p = lanePoint(L[k], R()); g.fillStyle = R() < .5 ? 'rgba(60,50,32,.5)' : 'rgba(190,170,120,.35)'; g.fillRect(P(p[0] + (R() - .5) * 4), P(p[1] + (R() - .5) * 4), N * .003, N * .003); }
    // Pits.
    const pit = (p, col) => {
      g.fillStyle = '#2c2a26'; g.beginPath(); g.arc(P(p.x), P(p.y), P(6.2), 0, TAU); g.fill();
      const gr = g.createRadialGradient(P(p.x), P(p.y), 0, P(p.x), P(p.y), P(5.6)); gr.addColorStop(0, rgba(col, .55)); gr.addColorStop(1, 'rgba(30,26,22,1)');
      g.fillStyle = gr; g.beginPath(); g.arc(P(p.x), P(p.y), P(5.4), 0, TAU); g.fill();
      g.strokeStyle = 'rgba(160,150,130,.5)'; g.lineWidth = N * .004; g.beginPath(); g.arc(P(p.x), P(p.y), P(5.8), 0, TAU); g.stroke();
    };
    pit(m.dragon, '#ff8a3d'); pit(m.baron, '#9b59ff');
    // Bases.
    for (let side = 0; side < 2; side++) {
      const b = bases[side], col = SIDE_COLORS[side][0];
      g.fillStyle = '#3a3d46'; g.beginPath(); g.arc(P(b[0]), P(b[1]), P(16), 0, TAU); g.fill();
      const gr = g.createRadialGradient(P(b[0]), P(b[1]), 0, P(b[0]), P(b[1]), P(16));
      gr.addColorStop(0, rgba(col, .5)); gr.addColorStop(.75, rgba(col, .2)); gr.addColorStop(1, rgba(col, .05));
      g.fillStyle = gr; g.fill();
      g.strokeStyle = rgba(col, .6); g.lineWidth = N * .005; g.beginPath(); g.arc(P(b[0]), P(b[1]), P(15.5), 0, TAU); g.stroke();
      for (let i = 0; i < 24; i++) { const a = i / 24 * TAU; g.strokeStyle = 'rgba(255,255,255,.06)'; g.lineWidth = N * .002; g.beginPath(); g.moveTo(P(b[0] + Math.cos(a) * 5), P(b[1] + Math.sin(a) * 5)); g.lineTo(P(b[0] + Math.cos(a) * 15), P(b[1] + Math.sin(a) * 15)); g.stroke(); }
      const f = FOUNTAIN[side];
      const fg = g.createRadialGradient(P(f[0]), P(f[1]), 0, P(f[0]), P(f[1]), P(4.5)); fg.addColorStop(0, rgba(SIDE_COLORS[side][1], .9)); fg.addColorStop(1, rgba(col, .1));
      g.fillStyle = fg; g.beginPath(); g.arc(P(f[0]), P(f[1]), P(4.5), 0, TAU); g.fill();
    }
    // Edge vignette.
    const vg = g.createRadialGradient(N / 2, N / 2, N * .45, N / 2, N / 2, N * .75); vg.addColorStop(0, 'rgba(0,0,0,0)'); vg.addColorStop(1, 'rgba(0,0,0,.45)');
    g.fillStyle = vg; g.fillRect(0, 0, N, N);
    return c;
  }

  drawMap() {
    const g = this.ctx, st = this.state, G = this.mapG, s = this.s, R = this.mapR;
    if (!this.mapCache) this.mapCache = this.buildMapCache();
    // The sharp texture for the action cam is built in the background.
    if (!this.hiCache && !this.hiTimer && typeof setTimeout !== 'undefined') this.hiTimer = setTimeout(() => { this.hiCache = this.buildMapCache(2.4); }, 600);
    if (this.zoomed) {
      g.save(); g.beginPath(); g.roundRect(R.x, R.y, R.w, R.h, 8 * s); g.clip();
      const bg = g.createLinearGradient(0, R.y, 0, R.y + R.h); bg.addColorStop(0, '#060a10'); bg.addColorStop(1, '#0b1612');
      g.fillStyle = bg; g.fillRect(R.x, R.y, R.w, R.h);
      this.drawMapLayers(this.hiCache || this.mapCache);
      g.restore();
    } else this.drawMapLayers(this.mapCache);
  }

  // The map texture through the tilted camera: horizontal strips, each with
  // the affine transform that matches the perspective at its depth.
  drawMapImageCam(img) {
    const g = this.ctx, G = this.mapG, R = this.mapR, V = this.camView, c = this.camC;
    const N = img.width, m = 100 * G.k * Math.SQRT1_2 / N, n = m * SQ, e = G.cx, f = G.cy - 100 * G.k * SQ * Math.SQRT1_2;
    const t = camTilt(c.z), sq = 1 - .2 * t, ax = lerp(V.cx, V.ax, t);
    const y0 = this.base(0, 0)[1], y1 = this.base(100, 100)[1];
    const strips = Math.round(clamp(30 * c.z, 30, 110)), hs = (y1 - y0) / strips;
    const projY = Y => camProject(c, V, c.fX, Y);
    let prev = projY(y0);
    for (let i = 0; i < strips; i++) {
      const ya = y0 + i * hs, yb = ya + hs, B = projY(yb), A = prev; prev = B;
      if (B[1] < R.y || A[1] > R.y + R.h) continue;
      const ym = (ya + yb) / 2, M = projY(ym), km = M[2];
      g.save(); g.beginPath(); g.rect(R.x, A[1] - .6, R.w, B[1] - A[1] + 1.2); g.clip();
      g.setTransform(km * m, km * sq * n, -km * m, km * sq * n, ax + km * (e - c.fX), M[1] + km * sq * (f - ym));
      g.drawImage(img, 0, 0);
      g.restore();
    }
  }

  drawMapLayers(img) {
    const g = this.ctx, G = this.mapG, s = this.s;
    const L = this.P(0, 100), Tp = this.P(0, 0), Rt = this.P(100, 0), B = this.P(100, 100), dep = G.depth;
    // Table slab under the map (depth).
    g.save(); g.shadowColor = 'rgba(0,0,0,.6)'; g.shadowBlur = 30 * s; g.shadowOffsetY = 12 * s;
    g.fillStyle = '#0d1a14'; g.beginPath(); g.moveTo(L[0], L[1]); g.lineTo(B[0], B[1]); g.lineTo(Rt[0], Rt[1]); g.lineTo(Rt[0], Rt[1] + dep); g.lineTo(B[0], B[1] + dep); g.lineTo(L[0], L[1] + dep); g.closePath(); g.fill(); g.restore();
    g.fillStyle = '#0a1510'; g.beginPath(); g.moveTo(L[0], L[1]); g.lineTo(B[0], B[1]); g.lineTo(B[0], B[1] + dep); g.lineTo(L[0], L[1] + dep); g.closePath(); g.fill();
    g.fillStyle = '#142a1f'; g.beginPath(); g.moveTo(B[0], B[1]); g.lineTo(Rt[0], Rt[1]); g.lineTo(Rt[0], Rt[1] + dep); g.lineTo(B[0], B[1] + dep); g.closePath(); g.fill();
    if (img && this.zoomed) this.drawMapImageCam(img);
    else if (img) {
      const N = img.width, m = 100 * G.k * Math.SQRT1_2 / N, n = m * SQ;
      g.save(); g.setTransform(m, n, -m, n, G.cx, G.cy - 100 * G.k * SQ * Math.SQRT1_2); g.drawImage(img, 0, 0); g.restore();
    }
    g.strokeStyle = 'rgba(242,217,141,.35)'; g.lineWidth = 1.5 * s; g.beginPath(); g.moveTo(L[0], L[1]); g.lineTo(Tp[0], Tp[1]); g.lineTo(Rt[0], Rt[1]); g.lineTo(B[0], B[1]); g.closePath(); g.stroke();
    this.drawWaves();
    this.drawPits();
    this.drawStructures();
    this.drawPlans();
    this.drawEffectsBelow();
    this.drawActs(false);
    this.drawChampions();
    this.drawActs(true);
    this.drawEffectsAbove();
  }

  // The replay: the recorded frames of the shot, slowed down.
  drawReplayMap() {
    const rp = this.replay, st = this.state, fr = rp.frames, n = fr.length;
    const pos = clamp(rp.t * rp.rate / .05, 0, n - 1), i = Math.floor(pos), f = pos - i;
    const A = fr[i], B = fr[Math.min(n - 1, i + 1)];
    const keep = { pos: st.pos, hp: this.hp, acts: this.acts, popups: this.popups, flashes: this.flashes, skulls: this.skulls, clashes: this.clashes, particles: this.particles, anim: this.anim, clock: this.clock, dash: this.dash, air: this.air, shot: this.shot };
    st.pos = A.pos.map((a, t) => a.map((p, r) => { const q = B.pos[t][r]; return p.alive === q.alive ? { x: lerp(p.x, q.x, f), y: lerp(p.y, q.y, f), alive: p.alive } : p; }));
    Object.assign(this, { hp: A.hp, acts: A.acts, popups: A.popups, flashes: A.flashes, skulls: A.skulls, clashes: A.clashes, particles: A.particles, anim: lerp(A.anim, B.anim, f), clock: A.clock, dash: A.dash, air: A.air, shot: rp.shot });
    try { this.drawMap(); } finally { st.pos = keep.pos; delete keep.pos; Object.assign(this, keep); }
  }

  // The objective monster of the current shot, big, with a draining HP bar.
  drawMonster() {
    const sh = this.shot, m = sh && sh.mon;
    if (!m || !Number.isFinite(m.t0) || !this.zoomed) return;
    const g = this.ctx, s = this.s, now = this.anim, dk = (now - m.dieAt) / .9;
    if (dk >= 1) return;
    const [X, Y, k] = this.PS(sh.x, sh.y), S0 = this.mapG.k * k * (MON_SCALE[m.kind] || 1), S = S0 * (dk > 0 ? 1 - .3 * dk : 1);
    g.save();
    g.globalAlpha = clamp(this.camT * 1.6, 0, 1) * (dk > 0 ? 1 - dk : 1);
    g.translate(X, Y + (dk > 0 ? dk * S0 * 1.2 : 0));
    const t = this.now;
    if (m.kind === 'baron') paintBaron(g, S, t);
    else if (m.kind === 'herald') paintHerald(g, S, t);
    else if (m.kind === 'grubs') paintGrubs(g, S, t, dk > 0 ? 0 : 3);
    else paintDrake(g, S, m.col, t, m.kind === 'elder');
    // Death: a white burst.
    if (dk > 0) { const gr = g.createRadialGradient(0, -S0 * 2, 0, 0, -S0 * 2, S0 * 6); gr.addColorStop(0, `rgba(255,255,255,${.8 * (1 - dk)})`); gr.addColorStop(1, 'rgba(255,255,255,0)'); g.fillStyle = gr; g.beginPath(); g.arc(0, -S0 * 2, S0 * 6, 0, TAU); g.fill(); }
    g.restore();
    if (dk > 0) return;
    // HP bar.
    const hp = drainHp(m, now), lag = drainHp(m, now - .3);
    // A boss bar at the top of the view, like in the game.
    const R = this.mapR, bw = Math.min(R.w * .44, 380 * s), bh = 11 * s, bx = R.x + (R.w - bw) / 2, by = R.y + 72 * s, BX = bx + bw / 2;
    g.globalAlpha = clamp(this.camT * 1.6, 0, 1);
    this.otxt(m.name, bx, by - 9 * s, 12 * s, shade(m.col, .4), 'left', 900);
    if (sh.label) this.otxt(sh.team != null ? `${this.names()[sh.team].toUpperCase()} ON IT` : '', bx + bw, by - 9 * s, 9 * s, teamColor(this.state.meta, sh.team), 'right', 800);
    this.panel(bx - 2 * s, by - 2 * s, bw + 4 * s, bh + 4 * s, 3 * s, 'rgba(5,5,10,.85)', rgba(m.col, .8));
    g.fillStyle = 'rgba(255,255,255,.75)'; g.fillRect(bx, by, bw * lag, bh);
    const hg = g.createLinearGradient(0, by, 0, by + bh); hg.addColorStop(0, shade(m.col, .25)); hg.addColorStop(1, shade(m.col, -.35));
    g.fillStyle = hg; g.fillRect(bx, by, bw * hp, bh);
    g.fillStyle = 'rgba(0,0,0,.45)'; for (let i = 1; i < 10; i++) g.fillRect(bx + bw * i / 10, by, Math.max(1, s), bh * (i % 5 ? .5 : 1));
    this.otxt(`${Math.ceil(hp * (m.kind === 'baron' ? 12000 : m.kind === 'elder' ? 9000 : 5500))}`, BX, by + bh / 2, 9 * s, '#fff', 'center', 800);
    g.globalAlpha = 1;
  }

  // Ability effects of the action cam (ground layer first, then the air layer).
  drawActs(above) {
    if (!this.acts.length) return;
    const g = this.ctx, s = this.s, sq = 1 - .2 * this.camT;
    const ground = (x, y, r) => { const [X, Y, k] = this.PS(x, y), uu = this.mapG.k * k; return [X, Y, r * uu, r * uu * SQ * sq, uu]; };
    for (const a of this.acts) {
      if (a.age < 0) continue;
      const k = clamp(a.age / a.life, 0, 1), fade = 1 - k;
      if (!above) {
        if (a.k === 'aoe') {
          const [X, Y, rx, ry] = ground(a.x, a.y, a.r);
          if (a.age < a.hitAt) {
            const w = a.age / a.hitAt;
            g.fillStyle = rgba(a.team, .16); g.beginPath(); g.ellipse(X, Y, rx, ry, 0, 0, TAU); g.fill();
            g.strokeStyle = rgba(a.team, .85); g.lineWidth = 2 * s; g.stroke();
            g.fillStyle = rgba(a.team, .3); g.beginPath(); g.ellipse(X, Y, rx * w, ry * w, 0, 0, TAU); g.fill();
          } else {
            const w = (a.age - a.hitAt) / (a.life - a.hitAt);
            const gr = g.createRadialGradient(X, Y, 0, X, Y, rx * (1 + .3 * w));
            gr.addColorStop(0, `rgba(255,255,255,${.85 * (1 - w)})`); gr.addColorStop(.5, rgba(a.team, .7 * (1 - w))); gr.addColorStop(1, rgba(a.team, 0));
            g.save(); g.translate(X, Y); g.scale(1, ry / rx); g.fillStyle = gr; g.beginPath(); g.arc(0, 0, rx * (1 + .3 * w), 0, TAU); g.fill(); g.restore();
          }
        } else if (a.k === 'slam') {
          const [X, Y, rx, ry, uu] = ground(a.x, a.y, a.r * (.3 + 1.1 * easeOut(k)));
          g.strokeStyle = rgba(a.team, fade); g.lineWidth = (5 * fade + 1) * s; g.beginPath(); g.ellipse(X, Y, rx, ry, 0, 0, TAU); g.stroke();
          g.strokeStyle = `rgba(255,240,200,${fade * .8})`; g.lineWidth = 2 * s;
          for (let i = 0; i < 8; i++) { const an = i / 8 * TAU + a.x; g.beginPath(); g.moveTo(X + Math.cos(an) * rx * .3, Y + Math.sin(an) * ry * .3); g.lineTo(X + Math.cos(an) * rx * .8, Y + Math.sin(an) * ry * .8); g.stroke(); }
          if (k < .3) { g.fillStyle = `rgba(255,255,255,${(.3 - k) * 2})`; g.beginPath(); g.ellipse(X, Y, uu * 1.2, uu * .5, 0, 0, TAU); g.fill(); }
        } else if (a.k === 'spin') {
          const [X, Y, rx, ry] = ground(a.x, a.y, a.r);
          g.lineCap = 'round';
          for (let i = 0; i < 3; i++) {
            const st0 = k * TAU * 1.5 + i * TAU / 3;
            g.strokeStyle = i ? rgba(a.team, fade * .8) : `rgba(255,255,255,${fade})`; g.lineWidth = (6 - i * 1.5) * s;
            g.beginPath(); g.ellipse(X, Y - ry * .4, rx, ry, 0, st0, st0 + 1.6); g.stroke();
          }
        }
        continue;
      }
      switch (a.k) {
        case 'proj': {
          const px = lerp(a.x0, a.x1, k), py = lerp(a.y0, a.y1, k), [X, Y, sc] = this.PS(px, py), uu = this.mapG.k * sc, lift = uu * 1.6;
          if (a.style === 'arrow') {
            const [X0, Y0] = this.P(lerp(a.x0, a.x1, Math.max(0, k - .25)), lerp(a.y0, a.y1, Math.max(0, k - .25)));
            const gr = g.createLinearGradient(X0, Y0 - lift, X, Y - lift); gr.addColorStop(0, 'rgba(255,240,180,0)'); gr.addColorStop(1, '#fff6c8');
            g.strokeStyle = gr; g.lineWidth = 3 * s; g.lineCap = 'round'; g.beginPath(); g.moveTo(X0, Y0 - lift); g.lineTo(X, Y - lift); g.stroke();
            g.fillStyle = '#ffffff'; g.beginPath(); g.arc(X, Y - lift, 2.2 * s, 0, TAU); g.fill();
          } else if (a.style === 'hook') {
            const [X0, Y0] = this.P(a.x0, a.y0);
            g.strokeStyle = '#c9ccd6'; g.lineWidth = 2.5 * s; g.setLineDash([5 * s, 3 * s]); g.beginPath(); g.moveTo(X0, Y0 - lift); g.lineTo(X, Y - lift); g.stroke(); g.setLineDash([]);
            g.strokeStyle = '#eef0f6'; g.lineWidth = 3 * s; g.beginPath(); g.arc(X, Y - lift, uu * .55, -.5, Math.PI + .5); g.stroke();
          } else {
            const r = uu * (a.big ? .9 : .65);
            for (let i = 4; i >= 1; i--) {
              const kk = Math.max(0, k - i * .06), [TX, TY] = this.P(lerp(a.x0, a.x1, kk), lerp(a.y0, a.y1, kk));
              g.fillStyle = rgba(a.team, .18 * (5 - i) / 4); g.beginPath(); g.arc(TX, TY - lift, r * (1 - i * .12), 0, TAU); g.fill();
            }
            const gr = g.createRadialGradient(X, Y - lift, 0, X, Y - lift, r * 1.6);
            gr.addColorStop(0, '#ffffff'); gr.addColorStop(.35, a.col.startsWith('#') ? a.col : '#fff2c0'); gr.addColorStop(1, 'rgba(255,255,255,0)');
            g.fillStyle = gr; g.beginPath(); g.arc(X, Y - lift, r * 1.6, 0, TAU); g.fill();
          }
          break;
        }
        case 'slash': {
          const [X, Y, sc] = this.PS(a.x, a.y), uu = this.mapG.k * sc, r = uu * (a.big ? 2.6 : 1.8) * (.7 + .5 * easeOut(k));
          g.save(); g.translate(X, Y - uu * 1.6); g.rotate(a.ang + k * 1.2); g.lineCap = 'round';
          g.shadowColor = a.team; g.shadowBlur = 10 * s;
          g.strokeStyle = `rgba(255,255,255,${fade})`; g.lineWidth = (a.big ? 5 : 3.5) * s * fade + s; g.beginPath(); g.arc(0, 0, r, -1.1, 1.1); g.stroke();
          g.strokeStyle = rgba(a.team, fade); g.lineWidth = 2 * s; g.beginPath(); g.arc(0, 0, r * .8, -.9, .9); g.stroke();
          g.restore();
          break;
        }
        case 'dash': {
          const cr = this.mapG.k * this.PS(a.x1, a.y1)[2] * 1.5;
          for (let i = 1; i <= 3; i++) {
            const f = i / 4, [X, Y] = this.P(lerp(a.x0, a.x1, f), lerp(a.y0, a.y1, f));
            g.fillStyle = rgba(a.col, .3 * fade * f); g.beginPath(); g.arc(X, Y - cr * .6, cr * .9, 0, TAU); g.fill();
          }
          const [X0, Y0] = this.P(a.x0, a.y0), [X1, Y1] = this.P(a.x1, a.y1);
          const gr = g.createLinearGradient(X0, Y0, X1, Y1); gr.addColorStop(0, rgba(a.col, 0)); gr.addColorStop(1, `rgba(255,255,255,${.8 * fade})`);
          g.strokeStyle = gr; g.lineWidth = cr * .5; g.lineCap = 'round'; g.beginPath(); g.moveTo(X0, Y0 - cr * .6); g.lineTo(X1, Y1 - cr * .6); g.stroke();
          break;
        }
        case 'shield': case 'heal': {
          const p = this.state.pos[a.t][a.r];
          if (!p || p.alive === false) break;
          const [mx, my] = this.champXY(a.t, a.r), [X, Y, sc] = this.PS(mx, my), cr = this.mapG.k * sc * 1.55, cy = Y - cr * .6;
          const al = Math.min(1, a.age * 6) * Math.min(1, (a.life - a.age) * 3);
          if (a.k === 'shield') {
            g.fillStyle = `rgba(255,224,138,${.16 * al})`; g.beginPath(); g.arc(X, cy, cr * 1.45, 0, TAU); g.fill();
            g.strokeStyle = `rgba(255,236,170,${.9 * al})`; g.lineWidth = 2.5 * s; g.beginPath(); g.arc(X, cy, cr * 1.45, 0, TAU); g.stroke();
            for (let i = 0; i < 6; i++) { const an = i / 6 * TAU + this.now * 2; drawDiamond(g, X + Math.cos(an) * cr * 1.45, cy + Math.sin(an) * cr * 1.45, 3.5 * s, `rgba(255,246,210,${al})`); }
          } else {
            g.fillStyle = `rgba(98,255,150,${.8 * al})`;
            for (let i = 0; i < 5; i++) { const ph = (k + i / 5) % 1, x = X + Math.sin(i * 2.3) * cr, y = cy + cr - ph * cr * 2.4; g.fillRect(x - 1.5 * s, y - 5 * s, 3 * s, 10 * s); g.fillRect(x - 5 * s, y - 1.5 * s, 10 * s, 3 * s); }
          }
          break;
        }
        case 'beam': {
          const p = this.state.pos[a.t][a.r];
          if (!p || p.alive === false) break;
          const [X0, Y0, s0] = this.PS(a.x0, a.y0), [mx, my] = this.champXY(a.t, a.r), [X1, Y1, s1] = this.PS(mx, my);
          const top = Y0 - this.mapG.k * s0 * 1.05 * 3.5, cy = Y1 - this.mapG.k * s1 * 1.55 * .6;
          g.save(); g.lineCap = 'round'; g.shadowColor = a.col; g.shadowBlur = 14 * s;
          g.strokeStyle = rgba(a.col, fade); g.lineWidth = 7 * s * fade + 2 * s; g.beginPath(); g.moveTo(X0, top); g.lineTo(X1, cy); g.stroke();
          g.strokeStyle = `rgba(255,255,255,${fade})`; g.lineWidth = 2 * s; g.stroke(); g.restore();
          break;
        }
        case 'spark': {
          const [X, Y, sc] = this.PS(a.x, a.y), uu = this.mapG.k * sc, r = uu * (.6 + 1.4 * k);
          g.strokeStyle = `rgba(255,250,220,${fade})`; g.lineWidth = 2 * s;
          for (let i = 0; i < 6; i++) { const an = i / 6 * TAU + a.x; g.beginPath(); g.moveTo(X + Math.cos(an) * r * .4, Y - uu * 1.6 + Math.sin(an) * r * .4); g.lineTo(X + Math.cos(an) * r, Y - uu * 1.6 + Math.sin(an) * r); g.stroke(); }
          break;
        }
        case 'smite': {
          const [X, Y, sc] = this.PS(a.x, a.y), uu = this.mapG.k * sc, R0 = Math.max(this.mapR.y, Y - uu * 14);
          g.save(); g.shadowColor = '#ffd23f'; g.shadowBlur = 20 * s; g.strokeStyle = `rgba(255,236,150,${fade})`; g.lineWidth = 5 * s * fade + s; g.lineJoin = 'round';
          g.beginPath(); g.moveTo(X + uu * .5, R0);
          for (let i = 1; i <= 6; i++) g.lineTo(X + (i % 2 ? -1 : 1) * uu * .8 * (1 - i / 6), lerp(R0, Y - uu * 2, i / 6));
          g.stroke(); g.restore();
          break;
        }
        case 'death': {
          const [X, Y, sc] = this.PS(a.x, a.y), cr = this.mapG.k * sc * 1.55, line = this.state.line[a.t][a.r] || {};
          g.globalAlpha = .85 * fade; drawPortrait(g, X, Y - cr * .6 - k * 30 * s, cr * (1 - .2 * k), champInfo(this.state.meta, line.champ), '#888', { dead: true }); g.globalAlpha = 1;
          break;
        }
        case 'num': {
          const [X, Y, sc] = this.PS(a.x, a.y), uu = this.mapG.k * sc;
          const pop = a.age < .12 ? 1 + (.12 - a.age) * 5 : 1, size = (a.big ? 19 : 13) * s * pop;
          g.globalAlpha = clamp((1 - k) * 3, 0, 1);
          this.otxt(a.text, X + (a.dx || 0) * uu * .6, Y - uu * 3.6 - k * 34 * s, size, a.col, 'center', 900);
          g.globalAlpha = 1;
          break;
        }
      }
    }
  }

  // Screen-space dressing of the action cam: vignette, letterbox, tags, flashes.
  drawCamOverlay() {
    const g = this.ctx, R = this.mapR, s = this.s, t = this.camT, rp = this.replay;
    if (this.state.mode !== 'map') return;
    if (t > .01) {
      g.save(); g.beginPath(); g.roundRect(R.x, R.y, R.w, R.h, 8 * s); g.clip();
      const vg = g.createRadialGradient(R.x + R.w / 2, R.y + R.h * .55, R.h * .35, R.x + R.w / 2, R.y + R.h * .55, R.w * .72);
      vg.addColorStop(0, 'rgba(0,0,0,0)'); vg.addColorStop(1, `rgba(0,0,0,${.6 * t})`);
      g.fillStyle = vg; g.fillRect(R.x, R.y, R.w, R.h);
      const lb = R.h * .06 * easeInOut(t);
      g.fillStyle = 'rgba(0,0,0,.88)'; g.fillRect(R.x, R.y, R.w, lb); g.fillRect(R.x, R.y + R.h - lb, R.w, lb);
      if (rp) {
        g.fillStyle = 'rgba(40,60,110,.12)'; g.fillRect(R.x, R.y, R.w, R.h);
        g.fillStyle = 'rgba(0,0,0,.08)'; g.beginPath();
        for (let y = R.y + ((this.now * 40 * s) % (4 * s)); y < R.y + R.h; y += 4 * s) g.rect(R.x, y, R.w, 1.2 * s);
        g.fill();
      }
      g.restore();
      if (rp) { g.strokeStyle = 'rgba(242,197,92,.9)'; g.lineWidth = 3 * s; g.beginPath(); g.roundRect(R.x + 1.5 * s, R.y + 1.5 * s, R.w - 3 * s, R.h - 3 * s, 8 * s); g.stroke(); }
    }
    // Tag: ACTION CAM (live) or REPLAY, top centre between the plan cards.
    const sh = rp ? rp.shot : this.shot;
    const a = rp ? 1 : clamp(t * 2, 0, 1);
    if (a > .02 && sh) {
      g.save(); g.globalAlpha = a;
      const fs = 11 * s, cx = R.x + R.w / 2, y = R.y + 8 * s, h = 24 * s;
      const head = rp ? `↺ ${T.replay}` : T.actionCam, sub = [sh.label, sh.where].filter(Boolean).join(' · ').toUpperCase();
      g.font = this.font(fs * (rp ? 1.25 : 1), 900); const hw = g.measureText(head).width + 30 * s;
      g.font = this.font(fs, 800); const maxW = R.w - 2 * Math.min(285 * s, R.w * .34) - 24 * s;
      const sw = Math.min(g.measureText(sub).width + 20 * s, Math.max(0, maxW - hw));
      const x0 = cx - (hw + sw) / 2;
      this.panel(x0, y, hw, h, 4 * s, rp ? '#f2c55c' : 'rgba(200,30,40,.95)', null);
      if (!rp) { g.fillStyle = `rgba(255,255,255,${.6 + .4 * Math.sin(this.now * 7)})`; g.beginPath(); g.arc(x0 + 11 * s, y + h / 2, 3.5 * s, 0, TAU); g.fill(); }
      this.txt(head, x0 + (rp ? hw / 2 : 18 * s), y + h / 2, fs * (rp ? 1.25 : 1), rp ? '#140d02' : '#fff', rp ? 'center' : 'left', 900);
      if (sw > 30 * s) { this.panel(x0 + hw, y, sw, h, 4 * s, 'rgba(8,11,20,.9)', null); this.txt(this.fit(sub, sw - 16 * s, fs, 800), x0 + hw + 10 * s, y + h / 2, fs, '#fff', 'left', 800); }
      if (rp) {
        const bw = hw + sw, k = rp.len > 0 ? rp.t / rp.len : 1;
        g.fillStyle = 'rgba(0,0,0,.6)'; g.fillRect(x0, y + h + 3 * s, bw, 3 * s);
        g.fillStyle = '#f2c55c'; g.fillRect(x0, y + h + 3 * s, bw * k, 3 * s);
        this.otxt(T.slowMo, cx, y + h + 16 * s, 9 * s, 'rgba(255,255,255,.75)', 'center', 800);
      }
      g.restore();
    } else if (!rp && t < .01) {
      // On the full map: a small MAP tag.
      const fs = 9.5 * s, h = 18 * s, y = R.y + 8 * s;
      g.font = this.font(fs, 900); const w = g.measureText(T.mapTag).width + 26 * s, x0 = R.x + R.w / 2 - w / 2;
      this.panel(x0, y, w, h, 4 * s, 'rgba(8,11,20,.72)', 'rgba(255,255,255,.12)');
      g.fillStyle = '#ff4d4d'; g.beginPath(); g.arc(x0 + 9 * s, y + h / 2, 3 * s, 0, TAU); g.fill();
      this.txt(T.mapTag, x0 + 16 * s, y + h / 2, fs, 'rgba(255,255,255,.85)', 'left', 900);
    }
    // Cut flash.
    const fa = 1 - (this.now - this.cutFlash) / .3;
    if (fa > 0) { g.fillStyle = `rgba(255,255,255,${.75 * fa})`; g.fillRect(R.x, R.y, R.w, R.h); }
  }

  // Text with a dark outline (readable over the busy zoomed map).
  otxt(str, x, y, size, color, align = 'center', weight = 800) {
    const g = this.ctx; g.font = this.font(size, weight); g.textAlign = align; g.textBaseline = 'middle';
    g.lineJoin = 'round'; g.lineWidth = Math.max(2, size * .22); g.strokeStyle = 'rgba(0,0,0,.85)'; g.strokeText(str, x, y);
    g.fillStyle = color; g.fillText(str, x, y);
  }

  drawWaves() {
    const g = this.ctx, st = this.state, u = this.u, L = lanes(st);
    if (st.mode === 'victory') return;
    for (const lane of ['top', 'mid', 'bot']) {
      const f = waveFront(st, lane), path = L[lane];
      for (let side = 0; side < 2; side++) {
        const col = SIDE_COLORS[side][0], sup = st.inhibs[`${1 - side}-${lane}`] === false;
        for (let w = 0; w < 3; w++) {
          const ph = ((this.anim * .09 + w / 3 + (lane === 'mid' ? .1 : lane === 'top' ? .2 : 0)) % 1);
          const sPos = side === 0 ? ph * f : 1 - ph * (1 - f);
          const fade = ph > .85 ? (1 - ph) / .15 : ph < .08 ? ph / .08 : 1;
          for (let i = 0; i < 4; i++) {
            const off = (side === 0 ? -1 : 1) * i * .012;
            const p = lanePoint(path, sPos + off);
            const jx = ph > .85 ? Math.sin(this.anim * 20 + i) * .5 : 0;
            const [X, Y] = this.P(p[0] + (i % 2 ? .8 : -.8) + jx, p[1] + (i % 2 ? -.8 : .8));
            g.globalAlpha = fade * .95; g.fillStyle = col;
            g.beginPath(); g.arc(X, Y, u * (sup && i === 0 ? .9 : .5), 0, TAU); g.fill();
            g.fillStyle = 'rgba(255,255,255,.5)'; g.beginPath(); g.arc(X - u * .12, Y - u * .15, u * .18, 0, TAU); g.fill();
          }
        }
      }
    }
    g.globalAlpha = 1;
  }

  // Tower health: structures the enemy is pushing look damaged.
  towerHp(tw) {
    const st = this.state;
    const front = waveFront(st, tw.lane === 'nexus' ? 'mid' : tw.lane);
    const next = nextStructure(st, tw.side, tw.lane === 'nexus' ? 'mid' : tw.lane);
    let hp = 1;
    if (next && Math.abs(next.x - tw.x) < .01 && Math.abs(next.y - tw.y) < .01) {
      const press = tw.side === 0 ? 1 - front : front;
      hp = clamp(1.25 - press * .9, .35, 1);
      for (let t = 0; t < 2; t++) { const tg = planTarget(st, t); if (tg && mapSide(st.meta, t) !== tw.side && Math.abs(tg.x - tw.x) < .01 && Math.abs(tg.y - tw.y) < .01) hp = Math.min(hp, .45); }
    }
    return hp;
  }

  drawStructures() {
    const g = this.ctx, st = this.state, s = this.s;
    const items = [];
    for (const tw of st.map.towers || []) items.push({ kind: 'tower', o: tw });
    for (const ih of st.map.inhibitors || []) items.push({ kind: 'inhib', o: ih });
    for (const nx of st.map.nexus || []) items.push({ kind: 'nexus', o: nx });
    for (const it of items) it.p = this.PS(it.o.x, it.o.y);
    items.sort((a, b) => a.p[1] - b.p[1]);
    // A structure the action cam is watching fall stays up until its HP runs out.
    const so = this.shot && this.shot.struct, held = so && Number.isFinite(so.dieAt) && this.anim < so.dieAt;
    const isHeld = (kind, o) => !!held && so.kind === kind && so.side === o.side && (kind === 'nexus' || so.lane === o.lane) && (kind !== 'tower' || so.tier === o.tier);
    const bar = (x, by, w, hp, col) => {
      const bh = Math.max(3 * s, w * .1);
      g.fillStyle = 'rgba(0,0,0,.8)'; g.fillRect(x - w / 2 - 1, by - 1, w + 2, bh + 2);
      g.fillStyle = hp > .6 ? col : hp > .3 ? '#f0b13a' : '#ef5350'; g.fillRect(x - w / 2, by, w * hp, bh);
    };
    for (const it of items) {
      const o = it.o, [x, y, sc] = it.p, u = this.mapG.k * sc, col = SIDE_COLORS[o.side][0], light = SIDE_COLORS[o.side][1];
      const hold = isHeld(it.kind, o), dhp = hold ? drainHp(so, this.anim) : 1;
      if (it.kind === 'tower') {
        const alive = hold || st.towers[`${o.side}-${o.lane}-${o.tier}`] !== false, a = u * 1.05;
        if (alive) {
          g.fillStyle = 'rgba(0,0,0,.45)'; g.beginPath(); g.ellipse(x, y, a * 1.3, a * .6, 0, 0, TAU); g.fill();
          const body = g.createLinearGradient(x - a, 0, x + a, 0); body.addColorStop(0, '#7b8190'); body.addColorStop(.5, '#a9afbd'); body.addColorStop(1, '#4f5462');
          g.fillStyle = body; g.beginPath(); g.moveTo(x - a * .75, y); g.lineTo(x - a * .5, y - a * 2.6); g.lineTo(x + a * .5, y - a * 2.6); g.lineTo(x + a * .75, y); g.closePath(); g.fill();
          g.fillStyle = col; g.fillRect(x - a * .75, y - a * 3.0, a * 1.5, a * .5);
          const pulse = .7 + .3 * Math.sin(this.now * 3 + o.x);
          g.shadowColor = light; g.shadowBlur = a * 2 * pulse; g.fillStyle = light; g.beginPath(); g.arc(x, y - a * 3.5, a * .45, 0, TAU); g.fill(); g.shadowBlur = 0;
          const hp = hold ? dhp : this.towerHp(o);
          const bw = a * 2.4, bh = Math.max(2 * s, a * .32), by = y - a * 4.5;
          g.fillStyle = 'rgba(0,0,0,.75)'; g.fillRect(x - bw / 2 - 1, by - 1, bw + 2, bh + 2);
          g.fillStyle = hp > .6 ? col : hp > .4 ? '#f0b13a' : '#ef5350'; g.fillRect(x - bw / 2, by, bw * hp, bh);
        } else {
          g.fillStyle = 'rgba(0,0,0,.4)'; g.beginPath(); g.ellipse(x, y, a * 1.6, a * .7, 0, 0, TAU); g.fill();
          const R = rng(hash(`${o.side}${o.lane}${o.tier}`));
          for (let i = 0; i < 6; i++) { g.fillStyle = i % 2 ? '#6d6a64' : '#4b4945'; g.beginPath(); const rx = x + (R() - .5) * a * 2, ry = y + (R() - .5) * a * .8 - a * .2; g.moveTo(rx, ry - a * .4); g.lineTo(rx + a * .4, ry); g.lineTo(rx, ry + a * .25); g.lineTo(rx - a * .35, ry); g.closePath(); g.fill(); }
          g.fillStyle = rgba(col, .6); g.fillRect(x - a * .4, y - a * .3, a * .8, a * .2);
        }
      } else if (it.kind === 'inhib') {
        const key = `${o.side}-${o.lane}`, up = hold || st.inhibs[key] !== false, a = u * 1.1;
        if (hold) bar(x, y - a * 3.3, a * 2.6, dhp, col);
        g.fillStyle = 'rgba(0,0,0,.45)'; g.beginPath(); g.ellipse(x, y, a * 1.3, a * .55, 0, 0, TAU); g.fill();
        g.fillStyle = '#5d6170'; g.beginPath(); g.ellipse(x, y - a * .2, a * 1.1, a * .45, 0, 0, TAU); g.fill();
        if (up) {
          const hov = Math.sin(this.now * 2 + o.x) * a * .2;
          g.shadowColor = light; g.shadowBlur = a * 2.5; g.fillStyle = col;
          g.beginPath(); g.moveTo(x, y - a * 2.6 + hov); g.lineTo(x + a * .7, y - a * 1.5 + hov); g.lineTo(x, y - a * .5 + hov); g.lineTo(x - a * .7, y - a * 1.5 + hov); g.closePath(); g.fill(); g.shadowBlur = 0;
          g.fillStyle = 'rgba(255,255,255,.6)'; g.beginPath(); g.moveTo(x, y - a * 2.4 + hov); g.lineTo(x + a * .25, y - a * 1.5 + hov); g.lineTo(x, y - a * 1.1 + hov); g.closePath(); g.fill();
        } else {
          g.fillStyle = '#444852'; g.beginPath(); g.moveTo(x - a * .6, y - a * .3); g.lineTo(x - a * .2, y - a * 1.0); g.lineTo(x + a * .1, y - a * .4); g.lineTo(x + a * .5, y - a * .9); g.lineTo(x + a * .6, y - a * .2); g.closePath(); g.fill();
          const t0 = st.inhibDown[key];
          if (Number.isFinite(t0)) {
            const left = clamp(1 - (this.clock - t0) / 300, 0, 1);
            g.strokeStyle = 'rgba(255,255,255,.2)'; g.lineWidth = 2 * s; g.beginPath(); g.arc(x, y - a * 1.2, a * 1.1, 0, TAU); g.stroke();
            g.strokeStyle = col; g.beginPath(); g.arc(x, y - a * 1.2, a * 1.1, -Math.PI / 2, -Math.PI / 2 + TAU * (1 - left)); g.stroke();
          }
        }
      } else {
        const alive = hold || st.nexus[o.side], a = u * 1.6;
        if (hold) bar(x, y - a * 3.8, a * 2.6, dhp, col);
        g.fillStyle = 'rgba(0,0,0,.5)'; g.beginPath(); g.ellipse(x, y, a * 1.6, a * .7, 0, 0, TAU); g.fill();
        g.fillStyle = '#4d5260'; g.beginPath(); g.ellipse(x, y - a * .2, a * 1.4, a * .6, 0, 0, TAU); g.fill();
        if (alive) {
          const pulse = .6 + .4 * Math.sin(this.now * 2 + o.side);
          g.shadowColor = light; g.shadowBlur = a * 3 * pulse;
          const gr = g.createLinearGradient(x, y - a * 3, x, y); gr.addColorStop(0, light); gr.addColorStop(1, col);
          g.fillStyle = gr; g.beginPath(); g.moveTo(x, y - a * 3.2); g.lineTo(x + a * .9, y - a * 1.6); g.lineTo(x + a * .5, y - a * .4); g.lineTo(x - a * .5, y - a * .4); g.lineTo(x - a * .9, y - a * 1.6); g.closePath(); g.fill(); g.shadowBlur = 0;
          g.strokeStyle = rgba(light, .6); g.lineWidth = 1.5 * s; g.beginPath(); g.ellipse(x, y - a * 1.6, a * 1.5, a * .5, 0, this.now % TAU, this.now % TAU + Math.PI * 1.3); g.stroke();
        } else {
          g.fillStyle = '#2a2b30'; g.beginPath(); g.ellipse(x, y - a * .3, a * 1.1, a * .45, 0, 0, TAU); g.fill();
          for (let i = 0; i < 3; i++) { const k = ((this.now * .4 + i / 3) % 1); g.fillStyle = `rgba(120,120,130,${.35 * (1 - k)})`; g.beginPath(); g.arc(x + Math.sin(i * 2 + this.now) * a * .3, y - a * (.6 + k * 3), a * (.4 + k * .8), 0, TAU); g.fill(); }
        }
      }
    }
  }

  drawBadge(mx, my, glyph, col, label, sub, subCol, up) {
    const g = this.ctx, u = this.u, s = this.s, [x, y0] = this.P(mx, my), y = y0 - u * 1.6, r = u * 2.3;
    if (up) { const p = .5 + .5 * Math.sin(this.now * 2.5); g.shadowColor = col; g.shadowBlur = r * (.6 + p); }
    g.fillStyle = up ? 'rgba(14,10,20,.92)' : 'rgba(14,14,18,.75)'; g.beginPath(); g.arc(x, y, r, 0, TAU); g.fill(); g.shadowBlur = 0;
    g.strokeStyle = up ? col : 'rgba(255,255,255,.25)'; g.lineWidth = Math.max(1.5 * s, r * .12); g.stroke();
    g.save(); g.translate(x, y); g.globalAlpha *= up ? 1 : .45; glyph(g, r * .62, up ? col : '#aaa'); g.restore();
    const fs = Math.max(9 * s, u * 1.25);
    g.font = this.font(fs, 800); const w = Math.max(g.measureText(label).width, sub ? (g.font = this.font(fs * .9, 700), g.measureText(sub).width) : 0) + fs;
    this.panel(x - w / 2, y + r + 2 * s, w, sub ? fs * 2.5 : fs * 1.45, fs * .4, 'rgba(6,8,14,.8)', null);
    this.txt(label, x, y + r + 2 * s + fs * .72, fs, up ? col : 'rgba(255,255,255,.75)', 'center', 800);
    if (sub) this.txt(sub, x, y + r + 2 * s + fs * 1.8, fs * .9, subCol || 'rgba(255,255,255,.7)', 'center', 700);
  }

  drawPits() {
    // The badges make way for the action cam.
    const fa = 1 - clamp(this.camT * 2.5, 0, 1);
    if (fa <= 0) return;
    this.ctx.globalAlpha = fa;
    try { this.drawPitBadges(); } finally { this.ctx.globalAlpha = 1; }
  }

  drawPitBadges() {
    const st = this.state, m = st.map, o = st.obj, c = this.clock, names = this.names();
    const taken = (rec, kind) => rec && c - rec.t < 70 ? { sub: `TAKEN BY ${names[rec.team].toUpperCase()}`, col: teamColor(st.meta, rec.team) } : null;
    // Dragon pit.
    {
      const soul = st.soul[0] || st.soul[1];
      const nTaken = st.dragons[0].length + st.dragons[1].length;
      const type = nTaken >= 3 ? (st.dragons[0].concat(st.dragons[1]).slice(-1)[0]) : null;
      let label, col, next, glyph = drawDrake;
      if (soul) { label = 'ELDER DRAGON'; col = DRAGON_COLORS.elder; next = o.elderNext; }
      else { label = type ? `${type.toUpperCase()} DRAKE` : 'DRAKE'; col = DRAGON_COLORS[type] || '#ff8a3d'; next = o.dragonNext; }
      const up = Number.isFinite(next) && c >= next;
      const tk = taken(o.last.dragon);
      const sub = tk ? tk.sub : up ? 'ALIVE' : Number.isFinite(next) ? `SPAWNS IN ${fmtClock(next - c)}` : '';
      this.drawBadge(m.dragon.x, m.dragon.y, glyph, col, label, sub, tk ? tk.col : up ? col : null, up);
    }
    // Baron pit: grubs → herald → baron.
    {
      let label, col = '#a35bff', glyph = drawBaronGlyph, up, sub, tk;
      if (taken(o.last.grubs) && c < 840 + 70) { label = 'VOID GRUBS'; col = '#a78bfa'; glyph = drawGrubsGlyph; up = false; tk = taken(o.last.grubs); }
      else if (c < 840 && !o.grubsTaken) { label = 'VOID GRUBS'; col = '#a78bfa'; glyph = drawGrubsGlyph; up = c >= 360; sub = up ? 'ALIVE' : `SPAWN IN ${fmtClock(360 - c)}`; tk = taken(o.last.grubs); }
      else if (c < 1185 && !o.heraldTaken && c < o.baronNext) { label = 'RIFT HERALD'; col = '#c48bff'; glyph = drawEyeGlyph; up = c >= 840; sub = up ? 'ALIVE' : `SPAWNS IN ${fmtClock(840 - c)}`; tk = taken(o.last.herald); }
      else if (taken(o.last.herald) && c < o.baronNext) { label = 'RIFT HERALD'; col = '#c48bff'; glyph = drawEyeGlyph; up = false; tk = taken(o.last.herald); }
      else { label = 'BARON NASHOR'; up = c >= o.baronNext; sub = up ? 'ALIVE' : `SPAWNS IN ${fmtClock(o.baronNext - c)}`; tk = taken(o.last.baron); }
      this.drawBadge(m.baron.x, m.baron.y, glyph, col, label, tk ? tk.sub : sub, tk ? tk.col : up ? col : null, up && !tk);
    }
  }

  // Each team's plan as an arrow + a label on the map.
  drawPlans() {
    const g = this.ctx, st = this.state, s = this.s, u = this.u, names = this.names();
    if (st.mode === 'victory') return;
    const pa = 1 - clamp(this.camT * 2.5, 0, 1);
    if (pa <= 0) return;
    g.globalAlpha = pa;
    const flash = clamp(1 - (this.anim - this.planFlash) / 1.5, 0, 1);
    for (let team = 0; team < 2; team++) {
      const plan = st.plan[team], tg = planTarget(st, team);
      if (!plan || !tg) continue;
      const col = teamColor(st.meta, team), side = mapSide(st.meta, team);
      let fx, fy;
      if (Number.isInteger(tg.slot)) { const p = st.pos[team][tg.slot]; if (!p || p.alive === false) continue; fx = p.x; fy = p.y; }
      else {
        const al = st.pos[team].filter(p => p && p.alive !== false);
        if (!al.length) continue;
        fx = al.reduce((a, p) => a + p.x, 0) / al.length; fy = al.reduce((a, p) => a + p.y, 0) / al.length;
      }
      const [x1, y1] = this.P(tg.x, tg.y);
      const dist = Math.hypot(tg.x - fx, tg.y - fy);
      if (dist > 5) {
        const [x0, y0] = this.P(fx, fy);
        const mx = (x0 + x1) / 2, my = (y0 + y1) / 2 - Math.hypot(x1 - x0, y1 - y0) * .18;
        g.strokeStyle = rgba(col, .85); g.lineWidth = (3 + 2 * flash) * s; g.setLineDash([9 * s, 6 * s]); g.lineDashOffset = -this.now * 28 * s;
        g.beginPath(); g.moveTo(x0, y0); g.quadraticCurveTo(mx, my, x1, y1); g.stroke(); g.setLineDash([]);
        const ang = Math.atan2(y1 - my, x1 - mx), ah = 11 * s;
        g.fillStyle = col; g.beginPath(); g.moveTo(x1, y1); g.lineTo(x1 - Math.cos(ang - .45) * ah, y1 - Math.sin(ang - .45) * ah); g.lineTo(x1 - Math.cos(ang + .45) * ah, y1 - Math.sin(ang + .45) * ah); g.closePath(); g.fill();
      }
      const pr = (1 + .25 * Math.sin(this.now * 4)) * u * 3;
      g.strokeStyle = rgba(col, .7); g.lineWidth = 2 * s; g.beginPath(); g.ellipse(x1, y1, pr, pr * SQ, 0, 0, TAU); g.stroke();
      // Label chip: blue above the target, red below.
      const text = `${names[team]}: ${planText(plan)}`;
      const fs = 12 * s * (1 + .15 * flash);
      g.font = this.font(fs, 800); const w = Math.min(this.mapR.w * .5, g.measureText(text).width + fs * 1.4);
      const lx = clamp(x1 - w / 2, this.mapR.x + 4 * s, this.mapR.x + this.mapR.w - w - 4 * s);
      const ly = side === 0 ? y1 - u * 7.5 - fs * 1.7 : y1 + u * 5.5;
      this.panel(lx, ly, w, fs * 1.7, fs * .5, rgba(col, .88), flash > 0 ? `rgba(255,255,255,${flash})` : null);
      this.txt(this.fit(text, w - fs, fs, 800), lx + w / 2, ly + fs * .87, fs, '#fff', 'center', 800);
    }
    g.globalAlpha = 1;
  }

  drawEffectsBelow() {
    const g = this.ctx, u = this.u, s = this.s;
    for (const f of this.flashes) {
      const k = f.age / f.life, [x, y] = this.P(f.x, f.y), r = u * (f.big ? 11 : 6) * easeOut(k);
      g.strokeStyle = rgba(f.color.startsWith('#') ? f.color : '#ffffff', 1 - k); g.lineWidth = (f.big ? 5 : 3) * s;
      g.beginPath(); g.ellipse(x, y, r, r * SQ, 0, 0, TAU); g.stroke();
      if (k < .3) { const gr = g.createRadialGradient(x, y, 0, x, y, u * (f.big ? 7 : 4)); gr.addColorStop(0, `rgba(255,255,240,${(.3 - k) * 3})`); gr.addColorStop(1, 'rgba(255,255,240,0)'); g.fillStyle = gr; g.beginPath(); g.arc(x, y, u * (f.big ? 7 : 4), 0, TAU); g.fill(); }
    }
    for (const q of this.particles) { const [x, y] = this.P(q.x, q.y); g.globalAlpha = clamp(q.life * 1.5, 0, 1); g.fillStyle = q.c; g.fillRect(x - u * .3, y - u * .3, u * .6, u * .6); }
    g.globalAlpha = 1;
  }

  drawChampions() {
    const g = this.ctx, st = this.state, s = this.s, ct = this.camT;
    const items = [];
    for (let t = 0; t < 2; t++) for (let r = 0; r < 5; r++) {
      const p = st.pos[t][r]; if (!p) continue;
      const dead = p.alive === false, [mx, my] = this.champXY(t, r), [x, y, sc] = this.PS(mx, my);
      items.push({ t, r, x, y, sc, dead });
    }
    // The filmed monster is depth-sorted with the champions.
    const sh = this.shot;
    if (sh && sh.mon && this.zoomed) items.push({ mon: true, dead: false, y: this.PS(sh.x, sh.y)[1] });
    items.sort((a, b) => (b.dead - a.dead) || a.y - b.y);
    for (const it of items) {
      if (it.mon) { this.drawMonster(); continue; }
      // Bigger tokens up close, but not as big as the zoom (room for the fight).
      const cr = Math.max(9 * s, this.mapG.k * it.sc * lerp(2.1, 1.4, ct));
      const line = st.line[it.t][it.r] || {}, info = champInfo(st.meta, line.champ);
      const airT = this.anim - (this.air[it.t][it.r] || -9), lift = airT >= 0 && airT < .6 ? Math.sin(airT / .6 * Math.PI) * cr * 1.1 : 0;
      const col = teamColor(st.meta, it.t), x = it.x, y = it.y - cr * .6 - lift;
      g.fillStyle = 'rgba(0,0,0,.45)'; g.beginPath(); g.ellipse(it.x, it.y + cr * .35, cr * .9, cr * .35, 0, 0, TAU); g.fill();
      if (ct > .05 && !it.dead) {
        g.strokeStyle = rgba(col, .85 * ct); g.lineWidth = 2.5 * s; g.beginPath(); g.ellipse(it.x, it.y + cr * .35, cr * 1.15, cr * .45, 0, 0, TAU); g.stroke();
      }
      if (it.dead) {
        g.globalAlpha = .55; drawPortrait(g, x, y, cr * .85, info, col, { dead: true }); g.globalAlpha = 1;
        const left = Math.ceil(Math.max(0, st.dead[it.t][it.r] - this.clock));
        if (left > 0) { this.panel(x - cr * .7, y - cr * .5, cr * 1.4, cr, cr * .3, 'rgba(0,0,0,.7)', null); this.txt(String(left), x, y, cr * .8, '#ff8080', 'center', 900); }
        continue;
      }
      if (st.buff[it.t].baron > this.clock) { g.strokeStyle = `rgba(170,100,255,${.6 + .3 * Math.sin(this.now * 5)})`; g.lineWidth = 2.5 * s; g.beginPath(); g.arc(x, y, cr * 1.3, 0, TAU); g.stroke(); }
      if (st.buff[it.t].elder > this.clock) { g.strokeStyle = 'rgba(245,225,122,.9)'; g.lineWidth = 2 * s; g.beginPath(); g.arc(x, y, cr * 1.5, 0, TAU); g.stroke(); }
      drawPortrait(g, x, y, cr, info, col, {});
      const hp = this.hp[it.t][it.r], bw = cr * 2.2, bh = Math.max(2.5 * s, cr * .26), by = y - cr * 1.55;
      g.fillStyle = 'rgba(0,0,0,.8)'; g.fillRect(x - bw / 2 - 1, by - 1, bw + 2, bh + 2);
      g.fillStyle = hp > .5 ? (it.t === blueTeam(st.meta) ? '#4fa3ff' : '#ff5d6a') : hp > .25 ? '#f0b13a' : '#ef3b3b'; g.fillRect(x - bw / 2, by, bw * hp, bh);
      g.fillStyle = 'rgba(0,0,0,.5)'; for (let i = 1; i < 4; i++) g.fillRect(x - bw / 2 + bw * i / 4, by, Math.max(1, s * .8), bh);
      // Up close, a name plate like in the game.
      if (ct > .35) {
        g.globalAlpha = clamp((ct - .35) * 3, 0, 1);
        this.otxt(line.champ || '', x, by - Math.min(12 * s, cr * .45), clamp(cr * .38, 8.5 * s, 12 * s), '#fff', 'center', 800);
        g.globalAlpha = 1;
      }
    }
  }

  drawEffectsAbove() {
    const g = this.ctx, u = Math.min(this.u, this.mapG.k * 1.6), s = this.s;
    for (const k of this.skulls) {
      const a = clamp(Math.min(k.age * 4, (k.life - k.age) / .6), 0, 1), [x, y] = this.P(k.x, k.y);
      g.globalAlpha = a; this.panel(x - u * 1.5, y - u * 4.6, u * 3, u * 3, u * 1.5, 'rgba(0,0,0,.65)', k.color); drawSkull(g, x, y - u * 3.05, u * 1.15, '#f2f2f2'); g.globalAlpha = 1;
    }
    for (const c of this.clashes) {
      const k = c.age / c.life, [x, y] = this.P(c.x, c.y), r = u * (c.big ? 3.2 : 2.2) * (1 + .4 * easeOut(Math.min(1, k * 3)));
      g.globalAlpha = 1 - k; g.shadowColor = '#fff2c0'; g.shadowBlur = 14 * s; drawSwords(g, x, y - u * 2, r, '#fff6d8'); g.shadowBlur = 0; g.globalAlpha = 1;
    }
    for (const p of this.popups) {
      const k = p.age / p.life, [x, y] = this.P(p.x, p.y);
      g.globalAlpha = clamp((1 - k) * 2, 0, 1);
      g.font = this.font(15 * s, 900); g.textAlign = 'center'; g.textBaseline = 'middle';
      g.lineWidth = 4 * s; g.strokeStyle = 'rgba(0,0,0,.8)'; g.strokeText(p.text, x + u * 2, y - u * 5 - k * 30 * s);
      g.fillStyle = p.color; g.fillText(p.text, x + u * 2, y - u * 5 - k * 30 * s);
      g.globalAlpha = 1;
    }
  }

  // Corners of the tilted map: plan cards (top), objective timers and kill feed (bottom).
  drawMapOverlays() {
    const g = this.ctx, st = this.state, s = this.s, R = this.mapR, names = this.names();
    if (st.mode === 'victory') return;
    const order = this.order(), flash = clamp(1 - (this.anim - this.planFlash) / 1.5, 0, 1);
    const cw = Math.min(285 * s, R.w * .34), ch = 50 * s;
    for (let k = 0; k < 2; k++) {
      const team = order[k], plan = st.plan[team], col = teamColor(st.meta, team);
      const x = k === 0 ? R.x + 4 * s : R.x + R.w - cw - 4 * s, y = R.y + 2 * s;
      this.panel(x, y, cw, ch, 8 * s, 'rgba(8,11,20,.88)', flash > 0 ? rgba(col, .4 + .6 * flash) : rgba(col, .4));
      g.fillStyle = col; g.fillRect(k === 0 ? x : x + cw - 4 * s, y, 4 * s, ch);
      const al = k === 0 ? 'left' : 'right', tx = k === 0 ? x + 12 * s : x + cw - 12 * s;
      this.txt(this.fit(`${names[team].toUpperCase()} · ${T.plan}`, cw - 24 * s, 10 * s, 800), tx, y + 14 * s, 10 * s, col, al, 800);
      this.txt(this.fit(plan ? planText(plan) : 'Loading into the Rift…', cw - 24 * s, 15 * s, 800), tx, y + 34 * s, 15 * s, '#fff', al, 800);
    }
    // Kill feed (bottom right).
    const fw = Math.min(215 * s, R.w * .28), rh = 24 * s;
    const feed = this.killFeed.filter(f => this.anim - f.born < 14);
    feed.slice(0, 3).forEach((f, i) => {
      const a = clamp((14 - (this.anim - f.born)) / 1.5, 0, 1) * clamp((this.anim - f.born) * 5, 0, 1);
      const x = R.x + R.w - fw - 4 * s, y = R.y + R.h - (i + 1) * (rh + 4 * s);
      g.globalAlpha = a;
      const kt = f.killer.side === 1 ? 1 : 0, vt = f.victim.side === 1 ? 1 : 0, kc = teamColor(st.meta, kt), vc = teamColor(st.meta, vt);
      this.panel(x, y, fw, rh, 5 * s, 'rgba(8,11,20,.88)', null);
      g.fillStyle = kc; g.fillRect(x, y, 3 * s, rh);
      const r = rh * .38;
      drawPortrait(g, x + 8 * s + r, y + rh / 2, r, champInfo(st.meta, f.killer.champ), kc);
      const nw = (fw - 4 * r - 60 * s) / 2;
      this.txt(this.fit(f.killer.champ || '', nw, 10.5 * s, 800), x + 12 * s + r * 2, y + rh / 2, 11 * s, '#fff', 'left', 800);
      drawSwords(g, x + fw / 2 + 6 * s, y + rh / 2, rh * .25, f.ace || f.multi >= 3 ? GOLDC : '#ddd');
      this.txt(this.fit(f.victim.champ || '', nw, 10.5 * s, 700), x + fw - 12 * s - r * 2, y + rh / 2, 10.5 * s, 'rgba(255,255,255,.75)', 'right', 700);
      drawPortrait(g, x + fw - 8 * s - r, y + rh / 2, r, champInfo(st.meta, f.victim.champ), vc, { dead: true });
      g.globalAlpha = 1;
    });
    // Upcoming objectives (bottom left).
    const o = st.obj, c = this.clock, rows = [];
    if (st.soul[0] || st.soul[1]) rows.push(['ELDER', o.elderNext, DRAGON_COLORS.elder]); else rows.push(['DRAGON', o.dragonNext, '#ff8a3d']);
    if (!o.heraldTaken && c < 1185 && c < o.baronNext) rows.push(['HERALD', 840, '#c48bff']);
    rows.push(['BARON', o.baronNext, '#a35bff']);
    const ow = Math.min(150 * s, R.w * .2), oh = 20 * s;
    rows.forEach(([lab, t, col], i) => {
      const x = R.x + 4 * s, y = R.y + R.h - (rows.length - i) * (oh + 3 * s);
      this.panel(x, y, ow, oh, 4 * s, 'rgba(8,11,20,.85)', null);
      drawDiamond(g, x + 10 * s, y + oh / 2, 5 * s, col);
      this.txt(lab, x + 20 * s, y + oh / 2, 10 * s, 'rgba(255,255,255,.85)', 'left', 800);
      const up = c >= t;
      this.txt(up ? 'UP' : Number.isFinite(t) ? fmtClock(t - c) : '—', x + ow - 8 * s, y + oh / 2, 11 * s, up ? col : '#fff', 'right', 800);
    });
  }

  drawTicker() {
    const st = this.state, s = this.s, K = this.tick;
    if (st.mode === 'victory' || !st.ticker) return;
    const ev = this.cur ? this.cur.ev : this.queue[this.idx - 1];
    const d = (ev && ev.data) || {};
    const col = ev && ev.kind === 'match_end' ? teamColor(st.meta, d.winner === 1 ? 1 : 0) : ev && (d.side === 0 || d.side === 1) ? teamColor(st.meta, d.side) : '#d8b25a';
    this.panel(K.x, K.y, K.w, K.h, 6 * s, 'rgba(8,11,20,.9)', null);
    this.ctx.fillStyle = col; this.ctx.fillRect(K.x, K.y, 5 * s, K.h);
    const fs = Math.min(15 * s, K.h * .52);
    this.txt(fmtClock(this.clock), K.x + 14 * s, K.y + K.h / 2, fs * .85, 'rgba(255,255,255,.5)', 'left', 800);
    this.txt(this.fit(st.ticker, K.w - 80 * s, fs, 700), K.x + 64 * s, K.y + K.h / 2, fs, ev && ev.hl ? '#ffd76a' : '#fff', 'left', 700);
  }

  // ---- top bar ----

  drawTopBar() {
    const g = this.ctx, W = this.W, s = this.s, st = this.state;
    const h = this.topH - 6 * s, y = 4 * s, names = this.names(), order = this.order();
    const grd = g.createLinearGradient(0, y, 0, y + h); grd.addColorStop(0, '#141b2e'); grd.addColorStop(1, '#090d18');
    g.fillStyle = grd; g.beginPath(); g.roundRect(6 * s, y, W - 12 * s, h, 10 * s); g.fill();
    const mid = W / 2, clockW = 130 * s;
    // Clock.
    this.panel(mid - clockW / 2, y, clockW, h, 10 * s, '#05070d', 'rgba(242,217,141,.4)');
    this.txt(fmtClock(this.clock), mid, y + h * .42, 24 * s, '#fff', 'center', 900);
    const status = this.match && this.match.status;
    const ph = this.clock < 840 ? 'LANING' : this.clock < 1500 ? 'MID GAME' : 'LATE GAME';
    if (status === 'live') { g.fillStyle = '#ff4d4d'; g.beginPath(); g.arc(mid - 30 * s, y + h * .8, 3.5 * s * (.7 + .3 * Math.sin(this.now * 5)), 0, TAU); g.fill(); }
    this.txt(status === 'final' ? `${T.final} · ${ph}` : status === 'live' ? `${T.live} · ${ph}` : ph, mid, y + h * .8, 9 * s, status === 'live' ? '#ff8080' : 'rgba(255,255,255,.55)', 'center', 800);
    const half = (W - 12 * s - clockW) / 2;
    for (let k = 0; k < 2; k++) {
      const team = order[k], col = teamColor(st.meta, team), left = k === 0;
      const x0 = left ? 6 * s : mid + clockW / 2, x1 = x0 + half;
      const cg = g.createLinearGradient(left ? x0 : x1, 0, left ? x1 : x0, 0); cg.addColorStop(0, rgba(col, .45)); cg.addColorStop(.6, rgba(col, .08)); cg.addColorStop(1, rgba(col, 0));
      g.fillStyle = cg; g.fillRect(x0, y, half, h);
      g.fillStyle = col; g.fillRect(left ? x0 : x1 - 6 * s, y, 6 * s, h);
      // Kills (big, next to the clock).
      const inner = left ? x1 - 14 * s : x0 + 14 * s, dir = left ? -1 : 1, al = left ? 'right' : 'left', oal = left ? 'left' : 'right';
      this.txt(String(st.kills[team] || 0), inner, y + h / 2 + 1 * s, 34 * s, '#fff', al, 900);
      // Name.
      const outer = left ? x0 + 16 * s : x1 - 16 * s;
      const statsW = Math.min(250 * s, half * .55);
      this.txt(this.fit(names[team].toUpperCase(), half < 420 * s ? half - 150 * s : half - statsW - 70 * s, 20 * s, 900), outer, y + h * .4, 20 * s, '#fff', oal, 900);
      this.txt(T.side[k], outer, y + h * .76, 9 * s, rgba(SIDE_COLORS[k][1], .9), oal, 800);
      // Stats: gold, towers, dragons.
      let sx = inner + dir * 52 * s;
      const cy = y + h / 2;
      const stat = (icon, text, w) => {
        const ix = left ? sx - w : sx;
        icon(ix + 8 * s, cy);
        this.txt(text, ix + 18 * s, cy, 14 * s, '#fff', 'left', 800);
        sx += dir * (w + 10 * s);
      };
      stat((x, yy) => drawCoin(g, x, yy, 6.5 * s), fmtGold(st.gold[team]), 62 * s);
      if (half < 420 * s) continue;
      stat((x, yy) => drawTowerIcon(g, x, yy, 7 * s, '#d9dde6'), String(st.towersK[team] || 0), 36 * s);
      const dr = st.dragons[team];
      const dw = 14 * s * Math.max(4, dr.length) + 6 * s, dx0 = left ? sx - dw : sx;
      for (let i = 0; i < Math.max(4, dr.length); i++) drawDiamond(g, dx0 + 8 * s + i * 14 * s, cy, 6 * s, dr[i] ? (DRAGON_COLORS[dr[i]] || '#fff') : 'rgba(255,255,255,.25)', !!dr[i]);
      if (st.soul[team]) this.txt('SOUL', dx0 + dw / 2, cy + 13 * s, 8 * s, DRAGON_COLORS[st.soul[team]] || '#fff', 'center', 900);
    }
  }

  // ---- side panel ----

  drawSidePanel() {
    const S = this.side, s = this.s;
    if (S.w < 60 * s || S.h < 60 * s) return;
    const order = this.order();
    if (this.wide) {
      const rosterH = Math.min(172 * s, S.h * .27);
      this.drawRoster(order[0], S.x, S.y, S.w, rosterH);
      this.drawRoster(order[1], S.x, S.y + rosterH + 8 * s, S.w, rosterH);
      const oy = S.y + 2 * rosterH + 16 * s, oh = 78 * s;
      this.drawObjectives(S.x, oy, S.w, oh);
      const gy = oy + oh + 8 * s;
      if (S.y + S.h - gy > 50 * s) this.drawGoldGraph(S.x, gy, S.w, S.y + S.h - gy);
    } else {
      const w2 = (S.w - 6 * s) / 2, rh = Math.min(140 * s, S.h * .5);
      this.drawRoster(order[0], S.x, S.y, w2, rh);
      this.drawRoster(order[1], S.x + w2 + 6 * s, S.y, w2, rh);
      const gy = S.y + rh + 6 * s;
      if (S.y + S.h - gy > 50 * s) this.drawGoldGraph(S.x, gy, S.w, S.y + S.h - gy);
    }
  }

  drawRoster(team, x, y, w, h) {
    const g = this.ctx, s = this.s, st = this.state, col = teamColor(st.meta, team), names = this.names();
    this.panel(x, y, w, h, 8 * s);
    const hh = Math.min(22 * s, h * .16);
    const hg = g.createLinearGradient(x, 0, x + w, 0); hg.addColorStop(0, rgba(col, .6)); hg.addColorStop(1, rgba(col, .05));
    g.fillStyle = hg; g.beginPath(); g.roundRect(x, y, w, hh, [8 * s, 8 * s, 0, 0]); g.fill();
    this.txt(this.fit(names[team].toUpperCase(), w * .55, hh * .55, 900), x + 8 * s, y + hh / 2, hh * .55, '#fff', 'left', 900);
    const narrowCols = w < 260 * s;
    this.txt(narrowCols ? 'K/D/A' : 'K / D / A      GOLD', x + w - 8 * s, y + hh / 2, hh * .42, 'rgba(255,255,255,.7)', 'right', 800);
    const rh = (h - hh - 4 * s) / 5;
    const cur = this.cur && this.cur.ev.kind === 'kill' ? this.cur.ev.data || {} : null;
    for (let r = 0; r < 5; r++) {
      const ry = y + hh + 2 * s + r * rh, line = st.line[team][r] || {}, alive = isAlive(st, team, r) || st.mode === 'victory';
      const hot = cur && ((cur.killer && cur.killer.side === team && cur.killer.slot === r) || (cur.victim && cur.victim.side === team && cur.victim.slot === r));
      if (hot) { g.fillStyle = rgba(cur.killer.side === team && cur.killer.slot === r ? GOLDC : '#ff4040', .18); g.fillRect(x + 2 * s, ry, w - 4 * s, rh - 1 * s); }
      else if (r % 2) { g.fillStyle = 'rgba(255,255,255,.03)'; g.fillRect(x + 2 * s, ry, w - 4 * s, rh - 1 * s); }
      const pr = rh * .38, px = x + 8 * s + pr, py = ry + rh / 2;
      drawPortrait(g, px, py, pr, champInfo(st.meta, line.champ), col, { dead: !alive });
      if (!alive) { const left = Math.ceil(Math.max(0, st.dead[team][r] - this.clock)); this.txt(String(left), px, py, pr * 1.05, '#ff8080', 'center', 900); }
      const nx = px + pr + 6 * s, fs = Math.min(13 * s, rh * .42);
      this.txt(T.roleShort[ROLES[r]], nx, py, fs * .7, rgba(col, 1), 'left', 900);
      this.txt(this.fit(line.champ || '—', w * (narrowCols ? .3 : .34), fs, 800), nx + 30 * s, py - fs * .3, fs, alive ? '#fff' : 'rgba(255,255,255,.45)', 'left', 800);
      this.txt(this.fit(line.handle || '', w * .3, fs * .72, 600), nx + 30 * s, py + fs * .62, fs * .72, 'rgba(255,255,255,.5)', 'left', 600);
      const kda = st.kda[team][r];
      if (narrowCols) this.txt(`${kda.k}/${kda.d}/${kda.a}`, x + w - 8 * s, py, fs, '#fff', 'right', 800);
      else {
        this.txt(`${kda.k} / ${kda.d} / ${kda.a}`, x + w - 66 * s, py, fs, '#fff', 'right', 800);
        this.txt(fmtGold(st.pgold[team][r]), x + w - 8 * s, py, fs, GOLDC, 'right', 800);
      }
    }
  }

  drawObjectives(x, y, w, h) {
    const g = this.ctx, s = this.s, st = this.state, order = this.order(), names = this.names();
    this.panel(x, y, w, h, 8 * s);
    this.txt(T.objectives, x + 8 * s, y + 11 * s, 10 * s, 'rgba(242,217,141,.9)', 'left', 900);
    const cols = [['TOWERS', w * .30], ['DRAGONS', w * .55], ['HERALD', w * .74], ['BARON', w * .9]];
    cols.forEach(([l, cx]) => this.txt(l, x + cx, y + 11 * s, 8 * s, 'rgba(255,255,255,.45)', 'center', 800));
    const rh = (h - 22 * s) / 2;
    for (let k = 0; k < 2; k++) {
      const team = order[k], col = teamColor(st.meta, team), cy = y + 22 * s + rh * k + rh / 2;
      g.fillStyle = col; g.fillRect(x + 8 * s, cy - rh * .3, 3 * s, rh * .6);
      this.txt(this.fit(names[team], w * .18, 11 * s, 800), x + 16 * s, cy, 11 * s, '#fff', 'left', 800);
      drawTowerIcon(g, x + w * .30 - 10 * s, cy, 7 * s, '#d9dde6'); this.txt(`${st.towersK[team] || 0}`, x + w * .30 + 2 * s, cy, 14 * s, '#fff', 'left', 900);
      const dr = st.dragons[team];
      for (let i = 0; i < 4; i++) drawDiamond(g, x + w * .55 + (i - 1.5) * 15 * s, cy, 6.5 * s, dr[i] ? DRAGON_COLORS[dr[i]] || '#fff' : 'rgba(255,255,255,.3)', !!dr[i]);
      if (st.soul[team]) this.txt('SOUL', x + w * .55, cy + 11 * s, 8 * s, DRAGON_COLORS[st.soul[team]] || '#fff', 'center', 900);
      if (st.elders[team]) this.txt(`+${st.elders[team]} ELDER`, x + w * .55 + 34 * s, cy - 10 * s, 7 * s, DRAGON_COLORS.elder, 'left', 900);
      g.save(); g.translate(x + w * .74 - 8 * s, cy); drawEyeGlyph(g, 6 * s, st.heralds[team] ? '#c48bff' : 'rgba(255,255,255,.25)'); g.restore();
      this.txt(`${st.heralds[team]}${st.grubs[team] ? ` +${st.grubs[team]}g` : ''}`, x + w * .74 + 2 * s, cy, 12 * s, '#fff', 'left', 800);
      g.save(); g.translate(x + w * .9 - 9 * s, cy); drawBaronGlyph(g, 6 * s, st.barons[team] ? '#b77bff' : 'rgba(255,255,255,.25)'); g.restore();
      this.txt(`${st.barons[team] || 0}`, x + w * .9 + 2 * s, cy, 12 * s, '#fff', 'left', 800);
    }
  }

  drawGoldGraph(x, y, w, h, title = true) {
    const g = this.ctx, s = this.s, st = this.state;
    this.panel(x, y, w, h, 8 * s);
    const sign = mapSide(st.meta, 0) === 0 ? 1 : -1; // blue team on top
    const pts = (st.goldPts.length ? st.goldPts : [[0, 0]]).map(p => [p[0], p[1] * sign]);
    const tMax = Math.max(900, pts[pts.length - 1][0], this.clock);
    const vMax = Math.max(3000, ...pts.map(p => Math.abs(p[1]))) * 1.1;
    const top = y + (title ? 24 * s : 8 * s), bot = y + h - 16 * s, mid = (top + bot) / 2;
    const px = t => x + 12 * s + t / tMax * (w - 24 * s), py = v => mid - v / vMax * (bot - top) / 2;
    // Grid.
    g.strokeStyle = 'rgba(255,255,255,.06)'; g.lineWidth = 1;
    for (let m = 0; m <= tMax; m += 300) { g.beginPath(); g.moveTo(px(m), top); g.lineTo(px(m), bot); g.stroke(); }
    for (let m = 0; m <= tMax; m += 600) this.txt(`${m / 60}`, px(m), bot + 8 * s, 8 * s, 'rgba(255,255,255,.35)', 'center', 700);
    g.strokeStyle = 'rgba(255,255,255,.22)'; g.beginPath(); g.moveTo(x + 8 * s, py(0)); g.lineTo(x + w - 8 * s, py(0)); g.stroke();
    const order = this.order();
    for (let k = 0; k < 2; k++) {
      const col = teamColor(st.meta, order[k]);
      g.save(); g.beginPath();
      if (k === 0) g.rect(x, top - 4 * s, w, py(0) - top + 4 * s); else g.rect(x, py(0), w, bot - py(0) + 4 * s);
      g.clip();
      const ag = g.createLinearGradient(0, k === 0 ? top : bot, 0, py(0)); ag.addColorStop(0, rgba(col, .55)); ag.addColorStop(1, rgba(col, .08));
      g.beginPath(); g.moveTo(px(pts[0][0]), py(0));
      pts.forEach(p => g.lineTo(px(p[0]), py(p[1])));
      g.lineTo(px(pts[pts.length - 1][0]), py(0)); g.closePath();
      g.fillStyle = ag; g.fill();
      g.strokeStyle = col; g.lineWidth = 2.2 * s; g.lineJoin = 'round'; g.beginPath(); pts.forEach((p, i) => i ? g.lineTo(px(p[0]), py(p[1])) : g.moveTo(px(p[0]), py(p[1]))); g.stroke();
      g.restore();
    }
    // Markers for big events.
    const valAt = t => { for (let i = 1; i < pts.length; i++) if (pts[i][0] >= t) { const a = pts[i - 1], b = pts[i], k = b[0] === a[0] ? 1 : (t - a[0]) / (b[0] - a[0]); return lerp(a[1], b[1], k); } return pts[pts.length - 1][1]; };
    for (const m of st.marks) {
      const mx = px(m.t), my = py(valAt(m.t));
      g.fillStyle = '#0a0d16'; g.beginPath(); g.arc(mx, my, 6 * s, 0, TAU); g.fill();
      if (m.kind === 'dragon') drawDiamond(g, mx, my, 4.5 * s, m.color);
      else if (m.kind === 'baron' || m.kind === 'herald') { g.save(); g.translate(mx, my); (m.kind === 'baron' ? drawBaronGlyph : drawEyeGlyph)(g, 4.5 * s, m.color); g.restore(); }
      else if (m.kind === 'ace' || m.kind === 'kill') drawSkull(g, mx, my, 4.2 * s, m.color);
      else { g.fillStyle = m.color; g.beginPath(); g.arc(mx, my, 3.5 * s, 0, TAU); g.fill(); }
    }
    if (title) {
      const diff = pts[pts.length - 1][1];
      this.txt(T.golddiff, x + 10 * s, y + 12 * s, 10 * s, 'rgba(242,217,141,.9)', 'left', 900);
      const lead = diff >= 0 ? order[0] : order[1];
      this.txt(Math.abs(diff) < 50 ? 'EVEN' : `${this.names()[lead]} +${(Math.abs(diff) / 1000).toFixed(1)}k`, x + w - 10 * s, y + 12 * s, 11 * s, teamColor(st.meta, lead), 'right', 900);
    }
  }

  // ---- banners ----

  drawBanner() {
    if (!this.banner || this.state.mode === 'victory' || this.replay) return;
    const g = this.ctx, s = this.s, b = this.banner;
    const age = this.anim - b.t0, life = b.big ? 2.6 : 1.8;
    if (age > life) { this.banner = null; return; }
    if (age < 0) return;
    const k = easeOut(clamp(age * 4, 0, 1)), out = clamp((life - age) * 3, 0, 1);
    // With the action cam on, banners move to the lower third, off the fight.
    const R = this.mapR, cx = R.x + R.w / 2, cy = R.y + R.h * lerp(b.big ? .4 : .2, b.big ? .76 : .8, clamp(this.camT * 1.5, 0, 1));
    const fs = (b.big ? 56 : 32) * s;
    g.save(); g.globalAlpha = out;
    g.font = this.font(fs, 900, true);
    let tw = g.measureText(b.text).width + fs * 1.8;
    if (b.sub) { g.font = this.font(fs * .32, 800); tw = Math.max(tw, g.measureText(b.sub).width + fs * 1.4); g.font = this.font(fs, 900, true); }
    tw = Math.min(R.w * .96, tw);
    const w = tw * k, slant = fs * .35, hh = fs * (b.sub ? 1.0 : .8);
    // Streak.
    if (b.big) { const sg = g.createLinearGradient(R.x, 0, R.x + R.w, 0); sg.addColorStop(0, rgba(b.color, 0)); sg.addColorStop(.5, rgba(b.color, .35)); sg.addColorStop(1, rgba(b.color, 0)); g.fillStyle = sg; g.fillRect(R.x, cy - hh * 1.3, R.w, hh * 2.6); }
    g.fillStyle = 'rgba(6,8,14,.88)';
    g.beginPath(); g.moveTo(cx - w / 2 + slant, cy - hh); g.lineTo(cx + w / 2 + slant, cy - hh); g.lineTo(cx + w / 2 - slant, cy + hh); g.lineTo(cx - w / 2 - slant, cy + hh); g.closePath(); g.fill();
    g.strokeStyle = b.color; g.lineWidth = 3 * s; g.stroke();
    g.fillStyle = b.color; g.fillRect(cx - w / 2, cy + hh - 5 * s, w, 5 * s);
    if (k > .5) {
      g.globalAlpha = out * clamp((k - .5) * 2, 0, 1);
      g.shadowColor = b.color; g.shadowBlur = fs * .5; g.fillStyle = '#fff'; g.font = this.font(fs, 900, true); g.textAlign = 'center'; g.textBaseline = 'middle';
      g.fillText(b.text, cx, cy - (b.sub ? fs * .18 : 0), tw - fs);
      g.shadowBlur = 0;
      if (b.sub) this.txt(this.fit(b.sub, tw - fs, fs * .32, 800), cx, cy + fs * .55, fs * .32, shade(b.color, .55), 'center', 800);
    }
    g.restore();
  }

  // ---- victory ----

  drawVictory() {
    const g = this.ctx, W = this.W, H = this.H, st = this.state, e = st.end || {};
    const s = Math.min(W / 1280, H / 720);
    const w = e.winner === 1 ? 1 : 0, col = teamColor(st.meta, w), names = this.names(), order = this.order();
    g.fillStyle = 'rgba(4,6,12,.94)'; g.fillRect(0, 0, W, H);
    const rg = g.createRadialGradient(W / 2, H * .15, 0, W / 2, H * .15, W * .6);
    rg.addColorStop(0, rgba(col, .45)); rg.addColorStop(1, rgba(col, 0)); g.fillStyle = rg; g.fillRect(0, 0, W, H);
    const narrow = W / H < 1.2;
    // Header.
    const vg = g.createLinearGradient(0, 20 * s, 0, 90 * s); vg.addColorStop(0, '#fff3c4'); vg.addColorStop(1, '#d9a53c');
    g.font = this.font(64 * s, 900, true); g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillStyle = vg;
    g.shadowColor = rgba(col, .9); g.shadowBlur = 30 * s; g.fillText(T.victory, W / 2, 58 * s); g.shadowBlur = 0;
    this.txt(this.fit((e.winner_name || names[w]).toUpperCase(), W * .8, 34 * s, 900), W / 2, 112 * s, 34 * s, col, 'center', 900);
    const stats = e.stats || {}, fk = finalKills(e) || st.kills;
    const steals = Array.isArray(stats.steals) ? stats.steals : null;
    const sub = `${T.duration} ${fmtClock(e.duration || st.t)} · ${T.kills} ${fk[w] || 0}–${fk[1 - w] || 0}${e.comeback ? ' · ' + T.comeback : ''}`;
    this.txt(sub, W / 2, 142 * s, 14 * s, 'rgba(255,255,255,.7)', 'center', 700);
    // Players tables (left = blue, right = red) and centre comparison.
    const players = e.players || [[], []];
    const top = 168 * s, rowH = 30 * s;
    const tw = narrow ? W - 24 * s : W * .32, cw = narrow ? W - 24 * s : W * .28;
    for (let k = 0; k < 2; k++) {
      const team = order[k], c = teamColor(st.meta, team);
      const x0 = narrow ? 12 * s : k === 0 ? 20 * s : W - 20 * s - tw;
      const y0 = narrow ? top + 200 * s + k * rowH * 6.3 : top;
      this.panel(x0, y0, tw, rowH * 6.1, 10 * s);
      g.fillStyle = c; g.fillRect(x0, y0, tw, 3 * s);
      this.txt(this.fit(names[team].toUpperCase(), tw * .55, 14 * s, 900), x0 + 12 * s, y0 + rowH * .55, 14 * s, c, 'left', 900);
      this.txt('K / D / A     GOLD', x0 + tw - 12 * s, y0 + rowH * .55, 10 * s, 'rgba(255,255,255,.5)', 'right', 800);
      (players[team] || []).forEach((p, i) => {
        const y = y0 + (i + 1.55) * rowH;
        const mvp = e.mvp && e.mvp.side === team && e.mvp.slot === i;
        if (mvp) { g.fillStyle = 'rgba(242,197,92,.15)'; g.fillRect(x0 + 4 * s, y - rowH * .45, tw - 8 * s, rowH * .9); }
        drawPortrait(g, x0 + 24 * s, y, rowH * .36, champInfo(st.meta, p.champ), c);
        this.txt(T.roleShort[p.role] || '', x0 + 42 * s, y, 9 * s, c, 'left', 900);
        this.txt(this.fit(p.champ, tw * .3, 13 * s, 800), x0 + 72 * s, y - 5 * s, 13 * s, '#fff', 'left', 800);
        this.txt(this.fit(p.handle || '', tw * .3, 10 * s, 600), x0 + 72 * s, y + 8 * s, 10 * s, 'rgba(255,255,255,.5)', 'left', 600);
        if (mvp) this.txt('MVP', x0 + tw * .55, y, 10 * s, GOLDC, 'center', 900);
        this.txt(`${p.k} / ${p.d} / ${p.a}`, x0 + tw - 64 * s, y, 13 * s, '#fff', 'right', 800);
        this.txt(fmtGold(p.gold), x0 + tw - 12 * s, y, 13 * s, GOLDC, 'right', 800);
      });
    }
    // Comparison.
    const cx0 = narrow ? 12 * s : W / 2 - cw / 2, cy0 = top;
    const rows = [
      [T.kills, fk], [T.gold, (stats.gold || st.gold).map(fmtGold), (stats.gold || st.gold)], [T.towers, st.towersK],
      [T.dragons, (stats.dragons || [st.dragons[0].length, st.dragons[1].length])], [T.barons, stats.barons || st.barons], [T.heralds, stats.heralds || st.heralds],
    ];
    if (Array.isArray(stats.grubs) && stats.grubs[0] + stats.grubs[1] > 0) rows.push([T.grubsStat, stats.grubs]);
    if (Array.isArray(stats.elders) && stats.elders[0] + stats.elders[1] > 0) rows.push([T.elders, stats.elders]);
    if (steals && steals[0] + steals[1] > 0) rows.push([T.steals, steals]);
    const rr = rowH * 6 / rows.length;
    this.panel(cx0, cy0, cw, rowH * 6.1, 10 * s);
    rows.forEach(([lab, vals, raw], i) => {
      const y = cy0 + (i + .6) * rr;
      const a = vals[order[0]], b = vals[order[1]], ra = +(raw || vals)[order[0]] || 0, rb = +(raw || vals)[order[1]] || 0;
      this.txt(String(a), cx0 + 14 * s, y, 15 * s, ra >= rb ? teamColor(st.meta, order[0]) : '#fff', 'left', 900);
      this.txt(String(b), cx0 + cw - 14 * s, y, 15 * s, rb >= ra ? teamColor(st.meta, order[1]) : '#fff', 'right', 900);
      this.txt(lab.toUpperCase(), cx0 + cw / 2, y - 5 * s, 9 * s, 'rgba(255,255,255,.55)', 'center', 800);
      const bw = cw * .5, tot = ra + rb || 1, bx = cx0 + cw / 2 - bw / 2, by = y + 5 * s;
      g.fillStyle = teamColor(st.meta, order[0]); g.fillRect(bx, by, bw * ra / tot, 4 * s);
      g.fillStyle = teamColor(st.meta, order[1]); g.fillRect(bx + bw * ra / tot, by, bw * rb / tot, 4 * s);
    });
    // MVP line under the comparison.
    const mvp = e.mvp || {};
    if (mvp.champ && !narrow) {
      const my = cy0 + rowH * 6.1 + 34 * s;
      drawPortrait(g, W / 2 - 90 * s, my, 24 * s, champInfo(st.meta, mvp.champ), col, { glow: true });
      this.txt(T.mvp, W / 2 - 58 * s, my - 13 * s, 11 * s, GOLDC, 'left', 900);
      this.txt(this.fit(`${mvp.handle || ''} · ${mvp.champ}`, 190 * s, 16 * s, 900), W / 2 - 58 * s, my + 3 * s, 16 * s, '#fff', 'left', 900);
      this.txt(`${mvp.k}/${mvp.d}/${mvp.a}`, W / 2 - 58 * s, my + 19 * s, 11 * s, 'rgba(255,255,255,.7)', 'left', 700);
    }
    const gy = narrow ? top + 200 * s + rowH * 12.8 : top + rowH * 6.1 + 76 * s;
    const gh = H - gy - 14 * s;
    if (gh > 50 * s) {
      const gx = narrow ? 12 * s : 20 * s, gw = W - 2 * gx;
      const saved = this.s; this.s = s; this.drawGoldGraph(gx, gy, gw, gh); this.s = saved;
    }
  }
  // ---- arena scenes ----
  //
  // Everything is procedural and framed in a 1600×900 "world" (cover-fit to the
  // canvas). The static parts (roof, tiers, stage, frames) are cached in one
  // layer; screens, lights, booths and the crowd are drawn live.

  arenaTeams() {
    const order = this.order(), meta = this.state.meta, names = this.names();
    return [0, 1].map(p => ({ team: order[p], col: teamColor(meta, order[p]), name: names[order[p]] || '' }));
  }

  // The arena caches: fans, static layer, a blurred copy (caster desk backdrop), haze.
  arenaPrep() {
    const tm = this.arenaTeams(), key = `${this.W}x${this.H}|${tm.map(t => t.col + t.name).join('|')}`;
    if (this.arena && this.arena.key === key) return this.arena;
    const R = rng(7), fans = [], tw = [], rows = [], pal = [], palIx = new Map();
    // Fans are drawn row by row, one path per shirt colour (a few quantized tones).
    const ci = c => { if (!palIx.has(c)) { palIx.set(c, pal.length); pal.push(c); } return palIx.get(c); };
    const addRow = (y, list) => {
      const by = new Map();
      for (const f of list) { if (!by.has(f.ci)) by.set(f.ci, []); by.get(f.ci).push(f); fans.push(f); }
      rows.push({ y, list, groups: [...by.entries()] });
    };
    // Floor crowd, seen from behind (they face the stage).
    for (let y = 600, row = 0; y < 935 && fans.length < 5000; row++) {
      const s = 4 + (y - 600) * .042, list = [];
      for (let x = -10 + (row % 2) * s * .75 + R() * s; x < AW + 10; x += s * (1.4 + R() * .35)) {
        const side = x < AW / 2 ? 0 : 1, r = R(), team = r < .74 ? side : r < .86 ? 1 - side : -1;
        const tone = Math.floor(R() * 3), col = team < 0 ? (tone < 2 ? '#2b2c35' : '#44454f') : shade(tm[team].col, -.22 - tone * .14);
        list.push({ x, y: y + (R() - .5) * s * .3, s, team, ph: R() * TAU, stick: R() < .3, arm: R(), back: true, ci: ci(col) });
      }
      addRow(y, list);
      y += s * 1.12;
    }
    // Side stands: rows that run towards the stage (we see the fans three-quarter).
    for (let r = 0; r < 14; r++) for (let k = 0; k < 2; k++) {
      const y0 = 222 + r * 28, list = [];
      for (let x = R() * 5; x < 232; x += 6 + R() * 2.5) {
        const sz = (3.6 + r * .2) * (1 - x / 900), y = y0 + (420 - y0) * x / 800, rr = R(), team = rr < .7 ? k : rr < .82 ? 1 - k : -1;
        const tone = Math.floor(R() * 3), col = team < 0 ? (tone < 2 ? '#c9ccd6' : '#2c2e36') : shade(tm[team].col, -.05 - tone * .16);
        list.push({ x: k ? AW - x : x, y, s: sz, team, ph: R() * TAU, stick: R() < .22, arm: R(), back: false, ci: ci(col) });
      }
      addRow(y0, list);
    }
    rows.sort((a, b) => a.y - b.y);
    // Twinkling lights in the far tiers.
    for (let i = 0; i < 160; i++) { const x = R() * AW, y = 122 + R() * 172; if (x > 455 && x < 1145 && y > 100) continue; tw.push({ x, y, ph: R() * TAU, team: x < AW / 2 ? 0 : 1, f: 1 + R() * 3 }); }
    const banners = [];
    for (let i = 0; i < 7; i++) banners.push({ x: 90 + i * 225 + R() * 60, y: 640 + R() * 150, ph: R() * TAU, team: i < 3.5 ? 0 : 1, kind: i % 3 });
    const A = this.arena = { key, fans, rows, pal, tw, banners, stat: null, blur: null, haze: null };
    if (typeof document !== 'undefined') {
      const { W, H, k0 } = this.arenaBand();
      const c = document.createElement('canvas'); c.width = W; c.height = H;
      const g = c.getContext('2d'); g.setTransform(k0, 0, 0, k0, W / 2 - 800 * k0, H / 2 - 450 * k0);
      this.paintArenaStatic(g);
      A.stat = c;
      const b = document.createElement('canvas'); b.width = Math.max(32, Math.ceil(W / 7)); b.height = Math.max(18, Math.ceil(H / 7));
      const bg = b.getContext('2d'); bg.drawImage(c, 0, 0, b.width, b.height);
      bg.setTransform(b.width / AW, 0, 0, b.height / AH, 0, 0);
      bg.globalCompositeOperation = 'lighter';
      for (const [x, col] of [[800, LED_GOLD], [300, tm[0].col], [1300, tm[1].col]]) { const gl = bg.createRadialGradient(x, 260, 0, x, 260, 380); gl.addColorStop(0, rgba(col, .45)); gl.addColorStop(1, rgba(col, 0)); bg.fillStyle = gl; bg.fillRect(0, 0, AW, AH); }
      A.blur = b;
      const hz = document.createElement('canvas'); hz.width = 256; hz.height = 144;
      const hg = hz.getContext('2d'), HR = rng(5);
      for (let i = 0; i < 46; i++) { const x = HR() * 256, y = 20 + HR() * 110, r = 20 + HR() * 50, gr = hg.createRadialGradient(x, y, 0, x, y, r); gr.addColorStop(0, `rgba(190,200,255,${.05 + HR() * .07})`); gr.addColorStop(1, 'rgba(190,200,255,0)'); hg.fillStyle = gr; hg.fillRect(x - r, y - r, 2 * r, 2 * r); }
      A.haze = hz;
      this.buildCrowdAtlas(A, k0);
    }
    return A;
  }

  // The crowd as sprites: every row pre-rendered twice (calm / arms up, sticks
  // raised), cut into short segments that each jump with their own phase.
  buildCrowdAtlas(A, k0) {
    const kk = clamp(k0 * 1.1, .5, 1.5), pad = 2, maxW = Math.ceil((AW + 40) * kk) + pad;
    // Shelf packing: full-width floor rows, the narrow side-stand rows side by side.
    let ax = 0, ay = 0, shelf = 0;
    for (const row of A.rows) {
      let x0 = Infinity, x1 = -Infinity, y0 = Infinity, y1 = -Infinity;
      for (const f of row.list) { x0 = Math.min(x0, f.x - f.s * 1.2); x1 = Math.max(x1, f.x + f.s * 1.4); y0 = Math.min(y0, f.y - f.s * 2.5); y1 = Math.max(y1, f.y + f.s * .8); }
      if (!row.list.length) continue;
      const w = Math.ceil((x1 - x0) * kk) + pad, h = Math.ceil((y1 - y0) * kk) + pad;
      if (ax + w > maxW) { ax = 0; ay += shelf + pad; shelf = 0; }
      row.box = { x0, x1, y0, y1, ax, ay, w, h };
      ax += w + pad; shelf = Math.max(shelf, h);
      // Segments of ~170 world units: each jumps on its own, with the team most of its fans support.
      const n = Math.max(1, Math.round((x1 - x0) / 170)), sw = (x1 - x0) / n, R = rng(Math.round(row.y * 13));
      row.segs = [];
      for (let i = 0; i < n; i++) {
        const a = x0 + i * sw, cnt = [0, 0];
        for (const f of row.list) if (f.x >= a && f.x < a + sw && f.team >= 0) cnt[f.team]++;
        row.segs.push({ x0: a, w: sw, ph: R() * TAU, team: cnt[0] + cnt[1] === 0 ? -1 : cnt[0] >= cnt[1] ? 0 : 1, s: row.list.reduce((m, f) => m + f.s, 0) / row.list.length });
      }
    }
    ay += shelf + pad;
    if (ay > 4096) return;
    const atlas = [0, 1].map(up => {
      const c = document.createElement('canvas'); c.width = maxW; c.height = Math.ceil(ay);
      const g = c.getContext('2d');
      for (const row of A.rows) if (row.box) { g.setTransform(kk, 0, 0, kk, row.box.ax - row.box.x0 * kk, row.box.ay - row.box.y0 * kk); this.paintFanRow(g, A, row, up); }
      return c;
    });
    A.atlas = atlas; A.kk = kk;
  }

  paintFanRow(g, A, row, up) {
    const tm = this.arenaTeams(), sticks = [[], []];
    for (const [c, list] of row.groups) {
      g.beginPath();
      for (const f of list) {
        const s = f.s, y = f.y + (f.back || up ? 0 : s * .5), arms = up && f.arm < .75;
        g.rect(f.x - s * .42, y - s * 1.05, s * .84, s * 1.3);
        if (arms) { g.rect(f.x - s * .5, y - s * 1.95, s * .17, s * .95); g.rect(f.x + s * .33, y - s * 1.95, s * .17, s * .95); }
        if (f.stick && f.team >= 0) sticks[f.team].push(f.x + s * .42, y - s * (arms ? 2.1 : 1.15), s, f.ph);
      }
      g.fillStyle = A.pal[c]; g.fill();
    }
    const back = row.list[0].back;
    g.beginPath();
    for (const f of row.list) { const y = f.y + (f.back || up ? 0 : f.s * .5); g.rect(f.x - f.s * .25, y - f.s * 1.5, f.s * .5, f.s * .5); }
    g.fillStyle = back ? '#0d0a0c' : '#b98564'; g.fill();
    if (back) {
      g.beginPath();
      for (const f of row.list) { g.rect(f.x - f.s * .42, f.y - f.s * 1.05, f.s * .84, f.s * .1); g.rect(f.x - f.s * .25, f.y - f.s * 1.5, f.s * .5, f.s * .08); }
      g.fillStyle = 'rgba(255,226,200,.3)'; g.fill();
    }
    for (let p = 0; p < 2; p++) {
      const c = shade(tm[p].col, .45), L = sticks[p];
      g.fillStyle = rgba(c, .3); g.beginPath();
      for (let i = 0; i < L.length; i += 4) { const s = L[i + 2] * 1.4; g.rect(L[i] - s / 2, L[i + 1] - s * .9, s, s); }
      g.fill();
      g.strokeStyle = rgba(shade(c, .3), 1); g.lineWidth = 1.6; g.beginPath();
      for (let i = 0; i < L.length; i += 4) { const a = (L[i + 3] % 1 - .5) * (up ? 1 : .5), s = L[i + 2]; g.moveTo(L[i], L[i + 1]); g.lineTo(L[i] + Math.sin(a) * s * .8, L[i + 1] - Math.cos(a) * s * .8); }
      g.stroke();
    }
  }

  // The crowd from the atlas: one blit per segment (calm or arms-up pose, a jump, a sway).
  drawFansAtlas(sc, hype) {
    const g = this.ctx, A = this.arena, now = this.now, pos = sc.pos, kk = A.kk;
    const hy = [fanHype(hype, 0, pos), fanHype(hype, 1, pos), fanHype(hype, -1, pos)];
    for (const row of A.rows) {
      const b = row.box; if (!b) continue;
      for (const sg of row.segs) {
        const h = hy[sg.team < 0 ? 2 : sg.team], up = h > .3 ? 1 : 0;
        const j = (h > .05 ? Math.abs(Math.sin(now * 8.5 + sg.ph)) * h * sg.s * .6 : 0) - Math.sin(now * 1.6 + sg.ph) * sg.s * .05;
        // The side stands sit (calm sprite is drawn seated) until they jump up.
        const sx = b.ax + (sg.x0 - b.x0) * kk, sw = Math.min(sg.w * kk + 1, b.ax + b.w - sx);
        g.drawImage(A.atlas[up], sx, b.ay, sw, b.h, sg.x0, b.y0 - j, sw / kk, b.h / kk);
      }
    }
  }

  // World → screen for an arena camera {x, y, z}; returns the scale (pixels per world unit).
  arenaXf(cam) {
    const { W, H, oy, k0 } = this.arenaBand(), hw0 = W / (2 * k0), hh0 = H / (2 * k0);
    const z = Math.max(1, cam.z || 1), hw = hw0 / z, hh = hh0 / z;
    const x = clamp(cam.x, 800 - hw0 + hw, 800 + hw0 - hw), y = clamp(cam.y, 450 - hh0 + hh, 450 + hh0 - hh), k = k0 * z;
    this.ctx.setTransform(k, 0, 0, k, W / 2 - x * k, oy + H / 2 - y * k);
    return { k, k0, hw0, hh0, x, y, z };
  }

  // Where arena scenes are framed: the whole canvas, or on a tall (phone)
  // canvas a 4:3-ish band across the middle so both booths stay in shot.
  arenaBand() {
    const W = this.W, H0 = this.H, H = W / H0 < 1.2 ? Math.min(H0, Math.round(W * .82)) : H0;
    return { W, H, oy: Math.round((H0 - H) / 2), k0: Math.max(W / AW, H / AH) };
  }

  paintArenaStatic(g) {
    const tm = this.arenaTeams(), R = rng(99);
    const bg = g.createLinearGradient(0, 0, 0, AH); bg.addColorStop(0, '#04050a'); bg.addColorStop(.4, '#0a0d1b'); bg.addColorStop(1, '#06070d');
    g.fillStyle = bg; g.fillRect(-60, -60, AW + 120, AH + 120);
    // Far tiers behind the stage, full of tiny fans.
    for (let r = 0; r < 9; r++) {
      const y = 118 + r * 20;
      g.fillStyle = r % 2 ? '#0c0f1e' : '#0a0c18'; g.fillRect(0, y, AW, 20);
      g.fillStyle = 'rgba(255,255,255,.035)'; g.fillRect(0, y, AW, 1.5);
      for (let x = R() * 6; x < AW; x += 4.5 + R() * 4) {
        const side = x < AW / 2 ? 0 : 1, c = R() < .65 ? tm[side].col : R() < .5 ? '#9aa0b5' : tm[1 - side].col;
        g.fillStyle = rgba(c, .22 + R() * .25); g.fillRect(x, y + 7 + R() * 5, 2.6, 4.5);
        g.fillStyle = 'rgba(20,16,18,.8)'; g.fillRect(x + .3, y + 4 + R() * 3, 2, 2);
      }
    }
    // Roof: trusses and light rigs.
    g.fillStyle = '#07080f'; g.fillRect(0, 0, AW, 110);
    g.strokeStyle = '#1b1f31'; g.lineWidth = 2;
    for (const y of [28, 86]) { g.fillStyle = '#141828'; g.fillRect(0, y, AW, 5); }
    g.beginPath(); for (let x = 0; x < AW; x += 40) { g.moveTo(x, 33); g.lineTo(x + 20, 86); g.lineTo(x + 40, 33); } g.stroke();
    g.fillStyle = '#1d2236'; for (let x = 60; x < AW; x += 110) { g.fillRect(x - 7, 91, 14, 10); g.fillRect(x - 3, 101, 6, 4); }
    // Cables holding the big screen.
    g.strokeStyle = '#151827'; g.lineWidth = 2; g.beginPath(); for (const x of [520, 800, 1080]) { g.moveTo(x, 91); g.lineTo(x, 104); } g.stroke();
    // Side stands (structure behind the fans).
    for (let k = 0; k < 2; k++) {
      const X = x => (k ? AW - x : x);
      g.fillStyle = '#0b0d17'; g.beginPath(); g.moveTo(X(0), 205); g.lineTo(X(240), 255); g.lineTo(X(240), 585); g.lineTo(X(0), 640); g.closePath(); g.fill();
      g.strokeStyle = 'rgba(255,255,255,.05)'; g.lineWidth = 1.2; g.beginPath();
      for (let r = 0; r < 15; r++) { const y0 = 226 + r * 28; g.moveTo(X(0), y0); g.lineTo(X(240), y0 + (420 - y0) * .3); }
      g.stroke();
      g.fillStyle = '#11141f'; g.fillRect(k ? AW - 252 : 240, 250, 12, 340);
    }
    // Side screens (frames) and the big screen (frame).
    for (const x of [250, 1250]) { g.fillStyle = '#020306'; g.fillRect(x - 6, 124, 112, 242); g.strokeStyle = '#2a2f45'; g.lineWidth = 2; g.strokeRect(x - 6, 124, 112, 242); }
    g.fillStyle = '#020306'; g.fillRect(458, 100, 684, 254); g.strokeStyle = '#30364f'; g.lineWidth = 3; g.strokeRect(458, 100, 684, 254);
    // Stage: top and front face.
    const st = g.createLinearGradient(0, 470, 0, 545); st.addColorStop(0, '#141a2b'); st.addColorStop(1, '#1d2438');
    g.fillStyle = st; g.beginPath(); g.moveTo(236, 478); g.lineTo(1364, 478); g.lineTo(1404, 546); g.lineTo(196, 546); g.closePath(); g.fill();
    g.strokeStyle = 'rgba(160,180,255,.08)'; g.lineWidth = 1; g.beginPath();
    for (let i = 1; i < 12; i++) { const u = i / 12; g.moveTo(lerp(236, 1364, u), 478); g.lineTo(lerp(196, 1404, u), 546); }
    g.stroke();
    g.fillStyle = '#080a12'; g.fillRect(196, 546, 1208, 46);
    g.fillStyle = '#05060b'; g.fillRect(0, 590, AW, 14);
    g.fillStyle = 'rgba(255,255,255,.06)'; g.fillRect(0, 590, AW, 1.5);
    // Centrepiece plinth.
    g.fillStyle = '#10131f'; g.beginPath(); g.moveTo(752, 500); g.lineTo(848, 500); g.lineTo(830, 470); g.lineTo(770, 470); g.closePath(); g.fill();
    g.fillStyle = '#1a1f33'; g.fillRect(770, 466, 60, 5);
  }

  // ---- the wide shot (also the trophy shot) ----

  drawArenaWide(sc) {
    const g = this.ctx, A = this.arenaPrep(), t = sc.t, now = this.now;
    const hype = crowdHype(sc.level || 0, t, sc.dur), pos = sc.pos;
    const cam = sceneCam(sc, t, pos);
    if (sc.kind !== 'trophy' && hype > .8 && t < 1.2) { const sh = (1 - t / 1.2) * 3; cam.x += (Math.random() - .5) * sh; cam.y += (Math.random() - .5) * sh; }
    const X = this.arenaXf(cam);
    if (A.stat) g.drawImage(A.stat, 800 - X.hw0, 450 - X.hh0, 2 * X.hw0, 2 * X.hh0); else this.paintArenaStatic(g);
    const tm = this.arenaTeams();
    // Twinkles in the far tiers.
    for (const p of A.tw) { const a = .25 + .6 * Math.max(0, Math.sin(now * p.f + p.ph)); g.fillStyle = rgba(shade(tm[p.team].col, .4), a * (.5 + hype * .5)); g.fillRect(p.x, p.y, 2.2, 2.2); }
    this.drawLed(sc, hype);
    this.drawSideScreens(sc, hype);
    this.drawBoothsWide(sc, hype);
    this.drawCentrepiece(sc, hype);
    this.drawStageLeds(sc, hype);
    if (sc.kind === 'trophy') this.drawWinners(sc);
    this.drawBeams(sc, hype);
    if (A.haze) { g.save(); g.globalAlpha = .4 + .15 * hype; g.drawImage(A.haze, -60 + Math.sin(now * .07) * 40, 0, AW + 120, AH); g.restore(); }
    this.drawPyro(sc, hype);
    this.drawFansWide(sc, hype);
    { const sp = g.createRadialGradient(800, 540, 0, 800, 540, 760); sp.addColorStop(0, `rgba(170,190,255,${.16 + .14 * hype})`); sp.addColorStop(1, 'rgba(170,190,255,0)');
      g.save(); g.globalCompositeOperation = 'lighter'; g.fillStyle = sp; g.fillRect(0, 540, AW, 380); g.restore(); }
    this.drawConfetti();
    g.setTransform(1, 0, 0, 1, 0, 0);
    const { W, H, oy } = this.arenaBand(), vg = g.createRadialGradient(W / 2, oy + H * .5, H * .35, W / 2, oy + H * .5, Math.max(W, H) * .75);
    vg.addColorStop(0, 'rgba(0,0,0,0)'); vg.addColorStop(1, 'rgba(0,0,0,.55)'); g.fillStyle = vg; g.fillRect(0, oy, W, H);
    if (hype > .75 && t < .35) { g.fillStyle = `rgba(255,255,255,${.22 * (1 - t / .35)})`; g.fillRect(0, oy, W, H); }
  }

  // The giant screen: the live game (map, kills, gold), the draft, the title, the champions.
  drawLed(sc, hype) {
    const g = this.ctx, st = this.state, tm = this.arenaTeams(), now = this.now;
    const x = 464, y = 106, w = 672, h = 242, cx = x + w / 2;
    g.save(); g.beginPath(); g.rect(x, y, w, h); g.clip();
    const bg = g.createLinearGradient(x, y, x, y + h); bg.addColorStop(0, '#0b1230'); bg.addColorStop(1, '#05081a'); g.fillStyle = bg; g.fillRect(x, y, w, h);
    for (let p = 0; p < 2; p++) { const gl = g.createRadialGradient(p ? x + w : x, y + h / 2, 0, p ? x + w : x, y + h / 2, w * .55); gl.addColorStop(0, rgba(tm[p].col, .35)); gl.addColorStop(1, rgba(tm[p].col, 0)); g.fillStyle = gl; g.fillRect(x, y, w, h); }
    const T2 = (s, xx, yy, size, col, al = 'center', wt = 900) => { g.font = this.font(size, wt); g.fillStyle = col; g.textAlign = al; g.textBaseline = 'middle'; g.fillText(s, xx, yy); };
    const fin = sc.sub === 'finale' || sc.kind === 'trophy';
    if (fin) {
      const w0 = sc.team >= 0 ? sc.team : 0, col = teamColor(st.meta, w0), names = this.names();
      g.fillStyle = rgba(col, .3 + .2 * Math.sin(now * 6)); g.fillRect(x, y, w, h);
      const tr = sc.kind === 'trophy';
      g.save(); g.shadowColor = col; g.shadowBlur = 30; T2(T.champions, cx, y + (tr ? 182 : 92), tr ? 44 : 64, '#fff3c4', 'center', 900); g.restore();
      T2(this.fit((names[w0] || '').toUpperCase(), w - 60, 40, 900), cx, y + (tr ? 222 : 165), tr ? 26 : 40, '#fff');
    } else if (st.mode === 'map') {
      // Live game: the map in the middle, a team panel on each side.
      if (!this.mapCache) this.mapCache = this.buildMapCache();
      const img = this.mapCache, mw = 290, mh = 190, my = y + h / 2 + 8;
      if (img) { const N = img.width; g.save(); g.transform(mw / 2 / N, mh / 2 / N, -mw / 2 / N, mh / 2 / N, cx, my - mh / 2); g.drawImage(img, 0, 0); g.restore(); }
      for (let t = 0; t < 2; t++) for (let r = 0; r < 5; r++) {
        const p = st.pos[t][r]; if (!p || p.alive === false) continue;
        const px = cx + (p.x - p.y) / 100 * mw / 2, py = my - mh / 2 + (p.x + p.y) / 100 * mh / 2;
        g.fillStyle = teamColor(st.meta, t); g.beginPath(); g.arc(px, py, 5, 0, TAU); g.fill(); g.strokeStyle = '#fff'; g.lineWidth = 1.2; g.stroke();
      }
      for (let p = 0; p < 2; p++) {
        const team = tm[p].team, px = p ? x + w - 22 : x + 22, al = p ? 'right' : 'left';
        T2(this.fit(tm[p].name.toUpperCase(), 150, 20, 900), px, y + 34, 20, tm[p].col, al);
        T2(String(st.kills[team] || 0), px, y + 112, 76, '#fff', al);
        T2(fmtGold(st.gold[team]), px, y + 176, 22, LED_GOLD, al, 800);
        T2(`${st.towersK[team] || 0} towers · ${st.dragons[team].length} drakes`, px, y + 206, 13, 'rgba(255,255,255,.7)', al, 700);
      }
      this.panel(cx - 44, y + h - 26, 88, 22, 6, 'rgba(0,0,0,.55)', null);
      T2(fmtClock(this.clock), cx, y + h - 15, 16, '#fff');
    } else if (sc.sub === 'opening' || sc.sub === 'intro' || sc.sub === 'ready' || !st.draft.picks[0].length) {
      // Title card: the event, then the two teams.
      const k = sc.sub === 'ready' ? 1 : clamp((sc.t - .4) / 1.2, 0, 1);
      T2(sc.sub === 'ready' ? 'GAME ON' : T.grandFinal, cx, y + 62, 46, LED_GOLD);
      T2(T.arena, cx, y + 104, 15, 'rgba(255,255,255,.7)', 'center', 800);
      g.globalAlpha = k;
      T2(this.fit(tm[0].name.toUpperCase(), 250, 34, 900), cx - 50 - (1 - k) * 80, y + 172, 34, tm[0].col, 'right');
      T2(this.fit(tm[1].name.toUpperCase(), 250, 34, 900), cx + 50 + (1 - k) * 80, y + 172, 34, tm[1].col, 'left');
      T2('VS', cx, y + 172, 28, '#fff');
      g.globalAlpha = 1;
    } else {
      // The draft so far.
      T2(T.draft, cx, y + 26, 20, LED_GOLD);
      for (let p = 0; p < 2; p++) {
        const team = tm[p].team, picks = st.draft.picks[team];
        T2(this.fit(tm[p].name.toUpperCase(), 280, 18, 900), p ? x + w - 24 : x + 24, y + 62, 18, tm[p].col, p ? 'right' : 'left');
        for (let i = 0; i < 5; i++) {
          const px = p ? x + w - 50 - i * 60 : x + 50 + i * 60, py = y + 130;
          if (picks[i]) drawPortrait(g, px, py, 25, champInfo(st.meta, picks[i].champ), tm[p].col);
          else { g.strokeStyle = 'rgba(255,255,255,.25)'; g.lineWidth = 2; g.beginPath(); g.arc(px, py, 25, 0, TAU); g.stroke(); }
        }
        const bans = st.draft.bans[team];
        for (let i = 0; i < bans.length; i++) drawPortrait(g, p ? x + w - 40 - i * 34 : x + 40 + i * 34, y + 200, 13, champInfo(st.meta, bans[i]), '#555', { banned: true });
      }
    }
    // Pixel sheen.
    const sh = g.createLinearGradient(x, y, x + w, y + h); sh.addColorStop(0, 'rgba(255,255,255,0)'); sh.addColorStop(.5, `rgba(255,255,255,${.04 + .03 * Math.sin(now)})`); sh.addColorStop(1, 'rgba(255,255,255,0)');
    g.fillStyle = sh; g.fillRect(x, y, w, h);
    g.fillStyle = 'rgba(0,0,0,.18)'; for (let yy = y; yy < y + h; yy += 4) g.fillRect(x, yy, w, 1.2);
    g.restore();
    // Glow it throws on the arena.
    const gl = g.createRadialGradient(cx, y + h, 0, cx, y + h, 420); gl.addColorStop(0, `rgba(120,150,255,${.12 + .1 * hype})`); gl.addColorStop(1, 'rgba(120,150,255,0)');
    g.fillStyle = gl; g.fillRect(cx - 420, y + h - 60, 840, 300);
  }

  drawSideScreens(sc, hype) {
    const g = this.ctx, tm = this.arenaTeams(), now = this.now;
    for (let p = 0; p < 2; p++) {
      const x = p ? 1250 : 250, y = 130, w = 100, h = 230, col = tm[p].col, hp = fanHype(hype, p, sc.pos);
      const bg = g.createLinearGradient(0, y, 0, y + h); bg.addColorStop(0, shade(col, -.35)); bg.addColorStop(1, shade(col, -.75)); g.fillStyle = bg; g.fillRect(x, y, w, h);
      // Chevrons running up (faster when their fans roar).
      g.fillStyle = rgba(shade(col, .5), .25 + .5 * hp);
      for (let i = 0; i < 6; i++) {
        const yy = y + h - ((now * (40 + 160 * hp) + i * 42) % (h + 40)) + 20;
        if (yy < y + 8 || yy > y + h - 8) continue;
        g.beginPath(); g.moveTo(x + 18, yy + 10); g.lineTo(x + 50, yy - 8); g.lineTo(x + 82, yy + 10); g.lineTo(x + 82, yy + 18); g.lineTo(x + 50, yy); g.lineTo(x + 18, yy + 18); g.closePath(); g.fill();
      }
      g.save(); g.translate(x + w / 2, y + h / 2); g.rotate(-Math.PI / 2);
      const nm = tm[p].name.toUpperCase(), fsz = clamp((h - 30) / (Math.max(4, nm.length) * .7), 14, 32);
      g.font = this.font(fsz, 900); g.fillStyle = '#fff'; g.textAlign = 'center'; g.textBaseline = 'middle';
      g.fillText(this.fit(nm, h - 20, fsz, 900), 0, 0);
      g.restore();
    }
  }

  // The two glass booths with the five players of each team at their PCs.
  drawBoothsWide(sc, hype) {
    const g = this.ctx, st = this.state, tm = this.arenaTeams(), now = this.now;
    const win = sc.kind === 'trophy' ? sc.pos : -1;
    for (let p = 0; p < 2; p++) {
      const x0 = p ? 900 : 260, y0 = 372, w = 440, h = 120, col = tm[p].col, team = tm[p].team;
      const hb = fanHype(hype, p, sc.pos) * (sc.pos === p ? 1 : 0) + (sc.pos < 0 ? hype * .3 : 0);
      const pulse = .5 + .5 * Math.sin(now * 2.2 + p * 2);
      const wall = g.createLinearGradient(0, y0, 0, y0 + h); wall.addColorStop(0, rgba(col, .3 + .15 * pulse + .4 * hb)); wall.addColorStop(1, rgba(shade(col, -.6), .9));
      g.fillStyle = '#05060b'; g.fillRect(x0, y0, w, h);
      g.fillStyle = wall; g.fillRect(x0, y0, w, h);
      g.fillStyle = rgba(shade(col, .4), .55 + .45 * pulse); g.fillRect(x0 + 6, y0 + 6, w - 12, 3);
      if (win !== p) for (let i = 0; i < 5; i++) {
        const px = x0 + 52 + i * 84, line = st.line[team][i] || {}, hh = hash(line.handle || String(i + p * 5));
        const celebrate = hb > .4 || (sc.mood === 'win' && sc.team === team);
        const jump = celebrate ? Math.abs(Math.sin(now * 7 + i * 1.3)) * 6 * hb : 0, rise = celebrate ? 14 : 0;
        const arms = celebrate ? (i % 2 ? POSE.up : wavePose(i, now, 1)) : POSE.type;
        drawPerson(g, px, y0 + 108 - rise - jump, 30, { shirt: col, trim: '#ffffff', skin: SKINS[hh % SKINS.length], hair: HAIRS[(hh >> 3) % HAIRS.length], hs: (hh >> 6) % 4, headset: true, mouth: celebrate ? .8 : 0, arms });
        // Monitor (we see its back) and its glow on the player.
        g.fillStyle = '#0a0b10'; g.fillRect(px - 15, y0 + 86, 30, 14);
        g.fillStyle = rgba('#9fd8ff', .1 + .06 * Math.sin(now * 3 + i)); g.fillRect(px - 18, y0 + 50, 36, 36);
      }
      g.fillStyle = '#0c0e16'; g.fillRect(x0, y0 + 100, w, 20);
      g.fillStyle = rgba(col, .8); g.fillRect(x0, y0 + 100, w, 2);
      // Glass, frame and the name sign.
      g.fillStyle = 'rgba(190,215,255,.05)'; g.fillRect(x0, y0, w, h);
      g.fillStyle = 'rgba(255,255,255,.07)'; g.beginPath(); g.moveTo(x0 + 40, y0); g.lineTo(x0 + 90, y0); g.lineTo(x0 + 30, y0 + h); g.lineTo(x0 - 20, y0 + h); g.closePath(); g.fill();
      g.strokeStyle = '#2b3047'; g.lineWidth = 4; g.strokeRect(x0, y0, w, h);
      g.fillStyle = '#2b3047'; g.fillRect(x0 + w / 2 - 2, y0, 4, h);
      g.fillStyle = '#06070c'; g.fillRect(x0 - 6, y0 - 30, w + 12, 26);
      g.fillStyle = rgba(col, .65 + .35 * pulse); g.fillRect(x0 - 6, y0 - 6, w + 12, 3);
      g.font = this.font(17, 900); g.fillStyle = '#fff'; g.textAlign = 'center'; g.textBaseline = 'middle';
      g.fillText(this.fit(tm[p].name.toUpperCase(), w - 30, 17, 900), x0 + w / 2, y0 - 17);
      // Light it throws on the stage.
      const sg = g.createRadialGradient(x0 + w / 2, y0 + h + 10, 0, x0 + w / 2, y0 + h + 10, 260); sg.addColorStop(0, rgba(col, .2 + .3 * hb)); sg.addColorStop(1, rgba(col, 0));
      g.fillStyle = sg; g.fillRect(x0 - 60, y0 + h - 10, w + 120, 80);
    }
  }

  // A floating crystal between the booths (it turns gold, or the winner's colour).
  drawCentrepiece(sc, hype) {
    const g = this.ctx, now = this.now, cx = 800, cy = 402 + Math.sin(now * 1.3) * 5;
    const fin = sc.sub === 'finale' || sc.kind === 'trophy';
    const col = fin && sc.team >= 0 ? teamColor(this.state.meta, sc.team) : '#58d6ff';
    const gl = g.createRadialGradient(cx, cy, 0, cx, cy, 120 + 60 * hype); gl.addColorStop(0, rgba(col, .45 + .3 * hype)); gl.addColorStop(1, rgba(col, 0));
    g.fillStyle = gl; g.fillRect(cx - 200, cy - 200, 400, 400);
    // Light shaft from the plinth.
    const sf = g.createLinearGradient(0, 470, 0, cy - 80); sf.addColorStop(0, rgba(col, .35)); sf.addColorStop(1, rgba(col, 0));
    g.fillStyle = sf; g.beginPath(); g.moveTo(cx - 26, 470); g.lineTo(cx + 26, 470); g.lineTo(cx + 50, cy - 80); g.lineTo(cx - 50, cy - 80); g.closePath(); g.fill();
    if (sc.kind === 'trophy') return;
    // Rings.
    g.lineWidth = 2;
    for (let i = 0; i < 2; i++) { g.strokeStyle = rgba(i ? LED_GOLD : col, .6); g.beginPath(); g.ellipse(cx, cy + 6, 70 + i * 16, 14 + i * 4, Math.sin(now * .6 + i) * .15, 0, TAU); g.stroke(); }
    // The crystal: an octahedron spinning.
    const a = now * .9, top = [cx, cy - 58], bot = [cx, cy + 48];
    const eq = [0, 1, 2, 3].map(i => { const b = a + i * Math.PI / 2; return [cx + Math.cos(b) * 38, cy + Math.sin(b) * 9, Math.sin(b)]; });
    const faces = [];
    for (let i = 0; i < 4; i++) { const p = eq[i], q = eq[(i + 1) % 4], z = (p[2] + q[2]) / 2; faces.push([p, q, z]); }
    faces.sort((u, v) => u[2] - v[2]);
    for (const [p, q, z] of faces) for (const apex of [top, bot]) {
      const lit = clamp(.45 + .45 * Math.cos(Math.atan2(q[1] - p[1], q[0] - p[0])) * (apex === top ? 1 : -.6), .1, 1);
      g.fillStyle = z > 0 ? shade(col, lit - .3) : shade(col, -.55);
      g.beginPath(); g.moveTo(apex[0], apex[1]); g.lineTo(p[0], p[1]); g.lineTo(q[0], q[1]); g.closePath(); g.fill();
    }
    g.strokeStyle = 'rgba(255,255,255,.5)'; g.lineWidth = 1; g.beginPath(); g.moveTo(top[0], top[1]); g.lineTo(bot[0], bot[1]); g.stroke();
  }

  // LED strip along the stage front: a chase in the team colours, all flashing on big moments.
  drawStageLeds(sc, hype) {
    const g = this.ctx, tm = this.arenaTeams(), now = this.now;
    for (let i = 0; i < 60; i++) {
      const x = 200 + i * 20, side = i < 30 ? 0 : 1;
      const b = .25 + .75 * Math.max(0, Math.sin(now * 5 - i * .45)), flash = hype > .6 ? .5 + .5 * Math.sin(now * 14) : 0;
      const col = sc.pos >= 0 && hype > .6 ? tm[sc.pos].col : tm[side].col;
      g.fillStyle = rgba(shade(col, .3), clamp(b * (1 - flash) + flash, 0, 1)); g.fillRect(x + 2, 552, 16, 7);
    }
    g.fillStyle = rgba(LED_GOLD, .7); g.fillRect(196, 545, 1208, 2);
  }

  // Moving-head light beams from the roof, sweeping the stage and the crowd.
  drawBeams(sc, hype) {
    const g = this.ctx, tm = this.arenaTeams(), now = this.now;
    const rigs = [150, 410, 640, 960, 1190, 1450];
    g.save(); g.globalCompositeOperation = 'lighter';
    rigs.forEach((rx, i) => {
      const sp = .35 + .12 * i + hype * .9;
      const ang = (i < 3 ? .25 : -.25) + Math.sin(now * sp + i * 1.7) * .42;
      const L = 820, ex = rx + Math.sin(ang) * L, ey = 96 + Math.cos(ang) * L;
      const col = i === 2 || i === 3 ? LED_GOLD : tm[i < 3 ? 0 : 1].col;
      const c2 = sc.pos >= 0 && hype > .5 ? tm[sc.pos].col : col;
      const gr = g.createLinearGradient(rx, 96, ex, ey); gr.addColorStop(0, rgba(shade(c2, .3), .3 + .25 * hype)); gr.addColorStop(1, rgba(c2, 0));
      const nx = Math.cos(ang), ny = -Math.sin(ang), w0 = 4, w1 = 70;
      g.fillStyle = gr; g.beginPath();
      g.moveTo(rx + nx * w0, 96 + ny * w0); g.lineTo(ex + nx * w1, ey + ny * w1); g.lineTo(ex - nx * w1, ey - ny * w1); g.lineTo(rx - nx * w0, 96 - ny * w0); g.closePath(); g.fill();
      const lg = g.createRadialGradient(rx, 98, 0, rx, 98, 16); lg.addColorStop(0, rgba('#ffffff', .9)); lg.addColorStop(1, rgba(c2, 0)); g.fillStyle = lg; g.fillRect(rx - 16, 82, 32, 32);
    });
    g.restore();
  }

  // Flame columns along the stage front on the biggest moments.
  drawPyro(sc, hype) {
    const fin = sc.sub === 'finale' || sc.kind === 'trophy';
    if (!(fin || (hype > .8 && sc.t < 1.8))) return;
    const g = this.ctx, now = this.now, life = fin ? 1 : clamp((1.8 - sc.t) / .6, 0, 1);
    g.save(); g.globalCompositeOperation = 'lighter';
    [214, 510, 1090, 1386].forEach((x, i) => {
      const fl = .7 + .3 * Math.sin(now * 31 + i * 2) * Math.sin(now * 17 + i), hgt = 170 * fl * life * (fin ? (.6 + .4 * Math.abs(Math.sin(now * 1.7 + i))) : 1);
      if (hgt < 5) return;
      const gr = g.createLinearGradient(0, 546, 0, 546 - hgt); gr.addColorStop(0, 'rgba(255,250,220,.95)'); gr.addColorStop(.35, 'rgba(255,170,60,.75)'); gr.addColorStop(1, 'rgba(255,80,20,0)');
      g.fillStyle = gr; g.beginPath(); g.moveTo(x - 14, 546); g.quadraticCurveTo(x - 26, 546 - hgt * .5, x, 546 - hgt); g.quadraticCurveTo(x + 26, 546 - hgt * .5, x + 14, 546); g.closePath(); g.fill();
    });
    g.restore();
  }

  // Thousands of fans: blocky bodies, heads, raised arms, light sticks, banners, phone flashes.
  drawFansWide(sc, hype) {
    const g = this.ctx, A = this.arena, tm = this.arenaTeams(), now = this.now, pos = sc.pos;
    const sticks = [[], []], rim = `rgba(255,226,200,${.22 + .2 * hype})`;
    const hy = [fanHype(hype, 0, pos), fanHype(hype, 1, pos), fanHype(hype, -1, pos)];
    if (A.atlas) this.drawFansAtlas(sc, hype);
    else for (const row of A.rows) {
      // Bodies (and raised arms), one path per colour; each fan's head position is kept for the next pass.
      for (const [c, list] of row.groups) {
        g.beginPath();
        for (const f of list) {
          const h = hy[f.team < 0 ? 2 : f.team], s = f.s;
          const j = h > .05 ? Math.abs(Math.sin(now * 8.5 + f.ph)) * h * s * .6 : 0;
          // Side-stand fans sit until their moment, then jump to their feet.
          const seat = f.back ? 0 : s * .5 * (1 - clamp((h - .25) * 3, 0, 1));
          const y = f.y - j + seat + Math.sin(now * 1.6 + f.ph) * s * .05;
          f.hy = y;
          g.rect(f.x - s * .42, y - s * 1.05, s * .84, s * 1.3);
          if (h > .3 && f.arm < h * 1.15) { const w = Math.sin(now * 7 + f.ph) * s * .15; g.rect(f.x - s * .5 + w, y - s * 1.95, s * .17, s * .95); g.rect(f.x + s * .33 + w, y - s * 1.95, s * .17, s * .95); f.up = true; } else f.up = false;
          if (f.stick && f.team >= 0) sticks[f.team].push(f.x + s * .42, y - s * (f.up ? 2.1 : 1.15), s, f.ph);
        }
        g.fillStyle = A.pal[c]; g.fill();
      }
      // Heads: hair from behind on the floor, faces in the side stands; stage light catching the backs.
      const back = row.list[0] && row.list[0].back;
      g.beginPath();
      for (const f of row.list) g.rect(f.x - f.s * .25, f.hy - f.s * 1.5, f.s * .5, f.s * .5);
      g.fillStyle = back ? '#0d0a0c' : '#b98564'; g.fill();
      if (back) {
        g.beginPath();
        for (const f of row.list) { g.rect(f.x - f.s * .42, f.hy - f.s * 1.05, f.s * .84, f.s * .1); g.rect(f.x - f.s * .25, f.hy - f.s * 1.5, f.s * .5, f.s * .08); }
        g.fillStyle = rim; g.fill();
      }
    }
    // Light sticks (additive), swaying more when their team scores.
    g.save(); g.globalCompositeOperation = 'lighter';
    for (let p = 0; p < 2; p++) {
      const c = shade(tm[p].col, .45), h = fanHype(hype, p, pos), L = sticks[p];
      g.strokeStyle = rgba(c, .9); g.lineWidth = 1.6; g.beginPath();
      for (let i = 0; i < L.length; i += 4) { const a = Math.sin(now * (2 + 6 * h) + L[i + 3]) * (.3 + .6 * h), s = L[i + 2]; g.moveTo(L[i], L[i + 1]); g.lineTo(L[i] + Math.sin(a) * s * .8, L[i + 1] - Math.cos(a) * s * .8); }
      g.stroke();
      g.fillStyle = rgba(c, .16 + .2 * h); g.beginPath();
      for (let i = 0; i < L.length; i += 4) { const s = L[i + 2] * 1.4; g.rect(L[i] - s / 2, L[i + 1] - s * .9, s, s); }
      g.fill();
    }
    // Phone flashes.
    const n = Math.floor(hype * 14) + (Math.random() < .3 ? 1 : 0);
    for (let k = 0; k < n; k++) {
      const f = A.fans[Math.floor(Math.random() * A.fans.length)], r = f.s * 1.8;
      const fg = g.createRadialGradient(f.x, f.y - f.s * 1.6, 0, f.x, f.y - f.s * 1.6, r); fg.addColorStop(0, 'rgba(255,255,255,.95)'); fg.addColorStop(1, 'rgba(255,255,255,0)');
      g.fillStyle = fg; g.fillRect(f.x - r, f.y - f.s * 1.6 - r, 2 * r, 2 * r);
    }
    g.restore();
    // Banners held up in the crowd.
    for (const b of A.banners) {
      const h = fanHype(hype, b.team, pos), s = 4 + (b.y - 600) * .042, wv = Math.sin(now * (2 + 5 * h) + b.ph) * s * .25;
      const bw = s * 9, bh = s * 2.6, x = b.x - bw / 2, y = b.y - s * 3.4 - h * s * .8;
      g.fillStyle = '#b8b8b8'; g.fillRect(x, y, s * .2, bh + s * 1.2); g.fillRect(x + bw - s * .2, y + wv, s * .2, bh + s * 1.2);
      g.fillStyle = b.kind === 1 ? '#f2f2f2' : tm[b.team].col;
      g.beginPath(); g.moveTo(x, y); g.lineTo(x + bw, y + wv); g.lineTo(x + bw, y + bh + wv); g.lineTo(x, y + bh); g.closePath(); g.fill();
      const txt = b.kind === 0 ? `GO ${tm[b.team].name.toUpperCase()}` : b.kind === 1 ? initials(tm[b.team].name).toUpperCase() + ' ♥' : 'LET\'S GO!';
      g.save(); g.translate(x + bw / 2, y + bh / 2 + wv / 2); g.transform(1, wv / bw, 0, 1, 0, 0);
      g.font = this.font(bh * .5, 900); g.fillStyle = b.kind === 1 ? tm[b.team].col : '#fff'; g.textAlign = 'center'; g.textBaseline = 'middle';
      g.fillText(this.fit(txt, bw * .9, bh * .5, 900), 0, 0);
      g.restore();
    }
  }

  drawConfetti() {
    const g = this.ctx;
    for (const q of this.confetti) {
      const c = Math.cos(q.r);
      g.fillStyle = q.c; g.fillRect(q.x - q.s / 2, q.y - q.s * .3 * Math.abs(c) - .5, q.s, q.s * .6 * Math.abs(c) + 1);
    }
  }

  // The champions on stage, the middle one lifting the cup.
  drawWinners(sc) {
    const g = this.ctx, st = this.state, team = sc.team >= 0 ? sc.team : 0, col = teamColor(st.meta, team), now = this.now, t = sc.t;
    const lift = easeOut(clamp((t - .5) / 1.2, 0, 1));
    for (const i of [0, 4, 1, 3, 2]) {
      const line = st.line[team][i] || {}, hh = hash(line.handle || String(i)), mid = i === 2;
      const x = 800 + (i - 2) * 78, jump = mid ? Math.abs(Math.sin(now * 3)) * 5 * lift : Math.abs(Math.sin(now * 6.5 + i * 1.7)) * 8;
      const arms = mid ? [[lerp(.2, 2.72, lift), lerp(.3, 3.0, lift)], [lerp(.2, 2.72, lift), lerp(.3, 3.0, lift)]] : i % 2 ? POSE.up : wavePose(i, now, 1);
      const r = drawPerson(g, x, 500 - jump, 38, { shirt: col, trim: '#ffffff', skin: SKINS[hh % SKINS.length], hair: HAIRS[(hh >> 3) % HAIRS.length], hs: (hh >> 6) % 4, legs: true, pants: '#14161e', mouth: .7 + .3 * Math.sin(now * 9 + i), arms });
      if (mid) { const [a, b] = r.hands, cx = (a[0] + b[0]) / 2, cy = Math.min(a[1], b[1]) + 8; drawCup(g, cx, cy, 20, true); }
    }
  }

  // ---- close-ups ----

  // Fans close-up: rows in the stands going wild (with sticks, scarves and signs).
  drawCrowdClose(sc) {
    const g = this.ctx, tm = this.arenaTeams(), now = this.now, t = sc.t, pos = sc.pos;
    const hype = crowdHype(sc.level || .6, t, sc.dur);
    if (!sc.fans) {
      const R = rng(1000 + sc.n * 77), rows = [];
      for (let r = 0; r < 5; r++) {
        const S = 62 + r * 18, y = 330 + r * 122, row = [];
        for (let x = -S * .6 + (r % 2) * S * .55 + R() * S * .3; x < AW + S; x += S * (1.12 + R() * .2)) {
          const rr = R(), home = pos >= 0 ? pos : x < AW / 2 ? 0 : 1, team = rr < .78 ? home : rr < .86 ? 1 - home : -1;
          const hh = Math.floor(R() * 1e6), acc = R();
          row.push({ x, team, ph: R() * TAU, skin: SKINS[hh % SKINS.length], hair: HAIRS[(hh >> 4) % HAIRS.length], hs: (hh >> 8) % 4, acc: acc < .22 ? 'stick' : acc < .34 ? 'scarf' : acc < .4 ? 'sign' : '', arm: R(), neutral: ['#e9e9ee', '#23252d', '#54586a'][hh % 3] });
        }
        rows.push({ S, y, row });
      }
      sc.fans = rows;
    }
    const e = easeInOut(clamp(t / sc.dur, 0, 1)), shake = hype > .7 ? (1 - clamp(t / 1.2, 0, 1)) * 4 : 0;
    this.arenaXf({ x: 800 + (pos === 0 ? -1 : pos === 1 ? 1 : 0) * 30 * e + (Math.random() - .5) * shake, y: 450 + (Math.random() - .5) * shake, z: lerp(1.02, 1.1, e) });
    const bg = g.createLinearGradient(0, 0, 0, AH); bg.addColorStop(0, '#06070e'); bg.addColorStop(1, '#10131f'); g.fillStyle = bg; g.fillRect(-40, -40, AW + 80, AH + 80);
    // Bokeh of the far stands and the stage lights.
    const BR = rng(31);
    for (let i = 0; i < 34; i++) {
      const x = BR() * AW, y = BR() * 260, r = 18 + BR() * 40, c = BR() < .4 ? LED_GOLD : tm[BR() < .5 ? 0 : 1].col, a = .1 + .12 * (.5 + .5 * Math.sin(now * (1 + BR()) + i));
      const gl = g.createRadialGradient(x, y, 0, x, y, r); gl.addColorStop(0, rgba(c, a)); gl.addColorStop(.7, rgba(c, a * .6)); gl.addColorStop(1, rgba(c, 0)); g.fillStyle = gl; g.fillRect(x - r, y - r, 2 * r, 2 * r);
    }
    // A beam cutting across.
    g.save(); g.globalCompositeOperation = 'lighter';
    const bc = pos >= 0 ? tm[pos].col : LED_GOLD, bx = 800 + Math.sin(now * .8) * 500;
    const bgr = g.createLinearGradient(bx, -40, bx + 200, 700); bgr.addColorStop(0, rgba(bc, .35 * (.4 + hype))); bgr.addColorStop(1, rgba(bc, 0));
    g.fillStyle = bgr; g.beginPath(); g.moveTo(bx - 20, -40); g.lineTo(bx + 20, -40); g.lineTo(bx + 380, 760); g.lineTo(bx + 60, 760); g.closePath(); g.fill();
    g.restore();
    for (const { S, y, row } of sc.fans) {
      for (const f of row) {
        const h = fanHype(hype, f.team, pos), jump = Math.abs(Math.sin(now * 8.5 + f.ph)) * h * S * .32;
        const shirt = f.team < 0 ? f.neutral : tm[f.team].col, up = h > .3 && f.arm < h * 1.25;
        let arms = up ? (f.acc ? POSE.up : f.arm < .3 ? POSE.fist : wavePose(f.ph, now, h)) : h > .12 ? POSE.clap : POSE.down;
        if (!up && h > .12 && Math.sin(now * 10 + f.ph) > 0) arms = [[.3, 2.0], [.3, 2.0]];
        const r = drawPerson(g, f.x, y - jump, S, { shirt, trim: f.team >= 0 ? '#ffffff' : null, skin: f.skin, hair: f.hair, hs: f.hs, mouth: h * (.55 + .45 * Math.abs(Math.sin(now * 11 + f.ph))), arms });
        const [L, Rh] = r.hands;
        if (f.acc === 'stick' && f.team >= 0) {
          const c = shade(tm[f.team].col, .5), a = Math.sin(now * (3 + 6 * h) + f.ph) * .5;
          g.save(); g.globalCompositeOperation = 'lighter';
          g.strokeStyle = c; g.lineWidth = S * .09; g.lineCap = 'round'; g.beginPath(); g.moveTo(Rh[0], Rh[1]); g.lineTo(Rh[0] + Math.sin(a) * S * .55, Rh[1] - Math.cos(a) * S * .55); g.stroke();
          const gl = g.createRadialGradient(Rh[0], Rh[1] - S * .3, 0, Rh[0], Rh[1] - S * .3, S * .6); gl.addColorStop(0, rgba(c, .35)); gl.addColorStop(1, rgba(c, 0)); g.fillStyle = gl; g.fillRect(Rh[0] - S * .6, Rh[1] - S * .9, S * 1.2, S * 1.2);
          g.restore(); g.lineCap = 'butt';
        } else if (f.acc === 'scarf' && f.team >= 0 && up) {
          const c = tm[f.team].col, sag = S * .12 + Math.sin(now * 6 + f.ph) * S * .05;
          g.strokeStyle = c; g.lineWidth = S * .16; g.beginPath(); g.moveTo(L[0], L[1]); g.quadraticCurveTo((L[0] + Rh[0]) / 2, (L[1] + Rh[1]) / 2 + sag, Rh[0], Rh[1]); g.stroke();
          g.strokeStyle = '#ffffff'; g.lineWidth = S * .04; g.stroke();
        } else if (f.acc === 'sign' && up) {
          const cx = (L[0] + Rh[0]) / 2, cy = Math.min(L[1], Rh[1]) - S * .2, w = S * 1.4, hh = S * .7;
          g.fillStyle = '#f4f1e8'; g.fillRect(cx - w / 2, cy - hh / 2, w, hh);
          g.font = this.font(S * .26, 900); g.fillStyle = f.team >= 0 ? tm[f.team].col : '#222'; g.textAlign = 'center'; g.textBaseline = 'middle';
          g.fillText(f.team >= 0 ? this.fit(`GO ${initials(tm[f.team].name).toUpperCase()}!`, w * .9, S * .26, 900) : 'GG WP', cx, cy);
        }
      }
      // The seat row in front hides the legs.
      const sb = g.createLinearGradient(0, y + S * .05, 0, y + S * 1.2); sb.addColorStop(0, '#1a1e2e'); sb.addColorStop(1, '#0a0c14');
      g.fillStyle = sb; g.fillRect(-40, y + S * .05, AW + 80, 500);
      g.fillStyle = 'rgba(255,255,255,.08)'; g.fillRect(-40, y + S * .05, AW + 80, 2);
    }
    // LED ribbon board along the front.
    g.fillStyle = '#05060a'; g.fillRect(-40, 836, AW + 80, 70);
    const label = `${sc.label || ''}  ★  `, col = pos >= 0 ? tm[pos].col : LED_GOLD;
    g.font = this.font(40, 900); const lw = Math.max(200, g.measureText(label).width);
    g.fillStyle = shade(col, .35); g.textAlign = 'left'; g.textBaseline = 'middle';
    for (let x = -((now * 260) % lw) - 40; x < AW + 40; x += lw) g.fillText(label, x, 868);
    g.fillStyle = 'rgba(0,0,0,.25)'; for (let x = -40; x < AW + 40; x += 5) g.fillRect(x, 836, 1.5, 64);
    g.setTransform(1, 0, 0, 1, 0, 0);
  }

  // Inside a team booth: five players at their PCs (focused in the draft,
  // jumping out of their chairs after a big objective or the win).
  drawBoothClose(sc) {
    const g = this.ctx, st = this.state, now = this.now, t = sc.t, team = sc.team >= 0 ? sc.team : 0;
    const col = teamColor(st.meta, team), name = this.names()[team] || '';
    const win = sc.mood === 'win', hype = win ? crowdHype(sc.level || 1, t, sc.dur) : 0;
    const e = easeInOut(clamp(t / sc.dur, 0, 1));
    this.arenaXf({ x: lerp(780, 820, e), y: 440, z: lerp(1.0, 1.04, e) });
    const wall = g.createLinearGradient(0, 0, 0, AH); wall.addColorStop(0, shade(col, -.55)); wall.addColorStop(.7, shade(col, -.8)); wall.addColorStop(1, '#05060a');
    g.fillStyle = wall; g.fillRect(-40, -40, AW + 80, AH + 80);
    // Light strips and the team name on the back wall.
    const pulse = .5 + .5 * Math.sin(now * 2.4);
    for (const [y, a] of [[150, .6], [250, .35]]) { g.fillStyle = rgba(shade(col, .5), a * (.5 + .5 * pulse) + .4 * hype); g.fillRect(-40, y, AW + 80, 6); }
    g.save(); g.globalAlpha = .13 + .1 * hype; g.font = this.font(170, 900, true); g.fillStyle = '#ffffff'; g.textAlign = 'center'; g.textBaseline = 'middle';
    g.fillText(this.fit(name.toUpperCase(), 1500, 170, 900), 800, 360); g.restore();
    if (win) { const fl = g.createRadialGradient(800, 300, 0, 800, 300, 900); fl.addColorStop(0, rgba(col, .35 * hype)); fl.addColorStop(1, rgba(col, 0)); g.fillStyle = fl; g.fillRect(-40, -40, AW + 80, AH + 80); }
    // Players.
    for (let i = 0; i < 5; i++) {
      const line = st.line[team][i] || {}, hh = hash(line.handle || String(i)), px = 210 + i * 295;
      const cel = win && t > .15 + i * .07, jump = cel ? Math.abs(Math.sin(now * 7 + i * 1.4)) * 26 * hype : 0;
      const rise = cel ? 95 : 0, S = 150;
      // Chair back.
      g.fillStyle = '#0c0d13'; g.beginPath(); g.roundRect(px - 92, 470, 184, 260, 26); g.fill();
      g.fillStyle = rgba(col, .85); g.fillRect(px - 92, 500, 184, 14);
      const talk = !win && Math.sin(now * 1.3 + i * 2.1) > .55;
      let arms = cel ? (i % 2 ? POSE.up : i === 2 ? POSE.fist : wavePose(i, now, 1)) : [[.38 + .04 * Math.sin(now * 9 + i), -1.2], [.38 + .04 * Math.sin(now * 8 + i * 2), -1.2]];
      if (talk) arms = [[.38, -1.2], [.45, 2.2 + .2 * Math.sin(now * 5)]];
      const lean = cel ? Math.sin(now * 3 + i) * .08 : 0;
      drawPerson(g, px, 725 - rise - jump, S, { shirt: col, trim: '#ffffff', skin: SKINS[hh % SKINS.length], hair: HAIRS[(hh >> 3) % HAIRS.length], hs: (hh >> 6) % 4, headset: true, lean, mouth: cel ? .6 + .4 * Math.abs(Math.sin(now * 10 + i)) : talk ? .3 + .3 * Math.abs(Math.sin(now * 12)) : 0, arms });
      // Monitor to the side, its glow on the player.
      const mx = px + 118;
      g.fillStyle = '#07080c'; g.beginPath(); g.moveTo(mx - 10, 560); g.lineTo(mx + 12, 548); g.lineTo(mx + 12, 708); g.lineTo(mx - 10, 700); g.closePath(); g.fill();
      g.fillStyle = rgba('#8fd3ff', .75); g.beginPath(); g.moveTo(mx - 10, 566); g.lineTo(mx - 5, 563); g.lineTo(mx - 5, 696); g.lineTo(mx - 10, 694); g.closePath(); g.fill();
      if (!cel) { const mg = g.createRadialGradient(mx - 30, 600, 0, mx - 30, 600, 140); mg.addColorStop(0, 'rgba(140,210,255,.18)'); mg.addColorStop(1, 'rgba(140,210,255,0)'); g.fillStyle = mg; g.fillRect(mx - 170, 460, 280, 280); }
    }
    // Desk, keyboards, name plates.
    g.fillStyle = '#0b0c12'; g.fillRect(-40, 715, AW + 80, 200);
    g.fillStyle = rgba(col, .9); g.fillRect(-40, 715, AW + 80, 5);
    g.fillStyle = 'rgba(255,255,255,.06)'; g.fillRect(-40, 720, AW + 80, 2);
    for (let i = 0; i < 5; i++) {
      const px = 210 + i * 295, line = st.line[team][i] || {};
      this.panel(px - 105, 722, 210, 38, 8, 'rgba(6,8,14,.92)', rgba(col, .7));
      g.font = this.font(19, 900); g.fillStyle = '#fff'; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText(this.fit((line.handle || ROLES[i]).toUpperCase(), 190, 19, 900), px, 735);
      g.font = this.font(11, 800); g.fillStyle = shade(col, .5); g.fillText(this.fit(`${T.roleShort[ROLES[i]]}${line.champ ? ' · ' + line.champ.toUpperCase() : ''}`, 190, 11, 800), px, 751);
    }
    // Glass: frame and reflections.
    g.fillStyle = '#05060a'; g.fillRect(-40, -40, AW + 80, 70); g.fillRect(-40, -40, 60, AH + 80); g.fillRect(AW - 20, -40, 60, AH + 80);
    g.fillStyle = rgba(col, .8); g.fillRect(20, 28, AW - 40, 3);
    g.save(); g.globalCompositeOperation = 'screen';
    const rx = ((now * 60) % 2400) - 600;
    const rg = g.createLinearGradient(rx, 0, rx + 260, 0); rg.addColorStop(0, 'rgba(255,255,255,0)'); rg.addColorStop(.5, 'rgba(255,255,255,.07)'); rg.addColorStop(1, 'rgba(255,255,255,0)');
    g.fillStyle = rg; g.beginPath(); g.moveTo(rx, 30); g.lineTo(rx + 260, 30); g.lineTo(rx - 40, AH); g.lineTo(rx - 300, AH); g.closePath(); g.fill();
    g.restore();
    if (sc.confetti) this.drawConfetti();
    g.setTransform(1, 0, 0, 1, 0, 0);
  }

  // The caster desk: three casters, the arena blurred behind them.
  drawCasters(sc) {
    const g = this.ctx, A = this.arenaPrep(), now = this.now, t = sc.t, tm = this.arenaTeams();
    const e = easeInOut(clamp(t / sc.dur, 0, 1));
    g.setTransform(1, 0, 0, 1, 0, 0);
    const B = this.arenaBand();
    if (A.blur) { g.imageSmoothingEnabled = true; g.drawImage(A.blur, 0, B.oy, B.W, B.H); }
    this.arenaXf({ x: lerp(760, 800, e), y: 450, z: lerp(1.0, 1.05, e) });
    if (!A.blur) { const bg = g.createLinearGradient(0, 0, 0, AH); bg.addColorStop(0, '#0a0d1b'); bg.addColorStop(1, '#05060b'); g.fillStyle = bg; g.fillRect(-40, -40, AW + 80, AH + 80); }
    // Bokeh lights behind.
    const BR = rng(57);
    for (let i = 0; i < 26; i++) {
      const x = BR() * AW, y = 60 + BR() * 420, r = 16 + BR() * 36, c = BR() < .5 ? LED_GOLD : tm[x < AW / 2 ? 0 : 1].col, a = .12 + .1 * Math.sin(now * (.6 + BR()) + i);
      const gl = g.createRadialGradient(x, y, 0, x, y, r); gl.addColorStop(0, rgba(c, a)); gl.addColorStop(1, rgba(c, 0)); g.fillStyle = gl; g.fillRect(x - r, y - r, 2 * r, 2 * r);
    }
    g.fillStyle = 'rgba(4,5,10,.35)'; g.fillRect(-40, -40, AW + 80, AH + 80);
    // Casters.
    const suits = ['#1d2335', '#2c1d27', '#1b2a26'], ties = [LED_GOLD, '#8a1c2c', '#2f6db5'];
    const speaker = Math.floor(now / 2.2) % 3;
    for (let i = 0; i < 3; i++) {
      const x = 470 + i * 330, hh = hash('caster' + i), talk = i === speaker;
      const rest = [[.38, -1.2], [.38, -1.2]];
      const arms = i === 1 ? POSE.mic : talk ? [[.3, -2.6], [.45 + .1 * Math.sin(now * 3), 2.0 + .3 * Math.sin(now * 4)]] : rest;
      const r = drawPerson(g, x, 640, 175, { shirt: suits[i], suit: true, trim: ties[i], skin: SKINS[(hh + i * 2) % SKINS.length], hair: HAIRS[(hh >> 3) % HAIRS.length], hs: (hh >> 6) % 4, mouth: talk ? .3 + .5 * Math.abs(Math.sin(now * 13)) : 0, arms, lean: Math.sin(now * .7 + i) * .03 });
      if (i === 1 || talk) {
        const h = i === 1 ? r.hands[1] : r.hands[0];
        g.fillStyle = '#0d0e12'; g.save(); g.translate(h[0], h[1]); g.rotate(i === 1 ? -.5 : .5);
        g.fillRect(-7, -48, 14, 52); g.fillStyle = '#3a3d47'; g.beginPath(); g.arc(0, -52, 12, 0, TAU); g.fill();
        g.fillStyle = LED_GOLD; g.fillRect(-7, -22, 14, 8); g.restore();
      }
    }
    // The desk.
    const dg = g.createLinearGradient(0, 610, 0, 900); dg.addColorStop(0, '#1b1f2e'); dg.addColorStop(1, '#07080d');
    g.fillStyle = dg; g.beginPath(); g.moveTo(170, 630); g.quadraticCurveTo(800, 600, 1430, 630); g.lineTo(1480, 920); g.lineTo(120, 920); g.closePath(); g.fill();
    g.fillStyle = '#2a2f44'; g.beginPath(); g.moveTo(160, 622); g.quadraticCurveTo(800, 590, 1440, 622); g.lineTo(1440, 640); g.quadraticCurveTo(800, 608, 160, 640); g.closePath(); g.fill();
    for (let i = 0; i < 48; i++) { const x = 190 + i * 25.6, b = .35 + .65 * Math.max(0, Math.sin(now * 3 - i * .3)); g.fillStyle = rgba(LED_GOLD, b); g.fillRect(x, 690, 19, 5); }
    g.font = this.font(46, 900); g.fillStyle = 'rgba(255,255,255,.9)'; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText(T.arena, 800, 760);
    g.font = this.font(18, 800); g.fillStyle = LED_GOLD; g.fillText('LIVE · CASTER DESK', 800, 806);
    g.setTransform(1, 0, 0, 1, 0, 0);
  }

  // ---- scene dispatch and overlays ----

  drawScene() {
    const sc = this.scene, g = this.ctx, B = this.arenaBand();
    g.save();
    if (B.oy > 0) { g.fillStyle = '#04050a'; g.fillRect(0, 0, this.W, this.H); g.beginPath(); g.rect(0, B.oy, B.W, B.H); g.clip(); }
    try {
      if (sc.kind === 'crowd') this.drawCrowdClose(sc);
      else if (sc.kind === 'booth') this.drawBoothClose(sc);
      else if (sc.kind === 'casters') this.drawCasters(sc);
      else this.drawArenaWide(sc);
    } finally { g.restore(); g.setTransform(1, 0, 0, 1, 0, 0); }
    this.drawSceneHud(sc);
  }

  sceneText(sc) {
    const st = this.state, names = this.names(), team = sc.team >= 0 ? sc.team : -1, nm = team >= 0 ? names[team] : '';
    const col = team >= 0 ? teamColor(st.meta, team) : LED_GOLD, tm = this.arenaTeams();
    const vs = `${tm[0].name} vs ${tm[1].name}`;
    if (sc.kind === 'trophy') return [`${nm.toUpperCase()} ARE THE CHAMPIONS`, 'The cup is lifted on stage', col];
    if (sc.sub === 'finale') return [`${nm.toUpperCase()} WIN!`, 'The nexus falls and the arena erupts', col];
    if (sc.sub === 'opening') return [sc.label || T.grandFinal, `Live from the arena · ${vs}`, LED_GOLD];
    if (sc.sub === 'ready') return ['GAME ON', 'Draft locked in · welcome to Summoner\'s Rift', LED_GOLD];
    if (sc.kind === 'casters') {
      if (sc.sub === 'intro') return ['THE CASTERS', `Champion draft coming up · ${vs}`, LED_GOLD];
      const picks = [0, 1].map(p => st.draft.picks[tm[p].team].map(x => x.champ).join(', ')).filter(Boolean);
      return [sc.label || 'CASTERS', picks.length ? `First picks: ${picks.join('  |  ')}` : 'Breaking down the draft', LED_GOLD];
    }
    if (sc.kind === 'booth') return [`${nm.toUpperCase()}`, sc.mood === 'win' ? (sc.label === 'VICTORY' ? 'The players jump out of their chairs' : `${sc.label} secured · the booth celebrates`) : `${sc.label || 'DRAFT'} · locking in champions`, col];
    if (sc.kind === 'crowd') return [sc.label || 'THE CROWD', team >= 0 ? `${nm} fans on their feet` : 'The arena roars', col];
    return [sc.label || 'ARENA', team >= 0 ? `The whole arena on its feet for ${nm}` : 'The arena roars', col];
  }

  drawSceneHud(sc) {
    const g = this.ctx, W = this.W, H = this.H, s = Math.min(W / 1280, H / 720);
    const onMap = this.state.mode === 'map' && !sc.fin;
    if (onMap) this.drawTopBar();
    // Camera tag.
    const tag = CAM_TAGS[sc.kind] || 'ARENA', ty = onMap ? this.topH + 8 * s : 14 * s, fs = 12 * s, th = 24 * s;
    g.font = this.font(fs, 900); const tw = g.measureText(tag).width + 34 * s;
    this.panel(14 * s, ty, tw, th, 4 * s, 'rgba(200,30,40,.95)', null);
    g.fillStyle = `rgba(255,255,255,${.6 + .4 * Math.sin(this.now * 7)})`; g.beginPath(); g.arc(14 * s + 11 * s, ty + th / 2, 3.5 * s, 0, TAU); g.fill();
    this.txt(tag, 14 * s + 20 * s, ty + th / 2, fs, '#fff', 'left', 900);
    // Lower third.
    const [title, sub, col] = this.sceneText(sc);
    const k = easeOut(clamp(sc.t / .35, 0, 1)) * clamp((sc.dur - sc.t) / .25, 0, 1);
    if (k <= 0) return;
    const x = 14 * s, y = ty + th + 8 * s, tfs = 28 * s, sfs = 13 * s;
    g.font = this.font(tfs, 900, true); const w1 = g.measureText(title).width;
    g.font = this.font(sfs, 700); const w2 = g.measureText(sub).width;
    const w = Math.min(W - 56 * s, Math.max(w1, w2) + 48 * s) * k;
    g.save(); g.beginPath(); g.rect(x, y - 4 * s, w, 80 * s); g.clip();
    this.panel(x, y, Math.max(w, 1), 46 * s, 4 * s, 'rgba(6,8,14,.9)', null);
    g.fillStyle = col; g.fillRect(x, y, 6 * s, 46 * s);
    g.font = this.font(tfs, 900, true); g.fillStyle = '#fff'; g.textAlign = 'left'; g.textBaseline = 'middle'; g.fillText(title, x + 20 * s, y + 24 * s, W - 100 * s);
    this.panel(x, y + 48 * s, Math.max(w, 1), 26 * s, 4 * s, rgba(col, .85), null);
    this.txt(this.fit(sub, W - 100 * s, sfs, 700), x + 20 * s, y + 61 * s, sfs, '#fff', 'left', 700);
    g.restore();
  }

  // A quick wipe between cameras.
  drawStinger() {
    const a = (this.now - this.stinger) / .42;
    if (!(a >= 0 && a < 1) || !this.state) return;
    const g = this.ctx, W = this.W, H = this.H, x = lerp(-W * .5, W * 1.3, easeInOut(a)), sk = H * .35;
    const tm = this.arenaTeams();
    g.save();
    const band = (off, w, col) => { g.fillStyle = col; g.beginPath(); g.moveTo(x + off, 0); g.lineTo(x + off + w, 0); g.lineTo(x + off + w - sk, H); g.lineTo(x + off - sk, H); g.closePath(); g.fill(); };
    band(-W * .16, W * .08, tm[0].col); band(-W * .08, W * .12, LED_GOLD); band(W * .04, W * .08, tm[1].col);
    g.restore();
  }
}
