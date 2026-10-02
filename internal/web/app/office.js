// office: documents Word/Excel/PowerPoint com a ciutadans de primera.
//
// Abans la pàgina Office era un formulari: tria fitxer, un <pre> amb el text i
// camps per posar un valor a una cel·la o substituir text a mà. El que es vol
// és: deixar caure el document, veure'l en una subfinestra lateral que es
// queda mentre parles amb l'agent, i demanar-li coses ("resumeix", "corregeix
// l'ortografia", "posa el total a B12") que ell fa amb office_read/office_edit
// sobre el fitxer real. Quan acaba el torn, el document es rellegeix sol.
//
// El servidor ja ho permetia: /api/office/upload desa el fitxer al disc i
// torna la ruta; les eines de l'agent treballen amb rutes. Faltava la UI.
const $ = id => document.getElementById(id);
const G = () => window.gregal;
import { t as T } from './i18n.js';

const esc = s => String(s == null ? '' : s)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

const KIND_LABEL = { docx: 'Word', xlsx: 'Excel', pptx: 'PowerPoint' };
const message = (key, values = {}) => T(key).replace(/\{(\w+)\}/g, (match, name) => values[name] == null ? match : String(values[name]));

// lletraCol converteix un índex de columna en la seva lletra (0→A, 26→AA),
// com a l'Excel: sense això, a la columna dotze no saps quina mires i no
// pots dir-li a l'agent «posa el total a L12».
function lletraCol(i) {
  let s = '';
  for (i += 1; i > 0; i = Math.floor((i - 1) / 26)) s = String.fromCharCode(65 + (i - 1) % 26) + s;
  return s;
}

const AMPLE_MIN = 340, CLAU_AMPLE = 'gregal_doc_ample';

// ampleDesat recupera l'amplada de la subfinestra. El màxim es calcula en
// cada moment: si desaves 1200px en un monitor gran i després obres el
// portàtil, el xat es quedava sense espai.
function maxAmple() { return Math.max(AMPLE_MIN, window.innerWidth - 420); }
function posaAmple(px) {
  const v = Math.round(Math.min(Math.max(px, AMPLE_MIN), maxAmple()));
  document.documentElement.style.setProperty('--doc-w', v + 'px');
  return v;
}

// Peticions ràpides per tipus: una frase que l'agent entén i que fa servir
// les eines office. L'usuari pot escriure la seva a més.
const QUICK = {
  docx: [
    ['office.quick.summarize', 'office.prompt.docSummary'],
    ['office.quick.correct', 'office.prompt.docCorrect'],
    ['office.quick.translate', 'office.prompt.docTranslate'],
    ['office.quick.tone', 'office.prompt.docTone'],
  ],
  xlsx: [
    ['office.quick.data', 'office.prompt.sheetExplain'],
    ['office.quick.errors', 'office.prompt.sheetErrors'],
    ['office.quick.value', 'office.prompt.sheetValue'],
  ],
  pptx: [
    ['office.quick.summarize', 'office.prompt.slidesSummary'],
    ['office.quick.correct', 'office.prompt.slidesCorrect'],
    ['office.quick.notes', 'office.prompt.slidesNotes'],
  ],
};

export function quickActions(kind) {
  return (QUICK[kind] || QUICK.docx).map(([label, prompt]) => [T(label), T(prompt)]);
}

