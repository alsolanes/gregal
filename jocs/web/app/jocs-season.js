// Gregal Jocs: la temporada vista per persones (/api/arena/season i
// /api/arena/player). jocs-site.js en pinta les pàgines: la classificació
// general amb els premis, els perfils de cada jugador amb l'informe d'IA,
// la galeria de millors moments i els principis de prompt que puntua el
// coach. Només fa HTML: els clics els porta jocs-site.js (data-player,
// data-moment, data-insight). Tot en anglès, com els jocs.

const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

// Els cinc criteris del coach (arenaFeedback), amb el que ensenyen.
export const PRINCIPLES = [
  { id: 'clarity', label: 'Clarity', icon: '🔍', teach: 'One meaning per line. Headings, short rules, no hedging the model has to guess.',
    bad: 'Be smart with the clock.', good: 'Q4, under 3:00, ahead → run_inside, tempo milk.' },
  { id: 'specificity', label: 'Specificity', icon: '🎯', teach: 'If/then rules with the exact option names and thresholds the game uses.',
    bad: 'Pass more on long downs.', good: '3rd & 7+ → pass_medium to wr1; if they blitz, screen to rb.' },
  { id: 'coverage', label: 'Coverage', icon: '🗺️', teach: 'An answer for every situation that comes up, plus a default when nothing matches.',
    bad: '(nothing about 4th down)', good: '4th down: field_goal in range, go for it on 4th & 1 past midfield, otherwise punt.' },
  { id: 'adaptivity', label: 'Adaptivity', icon: '🔄', teach: 'Read what the rival repeats and what is failing you, and say how to react.',
    bad: 'Always blitz.', good: 'If a call gained under 4 yds/play over 3+ plays, switch to option B.' },
  { id: 'consistency', label: 'Consistency', icon: '🧩', teach: 'No rules that fight each other. Give an order of priority for when two apply.',
    bad: 'Always run on 1st down. … Open with a deep pass.', good: 'Priorities: 1) 4th down 2) the clock 3) counter their top call.' },
];

const GAME_ICON = { nfl: '🏈', cs2: '💦', lol: '🏮' };
const GAME_SHORT = { nfl: 'NFL', cs2: 'Splash', lol: 'Lantern' };
const gameIcon = g => GAME_ICON[g] || '🎮';
const gameShort = g => GAME_SHORT[g] || g;

const initials = s => String(s || '?').replace(/\s*\(.*\)\s*$/, '').split(/\s+/).filter(Boolean).map(w => w[0]).join('').slice(0, 2).toUpperCase() || '?';
function hue(s) { let h = 0; for (const c of String(s)) h = (h * 31 + c.charCodeAt(0)) >>> 0; return h % 360; }

// Un avatar amb el color de la persona (sempre el mateix per al mateix nom).
export function avatar(p, cls = '') {
  const h = hue(p.owner || p.display);
  return `<span class="jx-pav ${cls}" style="--h:${h}" aria-hidden="true">${p.emoji ? `<i>${esc(p.emoji)}</i>` : ''}<b>${esc(initials(p.display))}</b></span>`;
}

const pct = v => Math.round(100 * (+v || 0)) + '%';
const ago = at => {
  const s = Math.max(0, Date.now() / 1000 - at);
  if (s < 90) return 'just now';
  if (s < 3600) return Math.round(s / 60) + ' min ago';
  if (s < 86400) return Math.round(s / 3600) + ' h ago';
  return new Date(at * 1000).toLocaleDateString('en', { month: 'short', day: 'numeric' });
};

export function formHTML(form) {
  if (!form || !form.length) return '<span class="jx-faint">—</span>';
  return `<span class="jx-form">${form.map(r => `<i class="f-${r}" title="${r === 'W' ? 'Win' : r === 'L' ? 'Loss' : 'Draw'}">${r}</i>`).join('')}</span>`;
}

