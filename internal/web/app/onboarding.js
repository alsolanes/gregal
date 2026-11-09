// Primer inici: connectar un model sense llegir cap document.
//
// Una instal·lació nova arrenca amb rols que apunten a un proveïdor remot
// sense clau (openai/gpt-4.1-mini). Abans, el peu deia «openai/gpt-4.1-mini»
// amb un punt verd i el primer missatge acabava amb el 401 en JSON del
// proveïdor. Ara, si el model actiu no pot respondre, surt un diàleg: tries
// el proveïdor, hi enganxes la clau, es prova i tots els rols passen a fer
// servir el model per defecte d'aquest proveïdor. El mateix diàleg s'obre des
// de Preferències → Models i agent i des de la paleta (Ctrl+K).
//
// Fa servir les accions que ja hi havia a /api/providers (add, key, test i
// preset); res de nou al servidor. Val per a Codi i per a Vents.
import { idioma, posa as posaIdioma, IDIOMES } from './i18n.js';

const G = () => window.gregal || {};
const api = (url, opts) => (G().api ? G().api(url, opts) : fetch(url, opts));
const L = (ca, en) => (idioma() === 'en' ? en : ca);
const esc = s => String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const icon = n => (window.gregalIcon && window.gregalIcon(n)) || '';
const LATER = 'gregal_ob_later';

// Els que surten al diàleg, en aquest ordre. La resta (Together, Cerebras,
// OpenAI Responses, endpoints propis) viuen a «Models i connexions».
const TRIA = [
  ['openrouter', ['Centenars de models amb una sola clau', 'Hundreds of models with one key'], 'https://openrouter.ai/keys'],
  ['anthropic', ['Els models Claude', 'The Claude models'], 'https://console.anthropic.com/settings/keys'],
  ['openai', ['Els models GPT', 'The GPT models'], 'https://platform.openai.com/api-keys'],
  ['deepseek', ['Models DeepSeek, a bon preu', 'DeepSeek models, low cost'], 'https://platform.deepseek.com/api_keys'],
  ['mistral', ['Models europeus de Mistral AI', 'European models by Mistral AI'], 'https://console.mistral.ai/api-keys'],
  ['groq', ['Respostes molt ràpides', 'Very fast answers'], 'https://console.groq.com/keys'],
  ['zen', ['El pla Go d’opencode.ai', 'The opencode.ai Go plan'], 'https://opencode.ai'],
];

const local = url => {
  try {
    const h = new URL(url).hostname.toLowerCase();
    return h === 'localhost' || h === '::1' || h === '[::1]' || h.startsWith('127.') || h.endsWith('.local') || h.endsWith('.localhost') ||
      h.startsWith('10.') || h.startsWith('192.168.') || /^172\.(1[6-9]|2\d|3[01])\./.test(h) || h === 'host.docker.internal';
  } catch (e) { return false; }
};

// Les frases del proveïdor, en una que es pugui entendre.
function humanErr(raw, label) {
  const m = String(raw || '').trim();
  const l = m.toLowerCase();
  if (/\b40[13]\b|unauthori[sz]ed|forbidden|invalid.*key|api key|incorrect/.test(l))
    return L('El proveïdor no accepta aquesta clau. Comprova que l’has copiada sencera i que el compte té crèdit.',
      'The provider does not accept this key. Check that you copied all of it and that the account has credit.');
  if (/no such host|dns|connection refused|network|unreachable|timeout|deadline|eof/.test(l))
    return L('No s’ha pogut arribar a ' + label + '. Comprova la connexió a internet i torna-ho a provar.',
      'Could not reach ' + label + '. Check your internet connection and try again.');
  if (/admin/.test(l) || /^403/.test(l))
    return L('Només l’administrador d’aquest servidor pot connectar models.', 'Only this server’s administrator can connect models.');
  return m || L('No s’ha pogut connectar.', 'Could not connect.');
}

