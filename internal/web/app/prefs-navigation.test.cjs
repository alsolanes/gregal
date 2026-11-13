const { test } = require('node:test');
const assert = require('node:assert/strict');

test('old preference links resolve to the new pages and select their parent', async () => {
  const { resolvePrefsPage, prefsNavPage } = await import('./prefs-navigation.js');
  for (const [old, page] of Object.entries({ agent:'models', integrations:'connections', extras:'appearance', advanced:'general', voice:'plugins', brands:'identity', calendar:'calendar', accounts:'accounts', powers:'powers' })) {
    assert.equal(resolvePrefsPage(old), page);
  }
  assert.equal(prefsNavPage('calendar'), 'connections');
  assert.equal(prefsNavPage('accounts'), 'server');
  assert.equal(resolvePrefsPage('missing'), 'general');
});

test('server pages require admin rights and unavailable endpoints stay hidden', async () => {
  const { prefsPageAvailable } = await import('./prefs-navigation.js');
  assert.equal(prefsPageAvailable('identity', {admin:false}), false);
  assert.equal(prefsPageAvailable('accounts', {admin:true, accounts:false}), false);
  assert.equal(prefsPageAvailable('accounts', {admin:true, accounts:true}), true);
  assert.equal(prefsPageAvailable('powers', {admin:true, powers:true}), true);
  assert.equal(prefsPageAvailable('permissions', {admin:false}), true);
  assert.equal(prefsPageAvailable('connected', {connected:false}), false);
  assert.equal(prefsPageAvailable('calendar', {calendar:true}), true);
});

test('global search finds page content, accents and former section names', async () => {
  const { searchPrefsPages } = await import('./prefs-navigation.js');
  const pages = [
    {id:'permissions', title:'Permisos', text:'Aprovació de les accions'},
    {id:'identity', title:'Identitat del servidor'},
    {id:'calendar', title:'Calendari i correu', text:'Google Calendar i Gmail'},
    {id:'plugins', title:'Funcions opcionals'},
  ];
  assert.deepEqual(searchPrefsPages('permís', pages).map(p => p.id), ['permissions']);
  assert.deepEqual(searchPrefsPages('brandings', pages).map(p => p.id), ['identity']);
  assert.deepEqual(searchPrefsPages('GMAIL', pages).map(p => p.id), ['calendar']);
  assert.deepEqual(searchPrefsPages('kanban', pages).map(p => p.id), ['plugins']);
  assert.deepEqual(searchPrefsPages('Models i agent', pages).map(p => p.id), ['permissions']);
  assert.deepEqual(searchPrefsPages('missing', pages), []);
});
