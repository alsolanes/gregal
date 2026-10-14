// Repetició (timelapse) d'un torn acabat a la sala d'agents: el mateix
// dibuix del directe (workshop-scene.js) alimentat amb els estats que
// replay-core.js reconstrueix del registre durable. Reproduir a 1×/4×/10×,
// moure's amb la barra i baixar-ne un clip WebM (canvas + MediaRecorder).
import './replay-core.js';
import { createWorkshopScene } from './workshop-scene.js';

const core = window.gregalReplayCore;
const G = () => window.gregal;
// Dates en l'idioma de la interfície, no en el del navegador.
const uiLocale = () => document.documentElement.lang || undefined;
const T = (key, vars) => {
  let s = (window.gregalT && window.gregalT(key)) || key;
  for (const k in vars || {}) s = s.replace('{' + k + '}', vars[k]);
  return s;
};
const SPEEDS = [1, 4, 10];
const SCENE_KEYS = ['thinking', 'tools', 'reviewing', 'compacting', 'working', 'completed', 'failed', 'cancelled', 'waiting', 'idle'];

const st = {
  dialog: null, scene: null, canvas: null, runs: [], runId: 0, timeline: null,
  pos: 0, speed: 4, playing: false, raf: 0, lastTick: 0, index: -1, recorder: null, title: '',
};

function el(tag, cls, text) {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text != null) n.textContent = text;
  return n;
}
function button(cls, text, label, onClick) {
  const b = el('button', cls, text);
  b.type = 'button';
  if (label) { b.setAttribute('aria-label', label); b.title = label; }
  if (onClick) b.addEventListener('click', onClick);
  return b;
}
async function getJSON(url) {
  const r = await G().api(url);
  if (!r.ok) throw new Error((await r.text()).trim() || 'HTTP ' + r.status);
  return r.json();
}
function labels() {
  const out = {};
  for (const k of SCENE_KEYS) out[k] = T('workshop.' + k);
  out.step = T('replay.toolsShort');
  out.delivered = T('workshop.deliveredN');
  return out;
}
function stateText(frame) {
  if (!frame) return '';
  const parts = [T('workshop.' + (SCENE_KEYS.includes(frame.state) ? frame.state : 'idle'))];
  if (frame.tool) parts.push(frame.tool);
  const working = frame.agents.filter(a => a.status === 'working').length;
  if (frame.agents.length) parts.push(T('replay.agents', { n: frame.agents.length, w: working }));
  return parts.join(' · ');
}

function ensureDialog() {
  if (st.dialog) return st.dialog;
  const d = el('dialog', 'replay-dialog');
  d.setAttribute('aria-labelledby', 'replayTitle');
  const head = el('header', 'replay-head');
  const h = el('h2', '', T('replay.title')); h.id = 'replayTitle';
  const pick = el('select', 'replay-run'); pick.setAttribute('aria-label', T('replay.pickRun'));
  pick.addEventListener('change', () => load(Number(pick.value)));
  head.append(h, pick, button('replay-icon', '✕', T('replay.close'), () => close()));
  const stage = el('div', 'replay-stage');
  st.canvas = el('canvas', 'replay-scene'); st.canvas.setAttribute('aria-hidden', 'true');
  const caption = el('p', 'replay-caption'); caption.setAttribute('aria-live', 'polite');
  const empty = el('div', 'replay-empty'); empty.hidden = true;
  stage.append(st.canvas, caption, empty);
  const bar = el('div', 'replay-controls');
  const play = button('replay-play primary', '▶', T('replay.play'), () => (st.playing ? pause() : play_()));
  const scrub = el('input', 'replay-scrub'); scrub.type = 'range'; scrub.min = '0'; scrub.step = '1'; scrub.value = '0';
  scrub.setAttribute('aria-label', T('replay.scrub'));
  scrub.addEventListener('input', () => { const to = Number(scrub.value); pause(); seek(to); });
  const speeds = el('div', 'replay-speeds'); speeds.setAttribute('role', 'radiogroup'); speeds.setAttribute('aria-label', T('replay.speed'));
  SPEEDS.forEach(s => {
    const b = button('replay-speed', s + '×', T('replay.speedN', { n: s }), () => { st.speed = s; paintControls(); });
    b.setAttribute('role', 'radio'); b.dataset.speed = String(s);
    speeds.appendChild(b);
  });
  const time = el('span', 'replay-time');
  const clip = button('replay-clip', T('replay.clip'), T('replay.clipHelp'), () => recordClip());
  const status = el('span', 'replay-status'); status.setAttribute('role', 'status');
  bar.append(play, scrub, speeds, time, clip, status);
  d.append(head, stage, bar);
  d.addEventListener('close', () => { pause(); stopRecording(true); st.scene && st.scene.hide(); });
  d.addEventListener('keydown', e => {
    if (e.target && /^(INPUT|SELECT|TEXTAREA)$/.test(e.target.tagName) && e.target.type !== 'range') return;
    if (e.key === ' ' || e.key === 'k') { e.preventDefault(); st.playing ? pause() : play_(); }
  });
  (document.getElementById('retroMount') || document.body).appendChild(d);
  st.dialog = d;
  st.scene = createWorkshopScene(st.canvas, null);
  return d;
}

