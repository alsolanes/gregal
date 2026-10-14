// Secció «Extres» de Preferències: tot el que és diversió es pot apagar.
// Viu al navegador (localStorage), com la resta d'aparença; el to de #retro
// per defecte, si no se'n tria cap, és el del config (retro.persona).
import { t as T } from './i18n.js';

const KEYS = { mascot: 'gregal_fun_mascot', wrapped: 'gregal_fun_wrapped', voice: 'gregal_fun_voice', desktopMascot: 'gregal_desktop_mascot' };
// La mascota flotant només existeix a l'app d'escriptori (desktop/main.js):
// el preload n'anuncia el suport i la preferència viatja per savePreference.
const hasDesktopMascot = () => { try { return !!(window.gregalDesktop && window.gregalDesktop.mascotWindow); } catch (e) { return false; } };
export const PERSONA_KEY = 'gregal_retro_persona';
export const PERSONAS = [['neutral', 'retro.persona.neutral'], ['friendly', 'retro.persona.friendly'], ['sarcastic', 'retro.persona.sarcastic'], ['mentor', 'retro.persona.mentor']];

const get = (k, d) => { try { const v = localStorage.getItem(k); return v == null ? d : v; } catch (e) { return d; } };
const set = (k, v) => { try { localStorage.setItem(k, v); } catch (e) {} };
const esc = s => String(s).replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

export function retroPersonaPref() { return get(PERSONA_KEY, ''); }

export const funPrefs = {
  section() {
    const row = (title, help, inner) => '<div class="pf-row"><div class="pf-lab"><b>' + esc(T(title)) + '</b><span>' + esc(T(help)) + '</span></div>' + inner + '</div>';
    const check = (id, on) => '<label class="pf-check"><input type="checkbox" id="' + id + '"' + (on ? ' checked' : '') + '><span></span></label>';
    const persona = retroPersonaPref();
    return '<h3 class="pf-sec">' + esc(T('fun.prefs')) + '</h3>' +
      row('fun.mascot', 'fun.mascot.help', check('prefsFunMascot', get(KEYS.mascot, '0') === '1')) +
      (hasDesktopMascot() ? row('fun.desktopMascot', 'fun.desktopMascot.help', check('prefsDesktopMascot', get(KEYS.desktopMascot, '0') === '1')) : '') +
      row('fun.wrapped', 'fun.wrapped.help', check('prefsFunWrapped', get(KEYS.wrapped, '1') !== '0')) +
      row('fun.voice', 'fun.voice.help', check('prefsFunVoice', get(KEYS.voice, '1') !== '0')) +
      row('retro.persona', 'retro.persona.help',
        '<select class="pf-select" id="prefsRetroPersona" aria-label="' + esc(T('retro.persona')) + '">' +
        '<option value=""' + (persona ? '' : ' selected') + '>' + esc(T('retro.persona.config')) + '</option>' +
        PERSONAS.map(([v, k]) => '<option value="' + v + '"' + (v === persona ? ' selected' : '') + '>' + esc(T(k)) + '</option>').join('') +
        '</select>');
  },
  bind(root) {
    const changed = () => document.dispatchEvent(new CustomEvent('gregal:fun-prefs'));
    const wire = (id, key) => {
      const el = root.querySelector('#' + id);
      if (el) el.onchange = () => { set(key, el.checked ? '1' : '0'); changed(); };
    };
    wire('prefsFunMascot', KEYS.mascot);
    wire('prefsDesktopMascot', KEYS.desktopMascot);
    wire('prefsFunWrapped', KEYS.wrapped);
    wire('prefsFunVoice', KEYS.voice);
    const p = root.querySelector('#prefsRetroPersona');
    if (p) p.onchange = () => { set(PERSONA_KEY, p.value); changed(); };
  },
};
