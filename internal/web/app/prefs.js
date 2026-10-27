// prefs: un sol lloc per a les preferències.
//
// Abans estaven escampades: ⚙ obria els proveïdors, ⋯ tenia la vista del
// transcript, i el mode de permís vivia a la capçalera. Res d'aparença. Ara
// hi ha un panell amb seccions —Aparença, Conversa, Agent, Proveïdors— i el
// que triïs es recorda al navegador.
// T i posaLang porten àlies perquè aquí ja hi ha una `aplica` (d'aparença)
// i la `t` es fa servir de variable de bucle en uns quants map.
import { t as T, idioma, posa as posaLang, IDIOMES } from './i18n.js';
import { desktopUpdates } from './desktop-updates.js';
import { chromeExt } from './chrome-ext.js';
import { shares } from './export.js';
import { funPrefs } from './fun-prefs.js';
import { brands } from './brands.js';

const $ = id => document.getElementById(id);
const G = () => window.gregal;

// Les preferències d'aparença viuen a l'atribut data-* de l'arrel; el CSS
// hi penja tot (tokens de color, densitat i mida del text). Els textos són
// claus de traducció, no frases: vegeu i18n.js.
const OPCIONS = {
  tema: {
    clau: 'gregal_tema', defecte: 'sistema', attr: 'data-theme',
    titol: 'prefs.theme', ajuda: 'prefs.theme.help',
    valors: [['sistema', 'prefs.theme.system'], ['dark', 'prefs.theme.dark'], ['light', 'prefs.theme.light'], ['gpt', 'prefs.theme.gpt'], ['claude', 'prefs.theme.claude'], ['opencode', 'prefs.theme.opencode'], ['mistral', 'prefs.theme.mistral'], ['gemini', 'prefs.theme.gemini'], ['grok', 'prefs.theme.grok']],
  },
  densitat: {
    clau: 'gregal_densitat', defecte: 'normal', attr: 'data-density',
    titol: 'prefs.density', ajuda: 'prefs.density.help',
    valors: [['compacta', 'prefs.density.compact'], ['normal', 'prefs.density.normal'], ['comoda', 'prefs.density.cosy']],
  },
  mida: {
    clau: 'gregal_mida', defecte: 'normal', attr: 'data-size',
    titol: 'prefs.size', ajuda: '',
    valors: [['petita', 'prefs.size.small'], ['normal', 'prefs.density.normal'], ['gran', 'prefs.size.large']],
  },
};

function llegeix(k) {
  try { return localStorage.getItem(OPCIONS[k].clau) || OPCIONS[k].defecte; } catch (e) { return OPCIONS[k].defecte; }
}
function desa(k, v) {
  try { localStorage.setItem(OPCIONS[k].clau, v); } catch (e) {}
}

// aplica posa (o treu) l'atribut a <html>. "sistema" i "normal" volen dir
// «no facis res»: el CSS base ja és aquest cas.
function aplica(k) {
  const o = OPCIONS[k];
  const v = llegeix(k);
  const arrel = document.documentElement;
  // Amb un branding actiu, el tema és el seu (és de tot el servidor).
  if (k === 'tema') {
    const b = brands.active();
    if (b) { arrel.setAttribute('data-theme', b.theme); return; }
  }
  if (v === o.defecte) {
    if (k === 'tema') {
      // Seguir el sistema: mirem la preferència del SO i l'escoltem.
      const fosc = !window.matchMedia || window.matchMedia('(prefers-color-scheme: dark)').matches;
      arrel.setAttribute('data-theme', fosc ? 'dark' : 'light');
    } else {
      arrel.removeAttribute(o.attr);
    }
    return;
  }
  arrel.setAttribute(o.attr, v);
}

// pintaBarraTitol diu a l'escriptori de quin color ha de pintar els botons
// de finestra. Els pinta el sistema, no el CSS: amb el color escrit a mà al
// main.js, en tema clar quedava un requadre fosc a dalt a la dreta enmig
// d'una barra clara. Els colors surten dels mateixos tokens que la fila de
// pestanyes, que és la que fa de barra.
function pintaBarraTitol() {
  const d = window.gregalDesktop;
  if (!d || !d.setTitleBar) return;
  const cs = getComputedStyle(document.documentElement);
  const hex = nom => {
    const v = cs.getPropertyValue(nom).trim();
    return /^#[0-9a-fA-F]{6}$/.test(v) ? v : null;
  };
  const fons = hex('--sidebar'), text = hex('--ink');
  if (fons && text) { try { d.setTitleBar(fons, text); } catch (e) {} }
}

