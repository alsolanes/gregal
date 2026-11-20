const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const pkg = require('./package.json');

test('Windows release includes an update-capable installer and distinct portable', () => {
  assert.deepEqual(pkg.build.win.target, ['nsis', 'portable']);
  assert.notEqual(pkg.build.nsis.artifactName, pkg.build.portable.artifactName);
  assert.equal(pkg.build.publish.provider, 'github');
  assert.equal(pkg.build.publish.owner, 'alsolanes');
  assert.equal(pkg.build.publish.repo, 'gregal');
  assert.equal(pkg.build.publish.releaseType, 'draft');
  assert.ok(pkg.build.files.includes('updater.js'), 'the update controller must ship with the app');
});

test('desktop, lockfile and backend version agree', () => {
  assert.equal(require('./package-lock.json').version, pkg.version);
  assert.equal(require('./package-lock.json').packages[''].version, pkg.version);
  const main = fs.readFileSync(path.join(__dirname, '..', 'main.go'), 'utf8');
  assert.ok(main.includes(`var version = "v${pkg.version}"`));
});

test('the Windows installer adds no separate Vents shortcut and removes the old ones', () => {
  // Codi i Vents es trien al desplegable de la mateixa finestra: una segona icona sobra.
  const nsh = fs.readFileSync(path.join(__dirname, 'build', 'installer.nsh'), 'utf8');
  const install = nsh.slice(nsh.indexOf('!macro customInstall'), nsh.indexOf('!macroend'));
  assert.doesNotMatch(nsh, /CreateShortCut[^\n]*Vents/);
  assert.ok(install.includes(String.raw`Delete "$SMPROGRAMS\Gregal Vents.lnk"`));
  assert.ok(install.includes(String.raw`Delete "$DESKTOP\Gregal Vents.lnk"`));
});

test('the app opens Codi even when started with the old --vents switch', () => {
  const vm = require('node:vm');
  const { createRequire } = require('node:module');
  const electron = {
    app: { getPath: () => path.join(require('node:os').tmpdir(), 'gregal-vents-test-no-profile'), whenReady: () => ({ then() {} }), on() {} },
    ipcMain: { on() {}, handle() {} },
    nativeTheme: { shouldUseDarkColors: false },
  };
  const localRequire = createRequire(path.join(__dirname, 'main.js'));
  const context = {
    require: name => name === 'electron' ? electron : localRequire(name),
    process: { env: {}, platform: process.platform, argv: ['Gregal.exe', '--vents'] },
    __dirname, URL, console, setTimeout, clearTimeout,
  };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'main.js'), 'utf8') + '\nthis.testAppPath = appPath;', context);
  assert.equal(context.testAppPath(), '/');
});
