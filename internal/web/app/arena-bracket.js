// El quadre d'una competició de l'Arena: la taula de cada grup, l'arbre de les
// eliminatòries (al millor de 3, amb el marcador de la sèrie) i el calendari
// (el que s'ha jugat, el que es juga i el que ve).
//
// Autònom (sense dependències): el fan servir la tele (arena-stream.html), les
// capes per a l'OBS (#overlay=bracket|standings) i, més endavant, la pàgina de
// l'Arena (renderCompetition). Treballa amb el que torna /api/arena/competition
// o /api/arena/stream: {games, status, schedule:[{game, stage, phase, a, b,
// match, matches, series, winner, score, done, pending, from}], current,
// champion, names, tables}.
//
// La part de càlcul (bracketLayout, scheduleList, seedLabel…) no toca el DOM:
// la proven els tests de node. Les funcions *HTML tornen text; render* el posen
// en un element i hi afegeixen l'estil un sol cop.

const TX = {
  en: {
    group: 'Group', winner: 'Winner', bye: 'Bye', live: 'LIVE', past: 'Played', next: 'Up next', now: 'Live now', champion: 'Champion',
    p: 'P', w: 'W', d: 'D', l: 'L', pts: 'Pts', player: 'Player', tbd: 'TBD', bo3: 'Best of 3', game: 'Game',
    rounds: { Quarterfinal: 'Quarterfinals', Semifinal: 'Semifinals', Final: 'Final' }, short: { Quarterfinal: 'QF', Semifinal: 'SF', Final: 'Final' },
    nth: n => n + (n === 1 ? 'st' : n === 2 ? 'nd' : n === 3 ? 'rd' : 'th'), empty: 'Nothing yet',
  },
  ca: {
    group: 'Grup', winner: 'Guanyador', bye: 'Exempt', live: 'EN DIRECTE', past: 'Jugats', next: 'A continuació', now: 'Ara en directe', champion: 'Campió',
    p: 'PJ', w: 'G', d: 'E', l: 'P', pts: 'Pts', player: 'Jugador', tbd: 'Per decidir', bo3: 'Al millor de 3', game: 'Partit',
    rounds: { Quarterfinal: 'Quarts de final', Semifinal: 'Semifinals', Final: 'Final' }, short: { Quarterfinal: 'QF', Semifinal: 'SF', Final: 'Final' },
    nth: n => n + (n === 1 ? 'r' : n === 2 ? 'n' : n === 3 ? 'r' : n === 4 ? 't' : 'è'), empty: 'Encara res',
  },
};
const tx = lang => TX[lang] || TX.en;
export const GAME_TITLES = { nfl: 'NFL', cs2: 'Counter-Strike 2', lol: 'League of Legends' };
const ROUND_ORDER = ['Quarterfinal', 'Semifinal', 'Final'];

const esc = v => String(v == null ? '' : v).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
const clip = (s, n) => { s = String(s || ''); return s.length > n ? s.slice(0, n - 1) + '…' : s; };

// La ronda d'una etapa: «Semifinal 2» → Semifinal.
export function roundOf(stage) {
  const s = String(stage || '');
  return ROUND_ORDER.find(r => s === r || s.startsWith(r + ' ')) || '';
}

// Nom curt d'una etapa d'eliminatòria: «Semifinal 2» → «SF2».
export function shortStage(stage, lang = 'en') {
  const r = roundOf(stage);
  if (!r) return String(stage || '');
  const n = String(stage).slice(r.length).trim();
  return tx(lang).short[r] + (n && r !== 'Final' ? n : '');
}

