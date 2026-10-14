'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const lib = require('./release-lib.cjs');

const root = path.resolve(__dirname, '..');
const real = () => Object.fromEntries(Object.entries(lib.FILES).map(([k, rel]) => [k, fs.readFileSync(path.join(root, rel), 'utf8')]));

// The guard CI relies on: a version change that misses a file fails here.
test('the repository release metadata is consistent', () => {
  assert.deepEqual(lib.checkConsistency(real()), []);
});

const fixture = () => ({
  main: 'package main\n\nvar version = "v1.2.3"\n',
  desktop: '{\n  "name": "gregal-desktop",\n  "version": "1.2.3"\n}\n',
  lock: JSON.stringify({ name: 'gregal-desktop', version: '1.2.3', lockfileVersion: 3, packages: { '': { version: '1.2.3' }, 'node_modules/x': { version: '1.2.3' } } }, null, 2) + '\n',
  changelog: '# Changelog\n\n## Unreleased\n\n- New thing.\n\n## v1.2.3 - 2026-01-01 - Old\n\n- Old thing.\n',
  android: 'versionCode = 37\n        versionName = "0.9.8"\n',
  vscode: '{ "version": "0.3.2" }\n',
});

test('bump updates every version and opens a fresh Unreleased section', () => {
  const out = lib.bump(fixture(), { version: '1.3.0', title: 'Nice', date: '2026-10-14', android: '0.9.9' });
  assert.equal(lib.mainVersion(out.main), '1.3.0');
  assert.equal(lib.packageVersion(out.desktop), '1.3.0');
  assert.deepEqual(lib.lockVersions(out.lock), ['1.3.0', '1.3.0']);
  assert.equal(JSON.parse(out.lock).packages['node_modules/x'].version, '1.2.3', 'dependencies keep their own versions');
  assert.deepEqual(lib.androidVersion(out.android), { name: '0.9.9', code: 38 });
  assert.equal(lib.changelogSection(out.changelog, 'Unreleased').body, '');
  assert.equal(lib.changelogSection(out.changelog, '1.3.0').body, '- New thing.');
  assert.equal(lib.changelogTitle(lib.changelogSection(out.changelog, '1.3.0').heading), 'Nice');
  assert.deepEqual(lib.checkConsistency(out), []);
});

test('bump refuses releases that would be wrong', () => {
  const f = fixture();
  assert.throws(() => lib.bump(f, { version: '1.3', title: 'x', date: 'd' }), /look like/);
  assert.throws(() => lib.bump(f, { version: '1.2.3', title: 'x', date: 'd' }), /already at/);
  assert.throws(() => lib.bump(f, { version: '1.3.0', title: '', date: 'd' }), /title/);
  const empty = { ...f, changelog: '# Changelog\n\n## Unreleased\n\n## v1.2.3 - 2026-01-01 - Old\n\n- x\n' };
  assert.throws(() => lib.bump(empty, { version: '1.3.0', title: 'x', date: 'd' }), /empty/);
});

test('consistency check names the file that drifted', () => {
  const f = fixture();
  f.desktop = f.desktop.replace('1.2.3', '1.2.4');
  assert.match(lib.checkConsistency(f).join('\n'), /desktop\/package\.json is 1\.2\.4/);
});

test('release notes come from the changelog section', () => {
  const notes = lib.releaseNotes(fixture().changelog, '1.2.3');
  assert.match(notes, /^## Old\n\n- Old thing\./);
  assert.match(notes, /Gregal-Setup-1\.2\.3-x64\.exe/);
  assert.throws(() => lib.releaseNotes(fixture().changelog, '9.9.9'), /no notes/);
});
