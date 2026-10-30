// L'Arena (arena.go): cada jugador escriu un .md d'estratègia per joc i el
// mateix model juga per tots dos equips llegint-lo. La pàgina és un sol camí:
// tria el joc, escriu (o deixa la plantilla, que ja hi és) i prem Juga; el
// rival el tria el servidor (el primer bot que encara no has guanyat). Al
// final del partit, l'informe del coach i «Torna-hi». El rànquing, els bots,
// els partits, les competicions i la retransmissió van plegats a sota.
import { makeView, LESSONS, GUIDE } from './arena-views.js';

const $ = id => document.getElementById(id);
// Els jocs van sempre en anglès.
const T = key => (window.gregalTEn ? window.gregalTEn(key) : '') || key;
const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const api = (path, opts) => window.api(path, opts).then(async r => { const j = await r.json().catch(() => ({})); if (!r.ok) throw new Error(j.error || (typeof j === 'string' ? j : r.status)); return j; });
const lang = () => 'en';
const post = (path, body) => api(path, { method: 'POST', body: JSON.stringify(body) });
const RUBRIC = ['clarity', 'coverage', 'adaptivity', 'specificity', 'consistency'];
const ICON = { nfl: '🏈', cs2: '💣', lol: '🐉' };
// Una decisió en paraules: els valors del JSON, sense guions baixos.
const callText = d => {
  const v = d && d.decision && typeof d.decision === 'object' ? Object.values(d.decision).flat().filter(x => typeof x === 'string' && x) : [];
  return v.join(' · ').replace(/_/g, ' ') || (d && d.fallback ? '—' : '');
};

