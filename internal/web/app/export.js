// export: exportar una conversa (Markdown o HTML autocontingut) i crear o
// revocar enllaços de només lectura.
//
// El menú s'obre des del ⋯ de la capçalera (conversa viva) i des del botó ⤓
// de cada fila de la llista de converses (conversa desada). La descàrrega
// passa per fetch + Blob + <a download>: així porta el token de l'API i
// funciona igual al navegador i a l'escriptori (Electron mostra el diàleg
// de desar), sense obrir finestres noves.
import { t } from './i18n.js';

const $ = id => document.getElementById(id);
const G = () => window.gregal;
const esc = s => String(s == null ? '' : s)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

// exportURL construeix la ruta de l'export: conversa desada si hi ha nom,
// la viva de la pestanya si no.
export function exportURL(name, format, redact, emails) {
  const base = name ? '/api/sessions/' + encodeURIComponent(name) + '/export' : '/api/export';
  const q = ['format=' + (format === 'html' ? 'html' : 'md')];
  if (redact) {
    q.push('redact=1');
    if (emails) q.push('emails=1');
  }
  return base + '?' + q.join('&');
}

// filenameFrom treu el nom del fitxer del Content-Disposition.
export function filenameFrom(header, fallback) {
  const h = String(header || '');
  const star = /filename\*=UTF-8''([^;]+)/i.exec(h);
  if (star) { try { return decodeURIComponent(star[1].trim()); } catch (e) {} }
  const plain = /filename="([^"]+)"/i.exec(h);
  return plain ? plain[1] : fallback;
}

// shareURL és l'enllaç absolut que es copia (mateix origen que l'app).
export function shareURL(path, origin) {
  return String(origin || '').replace(/\/+$/, '') + path;
}

const avisa = msg => { try { G().sys(msg); } catch (e) {} };

async function descarrega(name, format, redact, emails) {
  try {
    const r = await G().api(exportURL(name, format, redact, emails));
    if (!r.ok) { avisa(t('export.error') + (await r.text()).trim()); return; }
    const blob = await r.blob();
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = filenameFrom(r.headers.get('Content-Disposition'), 'gregal-conversation.' + (format === 'html' ? 'html' : 'md'));
    document.body.appendChild(a); a.click(); a.remove();
    setTimeout(() => URL.revokeObjectURL(a.href), 5000);
    avisa(t('export.done') + a.download);
  } catch (e) { avisa(t('export.error') + (e.message || e)); }
}

async function copia(text) {
  try { await navigator.clipboard.writeText(text); return true; } catch (e) { return false; }
}

async function comparteix(name, emails) {
  try {
    const r = await G().api('/api/shares', { method: 'POST', body: JSON.stringify({ name: name || '', emails }) });
    if (!r.ok) { avisa(t('share.error') + (await r.text()).trim()); return; }
    const d = await r.json();
    const url = shareURL(d.url, location.origin);
    const ok = await copia(url);
    avisa((ok ? t('share.copied') : t('share.created')) + url);
  } catch (e) { avisa(t('share.error') + (e.message || e)); }
}

let menu = null;
function tanca() {
  if (menu) { menu.remove(); menu = null; }
  document.removeEventListener('click', foraClic, true);
  document.removeEventListener('keydown', escTanca, true);
}
function foraClic(e) { if (menu && !menu.contains(e.target)) tanca(); }
function escTanca(e) { if (e.key === 'Escape' && menu) { e.stopPropagation(); tanca(); } }

