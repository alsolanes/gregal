// attach: què pots posar dins d'un missatge.
//
// El clip només obria el selector d'imatges. Ara obre un menú amb el que de
// debò vols adjuntar a un agent de codi: imatges, un fitxer, i sobretot una
// carpeta com a context — que és com li dius «treballa amb aquest tros del
// projecte» sense haver d'enumerar-li els fitxers un per un.
import { t as T } from './i18n.js';
import { office, uploadDocument } from './office.js';

const $ = id => document.getElementById(id);
const G = () => window.gregal;
const esc = s => String(s == null ? '' : s)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

async function j(url) {
  const r = await G().api(url);
  if (!r.ok) throw new Error((await r.text()) || ('HTTP ' + r.status));
  return r.json();
}

export const attach = {
  box: null,
  init() {
    // Enllaços a documents de Gregal (el PDF que ha fet un vent): l'API vol
    // la clau, o sigui que es baixen per fetch i no navegant-hi.
    document.addEventListener('click', e => {
      const a = e.target.closest && e.target.closest('a[data-gregal-doc]');
      if (!a) return;
      e.preventDefault();
      this.baixaDocument(a.getAttribute('href'), a.textContent);
    });
    // Imatges que fan els vents (md.js les deixa amb data-gregal-img): també
    // volen la clau, així que es baixen per fetch i van com a blob.
    const sel = 'img[data-gregal-img]:not([src])';
    const scan = n => {
      if (n.nodeType !== 1) return;
      if (n.matches(sel)) this.carregaImatge(n); else n.querySelectorAll(sel).forEach(i => this.carregaImatge(i));
      n.querySelectorAll('input.gc-check, input.md-check').forEach(c => { if (checks.get(checkKey(c))) c.checked = true; });
    };
    // Checklists i ingredients (md.js): el que marques es recorda en aquest
    // aparell, també quan la conversa es torna a pintar.
    const checks = {
      all() { try { return JSON.parse(localStorage.getItem('gregal-checks') || '{}'); } catch { return {}; } },
      get(k) { return !!this.all()[k]; },
      set(k, on) {
        const all = this.all();
        if (on) all[k] = Date.now(); else delete all[k];
        const keys = Object.keys(all);
        if (keys.length > 600) keys.sort((a, b) => all[a] - all[b]).slice(0, keys.length - 600).forEach(x => delete all[x]);
        try { localStorage.setItem('gregal-checks', JSON.stringify(all)); } catch {}
      },
    };
    const checkKey = c => {
      const li = c.closest('li'), box = c.closest('.gregal-card, ul, ol');
      const head = box?.querySelector('.gc-title')?.textContent || box?.textContent.slice(0, 80) || '';
      let h = 0;
      for (const ch of head + '\u0001' + (li?.textContent || '')) h = (h * 31 + ch.charCodeAt(0)) | 0;
      return 'c' + (h >>> 0).toString(36);
    };
    document.addEventListener('change', e => {
      const c = e.target;
      if (c.matches && c.matches('input.gc-check, input.md-check')) checks.set(checkKey(c), c.checked);
    });
    // Recepta: − / + canvien les racions i recalculen les quantitats.
    document.addEventListener('click', e => {
      const b = e.target.closest && e.target.closest('.gregal-recipe .gc-less, .gregal-recipe .gc-more');
      if (!b) return;
      const card = b.closest('.gregal-recipe'), base = +card.dataset.serves || 0;
      if (!base) return;
      const now = Math.max(1, Math.min(99, (+card.dataset.now || base) + (b.classList.contains('gc-more') ? 1 : -1)));
      card.dataset.now = now;
      card.querySelectorAll('.gc-serves-n').forEach(n => { n.textContent = now; });
      card.querySelectorAll('.gc-qty').forEach(q => { q.textContent = globalThis.gregalMD?.scaleQty ? globalThis.gregalMD.scaleQty(q.dataset.qty, base, now) : q.dataset.qty; });
    });
    // Targetes: «Detalls» obre la resta del pla; un botó d'acció envia el seu
    // text com a resposta (el xat escolta «gregal-say»).
    document.addEventListener('click', e => {
      const t = e.target.closest && e.target.closest('[data-gc-toggle]');
      if (t) {
        const card = t.closest('.gregal-card');
        if (card) t.setAttribute('aria-expanded', String(card.classList.toggle('gc-open')));
        return;
      }
      const say = e.target.closest && e.target.closest('[data-gregal-say]');
      if (say) say.dispatchEvent(new CustomEvent('gregal-say', { bubbles: true, detail: { text: say.dataset.gregalSay } }));
    });
    new MutationObserver(ms => { for (const m of ms) m.addedNodes.forEach(scan); }).observe(document.body, { childList: true, subtree: true });
    scan(document.body);
    document.addEventListener('click', e => {
      const img = e.target.closest && e.target.closest('img[data-gregal-img][src^="blob:"]');
      if (img) window.open(img.src, '_blank', 'noopener');
    });
    const pick = $('docPick');
    if (pick) pick.onchange = async () => {
      const fs = [...pick.files];
      pick.value = '';
      for (const f of fs) await this.posaDocument(f);
    };
    // El despatx dels vents (wind-desk.js) també pot rebre documents: la
    // ruta va a l'encàrrec i el vent la llegeix amb pdf_read/office_read.
    const wbtn = $('windWorkAttach'), wpick = $('windWorkDoc');
    if (wbtn && wpick) {
      wbtn.onclick = () => wpick.click();
      wpick.onchange = async () => {
        const fs = [...wpick.files];
        wpick.value = '';
        for (const f of fs) await this.documentPerAlVent(f);
      };
    }
    const btn = $('attach');
    if (!btn) return;
    btn.onclick = e => { e.stopPropagation(); this.toggle(); };
    document.addEventListener('click', e => { if (this.box && !this.box.hidden && !this.box.contains(e.target)) this.tanca(); });
    document.addEventListener('keydown', e => { if (e.key === 'Escape' && this.box && !this.box.hidden) this.tanca(); });
  },
  crea() {
    if (this.box) return this.box;
    this.box = document.createElement('div');
    this.box.id = 'attachMenu';
    this.box.hidden = true;
    // data-i18n: el menú es crea un cop i aplica() el retradueix.
    const opcio = (a, icona, clau) => '<button data-a="' + a + '"><b>' + icona + ' <span data-i18n="' + clau + '">' + esc(T(clau)) + '</span></b>' +
      '<span data-i18n="' + clau + 'Hint">' + esc(T(clau + 'Hint')) + '</span></button>';
    this.box.innerHTML =
      opcio('img', '🖼️', 'attach.images') +
      opcio('doc', '📄', 'attach.document') +
      opcio('folder', '📁', 'attach.projectFolder') +
      opcio('extfolder', '🌐', 'attach.externalFolder') +
      opcio('workspace', '⚡', 'attach.changeProject');
    document.body.appendChild(this.box);
    this.box.querySelectorAll('button').forEach(b => {
      b.onclick = () => {
        const a = b.dataset.a;
        this.tanca();
        if (a === 'img') $('imgPick').click();
        if (a === 'doc') $('docPick')?.click();
        if (a === 'folder') this.triaCarpeta();
        if (a === 'extfolder') this.triaCarpetaExterna();
        if (a === 'workspace') this.triaProjecte();
      };
    });
    return this.box;
  },
  toggle() {
    this.crea();
    if (!this.box.hidden) { this.tanca(); return; }
    const r = $('attach').getBoundingClientRect();
    const w = 280;
    this.box.style.left = Math.max(8, Math.min(r.left, window.innerWidth - w - 8)) + 'px';
    this.box.style.width = w + 'px';
    this.box.hidden = false;
    // Col·locat després de ser visible: abans no en sabem l'alçada.
    this.box.style.top = Math.max(8, r.top - this.box.offsetHeight - 8) + 'px';
  },
  tanca() { if (this.box) this.box.hidden = true; },

  // triaCarpeta ensenya les carpetes del projecte i posa l'arbre de la
  // triada al missatge. El model ja té read/grep: amb saber què hi ha,
  // llegeix el que li calgui sense que li ho hàgim d'encolomar tot.
  async triaCarpeta() {
    let arbre;
    try { arbre = await j('/api/tree?depth=3'); } catch (e) { G().sys(T('attach.readTreeFailed') + e.message); return; }
    const dirs = [];
    const walk = (ns, prof) => ns.forEach(n => {
      if (!n.dir) return;
      dirs.push({ path: n.path, prof });
      walk(n.children || [], prof + 1);
    });
    walk(arbre.entries || [], 0);
    if (!dirs.length) { G().sys(T('attach.noSubfolders')); return; }
    this.llista(T('attach.chooseFolder'), dirs.map(d => ({
      val: d.path, txt: '　'.repeat(d.prof) + d.path.split('/').pop(), sub: d.path,
    })), p => this.posaCarpeta(p));
  },

  async triaCarpetaExterna() {
    if (window.gregalDesktop && window.gregalDesktop.chooseFolder) {
      const dir = await window.gregalDesktop.chooseFolder();
      if (!dir) return;
      await this.posaCarpeta(dir);
      return;
    }
    const ruta = prompt(T('attach.folderPrompt'));
    if (ruta && ruta.trim()) {
      await this.posaCarpeta(ruta.trim());
    }
  },

  async posaCarpeta(path) {
    let t;
    try { t = await j('/api/tree?depth=4&path=' + encodeURIComponent(path)); } catch (e) { G().sys(e.message); return; }
    const linies = [];
    const walk = (ns, sagnat) => ns.forEach(n => {
      linies.push(sagnat + n.name + (n.dir ? '/' : ''));
      if (n.dir) walk(n.children || [], sagnat + '  ');
    });
    walk(t.entries || [], '  ');
    if (!linies.length) { G().sys(T('attach.folderEmptyPrefix') + path + T('attach.folderEmptySuffix')); return; }
    // Un topall honest: si és enorme, es diu que s'ha retallat.
    let cos = linies.slice(0, 200).join('\n');
    if (linies.length > 200) cos += '\n  ' + T('attach.truncatedMore').replace('{n}', linies.length - 200);
    const input = $('in');
    const prefix = input.value.trim() ? input.value.replace(/\s*$/, '\n\n') : '';
    input.value = prefix + T('attach.contextPrefix') + path + T('attach.contextMiddle') + linies.length + T('attach.entries') + '):\n```\n' + cos + '\n```\n';
    input.dispatchEvent(new Event('input'));
    input.focus();
    G().activityItem('context', path, linies.length + T('attach.entries'), 'ok');
  },

  // posaDocument puja un PDF/Office i en posa la ruta al missatge (@ruta):
  // el servidor n'adjunta el text en enviar-lo i l'agent el pot llegir
  // sencer (pdf_read, office_read) o editar (pdf_edit, office_edit).
  async posaDocument(f) {
    const msg = (k, v) => T(k).replace(/\{(\w+)\}/g, (m, n) => v[n] == null ? m : String(v[n]));
    let d;
    try { d = await uploadDocument(f); } catch (e) {
      G().sys(msg('attach.docFailed', { name: f.name, error: e.message || e }));
      return null;
    }
    office.remember(d);
    office.renderRecent && office.renderRecent();
    const input = $('in');
    if (input) {
      const prefix = input.value.trim() ? input.value.replace(/\s*$/, ' ') : '';
      input.value = prefix + '@' + d.path + ' ';
      input.dispatchEvent(new Event('input'));
      input.focus();
    }
    G().activityItem('office', msg('attach.docAdded', { name: d.name }), d.kind.toUpperCase(), 'ok');
    return d;
  },

  imatges: new Map(),
  async carregaImatge(img) {
    const href = img.getAttribute('data-gregal-img');
    // Un src provisional: així el MutationObserver no la torna a agafar.
    img.setAttribute('src', 'data:image/gif;base64,R0lGODlhAQABAAAAACw=');
    try {
      let url = this.imatges.get(href);
      if (!url) {
        const r = await G().api(href);
        if (!r.ok) throw new Error('HTTP ' + r.status);
        url = URL.createObjectURL(await r.blob());
        this.imatges.set(href, url);
      }
      img.src = url;
    } catch { img.style.display = 'none'; }
  },

  async baixaDocument(href, label) {
    try {
      const r = await G().api(href);
      if (!r.ok) throw new Error((await r.text()) || ('HTTP ' + r.status));
      const blob = await r.blob();
      const name = new URL(href, location.href).searchParams.get('name') || String(label || 'document').trim();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url; a.download = name;
      document.body.appendChild(a); a.click(); a.remove();
      setTimeout(() => URL.revokeObjectURL(url), 30000);
    } catch (e) { G().sys(String(e.message || e)); }
  },

  async documentPerAlVent(f) {
    const msg = (k, v) => T(k).replace(/\{(\w+)\}/g, (m, n) => v[n] == null ? m : String(v[n]));
    const st = $('windWorkStatus');
    if (st) st.textContent = msg('attach.docUploading', { name: f.name });
    let d;
    try { d = await uploadDocument(f); } catch (e) {
      if (st) st.textContent = msg('attach.docFailed', { name: f.name, error: e.message || e });
      return;
    }
    office.remember(d);
    const task = $('windWorkTask');
    if (task) {
      task.value = task.value.replace(/\s*$/, '') + (task.value.trim() ? '\n' : '') + msg('winds.attachedDoc', { name: d.name, path: d.path });
      task.focus();
    }
    if (st) st.textContent = msg('attach.docAdded', { name: d.name });
  },

  // triaProjecte canvia el directori de treball de la sessió. A l'escriptori
  // amb el diàleg natiu; al navegador, amb els recents que ja coneixem.
  async triaProjecte() {
    if (window.gregalDesktop && window.gregalDesktop.chooseFolder) {
      const dir = await window.gregalDesktop.chooseFolder();
      if (!dir) return;
      await this.canviaProjecte(dir);
      return;
    }
    let w;
    try { w = await j('/api/workspaces'); } catch (e) { G().sys(e.message); return; }
    const llista = (w.workspaces || []).map(x => ({ val: x.path, txt: x.name || x.path, sub: x.path }));
    if (!llista.length) { G().sys(T('attach.projectRecentNone')); return; }
    this.llista(T('attach.chooseProject'), llista, p => this.canviaProjecte(p));
  },

  async canviaProjecte(path) {
    try {
      const r = await G().api('/api/workspaces', { method: 'POST', body: JSON.stringify({ path }) });
      if (!r.ok) { G().sys(T('attach.changeFailed') + await r.text()); return; }
      G().sys(T('attach.projectChanged') + path);
      G().refresh();
      const P = window.gregalPanels;
      if (P) { P.files.load && P.files.load(); P.changes.load && P.changes.load(); }
    } catch (e) { G().sys(String(e.message || e)); }
  },

  // llista és un selector d'una columna amb filtre, com el de models.
  llista(titol, items, tria) {
    let box = $('attachPick');
    if (!box) {
      box = document.createElement('div');
      box.id = 'attachPick';
      box.innerHTML = '<div class="ap-head"></div><input class="ap-q" data-i18n-ph="attach.filter" placeholder="' + esc(T('attach.filter')) + '"><div class="ap-list"></div>';
      document.body.appendChild(box);
      box.addEventListener('click', e => e.stopPropagation());
      document.addEventListener('click', () => { box.hidden = true; });
      document.addEventListener('keydown', e => { if (e.key === 'Escape') box.hidden = true; });
    }
    box.querySelector('.ap-head').textContent = titol;
    const q = box.querySelector('.ap-q');
    const llista = box.querySelector('.ap-list');
    const pinta = () => {
      const f = q.value.trim().toLowerCase();
      const vis = f ? items.filter(i => i.sub.toLowerCase().includes(f)) : items;
      llista.innerHTML = vis.length
        ? vis.map(i => '<button data-v="' + esc(i.val) + '" title="' + esc(i.sub) + '">' + esc(i.txt) + '</button>').join('')
        : '<div class="ap-buit">' + esc(T('attach.noMatch')) + '</div>';
      llista.querySelectorAll('button').forEach(b => {
        b.onclick = () => { box.hidden = true; tria(b.dataset.v); };
      });
    };
    q.value = '';
    q.oninput = pinta;
    pinta();
    const r = $('attach').getBoundingClientRect();
    box.style.left = Math.max(8, Math.min(r.left, window.innerWidth - 340)) + 'px';
    box.hidden = false;
    box.style.top = Math.max(8, r.top - box.offsetHeight - 8) + 'px';
    q.focus();
  },
};