// El radar dels cinc principis (1-5).
export function radarSVG(pr, size = 220) {
  const c = size / 2, R = c - 34, n = PRINCIPLES.length;
  const pt = (i, v) => { const a = -Math.PI / 2 + i * 2 * Math.PI / n; return [c + Math.cos(a) * R * v, c + Math.sin(a) * R * v]; };
  const ring = k => PRINCIPLES.map((_, i) => pt(i, k / 5).map(x => x.toFixed(1)).join(',')).join(' ');
  const grid = [1, 2, 3, 4, 5].map(k => `<polygon points="${ring(k)}" class="rg${k === 5 ? ' out' : ''}"/>`).join('');
  const axes = PRINCIPLES.map((_, i) => { const [x, y] = pt(i, 1); return `<line x1="${c}" y1="${c}" x2="${x.toFixed(1)}" y2="${y.toFixed(1)}"/>`; }).join('');
  const has = pr && Object.keys(pr).length;
  const shape = has ? PRINCIPLES.map((p, i) => pt(i, Math.max(.05, (+pr[p.id] || 0) / 5)).map(x => x.toFixed(1)).join(',')).join(' ') : '';
  const dots = has ? PRINCIPLES.map((p, i) => { const [x, y] = pt(i, Math.max(.05, (+pr[p.id] || 0) / 5)); return `<circle cx="${x.toFixed(1)}" cy="${y.toFixed(1)}" r="3.5"/>`; }).join('') : '';
  const labels = PRINCIPLES.map((p, i) => {
    const [x, y] = pt(i, 1.2), anchor = Math.abs(x - c) < 8 ? 'middle' : x > c ? 'start' : 'end';
    return `<text x="${x.toFixed(1)}" y="${(y + 4).toFixed(1)}" text-anchor="${anchor}">${esc(p.label)}${has ? ` <tspan>${(+pr[p.id] || 0).toFixed(1)}</tspan>` : ''}</text>`;
  }).join('');
  return `<svg class="jx-radar" viewBox="-30 0 ${size + 60} ${size}" role="img" aria-label="Prompting principles">${grid}<g class="ax">${axes}</g>${has ? `<polygon class="shape" points="${shape}"/><g class="dots">${dots}</g>` : ''}<g class="lb">${labels}</g></svg>`;
}

// La corba de l'Elo oficial ([unix, elo]...), començant a 1500.
export function sparkSVG(hist, w = 260, h = 70) {
  if (!hist || !hist.length) return '';
  const pts = [[hist[0][0] - 1, 1500], ...hist];
  const ys = pts.map(p => p[1]), lo = Math.min(...ys) - 8, hi = Math.max(...ys) + 8;
  const X = i => 4 + i * (w - 8) / Math.max(1, pts.length - 1), Y = v => h - 6 - (v - lo) * (h - 12) / Math.max(1, hi - lo);
  const line = pts.map((p, i) => `${X(i).toFixed(1)},${Y(p[1]).toFixed(1)}`).join(' ');
  const last = pts[pts.length - 1][1], up = last >= 1500;
  return `<svg class="jx-spark ${up ? 'up' : 'down'}" viewBox="0 0 ${w} ${h}" preserveAspectRatio="none" aria-hidden="true">
    <line x1="0" x2="${w}" y1="${Y(1500).toFixed(1)}" y2="${Y(1500).toFixed(1)}" class="base"/>
    <polygon points="${X(0)},${h} ${line} ${X(pts.length - 1)},${h}" class="fill"/><polyline points="${line}"/>
    <circle cx="${X(pts.length - 1).toFixed(1)}" cy="${Y(last).toFixed(1)}" r="4"/></svg>`;
}

// ---- premis ----

export function awardsHTML(awards, me, compact = false) {
  if (!awards || !awards.length) return '';
  return `<div class="jx-awards ${compact ? 'compact' : ''}">${awards.map(a => `<button type="button" class="jx-award a-${esc(a.id)} ${a.owner === me ? 'me' : ''}" data-player="${esc(a.owner)}">
    <span class="jx-award-ic" aria-hidden="true">${esc(a.icon)}</span><span class="jx-award-t">${esc(a.title)}</span>
    <b>${esc(a.display)}</b><small>${esc(a.value)}</small>${compact ? '' : `<em>${esc(a.why)}</em>`}</button>`).join('')}</div>`;
}

// ---- classificació general ----

