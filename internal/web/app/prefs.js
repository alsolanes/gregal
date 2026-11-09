// prefs: un sol lloc per a les preferències.
//
// Abans estaven escampades: ⚙ obria els proveïdors, ⋯ tenia la vista del
// transcript, i el mode de permís vivia a la capçalera. Res d'aparença. Ara
// hi ha un panell amb grups —General, Aparença, Models i agent, Integracions,
// Plugins, Extres i Avançat— i el que triïs es recorda al navegador.
// T i posaLang porten àlies perquè aquí ja hi ha una `aplica` (d'aparença)
// i la `t` es fa servir de variable de bucle en uns quants map.
import { t as T, idioma, posa as posaLang, IDIOMES } from './i18n.js';
import { desktopUpdates } from './desktop-updates.js';
import { chromeExt } from './chrome-ext.js';
import { wordAddin } from './word-addin.js';
import { shares } from './export.js';
import { funPrefs } from './fun-prefs.js';
import { brands } from './brands.js';
import { plugins } from './plugins.js';
import { voicePrefs } from './voice-prefs.js';
import { collective } from './collective.js';
import { profileSync } from './profile-sync.js';
import { onboarding } from './onboarding.js';

const $ = id => document.getElementById(id);
const G = () => window.gregal;
// Textos nous d'aquest panell (com els de les connexions i els comptes, que
// ja vivien aquí): català i anglès, sense passar per i18n.js.
const L = (ca, en) => (idioma() === 'en' ? en : ca);
const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
const icon = n => (window.gregalIcon && window.gregalIcon(n)) || '';
const check = (id, on, label, extra = '') => '<label class="pf-check"><input type="checkbox" id="' + id + '"' + (on ? ' checked' : '') +
  ' aria-label="' + esc(label) + '"' + extra + '><span></span></label>';
// Element amb classe i text (les seccions que es pinten des de dades).
const el = (tag, cls, txt) => { const e = document.createElement(tag); if (cls) e.className = cls; if (txt != null) e.textContent = txt; return e; };
const filaEl = (titol, ajuda, ...dins) => {
  const r = el('div', 'pf-row'), lab = el('div', 'pf-lab');
  lab.append(el('b', '', titol));
  if (ajuda) lab.append(typeof ajuda === 'string' ? el('span', '', ajuda) : ajuda);
  r.append(lab, ...dins);
  return r;
};
const boto = (label, fn, cls = '') => { const b = el('button', 'pf-open' + (cls ? ' ' + cls : ''), label); b.type = 'button'; b.onclick = fn; return b; };
const entrada = (ph, type = 'text', val = '') => {
  const i = el('input', 'pf-input'); i.type = type; i.placeholder = ph; i.value = val; i.setAttribute('aria-label', ph); i.autocomplete = 'off';
  return i;
};
const estat = () => { const m = el('p', 'pf-skills-status'); m.setAttribute('role', 'status'); return m; };
const diu = (m, text, dolent) => { m.textContent = text || ''; m.classList.toggle('error', !!dolent); };

// Títol i descripció de cada grup. Els que han canviat de contingut porten
// el text aquí; la resta, el d'i18n.js.
const GRUPS = {
  general: () => [T('prefs.group.general'), L("L'idioma, quant detall mostra la conversa, el dictat per veu i si les preferències viatgen amb el teu compte.",
    'Language, how much detail the conversation shows, voice dictation and whether your preferences travel with your account.')],
  appearance: () => [T('prefs.group.appearance'), L('Tema, densitat, mida del text, com es diuen els ajudants i els brandings del servidor.',
    "Theme, density, text size, what the helpers are called and the server's brandings.")],
  agent: () => [T('prefs.group.agent'), T('prefs.group.agent.desc')],
  integrations: () => [T('prefs.group.integrations'), L('Gregal amb les teves eines: el calendari i el correu, Chrome, Word i els enllaços que has compartit.',
    'Gregal with your tools: calendar and email, Chrome, Word and the links you have shared.')],
  plugins: () => [T('prefs.group.plugins'), L("Funcions opcionals d'aquest servidor i les veus que narren els jocs. Un plugin apagat desapareix del menú i de l'app del mòbil, i deixa de treballar en segon pla.",
    'Optional features of this server and the voices that narrate the games. A disabled plugin disappears from the menu and the phone app, and stops working in the background.')],
  extras: () => [T('prefs.group.extras'), L('Les coses divertides: el penell, la mascota i el to de les retrospectives.',
    'The fun bits: the weathervane, the mascot and the tone of retrospectives.')],
  advanced: () => [T('prefs.group.advanced'), L("Dreceres de teclat, funcions experimentals, actualitzacions i, si administres el servidor, els comptes i els poders de servidor.",
    "Keyboard shortcuts, experimental features, updates and, if you run the server, accounts and server powers.")],
};
// Destins de prefs.obre() que no són un grup sencer.
const ANCORES = { calendar: 'prefsConn', voice: 'prefsVoiceSec', models: 'prefsModelRow', accounts: 'prefsAccounts', powers: 'prefsPowers' };

// Icones dels plugins (el servidor en dona un emoji; aquí, el joc de l'app).
const PLUGIN_ICONS = { corner: 'sparkles', games: 'ball', arena: 'trophy' };

