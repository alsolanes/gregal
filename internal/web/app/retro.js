// #retro: canal a l'estil Slack per a un torn acabat. Els agents expliquen
// què han fet amb «rebuts» (evidència real del torn), reaccionen i se'ls pot
// discutir amb @mencions. El servidor (retro.go) és la font de veritat; aquí
// només es pinta, s'envia i s'escolta el registre durable en directe.
import './retro-core.js';
import { retroPersonaPref, PERSONAS } from './fun-prefs.js';

const core = window.gregalRetroCore;
const G = () => window.gregal;
const T = (key, vars) => {
  let s = (window.gregalT && window.gregalT(key)) || key;
  for (const k in vars || {}) s = s.replace('{' + k + '}', vars[k]);
  return s;
};
const EMOJIS = ['👍', '🎉', '🔥', '🤔', '👀', '✅', '👎', '😂', '🙏'];
const AVATARS = ['coordinator', 'researcher', 'builder', 'reviewer'];
// Especialistes (no han fet la feina; la revisen des del seu angle): icona
// pròpia en lloc del personatge pixelat.
const SPECIALIST_ICONS = {
  design: '<path d="M12 3a9 9 0 1 0 0 18c1 0 1.5-.8 1.5-1.6 0-.5-.2-.9-.5-1.2-.3-.3-.5-.7-.5-1.2 0-.9.7-1.6 1.6-1.6H16a5 5 0 0 0 5-5C21 6.4 17 3 12 3z"/><circle cx="7.5" cy="11" r="1"/><circle cx="10" cy="7.5" r="1"/><circle cx="14.5" cy="7.5" r="1"/>',
  product: '<path d="M9 18h6M10 21h4M12 3a6 6 0 0 0-3.5 10.9c.6.5 1 1.2 1 2.1h5c0-.9.4-1.6 1-2.1A6 6 0 0 0 12 3z"/>',
  code: '<path d="M8 7l-5 5 5 5M16 7l5 5-5 5M13.5 4l-3 16"/>',
  security: '<path d="M12 3l7 3v5c0 4.5-3 8.3-7 10-4-1.7-7-5.5-7-10V6z"/><path d="M9 12l2 2 4-4"/>',
  qa: '<path d="M9 9V7a3 3 0 0 1 6 0v2"/><rect x="7" y="9" width="10" height="11" rx="5"/><path d="M12 13v7M3 13h4M17 13h4M4 19l3-2M20 19l-3-2M4 8l3 2M20 8l-3 2"/>',
};

const st = {
  dialog: null, channel: null, latest: null, key: '', cursor: 0,
  thread: '', typing: new Map(), open: false, abort: null, busy: false,
  expanded: new Set(), pickerFor: null, notice: '',
  filter: '', // barra de revisió: '' | all | high | medium | low
};

function el(tag, cls, text) {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text != null) n.textContent = text;
  return n;
}
function btn(cls, label, text, onClick) {
  const b = el('button', cls, text);
  b.type = 'button';
  if (label) { b.setAttribute('aria-label', label); b.title = label; }
  if (onClick) b.addEventListener('click', onClick);
  return b;
}
async function call(path, body) {
  const r = await G().api(path, body === undefined ? {} : { method: 'POST', body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } });
  const text = await r.text();
  let data = {};
  try { data = text ? JSON.parse(text) : {}; } catch (e) { data = { error: text }; }
  if (!r.ok) throw new Error((data && data.error) || text || ('HTTP ' + r.status));
  return data;
}

function agentOf(id) {
  return (st.channel && st.channel.agents || []).find(a => a.id === id) || null;
}
function nameOf(id) {
  if (id === 'user') return T('retro.you');
  if (id === 'system') return T('retro.system');
  const a = agentOf(id);
  return a ? a.name : id;
}
function avatar(id) {
  const box = el('span', 'retro-avatar');
  box.setAttribute('aria-hidden', 'true');
  const a = agentOf(id);
  if (a && SPECIALIST_ICONS[a.avatar]) {
    box.classList.add('is-spec', 'spec-' + a.avatar);
    box.innerHTML = '<svg viewBox="0 0 24 24" width="62%" height="62%" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">' + SPECIALIST_ICONS[a.avatar] + '</svg>';
  } else if (a) {
    const img = el('img');
    img.src = '/app/assets/pixel-office/' + (AVATARS.includes(a.avatar) ? a.avatar : 'builder') + '.png';
    img.alt = '';
    box.appendChild(img);
  } else {
    box.classList.add(id === 'system' ? 'is-system' : 'is-user');
    box.textContent = id === 'system' ? '•' : (T('retro.you')[0] || 'U').toUpperCase();
  }
  return box;
}
function clock(at) {
  const d = new Date(at);
  return isNaN(d) ? '' : d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}
