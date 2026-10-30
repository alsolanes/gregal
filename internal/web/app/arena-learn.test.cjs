const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const load = f => import(pathToFileURL(path.join(__dirname, f)).href);

const MD = [
  '# Identity',
  'Run the ball and control the clock.',
  '',
  '## Offense',
  '- On 3rd & long: pass_medium to wr1.',
  '- Inside the red zone: run_inside.',
  '',
  '## Defense',
  '- If they pass more than 60%: cover3.',
].join('\n');

test('Apply fix: replaces the rule the coach quotes, and undo puts it back', async () => {
  const L = await load('arena-learn.js');
  const issue = 'The rule «On 3rd & long: pass_medium to wr1» ignores the blitz.';
  const fix = 'On 3rd & long: screen to the rb against a blitz, pass_medium otherwise.';
  const { md, change } = L.applyFix(MD, fix, { issue });
  assert.equal(change.kind, 'replace');
  assert.equal(change.start, 4);
  assert.deepEqual(change.old, ['- On 3rd & long: pass_medium to wr1.']);
  assert.deepEqual(change.lines, ['- ' + fix], 'keeps the list marker');
  assert.equal(md.split('\n')[4], '- ' + fix);
  assert.equal(md.split('\n').length, MD.split('\n').length);
  assert.equal(L.undoFix(md, change), MD);
  // Applying it again: already there.
  assert.equal(L.applyFix(md, fix, { issue }).change.kind, 'none');
});

test('Apply fix: a similar rule without a quote is replaced; a related one gets the new rule below it', async () => {
  const L = await load('arena-learn.js');
  const r = L.applyFix(MD, 'If they pass more than 50% of plays: cover3 on every down.');
  assert.equal(r.change.kind, 'replace');
  assert.equal(r.change.start, 8);
  assert.match(r.md.split('\n')[8], /^- If they pass more than 50%/);
  // Same topic (red zone) but a different rule: inserted after the list it belongs to.
  const near = L.applyFix(MD, 'Red zone on 4th down: field_goal unless trailing by 4+.');
  assert.equal(near.change.kind, 'insert');
  assert.equal(near.change.start, 6, 'at the end of the Offense list');
  assert.deepEqual(near.md.split('\n').slice(4, 7), ['- On 3rd & long: pass_medium to wr1.', '- Inside the red zone: run_inside.', '- Red zone on 4th down: field_goal unless trailing by 4+.']);
  assert.equal(L.undoFix(near.md, near.change), MD);
});

