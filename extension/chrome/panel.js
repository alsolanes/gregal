// Panell lateral de Gregal: parla amb un vent del teu Gregal (el de
// l'escriptori o el teu servidor) amb la pàgina com a context, i posa la
// resposta a la pàgina o al porta-retalls. Fa servir la mateixa API que
// l'app del mòbil: /api/winds i /api/winds/chat/stream amb la clau Bearer.
import { baseUrl, connection, parseSSE, splitBlocks, buildMessage } from './logic.js';
import { pageInfo, scriptFiles, insertText } from './page.js';

const lang = (chrome.i18n.getUILanguage() || 'ca').startsWith('ca') ? 'ca' : 'en';
const T = {
  ca: {
    connect: 'Connecta amb Gregal', connectHelp: 'L\'adreça, el port i la clau que diu el servidor en engegar-se.',
    modeDesktop: '🖥️ App d\'escriptori', modeServer: '🌐 Servidor',
    desktopHelp: 'Es connecta sol a l\'app de Gregal oberta en aquest ordinador. No cal posar res.',
    noDesktop: 'No trobo l\'app d\'escriptori. Obre Gregal i torna-ho a provar.',
    address: 'Adreça', port: 'Port', key: 'Clau', save: 'Connecta', saved: 'Connectat ✓', failed: 'No respon: %s',
    settings: 'Connexió', placeholder: 'Què vols fer amb aquesta pàgina?', include: 'Inclou la pàgina',
    noWinds: 'Encara no tens cap vent: crea\'n un a l\'app.', empty: 'Pregunta el que vulguis: el vent veurà la pàgina que tens oberta.',
    copy: 'Copia', insert: 'Insereix', replace: 'Substitueix el fitxer', copied: 'Copiat al porta-retalls',
    inserted: 'Fet ✓', pasteHint: 'No s\'ha pogut posar directament: és al porta-retalls, prem Ctrl+V on vulguis.',
    reading: 'Llegint la pàgina…', unreadable: 'no s\'ha pogut llegir', error: 'Error: %s', needKey: 'Configura la connexió amb Gregal.',
    q: {
      docs: ['Resumeix el document', 'Millora la redacció del que he seleccionat', 'Revisa l\'ortografia i el to', 'Escriu-me un paràgraf sobre…'],
      sheets: ['Explica\'m aquestes dades', 'Fes-me una fórmula per…', 'Troba errors o valors estranys', 'Fes un script que…'],
      script: ['Explica\'m aquest codi', 'Troba errors', 'Afegeix una funció que…', 'Fes que s\'executi cada dia'],
      web: ['Resumeix la pàgina', 'Què en penses?'],
    },
  },
  en: {
    connect: 'Connect to Gregal', connectHelp: 'The address, port and key the server prints when it starts.',
    modeDesktop: '🖥️ Desktop app', modeServer: '🌐 Server',
    desktopHelp: 'Connects by itself to the Gregal app open on this computer. Nothing to fill in.',
    noDesktop: 'Can\'t find the desktop app. Open Gregal and try again.',
    address: 'Address', port: 'Port', key: 'Key', save: 'Connect', saved: 'Connected ✓', failed: 'No answer: %s',
    settings: 'Connection', placeholder: 'What do you want to do with this page?', include: 'Include the page',
    noWinds: 'You have no winds yet: create one in the app.', empty: 'Ask anything: the wind will see the page you have open.',
    copy: 'Copy', insert: 'Insert', replace: 'Replace the file', copied: 'Copied to the clipboard',
    inserted: 'Done ✓', pasteHint: 'Could not put it in directly: it is on the clipboard, press Ctrl+V where you want it.',
    reading: 'Reading the page…', unreadable: 'could not be read', error: 'Error: %s', needKey: 'Set up the connection to Gregal.',
    q: {
      docs: ['Summarise the document', 'Improve the wording of my selection', 'Check spelling and tone', 'Write a paragraph about…'],
      sheets: ['Explain this data', 'Write me a formula to…', 'Find errors or odd values', 'Write a script that…'],
      script: ['Explain this code', 'Find bugs', 'Add a function that…', 'Make it run every day'],
      web: ['Summarise the page', 'What do you think?'],
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
  $('settings-btn').textContent = desktop ? '🖥️' : '🌐';
}
$('mode-desktop').onclick = () => { conf.mode = 'desktop'; paintMode(); $('status').textContent = ''; };
$('mode-server').onclick = () => { conf.mode = 'server'; paintMode(); $('status').textContent = ''; };

$('settings-btn').onclick = () => { $('settings').hidden = !$('settings').hidden; };
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
    setTimeout(() => { $('settings').hidden = true; }, 700);
  } catch (e) { $('status').textContent = conf.mode === 'server' ? t('failed', e.message) : t('noDesktop'); }
};