// Les preferències d'aparença viuen a l'atribut data-* de l'arrel; el CSS
// hi penja tot (tokens de color, densitat i mida del text). Els textos són
// claus de traducció, no frases: vegeu i18n.js.
const OPCIONS = {
  tema: {
    clau: 'gregal_tema', defecte: 'sistema', attr: 'data-theme',
    titol: 'prefs.theme', ajuda: 'prefs.theme.help',
    valors: [['sistema', 'prefs.theme.system'], ['dark', 'prefs.theme.dark'], ['calma', 'prefs.theme.calma'], ['light', 'prefs.theme.light'], ['gpt', 'prefs.theme.gpt'], ['claude', 'prefs.theme.claude'], ['opencode', 'prefs.theme.opencode'], ['mistral', 'prefs.theme.mistral'], ['gemini', 'prefs.theme.gemini'], ['grok', 'prefs.theme.grok']],
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

// Dreceres de teclat: la taula del diàleg es pinta d'aquí perquè digui la
// veritat (Ctrl+K ja obre la paleta, i faltaven Ctrl+, i Ctrl+Alt+N). Les
// de Codi només surten a Codi.
const mac = /Mac|iPhone|iPad/.test(navigator.platform || '');
const tecla = k => (mac ? k.replace(/Ctrl\+/g, '⌘').replace(/Alt\+/g, '⌥') : k);
function dreceres() {
  const arreu = [
    ['Ctrl+K', L("Paleta d'ordres: accions, converses i fitxers", 'Command palette: actions, conversations and files')],
    ['Ctrl+,', L('Preferències', 'Preferences')],
    ['Ctrl+Alt+N', L('Nota nova, des de qualsevol lloc', 'New note, from anywhere')],
    ['Esc', L('Tanca el diàleg o atura el torn en curs', 'Close the dialog or stop the running turn')],
  ];
  const codi = [
    ['Enter', T('keys.enter')], ['↑ / ↓', T('keys.hist')], ['Tab', T('keys.modeCycle')],
    ['Ctrl+N', L('Conversa nova', 'New conversation')], ['Ctrl+T', T('keys.newSession')], ['Ctrl+F', L('Cerca a les converses', 'Search conversations')],
    ['Ctrl+D', T('keys.changes')], ['Ctrl+E', T('keys.filesView')], ['Ctrl+G', T('keys.graphsView')], ['@ / /', T('keys.filesCommands')],
  ];
  const files = rows => rows.map(([k, d]) => '<tr><td><kbd>' + esc(tecla(k)) + '</kbd></td><td>' + esc(d) + '</td></tr>').join('');
  return '<tbody>' + '<tr class="keys-sec"><th colspan="2">' + esc(L('Arreu', 'Everywhere')) + '</th></tr>' + files(arreu) +
    (window.gregalProduct === 'vents' ? '' : '<tr class="keys-sec"><th colspan="2">' + esc(L('A Codi', 'In Codi')) + '</th></tr>' + files(codi)) + '</tbody>';
}
function pintaDreceres() {
  const t = document.querySelector('#keysModal table');
  if (t) { t.classList.add('keys-table'); t.innerHTML = dreceres(); }
}

// focusables: el que es pot tabular dins d'un contenidor visible.
const focusables = box => [...box.querySelectorAll('a[href], button:not([disabled]), input:not([disabled]):not([type=hidden]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')]
  .filter(x => x.offsetParent !== null && !x.closest('[hidden]'));
// atrapa: el Tab no surt del diàleg obert.
function atrapa(e, box) {
  const f = focusables(box);
  if (!f.length) return;
  const primer = f[0], darrer = f[f.length - 1];
  if (e.shiftKey && (document.activeElement === primer || !box.contains(document.activeElement))) { e.preventDefault(); darrer.focus(); }
  else if (!e.shiftKey && (document.activeElement === darrer || !box.contains(document.activeElement))) { e.preventDefault(); primer.focus(); }
}

export const prefs = {
  tornaFocus: null,

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
    if (tanca) {
      tanca.onclick = () => this.tanca();
      // La creu del joc d'icones, no el glif «×» de la lletra.
      if (icon('close')) tanca.innerHTML = icon('close');
    }
    const modal = $('prefsModal');
    if (modal) modal.onclick = e => { if (e.target.id === 'prefsModal') this.tanca(); };
    desktopUpdates.init();
    plugins.init();
    const cerca = $('prefsSearch');
    if (cerca) cerca.oninput = () => this.filtra();
    // Quan canvien els plugins (des d'aquí o d'una altra pestanya), la
    // secció es repinta si és oberta.
    document.addEventListener('gregal:plugins', () => {
      const box = $('prefsPlugins');
      if (box && modal && modal.classList.contains('on')) { box.innerHTML = this.pluginsHTML(); this.lligaPlugins(); this.filtra(); }
    });
    // Un model nou (des del primer inici o d'una altra finestra): la fila es posa al dia.
    document.addEventListener('gregal:model-ready', () => { if (modal && modal.classList.contains('on')) this.carregaModel(); });
    pintaDreceres();
    document.addEventListener('gregal:idioma', pintaDreceres);
    const keys = $('keysModal');
    // En captura i a la finestra: l'Esc que tanca un diàleg no ha d'arribar
    // a l'index, que amb un torn en marxa l'aturaria.
    window.addEventListener('keydown', e => {
      const obert = modal && modal.classList.contains('on');
      if (obert && e.key === 'Escape') {
        e.preventDefault(); e.stopPropagation();
        // Primer buida la cerca; després tanca.
        if (document.activeElement === cerca && cerca.value) { cerca.value = ''; this.filtra(); return; }
        this.tanca();
        return;
      }
      if (obert && e.key === 'Tab') { atrapa(e, modal.querySelector('.card')); return; }
      if (keys && keys.classList.contains('on')) {
        if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); this.tancaDreceres(); return; }
        if (e.key === 'Tab') { atrapa(e, keys.querySelector('.card')); return; }
      }
      if ((e.ctrlKey || e.metaKey) && !e.altKey && e.key === ',') { e.preventDefault(); e.stopPropagation(); obert ? this.tanca() : this.obre(); }
    }, true);
    // El diàleg de dreceres també s'obre des de l'index (botó ?): el focus hi entra.
    if (keys) new MutationObserver(() => {
      if (!keys.classList.contains('on')) { this.tornaDreceres = null; return; }
      if (!keys.contains(document.activeElement)) {
        this.tornaDreceres = this.tornaDreceres || document.activeElement;
        const b = $('keysClose'); if (b) b.focus();
      }
    }).observe(keys, { attributes: true, attributeFilter: ['class'] });
    onboarding.init();
  },

  // obre('plugins') hi entra directament al grup que demanis; obre('calendar')
  // (i les altres d'ANCORES), a la secció.
  obre(grup) {
    const modal = $('prefsModal');
    if (!modal) return;
    if (!modal.classList.contains('on')) this.tornaFocus = document.activeElement;
    const cerca = $('prefsSearch');
    if (cerca) cerca.value = '';
    this.pinta();
    modal.classList.add('on');
    this.ves(grup);
    // Les posicions només existeixen amb el modal visible.
    this.marcaNav();
    if (cerca && !(window.matchMedia && window.matchMedia('(pointer:coarse)').matches)) cerca.focus();
  },
  ves(grup) {
    const main = $('prefsMain');
    if (!main || typeof grup !== 'string') return;
    const g = $(ANCORES[grup] || ('prefsG-' + grup));
    if (g) main.scrollTop = g.getBoundingClientRect().top - main.getBoundingClientRect().top + main.scrollTop - 8;
  },
  tanca() {
    const modal = $('prefsModal');
    if (!modal || !modal.classList.contains('on')) return;
    modal.classList.remove('on');
    const f = this.tornaFocus;
    this.tornaFocus = null;
    if (f && f.isConnected && f.focus) f.focus();
  },
  obreDreceres() {
    const m = $('keysModal');
    if (!m) return;
    pintaDreceres();
    this.tornaDreceres = document.activeElement;
    m.classList.add('on');
    const b = $('keysClose'); if (b) b.focus();
  },
  tancaDreceres() {
    const m = $('keysModal');
    if (m) m.classList.remove('on');
    const f = this.tornaDreceres;
    this.tornaDreceres = null;
    if (f && f.isConnected && f.focus) f.focus();
  },
  // segment pinta un tria-un-de-tres com els de macOS/VS Code: tot visible,
  // un clic, sense desplegables.
  segment(k) {
    const o = OPCIONS[k];
    const ara = llegeix(k);
    return '<div class="pf-row"><div class="pf-lab"><b>' + T(o.titol) + '</b>' +
      (o.ajuda ? '<span>' + T(o.ajuda) + '</span>' : '') + '</div>' +
      '<div class="pf-seg" data-k="' + k + '" role="group" aria-label="' + esc(T(o.titol)) + '">' +
      o.valors.map(([v, txt]) => '<button type="button" class="' + (v === ara ? 'on' : '') + '" aria-pressed="' + (v === ara) + '" data-v="' + v + '">' + T(txt) + '</button>').join('') +
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
  // pinta: Preferències en grups, cadascun amb títol i una línia que diu què
  // hi trobaràs. A l'esquerra, l'índex dels grups; a dalt, una cerca que
  // filtra files. Les seccions que pinten altres mòduls (marques, extres,
  // extensions…) hi entren tal qual: el seu h3 queda de subtítol.
  pinta() {
    const cos = $('prefsBody');
    if (!cos) return;
    const vista = (() => { try { return localStorage.getItem('gregal_view') || 'normal'; } catch (e) { return 'normal'; } })();
    // El permís d'Actua viu al servidor (/api/access), igual que al composer.
    const perm = (G().state && G().state.access) || 'edits';
    const fila = (titol, ajuda, dins) => '<div class="pf-row"><div class="pf-lab"><b>' + T(titol) + '</b>' +
      (ajuda ? '<span>' + T(ajuda) + '</span>' : '') + '</div>' + dins + '</div>';
    const seg = (k, ara2, valors, label) => '<div class="pf-seg" data-k="' + k + '" role="group" aria-label="' + esc(T(label)) + '">' +
      valors.map(([v, clau]) => '<button type="button" class="' + (v === ara2 ? 'on' : '') + '" aria-pressed="' + (v === ara2) + '" data-v="' + v + '">' + T(clau) + '</button>').join('') +
      '</div>';
    const grups = [
      ['general', 'gear',
        fila('prefs.lang', 'prefs.lang.help', seg('idioma', idioma(), IDIOMES.map(([v, nom]) => [v, nom]), 'prefs.lang')) +
        fila('prefs.detail', 'prefs.detail.help', seg('vista', vista,
          [['summary', 'prefs.detail.summary'], ['normal', 'prefs.density.normal'], ['verbose', 'prefs.detail.verbose']], 'prefs.detail')) +
        funPrefs.dictationRow() +
        // El perfil de la persona (tema, idioma, densitat…) viatja amb el compte.
        fila('prefs.sync', 'prefs.sync.help', check('prefsSync', profileSync.enabled(), T('prefs.sync')))],
      ['appearance', 'palette', this.select('tema') + this.segment('densitat') + this.segment('mida') + collective.section() + brands.section()],
      ['agent', 'model',
        // Què fa servir ara i si pot respondre (es pinta quan respon el servidor).
        '<div class="pf-row" id="prefsModelRow"><div class="pf-lab"><b>' + esc(T('prefs.providers')) + '</b><span>' + esc(T('prefs.providers.help')) + '</span></div>' +
        '<div class="pf-btns"><button type="button" class="pf-open" id="prefsProv">' + esc(L('Models i connexions…', 'Models & connections…')) + '</button></div></div>' +
        fila('prefs.perms', 'prefs.perms.help', seg('perm', perm,
          [['ask', 'perm.ask'], ['edits', 'perm.edits'], ['full', 'perm.full']], 'prefs.perms')) +
        // El revisor de permisos és avançat: fora del composer, aquí.
        fila('prefs.reviewer', 'prefs.reviewer.help', check('prefsReviewer', G().state && G().state.auto_review, T('prefs.reviewer'))) +
        // El revisor passava el model per sobre de CADA resposta i no hi
        // havia manera d'apagar-lo des de la finestra: només editant el
        // config a mà o des del TUI. Aquí és on el busques.
        fila('prefs.verify', 'prefs.verify.help', check('prefsVerify', this.verifyAuto(), T('prefs.verify'))) +
        '<h3 class="pf-sec">' + T('prefs.skills') + '</h3>' +
        '<p class="pf-skills-help">' + T('prefs.skills.help') + '</p>' +
        '<div id="prefsSkills"><div class="pf-skills-status">' + T('prefs.skills.loading') + '</div></div>'],
      // Calendari i correu: abans era el primer de General, i empenyia
      // l'idioma avall. És una integració amb Google, com les altres.
      ['integrations', 'plug', '<div id="prefsConn" class="pf-conn"></div>' + chromeExt.section() + wordAddin.section() + shares.section()],
      // Les veus només serveixen per als jocs (plugins): hi van a sota.
      ['plugins', 'puzzle', '<div id="prefsPlugins">' + this.pluginsHTML() + '</div>' +
        '<div id="prefsVoiceSec" class="pf-sub"><h3 class="pf-sec">' + esc(T('prefs.group.voice')) + '</h3>' + voicePrefs.section() + '</div>'],
      ['extras', 'sparkles', funPrefs.section()],
      ['advanced', 'sliders',
        fila('prefs.shortcuts', 'prefs.shortcuts.help', '<button type="button" class="pf-open" id="prefsKeys">' + esc(L('Mostra', 'Show')) + '</button>') +
        funPrefs.kanbanRow() +
        desktopUpdates.section() +
        '<div id="prefsAccounts" class="pf-sub" hidden></div><div id="prefsPowers" class="pf-sub" hidden></div>'],
    ];
    cos.innerHTML =
      '<nav class="pf-nav" aria-label="' + T('prefs.title') + '">' +
      grups.map(([id, icona]) => '<button type="button" data-g="' + id + '"><span class="pf-nav-ic" aria-hidden="true">' + icon(icona) + '</span>' + esc(GRUPS[id]()[0]) + '</button>').join('') +
      '</nav><div class="pf-main" id="prefsMain">' +
      grups.map(([id, , dins]) => '<section class="pf-group" id="prefsG-' + id + '" data-g="' + id + '" aria-labelledby="prefsT-' + id + '">' +
        '<header class="pf-ghead"><h3 class="pf-gtitle" id="prefsT-' + id + '">' + esc(GRUPS[id]()[0]) + '</h3>' +
        '<p class="pf-gdesc">' + esc(GRUPS[id]()[1]) + '</p></header>' +
        '<div class="pf-gbody">' + dins + '</div></section>').join('') +
      '<p class="pf-noresults" id="prefsNoResults" hidden>' + T('prefs.search.empty') + '</p></div>';

    cos.querySelectorAll('.pf-seg').forEach(seg => {
      seg.querySelectorAll('button').forEach(b => {
        b.onclick = async () => {
          const k = seg.dataset.k, v = b.dataset.v;
          seg.querySelectorAll('button').forEach(x => { x.classList.remove('on'); x.setAttribute('aria-pressed', 'false'); });
          b.classList.add('on'); b.setAttribute('aria-pressed', 'true');
          if (OPCIONS[k]) { desa(k, v); aplica(k); if (k === 'tema') pintaBarraTitol(); return; }
          if (k === 'vista' && window.gregalSetView) window.gregalSetView(v);
          if (k === 'perm' && window.gregalSetPerm) {
            await window.gregalSetPerm(v);
            this.repinta();
          }
          if (k === 'idioma') this.posaIdioma(v);
        };
      });
    });
    cos.querySelectorAll('.pf-select[data-k]').forEach(sel => {
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
        if (k === 'tema') { pintaBarraTitol(); this.repinta(); }
      };
    });
    const v = $('prefsVerify');
    if (v) v.onchange = () => this.posaVerify(v.checked);
    const ps = $('prefsSync');
    if (ps) ps.onchange = () => profileSync.setEnabled(ps.checked);
    const rv = $('prefsReviewer');
    if (rv) rv.onchange = () => this.posaRevisor(rv.checked);
    this.carregaModel();
    this.carregaSkills();
    this.carregaAccounts();
    this.carregaPowers();
    this.carregaConnexions();
    const k = $('prefsKeys');
    if (k) k.onclick = () => { this.tanca(); this.obreDreceres(); };
    this.lligaPlugins();
    voicePrefs.bind(cos);
    collective.bind(cos);
    desktopUpdates.bind(cos);
    chromeExt.bind(cos, () => this.repinta());
    wordAddin.bind(cos, () => this.repinta());
    shares.bind(cos);
    funPrefs.bind(cos);
    // Repintar conserva el desplaçament: la secció de brandings és llarga.
    brands.bind(cos, () => this.repinta());
    // Índex: un clic hi baixa; en desplaçar-te, marca el grup que mires.
    const main = $('prefsMain');
    cos.querySelectorAll('.pf-nav button').forEach(b => {
      b.onclick = () => {
        const g = $('prefsG-' + b.dataset.g);
        if (g && main) main.scrollTo({ top: g.offsetTop - main.offsetTop, behavior: 'smooth' });
      };
    });
    if (main) main.onscroll = () => this.marcaNav();
    this.filtra();
  },

  // La fila del model: quin fa servir Gregal ara i, si no en pot fer servir
  // cap (primer inici, clau buida), un botó per connectar-ne un.
  async carregaModel() {
    const row = $('prefsModelRow');
    if (!row) return;
    const obreProv = () => { this.tanca(); if (window.toggleProv) window.toggleProv(true); };
    const p0 = $('prefsProv');
    if (p0) p0.onclick = obreProv;
    const info = await onboarding.status();
    if (!info || !row.isConnected) return;
    const lab = el('div', 'pf-lab'), btns = el('div', 'pf-btns');
    lab.append(el('b', '', T('prefs.providers')));
    if (info.needs && !info.admin) {
      lab.append(el('span', 'pf-warn', L('Encara no hi ha cap model connectat. Demana a l’administrador d’aquest servidor que n’hi connecti un.',
        'No model is connected yet. Ask this server’s administrator to connect one.')));
      row.replaceChildren(lab);
      this.filtra();
      return;
    } else if (info.needs) {
      lab.append(el('span', 'pf-warn', L('Encara no hi ha cap model connectat: sense una clau d’API, l’agent no pot respondre.',
        'No model is connected yet: without an API key, the agent cannot answer.')));
      btns.append(boto(L('Connecta un model', 'Connect a model'), () => { this.tanca(); onboarding.open(); }, 'pf-primary'));
    } else {
      const ara = el('span');
      ara.append(L('Ara: ', 'Now: '), el('strong', '', info.model), ' · ' + info.label + '. ' + L('Pots tenir-ne més d’un i triar quin model fa cada feina.', 'You can have several and choose which model does each job.'));
      lab.append(ara);
      if (info.admin) btns.append(boto(L('Canvia de proveïdor', 'Switch provider'), () => { this.tanca(); onboarding.open(); }));
    }
    btns.append(boto(L('Models i connexions…', 'Models & connections…'), obreProv));
    row.replaceChildren(lab, btns);
    this.filtra();
  },

  // Calendari (Google Calendar per l'adreça iCal) i correu (Gmail per IMAP amb
  // contrasenya d'aplicació) de cada compte, i el resum del matí (connections.go).
  async carregaConnexions(missatge = '') {
    const box = $('prefsConn'); if (!box) return;
    let data, winds = [];
    try {
      const r = await window.api('/api/connections'); if (!r.ok || !box.isConnected) return;
      data = await r.json();
      const rw = await window.api('/api/winds'); if (rw.ok) winds = (await rw.json()).winds || [];
    } catch (_) { return; }
    if (!box.isConnected) return;
    const msg = estat();
    const post = async (body, ok) => {
      diu(msg, L('Un moment…', 'One moment…'));
      try {
        const r = await window.api('/api/connections', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
        if (!r.ok) throw new Error((await r.text()).trim());
        await this.carregaConnexions(ok);
      } catch (err) { diu(msg, L('No s’ha pogut desar: ', 'Could not save: ') + err.message, true); }
    };
    const link = (href, label) => { const a = el('a', '', label); a.href = href; a.target = '_blank'; a.rel = 'noopener'; return a; };
    box.hidden = false; box.replaceChildren();
    box.append(el('h3', 'pf-sec', L('Calendari i correu', 'Calendar and email')));
    box.append(el('p', 'pf-skills-help', L('Els teus vents poden llegir el teu Google Calendar i el teu Gmail (mai no envien ni accepten res) i fer-te un resum cada matí. Cada compte té els seus.',
      'Your winds can read your Google Calendar and Gmail (they never send or accept anything) and give you a briefing every morning. Each account has its own.')));

    // Google Calendar: la llista i el formulari per afegir-ne un.
    const calHelp = el('span');
    calHelp.append(L('Al ', 'In '), link('https://calendar.google.com/calendar/r/settings', 'Google Calendar → ' + L('Configuració', 'Settings')),
      L(' → el teu calendari → «Integra el calendari», copia l’«Adreça secreta en format iCal».', ' → your calendar → “Integrate calendar”, copy the “Secret address in iCal format”.'));
    const calRow = filaEl('Google Calendar', calHelp);
    calRow.classList.add('pf-row-stack');
    const calLab = calRow.querySelector('.pf-lab');
    const cals = data.calendars || [];
    if (cals.length) {
      const ul = el('ul', 'pf-list');
      for (const c of cals) {
        const li = el('li');
        const nom = el('div', 'pf-list-main');
        nom.append(el('b', '', c.name), el('small', '', c.url));
        const treu = boto('', () => post({ remove_calendar: c.name }, L('Calendari tret.', 'Calendar removed.')), 'pf-icon');
        treu.innerHTML = icon('trash'); treu.title = L('Treu ', 'Remove ') + c.name; treu.setAttribute('aria-label', treu.title);
        const ic = el('div', 'pf-list-ic'); ic.innerHTML = icon('calendar');
        li.append(ic, nom, treu);
        ul.append(li);
      }
      calLab.append(ul);
    }
    const calName = entrada(L('Nom (p. ex. Personal)', 'Name (e.g. Personal)')), calURL = entrada('https://calendar.google.com/calendar/ical/…/basic.ics', 'url');
    calName.classList.add('pf-input-short');
    const calForm = el('form', 'pf-inline');
    calForm.append(calName, calURL, boto(L('Afegeix', 'Add'), null));
    calForm.lastChild.type = 'submit';
    calForm.onsubmit = e => {
      e.preventDefault();
      if (!calName.value.trim()) { diu(msg, L('Posa-li un nom al calendari.', 'Give the calendar a name.'), true); calName.focus(); return; }
      if (!/^https:\/\//.test(calURL.value.trim())) { diu(msg, L('Enganxa l’adreça iCal (comença per https://).', 'Paste the iCal address (it starts with https://).'), true); calURL.focus(); return; }
      post({ add_calendar: { name: calName.value.trim(), url: calURL.value.trim() } }, L('Calendari afegit.', 'Calendar added.'));
    };
    calLab.append(calForm);
    box.append(calRow);

    // Gmail: connectat (amb qui) o el formulari amb la contrasenya d'aplicació.
    const m = data.mail || {};
    const mailHelp = el('span');
    if (m.has_password) mailHelp.append(L('Connectat com a ', 'Connected as '), el('strong', '', m.user || 'Gmail'), '.');
    else mailHelp.append(L('Fes servir una ', 'Use an '), link('https://myaccount.google.com/apppasswords', L('contrasenya d’aplicació', 'app password')),
      L(' (cal la verificació en dos passos), no la teva contrasenya de Google.', ' (needs 2-Step Verification), not your Google password.'));
    const mailRow = filaEl('Gmail', mailHelp);
    if (m.has_password) {
      mailRow.append(boto(L('Desconnecta', 'Disconnect'), () => post({ remove_mail: true }, L('Gmail desconnectat.', 'Gmail disconnected.'))));
    } else {
      mailRow.classList.add('pf-row-stack');
      const mailUser = entrada(L('el.teu@gmail.com', 'you@gmail.com'), 'email', m.user || '');
      const mailPass = entrada(L('Contrasenya d’aplicació (16 lletres)', 'App password (16 letters)'), 'password');
      mailPass.autocomplete = 'new-password';
      const mailForm = el('form', 'pf-inline');
      mailForm.append(mailUser, mailPass, boto(L('Connecta', 'Connect'), null));
      mailForm.lastChild.type = 'submit';
      mailForm.onsubmit = e => {
        e.preventDefault();
        if (!mailUser.value.trim() || !mailPass.value.trim()) { diu(msg, L('Cal l’adreça i la contrasenya d’aplicació.', 'The address and the app password are both needed.'), true); (mailUser.value.trim() ? mailPass : mailUser).focus(); return; }
        post({ mail: { host: m.host || 'imap.gmail.com', port: m.port || 993, user: mailUser.value.trim(), password: mailPass.value } }, L('Gmail connectat.', 'Gmail connected.'));
      };
      mailRow.querySelector('.pf-lab').append(mailForm);
    }
    box.append(mailRow);

    // Resum del matí: un interruptor, l'hora i qui el fa. Es desa en tocar-ho.
    const b = data.briefing || {};
    const ctrls = el('div', 'pf-btns');
    const time = entrada(L('Hora', 'Time'), 'time', b.time || '08:00');
    time.classList.add('pf-input-time');
    const wsel = el('select', 'pf-select');
    wsel.setAttribute('aria-label', L('Qui el fa', 'Who gives it'));
    for (const w of winds) { const o = el('option', '', w.name); o.value = w.id; if (w.id === b.wind) o.selected = true; wsel.append(o); }
    if (!b.wind && winds.length) { const pref = winds.find(w => /organi/i.test(w.role || '')) || winds[0]; wsel.value = pref.id; }
    const sw = el('label', 'pf-check'), on = el('input');
    on.type = 'checkbox'; on.checked = !!b.enabled; on.setAttribute('aria-label', L('Resum del matí', 'Morning briefing'));
    sw.append(on, el('span'));
    const desaResum = () => post({ briefing: { enabled: on.checked, time: time.value || '08:00', wind: wsel.value } },
      on.checked ? L('Resum programat a les ', 'Briefing scheduled at ') + (time.value || '08:00') + '.' : L('Resum desactivat.', 'Briefing off.'));
    on.onchange = desaResum;
    time.onchange = () => { if (on.checked) desaResum(); };
    wsel.onchange = () => { if (on.checked) desaResum(); };
    if (!winds.length) { on.disabled = true; wsel.disabled = true; time.disabled = true; wsel.hidden = true; }
    ctrls.append(time, wsel, sw);
    box.append(filaEl(L('Resum del matí', 'Morning briefing'),
      winds.length ? L('Cada dia a l’hora que triïs, un vent et resumeix l’agenda i el correu.', 'Every day at the time you choose, a wind sums up your agenda and email.')
        : L('Primer crea un vent a Gregal Vents (o activa la colla del dia a dia).', 'First create a wind in Gregal Vents (or start the everyday crew).'), ctrls));

    // Provar només té sentit amb alguna cosa connectada.
    const accions = el('div', 'pf-actions');
    if (cals.length || m.has_password) {
      accions.append(boto(L('Prova la connexió', 'Test the connection'), async () => {
        diu(msg, L('Provant…', 'Testing…'));
        try {
          const r = await window.api('/api/connections', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ action: 'test' }) });
          const j = await r.json(); const parts = []; let dolent = false;
          if (j.calendar) {
            parts.push(L('Calendari: ', 'Calendar: ') + j.calendar.events_next_7_days + L(' esdeveniments els pròxims 7 dies', ' events in the next 7 days'));
            if ((j.calendar.problems || []).length) { parts.push(j.calendar.problems.join('; ')); dolent = true; }
          }
          if (j.mail) { parts.push('Gmail: ' + (j.mail.error ? j.mail.error : j.mail.unread + L(' sense llegir', ' unread'))); if (j.mail.error) dolent = true; }
          diu(msg, parts.join(' · ') || L('No hi ha res connectat.', 'Nothing connected.'), dolent);
        } catch (err) { diu(msg, err.message, true); }
      }));
    }
    accions.append(msg);
    box.append(accions);
    if (missatge) diu(msg, missatge);
    this.filtra();
  },

  // Poders de servidor (server_powers.go): només l'administrador els veu.
  async carregaPowers(missatge = '') {
    const box = $('prefsPowers'); if (!box) return;
    try {
      const response = await window.api('/api/server/powers');
      if (!response.ok || !box.isConnected) return;
      const data = await response.json(); if (!box.isConnected) return;
      box.hidden = false; box.replaceChildren();
      box.append(el('h3', 'pf-sec', L('Poders de servidor', 'Server powers')));
      box.append(el('p', 'pf-skills-help', L(
        'Els teus vents fan de sysadmin en aquesta màquina, com un agent amb terminal: fan i canvien webs (també les que ja serveix), instal·len paquets, despleguen i obren túnels de Cloudflare. No poden fer servir sudo, aturar el Gregal ni tocar camins crítics. Mai per als altres comptes.',
        'Your winds act as sysadmins on this machine, like an agent with a terminal: they build and change websites (including the ones it already serves), install packages, deploy and open Cloudflare tunnels. They cannot use sudo, stop Gregal or touch critical paths. Never for other accounts.')));
      const status = estat();
      const sw = el('label', 'pf-check'), cb = el('input');
      cb.type = 'checkbox'; cb.checked = !!data.enabled; cb.setAttribute('aria-label', L('Activa els poders de servidor', 'Enable server powers'));
      sw.append(cb, el('span'));
      box.append(filaEl(L('Activa els poders de servidor', 'Enable server powers'),
        L('A la màquina: ', 'On the machine: ') + ((data.tools || []).join(', ') || '—') + (data.user ? ' · ' + data.user : ''), sw));
      cb.onchange = async () => {
        cb.disabled = true;
        try {
          const r = await window.api('/api/server/powers', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ enabled: cb.checked }) });
          if (!r.ok) throw new Error(await r.text());
          diu(status, cb.checked ? L('Activats: els teus vents ja hi poden treballar.', 'On: your winds can work on the server now.') : L('Desactivats.', 'Off.'));
        } catch (err) { cb.checked = !cb.checked; diu(status, L('No s’ha pogut desar: ', 'Could not save: ') + err.message, true); }
        finally { cb.disabled = false; }
      };
      const tunnels = data.tunnels || [];
      if (tunnels.length) {
        const lab = el('div', 'pf-lab');
        lab.append(el('b', '', L('Túnels oberts', 'Open tunnels')));
        const ul = el('ul', 'pf-list');
        for (const t of tunnels) {
          const li = el('li'), main = el('div', 'pf-list-main'), a = el('a', '', t.url || t.name);
          a.href = t.url || '#'; a.target = '_blank'; a.rel = 'noopener';
          main.append(a, el('small', '', '→ ' + t.target + ' · ' + t.status));
          const ic = el('div', 'pf-list-ic'); ic.innerHTML = icon('navegador');
          li.append(ic, main); ul.append(li);
        }
        lab.append(ul);
        const row = el('div', 'pf-row pf-row-stack'); row.append(lab); box.append(row);
      }
      const notesRow = el('div', 'pf-row pf-row-stack'), notesLab = el('div', 'pf-lab');
      notesLab.append(el('b', '', L('Notes del servidor', 'Server notes')),
        el('span', '', L('El que els vents saben de la màquina (webs, carpetes, dominis, com es despleguen). Les llegeixen a cada torn i les van posant al dia. Sense contrasenyes ni claus.',
          'What the winds know about the machine (sites, folders, domains, how they deploy). They read them every turn and keep them up to date. No passwords or keys.')));
      const notes = el('textarea', 'pf-notes'); notes.rows = 6; notes.value = data.notes || '';
      notes.setAttribute('aria-label', L('Notes del servidor', 'Server notes'));
      notes.placeholder = L('- blog: /srv/www/blog, Caddy, blog.exemple.cat\n- túnel per defecte: casa', '- blog: /srv/www/blog, Caddy, blog.example.com\n- default tunnel: home');
      const save = boto(L('Desa les notes', 'Save notes'), async () => {
        save.disabled = true;
        try {
          const r = await window.api('/api/server/powers', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ notes: notes.value }) });
          if (!r.ok) throw new Error(await r.text());
          diu(status, L('Notes desades.', 'Notes saved.'));
        } catch (err) { diu(status, L('No s’han pogut desar: ', 'Could not save: ') + err.message, true); }
        finally { save.disabled = false; }
      });
      const acc = el('div', 'pf-actions'); acc.append(save, status);
      notesLab.append(notes, acc); notesRow.append(notesLab); box.append(notesRow);
      if (missatge) diu(status, missatge);
      this.filtra();
    } catch (_) { /* Només l'administrador ho veu. */ }
  },

  // Comptes del servidor (server_accounts.go): crear-ne, canviar contrasenyes,
  // fer administradors, esborrar-ne i convidar amb un codi d'un sol ús.
  async carregaAccounts(missatge = '') {
    const box = $('prefsAccounts'); if (!box) return;
    try {
      const response = await window.api('/api/server/accounts');
      if (!response.ok || !box.isConnected) return;
      const data = await response.json(); if (!box.isConnected) return;
      box.hidden = false; box.replaceChildren();
      box.append(el('h3', 'pf-sec', L('Comptes del servidor', 'Server accounts')));
      box.append(el('p', 'pf-skills-help', L('Crea un perfil amb usuari i contrasenya per a cada persona. Cada perfil té el seu espai.', 'Create a profile with a username and password for each person. Each profile has its own space.')));
      const message = estat();
      let action = 'create';
      const name = entrada(L('Nom d’usuari', 'Username'));
      name.required = true; name.pattern = '[A-Za-z][A-Za-z0-9_-]{1,31}'; name.maxLength = 32;
      const password = entrada(L('Contrasenya (mínim 8 caràcters)', 'Password (at least 8 characters)'), 'password');
      password.autocomplete = 'new-password'; password.minLength = 8; password.required = true;
      const submit = el('button', 'pf-open pf-primary', L('Crea el compte', 'Create account')); submit.type = 'submit';
      const cancel = boto(L('Cancel·la', 'Cancel'), () => {
        action = 'create'; name.readOnly = false; name.value = ''; password.value = ''; submit.textContent = L('Crea el compte', 'Create account'); cancel.hidden = true;
      });
      cancel.hidden = true;
      const ul = el('ul', 'pf-list');
      for (const account of data.accounts || []) {
        const li = el('li'), main = el('div', 'pf-list-main'), ic = el('div', 'pf-list-ic');
        ic.innerHTML = icon('users');
        main.append(el('b', '', account.name));
        if (account.admin) main.append(el('small', 'pf-badge', L('administrador', 'administrator')));
        li.append(ic, main);
        if (account.managed) {
          const btns = el('div', 'pf-list-btns');
          btns.append(boto(L('Contrasenya', 'Password'), () => {
            action = 'reset'; name.value = account.name; name.readOnly = true; password.value = ''; password.focus();
            submit.textContent = L('Desa la contrasenya', 'Save password'); cancel.hidden = false;
          }));
          // Fer-lo administrador: el mateix que l'admin del config (poders de servidor, comptes, proveïdors).
          const admin = boto(account.admin ? L('Treu administrador', 'Remove administrator') : L('Fes-lo administrador', 'Make administrator'), async () => {
            admin.disabled = true;
            try {
              const r = await window.api('/api/server/accounts', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ action: 'admin', name: account.name, admin: !account.admin }) });
              if (!r.ok) throw new Error(await r.text());
              await this.carregaAccounts(account.admin ? L('Ja no és administrador.', 'No longer an administrator.') : L('Ara és administrador: té els mateixos permisos que tu.', 'Now an administrator: same permissions as you.'));
            } catch (err) { admin.disabled = false; diu(message, err.message, true); }
          });
          const remove = boto('', async () => {
            if (!confirm(L('Esborrar el compte «' + account.name + '»? Se’n tancaran les sessions i les seves dades s’apartaran (no s’esborren del disc).',
              'Delete the account "' + account.name + '"? Its sessions will be closed and its data set aside (not erased from disk).'))) return;
            remove.disabled = true;
            try {
              const r = await window.api('/api/server/accounts', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ action: 'delete', name: account.name }) });
              if (!r.ok) throw new Error(await r.text());
              await this.carregaAccounts(L('Compte esborrat.', 'Account deleted.'));
            } catch (err) { remove.disabled = false; diu(message, L('No s’ha pogut esborrar: ', 'Could not delete: ') + err.message, true); }
          }, 'pf-icon pf-danger');
          remove.innerHTML = icon('trash'); remove.title = L('Esborra el compte', 'Delete account'); remove.setAttribute('aria-label', remove.title + ' ' + account.name);
          btns.append(admin, remove);
          li.append(btns);
        }
        ul.append(li);
      }
      box.append(ul);
      const form = el('form', 'pf-inline');
      form.append(name, password, submit, cancel);
      form.onsubmit = async e => {
        e.preventDefault(); submit.disabled = true; diu(message, '');
        try {
          const r = await window.api('/api/server/accounts', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ action, name: name.value, password: password.value }) });
          if (!r.ok) throw new Error(await r.text());
          password.value = '';
          await this.carregaAccounts(action === 'reset' ? L('Contrasenya actualitzada.', 'Password updated.') : L('Perfil creat. Ja pot iniciar sessió.', 'Profile created. They can now sign in.'));
        } catch (err) { diu(message, (action === 'reset' ? L('No s’ha pogut canviar la contrasenya: ', 'Could not change the password: ') : L('No s’ha pogut crear el compte: ', 'Could not create account: ')) + err.message, true); }
        finally { submit.disabled = false; }
      };
      const formRow = el('div', 'pf-row pf-row-stack'), formLab = el('div', 'pf-lab');
      formLab.append(el('b', '', L('Compte nou', 'New account')), form, message);
      formRow.append(formLab); box.append(formRow);
      const inviteOut = estat();
      const invite = boto(L('Genera un codi', 'Generate a code'), async () => {
        invite.disabled = true; diu(inviteOut, '');
        try {
          const r = await window.api('/api/server/invitations', { method: 'POST' });
          if (!r.ok) throw new Error(await r.text());
          const result = await r.json();
          inviteOut.replaceChildren(L('Codi d’un sol ús (caduca en 30 minuts): ', 'One-use code (expires in 30 minutes): '), el('code', '', result.code));
        } catch (err) { diu(inviteOut, L('No s’ha pogut generar la invitació: ', 'Could not generate invitation: ') + err.message, true); }
        finally { invite.disabled = false; }
      });
      const invLab = el('div', 'pf-lab');
      invLab.append(el('b', '', L('Invitacions', 'Invitations')), el('span', '', L('Un codi perquè algú es creï el compte sense que li hagis de donar cap contrasenya.', 'A code so someone can create their own account without you giving them a password.')), inviteOut);
      const invRow = el('div', 'pf-row'); invRow.append(invLab, invite); box.append(invRow);
      if (missatge) diu(message, missatge);
      this.filtra();
    } catch (_) { /* Només als servidors amb comptes. */ }
  },

  // repinta conserva on eres i la cerca.
  repinta() {
    const main = $('prefsMain');
    const top = main ? main.scrollTop : 0;
    this.pinta();
    const m2 = $('prefsMain');
    if (m2) m2.scrollTop = top;
  },

  marcaNav() {
    const main = $('prefsMain');
    if (!main || !main.offsetParent) return;
    const grups = [...main.querySelectorAll('.pf-group:not([hidden])')];
    let actiu = grups[0];
    for (const g of grups) {
      if (g.offsetTop - main.offsetTop - main.scrollTop <= 24) actiu = g;
    }
    // Al final de tot, l'últim grup (pot ser massa curt per arribar a dalt).
    if (main.scrollTop > 0 && main.scrollTop + main.clientHeight >= main.scrollHeight - 4 && grups.length) actiu = grups[grups.length - 1];
    document.querySelectorAll('#prefsBody .pf-nav button').forEach(b => {
      const on = !!actiu && b.dataset.g === actiu.dataset.g;
      b.classList.toggle('on', on);
      if (on) b.setAttribute('aria-current', 'true'); else b.removeAttribute('aria-current');
    });
  },

  // filtra amaga les files que no contenen el que busques (sense accents
  // ni majúscules). Un grup o subtítol que coincideix es mostra sencer. Un
  // bloc que porta el seu propi subtítol (calendari, comptes, poders…) es
  // compta com una secció a part.
  filtra() {
    const inp = $('prefsSearch');
    const norm = s => String(s || '').normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
    const q = norm(inp ? inp.value.trim() : '');
    const main = $('prefsMain');
    if (!main) return;
    let total = 0;
    main.querySelectorAll('.pf-group').forEach(g => {
      const items = [...g.querySelector('.pf-gbody').children];
      const capGrup = !q || norm(g.querySelector('.pf-gtitle').textContent).includes(q);
      let visibles = 0;
      let cap = null, capMostra = false, capFills = 0;
      const tancaCap = () => { if (cap) cap.hidden = !(capMostra || capFills > 0); };
      for (const el of items) {
        if (el.matches('h3.pf-sec')) {
          tancaCap();
          cap = el; capMostra = capGrup || norm(el.textContent).includes(q); capFills = 0;
          continue;
        }
        // Un bloc buit i amagat (comptes sense servidor de comptes) es queda amagat.
        if (el.matches('.pf-sub, .pf-conn') && !el.childElementCount) { el.hidden = true; continue; }
        if (el.querySelector(':scope > h3.pf-sec')) { tancaCap(); cap = null; capMostra = false; }
        const ok = capGrup || (cap && capMostra) || norm(el.textContent).includes(q) ||
          [...el.querySelectorAll('[aria-label],[placeholder]')].some(x => norm((x.getAttribute('aria-label') || '') + ' ' + (x.getAttribute('placeholder') || '')).includes(q));
        el.hidden = !ok;
        if (ok) { visibles++; capFills++; }
      }
      tancaCap();
      g.hidden = !(capGrup || visibles > 0);
      if (!g.hidden) total++;
      const nb = document.querySelector('#prefsBody .pf-nav button[data-g="' + g.dataset.g + '"]');
      if (nb) nb.hidden = g.hidden;
    });
    const buit = $('prefsNoResults');
    if (buit) buit.hidden = total > 0;
    this.marcaNav();
  },

  // ---- Plugins: Racó, Partits, Arena ----
  pluginsHTML() {
    if (!plugins.loaded()) return '<div class="pf-skills-status">' + esc(T('prefs.plugins.loading')) + '</div>';
    const list = plugins.list();
    if (!list.length) return '<div class="pf-skills-status">' + esc(T('prefs.plugins.none')) + '</div>';
    const admin = plugins.isAdmin();
    return (admin ? '' : '<p class="pf-skills-help">' + esc(T('prefs.plugins.adminOnly')) + '</p>') +
      list.map(p => {
        const nom = T('plugin.' + p.id + '.name', p.name);
        return '<div class="pf-row pf-plugin' + (p.enabled ? ' on' : '') + '" data-plugin="' + esc(p.id) + '">' +
          '<span class="pf-plugin-ic" aria-hidden="true">' + icon(PLUGIN_ICONS[p.id] || 'puzzle') + '</span>' +
          '<div class="pf-lab"><b>' + esc(nom) + '</b><span>' + esc(T('plugin.' + p.id + '.desc', p.description)) + '</span></div>' +
          '<button type="button" class="pf-open" data-open="' + esc(p.view) + '"' + (p.enabled ? '' : ' disabled') + '>' + esc(T('prefs.plugins.open')) + '</button>' +
          '<label class="pf-check" title="' + esc(T(p.enabled ? 'prefs.plugins.on' : 'prefs.plugins.off')) + '"><input type="checkbox" data-plugin-toggle="' + esc(p.id) + '" aria-label="' + esc(nom) + '"' +
          (p.enabled ? ' checked' : '') + (admin ? '' : ' disabled') + '><span></span></label></div>';
      }).join('') +
      '<div class="pf-skills-status error" id="prefsPluginsErr" hidden></div>';
  },
  lligaPlugins() {
    const box = $('prefsPlugins');
    if (!box) return;
    const pinta = () => { box.innerHTML = this.pluginsHTML(); this.lligaPlugins(); this.filtra(); };
    if (!plugins.loaded()) { plugins.load().then(ok => { if (ok && $('prefsPlugins') === box) pinta(); }); return; }
    box.querySelectorAll('[data-open]').forEach(b => {
      b.onclick = () => { this.tanca(); G().setView(b.dataset.open); };
    });
    box.querySelectorAll('input[data-plugin-toggle]').forEach(inp => {
      inp.onchange = async () => {
        const all = [...box.querySelectorAll('input[data-plugin-toggle]')];
        all.forEach(x => { x.disabled = true; });
        try {
          await plugins.set(inp.dataset.pluginToggle, inp.checked);
          pinta();
        } catch (e) {
          inp.checked = !inp.checked;
          all.forEach(x => { x.disabled = false; });
          const err = $('prefsPluginsErr');
          if (err) { err.hidden = false; err.textContent = T('prefs.plugins.error') + ' ' + e.message; }
        }
      };
    });
  },

  async carregaSkills() {
    const box = $('prefsSkills');
    if (!box) return;
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
      this.repinta();
    } catch (e) {
      alert(L('No s’ha pogut canviar l’idioma: ', 'Could not change the language: ') + e.message);
      this.repinta();
    }
  },

  // El revisor té cinc modes, però la pregunta que et fas és una: revisa
  // sol o no. «strict» no es toca des d'aquí —és un bloqueig deliberat i
  // apagar-lo per error seria pitjor que haver de venir a buscar-lo.
  verifyAuto() {
    const m = (G().state && G().state.verify) || 'manual';
    return m === 'auto' || m === 'both' || m === 'strict';
  },
  // El revisor de permisos: un model ràpid decideix el que demanaria
  // permís (agent/autoreview.go). No toca el permís d'Actua; el mode
  // permissiu antic queda com estava (l'encén l'autonomia total).
  async posaRevisor(on) {
    const st = G().state || {};
    try {
      const r = await G().api('/api/permissive', { method: 'POST', body: JSON.stringify({ on: !!st.permissive && !on, review: on }) });
      if (!r.ok) throw new Error((await r.text()).trim());
      await G().refresh();
    } catch (e) {
      alert(T('prefs.reviewer') + ': ' + e.message);
    }
    this.repinta();
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
    } catch (e) {
      alert(L('No s’ha pogut canviar el revisor: ', 'Could not change the reviewer: ') + e.message);
    }
    this.repinta();
  },
};
