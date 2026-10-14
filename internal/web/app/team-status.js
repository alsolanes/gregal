export function progressSummary(roles, states, language = 'en') {
  const counts = Object.fromEntries(['working', 'waiting', 'completed'].map(state => [state, roles.filter(role => states[role] === state).length]));
  const ca = language === 'ca';
  const parts = [];
  if (counts.working) parts.push(`${counts.working} ${ca ? 'treballant' : 'working'}`);
  if (counts.waiting) parts.push(`${counts.waiting} ${ca ? 'esperant un company' : 'waiting for a teammate'}`);
  parts.push(`${counts.completed}/${roles.length} ${ca ? 'completats' : 'completed'}`);
  return parts.join(' · ');
}

// Waiting for a teammate does not require a user decision.
const ATTENTION_STATES = new Set(['failed', 'cancelled']);

export function attentionQueue(roles, states) {
  return roles.filter(role => ATTENTION_STATES.has(states[role]));
}

// nextAttention: següent de la cua després de current (cicle tancat).
// Torna null si no hi ha ningú que necessiti atenció.
export function nextAttention(roles, states, current) {
  const queue = attentionQueue(roles, states);
  if (!queue.length) return null;
  const at = queue.indexOf(current);
  return queue[(at + 1) % queue.length];
}

export function replaceAgentButtons(container, buttons, focusedRole) {
  container.replaceChildren(...buttons);
  if (focusedRole) buttons.find(button => button.dataset.teamRole === focusedRole)?.focus({ preventScroll: true });
}

// teamFlowTopology: el pipeline fix de l'equip (internal/team): el
// coordinador planifica, investigador i constructor treballen en paral·lel,
// el revisor tanca, i la revisió torna al constructor. El graf de la UI
// pinta exactament això; si el motor canvia l'ordre, aquest contracte ho
// assenyala.
export function teamFlowTopology() {
  return {
    nodes: ['coordinator', 'researcher', 'builder', 'reviewer'],
    edges: [
      { from: 'coordinator', to: 'researcher', kind: 'split' },
      { from: 'coordinator', to: 'builder', kind: 'split' },
      { from: 'researcher', to: 'reviewer', kind: 'merge' },
      { from: 'builder', to: 'reviewer', kind: 'merge' },
      { from: 'reviewer', to: 'builder', kind: 'revision' },
    ],
  };
}

// layoutFlow col·loca qualsevol spec per capes de profunditat (camins de
// dades des de les entrades). Tolera esborranys invàlids (cicles): la
// profunditat es talla a 5 perquè el dibuix no exploti; executar ja ho
// rebutja el servidor.
export function layoutFlow(nodes, edges) {
  const ids = nodes.map(n => n.id);
  const depth = Object.fromEntries(ids.map(id => [id, 0]));
  let changed = true, guard = 0;
  while (changed && guard++ < ids.length + 1) {
    changed = false;
    for (const e of edges || []) {
      if (!(e.from in depth) || !(e.to in depth)) continue;
      if (depth[e.to] < depth[e.from] + 1) { depth[e.to] = depth[e.from] + 1; changed = true; }
    }
  }
  const layers = {};
  for (const id of ids) {
    const d = Math.min(depth[id], 5);
    (layers[d] ??= []).push(id);
  }
  const maxDepth = Math.max(0, ...Object.keys(layers).map(Number));
  const pos = {};
  for (const [d, group] of Object.entries(layers)) {
    const y = 36 + Number(d) * 64;
    const gap = group.length < 2 ? 0 : 432 / (group.length - 1);
    group.forEach((id, i) => { pos[id] = [512 + (i - (group.length - 1) / 2) * gap, y]; });
  }
  return { pos, height: 36 + maxDepth * 64 + 32 };
}

