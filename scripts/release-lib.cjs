// Release helpers: one place that knows where every version lives and how
// the changelog is laid out. Pure functions over file contents, so the tests
// (release.test.cjs) and the CLI (release.cjs) share exactly the same rules.
'use strict';

const SEMVER = /^\d+\.\d+\.\d+$/;

// Every file that carries the product version. The VS Code extension and the
// Android app have their own version lines on purpose (see docs/release.md).
const FILES = {
  main: 'main.go',
  desktop: 'desktop/package.json',
  lock: 'desktop/package-lock.json',
  changelog: 'CHANGELOG.md',
  android: 'android/app/build.gradle.kts',
  vscode: 'vscode-gregal/package.json',
};

function mainVersion(src) {
  const m = /var version = "v(\d+\.\d+\.\d+)"/.exec(src);
  return m ? m[1] : '';
}

function packageVersion(src) {
  try { return JSON.parse(src).version || ''; } catch { return ''; }
}

// package-lock.json repeats the version at the root and in packages[""].
function lockVersions(src) {
  try {
    const j = JSON.parse(src);
    return [j.version || '', (j.packages && j.packages[''] && j.packages[''].version) || ''];
  } catch { return ['', '']; }
}

function androidVersion(src) {
  const name = /versionName = "([^"]+)"/.exec(src);
  const code = /versionCode = (\d+)/.exec(src);
  return { name: name ? name[1] : '', code: code ? Number(code[1]) : 0 };
}

