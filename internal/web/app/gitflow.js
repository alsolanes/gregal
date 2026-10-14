// Commit, PR i worktrees des de la pàgina Canvis.
//
// Els diàlegs són accions de l'usuari: el que s'hi escriu va tal qual a
// /api/git/commit, /api/git/pr o /api/worktrees/apply. «Suggereix» només
// omple el camp amb una proposta del rol chat feta a partir del diff.

import { t as T } from './i18n.js';

const G = () => window.gregal;
const $ = id => document.getElementById(id);

async function j(url, opts) {
  const r = await G().api(url, opts);
  if (!r.ok) throw new Error((await r.text()).trim() || ('HTTP ' + r.status));
  return r.json();
}

function el(tag, cls, text) {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text != null) n.textContent = text;
  return n;
}

function btn(label, cls, onclick) {
  const b = el('button', cls || 'ghost', label);
  b.type = 'button';
  b.onclick = onclick;
  return b;
}

// modal crea un <dialog> amb títol, pista, cos i fila d'accions.
function modal(title, hint) {
  const d = el('dialog', 'gitflow-dialog');
  d.append(el('h2', '', title));
  if (hint) d.append(el('p', 'gitflow-hint', hint));
  const body = el('div', 'gitflow-body');
  const status = el('p', 'gitflow-status');
  status.setAttribute('role', 'status');
  const actions = el('div', 'gitflow-actions');
  d.append(body, status, actions);
  d.addEventListener('close', () => d.remove());
  document.body.append(d);
  return { d, body, status, actions };
}

function field(label, input) {
  const wrap = el('label', 'gitflow-field');
  wrap.append(el('span', '', label), input);
  return wrap;
}

function refreshChanges() { $('changesRefresh')?.click(); }

function report(ok, summary) {
  try { G().activityItem('git', ok ? T('git.done') : T('git.failed').replace(/:\s*$/, ''), summary, ok ? 'ok' : 'bad'); } catch (e) {}
}

// busy desactiva els botons mentre dura una petició.
async function busy(m, fn) {
  const bs = [...m.actions.querySelectorAll('button')];
  bs.forEach(b => { b.disabled = true; });
  try { return await fn(); }
  finally { bs.forEach(b => { b.disabled = false; }); }
}

async function suggest(m, kind, apply) {
  m.status.textContent = T('git.suggesting');
  try {
    const r = await j('/api/git/suggest', { method: 'POST', body: JSON.stringify({ kind }) });
    apply(r);
    m.status.textContent = '';
  } catch (e) { m.status.textContent = T('git.failed') + e.message; }
}

export function commitDialog() {
  const m = modal(T('git.commitTitle'), T('git.commitHint'));
  const msg = el('textarea', 'gitflow-text');
  msg.rows = 6;
  m.body.append(field(T('git.message'), msg));
  m.actions.append(
    btn(T('git.suggest'), 'ghost', () => busy(m, () => suggest(m, 'commit', r => { msg.value = r.message || ''; }))),
    btn(T('git.cancel'), 'ghost', () => m.d.close()),
    btn(T('git.commitDo'), 'primary', () => busy(m, async () => {
      if (!msg.value.trim()) { msg.focus(); return; }
      try {
        const r = await j('/api/git/commit', { method: 'POST', body: JSON.stringify({ message: msg.value }) });
        report(true, r.summary);
        m.d.close();
        refreshChanges();
      } catch (e) { m.status.textContent = T('git.failed') + e.message; report(false, e.message); }
    })),
  );
  m.d.showModal();
  msg.focus();
}

export function prDialog() {
  const m = modal(T('git.prTitle'), T('git.prHint'));
  const title = el('input', 'gitflow-input');
  const body = el('textarea', 'gitflow-text');
  body.rows = 8;
  const base = el('input', 'gitflow-input');
  const draft = el('input');
  draft.type = 'checkbox';
  const draftRow = el('label', 'gitflow-check');
  draftRow.append(draft, el('span', '', T('git.draft')));
  m.body.append(field(T('git.title'), title), field(T('git.body'), body), field(T('git.base'), base), draftRow);
  m.actions.append(
    btn(T('git.suggest'), 'ghost', () => busy(m, () => suggest(m, 'pr', r => { title.value = r.title || ''; body.value = r.body || ''; }))),
    btn(T('git.cancel'), 'ghost', () => m.d.close()),
    btn(T('git.prDo'), 'primary', () => busy(m, async () => {
      if (!title.value.trim()) { title.focus(); return; }
      try {
        const r = await j('/api/git/pr', { method: 'POST', body: JSON.stringify({ title: title.value, body: body.value, base: base.value.trim(), draft: draft.checked }) });
        m.status.textContent = r.summary;
        report(true, r.summary);
        refreshChanges();
      } catch (e) { m.status.textContent = T('git.failed') + e.message; report(false, e.message); }
    })),
  );
  m.d.showModal();
  title.focus();
}

export async function worktreeDialog(name) {
  const m = modal(T('git.wtTitle') + ' · ' + name, T('git.wtHint'));
  const pre = el('pre', 'gitflow-diff', '…');
  m.body.append(pre);
  m.d.showModal();
  try {
    const r = await j('/api/worktrees/apply', { method: 'POST', body: JSON.stringify({ name, how: 'diff' }) });
    pre.textContent = (r.diff || '').trim() ? r.diff + (r.truncated ? '\n' + T('git.wtTruncated') : '') : T('git.wtEmpty');
  } catch (e) { pre.textContent = T('git.failed') + e.message; }
  const apply = how => busy(m, async () => {
    try {
      const r = await j('/api/worktrees/apply', { method: 'POST', body: JSON.stringify({ name, how }) });
      m.status.textContent = r.summary;
      report(true, r.summary);
    } catch (e) { m.status.textContent = T('git.failed') + e.message; report(false, e.message); }
  });
  m.actions.append(
    btn(T('git.cancel'), 'ghost', () => m.d.close()),
    btn(T('git.wtMerge'), 'ghost', () => apply('merge')),
    btn(T('git.wtPatch'), 'primary', () => apply('patch')),
  );
}

// El botó «Porta al repo principal» només surt a les sessions que treballen
// dins d'un worktree de gregal.
async function refreshWorktreeButton() {
  const b = $('changesWorktree');
  if (!b) return;
  try {
    const r = await j('/api/worktrees');
    b.hidden = !r.current;
    b.dataset.name = r.current || '';
  } catch (e) { b.hidden = true; }
}

function init() {
  const c = $('changesCommit'), p = $('changesPR'), w = $('changesWorktree');
  if (c) c.onclick = () => commitDialog();
  if (p) p.onclick = () => prDialog();
  if (w) w.onclick = () => { if (w.dataset.name) worktreeDialog(w.dataset.name); };
  const page = $('canvisPage');
  if (page && 'MutationObserver' in window) {
    new MutationObserver(() => { if (!page.hidden) refreshWorktreeButton(); })
      .observe(page, { attributes: true, attributeFilter: ['hidden'] });
  }
  $('changesRefresh')?.addEventListener('click', refreshWorktreeButton);
  window.gregalGitflow = { commitDialog, prDialog, worktreeDialog };
}

if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
else init();
