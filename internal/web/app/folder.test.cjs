// On treballa la sessió (folder.js): nom de la carpeta, worktree i el text
// del botó «Obre a…» segons el sistema; i que les peces hi siguin.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const { folderName, folderInfo, worktreeLabel, openLabelKey } = require('./folder.js');

const tEN = k => ({ 'folder.worktree': 'Worktree: {name} (from {repo})' }[k] || '');
const tCA = k => ({ 'folder.worktree': 'Worktree: {name} (de {repo})' }[k] || '');

test('folder name is the last path segment on Windows and POSIX', () => {
  assert.equal(folderName('C:\\dev\\gregal'), 'gregal');
  assert.equal(folderName('C:\\dev\\gregal\\'), 'gregal');
  assert.equal(folderName('/home/ana/projects/web/'), 'web');
  assert.equal(folderName('C:\\'), 'C:\\');
  assert.equal(folderName('/'), '/');
  assert.equal(folderName(''), '');
  assert.equal(folderName(null), '');
});

test('folder info: git, branch and worktree from /api/state', () => {
  const plain = folderInfo({ cwd: 'C:\\dev\\gregal', branch: 'main', git: true });
  assert.deepEqual(plain, { path: 'C:\\dev\\gregal', name: 'gregal', branch: 'main', git: true, worktree: '', repoName: '', repoPath: '' });
  assert.equal(worktreeLabel(plain, tEN), '');
  const detached = folderInfo({ cwd: '/srv/app', branch: '', git: true });
  assert.equal(detached.git, true);
  assert.equal(detached.branch, '');
  assert.equal(folderInfo({ cwd: '/tmp/notes', branch: '' }).git, false);
  // Sense git al servidor antic però amb branca: és git.
  assert.equal(folderInfo({ cwd: '/srv/app', branch: 'dev' }).git, true);
  const wt = folderInfo({ cwd: 'C:\\dev\\gregal\\.gregal\\worktrees\\feat-x', branch: 'gregal/feat-x', git: true, worktree: 'feat-x', worktree_repo: 'C:\\dev\\gregal' });
  assert.equal(wt.name, 'feat-x');
  assert.equal(wt.repoName, 'gregal');
  assert.equal(worktreeLabel(wt, tEN), 'Worktree: feat-x (from gregal)');
  assert.equal(worktreeLabel(wt, tCA), 'Worktree: feat-x (de gregal)');
  // Sense repo conegut, el nom tot sol.
  assert.equal(worktreeLabel(folderInfo({ cwd: '/x/.gregal/worktrees/a', worktree: 'a' }), tEN), 'Worktree: a');
  assert.deepEqual(folderInfo(null).path, '');
  assert.equal(folderInfo({ cwd: 42, project: 'p' }).name, 'p');
});

test('open-in label follows the platform; the web hides it', () => {
  assert.equal(openLabelKey('win32'), 'folder.openExplorer');
  assert.equal(openLabelKey('darwin'), 'folder.openFinder');
  assert.equal(openLabelKey('linux'), 'folder.openFiles');
  const src = fs.readFileSync(path.join(__dirname, 'folder.js'), 'utf8');
  // Només a l'escriptori (pont openWorkspaceFolder) surt l'acció d'obrir.
  assert.match(src, /canOpen\(\) \? b\('open'/);
  assert.match(src, /openWorkspaceFolder\(window\.gregalSession \|\| 'default'\)/);
});

test('header chip, home line and composer hint are wired with CA/EN strings', async () => {
  const html = fs.readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');
  for (const id of ['folderChip', 'folderChipBtn', 'folderChipMenu', 'hdrWorktree', 'heroFolder', 'hintFolder', 'activeWorkspacePath', 'project', 'branch']) {
    assert.match(html, new RegExp('id="' + id + '"'), id);
  }
  assert.match(html, /<script src="\/app\/folder\.js"><\/script>/);
  assert.match(html, /href="\/app\/folder\.css"/);
  const css = fs.readFileSync(path.join(__dirname, 'folder.css'), 'utf8');
  assert.doesNotMatch(css, /#[0-9a-fA-F]{3,8}\b/, 'theme variables only');
  assert.match(css, /@media \(max-width:480px\)[^]*\.workspace-location \{ display:none; \}/);
  const { CLAUS } = await import('./i18n.js');
  const keys = Object.keys(CLAUS.en).filter(k => k.startsWith('folder.'));
  assert.ok(keys.length >= 15);
  for (const k of keys) assert.ok(CLAUS.ca[k], 'missing CA ' + k);
  for (const k of Object.keys(CLAUS.ca).filter(k => k.startsWith('folder.'))) assert.ok(CLAUS.en[k], 'missing EN ' + k);
  const src = fs.readFileSync(path.join(__dirname, 'folder.js'), 'utf8');
  for (const k of src.matchAll(/T\('(folder\.[A-Za-z]+)'/g)) assert.ok(CLAUS.en[k[1]], 'unknown key ' + k[1]);
});
