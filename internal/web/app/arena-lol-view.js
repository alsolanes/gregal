// Visor de la partida de League of Legends de l'Arena (internal/arena/lol):
// pantalla de draft com una retransmissió (picks i bans amb retrats fets a mà),
// després la Clam de l'Invocador vista des de dalt amb els campions movent-se,
// baralles, objectius, torres, marcador, gràfic d'or i la pantalla de victòria.
//
// Els helpers purs (createState, applyEvent, layout…) no toquen el DOM i es
// proven amb node (arena-lol-view.test.cjs).

const lerp = (a, b, t) => a + (b - a) * t;
const clamp = (v, a, b) => Math.max(a, Math.min(b, v));
const easeOut = t => 1 - Math.pow(1 - t, 3);
const FONT = '"Segoe UI", system-ui, sans-serif';

export const TEXT = {
  ca: {
    draft: 'DRAFT', bans: 'Vetos', picks: 'Tries', pregame: 'Els equips preparen el draft…', waiting: 'Esperant la decisió següent…',
    live: 'EN DIRECTE', victory: 'VICTÒRIA', defeat: 'DERROTA', mvp: 'MVP', duration: 'Durada', kills: 'Morts', towers: 'Torres', dragons: 'Dracs', gold: 'Or', barons: 'Barons',
    firstBlood: 'PRIMERA SANG', ace: 'ACE', shutdown: 'SHUTDOWN', dragonStolen: 'DRAC ROBAT!', baronStolen: 'BARÓ ROBAT!', heraldStolen: 'HERALD ROBAT!',
    soul: t => `ÀNIMA ${t.toUpperCase()}`, drake: t => `DRAC ${t.toUpperCase()}`, elder: 'DRAC ANCESTRAL', baron: 'BARÓ NASHOR', herald: 'HERALD', grubs: 'LARVES DEL BUIT',
    inhib: 'INHIBIDOR DESTRUÏT', nexus: 'NEXE DESTRUÏT', comeback: 'REMUNTADA!', golddiff: 'Diferència d\'or', ban: 'VETA', pick: 'TRIA', laning: 'FASE DE LÍNIES',
    roles: { top: 'Top', jungle: 'Jungla', mid: 'Mid', adc: 'ADC', support: 'Suport' }, side: ['BLAU', 'VERMELL'], stance: 'postura', objective: 'objectiu',
  },
  en: {
    draft: 'DRAFT', bans: 'Bans', picks: 'Picks', pregame: 'Teams are preparing the draft…', waiting: 'Waiting for the next decision…',
    live: 'LIVE', victory: 'VICTORY', defeat: 'DEFEAT', mvp: 'MVP', duration: 'Duration', kills: 'Kills', towers: 'Towers', dragons: 'Dragons', gold: 'Gold', barons: 'Barons',
    firstBlood: 'FIRST BLOOD', ace: 'ACE', shutdown: 'SHUTDOWN', dragonStolen: 'DRAGON STOLEN!', baronStolen: 'BARON STOLEN!', heraldStolen: 'HERALD STOLEN!',
    soul: t => `${t.toUpperCase()} SOUL`, drake: t => `${t.toUpperCase()} DRAKE`, elder: 'ELDER DRAGON', baron: 'BARON NASHOR', herald: 'RIFT HERALD', grubs: 'VOID GRUBS',
    inhib: 'INHIBITOR DESTROYED', nexus: 'NEXUS DESTROYED', comeback: 'COMEBACK!', golddiff: 'Gold difference', ban: 'BAN', pick: 'PICK', laning: 'LANING PHASE',
    roles: { top: 'Top', jungle: 'Jungle', mid: 'Mid', adc: 'ADC', support: 'Support' }, side: ['BLUE', 'RED'], stance: 'stance', objective: 'objective',
  },
};

export const ROLES = ['top', 'jungle', 'mid', 'adc', 'support'];
export const CLASS_COLORS = { tank: '#7d8ea3', bruiser: '#c8692c', assassin: '#8e44ad', mage: '#5161d6', marksman: '#d8a020', enchanter: '#27ae78', engage: '#13958a' };
export const DRAGON_COLORS = { infernal: '#ff5a36', mountain: '#b0874f', ocean: '#2ec4c4', cloud: '#d6e4f0', hextech: '#4fd8ff', chemtech: '#8ef05a', elder: '#f5e17a' };
const SIDE_COLORS = [['#2f7df6', '#9cc8ff'], ['#e8414e', '#ffb3b8']];
const FOUNTAIN = [[5, 95], [95, 5]];

// Mapa per defecte (el mateix que torna Meta().map al motor).
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

// ---- helpers purs ----

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

// Equip (0/1) → costat del mapa (0 blau, 1 vermell).
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

export function champInfo(meta, name) {
  const list = (meta && meta.champions) || [];
  return list.find(c => c.name === name) || { name: name || '?', class: 'bruiser', roles: [], curve: 'mid', dmg: 'AD' };
}

