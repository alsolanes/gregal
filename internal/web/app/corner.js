// El Racó dels vents: la pàgina que fan ells (corner.go). Aquí es mostra en
// un marc aïllat (iframe sandbox sense mateix origen: no pot llegir res de
// l'app ni connectar enlloc) i es fa de pont: quan la pàgina diu «hello» li
// passem les dades dels vents, i quan un visitant hi pregunta alguna cosa a
// un vent ho passem a /api/corner/ask. També: diari, versions (veure-les i
// tornar-hi), comentaris per a la propera reunió, cridar-los i aturar-los.
const $ = id => document.getElementById(id);
const T = key => (window.gregalT ? window.gregalT(key) : '') || key;
const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

// Quant fa (minuts, hores, dies), curt.
export function ago(unix, now = Date.now() / 1000, lang = 'ca') {
  const s = Math.max(0, Math.round(now - unix));
  const ca = lang === 'ca';
  if (s < 60) return ca ? 'ara mateix' : 'just now';
  if (s < 3600) return (ca ? 'fa ' : '') + Math.round(s / 60) + ' min' + (ca ? '' : ' ago');
  if (s < 86400) return (ca ? 'fa ' : '') + Math.round(s / 3600) + ' h' + (ca ? '' : ' ago');
  return (ca ? 'fa ' : '') + Math.round(s / 86400) + (ca ? ' d' : ' d ago');
}

const KIND_ICON = { stopped: '⏹', meeting: '🗣', rest: '☕', built: '🛠', review: '✅', published: '✨', rejected: '↩', failed: '⚠', revert: '⏪', comment: '💬', talk: '👋' };
export function entryLine(e) {
  return { icon: KIND_ICON[e.kind] || '·', who: e.name || '', text: e.text || '', kind: e.kind };
}

const api = (path, opts) => window.api(path, opts).then(async r => {
  if (!r.ok) throw new Error((await r.text()).slice(0, 200) || r.status);
  return r.json();
});

