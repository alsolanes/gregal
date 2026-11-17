// Team mode for a company competition (arena_teams.go): the team table, the
// team picker and the organiser's tools (team list, who is in which team, the
// learning report). Pure HTML builders, tested in jocs-teams.test.cjs.

const esc = v => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const rate = wl => (wl && wl.played ? Math.round(100 * wl.won / wl.played) + '%' : '—');

export function teamChipHTML(team) {
  return team ? `<span class="jx-team-chip" title="Team">${esc(team)}</span>` : '';
}

export function teamsTableHTML(rows, myTeam) {
  if (!rows || !rows.length) return '<div class="jx-empty">No teams yet. The organizer sets them up in Cup → Bracket.</div>';
  const medal = i => (i < 3 ? ['🥇', '🥈', '🥉'][i] : i + 1);
  return `<p class="jx-muted jx-note-s">Teams rank by the <b>average XP</b> of their members (practice reviewed by the coach, bots beaten, official wins and coach score), so a big team does not win by size alone.</p>
  <div class="jx-scroll-x" role="region" aria-label="Teams" tabindex="0"><table class="jx-table jx-teams"><thead><tr><th>#</th><th>Team</th><th class="r">Avg XP</th><th class="r">Members</th><th class="r">Prompt score</th><th class="r">Coach gain</th><th class="r">Beat the vague prompt</th><th class="r">Official wins</th><th>Top player</th></tr></thead><tbody>
  ${rows.map((t, i) => `<tr class="${t.name === myTeam ? 'me' : ''} ${i < 3 && t.xp > 0 ? 'top' + (i + 1) : ''}">
    <td class="pos">${t.xp > 0 ? medal(i) : i + 1}</td>
    <td><b>${esc(t.name)}</b>${t.name === myTeam ? ' <small class="jx-you">YOUR TEAM</small>' : ''}</td>
    <td class="r"><b>${t.xp}</b></td>
    <td class="r">${t.active}/${t.members}<small class="jx-faint"> active</small></td>
    <td class="r">${t.prompt_score || '—'}</td>
    <td class="r">${t.improvement > 0 ? `<span class="jx-up">+${t.improvement}</span>` : t.improvement < 0 ? t.improvement : '—'}</td>
    <td class="r">${rate(t.vs_vague)}${t.vs_vague && t.vs_vague.played ? `<small class="jx-faint"> of ${t.vs_vague.played}</small>` : ''}</td>
    <td class="r">${t.official_wins || 0}</td>
    <td>${esc(t.top || '—')}</td></tr>`).join('')}</tbody></table></div>`;
}

export function teamPickerHTML(teams, mine) {
  if (!teams || !teams.length) return '';
  return `<label class="jx-chip jx-team-pick">Team <select id="jxTeamPick"><option value="">— choose —</option>${teams.map(t => `<option ${t === mine ? 'selected' : ''}>${esc(t)}</option>`).join('')}</select></label>`;
}

// The organiser's tools: the team list, who is in which team, and the report.
export function organizerTeamsHTML(teams, members, players) {
  const list = (teams || []).join('\n');
  const opts = sel => `<option value="">—</option>${(teams || []).map(t => `<option ${t === sel ? 'selected' : ''}>${esc(t)}</option>`).join('')}`;
  const people = (players || []).filter(p => p.owner && p.owner !== 'local');
  return `<div class="jx-org-teams">
    <div><h3 class="jx-h3">Teams</h3><p class="jx-muted sm">One per line (departments, offices, practices). Empty = no team mode. People pick theirs on the Play page; you can change anyone's here.</p>
      <textarea id="jxTeamList" rows="5" placeholder="Data Engineering&#10;Analytics&#10;Cloud">${esc(list)}</textarea>
      <button type="button" class="jx-btn sm" id="jxTeamSave">Save teams</button><span class="jx-faint" id="jxTeamSaved"></span></div>
    ${(teams || []).length && people.length ? `<div><h3 class="jx-h3">Who is in which team</h3><div class="jx-org-members">${people.map(p => `<label><span>${esc(p.display)}</span><select data-team-owner="${esc(p.owner)}">${opts((members || {})[p.owner] || '')}</select></label>`).join('')}</div></div>` : ''}
    <div><h3 class="jx-h3">Learning report</h3><p class="jx-muted sm">One row per person: matches, coach scores (first, latest, gain), the five principles, results against the vague prompt and the before/after of their .md. Opens in Excel.</p>
      <button type="button" class="jx-btn sm" id="jxReport">⬇ Download the report (CSV)</button><span class="jx-faint" id="jxReportMsg"></span></div>
  </div>`;
}
