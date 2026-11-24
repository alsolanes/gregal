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
pane.innerHTML = '<header><select id="docsList"></select><a id="docsDownload"></a>' +
  '<button id="docsWindow" type="button"></button><button id="docsClose" type="button">×</button></header><div id="docsBody" class="body"></div>';
$('workbench').append(pane);
const style = document.createElement('style');
// Les icones del panell són màscares: es pinten amb currentColor o amb el
// color del tipus de document, en tots els temes.
const mask = d => 'url("data:image/svg+xml,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="#000" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="' + d + '"/></svg>') + '")';
const ICON = {
  doc: 'M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7zM14 2v5h6M16 13H8M16 17H8M10 9H8',
  deck: 'M2 3h20M21 3v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V3M7 21l5-5 5 5',
  sheet: 'M3 5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2zM3 9h18M3 15h18M9 3v18',
  download: 'M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3',
  window: 'M15 3h6v6M10 14 21 3M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6',
  close: 'M18 6 6 18M6 6l12 12',
  plus: 'M12 5v14M5 12h14',
};
const glyph = (d, size = 16) => 'content:"";flex:none;width:' + size + 'px;height:' + size + 'px;background:currentColor;-webkit-mask:' + mask(d) + ' center/contain no-repeat;mask:' + mask(d) + ' center/contain no-repeat';
// Botó del panell: secundari (vora) o primari (color d'accent, com .ws-btn.primari).
const BTN = 'display:inline-flex;align-items:center;justify-content:center;gap:6px;height:30px;box-sizing:border-box;padding:0 11px;border:1px solid var(--line-hi);border-radius:6px;background:var(--surface);color:var(--ink);cursor:pointer;font-weight:500;font-size:12.5px;line-height:1;font-family:inherit;text-decoration:none;white-space:nowrap;transition:background-color .15s,border-color .15s,color .15s';
const PRIMARY = 'background:var(--accent);border-color:var(--accent);color:var(--bg);font-weight:600';
style.textContent = '#docsPanel[hidden]{display:none}#docsPanel{flex:0 0 460px;width:460px;min-width:0;border-left:1px solid var(--line);display:flex;flex-direction:column;background:var(--panel);color:var(--ink);overflow:hidden}' +
  '#docsPanel :is(button,a,select,input,textarea):focus-visible{outline:2px solid var(--accent);outline-offset:1px}' +
  // Capçalera: el títol del document (la llista), la descàrrega i dues icones.
  '#docsPanel header{display:flex;gap:4px;align-items:center;min-height:48px;box-sizing:border-box;padding:8px 8px 8px 10px;border-bottom:1px solid var(--line)}' +
  '#docsList{flex:1;min-width:0;height:32px;padding:0 6px;border:1px solid transparent;border-radius:6px;background:transparent;color:var(--ink);font-weight:600;font-size:14px;line-height:1;font-family:inherit;text-overflow:ellipsis;cursor:pointer;transition:border-color .15s,background-color .15s}' +
  '#docsList option{background:var(--panel);color:var(--ink);font-weight:400}' +
  '#docsDownload{' + BTN + '}#docsDownload::before{' + glyph(ICON.download, 15) + '}' +
  '#docsWindow,#docsClose{display:inline-grid;place-items:center;width:30px;height:30px;padding:0;border:0;border-radius:6px;background:transparent;color:var(--muted);cursor:pointer;font-size:0;transition:background-color .15s,color .15s}' +
  '#docsWindow::before{' + glyph(ICON.window) + '}#docsClose::before{' + glyph(ICON.close, 18) + '}#docsClose{margin-left:auto}' +
  '#docsBody{flex:1;overflow:auto;overscroll-behavior:contain;padding:16px 20px}' +
  '@media(hover:hover){#docsList:hover{border-color:var(--line-hi);background:var(--hover)}#docsDownload:hover{background:var(--surface-hi)}#docsWindow:hover,#docsClose:hover{background:var(--hover-alt);color:var(--ink)}}' +
  // Editor de text (doc-view.js): la barra d'eines queda a dalt en fer scroll,
  // en grups d'icones separats per una línia, i l'estil en desplegables compactes.
  '#docsBody .doc-toolbar{position:sticky;top:0;z-index:2;display:flex;flex-wrap:wrap;align-items:center;gap:4px 0;margin:0 -16px 24px;padding:6px 10px;border-bottom:1px solid var(--line);background:var(--panel);box-shadow:0 8px 16px -14px var(--ombra)}' +
  // A la finestra pròpia, més ampla que la pàgina, la barra va centrada sobre la pàgina.
  '@container (min-width:720px){#docsBody .doc-toolbar{justify-content:center}}' +
  '#docsBody .doc-tools,#docsBody .doc-style{display:flex;flex-wrap:wrap;align-items:center;gap:1px;padding:0 5px;border-left:1px solid var(--line)}#docsBody .doc-tools:first-child{padding-left:0;border-left:0}' +
  '#docsBody .doc-toolbar button{display:inline-grid;place-items:center;width:28px;height:28px;padding:0;border:0;border-radius:6px;background:transparent;color:var(--muted);cursor:pointer;font-weight:600;font-size:12px;line-height:1;font-family:inherit;transition:background-color .15s,color .15s}' +
  '#docsBody .doc-icon{width:18px;height:18px}#docsBody .doc-toolbar button[aria-pressed="true"]{background:var(--accent-soft-hi);color:var(--ink)}#docsBody .doc-toolbar button:disabled{opacity:.35;cursor:default}' +
  '#docsBody .doc-pick{position:relative;display:inline-flex;align-items:center;height:28px;border-radius:6px;color:var(--muted);transition:background-color .15s,color .15s}' +
  '#docsBody .doc-pick select{appearance:none;-webkit-appearance:none;height:28px;max-width:11em;padding:0 20px 0 7px;border:0;border-radius:6px;background:transparent;color:var(--ink);font-weight:500;font-size:12px;line-height:1;font-family:inherit;text-overflow:ellipsis;cursor:pointer}' +
  '#docsBody .doc-pick select option{background:var(--panel);color:var(--ink)}' +
  '#docsBody .doc-chevron{position:absolute;right:4px;width:12px;height:12px;pointer-events:none}' +
  // Color, ressaltat i alineació: la icona, i el desplegable transparent al damunt.
  '#docsBody .doc-pick-icon{width:42px;padding-left:6px;box-sizing:border-box}#docsBody .doc-pick-icon select{position:absolute;inset:0;width:100%;max-width:none;padding:0;color:transparent}' +
  '#docsBody .doc-pick-color::after,#docsBody .doc-pick-bg::after{content:"";position:absolute;left:7px;bottom:3px;width:16px;height:3px;border-radius:1px;background:var(--swatch,currentColor);box-shadow:inset 0 0 0 1px var(--line-hi);pointer-events:none}' +
  '#docsBody .doc-pick-color .doc-icon,#docsBody .doc-pick-bg .doc-icon{width:16px;height:16px;margin-top:-4px}#docsBody .doc-pick-align .doc-icon{width:16px;height:16px}' +
  '#docsBody .doc-pick:focus-within{outline:2px solid var(--accent);outline-offset:-2px}#docsBody .doc-pick select:focus-visible{outline:none}' +
  '@media(hover:hover){#docsBody .doc-toolbar button:not(:disabled):hover,#docsBody .doc-pick:hover{background:var(--hover-alt);color:var(--ink)}}' +
  '#docsBody .ProseMirror{outline:none;min-height:50vh;white-space:pre-wrap}#docsBody .doc-empty::before{content:attr(data-placeholder);position:absolute;color:var(--muted);pointer-events:none}' +
  // Taules i imatges (081) a la pàgina: la primera fila és la capçalera, com la del Word exportat.
  '#docsBody .ProseMirror table{border-collapse:collapse;margin:8px 0}#docsBody .ProseMirror td{border:1px solid var(--line-hi);padding:4px 8px;min-width:48px;vertical-align:top}' +
  '#docsBody .ProseMirror tr:first-child td{font-weight:600;background:var(--surface)}#docsBody .doc-editor tr:first-child td{font-weight:700;background:var(--doc-primary);color:#fff}#docsBody .ProseMirror img{max-width:100%}' +
  // Inici de Gregal Docs (doc-home.js): crear a dalt, i els documents en llista amb la icona del tipus.
  '.docs-home{display:flex;flex-direction:column;gap:6px;max-width:720px;margin:0 auto}.docs-home h2{margin:2px 0 10px;font-size:20px;font-weight:650;letter-spacing:-.01em}' +
  '.docs-home h3{margin:18px 0 4px;font-size:12.5px;font-weight:600;color:var(--muted)}' +
  '.docs-kind-doc{--kind:var(--accent);--kind-icon:' + mask(ICON.doc) + '}.docs-kind-deck{--kind:var(--amber);--kind-icon:' + mask(ICON.deck) + '}.docs-kind-sheet{--kind:var(--green);--kind-icon:' + mask(ICON.sheet) + '}' +
  '.docs-home-new{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px}' +
  '.docs-home-new input{grid-column:1/-1;height:38px;box-sizing:border-box;padding:0 12px;border:1px solid var(--line-hi);border-radius:8px;background:var(--bg);color:var(--ink);font:inherit}' +
  '.docs-home-new button{position:relative;display:flex;flex-direction:column;align-items:flex-start;gap:10px;min-width:0;padding:12px;border:1px solid var(--line);border-radius:10px;background:var(--surface);color:var(--ink);cursor:pointer;font-weight:600;font-size:12.5px;line-height:1.3;font-family:inherit;text-align:left;transition:border-color .15s,background-color .15s}' +
  '.docs-home-new button::before{content:"";width:32px;height:32px;border-radius:8px;background:color-mix(in srgb,var(--kind) 16%,transparent)}' +
  '.docs-home-new button::after{content:"";position:absolute;left:20px;top:20px;width:18px;height:18px;background:var(--kind);-webkit-mask:var(--kind-icon) center/contain no-repeat;mask:var(--kind-icon) center/contain no-repeat}' +
  '.docs-home-item{display:grid;grid-template-columns:auto minmax(0,1fr);column-gap:12px;align-items:center;width:100%;padding:8px 10px;border:0;border-radius:8px;background:transparent;color:var(--ink);text-align:left;cursor:pointer;font:inherit;transition:background-color .15s}' +
  '.docs-kind{grid-row:1/3;display:grid;place-items:center;width:34px;height:34px;border-radius:8px;background:color-mix(in srgb,var(--kind) 16%,transparent);color:var(--kind)}' +
  '.docs-kind::before{content:"";width:18px;height:18px;background:currentColor;-webkit-mask:var(--kind-icon) center/contain no-repeat;mask:var(--kind-icon) center/contain no-repeat}' +
  '.docs-home-title{font-weight:550;overflow-wrap:anywhere}.docs-home-meta{font-size:12px;color:var(--muted)}' +
  '.docs-home-empty{margin:4px 0;padding:18px;border:1px dashed var(--line-hi);border-radius:10px;font-size:12.5px;color:var(--muted);text-align:center}' +
  '@media(hover:hover){.docs-home-item:hover{background:var(--hover-alt)}.docs-home-new button:hover{border-color:var(--kind);background:var(--surface-hi)}}' +
  // L'avís d'un conflicte amb l'agent (els tres editors).
  '#docsBody .doc-conflict{margin:8px 0;padding:10px 12px;border:1px solid var(--line);border-left:3px solid var(--amber);border-radius:8px;background:var(--amber-soft);color:var(--ink)}#docsBody .doc-conflict p:first-child{margin-top:0;font-weight:600}' +
  '#docsBody .doc-conflict button{' + BTN + ';margin:0 6px 0 0}' +
  // Un document de text és una pàgina A4 blanca en tots dos temes, com a
  // Word, amb els valors del docx que n'exporta (/api/docs/theme, en punts
  // sense unitat). --u és un punt, o menys quan el panell és més estret que
  // la pàgina: tota la pàgina s'encongeix igual.
  '#docsBody.doc-paper{container-type:inline-size;background:color-mix(in srgb,var(--ink) 10%,var(--panel));padding:0 16px 32px}' +
  '#docsBody .doc-editor{--u:min(1pt,100cqw / var(--doc-page-w));box-sizing:border-box;width:calc(var(--doc-page-w) * var(--u));min-height:calc(var(--doc-page-w) * 1.414 * var(--u));margin:0 auto;' +
  'padding:calc(var(--doc-margin-top) * var(--u)) calc(var(--doc-margin-side) * var(--u));border-radius:2px;background:#fff;color:var(--doc-text);box-shadow:0 0 0 1px rgba(0,0,0,.05),0 1px 3px rgba(0,0,0,.12),0 14px 32px -14px rgba(0,0,0,.4);' +
  'font-family:var(--doc-body-font),sans-serif;font-size:calc(var(--doc-body-size) * var(--u));line-height:1.15;overflow-wrap:break-word}' +
  // Els colors del xat (.body strong, li…) no hi entren: tot surt del tema.
  '#docsBody .doc-editor *{color:inherit}#docsBody .doc-editor li>ul,#docsBody .doc-editor li>ol{margin-bottom:0}' +
  '#docsBody .doc-editor ::selection{background:color-mix(in srgb,var(--doc-accent) 28%,transparent)}' +
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
  // (ooxml «default»): una diapositiva és blanca en tots dos temes, com a
  // PowerPoint, amb el seu número al marge com les miniatures.
  '#docsBody .deck-slides{counter-reset:slide}' +
  '#docsBody .deck-slide{position:relative;margin:0 0 16px;padding-left:24px;container-type:inline-size;cursor:pointer;counter-increment:slide}' +
  '#docsBody .deck-slide::before{content:counter(slide);position:absolute;left:0;top:1px;width:16px;text-align:right;font-size:11.5px;font-weight:600;font-variant-numeric:tabular-nums;color:var(--faint)}' +
  '#docsBody .slide{aspect-ratio:16/9;box-sizing:border-box;overflow:hidden;padding:6cqw 7cqw;border-radius:6px;box-shadow:0 0 0 1px rgba(0,0,0,.06),0 1px 3px rgba(0,0,0,.14),0 10px 24px -12px rgba(0,0,0,.4);background:#fff;color:#222;font:3cqw/1.35 Calibri,system-ui,sans-serif;transition:box-shadow .15s}' +
  '@media(hover:hover){#docsBody .deck-slide:hover .slide{box-shadow:0 0 0 2px var(--accent),0 1px 3px rgba(0,0,0,.14),0 10px 24px -12px rgba(0,0,0,.4)}#docsBody .deck-slide:hover::before{color:var(--ink)}}' +
  '#docsBody .slide h2{margin:0 0 3cqw;padding-bottom:1.5cqw;border-bottom:.5cqw solid #2E86AB;color:#1F2A44;font-size:5.5cqw;line-height:1.15}#docsBody .slide h3{margin:0 0 1cqw;font-size:3.4cqw;color:#1F4E79}' +
  '#docsBody .slide p,#docsBody .slide ul{margin:0 0 1.5cqw}#docsBody .slide ul{padding-left:1.2em}#docsBody .slide-cols{display:flex;gap:5cqw}#docsBody .slide-cols>div{flex:1;min-width:0}#docsBody .slide img{max-width:100%;max-height:30cqw}' +
  '#docsBody .slide-title,#docsBody .slide-closing,#docsBody .slide-section{display:flex;flex-direction:column;justify-content:center;background:#1F4E79;color:#EAF1F8}#docsBody .slide-section{background:#1F2A44}' +
  '#docsBody .slide-title h2,#docsBody .slide-closing h2,#docsBody .slide-section h2{border:0;color:#fff;font-size:7cqw}' +
  '#docsBody .slide-number{font-size:14cqw;font-weight:700;color:#2E86AB;line-height:1.1}#docsBody .slide-chart{color:#6B7280;font-style:italic}' +
  '#docsBody .deck-form{display:flex;flex-direction:column;gap:10px;padding:14px;border:1px solid var(--line-hi);border-radius:10px;background:var(--surface);cursor:auto}#docsBody .deck-form h3{margin:0;font-size:11.5px;font-weight:600;letter-spacing:.06em;text-transform:uppercase;color:var(--faint)}' +
  '#docsBody .deck-form label{display:flex;flex-direction:column;gap:4px;font-size:12px;font-weight:500;color:var(--muted)}#docsBody .deck-form input,#docsBody .deck-form select,#docsBody .deck-form textarea{box-sizing:border-box;width:100%;padding:7px 9px;border:1px solid var(--line-hi);border-radius:6px;background:var(--bg);color:var(--ink);font:13px/1.5 system-ui,sans-serif}' +
  '#docsBody .deck-form-buttons,#docsBody .deck-add{display:flex;gap:6px;flex-wrap:wrap}#docsBody .deck-empty{margin:0 0 16px;padding:28px 18px;border:1px dashed var(--line-hi);border-radius:10px;color:var(--muted);text-align:center}' +
  '#docsBody .deck-form button,#docsBody .deck-add button,#docsBody .deck-add select{' + BTN + '}' +
  '#docsBody .deck-form-buttons button:first-child,#docsBody .deck-add button{' + PRIMARY + '}#docsBody .deck-form-buttons button:last-child{margin-left:auto;color:var(--red)}' +
  '#docsBody .deck-add{align-items:center;justify-content:center;margin-left:24px;padding:12px;border:1px dashed var(--line-hi);border-radius:10px}#docsBody .deck-add button::before{' + glyph(ICON.plus, 14) + '}' +
  // Fulls de càlcul (sheet-view.js): les files fan 24px, com ROW_H.
  '#docsBody .sheet-tabs{display:flex;gap:2px;flex-wrap:wrap;align-items:flex-end;margin-bottom:10px;border-bottom:1px solid var(--line)}' +
  '#docsBody .sheet-tabs button{display:inline-flex;align-items:center;gap:5px;margin-bottom:-1px;padding:7px 12px;border:0;border-bottom:2px solid transparent;border-radius:6px 6px 0 0;background:transparent;color:var(--muted);cursor:pointer;font-weight:500;font-size:12.5px;line-height:1.2;font-family:inherit;transition:background-color .15s,color .15s}' +
  '#docsBody .sheet-tabs .on{border-bottom-color:var(--accent);color:var(--ink);font-weight:600}#docsBody .sheet-tabs .sheet-add::before{' + glyph(ICON.plus, 13) + '}' +
  '@media(hover:hover){#docsBody .sheet-tabs button:hover{background:var(--hover-alt);color:var(--ink)}}' +
  '#docsBody .sheet-bar{position:relative;display:flex;gap:6px;align-items:center;margin-bottom:10px}' +
  '#docsBody .sheet-ref{display:grid;place-items:center;flex:none;width:60px;height:30px;box-sizing:border-box;border:1px solid var(--line-hi);border-radius:6px;background:var(--surface);font:600 12px ui-monospace,monospace;color:var(--ink)}' +
  '#docsBody .sheet-ref::after{content:"fx";position:absolute;left:78px;color:var(--faint);font:italic 600 13px/1 Georgia,serif;pointer-events:none}' +
  '#docsBody .sheet-formula{flex:1;min-width:0;height:30px;box-sizing:border-box;padding:0 9px 0 30px;border:1px solid var(--line-hi);border-radius:6px;background:var(--bg);color:var(--ink);font:13px ui-monospace,monospace}' +
  '#docsBody .sheet-grid{overflow:auto;overscroll-behavior:contain;scroll-padding:24px 0 0 40px;max-height:calc(100vh - 200px);border:1px solid var(--line-hi);border-radius:8px;background:var(--surface);outline:none}#docsBody .sheet-grid:focus-visible{outline:2px solid var(--accent);outline-offset:1px}' +
  '#docsBody .sheet-grid table{border-collapse:collapse;font-size:13px;font-variant-numeric:tabular-nums}' +
  '#docsBody .sheet-grid th,#docsBody .sheet-grid td{height:24px;box-sizing:border-box;padding:0 6px;border:1px solid var(--line);white-space:nowrap}' +
  '#docsBody .sheet-grid td{min-width:72px;max-width:180px;overflow:hidden;text-overflow:ellipsis;cursor:cell;color:var(--ink)}#docsBody .sheet-grid td.num{text-align:right}' +
  '#docsBody .sheet-grid th{position:sticky;background:color-mix(in srgb,var(--ink) 5%,var(--surface));color:var(--muted);font-weight:500;font-size:11.5px}#docsBody .sheet-grid thead th{top:0;z-index:1;border-top:0}#docsBody .sheet-grid tbody th{left:0;min-width:40px;text-align:right;border-left:0}' +
  '#docsBody .sheet-grid thead th:first-child{left:0;z-index:2}#docsBody .sheet-grid tr:has(td.sel)>th{background:var(--accent-soft-hi);color:var(--ink)}' +
  '#docsBody .sheet-grid td.sel{outline:2px solid var(--accent,#2E86AB);outline-offset:-2px;background:var(--accent-soft)}' +
  '@media(max-width:1000px){#docsPanel{position:fixed;inset:92px 0 0 auto;width:min(100%,560px);z-index:30;box-shadow:-4px 0 16px #0003}}';
