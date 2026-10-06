const { pathToFileURL } = require('url');
const net = require('node:net');

function sameOrigin(url, base) {
  try {
    const target = new URL(url);
    return ['http:', 'https:'].includes(target.protocol) && target.origin === new URL(base).origin && !target.username && !target.password;
  } catch { return false; }
}

function isLoopbackHost(hostname) {
  const host = hostname.startsWith('[') && hostname.endsWith(']')
    ? hostname.slice(1, -1)
    : hostname;
  if (host.toLowerCase() === 'localhost') return true;
  const version = net.isIP(host);
  if (version === 4) return Number(host.split('.')[0]) === 127;
  return version === 6 && host === '::1';
}

function backendURLError(url) {
  try {
    const target = new URL(url);
    if (target.username || target.password) return 'GREGAL_URL must not contain embedded credentials';
    if (!['http:', 'https:'].includes(target.protocol)) return 'GREGAL_URL must be an HTTP(S) URL';
    if (target.protocol === 'http:' && !isLoopbackHost(target.hostname)) {
      return 'GREGAL_URL must use HTTPS for non-loopback servers; HTTP is allowed only on loopback';
    }
    return '';
  } catch { return 'GREGAL_URL must be a valid HTTP(S) URL'; }
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

module.exports = { sameOrigin, externalURL, backendURLError, localServiceURL, trustedPage };
