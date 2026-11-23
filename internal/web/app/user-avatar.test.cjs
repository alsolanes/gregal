const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { initials, displayName } = require('./user-avatar.js');

test('the avatar shows up to two initials of the name', () => {
  assert.equal(initials('Aleix Solanes'), 'AS');
  assert.equal(initials('anna'), 'A');
  assert.equal(initials('Tu'), 'T');
  assert.equal(initials('  maria del mar puig  '), 'MP');
  assert.equal(initials('Èlia'), 'È');
  assert.equal(initials(''), '');
});

test('an empty name falls back to the account, or to «Tu»/«You» in local mode', () => {
  assert.equal(displayName({ saved: 'Aleix', account: 'asolanes', local: false }, 'ca'), 'Aleix');
  assert.equal(displayName({ saved: '  ', account: 'asolanes', local: false }, 'ca'), 'asolanes');
  assert.equal(displayName({ saved: '', account: '', local: true }, 'ca'), 'Tu');
  assert.equal(displayName({ saved: null, account: '', local: true }, 'en'), 'You');
});

test('the sidebar foot holds the person, not the model name', () => {
  const html = fs.readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');
  const start = html.indexOf('<div class="side-foot">');
  const foot = html.slice(start, html.indexOf('</aside>', start));
  assert.match(foot, /id="sideMe"/);
  assert.match(foot, /id="statusDot"/);
  assert.doesNotMatch(foot, /id="endpoint"/);
  assert.doesNotMatch(html, /\$\('endpoint'\)/, 'nothing writes the model name into the foot');
});
