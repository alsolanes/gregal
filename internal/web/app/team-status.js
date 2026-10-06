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
