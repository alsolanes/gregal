// Regles d'estil de les respostes dels vents, verificables sense model.
//
// Les fa servir scripts/style-eval.cjs (respostes reals d'un servidor) i les
// prova scripts/style-rules.test.cjs (exemples bons i dolents). Cada regla
// torna una llista de problemes: buida = correcte. Les regles venen dels
// errors que hem vist en converses reals i de com els altres assistents
// (ChatGPT, Claude, SillyTavern, AutoGen…) els eviten: no parlar per un altre
// personatge, no repetir l'etiqueta de qui parla, no fer murs de llistes en
// una xerrada, no filtrar marcadors interns.
'use strict';

// Text "de prosa": sense blocs de codi, imatges ni enllaços (que compten a
// part), perquè les frases es puguin comptar.
function prose(text) {
  return String(text || '')
    .replace(/```[\s\S]*?(```|$)/g, ' ')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/`[^`]*`/g, 'x');
}

// Frases: final amb . ! ? … (també línies de llista, que compten com a frase).
function sentences(text) {
  const p = prose(text).replace(/\b(p\. ex|etc|Sr|Sra|aprox|núm|vs)\./gi, '$1');
  return p.split(/(?<=[.!?…])\s+|\n+/).map(s => s.trim()).filter(s => /[\p{L}\p{N}]/u.test(s));
}

function escapeRe(s) { return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'); }

// «Nom:» o «**Nom:**» o «Nom —» al principi d'una línia.
function speakerLabel(name) {
  return new RegExp('^\\s*(\\*\\*|__)?' + escapeRe(name) + '(\\*\\*|__)?\\s*[:：—–-](\\*\\*|__)?\\s', 'imu');
}

const INTERNAL_MARKERS = [
  [/^\s*NEXT:\s*\d{4}-/m, 'línia NEXT: interna'],
  [/^\s*\(res\)\s*$/m, '«(res)» del cor que calla'],
  [/^\s*NOT_YET\s*$/m, 'NOT_YET d\'un recordatori'],
  [/<\/?(tool_call|tool_use|function_calls?|invoke|parameter)\b/i, 'marques d\'eina crues'],
  [/DSML|<｜/, 'marques DSML del model'],
  [/\b(web_search|web_fetch|web_images)\s*\(/, 'crida d\'eina escrita com a text'],
  [/^\s*(ASSISTANT|USER|SYSTEM)\b(\s+\p{L}[\p{L} ]{0,30})?\s*[:<]/mu, 'etiqueta de rol del transcript'],
  [/PREVIOUS CONVERSATION|WHO ANSWERS|VOICE CARD|HOW YOU TALK/, 'fragment de la consigna'],
];

// Paraules que delaten l'idioma (les més freqüents i que no comparteixen).
const LANG_WORDS = {
  ca: ['és', 'amb', 'per', 'els', 'les', 'què', 'però', 'també', 'això', 'molt', 'fer', 'avui', 'tens', 'pots', 'vull', 'perquè', 'doncs', 'ara', 'quan', 'jo', 'seva', 'meu', 'teu', 'i', 'són', 'fins', 'sense', 'dues'],
  es: ['con', 'para', 'los', 'qué', 'pero', 'también', 'esto', 'muy', 'hacer', 'hoy', 'tienes', 'puedes', 'porque', 'pues', 'ahora', 'y', 'cuando', 'yo', 'su', 'mi', 'tu', 'son', 'hasta', 'sin', 'dos'],
  en: ['the', 'and', 'with', 'for', 'you', 'is', 'are', 'this', 'that', 'what', 'but', 'also', 'very', 'today', 'can', 'here', 'because', 'now', 'it', 'of'],
};

function langText(text) {
  return prose(text)
    .replace(/^\s*\|.*\|\s*$/gm, ' ')
    .replace(/\*\*[^*]+\*\*|\*[^*\n]+\*|_[^_\n]+_|«[^»]*»|"[^"\n]*"/g, ' ');
}

function guessLang(text, minWords = 6) {
  const words = langText(text).toLowerCase().match(/[\p{L}']+/gu) || [];
  if (words.length < minWords) return '';
  const score = {};
  for (const [lang, list] of Object.entries(LANG_WORDS)) {
    const set = new Set(list);
    score[lang] = words.filter(w => set.has(w)).length;
  }
  // «l'» i «-ho/-la/-me» són molt catalans; «ñ» i «¿» molt castellans.
  score.ca += (langText(text).match(/\b[ldsmtn]['’]\p{L}|-(ho|hi|la|les|me|te|se|nos|vos)\b|·l/giu) || []).length;
  score.es += (langText(text).match(/ñ|¿|¡|ción\b/giu) || []).length;
  const best = Object.entries(score).sort((a, b) => b[1] - a[1]);
  return best[0][1] >= 2 && best[0][1] > best[1][1] ? best[0][0] : '';
}

function contentWords(text) {
  const stop = new Set([...LANG_WORDS.ca, ...LANG_WORDS.es, ...LANG_WORDS.en, 'de', 'la', 'el', 'a', 'i', 'o', 'que', 'en', 'un', 'no', 'si', 'del', 'al', 'es', 'se', 'hi', 'ho', 'em', 'et', 'ja', 'més', 'mes', 'to', 'in', 'on', 'be']);
  return new Set((prose(text).toLowerCase().match(/[\p{L}\p{N}]{4,}/gu) || []).filter(w => !stop.has(w)));
}

function jaccard(a, b) {
  const A = contentWords(a), B = contentWords(b);
  if (!A.size || !B.size) return 0;
  let inter = 0;
  for (const w of A) if (B.has(w)) inter++;
  return inter / (A.size + B.size - inter);
}

function emojiCount(text) {
  return (String(text || '').match(/\p{Extended_Pictographic}/gu) || []).length;
}

/**
 * Comprova una resposta d'un sol vent.
 * opts: { speaker, others: [noms d'altres vents], lang: 'ca'|'en'|'es',
 *         kind: 'casual'|'fact'|'explain'|'group-voice'|'mention',
 *         userName }
 */
function checkReply(text, opts = {}) {
  const t = String(text || '');
  const problems = [];
  const kind = opts.kind || 'casual';
  if (!t.trim()) return ['resposta buida'];
  // 1. Etiqueta pròpia: la UI ja diu qui parla.
  if (opts.speaker && speakerLabel(opts.speaker).test(t)) problems.push('comença amb el seu propi nom («' + opts.speaker + ':»)');
  // 2. Parla per un altre vent.
  for (const other of opts.others || []) {
    if (other && speakerLabel(other).test(t)) problems.push('escriu com si fos ' + other);
  }
  // 3. Marcadors interns filtrats.
  for (const [re, what] of INTERNAL_MARKERS) if (re.test(t)) problems.push('filtra ' + what);
  // 4. Llargada segons el tipus.
  const limits = { casual: 4, mention: 5, 'group-voice': 4, fact: 5, explain: 14 };
  const n = sentences(t).length;
  if (limits[kind] && n > limits[kind]) problems.push(n + ' frases (màxim ' + limits[kind] + ' en ' + kind + ')');
  // 5. Format de document en una xerrada.
  if (kind !== 'explain') {
    if (/^\s{0,3}#{1,6}\s/m.test(t)) problems.push('títols markdown en una xerrada');
    const bullets = (t.match(/^\s*([-*•]|\d+[.)])\s+/gm) || []).length;
    if (bullets > (kind === 'casual' || kind === 'group-voice' ? 0 : 5)) problems.push(bullets + ' punts de llista en una xerrada');
  }
  // 6. Enllaços: com a molt una font per veu en grup, tres en una resposta.
  const links = (prose(t) !== t ? (t.match(/\]\(https?:\/\//g) || []).length : 0) + (t.match(/(^|\s)https?:\/\/\S+/g) || []).length;
  const maxLinks = kind === 'group-voice' || kind === 'casual' ? 1 : 3;
  if (links > maxLinks) problems.push(links + ' enllaços (màxim ' + maxLinks + ')');
  if (kind === 'fact' && links === 0 && opts.needsSource) problems.push('dada actual sense cap font');
  // 7. Emojis: un de signatura, com a molt dos.
  if (emojiCount(t) > 2) problems.push(emojiCount(t) + ' emojis');
  // 8. Idioma de l'usuari.
  if (opts.lang) {
    const got = guessLang(t.replace(/```choices[\s\S]*?(```|$)/g, ''));
    if (got && got !== opts.lang) problems.push('idioma ' + got + ' (esperat ' + opts.lang + ')');
  }
  // 9. Bloc de respostes ràpides mal format.
  if (/```choices[ \t]+\S/.test(t)) problems.push('bloc choices en una sola línia (els botons es perden)');
  const choice = t.match(/```choices\n([\s\S]*?)(```|$)/);
  if (choice) {
    const opts2 = choice[1].split('\n').map(s => s.trim()).filter(Boolean);
    if (opts2.length < 2 || opts2.length > 4) problems.push('bloc choices amb ' + opts2.length + ' opcions');
    const cl = guessLang(opts2.join('. '), 4);
    if (opts.lang && cl && cl !== opts.lang) problems.push('respostes ràpides en ' + cl + ' (esperat ' + opts.lang + ')');
  }
  // 10. Com a molt una pregunta per resposta (Claude: intenta-ho abans de preguntar).
  const questions = sentences(t).filter(x => /\?\s*$/.test(x)).length;
  if (questions > 1) problems.push(questions + ' preguntes en una resposta');
  // 11. Accions de rol entre asteriscs (*somriu*) que ningú ha demanat; la negreta (**) no compta.
  // Només a l'inici d'una línia o d'una frase («*somriu* Hola»): una paraula en
  // cursiva enmig de la frase («crema més greix *durant* la sessió») és èmfasi.
  if (/(^|[.!?…]\s+)\*[a-zà-úç][^*\n]{2,40}\*(?!\*)/m.test(t.replace(/\*\*[^*]+\*\*/g, ''))) problems.push('accions de rol entre asteriscs');
  // 12. Frases buides d'assistent.
  if (/\b(com a (model|IA)|as an AI( language model)?|espero que (això )?t'ajudi|I hope this helps|no dubtis a preguntar|feel free to ask)\b/i.test(t)) problems.push('frase de farciment d\'assistent');
  return problems;
}