function q(sel) { return st.dialog.querySelector(sel); }
function setStatus(text) { const s = q('.replay-status'); if (s) s.textContent = text || ''; }

function paintControls() {
  if (!st.dialog) return;
  const tl = st.timeline;
  const dur = tl ? tl.duration : 0;
  const play = q('.replay-play');
  play.textContent = st.playing ? '❚❚' : '▶';
  play.setAttribute('aria-label', T(st.playing ? 'replay.pause' : 'replay.play'));
  play.title = play.getAttribute('aria-label');
  play.disabled = !tl || !tl.frames.length || !!st.recorder;
  const scrub = q('.replay-scrub');
  scrub.max = String(Math.max(1, Math.round(dur)));
  scrub.value = String(Math.round(st.pos));
  scrub.disabled = !tl || !tl.frames.length || !!st.recorder;
  scrub.setAttribute('aria-valuetext', core.fmtClock(st.pos) + ' / ' + core.fmtClock(dur));
  q('.replay-time').textContent = core.fmtClock(st.pos) + ' / ' + core.fmtClock(dur);
  st.dialog.querySelectorAll('.replay-speed').forEach(b => {
    const on = Number(b.dataset.speed) === st.speed;
    b.classList.toggle('on', on); b.setAttribute('aria-checked', String(on)); b.disabled = !!st.recorder;
  });
  const clip = q('.replay-clip');
  clip.hidden = !canRecord();
  clip.disabled = !tl || !tl.frames.length;
  clip.textContent = st.recorder ? T('replay.clipStop') : T('replay.clip');
}

function canRecord() {
  return !!(st.canvas && typeof st.canvas.captureStream === 'function' && typeof MediaRecorder === 'function');
}

function seek(pos) {
  const tl = st.timeline;
  if (!tl) return;
  st.pos = Math.max(0, Math.min(tl.duration, pos));
  const i = core.frameIndexAt(tl, st.pos);
  if (i !== st.index) {
    st.index = i;
    const frame = tl.frames[i];
    st.scene.update(core.toSession(frame, { id: 'replay-' + st.runId, title: st.title }), { labels: labels(), T });
    q('.replay-caption').textContent = stateText(frame);
  }
  paintControls();
}

function tick(now) {
  st.raf = 0;
  if (!st.playing || !st.timeline) return;
  const dt = st.lastTick ? now - st.lastTick : 0;
  st.lastTick = now;
  seek(core.advance(st.pos, dt, st.speed, st.timeline.duration));
  if (st.recorder) drawComposite();
  if (st.pos >= st.timeline.duration) {
    // Un moment al final perquè el clip no acabi de cop.
    if (st.recorder) { setTimeout(() => stopRecording(false), 700); }
    pause();
    return;
  }
  st.raf = requestAnimationFrame(tick);
}
function play_() {
  if (!st.timeline || !st.timeline.frames.length) return;
  if (st.pos >= st.timeline.duration) seek(0);
  st.playing = true; st.lastTick = 0;
  if (!st.raf) st.raf = requestAnimationFrame(tick);
  paintControls();
}
function pause() {
  st.playing = false;
  if (st.raf) cancelAnimationFrame(st.raf);
  st.raf = 0;
  if (st.dialog) paintControls();
}

// --- clip WebM ---