const corner = {
  st: null, preview: 0, timer: 0, data: null,

  async load() {
    const page = $('racoPage');
    if (!page || typeof window.api !== 'function') return;
    if (!page.dataset.ready) this.build(page);
    try { this.st = await api('/api/corner'); } catch (e) { $('racoStatus').textContent = e.message; return; }
    this.paint();
    await this.showPage(this.preview || 0);
    clearTimeout(this.timer);
    if (this.st.running && !page.hidden) this.timer = setTimeout(() => this.load(), 6000);
  },

  build(page) {
    page.dataset.ready = '1';
    page.innerHTML = `<div class="raco">
      <header class="raco-head">
        <div><div class="page-kicker">${esc(T('corner.kicker'))}</div><h1 class="page-title">${esc(T('corner.title'))}</h1>
        <p class="page-copy">${esc(T('corner.lead'))}</p></div>
        <div class="raco-actions"><span id="racoStatus" class="raco-status" role="status"></span>
          <button type="button" id="racoMeet" class="primary"></button><button type="button" id="racoPause"></button>
          <span id="racoSpend" class="raco-spend"></span></div>
      </header>
      <div class="raco-body">
        <div class="raco-stage"><div id="racoPreviewBar" class="raco-preview" hidden></div>
          <iframe id="racoFrame" sandbox="allow-scripts" title="${esc(T('corner.title'))}" referrerpolicy="no-referrer"></iframe>
          <div id="racoEmpty" class="raco-empty" hidden></div></div>
        <aside class="raco-side">
          <form id="racoCommentForm" class="raco-comment"><textarea id="racoComment" rows="2" maxlength="1000" placeholder="${esc(T('corner.commentPlaceholder'))}"></textarea>
            <button type="submit">${esc(T('corner.send'))}</button></form>
          <h3>${esc(T('corner.journal'))}</h3><ol id="racoJournal" class="raco-journal"></ol>
          <h3>${esc(T('corner.versions'))}</h3><ol id="racoVersions" class="raco-versions"></ol>
          <p id="racoBudget" class="raco-budget"></p>
        </aside>
      </div></div>`;
    // Mentre treballen, el mateix botó ho atura a l'instant (no es publica res).
    $('racoMeet').onclick = async () => {
      $('racoMeet').disabled = true;
      const action = this.st && this.st.running ? 'stop' : 'meet';
      try { await api('/api/corner', { method: 'POST', body: JSON.stringify({ action }) }); } catch (e) { $('racoStatus').textContent = e.message; }
      setTimeout(() => this.load(), 600);
    };
    $('racoPause').onclick = async () => {
      await api('/api/corner', { method: 'POST', body: JSON.stringify({ action: this.st && this.st.paused ? 'resume' : 'pause' }) }).catch(() => {});
      this.load();
    };
    $('racoCommentForm').onsubmit = async e => {
      e.preventDefault();
      const text = $('racoComment').value.trim();
      if (!text) return;
      try { await api('/api/corner', { method: 'POST', body: JSON.stringify({ action: 'comment', text }) }); $('racoComment').value = ''; } catch (err) { $('racoStatus').textContent = err.message; }
      this.load();
    };
    // Pont amb la pàgina: només missatges del seu propi marc.
    window.addEventListener('message', e => {
      const frame = $('racoFrame');
      if (!frame || e.source !== frame.contentWindow) return;
      const m = e.data || {};
      if (m.gregalCorner === 'hello' && this.data) frame.contentWindow.postMessage({ gregalCorner: 'init', data: this.data }, '*');
      if (m.gregalCorner === 'ask') {
        api('/api/corner/ask', { method: 'POST', body: JSON.stringify({ wind: m.wind, text: m.text }) })
          .then(r => frame.contentWindow.postMessage({ gregalCorner: 'reply', id: m.id, reply: r.reply }, '*'))
          .catch(err => frame.contentWindow.postMessage({ gregalCorner: 'reply', id: m.id, error: String(err.message || err) }, '*'));
      }
    });
  },

  paint() {
    const st = this.st, lang = (window.gregalLang && window.gregalLang()) || document.documentElement.lang || 'ca';
    const cur = (st.versions || []).find(v => v.n === st.current);
    let status = st.running ? T('corner.working') : st.paused ? T(st.current || (st.journal || []).length ? 'corner.paused' : 'corner.off') : '';
    if (!status && cur) status = cur.reverted_from ? T('corner.versionReverted').replace('{n}', cur.n).replace('{from}', cur.reverted_from)
      : T('corner.versionBy').replace('{n}', cur.n).replace('{builder}', cur.builder_name || '?').replace('{reviewer}', cur.reviewer_name || '?').replace('{ago}', ago(cur.at, undefined, lang));
    $('racoStatus').textContent = status;
    $('racoMeet').textContent = T(st.running ? 'corner.stopNow' : 'corner.meet');
    $('racoMeet').classList.toggle('danger', !!st.running);
    $('racoMeet').disabled = false;
    $('racoPause').textContent = T(st.paused ? 'corner.resume' : 'corner.pause');
    $('racoPause').classList.toggle('primary', !!st.paused && !st.running);
    const bb = st.budget || {};
    $('racoSpend').textContent = T('corner.spend').replace('{c}', bb.calls || 0).replace('{mc}', bb.max_calls || 0);
    $('racoJournal').innerHTML = (st.journal || []).slice(0, 30).map(e => {
      const l = entryLine(e);
      return `<li class="k-${esc(l.kind)}"><span class="ic">${l.icon}</span><div><b>${esc(l.who)}</b> <span>${esc(l.text)}</span><small>${esc(ago(e.at, undefined, lang))}</small></div></li>`;
    }).join('') || `<li class="none">${esc(T('corner.nothingYet'))}</li>`;
    $('racoVersions').innerHTML = (st.versions || []).map(v => `<li${v.n === st.current ? ' class="cur"' : ''}><div><b>v${v.n}</b> ${esc(v.summary)}
      <small>${esc(v.builder_name ? v.builder_name + ' · ' + T('corner.reviewedBy') + ' ' + (v.reviewer_name || '') : '')} · ${esc(ago(v.at, undefined, lang))}</small></div>
      <span><button type="button" data-see="${v.n}">${esc(T('corner.see'))}</button>${v.n === st.current ? '' : `<button type="button" data-back="${v.n}">${esc(T('corner.back'))}</button>`}</span></li>`).join('');
    $('racoVersions').querySelectorAll('[data-see]').forEach(b => b.onclick = () => this.showPage(+b.dataset.see));
    $('racoVersions').querySelectorAll('[data-back]').forEach(b => b.onclick = async () => {
      await api('/api/corner', { method: 'POST', body: JSON.stringify({ action: 'revert', version: +b.dataset.back }) }).catch(() => {});
      this.preview = 0; this.load();
    });
    const b = st.budget || {};
    $('racoBudget').textContent = T('corner.budget').replace('{s}', b.sessions || 0).replace('{ms}', b.max_sessions || 0).replace('{t}', b.talks || 0).replace('{mt}', b.max_talks || 0);
  },

  async showPage(n) {
    const frame = $('racoFrame');
    let page;
    try { page = await api('/api/corner/page' + (n ? '?v=' + n : '')); } catch (e) { return; }
    this.preview = n && n !== this.st.current ? n : 0;
    const bar = $('racoPreviewBar');
    bar.hidden = !this.preview;
    if (this.preview) {
      bar.innerHTML = esc(T('corner.previewing').replace('{n}', this.preview)) + ' <button type="button">' + esc(T('corner.backToCurrent')) + '</button>';
      bar.querySelector('button').onclick = () => this.showPage(0);
    }
    this.data = page.data || null;
    $('racoEmpty').hidden = !!page.doc;
    frame.hidden = !page.doc;
    if (!page.doc) { $('racoEmpty').textContent = T('corner.blank'); frame.removeAttribute('srcdoc'); return; }
    if (frame.dataset.version !== String(page.version)) { frame.dataset.version = String(page.version); frame.srcdoc = page.doc; }
  },
};

window.gregalCorner = corner;
