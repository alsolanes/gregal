const test = require('node:test');
const assert = require('node:assert/strict');
const P = require('./arena-positions.js');

test('every game has positions and every decided field belongs to one position only', () => {
  for (const game of ['nfl', 'cs2', 'lol']) {
    const ps = P.positions(game);
    assert.ok(ps.length >= 4, game);
    const seen = new Set();
    for (const p of ps) for (const f of p.decides || []) {
      assert.ok(!seen.has(f), game + ': ' + f + ' has two owners');
      seen.add(f);
    }
  }
  // Els camps que els motors demanen tenen amo.
  for (const f of ['play', 'target', 'tempo', 'call']) assert.ok(P.positions('nfl').some(p => (p.decides || []).includes(f)), 'nfl ' + f);
  for (const f of ['buy', 'strategy', 'setup', 'utility', 'pace', 'awp']) assert.ok(P.positions('cs2').some(p => (p.decides || []).includes(f)), 'cs2 ' + f);
  for (const f of ['bans', 'picks', 'top', 'mid', 'bot', 'jungle', 'objective', 'focus', 'action', 'stance']) assert.ok(P.positions('lol').some(p => (p.decides || []).includes(f)), 'lol ' + f);
});

test('a team round-trips through the same .md, and the model is told who decides what', () => {
  const md = P.compose('cs2', '# Team Splash\nPlay patient.', { igl: '- Eco after a loss.', awper: '- AWP on full buys.' });
  assert.ok(P.isTeam(md));
  assert.match(md, /AWPer decides `awp`/);
  assert.match(md, /start "rule" with the position/);
  const back = P.parse(md);
  assert.equal(back.head, '# Team Splash\nPlay patient.');
  assert.equal(back.sections.igl, '- Eco after a loss.');
  assert.equal(back.sections.awper, '- AWP on full buys.');
  assert.match(back.sections.support, /no instructions yet/);
  // Recomposar no duplica el preàmbul.
  const again = P.compose('cs2', back.head, back.sections);
  assert.equal(again.split('<!-- team -->').length, 2);
});

test('an old single .md becomes the team plan, and the rule says who decided', () => {
  const md = P.fromSolo('nfl', '# My playbook\n- Run on 1st down.');
  const t = P.parse(md);
  assert.equal(t.head, '# My playbook\n- Run on 1st down.');
  assert.ok(t.sections.qb.length > 0);
  const by = P.byline('nfl', '[Quarterback] Favour wr1 on medium passes');
  assert.equal(by.position.id, 'qb');
  assert.equal(by.text, 'Favour wr1 on medium passes');
  assert.equal(P.byline('nfl', 'no tag').position, null);
});
