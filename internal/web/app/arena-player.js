// El reproductor de partits de l'Arena (arena.js): la pista de comentaris amb
// el minut de cada moment (com els capítols d'un vídeo), els subtítols i el
// resum del final dins del reproductor. Aquí hi ha les peces pures (es proven
// sense DOM: arena-player.test.cjs) i el HTML del resum.
import { fmtTime } from './arena-scrubber.js';

const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

// Una decisió en paraules: els valors del JSON, sense guions baixos.
export function callText(d) {
  if (!d) return '';
  const v = d.decision && typeof d.decision === 'object' ? Object.values(d.decision).flat().filter(x => typeof x === 'string' && x && x !== 'normal') : [];
  return (v.length ? v.join(' · ') : (d.short || '')).replace(/_/g, ' ');
}

// La pista: una línia per moment que compta (moments del prompt ★, punts,
// jugades grosses i les frases del narrador), amb el segon de reproducció, què
// ha passat i, quan toca, la crida de l'equip i la regla del .md.
//   events: la llista del visor (getProgress().events), times: els seus inicis
//   (segons a 1×), lines: Map n → {text}, decisions: [{n, side, decision, rule, …}],
//   moments: Map n de l'esdeveniment → {side, rule, what, why}.
export function buildTrack({ events = [], times = [], lines = new Map(), decisions = [], moments = new Map() } = {}) {
  const dec = new Map();
  for (const d of decisions || []) dec.set(d.n + ':' + d.side, d);
  const out = [];
  let prev = [0, 0];
  events.forEach((e, i) => {
    const sc = Array.isArray(e.score) ? e.score : null;
    let scored = -1;
    if (sc) {
      const up0 = sc[0] > prev[0], up1 = sc[1] > prev[1];
      scored = up0 && !up1 ? 0 : up1 && !up0 ? 1 : up0 && up1 ? 2 : -1;
      prev = sc;
    }
    const mo = moments.get(e.n), line = lines.get(e.n);
    const kind = mo ? 'moment' : scored >= 0 ? 'score' : e.hl ? 'hl' : line ? 'line' : '';
    if (!kind) return;
    const side = mo ? mo.side : scored === 0 || scored === 1 ? scored : -1;
    const d = side >= 0 && e.decision != null ? dec.get(e.decision + ':' + side) : null;
    out.push({
      i, n: e.n, t: +times[i] || 0, kind, side,
      text: e.text || String(e.kind || '').replace(/_/g, ' '),
      caster: line ? line.text : '',
      score: sc ? sc.slice() : null,
      call: callText(d), rule: mo ? mo.rule : (d && d.rule) || '',
      what: mo ? mo.what : '', why: mo ? mo.why : '',
    });
  });
  return out;
}

// Filtre de la pista: 'all', 'key' (sense les frases soles del narrador) o 'prompt' (★).
export function filterTrack(entries, mode) {
  if (mode === 'prompt') return entries.filter(e => e.kind === 'moment');
  if (mode === 'key') return entries.filter(e => e.kind !== 'line');
  return entries;
}

// La línia que es veu ara: l'última que ha començat (index = esdeveniments començats).
export function activeEntry(entries, index) {
  let a = null;
  for (const e of entries) { if (e.i <= index - 1) a = e; else break; }
  return a;
}

// El que diu el subtítol d'una línia: la frase del narrador o, si no, què ha passat.
export function captionOf(e) { return e ? (e.caster || (e.kind === 'moment' ? e.what || e.text : e.text)) : ''; }

const ICON = { moment: '★', score: '●', hl: '⚡', line: '🎙' };

// El HTML d'una línia de la pista.
export function trackItemHTML(e, { teamName = s => '', sideColor = () => '' } = {}) {
  // El nom de l'equip, si el text no el porta ja.
  const tn = e.side >= 0 && e.side <= 1 ? teamName(e.side) : '', said = e.kind === 'moment' && e.what ? e.what : e.text;
  const who = tn && !String(said).includes(tn) ? `<b style="color:${esc(sideColor(e.side))}">${esc(tn)}</b> ` : '';
  const sc = e.score && (e.kind === 'score' || e.kind === 'moment') ? ` <span class="sc">${e.score[0]}-${e.score[1]}</span>` : '';
  return `<li class="ar-tk ${e.kind}" data-i="${e.i}"><time>${fmtTime(e.t)}</time><span class="ic">${ICON[e.kind] || '•'}</span><div>` +
    `<div class="tx">${who}${esc(said)}${sc}</div>` +
    (e.caster && e.kind !== 'line' ? `<div class="cs">🎙 ${esc(e.caster)}</div>` : e.kind === 'line' ? `<div class="cs">🎙 ${esc(e.caster)}</div>` : '') +
    (e.call ? `<div class="cl"><code>${esc(e.call)}</code></div>` : '') +
    (e.rule ? `<q>${esc(e.rule)}</q>` : '') + `</div></li>`;
}

