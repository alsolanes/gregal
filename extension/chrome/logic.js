// Lògica pura del panell (sense chrome.*): es prova amb node --test.

/** Adreça, port → URL base, com a l'app del mòbil (baseUrlOf a Models.kt). */
export function baseUrl(addr, port) {
  let h = String(addr || '').trim().replace(/\/+$/, '') || '127.0.0.1';
  const p = String(port || '').trim();
  let scheme = /^https:\/\//i.test(h) ? 'https' : /^http:\/\//i.test(h) ? 'http' : isPublicDomain(h) ? 'https' : 'http';
  const bare = h.replace(/^[a-z]+:\/\//i, '');
  if (/:\d+$/.test(bare.replace(/^\[.*\]/, ''))) return `${scheme}://${bare}`;
  if (scheme === 'https' && !p) return `${scheme}://${bare}`;
  return `${scheme}://${bare}:${p || (scheme === 'https' ? '443' : '8097')}`;
}

export function isPublicDomain(host) {
  const h = String(host).replace(/^[a-z]+:\/\//i, '').split('/')[0].split(':')[0].toLowerCase();
  if (!h || !h.includes('.') || h === 'localhost' || /^[\d.]+$/.test(h)) return false;
  const tld = h.split('.').pop();
  return !['local', 'lan', 'home', 'internal', 'localdomain', 'arpa'].includes(tld) && /[a-z]/.test(tld);
}

/**
 * On és Gregal: amb l'app d'escriptori, a aquesta màquina, al port de sempre
 * (l'app el prefereix si és lliure) i sense clau; si no, el servidor configurat.
 */
export function connection(conf) {
  if ((conf.mode || 'desktop') === 'desktop') return { addr: '127.0.0.1', port: conf.desktopPort || '8097', key: '' };
  return { addr: conf.addr || '127.0.0.1', port: conf.port || '', key: conf.key || '' };
}

/** On es connecta l'extensió perquè Gregal pugui fer servir aquest navegador. */
export function extensionSocketUrl(conn) {
  return baseUrl(conn.addr, conn.port).replace(/^http/, 'ws') + '/api/browser/extension';
}

/** Talla un flux SSE en esdeveniments {event, data}; torna també el que encara no és complet. */
export function parseSSE(buffer) {
  const events = [];
  const parts = buffer.split(/\r?\n\r?\n/);
  const rest = parts.pop();
  for (const block of parts) {
    let event = 'message'; const data = [];
    for (const line of block.split(/\r?\n/)) {
      if (line.startsWith('event:')) event = line.slice(6).trim();
      else if (line.startsWith('data:')) data.push(line.slice(5).replace(/^ /, ''));
    }
    if (!data.length) continue;
    let parsed;
    try { parsed = JSON.parse(data.join('\n')); } catch { parsed = data.join('\n'); }
    events.push({ event, data: parsed });
  }
  return { events, rest };
}

/** Separa la resposta en text i blocs de codi (```lang ... ```). */
export function splitBlocks(text) {
  const out = [];
  const re = /```([\w+-]*)[^\n]*\n([\s\S]*?)(?:```|$)/g;
  let last = 0, m;
  while ((m = re.exec(text))) {
    if (m.index > last) out.push({ type: 'text', text: text.slice(last, m.index) });
    out.push({ type: 'code', lang: m[1], text: m[2].replace(/\n$/, '') });
    last = re.lastIndex;
  }
  if (last < text.length) out.push({ type: 'text', text: text.slice(last) });
  return out.filter((b) => b.type === 'code' || b.text.trim());
}

const KIND = { docs: 'Google Docs', sheets: 'Google Sheets', script: 'Apps Script', web: 'Web', pdf: 'PDF' };

/** La pestanya és un PDF (el visor de Chrome): per l'extensió de la ruta. */
export function isPdfUrl(url) {
  try {
    const u = new URL(String(url || ''));
    return /^https?:$/.test(u.protocol) && /\.pdf$/i.test(u.pathname);
  } catch { return false; }
}

/** Bytes → base64 a trossos (String.fromCharCode amb tot de cop peta). */
export function bytesToBase64(bytes) {
  let bin = '';
  for (let i = 0; i < bytes.length; i += 0x8000) bin += String.fromCharCode.apply(null, bytes.subarray(i, i + 0x8000));
  return btoa(bin);
}
export const MAX_CONTEXT = 14000;

/**
 * El missatge per al vent: el que demana l'usuari i el context de la
 * pàgina (tallat), amb la instrucció de posar el que s'ha d'inserir en blocs.
 */
export function buildMessage(text, ctx, lang) {
  const ask = String(text || '').trim();
  if (!ctx || !(ctx.content || ctx.url)) return ask;
  // Sense text llegible, el vent sap igualment quina pàgina és i la pot obrir.
  if (!ctx.content) return `${ask}\n\n[${KIND[ctx.kind] || 'Web'}: «${ctx.title || ''}» — ${ctx.url}]`;
  let content = ctx.content;
  let cut = '';
  if (content.length > MAX_CONTEXT) {
    content = content.slice(0, MAX_CONTEXT);
    cut = lang === 'ca' ? '\n[… tallat: el document és més llarg]' : '\n[… truncated: the document is longer]';
  }
  const head = `[${KIND[ctx.kind] || 'Web'}: «${ctx.title || ''}» — ${ctx.url || ''}${ctx.range ? ` — ${lang === 'ca' ? 'selecció' : 'selection'} ${ctx.range}` : ''}]` +
    // Un PDF pujat a Gregal: el vent el pot llegir sencer (també les pàgines
    // escanejades) amb pdf_read sobre aquesta ruta.
    (ctx.kind === 'pdf' && ctx.path ? `\n[${lang === 'ca' ? 'fitxer a Gregal' : 'file in Gregal'}: ${ctx.path} — pdf_read]` : '');
  const hint = ctx.kind === 'sheets'
    ? (lang === 'ca' ? 'Si proposes fórmules, valors o codi d\'Apps Script, posa\'ls cadascun en un bloc ``` (valors de taula separats per tabuladors) perquè es puguin inserir amb un clic.'
      : 'If you propose formulas, values or Apps Script code, put each in a ``` block (table values separated by tabs) so they can be inserted with one click.')
    : ctx.kind === 'script'
      ? (lang === 'ca' ? 'Si proposes codi, posa el fitxer sencer o la funció en un bloc ```javascript perquè es pugui inserir a l\'editor.'
        : 'If you propose code, put the whole file or function in a ```javascript block so it can be inserted into the editor.')
      : (lang === 'ca' ? 'Si proposes text per posar a la pàgina, posa\'l en un bloc ``` perquè es pugui inserir amb un clic.'
        : 'If you propose text to put on the page, put it in a ``` block so it can be inserted with one click.');
  return `${ask}\n\n${head}\n${content}${cut}\n[/${KIND[ctx.kind] || 'Web'}]\n(${hint})`;
}

const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

/** Negretes, cursives, `codi` i enllaços http(s) dins una línia ja escapada. */
function inline(s) {
  const codes = [];
  s = s.replace(/`([^`]+)`/g, (_, c) => `\u0000${codes.push(c) - 1}\u0000`);
  s = s.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
  s = s.replace(/\*\*(.+?)\*\*/g, '<b>$1</b>').replace(/(^|[^*\w])\*([^*\s][^*]*?)\*(?!\w)/g, '$1<i>$2</i>');
  return s.replace(/\u0000(\d+)\u0000/g, (_, i) => `<code>${codes[i]}</code>`);
}

/**
 * Markdown del vent → HTML segur (tot s'escapa; només surten etiquetes
 * nostres): paràgrafs, títols, llistes, taules i el de dins de línia.
 * Els blocs ``` els separa splitBlocks abans.
 */
export function renderMarkdown(text) {
  const lines = esc(String(text || '').trim()).split(/\r?\n/);
  const out = [];
  let para = [], list = null;
  const flush = () => {
    if (para.length) { out.push(`<p>${para.map(inline).join('<br>')}</p>`); para = []; }
    if (list) { out.push(`<${list.tag}>${list.items.map((i) => `<li>${inline(i)}</li>`).join('')}</${list.tag}>`); list = null; }
  };
  for (let i = 0; i < lines.length; i++) {
    const l = lines[i];
    let m;
    if (!l.trim()) { flush(); continue; }
    if ((m = l.match(/^\s*(#{1,6})\s+(.*)$/))) { flush(); out.push(`<h${m[1].length < 3 ? 3 : 4}>${inline(m[2])}</h${m[1].length < 3 ? 3 : 4}>`); continue; }
    if (/^\s*\|.*\|\s*$/.test(l) && /^\s*\|[\s:|-]+\|\s*$/.test(lines[i + 1] || '')) {
      flush();
      const cells = (r) => r.trim().replace(/^\||\|$/g, '').split('|').map((c) => inline(c.trim()));
      let html = `<table><tr>${cells(l).map((c) => `<th>${c}</th>`).join('')}</tr>`;
      for (i += 2; i < lines.length && /^\s*\|.*\|\s*$/.test(lines[i]); i++) html += `<tr>${cells(lines[i]).map((c) => `<td>${c}</td>`).join('')}</tr>`;
      i--;
      out.push(html + '</table>');
      continue;
    }
    if ((m = l.match(/^\s*(?:([-*•])|(\d+)[.)])\s+(.*)$/))) {
      const tag = m[1] ? 'ul' : 'ol';
      if (para.length || (list && list.tag !== tag)) flush();
      if (!list) list = { tag, items: [] };
      list.items.push(m[3]);
      continue;
    }
    if (list) flush();
    para.push(l);
  }
  flush();
  return out.join('');
}
