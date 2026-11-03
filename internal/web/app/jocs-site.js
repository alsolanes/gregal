// Gregal Jocs (/jocs/): la web de la competició de prompts. És una pàgina
// sencera per sobre de la closca de Gregal (la mateixa sessió, el mateix
// window.api): portada amb l'estat de la competició, els tres jocs, el
// rànquing, el quadre de cada competició, els partits i com funciona.
// «Juga» hi encasta l'Arena de sempre (arena.js), que continua fent
// l'editor, els partits de prova, el reproductor i la tele.
// Tot en anglès, com els jocs.
import { renderCompetition, nameOf, currentGame } from './arena-bracket.js';

const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const n0 = v => Math.round(+v || 0).toLocaleString('en');
const api = (path, opts) => window.api(path, opts).then(async r => { const j = await r.json().catch(() => ({})); if (!r.ok) throw new Error(j.error || r.status); return j; });
const token = () => { try { return localStorage.getItem('gregal_token') || ''; } catch (e) { return ''; } };
const ago = at => {
  const s = Math.max(0, Date.now() / 1000 - at);
  if (s < 90) return 'just now';
  if (s < 3600) return Math.round(s / 60) + ' min ago';
  if (s < 86400) return Math.round(s / 3600) + ' h ago';
  return new Date(at * 1000).toLocaleDateString('en', { month: 'short', day: 'numeric' });
};

const GAMES = {
  nfl: { short: 'NFL', icon: '🏈', tag: 'Every snap is a call', decide: 'Each offensive play and each defensive front, down by down.', read: 'Read the defence, punish what repeats, manage the clock.' },
  cs2: { short: 'CS2', icon: '💣', tag: 'Buy, plan, execute', decide: 'The buy, the plan and the site for every round.', read: 'Manage the economy and break the rival\'s habits.' },
  lol: { short: 'LoL', icon: '🐉', tag: 'Draft to Nexus', decide: 'The draft, then objective and stance phase by phase.', read: 'Trade objectives, time your fights, scale or snowball.' },
};

const ROUTES = [
  ['home', 'Home'], ['play', 'Play'], ['ranking', 'Leaderboard'], ['cup', 'Tournament'], ['matches', 'Matches'], ['how', 'How it works'],
];

const S = { d: null, route: 'home', arg: '', rankGame: 'nfl', rankBots: true, matchGame: 'all', comp: null, compGame: '', timer: 0, liveComp: null };

function boot() {
  document.body.classList.add('jx-on');
  const root = document.createElement('div');
  root.id = 'jxSite'; root.className = 'jx';
  root.innerHTML = `
    <header class="jx-top">
      <a class="jx-brand" href="#/" aria-label="Home">${markSVG()}<span><b id="jxName">Gregal Prompt Cup</b><small>Gregal Jocs</small></span></a>
      <nav class="jx-nav" id="jxNav">${ROUTES.map(([id, label]) => `<a href="#/${id === 'home' ? '' : id}" data-r="${id}">${label}</a>`).join('')}</nav>
      <div class="jx-top-r">
        <a class="jx-watch" id="jxWatch" target="_blank" rel="noopener"><span class="jx-dot"></span><span id="jxWatchTx">Broadcast</span></a>
        <span class="jx-me" id="jxMe" hidden></span>
        <details class="jx-apps"><summary aria-label="Other Gregal apps">⋯</summary><div><a href="/">Gregal Codi</a><a href="/vents/">Gregal Vents</a></div></details>
      </div>
    </header>
    <main class="jx-main" id="jxMain">
      <section class="jx-route" data-route="home" id="jxHome"></section>
      <section class="jx-route" data-route="play"><div class="jx-wrap"><div class="jx-play-head" id="jxPlayHead"></div><div id="jxPlayMount"></div></div></section>
      <section class="jx-route" data-route="ranking" id="jxRanking"></section>
      <section class="jx-route" data-route="cup"><div class="jx-wrap"><div id="jxCup"></div><div id="jxOrganizer" class="jx-organizer" hidden></div></div></section>
      <section class="jx-route" data-route="matches" id="jxMatches"></section>
      <section class="jx-route" data-route="how" id="jxHow"></section>
    </main>
    <footer class="jx-foot"><div class="jx-wrap"><span>Same model for everyone. Only the prompt changes.</span><span>Gregal Jocs</span></div></footer>`;
  document.body.append(root);
  root.addEventListener('click', onClick);
  addEventListener('hashchange', route);
  $('jxWatch').href = streamURL();
  route();
  load();
}