// El resum del final dins del reproductor (el mateix que la tele): resultat,
// l'anàlisi, la regla que ho va decidir, els moments del prompt (clicables) i,
// per equip, la nota del coach i quant ha seguit el .md.
export function summaryHTML(m, { T = k => k, sideColor = () => '', momentIndex = () => null, times = [] } = {}) {
  const names = m.names || ['', ''], an = m.analysis || {}, mos = an.moments || [];
  const flip = m.game === 'lol' && m.meta && m.meta.blue === 1;
  const [l, r] = flip ? [1, 0] : [0, 1];
  const sc = m.score || [0, 0];
  const coach = s => { const f = (m.feedback || [])[s]; return f && f.scores ? Object.values(f.scores).reduce((x, y) => x + (+y || 0), 0) + '/25' : '—'; };
  const fol = m.followed || [0, 0];
  const w = m.winner === 0 || m.winner === 1 ? names[m.winner] : '';
  return `<div class="ar-end-box"><div class="k">${esc(T('ar.vp.summary'))}</div>
    <div class="res">${esc(names[l])} <b>${sc[l]} - ${sc[r]}</b> ${esc(names[r])}</div>
    <div class="win">${w ? '🏆 ' + esc(T('ar.vp.wins').replace('{name}', w)) : esc(T('ar.vp.draw'))}</div>
    <div class="acts"><button type="button" data-end="again" class="primary">${esc(T('ar.vp.again'))}</button><button type="button" data-end="download">${esc(T('ar.vp.download'))}</button><button type="button" data-end="close">${esc(T('ar.vp.close'))}</button></div>
    <div class="cols"><div>
      ${an.headline ? `<div class="quote"><small>${esc(T('ar.vp.analysis'))}</small><b>${esc(an.headline)}</b> ${esc(an.text || '')}</div>` : ''}
      ${an.rule ? `<div class="k2">${esc(T('ar.vp.keyRule'))} · ${esc(names[an.side === 1 ? 1 : 0])}</div><div class="rule">«${esc(an.rule)}»</div>` : ''}
      <div class="coach">${[0, 1].map(s => `<span><b style="color:${esc(sideColor(s))}">${esc(names[s])}</b> ${esc(T('ar.vp.coach'))} ${esc(coach(s))} · ${esc(T('ar.vp.followed'))} ${Math.round((fol[s] || 0) * 100)}%</span>`).join('')}</div>
    </div><div><div class="k2">★ ${esc(T('ar.vp.moments'))}</div><ol class="mos">${mos.map(mo => {
      const i = momentIndex(mo);
      return `<li${i != null ? ` data-seek="${i}"` : ''} style="--mc:${esc(sideColor(mo.side))}">${i != null && times[i] != null ? `<time>${fmtTime(times[i])}</time>` : ''}<b>${esc(names[mo.side] || '')}</b> ${esc(mo.what || '')}<q>«${esc(mo.rule || '')}»</q></li>`;
    }).join('') || `<li class="none">${esc(T('ar.vp.noMoments'))}</li>`}</ol></div></div>
</div>`;
}

