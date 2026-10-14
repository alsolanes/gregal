const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const vm = require('node:vm');
const { pathToFileURL } = require('node:url');
const { createRequire } = require('node:module');
const { createMascotWindow, loadPosition, savePosition, positionPath, clampToArea, magnet, nextCorner, resizeAnchored, validDelta, SIZE, COLLAPSED_SIZE, MARGIN } = require('./mascot-window');
const { loadPreferences } = require('./preferences');

function tmp(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gregal-mascot-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}

// BrowserWindow fals: recorda les opcions, els missatges enviats i els
// handlers, i simula loadFile posant la URL file:// al marc principal.
function fakeElectronWindows() {
  const created = [];
  class FakeWindow {
    constructor(options) {
      this.options = options;
      this.sent = [];
      this.handlers = {};
      this.destroyed = false;
      this.focused = 0;
      this.pos = [options.x ?? 0, options.y ?? 0];
      const frame = { url: '' };
      this.webContents = {
        mainFrame: frame,
        send: (channel, ...data) => this.sent.push([channel, ...data]),
        on: (name, fn) => { this.handlers['wc:' + name] = fn; },
        setWindowOpenHandler: fn => { this.openHandler = fn; },
      };
      created.push(this);
    }
    on(name, fn) { this.handlers[name] = fn; }
    once(name, fn) { this.handlers[name] = fn; }
    loadFile(file) { this.webContents.mainFrame.url = pathToFileURL(file).href; this.handlers['wc:did-finish-load']?.(); }
    isDestroyed() { return this.destroyed; }
    getPosition() { return this.pos; }
    setPosition(x, y) { this.pos = [x, y]; this.moves = (this.moves || 0) + 1; }
    setBounds(b) { this.pos = [b.x, b.y]; this.size = [b.width, b.height]; }
    showInactive() { this.shown = true; }
    close() { this.destroyed = true; this.handlers.closed?.(); }
    isMinimized() { return false; }
    restore() {}
    show() {}
    focus() { this.focused++; }
  }
  return { FakeWindow, created };
}

// Últim estat enviat a la finestreta, sense els textos dels controls.
const lastState = w => {
  const [channel, data] = w.sent.at(-1);
  return [channel, { mood: data.mood, label: data.label }];
};

const screen = {
  getAllDisplays: () => [{ workArea: { x: 0, y: 0, width: 1920, height: 1040 } }],
  getPrimaryDisplay: () => ({ workArea: { x: 0, y: 0, width: 1920, height: 1040 } }),
};