async function loadWinds() {
  const data = await (await api('/api/winds')).json();
  const winds = data.winds || [];
  const sel = $('wind');
  sel.innerHTML = '';
  for (const w of winds) {
    const o = document.createElement('option');
    o.value = w.id; o.textContent = w.name + (w.role ? ' · ' + w.role : '');
    sel.append(o);
  }
  if (!winds.length) { log(t('noWinds'), 'empty'); return; }
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

/** Llegeix la pestanya activa: Docs/Sheets amb l'exportació de Google, Apps Script de l'editor. */
async function readPage() {
  const tab = await activeTab();
  page = null;
  if (!tab || !/^https?:/.test(tab.url || '')) { paintContext(); return; }
  let info;
  try { info = await run(tab.id, pageInfo); } catch { info = { kind: 'web', title: tab.title, url: tab.url }; }
  page = { ...info, tabId: tab.id, content: '' };
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

function paintContext() {
  const el = $('context');
  el.innerHTML = '';
  if (!page) return;
  const icon = { docs: '📄', sheets: '📊', script: '🧩', web: '🌐' }[page.kind] || '🌐';
  const name = document.createElement('b');
  name.textContent = `${icon} ${page.title || page.url}`;
  el.append(name);
  if (!page.content) {
    const s = document.createElement('span'); s.textContent = '· ' + t('unreadable'); el.append(s);
  } else {
    const lab = document.createElement('label');
    const cb = document.createElement('input'); cb.type = 'checkbox'; cb.id = 'include'; cb.checked = true;
    lab.append(cb, document.createTextNode(t('include')));
    el.append(lab);
  }
  const q = $('quick');
  q.innerHTML = '';
  for (const s of T.q[page.kind] || T.q.web) {
    const b = document.createElement('button');
    b.type = 'button'; b.textContent = s;
    b.onclick = () => { if (s.endsWith('…')) { $('input').value = s.slice(0, -1); $('input').focus(); } else send(s); };
    q.append(b);
  }
}

chrome.tabs.onActivated.addListener(() => readPage());
chrome.tabs.onUpdated.addListener((id, ch, tab) => { if (tab.active && (ch.status === 'complete' || ch.title)) readPage(); });

// --- Xat ---
function log(text, cls) {
  const el = document.createElement('div');
  el.className = cls === 'empty' ? 'empty' : 'msg ' + cls;
  el.textContent = text;
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
  const b1 = document.createElement('button'); b1.textContent = t('copy');
  b1.onclick = async () => { await copy(text); toast(t('copied')); };
  const b2 = document.createElement('button'); b2.textContent = t('insert');
  b2.onclick = () => put(text, false);
  row.append(b1, b2);
  if (isCode && page && page.kind === 'script') {
    const b3 = document.createElement('button'); b3.textContent = t('replace');
    b3.onclick = () => put(text, true);
    row.append(b3);
  }
  return row;
}

function paintReply(el, text, final) {
  const body = el.querySelector('.body');
  body.innerHTML = '';
  for (const b of splitBlocks(text)) {
    if (b.type === 'code') {
      const pre = document.createElement('pre'); pre.textContent = b.text;
      body.append(pre);
      if (final) body.append(actions(b.text, true));
    } else {
      body.append(document.createTextNode(b.text.replace(/\*\*(.+?)\*\*/g, '$1')));
    }
  }
  if (final && text.trim() && !splitBlocks(text).some((b) => b.type === 'code')) el.append(actions(text.trim(), false));
}

async function send(text) {
  text = String(text || '').trim();
  if (!text || busy) return;
  if (!conf.wind) { $('settings').hidden = false; toast(t('needKey')); return; }
  busy = true; $('send').disabled = true;
  log(text, 'user');
  $('input').value = '';
  const include = $('include') ? $('include').checked : false;
  const message = buildMessage(text, include ? page : null, lang);
  const el = log('', 'wind busy');
  el.innerHTML = '';
  const who = document.createElement('div'); who.className = 'who'; who.textContent = $('wind').selectedOptions[0]?.textContent.split(' · ')[0] || '';
  const body = document.createElement('div'); body.className = 'body';
  el.append(who, body);
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

$('form').onsubmit = (e) => { e.preventDefault(); send($('input').value); };
$('input').onkeydown = (e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send($('input').value); } };

(async () => {
  await loadConf();
  try { await loadWinds(); log(t('empty'), 'empty'); } catch {
    $('settings').hidden = false;
    $('status').textContent = conf.mode === 'server' ? t('needKey') : t('noDesktop');
  }
  readPage();
})();
