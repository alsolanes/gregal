// Backlog: la llista de feina del projecte de la sessió (internal/backlog,
// /api/backlog). A l'esquerra les tasques agrupades per estat; a la dreta,
// la tasca triada amb els criteris, les accions que li toquen i el log.
// Les regles (què pot estar a punt, què demana motiu o prova) les aplica el
// servidor: aquí només es mostren els errors que torna.
import { idioma } from './i18n.js';

const $ = id => document.getElementById(id);
const tx = (ca, en) => (idioma() === 'en' ? en : ca);
const G = () => window.gregal;
const P = () => window.gregalPanels;
const el = (tag, cls, text) => { const e = document.createElement(tag); if (cls) e.className = cls; if (text != null) e.textContent = text; return e; };
const btn = (label, fn, cls = 'bl-btn') => { const b = el('button', cls, label); b.type = 'button'; b.onclick = fn; return b; };

async function call(method, body) {
  const opts = { method };
  if (body) { opts.headers = { 'Content-Type': 'application/json' }; opts.body = JSON.stringify(body); }
  const r = await G().api('/api/backlog', opts);
  const j = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(j.error || ('HTTP ' + r.status));
  return j;
}

const STATUS = {
  proposed: ['Proposada', 'Proposed'], ready: ['A punt', 'Ready'], in_progress: ['En curs', 'In progress'],
  in_review: ['En revisió', 'In review'], done: ['Feta', 'Done'], blocked: ['Bloquejada', 'Blocked'], dropped: ['Descartada', 'Dropped'],
};
const TYPES = {
  feature: ['Funcionalitat', 'Feature', 'Comportament nou que algú pot veure.', 'New behaviour someone can see.'],
  bug: ['Error', 'Bug', 'Fa una cosa diferent de la que hauria de fer. Cal saber com reproduir-ho.', 'It does something other than it should. Needs steps to reproduce.'],
  task: ['Tasca', 'Task', 'Feina sense canvi visible: refactor, dependències, CI, documentació.', 'Work with no visible change: refactor, dependencies, CI, docs.'],
  spike: ['Investigació', 'Spike', 'Una pregunta amb temps limitat. El resultat és una decisió, no codi.', 'A time-boxed question. The result is a decision, not code.'],
  epic: ['Èpica', 'Epic', 'Agrupa tasques massa grans per a un sol canvi. No es treballa directament.', 'Groups items too big for one change. Not worked on directly.'],
};
const PRIO = { now: ['Ara', 'Now'], next: ['Després', 'Next'], later: ['Més endavant', 'Later'] };
const lab = (map, k) => (map[k] ? tx(map[k][0], map[k][1]) : k);
const OPEN = ['in_progress', 'in_review', 'ready', 'blocked', 'proposed'];
const FILTERS = [['open', 'Obertes', 'Open'], ['done', 'Fetes', 'Done'], ['dropped', 'Descartades', 'Dropped'], ['all', 'Totes', 'All']];

let data = null;
let filter = 'open';
let typeFilter = '';
let selected = 0;
let mode = 'view'; // view | edit | new
let flash = null; // { text, error }

const items = () => (data && data.items) || [];
const byId = id => items().find(it => it.id === id);

export async function load() {
  const box = $('backlogInner');
  if (!box) return;
  if (!data) box.replaceChildren(el('p', 'bl-muted', tx('Carregant…', 'Loading…')));
  try {
    data = await call('GET');
  } catch (e) {
    box.replaceChildren(el('p', 'bl-error', tx('No s’ha pogut obrir el backlog: ', 'Couldn’t open the backlog: ') + e.message));
    return;
  }
  if (selected && !byId(selected)) selected = 0;
  pollReviews();
  render();
}

// act fa una crida, recarrega i deixa un missatge (o l'error) a la fitxa.
async function act(body, ok, errSlot) {
  try {
    const r = await call('POST', body);
    if (r.item) selected = r.item.id;
    mode = 'view';
    flash = ok ? { text: ok } : null;
    await load();
    return r;
  } catch (e) {
    // Dins d'un formulari l'error va al seu lloc: tornar a pintar perdria
    // el que s'hi ha escrit.
    if (errSlot) { errSlot.textContent = e.message; errSlot.hidden = false; return null; }
    flash = { text: e.message, error: true };
    render();
    return null;
  }
}

