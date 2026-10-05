const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

class Element {
  constructor(tag) {
    this.tagName = tag.toUpperCase(); this.children = []; this.dataset = {}; this.attributes = {};
    this.listeners = {}; this.hidden = false; this.disabled = false; this.textContent = '';
    this.style = { setProperty() {} };
  }
  set className(value) { this._className = value; }
  get className() { return this._className || ''; }
  setAttribute(name, value) { this.attributes[name] = String(value); if (name === 'class') this.className = String(value); }
  appendChild(node) { if (node.parent) node.parent.children = node.parent.children.filter(child => child !== node); this.children.push(node); node.parent = this; return node; }
  append(...nodes) { nodes.forEach(node => this.appendChild(node)); }
  replaceChildren(...nodes) { this.children = []; nodes.forEach(node => this.appendChild(node)); }
  addEventListener(name, fn) { (this.listeners[name] ||= []).push(fn); }
  matches(selector) {
    if (selector[0] === '.') return this.className.split(/\s+/).includes(selector.slice(1));
    const data = selector.match(/^\[data-([a-z-]+)\]$/);
    if (data) {
      const key = data[1].replace(/-([a-z])/g, (_, c) => c.toUpperCase());
      return Object.prototype.hasOwnProperty.call(this.dataset, key);
    }
    if (selector[0] === '#') return this.id === selector.slice(1);
    return false;
  }
  querySelector(selector) {
    for (const child of this.children) {
      if (child.matches(selector)) return child;
      const nested = child.querySelector(selector);
      if (nested) return nested;
    }
    return null;
  }
  closest(selector) {
    let node = this;
    while (node) { if (node.matches(selector)) return node; node = node.parent; }
    return null;
  }
  contains(node) { while (node) { if (node === this) return true; node = node.parent; } return false; }
  click() {
    const event = { target: this, preventDefault() { this.defaultPrevented = true; } };
    let node = this;
    while (node) { for (const fn of node.listeners.click || []) fn(event); node = node.parent; }
  }
}

function loadWorkshop({ api, pending = new Set(), sessions = [], failSwitch = false } = {}) {
  const root = new Element('main'); root.dataset.workshopRoot = '';
  const calls = { api: [], switched: [], views: [] };
  const window = {
    gregalSession: 'default', gregalPendingInteractions: pending,
    gregal: {
      api: async (...args) => { calls.api.push(args); if (api) return api(...args); return { ok: true, async json() { return { sessions }; } }; },
      setView(name) { calls.views.push(name); },
    },
    gregalPanels: { sessions: { _error: '', async switchTo(id) { calls.switched.push(id); if (failSwitch) throw new Error('workspace changed'); } } },
  };
  const document = {
    querySelector(selector) { return root.matches(selector) ? root : null; },
    createElement(tag) { return new Element(tag); },
    createElementNS(namespace, tag) { const element = new Element(tag); element.namespaceURI = namespace; return element; },
  };
  let source = fs.readFileSync(path.join(__dirname, 'workshop.js'), 'utf8');
  source = source.replace(/^import .*;\s*$/gm, '').replace(/^export (?=const workshop\b)/gm, '');
  source += '\n;globalThis.__workshop = workshop;';
  const sandbox = { window, document, AbortController, console, T: key => key };
  vm.runInNewContext(source, sandbox, { filename: 'workshop.js' });
  return { workshop: sandbox.__workshop, root, calls, window };
}

test('init is quiet; update derives accessible station and attention states without unsafe HTML', async () => {
  const pending = new Set(['waiting']);
  const { workshop, root, calls } = loadWorkshop({ pending });
  workshop.init();
  assert.equal(calls.api.length, 0, 'init must not poll or load while hidden');
  await workshop.show();
  workshop.update([
    { id: 'waiting', title: '<img src=x onerror=alert(1)>', busy: true },
    { id: 'busy', project: 'Projecte', busy: true },
    { id: 'idle' },
    { id: 'idle' },
    { title: 'invalid' },
  ]);
  const stage = root.querySelector('[data-workshop-stage]');
  const stations = stage.children.filter(station => station.dataset.workshopSession);
  assert.equal(stations.length, 3);
  assert.deepEqual(stations.map(station => station.dataset.state), ['waiting', 'busy', 'idle']);
  assert.equal(stations[0].tagName, 'BUTTON');
  assert.equal(stations[0].children.find(child => child.className === 'workshop-name').textContent, '<img src=x onerror=alert(1)>');
  const figure = stations[0].children.find(child => child.className === 'workshop-figure');
  const person = figure.children.find(child => child.className === 'workshop-person');
  assert.equal(person.namespaceURI, 'http://www.w3.org/2000/svg');
  assert.equal(person.attributes.viewBox, '0 0 64 82');
  assert.equal(root.querySelector('[data-workshop-find]').disabled, false);
  const queueItem = root.querySelector('.workshop-queue-list').children[0];
  assert.equal(queueItem.textContent, '', 'queue button must not retain duplicate constructor text');
  assert.equal(queueItem.children.find(child => child.className === 'workshop-queue-label').textContent, '<img src=x onerror=alert(1)>');
  const before = stations[0];
  workshop.update([{ id: 'waiting', title: '<img src=x onerror=alert(1)>', busy: true }, { id: 'busy', project: 'Projecte', busy: true }, { id: 'idle' }]);
  assert.equal(root.querySelector('[data-workshop-stage]').children.find(child => child.dataset.workshopSession), before, 'same list should not repaint focused controls');
});

test('show loads once and attention button waits for switchTo before opening the agent', async () => {
  const pending = new Set(['beta']);
  const { workshop, root, calls } = loadWorkshop({ pending, sessions: [{ id: 'alpha' }, { id: 'beta', busy: true, title: 'Beta' }] });
  workshop.init();
  await workshop.show();
  assert.equal(calls.api.length, 1);
  assert.equal(calls.api[0][0], '/api/sessions/live');
  const attention = root.querySelector('[data-workshop-find]');
  attention.click();
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(calls.switched, ['beta']);
  assert.deepEqual(calls.views, ['agent']);
});

test('hide invalidates an in-flight initial load and prevents late rendering', async () => {
  let resolveApi;
  const { workshop, root } = loadWorkshop({ api: () => new Promise(resolve => { resolveApi = resolve; }) });
  workshop.init();
  const loading = workshop.show();
  workshop.hide();
  resolveApi({ ok: true, async json() { return { sessions: [{ id: 'late' }] }; } });
  await loading;
  assert.equal(root.querySelector('[data-workshop-stage]').children.some(child => child.dataset.workshopSession), false);
  assert.equal(root.attributes['aria-hidden'], 'true');
});

test('failed session switch stays in workshop and reports an inline error', async () => {
  const { workshop, root, calls } = loadWorkshop({ sessions: [{ id: 'broken' }], failSwitch: true });
  workshop.init();
  await workshop.show();
  root.querySelector('[data-workshop-stage]').children.find(child => child.dataset.workshopSession).click();
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(calls.views, []);
  const error = root.querySelector('[data-workshop-error]');
  assert.equal(error.textContent, 'workspace changed');
  assert.equal(error.hidden, false);
});
