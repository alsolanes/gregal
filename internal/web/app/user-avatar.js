// La persona al peu de la barra lateral: un avatar rodó amb les inicials del
// seu nom sobre un color dels vents, i el nom al costat. El nom i el color
// viuen al perfil (gregal_user_name, gregal_user_color) i viatgen amb el
// compte; sense nom, el del compte (/api/me) o «Tu» sense comptes.
// UMD: el navegador el té a window.gregalUserAvatar i els tests de Node el
// carreguen amb require().
(function (root, factory) {
  const api = factory(root);
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.gregalUserAvatar = api;
})(typeof self !== 'undefined' ? self : this, function (root) {
  'use strict';
  const NAME = 'gregal_user_name', COLOR = 'gregal_user_color';
  // Gregal, el vent que dona nom a l'app, quan encara no se n'ha triat cap.
  const DEFAULT_COLOR = '#C1B4DE';

  // Primera lletra del primer i de l'últim mot: «Maria del Mar Puig» → «MP».
  function initials(name) {
    const words = String(name || '').trim().split(/\s+/).filter(Boolean);
    if (!words.length) return '';
    const first = w => Array.from(w)[0].toLocaleUpperCase();
    return words.length === 1 ? first(words[0]) : first(words[0]) + first(words[words.length - 1]);
  }

  function displayName({ saved, account, local }, lang) {
    const own = String(saved || '').trim();
    if (own) return own;
    if (!local && account) return account;
    return lang === 'en' ? 'You' : 'Tu';
  }

  const read = key => { try { return root.localStorage.getItem(key); } catch (e) { return null; } };
  const palette = () => root.gregalWinds?.palette || [{ name: 'Gregal', color: DEFAULT_COLOR }];
  function color() {
    const saved = read(COLOR);
    return palette().some(p => p.color === saved) ? saved : DEFAULT_COLOR;
  }

  // El nom accessible del botó: el punt de connexió és dins l'avatar (amagat
  // als lectors), així que el seu avís (sense clau d'API) va aquí.
  function label(name, warning, lang) {
    return (lang === 'en' ? 'Your profile: ' : 'El teu perfil: ') + name + (warning ? ' · ' + warning : '');
  }

  // La foto del compte (/api/profile/photo) quan n'hi ha i s'ha pogut
  // carregar; sense foto, o si falla, les inicials.
  function face({ src, failed }) { return src && !failed ? 'photo' : 'initials'; }

  let me = { account: '', local: true };
  let photo = { url: '', src: '', failed: false };
  const lang = () => root.document.documentElement.lang === 'en' ? 'en' : 'ca';
  // El que es veu sense nom propi: el camp del nom el mostra de suggeriment.
  const fallbackName = () => displayName({ saved: '', ...me }, lang());
  function paint() {
    const doc = root.document, button = doc.getElementById('sideMe');
    if (!button) return;
    const name = displayName({ saved: read(NAME), ...me }, lang()), fill = color();
    const avatar = doc.getElementById('sideMeAvatar');
    avatar.style.background = fill;
    avatar.style.color = root.gregalWinds?.inkOn ? root.gregalWinds.inkOn(fill) : '#202127';
    const initialsEl = doc.getElementById('sideMeInitials'), shown = face(photo) === 'photo';
    initialsEl.textContent = initials(name);
    initialsEl.hidden = shown;
    let img = avatar.querySelector('img');
    if (shown && !img) {
      img = doc.createElement('img');
      img.alt = '';
      img.onerror = () => { photo.failed = true; paint(); };
      avatar.prepend(img);
    }
    if (img && !shown) img.remove();
    else if (img && img.getAttribute('src') !== photo.src) img.src = photo.src;
    doc.getElementById('sideMeName').textContent = name;
    button.title = label(name, '', lang());
    button.setAttribute('aria-label', label(name, doc.getElementById('statusDot')?.title, lang()));
  }

  // La foto vol la clau, així que es baixa amb api() i va com a blob.
  async function setPhoto(url) {
    url = url || '';
    if (url === photo.url) return;
    if (photo.src) root.URL.revokeObjectURL(photo.src);
    const cur = photo = { url, src: '', failed: false };
    if (url) {
      try {
        const r = await root.api(url);
        if (!r.ok) throw new Error('HTTP ' + r.status);
        cur.src = root.URL.createObjectURL(await r.blob());
      } catch (e) { cur.failed = true; }
      // Mentrestant n'ha arribat una altra.
      if (cur !== photo) { if (cur.src) root.URL.revokeObjectURL(cur.src); return; }
    }
    paint();
  }
  const hasPhoto = () => !!photo.url;

  async function load() {
    let url = '';
    try {
      const r = await root.api('/api/me');
      if (r.ok) { const d = await r.json(); me = { account: d.user || '', local: !!d.local }; url = d.photo || ''; }
    } catch (e) { /* sense servidor: es queda «Tu» fins al pròxim inici */ }
    paint();
    setPhoto(url);
  }

  if (root.document) {
    const boot = () => {
      const button = root.document.getElementById('sideMe');
      if (!button) return;
      button.onclick = async () => { const m = await import('./prefs.js'); m.prefs.obre('name'); };
      paint();
      if (typeof root.api === 'function') load();
      root.document.addEventListener('gregal:idioma', paint);
      // Un canvi des d'una altra pestanya.
      root.addEventListener('storage', e => { if (e.key === NAME || e.key === COLOR) paint(); });
    };
    if (root.document.readyState === 'loading') root.document.addEventListener('DOMContentLoaded', boot, { once: true });
    else boot();
  }

  return { initials, displayName, label, face, fallbackName, paint, setPhoto, hasPhoto, palette, color, NAME, COLOR };
});
