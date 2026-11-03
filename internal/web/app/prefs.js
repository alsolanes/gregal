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
      ['general', '⚙', '<div id="prefsConn" hidden></div>' + fila('prefs.lang', 'prefs.lang.help', seg('idioma', idioma(), IDIOMES.map(([v, nom]) => [v, nom]))) +
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
      ['advanced', '⋯', '<div id="prefsPowers" hidden></div><div id="prefsAccounts" hidden></div>' + fila('prefs.shortcuts', 'prefs.shortcuts.help', '<button class="pf-open" id="prefsKeys">' + T('btn.open') + '</button>') +
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
    this.carregaAccounts();
    this.carregaPowers();
    this.carregaConnexions();
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

  // Calendari (Google Calendar per l'adreça iCal) i correu (Gmail per IMAP amb
  // contrasenya d'aplicació) de cada compte, i el resum del matí (connections.go).
  async carregaConnexions(status = '') {
    const box = $('prefsConn'); if (!box) return;
    let data, winds = [];
    try {
      const r = await window.api('/api/connections'); if (!r.ok || !box.isConnected) return;
      data = await r.json();
      const rw = await window.api('/api/winds'); if (rw.ok) winds = (await rw.json()).winds || [];
    } catch (_) { return; }
    if (!box.isConnected) return;
    const ca = idioma() === 'ca', text = (a, b) => ca ? a : b;
    const el = (tag, cls, txt) => { const e = document.createElement(tag); if (cls) e.className = cls; if (txt != null) e.textContent = txt; return e; };
    const input = (ph, type = 'text', val = '') => { const i = document.createElement('input'); i.type = type; i.placeholder = ph; i.value = val; i.setAttribute('aria-label', ph); i.autocomplete = 'off'; return i; };
    const btn = (label, fn) => { const b = el('button', 'pf-open', label); b.type = 'button'; b.onclick = fn; return b; };
    const msg = el('p', 'pf-skills-status'); msg.setAttribute('role', 'status');
    const post = async (body, ok) => {
      msg.textContent = text('Un moment…', 'One moment…');
      try {
        const r = await window.api('/api/connections', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
        if (!r.ok) throw new Error((await r.text()).trim());
        await this.carregaConnexions(ok);
      } catch (err) { msg.textContent = '⚠ ' + err.message; }
    };
    const link = (href, label) => { const a = el('a', '', label); a.href = href; a.target = '_blank'; a.rel = 'noopener'; return a; };
    box.hidden = false; box.replaceChildren();
    box.append(el('h3', 'pf-sec', text('Calendari i correu', 'Calendar and email')));
    box.append(el('p', 'pf-skills-help', text('Els teus vents poden mirar el teu Google Calendar i el teu Gmail (només llegir: mai no envien ni accepten res) i fer-te un resum cada matí. Cada compte té els seus.', 'Your winds can look at your Google Calendar and Gmail (read-only: they never send or accept anything) and give you a briefing every morning. Each account has its own.')));

    // Calendaris
    box.append(el('h4', '', 'Google Calendar'));
    const calHelp = el('p', 'pf-skills-help');
    calHelp.append(text('Al ', 'In '), link('https://calendar.google.com/calendar/r/settings', 'Google Calendar → ' + text('Configuració', 'Settings')), text(' → el teu calendari → «Integra el calendari» → copia l’«Adreça secreta en format iCal».', ' → your calendar → "Integrate calendar" → copy the "Secret address in iCal format".'));
    box.append(calHelp);
    const cals = el('ul', 'pf-tunnels');
    for (const c of data.calendars || []) {
      const li = el('li', '', c.name + ' · ' + c.url + ' ');
      li.append(btn(text('Treu', 'Remove'), () => post({ remove_calendar: c.name }, text('Calendari tret.', 'Calendar removed.'))));
      cals.append(li);
    }
    if ((data.calendars || []).length) box.append(cals);
    const calName = input(text('Nom (p. ex. Personal)', 'Name (e.g. Personal)')), calURL = input('https://calendar.google.com/calendar/ical/…/basic.ics');
    const calForm = el('div', 'pf-account-form'); calForm.append(calName, calURL, btn(text('Afegeix', 'Add'), () => post({ add_calendar: { name: calName.value, url: calURL.value } }, text('Calendari afegit.', 'Calendar added.'))));
    box.append(calForm);

    // Gmail
    box.append(el('h4', '', 'Gmail'));
    const mailHelp = el('p', 'pf-skills-help');
    mailHelp.append(text('Fes servir una ', 'Use an '), link('https://myaccount.google.com/apppasswords', text('contrasenya d’aplicació', 'app password')), text(' (cal la verificació en dos passos), no la teva contrasenya de Google.', ' (needs 2-Step Verification), not your Google password.'));
    box.append(mailHelp);
    const m = data.mail || {};
    const mailUser = input(text('el.teu@gmail.com', 'you@gmail.com'), 'email', m.user || '');
    const mailPass = input(m.has_password ? text('•••• (desada; deixa-ho buit per conservar-la)', '•••• (saved; leave empty to keep it)') : text('Contrasenya d’aplicació (16 lletres)', 'App password (16 letters)'), 'password');
    const mailForm = el('div', 'pf-account-form');
    mailForm.append(mailUser, mailPass, btn(m.has_password ? text('Desa', 'Save') : text('Connecta', 'Connect'), () => post({ mail: { host: m.host || 'imap.gmail.com', port: m.port || 993, user: mailUser.value, password: mailPass.value } }, text('Gmail connectat.', 'Gmail connected.'))));
    if (m.has_password) mailForm.append(btn(text('Desconnecta', 'Disconnect'), () => post({ remove_mail: true }, text('Gmail desconnectat.', 'Gmail disconnected.'))));
    box.append(mailForm);

    // Resum del matí
    box.append(el('h4', '', text('Resum del matí', 'Morning briefing')));
    const b = data.briefing || {};
    const row = el('div', 'pf-account-form');
    const on = document.createElement('input'); on.type = 'checkbox'; on.checked = !!b.enabled; on.setAttribute('aria-label', text('Resum del matí', 'Morning briefing'));
    const time = input('08:00', 'time', b.time || '08:00');
    const wsel = document.createElement('select'); wsel.setAttribute('aria-label', text('Qui el fa', 'Who gives it'));
    for (const w of winds) { const o = el('option', '', w.name); o.value = w.id; if (w.id === b.wind) o.selected = true; wsel.append(o); }
    if (!b.wind && winds.length) { const pref = winds.find(w => /organi/i.test(w.role || '')) || winds[0]; wsel.value = pref.id; }
    row.append(on, el('span', '', text('Cada dia a les', 'Every day at')), time, el('span', '', text('el fa', 'by')), wsel,
      btn(text('Desa', 'Save'), () => post({ briefing: { enabled: on.checked, time: time.value, wind: wsel.value } }, on.checked ? text('Resum programat.', 'Briefing scheduled.') : text('Resum desactivat.', 'Briefing off.'))));
    box.append(row);
    if (!winds.length) box.append(el('p', 'pf-skills-help', text('Primer crea un vent (o activa la colla del dia a dia).', 'Create a wind first (or start the everyday crew).')));

    const test = btn(text('Prova la connexió', 'Test the connection'), async () => {
      msg.textContent = text('Provant…', 'Testing…');
      try {
        const r = await window.api('/api/connections', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ action: 'test' }) });
        const j = await r.json(); const parts = [];
        if (j.calendar) parts.push(text('Calendari: ', 'Calendar: ') + j.calendar.events_next_7_days + text(' esdeveniments els pròxims 7 dies', ' events in the next 7 days') + ((j.calendar.problems || []).length ? ' ⚠ ' + j.calendar.problems.join('; ') : ''));
        if (j.mail) parts.push('Gmail: ' + (j.mail.error ? '⚠ ' + j.mail.error : j.mail.unread + text(' sense llegir', ' unread')));
        msg.textContent = parts.join(' · ') || text('No hi ha res connectat.', 'Nothing connected.');
      } catch (err) { msg.textContent = '⚠ ' + err.message; }
    });
    box.append(test, msg);
    if (status) msg.textContent = status;
  },

  // Poders de servidor (server_powers.go): només l'administrador els veu.
  async carregaPowers(success = "") {
    const box = $('prefsPowers'); if (!box) return;
    try {
      const response = await window.api('/api/server/powers');
      if (!response.ok || !box.isConnected) return;
      const data = await response.json(); if (!box.isConnected) return;
      const ca = idioma() === 'ca', text = (a, b) => ca ? a : b;
      const el = (tag, cls, txt) => { const e = document.createElement(tag); if (cls) e.className = cls; if (txt != null) e.textContent = txt; return e; };
      box.hidden = false; box.replaceChildren();
      box.append(el('h3', 'pf-sec', text('Poders de servidor', 'Server powers')));
      box.append(el('p', 'pf-skills-help', text(
        'Els teus vents fan de sysadmin en aquesta màquina, com un agent amb terminal: fan i canvien webs (també les que ja serveix), instal·len paquets, despleguen i obren túnels de Cloudflare. No poden fer servir sudo, aturar el Gregal ni tocar camins crítics. Mai per als altres comptes.',
        'Your winds act as sysadmins on this machine, like an agent with a terminal: they build and change websites (including the ones it already serves), install packages, deploy and open Cloudflare tunnels. They cannot use sudo, stop Gregal or touch critical paths. Never for other accounts.')));
      const row = el('label', 'pf-power-row');
      const cb = document.createElement('input'); cb.type = 'checkbox'; cb.checked = !!data.enabled;
      row.append(cb, el('span', '', text('Activa els poders de servidor', 'Enable server powers')));
      box.append(row);
      const status = el('p', 'pf-skills-status'); status.setAttribute('role', 'status');
      cb.onchange = async () => {
        cb.disabled = true;
        try {
          const r = await window.api('/api/server/powers', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ enabled: cb.checked }) });
          if (!r.ok) throw new Error(await r.text());
          status.textContent = cb.checked ? text('Activats: els teus vents ja hi poden treballar.', 'On: your winds can work on the server now.') : text('Desactivats.', 'Off.');
        } catch (err) { cb.checked = !cb.checked; status.textContent = text('No s’ha pogut desar: ', 'Could not save: ') + err.message; }
        finally { cb.disabled = false; }
      };
      box.append(el('p', 'pf-skills-help', text('A la màquina: ', 'On the machine: ') + (data.tools || []).join(', ') + ' · ' + (data.user || '') + ' · ' + (data.home || '')));
      const tunnels = data.tunnels || [];
      if (tunnels.length) {
        box.append(el('h4', '', text('Túnels oberts', 'Open tunnels')));
        const ul = el('ul', 'pf-tunnels');
        for (const t of tunnels) {
          const li = el('li'); const a = el('a', '', t.url || t.name); a.href = t.url || '#'; a.target = '_blank'; a.rel = 'noopener';
          li.append(a, document.createTextNode(' → ' + t.target + ' · ' + t.status)); ul.append(li);
        }
        box.append(ul);
      }
      box.append(el('h4', '', text('Notes del servidor', 'Server notes')));
      box.append(el('p', 'pf-skills-help', text('El que els vents saben de la màquina (webs, carpetes, dominis, com es despleguen). Les llegeixen a cada torn i les van posant al dia. Sense contrasenyes ni claus.', 'What the winds know about the machine (sites, folders, domains, how they deploy). They read them every turn and keep them up to date. No passwords or keys.')));
      const notes = document.createElement('textarea'); notes.className = 'pf-notes'; notes.rows = 8; notes.value = data.notes || '';
      notes.placeholder = text('- blog: /srv/www/blog, Caddy, blog.exemple.cat\n- túnel per defecte: casa', '- blog: /srv/www/blog, Caddy, blog.example.com\n- default tunnel: home');
      const save = el('button', 'pf-open', text('Desa les notes', 'Save notes')); save.type = 'button';
      save.onclick = async () => {
        save.disabled = true;
        try {
          const r = await window.api('/api/server/powers', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ notes: notes.value }) });
          if (!r.ok) throw new Error(await r.text());
          status.textContent = text('Notes desades.', 'Notes saved.');
        } catch (err) { status.textContent = text('No s’han pogut desar: ', 'Could not save: ') + err.message; }
        finally { save.disabled = false; }
      };
      box.append(notes, save, status);
      if (success) status.textContent = success;
    } catch (_) { /* Només l'administrador ho veu. */ }
  },

  async carregaAccounts(success = "") {
    const box = $('prefsAccounts'); if (!box) return;
    try {
      const response = await window.api('/api/server/accounts');
      if (!response.ok || !box.isConnected) return;
      const data = await response.json(); if (!box.isConnected) return;
      const ca = idioma() === 'ca', text = (a,b) => ca ? a : b;
      box.hidden = false; box.replaceChildren();
      if (success) { const confirmation = document.createElement('p'); confirmation.setAttribute('role','status'); confirmation.textContent = success; box.append(confirmation); }
      const title = document.createElement('h3'); title.className = 'pf-sec'; title.textContent = text('Comptes del servidor','Server accounts'); box.append(title);
      const hint = document.createElement('p'); hint.className = 'pf-skills-help'; hint.textContent = text('Crea un perfil amb usuari i contrasenya per a cada persona. Cada perfil té el seu espai.','Create a profile with a username and password for each person. Each profile has its own space.'); box.append(hint);
      let action='create';
      const list = document.createElement('ul'); for (const account of data.accounts) { const li = document.createElement('li'); li.textContent = account.name + (account.admin ? text(' · administrador',' · administrator') : ''); if(account.managed){ const reset=document.createElement('button'); reset.type='button'; reset.className='pf-open'; reset.textContent=text('Canvia la contrasenya','Change password'); reset.onclick=()=>{action='reset';name.value=account.name;name.readOnly=true;password.value='';password.focus();button.textContent=text('Desa la contrasenya','Save password');cancel.hidden=false;};
          const remove=document.createElement('button'); remove.type='button'; remove.className='pf-open'; remove.textContent=text('Esborra el compte','Delete account');
          remove.onclick=async()=>{ if(!confirm(text('Esborrar el compte «'+account.name+'»? Se’n tancaran les sessions i les seves dades s’apartaran (no s’esborren del disc).','Delete the account "'+account.name+'"? Its sessions will be closed and its data set aside (not erased from disk).'))) return;
            remove.disabled=true; try { const r=await window.api('/api/server/accounts',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({action:'delete',name:account.name})}); if(!r.ok) throw new Error(await r.text()); await this.carregaAccounts(text('Compte esborrat.','Account deleted.')); }
            catch(err){ remove.disabled=false; alert(text('No s’ha pogut esborrar: ','Could not delete: ')+err.message); } };
          // Fer-lo administrador: el mateix que l'admin del config (poders de servidor, comptes, proveïdors).
          const adminBtn=document.createElement('button'); adminBtn.type='button'; adminBtn.className='pf-open';
          adminBtn.textContent=account.admin ? text('Treu administrador','Remove administrator') : text('Fes-lo administrador','Make administrator');
          adminBtn.onclick=async()=>{ adminBtn.disabled=true; try { const r=await window.api('/api/server/accounts',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({action:'admin',name:account.name,admin:!account.admin})}); if(!r.ok) throw new Error(await r.text()); await this.carregaAccounts(account.admin ? text('Ja no és administrador.','No longer an administrator.') : text('Ara és administrador: té els mateixos permisos que tu.','Now an administrator: same permissions as you.')); }
            catch(err){ adminBtn.disabled=false; alert(err.message); } };
          li.append(' ',reset,' ',adminBtn,' ',remove); } list.append(li); } box.append(list);
      const inviteButton = document.createElement('button'); inviteButton.type='button'; inviteButton.className='pf-open'; inviteButton.textContent=text('Genera un codi d’invitació','Generate invitation code');
      const inviteResult = document.createElement('p'); inviteResult.setAttribute('role','status');
      inviteButton.onclick=async()=>{inviteButton.disabled=true; inviteResult.textContent=''; try {const r=await window.api('/api/server/invitations',{method:'POST'});if(!r.ok)throw new Error(await r.text());const result=await r.json();inviteResult.textContent=text('Codi d’un sol ús (caduca en 30 minuts): ','One-use code (expires in 30 minutes): ')+result.code;}catch(err){inviteResult.textContent=text('No s’ha pogut generar la invitació: ','Could not generate invitation: ')+err.message;}finally{inviteButton.disabled=false;}};
      box.append(inviteButton,inviteResult);
      const form = document.createElement('form'); form.className = 'pf-account-form';
      const name = document.createElement('input'); name.placeholder = text('Nom d’usuari','Username'); name.setAttribute('aria-label',name.placeholder); name.autocomplete = 'off'; name.required = true; name.pattern='[A-Za-z][A-Za-z0-9_-]{1,31}'; name.maxLength=32;
      const password = document.createElement('input'); password.type = 'password'; password.placeholder = text('Contrasenya (mínim 8 caràcters)','Password (at least 8 characters)'); password.setAttribute('aria-label',password.placeholder); password.autocomplete = 'new-password'; password.minLength=8; password.required=true;
      const button = document.createElement('button'); button.type='submit'; button.className='pf-open'; button.textContent=text('Crea el compte','Create account');
      const cancel=document.createElement('button'); cancel.type='button'; cancel.className='pf-open'; cancel.textContent=text('Cancel·la','Cancel'); cancel.hidden=true; cancel.onclick=()=>{action='create';name.readOnly=false;name.value='';password.value='';button.textContent=text('Crea el compte','Create account');cancel.hidden=true;};
      const message = document.createElement('p'); message.setAttribute('role','status'); form.append(name,password,button,cancel,message); box.append(form);
      form.onsubmit = async e => { e.preventDefault(); button.disabled=true; message.textContent=''; try {
        const r=await window.api('/api/server/accounts',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({action,name:name.value,password:password.value})}); if(!r.ok) throw new Error(await r.text()); password.value=''; await this.carregaAccounts(action === 'reset' ? text('Contrasenya actualitzada.','Password updated.') : text('Perfil creat. Ja pot iniciar sessió.','Profile created. They can now sign in.'));
      } catch(err) { message.textContent=text('No s’ha pogut crear el compte: ','Could not create account: ')+err.message; } finally { button.disabled=false; } };
    } catch (_) { /* Account management is available only on configured servers. */ }
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
