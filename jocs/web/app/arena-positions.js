// Equips d'agents per posició: cada joc té les seves posicions (el quarterback,
// l'AWPer, el jungler…) i cada posició té les seves instruccions. Cada camp de
// la decisió del motor el decideix la posició que en té cura; la resta són
// context. L'equip es desa en el mateix .md de sempre, amb una secció per
// posició marcada amb un comentari: el servidor, el rànquing, l'assistent i
// les correccions de després de cada partit continuen treballant amb el .md.
// Pur (sense DOM): el fan servir arena.js i els tests.
(function (root) {
  'use strict';

  // decides: els camps de l'esquema que aquesta posició tria. advises: no tria
  // cap camp, però el que diu entra a la decisió del capità.
  const POSITIONS = {
    nfl: [
      { id: 'head_coach', name: 'Head coach', decides: ['tempo'], also: 'and the special values of `play`: punt, field_goal, kneel, spike (4th downs and the clock)', blurb: 'Clock, tempo and fourth downs: when to punt, kick or go for it.', starter: '- On 4th & 1 past midfield, go for it.\n- Up by 9+ in the 4th quarter: milk the clock.' },
      { id: 'oc', name: 'Offensive coordinator', decides: ['play'], also: 'except the special values the head coach owns', blurb: 'The offensive play call, down by down.', starter: '- Mix run and pass; never call the same play three times in a row.\n- 3rd & long: pass_medium or screen.' },
      { id: 'qb', name: 'Quarterback', decides: ['target'], blurb: 'Who gets the ball on a pass.', starter: '- Favour wr1 on medium passes, the tight end in the red zone.' },
      { id: 'dc', name: 'Defensive coordinator', decides: ['call'], blurb: 'The defensive front and coverage.', starter: '- Blitz on 3rd & long; cover2 against the deep pass.' },
    ],
    cs2: [
      { id: 'igl', name: 'In-game leader', decides: ['strategy', 'setup', 'pace', 'buy'], blurb: 'Calls the round: the plan, the pace and the economy.', starter: '- Full buy only with 4000+ each; otherwise eco together.\n- After two lost rounds on A, fake A and go B.' },
      { id: 'awper', name: 'AWPer', decides: ['awp'], blurb: 'Whether the team buys and plays the AWP this round.', starter: '- Take the AWP on full buys; skip it on half buys.' },
      { id: 'support', name: 'Support', decides: ['utility'], blurb: 'Smokes, flashes and how much utility the round uses.', starter: '- Full utility on executes, none on rushes.' },
      { id: 'entry', name: 'Entry fragger', advises: true, blurb: 'Opens the site: pushes for rushes and fast executes when the enemy is weak.', starter: '- Rush when the enemy saved last round.' },
      { id: 'lurker', name: 'Lurker', advises: true, blurb: 'Fakes, splits and mid control: punishes rotations.', starter: '- If they stack a site twice, call a split.' },
    ],
    lol: [
      { id: 'coach', name: 'Coach (draft)', decides: ['bans', 'picks'], blurb: 'Bans and picks: the composition.', starter: '- Ban the strongest engage; pick a scaling carry when the enemy picks early fighters.' },
      { id: 'top', name: 'Top laner', decides: ['top'], blurb: 'How the top lane plays the laning phase.', starter: '- Passive against a stronger matchup; aggressive when ahead.' },
      { id: 'jungle', name: 'Jungler', decides: ['jungle', 'objective'], blurb: 'Ganks, invades and which neutral objective the team takes.', starter: '- Gank the aggressive enemy lane; take dragon when bot is winning.' },
      { id: 'mid', name: 'Mid laner', decides: ['mid', 'focus', 'action'], blurb: 'The mid lane, and where the team pushes plates and roams.', starter: '- Roam bot after the first back if mid is even.' },
      { id: 'bot', name: 'ADC', decides: ['bot'], blurb: 'The bot lane in the laning phase.', starter: '- Even lane until the support says engage.' },
      { id: 'support', name: 'Support', decides: ['stance'], blurb: 'The team stance in the macro: engage, siege, pick, split or protect.', starter: '- Engage when our team is stronger now; protect the carry when behind.' },
    ],
  };

  const MARK = /<!--\s*position:([a-z_]+)\s*-->/g;
  const TEAM_START = '<!-- team -->', TEAM_END = '<!-- /team -->';

  function positions(game) { return POSITIONS[game] || []; }
  function isTeam(md) { return /<!--\s*position:[a-z_]+\s*-->/.test(String(md || '')); }

  // El preàmbul que llegeix el model: qui decideix cada camp.
  function preamble(game) {
    const lines = positions(game).map(p => '- ' + p.name + (p.decides ? ' decides ' + p.decides.map(f => '`' + f + '`').join(', ') + (p.also ? ' ' + p.also : '') : ' advises the in-game call') + '.');
    return TEAM_START + '\nThis strategy is a TEAM of agents. Each field of a decision is decided by the agent in charge of it, following that agent\'s section below; the other sections are context and advice. When you answer, start "rule" with the position in brackets, e.g. "[' + (positions(game)[0]?.name || 'Captain') + '] …".\n' + lines.join('\n') + '\n' + TEAM_END;
  }

  // parse: el .md → {head, sections:{id: text}}. head és el que hi ha abans de
  // la primera posició (el nom de l'equip i el que val per a tots), sense el preàmbul.
  function parse(md) {
    let text = String(md || '').replace(/\r\n/g, '\n');
    const a = text.indexOf(TEAM_START), b = text.indexOf(TEAM_END);
    if (a >= 0 && b > a) text = text.slice(0, a) + text.slice(b + TEAM_END.length);
    const sections = {};
    const marks = [...text.matchAll(MARK)];
    const head = (marks.length ? text.slice(0, marks[0].index) : text).trim();
    marks.forEach((m, i) => {
      const end = i + 1 < marks.length ? marks[i + 1].index : text.length;
      let body = text.slice(m.index + m[0].length, end).trim();
      // La capçalera «## Nom — decideix …» la torna a posar compose.
      body = body.replace(/^##[^\n]*\n?/, '').trim();
      sections[m[1]] = body;
    });
    return { head, sections };
  }

  // compose: {head, sections} → el .md que es desa i que llegeix el model.
  function compose(game, head, sections) {
    const parts = [String(head || '').trim()];
    parts.push(preamble(game));
    for (const p of positions(game)) {
      const body = String(sections?.[p.id] || '').trim();
      const title = '## ' + p.name + (p.decides ? ' — decides ' + p.decides.map(f => '`' + f + '`').join(', ') : ' — advises');
      parts.push('<!-- position:' + p.id + ' -->\n' + title + '\n' + (body || '- (no instructions yet: follow the team\'s general plan)'));
    }
    return parts.filter(Boolean).join('\n\n') + '\n';
  }

  // Un .md d'abans (sense posicions) passa a equip: tot el que hi havia queda
  // com a pla general, i cada posició comença amb el seu exemple.
  function fromSolo(game, md) {
    const sections = {};
    for (const p of positions(game)) sections[p.id] = p.starter || '';
    return compose(game, String(md || '').trim(), sections);
  }

  // «[AWPer] Take the AWP…» → {position, text}: qui ha decidit, per al registre.
  function byline(game, rule) {
    const m = /^\s*\[([^\]]+)\]\s*/.exec(String(rule || ''));
    if (!m) return { position: null, text: String(rule || '') };
    const name = m[1].toLowerCase();
    const p = positions(game).find(x => x.name.toLowerCase() === name || x.id === name.replace(/\s+/g, '_'));
    return { position: p || { id: '', name: m[1] }, text: String(rule).slice(m[0].length) };
  }

  const api = { POSITIONS, positions, isTeam, parse, compose, fromSolo, byline, preamble };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  root.gregalArenaPositions = api;
})(typeof globalThis !== 'undefined' ? globalThis : this);