// Sections are "## vX.Y.Z - YYYY-MM-DD - Title" (title optional) plus one
// "## Unreleased" at the top.
function changelogSection(src, version) {
  const lines = src.split(/\r?\n/);
  const head = version === 'Unreleased'
    ? l => /^## Unreleased\s*$/.test(l)
    : l => l.startsWith('## v' + version + ' ') || l === '## v' + version;
  const start = lines.findIndex(head);
  if (start < 0) return null;
  let end = lines.findIndex((l, i) => i > start && /^## /.test(l));
  if (end < 0) end = lines.length;
  return { heading: lines[start], body: lines.slice(start + 1, end).join('\n').trim() };
}

function changelogTitle(heading) {
  const m = /^## v\d+\.\d+\.\d+ - \d{4}-\d{2}-\d{2}(?: - (.+))?$/.exec(heading || '');
  return m && m[1] ? m[1].trim() : '';
}

// Problems that would make a release inconsistent. Empty array = fine.
function checkConsistency(files) {
  const problems = [];
  const v = mainVersion(files.main);
  if (!SEMVER.test(v)) problems.push(`${FILES.main}: no "var version = \\"vX.Y.Z\\"" line`);
  const d = packageVersion(files.desktop);
  if (d !== v) problems.push(`${FILES.desktop} is ${d || '?'} but ${FILES.main} is ${v || '?'}`);
  const [l1, l2] = lockVersions(files.lock);
  if (l1 !== v || l2 !== v) problems.push(`${FILES.lock} is ${l1 || '?'}/${l2 || '?'} but ${FILES.main} is ${v || '?'}`);
  if (!changelogSection(files.changelog, 'Unreleased')) problems.push(`${FILES.changelog}: missing "## Unreleased" section`);
  if (v && !changelogSection(files.changelog, v)) problems.push(`${FILES.changelog}: missing "## v${v} - YYYY-MM-DD - Title" section`);
  return problems;
}

// Returns the new file contents for a release bump. Throws with a readable
// message when the request doesn't make sense.
function bump(files, { version, title, date, android }) {
  if (!SEMVER.test(version || '')) throw new Error(`version must look like 1.2.3, got "${version}"`);
  const current = mainVersion(files.main);
  if (current === version) throw new Error(`already at ${version}`);
  const unreleased = changelogSection(files.changelog, 'Unreleased');
  if (!unreleased) throw new Error('CHANGELOG.md has no "## Unreleased" section');
  if (!unreleased.body) throw new Error('CHANGELOG.md "## Unreleased" is empty: write what changed first');
  if (changelogSection(files.changelog, version)) throw new Error(`CHANGELOG.md already has a v${version} section`);
  if (!title || !title.trim()) throw new Error('a short release title is required (--title "...")');
  const out = { ...files };
  out.main = files.main.replace(/var version = "v\d+\.\d+\.\d+"/, `var version = "v${version}"`);
  const desk = JSON.parse(files.desktop);
  out.desktop = files.desktop.replace(`"version": "${desk.version}"`, `"version": "${version}"`);
  // Only the two root entries of the lock file are ours; dependencies keep theirs.
  const lock = JSON.parse(files.lock);
  lock.version = version;
  if (lock.packages && lock.packages['']) lock.packages[''].version = version;
  const eol = files.lock.includes('\r\n') ? '\r\n' : '\n';
  out.lock = JSON.stringify(lock, null, 2).replace(/\n/g, eol) + eol;
  out.changelog = files.changelog.replace(/^## Unreleased[ \t]*\r?$/m,
    `## Unreleased\n\n## v${version} - ${date} - ${title.trim()}`);
  if (android) {
    if (!SEMVER.test(android)) throw new Error(`--android must look like 0.9.9, got "${android}"`);
    const a = androidVersion(files.android);
    out.android = files.android
      .replace(/versionName = "[^"]+"/, `versionName = "${android}"`)
      .replace(/versionCode = \d+/, `versionCode = ${a.code + 1}`);
  }
  return out;
}

// Release notes for GitHub: the changelog section plus the download list.
function releaseNotes(changelog, version) {
  const s = changelogSection(changelog, version);
  if (!s || !s.body) throw new Error(`CHANGELOG.md has no notes for v${version}`);
  const title = changelogTitle(s.heading);
  return [
    title ? `## ${title}` : '', '', s.body, '',
    '## Downloads', '',
    `- **Gregal-Setup-${version}-x64.exe**: Windows installer with automatic updates.`,
    `- **Gregal-Portable-${version}-x64.exe**: portable Windows app.`,
    '- **gregal-<os>-<arch>**: standalone backend/CLI for Linux, macOS and Windows (amd64 and arm64).',
    '- **Gregal-Android-<version>-arm64-v8a.apk**: Android app for phones (debug-signed, install manually). `-x86_64` is for emulators; the plain `Gregal-Android-<version>.apk` works on both but is larger.',
    '- **SHA256SUMS.txt**: checksums for the CI-built files. Installer updates use the included latest.yml and blockmap.',
    '',
    'Windows packages are unsigned.',
  ].join('\n').replace(/^\n+/, '');
}

// The Android APKs attached to a release: AGP's per-ABI splits plus the
// universal APK. The universal one keeps the old name, so older apps (and
// people who don't know their phone's ABI) still get a working file.
const ANDROID_APKS = [
  { abi: null, from: 'app-universal-debug.apk' },
  { abi: 'arm64-v8a', from: 'app-arm64-v8a-debug.apk' },
  { abi: 'x86_64', from: 'app-x86_64-debug.apk' },
];

function androidApkName(versionName, abi) {
  return `Gregal-Android-${versionName}${abi ? '-' + abi : ''}.apk`;
}

// The in-app updater's manifest (gregal-android-version.json). The top-level
// downloadUrl/sha256/sizeBytes describe the universal APK, which is what
// apps from before the splits read; "apks" has one entry per ABI and the
// app picks the first of Build.SUPPORTED_ABIS listed there.
// apks: [{ abi, name, sha256, sizeBytes }], baseUrl ends with "/".
function androidManifest({ versionName, versionCode, changelog, baseUrl, apks }) {
  const entry = a => ({ downloadUrl: baseUrl + a.name, sha256: a.sha256, sizeBytes: a.sizeBytes });
  const universal = apks.find(a => !a.abi);
  if (!universal) throw new Error('the universal APK is missing');
  const perAbi = {};
  for (const a of apks) if (a.abi) perAbi[a.abi] = entry(a);
  return { versionCode, versionName, changelog: changelog || '', ...entry(universal), apks: perAbi };
}

module.exports = { ANDROID_APKS, androidApkName, androidManifest, FILES, SEMVER, mainVersion, packageVersion, lockVersions, androidVersion, changelogSection, changelogTitle, checkConsistency, bump, releaseNotes };
