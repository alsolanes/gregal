// Grafs: l'editor visual dels procediments.
//
// Un graf és «com es fa això aquí» dibuixat: passos, fletxes i, a cada
// fletxa, quan s'agafa. El motor és a internal/flow; aquí només hi ha el
// llenç, i el llenç desa exactament el que el motor sap llegir —per això
// els camps del panell de la dreta són els del node de Go i prou.
//
// El que NO fa, a posta: no hi ha subgrafs, ni paral·lel, ni variables
// pròpies. Tot això vol dir inventar un llenguatge, i el llenguatge ja el
// tenim: un pas d'agent. El graf és per a l'ordre i el rastre, no per a
// programar-hi.

const $ = id => document.getElementById(id);
const G = () => window.gregal;
const esc = s => String(s == null ? '' : s)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

async function j(url, opts) {
  const r = await G().api(url, opts);
  if (!r.ok) throw new Error((await r.text()).trim() || ('HTTP ' + r.status));
  return r.json();
}

const AMPLE = 210, ALT = 74;          // mida d'un pas al llenç
const EINES = ['bash', 'read', 'write', 'edit', 'grep', 'glob', 'ls', 'web_search', 'web_fetch'];

// Cada mena té color propi: es distingeixen d'un cop d'ull sense llegir.
const MENES = {
  agent: { nom: 'Agent', color: 'var(--accent)', fons: 'var(--accent-soft)', icona: '◆' },
  tool:  { nom: 'Eina',  color: 'var(--onada)',  fons: 'var(--surface-hi)',  icona: '▸' },
  note:  { nom: 'Nota',  color: 'var(--lila)',   fons: 'var(--lila-fons)',   icona: '·' },
};

// puntCorba torna el punt de la corba cúbica en t (0..1).
function puntCorba(x1, y1, cx1, cy1, cx2, cy2, x2, y2, t) {
  const u = 1 - t;
  const a = u * u * u, b = 3 * u * u * t, c = 3 * u * t * t, e = t * t * t;
  return [a * x1 + b * cx1 + c * cx2 + e * x2, a * y1 + b * cy1 + c * cy2 + e * y2];
}

