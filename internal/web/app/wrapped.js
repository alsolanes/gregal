// Gregal Wrapped: la setmana (o el mes) en xifres, amb dades reals de
// /api/stats. Una targeta a la pàgina d'inici un cop per setmana (es pot
// descartar o apagar a Preferències) i una vista completa a Més → Estadístiques
// amb una imatge PNG per compartir, dibuixada al navegador.
import './fun-core.js';

const fun = window.gregalFunCore;
const G = () => window.gregal;
// Dates en l'idioma de la interfície, no en el del navegador.
const uiLocale = () => document.documentElement.lang || undefined;
const T = (key, vars) => {
  let s = (window.gregalT && window.gregalT(key)) || key;
  for (const k in vars || {}) s = s.split('{' + k + '}').join(vars[k]);
  return s;
};
const KEY_ON = 'gregal_fun_wrapped';
const KEY_DISMISSED = 'gregal_wrapped_dismissed';
const KINDS = ['read', 'search', 'edit', 'shell', 'web', 'delegate', 'mcp', 'other'];

const ls = {
  get(k, d) { try { const v = localStorage.getItem(k); return v == null ? d : v; } catch (e) { return d; } },
  set(k, v) { try { localStorage.setItem(k, v); } catch (e) {} },
};
export const wrappedEnabled = () => ls.get(KEY_ON, '1') !== '0';

const st = { dialog: null, range: 'week', data: null, card: null, cardWeek: '' };

function el(tag, cls, text) {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text != null) n.textContent = text;
  return n;
}
function button(cls, text, label, onClick) {
  const b = el('button', cls, text);
  b.type = 'button';
  if (label) { b.setAttribute('aria-label', label); b.title = label; }
  if (onClick) b.addEventListener('click', onClick);
  return b;
}
async function fetchStats(range) {
  const r = await G().api('/api/stats?range=' + range);
  if (!r.ok) throw new Error((await r.text()).trim() || 'HTTP ' + r.status);
  return r.json();
}
const fmtCost = usd => (usd < 0.01 ? '$' + usd.toFixed(4) : usd < 10 ? '$' + usd.toFixed(3) : '$' + usd.toFixed(2));
const fmtDate = iso => { const d = new Date(iso); return isNaN(d) ? '' : d.toLocaleDateString(uiLocale(), { month: 'short', day: 'numeric' }); };
const KIND_KEYS = { read: 'wrapped.kind.read', search: 'wrapped.kind.search', edit: 'wrapped.kind.edit', shell: 'wrapped.kind.shell', web: 'wrapped.kind.web', delegate: 'wrapped.kind.delegate', mcp: 'wrapped.kind.mcp', other: 'wrapped.kind.other' };
const kindLabel = k => T(KIND_KEYS[k] || 'wrapped.kind.other');

// --- vista completa ---

function tiles(s) {
  const list = [
    ['wrapped.turns', s.turns], ['wrapped.sessions', s.sessions], ['wrapped.toolCalls', s.tool_calls],
    ['wrapped.files', s.files_changed], ['wrapped.green', s.tests_turned_green],
    ['wrapped.streak', T('wrapped.days', { n: s.longest_streak_days })],
  ];
  if (s.tokens && s.tokens.known) {
    list.push(['wrapped.tokens', '↑' + fun.fmtTokens(s.tokens.up) + ' · ↓' + fun.fmtTokens(s.tokens.down)]);
    list.push(['wrapped.cost', s.tokens.cost_usd != null ? fmtCost(s.tokens.cost_usd) : T('wrapped.unknown')]);
  } else list.push(['wrapped.tokens', T('wrapped.unknown')]);
  list.push(['wrapped.model', s.favorite_model || T('wrapped.unknown')]);
  const grid = el('div', 'wrapped-tiles');
  for (const [k, v] of list) {
    const t = el('div', 'wrapped-tile');
    t.append(el('span', 'wrapped-num', String(v)), el('span', 'wrapped-lab', T(k)));
    grid.appendChild(t);
  }
  return grid;
}

