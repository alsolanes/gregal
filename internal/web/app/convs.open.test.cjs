const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

async function loadConvs() {
  const stored = new Map();
  const calls = { opens: 0, resumes: 0, switches: [], views: [] };
  const window = { gregalSession: 'default' };
  const sessions = {
    async open() { calls.opens++; window.gregalSession = 'resumed-1'; },
    async list() { return calls.opens ? [{ id: 'resumed-1' }] : [{ id: 'default' }]; },
    async switchTo(id) { calls.switches.push(id); window.gregalSession = id; },
  };
  window.gregalPanels = { sessions };
  window.gregal = {
    isBusy() { return true; }, state: { cwd: 'C:/project' },
    async api(url) {
      if (url === '/api/resume') { calls.resumes++; return { ok: true, async json() { return { title: 'Old task', msgs: 2, transcript: [] }; } }; }
      return { ok: true, async json() { return []; } };
    },
    sys() {}, activityItem() {}, setView(view) { calls.views.push(view); }, refresh() {},
  };
  const document = {getElementById() {return null;}};
  const context = vm.createContext({
    window, localStorage: {
      getItem(key) { return stored.get(key) || null; },
      setItem(key, value) { stored.set(key, String(value)); },
    },
    document,
  });
  const source = fs.readFileSync(path.join(__dirname, 'convs.js'), 'utf8');
  const mod = new vm.SourceTextModule(source, { context, identifier: 'convs.js' });
  await mod.link(() => { throw new Error('convs.js no hauria de tenir imports'); });
  await mod.evaluate();
  return { convs: mod.namespace.convs, calls, window, document };
}

test('reopening a sidebar conversation reuses its tab and returns to the chat', async () => {
  const { convs, calls } = await loadConvs();
  await convs.obre('sessio-20261019-old');
  await convs.obre('sessio-20261019-old');

  assert.equal(calls.opens, 1);
  assert.equal(calls.resumes, 1);
  assert.deepEqual(calls.switches, ['resumed-1']);
  assert.deepEqual(calls.views, ['agent', 'agent']);
});


test('a known live conversation opens immediately without listing saved files', async () => {
  const {convs,calls,window} = await loadConvs();
  convs.items = [{name:'saved-alpha',live_session:'alpha'}];
  window.gregalPanels.sessions._lastList = [{id:'alpha',conversation:'saved-alpha'}];
  convs.load = () => {throw new Error('unnecessary disk/network refresh');};
  await convs.obre('saved-alpha');
  assert.deepEqual(calls.switches,['alpha']);
  assert.equal(calls.resumes,0);
});


test('live session state clears a stale working badge from the saved list', async () => {
  const {convs,document} = await loadConvs();
  const box={innerHTML:'',querySelectorAll(){return [];}};
  document.getElementById=id=>id==='convs'?box:null;
  convs.items=[{name:'saved-alpha',live_session:'alpha',live_busy:true}];
  convs.updateLiveSessions([{id:'alpha',busy:false}]);
  assert.doesNotMatch(box.innerHTML,/conv-live-state busy/);
  convs.updateLiveSessions([{id:'alpha',busy:true}]);
  assert.match(box.innerHTML,/conv-live-state busy/);
});
