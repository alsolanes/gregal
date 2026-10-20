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
  async emit(k,props={}) { const e={target:this,key:'',preventDefault(){},...props}; if (this['on'+k]) await this['on'+k](e); for(const fn of this.listeners[k]||[]) await fn(e); }
  async click() { if (!this.disabled) await this.emit('click'); }
  focus() { this.doc.activeElement=this; }
  showModal() { this.open=true; }
  close() { this.open=false; this.emit('close'); }
  scrollIntoView() {}
}
function fixture(apiOverride) {
  const html=fs.readFileSync(path.join(__dirname,'index.html'),'utf8');
  const pane=html.slice(html.indexOf('<section class="workspace-page" id="ventsPage"'),html.indexOf('<section class="workspace-page" id="programatPage"'));
  const doc={activeElement:null,createElement:t=>new Element(t,doc),createElementNS:(_,t)=>new Element(t,doc),listeners:{},addEventListener(k,f){(this.listeners[k]||=[]).push(f);}};
  const dom=new Element('root',doc), stack=[dom];
  for (const token of pane.matchAll(/<\/?([\w-]+)([^>]*)>/g)) {
    if(token[0].startsWith('</')) { if(stack.length>1) stack.pop(); continue; }
    const el=doc.createElement(token[1]); for (const a of token[2].matchAll(/([\w-]+)(?:="([^"]*)")?/g)) el.setAttribute(a[1],a[2]||'');
    if (el.getAttribute('hidden')!==null) el.hidden=true;
    stack.at(-1).append(el); if(!['link','input','br','hr'].includes(token[1])) stack.push(el);
  }
  doc.getElementById=id=>dom.querySelector('#'+id);
  const winds=[{id:'xaloc',name:'Xaloc',color:'#F5B45D',role:'Música',style:'pso'},{id:'mestral',name:'Mestral',color:'#8BD9CA',role:'Codi',style:'mtn'}];
  const rotllanes=[{id:'vespre',name:'Vespre',members:['xaloc','mestral']},{id:'musica',name:'Idees de música',members:['xaloc']}];
  const requests=[];
  const root={document:doc,api:async(url,opts={})=>{
    const req={url,method:opts.method||'GET',body:opts.body ? JSON.parse(opts.body) : undefined}; requests.push(req);
    const overridden=apiOverride && await apiOverride(req,{winds,rotllanes});
    if(overridden) return overridden;
    let data={};
    if(url==='/api/winds') data={winds};
    else if(url==='/api/winds/rotllanes') data={rotllanes};
    else if(url.startsWith('/api/winds/profile?')) data={wind:{...winds[0],persona:'Curiós i tranquil.'},knowledge:'Coneixement desat real',personal_memory:'Record personal real'};
    else if(url.startsWith('/api/winds/chat')||url.startsWith('/api/winds/rotllana')) data={history:[]};
    return {ok:true,status:200,json:async()=>data};
  }};
  vm.runInNewContext(fs.readFileSync(path.join(__dirname,'app/winds.js'),'utf8'),{window:root,document:doc,console,URLSearchParams,setTimeout,clearTimeout});
  return {root,doc,dom,requests,winds,rotllanes,$:id=>doc.getElementById(id)};
}
const settle=()=>new Promise(resolve=>setImmediate(resolve));
async function boot(override) { const f=fixture(override); await f.root.loadWindsPage(); await settle(); return f; }
function byText(root, selector, text) { return root.querySelectorAll(selector).find(n=>n.textContent.includes(text)); }

test('shows existing wind histories plus group and named single-wind conversations with faces',async()=>{
  const f=await boot();
  assert.ok(f.requests.some(r=>r.url==='/api/winds/rotllanes'),'load actual named conversations');
  assert.match(f.$('windsList').textContent,/Xaloc/);
  assert.match(f.$('windsConversations').textContent,/Vespre/);
  assert.match(f.$('windsConversations').textContent,/Idees de música/);
  assert.ok(f.dom.querySelectorAll('svg').length>=3,'faces, not color dots');
  assert.ok(f.dom.querySelectorAll('button').every(b=>b.getAttribute('type')==='button'||b.getAttribute('type')==='submit'));
});

test('opens group history with structured speaker attribution and preserves failed drafts while busy',async()=>{
  let finish;
  const f=await boot(req=>{
    if(req.url==='/api/winds/rotllana?rotlla=vespre') return {ok:true,json:async()=>({history:[{from:'user',text:'Hola'},{from:'assistant',speaker_id:'mestral',speaker_name:'Mestral',text:'<img src=x onerror=alert(1)>'}]})};
    if(req.method==='POST') return new Promise(resolve=>{finish=()=>resolve({ok:false,status:502});});
  });
  await byText(f.$('windsConversations'),'button','Vespre').click();
  assert.equal(f.$('windChatCard').hidden,false);
  assert.match(f.$('windChatHistory').textContent,/Tu/);
  assert.match(f.$('windChatHistory').textContent,/Mestral/);
  assert.match(f.$('windChatHistory').textContent,/<img src=x onerror=alert\(1\)>/);
  assert.equal(f.$('windChatHistory').querySelectorAll('img').length,0);
  f.$('windChatInput').value='Una idea';
  const sending=f.$('windsChatForm').emit('submit'); await settle();
  assert.equal(f.$('windChatSend').disabled,true);
  assert.match(f.$('windsChatStatus').textContent,/resposta/);
  await f.$('windsChatForm').emit('submit');
  assert.equal(f.requests.filter(r=>r.method==='POST').length,1,'no double send');
  finish(); await sending;
  assert.equal(f.$('windChatInput').value,'Una idea');
  assert.equal(f.$('windChatSend').disabled,false);
  assert.match(f.$('windsChatStatus').textContent,/No hem pogut/);
  assert.ok(!f.$('windsChatStatus').textContent.includes('502'));
  assert.deepEqual(f.requests.find(r=>r.method==='POST').body,{rotllana:'vespre',message:'Una idea'});
});

test('renders each voice of a multi-voice reply as its own message with the name in the header only',async()=>{
  const f=await boot(req=>{
    if(req.url==='/api/winds/rotllana?rotlla=vespre') return {ok:true,json:async()=>({history:[
      {from:'user',text:'Comenceu'},
      {from:'assistant',speaker_id:'mestral',speaker_name:'Mestral',text:'Mestral: endavant!'},
      {from:'assistant',speaker_id:'xaloc',speaker_name:'Xaloc',text:'Xaloc: jo hi sumo una idea.'}
    ]})};
  });
  await byText(f.$('windsConversations'),'button','Vespre').click();
  const msgs=[...f.$('windChatHistory').querySelectorAll('.winds-message')].filter(m=>!String(m.className||'').includes('winds-message-user'));
  assert.equal(msgs.length,2,'one message per voice');
  const texts=[...f.$('windChatHistory').querySelectorAll('.winds-message-text')].map(n=>n.textContent);
  assert.equal(texts[1],'endavant!','name prefix stripped from mestral text');
  assert.equal(texts[2],'jo hi sumo una idea.','name prefix stripped from xaloc text');
  const speakers=[...f.$('windChatHistory').querySelectorAll('.winds-speaker')].map(n=>n.textContent);
  assert.deepEqual(speakers.slice(1),['Mestral','Xaloc'],'speaker header carries the name');
});

test('legacy wind histories remain accessible and sending reads persisted history back',async()=>{
  let sent=false;
  const f=await boot(req=>{
    if(req.method==='POST') {sent=true; return {ok:true,json:async()=>({history:[]})};}
    if(req.url==='/api/winds/chat?wind=xaloc') return {ok:true,json:async()=>({history:sent ? [{from:'assistant',text:'Resposta desada'}] : [{from:'assistant',text:'Historial anterior'}]})};
  });
  await byText(f.$('windsList'),'button','Xaloc').click();
  assert.match(f.$('windChatHistory').textContent,/Historial anterior/);
  f.$('windChatInput').value='Hola'; await f.$('windsChatForm').emit('submit');
  assert.match(f.$('windChatHistory').textContent,/Resposta desada/);
  assert.equal(f.$('windChatInput').value,'');
  assert.deepEqual(f.requests.find(r=>r.method==='POST').body,{wind:'xaloc',message:'Hola'});
});

test('new wind opens a labelled dialog, previews a safe face and saves every character field',async()=>{
  const f=await boot((req,{winds})=>{
    if(req.url==='/api/winds' && req.method==='POST') {const w={...req.body,id:'ponent'}; winds.push(w); return {ok:true,json:async()=>({wind:w})};}
    if(req.url==='/api/winds/profile?wind=ponent') return {ok:true,json:async()=>({wind:{id:'ponent',name:'Ponent',color:'#8BD9CA',role:'Llibres',style:'etn',persona:'Parla amb calma.'},knowledge:'',personal_memory:''})};
  });
  await f.$('windNewBtn').click();
  assert.equal(f.$('windsNewMenu').hidden,false);
  assert.equal(f.$('windNewBtn').getAttribute('aria-expanded'),'true');
  await f.$('windsCreateWind').click();
  assert.equal(f.$('windsDialog').open,true);
  assert.equal(f.$('windsDialog').getAttribute('aria-labelledby'),'windsDialogTitle');
  assert.equal(f.$('windsNewMenu').hidden,true);
  await byText(f.$('windsDialogBody'),'button','personalitzat').click();
  for (const [id,value] of Object.entries({windName:'Ponent',windRole:'Llibres',windColor:'#8BD9CA',windPersona:'Parla amb calma.',windEyes:'e',windMouth:'t',windShape:'n'})) f.$(id).value=value;
  await f.$('windShape').emit('change');
  assert.ok(f.$('windFacePreview').querySelector('svg'));
  await f.$('windsWindForm').emit('submit');
  assert.deepEqual(f.requests.find(r=>r.method==='POST').body,{name:'Ponent',role:'Llibres',color:'#8BD9CA',persona:'Parla amb calma.',style:'etn'});
  assert.equal(f.$('windsDialog').open,false);
  assert.match(f.$('windsList').textContent,/Ponent/);
  assert.ok(f.requests.some(r=>r.url==='/api/winds/profile?wind=ponent'),'read exact saved target back');
  assert.equal(f.doc.activeElement,f.$('windNewBtn'),'restore focus');
});

test('new wind starts with eight pastel character presets that fill editable persona and never edit an existing wind',async()=>{
  const f=await boot((req,{winds})=>{
    if(req.url==='/api/winds' && req.method==='POST') {const wind={...req.body,id:'new-migjorn'};winds.push(wind);return {ok:true,json:async()=>({wind})};}
    if(req.url==='/api/winds/profile?wind=new-migjorn') return {ok:true,json:async()=>({wind:winds.at(-1)})};
  });
  const presets=[['Marinada','#EFC69A','bsn',/calma/i],['Xaloc','#EFA68F','eto',/música/i],['Tramuntana','#A8CDE2','pso',/curios/i],['Mestral','#BFCFA4','esn',/aprendre/i],['Llebeig','#E8AEC2','pto',/cultura/i],['Gregal','#C1B4DE','mso',/dia a dia/i],['Ponent','#EACF87','msn',/llibres/i],['Migjorn','#9ED3C5','bss',/natura/i]];
  for(const [name,color,style,interest] of presets) {
    await f.$('windsCreateWind').click();
    const gallery=f.$('windsPresetGallery');
    assert.ok(gallery,'creation begins with a preset gallery');
    assert.equal(f.$('windName'),null,'details wait until a character or custom create is chosen');
    assert.equal(gallery.querySelectorAll('button').length,8);
    assert.equal(gallery.querySelectorAll('svg').length,8);
    assert.equal(f.doc.activeElement,gallery.querySelector('button'),'keyboard starts on a preset');
    await byText(gallery,'button',name).click();
    assert.equal(f.$('windsPresetGallery'),null);
    assert.equal(f.$('windName').value,name);
    assert.equal(f.$('windColor').value,color);
    assert.equal(f.$('windEyes').value+f.$('windMouth').value+f.$('windShape').value,style);
    assert.match(f.$('windRole').value,interest);
    assert.ok(f.$('windPersona').value.length>30,'a real editable persona, not only a name and face');
    assert.equal(f.requests.filter(r=>r.method==='POST').length,0,'choosing a preset does not save or mutate an existing namesake');
    if(name!=='Migjorn') await byText(f.$('windsDialogBody'),'button','Cancel·la').click();
  }
  f.$('windName').value='El meu Migjorn'; f.$('windPersona').value='Una veu pròpia';
  await f.$('windsWindForm').emit('submit');
  const payload=f.requests.find(r=>r.method==='POST').body;
  assert.equal(payload.id,undefined,'preset creates a new wind even when a namesake already exists');
  assert.equal(payload.name,'El meu Migjorn'); assert.equal(payload.persona,'Una veu pròpia');
  assert.equal(payload.color,'#9ED3C5'); assert.equal(payload.style,'bss');
  assert.equal(f.$('windsDialog').open,false);
});

test('new conversation supports a named singleton and opens its actual persisted history',async()=>{
  const f=await boot((req,{rotllanes})=>{
    if(req.method==='POST'&&req.url==='/api/winds/rotllanes') {const c={...req.body,id:'lectures'};rotllanes.push(c);return {ok:true,json:async()=>({rotllana:c})};}
  });
  await f.$('windNewBtn').click(); await f.$('windsCreateChat').click();
  f.$('windsConversationName').value='Lectures';
  f.$('windsChatFormDialog').querySelectorAll('input').find(n=>n.value==='xaloc').checked=true;
  await f.$('windsChatFormDialog').emit('submit');
  assert.deepEqual(f.requests.find(r=>r.method==='POST').body,{name:'Lectures',members:['xaloc']});
  assert.match(f.$('windsConversations').textContent,/Lectures/);
  assert.ok(f.requests.some(r=>r.url==='/api/winds/rotllana?rotlla=lectures'));
  assert.equal(f.$('windChatTitle').textContent,'Lectures');
});

test('group header edits its prefilled name and members with a stable id, minimum two and verified readback',async()=>{
  const f=await boot((req,{winds,rotllanes})=>{
    if(req.url==='/api/winds' && req.method==='GET' && winds.length===2) winds.push({id:'gregal',name:'Gregal',style:'mso',color:'#C1B4DE'});
    if(req.url==='/api/winds/rotllanes' && req.method==='POST') {
      const index=rotllanes.findIndex(c=>c.id===req.body.id);
      assert.notEqual(index,-1,'editing must address the existing group');
      rotllanes[index]={...req.body};
      return {ok:true,json:async()=>({rotllana:rotllanes[index]})};
    }
  });
  await byText(f.$('windsConversations'),'button','Vespre').click();
  const edit=byText(f.$('windChatCard').querySelector('header'),'button','Edita');
  assert.ok(edit,'group conversation header exposes a visible edit action');
  assert.equal(edit.getAttribute('aria-haspopup'),'dialog');
  assert.match(edit.textContent,/membres/);
  await edit.click();
  assert.equal(f.$('windsConversationName').value,'Vespre');
  const checks=f.$('windsChatFormDialog').querySelectorAll('[name="members"]');
  assert.deepEqual(checks.filter(c=>c.checked).map(c=>c.value),['xaloc','mestral']);
  checks.find(c=>c.value==='mestral').checked=false;
  await f.$('windsChatFormDialog').emit('submit');
  assert.equal(f.requests.filter(r=>r.method==='POST').length,0,'an existing group cannot silently become a singleton');
  assert.match(f.$('windsChatFormDialog').textContent,/dos vents/);
  checks.find(c=>c.value==='gregal').checked=true;
  f.$('windsConversationName').value='<Grup nou>';
  await f.$('windsChatFormDialog').emit('submit');
  assert.deepEqual(f.requests.find(r=>r.method==='POST').body,{id:'vespre',name:'<Grup nou>',members:['xaloc','gregal']});
  assert.equal(f.rotllanes.length,2,'no duplicate group');
  const postIndex=f.requests.findIndex(r=>r.method==='POST');
  assert.ok(f.requests.slice(postIndex+1).some(r=>r.url==='/api/winds/rotllanes'),'read the saved group catalogue back');
  assert.ok(f.requests.slice(postIndex+1).some(r=>r.url==='/api/winds/rotllana?rotlla=vespre'),'keep the same persisted history');
  assert.equal(f.$('windChatTitle').textContent,'<Grup nou>');
  assert.match(f.$('windChatSubtitle').textContent,/Xaloc · Gregal/);
  assert.equal(f.$('windsDialog').open,false);
  assert.equal(f.doc.activeElement,byText(f.$('windChatCard').querySelector('header'),'button','Edita'));
});

test('group edit does not claim success when readback is stale or returns a different group id',async()=>{
  for(const wrongID of [false,true]) {
    const f=await boot(req=>{
      if(req.url==='/api/winds/rotllanes' && req.method==='POST') return {ok:true,json:async()=>({rotllana:{...req.body,id:wrongID?'different-group':req.body.id}})};
    });
    await byText(f.$('windsConversations'),'button','Vespre').click();
    await f.$('windsEditConversation').click();
    f.$('windsConversationName').value='Nom pendent';
    await f.$('windsChatFormDialog').emit('submit');
    assert.equal(f.$('windsDialog').open,true);
    assert.equal(f.$('windsConversationName').value,'Nom pendent');
    assert.equal(f.$('windChatTitle').textContent,'Vespre');
    assert.match(f.$('windsChatFormDialog').textContent,/no hem pogut comprovar/i);
    assert.equal(f.rotllanes.length,2);
  }
});

test('a delayed group edit preserves another current chat and its draft',async()=>{
  let finish;
  const f=await boot((req,{rotllanes})=>{
    if(req.url==='/api/winds/rotllanes' && req.method==='POST') return new Promise(resolve=>{finish=()=>{
      rotllanes[0]={...req.body};resolve({ok:true,json:async()=>({rotllana:rotllanes[0]})});
    };});
  });
  await byText(f.$('windsConversations'),'button','Vespre').click();
  await f.$('windsEditConversation').click();
  f.$('windsConversationName').value='Vespre actualitzat';
  const saving=f.$('windsChatFormDialog').emit('submit'); await settle();
  await f.$('windsChatFormDialog').emit('submit');
  assert.equal(f.requests.filter(r=>r.method==='POST').length,1,'no duplicate edit while saving');
  await byText(f.$('windsList'),'button','Xaloc').click();
  f.$('windChatInput').value='Un altre esborrany';
  finish(); await saving;
  assert.equal(f.$('windChatTitle').textContent,'Xaloc');
  assert.equal(f.$('windChatInput').value,'Un altre esborrany');
  assert.equal(f.$('windsEditConversation').hidden,true,'legacy wind chats do not expose group editing');
  assert.match(f.$('windsConversations').textContent,/Vespre actualitzat/);
});

test('profile displays only real character and both memory sources, edits with stable id and replaces knowledge',async()=>{
  let wind={id:'xaloc',name:'Xaloc',role:'Música',color:'#F5B45D',style:'pso',persona:'Curiós i tranquil.'}, knowledge='Coneixement desat real';
  const f=await boot((req,{winds})=>{
    if(req.url==='/api/winds/profile?wind=xaloc') return {ok:true,json:async()=>({wind,knowledge,personal_memory:'Record personal real'})};
    if(req.url==='/api/winds'&&req.method==='POST') {wind={...req.body};winds[0]=wind;return {ok:true,json:async()=>({wind})};}
    if(req.url==='/api/winds/memory') {knowledge=req.body.text;return {ok:true,json:async()=>({wind,knowledge,personal_memory:'Record personal real'})};}
  });
  await byText(f.$('windsList'),'button','Xaloc').click();
  await byText(f.$('windsChatMembers'),'button','Perfil').click();
  assert.equal(f.$('windsProfile').hidden,false);
  assert.match(f.$('windsProfile').textContent,/Curiós i tranquil/);
  assert.equal(f.$('windKnowledge').value,'Coneixement desat real');
  assert.match(f.$('windsProfile').textContent,/Record personal real/);
  await byText(f.$('windsProfile'),'button','Edita el caràcter').click();
  assert.equal(f.$('windsPresetGallery'),null,'existing wind editing bypasses presets');
  assert.equal(f.$('windName').value,'Xaloc');
  assert.equal(f.$('windRole').value,'Música');
  assert.equal(f.$('windColor').value,'#F5B45D');
  assert.equal(f.$('windEyes').value+f.$('windMouth').value+f.$('windShape').value,'pso');
  assert.equal(f.$('windPersona').value,'Curiós i tranquil.');
  f.$('windName').value='Xaloc nou'; f.$('windPersona').value='Una veu nova';
  await f.$('windsWindForm').emit('submit');
  assert.equal(f.requests.find(r=>r.method==='POST').body.id,'xaloc');
  assert.match(f.$('windsProfile').textContent,/Una veu nova/);
  f.$('windKnowledge').value='Només aquest coneixement';
  await f.$('windsMemoryForm').emit('submit');
  assert.deepEqual(f.requests.find(r=>r.url==='/api/winds/memory').body,{wind:'xaloc',text:'Només aquest coneixement'});
  assert.equal(f.$('windKnowledge').value,'Només aquest coneixement');
  assert.match(f.$('windsMemoryStatus').textContent,/desat/);
  assert.ok(f.requests.filter(r=>r.url==='/api/winds/profile?wind=xaloc').length>=3,'read memory/profile writes back');
});

test('missing memory and profile errors are honest, not fabricated companion facts',async()=>{
  let fail=false;
  const f=await boot(req=>{
    if(req.url.startsWith('/api/winds/profile?')) return fail ? {ok:false,status:500} : {ok:true,json:async()=>({wind:{id:'xaloc',name:'Xaloc',persona:''},knowledge:'',personal_memory:''})};
  });
  await byText(f.$('windsList'),'button','Xaloc').click(); await byText(f.$('windsChatMembers'),'button','Perfil').click();
  assert.equal(f.$('windKnowledge').value,'');
  assert.match(f.$('windsProfile').textContent,/Encara no hi ha records personals/);
  fail=true; await byText(f.$('windsChatMembers'),'button','Perfil').click();
  assert.match(f.$('windsProfile').textContent,/No hem pogut/);
  assert.equal(f.$('windsProfile').querySelectorAll('textarea').length,0,'do not offer a blank memory editor after failed load');
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

test('hostile names, persona, face codes and colors cannot add markup, attributes or remote SVG',async()=>{
  const hostile='<svg onload=alert(1)><script>x</script>';
  const f=await boot((req,{winds})=>{
    if(req.url==='/api/winds') {winds[0]={...winds[0],name:hostile,color:'url(https://invalid/x)',style:hostile};return {ok:true,json:async()=>({winds})};}
    if(req.url.startsWith('/api/winds/profile?')) return {ok:true,json:async()=>({wind:{id:'xaloc',name:hostile,persona:hostile,color:'red; background:url(x)',style:hostile},knowledge:hostile,personal_memory:hostile})};
  });
  assert.match(f.$('windsList').textContent,/<svg onload/);
  await byText(f.$('windsList'),'button',hostile).click(); await byText(f.$('windsChatMembers'),'button','Perfil').click();
  assert.ok(f.dom.querySelectorAll('script').every(n=>n.getAttribute('src')==='/app/winds.js'),'only the authored script tag exists');
  for (const svg of f.dom.querySelectorAll('svg')) {
    assert.equal(svg.querySelectorAll('[onload]').length,0);
    assert.equal(svg.querySelectorAll('image').length,0);
    assert.ok(!svg.querySelectorAll('[fill]').some(n=>String(n.getAttribute('fill')).includes('url(')));
  }
  const source=fs.readFileSync(path.join(__dirname,'app/winds.js'),'utf8');
  assert.ok(!/\b(?:prompt|alert|fetch)\s*\(|innerHTML\s*=|insertAdjacentHTML/.test(source));
});

test('renaming a wind refreshes the chat header and restores focus to the current edit button',async()=>{
  let wind={id:'xaloc',name:'Xaloc',role:'Música',color:'#F5B45D',style:'pso',persona:'Persona'};
  const f=await boot((req,{winds})=>{
    if(req.url.startsWith('/api/winds/profile?')) return {ok:true,json:async()=>({wind,knowledge:'',personal_memory:''})};
    if(req.url==='/api/winds'&&req.method==='POST') {wind={...req.body};winds[0]=wind;return {ok:true,json:async()=>({wind})};}
  });
  await byText(f.$('windsList'),'button','Xaloc').click(); await byText(f.$('windsChatMembers'),'button','Perfil').click();
  await byText(f.$('windsProfile'),'button','Edita el caràcter').click();
  f.$('windName').value='Xaloc nou'; await f.$('windsWindForm').emit('submit');
  assert.equal(f.$('windChatTitle').textContent,'Xaloc nou');
  assert.equal(f.doc.activeElement,byText(f.$('windsProfile'),'button','Edita el caràcter'));
});

test('returning to a conversation during its pending answer still refreshes it when the answer arrives',async()=>{
  let finish, answered=false;
  const f=await boot(req=>{
    if(req.url==='/api/winds/chat'&&req.method==='POST') return new Promise(resolve=>{finish=()=>{answered=true;resolve({ok:true,json:async()=>({})});};});
    if(req.url==='/api/winds/chat?wind=xaloc') return {ok:true,json:async()=>({history:answered ? [{from:'pet',text:'Ja he tornat'}] : []})};
  });
  await byText(f.$('windsList'),'button','Xaloc').click();
  f.$('windChatInput').value='Una pregunta'; const sending=f.$('windsChatForm').emit('submit'); await settle();
  await byText(f.$('windsConversations'),'button','Vespre').click();
  await byText(f.$('windsList'),'button','Xaloc').click();
  finish(); await sending;
  assert.match(f.$('windChatHistory').textContent,/Ja he tornat/);
  assert.equal(f.$('windsChatStatus').textContent,'');
  assert.equal(f.$('windChatSend').disabled,false);
});

test('new rotllana requires a name and members, then persists all selected winds',async()=>{
  const f=await boot((req,{rotllanes})=>{
    if(req.method==='POST'&&req.url==='/api/winds/rotllanes') {const c={...req.body,id:'grup'};rotllanes.push(c);return {ok:true,json:async()=>({rotllana:c})};}
  });
  await f.$('windNewBtn').click(); await f.$('windsCreateChat').click();
  await f.$('windsChatFormDialog').emit('submit');
  assert.equal(f.requests.filter(r=>r.method==='POST').length,0);
  f.$('windsConversationName').value='Grup';
  f.$('windsChatFormDialog').querySelectorAll('[name="members"]').forEach(c=>{c.checked=true;});
  await f.$('windsChatFormDialog').emit('submit');
  assert.deepEqual(f.requests.find(r=>r.method==='POST').body,{name:'Grup',members:['xaloc','mestral']});
  assert.equal(f.$('windsChatMembers').querySelectorAll('button').length,2);
});

test('memory save failure preserves draft and explicitly saving an empty string clears knowledge',async()=>{
  let fail=true, knowledge='Abans';
  const f=await boot(req=>{
    if(req.url.startsWith('/api/winds/profile?')) return {ok:true,json:async()=>({wind:{id:'xaloc',name:'Xaloc'},knowledge,personal_memory:''})};
    if(req.url==='/api/winds/memory') {if(fail)return {ok:false,status:500};knowledge=req.body.text;return {ok:true,json:async()=>({})};}
  });
  await byText(f.$('windsList'),'button','Xaloc').click(); await byText(f.$('windsChatMembers'),'button','Perfil').click();
  f.$('windKnowledge').value='Esborrany'; await f.$('windsMemoryForm').emit('submit');
  assert.equal(f.$('windKnowledge').value,'Esborrany');
  assert.match(f.$('windsMemoryStatus').textContent,/No hem pogut/);
  fail=false; f.$('windKnowledge').value=''; await f.$('windsMemoryForm').emit('submit');
  assert.equal(f.$('windKnowledge').value,'');
  assert.equal(f.requests.filter(r=>r.url==='/api/winds/memory').at(-1).body.text,'');
});

test('delayed history and profile responses cannot overwrite a different current conversation',async()=>{
  let finishHistory, finishProfile;
  const f=await boot(req=>{
    if(req.url==='/api/winds/chat?wind=xaloc') return new Promise(resolve=>{finishHistory=()=>resolve({ok:true,json:async()=>({history:[{from:'pet',text:'Resposta antiga'}]})});});
    if(req.url==='/api/winds/profile?wind=xaloc') return new Promise(resolve=>{finishProfile=()=>resolve({ok:true,json:async()=>({wind:{id:'xaloc',name:'Xaloc'},knowledge:'Antic',personal_memory:''})});});
  });
  const opening=byText(f.$('windsList'),'button','Xaloc').click(); await settle();
  const profile=byText(f.$('windsChatMembers'),'button','Perfil').click(); await settle();
  await byText(f.$('windsConversations'),'button','Vespre').click(); finishHistory(); finishProfile(); await opening; await profile;
  assert.equal(f.$('windChatTitle').textContent,'Vespre');
  assert.ok(!f.$('windChatHistory').textContent.includes('Resposta antiga'));
  assert.equal(f.$('windsProfile').hidden,true);
});

test('failed post-save catalogue verification does not claim success or lose form values',async()=>{
  let posted=false;
  const f=await boot((req,{rotllanes})=>{
    if(req.method==='POST') {posted=true;return {ok:true,json:async()=>({rotllana:{id:'nou',name:'Nou',members:['xaloc']}})};}
    if(posted&&req.url==='/api/winds/rotllanes') return {ok:false,status:503};
  });
  await f.$('windNewBtn').click(); await f.$('windsCreateChat').click();
  f.$('windsConversationName').value='Nou'; f.$('windsChatFormDialog').querySelector('[name="members"]').checked=true;
  await f.$('windsChatFormDialog').emit('submit');
  assert.equal(f.$('windsDialog').open,true);
  assert.equal(f.$('windsConversationName').value,'Nou');
  assert.match(f.$('windsChatFormDialog').textContent,/no hem pogut comprovar/i);
});
