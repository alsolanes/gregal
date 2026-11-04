const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, 'agent-browser.js'), 'utf8').replace(/^export /gm, '');
const indexSource = fs.readFileSync(path.join(__dirname, '../index.html'), 'utf8');

function fixture(desktop) {
  const elements = new Map();
  const listeners = new Map();
  class Element {
    constructor(tag, id = '') {
      this.tagName = tag.toUpperCase();
      this._id = id;
      this.hidden = false;
      this.children = [];
      this.attributes = {};
      this.events = {};
      this.textContent = '';
      this.title = '';
      this.classes = new Set();
      this.classList = { toggle: (name, on) => { if (on) this.classes.add(name); else this.classes.delete(name); }, add: n => this.classes.add(n), remove: n => this.classes.delete(n) };
      this.value = '';
    }
    set id(value){if(elements.get(this._id)===this)elements.delete(this._id);this._id=value;if(value)elements.set(value,this);}
    get id(){return this._id;}
    remove(){if(elements.get(this.id)===this)elements.delete(this.id);}
    set innerHTML(value) {
      this._html = value;
      for (const [, id] of value.matchAll(/id="([^"]+)"/g)) elements.set(id, new Element('div', id));
      this.parts = {};
    }
    get innerHTML() { return this._html || ''; }
    // Un fill estable per selector: n'hi ha prou per llegir-ne el text.
    querySelector(sel) { this.parts = this.parts || {}; return this.parts[sel] || (this.parts[sel] = new Element('div')); }
    append(child) { this.children.push(child); if (child.id) elements.set(child.id, child); }
    setAttribute(k, v) { this.attributes[k] = String(v); }
    addEventListener(name, fn) { this.events[name] = fn; }
    loadURL(u) { this.loaded = u; }
    canGoBack() { return !!this.back; }
    canGoForward() { return false; }
    blur() {}
    getBoundingClientRect() { return { left: 0, right: 1400, width: 1400 }; }
    setZoomFactor(z) { this.zoom = z; }
  }
  const workbench = new Element('div', 'workbench');
  elements.set('workbench', workbench);
  const document = {
    activeElement: null,
    hasFocus: () => true,
    head: { append() {} },
    createElement: tag => new Element(tag),
    getElementById: id => elements.get(id) || null,
    addEventListener: (name, fn) => listeners.set(name, fn),
    documentElement: { style: { props: {}, setProperty(k, v) { this.props[k] = v; }, removeProperty(k) { delete this.props[k]; } } },
    body: { classList: { add() {}, remove() {} } },
  };
  const store = new Map();
  const localStorage = { getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => store.set(k, String(v)), removeItem: k => store.delete(k) };
  const window = { gregalDesktop: desktop, gregalT: k => 'T:' + k, innerWidth: 1400, addEventListener() {} };
  const opened = [];
  window.open = (...a) => opened.push(a);
  vm.runInNewContext(source, { window, document, console, localStorage, URL, encodeURIComponent });
  return { elements, workbench, listeners, window, document, localStorage, opened };
}

test('agent browser stays out of the way outside the desktop app', () => {
  const { workbench } = fixture(undefined);
  assert.equal(workbench.children.length, 0);
});

test('agent browser pane opens on demand and registers its webview', () => {
  let push;
  const ready = [];
  const desktop = { onAgentBrowser: cb => { push = cb; return () => {}; }, agentBrowserReady: id => ready.push(id) };
  const { elements, workbench } = fixture(desktop);
  assert.equal(workbench.children.length, 0, 'nothing is built until the agent needs it');
  push('show');
  const pane = elements.get('agentBrowser');
  assert.ok(pane && !pane.hidden);
  const view = elements.get('agentBrowserView');
  assert.equal(view.tagName, 'WEBVIEW');
  assert.equal(view.attributes.partition, 'persist:gregal-browser');
  assert.equal(view.attributes.src, 'about:blank');
  view.getWebContentsId = () => 42;
  view.events['dom-ready']();
  view.events['dom-ready']();
  assert.deepEqual(ready, [42], 'the same guest is reported once');
  assert.equal(pane.querySelector('.ab-state').querySelector('span').textContent, 'T:agentBrowser.active');
  view.events['did-navigate']({ url: 'http://localhost:3000/' });
  assert.equal(elements.get('agentBrowserUrl').value, 'http://localhost:3000/');
  elements.get('agentBrowserClose').onclick();
  assert.equal(pane.hidden, true);
  push('released');
  assert.equal(pane.hidden, true, 'releasing does not force the pane open');
  assert.equal(pane.querySelector('.ab-state').querySelector('span').textContent, 'T:agentBrowser.released');
  push('show');
  assert.equal(pane.hidden, false);
  assert.equal(workbench.children.length, 1, 'one pane only');
});

