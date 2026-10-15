/* Panell del complement Office del Gregal (Word/Excel/PowerPoint).
   Fa el que fa la gent al Word: accions ràpides sobre la selecció
   (reescriu, corregeix, resumeix, tradueix…), una petició lliure, i el
   resultat es pot substituir a la selecció o inserir al cursor. Al Word
   el resultat entra amb format de debò (títols, llistes, negreta,
   taules) via insertHtml de l'API del Word; a l'Excel una taula amb |
   entra com a matriu de cel·les; al PowerPoint, text.
   La feina la fa l'agent local via /api/agent (SSE); office.js ve del CDN
   de Microsoft (obligatori). El panell segueix el tema de l'Office. */
(function () {
  'use strict';
  const $ = id => document.getElementById(id);
  const estat = $('estat'), prompt = $('prompt'), resp = $('resposta'), hostLbl = $('host');
  const bEnvia = $('bEnvia'), bStop = $('bAtura'), bNeteja = $('bNeteja');
  const bIns = $('bInsereix'), bSub = $('bSubstitueix'), bCopia = $('bCopia');
  const preg = $('preg'), pregTitol = $('pregTitol'), pregCos = $('pregCos'), pregBtns = $('pregBtns');
  const srv = $('srv'), tok = $('tok'), ambSel = $('ambSel'), nomesText = $('nomesText'), notaSel = $('notaSel');
  const consulta = $('nomesConsulta'), xipsEdicio = $('xips'), xipsConsulta = $('xipsConsulta');
  let ultima = '', avort = null, esperant = false, host = '';

  // ---- idioma: el de Gregal (lang del servidor), o el de l'Office fins
  // que el servidor contesta. L'HTML ve en anglès; aquí es tradueix. ----
  const TXT = {
    en: {
      'st.starting': 'starting…', 'st.connecting': 'connecting…', 'st.noOffice': 'Open this panel from Word, Excel or PowerPoint',
      'st.ready': 'ready', 'st.offline': 'no connection to Gregal', 'st.waitTurn': 'a turn is running, waiting…',
      'st.waitCancelled': 'wait cancelled', 'st.working': 'working…', 'st.noText': 'turn finished without text',
      'st.stopped': 'stopped', 'st.connError': 'connection error', 'st.selectFirst': 'select some text first',
      'st.requestFirst': 'type a request first', 'st.copied': 'copied', 'st.copyFailed': 'could not copy',
      'st.replaced': 'selection replaced', 'st.inserted': 'inserted into the document', 'st.insertFailed': 'could not insert: ',
      'consulta.help': 'In Ask mode the agent only answers in the panel: it never inserts or replaces anything in the document. Meant for documents several people are editing at once. Off = Act: it can write into the document.',
      'consulta.label': 'Ask only: never touches the document',
      'h.selection': 'With the selection', 'h.request': 'Request', 'h.result': 'Result',
      'xip.summarize': 'Summarize', 'xip.explain': 'Explain', 'xip.review': 'Review', 'xip.keyPoints': 'Key points',
      'xip.questions': 'Questions', 'xip.toEnglish': '→ English', 'xip.verify': 'Fact-check', 'xip.rewrite': 'Rewrite',
      'xip.proofread': 'Proofread', 'xip.shorten': 'Shorten', 'xip.expand': 'Expand', 'xip.formal': 'More formal',
      'xip.toCatalan': '→ Catalan', 'xip.bullets': 'As bullets',
      'ord.summarize': 'Summarize the text in a short paragraph (or 3 bullet points if it is long).',
      'ord.explain': 'Explain what the text says in plain words, as if to someone outside the field.',
      'ord.review': 'Review the text and list the problems you find (clarity, errors, inconsistencies, missing points), quoting the passage for each one. Do not rewrite it.',
      'ord.keyPoints': 'Extract the key points, figures and decisions from the text as a list.',
      'ord.questions': 'Suggest 3–5 questions a critical reader would ask about this text.',
      'ord.toEnglishPanel': 'Translate the text into English (show it only in the panel).',
      'ord.verify': 'Check the verifiable claims in the text with a web search and say which hold up, which do not and which you could not verify, citing the source.',
      'ord.rewrite': 'Rewrite the text so it reads more clearly and smoothly, keeping its meaning, language and approximate length.',
      'ord.proofread': 'Fix spelling, grammar and punctuation. Change only what is needed.',
      'ord.shorten': 'Make the text more concise: remove redundancy without losing information.',
      'ord.expand': 'Expand the text with more detail and examples, keeping the tone.',
      'ord.formal': 'Rewrite the text in a more formal, professional tone.',
      'ord.toEnglish': 'Translate the text into English.',
      'ord.toCatalan': 'Translate the text into Catalan.',
      'ord.bullets': 'Turn the text into a clear bulleted list.',
      'sel.none': 'Select text in the document and choose an action, or type your request below.',
      'sel.consulta': 'Ask mode: the agent answers here and does not change the document. Select text to ask about it.',
      'sel.prefix': 'Selection: ', 'sel.chars': ' chars',
      'request.ph': 'e.g. Draft a two-paragraph introduction about…',
      'opt.includeSel': 'Include the selection', 'opt.textOnly': 'Result text only',
      'btn.generate': 'Generate', 'btn.stop': 'Stop', 'btn.clear': 'Clear', 'btn.clearTitle': 'Clear the request and the result',
      'btn.copy': 'Copy', 'btn.insert': 'Insert', 'btn.insertTitle': 'Insert the result at the cursor',
      'btn.replace': 'Replace selection', 'btn.replaceTitle': 'Replace the selected text with the result',
      'q.title': 'Question', 'q.freeText': 'Answer with the request text', 'q.permission': 'The agent asks for permission: ',
      'q.tool': 'tool', 'q.allow': 'Allow', 'q.deny': 'Deny',
      'empty.edit': 'The result appears here. You can replace the selection or insert it at the cursor.',
      'empty.consulta': 'The answer appears here. In Ask mode nothing is inserted into the document.',
      'conn.title': 'Connection', 'conn.server': 'Gregal server', 'conn.token': 'Token (if the server requires one)',
      'conn.optional': '(optional)', 'conn.test': 'Test connection',
      'task.extra': 'Additional instructions: ', 'task.source': 'Source text (selection in {host}):',
      'task.consulta': 'Answer in the panel: do not modify any file or offer to insert anything into the document (it is shared and other people read it).',
      'task.textOnly': 'Reply ONLY with the resulting text, in the same language as the source text (if there is none, in English), without preambles, explanations or quotation marks. You may use simple markdown (headings, lists, bold, tables) if the result calls for it.',
      'err.connection': '[connection] '
    },
    ca: {
      'st.starting': 'iniciant…', 'st.connecting': 'connectant…', 'st.noOffice': 'obre aquest panell des del Word, l’Excel o el PowerPoint',
      'st.ready': 'llest', 'st.offline': 'sense connexió amb el gregal', 'st.waitTurn': 'hi ha un torn en marxa, espero…',
      'st.waitCancelled': 'espera cancel·lada', 'st.working': 'treballant…', 'st.noText': 'torn acabat sense text',
      'st.stopped': 'aturat', 'st.connError': 'error de connexió', 'st.selectFirst': 'selecciona text primer',
      'st.requestFirst': 'escriu una petició primer', 'st.copied': 'copiat', 'st.copyFailed': 'no s’ha pogut copiar',
      'st.replaced': 'selecció substituïda', 'st.inserted': 'inserit al document', 'st.insertFailed': 'no s’ha pogut inserir: ',
      'consulta.help': 'En mode Pregunta l’agent només respon al panell: no insereix ni substitueix res al document. Pensat per a documents on hi ha més gent treballant alhora. Desactivat = Actua: pot escriure al document.',
      'consulta.label': 'Només Pregunta: no toca el document',
      'h.selection': 'Amb la selecció', 'h.request': 'Petició', 'h.result': 'Resultat',
      'xip.summarize': 'Resumeix', 'xip.explain': 'Explica', 'xip.review': 'Revisa', 'xip.keyPoints': 'Punts clau',
      'xip.questions': 'Preguntes', 'xip.toEnglish': '→ Anglès', 'xip.verify': 'Verifica', 'xip.rewrite': 'Reescriu',
      'xip.proofread': 'Corregeix', 'xip.shorten': 'Escurça', 'xip.expand': 'Amplia', 'xip.formal': 'Més formal',
      'xip.toCatalan': '→ Català', 'xip.bullets': 'En punts',
      'ord.summarize': 'Resumeix el text en un paràgraf breu (o en 3 punts si és llarg).',
      'ord.explain': 'Explica què diu el text amb paraules planes, com a qui no és del tema.',
      'ord.review': 'Revisa el text i llista els problemes que hi trobis (claredat, errors, incoherències, coses que falten), citant el fragment de cadascun. No el reescriguis.',
      'ord.keyPoints': 'Extreu els punts clau, les dades i les decisions del text en una llista.',
      'ord.questions': 'Proposa 3-5 preguntes que un lector crític faria sobre aquest text.',
      'ord.toEnglishPanel': 'Tradueix el text a l’anglès (mostra’l només al panell).',
      'ord.verify': 'Comprova les afirmacions verificables del text amb cerca web i digues quines quadren, quines no i quines no has pogut comprovar, amb la font.',
      'ord.rewrite': 'Reescriu el text perquè quedi més clar i fluid, conservant el sentit, la llengua i la llargada aproximada.',
      'ord.proofread': 'Corregeix l’ortografia, la gramàtica i la puntuació. Canvia només el que calgui.',
      'ord.shorten': 'Fes el text més concís: elimina redundàncies sense perdre informació.',
      'ord.expand': 'Amplia el text amb més detall i exemples, mantenint el to.',
      'ord.formal': 'Reescriu el text amb un to més formal i professional.',
      'ord.toEnglish': 'Tradueix el text a l’anglès.',
      'ord.toCatalan': 'Tradueix el text al català.',
      'ord.bullets': 'Converteix el text en una llista de punts clars.',
      'sel.none': 'Selecciona text al document i tria una acció, o escriu la teva petició a sota.',
      'sel.consulta': 'Mode Pregunta: l’agent respon aquí i no modifica el document. Selecciona text per preguntar-hi.',
      'sel.prefix': 'Selecció: ', 'sel.chars': ' car.',
      'request.ph': 'p. ex. Redacta una introducció de dos paràgrafs sobre…',
      'opt.includeSel': 'Inclou la selecció', 'opt.textOnly': 'Només el text resultant',
      'btn.generate': 'Genera', 'btn.stop': 'Atura', 'btn.clear': 'Neteja', 'btn.clearTitle': 'Buida la petició i la resposta',
      'btn.copy': 'Copia', 'btn.insert': 'Insereix', 'btn.insertTitle': 'Insereix el resultat al cursor',
      'btn.replace': 'Substitueix la selecció', 'btn.replaceTitle': 'Substitueix el text seleccionat pel resultat',
      'q.title': 'Pregunta', 'q.freeText': 'Responc amb el text de la petició', 'q.permission': 'L’agent demana permís: ',
      'q.tool': 'eina', 'q.allow': 'Permet', 'q.deny': 'Denega',
      'empty.edit': 'Aquí veuràs el resultat. Podràs substituir la selecció o inserir-lo al cursor.',
      'empty.consulta': 'Aquí veuràs la resposta. En mode Pregunta no s’insereix res al document.',
      'conn.title': 'Connexió', 'conn.server': 'Servidor del gregal', 'conn.token': 'Token (si el servidor en demana)',
      'conn.optional': '(opcional)', 'conn.test': 'Comprova',
      'task.extra': 'Instruccions addicionals: ', 'task.source': 'Text de partida (selecció al {host}):',
      'task.consulta': 'Respon al panell: no modifiquis cap fitxer ni proposis inserir res al document (el document és compartit i el llegeix més gent).',
      'task.textOnly': 'Respon NOMÉS amb el text resultant, en la mateixa llengua que el text de partida (si no en té, en català), sense preàmbuls, explicacions ni cometes. Pots fer servir markdown senzill (títols, llistes, negreta, taules) si el resultat ho demana.',
      'err.connection': '[connexió] '
    }
  };
  let lang = 'en', langFixat = false;
  try { const v = localStorage.getItem('gregal_idioma'); if (TXT[v]) { lang = v; langFixat = true; } } catch (e) {}
  if (!langFixat && /^ca\b/i.test(navigator.language || '')) lang = 'ca';
  function t(k) { return (TXT[lang] && TXT[lang][k]) || TXT.en[k] || k; }
  let miraSeleccio = null;
  function aplicaIdioma() {
    document.documentElement.lang = lang;
    document.querySelectorAll('[data-k]').forEach(el => { el.textContent = t(el.dataset.k); });
    document.querySelectorAll('[data-k-title]').forEach(el => { el.title = t(el.dataset.kTitle); });
    document.querySelectorAll('[data-k-ph]').forEach(el => { el.placeholder = t(el.dataset.kPh); });
    document.querySelectorAll('[data-k-ordre]').forEach(el => { el.dataset.ordre = t(el.dataset.kOrdre); });
    resp.dataset.buit = t(consulta.checked ? 'empty.consulta' : 'empty.edit');
    if (miraSeleccio) miraSeleccio();
  }
  // posaIdioma: el lang del servidor mana (és la tria de Gregal); l'idioma
  // de l'Office només si encara no se'n sap res més.
  function posaIdioma(l, feble) {
    if (!TXT[l] || l === lang || (feble && langFixat)) return;
    lang = l;
    if (!feble) langFixat = true;
    aplicaIdioma();
  }
  aplicaIdioma();

  // ---- mode Pregunta (Ask; intern: «consulta»): l'agent respon, mai toca el document ----
  // Per a documents compartits on hi ha més gent editant alhora: cap
  // inserció ni substitució des del panell, i el torn va al servidor en
  // mode xat (només lectura) encara que la sessió estigui en code.
  function esConsulta() { return !!consulta.checked; }
  function aplicaConsulta() {
    document.body.classList.toggle('consulta', esConsulta());
    xipsEdicio.hidden = esConsulta();
    xipsConsulta.hidden = !esConsulta();
    resp.dataset.buit = t(esConsulta() ? 'empty.consulta' : 'empty.edit');
    try { localStorage.setItem('gregal_addin_consulta', esConsulta() ? '1' : '0'); } catch (e) {}
    vigilaSeleccio();
  }
  try { consulta.checked = localStorage.getItem('gregal_addin_consulta') === '1'; } catch (e) {}
  consulta.onchange = aplicaConsulta;

  // Per defecte, el servidor és el que ha servit aquest panell (el gregal
  // tria port lliure a l'escriptori): així no cal tocar res.
  const servitPelGregal = /^https?:\/\//.test(location.origin);
  try {
    // Un servidor desat només mana si el panell NO el serveix el gregal
    // (p. ex. obert des de fitxer): si el serveix, l'origen és la veritat
    // i el port desat d'una execució anterior seria vell.
    const desat = localStorage.getItem('gregal_addin_srv');
    if (servitPelGregal) srv.value = location.origin;
    else if (desat) srv.value = desat;
    tok.value = localStorage.getItem('gregal_addin_tok') || '';
  } catch (e) { if (servitPelGregal) srv.value = location.origin; }

  function base() { return srv.value.replace(/\/$/, ''); }
  function caps(extra) {
    const h = Object.assign({}, extra);
    if (tok.value.trim()) h['Authorization'] = 'Bearer ' + tok.value.trim();
    return h;
  }
  function dir(t, classe) { estat.textContent = t; estat.className = classe || ''; }
  function desa() {
    try {
      localStorage.setItem('gregal_addin_srv', srv.value);
      localStorage.setItem('gregal_addin_tok', tok.value);
    } catch (e) { /* tant és */ }
  }

  // ---- tema de l'Office: fosc o clar segons el programa ----
  function aplicaTema() {
    try {
      const t = Office.context && Office.context.officeTheme;
      if (!t || !t.bodyBackgroundColor) return;
      const c = t.bodyBackgroundColor.replace('#', '');
      const r = parseInt(c.slice(0, 2), 16), g = parseInt(c.slice(2, 4), 16), b = parseInt(c.slice(4, 6), 16);
      const clar = (r * 299 + g * 587 + b * 114) / 1000 > 128;
      document.documentElement.dataset.theme = clar ? 'light' : 'dark';
    } catch (e) { /* sense tema: clar */ }
  }

  // office.js triga a carregar (CDN): espera'l amb límit, si no, avisa.
  let intents = 0;
  const espera = setInterval(() => {
    if (typeof Office !== 'undefined' && Office.onReady) {
      clearInterval(espera);
      Office.onReady(info => {
        host = info.host || '';
        const noms = { Word: 'Word', Excel: 'Excel', PowerPoint: 'PowerPoint' };
        hostLbl.textContent = noms[host] || '';
        try { posaIdioma(/^ca\b/i.test(Office.context.displayLanguage || '') ? 'ca' : 'en', true); } catch (e) {}
        aplicaTema();
        try { if (Office.context.officeTheme && Office.onThemeChanged) Office.onThemeChanged(aplicaTema); } catch (e) {}
        dir(t('st.connecting'));
        comprova();
        aplicaConsulta();
      });
    } else if (++intents > 50) {
      clearInterval(espera);
      dir(t('st.noOffice'), 'mal');
    }
  }, 200);

  function enOffice() { return typeof Office !== 'undefined' && Office.context && Office.context.document; }
  const esWord = () => host === 'Word' && typeof Word !== 'undefined';
  const esExcel = () => host === 'Excel' && typeof Excel !== 'undefined';

  async function comprova() {
    desa();
    try {
      const r = await fetch(base() + '/api/state', { headers: caps() });
      if (!r.ok) throw new Error('HTTP ' + r.status);
      const j = await r.json();
      if (j.lang) posaIdioma(j.lang);
      dir((j.model || 'gregal') + ' · ' + t('st.ready'), 'ok');
      return true;
    } catch (e) {
      dir(t('st.offline') + ' (' + (e.message || e) + ')', 'mal');
      return false;
    }
  }

  function seleccio() {
    return new Promise(res => {
      if (!enOffice()) { res(''); return; }
      try {
        Office.context.document.getSelectedDataAsync(Office.CoercionType.Text, r => {
          if (r.status === Office.AsyncResultStatus.Succeeded && r.value) res(String(r.value).trim());
          else res('');
        });
      } catch (e) { res(''); }
    });
  }

  // La nota sota els xips diu si hi ha selecció (sense haver de prémer res).
  function vigilaSeleccio() {
    const mira = async () => {
      const sel = await seleccio();
      const xips = document.querySelectorAll('.xip');
      xips.forEach(x => { x.disabled = !sel; });
      notaSel.textContent = sel
        ? t('sel.prefix') + sel.replace(/\s+/g, ' ').slice(0, 90) + (sel.length > 90 ? '…' : '') + ' (' + sel.length + t('sel.chars') + ')'
        : t(esConsulta() ? 'sel.consulta' : 'sel.none');
      bSub.disabled = !sel || !ultima || esConsulta();
    };
    miraSeleccio = mira;
    try {
      Office.context.document.addHandlerAsync(Office.EventType.DocumentSelectionChanged, mira);
    } catch (e) { setInterval(mira, 1500); }
    mira();
  }

  // ---- markdown → HTML (subconjunt) per al resultat i per al Word ----
  function esc(s) { return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;'); }
  function inline(s) {
    return esc(s)
      .replace(/`([^`]+)`/g, '<code>$1</code>')
      .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
      .replace(/(^|[^*])\*([^*\n]+)\*/g, '$1<em>$2</em>')
      .replace(/\[([^\]]+)\]\((https?:[^)\s]+)\)/g, '<a href="$2">$1</a>');
  }
  function mdToHtml(md) {
    const lines = md.replace(/\r/g, '').split('\n');
    let html = '', list = null, para = [], code = null, table = null;
    const flushP = () => { if (para.length) { html += '<p>' + inline(para.join(' ')) + '</p>'; para = []; } };
    const flushL = () => { if (list) { html += '</' + list + '>'; list = null; } };
    const flushT = () => {
      if (!table) return;
      html += '<table>';
      table.forEach((row, i) => {
        html += '<tr>' + row.map(c => (i === 0 ? '<th>' : '<td>') + inline(c.trim()) + (i === 0 ? '</th>' : '</td>')).join('') + '</tr>';
      });
      html += '</table>'; table = null;
    };
    for (const ln of lines) {
      if (code !== null) {
        if (/^```/.test(ln)) { html += '<pre><code>' + esc(code.join('\n')) + '</code></pre>'; code = null; }
        else code.push(ln);
        continue;
      }
      if (/^```/.test(ln)) { flushP(); flushL(); flushT(); code = []; continue; }
      const t = ln.trim();
      if (/^\|.*\|$/.test(t)) {
        flushP(); flushL();
        if (/^\|\s*:?-{2,}/.test(t)) continue;
        (table = table || []).push(t.slice(1, -1).split('|'));
        continue;
      }
      flushT();
      let m;
      if ((m = /^(#{1,6})\s+(.+)$/.exec(t))) { flushP(); flushL(); const n = Math.min(m[1].length, 3); html += '<h' + n + '>' + inline(m[2]) + '</h' + n + '>'; continue; }
      if ((m = /^[-*•]\s+(.+)$/.exec(t))) { flushP(); if (list !== 'ul') { flushL(); list = 'ul'; html += '<ul>'; } html += '<li>' + inline(m[1]) + '</li>'; continue; }
      if ((m = /^\d+[.)]\s+(.+)$/.exec(t))) { flushP(); if (list !== 'ol') { flushL(); list = 'ol'; html += '<ol>'; } html += '<li>' + inline(m[1]) + '</li>'; continue; }
      if (t === '') { flushP(); flushL(); continue; }
      para.push(t);
    }
    flushP(); flushL(); flushT();
    return html;
  }
  function esTaula(md) { return /^\s*\|.*\|\s*$/m.test(md); }
  function taulaAMatriu(md) {
    return md.split('\n').map(l => l.trim()).filter(l => /^\|.*\|$/.test(l) && !/^\|\s*:?-{2,}/.test(l))
      .map(l => l.slice(1, -1).split('|').map(c => c.trim()));
  }

  function pintaResposta(md) {
    ultima = md.trim();
    resp.innerHTML = ultima ? mdToHtml(ultima) : '';
    const hiHa = !!ultima;
    bIns.disabled = !hiHa || !enOffice() || esConsulta();
    bCopia.disabled = !hiHa;
    seleccio().then(t => { bSub.disabled = !hiHa || !t || !enOffice() || esConsulta(); });
  }

  // ---- pregunta / permís del model ----
  function preguntaUI(ev, d, respondre) {
    preg.hidden = false;
    pregCos.textContent = '';
    pregBtns.innerHTML = '';
    if (ev === 'question_request') {
      pregTitol.textContent = d.query || t('q.title');
      (d.options || []).slice(0, 4).forEach(o => {
        const b = document.createElement('button');
        b.className = 'boto';
        b.textContent = o.label; if (o.description) b.title = o.description;
        b.onclick = () => { preg.hidden = true; respondre(o.label); };
        pregBtns.appendChild(b);
      });
      const lliure = document.createElement('button');
      lliure.className = 'boto subtil';
      lliure.textContent = t('q.freeText');
      lliure.onclick = () => { preg.hidden = true; respondre('text:' + prompt.value); prompt.value = ''; };
      pregBtns.appendChild(lliure);
    } else {
      pregTitol.textContent = t('q.permission') + (d.name || t('q.tool'));
      pregCos.textContent = (d.args || '').slice(0, 600);
      const si = document.createElement('button');
      si.textContent = t('q.allow'); si.className = 'boto primari';
      si.onclick = () => { preg.hidden = true; respondre(true); };
      const no = document.createElement('button');
      no.textContent = t('q.deny'); no.className = 'boto';
      no.onclick = () => { preg.hidden = true; respondre(false); };
      pregBtns.append(si, no);
    }
  }

  async function esperaTorn() {
    esperant = true;
    for (;;) {
      if (!esperant) return false;
      let busy = false;
      try {
        const r = await fetch(base() + '/api/state', { headers: caps() });
        if (r.ok) busy = !!(await r.json()).agent_busy;
      } catch (e) { return true; }
      if (!busy) { esperant = false; return true; }
      dir(t('st.waitTurn'), 'feina');
      await new Promise(r => setTimeout(r, 2000));
    }
  }

  // ---- muntar la tasca ----
  async function tasca(ordre) {
    const sel = await seleccio();
    let text = '';
    if (ordre) text = ordre;
    if (prompt.value.trim()) text = (text ? text + '\n\n' + t('task.extra') : '') + prompt.value.trim();
    if (!text) return null;
    if (sel && (ordre || ambSel.checked)) {
      text += '\n\n' + t('task.source').replace('{host}', host || 'document') + '\n"""\n' + sel.slice(0, 12000) + '\n"""';
    }
    if (esConsulta()) {
      text += '\n\n' + t('task.consulta');
    } else if (nomesText.checked || ordre) {
      text += '\n\n' + t('task.textOnly');
    }
    return text;
  }

  async function executa(text) {
    if (!await comprova()) return;
    desa();
    if (!await esperaTorn()) { dir(t('st.waitCancelled'), 'mal'); return; }
    pintaResposta('');
    let acumulat = '';
    bEnvia.disabled = true; bStop.disabled = false;
    document.querySelectorAll('.xip').forEach(x => { x.disabled = true; });
    dir(t('st.working'), 'feina');
    const ctl = new AbortController();
    avort = ctl;
    try {
      const r = await fetch(base() + '/api/agent', {
        method: 'POST', headers: caps({ 'Content-Type': 'application/json' }),
        body: JSON.stringify(esConsulta() ? { task: text, mode: 'chat' } : { task: text }), signal: ctl.signal,
      });
      if (!r.ok) {
        const cos = await r.text();
        if (r.status === 409 && await esperaTorn()) { executa(text); return; }
        throw new Error(cos || ('HTTP ' + r.status));
      }
      const rd = r.body.getReader(), dec = new TextDecoder();
      let buf = '', ev = '', fet = false;
      for (;;) {
        const { done, value } = await rd.read();
        if (done) break;
        buf += dec.decode(value, { stream: true });
        let i;
        while ((i = buf.indexOf('\n\n')) >= 0) {
          const marc = buf.slice(0, i); buf = buf.slice(i + 2);
          let data = '';
          marc.split('\n').forEach(l => {
            if (l.indexOf('event:') === 0) ev = l.slice(6).trim();
            else if (l.indexOf('data:') === 0) data += l.slice(5).trim();
          });
          if (!ev || !data) continue;
          let d = {};
          try { d = JSON.parse(data); } catch (e) { continue; }
          if (ev === 'token') { acumulat += d.text || ''; resp.textContent = acumulat; fet = true; }
          else if (ev === 'assistant' && d.text) { acumulat = d.text; pintaResposta(acumulat); fet = true; }
          else if (ev === 'status' && d.message) { dir(d.message, 'feina'); }
          else if (ev === 'tool_call') { dir((d.name || t('q.tool')) + '…', 'feina'); }
          else if (ev === 'approve_request') {
            await new Promise(resApprove => preguntaUI(ev, d, ok => {
              fetch(base() + '/api/approve', { method: 'POST', headers: caps({ 'Content-Type': 'application/json' }), body: JSON.stringify({ key: d.key, approve: !!ok }) }).catch(() => {});
              resApprove();
            }));
          } else if (ev === 'question_request') {
            await new Promise(resQ => preguntaUI(ev, d, ans => {
              fetch(base() + '/api/question', { method: 'POST', headers: caps({ 'Content-Type': 'application/json' }), body: JSON.stringify({ key: d.key, answer: ans }) }).catch(() => {});
              resQ();
            }));
          } else if (ev === 'error') {
            acumulat += '\n[error] ' + (d.message || '');
          } else if (ev === 'done') {
            if (d.reply && !fet) acumulat = d.reply;
          }
        }
      }
      pintaResposta(acumulat);
      dir(t(ultima ? 'st.ready' : 'st.noText'), ultima ? 'ok' : 'mal');
    } catch (e) {
      if (e && e.name === 'AbortError') dir(t('st.stopped'), 'mal');
      else { pintaResposta(acumulat + '\n\n' + t('err.connection') + (e.message || e)); dir(t('st.connError'), 'mal'); }
    }
    bEnvia.disabled = false; bStop.disabled = true; avort = null;
    vigilaSeleccio();
  }

  document.querySelectorAll('.xip').forEach(x => {
    x.onclick = async () => {
      const task = await tasca(x.dataset.ordre);
      if (!task) { dir(t('st.selectFirst'), 'mal'); return; }
      executa(task);
    };
  });
  bEnvia.onclick = async () => {
    const task = await tasca('');
    if (!task) { dir(t('st.requestFirst'), 'mal'); prompt.focus(); return; }
    executa(task);
  };
  prompt.addEventListener('keydown', e => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); bEnvia.onclick(); }
  });
  bStop.onclick = async () => {
    esperant = false;
    if (avort) { try { avort.abort(); } catch (e) {} }
    try { await fetch(base() + '/api/agent/cancel', { method: 'POST', headers: caps() }); } catch (e) {}
  };
  bNeteja.onclick = () => { prompt.value = ''; pintaResposta(''); dir(t('st.ready'), 'ok'); prompt.focus(); };
  bCopia.onclick = async () => {
    try { await navigator.clipboard.writeText(ultima); dir(t('st.copied'), 'ok'); } catch (e) { dir(t('st.copyFailed'), 'mal'); }
  };

  // ---- inserir al document, amb format quan el programa ho permet ----
  function insereix(mode) {
    if (!ultima || !enOffice() || esConsulta()) return;
    const acabat = (ok, err) => dir(ok ? t(mode === 'Replace' ? 'st.replaced' : 'st.inserted') : t('st.insertFailed') + (err || ''), ok ? 'ok' : 'mal');
    if (esWord()) {
      // HTML: títols, llistes, negreta i taules queden com a format de Word.
      Word.run(async ctx => {
        const rng = ctx.document.getSelection();
        const on = mode === 'Replace' ? Word.InsertLocation.replace : Word.InsertLocation.after;
        rng.insertHtml(mdToHtml(ultima), on);
        await ctx.sync();
      }).then(() => acabat(true)).catch(e => {
        // Recanvi: text pla per l'API compartida.
        Office.context.document.setSelectedDataAsync(ultima, { coercionType: Office.CoercionType.Text }, r => acabat(r.status === Office.AsyncResultStatus.Succeeded, r.error && r.error.message || (e && e.message)));
      });
      return;
    }
    if (esExcel() && esTaula(ultima)) {
      const m = taulaAMatriu(ultima);
      Excel.run(async ctx => {
        const cel = ctx.workbook.getSelectedRange().getCell(0, 0);
        const rng = cel.getResizedRange(m.length - 1, Math.max(...m.map(r => r.length)) - 1);
        const ample = Math.max(...m.map(r => r.length));
        rng.values = m.map(r => { const c = r.slice(); while (c.length < ample) c.push(''); return c.map(v => (v !== '' && !isNaN(v) ? Number(v) : v)); });
        rng.format.autofitColumns();
        await ctx.sync();
      }).then(() => acabat(true)).catch(e => acabat(false, e && e.message));
      return;
    }
    try {
      Office.context.document.setSelectedDataAsync(ultima, { coercionType: Office.CoercionType.Text }, r => acabat(r.status === Office.AsyncResultStatus.Succeeded, r.error && r.error.message));
    } catch (e) { acabat(false, e.message || e); }
  }
  aplicaConsulta();
  bIns.onclick = () => insereix('After');
  bSub.onclick = () => insereix('Replace');
  $('bProva').onclick = comprova;
})();
