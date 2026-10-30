// Els «prompt moments» d'un partit de l'Arena (analysis.moments, arena_moments.go):
// la targeta que surt quan es veu la jugada (a la tele i al visor de l'Arena) i
// el vídeo de resum que es descarrega: presentació → el partit accelerat entre
// moments i a velocitat normal a cada moment, amb la targeta → el resum.
//
// El vídeo es fa de dues maneres:
//  - Ràpida (WebCodecs): el visor es carrega en un iframe propi amb el rellotge
//    (performance.now) i requestAnimationFrame substituïts, i s'hi avança
//    fotograma a fotograma (onFrame(dt) del visor, sense rAF). Cada fotograma
//    es compon en un llenç (visor + rètols) i va a un VideoEncoder H.264; el
//    so (efectes sintetitzats i les veus) es mescla de cop amb un
//    OfflineAudioContext i va a un AudioEncoder AAC (o Opus); tot plegat es
//    munta en un MP4 amb mp4-muxer (vendor/, MIT). No depèn del temps real ni
//    de la pestanya visible: va diverses vegades més de pressa que el partit.
//  - En temps real (MediaRecorder): el camí d'abans, si el navegador no té
//    WebCodecs o no sap codificar H.264. Cal tenir la finestra a la vista.
// Les peces pures (Director, slowStarts, placeAudio, frameStamp, etaSecs) es
// proven a arena-export.test.cjs.
import { fmtTime } from './arena-scrubber.js';

export const GAME = { nfl: 'NFL', cs2: 'Counter-Strike 2', lol: 'League of Legends' };
export const SIDE_COLOR = ['#3e7bfa', '#e5484d'];
const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const sleep = ms => new Promise(r => setTimeout(r, ms));

// moments: event n → moment.
export function momentsByEvent(m) {
  const out = new Map();
  for (const mo of ((m && m.analysis && m.analysis.moments) || [])) if (mo.event) out.set(mo.event, mo);
  return out;
}
// Les decisions marcades (n:side).
export function momentKeys(m) {
  return new Set(((m && m.analysis && m.analysis.moments) || []).map(mo => mo.n + ':' + mo.side));
}
// Al LoL el blau va a l'esquerra: el color de cada equip segueix el mapa.
export function sideColor(m, side) {
  const flip = m && m.game === 'lol' && m.meta && m.meta.blue === 1;
  return SIDE_COLOR[flip ? 1 - side : side];
}

export function momentCSS() { css(); }
function css() {
  if (document.getElementById('arMomentCSS')) return;
  const st = document.createElement('style'); st.id = 'arMomentCSS';
  st.textContent = `
.ar-moment-card { position:absolute; left:50%; top:14px; transform:translate(-50%, -12px); width:min(92%, 860px); z-index:8; pointer-events:none; opacity:0; transition:opacity .35s, transform .35s;
  background:rgba(8,10,22,.94); color:#f3f5fb; border:1px solid rgba(255,255,255,.14); border-left:8px solid var(--mc, #ffd400); border-radius:14px; padding:12px 18px; box-shadow:0 18px 50px rgba(0,0,0,.55); font:15px/1.4 "Segoe UI", system-ui, sans-serif; text-align:left; }
.ar-dec.ar-star { box-shadow:inset 0 0 0 1px #ffd400; } .ar-dec.ar-star b span { color:#ffd400; }
.ar-moment-card.on { opacity:1; transform:translate(-50%, 0); }
.ar-moment-card .k { color:#ffd400; font-weight:900; letter-spacing:.14em; font-size:12px; text-transform:uppercase; }
.ar-moment-card .k span { color:var(--mc); margin-left:8px; letter-spacing:.06em; }
.ar-moment-card .r { font-family:ui-monospace, Consolas, monospace; background:rgba(255,255,255,.07); border-radius:8px; padding:6px 10px; margin:6px 0; font-size:14px; }
.ar-moment-card .w b { color:#ffd400; }
.ar-stage .ar-moment-card { top:8px; padding:8px 12px; font-size:13px; border-left-width:6px; } .ar-stage .ar-moment-card .r { font-size:12px; padding:4px 8px; margin:4px 0; }
.ar-moment-card.big { font-size:19px; padding:16px 22px; } .ar-moment-card.big .k { font-size:15px; } .ar-moment-card.big .r { font-size:18px; }
.ar-export { position:fixed; inset:0; z-index:3000; background:rgba(4,6,14,.86); display:flex; align-items:center; justify-content:center; color:#f3f5fb; font:15px/1.45 "Segoe UI", system-ui, sans-serif; }
.ar-export .box { background:#0d1222; border:1px solid rgba(255,255,255,.14); border-radius:16px; padding:20px 22px; width:min(720px, 94vw); display:flex; flex-direction:column; gap:12px; box-shadow:0 30px 90px rgba(0,0,0,.6); }
.ar-export h3 { margin:0; font-size:18px; } .ar-export canvas { width:100%; aspect-ratio:16/9; border-radius:10px; background:#000; }
.ar-export .bar { height:8px; border-radius:99px; background:rgba(255,255,255,.1); overflow:hidden; } .ar-export .bar i { display:block; height:100%; width:0; background:#ffd400; transition:width .3s; }
.ar-export .row { display:flex; gap:10px; align-items:center; } .ar-export .row span { flex:1; color:#9aa4bd; font-variant-numeric:tabular-nums; }
.ar-export button { font:inherit; border:1px solid rgba(255,255,255,.18); background:#1a2138; color:#f3f5fb; border-radius:8px; padding:6px 14px; cursor:pointer; }
/* El visor que es grava en temps real: a la pantalla (els visors s'aturen fora de la vista) però invisible. */
.ar-export .stage { position:fixed; left:0; top:0; width:1280px; height:720px; opacity:0; pointer-events:none; z-index:-1; } .ar-export .stage canvas { width:1280px; height:720px; }`;
  document.head.append(st);
}

// La targeta «📋 PROMPT MOMENT» sobre un contenidor (position:relative o fixed).
export function showMomentCard(host, mo, team, color, { secs = 5, big = false } = {}) {
  css();
  let el = host.querySelector(':scope > .ar-moment-card');
  if (!el) { el = document.createElement('div'); el.className = 'ar-moment-card'; host.append(el); }
  el.classList.toggle('big', big);
  el.style.setProperty('--mc', color);
  el.innerHTML = `<div class="k">📋 Prompt moment<span>${esc(team)}</span></div><div class="r">«${esc(mo.rule)}»</div>
    <div class="w"><b>What happened:</b> ${esc(mo.what)}</div><div class="w"><b>Why it mattered:</b> ${esc(mo.why)}</div>`;
  el.classList.remove('on'); void el.offsetWidth; el.classList.add('on');
  clearTimeout(el._t); el._t = setTimeout(() => el.classList.remove('on'), secs * 1000);
  return el;
}