// D'on surt un rival encara per decidir: «A1» → «1st Group A»,
// «W:Semifinal 1» → «Winner SF1».
export function seedLabel(from, lang = 'en') {
  const t = tx(lang), f = String(from || '');
  if (!f) return t.tbd;
  if (f.startsWith('W:')) return t.winner + ' ' + shortStage(f.slice(2), lang);
  const m = /^([A-H])(\d)$/.exec(f);
  if (m) return lang === 'ca' ? `${t.nth(+m[2])} ${t.group.toLowerCase()} ${m[1]}` : `${t.nth(+m[2])} ${t.group} ${m[1]}`;
  return f;
}

export const nameOf = (comp, owner) => (owner && comp && comp.names && comp.names[owner]) || (owner ? String(owner).replace(/^bot:/, 'Bot ') : '');

// Els jocs amb eliminatòries; per defecte, el del partit que es juga.
export function currentGame(comp) {
  if (!comp || !comp.schedule || !comp.schedule.length) return (comp && comp.games && comp.games[0]) || '';
  const cur = comp.schedule[Math.max(0, Math.min(comp.current || 0, comp.schedule.length - 1))];
  return (cur && cur.game) || comp.games[0];
}

// Es juga ara aquesta posició del calendari?
export const isLive = (comp, i) => !!comp && comp.status === 'running' && (comp.current || 0) === i && !comp.schedule[i].done;

// El marcador d'una posició: a les eliminatòries, les victòries de la sèrie.
export function slotScore(sl) {
  if (!sl) return null;
  if (sl.phase === 'playoff') return sl.a && sl.b ? sl.series || sl.score || [0, 0] : null;
  return sl.done ? (sl.score || [0, 0]) : null;
}

// bracketLayout col·loca l'arbre d'eliminatòries d'un joc: columnes per ronda
// (quarts, semis, final), cada partit a mig camí dels dos que l'alimenten, i
// les línies entre ells. Mides en unitats de l'SVG.
export function bracketLayout(comp, game, opt = {}) {
  const W = opt.cardW || 230, H = opt.cardH || 62, GX = opt.gapX || 56, GY = opt.gapY || 22, TOP = opt.top || 46;
  const sched = (comp && comp.schedule) || [];
  const slots = sched.map((s, i) => ({ s, i })).filter(x => x.s.game === game && x.s.phase === 'playoff');
  const rounds = ROUND_ORDER.map(r => ({ round: r, items: slots.filter(x => roundOf(x.s.stage) === r) })).filter(r => r.items.length);
  if (!rounds.length) return { rounds: [], links: [], width: 0, height: 0, cardW: W, cardH: H, champion: '' };
  rounds.forEach(r => r.items.sort((a, b) => String(a.s.stage).localeCompare(String(b.s.stage), 'en', { numeric: true })));
  const byStage = {};
  const out = [], links = [];
  const first = Math.max(...rounds.map(r => r.items.length));
  const height = TOP + first * H + (first - 1) * GY;
  rounds.forEach((r, ri) => {
    const x = ri * (W + GX);
    const cards = r.items.map((it, k) => {
      // Les que surten de partits anteriors: a mig camí dels dos.
      const feeders = (it.s.from || []).filter(f => String(f).startsWith('W:')).map(f => byStage[String(f).slice(2)]).filter(Boolean);
      let y;
      if (feeders.length) y = feeders.reduce((a, c) => a + c.y, 0) / feeders.length;
      else {
        const n = r.items.length, span = height - TOP;
        y = TOP + (n === 1 ? (span - H) / 2 : k * ((span - H) / (n - 1)));
      }
      const card = { index: it.i, stage: it.s.stage, round: r.round, x, y, w: W, h: H, slot: it.s, live: isLive(comp, it.i), feeders: feeders.map(c => c.stage) };
      feeders.forEach(c => links.push({ from: c.stage, to: card.stage, x1: c.x + W, y1: c.y + H / 2, x2: x, y2: y + H / 2 }));
      return card;
    });
    cards.forEach(c => { byStage[c.stage] = c; });
    out.push({ round: r.round, x, cards });
  });
  const champion = (comp.champion || {})[game] || '';
  let width = rounds.length * (W + GX) - GX;
  if (champion || opt.championColumn) width += GX + (opt.champW || 180);
  return { rounds: out, links, width, height, cardW: W, cardH: H, champion };
}

