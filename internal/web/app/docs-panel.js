// Panell lateral dels documents vius del projecte (internal/gdocs). S'obre
// sol quan l'agent escriu un document (event doc_changed del torn) i el
// segueix en directe amb /api/docs/{id}/stream; la vista és doc-view.js.
// El botó «Gregal Docs» de la barra lateral hi obre l'inici (doc-home.js).
import { idioma } from './i18n.js';
import { parseFrame } from './runs.js';

const $ = id => document.getElementById(id);
const tx = (ca, en) => (idioma() === 'en' ? en : ca);
const G = () => window.gregal;
// scope 'mine': un document de «Els meus documents», no del projecte.
const state = { id: '', visible: false, scope: '' };
let ctrl = null;

const pane = document.createElement('aside');
pane.id = 'docsPanel';
pane.hidden = true;
pane.innerHTML = '<header><select id="docsList"></select><a id="docsDownload" data-gregal-doc="1"></a>' +
  '<button id="docsWindow" type="button"></button><button id="docsClose" type="button">×</button></header><div id="docsBody" class="body"></div>';
$('workbench').append(pane);
const style = document.createElement('style');
style.textContent = '#docsPanel[hidden]{display:none}#docsPanel{flex:0 0 460px;width:460px;min-width:0;border-left:1px solid var(--line);display:flex;flex-direction:column;background:var(--panel);color:var(--ink);overflow:hidden}' +
  '#docsPanel header{display:flex;gap:6px;align-items:center;padding:8px 12px;border-bottom:1px solid var(--line)}' +
  '#docsList{flex:1;min-width:0;border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);padding:6px}' +
  '#docsDownload,#docsWindow,#docsClose{border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);cursor:pointer;padding:6px 9px;font-size:12px;text-decoration:none;white-space:nowrap}' +
  '#docsClose{margin-left:auto}#docsBody{flex:1;overflow:auto;padding:16px 20px}' +
  // Editor de text (doc-view.js): la barra d'eines queda a dalt en fer scroll.
  '#docsBody .doc-toolbar{position:sticky;top:0;z-index:2;display:flex;flex-wrap:wrap;gap:2px;margin:0 -16px 16px;padding:6px 16px;border-bottom:1px solid var(--line);background:var(--panel)}' +
  '#docsBody .doc-toolbar button{min-width:28px;border:0;border-radius:4px;background:transparent;color:var(--ink);cursor:pointer;padding:4px 6px;font-size:13px}#docsBody .doc-toolbar button:hover{background:var(--surface)}' +
  '#docsBody .doc-style{display:flex;flex-wrap:wrap;gap:4px;padding-left:6px;border-left:1px solid var(--line)}' +
  '#docsBody .doc-style select{max-width:9em;border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);padding:3px 4px;font-size:12px}' +
  '#docsBody .ProseMirror{outline:none;min-height:50vh;white-space:pre-wrap}#docsBody .doc-empty::before{content:attr(data-placeholder);position:absolute;color:var(--muted);pointer-events:none}' +
  // Taules i imatges (081): la primera fila és la capçalera.
  '#docsBody .doc-toolbar-group{display:flex;gap:2px;padding-left:6px;margin-left:4px;border-left:1px solid var(--line)}#docsBody .doc-toolbar button:disabled{opacity:.4;cursor:default}' +
  '#docsBody .ProseMirror table{border-collapse:collapse;margin:8px 0}#docsBody .ProseMirror td{border:1px solid var(--line-hi);padding:4px 8px;min-width:48px;vertical-align:top}' +
  '#docsBody .ProseMirror tr:first-child td{font-weight:600;background:var(--surface)}#docsBody .ProseMirror img{max-width:100%}' +
  '.docs-home h2{margin:0 0 12px;font-size:18px}.docs-home h3{margin:20px 0 6px;font-size:13px;color:var(--muted)}.docs-home-new{display:flex;gap:6px}' +
  '.docs-home-new input{flex:1;min-width:0;border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);padding:6px}' +
  '.docs-home-new button{border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);cursor:pointer;padding:6px 9px}' +
  '.docs-home-item{display:flex;flex-direction:column;width:100%;padding:8px 10px;border:0;border-radius:6px;background:transparent;color:var(--ink);text-align:left;cursor:pointer;font:inherit}' +
  '.docs-home-item:hover{background:var(--surface)}.docs-home-meta,.docs-home-empty{font-size:12px;color:var(--muted)}' +
  '#docsBody .doc-conflict{margin:6px 0;padding:4px 10px;border-left:3px solid var(--line-hi);color:var(--ink)}#docsBody .doc-conflict p:first-child{font-weight:600}' +
  '#docsBody .doc-conflict button{margin:0 6px 6px 0;border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);cursor:pointer;padding:4px 9px;font-size:12px}' +
  // Un document de text és una pàgina A4 blanca en tots dos temes, com a
  // Word, amb els valors del docx que n'exporta (/api/docs/theme, en punts
  // sense unitat). --u és un punt, o menys quan el panell és més estret que
  // la pàgina: tota la pàgina s'encongeix igual.
  '#docsBody.doc-paper{container-type:inline-size;background:color-mix(in srgb,var(--ink) 10%,var(--panel));padding:0 16px 24px}' +
  '#docsBody .doc-editor{--u:min(1pt,100cqw / var(--doc-page-w));box-sizing:border-box;width:calc(var(--doc-page-w) * var(--u));min-height:calc(var(--doc-page-w) * 1.414 * var(--u));margin:0 auto;' +
  'padding:calc(var(--doc-margin-top) * var(--u)) calc(var(--doc-margin-side) * var(--u));background:#fff;color:var(--doc-text);box-shadow:0 1px 4px rgba(0,0,0,.25);' +
  'font-family:var(--doc-body-font),sans-serif;font-size:calc(var(--doc-body-size) * var(--u));line-height:1.15;overflow-wrap:break-word}' +
  // Els colors del xat (.body strong, li…) no hi entren: tot surt del tema.
  '#docsBody .doc-editor *{color:inherit}#docsBody .doc-editor li>ul,#docsBody .doc-editor li>ol{margin-bottom:0}' +
  '#docsBody .doc-editor h1{margin:1.2em 0 .4em;font-family:var(--doc-heading-font),sans-serif;font-size:calc(var(--doc-h1-size) * var(--u));color:var(--doc-h1-color);line-height:1.15}' +
  '#docsBody .doc-editor h2{margin:1.1em 0 .35em;font-family:var(--doc-heading-font),sans-serif;font-size:calc(var(--doc-h2-size) * var(--u));color:var(--doc-h2-color);line-height:1.15}' +
  '#docsBody .doc-editor h3{margin:1em 0 .3em;font-family:var(--doc-heading-font),sans-serif;font-size:calc(var(--doc-h3-size) * var(--u));color:var(--doc-h3-color);line-height:1.15}' +
  '#docsBody .doc-editor p{margin:0 0 calc(var(--doc-space-after) * var(--u))}' +
  '#docsBody .doc-editor ul,#docsBody .doc-editor ol{margin:0 0 calc(var(--doc-space-after) * var(--u));padding-left:calc(36 * var(--u))}' +
  '#docsBody .doc-editor blockquote{margin:calc(6 * var(--u)) calc(28 * var(--u)) calc(10 * var(--u));padding:0 0 0 calc(12 * var(--u));border-left:calc(2.25 * var(--u)) solid var(--doc-accent);color:var(--doc-muted);font-style:italic}' +
  '#docsBody .doc-editor .codeblock{margin:0 0 calc(var(--doc-space-after) * var(--u));border:0;border-radius:0;background:none}#docsBody .doc-editor .codehead{display:none}' +
  '#docsBody .doc-editor pre{margin:0;padding:calc(4 * var(--u)) calc(8.5 * var(--u));border:0;border-radius:0;background:var(--doc-light);color:var(--doc-text);font-family:var(--doc-code-font),monospace;font-size:calc(var(--doc-code-size) * var(--u));white-space:pre-wrap}' +
  '#docsBody .doc-editor code{padding:0;border:0;background:var(--doc-light);color:inherit;font-family:var(--doc-code-font),monospace;font-size:.9em}#docsBody .doc-editor pre code{font-size:inherit;background:none}' +
  '#docsBody .doc-editor a{color:var(--doc-accent);text-decoration:underline}#docsBody .doc-editor img{max-width:100%;height:auto}' +
  '#docsBody .doc-editor hr{border:0;border-top:calc(.75 * var(--u)) solid var(--doc-muted);margin:calc(12 * var(--u)) 0}' +
  '#docsBody .doc-editor .md-table-scroll{overflow-x:auto;margin:0 0 calc(var(--doc-space-after) * var(--u))}#docsBody .doc-editor table{border-collapse:collapse;width:100%;margin:0;font-size:inherit}' +
  '#docsBody .doc-editor td{padding:calc(2.85 * var(--u)) calc(5.4 * var(--u));border:calc(.5 * var(--u)) solid var(--doc-table-border);background:transparent;color:var(--doc-text);text-align:left}' +
  '#docsBody .doc-editor th{padding:calc(2.85 * var(--u)) calc(5.4 * var(--u));border:calc(.5 * var(--u)) solid var(--doc-table-border);background:var(--doc-primary);color:#fff;font-weight:700;text-align:left}' +
  '#docsBody .doc-editor tbody tr:nth-child(even) td{background:var(--doc-light)}' +
  '#docsBody .doc-editor .doc-empty{color:var(--doc-muted)}#docsBody .doc-editor .doc-conflict{color:inherit;border-left-color:var(--doc-accent)}' +
  // Diapositives (deck-view.js) amb els colors del tema per defecte del pptx
  // (ooxml «default»): una diapositiva és blanca en tots dos temes, com a PowerPoint.
  '#docsBody .deck-slide{margin:0 0 14px;container-type:inline-size;cursor:pointer}' +
  '#docsBody .slide{aspect-ratio:16/9;box-sizing:border-box;overflow:hidden;padding:6cqw 7cqw;border-radius:4px;box-shadow:0 1px 4px #0004;background:#fff;color:#222;font:3cqw/1.35 Calibri,system-ui,sans-serif}' +
  '#docsBody .slide h2{margin:0 0 3cqw;padding-bottom:1.5cqw;border-bottom:.5cqw solid #2E86AB;color:#1F2A44;font-size:5.5cqw;line-height:1.15}#docsBody .slide h3{margin:0 0 1cqw;font-size:3.4cqw;color:#1F4E79}' +
  '#docsBody .slide p,#docsBody .slide ul{margin:0 0 1.5cqw}#docsBody .slide ul{padding-left:1.2em}#docsBody .slide-cols{display:flex;gap:5cqw}#docsBody .slide-cols>div{flex:1;min-width:0}#docsBody .slide img{max-width:100%;max-height:30cqw}' +
  '#docsBody .slide-title,#docsBody .slide-closing,#docsBody .slide-section{display:flex;flex-direction:column;justify-content:center;background:#1F4E79;color:#EAF1F8}#docsBody .slide-section{background:#1F2A44}' +
  '#docsBody .slide-title h2,#docsBody .slide-closing h2,#docsBody .slide-section h2{border:0;color:#fff;font-size:7cqw}' +
  '#docsBody .slide-number{font-size:14cqw;font-weight:700;color:#2E86AB;line-height:1.1}#docsBody .slide-chart{color:#6B7280;font-style:italic}' +
  '#docsBody .deck-form{display:flex;flex-direction:column;gap:8px;padding:10px;border:1px solid var(--line-hi);border-radius:4px}#docsBody .deck-form h3{margin:0;font-size:13px;color:var(--muted)}' +
  '#docsBody .deck-form label{display:flex;flex-direction:column;gap:3px;font-size:12px;color:var(--muted)}#docsBody .deck-form input,#docsBody .deck-form textarea{box-sizing:border-box;width:100%;padding:6px 8px;border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);font:13px/1.5 system-ui,sans-serif}' +
  '#docsBody .deck-form-buttons,#docsBody .deck-add{display:flex;gap:6px;flex-wrap:wrap}#docsBody .deck-empty{color:var(--muted)}' +
  '#docsBody .deck-form button,#docsBody .deck-add button,#docsBody .deck-add select{border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);cursor:pointer;padding:6px 9px;font-size:12px}' +
  // Fulls de càlcul (sheet-view.js): les files fan 24px, com ROW_H.
  '#docsBody .sheet-tabs{display:flex;gap:4px;flex-wrap:wrap;margin-bottom:8px}#docsBody .sheet-tabs button{border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);cursor:pointer;padding:4px 9px;font-size:12px}' +
  '#docsBody .sheet-tabs .on{background:var(--surface);font-weight:600}#docsBody .sheet-tabs .sheet-add{color:var(--muted)}' +
  '#docsBody .sheet-bar{display:flex;gap:6px;align-items:center;margin-bottom:8px}#docsBody .sheet-ref{min-width:44px;font:12px ui-monospace,monospace;color:var(--muted)}' +
  '#docsBody .sheet-formula{flex:1;min-width:0;padding:5px 8px;border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);font:13px ui-monospace,monospace}' +
  '#docsBody .sheet-grid{overflow:auto;scroll-padding:24px 0 0 40px;max-height:calc(100vh - 190px);border:1px solid var(--line);outline:none}#docsBody .sheet-grid table{border-collapse:collapse;font-size:13px}' +
  '#docsBody .sheet-grid th,#docsBody .sheet-grid td{height:24px;box-sizing:border-box;padding:0 6px;border:1px solid var(--line);white-space:nowrap}' +
  '#docsBody .sheet-grid td{min-width:72px;max-width:180px;overflow:hidden;text-overflow:ellipsis;cursor:cell}#docsBody .sheet-grid td.num{text-align:right}' +
  '#docsBody .sheet-grid th{position:sticky;background:var(--surface);color:var(--muted);font-weight:500;font-size:12px}#docsBody .sheet-grid thead th{top:0;z-index:1}#docsBody .sheet-grid tbody th{left:0;min-width:40px;text-align:right}' +
  '#docsBody .sheet-grid td.sel{outline:2px solid var(--accent,#2E86AB);outline-offset:-2px}' +
  '@media(max-width:1000px){#docsPanel{position:fixed;inset:92px 0 0 auto;width:min(100%,560px);z-index:30;box-shadow:-4px 0 16px #0003}}';