test('floating window is small, frameless, on top and isolated; state forwarded only when valid', t => {
  const dir = tmp(t);
  const { FakeWindow, created } = fakeElectronWindows();
  let activated = 0;
  const htmlPath = path.join(__dirname, 'mascot.html');
  const m = createMascotWindow({ BrowserWindow: FakeWindow, screen, dir, htmlPath, preloadPath: path.join(__dirname, 'mascot-preload.js'), onActivate: () => activated++ });
  // Off by default: setState sense finestra no en crea cap.
  assert.equal(m.setState('working', 'busy'), true);
  assert.equal(created.length, 0);
  assert.equal(m.isOpen(), false);
  m.setEnabled(true);
  assert.equal(created.length, 1);
  const w = created[0];
  const o = w.options;
  assert.equal(o.width, SIZE); assert.equal(o.height, SIZE);
  assert.equal(o.frame, false); assert.equal(o.transparent, true); assert.equal(o.alwaysOnTop, true);
  assert.equal(o.skipTaskbar, true); assert.equal(o.resizable, false); assert.equal(o.show, false);
  assert.equal(o.webPreferences.contextIsolation, true);
  assert.equal(o.webPreferences.nodeIntegration, false);
  assert.equal(o.webPreferences.sandbox, true);
  assert.equal(o.webPreferences.webviewTag, false);
  // Posició per defecte: a baix a la dreta de la pantalla principal.
  assert.deepEqual([o.x, o.y], [1920 - SIZE - 24, 1040 - SIZE - 24]);
  // En carregar rep l'últim estat conegut.
  assert.deepEqual(lastState(w), ['gregal:mascot-state', { mood: 'working', label: 'busy' }]);
  assert.deepEqual(w.openHandler(), { action: 'deny' });
  let prevented = false;
  w.handlers['wc:will-navigate']({ preventDefault: () => { prevented = true; } }, 'https://example.org/');
  assert.equal(prevented, true);
  w.handlers['ready-to-show']();
  assert.equal(w.shown, true);
  // Ànims vàlids passen; la resta es descarta.
  assert.equal(m.setState('done', 'x'.repeat(500)), true);
  assert.deepEqual(lastState(w), ['gregal:mascot-state', { mood: 'done', label: 'x'.repeat(200) }]);
  const before = w.sent.length;
  assert.equal(m.setState('<script>', 'x'), false);
  assert.equal(m.setState(42), false);
  assert.equal(w.sent.length, before);
  // Només la pròpia finestra (marc principal, mascot.html) pot activar.
  assert.equal(m.isSender({ sender: w.webContents, senderFrame: w.webContents.mainFrame }), true);
  assert.equal(m.isSender({ sender: {}, senderFrame: w.webContents.mainFrame }), false);
  assert.equal(m.isSender({ sender: w.webContents, senderFrame: { url: w.webContents.mainFrame.url } }), false);
  m.activate();
  assert.equal(activated, 1);
  // Tornar a activar no en crea una altra; apagar la tanca i desa la posició.
  m.setEnabled(true);
  assert.equal(created.length, 1);
  w.pos = [300, 200];
  m.setEnabled(false);
  assert.equal(w.destroyed, true);
  assert.equal(m.isOpen(), false);
  assert.equal(m.isSender({ sender: w.webContents, senderFrame: w.webContents.mainFrame }), false);
  assert.deepEqual(loadPosition(dir, screen), { x: 300, y: 200 });
  m.setEnabled(true);
  assert.deepEqual([created[1].options.x, created[1].options.y], [300, 200]);
});

test('remembered position is discarded when off-screen or corrupt', t => {
  const dir = tmp(t);
  assert.ok(savePosition(dir, { x: 5000, y: 5000 }));
  assert.deepEqual(loadPosition(dir, screen), { x: 1920 - SIZE - 24, y: 1040 - SIZE - 24 });
  fs.writeFileSync(positionPath(dir), '{bad');
  assert.deepEqual(loadPosition(dir, screen), { x: 1920 - SIZE - 24, y: 1040 - SIZE - 24 });
  assert.deepEqual(loadPosition(dir, null), {});
  assert.equal(savePosition(dir, { x: NaN, y: 1 }), false);
});

