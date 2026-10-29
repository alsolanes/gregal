// Partits de futbol americà entre els vents (football.go + football-view.js):
// les dues franquícies, el botó per jugar-ne un, la llista de partits i el
// visor en directe (pausa, velocitat, salta al final), amb les notes dels
// entrenadors i, al final, la crònica i el millor jugador.
import { FootballView } from './football-view.js';

const $ = id => document.getElementById(id);
const T = key => (window.gregalT ? window.gregalT(key) : '') || key;
const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const api = (path, opts) => window.api(path, opts).then(async r => { const j = await r.json().catch(() => ({})); if (!r.ok) throw new Error(j.error || r.status); return j; });
const lang = () => ((window.gregalLang && window.gregalLang()) || document.documentElement.lang || 'ca').startsWith('ca') ? 'ca' : 'en';

const games = {
  view: null, current: '', data: null,

  async load() {
    const page = $('partitsPage');
    if (!page || typeof window.api !== 'function') return;
    if (!page.dataset.ready) this.build(page);
    try { this.data = await api('/api/football'); } catch (e) { $('fbStatus').textContent = e.message; return; }
    this.paint();
    const live = (this.data.games || []).find(g => g.status === 'live' || g.status === 'pregame');
    if (!this.current && this.data.games && this.data.games.length) this.open((live || this.data.games[0]).id);
  },

  build(page) {
    page.dataset.ready = '1';
    page.innerHTML = `<div class="fb">
      <header class="fb-head"><div><div class="page-kicker">${esc(T('fb.kicker'))}</div><h1 class="page-title">${esc(T('fb.title'))}</h1>
        <p class="page-copy">${esc(T('fb.lead'))}</p></div>
        <div class="fb-actions"><span id="fbStatus" class="fb-status" role="status"></span><button type="button" id="fbPlay" class="primary"></button></div></header>
      <div id="fbTeams" class="fb-teams"></div>
      <div class="fb-stage"><canvas id="fbCanvas"></canvas>
        <div class="fb-controls"><button type="button" id="fbPause">⏸</button><button type="button" data-speed="1" class="on">1×</button><button type="button" data-speed="2">2×</button><button type="button" data-speed="4">4×</button><button type="button" id="fbSkip">${esc(T('fb.skip'))}</button></div></div>
      <div class="fb-below"><div id="fbNotes" class="fb-notes"></div><div><h3>${esc(T('fb.games'))}</h3><ol id="fbGames" class="fb-games"></ol></div></div>
    </div>`;
    $('fbPlay').onclick = async () => {
      const running = this.runningGame();
      $('fbPlay').disabled = true;
      try {
        if (running) await api('/api/football/match?id=' + encodeURIComponent(running.id), { method: 'POST', body: '{"action":"stop"}' });
        else { const r = await api('/api/football', { method: 'POST', body: JSON.stringify({ action: 'play' }) }); this.current = ''; await this.load(); this.open(r.id); }
      } catch (e) { $('fbStatus').textContent = e.message; }
      $('fbPlay').disabled = false;
      this.load();
    };
    $('fbPause').onclick = () => { if (this.view) $('fbPause').textContent = this.view.togglePause() ? '▶' : '⏸'; };
    page.querySelectorAll('[data-speed]').forEach(b => b.onclick = () => { this.view && this.view.setSpeed(+b.dataset.speed); page.querySelectorAll('[data-speed]').forEach(x => x.classList.toggle('on', x === b)); });
    $('fbSkip').onclick = () => this.view && this.view.skipToEnd();
  },

  runningGame() { return (this.data && this.data.games || []).find(g => g.status === 'live' || g.status === 'pregame'); },

  paint() {
    const d = this.data, running = this.runningGame();
    $('fbPlay').textContent = T(running ? 'fb.stop' : 'fb.play');
    $('fbPlay').classList.toggle('danger', !!running);
    $('fbStatus').textContent = d.error || T('fb.today').replace('{n}', d.today || 0).replace('{max}', d.max_per_day || 3);
    $('fbTeams').innerHTML = (d.teams || []).map(t => `<div class="fb-team" style="--c:${esc(t.color)};--c2:${esc(t.color2)}"><b>${esc(t.name)}</b><span>${esc(t.record)}</span>
      <small>${esc(T('fb.stars'))}: ${esc((t.players || []).filter(p => p.wind).map(p => p.name + ' (' + p.pos + ')').join(', '))}</small></div>`).join('<div class="fb-vs">vs</div>');
    $('fbGames').innerHTML = (d.games || []).map(g => `<li data-id="${esc(g.id)}" class="${g.id === this.current ? 'cur' : ''}"><b>${esc(g.teams[0].abbr)} ${g.score[0]} - ${g.score[1]} ${esc(g.teams[1].abbr)}</b>
      <small>${esc(T('fb.status.' + g.status))} · ${esc(new Date(g.started * 1000).toLocaleString())}</small></li>`).join('') || `<li class="none">${esc(T('fb.none'))}</li>`;
    $('fbGames').querySelectorAll('[data-id]').forEach(li => li.onclick = () => this.open(li.dataset.id));
  },

  open(id) {
    if (this.view) this.view.destroy();
    this.current = id;
    const source = { load: after => api('/api/football/match?id=' + encodeURIComponent(id) + '&after=' + after) };
    this.view = new FootballView($('fbCanvas'), { lang: lang(), source, onUpdate: m => this.notes(m) });
    if (this.data) this.paint();
  },

  notes(m) {
    let h = '';
    if (m.status === 'final' && m.summary) {
      const mvp = (m.teams || []).flatMap(t => t.players || []).find(p => p.id === m.mvp);
      h += `<div class="fb-recap"><b>${esc(T('fb.recap'))}</b><p>${esc(m.summary)}</p>${mvp ? `<p>⭐ ${esc(T('fb.mvp'))}: <b>${esc(mvp.name)}</b> (${esc(mvp.pos)})</p>` : ''}</div>`;
    }
    const plans = (m.plans || []).slice(-6).reverse();
    if (plans.length) h += `<h3>${esc(T('fb.coaches'))}</h3>` + plans.map(p => `<p class="fb-plan"><b>${esc(p.coach)}</b> · Q${p.q}: ${esc(p.note)}</p>`).join('');
    $('fbNotes').innerHTML = h;
    if (['final', 'stopped'].includes(m.status) && this.runningGame()) this.load();
  },
};

window.gregalGames = games;
