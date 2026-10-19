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
      async refresh() { return sandbox.refreshState || { transcript: [] }; },
      add(role, _className, content) { messages.push({ role, content }); return null; }, md(content) { return content; }, sys(message) { messages.push(message); },
      toolCard(name, args) { const card = { name, args, classList: { remove() {}, add() {} }, querySelector() { return null; } }; messages.push({ role: 'tool-call', name, args }); return card; },
      toolCardDone(_card, name, output) { messages.push({ role: 'tool-result', name, output }); },
      intoWorkGroup() {}, beginWorkGroup() { return {}; }, endWorkGroup() {},
      viewActual() { return ''; },
      get state() { return {}; },
    },
  };
  const sandbox = {
    T(key) { return key; },
    convs: { forgetSession() {} },
    window,
    localStorage: {
      getItem(key) { return stored.has(key) ? stored.get(key) : null; },
      setItem(key, value) { stored.set(key, String(value)); },
      removeItem(key) { stored.delete(key); },
    },
    Event: class Event { constructor(type) { this.type = type; } },
    document: { getElementById(id) { return id === 'in' ? input : id === 'main' ? main : null; } },
    confirm(message) { (sandbox.confirmations ||= []).push(message); return sandbox.confirmResult !== false; },
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

test('returning to a busy session makes its background work visible', async () => {
  const { sessions, messages, sandbox } = loadSessions();
  sandbox.refreshState = { agent_busy: true, transcript: [{ role: 'user', content: 'finish the task' }] };

  await sessions.switchTo('alpha');

  assert.ok(messages.includes('sessions.stillWorking'));
});

test('switching to a session restores tool calls and their results with the transcript', async () => {
  const { sessions, messages, sandbox } = loadSessions();
  sandbox.refreshState = { transcript: [
    { role: 'user', content: 'inspect this project' },
    { role: 'assistant', content: '', tool_calls: [{ id: 'call-1', function: { name: 'read', arguments: '{"path":"README.md"}' } }] },
    { role: 'tool', tool_call_id: 'call-1', name: 'read', content: 'project notes' },
    { role: 'assistant', content: 'The project is ready.' },
  ] };

  await sessions.switchTo('alpha');

  assert.ok(messages.some(m => m && m.role === 'tool-call' && m.name === 'read'));
  assert.ok(messages.some(m => m && m.role === 'tool-result' && m.output === 'project notes'));
  assert.ok(messages.some(m => m && m.role === 'a' && m.content === 'The project is ready.'));
});

test('a busy session can be closed after confirming that its work will stop', async () => {
  const { sessions, sandbox } = loadSessions();
  sessions._lastList = [{ id: 'beta', busy: true }];
  sandbox.confirmResult = false;

  await sessions.close('beta');
  assert.equal((sandbox.requests || []).some(r => r.url === '/api/sessions/close'), false);

  sandbox.confirmResult = true;
  await sessions.close('beta');
  assert.equal((sandbox.requests || []).some(r => r.url === '/api/sessions/close'), true);
  assert.equal(sandbox.confirmations.length, 2);
});
