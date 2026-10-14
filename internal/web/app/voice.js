// Composer: dictat per veu (Web Speech API) i imatges enganxades o
// arrossegades. La veu surt només si el navegador la suporta: mantenir
// premut = parlar mentre es prem; un clic curt (o Enter/Espai) l'encén fins
// que es torna a clicar. El text provisional es veu en directe al camp.
// Les imatges passen pel mateix flux que el clip (#imgPick), amb els seus
// límits (3 imatges, 8 MB cadascuna) i els seus missatges.
import './fun-core.js';

const fun = window.gregalFunCore;
const T = key => (window.gregalT && window.gregalT(key)) || key;
const KEY_ON = 'gregal_fun_voice';
const HOLD_MS = 280;

export const voiceEnabled = () => { try { return localStorage.getItem(KEY_ON) !== '0'; } catch (e) { return true; } };
const Recognition = () => (typeof window !== 'undefined' && (window.SpeechRecognition || window.webkitSpeechRecognition)) || null;

const MIC = '<svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' +
  '<rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5 11a7 7 0 0 0 14 0M12 18v3"/></svg>';

const st = { btn: null, rec: null, listening: false, base: '', final: '', downAt: 0, held: false };

function input() { return document.getElementById('in'); }
function sys(text) { try { window.gregal?.sys?.(text); } catch (e) {} }

function paintBtn() {
  const b = st.btn;
  if (!b) return;
  b.hidden = !Recognition() || !voiceEnabled();
  b.classList.toggle('is-listening', st.listening);
  b.setAttribute('aria-pressed', String(st.listening));
  const label = T(st.listening ? 'voice.stop' : 'voice.start');
  b.setAttribute('aria-label', label);
  b.title = label + ' — ' + T('voice.hint');
}

function writeField(interim) {
  const el = input();
  if (!el) return;
  el.value = fun.dictation(st.base, st.final, interim);
  el.dispatchEvent(new Event('input'));
}

function start() {
  const R = Recognition();
  if (!R || st.listening) return;
  const el = input();
  if (!el) return;
  const rec = new R();
  rec.lang = fun.speechLang(document.documentElement.lang || 'en');
  rec.interimResults = true;
  rec.continuous = true;
  st.base = el.value; st.final = '';
  rec.onresult = e => {
    let interim = '';
    for (let i = e.resultIndex; i < e.results.length; i++) {
      const r = e.results[i];
      if (r.isFinal) st.final = (st.final ? st.final + ' ' : '') + r[0].transcript.trim();
      else interim += r[0].transcript;
    }
    writeField(interim);
  };
  rec.onerror = e => {
    if (e && (e.error === 'not-allowed' || e.error === 'service-not-allowed')) sys(T('voice.denied'));
    else if (e && e.error && e.error !== 'aborted' && e.error !== 'no-speech') sys(T('voice.error') + ' ' + e.error);
  };
  rec.onend = () => {
    st.listening = false; st.rec = null;
    writeField('');
    paintBtn();
    const f = input(); if (f) f.focus();
  };
  try { rec.start(); } catch (e) { sys(T('voice.error') + ' ' + e.message); return; }
  st.rec = rec; st.listening = true;
  paintBtn();
}

function stop() {
  if (st.rec) { try { st.rec.stop(); } catch (e) {} }
}

function wireMic() {
  const attach = document.getElementById('attach');
  if (!attach || st.btn) return;
  const b = document.createElement('button');
  b.type = 'button';
  b.id = 'micBtn';
  b.className = 'icon-btn mic-btn';
  b.innerHTML = MIC;
  attach.parentNode.insertBefore(b, attach);
  st.btn = b;
  // Mantenir premut = parlar; clic curt = commutar.
  b.addEventListener('pointerdown', e => {
    if (e.button !== 0) return;
    st.downAt = Date.now(); st.held = false;
    if (!st.listening) { start(); st.held = true; }
  });
  const release = () => {
    if (!st.downAt) return;
    const long = Date.now() - st.downAt > HOLD_MS;
    st.downAt = 0;
    if (st.held && long) stop();          // push-to-talk acabat
    else if (!st.held && st.listening) stop(); // segon clic: atura
  };
  b.addEventListener('pointerup', release);
  b.addEventListener('pointerleave', () => { if (st.downAt && st.held && Date.now() - st.downAt > HOLD_MS) release(); });
  b.addEventListener('keydown', e => {
    if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); st.listening ? stop() : start(); }
    if (e.key === 'Escape' && st.listening) { e.preventDefault(); e.stopPropagation(); stop(); }
  });
  paintBtn();
}

// --- imatges: enganxar i arrossegar ---

function feed(files) {
  const pick = document.getElementById('imgPick');
  if (!pick || !files.length) return false;
  try {
    const dt = new DataTransfer();
    files.forEach(f => dt.items.add(f));
    pick.files = dt.files;
  } catch (e) { sys(T('voice.imgUnsupported')); return false; }
  pick.dispatchEvent(new Event('change'));
  return true;
}

function wireImages() {
  const field = input();
  const composer = document.getElementById('composer');
  if (field) field.addEventListener('paste', e => {
    const cd = e.clipboardData;
    if (!cd) return;
    const files = fun.imageFiles(cd.files && cd.files.length ? cd.files : [...(cd.items || [])].filter(i => i.kind === 'file').map(i => i.getAsFile()).filter(Boolean));
    if (!files.length) return;
    // Una captura de pantalla no porta text: llavors no s'enganxa res més.
    const hasText = [...(cd.types || [])].includes('text/plain');
    if (!hasText) e.preventDefault();
    feed(files);
  });
  if (!composer) return;
  const hasFiles = e => [...((e.dataTransfer && e.dataTransfer.types) || [])].includes('Files');
  let depth = 0;
  composer.addEventListener('dragenter', e => { if (!hasFiles(e)) return; e.preventDefault(); depth++; composer.classList.add('drop-over'); });
  composer.addEventListener('dragover', e => { if (!hasFiles(e)) return; e.preventDefault(); e.dataTransfer.dropEffect = 'copy'; });
  composer.addEventListener('dragleave', e => { if (!hasFiles(e)) return; depth = Math.max(0, depth - 1); if (!depth) composer.classList.remove('drop-over'); });
  composer.addEventListener('drop', e => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    depth = 0; composer.classList.remove('drop-over');
    const files = fun.imageFiles(e.dataTransfer.files);
    if (!files.length) { sys(T('voice.imgOnly')); return; }
    feed(files);
  });
}

export function boot() {
  wireMic();
  wireImages();
  document.addEventListener('gregal:fun-prefs', () => { if (!voiceEnabled()) stop(); paintBtn(); });
  document.addEventListener('gregal:idioma', paintBtn);
}
