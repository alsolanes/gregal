// Secció «Extres» de Preferències: tot el que és diversió es pot apagar.
// Viu al navegador (localStorage), com la resta d'aparença; el to de #retro
// per defecte, si no se'n tria cap, és el del config (retro.persona).
import { t as T } from './i18n.js';

const KEYS = { mascot: 'gregal_fun_mascot', wrapped: 'gregal_fun_wrapped', voice: 'gregal_fun_voice', desktopMascot: 'gregal_desktop_mascot', kanban: 'gregal_exp_kanban' };
// La mascota flotant només existeix a l'app d'escriptori (desktop/main.js):
// el preload n'anuncia el suport i la preferència viatja per savePreference.
const hasDesktopMascot = () => { try { return !!(window.gregalDesktop && window.gregalDesktop.mascotWindow); } catch (e) { return false; } };
export const PERSONA_KEY = 'gregal_retro_persona';
export const PERSONAS = [['neutral', 'retro.persona.neutral'], ['friendly', 'retro.persona.friendly'], ['sarcastic', 'retro.persona.sarcastic'], ['mentor', 'retro.persona.mentor']];

const get = (k, d) => { try { const v = localStorage.getItem(k); return v == null ? d : v; } catch (e) { return d; } };
const set = (k, v) => { try { localStorage.setItem(k, v); } catch (e) {} };
const esc = s => String(s).replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

export function retroPersonaPref() { return get(PERSONA_KEY, ''); }

// Les files que no són «diversió» viuen en un altre grup de Preferències: el
// dictat per veu és una manera d'escriure (General) i el kanban, una funció
// experimental (Avançat). Totes es lliguen igual, des de bind().
const row = (title, help, inner) => '<div class="pf-row"><div class="pf-lab"><b>' + esc(T(title)) + '</b><span>' + esc(T(help)) + '</span></div>' + inner + '</div>';
const check = (id, on, label) => '<label class="pf-check"><input type="checkbox" aria-label="' + esc(T(label)) + '" id="' + id + '"' + (on ? ' checked' : '') + '><span></span></label>';

export const funPrefs = {
  section() {
    const persona = retroPersonaPref();
    return row('fun.mascot', 'fun.mascot.help', check('prefsFunMascot', get(KEYS.mascot, '0') === '1', 'fun.mascot')) +
      (hasDesktopMascot() ? row('fun.desktopMascot', 'fun.desktopMascot.help', check('prefsDesktopMascot', get(KEYS.desktopMascot, '0') === '1', 'fun.desktopMascot')) : '') +
      row('retro.persona', 'retro.persona.help',
        '<select class="pf-select" id="prefsRetroPersona" aria-label="' + esc(T('retro.persona')) + '">' +
        '<option value=""' + (persona ? '' : ' selected') + '>' + esc(T('retro.persona.config')) + '</option>' +
        PERSONAS.map(([v, k]) => '<option value="' + v + '"' + (v === persona ? ' selected' : '') + '>' + esc(T(k)) + '</option>').join('') +
        '</select>');
  },
  // General: el micròfon del camp de text.
  dictationRow() { return row('fun.voice', 'fun.voice.help', check('prefsFunVoice', get(KEYS.voice, '1') !== '0', 'fun.voice')); },
  // Avançat: el tauler de terminals.
  kanbanRow() { return row('kanban.pref', 'kanban.pref.help', check('prefsKanban', get(KEYS.kanban, '0') === '1', 'kanban.pref')); },
  bind(root) {
    const changed = () => document.dispatchEvent(new CustomEvent('gregal:fun-prefs'));
    const wire = (id, key) => {
      const el = root.querySelector('#' + id);
      if (el) el.onchange = () => { set(key, el.checked ? '1' : '0'); changed(); };
    };
    wire('prefsFunMascot', KEYS.mascot);
    wire('prefsDesktopMascot', KEYS.desktopMascot);
    wire('prefsFunVoice', KEYS.voice);
    wire('prefsKanban', KEYS.kanban);
    const p = root.querySelector('#prefsRetroPersona');
    if (p) p.onchange = () => { set(PERSONA_KEY, p.value); changed(); };
  },
};