// checkFlow: validació lleugera al client abans d'enviar (estructura;
// les regles fondes —cicles, orfes, revisions— les imposa el servidor).
// Torna null si passa, o el motiu localitzat.
const flowProblems = {
  en: { empty: 'Add at least one agent.', id: 'Every agent needs an id.', dup: 'Two agents share an id.', role: 'Unknown role (coordinator, researcher, builder, reviewer).', mode: 'Unknown mode (task, review).', output: 'The output must be one of the agents.', edge: 'A link points to a missing agent.', self: 'An agent cannot link to itself.', kind: 'Unknown link kind (data, revision).', rounds: 'Max rounds must be 0 or more.' },
  ca: { empty: 'Cal almenys un agent.', id: 'Cada agent necessita un id.', dup: 'Dos agents comparteixen id.', role: 'Rol desconegut (coordinador, investigador, constructor, revisor).', mode: 'Mode desconegut (task, review).', output: 'La sortida ha de ser un dels agents.', edge: 'Un enllaç apunta a un agent inexistent.', self: 'Un agent no es pot enllaçar a si mateix.', kind: 'Tipus d\u2019enllaç desconegut (data, revision).', rounds: 'Les rondes han de ser 0 o més.' },
};
export function checkFlow(spec, lang = 'en') {
  const t = flowProblems[lang] || flowProblems.en;
  if (!spec || !Array.isArray(spec.nodes) || !spec.nodes.length) return t.empty;
  const ids = new Set();
  for (const n of spec.nodes) {
    if (!n || typeof n.id !== 'string' || !n.id.trim()) return t.id;
    if (ids.has(n.id)) return t.dup;
    ids.add(n.id);
    if (!['coordinator', 'researcher', 'builder', 'reviewer'].includes(n.role)) return t.role;
    if (n.mode != null && n.mode !== 'task' && n.mode !== 'review') return t.mode;
  }
  if (!ids.has(spec.output)) return t.output;
  for (const e of spec.edges || []) {
    if (!e || !ids.has(e.from) || !ids.has(e.to)) return t.edge;
    if (e.from === e.to) return t.self;
    if (e.kind !== 'data' && e.kind !== 'revision') return t.kind;
  }
  if (spec.max_rounds != null && (!Number.isInteger(spec.max_rounds) || spec.max_rounds < 0)) return t.rounds;
  return null;
}

// readJSON/writeJSON: localStorage amb degradació (navegadors que el
// bloquegen, JSON corromput). storage és qualsevol objecte amb get/setItem.
export function readJSON(storage, key) {
  try {
    const raw = storage?.getItem(key);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

export function writeJSON(storage, key, value) {
  try {
    storage?.setItem(key, JSON.stringify(value));
    return true;
  } catch {
    return false;
  }
}

// flowRoleGuide: què fa cada rol, en la llengua de la UI. És el que
// l'editor mostra perquè ningú hagi d'endevinar què pot fer cada agent:
// cap té eines; només el constructor redacta; només el revisor jutja.
const flowRoleGuideText = {
  en: {
    coordinator: ['Coordinator', 'Plans first and sets acceptance criteria. Its plan feeds every downstream agent. No tools.'],
    researcher: ['Researcher', 'Analyzes information in parallel: evidence, assumptions, gaps. Feeds the review. Reads nothing, browses nothing — pure reasoning.'],
    builder: ['Builder', 'Writes the deliverable from the plan, and revises it on review feedback. The only drafter. No tools.'],
    reviewer: ['Reviewer', 'Strict verdict: approved, feedback and deliverable. Approve ends it; reject loops back bounded by max rounds, then finalizes with the outstanding feedback.'],
  },
  ca: {
    coordinator: ['Coordinador', 'Planifica primer i fixa criteris d’acceptació. El seu pla alimenta la resta. Sense eines.'],
    researcher: ['Investigador', 'Analitza en paral·lel: evidències, supòsits, forats. Alimenta la revisió. No llegeix ni navega: només raona.'],
    builder: ['Constructor', 'Escriu el resultat a partir del pla i el revisa amb el feedback. L’únic redactor. Sense eines.'],
    reviewer: ['Revisor', 'Veredicte estricte: aprovat, feedback i resultat. Aprovar tanca; rebutjar torna enrere acotat per rondes, i acaba amb el feedback pendent.'],
  },
};
export function flowRoleGuide(lang = 'en') {
  const dict = flowRoleGuideText[lang] || flowRoleGuideText.en;
  return ['coordinator', 'researcher', 'builder', 'reviewer'].map(role => ({ role, title: dict[role][0], what: dict[role][1] }));
}
