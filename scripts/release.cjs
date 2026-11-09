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
//       Builds the Android APKs locally (one per ABI plus the universal one)
//       and attaches them to the v1.9.1 release with the in-app updater's
//       manifest, gregal-android-version.json (the APKs are not built in
//       CI: they need this machine's signing key).
//   node scripts/release.cjs apk 1.9.1 --vents
//       The same for Gregal Vents (-Pgregal.plotsPreview): Gregal-Vents-*.apk
//       and gregal-vents-version.json, which the Vents app's updater reads.
'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const lib = require('./release-lib.cjs');

const root = path.resolve(__dirname, '..');
const read = rel => fs.readFileSync(path.join(root, rel), 'utf8');

// La signatura dels APK no pot canviar mai entre versions: Android rebutja
// instal·lar una actualització signada amb una altra clau
// (INSTALL_FAILED_UPDATE_INCOMPATIBLE) i qui ja té l'app es queda sense poder
// actualitzar-se. Els APK es signen amb el keystore de depuració d'aquesta
// màquina; si algú els construeix en un altre entorn (contenidor, una altra
// HOME, un altre ordinador) la signatura és una altra. Aquest guard ho atura
// abans de penjar res a la release.
// És la clau amb què s'han publicat totes les versions que hi ha a GitHub
// (Vents 1.66–1.68, Android 1.62): CN=Android Debug d'aquest ordinador.
const EXPECTED_CERT_SHA256 = '1d8afee44182e2190c99405052374aca9666b0170b29cad57f56a229e8fa6c6b';

// apksigner viu a <sdk>/build-tools/<versió>/: es busca el primer que hi hagi.
function apksignerPath() {
  const bases = [];
  for (const env of [process.env.ANDROID_HOME, process.env.ANDROID_SDK_ROOT]) if (env) bases.push(env);
  try {
    const m = /^\s*sdk\.dir=(.+)$/m.exec(read(path.join('android', 'local.properties')));
    if (m) bases.push(m[1].trim());
  } catch { /* sense local.properties: es proven els llocs de sempre */ }
  bases.push(path.join(require('node:os').homedir(), 'Android'), path.join(require('node:os').homedir(), 'android-sdk'));
  for (const base of bases) {
    let versions = [];
    try { versions = fs.readdirSync(path.join(base, 'build-tools')).sort().reverse(); } catch { continue; }
    for (const v of versions) {
      // A Windows, «apksigner» sense extensió és l'script de shell i no
      // s'executa: cal el .bat.
      for (const exe of process.platform === 'win32' ? ['apksigner.bat'] : ['apksigner', 'apksigner.bat']) {
        const p = path.join(base, 'build-tools', v, exe);
        if (fs.existsSync(p)) return p;
      }
    }
  }
  return null;
}

function apkCertSha256(apk) {
  const signer = apksignerPath();
  if (!signer) return null;
  try {
    // Un .bat només s'executa a través del shell (i llavors cal posar
    // cometes a les rutes).
    const bat = /\.bat$/i.test(signer);
    const q = a => (bat ? '"' + a + '"' : a);
    const out = execFileSync(q(signer), ['verify', '--print-certs', q(apk)], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'], shell: bat });
    const m = /SHA-256 digest:\s*([0-9a-f]{64})/i.exec(out);
    return m ? m[1].toLowerCase() : null;
  } catch { return null; }
}
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
  const vents = process.argv.includes('--vents');
  const a = (vents ? lib.ventsVersion : lib.androidVersion)(read(lib.FILES.android));
  if (!a.name || !a.code) fail('could not read the ' + (vents ? 'Vents ' : '') + 'Android version from ' + lib.FILES.android);
  const android = path.join(root, 'android');
  // Ruta absoluta: cmd no sempre busca a la carpeta actual.
  const gradle = path.join(android, process.platform === 'win32' ? 'gradlew.bat' : 'gradlew');
  console.log(`building Android ${a.name} (${a.code})…`);
  execFileSync(gradle, ['testDebugUnitTest', 'assembleRelease', '-q', ...(vents ? ['-Pgregal.plotsPreview'] : [])], { cwd: android, stdio: 'inherit', shell: process.platform === 'win32' });
  const dir = path.join(root, 'output', 'release-' + value);
  fs.mkdirSync(dir, { recursive: true });
  const built = path.join(android, 'app/build/outputs/apk/release');
  const apks = lib.ANDROID_APKS.map(({ abi }) => {
    const from = lib.androidApkFrom(a.name, abi, vents);
    const name = (vents ? lib.ventsApkName : lib.androidApkName)(a.name, abi);
    const file = path.join(dir, name);
    fs.copyFileSync(path.join(built, from), file);
    const buf = fs.readFileSync(file);
    const sha256 = crypto.createHash('sha256').update(buf).digest('hex');
    console.log(`  ${name}  ${(buf.length / 1048576).toFixed(1)} MiB`);
    const cert = apkCertSha256(file);
    if (cert !== EXPECTED_CERT_SHA256) {
      fail(`${name} està signat amb una altra clau (${cert || 'no s\'ha pogut llegir la signatura'}).\n` +
        `  Android rebutjaria l\'actualització a qui ja té l\'app instal·lada (INSTALL_FAILED_UPDATE_INCOMPATIBLE).\n` +
        `  Construeix els APK en aquesta màquina, amb el keystore ~/.android/debug.keystore (el de sempre).`);
    }
    return { abi, name, file, sha256, sizeBytes: buf.length };
  });
  const repo = execFileSync('gh', ['repo', 'view', '--json', 'nameWithOwner', '-q', '.nameWithOwner'], { cwd: root, encoding: 'utf8' }).trim();
  const sec = lib.changelogSection(read(lib.FILES.changelog), value);
  const manifest = lib.androidManifest({
    versionName: a.name, versionCode: a.code,
    changelog: sec ? lib.changelogTitle(sec.heading) : '',
    baseUrl: `https://github.com/${repo}/releases/download/v${value}/`,
    apks,
  });
  const manifestName = vents ? 'gregal-vents-version.json' : 'gregal-android-version.json';
  const manifestFile = path.join(dir, manifestName);
  fs.writeFileSync(manifestFile, JSON.stringify(manifest, null, 2) + '\n');
  execFileSync('gh', ['release', 'upload', 'v' + value, ...apks.map(x => x.file), manifestFile, '--clobber'], { cwd: root, stdio: 'inherit' });
  console.log(`attached ${apks.length} APKs and ${manifestName} to v${value}`);
} else if (cmd === 'cert') {
  const apk = process.argv[3];
  if (!apk) fail('usage: release.cjs cert <apk>');
  const c = apkCertSha256(apk);
  if (!c) fail('no s\'ha pogut llegir la signatura de ' + apk);
  console.log(c + (c === EXPECTED_CERT_SHA256 ? '  (la clau de sempre)' : '  (NO és la clau de sempre: aquest APK no es pot publicar)'));
} else {
  fail('usage: release.cjs check | bump X.Y.Z --title "..." [--android A.B.C] | notes X.Y.Z | title X.Y.Z | apk X.Y.Z [--vents] | cert <apk>');
}
