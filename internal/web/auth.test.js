const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const html = fs.readFileSync(path.join(__dirname, 'index.html'), 'utf8');
const setup = html.slice(html.indexOf('const desktopToken ='), html.indexOf('// Fora de línia:'));
const api = html.slice(html.indexOf('async function api('), html.indexOf('async function stream('));

function runtime({ desktop, search = '', statuses = [200], storageBlocked = false } = {}) {
  const calls = { storage: [], requests: [], prompts: 0, cleanURL: 0 };
  const context = {
    window: { gregalDesktop: desktop, gregalSession: 'session-test', gregalT: key => key },
    localStorage: {
      getItem(key) { calls.storage.push(['get', key]); if (storageBlocked) throw new Error('Blocked'); return 'browser-token'; },
      setItem(key, value) { calls.storage.push(['set', key, value]); },
    },
    location: { search, pathname: '/' },
    history: { replaceState() { calls.cleanURL++; } },
    URLSearchParams,
    prompt() { calls.prompts++; return 'replacement-token'; },
    marcaOffline() {},
    async fetch(url, options) { calls.requests.push({ url, ...options }); return { status: statuses.shift() ?? 200 }; },
  };
  vm.runInNewContext(setup + api + '\nthis.request = api;', context);
  return { context, calls };
}

test('desktop bridge token takes precedence without browser persistence', async () => {
  const { context, calls } = runtime({ desktop: { apiToken: 'bridge-token' }, search: '?token=url-token' });
  await context.request('/api/state');
  assert.equal(calls.requests[0].headers.Authorization, 'Bearer bridge-token');
  assert.equal(calls.requests[0].headers['X-Gregal-Session'], 'session-test');
  assert.deepEqual(calls.storage, []);
  assert.equal(calls.cleanURL, 1);
});

test('empty desktop token does not reuse browser credentials or prompt on rejection', async () => {
  const { context, calls } = runtime({ desktop: { apiToken: '' }, statuses: [401] });
  await assert.rejects(context.request('/api/state'), /auth.required/);
  assert.equal(calls.requests[0].headers.Authorization, undefined);
  assert.deepEqual(calls.storage, []);
  assert.equal(calls.prompts, 0);
});

test('legacy browser token URLs remain compatible and are cleared', async () => {
  const { context, calls } = runtime({ search: '?token=url-token' });
  await context.request('/api/state');
  assert.equal(calls.requests[0].headers.Authorization, 'Bearer url-token');
  assert.equal(calls.cleanURL, 1);
  assert.ok(calls.storage.some(([action, key, value]) => action === 'set' && key === 'gregal_token' && value === 'url-token'));
});

test('blocked browser storage does not prevent API requests', async () => {
  const { context, calls } = runtime({ storageBlocked: true });
  await context.request('/api/state');
  assert.equal(calls.requests[0].headers.Authorization, undefined);
});

test('a rejected replacement token triggers at most one retry', async () => {
  const { context, calls } = runtime({ statuses: [401, 401] });
  await assert.rejects(context.request('/api/state'), /auth.required/);
  assert.equal(calls.prompts, 1);
  assert.equal(calls.requests.length, 2);
});