// openMenu obre el menú d'exportació al costat de l'element. `name` buit =
// conversa viva.
export function openMenu(anchor, name) {
  tanca();
  menu = document.createElement('div');
  menu.className = 'export-menu';
  menu.setAttribute('role', 'menu');
  menu.innerHTML =
    '<div class="export-h">' + esc(t('export.title')) + '</div>' +
    '<button class="ghost" data-act="md">' + esc(t('export.md')) + '</button>' +
    '<button class="ghost" data-act="html">' + esc(t('export.html')) + '</button>' +
    '<label class="export-opt"><input type="checkbox" data-opt="redact" checked> ' + esc(t('export.redact')) + '</label>' +
    '<label class="export-opt"><input type="checkbox" data-opt="emails"> ' + esc(t('export.emails')) + '</label>' +
    '<div class="export-sep"></div>' +
    '<button class="ghost" data-act="share" title="' + esc(t('share.help')) + '">' + esc(t('share.create')) + '</button>';
  document.body.appendChild(menu);
  const r = anchor && anchor.getBoundingClientRect ? anchor.getBoundingClientRect() : { bottom: 60, left: 20, right: 220 };
  const w = menu.offsetWidth || 220;
  menu.style.top = Math.min(r.bottom + 6, innerHeight - (menu.offsetHeight || 200) - 8) + 'px';
  menu.style.left = Math.max(8, Math.min(r.left, innerWidth - w - 8)) + 'px';
  const opt = k => !!menu.querySelector('[data-opt="' + k + '"]')?.checked;
  menu.addEventListener('click', e => {
    const b = e.target.closest('button[data-act]');
    if (!b) return;
    e.stopPropagation();
    const act = b.dataset.act, redact = opt('redact'), emails = opt('emails');
    tanca();
    if (act === 'share') comparteix(name, true);
    else descarrega(name, act, redact, emails);
  });
  setTimeout(() => {
    document.addEventListener('click', foraClic, true);
    document.addEventListener('keydown', escTanca, true);
  }, 0);
  menu.querySelector('button')?.focus();
}

// shares: la secció de Preferències amb els enllaços vius i el botó de
// revocar cadascun.
export const shares = {
  section() {
    return '<h3 class="pf-sec">' + esc(t('share.section')) + '</h3>' +
      '<div class="pf-row"><div class="pf-lab"><b>' + esc(t('share.list')) + '</b><span>' + esc(t('share.list.help')) + '</span></div></div>' +
      '<div id="sharesList" class="shares-list">' + esc(t('share.loading')) + '</div>';
  },
  async bind(root) {
    const box = (root && root.querySelector && root.querySelector('#sharesList')) || $('sharesList');
    if (!box) return;
    let rows = [];
    try {
      const r = await G().api('/api/shares');
      if (!r.ok) throw new Error((await r.text()).trim());
      rows = (await r.json()).shares || [];
    } catch (e) { box.textContent = t('share.error') + (e.message || e); return; }
    if (!rows.length) { box.textContent = t('share.empty'); return; }
    box.innerHTML = rows.map(s => {
      const url = shareURL(s.url, location.origin);
      const when = s.created ? new Date(s.created).toLocaleString() : '';
      return '<div class="share-row" data-id="' + esc(s.id) + '">' +
        '<div class="share-t"><b>' + esc(s.title || s.session || s.id) + '</b><span>' + esc(when) + '</span></div>' +
        '<button class="pf-open" data-act="copy" data-url="' + esc(url) + '">' + esc(t('share.copy')) + '</button>' +
        '<button class="pf-open share-revoke" data-act="revoke">' + esc(t('share.revoke')) + '</button></div>';
    }).join('');
    box.querySelectorAll('button[data-act]').forEach(b => {
      b.onclick = async () => {
        if (b.dataset.act === 'copy') {
          const ok = await copia(b.dataset.url);
          b.textContent = ok ? t('share.copiedShort') : b.dataset.url;
          return;
        }
        const id = b.closest('.share-row').dataset.id;
        if (!confirm(t('share.revoke.confirm'))) return;
        try {
          const r = await G().api('/api/shares/' + encodeURIComponent(id), { method: 'DELETE' });
          if (!r.ok) throw new Error((await r.text()).trim());
        } catch (e) { avisa(t('share.error') + (e.message || e)); }
        this.bind(root);
      };
    });
  },
};

// El ⋯ de la capçalera i les files de converses (convs.js no importa res:
// hi arriba per window).
window.gregalExport = { openMenu };
const btn = $('exportBtn');
if (btn) btn.addEventListener('click', () => openMenu($('moreBtn') || btn, ''));
