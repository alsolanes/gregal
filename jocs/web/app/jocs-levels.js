// Prompt Level and challenges: the learning ladder of Gregal Jocs.
//  - levelOf(player): XP from what a player has actually done (coach-reviewed
//    matches, bots beaten, official wins, coach score) → a named level and the
//    progress to the next one.
//  - CHALLENGES: short goals, each teaching one prompting principle. They are
//    checked live (against the Prompt check of the .md being written, or the
//    player's stats), so there is no extra server state.
// Pure functions: tested without a DOM in jocs-levels.test.cjs.

export const LEVELS = [
  { at: 0, name: 'Rookie Prompter', icon: '🌱' },
  { at: 60, name: 'Rule Writer', icon: '✏️' },
  { at: 150, name: 'Tactician', icon: '🧭' },
  { at: 300, name: 'Strategist', icon: '♟️' },
  { at: 500, name: 'Prompt Architect', icon: '🏛️' },
  { at: 800, name: 'Grandmaster', icon: '👑' },
];

// What earns XP, in plain words (also shown to players).
export const XP_RULES = [
  ['10', 'per match the coach reviewed'],
  ['20', 'per bot beaten'],
  ['30', 'per official win'],
  ['2', 'per point of your average coach score'],
];

export function xpOf(p) {
  if (!p) return 0;
  const wins = Array.isArray(p.official) ? +p.official[0] || 0 : 0;
  const reviewed = +p.reviewed || 0;
  return Math.round(reviewed * 10 + (+p.bots_beaten || 0) * 20 + wins * 30 + (reviewed ? (+p.prompt_score || 0) * 2 : 0));
}

export function levelOf(p) {
  const xp = xpOf(p);
  let i = 0;
  while (i + 1 < LEVELS.length && xp >= LEVELS[i + 1].at) i++;
  const cur = LEVELS[i], nxt = LEVELS[i + 1] || null;
  const pct = nxt ? Math.round(100 * (xp - cur.at) / (nxt.at - cur.at)) : 100;
  return { n: i + 1, name: cur.name, icon: cur.icon, xp, from: cur.at, to: nxt ? nxt.at : null, pct, next: nxt ? nxt.name : null, max: !nxt };
}

const pass = (lint, id) => !!lint?.checks?.find(c => c.id === id)?.ok;
const wins = p => (Array.isArray(p?.official) ? +p.official[0] || 0 : 0);

// ctx = { lint: lintMD(...) of the .md in the editor, p: the player's season entry (or null) }
export const CHALLENGES = [
  { id: 'measure', principle: 'Specificity', title: 'Make it measurable', how: 'Write at least 3 rules with a number a game can check (a down and distance, a clock time, a percentage).', done: c => pass(c.lint, 'numbers') },
  { id: 'vague', principle: 'Clarity', title: 'Cut the fluff', how: 'No “be smart”, “try to”, “sometimes”. Every line says exactly when and what.', done: c => pass(c.lint, 'vague') && (c.lint?.checks?.length || 0) > 0 && (c.lint.ok >= 3) },
  { id: 'default', principle: 'Coverage', title: 'Always have an answer', how: 'Add an “Otherwise: …” line so no situation leaves the model guessing.', done: c => pass(c.lint, 'default') },
  { id: 'read', principle: 'Adaptivity', title: 'Read the rival', how: 'Write what to do when the opponent repeats a habit: “if they key on X, switch to Y”.', done: c => pass(c.lint, 'opponent') },
  { id: 'clean', principle: 'All five', title: 'Clean sheet', how: 'Pass every check of the Prompt check (8/8).', done: c => !!c.lint && c.lint.ok === c.lint.total },
  { id: 'first', principle: 'Practice', title: 'First whistle', how: 'Play a practice match and read the coach report.', done: c => (+c.p?.reviewed || 0) >= 1 },
  { id: 'bot', principle: 'Practice', title: 'Beat a bot', how: 'Win a practice match against any bot.', done: c => (+c.p?.bots_beaten || 0) >= 1 },
  { id: 'rewrite', principle: 'Iteration', title: 'Rewrite and improve', how: 'Fix what the coach said and get a higher coach score than your first match.', done: c => (+c.p?.reviewed || 0) >= 2 && (+c.p?.coach_last || 0) > (+c.p?.coach_first || 0) },
  { id: 'good', principle: 'Quality', title: 'Coach approved', how: 'Reach an average coach score of 70/100.', done: c => (+c.p?.reviewed || 0) >= 1 && (+c.p?.prompt_score || 0) >= 70 },
  { id: 'official', principle: 'Competition', title: 'Win on the big stage', how: 'Win an official match in a competition.', done: c => wins(c.p) >= 1 },
];

// Which challenges are done now, merged with the ones already earned before
// (`earned`: an array of ids kept by the caller). → { done: [ids], fresh: [ids newly done] }
export function evaluate(ctx, earned = []) {
  const had = new Set(earned);
  const done = [], fresh = [];
  for (const ch of CHALLENGES) {
    const ok = had.has(ch.id) || !!ch.done(ctx || {});
    if (ok) { done.push(ch.id); if (!had.has(ch.id)) fresh.push(ch.id); }
  }
  return { done, fresh };
}

const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

export function levelBadgeHTML(p) {
  const l = levelOf(p);
  return `<span class="jx-lvl" title="Level ${l.n} · ${l.xp} XP"><i aria-hidden="true">${l.icon}</i>${esc(l.name)}</span>`;
}

export function levelCardHTML(p, doneIds) {
  const l = levelOf(p), done = new Set(doneIds || []);
  const list = CHALLENGES.map(ch => `<li class="${done.has(ch.id) ? 'done' : ''}"><i aria-hidden="true">${done.has(ch.id) ? '✓' : '○'}</i><div><b>${esc(ch.title)}</b><small>${esc(ch.principle)}</small><p>${esc(ch.how)}</p></div></li>`).join('');
  const nextCh = CHALLENGES.find(ch => !done.has(ch.id));
  return `<div class="jx-lvlcard">
    <div class="jx-lvl-top"><span class="jx-lvl-ic" aria-hidden="true">${l.icon}</span>
      <div class="jx-lvl-id"><small>Prompt level ${l.n}</small><b>${esc(l.name)}</b></div>
      <div class="jx-lvl-bar" role="progressbar" aria-label="${l.max ? 'Prompt level' : 'XP to ' + esc(l.next)}" aria-valuetext="${l.max ? l.xp + ' XP, top level' : l.xp + ' of ' + l.to + ' XP'}" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${l.pct}"><span style="width:${l.pct}%"></span></div>
      <span class="jx-lvl-xp">${l.max ? `${l.xp} XP · max level` : `${l.xp} / ${l.to} XP to ${esc(l.next)}`}</span></div>
    <details class="jx-chal"><summary><b>Challenges ${done.size}/${CHALLENGES.length}</b>${nextCh ? `<span>Next: ${esc(nextCh.title)}</span>` : '<span>All done 🎉</span>'}</summary>
      <ol class="jx-chal-list">${list}</ol>
      <p class="jx-faint">XP: ${XP_RULES.map(([n, t]) => `+${n} ${esc(t)}`).join(' · ')}.</p></details></div>`;
}
