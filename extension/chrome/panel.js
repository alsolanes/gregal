// Panell lateral de Gregal: parla amb un vent del teu Gregal (el de
// l'escriptori o el teu servidor) amb la pàgina com a context, i posa la
// resposta a la pàgina o al porta-retalls. Fa servir la mateixa API que
// l'app del mòbil: /api/winds i /api/winds/chat/stream amb la clau Bearer.
import { baseUrl, connection, parseSSE, splitBlocks, buildMessage, isPdfUrl, bytesToBase64, renderMarkdown } from './logic.js';
import { pageInfo, scriptFiles, insertText } from './page.js';

const lang = (chrome.i18n.getUILanguage() || 'ca').startsWith('ca') ? 'ca' : 'en';
const T = {
  ca: {
    connect: 'Connecta amb Gregal', connectHelp: 'L\'adreça, el port i la clau que diu el servidor en engegar-se.',
    modeDesktop: '🖥️ App d\'escriptori', modeServer: '🌐 Servidor',
    desktopHelp: 'Es connecta sol a l\'app de Gregal oberta en aquest ordinador. No cal posar res.',
    noDesktop: 'No trobo l\'app d\'escriptori. Obre Gregal i torna-ho a provar.',
    address: 'Adreça', port: 'Port', key: 'Clau', save: 'Connecta', saved: 'Connectat ✓', failed: 'No respon: %s',
    settings: 'Connexió', placeholder: 'Què vols fer amb aquesta pàgina?',
    noWinds: 'Encara no tens cap vent: crea\'n un a l\'app.', hello: 'Hola!', helloSub: 'Què fem amb aquesta pàgina?', send: 'Envia',
    copy: 'Copia', insert: 'Insereix', replace: 'Substitueix el fitxer', copied: 'Copiat al porta-retalls',
    inserted: 'Fet ✓', pasteHint: 'No s\'ha pogut posar directament: és al porta-retalls, prem Ctrl+V on vulguis.',
    error: 'Error: %s', needKey: 'Configura la connexió amb Gregal.',
    controlOn: 'Deixa que Gregal faci servir aquest navegador',
    controlLive: 'Navegador · %s', controlWaiting: 'Esperant Gregal…',
    controlDenied: 'Aquesta extensió és d\'una versió anterior: recarrega-la a chrome://extensions (o torna-la a preparar des de Gregal) i torna-ho a provar.',
    controlHelp: 'Gregal treballarà en pestanyes del grup «Gregal», amb les teves sessions. Et preguntarà abans d\'actuar a cada web i abans d\'enviar, pagar o esborrar res.',
    q: {
      docs: ['Resumeix el document', 'Millora la redacció del que he seleccionat', 'Revisa l\'ortografia i el to', 'Escriu-me un paràgraf sobre…'],
      sheets: ['Explica\'m aquestes dades', 'Fes-me una fórmula per…', 'Troba errors o valors estranys', 'Fes un script que…'],
      script: ['Explica\'m aquest codi', 'Troba errors', 'Afegeix una funció que…', 'Fes que s\'executi cada dia'],
      web: ['Resumeix la pàgina', 'Què en penses?'],
      pdf: ['Resumeix el PDF', 'Extreu les dades clau en una taula', 'Què he de fer o signar?'],
    },
  },
  en: {
    connect: 'Connect to Gregal', connectHelp: 'The address, port and key the server prints when it starts.',
    modeDesktop: '🖥️ Desktop app', modeServer: '🌐 Server',
    desktopHelp: 'Connects by itself to the Gregal app open on this computer. Nothing to fill in.',
    noDesktop: 'Can\'t find the desktop app. Open Gregal and try again.',
    address: 'Address', port: 'Port', key: 'Key', save: 'Connect', saved: 'Connected ✓', failed: 'No answer: %s',
    settings: 'Connection', placeholder: 'What do you want to do with this page?',
    noWinds: 'You have no winds yet: create one in the app.', hello: 'Hi!', helloSub: 'What shall we do with this page?', send: 'Send',
    copy: 'Copy', insert: 'Insert', replace: 'Replace the file', copied: 'Copied to the clipboard',
    inserted: 'Done ✓', pasteHint: 'Could not put it in directly: it is on the clipboard, press Ctrl+V where you want it.',
    error: 'Error: %s', needKey: 'Set up the connection to Gregal.',
    controlOn: 'Let Gregal use this browser',
    controlLive: 'Browser · %s', controlWaiting: 'Waiting for Gregal…',
    controlDenied: 'This is an older version of the extension: reload it on chrome://extensions (or prepare it again from Gregal) and try again.',
    controlHelp: 'Gregal will work in tabs of the «Gregal» group, with your logins. It asks before acting on each website and before sending, paying or deleting anything.',
    q: {
      docs: ['Summarise the document', 'Improve the wording of my selection', 'Check spelling and tone', 'Write a paragraph about…'],
      sheets: ['Explain this data', 'Write me a formula to…', 'Find errors or odd values', 'Write a script that…'],
      script: ['Explain this code', 'Find bugs', 'Add a function that…', 'Make it run every day'],
      web: ['Summarise the page', 'What do you think?'],
      pdf: ['Summarise the PDF', 'Extract the key data into a table', 'What do I need to do or sign?'],
    },
  },
}[lang];
const t = (k, ...a) => a.reduce((s, v) => s.replace('%s', v), T[k] ?? k);
const $ = (id) => document.getElementById(id);

