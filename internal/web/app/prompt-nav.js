// Navegador de prompts: una barra prima al costat de la conversa amb una
// ratlla per cada missatge teu, com a Copilot o ChatGPT. Passant-hi per
// sobre es veu què havies demanat, amb un clic hi vas, i cada ratlla diu
// com va acabar aquell torn (respost, treballant, aturat). La part pura
// (estats, quina és a la vista, espaiat) també la carreguen els tests.
(function (root) {
  // Una seqüència de marques en ordre de pàgina: 'u' (prompt teu), 'run'
  // (torn en marxa), 'stop' (aturat per tu), 'fail' (error), 'answer'
  // (resposta o feina acabada). Cada prompt pren l'estat del que ve fins al prompt següent.
  function promptStates(marks, busy) {
    const out = [];
    let cur = null;
    for (const m of marks || []) {
      if (m === 'u') { cur = { run: false, stop: false, fail: false, answer: false }; out.push(cur); continue; }
      if (!cur) continue;
      if (m === 'run') cur.run = true;
      else if (m === 'stop') cur.stop = true;
      else if (m === 'fail') cur.fail = true;
      else if (m === 'answer') cur.answer = true;
    }
    return out.map((c, i) => {
      if (c.run) return 'running';
      if (c.stop) return 'stopped';
      if (c.fail) return 'failed';
      if (c.answer) return 'done';
      // L'últim prompt sense res a sota mentre la sessió treballa: s'acaba d'enviar.
      return busy && i === out.length - 1 ? 'running' : 'empty';
    });
  }

  // Quin prompt és «a la vista»: l'últim que comença per sobre d'una línia
  // prop de dalt (80 px o un quart de la finestra, el que sigui menys), on
  // queda el prompt quan s'hi va amb un clic. Si cap ho fa, el primer. Si
  // s'ha arribat al fons, l'últim (el del final no arribaria mai a dalt).
  function activeIndex(tops, viewTop, viewHeight, atBottom) {
    if (!tops || !tops.length) return -1;
    if (atBottom) return tops.length - 1;
    const line = viewTop + Math.min(80, Math.max(0, viewHeight || 0) / 4);
    let first = 0, end = tops.length;
    while (first < end) {
      const mid = (first + end) >>> 1;
      if (tops[mid] <= line) first = mid + 1;
      else end = mid;
    }
    return Math.max(0, first - 1);
  }

  // Distància entre ratlles: àmplia amb pocs prompts, comprimida amb molts
  // perquè no faci una paret; per sota del mínim la barra fa scroll.
  function pitch(n, available, min, max) {
    min = min || 5; max = max || 14;
    if (!(n > 0) || !(available > 0)) return max;
    return Math.max(min, Math.min(max, Math.floor(available / n)));
  }

  function clip(text, max) {
    const s = String(text == null ? '' : text).replace(/\s+/g, ' ').trim();
    max = max || 140;
    return s.length > max ? s.slice(0, max - 1).trimEnd() + '…' : s;
  }

  const core = { promptStates, activeIndex, pitch, clip };
  if (typeof module !== 'undefined' && module.exports) { module.exports = core; return; }

  // --- DOM ---
  const T = (k, sino) => {
    const v = typeof root.gregalT === 'function' ? root.gregalT(k) : '';
    return v && v !== k ? v : sino;
  };
  const STATE_KEY = { done: ['pnav.done', 'Answered'], running: ['pnav.running', 'In progress'], stopped: ['pnav.stopped', 'Stopped'], failed: ['pnav.failed', 'Failed'], empty: ['pnav.empty', 'No answer'] };
  const stateLabel = s => T(STATE_KEY[s][0], STATE_KEY[s][1]);

  // opts: scroller (el que fa scroll), host (posició relativa on va la
  // barra), mark(el) → 'u'|'run'|'fail'|'answer'|null per a cada fill del
  // scroller, text(el) → el text del prompt, busy() opcional.
  function attach(opts) {
    const { scroller, host } = opts;
    if (!scroller || !host || scroller.__promptNav) return scroller && scroller.__promptNav;
    const rail = document.createElement('nav');
    rail.className = 'pnav';
    rail.hidden = true;
    const list = document.createElement('div');
    list.className = 'pnav-list';
    rail.appendChild(list);
    const tip = document.createElement('div');
    tip.className = 'pnav-tip';
    tip.setAttribute('role', 'tooltip');
    tip.id = 'pnavTip' + Math.random().toString(36).slice(2, 8);
    tip.hidden = true;
    host.append(rail, tip);

    let prompts = [], states = [], active = -1, focusIdx = -1, frame = 0, scrollFrame = 0, signature = '';
    let geometry = null, observed = new Set();
    const sizes = root.ResizeObserver ? new ResizeObserver(() => {
      geometry = null;
      scheduleScroll();
    }) : null;

    function collect() {
      const els = [], marks = [];
      for (const el of scroller.children) {
        const m = opts.mark(el);
        if (!m) continue;
        marks.push(m);
        if (m === 'u') els.push(el);
      }
      return { els, states: promptStates(marks, !!(opts.busy && opts.busy())) };
    }

    function place() {
      const visible = scroller.offsetParent !== null && scroller.clientHeight > 0;
      const wide = scroller.clientWidth >= (opts.minWidth || 640);
      // Amb dos prompts es veuen tots dos a la pantalla: la barra només
      // ajuda (i només ocupa la vora) quan la conversa ja és llarga.
      const show = visible && wide && prompts.length >= 3;
      rail.hidden = !show;
      // Perquè la conversa li pugui fer lloc (Vents no té marge a la dreta).
      if (scroller.classList.contains('pnav-on') !== show) scroller.classList.toggle('pnav-on', show);
      if (!show) { hideTip(); return; }
      const h = host.getBoundingClientRect(), s = scroller.getBoundingClientRect();
      const height = Math.max(40, scroller.clientHeight - 24), step = pitch(prompts.length, height - 12, 5, 10);
      // Centrada a l'alçada de la conversa; si no hi cap, de dalt a baix i fa scroll.
      const used = Math.min(height, prompts.length * step + 8);
      rail.style.top = (s.top - h.top + 12 + (height - used) / 2) + 'px';
      // Un pèl separada de la vora (i de la barra de desplaçament).
      rail.style.left = (s.left - h.left + scroller.clientWidth - 28) + 'px';
      rail.style.maxHeight = height + 'px';
      list.style.setProperty('--pnav-pitch', step + 'px');
    }

    function rebuild() {
      frame = 0;
      const got = collect();
      const sig = got.states.join(',') + '|' + got.els.length;
      const changed = prompts.length !== got.els.length || got.els.some((el, i) => el !== prompts[i]);
      prompts = got.els; states = got.states;
      if (sizes) {
        const next = new Set(scroller.children);
        for (const child of observed) if (!next.has(child)) sizes.unobserve(child);
        for (const child of next) if (!observed.has(child)) sizes.observe(child);
        observed = next;
      }
      if (changed || sig !== signature) {
        signature = sig;
        list.setAttribute('aria-label', T('pnav.label', 'Your prompts'));
        rail.setAttribute('aria-label', T('pnav.label', 'Your prompts'));
        const frag = document.createDocumentFragment();
        prompts.forEach((el, i) => {
          const b = document.createElement('button');
          b.type = 'button';
          b.className = 'pnav-item';
          b.dataset.state = states[i];
          b.dataset.i = i;
          b.tabIndex = -1;
          const text = clip(opts.text(el), 300);
          b.setAttribute('aria-label', T('pnav.item', 'Prompt {n} of {m}').replace('{n}', i + 1).replace('{m}', prompts.length) + ': ' + clip(text, 120) + ' — ' + stateLabel(states[i]));
          b.setAttribute('aria-describedby', tip.id);
          b.innerHTML = '<span class="pnav-line" aria-hidden="true"></span>';
          frag.appendChild(b);
        });
        list.replaceChildren(frag);
        if (focusIdx >= prompts.length) focusIdx = -1;
        if (focusIdx >= 0) list.children[focusIdx].tabIndex = 0;
      }
      place();
      geometry = null;
      updateActive();
    }
    function scheduleScroll() {
      if (!scrollFrame) scrollFrame = requestAnimationFrame(() => {
        scrollFrame = 0;
        if (!frame) updateActive();
      });
    }
    function schedule() { if (!frame) frame = requestAnimationFrame(rebuild); }

    function updateActive() {
      if (rail.hidden || !prompts.length) return;
      const scrollTop = scroller.scrollTop;
      if (!geometry) {
        const base = scroller.getBoundingClientRect().top - scrollTop;
        geometry = {
          tops: prompts.map(el => el.getBoundingClientRect().top - base),
          height: scroller.clientHeight,
          end: scroller.scrollHeight,
        };
      }
      const { tops, height, end } = geometry;
      const k = activeIndex(tops, scrollTop, height, end - scrollTop - height < 4);
      const next = list.children[k];
      if (k === active && next?.classList.contains('on')) return;
      const previous = list.children[active];
      if (previous && previous !== next) {
        previous.classList.remove('on');
        previous.removeAttribute('aria-current');
        previous.tabIndex = previous === list.children[focusIdx] ? 0 : -1;
      }
      active = k;
      if (next) {
        next.classList.add('on');
        next.setAttribute('aria-current', 'location');
        next.tabIndex = focusIdx < 0 || focusIdx === k ? 0 : -1;
      }
      // Amb molts prompts la barra fa scroll: el que és a la vista, a dins.
      const b = list.children[k];
      if (b && rail.scrollHeight > rail.clientHeight) {
        const r = b.offsetTop - rail.scrollTop;
        if (r < 0 || r > rail.clientHeight - 8) rail.scrollTop = b.offsetTop - rail.clientHeight / 2;
      }
    }

    function showTip(b) {
      const i = +b.dataset.i, el = prompts[i];
      if (!el) return;
      tip.replaceChildren();
      const head = document.createElement('div');
      head.className = 'pnav-tip-state';
      head.dataset.state = states[i];
      head.textContent = (i + 1) + ' · ' + stateLabel(states[i]);
      const body = document.createElement('div');
      body.className = 'pnav-tip-text';
      body.textContent = clip(opts.text(el), 180) || '…';
      tip.append(head, body);
      tip.hidden = false;
      const h = host.getBoundingClientRect(), r = b.getBoundingClientRect();
      const tw = tip.offsetWidth, th = tip.offsetHeight;
      tip.style.left = Math.max(8, r.left - h.left - tw - 10) + 'px';
      tip.style.top = Math.max(4, Math.min(host.clientHeight - th - 4, r.top - h.top + r.height / 2 - th / 2)) + 'px';
    }
    function hideTip() { tip.hidden = true; }

    function go(i) {
      const el = prompts[i];
      if (!el) return;
      const top = el.getBoundingClientRect().top - scroller.getBoundingClientRect().top + scroller.scrollTop - 16;
      scroller.scrollTo({ top: Math.max(0, top), behavior: root.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'instant' : 'smooth' });
      el.classList.remove('pnav-flash'); void el.offsetWidth; el.classList.add('pnav-flash');
      setTimeout(() => el.classList.remove('pnav-flash'), 1300);
    }
    function focusItem(i) {
      const n = list.children.length;
      if (!n) return;
      i = Math.max(0, Math.min(n - 1, i));
      focusIdx = i;
      [...list.children].forEach((b, j) => { b.tabIndex = j === i ? 0 : -1; });
      list.children[i].focus();
    }

    list.addEventListener('click', e => { const b = e.target.closest('.pnav-item'); if (b) go(+b.dataset.i); });
    list.addEventListener('mouseover', e => { const b = e.target.closest('.pnav-item'); if (b) showTip(b); });
    rail.addEventListener('mouseleave', () => { if (!rail.contains(document.activeElement)) hideTip(); });
    list.addEventListener('focusin', e => {
      const b = e.target.closest('.pnav-item');
      if (!b) return;
      const prior = list.children[focusIdx < 0 ? active : focusIdx];
      if (prior && prior !== b) prior.tabIndex = -1;
      b.tabIndex = 0;
      focusIdx = +b.dataset.i;
      showTip(b);
    });
    rail.addEventListener('focusout', e => { if (!rail.contains(e.relatedTarget)) { if (list.children[focusIdx]) list.children[focusIdx].tabIndex = -1; focusIdx = -1; if (list.children[active]) list.children[active].tabIndex = 0; hideTip(); } });
    list.addEventListener('keydown', e => {
      const b = e.target.closest('.pnav-item'); if (!b) return;
      const i = +b.dataset.i, n = list.children.length;
      const to = { ArrowUp: i - 1, ArrowLeft: i - 1, ArrowDown: i + 1, ArrowRight: i + 1, Home: 0, End: n - 1, PageUp: i - 5, PageDown: i + 5 }[e.key];
      if (to === undefined) { if (e.key === 'Escape') { hideTip(); b.blur(); } return; }
      e.preventDefault();
      focusItem(to);
      go(Math.max(0, Math.min(n - 1, to)));
    });

    scroller.addEventListener('scroll', scheduleScroll, { passive: true });
    const children = new MutationObserver(schedule);
    children.observe(scroller, { childList: true });
    const classes = new MutationObserver(records => {
      if (records.some(record => record.target.parentElement === scroller)) schedule();
    });
    classes.observe(scroller, { subtree: true, attributes: true, attributeFilter: ['class'] });
    const viewport = root.ResizeObserver ? new ResizeObserver(schedule) : null;
    viewport?.observe(scroller);
    root.addEventListener('resize', schedule);
    const onLanguage = () => { signature = ''; schedule(); };
    document.addEventListener('gregal:idioma', onLanguage);
    schedule();
    const api = { refresh: schedule, rail, dispose() {
      cancelAnimationFrame(frame);
      cancelAnimationFrame(scrollFrame);
      children.disconnect();
      classes.disconnect();
      viewport?.disconnect();
      sizes?.disconnect();
      scroller.removeEventListener('scroll', scheduleScroll);
      root.removeEventListener('resize', schedule);
      document.removeEventListener('gregal:idioma', onLanguage);
      scroller.classList.remove('pnav-on');
      rail.remove();
      tip.remove();
      delete scroller.__promptNav;
    } };
    scroller.__promptNav = api;
    return api;
  }

  root.gregalPromptNav = { ...core, attach, attachVents };

  // Codi: #main, amb els prompts (.msg.u) i els grups de feina de cada torn.
  function attachCodi() {
    const main = document.getElementById('main'), pane = document.getElementById('conversationPane');
    if (!main || !pane) return;
    attach({
      scroller: main, host: pane,
      mark: el => {
        const c = el.classList;
        if (!c || !c.contains('msg')) return null;
        if (c.contains('u')) return c.contains('steer') ? null : 'u';
        if (c.contains('work-group')) return c.contains('running') ? 'run' : c.contains('failed') ? 'fail' : 'answer';
        if (c.contains('a')) return c.contains('answering') ? 'run' : c.contains('failed') ? 'fail' : 'answer';
        if (c.contains('sys')) return c.contains('stopped') ? 'stop' : c.contains('err') ? 'fail' : null;
        return null;
      },
      text: el => (el.querySelector('.body') || el).textContent,
      busy: () => !!(root.gregal && root.gregal.isBusy && root.gregal.isBusy()),
    });
  }
  let vents;
  function attachVents(scroller) {
    if (scroller === vents) return;
    vents?.__promptNav?.dispose();
    vents = scroller;
    const host = scroller?.parentElement;
    if (!host) return;
    if (getComputedStyle(host).position === 'static') host.style.position = 'relative';
    attach({
      scroller, host, minWidth: 560,
      mark: el => {
        const c = el.classList;
        if (!c || !c.contains('vw-message')) return null;
        if (c.contains('vw-user')) return 'u';
        return c.contains('vw-pending') ? 'run' : 'answer';
      },
      text: el => (el.querySelector('.vw-message-text') || el).textContent,
    });
  }
  function boot() {
    if (document.documentElement.dataset.product !== 'vents') attachCodi();
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot); else boot();
})(typeof window !== 'undefined' ? window : globalThis);
