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
  const main = fs.readFileSync(path.join(__dirname, 'main.js'), 'utf8');
  assert.match(main, /process\.env\.PORTABLE_EXECUTABLE_FILE/);
});

test('desktop, lockfile and backend version agree', () => {
  assert.equal(require('./package-lock.json').version, pkg.version);
  assert.equal(require('./package-lock.json').packages[''].version, pkg.version);
  const main = fs.readFileSync(path.join(__dirname, '..', 'main.go'), 'utf8');
  assert.ok(main.includes(`var version = "v${pkg.version}"`));
});
