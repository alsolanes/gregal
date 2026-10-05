const test = require('node:test');
const assert = require('node:assert/strict');
const { EventEmitter } = require('node:events');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { createUpdater, RELEASE_PAGE } = require('./updater');

function fixture() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'gregal-update-'));
  fs.writeFileSync(path.join(root, 'app-update.yml'), 'provider: github\n');
  const updater = new EventEmitter();
  const calls = { check: 0, download: 0, install: 0, stopped: 0, opened: [] };
  updater.checkForUpdates = () => { calls.check++; return new Promise((resolve, reject) => { updater.checkResolve = resolve; updater.checkReject = reject; }); };
  updater.downloadUpdate = () => { calls.download++; return new Promise(resolve => { updater.downloadResolve = resolve; }); };
  updater.quitAndInstall = (...args) => { calls.install++; calls.installArgs = args; };
  const controller = createUpdater({
    app: { isPackaged: true, getVersion: () => '1.7.2' }, platform: 'win32', env: {}, resourcesPath: root,
    loadAutoUpdater: () => updater, openExternal: async url => calls.opened.push(url),
    stopBackend: async () => { calls.stopped++; },
  });
  return { controller, updater, calls, root };
}

test('installer updater keeps main-process snapshot and requires explicit actions', async t => {
  const f = fixture(); t.after(() => fs.rmSync(f.root, { recursive: true, force: true }));
  assert.deepEqual(f.controller.getStatus(), { state: 'idle', currentVersion: '1.7.2', version: '', progress: null, mode: 'automatic', reason: '' });
  assert.equal(f.updater.autoDownload, false);
  assert.equal(f.updater.autoInstallOnAppQuit, false);
  assert.equal(f.updater.allowPrerelease, false);
  assert.equal(f.updater.allowDowngrade, false);
  assert.equal(f.calls.check, 0);
  assert.equal(f.calls.download, 0);
  const seen = [];
  const off = f.controller.onStatus(status => seen.push(status));
  const a = f.controller.check();
  const b = f.controller.check();
  assert.equal(f.calls.check, 1);
  f.updater.emit('update-available', { version: '1.7.3' });
  assert.equal(f.controller.getStatus().version, '1.7.3');
  await f.controller.check(); // A repeat check preserves the discovered update.
  assert.equal(f.calls.check, 1);
  f.updater.checkResolve();
  await Promise.all([a, b]);
  assert.equal(f.controller.getStatus().state, 'available');
  const d1 = f.controller.download();
  const d2 = f.controller.download();
  assert.equal(f.calls.download, 1);
  assert.equal(f.controller.getStatus().state, 'downloading');
  await f.controller.check(); // Reload/manual check cannot interrupt the transfer.
  assert.equal(f.controller.getStatus().state, 'downloading');
  f.updater.emit('download-progress', { percent: 42 });
  assert.equal(f.controller.getStatus().progress, 42);
  f.updater.emit('update-downloaded', { version: '1.7.3' });
  f.updater.downloadResolve();
  await Promise.all([d1, d2]);
  assert.equal(f.controller.getStatus().state, 'ready');
  const installing = f.controller.install();
  assert.equal(f.controller.getStatus().state, 'installing');
  await Promise.all([installing, f.controller.install()]);
  assert.equal(f.calls.stopped, 1);
  assert.equal(f.calls.install, 1);
  assert.deepEqual(f.calls.installArgs, [false, true]);
  assert.equal((await f.controller.check()).state, 'installing');
  await f.controller.install();
  assert.equal(f.calls.install, 1);
  assert.ok(seen.length >= 4);
  off();
  await f.controller.openPage();
  assert.deepEqual(f.calls.opened, [RELEASE_PAGE]);
});

test('portable, development, disabled, and missing metadata give explicit manual status', t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'gregal-update-')); t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const app = { isPackaged: true, getVersion: () => '1.7.2' };
  const portable = createUpdater({ app, platform: 'win32', env: { PORTABLE_EXECUTABLE_FILE: 'Gregal.exe' }, resourcesPath: root });
  assert.equal(portable.getStatus().mode, 'portable');
  assert.equal(portable.getStatus().reason, 'portable');
  const dev = createUpdater({ app: { ...app, isPackaged: false }, platform: 'win32', env: {}, resourcesPath: root });
  assert.equal(dev.getStatus().state, 'unsupported');
  assert.equal(dev.getStatus().reason, 'development');
  const disabled = createUpdater({ app, platform: 'win32', env: { GREGAL_NO_UPDATE: '1' }, resourcesPath: root });
  assert.equal(disabled.getStatus().reason, 'disabled');
  const missing = createUpdater({ app, platform: 'win32', env: {}, resourcesPath: root });
  assert.equal(missing.getStatus().reason, 'missing-metadata');
});

test('check failures are visible and a later manual retry can recover', async t => {
  const f = fixture(); t.after(() => fs.rmSync(f.root, { recursive: true, force: true }));
  const failed = f.controller.check();
  f.updater.checkReject(new Error('GitHub release request failed'));
  const state = await failed;
  assert.equal(state.state, 'error');
  assert.match(state.reason, /GitHub release request failed/);
  const retried = f.controller.check();
  assert.equal(f.calls.check, 2);
  f.updater.emit('update-not-available');
  f.updater.checkResolve();
  assert.equal((await retried).state, 'up-to-date');
});