document.head.append(style);
$('docsClose').onclick = () => { ctrl?.abort(); state.visible = false; pane.hidden = true; };
$('docsList').onchange = e => open(e.target.value, state.scope);
// L'exportació vol la clau, que un enllaç no porta: es baixa amb G().api.
// attach.js ho fa amb els enllaços del xat, però no és a la finestra pròpia.
$('docsDownload').onclick = async e => {
  e.preventDefault();
  const href = $('docsDownload').href;
  try {
    const r = await G().api(href);
    if (!r.ok) throw new Error((await r.text()) || ('HTTP ' + r.status));
    const url = URL.createObjectURL(await r.blob());
    const a = document.createElement('a');
    a.href = url;
    a.download = new URL(href, location.href).searchParams.get('name');
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 30000);
  } catch (err) {
    G().sys(String(err.message || err));
  }
};
// Una finestra pròpia per al document (/app/doc/{id}), i el panell es tanca.
// A l'escriptori la crea el procés principal, una per document; al
// navegador, el nom de la finestra fa el mateix.
const withScope = (url, scope) => (scope ? url + (url.includes('?') ? '&' : '?') + 'scope=' + scope : url);

$('docsWindow').onclick = () => {
  const s = window.gregalSession || 'default';
  if (window.gregalDesktop?.openDocWindow) window.gregalDesktop.openDocWindow(state.id, s, state.scope);
  else window.open(withScope('/app/doc/' + encodeURIComponent(state.id) + '?session=' + encodeURIComponent(s), state.scope), 'gregal-doc-' + (state.scope ? state.scope + '-' : '') + state.id);
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
  // Tots dos es veuen només amb la icona: el títol en diu el nom.
  $('docsWindow').textContent = $('docsWindow').title = tx('Obre en una finestra', 'Open in a window');
  $('docsClose').setAttribute('aria-label', $('docsClose').title = tx('Tanca el document', 'Close the document'));
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
  $('docsClose').setAttribute('aria-label', $('docsClose').title = tx('Tanca Gregal Docs', 'Close Gregal Docs'));
  // Una llista que no es pot llegir és null: l'inici ho diu en aquella
  // llista i l'altra es veu igualment.
  const items = async scope => {
    const r = await G().api(withScope('/api/docs', scope), { signal: my.signal });
    if (!r.ok) return null;
    return (await r.json()).items || [];
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
// Un document del projecte és de la sessió d'abans: es tanca. Un dels meus
// documents no és de cap projecte i es queda; l'inici es torna a llegir.
document.addEventListener('gregal:session', () => {
  if (!state.visible) return;
  if (!state.id) home();
  else if (!state.scope) $('docsClose').click();
});

window.gregalDocs = {
  open,
  changed(d) {
    if (window.gregalDocView.onDocChanged(state, d) === 'open') open(d.id, d.scope || '');
    else list();
    // Al final del banc de treball queda al davant dels altres panells.
    $('workbench').append(pane);
  },
};