document.head.append(style);
$('docsClose').onclick = () => { ctrl?.abort(); state.visible = false; pane.hidden = true; };
$('docsList').onchange = e => open(e.target.value, state.scope);
// Una finestra pròpia per al document (/app/doc/{id}), i el panell es tanca.
// A l'escriptori la crea el procés principal, una per document; al
// navegador, el nom de la finestra fa el mateix.
const withScope = (url, scope) => (scope ? url + (url.includes('?') ? '&' : '?') + 'scope=' + scope : url);

$('docsWindow').onclick = () => {
  const s = window.gregalSession || 'default';
  if (window.gregalDesktop?.openDocWindow) window.gregalDesktop.openDocWindow(state.id, s, state.scope);
  else window.open(withScope('/app/doc/' + encodeURIComponent(state.id) + '?session=' + encodeURIComponent(s), state.scope), 'gregal-doc-' + state.id);
  $('docsClose').click();
};

async function list() {
  const r = await G().api(withScope('/api/docs', state.scope));
  if (!r.ok) return;
  const items = (await r.json()).items || [];
  $('docsList').replaceChildren(...items.map(d => new Option(d.title, d.id, false, d.id === state.id)));
}

// blockEdits desa a /api/docs/{id}/blocks el que la persona edita a la vista:
// el markdown d'un bloc de text, una diapositiva sencera (un objecte) o, en
// un full de càlcul, el cos tal qual ({cells} o {name}).
function blockEdits(id, scope, kind) {
  const url = '/api/docs/' + encodeURIComponent(id) + '/blocks';
  const send = async (path, method, body) => {
    const r = await G().api(withScope(url + path, scope), { method, headers: { 'Content-Type': 'application/json' }, body: body && JSON.stringify(body) });
    return { status: r.status, body: await r.json().catch(() => ({ error: r.statusText })) };
  };
  const body = v => (typeof v === 'string' ? { md: v } : kind === 'sheet' ? v : { slide: v });
  const images = new Map(); // nom → la promesa de l'URL de la imatge
  return {
    put: (block, v, base) => send('/' + encodeURIComponent(block), 'PUT', { ...body(v), base_version: base }),
    post: (after, v) => send('', 'POST', { after, ...body(v) }),
    del: (block, base) => send('/' + encodeURIComponent(block) + '?base_version=' + base, 'DELETE'),
    notice: () => tx("L'agent ha canviat aquest bloc", 'The agent changed this block'),
    empty: () => tx('Escriu aquí…', 'Write here…'),
    error: m => G().sys(m),
    tx,
    upload: async file => {
      const r = await G().api(withScope('/api/docs/' + encodeURIComponent(id) + '/images', scope), { method: 'POST', headers: { 'Content-Type': file.type }, body: file });
      return { status: r.status, body: await r.json().catch(() => ({ error: r.statusText })) };
    },
    // Les imatges es demanen amb G().api, que porta la sessió i el token que un <img> no porta.
    image: name => {
      if (!images.has(name)) {
        images.set(name, G().api(withScope('/api/docs/' + encodeURIComponent(id) + '/images/' + encodeURIComponent(name), scope))
          .then(async r => { if (!r.ok) throw new Error(await r.text()); return URL.createObjectURL(await r.blob()); }));
      }
      return images.get(name);
    },
  };
}