function dayBars(s) {
  const box = el('section', 'wrapped-section');
  box.appendChild(el('h3', '', T('wrapped.activity')));
  const max = Math.max(1, ...s.days.map(d => d.turns));
  const list = el('ol', 'wrapped-bars');
  list.setAttribute('aria-label', T('wrapped.activity'));
  for (const d of s.days) {
    const li = el('li', 'wrapped-bar');
    const date = new Date(d.date + 'T12:00:00');
    const name = isNaN(date) ? d.date : date.toLocaleDateString(uiLocale(), s.days.length > 7 ? { day: 'numeric' } : { weekday: 'short' });
    li.setAttribute('aria-label', name + ': ' + T('wrapped.turnsN', { n: d.turns }));
    li.title = li.getAttribute('aria-label');
    const fill = el('span', 'wrapped-bar-fill');
    fill.style.height = Math.round((d.turns / max) * 100) + '%';
    li.append(fill, el('span', 'wrapped-bar-day', name));
    list.appendChild(li);
  }
  box.appendChild(list);
  return box;
}

function kindBars(s) {
  const box = el('section', 'wrapped-section');
  box.appendChild(el('h3', '', T('wrapped.byKind')));
  const total = Math.max(1, s.tool_calls);
  const kinds = KINDS.filter(k => (s.tools_by_kind || {})[k] > 0);
  if (!kinds.length) { box.appendChild(el('p', 'wrapped-muted', T('wrapped.noTools'))); return box; }
  const list = el('ul', 'wrapped-kinds');
  for (const k of kinds) {
    const n = s.tools_by_kind[k];
    const li = el('li', 'wrapped-kind');
    const bar = el('span', 'wrapped-kind-bar');
    const fill = el('span', 'wrapped-kind-fill'); fill.style.width = Math.max(2, Math.round((n / total) * 100)) + '%';
    bar.appendChild(fill);
    li.append(el('span', 'wrapped-kind-name', kindLabel(k)), bar, el('span', 'wrapped-kind-n', String(n)));
    list.appendChild(li);
  }
  box.appendChild(list);
  if (s.top_tools && s.top_tools.length) box.appendChild(el('p', 'wrapped-muted', T('wrapped.topTools') + ': ' + s.top_tools.map(x => x.name + ' ×' + x.count).join(', ')));
  return box;
}

function stubborn(s) {
  const box = el('section', 'wrapped-section');
  box.appendChild(el('h3', '', T('wrapped.stubborn')));
  const b = s.stubborn_bug;
  if (!b) { box.appendChild(el('p', 'wrapped-muted', T('wrapped.stubbornNone'))); return box; }
  const p = el('p', 'wrapped-stubborn');
  p.append(el('code', '', b.command), document.createTextNode(' — ' + T('wrapped.stubbornText', { n: b.failures, run: b.run_id, d: fmtDate(b.at) })));
  box.appendChild(p);
  return box;
}

function render() {
  const d = st.dialog;
  const body = d.querySelector('.wrapped-body');
  d.querySelectorAll('.wrapped-range button').forEach(b => {
    const on = b.dataset.range === st.range;
    b.classList.toggle('on', on); b.setAttribute('aria-checked', String(on));
  });
  const s = st.data;
  if (!s) return;
  const sub = d.querySelector('.wrapped-sub');
  sub.textContent = fmtDate(s.from) + ' – ' + fmtDate(s.to);
  const hl = fun.wrappedHighlights(s, T);
  const intro = el('div', 'wrapped-intro');
  if (hl.length) {
    const ul = el('ul', 'wrapped-highlights');
    hl.forEach(h => ul.appendChild(el('li', '', h)));
    intro.appendChild(ul);
  } else intro.appendChild(el('p', 'wrapped-muted', T('wrapped.empty')));
  body.replaceChildren(intro, tiles(s), dayBars(s), kindBars(s), stubborn(s));
  if (s.tokens && s.tokens.known) body.appendChild(el('p', 'wrapped-note', T('wrapped.estimated')));
}

async function load() {
  const body = st.dialog.querySelector('.wrapped-body');
  body.replaceChildren(el('p', 'wrapped-muted', T('wrapped.loading')));
  try { st.data = await fetchStats(st.range); render(); }
  catch (e) { body.replaceChildren(el('p', 'wrapped-muted', T('wrapped.failed') + ' ' + e.message)); }
}

