const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { createRequire } = require('node:module');
const yaml = createRequire(require.resolve('electron-updater'))('js-yaml');

// edition: «gregal» (latest.yml, Gregal-Setup) o «jocs» (jocs.yml, Gregal-Jocs-Setup).
async function verifyRelease(directory, version = require('./package.json').version, edition = 'gregal') {
  const output = path.resolve(directory);
  const jocs = edition === 'jocs';
  const manifest = yaml.load(fs.readFileSync(path.join(output, jocs ? 'jocs.yml' : 'latest.yml'), 'utf8'));
  if (manifest?.version !== version) throw new Error('Update metadata version does not match the app');
  const prefix = jocs ? 'Gregal-Jocs' : 'Gregal';
  const installer = `${prefix}-Setup-${version}-x64.exe`;
  if (!Array.isArray(manifest.files) || !manifest.files.some(file => file.url === installer)) {
    throw new Error('Update metadata must reference the NSIS installer');
  }
  for (const file of manifest.files) {
    if (typeof file.url !== 'string' || path.basename(file.url) !== file.url || file.url.includes('\\') || file.url !== installer) {
      throw new Error('Update metadata contains an unexpected download path');
    }
    const artifact = path.join(output, file.url);
    const stat = fs.statSync(artifact);
    if (file.size !== stat.size) throw new Error('Update download size does not match the installer');
    const hash = crypto.createHash('sha512');
    for await (const chunk of fs.createReadStream(artifact)) hash.update(chunk);
    if (hash.digest('base64') !== file.sha512) throw new Error('Update checksum does not match the installer');
  }
  if (manifest.path && manifest.path !== installer) throw new Error('Legacy update path does not match the installer');
  if (manifest.sha512 && manifest.sha512 !== manifest.files.find(file => file.url === installer).sha512) {
    throw new Error('Legacy update checksum does not match the installer');
  }
  fs.accessSync(path.join(output, installer + '.blockmap'));
  fs.accessSync(path.join(output, `${prefix}-Portable-${version}-x64.exe`));
  // El backend empaquetat (si hi ha la carpeta desempaquetada): Gregal porta
  // gregal.exe, sense jocs; Gregal Jocs, gregal-jocs.exe. Mai l'altre.
  const backend = path.join(output, 'win-unpacked', 'resources', 'backend');
  if (fs.existsSync(backend)) {
    const want = jocs ? 'gregal-jocs.exe' : 'gregal.exe';
    const other = jocs ? 'gregal.exe' : 'gregal-jocs.exe';
    if (!fs.existsSync(path.join(backend, want))) throw new Error(`The packaged backend must be ${want}`);
    if (fs.existsSync(path.join(backend, other))) throw new Error(`The packaged backend must not include ${other}`);
  }
  return { version, installer };
}

module.exports = { verifyRelease };
if (require.main === module) {
  verifyRelease(process.argv[2] || path.join(__dirname, 'dist'), undefined, process.argv.includes('--jocs') ? 'jocs' : 'gregal').then(result => {
    console.log(`Update metadata verified: ${result.installer}`);
  }).catch(error => { console.error(error.message); process.exitCode = 1; });
}
