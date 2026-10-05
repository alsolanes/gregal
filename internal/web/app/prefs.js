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
    pintaBarraTitol();
    // Si segueixes el sistema i el sistema canvia, la finestra el segueix.
    if (window.matchMedia) {
      const mq = window.matchMedia('(prefers-color-scheme: dark)');
      const seguir = () => { if (llegeix('tema') === 'sistema') { aplica('tema'); pintaBarraTitol(); } };
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
    const ara = llegeix(k);
    return '<div class="pf-row"><div class="pf-lab"><b>' + T(o.titol) + '</b>' +
      (o.ajuda ? '<span>' + T(o.ajuda) + '</span>' : '') + '</div>' +
      '<select class="pf-select" data-k="' + k + '" aria-label="' + T(o.titol) + '">' +
      o.valors.map(([v, txt]) => '<option value="' + v + '"' + (v === ara ? ' selected' : '') + '>' + T(txt) + '</option>').join('') +
      '</select></div>';
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
      '<h3 class="pf-sec">' + T('prefs.conversation') + '</h3>' +
      fila('prefs.detail', 'prefs.detail.help', seg('vista', vista,
        [['summary', 'prefs.detail.summary'], ['normal', 'prefs.density.normal'], ['verbose', 'prefs.detail.verbose']])) +
      '<h3 class="pf-sec">' + T('prefs.agent') + '</h3>' +
      fila('prefs.perms', 'prefs.perms.help', seg('perm', perm,
        [['manual', 'prefs.perms.manual'], ['accept', 'prefs.perms.accept'], ['auto', 'prefs.perms.auto']])) +
      // El revisor passava el model per sobre de CADA resposta i no hi
      // havia manera d'apagar-lo des de la finestra: només editant el
      // config a mà o des del TUI. Aquí és on el busques.
      fila('prefs.verify', 'prefs.verify.help',
        '<label class="pf-check"><input type="checkbox" id="prefsVerify"' + (this.verifyAuto() ? ' checked' : '') + '><span></span></label>') +
      '<h3 class="pf-sec">' + T('prefs.providers') + '</h3>' +
      fila('prefs.providers.row', 'prefs.providers.help',
        '<button class="pf-open" id="prefsProv">' + T('btn.open') + '</button>') +
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
      sel.onchange = () => {
        const k = sel.dataset.k, v = sel.value;
        if (OPCIONS[k]) { desa(k, v); aplica(k); if (k === 'tema') pintaBarraTitol(); }
      };
    });
    const v = $('prefsVerify');
    if (v) v.onchange = () => this.posaVerify(v.checked);
    const p = $('prefsProv');
    if (p) p.onclick = () => { this.tanca(); if (window.toggleProv) window.toggleProv(true); };
    desktopUpdates.bind(cos);
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