function ensureDialog() {
  if (st.dialog) return st.dialog;
  const d = el('dialog', 'wrapped-dialog');
  d.setAttribute('aria-labelledby', 'wrappedTitle');
  const head = el('header', 'wrapped-head');
  const title = el('div', 'wrapped-titles');
  const h = el('h2', '', T('wrapped.title')); h.id = 'wrappedTitle';
  title.append(h, el('span', 'wrapped-sub'));
  const range = el('div', 'wrapped-range'); range.setAttribute('role', 'radiogroup'); range.setAttribute('aria-label', T('wrapped.range'));
  for (const [r, key] of [['week', 'wrapped.week'], ['month', 'wrapped.month']]) {
    const b = button('', T(key), '', () => { st.range = r; load(); });
    b.dataset.range = r; b.setAttribute('role', 'radio');
    range.appendChild(b);
  }
  head.append(title, range,
    button('wrapped-png', T('wrapped.png'), T('wrapped.pngHelp'), () => downloadPNG()),
    button('wrapped-icon', '✕', T('wrapped.close'), () => d.close()));
  d.append(head, el('div', 'wrapped-body'));
  (document.getElementById('retroMount') || document.body).appendChild(d);
  st.dialog = d;
  return d;
}

export function openStats(range) {
  const d = ensureDialog();
  if (range) st.range = range;
  if (!d.open) d.showModal();
  load();
}

// --- PNG per compartir (canvas, colors del tema actiu) ---

function cssVar(name, fallback) {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v || fallback;
}
function wrapText(ctx, text, maxW) {
  const words = String(text).split(/\s+/), lines = [];
  let line = '';
  for (const w of words) {
    const test = line ? line + ' ' + w : w;
    if (ctx.measureText(test).width > maxW && line) { lines.push(line); line = w; } else line = test;
  }
  if (line) lines.push(line);
  return lines;
}
export function drawShareCard(canvas, s) {
  const W = 1200, H = 630;
  canvas.width = W; canvas.height = H;
  const ctx = canvas.getContext('2d');
  const bg = cssVar('--bg', 'black'), surface = cssVar('--surface', 'black'), ink = cssVar('--ink', 'white');
  const muted = cssVar('--muted', 'gray'), accent = cssVar('--accent', 'teal'), line = cssVar('--line', 'gray');
  ctx.fillStyle = bg; ctx.fillRect(0, 0, W, H);
  // Ratlles de vent a 45° (NE): la marca, discreta.
  ctx.save(); ctx.strokeStyle = accent; ctx.globalAlpha = 0.18; ctx.lineWidth = 6; ctx.lineCap = 'round';
  for (let i = 0; i < 7; i++) { const x = 760 + i * 60, y = 600 - i * 18; ctx.beginPath(); ctx.moveTo(x, y); ctx.lineTo(x + 160, y - 160); ctx.stroke(); }
  ctx.restore();
  ctx.fillStyle = ink; ctx.font = '700 52px system-ui, sans-serif'; ctx.textBaseline = 'top';
  ctx.fillText(T(s.range === 'month' ? 'wrapped.shareTitleMonth' : 'wrapped.shareTitleWeek'), 60, 54);
  ctx.fillStyle = muted; ctx.font = '500 24px system-ui, sans-serif';
  ctx.fillText(fmtDate(s.from) + ' – ' + fmtDate(s.to), 60, 120);
  const big = [[s.turns, 'wrapped.turns'], [s.tool_calls, 'wrapped.toolCalls'], [s.files_changed, 'wrapped.files'], [s.tests_turned_green, 'wrapped.green']];
  big.forEach(([n, k], i) => {
    const x = 60 + i * 272, y = 180;
    ctx.fillStyle = surface; ctx.strokeStyle = line; ctx.lineWidth = 2;
    ctx.beginPath(); if (ctx.roundRect) ctx.roundRect(x, y, 248, 140, 18); else ctx.rect(x, y, 248, 140); ctx.fill(); ctx.stroke();
    ctx.fillStyle = accent; ctx.font = '800 56px system-ui, sans-serif'; ctx.fillText(String(n || 0), x + 22, y + 22);
    ctx.fillStyle = muted; ctx.font = '500 21px system-ui, sans-serif'; ctx.fillText(T(k), x + 22, y + 96);
  });
  ctx.fillStyle = ink; ctx.font = '500 25px system-ui, sans-serif';
  let y = 356;
  const extra = fun.wrappedHighlights(s, T).filter((_, i) => i > 0).slice(0, 4);
  for (const h of extra) {
    for (const l of wrapText(ctx, '• ' + h, 1060).slice(0, 2)) { ctx.fillText(l, 60, y); y += 34; }
    if (y > 520) break;
  }
  ctx.fillStyle = muted; ctx.font = '600 22px system-ui, sans-serif';
  ctx.fillText('Gregal · 45° NE', 60, H - 56);
  return canvas;
}