const $ = id => document.getElementById(id);
const streamURL = () => '/app/arena-stream.html#lang=en' + (token() ? '&token=' + encodeURIComponent(token()) : '');

function markSVG() {
  return `<svg class="jx-mark" viewBox="0 0 40 40" aria-hidden="true"><defs><linearGradient id="jxg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#5cf2c8"/><stop offset="1" stop-color="#ff7a66"/></linearGradient></defs>
    <rect x="1" y="1" width="38" height="38" rx="11" fill="#0d2a31" stroke="url(#jxg)" stroke-width="2"/>
    <path d="M9 15c6-4 12 4 22-1M9 21c8-4 13 4 22 0M12 27c6-3 10 3 17 0" fill="none" stroke="url(#jxg)" stroke-width="2.6" stroke-linecap="round"/></svg>`;
}

async function load() {
  try { S.d = await api('/api/arena'); }
  catch (e) { $('jxHome').innerHTML = `<div class="jx-wrap"><p class="jx-err">${esc(e.message)}</p></div>`; return; }
  const d = S.d;
  $('jxName').textContent = d.event?.name || 'Gregal Prompt Cup';
  document.title = (d.event?.name || 'Gregal Prompt Cup') + ' · Gregal Jocs';
  const me = $('jxMe'); me.hidden = false; me.textContent = (d.display || d.me || '').slice(0, 2).toUpperCase(); me.title = 'Playing as ' + (d.display || d.me);
  // El rànquing obre pel primer joc que ja té classificació.
  if (!(d.leaderboard[S.rankGame] || []).length) S.rankGame = (d.games.find(g => (d.leaderboard[g.id] || []).length) || d.games[0] || {}).id || 'nfl';
  const running = (d.competitions || []).find(c => c.status === 'running');
  S.liveComp = null;
  if (running) { try { S.liveComp = await api('/api/arena/competition?id=' + encodeURIComponent(running.id)); } catch (e) {} }
  $('jxWatch').classList.toggle('live', !!running);
  $('jxWatchTx').textContent = running ? 'Watch live' : 'Broadcast';
  paint();
  clearTimeout(S.timer);
  S.timer = setTimeout(load, running ? 10000 : 45000);
}