export const office = {
  doc: null,        // {id, kind, name, path, text}
  recent: [],       // documents d'aquesta sessió de navegador
  init() {
    const drop = $('offDrop'), pick = $('offPick');
    if (!drop || !pick) return;
    drop.addEventListener('click', () => pick.click());
    drop.addEventListener('keydown', e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); pick.click(); } });
    pick.addEventListener('change', () => { const f = pick.files[0]; if (f) this.upload(f); pick.value = ''; });
    // Deixar caure a tota la pàgina Office, no només al requadre.
    const page = $('officePage');
    ['dragenter', 'dragover'].forEach(ev => page.addEventListener(ev, e => { e.preventDefault(); drop.classList.add('over'); }));
    ['dragleave', 'drop'].forEach(ev => page.addEventListener(ev, e => { e.preventDefault(); drop.classList.remove('over'); }));
    page.addEventListener('drop', e => { const f = e.dataTransfer?.files?.[0]; if (f) this.upload(f); });
    this.initGrip();
    $('docClose').onclick = () => this.close();
    const minBtn = $('docMin');
    if (minBtn) minBtn.onclick = () => this.minimize();
    const minBar = $('docMinBar');
    if (minBar) minBar.onclick = e => { if (!e.target.closest('#docMinClose') && !e.target.closest('#docRestoreBtn')) this.restore(); };
    const restoreBtn = $('docRestoreBtn');
    if (restoreBtn) restoreBtn.onclick = e => { e.stopPropagation(); this.restore(); };
    const minClose = $('docMinClose');
    if (minClose) minClose.onclick = e => { e.stopPropagation(); this.close(); };
    const chatBtn = $('docChatSession');
    if (chatBtn) chatBtn.onclick = () => this.chatSession();
    $('docDl').onclick = () => this.download();
    $('docOpenApp').onclick = () => this.openApp();
    $('docVista').onclick = () => this.toggleView();
    // El fitxer viu al servidor: obrir-lo amb el Word només té sentit si
    // el servidor és local (app d'escriptori). Fora d'allà, el botó sobra.
    if (!window.gregalDesktop) $('docOpenApp').hidden = true;
    $('docAskBtn').onclick = () => this.ask($('docAsk').value);
    $('docAsk').addEventListener('keydown', e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); this.ask($('docAsk').value); } });
    $('docOpenOffice').onclick = () => G().setView('office');
    this.renderRecent();
    // Aquest text es construeix aquí, no amb data-i18n: sense escoltar el
    // canvi d'idioma es quedava en la llengua que hi havia en pintar-lo.
    document.addEventListener('gregal:idioma', () => { this.renderRecent(); this.renderControls(); });
  },
  msg(t, bad) { const el = $('offMsg'); el.textContent = t || ''; el.classList.toggle('bad', !!bad); },
  async upload(f) {
    if (!/\.(docx|xlsx|pptx)$/i.test(f.name)) { this.msg(T('office.formats'), true); return; }
    if (f.size > 25 * 1024 * 1024) { this.msg(T('office.tooLarge'), true); return; }
    this.msg(message('office.uploading', { name: f.name }));
    try {
      const buf = new Uint8Array(await f.arrayBuffer());
      let bin = ''; const CH = 0x8000;
      for (let i = 0; i < buf.length; i += CH) bin += String.fromCharCode.apply(null, buf.subarray(i, i + CH));
      const up = await G().api('/api/office/upload', { method: 'POST', body: JSON.stringify({ name: f.name, data: btoa(bin) }) });
      if (!up.ok) { this.msg(await up.text(), true); return; }
      const d = await up.json();
      this.doc = { id: d.id, kind: d.kind, name: d.name || f.name, path: d.path, text: '' };
      this.recent = [this.doc, ...this.recent.filter(r => r.id !== d.id)].slice(0, 8);
      await this.read();
      this.msg('');
      this.renderRecent();
      this.openPane();
      G().activityItem('office', message('office.uploaded', { name: this.doc.name }), KIND_LABEL[this.doc.kind] || '', 'ok');
    } catch (e) { this.msg(String(e.message || e), true); }
  },
  async read() {
    if (!this.doc) return;
    const r = await G().api('/api/office/read', { method: 'POST', body: JSON.stringify({ id: this.doc.id }) });
    if (!r.ok) { this.doc.text = message('office.readFailed', { error: await r.text() }); return; }
    const rd = await r.json();
    this.doc.text = rd.text || T('office.noText');
    this.doc.kind = this.doc.kind || rd.kind || '';
  },
  openPane() {
    const d = this.doc; if (!d) return;
    const textFile = d.kind === 'text';
    const pane = $('docPane');
    if (pane) pane.classList.toggle('doc-text-file', textFile);
    this.restore();
    $('docName').textContent = d.name;
    $('docKind').textContent = KIND_LABEL[d.kind] || d.kind || '';
    ['docOpenOffice', 'docOpenApp', 'docDl', 'docChatSession', 'docVista'].forEach(id => { const el = $(id); if (el) el.hidden = textFile; });
    if (!textFile) {
      const openApp = $('docOpenApp'); if (openApp && !window.gregalDesktop) openApp.hidden = true;
    }
    // docx/xlsx/pptx: render fidel per defecte; si falla, text amb nota.
    // La vista es conserva en canviar de document.
    if (!d.view) d.view = 'vista';
    this.paintDoc();
    this.renderControls();
    $('docPane').hidden = false;
    document.body.classList.add('doc-open');
    // En pantalles estretes la subfinestra és una capa de tot l'ample: la
    // barra lateral no pot quedar oberta a sota.
    if (window.innerWidth <= 900 && window.setSide) window.setSide(false);
  },
  // paintDoc pinta text o render segons d.view.
  // initGrip munta l'agafador de l'esquerra. Amb teclat també: fletxes de
  // 24 px, que un separador que només va amb ratolí no el pot fer servir
  // tothom.
  renderControls() {
    const d = this.doc; if (!d) return;
    const app = $('docOpenApp');
    if (app) app.textContent = message('office.openWith', { app: KIND_LABEL[d.kind] || T('office.application') });
    const toggle = $('docVista');
    if (toggle) toggle.textContent = T(d.view === 'vista' ? 'office.textView' : 'office.previewView');
    const q = $('docQuick'); if (!q) return;
    q.innerHTML = '';
    quickActions(d.kind).forEach(([label, prompt]) => {
      const button = document.createElement('button'); button.className = 'chip'; button.type = 'button'; button.textContent = label;
      button.onclick = () => { if (prompt.includes('___')) { $('docAsk').value = prompt; $('docAsk').focus(); } else this.ask(prompt); };
      q.appendChild(button);
    });
  },
  initGrip() {
    const grip = $('docGrip');
    if (!grip) return;
    let desat = 0;
    try { desat = parseInt(localStorage.getItem(CLAU_AMPLE), 10) || 0; } catch (e) {}
    if (desat) posaAmple(desat);

    const recorda = px => { try { localStorage.setItem(CLAU_AMPLE, String(px)); } catch (e) {} };
    grip.addEventListener('pointerdown', e => {
      e.preventDefault();
      grip.classList.add('arrossegant');
      document.body.classList.add('redimensionant');
      const mou = e2 => posaAmple(window.innerWidth - e2.clientX);
      const deixa = e2 => {
        window.removeEventListener('pointermove', mou);
        window.removeEventListener('pointerup', deixa);
        grip.classList.remove('arrossegant');
        document.body.classList.remove('redimensionant');
        recorda(posaAmple(window.innerWidth - e2.clientX));
      };
      window.addEventListener('pointermove', mou);
      window.addEventListener('pointerup', deixa);
    });
    grip.addEventListener('keydown', e => {
      const pas = e.key === 'ArrowLeft' ? 24 : e.key === 'ArrowRight' ? -24 : 0;
      if (!pas) return;
      e.preventDefault();
      const ara = parseInt(getComputedStyle(document.documentElement).getPropertyValue('--doc-w'), 10) || 520;
      recorda(posaAmple(ara + pas));
    });
    // En canviar la finestra, retalla si ja no hi cap.
    window.addEventListener('resize', () => {
      const ara = parseInt(getComputedStyle(document.documentElement).getPropertyValue('--doc-w'), 10) || 520;
      posaAmple(ara);
    });
  },

  async paintDoc() {
    const d = this.doc; if (!d) return;
    const box = $('docText'), tgl = $('docVista');
    if (d.kind === 'text') {
      box.textContent = d.text;
      return;
    }
    // Les pestanyes de fulls són cosa de l'xlsx en vista fidel; renderXlsx
    // les torna a posar. Si no s'amaguessin, en obrir un docx darrere d'un
    // xlsx et quedaven les del llibre anterior.
    const fulls = $('docSheets');
    if (fulls) { fulls.hidden = true; fulls.innerHTML = ''; }
    if (tgl) {
      tgl.hidden = false;
      tgl.textContent = T(d.view === 'vista' ? 'office.textView' : 'office.previewView');
    }
    if (d.view === 'vista') {
      box.innerHTML = '<p class="off-open">' + esc(T('office.rendering')) + '</p>';
      try {
        if (d.kind === 'docx') await this.renderDocx(box);
        else if (d.kind === 'xlsx') await this.renderXlsx(box);
        else await this.renderPptx(box);
      } catch (e) {
        box.innerHTML = this.preview(d.text);
        this.msgPane(message('office.previewFailed', { error: e.message || e }));
      }
      return;
    }
    box.innerHTML = this.preview(d.text);
  },
  toggleView() {
    const d = this.doc; if (!d) return;
    d.view = d.view === 'vista' ? 'text' : 'vista';
    this.paintDoc();
  },
  // Llibreries vendoritzades (/app/vendor/*): càrrega mandrosa, una vegada.
  _libs: {},
  ensureLib(name, files) {
    if (this._libs[name]) return this._libs[name];
    this._libs[name] = (async () => {
      for (const f of files) {
        await new Promise((res, rej) => {
          const s = document.createElement('script');
          s.src = '/app/vendor/' + f + '?v=1';
          s.onload = res;
          s.onerror = () => rej(new Error(message('office.loadFailed', { file: f })));
          document.head.appendChild(s);
        });
      }
    })();
    return this._libs[name];
  },
  async docBytes() {
    const r = await G().api('/api/office/download?id=' + encodeURIComponent(this.doc.id) + '&v=' + Date.now());
    if (!r.ok) throw new Error(await r.text());
    return r.arrayBuffer();
  },
  async renderDocx(box) {    await this.ensureLib('docx', ['jszip.min.js', 'docx-preview.min.js']);
    const buf = await this.docBytes();
    box.innerHTML = '';
    await window.docx.renderAsync(buf, box, null, {
      className: 'gregal-docx', inWrapper: true, ignoreWidth: true,
      ignoreHeight: true, ignoreFonts: false, breakPages: true,
      renderHeaders: true, renderFooters: true, renderFootnotes: true,
    });
  },
  async renderXlsx(box) {
    await this.ensureLib('xlsx', ['xlsx.mini.min.js']);
    const buf = await this.docBytes();
    const wb = window.XLSX.read(buf, { type: 'array' });
    if (!wb.SheetNames.length) throw new Error('sense fulls');
    const d = this.doc;
    if (!d.sheet || !wb.SheetNames.includes(d.sheet)) d.sheet = wb.SheetNames[0];
    const ws = wb.Sheets[d.sheet];
    const rows = window.XLSX.utils.sheet_to_json(ws, { header: 1, raw: true, defval: '' });
    // Prou columnes per al full, no 20 fixes: un pressupost en té trenta i
    // les que sobraven es perdien sense dir-ho.
    const MAXCOL = 60, MAXFILA = 800;
    const cols = Math.min(MAXCOL, rows.reduce((n, r) => Math.max(n, r.length), 0));
    const visibles = rows.slice(0, MAXFILA);

    let html = '<div class="doc-scroll"><table class="doc-grid"><thead><tr><th></th>';
    for (let c = 0; c < cols; c++) html += '<th>' + lletraCol(c) + '</th>';
    html += '</tr></thead><tbody>';
    visibles.forEach((cells, i) => {
      html += '<tr><th>' + (i + 1) + '</th>';
      for (let c = 0; c < cols; c++) {
        const v = cells[c];
        const buit = v == null || v === '';
        const num = !buit && typeof v === 'number';
        html += '<td class="' + (buit ? 'buida' : num ? 'num' : '') + '">' + esc(buit ? '' : String(v)) + '</td>';
      }
      html += '</tr>';
    });
    html += '</tbody></table></div>';
    const avisos = [];
    if (rows.length > MAXFILA) avisos.push(rows.length + ' files (en mostro ' + MAXFILA + ')');
    if (rows.some(r => r.length > MAXCOL)) avisos.push('més de ' + MAXCOL + ' columnes');
    if (avisos.length) html += '<p class="off-open">… ' + avisos.join(' · ') + '. L\'agent llegeix el full sencer.</p>';
    box.innerHTML = rows.length ? html : '<p class="off-open">(full buit)</p>';

    // Les pestanyes viuen fora de la zona que fa scroll: sempre a la vista,
    // com a l'Excel. Surten encara que n'hi hagi una de sola: saber com es
    // diu el full és part de llegir-lo.
    this.pintaFulls(wb.SheetNames, d.sheet);
  },

  pintaFulls(noms, actual) {
    const barra = $('docSheets');
    if (!barra) return;
    barra.hidden = !noms || !noms.length;
    if (barra.hidden) return;
    barra.innerHTML = noms.map(n =>
      '<button role="tab" aria-selected="' + (n === actual) + '" class="' + (n === actual ? 'on' : '') +
      '" data-sh="' + esc(n) + '" title="' + esc(n) + '">' + esc(n) + '</button>').join('');
    barra.querySelectorAll('[data-sh]').forEach(b => b.onclick = () => {
      this.doc.sheet = b.dataset.sh;
      this.paintDoc();
    });
  },
  // renderPptx pinta la diapo actual en canvas amb navegació. Chart.js cal
  // en carregar la llibreria (sense ell el factory peta en silenci).
  async renderPptx(box) {
    await this.ensureLib('pptx', ['jszip.min.js', 'chart.umd.min.js', 'PptxViewJS.min.js']);
    if (!window.PptxViewJS || typeof window.PptxViewJS.PPTXViewer !== 'function') {
      throw new Error('visor pptx no disponible');
    }
    const buf = await this.docBytes();
    box.innerHTML = '<div class="pptx-nav"><button data-prev aria-label="Diapo anterior">‹</button>' +
      '<span data-count></span><button data-next aria-label="Diapo següent">›</button></div>' +
      '<canvas class="pptx-canvas" width="960" height="540"></canvas>';
    const canvas = box.querySelector('canvas'), count = box.querySelector('[data-count]');
    const v = new window.PptxViewJS.PPTXViewer({ canvas });
    await v.loadFile(buf);
    const total = v.getSlideCount();
    if (!total) throw new Error('sense diapos');
    const show = async idx => {
      await v.render(canvas, { slideIndex: idx });
      count.textContent = (v.getCurrentSlideIndex() + 1) + ' / ' + total;
    };
    box.querySelector('[data-prev]').onclick = async () => { await v.previousSlide(canvas); await v.render(canvas); count.textContent = (v.getCurrentSlideIndex() + 1) + ' / ' + total; };
    box.querySelector('[data-next]').onclick = async () => { await v.nextSlide(canvas); await v.render(canvas); count.textContent = (v.getCurrentSlideIndex() + 1) + ' / ' + total; };
    await show(0);
  },
  minimize() {
    if (!this.doc) return;
    const pane = $('docPane');
    if (!pane) return;
    pane.classList.add('minimized');
    document.body.classList.add('doc-minimized');
    if ($('docMinName')) $('docMinName').textContent = this.doc.name;
    if ($('docMinKind')) $('docMinKind').textContent = KIND_LABEL[this.doc.kind] || this.doc.kind || '';
    const icon = this.doc.kind === 'xlsx' ? '📊' : this.doc.kind === 'pptx' ? '📽️' : '📄';
    if ($('docMinIcon')) $('docMinIcon').textContent = icon;
  },
  restore() {
    const pane = $('docPane');
    if (pane) pane.classList.remove('minimized');
    document.body.classList.remove('doc-minimized');
  },
  async chatSession() {
    if (!this.doc) return;
    G().setView('agent');
    const P = window.gregalPanels;
    let existent = null;
    if (P && P.convs && P.convs.items) {
      existent = P.convs.items.find(c => (c.title || c.name || '').includes(this.doc.name));
    }
    if (existent && P.convs.obre) {
      await P.convs.obre(existent.name);
      this.msgPane(message('office.sessionResumed', { name: this.doc.name }));
    } else {
      if (G().newChat) await G().newChat();
      const input = $('in');
      if (input) {
        input.value = message('office.reviewPrompt', { path: this.doc.path });
        input.dispatchEvent(new Event('input'));
        input.focus();
      }
      this.msgPane(message('office.chatOpened', { name: this.doc.name }));
    }
  },
  close() {
    this.restore();
    $('docPane').hidden = true;
    document.body.classList.remove('doc-open');
    $('docPane')?.classList.remove('doc-text-file');
    const fulls = $('docSheets');
    if (fulls) { fulls.hidden = true; fulls.innerHTML = ''; }
  },
  open(id) {
    const d = this.recent.find(r => r.id === id); if (!d) return;
    this.doc = d; this.read().then(() => this.openPane());
  },
  async openTextFile(path) {
    try {
      const r = await G().api('/api/file?path=' + encodeURIComponent(path));
      if (!r.ok) throw new Error(await r.text());
      const file = await r.json();
      if (file.binary || file.too_big) throw new Error(T(file.binary ? 'office.binaryFile' : 'office.displayTooLarge'));
      this.doc = { kind: 'text', name: path.split(/[\\/]/).pop() || path, path, text: String(file.content || ''), view: 'text' };
      this.openPane();
    } catch (e) { G().sys(message('office.openFailed', { error: e.message })); }
  },
  // Vista HTML del text extret: seccions ([FULL]/- Diapo N -), titulars
  // ([Heading1]/[Títol]) i taules (files amb " | "). Tot escapant HTML.
  preview(text) {
    const lines = String(text || '').split('\n');
    let html = '', table = [];
    const flushTable = () => {
      if (!table.length) return;
      html += '<table class="doc-table"><tbody>';
      table.forEach(cells => {
        html += '<tr>' + cells.map(c => '<td>' + esc(c) + '</td>').join('') + '</tr>';
      });
      html += '</tbody></table>';
      table = [];
    };
    lines.forEach(raw => {
      const l = raw.replace(/\s+$/, '');
      if (!l.trim()) { flushTable(); return; }
      let m;
      if ((m = l.match(/^\[(FULL[^\]]*)\]\s*(.*)$/)) || (m = l.match(/^-\s*(Diapo\s+\d+)\s*-?\s*(.*)$/))) {
        flushTable();
        html += '<h4 class="doc-sec">' + esc(m[1]) + (m[2] ? ' · ' + esc(m[2]) : '') + '</h4>';
      } else if ((m = l.match(/^\[([^\]]+)\]\s*(.*)$/))) {
        flushTable();
        if (/head|títol|titol|title/i.test(m[1])) html += '<h3>' + esc(m[2] || m[1]) + '</h3>';
        else html += '<p>' + esc(l) + '</p>';
      } else if (l.includes(' | ')) {
        table.push(l.split(' | ').slice(0, 20));
      } else {
        flushTable();
        html += '<p>' + esc(l) + '</p>';
      }
    });
    flushTable();
    return html || '<p class="off-open">' + esc(T('office.noReadableText')) + '</p>';
  },
  // openApp obre el document amb el Word/Excel/PowerPoint de debò (només
  // desktop: el fitxer és al servidor local). Mentre sigui obert, Windows
  // bloqueja l'escriptura: cal tancar-lo per tornar a editar.
  async openApp() {
    if (!this.doc) return;
    try {
      const r = await G().api('/api/office/open', { method: 'POST', body: JSON.stringify({ id: this.doc.id }) });
      if (!r.ok) { this.msgPane(await r.text()); return; }
      const d = await r.json();
      this.msgPane((d.result || T('office.opened')) + '.');
      G().activityItem('office', message('office.openFile', { name: this.doc.name }), T('office.associatedApp'), 'ok');
    } catch (e) { this.msgPane(String(e.message || e)); }
  },
  // ask envia la petició a l'agent amb el document adjunt (@ruta): el
  // servidor n'adjunta el text al prompt i l'agent hi pot escriure amb
  // office_edit. La subfinestra es queda oberta: veus el document mentre
  // l'agent treballa i es rellegeix quan acaba. Si el tens obert al Word,
  // l'edició fallarà bloquejada: tanca'l primer.
  ask(text) {
    text = (text || '').trim();
    if (!text || !this.doc) return;
    if (text.includes('___')) { this.msgPane(T('office.fillPlaceholders')); return; }
    G().setView('agent');
    const input = $('in');
    input.value = text + ' @' + this.doc.path;
    $('docAsk').value = '';
    $('send').click();
    this.msgPane(T('office.requested'));
  },
  msgPane(t) { const el = $('docMsg'); el.textContent = t || ''; if (t) setTimeout(() => { if (el.textContent === t) el.textContent = ''; }, 6000); },
  // refresh es crida quan acaba un torn: si l'agent ha tocat el document, es
  // veu de seguida.
  async refresh() {
    if (!this.doc || $('docPane').hidden) return;
    const before = this.doc.text;
    await this.read();
    this.paintDoc();
    if (this.doc.text !== before) this.msgPane(T('office.updated'));
  },
  async download() {
    if (!this.doc) return;
    try {
      const r = await G().api('/api/office/download?id=' + encodeURIComponent(this.doc.id));
      if (!r.ok) { this.msgPane(await r.text()); return; }
      const blob = await r.blob();
      const a = document.createElement('a');
      a.href = URL.createObjectURL(blob); a.download = this.doc.name || 'document';
      document.body.appendChild(a); a.click(); a.remove();
      setTimeout(() => URL.revokeObjectURL(a.href), 5000);
      this.msgPane(T('office.downloaded'));
    } catch (e) { this.msgPane(String(e.message || e)); }
  },
  renderRecent() {
    const el = $('offRecent'); if (!el) return;
    if (!this.recent.length) { el.innerHTML = '<div class="activity-empty">' + T('office.buit') + '</div>'; return; }
    el.innerHTML = this.recent.map(d =>
      '<button type="button" class="off-row" data-id="' + esc(d.id) + '"><span class="off-kind">' + esc(KIND_LABEL[d.kind] || d.kind) + '</span><b>' + esc(d.name) + '</b><span class="off-open">' + esc(T('office.open')) + ' ›</span></button>').join('');
    el.querySelectorAll('.off-row').forEach(b => b.onclick = () => this.open(b.dataset.id));
  },
};
