const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');

test('native IPC and credentials reject foreign pages and subframes', async () => {
  const listeners = new Map();
  const handlers = new Map();
  let dialogCalls = 0;
  const electron = {
    app: { getPath: () => path.join(require('node:os').tmpdir(), 'gregal-ipc-test-no-profile'), whenReady: () => ({ then() {} }), on() {} },
    ipcMain: { on: (name, fn) => listeners.set(name, fn), handle: (name, fn) => handlers.set(name, fn) },
    dialog: { showOpenDialog: async () => { dialogCalls++; return { canceled: true }; } },
    nativeTheme: { shouldUseDarkColors: false },
  };
  const localRequire = createRequire(path.join(__dirname, 'main.js'));
  const context = {
    require: name => name === 'electron' ? electron : localRequire(name),
    process: { env: { GREGAL_TOKEN: 'bridge-test-token' }, platform: process.platform },
    __dirname, URL, console, setTimeout, clearTimeout,
  };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'main.js'), 'utf8') + '\nthis.testWire = wireIPC; this.testSetUpdater = value => { updateController = value; };', context);
  const frame = { url: 'http://127.0.0.1:8097/' };
  const webContents = { mainFrame: frame };
  context.testWire(() => ({ webContents }));
  const trusted = { sender: webContents, senderFrame: frame };
  listeners.get('gregal:connection')(trusted);
  assert.equal(trusted.returnValue.token, 'bridge-test-token');
  assert.equal(await handlers.get('gregal:choose-folder')(trusted), null);
  assert.equal(dialogCalls, 1);
  for (const event of [
    { sender: {}, senderFrame: frame },
    { sender: webContents, senderFrame: { url: frame.url } },
  ]) {
    listeners.get('gregal:connection')(event);
    assert.equal(event.returnValue.token, '');
    assert.equal(await handlers.get('gregal:choose-folder')(event), null);
  }
  frame.url = 'https://untrusted.example.org/';
  listeners.get('gregal:connection')(trusted);
  assert.equal(trusted.returnValue.token, '');
  assert.equal(await handlers.get('gregal:retry-backend')(trusted).then(result => result.ok), false);
  assert.equal(await handlers.get('gregal:notify')(trusted, { title: 'Test', body: 'Test' }), false);
  assert.equal(await handlers.get('gregal:choose-folder')(trusted), null);
  assert.equal(dialogCalls, 1);
  frame.url = 'http://127.0.0.1:8097/';
  const updateCalls = [];
  context.testSetUpdater({
    getStatus: () => { updateCalls.push('status'); return { state: 'idle' }; },
    check: () => { updateCalls.push('check'); return 'check'; },
    download: () => { updateCalls.push('download'); return 'download'; },
    install: () => { updateCalls.push('install'); return 'install'; },
    openPage: () => { updateCalls.push('page'); return 'page'; },
  });
  const updateChannels = ['gregal:update-status', 'gregal:update-check', 'gregal:update-download', 'gregal:update-install', 'gregal:update-page'];
  for (const channel of updateChannels) assert.notEqual(await handlers.get(channel)(trusted), null);
  assert.deepEqual(updateCalls, ['status', 'check', 'download', 'install', 'page']);
  updateCalls.length = 0;
  for (const channel of updateChannels) {
    assert.equal(await handlers.get(channel)({ sender: {}, senderFrame: frame }), null);
    assert.equal(await handlers.get(channel)({ sender: webContents, senderFrame: { url: frame.url } }), null);
  }
  frame.url = 'https://untrusted.example.org/';
  for (const channel of ['gregal:update-status', 'gregal:update-check', 'gregal:update-download', 'gregal:update-install', 'gregal:update-page']) {
    assert.equal(await handlers.get(channel)(trusted), null);
  }
  assert.deepEqual(updateCalls, []);
});
