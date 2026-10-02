const { pathToFileURL } = require('url');

function sameOrigin(url, base) {
  try {
    const target = new URL(url);
    return ['http:', 'https:'].includes(target.protocol) && target.origin === new URL(base).origin && !target.username && !target.password;
  } catch { return false; }
}

function externalURL(url) {
  try {
    const target = new URL(url);
    return ['http:', 'https:'].includes(target.protocol) && !target.username && !target.password;
  } catch { return false; }
}

function localServiceURL(url) {
  try {
    const target = new URL(url);
    return target.protocol === 'http:' && ['127.0.0.1', '[::1]', 'localhost'].includes(target.hostname) && !target.username && !target.password && target.pathname === '/' && !target.search && !target.hash;
  } catch { return false; }
}

function trustedPage(url, base, errorPath) {
  return sameOrigin(url, base) || url.split('?')[0] === pathToFileURL(errorPath).href;
}

module.exports = { sameOrigin, externalURL, localServiceURL, trustedPage };
