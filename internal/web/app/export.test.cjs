const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

// Carrega export.js amb un i18n de joguina i un DOM mínim: només es
// proven les funcions pures i que el mòdul es publiqui a window.
async function load() {
  const source = fs.readFileSync(path.join(__dirname, 'export.js'), 'utf8');
  const window = {};
  const context = vm.createContext({ window, document: { getElementById: () => null }, encodeURIComponent, decodeURIComponent });
  const mod = new vm.SourceTextModule(source, { context, identifier: 'export.js' });
  await mod.link(async spec => {
    assert.equal(spec, './i18n.js');
    const i18n = new vm.SourceTextModule('export const t = k => k;', { context, identifier: 'i18n.js' });
    await i18n.link(() => {});
    return i18n;
  });
  await mod.evaluate();
  return { ns: mod.namespace, window };
}

test('exportURL tria conversa desada o viva i els flags de sanititzar', async () => {
  const { ns } = await load();
  assert.equal(ns.exportURL('', 'md', false, false), '/api/export?format=md');
  assert.equal(ns.exportURL('conv a', 'html', true, false), '/api/sessions/conv%20a/export?format=html&redact=1');
  assert.equal(ns.exportURL('x', 'html', true, true), '/api/sessions/x/export?format=html&redact=1&emails=1');
  // Els correus només compten si es sanititza.
  assert.equal(ns.exportURL('x', 'pdf', false, true), '/api/sessions/x/export?format=md');
});

test('filenameFrom llegeix el Content-Disposition del servidor', async () => {
  const { ns } = await load();
  assert.equal(ns.filenameFrom('attachment; filename="gregal-a.md"; filename*=UTF-8\'\'gregal-a.md', 'f'), 'gregal-a.md');
  assert.equal(ns.filenameFrom('attachment; filename="b.html"', 'f'), 'b.html');
  assert.equal(ns.filenameFrom(null, 'fallback.md'), 'fallback.md');
});

test('shareURL compon l’enllaç absolut i el mòdul es publica a window', async () => {
  const { ns, window } = await load();
  assert.equal(ns.shareURL('/share/abc', 'http://127.0.0.1:8090/'), 'http://127.0.0.1:8090/share/abc');
  assert.equal(typeof window.gregalExport.openMenu, 'function');
});