// ---- el guió del vídeo (pur) ----

const W = 1280, H = 720;
export const FPS = 30;
// Accelerat entre moments i al voltant de cada moment (per joc).
export const FAST = { nfl: 16, cs2: 12, lol: 6 }, SLOW = { nfl: 1, cs2: 1.5, lol: 1 };
export const INTRO_SECS = 4.5, TAIL_SECS = 1.5, SUMMARY_MIN = 9, SUMMARY_MAX = 30;
// Quant dura la targeta d'un moment i el subtítol (segons de vídeo).
export const CARD_SECS = 6.5, CAPTION_SECS = 7;
// La durada (segons) dels efectes sintetitzats.
export const FX_SECS = { sting: 1.4, roar: 3 };

// On s'alenteix: des de la primera jugada de la decisió del moment (com a molt 8 abans).
export function slowStarts(evs, moments) {
  const out = new Set();
  evs.forEach((e, i) => {
    if (!moments.has(e.n)) return;
    let j = i; while (j > 0 && i - j < 8 && evs[j - 1].decision === e.decision) j--;
    out.add(evs[Math.max(0, j - (j === i ? 1 : 0))].n);
  });
  return out;
}

// El realitzador del vídeo: amb cada esdeveniment que el visor ensenya i el
// temps de vídeo (segons), decideix la velocitat (ràpid entre moments, lent a
// cada moment), la targeta, el subtítol, el marcador i els sons (cues).
export class Director {
  constructor(m, { fast, slow } = {}) {
    this.m = m; this.game = m.game;
    this.fast = fast || FAST[m.game] || 6; this.slow = slow || SLOW[m.game] || 1;
    this.moments = momentsByEvent(m);
    this.lines = new Map((m.lines || []).map(l => [l.n, l]));
    this.slowFrom = slowStarts(m.events || [], this.moments);
    this.speed = this.slow; this.slowUntil = 0; this.playing = false;
    this.card = null; this.cap = null; this.score = [0, 0]; this.prevScore = [0, 0]; this.shown = 0;
    this.cues = [];
  }
  // Comença el partit (després de la presentació).
  begin() { this.playing = true; this.speed = this.fast; }
  // Un esdeveniment que el visor acaba de mostrar, al segon t del vídeo.
  event(e, t) {
    if (!e) return;
    this.shown++;
    if (Array.isArray(e.score)) { this.score = this.game === 'nfl' ? this.prevScore : e.score; this.prevScore = e.score; }
    if (!this.playing) return;
    const mo = this.moments.get(e.n);
    if (this.slowFrom.has(e.n) && !mo) { this.speed = this.slow; this.slowUntil = Infinity; }
    if (mo) {
      this.speed = this.slow; this.slowUntil = t + CARD_SECS;
      this.card = { mo, t0: t, until: t + CARD_SECS };
      this.cues.push({ t, kind: 'sting' });
      const l = this.lines.get(e.n);
      if (l) {
        this.cap = { text: l.text, until: t + CAPTION_SECS };
        if (l.audio) this.cues.push({ t, kind: 'voice', key: l.audio });
      }
    } else if (e.hl && this.speed <= this.slow) this.cues.push({ t, kind: 'roar', big: /touchdown|ace|baron|nexus|win/i.test(e.text || '') });
  }
  // Abans de cada fotograma: torna la velocitat que toca.
  tick(t) {
    if (this.playing && this.speed === this.slow && t > this.slowUntil) this.speed = this.fast;
    return this.speed;
  }
  // El resum del final: la fanfara i, si n'hi ha, la veu de l'analista.
  summary(t) {
    this.cues.push({ t, kind: 'sting' });
    const a = this.m.analysis || {};
    if (a.audio) this.cues.push({ t, kind: 'voice', key: a.audio });
  }
}

// Quant dura el resum: el que diu l'analista (entre 9 i 30 s).
export function summarySecs(voiceSecs) { return Math.min(SUMMARY_MAX, Math.max(SUMMARY_MIN, +voiceSecs || 0)); }

// On sona cada so: [{kind, at, end, key?, big?}] (segons). Una veu nova talla
// l'anterior (com els narradors a la tele); res passa del final del vídeo.
// durs: key → segons de cada veu (les que no hi són no sonen).
export function placeAudio(cues, durs, total) {
  const dur = k => (durs instanceof Map ? durs.get(k) : (durs || {})[k]);
  const sorted = cues.map((c, i) => ({ c, i })).sort((a, b) => a.c.t - b.c.t || a.i - b.i).map(x => x.c);
  const out = [];
  sorted.forEach((c, i) => {
    if (c.t >= total) return;
    if (c.kind === 'voice') {
      const d = dur(c.key);
      if (!(d > 0)) return;
      const next = sorted.slice(i + 1).find(x => x.kind === 'voice' && dur(x.key) > 0 && x.t < total);
      const end = Math.min(c.t + d, next ? next.t : Infinity, total);
      if (end > c.t) out.push({ kind: 'voice', key: c.key, at: c.t, end });
    } else {
      out.push({ kind: c.kind, at: c.t, end: Math.min(total, c.t + (FX_SECS[c.kind] || 1)), ...(c.kind === 'roar' ? { big: !!c.big } : {}) });
    }
  });
  return out;
}

// El segell de temps (microsegons) del fotograma i: sense deriva d'arrodoniment.
export const frameStamp = (i, fps = FPS) => Math.round(i * 1e6 / fps);
// Els segons que falten, amb la fracció feta i els segons que porta (null si encara no se sap).
export function etaSecs(frac, elapsed) {
  if (!(frac > 0.02) || !(elapsed > 0)) return null;
  return Math.max(0, elapsed * (1 - Math.min(1, frac)) / frac);
}
// La fracció de l'exportació: presentació 0-5 %, partit 5-90 % (pel minut del partit) i resum 90-100 %.
export function exportFrac(stage, pr, k = 0) {
  if (stage === 'intro') return 0.05 * Math.min(1, k);
  if (stage === 'summary') return 0.9 + 0.1 * Math.min(1, k);
  const f = pr && pr.duration > 0 ? (pr.elapsed || 0) / pr.duration : pr && pr.total ? (pr.index || 0) / pr.total : 0;
  return 0.05 + 0.85 * Math.max(0, Math.min(1, f));
}