function route() {
  const h = location.hash.replace(/^#\/?/, '');
  const [r, arg] = h.split('/');
  S.route = ROUTES.some(x => x[0] === r) ? r : 'home';
  S.arg = decodeURIComponent(arg || '');
  document.querySelectorAll('#jxNav [data-r]').forEach(a => a.classList.toggle('on', a.dataset.r === S.route));
  document.querySelectorAll('#jxSite .jx-route').forEach(s => { s.hidden = s.dataset.route !== S.route; });
  // L'Arena només treballa (i consulta) quan es veu.
  const page = $('arenaPage');
  if (S.route === 'play') mountArena();
  else if (page) page.hidden = true;
  if (S.route === 'cup') mountOrganizer();
  $('jxSite').scrollTop = 0;
  if (S.d) paint();
}

function paint() {
  if (!S.d) return;
  if (S.route === 'home') paintHome();
  if (S.route === 'play') paintPlayHead();
  if (S.route === 'ranking') paintRanking();
  if (S.route === 'cup') paintCup();
  if (S.route === 'matches') paintMatches();
  if (S.route === 'how') paintHow();
}

// ---- Juga: l'Arena de sempre, encastada ----

function mountArena() {
  const page = $('arenaPage'), mount = $('jxPlayMount');
  if (!page || !mount) return;
  if (page.parentElement !== mount) mount.append(page);
  page.hidden = false;
  const ar = window.gregalArena;
  if (!ar) return;
  if (S.arg && GAMES[S.arg] && ar.game !== S.arg) { ar.game = S.arg; if (ar.data) ar.paint(); }
  ar.load();
}

function paintPlayHead() {
  const d = S.d, g = window.gregalArena?.game || 'nfl';
  const rank = rankOf(g, d.me);
  $('jxPlayHead').innerHTML = `<div class="jx-kicker">Your lab</div><h1 class="jx-h1">Write the playbook. <span class="jx-grad">Let the model play it.</span></h1>
    <div class="jx-chips">${rank ? `<span class="jx-chip">#${rank.pos} in ${esc(GAMES[g]?.short || g)} · ${rank.elo} Elo</span>` : `<span class="jx-chip">Unranked in ${esc(GAMES[g]?.short || g)} yet</span>`}
    ${typeof d.practice_left === 'number' && !d.admin ? `<span class="jx-chip">${Math.max(0, d.practice_left)} practice matches left today</span>` : ''}</div>`;
}

// L'organitzador (admin): crear, començar i aturar competicions, i la
// retransmissió. És el plec de competicions de l'Arena, mogut aquí.
function mountOrganizer() {
  const box = $('jxOrganizer');
  if (!box || !S.d || !S.d.admin) { if (box) box.hidden = true; return; }
  const ar = window.gregalArena;
  const fold = document.querySelector('#arenaPage #arComps')?.closest('details');
  if (!fold) {
    // L'Arena encara no s'ha construït: es fa sense mostrar-la.
    if (ar && !$('arenaPage')?.dataset.ready) { ar.load().then(() => mountOrganizer()); }
    return;
  }
  box.hidden = false;
  if (!box.querySelector('.jx-org-head')) {
    box.innerHTML = `<div class="jx-org-head"><div><div class="jx-kicker">Organizer</div><h2 class="jx-h2">Run the tournament</h2>
      <p class="jx-muted">Create a competition from everyone's current strategy, start it and share the broadcast. Only admins see this.</p></div></div>
      <form class="jx-event" id="jxEventForm"><label>Event name<input name="name" maxlength="60"></label><label>Tagline<input name="tagline" maxlength="140"></label>
      <label>Date<input name="date" type="datetime-local"></label><label>Place<input name="place" maxlength="80" placeholder="Main hall · Teams"></label><button class="jx-btn" type="submit">Save event</button><span class="jx-muted" id="jxEventSaved"></span></form>`;
    const f = $('jxEventForm'), ev = S.d.event || {};
    f.name.value = ev.name || ''; f.tagline.value = ev.tagline || ''; f.place.value = ev.place || '';
    if (ev.date) f.date.value = new Date(ev.date * 1000 - new Date().getTimezoneOffset() * 60000).toISOString().slice(0, 16);
    f.onsubmit = async e => {
      e.preventDefault();
      const date = f.date.value ? Math.round(new Date(f.date.value).getTime() / 1000) : 0;
      try { S.d.event = await api('/api/arena', { method: 'POST', body: JSON.stringify({ action: 'event', event: { name: f.name.value, tagline: f.tagline.value, date, place: f.place.value } }) }); $('jxEventSaved').textContent = 'Saved'; load(); }
      catch (err) { $('jxEventSaved').textContent = err.message; }
    };
  }
  if (fold.parentElement !== box) { box.append(fold); fold.open = true; fold.classList.add('jx-org-fold'); }
}

// ---- Portada ----

function rankOf(game, owner) {
  const b = (S.d.leaderboard || {})[game] || [];
  const i = b.findIndex(r => r.owner === owner);
  return i < 0 ? null : { pos: i + 1, ...b[i] };
}

function heroStatus() {
  const d = S.d, c = S.liveComp;
  if (c) {
    const i = c.current || 0, sl = c.schedule[i] || {}, g = sl.game || currentGame(c);
    return `<div class="jx-status live"><div class="jx-status-k"><span class="jx-dot"></span>Live now · ${esc(c.title)}</div>
      <div class="jx-vs"><span>${esc(nameOf(c, sl.a))}</span><b>${sl.score ? esc(sl.score[0] + ' – ' + sl.score[1]) : 'vs'}</b><span>${esc(nameOf(c, sl.b))}</span></div>
      <div class="jx-muted">${esc(GAMES[g]?.short || g)} · ${esc(sl.stage || '')}</div>
      <a class="jx-btn hot" href="${esc(streamURL())}" target="_blank" rel="noopener">▶ Watch the broadcast</a></div>`;
  }
  const at = d.event?.date || 0;
  if (at > Date.now() / 1000) {
    return `<div class="jx-status"><div class="jx-status-k">Kick-off</div><div class="jx-count" id="jxCount" data-at="${at}">${countdown(at)}</div>
      <div class="jx-muted">${esc(new Date(at * 1000).toLocaleString('en', { weekday: 'long', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit' }))}${d.event.place ? ' · ' + esc(d.event.place) : ''}</div></div>`;
  }
  const last = (d.competitions || []).find(x => x.status === 'done' && x.champion && Object.keys(x.champion).length);
  if (last) {
    const champs = Object.entries(last.champion).map(([g, o]) => `<li><span>${GAMES[g]?.icon || '🎮'}</span><b>${esc(displayOf(g, o))}</b><small>${esc(GAMES[g]?.short || g)}</small></li>`).join('');
    return `<div class="jx-status"><div class="jx-status-k">🏆 Reigning champions · ${esc(last.title)}</div><ul class="jx-champs">${champs}</ul>
      <a class="jx-btn ghost" href="#/cup/${esc(last.id)}">See the bracket →</a></div>`;
  }
  return `<div class="jx-status"><div class="jx-status-k">Season open</div><p class="jx-muted">No tournament yet. Write your strategy and climb the practice ladder against the bots.</p></div>`;
}

function countdown(at) {
  let s = Math.max(0, Math.round(at - Date.now() / 1000));
  const dd = Math.floor(s / 86400); s -= dd * 86400; const hh = Math.floor(s / 3600); s -= hh * 3600; const mm = Math.floor(s / 60); s -= mm * 60;
  const cell = (v, l) => `<span><b>${String(v).padStart(2, '0')}</b><small>${l}</small></span>`;
  return cell(dd, 'days') + cell(hh, 'hrs') + cell(mm, 'min') + cell(s, 'sec');
}
setInterval(() => { const el = $('jxCount'); if (el) el.innerHTML = countdown(+el.dataset.at); }, 1000);

function displayOf(game, owner) {
  const r = ((S.d.leaderboard || {})[game] || []).find(x => x.owner === owner);
  return r ? r.display : String(owner || '').replace(/^bot:/, 'Bot ');
}

function paintHome() {
  const d = S.d, ev = d.event || {}, t = d.totals || {};
  const humans = new Set(Object.values(d.leaderboard || {}).flat().filter(r => !r.bot).map(r => r.owner));
  const cards = d.games.map(g => {
    const info = GAMES[g.id] || { short: g.id, icon: '🎮', tag: '', decide: '' };
    const rank = rankOf(g.id, d.me), mine = d.mine[g.id];
    return `<article class="jx-game jx-g-${esc(g.id)}">
      <div class="jx-game-art" aria-hidden="true"><span>${info.icon}</span></div>
      <div class="jx-game-body"><div class="jx-kicker">${esc(info.tag)}</div><h3>${esc(g.title)}</h3><p>${esc(info.decide)}</p>
        <div class="jx-game-me">${rank ? `<b>#${rank.pos}</b><span>${rank.elo} Elo · ${rank.won}W ${rank.lost}L</span>` : mine ? '<span>Strategy saved · not ranked yet</span>' : '<span>Not started</span>'}</div>
        <a class="jx-btn" href="#/play/${esc(g.id)}">${mine ? 'Improve my strategy' : 'Write my strategy'} →</a></div></article>`;
  }).join('');
  const podiums = d.games.map(g => {
    const top = ((d.leaderboard || {})[g.id] || []).slice(0, 3);
    if (!top.length) return '';
    const order = [top[1], top[0], top[2]];
    return `<div class="jx-podium"><h4>${GAMES[g.id]?.icon || ''} ${esc(GAMES[g.id]?.short || g.id)}</h4><div class="jx-steps">${order.map((r, i) => r ? `<div class="jx-step p${[2, 1, 3][i]} ${r.owner === d.me ? 'me' : ''}">
        <span class="jx-av">${esc(initials(r.display))}</span><b>${esc(r.display)}${r.bot ? ' <small>BOT</small>' : ''}</b><small>${r.elo} Elo</small><div class="jx-block">${[2, 1, 3][i]}</div></div>` : '<div class="jx-step empty"></div>').join('')}</div></div>`;
  }).join('');
  const recent = (d.matches || []).filter(m => m.status === 'final').slice(0, 8);
  $('jxHome').innerHTML = `
    <div class="jx-hero">
      <div class="jx-wind" aria-hidden="true">${windSVG()}</div>
      <div class="jx-wrap jx-hero-in">
        <div class="jx-hero-l">
          <div class="jx-kicker">Prompt competition${ev.place ? ' · ' + esc(ev.place) : ''}</div>
          <h1 class="jx-title">${esc(ev.name || 'Gregal Prompt Cup')}</h1>
          <p class="jx-lead">${esc(ev.tagline || '')}</p>
          <div class="jx-cta"><a class="jx-btn hot big" href="#/play">Write your strategy</a><a class="jx-btn ghost big" href="#/how">How it works</a></div>
        </div>
        <div class="jx-hero-r">${heroStatus()}</div>
      </div>
    </div>
    <div class="jx-wrap">
      <div class="jx-stats">
        <div><b>${n0(Math.max(t.players || 0, humans.size))}</b><span>players</span></div>
        <div><b>${n0(t.matches)}</b><span>matches played</span></div>
        <div><b>${n0(t.official)}</b><span>official matches</span></div>
        <div><b>${n0(t.decisions)}</b><span>calls made by the model</span></div>
      </div>
      <div class="jx-sec-head"><div><div class="jx-kicker">Three games · one model</div><h2 class="jx-h2">Pick your arena</h2></div></div>
      <div class="jx-games">${cards}</div>
      ${podiums ? `<div class="jx-sec-head"><div><div class="jx-kicker">Elo ranking</div><h2 class="jx-h2">Top of the table</h2></div><a class="jx-more" href="#/ranking">Full leaderboard →</a></div><div class="jx-podiums">${podiums}</div>` : ''}
      ${recent.length ? `<div class="jx-sec-head"><div><div class="jx-kicker">Results</div><h2 class="jx-h2">Latest matches</h2></div><a class="jx-more" href="#/matches">All matches →</a></div><div class="jx-results">${recent.map(matchCard).join('')}</div>` : ''}
      <div class="jx-sec-head"><div><div class="jx-kicker">The format</div><h2 class="jx-h2">Three steps to the trophy</h2></div></div>
      ${stepsHTML()}
    </div>`;
}

function stepsHTML() {
  return `<ol class="jx-how">
    <li><span>1</span><h3>Write a .md</h3><p>Plain English instructions for your team. Start from the template or let the builder ask you three questions.</p></li>
    <li><span>2</span><h3>Practice against bots</h3><p>The same model plays both sides, reading each team's .md at every decision. A coach report tells you what to fix.</p></li>
    <li><span>3</span><h3>Win the cup</h3><p>On competition day every strategy is frozen, seeded into groups by Elo and played out live, with casters, on the big screen.</p></li>
  </ol>`;
}

const initials = s => String(s || '?').replace(/^Bot /, '').split(/\s+/).map(w => w[0]).join('').slice(0, 2).toUpperCase() || '?';

function matchCard(m) {
  const info = GAMES[m.game] || { icon: '🎮', short: m.game };
  const w = m.winner;
  return `<button type="button" class="jx-match ${m.kind === 'official' ? 'official' : ''}" data-match="${esc(m.id)}">
    <div class="jx-match-k"><span>${info.icon} ${esc(info.short)}</span><span>${m.kind === 'official' ? esc(m.stage || 'Official') : 'Practice'}</span></div>
    <div class="jx-match-row ${w === 0 ? 'won' : w === 1 ? 'lost' : ''}"><span>${esc(m.names[0])}</span><b>${m.score[0]}</b></div>
    <div class="jx-match-row ${w === 1 ? 'won' : w === 0 ? 'lost' : ''}"><span>${esc(m.names[1])}</span><b>${m.score[1]}</b></div>
    <div class="jx-match-f"><span>${esc(ago(m.finished || m.started))}</span><span class="jx-play-ic">▶ Watch</span></div></button>`;
}

function windSVG() {
  const lines = Array.from({ length: 9 }, (_, i) => {
    const y = 40 + i * 52, a = 26 + (i % 3) * 14;
    return `<path d="M-200 ${y} C 200 ${y - a}, 500 ${y + a}, 900 ${y} S 1500 ${y - a}, 1900 ${y}" style="animation-delay:${-i * 1.7}s"/>`;
  }).join('');
  return `<svg viewBox="0 0 1600 520" preserveAspectRatio="none">${lines}</svg>`;
}

// ---- Rànquing ----

function paintRanking() {
  const d = S.d, g = S.rankGame;
  let rows = (d.leaderboard || {})[g] || [];
  if (!S.rankBots) rows = rows.filter(r => !r.bot);
  const max = Math.max(1, ...rows.map(r => r.elo)), min = Math.min(max - 1, ...rows.map(r => r.elo));
  $('jxRanking').innerHTML = `<div class="jx-wrap">
    <div class="jx-page-head"><div><div class="jx-kicker">Elo ranking</div><h1 class="jx-h1">Leaderboard</h1><p class="jx-muted">Official matches move the Elo. Practice matches against bots count too, so the ladder is live all week.</p></div></div>
    <div class="jx-toolbar"><div class="jx-tabs">${d.games.map(x => `<button type="button" data-rank="${esc(x.id)}" class="${x.id === g ? 'on' : ''}">${GAMES[x.id]?.icon || ''} ${esc(GAMES[x.id]?.short || x.id)}</button>`).join('')}</div>
      <label class="jx-toggle"><input type="checkbox" id="jxRankBots" ${S.rankBots ? 'checked' : ''}> Show bots</label></div>
    ${rows.length ? `<table class="jx-table"><thead><tr><th>#</th><th>Player</th><th class="r">Elo</th><th class="r">Played</th><th class="r">W</th><th class="r">L</th><th class="r">D</th><th>Win rate</th></tr></thead><tbody>
      ${rows.map((r, i) => { const wr = r.played ? Math.round(100 * r.won / r.played) : 0; return `<tr class="${r.owner === d.me ? 'me' : ''} ${i < 3 ? 'top' + (i + 1) : ''}">
        <td class="pos">${i < 3 ? ['🥇', '🥈', '🥉'][i] : i + 1}</td><td><span class="jx-av sm">${esc(initials(r.display))}</span>${esc(r.display)}${r.bot ? ' <small class="jx-bot">BOT</small>' : ''}${r.owner === d.me ? ' <small class="jx-you">YOU</small>' : ''}</td>
        <td class="r elo"><b>${r.elo}</b><i style="width:${Math.round(12 + 88 * (r.elo - min) / Math.max(1, max - min))}%"></i></td><td class="r">${r.played}</td><td class="r">${r.won}</td><td class="r">${r.lost}</td><td class="r">${r.drawn}</td>
        <td><div class="jx-wr"><i style="width:${wr}%"></i></div><small>${wr}%</small></td></tr>`; }).join('')}</tbody></table>`
      : `<div class="jx-empty">No one is ranked in ${esc(GAMES[g]?.short || g)} yet. <a href="#/play/${esc(g)}">Be the first →</a></div>`}
  </div>`;
}

// ---- Torneig ----

async function paintCup() {
  const d = S.d, comps = d.competitions || [];
  const id = S.arg || (comps.find(c => c.status === 'running') || comps[0] || {}).id;
  const status = s => ({ running: '<span class="jx-pill live">Live</span>', done: '<span class="jx-pill">Finished</span>', ready: '<span class="jx-pill soon">Ready</span>', stopped: '<span class="jx-pill">Stopped</span>' })[s] || '';
  const list = comps.map(c => `<a class="jx-comp ${c.id === id ? 'on' : ''}" href="#/cup/${esc(c.id)}"><b>${esc(c.title)}</b>${status(c.status)}
    <small>${(c.games || []).map(g => GAMES[g]?.icon || g).join(' ')} · ${esc(new Date((c.started || c.created) * 1000).toLocaleDateString('en', { month: 'short', day: 'numeric' }))}</small>
    ${c.champion && Object.keys(c.champion).length ? `<small class="jx-champ">🏆 ${Object.entries(c.champion).map(([g, o]) => esc(displayOf(g, o))).join(', ')}</small>` : ''}</a>`).join('');
  $('jxCup').innerHTML = `<div class="jx-page-head"><div><div class="jx-kicker">Tournament</div><h1 class="jx-h1">Groups &amp; bracket</h1>
      <p class="jx-muted">Groups seeded by Elo, then best-of-three playoffs. Strategies are frozen when the tournament is created and revealed when it ends.</p></div>
      <a class="jx-btn hot" href="${esc(streamURL())}" target="_blank" rel="noopener">📺 Open the broadcast</a></div>
    ${comps.length ? `<div class="jx-cup"><nav class="jx-comps">${list}</nav><div class="jx-cup-main" id="jxCupMain"><div class="jx-muted">Loading…</div></div></div>`
      : `<div class="jx-empty">No tournament yet. ${d.admin ? 'Create one below.' : 'The organizer will announce it soon.'}</div>`}`;
  mountOrganizer();
  if (!id) return;
  try { S.comp = await api('/api/arena/competition?id=' + encodeURIComponent(id)); } catch (e) { $('jxCupMain').textContent = e.message; return; }
  const c = S.comp;
  if (!c.games.includes(S.compGame)) S.compGame = currentGame(c) || c.games[0];
  const main = $('jxCupMain'); if (!main) return;
  main.innerHTML = `<div class="jx-cup-head"><h2 class="jx-h2">${esc(c.title)}</h2>${status(c.status)}
      ${c.games.length > 1 ? `<div class="jx-tabs">${c.games.map(g => `<button type="button" data-cupgame="${esc(g)}" class="${g === S.compGame ? 'on' : ''}">${GAMES[g]?.icon || ''} ${esc(GAMES[g]?.short || g)}</button>`).join('')}</div>` : ''}</div>
    ${c.champion && c.champion[S.compGame] ? `<div class="jx-trophy"><span>🏆</span><div><small>Champion · ${esc(GAMES[S.compGame]?.short || S.compGame)}</small><b>${esc(nameOf(c, c.champion[S.compGame]))}</b></div></div>` : ''}
    <div class="jx-bracket" id="jxBracket"></div>
    ${c.status === 'done' && c.entries && c.entries[S.compGame] ? `<h3 class="jx-h3">The strategies, revealed</h3><div class="jx-entries">${c.entries[S.compGame].map(e => `<button type="button" class="jx-entry" data-entry="${esc(e.owner)}"><span class="jx-av sm">${esc(initials(e.display))}</span>${esc(e.display)}<small>Read .md</small></button>`).join('')}</div>` : ''}`;
  renderCompetition($('jxBracket'), c, { game: S.compGame, lang: 'en' });
}

// ---- Partits ----

function paintMatches() {
  const d = S.d, f = S.matchGame;
  const ms = (d.matches || []).filter(m => f === 'all' || m.game === f || (f === 'official' && m.kind === 'official') || (f === 'mine' && m.owners.includes(d.me)));
  $('jxMatches').innerHTML = `<div class="jx-wrap">
    <div class="jx-page-head"><div><div class="jx-kicker">Replays</div><h1 class="jx-h1">Matches</h1><p class="jx-muted">Every decision, the line of the .md behind it and the prompt moments that swung the game. Open one to watch it with commentary.</p></div></div>
    <div class="jx-toolbar"><div class="jx-tabs">${[['all', 'All'], ['official', 'Official'], ['mine', 'Mine'], ...d.games.map(g => [g.id, (GAMES[g.id]?.icon || '') + ' ' + (GAMES[g.id]?.short || g.id)])].map(([k, l]) => `<button type="button" data-mf="${esc(k)}" class="${k === f ? 'on' : ''}">${esc(l)}</button>`).join('')}</div></div>
    ${ms.length ? `<div class="jx-results grid">${ms.map(matchCard).join('')}</div>` : '<div class="jx-empty">No matches here yet.</div>'}
  </div>`;
}

// ---- Com funciona ----

function paintHow() {
  const d = S.d;
  $('jxHow').innerHTML = `<div class="jx-wrap">
    <div class="jx-page-head"><div><div class="jx-kicker">The rules</div><h1 class="jx-h1">How it works</h1><p class="jx-muted">Two identical teams. One model plays both. The only difference is the strategy file each player wrote.</p></div></div>
    ${stepsHTML()}
    <div class="jx-rules">${d.games.map(g => `<article class="jx-rule jx-g-${esc(g.id)}"><div class="jx-rule-ic">${GAMES[g.id]?.icon || '🎮'}</div><h3>${esc(g.title)}</h3>
      <p><b>You decide:</b> ${esc(GAMES[g.id]?.decide || '')}</p><p><b>To win:</b> ${esc(GAMES[g.id]?.read || '')}</p>
      <details><summary>Full rules the model reads</summary><pre>${esc(g.rules || '')}</pre></details>
      <div class="jx-rule-f"><a class="jx-btn" href="#/play/${esc(g.id)}">Play ${esc(GAMES[g.id]?.short || g.id)}</a><button type="button" class="jx-btn ghost" data-demo="${esc(g.id)}">🎬 Watch a demo</button></div></article>`).join('')}</div>
    <div class="jx-fair"><h3 class="jx-h3">Fair play</h3><ul>
      <li>Same model, same settings, no reasoning tokens, for every team.</li>
      <li>Every decision is checked: ✓ when the rule the model quoted really is in your .md, ⚠ when it improvised.</li>
      <li>Playoffs are best of three, swapping sides. A match where the model failed to decide for a team more than 10% of the time is replayed.</li>
      <li>Official matches hide the counter hints: your .md has to read the matchup on its own.</li>
    </ul></div>
  </div>`;
}

// ---- Clics ----

function onClick(e) {
  const t = e.target.closest('[data-match],[data-rank],[data-mf],[data-cupgame],[data-entry],[data-demo],#jxRankBots');
  if (!t) return;
  if (t.id === 'jxRankBots') { S.rankBots = t.checked; paintRanking(); return; }
  if (t.dataset.match) { openMatch(t.dataset.match); return; }
  if (t.dataset.rank) { S.rankGame = t.dataset.rank; paintRanking(); return; }
  if (t.dataset.mf) { S.matchGame = t.dataset.mf; paintMatches(); return; }
  if (t.dataset.cupgame) { S.compGame = t.dataset.cupgame; paintCup(); return; }
  if (t.dataset.demo) { window.gregalArena?.openTV('demo-' + t.dataset.demo, 2); return; }
  if (t.dataset.entry && S.comp) {
    const en = (S.comp.entries[S.compGame] || []).find(x => x.owner === t.dataset.entry);
    if (en) window.gregalArena?.showMD(en.display, en.md);
  }
}

// Un partit s'obre al reproductor de l'Arena (comentaris, decisions, tele).
async function openMatch(id) {
  if (S.route !== 'play') location.hash = '#/play';
  await new Promise(r => setTimeout(r, 0));
  const ar = window.gregalArena;
  if (!ar) return;
  await ar.openMatch(id);
  $('arMatchCard')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

if (document.documentElement.dataset.product === 'jocs') boot();
