// Gregal Jocs no porta el backend dins de l'instal·lador: el baixa la
// primera vegada que s'obre (i després de cada actualització de l'app) de la
// release de la seva versió a alsolanes/gregal-jocs, el verifica amb el
// sha256 del manifest i el deixa a <userData>/backend/<versió>/. Les
// versions velles s'esborren quan la nova ja hi és.
//
// A la release hi ha d'haver, a més de l'instal·lador:
//   gregal-jocs-backend.json  {version, files: {"win32-x64": {name, sha256, size}}}
//   gregal-jocs-<versió>-win32-x64.exe (el nom que digui el manifest)
// scripts/release.cjs jocs X.Y.Z els genera i els penja.
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { backendName } = require('./server-manager');

const JOCS_RELEASES_REPO = 'alsolanes/gregal-jocs';
const MANIFEST_NAME = 'gregal-jocs-backend.json';
const OK_FILE = '.ok';

function releaseBase(version, env = process.env) {
  const override = String(env.GREGAL_JOCS_BACKEND_BASE || '').trim();
  // L'arrel equivalent a .../releases/download (un mirall, o els tests).
  if (override) return `${override.replace(/\/+$/, '')}/v${version}`;
  return `https://github.com/${JOCS_RELEASES_REPO}/releases/download/v${version}`;
}

function platformKey(platform = process.platform, arch = process.arch) {
  return `${platform}-${arch}`;
}

function backendDir(userData, version) {
  return path.join(userData, 'backend', String(version));
}

// installedBackend torna el camí del backend ja baixat i verificat d'aquesta
// versió, o null.
function installedBackend({ userData, version, platform = process.platform }) {
  const dir = backendDir(userData, version);
  const exe = path.join(dir, backendName('jocs', platform));
  try {
    const ok = JSON.parse(fs.readFileSync(path.join(dir, OK_FILE), 'utf8'));
    if (fs.statSync(exe).size === ok.size) return exe;
  } catch (_) { /* no hi és o està a mitges */ }
  return null;
}

async function fetchOK(url, fetchImpl, signal) {
  let res;
  try {
    res = await fetchImpl(url, { redirect: 'follow', signal });
  } catch (error) {
    // «fetch failed» tot sol no diu res: l'adreça i la causa (ENOTFOUND,
    // ECONNREFUSED, un certificat…) van als detalls d'error.html.
    const cause = error?.cause?.code || error?.cause?.message || error?.message || String(error);
    throw new Error(`${url}: ${cause}`);
  }
  if (!res.ok) throw new Error(`${url}: HTTP ${res.status}`);
  return res;
}

// ensureJocsBackend torna el camí del backend d'aquesta versió, baixant-lo
// si cal. onProgress rep {received, total} mentre baixa.
async function ensureJocsBackend({
  userData, version, platform = process.platform, arch = process.arch,
  env = process.env, fetchImpl = globalThis.fetch, onProgress = () => {}, signal,
}) {
  const ready = installedBackend({ userData, version, platform });
  if (ready) return ready;

  const base = releaseBase(version, env);
  const manifest = await (await fetchOK(`${base}/${MANIFEST_NAME}`, fetchImpl, signal)).json();
  const entry = manifest?.files?.[platformKey(platform, arch)];
  if (!entry || !entry.name || !/^[0-9a-f]{64}$/i.test(entry.sha256 || '')) {
    throw new Error(`La release v${version} de Gregal Jocs no té motor per a ${platformKey(platform, arch)}.`);
  }
  if (path.basename(entry.name) !== entry.name) throw new Error(`Nom de fitxer no vàlid al manifest: ${entry.name}`);

  const dir = backendDir(userData, version);
  fs.mkdirSync(dir, { recursive: true });
  const exe = path.join(dir, backendName('jocs', platform));
  const tmp = exe + '.part';
  const res = await fetchOK(`${base}/${entry.name}`, fetchImpl, signal);
  const total = Number(res.headers.get('content-length')) || Number(entry.size) || 0;
  const hash = crypto.createHash('sha256');
  const out = fs.createWriteStream(tmp);
  let received = 0;
  try {
    const reader = res.body.getReader();
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      hash.update(value);
      received += value.length;
      if (!out.write(value)) await new Promise(resolve => out.once('drain', resolve));
      onProgress({ received, total });
    }
    await new Promise((resolve, reject) => out.end(err => (err ? reject(err) : resolve())));
  } catch (error) {
    out.destroy();
    fs.rmSync(tmp, { force: true });
    throw error;
  }
  const sha = hash.digest('hex');
  if (sha.toLowerCase() !== entry.sha256.toLowerCase() || (entry.size && received !== Number(entry.size))) {
    fs.rmSync(tmp, { force: true });
    throw new Error(`El motor baixat no és correcte (sha256 ${sha}, esperat ${entry.sha256}).`);
  }
  fs.renameSync(tmp, exe);
  if (platform !== 'win32') fs.chmodSync(exe, 0o755);
  fs.writeFileSync(path.join(dir, OK_FILE), JSON.stringify({ sha256: sha, size: received, name: entry.name }));
  pruneOldBackends(userData, version);
  return exe;
}

// pruneOldBackends esborra els backends d'altres versions (un motor vell no
// serveix per a res: l'app sempre fa servir el de la seva versió).
function pruneOldBackends(userData, keepVersion) {
  const root = path.join(userData, 'backend');
  let names = [];
  try { names = fs.readdirSync(root); } catch (_) { return; }
  for (const name of names) {
    if (name === String(keepVersion)) continue;
    try { fs.rmSync(path.join(root, name), { recursive: true, force: true }); } catch (_) { /* en ús: ja s'esborrarà */ }
  }
}

module.exports = { JOCS_RELEASES_REPO, MANIFEST_NAME, releaseBase, platformKey, backendDir, installedBackend, ensureJocsBackend, pruneOldBackends };
