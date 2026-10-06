const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

function loadSessions() {
  const stored = new Map([
    ['gregal_session', 'alpha'],
    ['gregal_draft_sessions', JSON.stringify({ alpha: 'draft A', beta: 'draft B' })],
  ]);
  const listeners = {};
  const input = {
    value: '',
    addEventListener(name, fn) { (listeners[name] ||= []).push(fn); },
    dispatchEvent(event) { for (const fn of listeners[event.type] || []) fn(event); },
  };
  const messages = [];
  const main = { innerHTML: '' };
  const window = {
    addEventListener() {},
    gregal: {
      async api(url, opts) {
        (sandbox.requests ||= []).push({ url, opts });
        if (url === '/api/sessions/open') return { ok: true, async json() { return { id: 'new-project' }; } };
        if (url === '/api/sessions/close' && sandbox.closeFails) throw new Error('backend unavailable');
        return { ok: true, async json() { return { sessions: [] }; }, async text() { return ''; } };
      },
      onSessionSwitch() {},
      setView() {},
      async refresh() { return { transcript: [] }; },
      add() {}, md() { return ''; }, sys(message) { messages.push(message); },
      viewActual() { return ''; },
      get state() { return {}; },
    },
  };
  const sandbox = {
    T(key) { return key; },
    window,
    localStorage: {
      getItem(key) { return stored.has(key) ? stored.get(key) : null; },
      setItem(key, value) { stored.set(key, String(value)); },
      removeItem(key) { stored.delete(key); },
    },
    Event: class Event { constructor(type) { this.type = type; } },
    document: { getElementById(id) { return id === 'in' ? input : id === 'main' ? main : null; } },
    closeFails: false,
    alert(message) { messages.push(message); },
  };
  let source = fs.readFileSync(path.join(__dirname, 'panels.js'), 'utf8');
  source = source.replace(/^import .*;\s*$/gm, '');
  source = source.replace(/^export (?=(?:const|function)\b)/gm, '');
  source += '\n;globalThis.__sessions = sessions; globalThis.__workspaces = workspaces; globalThis.__newProject = nouProjecte;';
  vm.runInNewContext(source, sandbox, { filename: 'panels.js' });
  return { sessions: sandbox.__sessions, workspaces: sandbox.__workspaces, newProject: sandbox.__newProject, input, stored, messages, sandbox };
}

test('session drafts restore per tab, save on edits, and clear after submit', async () => {
  const { sessions, input, stored } = loadSessions();
  await sessions.init();
  assert.equal(input.value, 'draft A');

  await sessions.switchTo('beta');
  assert.equal(input.value, 'draft B');
  input.value = 'new beta draft';
  input.dispatchEvent(new Event('input'));

  await sessions.switchTo('alpha');
  assert.equal(input.value, 'draft A');
  await sessions.switchTo('beta');
  assert.equal(input.value, 'new beta draft');

  input.value = '';
  input.dispatchEvent(new Event('input'));
  assert.equal(JSON.parse(stored.get('gregal_draft_sessions')).beta, undefined);
});

test('creating a project opens its folder without changing the previous session', async () => {
  const { workspaces, sandbox, sessions } = loadSessions();
  assert.equal(await workspaces.set('C:/fixture/new-project', { create: true }), true);
  const requests = sandbox.requests;
  assert.equal(requests.some(r => r.url === '/api/workspaces'), false);
  assert.equal(JSON.parse(requests.find(r => r.url === '/api/sessions/open').opts.body).cwd, 'C:/fixture/new-project');
  assert.equal(sessions.current, 'new-project');
});

test('native Project selection does not mutate the previous session', async () => {
  const { newProject, sandbox } = loadSessions();
  sandbox.window.gregalDesktop = { async chooseFolder() { return 'C:/fixture/native-project'; } };
  await newProject();
  assert.equal(sandbox.requests.some(r => r.url === '/api/workspaces'), false);
  assert.equal(JSON.parse(sandbox.requests.find(r => r.url === '/api/sessions/open').opts.body).cwd, 'C:/fixture/native-project');
});

test('failed project selection reports failure and does not switch sessions', async () => {
  const { workspaces, sandbox, sessions, messages } = loadSessions();
  sandbox.window.gregal.api = async () => ({ ok: false, async text() { return 'folder unavailable'; } });
  assert.equal(await workspaces.set('C:/fixture/missing', { create: true }), false);
  assert.equal(sessions.current, 'default');
  assert.match(messages[0], /folder unavailable/);
});

test('a failed session close leaves the current session open and reports the error', async () => {
  const { sessions, input, messages, sandbox } = loadSessions();
  await sessions.init();
  await sessions.switchTo('beta');
  const before = input.value;
  sessions._lastList = [{ id: 'beta' }];
  sessions._error = '';

  sandbox.closeFails = true;
  await sessions.close('beta');
  assert.equal(sessions.current, 'beta');
  assert.equal(input.value, before);
  assert.match(messages.at(-1), /backend unavailable/);
});
