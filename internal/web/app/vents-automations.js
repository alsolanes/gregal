// Gregal Vents → Automatitzacions: tot el que els vents fan sols, al mateix
// lloc i per compte (com «Automations» de Grok o les rutines de dots):
// resum del matí, recordatoris, rutines (executa ara, pausa, historial),
// la vigilància de cada vent i crear-ne de noves. Fa servir només l'API per
// compte (/api/winds/…, /api/connections): la Marta hi veu les seves.

import { idioma } from './i18n.js';
import { renderAutomations } from './vents-activity.js';
import { renderResponsabilitats } from './vents-responsabilitats.js';
import { renderLocal } from './vents-local.js';
import { t } from './i18n.js';

const $ = id => document.getElementById(id);
const tx = (ca, en) => (idioma() === 'en' ? en : ca);
const api = (p, o) => (window.gregal?.api ? window.gregal.api(p, o) : window.api ? window.api(p, o) : fetch(p, o));
const post = async (path, body) => {
  const r = await api(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  if (!r.ok) throw new Error((await r.text()).trim() || r.status);
  return r.json().catch(() => ({}));
};
const getJSON = async path => { const r = await api(path); if (!r.ok) throw new Error((await r.text()).trim() || r.status); return r.json(); };
const el = (tag, cls, text) => { const e = document.createElement(tag); if (cls) e.className = cls; if (text != null) e.textContent = text; return e; };
const btn = (label, fn, cls = 'auto-btn') => { const b = el('button', cls, label); b.type = 'button'; b.onclick = fn; return b; };
const when = iso => {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d)) return '';
  return d.toLocaleString(idioma() === 'en' ? 'en-GB' : 'ca-ES', { weekday: 'short', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
};

async function openPrefs() {
  try { const m = await import('./prefs.js'); m.prefs.obre('calendar'); } catch (_) { /* sense preferències, res */ }
}

function section(title, help) {
  const s = el('section', 'auto-section');
  s.append(el('h2', '', title));
  if (help) s.append(el('p', 'auto-help', help));
  return s;
}

function empty(text) { return el('p', 'auto-empty', text); }

async function render(box) {
  box.replaceChildren(el('p', 'auto-help', tx('Carregant…', 'Loading…')));
  const [conn, prog, rut, winds] = await Promise.all([
    getJSON('/api/connections').catch(() => null),
    getJSON('/api/winds/programats').catch(() => ({})),
    getJSON('/api/winds/rutines').catch(() => ({ rutines: [] })),
    getJSON('/api/winds').catch(() => ({ winds: [] })),
  ]);
  const windList = winds.winds || [];
  const windName = id => (windList.find(w => w.id === id) || {}).name || id;
  const status = el('p', 'auto-status'); status.setAttribute('role', 'status');
  const refresh = async msg => { await render(box); const s = box.querySelector('.auto-status'); if (s && msg) s.textContent = msg; };
  const act = async (fn, okMsg) => { status.textContent = tx('Un moment…', 'One moment…'); status.setAttribute('role', 'status'); try { await fn(); await refresh(okMsg); } catch (e) { status.textContent = tx('No s’ha pogut fer: ', 'Couldn’t do it: ') + (e.message || e); status.setAttribute('role', 'alert'); } };
  box.replaceChildren();

  const head = el('div', 'auto-head');
  head.append(el('h1', 'page-title', tx('Automatitzacions', 'Automations')),
    el('p', 'page-copy', tx('Tot el que els teus vents fan sols: el resum del matí, els recordatoris, les rutines i el que vigilen. Pots provar-ho ara, pausar-ho o veure què han fet.',
      'Everything your winds do on their own: the morning briefing, reminders, routines and what they keep an eye on. Run it now, pause it or see what they did.')));
  box.append(head, status);

  // Resum del matí i connexions
  const b = (conn && conn.briefing) || {};
  const card = section(tx('Resum del matí', 'Morning briefing'));
  const line = b.enabled
    ? tx('Cada dia a les ' + b.time + ', de ' + windName(b.wind) + '.', 'Every day at ' + b.time + ', by ' + windName(b.wind) + '.')
    : tx('Desactivat.', 'Off.');
  const cals = (conn && conn.calendars) || [], mail = (conn && conn.mail) || {};
  const sources = [cals.length ? tx('Calendari: ', 'Calendar: ') + cals.map(c => c.name).join(', ') : tx('Sense calendari', 'No calendar'),
    mail.has_password ? tx('Correu: ', 'Email: ') + mail.user : tx('Sense correu', 'No email')].join(' · ');
  card.append(el('p', 'auto-line', line), el('p', 'auto-help', sources));
  const cardBtns = el('div', 'auto-actions');
  cardBtns.append(btn(tx('Configura', 'Set up'), openPrefs));
  const briefRoutine = (rut.rutines || []).find(r => (r.id || '').startsWith('briefing-'));
  if (briefRoutine) cardBtns.append(btn(tx('Envia-me’l ara', 'Send it now'), () => act(() => post('/api/winds/rutines/run', { id: briefRoutine.id }), tx('En marxa: et sortirà al xat del vent i com a notificació.', 'Running: it will show up in the wind’s chat and as a notification.'))));
  card.append(cardBtns);
  box.append(card);

  // «A prop teu» (vents-local.js): novetats de la zona a «Avui», amb la font.
  const local = section(tx('A prop teu', 'Near you'), tx('Concerts, festes majors, esdeveniments, obertures de botigues i locals i notícies del lloc on ets. Arriben a «Avui» amb la font, i respecten les hores tranquil·les.',
    'Concerts, local festivals, events, shop and venue openings and news from where you are. They arrive in Today with their source, and respect quiet hours.'));
  local.id = 'autoLocal';
  const localBox = el('div', 'local-box');
  local.append(localBox);
  box.append(local);
  await renderLocal(localBox, windList).catch(e => localBox.replaceChildren(el('p', 'auto-help', tx('No s’ha pogut carregar: ', 'Could not load: ') + (e.message || e))));

  // Responsabilitats (B6): el que vigilen de manera continuada, amb fonts,
  // pressupost i «per què?» (com ResponsabilitatsSheet d'Android).
  const resp = section(t('resp.title'), t('resp.intro'));
  resp.classList.add('resp-section');
  const respBox = el('div', 'resp-box');
  resp.append(respBox);
  box.append(resp);
  await renderResponsabilitats(respBox);

  // Tot el que faran sols, en una sola llista amb accions (com a Android).
  const all = section(tx('Què faran', 'Coming up'), tx('Recordatoris, seguiments que s’han ofert a fer, el que vigilen i les rutines. Fes-ho ara, pausa-ho, mira’n l’historial o treu-ho.',
    'Reminders, follow-ups they offered to do, what they watch and routines. Run it now, pause it, see its history or remove it.'));
  const listBox = el('div', 'act-list');
  all.append(listBox);
  box.append(all);
  await renderAutomations(listBox);

  // Nova rutina (canvas visual Trigger -> Agent -> Output)
  const ns = section(tx('Constructor visual de workflows', 'Visual workflow builder'), tx('Configura quan s’engega, quin minairó ho fa i com vols rebre les notificacions.', 'Configure when it triggers, which wind runs it, and how you want to receive notifications.'));
  if (!windList.length) ns.append(empty(tx('Primer crea un vent (o activa la colla del dia a dia).', 'Create a wind first (or start the everyday crew).')));
  else {
    const canvas = el('div', 'wf-canvas');

    const node = (tag, title) => {
      const n = el('div', 'wf-node');
      const h = el('div', 'wf-node-head');
      h.append(el('span', 'wf-node-tag', tag), el('strong', '', title));
      n.append(h);
      return n;
    };
    const conn = () => {
      const c = el('div', 'wf-conn');
      c.setAttribute('aria-hidden', 'true');
      c.append(el('span', 'wf-conn-dot', '→'));
      return c;
    };

    // 1. Trigger
    const nTrig = node('Trigger', tx('Quan s’executa', 'Schedule'));
    const titleIn = el('input'); titleIn.type = 'text'; titleIn.placeholder = tx('Nom del workflow (opcional)', 'Workflow title (optional)'); titleIn.setAttribute('aria-label', titleIn.placeholder);
    const kind = el('select'); kind.setAttribute('aria-label', tx('Quan', 'When'));
    [['daily', tx('Cada dia a les…', 'Every day at…')], ['interval', tx('Cada… hores', 'Every… hours')]].forEach(([v, t]) => { const o = el('option', '', t); o.value = v; kind.append(o); });
    const time = el('input'); time.type = 'time'; time.value = '08:00'; time.setAttribute('aria-label', tx('Hora', 'Time'));
    const hours = el('input'); hours.type = 'number'; hours.min = '1'; hours.max = '720'; hours.value = '6'; hours.hidden = true; hours.setAttribute('aria-label', tx('Hores', 'Hours'));
    kind.onchange = () => { time.hidden = kind.value !== 'daily'; hours.hidden = kind.value !== 'interval'; };
    const trigRow = el('div', 'wf-row');
    trigRow.append(kind, time, hours);
    nTrig.append(titleIn, trigRow);

    // 2. Agent
    const nAgent = node('Agent', tx('Què ha de fer', 'What to do'));
    const wsel = el('select'); wsel.setAttribute('aria-label', tx('Vent', 'Wind'));
    for (const w of windList) { const o = el('option', '', w.name); o.value = w.id; wsel.append(o); }
    const styleSel = el('select'); styleSel.setAttribute('aria-label', tx('Com fer-ho', 'How to run'));
    [
      ['news', tx('Avisa només si hi ha novetats', 'Notify only if something is new')],
      ['brief', tx('Resum breu en punts', 'Short bulleted summary')],
      ['full', tx('Informe complet', 'Full report')],
    ].forEach(([v, t]) => { const o = el('option', '', t); o.value = v; styleSel.append(o); });
    const prompt = el('textarea'); prompt.rows = 3; prompt.placeholder = tx('Descriu la tasca (p. ex. «Cerca notícies d’IA d’avui i resumeix-les»)', 'Describe the task (e.g. "Find today\'s AI news and summarize it")'); prompt.setAttribute('aria-label', prompt.placeholder);
    const agentRow = el('div', 'wf-row');
    agentRow.append(wsel, styleSel);
    nAgent.append(prompt, agentRow);

    // 3. Output / Notificacions
    const nOut = node('Output', tx('Notificacions', 'Notifications'));
    const notifySel = el('select'); notifySel.setAttribute('aria-label', tx('Com rebre l’avís', 'Notification channel'));
    [
      ['app', tx('Notificació a l’app', 'App notification')],
      ['telegram', tx('App + Telegram', 'App + Telegram')],
      ['silent', tx('Silenciós (només al fil)', 'Silent (thread only)')],
    ].forEach(([v, t]) => { const o = el('option', '', t); o.value = v; notifySel.append(o); });
    const notifyHint = el('p', 'wf-hint', tx('Arribarà com a missatge al xat del minairó i avís a l’app.', 'Arrives as a message in the wind\'s chat and an in-app alert.'));
    notifySel.onchange = () => {
      notifyHint.textContent = notifySel.value === 'telegram'
        ? tx('També s’enviarà per Telegram si el tens connectat.', 'Also sent via Telegram if connected.')
        : notifySel.value === 'silent'
          ? tx('Es guardarà al fil sense enviar notificacions actives.', 'Saved to the thread without active notifications.')
          : tx('Arribarà com a missatge al xat del minairó i avís a l’app.', 'Arrives as a message in the wind\'s chat and an in-app alert.');
    };

    const buildPayload = () => {
      let p = prompt.value.trim();
      if (p) {
        if (styleSel.value === 'news') p += tx('\n(Avisa només si hi ha novetats rellevants.)', '\n(Only notify if there are relevant updates.)');
        else if (styleSel.value === 'brief') p += tx('\n(Fes-ne un resum breu en punts amb enllaços.)', '\n(Write a short bulleted summary with links.)');
        if (notifySel.value === 'silent') p += tx('\n(Mode silenciós: desa-ho al fil sense urgència.)', '\n(Silent mode: save to thread quietly.)');
      }
      return {
        wind: wsel.value,
        name: titleIn.value.trim(),
        kind: kind.value,
        time: time.value,
        interval_h: Number(hours.value || 0),
        prompt: p,
        notify: notifySel.value === 'telegram',
      };
    };

    const outBtns = el('div', 'wf-actions');
    outBtns.append(
      btn(tx('Crea workflow', 'Create workflow'), () => act(() => post('/api/winds/rutina', buildPayload()), tx('Workflow creat.', 'Workflow created.')), 'auto-btn primary'),
      btn(tx('Crea i prova ara ▶', 'Create & run now ▶'), () => act(async () => {
        const res = await post('/api/winds/rutina', buildPayload());
        if (res?.job?.id) await post('/api/winds/rutines/run', { id: res.job.id });
      }, tx('Workflow creat i en execució.', 'Workflow created and running.')), 'auto-btn')
    );
    nOut.append(notifySel, notifyHint, outBtns);

    canvas.append(nTrig, conn(), nAgent, conn(), nOut);
    ns.append(canvas);
  }
  box.append(ns);
}

window.gregalAutomations = {
  async load() {
    const box = $('automatitzacionsInner');
    if (!box) return;
    try { await render(box); } catch (e) { const p = el('p', 'auto-help', tx('No s’han pogut carregar les automatitzacions: ', 'Could not load the automations: ') + (e.message || e)); p.setAttribute('role', 'alert'); box.replaceChildren(p); }
  },
};