/**
 * Comprova un torn de grup (rotllana): les veus per separat i entre elles.
 * voices: [{ speaker, text }], members: [noms]
 */
function checkGroupTurn(voices, opts = {}) {
  const problems = [];
  const members = opts.members || [];
  if (!voices.length) return ['cap veu ha respost'];
  voices.forEach((v, i) => {
    if (members.length && !members.includes(v.speaker)) problems.push('veu ' + (i + 1) + ': «' + v.speaker + '» no és del grup');
    const others = members.filter(m => m !== v.speaker);
    for (const p of checkReply(v.text, { ...opts, speaker: v.speaker, others, kind: opts.mention ? 'mention' : 'group-voice' })) {
      problems.push(v.speaker + ': ' + p);
    }
  });
  if (opts.mention) {
    const wrong = voices.filter(v => v.speaker !== opts.mention);
    if (wrong.length) problems.push('menció a ' + opts.mention + ' però parla ' + wrong.map(v => v.speaker).join(', '));
  }
  // Cada vent parla un sol cop per torn: dues veus del mateix vent són un
  // torn duplicat (dues crides que han escrit totes dues les veus de tots).
  const seen = new Map();
  for (const v of voices) seen.set(v.speaker, (seen.get(v.speaker) || 0) + 1);
  for (const [who, n] of seen) if (n > 1) problems.push(who + ' parla ' + n + ' vegades en el mateix torn');
  // Consell repetit entre veus.
  for (let i = 0; i < voices.length; i++) {
    for (let j = i + 1; j < voices.length; j++) {
      const sim = jaccard(voices[i].text, voices[j].text);
      if (sim > 0.45) problems.push(voices[i].speaker + ' i ' + voices[j].speaker + ' diuen gairebé el mateix (' + Math.round(sim * 100) + '%)');
    }
  }
  return problems;
}

