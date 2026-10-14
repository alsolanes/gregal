// A live, locally rendered view of coordinator sessions and their real child agents.
import { t as T } from './i18n.js';
import { createWorkshopScene } from './workshop-scene.js';
import { buildFlowGraph, updateFlowGraph } from './team-flow-graph.js';
import './workshop-roles.js';

const G = () => window.gregal;
// Pure role helpers (workshop-roles.js); absent in the node harness unless
// a test loads them, so every use is guarded.
const RA = () => (typeof window !== 'undefined' && window.gregalWorkshopRoles) || null;
const $ = (selector, root) => (root || document).querySelector(selector);

function pendingIds() {
  const pending = window.gregalPendingInteractions;
  return pending && typeof pending.has === 'function' ? pending : { has: () => false };
}

function make(tag, className, text) {
  const el = document.createElement(tag);
  if (className) el.className = className;
  if (text != null) el.textContent = text;
  return el;
}

function detach(node) {
  const parent = node && (node.parentNode || node.parent);
  if (!parent) return;
  if (typeof parent.removeChild === 'function') parent.removeChild(node);
  else if (Array.isArray(parent.children)) parent.children = parent.children.filter(child => child !== node);
}

function reconcile(parent, desired) {
  const active = typeof document !== 'undefined' ? document.activeElement : null;
  desired.forEach((node, index) => {
    if (parent.children[index] === node) return;
    const before = parent.children[index] || null;
    if (typeof parent.insertBefore === 'function') parent.insertBefore(node, before);
    else parent.appendChild(node);
  });
  while (parent.children.length > desired.length) detach(parent.children[parent.children.length - 1]);
  if (active && desired.includes(active) && typeof active.focus === 'function' && document.activeElement !== active) active.focus();
}

function svg(tag, attrs = {}) {
  const node = document.createElementNS('http://www.w3.org/2000/svg', tag);
  Object.entries(attrs).forEach(([key, value]) => node.setAttribute(key, value));
  return node;
}

function person() {
  const node = svg('svg', { class: 'workshop-person', viewBox: '0 0 80 96', 'aria-hidden': 'true', focusable: 'false' });
  node.append(
    svg('path', { class: 'person-hair', d: 'M20 31c-2-18 6-27 20-27s23 10 21 29l-7 9H26z' }),
    svg('circle', { class: 'person-face', cx: 40, cy: 30, r: 15 }),
    svg('path', { class: 'person-fringe', d: 'M25 25c2-13 10-17 18-16 8 0 13 6 15 15-7-2-12-6-15-11-4 6-11 10-18 12z' }),
    svg('circle', { class: 'person-eyes', cx: 35, cy: 31, r: 1.4 }), svg('circle', { class: 'person-eyes', cx: 46, cy: 31, r: 1.4 }),
    svg('path', { class: 'person-smile', d: 'M36 38q4 4 8 0' }), svg('path', { class: 'person-neck', d: 'M35 44h10v9H35z' }),
    svg('path', { class: 'person-shirt', d: 'M27 51l8-4h10l9 4 9 32H17z' }),
    svg('path', { class: 'person-arm', d: 'M28 54l-9 20m34-20 9 18' }),
  );
  return node;
}

const LIVE = new Set(['thinking', 'tools', 'compacting', 'reviewing', 'working']);
const STATE_KEY = { waiting: 'waiting', thinking: 'thinking', tools: 'tools', compacting: 'compacting', reviewing: 'reviewing', completed: 'completed', failed: 'failed', cancelled: 'cancelled', working: 'working', idle: 'idle' };
const CHILD_KEY = { working: 'working', completed: 'completed', failed: 'failed', cancelled: 'cancelled', idle: 'idle' };
const clean = (value, fallback = '') => typeof value === 'string' ? value : fallback;
const num = value => Number.isFinite(value) ? value : 0;
const keyState = state => T('workshop.' + (STATE_KEY[state] || 'idle'));
const validAgent = a => a && typeof a.id === 'string' && a.id;
// normalizeFlow keeps just what the bay graph needs: node ids and the
// deliverable / revision links. Unknown shapes hide the graph.
function normalizeFlow(flow) {
  if (!flow || typeof flow !== 'object') return null;
  const nodes = Array.isArray(flow.nodes) ? flow.nodes.filter(n => n && typeof n.id === 'string' && n.id).map(n => ({ id: n.id })) : [];
  if (!nodes.length) return null;
  const ids = new Set(nodes.map(n => n.id));
  const edges = Array.isArray(flow.edges) ? flow.edges.filter(e => e && ids.has(e.from) && ids.has(e.to) && (e.kind === 'data' || e.kind === 'revision')).map(e => ({ from: e.from, to: e.to, kind: e.kind })) : [];
  return { nodes, edges };
}
const fmtTokens = n => {
  if (n >= 1000000) return String(Math.round(n / 100000) / 10).replace(/\.0$/, '') + 'M';
  if (n >= 1000) return String(Math.round(n / 100) / 10).replace(/\.0$/, '') + 'k';
  return String(n);
};
// Relative age shared by the summary freshness and the bay elapsed time.
function ageText(ms) {
  if (ms < 5000) return T('workshop.justNow');
  if (ms < 60000) return T('workshop.updatedAgo').replace('{n}', String(Math.floor(ms / 1000)));
  if (ms < 3600000) return T('workshop.updatedAgoMin').replace('{n}', String(Math.floor(ms / 60000)));
  return T('workshop.updatedAgoH').replace('{n}', String(Math.floor(ms / 3600000)));
}
// Mirrors agent.FmtCost: never scientific notation, no invented prices.
const fmtCost = usd => usd < 0.01 ? '$' + usd.toFixed(4) : usd < 10 ? '$' + usd.toFixed(3) : '$' + usd.toFixed(2);
// Honest consumption line: real tokens when the provider declared them
// (↑ up · ↓ down · optional $cost), bare call count otherwise, empty
// before the first call.
function usageText({ up = 0, down = 0, calls = 0, known = false, cost = null }) {
  if (!calls) return '';
  if (!known && !up && !down) return `${calls} ${T('workshop.calls')}`;
  let text = `↑${fmtTokens(up)} · ↓${fmtTokens(down)}`;
  if (cost != null && Number.isFinite(cost)) text += ` · ${fmtCost(cost)}`;
  return text;
}

