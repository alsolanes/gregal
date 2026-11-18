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

test('Gregal Jocs ships without a backend, from its own releases repo, on the jocs channel', () => {
  const jocs = require('./electron-builder-jocs.js');
  assert.equal(jocs.extraMetadata.gregalEdition, 'jocs');
  assert.equal(jocs.publish.channel, 'jocs');
  assert.equal(jocs.publish.repo, 'gregal-jocs', 'Jocs has its own latest release, apart from Gregal');
  assert.equal(jocs.extraResources.find(r => r.to === 'backend'), undefined, 'the installer downloads gregal-jocs instead of bundling it');
  assert.equal(pkg.build.extraResources.find(r => r.to === 'backend').from, 'backend', 'Gregal keeps its own backend');
  assert.ok(pkg.build.files.includes('backend-download.js'), 'main.js requires it in both editions');
  assert.equal(require('./backend-download').JOCS_RELEASES_REPO, 'alsolanes/' + jocs.publish.repo, 'the app downloads from where the installer is published');
  assert.match(pkg.scripts['dist-win-jocs'], /build-backend\.js windows --jocs/);
});

test('desktop, lockfile and backend version agree', () => {
  assert.equal(require('./package-lock.json').version, pkg.version);
  assert.equal(require('./package-lock.json').packages[''].version, pkg.version);
  const main = fs.readFileSync(path.join(__dirname, '..', 'main.go'), 'utf8');
  assert.ok(main.includes(`var version = "v${pkg.version}"`));
  const jocs = fs.readFileSync(path.join(__dirname, '..', 'jocs', 'cmd', 'gregal-jocs', 'main.go'), 'utf8');
  assert.ok(jocs.includes(`var version = "v${pkg.version}"`), 'the Gregal Jocs backend has the same version');
});