let conf = { mode: 'desktop', addr: '', port: '', key: '', wind: '' };
let page = null; // {kind,title,url,range,content,tabId}
let busy = false;

for (const el of document.querySelectorAll('[data-t]')) el.textContent = t(el.dataset.t);
$('input').placeholder = t('placeholder');
$('settings-btn').title = t('settings');
$('send').title = t('send');

// La caixa creix amb el text, com als xats de sempre.
const grow = () => { const i = $('input'); i.style.height = 'auto'; i.style.height = Math.min(i.scrollHeight, 160) + 'px'; };
$('input').addEventListener('input', grow);

function toast(text) {
  const el = $('toast');
  el.textContent = text; el.hidden = false;
  clearTimeout(toast.timer); toast.timer = setTimeout(() => { el.hidden = true; }, 3500);
}

async function api(path, opts = {}) {
  const headers = { ...(opts.headers || {}) };
  const key = connection(conf).key;
  if (key) headers.Authorization = 'Bearer ' + key;
  if (opts.body) headers['Content-Type'] = 'application/json';
  headers['X-Gregal-Session'] = 'chrome';
  const res = await fetch(base() + path, { ...opts, headers });
  if (!res.ok) throw new Error(`${res.status} ${(await res.text()).slice(0, 120)}`);
  return res;
}

// --- Connexió ---
const base = () => { const c = connection(conf); return baseUrl(c.addr, c.port); };

async function loadConf() {
  conf = { ...conf, ...(await chrome.storage.local.get(['mode', 'addr', 'port', 'key', 'wind', 'desktopPort'])) };
  $('addr').value = conf.addr; $('port').value = conf.port; $('key').value = conf.key;
  paintMode();
}

function paintMode() {
  const desktop = conf.mode !== 'server';
  $('mode-desktop').classList.toggle('on', desktop);
  $('mode-server').classList.toggle('on', !desktop);
  $('desktop-help').hidden = !desktop;
  $('server-fields').hidden = desktop;
}
$('mode-desktop').onclick = () => { conf.mode = 'desktop'; paintMode(); $('status').textContent = ''; };
$('mode-server').onclick = () => { conf.mode = 'server'; paintMode(); $('status').textContent = ''; };

