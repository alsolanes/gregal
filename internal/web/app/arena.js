// L'Arena (arena.go): cada jugador escriu un .md d'estratègia per joc i el
// mateix model juga per tots dos equips llegint-lo. La pàgina és un sol camí:
// tria el joc, escriu (o deixa la plantilla, que ja hi és) i prem Juga; el
// rival el tria el servidor (el primer bot que encara no has guanyat). Al
// final del partit, l'informe del coach i «Torna-hi». El rànquing, els bots,
// els partits, les competicions i la retransmissió van plegats a sota.
import { makeView, LESSONS, GUIDE } from './arena-views.js';
import { exportVideo, momentsByEvent, momentKeys, showMomentCard, sideColor, momentCSS } from './arena-export.js';
import { mountScrubber, fmtTime } from './arena-scrubber.js';
import { buildTrack, filterTrack, activeEntry, captionOf, trackItemHTML, summaryHTML, playerCSS, callText } from './arena-player.js';
import { applyFix, undoFix, diffLines, diffHunks, diffCount, matchesByVersion, versionStats, lastPractice, topMistakes, coachScore } from './arena-learn.js';

const $ = id => document.getElementById(id);
// Els jocs van sempre en anglès.
const T = key => (window.gregalTEn ? window.gregalTEn(key) : '') || key;
// Per a la barra de progrés: buit si la clau no hi és (llavors fa servir els seus rètols).
const Tq = key => { const v = T(key); return v === key ? '' : v; };
const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const api = (path, opts) => window.api(path, opts).then(async r => { const j = await r.json().catch(() => ({})); if (!r.ok) throw new Error(j.error || (typeof j === 'string' ? j : r.status)); return j; });
const lang = () => 'en';
const post = (path, body) => api(path, { method: 'POST', body: JSON.stringify(body) });
const RUBRIC = ['clarity', 'coverage', 'adaptivity', 'specificity', 'consistency'];
const ICON = { nfl: '🏈', cs2: '💣', lol: '🐉' };
// Només pantalla tàctil (cap punter fi): el CS2 en directe necessita teclat i ratolí.
const touchOnly = () => { try { return !matchMedia('(any-pointer: fine)').matches && matchMedia('(any-pointer: coarse)').matches; } catch (e) { return false; } };
const n0 = v => Math.round(+v || 0).toLocaleString('en');
const when = at => { try { return new Date(at * 1000).toLocaleString('en', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }); } catch (e) { return String(at); } };
const MATCH_Q = '/api/arena/match?id=';

