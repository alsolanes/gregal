const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

test('session views preserve node identity, scroll position and per-session work state', () => {
  const source = fs.readFileSync(path.join(__dirname, 'index.html'), 'utf8');
  const start = source.indexOf('const sessionViews = new Map();');
  const end = source.indexOf('function onSessionSwitch(', start);
  const main = { childNodes:[], scrollTop:0, classList:{remove(){}}, replaceChildren(...nodes){this.childNodes=nodes;} };
  const sandbox = {document:{dispatchEvent(){}},CustomEvent:class{constructor(type,init){this.type=type;this.detail=init.detail;}},main,hero:{style:{}},state:{},turnGroup:null,turnActivity:null,turnNote:'',turnPhase:'idle',turnT0:0,
    currentView:'agent',todoActual:[],cua:[],sending:false,agentMode:false,goalMode:false,
    stoppedVisualSessions:new Set(),runningAgentSessions:new Set(),runEventHandlers:new Map(),
    setSending(active){sandbox.sending=active;},showMain(){},pintaTodo(){},pintaTurnBar(){} };
  vm.runInNewContext(source.slice(start,end)+'\nglobalThis.views={saveSessionView,restoreSessionView,forgetSessionView};',sandbox);
  const alphaNode={text:'streamed alpha'}, alphaGroup={open:true}, alphaTodo=[{text:'alpha task'}];
  main.childNodes=[alphaNode];main.scrollTop=231;
  sandbox.state={cwd:'alpha',agent_busy:true};sandbox.turnGroup=alphaGroup;sandbox.todoActual=alphaTodo;
  sandbox.runningAgentSessions.add('alpha');sandbox.views.saveSessionView('alpha');
  sandbox.views.restoreSessionView('beta');
  assert.equal(main.childNodes.length,0);
  assert.equal(sandbox.turnGroup,null);
  assert.equal(sandbox.sending,false);
  main.childNodes=[{text:'beta'}];sandbox.state={cwd:'beta'};sandbox.views.saveSessionView('beta');
  sandbox.views.restoreSessionView('alpha');
  assert.equal(main.childNodes[0],alphaNode,'never clone live bubbles or their event listeners');
  assert.equal(main.scrollTop,231);
  assert.equal(sandbox.turnGroup,alphaGroup);
  assert.equal(sandbox.todoActual,alphaTodo);
  assert.equal(sandbox.state.cwd,'alpha');
  assert.equal(sandbox.sending,true);
  sandbox.views.forgetSessionView('alpha');
  assert.equal(sandbox.views.restoreSessionView('alpha'),null);
});