let cssDone = false;
export function playerCSS() {
  if (cssDone || typeof document === 'undefined' || document.getElementById('arPlayerCSS')) return;
  cssDone = true;
  const st = document.createElement('style'); st.id = 'arPlayerCSS';
  st.textContent = `
.ar-player .ar-stage { position:relative; }
.ar-cc { position:absolute; left:50%; bottom:12px; transform:translate(-50%, 8px); max-width:86%; background:rgba(0,0,0,.78); color:#fff; padding:6px 14px; border-radius:8px; font:600 15px/1.35 "Segoe UI", system-ui, sans-serif; text-align:center; opacity:0; transition:opacity .3s, transform .3s; pointer-events:none; z-index:6; }
.ar-cc.on { opacity:1; transform:translate(-50%, 0); }
.ar-end { position:absolute; inset:0; z-index:7; background:rgba(5,7,15,.88); color:#f3f5fb; overflow:auto; display:flex; align-items:flex-start; justify-content:center; padding:14px; font:14px/1.4 "Segoe UI", system-ui, sans-serif; }
.ar-end[hidden] { display:none; }
.ar-end-box { margin:auto 0; width:min(100%, 880px); display:flex; flex-direction:column; gap:8px; }
.ar-end .k, .ar-end .k2 { color:#ffd400; font-weight:800; letter-spacing:.14em; text-transform:uppercase; font-size:12px; }
.ar-end .res { font-size:clamp(20px, 3vw, 32px); font-weight:900; } .ar-end .res b { color:#ffd400; }
.ar-end .win { font-weight:800; color:#ffd400; }
.ar-end .cols { display:grid; grid-template-columns:minmax(0,1.1fr) minmax(0,1fr); gap:16px; align-items:start; }
.ar-player .ar-stage { container-type:inline-size; }
@container (max-width:620px) { .ar-end .cols { grid-template-columns:minmax(0,1fr); } .ar-end .quote { -webkit-line-clamp:3; } }
.ar-end .quote { background:rgba(255,212,0,.08); border-left:4px solid #ffd400; border-radius:8px; padding:8px 12px; margin-bottom:8px; display:-webkit-box; -webkit-line-clamp:6; -webkit-box-orient:vertical; overflow:hidden; }
.ar-end .quote small { display:block; color:#ffd400; font-weight:800; letter-spacing:.1em; text-transform:uppercase; font-size:11px; }
.ar-end .rule { font-family:ui-monospace, Consolas, monospace; background:rgba(255,255,255,.07); border-radius:8px; padding:6px 10px; margin:4px 0 8px; font-size:13px; }
.ar-end .coach { display:flex; flex-direction:column; gap:3px; font-size:13px; }
.ar-end .mos { list-style:none; margin:6px 0 0; padding:0; display:flex; flex-direction:column; gap:6px; }
.ar-end .mos li { background:rgba(255,255,255,.05); border-left:4px solid var(--mc); border-radius:8px; padding:6px 10px; font-size:13px; }
.ar-end .mos li[data-seek] { cursor:pointer; } .ar-end .mos li[data-seek]:hover { background:rgba(255,255,255,.1); }
.ar-end .mos time { color:#9aa4bd; margin-right:6px; font-variant-numeric:tabular-nums; } .ar-end .mos b { color:var(--mc); }
.ar-end .mos q { quotes:none; display:block; color:#9aa4bd; font-family:ui-monospace, Consolas, monospace; font-size:12px; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
.ar-end .acts { display:flex; gap:8px; flex-wrap:wrap; margin-top:6px; }
.ar-end .acts button { font:inherit; font-weight:700; border-radius:9px; padding:8px 16px; cursor:pointer; border:1px solid rgba(255,255,255,.2); background:#1a2138; color:#f3f5fb; }
.ar-end .acts button.primary { background:#ffd400; color:#111; border-color:#ffd400; }
.ar-side { display:flex; flex-direction:column; min-width:0; min-height:0; border:1px solid var(--line, rgba(127,127,127,.25)); border-radius:12px; overflow:hidden; }
.ar-player > .ar-side { height:0; min-height:100%; }
@media (max-width:1100px) { .ar-player > .ar-side { height:440px; min-height:0; } }
.ar-trackpane { display:flex; flex-direction:column; min-height:0; flex:1; }
.ar-side .ar-decisions { max-height:none; flex:1; min-height:0; padding:8px; }
.ar-decisions[hidden], .ar-trackpane[hidden] { display:none !important; }
.ar-tabs { display:flex; border-bottom:1px solid var(--line, rgba(127,127,127,.25)); }
.ar-tabs button { flex:1; border:0; background:none; color:inherit; font:inherit; font-weight:700; padding:8px 10px; cursor:pointer; opacity:.65; border-bottom:2px solid transparent; }
.ar-tabs button.on { opacity:1; border-bottom-color:var(--accent, #ffd400); }
.ar-filter { display:flex; gap:4px; padding:6px 8px; flex-wrap:wrap; }
.ar-filter button { font:inherit; font-size:12px; border:1px solid var(--line-hi, rgba(127,127,127,.35)); background:none; color:inherit; border-radius:99px; padding:2px 10px; cursor:pointer; }
.ar-filter button.on { background:var(--accent, #ffd400); border-color:var(--accent, #ffd400); color:#fff; }
.ar-track { list-style:none; margin:0; padding:0 4px 8px; overflow:auto; flex:1; min-height:0; }
.ar-tk { display:grid; grid-template-columns:44px 18px minmax(0,1fr); gap:4px; padding:6px 6px; border-radius:8px; cursor:pointer; font-size:13px; line-height:1.35; }
.ar-tk:hover { background:rgba(127,127,127,.12); }
.ar-tk.now { background:rgba(255,212,0,.16); box-shadow:inset 3px 0 0 #ffd400; }
.ar-tk.future { opacity:.55; }
.ar-tk time { font-variant-numeric:tabular-nums; opacity:.7; padding-top:1px; }
.ar-tk .ic { text-align:center; } .ar-tk.moment .ic { color:#e0a800; } .ar-tk.score .ic { color:#2fbf71; }
.ar-tk .sc { font-weight:800; margin-left:4px; }
.ar-tk .cs { opacity:.8; font-style:italic; margin-top:2px; }
.ar-tk .cl code { font-size:12px; }
.ar-tk q { display:block; opacity:.75; font-family:ui-monospace, Consolas, monospace; font-size:12px; margin-top:2px; }
.ar-track .none { padding:12px; opacity:.7; font-size:13px; }
.ar-scrub .x { display:flex; gap:4px; align-items:center; flex-wrap:wrap; }
.ar-scrub button.dl { background:#ffd400; color:#111; border-color:#ffd400; font-weight:800; }
.ar-scrub button.dl.wait { background:rgba(255,255,255,.07); color:#9aa4bd; border-color:rgba(255,255,255,.14); cursor:help; }
.ar-scrub button.cc.on { background:#f3f5fb; color:#111; }`;
  document.head.append(st);
}
