// Preferències → Complement de Word: el panell del Gregal dins del Word.
// A l'app d'escriptori de Windows s'instal·la d'un clic (l'app el registra
// amb el port real del backend); fora, l'explicació de docs/addin.md.
const esc = value => String(value == null ? '' : value)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
const T = key => typeof window.gregalT === 'function' ? window.gregalT(key) : key;

export const wordAddin = {
  state: null,
  busy: false,
  asked: false,

  bridge() {
    const b = window.gregalDesktop;
    return b && typeof b.wordAddin === 'function' ? b : null;
  },

  section() {
    const b = this.bridge();
    const s = this.state;
    let body = '<p class="pf-skills-help">' + esc(T('prefs.wordAddin.help')) + '</p>';
    if (!b) {
      body += '<p class="pf-skills-help">' + esc(T('prefs.wordAddin.manual')) + '</p>';
    } else if (s && s.reason === 'platform') {
      body += '<p class="pf-skills-help">' + esc(T('prefs.wordAddin.windowsOnly')) + '</p>';
    } else {
      const installed = !!(s && s.installed);
      body += '<div class="pf-chrome-row">' +
        '<button class="pf-open" id="prefsWordInstall"' + (this.busy ? ' disabled' : '') + '>' +
        esc(T(installed ? 'prefs.wordAddin.reinstall' : 'prefs.wordAddin.install')) + '</button>' +
        (installed ? ' <button class="pf-open" id="prefsWordRemove"' + (this.busy ? ' disabled' : '') + '>' + esc(T('prefs.wordAddin.remove')) + '</button>' : '') +
        (s && s.ok === false ? '<span class="pf-skills-status">' + esc(T('prefs.wordAddin.failed')) + '</span>' : '') +
        (installed ? '<span class="pf-skills-status">' + esc(T('prefs.wordAddin.installed')) + '</span>' : '') +
        '</div>';
      if (installed) {
        body += '<ol class="pf-chrome-steps">' +
          '<li>' + esc(T('prefs.wordAddin.step1')) + '</li>' +
          '<li>' + esc(T('prefs.wordAddin.step2')) + '</li>' +
          '</ol>';
      }
    }
    return '<h3 class="pf-sec">' + esc(T('prefs.wordAddin')) + '</h3><div id="prefsWordAddinBox">' + body + '</div>';
  },

  async run(action, repaint) {
    this.busy = true;
    repaint();
    try { this.state = await this.bridge().wordAddin(action); } catch (e) { this.state = { ok: false }; }
    this.busy = false;
    repaint();
  },

  bind(parent, repaint) {
    if (!this.bridge()) return;
    // L'estat (instal·lat o no) es demana un cop en obrir les preferències.
    if (!this.asked) { this.asked = true; this.run('status', repaint); return; }
    const install = parent.querySelector('#prefsWordInstall');
    if (install) install.onclick = () => this.run('install', repaint);
    const remove = parent.querySelector('#prefsWordRemove');
    if (remove) remove.onclick = () => this.run('remove', repaint);
  },
};