function render() {
  const box = $('backlogInner');
  if (!box || !data) return;
  box.replaceChildren(head(), toolbar(), body());
}

function head() {
  const h = el('header', 'bl-head');
  const left = el('div', 'bl-head-text');
  left.append(el('div', 'page-kicker', tx('Feina del projecte', 'Project work')), el('h1', 'page-title', 'Backlog'));
  const where = el('p', 'page-copy bl-where');
  where.append(el('b', '', data.project || data.root), document.createTextNode(' · '));
  if (data.storage === 'repo') {
    where.append(document.createTextNode(tx('desat al projecte, a .gregal/backlog/', 'saved in the project, in .gregal/backlog/')));
  } else {
    where.append(document.createTextNode(tx('desat a Gregal', 'saved in Gregal') + ' · '));
    where.append(btn(tx('Passa-ho a la carpeta del projecte', 'Move it into the project folder'), toRepo, 'bl-link'));
  }
  left.append(where);
  h.append(left, btn('＋ ' + tx('Tasca nova', 'New item'), () => { mode = 'new'; flash = null; render(); }, 'bl-btn primary'));
  return h;
}

async function toRepo() {
  const ok = confirm(tx(
    'Cada tasca passarà a ser un fitxer .gregal/backlog/<número>.md dins de la carpeta del projecte, i es podrà versionar amb el codi. Vols fer-ho?',
    'Each item becomes a file .gregal/backlog/<number>.md inside the project folder, so it can be versioned with the code. Do you want to do it?'));
  if (ok) await act({ action: 'to_repo' }, tx('Les tasques ja són a .gregal/backlog/.', 'The items are now in .gregal/backlog/.'));
}

function counts() {
  const c = { open: 0, done: 0, dropped: 0, all: 0 };
  for (const it of items()) {
    if (typeFilter && it.type !== typeFilter) continue;
    c.all++;
    if (it.status === 'done') c.done++; else if (it.status === 'dropped') c.dropped++; else c.open++;
  }
  return c;
}

function toolbar() {
  const bar = el('div', 'bl-toolbar');
  const seg = el('div', 'bl-seg'); seg.setAttribute('role', 'tablist'); seg.setAttribute('aria-label', tx('Filtre per estat', 'Filter by status'));
  const c = counts();
  for (const [k, ca, en] of FILTERS) {
    const b = btn('', () => { filter = k; render(); }, 'bl-seg-btn' + (filter === k ? ' on' : ''));
    b.setAttribute('role', 'tab'); b.setAttribute('aria-selected', String(filter === k));
    b.append(el('span', '', tx(ca, en)), el('span', 'bl-count', String(c[k])));
    seg.append(b);
  }
  const sel = el('select', 'bl-select'); sel.setAttribute('aria-label', tx('Tipus', 'Type'));
  sel.append(new Option(tx('Tots els tipus', 'All types'), ''));
  for (const t of Object.keys(TYPES)) sel.append(new Option(lab(TYPES, t), t));
  sel.value = typeFilter;
  sel.onchange = () => { typeFilter = sel.value; render(); };
  bar.append(seg, sel);
  return bar;
}

function visible() {
  return items().filter(it => {
    if (typeFilter && it.type !== typeFilter) return false;
    if (filter === 'open') return it.status !== 'done' && it.status !== 'dropped';
    if (filter === 'all') return true;
    return it.status === filter;
  });
}

function body() {
  const wrap = el('div', 'bl-body');
  wrap.append(list(), detail());
  return wrap;
}

function list() {
  const col = el('div', 'bl-list');
  const vis = visible();
  if (!items().length) {
    const e = el('div', 'bl-empty');
    e.append(el('p', '', tx('Encara no hi ha cap tasca en aquest projecte.', 'There are no items in this project yet.')),
      el('p', 'bl-muted', tx('Crea’n una aquí o demana-ho a l’agent: «posa al backlog que l’exportació falla amb fitxers buits».',
        'Create one here or ask the agent: “add to the backlog that export fails on empty files”.')));
    col.append(e);
    return col;
  }
  if (!vis.length) { col.append(el('p', 'bl-empty bl-muted', tx('Cap tasca amb aquest filtre.', 'No items match this filter.'))); return col; }
  if (filter === 'open') {
    for (const st of OPEN) {
      const group = vis.filter(it => it.status === st);
      if (!group.length) continue;
      const g = el('section', 'bl-group'); g.dataset.status = st;
      const gh = el('h3', 'bl-group-head');
      gh.append(el('span', 'bl-dot'), el('span', '', lab(STATUS, st)), el('span', 'bl-count', String(group.length)));
      g.append(gh);
      group.sort(byPriority).forEach(it => g.append(row(it)));
      col.append(g);
    }
  } else {
    vis.sort((a, b) => String(b.updated).localeCompare(String(a.updated))).forEach(it => col.append(row(it)));
  }
  return col;
}