// Les dues bandes d'una posició, a punt per pintar: nom (o d'on surt), marcador
// i si ha guanyat.
export function slotSides(comp, sl, lang = 'en') {
  const sc = slotScore(sl);
  return [0, 1].map(k => {
    const owner = k ? sl.b : sl.a;
    const pend = !owner;
    return {
      owner, pending: pend,
      // Sense rival i ja resolta: el que hi havia s'ha classificat sense jugar.
      name: pend ? (sl.done && !(sl.from || [])[k] ? tx(lang).bye : seedLabel((sl.from || [])[k], lang)) : nameOf(comp, owner),
      score: sc ? sc[k] : null,
      won: !!sl.done && sl.winner === k,
      lost: !!sl.done && sl.winner === 1 - k,
    };
  });
}

// scheduleList: el calendari en tres trossos. past = els acabats (els últims
// `max`), live = el que es juga, next = els que vénen per ordre.
export function scheduleList(comp, opt = {}) {
  const max = opt.max || 5, game = opt.game || '';
  const sched = (comp && comp.schedule) || [];
  const all = sched.map((s, i) => ({ s, i })).filter(x => !game || x.s.game === game);
  const item = x => ({ index: x.i, game: x.s.game, stage: x.s.stage, phase: x.s.phase, sides: slotSides(comp, x.s, opt.lang), done: !!x.s.done,
    games: (x.s.matches || []).length + (x.s.match && !(x.s.matches || []).includes(x.s.match) ? 1 : 0) });
  const live = all.filter(x => isLive(comp, x.i)).map(item);
  const past = all.filter(x => x.s.done).map(item).slice(-max);
  const cur = comp ? comp.current || 0 : 0;
  const next = all.filter(x => !x.s.done && !isLive(comp, x.i)).sort((a, b) => (a.i < cur) - (b.i < cur) || a.i - b.i).map(item).slice(0, max);
  return { past, live, next };
}

// Les taules dels grups d'un joc (les calcula el servidor).
export function groupTables(comp, game) {
  return ((comp && comp.tables) || {})[game] || [];
}

// Es juguen ja (o s'han jugat) les eliminatòries d'un joc?
export function playoffsStarted(comp, game) {
  const sched = (comp && comp.schedule) || [];
  const po = sched.filter(s => s.game === game && s.phase === 'playoff');
  if (!po.length) return false;
  const groups = sched.filter(s => s.game === game && s.phase === 'group');
  return po.some(s => !s.pending || s.done || (s.matches || []).length) || (groups.length > 0 && groups.every(s => s.done));
}

