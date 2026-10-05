// convs: la llista de converses de l'esquerra, com la de ChatGPT o Claude.
//
// Abans era una filera de botons amb el nom del fitxer («sessio-20260915…»)
// i només hi apareixia el que s'havia desat clicant «Nova sessió». Ara el
// servidor desa cada conversa sola en acabar el torn i li posa títol, i
// aquí es veuen agrupades per data, es poden cercar, obrir i esborrar.
const $ = id => document.getElementById(id);
const G = () => window.gregal;
const esc = s => String(s == null ? '' : s)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

const gregalT = k => (typeof window.gregalT === 'function' ? window.gregalT(k) : '');

// grup diu a quin bloc va una conversa segons quan es va tocar per últim
// cop. Els mateixos trams que fan servir les altres apps: el que importa és
// trobar de seguida la d'aquest matí.
function grup(at) {
  const d = new Date(at);
  if (isNaN(d)) return { ordre: 9, nom: gregalT('conv.group.older') || 'Abans' };
  const avui = new Date(); avui.setHours(0, 0, 0, 0);
  const dies = Math.floor((avui - new Date(d).setHours(0, 0, 0, 0)) / 86400000);
  if (dies <= 0) return { ordre: 0, nom: gregalT('conv.group.today') || 'Avui' };
  if (dies === 1) return { ordre: 1, nom: gregalT('conv.group.yesterday') || 'Ahir' };
  if (dies < 7) return { ordre: 2, nom: gregalT('conv.group.thisWeek') || 'Aquesta setmana' };
  if (dies < 30) return { ordre: 3, nom: gregalT('conv.group.thisMonth') || 'Aquest mes' };
  return { ordre: 4, nom: gregalT('conv.group.older') || 'Abans' };
}

// xipProjecte diu de quin projecte és la conversa quan no és el d'ara:
// la mateixa llista barreja feina de repositoris diferents i, sense això,
// obrir-ne una canviava el workspace sense que se sabés cap a on.
function xipProjecte(c) {
  const ws = c.workspace || '';
  if (!ws) return '';
  const nom = ws.replace(/[\\/]+$/, '').split(/[\\/]/).pop() || ws;
  const actual = (window.gregalCwd || '').replace(/[\\/]+$/, '');
  if (actual && actual.toLowerCase() === ws.replace(/[\\/]+$/, '').toLowerCase()) return '';
  return '<span class="conv-p" title="' + esc(ws) + '">' + esc(nom) + '</span>';
}

function titolVisible(c) {
  const title = String(c.title || c.name || '');
  if (!/^sessio-\d{8}/i.test(title)) return title;
  const d = new Date(c.at);
  if (isNaN(d)) return 'Sessió anterior';
  return 'Sessió · ' + new Intl.DateTimeFormat(undefined, {
    day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit'
  }).format(d);
}