export const workshop = {
  _root: null, _stage: null, _window: null, _queue: null, _queueList: null,
  _queueHeading: null, _summary: null, _error: null, _find: null,
  _detail: null, _detailTitle: null, _detailBody: null, _detailClose: null,
  _list: [], _stations: new Map(), _queueItems: new Map(), _signature: '',
  _initialized: false, _visible: false, _loadVersion: 0, _selectionVersion: 0,
  _abort: null, _selectedAgent: null, _sceneCanvas: null, _scene: null, _sceneSessionId: null, _lastWaitingId: null,

  init() {
    if (this._initialized) return this;
    const root = $('[data-workshop-root]') || $('#workshopRoot') || $('.workshop-root');
    if (!root) return this;
    this._root = root;
    this._stage = $('[data-workshop-stage]', root) || $('.workshop-stage', root);
    this._queue = $('[data-workshop-queue]', root) || $('.workshop-queue', root);
    this._error = $('[data-workshop-error]', root) || $('.workshop-error', root);
    this._find = $('[data-workshop-find]', root);
    if (!this._stage) { this._stage = make('div', 'workshop-stage'); this._stage.dataset.workshopStage = ''; root.appendChild(this._stage); }
    if (!this._queue) { this._queue = make('aside', 'workshop-queue'); this._queue.dataset.workshopQueue = ''; root.appendChild(this._queue); }
    this._window = $('.workshop-window', this._stage) || make('div', 'workshop-window');
    this._window.setAttribute('aria-hidden', 'true');
    this._stage.appendChild(this._window);
    this._queueHeading = $('.workshop-needs', this._queue);
    this._summary = $('.workshop-summary', this._queue);
    if (!this._queueHeading) { this._queueHeading = make('h2', 'workshop-needs', T('workshop.needs')); this._queue.appendChild(this._queueHeading); }
      if (!this._summary) { this._summary = make('p', 'workshop-summary'); this._queue.appendChild(this._summary); }
    this._queueList = $('.workshop-queue-list', this._queue);
    if (!this._queueList) { this._queueList = make('div', 'workshop-queue-list'); this._queue.appendChild(this._queueList); }
    if (!this._detail) {
      this._detail = make('section', 'workshop-detail'); this._detail.hidden = true; this._detail.setAttribute('aria-labelledby', 'workshopDetailTitle');
      this._detailTitle = make('h2', 'workshop-detail-title'); this._detailTitle.id = 'workshopDetailTitle';
      this._detailClose = make('button', 'workshop-detail-close', T('workshop.close')); this._detailClose.type = 'button'; this._detailClose.dataset.workshopClose = '';
      this._detailBody = make('div', 'workshop-detail-body');
      this._detailStatus = make('p', 'detail-state');
      this._detailFacts = make('dl', 'detail-facts'); this._detailFields = new Map();
      for (const key of ['parent', 'model', 'tool', 'step', 'usage', 'message', 'output']) {
        const dt = make('dt', '', T('workshop.' + key)); const dd = make('dd');
        const field = { dt, dd };
        if (key === 'message' || key === 'output') {
          const text = make('span', 'detail-text'); const more = make('button', 'detail-more', T('workshop.more'));
          more.type = 'button'; more.hidden = true;
          more.onclick = () => {
            const clamped = text.className === 'detail-text detail-clamp';
            text.className = clamped ? 'detail-text' : 'detail-text detail-clamp';
            more.textContent = clamped ? T('workshop.less') : T('workshop.more');
          };
          dd.append(text, more);
          field.text = text; field.more = more;
        }
        this._detailFacts.append(dt, dd); this._detailFields.set(key, field);
      }
      this._detailOpen = make('button', 'detail-open-parent', T('workshop.openParent')); this._detailOpen.type = 'button';
      this._detailBody.append(this._detailStatus, this._detailFacts, this._detailOpen);
      this._detail.append(this._detailTitle, this._detailClose, this._detailBody); this._queue.appendChild(this._detail);
    }
    if (!this._error) { this._error = make('div', 'workshop-error'); this._error.dataset.workshopError = ''; this._error.setAttribute('role', 'alert'); (this._root.querySelector('.workshop-header') || root).appendChild(this._error); }
    if (!this._find) { this._find = make('button', 'workshop-attention', T('workshop.find')); this._find.type = 'button'; this._find.dataset.workshopFind = ''; root.appendChild(this._find); }
    // The canvas office is a live portrait of one session (the current one):
    // agents walk in, monitors glow while they work, finished work is
    // delivered to the coordinator. The DOM bays below stay the accessible
    // layer. Guarded: the node test harness strips imports, so the factory
    // may not exist there.
    if (!this._sceneCanvas && typeof createWorkshopScene === 'function') {
      this._sceneCanvas = make('canvas', 'workshop-scene');
      this._sceneCanvas.setAttribute('aria-hidden', 'true');
      this._scene = createWorkshopScene(this._sceneCanvas, agentId => {
        if (!this._sceneSessionId) return;
        if (typeof agentId === 'string' && agentId.startsWith('role:')) this.selectRole(this._sceneSessionId, agentId.slice(5));
        else this.selectAgent(this._sceneSessionId, agentId);
      });
    }
    // The room: canvas, a clickable step strip and the role detail panel.
    // The legend buttons are also the accessible way into each role.
    if (!this._room && this._sceneCanvas) {
      this._room = make('section', 'workshop-room');
      this._room.setAttribute('aria-label', T('workshop.room'));
      this._legend = make('div', 'room-legend'); this._legend.setAttribute('role', 'group'); this._legend.setAttribute('aria-label', T('workshop.roles'));
      this._strip = make('div', 'room-strip'); this._strip.setAttribute('role', 'list'); this._strip.setAttribute('aria-label', T('workshop.timeline'));
      this._roleDetail = make('section', 'room-detail'); this._roleDetail.hidden = true; this._roleDetail.setAttribute('aria-live', 'polite');
      this._room.append(this._sceneCanvas, this._legend, this._strip, this._roleDetail);
      // Full width above the bays and the attention list: the room is the
      // hero of the page, and at this size its labels stay legible.
      if (typeof root.insertBefore === 'function') root.insertBefore(this._room, this._stage); else root.appendChild(this._room);
      this._room.addEventListener('click', e => {
        const t = e.target && e.target.closest ? e.target : null; if (!t) return;
        const lane = t.closest('[data-room-lane]');
        if (lane) { e.preventDefault(); this.selectLane(Number(lane.dataset.roomLane)); return; }
        const role = t.closest('[data-room-role]');
        if (role) { e.preventDefault(); if (this._sceneSessionId) this.selectRole(this._sceneSessionId, role.dataset.roomRole); return; }
        const file = t.closest('[data-room-file]');
        if (file) { e.preventDefault(); this.openFile(file.dataset.roomFile); return; }
        if (t.closest('[data-room-changes]')) { e.preventDefault(); this.openChanges(); return; }
        if (t.closest('[data-room-close]')) { e.preventDefault(); this.closeRole(); }
      });
    }
    this._find.type = 'button'; this._find.setAttribute('aria-label', T('workshop.find'));
    root.setAttribute('aria-hidden', this._visible ? 'false' : 'true');
    root.addEventListener('click', e => {
      const target = e.target && e.target.closest ? e.target.closest('[data-workshop-agent]') : null;
      if (target && root.contains(target)) { e.preventDefault(); this.selectAgent(target.dataset.workshopSession, target.dataset.workshopAgent); return; }
      const close = e.target && e.target.closest ? e.target.closest('[data-workshop-close]') : null;
      if (close && root.contains(close)) { e.preventDefault(); this.closeAgent(); return; }
      // The bay stop sits inside the session station: match it before the
      // station itself, or every stop becomes a session switch.
      const stop = e.target && e.target.closest ? e.target.closest('[data-workshop-stop]') : null;
      if (stop && root.contains(stop)) { e.preventDefault(); this.stopSession(stop.dataset.workshopSession, stop); return; }
      const station = e.target && e.target.closest ? e.target.closest('[data-workshop-session]') : null;
      if (station && root.contains(station)) { e.preventDefault(); this.select(station.dataset.workshopSession); return; }
      const find = e.target && e.target.closest ? e.target.closest('[data-workshop-find]') : null;
      if (find && root.contains(find)) { e.preventDefault(); this.selectWaiting(); return; }
    });
    this._initialized = true;
    if (typeof document.addEventListener === 'function') document.addEventListener('gregal:idioma', () => { this._signature = ''; this._render(); });
    if (typeof document.addEventListener === 'function') document.addEventListener('keydown', e => {
      if (!e || e.key !== 'Escape' || !this._visible || !this._detail || this._detail.hidden) return;
      const target = e.target;
      if (typeof HTMLElement === 'function' && target instanceof HTMLElement && (target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName))) return;
      this.closeAgent();
    });
    this._render();
    return this;
  },

  async show() {
    this.init(); if (!this._root || this._visible) return;
    this._visible = true; const version = ++this._loadVersion; this._selectionVersion++;
    this._root.setAttribute('aria-hidden', 'false'); this._render();
    if (this._scene) this._scene.show();
    const Gref = G(); if (!Gref || typeof Gref.api !== 'function') return;
    const controller = typeof AbortController === 'function' ? new AbortController() : null; this._abort = controller;
    try {
      const response = await Gref.api('/api/sessions/live', controller ? { signal: controller.signal } : undefined);
      if (version !== this._loadVersion || !this._visible || G() !== Gref) return;
      if (!response || !response.ok) throw new Error(response ? 'HTTP ' + response.status : T('workshop.loadError'));
      const body = await response.json(); if (version !== this._loadVersion || !this._visible || G() !== Gref) return;
      this.update(Array.isArray(body && body.sessions) ? body.sessions : []); this._showError('');
    } catch (error) {
      if (version !== this._loadVersion || !this._visible || (error && error.name === 'AbortError')) return;
      this._showError(T('workshop.loadError') + (error && error.message ? ': ' + error.message : ''));
    } finally { if (version === this._loadVersion) this._abort = null; }
  },

  hide() {
    this._visible = false; ++this._loadVersion; ++this._selectionVersion;
    if (this._abort) { try { this._abort.abort(); } catch (e) {} this._abort = null; }
    if (this._scene) this._scene.hide();
    if (this._root) this._root.setAttribute('aria-hidden', 'true');
  },

  update(list) {
    if (Array.isArray(list)) {
      const seen = new Set();
      this._list = list.filter(s => s && typeof s.id === 'string' && s.id.trim() && !seen.has(s.id) && seen.add(s.id)).map(s => {
        const activity = s.activity && typeof s.activity === 'object' ? s.activity : {};
        return { id: s.id, title: clean(s.title), project: clean(s.project), cwd: clean(s.cwd), busy: s.busy === true,
          sess_cost_usd: Number.isFinite(s.sess_cost_usd) ? s.sess_cost_usd : 0, sess_cost_known: s.sess_cost_known === true,
          agents_evicted: num(s.agents_evicted), history_trimmed: num(s.history_trimmed),
          kind: clean(activity.kind), flow: normalizeFlow(activity.flow), activity: {
          state: clean(activity.state), task: clean(activity.task), model: clean(activity.model), step: Number.isFinite(activity.step) ? activity.step : null,
          limit: Number.isFinite(activity.limit) ? activity.limit : null, tool: clean(activity.tool), message: clean(activity.message),
          up_tokens: num(activity.up_tokens), down_tokens: num(activity.down_tokens), calls: num(activity.calls),
          tokens_source: clean(activity.tokens_source), cost_usd: Number.isFinite(activity.cost_usd) ? activity.cost_usd : null,
          history: Array.isArray(activity.history) ? activity.history.filter(h => h && typeof h === 'object').map(h => ({
            step: num(h.step), state: clean(h.state), tool: clean(h.tool), message: clean(h.message), at: clean(h.at),
          })) : [],
          started_at: clean(activity.started_at), updated_at: clean(activity.updated_at), agents: Array.isArray(activity.agents) ? activity.agents.filter(validAgent).map(a => ({
            id: a.id, task: clean(a.task), agent: clean(a.agent), status: clean(a.status), model: clean(a.model), step: Number.isFinite(a.step) ? a.step : null,
            max_steps: Number.isFinite(a.max_steps) ? a.max_steps : null, tool: clean(a.tool), message: clean(a.message), output: clean(a.output),
            up_tokens: num(a.up_tokens), down_tokens: num(a.down_tokens), calls: num(a.calls), usage_known: a.usage_known === true,
            started_at: clean(a.started_at), updated_at: clean(a.updated_at),
          })) : [],
          // Role lanes (workshop_roles.go): kept raw, the helpers sanitize.
          role: clean(activity.role), lanes: Array.isArray(activity.lanes) ? activity.lanes : [],
          handoffs: Array.isArray(activity.handoffs) ? activity.handoffs : [],
          role_usage: activity.role_usage && typeof activity.role_usage === 'object' ? activity.role_usage : null,
          lanes_trimmed: num(activity.lanes_trimmed),
        } };
      });
    }
    this._render();
  },

  _showError(message) {
    if (!this._error) return;
    const stale = Boolean(window.gregalPanels?.sessions?._error); const text = message || (stale ? T('workshop.stale') : '');
    this._error.textContent = text; this._error.hidden = !text; this._error.dataset.state = stale && !message ? 'stale' : message ? 'error' : '';
  },

  _sessionState(session) {
    if (pendingIds().has(session.id)) return 'waiting';
    const state = session.activity.state;
    return STATE_KEY[state] ? state : (session.busy ? 'thinking' : 'idle');
  },

  _costSuffix() {
    if (!this._list.length) return '';
    const priced = this._list.filter(s => s.sess_cost_known);
    if (!priced.length) return ` · ${T('workshop.costNA')}`;
    return ` · ${fmtCost(priced.reduce((sum, s) => sum + s.sess_cost_usd, 0))} ${T('workshop.total')}`;
  },

  _freshBucket() {
    const at = window.gregalPanels?.sessions?._lastOkAt || 0;
    if (!at) return '';
    const age = Date.now() - at;
    if (age < 5000) return 'now';
    if (age < 60000) return `s${Math.floor(age / 1000)}`;
    if (age < 3600000) return `m${Math.floor(age / 60000)}`;
    return `h${Math.floor(age / 3600000)}`;
  },

  _freshSuffix() {
    const at = window.gregalPanels?.sessions?._lastOkAt || 0;
    if (!at) return '';
    return ` · ${ageText(Date.now() - at)}`;
  },

  _render() {
    if (!this._visible || !this._root || !this._stage || !this._queue) return;
    const pending = pendingIds(); const current = String(window.gregalSession || 'default');
    const sig = JSON.stringify([this._list, this._list.filter(s => pending.has(s.id)).map(s => s.id), current, String(window.gregalPanels?.sessions?._error || ''), this._freshBucket()]);
    if (sig === this._signature) return; this._signature = sig;
    const waiting = this._list.filter(s => pending.has(s.id));
    const counts = this._list.reduce((acc, s) => { const state = this._sessionState(s); if (LIVE.has(state) || (state === 'waiting' && s.busy)) acc.active++; if (state === 'completed') acc.done++; if (state === 'failed') acc.failed++; return acc; }, { active: 0, done: 0, failed: 0 });
    this._summary.textContent = `${this._list.length} ${T('workshop.sessions')} · ${counts.active} ${T('workshop.active')} · ${counts.done} ${T('workshop.done')}${counts.failed ? ` · ${counts.failed} ${T('workshop.failures')}` : ''}${this._costSuffix()}${this._freshSuffix()}`;
    this._queueHeading.textContent = T('workshop.needs'); this._queueHeading.dataset.count = String(waiting.length);
    this._queueHeading.setAttribute('aria-label', T('workshop.needs') + ': ' + waiting.length); this._queueHeading.setAttribute('aria-live', 'polite');
    this._find.disabled = waiting.length === 0; this._find.textContent = T('workshop.find'); this._find.setAttribute('aria-label', waiting.length ? `${T('workshop.find')} (${waiting.length})` : T('workshop.attention'));

    const ids = new Set(this._list.map(s => s.id));
    for (const [id, node] of this._stations) if (!ids.has(id)) { detach(node); this._stations.delete(id); }
    if (!this._list.length && !this._empty) this._empty = make('div', 'workshop-empty-state');
    if (!this._list.length) {
      if (!this._empty.children.length) { this._empty.append(make('span', 'empty-mark', '✳'), make('h2', '', T('workshop.emptyTitle')), make('p', '', T('workshop.empty'))); }
    }
    const desiredBays = [];
    this._list.forEach((session, index) => {
      let bay = this._stations.get(session.id); if (!bay) { bay = this._createBay(session); this._stations.set(session.id, bay); }
      this._updateBay(bay, session, current, index); desiredBays.push(bay);
    });
    const roomNode = this._room ? null : this._sceneCanvas;
    reconcile(this._stage, [this._window, ...(this._list.length ? desiredBays : [this._empty]), ...(roomNode ? [roomNode] : [])]);
    const hero = this._list.find(s => s.id === current) || this._list[0] || null;
    if (this._sceneSessionId !== (hero ? hero.id : null)) { this._selectedRole = null; this._selectedLane = 0; }
    this._sceneSessionId = hero ? hero.id : null;
    if (this._sceneCanvas) this._sceneCanvas.hidden = !hero;
    if (this._room) this._room.hidden = !hero;
    if (this._scene) {
      const labels = {};
      for (const key of ['thinking', 'tools', 'reviewing', 'compacting', 'working', 'completed', 'failed', 'cancelled', 'waiting', 'idle']) labels[key] = T('workshop.' + key);
      labels.step = T('workshop.stepShort'); labels.delivered = T('workshop.deliveredN');
      this._scene.update(hero, hero ? { waiting: pendingIds().has(hero.id), labels, T } : undefined);
      if (hero && this._selectedRole) this._scene.select('role:' + this._selectedRole);
      else if (hero && this._selectedAgent && this._selectedAgent.sessionId === hero.id) this._scene.select(this._selectedAgent.agentId);
      if (typeof this._scene.highlight === 'function') this._scene.highlight(this._selectedLane || 0);
    }
    this._updateRoom(hero);
    const queueIds = new Set(waiting.map(s => s.id));
    const failed = this._list.filter(s => !pending.has(s.id) && this._sessionState(s) === 'failed');
    for (const s of failed) queueIds.add(s.id);
    for (const [id, node] of this._queueItems) if (!queueIds.has(id)) { detach(node); this._queueItems.delete(id); }
    const desiredQueue = waiting.map(session => this._queueNode(session, ''));
    if (failed.length) {
      if (!this._queueDivider) this._queueDivider = make('div', 'workshop-queue-divider');
      this._queueDivider.textContent = T('workshop.withErrors');
      desiredQueue.push(this._queueDivider, ...failed.map(session => this._queueNode(session, 'is-failed')));
    }
    if (!desiredQueue.length) { if (!this._queueEmpty) this._queueEmpty = make('span', 'workshop-queue-empty', T('workshop.noNeeds')); this._queueEmpty.textContent = T('workshop.noNeeds'); desiredQueue.push(this._queueEmpty); }
    reconcile(this._queueList, desiredQueue);
    this._syncDetail(); this._showError('');
  },

  _queueNode(session, cls) {
    let node = this._queueItems.get(session.id);
    if (!node) { node = make('button', 'workshop-queue-item'); node.type = 'button'; node.append(make('span', 'queue-signal', '●'), make('span', 'workshop-queue-label')); this._queueItems.set(session.id, node); }
    node.dataset.workshopSession = session.id;
    node.className = 'workshop-queue-item' + (cls ? ' ' + cls : '');
    node.setAttribute('aria-label', T('workshop.select') + ': ' + (session.title || session.id));
    node.children[1].textContent = session.title || session.project || session.id;
    return node;
  },

  _createBay(session) {
    const bay = make('article', 'workshop-bay'); bay.dataset.workshopBay = session.id;
    const bayHead = make('div', 'bay-heading');
    const coordinator = make('button', 'workshop-station'); coordinator.type = 'button'; coordinator.dataset.workshopSession = session.id;
    const avatar = make('span', 'coordinator-avatar'); avatar.appendChild(person());
    coordinator.append(make('span', 'coordinator-label', T('workshop.coordinator')), avatar, make('span', 'coordinator-desk'), make('span', 'workshop-name'), make('span', 'workshop-status'), make('span', 'workshop-details'));
    const pulse = make('span', 'signal-route'); pulse.setAttribute('aria-hidden', 'true');
    const bayMeta = make('div', 'bay-meta'); bayMeta.append(make('span', 'bay-model'), make('span', 'bay-step'));
    const bayUsage = make('span', 'bay-usage'); bayUsage.hidden = true;
    const bayAge = make('span', 'bay-age'); bayAge.hidden = true;
    const bayStop = make('button', 'bay-stop', T('workshop.stop')); bayStop.type = 'button'; bayStop.hidden = true; bayStop.dataset.workshopStop = '';
    bayMeta.append(bayUsage, bayAge, bayStop);
    bayHead.append(coordinator, pulse, bayMeta);
    const teamHeading = make('div', 'child-heading'); teamHeading.append(make('h3', '', T('workshop.childAgents')), make('span', 'child-count'));
    const graph = make('section', 'bay-graph'); graph.hidden = true;
    const children = make('div', 'child-agents');
    const empty = make('p', 'no-children', T('workshop.noChildren'));
    const history = make('section', 'bay-history'); history.hidden = true;
    const historyHeading = make('h3', 'history-heading', T('workshop.history'));
    const historyList = make('ol', 'history-list');
    history.append(historyHeading, historyList);
    bay.append(bayHead, teamHeading, graph, children, empty, history);
    bay._coordinator = coordinator; bay._bayModel = bayMeta.children[0]; bay._bayStep = bayMeta.children[1]; bay._bayUsage = bayUsage; bay._bayAge = bayAge; bay._stop = bayStop; bay._graph = graph; bay._graphSvg = null; bay._graphSig = ''; bay._children = children; bay._noChildren = empty; bay._history = history; bay._historyList = historyList; bay._historySig = ''; bay._agentNodes = new Map();
    return bay;
  },

  _updateBay(bay, session, current, index) {
    const state = this._sessionState(session); const activity = session.activity; const coordinator = bay._coordinator;
    bay.style.setProperty('--bay-index', String(index)); bay.dataset.state = state;
    coordinator.dataset.state = state; coordinator.className = 'workshop-station' + (current === session.id ? ' is-selected' : '');
    coordinator.setAttribute('aria-current', current === session.id ? 'true' : 'false');
    coordinator.setAttribute('aria-label', `${T('workshop.openParent')}: ${session.title || session.project || session.id}, ${keyState(state)}`);
    coordinator.children[0].textContent = T('workshop.coordinator');
    coordinator.children[3].textContent = session.title || session.project || session.id;
    coordinator.children[4].textContent = keyState(state);
    coordinator.children[5].textContent = activity.task || activity.message || session.project || session.cwd || '';
    bay._bayModel.textContent = activity.model ? `${T('workshop.model')}: ${activity.model}` : T('workshop.mainAgent');
    bay._bayStep.textContent = activity.step != null ? `${T('workshop.step')} ${activity.step}${activity.limit != null ? ` / ${activity.limit} ${T('workshop.budget')}` : ''}` : '';
    const usage = usageText({ up: activity.up_tokens, down: activity.down_tokens, calls: activity.calls, known: activity.tokens_source === 'api' || activity.tokens_source === 'mixed', cost: activity.cost_usd });
    bay._bayUsage.textContent = usage; bay._bayUsage.hidden = !usage;
    const seen = activity.updated_at ? new Date(activity.updated_at) : null;
    const ageMs = seen && !Number.isNaN(seen.getTime()) ? Date.now() - seen.getTime() : -1;
    // Only live or waiting turns get an age: telling a fresh "thinking"
    // from a two-hour one (or how long a human has been awaited) apart.
    const showAge = ageMs >= 0 && (LIVE.has(state) || state === 'waiting');
    bay._bayAge.textContent = showAge ? ageText(ageMs) : ''; bay._bayAge.hidden = !showAge;
    bay._stop.textContent = T('workshop.stop'); bay._stop.disabled = false;
    bay._stop.hidden = !session.busy; bay._stop.dataset.workshopSession = session.id;
    bay._stop.setAttribute('aria-label', `${T('workshop.stop')}: ${session.title || session.project || session.id}`);
    const valid = new Set(activity.agents.map(a => a.id));
    for (const [id, node] of bay._agentNodes) if (!valid.has(id)) { detach(node); bay._agentNodes.delete(id); }
    const desiredAgents = [];
    activity.agents.forEach((agent, index) => {
      let node = bay._agentNodes.get(agent.id); if (!node) { node = this._createAgent(session.id, agent); bay._agentNodes.set(agent.id, node); }
      this._updateAgent(node, session.id, agent, index); desiredAgents.push(node);
    });
    reconcile(bay._children, desiredAgents);
    const working = activity.agents.filter(a => a.status === 'working').length;
    const done = activity.agents.filter(a => a.status === 'completed').length;
    const failed = activity.agents.filter(a => a.status === 'failed').length;
    const countParts = [`${activity.agents.length} ${T('workshop.agents')}`];
    if (working) countParts.push(`${working} ${T('workshop.active')}`);
    if (done) countParts.push(`${done} ${T('workshop.done')}`);
    if (failed) countParts.push(`${failed} ${T('workshop.failures')}`);
    if (session.agents_evicted > 0) countParts.push(T('workshop.older').replace('{n}', String(session.agents_evicted)));
    bay.children[1].children[1].textContent = countParts.join(' · ');
    bay._noChildren.hidden = activity.agents.length > 0;
    this._updateHistory(bay, activity.history, session.history_trimmed);
    this._updateGraph(bay, session);
  },

  _updateGraph(bay, session) {
    const flow = session.kind === 'team' ? session.flow : null;
    // The import is stripped in the node test harness: without the shared
    // graph module the room simply shows no graph (as before this change).
    if (!flow || typeof buildFlowGraph !== 'function') { bay._graph.hidden = true; return; }
    const nodes = flow.nodes, edges = flow.edges;
    bay._graph.hidden = nodes.length === 0;
    if (!nodes.length) return;
    const sig = JSON.stringify([nodes.map(n => n.id), edges]);
    let svg = bay._graphSvg;
    if (!svg || bay._graphSig !== sig) {
      if (svg) svg.remove();
      svg = buildFlowGraph(nodes, edges, agentId => this.selectAgent(session.id, agentId));
      bay._graph.append(svg); bay._graphSvg = svg; bay._graphSig = sig;
    }
    const states = {};
    for (const a of session.activity.agents) states[a.id] = a.status;
    const selected = this._selectedAgent && this._selectedAgent.sessionId === session.id ? this._selectedAgent.agentId : null;
    updateFlowGraph(svg, nodes, {
      states, selected, liveEdge: null,
      shortName: id => id, fullName: id => id, stateName: state => keyState(state),
    });
  },

  _updateHistory(bay, history, trimmed) {
    bay._history.hidden = !history.length;
    bay._history.querySelector('.history-heading').textContent = T('workshop.history') + (trimmed > 0 ? ` · ${T('workshop.older').replace('{n}', String(trimmed))}` : '');
    const sig = JSON.stringify(history);
    if (sig === bay._historySig) return;
    bay._historySig = sig;
    const top = bay._historyList.scrollTop || 0;
    bay._historyList.replaceChildren(...history.map(entry => {
      const item = make('li', 'history-item');
      // Step budgets don't exist for every run kind (team pipelines have
      // none): a zero step shows no marker instead of a bogus "#0".
      if (entry.step > 0) item.append(make('span', 'history-step', `#${entry.step}`));
      const what = make('span', 'history-what', keyState(entry.state) + (entry.tool ? ` · ${entry.tool}` : '') + (entry.message ? ` — ${entry.message}` : ''));
      item.append(what);
      const date = entry.at ? new Date(entry.at) : null;
      if (date && !Number.isNaN(date.getTime())) item.append(make('time', 'history-time', date.toLocaleTimeString()));
      return item;
    }));
    bay._historyList.scrollTop = top;
  },

  _createAgent(sessionId, agent) {
    const button = make('button', 'child-agent'); button.type = 'button'; button.dataset.workshopSession = sessionId; button.dataset.workshopAgent = agent.id;
    const portrait = make('span', 'child-portrait'); portrait.appendChild(person());
    const desk = make('span', 'child-desk'); desk.append(make('span', 'child-monitor'), make('span', 'child-keyboard'));
    const info = make('span', 'child-info'); info.append(make('span', 'child-task'), make('span', 'child-state'), make('span', 'child-facts'));
    const link = make('span', 'courier-link'); link.setAttribute('aria-hidden', 'true');
    button.append(portrait, desk, info, link); return button;
  },

  _updateAgent(node, sessionId, agent, index) {
    const state = CHILD_KEY[agent.status] || 'idle'; node.dataset.state = state; node.dataset.workshopSession = sessionId; node.dataset.workshopAgent = agent.id;
    node.style.setProperty('--agent-index', String(index));
    // A user-defined subagent shows its name: it says more than the prompt.
    const title = agent.agent ? `${agent.agent} · ${agent.task || agent.id}` : (agent.task || agent.id);
    node.setAttribute('aria-label', `${T('workshop.agentDetails')}: ${title}, ${keyState(state)}`);
    node.children[2].children[0].textContent = title;
    node.children[2].children[1].textContent = keyState(state);
    const facts = [agent.model, agent.tool, agent.step != null ? `${T('workshop.step')} ${agent.step}${agent.max_steps != null ? ` / ${agent.max_steps} ${T('workshop.budget')}` : ''}` : '', usageText({ up: agent.up_tokens, down: agent.down_tokens, calls: agent.calls, known: agent.usage_known })].filter(Boolean);
    node.children[2].children[2].textContent = facts.join(' · ');
  },

  // Role view of the hero session: legend, step strip and role detail.
  _roomActivity(hero) {
    if (!hero || hero.kind === 'team') return null;
    const R = RA(); if (!R) return null;
    return R.normalize({ ...hero.activity, state: this._sessionState(hero) });
  },

  _updateRoom(hero) {
    if (!this._room) return;
    const R = RA(); const act = this._roomActivity(hero);
    this._legend.hidden = !act; this._strip.hidden = !act;
    if (!act) { this._roleDetail.hidden = true; return; }
    const states = R.roleStates(act);
    const sig = JSON.stringify([hero.id, act.lanes, act.handoffs, states, act.usage, act.agents.map(a => [a.id, a.status, a.output]), this._selectedRole, this._selectedLane, String(T('workshop.role.coordinator'))]);
    if (sig === this._roomSig) return; this._roomSig = sig;
    this._legend.replaceChildren(...R.ROLES.map(role => {
      const b = make('button', 'room-role'); b.type = 'button'; b.dataset.roomRole = role; b.dataset.role = role; b.dataset.state = states[role];
      b.setAttribute('aria-pressed', this._selectedRole === role ? 'true' : 'false');
      b.append(make('span', 'room-role-dot'), make('span', 'room-role-name', T('workshop.role.' + role)), make('span', 'room-role-state', T('workshop.roleState.' + states[role])));
      return b;
    }));
    const items = R.timeline(act).map(seg => {
      const lane = act.lanes.find(l => l.seq === seg.seq);
      const b = make('button', 'room-step'); b.type = 'button'; b.dataset.roomLane = String(seg.seq); b.dataset.role = seg.role;
      b.setAttribute('role', 'listitem');
      if (seg.result) b.dataset.result = seg.result;
      if (!seg.done) b.dataset.live = 'true';
      const text = `${T('workshop.role.' + seg.role)} · ${R.laneText(lane, T)}${seg.duration_ms ? ' · ' + R.fmtDuration(seg.duration_ms) : ''}`;
      b.title = text; b.setAttribute('aria-label', text);
      b.setAttribute('aria-pressed', this._selectedLane === seg.seq ? 'true' : 'false');
      b.style.flexGrow = String(Math.max(1, Math.min(6, Math.round((seg.duration_ms || 1500) / 4000))));
      return b;
    });
    if (!items.length) items.push(make('span', 'room-strip-empty', T('workshop.timelineEmpty')));
    this._strip.replaceChildren(...items);
    this._renderRoleDetail(hero, act);
  },

  _renderRoleDetail(hero, act) {
    const box = this._roleDetail; const R = RA();
    if (!this._selectedRole || !R) { box.hidden = true; return; }
    const role = this._selectedRole; const sum = R.roleSummary(act, role);
    box.hidden = false; box.dataset.role = role;
    const head = make('header', 'room-detail-head');
    const title = make('h3', 'room-detail-title', T('workshop.role.' + role)); title.id = 'roomDetailTitle';
    const close = make('button', 'room-detail-close', '×'); close.type = 'button'; close.dataset.roomClose = ''; close.setAttribute('aria-label', T('workshop.close'));
    const facts = [T('workshop.roleState.' + sum.state)];
    if (sum.lanes.length) facts.push(sum.lanes.length === 1 ? T('workshop.stepOne') : T('workshop.stepsN').replace('{n}', String(sum.lanes.length)));
    if (sum.durationMs) facts.push(R.fmtDuration(sum.durationMs));
    if (sum.usage) { const u = usageText({ up: sum.usage.up, down: sum.usage.down, calls: sum.usage.calls, known: sum.usage.up > 0 || sum.usage.down > 0 }); if (u) facts.push(u); }
    head.append(title, make('p', 'room-detail-facts', facts.join(' · ')), close);
    box.setAttribute('aria-labelledby', 'roomDetailTitle');
    const parts = [head];
    if (!sum.lanes.length && !sum.children.length) parts.push(make('p', 'room-detail-empty', T('workshop.roleNoWork')));
    // Real researcher children (the scout): their brief and summary.
    for (const child of sum.children) {
      const card = make('div', 'room-child');
      card.append(make('h4', '', `${T('workshop.realChild')} · ${child.agent || child.id} · ${keyState(CHILD_KEY[child.status] || 'idle')}`));
      if (child.task) card.append(make('span', 'room-label', T('workshop.brief')), make('p', 'room-child-brief', child.task));
      if (child.output) card.append(make('span', 'room-label', T('workshop.childSummary')), make('p', 'room-child-out', child.output));
      parts.push(card);
    }
    if (sum.lanes.length) {
      parts.push(make('h4', 'room-section', T('workshop.actions')));
      const list = make('ol', 'room-actions');
      for (const lane of sum.lanes.slice(-14)) {
        const li = make('li', 'room-action' + (lane.seq === this._selectedLane ? ' is-selected' : '')); li.dataset.roomLane = String(lane.seq);
        if (lane.result) li.dataset.result = lane.result;
        if (lane.step > 0) li.append(make('span', 'room-action-step', '#' + lane.step));
        li.append(make('span', 'room-action-text', R.laneText(lane, T)));
        const meta = [lane.done ? R.fmtDuration(lane.duration_ms) : T('workshop.running'), lane.result === 'ok' ? '✓' : lane.result === 'fail' ? '✗' : ''].filter(Boolean).join(' ');
        li.append(make('span', 'room-action-meta', meta));
        list.append(li);
      }
      parts.push(list);
    }
    if (sum.files.length) {
      parts.push(make('h4', 'room-section', T('workshop.files')));
      const files = make('div', 'room-files');
      for (const f of sum.files.slice(0, 16)) {
        const b = make('button', 'room-file', R.shortPath(f.path, 40)); b.type = 'button'; b.dataset.roomFile = f.path; b.title = f.path;
        files.append(b);
      }
      if (role === 'builder') { const c = make('button', 'room-changes', T('workshop.openChanges')); c.type = 'button'; c.dataset.roomChanges = ''; files.append(c); }
      parts.push(files);
    }
    if (sum.commands.length) {
      parts.push(make('h4', 'room-section', T('workshop.commands')));
      const list = make('ul', 'room-commands');
      for (const c of sum.commands.slice(-8)) {
        const li = make('li', 'room-command'); if (c.result) li.dataset.result = c.result;
        li.append(make('code', '', c.command), make('span', 'room-command-meta', [c.result === 'ok' ? T('workshop.exitOk') : c.result === 'fail' ? T('workshop.exitFail') : T('workshop.running'), R.fmtDuration(c.duration_ms)].filter(Boolean).join(' · ')));
        list.append(li);
      }
      parts.push(list);
    }
    box.replaceChildren(...parts);
  },

  selectRole(sessionId, role) {
    const R = RA(); if (!R || !R.ROLES.includes(role)) return;
    this._selectedRole = role; this._selectedLane = 0; this._roomSig = '';
    if (this._scene) { this._scene.select('role:' + role); if (typeof this._scene.highlight === 'function') this._scene.highlight(0); }
    this._updateRoom(this._list.find(s => s.id === sessionId) || null);
    if (this._roleDetail && !this._roleDetail.hidden) this._roleDetail.scrollIntoView?.({ block: 'nearest', behavior: 'smooth' });
  },

  selectLane(seq) {
    const hero = this._list.find(s => s.id === this._sceneSessionId); const act = this._roomActivity(hero);
    const lane = act && act.lanes.find(l => l.seq === seq); if (!lane) return;
    this._selectedLane = this._selectedLane === seq ? 0 : seq; this._selectedRole = lane.role; this._roomSig = '';
    if (this._scene) { this._scene.select('role:' + lane.role); if (typeof this._scene.highlight === 'function') this._scene.highlight(this._selectedLane); }
    this._updateRoom(hero);
  },

  closeRole() {
    const role = this._selectedRole; this._selectedRole = null; this._selectedLane = 0; this._roomSig = '';
    if (this._scene) { this._scene.select(''); if (typeof this._scene.highlight === 'function') this._scene.highlight(0); }
    this._updateRoom(this._list.find(s => s.id === this._sceneSessionId) || null);
    const back = role && this._legend ? Array.from(this._legend.children || []).find(b => b.dataset && b.dataset.roomRole === role) : null;
    back?.focus?.();
  },

  openFile(path) {
    if (!path) return;
    const files = window.gregalPanels && window.gregalPanels.files;
    if (files && typeof files.view === 'function') files.view(path);
  },

  openChanges() {
    G()?.setView?.('canvis');
    const changes = window.gregalPanels && window.gregalPanels.changes;
    if (changes && typeof changes.load === 'function') changes.load();
  },

  selectAgent(sessionId, agentId) {
    const session = this._list.find(s => s.id === sessionId); const agent = session?.activity.agents.find(a => a.id === agentId);
    if (!agent) return; this._selectedAgent = { sessionId, agentId }; this._syncDetail();
    this._detail.hidden = false; this._detailClose.focus?.();
    // On small screens the detail lives below the stage: bring it up.
    if (window.innerWidth < 760) this._detail.scrollIntoView?.({ block: 'nearest' });
  },

  closeAgent() {
    const selection = this._selectedAgent;
    this._selectedAgent = null;
    if (this._detail) this._detail.hidden = true;
    const target = selection && this._stations.get(selection.sessionId)?._agentNodes.get(selection.agentId);
    target?.focus?.();
  },

  _syncDetail() {
    if (!this._detail) return;
    if (!this._selectedAgent) { this._detail.hidden = true; return; }
    const session = this._list.find(s => s.id === this._selectedAgent.sessionId); const agent = session?.activity.agents.find(a => a.id === this._selectedAgent.agentId);
    if (!session || !agent) { this._selectedAgent = null; this._detail.hidden = true; return; }
    this._detail.hidden = false; this._detailTitle.textContent = agent.agent ? `${agent.agent} · ${agent.task || agent.id}` : (agent.task || agent.id);
    this._detailClose.textContent = T('workshop.close');
    this._detailStatus.textContent = `${T('workshop.status')}: ${keyState(agent.status)}`;
    const values = {
      parent: session.title || session.project || session.id, model: agent.model, tool: agent.tool,
      step: agent.step == null ? '' : `${agent.step}${agent.max_steps != null ? ` / ${agent.max_steps} ${T('workshop.budget')}` : ''}`,
      usage: usageText({ up: agent.up_tokens, down: agent.down_tokens, calls: agent.calls, known: agent.usage_known }),
      message: agent.message, output: agent.output,
    };
    for (const [key, field] of this._detailFields) {
      field.dt.textContent = T('workshop.' + key);
      (field.text || field.dd).textContent = values[key];
      field.dt.hidden = !values[key]; field.dd.hidden = !values[key];
      // Long message/output stay clamped to 4 lines with an expander that
      // survives polls: only fresh long content re-clamps.
      if (field.more) {
        const long = (values[key] || '').length > 400;
        if (!long) { field.text.className = 'detail-text'; field.more.hidden = true; }
        else {
          if (field.more.hidden) field.text.className = 'detail-text detail-clamp';
          field.more.hidden = false;
        }
        field.more.textContent = field.text.className === 'detail-text detail-clamp' ? T('workshop.more') : T('workshop.less');
      }
    }
    this._detailOpen.textContent = T('workshop.openParent'); this._detailOpen.dataset.workshopSession = session.id;
  },

  selectWaiting() {
    const ids = this._list.filter(s => pendingIds().has(s.id)).map(s => s.id);
    if (!ids.length) { this._showError(T('workshop.noNeeds')); return; }
    const at = (ids.indexOf(this._lastWaitingId) + 1) % ids.length;
    this._lastWaitingId = ids[at];
    return this.select(ids[at]);
  },

  async stopSession(id, button) {
    if (typeof id !== 'string' || !id) return;
    const Gref = G(); if (!Gref || typeof Gref.api !== 'function') return;
    if (button) button.disabled = true;
    try {
      const response = await Gref.api('/api/agent/cancel', { method: 'POST', body: '{}', headers: { 'X-Gregal-Session': id } });
      if (!response || !response.ok) throw new Error('HTTP ' + (response ? response.status : '?'));
      // Un torn només encuat no es pot aturar per aquí (cancelled:false):
      // dir-ho, no fer veure que s'ha aturat mentre crema tokens.
      const body = await response.json().catch(() => null);
      if (!body || body.cancelled !== true) throw new Error(T('workshop.stopQueued'));
    } catch (error) {
      if (button) button.disabled = false;
      this._showError((error && error.message) || T('workshop.loadError'));
    }
  },

  async select(id) {
    if (typeof id !== 'string' || !this._list.some(s => s.id === id)) return;
    const version = ++this._selectionVersion; this._showError('');
    try {
      const panels = window.gregalPanels; if (!panels || !panels.sessions || typeof panels.sessions.switchTo !== 'function') throw new Error(T('workshop.loadError'));
      await panels.sessions.switchTo(id); if (version !== this._selectionVersion || !this._visible) return; G()?.setView?.('agent');
    } catch (error) { if (version !== this._selectionVersion || !this._visible) return; this._showError((error && error.message) || T('workshop.loadError')); }
  },
};