const PRIO_ORDER = { now: 0, next: 1, later: 2 };
const byPriority = (a, b) => (PRIO_ORDER[a.priority] ?? 1) - (PRIO_ORDER[b.priority] ?? 1) || a.id - b.id;

function row(it) {
  const r = btn('', () => { selected = it.id; mode = 'view'; flash = null; render(); }, 'bl-row' + (it.id === selected ? ' on' : ''));
  r.dataset.status = it.status;
  r.setAttribute('aria-current', it.id === selected ? 'true' : 'false');
  const top = el('span', 'bl-row-top');
  top.append(el('span', 'bl-id', '#' + it.id), el('span', 'bl-type t-' + it.type, lab(TYPES, it.type)), el('span', 'bl-row-title', it.title));
  const meta = el('span', 'bl-row-meta');
  const bits = [lab(PRIO, it.priority)];
  if (it.size) bits.push(it.size);
  const cr = it.criteria || [];
  if (cr.length) bits.push(cr.filter(c => c.done).length + '/' + cr.length + ' ' + tx('criteris', 'criteria'));
  if (filter !== 'open') bits.push(lab(STATUS, it.status));
  meta.textContent = bits.join(' · ');
  r.append(top, meta);
  return r;
}

function detail() {
  const pane = el('div', 'bl-detail');
  if (mode === 'new') { pane.append(form(null)); return pane; }
  const it = byId(selected);
  if (!it) {
    const e = el('div', 'bl-guide');
    e.append(el('h2', '', tx('Com s’ordena', 'How it stays in order')));
    const ul = el('ul');
    [
      tx('Cada tasca és un sol resultat que es pot comprovar i revisar d’un cop.', 'Each item is one outcome that can be checked and reviewed in one go.'),
      tx('Només es treballa en tasques «A punt»: amb el perquè, criteris i mida S o M.', 'Only “Ready” items are worked on: with a why, criteria and size S or M.'),
      tx('Un criteri es marca amb la seva prova. «Feta» la decideix qui revisa.', 'A criterion is ticked with its evidence. “Done” is decided by the reviewer.'),
      tx('El que surt pel camí és una tasca nova, no més abast.', 'Whatever comes up on the way is a new item, not more scope.'),
    ].forEach(t => ul.append(el('li', '', t)));
    e.append(ul);
    pane.append(e);
    return pane;
  }
  if (mode === 'edit') { pane.append(form(it)); return pane; }

  const card = el('article', 'bl-card'); card.dataset.status = it.status;
  const top = el('div', 'bl-card-top');
  top.append(el('span', 'bl-id', '#' + it.id), el('span', 'bl-type t-' + it.type, lab(TYPES, it.type)), el('span', 'bl-status', lab(STATUS, it.status)));
  card.append(top, el('h2', 'bl-title', it.title));

  const meta = el('div', 'bl-meta');
  meta.append(el('span', '', tx('Prioritat: ', 'Priority: ') + lab(PRIO, it.priority)));
  if (it.size) meta.append(el('span', '', tx('Mida: ', 'Size: ') + it.size));
  if (it.parent) meta.append(linkTo(tx('Èpica', 'Epic'), it.parent));
  (it.depends_on || []).forEach(d => meta.append(linkTo(tx('Depèn de', 'Depends on'), d)));
  card.append(meta);
  if (it.reason && (it.status === 'blocked' || it.status === 'dropped')) card.append(el('p', 'bl-reason', it.reason));
  if (flash) { const f = el('p', flash.error ? 'bl-error' : 'bl-ok', flash.text); f.setAttribute('role', flash.error ? 'alert' : 'status'); card.append(f); }
  card.append(actions(it));

  if (it.type === 'epic') {
    const kids = items().filter(k => k.parent === it.id);
    const s = section(tx('Tasques de l’èpica', 'Items in this epic'));
    if (!kids.length) s.append(el('p', 'bl-muted', tx('Encara cap. Crea tasques i tria aquesta èpica.', 'None yet. Create items and pick this epic.')));
    kids.forEach(k => s.append(linkTo(lab(STATUS, k.status), k.id, true)));
    card.append(s);
  }
  if (it.why) card.append(textSection(it.type === 'bug' ? tx('Què passa', 'What happens') : tx('Perquè', 'Why'), it.why));
  const rv = reviewSection(it);
  if (rv) card.append(rv);
  card.append(criteria(it));
  if (it.out_of_scope) card.append(textSection(tx('Fora d’abast', 'Out of scope'), it.out_of_scope));
  card.append(logSection(it));
  pane.append(card);
  return pane;
}

