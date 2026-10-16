// Brandings: paquets de colors, tipus de lletra i logo (internal/brand).
//
// Un paquet és un sol fitxer (.gregal-brand.json). S'instal·la
// arrossegant-lo a la secció Brandings de Preferències: abans d'instal·lar
// res se'n mostra una targeta amb els colors, les lletres, el logo i una
// miniatura de l'app, i només s'incorpora si l'acceptes. També se'n pot
// crear un des de zero amb l'editor (selectors de color, desplegables de
// lletra i logo) i exportar-lo per passar-lo a algú altre.
//
// El branding actiu és de tot el servidor (config `brand`): mana al web, a
// l'escriptori, al TUI i als documents que genera l'agent.
import { t as T } from './i18n.js';

const G = () => window.gregal;
const esc = s => String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const CACHE = 'gregal_brand_cache';
const MAX_LOGO = 512 * 1024;

// Tipus de lletra que surten als desplegables. «Personalitzada…» deixa
// escriure'n qualsevol (ha d'estar instal·lada a l'ordinador que la mostri).
const FONTS = ['Inter', 'Segoe UI', 'Arial', 'Helvetica', 'Calibri', 'Cambria', 'Georgia', 'Verdana', 'Times New Roman',
  'Roboto', 'Open Sans', 'Lato', 'Montserrat', 'Source Sans 3', 'IBM Plex Sans', 'Nunito', 'Poppins'];
const MONO = ['JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Consolas', 'Source Code Pro', 'IBM Plex Mono', 'Courier New'];
const COLORS = [
  ['primary', 'brands.c.primary', true], ['accent', 'brands.c.accent'], ['secondary', 'brands.c.secondary'],
  ['background', 'brands.c.background'], ['surface', 'brands.c.surface'], ['text', 'brands.c.text'],
  ['muted', 'brands.c.muted'], ['success', 'brands.c.success'], ['danger', 'brands.c.danger'],
];

let state = { active: '', brands: [] };
let pending = null; // fitxer arrossegat que espera resposta: {text, preview}
let editing = null; // paquet que s'està editant (objecte del format)
let rerender = () => {};

function cached() { try { return JSON.parse(localStorage.getItem(CACHE) || 'null'); } catch (e) { return null; } }
function setCache(v) { try { v ? localStorage.setItem(CACHE, JSON.stringify(v)) : localStorage.removeItem(CACHE); } catch (e) {} }

function styleTag() {
  let el = document.getElementById('brandCSS');
  if (!el) {
    el = document.createElement('style');
    el.id = 'brandCSS';
    document.head.appendChild(el);
  }
  return el;
}

// vars treu els tokens d'un bloc :root (per pintar la miniatura d'un paquet
// que encara no està instal·lat sense tocar la pàgina).
export function vars(css) {
  const out = {};
  String(css || '').replace(/--([a-z0-9-]+)\s*:\s*([^;]+);/gi, (_, k, v) => { out[k] = v.trim(); return ''; });
  return out;
}

// mock és una miniatura de l'app amb els colors d'un paquet: barra
// lateral, una resposta, el composer i un botó.
export function mock(v, logo, name) {
  const s = (k, d) => esc(v[k] || d);
  const font = s('font', 'inherit');
  return '<div class="br-mock" style="background:' + s('bg', '#111') + ';color:' + s('text', '#ddd') + ';font-family:' + font + '">' +
    '<div class="br-mock-side" style="background:' + s('sidebar', '#0b0b0b') + ';border-color:' + s('line', 'transparent') + '">' +
    (logo ? '<img src="' + esc(logo) + '" alt="">' : '<i style="background:' + s('accent', '#58f') + '"></i>') +
    '<b style="color:' + s('ink', '#fff') + ';font-family:' + s('font-heading', font) + '">' + esc(name || '') + '</b>' +
    '<span style="background:' + s('accent-soft', 'transparent') + ';color:' + s('accent', '#58f') + '">●</span><span></span><span></span></div>' +
    '<div class="br-mock-main"><div class="br-mock-msg" style="background:' + s('surface', '#222') + '">' +
    '<b style="color:' + s('ink', '#fff') + ';font-family:' + s('font-heading', font) + '">Aa Bb Cc</b> ' +
    '<span style="color:' + s('muted', '#999') + '">' + esc(T('brands.mock.text')) + '</span> ' +
    '<code style="font-family:' + s('mono', 'monospace') + ';color:' + s('accent-hi', '#8af') + '">brand()</code></div>' +
    '<div class="br-mock-row"><span class="br-mock-in" style="background:' + s('surface', '#222') + ';border-color:' + s('line-hi', '#444') + '"></span>' +
    '<span class="br-mock-btn" style="background:' + s('accent', '#58f') + '"></span>' +
    '<span class="br-mock-dot" style="background:' + s('green', '#3c6') + '"></span><span class="br-mock-dot" style="background:' + s('amber', '#fa3') + '"></span>' +
    '<span class="br-mock-dot" style="background:' + s('red', '#e55') + '"></span></div></div></div>';
}