export const flows = {
  flow: null,          // graf obert
  nomDesat: '',        // nom amb què és al disc (per renombrar sense duplicar)
  sel: null,           // id del pas seleccionat
  selEdge: null,       // índex de la fletxa seleccionada
  brut: false,         // hi ha canvis sense desar
  execucio: null,      // {abort, estats:{id:'run'|'ok'|'err'}, sortides:{}}

  async init() {
    if (this.llest) return;
    this.llest = true;
    $('flowEines').innerHTML = EINES.map(e => `<option value="${e}"></option>`).join('');
    $('flowNou').onclick = () => this.nou();
    $('flowDesa').onclick = () => this.desa();
    $('flowExecuta').onclick = () => this.executa();
    $('flowEsborra').onclick = () => this.esborra();
    $('flowNom').oninput = () => { this.flow.name = $('flowNom').value; this.marca(); };
    $('flowDesc').oninput = () => { this.flow.desc = $('flowDesc').value; this.marca(); };
    document.querySelectorAll('[data-afegeix]').forEach(b => {
      b.onclick = () => this.afegeix(b.dataset.afegeix);
    });
    $('flowCanvas').addEventListener('pointerdown', e => {
      if (e.target === $('flowCanvas') || e.target.id === 'flowEdges') this.tria(null);
    });
    // Suprimir esborra el que estigui triat, com a qualsevol editor.
    $('grafsPage').addEventListener('keydown', e => {
      if (e.key !== 'Delete' && e.key !== 'Backspace') return;
      if (/^(INPUT|TEXTAREA|SELECT)$/.test(e.target.tagName)) return;
      if (this.selEdge != null) { this.flow.edges.splice(this.selEdge, 1); this.selEdge = null; this.marca(); this.pinta(); }
      else if (this.sel) this.esborraPas(this.sel);
    });
    // Sortir amb canvis sense desar era perdre'ls en silenci.
    window.addEventListener('beforeunload', e => {
      if (this.brut) { e.preventDefault(); e.returnValue = ''; }
    });
    this.nou();
  },

  async load() {
    await this.init();
    await this.llista();
  },

  // ------------------------------------------------------------- llistat

  async llista() {
    const cont = $('flowList');
    try {
      const d = await j('/api/flows');
      const ll = d.flows || [];
      if (!ll.length) {
        cont.innerHTML = '<p class="page-copy" style="padding:8px 2px">Encara no n\'hi ha cap. Els que facis es desaran a <code>' + esc(d.dir) + '</code> i viatjaran amb el repositori.</p>';
        return;
      }
      cont.innerHTML = ll.map(f => `
        <button class="flow-item${f.name === this.nomDesat ? ' on' : ''}" data-obre="${esc(f.name)}">
          <b>${esc(f.name)}</b>
          <span>${esc(f.desc || '')}</span>
          <em>${f.steps} pas${f.steps === 1 ? '' : 'sos'}</em>
        </button>`).join('');
      cont.querySelectorAll('[data-obre]').forEach(b => b.onclick = () => this.obre(b.dataset.obre));
    } catch (e) {
      cont.innerHTML = '<p class="page-copy" style="color:var(--red)">' + esc(e.message) + '</p>';
    }
  },

  async obre(nom) {
    if (!this.confirmaPerdre()) return;
    try {
      const d = await j('/api/flows/load?name=' + encodeURIComponent(nom));
      this.flow = d.flow;
      this.flow.nodes = this.flow.nodes || [];
      this.flow.edges = this.flow.edges || [];
      this.nomDesat = this.flow.name;
      this.sel = this.selEdge = null;
      this.brut = false;
      this.execucio = null;
      this.pinta(); this.llista();
    } catch (e) { this.diu(e.message, true); }
  },

  nou() {
    if (this.llest && !this.confirmaPerdre()) return;
    this.flow = { name: '', desc: '', nodes: [], edges: [] };
    this.nomDesat = '';
    this.sel = this.selEdge = null;
    this.brut = false;
    this.execucio = null;
    this.pinta();
  },

  confirmaPerdre() {
    if (this.corrent && !confirm('Hi ha un graf executant-se. L\'aturo?')) return false;
    if (this.corrent) this.corrent.abort();
    return !this.brut || confirm('Hi ha canvis sense desar al graf. Els vols perdre?');
  },

  marca() { this.brut = true; $('flowEstat').textContent = 'sense desar'; },

  diu(text, mal) {
    const el = $('flowEstat');
    el.textContent = text;
    el.style.color = mal ? 'var(--red)' : 'var(--muted)';
    if (mal) el.title = text;
  },

  // -------------------------------------------------------------- edició

  idLliure(base) {
    let n = 1, id = base;
    while (this.flow.nodes.some(x => x.id === id)) id = base + (++n);
    return id;
  },

  afegeix(kind) {
    // Cau sota l'últim, no a sobre: afegir-ne cinc seguits no els ha
    // d'apilar al mateix punt.
    const ult = this.flow.nodes[this.flow.nodes.length - 1];
    const node = {
      id: this.idLliure(kind === 'tool' ? 'eina' : kind === 'note' ? 'nota' : 'pas'),
      kind, title: '', x: ult ? ult.x : 60, y: ult ? ult.y + ALT + 56 : 40,
    };
    if (kind === 'tool') { node.tool = 'bash'; node.args = '{"command":""}'; }
    if (kind === 'agent') node.task = '';
    this.flow.nodes.push(node);
    // Enganxa-la a l'últim: el 90% de les vegades és el que vols, i si no
    // es treu amb un clic.
    if (ult) this.flow.edges.push({ from: ult.id, to: node.id });
    this.sel = node.id; this.selEdge = null;
    this.marca(); this.pinta();
    const camp = $('flowInsp').querySelector('input, textarea');
    if (camp) camp.focus();
  },

  esborraPas(id) {
    this.flow.nodes = this.flow.nodes.filter(n => n.id !== id);
    this.flow.edges = this.flow.edges.filter(e => e.from !== id && e.to !== id);
    this.sel = null;
    this.marca(); this.pinta();
  },

  tria(id, iEdge) {
    this.sel = id || null;
    this.selEdge = iEdge == null ? null : iEdge;
    this.pinta();
  },

  node(id) { return this.flow.nodes.find(n => n.id === id); },

  // --------------------------------------------------------------- pinta

  pinta() {
    $('flowNom').value = this.flow.name || '';
    $('flowDesc').value = this.flow.desc || '';
    $('flowEsborra').disabled = !this.nomDesat;
    $('flowExecuta').disabled = !this.nomDesat || this.brut;
    $('flowExecuta').title = this.brut ? 'Desa els canvis abans d\'executar' : 'Executa el graf';
    this.pintaLlenc();
    this.pintaInspector();
  },

  pintaLlenc() {
    const c = $('flowCanvas');
    const buit = !this.flow.nodes.length;
    $('flowBuit').hidden = !buit;

    // Prou llenç per al pas més llunyà, amb marge per arrossegar-hi més.
    const maxX = Math.max(600, ...this.flow.nodes.map(n => n.x + AMPLE)) + 220;
    const maxY = Math.max(400, ...this.flow.nodes.map(n => n.y + ALT)) + 220;
    c.style.width = maxX + 'px';
    c.style.height = maxY + 'px';

    const arrencada = this.arrencada();
    c.querySelectorAll('.flow-node').forEach(el => el.remove());
    for (const n of this.flow.nodes) {
      const m = MENES[n.kind] || MENES.note;
      const est = this.execucio ? this.execucio.estats[n.id] : null;
      const el = document.createElement('div');
      el.className = 'flow-node' + (this.sel === n.id ? ' sel' : '') + (est ? ' e-' + est : '');
      el.style.cssText = `left:${n.x}px; top:${n.y}px; --mena:${m.color}; --mena-fons:${m.fons}`;
      el.dataset.id = n.id;
      el.innerHTML = `
        <div class="fn-cap">
          <span class="fn-mena">${m.icona} ${m.nom}</span>
          ${n.id === arrencada ? '<span class="fn-tag">inici</span>' : ''}
          ${est === 'run' ? '<span class="fn-tag viu">executant…</span>' : ''}
          ${est === 'ok' ? '<span class="fn-tag ok">fet</span>' : ''}
          ${est === 'err' ? '<span class="fn-tag mal">error</span>' : ''}
        </div>
        <div class="fn-nom">${esc(n.title || n.id)}</div>
        <div class="fn-sub">${esc(this.resum(n))}</div>
        <button class="fn-port" title="Arrossega d'aquí a un altre pas per connectar-los" aria-label="Connecta"></button>`;
      c.appendChild(el);
      this.arrossega(el, n);
    }
    this.pintaFletxes();
  },

  resum(n) {
    if (n.kind === 'tool') return (n.tool || '?') + ' ' + (n.args || '');
    if (n.kind === 'note') return n.text || '';
    return n.task || 'sense instruccions';
  },

  // arrencada és el pas sense cap fletxa d'entrada: el mateix criteri que
  // fa servir el motor, perquè el dibuix no digui una cosa i l'execució
  // en faci una altra.
  arrencada() {
    const entren = new Set(this.flow.edges.map(e => e.to));
    const caps = this.flow.nodes.filter(n => !entren.has(n.id));
    return caps.length === 1 ? caps[0].id : null;
  },

  pintaFletxes() {
    const svg = $('flowEdges');
    const c = $('flowCanvas');
    svg.setAttribute('width', c.style.width.replace('px', ''));
    svg.setAttribute('height', c.style.height.replace('px', ''));
    let html = `<defs>
      <marker id="fa" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
        <path d="M0 0 L10 5 L0 10 z" fill="var(--line-hi)"/>
      </marker>
      <marker id="fa-sel" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
        <path d="M0 0 L10 5 L0 10 z" fill="var(--accent)"/>
      </marker>
    </defs>`;
    this.flow.edges.forEach((e, i) => {
      const a = this.node(e.from), b = this.node(e.to);
      if (!a || !b) return;
      const x1 = a.x + AMPLE / 2, y1 = a.y + ALT;
      const x2 = b.x + AMPLE / 2, y2 = b.y;
      // Corba en S: dues fletxes entre els mateixos dos passos no se
      // superposen i es veu cap on va cadascuna.
      const d = Math.max(40, Math.abs(y2 - y1) / 2);
      const sel = this.selEdge === i;
      html += `<path d="M${x1} ${y1} C ${x1} ${y1 + d}, ${x2} ${y2 - d}, ${x2} ${y2}"
        fill="none" stroke="${sel ? 'var(--accent)' : 'var(--line-hi)'}" stroke-width="${sel ? 2.5 : 1.75}"
        marker-end="url(#${sel ? 'fa-sel' : 'fa'})" class="flow-edge" data-edge="${i}"/>`;
      // L'etiqueta no va al mig sinó a prop de la sortida: al mig sovint
      // cau just a sobre d'un pas i, com que els passos són HTML damunt de
      // l'SVG, quedava tapada. Les que surten del mateix pas s'escalonen,
      // que si no dues condicions se superposen i no se'n llegeix cap.
      const germans = this.flow.edges.filter(x => x.from === e.from);
      const ordre = germans.indexOf(e);
      const [mx, my] = puntCorba(x1, y1, x1, y1 + d, x2, y2 - d, x2, y2, 0.26 + 0.2 * ordre);
      const txt = e.when ? e.when : 'sempre';
      html += `<g class="flow-cond${sel ? ' sel' : ''}" data-edge="${i}" transform="translate(${mx},${my})">
        <rect x="${-(txt.length * 3.6 + 10)}" y="-10" width="${txt.length * 7.2 + 20}" height="20" rx="10"></rect>
        <text text-anchor="middle" dy="4">${esc(txt)}</text>
      </g>`;
    });
    svg.innerHTML = html;
    svg.querySelectorAll('[data-edge]').forEach(el => {
      el.addEventListener('pointerdown', ev => { ev.stopPropagation(); this.tria(null, +el.dataset.edge); });
    });
  },

  // arrossega mou un pas i, des del port, dibuixa una fletxa nova.
  arrossega(el, n) {
    const port = el.querySelector('.fn-port');
    el.addEventListener('pointerdown', ev => {
      if (ev.button !== 0) return;
      ev.stopPropagation();
      const desDelPort = ev.target === port;
      this.tria(n.id);
      const c = $('flowCanvas');
      const r0 = c.getBoundingClientRect();
      const dx = ev.clientX - r0.left - n.x, dy = ev.clientY - r0.top - n.y;
      let mogut = false;
      const temp = desDelPort ? document.createElementNS('http://www.w3.org/2000/svg', 'path') : null;
      if (temp) { temp.setAttribute('class', 'flow-temp'); $('flowEdges').appendChild(temp); }

      const mou = e2 => {
        mogut = true;
        const x = e2.clientX - r0.left, y = e2.clientY - r0.top;
        if (desDelPort) {
          const x1 = n.x + AMPLE / 2, y1 = n.y + ALT;
          temp.setAttribute('d', `M${x1} ${y1} C ${x1} ${y1 + 50}, ${x} ${y - 50}, ${x} ${y}`);
          el.classList.add('connectant');
          this.ressalta(x, y, n.id);
        } else {
          n.x = Math.max(0, Math.round((x - dx) / 10) * 10);
          n.y = Math.max(0, Math.round((y - dy) / 10) * 10);
          el.style.left = n.x + 'px'; el.style.top = n.y + 'px';
          this.pintaFletxes();
        }
      };
      const deixa = e2 => {
        window.removeEventListener('pointermove', mou);
        window.removeEventListener('pointerup', deixa);
        el.classList.remove('connectant');
        document.querySelectorAll('.flow-node.diana').forEach(x => x.classList.remove('diana'));
        if (temp) temp.remove();
        if (!mogut) return;
        if (desDelPort) {
          const dest = this.sotaCursor(e2.clientX, e2.clientY, n.id);
          if (dest) this.connecta(n.id, dest);
        } else {
          this.marca(); this.pinta();
        }
      };
      window.addEventListener('pointermove', mou);
      window.addEventListener('pointerup', deixa);
    });
  },

  sotaCursor(cx, cy, excepte) {
    for (const el of document.querySelectorAll('.flow-node')) {
      if (el.dataset.id === excepte) continue;
      const r = el.getBoundingClientRect();
      if (cx >= r.left && cx <= r.right && cy >= r.top && cy <= r.bottom) return el.dataset.id;
    }
    return null;
  },

  ressalta(x, y, excepte) {
    const c = $('flowCanvas').getBoundingClientRect();
    const dest = this.sotaCursor(c.left + x, c.top + y, excepte);
    document.querySelectorAll('.flow-node').forEach(el => el.classList.toggle('diana', el.dataset.id === dest));
  },

  connecta(from, to) {
    if (this.flow.edges.some(e => e.from === from && e.to === to)) return;
    this.flow.edges.push({ from, to });
    this.selEdge = this.flow.edges.length - 1;
    this.marca(); this.pinta();
  },

  // ---------------------------------------------------------- inspector

  pintaInspector() {
    const box = $('flowInsp');
    if (this.selEdge != null) { box.hidden = false; box.innerHTML = this.inspFletxa(); this.lligaInspector(); return; }
    const n = this.sel && this.node(this.sel);
    // Sense res triat l'inspector desapareix i el llenç recupera l'amplada:
    // un panell buit ocupant un terç de la pantalla no dona informació.
    if (!n) { box.hidden = true; box.innerHTML = ''; return; }
    box.hidden = false;
    const camp = (et, id, valor, pista) =>
      `<label class="fi-camp"><span>${et}</span><input id="${id}" value="${esc(valor || '')}" placeholder="${esc(pista || '')}"></label>`;
    const area = (et, id, valor, pista, files) =>
      `<label class="fi-camp"><span>${et}</span><textarea id="${id}" rows="${files || 5}" placeholder="${esc(pista || '')}">${esc(valor || '')}</textarea></label>`;

    let cos = camp('Identificador', 'fiId', n.id, 'lletres, xifres, - i _') +
      camp('Títol', 'fiTitol', n.title, 'el que es llegeix al pas');
    if (n.kind === 'agent') {
      cos += area('Què ha de fer', 'fiTask', n.task, 'Fes servir {{id_del_pas}} per agafar el que ha sortit abans') +
        `<label class="fi-camp"><span>Mode</span><select id="fiMode">
          <option value=""${!n.mode ? ' selected' : ''}>el de la sessió</option>
          <option value="code"${n.mode === 'code' ? ' selected' : ''}>codi (pot tocar fitxers)</option>
          <option value="chat"${n.mode === 'chat' ? ' selected' : ''}>xat (només respon)</option>
        </select></label>` +
        camp('Passos màxims', 'fiMax', n.max_steps || '', '0 = el de la configuració');
    } else if (n.kind === 'tool') {
      cos += `<label class="fi-camp"><span>Eina</span><input id="fiTool" value="${esc(n.tool || '')}" list="flowEines"></label>` +
        area('Arguments (JSON)', 'fiArgs', n.args, '{"command":"go test ./..."}', 4);
    } else {
      cos += area('Text', 'fiText', n.text, 'una nota que es passa tal qual al pas següent', 4);
    }
    cos += camp('Desa el resultat a', 'fiOut', n.out, 'per defecte, l\'identificador');

    box.innerHTML = `<div class="fi-cap">${MENES[n.kind].icona} ${MENES[n.kind].nom}
      <button class="fi-esborra" id="fiEsborra">Esborra el pas</button><button class="fi-tanca" id="fiTanca" aria-label="Tanca l'inspector">×</button></div>${cos}${this.sortidaDe(n.id)}`;
    this.lligaInspector();
  },

  inspFletxa() {
    const e = this.flow.edges[this.selEdge];
    return `<div class="fi-cap">Fletxa: ${esc(e.from)} → ${esc(e.to)}
        <button class="fi-esborra" id="fiEsborraFletxa">Treu la fletxa</button><button class="fi-tanca" id="fiTancaF" aria-label="Tanca l'inspector">×</button></div>
      <label class="fi-camp"><span>Quan s'agafa</span>
        <input id="fiWhen" value="${esc(e.when || '')}" placeholder="buit = sempre" list="flowConds"></label>
      <p class="page-copy" style="margin-top:4px">
        <code>clau</code> si té valor · <code>!clau</code> si no en té ·
        <code>clau == "x"</code> · <code>clau != "x"</code> · <code>clau conté "x"</code>.<br>
        Les claus són els identificadors dels passos, més <code>last</code> i
        <code>&lt;id&gt;.error</code>.<br><br>
        Si un pas <b>falla</b>, només se segueixen les fletxes que tinguin una
        condició escrita. Si no n'hi ha cap, el graf s'atura i diu on.
      </p>`;
  },

  sortidaDe(id) {
    const out = this.execucio && this.execucio.sortides[id];
    if (!out) return '';
    return `<div class="fi-sortida"><span>Última sortida</span><pre>${esc(out)}</pre></div>`;
  },

  lligaInspector() {
    const set = (id, fn) => { const el = $(id); if (el) el.oninput = () => { fn(el.value); this.marca(); }; };
    const n = this.sel && this.node(this.sel);
    if (this.selEdge != null) {
      set('fiWhen', v => { this.flow.edges[this.selEdge].when = v.trim(); this.pintaFletxes(); });
      const b = $('fiEsborraFletxa');
      if (b) b.onclick = () => { this.flow.edges.splice(this.selEdge, 1); this.selEdge = null; this.marca(); this.pinta(); };
      const t = $('fiTancaF');
      if (t) t.onclick = () => this.tria(null);
      return;
    }
    if (!n) return;
    // L'id canvia a l'acabar d'escriure, no a cada tecla: si no, cada
    // lletra reescriuria les fletxes i et quedaries sense connexions.
    const idIn = $('fiId');
    if (idIn) idIn.onchange = () => {
      const nou = idIn.value.trim();
      if (!nou || nou === n.id) { idIn.value = n.id; return; }
      if (this.flow.nodes.some(x => x.id === nou)) { this.diu('Ja hi ha un pas amb l\'id «' + nou + '»', true); idIn.value = n.id; return; }
      this.flow.edges.forEach(e => { if (e.from === n.id) e.from = nou; if (e.to === n.id) e.to = nou; });
      const vell = n.id;
      n.id = nou; this.sel = nou;
      this.diu('L\'id ha canviat: repassa els {{' + vell + '}} de les instruccions');
      this.marca(); this.pinta();
    };
    set('fiTitol', v => { n.title = v; this.pintaLlenc(); });
    set('fiTask', v => { n.task = v; this.pintaLlenc(); });
    set('fiText', v => { n.text = v; this.pintaLlenc(); });
    set('fiTool', v => { n.tool = v.trim(); this.pintaLlenc(); });
    set('fiArgs', v => { n.args = v; this.pintaLlenc(); });
    set('fiOut', v => { n.out = v.trim(); });
    set('fiMax', v => { n.max_steps = parseInt(v, 10) || 0; });
    const mode = $('fiMode');
    if (mode) mode.onchange = () => { n.mode = mode.value; this.marca(); };
    const del = $('fiEsborra');
    if (del) del.onclick = () => this.esborraPas(n.id);
    const tanca = $('fiTanca');
    if (tanca) tanca.onclick = () => this.tria(null);
  },

  // ------------------------------------------------------------- disc

  async desa() {
    if (!(this.flow.name || '').trim()) { this.diu('Posa-li un nom al graf', true); $('flowNom').focus(); return; }
    try {
      const d = await j('/api/flows/save', {
        method: 'POST',
        body: JSON.stringify({ flow: this.flow, rename: this.nomDesat }),
      });
      this.nomDesat = this.flow.name;
      this.brut = false;
      this.diu('desat a ' + d.path.split(/[\\/]/).slice(-2).join('/'));
      this.pinta();
      await this.llista();
    } catch (e) {
      // El servidor valida amb el mateix codi que executa: el que digui
      // aquí és exactament el que passaria en executar-lo.
      this.diu(e.message, true);
      // Si el motiu anomena un pas, tria'l: llegir «el pas "pas3" no diu
      // què ha de fer» i haver-lo de buscar al llenç sobrava.
      const culpable = [...e.message.matchAll(/"([^"]+)"/g)].map(m => m[1]).find(id => this.node(id));
      if (culpable) this.tria(culpable);
    }
  },

  async esborra() {
    if (!confirm('Esborro el graf «' + this.nomDesat + '»?')) return;
    try {
      await j('/api/flows/delete', { method: 'POST', body: JSON.stringify({ name: this.nomDesat }) });
      this.nou(); this.llista();
    } catch (e) { this.diu(e.message, true); }
  },

  // --------------------------------------------------------- execució

  async executa() {
    // Mentre corre, el mateix botó atura: un botó per a un estat.
    if (this.corrent) { this.corrent.abort(); return; }
    const auto = $('flowAuto').checked;
    if (auto && !confirm('El graf s\'executarà sense demanar permís a cada eina (les denegades continuen bloquejades). Endavant?')) return;

    const ctrl = new AbortController();
    this.corrent = ctrl;
    this.execucio = { estats: {}, sortides: {} };
    $('flowExecuta').textContent = 'Atura';
    $('flowExecuta').disabled = false;
    $('flowRegistre').hidden = false;
    $('flowRegistre').innerHTML = '';
    const anota = (txt, mena) => {
      const l = document.createElement('div');
      l.className = 'flow-log' + (mena ? ' ' + mena : '');
      l.innerHTML = txt;
      $('flowRegistre').appendChild(l);
      $('flowRegistre').scrollTop = $('flowRegistre').scrollHeight;
    };

    // El següent a executar es marca com a «executant»: si no, entre pas i
    // pas el llenç es queda quiet i sembla penjat.
    let seguent = this.arrencada();
    if (seguent) { this.execucio.estats[seguent] = 'run'; this.pintaLlenc(); }

    const url = '/api/flows/run?name=' + encodeURIComponent(this.nomDesat) + (auto ? '&auto=1' : '');
    try {
      await this.sse(url, ctrl.signal, (ev, d) => {
        if (ev === 'start') {
          anota('Executant <b>' + esc(d.flow) + '</b> · ' + d.steps + ' passos' + (d.auto ? ' · sense demanar permís' : ''));
        } else if (ev === 'step') {
          this.execucio.estats[d.node] = d.error ? 'err' : 'ok';
          this.execucio.sortides[d.node] = d.error ? d.error : d.output;
          anota('<b>' + esc(d.title || d.node) + '</b> · ' + (d.ms / 1000).toFixed(1) + 's' +
            (d.error ? ' · <span class="mal">' + esc(d.error) + '</span>' : ' · fet'), d.error ? 'mal' : 'ok');
          this.pintaLlenc(); this.pintaInspector();
        } else if (ev === 'done') {
          if (d.stopped) anota('Aturat: ' + esc(d.stopped), 'mal');
          else if (d.error) anota('Error: ' + esc(d.error), 'mal');
          else anota('Fet: ' + d.steps + ' passos.', 'ok');
          if (d.answer) anota('<pre>' + esc(d.answer.slice(0, 4000)) + '</pre>');
          // El servidor ho deixa a la conversa perquè el model ho tingui al
          // torn següent; posa-ho també a la pantalla, que si no la conversa
          // es contradiu amb el que l'agent recorda.
          this.aLaConversa(d);
        }
      });
    } catch (e) {
      if (e.name !== 'AbortError') anota('Error: ' + esc(e.message), 'mal');
    } finally {
      // Els que quedaven marcats «executant» no s'han arribat a fer: si es
      // queden encesos, el llenç menteix sobre què ha passat.
      for (const k of Object.keys(this.execucio.estats)) {
        if (this.execucio.estats[k] === 'run') delete this.execucio.estats[k];
      }
      this.corrent = null;
      $('flowExecuta').textContent = 'Executa';
      this.pinta();
    }
  },

  aLaConversa(d) {
    const linies = this.flow.nodes
      .filter(n => this.execucio.estats[n.id])
      .map(n => '- ' + (n.title || n.id) + ' — ' + (this.execucio.estats[n.id] === 'err' ? 'error' : 'fet'));
    let txt = '**Graf «' + this.flow.name + '»**\n\n' + linies.join('\n');
    if (d.stopped) txt += '\n\nAturat: ' + d.stopped;
    if (d.answer) txt += '\n\n' + d.answer;
    try { G().add('a', '', G().md(txt)); } catch (e) {}
  },

  // sse llegeix un stream d'events. No fem servir EventSource perquè no
  // deixa posar capçaleres i aquí calen el token i la sessió.
  async sse(url, signal, onEvent) {
    const r = await G().api(url, { signal });
    if (!r.ok) throw new Error((await r.text()).trim() || ('HTTP ' + r.status));
    const rd = r.body.getReader(), dec = new TextDecoder();
    let buf = '', ev = '';
    for (;;) {
      const { done, value } = await rd.read();
      if (done) return;
      buf += dec.decode(value, { stream: true });
      let i;
      while ((i = buf.indexOf('\n\n')) >= 0) {
        const frame = buf.slice(0, i); buf = buf.slice(i + 2);
        let data = '';
        frame.split('\n').forEach(l => {
          if (l.startsWith('event:')) ev = l.slice(6).trim();
          else if (l.startsWith('data:')) data += l.slice(5).trim();
        });
        if (ev && data) { try { onEvent(ev, JSON.parse(data)); } catch (e) {} }
      }
    }
  },
};