export const prefs = {
  init() {
    Object.keys(OPCIONS).forEach(aplica);
    brands.applyCached();
    pintaBarraTitol();
    brands.init().then(pintaBarraTitol);
    // Quan es treu el branding, torna el tema que hi hagués triat.
    document.addEventListener('gregal:brand', () => { aplica('tema'); pintaBarraTitol(); });
    document.addEventListener('gregal:titlebar', pintaBarraTitol);
    // Si segueixes el sistema i el sistema canvia, la finestra el segueix.
    if (window.matchMedia) {
      const mq = window.matchMedia('(prefers-color-scheme: dark)');
      const seguir = () => { if (llegeix('tema') === 'sistema' && !brands.active()) { aplica('tema'); pintaBarraTitol(); } };
      mq.addEventListener ? mq.addEventListener('change', seguir) : mq.addListener(seguir);
    }
    const btn = $('prefsBtn');
    if (btn) btn.onclick = () => this.obre();
    const tanca = $('prefsClose');
    if (tanca) tanca.onclick = () => this.tanca();
    const modal = $('prefsModal');
    if (modal) modal.onclick = e => { if (e.target.id === 'prefsModal') this.tanca(); };
    desktopUpdates.init();
    document.addEventListener('keydown', e => {
      if (e.key === 'Escape' && modal && modal.classList.contains('on')) { e.stopPropagation(); this.tanca(); }
      if ((e.ctrlKey || e.metaKey) && e.key === ',') { e.preventDefault(); this.obre(); }
    });
  },
  obre() {
    const modal = $('prefsModal');
    if (!modal) return;
    this.pinta();
    modal.classList.add('on');
  },
  tanca() {
    const modal = $('prefsModal');
    if (modal) modal.classList.remove('on');
  },
  // segment pinta un tria-un-de-tres com els de macOS/VS Code: tot visible,
  // un clic, sense desplegables.
  segment(k) {
    const o = OPCIONS[k];
    const ara = llegeix(k);
    return '<div class="pf-row"><div class="pf-lab"><b>' + T(o.titol) + '</b>' +
      (o.ajuda ? '<span>' + T(o.ajuda) + '</span>' : '') + '</div>' +
      '<div class="pf-seg" data-k="' + k + '">' +
      o.valors.map(([v, txt]) => '<button class="' + (v === ara ? 'on' : '') + '" data-v="' + v + '">' + T(txt) + '</button>').join('') +
      '</div></div>';
  },
  select(k) {
    const o = OPCIONS[k];
    const actiu = k === 'tema' ? brands.active() : null;
    const ara = actiu ? 'brand:' + actiu.id : llegeix(k);
    // Els brandings instal·lats també són temes: surten al final de la llista.
    const extres = k === 'tema' && brands.list().length
      ? '<optgroup label="' + T('brands.title') + '">' + brands.list().map(b => {
        const v = 'brand:' + b.id;
        return '<option value="' + v + '"' + (v === ara ? ' selected' : '') + '>' + b.name.replace(/[&<>"]/g, '') + '</option>';
      }).join('') + '</optgroup>'
      : '';
    return '<div class="pf-row"><div class="pf-lab"><b>' + T(o.titol) + '</b>' +
      (o.ajuda ? '<span>' + T(o.ajuda) + '</span>' : '') + '</div>' +
      '<select class="pf-select" data-k="' + k + '" aria-label="' + T(o.titol) + '">' +
      o.valors.map(([v, txt]) => '<option value="' + v + '"' + (v === ara ? ' selected' : '') + '>' + T(txt) + '</option>').join('') +
      extres + '</select></div>';
  },
  pinta() {
    const cos = $('prefsBody');
    if (!cos) return;
    const vista = (() => { try { return localStorage.getItem('gregal_view') || 'normal'; } catch (e) { return 'normal'; } })();
    const perm = (() => { try { return localStorage.getItem('gregal_perm') || 'manual'; } catch (e) { return 'manual'; } })();
    const fila = (titol, ajuda, dins) => '<div class="pf-row"><div class="pf-lab"><b>' + T(titol) + '</b>' +
      (ajuda ? '<span>' + T(ajuda) + '</span>' : '') + '</div>' + dins + '</div>';
    const seg = (k, ara2, valors) => '<div class="pf-seg" data-k="' + k + '">' +
      valors.map(([v, clau]) => '<button class="' + (v === ara2 ? 'on' : '') + '" data-v="' + v + '">' + T(clau) + '</button>').join('') +
      '</div>';
    cos.innerHTML =
      '<h3 class="pf-sec">' + T('prefs.appearance') + '</h3>' +
      this.select('tema') + this.segment('densitat') + this.segment('mida') +
      fila('prefs.lang', 'prefs.lang.help', seg('idioma', idioma(), IDIOMES.map(([v, nom]) => [v, nom]))) +
      brands.section() +
      '<h3 class="pf-sec">' + T('prefs.conversation') + '</h3>' +
      fila('prefs.detail', 'prefs.detail.help', seg('vista', vista,
        [['summary', 'prefs.detail.summary'], ['normal', 'prefs.density.normal'], ['verbose', 'prefs.detail.verbose']])) +
      '<h3 class="pf-sec">' + T('prefs.agent') + '</h3>' +
      fila('prefs.perms', 'prefs.perms.help', seg('perm', perm,
        [['manual', 'prefs.perms.manual'], ['accept', 'prefs.perms.accept'], ['auto', 'prefs.perms.auto'], ['review', 'prefs.perms.review']])) +
      // El revisor passava el model per sobre de CADA resposta i no hi
      // havia manera d'apagar-lo des de la finestra: només editant el
      // config a mà o des del TUI. Aquí és on el busques.
      fila('prefs.verify', 'prefs.verify.help',
        '<label class="pf-check"><input type="checkbox" id="prefsVerify"' + (this.verifyAuto() ? ' checked' : '') + '><span></span></label>') +
      '<h3 class="pf-sec">' + T('prefs.skills') + '</h3>' +
      '<p class="pf-skills-help">' + T('prefs.skills.help') + '</p>' +
      '<div id="prefsSkills"><div class="pf-skills-status">' + T('prefs.skills.loading') + '</div></div>' +
      '<h3 class="pf-sec">' + T('prefs.providers') + '</h3>' +
      fila('prefs.providers.row', 'prefs.providers.help',
        '<button class="pf-open" id="prefsProv">' + T('btn.open') + '</button>') +
      funPrefs.section() +
      shares.section() +
      chromeExt.section() +
      desktopUpdates.section();

    cos.querySelectorAll('.pf-seg').forEach(seg => {
      seg.querySelectorAll('button').forEach(b => {
        b.onclick = async () => {
          const k = seg.dataset.k, v = b.dataset.v;
          seg.querySelectorAll('button').forEach(x => x.classList.remove('on'));
          b.classList.add('on');
          if (OPCIONS[k]) { desa(k, v); aplica(k); if (k === 'tema') pintaBarraTitol(); return; }
          if (k === 'vista' && window.gregalSetView) window.gregalSetView(v);
          if (k === 'perm' && window.gregalSetPerm) {
            await window.gregalSetPerm(v);
            this.pinta();
          }
          if (k === 'idioma') this.posaIdioma(v);
          return;
        };
      });
    });
    cos.querySelectorAll('.pf-select').forEach(sel => {
      sel.onchange = async () => {
        const k = sel.dataset.k, v = sel.value;
        if (!OPCIONS[k]) return;
        try {
          if (k === 'tema' && v.startsWith('brand:')) {
            await brands.activate(v.slice(6));
          } else {
            if (k === 'tema' && brands.active()) await brands.activate('');
            desa(k, v);
            aplica(k);
          }
        } catch (e) {
          alert(e.message);
        }
        if (k === 'tema') { pintaBarraTitol(); this.pinta(); }
      };
    });
    const v = $('prefsVerify');
    if (v) v.onchange = () => this.posaVerify(v.checked);
    this.carregaSkills();
    const p = $('prefsProv');
    if (p) p.onclick = () => { this.tanca(); if (window.toggleProv) window.toggleProv(true); };
    desktopUpdates.bind(cos);
    chromeExt.bind(cos, () => this.pinta());
    shares.bind(cos);
    funPrefs.bind(cos);
    // Repintar conserva el desplaçament: la secció de brandings és llarga.
    brands.bind(cos, () => {
      const card = cos.closest('.card') || cos;
      const top = card.scrollTop;
      this.pinta();
      card.scrollTop = top;
    });
  },

  async carregaSkills() {
    const box = $('prefsSkills');
    if (!box) return;
    const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
    try {
      const response = await G().api('/api/skills');
      if (!response.ok) throw new Error((await response.text()).trim());
      const data = await response.json();
      const list = Array.isArray(data.skills) ? data.skills : [];
      box.innerHTML = list.map(skill => {
        const key = 'prefs.skill.' + skill.name;
        return '<div class="pf-row"><div class="pf-lab"><b>' + esc(T(key + '.title', skill.title || skill.name)) + '</b>' +
          '<span>' + esc(T(key + '.help', skill.description || '')) + '</span></div>' +
          '<label class="pf-check"><input type="checkbox" data-skill="' + esc(skill.name) + '" aria-label="' + esc(T(key + '.title', skill.title || skill.name)) + '"' + (skill.enabled ? ' checked' : '') + '><span></span></label></div>';
      }).join('');
      box.querySelectorAll('input[data-skill]').forEach(input => {
        input.onchange = async () => {
          const controls = [...box.querySelectorAll('input[data-skill]')];
          controls.forEach(control => { control.disabled = true; });
          try {
            const saved = await G().api('/api/skills', {
              method: 'PUT',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ enabled: controls.filter(control => control.checked).map(control => control.dataset.skill) }),
            });
            if (!saved.ok) throw new Error((await saved.text()).trim());
          } catch (error) {
            input.checked = !input.checked;
            const status = document.createElement('div');
            status.className = 'pf-skills-status error';
            status.textContent = T('prefs.skills.error') + ' ' + error.message;
            box.appendChild(status);
          } finally {
            controls.forEach(control => { control.disabled = false; });
          }
        };
      });
    } catch (error) {
      box.innerHTML = '<div class="pf-skills-status error">' + esc(T('prefs.skills.error')) + ' ' + esc(error.message) + '</div>';
    }
  },

  // L'idioma va al servidor abans que a la pantalla: si el servidor no el
  // pot desar, no es canvia res. Val més seguir en català que tenir la
  // finestra en anglès i l'agent responent en català.
  async posaIdioma(codi) {
    try {
      await posaLang(codi, G().api);
      await G().refresh();
      this.pinta();
    } catch (e) {
      alert('no s\'ha pogut canviar l\'idioma: ' + e.message);
      this.pinta();
    }
  },

  // El revisor té cinc modes, però la pregunta que et fas és una: revisa
  // sol o no. «strict» no es toca des d'aquí —és un bloqueig deliberat i
  // apagar-lo per error seria pitjor que haver de venir a buscar-lo.
  verifyAuto() {
    const m = (G().state && G().state.verify) || 'manual';
    return m === 'auto' || m === 'both' || m === 'strict';
  },
  async posaVerify(on) {
    const ara = (G().state && G().state.verify) || 'manual';
    if (ara === 'strict' && on) return;
    // Apagat vol dir «manual», no «off»: el que molesta és que salti sol,
    // no poder-lo demanar. Amb «off» perdries el botó de revisar.
    const mode = on ? 'auto' : 'manual';
    try {
      const r = await G().api('/api/verify', { method: 'POST', body: JSON.stringify({ mode }) });
      if (!r.ok) throw new Error((await r.text()).trim());
      await G().refresh();
      this.pinta();
    } catch (e) {
      alert('no s\'ha pogut canviar el revisor: ' + e.message);
      this.pinta();
    }
  },
};
