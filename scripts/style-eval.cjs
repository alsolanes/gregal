#!/usr/bin/env node
// Avaluació d'estil amb el model real: parla amb un servidor Gregal en marxa
// (xat d'un vent, rotllana a tothom, menció a un vent, en català i en anglès)
// i passa cada resposta per les regles de scripts/style-rules.cjs.
//
//   GREGAL_URL=http://127.0.0.1:8240 GREGAL_TOKEN=… node scripts/style-eval.cjs [--crew] [--json out.json]
//
// Crea vents i una rotllana temporals (amb prefix «Prova») i els esborra en
// acabar. No toca els vents ni les converses de l'usuari. --crew afegeix un
// encàrrec de colla (lent: uns minuts) i en revisa l'informe final.
'use strict';
const fs = require('fs');
const rules = require('./style-rules.cjs');

const BASE = (process.env.GREGAL_URL || 'http://127.0.0.1:8240').replace(/\/$/, '');
const TOKEN = process.env.GREGAL_TOKEN || '';
const args = process.argv.slice(2);
const withCrew = args.includes('--crew');
const jsonOut = args.includes('--json') ? args[args.indexOf('--json') + 1] : '';
const only = args.includes('--only') ? args[args.indexOf('--only') + 1] : '';

async function api(path, body, method) {
  const res = await fetch(BASE + path, {
    method: method || (body ? 'POST' : 'GET'),
    headers: { 'Content-Type': 'application/json', ...(TOKEN ? { Authorization: 'Bearer ' + TOKEN } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  if (!res.ok) throw new Error(path + ' → HTTP ' + res.status + ': ' + text.slice(0, 200));
  try { return JSON.parse(text); } catch { return text; }
}

// Tres vents amb veus ben diferents: si al grup sonen iguals, la rotllana
// està barrejant personalitats.
const WINDS = [
  { name: 'Prova Llevant', color: '#7FB3D5', role: 'cuina i receptes', persona: "Ets Prova Llevant, un vent cuiner: pràctic, amb receptes concretes i quantitats. Parles curt i amb gràcia. Emoji de signatura: 🍳 (com a molt un)." },
  { name: 'Prova Mestral', color: '#A3C586', role: 'esport i salut', persona: "Ets Prova Mestral, un vent esportista: directe, amb dades i una mica de pique amable. Parles curt. Mai fas servir emojis." },
  { name: 'Prova Migjorn', color: '#E8B4B8', role: 'llibres i cultura', persona: "Ets Prova Migjorn, un vent lector: reflexiu, amb una referència cultural quan ve a tomb. Parles curt i pausat. Emoji de signatura: 📚 (com a molt un)." },
];

const scenarios = [];
const add = (id, fn) => { if (!only || id.includes(only)) scenarios.push({ id, fn }); };

let winds = [], rot = null;
const names = () => winds.map(w => w.name);

async function windSay(w, message) {
  const r = await api('/api/winds/chat', { wind: w.id, message });
  return String(r.reply || '');
}

async function groupSay(message, extra = {}) {
  const before = (await api('/api/winds/rotllana?rotlla=' + encodeURIComponent(rot.id))).history || [];
  const r = await api('/api/winds/rotllana', { rotllana: rot.id, message, ...extra });
  const after = r.history || [];
  return after.slice(before.length).filter(m => m.from !== 'user').map(m => ({ speaker: m.speaker_name || '?', text: m.text || '' }));
}

// --- Xat d'un vent ---
add('vent-casual-ca', async () => {
  const w = winds[0];
  const t = await windSay(w, 'Hola! Com va el dia?');
  return { reply: t, problems: rules.checkReply(t, { speaker: w.name, others: names().filter(n => n !== w.name), lang: 'ca', kind: 'casual' }) };
});
add('vent-casual-en', async () => {
  const w = winds[1];
  const t = await windSay(w, "Hey! What's a good quick workout for today?");
  return { reply: t, problems: rules.checkReply(t, { speaker: w.name, others: names().filter(n => n !== w.name), lang: 'en', kind: 'casual' }) };
});
add('vent-fet-actual', async () => {
  const w = winds[1];
  const t = await windSay(w, 'Qui va guanyar l\'últim Tour de França? Digue-m\'ho amb la font.');
  return { reply: t, problems: rules.checkReply(t, { speaker: w.name, lang: 'ca', kind: 'fact', needsSource: true }) };
});
add('vent-explica', async () => {
  const w = winds[2];
  const t = await windSay(w, "Explica'm amb detall què és el realisme màgic i quins autors el representen.");
  return { reply: t, problems: rules.checkReply(t, { speaker: w.name, lang: 'ca', kind: 'explain' }) };
});

// --- Rotllana ---
add('grup-tothom-ca', async () => {
  const voices = await groupSay('Què podem sopar avui que sigui ràpid? Doneu-me idees.');
  return { voices, problems: rules.checkGroupTurn(voices, { members: names(), lang: 'ca' }) };
});
add('grup-continua', async () => {
  const voices = await groupSay("I després de sopar, què faríeu per desconnectar una estona?");
  return { voices, problems: rules.checkGroupTurn(voices, { members: names(), lang: 'ca' }) };
});
add('grup-mencio', async () => {
  const w = winds[2];
  const voices = await groupSay('@' + w.name + ' quin llibre curt em recomanes per aquest cap de setmana?');
  return { voices, problems: rules.checkGroupTurn(voices, { members: names(), lang: 'ca', mention: w.name }) };
});
add('grup-tothom-en', async () => {
  const voices = await groupSay("Quick question for everyone: what's one habit that made your week better?");
  return { voices, problems: rules.checkGroupTurn(voices, { members: names(), lang: 'en' }) };
});

add('grup-dues-mencions', async () => {
  const [a, b] = winds;
  const voices = await groupSay('@' + a.name + ' i @' + b.name + ', què és millor abans d\'entrenar: menjar alguna cosa o anar en dejú?');
  const problems = rules.checkGroupTurn(voices, { members: names(), lang: 'ca' });
  const who = new Set(voices.map(v => v.speaker));
  if (who.has(winds[2].name)) problems.push('parla ' + winds[2].name + ' sense que l\'hagin nomenat');
  for (const w of [a, b]) if (!who.has(w.name)) problems.push(w.name + ' nomenat però no respon');
  return { voices, problems };
});
add('vent-castella', async () => {
  const w = winds[0];
  const t = await windSay(w, '¿Qué cocino esta noche con lo que tengo: arroz, huevos y un calabacín?');
  return { reply: t, problems: rules.checkReply(t, { speaker: w.name, lang: 'es', kind: 'casual' }) };
});

// --- Colla (opcional, lent) ---
if (withCrew) add('colla-informe', async () => {
  const before = ((await api('/api/winds/rotllana?rotlla=' + encodeURIComponent(rot.id))).history || []).length;
  // La petició de colla pot quedar oberta mentre treballa (més que el límit de
  // capçaleres de fetch): no l'esperem, seguim el fil i l'estat.
  api('/api/winds/rotllana', { rotllana: rot.id, message: "Feu una pàgina web senzilla amb un comptador de clics a la carpeta prova-comptador i proveu que funciona." }).catch(() => {});
  const t0 = Date.now();
  let report = '', step = '', started = false;
  while (Date.now() - t0 < 15 * 60_000) {
    await new Promise(r => setTimeout(r, 15_000));
    const st = await api('/api/winds/crew/status?rotllana=' + encodeURIComponent(rot.id)).catch(() => ({}));
    step = String(st.step || '');
    if (st.running) { started = true; continue; }
    if (!started && Date.now() - t0 < 60_000) continue;
    const hist = (await api('/api/winds/rotllana?rotlla=' + encodeURIComponent(rot.id))).history || [];
    const fresh = hist.slice(before).filter(m => m.from !== 'user');
    report = (fresh[fresh.length - 1] || {}).text || '';
    break;
  }
  const problems = report ? rules.checkCrewReport(report, { lang: 'ca' }) : ['la colla no ha deixat cap informe (pas: ' + (step || '?') + ')'];
  if (step === 'failed') problems.push('la colla ha fallat');
  return { reply: report, problems };
});

(async () => {
  const results = [];
  try {
    for (const w of WINDS) winds.push((await api('/api/winds', w)).wind);
    rot = (await api('/api/winds/rotllanes', { name: 'Prova estil', members: winds.map(w => w.id) })).rotllana;
    for (const sc of scenarios) {
      const t0 = Date.now();
      let out;
      try { out = await sc.fn(); } catch (e) { out = { problems: ['error: ' + e.message] }; }
      out.id = sc.id; out.ms = Date.now() - t0;
      results.push(out);
      const mark = out.problems.length ? '✖' : '✔';
      console.log(mark + ' ' + sc.id + ' (' + Math.round(out.ms / 1000) + ' s)');
      for (const p of out.problems) console.log('    - ' + p);
      if (process.env.STYLE_VERBOSE) {
        if (out.reply) console.log('    > ' + out.reply.replace(/\n/g, '\n      '));
        for (const v of out.voices || []) console.log('    > [' + v.speaker + '] ' + v.text.replace(/\n/g, '\n      '));
      }
    }
  } finally {
    if (rot) await api('/api/winds/rotllanes', { id: rot.id }, 'DELETE').catch(() => {});
    for (const w of winds) await api('/api/winds', { id: w.id }, 'DELETE').catch(() => {});
  }
  const bad = results.filter(r => r.problems.length).length;
  console.log('\n' + (results.length - bad) + '/' + results.length + ' escenaris nets');
  if (jsonOut) fs.writeFileSync(jsonOut, JSON.stringify(results, null, 2));
  process.exitCode = bad ? 1 : 0;
})();
