const test = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { EventEmitter } = require('node:events');
const { createRequire } = require('node:module');
const { createBrowserBridge, validGuest, isLoopbackAddress, allowedMethod, navigableURL } = require('./browser-bridge');

// fakeGuest imita el webContents d'un webview amb el seu debugger.
function fakeGuest(extra = {}) {
  const wc = new EventEmitter();
  const dbg = new EventEmitter();
  dbg.calls = [];
  dbg.attachedFlag = false;
  dbg.attach = version => { dbg.calls.push('attach:' + version); dbg.attachedFlag = true; };
  dbg.detach = () => { dbg.calls.push('detach'); dbg.attachedFlag = false; };
  dbg.isAttached = () => dbg.attachedFlag;
  dbg.sendCommand = async (method, params) => {
    dbg.calls.push(method);
    if (method === 'Runtime.evaluate') return { result: { value: 'eval:' + params.expression } };
    if (method === 'Page.navigate') return { frameId: 'F1' };
    throw new Error('unexpected ' + method);
  };
  wc.debugger = dbg;
  wc.destroyed = false;
  wc.isDestroyed = () => wc.destroyed;
  wc.getType = () => 'webview';
  Object.assign(wc, extra);
  return wc;
}

function request(port, { method = 'POST', pathName = '/cdp', token, body, headers = {} } = {}) {
  return new Promise((resolve, reject) => {
    const raw = body === undefined ? '' : JSON.stringify(body);
    const h = { 'Content-Type': 'application/json', ...headers };
    if (token) h.Authorization = 'Bearer ' + token;
    const req = http.request({ host: '127.0.0.1', port, method, path: pathName, headers: h }, res => {
      let data = '';
      res.setEncoding('utf8');
      res.on('data', c => { data += c; });
      res.on('end', () => resolve({ status: res.statusCode, body: data ? JSON.parse(data) : null }));
    });
    req.on('error', reject);
    req.end(raw);
  });
}

async function startedBridge(t, options = {}) {
  const states = [];
  const bridge = createBrowserBridge({ token: 'tok-123', notify: s => states.push(s), blockedOrigin: () => 'http://127.0.0.1:8097', attachTimeoutMs: 300, ...options });
  await bridge.start();
  t.after(() => bridge.close());
  return { bridge, states, port: bridge.port, token: 'tok-123' };
}

test('bridge listens on loopback and hands the backend a tokenised URL', async t => {
  const { bridge, port } = await startedBridge(t);
  assert.match(bridge.envValue(), new RegExp('^http://127\\.0\\.0\\.1:' + port + '/\\?token=tok-123$'));
  const random = createBrowserBridge();
  await random.start();
  try {
    const token = new URL(random.envValue()).searchParams.get('token');
    assert.match(token, /^[0-9a-f]{64}$/);
  } finally { random.close(); }
});

test('bridge requires the token and refuses browsers, foreign hosts and other verbs', async t => {
  const { bridge, port, token } = await startedBridge(t);
  const guest = fakeGuest();
  bridge.setTarget(guest);
  const cdp = { method: 'Runtime.evaluate', params: { expression: '1' } };
  assert.equal((await request(port, { body: cdp })).status, 401);
  assert.equal((await request(port, { body: cdp, token: 'tok-12' })).status, 401);
  assert.equal((await request(port, { body: cdp, token: token + 'x' })).status, 401);
  assert.equal((await request(port, { body: cdp, token, headers: { Origin: 'https://evil.example' } })).status, 403);
  assert.equal((await request(port, { body: cdp, token, headers: { Host: 'evil.example:' + port } })).status, 403);
  assert.equal((await request(port, { method: 'GET', token })).status, 405);
  assert.deepEqual(guest.debugger.calls, [], 'nothing reaches the debugger without authorisation');
  const ok = await request(port, { body: cdp, token });
  assert.equal(ok.status, 200);
  assert.deepEqual(ok.body, { result: { result: { value: 'eval:1' } } });
});

test('bridge rejects non-loopback peers before anything else', async () => {
  const bridge = createBrowserBridge({ token: 't' });
  const res = { status: 0, body: '', writeHead(s) { this.status = s; }, end(b) { this.body = b; } };
  await bridge.handle({ socket: { remoteAddress: '192.168.1.20' }, headers: { authorization: 'Bearer t' }, method: 'POST', url: '/cdp' }, res);
  assert.equal(res.status, 403);
  assert.equal(isLoopbackAddress('127.0.0.1'), true);
  assert.equal(isLoopbackAddress('::ffff:127.0.0.1'), true);
  assert.equal(isLoopbackAddress('::1'), true);
  for (const a of ['10.0.0.1', '::ffff:10.0.0.1', '', undefined, '127.0.0.1.evil']) assert.equal(isLoopbackAddress(a), false);
});