/** Informe final d'una colla: resum primer, honest, sense volcats interns. */
function checkCrewReport(text, opts = {}) {
  const t = String(text || '');
  const problems = [];
  if (!t.trim()) return ['informe buit'];
  for (const [re, what] of INTERNAL_MARKERS) if (re.test(t)) problems.push('filtra ' + what);
  if (/^\s*[{[]/.test(t) && /"\w+"\s*:/.test(t.slice(0, 200))) problems.push('JSON cru com a informe');
  const first = sentences(t)[0] || '';
  if (first.length > 280) problems.push('la primera frase no és un resum (' + first.length + ' caràcters)');
  if (opts.lang) {
    const got = guessLang(t);
    if (got && got !== opts.lang) problems.push('idioma ' + got + ' (esperat ' + opts.lang + ')');
  }
  // Diu «fet»/«provat» sense res que ho demostri.
  if (/\b(tests? (passen|passed|ok)|tot funciona|all tests pass)\b/i.test(t) && !/(`[^`]+`|\b\d+ (tests?|proves))/i.test(t)) {
    problems.push('afirma que passa les proves sense dir quines');
  }
  return problems;
}

module.exports = { checkReply, checkGroupTurn, checkCrewReport, sentences, guessLang, jaccard, emojiCount, speakerLabel };