export function standingsHTML(season, me) {
  const ps = season.players || [];
  if (!ps.length) return '<div class="jx-empty">No players yet. <a href="#/play">Write your strategy →</a></div>';
  const anyElo = ps.some(p => p.elo > 0);
  const row = (p, i) => {
    const bots = Object.values(p.games || {}).reduce((n, g) => n + (g.bots || []).length, 0);
    return `<tr class="${p.owner === me ? 'me' : ''} ${i < 3 ? 'top' + (i + 1) : ''}" data-player="${esc(p.owner)}" tabindex="0">
      <td class="pos">${i < 3 ? ['🥇', '🥈', '🥉'][i] : i + 1}</td>
      <td><span class="jx-who">${avatar(p, 'sm')}<span><b>${esc(p.display)}${p.owner === me ? ' <small class="jx-you">YOU</small>' : ''}</b>${p.archetype ? `<small>${esc(p.archetype)}</small>` : ''}</span></span></td>
      <td class="r elo">${p.elo > 0 ? `<b>${p.elo}</b>` : '<span class="jx-faint">—</span>'}</td>
      <td class="r">${p.official[0]}–${p.official[1]}${p.official[2] ? '–' + p.official[2] : ''}</td>
      <td><span class="jx-ladder" title="${bots} bot wins">${'<i class="on"></i>'.repeat(Math.min(bots, 4))}${'<i></i>'.repeat(Math.max(0, 4 - bots))}</span>${bots > 4 ? `<small> +${bots - 4}</small>` : ''}</td>
      <td class="r">${p.reviewed ? `<b class="jx-ps">${p.prompt_score}</b>` : '<span class="jx-faint">—</span>'}</td>
      <td class="r">${p.followed ? pct(p.followed) : '<span class="jx-faint">—</span>'}</td>
      <td>${formHTML(p.form)}</td></tr>`;
  };
  return `<p class="jx-muted jx-note-s">${anyElo ? 'Ordered by official Elo (average over the games played), then the practice ladder and the prompt score.' : 'No official matches yet: the table follows the practice ladder (bots beaten, from Rookie to Analyst) and the coach’s prompt score. The tournament’s Elo takes over once it starts.'}</p>
    <table class="jx-table jx-standings"><thead><tr><th>#</th><th>Player</th><th class="r">Elo</th><th class="r">Official</th><th>Bot ladder</th><th class="r" title="Average coach score on the five principles, out of 100">Prompt</th><th class="r" title="Decisions where the model found a real line of the .md">By the book</th><th>Form</th></tr></thead>
    <tbody>${ps.map(row).join('')}</tbody></table>`;
}

// ---- jugadors ----

export function playersHTML(season, me) {
  const ps = season.players || [];
  if (!ps.length) return '<div class="jx-empty">No players yet. <a href="#/play">Be the first →</a></div>';
  return `<div class="jx-pcards">${ps.map(p => `<button type="button" class="jx-pcard ${p.owner === me ? 'me' : ''}" data-player="${esc(p.owner)}" style="--h:${hue(p.owner)}">
    <span class="jx-pcard-rank">#${p.rank}</span>${avatar(p, 'lg')}
    <b class="jx-pcard-n">${esc(p.display)}</b>
    <span class="jx-pcard-a">${p.archetype ? esc(p.archetype) : p.played ? 'Scouting report pending' : 'Warming up'}</span>
    <span class="jx-pcard-s"><span><b>${p.elo > 0 ? p.elo : '—'}</b><small>Elo</small></span><span><b>${p.won}-${p.lost}</b><small>W-L</small></span><span><b>${p.reviewed ? p.prompt_score : '—'}</b><small>Prompt</small></span></span>
    ${formHTML(p.form)}</button>`).join('')}</div>`;
}