test('bridge relays page CDP only and guards navigation', async t => {
  const { bridge, states, port, token } = await startedBridge(t);
  const guest = fakeGuest();
  bridge.setTarget(guest);
  const attach = await request(port, { pathName: '/attach', token, body: {} });
  assert.equal(attach.status, 200);
  assert.deepEqual(guest.debugger.calls, ['attach:1.3']);
  assert.equal(bridge.isAttached(), true);
  assert.deepEqual(states, ['active']);
  for (const method of ['Browser.close', 'Target.createTarget', 'Storage.clearDataForOrigin', 'Network.getAllCookies', 'nope', '']) {
    const r = await request(port, { token, body: { method } });
    assert.equal(r.status, 200);
    assert.match(r.body.error.message, /not allowed/);
  }
  for (const url of ['file:///etc/passwd', 'javascript:alert(1)', 'http://127.0.0.1:8097/api/state', 'https://u:p@example.org']) {
    const r = await request(port, { token, body: { method: 'Page.navigate', params: { url } } });
    assert.match(r.body.error.message, /navigation blocked/, url);
  }
  const nav = await request(port, { token, body: { method: 'Page.navigate', params: { url: 'http://localhost:3000/' } } });
  assert.deepEqual(nav.body, { result: { frameId: 'F1' } });
  assert.deepEqual(states, ['active', 'show'], 'navigating brings the pane into view');
  const failing = await request(port, { token, body: { method: 'DOM.getDocument' } });
  assert.match(failing.body.error.message, /unexpected DOM.getDocument/);
  assert.deepEqual(guest.debugger.calls, ['attach:1.3', 'Page.navigate', 'DOM.getDocument']);
  assert.equal(allowedMethod('Input.dispatchMouseEvent'), true);
  assert.equal(allowedMethod('Accessibility.getFullAXTree'), true);
  assert.equal(allowedMethod('Browser.close'), false);
  assert.equal(allowedMethod('Target.createTarget'), false);
  assert.equal(navigableURL('https://example.org', () => ''), true);
});

test('bridge follows the webview lifecycle: detach, release and destroy', async t => {
  const { bridge, states, port, token } = await startedBridge(t);
  const guest = fakeGuest();
  bridge.setTarget(guest);
  const evalBody = { method: 'Runtime.evaluate', params: { expression: 'x' } };
  assert.equal((await request(port, { token, body: evalBody })).status, 200);
  assert.equal(bridge.isAttached(), true);
  // DevTools o Chromium desenganxen el depurador: la propera ordre el torna a enganxar.
  guest.debugger.attachedFlag = false;
  guest.debugger.emit('detach', {}, 'target closed');
  assert.equal(bridge.isAttached(), false);
  assert.equal((await request(port, { token, body: evalBody })).status, 200);
  assert.equal(guest.debugger.calls.filter(c => c.startsWith('attach')).length, 2);
  // L'agent tanca: el depurador es desenganxa i el panell torna a l'usuari.
  assert.equal((await request(port, { pathName: '/release', token, body: {} })).status, 200);
  assert.equal(guest.debugger.calls.at(-1), 'detach');
  assert.equal(bridge.isAttached(), false);
  assert.equal(states.at(-1), 'released');
  // El webview desapareix (recàrrega de la finestra): 410 i cap ordre més.
  guest.destroyed = true;
  guest.emit('destroyed');
  assert.equal(bridge.hasTarget(), false);
  const gone = await request(port, { token, body: evalBody });
  assert.equal(gone.status, 410);
  // Un webview nou substitueix l'anterior i desenganxa el vell.
  const next = fakeGuest();
  const old = fakeGuest();
  bridge.setTarget(old);
  assert.equal((await request(port, { token, body: evalBody })).status, 200);
  bridge.setTarget(next);
  assert.equal(old.debugger.calls.at(-1), 'detach');
  assert.equal((await request(port, { token, body: evalBody })).status, 200);
  assert.deepEqual(next.debugger.calls, ['attach:1.3', 'Runtime.evaluate']);
});

test('attach asks the window to show the pane and waits for the webview', async t => {
  const { bridge, states, port, token } = await startedBridge(t, { attachTimeoutMs: 2000 });
  const pending = request(port, { pathName: '/attach', token, body: {} });
  await new Promise(r => setTimeout(r, 50));
  assert.deepEqual(states, ['show']);
  const guest = fakeGuest();
  bridge.setTarget(guest);
  const res = await pending;
  assert.equal(res.status, 200);
  assert.deepEqual(guest.debugger.calls, ['attach:1.3']);
});

