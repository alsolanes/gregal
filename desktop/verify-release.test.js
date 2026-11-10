const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const crypto = require('node:crypto');
const { verifyRelease } = require('./verify-release');

test('release metadata rejects mismatched versions, portable targets and tampered installers', async t => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gregal-release-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  const installer = 'Gregal-Setup-1.7.3-x64.exe';
  const contents = Buffer.from('test installer');
  const checksum = crypto.createHash('sha512').update(contents).digest('base64');
  fs.writeFileSync(path.join(dir, installer), contents);
  fs.writeFileSync(path.join(dir, installer + '.blockmap'), 'map');
  fs.writeFileSync(path.join(dir, 'Gregal-Portable-1.7.3-x64.exe'), 'portable');
  const manifest = (version, file = installer) => `version: ${version}\nfiles:\n  - url: ${file}\n    sha512: ${checksum}\n    size: ${contents.length}\n`;
  fs.writeFileSync(path.join(dir, 'latest.yml'), manifest('1.7.3'));
  assert.equal((await verifyRelease(dir, '1.7.3')).installer, installer);
  fs.writeFileSync(path.join(dir, 'latest.yml'), manifest('1.7.2'));
  await assert.rejects(verifyRelease(dir, '1.7.3'), /version/);
  fs.writeFileSync(path.join(dir, 'latest.yml'), manifest('1.7.3', 'Gregal-Portable-1.7.3-x64.exe'));
  await assert.rejects(verifyRelease(dir, '1.7.3'), /NSIS/);
  fs.writeFileSync(path.join(dir, 'latest.yml'), manifest('1.7.3'));
  fs.writeFileSync(path.join(dir, installer), 'tampered bytes');
  await assert.rejects(verifyRelease(dir, '1.7.3'), /checksum/);
});

test('Gregal Jocs release carries its own backend, gregal-jocs.exe, and never the Gregal one', async t => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gregal-jocs-release-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  const installer = 'Gregal-Jocs-Setup-1.7.3-x64.exe';
  const contents = Buffer.from('jocs installer');
  const checksum = crypto.createHash('sha512').update(contents).digest('base64');
  fs.writeFileSync(path.join(dir, installer), contents);
  fs.writeFileSync(path.join(dir, installer + '.blockmap'), 'map');
  fs.writeFileSync(path.join(dir, 'Gregal-Jocs-Portable-1.7.3-x64.exe'), 'portable');
  fs.writeFileSync(path.join(dir, 'jocs.yml'), `version: 1.7.3\nfiles:\n  - url: ${installer}\n    sha512: ${checksum}\n    size: ${contents.length}\n`);
  assert.equal((await verifyRelease(dir, '1.7.3', 'jocs')).installer, installer, 'without win-unpacked only the metadata is checked');
  const backend = path.join(dir, 'win-unpacked', 'resources', 'backend');
  fs.mkdirSync(backend, { recursive: true });
  fs.writeFileSync(path.join(backend, 'gregal.exe'), 'gregal');
  await assert.rejects(verifyRelease(dir, '1.7.3', 'jocs'), /must be gregal-jocs\.exe/);
  fs.writeFileSync(path.join(backend, 'gregal-jocs.exe'), 'jocs');
  await assert.rejects(verifyRelease(dir, '1.7.3', 'jocs'), /must not include gregal\.exe/);
  fs.rmSync(path.join(backend, 'gregal.exe'));
  assert.equal((await verifyRelease(dir, '1.7.3', 'jocs')).installer, installer);
});