function downloadPNG() {
  const s = st.data;
  if (!s) return;
  const c = drawShareCard(document.createElement('canvas'), s);
  c.toBlob(blob => {
    if (!blob) return;
    const url = URL.createObjectURL(blob);
    const a = el('a'); a.href = url; a.download = 'gregal-wrapped-' + (s.range === 'month' ? 'month' : fun.weekKey(new Date())) + '.png';
    document.body.appendChild(a); a.click(); a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 4000);
  }, 'image/png');
}

// --- targeta a la pàgina d'inici ---

function heroVisible() {
  const hero = document.getElementById('hero');
  return !!hero && !hero.hidden && hero.style.display !== 'none';
}

async function maybeCard() {
  const hero = document.getElementById('hero');
  if (!hero || !heroVisible() || !G() || !G().api) return;
  const week = fun.weekKey(new Date());
  if (!wrappedEnabled() || ls.get(KEY_DISMISSED, '') === week) { if (st.card) st.card.hidden = true; return; }
  if (st.card && st.cardWeek === week) { st.card.hidden = false; return; }
  let s;
  try { s = await fetchStats('week'); } catch (e) { return; }
  if (!fun.wrappedDue({ enabled: wrappedEnabled(), stats: s, dismissed: ls.get(KEY_DISMISSED, ''), now: new Date() })) return;
  const card = st.card || el('section', 'wrapped-card');
  card.setAttribute('aria-labelledby', 'wrappedCardTitle');
  const h = el('h2', 'wrapped-card-title', T('wrapped.cardTitle')); h.id = 'wrappedCardTitle';
  const ul = el('ul', 'wrapped-card-list');
  fun.wrappedHighlights(s, T).slice(0, 3).forEach(x => ul.appendChild(el('li', '', x)));
  const acts = el('div', 'wrapped-card-acts');
  acts.append(
    button('primary', T('wrapped.seeAll'), '', () => openStats('week')),
    button('ghost', T('wrapped.dismiss'), T('wrapped.dismissHelp'), () => { ls.set(KEY_DISMISSED, week); card.hidden = true; }),
  );
  card.replaceChildren(h, ul, acts);
  card.hidden = false;
  if (!st.card) {
    const ctx = document.getElementById('homeContext');
    if (ctx && ctx.parentNode) ctx.parentNode.insertBefore(card, ctx); else hero.appendChild(card);
    st.card = card;
  }
  st.cardWeek = week;
}

function navItem() {
  const more = document.querySelector('#sideMore .rail-more-menu') || document.getElementById('sideMore');
  if (!more || more.querySelector('[data-stats-open]')) return;
  // side-view dona l'estil de la resta d'entrades (sense, el botó sortia
  // amb el fons blanc per defecte del navegador).
  const b = el('button', 'nav-item side-view');
  b.type = 'button'; b.dataset.statsOpen = '1';
  const icon = el('span', 'nav-icon');
  icon.innerHTML = '<svg class="ui-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 20h16 M6 16l4-5 3 3 5-7 M18 7h-3 M18 7v3"/></svg>';
  b.append(icon, el('span', 'stats-nav-label', T('wrapped.nav')));
  b.addEventListener('click', () => openStats());
  more.appendChild(b);
}

export function boot() {
  navItem();
  const hero = document.getElementById('hero');
  if (hero) new MutationObserver(() => { if (heroVisible()) maybeCard(); }).observe(hero, { attributes: true, attributeFilter: ['style', 'hidden'] });
  document.addEventListener('gregal:fun-prefs', () => { st.cardWeek = ''; maybeCard(); });
  document.addEventListener('gregal:idioma', () => {
    const l = document.querySelector('.stats-nav-label'); if (l) l.textContent = T('wrapped.nav');
    st.cardWeek = '';
    maybeCard();
    if (st.dialog) { const open = st.dialog.open; st.dialog.remove(); st.dialog = null; if (open) openStats(); }
  });
  // L'API és a punt una mica després que el mòdul: un reintent curt.
  const first = () => (G() && G().api ? maybeCard() : setTimeout(first, 300));
  first();
}

window.gregalWrapped = { open: openStats };
