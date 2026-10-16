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
  appendChild(node) { if (node.parent) node.parent.removeChild(node); this.children.push(node); node.parent = this; return node; }
  insertBefore(node, reference) { if (node.parent) node.parent.removeChild(node); const index = reference ? this.children.indexOf(reference) : -1; this.children.splice(index < 0 ? this.children.length : index, 0, node); node.parent = this; return node; }
  removeChild(node) { const index = this.children.indexOf(node); if (index >= 0) this.children.splice(index, 1); node.parent = null; return node; }
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
  focus() { this.focused = true; }
}

function loadWorkshop({ api, pending = new Set(), sessions = [], failSwitch = false, withScene = false, withFlowGraph = false, lastOkAt = 0 } = {}) {
  const root = new Element('main'); root.dataset.workshopRoot = '';
  const calls = { api: [], switched: [], views: [] };
  const scenes = [];
  const graphs = { builds: [], updates: [] };
  const window = {
    gregalSession: 'default', gregalPendingInteractions: pending,
    gregal: {
      api: async (...args) => { calls.api.push(args); if (api) return api(...args); if (String(args[0]).includes('/cancel')) return { ok: true, async json() { return { cancelled: true }; } }; return { ok: true, async json() { return { sessions }; } }; },
      setView(name) { calls.views.push(name); },
    },
    gregalPanels: { sessions: { _error: '', _lastOkAt: lastOkAt, async switchTo(id) { calls.switched.push(id); if (failSwitch) throw new Error('workspace changed'); } } },
  };
  const document = {
    querySelector(selector) { return root.matches(selector) ? root : null; },
    createElement(tag) { return new Element(tag); },
    createElementNS(namespace, tag) { const element = new Element(tag); element.namespaceURI = namespace; return element; },
    listeners: {},
    addEventListener(name, fn) { (this.listeners[name] ||= []).push(fn); },
  };
  let source = fs.readFileSync(path.join(__dirname, 'workshop.js'), 'utf8');
  const sandbox = { window, document, AbortController, console, T: key => key };
  if (withScene) {
    // The canvas scene is a browser-only collaborator: stand in with a
    // recorder so the wiring (hero session, selection, show/hide) is tested.
    source = source.replace(`import { createWorkshopScene } from './workshop-scene.js';`, 'const createWorkshopScene = globalThis.__sceneStub;');
    sandbox.__sceneStub = (canvas, onSelect) => {
      const scene = { canvas, onSelect, updates: [], updateOpts: [], selected: [], shown: 0, hidden: 0,
        update(session, opts) { this.updates.push(session ? session.id : null); this.updateOpts.push(opts); },
        select(id) { this.selected.push(id); },
        show() { this.shown++; },
        hide() { this.hidden++; } };
      scenes.push(scene);
      return scene;
    };
  }
  if (withFlowGraph) {
    // Same pattern as the scene: the shared graph module is browser-only
    // here, so stand in with clickable groups that record builds/updates.
    source = source.replace(`import { buildFlowGraph, updateFlowGraph } from './team-flow-graph.js';`,
      'const buildFlowGraph = globalThis.__flowBuild; const updateFlowGraph = globalThis.__flowUpdate;');
    sandbox.__flowBuild = (nodes, edges, onSelect) => {
      const svg = new Element('svg');
      for (const n of nodes) {
        const g = new Element('g'); g.dataset.flowNode = n.id;
        g.addEventListener('click', () => onSelect(n.id));
        svg.appendChild(g);
      }
      graphs.builds.push({ nodes, edges, svg });
      return svg;
    };
    sandbox.__flowUpdate = (svg, nodes, view) => { graphs.updates.push({ svg, nodes, view }); };
  }
  source = source.replace(/^import .*;\s*$/gm, '').replace(/^export (?=const workshop\b)/gm, '');
  source += '\n;globalThis.__workshop = workshop;';
  vm.runInNewContext(source, sandbox, { filename: 'workshop.js' });
  return { workshop: sandbox.__workshop, root, calls, window, scenes, graphs, document };
}