// La descàrrega va en el format del document: una presentació en PowerPoint.
const DOWNLOADS = {
  doc: ['docx', () => tx('Descarrega en Word', 'Download as Word')],
  deck: ['pptx', () => tx('Descarrega en PowerPoint', 'Download as PowerPoint')],
  sheet: ['xlsx', () => tx('Descarrega en Excel', 'Download as Excel')],
};

// page posa al cos la pàgina d'un document de text amb el tema de
// l'exportació a Word, i a sobre el lloc de la barra d'eines, que queda
// fix en desplaçar la pàgina. Torna l'element on es pinta el document.
async function page(body, scope, my) {
  const r = await G().api(withScope('/api/docs/theme', scope), { signal: my.signal });
  if (!r.ok) throw new Error(await r.text());
  for (const [k, v] of Object.entries((await r.json()).vars)) body.style.setProperty(k, v);
  // L'editor (doc-view.js) hi posa la barra, fixa a dalt, i la pàgina .doc-editor.
  body.classList.toggle('doc-paper', true);
  return body;
}

// write: el document és nou i s'obre amb el cursor al primer bloc.
async function follow(id, scope, my, write) {
  const at = path => withScope('/api/docs/' + encodeURIComponent(id) + path, scope);
  const r = await G().api(at('/stream'), { signal: my.signal });
  if (!r.ok) throw new Error(await r.text());
  // El stream ja és subscrit: el document que es llegeix ara no perd cap canvi.
  const doc = await (await G().api(at(''), { signal: my.signal })).json();
  const [format, label] = DOWNLOADS[doc.kind] || DOWNLOADS.doc;
  $('docsDownload').href = at('/export?format=' + format + '&name=' + encodeURIComponent(doc.title + '.' + format));
  $('docsDownload').textContent = label();
  const view = { deck: window.gregalDeckView, sheet: window.gregalSheetView }[doc.kind] || window.gregalDocView;
  let box = $('docsBody');
  if (view === window.gregalDocView) box = await page(box, scope, my);
  const v = view.view(box, doc, window.gregalMD.render, blockEdits(id, scope, doc.kind));
  // Una presentació o un full de càlcul nous no tenen text on escriure.
  if (write && view === window.gregalDocView) v.focus();
  const rd = r.body.getReader(), dec = new TextDecoder();
  let buf = '';
  for (;;) {
    const x = await rd.read();
    if (x.done) return;
    buf += dec.decode(x.value, { stream: true });
    let at;
    while ((at = buf.indexOf('\n\n')) >= 0) {
      const f = parseFrame(buf.slice(0, at));
      buf = buf.slice(at + 2);
      if (f?.name === 'block') v.apply(f.data);
    }
  }
}

