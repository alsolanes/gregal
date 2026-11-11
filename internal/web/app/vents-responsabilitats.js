// Gregal Vents → Responsabilitats (B6): el que Gregal vigila per a la persona.
// Mateixa API i mateix comportament que ResponsabilitatsSheet/RespWhySheet
// d'Android: llista amb estat, fonts, què fa sol i pressupost del dia;
// pausar, reprendre, acabar o esborrar; crear-ne una amb una pàgina web o
// l'agenda; i «per què ho has fet?» d'un missatge del vent.

import { t } from './i18n.js';

const api = (p, o) => (window.gregal?.api ? window.gregal.api(p, o) : window.api ? window.api(p, o) : fetch(p, o));
const getJSON = async path => { const r = await api(path); if (!r.ok) throw new Error((await r.text()).trim() || r.status); return r.json(); };
const post = async (path, body) => {
  const r = await api(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  if (!r.ok) {
    let msg = (await r.text()).trim();
    try { msg = JSON.parse(msg).error || msg; } catch (_) { /* text pla */ }
    throw new Error(msg || r.status);
  }
  return r.json().catch(() => ({}));
};
const el = (tag, cls, text) => { const e = document.createElement(tag); if (cls) e.className = cls; if (text != null) e.textContent = text; return e; };
const btn = (label, fn, cls = 'auto-btn') => { const b = el('button', cls, label); b.type = 'button'; b.onclick = fn; return b; };

// --- lògica pura (testejable sense DOM) ---

/** L'estat que es mostra: esperar l'ordinador passa per davant d'«activa». */
export function stateKey(r) {
  if (r.waitingComputer && r.state === 'active') return 'waitingComputer';
  return ['active', 'paused', 'done'].includes(r.state) ? r.state : 'active';
}

/** Accions que admet: un cor es pausa però no s'acaba ni s'esborra. */
export function actionsFor(r) {
  const heart = r.kind === 'heartbeat';
  if (r.state === 'active') return heart ? ['pause'] : ['pause', 'finish'];
  if (r.state === 'paused') return heart ? ['resume'] : ['resume', 'finish'];
  return heart ? [] : ['delete'];
}

/** «consultar, preparar (…)»: el que fa sola, en llenguatge normal. */
export function canDo(r) {
  const allowed = (r.allowed && r.allowed.length) ? r.allowed : ['explore', 'prepare'];
  return allowed.map(a => t('resp.level.' + a, a)).join(', ');
}

export const sourceLabel = s => s.label || s.url || s.kind || '';

/** «Avui: 1/6 preparacions · 0/3 missatges». */
export function budgetLine(r) {
  const b = r.budget || {}, s = r.spent || {};
  return t('resp.today') + ': ' + (s.preps || 0) + '/' + (b.prepsPerDay || 0) + ' ' + t('resp.preps') +
    ' · ' + (s.messages || 0) + '/' + (b.messagesPerDay || 0) + ' ' + t('resp.msgs');
}

/** El missatge del fil ve d'una responsabilitat o del cor: porta run o resultat. */
export const isProactive = m => !!m && m.from !== 'user' && ((m.relatedRunId || 0) > 0 || (Array.isArray(m.resultRefs) && m.resultRefs.length > 0));

/** El cos de POST per crear-ne una des del formulari. */
export function createBody(form) {
  const body = { action: 'create', wind: form.wind, objective: String(form.objective || '').trim(), agenda: !!form.agenda };
  const url = String(form.url || '').trim();
  if (url) body.urls = [url];
  const every = parseInt(form.every, 10);
  if (every > 0) body.everyMinutes = every;
  return body;
}

export const canCreate = form => !!form.wind && !!String(form.objective || '').trim() && (!!String(form.url || '').trim() || !!form.agenda);

// --- DOM ---

function card(r, onAction, onWhy) {
  const c = el('article', 'resp-card');
  c.dataset.id = r.id;
  const top = el('div', 'resp-top');
  const who = r.kind === 'heartbeat' ? t('resp.heartbeat') + (r.windName ? ' · ' + r.windName : '') : (r.windName || r.wind || '');
  const st = el('span', 'resp-state resp-' + stateKey(r), t('resp.state.' + stateKey(r)));
  top.append(el('span', 'resp-who', who), st);
  c.append(top, el('h3', 'resp-goal', r.objective || ''));
  const srcs = (r.sources || []).map(sourceLabel).filter(Boolean);
  if (srcs.length) c.append(el('p', 'auto-help', t('resp.sources') + ': ' + srcs.join(' · ')));
  const bad = (r.sources || []).find(s => s.error);
  if (bad) { const p = el('p', 'resp-error', bad.error); c.append(p); }
  c.append(el('p', 'auto-help', t('resp.canDo') + ': ' + canDo(r) + '. ' + t('resp.askAlways') + '.'));
  c.append(el('p', 'auto-help', budgetLine(r) + (r.pending > 0 ? ' · ' + r.pending + ' ' + t('resp.pending') : '')));
  const last = (r.log || [])[0];
  if (last) c.append(el('p', 'resp-last', t('resp.act.' + last.kind, last.kind)));
  const row = el('div', 'auto-actions');
  for (const a of actionsFor(r)) row.append(btn(t('resp.' + a), () => onAction(a, r), a === 'resume' ? 'auto-btn primary' : 'auto-btn'));
  const prepared = (r.log || []).find(a => a.kind === 'prepared');
  if (prepared) row.append(btn(t('resp.whyShort'), () => onWhy({ responsabilitat: r, act: prepared })));
  c.append(row);
  return c;
}

function form(winds, onCreate, onCancel) {
  const f = el('div', 'auto-form resp-form');
  const wsel = el('select'); wsel.setAttribute('aria-label', t('resp.wind'));
  for (const w of winds) { const o = el('option', '', w.name); o.value = w.id; wsel.append(o); }
  const goal = el('textarea'); goal.rows = 2; goal.placeholder = t('resp.goal'); goal.setAttribute('aria-label', goal.placeholder);
  const url = el('input'); url.type = 'url'; url.placeholder = t('resp.url'); url.setAttribute('aria-label', url.placeholder);
  const agendaLab = el('label', 'vw-check'), agenda = el('input'); agenda.type = 'checkbox';
  agendaLab.append(agenda, el('span', '', t('resp.agenda')));
  const every = el('input'); every.type = 'number'; every.min = '15'; every.value = '60'; every.setAttribute('aria-label', t('resp.every'));
  const everyLab = el('label', 'resp-every'); everyLab.append(el('span', '', t('resp.every')), every);
  const values = () => ({ wind: wsel.value, objective: goal.value, url: url.value, agenda: agenda.checked, every: every.value });
  const row = el('div', 'auto-actions');
  row.append(btn(t('resp.cancel'), onCancel), btn(t('resp.create'), () => { if (canCreate(values())) onCreate(createBody(values())); }, 'auto-btn primary'));
  f.append(wsel, goal, url, agendaLab, everyLab, row);
  return f;
}

/** Pinta la llista (i el formulari de crear) dins de box. */
export async function renderResponsabilitats(box) {
  const status = el('p', 'auto-status'); status.setAttribute('role', 'status');
  const list = el('div', 'resp-list');
  box.replaceChildren(status, list);
  let creating = false;
  const draw = async () => {
    const [data, winds] = await Promise.all([
      getJSON('/api/winds/responsabilitats'),
      getJSON('/api/winds').catch(() => ({ winds: [] })),
    ]);
    const items = data.responsabilitats || [], windList = winds.winds || [];
    list.replaceChildren();
    if (!items.length && !creating) list.append(el('p', 'auto-empty', t('resp.empty')));
    for (const r of items) list.append(card(r, act, openWhyData));
    if (creating) {
      list.append(form(windList, async body => {
        try { await post('/api/winds/responsabilitats', body); creating = false; status.textContent = t('resp.created'); await draw(); }
        catch (e) { status.textContent = t('resp.failed') + (e.message || e); status.setAttribute('role', 'alert'); }
      }, () => { creating = false; draw(); }));
    } else if (windList.length) {
      list.append(btn(t('resp.new'), () => { creating = true; draw(); }));
    }
  };
  const act = async (action, r) => {
    status.textContent = t('resp.wait'); status.setAttribute('role', 'status');
    try { await post('/api/winds/responsabilitats', { action, id: r.id }); status.textContent = ''; await draw(); }
    catch (e) { status.textContent = t('resp.failed') + (e.message || e); status.setAttribute('role', 'alert'); }
  };
  try { await draw(); }
  catch (e) { status.textContent = t('resp.loadError'); status.setAttribute('role', 'alert'); }
}

// --- «per què ho has fet?» ---

/** El contingut de «per què?» com a node (why null = el missatge no ve de cap responsabilitat). */
export function whyBody(why) {
  const box = el('div', 'resp-why');
  if (!why) { box.append(el('p', 'auto-help', t('resp.notFromResp'))); return box; }
  const r = why.responsabilitat || {}, a = why.act || {};
  const label = text => el('h4', 'resp-label', text);
  box.append(label(t('resp.goal')), el('p', '', r.objective || ''));
  box.append(label(t('resp.sources')));
  const ul = el('ul', 'resp-sources');
  for (const s of (a.sources && a.sources.length ? a.sources : (r.sources || []).map(sourceLabel))) ul.append(el('li', '', s));
  box.append(ul);
  if (a.changes && a.changes.length) {
    box.append(label(t('resp.changes')));
    for (const c of a.changes) box.append(el('pre', 'resp-change', c));
  }
  box.append(label(t('resp.canDo')), el('p', '', canDo(r) + '. ' + t('resp.askAlways') + '.'));
  if (a.proposal) box.append(label(t('resp.proposal')), el('p', '', a.proposal));
  if (a.held) box.append(el('p', 'auto-help', t('resp.held')));
  return box;
}

function openWhyData(why) {
  const d = el('dialog', 'vw-dialog resp-dialog');
  d.setAttribute('aria-label', t('resp.why'));
  const head = el('div', 'resp-dialog-head');
  head.append(el('h2', '', t('resp.why')), btn(t('resp.close'), () => d.close()));
  d.append(head, whyBody(why));
  d.addEventListener('close', () => d.remove());
  document.body.append(d);
  if (d.showModal) d.showModal(); else d.setAttribute('open', '');
  return d;
}

/** Obre «per què ho has fet?» d'un missatge del fil del vent. */
export async function openWhy(messageId) {
  let why = null;
  try { why = await getJSON('/api/winds/responsabilitats/why?message_id=' + encodeURIComponent(messageId)); }
  catch (_) { why = null; }
  return openWhyData(why);
}

if (typeof window !== 'undefined') {
  window.gregalResponsabilitats = { render: renderResponsabilitats, openWhy, isProactive, whyLabel: () => t('resp.why') };
}