test('init stays quiet and the live room uses safe text with real coordinator and child seats', async () => {
  const pending = new Set(['waiting']);
  const { workshop, root, calls } = loadWorkshop({ pending });
  workshop.init();
  assert.equal(calls.api.length, 0, 'init must not poll or load while hidden');
  await workshop.show();
  workshop.update([
    { id: 'waiting', title: '<img src=x onerror=alert(1)>', busy: true, activity: { state: 'tools', model: 'm1', step: 4, limit: 20, task: 'Coordinator task', agents: [{ id: 'child-1', task: '<b>unsafe task</b>', status: 'working', model: 'm2', step: 3, max_steps: 12, tool: 'shell' }] } },
    { id: 'busy', project: 'Projecte', busy: true, activity: { state: 'thinking' } },
    { id: 'idle' },
    { id: 'idle' },
    { title: 'invalid' },
  ]);
  const stage = root.querySelector('[data-workshop-stage]');
  const bays = stage.children.filter(bay => bay.dataset.workshopBay);
  assert.equal(bays.length, 3);
  const stations = bays.map(bay => bay.querySelector('[data-workshop-session]'));
  assert.deepEqual(stations.map(station => station.dataset.state), ['waiting', 'thinking', 'idle']);
  assert.equal(stations[0].tagName, 'BUTTON');
  assert.equal(stations[0].children.find(child => child.className === 'workshop-name').textContent, '<img src=x onerror=alert(1)>');
  assert.equal(stations[0].children.find(child => child.className === 'workshop-details').textContent, 'Coordinator task');
  const figure = stations[0].children.find(child => child.className === 'coordinator-avatar');
  const person = figure.children.find(child => child.className === 'workshop-person');
  assert.equal(person.namespaceURI, 'http://www.w3.org/2000/svg');
  assert.equal(person.attributes.viewBox, '0 0 80 96');
  assert.equal(root.querySelector('[data-workshop-find]').disabled, false);
  const queueItem = root.querySelector('.workshop-queue-list').children[0];
  assert.equal(queueItem.children.find(child => child.className === 'workshop-queue-label').textContent, '<img src=x onerror=alert(1)>');
  assert.equal(bays[0].querySelector('[data-workshop-agent]').children[2].children[0].textContent, '<b>unsafe task</b>');
  assert.equal(bays[0].querySelector('[data-workshop-agent]').children[2].children[2].textContent, 'm2 · shell · workshop.step 3 / 12 workshop.budget');
  const before = stations[0];
  workshop.update([{ id: 'waiting', title: '<img src=x onerror=alert(1)>', busy: true, activity: { state: 'tools', model: 'm1', step: 5, limit: 20, task: 'Coordinator task', agents: [{ id: 'child-1', task: '<b>unsafe task</b>', status: 'working', model: 'm2', step: 4, max_steps: 12, tool: 'shell' }] } }, { id: 'busy', project: 'Projecte', busy: true, activity: { state: 'thinking' } }, { id: 'idle' }]);
  assert.equal(root.querySelector('[data-workshop-stage]').querySelector('[data-workshop-session]'), before, 'poll keeps coordinator button stable');
  assert.equal(bays[0].querySelector('[data-workshop-agent]'), stage.children[1].querySelector('[data-workshop-agent]'), 'poll keeps child button stable');
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
  root.querySelector('[data-workshop-stage]').querySelector('[data-workshop-session]').click();
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(calls.views, []);
  const error = root.querySelector('[data-workshop-error]');
  assert.equal(error.textContent, 'workspace changed');
  assert.equal(error.hidden, false);
});

test('child selection opens a persistent in-place detail view and only the explicit action opens its parent', async () => {
  const sessions = [{ id: 'parent', title: 'Coordinator', activity: { state: 'tools', task: 'Delegating', agents: [
    { id: 'child', task: 'Inspect files', status: 'working', model: 'model-x', step: 2, max_steps: 10, tool: 'read_file', message: 'Reading source' },
  ] } }];
  const { workshop, root, calls } = loadWorkshop({ sessions });
  workshop.init(); await workshop.show();
  const child = root.querySelector('[data-workshop-agent]'); child.click();
  assert.deepEqual(calls.switched, [], 'opening details must not switch sessions');
  const detail = root.querySelector('.workshop-detail');
  assert.equal(detail.hidden, false);
  const open = detail.querySelector('.detail-open-parent');
  assert.equal(open.dataset.workshopSession, 'parent');
  workshop.update([{ ...sessions[0], activity: { ...sessions[0].activity, agents: [{ ...sessions[0].activity.agents[0], step: 3 }] } }]);
  assert.equal(detail.querySelector('.detail-open-parent'), open, 'poll keeps detail controls stable');
  assert.equal(detail.querySelector('.workshop-detail-close').focused, true, 'selection moves keyboard focus to the detail close button');
  detail.querySelector('.workshop-detail-close').click();
  assert.equal(detail.hidden, true);
  assert.equal(child.focused, true, 'closing details returns focus to the selected child');
  child.click();
  open.click();
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(calls.switched, ['parent']);
  assert.deepEqual(calls.views, ['agent']);
});