const slug = s => String(s || '').toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 30) || 'team';
export const videoName = (m, ext) => `gregal-arena-${m.game}-${slug(m.names[0])}-vs-${slug(m.names[1])}.${ext}`;

// ---- pintar els rètols ----

function wrap(g, text, maxW) {
  const words = String(text || '').split(/\s+/).filter(Boolean), lines = [];
  let cur = '';
  for (const w of words) { const t = cur ? cur + ' ' + w : w; if (g.measureText(t).width > maxW && cur) { lines.push(cur); cur = w; } else cur = t; }
  if (cur) lines.push(cur);
  return lines;
}
function rrect(g, x, y, w, h, r) { g.beginPath(); g.moveTo(x + r, y); g.arcTo(x + w, y, x + w, y + h, r); g.arcTo(x + w, y + h, x, y + h, r); g.arcTo(x, y + h, x, y, r); g.arcTo(x, y, x + w, y, r); g.closePath(); }
// Text partit en línies; torna la y de sota.
function para(g, text, x, y, maxW, lh, maxLines = 9) {
  let lines = wrap(g, text, maxW);
  if (lines.length > maxLines) { lines = lines.slice(0, maxLines); lines[maxLines - 1] += '…'; }
  lines.forEach((l, i) => g.fillText(l, x, y + i * lh));
  return y + lines.length * lh;
}
const FONT = (w, px) => `${w} ${px}px "Segoe UI", system-ui, sans-serif`;