let comp = null, compCtx = null;
function cssVar(name, fallback) {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v || fallback;
}
function drawComposite() {
  if (!compCtx) return;
  const W = comp.width, H = comp.height, band = 44;
  compCtx.fillStyle = cssVar('--bg', 'black');
  compCtx.fillRect(0, 0, W, H);
  try { compCtx.drawImage(st.canvas, 0, 0, W, H - band); } catch (e) {}
  compCtx.fillStyle = cssVar('--surface', 'black');
  compCtx.fillRect(0, H - band, W, band);
  const frame = st.timeline && st.timeline.frames[st.index];
  compCtx.fillStyle = cssVar('--ink', 'white');
  compCtx.font = '600 16px system-ui, sans-serif';
  compCtx.textBaseline = 'middle';
  compCtx.fillText('Gregal · ' + (st.title || T('replay.runN', { n: st.runId })), 14, H - band / 2);
  const right = core.fmtClock(st.pos) + ' · ' + st.speed + '× · ' + stateText(frame);
  compCtx.font = '500 14px system-ui, sans-serif';
  compCtx.fillStyle = cssVar('--muted', 'gray');
  const w = compCtx.measureText(right).width;
  compCtx.fillText(right, Math.max(14, W - w - 14), H - band / 2);
  // Barra de progrés fina a la vora de la franja.
  compCtx.fillStyle = cssVar('--accent', 'teal');
  compCtx.fillRect(0, H - band, W * (st.timeline.duration ? st.pos / st.timeline.duration : 0), 3);
}

function recordClip() {
  if (st.recorder) { stopRecording(false); return; }
  if (!canRecord() || !st.timeline) { setStatus(T('replay.clipUnsupported')); return; }
  const mime = ['video/webm;codecs=vp9', 'video/webm;codecs=vp8', 'video/webm'].find(m => MediaRecorder.isTypeSupported && MediaRecorder.isTypeSupported(m));
  if (!mime) { setStatus(T('replay.clipUnsupported')); return; }
  comp = document.createElement('canvas');
  comp.width = 960; comp.height = 444;
  compCtx = comp.getContext('2d');
  const chunks = [];
  let rec;
  try { rec = new MediaRecorder(comp.captureStream(30), { mimeType: mime, videoBitsPerSecond: 2500000 }); }
  catch (e) { setStatus(T('replay.clipUnsupported')); return; }
  rec.ondataavailable = e => { if (e.data && e.data.size) chunks.push(e.data); };
  rec.onstop = () => {
    const discard = rec._discard;
    st.recorder = null; comp = null; compCtx = null;
    paintControls();
    if (discard || !chunks.length) { setStatus(''); return; }
    const url = URL.createObjectURL(new Blob(chunks, { type: 'video/webm' }));
    const a = el('a'); a.href = url; a.download = 'gregal-replay-run-' + st.runId + '.webm';
    document.body.appendChild(a); a.click(); a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 4000);
    setStatus(T('replay.clipDone'));
  };
  st.recorder = rec;
  seek(0); st.index = -1; seek(0);
  drawComposite();
  rec.start(250);
  setStatus(T('replay.clipRecording'));
  play_();
}
function stopRecording(discard) {
  const rec = st.recorder;
  if (!rec) return;
  rec._discard = !!discard;
  try { if (rec.state !== 'inactive') rec.stop(); } catch (e) { st.recorder = null; }
  pause();
}

// --- càrrega ---

function runLabel(r) {
  const when = r.started_at ? new Date(r.started_at) : null;
  const time = when && !isNaN(when) ? when.toLocaleString(uiLocale(), { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }) : '';
  return T('replay.runN', { n: r.run_id }) + (time ? ' · ' + time : '') + ' · ' + T('workshop.' + (SCENE_KEYS.includes(r.state) ? r.state : 'idle'));
}

function showEmpty(text) {
  const e = q('.replay-empty');
  e.textContent = text || '';
  e.hidden = !text;
  st.canvas.hidden = !!text;
}