test('tool cards only render screenshot data URLs', () => {
  const line = indexSource.split('\n').find(l => l.startsWith('const toolShotOK'));
  assert.ok(line, 'toolShotOK must exist in index.html');
  const toolShotOK = vm.runInNewContext(line.replace('const toolShotOK =', '(').trim().replace(/;$/, '') + ')');
  assert.equal(toolShotOK('data:image/jpeg;base64,/9j/4AAQSkZJRg=='), true);
  assert.equal(toolShotOK('data:image/png;base64,iVBORw0KGgo='), true);
  for (const bad of ['https://evil.example/x.png', 'data:image/svg+xml;base64,PHN2Zz4=', 'data:text/html;base64,PGI+', 'data:image/png;base64,"onerror=', null, undefined]) {
    assert.equal(toolShotOK(bad), false, String(bad));
  }
  assert.match(indexSource, /toolCardDone\(lastDoneCard, d\.name, d\.output, d\.image\)/);
});


test('background browser navigation stays with its owning session', () => {
  let push;const ready=[];
  const {elements,listeners}=fixture({onAgentBrowser:cb=>{push=cb;},agentBrowserReady:(id,scope)=>ready.push([id,scope])});
  listeners.get('gregal:session')({detail:{id:'alpha'}});
  push({state:'active',session:'alpha'});
  const alpha=elements.get('agentBrowserView');alpha.getWebContentsId=()=>41;alpha.events['dom-ready']();
  alpha.events['did-navigate']({url:'http://localhost:3000/alpha'});
  push({state:'active',session:'beta'});
  assert.equal(elements.get('agentBrowserView'),alpha);
  assert.equal(elements.get('agentBrowserUrl').value,'http://localhost:3000/alpha');
  listeners.get('gregal:session')({detail:{id:'beta'}});
  const beta=elements.get('agentBrowserView');beta.getWebContentsId=()=>42;beta.events['dom-ready']();
  assert.notEqual(alpha,beta);
  assert.equal(alpha.hidden,true);
  assert.deepEqual(ready,[[41,'alpha'],[42,'beta']]);
  beta.events['did-navigate']({url:'http://localhost:4000/beta'});
  listeners.get('gregal:session')({detail:{id:'alpha'}});
  assert.equal(elements.get('agentBrowserView'),alpha);
  assert.equal(elements.get('agentBrowserUrl').value,'http://localhost:3000/alpha');
});

test('the pane width stays between its minimum and what leaves room for the chat', () => {
  const ctx = { window: {}, document: { getElementById: () => null } };
  vm.runInNewContext(source + '\nthis.clamp = clampBrowserWidth; this.zoom = nextZoom;', ctx);
  assert.equal(ctx.clamp(100, 1400), 340);
  assert.equal(ctx.clamp(700, 1400), 700);
  assert.equal(ctx.clamp(1300, 1400), 1020);
  assert.equal(ctx.clamp(900, 600), 340);
  assert.equal(ctx.zoom(1, 1), 1.1);
  assert.equal(ctx.zoom(1, -1), 0.9);
  assert.equal(ctx.zoom(2, 1), 2);
  assert.equal(ctx.zoom(0.5, -1), 0.5);
  assert.equal(ctx.zoom(1.5, 0), 1);
});

function openPane() {
  let push;
  const answers = [], notes = [];
  const desktop = { onAgentBrowser: cb => { push = cb; }, agentBrowserReady() {}, agentBrowserHandoff: (id, done) => answers.push([id, done]), notify: (t, b) => notes.push([t, b]) };
  const fx = fixture(desktop);
  push('active');
  return { ...fx, push: v => push(v), answers, notes, pane: fx.elements.get('agentBrowser') };
}
const badgeText = pane => pane.querySelector('.ab-state').querySelector('span').textContent;