test('no activity payload remains compatible and empty room gives a real next step', async () => {
  const { workshop, root } = loadWorkshop(); workshop.init(); await workshop.show();
  workshop.update([{ id: 'legacy', busy: true }]);
  const station = root.querySelector('[data-workshop-session]');
  assert.equal(station.dataset.state, 'thinking');
  assert.equal(root.querySelector('.no-children').hidden, false);
  workshop.update([]);
  assert.equal(root.querySelector('.workshop-empty-state').children[1].textContent, 'workshop.emptyTitle');
});

test('new run removes departed child nodes and closes stale child details', async () => {  const first = { id: 'parent', title: 'Coordinator', activity: { state: 'tools', agents: [{ id: 'old-child', task: 'Old run', status: 'working' }] } };
  const { workshop, root } = loadWorkshop({ sessions: [first] });
  workshop.init(); await workshop.show();
  const oldChild = root.querySelector('[data-workshop-agent]'); oldChild.click();
  const detail = root.querySelector('.workshop-detail');
  assert.equal(detail.hidden, false);

  workshop.update([{ id: 'parent', title: 'Coordinator', activity: { state: 'thinking', agents: [] } }]);
  assert.equal(root.querySelector('[data-workshop-agent]'), null, 'removed child is detached from its real container');
  assert.equal(oldChild.parent, null);
  assert.equal(detail.hidden, true, 'details close when the selected agent disappears in the next run');
});

test('canvas office portraits the current session and scene picks open the detail', async () => {
  const sessions = [
    { id: 'default', title: 'Main', busy: true, activity: { state: 'tools', task: 'Build', agents: [{ id: 'a1', task: 'Read sources', status: 'working', model: 'm2', step: 2, max_steps: 12, tool: 'read' }] } },
    { id: 'other', title: 'Other', activity: { state: 'idle', agents: [] } },
  ];
  const { workshop, root, scenes } = loadWorkshop({ sessions, withScene: true, pending: new Set(['default']) });
  workshop.init();
  await workshop.show();
  assert.equal(scenes.length, 1, 'one scene is created for the room');
  const scene = scenes[0];
  assert.equal(scene.canvas.className, 'workshop-scene');
  assert.equal(scene.canvas.hidden, false);
  assert.equal(scene.updates[scene.updates.length - 1], 'default', 'scene follows the current session, not the first bay');
  const lastOpts = scene.updateOpts[scene.updateOpts.length - 1];
  assert.equal(lastOpts.waiting, true, 'scene sees the same pending state as the DOM bays');
  assert.equal(lastOpts.labels.tools, 'workshop.tools', 'scene badges use the interface language');
  assert.equal(scene.shown, 1);
  scene.onSelect('a1');
  const detail = root.querySelector('.workshop-detail');
  assert.equal(detail.hidden, false, 'picking an agent in the scene opens its detail');
  workshop.update([]);
  assert.equal(scene.canvas.hidden, true, 'scene hides with an empty room');
  workshop.hide();
  assert.equal(scene.hidden, 1);
});