test('main process: mascot off by default, pref toggles it and IPC only from trusted pages', async t => {
  const dir = tmp(t);
  const listeners = new Map();
  const handles = new Map();
  const opened = [];
  const { FakeWindow, created } = fakeElectronWindows();
  const electron = {
    shell: { openPath: async p => { opened.push(p); return ''; }, openExternal: async () => {} },
    app: { getPath: () => dir, whenReady: () => ({ then() {} }), on() {} },
    ipcMain: { on: (name, fn) => listeners.set(name, fn), handle: (name, fn) => handles.set(name, fn) },
    dialog: {}, nativeTheme: { shouldUseDarkColors: false }, screen, BrowserWindow: FakeWindow,
  };
  const localRequire = createRequire(path.join(__dirname, 'main.js'));
  const fetched = [];
  let workspace = dir;
  const context = {
    require: name => name === 'electron' ? electron : localRequire(name),
    process: { env: {}, platform: process.platform },
    __dirname, URL, console, setTimeout, clearTimeout, AbortController,
    fetch: async (url, opts) => { fetched.push([url, opts]); return { ok: true, json: async () => ({ cwd: workspace }) }; },
  };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'main.js'), 'utf8') +
    '\nthis.testWire = wireIPC; this.testSetMain = w => { mainWin = w; };', context);
  const frame = { url: 'http://127.0.0.1:8097/' };
  const main = new FakeWindow({});
  main.webContents.mainFrame = frame;
  created.length = 0;
  context.testWire(() => main);
  context.testSetMain(main);
  const trusted = { sender: main.webContents, senderFrame: frame };
  const foreign = [{ sender: {}, senderFrame: frame }, { sender: main.webContents, senderFrame: { url: frame.url } }];
  // Sense preferència desada: apagada.
  assert.equal(loadPreferences(dir).gregal_desktop_mascot, undefined);
  listeners.get('gregal:mascot-state')(trusted, 'working', 'Busy');
  assert.equal(created.length, 0);
  // Pàgines o marcs estranys no la poden encendre.
  for (const ev of foreign) listeners.get('gregal:preference-save')(ev, 'gregal_desktop_mascot', '1');
  assert.equal(created.length, 0);
  listeners.get('gregal:preference-save')(trusted, 'gregal_desktop_mascot', '1');
  assert.equal(created.length, 1);
  assert.equal(loadPreferences(dir).gregal_desktop_mascot, '1');
  const w = created[0];
  assert.deepEqual(lastState(w), ['gregal:mascot-state', { mood: 'working', label: 'Busy' }]);
  // L'estat només arriba des de la pàgina de confiança.
  for (const ev of foreign) listeners.get('gregal:mascot-state')(ev, 'error', 'nope');
  frame.url = 'https://untrusted.example.org/';
  listeners.get('gregal:mascot-state')(trusted, 'error', 'nope');
  frame.url = 'http://127.0.0.1:8097/';
  assert.deepEqual(lastState(w), ['gregal:mascot-state', { mood: 'working', label: 'Busy' }]);
  listeners.get('gregal:mascot-state')(trusted, 'waiting', 'Approve');
  assert.deepEqual(lastState(w), ['gregal:mascot-state', { mood: 'waiting', label: 'Approve' }]);
  // Activar: la pàgina principal no pot; la finestra de la mascota sí.
  listeners.get('gregal:mascot-activate')(trusted);
  assert.equal(main.focused, 0);
  listeners.get('gregal:mascot-activate')({ sender: w.webContents, senderFrame: w.webContents.mainFrame });
  assert.equal(main.focused, 1);
  // Arrossegar, plegar i enganxar: només des de la finestra de la mascota,
  // i amb desplaçaments vàlids.
  const own = { sender: w.webContents, senderFrame: w.webContents.mainFrame };
  const start = [...w.pos];
  listeners.get('gregal:mascot-drag')(trusted, 'start', 0, 0);
  listeners.get('gregal:mascot-drag')(trusted, 'move', -100, -100);
  assert.deepEqual(w.pos, start);
  listeners.get('gregal:mascot-drag')(own, 'start', 0, 0);
  listeners.get('gregal:mascot-drag')(own, 'move', -100, -50);
  assert.deepEqual(w.pos, [start[0] - 100, start[1] - 50]);
  for (const bad of [[NaN, 0], [1.5, 0], ['10', 0], [1e9, 0], [0, Infinity]]) listeners.get('gregal:mascot-drag')(own, 'move', ...bad);
  assert.deepEqual(w.pos, [start[0] - 100, start[1] - 50]);
  listeners.get('gregal:mascot-drag')(own, 'move', 9999, 9998);
  assert.deepEqual(w.pos, [1920 - SIZE, 1040 - SIZE], 'clamped to the work area');
  listeners.get('gregal:mascot-drag')(own, 'end', 9999, 9998);
  assert.deepEqual(w.pos, [1920 - SIZE - MARGIN, 1040 - SIZE - MARGIN], 'magnet to the margin');
  listeners.get('gregal:mascot-collapse')(trusted, true);
  listeners.get('gregal:mascot-collapse')(own, 'yes');
  assert.deepEqual(w.size, [SIZE, SIZE]);
  listeners.get('gregal:mascot-collapse')(own, true);
  assert.deepEqual(w.size, [COLLAPSED_SIZE, COLLAPSED_SIZE]);
  assert.equal(w.sent.at(-1)[1].collapsed, true);
  listeners.get('gregal:mascot-collapse')(own, false);
  assert.deepEqual(w.size, [SIZE, SIZE]);
  listeners.get('gregal:mascot-snap')(trusted);
  assert.deepEqual(w.pos, [1920 - SIZE - MARGIN, 1040 - SIZE - MARGIN]);
  listeners.get('gregal:mascot-snap')(own);
  assert.deepEqual(w.pos, [MARGIN, 1040 - SIZE - MARGIN], 'next corner clockwise');
  // Obrir la carpeta: només la pàgina de confiança, i la ruta la dona el
  // backend (no la pàgina).
  assert.equal(await handles.get('gregal:open-workspace')(own, 'default'), false);
  assert.equal(await handles.get('gregal:open-workspace')(trusted, '../../x'), true);
  assert.deepEqual(opened, [dir]);
  assert.equal(fetched.at(-1)[1].headers['X-Gregal-Session'], 'default', 'bad session ids fall back to default');
  assert.match(fetched.at(-1)[0], /\/api\/state$/);
  workspace = 'relative/dir';
  assert.equal(await handles.get('gregal:open-workspace')(trusted, 'abc'), false);
  workspace = path.join(dir, 'missing');
  assert.equal(await handles.get('gregal:open-workspace')(trusted, 'abc'), false);
  assert.equal(opened.length, 1);
  // Amagar des de la finestreta: la pàgina no pot; ella sí, i apaga la
  // preferència i ho diu a la pàgina.
  listeners.get('gregal:mascot-hide')(trusted);
  assert.equal(w.destroyed, false);
  listeners.get('gregal:mascot-hide')(own);
  assert.equal(w.destroyed, true);
  assert.equal(loadPreferences(dir).gregal_desktop_mascot, '0');
  assert.deepEqual(main.sent.at(-1), ['gregal:preference-changed', 'gregal_desktop_mascot', '0']);
  // Tornar-la a encendre; apagar-la des de Preferències la tanca.
  listeners.get('gregal:preference-save')(trusted, 'gregal_desktop_mascot', '1');
  const w2 = created.at(-1);
  assert.notEqual(w2, w);
  assert.deepEqual([w2.options.x, w2.options.y], [MARGIN, 1040 - SIZE - MARGIN], 'position remembered');
  listeners.get('gregal:preference-save')(trusted, 'gregal_desktop_mascot', '0');
  assert.equal(w2.destroyed, true);
  assert.equal(loadPreferences(dir).gregal_desktop_mascot, '0');
});

