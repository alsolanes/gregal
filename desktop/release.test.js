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

// build.files is an explicit list: a module main.js requires but the list
// forgets builds fine and crashes the packaged app at startup.
test('every local module the shipped scripts require is packaged', () => {
  const shipped = new Set(pkg.build.files);
  for (const file of pkg.build.files.filter(f => f.endsWith('.js'))) {
    const src = fs.readFileSync(path.join(__dirname, file), 'utf8');
    for (const [, mod] of src.matchAll(/require\(\s*['"]\.\/([^'"]+)['"]\s*\)/g)) {
      const name = mod.endsWith('.js') || mod.endsWith('.json') ? mod : mod + '.js';
      if (name === 'package.json') continue;
      assert.ok(shipped.has(name), `${file} requires ./${mod}, which build.files does not ship`);
    }
  }
});

test('desktop, lockfile and backend version agree', () => {
  assert.equal(require('./package-lock.json').version, pkg.version);
  assert.equal(require('./package-lock.json').packages[''].version, pkg.version);
  const main = fs.readFileSync(path.join(__dirname, '..', 'main.go'), 'utf8');
  assert.ok(main.includes(`var version = "v${pkg.version}"`));
});