test('attach times out with 503 when no webview registers', async t => {
  const { port, token } = await startedBridge(t, { attachTimeoutMs: 50 });
  const res = await request(port, { pathName: '/attach', token, body: {} });
  assert.equal(res.status, 503);
  assert.match(res.body.error, /not ready/);
});

test('attach reports a debugger conflict (DevTools already attached)', async t => {
  const { bridge, port, token } = await startedBridge(t);
  const guest = fakeGuest();
  guest.debugger.attach = () => { throw new Error('Another debugger is already attached'); };
  bridge.setTarget(guest);
  const res = await request(port, { pathName: '/attach', token, body: {} });
  assert.equal(res.status, 409);
  assert.match(res.body.error, /already attached/);
});

test('only webview guests of this window and partition are valid targets', () => {
  const host = {};
  const partition = {};
  assert.equal(validGuest(fakeGuest({ hostWebContents: host, session: partition }), host, partition), true);
  assert.equal(validGuest(fakeGuest({ hostWebContents: {}, session: partition }), host, partition), false);
  assert.equal(validGuest(fakeGuest({ hostWebContents: host, session: {} }), host, partition), false);
  assert.equal(validGuest(fakeGuest({ hostWebContents: host, session: partition, getType: () => 'window' }), host, partition), false);
  const dead = fakeGuest({ hostWebContents: host, session: partition });
  dead.destroyed = true;
  assert.equal(validGuest(dead, host, partition), false);
  assert.equal(validGuest(null, host, partition), false);
});

test('main registers the agent webview only from the trusted page', () => {
  const listeners = new Map();
  const host = { mainFrame: { url: 'http://127.0.0.1:8097/' } };
  const partition = { name: 'persist:gregal-browser' };
  const guests = new Map([
    [7, fakeGuest({ hostWebContents: host, session: partition })],
    [8, fakeGuest({ hostWebContents: {}, session: partition })],
    [9, fakeGuest({ hostWebContents: host, session: {} })],
  ]);
  const electron = {
    app: { getPath: () => path.join(require('node:os').tmpdir(), 'gregal-bridge-test-no-profile'), whenReady: () => ({ then() {} }), on() {} },
    ipcMain: { on: (name, fn) => listeners.set(name, fn), handle() {} },
    nativeTheme: { shouldUseDarkColors: false },
    webContents: { fromId: id => guests.get(id) || null },
    session: { fromPartition: name => name === 'persist:gregal-browser' ? partition : {} },
  };
  const localRequire = createRequire(path.join(__dirname, 'main.js'));
  const context = {
    require: name => name === 'electron' ? electron : localRequire(name),
    process: { env: {}, platform: process.platform },
    __dirname, URL, console, setTimeout, clearTimeout,
  };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'main.js'), 'utf8') + '\nthis.testWire = wireIPC; this.testSetBridge = value => { browserBridge = value; };', context);
  const targets = [];
  context.testSetBridge({ setTarget: g => targets.push(g) });
  context.testWire(() => ({ webContents: host }));
  const ready = listeners.get('gregal:agent-browser-ready');
  const trusted = { sender: host, senderFrame: host.mainFrame };
  ready({ sender: {}, senderFrame: host.mainFrame }, 7);
  ready({ sender: host, senderFrame: { url: host.mainFrame.url } }, 7);
  ready(trusted, '7');
  ready(trusted, 8);
  ready(trusted, 9);
  ready(trusted, 404);
  assert.equal(targets.length, 0, 'foreign pages, other hosts and other partitions are ignored');
  ready(trusted, 7);
  assert.deepEqual(targets, [guests.get(7)]);
});


test('session browser targets receive only their own commands and release independently', async t => {
  const {bridge,states,port,token}=await startedBridge(t);
  const alpha=fakeGuest(),beta=fakeGuest();
  bridge.setTarget(alpha,'alpha');bridge.setTarget(beta,'beta');
  const scoped=(scope,pathName,body={})=>request(port,{token,pathName,body,headers:{'X-Gregal-Session':scope}});
  await Promise.all([scoped('alpha','/attach'),scoped('beta','/attach')]);
  await scoped('alpha','/cdp',{method:'Page.navigate',params:{url:'http://localhost:3000/alpha'}});
  assert.equal(alpha.debugger.calls.filter(c=>c==='Page.navigate').length,1);
  assert.equal(beta.debugger.calls.includes('Page.navigate'),false);
  await scoped('alpha','/release');
  assert.equal(alpha.debugger.attachedFlag,false);
  assert.equal(beta.debugger.attachedFlag,true);
  assert.ok(states.some(s=>s?.state==='show'&&s.session==='alpha'));
  assert.equal((await scoped('../other','/attach')).status,400);
});
