const test = require('node:test');
const assert = require('node:assert/strict');
const { createDocWindows } = require('./doc-window');

function fakeWindows() {
  const created = [];
  class FakeWindow {
    constructor(options) {
      this.options = options;
      this.handlers = {};
      this.focused = 0;
      this.destroyed = false;
      this.webContents = {
        mainFrame: { url: '' },
        on: (name, fn) => { this.handlers['wc:' + name] = fn; },
        setWindowOpenHandler: fn => { this.openHandler = fn; },
      };
      created.push(this);
    }
    once(name, fn) { this.handlers[name] = fn; }
    loadURL(url) { this.webContents.mainFrame.url = url; return Promise.resolve(); }
    isDestroyed() { return this.destroyed; }
    isMinimized() { return false; }
    restore() {}
    focus() { this.focused++; }
    close() { this.destroyed = true; this.handlers.closed?.(); }
  }
  return { FakeWindow, created };
}

test('one isolated window per document; pressing again focuses it', () => {
  const { FakeWindow, created } = fakeWindows();
  const external = [];
  const docs = createDocWindows({ BrowserWindow: FakeWindow, base: () => 'http://127.0.0.1:8097', preloadPath: '/p/preload.js', openExternal: url => external.push(url) });
  assert.equal(docs.open('abc123def456', 'tab-1'), true);
  assert.equal(created.length, 1);
  const w = created[0];
  assert.equal(w.webContents.mainFrame.url, 'http://127.0.0.1:8097/app/doc/abc123def456?session=tab-1');
  assert.deepEqual(w.options.webPreferences, { preload: '/p/preload.js', contextIsolation: true, nodeIntegration: false, sandbox: true });

  assert.equal(docs.open('abc123def456', 'tab-1'), true);
  assert.equal(created.length, 1);
  assert.equal(w.focused, 1);
  assert.equal(docs.open('zzz999', 'tab-1'), true);
  assert.equal(created.length, 2);

  // La finestra no navega enlloc: els enllaços van al navegador.
  let prevented = false;
  w.handlers['wc:will-navigate']({ preventDefault: () => { prevented = true; } }, 'https://example.org/');
  assert.equal(prevented, true);
  assert.deepEqual(w.openHandler({ url: 'https://example.org/b' }), { action: 'deny' });
  assert.deepEqual(external, ['https://example.org/', 'https://example.org/b']);

  // Tancada, el mateix document torna a obrir una finestra nova.
  w.close();
  assert.equal(docs.open('abc123def456', 'tab-1'), true);
  assert.equal(created.length, 3);

  // Tancar la finestra principal tanca les dels documents.
  docs.closeAll();
  assert.deepEqual(created.map(x => x.destroyed), [true, true, true]);
});

test('a personal document («Els meus documents») opens its own page with scope=mine', () => {
  const { FakeWindow, created } = fakeWindows();
  const docs = createDocWindows({ BrowserWindow: FakeWindow, base: () => 'http://127.0.0.1:8097', preloadPath: '', openExternal() {} });
  assert.equal(docs.open('w1', 'default', 'mine'), true);
  assert.equal(created[0].webContents.mainFrame.url, 'http://127.0.0.1:8097/app/doc/w1?session=default&scope=mine');
  assert.equal(docs.open('p1', 'default', ''), true);
  assert.equal(created[1].webContents.mainFrame.url, 'http://127.0.0.1:8097/app/doc/p1?session=default');
  assert.equal(docs.open('p2', 'default', 'other'), false);
  assert.equal(created.length, 2);
});

test('refuses ids and sessions that are not plain tokens', () => {
  const { FakeWindow, created } = fakeWindows();
  const docs = createDocWindows({ BrowserWindow: FakeWindow, base: () => 'http://127.0.0.1:8097', preloadPath: '', openExternal() {} });
  for (const [id, session] of [['../api/state', 'default'], ['ABC', 'default'], ['abc', 'a?b'], [42, 'default'], ['abc', undefined]]) {
    assert.equal(docs.open(id, session), false);
  }
  assert.equal(created.length, 0);
});

test('isSender only accepts the main frame of an open document window', () => {
  const { FakeWindow, created } = fakeWindows();
  const docs = createDocWindows({ BrowserWindow: FakeWindow, base: () => 'http://127.0.0.1:8097', preloadPath: '', openExternal() {} });
  docs.open('abc', 'default');
  const wc = created[0].webContents;
  assert.equal(docs.isSender({ sender: wc, senderFrame: wc.mainFrame }), true);
  assert.equal(docs.isSender({ sender: wc, senderFrame: { url: wc.mainFrame.url } }), false);
  assert.equal(docs.isSender({ sender: {}, senderFrame: wc.mainFrame }), false);
  created[0].close();
  assert.equal(docs.isSender({ sender: wc, senderFrame: wc.mainFrame }), false);
});