// maskUnseen: la competició tal com la veu qui mira una retransmissió que va
// per darrere del servidor (sense espòilers). `unseen` són els partits que la
// tele encara no ha ensenyat; `winners` (id → owner, '' si empat) diu qui ha
// guanyat cadascun. Una sèrie amb partits per veure torna a estar en joc amb
// les victòries vistes; qui en surt encara no és a la ronda següent, i el
// campió encara no se sap. Torna una còpia; l'original no es toca.
export function maskUnseen(comp, unseen, winners = new Map()) {
  if (!comp || !comp.schedule || !unseen || !unseen.size) return comp;
  const c = { ...comp, champion: { ...(comp.champion || {}) }, schedule: comp.schedule.map(s => ({ ...s })) };
  const open = new Set(); // «game|stage» d'eliminatòries que tornen a estar en joc
  let first = -1;
  c.schedule.forEach((s, i) => {
    const ids = s.matches || [];
    const hidden = ids.filter(id => unseen.has(id));
    if (!hidden.length) return;
    if (first < 0) first = i;
    if (s.phase === 'playoff') {
      const ser = [...(s.series || s.score || [0, 0])];
      hidden.forEach(id => { const w = winners.get(id); if (w && w === s.a) ser[0]--; else if (w && w === s.b) ser[1]--; });
      s.series = s.score = [Math.max(0, ser[0]), Math.max(0, ser[1])];
      if (s.done) open.add(s.game + '|' + s.stage);
      // Els partits vistos, i el primer per veure és el que «es juga».
      s.matches = ids.filter(id => !unseen.has(id)); s.match = hidden[0];
    } else if (hidden.length === ids.length) s.score = [0, 0];
    s.done = false; s.winner = -1;
  });
  if (first < 0) return comp;
  // Qui surt d'una sèrie per veure encara no ha passat de ronda.
  c.schedule.forEach(s => {
    (s.from || []).forEach((f, k) => {
      if (!String(f).startsWith('W:') || !open.has(s.game + '|' + String(f).slice(2))) return;
      if (k) s.b = ''; else s.a = '';
      s.pending = true; s.done = false; s.winner = -1; s.series = s.score = [0, 0]; s.matches = []; delete s.match;
      open.add(s.game + '|' + s.stage);
    });
  });
  c.schedule.forEach(s => { if (s.phase === 'playoff' && open.has(s.game + '|' + s.stage)) delete c.champion[s.game]; });
  c.status = 'running';
  c.current = first;
  return c;
}

// ---- pintar ----

export const BRACKET_CSS = `
.abk { --abk-panel: var(--panel, rgba(13,18,34,.92)); --abk-line: var(--line, rgba(255,255,255,.14)); --abk-ink: var(--ink, #f3f5fb);
  --abk-muted: var(--muted, #9aa4bd); --abk-accent: var(--accent, #ffd400); --abk-live: #e5484d; color: var(--abk-ink); font-family: inherit; }
.abk-svg { width: 100%; height: 100%; display: block; overflow: visible; }
.abk-svg .abk-card { fill: var(--abk-panel); stroke: var(--abk-line); stroke-width: 1.5; }
.abk-svg .abk-card.abk-onair { stroke: var(--abk-live); stroke-width: 3; }
.abk-svg .abk-card.abk-done { stroke: rgba(255,255,255,.22); }
.abk-svg .abk-sep { stroke: var(--abk-line); }
.abk-svg .abk-nm { fill: var(--abk-ink); font-size: 17px; font-weight: 700; }
.abk-svg .abk-nm.abk-pend { fill: var(--abk-muted); font-style: italic; font-weight: 500; }
.abk-svg .abk-nm.abk-won, .abk-svg .abk-sc.abk-won { fill: var(--abk-accent); font-weight: 900; }
.abk-svg .abk-nm.abk-lost, .abk-svg .abk-sc.abk-lost { opacity: .55; }
.abk-svg .abk-sc { fill: var(--abk-ink); font-size: 18px; font-weight: 900; text-anchor: middle; }
.abk-svg .abk-scbox { fill: rgba(255,255,255,.07); }
.abk-svg .abk-stg { fill: var(--abk-muted); font-size: 12px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
.abk-svg .abk-rnd { fill: var(--abk-accent); font-size: 14px; font-weight: 800; letter-spacing: .14em; text-transform: uppercase; }
.abk-svg .abk-lnk { fill: none; stroke: var(--abk-line); stroke-width: 2.5; }
.abk-svg .abk-lnk.abk-on { stroke: var(--abk-accent); }
.abk-svg .abk-pill { fill: var(--abk-live); } .abk-svg .abk-pilltx { fill: #fff; font-size: 11px; font-weight: 900; letter-spacing: .1em; text-anchor: middle; }
.abk-svg .abk-champ { fill: var(--abk-accent); font-size: 22px; font-weight: 900; } .abk-svg .abk-champl { fill: var(--abk-muted); font-size: 12px; font-weight: 700; letter-spacing: .12em; text-transform: uppercase; }
.abk-groups { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 14px 22px; }
.abk-group { background: var(--abk-panel); border: 1px solid var(--abk-line); border-radius: 12px; padding: 8px 12px 6px; }
.abk-group h4 { margin: 2px 0 6px; color: var(--abk-accent); font-size: 13px; letter-spacing: .12em; text-transform: uppercase; }
.abk-group table { width: 100%; border-collapse: collapse; font-size: 16px; }
.abk-group th, .abk-group td { padding: 4px 6px; border-bottom: 1px solid var(--abk-line); text-align: center; }
.abk-group th { color: var(--abk-muted); font-size: 11px; text-transform: uppercase; letter-spacing: .08em; }
.abk-group td.n { text-align: left; font-weight: 700; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 220px; }
.abk-group td.pts { color: var(--abk-accent); font-weight: 900; }
.abk-group tr.q td:first-child { color: var(--abk-accent); font-weight: 900; }
.abk-group tr.q td:first-child::after { content: ' ▲'; font-size: 10px; }
.abk-sched { display: flex; flex-direction: column; gap: 10px; }
.abk-sched h4 { margin: 0 0 4px; color: var(--abk-muted); font-size: 12px; letter-spacing: .14em; text-transform: uppercase; }
.abk-sched h4.onair { color: var(--abk-live); }
.abk-sched ol { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }
.abk-sched li { display: grid; grid-template-columns: 64px 1fr auto 1fr; gap: 8px; align-items: center; background: var(--abk-panel); border: 1px solid var(--abk-line); border-radius: 8px; padding: 5px 10px; font-size: 15px; }
.abk-sched li.onair { border-color: var(--abk-live); }
.abk-sched li .st { color: var(--abk-muted); font-size: 11px; font-weight: 800; text-transform: uppercase; letter-spacing: .06em; }
.abk-sched li .a, .abk-sched li .b { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; } .abk-sched li .b { text-align: right; }
.abk-sched li .won { color: var(--abk-accent); font-weight: 800; } .abk-sched li .pend { color: var(--abk-muted); font-style: italic; }
.abk-sched li .sc { font-weight: 900; font-variant-numeric: tabular-nums; min-width: 46px; text-align: center; }
.abk-sched .none { color: var(--abk-muted); font-size: 14px; }
`;