function linkTo(prefix, id, full) {
  const t = byId(id);
  const b = btn(prefix + ' #' + id + (full && t ? ' · ' + t.title : ''), () => { selected = id; mode = 'view'; flash = null; render(); }, 'bl-chip-link');
  return b;
}

function section(title) { const s = el('section', 'bl-section'); s.append(el('h3', '', title)); return s; }
function textSection(title, text) { const s = section(title); s.append(el('p', 'bl-text', text)); return s; }

// reasonForm demana el motiu (bloquejar o descartar) sense diàlegs del navegador.
function reasonForm(host, label, onOk) {
  host.querySelector('.bl-inline')?.remove();
  const f = el('form', 'bl-inline');
  const inp = el('input'); inp.placeholder = label; inp.setAttribute('aria-label', label); inp.required = true;
  f.append(inp, el('button', 'bl-btn primary', tx('D’acord', 'OK')), btn(tx('Cancel·la', 'Cancel'), () => f.remove()));
  f.onsubmit = e => { e.preventDefault(); if (inp.value.trim()) onOk(inp.value.trim()); };
  host.append(f);
  inp.focus();
}

function actions(it) {
  const row = el('div', 'bl-actions');
  const move = (status, ok, reason) => act({ action: 'move', id: it.id, status, reason: reason || '' }, ok);
  const add = (label, fn, primary) => row.append(btn(label, fn, 'bl-btn' + (primary ? ' primary' : '')));
  const block = () => reasonForm(row, tx('Què la bloqueja i qui ho pot resoldre?', 'What blocks it and who can unblock it?'), r => move('blocked', '', r));
  const drop = () => reasonForm(row, tx('Per què es descarta?', 'Why is it dropped?'), r => move('dropped', '', r));
  const edit = () => { mode = 'edit'; flash = null; render(); };
  switch (it.status) {
    case 'proposed':
      add(tx('Marca-la a punt', 'Mark ready'), () => move('ready', tx('A punt: ja s’hi pot treballar.', 'Ready: it can be worked on now.')), true);
      add(tx('Edita', 'Edit'), edit); add(tx('Descarta', 'Drop'), drop); break;
    case 'ready':
      if (it.type !== 'epic') add('▶ ' + tx('Treballa-hi', 'Work on it'), () => work(it), true);
      add(tx('Edita', 'Edit'), edit); add(tx('Bloqueja', 'Block'), block); add(tx('Descarta', 'Drop'), drop); break;
    case 'in_progress':
      add(tx('Envia a revisió', 'Send to review'), () => move('in_review'), true);
      add('▶ ' + tx('Continua en una sessió nova', 'Continue in a new session'), () => work(it));
      add(tx('Edita', 'Edit'), edit); add(tx('Bloqueja', 'Block'), block); break;
    case 'in_review': {
      const running = reviewRunning(it);
      const done = btn(tx('Marca-la feta', 'Mark done'), () => move('done', tx('Feta.', 'Done.')), 'bl-btn primary');
      if (running) { done.disabled = true; done.title = tx('Espera el veredicte del revisor', 'Wait for the reviewer’s verdict'); }
      row.append(done);
      if (!running) add(it.review ? tx('Revisa de nou', 'Review again') : tx('Revisa-la', 'Review it'),
        () => act({ action: 'review', id: it.id }, tx('El revisor s’hi ha posat.', 'The reviewer is on it.')));
      add(tx('Torna-la a en curs', 'Back to in progress'), () => move('in_progress'));
      break;
    }
    case 'blocked':
      add(tx('Desbloqueja', 'Unblock'), () => move('ready'), true);
      add(tx('Edita', 'Edit'), edit); add(tx('Descarta', 'Drop'), drop); break;
    default:
      add(tx('Reobre', 'Reopen'), () => move('proposed'));
  }
  return row;
}

