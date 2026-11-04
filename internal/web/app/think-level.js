// Nivell de raonament (thinking) al compositor, com a OpenCode: una pastilla
// «Alt ▾» al costat del model amb els nivells que el model admet (ve de
// l'estat: think_options). Triar-ne un fa POST /api/think; el servidor el desa
// per usuari. Si el model no raona, la pastilla s'amaga.
(function () {
  'use strict';
  const CA = { default: 'Per defecte', auto: 'Auto', no: 'Sense', low: 'Baix', medium: 'Mitjà', high: 'Alt', max: 'Màxim' };
  const EN = { default: 'Default', auto: 'Auto', no: 'Off', low: 'Low', medium: 'Medium', high: 'High', max: 'Max' };
  const HELP = {
    ca: { default: 'El que digui la configuració', auto: 'Raona només on aporta (planificar, decidir)', no: 'Sense raonament: més ràpid', low: 'Raonament curt', medium: 'Raonament mitjà', high: 'Raonament llarg', max: 'Tot el que el model pugui' },
    en: { default: 'Whatever the config says', auto: 'Thinks only where it helps (planning, deciding)', no: 'No reasoning: faster', low: 'Short reasoning', medium: 'Medium reasoning', high: 'Long reasoning', max: 'As much as the model can' },
  };
  const lang = () => (document.documentElement.lang === 'ca' ? 'ca' : 'en');
  const label = l => (lang() === 'ca' ? CA : EN)[l] || l;
  const title = () => (lang() === 'ca' ? 'Nivell de raonament' : 'Thinking level');

  let pill = null, menu = null, last = null;

  function pillText(r) {
    const level = r.think || 'default';
    // «Per defecte» mostra el que de fet s'aplica, si el config en fixa un.
    const shown = level === 'default' ? (r.think_effective || 'default') : level;
    return shown === 'default' ? (lang() === 'ca' ? 'Raonament' : 'Thinking') : label(shown);
  }

  function close() { if (menu) menu.hidden = true; if (pill) pill.setAttribute('aria-expanded', 'false'); }

  function ensure() {
    if (pill) return;
    const anchor = document.getElementById('modelPill');
    if (!anchor) return;
    pill = document.createElement('button');
    pill.type = 'button'; pill.id = 'thinkPill'; pill.className = 'meta-pill think-pill';
    pill.setAttribute('aria-haspopup', 'listbox'); pill.setAttribute('aria-expanded', 'false');
    pill.hidden = true;
    anchor.insertAdjacentElement('afterend', pill);
    menu = document.createElement('div');
    menu.id = 'thinkMenu'; menu.hidden = true; menu.setAttribute('role', 'listbox');
    document.body.appendChild(menu);
    pill.addEventListener('click', e => { e.stopPropagation(); menu.hidden ? open() : close(); });
    document.addEventListener('click', e => { if (!menu.hidden && !menu.contains(e.target)) close(); });
    document.addEventListener('keydown', e => { if (e.key === 'Escape') close(); });
  }

  function open() {
    if (!last) return;
    const cur = last.think || 'default';
    const help = HELP[lang()];
    menu.innerHTML = '';
    const head = document.createElement('div'); head.className = 'think-head'; head.textContent = title(); menu.append(head);
    for (const l of last.think_options || []) {
      const b = document.createElement('button');
      b.type = 'button'; b.className = 'think-item' + (l === cur ? ' on' : ''); b.setAttribute('role', 'option'); b.setAttribute('aria-selected', String(l === cur));
      b.innerHTML = '<span class="think-mark">' + (l === cur ? '✓' : '') + '</span><span class="think-name"></span><small></small>';
      b.querySelector('.think-name').textContent = label(l);
      b.querySelector('small').textContent = help[l] || '';
      b.addEventListener('click', () => choose(l));
      menu.append(b);
    }
    const r = pill.getBoundingClientRect();
    menu.hidden = false;
    const above = r.top > window.innerHeight / 2;
    menu.style.left = Math.max(8, Math.min(r.left, window.innerWidth - 280)) + 'px';
    menu.style.top = above ? 'auto' : (r.bottom + 6) + 'px';
    menu.style.bottom = above ? (window.innerHeight - r.top + 6) + 'px' : 'auto';
    pill.setAttribute('aria-expanded', 'true');
  }

  async function choose(level) {
    close();
    try {
      const res = await window.gregal.api('/api/think', { method: 'POST', body: JSON.stringify({ level }) });
      if (!res.ok) throw new Error(await res.text());
      window.gregal.refresh();
    } catch (e) { window.gregal.sys((lang() === 'ca' ? 'No he pogut canviar el raonament: ' : 'Could not change thinking: ') + (e.message || e)); }
  }

  function paint(r) {
    last = r; ensure(); if (!pill) return;
    const opts = r.think_options || [];
    pill.hidden = opts.length === 0;
    pill.innerHTML = '<b></b> <span class="mp-caret">▾</span>';
    pill.firstElementChild.textContent = pillText(r);
    pill.title = title();
    pill.setAttribute('aria-label', title());
  }

  document.addEventListener('gregal:state', e => paint(e.detail || {}));
  if (typeof module === 'object' && module.exports) module.exports = { pillText: null };
})();