test('bays show honest usage and a busy bay can be stopped in place', async () => {
  const sessions = [
    { id: 'metered', title: 'Metered', busy: true, activity: { state: 'tools', model: 'm1', step: 3, limit: 10,
      up_tokens: 1500, down_tokens: 250, calls: 4, tokens_source: 'api', cost_usd: 0.0042, agents: [
        { id: 'c1', task: 'Read', status: 'working', model: 'm2', tool: 'read', step: 2, max_steps: 12, up_tokens: 1500, down_tokens: 250, calls: 4, usage_known: true },
      ] } },
    { id: 'idle', activity: { state: 'idle' } },
  ];
  const { workshop, root, calls } = loadWorkshop({ sessions });
  workshop.init();
  await workshop.show();
  const bays = root.querySelector('[data-workshop-stage]').children.filter(bay => bay.dataset.workshopBay);
  const usage = bays[0].querySelector('.bay-usage');
  assert.equal(usage.hidden, false);
  assert.equal(usage.textContent, '↑1.5k · ↓250 · $0.0042');
  const facts = bays[0].querySelector('[data-workshop-agent]').children[2].children[2].textContent;
  assert.match(facts, /↑1\.5k · ↓250/);
  const stop = bays[0].querySelector('.bay-stop');
  assert.equal(stop.hidden, false);
  stop.click();
  await new Promise(resolve => setImmediate(resolve));
  const cancel = calls.api[calls.api.length - 1];
  assert.equal(cancel[0], '/api/agent/cancel');
  assert.equal(cancel[1].method, 'POST');
  assert.equal(cancel[1].headers['X-Gregal-Session'], 'metered');
  assert.equal(bays[1].querySelector('.bay-stop').hidden, true, 'idle bays offer no stop');
});

test('a queued turn reports honestly instead of faking a stop', async () => {
  const sessions = [
    { id: 'queued', title: 'Queued', busy: true, activity: { state: 'thinking' } },
  ];
  const api = async url => String(url).includes('/cancel')
    ? { ok: true, async json() { return { cancelled: false }; } }
    : { ok: true, async json() { return { sessions }; } };
  const { workshop, root } = loadWorkshop({ sessions, api });
  workshop.init();
  await workshop.show();
  const stop = root.querySelector('.bay-stop');
  stop.click();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(root.querySelector('[data-workshop-error]').textContent, 'workshop.stopQueued');
  assert.equal(stop.disabled, false, 'failed stop re-enables the button');
});

test('calls without declared usage stay silent instead of inventing zeros', async () => {
  const sessions = [
    { id: 'quiet', title: 'Quiet', busy: true, activity: { state: 'thinking', calls: 2, agents: [
      { id: 'c1', task: 'Read', status: 'working', calls: 2 },
    ] } },
  ];
  const { workshop, root } = loadWorkshop({ sessions });
  workshop.init();
  await workshop.show();
  const bay = root.querySelector('[data-workshop-stage]').children.find(child => child.dataset.workshopBay);
  assert.equal(bay.querySelector('.bay-usage').textContent, '2 workshop.calls');
  const facts = bay.querySelector('[data-workshop-agent]').children[2].children[2].textContent;
  assert.match(facts, /2 workshop\.calls/);
});

test('turn history lists steps in order and stays hidden when empty', async () => {
  const sessions = [
    { id: 'lined', title: 'Lined', busy: true, activity: { state: 'tools', step: 2, limit: 10, history: [
      { step: 1, state: 'thinking', at: '2026-10-09T09:00:01Z' },
      { step: 2, state: 'tools', tool: 'read', message: '<img src=x onerror=alert(1)>', at: '2026-10-09T09:00:02Z' },
    ] } },
    { id: 'fresh', activity: { state: 'idle' } },
  ];
  const { workshop, root } = loadWorkshop({ sessions });
  workshop.init();
  await workshop.show();
  const bays = root.querySelector('[data-workshop-stage]').children.filter(child => child.dataset.workshopBay);
  const items = bays[0].querySelector('.history-list').children;
  assert.equal(bays[0].querySelector('.bay-history').hidden, false);
  assert.equal(items.length, 2);
  assert.equal(items[0].children[0].textContent, '#1');
  assert.equal(items[1].children[1].textContent, 'workshop.tools · read — <img src=x onerror=alert(1)>');
  assert.equal(typeof items[1].children[2].textContent, 'string');
  assert.equal(bays[1].querySelector('.bay-history').hidden, true);
});

test('summary totals priced session cost and admits unknown cost', async () => {
  const { workshop, root } = loadWorkshop({ sessions: [
    { id: 'a', sess_cost_usd: 0.02, sess_cost_known: true, activity: { state: 'completed' } },
    { id: 'b', sess_cost_usd: 0.0042, sess_cost_known: true, activity: { state: 'completed' } },
  ] });
  workshop.init();
  await workshop.show();
  assert.match(root.querySelector('.workshop-summary').textContent, /\$0\.024 workshop\.total/);
  workshop.update([{ id: 'c', activity: { state: 'idle' } }]);
  assert.match(root.querySelector('.workshop-summary').textContent, /workshop\.costNA/);
});

