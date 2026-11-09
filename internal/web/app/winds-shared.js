// Rotllanes compartides a la web: xats de grup amb altres persones del mateix
// servidor i els vents de qui el crea. Invitació (Accepta / Rebutja) en obrir
// l'app; després el xat surt a la barra dels vents. Servidor: winds_shared.go.
// winds-workspace.js crida nav() i view(); tot el text remot va per textContent
// (o pel renderitzador de markdown segur, com els altres xats).
(function (root) {
  'use strict';
  const ca = () => document.documentElement.lang === 'ca', t = (a, b) => ca() ? a : b;
  const state = { me: '', chats: [], invites: [], later: new Set(), ctx: null, open: null, history: [], busy: false, sending: false, off: false };
  const el = (tag, cls, text) => { const n = document.createElement(tag); if (cls) n.className = cls; if (text !== undefined) n.textContent = text; return n; };
  const btn = (label, action, cls = 'vw-button') => { const n = el('button', cls, label); n.type = 'button'; n.onclick = () => Promise.resolve().then(action).catch(fail); return n; };
  async function call(url, method = 'GET', body) {
    const r = await root.api(url, { method, ...(body === undefined ? {} : { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }) });
    if (r.status === 404 && url === '/api/winds/shared') { state.off = true; return { chats: [], invites: [] }; }
    if (!r.ok) throw new Error((await r.text()).trim() || ('HTTP ' + r.status));
    return r.json();
  }
  function fail(e) { const s = document.querySelector('.vw-dialog[open] .vw-error') || document.querySelector('.vw-status'); if (s) { s.textContent = t('No s’ha pogut fer: ', 'Couldn’t do it: ') + e.message; s.setAttribute('role', 'alert'); } }
  function dialog(title) {
    const d = el('dialog', 'vw-dialog'); d.setAttribute('aria-label', title);
    const head = el('header'); head.append(el('h2', '', title), btn('×', () => d.close(), 'vw-close')); d.append(head);
    document.body.append(d); d.addEventListener('close', () => d.remove()); d.showModal(); return d;
  }
  const humans = c => [c.owner, ...(c.people || []).filter(p => p.status === 'accepted').map(p => p.name)];

  async function refresh() {
    if (!root.api || state.off) return;
    try {
      const d = await call('/api/winds/shared');
      state.me = d.me || ''; state.chats = d.chats || []; state.invites = d.invites || [];
      state.ctx?.rerender?.(); invitePopup();
    } catch (_) { /* sense xarxa: ja ho tornarem a provar */ }
  }

  // La invitació salta sola; tancar-la la deixa per a més tard (fins a recarregar).
  function invitePopup() {
    if (document.querySelector('.vw-shared-invite')) return;
    const inv = state.invites.find(i => !state.later.has(i.id)); if (!inv) return;
    const d = dialog(t('Invitació a un xat compartit', 'Invitation to a shared chat')); d.classList.add('vw-shared-invite');
    d.append(el('p', '', t(`${inv.owner} et convida a «${inv.name}».`, `${inv.owner} invites you to “${inv.name}”.`)));
    const others = (inv.people || []).map(p => p.name).filter(n => n !== state.me);
    if (others.length) d.append(el('p', 'vw-muted', t('També hi són: ', 'Also in it: ') + others.join(', ')));
    if ((inv.winds || []).length) d.append(el('p', 'vw-muted', t('Vents: ', 'Winds: ') + inv.winds.map(w => w.name).join(', ')));
    d.append(el('p', 'vw-error'));
    let answered = false;
    const answer = async accept => {
      await call('/api/winds/shared/answer', 'POST', { id: inv.id, accept }); answered = true; d.close();
      await refresh(); if (accept) openWhenReady(inv.id);
    };
    const row = el('div', 'vw-dialog-actions'); row.append(btn(t('Rebutja', 'Decline'), () => answer(false)), btn(t('Accepta', 'Accept'), () => answer(true), 'vw-primary')); d.append(row);
    d.addEventListener('close', () => { if (!answered) state.later.add(inv.id); setTimeout(invitePopup, 0); });
  }

  // Obre un xat encara que els Vents no siguin a la vista: hi va i l'obre quan la barra hi és.
  let pendingOpen = '';
  function openWhenReady(id) {
    const c = state.chats.find(c => c.id === id);
    if (c && state.ctx && document.body.classList.contains('shell-winds')) { state.ctx.open(c); return; }
    // Al Codi no hi ha Vents: el xat s'obre a /vents.
    if (root.gregalProduct && root.gregalProduct !== 'vents') { root.location.assign('/vents?shared=' + encodeURIComponent(id)); return; }
    pendingOpen = id; root.setView?.('vents');
  }
  function nav(navEl, ctx) {
    state.ctx = ctx;
    if (pendingOpen) { const c = state.chats.find(c => c.id === pendingOpen); if (c) { pendingOpen = ''; setTimeout(() => ctx.open(c), 0); } }
    const head = el('div', 'vw-section-label'); head.append(el('span', '', t('Compartits', 'Shared')));
    if (!state.off) head.append(btn('+', creator, 'vw-small'));
    if (state.off) return;
    navEl.append(head);
    for (const c of state.chats) {
      const n = btn(c.name, () => ctx.open(c), 'vw-nav-item vw-thread'); n.title = humans(c).join(', ');
      n.dataset.selected = String(state.open?.id === c.id && ctx.active?.id === c.id); navEl.append(n);
    }
    if (!state.chats.length) navEl.append(el('p', 'vw-nav-empty', t('Xats amb altres persones del servidor.', 'Chats with other people on this server.')));
  }

  async function creator() {
    const d = dialog(t('Nou xat compartit', 'New shared chat'));
    d.append(el('p', 'vw-muted', t('Un xat de grup amb altres persones d’aquest servidor i els teus vents. Cadascú rep una invitació i hi entra quan l’accepta. Aquí els vents només saben el que es diu en aquest xat.', 'A group chat with other people on this server and your winds. Each person gets an invitation and joins when they accept. Here the winds only know what is said in this chat.')));
    const wrap = el('label', 'vw-field', t('Nom del xat', 'Chat name')), name = el('input'); wrap.append(name); d.append(wrap);
    const checks = (title, items) => {
      const box = el('fieldset', 'vw-shared-pick'); box.append(el('legend', '', title)); const out = [];
      for (const [value, label] of items) { const l = el('label', 'vw-check'), c = el('input'); c.type = 'checkbox'; c.value = value; out.push(c); l.append(c, el('span', '', label)); box.append(l); }
      if (!items.length) box.append(el('p', 'vw-muted', t('No n’hi ha cap.', 'None available.')));
      d.append(box); return out;
    };
    const [people, winds] = await Promise.all([call('/api/winds/shared/people').then(x => x.people || []), call('/api/winds').then(x => x.winds || [])]);
    const pc = checks(t('Persones', 'People'), people.map(p => [p, p]));
    const wc = checks(t('Vents que hi parlen', 'Winds who talk in it'), winds.map(w => [w.id, w.name]));
    d.append(el('p', 'vw-error'));
    d.append(btn(t('Convida', 'Invite'), async () => {
      const body = { name: name.value.trim(), people: pc.filter(c => c.checked).map(c => c.value), winds: wc.filter(c => c.checked).map(c => c.value) };
      if (!body.name || !body.people.length || !body.winds.length) throw new Error(t('Cal un nom, almenys una persona i un vent.', 'A name, at least one person and one wind are needed.'));
      const saved = await call('/api/winds/shared', 'POST', body); d.close(); await refresh();
      state.ctx?.open(state.chats.find(c => c.id === saved.chat?.id) || saved.chat);
    }, 'vw-primary'));
    name.focus();
  }

  let timer = 0, viewEpoch = 0;
  function view(content, ctx) {
    const chat = ctx.chat; state.open = chat; const epoch = ++viewEpoch; clearTimeout(timer);
    content.classList.add('vw-thread-view');
    const info = el('div', 'vw-shared-info');
    info.append(el('span', 'vw-muted', humans(chat).join(', ') + ' · ' + (chat.winds || []).map(w => w.name).join(', ')));
    info.append(btn(chat.mine ? t('Esborra el xat', 'Delete chat') : t('Surt del xat', 'Leave chat'), async () => {
      if (!confirm(chat.mine ? t('Esborrar-lo per a tothom, amb tota la conversa?', 'Delete it for everyone, with the whole conversation?') : t('Sortir del xat? No el veuràs fins que et tornin a convidar.', 'Leave the chat? You won’t see it until you’re invited again.'))) return;
      await call('/api/winds/shared/leave', 'POST', { id: chat.id }); state.open = null; await refresh(); ctx.back();
    }, 'vw-small'));
    const messages = el('div', 'vw-messages'), form = el('form', 'vw-composer'), input = el('textarea');
    input.rows = 2; input.placeholder = t('Escriu al grup…', 'Write to the group…'); input.setAttribute('aria-label', input.placeholder);
    input.addEventListener('keydown', e => { if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) { e.preventDefault(); form.requestSubmit(); } });
    const foot = el('div', 'vw-composer-foot'), send = el('button', 'vw-send', t('Envia', 'Send')); send.type = 'submit'; foot.append(send); form.append(input, foot);
    content.append(info, messages, form);
    const paint = () => {
      const atEnd = messages.scrollHeight - messages.scrollTop - messages.clientHeight < 40;
      messages.replaceChildren();
      if (!state.history.length) messages.append(el('p', 'vw-empty', t('Encara no hi ha res. Saluda!', 'Nothing here yet. Say hi!')));
      for (const m of state.history) {
        const mine = !!m.mine, w = (chat.winds || []).find(w => w.id === m.speaker_id);
        const row = el('article', mine ? 'vw-message vw-user' : 'vw-message');
        if (!mine) row.append(w ? root.gregalWinds.face(w, false) : el('span', 'vw-shared-person', (m.speaker_name || '?').slice(0, 1).toUpperCase()));
        const copy = el('div'), body = el('div', 'vw-message-text');
        const text = mine ? (m.text || '') : (root.gregalChoices ? root.gregalChoices.strip(m.text || '') : (m.text || ''));
        if (root.gregalMD) body.innerHTML = root.gregalMD.render(text); else body.textContent = text;
        copy.append(el('strong', '', mine ? t('Tu', 'You') : (m.speaker_name || '')), body); row.append(copy); messages.append(row);
      }
      if (state.busy) messages.append(el('p', 'vw-muted', t('Els vents hi pensen…', 'The winds are thinking…')));
      if (atEnd || state.sending) messages.scrollTop = messages.scrollHeight;
    };
    const poll = async () => {
      if (epoch !== viewEpoch || !messages.isConnected) return;
      try { const d = await call('/api/winds/shared/chat?id=' + encodeURIComponent(chat.id)); if (epoch !== viewEpoch) return; state.history = d.history || []; state.busy = !!d.busy || state.sending; paint(); }
      catch (e) { fail(e); }
      timer = setTimeout(poll, state.busy ? 2000 : 4000);
    };
    form.onsubmit = async e => {
      e.preventDefault(); const text = input.value.trim(); if (!text || state.sending) return;
      state.sending = true; state.busy = true; input.value = ''; send.disabled = true;
      state.history = [...state.history, { from: 'user', mine: true, text, speaker_name: state.me }]; paint();
      try { const d = await call('/api/winds/shared/chat', 'POST', { id: chat.id, message: text }); if (epoch === viewEpoch) state.history = d.history || state.history; }
      catch (err) { input.value = text; fail(err); }
      finally { state.sending = false; state.busy = false; send.disabled = false; if (epoch === viewEpoch) { paint(); input.focus(); } }
    };
    state.history = []; paint(); poll(); input.focus();
  }

  root.gregalShared = { nav, view, refresh, chats: () => state.chats };
  // La llista i les invitacions es miren en obrir i cada 30 s.
  const start = () => {
    const wanted = new URLSearchParams(root.location.search).get('shared');
    if (wanted) pendingOpen = wanted;
    refresh(); setInterval(refresh, 30000);
  };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', () => setTimeout(start, 1500)); else setTimeout(start, 1500);
})(window);
