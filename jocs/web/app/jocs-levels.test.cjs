const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const load = f => import(pathToFileURL(path.join(__dirname, f)).href);

test('XP and level come from what the player did', async () => {
  const L = await load('jocs-levels.js');
  assert.equal(L.xpOf(null), 0);
  assert.equal(L.levelOf({}).name, 'Rookie Prompter');
  // 3 reviewed (30) + 2 bots (40) + 1 official win (30) + score 50*2 (100) = 200
  const p = { reviewed: 3, bots_beaten: 2, official: [1, 0, 0], prompt_score: 50 };
  assert.equal(L.xpOf(p), 200);
  const l = L.levelOf(p);
  assert.equal(l.name, 'Tactician');
  assert.equal(l.n, 3);
  assert.equal(l.to, 300);
  assert.equal(l.pct, Math.round(100 * 50 / 150));
});

test('The top level has no next step and a full bar', async () => {
  const L = await load('jocs-levels.js');
  const l = L.levelOf({ reviewed: 20, bots_beaten: 20, official: [10], prompt_score: 90 });
  assert.equal(l.max, true);
  assert.equal(l.pct, 100);
  assert.equal(l.name, 'Grandmaster');
});

test('Challenges are checked against the Prompt check and the stats', async () => {
  const L = await load('jocs-levels.js');
  const { lintMD } = await load('arena-lint.js');
  const weak = lintMD('Be smart with the clock.');
  assert.deepEqual(L.evaluate({ lint: weak, p: null }).done, []);
  const strong = lintMD([
    '# Plan', 'We counter what they repeat.', '## Priorities (the first rule that applies wins)',
    '1. 4th down, under 2 yards: run_inside.', '2. 3rd & 7+: pass_medium; if they blitz, screen.',
    '3. Q4, under 3:00, ahead by 7: run_inside.', '4. If they key on the run more than 60%: pass_short.',
    '5. In the red zone when behind: pass_deep.', '6. Against cover2: pass_medium.', '7. Otherwise: run_outside.', 'x'.repeat(200)].join('\n'));
  const r = L.evaluate({ lint: strong, p: { reviewed: 2, bots_beaten: 1, prompt_score: 72, coach_first: 40, coach_last: 70, official: [1, 0, 0] } });
  assert.equal(r.done.length, L.CHALLENGES.length, 'all done: ' + r.done.join(','));
});

test('Earned challenges stay earned and only new ones are fresh', async () => {
  const L = await load('jocs-levels.js');
  const r = L.evaluate({ lint: null, p: { reviewed: 1 } }, ['read']);
  assert.deepEqual(r.done.sort(), ['first', 'read']);
  assert.deepEqual(r.fresh, ['first']);
});

test('The card renders the level, the bar and the challenge list', async () => {
  const L = await load('jocs-levels.js');
  const html = L.levelCardHTML({ reviewed: 1 }, ['first']);
  assert.match(html, /Prompt level 1/);
  assert.match(html, /Challenges 1\/10/);
  assert.match(html, /Next: Make it measurable/);
  assert.match(L.levelBadgeHTML({}), /Rookie Prompter/);
});