async function load(runId) {
  pause(); stopRecording(true);
  st.runId = runId; st.timeline = null; st.index = -1; st.pos = 0;
  setStatus(T('replay.loading'));
  paintControls();
  try {
    const data = await getJSON('/api/replay?run_id=' + encodeURIComponent(runId));
    if (st.runId !== runId) return;
    st.timeline = core.buildTimeline(data.events || []);
    const run = st.runs.find(r => r.run_id === runId);
    st.title = run && run.title ? run.title : T('replay.runN', { n: runId });
    showEmpty(st.timeline.frames.length ? '' : T('replay.noFrames'));
    const real = core.fmtClock(st.timeline.realMs);
    setStatus(T('replay.summary', { t: real, n: st.timeline.tools, a: st.timeline.agents }) + (data.truncated ? ' · ' + T('replay.truncated') : ''));
    st.scene.show();
    seek(0);
  } catch (e) {
    showEmpty(T('replay.failed') + ' ' + e.message);
    setStatus('');
  }
}

export async function open(opts) {
  const o = opts || {};
  const d = ensureDialog();
  if (!d.open) d.showModal();
  showEmpty('');
  setStatus(T('replay.loading'));
  try {
    const data = await getJSON('/api/replay');
    st.runs = Array.isArray(data.runs) ? data.runs : [];
  } catch (e) { st.runs = []; showEmpty(T('replay.failed') + ' ' + e.message); setStatus(''); return; }
  const finished = st.runs.filter(r => r.state !== 'running');
  const pick = q('.replay-run');
  pick.replaceChildren(...finished.map(r => { const op = el('option', '', runLabel(r)); op.value = String(r.run_id); return op; }));
  pick.hidden = finished.length < 2;
  if (!finished.length) { showEmpty(T('replay.none')); setStatus(''); paintControls(); return; }
  let target = o.runId ? finished.find(r => r.run_id === Number(o.runId)) : null;
  if (!target && o.groupIndexFromEnd != null) target = core.runForGroup(st.runs, o.groupIndexFromEnd);
  if (!target) target = finished[0];
  pick.value = String(target.run_id);
  await load(target.run_id);
  if (o.autoplay !== false) play_();
}

export function close() {
  if (st.dialog && st.dialog.open) st.dialog.close();
}

// --- punts d'entrada: pestanya En directe i grups de feina acabats ---

function replayButton(cls) {
  const b = button(cls, '', T('replay.open'));
  b.append(el('span', 'replay-open-icon', '▶'), el('span', 'replay-open-label', T('replay.button')));
  b.dataset.replayOpen = '1';
  return b;
}

function decorate() {
  const header = document.querySelector('#workshopRoot .workshop-header');
  if (header && !header.querySelector('[data-replay-open]')) {
    const b = replayButton('ghost replay-open replay-live');
    b.addEventListener('click', e => { e.preventDefault(); open({}); });
    const find = header.querySelector('[data-workshop-find]');
    header.insertBefore(b, find || null);
  }
  document.querySelectorAll('#main .work-group:not(.running) > summary').forEach(s => {
    if (s.querySelector('[data-replay-open]')) return;
    const b = replayButton('replay-open replay-group');
    b.addEventListener('click', e => {
      e.preventDefault(); e.stopPropagation();
      const groups = [...document.querySelectorAll('#main .work-group:not(.running)')];
      const idx = groups.length - 1 - groups.indexOf(s.parentElement);
      open({ groupIndexFromEnd: Math.max(0, idx) });
    });
    s.appendChild(b);
  });
}

function relabel() {
  document.querySelectorAll('[data-replay-open]').forEach(b => {
    b.title = T('replay.open'); b.setAttribute('aria-label', T('replay.open'));
    const l = b.querySelector('.replay-open-label'); if (l) l.textContent = T('replay.button');
  });
  if (st.dialog) {
    const wasOpen = st.dialog.open, run = st.runId;
    pause(); stopRecording(true);
    if (st.scene) st.scene.destroy();
    st.dialog.remove(); st.dialog = null; st.scene = null;
    if (wasOpen) open({ runId: run, autoplay: false });
  }
}

export function boot() {
  decorate();
  const main = document.getElementById('main');
  if (main) {
    new MutationObserver(muts => {
      if (muts.some(m => (m.type === 'childList' && m.addedNodes.length) || (m.type === 'attributes' && m.target.classList && m.target.classList.contains('work-group')))) decorate();
    }).observe(main, { childList: true, subtree: true, attributes: true, attributeFilter: ['class'] });
  }
  document.addEventListener('gregal:idioma', relabel);
}

window.gregalReplay = { open, close };
