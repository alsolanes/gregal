'use strict';
const test = require('node:test');
const assert = require('node:assert');
const { renderHTML, addBody } = require('./connectors.js');

const T = k => ({ 'mcp.connect': 'Connect', 'mcp.reconnect': 'Reconnect', 'mcp.status.needs_auth': 'Needs authorization', 'mcp.toolsN': '{n} tools' }[k] || k);
const esc = s => String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

test('connectors: estat, botons i escapament', () => {
  const html = renderHTML({
    summary: '2 servers, 3 tools',
    servers: [
      { name: 'docs', status: 'connected', transport: 'http', host: 'mcp.example.com', tools: 3, auth: 'authorized' },
      { name: '<x>', status: 'needs_auth', transport: 'http', auth: 'required', error: 'authorization required' },
    ],
    tools: ['mcp_docs_search'],
    prompts: [{ command: '/mcp__docs__review' }],
  }, T, esc);
  assert.match(html, /3 tools/);
  assert.match(html, /data-mcp-act="connect" data-mcp-name="docs">Reconnect/);
  assert.match(html, /data-mcp-act="disconnect" data-mcp-name="docs"/);
  assert.match(html, /data-mcp-name="&lt;x&gt;">Connect/);
  assert.match(html, /Needs authorization/);
  assert.match(html, /\/mcp__docs__review/);
  assert.doesNotMatch(html, /<x>/);
  assert.match(html, /<form class="mcp-add"/);
});

test('connectors: el formulari distingeix URL i ordre local', () => {
  assert.deepStrictEqual(addBody(' remot ', 'https://x/mcp', 'ignored'), { name: 'remot', url: 'https://x/mcp' });
  assert.deepStrictEqual(addBody('fs', '', 'npx -y "@scope/server files" /tmp'), { name: 'fs', command: 'npx', args: ['-y', '@scope/server files', '/tmp'] });
});

test('connectors: botó de la base de dades de demo i pas per connectar-la', () => {
  const { hasDemo, demoStep, ensureDemo } = require('./connectors.js');
  const empty = renderHTML({ servers: [] }, T, esc);
  assert.match(empty, /data-mcp-act="demo"/);
  const withDemo = { servers: [{ name: 'demo', status: 'connected', transport: 'stdio', tools: 3, readonly: true }] };
  assert.ok(hasDemo(withDemo));
  const html = renderHTML(withDemo, T, esc);
  assert.doesNotMatch(html, /data-mcp-act="demo"/);
  assert.match(html, /mcp\.readonly|read-only/);
  assert.strictEqual(demoStep({ servers: [] }), 'add');
  assert.strictEqual(demoStep(withDemo), 'none');
  assert.strictEqual(demoStep({ servers: [{ name: 'demo', status: 'error' }] }), 'connect');
  assert.strictEqual(typeof ensureDemo, 'function');
});

test('connectors: ensureDemo afegeix el preset quan no hi és', async () => {
  const { ensureDemo } = require('./connectors.js');
  const calls = [];
  const fake = async (url, opts) => {
    calls.push([url, opts && opts.body]);
    const body = url === '/api/mcp' ? { servers: [] } : { saved: true, servers: [{ name: 'demo', status: 'connected' }] };
    return { ok: true, status: 200, text: async () => JSON.stringify(body) };
  };
  await ensureDemo(fake);
  assert.deepStrictEqual(calls, [['/api/mcp', undefined], ['/api/mcp/servers', '{"preset":"demo"}']]);
  const failing = async url => ({ ok: true, status: 200, text: async () => JSON.stringify(url === '/api/mcp' ? { servers: [] } : { saved: true, error: 'spawn failed' }) });
  await assert.rejects(ensureDemo(failing), /spawn failed/);
});