export function fmtClock(sec) { const s = Math.max(0, Math.floor(sec || 0)); return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`; }

// Posició de cada rol a la fase de línies (costat blau); el vermell és la transposada.
const LANE_POS = [[10, 27], [27, 60], [43, 57], [74, 91], [71, 88]];
const tr = p => [p[1], p[0]];
export function homePos(side, role) { const p = LANE_POS[role] || [50, 50]; return side === 0 ? p.slice() : tr(p); }

export function respawnTime(t) { return 8 + (t / 60) * 1.15; }

export function eventDuration(ev) {
  const d = ev && ev.data || {};
  switch (ev && ev.kind) {
    case 'phase': return d.phase === 'macro' ? 2.6 : d.phase === 'draft' ? 1.4 : 2.4;
    case 'draft_ban': return 1.3;
    case 'draft_pick': return 1.9;
    case 'lane': return 1.1;
    case 'gank': return 1.4;
    case 'kill': return ev.hl ? 1.9 : 1.0;
    case 'teamfight': return 1.5;
    case 'dragon': case 'baron': case 'herald': return ev.hl ? 2.8 : 1.9;
    case 'tower': return ev.hl ? 1.6 : 1.2;
    case 'inhibitor': return d.respawn ? 1 : 2.2;
    case 'nexus': return 3.2;
    case 'match_end': return 1.5;
  }
  return 1;
}

export function createState(meta, names) {
  meta = meta || {};
  const map = meta.map || defaultMap();
  const state = {
    meta, map, names: names || (meta.teams || []).map(t => t.name) || ['', ''],
    mode: 'draft', t: 0, phase: '', clockFrom: 0, clockTo: 0,
    draft: { bans: [[], []], picks: [[], []], step: '', active: null },
    line: [[], []], kills: [0, 0], towersK: [0, 0], gold: [2500, 2500], goldPts: [[0, 0]],
    dragons: [[], []], barons: [0, 0], grubs: [0, 0], heralds: [0, 0],
    towers: {}, inhibs: {}, nexus: [true, true],
    dead: [[0, 0, 0, 0, 0], [0, 0, 0, 0, 0]], feed: [], plan: [null, null], buff: [{}, {}],
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

// Punt d'ancoratge del pla macro d'un equip.
function planAnchor(state, team) {
  const side = mapSide(state.meta, team), plan = state.plan[team] || {};
  const m = state.map;
  const toward = (p, k) => { const f = FOUNTAIN[side]; return [lerp(p[0], f[0], k), lerp(p[1], f[1], k)]; };
  switch (plan.objective) {
    case 'dragon': case 'elder': return toward([m.dragon.x, m.dragon.y], .08);
    case 'baron': case 'herald': return toward([m.baron.x, m.baron.y], .08);
    case 'farm': return side === 0 ? [30, 62] : [62, 30];
    case 'towers': {
      const enemy = 1 - side;
      const t = (m.towers || []).filter(x => x.side === enemy && x.lane === 'mid' && state.towers[`${x.side}-${x.lane}-${x.tier}`]).sort((a, b) => a.tier - b.tier)[0];
      return t ? toward([t.x, t.y], .12) : [50, 50];
    }
  }
  return side === 0 ? [42, 58] : [58, 42];
}

const RING = [[0, 0], [2.6, -1.4], [-2.6, 1.4], [1.6, 2.4], [-1.6, -2.4]];

// layout: on ha d'anar cada campió per a aquest esdeveniment.
export function layout(state, ev) {
  const d = (ev && ev.data) || {};
  const out = [[], []];
  const laning = state.t < 840;
  for (let team = 0; team < 2; team++) {
    const side = mapSide(state.meta, team);
    const anchor = planAnchor(state, team);
    for (let r = 0; r < 5; r++) {
      let p;
      if (laning || !state.plan[team]) p = homePos(side, r);
      else {
        p = [anchor[0] + RING[r][0], anchor[1] + RING[r][1]];
        if ((state.plan[team] || {}).stance === 'split' && r === 0) p = side === 0 ? [72, 88] : [12, 30];
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
  if (x !== null && y !== null) {
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
  // Els morts esperen a la font.
  for (let team = 0; team < 2; team++) {
    const f = FOUNTAIN[mapSide(state.meta, team)];
    for (let r = 0; r < 5; r++) {
      if (!isAlive(state, team, r)) out[team][r] = { x: f[0] + RING[r][0] * .6, y: f[1] + RING[r][1] * .6, alive: false };
      out[team][r].x = clamp(out[team][r].x, 1, 99); out[team][r].y = clamp(out[team][r].y, 1, 99);
    }
  }
  return out;
}

// applyEvent: aplica un esdeveniment a l'estat i torna què cal animar.
export function applyEvent(state, ev) {
  const T = TEXT[state.lang === 'ca' ? 'ca' : 'en'];
  const d = (ev && ev.data) || {};
  const fx = { banner: null, flashes: [], focus: null, dur: eventDuration(ev) };
  state.clockFrom = state.t;
  if (Number.isFinite(d.t) && d.t >= state.t) state.t = d.t;
  state.clockTo = state.t;
  if (d.hud) {
    if (Array.isArray(d.hud.k)) state.kills = d.hud.k.slice();
    if (Array.isArray(d.hud.tw)) state.towersK = d.hud.tw.slice();
    if (Array.isArray(d.hud.g)) { state.gold = d.hud.g.slice(); const last = state.goldPts[state.goldPts.length - 1]; const pt = [state.t, state.gold[0] - state.gold[1]]; if (!last || last[0] !== pt[0]) state.goldPts.push(pt); else last[1] = pt[1]; }
    if (Array.isArray(d.hud.bn)) state.barons = d.hud.bn.slice();
    if (Array.isArray(d.hud.gr)) state.grubs = d.hud.gr.slice();
  } else if (Array.isArray(ev.score)) state.kills = ev.score.slice();
  const team = d.side === 1 ? 1 : 0;
  const color = teamColor(state.meta, team);
  const banner = (text, c = color, big = true) => { fx.banner = { text, color: c, big }; };
  state.ticker = ev.text || '';
  switch (ev.kind) {
    case 'phase':
      state.phase = d.phase || '';
      if (d.phase === 'draft') state.mode = 'draft';
      if (Array.isArray(d.lineups)) d.lineups.forEach((l, t) => (l || []).forEach((p, r) => { if (state.line[t] && p) state.line[t][r] = { role: p.role, handle: p.handle, champ: p.champ }; }));
      if (d.phase === 'laning') { state.mode = 'map'; banner(T.laning, '#d8b25a'); }
      if (d.phase === 'macro' && Array.isArray(d.choices)) state.plan = d.choices.map(c => c || null);
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
      if ((v.side === 0 || v.side === 1) && v.slot >= 0 && v.slot < 5) state.dead[v.side][v.slot] = state.t + respawnTime(state.t);
      state.feed.unshift({ killer: k, victim: v, t: state.t, multi: d.multi || 0, ace: !!d.ace });
      state.feed.length = Math.min(state.feed.length, 6);
      fx.flashes.push({ x: d.x, y: d.y, color: teamColor(state.meta, k.side === 1 ? 1 : 0), big: false });
      const kc = teamColor(state.meta, k.side === 1 ? 1 : 0);
      if (d.multi >= 5) banner('PENTAKILL', kc);
      else if (d.ace) banner(T.ace, kc);
      else if (d.multi === 4) banner('QUADRA KILL', kc);
      else if (d.multi === 3) banner('TRIPLE KILL', kc, false);
      else if (d.tag === 'first_blood') banner(T.firstBlood, kc, false);
      else if (d.shutdown) banner(T.shutdown, kc, false);
      break;
    }
    case 'teamfight':
      fx.flashes.push({ x: d.x, y: d.y, color: '#fff2c0', big: true });
      break;
    case 'dragon':
      if (d.type === 'elder') { state.buff[team].elder = state.t + 150; banner(d.stolen ? T.dragonStolen : T.elder, DRAGON_COLORS.elder); }
      else {
        state.dragons[team].push(d.type);
        if (d.stolen) banner(T.dragonStolen, DRAGON_COLORS[d.type] || color);
        else if (d.soul) banner(T.soul(d.type || ''), DRAGON_COLORS[d.type] || color);
        else banner(T.drake(d.type || ''), DRAGON_COLORS[d.type] || color, false);
      }
      fx.flashes.push({ x: d.x, y: d.y, color: DRAGON_COLORS[d.type] || '#ffb347', big: true });
      break;
    case 'baron':
      state.buff[team].baron = state.t + 180;
      banner(d.stolen ? T.baronStolen : T.baron, '#9b59ff');
      fx.flashes.push({ x: d.x, y: d.y, color: '#b77bff', big: true });
      break;
    case 'herald':
      if (d.obj === 'grubs') banner(T.grubs, '#a78bfa', false);
      else { state.heralds[team]++; banner(d.stolen ? T.heraldStolen : T.herald, '#a78bfa', !!d.stolen); }
      fx.flashes.push({ x: d.x, y: d.y, color: '#a78bfa', big: false });
      break;
    case 'tower': {
      const owner = d.owner === 1 ? 1 : 0;
      state.towers[`${mapSide(state.meta, owner)}-${d.lane}-${d.tier}`] = false;
      fx.flashes.push({ x: d.x, y: d.y, color: '#ffcf6b', big: false, debris: true });
      break;
    }
    case 'inhibitor': {
      const owner = d.owner === 1 ? 1 : 0;
      const key = `${mapSide(state.meta, owner)}-${d.lane}`;
      state.inhibs[key] = !!d.respawn;
      if (!d.respawn) { banner(T.inhib, color); fx.flashes.push({ x: d.x, y: d.y, color: '#ffcf6b', big: true, debris: true }); }
      break;
    }
    case 'nexus':
      state.nexus[mapSide(state.meta, d.owner === 1 ? 1 : 0)] = false;
      banner(T.nexus, color);
      fx.flashes.push({ x: d.x, y: d.y, color: '#ffffff', big: true, debris: true });
      break;
    case 'match_end':
      state.end = d;
      if (Array.isArray(d.graph) && d.graph.length > 1) state.goldPts = d.graph.map(p => [p[0], p[1]]);
      break;
  }
  if (Number.isFinite(d.x) && Number.isFinite(d.y)) fx.focus = { x: d.x, y: d.y };
  state.target = layout(state, ev);
  return fx;
}

// ---- dibuix ----

const CLASS_GLYPH = {
  tank(g, r) { g.beginPath(); g.moveTo(0, -r); g.lineTo(r * .85, -r * .55); g.lineTo(r * .7, r * .45); g.lineTo(0, r); g.lineTo(-r * .7, r * .45); g.lineTo(-r * .85, -r * .55); g.closePath(); g.fill(); },
  bruiser(g, r) { g.beginPath(); g.moveTo(-r * .2, r); g.lineTo(-r * .2, -r * .5); g.lineTo(-r * .9, -r * .9); g.lineTo(r * .9, -r * .9); g.lineTo(r * .2, -r * .5); g.lineTo(r * .2, r); g.closePath(); g.fill(); },
  assassin(g, r) { g.beginPath(); g.moveTo(0, -r); g.lineTo(r * .3, r * .3); g.lineTo(0, r); g.lineTo(-r * .3, r * .3); g.closePath(); g.fill(); },
  mage(g, r) { g.beginPath(); for (let i = 0; i < 10; i++) { const a = -Math.PI / 2 + i * Math.PI / 5, rr = i % 2 ? r * .42 : r; g.lineTo(Math.cos(a) * rr, Math.sin(a) * rr); } g.closePath(); g.fill(); },
  marksman(g, r) { g.lineWidth = r * .22; g.beginPath(); g.moveTo(-r * .8, r * .8); g.lineTo(r * .6, -r * .6); g.stroke(); g.beginPath(); g.moveTo(r * .95, -r * .95); g.lineTo(r * .2, -r * .7); g.lineTo(r * .7, -r * .2); g.closePath(); g.fill(); },
  enchanter(g, r) { g.beginPath(); g.moveTo(0, r * .9); g.bezierCurveTo(-r * 1.2, 0, -r * .5, -r * 1.1, 0, -r * .4); g.bezierCurveTo(r * .5, -r * 1.1, r * 1.2, 0, 0, r * .9); g.fill(); },
  engage(g, r) { g.lineWidth = r * .26; g.beginPath(); g.moveTo(-r * .8, -r * .1); g.lineTo(0, r * .7); g.lineTo(r * .8, -r * .1); g.stroke(); g.beginPath(); g.moveTo(-r * .8, -r * .8); g.lineTo(0, 0); g.lineTo(r * .8, -r * .8); g.stroke(); },
};

// Retrat procedural d'un campió: emblema del color de la classe amb inicials.
export function drawPortrait(g, x, y, r, info, ring, opts = {}) {
  const cls = info.class || 'bruiser';
  const base = CLASS_COLORS[cls] || '#888';
  const hue = (hash(info.name) % 40) - 20;
  g.save(); g.translate(x, y);
  if (opts.glow) { g.shadowColor = ring; g.shadowBlur = r * .9; }
  g.beginPath(); g.arc(0, 0, r, 0, Math.PI * 2);
  const grd = g.createRadialGradient(-r * .3, -r * .4, r * .1, 0, 0, r);
  grd.addColorStop(0, shade(base, .35 + hue / 200)); grd.addColorStop(.7, base); grd.addColorStop(1, shade(base, -.45));
  g.fillStyle = grd; g.fill(); g.shadowBlur = 0;
  // Motiu de fons: franja diagonal (varia per campió).
  g.save(); g.clip(); g.globalAlpha = .18; g.fillStyle = '#fff'; g.rotate((hash(info.name) % 6) * .5);
  g.fillRect(-r, -r * .25, r * 2, r * .3); g.restore();
  g.globalAlpha = .28; g.fillStyle = '#fff'; g.strokeStyle = '#fff';
  (CLASS_GLYPH[cls] || CLASS_GLYPH.bruiser)(g, r * .62);
  g.globalAlpha = 1;
  if (r > 9) {
    g.fillStyle = '#fff'; g.font = `800 ${Math.round(r * .78)}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle';
    g.shadowColor = 'rgba(0,0,0,.6)'; g.shadowBlur = r * .2; g.fillText(initials(info.name), 0, r * .05); g.shadowBlur = 0;
  }
  g.lineWidth = Math.max(1.5, r * .16); g.strokeStyle = ring; g.beginPath(); g.arc(0, 0, r, 0, Math.PI * 2); g.stroke();
  if (opts.dead) {
    g.fillStyle = 'rgba(10,10,14,.6)'; g.beginPath(); g.arc(0, 0, r, 0, Math.PI * 2); g.fill();
  }
  if (opts.banned) {
    g.fillStyle = 'rgba(20,20,24,.55)'; g.beginPath(); g.arc(0, 0, r, 0, Math.PI * 2); g.fill();
    g.strokeStyle = '#e8414e'; g.lineWidth = r * .16; g.beginPath(); g.moveTo(-r * .7, -r * .7); g.lineTo(r * .7, r * .7); g.stroke();
  }
  g.restore();
}