export function ensureBracketCSS(doc = typeof document !== 'undefined' ? document : null) {
  if (!doc || doc.getElementById('abk-css')) return;
  const st = doc.createElement('style'); st.id = 'abk-css'; st.textContent = BRACKET_CSS;
  (doc.head || doc.documentElement).appendChild(st);
}

// bracketSVG: l'arbre com a SVG (s'escala sencer al seu contenidor).
export function bracketSVG(comp, game, opt = {}) {
  const lang = opt.lang || 'en', t = tx(lang);
  const L = bracketLayout(comp, game, opt);
  if (!L.rounds.length) return '';
  const W = L.cardW, H = L.cardH, half = H / 2, scW = 40;
  const parts = [];
  // Títols de ronda
  L.rounds.forEach(r => parts.push(`<text class="abk-rnd" x="${r.x}" y="14">${esc(t.rounds[r.round] || r.round)}</text>`));
  // Línies (en colze); la del guanyador, de color.
  L.links.forEach(k => {
    const mx = (k.x1 + k.x2) / 2;
    const src = L.rounds.flatMap(r => r.cards).find(c => c.stage === k.from);
    const on = src && src.slot.done;
    parts.push(`<path class="abk-lnk${on ? ' abk-on' : ''}" d="M${k.x1} ${k.y1} H${mx} V${k.y2} H${k.x2}"/>`);
  });
  L.rounds.forEach(r => r.cards.forEach(c => {
    const sides = slotSides(comp, c.slot, lang), cls = c.live ? ' abk-onair' : c.slot.done ? ' abk-done' : '';
    parts.push(`<g class="abk-m" data-stage="${esc(c.stage)}" transform="translate(${c.x} ${c.y})">`);
    parts.push(`<rect class="abk-card${cls}" width="${W}" height="${H}" rx="9"/>`);
    parts.push(`<line class="abk-sep" x1="8" x2="${W - 8}" y1="${half}" y2="${half}"/>`);
    parts.push(`<rect class="abk-scbox" x="${W - scW}" y="1" width="${scW - 1}" height="${H - 2}" rx="8"/>`);
    sides.forEach((sd, k) => {
      const y = k ? half + half / 2 + 6 : half / 2 + 6;
      const st = sd.pending ? ' abk-pend' : sd.won ? ' abk-won' : sd.lost ? ' abk-lost' : '';
      parts.push(`<text class="abk-nm${st}" x="12" y="${y}">${esc(clip(sd.name, opt.nameChars || 19))}</text>`);
      if (sd.score != null && !sd.pending) parts.push(`<text class="abk-sc${sd.won ? ' abk-won' : sd.lost ? ' abk-lost' : ''}" x="${W - scW / 2}" y="${y}">${sd.score}</text>`);
    });
    // Etapa a sobre (i «al millor de 3» la primera vegada).
    const games = (c.slot.matches || []).length + (c.live && c.slot.match && !(c.slot.matches || []).includes(c.slot.match) ? 1 : 0);
    // (Una ronda d'un sol partit ja té el nom a dalt: «Final».)
    const lab = (r.cards.length > 1 ? shortStage(c.stage, lang) : '') + (c.live && games ? (r.cards.length > 1 ? ' · ' : '') + t.game + ' ' + games : '');
    if (lab) parts.push(`<text class="abk-stg" x="2" y="-6">${esc(lab)}</text>`);
    if (c.live) parts.push(`<rect class="abk-pill" x="${W - 62}" y="-17" width="60" height="15" rx="4"/><text class="abk-pilltx" x="${W - 32}" y="-6">${esc(t.live)}</text>`);
    parts.push('</g>');
  }));
  if (L.champion) {
    const fin = L.rounds[L.rounds.length - 1].cards[0];
    const x = fin.x + W + (opt.gapX || 56);
    parts.push(`<path class="abk-lnk abk-on" d="M${fin.x + W} ${fin.y + half} H${x - 6}"/>`);
    parts.push(`<text class="abk-champl" x="${x}" y="${fin.y + half - 14}">🏆 ${esc(t.champion)}</text><text class="abk-champ" x="${x}" y="${fin.y + half + 12}">${esc(clip(nameOf(comp, L.champion), 16))}</text>`);
  }
  const pad = 8;
  // Com a molt, una mida i mitja (un quadre d'una sola final no ha d'omplir la pantalla).
  const k = opt.maxScale || 1.5, vw = L.width + pad * 2, vh = L.height + pad * 2;
  return `<svg class="abk-svg" style="max-width:${Math.round(vw * k)}px;max-height:${Math.round(vh * k)}px;margin:auto" xmlns="http://www.w3.org/2000/svg" viewBox="${-pad} ${-pad} ${vw} ${vh}" preserveAspectRatio="xMidYMid meet" role="img" aria-label="${esc(t.rounds.Final)}">${parts.join('')}</svg>`;
}

