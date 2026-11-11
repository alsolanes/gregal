// connexions: els comptes connectats de la persona (B5), a Preferències →
// Integracions. Compte, estat comprovat de debò, permisos (què pot fer
// l'eina i què hi autoritzes tu), connectar, tornar a iniciar sessió,
// desconnectar i treure. Mateixa API i mateixos textos que la pantalla de
// connexions de l'app Android (ConnexionsSheet.kt). Cap secret arriba aquí:
// només l'authRef opac.

import { idioma } from './i18n.js';

const L = (ca, en) => (idioma() === 'en' ? en : ca);
const el = (tag, cls, txt) => { const e = document.createElement(tag); if (cls) e.className = cls; if (txt != null) e.textContent = txt; return e; };
const boto = (label, fn, cls = '') => { const b = el('button', 'pf-open' + (cls ? ' ' + cls : ''), label); b.type = 'button'; b.onclick = fn; return b; };

async function api(path, body) {
  const opts = body === undefined ? {} : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) };
  const r = await window.api(path, opts);
  const txt = await r.text();
  let j = {};
  try { j = JSON.parse(txt); } catch (_) { /* text pla */ }
  if (!r.ok) { const e = new Error(j.error || txt.trim()); e.code = j.code; e.browserPath = j.browserPath; throw e; }
  return j;
}

// Textos compartits amb Android (connexioCopy a ConnexionsSheet.kt).
export function statusText(st) {
  return ({
    available: L('Disponible', 'Available'), connecting: L('Connectant…', 'Connecting…'), connected: L('Connectat', 'Connected'),
    needs_session: L('Cal tornar a iniciar sessió', 'Sign in again'), error: L('Error', 'Error'), disconnected: L('Desconnectat', 'Disconnected'),
  })[st] || st;
}

export function serviceText(c) {
  switch (c.service) {
    case 'mcp': return L('Connector', 'Connector') + (c.provider ? ' · ' + c.provider : '');
    case 'calendar-ics': return L('Calendari (iCal)', 'Calendar (iCal)');
    case 'mail-imap': return L('Correu (IMAP)', 'Email (IMAP)');
    case 'browser': return L("Sessió al navegador de l'ordinador", "Session on the computer's browser") + (c.provider ? ' · ' + c.provider : '');
  }
  return c.service || '';
}