const arenaUI = {
  data: null, game: 'nfl', view: null, match: null, poll: 0, comp: null, speed: 1,

  async load() {
    const page = $('arenaPage');
    if (!page || typeof window.api !== 'function') return;
    if (!page.dataset.ready) this.build(page);
    this.resume();
    try { this.data = await api('/api/arena'); } catch (e) { $('arStatus').textContent = e.message; return; }
    if (!this.data.games.find(g => g.id === this.game) && this.data.games.length) this.game = this.data.games[0].id;
    this.paint();
    // Mentre els bots s'escriuen, es torna a mirar de tant en tant.
    clearTimeout(this.seedTimer);
    if ((this.data.seeding || []).length) this.seedTimer = setTimeout(() => this.load(), 15000);
  },

  build(page) {
    page.dataset.ready = '1';
    momentCSS(); playerCSS();
    page.innerHTML = `<div class="ar">
      <header class="ar-head"><div><div class="page-kicker">${esc(T('ar.kicker'))}</div><h1 class="page-title">${esc(T('ar.title'))}</h1>
        <p class="page-copy">${esc(T('ar.lead2'))}</p></div>
        <label class="ar-field ar-me">${esc(T('ar.playAs'))} <input id="arDisplay" maxlength="40"></label></header>
      <nav class="ar-games" id="arGames" role="tablist"></nav>
      <span id="arStatus" class="ar-status" role="status"></span>
      <section class="ar-card" id="arMatchCard" hidden>
        <div class="ar-row"><h2 id="arMatchTitle"></h2><span class="ar-muted" id="arMatchStatus"></span>
          <span class="ar-ctl"><button type="button" id="arClose" title="${esc(T('ar.close'))}">✕</button></span></div>
        <div class="ar-match ar-player"><div class="ar-watch"><div class="ar-stage" id="arStage" style="position:relative"><canvas id="arCanvas"></canvas><div class="ar-cc" id="arCC" aria-live="polite"></div><div class="ar-end" id="arEnd" hidden></div></div><div id="arScrub" class="ar-scrub-host"></div><div id="arStats" class="ar-stats" hidden></div></div>
          <div class="ar-side"><div class="ar-tabs" role="tablist"><button type="button" role="tab" data-tab="track" class="on">${esc(T('ar.vp.commentary'))}</button><button type="button" role="tab" data-tab="dec">${esc(T('ar.vp.decisions'))}</button></div>
            <div id="arTrackPane" class="ar-trackpane"><div class="ar-filter" id="arFilter"><button type="button" data-f="all" class="on">${esc(T('ar.vp.all'))}</button><button type="button" data-f="key">${esc(T('ar.vp.key'))}</button><button type="button" data-f="prompt">★ ${esc(T('ar.vp.prompt'))}</button></div><ol class="ar-track" id="arTrack"></ol></div>
            <div class="ar-decisions" id="arDecisions" hidden></div></div></div>
        <div class="ar-feedback" id="arFeedback"></div>
        <div class="ar-row" id="arAgainRow" hidden><button type="button" id="arAgain" class="primary ar-big">${esc(T('ar.playAgain'))}</button><span class="ar-muted">${esc(T('ar.againHint'))}</span></div>
      </section>
      <section class="ar-card ar-play" id="arPlayCard">
        <ol class="ar-steps" id="arSteps"><li>${esc(T('ar.step1'))}</li><li>${esc(T('ar.step2'))}</li><li>${esc(T('ar.step3'))}</li></ol>
        <div class="ar-play-grid">
          <div class="ar-editor">
            <textarea id="arMD" spellcheck="false" placeholder="${esc(T('ar.mdPlaceholder'))}"></textarea>
            <div class="ar-row"><span id="arSaved" class="ar-muted"></span><span id="arCount" class="ar-muted"></span>
              <button type="button" id="arStarter" class="ar-link">${esc(T('ar.resetStarter'))}</button></div>
            <details id="arHist" class="ar-hist"><summary id="arHistSum"></summary><div id="arHistBody"></div></details>
          </div>
          <aside class="ar-tips">
            <section id="arLast" class="ar-last" hidden></section>
            <h3>${esc(T('ar.decides'))}</h3><div id="arGuide" class="ar-guide"></div>
            <details><summary>${esc(T('ar.tips'))}</summary><ol id="arLessons" class="ar-lessons"></ol></details>
          </aside>
        </div>
        <div class="ar-go">
          <button type="button" id="arPlay" class="primary ar-big"></button>
          <span class="ar-muted" id="arPlayHint"></span>
          <button type="button" id="arDemo" class="ar-demo">🎬 ${esc(T('ar.demo'))}</button>
          <button type="button" id="arLive" class="ar-demo" title="${esc(T('ar.playLiveHint'))}" hidden>🎮 ${esc(T('ar.playLive'))}</button>
          <span id="arLiveTouch" class="ar-muted" hidden>${esc(T('ar.liveTouch'))}</span>
          <button type="button" id="arReview" class="ar-link">${esc(T('ar.askTips'))}</button>
        </div>
        <div id="arVoiceNote" class="ar-voice-note" hidden></div>
        <div id="arReviewOut"></div>
        <div id="arProgress" class="ar-progress"></div>
      </section>
      <details class="ar-card ar-fold"><summary>${esc(T('ar.foldBoard'))}</summary>
        <div class="ar-grid2"><div><h3>${esc(T('ar.leaderboard'))}</h3><ol id="arBoard" class="ar-board"></ol></div>
        <div><h3>${esc(T('ar.bots'))}</h3><ul id="arBots" class="ar-bots"></ul></div></div></details>
      <details class="ar-card ar-fold"><summary>${esc(T('ar.foldMatches'))}</summary><ul id="arMatches" class="ar-list"></ul></details>
      <details class="ar-card ar-fold"><summary>${esc(T('ar.foldComps'))}</summary>
        <div class="ar-row"><a id="arStreamLink" target="_blank" rel="noopener">${esc(T('ar.openStream'))} ↗</a></div>
        <div id="arCompAdmin"></div><ul id="arComps" class="ar-list"></ul><div id="arComp"></div>
        <section id="arBroadcast" class="ar-broadcast" hidden><h3>${esc(T('ar.broadcast'))}</h3>
          <div class="ar-row"><b>${esc(T('ar.share'))}</b><span class="ar-muted">${esc(T('ar.shareHelp'))}</span></div>
          <div class="ar-row"><input id="arShareLink" readonly style="flex:1;min-width:240px"><button type="button" id="arShareCopy">${esc(T('ar.copy'))}</button><button type="button" id="arShareNew">${esc(T('ar.renew'))}</button></div>
          <p class="ar-muted">${esc(T('ar.obsHelp'))}</p>
          <div class="ar-row"><b>${esc(T('ar.voice'))}</b><span class="ar-muted" id="arVoiceState"></span></div>
          <div class="ar-row"><label>${esc(T('ar.voiceEngine'))} <select id="arVoiceEngine"><option value="piper">${esc(T('ar.voicePiper'))}</option><option value="http">${esc(T('ar.voiceHttp'))}</option><option value="off">${esc(T('ar.voiceOff'))}</option></select></label>
            <button type="button" id="arVoiceInstall" class="primary"></button><button type="button" id="arVoiceTestC">${esc(T('ar.testCaster'))}</button><button type="button" id="arVoiceTestA">${esc(T('ar.testAnalyst'))}</button></div>
          <div class="ar-row" id="arVoiceHttp" hidden><input id="arVoiceURL" placeholder="http://localhost:8004" style="flex:1"><input id="arVoiceModel" placeholder="model" style="width:140px"><input id="arVoiceC" placeholder="${esc(T('ar.voiceCasterName'))}" style="width:150px"><input id="arVoiceA" placeholder="${esc(T('ar.voiceAnalystName'))}" style="width:150px"><button type="button" id="arVoiceSave">${esc(T('ar.save'))}</button></div>
          <p class="ar-muted">${esc(T('ar.voiceHelp'))}</p>
        </section></details>
    </div>`;
    const md = $('arMD');
    md.addEventListener('input', () => {
      $('arCount').textContent = md.value.length + ' / 12000';
      $('arSaved').textContent = '';
      clearTimeout(this.saveTimer); this.saveTimer = setTimeout(() => this.save(), 1500);
    });
    $('arDisplay').addEventListener('change', () => this.save(true));
    $('arStarter').onclick = () => {
      const g = this.data.games.find(x => x.id === this.game);
      if (g && (!md.value.trim() || md.value === g.starter || confirm(T('ar.replaceConfirm')))) { md.value = g.starter; md.dispatchEvent(new Event('input')); }
    };
    $('arReview').onclick = async () => {
      $('arReview').disabled = true; $('arReviewOut').innerHTML = `<p class="ar-muted">${esc(T('ar.reviewing'))}</p>`;
      try { const f = await post('/api/arena', { action: 'review', game: this.game, md: md.value }); $('arReviewOut').innerHTML = this.feedbackHTML(f, T('ar.reviewTitle')); }
      catch (e) { $('arReviewOut').textContent = e.message; }
      $('arReview').disabled = false;
    };
    $('arPlay').onclick = () => this.play('');
    // Un partit de demostració (gravat, amb comentaris), a la tele i a 2×.
    $('arDemo').onclick = () => this.openTV('demo-' + this.game, 2);
    // El CS2 en directe: la persona porta un jugador (arena-cs2-live.html).
    $('arLive').onclick = async () => { clearTimeout(this.saveTimer); await this.save(); this.openLive(); };
    $('arAgain').onclick = () => { this.closeMatch(); $('arPlayCard').scrollIntoView({ behavior: 'smooth', block: 'start' }); md.focus(); };
    $('arClose').onclick = () => this.closeMatch();
    // Les decisions: un sol escoltador (les files s'afegeixen a mesura que arriben).
    $('arDecisions').onclick = e => {
      const el = e.target.closest('.ar-dec'); if (!el || !this.match) return;
      this.showSituation(this.match.id, +el.dataset.n, +el.dataset.side, this.match.names[+el.dataset.side]);
    };
    // El pas 3 porta a la targeta de l'últim partit.
    $('arSteps').onclick = e => { if (e.target.closest('[data-go="last"]')) this.focusLast(); };
    $('arLast').onclick = e => this.lastClick(e);
    $('arHist').addEventListener('toggle', () => { if ($('arHist').open) this.paintHistory(); });
    $('arHistBody').onclick = e => this.histClick(e);
    $('arHistBody').onchange = () => this.paintCompare();
    // Fora de la vista de l'Arena (o tancant el partit) no es consulta res.
    new MutationObserver(() => { if (page.hidden) this.suspend(); }).observe(page, { attributes: true, attributeFilter: ['hidden'] });
    // El reproductor: pestanyes (comentaris | decisions), el filtre de la pista i clic a una línia per anar-hi.
    page.querySelectorAll('.ar-tabs [data-tab]').forEach(b => b.onclick = () => {
      page.querySelectorAll('.ar-tabs [data-tab]').forEach(x => x.classList.toggle('on', x === b));
      $('arTrackPane').hidden = b.dataset.tab !== 'track'; $('arDecisions').hidden = b.dataset.tab !== 'dec';
    });
    $('arFilter').querySelectorAll('[data-f]').forEach(b => b.onclick = () => {
      this.trackMode = b.dataset.f; $('arFilter').querySelectorAll('[data-f]').forEach(x => x.classList.toggle('on', x === b));
      this.trackKey = ''; this.paintTrack();
    });
    $('arTrack').onclick = e => { const li = e.target.closest('[data-i]'); if (li && this.scrub) this.scrub.seek(+li.dataset.i); };
    $('arEnd').onclick = e => {
      const b = e.target.closest('[data-end]'), mo = e.target.closest('[data-seek]');
      if (mo && this.scrub) { this.endShown = 'closed'; $('arEnd').hidden = true; this.scrub.seek(+mo.dataset.seek); return; }
      if (!b) return;
      if (b.dataset.end === 'again') { this.endShown = ''; $('arEnd').hidden = true; if (this.scrub) this.scrub.seek(0); if (this.view && this.view.paused) this.view.togglePause(); }
      else if (b.dataset.end === 'download') this.match && this.exportVideo(this.match.id);
      else { this.endShown = 'closed'; $('arEnd').hidden = true; }
    };
    this.cc = (() => { try { return localStorage.getItem('arenaCC') !== '0'; } catch (e) { return true; } })();
    this.trackMode = 'all';
    addEventListener('message', e => {
      if (e.origin !== location.origin || !e.data) return;
      if (e.data.arenaTV === 'close') this.closeTV();
      if (e.data.arenaOpen) { this.closeTV(); this.load(); this.openMatch(e.data.arenaOpen); }
    });
    this.buildBroadcast();
    $('arStreamLink').href = '/app/arena-stream.html#lang=' + lang() + (() => { let t = ''; try { t = localStorage.getItem('gregal_token') || ''; } catch (e) {} return t ? '&token=' + encodeURIComponent(t) : ''; })();
  },

  // Un clic: el servidor desa el .md que hi ha a l'editor i tria el rival.
  async play(opponent) {
    $('arPlay').disabled = true; clearTimeout(this.saveTimer);
    try {
      const r = await post('/api/arena', { action: 'practice', game: this.game, md: $('arMD').value, display: $('arDisplay').value, opponent });
      this.openMatch(r.id); this.load();
    } catch (e) { $('arStatus').textContent = e.message; }
    $('arPlay').disabled = false;
  },

  // Desa sol, en deixar d'escriure o en canviar el nom.
  async save(force) {
    const md = $('arMD').value.trim(), mine = this.data && this.data.mine[this.game];
    if (!md || (!force && mine && mine.md === md)) return;
    $('arSaved').textContent = T('ar.saving');
    try {
      await post('/api/arena', { action: 'save', game: this.game, md, display: $('arDisplay').value });
      if (this.data) this.data.mine[this.game] = { ...(mine || {}), md, display: $('arDisplay').value };
      $('arSaved').textContent = T('ar.autosaved');
    } catch (e) { $('arSaved').textContent = e.message; }
  },

  paint() {
    const d = this.data, g = d.games.find(x => x.id === this.game);
    const seeding = (d.seeding || []).includes(this.game);
    $('arGames').innerHTML = d.games.map(x => `<button type="button" role="tab" data-g="${esc(x.id)}" class="${x.id === this.game ? 'on' : ''}">
      <span class="ic">${ICON[x.id] || '🎮'}</span><b>${esc(x.title)}</b><small>${esc(T('ar.game.' + x.id))}</small></button>`).join('');
    $('arGames').querySelectorAll('[data-g]').forEach(b => b.onclick = () => { this.save(); this.game = b.dataset.g; $('arReviewOut').innerHTML = ''; this.paint(); });
    const mine = d.mine[this.game];
    if (document.activeElement !== $('arMD')) {
      $('arMD').value = mine ? mine.md : (g ? g.starter : '');
      $('arCount').textContent = $('arMD').value.length + ' / 12000';
      $('arSaved').textContent = mine ? T('ar.autosaved') : T('ar.isStarter');
    }
    if (document.activeElement !== $('arDisplay')) $('arDisplay').value = mine && mine.display && mine.display !== 'local' ? mine.display : (d.display || d.me);
    const next = (d.next || {})[this.game];
    // Les partides de prova que queden: no hi és si no compta (l'administrador no en té límit).
    const limited = !d.admin && typeof d.practice_left === 'number';
    const left = limited ? Math.max(0, d.practice_left) : Infinity;
    $('arPlay').textContent = T('ar.playVs').replace('{name}', next ? next.display : 'Bot');
    $('arPlay').disabled = left <= 0;
    $('arPlayHint').textContent = [seeding ? T('ar.seedingNow') : '', next ? T('ar.level.' + next.level) : '',
      limited ? (left > 0 ? T('ar.practiceLeft').replace('{n}', left) : T('ar.practiceNone')) : ''].filter(Boolean).join(' · ');
    // El CS2 en directe es juga amb teclat i ratolí: a una pantalla només tàctil, apagat i amb el perquè.
    const touch = touchOnly();
    $('arLive').hidden = this.game !== 'cs2';
    $('arLive').disabled = touch;
    $('arLiveTouch').hidden = this.game !== 'cs2' || !touch;
    // El pas on som: 1 escriure, 2 jugar, 3 llegir l'informe i millorar.
    const last = lastPractice(((d.progress || {})[this.game]) || []);
    const played = !!last || (d.matches || []).some(m => m.game === this.game && m.kind === 'practice' && m.status === 'final');
    const step = !mine ? 0 : played ? 2 : 1;
    $('arSteps').querySelectorAll('li').forEach((li, i) => {
      li.classList.toggle('on', i === step); li.classList.toggle('done', i < step);
      // El pas 3 assenyala la targeta de l'últim partit (al costat de l'editor).
      if (i === 2) li.innerHTML = last ? `<button type="button" class="ar-link" data-go="last">${esc(T('ar.step3'))} →</button>` : esc(T('ar.step3'));
    });
    this.paintLast(last);
    this.paintHistSummary();
    const gd = GUIDE[this.game];
    $('arGuide').innerHTML = gd ? `<dl>${gd.decides.map(([w, o]) => `<dt>${esc(w)}</dt><dd>${esc(o)}</dd>`).join('')}</dl>
      <p class="ar-muted"><b>${esc(T('ar.sees'))}</b> ${esc(gd.sees)}</p>
      <div class="ar-ex"><b>${esc(T('ar.example'))}</b><pre>${esc(gd.example)}</pre></div>` : '';
    this.paintProgress(d);
    $('arLessons').innerHTML = LESSONS[lang()].slice(0, 5).map(([h, p]) => `<li><b>${esc(h)}</b><span>${esc(p)}</span></li>`).join('');
    const bots = d.bots[this.game] || [];
    const board = d.leaderboard[this.game] || [];
    $('arBoard').innerHTML = board.map((r, i) => `<li class="${r.owner === d.me ? 'me' : ''}"><span class="pos">${i + 1}</span><span class="who">${esc(r.display)}${r.bot ? ' 🤖' : ''}</span><b>${r.elo}</b><small>${r.won}-${r.lost}${r.drawn ? '-' + r.drawn : ''}</small></li>`).join('') || `<li class="none">${esc(T('ar.noRanking'))}</li>`;
    $('arBots').innerHTML = bots.map(b => `<li><b>${esc(b.display)}</b> <small>${esc(T('ar.level.' + b.level))}</small><button type="button" data-play="${esc(b.owner)}">▶ ${esc(T('ar.practice'))}</button><button type="button" data-md="${esc(b.owner)}">${esc(T('ar.readMD'))}</button></li>`).join('') +
      (d.admin && bots.length ? `<li><button type="button" id="arSeed">${esc(T('ar.reseed'))}</button></li>` : '') || `<li class="none">${esc(T(seeding ? 'ar.seeding' : 'ar.noBots'))}</li>`;
    $('arBots').querySelectorAll('[data-md]').forEach(b => b.onclick = () => { const bot = bots.find(x => x.owner === b.dataset.md); this.showMD(bot.display, bot.md); });
    $('arBots').querySelectorAll('[data-play]').forEach(b => b.onclick = () => this.play(b.dataset.play));
    if ($('arSeed')) $('arSeed').onclick = async () => { $('arSeed').disabled = true; await post('/api/arena', { action: 'seed_bots', games: [this.game] }).catch(e => { $('arStatus').textContent = e.message; }); $('arStatus').textContent = T('ar.seeding'); setTimeout(() => this.load(), 20000); };
    $('arMatches').innerHTML = (d.matches || []).filter(m => m.game === this.game).map(m => `<li data-id="${esc(m.id)}"><b>${esc(m.names[0])} ${m.score[0]} - ${m.score[1]} ${esc(m.names[1])}</b><small>${esc(T('ar.kind.' + m.kind))}${m.stage && m.kind === 'official' ? ' · ' + esc(m.stage) : ''} · ${esc(T('ar.status.' + m.status))}</small></li>`).join('') || `<li class="none">${esc(T('ar.noMatches'))}</li>`;
    $('arMatches').querySelectorAll('[data-id]').forEach(li => li.onclick = () => this.openMatch(li.dataset.id));
    // Competicions i retransmissió (l'organitzador).
    $('arBroadcast').hidden = !d.admin;
    if (d.admin && !this.broadcastLoaded) { this.broadcastLoaded = true; this.share(false); }
    if (d.admin) this.voiceStatus();
    $('arCompAdmin').innerHTML = d.admin ? `<div class="ar-row ar-actions"><input id="arCompTitle" placeholder="${esc(T('ar.compTitle'))}"><label><input type="checkbox" id="arCompBots" checked> ${esc(T('ar.withBots'))}</label><button type="button" id="arCompCreate">${esc(T('ar.compCreate'))}</button></div>` : '';
    if ($('arCompCreate')) $('arCompCreate').onclick = async () => {
      try { const r = await post('/api/arena/competition', { action: 'create', title: $('arCompTitle').value, bots: $('arCompBots').checked }); this.openComp(r.id); this.load(); }
      catch (e) { $('arStatus').textContent = e.message; }
    };
    $('arComps').innerHTML = (d.competitions || []).map(c => `<li data-c="${esc(c.id)}"><b>${esc(c.title)}</b><small>${esc(T('ar.compStatus.' + c.status))} · ${esc((c.games || []).join(', '))}</small></li>`).join('') || `<li class="none">${esc(T('ar.noComps'))}</li>`;
    $('arComps').querySelectorAll('[data-c]').forEach(li => li.onclick = () => this.openComp(li.dataset.c));
  },

  // Com evoluciones: cada partit acabat és una barra (verda si guanyes, vermella
  // si perds, alçada = diferència del marcador) i la línia és la nota del coach
  // al teu .md (0-25). Si el .md millora, la línia puja. Cada partit té un
  // espai fix (el gràfic creix amb els partits): amb 1 o 2 no queda un llenç
  // buit amb una barra perduda al mig.
  paintProgress(d) {
    const rows = ((d.progress || {})[this.game]) || [];
    const box = $('arProgress');
    if (!rows.length) { box.innerHTML = `<h3>${esc(T('ar.progress'))}</h3><p class="ar-muted">${esc(T('ar.progressNone'))}</p>`; return; }
    const n = rows.length, slot = 56, padL = 30, padR = 14, H = 150, W = padL + padR + n * slot, bw = 30;
    const x = i => padL + slot * (i + .5);
    const maxDiff = Math.max(1, ...rows.map(r => Math.abs(r.score[0] - r.score[1])));
    const mid = H * .55, cy = v => mid - (v / 25 - .5) * H * .8;
    const bars = rows.map((r, i) => {
      const diff = r.score[0] - r.score[1], h = Math.max(4, Math.abs(diff) / maxDiff * (H * .38));
      const col = r.result === 'win' ? '#2fbf71' : r.result === 'loss' ? '#e5484d' : '#9aa4bd';
      return `<g><title>${esc(r.opponent)} · ${r.score[0]}-${r.score[1]}${r.coach ? ' · coach ' + r.coach + '/25' : ''}</title>
        <rect x="${x(i) - bw / 2}" y="${diff >= 0 ? mid - h : mid}" width="${bw}" height="${h}" rx="4" fill="${col}" opacity=".85"/>
        <text x="${x(i)}" y="${H - 4}" text-anchor="middle" font-size="10" fill="currentColor" opacity=".7">${r.score[0]}-${r.score[1]}</text></g>`;
    }).join('');
    const pts = rows.map((r, i) => r.coach ? [x(i), cy(r.coach), r.coach] : null).filter(Boolean);
    const line = pts.length > 1 ? `<polyline points="${pts.map(p => p[0] + ',' + p[1]).join(' ')}" fill="none" stroke="var(--accent)" stroke-width="2.5"/>` : '';
    const dots = pts.map(p => `<circle cx="${p[0]}" cy="${p[1]}" r="4" fill="var(--accent)"/>${pts.length < 3 ? `<text x="${p[0] + 7}" y="${p[1] - 6}" font-size="10" fill="var(--accent)">${p[2]}</text>` : ''}`).join('');
    // L'escala de la nota del coach, a l'esquerra.
    const axis = [0, 25].map(v => `<text x="${padL - 6}" y="${cy(v) + 3}" text-anchor="end" font-size="9" fill="currentColor" opacity=".5">${v}</text>`).join('');
    const wins = rows.filter(r => r.result === 'win').length;
    const coached = rows.filter(r => r.coach);
    const trend = coached.length > 1 ? `${coached[0].coach} → ${coached[coached.length - 1].coach}/25` : coached.length ? `${coached[0].coach}/25` : '—';
    box.innerHTML = `<div class="ar-row"><h3>${esc(T('ar.progress'))}</h3><span class="ar-muted">${esc(T('ar.progressSum').replace('{w}', wins).replace('{n}', n).replace('{c}', trend))}</span></div>
      <div class="ar-chart-row"><svg viewBox="0 0 ${W} ${H}" width="${W}" height="${H}" class="ar-chart" role="img"><line x1="${padL}" x2="${W - padR}" y1="${mid}" y2="${mid}" stroke="currentColor" opacity=".2"/>${axis}${bars}${line}${dots}</svg>
      ${n < 3 ? `<p class="ar-muted ar-chart-few">${esc(T('ar.progressFew'))}</p>` : ''}</div>
      <p class="ar-muted">${esc(T('ar.progressHelp'))}</p>`;
  },

  buildBroadcast() {
    const lk = key => location.origin + '/app/arena-stream.html#spectator=' + encodeURIComponent(key) + '&lang=' + lang();
    const share = async reset => { try { const r = await post('/api/arena/competition', { action: 'spectator', title: reset ? 'reset' : '' }); $('arShareLink').value = lk(r.key); } catch (e) { $('arStatus').textContent = e.message; } };
    $('arShareCopy').onclick = () => navigator.clipboard.writeText($('arShareLink').value).then(() => { $('arStatus').textContent = T('ar.copied'); });
    $('arShareNew').onclick = () => { if (confirm(T('ar.renewConfirm'))) share(true); };
    this.share = share;
    const play = async who => {
      try {
        const r = await post('/api/arena/voice', { action: 'test', who, lang: lang() });
        const a = await window.api('/api/arena/voice?k=' + r.key);
        new Audio(URL.createObjectURL(await a.blob())).play();
      } catch (e) { $('arStatus').textContent = e.message; }
    };
    $('arVoiceTestC').onclick = () => play('caster');
    $('arVoiceTestA').onclick = () => play('analyst');
    $('arVoiceInstall').onclick = async () => { await post('/api/arena/voice', { action: 'install', lang: lang() }).catch(e => { $('arStatus').textContent = e.message; }); this.voiceStatus(); };
    $('arVoiceEngine').onchange = () => { $('arVoiceHttp').hidden = $('arVoiceEngine').value !== 'http'; if ($('arVoiceEngine').value !== 'http') this.saveVoice(); };
    $('arVoiceSave').onclick = () => this.saveVoice();
  },

  async saveVoice() {
    const config = { engine: $('arVoiceEngine').value, url: $('arVoiceURL').value.trim(), model: $('arVoiceModel').value.trim(), voices: { caster: $('arVoiceC').value.trim(), analyst: $('arVoiceA').value.trim() } };
    try { await post('/api/arena/voice', { action: 'config', config }); $('arStatus').textContent = T('ar.saved'); } catch (e) { $('arStatus').textContent = e.message; }
    this.voiceStatus();
  },

  async voiceStatus() {
    let v; try { v = await api('/api/arena/voice'); } catch (e) { return; }
    const l = lang(), inst = v.install || {};
    if (document.activeElement !== $('arVoiceEngine')) $('arVoiceEngine').value = v.config.engine || 'piper';
    $('arVoiceHttp').hidden = $('arVoiceEngine').value !== 'http';
    if (v.config.url && !$('arVoiceURL').value) { $('arVoiceURL').value = v.config.url; $('arVoiceModel').value = v.config.model || ''; $('arVoiceC').value = (v.config.voices || {}).caster || ''; $('arVoiceA').value = (v.config.voices || {}).analyst || ''; }
    const piper = $('arVoiceEngine').value === 'piper';
    $('arVoiceInstall').hidden = !piper || v.ready[l];
    $('arVoiceInstall').textContent = T('ar.voiceInstall').replace('{size}', (v.sizes || {})[l] || '');
    $('arVoiceInstall').disabled = inst.state === 'installing';
    let st = '';
    if (inst.state === 'installing') st = T('ar.voiceInstalling').replace('{step}', inst.step || '…').replace('{pct}', inst.total > 0 ? Math.round(inst.done * 100 / inst.total) + '%' : Math.round(inst.done / 1e6) + ' MB');
    else if (inst.state === 'error') st = '⚠ ' + inst.error;
    else if (piper) st = v.ready[l] ? T('ar.voiceReady') : (v.platform ? T('ar.voiceMissing') : T('ar.voiceNoPlatform'));
    else if (v.config.engine === 'http') st = v.config.url ? T('ar.voiceHttpReady') : T('ar.voiceHttpMissing');
    else st = T('ar.voiceDisabled');
    $('arVoiceState').textContent = st;
    // Al costat de Juga: si el narrador no té veu en anglès, dir-ho i
    // instal·lar-la amb un clic (els jocs van en anglès).
    const note = $('arVoiceNote');
    if (inst.state === 'installing') { note.hidden = false; note.innerHTML = `🔊 ${esc(st)}`; }
    else if (v.config.engine !== 'off' && !v.ready[l]) {
      note.hidden = false;
      note.innerHTML = `🔇 ${esc(T('ar.noVoice'))} <button type="button" class="primary">${esc(T('ar.voiceInstall').replace('{size}', (v.sizes || {})[l] || ''))}</button>${inst.state === 'error' ? ` <span class="ar-muted">⚠ ${esc(inst.error)}</span>` : ''}`;
      note.querySelector('button').onclick = () => $('arVoiceInstall').click();
    } else note.hidden = true;
    $('arVoiceTestC').disabled = $('arVoiceTestA').disabled = !(v.ready[l]);
    clearTimeout(this.voiceTimer);
    if (inst.state === 'installing') this.voiceTimer = setTimeout(() => this.voiceStatus(), 1500);
  },

  // ---- de «escriure» a «guanyar»: l'últim partit i les versions del .md ----

  // L'últim partit de pràctica, al costat de l'editor (el pas 3 hi porta): el
  // resultat, la nota del coach i els 2-3 errors amb la regla reescrita, cada
  // un amb «Aplica»: la posa a l'editor, ensenya què ha canviat i es pot desfer.
  paintLast(last) {
    const box = $('arLast');
    if (!last) { box.hidden = true; this.lastId = ''; clearTimeout(this.lastTimer); return; }
    box.hidden = false;
    const c = this.lastCache && this.lastCache.id === last.id ? this.lastCache : null;
    this.lastRow = last;
    if (!c && this.lastId !== last.id) { this.lastId = last.id; this.lastTries = 0; this.loadLast(last.id); }
    this.renderLast();
  },
  async loadLast(id) {
    clearTimeout(this.lastTimer);
    let m;
    try { m = await api(MATCH_Q + encodeURIComponent(id) + '&after=999999&dafter=999999&lafter=999999'); } catch (e) { return; }
    if (this.lastId !== id) return;
    const me = this.data && this.data.me, side = (m.owners || []).indexOf(me);
    const fb = (m.feedback || [])[side] || (side < 0 ? (m.feedback || []).find(Boolean) : null) || null;
    this.lastCache = { id, fb };
    this.renderLast();
    // L'informe del coach arriba una mica després del final: es torna a mirar (un temps).
    if (!fb && !this.suspended && ++this.lastTries < 20) this.lastTimer = setTimeout(() => this.loadLast(id), 10000);
  },
  renderLast() {
    const box = $('arLast'), r = this.lastRow; if (!r) return;
    const c = this.lastCache && this.lastCache.id === r.id ? this.lastCache : null, fb = c && c.fb;
    const res = r.result === 'win' ? 'win' : r.result === 'loss' ? 'loss' : 'draw';
    const coach = fb ? coachScore(fb) : r.coach;
    const mist = fb ? topMistakes(fb, 3) : [];
    this.lastMistakes = mist;
    this.fixes = this.fixes || {};
    const fixHTML = (mo, i) => {
      const ch = this.fixes[r.id + ':' + i];
      const applied = ch ? `<div class="ar-applied"><span>${esc(T(ch.kind === 'replace' ? 'ar.fix.replaced' : 'ar.fix.added').replace('{n}', ch.start + 1))}</span>
        <pre>${ch.old.map(l => `<del>- ${esc(l)}</del>`).join('')}${ch.lines.map(l => `<ins>+ ${esc(l)}</ins>`).join('')}</pre>
        <button type="button" data-undo="${i}">${esc(T('ar.fix.undo'))}</button></div>` : '';
      return `<li><b>${esc(mo.moment || '')}</b> ${esc(mo.what || '')}${mo.md_issue ? `<div class="ar-issue">${esc(mo.md_issue)}</div>` : ''}
        ${mo.fix ? `<div class="ar-fix">✍ ${esc(mo.fix)}</div>${ch ? '' : `<button type="button" class="primary" data-fix="${i}">${esc(T('ar.fix.apply'))}</button>`}<span class="ar-muted" data-note="${i}"></span>` : ''}${applied}</li>`;
    };
    box.innerHTML = `<div class="ar-row"><h3>${esc(T('ar.last.title'))}</h3><span class="ar-res ${res}">${esc(T('ar.last.' + res))} ${r.score[0]}-${r.score[1]}</span></div>
      <p class="ar-last-sub">${esc(T('ar.last.vs').replace('{name}', r.opponent || ''))}${coach ? ' · ' + esc(T('ar.last.coach').replace('{n}', coach)) : ''} · <button type="button" class="ar-link" data-open="${esc(r.id)}">${esc(T('ar.last.watch'))}</button></p>` +
      (!c ? `<p class="ar-muted">${esc(T('ar.hist.loading'))}</p>` : !fb ? `<p class="ar-muted">${esc(T('ar.last.pending'))}</p>` :
        mist.length ? `<h4>${esc(T('ar.last.fixes'))}</h4><ol class="ar-fixes">${mist.map(fixHTML).join('')}</ol>` : `<p class="ar-muted">${esc(T('ar.last.none'))}</p>`);
  },
  lastClick(e) {
    const b = e.target.closest('button'); if (!b) return;
    if (b.dataset.open) { this.openMatch(b.dataset.open); return; }
    const md = $('arMD'), r = this.lastRow;
    if (b.dataset.fix != null) {
      const mo = this.lastMistakes[+b.dataset.fix]; if (!mo || !r) return;
      const res = applyFix(md.value, mo.fix, { issue: mo.md_issue || '' });
      if (res.change.kind === 'none') { const n = $('arLast').querySelector(`[data-note="${b.dataset.fix}"]`); if (n) n.textContent = T('ar.fix.already'); return; }
      md.value = res.md; md.dispatchEvent(new Event('input'));
      this.fixes[r.id + ':' + b.dataset.fix] = res.change;
      this.renderLast();
      // Les línies de la regla (sense el títol «## Coach fixes» si s'ha creat).
      const k = res.change.lines.filter(l => l.trim() && !/^#/.test(l.trim())).length || res.change.lines.length;
      this.selectLines(res.change.start + res.change.lines.length - k, k);
    } else if (b.dataset.undo != null) {
      const key = r.id + ':' + b.dataset.undo, ch = this.fixes[key];
      const back = undoFix(md.value, ch);
      if (back == null) { const n = b.parentElement.querySelector('span'); if (n) n.textContent = T('ar.fix.cantUndo'); return; }
      md.value = back; md.dispatchEvent(new Event('input'));
      delete this.fixes[key];
      this.renderLast();
    }
  },
  // Selecciona unes línies de l'editor (el canvi que s'acaba d'aplicar).
  selectLines(start, count) {
    const md = $('arMD'), lines = md.value.split('\n');
    let a = 0; for (let i = 0; i < start && i < lines.length; i++) a += lines[i].length + 1;
    const b = a + lines.slice(start, start + count).join('\n').length;
    try { md.focus({ preventScroll: true }); md.setSelectionRange(a, b); } catch (e) { /* res */ }
    md.scrollTop = Math.max(0, start / Math.max(1, lines.length) * md.scrollHeight - md.clientHeight / 3);
  },
  focusLast() {
    const box = $('arLast'); if (box.hidden) return;
    box.scrollIntoView({ behavior: 'smooth', block: 'center' });
    box.classList.remove('ar-flash'); void box.offsetWidth; box.classList.add('ar-flash');
  },

  // Les versions del .md (la llista «history» de GET /api/arena).
  history() { const mine = this.data && this.data.mine[this.game]; return (mine && mine.history) || []; },
  paintHistSummary() {
    const h = this.history();
    $('arHist').hidden = !h.length;
    $('arHistSum').textContent = T('ar.hist.title').replace('{n}', h.length);
    if ($('arHist').open) this.paintHistory();
  },
  async versionText(at) {
    this.verText = this.verText || new Map();
    const k = this.game + ':' + at;
    if (!this.verText.has(k)) this.verText.set(k, post('/api/arena', { action: 'version', game: this.game, at }).then(r => r.md || '').catch(e => { this.verText.delete(k); throw e; }));
    return this.verText.get(k);
  },
  // La llista (la més nova a dalt): quan, el hash, la mida i els partits
  // jugats amb cada una (pel temps); tria A i B per comparar-les.
  paintHistory() {
    const h = this.history(), body = $('arHistBody');
    if (!h.length) { body.innerHTML = ''; return; }
    const games = matchesByVersion(h, ((this.data.progress || {})[this.game]) || []);
    const sel = this.histSel && this.histSel.game === this.game && this.histSel.n === h.length ? this.histSel : (this.histSel = { game: this.game, n: h.length, a: Math.max(0, h.length - 2), b: h.length - 1 });
    const rows = h.map((v, i) => ({ v, i, st: versionStats(games[i]) })).reverse();
    body.innerHTML = (h.length < 2 ? `<p class="ar-muted">${esc(T('ar.hist.none'))}</p>` : `<p class="ar-muted">${esc(T('ar.hist.help'))}</p>`) +
      `<table class="ar-hist-t"><thead><tr>${h.length > 1 ? '<th>A</th><th>B</th>' : ''}<th>${esc(T('ar.hist.saved'))}</th><th>${esc(T('ar.hist.version'))}</th><th>${esc(T('ar.hist.results'))}</th><th></th></tr></thead><tbody>` +
      rows.map(({ v, i, st }) => `<tr${v.current ? ' class="cur"' : ''}>${h.length > 1 ? `<td><input type="radio" name="arHA" value="${i}" aria-label="A"${i === sel.a ? ' checked' : ''}></td><td><input type="radio" name="arHB" value="${i}" aria-label="B"${i === sel.b ? ' checked' : ''}></td>` : ''}
        <td>${esc(when(v.at))}${v.current ? ` <span class="ar-badge">${esc(T('ar.hist.current'))}</span>` : ''}</td><td><code>#${esc(v.hash)}</code> <small>${n0(v.size)} B</small></td>
        <td>${esc(this.statText(st))}</td><td>${v.current ? '' : `<button type="button" class="ar-link" data-restore="${i}">${esc(T('ar.hist.restore'))}</button>`}</td></tr>`).join('') +
      `</tbody></table><div id="arHistCmp" class="ar-hist-cmp"></div>`;
    if (h.length > 1) this.paintCompare();
  },
  statText(st) {
    if (!st.n) return T('ar.hist.noGames');
    return `${st.n === 1 ? T('ar.hist.game1') : T('ar.hist.games').replace('{n}', st.n)} · ${st.w}-${st.l}-${st.d}${st.coach != null ? ' · ' + T('ar.last.coach').replace('{n}', st.coach) : ''}`;
  },
  // A → B: què ha canviat al .md i com han anat els partits de cada una.
  async paintCompare() {
    const box = $('arHistCmp'), h = this.history(); if (!box || h.length < 2) return;
    const pick = name => { const r = $('arHistBody').querySelector(`input[name="${name}"]:checked`); return r ? +r.value : -1; };
    const a = pick('arHA'), b = pick('arHB');
    if (a < 0 || b < 0) return;
    this.histSel = { ...this.histSel, a, b };
    const games = matchesByVersion(h, ((this.data.progress || {})[this.game]) || []);
    const sa = versionStats(games[a]), sb = versionStats(games[b]);
    const rate = s => (s.n ? Math.round(s.w * 100 / s.n) + '%' : '—');
    const dc = sa.coach != null && sb.coach != null ? Math.round((sb.coach - sa.coach) * 10) / 10 : null;
    const head = `<div class="ar-hist-res"><span><b>A</b> ${esc(this.statText(sa))} · ${esc(T('ar.hist.winRate'))} ${rate(sa)}</span><span>→</span><span><b>B</b> ${esc(this.statText(sb))} · ${esc(T('ar.hist.winRate'))} ${rate(sb)}${dc != null ? ` <b class="${dc > 0 ? 'up' : dc < 0 ? 'down' : ''}">${dc > 0 ? '+' : ''}${dc}</b>` : ''}</span></div>`;
    box.innerHTML = head + `<p class="ar-muted">${esc(T('ar.hist.loading'))}</p>`;
    let ta, tb;
    try { [ta, tb] = await Promise.all([this.versionText(h[a].at), this.versionText(h[b].at)]); } catch (e) { box.innerHTML = head + `<p class="ar-muted">${esc(e.message)}</p>`; return; }
    if (!$('arHistCmp') || this.histSel.a !== a || this.histSel.b !== b) return;
    const ops = diffLines(ta, tb), cnt = diffCount(ops);
    const lines = diffHunks(ops, 2).map(o => o.op === '…' ? `<span class="d-skip">⋯ ${esc(T('ar.hist.unchanged').replace('{n}', o.skip))}</span>` :
      `<span class="${o.op === '+' ? 'd-add' : o.op === '-' ? 'd-del' : 'd-same'}">${o.op} ${esc(o.text)}</span>`).join('');
    box.innerHTML = head + (cnt.add || cnt.del ? `<div class="ar-muted">${esc(T('ar.hist.lines').replace('{a}', cnt.add).replace('{d}', cnt.del))}</div><pre class="ar-diff">${lines}</pre>` : `<p class="ar-muted">${esc(T('ar.hist.same'))}</p>`);
  },
  histClick(e) {
    const b = e.target.closest('[data-restore]'); if (b) this.restoreVersion(+b.dataset.restore);
  },
  // Torna a posar una versió a l'editor i la desa (queda com la més nova; l'actual es conserva a la llista).
  async restoreVersion(i) {
    const v = this.history()[i]; if (!v) return;
    let text;
    try { text = await this.versionText(v.at); } catch (e) { $('arStatus').textContent = e.message; return; }
    const md = $('arMD');
    if (md.value.trim() && md.value.trim() !== text.trim() && !confirm(T('ar.hist.restoreConfirm'))) return;
    md.value = text; $('arCount').textContent = md.value.length + ' / 12000';
    clearTimeout(this.saveTimer);
    await this.save();
    $('arStatus').textContent = T('ar.hist.restored');
    await this.load();
  },

  showMD(title, md) {
    const dlg = document.createElement('dialog');
    dlg.className = 'ar-dialog';
    dlg.innerHTML = `<h3>${esc(title)}</h3><pre>${esc(md)}</pre><button type="button">${esc(T('ar.close'))}</button>`;
    dlg.querySelector('button').onclick = () => { dlg.close(); dlg.remove(); };
    document.body.append(dlg); dlg.showModal();
  },

  feedbackHTML(f, title) {
    if (!f) return '';
    const bars = RUBRIC.map(k => { const v = (f.scores || {})[k] || 0; return `<div class="ar-bar"><span>${esc(T('ar.rubric.' + k))}</span><i style="--v:${v * 20}%"></i><b>${v}/5</b></div>`; }).join('');
    const mistakes = (f.mistakes || []).map(m => `<li><b>${esc(m.moment)}</b> — ${esc(m.what)}<div class="ar-issue">${esc(m.md_issue)}</div><div class="ar-fix">✍ ${esc(m.fix)}</div></li>`).join('');
    return `<div class="ar-report"><h4>${esc(title)}</h4><div class="ar-bars">${bars}</div><p>${esc(f.summary || '')}</p>
      ${(f.strengths || []).length ? `<h5>${esc(T('ar.strengths'))}</h5><ul>${f.strengths.map(x => `<li>${esc(x)}</li>`).join('')}</ul>` : ''}
      ${mistakes ? `<h5>${esc(T('ar.mistakes'))}</h5><ol class="ar-mistakes">${mistakes}</ol>` : ''}
      ${(f.suggestions || []).length ? `<h5>${esc(T('ar.suggestions'))}</h5><ul>${f.suggestions.map(x => `<li>${esc(x)}</li>`).join('')}</ul>` : ''}</div>`;
  },

  // Mode tele: la retransmissió d'aquest partit (explicació, prèvia, directe
  // amb les crides de cada equip, resultat i anàlisi) a pantalla completa.
  openTV(id, speed) {
    this.closeTV();
    this.pauseView();
    let tok = ''; try { tok = localStorage.getItem('gregal_token') || ''; } catch (e) {}
    const box = document.createElement('div'); box.className = 'ar-tv'; box.id = 'arTVBox';
    const fr = document.createElement('iframe'); fr.allow = 'autoplay; fullscreen'; fr.title = T('ar.tv');
    fr.src = '/app/arena-stream.html#match=' + encodeURIComponent(id) + '&lang=' + lang() + (speed ? '&speed=' + speed : '') + (tok ? '&token=' + encodeURIComponent(tok) : '');
    box.append(fr); document.body.append(box);
    if (box.requestFullscreen) box.requestFullscreen().catch(() => {});
    box.addEventListener('fullscreenchange', () => { if (!document.fullscreenElement) this.closeTV(); });
    fr.focus();
  },

  // El mode en directe, a pantalla completa com la tele (el mateix marc).
  openLive() {
    this.closeTV();
    this.pauseView();
    let tok = ''; try { tok = localStorage.getItem('gregal_token') || ''; } catch (e) {}
    const box = document.createElement('div'); box.className = 'ar-tv'; box.id = 'arTVBox';
    const fr = document.createElement('iframe'); fr.allow = 'fullscreen; pointer-lock; autoplay'; fr.title = T('ar.playLive');
    fr.src = '/app/arena-cs2-live.html#lang=' + lang() + (tok ? '&token=' + encodeURIComponent(tok) : '');
    // Sortir de la pantalla completa (Esc) no tanca el partit: el tanca el botó de dins.
    box.append(fr); document.body.append(box);
    if (box.requestFullscreen) box.requestFullscreen().catch(() => {});
    fr.focus();
  },

  closeTV() {
    const box = $('arTVBox'); if (!box) return;
    box.remove();
    if (document.fullscreenElement) document.exitFullscreen().catch(() => {});
  },

  // Els botons de la dreta de la barra: subtítols, mode tele, descarregar el vídeo i compartir-lo.
  playerButtons() {
    const box = document.createElement('span'); box.className = 'x';
    box.innerHTML = `<button type="button" class="cc" id="arCCBtn" title="${esc(T('ar.vp.ccHelp'))}">CC</button><button type="button" id="arTV" title="${esc(T('ar.tv'))}">📺</button>` +
      `<button type="button" class="dl" id="arExport">${esc(T('ar.vp.download'))}</button><button type="button" id="arShare" hidden>${esc(T('ar.vp.share'))}</button>`;
    box.querySelector('#arCCBtn').classList.toggle('on', this.cc);
    box.querySelector('#arCCBtn').onclick = () => { this.cc = !this.cc; try { localStorage.setItem('arenaCC', this.cc ? '1' : '0'); } catch (e) {} box.querySelector('#arCCBtn').classList.toggle('on', this.cc); if (!this.cc) $('arCC').classList.remove('on'); };
    box.querySelector('#arTV').onclick = () => this.match && this.openTV(this.match.id);
    box.querySelector('#arExport').onclick = () => {
      if (!this.match) return;
      if (this.match.status !== 'final') { $('arStatus').textContent = T('ar.vp.downloadWait'); return; }
      this.exportVideo(this.match.id);
    };
    box.querySelector('#arShare').onclick = () => this.shareVideo();
    return box;
  },

  // ⬇ sempre a la vista: apagat (amb el perquè) fins que el partit s'acaba.
  paintDownload() {
    const b = $('arExport'); if (!b || !this.match) return;
    const ready = this.match.status === 'final';
    b.classList.toggle('wait', !ready); b.setAttribute('aria-disabled', ready ? 'false' : 'true');
    b.title = ready ? T('ar.vp.downloadHelp') : T('ar.vp.downloadWait');
  },

  // Compartir el vídeo exportat (el sistema ho fa: Web Share amb el fitxer).
  async shareVideo() {
    const v = this.lastVideo; if (!v) return;
    try { await navigator.share({ files: [v.file], title: v.file.name }); } catch (e) { if (e && e.name !== 'AbortError') $('arStatus').textContent = e.message; }
  },

  // A cada repàs de la barra (4 cops per segon): la línia de la pista que toca,
  // el subtítol i, quan s'ha vist tot, el resum dins del reproductor.
  onProgress(pr) {
    if (!this.match) return;
    this.progress = pr;
    this.paintTrack();
    const act = activeEntry(this.track || [], pr.index);
    const key = act ? act.i : -1;
    if (key !== this.activeKey) {
      this.activeKey = key;
      $('arTrack').querySelectorAll('.ar-tk.now').forEach(x => x.classList.remove('now'));
      const li = act && $('arTrack').querySelector(`.ar-tk[data-i="${act.i}"]`);
      if (li) { li.classList.add('now'); if (!$('arTrack').matches(':hover')) li.scrollIntoView({ block: 'nearest' }); }
      $('arTrack').querySelectorAll('.ar-tk').forEach(x => x.classList.toggle('future', +x.dataset.i > pr.index - 1));
      // Subtítol: només quan la línia comença ara (no en saltar enrere a mig partit).
      clearTimeout(this.ccT);
      if (this.cc && act && act.i === pr.index - 1 && captionOf(act)) {
        $('arCC').textContent = captionOf(act); $('arCC').classList.add('on');
        this.ccT = setTimeout(() => $('arCC').classList.remove('on'), 6500);
      } else $('arCC').classList.remove('on');
    }
    // El resum, quan s'arriba al final d'un partit acabat; tornar enrere el treu.
    const atEnd = this.match.status === 'final' && pr.total > 0 && this.view && this.view.isFinished && this.view.isFinished();
    if (atEnd && !this.endShown) {
      this.endShown = 'on';
      const ix = new Map((pr.events || []).map((e, i) => [e.n, i]));
      $('arEnd').innerHTML = summaryHTML(this.match, { T, sideColor: s => sideColor(this.match, s), momentIndex: mo => (mo.event && ix.has(mo.event) ? ix.get(mo.event) : null), times: pr.times || [] });
      $('arEnd').hidden = false;
    } else if (!atEnd && this.endShown) { this.endShown = ''; $('arEnd').hidden = true; }
  },

  // La pista de comentaris: es refà quan arriben esdeveniments, frases o moments.
  paintTrack() {
    const pr = this.progress; if (!pr || !this.match) return;
    const key = [pr.total, (this.lines && this.lines.size) || 0, this.moments ? this.moments.size : 0, (this.match.decisions || []).length, this.trackMode].join('|');
    if (key === this.trackKey) return;
    this.trackKey = key; this.activeKey = null;
    this.track = buildTrack({ events: pr.events || [], times: pr.times || [], lines: this.lines || new Map(), decisions: this.match.decisions || [], moments: this.moments || new Map() });
    const list = filterTrack(this.track, this.trackMode || 'all');
    const names = this.match.names || [];
    $('arTrack').innerHTML = list.map(e => trackItemHTML(e, { teamName: s => names[s] || '', sideColor: s => sideColor(this.match, s) })).join('') ||
      `<li class="none">${esc(T('ar.vp.noTrack'))}</li>`;
  },

  pauseView() { if (this.view && !this.view.paused) this.view.togglePause(); if (this.scrub) this.scrub.sync(); },

  closeMatch() {
    if (this.view) this.view.destroy();
    if (this.scrub) this.scrub.destroy();
    this.view = null; this.scrub = null; this.match = null; this.progress = null; this.track = []; this.trackKey = ''; this.lines = new Map(); this.endShown = ''; this.lastVideo = null; clearTimeout(this.ccT); clearTimeout(this.poll); clearInterval(this.thinkTimer);
    this.decN = 0; this.lineN = 0; this.starKey = ''; this.fbKey = ''; this.folKey = null;
    $('arMatchCard').hidden = true; $('arPlayCard').hidden = false;
  },

  // Fora de la vista de l'Arena: ni consultes ni rellotges (el visor s'atura sol
  // quan no es veu). load() ho torna a engegar en tornar-hi.
  suspend() {
    this.suspended = true;
    clearTimeout(this.poll); clearInterval(this.thinkTimer); clearTimeout(this.seedTimer); clearTimeout(this.voiceTimer); clearTimeout(this.lastTimer);
    this.pauseView();
  },
  resume() {
    if (!this.suspended) return;
    this.suspended = false;
    const m = this.match;
    if (m && !['final', 'stopped', 'failed'].includes(m.status)) { clearInterval(this.thinkTimer); this.thinkTimer = setInterval(() => this.paintMatchStatus(), 1000); }
    if (m && m.status === 'final' && !(m.feedback && (m.feedback[0] || m.feedback[1]))) this.pollFeedback(m.id, 1000);
    // L'informe de l'últim partit que encara no havia arribat.
    if (this.lastId && this.lastCache && this.lastCache.id === this.lastId && !this.lastCache.fb) { this.lastTries = 0; this.loadLast(this.lastId); }
  },

  // La consulta del partit: només les decisions i frases noves (dafter/lafter).
  matchURL(id, after) { return MATCH_Q + encodeURIComponent(id) + '&after=' + after + '&dafter=' + (this.decN || 0) + '&lafter=' + (this.lineN || 0); },
  fetchMatch(id, after) {
    const dk = this.decN || 0, lk = this.lineN || 0;
    return api(this.matchURL(id, after)).then(m => { this.updateMatch(m, dk, lk); return m; });
  },

  async openMatch(id) {
    this.closeMatch();
    const card = $('arMatchCard');
    card.hidden = false; $('arPlayCard').hidden = true; $('arAgainRow').hidden = true; $('arFeedback').innerHTML = ''; $('arStats').hidden = true;
    $('arDecisions').innerHTML = `<h4>${esc(T('ar.why'))}</h4><p class="ar-how">${esc(T('ar.howDecides'))}</p><div id="arDecFollow"></div><p class="ar-muted" id="arDecWait">${esc(T('ar.thinking'))}</p><div id="arDecList" class="ar-dec-list"></div>`;
    $('arTrack').innerHTML = ''; $('arEnd').hidden = true; $('arCC').classList.remove('on');
    card.scrollIntoView({ behavior: 'smooth', block: 'start' });
    let first;
    try { first = await api(MATCH_Q + encodeURIComponent(id) + '&after=0'); } catch (e) { $('arStatus').textContent = e.message; return; }
    this.match = { ...first, decisions: [] }; this.lastEvAt = Date.now();
    // Al LoL el mapa posa el blau a l'esquerra: el títol i el marcador, igual.
    this.flip = first.game === 'lol' && first.meta && first.meta.blue === 1;
    const [l, r] = this.flip ? [1, 0] : [0, 1];
    $('arMatchTitle').textContent = `${first.names[l]} vs ${first.names[r]}`;
    this.updateMatch(first, 0, 0);
    try {
      this.view = await makeView(first.game, $('arCanvas'), { lang: lang(), fetchMatch: after => this.fetchMatch(id, after), onEvent: e => this.highlight(e) });
      this.view.setSpeed(this.speed);
      // La barra del reproductor: enrere, des del principi, saltar a un moment
      // del prompt (★), el temps, la velocitat, subtítols, tele i descarregar.
      this.scrub = mountScrubber($('arScrub'), {
        view: this.view, t: Tq, speed: this.speed, keys: true, extra: this.playerButtons(),
        moments: () => this.moments, teamName: s => this.match ? this.match.names[s] : '', sideColor: s => sideColor(this.match, s),
        onSpeed: x => { this.speed = x; }, onSeek: () => this.seeked(), onSync: pr => this.onProgress(pr),
      });
      this.paintDownload();
    } catch (e) { $('arStatus').textContent = e.message; }
    // Si el model triga, que es vegi que hi està pensant i no que s'ha penjat.
    clearInterval(this.thinkTimer);
    if (this.match && !['final', 'stopped', 'failed'].includes(this.match.status)) this.thinkTimer = setInterval(() => this.paintMatchStatus(), 1000);
  },

  paintMatchStatus() {
    const m = this.match; if (!m) return;
    let st = T('ar.status.' + m.status) + ' · ' + (this.flip ? m.score[1] + ' - ' + m.score[0] : m.score[0] + ' - ' + m.score[1]);
    // A la cua: quants partits hi ha davant (1 = el següent).
    if (m.status === 'pregame' && m.queue > 0) st = T('ar.queue').replace('{n}', m.queue);
    else if ((m.status === 'live' || m.status === 'pregame') && Date.now() - this.lastEvAt > 4000) st += ' · ' + T('ar.thinking');
    $('arMatchStatus').textContent = st;
  },

  // El que ha costat el partit: crides al model · tokens (quants de la memòria cau) · temps.
  paintStats(st) {
    const el = $('arStats');
    const calls = st ? st.all_calls || st.calls || 0 : 0, pt = st ? st.all_prompt_tokens || st.prompt_tokens || 0 : 0, ct = st ? st.all_completion_tokens || st.completion_tokens || 0 : 0;
    // Fins que no hi ha la primera crida, res (sense «0 crides»).
    if (!st || (!calls && !pt && !st.memo)) { el.hidden = true; return; }
    const cached = st.all_cached_tokens || st.cached_tokens || 0, secs = (st.wall_ms || 0) / 1000;
    el.textContent = (pt + ct ? T('ar.stats') : T('ar.statsNoTokens')).replace('{calls}', n0(calls)).replace('{tokens}', n0(pt + ct))
      .replace('{cached}', pt ? Math.round(cached * 100 / pt) : 0).replace('{time}', secs ? fmtTime(secs) : '—');
    el.title = [`decisions ${st.decisions || 0}`, `decision calls ${st.calls || 0}`, st.retries ? `retries ${st.retries}` : '', st.fallbacks ? `fallbacks ${st.fallbacks}` : '', st.memo ? `reused ${st.memo}` : '',
      `prompt ${n0(pt)} · completion ${n0(ct)} · cached ${n0(cached)}`, `model ${fmtTime((st.model_ms || 0) / 1000)}`, st.queue_ms ? `queue ${fmtTime(st.queue_ms / 1000)}` : ''].filter(Boolean).join('\n');
    el.hidden = false;
  },

  // Una resposta de /api/arena/match. dk/lk: quantes decisions i frases teníem
  // quan es va demanar (amb dafter/lafter només arriben les noves). Les files
  // de decisions s'afegeixen (no es refà la llista) i el clic va per delegació.
  updateMatch(m, dk = 0, lk = 0) {
    if (!this.match || m.id !== this.match.id) return;
    if ((m.events || []).length) this.lastEvAt = Date.now();
    this.match.status = m.status; this.match.score = m.score; this.match.feedback = m.feedback; this.match.queue = m.queue || 0;
    if (m.winner != null) this.match.winner = m.winner;
    this.match.md = m.md || this.match.md;
    if (m.analysis) this.match.analysis = m.analysis;
    if (m.stats) this.match.stats = m.stats;
    this.moments = momentsByEvent(this.match);
    // Les frases del narrador arriben una mica després de les jugades.
    if (!this.lines) this.lines = new Map();
    (m.lines || []).forEach(l => this.lines.set(l.n, l));
    this.lineN = Math.max(this.lineN || 0, (m.lines_total != null ? lk : 0) + (m.lines || []).length);
    // Les decisions noves: les que arriben a partir de la que ja teníem.
    const from = m.decisions_total != null ? dk : 0, got = m.decisions || [];
    const fresh = got.slice(Math.max(0, (this.decN || 0) - from));
    if (from <= (this.decN || 0) && fresh.length) {
      this.match.decisions = (this.match.decisions || []).concat(fresh);
      this.decN = this.match.decisions.length;
      this.appendDecisions(fresh);
    }
    if (m.followed) this.match.followed = m.followed;
    this.paintDecHead();
    this.paintStars();
    this.paintDownload();
    this.paintMatchStatus();
    this.paintStats(this.match.stats);
    if (['final', 'stopped', 'failed'].includes(m.status)) { $('arAgainRow').hidden = false; clearInterval(this.thinkTimer); }
    if (m.status === 'final' && m.feedback) {
      const names = this.match.names;
      const key = [0, 1].map(s => (m.feedback[s] ? 1 : 0)).join('');
      if (key !== this.fbKey) {
        this.fbKey = key;
        $('arFeedback').innerHTML = [0, 1].map(s => m.feedback[s] ? this.feedbackHTML(m.feedback[s], T('ar.reportFor').replace('{name}', names[s])) : '').join('') ||
          `<p class="ar-muted">${esc(T('ar.reportPending'))}</p>`;
      }
      if (!m.feedback[0] && !m.feedback[1]) this.pollFeedback(m.id, 8000);
    }
  },

  // L'informe del coach arriba després del final: es torna a mirar (sense les jugades).
  pollFeedback(id, ms) {
    clearTimeout(this.poll);
    if (this.suspended) return;
    this.poll = setTimeout(() => { if (this.match && this.match.id === id && !this.suspended) this.fetchMatch(id, 999999).catch(() => {}); }, ms);
  },

  // Cada decisió: què ha triat, per què, la regla del .md que cita i si el
  // servidor l'ha trobada de debò al .md (✓) o el model ha improvisat (⚠).
  // Clicant-la, el que el model va veure. Com a molt 400 a la pàgina.
  appendDecisions(decs) {
    const list = $('arDecList'); if (!list) return;
    const names = this.match.names;
    const badge = d => d.fallback ? `<em class="bad">⚠ ${esc(T('ar.fallback'))}</em>` : d.rule_found ? `<em class="ok">✓ ${esc(T('ar.ruleFound'))}</em>` : `<em class="warn">⚠ ${esc(T('ar.improvised'))}</em>`;
    const starred = momentKeys(this.match);
    list.insertAdjacentHTML('beforeend', decs.slice(-400).map(d => `<div class="ar-dec side${d.side}${starred.has(d.n + ':' + d.side) ? ' ar-star' : ''}" data-n="${d.n}" data-side="${d.side}" title="${esc(T('ar.seeSituation'))}"><b><span class="ar-st" title="Prompt moment: this .md line mattered">★ </span>${esc(names[d.side])}</b> <small>${esc(d.short)}</small>
      <code>${esc(callText(d, { short: false }))}</code>${d.why ? `<span>${esc(d.why)}</span>` : ''}${d.rule ? `<q>${esc(d.rule)}</q>` : ''}${badge(d)}</div>`).join(''));
    while (list.childElementCount > 400) list.firstElementChild.remove();
  },
  paintDecHead() {
    const fol = this.match.followed, names = this.match.names, n = this.decN || 0;
    const pct = v => Math.round((v || 0) * 100) + '%';
    const key = n ? JSON.stringify(fol || null) : '';
    if (key !== this.folKey) {
      this.folKey = key;
      const el = $('arDecFollow'); if (el) el.innerHTML = fol && n ? `<div class="ar-follow">${[0, 1].map(i => `<span class="side${i}"><b>${esc(names[i])}</b> ${esc(T('ar.followed'))} <b>${pct(fol[i])}</b></span>`).join('')}</div>` : '';
    }
    const w = $('arDecWait'); if (w) w.hidden = n > 0;
  },
  // Les decisions que són moments del prompt (★) arriben amb l'anàlisi, al final.
  paintStars() {
    const starred = momentKeys(this.match), key = [...starred].join(',');
    if (key === this.starKey) return;
    this.starKey = key;
    const list = $('arDecList'); if (!list) return;
    list.querySelectorAll('.ar-dec').forEach(el => el.classList.toggle('ar-star', starred.has(el.dataset.n + ':' + el.dataset.side)));
  },

  // El vídeo de resum del partit (arena-export.js), descarregat.
  async exportVideo(id) {
    if (this.exporting) return;
    const btn = (on, text) => { const x = $('arExport'); if (x) { x.disabled = on; x.textContent = text; } };
    this.exporting = true; btn(true, T('ar.vp.exporting'));
    this.pauseView();
    try {
      const r = await exportVideo({
        id,
        fetchMatch: after => api('/api/arena/match?id=' + encodeURIComponent(id) + '&after=' + after),
        fetchVoice: key => window.api('/api/arena/voice?k=' + encodeURIComponent(key)).then(x => x.ok ? x.arrayBuffer() : null),
      });
      if (r) {
        // Quant dura el vídeo i quant ha costat fer-lo (el camí ràpid va més de pressa que el partit).
        $('arStatus').textContent = `⬇ ${r.name} (${(r.size / 1e6).toFixed(1)} MB` + (r.secs ? ` · ${fmtTime(r.secs)} ` + T('ar.vp.madeIn').replace('{t}', fmtTime(r.wall)).replace('{x}', (r.secs / Math.max(0.1, r.wall)).toFixed(1)) : '') + ')';
        // Compartir-lo, si el sistema ho sap fer amb fitxers.
        const file = typeof File === 'function' ? new File([r.blob], r.name, { type: r.type }) : null;
        if (file && navigator.canShare && navigator.canShare({ files: [file] }) && $('arShare')) { this.lastVideo = { file }; $('arShare').hidden = false; }
      }
    } catch (e) { $('arStatus').textContent = e.message; }
    this.exporting = false; btn(false, T('ar.vp.download'));
  },

  // El que el model va veure en una decisió: la situació i el format.
  async showSituation(id, n, side, who) {
    let x;
    try { x = await api('/api/arena/match?id=' + encodeURIComponent(id) + '&situation=' + n + '&side=' + side); }
    catch (e) { $('arStatus').textContent = T('ar.noSituation'); return; }
    const d = (this.match.decisions || []).find(v => v.n === n && v.side === side) || {};
    this.showMD(`${who} · ${d.short || ''}`, `${T('ar.sawTitle')}\n\n${x.situation}\n\n${T('ar.formatTitle')}\n${x.schema}\n\n${T('ar.answerTitle')}\n${JSON.stringify(d.decision || {})}\n${d.why ? 'why: ' + d.why + '\n' : ''}${d.rule ? 'rule: ' + d.rule : ''}`);
  },

  // Després d'un salt: fora la targeta i la decisió marcada; l'esdeveniment on
  // s'aterra (onEvent) les torna a posar si toca.
  seeked() {
    const card = $('arStage').querySelector('.ar-moment-card');
    if (card) { clearTimeout(card._t); card.classList.remove('on'); }
    clearTimeout(this.ccT); $('arCC').classList.remove('on'); this.activeKey = null;
    $('arDecisions').querySelectorAll('.ar-dec.now').forEach(x => x.classList.remove('now'));
  },

  highlight(e) {
    if (!e || !e.decision) return;
    // Un moment del prompt: la targeta sobre el partit.
    const mo = this.moments && this.moments.get(e.n);
    if (mo && this.match) showMomentCard($('arStage'), mo, this.match.names[mo.side], sideColor(this.match, mo.side), { secs: 5 });
    const box = $('arDecisions');
    box.querySelectorAll('.ar-dec.now').forEach(x => x.classList.remove('now'));
    const els = box.querySelectorAll(`.ar-dec[data-n="${e.decision}"]`);
    els.forEach(x => x.classList.add('now'));
    if (els[0]) els[0].scrollIntoView({ block: 'nearest' });
  },

  async openComp(id) {
    let c;
    try { c = await api('/api/arena/competition?id=' + encodeURIComponent(id)); } catch (e) { $('arStatus').textContent = e.message; return; }
    this.comp = c;
    const name = o => c.names[o] || o;
    const admin = this.data && this.data.admin;
    let h = `<div class="ar-comp"><div class="ar-row"><h4>${esc(c.title)}</h4><span class="ar-muted">${esc(T('ar.compStatus.' + c.status))}</span>
      ${admin && (c.status === 'ready' || c.status === 'stopped') ? `<button type="button" id="arCompStart" class="primary">${esc(T('ar.compStart'))}</button>` : ''}
      ${admin && c.status === 'running' ? `<button type="button" id="arCompStop">${esc(T('ar.compStop'))}</button>` : ''}</div>`;
    for (const g of c.games) {
      h += `<h5>${esc(g.toUpperCase())}${c.champion && c.champion[g] ? ` · 🏆 ${esc(name(c.champion[g]))}` : ''}</h5>`;
      for (const tb of (c.tables[g] || [])) {
        h += `<table class="ar-table"><caption>${esc(tb.stage)}</caption><tr><th></th><th>${esc(T('ar.player'))}</th><th>${esc(T('ar.pj'))}</th><th>${esc(T('ar.w'))}</th><th>${esc(T('ar.d'))}</th><th>${esc(T('ar.l'))}</th><th>${esc(T('ar.pts'))}</th></tr>` +
          tb.rows.map((r, i) => `<tr><td>${i + 1}</td><td>${esc(name(r.owner))}</td><td>${r.played}</td><td>${r.won}</td><td>${r.drawn}</td><td>${r.lost}</td><td><b>${r.pts}</b></td></tr>`).join('') + '</table>';
      }
      h += '<ul class="ar-list">' + c.schedule.filter(s => s.game === g).map(s => `<li ${s.match ? `data-id="${esc(s.match)}"` : ''}><b>${esc(s.stage)}</b> ${s.pending ? esc(s.from.join(' vs ')) : `${esc(name(s.a))} ${s.done ? s.score[0] + ' - ' + s.score[1] : 'vs'} ${esc(name(s.b))}`}</li>`).join('') + '</ul>';
      if (c.status === 'done' && c.entries[g]) h += `<p class="ar-muted">${esc(T('ar.learnFrom'))}</p><ul class="ar-bots">` + c.entries[g].map(e => `<li><b>${esc(e.display)}</b><button type="button" data-md="${esc(g + '|' + e.owner)}">${esc(T('ar.readMD'))}</button></li>`).join('') + '</ul>';
    }
    $('arComp').innerHTML = h + '</div>';
    $('arComp').querySelectorAll('[data-id]').forEach(li => li.onclick = () => this.openMatch(li.dataset.id));
    $('arComp').querySelectorAll('[data-md]').forEach(b => b.onclick = () => { const [g, o] = b.dataset.md.split('|'); const e = c.entries[g].find(x => x.owner === o); this.showMD(e.display, e.md); });
    if ($('arCompStart')) $('arCompStart').onclick = async () => { await post('/api/arena/competition', { action: 'start', id }).catch(e => { $('arStatus').textContent = e.message; }); this.openComp(id); };
    if ($('arCompStop')) $('arCompStop').onclick = async () => { await post('/api/arena/competition', { action: 'stop', id }).catch(() => {}); this.openComp(id); };
  },
};

window.gregalArena = arenaUI;
