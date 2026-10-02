const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

test('scheduled tasks and MCP render both languages without translating service output', async () => {
  const { t } = await import('./app/i18n.js');
  const previousStorage = global.localStorage;
  const html = fs.readFileSync(path.join(__dirname, 'index.html'), 'utf8');
  const code = html.slice(html.indexOf('async function loadJobsPage()'), html.indexOf('function setSide(open)'));
  try {
    for (const [lang, runLabel, emptyLabel, toolsLabel] of [
      ['en', 'Run', 'No scheduled tasks yet.', 'Tools:'],
      ['ca', 'Executa', 'Encara no hi ha tasques programades.', 'Eines:'],
    ]) {
      global.localStorage = { getItem: () => lang };
      const nodes = {};
      const $ = id => nodes[id] ||= { dataset: {}, style: {}, value: '', querySelectorAll: () => [] };
      let jobs = [{ id: 'sample-job', name: 'Service-owned name', prompt: 'Service-owned prompt', enabled: true }];
      const context = {
        $, gregalT: t, escapeHTML: value => String(value), confirm: () => false,
        api: async url => ({ ok: true, json: async () => url === '/api/jobs' ? { jobs } : { servers: [], tools: ['service_tool'], summary: 'Service-owned summary' } }),
      };
      vm.runInNewContext(code + '\nthis.jobs = loadJobsPage; this.mcp = loadMcpPage;', context);
      await context.jobs();
      assert.ok($('jobsPage').innerHTML.includes('>' + runLabel + '</button>'));
      assert.ok($('jobsPage').innerHTML.includes('Service-owned name'));
      assert.ok($('jobsPage').innerHTML.includes('Service-owned prompt'));
      jobs = [];
      await context.jobs();
      assert.ok($('jobsPage').innerHTML.includes(emptyLabel));
      await context.mcp();
      assert.ok($('mcpPageBody').innerHTML.includes(toolsLabel));
      assert.ok($('mcpPageBody').innerHTML.includes('Service-owned summary'));
      assert.ok($('mcpPageBody').innerHTML.includes('service_tool'));
    }
  } finally {
    global.localStorage = previousStorage;
  }
});
