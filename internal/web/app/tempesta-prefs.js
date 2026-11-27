// Secció «A fons» de Preferències › Models: el model de cada agent de la
// Tempesta i el de la síntesi (/api/winds/tempesta/models), o «Automàtic»
// (el servidor tria). Els models són els de /api/models, com al selector dels vents.
import { idioma } from './i18n.js';

const L = (ca, en) => (idioma() === 'en' ? en : ca);
const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
const el = (tag, cls, txt) => { const e = document.createElement(tag); if (cls) e.className = cls; if (txt != null) e.textContent = txt; return e; };

const SLOTS = () => [
  ['direct', L('Directe', 'Direct'), L('Respon pas a pas i comprova els fets clau.', 'Answers step by step and checks the key facts.')],
  ['sources', L('Fonts', 'Sources'), L('Busca fonts oficials i actuals.', 'Looks for official, current sources.')],
  ['skeptic', L('Advocat del diable', 'Devil’s advocate'), L('Busca el parany o l’alternativa millor.', 'Looks for the trap or the better option.')],
  ['synth', L('Síntesi', 'Synthesis'), L('Ajunta els esborranys en una sola resposta.', 'Merges the drafts into one answer.')],
];

async function json(path, opts) {
  const r = await window.api(path, opts);
  if (!r.ok) throw new Error((await r.text()).trim());
  return r.json();
}

export const tempestaPrefs = {
  section() {
    return '<h3 class="pf-sec">' + esc(L('A fons', 'Deep')) + '</h3><div id="prefsTempesta" class="pf-sub"></div>';
  },

  async bind(root) {
    const box = root.querySelector('#prefsTempesta');
    if (!box) return;
    let chosen, list;
    try {
      [chosen, list] = await Promise.all([json('/api/winds/tempesta/models'), json('/api/models')]);
    } catch (err) {
      box.replaceChildren(el('p', 'pf-skills-status error', L('No s’han pogut llegir els models: ', 'Could not read the models: ') + err.message));
      return;
    }
    chosen = chosen.models || {};
    const all = [];
    for (const prov of Object.keys(list.models || {}).sort()) for (const id of list.models[prov] || []) all.push(prov + '/' + id);
    const status = el('p', 'pf-skills-status');
    status.setAttribute('role', 'status');
    const rows = SLOTS().map(([slot, name, help]) => {
      const row = el('div', 'pf-row'), lab = el('div', 'pf-lab'), sel = el('select', 'pf-select');
      lab.append(el('b', '', name), el('span', '', help));
      sel.setAttribute('aria-label', name);
      const opt = (value, label) => { const o = el('option', '', label); o.value = value; sel.append(o); };
      opt('', L('Automàtic', 'Automatic'));
      // Un model triat que el proveïdor ja no llista segueix sortint: el servidor fa servir l'automàtic.
      for (const m of chosen[slot] && !all.includes(chosen[slot]) ? [chosen[slot], ...all] : all) opt(m, m);
      sel.value = chosen[slot] || '';
      sel.onchange = async () => {
        status.classList.remove('error');
        status.textContent = L('Desant…', 'Saving…');
        try {
          chosen = (await json('/api/winds/tempesta/models', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ slot, model: sel.value }) })).models || {};
          status.textContent = L('Desat', 'Saved');
        } catch (err) {
          sel.value = chosen[slot] || '';
          status.classList.add('error');
          status.textContent = L('No s’ha pogut desar: ', 'Could not save: ') + err.message;
        }
      };
      row.append(lab, sel);
      return row;
    });
    box.replaceChildren(el('p', 'pf-skills-help', L('Amb «A fons», tres agents treballen alhora la pregunta i una síntesi n’escriu una sola resposta. Tria el model de cadascun.',
      'With Deep, three agents work on the question at once and a synthesis writes one answer. Choose the model for each.')), ...rows, status);
  },
};
