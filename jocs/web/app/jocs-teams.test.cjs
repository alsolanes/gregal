const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const load = f => import(pathToFileURL(path.join(__dirname, f)).href);

const ROWS = [
  { name: 'Data', rank: 1, members: 2, active: 2, xp: 150, prompt_score: 55, improvement: 24, vs_vague: { played: 4, won: 3 }, official_wins: 1, top: 'Ana' },
  { name: 'Cloud <b>', rank: 2, members: 1, active: 0, xp: 0, prompt_score: 0, improvement: 0, vs_vague: { played: 0, won: 0 }, official_wins: 0 },
];

test('The team table shows the averages and marks my team', async () => {
  const T = await load('jocs-teams.js');
  const html = T.teamsTableHTML(ROWS, 'Data');
  assert.match(html, /YOUR TEAM/);
  assert.match(html, /75%/);
  assert.match(html, /\+24/);
  assert.match(html, /class="pos">1</);
  assert.equal(html.includes('<b>Cloud <b></b>'), false, 'team names are escaped');
  assert.match(T.teamsTableHTML([], ''), /No teams yet/);
});

test('The picker only appears with teams and keeps my choice', async () => {
  const T = await load('jocs-teams.js');
  assert.equal(T.teamPickerHTML([], ''), '');
  assert.match(T.teamPickerHTML(['Data', 'Cloud'], 'Cloud'), /<option selected>Cloud<\/option>/);
});

test('The organiser tools list people with their team and the report button', async () => {
  const T = await load('jocs-teams.js');
  const html = T.organizerTeamsHTML(['Data', 'Cloud'], { ana: 'Cloud' }, [{ owner: 'ana', display: 'Ana' }, { owner: 'local', display: 'local' }]);
  assert.match(html, /Data\nCloud<\/textarea>/);
  assert.match(html, /data-team-owner="ana"/);
  assert.equal(html.includes('data-team-owner="local"'), false);
  assert.match(html, /Download the report/);
  assert.equal(T.organizerTeamsHTML([], {}, [{ owner: 'ana', display: 'Ana' }]).includes('Who is in which team'), false);
});
