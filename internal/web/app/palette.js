// Paleta d'ordres (Ctrl/Cmd+K), a l'estil de Linear o Raycast: una sola
// caixa per fer accions (sessió nova, mode, model, tema, preferències…),
// obrir una conversa recent o un projecte. Les converses i els projectes
// són els que ja ha carregat la barra lateral (convs.js), sense cap crida
// nova al servidor.
(function () {
  'use strict';
  const $ = id => document.getElementById(id);
  const T = (k, sino) => (window.gregalT ? window.gregalT(k) : sino || k);
  const G = () => window.gregal || {};
  const P = () => window.gregalPanels || {};
  const mac = /Mac|iPhone|iPad/.test(navigator.platform || '');
  const kbd = k => (mac ? '⌘' : 'Ctrl+') + k;
  const producte = () => window.gregalProduct || 'codi';
  const esc = s => String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const icona = nom => (window.gregalIcon && window.gregalIcon(nom)) || '';
  // Textos nous de la paleta (sense passar per i18n.js).
  const L = (ca, en) => ((document.documentElement.lang || 'ca').startsWith('en') ? en : ca);

  // El CSS va a part; s'enganxa aquí perquè l'index només necessiti una línia.
  const css = document.createElement('link');
  css.rel = 'stylesheet'; css.href = '/app/palette.css';
  document.head.appendChild(css);

  let root = null, input = null, list = null, items = [], sel = 0, abans = null, fitxers = [];

  // Coincidència difusa: les lletres de la cerca, en ordre. Puntua més les
  // seguides i les que comencen paraula; -1 si no hi són totes.
  function puntua(text, q) {
    if (!q) return 0;
    const t = String(text || '').toLowerCase();
    const directe = t.indexOf(q);
    if (directe >= 0) return 1000 - directe + (directe === 0 || /[\s/·._-]/.test(t[directe - 1]) ? 200 : 0);
    let s = 0, i = 0, seguit = 0;
    for (const c of q) {
      const j = t.indexOf(c, i);
      if (j < 0) return -1;
      seguit = j === i ? seguit + 1 : 0;
      s += 1 + seguit * 3 + (j === 0 || /[\s/·._-]/.test(t[j - 1]) ? 4 : 0) - Math.min(j - i, 6) * 0.3;
      i = j + 1;
    }
    return s;
  }

  function accions() {
    const g = G(), a = [];
    const tema = document.documentElement.getAttribute('data-theme') === 'light' ? 'dark' : 'light';
    if (producte() === 'codi') {
      a.push({ label: T('pal.newSession'), icon: 'plus', key: kbd('N'), run: () => g.newChat && g.newChat() });
      // ui_mode és el que mostra el selector (act/plan/chat); mode pot ser
      // «code» o «autonomous» i no coincidia mai amb cap opció.
      const mode = (g.state && (g.state.ui_mode || g.state.mode)) || '';
      for (const [m, k] of [['chat', 'mode.ask'], ['plan', 'mode.plan'], ['act', 'mode.act']]) {
        const actual = mode === m || (m === 'act' && mode === 'code') || (m === 'chat' && mode === 'ask');
        a.push({ label: T('pal.mode').replace('{mode}', T(k)), icon: actual ? 'check' : 'agent', hint: actual ? L('actual', 'current') : '', run: () => { if (!actual && g.setMode) { g.setView && g.setView('agent'); g.setMode(m); } } });
      }
      a.push({ label: T('pal.model'), icon: 'model', run: () => { g.setView && g.setView('agent'); const b = $('modelPill'); if (b) b.click(); } });
      a.push({ label: T('pal.scheduled'), icon: 'programat', run: () => g.setView && g.setView('programat') });
      a.push({ label: T('product.openVents'), icon: 'vents', run: () => { location.href = '/vents/'; } });
    } else if (producte() === 'vents') {
      a.push({ label: T('nav.winds'), icon: 'vents', run: () => g.setView && g.setView('vents') });
      a.push({ label: T('nav.corner'), icon: 'objectius', run: () => g.setView && g.setView('raco') });
      a.push({ label: T('nav.automations'), icon: 'automatitzacions', run: () => g.setView && g.setView('automatitzacions') });
      a.push({ label: T('product.openCodi'), icon: 'agent', run: () => { location.href = '/'; } });
    }
    if (producte() !== 'jocs') {
      a.push({ label: T('nav.memory'), icon: 'knowledge', run: () => g.setView && g.setView('memoria') });
      a.push({ label: T('library.title'), icon: 'publicacions', run: () => g.setView && g.setView('publicacions') });
    }
    // Notes a mà (notes-quick.js): obrir-les o començar-ne una.
    const qn = window.gregalQuickNotes;
    if (qn) {
      a.push({ label: L('Nota nova', 'New note'), icon: 'note', key: kbd(mac ? '⌥N' : 'Alt+N'), run: () => qn.newNote() });
      a.push({ label: L('Obre les notes', 'Open notes'), icon: 'note', run: () => qn.open() });
    }
    // Sense model que respongui (primer inici), connectar-ne un és el primer.
    const ob = window.gregalOnboarding;
    if (ob && ob.info && ob.info.needs) a.unshift({ label: L('Connecta un model', 'Connect a model'), icon: 'key', hint: L('cal per començar', 'needed to start'), run: () => ob.open() });
    a.push({ label: L('Models i connexions', 'Models & connections'), icon: 'model', run: () => { if (window.toggleProv) window.toggleProv(true); else if (ob) ob.open(); } });
    a.push({ label: T('pal.theme'), icon: tema === 'light' ? 'sun' : 'moon', run: () => posaTema(tema) });
    a.push({ label: T('pal.settings'), icon: 'sliders', key: kbd(','), run: () => { const p = P().prefs; if (p && p.obre) p.obre(); else { const b = $('prefsBtn'); if (b) b.click(); } } });
    a.push({ label: L('Dreceres de teclat', 'Keyboard shortcuts'), icon: 'keyboard', run: () => { const p = P().prefs; if (p && p.obreDreceres) p.obreDreceres(); else { const m = $('keysModal'); if (m) m.classList.add('on'); } } });
    a.push({ label: T('pal.sidebar'), icon: 'menu', run: () => { const b = $('menu'); if (b) b.click(); } });
    return a.map(x => ({ ...x, group: 'actions' }));
  }

  // Mateix camí que Preferències → Tema: es desa i es pinta a l'arrel.
  function posaTema(v) {
    try { localStorage.setItem('gregal_tema', v); } catch (e) {}
    document.documentElement.setAttribute('data-theme', v);
    document.dispatchEvent(new CustomEvent('gregal:titlebar'));
  }

  const norm = p => String(p || '').replace(/[\\/]+$/, '').toLowerCase();
  const nomProjecte = p => p.name || String(p.path || '').replace(/[\\/]+$/, '').split(/[\\/]/).pop();

  function converses() {
    if (producte() !== 'codi') return [];
    const c = P().convs;
    if (!c || !Array.isArray(c.items)) return [];
    return c.items.filter(x => x && !x.missing).map(x => ({
      group: 'convs', icon: 'activitat', label: x.title || x.name, hint: x.project || '',
      extra: x.workspace || '', run: () => { G().setView && G().setView('agent'); c.obre(x.name); },
    }));
  }

  function projectes() {
    if (producte() !== 'codi') return [];
    const c = P().convs;
    if (!c || !Array.isArray(c.projects)) return [];
    return c.projects.filter(p => p && p.path).map(p => ({
      group: 'projects', icon: 'fitxers', label: nomProjecte(p), hint: p.path,
      run: async () => {
        // Com el clic a la barra: la conversa més recent del projecte o una de nova.
        const darrera = (c.items || []).find(x => norm(x.workspace) === norm(p.path) && !x.missing);
        if (darrera) await c.obre(darrera.name); else await c.newProjectConversation(p.path);
      },
    }));
  }

  // Fitxers del projecte (la mateixa llista que l'autocompleció de @), com
  // el Ctrl+P de Cursor: «store.py» o «store.py:42» l'obre a la línia.
  function fitxersDe(linia) {
    if (producte() !== 'codi') return [];
    return fitxers.map(p => {
      const i = p.lastIndexOf('/');
      return { group: 'files', icon: 'fitxers', label: p.slice(i + 1), hint: i > 0 ? p.slice(0, i) : '', extra: p,
        run: () => { const g = G(); g.setView && g.setView('agent'); if (g.openFileRef) g.openFileRef(p + (linia ? ':' + linia : '')); } };
    });
  }

  function filtra(q) {
    q = q.trim().toLowerCase();
    const ln = q.match(/^(.+?):(\d+)$/);
    const qf = ln ? ln[1] : q;
    // Sense cerca, primer el que hi ha més a mà (les converses recents);
    // amb cerca, les accions manen i després la resta.
    const conv = ['convs', converses(), q ? 8 : 5], acc = ['actions', accions(), 99], proj = ['projects', projectes(), 5];
    // Els fitxers només amb cerca: sense, serien centenars de files.
    const fit = ['files', q ? fitxersDe(ln ? ln[2] : 0) : [], ln ? 12 : 6, qf];
    const grups = ln ? [fit, acc, conv, proj] : q ? [acc, conv, fit, proj] : [conv, acc, proj];
    const out = [];
    for (const [, llista, max, cerca = q] of grups) {
      const fets = llista.map(x => ({ x, s: Math.max(puntua(x.label, cerca), puntua(x.hint, cerca) - 50, puntua(x.extra, cerca) - 80) }))
        .filter(o => o.s >= 0);
      if (q) fets.sort((a, b) => b.s - a.s);
      out.push(...fets.slice(0, max).map(o => o.x));
    }
    return out;
  }

  function pinta() {
    items = filtra(input.value);
    if (sel >= items.length) sel = Math.max(0, items.length - 1);
    let html = '', grup = '';
    items.forEach((it, i) => {
      if (it.group !== grup) {
        grup = it.group;
        html += '<li class="pal-group" role="presentation">' + esc(T('pal.g.' + grup)) + '</li>';
      }
      html += '<li class="pal-item' + (i === sel ? ' on' : '') + '" role="option" id="palOpt' + i + '" data-i="' + i + '" aria-selected="' + (i === sel) + '">' +
        '<span class="pal-ico">' + icona(it.icon) + '</span><span class="pal-label">' + esc(it.label) + '</span>' +
        (it.hint ? '<span class="pal-hint">' + esc(it.hint) + '</span>' : '') +
        (it.key ? '<kbd class="pal-kbd">' + esc(it.key) + '</kbd>' : '') + '</li>';
    });
    if (!items.length) html = '<li class="pal-empty" role="presentation">' + esc(T('pal.empty')) + '</li>';
    list.innerHTML = html;
    marca();
  }

  function marca() {
    list.querySelectorAll('.pal-item').forEach(li => {
      const on = +li.dataset.i === sel;
      li.classList.toggle('on', on); li.setAttribute('aria-selected', String(on));
    });
    const on = $('palOpt' + sel);
    if (on) { input.setAttribute('aria-activedescendant', on.id); on.scrollIntoView({ block: 'nearest' }); }
    else input.removeAttribute('aria-activedescendant');
  }

  function munta() {
    if (root) return;
    root = document.createElement('div');
    root.id = 'palette'; root.hidden = true;
    root.innerHTML = '<div class="pal-box" role="dialog" aria-modal="true">' +
      '<div class="pal-search"><span class="pal-ico" aria-hidden="true">' + icona('search') + '</span>' +
      '<input id="palInput" type="text" role="combobox" aria-expanded="true" aria-controls="palList" aria-autocomplete="list" autocomplete="off" spellcheck="false"></div>' +
      '<ul id="palList" role="listbox"></ul><div class="pal-foot" aria-hidden="true"></div></div>';
    document.body.appendChild(root);
    input = $('palInput'); list = $('palList');
    input.addEventListener('input', () => { sel = 0; pinta(); });
    input.addEventListener('keydown', tecla);
    list.addEventListener('mousemove', e => { const li = e.target.closest('.pal-item'); if (li && +li.dataset.i !== sel) { sel = +li.dataset.i; marca(); } });
    list.addEventListener('click', e => { const li = e.target.closest('.pal-item'); if (li) executa(+li.dataset.i); });
    root.addEventListener('mousedown', e => { if (e.target === root) { e.preventDefault(); tanca(); } });
  }

  function tecla(e) {
    // Les tecles de la paleta no surten d'aquí: l'Escape global atura el torn.
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault(); e.stopPropagation();
      if (items.length) { sel = (sel + (e.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length; marca(); }
    } else if (e.key === 'Home' && e.ctrlKey || e.key === 'PageUp') { e.preventDefault(); sel = 0; marca(); }
    else if (e.key === 'End' && e.ctrlKey || e.key === 'PageDown') { e.preventDefault(); sel = Math.max(0, items.length - 1); marca(); }
    else if (e.key === 'Enter') { e.preventDefault(); e.stopPropagation(); if (!e.isComposing) executa(sel); }
    else if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); tanca(); }
    // Trampa del focus: a dins només hi ha el camp; el Tab no en surt.
    else if (e.key === 'Tab') { e.preventDefault(); }
  }

  function executa(i) {
    const it = items[i];
    if (!it) return;
    tanca(false);
    try { const r = it.run(); if (r && r.catch) r.catch(err => G().sys && G().sys(String(err.message || err))); } catch (err) { G().sys && G().sys(String(err.message || err)); }
  }

  function obre() {
    munta();
    abans = document.activeElement;
    root.querySelector('.pal-box').setAttribute('aria-label', T('pal.dialog'));
    input.placeholder = T('pal.placeholder');
    input.setAttribute('aria-label', T('pal.placeholder'));
    root.querySelector('.pal-foot').textContent = T('pal.hint');
    input.value = ''; sel = 0;
    // La llista de converses pot ser vella: es refà en segon pla i es repinta.
    const c = P().convs;
    if (c && c.load && producte() === 'codi') c.load().then(() => { if (obert()) pinta(); }).catch(() => {});
    const f = P().files;
    if (f && f.paths && producte() === 'codi') f.paths().then(l => { fitxers = Array.isArray(l) ? l : []; if (obert() && input.value.trim()) pinta(); }).catch(() => {});
    pinta();
    root.hidden = false;
    document.body.classList.add('pal-open');
    input.focus();
  }

  function tanca(tornaFocus = true) {
    if (!root || root.hidden) return;
    root.hidden = true;
    document.body.classList.remove('pal-open');
    if (tornaFocus && abans && abans.focus && document.contains(abans)) abans.focus();
    abans = null;
  }

  const obert = () => !!root && !root.hidden;

  // Llocs on Ctrl+K ja vol dir una altra cosa: editors de codi i el
  // terminal del kanban (allà Ctrl+K és «esborra fins a final de línia»).
  const EXCLOSOS = '.cm-editor, .monaco-editor, .CodeMirror, .xterm, #kbInput, #kbTerm, [data-keep-ctrl-k]';

  // En captura, abans que el drecera antiga de l'index (Ctrl+K = enfoca el
  // compositor), que ara queda per als llocs exclosos.
  window.addEventListener('keydown', e => {
    if (!(e.ctrlKey || e.metaKey) || e.altKey || e.shiftKey || (e.key || '').toLowerCase() !== 'k') return;
    const a = document.activeElement;
    if (!obert() && a && a.closest && a.closest(EXCLOSOS)) return;
    e.preventDefault(); e.stopPropagation();
    obert() ? tanca() : obre();
  }, true);

  window.gregalPalette = { open: obre, close: tanca, isOpen: obert, _score: puntua };
})();