export function profileHTML(d, me, opts = {}) {
  const p = d.player, ins = d.insight, games = Object.entries(p.games || {}).sort((a, b) => (b[1].played || 0) - (a[1].played || 0));
  const top = PRINCIPLES.slice().sort((a, b) => ((p.principles || {})[b.id] || 0) - ((p.principles || {})[a.id] || 0));
  const stat = (v, l) => `<div><b>${v}</b><span>${l}</span></div>`;
  const insight = ins ? `<div class="jx-insight">
      <div class="jx-insight-k"><span>AI scouting report</span>${d.insight_stale && opts.canRefresh ? `<button type="button" class="jx-btn ghost sm" data-insight="${esc(p.owner)}">↻ Update</button>` : ''}</div>
      <p class="jx-insight-sum">${esc(ins.summary)}</p>
      ${ins.signature ? `<blockquote class="jx-sig"><small>Signature move</small>${esc(ins.signature)}</blockquote>` : ''}
      <div class="jx-insight-grid">
        <div class="plus"><small>Strength</small>${esc(ins.strength)}</div>
        <div class="minus"><small>Watch out</small>${esc(ins.weakness)}</div>
        <div class="tip"><small>Next version${ins.principle ? ` · <em>${esc(ins.principle)}</em>` : ''}</small>${esc(ins.tip)}</div>
        <div class="rival"><small>How to beat them</small>${esc(ins.rival)}</div>
      </div>
      <p class="jx-faint jx-insight-f">Written by the model from public data (results, coach scores, official matches) · ${esc(ago(ins.at))}</p></div>`
    : p.played ? `<div class="jx-insight empty"><div class="jx-insight-k"><span>AI scouting report</span></div>${opts.busy ? '<p class="jx-muted"><span class="jx-spin"></span> The analyst is watching the tapes…</p>' : `<p class="jx-muted">No report yet.</p><button type="button" class="jx-btn" data-insight="${esc(p.owner)}">✨ Write the scouting report</button>`}${opts.error ? `<p class="jx-err-s">${esc(opts.error)}</p>` : ''}</div>`
      : `<div class="jx-insight empty"><div class="jx-insight-k"><span>AI scouting report</span></div><p class="jx-muted">The report comes after the first match.</p></div>`;
  const hl = d.highlights || [];
  const ms = d.matches || [];
  return `<div class="jx-profile" style="--h:${hue(p.owner)}">
    <a class="jx-back" href="#/players">← All players</a>
    <header class="jx-prof-head">
      ${avatar(p, 'xl')}
      <div class="jx-prof-id"><div class="jx-kicker">#${p.rank} overall${p.owner === me ? ' · you' : ''}</div>
        <h1 class="jx-h1">${esc(p.display)}</h1>
        ${ins ? `<div class="jx-arch"><span>${esc(ins.emoji || '')}</span><b>${esc(ins.archetype)}</b><em>${esc(ins.tagline || '')}</em></div>` : ''}
        <div class="jx-prof-form">${formHTML(p.form)}${p.streak >= 2 ? `<span class="jx-chip hot">🔥 ${p.streak} in a row</span>` : ''}</div></div>
    </header>
    <div class="jx-stats jx-prof-stats" style="--n:6">${stat(p.elo > 0 ? p.elo : '—', 'official Elo')}${stat(`${p.won}-${p.lost}${p.drawn ? '-' + p.drawn : ''}`, 'W-L overall')}${stat(`${p.official[0]}-${p.official[1]}`, 'official W-L')}${stat(p.bots_beaten, 'bots beaten')}${stat(p.reviewed ? p.prompt_score : '—', 'prompt score')}${stat(p.followed ? pct(p.followed) : '—', 'by the book')}</div>
    <div class="jx-prof-grid">
      ${insight}
      <div class="jx-prof-side">
        <div class="jx-panel"><div class="jx-panel-k">Prompting principles</div>${radarSVG(p.principles)}
          ${p.reviewed ? `<p class="jx-muted sm">Coach average over ${p.reviewed} match${p.reviewed === 1 ? '' : 'es'}. Best at <b>${esc(top[0].label.toLowerCase())}</b>; most room in <b>${esc(top[top.length - 1].label.toLowerCase())}</b>.</p>` : '<p class="jx-muted sm">Appears after the first coach report.</p>'}</div>
        ${games.length ? `<div class="jx-panel"><div class="jx-panel-k">By game</div>${games.map(([g, x]) => `<div class="jx-pgame"><div class="jx-pgame-h"><span>${gameIcon(g)} ${esc(gameShort(g))}</span><b>${x.elo > 0 ? x.elo + ' Elo' : 'unranked'}${x.rank ? ` · #${x.rank}` : ''}</b></div>
          ${sparkSVG(x.history)}<div class="jx-pgame-f"><span>${x.played ? `${x.won}W ${x.lost}L official` : 'no official matches'}</span><span>${(x.bots || []).length ? 'beat ' + x.bots.map(esc).join(', ') : ''}</span></div></div>`).join('')}</div>` : ''}
      </div>
    </div>
    ${hl.length ? `<div class="jx-sec-head"><div><div class="jx-kicker">The prompt behind the play</div><h2 class="jx-h2">Best moments</h2></div></div><div class="jx-moments">${hl.map(h => momentCard(h)).join('')}</div>` : ''}
    ${ms.length ? `<div class="jx-sec-head"><div><div class="jx-kicker">Replays</div><h2 class="jx-h2">Matches</h2></div></div>
      <div class="jx-plist">${ms.map(m => `<button type="button" class="jx-prow r-${esc(m.result)}" data-match="${esc(m.id)}"><span class="jx-prow-r">${esc(m.result)}</span><span>${gameIcon(m.game)} vs <b>${esc(m.rival)}</b></span><span class="jx-faint">${m.kind === 'official' ? esc(m.stage || 'Official') : 'Practice'}</span><b class="jx-prow-s">${m.score[0]}–${m.score[1]}</b><span class="jx-faint">${esc(ago(m.at))}</span><span class="jx-play-ic">▶</span></button>`).join('')}</div>` : ''}
  </div>`;
}

// ---- millors moments ----

// Una fila de taula del .md («| base | pass_medium to wr1 |») es llegeix millor sense les barres.
const ruleText = r => (String(r || '').match(/\|/g) || []).length >= 2 ? String(r).replace(/^\s*\|\s*|\s*\|\s*$/g, '').split(/\s*\|\s*/).filter(Boolean).join('  ·  ') : String(r || '');

export function momentCard(h, big = false) {
  const tags = [h.kind === 'official' ? `<span class="t off">${esc(h.stage || 'Official')}</span>` : '<span class="t">Practice</span>', h.lead ? '<span class="t lead">↗ Go-ahead</span>' : '', h.clutch ? '<span class="t clutch">⏱ Clutch</span>' : '', h.won ? '<span class="t won">Won the match</span>' : ''].join('');
  return `<article class="jx-moment ${big ? 'big' : ''}" style="--h:${hue(h.owner)}">
    <div class="jx-moment-k"><span>${gameIcon(h.game)} ${esc(gameShort(h.game))}</span>${tags}</div>
    <div class="jx-moment-who"><button type="button" class="jx-linkbtn" data-player="${esc(h.owner)}">${esc(h.display)}</button><span class="jx-faint">vs ${esc(h.rival)} · ${h.score[0]}–${h.score[1]}</span></div>
    <p class="jx-moment-what">${esc(h.what)}</p>
    <blockquote class="jx-rule-q"><small>The line of the .md</small>${esc(ruleText(h.rule))}</blockquote>
    ${h.why ? `<p class="jx-moment-why">${esc(h.why)}</p>` : ''}
    <button type="button" class="jx-btn ${big ? 'hot' : 'ghost'} sm" data-moment="${esc(h.match)}" data-at="${h.event}">▶ Watch the play</button></article>`;
}

export function highlightsHTML(season, filter) {
  let hl = season.highlights || [];
  if (filter && filter !== 'all') hl = hl.filter(h => filter === 'official' ? h.kind === 'official' : h.game === filter);
  if (!hl.length) return '<div class="jx-empty">No prompt moments here yet. They appear when a line of someone’s .md swings a match.</div>';
  const [first, ...rest] = hl;
  return `<div class="jx-moment-hero"><div class="jx-kicker">${first.kind === 'official' ? '🏅 Moment of the season' : '⭐ Top moment (only you see your practice)'}</div>${momentCard(first, true)}</div>
    ${rest.length ? `<div class="jx-moments">${rest.map(h => momentCard(h)).join('')}</div>` : ''}`;
}

// ---- com s'escriu un bon prompt ----

export function principlesHTML() {
  return `<div class="jx-sec-head"><div><div class="jx-kicker">What the coach scores</div><h2 class="jx-h2">Five prompting principles</h2></div></div>
    <p class="jx-muted">After every match the coach scores your .md from 1 to 5 on each principle. They are the same ones that make any prompt or agent instruction work at your desk.</p>
    <div class="jx-princ">${PRINCIPLES.map(p => `<article><div class="jx-princ-h"><span aria-hidden="true">${p.icon}</span><h3>${esc(p.label)}</h3></div><p>${esc(p.teach)}</p>
      <div class="jx-princ-ex"><div class="bad"><small>Vague</small><code>${esc(p.bad)}</code></div><div class="good"><small>Better</small><code>${esc(p.good)}</code></div></div></article>`).join('')}</div>`;
}
