// Preferències → Veus dels narradors: quines llengües hi ha instal·lades,
// baixar-ne una, provar-les i triar el motor. Abans només es podia des d'un
// plec de l'Arena, i no es veia per què la tele no parlava.
// Fa servir /api/arena/voice (el plugin Arena): sense el plugin, ho diu.

const G = () => window.gregal || {};
const T = (k, alt) => (window.gregalT ? window.gregalT(k) : '') || alt || k;
const esc = v => String(v == null ? '' : v).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
const api = (p, o) => (G().api ? G().api(p, o) : fetch(p, o));
const LANGS = [['en', 'English'], ['ca', 'Català']];

export const voicePrefs = {
  st: null, err: '', timer: 0,

  section() { return '<div id="prefsVoice"><div class="pf-skills-status">' + esc(T('prefs.voice.loading')) + '</div></div>'; },

  async load() {
    try {
      const r = await api('/api/arena/voice');
      if (r.status === 404) { this.st = null; this.err = T('prefs.voice.noArena'); return; }
      if (!r.ok) throw new Error((await r.text()).trim() || r.status);
      this.st = await r.json(); this.err = '';
    } catch (e) { this.st = null; this.err = String(e.message || e); }
  },

  html() {
    if (!this.st) return '<div class="pf-skills-status' + (this.err ? ' error' : '') + '">' + esc(this.err || T('prefs.voice.loading')) + '</div>';
    const v = this.st, inst = v.install || {}, eng = (v.config && v.config.engine) || 'piper';
    const busy = inst.state === 'installing';
    const pct = inst.total > 0 ? Math.round(inst.done * 100 / inst.total) + '%' : Math.round((inst.done || 0) / 1e6) + ' MB';
    const engine = '<div class="pf-row"><div class="pf-lab"><b>' + esc(T('prefs.voice.engine')) + '</b><span>' + esc(T('prefs.voice.engine.help')) + '</span></div>' +
      '<div class="pf-seg" data-voice-engine>' + [['piper', 'prefs.voice.piper'], ['http', 'prefs.voice.http'], ['off', 'prefs.voice.off']]
        .map(([k, t]) => '<button data-v="' + k + '" class="' + (k === eng ? 'on' : '') + '">' + esc(T(t)) + '</button>').join('') + '</div></div>';
    const rows = LANGS.map(([l, name]) => {
      const ready = !!(v.ready || {})[l];
      const state = ready ? '✓ ' + T('prefs.voice.ready') : eng === 'piper' ? (v.platform ? T('prefs.voice.missing') : T('prefs.voice.noPlatform')) : eng === 'off' ? T('prefs.voice.offState') : T('prefs.voice.server');
      const btn = eng === 'piper' && v.platform
        ? '<button class="pf-open" data-voice-install="' + l + '"' + (busy ? ' disabled' : '') + '>' + esc(ready ? T('prefs.voice.reinstall') : T('prefs.voice.install').replace('{size}', (v.sizes || {})[l] || '')) + '</button>' : '';
      const test = ready ? '<button class="pf-open" data-voice-test="caster" data-lang="' + l + '">▶ ' + esc(T('prefs.voice.caster')) + '</button>' +
        '<button class="pf-open" data-voice-test="analyst" data-lang="' + l + '">▶ ' + esc(T('prefs.voice.analyst')) + '</button>' : '';
      return '<div class="pf-row pf-voice' + (ready ? ' on' : '') + '"><div class="pf-lab"><b>' + esc(name) + (l === 'en' ? ' · ' + esc(T('prefs.voice.games')) : '') +
        '</b><span>' + esc(state) + '</span></div><div class="pf-voice-btns">' + test + btn + '</div></div>';
    }).join('');
    const prog = busy ? '<div class="pf-skills-status">⬇ ' + esc(T('prefs.voice.installing').replace('{step}', inst.step || '…').replace('{pct}', pct)) + '</div>'
      : inst.state === 'error' ? '<div class="pf-skills-status error">⚠ ' + esc(inst.error || '') + '</div>' : '';
    return '<p class="pf-skills-help">' + esc(T('prefs.voice.help')) + '</p>' + engine + rows + prog +
      '<div class="pf-skills-status" id="prefsVoiceMsg" hidden></div>';
  },

  async bind(cos) {
    const box = cos && cos.querySelector('#prefsVoice');
    if (!box) return;
    await this.load();
    if (!box.isConnected) return;
    const paint = () => { box.innerHTML = this.html(); wire(); };
    const msg = t => { const m = box.querySelector('#prefsVoiceMsg'); if (m) { m.hidden = !t; m.textContent = t || ''; } };
    const poll = () => {
      clearTimeout(this.timer);
      if (this.st && this.st.install && this.st.install.state === 'installing') {
        this.timer = setTimeout(async () => { await this.load(); if (box.isConnected) { paint(); poll(); } }, 1500);
      }
    };
    const post = body => api('/api/arena/voice', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
      .then(async r => { const j = await r.json().catch(() => ({})); if (!r.ok) throw new Error(j.error || r.status); return j; });
    const wire = () => {
      box.querySelectorAll('[data-voice-install]').forEach(b => b.onclick = async () => {
        b.disabled = true;
        try { await post({ action: 'install', lang: b.dataset.voiceInstall }); } catch (e) { msg('⚠ ' + e.message); }
        await this.load(); paint(); poll();
      });
      box.querySelectorAll('[data-voice-test]').forEach(b => b.onclick = async () => {
        b.disabled = true; msg(T('prefs.voice.testing'));
        try {
          const r = await post({ action: 'test', who: b.dataset.voiceTest, lang: b.dataset.lang });
          const a = await api('/api/arena/voice?k=' + encodeURIComponent(r.key));
          if (!a.ok) throw new Error(a.status);
          const el = new Audio(URL.createObjectURL(await a.blob()));
          await el.play();
          msg('🔊 ' + T('prefs.voice.playing'));
        } catch (e) { msg('⚠ ' + T('prefs.voice.testFail') + ' ' + (e.message || e)); }
        b.disabled = false;
      });
      box.querySelectorAll('[data-voice-engine] button').forEach(b => b.onclick = async () => {
        const c = Object.assign({}, (this.st && this.st.config) || {}, { engine: b.dataset.v });
        try { await post({ action: 'config', config: c }); } catch (e) { msg('⚠ ' + e.message); }
        await this.load(); paint();
      });
    };
    paint(); poll();
  },
};