test('geometry helpers: clamp, magnet, corners and anchored resize', () => {
  const area = { x: 0, y: 0, width: 1920, height: 1040 };
  assert.deepEqual(clampToArea({ x: -50, y: 2000 }, SIZE, area), { x: 0, y: 1040 - SIZE });
  assert.deepEqual(clampToArea({ x: 10, y: 10 }, SIZE, null), { x: 10, y: 10 });
  // Segona pantalla a l'esquerra, amb coordenades negatives.
  const left = { x: -1280, y: 0, width: 1280, height: 984 };
  assert.deepEqual(clampToArea({ x: -2000, y: -5 }, SIZE, left), { x: -1280, y: 0 });
  assert.deepEqual(magnet({ x: 10, y: 500 }, SIZE, area), { x: MARGIN, y: 500 });
  assert.deepEqual(magnet({ x: 900, y: 500 }, SIZE, area), { x: 900, y: 500 });
  assert.deepEqual(magnet({ x: 1920 - SIZE - 5, y: 1040 - SIZE - 5 }, SIZE, area), { x: 1920 - SIZE - MARGIN, y: 1040 - SIZE - MARGIN });
  const tl = { x: MARGIN, y: MARGIN }, tr = { x: 1920 - SIZE - MARGIN, y: MARGIN };
  const br = { x: 1920 - SIZE - MARGIN, y: 1040 - SIZE - MARGIN }, bl = { x: MARGIN, y: 1040 - SIZE - MARGIN };
  assert.deepEqual(nextCorner({ x: 1500, y: 100 }, SIZE, area), tr, 'nearest');
  assert.deepEqual(nextCorner(tr, SIZE, area), br);
  assert.deepEqual(nextCorner(br, SIZE, area), bl);
  assert.deepEqual(nextCorner(bl, SIZE, area), tl);
  assert.deepEqual(nextCorner(tl, SIZE, area), tr);
  // Plegar a baix a la dreta manté les vores de baix i de la dreta.
  assert.deepEqual(resizeAnchored(br, SIZE, COLLAPSED_SIZE, area), { x: 1920 - COLLAPSED_SIZE - MARGIN, y: 1040 - COLLAPSED_SIZE - MARGIN });
  assert.deepEqual(resizeAnchored(tl, SIZE, COLLAPSED_SIZE, area), tl);
  // Desplegar a tocar de la vora no surt de la pantalla.
  assert.deepEqual(resizeAnchored({ x: 0, y: 1040 - COLLAPSED_SIZE }, COLLAPSED_SIZE, SIZE, area), { x: 0, y: 1040 - SIZE });
  assert.equal(validDelta(3, -4), true);
  for (const [a, b] of [[NaN, 0], [0.5, 0], ['1', 0], [20001, 0], [0, -20001], [undefined, 0]]) assert.equal(validDelta(a, b), false);
});