test('summary shows data age and stale errors survive repaints', async () => {
  const { workshop, root } = loadWorkshop({ sessions: [{ id: 'a', activity: { state: 'idle' } }], lastOkAt: Date.now() - 12000 });
  workshop.init();
  await workshop.show();
  assert.match(root.querySelector('.workshop-summary').textContent, /workshop\.updatedAgo/);
});

test('attention button cycles through waiting sessions', async () => {
  const sessions = [{ id: 'a', title: 'A' }, { id: 'b', title: 'B' }, { id: 'c', title: 'C' }];
  const { workshop, root, calls } = loadWorkshop({ sessions, pending: new Set(['a', 'b', 'c']) });
  workshop.init();
  await workshop.show();
  const attention = root.querySelector('[data-workshop-find]');
  for (const want of ['a', 'b', 'c', 'a']) {
    attention.click();
    await new Promise(resolve => setImmediate(resolve));
  }
  assert.deepEqual(calls.switched, ['a', 'b', 'c', 'a']);
});

test('failed sessions get their own queue section', async () => {
  const sessions = [
    { id: 'w', title: 'Waiting', busy: true, activity: { state: 'tools' } },
    { id: 'f', title: 'Broken', activity: { state: 'failed', message: 'boom' } },
    { id: 'ok', activity: { state: 'idle' } },
  ];
  const { workshop, root } = loadWorkshop({ sessions, pending: new Set(['w']) });
  workshop.init();
  await workshop.show();
  const items = root.querySelector('.workshop-queue-list').children;
  assert.equal(items.length, 3);
  assert.equal(items[1].className, 'workshop-queue-divider');
  assert.equal(items[1].textContent, 'workshop.withErrors');
  assert.match(items[2].className, /is-failed/);
  assert.equal(items[2].children[1].textContent, 'Broken');
});

test('long outputs clamp with an expander that survives polls', async () => {
  const big = 'x'.repeat(500);
  const sessions = [{ id: 'parent', title: 'Coordinator', activity: { state: 'tools', agents: [
    { id: 'child', task: 'Inspect', status: 'working', output: big },
  ] } }];
  const { workshop, root } = loadWorkshop({ sessions });
  workshop.init();
  await workshop.show();
  root.querySelector('[data-workshop-agent]').click();
  const detail = root.querySelector('.workshop-detail');
  const output = workshop._detailFields.get('output');
  assert.equal(output.text.className, 'detail-text detail-clamp');
  assert.equal(output.more.hidden, false);
  assert.equal(output.more.textContent, 'workshop.more');
  output.more.onclick();
  assert.equal(output.text.className, 'detail-text');
  workshop.update([{ id: 'parent', title: 'Coordinator', activity: { state: 'tools', agents: [
    { id: 'child', task: 'Inspect', status: 'working', output: big, step: 3 },
  ] } }]);
  assert.equal(output.text.className, 'detail-text', 'polls keep the expanded state');
  workshop.update([{ id: 'parent', title: 'Coordinator', activity: { state: 'tools', agents: [
    { id: 'child', task: 'Inspect', status: 'working', output: 'short' },
  ] } }]);
  assert.equal(output.more.hidden, true, 'short content needs no expander');
});

test('Escape closes the detail and returns focus', async () => {
  const sessions = [{ id: 'parent', title: 'Coordinator', activity: { state: 'tools', agents: [
    { id: 'child', task: 'Inspect', status: 'working' },
  ] } }];
  const { workshop, root, document } = loadWorkshop({ sessions });
  workshop.init();
  await workshop.show();
  const child = root.querySelector('[data-workshop-agent]');
  child.click();
  assert.equal(root.querySelector('.workshop-detail').hidden, false);
  const keydown = document.listeners.keydown[document.listeners.keydown.length - 1];
  keydown({ key: 'Escape', target: {} });
  assert.equal(root.querySelector('.workshop-detail').hidden, true);
  assert.equal(child.focused, true);
});

