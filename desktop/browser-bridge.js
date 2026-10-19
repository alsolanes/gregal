// Pont entre l'eina `browser` de l'agent (backend Go) i el panell de
// navegador de l'app. El procés principal obre un servidor HTTP NOMÉS a
// 127.0.0.1, amb port aleatori i un token de 256 bits que passa al backend
// per l'entorn quan l'engega (GREGAL_APP_BROWSER). El backend hi envia
// ordres CDP i aquí es reenvien a webContents.debugger del webview de
// l'agent (partició persist:gregal-browser, la mateixa que el navegador de
// l'app: comparteixen sessions iniciades).
//
// Seguretat:
//   - només loopback (adreça remota i capçalera Host comprovades: res de
//     DNS rebinding) i cap petició amb Origin (cap pàgina web hi pot parlar);
//   - Authorization: Bearer <token> obligatori, comparació en temps constant;
//   - només dominis CDP de pàgina (Page, Runtime, Input, DOM): res de
//     Browser.*, Target.*, Storage.* ni Network.* que sortirien del webview;
//   - Page.navigate només a http(s) i mai a l'origen del mateix backend;
//   - el webview de destí l'ha de registrar la pàgina de confiança per IPC
//     i main.js comprova que és un guest d'aquesta finestra i partició.
//
// Contracte (POST, JSON): /attach, /cdp {method, params}, /release.
// Vegeu internal/tools/browser_app.go.
const http = require('node:http');
const crypto = require('node:crypto');

const ALLOWED_DOMAINS = new Set(['Page', 'Runtime', 'Input', 'DOM']);
const MAX_BODY = 1024 * 1024;
const PROTOCOL_VERSION = '1.3';

function isLoopbackAddress(address) {
  const a = String(address || '');
  return a === '::1' || /^(::ffff:)?127\.\d{1,3}\.\d{1,3}\.\d{1,3}$/.test(a);
}

function safeEqual(a, b) {
  const x = Buffer.from(String(a));
  const y = Buffer.from(String(b));
  return x.length === y.length && crypto.timingSafeEqual(x, y);
}

function allowedMethod(method) {
  if (typeof method !== 'string' || !/^[A-Z][A-Za-z]+\.[a-zA-Z]+$/.test(method)) return false;
  return ALLOWED_DOMAINS.has(method.split('.')[0]);
}

function navigableURL(url, blockedOrigin) {
  try {
    const u = new URL(url);
    if (!['http:', 'https:'].includes(u.protocol) || u.username || u.password) return false;
    const blocked = blockedOrigin ? blockedOrigin() : '';
    if (blocked) {
      try { if (new URL(blocked).origin === u.origin) return false; } catch (e) {}
    }
    return true;
  } catch (e) { return false; }
}