test('collapsed state and position persist; display changes keep it on screen', t => {
  const dir = tmp(t);
  const { FakeWindow, created } = fakeElectronWindows();
  const listeners = {};
  let area = { x: 0, y: 0, width: 1920, height: 1040 };
  const scr = {
    getAllDisplays: () => [{ workArea: area }],
    getPrimaryDisplay: () => ({ workArea: area }),
    getDisplayNearestPoint: () => ({ workArea: area }),
    on: (ev, fn) => { listeners[ev] = fn; },
    removeListener: ev => { delete listeners[ev]; },
  };
  let hidden = 0;
  const m = createMascotWindow({ BrowserWindow: FakeWindow, screen: scr, dir, htmlPath: path.join(__dirname, 'mascot.html'), preloadPath: 'x', onHide: () => hidden++, lang: () => 'ca' });
  m.setEnabled(true);
  const w = created[0];
  assert.equal(w.sent.at(-1)[1].strings.collapse, 'Plega (Esc)');
  assert.equal(m.drag('move', 1, 1), false, 'move without start');
  assert.equal(m.drag('bogus', 0, 0), false);
  assert.equal(m.setCollapsed('1'), false);
  assert.equal(m.setCollapsed(true), true);
  assert.equal(m.isCollapsed(), true);
  assert.deepEqual(JSON.parse(fs.readFileSync(positionPath(dir), 'utf8')), { x: 1920 - COLLAPSED_SIZE - MARGIN, y: 1040 - COLLAPSED_SIZE - MARGIN, collapsed: true });
  // Es torna a obrir plegada i al mateix lloc.
  m.setEnabled(false);
  assert.equal(Object.keys(listeners).length, 0, 'display listeners removed on close');
  m.setEnabled(true);
  const w2 = created[1];
  assert.deepEqual([w2.options.width, w2.options.height, w2.options.x, w2.options.y], [COLLAPSED_SIZE, COLLAPSED_SIZE, 1920 - COLLAPSED_SIZE - MARGIN, 1040 - COLLAPSED_SIZE - MARGIN]);
  assert.equal(w2.sent.at(-1)[1].collapsed, true);
  // La pantalla s'encongeix (p. ex. es desconnecta un monitor gran).
  area = { x: 0, y: 0, width: 1280, height: 720 };
  listeners['display-metrics-changed']();
  assert.deepEqual(w2.pos, [1280 - COLLAPSED_SIZE, 720 - COLLAPSED_SIZE]);
  assert.equal(m.hide(), true);
  assert.equal(hidden, 1);
  assert.equal(w2.destroyed, true);
  assert.equal(m.hide(), false);
  // Una posició desada fora de pantalla es descarta; una de mig fora, s'hi encaixa.
  savePosition(dir, { x: 1200, y: 100, collapsed: false });
  assert.deepEqual(loadPosition(dir, scr), { x: 1280 - SIZE, y: 100 });
});