test('team pipeline runs render as bays with graph, agents and detail', async () => {
  const flow = { nodes: [
    { id: 'coordinator' }, { id: 'researcher' }, { id: 'builder' }, { id: 'reviewer' },
  ], edges: [
    { from: 'coordinator', to: 'researcher', kind: 'data' },
    { from: 'coordinator', to: 'builder', kind: 'data' },
    { from: 'researcher', to: 'reviewer', kind: 'data' },
    { from: 'builder', to: 'reviewer', kind: 'data' },
    { from: 'reviewer', to: 'builder', kind: 'revision' },
  ] };
  const sessions = [{
    id: 'team-one', title: 'Festival site', busy: true,
    activity: { kind: 'team', state: 'tools', task: 'Build a site', model: 'm1', flow,
      agents: [
        { id: 'coordinator', status: 'completed', model: 'm1', output: 'plan ok' },
        { id: 'researcher', status: 'completed', model: 'm1', output: 'research ok' },
        { id: 'builder', status: 'working', model: 'm1', tool: 'draft' },
      ] },
  }, { id: 'plain', activity: { state: 'idle' } }];
  const { workshop, root, graphs } = loadWorkshop({ sessions, withFlowGraph: true });
  workshop.init();
  await workshop.show();
  assert.equal(graphs.builds.length, 1, 'one graph for the team bay');
  assert.deepEqual(graphs.builds[0].nodes.map(n => n.id), ['coordinator', 'researcher', 'builder', 'reviewer']);
  assert.equal(graphs.builds[0].edges.length, 5);
  const last = graphs.updates[graphs.updates.length - 1];
  assert.equal(JSON.stringify(last.view.states), JSON.stringify({ coordinator: 'completed', researcher: 'completed', builder: 'working' }));
  const bays = root.querySelector('[data-workshop-stage]').children.filter(c => c.dataset && c.dataset.workshopBay);
  assert.equal(bays.length, 2);
  const teamBay = bays[0];
  assert.equal(teamBay.querySelector('.bay-graph').hidden, false);
  assert.equal(bays[1].querySelector('.bay-graph').hidden, true, 'plain sessions show no graph');
  assert.equal(workshop._stations.get('team-one')._children.children.length, 3);
  assert.equal(teamBay.querySelector('.bay-stop').hidden, false, 'running team bay can be stopped');
  const node = graphs.builds[0].svg.children.find(c => c.dataset.flowNode === 'builder');
  node.click();
  assert.equal(root.querySelector('.workshop-detail').hidden, false);
  assert.equal(root.querySelector('.workshop-detail-title').textContent, 'builder');
});

test('role room: legend, clickable step strip and role detail with files', async () => {
  const lanes = [
    { seq: 1, step: 0, role: 'researcher', kind: 'scout', done: true, result: 'ok', agent: 'scout-1', duration_ms: 4000 },
    { seq: 3, step: 2, role: 'builder', kind: 'edit', target: 'src/stats.py', files: ['src/stats.py'], done: true, result: 'ok', duration_ms: 900 },
    { seq: 5, step: 3, role: 'tester', kind: 'test', command: 'pytest -q', done: false },
  ];
  const sessions = [{ id: 'default', title: 'Main', busy: true, activity: { state: 'tools', role: 'tester', lanes, handoffs: [{ seq: 4, from: 'coordinator', to: 'builder', kind: 'edit', n: 1 }],
    agents: [{ id: 'scout-1', agent: 'scout', task: 'Investigate', status: 'completed', output: 'stats.py has mean()' }] } }];
  const { workshop, root, window, scenes } = loadWorkshop({ sessions, withScene: true });
  window.gregalWorkshopRoles = require('./workshop-roles.js');
  const viewed = [];
  window.gregalPanels.files = { view(p) { viewed.push(p); } };
  workshop.init();
  await workshop.show();
  const room = root.querySelector('.workshop-room');
  assert.ok(room, 'the room wraps the canvas');
  const legend = room.querySelector('.room-legend');
  assert.deepEqual(legend.children.map(b => [b.dataset.roomRole, b.dataset.state]), [['coordinator', 'done'], ['researcher', 'done'], ['builder', 'done'], ['tester', 'active'], ['reviewer', 'idle']]);
  const strip = room.querySelector('.room-strip');
  assert.deepEqual(strip.children.map(s => s.dataset.role), ['researcher', 'builder', 'tester']);
  assert.equal(strip.children[2].dataset.live, 'true');
  // Clicking a step selects it: the character is highlighted and the role detail opens.
  strip.children[1].click();
  const detail = room.querySelector('.room-detail');
  assert.equal(detail.hidden, false);
  assert.equal(detail.dataset.role, 'builder');
  assert.equal(scenes[0].selected[scenes[0].selected.length - 1], 'role:builder');
  const file = detail.querySelector('.room-file');
  assert.equal(file.dataset.roomFile, 'src/stats.py');
  file.click();
  assert.deepEqual(viewed, ['src/stats.py']);
  // The researcher's detail shows the real scout child with its summary.
  legend.children[1].click();
  assert.equal(detail.dataset.role, 'researcher');
  assert.equal(detail.querySelector('.room-child').querySelector('.room-child-out').textContent, 'stats.py has mean()');
  // Scene clicks on a role desk route to the same panel; close hides it.
  scenes[0].onSelect('role:tester');
  assert.equal(detail.dataset.role, 'tester');
  assert.equal(detail.querySelector('.room-command').children[0].textContent, 'pytest -q');
  detail.querySelector('.room-detail-close').click();
  assert.equal(detail.hidden, true);
});

