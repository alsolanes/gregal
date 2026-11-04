// El perfil de la persona, al servidor. Tema, idioma, densitat, mida de lletra…
// es desen al navegador (localStorage) i aquí també al compte del servidor
// (/api/profile): entrant des d'un altre dispositiu amb el mateix usuari, es
// recuperen. Cada compte té el seu perfil; les claus que viatgen les dicta el
// servidor, i mai el token, les sessions obertes ni res del dispositiu.
//
// Cada clau porta l'hora del seu últim canvi: dos dispositius que canvien coses
// diferents no es trepitgen, i en una mateixa clau guanya l'últim canvi. Es pot
// apagar per dispositiu (Preferències → General).
const OFF = 'gregal_profile_sync';   // '0' = aquest dispositiu no sincronitza
const META = 'gregal_profile_meta';  // {clau: hora local de l'últim canvi}
const RELOAD = 'gregal_profile_reload';

const rawGet = Storage.prototype.getItem, rawSet = Storage.prototype.setItem, rawRemove = Storage.prototype.removeItem;
const get = k => { try { return rawGet.call(localStorage, k); } catch (e) { return null; } };
const set = (k, v) => { try { rawSet.call(localStorage, k, v); } catch (e) {} };
const del = k => { try { rawRemove.call(localStorage, k); } catch (e) {} };

let keys = null;            // les claus sincronitzables (les dóna el servidor)
let applying = false;       // aplicant el perfil del servidor: no és un canvi local
let timer = 0;
const dirty = new Set();

export const profileSync = {
  enabled: () => get(OFF) !== '0',
  setEnabled(on) {
    if (on) { del(OFF); this.sync(); } else set(OFF, '0');
  },
};

const meta = () => { try { return JSON.parse(get(META) || '{}') || {}; } catch (e) { return {}; } };
const saveMeta = m => set(META, JSON.stringify(m));

// Qualsevol canvi local d'una clau del perfil el marca i el porta al servidor.
function watch() {
  Storage.prototype.setItem = function (k, v) {
    rawSet.call(this, k, v);
    if (this === localStorage) touched(k);
  };
  Storage.prototype.removeItem = function (k) {
    rawRemove.call(this, k);
    if (this === localStorage) touched(k);
  };
}

function touched(k) {
  if (applying || !profileSync.enabled()) return;
  // Abans de saber la llista (els primers instants), només claus de Gregal; després, la llista.
  if (keys ? !keys.includes(k) : !/^(gregal_|arena)/.test(k)) return;
  const m = meta(); m[k] = Date.now(); saveMeta(m);
  dirty.add(k);
  clearTimeout(timer);
  timer = setTimeout(push, 1500);
}

const api = (path, opts) => window.api(path, opts).then(async r => { if (!r.ok) throw new Error(r.status); return r.json(); });

async function push() {
  if (!keys || !dirty.size || !profileSync.enabled()) return;
  const m = meta(), prefs = {};
  for (const k of dirty) {
    if (!keys.includes(k)) continue;
    const v = get(k);
    prefs[k] = { v: v === null ? null : v, t: m[k] || Date.now() };
  }
  dirty.clear();
  if (!Object.keys(prefs).length) return;
  try { await api('/api/profile', { method: 'POST', body: JSON.stringify({ prefs }) }); }
  catch (e) { Object.keys(prefs).forEach(k => dirty.add(k)); timer = setTimeout(push, 15000); }
}

// El servidor guanya si té un canvi més nou que el d'aquest dispositiu; si el
// dispositiu en té un de més nou (o el servidor no té la clau), hi puja.
async function pull() {
  const d = await api('/api/profile');
  keys = Array.isArray(d.keys) ? d.keys : [];
  const m = meta(), remote = d.prefs || {};
  let changed = false;
  applying = true;
  try {
    for (const k of keys) {
      const r = remote[k], localT = m[k] || 0;
      if (r && r.t > localT) {
        const now = get(k);
        const same = r.v === null ? now === null : now === r.v;
        m[k] = r.t;
        if (!same) { if (r.v === null) del(k); else set(k, r.v); changed = true; }
      } else if (!r || (r.t < localT)) {
        if (get(k) !== null || r) dirty.add(k);       // el dispositiu té un valor més nou: puja
      }
    }
  } finally { applying = false; }
  saveMeta(m);
  // Un valor primer vist en aquest dispositiu (sense hora) i sense res al servidor: puja amb l'hora d'ara.
  for (const k of keys) { if (!remote[k] && get(k) !== null && !m[k]) { m[k] = Date.now(); dirty.add(k); } }
  saveMeta(m);
  if (dirty.size) await push();
  return changed;
}

// El tema, l'idioma i la densitat s'apliquen en carregar la pàgina: si el
// perfil ha canviat alguna cosa, es recarrega una sola vegada.
export async function sync() {
  if (!profileSync.enabled() || typeof window.api !== 'function') return;
  try {
    if (await pull()) {
      const last = +(sessionStorage.getItem(RELOAD) || 0);
      if (Date.now() - last > 30000) { sessionStorage.setItem(RELOAD, String(Date.now())); location.reload(); }
    }
  } catch (e) { /* sense servidor o sense sessió: es torna a provar en el següent inici */ }
}
profileSync.sync = sync;

watch();
// L'api i el login es munten durant l'arrencada: es lliga quan la pàgina és a punt.
if (document.readyState === 'complete') setTimeout(sync, 800);
else addEventListener('load', () => setTimeout(sync, 800), { once: true });
// En tornar a la pestanya (p. ex. després de canviar d'ordinador), es refresca.
document.addEventListener('visibilitychange', () => { if (!document.hidden) sync(); });
