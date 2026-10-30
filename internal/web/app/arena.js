// L'Arena (arena.go): cada jugador escriu un .md d'estratègia per joc i el
// mateix model juga per tots dos equips llegint-lo. Aquí: l'editor del .md
// (amb la plantilla, l'historial i la revisió educativa), partits de prova
// contra bots, el rànquing, els bots (els seus .md es poden llegir per
// aprendre), el visor de partits amb les decisions i la línia del .md que les
// va causar, i les competicions (per a l'organitzador).
import { makeView, LESSONS } from './arena-views.js';

const $ = id => document.getElementById(id);
const T = key => (window.gregalT ? window.gregalT(key) : '') || key;
const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const api = (path, opts) => window.api(path, opts).then(async r => { const j = await r.json().catch(() => ({})); if (!r.ok) throw new Error(j.error || (typeof j === 'string' ? j : r.status)); return j; });
const lang = () => ((window.gregalLang && window.gregalLang()) || document.documentElement.lang || 'ca').startsWith('ca') ? 'ca' : 'en';
const post = (path, body) => api(path, { method: 'POST', body: JSON.stringify(body) });
const RUBRIC = ['clarity', 'coverage', 'adaptivity', 'specificity', 'consistency'];

const arenaUI = {
  data: null, game: 'nfl', view: null, match: null, poll: 0, comp: null,

  async load() {
    const page = $('arenaPage');
    if (!page || typeof window.api !== 'function') return;
    if (!page.dataset.ready) this.build(page);
    try { this.data = await api('/api/arena'); } catch (e) { $('arStatus').textContent = e.message; return; }
    if (!this.data.games.find(g => g.id === this.game) && this.data.games.length) this.game = this.data.games[0].id;
    this.paint();
  },

  build(page) {
    page.dataset.ready = '1';
    page.innerHTML = `<div class="ar">
      <header class="ar-head"><div><div class="page-kicker">${esc(T('ar.kicker'))}</div><h1 class="page-title">${esc(T('ar.title'))}</h1>
        <p class="page-copy">${esc(T('ar.lead'))}</p></div><div class="ar-me" id="arMe"></div></header>
      <nav class="ar-tabs" id="arTabs" role="tablist"></nav>
      <span id="arStatus" class="ar-status" role="status"></span>
      <div class="ar-grid">
        <section class="ar-card ar-editor">
          <div class="ar-row"><h2>${esc(T('ar.myStrategy'))}</h2><span id="arMeta" class="ar-muted"></span></div>
          <label class="ar-field">${esc(T('ar.display'))} <input id="arDisplay" maxlength="40"></label>
          <textarea id="arMD" spellcheck="false" placeholder="${esc(T('ar.mdPlaceholder'))}"></textarea>
          <div class="ar-row ar-actions"><span id="arCount" class="ar-muted"></span>
            <button type="button" id="arStarter">${esc(T('ar.loadStarter'))}</button>
            <button type="button" id="arReview">${esc(T('ar.review'))}</button>
            <button type="button" id="arSave" class="primary">${esc(T('ar.save'))}</button></div>
          <div class="ar-row ar-actions"><label>${esc(T('ar.practiceVs'))} <select id="arBot"></select></label>
            <button type="button" id="arPractice" class="primary">${esc(T('ar.practice'))}</button><span id="arLeft" class="ar-muted"></span></div>
          <div id="arReviewOut"></div>
        </section>
        <aside class="ar-side">
          <section class="ar-card"><h3>${esc(T('ar.leaderboard'))}</h3><ol id="arBoard" class="ar-board"></ol></section>
          <section class="ar-card"><h3>${esc(T('ar.bots'))}</h3><ul id="arBots" class="ar-bots"></ul></section>
          <section class="ar-card"><h3>${esc(T('ar.lessons'))}</h3><ol id="arLessons" class="ar-lessons"></ol></section>
        </aside>
      </div>
      <section class="ar-card" id="arMatchCard" hidden>
        <div class="ar-row"><h2 id="arMatchTitle"></h2><span class="ar-muted" id="arMatchStatus"></span>
          <span class="ar-ctl"><button type="button" id="arPause">⏸</button><button type="button" data-speed="1" class="on">1×</button><button type="button" data-speed="2">2×</button><button type="button" data-speed="4">4×</button><button type="button" id="arSkip">⏭</button><button type="button" id="arClose">✕</button></span></div>
        <div class="ar-match"><div class="ar-stage"><canvas id="arCanvas"></canvas></div><div class="ar-decisions" id="arDecisions"></div></div>
        <div class="ar-feedback" id="arFeedback"></div>
      </section>
      <div class="ar-grid2">
        <section class="ar-card"><h3>${esc(T('ar.matches'))}</h3><ul id="arMatches" class="ar-list"></ul></section>
        <section class="ar-card"><div class="ar-row"><h3>${esc(T('ar.competitions'))}</h3><a id="arStreamLink" target="_blank" rel="noopener">${esc(T('ar.openStream'))} ↗</a></div>
          <div id="arCompAdmin"></div><ul id="arComps" class="ar-list"></ul><div id="arComp"></div></section>
      </div>
      <section class="ar-card" id="arBroadcast" hidden><h3>${esc(T('ar.broadcast'))}</h3>
        <div class="ar-row"><b>${esc(T('ar.share'))}</b><span class="ar-muted">${esc(T('ar.shareHelp'))}</span></div>
        <div class="ar-row"><input id="arShareLink" readonly style="flex:1;min-width:240px"><button type="button" id="arShareCopy">${esc(T('ar.copy'))}</button><button type="button" id="arShareNew">${esc(T('ar.renew'))}</button></div>
        <p class="ar-muted">${esc(T('ar.obsHelp'))}</p>
        <div class="ar-row"><b>${esc(T('ar.voice'))}</b><span class="ar-muted" id="arVoiceState"></span></div>
        <div class="ar-row"><label>${esc(T('ar.voiceEngine'))} <select id="arVoiceEngine"><option value="piper">${esc(T('ar.voicePiper'))}</option><option value="http">${esc(T('ar.voiceHttp'))}</option><option value="off">${esc(T('ar.voiceOff'))}</option></select></label>
          <button type="button" id="arVoiceInstall" class="primary"></button><button type="button" id="arVoiceTestC">${esc(T('ar.testCaster'))}</button><button type="button" id="arVoiceTestA">${esc(T('ar.testAnalyst'))}</button></div>
        <div class="ar-row" id="arVoiceHttp" hidden><input id="arVoiceURL" placeholder="http://localhost:8004" style="flex:1"><input id="arVoiceModel" placeholder="model" style="width:140px"><input id="arVoiceC" placeholder="${esc(T('ar.voiceCasterName'))}" style="width:150px"><input id="arVoiceA" placeholder="${esc(T('ar.voiceAnalystName'))}" style="width:150px"><button type="button" id="arVoiceSave">${esc(T('ar.save'))}</button></div>
        <p class="ar-muted">${esc(T('ar.voiceHelp'))}</p>
      </section>
    </div>`;
    const md = $('arMD');
    md.addEventListener('input', () => { $('arCount').textContent = md.value.length + ' / 12000'; });
    $('arSave').onclick = async () => {
      try { await post('/api/arena', { action: 'save', game: this.game, md: md.value, display: $('arDisplay').value }); $('arStatus').textContent = T('ar.saved'); this.load(); }
      catch (e) { $('arStatus').textContent = e.message; }
    };
    $('arStarter').onclick = () => {
      const g = this.data.games.find(x => x.id === this.game);
      if (g && (!md.value.trim() || confirm(T('ar.replaceConfirm')))) { md.value = g.starter; md.dispatchEvent(new Event('input')); }
    };
    $('arReview').onclick = async () => {
      $('arReview').disabled = true; $('arReviewOut').innerHTML = `<p class="ar-muted">${esc(T('ar.reviewing'))}</p>`;
      try { const f = await post('/api/arena', { action: 'review', game: this.game, md: md.value }); $('arReviewOut').innerHTML = this.feedbackHTML(f, T('ar.reviewTitle')); }
      catch (e) { $('arReviewOut').textContent = e.message; }
      $('arReview').disabled = false;
    };
    $('arPractice').onclick = async () => {
      $('arPractice').disabled = true;
      try { const r = await post('/api/arena', { action: 'practice', game: this.game, opponent: $('arBot').value }); this.openMatch(r.id); this.load(); }
      catch (e) { $('arStatus').textContent = e.message; }
      $('arPractice').disabled = false;
    };
    $('arPause').onclick = () => { if (this.view) $('arPause').textContent = this.view.togglePause() ? '▶' : '⏸'; };
    page.querySelectorAll('[data-speed]').forEach(b => b.onclick = () => { this.view && this.view.setSpeed(+b.dataset.speed); page.querySelectorAll('[data-speed]').forEach(x => x.classList.toggle('on', x === b)); });
    $('arSkip').onclick = () => this.view && this.view.skipToEnd();
    $('arClose').onclick = () => this.closeMatch();
    this.buildBroadcast();
    $('arStreamLink').href = '/app/arena-stream.html#lang=' + lang() + (() => { let t = ''; try { t = localStorage.getItem('gregal_token') || ''; } catch (e) {} return t ? '&token=' + encodeURIComponent(t) : ''; })();
  },

  paint() {
    const d = this.data, g = d.games.find(x => x.id === this.game);
    $('arMe').innerHTML = `<b>${esc(d.me)}</b>${d.admin ? ` · <span class="ar-badge">${esc(T('ar.organiser'))}</span>` : ''}`;
    $('arTabs').innerHTML = d.games.map(x => `<button type="button" role="tab" data-g="${esc(x.id)}" class="${x.id === this.game ? 'on' : ''}">${esc(x.title)}</button>`).join('');
    $('arTabs').querySelectorAll('[data-g]').forEach(b => b.onclick = () => { this.game = b.dataset.g; $('arReviewOut').innerHTML = ''; this.paint(); });
    const mine = d.mine[this.game];
    if (document.activeElement !== $('arMD')) { $('arMD').value = mine ? mine.md : ''; $('arMD').dispatchEvent(new Event('input')); }
    if (document.activeElement !== $('arDisplay')) $('arDisplay').value = mine ? mine.display : d.me;
    $('arMeta').textContent = mine ? T('ar.updated').replace('{when}', new Date(mine.updated * 1000).toLocaleString()).replace('{n}', mine.versions) : T('ar.noStrategy');
    const bots = d.bots[this.game] || [];
    $('arBot').innerHTML = bots.map(b => `<option value="${esc(b.owner)}">${esc(b.display)} (${esc(T('ar.level.' + b.level))})</option>`).join('') || `<option value="">${esc(T('ar.noBots'))}</option>`;
    $('arPractice').disabled = !bots.length || !mine;
    $('arLeft').textContent = T('ar.left').replace('{n}', Math.max(0, d.practice_left));
    const board = d.leaderboard[this.game] || [];
    $('arBoard').innerHTML = board.map((r, i) => `<li class="${r.owner === d.me ? 'me' : ''}"><span class="pos">${i + 1}</span><span class="who">${esc(r.display)}${r.bot ? ' 🤖' : ''}</span><b>${r.elo}</b><small>${r.won}-${r.lost}${r.drawn ? '-' + r.drawn : ''}</small></li>`).join('') || `<li class="none">${esc(T('ar.noRanking'))}</li>`;
    $('arBots').innerHTML = bots.map(b => `<li><b>${esc(b.display)}</b> <small>${esc(T('ar.level.' + b.level))}</small><button type="button" data-md="${esc(b.owner)}">${esc(T('ar.readMD'))}</button></li>`).join('') +
      (d.admin ? `<li><button type="button" id="arSeed">${esc(T(bots.length ? 'ar.reseed' : 'ar.seed'))}</button></li>` : '') || `<li class="none">${esc(T('ar.noBots'))}</li>`;
    $('arBots').querySelectorAll('[data-md]').forEach(b => b.onclick = () => { const bot = bots.find(x => x.owner === b.dataset.md); this.showMD(bot.display, bot.md); });
    if ($('arSeed')) $('arSeed').onclick = async () => { $('arSeed').disabled = true; await post('/api/arena', { action: 'seed_bots', games: [this.game] }).catch(e => { $('arStatus').textContent = e.message; }); $('arStatus').textContent = T('ar.seeding'); setTimeout(() => this.load(), 20000); };
    $('arLessons').innerHTML = LESSONS[lang()].map(([h, p]) => `<li><b>${esc(h)}</b><span>${esc(p)}</span></li>`).join('');
    $('arMatches').innerHTML = (d.matches || []).filter(m => m.game === this.game).map(m => `<li data-id="${esc(m.id)}"><b>${esc(m.names[0])} ${m.score[0]} - ${m.score[1]} ${esc(m.names[1])}</b><small>${esc(T('ar.kind.' + m.kind))}${m.stage && m.kind === 'official' ? ' · ' + esc(m.stage) : ''} · ${esc(T('ar.status.' + m.status))}</small></li>`).join('') || `<li class="none">${esc(T('ar.noMatches'))}</li>`;
    $('arMatches').querySelectorAll('[data-id]').forEach(li => li.onclick = () => this.openMatch(li.dataset.id));
    // Competicions.
    $('arBroadcast').hidden = !d.admin;
    if (d.admin && !this.broadcastLoaded) { this.broadcastLoaded = true; this.share(false); this.voiceStatus(); }
    $('arCompAdmin').innerHTML = d.admin ? `<div class="ar-row ar-actions"><input id="arCompTitle" placeholder="${esc(T('ar.compTitle'))}"><label><input type="checkbox" id="arCompBots"> ${esc(T('ar.withBots'))}</label><button type="button" id="arCompCreate">${esc(T('ar.compCreate'))}</button></div>` : '';
    if ($('arCompCreate')) $('arCompCreate').onclick = async () => {
      try { const r = await post('/api/arena/competition', { action: 'create', title: $('arCompTitle').value, bots: $('arCompBots').checked }); this.openComp(r.id); this.load(); }
      catch (e) { $('arStatus').textContent = e.message; }
    };
    $('arComps').innerHTML = (d.competitions || []).map(c => `<li data-c="${esc(c.id)}"><b>${esc(c.title)}</b><small>${esc(T('ar.compStatus.' + c.status))} · ${esc((c.games || []).join(', '))}</small></li>`).join('') || `<li class="none">${esc(T('ar.noComps'))}</li>`;
    $('arComps').querySelectorAll('[data-c]').forEach(li => li.onclick = () => this.openComp(li.dataset.c));
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

  closeMatch() { if (this.view) this.view.destroy(); this.view = null; this.match = null; clearTimeout(this.poll); $('arMatchCard').hidden = true; },

  async openMatch(id) {
    this.closeMatch();
    const card = $('arMatchCard');
    card.hidden = false; card.scrollIntoView({ behavior: 'smooth', block: 'start' });
    let first;
    try { first = await api('/api/arena/match?id=' + encodeURIComponent(id) + '&after=0'); } catch (e) { $('arStatus').textContent = e.message; return; }
    this.match = first;
    $('arMatchTitle').textContent = `${first.names[0]} vs ${first.names[1]}`;
    const fetchMatch = after => api('/api/arena/match?id=' + encodeURIComponent(id) + '&after=' + after).then(m => { this.updateMatch(m); return m; });
    try {
      this.view = await makeView(first.game, $('arCanvas'), { lang: lang(), fetchMatch, onEvent: e => this.highlight(e) });
    } catch (e) { $('arStatus').textContent = e.message; }
    this.updateMatch(first);
  },

  updateMatch(m) {
    if (!this.match || m.id !== this.match.id) return;
    this.match.status = m.status; this.match.score = m.score; this.match.feedback = m.feedback;
    this.match.decisions = m.decisions || this.match.decisions; this.match.md = m.md || this.match.md;
    $('arMatchStatus').textContent = T('ar.status.' + m.status) + ' · ' + m.score[0] + ' - ' + m.score[1];
    const names = this.match.names;
    const decs = this.match.decisions || [];
    $('arDecisions').innerHTML = `<h4>${esc(T('ar.decisions'))}</h4>` + decs.slice(-400).map(d => `<div class="ar-dec side${d.side}" data-n="${d.n}"><b>${esc(names[d.side])}</b> <small>d${d.n} · ${esc(d.short)}</small>
      <code>${esc(JSON.stringify(d.decision || {}))}</code>${d.fallback ? `<em>${esc(T('ar.fallback'))}</em>` : ''}${d.why ? `<span>${esc(d.why)}</span>` : ''}${d.rule ? `<q>${esc(d.rule)}</q>` : ''}</div>`).join('');
    if (m.status === 'final' && m.feedback) {
      $('arFeedback').innerHTML = [0, 1].map(s => m.feedback[s] ? this.feedbackHTML(m.feedback[s], T('ar.reportFor').replace('{name}', names[s])) : '').join('') ||
        `<p class="ar-muted">${esc(T('ar.reportPending'))}</p>`;
      if (!m.feedback[0] && !m.feedback[1]) { clearTimeout(this.poll); this.poll = setTimeout(() => fetchAgain(), 8000); }
    }
    const fetchAgain = () => api('/api/arena/match?id=' + encodeURIComponent(m.id) + '&after=999999').then(x => this.updateMatch(x)).catch(() => {});
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
      h += '<ul class="ar-list">' + c.schedule.filter(s => s.game === g).map((s, i) => `<li ${s.match ? `data-id="${esc(s.match)}"` : ''}><b>${esc(s.stage)}</b> ${s.pending ? esc(s.from.join(' vs ')) : `${esc(name(s.a))} ${s.done ? s.score[0] + ' - ' + s.score[1] : 'vs'} ${esc(name(s.b))}`}</li>`).join('') + '</ul>';
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