// createBrowserBridge: notify(state) avisa la finestra ('show' mostra el
// panell de l'agent, 'active' l'agent el fa servir, 'released' torna a
// l'usuari); blockedOrigin() és l'origen del backend (no s'hi navega).
function createBrowserBridge(options = {}) {
  const notify = options.notify || (() => {});
  const blockedOrigin = options.blockedOrigin || (() => '');
  const token = options.token || crypto.randomBytes(32).toString('hex');
  const attachTimeoutMs = options.attachTimeoutMs ?? 12000;
  let server = null;
  let port = 0;
  let target = null;
  let attached = false;
  let waiters = [];
  const scoped = new Map();
  function forSession(id) {
    if (!scoped.has(id)) scoped.set(id, createBrowserBridge({ ...options, notify: state => notify({state, session:id}) }));
    return scoped.get(id);
  }

  const alive = () => !!target && !(typeof target.isDestroyed === 'function' && target.isDestroyed());

  function detach() {
    if (target && attached) {
      try { target.debugger.detach(); } catch (e) { /* ja desenganxat */ }
    }
    attached = false;
  }

  // setTarget: el webview de l'agent ja és a punt (main.js n'ha validat
  // l'origen). Un de nou substitueix l'anterior.
  function setTarget(wc, session = '') {
    if (session) { forSession(session).setTarget(wc); return; }
    if (!wc || wc === target) return;
    detach();
    target = wc;
    wc.once?.('destroyed', () => {
      if (target === wc) { target = null; attached = false; }
    });
    wc.debugger.on?.('detach', () => {
      if (target === wc) attached = false;
    });
    const ready = waiters;
    waiters = [];
    for (const w of ready) w(true);
  }

  function clearTarget(wc) {
    for (const child of scoped.values()) child.clearTarget(wc);
    if (wc && wc !== target) return;
    detach();
    target = null;
  }

  function ensureAttached() {
    if (!alive()) return false;
    if (!attached) {
      if (!target.debugger.isAttached()) target.debugger.attach(PROTOCOL_VERSION);
      attached = true;
    }
    return true;
  }

  function waitTarget(ms) {
    if (alive()) return Promise.resolve(true);
    notify('show');
    return new Promise(resolve => {
      const done = ok => { clearTimeout(timer); waiters = waiters.filter(w => w !== done); resolve(ok); };
      const timer = setTimeout(() => done(false), ms);
      waiters.push(done);
    });
  }

  function send(res, status, body) {
    const raw = JSON.stringify(body);
    res.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
    res.end(raw);
  }

  function readBody(req) {
    return new Promise((resolve, reject) => {
      let size = 0;
      const chunks = [];
      req.on('data', chunk => {
        size += chunk.length;
        if (size > MAX_BODY) { reject(Object.assign(new Error('body too large'), { status: 413 })); req.destroy(); return; }
        chunks.push(chunk);
      });
      req.on('end', () => {
        if (!chunks.length) return resolve({});
        try { resolve(JSON.parse(Buffer.concat(chunks).toString('utf8'))); } catch (e) { reject(Object.assign(new Error('invalid JSON'), { status: 400 })); }
      });
      req.on('error', reject);
    });
  }

  // authorize torna l'error (status, missatge) o null si la petició és bona.
  function authorize(req) {
    if (!isLoopbackAddress(req.socket && req.socket.remoteAddress)) return [403, 'loopback only'];
    const host = String(req.headers.host || '');
    if (port && host !== '127.0.0.1:' + port) return [403, 'bad host'];
    if (req.headers.origin) return [403, 'browser requests are not allowed'];
    if (req.method !== 'POST') return [405, 'POST only'];
    const auth = String(req.headers.authorization || '');
    if (!auth.startsWith('Bearer ') || !safeEqual(auth.slice(7), token)) return [401, 'unauthorized'];
    return null;
  }

  async function handle(req, res) {
    const denied = authorize(req);
    if (denied) return send(res, denied[0], { error: denied[1] });
    let body;
    try { body = await readBody(req); } catch (e) { return send(res, e.status || 400, { error: e.message }); }
    const path = String(req.url || '').split('?')[0];
    const scope = req.headers['x-gregal-session'];
    if (scope && (typeof scope !== 'string' || !/^[a-zA-Z0-9_.-]{1,128}$/.test(scope))) return send(res,400,{error:'invalid browser session'});
    return (scope ? forSession(scope).dispatch : dispatch)(path, body, res);
  }

  async function dispatch(path, body, res) {
    if (path === '/attach') {
      if (!(await waitTarget(attachTimeoutMs))) return send(res, 503, { error: 'the app browser pane is not ready' });
      try { ensureAttached(); } catch (e) { return send(res, 409, { error: 'cannot attach to the app browser: ' + e.message }); }
      notify('active');
      return send(res, 200, { ok: true });
    }
    if (path === '/release') {
      detach();
      notify('released');
      return send(res, 200, { ok: true });
    }
    if (path === '/cdp') {
      const method = body && body.method;
      const params = body && body.params && typeof body.params === 'object' ? body.params : {};
      if (!allowedMethod(method)) return send(res, 200, { error: { message: 'CDP method not allowed by the app bridge: ' + String(method).slice(0, 80) } });
      if (method === 'Page.navigate' && !navigableURL(params.url, blockedOrigin)) return send(res, 200, { error: { message: 'navigation blocked by the app bridge' } });
      if (!alive()) return send(res, 410, { error: 'the app browser pane was closed' });
      try { ensureAttached(); } catch (e) { return send(res, 409, { error: 'cannot attach to the app browser: ' + e.message }); }
      // L'usuari ha de veure on va l'agent: navegar o capturar porta el
      // panell a la vista si l'havia amagat.
      if (method === 'Page.navigate' || method === 'Page.captureScreenshot') notify('show');
      try {
        const result = await target.debugger.sendCommand(method, params);
        return send(res, 200, { result: result || {} });
      } catch (e) {
        return send(res, 200, { error: { message: String(e && e.message || e) } });
      }
    }
    return send(res, 404, { error: 'not found' });
  }

  function start() {
    return new Promise((resolve, reject) => {
      server = http.createServer((req, res) => {
        handle(req, res).catch(e => { try { send(res, 500, { error: String(e && e.message || e) }); } catch (x) {} });
      });
      server.on('error', reject);
      server.listen(0, '127.0.0.1', () => {
        port = server.address().port;
        resolve({ port, url: envValue() });
      });
    });
  }

  function envValue() {
    return port ? 'http://127.0.0.1:' + port + '/?token=' + token : '';
  }

  function close() {
    for (const child of scoped.values()) child.close();
    scoped.clear();
    detach();
    target = null;
    for (const w of waiters.splice(0)) w(false);
    if (server) server.close();
    server = null;
  }

  return {
    start, close, envValue, handle, dispatch, setTarget, clearTarget,
    isAttached: () => attached,
    hasTarget: alive,
    get port() { return port; },
  };
}

// validGuest diu si un webContents és el webview de l'agent d'aquesta
// finestra: tipus webview, allotjat a la finestra i a la partició pròpia.
function validGuest(guest, hostWebContents, partitionSession) {
  if (!guest || typeof guest.getType !== 'function') return false;
  if (typeof guest.isDestroyed === 'function' && guest.isDestroyed()) return false;
  if (guest.getType() !== 'webview') return false;
  if (!hostWebContents || guest.hostWebContents !== hostWebContents) return false;
  if (partitionSession && guest.session !== partitionSession) return false;
  return true;
}

module.exports = { createBrowserBridge, validGuest, isLoopbackAddress, allowedMethod, navigableURL, AGENT_BROWSER_PARTITION: 'persist:gregal-browser' };
