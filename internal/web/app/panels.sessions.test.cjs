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
        if (url === '/api/active') return { ok: true, async json() { return sandbox.activeResponse || { active: null }; } };
        if (url.startsWith('/api/v2/events?')) return { ok: true, async json() { return sandbox.eventResponse || { events: [], next: 0 }; } };
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

test('returning to a busy session replays that run from the durable v2 event log', async () => {
  const { sessions, messages, sandbox } = loadSessions();
  sandbox.refreshState = { agent_busy: true, transcript: [{ role: 'user', content: 'important task' }] };
  sandbox.activeResponse = { active: { id: 7, task: 'important task' } };
  sandbox.eventResponse = { next: 3, events: [
    { id: 1, run_id: 7, kind: 'token', text: 'Earlier visible progress' },
    { id: 2, run_id: 8, kind: 'token', text: 'Another session' },
    { id: 3, run_id: 7, kind: 'thinking', text: '💭 Checking the project', payload: { text: 'Checking the project' } },
  ] };

  await sessions.switchTo('alpha');

  assert.ok(sandbox.requests.some(r => r.url.startsWith('/api/v2/events?session=alpha')));
  assert.ok(messages.some(m => m && m.role === 'a' && m.content === 'Earlier visible progress'));
  assert.ok(messages.some(m => m && m.role === 't' && /Checking the project/.test(m.content)));
  assert.ok(!messages.some(m => m && m.role === 'a' && m.content === 'Another session'));
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


test('late responses cannot overwrite a newer A-B-A navigation', async () => {
  const {sessions,messages,sandbox} = loadSessions();
  const pending = [];
  sandbox.window.gregal.refresh = () => new Promise(resolve => pending.push(resolve));
  const oldA = sessions.switchTo('alpha');
  const beta = sessions.switchTo('beta');
  const newA = sessions.switchTo('alpha');
  pending[2]({transcript:[{role:'assistant',content:'newest alpha'}]});
  await newA;
  pending[0]({transcript:[{role:'assistant',content:'stale alpha'}]});
  pending[1]({transcript:[{role:'assistant',content:'stale beta'}]});
  await Promise.all([oldA,beta]);
  assert.equal(sessions.current,'alpha');
  assert.deepEqual(messages.filter(m=>m?.role==='a').map(m=>m.content),['newest alpha']);
});

test('cached history and buffered progress restore before a slow state request', async () => {
  const {sessions,sandbox} = loadSessions();
  const main = sandbox.document.getElementById('main');
  let resolveState, flushed = false;
  sandbox.window.gregal.restoreSessionView = () => {
    main.innerHTML = 'cached history';
    return {state:{transcript:[]},nodes:[]};
  };
  sandbox.window.gregalRunEventHandlers = new Map([['alpha',{flushPending(){flushed=true;}}]]);
  sandbox.window.gregal.refresh = () => new Promise(resolve => {resolveState=resolve;});
  const switching = sessions.switchTo('alpha');
  assert.equal(main.innerHTML,'cached history');
  assert.equal(flushed,true);
  resolveState({agent_busy:true,transcript:[]});
  await switching;
  assert.equal(main.innerHTML,'cached history');
  assert.equal((sandbox.requests||[]).some(r=>r.url.includes('/api/v2/events')),false);
});

test('clicking the current loaded tab keeps its stream and history intact', async () => {
  const {sessions,sandbox} = loadSessions();
  await sessions.switchTo('alpha');
  sandbox.window.gregal.refresh = () => {throw new Error('unexpected reload');};
  await sessions.switchTo('alpha');
  assert.equal(sessions._loadedSession,'alpha');
});


test('a queued or externally changed session is followed without a second run', async () => {
  const {sessions,sandbox} = loadSessions();
  sessions.current='alpha';
  const visits=[];
  sessions.switchTo=async(id,opts)=>visits.push({id,force:opts.force});
  await sessions.syncCurrent([{id:'alpha',busy:true,msgs:1}]);
  assert.deepEqual(visits,[{id:'alpha',force:true}]);
  sandbox.window.gregalRunEventHandlers=new Map([['alpha',{}]]);
  await sessions.syncCurrent([{id:'alpha',busy:true,msgs:1}]);
  assert.equal(visits.length,1,'keep the existing live subscription');
  sandbox.window.gregalRunEventHandlers.clear();
  await sessions.syncCurrent([{id:'alpha',busy:false,msgs:2}]);
  assert.equal(visits.length,2,'fetch a completed turn from another client');
});


test('recovery includes an already-finished run terminal without stale questions', async () => {
  const {sessions,sandbox} = loadSessions();
  sandbox.refreshState={agent_busy:true,transcript:[{role:'user',content:'important task'}]};
  sandbox.activeResponse={active:{id:7,task:'important task'},pending_interactions:['q2']};
  sandbox.eventResponse={next:103,events:[
    {id:100,run_id:7,kind:'question_request',payload:{key:'q1',query:'expired'}},
    {id:101,run_id:7,kind:'question_request',payload:{key:'q2',query:'still pending'}},
    {id:102,run_id:7,kind:'text',text:'completed while history loaded'},
    {id:103,run_id:7,kind:'run_completed'},
  ]};
  let attached;
  sandbox.window.gregal.attachActiveRun=(active,restored,after,pending)=>{attached={active,restored,after,pending};};
  await sessions.switchTo('alpha');
  assert.equal(attached.after,102,'subscribe before the terminal so recovery cannot wait forever');
  assert.equal(attached.pending.length,0);
});


test('an attached live run restores only questions the server still accepts', async () => {
  const {sessions,sandbox} = loadSessions();
  sandbox.refreshState={agent_busy:true,transcript:[]};
  sandbox.activeResponse={active:{id:7,task:'important task'},pending_interactions:['q2']};
  sandbox.eventResponse={next:101,events:[
    {id:100,run_id:7,kind:'question_request',payload:{key:'q1',query:'expired'}},
    {id:101,run_id:7,kind:'question_request',payload:{key:'q2',query:'still pending'}},
  ]};
  let pending;
  sandbox.window.gregal.attachActiveRun=(_active,_restored,_after,questions)=>{pending=questions;};
  await sessions.switchTo('alpha');
  assert.equal(pending.length,1);
  assert.equal(pending[0].value.key,'q2');
});
