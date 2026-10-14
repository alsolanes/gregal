// Ajudes pures dels extres «divertits» (Wrapped, mascota, veu, imatges):
// sense DOM, perquè es puguin provar amb node. Al navegador queden a
// window.gregalFunCore.
(function (root) {
  'use strict';

  // Setmana ISO («2026-W42»): el Wrapped surt com a molt un cop per setmana.
  function weekKey(date) {
    const d = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()));
    const day = d.getUTCDay() || 7;
    d.setUTCDate(d.getUTCDate() + 4 - day);
    const year = d.getUTCFullYear();
    const week = Math.ceil(((d - Date.UTC(year, 0, 1)) / 86400000 + 1) / 7);
    return year + '-W' + String(week).padStart(2, '0');
  }

  // La targeta de la pàgina d'inici: activada, no descartada aquesta
  // setmana i amb alguna cosa a explicar (cap targeta buida).
  function wrappedDue(o) {
    const opts = o || {};
    if (opts.enabled === false || !opts.stats) return false;
    if (!(opts.stats.turns > 0)) return false;
    return opts.dismissed !== weekKey(opts.now || new Date());
  }

  // Estat de la mascota: qui espera una persona mana; després la feina; un
  // error o un «fet» recents es veuen uns segons i torna a la brisa.
  function mascotMood(s) {
    const o = s || {};
    if (o.waiting) return 'waiting';
    if (o.working) return 'working';
    if (o.error) return 'error';
    if (o.done) return 'done';
    return 'idle';
  }

  // Idioma del reconeixement de veu a partir del de la interfície.
  function speechLang(ui) {
    const l = String(ui || '').toLowerCase();
    if (l.startsWith('ca')) return 'ca-ES';
    if (l.startsWith('es')) return 'es-ES';
    return 'en-US';
  }

  // Text del camp mentre es dicta: el que hi havia + el confirmat + el
  // provisional (que es reescriu a cada resultat).
  function dictation(base, finalText, interim) {
    const b = String(base || '');
    const add = [String(finalText || '').trim(), String(interim || '').trim()].filter(Boolean).join(' ');
    if (!add) return b;
    return b + (b && !/\s$/.test(b) ? ' ' : '') + add;
  }

  // Només imatges del porta-retalls o d'un arrossegament (la resta de
  // límits —3 imatges, 8 MB— els aplica el flux d'adjunts existent).
  const IMAGE = /^image\/(png|jpe?g|webp|gif)$/i;
  function imageFiles(list) {
    return Array.from(list || []).filter(f => f && IMAGE.test(String(f.type || '')));
  }
  // Les frases del Wrapped (targeta i PNG). t(clau) tradueix; les xifres
  // que no es coneixen no surten (mai «0 $» inventat).
  function wrappedHighlights(stats, t) {
    const s = stats || {};
    const T = (k, vars) => { let out = t(k); for (const key in vars || {}) out = out.replace('{' + key + '}', vars[key]); return out; };
    const out = [];
    if (s.turns > 0) out.push(T('wrapped.hl.turns', { n: s.turns, s: s.sessions || 0 }));
    if (s.tool_calls > 0) out.push(T('wrapped.hl.tools', { n: s.tool_calls }));
    if (s.files_changed > 0) out.push(T('wrapped.hl.files', { n: s.files_changed }));
    if (s.tests_turned_green > 0) out.push(T('wrapped.hl.green', { n: s.tests_turned_green }));
    if (s.stubborn_bug && s.stubborn_bug.failures > 0) out.push(T('wrapped.hl.stubborn', { n: s.stubborn_bug.failures, cmd: s.stubborn_bug.command }));
    if (s.longest_streak_days > 1) out.push(T('wrapped.hl.streak', { n: s.longest_streak_days }));
    if (s.favorite_model) out.push(T('wrapped.hl.model', { m: s.favorite_model }));
    return out;
  }

  function fmtTokens(n) {
    const v = Number(n) || 0;
    if (v >= 1e6) return (Math.round(v / 1e5) / 10).toString().replace(/\.0$/, '') + 'M';
    if (v >= 1e3) return (Math.round(v / 100) / 10).toString().replace(/\.0$/, '') + 'k';
    return String(v);
  }

  const api = { weekKey, wrappedDue, mascotMood, speechLang, dictation, imageFiles, wrappedHighlights, fmtTokens };
  if (typeof module === 'object' && module.exports) module.exports = api;
  if (root) root.gregalFunCore = api;
})(typeof window !== 'undefined' ? window : null);
