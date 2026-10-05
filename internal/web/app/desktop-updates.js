// Updates live in Preferences and use only the narrow, pinned Electron bridge.
// The ordinary browser UI never advertises an update capability.
const esc = value => String(value == null ? '' : value)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
const translate = key => typeof window.gregalT === 'function' ? window.gregalT(key) : key;
const format = (key, values = {}) => Object.entries(values).reduce(
  (text, [name, value]) => text.replaceAll('{' + name + '}', String(value)), translate(key));
const REASONS = {
  disabled: 'prefs.update.reason.disabled',
  development: 'prefs.update.reason.development',
  portable: 'prefs.update.reason.portable',
  'unsupported-platform': 'prefs.update.reason.unsupported-platform',
  'missing-metadata': 'prefs.update.reason.missing-metadata',
  'updater-unavailable': 'prefs.update.reason.updater-unavailable',
};

export const desktopUpdates = {
  bridge: null,
  status: { state: 'idle' },
  root: null,
  unsubscribe: null,
  initialized: false,

  init() {
    const bridge = window.gregalDesktop;
    if (!bridge || typeof bridge.getUpdateStatus !== 'function' || this.initialized) return;
    this.bridge = bridge;
    this.initialized = true;
    const revision = this._revision || 0;
    if (typeof bridge.onUpdate === 'function') {
      this.unsubscribe = bridge.onUpdate(status => this.setStatus(status));
    }
    document.addEventListener('gregal:idioma', () => this.updateBadge());
    // Subscribe first so an event arriving while the snapshot is in flight wins.
    Promise.resolve().then(() => bridge.getUpdateStatus()).then(status => {
      if ((this._revision || 0) === revision) this.setStatus(status);
    }).catch(error => {
      if ((this._revision || 0) === revision) this.setStatus({ state: 'error', reason: error?.message || String(error) });
    });
  },

  section() {
    if (!this.bridge) return '';
    return '<h3 class="pf-sec">' + esc(translate('prefs.updates')) + '</h3>' +
      '<div class="pf-update" id="desktopUpdateContent"></div>';
  },

  bind(parent) {
    if (!this.bridge || !parent) return;
    this.root = parent.querySelector('#desktopUpdateContent');
    this.paint();
  },

  setStatus(status) {
    if (!status || typeof status !== 'object') return;
    this._revision = (this._revision || 0) + 1;
    this.status = { ...this.status, ...status };
    this.updateBadge();
    this.paint();
  },

  updateBadge() {
    const button = document.getElementById('prefsBtn');
    if (!button) return;
    const pending = this.status.state === 'available' || this.status.state === 'ready';
    button.classList.toggle('has-update', pending);
    const label = pending ? translate('prefs.update.notice') : translate('prefs.title');
    button.title = label;
    button.setAttribute('aria-label', label);
  },

  message() {
    const state = this.status.state || 'idle';
    const version = this.status.version || '';
    switch (state) {
      case 'checking': return translate('prefs.update.checking');
      case 'available': return format('prefs.update.available', { version: version || '—' });
      case 'downloading': return format('prefs.update.downloading', { version: version || '—' });
      case 'ready': return format('prefs.update.ready', { version: version || '—' });
      case 'installing': return translate('prefs.update.installing');
      case 'up-to-date': return translate('prefs.update.current');
      case 'unsupported': return translate(this.status.mode === 'portable' ? 'prefs.update.portable' : 'prefs.update.unsupported');
      case 'error': return translate('prefs.update.error');
      default: return translate('prefs.update.idle');
    }
  },

  content() {
    const state = this.status.state || 'idle';
    const mode = this.status.mode || 'manual';
    const version = this.status.currentVersion || this.bridge.version || '—';
    const progress = Number(this.status.progress);
    const busy = state === 'checking' || state === 'downloading' || state === 'installing';
    let action = null;
    if (state === 'ready') action = ['install', 'prefs.update.install'];
    else if (state === 'downloading') action = ['download', 'prefs.update.busy'];
    else if (state === 'installing') action = ['install', 'prefs.update.busy'];
    else if (state === 'available' && mode === 'portable') action = ['open', 'prefs.update.releases'];
    else if (state === 'available') action = ['download', 'prefs.update.download'];
    else if (state === 'unsupported' || state === 'error' || mode === 'manual') action = ['open', 'prefs.update.releases'];
    else action = ['check', 'prefs.update.check'];

    const showProgress = state === 'downloading';
    const progressMarkup = showProgress
      ? '<progress class="pf-update-progress" max="100"' + (Number.isFinite(progress) ? ' value="' + Math.max(0, Math.min(100, progress)) + '"' : '') + ' aria-label="' + esc(translate('prefs.update.progress')) + '"></progress>' +
        (Number.isFinite(progress) ? '<span class="pf-update-percent">' + Math.round(progress) + '%</span>' : '')
      : '';
    const detail = state === 'error' && this.status.reason
      ? '<div class="pf-update-error-detail">' + esc(this.status.reason) + '</div>' : '';
    const retry = state === 'error'
      ? '<button class="pf-open" data-update-action="check">' + esc(translate('prefs.update.check')) + '</button>' : '';
    const reason = state === 'unsupported' && REASONS[this.status.reason]
      ? '<div class="pf-update-help">' + esc(translate(REASONS[this.status.reason])) + '</div>' : '';
    return '<div class="pf-update-row"><div class="pf-lab"><b>' + esc(translate('prefs.update.version')) + '</b>' +
      '<span>' + esc(version) + '</span></div><div class="pf-update-actions">' +
      '<button class="pf-open pf-update-primary" data-update-action="' + action[0] + '"' + (busy ? ' disabled' : '') + '>' + esc(translate(action[1])) + '</button>' + retry +
      '</div></div><div class="pf-update-message" role="status" aria-live="polite">' + esc(this.message()) + '</div>' +
      (progressMarkup ? '<div class="pf-update-progress-row">' + progressMarkup + '</div>' : '') + detail + reason;
  },

  paint() {
    if (!this.root) return;
    const active = this.root.contains(document.activeElement)
      ? document.activeElement?.dataset?.updateAction : '';
    this.root.innerHTML = this.content();
    this.root.querySelectorAll('[data-update-action]').forEach(button => {
      button.onclick = () => this.run(button.dataset.updateAction);
    });
    if (active) {
      const next = [...this.root.querySelectorAll('[data-update-action]')].find(button => button.dataset.updateAction === active);
      if (next && !next.disabled) next.focus({ preventScroll: true });
      else { this.root.tabIndex = -1; this.root.focus({ preventScroll: true }); }
    }
  },

  async run(action) {
    const method = { check: 'checkForUpdates', download: 'downloadUpdate', open: 'openUpdatePage' }[action];
    if (action === 'install') {
      if (!window.confirm?.(translate('prefs.update.confirm'))) return;
      await this.call('installUpdate');
      return;
    }
    if (method) await this.call(method);
  },

  async call(method) {
    try {
      const result = await this.bridge[method]();
      if (result && typeof result === 'object' && result.state) this.setStatus(result);
    } catch (error) {
      this.setStatus({ state: 'error', reason: error?.message || String(error) });
    }
  },
};
