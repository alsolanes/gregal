// Primera pàgina d'un projecte: què hi ha pendent ara mateix (canvis sense
// desar, agents treballant, objectius oberts) i les converses recents per
// continuar-les. Els suggeriments genèrics només surten en un projecte que
// encara no té historial. També manté els comptadors de la barra lateral.
(function () {
  const $ = id => document.getElementById(id);
  const G = () => window.gregal;
  const P = () => window.gregalPanels;
  const T = (key, fallback) => {
    try { const v = window.gregalT && window.gregalT(key); if (v && v !== key) return v; } catch (e) {}
    return fallback;
  };
  const esc = s => String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const norm = p => String(p || '').replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase();
  const count = (n, one, many) => (n === 1 ? T(one, '') : T(many, '').replace('{n}', n));

  let diff = null, goals = [], lastDiffAt = 0, lastGoalsAt = 0;

  function relTime(iso) {
    const t = Date.parse(iso || '');
    if (!t) return '';
    const s = Math.round((t - Date.now()) / 1000);
    const lang = document.documentElement.lang || 'en';
    const rtf = new Intl.RelativeTimeFormat(lang, { numeric: 'auto' });
    const a = Math.abs(s);
    if (a < 60) return rtf.format(Math.round(s), 'second');
    if (a < 3600) return rtf.format(Math.round(s / 60), 'minute');
    if (a < 86400) return rtf.format(Math.round(s / 3600), 'hour');
    return rtf.format(Math.round(s / 86400), 'day');
  }

  async function loadDiff() {
    try {
      const r = await G().api('/api/diff');
      if (r.ok) diff = await r.json();
    } catch (e) {}
    lastDiffAt = Date.now();
  }
  async function loadGoals() {
    try {
      const r = await G().api('/api/goal');
      if (r.ok) goals = ((await r.json()).goals || []).filter(g => g.status !== 'fet');
    } catch (e) {}
    lastGoalsAt = Date.now();
  }
  const busySessions = () => ((P() && P().sessions && P().sessions._lastList) || []).filter(s => s.busy).length;

  function paintBadges() {
    const files = diff && Array.isArray(diff.files) ? diff.files.length : 0;
    const cb = $('navChangesBadge');
    if (cb) { cb.hidden = !files; cb.textContent = files; }
    const busy = busySessions();
    const ab = $('navAgentsBadge');
    if (ab) { ab.hidden = !busy; ab.textContent = busy; }
  }

  function heroVisible() {
    const hero = $('hero');
    return !!hero && !hero.hidden && hero.style.display !== 'none';
  }

  // Les converses i els suggeriments viuen sota el compositor, fora del
  // #hero: es mostren i s'amaguen amb ell.
  function syncMore() {
    const more = $('heroMore');
    if (more) more.hidden = !heroVisible();
  }

  // Inici: una salutació segons l'hora en comptes de «Què vols fer?».
  function greet() {
    const h = $('heroTitle');
    if (!h) return;
    const hour = new Date().getHours();
    const key = hour >= 6 && hour < 14 ? 'hero.morning' : hour >= 14 && hour < 21 ? 'hero.afternoon' : 'hero.evening';
    h.textContent = T(key, T('hero.title', 'What do you want to do?'));
  }

  function render() {
    paintBadges();
    syncMore();
    greet();
    const box = $('homeContext');
    if (!box || !heroVisible()) return;
    const st = (G() && G().state) || {};
    const cwd = norm(st.cwd);
    const project = (st.cwd || '').split(/[\\/]/).filter(Boolean).pop() || '';

    const pills = [];
    const files = diff && Array.isArray(diff.files) ? diff.files.length : 0;
    if (files) {
      const stat = (diff.added || diff.removed) ? ' · +' + (diff.added || 0) + ' −' + (diff.removed || 0) : '';
      pills.push('<button type="button" class="home-pill" data-go="canvis"><span class="home-pill-dot changes"></span>' +
        esc(count(files, 'home.changesOne', 'home.changes')) + esc(stat) + '</button>');
    }
    const busy = busySessions();
    if (busy) pills.push('<button type="button" class="home-pill" data-go="workshop"><span class="home-pill-dot busy"></span>' +
      esc(count(busy, 'home.agentsOne', 'home.agents')) + '</button>');
    if (goals.length) pills.push('<button type="button" class="home-pill" data-go="objectius"><span class="home-pill-dot goals"></span>' +
      esc(count(goals.length, 'home.goalsOne', 'home.goals')) + '</button>');

    const convs = ((P() && P().convs && P().convs.items) || [])
      .filter(c => !c.current && cwd && norm(c.workspace) === cwd)
      .sort((a, b) => String(b.at || '').localeCompare(String(a.at || '')))
      .slice(0, 3);

    let html = '';
    if (pills.length) html += '<div class="home-pills">' + pills.join('') + '</div>';
    if (convs.length) {
      html += '<div class="home-recent"><div class="home-recent-head">' + esc(T('home.continue', 'Continue in {project}').replace('{project}', project)) + '</div>' +
        convs.map(c => '<button type="button" class="home-recent-row" data-conv="' + esc(c.name) + '"><span class="home-recent-t">' + esc(c.title || c.name) +
          '</span><span class="home-recent-w">' + esc(relTime(c.at) || c.when || '') + '</span></button>').join('') + '</div>';
    }
    box.innerHTML = html;
    box.hidden = !html;
    // Suggeriments genèrics: només si encara no hi ha res a continuar.
    const chips = $('chips');
    if (chips) chips.hidden = convs.length > 0;
  }

  async function refresh(force) {
    if (!G() || !G().api) return;
    const tasks = [];
    if (force || Date.now() - lastDiffAt > 15000) tasks.push(loadDiff());
    if (heroVisible() && (force || Date.now() - lastGoalsAt > 15000)) tasks.push(loadGoals());
    await Promise.all(tasks);
    render();
  }

  function wire() {
    const box = $('homeContext');
    if (box) box.addEventListener('click', e => {
      const go = e.target.closest('[data-go]');
      if (go) { G().setView(go.dataset.go); return; }
      const conv = e.target.closest('[data-conv]');
      if (conv && P() && P().convs) P().convs.obre(conv.dataset.conv);
    });
    const hero = $('hero');
    if (hero) new MutationObserver(() => { syncMore(); if (heroVisible()) refresh(true); }).observe(hero, { attributes: true, attributeFilter: ['style', 'hidden'] });
    document.addEventListener('gregal:idioma', render);
    setInterval(() => refresh(false), 5000);
    refresh(true);
  }

  function start() {
    if (G() && G().api && P()) { wire(); return; }
    setTimeout(start, 200);
  }
  start();
})();
