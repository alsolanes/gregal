// Finestra pròpia d'un document (/app/doc/{id}, docs_api.go): el panell de
// documents a pàgina sencera, amb la mateixa vista, el mateix stream i la
// mateixa edició. Aquí no hi ha index.html: window.gregal és el mínim que
// docs-panel.js en fa servir.
const session = new URLSearchParams(location.search).get('session') || 'default';
const id = decodeURIComponent(location.pathname.split('/').pop());
let token = window.gregalDesktop?.apiToken;
if (typeof token !== 'string') {
  try { token = localStorage.getItem('gregal_token') || ''; } catch (e) { token = ''; }
}
const status = document.createElement('p');
status.setAttribute('role', 'status');
status.style.cssText = 'margin:0;padding:8px 20px';
window.gregal = {
  api: (url, opts = {}) => fetch(url, { ...opts, headers: { ...opts.headers, 'X-Gregal-Session': session, ...(token ? { Authorization: 'Bearer ' + token } : {}) } }),
  sys: m => { status.textContent = m; document.body.prepend(status); },
};
await import('./docs-panel.js');
window.gregalDocs.open(id);
const r = await window.gregal.api('/api/docs/' + encodeURIComponent(id));
if (r.ok) document.title = (await r.json()).title + ' · Gregal';
