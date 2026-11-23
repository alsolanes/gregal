// Panell lateral dels documents vius del projecte (internal/gdocs). S'obre
// sol quan l'agent escriu un document (event doc_changed del torn) i el
// segueix en directe amb /api/docs/{id}/stream; la vista és doc-view.js.
import { idioma } from './i18n.js';
import { parseFrame } from './runs.js';

const $ = id => document.getElementById(id);
const tx = (ca, en) => (idioma() === 'en' ? en : ca);
const G = () => window.gregal;
const state = { id: '', visible: false };
let ctrl = null;

const pane = document.createElement('aside');
pane.id = 'docsPanel';
pane.hidden = true;
pane.innerHTML = '<header><select id="docsList"></select><a id="docsDownload" data-gregal-doc="1"></a>' +
  '<button id="docsClose" type="button">×</button></header><div id="docsBody" class="body"></div>';
$('workbench').append(pane);
const style = document.createElement('style');
style.textContent = '#docsPanel[hidden]{display:none}#docsPanel{flex:0 0 460px;width:460px;min-width:0;border-left:1px solid var(--line);display:flex;flex-direction:column;background:var(--panel);color:var(--ink);overflow:hidden}' +
  '#docsPanel header{display:flex;gap:6px;align-items:center;padding:8px 12px;border-bottom:1px solid var(--line)}' +
  '#docsList{flex:1;min-width:0;border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);padding:6px}' +
  '#docsDownload,#docsClose{border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);cursor:pointer;padding:6px 9px;font-size:12px;text-decoration:none;white-space:nowrap}' +
  '#docsBody{flex:1;overflow:auto;padding:16px 20px}' +
  '@media(max-width:1000px){#docsPanel{position:fixed;inset:92px 0 0 auto;width:min(100%,560px);z-index:30;box-shadow:-4px 0 16px #0003}}';
document.head.append(style);
$('docsClose').onclick = () => { ctrl?.abort(); state.visible = false; pane.hidden = true; };
$('docsList').onchange = e => open(e.target.value);

async function list() {
  const r = await G().api('/api/docs');
  if (!r.ok) return;
  const items = (await r.json()).items || [];
  $('docsList').replaceChildren(...items.map(d => new Option(d.title, d.id, false, d.id === state.id)));
}

async function follow(id, my) {
  const r = await G().api('/api/docs/' + encodeURIComponent(id) + '/stream', { signal: my.signal });
  if (!r.ok) throw new Error(await r.text());
  // El stream ja és subscrit: el document que es llegeix ara no perd cap canvi.
  const doc = await (await G().api('/api/docs/' + encodeURIComponent(id), { signal: my.signal })).json();
  $('docsDownload').href = '/api/docs/' + encodeURIComponent(id) + '/export?format=docx&name=' + encodeURIComponent(doc.title + '.docx');
  const v = window.gregalDocView.view($('docsBody'), doc, window.gregalMD.render);
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

function open(id) {
  ctrl?.abort();
  const my = ctrl = new AbortController();
  Object.assign(state, { id, visible: true });
  pane.hidden = false;
  $('docsList').setAttribute('aria-label', tx('Documents del projecte', 'Project documents'));
  $('docsDownload').textContent = tx('Descarrega en Word', 'Download as Word');
  $('docsClose').setAttribute('aria-label', tx('Tanca el document', 'Close the document'));
  list();
  // Si el stream es talla (xarxa, o el servidor l'ha tancat perquè anàvem
  // endarrere), es torna a llegir el document sencer.
  follow(id, my).then(
    () => { if (ctrl === my && !my.signal.aborted) setTimeout(() => { if (ctrl === my && state.visible) open(id); }, 2000); },
    e => { if (!my.signal.aborted) G().sys(String(e.message || e)); });
}

window.gregalDocs = {
  open,
  changed(d) {
    if (window.gregalDocView.onDocChanged(state, d) === 'open') open(d.id);
    else list();
    // Al final del banc de treball queda al davant dels altres panells.
    $('workbench').append(pane);
  },
};
