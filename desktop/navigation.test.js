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

test('offline screen defaults to English independently of operating system language', () => {
  const html = fs.readFileSync(require('node:path').join(__dirname, 'error.html'), 'utf8');
  const script = html.match(/<script>([\s\S]*?)<\/script>/)[1];
  for (const [search, lang] of [['', 'en'], ['?lang=en', 'en'], ['?lang=ca', 'ca']]) {
    const elements = {};
    const document = { documentElement: {}, getElementById: id => elements[id] ||= {} };
    vm.runInNewContext(script, { document, location: { search }, URLSearchParams });
    assert.equal(document.documentElement.lang, lang);
    if (lang === 'ca') assert.equal(elements.retryBtn.textContent, 'Torna-ho a provar');
  }
});

test('desktop credentials are delivered by the guarded bridge, never URL query parameters', () => {
  const main = fs.readFileSync(require('node:path').join(__dirname, 'main.js'), 'utf8');
  assert.equal(main.includes('encodeURIComponent(TOKEN)'), false);
  assert.ok(main.includes("trusted(event) ? { token: TOKEN } : { token: '' }"));
  assert.ok(main.includes("guest.setWindowOpenHandler(() => ({ action: 'deny' }))"));
});
