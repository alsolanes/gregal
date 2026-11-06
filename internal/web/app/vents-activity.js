// Gregal Vents: el que els vents faran sols (/api/winds/automations) i el que
// han fet per a tu (/api/winds/saved). Una sola peça per a la pàgina
// d'Automatitzacions, la vista «Desat» de l'espai dels vents i les pestanyes
// del perfil de cada vent (mateixa API que l'app d'Android).

import { idioma } from './i18n.js';

const tx = (ca, en) => (idioma() === 'en' ? en : ca);
const api = (p, o) => (window.gregal?.api ? window.gregal.api(p, o) : window.api ? window.api(p, o) : fetch(p, o));
const getJSON = async path => { const r = await api(path); if (!r.ok) throw new Error((await r.text()).trim() || r.status); return r.json(); };
const post = async (path, body) => {
  const r = await api(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  if (!r.ok) throw new Error((await r.text()).trim() || r.status);
  return r.json().catch(() => ({}));
};
const el = (tag, cls, text) => { const e = document.createElement(tag); if (cls) e.className = cls; if (text != null) e.textContent = text; return e; };
const btn = (label, fn, cls = 'auto-btn') => { const b = el('button', cls, label); b.type = 'button'; b.onclick = fn; return b; };

/** «avui 18:00», «demà 09:00», «dj. 21/11 09:00». */
export function whenText(iso, now = new Date()) {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d)) return '';
  const hm = d.toLocaleTimeString(idioma() === 'en' ? 'en-GB' : 'ca-ES', { hour: '2-digit', minute: '2-digit' });
  const day = x => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diff = Math.round((day(d) - day(now)) / 86400000);
  if (diff === 0) return tx('avui ', 'today ') + hm;
  if (diff === 1) return tx('demà ', 'tomorrow ') + hm;
  return d.toLocaleDateString(idioma() === 'en' ? 'en-GB' : 'ca-ES', { weekday: 'short', day: '2-digit', month: '2-digit' }) + ' ' + hm;
}

const KIND = {
  reminder: ['Recordatori', 'Reminder'], task: ['Encàrrec', 'Errand'], followup: ['Seguiment', 'Follow-up'],
  watch: ['Vigila', 'Watch'], routine: ['Rutina', 'Routine'],
};
const kindLabel = k => tx(...(KIND[k] || [k, k]));

function repeatText(a) {
  if (a.repeat === 'daily') return tx('cada dia ', 'daily ') + (a.time || '');
  if (a.repeat === 'interval') return tx('cada ', 'every ') + a.interval_h + ' h';
  if (a.repeat === 'watch') return tx('quan hi hagi novetats', "when there's news");
  return '';
}

/** Filtres de la llista: el mateix tall que a Android. */
export function filterItems(items, filter, now = new Date()) {
  const endOfDay = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 23, 59, 59).getTime();
  switch (filter) {
    case 'today': return items.filter(a => a.enabled && a.next_at && new Date(a.next_at).getTime() <= endOfDay);
    case 'followup': return items.filter(a => a.kind === 'followup');
    case 'reminder': return items.filter(a => a.kind === 'reminder' || a.kind === 'task');
    case 'watch': return items.filter(a => a.kind === 'watch');
    case 'routine': return items.filter(a => a.kind === 'routine');
    case 'paused': return items.filter(a => !a.enabled);
    default: return items;
  }
}

function chips(options, selected, onPick) {
  const row = el('div', 'act-chips');
  for (const [key, label] of options) {
    const c = btn(label, () => onPick(key), 'act-chip' + (key === selected ? ' on' : ''));
    c.setAttribute('aria-pressed', String(key === selected));
    row.append(c);
  }
  return row;
}