// Els rètols del vídeo sobre el llenç g (1280×720). t = segons de vídeo.
// stripe: la franja que es mou a la presentació i al resum (només cal per a MediaRecorder).
function painter(g, m, { stripe = true } = {}) {
  const flip = m.game === 'lol' && m.meta && m.meta.blue === 1;
  const LR = flip ? [1, 0] : [0, 1];
  const nm = s => m.names[s] || '';
  const fol = m.followed || [0, 0];
  const moments = momentsByEvent(m);
  const coach = s => { const f = (m.feedback || [])[s]; return f && f.scores ? Object.values(f.scores).reduce((a, b) => a + (+b || 0), 0) : null; };
  function scoreBar(d, pr) {
    const y = H - 62;
    g.fillStyle = 'rgba(6,8,18,.86)'; rrect(g, 24, y, W - 48, 44, 10); g.fill();
    g.font = FONT(800, 22); g.textBaseline = 'middle';
    const [l, r] = LR, sc = `${d.score[l]} - ${d.score[r]}`;
    g.fillStyle = sideColor(m, l); g.fillRect(24, y, 8, 44); g.fillStyle = sideColor(m, r); g.fillRect(W - 32, y, 8, 44);
    g.fillStyle = '#f3f5fb'; g.textAlign = 'left'; g.fillText(nm(l), 46, y + 22); g.textAlign = 'right'; g.fillText(nm(r), W - 46, y + 22);
    g.textAlign = 'center'; g.font = FONT(900, 26); g.fillStyle = '#ffd400'; g.fillText(sc, W / 2, y + 22);
    g.font = FONT(700, 14); g.fillStyle = '#9aa4bd'; g.textAlign = 'left';
    g.fillText(`GREGAL ARENA · ${GAME[m.game] || m.game}${d.speed > d.slow ? `   ▶▶ ${d.speed}×` : ''}`, 24, 26);
    // El minut del partit, el mateix que la barra i la pista del reproductor (temps de reproducció a 1×).
    if (pr && pr.duration) { g.textAlign = 'right'; g.fillText(`${fmtTime(pr.elapsed)} / ${fmtTime(pr.duration)}`, W - 24, 26); g.textAlign = 'left'; }
    g.textBaseline = 'alphabetic';
  }
  function momentCard(card, t) {
    if (!card || t > card.until) return;
    const a = Math.min(1, (t - card.t0) / 0.3, (card.until - t) / 0.4), mo = card.mo, x = 120, w = W - 240;
    g.save(); g.globalAlpha = Math.max(0, a);
    g.font = FONT(400, 22);
    const rl = wrap(g, '«' + mo.rule + '»', w - 70).slice(0, 3), wl = wrap(g, 'What happened: ' + mo.what, w - 40).slice(0, 2), yl = wrap(g, 'Why it mattered: ' + mo.why, w - 40).slice(0, 3);
    const h = 64 + rl.length * 28 + 16 + (wl.length + yl.length) * 29 + 10, y = 40;
    g.fillStyle = 'rgba(8,10,22,.94)'; rrect(g, x, y, w, h, 16); g.fill();
    g.fillStyle = sideColor(m, mo.side); g.fillRect(x, y, 10, h);
    g.font = FONT(900, 18); g.fillStyle = '#ffd400'; g.fillText('📋 PROMPT MOMENT', x + 30, y + 34);
    g.fillStyle = sideColor(m, mo.side); g.fillText(nm(mo.side), x + 30 + g.measureText('📋 PROMPT MOMENT   ').width, y + 34);
    g.fillStyle = 'rgba(255,255,255,.07)'; rrect(g, x + 26, y + 48, w - 52, rl.length * 28 + 14, 8); g.fill();
    g.font = '400 22px ui-monospace, Consolas, monospace'; g.fillStyle = '#f3f5fb';
    rl.forEach((l, i) => g.fillText(l, x + 38, y + 72 + i * 28));
    let yy = y + 72 + rl.length * 28 + 28; g.font = FONT(400, 22);
    // L'etiqueta en groc, el text en blanc.
    [[wl, 'What happened:'], [yl, 'Why it mattered:']].forEach(([ls, label]) => ls.forEach((l, i) => {
      let x0 = x + 30;
      if (i === 0 && l.startsWith(label)) { g.fillStyle = '#ffd400'; g.fillText(label, x0, yy); x0 += g.measureText(label + ' ').width; l = l.slice(label.length + 1); }
      g.fillStyle = '#e6e9f2'; g.fillText(l, x0, yy); yy += 29;
    }));
    g.restore();
  }
  function caption(cap, t) {
    if (!cap || t > cap.until) return;
    g.font = FONT(600, 21); const ls = wrap(g, cap.text, W - 300).slice(0, 2), h = ls.length * 28 + 20, y = H - 80 - h;
    g.fillStyle = 'rgba(8,10,22,.88)'; rrect(g, 150, y, W - 300, h, 10); g.fill(); g.fillStyle = '#ffd400'; g.fillRect(150, y, 5, h);
    g.fillStyle = '#f3f5fb'; ls.forEach((l, i) => g.fillText(l, 170, y + 32 + i * 28));
  }
  function bg(t) { const gr = g.createRadialGradient(W / 2, -200, 100, W / 2, 0, 1100); gr.addColorStop(0, '#1d2a5c'); gr.addColorStop(1, '#070a14'); g.fillStyle = gr; g.fillRect(0, 0, W, H);
    // Una franja que es mou: els rètols fixos també han de fer fotogrames (si no, la gravació en temps real hi deixa forats).
    if (stripe) { const p = (t / 4) % 1; g.fillStyle = '#ffd400'; g.fillRect(0, H - 6, W * p, 6); } }
  function intro(t) {
    bg(t); g.textAlign = 'center';
    g.font = FONT(900, 20); g.fillStyle = '#ffd400'; g.fillText('GREGAL ARENA · HIGHLIGHTS', W / 2, 120);
    g.font = FONT(800, 30); g.fillStyle = '#9aa4bd'; g.fillText(GAME[m.game] || m.game, W / 2, 170);
    [0, 1].forEach(i => {
      const s = LR[i], x = i ? W * .74 : W * .26;
      g.fillStyle = sideColor(m, s); g.fillRect(x - 220, 230, 440, 6);
      g.font = FONT(900, 44); g.fillStyle = '#f3f5fb'; g.fillText(nm(s), x, 300);
      g.font = 'italic 400 20px "Segoe UI", system-ui, sans-serif'; g.fillStyle = '#c9cfe0'; para(g, (m.titles || [])[s] || '', x, 340, 420, 26, 3);
    });
    g.font = FONT(900, 60); g.fillStyle = '#ffd400'; g.fillText('VS', W / 2, 310);
    g.font = FONT(600, 22); g.fillStyle = '#d6dbea';
    g.fillText(`${moments.size} prompt moment${moments.size === 1 ? '' : 's'}: where a line of a team's .md decided a play.`, W / 2, 520);
    g.font = FONT(400, 18); g.fillStyle = '#9aa4bd'; g.fillText('Same model, same players: the only difference is each team\'s strategy .md.', W / 2, 556);
    g.textAlign = 'left';
  }
  function summary(t) {
    bg(t);
    const a = m.analysis || {}, mos = (a.moments || []);
    g.font = FONT(900, 18); g.fillStyle = '#ffd400'; g.fillText('MATCH SUMMARY · ' + (GAME[m.game] || m.game).toUpperCase(), 60, 60);
    const [l, r] = LR;
    g.font = FONT(900, 46); g.fillStyle = '#f3f5fb'; g.fillText(`${nm(l)} ${m.score[l]} - ${m.score[r]} ${nm(r)}`, 60, 118);
    g.font = FONT(800, 24); g.fillStyle = '#ffd400'; g.fillText(m.winner >= 0 ? `🏆 ${nm(m.winner)} wins` : 'Draw', 60, 156);
    // Esquerra: l'anàlisi i la regla; dreta: els moments.
    let y = 200;
    if (a.headline) { g.font = FONT(800, 22); g.fillStyle = '#f3f5fb'; y = para(g, a.headline, 60, y, 560, 28, 2) + 4; g.font = FONT(400, 17); g.fillStyle = '#c9cfe0'; y = para(g, a.text, 60, y, 560, 23, 6) + 10; }
    if (a.rule) { g.font = FONT(800, 13); g.fillStyle = '#ffd400'; g.fillText('THE RULE THAT DECIDED IT · ' + nm(a.side === 1 ? 1 : 0).toUpperCase(), 60, y + 6); g.font = '400 17px ui-monospace, Consolas, monospace'; g.fillStyle = '#f3f5fb'; y = para(g, '«' + a.rule + '»', 60, y + 30, 560, 22, 3) + 8; }
    y = Math.max(y, 540);
    g.font = FONT(800, 13); g.fillStyle = '#9aa4bd'; g.fillText('COACH SCORE · FOLLOWED ITS .MD', 60, y);
    [0, 1].forEach((s, i) => {
      const c = coach(s);
      g.fillStyle = sideColor(m, s); g.fillRect(60, y + 16 + i * 40, 6, 30);
      g.font = FONT(700, 20); g.fillStyle = '#f3f5fb'; g.fillText(`${nm(s)}  ·  ${c == null ? '—' : c + '/25'}  ·  followed its .md ${Math.round((fol[s] || 0) * 100)}%`, 78, y + 38 + i * 40);
    });
    g.font = FONT(900, 15); g.fillStyle = '#ffd400'; g.fillText('📋 PROMPT MOMENTS', 680, 200);
    let yy = 230;
    mos.slice(0, 6).forEach((mo, i) => {
      g.fillStyle = sideColor(m, mo.side); g.beginPath(); g.arc(690, yy - 6, 7, 0, 7); g.fill();
      g.font = FONT(800, 17); g.fillStyle = '#f3f5fb'; g.fillText(`${i + 1}. ${nm(mo.side)}`, 706, yy);
      g.font = FONT(400, 15); g.fillStyle = '#c9cfe0'; yy = para(g, mo.what, 706, yy + 21, 520, 19, 2);
      g.font = 'italic 400 14px ui-monospace, Consolas, monospace'; g.fillStyle = '#9aa4bd'; yy = para(g, '«' + mo.rule + '»', 706, yy + 1, 520, 18, 1) + 14;
    });
  }
  // Un fotograma del partit: el visor i, a sobre, el marcador, la targeta i el subtítol.
  // vc: el llenç del visor, si és un altre (null: ja és a sota, al mateix llenç).
  function play(vc, d, t, pr) {
    if (vc) { g.fillStyle = '#000'; g.fillRect(0, 0, W, H); try { g.drawImage(vc, 0, 0, W, H); } catch (e) { /* encara no */ } }
    scoreBar(d, pr); momentCard(d.card, t); caption(d.cap, t);
  }
  // Sempre sense transformacions ni estat del visor (que pinta al mateix llenç).
  const clean = f => (...a) => { g.save(); g.setTransform(1, 0, 0, 1, 0, 0); g.globalAlpha = 1; g.globalCompositeOperation = 'source-over'; g.shadowBlur = 0; g.shadowColor = 'transparent'; g.filter = 'none'; g.textAlign = 'left'; g.textBaseline = 'alphabetic'; try { f(...a); } finally { g.restore(); } };
  return { intro: clean(intro), summary: clean(summary), play: clean(play) };
}

// ---- la finestra de progrés ----