test('mascot page is local-only with a strict CSP and ships in the package', () => {
  const html = fs.readFileSync(path.join(__dirname, 'mascot.html'), 'utf8');
  assert.match(html, /Content-Security-Policy" content="default-src 'none'; script-src 'self'; style-src 'self'/);
  assert.doesNotMatch(html, /https?:\/\//);
  assert.doesNotMatch(html, /<script>(?!<)/, 'no inline scripts');
  const css = fs.readFileSync(path.join(__dirname, 'mascot.css'), 'utf8');
  assert.match(css, /prefers-reduced-motion: no-preference/);
  // El penell: dibuix accessible, fletxa que gira sobre el pivot, fons
  // transparent i vora arrossegable amb el disc clicable.
  assert.match(html, /<svg class="mascot-svg"[^>]*role="img"/);
  assert.match(html, /class="v-arrow"/);
  assert.match(css, /\.v-arrow \{[^}]*transform-origin:32px 20px/);
  assert.match(css, /html, body \{[^}]*background:transparent/);
  // Tot el penell s'arrossega amb pointer events (mascot-view.js): cap zona
  // -webkit-app-region:drag, que es menjaria el clic.
  assert.doesNotMatch(css, /-webkit-app-region:\s*drag/);
  const view = fs.readFileSync(path.join(__dirname, 'mascot-view.js'), 'utf8');
  for (const ev of ['pointerdown', 'pointermove', 'pointerup', 'pointercancel']) assert.match(view, new RegExp("'" + ev + "'"), ev);
  assert.match(view, /setPointerCapture/);
  assert.match(view, /'Escape'/);
  // Controls en passar-hi per sobre: plegar, enganxar a un racó i amagar.
  for (const id of ['mascotCollapse', 'mascotSnap', 'mascotHide']) assert.match(html, new RegExp('<button type="button" class="m-ctl" id="' + id + '"'), id);
  assert.match(css, /\.mascot-stage:hover \.mascot-ctrls/);
  assert.match(css, /\[data-collapsed="1"\] \.mascot-btn \{[^}]*width:32px/);
  const pre = fs.readFileSync(path.join(__dirname, 'mascot-preload.js'), 'utf8');
  for (const ch of ['gregal:mascot-drag', 'gregal:mascot-collapse', 'gregal:mascot-snap', 'gregal:mascot-hide']) assert.match(pre, new RegExp("'" + ch + "'"), ch);
  const pkg = JSON.parse(fs.readFileSync(path.join(__dirname, 'package.json'), 'utf8'));
  for (const f of ['mascot-window.js', 'mascot-preload.js', 'mascot.html', 'mascot-view.js', 'mascot.css']) assert.ok(pkg.build.files.includes(f), f);
  assert.ok(require('./preferences').allowed('gregal_desktop_mascot'));
});

// La vista: un clic curt activa (o desplega el puntet), mantenir i moure
// arrossega sense activar, i Esc plega.
function loadView() {
  const mk = () => {
    const n = { listeners: {}, dataset: {}, attrs: {}, title: '', classes: new Set() };
    n.addEventListener = (ev, fn) => { n.listeners[ev] = fn; };
    n.setAttribute = (k, v) => { n.attrs[k] = v; };
    n.querySelector = () => null;
    n.closest = () => null;
    n.classList = { add: c => n.classes.add(c), remove: c => n.classes.delete(c) };
    n.setPointerCapture = () => {}; n.releasePointerCapture = () => {};
    return n;
  };
  const stage = mk(), btn = mk(), collapse = mk(), snap = mk(), hide = mk();
  const ids = { mascot: btn, mascotCollapse: collapse, mascotSnap: snap, mascotHide: hide };
  const calls = [];
  let onState = null;
  const doc = mk();
  doc.querySelector = sel => (sel === '.mascot-stage' ? stage : null);
  doc.getElementById = id => ids[id] || null;
  const bridge = {
    onState: cb => { onState = cb; },
    activate: () => calls.push(['activate']),
    drag: (...a) => calls.push(['drag', ...a]),
    setCollapsed: on => calls.push(['collapse', on]),
    snap: () => calls.push(['snap']),
    hide: () => calls.push(['hide']),
  };
  const frames = [];
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'mascot-view.js'), 'utf8'), {
    window: { gregalMascot: bridge }, document: doc, Math, Date,
    requestAnimationFrame: fn => { frames.push(fn); return frames.length; }, cancelAnimationFrame: () => {},
  });
  const flush = () => { while (frames.length) frames.shift()(); };
  return { stage, btn, collapse, snap, hide, doc, calls, flush, state: s => onState(s) };
}

