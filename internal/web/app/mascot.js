// El penell de Gregal (internament, «mascota»): petit, al peu de la barra
// lateral (apagat per defecte). Reacciona a l'estat (assenyala el NE en
// repòs, gira a ràfegues treballant, apunta cap al camp de text quan cal
// aprovar, torna al NE en acabar, s'encalla si falla) i, clicant-lo, obre un
// xat ràpid que envia el missatge a la sessió actual. S'activa a
// Preferències; amb «reduir moviment» queda quiet i només canvia el color.
import './fun-core.js';

const fun = window.gregalFunCore;
const T = key => (window.gregalT && window.gregalT(key)) || key;
const KEY_ON = 'gregal_fun_mascot';
const MOOD_KEYS = { idle: 'mascot.idle', working: 'mascot.working', waiting: 'mascot.waiting', done: 'mascot.done', error: 'mascot.error' };

// Apagada per defecte: només surt si l'activen a Preferències → Extres
// (també qui no hi ha tocat mai res).
export const mascotEnabled = () => { try { return localStorage.getItem(KEY_ON) === '1'; } catch (e) { return false; } };

// El penell: pal i creu (N E S W) quiets i una fletxa que gira sobre el
// pivot (32,20). L'angle de la fletxa és el rumb (0° = N, 45° = NE) i el posa
// fun.css segons l'ànim (--vane). El mateix dibuix és a desktop/mascot.html.
const SVG = `<svg class="mascot-svg" viewBox="0 0 64 64" role="img" aria-label="Gregal" focusable="false">
<g class="v-streaks"><path d="M4 9h11"/><path d="M1 17h8"/><path d="M6 30h9"/></g>
<g class="v-stand">
<path class="v-cross" d="M19 48H45M38 43.5L26 52.5"/>
<path class="v-post" d="M32 21V58"/>
<path class="v-base" d="M25 62h14l-2.5-4h-9z"/>
<text class="v-letter" x="13.5" y="48">W</text>
<text class="v-letter" x="50.5" y="48">E</text>
<text class="v-letter" x="41.5" y="40.5">N</text>
<text class="v-letter" x="22.5" y="56">S</text>
</g>
<g class="v-arrow">
<path class="v-shaft" d="M32 35V7"/>
<path class="v-curl" d="M32 31c4 0 6-2.3 5.6-4.8c-.3-2.1-3.1-2.6-4-.8"/>
<path class="v-tail" d="M32 31.5l5.2 4.3V41l-5.2-3.2l-5.2 3.2v-5.2z"/>
<path class="v-head" d="M32 1l5 9.5l-5-2.2l-5 2.2z"/>
<circle class="v-ring" cx="32" cy="4" r="4"/>
<path class="v-spark" d="M39 0v3M37.5 1.5h3M24.5 3v2.6M23.2 4.3h2.6"/>
</g>
<circle class="v-hub" cx="32" cy="20" r="2.4"/>
</svg>`;

const st = { wrap: null, btn: null, pop: null, mood: '', sent: '', wasRunning: false, doneUntil: 0, errorUntil: 0, timer: 0 };

function sense(now) {
  const groups = document.querySelectorAll('#main .work-group');
  const last = groups[groups.length - 1];
  const timer = document.getElementById('turnTimer');
  const running = !!document.querySelector('#main .work-group.running') || !!(timer && timer.textContent.trim() && !document.getElementById('turnBar')?.hidden);
  if (st.wasRunning && !running) {
    if (last && last.classList.contains('failed')) st.errorUntil = now + 8000;
    else st.doneUntil = now + 4500;
  }
  st.wasRunning = running;
  const pend = window.gregalPendingInteractions;
  return fun.mascotMood({ waiting: !!(pend && pend.size > 0), working: running, error: now < st.errorUntil, done: now < st.doneUntil });
}

// A l'app d'escriptori, la mascota flotant (desktop/mascot.html) fa de
// mirall d'aquest estat: se li envia cada canvi, encara que la de la barra
// lateral estigui amagada. El procés principal la descarta si no és oberta.
function emit(mood) {
  if (mood === st.sent) return;
  const bridge = window.gregalDesktop;
  if (!bridge || typeof bridge.mascotState !== 'function') return;
  st.sent = mood;
  try { bridge.mascotState(mood, T(MOOD_KEYS[mood])); } catch (e) {}
}

function paint() {
  if (!st.wrap) return;
  const mood = sense(Date.now());
  emit(mood);
  const on = mascotEnabled();
  st.wrap.hidden = !on;
  if (!on) { closePop(); st.mood = ''; return; }
  if (mood === st.mood) return;
  st.mood = mood;
  st.btn.dataset.mood = mood;
  const label = T(MOOD_KEYS[mood]) + ' — ' + T('mascot.click');
  st.btn.setAttribute('aria-label', label);
  st.btn.title = label;
  const svg = st.btn.querySelector('.mascot-svg');
  if (svg) svg.setAttribute('aria-label', T(MOOD_KEYS[mood]));
}