function progressUI(fast) {
  css();
  const ui = document.createElement('div'); ui.className = 'ar-export';
  ui.innerHTML = `<div class="box"><h3>⬇ Exporting highlights video</h3><canvas width="${W}" height="${H}"></canvas>
    <div class="bar"><i></i></div><div class="row"><span>Preparing…</span><button type="button">Cancel</button></div>
    <small style="color:#9aa4bd">${fast ? 'Rendering frame by frame, faster than real time: you can switch to another tab meanwhile.' : 'Recording in real time: keep this window open and visible.'}</small></div><div class="stage"><canvas></canvas></div>`;
  document.body.append(ui);
  const state = { cancelled: false };
  ui.querySelector('.row button').onclick = () => { state.cancelled = true; };
  return {
    ui, state,
    canvas: ui.querySelector('.box canvas'), stage: ui.querySelector('.stage canvas'),
    status: t => { ui.querySelector('.row span').textContent = t; },
    bar: f => { ui.querySelector('.bar i').style.width = Math.round(Math.max(0, Math.min(1, f)) * 100) + '%'; },
  };
}

function download(blob, name) {
  const url = URL.createObjectURL(blob), link = document.createElement('a');
  link.href = url; link.download = name; document.body.append(link); link.click(); link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 60000);
}

// exportVideo: munta el vídeo i el descarrega. opts:
//   id, fetchMatch(after) → Promise<partit>, fetchVoice(key) → Promise<ArrayBuffer|null>,
//   mode: 'auto' (WebCodecs si es pot; si no, en temps real) | 'fast' | 'realtime',
//   download (false: no el descarrega; només el torna).
// Torna { name, size, type, blob, mode, secs (durada), wall (segons que ha costat) } o null si es cancel·la.
export async function exportVideo({ id, fetchMatch, fetchVoice, mode = 'auto', download: dl = true }) {
  const m = await fetchMatch(0);
  if (!m || m.status !== 'final') throw new Error('The match is not finished yet.');
  if (mode !== 'realtime') {
    const enc = await encoderSupport();
    if (enc) {
      try {
        const r = await exportFast(m, { fetchVoice, enc });
        if (r && dl) download(r.blob, r.name);
        return r;
      } catch (e) {
        if (e && e.cancelled) return null;
        if (mode === 'fast') throw e;
        console.warn('arena export: WebCodecs failed, recording in real time instead', e);
      }
    } else if (mode === 'fast') throw new Error('This browser cannot encode H.264 with WebCodecs.');
  }
  const r = await exportRealtime(m, { fetchMatch, fetchVoice });
  if (r && dl) download(r.blob, r.name);
  return r;
}

// ---- WebCodecs: fotograma a fotograma ----

const VIDEO_CODECS = ['avc1.640028', 'avc1.4d0028', 'avc1.42e028', 'avc1.42001f'];
// El codificador de programari (OpenH264) primer: als equips provats va el doble
// de ràpid que el de la GPU per a aquest ús (molts fotogrames seguits, sense pressa
// de latència) i és igual a tot arreu; si no hi és, el que triï el navegador.
const ACCEL = ['prefer-software', 'no-preference'];
// Quins còdecs hi ha: { video: config, audio: config|null, audioCodec: 'aac'|'opus'|null } o null.
export async function encoderSupport() {
  if (typeof VideoEncoder !== 'function' || typeof VideoFrame !== 'function') return null;
  let video = null;
  outer: for (const hardwareAcceleration of ACCEL) for (const codec of VIDEO_CODECS) {
    const cfg = { codec, width: W, height: H, bitrate: 4_000_000, framerate: FPS, avc: { format: 'avc' }, latencyMode: 'quality', hardwareAcceleration };
    try { const s = await VideoEncoder.isConfigSupported(cfg); if (s && s.supported) { video = cfg; break outer; } } catch (e) { /* el següent */ }
  }
  if (!video) return null;
  let audio = null, audioCodec = null;
  if (typeof AudioEncoder === 'function' && typeof AudioData === 'function' && typeof OfflineAudioContext === 'function') {
    for (const [codec, name] of [['mp4a.40.2', 'aac'], ['opus', 'opus']]) {
      const cfg = { codec, sampleRate: 48000, numberOfChannels: 2, bitrate: 128_000 };
      try { const s = await AudioEncoder.isConfigSupported(cfg); if (s && s.supported) { audio = cfg; audioCodec = name; break; } } catch (e) { /* el següent */ }
    }
  }
  return { video, audio, audioCodec };
}

// Cedeix el torn (sense setTimeout, que a les pestanyes amagades s'endarrereix).
let chan = null;
function yieldTask() {
  if (typeof MessageChannel !== 'function') return Promise.resolve();
  if (!chan) { chan = new MessageChannel(); chan.q = []; chan.port1.onmessage = () => { const f = chan.q.shift(); if (f) f(); }; }
  return new Promise(r => { chan.q.push(r); chan.port2.postMessage(0); });
}

// El visor en un iframe propi, amb el rellotge a mà: performance.now() torna el
// temps de vídeo, requestAnimationFrame no fa res (el bucle el portem nosaltres)
// i la densitat de píxels és 1. Torna { win, canvas, clock, mod, remove }.
async function viewRealm() {
  const fr = document.createElement('iframe');
  fr.setAttribute('aria-hidden', 'true'); fr.tabIndex = -1;
  fr.style.cssText = `position:fixed;left:-30000px;top:0;width:${W}px;height:${H}px;border:0;pointer-events:none`;
  fr.srcdoc = `<!doctype html><meta charset="utf-8"><body style="margin:0;background:#000;overflow:hidden"><canvas style="display:block;width:${W}px;height:${H}px"></canvas></body>`;
  const loaded = new Promise(r => fr.addEventListener('load', r, { once: true }));
  document.body.append(fr);
  await loaded;
  const win = fr.contentWindow, doc = fr.contentDocument;
  const clock = { ms: 0 };
  Object.defineProperty(win.performance, 'now', { configurable: true, value: () => clock.ms });
  win.requestAnimationFrame = () => 1;
  win.cancelAnimationFrame = () => {};
  win.requestIdleCallback = f => { Promise.resolve().then(() => f({ didTimeout: true, timeRemaining: () => 0 })); return 1; };
  try { Object.defineProperty(win, 'devicePixelRatio', { configurable: true, get: () => 1 }); } catch (e) { /* res */ }
  const url = new URL('./arena-views.js', import.meta.url).href, key = '__arenaViews' + Math.random().toString(36).slice(2);
  const mod = await Promise.race([
    new Promise((res, rej) => {
      win[key] = { res, rej };
      const s = doc.createElement('script'); s.type = 'module';
      s.textContent = `import(${JSON.stringify(url)}).then(m => window[${JSON.stringify(key)}].res(m), e => window[${JSON.stringify(key)}].rej(e));`;
      doc.body.append(s);
    }),
    new Promise((_, rej) => setTimeout(() => rej(new Error('the viewer did not load')), 20000)),
  ]);
  return { win, canvas: doc.querySelector('canvas'), clock, mod, remove: () => fr.remove() };
}

