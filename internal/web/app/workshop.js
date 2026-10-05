// Taller 2D local: una estació per sessió viva, amb la cua d'atenció real
// de la finestra. El sondeig general de sessions el gestiona panels.js.
import { t as T } from './i18n.js';

const G = () => window.gregal;
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

// SVG dibuixat per Gregal; cap valor del servidor s'insereix al markup.
const PERSON = '<svg class="workshop-person" viewBox="0 0 64 82" aria-hidden="true" focusable="false"><path class="person-hair" d="M19 19c0-10 6-16 14-16s14 6 14 16v9H19z"/><circle class="person-face" cx="33" cy="24" r="13"/><circle class="person-eyes" cx="28" cy="23" r="1.2"/><circle class="person-eyes" cx="38" cy="23" r="1.2"/><path class="person-smile" d="M29 29q4 4 8 0"/><path class="person-neck" d="M28 35h10v8H28z"/><path class="person-shirt" d="M18 43l10-4h10l10 4 8 29H10z"/><path class="person-arm" d="M18 46l-7 18 6 3 10-15m18-6 7 18-6 3-10-15"/><path class="person-dots" d="M29 51h2m6 0h2"/></svg>';

export const workshop = {
  _root: null,
  _stage: null,
  _window: null,
  _queue: null,
  _queueList: null,
  _queueHeading: null,
  _summary: null,
  _error: null,
  _find: null,
  _list: [],
  _signature: '',
  _initialized: false,
  _visible: false,
  _loadVersion: 0,
  _selectionVersion: 0,
  _abort: null,

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
    this._window = $('.workshop-window', root);
    if (!this._window) {
      this._window = make('div', 'workshop-window');
    }
    this._window.setAttribute('aria-hidden', 'true');
    this._stage.appendChild(this._window);
    if (!this._queue) { this._queue = make('div', 'workshop-queue'); this._queue.dataset.workshopQueue = ''; root.appendChild(this._queue); }
    this._queueHeading = $('.workshop-needs', root);
    this._summary = $('.workshop-summary', root);
    if (!this._queueHeading) { this._queueHeading = make('h2', 'workshop-needs', T('workshop.needs')); this._queue.appendChild(this._queueHeading); }
    if (!this._summary) { this._summary = make('p', 'workshop-summary'); this._queue.appendChild(this._summary); }
    this._queueList = $('.workshop-queue-list', this._queue);
    if (!this._queueList) { this._queueList = make('div', 'workshop-queue-list'); this._queue.appendChild(this._queueList); }
    if (!this._error) {
      this._error = make('div', 'workshop-error'); this._error.dataset.workshopError = ''; this._error.setAttribute('role', 'alert');
      (this._root.querySelector('.workshop-header') || root).appendChild(this._error);
    }
    if (!this._find) {
      this._find = make('button', 'workshop-attention', T('workshop.find'));
      this._find.type = 'button'; this._find.dataset.workshopFind = '';
      root.appendChild(this._find);
    }
    this._find.type = 'button';
    this._find.setAttribute('aria-label', T('workshop.find'));
    root.setAttribute('aria-hidden', this._visible ? 'false' : 'true');
    root.addEventListener('click', e => {
      const target = e.target && e.target.closest ? e.target.closest('[data-workshop-session]') : null;
      if (target && root.contains(target)) {
        e.preventDefault();
        this.select(target.dataset.workshopSession);
        return;
      }
      const find = e.target && e.target.closest ? e.target.closest('[data-workshop-find]') : null;
      if (find && root.contains(find)) { e.preventDefault(); this.selectWaiting(); }
    });
    this._initialized = true;
    if (typeof document.addEventListener === 'function') {
      document.addEventListener('gregal:idioma', () => { this._signature = ''; this._render(); });
    }
    this._render();
    return this;
  },

  async show() {
    this.init();
    if (!this._root) return;
    if (this._visible) return;
    this._visible = true;
    const version = ++this._loadVersion;
    this._selectionVersion++;
    this._root.setAttribute('aria-hidden', 'false');
    this._render();
    const Gref = G();
    if (!Gref || typeof Gref.api !== 'function') return;
    const controller = typeof AbortController === 'function' ? new AbortController() : null;
    this._abort = controller;
    try {
      const response = await Gref.api('/api/sessions/live', controller ? { signal: controller.signal } : undefined);
      if (version !== this._loadVersion || !this._visible || G() !== Gref) return;
      if (!response || !response.ok) throw new Error(response ? 'HTTP ' + response.status : T('workshop.loadError'));
      const body = await response.json();
      if (version !== this._loadVersion || !this._visible || G() !== Gref) return;
      this.update(Array.isArray(body && body.sessions) ? body.sessions : []);
      this._showError('');
    } catch (error) {
      if (version !== this._loadVersion || !this._visible || (error && error.name === 'AbortError')) return;
      this._showError(T('workshop.loadError') + (error && error.message ? ': ' + error.message : ''));
    } finally {
      if (version === this._loadVersion) this._abort = null;
    }
  },

  hide() {
    this._visible = false;
    ++this._loadVersion;
    ++this._selectionVersion;
    if (this._abort) { try { this._abort.abort(); } catch (e) {} this._abort = null; }
    if (this._root) this._root.setAttribute('aria-hidden', 'true');
  },

  update(list) {
    if (Array.isArray(list)) {
      const seen = new Set();
      this._list = list.filter(s => s && typeof s.id === 'string' && s.id.trim() && !seen.has(s.id) && seen.add(s.id))
        .map(s => ({ id: s.id, title: typeof s.title === 'string' ? s.title : '', project: typeof s.project === 'string' ? s.project : '', cwd: typeof s.cwd === 'string' ? s.cwd : '', busy: s.busy === true }));
    }
    this._render();
  },

  _showError(message) {
    if (!this._error) return;
    const stale = Boolean(window.gregalPanels?.sessions?._error);
    const text = message || (stale ? T('workshop.stale') : '');
    this._error.textContent = text;
    this._error.hidden = !text;
    this._error.dataset.state = stale && !message ? 'stale' : message ? 'error' : '';
  },

  _render() {
    if (!this._visible || !this._root || !this._stage || !this._queue) return;
    const waiting = pendingIds();
    const current = String(window.gregalSession || 'default');
    const signature = JSON.stringify({
      list: this._list.map(s => [s.id, s.title, s.project, s.cwd, s.busy]),
      waiting: this._list.filter(s => waiting.has(s.id)).map(s => s.id),
      current,
      error: String(window.gregalPanels?.sessions?._error || ''),
    });
    if (signature === this._signature) return;
    this._signature = signature;
    this._stage.replaceChildren(this._window);
    this._queueList.replaceChildren();
    const needs = this._list.filter(s => waiting.has(s.id));
    this._summary.textContent = this._list.length + ' · ' + T('workshop.summary');
    this._queueHeading.textContent = T('workshop.needs');
    this._queueHeading.dataset.count = String(needs.length);
    this._queueHeading.setAttribute('aria-label', T('workshop.needs') + ': ' + needs.length);
    this._queueHeading.setAttribute('aria-live', 'polite');
    this._find.disabled = needs.length === 0;
    this._find.textContent = T('workshop.find');
    this._find.setAttribute('aria-label', needs.length ? T('workshop.find') + ' (' + needs.length + ')' : T('workshop.attention'));

    if (!this._list.length) {
      const empty = make('p', 'workshop-empty', T('workshop.empty'));
      this._stage.appendChild(empty);
    }
    this._list.forEach((session, index) => {
      const isWaiting = waiting.has(session.id);
      const state = isWaiting ? 'waiting' : session.busy ? 'busy' : 'idle';
      const button = make('button', 'workshop-station' + (current === session.id ? ' is-selected' : ''));
      button.type = 'button';
      button.dataset.workshopSession = session.id;
      button.dataset.state = state;
      button.setAttribute('aria-current', current === session.id ? 'true' : 'false');
      button.setAttribute('aria-label', T('workshop.select') + ': ' + (session.title || session.project || session.id) + ', ' + T('workshop.' + state));
      button.style.setProperty('--station-index', String(index));
      const figure = make('span', 'workshop-figure');
      const person = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
      person.setAttribute('class', 'workshop-person');
      person.setAttribute('viewBox', '0 0 64 82'); person.setAttribute('aria-hidden', 'true'); person.setAttribute('focusable', 'false');
      person.innerHTML = PERSON.replace(/^<svg[^>]*>|<\/svg>$/g, '');
      figure.appendChild(person);
      const desk = make('span', 'workshop-desk');
      desk.setAttribute('aria-hidden', 'true');
      const screen = make('span', 'workshop-screen');
      screen.setAttribute('aria-hidden', 'true');
      desk.appendChild(screen);
      const name = make('span', 'workshop-name', session.title || session.project || session.id);
      const status = make('span', 'workshop-status', T('workshop.' + state));
      const details = make('span', 'workshop-details', session.project || session.cwd || '');
      button.append(figure, desk, name, status, details);
      this._stage.appendChild(button);
    });
    if (needs.length) {
      needs.forEach(session => {
        const button = make('button', 'workshop-queue-item');
      button.type = 'button';
      button.dataset.workshopSession = session.id;
      button.setAttribute('aria-label', T('workshop.select') + ': ' + (session.title || session.id));
        const marker = make('span', 'workshop-queue-marker', '●'); marker.setAttribute('aria-hidden', 'true');
        const label = make('span', 'workshop-queue-label', session.title || session.project || session.id);
        button.append(marker, label);
        this._queueList.appendChild(button);
      });
    } else {
      this._queueList.appendChild(make('span', 'workshop-queue-empty', T('workshop.noNeeds')));
    }
    this._showError('');
  },

  selectWaiting() {
    const pending = pendingIds();
    const session = this._list.find(s => pending.has(s.id));
    if (session) return this.select(session.id);
    this._showError(T('workshop.noNeeds'));
  },

  async select(id) {
    if (typeof id !== 'string' || !this._list.some(s => s.id === id)) return;
    const version = ++this._selectionVersion;
    this._showError('');
    try {
      const panels = window.gregalPanels;
      if (!panels || !panels.sessions || typeof panels.sessions.switchTo !== 'function') throw new Error(T('workshop.loadError'));
      await panels.sessions.switchTo(id);
      if (version !== this._selectionVersion || !this._visible) return;
      G()?.setView?.('agent');
    } catch (error) {
      if (version !== this._selectionVersion || !this._visible) return;
      this._showError((error && error.message) || T('workshop.loadError'));
    }
  },
};
