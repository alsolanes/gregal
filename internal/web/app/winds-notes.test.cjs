// Funcions pures de les notes a la web (winds-notes.js).
const test = require('node:test');
const assert = require('node:assert/strict');
const N = require('./winds-notes.js');
const md = require('./md.js');

test('[[Títol]] es converteix en enllaç de nota, però no dins del codi', () => {
  const html = N.linkNotes(md.render('Mira [[Viatge a Menorca]] i [[llista|la llista]].\n\n`[[codi]]`'));
  assert.match(html, /<a href="#" class="note-link" data-note="Viatge a Menorca">Viatge a Menorca<\/a>/);
  assert.match(html, /data-note="llista">la llista<\/a>/);
  assert.match(html, /<code>\[\[codi\]\]<\/code>/);
});

test('el títol de l’enllaç no pot injectar HTML', () => {
  const html = N.linkNotes(md.render('[[a" onclick="x]]'));
  assert.doesNotMatch(html, /onclick="x"/);
  assert.match(html, /data-note="a&quot; onclick=&quot;x"/);
});

test('toggleTask marca només la tasca que toca i salta els blocs de codi', () => {
  const body = '- [ ] pa\n```\n- [ ] no\n```\n- [x] llet\n* [ ] ous';
  assert.equal(N.toggleTask(body, 0), '- [x] pa\n```\n- [ ] no\n```\n- [x] llet\n* [ ] ous');
  assert.equal(N.toggleTask(body, 1), '- [ ] pa\n```\n- [ ] no\n```\n- [ ] llet\n* [ ] ous');
  assert.equal(N.toggleTask(body, 2), '- [ ] pa\n```\n- [ ] no\n```\n- [x] llet\n* [x] ous');
});

test('parseTags neteja, treu # i no repeteix (sense accents ni majúscules)', () => {
  assert.deepEqual(N.parseTags('música, #Pendents ; musica\nidees noves'), ['música', 'Pendents', 'idees-noves']);
  assert.deepEqual(N.parseTags(''), []);
});

test('dayText: hora avui, «ahir» i data curta', () => {
  const now = new Date(2026, 10, 6, 18, 0);
  assert.match(N.dayText(new Date(2026, 10, 6, 9, 5).toISOString(), 'ca', now), /09:05/);
  assert.equal(N.dayText(new Date(2026, 10, 5, 9, 5).toISOString(), 'ca', now), 'ahir');
  assert.equal(N.dayText(new Date(2026, 10, 5, 9, 5).toISOString(), 'en', now), 'yesterday');
  assert.equal(N.dayText('', 'ca', now), '');
});

test('agendaGroups reparteix vençudes, avui, setmana i més endavant', () => {
  const now = new Date(2026, 10, 6, 10, 0);
  const g = N.agendaGroups([{ due: '2026-11-01' }, { due: '2026-11-06' }, { due: '2026-11-13' }, { due: '2026-11-14' }, { due: '' }], now);
  assert.deepEqual([g.overdue.length, g.today.length, g.week.length, g.later.length], [1, 1, 1, 1]);
});

test('meetingTemplate porta la data i una tasca amb termini', () => {
  const m = N.meetingTemplate('ca', new Date(2026, 10, 6));
  assert.equal(m.notebook, 'Reunions');
  assert.match(m.body, /\*\*Data:\*\* 2026-11-06/);
  assert.match(m.body, /- \[ \] {2}@2026-11-13/);
  assert.match(N.meetingTemplate('en', new Date(2026, 10, 6)).body, /Action items/);
});