const openSettings = (open) => { $('settings').hidden = !open; };
$('settings-btn').onclick = () => openSettings(true);
$('settings-close').onclick = () => openSettings(false);
$('settings').onclick = (e) => { if (e.target === $('settings')) openSettings(false); };
document.addEventListener('keydown', (e) => { if (e.key === 'Escape') openSettings(false); });
$('save').onclick = async () => {
  if (conf.mode === 'server') {
    conf.addr = $('addr').value.trim();
    conf.port = $('port').value.trim();
    conf.key = $('key').value.trim();
    // Un servidor que no és aquesta màquina necessita permís per al seu origen.
    const origin = new URL(base()).origin + '/*';
    if (!/^http:\/\/(127\.0\.0\.1|localhost)[:/]/.test(origin)) {
      const ok = await chrome.permissions.request({ origins: [origin] }).catch(() => false);
      if (!ok) { $('status').textContent = t('failed', origin); return; }
    }
  }
  await chrome.storage.local.set({ mode: conf.mode, addr: conf.addr, port: conf.port, key: conf.key });
  $('status').textContent = '…';
  try {
    await loadWinds();
    $('status').textContent = t('saved');
    setTimeout(() => openSettings(false), 700);
  } catch (e) { $('status').textContent = conf.mode === 'server' ? t('failed', e.message) : t('noDesktop'); }
};

async function loadWinds() {
  const data = await (await api('/api/winds')).json();
  const winds = data.winds || [];
  const sel = $('wind');
  sel.innerHTML = '';
  for (const w of winds) {
    const o = document.createElement('option');
    o.value = w.id; o.textContent = w.name; o.title = w.role || '';
    sel.append(o);
  }
  if (!winds.length) { log(t('noWinds'), 'notice'); return; }
  sel.value = winds.some((w) => w.id === conf.wind) ? conf.wind : winds[0].id;
  conf.wind = sel.value;
}
$('wind').onchange = () => { conf.wind = $('wind').value; chrome.storage.local.set({ wind: conf.wind }); };

// --- Context de la pàgina ---
async function activeTab() {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  return tab;
}

async function run(tabId, func, args = [], world = 'ISOLATED') {
  const [r] = await chrome.scripting.executeScript({ target: { tabId }, func, args, world });
  return r && r.result;
}

/**
 * Llegeix la pestanya activa tal com és ara: Docs/Sheets amb l'exportació de
 * Google, Apps Script de l'editor i qualsevol web pel seu text. Es crida en
 * canviar de pestanya (per pintar-la) i abans d'enviar, perquè el vent vegi
 * la pàgina d'aquell moment (també el que s'ha carregat després).
 */
async function readPage() {
  const tab = await activeTab();
  if (!tab || !/^https?:/.test(tab.url || '')) { page = null; paintContext(); return; }
  if (isPdfUrl(tab.url)) {
    // Un PDF no canvia: es puja una sola vegada.
    if (!(page && page.kind === 'pdf' && page.url === tab.url && page.content)) await readPdf(tab);
    paintContext(); return;
  }
  let info;
  // On Chrome no deixa entrar (Web Store, chrome://): com a mínim el títol i l'adreça.
  try { info = await run(tab.id, pageInfo); } catch { info = { kind: 'web', title: tab.title, url: tab.url }; }
  page = { ...info, tabId: tab.id, icon: tab.favIconUrl || '', content: '' };
  try {
    if (info.kind === 'docs') {
      const r = await fetch(`https://docs.google.com/document/d/${info.id}/export?format=txt`, { credentials: 'include' });
      if (r.ok) page.content = (await r.text()).trim();
      if (info.selection) page.content = `${lang === 'ca' ? 'Selecció' : 'Selection'}:\n${info.selection}\n\n---\n${page.content}`;
    } else if (info.kind === 'sheets') {
      const r = await fetch(`https://docs.google.com/spreadsheets/d/${info.id}/export?format=csv&gid=${info.gid}`, { credentials: 'include' });
      if (r.ok) page.content = (await r.text()).trim();
    } else if (info.kind === 'script') {
      const files = await run(tab.id, scriptFiles, [], 'MAIN');
      page.content = (files || []).map((f) => `// ${f.name}${f.partial ? ' (part visible)' : ''}\n${f.code}`).join('\n\n');
    } else {
      page.content = info.selection || info.text || '';
    }
  } catch { /* sense context: el vent igualment pot respondre */ }
  paintContext();
}

/** Un PDF obert al visor de Chrome: se'n baixen els bytes (amb la sessió
 *  del navegador), es pugen a Gregal (/api/office/upload, com des de l'app) i
 *  el context és el seu text; la ruta deixa que el vent el llegeixi sencer. */
