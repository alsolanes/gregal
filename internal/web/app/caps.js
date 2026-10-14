// Capacitats d'agent a la UI: el xip de les apostes de confiança a la fila
// de l'eina («bet 80% ✓»), el bloc del crític a la targeta del pla i el
// «Prova N maneres» (best-of-N en worktrees) del menú ⋯. El servidor ho
// fa tot (internal/web/caps.go); aquí només es pinta i es demana.
(function () {
  const $ = id => document.getElementById(id);
  const G = () => window.gregal;
  const T = (key, fallback) => {
    try { const v = window.gregalT && window.gregalT(key); if (v && v !== key) return v; } catch (e) {}
    return fallback;
  };
  const esc = s => String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

  // ---- apostes ----
  function betChip(card, d) {
    if (!d) return;
    const ok = String(d.passed) === 'true';
    const chip = document.createElement('span');
    chip.className = 'bet-chip ' + (ok ? 'bet-ok' : 'bet-bad');
    chip.textContent = T('bet.label', 'bet') + ' ' + (d.p || '?') + '% ' + (ok ? '✓' : '✗');
    chip.title = T('bet.title', 'Predicted chance that this check passes, against the real outcome') + (d.command ? ' · ' + d.command : '');
    const summary = card && card.querySelector && card.querySelector('summary');
    if (summary) summary.appendChild(chip);
  }

  // ---- crític del pla ----
  const KIND = { missing: 'critic.missing', risky: 'critic.risky', assumption: 'critic.assumption' };
  function criticHTML(c) {
    if (!c || c.skipped) return '';
    const issues = Array.isArray(c.issues) ? c.issues : [];
    if (!issues.length) return '<div class="plan-critic plan-critic-ok">' + esc(T('critic.none', 'Critic: no concerns')) + '</div>';
    const title = T('critic.concerns', 'Critic: {n} concerns').replace('{n}', issues.length);
    return '<details class="plan-critic"><summary>' + esc(title) + '</summary><ul>' +
      issues.map(i => '<li><b class="critic-kind critic-' + esc(i.kind) + '">' + esc(T(KIND[i.kind] || '', i.kind)) + '</b> ' + esc(i.text) + '</li>').join('') +
      '</ul>' + (c.model ? '<div class="faint">' + esc(c.model) + '</div>' : '') + '</details>';
  }
  // Text que acompanya «Executa el pla»: en autònom, revisar-lo una vegada
  // amb la crítica abans d'executar; en code, tenir-la present.
  function criticPrompt(c, auto) {
    const issues = c && Array.isArray(c.issues) ? c.issues : [];
    if (!issues.length) return '';
    return '\n\nPlan critic concerns:\n' + issues.map(i => '- [' + i.kind + '] ' + i.text).join('\n') +
      (auto ? '\nRevise the plan ONCE to address the valid concerns (say briefly why you ignore any), then execute it.'
        : '\nAddress the valid concerns while executing the plan.');
  }

  // ---- best-of-N ----
  function fmtCost(v) { return typeof v === 'number' ? '$' + v.toFixed(v < 0.01 ? 4 : 2) : '–'; }
  function checksCell(a) {
    if (a.checks_passed === true) return '<span class="bo-ok">✓ ' + esc(T('bestof.pass', 'pass')) + '</span>';
    if (a.checks_passed === false) return '<span class="bo-bad">✗ ' + esc(T('bestof.fail', 'fail')) + '</span>';
    return '<span class="faint">–</span>';
  }
  function renderBestOf(box, d) {
    const atts = Array.isArray(d.attempts) ? d.attempts : [];
    const head = '<div class="goal-head"><b>⑂ ' + esc(T('bestof.title', 'Try N ways')) + ' · ' + atts.length + '</b><span>' +
      esc(d.running ? T('bestof.running', 'running…') : (d.applied ? T('bestof.applied', 'applied #{n}').replace('{n}', d.applied) : T('bestof.ready', 'ready to compare'))) + '</span></div>';
    const rows = atts.map(a => {
      const rec = d.recommended === a.index && !d.running;
      const canApply = !d.running && !d.applied && a.state === 'done' && a.diff_bytes > 0;
      return '<tr class="' + (rec ? 'bo-rec' : '') + '"><td>#' + a.index + (rec ? ' <span class="bo-badge">' + esc(T('bestof.recommended', 'best')) + '</span>' : '') + '</td>' +
        '<td title="' + esc(a.worktree || '') + '">' + esc(a.model || a.role || '') + '</td>' +
        '<td>' + esc(T('bestof.state.' + a.state, a.state)) + (a.error ? ' <span class="bo-bad" title="' + esc(a.error) + '">!</span>' : '') + '</td>' +
        '<td>' + ((a.files || []).length) + ' <span class="faint">+' + (a.added || 0) + ' −' + (a.deleted || 0) + '</span></td>' +
        '<td>' + checksCell(a) + '</td>' +
        '<td title="' + esc(a.score_note || '') + '">' + (typeof a.score === 'number' ? a.score + '/10' : '–') + '</td>' +
        '<td>' + fmtCost(a.cost_usd) + '</td>' +
        '<td>' + (canApply ? '<button data-apply="' + a.index + '">' + esc(T('bestof.apply', 'Apply')) + '</button>' : '') + '</td></tr>';
    }).join('');
    const details = atts.filter(a => (a.files || []).length || a.answer).map(a =>
      '<details class="bo-detail"><summary>#' + a.index + ' · ' + esc((a.files || []).slice(0, 6).join(', ') || T('bestof.noChanges', 'no changes')) + '</summary>' +
      '<div class="out">' + (G() && G().md ? G().md(String(a.answer || '').slice(0, 2500)) : esc(a.answer || '')) + '</div></details>').join('');
    box.querySelector('.body').innerHTML = '<div class="goal bestof">' + head +
      '<table class="bo-table"><thead><tr><th></th><th>' + esc(T('bestof.model', 'Model')) + '</th><th>' + esc(T('bestof.stateCol', 'State')) +
      '</th><th>' + esc(T('bestof.files', 'Files')) + '</th><th>' + esc(T('bestof.checks', 'Checks')) + '</th><th>' + esc(T('bestof.score', 'Score')) +
      '</th><th>' + esc(T('bestof.cost', 'Cost')) + '</th><th></th></tr></thead><tbody>' + rows + '</tbody></table>' + details +
      (d.message ? '<div class="faint bo-msg">' + esc(d.message) + '</div>' : '') +
      (!d.applied ? '<div class="goal-row"><button data-discard>' + esc(T('bestof.discard', 'Discard all')) + '</button></div>' : '') + '</div>';
    box.querySelectorAll('[data-apply]').forEach(b => b.onclick = () => apply(box, d.id, +b.dataset.apply));
    const del = box.querySelector('[data-discard]');
    if (del) del.onclick = () => discard(box, d.id);
  }
  async function poll(box, id) {
    for (;;) {
      await new Promise(r => setTimeout(r, 2000));
      let d;
      try {
        const r = await G().api('/api/bestof?id=' + encodeURIComponent(id));
        if (!r.ok) return;
        d = await r.json();
      } catch (e) { continue; }
      renderBestOf(box, d);
      if (!d.running) {
        G().activityItem('best-of', T('bestof.title', 'Try N ways'), T('bestof.ready', 'ready to compare'), 'ok');
        return;
      }
    }
  }
  async function apply(box, id, index) {
    if (!confirm(T('bestof.confirmApply', 'Apply attempt #{n} to the main working tree (no commit) and remove the other worktrees?').replace('{n}', index))) return;
    const r = await G().api('/api/bestof/apply', { method: 'POST', body: JSON.stringify({ id, attempt: index }) });
    if (!r.ok) { G().sys(T('bestof.error', 'Try N ways failed: ') + await r.text()); return; }
    const j = await r.json();
    G().sys(j.message || '');
    G().activityItem('best-of', T('bestof.applied', 'applied #{n}').replace('{n}', index), (j.removed || []).join(', '), 'ok');
    const s = await G().api('/api/bestof?id=' + encodeURIComponent(id));
    if (s.ok) renderBestOf(box, await s.json());
    G().refresh();
  }
  async function discard(box, id) {
    if (!confirm(T('bestof.confirmDiscard', 'Remove all the attempt worktrees and their changes?'))) return;
    const r = await G().api('/api/bestof?id=' + encodeURIComponent(id), { method: 'DELETE' });
    if (!r.ok) { G().sys(T('bestof.error', 'Try N ways failed: ') + await r.text()); return; }
    box.querySelector('.body').innerHTML = '<span class="faint">' + esc(T('bestof.discarded', 'Attempts discarded.')) + '</span>';
  }
  async function start() {
    const input = $('in');
    const task = input ? input.value.trim() : '';
    if (!task) { G().sys(T('bestof.empty', 'Write the task in the field first, then choose Try N ways.')); return; }
    const raw = prompt(T('bestof.askN', 'How many attempts? (2-4)'), '2');
    if (raw == null) return;
    const n = parseInt(raw, 10);
    if (!(n >= 2 && n <= 4)) { G().sys(T('bestof.badN', 'Choose between 2 and 4 attempts.')); return; }
    const models = (prompt(T('bestof.askModels', 'Models per attempt (optional, comma-separated roles or provider/model; empty = current code model)'), '') || '')
      .split(',').map(s => s.trim()).filter(Boolean);
    input.value = ''; input.dispatchEvent(new Event('input'));
    G().add('u', '', G().md ? G().md(task) : esc(task));
    const box = G().add('a', '', '<span class="spin">' + esc(T('bestof.starting', 'Creating worktrees…')) + '</span>');
    G().activityItem('best-of', T('bestof.title', 'Try N ways'), task.slice(0, 70), 'warn');
    try {
      const r = await G().api('/api/bestof', { method: 'POST', body: JSON.stringify({ task, n, models }) });
      if (!r.ok) throw new Error(await r.text());
      const d = await r.json();
      renderBestOf(box, d);
      poll(box, d.id);
    } catch (e) {
      box.querySelector('.body').textContent = T('bestof.error', 'Try N ways failed: ') + (e.message || e);
    }
  }
  // ---- /debug <tasca> ----
  // Passa la sessió a mode debug i envia la tasca. Escolta en fase de
  // captura, com /btw (extras.js), perquè el text no arribi a l'agent tal qual.
  function interceptDebug(text) {
    const m = /^\/debug(?:\s+([\s\S]*))?$/i.exec(String(text || '').trim());
    if (!m) return false;
    const input = $('in');
    const task = (m[1] || '').trim();
    if (input) { input.value = task; input.dispatchEvent(new Event('input')); }
    Promise.resolve(G().setMode('debug')).then(() => {
      if (task && $('send')) $('send').click();
      else G().sys(T('mode.detail.debug', 'Debug mode'));
    });
    return true;
  }
  document.addEventListener('keydown', e => {
    if (e.target && e.target.id === 'in' && e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
      if (interceptDebug(e.target.value)) { e.preventDefault(); e.stopImmediatePropagation(); }
    }
  }, true);
  document.addEventListener('click', e => {
    const send = e.target && e.target.closest && e.target.closest('#send');
    const input = $('in');
    if (send && input && interceptDebug(input.value)) { e.preventDefault(); e.stopImmediatePropagation(); }
  }, true);

  function wire() {
    const b = $('bestofBtn');
    if (b && !b.dataset.wired) { b.dataset.wired = '1'; b.onclick = () => start(); }
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', wire); else wire();

  window.gregalCaps = { betChip, criticHTML, criticPrompt, renderBestOf, interceptDebug };
})();
