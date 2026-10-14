#!/usr/bin/env node
// The single entry point for releases (see docs/release.md).
//
//   node scripts/release.cjs check
//       Fails if the versions or the changelog are inconsistent (CI runs it).
//   node scripts/release.cjs bump 1.9.1 --title "Short title" [--android 0.9.9]
//       Sets the version everywhere and turns "## Unreleased" into the
//       release section (a fresh empty "## Unreleased" stays on top).
//   node scripts/release.cjs notes 1.9.1
//       Prints the GitHub release notes taken from CHANGELOG.md (CI uses it).
//   node scripts/release.cjs title 1.9.1
//       Prints the release title ("Gregal v1.9.1 — <changelog title>").
//   node scripts/release.cjs apk 1.9.1
//       Builds the Android APK locally and attaches it to the v1.9.1 release
//       (the APK is not built in CI: it needs this machine's signing key).
'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const lib = require('./release-lib.cjs');

const root = path.resolve(__dirname, '..');
const read = rel => fs.readFileSync(path.join(root, rel), 'utf8');
const write = (rel, text) => fs.writeFileSync(path.join(root, rel), text);

function loadAll() {
  const out = {};
  for (const [k, rel] of Object.entries(lib.FILES)) out[k] = read(rel);
  return out;
}

function arg(name) {
  const i = process.argv.indexOf(name);
  return i > 0 ? process.argv[i + 1] : undefined;
}

function fail(msg) {
  console.error('release: ' + msg);
  process.exit(1);
}

const [cmd, value] = process.argv.slice(2);

if (cmd === 'check') {
  const problems = lib.checkConsistency(loadAll());
  if (problems.length) fail('inconsistent release metadata:\n  - ' + problems.join('\n  - '));
  console.log('release metadata OK: v' + lib.mainVersion(read(lib.FILES.main)));
} else if (cmd === 'bump') {
  const files = loadAll();
  let out;
  try {
    out = lib.bump(files, { version: value, title: arg('--title'), date: new Date().toISOString().slice(0, 10), android: arg('--android') });
  } catch (e) { fail(e.message); }
  for (const [k, rel] of Object.entries(lib.FILES)) if (out[k] !== files[k]) { write(rel, out[k]); console.log('updated ' + rel); }
  const problems = lib.checkConsistency(out);
  if (problems.length) fail('bump left inconsistencies:\n  - ' + problems.join('\n  - '));
  console.log(`\nNext: open a PR with these changes, merge it, then tag:\n  git tag -a v${value} -m "Gregal v${value}" && git push origin v${value}\nCI builds every asset and creates a draft release with the notes from CHANGELOG.md.`);
} else if (cmd === 'notes') {
  try { process.stdout.write(lib.releaseNotes(read(lib.FILES.changelog), value) + '\n'); } catch (e) { fail(e.message); }
} else if (cmd === 'title') {
  const sec = lib.changelogSection(read(lib.FILES.changelog), value || '');
  const t = sec ? lib.changelogTitle(sec.heading) : '';
  process.stdout.write('Gregal v' + value + (t ? ' — ' + t : '') + '\n');
} else if (cmd === 'apk') {
  if (!lib.SEMVER.test(value || '')) fail('usage: release.cjs apk X.Y.Z');
  const a = lib.androidVersion(read(lib.FILES.android));
  const android = path.join(root, 'android');
  const gradle = process.platform === 'win32' ? 'gradlew.bat' : './gradlew';
  console.log(`building Android ${a.name} (${a.code})…`);
  execFileSync(gradle, ['testDebugUnitTest', 'assembleDebug', '-q'], { cwd: android, stdio: 'inherit', shell: process.platform === 'win32' });
  const dir = path.join(root, 'output', 'release-' + value);
  fs.mkdirSync(dir, { recursive: true });
  const apk = path.join(dir, `Gregal-Android-${a.name}.apk`);
  fs.copyFileSync(path.join(android, 'app/build/outputs/apk/debug/app-debug.apk'), apk);
  execFileSync('gh', ['release', 'upload', 'v' + value, apk, '--clobber'], { cwd: root, stdio: 'inherit' });
  console.log('attached ' + path.basename(apk) + ' to v' + value);
} else {
  fail('usage: release.cjs check | bump X.Y.Z --title "..." [--android A.B.C] | notes X.Y.Z | title X.Y.Z | apk X.Y.Z');
}
