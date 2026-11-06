// Com es diuen els ajudants (a la web).
//
// La persona pot dir-los d'una altra manera que «vents»: fullets, minairons,
// pirates… El nom viu al servidor (/api/winds/collective, compartit amb el
// mòbil) i aquí se'n guarda una còpia perquè la pàgina surti bé des del primer
// moment. Els textos de la interfície que diuen «vent/vents» (o «wind/winds»)
// el fan servir, menys la marca «Gregal Vents». El primer cop es pregunta; amb
// «Ara no» no es torna a preguntar i es pot canviar a Preferències.

const G = () => window.gregal || {};
const api = (p, o) => (G().api ? G().api(p, o) : fetch(p, o));
const T = (k, alt) => (window.gregalT ? window.gregalT(k) : '') || alt || k;
const esc = v => String(v == null ? '' : v).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
const KEY = 'gregal_collective';
const lang = () => (document.documentElement.lang === 'en' ? 'en' : 'ca');

function readLocal() {
  try { return JSON.parse(localStorage.getItem(KEY) || 'null') || { singular: '', plural: '', asked: false }; }
  catch (e) { return { singular: '', plural: '', asked: false }; }
}

let cur = readLocal();

/** Opcions proposades: la primera és la de sempre (es desa buida). */
export function options(l) {
  return l === 'en' ? [
    ['Wind', 'Winds', 'Zephyr, Gust, Squall…'], ['Sprite', 'Sprites', 'Acorn, Moss, Bramble…'],
    ['Helper', 'Helpers', 'Tinker, Pinch, Spool…'], ['Spirit', 'Spirits', 'Mist, Ember, Echo…'],
  ] : [
    ['Vent', 'Vents', 'Garbinada, Terral, Embat…'], ['Fullet', 'Fullets', 'Bolet, Rovelló, Molsa…'],
    ['Minairó', 'Minairons', 'Pessic, Picarol, Didal…'], ['Esperit', 'Esperits', 'Boira, Guspira, Eco…'],
  ];
}

/** Un a partir de molts (com a l'Android): «pirates» → «pirata» / «pirate». */
export function singularOf(plural, l) {
  const p = String(plural || '').trim(), low = p.toLowerCase();
  if (l === 'en') {
    if (low.length > 4 && low.endsWith('ies')) return p.slice(0, -3) + 'y';
    if (low.length > 4 && /(ches|shes|xes|sses)$/.test(low)) return p.slice(0, -2);
    if (low.length > 3 && low.endsWith('ves')) return p.slice(0, -3) + 'f';
    if (low.length > 2 && low.endsWith('s') && !low.endsWith('ss')) return p.slice(0, -1);
    return p;
  }
  if (low.length > 4 && low.endsWith('ons')) return p.slice(0, -3) + 'ó';
  if (low.length > 4 && low.endsWith('ies') && !low.endsWith('eies')) return p.slice(0, -3) + 'y';
  if (low.length > 3 && low.endsWith('es')) return p.slice(0, -2) + 'a';
  if (low.length > 2 && low.endsWith('s')) return p.slice(0, -1);
  return p;
}

const keepCase = (orig, word) => {
  if (!word) return orig;
  return orig[0] === orig[0].toUpperCase() ? word[0].toUpperCase() + word.slice(1) : word[0].toLowerCase() + word.slice(1);
};

/**
 * Posa el nom triat on el text diu vent/vents (wind/winds). Buit = res a fer.
 * «Gregal Vents» és la marca i no es toca.
 */
export function apply(text, c = cur) {
  if (text == null || !c || !c.plural) return text;
  const s = String(text);
  const sing = c.singular || singularOf(c.plural, lang());
  return s
    .replace(/(?<!Gregal )\b(Vents|vents|Winds|winds)\b/g, m => keepCase(m, c.plural))
    .replace(/(?<!Gregal )\b(Vent|vent|Wind|wind)\b/g, m => keepCase(m, sing));
}

