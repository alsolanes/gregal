// La barra de progrés dels partits (Arena, tele i pàgina de Partits): una
// línia de temps amb el que s'ha vist i el que ha arribat, marques per als
// moments del prompt (★, analysis.moments) i les jugades grosses (hl / punts),
// un rètol en passar-hi el ratolí, clic o arrossegar per saltar, i els botons
// ⏮ des del principi, ⏪ enrere, ⏯, ⏩ endavant, ⏭ al final / al directe i la
// velocitat. Teclat: ←/→, espai, Inici, Fi. A la tele s'amaga sola i torna en
// moure el ratolí.
//
// El visor ha de tenir seek(index) i getProgress() → { index, total, live,
// events, marks, chapters, step } (football-view, arena-cs2-view, arena-lol-view).

const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const clamp = (v, a, b) => Math.max(a, Math.min(b, v));

// Els rètols, en anglès (els jocs van en anglès); opts.t(clau) els pot canviar.
export const LABELS = {
  'scrub.restart': 'Restart', 'scrub.back': 'Back 15 s', 'scrub.play': 'Play', 'scrub.pause': 'Pause', 'scrub.forward': 'Forward 15 s',
  'scrub.end': 'Skip to the end', 'scrub.live': 'Back to live', 'scrub.speed': 'Speed', 'scrub.liveTag': 'LIVE',
  'scrub.moment': 'Prompt moment', 'scrub.big': 'Big play', 'scrub.scored': 'Score', 'scrub.of': '{i} of {n}', 'scrub.seek': 'Match progress',
};

