import { idioma } from './i18n.js';
import './plots-core.js';

const core = window.gregalPlotsCore;
const words = {
  en: { open: 'Plots & dashboard', library: 'Library', dashboard: 'Dashboard', close: 'Close', save: 'Save plot', add: 'Add to dashboard', remove: 'Remove from dashboard', empty: 'No plots yet. Ask Gregal for a chart, or import plot JSON.', download: 'Download PNG', expand: 'Expand', edit: 'Edit data / title', delete: 'Delete plot', confirm: 'Delete this saved plot?', cancel: 'Cancel', apply: 'Save changes', import: 'Import plot JSON', wide: 'Wide', narrow: 'Compact', note: 'Note', earlier: 'Move earlier', later: 'Move later', saved: 'Saved', source: 'Source session', snapshot: 'Saved snapshots · no automatic refresh', reload: 'Reload', json: 'Download JSON' },
  ca: { open: 'Plots i dashboard', library: 'Biblioteca', dashboard: 'Dashboard', close: 'Tancar', save: 'Guardar plot', add: 'Afegir al dashboard', remove: 'Treure del dashboard', empty: 'Encara no hi ha plots. Demana un gràfic a Gregal o importa un plot en JSON.', download: 'Descarregar PNG', expand: 'Ampliar', edit: 'Editar dades / títol', delete: 'Eliminar plot', confirm: 'Vols eliminar aquest plot guardat?', cancel: 'Cancel·lar', apply: 'Guardar canvis', import: 'Importar plot JSON', wide: 'Ample', narrow: 'Compacte', note: 'Nota', earlier: 'Moure abans', later: 'Moure després', saved: 'Guardat', source: 'Sessió d’origen', snapshot: 'Captures guardades · sense actualització automàtica', reload: 'Recarregar', json: 'Descarregar JSON' }
};
const t = key => (words[idioma()] || words.en)[key];
Object.assign(words.en, { newBoard: 'New dashboard', renameBoard: 'Rename dashboard', deleteBoard: 'Delete dashboard', boardName: 'Dashboard name', search: 'Search plots', exportAll: 'Export collection JSON', moveBoard: 'Move to dashboard', deleteBoardConfirm: 'Delete dashboard? Plots stay in the library.' });
Object.assign(words.ca, { newBoard: 'Nou dashboard', renameBoard: 'Canviar nom del dashboard', deleteBoard: 'Eliminar dashboard', boardName: 'Nom del dashboard', search: 'Cercar plots', exportAll: 'Exportar col·lecció JSON', moveBoard: 'Moure al dashboard', deleteBoardConfirm: 'Eliminar dashboard? Els plots es conserven a la biblioteca.' });
const context = () => ({ session: window.gregalSession || 'default', cwd: window.gregalCwd || '' });
const same = ctx => ctx.session === context().session && ctx.cwd === context().cwd;
const charts = new Map();
let chartLibrary;
function ensureCharts() {
  if (window.Chart) return Promise.resolve();
  if (!chartLibrary) chartLibrary = new Promise((resolve, reject) => {
    const script = document.createElement('script'); script.src = '/app/vendor/chart.umd.min.js';
    script.onload = resolve; script.onerror = () => { script.remove(); chartLibrary = null; reject(Error('Cannot load Chart.js')); }; document.head.append(script);
  });
  return chartLibrary;
}
function el(tag, cls, text) { const node = document.createElement(tag); if (cls) node.className = cls; if (text != null) node.textContent = text; return node; }
function button(key, action) {
  const node = el('button', 'plot-button', t(key)); node.type = 'button';
  node.addEventListener('click', async () => { node.disabled = true; try { await action(); } catch (e) { report(node, e); } finally { node.disabled = false; } }); return node;
}
function report(node, error) {
  const parent = node.closest('.plot-card, dialog') || node.parentElement;
  let status = parent.querySelector('.plot-error'); if (!status) { status = el('p', 'plot-error'); status.setAttribute('role', 'alert'); parent.append(status); }
  status.textContent = error.message;
}
async function api(ctx, method = 'GET', body) {
  const r = await window.gregal.api('/api/plots', { method, headers: { 'X-Gregal-Session': ctx.session }, ...(body ? { body: JSON.stringify(body) } : {}) });
  if (!r.ok) throw Error((await r.text()).trim() || `HTTP ${r.status}`);
  return r.json();
}
function download(name, blob) {
  const url = URL.createObjectURL(blob); const a = el('a'); a.href = url; a.download = name; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
}
function modal(title) {
  const dialog = el('dialog', 'plots-dialog'); const head = el('div', 'plot-toolbar'); head.append(el('h2', '', title), button('close', () => dialog.close())); dialog.append(head);
  dialog.addEventListener('close', () => { dialog.remove(); prune(); }); document.body.append(dialog); dialog.showModal(); return dialog;
}
async function draw(canvas, spec) {
  await ensureCharts(); if (!canvas.isConnected) return;
  charts.set(canvas, new window.Chart(canvas, core.config(spec, getComputedStyle(canvas).color)));
}
function prune() { for (const [canvas, chart] of charts) if (!canvas.isConnected) { chart.destroy(); charts.delete(canvas); } }
async function save(spec, ctx, dashboard, board = 'default') {
  if (!same(ctx)) throw Error('Project or session changed; reopen the plot.');
  const lib = await api(ctx);
  if (!same(ctx)) throw Error('Project or session changed; reopen the plot.');
  return api(ctx, 'POST', { revision: lib.revision, spec, dashboard, board });
}
function editor(spec, apply) {
  const dialog = modal(t(spec ? 'edit' : 'import')); const text = el('textarea', 'plot-editor');
  text.setAttribute('aria-label', 'Plot JSON'); text.spellcheck = false;
  text.value = JSON.stringify(spec || { title: 'Example', type: 'line', labels: ['Jan', 'Feb'], series: [{ label: 'Example data', values: [10, 20] }] }, null, 2);
  dialog.append(text, button('apply', async () => { const value = core.validate(JSON.parse(text.value)); await apply(value); dialog.close(); }));
}
function card(spec, ctx, saved, mutate, boards = []) {
  const node = el('article', 'plot-card'); node.style.gridColumn = saved?.width === 2 ? 'span 2' : 'span 1';
  node.append(el('h3', '', spec.title)); const frame = el('div', 'plot-canvas'); const canvas = el('canvas'); canvas.setAttribute('role', 'img'); canvas.setAttribute('aria-label', spec.title); frame.append(canvas); node.append(frame);
  const details = el('details', 'plot-data'); details.append(el('summary', '', idioma() === 'ca' ? 'Veure dades' : 'View data'));
  details.addEventListener('toggle', () => {
    if (!details.open || details.querySelector('table')) return;
    const table = el('table'); const head = el('tr'); head.append(el('th', '', 'X')); spec.series.forEach(s => head.append(el('th', '', s.label))); table.append(head);
    spec.labels.forEach((x, i) => { const row = el('tr'); row.append(el('td', '', x)); spec.series.forEach(s => row.append(el('td', '', String(s.values[i])))); table.append(row); }); details.append(table);
  });
  node.append(details);
  const actions = el('div', 'plot-toolbar');
  actions.append(button('expand', () => { const dialog = modal(spec.title); dialog.append(card(spec, ctx)); }), button('download', async () => { if (!charts.has(canvas)) throw Error('Chart is still loading'); const blob = await new Promise(resolve => canvas.toBlob(resolve)); if (!blob) throw Error('Cannot export chart'); download('plot.png', blob); }), button('json', () => download('plot.json', new Blob([JSON.stringify(spec, null, 2)], {type: 'application/json'}))));
  if (saved) {
    const chooser = el('select', 'plot-board-select'); chooser.setAttribute('aria-label', t('moveBoard'));
    boards.forEach(board => { const option = el('option', '', board.name); option.value = board.id; chooser.append(option); }); chooser.value = saved.board || 'default';
    chooser.addEventListener('change', async () => { chooser.disabled = true; try { await mutate('PATCH', { ...saved, board: chooser.value, dashboard: true }); } catch (e) { chooser.value = saved.board || 'default'; report(chooser, e); } finally { chooser.disabled = false; } });
    if (boards.length) node.append(el('label', 'plot-source', t('moveBoard')), chooser);
    node.append(el('p', 'plot-source', `${t('source')}: ${saved.session} · ${saved.created}`));
    actions.append(button(saved.dashboard ? 'remove' : 'add', () => mutate('PATCH', { ...saved, dashboard: !saved.dashboard })), button(saved.width === 2 ? 'narrow' : 'wide', () => mutate('PATCH', { ...saved, width: saved.width === 2 ? 1 : 2 })), button('edit', () => editor(spec, value => mutate('PATCH', { ...saved, spec: value }))), button('earlier', () => mutate('PATCH', { ...saved, direction: -1 })), button('later', () => mutate('PATCH', { ...saved, direction: 1 })));
    const note = el('textarea', 'plot-note'); note.value = saved.note || ''; note.maxLength = 4000; note.placeholder = t('note'); note.setAttribute('aria-label', t('note'));
    node.append(note, button('apply', () => mutate('PATCH', { ...saved, note: note.value })));
    actions.append(button('delete', () => { const dialog = modal(t('confirm')); dialog.append(button('cancel', () => dialog.close()), button('delete', async () => { await mutate('DELETE', saved); dialog.close(); })); }));
  } else {
    actions.append(button('save', async () => { await save(spec, ctx, false); const status = el('p', 'plot-source', t('saved')); status.setAttribute('role', 'status'); node.append(status); }), button('add', async () => { await save(spec, ctx, true); const status = el('p', 'plot-source', t('saved')); status.setAttribute('role', 'status'); node.append(status); }));
  }
  node.append(actions); requestAnimationFrame(() => draw(canvas, spec).catch(e => report(canvas, e))); return node;
}

