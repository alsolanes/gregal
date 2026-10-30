// Extres del xat: preguntes al marge (/btw), resums de sessió (/recap i el
// botó «Resumeix» quan tornes a una sessió amb novetats) i memòria
// automàtica («Ho recordo?» en acabar un torn de codi, i la secció de
// memòria a Preferències amb el diff abans d'escriure).
//
// Res d'això toca el torn en marxa ni l'historial: /btw i /recap són
// crides al marge, i la memòria només s'escriu quan l'usuari ho accepta.
// El mòdul s'enganxa a la pàgina per fora (escolta el composer en fase de
// captura i observa el panell de preferències) per no engreixar index.html.
(function () {
  const $ = id => document.getElementById(id);
  const G = () => window.gregal;
  const T = (key, fallback) => {
    try { const v = window.gregalT && window.gregalT(key); if (v && v !== key) return v; } catch (e) {}
    return fallback;
  };
  const esc = s => String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const md = text => { try { return G().md(text); } catch (e) { return esc(text).replace(/\n/g, '<br>'); } };
  const session = () => window.gregalSession || 'default';
  const store = () => { try { return window.localStorage; } catch (e) { return null; } };

  // ---- targetes al marge ----
  function stack() {
    let box = $('asideStack');
    if (!box) {
      box = document.createElement('div');
      box.id = 'asideStack';
      box.setAttribute('aria-live', 'polite');
      document.body.appendChild(box);
    }
    return box;
  }
  function card(title, html, kind) {
    const el = document.createElement('section');
    el.className = 'aside-card' + (kind ? ' aside-' + kind : '');
    el.innerHTML = '<header><b></b><button type="button" class="aside-x"></button></header><div class="aside-body"></div><div class="aside-actions"></div>';
    el.querySelector('b').textContent = title;
    const x = el.querySelector('.aside-x');
    x.textContent = '×';
    x.setAttribute('aria-label', T('extras.close', 'Close'));
    x.onclick = () => el.remove();
    el.querySelector('.aside-body').innerHTML = html;
    stack().appendChild(el);
    return el;
  }
  const setBody = (el, html) => { el.querySelector('.aside-body').innerHTML = html; };

  // ---- /btw ----
  async function btw(question) {
    const q = question.trim();
    if (!q) { card(T('extras.btw.title', 'By the way'), esc(T('extras.btw.empty', 'Write a question after /btw.')), 'btw'); return; }
    const el = card(T('extras.btw.title', 'By the way') + ': ' + (q.length > 60 ? q.slice(0, 60) + '…' : q), '<span class="spin">◌</span> ' + esc(T('extras.btw.asking', 'Asking on the side…')), 'btw');
    try {
      const r = await G().api('/api/btw', { method: 'POST', body: JSON.stringify({ question: q }) });
      if (!r.ok) throw new Error((await r.text()).trim() || ('HTTP ' + r.status));
      const j = await r.json();
      setBody(el, md(j.answer || ''));
    } catch (e) {
      setBody(el, '<span class="aside-bad">' + esc(T('extras.btw.failed', 'The side question failed.')) + '</span> ' + esc(e && e.message || ''));
    }
  }

  // ---- /recap ----
  async function recap(body, title) {
    const el = card(title || T('extras.recap.title', 'Recap'), '<span class="spin">◌</span> ' + esc(T('extras.recap.loading', 'Summarizing…')), 'recap');
    try {
      const r = await G().api('/api/recap', { method: 'POST', body: JSON.stringify(body || {}) });
      if (!r.ok) throw new Error((await r.text()).trim() || ('HTTP ' + r.status));
      const j = await r.json();
      setBody(el, j.empty ? esc(T('extras.recap.empty', 'Nothing new to summarize.')) : md(j.summary || ''));
    } catch (e) {
      setBody(el, '<span class="aside-bad">' + esc(T('extras.recap.failed', 'The recap failed.')) + '</span> ' + esc(e && e.message || ''));
    }
  }

  // El composer: /btw i /recap no són missatges per a l'agent. Escoltem en
  // fase de captura (document) per passar abans que l'Enter encui el text
  // mentre l'agent treballa.
  function intercept(text) {
    const m = /^\/(btw|recap)(?:\s+([\s\S]*))?$/i.exec(text.trim());
    if (!m) return false;
    const input = $('in');
    if (input) { input.value = ''; input.dispatchEvent(new Event('input')); input.style.height = 'auto'; }
    if (m[1].toLowerCase() === 'btw') { btw(m[2] || ''); return true; }
    const arg = (m[2] || '').trim();
    if (/^\d+$/.test(arg)) recap({ turns: parseInt(arg, 10) });
    else if (arg) recap({ since: arg });
    else recap({});
    return true;
  }
  document.addEventListener('keydown', e => {
    if (e.target && e.target.id === 'in' && e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
      if (intercept(e.target.value || '')) { e.preventDefault(); e.stopImmediatePropagation(); }
    }
  }, true);
  document.addEventListener('click', e => {
    const send = e.target && e.target.closest && e.target.closest('#send');
    const input = $('in');
    if (send && input && intercept(input.value || '')) { e.preventDefault(); e.stopImmediatePropagation(); }
  }, true);

  // ---- novetats sense veure → botó «Resumeix» ----
  // Per sessió es recorda fins a quin event del registre durable s'ha vist
  // i quan. En tornar (càrrega, pestanya visible després d'una estona,
  // canvi de sessió) es compten les novetats; si n'hi ha, surt el botó.
  const SEEN = 'gregal.recap.seen.';
  const noise = new Set(['token', 'progress', 'ping']);
  const readSeen = id => { try { return JSON.parse(store().getItem(SEEN + id) || 'null'); } catch (e) { return null; } };
  const writeSeen = (id, v) => { try { store().setItem(SEEN + id, JSON.stringify(v)); } catch (e) {} };
  async function scan(after) {
    let cursor = after || 0, count = 0;
    for (let i = 0; i < 20; i++) {
      const r = await G().api('/api/v2/events?limit=500&after=' + cursor);
      if (!r.ok) return null;
      const j = await r.json();
      const evs = Array.isArray(j.events) ? j.events : [];
      count += evs.filter(e => e && !noise.has(e.kind)).length;
      cursor = j.next || cursor;
      if (evs.length < 500) break;
    }
    return { cursor, count };
  }
  let lastSession = null, hiddenAt = 0, checking = false;
  async function checkUnseen() {
    if (checking || !G() || !G().api) return;
    checking = true;
    try {
      const id = session();
      const seen = readSeen(id);
      const res = await scan(seen ? seen.cursor : 0);
      if (!res) return;
      if (seen && res.count >= 3 && !(G().isBusy && G().isBusy())) showRecapChip(res.count, seen.at);
      writeSeen(id, { cursor: res.cursor, at: new Date().toISOString() });
    } catch (e) {} finally { checking = false; }
  }
  async function markSeen() {
    if (document.hidden || checking || !G() || !G().api) return;
    const id = session();
    const seen = readSeen(id);
    if (!seen) return checkUnseen();
    try {
      const res = await scan(seen.cursor);
      if (res) writeSeen(id, { cursor: res.cursor, at: new Date().toISOString() });
    } catch (e) {}
  }
  function showRecapChip(n, since) {
    document.querySelectorAll('.recap-chip').forEach(el => el.remove());
    const chip = document.createElement('div');
    chip.className = 'recap-chip';
    const label = document.createElement('span');
    label.textContent = T('extras.recap.unseen', '{n} updates since you left').replace('{n}', n);
    const go = document.createElement('button');
    go.type = 'button'; go.className = 'primary';
    go.textContent = T('extras.recap.button', 'Recap');
    go.onclick = () => { chip.remove(); recap({ since }); };
    const x = document.createElement('button');
    x.type = 'button'; x.className = 'ghost'; x.textContent = '×';
    x.setAttribute('aria-label', T('extras.close', 'Close'));
    x.onclick = () => chip.remove();
    chip.append(label, go, x);
    // A la pàgina d'inici (hero) va sobre les converses recents; si no,
    // a la pila de targetes al marge.
    const home = $('homeContext'), hero = $('hero');
    if (home && hero && !hero.hidden && hero.style.display !== 'none') home.before(chip);
    else stack().appendChild(chip);
  }
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) { hiddenAt = Date.now(); markSeen(); return; }
    if (hiddenAt && Date.now() - hiddenAt > 60000) checkUnseen();
    hiddenAt = 0;
  });

  // ---- memòria: «Ho recordo?» ----
  let wasBusy = false, memPoll = 0;
  function memChip(note) {
    const el = card(T('extras.mem.title', 'Remember this?'), esc(note), 'mem');
    const actions = el.querySelector('.aside-actions');
    const yes = document.createElement('button'); yes.type = 'button'; yes.className = 'primary'; yes.textContent = T('extras.mem.accept', 'Remember');
    const no = document.createElement('button'); no.type = 'button'; no.className = 'ghost'; no.textContent = T('extras.mem.dismiss', 'Dismiss');
    const act = async action => {
      yes.disabled = no.disabled = true;
      try {
        const r = await G().api('/api/memory/suggest', { method: 'POST', body: JSON.stringify({ action, note }) });
        if (r.ok && action === 'accept') { actions.textContent = T('extras.mem.saved', 'Saved to .gregal/memory.md'); setTimeout(() => el.remove(), 2500); return; }
      } catch (e) {}
      el.remove();
    };
    yes.onclick = () => act('accept');
    no.onclick = () => act('dismiss');
    el.querySelector('.aside-x').onclick = () => act('dismiss');
    actions.append(yes, no);
  }
  async function pollMemory(attempt) {
    clearTimeout(memPoll);
    try {
      const r = await G().api('/api/memory/suggest');
      if (!r.ok) return;
      const j = await r.json();
      if (j.state === 'ready' && Array.isArray(j.notes)) {
        document.querySelectorAll('.aside-mem').forEach(el => el.remove());
        j.notes.forEach(memChip);
        return;
      }
      if (j.state === 'pending' && attempt < 8) memPoll = setTimeout(() => pollMemory(attempt + 1), 2500 * (attempt + 1));
    } catch (e) {}
  }

  // ---- Preferències: secció de memòria ----
  async function memorySection(body) {
    const sec = document.createElement('section');
    sec.id = 'memSection';
    sec.className = 'prefs-section mem-section';
    sec.innerHTML = '<h3 class="pf-sec"></h3><p class="faint"></p><ul class="mem-notes"></ul><div class="mem-diff" hidden></div><div class="row"><button type="button" class="ghost mem-tidy"></button></div>';
    sec.querySelector('h3').textContent = T('extras.mem.section', 'Project memory');
    sec.querySelector('p').textContent = T('extras.mem.help', 'Durable notes in .gregal/memory.md that every session reads.');
    const list = sec.querySelector('.mem-notes'), diffBox = sec.querySelector('.mem-diff'), tidy = sec.querySelector('.mem-tidy');
    tidy.textContent = T('extras.mem.tidy', 'Tidy notes');
    // Preferències va per grups: la memòria del projecte és de «Models i agent».
    (body.querySelector('#prefsG-agent .pf-gbody') || body).appendChild(sec);
    const paint = notes => {
      list.replaceChildren();
      if (!notes.length) { const li = document.createElement('li'); li.className = 'faint'; li.textContent = T('extras.mem.none', 'No project notes yet.'); list.append(li); }
      for (const n of notes) { const li = document.createElement('li'); li.textContent = n; list.append(li); }
      tidy.disabled = notes.length < 2;
    };
    const load = async () => {
      try { const r = await G().api('/api/memory/tidy'); if (r.ok) paint((await r.json()).notes || []); } catch (e) {}
    };
    tidy.onclick = async () => {
      tidy.disabled = true;
      diffBox.hidden = false;
      diffBox.textContent = T('extras.mem.tidying', 'Proposing a tidier list…');
      try {
        const r = await G().api('/api/memory/tidy', { method: 'POST', body: '{}' });
        if (!r.ok) throw new Error((await r.text()).trim());
        const j = await r.json();
        if (!j.changed) { diffBox.textContent = T('extras.mem.nochange', 'Nothing to merge or drop.'); tidy.disabled = false; return; }
        diffBox.replaceChildren();
        const pre = document.createElement('pre');
        for (const line of String(j.diff || '').split('\n')) {
          const span = document.createElement('span');
          span.className = line.startsWith('+') ? 'add' : line.startsWith('-') ? 'del' : '';
          span.textContent = line + '\n';
          pre.append(span);
        }
        const row = document.createElement('div'); row.className = 'row';
        const apply = document.createElement('button'); apply.type = 'button'; apply.className = 'primary'; apply.textContent = T('extras.mem.apply', 'Apply changes');
        const cancel = document.createElement('button'); cancel.type = 'button'; cancel.className = 'ghost'; cancel.textContent = T('extras.mem.cancel', 'Cancel');
        cancel.onclick = () => { diffBox.hidden = true; diffBox.replaceChildren(); tidy.disabled = false; };
        apply.onclick = async () => {
          apply.disabled = true;
          const w = await G().api('/api/memory/tidy/apply', { method: 'POST', body: JSON.stringify({ after: j.after || [], hash: j.hash || '' }) });
          diffBox.textContent = w.ok ? T('extras.mem.applied', 'Notes updated.') : w.status === 409 ? T('extras.mem.conflict', 'The notes changed meanwhile; tidy again.') : ((await w.text()).trim() || 'error');
          load();
        };
        row.append(apply, cancel);
        diffBox.append(pre, row);
      } catch (e) {
        diffBox.textContent = (e && e.message) || 'error';
        tidy.disabled = false;
      }
    };
    load();
  }

  const CSS = `
#asideStack{position:fixed;right:16px;bottom:96px;z-index:60;display:flex;flex-direction:column;gap:8px;max-width:min(400px,calc(100vw - 32px));max-height:60vh;overflow:auto}
#asideStack:empty{display:none}
.aside-card{background:var(--panel);border:1px solid var(--line,rgba(127,127,127,.25));border-left:3px solid var(--accent,#8193FF);border-radius:8px;padding:8px 10px;box-shadow:0 6px 20px rgba(0,0,0,.18);font-size:13px}
.aside-card header{display:flex;align-items:center;gap:8px;justify-content:space-between}
.aside-card header b{font-size:12px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.aside-x{background:none;border:0;color:var(--muted);cursor:pointer;font-size:16px;line-height:1}
.aside-body{margin-top:4px;max-height:40vh;overflow:auto}
.aside-actions{display:flex;gap:6px;margin-top:6px}
.aside-actions:empty{display:none}
.aside-mem{border-left-color:var(--amber,#c98a2b)}
.aside-bad{color:var(--red,#c0392b)}
.recap-chip{display:flex;align-items:center;gap:8px;padding:6px 10px;border:1px solid var(--line,rgba(127,127,127,.25));border-radius:999px;background:var(--panel);font-size:12px;margin:6px auto;width:max-content;max-width:100%}
.mem-section .mem-notes{list-style:none;padding:0;margin:6px 0;font-size:12px;max-height:180px;overflow:auto}
.mem-section .mem-notes li{padding:3px 0;border-bottom:1px solid var(--line,rgba(127,127,127,.15))}
.mem-diff pre{font-size:11px;white-space:pre-wrap;max-height:220px;overflow:auto}
.mem-diff .add{color:var(--green,#6EDB95)}
.mem-diff .del{color:var(--red,#c0392b);text-decoration:line-through}
`;
  function wire() {
    const style = document.createElement('style');
    style.textContent = CSS;
    document.head.appendChild(style);
    // Canvi de sessió: el botó «Resumeix» mira les novetats de la nova.
    setInterval(() => {
      const id = session();
      if (id !== lastSession) { lastSession = id; checkUnseen(); }
      const busy = !!(G().isBusy && G().isBusy());
      if (wasBusy && !busy) memPoll = setTimeout(() => pollMemory(0), 1500);
      wasBusy = busy;
    }, 1500);
    setInterval(markSeen, 20000);
    const body = $('prefsBody');
    if (body) new MutationObserver(() => {
      if (body.children.length && !$('memSection')) memorySection(body);
    }).observe(body, { childList: true });
    window.gregalExtras = { btw, recap };
  }
  function start() {
    if (G() && G().api) { wire(); return; }
    setTimeout(start, 200);
  }
  start();
})();