// Text pla amb les @mencions ressaltades (mai HTML del model).
function richText(text) {
  const frag = document.createDocumentFragment();
  const parts = String(text || '').split(/(@[A-Za-z][\w-]*)/g);
  for (const p of parts) {
    if (/^@[A-Za-z]/.test(p) && core.parseMentions(p, st.channel && st.channel.agents).length) frag.appendChild(el('span', 'retro-mention', p));
    else if (p) frag.appendChild(document.createTextNode(p));
  }
  return frag;
}

// --- muntatge ---

function ensureDialog() {
  if (st.dialog) return st.dialog;
  const mount = document.getElementById('retroMount') || document.body;
  const d = el('dialog', 'retro-dialog');
  d.setAttribute('aria-labelledby', 'retroHeading');
  d.innerHTML = '<header class="retro-head"><span class="retro-hash" aria-hidden="true">#</span><h2 id="retroHeading">retro</h2>' +
    '<span class="retro-sub"></span><span class="retro-people"></span></header>' +
    '<div class="retro-body"><section class="retro-main"><div class="retro-list" role="log" aria-live="polite"></div>' +
    '<div class="retro-typing" aria-live="polite"></div><form class="retro-composer" data-scope="main"></form></section>' +
    '<aside class="retro-thread" hidden><header class="retro-thread-head"><h3></h3></header><div class="retro-list" role="log" aria-live="polite"></div>' +
    '<div class="retro-typing" aria-live="polite"></div><form class="retro-composer" data-scope="thread"></form></aside></div>' +
    '<div class="retro-picker" role="menu" hidden></div><div class="retro-notice" role="status" hidden></div>';
  const head = d.querySelector('.retro-head');
  head.appendChild(personaPicker());
  head.appendChild(btn('retro-icon retro-close', T('retro.close'), '✕', () => close()));
  d.querySelector('.retro-thread-head').appendChild(btn('retro-icon', T('retro.closeThread'), '✕', () => openThread('')));
  d.querySelectorAll('.retro-composer').forEach(buildComposer);
  const picker = d.querySelector('.retro-picker');
  EMOJIS.forEach(e => {
    const b = btn('retro-emoji', T('retro.reactWith', { e }), e, () => { const id = st.pickerFor; hidePicker(); if (id) react(id, e); });
    b.setAttribute('role', 'menuitem');
    picker.appendChild(b);
  });
  picker.addEventListener('keydown', ev => {
    const items = [...picker.querySelectorAll('button')];
    const i = items.indexOf(document.activeElement);
    if (ev.key === 'ArrowRight' || ev.key === 'ArrowDown') { ev.preventDefault(); items[(i + 1) % items.length].focus(); }
    if (ev.key === 'ArrowLeft' || ev.key === 'ArrowUp') { ev.preventDefault(); items[(i - 1 + items.length) % items.length].focus(); }
  });
  // Esc tanca primer el selector, després el fil i, només llavors, el canal.
  d.addEventListener('cancel', ev => {
    if (!picker.hidden) { ev.preventDefault(); hidePicker(); return; }
    if (d.querySelector('.retro-mentions:not([hidden])')) { ev.preventDefault(); d.querySelectorAll('.retro-mentions').forEach(m => { m.hidden = true; }); return; }
    if (st.thread) { ev.preventDefault(); openThread(''); return; }
  });
  d.addEventListener('close', () => stopLive());
  d.addEventListener('click', ev => {
    if (!picker.hidden && !picker.contains(ev.target) && !ev.target.closest('[data-picker]')) hidePicker();
  });
  mount.appendChild(d);
  st.dialog = d;
  return d;
}