async function readPdf(tab) {
  const name = decodeURIComponent(new URL(tab.url).pathname.split('/').pop() || 'document.pdf');
  page = { kind: 'pdf', title: tab.title || name, url: tab.url, tabId: tab.id, icon: '', content: '', path: '' };
  try {
    const r = await fetch(tab.url, { credentials: 'include' });
    if (!r.ok) return;
    const bytes = new Uint8Array(await r.arrayBuffer());
    if (bytes.length > 25 * 1024 * 1024) return;
    const up = await (await api('/api/office/upload', { method: 'POST', body: JSON.stringify({ name, data: bytesToBase64(bytes) }) })).json();
    page.path = up.path || '';
    const rd = await (await api('/api/office/read', { method: 'POST', body: JSON.stringify({ id: up.id }) })).json();
    page.content = String(rd.text || '').trim();
  } catch { /* sense context: el vent igualment pot respondre */ }
}


/** El xip de la pestanya dins la caixa d'escriure, i les idees de la salutació. */
function paintContext() {
  const el = $('context');
  el.innerHTML = '';
  if (page) {
    if (page.icon) {
      const img = document.createElement('img'); img.src = page.icon; img.alt = '';
      img.onerror = () => img.replaceWith(glyph());
      el.append(img);
    } else el.append(glyph());
    const name = document.createElement('span');
    name.textContent = page.title || page.url;
    name.title = page.url;
    el.append(name);
  }
  const q = $('quick');
  q.innerHTML = '';
  for (const s of T.q[page ? page.kind : 'web'] || T.q.web) {
    const b = document.createElement('button');
    b.type = 'button'; b.textContent = s;
    b.onclick = () => { if (s.endsWith('…')) { $('input').value = s.slice(0, -1); $('input').focus(); grow(); } else send(s); };
    q.append(b);
  }
}

function glyph() {
  const g = document.createElement('span');
  g.className = 'glyph';
  g.textContent = { docs: '📄', sheets: '📊', script: '🧩', pdf: '📕' }[page && page.kind] || '🌐';
  return g;
}

chrome.tabs.onActivated.addListener(() => readPage());
chrome.tabs.onUpdated.addListener((id, ch, tab) => { if (tab.active && (ch.status === 'complete' || ch.title || ch.favIconUrl)) readPage(); });

// --- Xat ---
function log(text, cls) {
  const el = document.createElement('div');
  el.className = cls === 'notice' ? 'notice' : 'msg ' + cls;
  el.textContent = text;
  if (cls !== 'notice') $('hello').hidden = true;
  $('log').append(el);
  el.scrollIntoView({ block: 'end' });
  return el;
}

async function copy(text) {
  await navigator.clipboard.writeText(text);
}

async function put(text, replaceAll) {
  const kind = page ? page.kind : 'web';
  let ok = false;
  try {
    const tab = await activeTab();
    ok = !!(await run(tab.id, insertText, [text, kind, !!replaceAll], kind === 'script' ? 'MAIN' : 'ISOLATED'));
  } catch { ok = false; }
  if (ok) toast(t('inserted'));
  else { await copy(text).catch(() => {}); toast(t('pasteHint')); }
}

function actions(text, isCode) {
  const row = document.createElement('div');
  row.className = 'actions';
  const b1 = document.createElement('button'); b1.type = 'button'; b1.textContent = t('copy');
  b1.onclick = async () => { await copy(text); toast(t('copied')); };
  const b2 = document.createElement('button'); b2.type = 'button'; b2.textContent = t('insert');
  b2.onclick = () => put(text, false);
  row.append(b1, b2);
  if (isCode && page && page.kind === 'script') {
    const b3 = document.createElement('button'); b3.type = 'button'; b3.textContent = t('replace');
    b3.onclick = () => put(text, true);
    row.append(b3);
  }
  return row;
}