// work obre una sessió per a la tasca (en un worktree propi si el projecte
// és un repo de git) i hi engega l'agent amb la tasca com a encàrrec.
async function work(it) {
  let pre;
  try { pre = await call('POST', { action: 'work', id: it.id, dry: true }); }
  catch (e) { flash = { text: e.message, error: true }; render(); return; }
  flash = { text: tx('Obrint una sessió per a la tasca…', 'Opening a session for the item…') };
  render();
  const sessions = P() && P().sessions;
  try {
    let opened = false;
    try {
      const r = await G().api('/api/worktrees', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name: pre.branch }) });
      const j = r.ok ? await r.json() : null;
      if (j && j.session && j.session.id) { await sessions.switchTo(j.session.id); opened = true; }
    } catch (_) { /* no és un repo de git, o el worktree ja hi és: sessió normal */ }
    if (!opened) await sessions.open(data.root);
    const r = await call('POST', { action: 'work', id: it.id, session: window.gregalSession || '' });
    await G().runTask(r.task);
  } catch (e) {
    G().setView('backlog');
    flash = { text: tx('No s’ha pogut començar: ', 'Couldn’t start: ') + e.message, error: true };
    await load();
  }
}

// Una revisió en marxa es dona per perduda als 30 minuts (com al servidor).
const reviewRunning = it => !!(it.review && it.review.status === 'running' && Date.now() - new Date(it.review.at) < 30 * 60 * 1000);

// Mentre el revisor treballa (o just després de passar a revisió, abans que
// consti), la pantalla es refresca sola fins que arriba el veredicte.
let pollTimer = 0;
function pollReviews() {
  clearTimeout(pollTimer);
  const waiting = items().some(it => reviewRunning(it) ||
    (it.status === 'in_review' && !it.review && Date.now() - new Date(it.updated) < 20000));
  if (waiting) pollTimer = setTimeout(() => { if (G().viewActual() === 'backlog') load(); }, 3000);
}

const VERDICT = { MET: ['Complert', 'Met'], 'NOT MET': ['No complert', 'Not met'], UNVERIFIED: ['Sense comprovar', 'Unverified'] };

function reviewSection(it) {
  const r = it.review;
  if (!r && it.status !== 'in_review') return null;
  const s = section(tx('Revisió independent', 'Independent review'));
  const head = el('p', 'bl-review-head');
  if (!r) {
    head.textContent = tx('Encara no s’ha revisat. Un model que no ha escrit el codi la pot comprovar contra els criteris.',
      'Not reviewed yet. A model that did not write the code can check it against the criteria.');
    s.append(head);
    return s;
  }
  s.dataset.review = reviewRunning(it) ? 'running' : r.status;
  const label = {
    running: tx('Revisant: mira el diff, passa les comprovacions del projecte i jutja cada criteri…', 'Reviewing: reading the diff, running the project’s checks and judging each criterion…'),
    met: tx('Superada. Tots els criteris tenen prova; ja la pots marcar com a feta.', 'Passed. Every criterion has evidence; you can mark it done.'),
    not_met: tx('No superada: ', 'Not passed: ') + (r.summary || ''),
    error: tx('El revisor ha fallat: ', 'The reviewer failed: ') + (r.summary || '') + tx(' Decideixes tu, o torna-la a revisar.', ' You decide, or review it again.'),
  };
  head.textContent = reviewRunning(it) ? label.running : (r.status === 'running' ? tx('La revisió es va quedar a mitges. Torna-la a demanar.', 'The review did not finish. Ask for it again.') : label[r.status] || r.status);
  s.append(head);
  (r.checks || []).forEach(c => s.append(el('div', 'bl-review-check' + (c.passed ? ' ok' : ' ko'), (c.passed ? '✓ ' : '✗ ') + c.command)));
  if ((r.criteria || []).length) {
    const ul = el('ul', 'bl-review-list');
    r.criteria.forEach((c, n) => {
      const li = el('li', 'v-' + c.status.toLowerCase().replace(' ', '-'));
      li.append(el('span', 'bl-review-tag', lab(VERDICT, c.status)), el('span', '', (n + 1) + '. ' + c.text));
      if (c.evidence) li.append(el('div', 'bl-evidence', c.evidence));
      ul.append(li);
    });
    s.append(ul);
  }
  return s;
}

