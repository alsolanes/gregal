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

test('Gregal Jocs packages its own backend (gregal-jocs) and keeps the jocs update channel', () => {
  const jocs = require('./electron-builder-jocs.js');
  assert.equal(jocs.extraMetadata.gregalEdition, 'jocs');
  assert.equal(jocs.publish.channel, 'jocs');
  const backend = jocs.extraResources.find(r => r.to === 'backend');
  assert.equal(backend.from, 'backend-jocs', 'the Jocs edition bundles backend-jocs/ (gregal-jocs), not the Gregal backend');
  assert.equal(pkg.build.extraResources.find(r => r.to === 'backend').from, 'backend', 'Gregal keeps its own backend');
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
