// La icona obre el panell lateral: el xat amb els vents viu al costat de la pàgina.
//
// A més, si l'usuari ho activa al panell («Deixa que Gregal faci servir aquest
// navegador»; el permís de depuració ve amb la instal·lació, perquè Chrome
// no el deixa demanar després), aquest service worker obre un websocket cap a Gregal i hi fa de
// mans: obre pestanyes al grup «Gregal», hi enganxa el depurador i hi envia
// les ordres CDP de l'agent. Només toca pestanyes d'aquell grup; l'usuari pot
// arrossegar-hi una pestanya per compartir-la, o prémer Atura.
import { connection, extensionSocketUrl } from './logic.js';

chrome.runtime.onInstalled.addListener(() => {
  chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: true }).catch(() => {});
  link();
});
chrome.runtime.onStartup.addListener(link);

const GROUP = 'Gregal';
let ws = null;
let pinger = null;
const attached = new Set();

async function settings() {
  return chrome.storage.local.get(['mode', 'addr', 'port', 'key', 'desktopPort', 'control']);
}

async function canControl() {
  const c = await settings();
  return !!c.control && (await chrome.permissions.contains({ permissions: ['debugger'] }).catch(() => false));
}

function send(msg) {
  if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(msg));
}

function status() {
  return { connected: !!(ws && ws.readyState === WebSocket.OPEN), tabs: attached.size };
}

function broadcast() {
  chrome.runtime.sendMessage({ gregalControl: status() }).catch(() => {});
}

/** Connecta amb Gregal si l'usuari ho ha activat (i torna-hi si cau). */
async function link() {
  if (!(await canControl())) { unlink(); return; }
  if (ws && ws.readyState <= WebSocket.OPEN) return;
  const c = await settings();
  const conn = connection(c);
  let sock;
  try { sock = new WebSocket(extensionSocketUrl(conn)); } catch { return; }
  ws = sock;
  sock.onopen = () => {
    sock.send(JSON.stringify({ type: 'hello', key: conn.key, version: chrome.runtime.getManifest().version }));
    clearInterval(pinger);
    // L'activitat del websocket manté viu el service worker (Chrome 116+).
    pinger = setInterval(() => send({ event: 'ping' }), 20000);
    broadcast();
  };
  sock.onmessage = (e) => { let m; try { m = JSON.parse(e.data); } catch { return; } if (m.id) handle(m); };
  sock.onclose = () => { if (ws === sock) { ws = null; clearInterval(pinger); broadcast(); } };
  sock.onerror = () => {};
}

function unlink() {
  if (ws) { const s = ws; ws = null; s.close(); }
  clearInterval(pinger);
  broadcast();
}

// Si el worker s'ha adormit o Gregal s'ha reiniciat, es torna a connectar.
chrome.alarms.create('gregal-link', { periodInMinutes: 0.5 });
chrome.alarms.onAlarm.addListener((a) => { if (a.name === 'gregal-link') link(); });

async function groupId(windowId) {
  const groups = await chrome.tabGroups.query({ title: GROUP, ...(windowId ? { windowId } : {}) });
  return groups.length ? groups[0].id : null;
}

async function addToGroup(tabId) {
  const tab = await chrome.tabs.get(tabId);
  const gid = await groupId(tab.windowId);
  if (gid !== null) { await chrome.tabs.group({ tabIds: [tabId], groupId: gid }); return; }
  const id = await chrome.tabs.group({ tabIds: [tabId] });
  await chrome.tabGroups.update(id, { title: GROUP, color: 'blue' });
}

async function inGroup(tabId) {
  const tab = await chrome.tabs.get(tabId).catch(() => null);
  if (!tab || tab.groupId === -1) return false;
  const g = await chrome.tabGroups.get(tab.groupId).catch(() => null);
  return !!g && g.title === GROUP;
}

async function attach(tabId) {
  if (attached.has(tabId)) return;
  await chrome.debugger.attach({ tabId }, '1.3');
  attached.add(tabId);
  broadcast();
}

const ops = {
  async open({ url }) {
    const tab = await chrome.tabs.create({ url: url || 'about:blank', active: true });
    await addToGroup(tab.id);
    await attach(tab.id);
    return { tab: tab.id };
  },
  async cdp({ tab, method, params }) {
    if (!attached.has(tab)) throw new Error('not a Gregal tab (or the user stopped it)');
    return (await chrome.debugger.sendCommand({ tabId: tab }, method, params || {})) || {};
  },
  async tabs() {
    const all = await chrome.tabs.query({});
    const out = [];
    for (const t of all) if (await inGroup(t.id)) out.push({ id: t.id, title: t.title || '', url: t.url || '' });
    return out;
  },
  async select({ tab }) {
    if (!(await inGroup(tab))) throw new Error('Gregal only works in tabs of the «Gregal» group');
    await chrome.tabs.update(tab, { active: true });
    await attach(tab);
    return { tab };
  },
  async close({ tab }) {
    if (!(await inGroup(tab))) throw new Error('Gregal only closes tabs of the «Gregal» group');
    attached.delete(tab);
    await chrome.tabs.remove(tab);
    broadcast();
    return {};
  },
  async detach({ tab }) {
    if (attached.has(tab)) { attached.delete(tab); await chrome.debugger.detach({ tabId: tab }).catch(() => {}); broadcast(); }
    return {};
  },
};

async function handle(m) {
  const op = ops[m.op];
  if (!op) { send({ id: m.id, error: 'unknown op ' + m.op }); return; }
  try {
    send({ id: m.id, result: await op(m) });
  } catch (e) {
    send({ id: m.id, error: String((e && e.message) || e) });
  }
}

// Pestanyes que obre una pestanya de Gregal (enllaços target=_blank): al grup.
chrome.tabs.onCreated.addListener(async (tab) => {
  if (tab.openerTabId && attached.has(tab.openerTabId)) await addToGroup(tab.id).catch(() => {});
});
chrome.tabs.onRemoved.addListener((tabId) => { if (attached.delete(tabId)) broadcast(); });

// La barra de Chrome «Gregal està depurant aquest navegador» té Cancel·la:
// aleshores Gregal no s'hi torna a enganxar sol.
chrome.debugger.onDetach.addListener(async (src, reason) => {
  if (!attached.delete(src.tabId)) return;
  send({ event: 'detached', tab: src.tabId, reason });
  // L'usuari ho ha cancel·lat: l'interruptor del panell queda apagat.
  if (reason === 'canceled_by_user') await turnOff();
  broadcast();
});

async function stopAll() {
  for (const tabId of [...attached]) await chrome.debugger.detach({ tabId }).catch(() => {});
  attached.clear();
  send({ event: 'stop' });
  broadcast();
}

/** Atura i apaga: Gregal no hi torna fins que l'usuari ho torni a activar. */
async function turnOff() {
  await stopAll();
  await chrome.storage.local.set({ control: false });
  unlink();
}

chrome.runtime.onMessage.addListener((msg, _sender, reply) => {
  if (msg && msg.gregalLink) { link().then(() => reply(status())); return true; }
  if (msg && msg.gregalStop) { turnOff().then(() => reply(status())); return true; }
  if (msg && msg.gregalStatus) { reply(status()); return false; }
  return false;
});

link();