function criteria(it) {
  const s = section(tx('Criteris d’acceptació', 'Acceptance criteria'));
  const cr = it.criteria || [];
  if (!cr.length) {
    s.append(el('p', 'bl-muted', it.type === 'epic'
      ? tx('Una èpica es dona per feta quan ho són les seves tasques.', 'An epic is done when its items are.')
      : tx('Sense criteris no pot estar a punt. Edita la tasca i afegeix-ne.', 'Without criteria it cannot be ready. Edit the item and add some.')));
    return s;
  }
  const ul = el('ul', 'bl-criteria');
  cr.forEach((c, i) => {
    const li = el('li', c.done ? 'done' : '');
    const box = el('input'); box.type = 'checkbox'; box.checked = !!c.done; box.id = 'blc-' + it.id + '-' + i;
    const lb = el('label', '', c.text); lb.htmlFor = box.id;
    li.append(box, lb);
    if (c.evidence) li.append(el('div', 'bl-evidence', tx('Prova: ', 'Evidence: ') + c.evidence));
    box.onchange = () => {
      if (!box.checked) { act({ action: 'check', id: it.id, n: i + 1, done: false }); return; }
      box.checked = false;
      reasonForm(li, tx('Quina prova ho demostra? (test, comanda, fitxer, captura)', 'What proves it? (test, command, file, screenshot)'),
        ev => act({ action: 'check', id: it.id, n: i + 1, done: true, evidence: ev }));
    };
    ul.append(li);
  });
  s.append(ul);
  return s;
}

function logSection(it) {
  const s = section(tx('Registre', 'Log'));
  const ol = el('ol', 'bl-log');
  const fmt = iso => { const d = new Date(iso); return isNaN(d) ? '' : d.toLocaleString(idioma() === 'en' ? 'en-GB' : 'ca-ES', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }); };
  (it.log || []).slice().reverse().forEach(l => {
    const li = el('li');
    li.append(el('time', '', fmt(l.at)));
    li.append(el('span', 'bl-who', l.who || tx('tu', 'you')));
    li.append(el('span', 'bl-log-text', l.text));
    ol.append(li);
  });
  const f = el('form', 'bl-inline bl-log-add');
  const inp = el('input'); inp.placeholder = tx('Afegeix una nota al registre', 'Add a note to the log'); inp.setAttribute('aria-label', inp.placeholder);
  f.append(inp, el('button', 'bl-btn', tx('Afegeix', 'Add')));
  f.onsubmit = e => { e.preventDefault(); if (inp.value.trim()) act({ action: 'log', id: it.id, text: inp.value.trim() }); };
  s.append(f, ol);
  return s;
}

function field(label, input, help) {
  const l = el('label', 'bl-field');
  l.append(el('span', 'bl-label', label), input);
  if (help) l.append(help instanceof Node ? help : el('span', 'bl-help', help));
  return l;
}

