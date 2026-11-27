const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

// Small DOM boundary, as in the existing web tests; no npm/browser server needed.
class Element {
  constructor(tag, doc) { this.tagName = tag.toUpperCase(); this.doc = doc; this.children = []; this.attributes = {}; this.style = { setProperty(k,v) { this[k]=v; } }; this.listeners = {}; this.value = ''; this.hidden = false; this.disabled = false; this._text = ''; }
  setAttribute(k,v) { this.attributes[k] = String(v); if (k === 'id') this.id = String(v); if (k === 'class') this.className = String(v); }
  getAttribute(k) { return this.attributes[k] ?? null; }
  removeAttribute(k) { delete this.attributes[k]; }
  append(...nodes) { for (const n of nodes) { const e = typeof n === 'string' ? Object.assign(new Element('text',this.doc),{_text:n}) : n; e.parentNode=this; this.children.push(e); } }
  appendChild(n) { this.append(n); return n; }
  replaceChildren(...nodes) { this.children=[]; this._text=''; this.append(...nodes); }
  get textContent() { return this._text + this.children.map(n=>n.textContent).join(''); }
  set textContent(v) { this.children=[]; this._text=String(v); }
  get classList() { return { add: c => { this.className = ((this.className||'')+' '+c).trim(); }, remove: c => { this.className=(this.className||'').split(' ').filter(x=>x!==c).join(' '); }, toggle: (c,on) => on ? this.classList.add(c) : this.classList.remove(c) }; }
  matches(s) { if (s.startsWith('#')) return this.id===s.slice(1); if (s.startsWith('.')) return (this.className||'').split(' ').includes(s.slice(1)); const a=s.match(/^\[([^=\]]+)(?:="?([^"\]]+)"?)?\]$/); if(a) return a[2] === undefined ? this.getAttribute(a[1])!==null : this.getAttribute(a[1])===a[2]; return this.tagName.toLowerCase()===s; }
  querySelectorAll(s) { return this.children.flatMap(n=>[...(s.split(',').some(x=>n.matches(x.trim()))?[n]:[]),...n.querySelectorAll(s)]); }
  querySelector(s) { return this.querySelectorAll(s)[0]||null; }
  contains(n) { return n===this || this.children.some(c=>c.contains(n)); }
  addEventListener(k,f) { (this.listeners[k] ||= []).push(f); }
  dispatchEvent(e) { this.emit(e.type,e); }
  async emit(k,props={}) { const e={target:this,key:'',preventDefault(){},...props}; if (this['on'+k]) await this['on'+k](e); for(const fn of this.listeners[k]||[]) await fn(e); }
  async click() { if (!this.disabled) await this.emit('click'); }
  focus() { this.doc.activeElement=this; }
  showModal() { this.open=true; }
  close() { this.open=false; this.emit('close'); }
  scrollIntoView() {}
  getBoundingClientRect() { return { top:0 }; }
}
function fixture(apiOverride) {
  const doc={activeElement:null,createElement:t=>Object.assign(new Element(t,doc),t==='canvas'?{getContext:()=>({})}:{}),createElementNS:(_,t)=>new Element(t,doc),listeners:{},addEventListener(k,f){(this.listeners[k]||=[]).push(f);}};
  const winds=[{id:'xaloc',name:'Xaloc',color:'#F5B45D',role:'Música',style:'pso'},{id:'mestral',name:'Mestral',color:'#8BD9CA',role:'Codi',style:'mtn'}];
  const requests=[];
  const root={document:doc,Event:class{constructor(type){this.type=type;}},IntersectionObserver:class{observe(){}},api:async(url,opts={})=>{
    const req={url,method:opts.method||'GET',body:opts.body ? JSON.parse(opts.body) : undefined}; requests.push(req);
    const overridden=apiOverride && await apiOverride(req,{winds});
    if(overridden) return overridden;
    return {ok:true,status:200,json:async()=>(url==='/api/winds' ? {winds} : {})};
  }};
  vm.runInNewContext(fs.readFileSync(path.join(__dirname,'app/winds.js'),'utf8'),{window:root,document:doc,console,URLSearchParams,setTimeout,clearTimeout});
  return {root,doc,requests,winds};
}
const settle=()=>new Promise(resolve=>setImmediate(resolve));
async function boot(override) { const f=fixture(override); await settle(); return f; }
function byText(root, selector, text) { return root.querySelectorAll(selector).find(n=>n.textContent.includes(text)); }

