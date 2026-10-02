// attach: què pots posar dins d'un missatge.
//
// El clip només obria el selector d'imatges. Ara obre un menú amb el que de
// debò vols adjuntar a un agent de codi: imatges, un fitxer, i sobretot una
// carpeta com a context — que és com li dius «treballa amb aquest tros del
// projecte» sense haver d'enumerar-li els fitxers un per un.
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
    this.box.innerHTML =
      '<button data-a="img"><b>🖼️ Imatges…</b><span>Fins a 3, per mirar-les o analitzar-les</span></button>' +
      '<button data-a="folder"><b>📁 Carpeta del projecte…</b><span>Li passa l’arbre de fitxers d’una subcarpeta</span></button>' +
      '<button data-a="extfolder"><b>🌐 Carpeta externa com a context…</b><span>Tria qualsevol carpeta del disc per donar context</span></button>' +
      '<button data-a="workspace"><b>⚡ Canvia de projecte…</b><span>Obre una altra carpeta de treball</span></button>';
    document.body.appendChild(this.box);
    this.box.querySelectorAll('button').forEach(b => {
      b.onclick = () => {
        const a = b.dataset.a;
        this.tanca();
        if (a === 'img') $('imgPick').click();
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
    try { arbre = await j('/api/tree?depth=3'); } catch (e) { G().sys('No s’ha pogut llegir l’arbre: ' + e.message); return; }
    const dirs = [];
    const walk = (ns, prof) => ns.forEach(n => {
      if (!n.dir) return;
      dirs.push({ path: n.path, prof });
      walk(n.children || [], prof + 1);
    });
    walk(arbre.entries || [], 0);
    if (!dirs.length) { G().sys('Aquest projecte no té subcarpetes.'); return; }
    this.llista('Quina carpeta?', dirs.map(d => ({
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
    const ruta = prompt('Ruta absoluta o amb ~ de la carpeta que vols passar com a context:');
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
    if (!linies.length) { G().sys('La carpeta ' + path + ' és buida.'); return; }
    // Un topall honest: si és enorme, es diu que s'ha retallat.
    let cos = linies.slice(0, 200).join('\n');
    if (linies.length > 200) cos += '\n  … i ' + (linies.length - 200) + ' més';
    const input = $('in');
    const prefix = input.value.trim() ? input.value.replace(/\s*$/, '\n\n') : '';
    input.value = prefix + 'Context — carpeta `' + path + '` (' + linies.length + ' entrades):\n```\n' + cos + '\n```\n';
    input.dispatchEvent(new Event('input'));
    input.focus();
    G().activityItem('context', path, linies.length + ' entrades', 'ok');
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
    if (!llista.length) { G().sys('Cap projecte recent. A l’escriptori pots obrir-ne un amb el diàleg natiu.'); return; }
    this.llista('Quin projecte?', llista, p => this.canviaProjecte(p));
  },

  async canviaProjecte(path) {
    try {
      const r = await G().api('/api/workspaces', { method: 'POST', body: JSON.stringify({ path }) });
      if (!r.ok) { G().sys('No s’ha pogut canviar: ' + await r.text()); return; }
      G().sys('Projecte → ' + path);
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
      box.innerHTML = '<div class="ap-head"></div><input class="ap-q" placeholder="Filtra…"><div class="ap-list"></div>';
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
        : '<div class="ap-buit">Res que hi casi.</div>';
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