function show(home) {
  pane.hidden = false;
  for (const b of ['docsList', 'docsDownload', 'docsWindow']) $(b).hidden = home;
  $('docsBody').classList.toggle('doc-paper', false);
  $('workbench').append(pane);
}

function open(id, scope = '', write = false) {
  ctrl?.abort();
  const my = ctrl = new AbortController();
  Object.assign(state, { id, visible: true, scope });
  show(false);
  $('docsList').setAttribute('aria-label', scope ? tx('Els meus documents', 'My documents') : tx('Documents del projecte', 'Project documents'));
  $('docsDownload').textContent = '';
  $('docsWindow').textContent = tx('Obre en una finestra', 'Open in a window');
  $('docsClose').setAttribute('aria-label', tx('Tanca el document', 'Close the document'));
  list();
  // Si el stream es talla (xarxa, o el servidor l'ha tancat perquè anàvem
  // endarrere), es torna a llegir el document sencer.
  follow(id, scope, my, write).then(
    () => { if (ctrl === my && !my.signal.aborted) setTimeout(() => { if (ctrl === my && state.visible) open(id, scope); }, 2000); },
    e => { if (!my.signal.aborted) G().sys(String(e.message || e)); });
}

// home obre l'inici de Gregal Docs: «Els meus documents» i, a Codi, els
// del projecte obert.
async function home() {
  ctrl?.abort();
  const my = ctrl = new AbortController();
  Object.assign(state, { id: '', visible: true, scope: '' });
  show(true);
  $('docsClose').setAttribute('aria-label', tx('Tanca Gregal Docs', 'Close Gregal Docs'));
  const items = async scope => {
    const r = await G().api(withScope('/api/docs', scope), { signal: my.signal });
    const body = await r.json().catch(() => ({ error: r.statusText }));
    if (!r.ok) throw new Error(body.error || String(r.status));
    return body.items || [];
  };
  const st = G().state || {};
  const codi = window.gregalProduct === 'codi' && G().viewActual?.() !== 'vents' && st.cwd;
  try {
    const [mine, project] = await Promise.all([items('mine'), codi ? items('') : null]);
    if (ctrl !== my) return;
    window.gregalDocHome.home($('docsBody'), { mine, project: codi ? { name: st.project || st.cwd, items: project } : null }, {
      tx, lang: idioma(), open,
      create: (title, kind) => window.gregalDocHome.newDoc(title, { api: G().api, open, error: G().sys, kind }),
    });
  } catch (e) {
    if (!my.signal.aborted) G().sys(String(e.message || e));
  }
}
$('docsHomeBtn')?.addEventListener('click', home);

window.gregalDocs = {
  open,
  changed(d) {
    if (window.gregalDocView.onDocChanged(state, d) === 'open') open(d.id, d.scope || '');
    else list();
    // Al final del banc de treball queda al davant dels altres panells.
    $('workbench').append(pane);
  },
};