test('mascot view: short click activates, hold-and-move drags, controls and Esc', () => {
  const v = loadView();
  const ev = (x, y, extra = {}) => ({ button: 0, pointerId: 1, screenX: x, screenY: y, target: v.btn, ...extra });
  // Clic curt (moviment de menys de 4 px): activa, no arrossega.
  v.stage.listeners.pointerdown(ev(100, 100));
  v.stage.listeners.pointermove(ev(102, 101));
  v.stage.listeners.pointerup(ev(102, 101));
  v.btn.listeners.click({ preventDefault() {} });
  assert.deepEqual(v.calls, [['activate']]);
  v.calls.length = 0;
  // Mantenir i moure: start, move per fotograma, end; el clic posterior no activa.
  v.stage.listeners.pointerdown(ev(100, 100));
  v.stage.listeners.pointermove(ev(110, 100));
  v.stage.listeners.pointermove(ev(150, 80));
  v.flush();
  v.stage.listeners.pointerup(ev(160, 70));
  v.btn.listeners.click({ preventDefault() {} });
  assert.deepEqual(v.calls, [['drag', 'start', 0, 0], ['drag', 'move', 50, -20], ['drag', 'end', 60, -30]]);
  v.calls.length = 0;
  // Els controls no comencen cap arrossegament.
  v.stage.listeners.pointerdown(ev(0, 0, { target: { closest: sel => (sel === '.mascot-ctrls' ? {} : null) } }));
  v.stage.listeners.pointermove(ev(50, 50));
  assert.deepEqual(v.calls, []);
  v.collapse.listeners.click({ stopPropagation() {} });
  v.snap.listeners.click({ stopPropagation() {} });
  v.hide.listeners.click({ stopPropagation() {} });
  assert.deepEqual(v.calls, [['collapse', true], ['snap'], ['hide']]);
  v.calls.length = 0;
  // Esc plega; plegat, el clic desplega i Esc ja no fa res.
  v.doc.listeners.keydown({ key: 'Escape', preventDefault() {} });
  assert.deepEqual(v.calls, [['collapse', true]]);
  v.state({ mood: 'working', label: 'Busy', collapsed: true, strings: { expand: 'Expand', drag: 'Drag' } });
  assert.equal(v.stage.dataset.collapsed, '1');
  assert.equal(v.stage.dataset.mood, 'working');
  assert.equal(v.btn.title, 'Busy — Expand');
  v.calls.length = 0;
  v.btn.listeners.click({ preventDefault() {} });
  v.doc.listeners.keydown({ key: 'Escape', preventDefault() {} });
  assert.deepEqual(v.calls, [['collapse', false]]);
});