function swatches(list) {
  return '<div class="br-sw">' + (list || []).map(s =>
    '<span class="br-swatch" title="' + esc(s.name) + ' ' + esc(s.hex) + '"><i style="background:' + esc(s.hex) + '"></i><small>' + esc(T('brands.c.' + s.name.split(' ')[0], s.name)) + (s.name.startsWith('chart') ? ' ' + esc(s.name.split(' ')[1]) : '') + '<br>' + esc(s.hex) + '</small></span>').join('') + '</div>';
}

function fontsLine(f) {
  const parts = [];
  if (f && f.body) parts.push('<span style="font-family:' + esc(f.body) + '">' + esc(T('brands.f.body')) + ': ' + esc(f.body) + '</span>');
  if (f && f.heading) parts.push('<span style="font-family:' + esc(f.heading) + ';font-weight:700">' + esc(T('brands.f.heading')) + ': ' + esc(f.heading) + '</span>');
  if (f && f.mono) parts.push('<span style="font-family:' + esc(f.mono) + '">' + esc(T('brands.f.mono')) + ': ' + esc(f.mono) + '</span>');
  return parts.length ? '<div class="br-fonts">' + parts.join('') + '</div>' : '';
}

async function call(path, opts) {
  const r = await G().api(path, opts);
  const text = await r.text();
  let body = null;
  try { body = JSON.parse(text); } catch (e) {}
  if (!r.ok) throw new Error((body && body.error) || text.trim() || r.status);
  return body;
}

function readFile(file, asDataURL) {
  return new Promise((ok, ko) => {
    const fr = new FileReader();
    fr.onload = () => ok(fr.result);
    fr.onerror = () => ko(fr.error);
    asDataURL ? fr.readAsDataURL(file) : fr.readAsText(file);
  });
}