export class LolView {
  constructor(canvas, opts = {}) {
    this.cv = canvas; this.ctx = canvas.getContext('2d');
    this.lang = opts.lang === 'ca' ? 'ca' : 'en';
    this.T = TEXT[this.lang];
    this.source = opts.source;
    this.onUpdate = opts.onUpdate || (() => {});
    this.onEvent = opts.onEvent || (() => {});
    this.match = null; this.state = null; this.queue = []; this.idx = 0; this.speed = 1; this.paused = false;
    this.cur = null; this.prog = 0; this.lastN = 0; this.stopped = false; this.last = performance.now();
    this.flashes = []; this.particles = []; this.banner = null; this.mapCache = null; this.now = 0;
    this.resize = this.resize.bind(this);
    if (typeof window !== 'undefined') window.addEventListener('resize', this.resize);
    this.resize();
    this.loop = this.loop.bind(this); requestAnimationFrame(this.loop);
    this.poll();
  }

  destroy() { this.stopped = true; if (typeof window !== 'undefined') window.removeEventListener('resize', this.resize); clearTimeout(this.pollTimer); }
  setSpeed(x) { this.speed = Math.max(.1, +x || 1); }
  togglePause() { this.paused = !this.paused; return this.paused; }
  skipToEnd() {
    if (!this.state) return;
    if (this.cur) this.cur = null;
    while (this.idx < this.queue.length) { const ev = this.queue[this.idx++]; applyEvent(this.state, ev); this.onEvent(ev); }
    this.snapPositions(); this.banner = null; this.flashes = [];
    if (this.state.end) this.state.mode = 'victory';
  }
  isFinished() {
    const m = this.match;
    if (!m || !['final', 'stopped', 'failed'].includes(m.status)) return false;
    if (this.cur || this.idx < this.queue.length) return false;
    return m.status !== 'final' || !!(this.state && this.state.mode === 'victory');
  }

  async poll() {
    if (this.stopped) return;
    try {
      const m = await this.source.load(this.lastN);
      if (m) {
        if (!this.match) this.match = { ...m }; else Object.assign(this.match, { ...m, meta: m.meta || this.match.meta });
        if (!this.state && this.match.meta) { this.state = createState(this.match.meta, this.match.names); this.state.lang = this.lang; }
        for (const e of m.events || []) if (e.n > this.lastN) { this.queue.push(e); this.lastN = e.n; }
        this.onUpdate(this.match, this.queue[this.idx - 1] || null);
      }
    } catch (e) { /* torna-ho a provar */ }
    const done = this.match && ['final', 'stopped', 'failed'].includes(this.match.status);
    if (!done) this.pollTimer = setTimeout(() => this.poll(), 3000);
  }

  resize() {
    const r = this.cv.getBoundingClientRect ? this.cv.getBoundingClientRect() : { width: this.cv.width, height: this.cv.height };
    const d = (typeof window !== 'undefined' && window.devicePixelRatio) || 1;
    this.cv.width = Math.max(320, Math.round(r.width * d)); this.cv.height = Math.max(240, Math.round(r.height * d));
    this.W = this.cv.width; this.H = this.cv.height; this.dpr = d;
    this.mapCache = null;
    this.computeLayout();
  }

  computeLayout() {
    const W = this.W, H = this.H, d = this.dpr;
    this.wide = W / H >= 1.2;
    const top = 64 * d;
    if (this.wide) {
      const size = Math.min(H - top - 16 * d, W * .6);
      this.mapR = { x: 16 * d, y: top + 8 * d, s: size };
      this.side = { x: this.mapR.x + size + 16 * d, y: top + 8 * d, w: W - size - 48 * d, h: size };
    } else {
      const size = Math.min(W - 16 * d, H * .58);
      this.mapR = { x: (W - size) / 2, y: top + 6 * d, s: size };
      this.side = { x: 8 * d, y: this.mapR.y + size + 8 * d, w: W - 16 * d, h: H - (this.mapR.y + size + 16 * d) };
    }
  }

