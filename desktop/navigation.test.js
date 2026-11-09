const test = require('node:test');
const assert = require('node:assert/strict');
const { pathToFileURL } = require('node:url');
const { sameOrigin, externalURL, backendURLError, localServiceURL, trustedPage } = require('./navigation');
const fs = require('node:fs');
const vm = require('node:vm');

test('navigation checks origins rather than URL prefixes', () => {
  const base = 'https://chat.example.org';
  assert.equal(sameOrigin(base + '/session', base), true);
  for (const url of ['https://chat.example.org.evil.test', 'https://chat.example.org@evil.test', 'https://chat.example.org:8443', 'javascript:alert(1)', 'file:///tmp/test']) assert.equal(sameOrigin(url, base), false);
  assert.equal(externalURL('https://example.org/docs'), true);
  assert.equal(externalURL('http://example.org/docs'), true);
  for (const url of ['javascript:alert(1)', 'file:///tmp/test', 'https://user:secret@example.org', 'not a URL']) assert.equal(externalURL(url), false);
  assert.equal(backendURLError('https://example.org'), '');
  for (const url of ['http://localhost:8097', 'http://127.0.0.1:8097', 'http://127.255.0.1:8097', 'http://[::1]:8097']) assert.equal(backendURLError(url), '');
  assert.match(backendURLError('http://example.org'), /must use HTTPS for non-loopback servers/);
  assert.match(backendURLError('http://192.0.2.10'), /must use HTTPS for non-loopback servers/);
  assert.match(backendURLError('https://user:secret@example.org'), /embedded credentials/);
});

test('managed descriptors stay on loopback and IPC trusts only application pages', () => {
  assert.equal(localServiceURL('http://127.0.0.1:8097'), true);
  assert.equal(localServiceURL('http://[::1]:8097/'), true);
  for (const url of ['http://example.org', 'http://127.0.0.1.evil.test', 'http://127.0.0.1:8097/?token=secret', 'http://127.0.0.1:8097/path']) assert.equal(localServiceURL(url), false);
  const errorPath = require('node:path').resolve(__dirname, 'error.html');
  assert.equal(trustedPage(pathToFileURL(errorPath).href + '?lang=en', 'http://127.0.0.1:8097', errorPath), true);
  assert.equal(trustedPage('https://evil.test', 'http://127.0.0.1:8097', errorPath), false);
});

// Un DOM de joguina per executar els scripts de les pàgines locals.
function fakePage(search) {
  const elements = {};
  const style = {};
  const el = () => ({ hidden: false, textContent: '', innerHTML: '', listeners: {}, addEventListener(ev, fn) { this.listeners[ev] = fn; } });
  const document = {
    title: '',
    documentElement: { dataset: {}, style: { setProperty: (k, v) => { style[k] = v; } } },
    getElementById: id => elements[id] ||= el(),
  };
  return { elements, style, document, context: { document, location: { search }, URLSearchParams, window: {} } };
}
const scriptsOf = file => [...fs.readFileSync(require('node:path').join(__dirname, file), 'utf8').matchAll(/<script>([\s\S]*?)<\/script>/g)].map(m => m[1]);

test('offline screen defaults to English independently of operating system language', () => {
  for (const file of ['error.html', 'loading.html']) {
    for (const [search, lang] of [['', 'en'], ['?lang=en', 'en'], ['?lang=ca', 'ca']]) {
      const page = fakePage(search);
      for (const script of scriptsOf(file)) vm.runInNewContext(script, page.context);
      assert.equal(page.document.documentElement.lang, lang, file + search);
      if (file === 'error.html' && lang === 'ca') assert.equal(page.elements.retryBtn.textContent, 'Torna-ho a provar');
      if (file === 'loading.html') assert.equal(page.elements.first.textContent, (lang === 'ca' ? 'Obrint' : 'Starting') + ' Gregal…');
    }
  }
});

test('local pages take the theme colours and product name from the query, and ignore anything else', () => {
  const ok = fakePage('?scheme=light&bg=%23F0EEE6&bar=%23E7E2D3&ink=%231F1E1D&accent=%23C15F3C&name=Gregal%20Jocs&reason=stopped');
  for (const script of scriptsOf('error.html')) vm.runInNewContext(script, ok.context);
  assert.deepEqual(ok.style, { '--bg': '#F0EEE6', '--bar': '#E7E2D3', '--ink': '#1F1E1D', '--accent': '#C15F3C' });
  assert.equal(ok.document.documentElement.dataset.scheme, 'light');
  assert.equal(ok.document.title, 'Gregal Jocs');
  assert.match(ok.elements.errTitle.textContent, /^Gregal Jocs's local server stopped$/);
  const bad = fakePage('?bg=red;background:url(x)&ink=%23fff&reason=<b>');
  for (const script of scriptsOf('error.html')) vm.runInNewContext(script, bad.context);
  assert.equal(bad.style['--bg'], '#14161B');
  assert.equal(bad.style['--ink'], '#F2F4F8');
  assert.equal(bad.elements.errTitle.textContent, "Gregal couldn't start");
});

test('desktop credentials are delivered by the guarded bridge, never URL query parameters', () => {
  const main = fs.readFileSync(require('node:path').join(__dirname, 'main.js'), 'utf8');
  assert.equal(main.includes('encodeURIComponent(TOKEN)'), false);
  // El token només per a la pàgina de confiança; l'edició no és secreta.
  assert.ok(main.includes("trusted(event) ? { token: TOKEN, edition: EDITION } : { token: '', edition: EDITION }"));
  // Els webviews obren enllaços al mateix panell o en una finestra sense
  // preload ni node (browser-bridge.js guestWindowOpen); la resta, denegat.
  assert.ok(main.includes('const how = guestWindowOpen(details, () => BASE);'));
  assert.ok(main.includes("if (how !== 'popup') return { action: 'deny' };"));
  assert.ok(main.includes("webPreferences: { partition: AGENT_BROWSER_PARTITION, sandbox: true, contextIsolation: true, nodeIntegration: false }"));
  assert.ok(main.includes("child.webContents.setWindowOpenHandler(() => ({ action: 'deny' }))"));
});