test('handoff shows what the user must do and answers with Done or Cancel', () => {
  const { push, pane, elements, answers, notes, document } = openPane();
  const banner = elements.get('agentBrowserHandoff');
  assert.equal(banner.hidden, true);
  // Minimitzat: el «passa-m'ho» el torna a la vista.
  elements.get('agentBrowserMin').onclick();
  assert.equal(pane.classes.has('min'), true);
  document.hasFocus = () => false;
  push({ state: 'handoff', id: 'abcdef0123456789', message: 'Inicia sessió a GitHub' });
  assert.equal(pane.classes.has('min'), false, 'a handoff restores a minimized pane');
  assert.equal(banner.hidden, false);
  assert.equal(elements.get('agentBrowserHandoffMsg').textContent, 'Inicia sessió a GitHub');
  assert.equal(badgeText(pane), 'T:agentBrowser.handoff');
  assert.equal(pane.classes.has('handoff'), true);
  assert.deepEqual(notes, [['T:agentBrowser.handoffTitle', 'Inicia sessió a GitHub']], 'unfocused window gets a system notification');
  elements.get('agentBrowserHandoffDone').onclick();
  assert.deepEqual(answers, [['abcdef0123456789', true]]);
  assert.equal(banner.hidden, true);
  assert.equal(badgeText(pane), 'T:agentBrowser.active');
  // Cancel·lar, i retirar-lo des del backend (temps esgotat, torn aturat).
  push({ state: 'handoff', id: '1111111111111111', message: 'CAPTCHA' });
  elements.get('agentBrowserHandoffCancel').onclick();
  assert.deepEqual(answers.at(-1), ['1111111111111111', false]);
  push({ state: 'handoff', id: '2222222222222222', message: 'Tria compte' });
  push({ state: 'handoff-end', id: '9999999999999999' });
  assert.equal(banner.hidden, false, 'another handoff ending leaves this one up');
  push({ state: 'handoff-end', id: '2222222222222222', reason: 'timeout' });
  assert.equal(banner.hidden, true);
});

test('the pane minimizes to a pill, expands to the whole window and restores', () => {
  const { pane, elements } = openPane();
  elements.get('agentBrowserMax').onclick();
  assert.equal(pane.classes.has('max'), true);
  assert.equal(elements.get('agentBrowserMax').title, 'T:agentBrowser.restore');
  elements.get('agentBrowserMax').onclick();
  assert.equal(pane.classes.has('max'), false);
  elements.get('agentBrowserMin').onclick();
  assert.equal(pane.classes.has('min'), true);
  assert.equal(pane.hidden, false, 'minimized is still there (the agent keeps working)');
  elements.get('agentBrowserRestore').onclick();
  assert.equal(pane.classes.has('min'), false);
  elements.get('agentBrowserMax').onclick();
  elements.get('agentBrowserClose').onclick();
  assert.equal(pane.hidden, true);
  assert.equal(pane.classes.has('max'), false, 'closing forgets the size');
});

test('the address bar navigates for real and searches the rest', () => {
  const { elements, opened } = openPane();
  const url = elements.get('agentBrowserUrl'), view = elements.get('agentBrowserView');
  url.value = 'localhost:3000/app';
  url.events.keydown({ key: 'Enter', preventDefault() {} });
  assert.equal(view.loaded, 'http://localhost:3000/app');
  url.value = 'com fer una truita';
  url.events.keydown({ key: 'Enter', preventDefault() {} });
  assert.equal(view.loaded, 'https://duckduckgo.com/?q=com%20fer%20una%20truita');
  view.back = true;
  view.events['did-navigate']({ url: 'https://example.org/' });
  assert.equal(elements.get('agentBrowserBack').disabled, false);
  assert.equal(elements.get('agentBrowserForward').disabled, true);
  view.events['did-start-loading']();
  assert.equal(elements.get('agentBrowserReload').title, 'T:agentBrowser.stop');
  assert.equal(elements.get('agentBrowserProgress').classes.has('loading'), true);
  view.events['did-stop-loading']();
  assert.equal(elements.get('agentBrowserReload').title, 'T:agentBrowser.reload');
  elements.get('agentBrowserExternal').onclick();
  assert.equal(opened[0][0], 'https://example.org/');
});

test('addresses and states', () => {
  const ctx = { window: {}, document: { getElementById: () => null }, URL, encodeURIComponent };
  vm.runInNewContext(source + '\nthis.norm = normalizeAddress; this.state = agentBrowserState;', ctx);
  assert.equal(ctx.norm('example.org'), 'https://example.org/');
  assert.equal(ctx.norm('http://localhost:8080/x'), 'http://localhost:8080/x');
  assert.equal(ctx.norm('127.0.0.1:5173'), 'http://127.0.0.1:5173/');
  assert.equal(ctx.norm('gregal'), 'https://duckduckgo.com/?q=gregal');
  assert.equal(ctx.norm('https://u:p@example.org'), '');
  assert.equal(ctx.norm('   '), '');
  assert.equal(ctx.state('handoff', 'active'), 'handoff');
  assert.equal(ctx.state('handoff-end', 'handoff'), 'active');
  assert.equal(ctx.state('handoff-end', 'released'), 'released');
  assert.equal(ctx.state('show', 'released'), 'released');
});