// mm:ss (o h:mm:ss) d'uns segons de reproducció.
export function fmtTime(s) {
  s = Math.max(0, Math.floor(+s || 0));
  const h = Math.floor(s / 3600), m = Math.floor(s / 60) % 60, ss = String(s % 60).padStart(2, '0');
  return h ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`;
}
// L'esdeveniment que es veu al segon t: l'últim que comença abans (o a) t.
export function indexAtTime(times, t) {
  let lo = 0, hi = times.length - 2;
  if (hi < 0) return 0;
  while (lo < hi) { const mid = (lo + hi + 1) >> 1; if (times[mid] <= t) lo = mid; else hi = mid - 1; }
  return lo;
}

// Cap on va ⏪ / ⏩: uns segons enrere o endavant (com un vídeo) si hi ha temps
// de reproducció; si no, per capítols (les rondes del CS2) o uns quants esdeveniments.
// cur = l'esdeveniment que es veu ara (index - 1).
const timedPr = pr => Array.isArray(pr.times) && pr.times.length === pr.total + 1 && pr.duration > 0;
export const JUMP_SECS = 15;
export function backTarget(pr, secs = JUMP_SECS) {
  const cur = Math.max(0, (pr.index || 0) - 1);
  if (timedPr(pr)) return Math.min(cur, indexAtTime(pr.times, (pr.elapsed || 0) - secs));
  const ch = (pr.chapters || []).filter(c => c <= cur);
  if (ch.length) {
    let t = ch.pop();
    if (cur - t <= 2 && ch.length) t = ch.pop();
    return t;
  }
  return Math.max(0, cur - (pr.step || 5));
}
export function forwardTarget(pr, secs = JUMP_SECS) {
  const cur = Math.max(0, (pr.index || 0) - 1);
  if (timedPr(pr)) {
    const t = (pr.elapsed || 0) + secs;
    if (t >= pr.duration) return pr.total;
    let i = indexAtTime(pr.times, t);
    if (pr.times[i] < t - 1e-6) i++; // the first event at or after t: at least that far ahead
    return Math.min(pr.total, Math.max(cur + 1, i));
  }
  const next = (pr.chapters || []).find(c => c > cur);
  if (pr.chapters && pr.chapters.length) return next != null ? next : pr.total;
  return Math.min(pr.total, cur + (pr.step || 5));
}

function css() {
  if (document.getElementById('arScrubCSS')) return;
  const st = document.createElement('style'); st.id = 'arScrubCSS';
  st.textContent = `
.ar-scrub { --sc:#ffd400; position:relative; display:flex; flex-direction:column; gap:6px; padding:8px 12px 8px; background:rgba(8,10,22,.9); color:#f3f5fb; border-radius:12px; font:13px/1.3 "Segoe UI", system-ui, sans-serif; user-select:none; -webkit-user-select:none; transition:opacity .3s, transform .3s; }
.ar-scrub.sc-overlay { position:absolute; left:10px; right:10px; bottom:10px; z-index:9; box-shadow:0 10px 40px rgba(0,0,0,.5); }
.ar-scrub.sc-overlay.sc-big { left:24px; right:24px; bottom:58px; font-size:15px; padding:10px 16px; }
.ar-scrub.sc-gone { opacity:0; transform:translateY(8px); pointer-events:none; }
.ar-scrub .sc-trk { position:relative; height:18px; cursor:pointer; touch-action:none; }
.ar-scrub.sc-ro .sc-trk { cursor:default; }
.ar-scrub .sc-rail { position:absolute; left:0; right:0; top:7px; height:5px; border-radius:99px; background:rgba(255,255,255,.14); overflow:hidden; transition:top .15s, height .15s; }
.ar-scrub .sc-trk:hover .sc-rail, .ar-scrub.sc-drag .sc-rail { top:6px; height:7px; }
.ar-scrub .sc-buf { position:absolute; left:0; top:0; bottom:0; background:rgba(255,255,255,.22); }
.ar-scrub .sc-done { position:absolute; left:0; top:0; bottom:0; background:var(--sc); }
.ar-scrub .sc-ghost { position:absolute; top:0; bottom:0; width:2px; background:#fff; opacity:0; }
.ar-scrub .sc-trk:hover .sc-ghost, .ar-scrub.sc-drag .sc-ghost { opacity:.75; }
.ar-scrub .sc-knob { position:absolute; top:4px; width:11px; height:11px; margin-left:-5.5px; border-radius:50%; background:#fff; box-shadow:0 0 0 2px var(--sc); pointer-events:none; }
.ar-scrub .sc-mk { position:absolute; pointer-events:none; transform:translateX(-50%); }
.ar-scrub .sc-mk.sc-hl { top:8px; width:3px; height:3px; border-radius:50%; background:#fff; opacity:.85; }
.ar-scrub .sc-mk.sc-score { top:5px; width:2px; height:9px; background:#9be7ff; opacity:.9; }
.ar-scrub .sc-mk.sc-moment { top:-9px; font-size:12px; line-height:1; color:var(--sc); text-shadow:0 1px 3px #000; }
.ar-scrub.sc-big .sc-mk.sc-moment { font-size:15px; top:-12px; }
.ar-scrub .sc-tip { position:absolute; bottom:24px; transform:translateX(-50%); max-width:min(420px, 80vw); width:max-content; background:rgba(4,6,14,.97); border:1px solid rgba(255,255,255,.16); border-radius:8px; padding:6px 10px; font-size:12.5px; pointer-events:none; opacity:0; transition:opacity .12s; z-index:2; }
.ar-scrub .sc-tip.sc-on { opacity:1; }
.ar-scrub .sc-tip b { color:var(--sc); } .ar-scrub .sc-tip small { display:block; color:#9aa4bd; }
.ar-scrub .sc-tip .sc-mo { display:block; margin-top:4px; color:#f3f5fb; } .ar-scrub .sc-tip .sc-mo b { color:var(--mc, #ffd400); }
.ar-scrub .sc-row { display:flex; align-items:center; gap:4px; flex-wrap:wrap; }
.ar-scrub button { font:inherit; min-width:32px; height:28px; padding:0 8px; border:1px solid rgba(255,255,255,.14); background:rgba(255,255,255,.07); color:#f3f5fb; border-radius:7px; cursor:pointer; }
.ar-scrub button:hover { background:rgba(255,255,255,.16); } .ar-scrub button:focus-visible { outline:2px solid var(--sc); outline-offset:1px; }
.ar-scrub button.sc-sp { min-width:44px; font-weight:700; }
.ar-scrub.sc-big button { height:34px; min-width:40px; }
.ar-scrub .sc-pos { margin-left:8px; color:#c9cfe0; font-variant-numeric:tabular-nums; white-space:nowrap; }
.ar-scrub .sc-lv { padding:2px 7px; border-radius:5px; background:#e5484d; color:#fff; font-weight:800; font-size:11px; letter-spacing:.1em; }
.ar-scrub .sc-grow { flex:1; }
.ar-scrub.sc-ro .sc-seek { display:none; }`;
  document.head.append(st);
}

// mountScrubber(host, opts) → { el, setView, sync, seek, show, destroy }.
// opts: view, t(key), moments() → Map(n → moment), teamName(side), sideColor(side),
//   speeds [1,2,4], speed, onSpeed(x), onPause(paused), onSeek(index, progress),
//   overlay (bar over the stage), big, autoHide (ms of quiet before hiding; 0 = never),
//   seekable (false: only pause/speed), keys (listen to the keyboard), extra (element appended to the row).
export function mountScrubber(host, opts = {}) {
  css();
  const L = k => (opts.t && opts.t(k)) || LABELS[k] || k;
  const speeds = opts.speeds || [1, 2, 4];
  let view = opts.view || null, speed = opts.speed || speeds[0], seekable = opts.seekable !== false;
  const el = document.createElement('div');
  el.className = 'ar-scrub' + (opts.overlay ? ' sc-overlay' : '') + (opts.big ? ' sc-big' : '') + (seekable ? '' : ' sc-ro');
  el.innerHTML = `<div class="sc-trk" role="slider" tabindex="-1" aria-label="${esc(L('scrub.seek'))}"><div class="sc-rail"><div class="sc-buf"></div><div class="sc-done"></div><div class="sc-ghost"></div></div><div class="sc-mks"></div><div class="sc-knob"></div><div class="sc-tip"></div></div>
    <div class="sc-row"><button type="button" class="sc-seek" data-a="restart" title="${esc(L('scrub.restart'))} (Home)">⏮</button><button type="button" class="sc-seek" data-a="back" title="${esc(L('scrub.back'))} (←)">⏪</button><button type="button" data-a="play"></button><button type="button" class="sc-seek" data-a="fwd" title="${esc(L('scrub.forward'))} (→)">⏩</button><button type="button" class="sc-seek" data-a="end"></button><button type="button" class="sc-sp" data-a="speed" title="${esc(L('scrub.speed'))}"></button>
    <span class="sc-pos"></span><span class="sc-lv" hidden>${esc(L('scrub.liveTag'))}</span><span class="sc-grow"></span></div>`;
  if (opts.extra) el.querySelector('.sc-row').append(opts.extra);
  host.append(el);
  const q = s => el.querySelector(s);
  const trk = q('.sc-trk'), tip = q('.sc-tip'), btn = a => q(`[data-a="${a}"]`);
  let pr = { index: 0, total: 0, live: false, events: [], marks: [], chapters: [] };
  let marksKey = '', nIdx = { len: -1, map: new Map() }, drag = null, hover = -1;

  const progress = () => { try { return view && view.getProgress ? view.getProgress() : null; } catch (e) { return null; } };
  // La línia va en temps de reproducció (pr.times, segons a 1×) si el visor el dona; si no, per esdeveniments.
  const timed = () => Array.isArray(pr.times) && pr.times.length === pr.total + 1 && pr.duration > 0;
  const frac = i => !pr.total ? 0 : timed() ? clamp(pr.times[clamp(Math.floor(i), 0, pr.total)] / pr.duration, 0, 1) : clamp(i / pr.total, 0, 1);
  const fracNow = () => timed() ? clamp((pr.elapsed || 0) / pr.duration, 0, 1) : frac(pr.index);
  const idxOfN = n => {
    const ev = pr.events || [];
    if (nIdx.len !== ev.length) { nIdx = { len: ev.length, map: new Map(ev.map((e, i) => [e.n, i])) }; }
    return nIdx.map.get(n);
  };
  const moments = () => { try { const m = opts.moments && opts.moments(); return m instanceof Map ? m : new Map(); } catch (e) { return new Map(); } };

  function paintMarks() {
    const mo = moments();
    const key = pr.total + '|' + pr.marks.length + '|' + [...mo.keys()].join(',');
    if (key === marksKey) return;
    marksKey = key;
    let h = '';
    for (const m of pr.marks) h += `<i class="sc-mk sc-${m.kind}" style="left:${(frac(m.i + .5) * 100).toFixed(3)}%"></i>`;
    for (const [n, m] of mo) {
      const i = idxOfN(n);
      if (i == null) continue;
      const col = opts.sideColor ? opts.sideColor(m.side) : '';
      h += `<i class="sc-mk sc-moment" style="left:${(frac(i + .5) * 100).toFixed(3)}%${col ? ';color:' + col : ''}">★</i>`;
    }
    q('.sc-mks').innerHTML = h;
  }

  function sync() {
    const p = progress();
    if (p) pr = p;
    const f = drag ? frac(drag.i) : fracNow();
    q('.sc-done').style.width = (f * 100) + '%';
    q('.sc-buf').style.width = pr.total ? '100%' : '0';
    q('.sc-knob').style.left = (f * 100) + '%';
    q('.sc-pos').textContent = timed() ? `${fmtTime(drag ? pr.times[drag.i] : pr.elapsed)} / ${fmtTime(pr.duration)}`
      : L('scrub.of').replace('{i}', Math.min(pr.total, Math.max(0, pr.index))).replace('{n}', pr.total);
    q('.sc-lv').hidden = !pr.live;
    const paused = !!(view && view.paused);
    btn('play').textContent = paused ? '▶' : '⏸';
    btn('play').title = L(paused ? 'scrub.play' : 'scrub.pause') + ' (Space)';
    btn('end').textContent = '⏭';
    btn('end').title = L(pr.live ? 'scrub.live' : 'scrub.end') + ' (End)';
    if (opts.onSync) { try { opts.onSync(pr); } catch (e) { /* res */ } }
    btn('speed').textContent = speed + '×';
    trk.setAttribute('aria-valuemin', '0'); trk.setAttribute('aria-valuemax', String(pr.total)); trk.setAttribute('aria-valuenow', String(pr.index));
    paintMarks();
    if (hover >= 0) tipAt(hover);
  }

  function seek(i) {
    if (!view || !view.seek || !seekable) return;
    const p = progress() || pr;
    i = clamp(Math.round(i), 0, p.total);
    hideTip();
    if (view.seek(i) === false) return;
    if (opts.onSeek) { try { opts.onSeek(i, p); } catch (e) { /* res */ } }
    sync(); show();
  }
  function togglePause() {
    if (!view) return;
    // Al final d'un partit acabat, ▶ el torna a posar des del principi.
    if (view.paused === false && view.isFinished && view.isFinished() && seekable) { seek(0); return; }
    const paused = view.togglePause();
    if (opts.onPause) opts.onPause(paused);
    sync(); show();
  }
  function setSpeed(x) {
    speed = x;
    if (view) view.setSpeed(x);
    if (opts.onSpeed) opts.onSpeed(x);
    sync();
  }
  const act = {
    restart: () => seek(0),
    back: () => seek(backTarget(progress() || pr)),
    fwd: () => seek(forwardTarget(progress() || pr)),
    end: () => seek((progress() || pr).total),
    play: togglePause,
    speed: () => setSpeed(speeds[(speeds.indexOf(speed) + 1) % speeds.length]),
  };
  el.querySelectorAll('[data-a]').forEach(b => b.addEventListener('click', e => { e.stopPropagation(); act[b.dataset.a](); }));

  // ---- la línia: rètol, clic i arrossegar ----
  const idxAt = x => {
    const r = trk.getBoundingClientRect(), k = clamp((x - r.left) / Math.max(1, r.width), 0, 1);
    if (!timed()) return clamp(Math.floor(k * pr.total), 0, Math.max(0, pr.total - 1));
    return clamp(indexAtTime(pr.times, k * pr.duration), 0, Math.max(0, pr.total - 1));
  };
  function hideTip() { hover = -1; tip.classList.remove('sc-on'); }
  function tipAt(i) {
    const e = (pr.events || [])[i];
    if (!e) { hideTip(); return; }
    hover = i;
    const mo = moments().get(e.n);
    const mark = pr.marks.find(m => m.i === i);
    const tag = mo ? '★ ' + L('scrub.moment') : mark ? (mark.kind === 'score' ? L('scrub.scored') : L('scrub.big')) : '';
    const sc = Array.isArray(e.score) ? ` · ${e.score[0]}-${e.score[1]}` : '';
    const when = timed() ? fmtTime(pr.times[i]) + ' · ' : '';
    tip.innerHTML = `${tag ? `<b>${esc(tag)}</b> ` : ''}${esc(e.text || e.kind || '')}<small>${esc(when + L('scrub.of').replace('{i}', i + 1).replace('{n}', pr.total))}${esc(sc)}</small>` +
      (mo ? `<span class="sc-mo" style="--mc:${opts.sideColor ? opts.sideColor(mo.side) : '#ffd400'}"><b>${esc(opts.teamName ? opts.teamName(mo.side) : '')}</b> «${esc(mo.rule)}»</span>` : '');
    const r = trk.getBoundingClientRect(), x = frac(i + .5) * r.width;
    tip.style.left = clamp(x, Math.min(r.width / 2, 120), Math.max(r.width / 2, r.width - 120)) + 'px';
    q('.sc-ghost').style.left = x + 'px';
    tip.classList.add('sc-on');
  }
  trk.addEventListener('pointermove', e => {
    if (!pr.total) return;
    const i = idxAt(e.clientX);
    if (drag) { drag.i = i; sync(); }
    tipAt(i); show();
  });
  trk.addEventListener('pointerleave', () => { if (!drag) hideTip(); });
  trk.addEventListener('pointerdown', e => {
    if (!seekable || !pr.total || e.button > 0) return;
    e.preventDefault();
    drag = { i: idxAt(e.clientX) };
    el.classList.add('sc-drag');
    try { trk.setPointerCapture(e.pointerId); } catch (x) { /* res */ }
    sync(); tipAt(drag.i);
  });
  const endDrag = e => {
    if (!drag) return;
    const i = e && e.type === 'pointerup' ? idxAt(e.clientX) : drag.i;
    drag = null; el.classList.remove('sc-drag');
    if (e && e.type === 'pointerup') seek(i);
    else sync();
  };
  trk.addEventListener('pointerup', endDrag);
  trk.addEventListener('pointercancel', () => endDrag(null));

  // ---- teclat ----
  const editable = t => t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName));
  const onKey = e => {
    if (!opts.keys || e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey || editable(e.target)) return;
    if (!el.isConnected || !el.getClientRects().length) return; // amagada (una altra pàgina)
    const k = e.key;
    let a = k === 'ArrowLeft' ? 'back' : k === 'ArrowRight' ? 'fwd' : k === 'Home' ? 'restart' : k === 'End' ? 'end' : k === ' ' || k === 'Spacebar' ? 'play' : '';
    if (!a) return;
    if (a === 'play' && e.target && e.target.tagName === 'BUTTON') return; // l'espai ja prem el botó
    if (a !== 'play' && !seekable) return;
    e.preventDefault();
    act[a](); show();
  };
  document.addEventListener('keydown', onKey);

  // ---- s'amaga sola (tele) ----
  let hideT = 0;
  function show() {
    el.classList.remove('sc-gone');
    clearTimeout(hideT);
    if (!opts.autoHide) return;
    hideT = setTimeout(function hide() {
      if (drag || el.matches(':hover') || (view && view.paused)) { hideT = setTimeout(hide, 800); return; }
      el.classList.add('sc-gone'); hideTip();
    }, opts.autoHide);
  }
  const wake = () => show();
  if (opts.autoHide) { addEventListener('mousemove', wake); addEventListener('pointerdown', wake); addEventListener('touchstart', wake, { passive: true }); show(); }

  const timer = setInterval(sync, 250);
  sync();
  return {
    el, sync, seek, show,
    setView(v) { view = v; marksKey = ''; if (v) v.setSpeed && v.setSpeed(speed); sync(); },
    setSpeed(x) { speed = x; sync(); },
    get speed() { return speed; },
    setSeekable(on) { seekable = !!on; el.classList.toggle('sc-ro', !seekable); },
    hide() { el.classList.add('sc-gone'); },
    destroy() {
      clearInterval(timer); clearTimeout(hideT);
      document.removeEventListener('keydown', onKey);
      removeEventListener('mousemove', wake); removeEventListener('pointerdown', wake); removeEventListener('touchstart', wake);
      el.remove();
    },
  };
}
