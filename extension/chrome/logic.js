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
 * El missatge per al vent: el que demana l'usuari i, si vol, el context de la
 * pàgina (tallat), amb la instrucció de posar el que s'ha d'inserir en blocs.
 */
export function buildMessage(text, ctx, lang) {
  const ask = String(text || '').trim();
  if (!ctx || !ctx.content) return ask;
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
