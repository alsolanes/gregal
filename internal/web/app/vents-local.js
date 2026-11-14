// Gregal Vents → «A prop teu»: novetats locals del lloc on ets (concerts,
// festes majors, esdeveniments, obertures, notícies de la zona). Mateixa API
// i mateixes regles que LocalInfoCard d'Android (/api/winds/local):
// consentiment explícit, només el nom de la ciutat o el barri (mai
// coordenades desades), temes, distància, freqüència, hora i si també ho ha
// de dir com a veu. Les novetats arriben a «Avui» amb la font citada.

import { idioma } from './i18n.js';

const tx = (ca, en) => (idioma() === 'en' ? en : ca);
const api = (p, o) => (globalThis.window?.gregal?.api ? window.gregal.api(p, o) : globalThis.window?.api ? window.api(p, o) : fetch(p, o));
const getJSON = async path => { const r = await api(path); if (!r.ok) throw new Error((await r.text()).trim() || r.status); return r.json(); };
const post = async (path, body) => {
  const r = await api(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  if (!r.ok) throw new Error((await r.text()).trim() || r.status);
  return r.json().catch(() => ({}));
};
const el = (tag, cls, text) => { const e = document.createElement(tag); if (cls) e.className = cls; if (text != null) e.textContent = text; return e; };
const btn = (label, fn, cls = 'auto-btn') => { const b = el('button', cls, label); b.type = 'button'; b.onclick = fn; return b; };

// --- lògica pura (testejable sense DOM) ---

export const RADII = [2, 5, 10, 25, 50];
export const FREQUENCIES = ['daily', '2days', 'weekly'];

/** Com es diu la freqüència, amb l'hora. */
export function frequencyLabel(freq, time) {
  switch (freq) {
    case '2days': return tx('Cada dos dies a les ' + time, 'Every two days at ' + time);
    case 'weekly': return tx('Cada setmana a les ' + time, 'Every week at ' + time);
    default: return tx('Cada dia a les ' + time, 'Every day at ' + time);
  }
}

/** La línia que resumeix com està configurat. */
export function summaryLine(st, topics) {
  if (!st || !st.enabled) return tx('Desactivat.', 'Off.');
  const names = (st.topics || []).map(id => (topics || []).find(t => t.id === id)?.label || id);
  return [frequencyLabel(st.frequency, st.time), st.place?.label || '', tx('a ' + st.radius_km + ' km', 'within ' + st.radius_km + ' km'), names.join(', ')]
    .filter(Boolean).join(' · ');
}

/** Per què encara no es pot activar ('' = es pot). Mateixes regles que el servidor. */
export function enableProblem(form) {
  if (!form.consent) return tx('Cal el teu consentiment per fer servir el teu lloc.', 'Your consent to use your place is required.');
  if (!form.place || !String(form.place.label || '').trim()) return tx('Tria on ets (una ciutat o un barri).', 'Choose where you are (a town or a neighbourhood).');
  if (!(form.topics || []).length) return tx('Tria almenys un tema.', 'Choose at least one topic.');
  return '';
}

/** Un resultat de la cerca de llocs → el lloc que es desa (sense coordenades). */
export function placeFromSearch(p) {
  return { label: p.name || '', region: p.where || '', source: 'manual' };
}

/** Coordenades del navegador arrodonides a ~1 km abans de sortir de la pàgina. */
export function roundCoords(lat, lon) {
  const r = v => Math.round(v * 100) / 100;
  return { lat: r(lat), lon: r(lon) };
}

/** El cos del POST a partir del formulari (mai coordenades). */
export function payloadFrom(form) {
  const out = {
    enabled: !!form.enabled, consent: !!form.consent, topics: [...(form.topics || [])],
    interests: form.interests || '', avoid: form.avoid || '', radius_km: Number(form.radius_km) || 10,
    frequency: FREQUENCIES.includes(form.frequency) ? form.frequency : 'daily', time: form.time || '09:00',
    voice: !!form.voice,
  };
  if (form.wind) out.wind = form.wind;
  if (form.place && form.place.label) out.place = { label: form.place.label, region: form.place.region || '', country: form.place.country || '', source: form.place.source || 'manual' };
  return out;
}

// --- vista ---

function browserPosition() {
  return new Promise((resolve, reject) => {
    if (!navigator.geolocation) { reject(new Error(tx('Aquest navegador no dona la ubicació.', 'This browser cannot give your location.'))); return; }
    navigator.geolocation.getCurrentPosition(p => resolve(p.coords), e => reject(new Error(e.message || 'geolocation')),
      { enableHighAccuracy: false, maximumAge: 3600e3, timeout: 15e3 });
  });
}

/** Pinta la secció «A prop teu» dins de [box]. */
export async function renderLocal(box, winds = []) {
  const data = await getJSON('/api/winds/local');
  const st = data.settings || {};
  const topics = data.topics || [];
  const form = { ...st, topics: [...(st.topics || [])], consent: !!st.consent, place: st.place && st.place.label ? { ...st.place } : null };
  box.replaceChildren();
  const status = el('p', 'auto-status'); status.setAttribute('role', 'status');
  const say = (msg, bad) => { status.textContent = msg; status.setAttribute('role', bad ? 'alert' : 'status'); };

  box.append(el('p', 'auto-line local-summary', summaryLine(st, topics)));
  if (data.job?.next_due_at && st.enabled) {
    const d = new Date(data.job.next_due_at);
    if (!isNaN(d)) box.append(el('p', 'auto-help', tx('Propera mirada: ', 'Next look: ') + d.toLocaleString(idioma() === 'en' ? 'en-GB' : 'ca-ES', { weekday: 'short', hour: '2-digit', minute: '2-digit' })));
  }

  const f = el('div', 'auto-form local-form');
  // On ets
  const placeRow = el('div', 'local-place');
  const placeName = el('strong', 'local-place-name', form.place?.label || tx('Encara no has triat on ets.', 'You have not chosen where you are yet.'));
  const placeSrc = el('small', 'auto-help', form.place ? ({ device: tx('ubicació del mòbil', 'phone location'), browser: tx('ubicació del navegador', 'browser location') }[form.place.source] || tx('triat a mà', 'chosen by hand')) : '');
  const setPlace = p => { form.place = p; placeName.textContent = p.label; placeSrc.textContent = p.source === 'browser' ? tx('ubicació del navegador', 'browser location') : tx('triat a mà', 'chosen by hand'); };
  placeRow.append(placeName, placeSrc);
  const q = el('input'); q.type = 'search'; q.placeholder = tx('Cerca la teva ciutat o barri…', 'Search your town or neighbourhood…'); q.setAttribute('aria-label', q.placeholder);
  const results = el('div', 'local-results');
  let timer = 0;
  q.oninput = () => {
    clearTimeout(timer);
    timer = setTimeout(async () => {
      const r = await getJSON('/api/winds/weather/places?q=' + encodeURIComponent(q.value)).catch(() => ({ places: [] }));
      results.replaceChildren();
      for (const p of r.places || []) {
        const b = btn(p.name + (p.where ? ' · ' + p.where : ''), () => { setPlace(placeFromSearch(p)); results.replaceChildren(); q.value = ''; }, 'auto-btn local-result');
        results.append(b);
      }
    }, 250);
  };
  const locate = btn(tx('Fes servir la meva ubicació aproximada', 'Use my approximate location'), async () => {
    say(tx('Buscant on ets…', 'Finding where you are…'));
    try {
      const c = await browserPosition();
      const r = await post('/api/winds/local/locate', { ...roundCoords(c.latitude, c.longitude), source: 'browser' });
      setPlace(r.place); say(tx('Ets a ' + r.place.label + '. Desa per fer-lo servir.', 'You are in ' + r.place.label + '. Save to use it.'));
    } catch (e) { say(tx('No s’ha pogut saber on ets: ', 'Could not find where you are: ') + (e.message || e), true); }
  });
  f.append(placeRow, q, results, locate);

  // Temes
  const tbox = el('fieldset', 'local-topics');
  tbox.append(el('legend', '', tx('Què vols que et porti', 'What to bring you')));
  for (const tp of topics) {
    const lab = el('label', 'local-topic'), cb = el('input'); cb.type = 'checkbox'; cb.checked = form.topics.includes(tp.id);
    cb.onchange = () => { form.topics = cb.checked ? [...new Set([...form.topics, tp.id])] : form.topics.filter(x => x !== tp.id); };
    lab.append(cb, el('span', '', tp.label)); tbox.append(lab);
  }
  f.append(tbox);
  const interests = el('input'); interests.value = form.interests || ''; interests.placeholder = tx('M’interessa especialment… (jazz, mercats, activitats per a nens)', 'I am especially into… (jazz, markets, kids’ activities)'); interests.setAttribute('aria-label', interests.placeholder);
  interests.oninput = () => { form.interests = interests.value; };
  const avoid = el('input'); avoid.value = form.avoid || ''; avoid.placeholder = tx('No em portis… (futbol, política)', 'Never bring me… (football, politics)'); avoid.setAttribute('aria-label', avoid.placeholder);
  avoid.oninput = () => { form.avoid = avoid.value; };
  f.append(interests, avoid);

  // Distància, freqüència, hora, vent
  const row = el('div', 'local-row');
  const radius = el('select'); radius.setAttribute('aria-label', tx('Distància', 'Distance'));
  for (const km of RADII) { const o = el('option', '', tx('A ' + km + ' km', 'Within ' + km + ' km')); o.value = String(km); radius.append(o); }
  radius.value = String(RADII.includes(form.radius_km) ? form.radius_km : 10); radius.onchange = () => { form.radius_km = Number(radius.value); };
  const freq = el('select'); freq.setAttribute('aria-label', tx('Freqüència', 'Frequency'));
  for (const [v, l] of [['daily', tx('Cada dia', 'Every day')], ['2days', tx('Cada dos dies', 'Every two days')], ['weekly', tx('Cada setmana', 'Every week')]]) { const o = el('option', '', l); o.value = v; freq.append(o); }
  freq.value = form.frequency || 'daily'; freq.onchange = () => { form.frequency = freq.value; };
  const time = el('input'); time.type = 'time'; time.value = form.time || '09:00'; time.setAttribute('aria-label', tx('Hora', 'Time')); time.onchange = () => { form.time = time.value; };
  row.append(radius, freq, time);
  if (winds.length) {
    const ws = el('select'); ws.setAttribute('aria-label', tx('Qui t’ho porta', 'Who brings it'));
    for (const w of winds) { const o = el('option', '', w.name); o.value = w.id; ws.append(o); }
    ws.value = form.wind && winds.some(w => w.id === form.wind) ? form.wind : winds[0].id; form.wind = ws.value; ws.onchange = () => { form.wind = ws.value; };
    row.append(ws);
  }
  f.append(row);

  const check = (text, key) => { const lab = el('label', 'local-check'), cb = el('input'); cb.type = 'checkbox'; cb.checked = !!form[key]; cb.onchange = () => { form[key] = cb.checked; }; lab.append(cb, el('span', '', text)); return lab; };
  f.append(check(tx('També com a veu: el vent t’ho diu al seu xat i avisa (mai en hores tranquil·les). Si no, només a «Avui».', 'Also as a voice: the wind tells you in its chat and notifies you (never during quiet hours). Otherwise, only in Today.'), 'voice'));
  f.append(check(tx('Accepto que Gregal faci servir el lloc que trio (només el nom de la ciutat o el barri, mai la ubicació exacta) per buscar novetats properes.', 'I agree that Gregal uses the place I choose (only the town or neighbourhood name, never my exact location) to look for things nearby.'), 'consent'));

  const acts = el('div', 'auto-actions');
  const save = async enabled => {
    const body = payloadFrom({ ...form, enabled });
    if (enabled) { const p = enableProblem(body); if (p) { say(p, true); return; } }
    say(tx('Un moment…', 'One moment…'));
    try { await post('/api/winds/local', body); await renderLocal(box, winds); const s = box.querySelector('.auto-status'); if (s) s.textContent = enabled ? tx('Activat. Les novetats arribaran a «Avui», amb la font.', 'On. Local news will show up in Today, with its source.') : tx('Desat.', 'Saved.'); }
    catch (e) { say(tx('No s’ha pogut desar: ', 'Couldn’t save: ') + (e.message || e), true); }
  };
  acts.append(btn(st.enabled ? tx('Desa', 'Save') : tx('Activa', 'Turn on'), () => save(true), 'auto-btn primary'));
  if (st.enabled) {
    acts.append(btn(tx('Mira-ho ara', 'Look now'), async () => { try { await post('/api/winds/local', { action: 'run' }); say(tx('En marxa: el que trobi sortirà a «Avui».', 'Running: what it finds will show up in Today.')); } catch (e) { say(e.message || String(e), true); } }));
    acts.append(btn(tx('Apaga', 'Turn off'), () => save(false)));
  }
  if (st.place?.label || st.consent) acts.append(btn(tx('Oblida el meu lloc', 'Forget my place'), async () => { try { await post('/api/winds/local', { action: 'forget' }); await renderLocal(box, winds); } catch (e) { say(e.message || String(e), true); } }));
  f.append(acts, status);
  box.append(f);
}