/** La resposta amb format: markdown (renderMarkdown ho escapa tot) i blocs de codi amb Copia/Insereix. */
function paintReply(el, text, final) {
  const body = el.querySelector('.body');
  body.innerHTML = '';
  const blocks = splitBlocks(text);
  for (const b of blocks) {
    if (b.type === 'code') {
      const box = document.createElement('div'); box.className = 'code';
      const pre = document.createElement('pre'); pre.textContent = b.text;
      box.append(pre);
      if (final) box.append(actions(b.text, true));
      body.append(box);
    } else {
      const div = document.createElement('div');
      div.innerHTML = renderMarkdown(b.text);
      body.append(...div.childNodes);
    }
  }
  if (final && text.trim() && !blocks.some((b) => b.type === 'code')) el.append(actions(text.trim(), false));
}

async function send(text) {
  text = String(text || '').trim();
  if (!text || busy) return;
  if (!conf.wind) { openSettings(true); toast(t('needKey')); return; }
  busy = true; $('send').disabled = true;
  log(text, 'user');
  $('input').value = ''; grow();
  const el = log('', 'wind busy');
  el.innerHTML = '';
  const who = document.createElement('div'); who.className = 'who'; who.textContent = $('wind').selectedOptions[0]?.textContent || '';
  const body = document.createElement('div'); body.className = 'body';
  el.append(who, body);
  await readPage().catch(() => {});
  const message = buildMessage(text, page, lang);
  let reply = '';
  try {
    const res = await api('/api/winds/chat/stream', { method: 'POST', body: JSON.stringify({ wind: conf.wind, message }) });
    const reader = res.body.getReader();
    const dec = new TextDecoder();
    let buf = '';
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      const { events, rest } = parseSSE(buf);
      buf = rest;
      for (const e of events) {
        // Els tokens d'abans d'una fase (cerca, eines) eren especulatius: fora.
        if (e.event === 'phase') reply = '';
        else if (e.event === 'token') reply += e.data.token || '';
        else if (e.event === 'done') reply = e.data.reply || reply;
        else if (e.event === 'error') throw new Error(e.data.error || 'error');
        paintReply(el, reply, false);
        el.scrollIntoView({ block: 'end' });
      }
    }
    paintReply(el, reply, true);
  } catch (e) {
    body.textContent = t('error', e.message);
  } finally {
    el.classList.remove('busy');
    busy = false; $('send').disabled = false;
  }
}

// --- Deixar que Gregal faci servir aquest navegador (als ajustos) ---
function paintControl(st, on) {
  $('control').checked = on;
  $('control-live').hidden = !on;
  $('control-live').classList.toggle('waiting', !(st && st.connected));
  $('control-live').title = t('controlHelp');
  $('control-text').textContent = st && st.connected ? t('controlLive', String(st.tabs || 0)) : t('controlWaiting');
}

async function refreshControl() {
  const { control } = await chrome.storage.local.get(['control']);
  const st = await chrome.runtime.sendMessage({ gregalStatus: true }).catch(() => null);
  paintControl(st, !!control);
}

$('control').onchange = async () => {
  if (!$('control').checked) {
    paintControl(await chrome.runtime.sendMessage({ gregalStop: true }).catch(() => null), false);
    return;
  }
  // Chrome no deixa que «debugger» sigui opcional: es concedeix en instal·lar
  // l'extensió, i l'interruptor és qui decideix si Gregal el fa servir. Si
  // falta (una versió antiga carregada), cal recarregar l'extensió.
  const ok = await chrome.permissions.contains({ permissions: ['debugger'] }).catch(() => false);
  if (!ok) { $('control').checked = false; toast(t('controlDenied')); return; }
  await chrome.storage.local.set({ control: true });
  paintControl(await chrome.runtime.sendMessage({ gregalLink: true }).catch(() => null), true);
};
$('control-live').onclick = () => openSettings(true);
chrome.runtime.onMessage.addListener((msg) => {
  if (msg && msg.gregalControl) chrome.storage.local.get(['control']).then(({ control }) => paintControl(msg.gregalControl, !!control));
});

$('form').onsubmit = (e) => { e.preventDefault(); send($('input').value); };
$('input').onkeydown = (e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send($('input').value); } };

(async () => {
  paintContext();
  await loadConf();
  refreshControl();
  try { await loadWinds(); } catch {
    openSettings(true);
    $('status').textContent = conf.mode === 'server' ? t('needKey') : t('noDesktop');
  }
  readPage();
  $('input').focus();
})();
