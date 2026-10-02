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

  // ---- mode consulta: l'agent respon, mai toca el document ----
  // Per a documents compartits on hi ha més gent editant alhora: cap
  // inserció ni substitució des del panell, i el torn va al servidor en
  // mode xat (només lectura) encara que la sessió estigui en code.
  function esConsulta() { return !!consulta.checked; }
  function aplicaConsulta() {
    document.body.classList.toggle('consulta', esConsulta());
    xipsEdicio.hidden = esConsulta();
    xipsConsulta.hidden = !esConsulta();
    resp.dataset.buit = esConsulta()
      ? 'Aquí veuràs la resposta. En mode consulta no s’insereix res al document.'
      : 'Aquí veuràs el resultat. Podràs substituir la selecció o inserir-lo al cursor.';
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
        aplicaTema();
        try { if (Office.context.officeTheme && Office.onThemeChanged) Office.onThemeChanged(aplicaTema); } catch (e) {}
        dir('connectant…');
        comprova();
        aplicaConsulta();
      });
    } else if (++intents > 50) {
      clearInterval(espera);
      dir('obre aquest panell des del Word, l’Excel o el PowerPoint', 'mal');
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
      dir((j.model || 'gregal') + ' · llest', 'ok');
      return true;
    } catch (e) {
      dir('sense connexió amb el gregal (' + (e.message || e) + ')', 'mal');
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
      const t = await seleccio();
      const xips = document.querySelectorAll('.xip');
      xips.forEach(x => { x.disabled = !t; });
      notaSel.textContent = t
        ? 'Selecció: ' + t.replace(/\s+/g, ' ').slice(0, 90) + (t.length > 90 ? '…' : '') + ' (' + t.length + ' car.)'
        : (esConsulta()
          ? 'Mode consulta: l\u2019agent respon aquí i no modifica el document. Selecciona text per preguntar-hi.'
          : 'Selecciona text al document i tria una acció, o escriu la teva petició a sota.');
      bSub.disabled = !t || !ultima || esConsulta();
    };
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
      pregTitol.textContent = d.query || 'Pregunta';
      (d.options || []).slice(0, 4).forEach(o => {
        const b = document.createElement('button');
        b.className = 'boto';
        b.textContent = o.label; if (o.description) b.title = o.description;
        b.onclick = () => { preg.hidden = true; respondre(o.label); };
        pregBtns.appendChild(b);
      });
      const lliure = document.createElement('button');
      lliure.className = 'boto subtil';
      lliure.textContent = 'Responc amb el text de la petició';
      lliure.onclick = () => { preg.hidden = true; respondre('text:' + prompt.value); prompt.value = ''; };
      pregBtns.appendChild(lliure);
    } else {
      pregTitol.textContent = 'L’agent demana permís: ' + (d.name || 'eina');
      pregCos.textContent = (d.args || '').slice(0, 600);
      const si = document.createElement('button');
      si.textContent = 'Permet'; si.className = 'boto primari';
      si.onclick = () => { preg.hidden = true; respondre(true); };
      const no = document.createElement('button');
      no.textContent = 'Denega'; no.className = 'boto';
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
      dir('hi ha un torn en marxa, espero…', 'feina');
      await new Promise(r => setTimeout(r, 2000));
    }
  }

  // ---- muntar la tasca ----
  const guiaNomesText = 'Respon NOMÉS amb el text resultant, en la mateixa llengua que el text de partida (si no en té, en català), sense preàmbuls, explicacions ni cometes. Pots fer servir markdown senzill (títols, llistes, negreta, taules) si el resultat ho demana.';
  async function tasca(ordre) {
    const sel = await seleccio();
    let t = '';
    if (ordre) t = ordre;
    if (prompt.value.trim()) t = (t ? t + '\n\nInstruccions addicionals: ' : '') + prompt.value.trim();
    if (!t) return null;
    if (sel && (ordre || ambSel.checked)) {
      t += '\n\nText de partida (selecció al ' + (host || 'document') + '):\n"""\n' + sel.slice(0, 12000) + '\n"""';
    }
    if (esConsulta()) {
      t += '\n\nRespon al panell: no modifiquis cap fitxer ni proposis inserir res al document (el document és compartit i el llegeix més gent).';
    } else if (nomesText.checked || ordre) {
      t += '\n\n' + guiaNomesText;
    }
    return t;
  }

  async function executa(text) {
    if (!await comprova()) return;
    desa();
    if (!await esperaTorn()) { dir('espera cancel·lada', 'mal'); return; }
    pintaResposta('');
    let acumulat = '';
    bEnvia.disabled = true; bStop.disabled = false;
    document.querySelectorAll('.xip').forEach(x => { x.disabled = true; });
    dir('treballant…', 'feina');
    const ctl = new AbortController();
    avort = ctl;
    try {
      const r = await fetch(base() + '/api/agent', {
        method: 'POST', headers: caps({ 'Content-Type': 'application/json' }),
        body: JSON.stringify(esConsulta() ? { task: text, mode: 'chat' } : { task: text }), signal: ctl.signal,
      });
      if (!r.ok) {
        const t = await r.text();
        if (r.status === 409 && await esperaTorn()) { executa(text); return; }
        throw new Error(t || ('HTTP ' + r.status));
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
          else if (ev === 'tool_call') { dir((d.name || 'eina') + '…', 'feina'); }
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
      dir(ultima ? 'llest' : 'torn acabat sense text', ultima ? 'ok' : 'mal');
    } catch (e) {
      if (e && e.name === 'AbortError') dir('aturat', 'mal');
      else { pintaResposta(acumulat + '\n\n[connexió] ' + (e.message || e)); dir('error de connexió', 'mal'); }
    }
    bEnvia.disabled = false; bStop.disabled = true; avort = null;
    vigilaSeleccio();
  }

  document.querySelectorAll('.xip').forEach(x => {
    x.onclick = async () => {
      const t = await tasca(x.dataset.ordre);
      if (!t) { dir('selecciona text primer', 'mal'); return; }
      executa(t);
    };
  });
  bEnvia.onclick = async () => {
    const t = await tasca('');
    if (!t) { dir('escriu una petició primer', 'mal'); prompt.focus(); return; }
    executa(t);
  };
  prompt.addEventListener('keydown', e => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); bEnvia.onclick(); }
  });
  bStop.onclick = async () => {
    esperant = false;
    if (avort) { try { avort.abort(); } catch (e) {} }
    try { await fetch(base() + '/api/agent/cancel', { method: 'POST', headers: caps() }); } catch (e) {}
  };
  bNeteja.onclick = () => { prompt.value = ''; pintaResposta(''); dir('llest', 'ok'); prompt.focus(); };
  bCopia.onclick = async () => {
    try { await navigator.clipboard.writeText(ultima); dir('copiat', 'ok'); } catch (e) { dir('no s’ha pogut copiar', 'mal'); }
  };

  // ---- inserir al document, amb format quan el programa ho permet ----
  function insereix(mode) {
    if (!ultima || !enOffice() || esConsulta()) return;
    const acabat = (ok, err) => dir(ok ? (mode === 'Replace' ? 'selecció substituïda' : 'inserit al document') : 'no s’ha pogut inserir: ' + (err || ''), ok ? 'ok' : 'mal');
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
