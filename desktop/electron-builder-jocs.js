// Gregal Jocs: el mateix projecte que Gregal, empaquetat com una app a part
// (una altra appId, un altre nom, els seus instal·ladors i el seu canal
// d'actualitzacions, jocs.yml). main.js llegeix gregalEdition i només obre /jocs/.
// El backend és un altre binari, gregal-jocs(.exe) (jocs/cmd/gregal-jocs: el
// servidor de Gregal amb els jocs), que build-backend.js --jocs deixa a
// backend-jocs/ i aquí va a resources/backend/. El de Gregal no hi entra.
// Ús: node build-backend.js windows --jocs &&
//     npx electron-builder --config electron-builder-jocs.js --win nsis portable --publish never
const fs = require('fs');
const path = require('path');
const base = require('./package.json').build;
const { backendName } = require('./server-manager');

module.exports = {
  ...base,
  // Sense gregal-jocs compilat, l'app sortiria sense backend (o amb un de
  // vell): s'atura abans d'empaquetar i diu què cal fer.
  beforePack: async context => {
    const exe = backendName('jocs', context.electronPlatformName);
    if (!fs.existsSync(path.join(__dirname, 'backend-jocs', exe))) {
      throw new Error('Gregal Jocs: falta backend-jocs/' + exe + '. Compila\'l primer: node build-backend.js <windows|linux|darwin> --jocs (o npm run dist-win-jocs).');
    }
  },
  appId: 'xyz.solanes.gregal.jocs',
  productName: 'Gregal Jocs',
  extraMetadata: { name: 'gregal-jocs', productName: 'Gregal Jocs', gregalEdition: 'jocs' },
  extraResources: base.extraResources.map(r => (r.from === 'backend' ? { ...r, from: 'backend-jocs' } : r)),
  publish: { ...base.publish, channel: 'jocs' },
  nsis: {
    ...base.nsis,
    artifactName: 'Gregal-Jocs-Setup-${version}-${arch}.${ext}',
    include: 'build/installer-jocs.nsh',
    shortcutName: 'Gregal Jocs',
  },
  portable: { ...base.portable, artifactName: 'Gregal-Jocs-Portable-${version}-${arch}.${ext}' },
};