// Els efectes i les veus, mesclats de cop (OfflineAudioContext) → AudioBuffer.
export async function renderAudio(placed, buffers, secs, sampleRate = 48000) {
  const ctx = new OfflineAudioContext(2, Math.max(1, Math.ceil(secs * sampleRate)), sampleRate);
  const master = ctx.createGain(); master.gain.value = .9; master.connect(ctx.destination);
  let seed = 7;
  const rnd = () => ((seed = (seed * 1664525 + 1013904223) >>> 0) / 4294967296);
  const noise = len => { const b = ctx.createBuffer(1, Math.ceil(sampleRate * len), sampleRate), d = b.getChannelData(0); let last = 0; for (let i = 0; i < d.length; i++) { last = (last + .02 * (rnd() * 2 - 1)) / 1.02; d[i] = last * 3.5; } return b; };
  for (const p of placed) {
    const t = p.at;
    if (p.kind === 'sting') {
      [[523.25, 0], [659.25, .12], [783.99, .24], [1046.5, .36]].forEach(([f, d]) => {
        const o = ctx.createOscillator(), g = ctx.createGain(); o.type = 'triangle'; o.frequency.value = f;
        g.gain.setValueAtTime(0, t + d); g.gain.linearRampToValueAtTime(.18, t + d + .03); g.gain.exponentialRampToValueAtTime(.001, t + d + .9);
        o.connect(g); g.connect(master); o.start(t + d); o.stop(Math.min(p.end, t + d + 1));
      });
    } else if (p.kind === 'roar') {
      const src = ctx.createBufferSource(); src.buffer = noise(3);
      const bp = ctx.createBiquadFilter(); bp.type = 'bandpass'; bp.frequency.value = 1100; bp.Q.value = .5;
      const g = ctx.createGain(); g.gain.setValueAtTime(0, t); g.gain.linearRampToValueAtTime(p.big ? .6 : .3, t + .3); g.gain.exponentialRampToValueAtTime(.001, t + (p.big ? 2.8 : 1.8));
      src.connect(bp); bp.connect(g); g.connect(master); src.start(t); src.stop(p.end);
    } else if (p.kind === 'voice') {
      const buf = buffers.get(p.key); if (!buf) continue;
      const src = ctx.createBufferSource(); src.buffer = buf; src.connect(master); src.start(t); src.stop(p.end);
    }
  }
  return ctx.startRendering();
}

