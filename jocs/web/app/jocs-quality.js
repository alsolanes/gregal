// «Does a better prompt win?»: the evidence pieces of Gregal Jocs, from the
// season data (arena_quality.go). The bots are the measuring stick: Bot Rookie
// plays a vague .md, Bot Aggro and Bot Turtle clear ones, Bot Analyst an
// expert one. Pure functions (tested without a DOM in jocs-quality.test.cjs).

const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const pct = wl => (wl && wl.played ? Math.round(100 * wl.won / wl.played) : null);

export const MIN_EVIDENCE = 3; // matches before a number is shown

export const RUNGS = [
  { level: 'expert', label: 'Expert prompt', what: 'Priorities, if/then rules for every situation, reads the rival, examples.' },
  { level: 'solid', label: 'Clear prompt', what: 'A clear structure and identity, some situational rules.' },
  { level: 'rookie', label: 'Vague prompt', what: '4–6 generic lines: “play aggressive”, “be smart”. No rules.' },
];

// The headline: how often a structured prompt beat the vague one.
export function headline(q) {
  const wl = q && q.vs_vague;
  if (!wl || wl.played < MIN_EVIDENCE) return null;
  return { pct: pct(wl), played: wl.played, people: q.people && q.people.played >= MIN_EVIDENCE ? pct(q.people) : null };
}

// Each level of the ladder: its bots, their combined record and the people who beat it.
export function ladder(q, players) {
  const rungs = (q && q.rungs) || [];
  return RUNGS.map(r => {
    const bots = rungs.filter(b => b.level === r.level);
    const all = bots.reduce((a, b) => ({ played: a.played + (b.overall?.played || 0), won: a.won + (b.overall?.won || 0) }), { played: 0, won: 0 });
    const beat = (players || []).filter(p => (p.vs_bots?.[r.level]?.won || 0) > 0)
      .map(p => ({ owner: p.owner, display: p.display, won: p.vs_bots[r.level].won, played: p.vs_bots[r.level].played }))
      .sort((a, b) => b.won / b.played - a.won / a.played || b.won - a.won);
    return { ...r, bots: bots.map(b => b.name), winRate: all.played ? Math.round(100 * all.won / all.played) : null, played: all.played, beat };
  });
}

export function qualityHTML(q, players, me, { compact = false } = {}) {
  const h = headline(q);
  if (compact) {
    if (!h) return '';
    return `<a class="jx-qcard" href="#/ranking"><b class="jx-q-big">${h.pct}%</b><span><b>Better prompts win.</b> A structured prompt beat the vague one in ${h.pct}% of ${h.played} matches, with the same model. <em>See the ladder →</em></span></a>`;
  }
  const rows = ladder(q, players).map(r => `<div class="jx-q-rung lv-${esc(r.level)}">
      <div class="jx-q-l"><b>${esc(r.label)}</b><small>${esc(r.bots.join(' · ') || '—')}</small><p>${esc(r.what)}</p></div>
      <div class="jx-q-wr">${r.winRate == null ? '<span class="jx-faint">no matches yet</span>' : `<div class="jx-wr"><i style="width:${r.winRate}%"></i></div><small>wins ${r.winRate}% of ${r.played}</small>`}</div>
      <div class="jx-q-who">${r.beat.length ? r.beat.map(p => `<button type="button" class="jx-q-chip ${p.owner === me ? 'me' : ''}" data-player="${esc(p.owner)}" title="${p.won} of ${p.played} won">${esc(p.display)} <small>${p.won}/${p.played}</small></button>`).join('') : '<span class="jx-faint">nobody has beaten it yet</span>'}</div>
    </div>`).join('');
  return `<section class="jx-quality">
    <div class="jx-sec-head"><div><div class="jx-kicker">The evidence</div><h2 class="jx-h2">Does a better prompt win?</h2></div></div>
    ${h ? `<div class="jx-q-hero"><b class="jx-q-big">${h.pct}%</b><p>of the time a <b>structured prompt beat the vague one</b>: ${h.played} matches against Bot Rookie, same model, same rules.${h.people != null ? ` People alone: ${h.people}%.` : ''}</p></div>`
      : `<p class="jx-muted">The number appears after ${MIN_EVIDENCE} matches against Bot Rookie, the bot with a vague prompt.</p>`}
    <div class="jx-q-ladder"><div class="jx-q-head"><span>Prompt quality</span><span>How the bot does</span><span>Who has beaten it</span></div>${rows}</div>
  </section>`;
}

// The versions of a player's .md in the game where they iterated most.
export function bestVersions(p) {
  let best = null;
  for (const [g, x] of Object.entries((p && p.games) || {})) {
    const v = x.versions || [];
    if (v.length && (!best || v.length > best.versions.length)) best = { game: g, versions: v };
  }
  return best;
}

export function beforeAfterHTML(p, gameName = g => g) {
  const b = bestVersions(p);
  if (!b || b.versions.length < 2) {
    return `<div class="jx-panel jx-ba"><div class="jx-panel-k">Before / after</div><p class="jx-muted sm">Change the .md after a coach report and play again: each version's results show up here.</p></div>`;
  }
  const v = b.versions, first = v[0], last = v[v.length - 1];
  const wr = x => (x.played ? Math.round(100 * x.won / x.played) : 0);
  const delta = (a, z, unit) => { const d = z - a; return `<em class="${d > 0 ? 'up' : d < 0 ? 'down' : ''}">${d > 0 ? '+' : ''}${d}${unit}</em>`; };
  const bars = v.map(x => `<div class="jx-ba-bar" title="v${x.n}: ${x.won}/${x.played} won${x.coach ? ', coach ' + x.coach : ''}"><i style="height:${Math.max(6, wr(x))}%"></i><small>v${x.n}</small></div>`).join('');
  const col = (x, label) => `<div><small>${label} · v${x.n}</small><b>${wr(x)}%</b><span>won (${x.won}/${x.played})</span>${x.coach ? `<span>coach ${x.coach}/100</span>` : ''}<span>${x.lines} lines</span></div>`;
  return `<div class="jx-panel jx-ba"><div class="jx-panel-k">Before / after · ${esc(gameName(b.game))}</div>
    <div class="jx-ba-cols">${col(first, 'First')}<div class="jx-ba-arrow">→${delta(wr(first), wr(last), ' pts')}${first.coach && last.coach ? delta(first.coach, last.coach, ' coach') : ''}</div>${col(last, 'Latest')}</div>
    <div class="jx-ba-bars" aria-label="Win rate of each version">${bars}</div></div>`;
}