test('parallel children: badge, time bars, queued state and failure reason', async () => {
  const t = s => new Date(Date.now() - (60 - s) * 1000).toISOString();
  const agents = [
    { id: 'c1', agent: 'scout', task: 'internal/team', status: 'working', started_at: t(0), updated_at: t(50), tool: 'read' },
    { id: 'c2', agent: 'scout', task: 'internal/web/app/workshop.js', status: 'working', started_at: t(2), updated_at: t(50), tool: 'grep' },
    { id: 'c3', task: 'web research', status: 'queued', started_at: t(3), updated_at: t(3) },
    { id: 'c4', task: 'x'.repeat(600), status: 'failed', message: 'provider 429: rate limited', started_at: t(1), updated_at: t(9) },
  ];
  const sessions = [{ id: 'default', title: 'Main', busy: true, activity: { state: 'tools', started_at: t(0), agents } }];
  const { workshop, root, window } = loadWorkshop({ sessions });
  window.gregalWorkshopRoles = require('./workshop-roles.js');
  workshop.init();
  await workshop.show();
  const bay = root.querySelector('.workshop-bay');
  const badge = bay.querySelector('.child-parallel');
  assert.equal(badge.hidden, false);
  assert.equal(badge.dataset.live, 'true');
  assert.equal(badge.textContent, 'workshop.atOnce'.replace('{n}', '2'));
  assert.match(bay.querySelector('.child-count').textContent, /1 workshop\.queuedN/);
  const cards = bay.querySelector('.child-agents').children;
  assert.deepEqual(cards.map(c => c.dataset.state), ['working', 'working', 'queued', 'failed']);
  // Every card has a bar on the turn's time axis.
  assert.ok(cards.every(c => c.querySelector('.child-span').hidden === false));
  assert.equal(cards[0].querySelector('.child-span-bar').style.left, '0%');
  // The failed card says why; long tasks keep a short accessible name.
  assert.match(cards[3].querySelector('.child-facts').textContent, /rate limited/);
  assert.ok(cards[3].attributes['aria-label'].length < 220);
  // After the fan-out ends the badge keeps the peak.
  const done = agents.map(a => ({ ...a, status: 'completed', updated_at: t(55) }));
  workshop.update([{ id: 'default', title: 'Main', busy: false, activity: { state: 'completed', started_at: t(0), agents: done } }]);
  assert.equal(badge.dataset.live, 'false');
  assert.equal(badge.textContent, 'workshop.peakAtOnce');
});

test('counts use singular forms and the step strip keeps button semantics', async () => {
  const sessions = [{ id: 'default', title: 'Main', busy: true, activity: { state: 'tools', agents: [{ id: 'c1', task: 'one', status: 'working' }],
    lanes: [{ seq: 1, step: 1, role: 'researcher', kind: 'read', target: 'a.go', done: true, result: 'ok' }] } }];
  const { workshop, root, window } = loadWorkshop({ sessions, withScene: true });
  window.gregalWorkshopRoles = require('./workshop-roles.js');
  workshop.init();
  await workshop.show();
  assert.match(root.querySelector('.workshop-summary').textContent, /^1 workshop\.sessionsOne/);
  assert.match(root.querySelector('.child-count').textContent, /^1 workshop\.agentsOne/);
  const strip = root.querySelector('.room-strip');
  assert.equal(strip.attributes.role, 'group');
  assert.ok(strip.children.every(b => b.tagName === 'BUTTON' && !b.attributes.role));
  assert.equal(root.querySelector('.room-detail').attributes['aria-live'], undefined);
});
