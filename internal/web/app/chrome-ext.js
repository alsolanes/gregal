// Preferències → Extensió de Chrome: com tenir els vents al costat de Google
// Docs, Sheets i Apps Script. A l'app d'escriptori, un botó la deixa a punt
// (carpeta + pàgina d'extensions); la connexió no cal configurar-la.
const esc = value => String(value == null ? '' : value)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
const T = key => typeof window.gregalT === 'function' ? window.gregalT(key) : key;

export const chromeExt = {
  result: null,

  bridge() {
    const b = window.gregalDesktop;
    return b && typeof b.setupChromeExtension === 'function' ? b : null;
  },

  section() {
    const desktop = !!this.bridge();
    const r = this.result;
    let body = '<p class="pf-skills-help">' + esc(T('prefs.chromeExt.help')) + '</p>';
    if (desktop) {
      body += '<div class="pf-chrome-row"><button class="pf-open" id="prefsChromeExt">' + esc(T('prefs.chromeExt.prepare')) + '</button>' +
        (r && !r.ok ? '<span class="pf-skills-status">' + esc(T('prefs.chromeExt.missing')) + '</span>' : '') + '</div>';
    }
    if (!desktop || (r && r.ok)) {
      const folder = r && r.path ? r.path : 'extension/chrome';
      body += '<ol class="pf-chrome-steps">' +
        '<li>' + esc(r && r.browser === 'edge' ? T('prefs.chromeExt.step1edge') : T('prefs.chromeExt.step1')) + '</li>' +
        '<li>' + esc(T('prefs.chromeExt.step2')) + ' <code id="prefsChromePath">' + esc(folder) + '</code>' +
        (r && r.path ? ' <button class="pf-open" id="prefsChromeCopy">' + esc(T('prefs.chromeExt.copy')) + '</button>' : '') + '</li>' +
        '<li>' + esc(desktop ? T('prefs.chromeExt.step3') : T('prefs.chromeExt.step3server')) + '</li>' +
        '</ol>';
    }
    // Deixar que l'agent faci servir el Chrome de l'usuari (les seves sessions).
    body += '<p class="pf-skills-help" id="prefsChromeControl">' + esc(T('prefs.chromeExt.control')) + '</p>';
    return '<h3 class="pf-sec">' + esc(T('prefs.chromeExt')) + '</h3><div id="prefsChromeExtBox">' + body + '</div>';
  },

  bind(parent, repaint) {
    const ctl = parent.querySelector('#prefsChromeControl');
    if (ctl && typeof window.api === 'function') {
      window.api('/api/browser/extension/status').then(r => r.json()).then(st => {
        if (st && st.connected) ctl.textContent = T('prefs.chromeExt.controlOn');
      }).catch(() => {});
    }
    const prep = parent.querySelector('#prefsChromeExt');
    if (prep) prep.onclick = async () => {
      prep.disabled = true;
      try { this.result = await this.bridge().setupChromeExtension(); } catch (e) { this.result = { ok: false }; }
      repaint();
    };
    const copy = parent.querySelector('#prefsChromeCopy');
    if (copy) copy.onclick = () => {
      navigator.clipboard.writeText(this.result.path).then(() => { copy.textContent = T('prefs.chromeExt.copied'); });
    };
  },
};