function openLibrary() {
  const ctx = context(); const dialog = modal(t('open')); dialog.append(el('p', 'plot-source', `${ctx.cwd} · ${t('snapshot')}`));
  const toolbar = el('div', 'plot-toolbar'); const grid = el('div', 'plot-grid'); let lib; let view = 'library'; let version = 0; let selectedBoard = 'default';
  const chooser = el('select', 'plot-board-select'); chooser.setAttribute('aria-label', t('dashboard'));
  const search = el('input', 'plot-search'); search.type = 'search'; search.placeholder = t('search'); search.setAttribute('aria-label', t('search'));
  chooser.addEventListener('change', () => { selectedBoard = chooser.value; view = 'dashboard'; if (lib) render(); });
  search.addEventListener('input', () => { if (lib) render(); });
  async function load() { const v = ++version; const value = await api(ctx); if (!dialog.isConnected || v !== version) return; lib = value; render(); }
  async function mutate(method, plot) {
    if (!same(ctx)) throw Error('Project or session changed; reopen the dashboard.');
    const body = { revision: lib.revision, id: plot.id };
    if (method === 'PATCH') Object.assign(body, { spec: plot.spec, dashboard: plot.dashboard, board: plot.board || 'default', width: plot.width, note: plot.note || '', direction: plot.direction || 0, dashboard_only: view === 'dashboard' });
    lib = await api(ctx, method, body); render();
  }
  async function boardOperation(operation, name) {
    if (!same(ctx)) throw Error('Project or session changed; reopen the dashboard.');
    lib = await api(ctx, 'POST', {revision: lib.revision, operation, board: selectedBoard, name});
    if (operation === 'create_board') { selectedBoard = lib.boards[lib.boards.length - 1].id; view = 'dashboard'; }
    if (operation === 'delete_board') selectedBoard = 'default';
    render();
  }
  function nameEditor(operation) {
    if (!lib) return;
    const edit = modal(t(operation === 'create_board' ? 'newBoard' : 'renameBoard')); const input = el('input', 'plot-search'); input.setAttribute('aria-label', t('boardName')); input.maxLength = 200;
    input.value = operation === 'rename_board' ? lib.boards.find(b => b.id === selectedBoard).name : '';
    edit.append(input, button('apply', async () => { await boardOperation(operation, input.value); edit.close(); })); input.focus();
  }
  function render() {
    if (!lib.boards.some(b => b.id === selectedBoard)) selectedBoard = 'default';
    chooser.replaceChildren(); lib.boards.forEach(board => { const option = el('option', '', board.name); option.value = board.id; chooser.append(option); }); chooser.value = selectedBoard;
    grid.replaceChildren(); prune(); toolbar.querySelectorAll('[data-plot-view]').forEach(b => b.setAttribute('aria-pressed', String(b.dataset.plotView === view)));
    const query = search.value.trim().toLocaleLowerCase();
    const plots = lib.plots.filter(p => (view === 'library' || (p.dashboard && p.board === selectedBoard)) && `${p.spec.title} ${p.note || ''} ${p.session}`.toLocaleLowerCase().includes(query));
    if (!plots.length) grid.append(el('p', '', t('empty')));
    for (const plot of plots) grid.append(card(core.validate(plot.spec), ctx, plot, mutate, lib.boards));
  }
  for (const key of ['library', 'dashboard']) { const b = button(key, () => { view = key; if (lib) render(); }); b.dataset.plotView = key; toolbar.append(b); }
  toolbar.append(chooser, button('newBoard', () => nameEditor('create_board')), button('renameBoard', () => nameEditor('rename_board')), button('deleteBoard', () => {
    if (!lib) return; const confirm = modal(t('deleteBoardConfirm')); confirm.append(button('cancel', () => confirm.close()), button('deleteBoard', async () => { await boardOperation('delete_board'); confirm.close(); }));
  }), search);
  toolbar.append(button('import', () => editor(null, async spec => { lib = await save(spec, ctx, view === 'dashboard', selectedBoard); render(); })), button('reload', load), button('exportAll', () => {
    if (lib) download('plot-collection.json', new Blob([JSON.stringify({format: 'gregal-plots-v1', ...lib}, null, 2)], {type: 'application/json'}));
  }));
  dialog.append(toolbar, grid); load().catch(e => report(dialog, e));
}
let scheduled = false;
function scan() {
  scheduled = false; prune();
  document.querySelectorAll('.lang-gregal-plot').forEach(block => {
    if (block.dataset.plotProcessed) return;
    const code = block.querySelector('code'); if (!code) return;
    let spec; try { spec = core.validate(JSON.parse(code.textContent)); } catch { return; }
    block.dataset.plotProcessed = 'true'; block.hidden = true;
    const rendered = card(spec, context()); block.after(rendered);
    rendered.dataset.plotInline = 'true';
  });
}
const observer = new MutationObserver(() => { if (!scheduled) { scheduled = true; requestAnimationFrame(scan); } });
observer.observe(document.getElementById('main') || document.body, {childList: true, subtree: true, characterData: true});
const open = button('open', openLibrary); open.id = 'plotsOpen'; open.className = 'nav-item';
document.querySelector('#side details')?.append(open);
document.addEventListener('gregal:idioma', () => { open.textContent = t('open'); });
scan();