/** Llista d'automatitzacions amb accions. opts.wind = només d'un vent. */
export async function renderAutomations(box, opts = {}) {
  const wind = opts.wind || '';
  box.replaceChildren(el('p', 'auto-help', tx('Carregant…', 'Loading…')));
  let data;
  try { data = await getJSON('/api/winds/automations' + (wind ? '?wind=' + encodeURIComponent(wind) : '')); }
  catch (e) { box.replaceChildren(el('p', 'auto-help', '⚠ ' + (e.message || e))); return; }
  const items = data.items || [], counts = data.counts || {};
  const status = el('p', 'auto-status'); status.setAttribute('role', 'status');
  let filter = box.dataset.filter || 'all';
  const act = async (a, action, okMsg) => {
    status.textContent = tx('Un moment…', 'One moment…');
    try {
      const path = { run: '/api/winds/rutines/run', pause: '/api/winds/rutines/toggle', resume: '/api/winds/rutines/toggle', delete: '/api/winds/rutines/delete' }[action];
      const body = action === 'pause' ? { id: a.id, enabled: false } : action === 'resume' ? { id: a.id, enabled: true } : { id: a.id };
      await post(path, body);
      await renderAutomations(box, opts);
      const s = box.querySelector('.auto-status'); if (s) s.textContent = okMsg;
    } catch (e) { status.textContent = '⚠ ' + (e.message || e); }
  };
  const draw = () => {
    box.dataset.filter = filter;
    box.replaceChildren(status);
    if (!items.length) {
      box.append(el('p', 'auto-empty', wind
        ? tx('Aquest vent encara no té res programat. Demana-li un recordatori o una rutina al xat.', 'This wind has nothing scheduled yet. Ask it for a reminder or a routine in the chat.')
        : tx('Encara no hi ha res programat. Demana-ho parlant: «avisa’m demà a les 9», «cada matí mira si hi ha concerts», o encén la vigilància d’un vent.',
          'Nothing scheduled yet. Just ask: “remind me tomorrow at 9”, “every morning check for concerts”, or turn on a wind’s watch.')));
      return;
    }
    if (!wind) {
      const sum = el('div', 'act-summary');
      for (const [n, a, b] of [[counts.today, 'avui', 'today'], [counts.week, 'aquesta setmana', 'this week'], [counts.paused, 'en pausa', 'paused']]) {
        const c = el('div', 'act-count'); c.append(el('strong', '', String(n || 0)), el('span', '', tx(a, b))); sum.append(c);
      }
      box.append(sum);
    }
    const kinds = new Set(items.map(a => a.kind));
    const options = [['all', tx('Tot', 'All')]];
    if (counts.today) options.push(['today', tx('Avui', 'Today')]);
    if (kinds.has('followup')) options.push(['followup', tx('Seguiments', 'Follow-ups')]);
    if (kinds.has('reminder') || kinds.has('task')) options.push(['reminder', tx('Recordatoris', 'Reminders')]);
    if (kinds.has('watch')) options.push(['watch', tx('Vigilàncies', 'Watches')]);
    if (kinds.has('routine')) options.push(['routine', tx('Rutines', 'Routines')]);
    if (counts.paused) options.push(['paused', tx('En pausa', 'Paused')]);
    if (options.length > 2) box.append(chips(options, filter, k => { filter = k; draw(); }));
    const shown = filterItems(items, filter);
    if (!shown.length) box.append(el('p', 'auto-empty', tx('Res en aquest filtre.', 'Nothing in this filter.')));
    for (const a of shown) box.append(row(a));
  };
  const row = a => {
    const r = el('div', 'auto-row act-row' + (a.enabled ? '' : ' off'));
    if (!wind && window.gregalWinds?.face) {
      const f = el('span', 'act-face'); try { f.append(window.gregalWinds.face({ name: a.wind_name, color: a.wind_color || '#FF9F00', style: a.wind_style || 'pso' }, false)); } catch (_) { /* sense cara */ }
      r.append(f);
    }
    const main = el('div', 'auto-main');
    const meta = [kindLabel(a.kind), wind ? '' : a.wind_name, a.enabled ? whenText(a.next_at) : tx('en pausa', 'paused'), repeatText(a)].filter(Boolean).join(' · ');
    main.append(el('strong', '', a.title), el('span', 'auto-sub', meta + (a.last_status === 'error' ? tx(' · ⚠ va fallar', ' · ⚠ failed') : '')));
    if (a.detail && a.detail !== a.title) { const d = el('details', 'act-detail'); d.append(el('summary', '', tx('Què farà', 'What it will do')), el('p', '', a.detail)); main.append(d); }
    const actions = el('div', 'auto-actions');
    if (a.enabled) actions.append(btn(tx('Fes-ho ara', 'Run now'), () => act(a, 'run', tx('En marxa: el resultat sortirà al xat del vent.', 'Running: the result will show up in the wind’s chat.'))));
    if (a.can_pause) actions.append(btn(a.enabled ? tx('Pausa', 'Pause') : tx('Reprèn', 'Resume'), () => act(a, a.enabled ? 'pause' : 'resume', a.enabled ? tx('En pausa.', 'Paused.') : tx('Represa.', 'Resumed.')), 'auto-btn subtle'));
    const hist = el('div', 'auto-history'); hist.hidden = true;
    if (a.kind === 'routine' || a.kind === 'watch') actions.append(btn(tx('Historial', 'History'), async () => {
      hist.hidden = !hist.hidden;
      if (hist.hidden || hist.dataset.loaded) return;
      hist.replaceChildren(el('p', 'auto-help', tx('Carregant…', 'Loading…')));
      try {
        const runs = (await getJSON('/api/winds/rutines/runs?id=' + encodeURIComponent(a.id))).runs || [];
        hist.replaceChildren();
        if (!runs.length) hist.append(el('p', 'auto-empty', tx('Encara no ha corregut mai.', 'It has not run yet.')));
        for (const run of runs) {
          const item = el('details', 'auto-run');
          item.append(el('summary', '', whenText(run.started_at) + ' · ' + (run.status === 'ok' ? (run.output ? '✓' : tx('✓ en silenci', '✓ quiet')) : '⚠ ' + (run.error || run.status))),
            el('pre', '', (run.output || run.error || '').slice(0, 4000)));
          hist.append(item);
        }
        hist.dataset.loaded = '1';
      } catch (e) { hist.replaceChildren(el('p', 'auto-help', '⚠ ' + e.message)); }
    }, 'auto-btn subtle'));
    actions.append(btn(a.kind === 'followup' ? tx('Desfés', 'Undo') : tx('Treu', 'Remove'), () => {
      if (confirm(tx('Treure aquesta automatització?', 'Remove this automation?') + '\n\n' + a.title)) act(a, 'delete', tx('Fet, l’he tret.', 'Done, removed.'));
    }, 'auto-btn subtle'));
    r.append(main, actions, hist);
    return r;
  };
  draw();
}

