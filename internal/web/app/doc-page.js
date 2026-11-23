// Finestra pròpia d'un document (/app/doc/{id}, docs_api.go): el panell de
// documents a pàgina sencera, amb la mateixa vista, el mateix stream i la
// mateixa edició. Aquí no hi ha index.html: window.gregal és el mínim que
// docs-panel.js en fa servir.
const query = new URLSearchParams(location.search);
const session = query.get('session') || 'default';
// scope=mine: un document de «Els meus documents» (els dels vents), no del projecte.
const scope = query.get('scope') === 'mine' ? 'mine' : '';
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
window.gregalDocs.open(id, scope);
const r = await window.gregal.api('/api/docs/' + encodeURIComponent(id) + (scope ? '?scope=' + scope : ''));
if (r.ok) document.title = (await r.json()).title + ' · Gregal';