export const collective = {
  get: () => ({ ...cur }),
  apply,
  /** Llegeix el servidor; si encara no s'ha preguntat, pregunta. */
  async refresh({ ask = true } = {}) {
    let server;
    try {
      const r = await api('/api/winds/collective');
      if (!r.ok) return;
      server = await r.json();
    } catch (e) { return; }
    const changed = server.plural !== cur.plural || server.singular !== cur.singular;
    cur = { singular: server.singular || '', plural: server.plural || '', asked: !!server.asked };
    try { localStorage.setItem(KEY, JSON.stringify(cur)); } catch (e) {}
    if (changed) location.reload();
    else if (ask && !cur.asked) this.ask();
  },
  async save(singular, plural) {
    const r = await api('/api/winds/collective', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ singular, plural }) });
    if (!r.ok) throw new Error(String(r.status));
    const changed = plural !== cur.plural || singular !== cur.singular;
    cur = { singular, plural, asked: true };
    try { localStorage.setItem(KEY, JSON.stringify(cur)); } catch (e) {}
    // Tots els textos de la pàgina (també els que pinta cada mòdul) amb el nom nou.
    if (changed) location.reload();
  },
  /** Noms per a ajudants nous al gust del col·lectiu (buit si el servidor no en té). */
  async names(n, l = lang()) {
    try {
      const r = await api('/api/winds/names?n=' + n + '&lang=' + l);
      if (!r.ok) return [];
      return ((await r.json()).names || []).filter(Boolean);
    } catch (e) { return []; }
  },
  /** El diàleg del primer cop. */
  ask() {
    if (document.getElementById('collectiveDialog')) return;
    // Només a Vents (a Codi no hi pinta res) i mai damunt d'un altre diàleg o
    // d'una aprovació pendent: es torna a provar d'aquí a una estona.
    if (window.gregalProduct !== 'vents') return;
    // Tampoc mentre l'agent treballa o hi ha una targeta de permís/pregunta
    // esperant: el diàleg modal la deixaria inert mentre corre el compte enrere.
    const ocupat = window.gregal?.isBusy?.() ||
      document.querySelector('.approve:not(.resolved), .qcard .qopt, .qcard .qfree input');
    if (document.querySelector('dialog[open]') || ocupat) { setTimeout(() => this.ask(), 5000); return; }
    const l = lang(), opts = options(l);
    const d = document.createElement('dialog');
    d.id = 'collectiveDialog';
    d.className = 'collective-dialog';
    d.setAttribute('aria-labelledby', 'collectiveTitle');
    d.innerHTML = '<form method="dialog" class="collective-form">' +
      '<h2 id="collectiveTitle">' + esc(T('collective.title')) + '</h2>' +
      '<p class="collective-help">' + esc(T('collective.help')) + '</p>' +
      opts.map(([, pl, hint], i) => '<label class="collective-opt"><input type="radio" name="collective" value="' + i + '"' + (i === 0 ? ' checked' : '') + '>' +
        '<span><b>' + esc(pl) + '</b><small>' + esc(hint) + '</small></span></label>').join('') +
      '<label class="collective-opt"><input type="radio" name="collective" value="other"><span><b>' + esc(T('collective.other')) + '</b><small>' + esc(T('collective.otherHint')) + '</small></span></label>' +
      '<div class="collective-custom" hidden>' +
      '<label>' + esc(T('collective.plural')) + '<input id="collectivePlural" maxlength="20" autocomplete="off"></label>' +
      '<label>' + esc(T('collective.singular')) + '<input id="collectiveSingular" maxlength="20" autocomplete="off"></label></div>' +
      '<p class="collective-error" role="alert" hidden></p>' +
      '<div class="collective-actions"><button type="button" class="ghost" id="collectiveLater">' + esc(T('collective.laterButton')) + '</button>' +
      '<button type="submit" class="ghost winds-primary" id="collectiveOk">' + esc(T('collective.ok')) + '</button></div></form>';
    document.body.append(d);
    const custom = d.querySelector('.collective-custom'), plural = d.querySelector('#collectivePlural'), singular = d.querySelector('#collectiveSingular');
    let singularEdited = false;
    d.querySelectorAll('input[name=collective]').forEach(r => { r.onchange = () => { custom.hidden = r.value !== 'other'; if (!custom.hidden) plural.focus(); }; });
    plural.oninput = () => { if (!singularEdited) singular.value = singularOf(plural.value, l); };
    singular.oninput = () => { singularEdited = true; };
    const close = () => { d.close(); d.remove(); };
    d.querySelector('#collectiveLater').onclick = async () => {
      close();
      try { await this.save(cur.singular, cur.plural); } catch (e) {}
      toast(T('collective.later'));
    };
    d.addEventListener('cancel', e => { e.preventDefault(); d.querySelector('#collectiveLater').click(); });
    d.querySelector('form').onsubmit = async e => {
      e.preventDefault();
      const v = d.querySelector('input[name=collective]:checked')?.value;
      let sing = '', plur = '';
      if (v === 'other') {
        plur = plural.value.trim().toLowerCase();
        if (!plur) { plural.focus(); return; }
        sing = (singular.value.trim() || singularOf(plur, l)).toLowerCase();
      } else if (v !== '0') {
        const o = opts[Number(v)];
        sing = o[0].toLowerCase(); plur = o[1].toLowerCase();
      }
      try { await this.save(sing, plur); close(); }
      catch (err) { const m = d.querySelector('.collective-error'); m.hidden = false; m.textContent = T('collective.failed'); }
    };
    d.showModal();
  },
  /** Fila de Preferències. */
  section() {
    return '<div id="prefsCollective" class="pf-row"><div class="pf-lab"><b>' + esc(T('collective.prefs')) + '</b><span>' + esc(T('collective.prefsHelp')) + '</span></div>' +
      '<div class="collective-prefs"><input id="prefsCollPlural" maxlength="20" placeholder="' + esc(T('collective.plural')) + '" value="' + esc(cur.plural) + '">' +
      '<input id="prefsCollSingular" maxlength="20" placeholder="' + esc(T('collective.singular')) + '" value="' + esc(cur.singular) + '">' +
      '<button class="pf-open" id="prefsCollSave">' + esc(T('collective.save')) + '</button>' +
      '<button class="pf-open" id="prefsCollReset">' + esc(T('collective.reset')) + '</button></div></div>';
  },
  bind(cos) {
    const box = cos && cos.querySelector('#prefsCollective');
    if (!box) return;
    const pl = box.querySelector('#prefsCollPlural'), sg = box.querySelector('#prefsCollSingular');
    pl.oninput = () => { sg.placeholder = singularOf(pl.value, lang()) || T('collective.singular'); };
    box.querySelector('#prefsCollSave').onclick = () => {
      const plur = pl.value.trim().toLowerCase();
      this.save(plur ? (sg.value.trim() || singularOf(plur, lang())).toLowerCase() : '', plur).catch(() => toast(T('collective.failed')));
    };
    box.querySelector('#prefsCollReset').onclick = () => this.save('', '').catch(() => toast(T('collective.failed')));
  },
};

function toast(msg) {
  if (G().toast) { G().toast(msg); return; }
  const n = document.createElement('div');
  n.className = 'collective-toast';
  n.setAttribute('role', 'status');
  n.textContent = msg;
  document.body.append(n);
  setTimeout(() => n.remove(), 6000);
}

window.gregalCollective = collective;
