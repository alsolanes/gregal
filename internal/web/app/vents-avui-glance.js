// Gregal Vents → «Avui» d'una ullada: el temps on ets, el que tens al
// calendari i el que segueixes (el pròxim partit del teu equip o la pròxima
// cosa del que sigui), a dalt de tot de la safata. Les dades vénen fetes de
// /api/winds/avui (`glance`); el lloc i els seguiments es desen a
// /api/winds/avui/settings (`place`, `follows`). Mateixa API que Android.

import { idioma } from './i18n.js';

const tx = (ca, en) => (idioma() === 'en' ? en : ca);
const api = (p, o) => (window.gregal?.api ? window.gregal.api(p, o) : window.api ? window.api(p, o) : fetch(p, o));
const post = async (path, body) => {
  const r = await api(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  if (!r.ok) throw new Error((await r.text()).trim() || r.status);
  return r.json().catch(() => ({}));
};
const el = (tag, cls, text) => { const e = document.createElement(tag); if (cls) e.className = cls; if (text != null) e.textContent = text; return e; };
const btn = (label, fn, cls) => { const b = el('button', cls, label); b.type = 'button'; b.onclick = fn; return b; };
const locale = () => (idioma() === 'en' ? 'en-GB' : 'ca-ES');
const hhmm = d => d.toLocaleTimeString(locale(), { hour: '2-digit', minute: '2-digit' });
const dayDiff = (d, now) => Math.round((new Date(d.getFullYear(), d.getMonth(), d.getDate()) - new Date(now.getFullYear(), now.getMonth(), now.getDate())) / 86400000);
const reducedMotion = () => !!globalThis.matchMedia?.('(prefers-reduced-motion: reduce)').matches;

// --- lògica pura (testejable sense DOM) ---

/** El codi WMO d'Open-Meteo → el tipus de cel (el mateix tall que el temps del vent). */
export const weatherKind = code => code === 0 ? 'clear' : code <= 2 ? 'partly' : code === 3 ? 'cloudy' : code <= 48 ? 'fog' : code <= 57 ? 'drizzle' : code <= 67 ? 'rain' : code <= 77 ? 'snow' : code <= 82 ? 'rain' : code <= 86 ? 'snow' : 'storm';
const kindName = k => ({ clear: tx('Serè', 'Clear'), partly: tx('Clarianes', 'Partly cloudy'), cloudy: tx('Ennuvolat', 'Cloudy'), fog: tx('Boira', 'Fog'), drizzle: tx('Plugim', 'Drizzle'), rain: tx('Pluja', 'Rain'), snow: tx('Neu', 'Snow'), storm: tx('Tempesta', 'Storm') })[k] || '';

/** «2026-11-24T07:52» o «07:52» → «07:52». */
export const clock = s => (String(s || '').match(/(\d{2}:\d{2})/) || [])[1] || '';

/** Les hores de pluja en franges: «Pluja: 15–18 h, 21 h». */
export function rainText(hours) {
  const hs = [...new Set((hours || []).map(Number))].filter(h => h >= 0 && h < 24).sort((a, b) => a - b);
  if (!hs.length) return tx('Sense pluja', 'No rain');
  const p = n => String(n).padStart(2, '0'), spans = [];
  for (const h of hs) { const last = spans.at(-1); if (last && last[1] === h - 1) last[1] = h; else spans.push([h, h]); }
  return tx('Pluja: ', 'Rain: ') + spans.map(([a, b]) => (a === b ? p(a) : p(a) + '–' + p(b + 1)) + ' h').join(', ');
}

/** Quan, com ho diria una persona: «avui a les 21:00», «demà…», «dissabte…». */
export function followWhen(iso, now = new Date()) {
  const d = iso ? new Date(iso) : null;
  if (!d || isNaN(d)) return '';
  const at = tx(' a les ', ' at ') + hhmm(d), diff = dayDiff(d, now);
  if (diff === 0) return tx('avui', 'today') + at;
  if (diff === 1) return tx('demà', 'tomorrow') + at;
  if (diff > 1 && diff < 7) return d.toLocaleDateString(locale(), { weekday: 'long' }) + at;
  return d.toLocaleDateString(locale(), { weekday: 'short', day: 'numeric', month: 'short' }) + at;
}

/** El compte enrere: «d'aquí a 25 min», «d'aquí a 2 h», «d'aquí a 3 dies»; buit si ja ha passat. */
export function countdown(iso, now = new Date()) {
  const d = iso ? new Date(iso) : null;
  if (!d || isNaN(d)) return '';
  const min = Math.floor((d - now) / 60000);
  if (min < 0) return '';
  const pre = tx('d’aquí a ', 'in ');
  if (min < 60) return pre + Math.max(min, 1) + ' min';
  if (min < 1440) return pre + Math.floor(min / 60) + ' h';
  const days = Math.floor(min / 1440);
  return pre + days + (days === 1 ? tx(' dia', ' day') : tx(' dies', ' days'));
}

/** Els esdeveniments que encara no s'han acabat, per ordre. */
export function upcomingEvents(events, now = new Date(), max = 4) {
  return (events || [])
    .filter(e => { const end = new Date(e.end || e.start); return !isNaN(end) && (String(e.start).length <= 10 ? dayDiff(end, now) >= 0 : end >= now); })
    .sort((a, b) => new Date(a.start) - new Date(b.start))
    .slice(0, max);
}

function eventWhen(e, now) {
  const d = new Date(e.start), allDay = String(e.start).length <= 10, tomorrow = dayDiff(d, now) === 1;
  if (allDay) return tomorrow ? tx('demà, tot el dia', 'tomorrow, all day') : tx('tot el dia', 'all day');
  return (tomorrow ? tx('demà ', 'tomorrow ') : '') + hhmm(d);
}

// El que es desa d'un seguiment (sense la resposta fresca del vent).
const stored = f => { const o = { id: f.id, kind: f.kind, name: f.name }; if (f.wind) o.wind = f.wind; return o; };

/** Encara s'està buscant alguna cosa (un seguiment sense resposta, el temps del lloc): cal tornar a mirar. */
export function glancePending(glance, settings) {
  const g = glance || {}, st = settings || {}, fresh = new Map((g.follows || []).map(f => [f.id, f]));
  const waiting = (st.follows || g.follows || []).some(f => { const n = fresh.get(f.id) || {}; return !n.title && !n.none && !n.error; });
  return waiting || (!!(st.place || '').trim() && !g.weather && !g.weather_error);
}

/** Actualitza els comptes enrere ja pintats. */
export function tickCountdowns(box, now = new Date()) {
  const walk = n => { if (n.dataset?.when) n.textContent = countdown(n.dataset.when, now); for (const c of n.children || []) walk(c); };
  walk(box);
}

// --- dibuix ---

// La icona del temps, amb peces que es mouen soles (el sol gira, les gotes cauen).
function weatherIcon(kind, day) {
  const sun = day
    ? '<g class="vw-gi-sun"><circle cx="12" cy="12" r="4.2"/><path d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5.3 5.3l1.4 1.4M17.3 17.3l1.4 1.4M5.3 18.7l1.4-1.4M17.3 6.7l1.4-1.4"/></g>'
    : '<path class="vw-gi-moon" d="M15.5 4.5a7.5 7.5 0 1 0 4 12.8 6 6 0 0 1-4-12.8z"/>';
  // El núvol va dins d'un grup que es mou: l'animació CSS trepitjaria el seu `transform`.
  const cloud = (cls, dy = 0) => '<g class="vw-gi-drift"><path class="vw-gi-cloud ' + cls + '" transform="translate(0 ' + dy + ')" d="M7 19h10.5a4 4 0 0 0 .4-8 5.5 5.5 0 0 0-10.6-1.2A4.2 4.2 0 0 0 7 19z"/></g>';
  const drops = '<g class="vw-gi-drops"><path d="M8 20.5l-1 2"/><path d="M12 20.5l-1 2"/><path d="M16 20.5l-1 2"/></g>';
  const inner = {
    clear: sun,
    partly: '<g transform="translate(-3 -3) scale(.8)">' + sun + '</g>' + cloud('is-light', 2),
    cloudy: cloud('is-mid', -1),
    fog: cloud('is-mid', -3) + '<g class="vw-gi-fog"><path d="M4 20h16"/><path d="M6 23h12"/></g>',
    drizzle: cloud('is-mid', -3) + drops,
    rain: cloud('is-dark', -3) + drops,
    snow: cloud('is-light', -3) + '<g class="vw-gi-flakes"><circle cx="8" cy="21.5" r="1.2"/><circle cx="12" cy="22.5" r="1.2"/><circle cx="16" cy="21.5" r="1.2"/></g>',
    storm: cloud('is-dark', -3) + '<path class="vw-gi-bolt" d="M12.5 17l-3 4.5h3l-1.5 3.5 4.5-5h-3l1.5-3z"/>',
  }[kind] || sun;
  return '<svg viewBox="0 0 24 26" aria-hidden="true">' + inner + '</svg>';
}

// El partit: un camp i una pilota que bota, del color del vent (o del tint de la targeta).
function matchArt() {
  const ink = 'var(--wind, var(--gl-tint, var(--accent)))';
  return '<svg class="vw-match" viewBox="0 0 24 24" aria-hidden="true">' +
    '<rect x="2" y="6" width="20" height="12" rx="2" style="fill:' + ink + ';fill-opacity:.16;stroke:' + ink + ';stroke-opacity:.7;stroke-width:1.1"/>' +
    '<path d="M12 6v12" style="stroke:' + ink + ';stroke-opacity:.7;stroke-width:1.1"/>' +
    '<circle cx="12" cy="12" r="2.6" style="fill:none;stroke:' + ink + ';stroke-opacity:.7;stroke-width:1.1"/>' +
    '<g class="vw-match-ball"><circle cx="17" cy="13.5" r="2.8" style="fill:var(--surface);stroke:' + ink + ';stroke-width:.8"/><circle cx="17" cy="13.5" r="1" style="fill:' + ink + '"/></g></svg>';
}

function isDay(w, now) {
  const at = s => { const c = clock(s); if (!c) return null; const [h, m] = c.split(':').map(Number); return h * 60 + m; };
  const rise = at(w.sunrise), set = at(w.sunset), t = now.getHours() * 60 + now.getMinutes();
  return rise == null || set == null || (t >= rise && t < set);
}

function weatherCard(w, now, openPlace) {
  const kind = weatherKind(Number(w.code)), card = el('article', 'vw-glance-card vw-glance-wx');
  card.dataset.kind = kind;
  const top = el('div', 'vw-glance-wx-top'), ic = el('span', 'vw-glance-icon');
  ic.innerHTML = weatherIcon(kind, isDay(w, now));
  const temp = el('div', 'vw-glance-temp');
  const range = el('span', 'vw-glance-range');
  range.append(el('span', 'vw-glance-min', Math.round(w.min_c) + '°'), el('span', 'vw-glance-sep', ' / '), el('span', 'vw-glance-max', Math.round(w.max_c) + '°'));
  temp.append(el('b', '', Math.round(w.now_c) + '°'), range);
  top.append(ic, temp);
  const where = btn('', openPlace, 'vw-glance-place');
  where.append(el('strong', '', w.place || ''), el('span', '', kindName(kind)));
  where.setAttribute('aria-label', tx('Canvia el lloc: ', 'Change the place: ') + (w.place || ''));
  const rainy = new Set((w.rain_hours || []).map(Number)), bar = el('div', 'vw-glance-hours');
  bar.setAttribute('aria-hidden', 'true');
  for (let h = 0; h < 24; h++) {
    const cell = el('i', 'vw-glance-hour' + (rainy.has(h) ? ' is-rain' : '') + (h === now.getHours() ? ' is-now' : ''));
    cell.dataset.h = String(h).padStart(2, '0');
    if (rainy.has(h)) cell.style.setProperty('--d', String(h % 6));
    bar.append(cell);
  }
  const rain = el('p', 'vw-glance-rain' + (rainy.size ? ' is-wet' : ''), rainText(w.rain_hours) + (rainy.size && w.rain_mm ? ' · ' + w.rain_mm + ' mm' : ''));
  const sun = el('p', 'vw-glance-sunline');
  sun.append(el('span', 'vw-glance-rise', tx('Surt ', 'Sunrise ') + clock(w.sunrise)), el('span', 'vw-glance-set', tx('Es pon ', 'Sunset ') + clock(w.sunset)));
  card.append(top, where, bar, rain, sun);
  return card;
}

function calendarCard(events, now) {
  const card = el('article', 'vw-glance-card vw-glance-cal');
  card.append(el('h3', 'vw-glance-kicker', tx('Calendari', 'Calendar')));
  const list = el('ul', 'vw-glance-events');
  for (const e of events) {
    const row = el('li', 'vw-glance-event'), when = el('span', 'vw-glance-time', eventWhen(e, now));
    const copy = el('span', 'vw-glance-what');
    copy.append(el('strong', '', e.title || ''));
    if (e.calendar) copy.append(el('small', '', e.calendar));
    row.append(when, copy);
    list.append(row);
  }
  card.append(list);
  return card;
}

function followCard(f, now, remove) {
  const card = el('article', 'vw-glance-card vw-glance-follow');
  card.dataset.kind = f.kind === 'team' ? 'team' : 'other';
  const head = el('div', 'vw-glance-follow-head');
  const badge = el('span', 'vw-glance-badge', f.kind === 'team' ? null : (f.name || '?').trim().charAt(0).toUpperCase());
  if (f.kind === 'team') badge.innerHTML = matchArt();
  badge.setAttribute('aria-hidden', 'true');
  const x = btn('×', remove, 'vw-glance-remove');
  x.setAttribute('aria-label', tx('Deixa de seguir ', 'Stop following ') + f.name);
  head.append(badge, el('span', 'vw-glance-kicker', f.name || ''), x);
  card.append(head);
  if (!f.title) {
    if (f.error) card.append(el('p', 'vw-glance-error', '⚠ ' + f.error));
    else if (f.none) card.append(el('p', 'vw-glance-pending', f.kind === 'team' ? tx('No he trobat partits per a ', 'No upcoming matches found for ') + f.name + '.' : tx('No he trobat res sobre ', 'Nothing found about ') + f.name + '.'));
    else card.append(el('p', 'vw-glance-pending', tx('Gregal ho buscarà i t’ho dirà aquí.', 'Gregal will look it up and tell you here.')));
    return card;
  }
  card.append(el('strong', 'vw-glance-title', f.title));
  const when = followWhen(f.when, now);
  if (when) {
    const line = el('p', 'vw-glance-when', when), left = countdown(f.when, now);
    if (left) { const pill = el('span', 'vw-glance-count', left); pill.dataset.when = f.when; line.append(pill); }
    card.append(line);
  }
  if (f.detail) card.append(el('p', 'vw-glance-detail', f.detail));
  if (f.url) {
    const a = el('a', 'vw-glance-link', tx('Més', 'More'));
    a.href = f.url; a.target = '_blank'; a.rel = 'noopener noreferrer';
    card.append(a);
  }
  return card;
}

// [art]: el dibuix de la targeta que vindrà (el cel, el partit), petit i quiet.
function invite(text, label, fn, art) {
  const card = el('article', 'vw-glance-card vw-glance-invite');
  if (art) { const ic = el('span', 'vw-glance-icon'); ic.innerHTML = art; card.append(ic); }
  card.append(el('p', '', text), btn(label, fn, 'vw-glance-chip'));
  return card;
}

/** La fila d'una ullada. opts: {glance, settings, onChange, now}. */
export function renderGlance(box, opts = {}) {
  const g = opts.glance || {}, st = opts.settings || {}, now = opts.now || new Date();
  const follows = (st.follows || g.follows || []).map(stored);
  const status = el('p', 'vw-glance-status');
  status.setAttribute('role', 'status');
  const save = async body => {
    try { await post('/api/winds/avui/settings', body); } catch (e) { status.textContent = '⚠ ' + (e.message || e); return; }
    opts.onChange?.();
  };

  // «Segueix…»: un equip o una altra cosa, i on ets.
  const form = el('div', 'vw-glance-form');
  form.hidden = true;
  let kind = 'team';
  const kinds = el('div', 'vw-glance-kinds');
  kinds.setAttribute('role', 'group');
  kinds.setAttribute('aria-label', tx('Què vols seguir?', 'What do you want to follow?'));
  const kindBtns = [['team', tx('Un equip', 'A team')], ['other', tx('Una altra cosa', 'Something else')]].map(([k, label]) => {
    const b = btn(label, () => { kind = k; for (const [kk, bb] of kindBtns) bb.setAttribute('aria-pressed', String(kk === k)); name.placeholder = k === 'team' ? tx('Barça, Girona FC, els Lakers…', 'Barça, Girona FC, the Lakers…') : tx('Un festival, un llançament, una sèrie…', 'A festival, a launch, a series…'); }, 'vw-glance-kind');
    b.setAttribute('aria-pressed', String(k === kind));
    return [k, b];
  });
  kinds.append(...kindBtns.map(([, b]) => b));
  const add = el('form', 'vw-glance-add'), name = el('input', 'vw-glance-name');
  name.placeholder = tx('Barça, Girona FC, els Lakers…', 'Barça, Girona FC, the Lakers…');
  name.setAttribute('aria-label', tx('Què vols seguir', 'What to follow'));
  const addBtn = el('button', 'vw-glance-chip is-primary', tx('Segueix', 'Follow'));
  addBtn.type = 'submit';
  add.append(name, addBtn);
  add.onsubmit = async e => {
    e.preventDefault();
    const n = name.value.trim();
    if (!n) return;
    await save({ follow: { kind, name: n } });
  };
  const placeForm = el('form', 'vw-glance-placeform'), place = el('input', 'vw-glance-place-in');
  place.value = st.place || g.weather?.place || '';
  place.placeholder = tx('Barcelona, Puigcerdà, Sóller…', 'Barcelona, Puigcerdà, Sóller…');
  place.setAttribute('aria-label', tx('On ets', 'Where you are'));
  const placeBtn = el('button', 'vw-glance-chip', tx('Desa', 'Save'));
  placeBtn.type = 'submit';
  placeForm.append(el('span', 'vw-glance-formlabel', tx('On ets?', 'Where are you?')), place, placeBtn);
  placeForm.onsubmit = async e => { e.preventDefault(); await save({ place: place.value.trim() }); };
  form.append(el('span', 'vw-glance-formlabel', tx('Què vols seguir?', 'What do you want to follow?')), kinds, add, placeForm);
  const open = focus => () => { form.hidden = false; toggle.setAttribute('aria-expanded', 'true'); focus.focus(); };
  const toggle = btn(tx('Segueix…', 'Follow…'), () => { if (form.hidden) open(name)(); else { form.hidden = true; toggle.setAttribute('aria-expanded', 'false'); } }, 'vw-glance-chip vw-glance-toggle');
  toggle.setAttribute('aria-expanded', 'false');

  const row = el('div', 'vw-glance-row'), cards = [];
  const where = (st.place || '').trim();
  if (g.weather) cards.push(weatherCard(g.weather, now, open(place)));
  else if (where) cards.push(invite(g.weather_error ? '⚠ ' + g.weather_error : tx('Buscant el temps de ', 'Looking up the weather for ') + where + '…', tx('Canvia el lloc', 'Change the place'), open(place), weatherIcon('partly', true)));
  else cards.push(invite(tx('Digues on ets i cada matí tindràs aquí el temps que farà.', 'Tell me where you are and every morning the weather shows up here.'), tx('Digues on ets', 'Tell me where you are'), open(place), weatherIcon('partly', true)));
  const events = upcomingEvents(g.events, now);
  if (events.length) cards.push(calendarCard(events, now));
  const fresh = new Map((g.follows || []).map(f => [f.id, f]));
  for (const f of follows) cards.push(followCard({ ...f, ...fresh.get(f.id) }, now, () => save({ unfollow: f.id })));
  if (!follows.length) cards.push(invite(tx('Segueix el teu equip i el pròxim partit sortirà aquí, amb el compte enrere.', 'Follow your team and the next match shows up here, with a countdown.'), tx('Segueix un equip', 'Follow a team'), open(name), matchArt()));
  cards.forEach((c, i) => c.style.setProperty('--i', String(i)));
  row.append(...cards);
  // Al mòbil la fila llisca: s'esvaeix només pel costat on queden targetes amagades (com la fila de cares).
  const edge = () => { row.dataset.fade = [row.scrollLeft > 1 && 'start', row.scrollLeft + row.clientWidth < row.scrollWidth - 1 && 'end'].filter(Boolean).join(' '); };
  row.addEventListener('scroll', edge, { passive: true }); new ResizeObserver(edge).observe(row);
  // Sense seguiments, «Segueix un equip» ja és a la seva targeta: el xip de sota quedaria orfe.
  const bar = el('div', 'vw-glance-bar');
  if (follows.length) bar.append(toggle);
  bar.append(status);
  box.replaceChildren(row, bar, form);

  if (!reducedMotion()) {
    const id = setInterval(() => { if (!box.isConnected) clearInterval(id); else tickCountdowns(box); }, 30000);
  }
}

// --- el resum del dia (com el Daily Brief de Duet) ---

/** Feina delegada acabada o una rutina que ha corregut: el que el vent ha fet. */
const worked = c => String(c.source || '').startsWith('feina:') || c.source === 'job';

/**
 * Un per vent que ha fet o proposa alguna cosa, en l'ordre de la safata (Ara toca
 * primer) i després els que només han fet. La mateixa regla que l'Android
 * (AvuiBriefLogic.briefs), perquè les dues apps ensenyin el mateix resum.
 * did: la feina acabada; proposes: la primera altra targeta oberta;
 * act: [targeta, acció] del botó, la de la proposta o, si no, la de la feina.
 */
export function briefs(data, winds) {
  const d = data || {}, open = [...(d.now || []), ...(d.fyi || []), ...(d.ahead || [])].filter(c => c.state === 'open');
  const cards = [...open, ...(d.done || [])];
  return [...new Set(cards.map(c => c.wind))].flatMap(id => {
    const mine = cards.filter(c => c.wind === id);
    const did = mine.find(worked) || null;
    const proposes = open.find(c => c.wind === id && c !== did) || null;
    if (!did && !proposes) return [];
    // Una vigilància pausada no: el xat no la reprèn, ho fa «Torna-hi» a la seva targeta.
    const withAct = [proposes, did].find(c => c && c.source !== 'pausa' && c.actions?.length);
    const wind = (winds || []).find(w => w.id === id) || { id, name: mine[0].wind_name || '' };
    return [{ wind, did, proposes, act: withAct ? [withAct, withAct.actions[0]] : null }];
  });
}

function briefCard(b, label, art, face, onRun) {
  const card = el('article', 'vw-brief-card');
  card.dataset.wind = b.wind.id;
  card.setAttribute('aria-label', label);
  if (b.wind.color) card.style.setProperty('--wind', b.wind.color);
  const head = el('div', 'vw-brief-head'), pic = el('span', 'vw-brief-face'), f = face?.(b.wind);
  if (f) pic.append(f);
  head.append(pic, el('strong', 'vw-brief-name', b.wind.name || ''));
  if (art) { const a = el('span', 'vw-brief-art'); a.innerHTML = art; head.append(a); }
  card.append(head);
  if (b.did) {
    const row = el('p', 'vw-brief-did'), tick = el('span', 'vw-brief-tick');
    tick.innerHTML = '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3.5 8.5l3 3 6-7"/></svg>';
    const copy = el('span', 'vw-brief-copy');
    copy.append(el('span', 'vw-brief-label', tx('Ha fet', 'Done')), el('span', 'vw-brief-what', b.did.title || ''));
    row.append(tick, copy);
    card.append(row);
  }
  if (b.proposes) {
    const box = el('div', 'vw-brief-proposes');
    box.append(el('span', 'vw-brief-label', tx('Proposa', 'Suggests')), el('strong', 'vw-brief-title', b.proposes.title || ''));
    if (b.proposes.why) box.append(el('p', 'vw-brief-why', b.proposes.why));
    card.append(box);
  }
  if (b.act) {
    const [c, a] = b.act, go = btn('', () => onRun?.(c, a), 'vw-brief-act'), arrow = el('span', 'vw-brief-arrow');
    arrow.innerHTML = '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 8h10M9 4l4 4-4 4"/></svg>';
    go.append(el('span', '', a.label || ''), arrow);
    card.append(go);
  }
  return card;
}

/**
 * El resum a dalt d'Avui: una targeta per vent que es llisca amb el paginador de
 * les comparacions (md.js escolta `.gx-carousel` a tot el document: fletxes,
 * punts i lliscar). Sense cap vent amb res, la caixa queda amagada.
 * opts: {data (GET /api/winds/avui), winds, glance, settings, face(w), onRun(card, action), now}.
 */
export function renderBrief(box, opts = {}) {
  const list = briefs(opts.data, opts.winds), g = opts.glance || {}, now = opts.now || new Date();
  box.hidden = !list.length;
  if (!list.length) { box.replaceChildren(); return; }
  // Els dibuixos: el vent del temps porta el cel d'avui; el que segueix un equip, el partit.
  const wx = g.weather && (opts.winds || []).find(w => w.kit === 'weather' || w.specialty === 'weather');
  const teams = new Set([...(opts.settings?.follows || []), ...(g.follows || [])].filter(f => f.kind === 'team' && f.wind).map(f => f.wind));
  const art = w => (wx && w.id === wx.id ? weatherIcon(weatherKind(Number(g.weather.code)), isDay(g.weather, now)) : teams.has(w.id) ? matchArt() : '');
  const title = tx('El dia dels teus vents', 'Your winds today');
  const track = el('div', 'gx-track');
  track.tabIndex = 0;
  track.setAttribute('role', 'region');
  track.setAttribute('aria-label', title);
  track.style.setProperty('--gx-rows', '1');
  list.forEach((b, i) => {
    const c = briefCard(b, b.wind.name + ' · ' + (i + 1) + ' / ' + list.length, art(b.wind), opts.face, opts.onRun);
    c.style.setProperty('--i', String(i));
    track.append(c);
  });
  // md.js mira les fletxes en passar-hi el ratolí: hi han de ser encara que només hi hagi una targeta.
  const step = (dir, label) => { const b = btn('‹›'[dir > 0 ? 1 : 0], null, 'gx-step ' + (dir > 0 ? 'gx-next' : 'gx-prev')); b.dataset.gxStep = String(dir); b.setAttribute('aria-label', label); return b; };
  const prev = step(-1, tx('Anterior', 'Previous')), next = step(1, tx('Següent', 'Next'));
  prev.disabled = true;
  next.disabled = list.length < 2;
  const car = el('div', 'gx-carousel');
  car.append(track, prev, next);
  if (list.length > 1) {
    const dots = el('div', 'gx-dots');
    list.forEach((b, i) => {
      const d = btn('', null);
      d.dataset.gxDot = String(i);
      d.setAttribute('aria-label', b.wind.name || String(i + 1));
      if (!i) d.setAttribute('aria-current', 'true');
      dots.append(d);
    });
    car.append(dots);
  }
  box.className = 'vw-brief gregal-compare';
  box.setAttribute('aria-label', title);
  box.replaceChildren(el('h3', 'vw-glance-kicker', title), car);
}

/**
 * El botó d'una targeta `pausa` (una vigilància que s'ha pausat sola): la
 * reprèn amb POST resume, no pas demanant-ho al xat del vent, que no ho pot fer.
 * [status] diu si ha anat bé o per què no; [onResumed] treu la targeta.
 */
export function resumeButton(card, status, onResumed) {
  const b = btn(tx('Torna-hi', 'Resume'), async () => {
    b.disabled = true;
    try { await post('/api/winds/avui', { id: card.id, action: 'resume' }); } catch (e) {
      status.textContent = '⚠ ' + tx('No s’ha pogut reprendre: ', 'Could not resume: ') + (e.message || e);
      b.disabled = false;
      return;
    }
    status.textContent = tx('Hi torno a mirar.', 'Watching it again.');
    onResumed?.();
  }, 'vw-avui-chip');
  return b;
}

window.gregalAvuiGlance = { renderGlance, glancePending, resumeButton, renderBrief };