// The old «Fem lloc…» desk was hidden behind the workspace (winds-workspace.js)
// and still shipped; only the scripts the workspace needs stay in #ventsPage.
test('the legacy wind page is gone: #ventsPage only loads the scripts the workspace uses',()=>{
  const html=fs.readFileSync(path.join(__dirname,'index.html'),'utf8');
  assert.ok(!html.includes('winds-page'),'no .winds-page markup');
  const pane=html.slice(html.indexOf('<section class="workspace-page" id="ventsPage"'),html.indexOf('<section class="workspace-page" id="racoPage"'));
  assert.deepEqual(pane.match(/<[\w-]+/g),['<section','<script','<script','<script']);
  assert.ok(!fs.readFileSync(path.join(__dirname,'app/winds-workspace.css'),'utf8').includes('winds-page'));
  const f=fixture();
  assert.equal(f.root.gregalWinds.create,undefined,'no gallery of the legacy page');
  assert.equal(f.root.loadWindsPage,undefined,'winds-workspace.js owns loadWindsPage');
});

test('wind-only stylesheet is embedded, responsive, keyboard-visible and honors reduced motion',()=>{
  const html=fs.readFileSync(path.join(__dirname,'index.html'),'utf8');
  assert.match(html,/<link rel="stylesheet" href="\/app\/winds.css">/);
  const pane=html.slice(html.indexOf('<section class="workspace-page" id="ventsPage"'),html.indexOf('<section class="workspace-page" id="programatPage"'));
  assert.match(pane,/<script defer src="\/app\/winds.js"><\/script>/,'pane loads its own script, independently of the malformed legacy script tag');
  const css=fs.readFileSync(path.join(__dirname,'app/winds.css'),'utf8');
  assert.match(css,/\.winds-[\w-]+\[hidden\]/);
  assert.match(css,/:focus-visible/);
  assert.match(css,/@media\s*\(max-width:/);
  assert.match(css,/prefers-reduced-motion:\s*reduce/);
  assert.match(css,/var\(--surface\)/);
  assert.match(css,/minmax\(0,\s*1fr\)/);
  assert.ok(!/(?:^|\})\s*(?:body|:root|html)\s*\{/m.test(css),'no changes outside Vents');
});

test('hostile names, face codes and colors cannot add markup, attributes or remote SVG',async()=>{
  const hostile='<svg onload=alert(1)><script>x</script>';
  const f=await boot();
  const faces=[f.root.gregalWinds.face({id:'xaloc',name:hostile,color:'url(https://invalid/x)',style:hostile}),f.root.gregalWinds.face({id:'x',name:hostile,color:'red; background:url(x)',style:hostile},true)];
  for (const svg of faces) {
    assert.equal(svg.querySelectorAll('script').length,0);
    assert.equal(svg.querySelectorAll('[onload]').length,0);
    assert.equal(svg.querySelectorAll('image').length,0);
    // Les cares fan servir gradients locals (`url(#wf-body-N)`); el que no pot
    // passar és una referència remota o de fitxer.
    assert.ok(!svg.querySelectorAll('[fill]').some(n=>{
      const fill=String(n.getAttribute('fill'));
      return fill.includes('url(') && !/url\(\s*['"]?#/.test(fill);
    }),'fills only reference local gradients');
  }
  const source=fs.readFileSync(path.join(__dirname,'app/winds.js'),'utf8');
  assert.ok(!/\b(?:prompt|alert|fetch)\s*\(|innerHTML\s*=|insertAdjacentHTML/.test(source));
});

// Avatar packs: the account theme (Cares, Esperit de vent) draws every face,
// Free lets each wind use its own `avatar`.
function avatarServer(state) {
  const json=data=>({ok:true,json:async()=>data});
  return (req,{winds})=>{
    if(req.url==='/api/winds'&&req.method==='GET') return json({winds:winds.map(w=>({...w,avatar:state.avatars[w.id]||''})),avatarTheme:state.theme});
    if(req.url==='/api/winds/avatar-theme') {state.theme=req.body.theme;return json({theme:state.theme});}
  };
}
const packsOf=(f,state)=>f.winds.map(w=>f.root.gregalWinds.face({...w,avatar:state.avatars[w.id]||''}).getAttribute('data-avatar')||'cares');

test('«Avatars» in Preferences offers every pack and Free, and a pack draws every face',async()=>{
  const state={theme:'cares',avatars:{mestral:'esperit'}};
  const f=await boot(avatarServer(state));
  const box=f.doc.createElement('div');
  await f.root.gregalWinds.avatarPrefs(box);
  assert.deepEqual(packsOf(f,state),['cares','cares'],'Cares by default, even for a wind that chose another pack');
  assert.match(box.textContent,/Avatars/);
  assert.deepEqual(box.querySelectorAll('button').map(b=>b.textContent),['Cares','Esperit de vent','Cap de vent','Rosa de rellotge','Mapa de vent','Minairons','Pelut','Lliure']);
  assert.equal(byText(box,'button','Cares').getAttribute('aria-pressed'),'true');
  await byText(box,'button','Esperit de vent').click();
  assert.deepEqual(f.requests.find(r=>r.url==='/api/winds/avatar-theme').body,{theme:'esperit'});
  assert.equal(byText(box,'button','Esperit de vent').getAttribute('aria-pressed'),'true');
  assert.equal(byText(box,'button','Cares').getAttribute('aria-pressed'),'false');
  assert.deepEqual(packsOf(f,state),['esperit','esperit']);
  await byText(box,'button','Lliure').click();
  assert.deepEqual(packsOf(f,state),['cares','esperit'],'Free: each wind with its own avatar');
});

test('Cap de vent and Rosa de rellotge are packs in Preferences and draw every face',async()=>{
  const state={theme:'cares',avatars:{xaloc:'cap',mestral:'rosa'}};
  const f=await boot(avatarServer(state));
  const box=f.doc.createElement('div');
  await f.root.gregalWinds.avatarPrefs(box);
  for (const [name,pack] of [['Cap de vent','cap'],['Rosa de rellotge','rosa']]) {
    await byText(box,'button',name).click();
    assert.deepEqual(f.requests.findLast(r=>r.url==='/api/winds/avatar-theme').body,{theme:pack});
    assert.deepEqual(packsOf(f,state),[pack,pack],name+' draws every face');
  }
  await byText(box,'button','Lliure').click();
  assert.deepEqual(packsOf(f,state),['cap','rosa'],'Free: each wind with its own pack');
});

test('watchNews announces a novetat that arrives while the app is open, once, even with the tab hidden',async()=>{
  const now=Math.floor(Date.now()/1000);
  let feed=[{id:'n-1',wind:'xaloc',name:'Xaloc',text:'Abans d’obrir',ts:now-120}];
  const f=await boot(req=>req.url.startsWith('/api/winds/novetats?')?{ok:true,json:async()=>({novetats:feed})}:null);
  let tick;f.root.setInterval=fn=>{tick=fn;return 7;};
  const heard=[];
  f.root.gregalWinds.watchNews(list=>heard.push(...list.map(n=>n.id)));
  await settle();
  assert.deepEqual(heard,[],'what was already waiting on opening only shows on the face');
  assert.equal(f.root.gregalWinds.hasNews('xaloc'),true);
  feed=[...feed,{id:'n-2',wind:'mestral',name:'Mestral',text:'Ja ho tinc',ts:now}];
  f.doc.hidden=true;
  await tick();
  assert.deepEqual(heard,['n-2'],'a hidden tab still hears it, for the system notification');
  assert.equal(f.root.gregalWinds.hasNews('mestral'),true);
  await tick();
  assert.deepEqual(heard,['n-2'],'announced once');
});