  mx(x) { return this.mapR.x + x / 100 * this.mapR.s; }
  my(y) { return this.mapR.y + y / 100 * this.mapR.s; }

  loop(now) {
    if (this.stopped) return;
    const dt = Math.min(.25, (now - this.last) / 1000); this.last = now; this.now = now / 1000;
    if (!this.paused) this.advance(dt * this.speed, dt);
    try { this.draw(); } catch (e) { /* un frame dolent no atura el visor */ }
    requestAnimationFrame(this.loop);
  }

  advance(dt, realDt) {
    if (!this.state) return;
    if (!this.cur && this.idx < this.queue.length && this.state.mode !== 'victory') {
      const ev = this.queue[this.idx++];
      const fx = applyEvent(this.state, ev);
      this.cur = { ev, fx }; this.prog = 0;
      if (fx.banner) this.banner = { ...fx.banner, t0: this.now };
      for (const f of fx.flashes) this.addFlash(f);
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
    // Moviment suau cap a les posicions objectiu.
    const k = 1 - Math.exp(-dt * 3.2);
    for (let t = 0; t < 2; t++) for (let r = 0; r < 5; r++) {
      const p = this.state.pos[t][r], q = this.state.target[t][r];
      if (!p || !q) continue;
      if (q.alive === false && p.alive !== false) { p.x = q.x; p.y = q.y; }
      p.x = lerp(p.x, q.x, k); p.y = lerp(p.y, q.y, k); p.alive = q.alive;
    }
    this.flashes = this.flashes.filter(f => (f.age += dt) < f.life);
    this.particles = this.particles.filter(q => (q.life -= dt) > 0);
    for (const q of this.particles) { q.x += q.vx * dt; q.y += q.vy * dt; q.vx *= .96; q.vy *= .96; }
  }

  snapPositions() {
    for (let t = 0; t < 2; t++) for (let r = 0; r < 5; r++) { const q = this.state.target[t][r]; this.state.pos[t][r] = { x: q.x, y: q.y, alive: q.alive }; }
  }

  addFlash(f) {
    if (!Number.isFinite(f.x) || !Number.isFinite(f.y)) return;
    this.flashes.push({ ...f, age: 0, life: f.big ? 1.2 : .8 });
    const n = f.debris ? 26 : f.big ? 18 : 8;
    for (let i = 0; i < n; i++) {
      const a = Math.random() * Math.PI * 2, v = (f.big ? 9 : 5) * (.4 + Math.random());
      this.particles.push({ x: f.x, y: f.y, vx: Math.cos(a) * v, vy: Math.sin(a) * v, life: .5 + Math.random() * .7, c: f.debris && Math.random() < .5 ? '#8a7a66' : f.color });
    }
  }

  // ---- dibuix principal ----

  draw() {
    const g = this.ctx, W = this.W, H = this.H;
    const bg = g.createLinearGradient(0, 0, 0, H);
    bg.addColorStop(0, '#0a0f1a'); bg.addColorStop(1, '#05070c');
    g.fillStyle = bg; g.fillRect(0, 0, W, H);
    if (!this.state) { this.centerText(this.T.pregame); return; }
    if (this.state.mode === 'draft') this.drawDraft();
    else {
      this.drawMap();
      this.drawSidePanel();
      this.drawScoreboard();
      this.drawBanner();
      if (this.state.mode === 'victory') this.drawVictory();
    }
    if (!this.cur && this.idx >= this.queue.length && this.match && this.match.status !== 'final' && this.state.mode !== 'victory') {
      const d = this.dpr; g.fillStyle = 'rgba(255,255,255,.55)'; g.font = `600 ${12 * d}px ${FONT}`; g.textAlign = 'right'; g.textBaseline = 'bottom';
      g.fillText(this.match.status === 'pregame' ? this.T.pregame : this.T.waiting, W - 12 * d, H - 8 * d);
    }
  }

  centerText(t) { const g = this.ctx; g.fillStyle = 'rgba(255,255,255,.8)'; g.font = `600 ${16 * this.dpr}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText(t, this.W / 2, this.H / 2); }

  names() {
    const m = this.match || {}, meta = this.state.meta;
    const n = m.names || this.state.names || [];
    return [0, 1].map(i => n[i] || (meta.teams && meta.teams[i] && meta.teams[i].name) || `Team ${i + 1}`);
  }

  // ---- draft ----

  drawDraft() {
    const g = this.ctx, W = this.W, H = this.H, d = this.dpr, st = this.state, T = this.T;
    const blueTeam = [0, 1].find(t => mapSide(st.meta, t) === 0) ?? 0;
    const order = [blueTeam, 1 - blueTeam];
    const names = this.names();
    // Fons amb llum de cada costat.
    for (let k = 0; k < 2; k++) {
      const c = teamColor(st.meta, order[k]);
      const rg = g.createRadialGradient(k ? W : 0, H * .5, 0, k ? W : 0, H * .5, W * .6);
      rg.addColorStop(0, rgba(c, .28)); rg.addColorStop(1, rgba(c, 0));
      g.fillStyle = rg; g.fillRect(0, 0, W, H);
    }
    const narrow = W / H < 1.2;
    const fs = Math.min(W / 50, H / 26);
    // Capçalera.
    g.textBaseline = 'middle';
    g.fillStyle = '#e8d9a8'; g.font = `800 ${fs * 1.1}px ${FONT}`; g.textAlign = 'center';
    g.fillText(T.draft + (st.draft.step ? ' · ' + st.draft.step : ''), W / 2, fs * 1.4);
    const colW = narrow ? W * .46 : W * .34;
    const slotH = Math.min((H - fs * 9.5) / 5.5, colW * .3);
    const yTop = fs * 7.2;
    for (let k = 0; k < 2; k++) {
      const team = order[k], c = teamColor(st.meta, team);
      const x0 = k === 0 ? fs : W - fs - colW;
      // Nom de l'equip (nom del jugador humà).
      g.fillStyle = c; g.font = `900 ${fs * 1.45}px ${FONT}`; g.textAlign = k === 0 ? 'left' : 'right';
      g.fillText(this.fit(names[team], colW, fs * 1.45, 900), k === 0 ? x0 : x0 + colW, fs * 3.1);
      g.fillStyle = 'rgba(255,255,255,.5)'; g.font = `700 ${fs * .7}px ${FONT}`;
      g.fillText(T.side[k], k === 0 ? x0 : x0 + colW, fs * 4.25);
      // Bans.
      const bans = st.draft.bans[team], br = Math.min(fs * .95, colW / 12);
      for (let i = 0; i < 5; i++) {
        const bx = k === 0 ? x0 + br + i * br * 2.4 : x0 + colW - br - i * br * 2.4, by = fs * 4.9 + br * 1.15;
        if (bans[i]) drawPortrait(g, bx, by, br, champInfo(st.meta, bans[i]), '#555', { banned: true, glow: this.isActive(team, 'ban', i) });
        else { g.strokeStyle = 'rgba(255,255,255,.18)'; g.lineWidth = 1.5 * d; g.beginPath(); g.arc(bx, by, br, 0, Math.PI * 2); g.stroke(); }
      }
      // Picks.
      for (let i = 0; i < 5; i++) {
        const y = yTop + i * slotH * 1.08 + slotH * .3, p = st.draft.picks[team][i];
        const active = this.isActive(team, 'pick', i);
        const grd = g.createLinearGradient(x0, 0, x0 + colW, 0);
        grd.addColorStop(k === 0 ? 0 : 1, rgba(c, active ? .55 : .25)); grd.addColorStop(k === 0 ? 1 : 0, 'rgba(255,255,255,.03)');
        g.fillStyle = grd; g.beginPath(); g.roundRect(x0, y, colW, slotH, 8 * d); g.fill();
        if (active) { g.strokeStyle = c; g.lineWidth = 2 * d; g.stroke(); }
        const pr = slotH * .4, px = k === 0 ? x0 + slotH * .55 : x0 + colW - slotH * .55;
        const pl = (st.meta.teams && st.meta.teams[team] && st.meta.teams[team].players) || [];
        if (p) {
          const appear = active && this.cur ? easeOut(clamp(this.prog * 2.5, 0, 1)) : 1;
          drawPortrait(g, px, y + slotH / 2, pr * (.6 + .4 * appear), champInfo(st.meta, p.champ), c, { glow: active });
          g.globalAlpha = appear;
          g.fillStyle = '#fff'; g.font = `800 ${slotH * .3}px ${FONT}`; g.textAlign = k === 0 ? 'left' : 'right';
          const tx = k === 0 ? px + pr * 1.4 : px - pr * 1.4;
          g.fillText(p.champ, tx, y + slotH * .38);
          const roleIdx = ROLES.indexOf(p.role);
          const handle = roleIdx >= 0 && pl[roleIdx] ? pl[roleIdx].handle : '';
          g.fillStyle = 'rgba(255,255,255,.6)'; g.font = `600 ${slotH * .2}px ${FONT}`;
          g.fillText(`${T.roles[p.role] || p.role || ''}${handle ? ' · ' + handle : ''} · ${p.cls || ''}`, tx, y + slotH * .7);
          g.globalAlpha = 1;
        } else {
          g.strokeStyle = 'rgba(255,255,255,.2)'; g.lineWidth = 1.5 * d; g.setLineDash([4 * d, 4 * d]);
          g.beginPath(); g.arc(px, y + slotH / 2, pr, 0, Math.PI * 2); g.stroke(); g.setLineDash([]);
        }
      }
    }
    // Centre: rellotge del torn i acció actual.
    const cx = W / 2, cy = narrow ? H - fs * 3.2 : H * .5, cr = Math.min(fs * 2.6, W * .08);
    const left = this.cur ? 1 - this.prog : 0;
    g.strokeStyle = 'rgba(255,255,255,.12)'; g.lineWidth = 5 * d; g.beginPath(); g.arc(cx, cy, cr, 0, Math.PI * 2); g.stroke();
    const act = st.draft.active, ac = act ? teamColor(st.meta, act.team) : '#e8d9a8';
    g.strokeStyle = ac; g.beginPath(); g.arc(cx, cy, cr, -Math.PI / 2, -Math.PI / 2 + Math.PI * 2 * left); g.stroke();
    g.fillStyle = '#fff'; g.font = `800 ${cr * .7}px ${FONT}`; g.textAlign = 'center';
    g.fillText(String(Math.ceil(27 * left) + (this.cur ? 3 : 0)), cx, cy - cr * .08);
    if (act) { g.fillStyle = ac; g.font = `800 ${cr * .32}px ${FONT}`; g.fillText(act.kind === 'ban' ? T.ban : T.pick, cx, cy + cr * .5); }
    if (!narrow) { g.fillStyle = 'rgba(232,217,168,.85)'; g.font = `900 ${fs * 1.6}px ${FONT}`; g.fillText('VS', cx, cy - cr * 1.8); }
    if (this.cur && this.cur.ev.text) { g.fillStyle = 'rgba(255,255,255,.8)'; g.font = `600 ${fs * .75}px ${FONT}`; g.fillText(this.cur.ev.text, cx, narrow ? cy - cr * 1.5 : cy + cr * 1.6); }
  }

  isActive(team, kind, slot) { const a = this.state.draft.active; return !!(this.cur && a && a.team === team && a.kind === kind && a.slot === slot); }

  fit(text, w, size, weight) { const g = this.ctx; g.font = `${weight} ${size}px ${FONT}`; let s = String(text || ''); while (s.length > 3 && g.measureText(s).width > w) s = s.slice(0, -2) + '…'; return s; }

  // ---- mapa ----

  buildMapCache() {
    if (typeof document === 'undefined') return null;
    const s = Math.round(this.mapR.s);
    const c = document.createElement('canvas'); c.width = s; c.height = s;
    const g = c.getContext('2d'), m = this.state.map, P = v => v / 100 * s;
    const R = rng(42);
    // Terra i jungla.
    const bg = g.createLinearGradient(0, s, s, 0);
    bg.addColorStop(0, '#13321f'); bg.addColorStop(.5, '#0f2a1c'); bg.addColorStop(1, '#13321f');
    g.fillStyle = bg; g.beginPath(); g.roundRect(0, 0, s, s, s * .03); g.fill();
    for (let i = 0; i < 260; i++) { g.fillStyle = `rgba(${R() < .5 ? '8,30,16' : '30,70,40'},${.25 + R() * .3})`; g.beginPath(); g.arc(R() * s, R() * s, s * (.01 + R() * .035), 0, Math.PI * 2); g.fill(); }
    // Murs de la jungla (taques fosques simètriques).
    const walls = [[24, 40, 7], [40, 24, 7], [28, 52, 6], [52, 28, 6], [60, 76, 7], [76, 60, 7], [48, 72, 6], [72, 48, 6], [36, 44, 4], [44, 36, 4], [56, 64, 4], [64, 56, 4], [20, 62, 5], [62, 20, 5], [38, 80, 5], [80, 38, 5]];
    for (const [x, y, r] of walls) { g.fillStyle = 'rgba(4,14,8,.75)'; g.beginPath(); g.ellipse(P(x), P(y), P(r), P(r * .7), (x + y) * .1, 0, Math.PI * 2); g.fill(); }
    // Riu.
    const poly = (pts, w, col) => { g.strokeStyle = col; g.lineWidth = w; g.lineCap = 'round'; g.lineJoin = 'round'; g.beginPath(); pts.forEach((p, i) => i ? g.lineTo(P(p[0]), P(p[1])) : g.moveTo(P(p[0]), P(p[1]))); g.stroke(); };
    poly(m.river, s * .085, '#163b4d'); poly(m.river, s * .055, '#1d5468'); poly(m.river, s * .02, 'rgba(120,200,230,.25)');
    // Carrils.
    for (const k of ['top', 'mid', 'bot']) { poly(m.lanes[k], s * .05, '#4a432f'); poly(m.lanes[k], s * .034, '#7a6b49'); poly(m.lanes[k], s * .004, 'rgba(255,240,200,.15)'); }
    // Fosses.
    const pit = (p, col) => { const gr = g.createRadialGradient(P(p.x), P(p.y), 0, P(p.x), P(p.y), P(6)); gr.addColorStop(0, rgba(col, .55)); gr.addColorStop(1, rgba(col, 0)); g.fillStyle = gr; g.beginPath(); g.arc(P(p.x), P(p.y), P(6), 0, Math.PI * 2); g.fill(); g.strokeStyle = rgba(col, .8); g.lineWidth = s * .004; g.beginPath(); g.arc(P(p.x), P(p.y), P(3.6), 0, Math.PI * 2); g.stroke(); };
    pit(m.dragon, '#ff8a3d'); pit(m.baron, '#9b59ff');
    // Bases.
    for (let side = 0; side < 2; side++) {
      const n = (m.nexus || [])[side] || { x: side ? 88 : 12, y: side ? 12 : 88 }, col = SIDE_COLORS[side][0];
      const gr = g.createRadialGradient(P(n.x), P(n.y), 0, P(n.x), P(n.y), P(17));
      gr.addColorStop(0, rgba(col, .45)); gr.addColorStop(.7, rgba(col, .15)); gr.addColorStop(1, rgba(col, 0));
      g.fillStyle = gr; g.beginPath(); g.arc(P(n.x), P(n.y), P(17), 0, Math.PI * 2); g.fill();
      g.fillStyle = 'rgba(30,34,44,.85)'; g.beginPath(); g.arc(P(FOUNTAIN[side][0]), P(FOUNTAIN[side][1]), P(3.5), 0, Math.PI * 2); g.fill();
    }
    g.strokeStyle = 'rgba(255,255,255,.08)'; g.lineWidth = 2; g.beginPath(); g.roundRect(1, 1, s - 2, s - 2, s * .03); g.stroke();
    return c;
  }

  drawMap() {
    const g = this.ctx, st = this.state, R = this.mapR, d = this.dpr;
    if (!this.mapCache) this.mapCache = this.buildMapCache();
    if (this.mapCache) g.drawImage(this.mapCache, R.x, R.y, R.s, R.s);
    const u = R.s / 100;
    // Estructures.
    for (const tw of st.map.towers || []) {
      const alive = st.towers[`${tw.side}-${tw.lane}-${tw.tier}`] !== false, x = this.mx(tw.x), y = this.my(tw.y), col = SIDE_COLORS[tw.side][0];
      if (alive) {
        g.fillStyle = 'rgba(0,0,0,.4)'; g.beginPath(); g.ellipse(x, y + u * .9, u * 1.6, u * .7, 0, 0, Math.PI * 2); g.fill();
        g.fillStyle = shade(col, -.35); g.beginPath(); g.roundRect(x - u * 1.1, y - u * 1.4, u * 2.2, u * 2.4, u * .4); g.fill();
        g.fillStyle = col; g.beginPath(); g.moveTo(x, y - u * 2.4); g.lineTo(x + u * 1.2, y - u * 1.2); g.lineTo(x - u * 1.2, y - u * 1.2); g.closePath(); g.fill();
        g.fillStyle = 'rgba(255,255,255,.85)'; g.beginPath(); g.arc(x, y - u * .3, u * .4, 0, Math.PI * 2); g.fill();
      } else {
        g.fillStyle = 'rgba(70,64,56,.85)'; for (let i = 0; i < 4; i++) { g.beginPath(); g.arc(x + (i - 1.5) * u * .6, y + (i % 2) * u * .4, u * .45, 0, Math.PI * 2); g.fill(); }
      }
    }
    for (const ih of st.map.inhibitors || []) {
      const up = st.inhibs[`${ih.side}-${ih.lane}`] !== false, x = this.mx(ih.x), y = this.my(ih.y), col = SIDE_COLORS[ih.side][0];
      g.save(); g.translate(x, y); g.rotate(Math.PI / 4);
      g.fillStyle = up ? col : 'rgba(60,60,64,.9)'; g.fillRect(-u * 1.1, -u * 1.1, u * 2.2, u * 2.2);
      if (up) { g.fillStyle = 'rgba(255,255,255,.7)'; g.fillRect(-u * .4, -u * .4, u * .8, u * .8); }
      g.restore();
    }
    for (let side = 0; side < 2; side++) {
      const n = (st.map.nexus || [])[side]; if (!n) continue;
      const x = this.mx(n.x), y = this.my(n.y), col = SIDE_COLORS[side][0], alive = st.nexus[side];
      const pulse = .5 + .5 * Math.sin(this.now * 2 + side);
      if (alive) { g.shadowColor = col; g.shadowBlur = u * 3 * pulse; }
      g.fillStyle = alive ? col : '#444'; g.beginPath();
      for (let i = 0; i < 6; i++) { const a = i * Math.PI / 3; g.lineTo(x + Math.cos(a) * u * 2.6, y + Math.sin(a) * u * 2.6); }
      g.closePath(); g.fill(); g.shadowBlur = 0;
      if (alive) { g.fillStyle = 'rgba(255,255,255,.85)'; g.beginPath(); g.arc(x, y, u * 1, 0, Math.PI * 2); g.fill(); }
    }
    // Monstres: drac i baró (només si no hi ha ànima/si toca).
    this.drawMonster(st.map.dragon, '#ff7a33', 'D');
    if (st.t >= 1200) this.drawMonster(st.map.baron, '#a35bff', 'B');
    // Efectes de baralla.
    for (const f of this.flashes) {
      const k = f.age / f.life, x = this.mx(f.x), y = this.my(f.y);
      g.strokeStyle = rgba(f.color.startsWith('#') ? f.color : '#ffffff', 1 - k); g.lineWidth = (f.big ? 4 : 2.5) * d;
      g.beginPath(); g.arc(x, y, u * (f.big ? 10 : 5) * easeOut(k), 0, Math.PI * 2); g.stroke();
      if (k < .25) { g.fillStyle = `rgba(255,255,255,${(.25 - k) * 3})`; g.beginPath(); g.arc(x, y, u * (f.big ? 4 : 2.2), 0, Math.PI * 2); g.fill(); }
    }
    for (const q of this.particles) { g.globalAlpha = clamp(q.life * 1.5, 0, 1); g.fillStyle = q.c; g.fillRect(this.mx(q.x) - u * .25, this.my(q.y) - u * .25, u * .5, u * .5); }
    g.globalAlpha = 1;
    // Campions.
    const items = [];
    for (let t = 0; t < 2; t++) for (let r = 0; r < 5; r++) items.push({ t, r, p: st.pos[t][r] });
    items.sort((a, b) => (a.p.alive === false) - (b.p.alive === false));
    const cr = Math.max(7 * d, u * 2.1);
    for (const it of items) {
      const line = st.line[it.t][it.r] || {}, info = champInfo(st.meta, line.champ);
      const col = teamColor(st.meta, it.t), x = this.mx(it.p.x), y = this.my(it.p.y);
      const dead = it.p.alive === false;
      g.globalAlpha = dead ? .45 : 1;
      if (!dead && st.buff[it.t].baron > st.t) { g.strokeStyle = 'rgba(170,100,255,.8)'; g.lineWidth = 2 * d; g.beginPath(); g.arc(x, y, cr * 1.35, 0, Math.PI * 2); g.stroke(); }
      if (!dead && st.buff[it.t].elder > st.t) { g.strokeStyle = 'rgba(245,225,122,.9)'; g.lineWidth = 2 * d; g.beginPath(); g.arc(x, y, cr * 1.55, 0, Math.PI * 2); g.stroke(); }
      drawPortrait(g, x, y, cr, info, col, { dead });
      g.globalAlpha = 1;
    }
    // Rètol de l'esdeveniment a sota del mapa (o a dins si és estret).
    if (this.cur && this.cur.ev.text && this.state.mode !== 'victory') {
      const fs = Math.max(11 * d, R.s * .024);
      g.font = `600 ${fs}px ${FONT}`;
      const tw = Math.min(R.s - 20 * d, g.measureText(this.cur.ev.text).width + 24 * d);
      const bx = R.x + (R.s - tw) / 2, by = R.y + R.s - fs * 2.4;
      g.fillStyle = 'rgba(6,8,14,.78)'; g.beginPath(); g.roundRect(bx, by, tw, fs * 1.8, fs * .5); g.fill();
      g.fillStyle = this.cur.ev.hl ? '#ffd76a' : '#fff'; g.textAlign = 'center'; g.textBaseline = 'middle';
      g.fillText(this.fit(this.cur.ev.text, tw - 16 * d, fs, 600), R.x + R.s / 2, by + fs * .9);
    }
  }

  drawMonster(p, col, letter) {
    if (!p) return;
    const g = this.ctx, u = this.mapR.s / 100, x = this.mx(p.x), y = this.my(p.y);
    const b = 1 + .06 * Math.sin(this.now * 3);
    g.fillStyle = shade(col, -.3); g.beginPath(); g.arc(x, y, u * 1.8 * b, 0, Math.PI * 2); g.fill();
    g.strokeStyle = col; g.lineWidth = u * .4; g.stroke();
    g.fillStyle = '#fff'; g.font = `800 ${u * 1.9}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillText(letter, x, y + u * .1);
  }

  // ---- marcador ----

  drawScoreboard() {
    const g = this.ctx, W = this.W, d = this.dpr, st = this.state, T = this.T;
    const h = 56 * d, y = 4 * d;
    const names = this.names();
    const blueTeam = [0, 1].find(t => mapSide(st.meta, t) === 0) ?? 0;
    const order = [blueTeam, 1 - blueTeam];
    g.fillStyle = 'rgba(8,11,20,.92)'; g.beginPath(); g.roundRect(6 * d, y, W - 12 * d, h, 10 * d); g.fill();
    const mid = W / 2, clockW = Math.min(110 * d, W * .16);
    // Rellotge.
    const k = this.cur ? clamp(this.prog * 1.4, 0, 1) : 1;
    const clock = lerp(st.clockFrom, st.clockTo, k);
    g.fillStyle = '#fff'; g.textAlign = 'center'; g.textBaseline = 'middle';
    g.font = `800 ${18 * d}px ${FONT}`; g.fillText(fmtClock(clock), mid, y + h * .4);
    g.fillStyle = this.match && this.match.status === 'live' ? '#ff5a5a' : 'rgba(255,255,255,.5)'; g.font = `700 ${9 * d}px ${FONT}`;
    g.fillText(this.match && this.match.status === 'final' ? 'FINAL' : T.live, mid, y + h * .78);
    const half = (W - 12 * d - clockW) / 2;
    for (let s = 0; s < 2; s++) {
      const team = order[s], col = teamColor(st.meta, team);
      const x0 = s === 0 ? 6 * d : mid + clockW / 2, x1 = x0 + half;
      const inner = s === 0 ? x1 - 10 * d : x0 + 10 * d, outer = s === 0 ? x0 + 12 * d : x1 - 12 * d, dir = s === 0 ? -1 : 1;
      g.fillStyle = col; g.fillRect(s === 0 ? x0 : x1 - 5 * d, y + 8 * d, 5 * d, h - 16 * d);
      // Morts (gran).
      g.fillStyle = '#fff'; g.font = `900 ${26 * d}px ${FONT}`; g.textAlign = s === 0 ? 'right' : 'left';
      g.fillText(String(st.kills[team] || 0), inner, y + h / 2);
      const kw = 40 * d;
      // Nom del jugador humà.
      const nameMax = half - kw - (this.wide ? 190 * d : 60 * d);
      g.fillStyle = col; g.font = `800 ${(this.wide ? 17 : 13) * d}px ${FONT}`; g.textAlign = s === 0 ? 'left' : 'right';
      g.fillText(this.fit(names[team], Math.max(40 * d, nameMax), (this.wide ? 17 : 13) * d, 800), outer + (s === 0 ? 4 * d : -4 * d), y + h * (this.wide ? .5 : .32));
      // Torres, dracs, or.
      const statsX = this.wide ? inner + dir * (kw + 4 * d) : outer + (s === 0 ? 4 * d : -4 * d);
      const sy = this.wide ? y + h / 2 : y + h * .72;
      g.font = `700 ${11 * d}px ${FONT}`; g.fillStyle = 'rgba(255,255,255,.85)'; g.textAlign = s === 0 ? 'right' : 'left';
      if (!this.wide) g.textAlign = s === 0 ? 'left' : 'right';
      const parts = [`♜ ${st.towersK[team] || 0}`, `◆ ${(st.gold[team] / 1000).toFixed(1)}k`];
      if (st.barons[team]) parts.push(`B ${st.barons[team]}`);
      g.fillText(parts.join('   '), statsX, sy - (this.wide ? 8 * d : 0));
      // Dracs com a rombes de colors.
      if (this.wide) {
        const dr = st.dragons[team];
        for (let i = 0; i < dr.length; i++) {
          const dx = statsX + dir * (i * 12 * d + 6 * d), dy = sy + 10 * d;
          g.fillStyle = DRAGON_COLORS[dr[i]] || '#fff'; g.beginPath(); g.moveTo(dx, dy - 5 * d); g.lineTo(dx + 4 * d, dy); g.lineTo(dx, dy + 5 * d); g.lineTo(dx - 4 * d, dy); g.closePath(); g.fill();
        }
      }
    }
  }

  // ---- panell lateral: or i registre de morts ----

  drawSidePanel() {
    const g = this.ctx, S = this.side, d = this.dpr, st = this.state;
    if (S.w < 60 * d || S.h < 60 * d) return;
    const graphH = this.wide ? Math.min(S.h * .32, 180 * d) : Math.min(S.h * .45, 120 * d);
    this.drawGoldGraph(S.x, S.y + S.h - graphH, S.w, graphH);
    // Pla actual de cada equip (objectiu i postura de la darrera decisió).
    let top = S.y;
    if (st.plan[0] || st.plan[1]) {
      const names = this.names(), ph = Math.min(26 * d, S.h * .08);
      for (let t = 0; t < 2; t++) {
        const p = st.plan[t] || {}, c = teamColor(st.meta, t), y = top + t * (ph + 4 * d);
        g.fillStyle = rgba(c, .18); g.beginPath(); g.roundRect(S.x, y, S.w, ph, 6 * d); g.fill();
        g.fillStyle = c; g.fillRect(S.x, y, 4 * d, ph);
        g.font = `800 ${ph * .45}px ${FONT}`; g.textAlign = 'left'; g.textBaseline = 'middle';
        g.fillText(this.fit(names[t], S.w * .4, ph * .45, 800), S.x + 10 * d, y + ph / 2);
        g.fillStyle = '#fff'; g.font = `600 ${ph * .42}px ${FONT}`; g.textAlign = 'right';
        g.fillText(`${(p.objective || '–').toUpperCase()} · ${(p.stance || '–').toUpperCase()}`, S.x + S.w - 10 * d, y + ph / 2);
      }
      top += 2 * (ph + 4 * d) + 6 * d;
    }
    // Registre de morts.
    const feedH = S.y + S.h - graphH - 10 * d - top;
    const rowH = Math.min(30 * d, feedH / 6);
    const r = rowH * .38;
    st.feed.slice(0, Math.floor(feedH / (rowH + 4 * d))).forEach((f, i) => {
      const y = top + i * (rowH + 4 * d);
      const kc = teamColor(st.meta, f.killer.side === 1 ? 1 : 0), vc = teamColor(st.meta, f.victim.side === 1 ? 1 : 0);
      g.fillStyle = 'rgba(10,14,24,.85)'; g.beginPath(); g.roundRect(S.x, y, S.w, rowH, 6 * d); g.fill();
      g.fillStyle = kc; g.fillRect(S.x, y, 4 * d, rowH);
      drawPortrait(g, S.x + 10 * d + r, y + rowH / 2, r, champInfo(st.meta, f.killer.champ), kc);
      g.fillStyle = '#fff'; g.font = `600 ${rowH * .36}px ${FONT}`; g.textAlign = 'left'; g.textBaseline = 'middle';
      const label = `${f.killer.champ || ''}  ⚔  ${f.victim.champ || ''}${f.multi >= 2 ? '  ×' + f.multi : ''}${f.ace ? '  ACE' : ''}`;
      g.fillText(this.fit(label, S.w - r * 5 - 60 * d, rowH * .36, 600), S.x + 16 * d + r * 2, y + rowH / 2);
      drawPortrait(g, S.x + S.w - 10 * d - r, y + rowH / 2, r, champInfo(st.meta, f.victim.champ), vc, { dead: true });
      g.fillStyle = 'rgba(255,255,255,.45)'; g.textAlign = 'right'; g.font = `600 ${rowH * .3}px ${FONT}`;
      g.fillText(fmtClock(f.t), S.x + S.w - 16 * d - r * 2, y + rowH / 2);
    });
  }

  drawGoldGraph(x, y, w, h, title = true) {
    const g = this.ctx, d = this.dpr, st = this.state, T = this.T;
    g.fillStyle = 'rgba(10,14,24,.85)'; g.beginPath(); g.roundRect(x, y, w, h, 8 * d); g.fill();
    const pts = st.goldPts.length ? st.goldPts : [[0, 0]];
    const tMax = Math.max(900, pts[pts.length - 1][0]);
    const vMax = Math.max(3000, ...pts.map(p => Math.abs(p[1])));
    const px = t => x + 10 * d + t / tMax * (w - 20 * d), py = v => y + h / 2 - v / vMax * (h / 2 - 16 * d);
    g.strokeStyle = 'rgba(255,255,255,.15)'; g.lineWidth = 1; g.beginPath(); g.moveTo(x + 8 * d, py(0)); g.lineTo(x + w - 8 * d, py(0)); g.stroke();
    // Àrees de cada equip (equip 0 a dalt).
    for (let team = 0; team < 2; team++) {
      g.save(); g.beginPath();
      if (team === 0) g.rect(x, y, w, py(0) - y); else g.rect(x, py(0), w, y + h - py(0));
      g.clip();
      g.beginPath(); g.moveTo(px(pts[0][0]), py(0));
      pts.forEach(p => g.lineTo(px(p[0]), py(p[1])));
      g.lineTo(px(pts[pts.length - 1][0]), py(0)); g.closePath();
      g.fillStyle = rgba(teamColor(st.meta, team), .35); g.fill();
      g.strokeStyle = teamColor(st.meta, team); g.lineWidth = 2 * d; g.beginPath(); pts.forEach((p, i) => i ? g.lineTo(px(p[0]), py(p[1])) : g.moveTo(px(p[0]), py(p[1]))); g.stroke();
      g.restore();
    }
    if (title) {
      const diff = pts[pts.length - 1][1];
      g.fillStyle = 'rgba(255,255,255,.7)'; g.font = `700 ${10 * d}px ${FONT}`; g.textAlign = 'left'; g.textBaseline = 'top';
      g.fillText(T.golddiff, x + 10 * d, y + 6 * d);
      g.textAlign = 'right'; g.fillStyle = teamColor(st.meta, diff >= 0 ? 0 : 1);
      g.fillText(`${diff >= 0 ? '+' : '−'}${(Math.abs(diff) / 1000).toFixed(1)}k`, x + w - 10 * d, y + 6 * d);
    }
  }

  // ---- rètols ----

  drawBanner() {
    if (!this.banner) return;
    const g = this.ctx, d = this.dpr, b = this.banner;
    const age = this.now - b.t0, life = b.big ? 2.8 : 1.8;
    if (age > life) { this.banner = null; return; }
    const k = easeOut(clamp(age * 4, 0, 1));
    const R = this.mapR;
    const fs = (b.big ? Math.min(46 * d, R.s * .085) : Math.min(28 * d, R.s * .055)) * (.7 + .3 * k);
    const cx = R.x + R.s / 2, cy = R.y + R.s * (b.big ? .42 : .2);
    g.save(); g.globalAlpha = clamp(life - age, 0, 1);
    g.font = `900 italic ${fs}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle';
    const tw = Math.min(R.s * .98, g.measureText(b.text).width + fs * 1.6);
    const slant = fs * .4;
    g.fillStyle = 'rgba(6,8,14,.75)';
    g.beginPath(); g.moveTo(cx - tw / 2 + slant, cy - fs * .75); g.lineTo(cx + tw / 2 + slant, cy - fs * .75); g.lineTo(cx + tw / 2 - slant, cy + fs * .75); g.lineTo(cx - tw / 2 - slant, cy + fs * .75); g.closePath(); g.fill();
    g.strokeStyle = b.color; g.lineWidth = 3 * d; g.stroke();
    g.shadowColor = b.color; g.shadowBlur = fs * .5; g.fillStyle = '#fff';
    g.fillText(b.text, cx, cy + fs * .04);
    g.restore();
  }

  // ---- victòria ----

  drawVictory() {
    const g = this.ctx, W = this.W, H = this.H, d = this.dpr, st = this.state, T = this.T, e = st.end || {};
    const w = e.winner === 1 ? 1 : 0, col = teamColor(st.meta, w), names = this.names();
    g.fillStyle = 'rgba(4,6,12,.86)'; g.fillRect(0, 0, W, H);
    const rg = g.createRadialGradient(W / 2, H * .2, 0, W / 2, H * .2, W * .6);
    rg.addColorStop(0, rgba(col, .35)); rg.addColorStop(1, rgba(col, 0)); g.fillStyle = rg; g.fillRect(0, 0, W, H);
    const narrow = W / H < 1;
    const fs = narrow ? Math.min(W / 11, H / 16) : Math.min(W / 22, H / 12);
    g.textAlign = 'center'; g.textBaseline = 'middle';
    g.fillStyle = '#f2d98d'; g.font = `900 ${fs * 1.3}px ${FONT}`; g.fillText(T.victory, W / 2, fs * 1.3);
    g.fillStyle = col; g.font = `900 ${fs * .9}px ${FONT}`; g.fillText(this.fit(names[w], W * .9, fs * .9, 900), W / 2, fs * 2.6);
    g.fillStyle = 'rgba(255,255,255,.7)'; g.font = `600 ${fs * .4}px ${FONT}`;
    const sub = `${T.duration} ${fmtClock(e.duration || st.t)} · ${T.kills} ${st.kills[0]}–${st.kills[1]}${e.comeback ? ' · ' + T.comeback : ''}`;
    g.fillText(sub, W / 2, fs * 3.4);
    // MVP.
    const mvp = e.mvp || {};
    const my = fs * 4.9, mr = fs * .75;
    if (mvp.champ) {
      drawPortrait(g, W / 2 - fs * 2.6, my, mr, champInfo(st.meta, mvp.champ), col, { glow: true });
      g.textAlign = 'left'; g.fillStyle = '#f2d98d'; g.font = `800 ${fs * .32}px ${FONT}`; g.fillText(T.mvp, W / 2 - fs * 1.6, my - mr * .55);
      g.fillStyle = '#fff'; g.font = `800 ${fs * .5}px ${FONT}`; g.fillText(`${mvp.handle || ''} · ${mvp.champ}`, W / 2 - fs * 1.6, my);
      g.fillStyle = 'rgba(255,255,255,.75)'; g.font = `700 ${fs * .36}px ${FONT}`; g.fillText(`${mvp.k}/${mvp.d}/${mvp.a}`, W / 2 - fs * 1.6, my + mr * .6);
    }
    // Taula dels jugadors.
    const players = e.players || [[], []];
    const top = my + mr * 1.8;
    const colW = narrow ? W - 24 * d : Math.min(W * .46, 520 * d);
    const rowH = narrow ? Math.min(fs * .62, (H - top) / 16) : Math.min(fs * .62, (H - top - fs * 3.2) / 6);
    for (let t = 0; t < 2; t++) {
      const x0 = narrow ? 12 * d : W / 2 + (t === 0 ? -colW - 8 * d : 8 * d), c = teamColor(st.meta, t);
      const y0 = narrow ? top + t * rowH * 6.4 : top;
      g.fillStyle = c; g.font = `800 ${rowH * .5}px ${FONT}`; g.textAlign = 'left';
      g.fillText(this.fit(names[t], colW, rowH * .5, 800), x0, y0);
      (players[t] || []).forEach((p, i) => {
        const y = y0 + (i + 1) * rowH;
        g.fillStyle = i % 2 ? 'rgba(255,255,255,.03)' : 'rgba(255,255,255,.07)'; g.fillRect(x0, y - rowH * .45, colW, rowH * .9);
        drawPortrait(g, x0 + rowH * .45, y, rowH * .36, champInfo(st.meta, p.champ), c);
        g.fillStyle = '#fff'; g.font = `600 ${rowH * .38}px ${FONT}`; g.textAlign = 'left';
        g.fillText(this.fit(`${p.handle} · ${p.champ}`, colW * .55, rowH * .38, 600), x0 + rowH, y);
        g.textAlign = 'right'; g.fillText(`${p.k}/${p.d}/${p.a}`, x0 + colW * .8, y);
        g.fillStyle = '#f2d98d'; g.fillText(`${((p.gold || 0) / 1000).toFixed(1)}k`, x0 + colW - 6 * d, y);
      });
    }
    const gy = narrow ? top + rowH * 13 : top + rowH * 6.6;
    const gh = Math.min(H - gy - 10 * d, narrow ? H * .22 : 170 * d);
    const gx = narrow ? 12 * d : W / 2 - colW - 8 * d, gw = narrow ? colW : colW * 2 + 16 * d;
    if (gh > 40 * d) this.drawGoldGraph(gx, gy, gw, gh);
  }
}
