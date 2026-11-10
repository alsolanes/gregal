// El camí de «escriure» a «guanyar» de la pàgina de l'Arena (arena.js), les
// peces pures (es proven sense DOM a arena-learn.test.cjs):
//  - applyFix / undoFix: posar al .md la regla reescrita d'un error del coach
//    (feedback.mistakes[].fix): substitueix la línia que diu el mateix o,
//    si no n'hi ha cap, l'afegeix a la secció que hi té més a veure (o al
//    final, sota «## Coach fixes»), i ho sap desfer.
//  - diffLines / diffHunks: què ha canviat entre dues versions del .md.
//  - matchesByVersion / versionStats: quins partits es van jugar amb cada
//    versió (pel temps: començat després de desar-la i abans de la següent).

const STOP = new Set('the and for with that this when then than you your our are was were has have had not but use from into onto they them their its it\'s also only just more less very can will should would could all any each every one two over under out off'.split(' '));
const MARK = /^(\s*)([-*+]|\d+[.)])\s+/;

// Les paraules que compten d'una línia (minúscules, sense marques ni signes).
export function words(s) {
  return String(s || '').toLowerCase().replace(MARK, '').replace(/[`*_#>«»"“”'’()[\]{}:;,.!?¿¡]/g, ' ').split(/[^a-z0-9%+<>=&/-]+/)
    .map(w => w.replace(/^[-/]+|[-/]+$/g, '')).filter(w => w.length > 2 && !STOP.has(w));
}
// Semblança entre dues línies (0..1): la mitjana del Jaccard de les paraules i
// de quanta part de la més curta hi ha a l'altra (una regla reescrita sol ser més llarga).
export function similarity(a, b) {
  const A = new Set(words(a)), B = new Set(words(b));
  if (!A.size || !B.size) return 0;
  let n = 0; for (const w of A) if (B.has(w)) n++;
  const j = n / (A.size + B.size - n);
  return n < 2 ? j : (j + n / Math.min(A.size, B.size)) / 2; // una sola paraula en comú no fa una regla semblant
}
const norm = s => String(s || '').toLowerCase().replace(MARK, '').replace(/\s+/g, ' ').replace(/[«»"“”`]/g, '').trim();
// Per comparar una cita amb una línia: sense signes de puntuació ni cometes.
const loose = s => norm(s).replace(/[.,;:!?…'’]+/g, '').replace(/\s+/g, ' ').trim();
// Les cites d'un text («...», "...", “...”, `...` i '...' entre espais, que
// l'apòstrof de «l'equip» o «don't» no compta) prou llargues per buscar-les.
export function quotes(s) {
  const out = [], re = /«([^»]{8,})»|"([^"]{8,})"|“([^”]{8,})”|`([^`]{8,})`|(?:^|[\s(:,])'(.{8,}?)'(?=$|[\s.,;:)!?—-])/g;
  let m; while ((m = re.exec(String(s || '')))) out.push((m[1] || m[2] || m[3] || m[4] || m[5]).trim());
  return out;
}
const isRule = l => l.trim() && !/^\s*(#|```|---|===)/.test(l);

// El que proposa el coach: la regla i si cal afegir-la o substituir-ne una.
// «Add: '…'», «Afegeix: '…'», «Replace with: '…'», «Reescriu: '…'» o la
// regla sola (entre cometes o no) → { rule, intent: 'add' | 'replace' | '' }.
const ADD = /^(add|adds|added|afegeix|afegir|afegiu|añade|añadir|insert|include|new rule)\b/i;
const REPLACE = /^(replace|rewrite|change|reword|substitu\w*|reescriu\w*|canvia\w*|reemplaza\w*|cambia\w*)\b/i;
export function parseFix(fix) {
  const t = String(fix || '').trim();
  // Tota entre cometes (sense cap altra cometa a dins, sense comptar apòstrofs).
  const PAIR = { "'": "'", '"': '"', '«': '»', '“': '”' };
  const wrapped = /^(['"«“])([\s\S]+)(['"»”])\.?$/.exec(t);
  if (wrapped && PAIR[wrapped[1]] === wrapped[3] && !wrapped[2].replace(/(\p{L})['’](\p{L})/gu, '$1$2').includes(wrapped[3])) return { rule: wrapped[2].trim(), intent: '' };
  // Una introducció curta acabada en «:» i la regla entre cometes fins al final.
  const pre = /^([^'"«“:\n]{2,90}):\s*(['"«“])([\s\S]+)$/.exec(t);
  if (pre) {
    const lead = pre[1].trim();
    const intent = ADD.test(lead) ? 'add' : REPLACE.test(lead) ? 'replace' : /\b(add|afeg|nova regla|new rule|override)/i.test(lead) ? 'add' : '';
    return { rule: pre[3].replace(/['"»”]\s*\.?\s*$/, '').trim(), intent };
  }
  return { rule: t, intent: '' };
}

// Les línies de la regla nova, amb la marca de llista de la línia veïna.
function fixLines(fix, like) {
  const raw = String(fix || '').replace(/\r/g, '').split('\n').map(l => l.replace(/\s+$/, '')).filter(l => l.trim());
  const m = like != null ? MARK.exec(like) : null;
  const indent = m ? m[1] : '', mark = m ? (/\d/.test(m[2]) ? '-' : m[2]) : '-';
  return raw.map(l => (MARK.test(l) ? indent + l.trim() : `${indent}${mark} ${l.trim()}`));
}

// applyFix(md, fix, {issue}) → { md, change } on change = { kind: 'replace' |
// 'insert' | 'none', start (línia, 0..), old: [línies tretes], lines: [línies posades] }.
// De la proposta es pren només la regla (parseFix: sense «Add:» ni cometes).
//   replace: una línia del .md és la mateixa regla (l'issue la cita, o
//            s'assembla prou a la nova) i el coach no diu «afegeix»: es canvia.
//   insert:  si no, a sota de la línia citada o de la que més s'hi assembla
//            (al final de la seva llista) o, si cap no s'hi assembla, al final
//            dins de «## Coach fixes».
//   none:    la regla ja hi és tal qual.
export function applyFix(md, fix, { issue = '', replaceAt = 0.42, nearAt = 0.15 } = {}) {
  const src = String(md || '').replace(/\r\n/g, '\n');
  const lines = src.split('\n');
  const { rule: fx, intent } = parseFix(fix);
  if (!fx) return { md: src, change: { kind: 'none', start: -1, old: [], lines: [] } };
  const fxNorm = fixLines(fx).map(loose);
  // Ja hi és?
  for (let i = 0; i < lines.length; i++) {
    if (loose(lines[i]) === fxNorm[0] && fxNorm.every((t, k) => loose(lines[i + k]) === t)) return { md: src, change: { kind: 'none', start: i, old: [], lines: [] } };
  }
  // La línia que l'issue cita (o la que més s'assembla a la regla nova).
  let best = -1, bestSim = 0;
  const qs = quotes(issue).map(loose).filter(q => q.length >= 8);
  for (let i = 0; i < lines.length && qs.length; i++) {
    const l = loose(lines[i]);
    if (isRule(lines[i]) && qs.some(q => l.includes(q) || (q.length > 20 && l.length > 12 && q.includes(l)))) { best = i; bestSim = 1; break; }
  }
  if (best < 0) {
    for (let i = 0; i < lines.length; i++) {
      if (!isRule(lines[i])) continue;
      const s = Math.max(similarity(lines[i], fx), issue ? similarity(lines[i], issue) * 0.8 : 0);
      if (s > bestSim + 1e-9) { best = i; bestSim = s; }
    }
  }
  let change;
  if (best >= 0 && intent !== 'add' && (bestSim >= replaceAt || (intent === 'replace' && bestSim >= nearAt))) {
    change = { kind: 'replace', start: best, old: [lines[best]], lines: fixLines(fx, lines[best]) };
  } else if (best >= 0 && bestSim >= nearAt) {
    // Al final del bloc de la línia (la llista on és, abans d'una línia en blanc o un títol).
    let at = best + 1;
    while (at < lines.length && isRule(lines[at]) && (MARK.test(lines[at]) || /^\s+\S/.test(lines[at])) && MARK.test(lines[best])) at++;
    change = { kind: 'insert', start: at, old: [], lines: fixLines(fx, lines[best]) };
  } else {
    // Al final, sota «## Coach fixes» (es crea el primer cop).
    const h = lines.findIndex(l => /^#{1,6}\s+coach fixes\s*$/i.test(l.trim()));
    let pre = [], at;
    if (h >= 0) {
      at = h + 1;
      while (at < lines.length && !/^#{1,6}\s/.test(lines[at])) at++;
      while (at > h + 1 && !lines[at - 1].trim()) at--;
    } else {
      at = lines.length;
      while (at > 0 && !lines[at - 1].trim()) at--;
      pre = at > 0 ? ['', '## Coach fixes'] : ['## Coach fixes'];
    }
    change = { kind: 'insert', start: at, old: [], lines: [...pre, ...fixLines(fx)] };
  }
  const out = lines.slice(0, change.start).concat(change.lines, lines.slice(change.start + change.old.length));
  return { md: out.join('\n'), change };
}

// Desfà un applyFix: torna a posar el que hi havia (si les línies noves encara
// hi són; si s'han mogut, les busca). null si ja no es pot.
export function undoFix(md, change) {
  if (!change || change.kind === 'none') return String(md || '');
  const lines = String(md || '').replace(/\r\n/g, '\n').split('\n'), nl = change.lines;
  const at = i => nl.every((l, k) => lines[i + k] === l);
  let i = at(change.start) ? change.start : -1;
  for (let j = 0; i < 0 && j <= lines.length - nl.length; j++) if (at(j)) i = j;
  if (i < 0) return null;
  return lines.slice(0, i).concat(change.old, lines.slice(i + nl.length)).join('\n');
}

// ---- diff ----

// diffLines(a, b) → [{op: ' '|'-'|'+', text}] (línia a línia, LCS).
export function diffLines(a, b) {
  const A = String(a || '').replace(/\r\n/g, '\n').split('\n'), B = String(b || '').replace(/\r\n/g, '\n').split('\n');
  let s = 0; while (s < A.length && s < B.length && A[s] === B[s]) s++;
  let e = 0; while (e < A.length - s && e < B.length - s && A[A.length - 1 - e] === B[B.length - 1 - e]) e++;
  const a2 = A.slice(s, A.length - e), b2 = B.slice(s, B.length - e);
  const out = A.slice(0, s).map(text => ({ op: ' ', text }));
  const n = a2.length, m = b2.length;
  if (n * m > 4e6) {
    a2.forEach(text => out.push({ op: '-', text })); b2.forEach(text => out.push({ op: '+', text }));
  } else {
    // L[i][j] = LCS de a2[i..] i b2[j..].
    const L = Array.from({ length: n + 1 }, () => new Uint16Array(m + 1));
    for (let i = n - 1; i >= 0; i--) for (let j = m - 1; j >= 0; j--) L[i][j] = a2[i] === b2[j] ? L[i + 1][j + 1] + 1 : Math.max(L[i + 1][j], L[i][j + 1]);
    let i = 0, j = 0;
    while (i < n || j < m) {
      if (i < n && j < m && a2[i] === b2[j]) { out.push({ op: ' ', text: a2[i] }); i++; j++; }
      else if (i < n && (j >= m || L[i + 1][j] >= L[i][j + 1])) out.push({ op: '-', text: a2[i++] });
      else out.push({ op: '+', text: b2[j++] });
    }
  }
  A.slice(A.length - e).forEach(text => out.push({ op: ' ', text }));
  return out;
}

// Els trossos que han canviat amb `context` línies de context; la resta, plegada:
// [{op, text}] amb {op: '…', skip: N} on es plega.
export function diffHunks(ops, context = 2) {
  const keep = new Uint8Array(ops.length);
  ops.forEach((o, i) => { if (o.op !== ' ') for (let k = Math.max(0, i - context); k <= Math.min(ops.length - 1, i + context); k++) keep[k] = 1; });
  const out = [];
  for (let i = 0; i < ops.length;) {
    if (keep[i]) { out.push(ops[i]); i++; continue; }
    let j = i; while (j < ops.length && !keep[j]) j++;
    out.push({ op: '…', skip: j - i });
    i = j;
  }
  return out;
}
// Quantes línies s'han afegit i tret.
export function diffCount(ops) { let add = 0, del = 0; for (const o of ops) { if (o.op === '+') add++; else if (o.op === '-') del++; } return { add, del }; }

// ---- versions i partits ----

// matchesByVersion(history, matches) → una llista per versió (en el mateix
// ordre que history, la més antiga primer) amb els partits jugats amb ella:
// començats a partir del moment que es va desar (at) i abans de la següent.
// Els d'abans de la primera versió que es conserva no hi surten.
export function matchesByVersion(history, matches) {
  const h = history || [], out = h.map(() => []);
  for (const g of matches || []) {
    const t = +g.started;
    if (!Number.isFinite(t)) continue;
    let k = -1;
    for (let i = 0; i < h.length; i++) if (+h[i].at <= t) k = i; else break;
    if (k >= 0) out[k].push(g);
  }
  return out;
}

// El balanç d'uns partits: { n, w, l, d, coach (mitjana o null) }.
export function versionStats(list) {
  const r = { n: 0, w: 0, l: 0, d: 0, coach: null };
  let cs = 0, cn = 0;
  for (const g of list || []) {
    r.n++;
    if (g.result === 'win') r.w++; else if (g.result === 'loss') r.l++; else r.d++;
    if (g.coach) { cs += +g.coach; cn++; }
  }
  if (cn) r.coach = Math.round(cs / cn * 10) / 10;
  return r;
}

// El partit de pràctica acabat més nou d'un joc (de la llista progress).
export function lastPractice(progress) {
  const rows = (progress || []).filter(r => r.kind === 'practice');
  return rows.length ? rows.reduce((a, b) => (+b.started >= +a.started ? b : a)) : null;
}

// Els errors més útils d'un informe: els que porten regla reescrita primer, com a molt n.
export function topMistakes(feedback, n = 3) {
  const ms = ((feedback && feedback.mistakes) || []).filter(m => m && (m.fix || m.what));
  return ms.filter(m => m.fix).concat(ms.filter(m => !m.fix)).slice(0, n);
}

// La nota del coach (0-25) d'un informe.
export const coachScore = f => (f && f.scores ? Object.values(f.scores).reduce((a, b) => a + (+b || 0), 0) : null);