function buildComposer(form) {
  const thread = form.dataset.scope === 'thread';
  const ta = el('textarea', 'retro-input');
  ta.rows = 1;
  ta.maxLength = 4000;
  const label = T(thread ? 'retro.threadComposer' : 'retro.composer');
  ta.placeholder = label;
  ta.setAttribute('aria-label', label);
  ta.setAttribute('aria-autocomplete', 'list');
  const list = el('div', 'retro-mentions');
  list.setAttribute('role', 'listbox');
  list.setAttribute('aria-label', T('retro.mentions'));
  list.hidden = true;
  const send = btn('retro-send', T('retro.send'), T('retro.send'));
  send.type = 'submit';
  form.append(list, ta, send);
  let sel = 0;
  const matches = () => {
    const q = core.mentionQuery(ta.value, ta.selectionStart);
    return q ? core.mentionMatches(q.query, st.channel && st.channel.agents) : [];
  };
  const paint = () => {
    const m = matches();
    list.innerHTML = '';
    list.hidden = !m.length;
    if (sel >= m.length) sel = 0;
    m.forEach((a, i) => {
      const o = el('div', 'retro-mention-opt');
      o.setAttribute('role', 'option');
      o.setAttribute('aria-selected', String(i === sel));
      o.append(avatar(a.id), el('b', '', a.name), el('span', 'retro-role', '@' + a.id));
      o.addEventListener('mousedown', ev => { ev.preventDefault(); pick(a.id); });
      list.appendChild(o);
    });
  };
  const pick = id => {
    const out = core.insertMention(ta.value, ta.selectionStart, id);
    ta.value = out.text;
    ta.setSelectionRange(out.caret, out.caret);
    list.hidden = true;
    ta.focus();
  };
  ta.addEventListener('input', () => { ta.style.height = 'auto'; ta.style.height = Math.min(ta.scrollHeight, 140) + 'px'; paint(); });
  ta.addEventListener('keydown', ev => {
    const m = list.hidden ? [] : matches();
    if (m.length && (ev.key === 'ArrowDown' || ev.key === 'ArrowUp')) {
      ev.preventDefault();
      sel = (sel + (ev.key === 'ArrowDown' ? 1 : m.length - 1)) % m.length;
      paint();
      return;
    }
    if (m.length && (ev.key === 'Enter' || ev.key === 'Tab')) { ev.preventDefault(); pick(m[sel].id); return; }
    if (ev.key === 'Escape' && !list.hidden) { ev.preventDefault(); ev.stopPropagation(); list.hidden = true; return; }
    if (ev.key === 'Enter' && !ev.shiftKey && !ev.isComposing) { ev.preventDefault(); form.requestSubmit(); }
  });
  ta.addEventListener('blur', () => setTimeout(() => { list.hidden = true; }, 120));
  form.addEventListener('submit', async ev => {
    ev.preventDefault();
    const text = ta.value.trim();
    if (!text || !st.channel || st.busy) return;
    ta.value = '';
    ta.style.height = 'auto';
    send.disabled = true;
    try {
      const out = await call('/api/retro/message', { key: st.channel.key, text, thread_id: thread ? st.thread : '' });
      if (out.channel) st.channel = out.channel;
      if (!thread && out.message) st.thread = out.message.id;
      render();
    } catch (e) { notify(T('retro.failed') + e.message); ta.value = text; }
    finally { send.disabled = false; ta.focus(); }
  });
}

// --- obertura ---

export async function open(opts) {
  const o = opts || {};
  const d = ensureDialog();
  st.thread = '';
  st.expanded.clear();
  st.channel = null;
  st.notice = '';
  if (!d.open) d.showModal();
  st.open = true;
  renderLoading(T('retro.loading'));
  try {
    const q = o.key ? '?key=' + encodeURIComponent(o.key) : o.runId ? '?run_id=' + encodeURIComponent(o.runId) : '';
    const data = await call('/api/retro' + q);
    st.latest = data.latest || {};
    st.key = data.key || '';
    st.cursor = Number(data.cursor) || 0;
    // Un canal sense missatges és una generació en curs (o tallada): es
    // tracta com «encara no fet» i start() s'hi enganxa.
    st.channel = data.channel && data.channel.messages && data.channel.messages.length ? data.channel : null;
    render();
    startLive();
    if (!st.channel && st.key && !st.latest.running && (o.autostart || data.channel)) start();
  } catch (e) { renderEmpty(T('retro.failed') + e.message); }
}

export function close() {
  if (st.dialog && st.dialog.open) st.dialog.close();
  st.open = false;
  stopLive();
}