async function post(body) {
  const r = await api('/api/providers', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  const text = (await r.text()).trim();
  if (!r.ok) {
    const err = new Error(r.status === 403 ? 'admin' : text || 'HTTP ' + r.status);
    err.status = r.status;
    throw err;
  }
  try { return JSON.parse(text).msg || ''; } catch (e) { return text; }
}

export const onboarding = {
  info: null,   // { needs, provider, model, label, admin }
  dlg: null,

  // status: què fa servir ara Gregal i si pot respondre. null si no es pot
  // saber (p. ex. un compte que no és administrador no veu els proveïdors).
  async status() {
    try {
      const [rs, rp, rg] = await Promise.all([api('/api/state'), api('/api/providers'), api('/api/plugins').catch(() => null)]);
      if (!rs.ok || !rp.ok) return (this.info = null);
      const st = await rs.json(), d = await rp.json();
      // Només l'administrador pot posar claus (/api/providers és adminWrites);
      // /api/plugins ho diu. Sense resposta, se suposa que sí (un sol usuari).
      let admin = true;
      try { if (rg && rg.ok) admin = (await rg.json()).admin !== false; } catch (e) {}
      const provs = d.providers || [], cat = d.catalog || [];
      const name = st.provider || (d.roles && d.roles.code && d.roles.code.provider) || '';
      const p = provs.find(x => x.name === name);
      const c = cat.find(x => x.name === name);
      const ok = !!p && (p.has_api_key || local(p.url));
      this.info = { needs: !ok, admin, provider: name, model: st.model_name || st.model || '', label: (c && c.label) || name, catalog: cat, providers: provs };
      return this.info;
    } catch (e) { return (this.info = null); }
  },

  async init() {
    const info = await this.status();
    // Qui no ho pot arreglar no ha de veure el diàleg: ho veurà a Preferències.
    if (!info || !info.needs || !info.admin) return;
    let later = false;
    try { later = sessionStorage.getItem(LATER) === '1'; } catch (e) {}
    if (later) return;
    // Mai damunt d'un altre diàleg (el de com es diuen els ajudants, a Vents).
    const prova = (n = 0) => {
      if (document.querySelector('dialog[open], #prefsModal.on') && n < 60) { setTimeout(() => prova(n + 1), 1000); return; }
      this.open();
    };
    prova();
  },

  // open(keep): keep = {prov, key} quan es torna a pintar (canvi d'idioma)
  // sense perdre el que ja havies triat.
  async open(keep) {
    if (this.dlg && this.dlg.open && !keep) return;
    if (!this.info) await this.status();
    const info = this.info || { catalog: [], providers: [] };
    const cat = info.catalog || [];
    const opcions = TRIA.map(([id, desc, keyURL]) => {
      const c = cat.find(x => x.name === id);
      return c ? { id, label: c.label, desc: L(desc[0], desc[1]), keyURL, url: c.url, model: c.default_model } : null;
    }).filter(Boolean);
    const triat = (keep && keep.prov === 'local') ? null : opcions.find(o => o.id === (keep && keep.prov)) || opcions.find(o => o.id === info.provider) || opcions[0];

    const d = this.dlg || document.createElement('dialog');
    d.className = 'ob';
    d.id = 'gregalOnboarding';
    d.setAttribute('aria-labelledby', 'obTitle');
    // L'idioma, aquí mateix: una instal·lació nova arrenca en anglès (el
    // valor per defecte del config) i és la primera cosa que vols canviar.
    const llengua = '<div class="ob-lang" role="group" aria-label="' + esc(L('Idioma', 'Language')) + '">' +
      IDIOMES.map(([v, nom]) => '<button type="button" data-lang="' + v + '" aria-pressed="' + (v === idioma()) + '"' + (v === idioma() ? ' class="on"' : '') + '>' + esc(nom) + '</button>').join('') + '</div>';
    // Des de Preferències, amb un model que ja respon, és un canvi de proveïdor.
    const canvi = !!(this.info && !this.info.needs);
    const titol = canvi ? L('Canvia de proveïdor', 'Switch provider') : L('Comencem: connecta un model', 'Let’s start: connect a model');
    const entrada = canvi
      ? L('Tria el proveïdor i enganxa-hi la clau d’API. Tots els rols passaran a fer servir el seu model per defecte; després ho pots afinar a «Models i connexions».',
        'Pick the provider and paste its API key. Every role will switch to its default model; you can fine-tune it later in “Models & connections”.')
      : L('Gregal necessita un model d’IA per respondre. Tria un proveïdor i enganxa-hi la teva clau d’API; només s’envia a aquest proveïdor.',
        'Gregal needs an AI model to answer. Pick a provider and paste your API key; it is only sent to that provider.');
    d.innerHTML =
      '<form class="ob-card" method="dialog" novalidate>' +
      '<header class="ob-head"><span class="ob-mark" aria-hidden="true">' + icon(canvi ? 'model' : 'sparkles') + '</span>' +
      '<div><h2 id="obTitle">' + esc(titol) + '</h2>' +
      '<p>' + esc(entrada) + '</p></div></header>' +
      '<fieldset class="ob-grid"><legend>' + esc(L('Proveïdor', 'Provider')) + '</legend>' +
      opcions.map(o => '<label class="ob-opt"><input type="radio" name="obProv" value="' + esc(o.id) + '"' + (o === triat ? ' checked' : '') + '>' +
        '<span><b>' + esc(o.label) + '</b><small>' + esc(o.desc) + '</small></span></label>').join('') +
      '<label class="ob-opt"><input type="radio" name="obProv" value="local"' + (triat ? '' : ' checked') + '><span><b>' + esc(L('Model local', 'Local model')) + '</b><small>' +
        esc(L('Ollama, LM Studio o llama.cpp, sense clau', 'Ollama, LM Studio or llama.cpp, no key')) + '</small></span></label>' +
      '</fieldset>' +
      '<div class="ob-key">' +
      '<label for="obKey" id="obKeyLabel"></label>' +
      '<div class="ob-key-row"><input id="obKey" type="password" autocomplete="off" spellcheck="false">' +
      '<a id="obKeyLink" target="_blank" rel="noopener">' + esc(L('Aconsegueix-ne una', 'Get one')) + icon('external') + '</a></div>' +
      '</div>' +
      '<div class="ob-local" hidden><p>' + esc(L('Engega el servidor local (per exemple, ollama serve) i tria quin model fa servir cada rol a «Models i connexions».',
        'Start the local server (for example, ollama serve) and choose the model for each role in “Models & connections”.')) + '</p></div>' +
      '<p class="ob-status" id="obStatus" role="status" aria-live="polite"></p>' +
      '<footer class="ob-foot">' + (canvi ? '' : llengua) + '<button type="button" class="ob-ghost" id="obLater">' + esc(canvi ? L('Cancel·la', 'Cancel') : L('Ara no', 'Not now')) + '</button>' +
      '<button type="submit" class="ob-primary" id="obGo">' + esc(L('Connecta', 'Connect')) + '</button></footer>' +
      '</form>';
    if (!d.isConnected) document.body.append(d);
    this.dlg = d;

    const $ = s => d.querySelector(s);
    const keyBox = $('.ob-key'), localBox = $('.ob-local'), key = $('#obKey'), go = $('#obGo'), status = $('#obStatus');
    const say = (t, bad) => { status.textContent = t || ''; status.classList.toggle('bad', !!bad); };
    const actual = () => d.querySelector('input[name=obProv]:checked')?.value;
    const pinta = () => {
      const id = actual(), o = opcions.find(x => x.id === id);
      keyBox.hidden = !o; localBox.hidden = !!o;
      go.textContent = o ? L('Connecta', 'Connect') : L('Obre Models i connexions', 'Open Models & connections');
      if (o) {
        $('#obKeyLabel').textContent = L('Clau d’API de ', 'API key for ') + o.label;
        // Un proveïdor que ja té clau es pot triar sense tornar-la a enganxar.
        const desada = (info.providers || []).some(p => p.name === o.id && p.has_api_key);
        key.placeholder = desada ? L('Ja n’hi ha una de desada (deixa-ho buit)', 'One is saved (leave empty to keep it)') : L('Enganxa-la aquí', 'Paste it here');
        const a = $('#obKeyLink'); a.href = o.keyURL; a.hidden = !o.keyURL;
      }
      say('');
    };
    d.querySelectorAll('input[name=obProv]').forEach(r => { r.onchange = () => { pinta(); if (!keyBox.hidden) key.focus(); }; });
    pinta();
    if (keep && keep.key) key.value = keep.key;
    d.querySelectorAll('.ob-lang button').forEach(b => {
      b.onclick = async () => {
        if (b.dataset.lang === idioma()) return;
        try {
          await posaIdioma(b.dataset.lang, G().api);
          if (G().refresh) await G().refresh();
        } catch (err) { say(err.message, true); return; }
        this.open({ prov: actual(), key: key.value });
        d.querySelector('.ob-lang button.on')?.focus();
      };
    });

    const later = () => { try { sessionStorage.setItem(LATER, '1'); } catch (e) {} d.close(); };
    $('#obLater').onclick = later;
    d.oncancel = e => { e.preventDefault(); later(); };
    d.onclose = () => { if (this.tornaFocus && this.tornaFocus.isConnected) this.tornaFocus.focus(); };

    $('form').onsubmit = async e => {
      e.preventDefault();
      const o = opcions.find(x => x.id === actual());
      if (!o) { d.close(); if (window.toggleProv) window.toggleProv(true); return; }
      const k = key.value.trim();
      const abans = (info.providers || []).find(p => p.name === o.id);
      if (!k && !(abans && abans.has_api_key)) { say(L('Enganxa la clau abans de connectar.', 'Paste the key before connecting.'), true); key.focus(); return; }
      go.disabled = true; key.disabled = true;
      d.querySelectorAll('input[name=obProv]').forEach(r => { r.disabled = true; });
      try {
        if (k) {
          say(L('Desant la clau…', 'Saving the key…'));
          if (abans) await post({ action: 'key', name: o.id, key: k });
          else await post({ action: 'add', name: o.id, url: o.url, key: k });
        }
        say(L('Provant la connexió amb ', 'Testing the connection to ') + o.label + '…');
        try { await post({ action: 'test', name: o.id }); }
        catch (err) {
          // Una clau que no funciona no es queda posada (si abans no n'hi
          // havia cap): el proveïdor tornaria a semblar configurat.
          if (abans && !abans.has_api_key) await post({ action: 'key', name: o.id, key: '-' }).catch(() => {});
          else if (!abans) await post({ action: 'rm', name: o.id }).catch(() => {});
          const h = new Error(humanErr(err.message, o.label)); h.human = true; throw h;
        }
        say(L('Triant el model…', 'Choosing the model…'));
        try { await post({ action: 'preset', name: o.id }); }
        catch (err) {
          // Un endpoint propi amb el mateix nom: es posen els rols un per un.
          for (const role of ['chat', 'think', 'code', 'reviewer']) await post({ action: 'rol', role, provider: o.id, model: o.model }).catch(() => {});
        }
        try { sessionStorage.removeItem(LATER); } catch (e2) {}
        if (G().refresh) await G().refresh();
        document.dispatchEvent(new CustomEvent('gregal:model-ready', { detail: { provider: o.id, model: o.model } }));
        await this.status();
        this.fet(o);
      } catch (err) {
        say(err.human ? err.message : humanErr(err.message, o.label), true);
        go.disabled = false; key.disabled = false;
        d.querySelectorAll('input[name=obProv]').forEach(r => { r.disabled = false; });
        key.focus();
      }
    };

    if (keep) return;
    this.tornaFocus = document.activeElement;
    if (!d.open) d.showModal();
    (keyBox.hidden ? go : key).focus();
  },

  // Tot a punt: què farà servir i on es canvia.
  fet(o) {
    const d = this.dlg;
    if (!d) return;
    const model = (this.info && this.info.provider === o.id && this.info.model) || o.model;
    d.innerHTML = '<div class="ob-card ob-done">' +
      '<span class="ob-ok" aria-hidden="true">' + icon('check') + '</span>' +
      '<h2 id="obTitle">' + esc(L('Tot a punt', 'All set')) + '</h2>' +
      '<p>' + esc(L('Gregal farà servir ' + model + ' de ' + o.label + '. Ho pots canviar quan vulguis a Preferències → Models i agent.',
        'Gregal will use ' + model + ' from ' + o.label + '. You can change it any time in Preferences → Models & agent.')) + '</p>' +
      '<footer class="ob-foot"><button type="button" class="ob-primary" id="obStart">' + esc(L('Comença', 'Start')) + '</button></footer></div>';
    const start = d.querySelector('#obStart');
    start.onclick = () => {
      // El focus va al camp de text, no on era abans d'obrir el diàleg.
      this.tornaFocus = null;
      d.close();
      const input = document.getElementById(window.gregalProduct === 'vents' ? 'windChatInput' : 'in');
      if (input && input.offsetParent) input.focus();
    };
    start.focus();
  },
};

window.gregalOnboarding = onboarding;
// L'estil va a part: el diàleg es pinta tard (després de preguntar al
// servidor), i així index.html no ha de saber res d'aquest fitxer.
(() => {
  if (document.querySelector('link[href="/app/onboarding.css"]')) return;
  const l = document.createElement('link');
  l.rel = 'stylesheet'; l.href = '/app/onboarding.css';
  document.head.appendChild(l);
})();
