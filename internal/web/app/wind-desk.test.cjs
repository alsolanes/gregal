const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const listeners = {};
const root = { document:{getElementById(){return null;},addEventListener(type,fn){listeners[type]=fn;}} };
vm.runInNewContext(fs.readFileSync(path.join(__dirname,'wind-desk.js'),'utf8'),{window:root});
const flowFor=root.gregalWindDesk.flowFor;
const owner={id:'tramuntana',name:'Tramuntana',role:'webs i codi',persona:'Precís i curiós'};
const reviewer={id:'mestral',name:'Mestral',role:'revisió',persona:'Pacient'};

test('wind work routes the deliverable into an independent bounded review and revision',()=>{
  const flow=flowFor(owner,reviewer);
  assert.equal(flow.output,'reviewer');
  assert.equal(flow.max_rounds,1);
  assert.deepEqual(Array.from(flow.nodes,n=>n.role),['builder','reviewer']);
  assert.equal(flow.nodes[1].mode,'review');
  assert.match(flow.nodes[0].instruction,/Tramuntana.*webs i codi.*Precís/s);
  assert.match(flow.nodes[1].instruction,/Mestral.*required review verdict/s);
  assert.deepEqual(Array.from(flow.edges,e=>`${e.from}:${e.to}:${e.kind}`),['builder:reviewer:data','reviewer:builder:revision']);
});
test('research uses the read-only researcher seat and a solo task has no fictitious review',()=>{
  const flow=flowFor(owner,null,'research');
  assert.equal(flow.nodes[0].role,'researcher');
  assert.equal(flow.nodes.length,1);
  assert.equal(flow.edges.length,0);
  assert.equal(flow.output,'builder');
});
test('missing owner and self review cannot launch a model request',()=>{
  assert.throws(()=>flowFor(null,reviewer),/Tria un vent/);
  assert.throws(()=>flowFor(owner,owner),/altre vent/);
});
test('task flow name uses the active language and wind name interpolation',()=>{
  root.gregalWindI18n={text(key,fallback,vars){
    if(key==='winds.taskBy')return new String(`Task for ${vars.name}`);
    return fallback;
  }};
  assert.equal(flowFor(owner).name,'Task for Tramuntana');
  delete root.gregalWindI18n;
  assert.equal(flowFor(owner).name,'Encàrrec de Tramuntana');
});
test('locale changes do not reach into bind-local example controls',()=>{
  assert.doesNotThrow(()=>listeners['gregal:idioma']());
});
const resolveTask=root.gregalWindDesk.resolveTask;
const research={id:'marinada',name:'Marinada',role:'research and content'};
test('a website selects the builder and independent review without configuring a flow',()=>{
  const selected=resolveTask([research,owner,reviewer],'Crea una web per a una cafeteria');
  assert.equal(selected.owner.id,owner.id);
  assert.equal(selected.reviewer.id,reviewer.id);
  assert.equal(selected.kind,'build');
});
test('research is read-only and avoids an unnecessary review pass',()=>{
  const selected=resolveTask([owner,research,reviewer],'Compare local coffee suppliers');
  assert.equal(selected.owner.id,research.id);
  assert.equal(selected.reviewer,null);
  assert.equal(selected.kind,'research');
});
test('mixed research and implementation remains capable of writing files',()=>{
  assert.equal(resolveTask([owner,research],'Investiga i crea una web').kind,'build');
});
test('manual choices and no-review override automatic routing',()=>{
  const selected=resolveTask([research,owner,reviewer],'Create a website',{owner:research.id,reviewer:'',kind:'research'});
  assert.equal(selected.owner.id,research.id);
  assert.equal(selected.reviewer,null);
  assert.equal(selected.kind,'research');
  assert.equal(resolveTask([owner],'Build a website').reviewer,null);
});

test('data analysis selects the matching specialist and can save results for independent review',()=>{
  const roster=[{id:'quality',name:'Marinada',squad:'data',specialty:'quality'},{id:'stats',name:'Llevant',squad:'data',specialty:'regression'},{id:'time',name:'Migjorn',squad:'data',specialty:'timeseries'},{id:'review',name:'Mestral',squad:'data',specialty:'review'}];
  const regression=resolveTask(roster,'Analitza aquestes dades amb regressions');
  assert.equal(regression.owner.id,'stats');assert.equal(regression.kind,'build');assert.equal(regression.reviewer.id,'review');
  assert.equal(resolveTask(roster,'Analitza aquesta sèrie temporal de dades').owner.id,'time');
});
