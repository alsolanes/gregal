const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const load = f => import(pathToFileURL(path.join(__dirname, f)).href);
const opt = id => ({ id, label: id, plain: 'what ' + id + ' does in plain words' });
const sec = (id, mine, theirs, grid, extra) => ({ id, title: id, intro: 'intro', rule: '- If they {theirs}: {mine}.', mine: mine.map(opt), theirs: theirs.map(opt), grid, ...extra });

// Unes guies petites amb la forma de les del servidor (games[].playbook).
const PB = {
  nfl: { sections: [
    sec('offense', ['run_inside', 'qb_sneak', 'pass_short', 'screen', 'play_action'], ['base', 'blitz', 'cover2'], [[-1, -2, 2], [0, -1, 1], [0, 1, -1], [-1, 2, 0], [2, -2, 0]]),
    sec('defense', ['base', 'blitz', 'cover2', 'goal_line'], ['run_inside', 'pass_short', 'screen'], [[1, 0, 1], [2, -1, -2], [-2, 1, 0], [2, -1, -1]]),
  ] },
  cs2: { sections: [
    sec('t', ['rush_a', 'rush_b', 'fake_a_go_b', 'default_mid'], ['standard_2_1_2', 'stack_a', 'aggressive_mid'], [[-1, -2, 2], [-1, 2, 2], [1, 2, 0], [0, 2, -1]], { values: [[40, 25, 70], [40, 70, 70], [62, 75, 50], [47, 68, 35]], unit: '% rounds won' }),
    sec('ct', ['standard_2_1_2', 'stack_a', 'aggressive_mid'], ['rush_a', 'fake_a_go_b', 'default_mid'], [[1, -1, 0], [2, -2, -2], [-2, 0, 2]], { values: [[60, 38, 53], [75, 25, 32], [30, 50, 65]] }),
  ] },
  lol: { sections: [sec('stance', ['engage', 'siege', 'pick', 'split', 'protect'], ['engage', 'siege', 'pick', 'split', 'protect'],
    [[0, 2, 2, -2, -2], [-2, 0, -2, 2, 2], [-2, 2, 0, 2, -2], [2, -2, -2, 0, 2], [2, -2, 2, -2, 0]])] },
};

test('answers: best grade first, then the engine number', async () => {
  const { answers } = await load('arena-playbook.js');
  const a = answers(PB.cs2.sections[0], 1); // vs stack_a
  assert.deepEqual(a.map(x => x.id), ['fake_a_go_b', 'rush_b', 'default_mid', 'rush_a']);
});

test('addRule: makes the counters section, appends, replaces the same situation, never duplicates', async () => {
  const { addRule, RULES_HEAD } = await load('arena-playbook.js');
  let r = addRule('# Strategy\n\n## Offense\n- run.\n', '- If their most-called defense is blitz: call screen.');
  assert.equal(r.kind, 'added');
  assert.ok(r.md.includes(RULES_HEAD + '\n'));
  assert.equal(r.md.split('\n')[r.line], '- If their most-called defense is blitz: call screen.');
  r = addRule(r.md, '- If their most-called defense is cover2: call run_inside.');
  assert.equal(r.kind, 'added');
  assert.equal(r.md.split(RULES_HEAD).length, 2, 'one heading only');
  r = addRule(r.md, '- If their most-called defense is blitz: call pass_short.');
  assert.equal(r.kind, 'replaced');
  assert.ok(!r.md.includes('call screen.'));
  const again = addRule(r.md, '- If their most-called defense is blitz: call pass_short.');
  assert.equal(again.kind, 'same');
  assert.equal(again.md, r.md);
  // Les regles van dins del bloc, abans del títol següent.
  const mid = addRule(RULES_HEAD + '\n- a: x.\n\n## Defense\n- d.\n', '- b: y.');
  assert.deepEqual(mid.md.split('\n').slice(0, 3), [RULES_HEAD, '- a: x.', '- b: y.']);
  assert.ok(addRule('', '- z: w.').md.startsWith(RULES_HEAD));
});

test('counters: the style only breaks ties between equally good answers', async () => {
  const { counters } = await load('arena-builder.js');
  const t = PB.cs2.sections[0];
  assert.equal(counters(t, {})['stack_a'][0], 'fake_a_go_b');
  // 75% vs 70%: a style that prefers rushes does not pick a clearly worse answer…
  assert.equal(counters(t, { rush_b: 3 })['stack_a'][0], 'fake_a_go_b');
  // …but it breaks a real tie (rush_a and rush_b are both 70% vs aggressive_mid).
  assert.equal(counters(t, { rush_b: 3 })['aggressive_mid'][0], 'rush_b');
  assert.equal(counters(t, {})['aggressive_mid'].near, 2);
  // rush_a/b +2 vs aggressive_mid: a style that prefers fakes (only +0) does not change it.
  assert.ok(['rush_a', 'rush_b'].includes(counters(t, { fake_a_go_b: 3 })['aggressive_mid'][0]));
  // Only winning answers are listed; situational calls are skipped.
  const off = counters(PB.nfl.sections[0], {}, ['qb_sneak']);
  assert.ok(!Object.values(off).flat().includes('qb_sneak'));
  assert.deepEqual(off['cover2'], ['run_inside']);
});

test('buildMD: every combination gives a complete, readable .md with the counter tables', async () => {
  const { QUESTIONS, DEFAULTS, buildMD } = await load('arena-builder.js');
  for (const game of ['nfl', 'cs2', 'lol']) {
    const qs = QUESTIONS[game];
    assert.equal(qs.length, 3);
    for (const q of qs) assert.ok(q.options.filter(o => o.rec).length === 1, `${game}/${q.id} has one recommended option`);
    const combos = qs.reduce((acc, q) => acc.flatMap(a => q.options.map(o => ({ ...a, [q.id]: o.id }))), [{}]);
    for (const a of combos) {
      const md = buildMD(game, a, PB[game]);
      const tag = game + ' ' + JSON.stringify(a);
      assert.ok(md.startsWith('# Strategy: "'), tag);
      assert.ok(!/undefined|null|NaN|\[object/.test(md), tag + '\n' + md);
      assert.ok(!md.includes('\n\n\n'), tag);
      assert.ok(md.length > 1200 && md.length < 12000, tag + ' ' + md.length);
      assert.ok(md.includes('## Identity'), tag);
    }
    assert.equal(buildMD(game, DEFAULTS(game), PB[game], 'My "plan"').split('\n')[0], '# Strategy: "My \'plan\'"');
  }
  const nfl = buildMD('nfl', DEFAULTS('nfl'), PB.nfl);
  assert.match(nfl, /\| blitz \| \*\*screen\*\* to rb \|/);
  const cs = buildMD('cs2', { eco: 'aggressive' }, PB.cs2);
  assert.match(cs, /at least \*\*40%\*\*/);
  assert.match(cs, /\| aggressive_mid \| rush_a, or rush_b if our last T round was rush_a \|/);
  const lol = buildMD('lol', { comp: 'early' }, PB.lol);
  assert.match(lol, /home stance is \*\*engage\*\*/);
  assert.match(lol, /- they siege → engage or pick/);
});