// permsText: el que val de debò (capacitat ∩ autorització), en paraules.
export function permsText(c) {
  const eff = c.effective || [];
  const parts = [];
  if (eff.includes('read')) parts.push(L('pot llegir', 'can read'));
  if (eff.includes('prepare')) parts.push(L('pot preparar', 'can prepare'));
  if (eff.includes('send')) parts.push(L('pot actuar amb la teva confirmació', 'can act with your confirmation'));
  if (!parts.length) return L('en pausa: no pot fer res', 'paused: cannot do anything');
  const s = parts.join(', ');
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function quan(iso) {
  if (!iso) return L('encara no comprovat', 'not checked yet');
  const d = new Date(iso);
  return L('comprovat ', 'checked ') + d.toLocaleString(idioma(), { dateStyle: 'short', timeStyle: 'short' });
}

export const connexions = {
  section() { return '<div id="prefsConnexions" class="pf-conn"></div>'; },

  async bind(root, missatge = '') {
    const box = (root && root.querySelector && root.querySelector('#prefsConnexions')) || document.getElementById('prefsConnexions');
    if (!box) return;
    let data;
    try { data = await api('/api/connexions'); } catch (_) { box.hidden = true; return; }
    if (!box.isConnected) return;
    const msg = el('p', 'pf-skills-status'); msg.setAttribute('role', 'status');
    const diu = (text, dolent) => { msg.textContent = text || ''; msg.classList.toggle('error', !!dolent); };
    const torna = text => this.bind(root, text);
    const fes = async (fn, ok) => {
      diu(L('Un moment…', 'One moment…'));
      try { const j = await fn(); await torna(j && j.note ? j.note : ok); } catch (e) { diu(e.message, true); }
    };
    box.hidden = false; box.replaceChildren();
    box.append(el('h3', 'pf-sec', L('Comptes connectats', 'Connected accounts')));
    box.append(el('p', 'pf-skills-help', L(
      "Els comptes que els teus vents poden fer servir. L'estat és el de l'última comprovació real; tenir un compte connectat no autoritza totes les accions: tu tries què hi poden fer.",
      'The accounts your winds can use. The status is the last real check; having an account connected does not authorize every action: you choose what they may do.')));

    const list = data.connexions || [];
    if (!list.length) box.append(el('p', 'pf-skills-help', L('Encara no hi ha cap compte connectat.', 'No account is connected yet.')));
    const ul = el('ul', 'pf-list');
    for (const c of list) {
      const li = el('li');
      li.dataset.id = c.id;
      const main = el('div', 'pf-list-main');
      main.append(el('b', '', c.accountLabel || c.id));
      main.append(el('small', '', serviceText(c) + ' · ' + statusText(c.status) + ' · ' + quan(c.lastCheckedAt)));
      if (c.status !== 'disconnected') main.append(el('small', '', permsText(c)));
      if (c.error) main.append(el('small', 'pf-warn', c.error));
      const acts = el('div', 'pf-btns');
      if (c.service === 'mcp' && (c.status === 'needs_session' || c.status === 'disconnected' || c.status === 'error')) {
        acts.append(boto(L('Torna a connectar', 'Reconnect'), () => this.connecta({ id: c.id }, diu, torna)));
      }
      if (c.status !== 'disconnected') {
        acts.append(boto(L('Comprova', 'Check'), () => fes(() => api('/api/connexions/check', { id: c.id }), L('Comprovat.', 'Checked.'))));
        if ((c.capabilities || []).includes('send')) {
          const pot = (c.allowed || []).includes('send');
          acts.append(boto(pot ? L('Només llegir', 'Read only') : L('Deixa actuar', 'Allow actions'), () => fes(() => api('/api/connexions/update', {
            id: c.id, allowed: pot ? (c.allowed || []).filter(a => a !== 'send') : [...new Set([...(c.allowed || []), 'send'])],
          }), L('Permisos desats.', 'Permissions saved.'))));
        }
        acts.append(boto(L('Desconnecta', 'Disconnect'), () => fes(() => api('/api/connexions/disconnect', { id: c.id }), L('Desconnectat.', 'Disconnected.'))));
      }
      acts.append(boto(L('Treu', 'Remove'), () => fes(() => api('/api/connexions/remove', { id: c.id }), L('Tret.', 'Removed.'))));
      li.append(main, acts);
      ul.append(li);
    }
    if (list.length) box.append(ul);

    // Connectar un compte nou: connectors del servidor (OAuth) i el camí del navegador.
    const cat = data.catalogue || [];
    const add = el('div', 'pf-btns');
    for (const k of cat.filter(k => k.service === 'mcp')) {
      add.append(boto(L('Connecta un compte de ', 'Connect an account of ') + k.provider, () => this.connecta({ service: 'mcp', provider: k.provider }, diu, torna)));
    }
    const br = cat.find(k => k.service === 'browser');
    const help = el('p', 'pf-skills-help', br && br.available
      ? L("Per a llocs sense connector: demana-ho a un vent i inicia sessió tu mateix al navegador de l'ordinador; la sessió queda aquí i la propera feina la reutilitza.",
        "For sites without a connector: ask a wind and sign in yourself on the computer's browser; the session stays here and the next task reuses it.")
      : L("Per a llocs sense connector cal un ordinador amb visor; aquest servidor encara no en té.", 'Sites without a connector need a computer with a viewer; this server has none yet.'));
    box.append(add, help, msg);
    if (missatge) diu(missatge);
  },

  // connecta obre el consentiment en una finestra nova (oberta al clic perquè
  // el navegador no la bloquegi) i va mirant l'estat fins que acaba.
  async connecta(body, diu, torna) {
    const win = window.open('about:blank', 'gregal-connexio');
    diu(L('Obrint el consentiment…', 'Opening the consent page…'));
    try {
      const r = await api('/api/connexions/connect', body);
      if (win) win.location.href = r.authUrl; else window.location.href = r.authUrl;
      const id = r.connexio && r.connexio.id;
      for (let i = 0; i < 60; i++) {
        await new Promise(res => setTimeout(res, 3000));
        const d = await api('/api/connexions').catch(() => null);
        const c = d && (d.connexions || []).find(x => x.id === id);
        if (c && c.status !== 'connecting') {
          await torna(c.status === 'connected' ? L('Connectat com ', 'Connected as ') + c.accountLabel + '.' : (c.error || statusText(c.status)));
          return;
        }
      }
      await torna('');
    } catch (e) {
      if (win) win.close();
      diu(e.code === 'needs_client_id'
        ? L("Aquest proveïdor necessita que l'administrador hi registri Gregal (client_id). Mentrestant, inicia sessió al navegador de l'ordinador.",
          "This provider needs the administrator to register Gregal with it (client_id). Meanwhile, sign in on the computer's browser.")
        : e.message, true);
    }
  },
};
