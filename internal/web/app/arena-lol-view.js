// League of Legends viewer for the Arena (internal/arena/lol), drawn like an
// esports broadcast: a punchy draft, then an isometric Summoner's Rift with
// champion tokens, minion waves, fights, objectives and each team's plan, a
// broadcast top bar, a stats panel (rosters, objectives, gold graph) and a
// victory screen. Everything on screen is English, whatever the page language.
//
// Pure helpers (createState, applyEvent, layout, caption…) don't touch the DOM
// and are tested with node (arena-lol-view.test.cjs).

const lerp = (a, b, t) => a + (b - a) * t;
const clamp = (v, a, b) => Math.max(a, Math.min(b, v));
const easeOut = t => 1 - Math.pow(1 - t, 3);
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
function rng(seed) { let s = seed >>> 0 || 1; return () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296); }
function hexToRgb(h) { const m = /^#?([0-9a-f]{6})$/i.exec(h || ''); if (!m) return [128, 128, 128]; const n = parseInt(m[1], 16); return [n >> 16, (n >> 8) & 255, n & 255]; }
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
    case 'phase': return d.phase === 'macro' ? 2.6 : d.phase === 'draft' ? 1.0 : d.phase === 'draft_done' ? 1.8 : d.phase === 'laning' ? 2.4 : 2.0;
    case 'draft_ban': return 0.75;
    case 'draft_pick': return 1.15;
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

// Procedural champion portrait: class-coloured emblem with initials.
export function drawPortrait(g, x, y, r, info, ring, opts = {}) {
  const cls = info.class || 'bruiser';
  const base = opts.dead ? '#555a63' : (CLASS_COLORS[cls] || '#888');
  const hue = (hash(info.name) % 40) - 20;
  g.save(); g.translate(x, y);
  if (opts.glow) { g.shadowColor = ring; g.shadowBlur = r * .9; }
  g.beginPath(); g.arc(0, 0, r, 0, TAU);
  const grd = g.createRadialGradient(-r * .3, -r * .4, r * .1, 0, 0, r);
  grd.addColorStop(0, shade(base, .35 + hue / 200)); grd.addColorStop(.7, base); grd.addColorStop(1, shade(base, -.45));
  g.fillStyle = grd; g.fill(); g.shadowBlur = 0;
  g.save(); g.clip(); g.globalAlpha = .18; g.fillStyle = '#fff'; g.rotate((hash(info.name) % 6) * .5);
  g.fillRect(-r, -r * .25, r * 2, r * .3); g.restore();
  g.globalAlpha = .26; g.fillStyle = '#fff'; g.strokeStyle = '#fff';
  (CLASS_GLYPH[cls] || CLASS_GLYPH.bruiser)(g, r * .62);
  g.globalAlpha = 1;
  if (r > 7) {
    g.fillStyle = '#fff'; g.font = `800 ${Math.round(r * .8)}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle';
    g.shadowColor = 'rgba(0,0,0,.7)'; g.shadowBlur = r * .25; g.fillText(initials(info.name), 0, r * .05); g.shadowBlur = 0;
  }
  g.lineWidth = Math.max(1.5, r * .17); g.strokeStyle = opts.dead ? '#777' : ring; g.beginPath(); g.arc(0, 0, r, 0, TAU); g.stroke();
  if (opts.banned) {
    g.fillStyle = 'rgba(20,20,24,.55)'; g.beginPath(); g.arc(0, 0, r, 0, TAU); g.fill();
    const k = opts.slash == null ? 1 : opts.slash;
    g.strokeStyle = '#ff3b4b'; g.lineWidth = r * .2; g.lineCap = 'round'; g.beginPath(); g.moveTo(-r * .7, -r * .7); g.lineTo(-r * .7 + r * 1.4 * k, -r * .7 + r * 1.4 * k); g.stroke();
  }
  g.restore();
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

export class LolView {
  constructor(canvas, opts = {}) {
    this.cv = canvas; this.ctx = canvas.getContext('2d');
    this.lang = 'en'; this.T = T;
    this.source = opts.source;
    this.onUpdate = opts.onUpdate || (() => {});
    this.onEvent = opts.onEvent || (() => {});
    this.match = null; this.state = null; this.queue = []; this.idx = 0; this.speed = 1; this.paused = false;
    this.cur = null; this.prog = 0; this.lastN = 0; this.stopped = false; this.last = performance.now();
    this.flashes = []; this.particles = []; this.popups = []; this.skulls = []; this.clashes = []; this.killFeed = [];
    this.banner = null; this.mapCache = null; this.now = 0; this.anim = 0; this.clock = 0;
    this.hp = [[1, 1, 1, 1, 1], [1, 1, 1, 1, 1]]; this.planFlash = -10; this.draftAnim = { t0: 0 };
    // Action cam: the current shot, the camera tween, ability effects, a replay.
    this.shot = null; this.camTw = null; this.acts = []; this.pending = []; this.replay = null;
    this.lastShotEnd = -99; this.lastReplay = -999; this.cutFlash = -10; this.hiCache = null; this.hiTimer = null;
    this.dash = [[], []]; this.air = [[], []];
    this.camC = { fX: 0, fY: 0, z: 1 }; this.camT = 0; this.camFocus = [50, 50]; this.zoomed = false;
    this.resize = this.resize.bind(this);
    if (typeof window !== 'undefined') window.addEventListener('resize', this.resize);
    this.resize();
    this.loop = this.loop.bind(this); requestAnimationFrame(this.loop);
    this.poll();
  }

  destroy() { this.stopped = true; if (typeof window !== 'undefined') window.removeEventListener('resize', this.resize); clearTimeout(this.pollTimer); clearTimeout(this.hiTimer); }
  setSpeed(x) { this.speed = Math.max(.1, +x || 1); }
  togglePause() { this.paused = !this.paused; return this.paused; }
  skipToEnd() {
    if (!this.state) return;
    if (this.cur) this.cur = null;
    while (this.idx < this.queue.length) { const ev = this.queue[this.idx++]; applyEvent(this.state, ev); this.onEvent(ev); }
    this.snapPositions(); this.banner = null; this.flashes = []; this.popups = []; this.clashes = [];
    this.shot = null; this.replay = null; this.camTw = null; this.acts = []; this.pending = [];
    this.clock = this.state.t;
    if (this.state.end) this.state.mode = 'victory';
  }
  isFinished() {
    const m = this.match;
    if (!m || !['final', 'stopped', 'failed'].includes(m.status)) return false;
    if (this.cur || this.replay || this.idx < this.queue.length) return false;
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
      }
    } catch (e) { /* retry */ }
    const done = this.match && ['final', 'stopped', 'failed'].includes(this.match.status);
    if (!done) this.pollTimer = setTimeout(() => this.poll(), 3000);
  }

  resize() {
    const r = this.cv.getBoundingClientRect ? this.cv.getBoundingClientRect() : { width: this.cv.width, height: this.cv.height };
    const d = (typeof window !== 'undefined' && window.devicePixelRatio) || 1;
    this.cv.width = Math.max(320, Math.round(r.width * d)); this.cv.height = Math.max(240, Math.round(r.height * d));
    this.W = this.cv.width; this.H = this.cv.height; this.dpr = d;
    this.mapCache = null; this.hiCache = null; clearTimeout(this.hiTimer); this.hiTimer = null;
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

  loop(now) {
    if (this.stopped) return;
    const dt = Math.min(.25, (now - this.last) / 1000); this.last = now; this.now = now / 1000;
    if (!this.paused) this.advance(dt * this.speed, dt);
    try { this.draw(); } catch (e) { /* one bad frame doesn't stop the viewer */ }
    requestAnimationFrame(this.loop);
  }

  advance(dt) {
    if (!this.state) return;
    // A replay pauses the match: only the replay clock runs.
    if (this.replay) { this.replay.t += dt; if (this.replay.t >= this.replay.len) this.finishReplay(); return; }
    this.anim += dt;
    if (this.pending.length) this.pending = this.pending.filter(p => this.anim < p.at || (p.fn(), false));
    if (!this.cur && this.idx < this.queue.length && this.state.mode !== 'victory') {
      const ev = this.queue[this.idx++];
      const wasDraft = this.state.mode === 'draft';
      const fx = applyEvent(this.state, ev);
      if (wasDraft && this.state.mode === 'map') this.snapPositions();
      this.cur = { ev, fx }; this.prog = 0;
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
      this.onEvent(ev);
      this.onUpdate(this.match, ev);
    }
    if (this.cur) {
      this.prog += dt / this.cur.fx.dur;
      if (this.prog >= 1) {
        const ev = this.cur.ev; this.cur = null;
        if (ev.kind === 'match_end') this.state.mode = 'victory';
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
    if (this.state.mode === 'draft') this.drawDraft();
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
    if (!this.cur && this.idx >= this.queue.length && this.match && this.match.status !== 'final' && this.state.mode !== 'victory') {
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
}
