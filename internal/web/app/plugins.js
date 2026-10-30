// plugins: quines funcions opcionals (Racó, Partits, Arena…) són enceses en
// aquest servidor. Es llegeix /api/plugins en arrencar i després de cada
// canvi; la barra lateral només mostra les enceses i, si mires la vista
// d'un plugin que s'apaga, tornes al xat. Mentre no se sap (servidor antic,
// error), tot es mostra com abans.
const G = () => window.gregal;
let llista = null;
let admin = false;

function aplica() {
  if (!llista) return;
  for (const p of llista) {
    if (!p.view) continue;
    document.querySelectorAll('.side-view[data-view="' + p.view + '"]').forEach(b => { b.hidden = !p.enabled; });
    if (!p.enabled && G()?.viewActual?.() === p.view) G().setView('agent');
  }
  document.dispatchEvent(new CustomEvent('gregal:plugins', { detail: llista }));
}

export const plugins = {
  list: () => llista || [],
  loaded: () => llista !== null,
  isAdmin: () => admin,
  // Sense dades, tot val: millor ensenyar un plugin de més que amagar-ne un.
  enabled(id) {
    const p = llista && llista.find(x => x.id === id);
    return !p || !!p.enabled;
  },
  viewAllowed(view) {
    const p = llista && llista.find(x => x.view === view);
    return !p || !!p.enabled;
  },
  async load() {
    try {
      const r = await G().api('/api/plugins');
      if (!r.ok) return false;
      const d = await r.json();
      llista = Array.isArray(d.plugins) ? d.plugins : [];
      admin = !!d.admin;
      aplica();
      return true;
    } catch (e) {
      return false;
    }
  },
  async set(id, on) {
    const r = await G().api('/api/plugins', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id, enabled: !!on }),
    });
    if (!r.ok) {
      let msg = '';
      try { msg = (await r.json()).error || ''; } catch (e) { /* cos no JSON */ }
      throw new Error(msg || ('HTTP ' + r.status));
    }
    const d = await r.json();
    llista = Array.isArray(d.plugins) ? d.plugins : llista;
    aplica();
  },
  init() {
    this.load();
    // Si la primera lectura falla (encara sense sessió), es torna a provar
    // quan arriba l'estat.
    document.addEventListener('gregal:state', () => { if (!llista) this.load(); });
  },
};

window.gregalPlugins = plugins;
