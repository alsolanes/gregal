const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

function load({ bridge = null, confirmed = true, language = 'en' } = {}) {
  const labels = {
    en: {
      'prefs.title': 'Preferences', 'prefs.update.notice': 'An update is available',
      'prefs.update.version': 'Gregal version', 'prefs.update.idle': 'Check whether a newer version is available.',
      'prefs.update.check': 'Check for updates', 'prefs.update.checking': 'Checking for updates…',
      'prefs.update.available': 'Version {version} is available.', 'prefs.update.download': 'Download update',
      'prefs.update.downloading': 'Downloading version {version}…', 'prefs.update.progress': 'Download progress',
      'prefs.update.install': 'Install and restart', 'prefs.update.ready': 'Version {version} is ready to install.',
      'prefs.update.current': 'You’re up to date.', 'prefs.update.error': 'The update could not be completed. Try again.',
      'prefs.update.portable': 'This portable build is updated manually from official releases.',
      'prefs.update.unsupported': 'Integrated updates are not available in this build.',
      'prefs.update.releases': 'Open official releases', 'prefs.update.busy': 'Please wait…',
      'prefs.update.reason.development': 'Development builds cannot update from the app.',
    },
    ca: {
      'prefs.title': 'Preferències', 'prefs.update.notice': 'Hi ha una actualització disponible',
      'prefs.update.version': 'Versió de Gregal', 'prefs.update.idle': 'Comprova si hi ha una versió més nova.',
      'prefs.update.check': 'Comprova si hi ha actualitzacions', 'prefs.update.checking': 'Comprovant…',
      'prefs.update.available': 'La versió {version} està disponible.', 'prefs.update.download': 'Baixa l’actualització',
      'prefs.update.downloading': 'Baixant la versió {version}…', 'prefs.update.progress': 'Progrés',
      'prefs.update.install': 'Instal·la i reinicia', 'prefs.update.ready': 'La versió {version} està a punt.',
      'prefs.update.current': 'Ja tens la versió més recent.', 'prefs.update.error': 'No s’ha pogut completar.',
      'prefs.update.portable': 'Aquesta versió portable s’actualitza manualment des de les versions oficials.',
      'prefs.update.unsupported': 'Les actualitzacions integrades no estan disponibles.',
      'prefs.update.releases': 'Obre les versions oficials', 'prefs.update.busy': 'Un moment…',
    },
  };
  const button = { classList: { toggle() {} }, setAttribute() {}, title: '' };
  const document = {
    activeElement: null,
    addEventListener() {},
    getElementById(id) { return id === 'prefsBtn' ? button : null; },
  };
  const window = {
    gregalDesktop: bridge,
    gregalT: key => labels[language][key] || key,
    confirm: () => confirmed,
  };
  let source = fs.readFileSync(path.join(__dirname, 'desktop-updates.js'), 'utf8');
  source = source.replace(/^export const /m, 'const ');
  source += '\nglobalThis.__updates = desktopUpdates;';
  const sandbox = { window, document, console, Promise, Set, Number, String, Object };
  vm.runInNewContext(source, sandbox, { filename: 'desktop-updates.js' });
  return { updates: sandbox.__updates, button, sandbox };
}

test('updates are absent from the browser and present only with the desktop bridge', () => {
  const browser = load();
  browser.updates.init();
  assert.equal(browser.updates.section(), '');

  const desktop = load({ bridge: { getUpdateStatus() {}, version: '1.7.2' } });
  desktop.updates.init();
  assert.match(desktop.updates.section(), /desktopUpdateContent/);
  assert.match(desktop.updates.content(), /Gregal version/);
  assert.match(desktop.updates.content(), /1\.7\.2/);
});

test('available updates offer download for installed builds and official releases for portable builds', () => {
  const installed = load({ bridge: { version: '1.7.2', getUpdateStatus() {} } }).updates;
  installed.bridge = { version: '1.7.2' };
  installed.setStatus({ state: 'available', mode: 'automatic', currentVersion: '1.7.2', version: '1.7.3' });
  assert.match(installed.content(), /Version 1\.7\.3 is available/);
  assert.match(installed.content(), /data-update-action="download"/);
  assert.doesNotMatch(installed.content(), /data-update-action="check"/);

  const portable = load({ bridge: { version: '1.7.2', getUpdateStatus() {} } }).updates;
  portable.bridge = { version: '1.7.2' };
  portable.setStatus({ state: 'available', mode: 'portable', currentVersion: '1.7.2', version: '1.7.3' });
  assert.match(portable.content(), /data-update-action="open"/);
  assert.doesNotMatch(portable.content(), /data-update-action="download"/);
});

test('install requires an explicit restart confirmation', async () => {
  let installs = 0;
  const bridge = { installUpdate: async () => { installs++; } };
  const declined = load({ bridge, confirmed: false }).updates;
  declined.bridge = bridge;
  await declined.run('install');
  assert.equal(installs, 0);

  const accepted = load({ bridge, confirmed: true }).updates;
  accepted.bridge = bridge;
  await accepted.run('install');
  assert.equal(installs, 1);
});

test('errors show an escaped detail and offer both retry and the official release page', () => {
  const updates = load({ bridge: { version: '1.7.2' } }).updates;
  updates.bridge = { version: '1.7.2' };
  updates.setStatus({ state: 'error', reason: '<network unavailable>' });
  const content = updates.content();
  assert.match(content, /&lt;network unavailable&gt;/);
  assert.match(content, /data-update-action="check"/);
  assert.match(content, /data-update-action="open"/);
});

test('update messages follow the selected language', () => {
  const { updates } = load({ language: 'ca', bridge: { version: '1.7.2' } });
  updates.bridge = { version: '1.7.2' };
  updates.setStatus({ state: 'available', mode: 'automatic', version: '1.7.3' });
  assert.match(updates.content(), /La versió 1\.7\.3 està disponible/);
});
