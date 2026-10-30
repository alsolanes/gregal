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
import { wordAddin } from './word-addin.js';
import { shares } from './export.js';
import { funPrefs } from './fun-prefs.js';
import { brands } from './brands.js';
import { plugins } from './plugins.js';
import { voicePrefs } from './voice-prefs.js';

const $ = id => document.getElementById(id);
const G = () => window.gregal;

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
    plugins.init();
    const cerca = $('prefsSearch');
    if (cerca) cerca.oninput = () => this.filtra();
    // Quan canvien els plugins (des d'aquí o d'una altra pestanya), la
    // secció es repinta si és oberta.
    document.addEventListener('gregal:plugins', () => {
      const box = $('prefsPlugins');
      if (box && modal && modal.classList.contains('on')) { box.innerHTML = this.pluginsHTML(); this.lligaPlugins(); this.filtra(); }
    });
    document.addEventListener('keydown', e => {
      if (e.key === 'Escape' && modal && modal.classList.contains('on')) { e.stopPropagation(); this.tanca(); }
      if ((e.ctrlKey || e.metaKey) && e.key === ',') { e.preventDefault(); this.obre(); }
    });
  },
  // obre('plugins') hi entra directament al grup que demanis.
  obre(grup) {
    const modal = $('prefsModal');
    if (!modal) return;
    this.pinta();
    modal.classList.add('on');
    const g = typeof grup === 'string' && $('prefsG-' + grup), main = $('prefsMain');
    if (g && main) main.scrollTop = g.offsetTop - main.offsetTop;
    // Les posicions només existeixen amb el modal visible.
    this.marcaNav();
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
  // pinta: Preferències en grups (General, Aparença, Models i agent,
  // Plugins, Extres, Integracions, Avançat), cadascun amb títol i una línia
  // que diu què hi trobaràs. A l'esquerra, l'índex dels grups; a dalt, una
  // cerca que filtra files. Les seccions que pinten altres mòduls (marques,
  // extres, extensions…) hi entren tal qual: el seu h3 queda de subtítol.
  pinta() {
    const cos = $('prefsBody');
    if (!cos) return;
    const vista = (() => { try { return localStorage.getItem('gregal_view') || 'normal'; } catch (e) { return 'normal'; } })();
    // El permís d'Actua viu al servidor (/api/access), igual que al composer.
    const perm = (G().state && G().state.access) || 'edits';
    const fila = (titol, ajuda, dins) => '<div class="pf-row"><div class="pf-lab"><b>' + T(titol) + '</b>' +
      (ajuda ? '<span>' + T(ajuda) + '</span>' : '') + '</div>' + dins + '</div>';
    const seg = (k, ara2, valors) => '<div class="pf-seg" data-k="' + k + '">' +
      valors.map(([v, clau]) => '<button class="' + (v === ara2 ? 'on' : '') + '" data-v="' + v + '">' + T(clau) + '</button>').join('') +
      '</div>';
    // Treu l'h3 del principi quan la secció d'un mòdul és tot el grup: el
    // títol ja el posa el grup.
    const senseCap = html => String(html || '').replace(/^\s*<h3 class="pf-sec"[^>]*>[\s\S]*?<\/h3>/, '');
    const grups = [
      ['general', '⚙', fila('prefs.lang', 'prefs.lang.help', seg('idioma', idioma(), IDIOMES.map(([v, nom]) => [v, nom]))) +
        fila('prefs.detail', 'prefs.detail.help', seg('vista', vista,
          [['summary', 'prefs.detail.summary'], ['normal', 'prefs.density.normal'], ['verbose', 'prefs.detail.verbose']]))],
      ['appearance', '◐', this.select('tema') + this.segment('densitat') + this.segment('mida') + brands.section()],
      ['agent', '◇', fila('prefs.providers.row', 'prefs.providers.help',
        '<button class="pf-open" id="prefsProv">' + T('btn.open') + '</button>') +
        fila('prefs.perms', 'prefs.perms.help', seg('perm', perm,
          [['ask', 'perm.ask'], ['edits', 'perm.edits'], ['full', 'perm.full']])) +
        // El revisor de permisos és avançat: fora del composer, aquí.
        fila('prefs.reviewer', 'prefs.reviewer.help',
          '<label class="pf-check"><input type="checkbox" id="prefsReviewer"' + (G().state && G().state.auto_review ? ' checked' : '') + '><span></span></label>') +
        // El revisor passava el model per sobre de CADA resposta i no hi
        // havia manera d'apagar-lo des de la finestra: només editant el
        // config a mà o des del TUI. Aquí és on el busques.
        fila('prefs.verify', 'prefs.verify.help',
          '<label class="pf-check"><input type="checkbox" id="prefsVerify"' + (this.verifyAuto() ? ' checked' : '') + '><span></span></label>') +
        '<h3 class="pf-sec">' + T('prefs.skills') + '</h3>' +
        '<p class="pf-skills-help">' + T('prefs.skills.help') + '</p>' +
        '<div id="prefsSkills"><div class="pf-skills-status">' + T('prefs.skills.loading') + '</div></div>'],
      ['plugins', '✚', '<div id="prefsPlugins">' + this.pluginsHTML() + '</div>'],
      ['voice', '🔊', voicePrefs.section()],
      ['extras', '✧', senseCap(funPrefs.section())],
      ['integrations', '⇄', chromeExt.section() + wordAddin.section() + shares.section()],
      ['advanced', '⋯', fila('prefs.shortcuts', 'prefs.shortcuts.help', '<button class="pf-open" id="prefsKeys">' + T('btn.open') + '</button>') +
        desktopUpdates.section()],
    ];
    cos.innerHTML =
      '<nav class="pf-nav" aria-label="' + T('prefs.title') + '">' +
      grups.map(([id, icona]) => '<button type="button" data-g="' + id + '"><span class="pf-nav-ic" aria-hidden="true">' + icona + '</span>' + T('prefs.group.' + id) + '</button>').join('') +
      '</nav><div class="pf-main" id="prefsMain">' +
      grups.map(([id, , dins]) => '<section class="pf-group" id="prefsG-' + id + '" data-g="' + id + '">' +
        '<header class="pf-ghead"><h3 class="pf-gtitle">' + T('prefs.group.' + id) + '</h3>' +
        '<p class="pf-gdesc">' + T('prefs.group.' + id + '.desc') + '</p></header>' +
        '<div class="pf-gbody">' + dins + '</div></section>').join('') +
      '<p class="pf-noresults" id="prefsNoResults" hidden>' + T('prefs.search.empty') + '</p></div>';

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
            this.repinta();
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
        if (k === 'tema') { pintaBarraTitol(); this.repinta(); }
      };
    });
    const v = $('prefsVerify');
    if (v) v.onchange = () => this.posaVerify(v.checked);
    const rv = $('prefsReviewer');
    if (rv) rv.onchange = () => this.posaRevisor(rv.checked);
    this.carregaSkills();
    const p = $('prefsProv');
    if (p) p.onclick = () => { this.tanca(); if (window.toggleProv) window.toggleProv(true); };
    const k = $('prefsKeys');
    if (k) k.onclick = () => { this.tanca(); const m = $('keysModal'); if (m) m.classList.add('on'); };
    this.lligaPlugins();
    voicePrefs.bind(cos);
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
      b.classList.toggle('on', !!actiu && b.dataset.g === actiu.dataset.g);
    });
  },

  // filtra amaga les files que no contenen el que busques (sense accents
  // ni majúscules). Un grup o subtítol que coincideix es mostra sencer.
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

  // ---- Plugins: Arena, Partits, Racó ----
  pluginsHTML() {
    const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
    if (!plugins.loaded()) return '<div class="pf-skills-status">' + esc(T('prefs.plugins.loading')) + '</div>';
    const list = plugins.list();
    if (!list.length) return '<div class="pf-skills-status">' + esc(T('prefs.plugins.none')) + '</div>';
    const admin = plugins.isAdmin();
    return (admin ? '' : '<p class="pf-skills-help">' + esc(T('prefs.plugins.adminOnly')) + '</p>') +
      list.map(p => {
        const nom = T('plugin.' + p.id + '.name', p.name);
        return '<div class="pf-row pf-plugin' + (p.enabled ? ' on' : '') + '" data-plugin="' + esc(p.id) + '">' +
          '<span class="pf-plugin-ic" aria-hidden="true">' + esc(p.icon) + '</span>' +
          '<div class="pf-lab"><b>' + esc(nom) + '</b><span>' + esc(T('plugin.' + p.id + '.desc', p.description)) + '</span></div>' +
          '<button class="pf-open" data-open="' + esc(p.view) + '"' + (p.enabled ? '' : ' disabled') + '>' + esc(T('prefs.plugins.open')) + '</button>' +
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
    this.pinta();
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