// start genera (o s'enganxa a) el resum d'obertura del canal obert. Una
// petició penjada ja no bloqueja res: cada canal porta la seva, la resposta
// d'un canal que ja no és l'obert s'ignora i, si la petició es perd, es
// recupera el canal del servidor (que el pot tenir fet).
async function start(quick) {
  const key = st.key;
  if (!key || (st.busy && st.startKey === key && !quick)) return;
  st.busy = true;
  st.startKey = key;
  const t0 = Date.now();
  renderStarting(t0);
  const tick = setInterval(() => { if (st.startKey === key && !st.channel) renderStarting(t0); else clearInterval(tick); }, 1000);
  const ctrl = new AbortController();
  const timeout = setTimeout(() => ctrl.abort(), 150000);
  let out = null, err = null;
  try {
    const body = channelRef(key);
    if (retroPersonaPref()) body.persona = retroPersonaPref();
    if (quick) body.quick = true;
    const r = await G().api('/api/retro/start', { method: 'POST', body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' }, signal: ctrl.signal });
    const text = await r.text();
    out = text ? JSON.parse(text) : {};
    if (!r.ok) throw new Error((out && out.error) || ('HTTP ' + r.status));
  } catch (e) { err = e; }
  clearTimeout(timeout);
  clearInterval(tick);
  if (st.startKey !== key) return; // l'usuari ja mira un altre canal
  st.busy = false;
  st.startKey = '';
  if (!err && out && out.channel) {
    st.channel = out.channel;
    st.key = out.channel.key;
    render();
    return;
  }
  // Rescat: potser el servidor l'ha acabat igualment.
  try {
    const d = await call('/api/retro?key=' + encodeURIComponent(key));
    if (d.channel && d.channel.messages && d.channel.messages.length) { st.channel = d.channel; render(); return; }
  } catch (e) {}
  renderEmpty(T('retro.failed') + (err ? (err.name === 'AbortError' ? T('retro.timeout') : err.message) : ''));
}

// «Els agents s'estan preparant…» amb el temps que porta i, passats 20 s,
// l'opció de no esperar el model.
function renderStarting(t0) {
  const s = Math.round((Date.now() - t0) / 1000);
  renderLoading(T('retro.starting') + (s >= 3 ? ' · ' + s + 's' : ''));
  if (s < 20) return;
  const box = st.dialog && st.dialog.querySelector('.retro-main .retro-empty');
  if (!box || box.querySelector('.retro-quick')) return;
  box.appendChild(el('p', 'retro-slow', T('retro.slow')));
  box.appendChild(btn('retro-primary retro-quick', '', T('retro.quick'), () => start(true)));
}

async function react(id, emoji) {
  const m = st.channel && st.channel.messages.find(x => x.id === id);
  if (!m) return;
  const had = (m.reactions || []).some(r => r.emoji === emoji && (r.by || []).includes('user'));
  m.reactions = core.toggleReaction(m.reactions, emoji, 'user');
  render();
  try {
    const out = await call('/api/retro/react', { key: st.channel.key, message_id: id, emoji, action: had ? 'remove' : 'add' });
    if (out.channel) st.channel = out.channel;
    if (!had && (emoji === '🤔' || emoji === '👎') && agentOf(m.author)) st.thread = m.thread_id || m.id;
    if (!had && emoji === '👎' && out.replies && out.replies.some(r => r.note)) notify(T('retro.note'));
    render();
  } catch (e) { notify(T('retro.failed') + e.message); }
}

async function fix(id, button) {
  button.disabled = true;
  try {
    const out = await call('/api/retro/fix', { key: st.channel.key, message_id: id });
    if (out.channel) st.channel = out.channel;
    render();
    notify(T('retro.fixStarted', { n: out.run && out.run.id }), true);
  } catch (e) { button.disabled = false; notify(T('retro.failed') + e.message); }
}

function notify(text, watch) {
  const n = st.dialog && st.dialog.querySelector('.retro-notice');
  if (!n) return;
  n.innerHTML = '';
  n.hidden = false;
  n.appendChild(el('span', '', text));
  if (watch) n.appendChild(btn('retro-link', '', T('retro.watch'), () => { close(); G()?.setView?.('workshop'); }));
  n.appendChild(btn('retro-icon', T('retro.close'), '✕', () => { n.hidden = true; }));
  clearTimeout(notify.t);
  notify.t = setTimeout(() => { n.hidden = true; }, 9000);
}

function openThread(id) {
  st.thread = id || '';
  render();
  if (id) st.dialog.querySelector('.retro-thread .retro-input')?.focus();
}

function showPicker(id, anchor) {
  const p = st.dialog.querySelector('.retro-picker');
  st.pickerFor = id;
  p.hidden = false;
  const r = anchor.getBoundingClientRect(), dr = st.dialog.getBoundingClientRect();
  p.style.top = Math.max(8, r.top - dr.top - p.offsetHeight - 6) + 'px';
  p.style.left = Math.max(8, Math.min(r.left - dr.left - p.offsetWidth + r.width, dr.width - p.offsetWidth - 8)) + 'px';
  p.querySelector('button').focus();
}
function hidePicker() {
  const p = st.dialog && st.dialog.querySelector('.retro-picker');
  if (p) p.hidden = true;
  st.pickerFor = null;
}

// --- pintat ---

function renderHead() {
  const d = st.dialog;
  const ch = st.channel;
  const title = ch ? ch.title : (st.latest && st.latest.title) || '';
  d.querySelector('.retro-sub').textContent = title ? '· ' + title + (ch ? ' · ' + clock(ch.created_at) : '') : '';
  const people = d.querySelector('.retro-people');
  people.innerHTML = '';
  const persona = d.querySelector('.retro-persona');
  if (persona) { persona.hidden = !ch; persona.value = (ch && ch.persona) || 'neutral'; }
  if (ch) {
    people.setAttribute('aria-label', T('retro.agents') + ': ' + ch.agents.map(a => a.name).join(', '));
    ch.agents.forEach(a => { const av = avatar(a.id); av.title = a.name + (a.role ? ' — ' + a.role : ''); people.appendChild(av); });
  }
}

function renderLoading(text) {
  const d = ensureDialog();
  renderHead();
  const list = d.querySelector('.retro-main .retro-list');
  list.innerHTML = '';
  const box = el('div', 'retro-empty');
  box.appendChild(el('div', 'retro-spinner'));
  box.appendChild(el('p', '', text));
  list.appendChild(box);
  d.querySelector('.retro-thread').hidden = true;
  d.querySelectorAll('.retro-composer').forEach(f => { f.hidden = true; });
  paintTyping();
}

function renderEmpty(text) {
  const d = ensureDialog();
  renderHead();
  const list = d.querySelector('.retro-main .retro-list');
  list.innerHTML = '';
  const box = el('div', 'retro-empty');
  box.appendChild(el('p', '', text));
  if (st.key && !(st.latest && st.latest.running)) box.appendChild(btn('retro-primary', '', T('retro.start'), () => start()));
  list.appendChild(box);
  d.querySelector('.retro-thread').hidden = true;
  d.querySelectorAll('.retro-composer').forEach(f => { f.hidden = true; });
}

function render() {
  if (!st.dialog) return;
  hidePicker();
  if (!st.channel) {
    if (!st.key) renderEmpty(T('retro.noRuns'));
    else if (st.latest && st.latest.running && st.key === st.latest.key) renderEmpty(T('retro.running'));
    else renderEmpty(T('retro.empty'));
    return;
  }
  renderHead();
  const d = st.dialog;
  d.querySelectorAll('.retro-composer').forEach(f => { f.hidden = false; });
  const mainList = d.querySelector('.retro-main .retro-list');
  const stick = mainList.scrollHeight - mainList.scrollTop - mainList.clientHeight < 80;
  mainList.innerHTML = '';
  if (st.channel.degraded) mainList.appendChild(el('div', 'retro-banner', T('retro.degraded')));
  const bar = renderReviewBar();
  if (bar) mainList.appendChild(bar);
  const counts = core.replyCounts(st.channel.messages);
  for (const g of core.groupMessages(core.filterFindings(st.channel.messages, st.filter))) mainList.appendChild(renderGroup(g.author, g.items, counts, false));
  if (stick) mainList.scrollTop = mainList.scrollHeight;
  const aside = d.querySelector('.retro-thread');
  aside.hidden = !st.thread;
  if (st.thread) {
    aside.querySelector('h3').textContent = T('retro.thread');
    const tl = aside.querySelector('.retro-list');
    tl.innerHTML = '';
    for (const m of core.threadMessages(st.channel.messages, st.thread)) tl.appendChild(renderGroup(m.author, [m], {}, true));
    tl.scrollTop = tl.scrollHeight;
  }
  d.classList.toggle('has-thread', !!st.thread);
  paintTyping();
}

// Especialistes que encara revisen (els que «escriuen» sense haver publicat).
function pendingReviewers() {
  const now = Date.now();
  return (st.channel.agents || []).filter(a => SPECIALIST_ICONS[a.avatar] && (st.typing.get(a.id) || 0) > now);
}

// Barra de la revisió de l'equip: troballes per gravetat (fan de filtre) i
// qui encara revisa.
function renderReviewBar() {
  const n = core.findingCounts(st.channel.messages);
  const pending = pendingReviewers();
  if (!n.total && !pending.length) { st.filter = ''; return null; }
  const bar = el('div', 'retro-reviewbar');
  bar.setAttribute('role', 'toolbar');
  bar.setAttribute('aria-label', T('retro.review'));
  bar.appendChild(el('span', 'retro-reviewbar-label', T('retro.review')));
  const chip = (key, label, count) => {
    const on = st.filter === key;
    const b = btn('retro-sevchip' + (key !== 'all' ? ' sev-' + key : '') + (on ? ' is-on' : ''), '', '', () => { st.filter = on ? '' : key; render(); });
    b.setAttribute('aria-pressed', String(on));
    if (key !== 'all') b.appendChild(el('span', 'retro-sevdot'));
    b.append(el('span', '', label), el('span', 'retro-count', String(count)));
    return b;
  };
  if (n.total) {
    bar.appendChild(chip('all', T('retro.findings'), n.total));
    for (const sev of core.SEVERITIES) if (n[sev]) bar.appendChild(chip(sev, T('retro.sev.' + sev), n[sev]));
  }
  if (pending.length) {
    const p = el('span', 'retro-reviewing');
    p.appendChild(el('span', 'retro-spinner is-small'));
    p.appendChild(el('span', '', T(pending.length === 1 ? 'retro.reviewingOne' : 'retro.reviewingMany', { a: core.typingNames(pending.map(a => a.name), T('retro.and')) })));
    bar.appendChild(p);
  }
  return bar;
}

function renderGroup(author, items, counts, inThread) {
  const g = el('div', 'retro-group' + (author === 'user' ? ' is-user' : '') + (author === 'system' ? ' is-system' : ''));
  g.appendChild(avatar(author));
  const body = el('div', 'retro-group-body');
  const meta = el('div', 'retro-meta');
  meta.appendChild(el('b', 'retro-name', nameOf(author)));
  const a = agentOf(author);
  // Especialistes: només el seu angle, traduït (el rol és per al model).
  if (a && a.role) meta.appendChild(el('span', 'retro-role', SPECIALIST_ICONS[a.avatar] ? T('retro.angle.' + a.avatar) : a.role));
  const t = el('time', 'retro-time', clock(items[0].at));
  t.dateTime = items[0].at;
  meta.appendChild(t);
  body.appendChild(meta);
  items.forEach(m => body.appendChild(renderMessage(m, counts, inThread)));
  g.appendChild(body);
  return g;
}

function renderMessage(m, counts, inThread) {
  const sev = core.SEVERITIES.includes(m.severity) ? m.severity : 'low';
  const box = el('div', 'retro-msg' + (m.concede ? ' is-concede' : '') +
    (m.kind === 'finding' ? ' is-finding sev-' + sev : '') + (m.kind === 'verdict' ? ' is-verdict' : ''));
  box.dataset.id = m.id;
  box.tabIndex = -1;
  if (m.kind === 'finding') {
    const head = el('div', 'retro-finding-head');
    head.appendChild(el('span', 'retro-sev', T('retro.sev.' + sev)));
    if (m.title) head.appendChild(el('b', 'retro-finding-title', m.title));
    box.appendChild(head);
  }
  const text = el('div', 'retro-text');
  text.appendChild(richText(m.text));
  box.appendChild(text);
  if (m.receipts && m.receipts.length) {
    const rc = el('div', 'retro-receipts');
    for (const id of m.receipts) {
      const ev = (st.channel.evidence || []).find(e => e.id === id);
      if (!ev) continue;
      const key = m.id + ':' + id;
      const open = st.expanded.has(key);
      const chip = btn('retro-chip' + (ev.ok === false ? ' is-bad' : ev.ok === true ? ' is-ok' : ''), T('retro.receipt') + ' ' + id + ': ' + ev.title, '', () => {
        if (st.expanded.has(key)) st.expanded.delete(key); else st.expanded.add(key);
        render();
      });
      chip.setAttribute('aria-expanded', String(open));
      chip.append(el('span', 'retro-chip-id', id), el('span', 'retro-chip-title', ev.title));
      rc.appendChild(chip);
      if (open && ev.detail) {
        const pre = el('pre', 'retro-detail' + (ev.kind === 'file' ? ' is-diff' : ''));
        if (ev.kind === 'file') {
          ev.detail.split('\n').forEach(line => {
            const span = el('span', line.startsWith('+') && !line.startsWith('+++') ? 'add' : line.startsWith('-') && !line.startsWith('---') ? 'del' : line.startsWith('@@') ? 'hunk' : '', line + '\n');
            pre.appendChild(span);
          });
        } else pre.textContent = ev.detail;
        rc.appendChild(pre);
      }
    }
    box.appendChild(rc);
  }
  if (m.fix_suggestion) {
    const f = el('div', 'retro-fix');
    f.appendChild(el('div', 'retro-fix-label', T('retro.fix')));
    f.appendChild(el('p', '', m.fix_suggestion));
    if (m.fix_run) f.appendChild(el('span', 'retro-role', T('retro.fixStarted', { n: m.fix_run })));
    else { const b = btn('retro-primary', '', T('retro.fixIt')); b.addEventListener('click', () => fix(m.id, b)); f.appendChild(b); }
    box.appendChild(f);
  }
  if (m.note) box.appendChild(el('div', 'retro-note', '📝 ' + m.note));
  const pills = el('div', 'retro-reactions');
  for (const r of m.reactions || []) {
    const mine = (r.by || []).includes('user');
    const who = (r.by || []).map(nameOf).join(', ');
    const p = btn('retro-pill' + (mine ? ' is-mine' : ''), r.emoji + ' ' + who, '', () => react(m.id, r.emoji));
    p.setAttribute('aria-pressed', String(mine));
    p.append(el('span', '', r.emoji), el('span', 'retro-count', String((r.by || []).length)));
    pills.appendChild(p);
  }
  if ((m.reactions || []).length) {
    const add = btn('retro-pill retro-pill-add', T('retro.react'), '+', ev => showPicker(m.id, ev.currentTarget));
    add.dataset.picker = '1';
    pills.appendChild(add);
    box.appendChild(pills);
  }
  const n = counts[m.id];
  if (!inThread && n) box.appendChild(btn('retro-replies', '', n === 1 ? T('retro.repliesOne') : T('retro.replies', { n }), () => openThread(m.id)));
  const tools = el('div', 'retro-tools');
  const r = btn('retro-icon', T('retro.react'), '☺', ev => showPicker(m.id, ev.currentTarget));
  r.dataset.picker = '1';
  tools.appendChild(r);
  if (!inThread) tools.appendChild(btn('retro-icon', T('retro.reply'), '↩', () => openThread(m.thread_id || m.id)));
  tools.appendChild(btn('retro-icon', T('retro.copy'), '⧉', async () => {
    try { await navigator.clipboard.writeText(m.text); notify(T('retro.copied')); } catch (e) {}
  }));
  box.appendChild(tools);
  return box;
}

function paintTyping() {
  if (!st.dialog) return;
  const now = Date.now();
  for (const [id, exp] of st.typing) if (exp < now) st.typing.delete(id);
  const reviewing = id => { const a = agentOf(id); return !!(a && SPECIALIST_ICONS[a.avatar]) && !!st.channel; };
  const names = [...st.typing.keys()].filter(id => !reviewing(id)).map(nameOf);
  const text = !names.length ? '' : T(names.length === 1 ? 'retro.typingOne' : 'retro.typingMany', { a: core.typingNames(names, T('retro.and')) });
  st.dialog.querySelectorAll('.retro-typing').forEach(t => { t.textContent = text; t.classList.toggle('on', !!text); });
}

// --- directe: registre durable ---

function onEvent(e) {
  if (!e || !String(e.kind || '').startsWith('retro_')) return;
  const p = e.payload || {};
  if (e.id) st.cursor = Math.max(st.cursor, Number(e.id));
  if (!p.key || (p.key !== st.key && !(st.channel && p.key === st.channel.key))) return;
  if (e.kind === 'retro_typing') {
    (p.agents || []).forEach(id => { if (p.typing) st.typing.set(id, Date.now() + 120000); else st.typing.delete(id); });
    if (st.channel) render(); else paintTyping();
    return;
  }
  if (!st.channel) {
    // El canal s'ha obert des d'una altra finestra: el recarreguem sencer.
    // També quan s'està esperant el resum d'obertura: si la resposta HTTP es
    // perd, els missatges arriben igualment per aquí.
    if (e.kind === 'retro_message') call('/api/retro?key=' + encodeURIComponent(p.key)).then(d => {
      if (d.channel && d.channel.messages && d.channel.messages.length && !st.channel) { st.channel = d.channel; if (st.startKey === d.channel.key) { st.busy = false; st.startKey = ''; } render(); }
    }).catch(() => {});
    return;
  }
  if (e.kind === 'retro_message' && p.message) st.typing.delete(p.message.author);
  if (core.applyEvent(st.channel, e.kind, p)) render();
}

function stopLive() {
  if (st.abort) st.abort.abort();
  st.abort = null;
}

async function startLive() {
  stopLive();
  const ctrl = new AbortController();
  st.abort = ctrl;
  const loop = async () => {
    while (!ctrl.signal.aborted) {
      try {
        const r = await G().api('/api/v2/events/stream?after=' + st.cursor + '&limit=200', { signal: ctrl.signal });
        if (!r.ok || !r.body || !r.body.getReader) throw new Error('stream');
        const rd = r.body.getReader(), dec = new TextDecoder();
        let buf = '';
        for (;;) {
          const x = await rd.read();
          if (x.done) break;
          buf += dec.decode(x.value, { stream: true });
          let at;
          while ((at = buf.indexOf('\n\n')) >= 0) {
            const frame = buf.slice(0, at); buf = buf.slice(at + 2);
            const data = frame.split('\n').filter(l => l.startsWith('data:')).map(l => l.slice(5).trim()).join('\n');
            if (!data) continue;
            try { onEvent(JSON.parse(data)); } catch (e) {}
          }
        }
      } catch (e) {
        if (ctrl.signal.aborted) return;
        // Sense stream: polling del registre, mateix cursor.
        try {
          const r = await G().api('/api/v2/events?after=' + st.cursor + '&limit=200', { signal: ctrl.signal });
          if (r.ok) { const page = await r.json(); (page.events || []).forEach(onEvent); }
        } catch (e2) { if (ctrl.signal.aborted) return; }
        await new Promise(res => setTimeout(res, 1000));
      }
    }
  };
  loop();
}

// --- punts d'entrada ---

function retroButton(stamp) {
  const b = btn('retro-open', T('retro.open'), '');
  b.append(el('span', 'retro-open-hash', '#'), el('span', 'retro-open-label', T('retro.button')));
  b.dataset.retroOpen = '1';
  if (stamp) b.dataset.runId = stamp;
  b.addEventListener('click', ev => {
    ev.preventDefault();
    ev.stopPropagation();
    const run = b.dataset.runId;
    if (run && +run >= latestRun) markSeen();
    open(run ? { runId: run, autostart: true } : { autostart: true });
  });
  return b;
}

let latestRun = 0;
async function stampLatest(node) {
  try {
    const d = await call('/api/retro');
    latestRun = (d.latest && d.latest.run_id) || latestRun;
    if (latestRun && node && node.dataset.retroOpen) node.dataset.runId = String(latestRun);
    paintNav();
  } catch (e) {}
}

// Entrada de primer nivell a la barra lateral: el #retro és la manera
// d'entendre què ha fet cada agent, no pot quedar amagat dins d'Agents.
// Un punt avisa quan hi ha un torn acabat que encara no has comentat.
function seenRun() { try { return +localStorage.getItem('gregal_retro_seen') || 0; } catch (e) { return 0; } }
function markSeen() { try { if (latestRun) localStorage.setItem('gregal_retro_seen', String(latestRun)); } catch (e) {} paintNav(); }
let nav = null;
function paintNav() {
  if (!nav) return;
  nav.querySelector('.retro-nav-dot').hidden = !(latestRun && latestRun > seenRun());
}
function mountNav() {
  const side = document.querySelector('#sideMore .side-main') || document.querySelector('#side .side-main');
  if (!side || document.getElementById('retroNav')) return;
  nav = btn('nav-item', T('retro.navHelp'), '', () => { markSeen(); open(latestRun ? { runId: String(latestRun), autostart: true } : { autostart: false }); });
  nav.id = 'retroNav';
  nav.append(el('span', 'retro-nav-label', T('retro.button')), el('span', 'retro-nav-dot'));
  nav.querySelector('.retro-nav-dot').hidden = true;
  side.appendChild(nav);
}

function decorate(root) {
  const scope = root && root.querySelectorAll ? root : document;
  scope.querySelectorAll('.work-group:not(.running) > summary').forEach(s => {
    if (s.querySelector('[data-retro-open]')) return;
    const b = retroButton();
    s.appendChild(b);
    if (s.parentElement === [...document.querySelectorAll('#main .work-group')].pop()) stampLatest(b);
  });
  scope.querySelectorAll('.run-summary').forEach(card => {
    if (card.querySelector('[data-retro-open]')) return;
    let bar = card.querySelector('.run-summary-actions');
    if (!bar) { bar = el('div', 'run-summary-actions'); card.appendChild(bar); }
    const b = retroButton();
    bar.appendChild(b);
    stampLatest(b);
  });
  document.querySelectorAll('.agents-tabs').forEach(tabs => {
    if (tabs.querySelector('[data-retro-tab]')) return;
    const tab = btn('agents-tab retro-tab', '', T('retro.button'), () => open({ autostart: false }));
    tab.setAttribute('role', 'tab');
    tab.setAttribute('aria-selected', 'false');
    tab.dataset.retroTab = '1';
    tabs.appendChild(tab);
  });
}

function boot() {
  mountNav();
  stampLatest(null);
  decorate(document);
  const main = document.getElementById('main');
  if (main) {
    new MutationObserver(muts => {
      if (muts.some(m => m.type === 'childList' ? m.addedNodes.length : m.target.classList && m.target.classList.contains('work-group'))) decorate(main);
    }).observe(main, { childList: true, subtree: true, attributes: true, attributeFilter: ['class'] });
  }
  document.addEventListener('gregal:idioma', () => {
    document.querySelectorAll('[data-retro-open]').forEach(b => { b.title = T('retro.open'); b.setAttribute('aria-label', T('retro.open')); b.querySelector('.retro-open-label').textContent = T('retro.button'); });
    document.querySelectorAll('[data-retro-tab]').forEach(b => { b.textContent = T('retro.button'); });
    if (nav) { nav.title = T('retro.navHelp'); nav.setAttribute('aria-label', T('retro.navHelp')); nav.querySelector('.retro-nav-label').textContent = T('retro.button'); }
    if (st.dialog) { st.dialog.remove(); st.dialog = null; if (st.open) open({ key: st.channel ? st.channel.key : st.key }); }
  });
  setInterval(paintTyping, 5000);
}

if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot);
else boot();
window.gregalRetro = { open, close };

// --- to dels agents (personatge) ---

// Referència al canal per a /api/retro/start (torn o equip).
function channelRef(key) {
  return key.startsWith('team-') ? { source: 'team' } : key.startsWith('run-') ? { run_id: Number(key.slice(4)) } : {};
}

// Selector a la capçalera: canvia el to de les respostes següents d'aquest
// canal (el servidor el desa; les regles de rebuts no canvien).
function personaPicker() {
  const sel = el('select', 'retro-persona');
  sel.setAttribute('aria-label', T('retro.persona'));
  sel.title = T('retro.persona.help');
  PERSONAS.forEach(([v, k]) => { const o = el('option', '', T(k)); o.value = v; sel.appendChild(o); });
  sel.hidden = true;
  sel.addEventListener('change', async () => {
    if (!st.channel) return;
    const value = sel.value;
    try {
      const out = await call('/api/retro/start', Object.assign(channelRef(st.channel.key), { persona: value }));
      if (out.channel) st.channel = out.channel;
      notify(T('retro.persona.changed', { p: sel.options[sel.selectedIndex].text }));
    } catch (e) { notify(T('retro.failed') + e.message); }
    renderHead();
  });
  return sel;
}