async function exportFast(m, { fetchVoice, enc }) {
  const { Muxer, ArrayBufferTarget } = await import('./vendor/mp4-muxer.esm.min.js');
  const P = progressUI(true), preview = P.canvas.getContext('2d');
  const cancelled = () => { if (P.state.cancelled) { const e = new Error('cancelled'); e.cancelled = true; throw e; } };
  const t0 = performance.now();
  let realm = null, view = null, venc = null, aenc = null;
  try {
    // Les veus primer (en paral·lel): cal saber quant duren per fer el resum.
    P.status('Preparing…');
    const dir = new Director(m);
    const want = new Set();
    for (const e of m.events || []) { const l = dir.moments.has(e.n) && dir.lines.get(e.n); if (l && l.audio) want.add(l.audio); }
    if (m.analysis && m.analysis.audio) want.add(m.analysis.audio);
    const buffers = new Map();
    if (enc.audio && fetchVoice) {
      const dec = new OfflineAudioContext(2, 1, 48000);
      await Promise.all([...want].map(async k => {
        // Dos intents: una veu que falla un moment no ha de deixar el vídeo mut.
        for (let i = 0; i < 2 && !buffers.has(k); i++) {
          try { const ab = await fetchVoice(k); if (ab) buffers.set(k, await dec.decodeAudioData(ab.slice(0))); } catch (e) { /* sense veu */ }
        }
      }));
    }
    cancelled();
    realm = await viewRealm();
    const data = realm.win.structuredClone(m), clock = { t: 0 };
    view = await realm.mod.makeView(m.game, realm.canvas, { lang: 'en', fetchMatch: () => Promise.resolve(data), onEvent: e => dir.event(e, clock.t) });
    const total = (m.events || []).length;
    for (let k = 0; k < 400; k++) { const pr = view.getProgress(); if (view.match && pr.total >= total) break; await yieldTask(); }
    if (!view.match) throw new Error('the viewer did not load the match');
    cancelled();

    const target = new ArrayBufferTarget();
    const muxer = new Muxer({
      target, fastStart: 'in-memory', firstTimestampBehavior: 'offset',
      video: { codec: 'avc', width: W, height: H, frameRate: FPS },
      ...(enc.audio ? { audio: { codec: enc.audioCodec, sampleRate: enc.audio.sampleRate, numberOfChannels: enc.audio.numberOfChannels } } : {}),
    });
    let encErr = null;
    venc = new VideoEncoder({ output: (chunk, meta) => muxer.addVideoChunk(chunk, meta), error: e => { encErr = e; } });
    venc.configure(enc.video);
    // Els rètols es pinten al mateix llenç del visor (sense copiar-lo a un altre
    // a cada fotograma, que és el que més costava) i el fotograma surt d'allà.
    const out = realm.canvas;
    if (out.width !== W || out.height !== H) { out.width = W; out.height = H; }
    const paint = painter(out.getContext('2d'), m, { stripe: false });
    let frame = 0, stage = 'intro', lastUI = 0;
    const dt = 1 / FPS;
    // On va el temps (ms): el visor, els rètols, el fotograma i l'espera al codificador.
    const prof = { view: 0, paint: 0, frame: 0, wait: 0, audio: 0 };
    const lap = (k, t) => { const n = performance.now(); prof[k] += n - t; return n; };
    const ui = (k) => {
      const now = performance.now();
      if (now - lastUI < 250) return;
      lastUI = now;
      const f = exportFrac(stage, stage === 'play' ? view.getProgress() : null, k), wall = (now - t0) / 1000, eta = etaSecs(f, wall);
      P.bar(f);
      const x = clock.t / Math.max(0.001, wall);
      P.status(`${stage === 'intro' ? 'Intro' : stage === 'summary' ? 'Match summary' : `Match · ${Math.min(dir.shown, total)}/${total} plays`} · ${Math.round(f * 100)}% · ${x.toFixed(1)}× real time${eta != null ? ` · about ${fmtTime(Math.ceil(eta))} left` : ''}`);
    };
    // still: una imatge fixa ja feta (la presentació i el resum no es mouen: es pinten un cop).
    const emit = async still => {
      if (encErr) throw encErr;
      let t = performance.now();
      // Una instantània (ImageBitmap) del llenç: no espera que la GPU acabi de
      // pintar-lo, així el fotograma següent es prepara mentre aquest es pinta i es codifica.
      const bmp = still || await createImageBitmap(out);
      const vf = new VideoFrame(bmp, { timestamp: frameStamp(frame, FPS), duration: frameStamp(1, FPS) });
      if (!still) bmp.close();
      // La vista prèvia de la finestra, de tant en tant.
      if (frame % 15 === 0) try { preview.drawImage(out, 0, 0, P.canvas.width, P.canvas.height); } catch (e) { /* res */ }
      venc.encode(vf, { keyFrame: frame % (FPS * 2) === 0 });
      vf.close();
      t = lap('frame', t);
      frame++; clock.t = frame / FPS; realm.clock.ms = clock.t * 1000;
      // Que el codificador no s'empatxi, i que la pàgina respiri.
      while (venc.encodeQueueSize > 12) await yieldTask();
      if (frame % 8 === 0) { await yieldTask(); cancelled(); }
      lap('wait', t);
    };
    // 1) La presentació.
    const nIntro = Math.round(INTRO_SECS * FPS);
    paint.intro(0);
    let still = await createImageBitmap(out);
    for (let i = 0; i < nIntro; i++) { ui(i / nIntro); await emit(still); }
    still.close();
    // 2) El partit, pas a pas (el visor avança dt × velocitat a cada fotograma).
    stage = 'play'; dir.begin();
    const maxFrames = frame + 20 * 60 * FPS;
    let tail = -1;
    while (frame < maxFrames) {
      let t = performance.now();
      view.setSpeed(dir.tick(clock.t));
      view.onFrame(dt, realm.clock.ms);
      t = lap('view', t);
      paint.play(null, dir, clock.t, view.getProgress());
      lap('paint', t);
      ui();
      await emit();
      if (tail < 0 && view.isFinished()) tail = Math.round(TAIL_SECS * FPS);
      if (tail >= 0 && tail-- === 0) break;
    }
    // 3) El resum, el temps que parla l'analista (9-30 s).
    stage = 'summary';
    const a = m.analysis || {}, sumAt = clock.t;
    dir.summary(sumAt);
    const nSum = Math.round(summarySecs(a.audio && buffers.get(a.audio) ? buffers.get(a.audio).duration : 0) * FPS);
    paint.summary(clock.t);
    still = await createImageBitmap(out);
    for (let i = 0; i < nSum; i++) { ui(i / nSum); await emit(still); }
    still.close();
    const secs = frame / FPS;
    view.destroy(); view = null; realm.remove(); realm = null;
    await venc.flush(); venc.close(); venc = null;
    if (encErr) throw encErr;

    // 4) El so: tot el vídeo de cop, i a l'AudioEncoder per trossos.
    const ta = performance.now();
    if (enc.audio) {
      P.status('Mixing the sound…');
      const durs = new Map([...buffers].map(([k, b]) => [k, b.duration]));
      const mix = await renderAudio(placeAudio(dir.cues, durs, secs), buffers, secs, enc.audio.sampleRate);
      cancelled();
      aenc = new AudioEncoder({ output: (chunk, meta) => muxer.addAudioChunk(chunk, meta), error: e => { encErr = e; } });
      aenc.configure(enc.audio);
      const sr = mix.sampleRate, N = mix.length, step = 4096, L = mix.getChannelData(0), R = mix.getChannelData(1);
      for (let at = 0; at < N; at += step) {
        const n = Math.min(step, N - at), buf = new Float32Array(n * 2);
        buf.set(L.subarray(at, at + n), 0); buf.set(R.subarray(at, at + n), n);
        const ad = new AudioData({ format: 'f32-planar', sampleRate: sr, numberOfFrames: n, numberOfChannels: 2, timestamp: Math.round(at * 1e6 / sr), data: buf });
        aenc.encode(ad); ad.close();
        if (aenc.encodeQueueSize > 16) await yieldTask();
      }
      await aenc.flush(); aenc.close(); aenc = null;
      if (encErr) throw encErr;
    }
    lap('audio', ta);
    muxer.finalize();
    const blob = new Blob([target.buffer], { type: 'video/mp4' });
    const wall = (performance.now() - t0) / 1000;
    for (const k in prof) prof[k] = Math.round(prof[k]);
    return { name: videoName(m, 'mp4'), size: blob.size, type: blob.type, blob, mode: 'fast', secs, wall, frames: frame, prof };
  } finally {
    if (view) try { view.destroy(); } catch (e) { /* res */ }
    if (realm) realm.remove();
    if (venc && venc.state !== 'closed') try { venc.close(); } catch (e) { /* res */ }
    if (aenc && aenc.state !== 'closed') try { aenc.close(); } catch (e) { /* res */ }
    P.ui.remove();
  }
}

// ---- en temps real (MediaRecorder): el camí de reserva ----

function pickMime() {
  const types = ['video/mp4;codecs=avc1.42E01E,mp4a.40.2', 'video/mp4;codecs=avc1,mp4a', 'video/mp4', 'video/webm;codecs=vp9,opus', 'video/webm;codecs=vp8,opus', 'video/webm'];
  return types.find(t => window.MediaRecorder && MediaRecorder.isTypeSupported(t)) || '';
}

