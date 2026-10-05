const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

async function loadMatcher() {
  const source = fs.readFileSync(path.join(__dirname, 'convs.js'), 'utf8');
  const context = vm.createContext({ window: {} });
  const mod = new vm.SourceTextModule(source, { context, identifier: 'convs.js' });
  await mod.link(() => { throw new Error('convs.js no hauria de tenir imports'); });
  await mod.evaluate();
  return mod.namespace.matchesConversation;
}

test('la cerca troba el títol i el nom de conversa sense distingir majúscules', async () => {
  const match = await loadMatcher();
  assert.equal(match({ title: 'Revisió de Gregal' }, 'revisió'), true);
  assert.equal(match({ name: 'sessio-20261004-a' }, 'SESSIO-20261004'), true);
  assert.equal(match({ title: 'Una altra tasca' }, 'gregal'), false);
});

test('la cerca troba el nom de projecte i qualsevol fragment de la ruta', async () => {
  const match = await loadMatcher();
  assert.equal(match({ project: 'Gregal Desktop' }, 'desktop'), true);
  assert.equal(match({ workspace: 'C:\\Users\\Ada\\Projects\\Gregal' }, 'users\\ada'), true);
  assert.equal(match({ workspace: '/work/repos/Gregal' }, 'repos/gregal'), true);
});

test('la cerca tracta camps nuls, consultes buides i converses null sense errors', async () => {
  const match = await loadMatcher();
  assert.equal(match(null, 'projecte'), false);
  assert.equal(match({ title: null, name: undefined, project: null, workspace: null }, 'x'), false);
  assert.equal(match({ title: null, workspace: null }, ''), true);
});