export function bracketHTML(comp, opt = {}) {
  const game = opt.game || currentGame(comp);
  const svg = bracketSVG(comp, game, opt);
  return svg ? `<div class="abk abk-bracket" style="display:flex">${svg}</div>` : '';
}

// groupsHTML: les taules dels grups (els dos primers passen, ▲).
export function groupsHTML(comp, opt = {}) {
  const lang = opt.lang || 'en', t = tx(lang), game = opt.game || currentGame(comp);
  let tables = groupTables(comp, game);
  if (opt.stage) tables = tables.filter(tb => tb.stage === opt.stage);
  if (!tables.length) return '';
  const qual = opt.qualify == null ? 2 : opt.qualify;
  const label = st => lang === 'ca' ? String(st).replace(/^Group /, t.group + ' ') : st;
  return `<div class="abk abk-groups">${tables.map(tb => `<div class="abk-group"><h4>${esc(label(tb.stage))}</h4><table><tr><th></th><th style="text-align:left">${esc(t.player)}</th><th>${t.p}</th><th>${t.w}</th><th>${t.d}</th><th>${t.l}</th><th>${t.pts}</th></tr>` +
    (tb.rows || []).map((r, i) => `<tr class="${i < qual ? 'q' : ''}"><td>${i + 1}</td><td class="n">${esc(nameOf(comp, r.owner))}</td><td>${r.played}</td><td>${r.won}</td><td>${r.drawn}</td><td>${r.lost}</td><td class="pts">${r.pts}</td></tr>`).join('') +
    '</table></div>').join('')}</div>`;
}