test('Apply fix: an unrelated rule goes to a «Coach fixes» section at the end (created once)', async () => {
  const L = await load('arena-learn.js');
  const a = L.applyFix(MD + '\n\n', 'Two-minute drill: tempo hurry, spike to stop the clock.');
  assert.equal(a.change.kind, 'insert');
  assert.deepEqual(a.change.lines, ['', '## Coach fixes', '- Two-minute drill: tempo hurry, spike to stop the clock.']);
  assert.ok(a.md.includes('## Coach fixes\n- Two-minute drill'));
  const b = L.applyFix(a.md, 'Kneel when leading in the last 40 seconds.');
  assert.deepEqual(b.change.lines, ['- Kneel when leading in the last 40 seconds.'], 'the section already exists');
  assert.equal(b.md.match(/## Coach fixes/g).length, 1);
  assert.ok(b.md.includes('stop the clock.\n- Kneel when leading'));
  // Undo in a different order still works (the lines are searched for).
  const back = L.undoFix(b.md, a.change);
  assert.ok(!back.includes('Two-minute'), 'first fix removed');
  assert.ok(back.includes('Kneel'), 'second kept');
  assert.equal(L.undoFix('totally different', b.change), null, 'gone: cannot undo');
  // Multi-line fixes keep their own markers; empty fixes do nothing.
  const c = L.applyFix('', '- one rule here\n- another rule there');
  assert.deepEqual(c.change.lines, ['## Coach fixes', '- one rule here', '- another rule there']);
  assert.equal(L.applyFix(MD, '  ').change.kind, 'none');
});

test('diff: line by line with folded context', async () => {
  const L = await load('arena-learn.js');
  const a = 'a\nb\nc\nd\ne\nf\ng\nh';
  const b = 'a\nb\nC\nd\ne\nf\ng\nh\ni';
  const ops = L.diffLines(a, b);
  assert.deepEqual(ops.filter(o => o.op !== ' ').map(o => o.op + o.text), ['-c', '+C', '+i']);
  assert.deepEqual(L.diffCount(ops), { add: 2, del: 1 });
  // Unchanged lines rebuild both sides.
  assert.equal(ops.filter(o => o.op !== '+').map(o => o.text).join('\n'), a);
  assert.equal(ops.filter(o => o.op !== '-').map(o => o.text).join('\n'), b);
  const h = L.diffHunks(ops, 1);
  assert.deepEqual(h.map(o => o.op === '…' ? '…' + o.skip : o.op + o.text), ['…1', ' b', '-c', '+C', ' d', '…3', ' h', '+i']);
  assert.deepEqual(L.diffLines('same', 'same'), [{ op: ' ', text: 'same' }]);
  assert.deepEqual(L.diffHunks(L.diffLines('x\ny\nz\nw', 'x\ny\nz\nw'), 1), [{ op: '…', skip: 4 }]);
  // Moved block: LCS keeps the longest common run.
  const mv = L.diffLines('1\n2\n3\n4', '3\n4\n1\n2');
  assert.equal(L.diffCount(mv).add, 2);
  assert.equal(L.diffCount(mv).del, 2);
});

test('versions: each match goes to the version saved before it started', async () => {
  const L = await load('arena-learn.js');
  const history = [{ at: 100 }, { at: 200 }, { at: 300, current: true }];
  const games = [
    { id: 'old', started: 50, result: 'loss' },
    { id: 'a1', started: 100, result: 'loss', coach: 10 },
    { id: 'a2', started: 150, result: 'win', coach: 14 },
    { id: 'b1', started: 299, result: 'draw' },
    { id: 'c1', started: 300, result: 'win', coach: 20 },
    { id: 'c2', started: 999, result: 'win', coach: 21 },
    { id: 'bad' },
  ];
  const by = L.matchesByVersion(history, games);
  assert.deepEqual(by.map(l => l.map(g => g.id)), [['a1', 'a2'], ['b1'], ['c1', 'c2']]);
  assert.deepEqual(L.versionStats(by[0]), { n: 2, w: 1, l: 1, d: 0, coach: 12 });
  assert.deepEqual(L.versionStats(by[1]), { n: 1, w: 0, l: 0, d: 1, coach: null });
  assert.deepEqual(L.versionStats(by[2]), { n: 2, w: 2, l: 0, d: 0, coach: 20.5 });
  assert.deepEqual(L.matchesByVersion([], games), []);
});

test('last match and its top mistakes', async () => {
  const L = await load('arena-learn.js');
  const rows = [{ id: 'x', kind: 'practice', started: 5 }, { id: 'y', kind: 'official', started: 9 }, { id: 'z', kind: 'practice', started: 7 }];
  assert.equal(L.lastPractice(rows).id, 'z');
  assert.equal(L.lastPractice([]), null);
  const f = { scores: { clarity: 3, coverage: 4, adaptivity: 2, specificity: 5, consistency: 3 }, mistakes: [{ what: 'a' }, { what: 'b', fix: 'B' }, { what: 'c', fix: 'C' }, { what: 'd', fix: 'D' }, null] };
  assert.deepEqual(L.topMistakes(f).map(m => m.what), ['b', 'c', 'd'], 'the ones with a rewritten rule first');
  assert.deepEqual(L.topMistakes(f, 4).map(m => m.what), ['b', 'c', 'd', 'a']);
  assert.equal(L.coachScore(f), 17);
  assert.equal(L.coachScore(null), null);
});

test('Apply fix: the rule is taken out of «Add: …» / «Replace with: …» and quotes', async () => {
  const L = await load('arena-learn.js');
  assert.deepEqual(L.parseFix("Add: 'Max 2 stacks in a row on the same site.'"), { rule: 'Max 2 stacks in a row on the same site.', intent: 'add' });
  assert.deepEqual(L.parseFix("Afegeix: 'Si l'enemic fa stack, canvia de site.'"), { rule: "Si l'enemic fa stack, canvia de site.", intent: 'add' });
  assert.deepEqual(L.parseFix("Replace with: 'Full-buy if team money ≥ $20,000.'"), { rule: 'Full-buy if team money ≥ $20,000.', intent: 'replace' });
  assert.deepEqual(L.parseFix("Reescriu: 'Force buy només si team avg < $3.000.'"), { rule: 'Force buy només si team avg < $3.000.', intent: 'replace' });
  assert.deepEqual(L.parseFix("Add a pistol-round override: 'Pistol round: buy full.'"), { rule: 'Pistol round: buy full.', intent: 'add' });
  assert.deepEqual(L.parseFix("'Si has fet stack 2 rondes, canvia.'"), { rule: 'Si has fet stack 2 rondes, canvia.', intent: '' });
  assert.equal(L.parseFix("'Si la stance és la mateixa que l'última, canvia a protect.'").rule, "Si la stance és la mateixa que l'última, canvia a protect.", 'apostrophes before accented letters');
  assert.equal(L.parseFix(`"Alterna com a màxim 2 crides de 'man'; a la tercera, 'cover2'."`).rule, "Alterna com a màxim 2 crides de 'man'; a la tercera, 'cover2'.");
  // A plain rule with short quoted words stays as it is.
  const plain = "On pistol rounds: buy kevlar, utility 'flash_heavy' on T and 'normal' on CT.";
  assert.deepEqual(L.parseFix(plain), { rule: plain, intent: '' });
  // Quotes in the issue: single quotes too, but not apostrophes.
  assert.deepEqual(L.quotes("The .md says 'Be smart on fourth down and take the points when you can,' but gives no guidance; don't guess."), ['Be smart on fourth down and take the points when you can,']);
  assert.deepEqual(L.quotes("'Buy when you can' és ambigu: l'equip no sap què fer"), ['Buy when you can']);
});

test('Apply fix: «Add» inserts next to the quoted rule; a quoted rule is replaced', async () => {
  const L = await load('arena-learn.js');
  const md = '# Plan\n\n- Mix it up.\n- Be smart on fourth down and take the points when you can.\n- Protect the lead late.';
  const issue = "The .md says 'Be smart on fourth down and take the points when you can,' but gives no guidance.";
  const add = L.applyFix(md, "Add a rule: 'On 4th & 3 or less past the 40, go for it with qb_sneak.'", { issue });
  assert.equal(add.change.kind, 'insert');
  assert.equal(add.change.start, 5, 'after the list the quoted rule is in');
  assert.deepEqual(add.change.lines, ['- On 4th & 3 or less past the 40, go for it with qb_sneak.']);
  const rep = L.applyFix(md, 'On 4th & 3 or less past the 40, go for it; otherwise punt or kick.', { issue });
  assert.equal(rep.change.kind, 'replace');
  assert.deepEqual(rep.change.old, ['- Be smart on fourth down and take the points when you can.']);
  assert.equal(L.undoFix(rep.md, rep.change), md);
});