function download(name, text) {
  const url = URL.createObjectURL(new Blob([text], { type: 'application/json' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export const brands = {
  // Abans que arribi la llista del servidor, el darrer branding actiu ve de
  // la memòria cau: així la finestra no parpelleja amb el tema de sempre.
  applyCached() {
    const c = cached();
    if (!c || !c.theme) return;
    styleTag().textContent = c.css || '';
    document.documentElement.setAttribute('data-theme', c.theme);
    this.paintIdentity(c);
  },

  async init() {
    try { await this.load(); } catch (e) { /* sense servidor: es queda la memòria cau */ }
    document.addEventListener('dragover', e => {
      if (document.getElementById('prefsModal')?.classList.contains('on') && [...(e.dataTransfer?.types || [])].includes('Files')) e.preventDefault();
    });
  },

  async load() {
    state = await call('/api/brands');
    styleTag().textContent = state.brands.map(b => b.css).join('');
    this.apply();
  },

  active() { return state.active ? state.brands.find(b => b.id === state.active) || null : null; },
  list() { return state.brands; },

  // apply posa (o treu) el branding actiu: tema, logo, nom i favicon.
  apply() {
    const b = this.active();
    if (b) {
      document.documentElement.setAttribute('data-theme', b.theme);
      setCache({ theme: b.theme, css: b.css, logo: b.logo || '', name: b.name });
      this.paintIdentity({ logo: b.logo, name: b.name });
    } else {
      const had = cached();
      setCache(null);
      this.paintIdentity(null);
      if (had) document.dispatchEvent(new CustomEvent('gregal:brand'));
    }
    if (window.gregalDesktop) document.dispatchEvent(new CustomEvent('gregal:titlebar'));
  },

  paintIdentity(b) {
    const img = document.querySelector('.brand-mark img');
    const name = document.querySelector('.brand-name');
    const fav = document.querySelector('link[rel="icon"]');
    if (img) {
      if (!img.dataset.orig) img.dataset.orig = img.getAttribute('src');
      img.src = (b && b.logo) || img.dataset.orig;
      img.style.objectFit = b && b.logo ? 'contain' : '';
    }
    if (fav) {
      if (!fav.dataset.orig) fav.dataset.orig = fav.getAttribute('href');
      fav.href = (b && b.logo) || fav.dataset.orig;
    }
    if (name) {
      if (!name.dataset.orig) name.dataset.orig = name.innerHTML;
      name.innerHTML = b && b.name ? '<span class="brand-own">' + esc(b.name) + '</span>' : name.dataset.orig;
    }
  },

  async activate(id) {
    state = await call('/api/brands/active', { method: 'POST', body: JSON.stringify({ id: id || '' }) });
    styleTag().textContent = state.brands.map(b => b.css).join('');
    this.apply();
  },

  // ---------- Preferències ----------

  section() {
    const cards = state.brands.map(b => {
      const on = b.id === state.active;
      return '<div class="br-card' + (on ? ' on' : '') + '" data-id="' + esc(b.id) + '">' +
        '<div class="br-head">' + (b.logo ? '<img class="br-logo" src="' + esc(b.logo) + '" alt="">' : '<span class="br-logo br-nologo" style="background:' + esc(b.colors.primary) + '"></span>') +
        '<div class="br-title"><b>' + esc(b.name) + '</b><span>v' + esc(b.version) + (b.author ? ' · ' + esc(b.author) : '') + ' · ' + esc(T(b.mode === 'light' ? 'brands.mode.light' : 'brands.mode.dark')) + '</span></div>' +
        (on ? '<span class="br-badge">' + esc(T('brands.active')) + '</span>' : '') + '</div>' +
        swatches(b.swatches) +
        '<div class="br-actions">' +
        (on ? '<button class="pf-open" data-act="off">' + esc(T('brands.deactivate')) + '</button>' : '<button class="pf-open br-primary" data-act="on">' + esc(T('brands.activate')) + '</button>') +
        '<button class="pf-open" data-act="edit">' + esc(T('brands.edit')) + '</button>' +
        '<button class="pf-open" data-act="export">' + esc(T('brands.export')) + '</button>' +
        '<button class="pf-open br-danger" data-act="del">' + esc(T('brands.delete')) + '</button></div></div>';
    }).join('');
    return '<h3 class="pf-sec" id="prefsBrands">' + esc(T('brands.title')) + '</h3>' +
      '<div class="br-wrap">' +
      '<p class="br-help">' + esc(T('brands.help')) + '</p>' +
      (cards || '<p class="br-empty">' + esc(T('brands.none')) + '</p>') +
      '<div id="brPreview">' + this.previewHTML() + '</div>' +
      (pending ? '' : '<label class="br-drop" id="brDrop" tabindex="0"><input type="file" id="brFile" accept=".json,application/json" hidden>' +
        '<b>' + esc(T('brands.drop')) + '</b><span>' + esc(T('brands.drop.help')) + '</span></label>') +
      '<div id="brEditor">' + (editing ? this.editorHTML() : '<button class="pf-open" id="brNew">' + esc(T('brands.new')) + '</button>') + '</div>' +
      '</div>';
  },

  previewHTML() {
    if (!pending) return '';
    if (pending.error) {
      return '<div class="br-preview br-bad"><b>' + esc(T('brands.invalid')) + '</b><span>' + esc(pending.error) + '</span>' +
        '<div class="br-actions"><button class="pf-open" data-pv="cancel">' + esc(T('btn.close')) + '</button></div></div>';
    }
    const p = pending.preview, b = p.brand;
    const status = {
      new: T('brands.status.new'),
      update: T('brands.status.update').replace('{from}', p.installed_version).replace('{to}', b.version),
      same: T('brands.status.same'),
      older: T('brands.status.older').replace('{v}', p.installed_version),
    }[p.status] || '';
    const verb = p.status === 'update' ? 'brands.accept.update' : p.status === 'older' ? 'brands.accept.anyway' : 'brands.accept';
    return '<div class="br-preview br-' + esc(p.status) + '">' +
      '<div class="br-head">' + (b.logo ? '<img class="br-logo big" src="' + esc(b.logo) + '" alt="">' : '') +
      '<div class="br-title"><b>' + esc(b.name) + '</b><span>v' + esc(b.version) + (b.author ? ' · ' + esc(b.author) : '') + ' · ' + esc(T(b.mode === 'light' ? 'brands.mode.light' : 'brands.mode.dark')) + '</span></div></div>' +
      '<p class="br-status">' + esc(status) + '</p>' +
      swatches(b.swatches) + fontsLine(b.fonts) + mock(vars(b.css), b.logo, b.name) +
      '<label class="br-activate"><input type="checkbox" id="brActivate" checked> ' + esc(T('brands.activateNow')) + '</label>' +
      '<div class="br-actions"><button class="pf-open br-primary" data-pv="ok">' + esc(T(verb)) + '</button>' +
      '<button class="pf-open" data-pv="cancel">' + esc(T('brands.cancel')) + '</button></div></div>';
  },

  editorHTML() {
    const e = editing;
    const c = e.colors || {};
    const colorRow = ([k, label, req]) => {
      const v = c[k] || '';
      return '<label class="br-col"><input type="color" data-c="' + k + '" value="' + esc(v || '#888888') + '"' + (v ? '' : ' data-empty="1"') + '>' +
        '<span>' + esc(T(label)) + (req ? ' *' : '') + '<small>' + esc(v || T('brands.auto')) + '</small></span>' +
        (req || !v ? '' : '<button type="button" class="br-x" data-clear="' + k + '" title="' + esc(T('brands.auto')) + '">×</button>') + '</label>';
    };
    const fontSel = (k, list) => {
      const v = (e.fonts && e.fonts[k]) || '';
      const known = !v || list.includes(v);
      return '<label class="br-font"><span>' + esc(T('brands.f.' + k)) + '</span><select data-f="' + k + '">' +
        '<option value=""' + (v ? '' : ' selected') + '>' + esc(T('brands.f.default')) + '</option>' +
        list.map(f => '<option value="' + esc(f) + '" style="font-family:' + esc(f) + '"' + (f === v ? ' selected' : '') + '>' + esc(f) + '</option>').join('') +
        '<option value="__custom"' + (known ? '' : ' selected') + '>' + esc(T('brands.f.custom')) + '</option></select>' +
        '<input type="text" data-fc="' + k + '" placeholder="' + esc(T('brands.f.customHint')) + '" value="' + (known ? '' : esc(v)) + '"' + (known ? ' hidden' : '') + '></label>';
    };
    return '<div class="br-editor">' +
      '<div class="br-grid2"><label><span>' + esc(T('brands.name')) + ' *</span><input type="text" id="brName" maxlength="60" value="' + esc(e.name || '') + '"></label>' +
      '<label><span>' + esc(T('brands.version')) + '</span><input type="text" id="brVer" maxlength="32" value="' + esc(e.version || '1.0') + '"></label>' +
      '<label><span>' + esc(T('brands.author')) + '</span><input type="text" id="brAuthor" maxlength="80" value="' + esc(e.author || '') + '"></label>' +
      '<label><span>' + esc(T('brands.mode')) + '</span><select id="brMode"><option value="dark"' + (e.mode === 'light' ? '' : ' selected') + '>' + esc(T('brands.mode.dark')) + '</option>' +
      '<option value="light"' + (e.mode === 'light' ? ' selected' : '') + '>' + esc(T('brands.mode.light')) + '</option></select></label></div>' +
      '<h4>' + esc(T('brands.colors')) + '</h4><div class="br-cols">' + COLORS.map(colorRow).join('') + '</div>' +
      '<h4>' + esc(T('brands.fonts')) + '</h4><div class="br-grid3">' + fontSel('body', FONTS) + fontSel('heading', FONTS) + fontSel('mono', MONO) + '</div>' +
      '<h4>' + esc(T('brands.logo')) + '</h4><div class="br-logo-row">' +
      (e.logo ? '<img class="br-logo big" src="' + esc(e.logo) + '" alt="">' : '<span class="br-empty">' + esc(T('brands.logo.none')) + '</span>') +
      '<label class="pf-open"><input type="file" id="brLogo" accept="image/png,image/jpeg,image/webp,image/svg+xml" hidden>' + esc(T('brands.logo.pick')) + '</label>' +
      (e.logo ? '<button class="pf-open" id="brLogoDel">' + esc(T('brands.delete')) + '</button>' : '') +
      '<small>' + esc(T('brands.logo.help')) + '</small></div>' +
      '<h4>' + esc(T('brands.preview')) + '</h4><div id="brLive"></div>' +
      '<p class="br-err" id="brErr"></p>' +
      '<div class="br-actions"><button class="pf-open br-primary" id="brSave">' + esc(T('brands.save')) + '</button>' +
      '<button class="pf-open" id="brDownload">' + esc(T('brands.download')) + '</button>' +
      '<button class="pf-open" id="brCancel">' + esc(T('brands.cancel')) + '</button></div></div>';
  },

  // pack és el paquet que descriu l'editor ara mateix.
  pack() {
    const e = editing;
    const colors = {};
    Object.entries(e.colors || {}).forEach(([k, v]) => { if (v) colors[k] = v; });
    const fonts = {};
    Object.entries(e.fonts || {}).forEach(([k, v]) => { if (v) fonts[k] = v; });
    const p = { format: 'gregal-brand/1', id: e.id || '', name: e.name || '', version: e.version || '1.0', mode: e.mode || 'dark', colors, fonts };
    if (e.author) p.author = e.author;
    if (e.logo) p.logo = e.logo;
    return p;
  },

  async livePreview(root) {
    const box = root.querySelector('#brLive');
    if (!box || !editing) return;
    const err = root.querySelector('#brErr');
    try {
      // Mentre no té nom, la miniatura en fa servir un de provisional: un
      // error abans d'haver escrit res només fa nosa.
      const p = this.pack();
      if (!p.name) p.name = T('brands.preview');
      const pv = await call('/api/brands/preview', { method: 'POST', body: JSON.stringify(p) });
      box.innerHTML = mock(vars(pv.brand.css), pv.brand.logo, pv.brand.name);
      if (err) err.textContent = '';
    } catch (x) {
      if (err) err.textContent = x.message;
    }
  },

  async takeFile(file) {
    if (!file) return;
    try {
      const text = await readFile(file, false);
      const preview = await call('/api/brands/preview', { method: 'POST', body: text });
      pending = { text, preview };
    } catch (e) {
      pending = { error: e.message };
    }
    rerender();
    document.getElementById('brPreview')?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  },

  bind(root, repaint) {
    rerender = repaint || rerender;
    const wrap = root.querySelector('.br-wrap');
    if (!wrap) return;

    // Arrossegar: a la zona i a tota la finestra de Preferències.
    const modal = document.getElementById('prefsModal');
    const card = modal ? modal.querySelector('.card') || modal : wrap;
    const drop = root.querySelector('#brDrop');
    const isFiles = e => [...(e.dataTransfer?.types || [])].includes('Files');
    card.ondragover = e => { if (!isFiles(e)) return; e.preventDefault(); e.stopPropagation(); drop?.classList.add('over'); };
    card.ondragleave = e => { if (e.target === card || e.target === drop) drop?.classList.remove('over'); };
    card.ondrop = e => {
      if (!isFiles(e)) return;
      e.preventDefault();
      e.stopPropagation();
      drop?.classList.remove('over');
      this.takeFile(e.dataTransfer.files[0]);
    };
    const fileIn = root.querySelector('#brFile');
    if (fileIn) fileIn.onchange = () => this.takeFile(fileIn.files[0]);
    if (drop) drop.onkeydown = e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); fileIn?.click(); } };

    // Targeta de previsualització.
    root.querySelectorAll('[data-pv]').forEach(b => {
      b.onclick = async () => {
        if (b.dataset.pv === 'cancel') { pending = null; rerender(); return; }
        const act = root.querySelector('#brActivate')?.checked;
        b.disabled = true;
        try {
          const st = await call('/api/brands', { method: 'POST', body: pending.text });
          state = st;
          if (act) await this.activate(st.installed);
          else await this.load();
          pending = null;
        } catch (e) {
          pending = { error: e.message };
        }
        rerender();
      };
    });

    // Targetes dels paquets instal·lats.
    root.querySelectorAll('.br-card').forEach(c => {
      const id = c.dataset.id;
      c.querySelectorAll('[data-act]').forEach(b => {
        b.onclick = async () => {
          const act = b.dataset.act;
          try {
            if (act === 'on') await this.activate(id);
            if (act === 'off') await this.activate('');
            if (act === 'export') {
              const r = await G().api('/api/brands/' + encodeURIComponent(id) + '/file');
              if (!r.ok) throw new Error((await r.text()).trim());
              download(id + '.gregal-brand.json', await r.text());
              return;
            }
            if (act === 'edit') {
              const b2 = state.brands.find(x => x.id === id);
              editing = { id: b2.id, name: b2.name, version: b2.version, author: b2.author, mode: b2.mode, colors: { ...b2.colors }, fonts: { ...b2.fonts }, logo: b2.logo || '' };
              delete editing.colors.chart;
            }
            if (act === 'del') {
              if (!confirm(T('brands.confirmDelete').replace('{name}', state.brands.find(x => x.id === id)?.name || id))) return;
              state = await call('/api/brands/' + encodeURIComponent(id), { method: 'DELETE' });
              styleTag().textContent = state.brands.map(x => x.css).join('');
              this.apply();
            }
          } catch (e) {
            alert(e.message);
          }
          rerender();
          if (act === 'edit') document.getElementById('brEditor')?.scrollIntoView({ block: 'start', behavior: 'smooth' });
        };
      });
    });

    // Editor.
    const nou = root.querySelector('#brNew');
    if (nou) nou.onclick = () => { editing = { name: '', version: '1.0', mode: 'dark', colors: { primary: '#3B82F6' }, fonts: {} }; rerender(); };
    if (!editing) return;
    let timer = null;
    const changed = () => { clearTimeout(timer); timer = setTimeout(() => this.livePreview(root), 150); };
    const txt = (sel, k) => { const el = root.querySelector(sel); if (el) el.oninput = () => { editing[k] = el.value.trim(); changed(); }; };
    txt('#brName', 'name');
    txt('#brVer', 'version');
    txt('#brAuthor', 'author');
    const mode = root.querySelector('#brMode');
    if (mode) mode.onchange = () => { editing.mode = mode.value; changed(); };
    root.querySelectorAll('input[data-c]').forEach(inp => {
      inp.oninput = () => {
        editing.colors[inp.dataset.c] = inp.value.toUpperCase();
        inp.removeAttribute('data-empty');
        const small = inp.parentElement.querySelector('small');
        if (small) small.textContent = inp.value.toUpperCase();
        changed();
      };
      // En tancar el selector, la creueta per tornar-lo a automàtic.
      inp.onchange = () => { if (!inp.parentElement.querySelector('.br-x') && inp.dataset.c !== 'primary') rerender(); };
    });
    root.querySelectorAll('[data-clear]').forEach(b => {
      b.onclick = e => { e.preventDefault(); delete editing.colors[b.dataset.clear]; rerender(); };
    });
    root.querySelectorAll('select[data-f]').forEach(sel => {
      const custom = root.querySelector('input[data-fc="' + sel.dataset.f + '"]');
      sel.onchange = () => {
        if (sel.value === '__custom') { custom.hidden = false; custom.focus(); editing.fonts[sel.dataset.f] = custom.value.trim(); }
        else { custom.hidden = true; editing.fonts[sel.dataset.f] = sel.value; }
        changed();
      };
      if (custom) custom.oninput = () => { editing.fonts[sel.dataset.f] = custom.value.trim(); changed(); };
    });
    const logo = root.querySelector('#brLogo');
    if (logo) logo.onchange = async () => {
      const f = logo.files[0];
      if (!f) return;
      if (f.size > MAX_LOGO) { alert(T('brands.logo.tooBig')); return; }
      editing.logo = await readFile(f, true);
      rerender();
    };
    const logoDel = root.querySelector('#brLogoDel');
    if (logoDel) logoDel.onclick = () => { editing.logo = ''; rerender(); };
    const save = root.querySelector('#brSave');
    if (save) save.onclick = async () => {
      save.disabled = true;
      try {
        const st = await call('/api/brands', { method: 'POST', body: JSON.stringify(this.pack()) });
        state = st;
        await this.activate(st.installed);
        editing = null;
        rerender();
      } catch (e) {
        root.querySelector('#brErr').textContent = e.message;
        save.disabled = false;
      }
    };
    const dl = root.querySelector('#brDownload');
    if (dl) dl.onclick = async () => {
      try {
        const pv = await call('/api/brands/preview', { method: 'POST', body: JSON.stringify(this.pack()) });
        download(pv.brand.id + '.gregal-brand.json', JSON.stringify(this.pack(), null, 2));
      } catch (e) {
        root.querySelector('#brErr').textContent = e.message;
      }
    };
    const cancel = root.querySelector('#brCancel');
    if (cancel) cancel.onclick = () => { editing = null; rerender(); };
    this.livePreview(root);
  },
};
