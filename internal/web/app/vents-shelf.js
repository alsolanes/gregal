// Gregal Vents: el prestatge d'un vent (/api/winds/outputs), el que ha fet al
// seu xat i es pot tornar a obrir: documents vius, webs publicades, enllaços i
// fitxers. Pestanya del perfil de cada vent; la mateixa API que l'app d'Android.

import { idioma } from './i18n.js';
import { whenText } from './vents-activity.js';

const tx = (ca, en) => (idioma() === 'en' ? en : ca);
const api = (p, o) => (window.gregal?.api ? window.gregal.api(p, o) : window.api ? window.api(p, o) : fetch(p, o));
const el = (tag, cls, text) => { const e = document.createElement(tag); if (cls) e.className = cls; if (text != null) e.textContent = text; return e; };

const ICON = { doc: '📄', deck: '📽️', sheet: '📊', site: '🌐', link: '🔗', file: '📎' };
const DOC = /^\/api\/docs\/([^/?]+)\/export\?/;

/** On s'obre una peça: un document viu a la seva pàgina, la resta al seu enllaç. */
export function shelfHref(it) {
  const m = DOC.exec(it.url || '');
  return m ? '/app/doc/' + m[1] + '?scope=mine' : it.url;
}

/** Pinta el prestatge del vent opts.wind dins box. */
export async function renderShelf(box, opts = {}) {
  box.replaceChildren(el('p', 'auto-help', tx('Carregant…', 'Loading…')));
  let items;
  try {
    const r = await api('/api/winds/outputs?wind=' + encodeURIComponent(opts.wind || ''));
    if (!r.ok) throw new Error((await r.text()).trim() || r.status);
    items = (await r.json()).items || [];
  } catch (e) { box.replaceChildren(el('p', 'auto-help', '⚠ ' + (e.message || e))); return; }
  if (!items.length) {
    box.replaceChildren(el('p', 'auto-empty', tx('Aquest vent encara no ha fet res que puguis obrir. Quan et faci un document, una web o un fitxer, el trobaràs aquí.',
      "This wind hasn't made anything you can open yet. When it makes you a document, a website or a file, you'll find it here.")));
    return;
  }
  const status = el('p', 'vw-status'); status.setAttribute('role', 'status');
  const grid = el('div', 'saved-grid');
  items.forEach((it, i) => {
    const c = el('article', 'saved-card shelf-card');
    c.style.animationDelay = Math.min(i, 8) * 40 + 'ms';
    const head = el('div', 'saved-head'), a = el('a', 'saved-title', it.title || it.url);
    a.href = shelfHref(it);
    if (it.kind === 'file') a.dataset.gregalDoc = '1';
    else { a.target = '_blank'; a.rel = 'noopener'; }
    head.append(el('span', 'saved-icon', ICON[it.kind] || '•'), a);
    c.append(head, el('p', 'saved-meta', whenText(it.at)));
    // Webs i enllaços s'obren sense la clau de l'app: són els que es poden passar.
    if (it.kind === 'site' || it.kind === 'link') {
      const share = el('button', 'auto-btn subtle', tx('Comparteix', 'Share')); share.type = 'button';
      share.onclick = async () => {
        const link = new URL(it.url, location.origin).href;
        try {
          if (navigator.share) await navigator.share({ title: it.title, url: link });
          else { await navigator.clipboard.writeText(link); status.textContent = tx('Enllaç copiat', 'Link copied'); }
        } catch (e) { status.textContent = '⚠ ' + (e.message || e); }
      };
      const row = el('div', 'saved-actions'); row.append(share); c.append(row);
    }
    grid.append(c);
  });
  box.replaceChildren(grid, status);
}

window.gregalVentsShelf = { renderShelf };