function form(it) {
  const f = el('form', 'bl-form');
  f.append(el('h2', '', it ? tx('Edita la tasca #', 'Edit item #') + it.id : tx('Tasca nova', 'New item')));
  const type = el('select', 'bl-select');
  for (const t of Object.keys(TYPES)) type.append(new Option(lab(TYPES, t), t));
  type.value = it ? it.type : 'feature';
  const typeHelp = el('span', 'bl-help');
  const title = el('input'); title.required = true; title.value = it ? it.title : '';
  title.placeholder = tx('En imperatiu i concret: «Exporta l’informe mensual en CSV»', 'Imperative and specific: “Export the monthly report as CSV”');
  const why = el('textarea'); why.rows = 3; why.value = it ? it.why || '' : '';
  const crit = el('textarea'); crit.rows = 4; crit.value = it ? (it.criteria || []).map(c => c.text).join('\n') : '';
  crit.placeholder = tx('L’informe té un botó «Exporta CSV».\nUn mes buit dona un fitxer només amb la capçalera.', 'The report page has an “Export CSV” button.\nAn empty month gives a file with only the header row.');
  const out = el('textarea'); out.rows = 2; out.value = it ? it.out_of_scope || '' : '';
  const prio = el('select', 'bl-select');
  for (const p of Object.keys(PRIO)) prio.append(new Option(lab(PRIO, p), p));
  prio.value = it ? it.priority || 'next' : 'next';
  const size = el('select', 'bl-select');
  [['', '—'], ['S', tx('S · hores', 'S · hours')], ['M', tx('M · un o dos dies', 'M · a day or two')], ['L', tx('L · cal partir-la', 'L · must be split')]].forEach(([v, t]) => size.append(new Option(t, v)));
  size.value = it ? it.size || '' : '';
  const parent = el('select', 'bl-select');
  parent.append(new Option(tx('Cap', 'None'), '0'));
  items().filter(e => e.type === 'epic' && (!it || e.id !== it.id)).forEach(e => parent.append(new Option('#' + e.id + ' ' + e.title, String(e.id))));
  parent.value = String(it ? it.parent || 0 : 0);
  const deps = el('input'); deps.value = it ? (it.depends_on || []).join(', ') : ''; deps.placeholder = '3, 5';

  const whyField = field('', why);
  const syncType = () => {
    const t = TYPES[type.value];
    typeHelp.textContent = t ? tx(t[2], t[3]) : '';
    whyField.querySelector('.bl-label').textContent = type.value === 'bug' ? tx('Què passa', 'What happens') : tx('Perquè', 'Why');
    why.placeholder = type.value === 'bug'
      ? tx('Passos per reproduir-ho, què esperaves i què ha passat.', 'Steps to reproduce, what you expected and what happened.')
      : tx('Qui en surt guanyant i què costa avui.', 'Who benefits and what it costs today.');
  };
  type.onchange = syncType; syncType();

  const grid = el('div', 'bl-form-grid');
  grid.append(field(tx('Prioritat', 'Priority'), prio), field(tx('Mida', 'Size'), size), field(tx('Èpica', 'Epic'), parent), field(tx('Depèn de', 'Depends on'), deps));
  f.append(field(tx('Tipus', 'Type'), type, typeHelp), field(tx('Títol', 'Title'), title), whyField,
    field(tx('Criteris d’acceptació', 'Acceptance criteria'), crit, tx('Un per línia. Cadascun s’ha de poder comprovar amb un test, una comanda o una cosa visible.', 'One per line. Each must be checkable by a test, a command or something visible.')),
    field(tx('Fora d’abast', 'Out of scope'), out, tx('El que algú podria esperar i no hi entra.', 'What someone might expect and is not included.')), grid);
  const err = el('p', 'bl-error'); err.setAttribute('role', 'alert'); err.hidden = true;
  f.append(err);
  const foot = el('div', 'bl-form-foot');
  foot.append(btn(tx('Cancel·la', 'Cancel'), () => { mode = 'view'; flash = null; render(); }), el('button', 'bl-btn primary', it ? tx('Desa', 'Save') : tx('Crea-la', 'Create')));
  f.append(foot);
  f.onsubmit = e => {
    e.preventDefault();
    const body = {
      action: it ? 'edit' : 'create', id: it ? it.id : 0,
      type: type.value, title: title.value, why: why.value, out_of_scope: out.value,
      priority: prio.value, size: size.value, parent: Number(parent.value) || 0,
      criteria: crit.value.split('\n').map(s => s.trim()).filter(Boolean),
      depends_on: deps.value.split(/[\s,#]+/).map(Number).filter(n => n > 0),
    };
    act(body, it ? tx('Desada.', 'Saved.') : tx('Creada com a proposada. Quan el perquè i els criteris siguin bons, marca-la a punt.', 'Created as proposed. When the why and the criteria are right, mark it ready.'), err);
  };
  setTimeout(() => title.focus(), 0);
  return f;
}

// open porta a la tasca dins de la vista Backlog (des del xat o l'inspector).
async function open(id) {
  selected = Number(id) || 0; mode = 'view'; flash = null; filter = 'all';
  G().setView('backlog');
  await load();
}

// workById és «Treballa-hi» des de fora de la vista: la targeta del xat.
async function workById(id) {
  await load();
  const it = byId(Number(id));
  if (!it) { await open(id); return; }
  if (it.status === 'proposed') {
    // Des del xat, «Treballa-hi» sobre la tasca acabada de definir és la
    // confirmació de la persona: es marca a punt (si li falta res, ho diu).
    try { await call('POST', { action: 'move', id: it.id, status: 'ready' }); }
    catch (e) { selected = it.id; flash = { text: e.message, error: true }; G().setView('backlog'); render(); return; }
    await load();
  }
  await work(byId(it.id));
}

window.gregalBacklog = { load, open, work: workById };
