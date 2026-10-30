// Els «prompt moments» d'un partit de l'Arena (analysis.moments, arena_moments.go):
// la targeta que surt quan es veu la jugada (a la tele i al visor de l'Arena) i
// el vídeo de resum que es descarrega: presentació → el partit accelerat entre
// moments i a velocitat normal a cada moment, amb la targeta → el resum. Es grava
// amb MediaRecorder d'un llenç propi on es pinten el visor i els rètols (els
// elements de la pàgina no surten a la gravació), amb el so dels efectes i la veu.
import { makeView } from './arena-views.js';

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
.ar-export .row { display:flex; gap:10px; align-items:center; } .ar-export .row span { flex:1; color:#9aa4bd; }
.ar-export button { font:inherit; border:1px solid rgba(255,255,255,.18); background:#1a2138; color:#f3f5fb; border-radius:8px; padding:6px 14px; cursor:pointer; }
.ar-export .stage { position:fixed; left:-20000px; top:0; width:1280px; height:720px; } .ar-export .stage canvas { width:1280px; height:720px; }`;
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

// ---- el vídeo ----

const W = 1280, H = 720;
// Accelerat entre moments i al voltant de cada moment (per joc).
const FAST = { nfl: 16, cs2: 12, lol: 6 }, SLOW = { nfl: 1, cs2: 1.5, lol: 1 };

function pickMime() {
  const types = ['video/mp4;codecs=avc1.42E01E,mp4a.40.2', 'video/mp4;codecs=avc1,mp4a', 'video/mp4', 'video/webm;codecs=vp9,opus', 'video/webm;codecs=vp8,opus', 'video/webm'];
  return types.find(t => window.MediaRecorder && MediaRecorder.isTypeSupported(t)) || '';
}
const slug = s => String(s || '').toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 30) || 'team';

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

// exportVideo: munta el vídeo i el descarrega. opts:
//   id, fetchMatch(after) → Promise<partit>, fetchVoice(key) → Promise<ArrayBuffer|null>.
export async function exportVideo({ id, fetchMatch, fetchVoice }) {
  css();
  const m = await fetchMatch(0);
  if (!m || m.status !== 'final') throw new Error('The match is not finished yet.');
  const mime = pickMime();
  if (!mime || !HTMLCanvasElement.prototype.captureStream) throw new Error('This browser cannot record video.');
  const ext = mime.startsWith('video/mp4') ? 'mp4' : 'webm';
  const fname = `gregal-arena-${m.game}-${slug(m.names[0])}-vs-${slug(m.names[1])}.${ext}`;
  const flip = m.game === 'lol' && m.meta && m.meta.blue === 1;
  const LR = flip ? [1, 0] : [0, 1];
  const moments = momentsByEvent(m), evs = m.events || [];
  const lines = new Map((m.lines || []).map(l => [l.n, l]));
  const fast = FAST[m.game] || 6, slow = SLOW[m.game] || 1;
  // On s'alenteix: des de la primera jugada de la decisió del moment (com a molt 8 abans).
  const slowFrom = new Set();
  evs.forEach((e, i) => {
    if (!moments.has(e.n)) return;
    let j = i; while (j > 0 && i - j < 8 && evs[j - 1].decision === e.decision) j--;
    slowFrom.add(evs[Math.max(0, j - (j === i ? 1 : 0))].n);
  });

  // La finestra de progrés, amb la gravació a la vista.
  const ui = document.createElement('div'); ui.className = 'ar-export';
  ui.innerHTML = `<div class="box"><h3>⬇ Exporting highlights video</h3><canvas width="${W}" height="${H}"></canvas>
    <div class="bar"><i></i></div><div class="row"><span>Preparing…</span><button type="button">Cancel</button></div>
    <small style="color:#9aa4bd">Recording in real time: keep this window open and visible.</small></div><div class="stage"><canvas></canvas></div>`;
  document.body.append(ui);
  const rec = ui.querySelector('.box canvas'), g = rec.getContext('2d'), vc = ui.querySelector('.stage canvas');
  const status = t => { ui.querySelector('.row span').textContent = t; };
  const bar = f => { ui.querySelector('.bar i').style.width = Math.round(Math.max(0, Math.min(1, f)) * 100) + '%'; };
  let cancelled = false, stage = 'intro', shownN = 0, score = [0, 0], prevScore = [0, 0], speed = slow;
  let card = null, cap = null, slowUntil = 0, view = null;
  const setSpeed = x => { speed = x; if (view) view.setSpeed(x); };
  ui.querySelector('button').onclick = () => { cancelled = true; };

  const rig = audioRig();
  if (rig) await rig.ctx.resume().catch(() => {});
  const stream = new MediaStream([...rec.captureStream(30).getVideoTracks(), ...(rig ? rig.dest.stream.getAudioTracks() : [])]);
  const chunks = [];
  const mr = new MediaRecorder(stream, { mimeType: mime, videoBitsPerSecond: 3_500_000, audioBitsPerSecond: 128_000 });
  mr.ondataavailable = e => { if (e.data && e.data.size) chunks.push(e.data); };
  const stopped = new Promise(res => { mr.onstop = res; });

  const voice = async key => { if (!key || !rig || !fetchVoice) return null; try { return await fetchVoice(key); } catch (e) { return null; } };
  try {
    view = await makeView(m.game, vc, { lang: 'en', fetchMatch: after => fetchMatch(after), onEvent: e => {
      if (!e) return;
      shownN++;
      if (Array.isArray(e.score)) { score = m.game === 'nfl' ? prevScore : e.score; prevScore = e.score; }
      if (stage !== 'play') return;
      const now = performance.now(), mo = moments.get(e.n);
      if (slowFrom.has(e.n) && !mo) { setSpeed(slow); slowUntil = Infinity; }
      if (mo) {
        setSpeed(slow); slowUntil = now + 6500;
        card = { mo, t0: now, until: now + 6500 };
        rig && rig.sting();
        const l = lines.get(e.n);
        if (l) { cap = { text: l.text, until: now + 7000 }; voice(l.audio).then(b => rig && rig.say(b)); }
      } else if (e.hl && rig && speed <= slow) rig.roar(/touchdown|ace|baron|nexus|win/i.test(e.text || ''));
    } });
  } catch (e) { ui.remove(); throw e; }
  if (!view.paused) view.togglePause();

  // ---- pintar ----
  const nm = s => m.names[s] || '';
  const fol = m.followed || [0, 0];
  const coach = s => { const f = (m.feedback || [])[s]; return f && f.scores ? Object.values(f.scores).reduce((a, b) => a + (+b || 0), 0) : null; };
  function scoreBar() {
    const y = H - 62;
    g.fillStyle = 'rgba(6,8,18,.86)'; rrect(g, 24, y, W - 48, 44, 10); g.fill();
    g.font = FONT(800, 22); g.textBaseline = 'middle';
    const [l, r] = LR, sc = `${score[l]} - ${score[r]}`;
    g.fillStyle = sideColor(m, l); g.fillRect(24, y, 8, 44); g.fillStyle = sideColor(m, r); g.fillRect(W - 32, y, 8, 44);
    g.fillStyle = '#f3f5fb'; g.textAlign = 'left'; g.fillText(nm(l), 46, y + 22); g.textAlign = 'right'; g.fillText(nm(r), W - 46, y + 22);
    g.textAlign = 'center'; g.font = FONT(900, 26); g.fillStyle = '#ffd400'; g.fillText(sc, W / 2, y + 22);
    g.font = FONT(700, 14); g.fillStyle = '#9aa4bd'; g.textAlign = 'left';
    g.fillText(`GREGAL ARENA · ${GAME[m.game] || m.game}${speed > slow ? `   ▶▶ ${speed}×` : ''}`, 24, 26);
    g.textBaseline = 'alphabetic';
  }
  function momentCard(now) {
    if (!card || now > card.until) return;
    const a = Math.min(1, (now - card.t0) / 300, (card.until - now) / 400), mo = card.mo, x = 120, w = W - 240;
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
  function caption(now) {
    if (!cap || now > cap.until) return;
    g.font = FONT(600, 21); const ls = wrap(g, cap.text, W - 300).slice(0, 2), h = ls.length * 28 + 20, y = H - 80 - h;
    g.fillStyle = 'rgba(8,10,22,.88)'; rrect(g, 150, y, W - 300, h, 10); g.fill(); g.fillStyle = '#ffd400'; g.fillRect(150, y, 5, h);
    g.fillStyle = '#f3f5fb'; ls.forEach((l, i) => g.fillText(l, 170, y + 32 + i * 28));
  }
  function bg() { const gr = g.createRadialGradient(W / 2, -200, 100, W / 2, 0, 1100); gr.addColorStop(0, '#1d2a5c'); gr.addColorStop(1, '#070a14'); g.fillStyle = gr; g.fillRect(0, 0, W, H);
    // Una franja que es mou: els rètols fixos també han de fer fotogrames (si no, el vídeo hi deixa forats).
    const p = (performance.now() / 4000) % 1; g.fillStyle = '#ffd400'; g.fillRect(0, H - 6, W * p, 6); }
  function intro() {
    bg(); g.textAlign = 'center';
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
  function summary() {
    bg();
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

  let raf = 0;
  const frame = () => {
    const now = performance.now();
    if (stage === 'intro') intro();
    else if (stage === 'summary') summary();
    else {
      g.fillStyle = '#000'; g.fillRect(0, 0, W, H);
      try { g.drawImage(vc, 0, 0, W, H); } catch (e) { /* encara no */ }
      if (speed === slow && now > slowUntil && !cancelled) setSpeed(fast);
      scoreBar(); momentCard(now); caption(now);
    }
    raf = requestAnimationFrame(frame);
  };
  frame();
  // El primer dibuix del visor (l'estadi, el mapa) bloqueja una estona: es
  // grava quan ja va fi, que si no el vídeo comença amb un forat.
  status('Preparing…');
  for (let ok = 0, t = performance.now(), end = t + 10000; (ok < 15 || !view.match) && t < end;) {
    const n = await new Promise(r => requestAnimationFrame(r));
    ok = n - t < 60 ? ok + 1 : 0; t = n;
  }
  mr.start(1000);
  const deadline = Date.now() + 20 * 60 * 1000;
  try {
    status('Intro'); await sleep(4500);
    if (cancelled) throw new Error('cancelled');
    stage = 'play'; setSpeed(fast); view.togglePause();
    while (!view.isFinished() && !cancelled && Date.now() < deadline) {
      bar(evs.length ? shownN / evs.length * .9 : .5);
      status(`Recording the match · ${Math.min(shownN, evs.length)}/${evs.length} plays${speed > slow ? ` · ${speed}×` : ' · prompt moment'}`);
      await sleep(400);
    }
    if (cancelled) throw new Error('cancelled');
    await sleep(1500);
    stage = 'summary'; status('Match summary'); bar(.95); rig && rig.sting();
    const a = m.analysis || {};
    const said = a.audio ? voice(a.audio).then(b => rig && rig.say(b)) : null;
    await Promise.race([Promise.all([sleep(9000), said || Promise.resolve()]), sleep(30000)]);
  } catch (e) {
    cancelled = true;
  } finally {
    view.destroy();
  }
  mr.stop(); await stopped;
  cancelAnimationFrame(raf);
  if (rig) rig.ctx.close().catch(() => {});
  stream.getTracks().forEach(t => t.stop());
  ui.remove();
  if (cancelled) return null;
  const blob = new Blob(chunks, { type: mime.split(';')[0] });
  const url = URL.createObjectURL(blob), link = document.createElement('a');
  link.href = url; link.download = fname; document.body.append(link); link.click(); link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 60000);
  return { name: fname, size: blob.size, type: blob.type, blob };
}
