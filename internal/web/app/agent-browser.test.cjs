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
      this.classList = { toggle: (name, on) => { this[name] = on; } };
    }
    set id(value){if(elements.get(this._id)===this)elements.delete(this._id);this._id=value;if(value)elements.set(value,this);}
    get id(){return this._id;}
    remove(){if(elements.get(this.id)===this)elements.delete(this.id);}
    set innerHTML(value) {
      this._html = value;
      for (const [, id] of value.matchAll(/id="([^"]+)"/g)) elements.set(id, new Element('div', id));
      this.parts = {};
      for (const cls of ['ab-state', 'ab-view']) this.parts['.' + cls] = new Element('div');
      this.parts['header strong'] = new Element('strong');
      this.parts.small = new Element('small');
    }
    get innerHTML() { return this._html || ''; }
    querySelector(sel) { return this.parts[sel]; }
    append(child) { this.children.push(child); if (child.id) elements.set(child.id, child); }
    setAttribute(k, v) { this.attributes[k] = String(v); }
    addEventListener(name, fn) { this.events[name] = fn; }
  }
  const workbench = new Element('div', 'workbench');
  elements.set('workbench', workbench);
  const document = {
    head: { append() {} },
    createElement: tag => new Element(tag),
    getElementById: id => elements.get(id) || null,
    addEventListener: (name, fn) => listeners.set(name, fn),
  };
  const window = { gregalDesktop: desktop, gregalT: k => 'T:' + k };
  vm.runInNewContext(source, { window, document, console });
  return { elements, workbench, listeners, window };
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
  assert.equal(pane.querySelector('.ab-state').textContent, 'T:agentBrowser.active');
  view.events['did-navigate']({ url: 'http://localhost:3000/' });
  assert.equal(elements.get('agentBrowserUrl').textContent, 'http://localhost:3000/');
  elements.get('agentBrowserClose').onclick();
  assert.equal(pane.hidden, true);
  push('released');
  assert.equal(pane.hidden, true, 'releasing does not force the pane open');
  assert.equal(pane.querySelector('.ab-state').textContent, 'T:agentBrowser.released');
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
  assert.equal(elements.get('agentBrowserUrl').textContent,'http://localhost:3000/alpha');
  listeners.get('gregal:session')({detail:{id:'beta'}});
  const beta=elements.get('agentBrowserView');beta.getWebContentsId=()=>42;beta.events['dom-ready']();
  assert.notEqual(alpha,beta);
  assert.equal(alpha.hidden,true);
  assert.deepEqual(ready,[[41,'alpha'],[42,'beta']]);
  beta.events['did-navigate']({url:'http://localhost:4000/beta'});
  listeners.get('gregal:session')({detail:{id:'alpha'}});
  assert.equal(elements.get('agentBrowserView'),alpha);
  assert.equal(elements.get('agentBrowserUrl').textContent,'http://localhost:3000/alpha');
});