// Sons generats (com a la tele) cap a la gravació.
function audioRig() {
  const C = window.AudioContext || window.webkitAudioContext;
  if (!C) return null;
  const ctx = new C(), dest = ctx.createMediaStreamDestination(), master = ctx.createGain();
  master.gain.value = .9; master.connect(dest);
  // Un silenci actiu: sense cap so connectat, la pista d'àudio no fa mostres,
  // la gravació se'n salta els trams i el so es desquadra amb la imatge.
  if (ctx.createConstantSource) { const zero = ctx.createConstantSource(); zero.offset.value = 0; zero.connect(dest); zero.start(); }
  const noise = secs => { const b = ctx.createBuffer(1, ctx.sampleRate * secs, ctx.sampleRate), d = b.getChannelData(0); let last = 0; for (let i = 0; i < d.length; i++) { last = (last + .02 * (Math.random() * 2 - 1)) / 1.02; d[i] = last * 3.5; } return b; };
  let voice = null;
  return {
    ctx, dest,
    sting() {
      const t = ctx.currentTime;
      [[523.25, 0], [659.25, .12], [783.99, .24], [1046.5, .36]].forEach(([f, d]) => {
        const o = ctx.createOscillator(), g = ctx.createGain(); o.type = 'triangle'; o.frequency.value = f;
        g.gain.setValueAtTime(0, t + d); g.gain.linearRampToValueAtTime(.18, t + d + .03); g.gain.exponentialRampToValueAtTime(.001, t + d + .9);
        o.connect(g); g.connect(master); o.start(t + d); o.stop(t + d + 1);
      });
    },
    roar(big) {
      const src = ctx.createBufferSource(); src.buffer = noise(3);
      const bp = ctx.createBiquadFilter(); bp.type = 'bandpass'; bp.frequency.value = 1100; bp.Q.value = .5;
      const g = ctx.createGain(), t = ctx.currentTime; g.gain.setValueAtTime(0, t); g.gain.linearRampToValueAtTime(big ? .6 : .3, t + .3); g.gain.exponentialRampToValueAtTime(.001, t + (big ? 2.8 : 1.8));
      src.connect(bp); bp.connect(g); g.connect(master); src.start(); src.stop(t + 3);
    },
    // Una frase dels narradors; torna quan s'acaba (o de seguida si no n'hi ha).
    async say(buf) {
      if (!buf) return;
      let ab; try { ab = await ctx.decodeAudioData(buf.slice(0)); } catch (e) { return; }
      if (voice) try { voice.stop(); } catch (e) { /* ja parat */ }
      const src = ctx.createBufferSource(); src.buffer = ab; src.connect(master); voice = src;
      await new Promise(res => { src.onended = res; src.start(); });
    },
  };
}

async function exportRealtime(m, { fetchMatch, fetchVoice }) {
  const mime = pickMime();
  if (!mime || !HTMLCanvasElement.prototype.captureStream) throw new Error('This browser cannot record video.');
  const { makeView } = await import('./arena-views.js');
  const t0 = performance.now();
  const ext = mime.startsWith('video/mp4') ? 'mp4' : 'webm';
  const evs = m.events || [];
  const P = progressUI(false), rec = P.canvas, g = rec.getContext('2d'), vc = P.stage;
  const dir = new Director(m), paint = painter(g, m);
  const sec = () => performance.now() / 1000;
  let stage = 'intro', view = null, heard = 0;

  const rig = audioRig();
  if (rig) await rig.ctx.resume().catch(() => {});
  const stream = new MediaStream([...rec.captureStream(30).getVideoTracks(), ...(rig ? rig.dest.stream.getAudioTracks() : [])]);
  const chunks = [];
  const mr = new MediaRecorder(stream, { mimeType: mime, videoBitsPerSecond: 3_500_000, audioBitsPerSecond: 128_000 });
  mr.ondataavailable = e => { if (e.data && e.data.size) chunks.push(e.data); };
  const stopped = new Promise(res => { mr.onstop = res; });

  const voice = async key => { if (!key || !rig || !fetchVoice) return null; try { return await fetchVoice(key); } catch (e) { return null; } };
  // Els sons que el realitzador demana, a mesura que surten.
  const play = () => {
    for (; heard < dir.cues.length; heard++) {
      const c = dir.cues[heard];
      if (!rig) continue;
      if (c.kind === 'sting') rig.sting();
      else if (c.kind === 'roar') rig.roar(c.big);
      else if (c.kind === 'voice' && stage === 'play') voice(c.key).then(b => rig.say(b));
    }
  };
  try {
    view = await makeView(m.game, vc, { lang: 'en', fetchMatch: after => fetchMatch(after), onEvent: e => { dir.event(e, sec()); play(); } });
  } catch (e) { P.ui.remove(); throw e; }
  if (!view.paused) view.togglePause();

  let raf = 0;
  const frame = () => {
    const t = sec();
    if (stage === 'intro') paint.intro(t);
    else if (stage === 'summary') paint.summary(t);
    else {
      if (!P.state.cancelled) { const s = dir.tick(t); if (s !== view.speed) view.setSpeed(s); }
      paint.play(vc, dir, t, view.getProgress ? view.getProgress() : null);
    }
    raf = requestAnimationFrame(frame);
  };
  frame();
  // El primer dibuix del visor (l'estadi, el mapa) bloqueja una estona: es
  // grava quan ja va fi, que si no el vídeo comença amb un forat.
  P.status('Preparing…');
  for (let ok = 0, t = performance.now(), end = t + 10000; (ok < 15 || !view.match) && t < end;) {
    const n = await new Promise(r => requestAnimationFrame(r));
    ok = n - t < 60 ? ok + 1 : 0; t = n;
  }
  mr.start(1000);
  const deadline = Date.now() + 20 * 60 * 1000;
  const recStart = performance.now();
  try {
    P.status('Intro'); await sleep(INTRO_SECS * 1000);
    if (P.state.cancelled) throw new Error('cancelled');
    stage = 'play'; dir.begin(); view.setSpeed(dir.speed); view.togglePause();
    while (!view.isFinished() && !P.state.cancelled && Date.now() < deadline) {
      P.bar(exportFrac('play', view.getProgress()));
      P.status(`Recording the match · ${Math.min(dir.shown, evs.length)}/${evs.length} plays${dir.speed > dir.slow ? ` · ${dir.speed}×` : ' · prompt moment'}`);
      await sleep(400);
    }
    if (P.state.cancelled) throw new Error('cancelled');
    await sleep(TAIL_SECS * 1000);
    stage = 'summary'; P.status('Match summary'); P.bar(.95);
    dir.summary(sec()); play();
    const a = m.analysis || {};
    const said = a.audio ? voice(a.audio).then(b => rig && rig.say(b)) : null;
    await Promise.race([Promise.all([sleep(SUMMARY_MIN * 1000), said || Promise.resolve()]), sleep(SUMMARY_MAX * 1000)]);
  } catch (e) {
    P.state.cancelled = true;
  } finally {
    view.destroy();
  }
  mr.stop(); await stopped;
  const secs = (performance.now() - recStart) / 1000;
  cancelAnimationFrame(raf);
  if (rig) rig.ctx.close().catch(() => {});
  stream.getTracks().forEach(t => t.stop());
  P.ui.remove();
  if (P.state.cancelled) return null;
  const blob = new Blob(chunks, { type: mime.split(';')[0] });
  return { name: videoName(m, ext), size: blob.size, type: blob.type, blob, mode: 'realtime', secs, wall: (performance.now() - t0) / 1000 };
}
