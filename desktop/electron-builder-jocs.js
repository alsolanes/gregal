// Gregal Jocs: el mateix projecte que Gregal, empaquetat com una app a part
// (una altra appId, un altre nom, els seus instal·ladors i el seu canal
// d'actualitzacions, jocs.yml). main.js llegeix gregalEdition i només obre /jocs/.
// Ús: npx electron-builder --config electron-builder-jocs.js --win nsis portable --publish never
const base = require('./package.json').build;

module.exports = {
  ...base,
  appId: 'xyz.solanes.gregal.jocs',
  productName: 'Gregal Jocs',
  extraMetadata: { name: 'gregal-jocs', productName: 'Gregal Jocs', gregalEdition: 'jocs' },
  publish: { ...base.publish, channel: 'jocs' },
  nsis: {
    ...base.nsis,
    artifactName: 'Gregal-Jocs-Setup-${version}-${arch}.${ext}',
    include: 'build/installer-jocs.nsh',
    shortcutName: 'Gregal Jocs',
  },
  portable: { ...base.portable, artifactName: 'Gregal-Jocs-Portable-${version}-${arch}.${ext}' },
};
