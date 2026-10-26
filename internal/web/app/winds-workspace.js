// One shared crew, two complementary contexts: conversation and project work.
(function(root){
 'use strict';
 const $=id=>document.getElementById(id), ca=()=>document.documentElement.lang==='ca', t=(a,b)=>ca()?a:b;
 const state={winds:[],squads:[],chats:[],tasks:[],projects:[],workspace:'',view:'home',squad:'daily',active:null,history:[],busy:false,draft:'',snapshot:null,events:[],library:[],filter:'all'};
 let mounted=false, page, nav, content, messages, composer, input, status, loadVersion=0, chatController=null, pendingFrame=0, demoTimer=0, demoRun=0;
 const el=(tag,cls,text)=>{const n=document.createElement(tag);if(cls)n.className=cls;if(text!==undefined)n.textContent=text;return n;};
 const btn=(label,action,cls='vw-button')=>{const n=el('button',cls,label);n.type='button';n.onclick=()=>Promise.resolve().then(action).catch(fail);return n;};
 async function call(url,method='GET',body){const r=await root.api(url,{method,...(body===undefined?{}:{headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})});if(!r.ok)throw new Error(await r.text());return r.json();}
 function fail(error){let target=document.querySelector('.vw-dialog[open] .vw-error');const modal=document.querySelector('.vw-dialog[open]');if(modal&&!target){target=el('p','vw-error');modal.append(target);}target=target||status;if(target){target.textContent=t('No s’ha pogut completar: ','Could not complete: ')+error.message;target.setAttribute('role','alert');}}
 const face=(w,work)=>root.gregalWinds.face(w||{name:work?.name,color:'#BFCCA8',style:'psn'},false,work&&root.gregalWinds.moodOf(work));
 const folder=()=>{const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 24 24');svg.setAttribute('class','vw-folder');svg.setAttribute('aria-hidden','true');const p=document.createElementNS(svg.namespaceURI,'path');p.setAttribute('d','M3 6a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v10H3Z');p.setAttribute('fill','none');p.setAttribute('stroke','currentColor');p.setAttribute('stroke-width','1.6');svg.append(p);return svg;};
 const squadName=id=>{const s=state.squads.find(s=>s.id===id);return s?(ca()?s.ca:s.en):t('La teva colla','Your crew');};
 const crew=()=>state.winds.filter(w=>w.squad===state.squad);
 // Group replies are stored as "Name: text" for the model's context; the face and name already say who speaks.
 const withoutSpeaker=(text,name)=>{let out=String(text||'');const prefix=String(name||'').trim().toLowerCase()+':';if(prefix.length<2)return out;while(out.slice(0,prefix.length).toLowerCase()===prefix)out=out.slice(prefix.length).trimStart();return out;};
 const roleName=w=>{const specialist=state.squads.find(s=>s.id===w.squad)?.members?.find(m=>m.key===w.specialty);return specialist?(ca()?specialist.ca:specialist.en):w.role||'';};
 function heading(title,copy){const h=el('div','vw-heading');h.append(el('h1','',title));if(copy)h.append(el('p','',copy));content.append(h);}
 function section(title,action){const h=el('div','vw-section-label');h.append(el('span','',title));if(action)h.append(action);nav.append(h);}
 function route(view,active=null){if(state.preparing&&!state.busy)return;clearTimeout(demoTimer);demoRun++;state.chatEpoch=(state.chatEpoch||0)+1;state.draft=input?.value??state.draft;state.view=view;state.active=active;state.history=[];if(view!=='task'&&root.gregalPreview?.getState().open)$('previewClose')?.click();render();}
 function newChat(){route('home');state.draft='';if(input){input.value='';input.focus();}}
 function mount(){
  if(mounted)return;mounted=true;
  page=el('div','wind-workspace');page.id='windWorkspace';$('ventsPage').append(page);
  nav=el('nav','vw-nav');nav.setAttribute('aria-label',t('Espai dels vents','Winds workspace'));$('windContextNav').append(nav);
  $('windsListCard').hidden=true;
  nav.addEventListener('click',e=>{if(e.target.closest('button'))closeDrawer();});
  $('navRail')?.addEventListener('click',e=>{if(e.target.closest('a,button'))closeDrawer();});
  $('sideShade')?.addEventListener('click',closeDrawer);
 }
 // On phones the Winds bar replaces the app rail; the menu opens both together as one drawer.
 function setDrawer(open){$('side').classList.toggle('closed',!open);$('side').classList.toggle('vw-open',open);document.body.classList.toggle('vw-drawer',open);$('sideShade')?.classList.toggle('on',open);page?.querySelector('.vw-mobile-menu')?.setAttribute('aria-expanded',String(open));}
 function closeDrawer(){if(document.body.classList.contains('vw-drawer'))setDrawer(false);}
 function renderNav(){
  nav.replaceChildren(btn(t('＋ Nova conversa','＋ New conversation'),newChat,'vw-new'));
  for(const [id,a,b] of [['home','Inici','Home'],['tasks','Feines','Work'],['crew','La colla','Crew'],['library','Biblioteca','Library'],['wiki','Wiki','Wiki']]){const n=btn('',()=>route(id),'vw-nav-item vw-nav-icon');n.append(navIcon(id),el('span','',t(a,b)));n.setAttribute('aria-current',state.view===id?'page':'false');nav.append(n);}
  section(t('EQUIPS','SQUADS'));
  for(const squad of state.squads){const n=btn(ca()?squad.ca:squad.en,()=>{state.squad=squad.id;newChat();},'vw-nav-item vw-squad');n.dataset.selected=String(state.squad===squad.id);nav.append(n);}
  section(t('CONVERSES','CONVERSATIONS'),btn('+',newChat,'vw-small'));
  for(const chat of state.chats.filter(c=>!c.workspace).slice(0,12))nav.append(btn(chat.name,()=>openChat(chat),'vw-nav-item vw-thread'));
  if(!state.chats.some(c=>!c.workspace))nav.append(el('p','vw-nav-empty',t('Aquí comencen les bones idees.','Good ideas start here.')));
  section(t('PROJECTES','PROJECTS'),btn('+',chooseFolder,'vw-small'));
  for(const project of state.projects.slice(0,12)){
   const n=btn('',()=>openProject(project),'vw-nav-item vw-project');n.append(folder(),el('span','',project.name));n.title=project.path;n.dataset.selected=String((state.active?.path||state.active?.workspace)===project.path);nav.append(n);
   if((state.active?.path||state.active?.workspace)===project.path){for(const chat of state.chats.filter(c=>c.workspace===project.path).slice(0,8))nav.append(btn(chat.name,()=>openChat(chat),'vw-nav-item vw-thread vw-nested'));}
  }
 }
 function render(){
  if(!mounted)return;renderNav();page.replaceChildren();messages=null;input=null;composer=null;
  const top=el('header','vw-top');const menu=btn('☰',()=>setDrawer(!document.body.classList.contains('vw-drawer')),'vw-mobile-menu');menu.setAttribute('aria-label',t('Obre la navegació','Open navigation'));menu.setAttribute('aria-expanded',String(document.body.classList.contains('vw-drawer')));top.append(menu,el('span','',t('Vents','Winds')),el('span','vw-separator','/'),el('span','',state.active?.name||squadName(state.squad)));
  if(state.view==='chat'&&state.active?.id)top.append(btn(t('Desa a la wiki','Save to wiki'),summarizeToWiki,'vw-top-action'));
  top.append(btn(t('La colla','Crew'),()=>route('crew'),'vw-top-action'));page.append(top);
  content=el('div','vw-content');page.append(content);status=el('div','vw-status');status.setAttribute('role','status');page.append(status);
  if(state.view==='home')home();else if(state.view==='crew')team();else if(state.view==='library')library();else if(state.view==='project')projectView();else if(state.view==='tasks')tasks();else if(state.view==='chat')chatView();else if(state.view==='task')taskView();else if(state.view==='wiki')wikiView();
  const bottom=el('nav','vw-mobile-nav');for(const [id,a,b]of [['home','Inici','Home'],['tasks','Feines','Work'],['crew','Colla','Crew'],['library','Biblioteca','Library'],['wiki','Wiki','Wiki']]){const n=btn(t(a,b),()=>route(id));n.dataset.selected=String(state.view===id);bottom.append(n);}page.append(bottom);
 }
 function addComposer(workspace=false){
  composer=el('form','vw-composer');input=el('textarea');input.rows=2;input.value=state.draft;input.placeholder=t('Què tens al cap?','What’s on your mind?');input.setAttribute('aria-label',input.placeholder);input.oninput=()=>{state.draft=input.value;};
  input.addEventListener('keydown',e=>{if(e.key==='Enter'&&!e.shiftKey&&!e.isComposing){e.preventDefault();composer.requestSubmit();}});
  const foot=el('div','vw-composer-foot'),select=el('select');select.setAttribute('aria-label',t('Amb qui parles','Who you are talking to'));
  select.append(new Option(squadName(state.squad),'auto'));for(const w of state.winds)select.append(new Option(w.name+(roleName(w)?' · '+roleName(w):''),w.id));select.value=state.active?.wind||'auto';
  select.onchange=()=>{state.recipient=select.value;};state.recipient=select.value;
  const model=el('button','vw-model');model.type='button';paintModel(model);
  // stopPropagation: el clic que l'obre no ha d'arribar al «clic fora» que el tanca.
  model.onclick=e=>{e.stopPropagation();openModelPicker(model).catch(fail);};
  loadModels().then(()=>paintModel(model)).catch(()=>{});
  const send=el('button','vw-send');paintSend(send);
  foot.append(select,model,send);composer.append(input,foot);
  if(workspace){const line=el('div','vw-folder-hint');line.append(folder(),el('span','',state.workspace));composer.append(line);}
  composer.onsubmit=e=>{e.preventDefault();submit().catch(fail);};content.append(composer);
 }
 // Model dels vents: propi (POST /api/model amb scope «vents»), independent
 // del de la feina. Abans el botó obria el selector de la feina, que es
 // tancava amb el mateix clic, i ensenyava el model de la feina.
 const models={list:{},current:'',work:'',loaded:false};
 async function loadModels(force=false){
  if(models.loaded&&!force)return models;
  const d=await call('/api/models');
  models.list=d.models||{};models.work=d.fallback||d.current||'';models.current=d.vents_selected||'';models.loaded=true;
  return models;
 }
 const modelActual=()=>models.current||models.work||root.gregal?.state?.model||'';
 function paintModel(button){
  const full=modelActual(),name=full?full.split('/').slice(1).join('/')||full:t('Model','Model');
  const chev=document.createElementNS('http://www.w3.org/2000/svg','svg');chev.setAttribute('viewBox','0 0 24 24');chev.setAttribute('class','vw-model-chev');chev.setAttribute('aria-hidden','true');
  const path=document.createElementNS(chev.namespaceURI,'path');path.setAttribute('d','m6 9 6 6 6-6');path.setAttribute('fill','none');path.setAttribute('stroke','currentColor');path.setAttribute('stroke-width','2');path.setAttribute('stroke-linecap','round');chev.append(path);
  button.replaceChildren(el('span','vw-model-name',name),chev);
  button.title=t('Model dels vents: ','Winds model: ')+(full||'—')+(models.current?'':t(' (el de la feina)',' (same as work)'));
  button.setAttribute('aria-label',button.title);
 }
 function closeModelPicker(){document.querySelector('.vw-model-pop')?.remove();document.removeEventListener('click',outsideModel,true);document.removeEventListener('keydown',escModel,true);}
 function outsideModel(e){const pop=document.querySelector('.vw-model-pop');if(pop&&!pop.contains(e.target)&&!e.target.closest?.('.vw-model'))closeModelPicker();}
 function escModel(e){if(e.key==='Escape'){e.stopPropagation();closeModelPicker();}}
 async function openModelPicker(anchor){
  if(document.querySelector('.vw-model-pop')){closeModelPicker();return;}
  const pop=el('div','vw-model-pop');pop.setAttribute('role','dialog');pop.setAttribute('aria-label',t('Model dels vents','Winds model'));
  const head=el('p','vw-model-head',t('Model amb què responen els vents','Model the winds answer with'));
  const filter=el('input','vw-model-filter');filter.type='search';filter.placeholder=t('Filtra models…','Filter models…');filter.setAttribute('aria-label',filter.placeholder);
  const list=el('div','vw-model-list');list.setAttribute('role','listbox');list.append(el('p','vw-model-empty',t('Carregant…','Loading…')));
  pop.append(head,filter,list);document.body.append(pop);
  const place=()=>{const r=anchor.getBoundingClientRect(),w=Math.min(360,innerWidth-16);pop.style.width=w+'px';pop.style.left=Math.max(8,Math.min(r.left,innerWidth-w-8))+'px';const above=r.top>innerHeight/2;pop.style.top=above?'auto':(r.bottom+6)+'px';pop.style.bottom=above?(innerHeight-r.top+6)+'px':'auto';pop.style.maxHeight=Math.max(160,Math.min(420,above?r.top-14:innerHeight-r.bottom-14))+'px';};
  place();document.addEventListener('click',outsideModel,true);document.addEventListener('keydown',escModel,true);filter.focus();
  try{await loadModels(true);}catch(e){list.replaceChildren(el('p','vw-model-empty',t('No s’han pogut llegir els models: ','Could not read the models: ')+e.message));return;}
  const all=[];for(const prov of Object.keys(models.list).sort())for(const id of models.list[prov]||[])all.push(prov+'/'+id);
  const draw=()=>{
   const q=filter.value.trim().toLowerCase(),sel=modelActual();
   const rows=all.filter(m=>!q||m.toLowerCase().includes(q)).slice(0,200);
   list.replaceChildren();
   if(!rows.length){list.append(el('p','vw-model-empty',all.length?t('Cap model coincideix','No model matches'):t('Cap proveïdor no llista models','No provider lists models')));return;}
   let prov='';
   for(const m of rows){
    const p=m.split('/')[0];if(p!==prov){prov=p;list.append(el('p','vw-model-prov',p));}
    const row=el('button','vw-model-row');row.type='button';row.setAttribute('role','option');row.setAttribute('aria-selected',String(m===sel));
    row.append(el('span','',m.slice(p.length+1)));
    if(m===models.work)row.append(el('small','',t('feina','work')));
    row.onclick=async e=>{e.stopPropagation();try{await call('/api/model','POST',{model:m,scope:'vents'});models.current=m;closeModelPicker();document.querySelectorAll('.vw-model').forEach(paintModel);}catch(err){fail(err);}};
    list.append(row);
   }
  };
  filter.oninput=draw;filter.onkeydown=e=>{if(e.key==='Enter'){e.preventDefault();list.querySelector('.vw-model-row')?.click();}};
  draw();place();
 }
 function paintSend(button){
  button.replaceChildren();button.type=state.busy?'button':'submit';button.dataset.busy=String(state.busy);button.disabled=false;
  const label=state.busy?t('Atura la resposta','Stop response'):t('Envia','Send');button.setAttribute('aria-label',label);button.title=label;
  const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 24 24');svg.setAttribute('aria-hidden','true');
  const shape=document.createElementNS(svg.namespaceURI,state.busy?'rect':'path');
  if(state.busy){for(const [key,value]of Object.entries({x:6,y:6,width:12,height:12,rx:2,fill:'currentColor'}))shape.setAttribute(key,value);}
  else{shape.setAttribute('d','M12 19V5m-6 6 6-6 6 6');shape.setAttribute('fill','none');shape.setAttribute('stroke','currentColor');shape.setAttribute('stroke-width','1.8');shape.setAttribute('stroke-linecap','round');shape.setAttribute('stroke-linejoin','round');}
  svg.append(shape);button.append(svg);button.onclick=state.busy?()=>{if(chatController)chatController.abort();else root.gregalWindRunner?.stop();}:null;
 }
 function paintPending(){
  if(!messages||state.view!=='chat')return;const pending=state.pending;
  const old=messages.querySelector('.vw-pending');const follow=!old||messages.scrollHeight-messages.clientHeight-messages.scrollTop<64;old?.remove();
  if(!pending||pending.thread!==state.active?.id)return;
  const wind=state.winds.find(w=>w.id===pending.wind),row=el('article','vw-message vw-pending'),copy=el('div','vw-pending-copy');row.append(face(wind,pending));
  copy.append(el('strong','',wind?.name||t('La colla','Your crew')));
  const phases={thinking:t('Preparant la resposta','Preparing a response'),searching:t('Cercant informació','Searching for information'),scheduling:t('Preparant el recordatori','Preparing the reminder'),working:t('Treballant en la resposta','Working on the response')};
  const activity=el('div','vw-thinking');activity.setAttribute('role',pending.text?'presentation':'status');const dots=el('span','vw-thinking-dots');dots.setAttribute('aria-hidden','true');for(let i=0;i<3;i++)dots.append(el('i'));
  activity.append(el('span','',pending.text?t('Escrivint la resposta','Writing the response'):phases[pending.phase]||phases.thinking),dots);copy.append(activity);
  // Streamed text is formatted as it arrives, like the finished message it becomes.
  if(pending.text){const body=el('div','vw-message-text vw-stream-text'),shown=withoutSpeaker(pending.text,wind?.name).replace(/```(?:chart|gregal-plot)\n[\s\S]*?```/g,'\n_'+t('Preparant un gràfic…','Preparing a chart…')+'_\n');if(root.gregalMD)body.innerHTML=root.gregalMD.render(shown);else body.textContent=shown;copy.append(body);}
  row.append(copy);messages.append(row);if(follow)messages.scrollTop=messages.scrollHeight;
 }
 function schedulePending(){if(pendingFrame)return;pendingFrame=requestAnimationFrame(()=>{pendingFrame=0;paintPending();});}
 async function streamChat(destination,text,controller,progress={}){
  const response=await root.api(destination.legacyWind?'/api/winds/chat/stream':'/api/winds/rotllana/stream',{method:'POST',signal:controller.signal,headers:{'Content-Type':'application/json'},body:JSON.stringify(destination.legacyWind?{wind:destination.id,message:text}:{rotllana:destination.id,message:text})});
  if(!response.ok||!response.body)throw new Error(t('No s’ha pogut iniciar la resposta.','Could not start the response.'));
  const reader=response.body.getReader(),decoder=new TextDecoder();let buffer='',result=null;
  try{while(true){const chunk=await reader.read();if(chunk.done)break;buffer+=decoder.decode(chunk.value,{stream:true}).replace(/\r\n/g,'\n');let end;
   while((end=buffer.indexOf('\n\n'))>=0){const frame=buffer.slice(0,end);buffer=buffer.slice(end+2);const lines=frame.split('\n'),kind=lines.find(l=>l.startsWith('event:'))?.slice(6).trim(),raw=lines.filter(l=>l.startsWith('data:')).map(l=>l.slice(5).trimStart()).join('\n');if(!raw)continue;const data=JSON.parse(raw);
    if(kind==='error')throw new Error(root.gregalFriendlyRunError(data.error)||t('La resposta s’ha interromput.','The response was interrupted.'));
    if(kind==='done'){result=data;continue;}
    // A group voice finished and is already saved: show it in place before the next voice starts.
    if(kind==='reply'){if(Array.isArray(data.history)){progress.history=data.history;if(state.pending?.thread===destination.id)state.pending.text='';if(state.view==='chat'&&state.active?.id===destination.id){state.history=data.history;paintHistory();}}continue;}
    const pending=state.pending;if(!pending||pending.thread!==destination.id)continue;
    if(kind==='phase'){if(data.phase.startsWith('speaker:')){const next=data.phase.slice(8);if(pending.wind&&pending.wind!==next)pending.voices=(pending.voices||1)+1;pending.wind=next;}pending.phase=data.phase.startsWith('speaker:')?'thinking':data.phase;pending.text='';}
    else if(kind==='token')pending.text+=data.token||'';
    schedulePending();
   }
   if(buffer.length>1000000)throw new Error(t('La resposta és massa gran.','The response is too large.'));
  }}finally{if(!result){try{await reader.cancel();}catch{}}reader.releaseLock();}
  if(!result)throw new Error(t('La connexió s’ha interromput abans d’acabar.','The connection ended before the response was complete.'));return result;
 }
 function home(){
  content.classList.add('vw-home');const welcome=el('div','vw-welcome');welcome.append(face(crew()[0]),el('p','vw-eyebrow',squadName(state.squad)),el('h1','',t('Què fem avui?','What shall we do today?')),el('p','',t('Una idea, una pregunta o una feina. La colla t’hi acompanya.','An idea, a question or a task. Your crew is here to help.')));content.append(welcome);addComposer(!!state.active?.project);
  const prompts=el('div','vw-prompts');for(const [a,b]of state.squad==='data'?[['Analitza aquestes dades','Analyze this data'],['Compara dos models','Compare two models'],['Explora una sèrie temporal','Explore a time series']]:state.squad==='software'?[['Crea una web','Create a website'],['Revisa aquest projecte','Review this project'],['Comprova la seguretat','Check security']]:[['Ajuda’m a triar','Help me choose'],['Recomana’m un llibre','Recommend a book'],['Organitzem la setmana','Let’s organize the week']])prompts.append(btn(t(a,b),()=>{input.value=t(a,b)+' ';input.focus();}));content.append(prompts);
  const recent=state.chats.filter(c=>!c.workspace).slice(0,2);if(recent.length){content.append(el('h2','vw-subtitle',t('Repren el fil','Pick up where you left off')));for(const c of recent)content.append(btn(c.name+'  ↗',()=>openChat(c),'vw-recent'));}
  // The example explains the crew before first use; afterwards home stays focused on real threads.
  if(!state.chats.length&&!state.tasks.length)collaborationDemo();
  const strip=el('div','vw-crew-strip');crew().slice(0,5).forEach(w=>{const n=btn('',()=>talkTo(w));n.append(face(w),el('span','',w.name));strip.append(n);});if(!crew().length)strip.append(el('span','',t('Els especialistes es preparen amb el primer missatge.','Your specialists are ready with your first message.')));content.append(strip);
 }
 function collaborationDemo(){
  const examples={
   daily:{title:t('Un cap de setmana a Girona','A weekend in Girona'),turns:[
    ['organisation','Gregal',t('Primer fixem horaris i pressupost; després et proposaré un pla per dies, amb temps lliure.','Let’s settle times and budget first; then I’ll propose a day-by-day plan with some free time.')],
    ['books','Ponent',t('Hi afegeixo el Call i una llibreria del barri vell, comprovant els horaris actuals abans de recomanar-los.','I’ll add the Jewish Quarter and an old-town bookshop, checking current opening hours before recommending them.')],
    ['music','Xaloc',t('Miraré si hi ha concerts aquell dissabte i només te’ls proposaré si encaixen amb el pla.','I’ll look for concerts that Saturday and only suggest them if they fit the plan.')]
   ],summary:t('Cada especialista hi aporta la seva part i Gregal ho reuneix en un pla que pots conservar.','Each specialist adds their part and Gregal pulls it together into a plan you can keep.')},
   data:{title:t('Entendre una sèrie de vendes','Understanding a sales time series'),turns:[
    ['quality','Cerç',t('Primer revisaré dates, valors que falten i si les vendes són diàries o setmanals.','First I’ll check dates, missing values, and whether sales are daily or weekly.')],
    ['timeseries','Migjorn',t('Després separaré tendència i estacionalitat, i compararé la previsió amb una referència senzilla.','Then I’ll separate trend and seasonality, and compare the forecast with a simple baseline.')],
    ['review','Oratge',t('I comprovaré que la validació respecti l’ordre temporal i que el gràfic no suggereixi més certesa de la que tenim.','I’ll check that validation respects time order and that the chart does not imply more certainty than we have.')]
   ],summary:t('Cada especialista aporta una comprovació; la conclusió es basa en les dades i queda revisada.','Each specialist contributes a check; the conclusion is grounded in the data and reviewed.')},
   software:{title:t('Preparar una web accessible','Making a website accessible'),turns:[
    ['ux','Llebeig',t('Comencem pels usos principals i els criteris d’accessibilitat que cal complir.','Let’s start with the main user journeys and the accessibility criteria to meet.')],
    ['frontend','Tramuntana',t('Prepararé la interfície adaptable i faré servir els components i estils que ja té el projecte.','I’ll build the responsive interface using the project’s existing components and styles.')],
    ['testing','Mestral',t('Després revisaré teclat, contrast i amplades mòbils, i indicaré què he comprovat.','Then I’ll check keyboard use, contrast and mobile widths, and report what I verified.')]
   ],summary:t('Disseny, implementació i revisió s’uneixen en un sol lliurable.','Design, implementation and review come together in one deliverable.')}
  };
  const squad=examples[state.squad]?state.squad:'daily',example=examples[squad],template=state.squads.find(s=>s.id===squad);
  const participants=example.turns.map(([key,name])=>{const member=template?.members?.find(m=>m.key===key);return state.winds.find(w=>w.squad===squad&&w.specialty===key)||{name:member?.name||name,color:member?.color||'#BFC7DF',style:member?.style||'psn',squad,specialty:key};});
  const card=el('section','vw-demo');card.setAttribute('aria-label',t('Exemple de col·laboració dels vents','Example of the winds collaborating'));
  const head=el('div','vw-demo-head');const titles=el('div');titles.append(el('p','vw-demo-kicker',t('UN EXEMPLE ANIMAT','A SHORT EXAMPLE')),el('h2','',t('Mira com col·labora la colla','See the crew work together')));head.append(titles,btn(t('Torna-ho a veure','Play again'),()=>playCollaborationDemo(card,example,participants),'vw-demo-replay'));card.append(head,el('p','vw-demo-prompt',example.title));
  const thread=el('div','vw-demo-thread');thread.setAttribute('aria-live','polite');thread.setAttribute('aria-relevant','additions text');card.append(thread);content.append(card);playCollaborationDemo(card,example,participants);
 }
 function playCollaborationDemo(card,example,participants){
  clearTimeout(demoTimer);const run=++demoRun,thread=card.querySelector('.vw-demo-thread');thread.replaceChildren();card.classList.add('is-playing');
  const turns=[...example.turns.map(([key,name,text],i)=>({person:participants[i],text,role:roleName(participants[i])||name})),{person:null,role:t('La colla','The crew'),text:example.summary,summary:true}];let index=0;
  const show=()=>{if(run!==demoRun||!card.isConnected)return;const turn=turns[index],row=el('article','vw-demo-turn'+(turn.summary?' vw-demo-summary':''));if(turn.person)row.append(face(turn.person));else row.append(el('span','vw-demo-mark','✧'));const copy=el('div');copy.append(el('strong','',turn.person?.name||turn.role),el('span','vw-demo-role',turn.person?turn.role:t('Conclusió compartida','Shared takeaway')),el('p','',turn.text));row.append(copy);thread.append(row);index++;if(index<turns.length)demoTimer=setTimeout(show,1900);else card.classList.remove('is-playing');};show();
 }
 async function activate(id){await call('/api/winds/squads','POST',{id,lang:ca()?'ca':'en'});state.winds=(await call('/api/winds')).winds;renderNav();}
 async function talkTo(w){root.gregalWinds.seen?.(w.id);const saved=await call('/api/winds/chat?wind='+encodeURIComponent(w.id));if(saved.history?.length){state.active={id:w.id,wind:w.id,name:w.name,members:[w.id],legacyWind:true};state.history=saved.history;state.view='chat';state.draft='';render();return;}state.squad=w.squad||state.squad;state.active={wind:w.id,name:w.name};state.view='home';state.draft='';render();const option=[...content.querySelector('select').options].find(o=>o.value===w.id);if(option){content.querySelector('select').value=w.id;state.recipient=w.id;}input.focus();}
 function team(){heading(t('Tothom té alguna cosa a aportar.','Everyone brings something.'),t('Especialistes amb memòria pròpia i coneixement que pots conservar.','Specialists with their own memory and knowledge you can keep.'));
  const tabs=el('div','vw-tabs');for(const s of state.squads){const b=btn(ca()?s.ca:s.en,()=>{state.squad=s.id;render();});b.dataset.selected=String(s.id===state.squad);tabs.append(b);}content.append(tabs);
  const roster=crew();if(!roster.length){const empty=el('div','vw-empty');empty.append(face(),el('h2','',t('La teva colla, a punt.','Your crew, ready to help.')),el('p','',t('Activa l’equip amb un sol clic. Les especialitats ja venen preparades.','Activate this squad in one click. Specialties are already defined.')),btn(t('Prepara aquest equip','Prepare this squad'),async()=>{await activate(state.squad);render();},'vw-primary'));content.append(empty);}
  for(const w of roster){const row=el('article','vw-member');const copy=el('div');copy.append(el('h2','',w.name),el('p','',roleName(w)));row.append(face(w),copy,btn(t('Parla','Talk'),()=>talkTo(w)),btn(t('Perfil','Profile'),()=>profile(w)));content.append(row);}
  const personal=state.winds.filter(w=>!w.squad);if(personal.length){content.append(el('h2','vw-subtitle',t('Els teus vents','Your winds')));for(const w of personal){const row=el('article','vw-member');row.append(face(w),el('strong','',w.name),btn(t('Parla','Talk'),()=>talkTo(w)),btn(t('Perfil','Profile'),()=>profile(w)));content.append(row);}}
  content.append(btn(t('＋ Crea un vent propi','＋ Create your own wind'),createWind));
 }
 function dialog(title){const d=el('dialog','vw-dialog');d.setAttribute('aria-label',title);const head=el('header');head.append(el('h2','',title),btn('×',()=>d.close(),'vw-close'));d.append(head);document.body.append(d);d.addEventListener('close',()=>d.remove());d.showModal();return d;}
 function field(parent,label,value='',multiline=false){const wrap=el('label','vw-field',label),n=el(multiline?'textarea':'input');n.value=value;if(multiline)n.rows=7;wrap.append(n);parent.append(wrap);return n;}
 function createWind(){const d=dialog(t('Una nova companyia','A new companion'));const name=field(d,t('Nom','Name')),role=field(d,t('Com et pot ajudar?','How can they help?')),persona=field(d,t('Manera de ser i instruccions','Personality and instructions'),'',true);d.append(btn(t('Crea el vent','Create wind'),async()=>{if(!name.value.trim()){name.focus();return;}await call('/api/winds','POST',{name:name.value,role:role.value,persona:persona.value,color:'#C1CDA9',style:'psn'});d.close();await load();route('crew');},'vw-primary'));}
 async function profile(w){
  const data=await call('/api/winds/profile?wind='+encodeURIComponent(w.id));const d=dialog(w.name);d.append(face(w),el('p','vw-muted',roleName(w)));const tabs=el('div','vw-tabs'),body=el('div');d.append(tabs,body);
  const draw=kind=>{body.replaceChildren();for(const b of tabs.children)b.dataset.selected=String(b.dataset.kind===kind);const memory=kind==='memory';body.append(el('p','vw-muted',memory?t('Preferències, decisions i coses sobre tu que aquest vent recorda.','Preferences, decisions and things about you this wind remembers.'):t('Fets, documents i procediments que pot utilitzar per ajudar-te.','Facts, documents and procedures it can use to help you.')));const area=field(body,memory?t('Memòria','Memory'):t('Coneixement','Knowledge'),memory?data.memory:data.knowledge,true);area.maxLength=memory?8000:24576;body.append(btn(t('Desa els canvis','Save changes'),async()=>{await call(memory?'/api/winds/memoria':'/api/winds/memory',memory?'PUT':'POST',memory?{wind:w.id,memoria:area.value}:{wind:w.id,text:area.value});if(memory)data.memory=area.value;else data.knowledge=area.value;notice.textContent=t('Desat','Saved');},'vw-primary'));
   if(!memory){for(const skill of data.skills||[]){const detail=el('details','vw-document');detail.append(el('summary','',skill.Titol||skill.Nom),el('p','',skill.Cos));body.append(detail);}body.append(btn(t('＋ Afegeix un document o procediment','＋ Add a document or procedure'),()=>knowledgeDocument(w)));}
  };for(const [id,a,b]of [['memory','Memòria','Memory'],['knowledge','Coneixement','Knowledge']]){const n=btn(t(a,b),()=>draw(id));n.dataset.kind=id;tabs.append(n);}const notice=el('p','vw-status');notice.setAttribute('role','status');d.append(notice);draw('memory');
  d.append(btn(t('Arxiva el vent i conserva la seva informació','Archive wind and keep its information'),async()=>{await call('/api/winds','DELETE',{id:w.id});d.close();await load();route('library');},'vw-archive'));
 }
 function knowledgeDocument(w){const d=dialog(t('Afegeix coneixement','Add knowledge')),title=field(d,t('Títol','Title')),description=field(d,t('Quan li serà útil?','When is it useful?')),body=field(d,t('Contingut','Content'),'',true),file=el('input');file.type='file';file.accept='.md,.txt,.csv,.json';file.setAttribute('aria-label',t('Importa un document de text','Import a text document'));file.onchange=async()=>{const f=file.files[0];if(f){if(f.size>50000)throw new Error(t('El document és massa gran (màxim 50 KB).','Document too large (maximum 50 KB).'));body.value=await f.text();if(!title.value)title.value=f.name;}};d.append(file,btn(t('Desa el document','Save document'),async()=>{if(!title.value.trim()){title.focus();return;}await call('/api/winds/coneixements','PUT',{wind:w.id,nom:title.value,titol:title.value,descripcio:description.value||t('Document de referència: ','Reference document: ')+title.value,cos:body.value});d.close();},'vw-primary'));}
 async function library(){heading(t('Allò que anem aprenent.','What we learn along the way.'),t('La memòria i el coneixement es poden conservar, recuperar i compartir entre vents.','Memory and knowledge can be kept, recovered and shared between winds.'));const tabs=el('div','vw-tabs');for(const [id,a,b]of [['all','Tot','All'],['active','Actius','Active'],['archived','Arxivats','Archived']]){const n=btn(t(a,b),()=>{state.filter=id;render();});n.dataset.selected=String(state.filter===id);tabs.append(n);}content.append(tabs);const search=el('input','vw-search');search.placeholder=t('Cerca a la biblioteca…','Search the library…');search.setAttribute('aria-label',search.placeholder);content.append(search);const list=el('div','vw-library-list');content.append(list);const target=content;
  try{state.library=(await call('/api/winds/library')).items||[];if(content!==target)return;
   const draw=()=>{list.replaceChildren();const items=state.library.filter(i=>(state.filter==='all'||state.filter==='active'&&i.kind==='active'||state.filter==='archived'&&['archived','recovered'].includes(i.kind))&&(i.wind.name+' '+i.memory+' '+i.knowledge).toLowerCase().includes(search.value.toLowerCase()));for(const item of items){const row=el('article','vw-library-item');const copy=el('div');copy.append(el('h2','',item.wind.name),el('p','vw-muted',(item.kind==='active'?t('Actiu','Active'):item.kind==='version'?t('Còpia abans d’un traspàs','Copy before a transfer'):t('Arxivat · recuperable','Archived · recoverable'))),el('p','',(item.memory?t('Memòria','Memory'):'')+(item.memory&&item.knowledge?' · ':'')+(item.knowledge?t('Coneixement','Knowledge'):'')));row.append(face(item.wind),copy);const actions=el('div','vw-actions');actions.append(btn(t('Consulta','View'),()=>{const d=dialog(item.wind.name);d.append(el('h3','',t('Memòria','Memory')),el('pre','vw-library-text',item.memory||'—'),el('h3','',t('Coneixement','Knowledge')),el('pre','vw-library-text',item.knowledge||'—'));for(const sk of item.skills||[])d.append(el('h3','',sk.Titol||sk.Nom),el('pre','vw-library-text',sk.Cos));}));if(item.kind!=='active'&&item.kind!=='version'&&!state.winds.some(w=>w.id===item.wind.id))actions.append(btn(t('Recupera','Restore'),async()=>{await call('/api/winds/library','POST',{action:'restore',source:item.id});await load();route('library');}));actions.append(btn(t('Assigna…','Assign…'),()=>transfer(item)),btn(t('Exporta','Export'),()=>exportItem(item)));row.append(actions);list.append(row);}if(!items.length)list.append(el('p','vw-empty',t('Encara no hi ha informació en aquesta secció.','No information in this section yet.')));};search.oninput=draw;draw();
  }catch(e){fail(e);}
 }
 function transfer(item){const d=dialog(t('Comparteix el que sap','Share what they know'));d.append(el('p','vw-muted',t('S’afegeix a la informació del destí. Es conserva una còpia prèvia; els documents en conflicte no se sobreescriuen.','Adds to the destination’s information. A previous copy is kept; conflicting documents are not overwritten.')));const select=el('select');select.setAttribute('aria-label',t('Vent de destí','Destination wind'));for(const w of state.winds.filter(w=>w.id!==item.wind.id))select.append(new Option(w.name,w.id));d.append(select);const checks={};for(const [key,a,b]of [['memory','Memòria','Memory'],['knowledge','Coneixement i documents','Knowledge and documents']]){const label=el('label','vw-check'),box=el('input');box.type='checkbox';box.checked=true;checks[key]=box;label.append(box,el('span','',t(a,b)));d.append(label);}d.append(btn(t('Assigna la informació','Assign information'),async()=>{await call('/api/winds/library','POST',{action:'assign',source:item.id,target:select.value,memory:checks.memory.checked,knowledge:checks.knowledge.checked});d.close();render();},'vw-primary'));}
 async function exportItem(item){const response=await root.api('/api/winds/library?export='+encodeURIComponent(item.id));if(!response.ok)throw new Error(t('No s’ha pogut exportar','Export failed'));const url=URL.createObjectURL(await response.blob()),link=el('a');link.href=url;link.download='wind-information.json';link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);}
 async function chooseFolder(){if(root.gregalPanels?.workspaces?.pick)return root.gregalPanels.workspaces.pick({title:t('Obre un projecte','Open a project'),onChoose:async path=>{try{await switchFolder(path);await refreshProjects();await openProject(state.projects.find(p=>p.path===state.workspace)||{path:state.workspace,name:state.workspace.split(/[\\/]/).pop()});return true;}catch(e){fail(e);return false;}}});const d=dialog(t('Obre un projecte','Open a project'));d.append(el('p','vw-muted',t('Les converses i els encàrrecs d’aquest projecte treballaran en aquesta carpeta.','This project’s conversations and tasks will work in this folder.')));const path=field(d,t('Carpeta del projecte','Project folder'),state.workspace);d.append(btn(t('Obre la carpeta','Open folder'),async()=>{await switchFolder(path.value);d.close();await refreshProjects();await openProject(state.projects.find(p=>p.path===state.workspace)||{path:state.workspace,name:state.workspace.split(/[\\/]/).pop()});},'vw-primary'));}
 async function switchFolder(path){if(path!==state.workspace){await call('/api/workspaces','POST',{path});state.workspace=path;await root.gregal?.refresh?.();}}
 async function refreshProjects(){const data=await call('/api/workspaces');state.projects=data.workspaces||[];state.workspace=data.current;}
 async function openProject(project){await switchFolder(project.path);route('project',project);}
 function projectView(){heading(state.active.name,t('Un lloc per conversar, treballar i trobar els resultats.','A place to talk, work and find the results.'));const path=el('p','vw-project-path');path.append(folder(),el('span','',state.active.path));content.append(path);content.append(btn(t('＋ Nova conversa al projecte','＋ New project conversation'),()=>{state.active={...state.active,project:true};state.view='home';render();}));const chats=state.chats.filter(c=>c.workspace===state.active.path);for(const c of chats)content.append(btn(c.name+' ↗',()=>openChat(c),'vw-recent'));taskList(state.tasks.filter(task=>task.workspace===state.active.path));}
 function tasks(){heading(t('La feina, a la vista.','Your work, in view.'),t('Progrés real, comprovacions i resultats que pots obrir.','Real progress, checks and results you can open.'));if(state.busy)content.append(btn(t('Torna a la feina en marxa','Return to current work'),()=>route('task',state.liveActive||{name:state.liveSnapshot?.task||'',live:true}),'vw-recent'));taskList(state.tasks);const routines=el('section');content.append(routines);showRoutines(routines).catch(fail);}
 
async function wikiView(){
  heading(t('Wiki dels vents','Winds wiki'),t('Resums que es queden: el que s\'ha parlat, en pàgines.','Summaries that stay: what was talked about, as pages.'));
  const data=await call('/api/winds/wiki');if(!content.isConnected)return;
  const pages=data.pages||[];
  if(!pages.length){content.append(el('p','vw-empty',t('Encara no hi ha pàgines. Obre una conversa i tria «Desa a la wiki».','No pages yet. Open a conversation and choose “Save to wiki”.')));return;}
  for(const p of pages){
    const row=btn('',()=>openWikiPage(p),'vw-wiki-row');
    row.append(el('strong','',p.titol||p.nom));
    if(p.descripcio)row.append(el('span','',p.descripcio));
    row.append(el('small','',wikiMeta(p)));
    content.append(row);
  }
}
const wikiMeta=p=>[p.autor,p.updated?new Date(p.updated).toLocaleDateString(document.documentElement.lang||undefined,{day:'numeric',month:'short',year:'numeric'}):''].filter(Boolean).join(' · ');
async function openWikiPage(p){
  const d=dialog(p.titol||p.nom);d.classList.add('vw-wiki-dialog');
  let full;try{full=(await call('/api/winds/wiki/'+encodeURIComponent(p.nom))).page;}catch(e){full=p;}
  if(full.descripcio)d.append(el('p','vw-wiki-lede',full.descripcio));
  const meta=wikiMeta(full);if(meta)d.append(el('p','vw-muted',meta));
  const body=el('div','vw-result-text');
  if(root.gregalMD)body.innerHTML=root.gregalMD.render(full.cos||'');else body.textContent=full.cos||'';
  d.append(body,btn(t('Esborra la pàgina','Delete page'),async()=>{if(!confirm(t('Esborrar aquesta pàgina de la wiki?','Delete this wiki page?')))return;await call('/api/winds/wiki/'+encodeURIComponent(p.nom),'DELETE');d.close();route('wiki');},'vw-archive'));
}
// Resumeix la conversa oberta en una pàgina; si ja en tenia, el servidor l'actualitza.
async function summarizeToWiki(){
  const chat=state.active;if(!chat?.id)return;
  status.removeAttribute('role');status.textContent=t('Resumint la conversa per a la wiki…','Summarizing the conversation for the wiki…');
  const data=await call('/api/winds/wiki/summary','POST',chat.legacyWind?{wind:chat.wind||chat.id}:{rotllana:chat.id});
  status.textContent=(data.updated?t('Pàgina actualitzada: ','Page updated: '):t('Pàgina creada: ','Page created: '))+(data.page?.titol||'');
  await openWikiPage(data.page);
}
async function showRoutines(container){container.replaceChildren(el('h2','vw-subtitle',t('Rutines','Routines')),el('p','vw-muted',t('Cerques i resums recurrents. Mantén Gregal obert perquè s’executin.','Recurring searches and summaries. Keep Gregal open for them to run.')),btn(t('＋ Una nova rutina','＋ New routine'),newRoutine));const data=await call('/api/jobs');if(!container.isConnected)return;for(const job of (data.jobs||[]).filter(j=>j.wind)){const row=el('article','vw-routine');row.append(el('strong','',job.name),el('span','vw-muted',(job.enabled?t('Activa','Active'):t('Pausada','Paused'))+' · '+(job.next_due||job.time||'')),btn(job.enabled?t('Pausa','Pause'):t('Activa','Enable'),async()=>{await call('/api/jobs/save','POST',{...job,enabled:!job.enabled});await showRoutines(container);}));row.append(btn(t('Resultats','Results'),async()=>{const data=await call('/api/jobs/runs?job='+encodeURIComponent(job.id));const d=dialog(job.name);for(const run of data.runs||[]){const body=el('div','vw-result-text');const shown=run.output||(run.error?root.gregalFriendlyRunError(run.error):'');if(root.gregalMD)body.innerHTML=root.gregalMD.render(shown||'');else body.textContent=shown||'';d.append(el('p','vw-muted',new Date(run.finished_at).toLocaleString()),body);}if(!(data.runs||[]).length)d.append(el('p','vw-muted',t('Encara no s’ha executat.','No runs yet.')));}));container.append(row);}}
 async function newRoutine(){if(!crew().length)await activate(state.squad);const d=dialog(t('Una petita ajuda, cada dia.','A little help, every day.'));const name=field(d,t('Nom de la rutina','Routine name')),prompt=field(d,t('Què vols que cerqui o resumeixi?','What should it research or summarize?'),state.draft,true),time=field(d,t('Cada dia a les','Every day at'),'09:00');time.type='time';const select=el('select');select.setAttribute('aria-label',t('Vent responsable','Responsible wind'));for(const w of state.winds)select.append(new Option(w.name+' · '+roleName(w),w.id));select.value=crew()[0]?.id||state.winds[0]?.id;d.append(select,el('p','vw-muted',t('Fa servir la memòria i el coneixement actuals del vent. És una rutina de lectura i cerca; Gregal ha d’estar obert.','Uses the wind’s current memory and knowledge. This is a reading and research routine; Gregal must stay open.')),btn(t('Desa la rutina','Save routine'),async()=>{if(!name.value.trim()||!prompt.value.trim()){(!name.value.trim()?name:prompt).focus();return;}await call('/api/jobs/save','POST',{name:name.value,prompt:prompt.value,kind:'daily',time:time.value,enabled:true,notify:false,wind:select.value});d.close();route('tasks');},'vw-primary'));}
 // Cada feina diu qui hi ha treballat, quan, en quin projecte i què ha deixat.
 function taskList(items){for(const task of items){const row=btn('',()=>{state.events=task.events||[];state.snapshot=null;route('task',{...task,name:task.task});},'vw-task-row');
   const who=el('span','vw-task-faces');for(const name of Object.values(task.agents||{}))who.append(face(state.winds.find(w=>w.name===name)||{name,color:'#BFC7DF',style:'psn'}));
   const files=[...new Set((task.events||[]).filter(e=>e.type==='tool_result'&&e.path).map(e=>e.path.split(/[\\/]/).pop()))];
   const head=el('div','vw-task-head');head.append(who,el('strong','',task.task),el('span','vw-task-state is-'+(['done','completed'].includes(task.status)?'done':['failed','cancelled'].includes(task.status)?'stopped':'running'),statusName(task.status)));
   const meta=[(task.workspace||'').replace(/[\\/]+$/,'').split(/[\\/]/).pop(),task.created?new Date(task.created).toLocaleString(document.documentElement.lang||undefined,{day:'numeric',month:'short',hour:'2-digit',minute:'2-digit'}):'',files.slice(0,3).join(', ')].filter(Boolean).join(' · ');
   row.append(head,el('small','',meta));content.append(row);}
  if(!items.length)content.append(el('p','vw-empty',t('Els encàrrecs que facis apareixeran aquí.','Your tasks will appear here.')));}
 // Icones de la navegació dels vents: traç fi com les del rail, no caràcters Unicode.
 function navIcon(id){const paths={home:'M3 7.5 8 3.5l5 4V13H3Z M6.5 13V9.5h3V13',tasks:'M8 2.75a5.25 5.25 0 1 0 0 10.5 5.25 5.25 0 0 0 0-10.5Z M8 5v3.2l2.2 1.3',crew:'M5.5 7.5a2 2 0 1 0 0-4 2 2 0 0 0 0 4Z M10.8 7.2a1.7 1.7 0 1 0 0-3.4 M2.5 13c.4-2.2 1.6-3.4 3-3.4s2.6 1.2 3 3.4 M10 9.7c1.4.1 2.6 1.2 3 3.3',library:'M3 3.5h3v9H3Z M7 3.5h3v9H7Z M11 4.2l2.4-.6 1.9 8.4-2.4.6Z',wiki:'M4 2.75h5.5L12 5.25v8H4Z M9.5 2.75v2.5H12 M6 8h4 M6 10.5h4'};const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 16 16');svg.setAttribute('aria-hidden','true');svg.setAttribute('class','vw-nav-svg');const p=document.createElementNS(svg.namespaceURI,'path');p.setAttribute('d',paths[id]||'');p.setAttribute('fill','none');p.setAttribute('stroke','currentColor');p.setAttribute('stroke-width','1.3');p.setAttribute('stroke-linecap','round');p.setAttribute('stroke-linejoin','round');svg.append(p);return svg;}
 async function openChat(chat){if(state.preparing&&!state.busy)return;const ticket=state.chatEpoch=(state.chatEpoch||0)+1;const data=await call('/api/winds/rotllana?rotlla='+encodeURIComponent(chat.id));if(ticket!==state.chatEpoch)return;state.active=chat;state.history=data.history||[];state.squad=state.winds.find(w=>chat.members.includes(w.id))?.squad||state.squad;state.view='chat';state.draft='';render();followCrew(chat);}
 // Una colla convocada treballa al servidor: el fil s'actualitza sol mentre parlen i fan la feina.
 function followCrew(chat){root.gregalCrew?.follow(chat.id,async st=>{if(state.view!=='chat'||state.active?.id!==chat.id)return false;const steps={discussing:t('La colla en parla…','The crew is discussing…'),waiting:t('La colla espera torn…','The crew is waiting…'),working:t('La colla hi treballa…','The crew is working…')};status.textContent=st.running?steps[st.step]||'':'';const data=await call('/api/winds/rotllana?rotlla='+encodeURIComponent(chat.id));if(state.active?.id!==chat.id)return false;if((data.history||[]).length!==state.history.length){state.history=data.history||[];paintHistory();}});}
 function chatView(){content.classList.add('vw-thread-view');messages=el('div','vw-messages');content.append(messages);paintHistory();const linked=state.tasks.filter(task=>task.thread===state.active.id);for(const task of linked){const card=btn(task.task+' · '+statusName(task.status)+' ↗',()=>{state.events=task.events||[];route('task',{...task,name:task.task,conversation:state.active});},'vw-recent');messages.append(card);}addComposer(!!state.active?.workspace);}
 function paintHistory(){messages.replaceChildren();for(const item of state.history){const user=item.from==='user';const w=state.winds.find(w=>w.id===item.speaker_id||w.id===item.from||w.name===item.from)||(state.active?.legacyWind?state.winds.find(w=>w.id===state.active.wind):null);const row=el('article',user?'vw-message vw-user':'vw-message');if(!user)row.append(face(w));const copy=el('div'),body=el('div','vw-message-text'),text=user?item.text:withoutSpeaker(item.text,w?.name||item.speaker_name);if(root.gregalMD)body.innerHTML=root.gregalMD.render(text);else body.textContent=text;copy.append(el('strong','',user?t('Tu','You'):w?.name||item.speaker_name||t('La colla','Your crew')),body);row.append(copy);messages.append(row);}paintPending();messages.scrollTop=messages.scrollHeight;}
 const isWork=task=>/\b(crea|crear|create|build|make|fes|fer|fix|arregla|implement|modifica|edita|desenvolupa|programa|executa|run|revisa|review|analitza|analy[sz]e)\b/i.test(task.normalize('NFD').replace(/[\u0300-\u036f]/g,''));
 async function submit(){if(state.preparing)return;state.preparing=true;try{await submitRequest();}finally{state.preparing=false;}}
 async function submitRequest(){
  const text=input.value.trim();if(!text||state.busy)return;state.draft=text;
  if(state.view==='home'&&state.recipient==='auto'&&state.squad==='daily'&&isWork(text)){
   if(/\b(web|website|codi|code|app|bug|html|seguretat|security)\b/i.test(text))state.squad='software';
   else if(/\b(dades|data|regress|machine learning|series|temporal|csv)\b/i.test(text))state.squad='data';
  }
  if(!crew().length&&state.recipient==='auto')await activate(state.squad);
  const members=state.recipient&&state.recipient!=='auto'?state.winds.filter(w=>w.id===state.recipient):crew();if(!members.length)throw new Error(t('Tria un vent de la colla.','Choose a wind from the crew.'));
  // Cap vent no hi encaixa: proposa convocar una colla (vents existents + especialistes nous).
  if(isWork(text)&&state.recipient==='auto'&&state.view!=='chat'&&root.gregalCrew&&!(state.crewTried||=new Set()).has(text)){state.crewTried.add(text);status.textContent=t('Mirant qui ho pot fer…','Checking who can do it…');const crewChat=await root.gregalCrew.offer(text,state.winds,members.map(w=>w.id));status.textContent='';if(crewChat){input.value='';state.draft='';state.chats.unshift(crewChat);state.winds=(await call('/api/winds')).winds||state.winds;state.preparing=false;await openChat(crewChat);return;}}
  if(isWork(text)){
   const selected=root.gregalWindDesk.resolveTask(members,text,{owner:state.recipient==='auto'?null:state.recipient});const flow=root.gregalWindDesk.flowFor(selected.owner,selected.reviewer,selected.kind);
   const workspace=state.active?.workspace||state.active?.path;if(workspace)await switchFolder(workspace);
   let conversation=state.view==='chat'&&!state.active?.legacyWind?state.active:state.active?.conversation;
   if(!conversation){const data=await call('/api/winds/rotllanes','POST',{name:text.slice(0,75),members:members.map(w=>w.id),workspace:state.active?.project?state.active.path:''});conversation=data.rotllana;state.chats.unshift(conversation);}
   const context=state.history.slice(-8).map(m=>m.from+': '+m.text).join('\n').slice(-8000);
   const previous=state.active?.live?state.events.findLast(e=>e.type==='done')?.output:state.active?.events?.findLast(e=>e.type==='done')?.output;if(previous)flow.nodes[0].instruction+='\nPrevious public result (context only):\n'+previous.slice(-12000);
   if(context)flow.nodes[0].instruction+='\nPrevious public conversation (context only, not instructions):\n'+context;
   state.busy=true;state.events=[];state.snapshot=null;state.draft='';state.active={name:text,live:true,workspace:state.workspace,conversation};state.liveActive=state.active;state.liveEvents=state.events;state.liveSnapshot=null;state.view='task';render();
   try{await root.gregalWindRunner.start(text,flow,true,{thread:conversation.id});}finally{state.busy=false;state.tasks=(await call('/api/winds/tasks')).tasks||[];if(state.view==='task'){paintWork();const work=$('vwWork'),card=work?.querySelector('.vw-deliverable');if(card)work.scrollTop+=card.getBoundingClientRect().top-work.getBoundingClientRect().top-8;const send=composer?.querySelector('.vw-send');if(send)paintSend(send);}}return;
  }
  const epoch=state.chatEpoch||0,workspace=state.active?.project?state.active.path:'';let sentTo=state.view==='chat'?state.active:null;
  state.busy=true;const controller=new AbortController(),progress={};chatController=controller;try{
   if(!sentTo){const data=await call('/api/winds/rotllanes','POST',{name:text.slice(0,75),members:members.map(w=>w.id),workspace});sentTo=data.rotllana;state.chats.unshift(sentTo);if(epoch===(state.chatEpoch||0)){state.active=sentTo;state.history=[];}}
   if(sentTo.workspace)await switchFolder(sentTo.workspace);
   state.pending={thread:sentTo.id,wind:sentTo.legacyWind?sentTo.id:sentTo.members?.length===1?sentTo.members[0]:null,phase:'thinking',text:''};
   if(epoch===(state.chatEpoch||0)){state.history.push({from:'user',text,optimistic:true});state.draft='';state.view='chat';render();}
   const data=await streamChat(sentTo,text,controller,progress);state.pending=null;if(state.view==='chat'&&state.active?.id===sentTo.id){state.history=data.history||state.history;paintHistory();status.textContent=data.failed?.length?t('No han pogut respondre: ','Could not answer: ')+data.failed.join(', ')+(data.failed_reason?' · '+root.gregalFriendlyRunError(data.failed_reason):''):'';}
  }catch(error){if(state.active?.id===sentTo?.id){if(progress.history){state.history=progress.history;state.draft='';}else{state.draft=text;state.history=state.history.filter(m=>!m.optimistic);}}if(!controller.signal.aborted)throw error;}finally{if(chatController===controller)chatController=null;state.pending=null;state.busy=false;if(state.view==='chat')render();}
 }
 const statusName=value=>({started:t('En marxa','Running'),working:t('En marxa','Running'),done:t('Enllestit','Complete'),completed:t('Enllestit','Complete'),failed:t('Cal revisar-ho','Needs attention'),cancelled:t('Aturat','Stopped'),waiting:t('En espera','Waiting')})[value]||value;
 function taskView(){content.classList.add('vw-thread-view');const title=state.active?.name||state.snapshot?.task||'';heading(title);if(state.active?.conversation)content.append(btn(t('← Torna a la conversa','← Back to conversation'),()=>openChat(state.active.conversation),'vw-back'));const work=el('div','vw-work');work.id='vwWork';content.append(work);paintWork();addComposer(true);input.placeholder=t('Una altra cosa que vulguis fer…','Something else you’d like to do…');}
 function paintWork(){const work=$('vwWork');if(!work)return;const previousScroll=work.scrollTop,following=work.scrollHeight-work.clientHeight-work.scrollTop<48,logOpen=work.querySelector('.vw-log')?.open||false,reviewOpen=work.querySelector('.vw-review')?.open||false;work.replaceChildren();const snapshot=state.active?.live?(state.liveSnapshot||state.snapshot):null;const busy=state.active?.live&&state.busy;work.append(el('p','vw-progress',statusName(snapshot?.state||state.active?.status||(busy?'started':'done'))));
  for(const n of snapshot?.nodes||[]){const row=el('div','vw-live-agent');row.append(face(state.winds.find(w=>n.wind?w.id===n.wind:w.name===n.name),n),el('strong','',n.name),el('span','',statusName(n.state)+(n.activity?' · '+activityName(n.activity):'')));work.append(row);}
  const events=state.active?.live?(state.liveEvents||state.events):state.active?.events||state.events;const paths=[...new Set(events.filter(e=>e.type==='tool_result'&&e.path).map(e=>e.path))];
  // The deliverable leads; the reviewer's verdict is folded under the builder's own summary.
  const outputs=events.filter(e=>e.output),review=outputs.findLast(e=>e.type==='completed'&&e.activity==='review'),made=outputs.findLast(e=>e.type==='completed'&&e.activity!=='review'),final=outputs.findLast(e=>e.type==='done')?.output||outputs.findLast(e=>e.type==='completed')?.output||'',summary=review?made?.output||'':final;
  const markdown=text=>{const n=el('div','vw-result-text');if(root.gregalMD)n.innerHTML=root.gregalMD.render(text);else n.textContent=text;return n;};
  if(paths.length||final){const card=el('section','vw-deliverable');card.append(el('h2','',t('El resultat','The result')));for(const path of paths){const b=btn(path.split(/[\\/]/).pop()+' ↗',async()=>{await switchFolder(state.active?.workspace||state.workspace);await root.gregalWindRunner.openResult(summary||final,titleSafe(),path);},/\.html?$/i.test(path)?'vw-primary':'vw-button');b.title=path;card.append(b);}if(!paths.length)card.append(btn(t('Obre el resultat','Open result'),()=>root.gregalWindRunner.openResult(summary||final,titleSafe()),'vw-primary'));work.append(card);}
  if(summary)work.append(markdown(summary));
  if(review){const details=el('details','vw-review');details.append(el('summary','',t('Revisió de ','Review by ')+agentName(review.agent)),markdown(review.output));details.open=reviewOpen;work.append(details);}
  const lines=events.map(logLine).filter(Boolean),log=el('details','vw-log');log.append(el('summary','',t('Activitat i comprovacions','Activity and checks')+' · '+lines.length));for(const [who,what,detail] of lines.slice(-40)){const row=el('p');if(who)row.append(el('strong','',who+' '));row.append(el('span','',what));if(detail)row.append(el('span','vw-log-detail',detail));log.append(row);}log.open=logOpen;work.append(log);work.scrollTop=following&&busy?work.scrollHeight:previousScroll;
  if(!busy&&['started','working'].includes(state.active?.status))work.append(el('p','vw-muted',t('Aquesta feina es va interrompre. Pots tornar a encarregar-la.','This task was interrupted. You can submit it again.')));
 }
 const titleSafe=()=>state.active?.name||t('Resultat','Result');
 // Flow seats (builder, reviewer…) are shown by the wind that sits in them, live or from the saved record.
 const agentName=agent=>(state.active?.live?(state.liveSnapshot||state.snapshot)?.nodes?.find(n=>n.id===agent)?.name:'')||state.active?.agents?.[agent]||({builder:t('Encarregat','Builder'),reviewer:t('Revisor','Reviewer'),researcher:t('Recerca','Researcher'),coordinator:t('Coordinació','Coordinator')})[agent]||agent||'';
 const clip=(text,max)=>{const value=String(text||'').replace(/\s+/g,' ').trim();return value.length>max?value.slice(0,max-1)+'…':value;};
 // Tool arguments arrive as truncated JSON: keep only the one field a person would recognise.
 function toolDetail(args){const m=String(args||'').match(/"(file_path|path|pattern|query|url|command)"\s*:\s*"((?:[^"\\]|\\.)*)/);if(!m)return '';const value=m[2].replace(/\\n/g,' ').replace(/\\(.)/g,'$1');return clip(/path$/.test(m[1])?value.split(/[\\/]/).pop():value,80);}
 // One readable line per public event; raw tool output stays out of the activity view.
 function logLine(event){const who=agentName(event.agent);switch(event.type){
  case 'tool':return [who,activityName(event.activity),toolDetail(event.message)];
  case 'tool_result':return event.path?[who,t('Fitxer desat','File saved'),event.path.split(/[\\/]/).pop()]:null;
  case 'reviewing':return null;
  case 'review':return [who,t('Acció revisada','Action reviewed'),clip(event.message,140)];
  case 'approval':return [who,t('Demana permís','Asks for permission'),toolDetail(event.message)];
  case 'handoff':return [who,t('Passa la feina a ','Hands over to ')+agentName(event.to)];
  case 'discussion':return [who,t('Demana canvis','Requests changes'),clip(event.message,140)];
  case 'completed':return [who,event.activity==='review'?t('Revisió acabada','Review finished'):t('Part enllestida','Part complete')];
  case 'failed':return [who,t('No s’ha pogut acabar','Could not finish'),clip(event.message,140)];
  case 'done':case 'cancelled':return [who,statusName(event.type)];
  default:return [who,activityName(event.activity||event.type)];
 }}
 function activityName(value){return ({read:t('Llegint fitxers','Reading files'),write:t('Escrivint fitxers','Writing files'),edit:t('Editant fitxers','Editing files'),patch:t('Editant fitxers','Editing files'),glob:t('Buscant fitxers','Finding files'),grep:t('Cercant al codi','Searching the code'),lsp:t('Analitzant el codi','Analysing the code'),bash:t('Comprovant el projecte','Checking the project'),bash_background:t('Comprovant el projecte','Checking the project'),bash_output:t('Comprovant el projecte','Checking the project'),bash_kill:t('Aturant una comprovació','Stopping a check'),browser:t('Fent servir el navegador','Using the browser'),preview:t('Previsualitzant','Previewing'),read_image:t('Mirant una imatge','Viewing an image'),web_search:t('Cercant fonts','Searching sources'),web_fetch:t('Llegint fonts','Reading sources'),skill:t('Consultant un coneixement','Consulting knowledge'),knowledge_search:t('Consultant un coneixement','Consulting knowledge'),knowledge_note:t('Desant una nota','Saving a note'),todowrite:t('Organitzant els passos','Organising the steps'),todoread:t('Organitzant els passos','Organising the steps'),office_create:t('Treballant en un document','Working on a document'),office_edit:t('Treballant en un document','Working on a document'),office_read:t('Llegint un document','Reading a document'),office_open:t('Obrint un document','Opening a document'),office_render:t('Previsualitzant un document','Previewing a document'),git_commit:t('Desant els canvis a git','Committing changes'),delegate:t('Delegant una part','Delegating a part'),question:t('Preparant una pregunta','Preparing a question'),draft:t('Preparant el lliurable','Preparing the deliverable'),analyze:t('Analitzant','Analysing'),plan:t('Planificant','Planning'),revision:t('Corregint','Revising'),reviewing:t('Revisant una acció','Reviewing an action'),review:t('Revisant el resultat','Reviewing the result'),tool_result:t('Comprovació acabada','Check complete'),working:t('Treballant','Working'),done:t('Enllestit','Complete'),completed:t('Pas completat','Step complete'),started:t('Començant','Starting')})[value]||(/^[\w.:-]+$/.test(value||'')?t('Fent servir una eina','Using a tool'):value);}
 async function load(){mount();root.gregalWinds.watchNews?.();const version=++loadVersion;try{const [winds,squads,chats,tasks]=await Promise.all([call('/api/winds'),call('/api/winds/squads'),call('/api/winds/rotllanes'),call('/api/winds/tasks')]);if(version!==loadVersion)return;state.winds=winds.winds||[];state.squads=squads.squads||[];state.chats=chats.rotllanes||[];state.tasks=tasks.tasks||[];await refreshProjects();render();}catch(e){render();fail(e);}}
 root.gregalWindWorkspace={update(snapshot){if(snapshot.source!=='winds')return;state.liveSnapshot=snapshot;if(state.active?.live)state.snapshot=snapshot;paintWork();},event(event){const events=state.liveEvents||state.events;events.push(event);if(events.length>160)events.shift();paintWork();},load};
 // Error de connexió del proveïdor traduït a humà (mateixa taula que Go i l'app).
 root.gregalFriendlyRunError=function(raw){const e=(raw||'').trim();if(!e)return e;const l=e.toLowerCase();const m=e.match(/lookup ([a-z0-9.-]+)/i);const host=m?m[1]:'';if(l.includes('no such host')||l.includes('dns'))return host?t('No s’ha pogut arribar al proveïdor: el dispositiu no resol l’adreça '+host+'. Comprova que hi ha internet i torna-ho a provar.','Could not reach the provider: the device cannot resolve '+host+'. Check the connection and try again.'):t('No s’ha pogut arribar al proveïdor: no es resol la seva adreça. Comprova que hi ha internet i torna-ho a provar.','Could not reach the provider: its address cannot be resolved. Check the connection and try again.');if(l.includes('connection refused'))return t('No s’ha pogut arribar al proveïdor (connexió refusada). Pot ser un tall puntual: torna-ho a provar més tard.','Could not reach the provider (connection refused). Might be a transient outage: try again later.');if(l.includes('context deadline exceeded')||l.includes('timeout'))return t('El proveïdor ha trigat massa a respondre. Torna-ho a provar: si continua així, pot estar saturat.','The provider took too long to respond. Try again: if it persists it may be overloaded.');if(l.includes('certificate')||l.includes('x509'))return t('El proveïdor presenta un certificat que no es confia. Comprova la data/hora del dispositiu.','The provider certificate is not trusted. Check the device date/time.');if(l.includes('401')||l.includes('unauthorized'))return t('El proveïdor ha rebutjat la clau (401). Revisa la clau API a la configuració.','The provider rejected the key (401). Check the API key in settings.');if(l.includes('429')||l.includes('too many requests'))return t('El proveïdor ha limitat les peticions (429). Espera una mica i torna-ho a provar.','The provider is rate limiting (429). Wait a bit and try again.');if(l.includes('no route to host')||l.includes('network is unreachable'))return t('No hi ha ruta cap al servidor. Comprova que hi ha internet i torna-ho a provar.','No route to the server. Check the connection and try again.');if(l==='eof'||l.endsWith('eof'))return t('La connexió amb el proveïdor s’ha tallat a mig camí. Torna-ho a provar.','The connection to the provider was cut midway. Try again.');return e;};
 root.loadWindsPage=load;
 document.addEventListener('gregal:idioma',()=>{if(mounted){state.draft=input?.value??state.draft;render();}});
 document.addEventListener('gregal:state',()=>{page?.querySelectorAll('.vw-model').forEach(paintModel);});
})(window);
