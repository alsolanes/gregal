// webs: les webs amb nom del compte (B11), a Preferències → Integracions.
// Nom, visibilitat, versió i estat de l'última comprovació; obrir,
// compartir (copiar l'enllaç), desfer i canviar la visibilitat sense tornar
// a publicar. Mateixa API que la llista «Webs» de l'app Android.
import { t } from './i18n.js';

const G = () => window.gregal;
const esc = s => String(s == null ? '' : s)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

export const VISIBILITATS = ['public', 'lan', 'private'];

// webStateKey: la clau del text de l'estat (live, draft, check_failed).
export function webStateKey(s) {
  return 'webs.state.' + (['live', 'draft', 'check_failed'].includes(s && s.state) ? s.state : 'draft');
}

async function api(path, body) {
  const opts = body === undefined ? {} : { method: 'POST', body: JSON.stringify(body) };
  const r = await G().api(path, opts);
  const txt = await r.text();
  let j = {};
  try { j = JSON.parse(txt); } catch (_) { /* text pla */ }
  if (!r.ok) throw new Error(j.error || txt.trim());
  return j;
}

async function copia(text) {
  try { await navigator.clipboard.writeText(text); return true; } catch (_) { return false; }
}

export const webs = {
  section() {
    return '<h3 class="pf-sec">' + esc(t('webs.section')) + '</h3>' +
      '<div class="pf-row"><div class="pf-lab"><b>' + esc(t('webs.list')) + '</b><span>' + esc(t('webs.list.help')) + '</span></div></div>' +
      '<div id="websList" class="shares-list">' + esc(t('share.loading')) + '</div>';
  },
  async bind(root) {
    const box = (root && root.querySelector && root.querySelector('#websList')) || document.getElementById('websList');
    if (!box) return;
    let data;
    try { data = await api('/api/webs'); } catch (e) { box.textContent = t('webs.error') + (e.message || e); return; }
    const rows = data.sites || [];
    if (!rows.length) { box.textContent = t(data.canPublish ? 'webs.empty' : 'webs.cannot'); return; }
    box.innerHTML = rows.map(s => {
      const opts = VISIBILITATS.map(v => '<option value="' + v + '"' + (v === s.visibility ? ' selected' : '') + '>' + esc(t('webs.vis.' + v)) + '</option>').join('');
      const chk = s.check && s.check.problems && s.check.problems.length ? ' — ' + s.check.problems.join('; ') : '';
      return '<div class="share-row web-row" data-name="' + esc(s.name) + '">' +
        '<div class="share-t"><b>' + esc(s.host || s.name) + '</b><span>' +
        esc(t('webs.version') + ' ' + (s.version || '–') + ' · ' + t(webStateKey(s)) + chk) + '</span></div>' +
        '<select class="pf-select web-vis" aria-label="' + esc(t('webs.visibility')) + '">' + opts + '</select>' +
        '<button class="pf-open" data-act="open">' + esc(t('webs.open')) + '</button>' +
        '<button class="pf-open" data-act="copy" data-url="' + esc(s.url) + '">' + esc(t('webs.share')) + '</button>' +
        (s.canUndo ? '<button class="pf-open" data-act="undo">' + esc(t('webs.undo')) + '</button>' : '') +
        '</div>';
    }).join('');
    box.querySelectorAll('.web-row').forEach(row => {
      const name = row.dataset.name, base = '/api/webs/' + encodeURIComponent(name);
      row.querySelector('.web-vis').onchange = async e => {
        const v = e.target.value;
        if (v === 'public' && !confirm(t('webs.public.confirm'))) { this.bind(root); return; }
        try { await api(base + '/visibility', { visibility: v }); } catch (err) { alert(t('webs.error') + err.message); }
        this.bind(root);
      };
      row.querySelectorAll('button[data-act]').forEach(b => {
        b.onclick = async () => {
          try {
            if (b.dataset.act === 'copy') {
              b.textContent = (await copia(b.dataset.url)) ? t('share.copiedShort') : b.dataset.url;
              return;
            }
            if (b.dataset.act === 'open') {
              const { url } = await api(base + '/open', {});
              window.open(url, '_blank', 'noopener');
              return;
            }
            if (b.dataset.act === 'undo') {
              await api(base + '/undo', {});
              this.bind(root);
            }
          } catch (err) { alert(t('webs.error') + err.message); }
        };
      });
    });
  },
};