// Cerca local als camps que ja proporciona /api/sessions, inclosa la ruta
// completa per distingir projectes amb noms de carpeta iguals.
export function matchesConversation(c, query) {
  const q = String(query == null ? '' : query).trim().toLowerCase();
  if (!q) return true;
  if (!c || typeof c !== 'object') return false;
  return [c.title, c.name, c.project, c.workspace]
    .some(value => typeof value === 'string' && value.toLowerCase().includes(q));
}
export const convs = {
  items: [],
  expandedProjects: new Set(),
  filtre: '',
  loading: false,
  loadError: false,
  init() {
    const cerca = $('convSearch');
    if (cerca) {
      cerca.addEventListener('input', () => { this.filtre = cerca.value.trim().toLowerCase(); this.draw(); });
      cerca.addEventListener('keydown', e => {
        if (e.key === 'Escape') { cerca.value = ''; this.filtre = ''; this.draw(); cerca.blur(); }
      });
    }
    this.load();
  },
  async load() {
    this.loading = true;
    this.loadError = false;
    this.draw();
    try {
      const r = await G().api('/api/sessions');
      if (r.ok) {
        this.items = (await r.json()) || [];
      } else {
        this.items = [];
        this.loadError = true;
      }
    } catch (e) {
      this.items = [];
      this.loadError = true;
    } finally {
      this.loading = false;
      this.draw();
    }
  },
  draw() {
    const box = $('convs');
    if (!box) return;
    if (this.loading && !this.items.length) {
      box.innerHTML = '<div class="conv-empty">' + esc(gregalT('conv.loading') || 'Carregant converses…') + '</div>';
      return;
    }
    if (this.loadError && !this.items.length) {
      box.innerHTML = '<div class="conv-empty">' + esc(gregalT('conv.error.load') || 'No s’han pogut carregar les converses.') + '</div>';
      return;
    }
    const q = this.filtre;
    const llista = q ? this.items.filter(c => matchesConversation(c, q)) : this.items;
    if (!llista.length) {
      box.innerHTML = '<div class="conv-empty">' +
        esc(q ? (gregalT('conv.empty.search') || 'Cap conversa amb aquest text.') : (gregalT('conv.empty.none') || 'Encara no hi ha converses. La d’ara es desa sola.')) + '</div>';
      return;
    }
    // La jerarquia segueix el shell del desktop: fixats primer, després
    // projectes i finalment converses sense projecte agrupades per data.
    const pins = llista.filter(c => c.pinned);
    const projectes = new Map();
    const recents = [];
    llista.filter(c => !c.pinned).forEach(c => {
      const ws = c.workspace || '';
      if (!ws) { recents.push(c); return; }
      // La ruta completa és la clau: dos repositoris poden acabar amb el
      // mateix nom de carpeta (C:\\a\\app i C:\\b\\app).
      if (!projectes.has(ws)) projectes.set(ws, []);
      projectes.get(ws).push(c);
    });
    const msgsWord = gregalT('conv.messages') || 'missatges';
    const delTitle = gregalT('conv.delete.title') || 'Esborra aquesta conversa';
    const delAria = gregalT('conv.delete.aria') || 'Esborra';
    const fila = c =>
      '<div class="conv' + (c.current ? ' current' : '') + '" data-name="' + esc(c.name) + '" title="' +
      esc(titolVisible(c) + ' · ' + c.msgs + ' ' + msgsWord + (c.when ? ' · ' + c.when : '')) + '">' +
      '<span class="conv-t">' + esc(titolVisible(c)) + '</span>' + xipProjecte(c) +
      '<button class="conv-pin' + (c.pinned ? ' pinned' : '') + '" title="' + (c.pinned ? 'Desfixa' : 'Fixa') + '" aria-label="' + (c.pinned ? 'Desfixa' : 'Fixa') + '">★</button>' +
      '<button class="conv-x" title="' + esc(delTitle) + '" aria-label="' + esc(delAria) + '">×</button></div>';
    let html = '';
    if (pins.length) html += '<div class="conv-grup">Fixats</div>' + pins.map(fila).join('');
    [...projectes.entries()].sort((a,b) => a[0].localeCompare(b[0])).forEach(([ws, files]) => {
      const nom = ws.replace(/[\\/]+$/, '').split(/[\\/]/).pop() || ws;
      const oberta = this.expandedProjects.has(ws);
      html += '<div class="conv-project" title="' + esc(ws) + '">' + esc(nom) + '</div>' + files.slice(0, oberta ? files.length : 5).map(fila).join('');
      if (!oberta && files.length > 5) html += '<button class="conv-more" data-project-more="' + esc(ws) + '">Mostra’n més</button>';
    });
    if (recents.length) {
      const grups = new Map();
      recents.forEach(c => { const g = grup(c.at); if (!grups.has(g.nom)) grups.set(g.nom, []); grups.get(g.nom).push(c); });
      for (const [nom, files] of grups) html += '<div class="conv-grup">' + esc(nom) + '</div>' + files.map(fila).join('');
    }
    box.innerHTML = html;
    box.querySelectorAll('.conv-more').forEach(el => {
      el.onclick = e => { e.stopPropagation(); this.expandedProjects.add(el.dataset.projectMore); this.draw(); };
    });
    box.querySelectorAll('.conv').forEach(el => {
      el.onclick = e => {
        if (e.target.classList.contains('conv-x')) { this.esborra(el.dataset.name); return; }
        if (e.target.classList.contains('conv-pin')) { this.fixa(el.dataset.name, !e.target.classList.contains('pinned')); return; }
        this.obre(el.dataset.name);
      };
    });
  },
  async fixa(name, pinned) {
    try {
      const r = await G().api('/api/sessions/pin', { method: 'POST', body: JSON.stringify({ name, pinned }) });
      if (!r.ok) throw new Error(await r.text());
      await this.load();
    } catch (e) { G().sys('No s’ha pogut actualitzar el fixat: ' + (e.message || e)); }
  },
  async obre(name) {
    try {
      // Reprendre sobre una sessió ocupada canviaria el context del worker.
      if (G().isBusy()) await window.gregalPanels.sessions.open(G().state.cwd);
      const r = await G().api('/api/resume', { method: 'POST', body: JSON.stringify({ name }) });
      if (!r.ok) { G().sys((gregalT('conv.open.error') || 'No s’ha pogut obrir: ') + await r.text()); return; }
      const d = await r.json();
      const main = $('main');
      if (main) main.innerHTML = '';
      (d.transcript || []).forEach(m => {
        if (m.role === 'user') G().add('u', '', G().md(m.content));
        else if (m.role === 'assistant' && (m.content || '').trim()) G().add('a', '', G().md(m.content));
      });
      const msgsWord = gregalT('conv.messages') || 'missatges';
      G().sys((d.title || name) + ' · ' + d.msgs + ' ' + msgsWord);
      G().activityItem('conversa', d.title || name, d.msgs + ' ' + msgsWord, 'ok');
      await this.load();
      G().refresh();
    } catch (e) { G().sys(String(e.message || e)); }
  },
  async esborra(name) {
    const c = this.items.find(x => x.name === name);
    const title = c ? (c.title || c.name) : name;
    const confirmTpl = gregalT('conv.delete.confirm') || 'Esborrar «{title}»? No es pot desfer.';
    if (!confirm(confirmTpl.replace('{title}', title))) return;
    try {
      const r = await G().api('/api/sessions?name=' + encodeURIComponent(name), { method: 'DELETE' });
      if (!r.ok) { G().sys((gregalT('conv.delete.error') || 'No s’ha pogut esborrar: ') + await r.text()); return; }
    } catch (e) { G().sys(String(e.message || e)); return; }
    await this.load();
  },
};