function send(text) {
  const input = document.getElementById('in');
  const go = document.getElementById('send');
  if (!input || !go || !text.trim()) return false;
  // L'esborrany que tinguis a mig escriure no es perd: el torn llegeix el
  // camp en sincronia i després es torna a posar.
  try { window.gregal?.setView?.('agent'); } catch (e) {}
  const draft = input.value;
  input.value = text.trim();
  input.dispatchEvent(new Event('input'));
  go.click();
  if (draft && !input.value) { input.value = draft; input.dispatchEvent(new Event('input')); }
  return true;
}

// Apaga el penell de la barra lateral i avisa qui escolti (Preferències).
export function hideMascot() {
  try { localStorage.setItem(KEY_ON, '0'); } catch (e) {}
  closePop(false);
  document.dispatchEvent(new CustomEvent('gregal:fun-prefs'));
}

function closePop(focusBack) {
  if (st.pop) st.pop.hidden = true;
  if (st.btn) st.btn.setAttribute('aria-expanded', 'false');
  if (focusBack && st.btn) st.btn.focus();
}

function openPop() {
  if (!st.pop) {
    const pop = document.createElement('form');
    pop.className = 'mascot-pop';
    pop.setAttribute('role', 'dialog');
    pop.setAttribute('aria-label', T('mascot.chat'));
    pop.innerHTML = '<label class="mascot-pop-lab"></label><textarea rows="2" maxlength="4000"></textarea>' +
      '<div class="mascot-pop-acts"><span class="mascot-pop-hint"></span><button type="button" class="ghost mascot-pop-hide"></button><button type="submit" class="primary"></button></div>';
    const ta = pop.querySelector('textarea');
    ta.id = 'mascotChatText';
    pop.querySelector('label').htmlFor = ta.id;
    pop.addEventListener('submit', e => {
      e.preventDefault();
      if (send(ta.value)) { ta.value = ''; closePop(false); }
    });
    // «Amaga»: apaga el penell de la barra (es torna a encendre a
    // Preferències → Extres).
    pop.querySelector('.mascot-pop-hide').addEventListener('click', e => { e.preventDefault(); hideMascot(); });
    ta.addEventListener('keydown', e => {
      if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) { e.preventDefault(); pop.requestSubmit(); }
      if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); closePop(true); }
    });
    document.addEventListener('click', e => { if (!pop.hidden && !pop.contains(e.target) && !st.btn.contains(e.target)) closePop(false); });
    st.wrap.appendChild(pop);
    st.pop = pop;
  }
  st.pop.querySelector('label').textContent = T('mascot.chat');
  st.pop.querySelector('textarea').placeholder = T('mascot.placeholder');
  st.pop.querySelector('.mascot-pop-hint').textContent = T('mascot.hint');
  st.pop.querySelector('button[type="submit"]').textContent = T('mascot.send');
  const hide = st.pop.querySelector('.mascot-pop-hide');
  hide.textContent = T('mascot.hide');
  hide.title = T('mascot.hideHelp');
  st.pop.hidden = false;
  st.btn.setAttribute('aria-expanded', 'true');
  st.pop.querySelector('textarea').focus();
}

export function boot() {
  const foot = document.querySelector('#side .side-foot');
  if (!foot || st.wrap) return;
  const wrap = document.createElement('div');
  wrap.className = 'mascot-wrap';
  const btn = document.createElement('button');
  btn.type = 'button';
  btn.className = 'mascot-btn';
  btn.setAttribute('aria-haspopup', 'dialog');
  btn.setAttribute('aria-expanded', 'false');
  btn.innerHTML = SVG + '<span class="mascot-name">Gregal</span>';
  btn.addEventListener('click', e => { e.stopPropagation(); st.pop && !st.pop.hidden ? closePop(false) : openPop(); });
  // Clic dret: el mateix xat ràpid, amb el focus a «Amaga».
  btn.addEventListener('contextmenu', e => { e.preventDefault(); e.stopPropagation(); openPop(); st.pop.querySelector('.mascot-pop-hide')?.focus(); });
  wrap.appendChild(btn);
  foot.insertBefore(wrap, foot.firstChild);
  st.wrap = wrap; st.btn = btn;
  paint();
  st.timer = setInterval(paint, 800);
  document.addEventListener('gregal:fun-prefs', () => { st.mood = ''; st.sent = ''; paint(); });
  document.addEventListener('gregal:idioma', () => { st.mood = ''; st.sent = ''; paint(); if (st.pop && !st.pop.hidden) openPop(); });
}
