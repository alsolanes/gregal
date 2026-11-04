// Nom llegible d'un model: «deepseek-v4.1-flash» → «DeepSeek V4.1 Flash».
// L'identificador complet es continua veient al títol i als detalls; això és
// només per al que es pinta. La mateixa lògica que l'app d'Android
// (ModelName.kt). UMD: el navegador la té a window.gregalModelName i els tests
// de Node la carreguen amb require().
(function (root, factory) {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.gregalModelName = api;
})(typeof self !== 'undefined' ? self : this, function () {
  'use strict';
  const BRANDS = {
    deepseek: 'DeepSeek', gpt: 'GPT', claude: 'Claude', qwen: 'Qwen', llama: 'Llama', gemini: 'Gemini',
    mistral: 'Mistral', glm: 'GLM', kimi: 'Kimi', grok: 'Grok', gemma: 'Gemma', phi: 'Phi',
    minimax: 'MiniMax', codestral: 'Codestral', devstral: 'Devstral', command: 'Command', sonar: 'Sonar',
  };
  const ATTACH = new Set(['qwen']); // «Qwen3», no «Qwen 3»
  const IGNORE = new Set(['latest']);

  function word(t) {
    if (/^\d+(\.\d+)*$/.test(t)) return t;                      // 4.1
    if (/^v\d/.test(t)) return 'V' + t.slice(1);                // v4.1 → V4.1
    if (/^\d+(x\d+)?[bkm]$/.test(t)) return t.slice(0, -1) + t.slice(-1).toUpperCase(); // 70b → 70B
    if (/^\d/.test(t)) return t;                                // 4o
    if (/^o\d/.test(t)) return t;                               // o3
    if (/^[a-z]\d+[bkm]$/.test(t)) return t.toUpperCase();      // a3b → A3B
    return t.charAt(0).toUpperCase() + t.slice(1);
  }

  function name(id) {
    let s = String(id == null ? '' : id).trim();
    if (!s) return '';
    s = s.slice(s.lastIndexOf('/') + 1).toLowerCase();
    let tag = '';
    const colon = s.indexOf(':');
    if (colon >= 0) { tag = s.slice(colon + 1); s = s.slice(0, colon); }
    s = s.replace(/-(20\d{6}|\d{4})$/, '');                      // data al final
    // «llama3.2» → llama 3.2; «qwen3» es manté enganxat.
    let m = /^([a-z]+)(\d[\d.]*)(.*)$/.exec(s);
    const toks = s.split(/[-_]/).filter(Boolean);
    if (m && BRANDS[m[1]] && toks[0] === s.split(/[-_]/)[0] && /^[a-z]+\d/.test(toks[0])) {
      if (!ATTACH.has(m[1])) toks.splice(0, 1, m[1], m[2] + (m[3] && !m[3].startsWith('-') ? m[3] : ''));
    }
    // Números separats per «-» són una versió: 5-5 → 5.5
    const merged = [];
    for (const t of toks) {
      const prev = merged[merged.length - 1];
      if (/^\d+$/.test(t) && prev && /^\d+(\.\d+)*$/.test(prev) && t.length <= 2) merged[merged.length - 1] = prev + '.' + t;
      else merged.push(t);
    }
    if (tag && !IGNORE.has(tag)) merged.push(tag);
    const words = merged.filter(t => !IGNORE.has(t)).map(t => BRANDS[t] || word(t));
    // «GPT» s'uneix a la seva versió: GPT-5.1, GPT-4o.
    if (words.length > 1 && words[0] === 'GPT' && /^\d/.test(words[1])) words.splice(0, 2, 'GPT-' + words[1]);
    return words.join(' ') || String(id);
  }

  return name;
});
