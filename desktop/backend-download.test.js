const test = require('node:test');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const os = require('node:os');
const path = require('node:path');
const { ensureJocsBackend, installedBackend, releaseBase, backendDir } = require('./backend-download');

// Una release de GitHub de mentida: el manifest i el binari (amb una
// redirecció, com fa GitHub cap a objects.githubusercontent.com).
function fakeRelease(t, { version = '2.0.0', binary = Buffer.from('motor de jocs'), sha256, platform = 'win32-x64' } = {}) {
  const hits = [];
  const server = http.createServer((req, res) => {
    hits.push(req.url);
    if (req.url === `/v${version}/gregal-jocs-backend.json`) {
      res.setHeader('content-type', 'application/json');
      return res.end(JSON.stringify({
        version,
        files: { [platform]: { name: `gregal-jocs-${version}-${platform}.exe`, sha256: sha256 || crypto.createHash('sha256').update(binary).digest('hex'), size: binary.length } },
      }));
    }
    if (req.url === `/v${version}/gregal-jocs-${version}-${platform}.exe`) {
      res.writeHead(302, { location: '/blob' });
      return res.end();
    }
    if (req.url === '/blob') {
      res.setHeader('content-length', binary.length);
      return res.end(binary);
    }
    res.writeHead(404);
    res.end();
  });
  return new Promise(resolve => server.listen(0, '127.0.0.1', () => {
    t.after(() => server.close());
    resolve({ base: `http://127.0.0.1:${server.address().port}`, hits });
  }));
}

function tmpUserData(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gregal-jocs-userdata-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}

test('baixa el motor de la seva versió, el verifica i la segona vegada no torna a baixar res', async t => {
  const { base, hits } = await fakeRelease(t);
  const userData = tmpUserData(t);
  const env = { GREGAL_JOCS_BACKEND_BASE: base };
  const progress = [];
  const exe = await ensureJocsBackend({ userData, version: '2.0.0', platform: 'win32', arch: 'x64', env, onProgress: p => progress.push(p) });
  assert.equal(exe, path.join(backendDir(userData, '2.0.0'), 'gregal-jocs.exe'));
  assert.equal(fs.readFileSync(exe, 'utf8'), 'motor de jocs');
  assert.ok(progress.length > 0 && progress.at(-1).received === progress.at(-1).total);
  assert.equal(installedBackend({ userData, version: '2.0.0', platform: 'win32' }), exe);

  const before = hits.length;
  assert.equal(await ensureJocsBackend({ userData, version: '2.0.0', platform: 'win32', arch: 'x64', env }), exe);
  assert.equal(hits.length, before, 'ja baixat: cap petició');
});

test('una versió nova esborra el motor de la vella', async t => {
  const userData = tmpUserData(t);
  const old = await fakeRelease(t, { version: '1.0.0' });
  await ensureJocsBackend({ userData, version: '1.0.0', platform: 'win32', arch: 'x64', env: { GREGAL_JOCS_BACKEND_BASE: old.base } });
  const nou = await fakeRelease(t, { version: '1.1.0' });
  await ensureJocsBackend({ userData, version: '1.1.0', platform: 'win32', arch: 'x64', env: { GREGAL_JOCS_BACKEND_BASE: nou.base } });
  assert.deepEqual(fs.readdirSync(path.join(userData, 'backend')), ['1.1.0']);
});

test('un sha256 que no quadra no deixa cap motor (ni a mitges)', async t => {
  const { base } = await fakeRelease(t, { sha256: 'a'.repeat(64) });
  const userData = tmpUserData(t);
  await assert.rejects(
    ensureJocsBackend({ userData, version: '2.0.0', platform: 'win32', arch: 'x64', env: { GREGAL_JOCS_BACKEND_BASE: base } }),
    /no és correcte/,
  );
  assert.equal(installedBackend({ userData, version: '2.0.0', platform: 'win32' }), null);
  assert.deepEqual(fs.readdirSync(backendDir(userData, '2.0.0')), []);
});

test('sense motor per a la plataforma, o sense release, l\'error ho diu', async t => {
  const { base } = await fakeRelease(t, { platform: 'linux-x64' });
  const userData = tmpUserData(t);
  const env = { GREGAL_JOCS_BACKEND_BASE: base };
  await assert.rejects(ensureJocsBackend({ userData, version: '2.0.0', platform: 'win32', arch: 'x64', env }), /win32-x64/);
  await assert.rejects(ensureJocsBackend({ userData, version: '9.9.9', platform: 'win32', arch: 'x64', env }), /HTTP 404/);
});

test('per defecte baixa de les releases d\'alsolanes/gregal-jocs', () => {
  assert.equal(releaseBase('1.2.3', {}), 'https://github.com/alsolanes/gregal-jocs/releases/download/v1.2.3');
  assert.equal(releaseBase('1.2.3', { GREGAL_JOCS_BACKEND_BASE: 'http://x/y/' }), 'http://x/y/v1.2.3');
});
