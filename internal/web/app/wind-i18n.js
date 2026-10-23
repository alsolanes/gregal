// Locale metadata belongs to interface labels, never to user content or drafts.
(function (root) {
  'use strict';
  function text(key, fallback, vars = {}) {
    const translated = typeof root.gregalT === 'function' ? root.gregalT(key) : fallback;
    const template = translated && translated !== key ? translated : fallback || key;
    const value = new String(String(template).replace(/\{(\w+)\}/g, (match, name) =>
      Object.hasOwn(vars, name) ? String(vars[name]) : match));
    value.i18nKey = key;
    value.i18nVars = vars;
    value.i18nFallback = fallback;
    return value;
  }
  function paint(el, value) {
    if (!el) return;
    el.textContent = String(value == null ? '' : value);
    if (value?.i18nKey) {
      el.setAttribute('data-wind-i18n', value.i18nKey);
      el.setAttribute('data-wind-vars', JSON.stringify(value.i18nVars || {}));
      el.setAttribute('data-wind-fallback', value.i18nFallback || '');
    } else {
      el.removeAttribute('data-wind-i18n');
      el.removeAttribute('data-wind-vars');
      el.removeAttribute('data-wind-fallback');
    }
  }
  function read(el, attr) {
    try { return JSON.parse(el.getAttribute(attr) || '{}'); } catch { return {}; }
  }
  function attribute(el, name, value) {
    el.setAttribute(name, String(value));
    const attrs = read(el, 'data-wind-i18n-attrs');
    if (value?.i18nKey) attrs[name] = { key:value.i18nKey, fallback:value.i18nFallback, vars:value.i18nVars };
    else delete attrs[name];
    if (Object.keys(attrs).length) el.setAttribute('data-wind-i18n-attrs', JSON.stringify(attrs));
    else el.removeAttribute('data-wind-i18n-attrs');
  }
  function refresh() {
    root.document.querySelectorAll('[data-wind-i18n]').forEach(el => {
      paint(el, text(el.getAttribute('data-wind-i18n'), el.getAttribute('data-wind-fallback'), read(el, 'data-wind-vars')));
    });
    root.document.querySelectorAll('[data-wind-i18n-attrs]').forEach(el => {
      for (const [name, item] of Object.entries(read(el, 'data-wind-i18n-attrs'))) {
        attribute(el, name, text(item.key, item.fallback, item.vars));
      }
    });
  }
  function error(key, fallback, vars) {
    const value = text(key, fallback, vars);
    const result = new Error(String(value));
    result.localized = value;
    return result;
  }
  root.gregalWindI18n = { text, paint, attribute, refresh, error };
  root.document.addEventListener('gregal:idioma', refresh);
})(window);