const arenaUI = {
  data: null, game: 'nfl', view: null, match: null, poll: 0, comp: null, speed: 1,

  async load() {
    const page = $('arenaPage');
    if (!page || typeof window.api !== 'function') return;
    if (!page.dataset.ready) this.build(page);
    try { this.data = await api('/api/arena'); } catch (e) { $('arStatus').textContent = e.message; return; }
    if (!this.data.games.find(g => g.id === this.game) && this.data.games.length) this.game = this.data.games[0].id;
    this.paint();
    // Mentre els bots s'escriuen, es torna a mirar de tant en tant.
    clearTimeout(this.seedTimer);
    if ((this.data.seeding || []).length) this.seedTimer = setTimeout(() => this.load(), 15000);
  },

  build(page) {
    page.dataset.ready = '1';
    page.innerHTML = `<div class="ar">
      <header class="ar-head"><div><div class="page-kicker">${esc(T('ar.kicker'))}</div><h1 class="page-title">${esc(T('ar.title'))}</h1>
        <p class="page-copy">${esc(T('ar.lead2'))}</p></div>
        <label class="ar-field ar-me">${esc(T('ar.playAs'))} <input id="arDisplay" maxlength="40"></label></header>
      <nav class="ar-games" id="arGames" role="tablist"></nav>
      <span id="arStatus" class="ar-status" role="status"></span>
      <section class="ar-card" id="arMatchCard" hidden>
        <div class="ar-row"><h2 id="arMatchTitle"></h2><span class="ar-muted" id="arMatchStatus"></span>
          <span class="ar-ctl"><button type="button" id="arTV" class="primary">📺 ${esc(T('ar.tv'))}</button><button type="button" id="arPause">⏸</button><button type="button" id="arSpeed"></button><button type="button" id="arSkip">⏭ ${esc(T('ar.skipEnd'))}</button><button type="button" id="arClose" title="${esc(T('ar.close'))}">✕</button></span></div>
        <div class="ar-match"><div class="ar-stage"><canvas id="arCanvas"></canvas></div><div class="ar-decisions" id="arDecisions"></div></div>
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
          </div>
          <aside class="ar-tips">
            <h3>${esc(T('ar.decides'))}</h3><div id="arGuide" class="ar-guide"></div>
            <details><summary>${esc(T('ar.tips'))}</summary><ol id="arLessons" class="ar-lessons"></ol></details>
          </aside>
        </div>
        <div class="ar-go">
          <button type="button" id="arPlay" class="primary ar-big"></button>
          <span class="ar-muted" id="arPlayHint"></span>
          <button type="button" id="arDemo" class="ar-demo">🎬 ${esc(T('ar.demo'))}</button>
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
    $('arAgain').onclick = () => { this.closeMatch(); $('arPlayCard').scrollIntoView({ behavior: 'smooth', block: 'start' }); md.focus(); };
    $('arPause').onclick = () => { if (this.view) $('arPause').textContent = this.view.togglePause() ? '▶' : '⏸'; };
    const paintSpeed = () => { $('arSpeed').textContent = T('ar.speed').replace('{x}', this.speed + '×'); };
    paintSpeed();
    $('arSpeed').onclick = () => { this.speed = this.speed === 1 ? 2 : this.speed === 2 ? 4 : 1; if (this.view) this.view.setSpeed(this.speed); paintSpeed(); };
    $('arSkip').onclick = () => this.view && this.view.skipToEnd();
    $('arClose').onclick = () => this.closeMatch();
    $('arTV').onclick = () => this.match && this.openTV(this.match.id);
    addEventListener('message', e => { if (e.origin === location.origin && e.data && e.data.arenaTV === 'close') this.closeTV(); });
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
    const left = Math.max(0, d.practice_left);
    $('arPlay').textContent = T('ar.playVs').replace('{name}', next ? next.display : 'Bot');
    $('arPlay').disabled = !d.admin && left <= 0;
    $('arPlayHint').textContent = (seeding ? T('ar.seedingNow') + ' ' : '') +
      T('ar.playHint').replace('{level}', next ? T('ar.level.' + next.level) : '').replace('{n}', left);
    // El pas on som: 1 escriure, 2 jugar, 3 llegir l'informe i millorar.
    const played = (d.matches || []).some(m => m.game === this.game && m.kind === 'practice' && m.status === 'final');
    const step = !mine ? 0 : played ? 2 : 1;
    $('arSteps').querySelectorAll('li').forEach((li, i) => { li.classList.toggle('on', i === step); li.classList.toggle('done', i < step); });
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
  // al teu .md (0-25). Si el .md millora, la línia puja.
  paintProgress(d) {
    const rows = ((d.progress || {})[this.game]) || [];
    const box = $('arProgress');
    if (!rows.length) { box.innerHTML = `<h3>${esc(T('ar.progress'))}</h3><p class="ar-muted">${esc(T('ar.progressNone'))}</p>`; return; }
    const W = 640, H = 150, pad = 26, n = rows.length, bw = Math.min(46, (W - pad * 2) / n * .62);
    const x = i => pad + (W - pad * 2) * (n === 1 ? .5 : i / (n - 1));
    const maxDiff = Math.max(1, ...rows.map(r => Math.abs(r.score[0] - r.score[1])));
    const mid = H * .55;
    const bars = rows.map((r, i) => {
      const diff = r.score[0] - r.score[1], h = Math.max(4, Math.abs(diff) / maxDiff * (H * .38));
      const col = r.result === 'win' ? '#2fbf71' : r.result === 'loss' ? '#e5484d' : '#9aa4bd';
      return `<g><title>${esc(r.opponent)} · ${r.score[0]}-${r.score[1]}${r.coach ? ' · coach ' + r.coach + '/25' : ''}</title>
        <rect x="${x(i) - bw / 2}" y="${diff >= 0 ? mid - h : mid}" width="${bw}" height="${h}" rx="4" fill="${col}" opacity=".85"/>
        <text x="${x(i)}" y="${H - 4}" text-anchor="middle" font-size="10" fill="currentColor" opacity=".7">${r.score[0]}-${r.score[1]}</text></g>`;
    }).join('');
    const pts = rows.map((r, i) => r.coach ? [x(i), mid - (r.coach / 25 - .5) * H * .8] : null).filter(Boolean);
    const line = pts.length > 1 ? `<polyline points="${pts.map(p => p.join(',')).join(' ')}" fill="none" stroke="var(--accent)" stroke-width="2.5"/>` : '';
    const dots = pts.map(p => `<circle cx="${p[0]}" cy="${p[1]}" r="4" fill="var(--accent)"/>`).join('');
    const wins = rows.filter(r => r.result === 'win').length;
    const coached = rows.filter(r => r.coach);
    const trend = coached.length > 1 ? `${coached[0].coach} → ${coached[coached.length - 1].coach}/25` : coached.length ? `${coached[0].coach}/25` : '—';
    box.innerHTML = `<div class="ar-row"><h3>${esc(T('ar.progress'))}</h3><span class="ar-muted">${esc(T('ar.progressSum').replace('{w}', wins).replace('{n}', n).replace('{c}', trend))}</span></div>
      <svg viewBox="0 0 ${W} ${H}" class="ar-chart" role="img"><line x1="${pad}" x2="${W - pad}" y1="${mid}" y2="${mid}" stroke="currentColor" opacity=".2"/>${bars}${line}${dots}</svg>
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
    if (this.view && !this.view.paused) { this.view.togglePause(); $('arPause').textContent = '▶'; }
    let tok = ''; try { tok = localStorage.getItem('gregal_token') || ''; } catch (e) {}
    const box = document.createElement('div'); box.className = 'ar-tv'; box.id = 'arTVBox';
    const fr = document.createElement('iframe'); fr.allow = 'autoplay; fullscreen'; fr.title = T('ar.tv');
    fr.src = '/app/arena-stream.html#match=' + encodeURIComponent(id) + '&lang=' + lang() + (speed ? '&speed=' + speed : '') + (tok ? '&token=' + encodeURIComponent(tok) : '');
    box.append(fr); document.body.append(box);
    if (box.requestFullscreen) box.requestFullscreen().catch(() => {});
    box.addEventListener('fullscreenchange', () => { if (!document.fullscreenElement) this.closeTV(); });
    fr.focus();
  },

  closeTV() {
    const box = $('arTVBox'); if (!box) return;
    box.remove();
    if (document.fullscreenElement) document.exitFullscreen().catch(() => {});
  },

  closeMatch() {
    if (this.view) this.view.destroy();
    this.view = null; this.match = null; clearTimeout(this.poll); clearInterval(this.thinkTimer);
    $('arMatchCard').hidden = true; $('arPlayCard').hidden = false;
  },

  async openMatch(id) {
    this.closeMatch();
    const card = $('arMatchCard');
    card.hidden = false; $('arPlayCard').hidden = true; $('arAgainRow').hidden = true; $('arFeedback').innerHTML = ''; $('arDecisions').innerHTML = '';
    card.scrollIntoView({ behavior: 'smooth', block: 'start' });
    let first;
    try { first = await api('/api/arena/match?id=' + encodeURIComponent(id) + '&after=0'); } catch (e) { $('arStatus').textContent = e.message; return; }
    this.match = first; this.lastEvAt = Date.now();
    // Al LoL el mapa posa el blau a l'esquerra: el títol i el marcador, igual.
    this.flip = first.game === 'lol' && first.meta && first.meta.blue === 1;
    const [l, r] = this.flip ? [1, 0] : [0, 1];
    $('arMatchTitle').textContent = `${first.names[l]} vs ${first.names[r]}`;
    const fetchMatch = after => api('/api/arena/match?id=' + encodeURIComponent(id) + '&after=' + after).then(m => { this.updateMatch(m); return m; });
    try {
      this.view = await makeView(first.game, $('arCanvas'), { lang: lang(), fetchMatch, onEvent: e => this.highlight(e) });
      this.view.setSpeed(this.speed);
    } catch (e) { $('arStatus').textContent = e.message; }
    this.updateMatch(first);
    // Si el model triga, que es vegi que hi està pensant i no que s'ha penjat.
    clearInterval(this.thinkTimer);
    this.thinkTimer = setInterval(() => this.paintMatchStatus(), 1000);
  },

  paintMatchStatus() {
    const m = this.match; if (!m) return;
    let st = T('ar.status.' + m.status) + ' · ' + (this.flip ? m.score[1] + ' - ' + m.score[0] : m.score[0] + ' - ' + m.score[1]);
    if ((m.status === 'live' || m.status === 'pregame') && Date.now() - this.lastEvAt > 4000) st += ' · ' + T('ar.thinking');
    $('arMatchStatus').textContent = st;
  },

  updateMatch(m) {
    if (!this.match || m.id !== this.match.id) return;
    if ((m.events || []).length) this.lastEvAt = Date.now();
    this.match.status = m.status; this.match.score = m.score; this.match.feedback = m.feedback;
    this.match.decisions = m.decisions || this.match.decisions; this.match.md = m.md || this.match.md;
    this.paintMatchStatus();
    const names = this.match.names;
    const decs = this.match.decisions || [];
    // Cada decisió: què ha triat, per què, la regla del .md que cita i si el
    // servidor l'ha trobada de debò al .md (✓) o el model ha improvisat (⚠).
    // Clicant-la, el que el model va veure.
    const fol = m.followed || this.match.followed;
    if (m.followed) this.match.followed = m.followed;
    const pct = v => Math.round((v || 0) * 100) + '%';
    const badge = d => d.fallback ? `<em class="bad">⚠ ${esc(T('ar.fallback'))}</em>` : d.rule_found ? `<em class="ok">✓ ${esc(T('ar.ruleFound'))}</em>` : `<em class="warn">⚠ ${esc(T('ar.improvised'))}</em>`;
    $('arDecisions').innerHTML = `<h4>${esc(T('ar.why'))}</h4>
      <p class="ar-how">${esc(T('ar.howDecides'))}</p>
      ${fol && decs.length ? `<div class="ar-follow">${[0, 1].map(i => `<span class="side${i}"><b>${esc(names[i])}</b> ${esc(T('ar.followed'))} <b>${pct(fol[i])}</b></span>`).join('')}</div>` : ''}` +
      (decs.length ? '' : `<p class="ar-muted">${esc(T('ar.thinking'))}</p>`) +
      decs.slice(-400).map(d => `<div class="ar-dec side${d.side}" data-n="${d.n}" data-side="${d.side}" title="${esc(T('ar.seeSituation'))}"><b>${esc(names[d.side])}</b> <small>${esc(d.short)}</small>
      <code>${esc(callText(d))}</code>${d.why ? `<span>${esc(d.why)}</span>` : ''}${d.rule ? `<q>${esc(d.rule)}</q>` : ''}${badge(d)}</div>`).join('');
    $('arDecisions').querySelectorAll('.ar-dec').forEach(el => el.onclick = () => this.showSituation(m.id || this.match.id, +el.dataset.n, +el.dataset.side, names[+el.dataset.side]));
    if (['final', 'stopped', 'failed'].includes(m.status)) { $('arAgainRow').hidden = false; clearInterval(this.thinkTimer); }
    const fetchAgain = () => api('/api/arena/match?id=' + encodeURIComponent(m.id) + '&after=999999').then(x => this.updateMatch(x)).catch(() => {});
    if (m.status === 'final' && m.feedback) {
      $('arFeedback').innerHTML = [0, 1].map(s => m.feedback[s] ? this.feedbackHTML(m.feedback[s], T('ar.reportFor').replace('{name}', names[s])) : '').join('') ||
        `<p class="ar-muted">${esc(T('ar.reportPending'))}</p>`;
      if (!m.feedback[0] && !m.feedback[1]) { clearTimeout(this.poll); this.poll = setTimeout(fetchAgain, 8000); }
    }
  },

  // El que el model va veure en una decisió: la situació i el format.
  async showSituation(id, n, side, who) {
    let x;
    try { x = await api('/api/arena/match?id=' + encodeURIComponent(id) + '&situation=' + n + '&side=' + side); }
    catch (e) { $('arStatus').textContent = T('ar.noSituation'); return; }
    const d = (this.match.decisions || []).find(v => v.n === n && v.side === side) || {};
    this.showMD(`${who} · ${d.short || ''}`, `${T('ar.sawTitle')}\n\n${x.situation}\n\n${T('ar.formatTitle')}\n${x.schema}\n\n${T('ar.answerTitle')}\n${JSON.stringify(d.decision || {})}\n${d.why ? 'why: ' + d.why + '\n' : ''}${d.rule ? 'rule: ' + d.rule : ''}`);
  },

  highlight(e) {
    if (!e || !e.decision) return;
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
