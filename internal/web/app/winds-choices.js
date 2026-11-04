// Respostes ràpides dels vents: un bloc ```choices al final del missatge del
// vent són botons (2-4 opcions curtes, una per línia), no text. La mateixa
// lògica que l'app d'Android (WindAsks.kt). UMD: el navegador la té a
// window.gregalChoices i els tests de Node la carreguen amb require().
(function (root, factory) {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.gregalChoices = api;
})(typeof self !== 'undefined' ? self : this, function () {
  'use strict';
  const FENCE = '```choices';

  // Les opcions: fins a 4, netes de «-», «*» i «1.», sense repetits ni de més de 60 caràcters.
  function extract(text) {
    const s = String(text == null ? '' : text);
    const start = s.lastIndexOf(FENCE);
    if (start < 0) return [];
    const body = s.slice(start + FENCE.length).split('```')[0];
    const out = [];
    for (let line of body.split('\n')) {
      line = line.trim().replace(/^[-*]\s*/, '').replace(/^\d+[.)]\s+/, '').trim();
      if (line && line.length <= 60 && !out.includes(line)) out.push(line);
      if (out.length === 4) break;
    }
    return out;
  }

  // El missatge sense el bloc; també si encara arriba en directe i no s'ha tancat.
  function strip(text) {
    const s = String(text == null ? '' : text);
    const start = s.lastIndexOf(FENCE);
    if (start < 0) return s;
    const after = s.slice(start + FENCE.length);
    const close = after.indexOf('```');
    const tail = close >= 0 ? after.slice(close + 3).trim() : '';
    return (s.slice(0, start).trimEnd() + (tail ? '\n' + tail : '')).trimEnd();
  }

  return { extract, strip };
});