const SAVED = {
  doc: ['📄', 'Documents', 'Documents'], web: ['🌐', 'Webs', 'Websites'], chart: ['📊', 'Gràfics', 'Charts'],
  image: ['🖼️', 'Imatges', 'Images'], card: ['🗂️', 'Fitxes', 'Cards'], link: ['🔗', 'Fonts', 'Sources'],
};

function dayText(ts) {
  if (!ts) return '';
  const d = new Date(ts * 1000), now = new Date();
  const day = x => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diff = Math.round((day(now) - day(d)) / 86400000);
  if (diff === 0) return tx('avui', 'today');
  if (diff === 1) return tx('ahir', 'yesterday');
  return d.toLocaleDateString(idioma() === 'en' ? 'en-GB' : 'ca-ES');
}

/** Galeria del que han fet els vents. opts.wind = només d'un vent; opts.onCard(item) obre una fitxa. */
export async function renderSaved(box, opts = {}) {
  const wind = opts.wind || '';
  box.replaceChildren(el('p', 'auto-help', tx('Carregant…', 'Loading…')));
  let items;
  try { items = (await getJSON('/api/winds/saved' + (wind ? '?wind=' + encodeURIComponent(wind) : ''))).items || []; }
  catch (e) { box.replaceChildren(el('p', 'auto-help', '⚠ ' + (e.message || e))); return; }
  // Si encara no han fet res però han citat fonts, s'obre per les fonts: millor que una llista buida.
  let kind = items.some(i => i.kind !== 'link') ? '' : 'link', who = '';
  const draw = () => {
    box.replaceChildren();
    if (!items.length) {
      box.append(el('p', 'auto-empty', wind ? tx('Aquest vent encara no t’ha fet cap document, web ni gràfic.', "This wind hasn't made you any document, website or chart yet.")
        : tx('Encara no hi ha res desat. Quan un vent et faci un document, una web o un gràfic, sortirà aquí.', 'Nothing saved yet. When a wind makes you a document, a website or a chart, it shows up here.')));
      return;
    }
    const kinds = new Set(items.map(i => i.kind));
    const options = [['', tx('Feina feta', 'Made')], ...Object.keys(SAVED).filter(k => kinds.has(k)).map(k => [k, SAVED[k][0] + ' ' + tx(SAVED[k][1], SAVED[k][2])])];
    box.append(chips(options, kind, k => { kind = k; draw(); }));
    if (!wind) {
      const winds = [...new Map(items.map(i => [i.wind, i.wind_name])).entries()];
      if (winds.length > 1) box.append(chips([['', tx('Tots', 'All')], ...winds], who, k => { who = k; draw(); }));
    }
    const shown = items.filter(i => (kind ? i.kind === kind : i.kind !== 'link') && (!who || i.wind === who)).slice(0, 150);
    if (!shown.length) box.append(el('p', 'auto-empty', tx('Res en aquest filtre.', 'Nothing in this filter.')));
    const grid = el('div', 'saved-grid');
    for (const it of shown) grid.append(card(it));
    box.append(grid);
  };
  const card = it => {
    const c = el('article', 'saved-card saved-' + it.kind);
    const head = el('div', 'saved-head');
    head.append(el('span', 'saved-icon', (SAVED[it.kind] || ['•'])[0]));
    const title = it.url && it.kind !== 'image' && it.kind !== 'chart' ? el('a', 'saved-title', it.title || it.url) : el('strong', 'saved-title', it.title || tx(...(SAVED[it.kind] || ['', '']).slice(1)));
    if (title.tagName === 'A') {
      title.href = it.url; title.target = '_blank'; title.rel = 'noopener';
      if (it.kind === 'doc') title.dataset.gregalDoc = '1';
    }
    head.append(title);
    c.append(head);
    if (it.kind === 'chart') {
      // plots.js converteix sol qualsevol bloc gregal-plot del document en gràfic.
      const pre = el('pre', 'codeblock lang-gregal-plot'); pre.append(el('code', '', it.spec)); c.append(pre);
    } else if (it.kind === 'image') {
      const img = el('img', 'saved-img'); img.src = it.url; img.alt = it.title || ''; img.loading = 'lazy'; img.referrerPolicy = 'no-referrer';
      const a = el('a'); a.href = it.url; a.target = '_blank'; a.rel = 'noopener'; a.append(img); c.append(a);
    } else if (it.kind === 'card' && opts.onCard) {
      c.append(btn(tx('Obre la fitxa', 'Open card'), () => opts.onCard(it), 'auto-btn subtle'));
    }
    c.append(el('p', 'saved-meta', [wind ? '' : it.wind_name, it.where, dayText(it.ts)].filter(Boolean).join(' · ')));
    return c;
  };
  draw();
}

window.gregalVentsActivity = { renderAutomations, renderSaved, whenText, filterItems };