// scheduleHTML: jugats, en directe i el que ve.
export function scheduleHTML(comp, opt = {}) {
  const lang = opt.lang || 'en', t = tx(lang);
  const sl = scheduleList(comp, { ...opt, lang });
  const games = new Set(((comp && comp.schedule) || []).map(s => s.game));
  const stage = it => (games.size > 1 && !opt.game ? (GAME_TITLES[it.game] || it.game) + ' · ' : '') + (it.phase === 'playoff' ? shortStage(it.stage, lang) : (lang === 'ca' ? String(it.stage).replace(/^Group /, t.group + ' ') : it.stage));
  const li = (it, live) => {
    const [a, b] = it.sides, sc = a.score != null && b.score != null ? `${a.score} - ${b.score}` : 'vs';
    const cl = s => s.pending ? 'pend' : s.won ? 'won' : '';
    return `<li class="${live ? 'onair' : ''}"><span class="st">${esc(stage(it))}</span><span class="a ${cl(a)}">${esc(a.name)}</span><span class="sc">${esc(sc)}</span><span class="b ${cl(b)}">${esc(b.name)}</span></li>`;
  };
  const block = (title, items, live) => items.length ? `<div><h4 class="${live ? 'onair' : ''}">${esc(title)}</h4><ol>${items.map(x => li(x, live)).join('')}</ol></div>` : '';
  const html = block(t.now, sl.live, true) + block(t.next, sl.next) + block(t.past, sl.past.slice().reverse());
  return `<div class="abk abk-sched">${html || `<div class="none">${esc(t.empty)}</div>`}</div>`;
}

// ---- al DOM ----

export function renderBracket(el, comp, opt = {}) { ensureBracketCSS(el.ownerDocument); el.innerHTML = bracketHTML(comp, opt); return el; }
export function renderGroups(el, comp, opt = {}) { ensureBracketCSS(el.ownerDocument); el.innerHTML = groupsHTML(comp, opt); return el; }
export function renderSchedule(el, comp, opt = {}) { ensureBracketCSS(el.ownerDocument); el.innerHTML = scheduleHTML(comp, opt); return el; }

// renderCompetition: el quadre sencer d'un joc (grups, eliminatòries i
// calendari), per a la pàgina de l'Arena. opt: {game, lang, max}.
export function renderCompetition(el, comp, opt = {}) {
  ensureBracketCSS(el.ownerDocument);
  const game = opt.game || currentGame(comp);
  const o = { ...opt, game };
  const br = bracketHTML(comp, o);
  el.innerHTML = `<div class="abk abk-comp" style="display:grid;gap:18px">${groupsHTML(comp, o)}${br ? `<div style="aspect-ratio:${Math.max(1, bracketLayout(comp, game).width)}/${Math.max(1, bracketLayout(comp, game).height + 16)};max-height:520px">${br}</div>` : ''}${scheduleHTML(comp, o)}</div>`;
  return el;
}
