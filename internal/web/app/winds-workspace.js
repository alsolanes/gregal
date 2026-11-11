// One shared crew, two complementary contexts: conversation and project work.
(function(root){
 'use strict';
 const $=id=>document.getElementById(id), ca=()=>document.documentElement.lang==='ca', t=(a,b)=>{const v=ca()?a:b;return window.gregalCollective?window.gregalCollective.apply(v):v;};
 const state={asks:[],winds:[],squads:[],chats:[],tasks:[],projects:[],workspace:'',view:'home',squad:'daily',active:null,history:[],busy:false,draft:'',snapshot:null,events:[],library:[],filter:'all',meta:null};
 let mounted=false, page, nav, content, messages, composer, input, status, loadVersion=0, chatTurn=null, pendingTimer=0, demoTimer=0, demoRun=0;
 const el=(tag,cls,text)=>{const n=document.createElement(tag);if(cls)n.className=cls;if(text!==undefined)n.textContent=text;return n;};
 const accent=()=>{const v=getComputedStyle(document.documentElement).getPropertyValue('--accent').trim();return /^#[0-9a-f]{6}$/i.test(v)?v:'#8193FF';};
 const btn=(label,action,cls='vw-button')=>{const n=el('button',cls,label);n.type='button';n.onclick=()=>Promise.resolve().then(action).catch(fail);return n;};
 async function call(url,method='GET',body){const r=await root.api(url,{method,...(body===undefined?{}:{headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})});if(!r.ok)throw new Error(await r.text());return r.json();}
 function fail(error){let target=document.querySelector('.vw-dialog[open] .vw-error');const modal=document.querySelector('.vw-dialog[open]');if(modal&&!target){target=el('p','vw-error');modal.append(target);}target=target||status;if(target){target.textContent=t('No s’ha pogut completar: ','Could not complete: ')+error.message;target.setAttribute('role','alert');}}
 const face=(w,work)=>root.gregalWinds.face(w||{name:work?.name,color:accent(),style:'psn'},false,work&&root.gregalWinds.moodOf(work));
 // On es presenta un vent, la figura sencera amb l'eina del seu ofici; `ground` hi posa el terra.
 const figure=(w,ground)=>root.gregalWinds.face(w||{color:accent(),style:'psn'},ground?true:'figure');
 const folder=()=>{const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 24 24');svg.setAttribute('class','vw-folder');svg.setAttribute('aria-hidden','true');const p=document.createElementNS(svg.namespaceURI,'path');p.setAttribute('d','M3 6a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v10H3Z');p.setAttribute('fill','none');p.setAttribute('stroke','currentColor');p.setAttribute('stroke-width','1.6');svg.append(p);return svg;};
 // Icones: el joc de traç de shell-icons.js (window.gregalIcon), igual que a Codi;
 // les que aquell joc no té van aquí amb el mateix traç i graella. Res de glifs de text.
 const ownIcons={clock:'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18 M12 7v5l3 2',thumbUp:'M7 10v10H4V10z M7 10l4-7a2 2 0 0 1 3 2l-1 4h5a2 2 0 0 1 2 2.3l-1.2 7A2 2 0 0 1 16.8 20H7',thumbDown:'M17 14V4h3v10z M17 14l-4 7a2 2 0 0 1-3-2l1-4H6a2 2 0 0 1-2-2.3l1.2-7A2 2 0 0 1 7.2 4H17',tune:'M4 6h10 M18 6h2 M4 12h4 M12 12h8 M4 18h12 M20 18h0 M14 4v4 M8 10v4 M16 16v4',plus:'M12 5v14 M5 12h14',close:'M7 7l10 10 M17 7L7 17',menu:'M4 5h16v14H4z M9 5v14',star:'M12 3.8l2.5 5.1 5.6.8-4 3.9.9 5.6-5-2.6-5 2.6.9-5.6-4-3.9 5.6-.8z',back:'M19 12H5 M11 6l-6 6 6 6',arrow:'M5 12h14 M13 6l6 6-6 6',open:'M8 16 16 8 M9.5 8H16v6.5',bell:'M6 16v-5a6 6 0 0 1 12 0v5l1.5 2h-15z M10 20.5h4',chat:'M4 5h16v11H10l-5 4v-4H4z',work:'M4 8h16v11H4z M9 8V5h6v3 M4 13h16',sun:'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8 M12 2v2 M12 20v2 M4.9 4.9l1.4 1.4 M17.7 17.7l1.4 1.4 M2 12h2 M20 12h2 M4.9 19.1l1.4-1.4 M17.7 6.3l1.4-1.4',sparkles:'M12 3l1.8 4.7L18.5 9.5l-4.7 1.8L12 16l-1.8-4.7L5.5 9.5l4.7-1.8z M19 15l.8 2.2L22 18l-2.2.8L19 21l-.8-2.2L16 18l2.2-.8z',calendar:'M4 6h16v14H4z M4 10h16 M8 3v4 M16 3v4',pin:'M9 3h6 M10 3v6l-3 4h10l-3-4V3 M12 13v8',lock:'M6 11h12v9H6z M8 11V8a4 4 0 0 1 8 0v3',eyeOff:'M3 3l18 18 M10.6 6.1A10 10 0 0 1 12 6c5 0 9 6 9 6a17 17 0 0 1-3.2 3.7 M6.6 7.6C4.4 9.1 3 12 3 12s4 6 9 6a8.7 8.7 0 0 0 4-1 M9.9 9.9a3 3 0 0 0 4.2 4.2',notebook:'M6 3h12v18H6z M9 3v18 M12 8h3',tasks:'M5 12l3 3 6-6 M4 4h16v16H4z'};
 function icon(name){const s=el('span','vw-ic');s.setAttribute('aria-hidden','true');const html=root.gregalIcon?.(name);if(html)s.innerHTML=html;else if(ownIcons[name])s.innerHTML='<svg class="ui-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="'+ownIcons[name]+'"/></svg>';return s;}
 // Botó amb icona i text: el text continua sent el nom accessible.
 const iconBtn=(name,label,action,cls)=>{const b=btn('',action,cls);b.append(icon(name),el('span','',label));return b;};
 // Botó només amb icona: el nom va a aria-label i al títol.
 const iconOnly=(name,label,action,cls)=>{const b=btn('',action,cls);b.append(icon(name));b.setAttribute('aria-label',label);b.title=label;return b;};
 // Fila que obre una conversa o una feina: el text i una fletxa d'icona.
 const linkRow=(label,action)=>{const b=btn('',action,'vw-recent');b.append(el('span','',label),icon('open'));return b;};
 const squadName=id=>{const s=state.squads.find(s=>s.id===id);return s?(ca()?s.ca:s.en):t('La teva colla','Your crew');};
 const crew=()=>state.winds.filter(w=>w.squad===state.squad);
 const MOBILE_NAV=[['home','Inici','Home'],['tasks','Feines','Work'],['notes','Notes','Notes'],['saved','Desat','Saved'],['crew','Colla','Crew']];
 // Group replies are stored as "Name: text" for the model's context; the face and name already say who speaks.
 const withoutSpeaker=(text,name)=>{let out=String(text||'');const prefix=String(name||'').trim().toLowerCase()+':';if(prefix.length<2)return out;while(out.slice(0,prefix.length).toLowerCase()===prefix)out=out.slice(prefix.length).trimStart();return out;};
 const roleName=w=>{const specialist=state.squads.find(s=>s.id===w.squad)?.members?.find(m=>m.key===w.specialty);return specialist?(ca()?specialist.ca:specialist.en):w.role||'';};
 function heading(title,copy){const h=el('div','vw-heading');h.append(el('h1','',title));if(copy)h.append(el('p','',copy));content.append(h);}
 function section(title,action){const h=el('div','vw-section-label');h.append(el('span','',title));if(action)h.append(action);nav.append(h);}
 function route(view,active=null){if(state.preparing&&!state.busy)return;clearTimeout(demoTimer);demoRun++;state.chatEpoch=(state.chatEpoch||0)+1;state.draft=input?.value??state.draft;state.view=view;state.active=active;state.history=[];if(view!=='task'&&root.gregalPreview?.getState().open)$('previewClose')?.click();render();}
 const draftKey='gregal_vents_draft';
 function saveDraft(){
  try{sessionStorage.setItem(draftKey,JSON.stringify({draft:input?.value??state.draft,chat:state.view==='chat'?state.active?.id:null,wind:state.active?.legacyWind?state.active.wind:null}));}catch{}
 }
 function newChat(){route('home');state.draft='';if(input){input.value='';input.focus();}}
 function mount(){
  if(mounted)return;mounted=true;
  window.addEventListener('pagehide',saveDraft);
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
 // L'estrella és un botó germà del fil, no un fill: un botó dins d'un botó no s'hi pot arribar amb el teclat.
 function threadStar(key,fav){const label=fav?t('Treu dels preferits','Remove from favorites'):t('Afegeix als preferits','Add to favorites');const s=iconOnly('star',label,async()=>{const m=state.meta||{};const favs=new Set(m.favorites||[]);fav?favs.delete(key):favs.add(key);await call('/api/winds/meta','POST',{favorites:[...favs],sections:m.sections||[]});await load();},'vw-star'+(fav?' on':''));s.setAttribute('aria-pressed',String(fav));return s;}
 function threadButton(chat,cls='vw-nav-item vw-thread'){const n=btn('',()=>openChat(chat),cls);n.append(el('span','vw-thread-label',chat.name));n.title=chat.name;if(state.view==='chat'&&state.active?.id===chat.id)n.setAttribute('aria-current','page');return n;}
 // Una conversa a soles amb un vent: la seva cara i la primera cosa que li vas demanar.
 function soloButton(th){const w=state.winds.find(w=>w.id===th.wind);const n=btn('',()=>{if(w)talkTo(w,th.name).catch(fail);},'vw-nav-item vw-thread vw-thread-solo');if(w)n.append(face(w));n.append(el('span','vw-thread-label',th.title||w?.name||''));n.title=(w?w.name+' · ':'')+(th.title||'');if(state.view==='chat'&&state.active?.id===th.name)n.setAttribute('aria-current','page');return n;}
 async function refreshSolo(){try{state.solo=(await call('/api/winds/threads')).threads||[];}catch{}}
 function threadRow(chat,fav){const row=el('div','vw-thread-row');row.append(threadButton(chat),threadStar('rot:'+chat.id,fav));return row;}
 function renderThreads(nav){
  const m=state.meta||{};const favs=new Set(m.favorites||[]);
  const free=state.chats.filter(c=>!c.workspace);
  const favRows=free.filter(c=>favs.has('rot:'+c.id));
  const secRows=(m.sections||[]).map(s=>({s,rows:free.filter(c=>(s.items||[]).includes('rot:'+c.id))})).filter(x=>x.rows.length);
  const claimed=new Set([...favRows,...secRows.flatMap(x=>x.rows)].map(c=>'rot:'+c.id));
  const rest=free.filter(c=>!claimed.has('rot:'+c.id));
  for(const chat of favRows)nav.append(threadRow(chat,true));
  for(const {s,rows} of secRows){section(s.name);for(const chat of rows)nav.append(threadButton(chat));}
  // Les converses a soles (el que fa «Automàtic») primer: són les de cada dia.
  for(const th of (state.solo||[]).slice(0,8))nav.append(soloButton(th));
  for(const chat of rest.slice(0,12))nav.append(threadRow(chat,false));
 }
 function renderNav(){
  nav.replaceChildren(iconBtn('plus',t('Nova conversa','New conversation'),newChat,'vw-new'));
  const navRow=([id,a,b])=>{const n=btn('',()=>route(id),'vw-nav-item vw-nav-icon');n.append(navIcon(id),el('span','',t(a,b)));n.setAttribute('aria-current',state.view===id?'page':'false');return n;};
  for(const item of [['home','Inici','Home'],['tasks','Feines','Work'],['crew','La colla','Crew'],['notes','Notes','Notes'],['saved','Desat','Saved']])nav.append(navRow(item));
  // «Automatitzacions» no hi va: ja és al capdamunt de la barra (Vents ·
  // Racó · Automatitzacions), que es veu des de totes les pàgines de Vents.
  // La resta d'espais, plegats (s'obre sol si n'hi ha un d'obert).
  const more=el('details','vw-more'),moreSum=el('summary','vw-nav-item vw-nav-icon');moreSum.append(navIcon('more'),el('span','',t('Més','More')));more.append(moreSum);
  for(const item of [['pati','El pati','Playground'],['library','Biblioteca','Library'],['wiki','Wiki','Wiki']])more.append(navRow(item));
  more.open=['pati','library','wiki'].includes(state.view)||(()=>{try{return localStorage.getItem('gregal_vw_more')==='1';}catch{return false;}})();
  more.addEventListener('toggle',()=>{try{localStorage.setItem('gregal_vw_more',more.open?'1':'0');}catch{}});nav.append(more);
  section(t('Converses','Conversations'),iconOnly('plus',t('Nova conversa','New conversation'),newChat,'vw-small'));
  renderThreads(nav);
  if(!state.chats.some(c=>!c.workspace))nav.append(el('p','vw-nav-empty',t('Aquí comencen les bones idees.','Good ideas start here.')));
  // Xats compartits amb altres persones del servidor (winds-shared.js).
  root.gregalShared?.nav(nav,{open:c=>route('shared',c),active:state.view==='shared'?state.active:null,rerender:()=>{if(mounted){if(state.view==='shared'&&state.active&&!root.gregalShared.chats().some(c=>c.id===state.active.id))route('home');else renderNav();}}});
  section(t('Projectes','Projects'),iconOnly('plus',t('Obre un projecte','Open a project'),chooseFolder,'vw-small'));
  // Dues carpetes amb el mateix nom («proj», «proj») es distingeixen per la carpeta pare.
  const shown=state.projects.slice(0,12),sameName=name=>shown.filter(p=>p.name===name).length>1;
  for(const project of shown){
   const n=btn('',()=>openProject(project),'vw-nav-item vw-project');n.append(folder(),el('span','',project.name));
   if(sameName(project.name)){const parent=String(project.path||'').replace(/[\\/]+$/,'').split(/[\\/]/).slice(-2,-1)[0];if(parent)n.append(el('small','vw-project-parent',parent));}
   n.title=project.path;n.dataset.selected=String((state.active?.path||state.active?.workspace)===project.path);nav.append(n);
   if((state.active?.path||state.active?.workspace)===project.path){for(const chat of state.chats.filter(c=>c.workspace===project.path).slice(0,8))nav.append(threadButton(chat,'vw-nav-item vw-thread vw-nested'));}
  }
 }
 // Qui porta la conversa: la cara, el nom i l'ofici. En un grup, les cares de qui hi és.
 function owner(){
  const box=el('span','vw-owner'),members=state.winds.filter(w=>(state.active?.members||[]).includes(w.id));
  if(state.active?.legacyWind||members.length===1){const w=state.winds.find(w=>w.id===(state.active.wind||members[0]?.id));if(!w)return el('span','',state.active?.name||'');box.append(face(w),el('strong','',w.name));const role=roleName(w);if(role)box.append(el('small','',role));return box;}
  const faces=el('span','vw-owner-faces');for(const w of members.slice(0,5))faces.append(face(w));box.append(faces,el('strong','',state.active?.name||t('Conversa en grup','Group conversation')));box.title=members.map(w=>w.name).join(', ');return box;
 }
 function render(){
  if(!mounted)return;
  const savedScroll=state.view==='chat'&&messages&&messages.dataset.thread===state.active?.id&&!nearChatEnd(messages)?messages.scrollTop:null;
  renderNav();page.replaceChildren();messages=null;input=null;composer=null;
  const top=el('header','vw-top');const menu=iconOnly('menu',t('Obre la navegació','Open navigation'),()=>{if(matchMedia('(max-width:760px)').matches){setDrawer(!document.body.classList.contains('vw-drawer'));return;}const side=$('side');const reopen=side.classList.contains('closed')||side.classList.contains('compact');if(reopen&&root.setSideCompact)root.setSideCompact(false);if(root.setSide)root.setSide(reopen);},'vw-mobile-menu');menu.setAttribute('aria-expanded',String(document.body.classList.contains('vw-drawer')));top.append(menu,el('span','',t('Vents','Winds')),el('span','vw-separator','/'),state.view==='chat'?owner():el('span','',state.active?.name||squadName(state.squad)));
  // «La colla» ja és a la barra lateral (i a la de sota, al mòbil): aquí només hi van les accions del fil obert.
  if(state.view==='chat'&&state.active?.legacyWind)top.append(btn(t('Fils','Threads'),windThreads,'vw-top-action'));
  if(state.view==='chat'&&state.active?.id)top.append(btn(t('Desa a la wiki','Save to wiki'),summarizeToWiki,'vw-top-action'));
  page.append(top);
  content=el('div','vw-content');page.append(content);status=el('div','vw-status');status.setAttribute('role','status');status.textContent=state.notice?.thread===state.active?.id?state.notice?.text||'':'';page.append(status);
  if(state.view==='home')home();else if(state.view==='crew')team();else if(state.view==='library')library();else if(state.view==='project')projectView();else if(state.view==='tasks')tasks();else if(state.view==='chat')chatView();else if(state.view==='task')taskView();else if(state.view==='wiki')wikiView();else if(state.view==='pati')patiView();else if(state.view==='saved')savedView();else if(state.view==='notes')notesView();else if(state.view==='shared')root.gregalShared?.view(content,{chat:state.active,back:()=>route('home')});
  // Al mòbil, cinc destinacions amb icona; El pati, la biblioteca i la wiki són al menú, dins de «Més».
  const bottom=el('nav','vw-mobile-nav');bottom.setAttribute('aria-label',t('Seccions dels vents','Winds sections'));for(const [id,a,b]of MOBILE_NAV){const n=btn('',()=>route(id));n.append(navIcon(id),el('span','',t(a,b)));n.dataset.selected=String(state.view===id);if(state.view===id)n.setAttribute('aria-current','page');bottom.append(n);}page.append(bottom);
  root.gregalPromptNav?.attachVents(page.querySelector('.vw-messages'));
  pageNavs();
  if(messages)messages.scrollTop=savedScroll??messages.scrollHeight;
 }
 function addComposer(workspace=false){
  composer=el('form','vw-composer');input=el('textarea');input.rows=2;input.value=state.draft;input.placeholder=t('Què tens al cap?','What’s on your mind?');input.setAttribute('aria-label',input.placeholder);input.oninput=()=>{state.draft=input.value;};
  input.addEventListener('keydown',e=>{if(e.key==='Enter'&&!e.shiftKey&&!e.isComposing){e.preventDefault();composer.requestSubmit();}});
  const foot=el('div','vw-composer-foot'),pick=el('span','vw-select'),select=el('select');select.setAttribute('aria-label',t('Amb qui parles','Who you are talking to'));
  select.append(new Option(t('Automàtic','Automatic'),'auto'),new Option(t('Tota la colla','The whole crew'),'crew'));for(const w of state.winds)select.append(new Option(w.name+(roleName(w)?' · '+roleName(w):''),w.id));select.value=state.active?.wind||'auto';
  if(state.view==='chat')pick.hidden=true;
  const group=state.view==='chat'&&!state.active?.legacyWind?state.winds.filter(w=>(state.active?.members||[]).includes(w.id)):[];
  let address=null;
  if(group.length>1){address=el('div','vw-address');address.setAttribute('role','radiogroup');address.setAttribute('aria-label',t('A qui parles','Who you are talking to'));
   const paint=()=>{for(const b of address.children)b.setAttribute('aria-checked',String((b.dataset.wind||null)===(state.replyTo||null)));};
   const anyone=btn(t('Qualsevol','Anyone'),()=>{state.replyTo=null;paint();},'vw-address-any');anyone.setAttribute('role','radio');address.append(anyone);
   for(const w of group){const b=btn('',()=>{state.replyTo=state.replyTo===w.id?null:w.id;paint();input?.focus();},'vw-address-wind');b.dataset.wind=w.id;b.setAttribute('role','radio');b.title=t('Parla amb ','Talk to ')+w.name;b.setAttribute('aria-label',b.title);b.append(face(w));address.append(b);}
   paint();}
  select.onchange=()=>{state.recipient=select.value;};state.recipient=select.value;
  const model=el('button','vw-model');model.type='button';paintModel(model);
  // stopPropagation: el clic que l'obre no ha d'arribar al «clic fora» que el tanca.
  model.onclick=e=>{e.stopPropagation();openModelPicker(model).catch(fail);};
  loadModels().then(()=>paintModel(model)).catch(()=>{});
  const send=el('button','vw-send');paintSend(send);
  pick.append(select);foot.append(pick);if(address)foot.append(address);foot.append(model,send);composer.append(input,foot);
  composer.onsubmit=e=>{e.preventDefault();submit().catch(fail);};content.append(composer);
  // La carpeta va fora del quadre d'escriure, a sota i més petita: dins ocupava
  // una línia sencera i semblava un camp més del missatge.
  if(workspace){const line=el('div','vw-folder-hint vw-folder-under');line.title=state.workspace;line.append(folder(),el('span','',state.workspace));content.append(line);}
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
  const full=modelActual(),id=full?full.split('/').slice(1).join('/')||full:'',name=id?id.replace(/[-_]+/g,' ').replace(/\s+/g,' ').trim():t('Model','Model');
  const chev=el('span','vw-model-chev');chev.setAttribute('aria-hidden','true');
  button.replaceChildren(el('span','vw-model-name',name),chev);
  button.title=t('Model dels vents: ','Winds model: ')+(full||'—')+(models.current?'':t(' (el de la feina)',' (same as work)'));
  button.setAttribute('aria-label',button.title);button.setAttribute('aria-haspopup','dialog');button.setAttribute('aria-expanded',String(!!document.querySelector('.vw-model-pop')));
 }
 function closeModelPicker(){document.querySelector('.vw-model-pop')?.remove();document.querySelectorAll('.vw-model').forEach(b=>b.setAttribute('aria-expanded','false'));document.removeEventListener('click',outsideModel,true);document.removeEventListener('keydown',escModel,true);}
 function outsideModel(e){const pop=document.querySelector('.vw-model-pop');if(pop&&!pop.contains(e.target)&&!e.target.closest?.('.vw-model'))closeModelPicker();}
 function escModel(e){if(e.key==='Escape'){e.stopPropagation();closeModelPicker();}}
 async function openModelPicker(anchor){
  if(document.querySelector('.vw-model-pop')){closeModelPicker();return;}
  const pop=el('div','vw-model-pop');pop.setAttribute('role','dialog');pop.setAttribute('aria-label',t('Model dels vents','Winds model'));
  const head=el('p','vw-model-head',t('Model amb què responen els vents','Model the winds answer with'));
  const filter=el('input','vw-model-filter');filter.type='search';filter.placeholder=t('Filtra models…','Filter models…');filter.setAttribute('aria-label',filter.placeholder);
  const list=el('div','vw-model-list');list.setAttribute('role','listbox');list.append(el('p','vw-model-empty',t('Carregant…','Loading…')));
  pop.append(head,filter,list);document.body.append(pop);anchor.setAttribute('aria-expanded','true');
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
  else{shape.setAttribute('d','M12 19V5M5 12l7-7 7 7');shape.setAttribute('fill','none');shape.setAttribute('stroke','currentColor');shape.setAttribute('stroke-width','2');shape.setAttribute('stroke-linecap','round');shape.setAttribute('stroke-linejoin','round');}
  svg.append(shape);button.append(svg);button.onclick=state.busy?()=>{
   if(!chatTurn){root.gregalWindRunner?.stop();return;}
   chatTurn.stopping=true;
   stopChat(chatTurn).catch(fail);
  }:null;
 }
 const nearChatEnd=box=>box.scrollHeight-box.clientHeight-box.scrollTop<64;
 function paintPending(){
  if(!messages||state.view!=='chat')return;const pending=state.pending;
  const old=messages.querySelector('.vw-pending');const follow=nearChatEnd(messages);old?.remove();
  if(!pending||pending.thread!==state.active?.id)return;
  const wind=state.winds.find(w=>w.id===pending.wind),row=el('article','vw-message vw-pending'),copy=el('div','vw-pending-copy');row.append(face(wind,pending));
  copy.append(el('strong','',wind?.name||t('La colla','Your crew')));
  const phases={thinking:t('Preparant la resposta','Preparing a response'),searching:t('Cercant informació','Searching for information'),scheduling:t('Preparant el recordatori','Preparing the reminder'),working:t('Treballant en la resposta','Working on the response')};
  const activity=el('div','vw-thinking');activity.setAttribute('role',pending.text?'presentation':'status');const dots=el('span','vw-thinking-dots');dots.setAttribute('aria-hidden','true');for(let i=0;i<3;i++)dots.append(el('i'));
  activity.append(el('span','',pending.text?t('Escrivint la resposta','Writing the response'):phases[pending.phase]||phases.thinking),dots);copy.append(activity);
  // Streamed text is formatted as it arrives, like the finished message it becomes.
  if(pending.text){const body=el('div','vw-message-text vw-stream-text'),shown=choicesStrip(withoutSpeaker(pending.text,wind?.name)).replace(/```(?:chart|gregal-plot)\n[\s\S]*?```/g,'\n_'+t('Preparant un gràfic…','Preparing a chart…')+'_\n');if(root.gregalMD)body.innerHTML=root.gregalMD.render(shown);else body.textContent=shown;copy.append(body);}
  row.append(copy);messages.append(row);if(follow)messages.scrollTop=messages.scrollHeight;
 }
 function schedulePending(){
  if(pendingTimer)return;
  pendingTimer=setTimeout(()=>{pendingTimer=0;paintPending();},80);
 }
 async function stopChat(turn){
  if(!turn.started||turn.cancelSent||chatTurn!==turn)return;
  turn.cancelSent=true;
  const destination=turn.destination;
  try{await call('/api/winds/cancel','POST',destination.legacyWind?{wind:destination.wind}:{rotllana:destination.id});}
  catch(error){turn.cancelSent=false;throw error;}
 }
 async function streamChat(destination,text,turn,progress={}){
  turn.destination=destination;
  if(turn.stopping)return {cancelat:true};
  // Una proposta («choices») la continua qui l'ha feta, també en una conversa en grup.
  const payload=destination.legacyWind?{wind:destination.wind,session:destination.id,message:text}:{rotllana:destination.id,message:text,...((state.replyOnce||state.replyTo)?{wind:state.replyOnce||state.replyTo}:{})};state.replyOnce=null;
  const response=await root.api(destination.legacyWind?'/api/winds/chat/stream':'/api/winds/rotllana/stream',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)});
  if(!response.ok||!response.body)throw new Error(t('No s’ha pogut iniciar la resposta.','Could not start the response.'));
  const reader=response.body.getReader(),decoder=new TextDecoder();let buffer='',result=null;
  try{while(true){const chunk=await reader.read();if(chunk.done)break;buffer+=decoder.decode(chunk.value,{stream:true}).replace(/\r\n/g,'\n');let end;
   while((end=buffer.indexOf('\n\n'))>=0){const frame=buffer.slice(0,end);buffer=buffer.slice(end+2);const lines=frame.split('\n'),kind=lines.find(l=>l.startsWith('event:'))?.slice(6).trim(),raw=lines.filter(l=>l.startsWith('data:')).map(l=>l.slice(5).trimStart()).join('\n');if(!raw)continue;const data=JSON.parse(raw);
    if(kind==='phase'||(kind==='activity'&&data.phase==='learning')){turn.started=true;if(turn.stopping)stopChat(turn).catch(fail);}
    if(kind==='error'&&data.cancelat){result=data;continue;}
    if(kind==='error')throw new Error(root.gregalFriendlyRunError(data.error)||t('La resposta s’ha interromput.','The response was interrupted.'));
    if(kind==='done'){result=data;continue;}
    // Un vent espera una resposta (aprovació o pregunta): el torn està aturat fins que contestes.
    if(kind==='approve_request'||kind==='question_request'){state.asks=state.asks.filter(x=>x.key!==data.key).concat(data);paintAsks();continue;}
    if(kind==='ask_resolved'){state.asks=state.asks.filter(x=>x.key!==data.key);paintAsks();continue;}
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
 // Inici, a l'estil de Grok i dots: salutació, compositor gran, la colla a un
 // toc i «Avui»: el que hi ha en marxa i el que ve, en targetes.
 function home(){
  content.classList.add('vw-home');
  const hour=new Date().getHours(),greet=hour<6?t('Bona nit','Good night'):hour<13?t('Bon dia','Good morning'):hour<20?t('Bona tarda','Good afternoon'):t('Bon vespre','Good evening');
  const welcome=el('div','vw-welcome'),orb=el('div','vw-orb');orb.append(figure(crew()[0],true));
  welcome.append(orb,el('h1','',greet+'.'),el('p','',t('Pregunta, encarrega o planifica. La colla s’encarrega de la resta.','Ask, delegate or plan. Your crew takes care of the rest.')));content.append(welcome);
  if(state.squads.length>1){const squads=el('div','vw-squads');squads.setAttribute('role','tablist');squads.setAttribute('aria-label',t('Equips','Squads'));for(const s of state.squads){const b=btn(ca()?s.ca:s.en,()=>{state.squad=s.id;newChat();},'vw-squad-pill');b.setAttribute('role','tab');b.setAttribute('aria-selected',String(s.id===state.squad));squads.append(b);}content.append(squads);}
  addComposer(!!state.active?.project);
  const strip=el('div','vw-crew-strip');crew().slice(0,6).forEach(w=>{const n=btn('',()=>talkTo(w),'vw-crew-chip');n.append(face(w),el('span','',w.name));n.title=roleName(w);strip.append(n);});if(crew().length)content.append(strip);
  const prompts=el('div','vw-prompts');for(const [a,b]of state.squad==='data'?[['Analitza aquestes dades','Analyze this data'],['Compara dos models','Compare two models'],['Explora una sèrie temporal','Explore a time series']]:state.squad==='software'?[['Crea una web','Create a website'],['Revisa aquest projecte','Review this project'],['Comprova la seguretat','Check security']]:[['Ajuda’m a triar','Help me choose'],['Recomana’m un llibre','Recommend a book'],['Organitzem la setmana','Let’s organize the week']])prompts.append(btn(t(a,b),()=>{input.value=t(a,b)+' ';input.focus();},'vw-prompt'));content.append(prompts);
  // The example explains the crew before first use, right under the composer;
  // afterwards home stays focused on real threads.
  if(!state.chats.length&&!state.tasks.length)collaborationDemo();
  const feed=el('section','vw-avui');feed.setAttribute('aria-label',t('Avui','Today'));content.append(feed);paintAvui(feed).catch(()=>paintToday(feed).catch(()=>{}));
 }
 // ---- «Avui»: la safata dels vents (winds_avui.go), a l'estil de Duet i del Daily Brief ----
 // Tot el que un vent t'ha dit pel seu compte és una targeta amb una decisió; la safata s'acaba.
 async function paintAvui(feed){
  const target=content,data=await call('/api/winds/avui');if(content!==target||!feed.isConnected)return;
  const date=(()=>{try{return new Date(data.date+'T12:00').toLocaleDateString(ca()?'ca':'en',{weekday:'long',day:'numeric',month:'long'});}catch{return data.date;}})();
  const head=el('header','vw-avui-head'),title=el('div','vw-avui-title');title.append(el('h2','',t('Avui','Today')),el('span','vw-avui-date',date));
  head.append(title,iconOnly('tune',t('Ajusta Avui','Tune Today'),()=>avuiSettings(data.settings,()=>paintAvui(feed)),'vw-small'));
  const body=[head];
  const wxWind=state.winds.find(w=>w.specialty==='weather'||/\b(el temps|meteo|weather)\b/i.test(w.role||''));
  if(wxWind){const wx=el('div','vw-wx-host');body.push(wx);weatherCard(wx,wxWind,true).catch(()=>wx.remove());}
  if(data.held>0)body.push(el('p','vw-avui-held',t(data.held===1?'1 cosa ha esperat a la safata en hores tranquil·les.':data.held+' coses han esperat a la safata en hores tranquil·les.',data.held+' waited here during quiet hours.')));
  const groups=[['now',t('Ara toca','Top of mind')],['fyi',t('Per si de cas','Good to know')],['ahead',t('Més endavant','Looking ahead')]];
  let shown=0,step=0;
  for(const [key,label] of groups){const cards=data[key]||[];if(!cards.length)continue;const sec=el('section','vw-avui-group');sec.dataset.section=key;sec.append(el('h3','',label));for(const c of cards){const n=avuiCard(c,feed);n.style.setProperty('--i',String(step++));sec.append(n);}body.push(sec);shown+=cards.length;}
  const foot=el('footer','vw-avui-end');
  if(shown){const done=(data.done||[]).length;foot.append(el('p','vw-avui-fin',t('Això és tot per avui.','That’s all for today.')+(done?' '+t(done===1?'Una cosa feta.':done+' coses fetes.',done===1?'One thing done.':done+' things done.'):'')));}
  else{foot.append(el('p','vw-avui-fin',t('Res pendent per avui. Quan un vent tingui alguna cosa per tu, sortirà aquí.','Nothing waiting today. When a wind has something for you, it shows up here.')));
   const conn=await call('/api/connections').catch(()=>null);if(!conn?.briefing?.enabled){const b=btn('',()=>root.setView?.('automatitzacions'),'vw-button vw-avui-cta');b.append(icon('sun'),el('span','',t('Activa el resum del matí','Turn on the morning brief')));foot.append(b);}}
  foot.append(btn(t('Ajusta el de demà','Tune tomorrow'),()=>avuiSettings(data.settings,()=>paintAvui(feed),true),'vw-avui-tune'));
  body.push(foot);
  // Per reprendre: les converses recents, discretes al final.
  const resume=[...(state.solo||[]).slice(0,3).map(th=>({w:state.winds.find(w=>w.id===th.wind),label:th.title,go:()=>{const w=state.winds.find(w=>w.id===th.wind);if(w)talkTo(w,th.name).catch(fail);}})),...state.chats.filter(c=>!c.workspace).slice(0,2).map(c=>({w:state.winds.find(w=>(c.members||[]).includes(w.id)),label:c.name,go:()=>openChat(c)}))].slice(0,4);
  if(resume.length){const row=el('div','vw-avui-resume');row.append(el('span','',t('Per reprendre','Pick up')));for(const r of resume){const b=btn('',r.go,'vw-crew-chip');if(r.w)b.append(face(r.w));b.append(el('span','',r.label||''));row.append(b);}body.push(row);}
  if(content!==target||!feed.isConnected)return;feed.replaceChildren(...body);
 }
 function avuiCard(c,feed){
  const w=state.winds.find(w=>w.id===c.wind),card=el('article','vw-avui-card');card.dataset.id=c.id;card.dataset.source=c.source;
  const copy=el('div','vw-avui-copy'),titleBtn=btn('',()=>{card.classList.toggle('is-open');titleBtn.setAttribute('aria-expanded',String(card.classList.contains('is-open')));},'vw-avui-card-title');titleBtn.append(el('strong','',c.title));titleBtn.setAttribute('aria-expanded','false');copy.append(titleBtn);
  if(c.why)copy.append(el('p','vw-avui-why',c.why));
  const meta=el('p','vw-avui-meta');meta.append(el('span','',c.wind_name||w?.name||''));if(c.days)meta.append(el('span','vw-avui-age',c.days===1?t('des d’ahir','since yesterday'):t('fa '+c.days+' dies','for '+c.days+' days')));copy.append(meta);
  if(c.body){const more=el('div','vw-avui-body vw-message-text');more.innerHTML=root.gregalMD?root.gregalMD.render(c.body):'';if(!root.gregalMD)more.textContent=c.body;copy.append(more);}
  const acts=el('div','vw-avui-acts');
  for(const a of c.actions||[])acts.append(btn(a.label,async()=>{if(!w)return;const thread=await call('/api/winds/sessions','POST',{wind:w.id});showWindThread(w,thread.name,[]);if(input){input.value=a.prompt;composer.requestSubmit();}},'vw-avui-chip'));
  const act=(action,after)=>async()=>{try{await call('/api/winds/avui','POST',{id:c.id,action});}catch(e){fail(e);return;}after();};
  const leave=()=>{card.classList.add('is-leaving');setTimeout(()=>paintAvui(feed).catch(()=>{}),matchMedia('(prefers-reduced-motion: reduce)').matches?0:260);};
  const side=el('div','vw-avui-decide');
  const useful=iconOnly('thumbUp',t('Útil','Useful'),act('useful',()=>{useful.setAttribute('aria-pressed',String(useful.getAttribute('aria-pressed')!=='true'));}),'vw-small');useful.setAttribute('aria-pressed',String(c.feedback==='useful'));
  side.append(useful,iconOnly('thumbDown',t('No m’interessa','Not interesting'),act('notuseful',leave),'vw-small'),iconOnly('clock',t('Més tard','Later'),act('later',leave),'vw-small'),iconOnly('close',t('Ara no','Not now'),act('dismiss',leave),'vw-small'));
  const done=btn('',act('done',leave),'vw-avui-done');done.append(icon('check'),el('span','',t('Fet','Done')));side.append(done);
  acts.append(side);copy.append(acts);
  card.append(w?face(w):el('span','vw-shared-person','·'),copy);
  return card;
 }
 // Ajusta Avui: hores tranquil·les, mode tranquil i el que vols que tingui en compte el resum de demà.
 function avuiSettings(st,after,focusTune){
  const d=dialog(t('Ajusta Avui','Tune Today'));
  d.append(el('p','vw-muted',t('En hores tranquil·les els vents no fan sonar el mòbil: el que trobin t’espera aquí. Els recordatoris i les feines acabades sí que avisen.','During quiet hours winds don’t ring your phone: what they find waits here. Reminders and finished work still notify you.')));
  const row=el('div','vw-avui-hours');const from=field(row,t('Hores tranquil·les des de','Quiet from'),st?.quiet_from||'22:00'),to=field(row,t('fins a','until'),st?.quiet_to||'08:00');from.type='time';to.type='time';d.append(row);
  const lab=el('label','vw-check'),calm=el('input');calm.type='checkbox';calm.checked=st?.level==='calm';lab.append(calm,el('span','',t('Mode tranquil: durant el dia, només el resum del matí, els recordatoris i les feines fan sonar el mòbil.','Calm mode: during the day only the morning brief, reminders and finished work ring the phone.')));d.append(lab);
  const tune=field(d,t('Què vols que tingui en compte el resum de demà?','What should tomorrow’s brief take into account?'),st?.tune||'',true);tune.placeholder=t('Per exemple: més esport, menys notícies, recorda’m el gimnàs els dimarts…','For example: more sport, less news, remind me about the gym on Tuesdays…');
  d.append(el('p','vw-error'));
  const foot=el('div','vw-dialog-foot');foot.append(btn(t('Cancel·la','Cancel'),()=>d.close(),'vw-button'),btn(t('Desa','Save'),async()=>{await call('/api/winds/avui/settings','POST',{quiet_from:from.value,quiet_to:to.value,level:calm.checked?'calm':'normal',tune:tune.value});d.close();after?.();},'vw-primary'));d.append(foot);
  if(focusTune)setTimeout(()=>tune.focus(),0);
 }
 // «Avui» (abans de la safata): converses per reprendre, la feina, els recordatoris que vénen i el resum del matí.
 async function paintToday(feed){
  const cards=[];
  const card=(ic0,kicker,title,sub,action,cls='')=>{const c=btn('',action,'vw-card '+cls);const ic=el('span','vw-card-ic');ic.append(typeof ic0==='string'?icon(ic0):ic0);const copy=el('span','vw-card-copy');copy.append(el('small','',kicker),el('strong','',title));if(sub)copy.append(el('span','',sub));c.append(ic,copy);return c;};
  for(const c of state.chats.filter(c=>!c.workspace).slice(0,3)){const w=state.winds.find(w=>(c.members||[]).includes(w.id));cards.push(card(w?face(w):'chat',t('Repren','Pick up'),c.name,(c.members||[]).length>1?t('Amb la colla','With the crew'):w?.name||'',()=>openChat(c)));}
  for(const task of state.tasks.slice(0,2))cards.push(card('work',t('Feina','Work'),task.task||t('Feina','Work'),task.status||'',()=>{state.events=task.events||[];state.snapshot=null;route('task',{...task,name:task.task});}));
  const target=content;
  const [prog,conn]=await Promise.all([call('/api/winds/programats').catch(()=>({})),call('/api/connections').catch(()=>null)]);
  if(content!==target||!feed.isConnected)return;
  const when=at=>{try{return new Date(at).toLocaleString(ca()?'ca':'en',{weekday:'short',hour:'2-digit',minute:'2-digit'});}catch{return String(at);}};
  for(const r of (prog.reminders||[]).slice(0,2))cards.push(card('bell',t('Recordatori · ','Reminder · ')+when(r.at),r.text,r.wind_name||'',()=>root.setView?.('automatitzacions'),'vw-card-soft'));
  const b=conn?.briefing;if(b)cards.push(card('sun',t('Resum del matí','Morning briefing'),b.enabled?t('Cada dia a les ','Every day at ')+b.time:t('Encara no està activat','Not turned on yet'),b.enabled?t('Calendari i correu, en dues línies','Calendar and email, in two lines'):t('Connecta el calendari i el correu','Connect your calendar and email'),()=>root.setView?.('automatitzacions'),'vw-card-soft'));
  if(!cards.length)return;
  const head=el('div','vw-today-head');const more=btn('',()=>root.setView?.('automatitzacions'),'vw-today-more');more.append(el('span','',t('Automatitzacions','Automations')),icon('arrow'));head.append(el('h2','',t('Avui','Today')),more);
  const grid=el('div','vw-cards');grid.append(...cards);feed.replaceChildren(head,grid);
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
  const show=()=>{if(run!==demoRun||!card.isConnected)return;const turn=turns[index],row=el('article','vw-demo-turn'+(turn.summary?' vw-demo-summary':''));if(turn.person)row.append(face(turn.person));else{const mark=el('span','vw-demo-mark');mark.append(icon('sparkles'));row.append(mark);}const copy=el('div');copy.append(el('strong','',turn.person?.name||turn.role),el('span','vw-demo-role',turn.person?turn.role:t('Conclusió compartida','Shared takeaway')),el('p','',turn.text));row.append(copy);thread.append(row);index++;if(index<turns.length)demoTimer=setTimeout(show,1900);else card.classList.remove('is-playing');};show();
 }
 async function activate(id){await call('/api/winds/squads','POST',{id,lang:ca()?'ca':'en'});state.winds=(await call('/api/winds')).winds;renderNav();}
 async function bestWind(text){
  const pool=crew().length?crew():state.winds;if(!pool.length)return null;
  let cards=[];try{cards=(await call('/api/winds/specialists?task='+encodeURIComponent(text.slice(0,600)))).specialists||[];}catch{}
  const inPool=c=>pool.find(w=>w.id===c.id);
  return inPool(cards.find(c=>c.score>0&&inPool(c))||{})||inPool(cards.find(c=>c.kit==='organisation'&&inPool(c))||{})||pool.find(w=>w.specialty==='organisation')||pool[0];
 }
 async function talkTo(w,session=''){
  const ticket=state.chatEpoch=(state.chatEpoch||0)+1;
  root.gregalWinds.seen?.(w.id);
  const query=new URLSearchParams({wind:w.id});
  if(session)query.set('session',session);
  const data=await call('/api/winds/chat?'+query);
  if(ticket!==state.chatEpoch)return;
  showWindThread(w,data.session||session||'company-'+w.id,data.history||[]);
 }
 function showWindThread(w,session,history){
  state.active={id:session,wind:w.id,name:w.name,members:[w.id],legacyWind:true};
  state.history=history;
  state.view='chat';
  state.draft='';
  render();
 }
 async function windThreads(){
  if(state.busy)return;
  const wind=state.winds.find(w=>w.id===state.active?.wind);
  if(!wind)return;
  const data=await call('/api/winds/sessions?wind='+encodeURIComponent(wind.id));
  const d=dialog(t('Fils de ','Threads with ')+wind.name);
  const rows=el('div','vw-session-list');
  for(const thread of data.sessions||[]){
   const label=(thread.title||wind.name)+(thread.when?' · '+thread.when:'');
   const row=btn(label,async()=>{await talkTo(wind,thread.name);d.close();},'vw-button vw-session-row');
   if(thread.name===state.active?.id)row.setAttribute('aria-current','true');
   rows.append(row);
  }
  d.append(rows,btn(t('Nou fil','New thread'),async()=>{
   const thread=await call('/api/winds/sessions','POST',{wind:wind.id});
   showWindThread(wind,thread.name,[]);
   d.close();
  },'vw-primary'));
 }
 function team(){heading(t('Tothom té alguna cosa a aportar.','Everyone brings something.'),t('Especialistes amb memòria pròpia i coneixement que pots conservar.','Specialists with their own memory and knowledge you can keep.'));
  {const tpl=state.squads.find(x=>x.id===state.squad),mine=new Set(crew().map(w=>w.specialty));const missing=crew().length?(tpl?.members||[]).filter(m=>!mine.has(m.key)):[];
   if(missing.length){const card=el('div','vw-newcomers'),faces=el('span','vw-owner-faces');for(const m of missing)faces.append(root.gregalWinds.face({name:m.name,color:m.color,style:m.style,kit:m.key},false));const copy=el('span','');copy.append(el('strong','',t('Nous a la colla: ','New in the crew: ')),document.createTextNode(missing.map(m=>m.name+' ('+(ca()?m.ca:m.en).toLowerCase()+')').join(t(' i ',' and '))));
    card.append(faces,copy,btn(t('Afegeix-los','Add them'),async()=>{await activate(state.squad);render();},'vw-primary'));content.append(card);}}
  const tabs=el('div','vw-tabs');tabs.setAttribute('aria-label',t('Equips','Squads'));for(const s of state.squads){const b=btn(ca()?s.ca:s.en,()=>{state.squad=s.id;render();});b.dataset.selected=String(s.id===state.squad);b.setAttribute('aria-pressed',String(s.id===state.squad));tabs.append(b);}content.append(tabs);
  const roster=crew();if(!roster.length){const empty=el('div','vw-empty');empty.append(face(),el('h2','',t('La teva colla, a punt.','Your crew, ready to help.')),el('p','',t('Activa l’equip amb un sol clic. Les especialitats ja venen preparades.','Activate this squad in one click. Specialties are already defined.')),btn(t('Prepara aquest equip','Prepare this squad'),async()=>{await activate(state.squad);render();},'vw-primary'));content.append(empty);}
  for(const w of roster){const row=el('article','vw-member');const copy=el('div');copy.append(el('h2','',w.name),el('p','',roleName(w)));row.append(figure(w),copy,btn(t('Parla','Talk'),()=>talkTo(w)),btn(t('Perfil','Profile'),()=>profile(w)));content.append(row);}
  const personal=state.winds.filter(w=>!w.squad);if(personal.length){content.append(el('h2','vw-subtitle',t('Els teus vents','Your winds')));for(const w of personal){const row=el('article','vw-member'),copy=el('div');copy.append(el('h2','',w.name));if(roleName(w))copy.append(el('p','',roleName(w)));row.append(figure(w),copy,btn(t('Parla','Talk'),()=>talkTo(w)),btn(t('Perfil','Profile'),()=>profile(w)));content.append(row);}}
  content.append(iconBtn('plus',t('Crea un vent propi','Create your own wind'),createWind,'vw-button vw-add'));
 }
 function dialog(title){const d=el('dialog','vw-dialog');d.setAttribute('aria-label',title);const head=el('header');head.append(el('h2','',title),iconOnly('close',t('Tanca','Close'),()=>d.close(),'vw-close'));d.append(head);document.body.append(d);d.addEventListener('close',()=>d.remove());d.showModal();return d;}
 function field(parent,label,value='',multiline=false){const wrap=el('label','vw-field',label),n=el(multiline?'textarea':'input');n.value=value;if(multiline)n.rows=7;wrap.append(n);parent.append(wrap);return n;}
 function createWind(){
  const d=dialog(t('Una nova companyia','A new companion'));
  d.append(el('p','vw-muted',t('Tria una idea o crea un vent a la teva mida. Pots canviar-ho tot abans de crear-lo.','Choose an idea or create your own wind. You can change everything before creating it.')));
  const suggestions=[
   ['Brúixola','Recerca i comparatives','Research and comparisons','Busca fonts fiables, contrasta la informació i explica les incerteses.','Find reliable sources, compare information and explain uncertainty.','#8fcabb','psn'],
   ['Espurna','Idees i escriptura','Ideas and writing','Ajuda a imaginar, escriure i polir textos amb una veu propera.','Help brainstorm, write and polish text with a warm voice.','#efad85','eso'],
   ['Farcell','Organització i plans','Organization and planning','Converteix idees en plans clars, pràctics i flexibles.','Turn ideas into clear, practical and flexible plans.','#d6bb87','pso'],
   ['Traç','Disseny i webs','Design and websites','Dissenya interfícies clares, boniques i accessibles.','Design clear, attractive and accessible interfaces.','#b3a3df','bsn'],
   ['Bit','Programació','Programming','Escriu codi entenedor i comprova que funcioni.','Write understandable code and check that it works.','#8eb4d6','pso'],
   ['Lupa','Revisió i proves','Review and testing','Revisa amb cura, troba problemes i proposa millores concretes.','Review carefully, find problems and suggest concrete improvements.','#d7adba','bss'],
   ['Gràfica','Dades i gràfics','Data and charts','Analitza dades amb rigor i explica els resultats de manera entenedora.','Analyze data carefully and explain results clearly.','#91c7ac','psn'],
   ['Teca','Cuina i àpats','Cooking and meals','Proposa receptes i menús adaptats als gustos i al temps disponible.','Suggest recipes and meal plans suited to tastes and available time.','#eebc78','pso']
  ];
  const list=el('div','vw-wind-suggestions');list.setAttribute('aria-label',t('Vents suggerits','Suggested winds'));d.append(list);
  const preview=el('div','vw-wind-preview');d.append(preview);
  const name=field(d,t('Nom','Name')),role=field(d,t('Com et pot ajudar?','How can they help?')),persona=field(d,t('Manera de ser i instruccions','Personality and instructions'),'',true);persona.rows=3;
  const appearance=el('div','vw-wind-appearance');d.append(appearance);
  const tint=field(appearance,t('Color','Color'),'#c1cda9');tint.type='color';
  function choice(label,options,value){const wrap=el('label','vw-field',label),select=el('select');for(const [key,caLabel,enLabel] of options){const option=el('option','',t(caLabel,enLabel));option.value=key;select.append(option);}select.value=value;wrap.append(select);appearance.append(wrap);return select;}
  const shape=choice(t('Forma','Shape'),[['o','Bola','Ball'],['n','Núvol','Cloud'],['s','Rínxol','Curl']],'n');
  const eyes=choice(t('Ulls','Eyes'),[['p','Punts','Dots'],['b','Ben oberts','Wide'],['e','Estrellats','Starry'],['m','Mig tancats','Sleepy']],'p');
  const mouth=choice(t('Boca','Mouth'),[['s','Somriure','Smile'],['o','Oberta','Open'],['n','Tranquil·la','Relaxed'],['t','Llengua','Tongue']],'s');
  const update=()=>preview.replaceChildren(root.gregalWinds.face({name:name.value,color:tint.value,style:eyes.value+mouth.value+shape.value},true));
  for(const control of [name,tint,shape,eyes,mouth]){control.addEventListener('input',update);control.addEventListener('change',update);}
  // Amb un nom propi per als ajudants (fullets, pirates…), els noms
  // suggerits vénen del servidor i hi encaixen; el rol i la persona es queden.
  const labels=[];
  if(window.gregalCollective&&window.gregalCollective.get().plural)window.gregalCollective.names(suggestions.length,ca()?'ca':'en').then(names=>{names.slice(0,suggestions.length).forEach((n,i)=>{suggestions[i][0]=n;if(labels[i])labels[i].textContent=n;});});
  for(const entry of suggestions){
   const [,caRole,enRole,caPersona,enPersona,color,style]=entry;
   const b=btn('',()=>{const suggested=entry[0];let candidate=suggested,index=2;const used=new Set(state.winds.map(w=>w.name.trim().toLocaleLowerCase()));while(used.has(candidate.toLocaleLowerCase()))candidate=suggested+' '+index++;name.value=candidate;role.value=t(caRole,enRole);persona.value=t(caPersona,enPersona);tint.value=color;eyes.value=style[0];mouth.value=style[1];shape.value=style[2];update();},'vw-wind-suggestion');
   const label=el('strong','',entry[0]);labels.push(label);b.append(label,el('span','',t(caRole,enRole)));list.append(b);
  }
  update();
  const foot=el('div','vw-dialog-foot');foot.append(btn(t('Cancel·la','Cancel'),()=>d.close(),'vw-button'),btn(t('Crea el vent','Create wind'),async()=>{if(!name.value.trim()){name.focus();return;}await call('/api/winds','POST',{name:name.value.trim(),role:role.value.trim(),persona:persona.value.trim(),color:tint.value,style:eyes.value+mouth.value+shape.value});d.close();await load();route('crew');},'vw-primary'));d.append(foot);
 }
 async function profile(w){
  const data=await call('/api/winds/profile?wind='+encodeURIComponent(w.id));const d=dialog(w.name);const who=el('div','vw-profile-head'),whoCopy=el('p','vw-muted',roleName(w)||t('Vent propi','Your own wind'));who.append(figure(w),whoCopy,btn(t('Fitxes consultables','Reference cards'),()=>windCards(w)));d.append(who);if(data.trade?.kit==='weather'){const wx=el('div','vw-wx-host');d.append(wx);weatherCard(wx,w).catch(()=>{});}if(data.trade)d.append(tradePanel(w,data.trade));const tabs=el('div','vw-tabs'),body=el('div');d.append(tabs,body);
  const draw=kind=>{body.replaceChildren();for(const b of tabs.children){b.dataset.selected=String(b.dataset.kind===kind);b.setAttribute('aria-pressed',String(b.dataset.kind===kind));}if(kind==='calaix'){calaixPanel(body,w).catch(fail);return;}if(kind==='scheduled'||kind==='saved'){const box=el('div','act-list');if(kind==='scheduled'){const lab=el('label','vw-check'),cb=el('input');cb.type='checkbox';cb.checked=w.followups!=='off';cb.onchange=async()=>{await call('/api/winds','POST',{id:w.id,name:w.name,color:w.color,role:w.role,style:w.style,followups:cb.checked?'on':'off'});w.followups=cb.checked?'on':'off';notice.textContent=t('Desat','Saved');};lab.append(cb,el('span','',t('Seguiments pel seu compte: si li expliques una cosa teva amb data, s’ofereix a tornar-hi després.','Follow-ups on its own: if you mention something of yours with a date, it offers to check back afterwards.')));body.append(lab);}body.append(box);const A=root.gregalVentsActivity;if(A)(kind==='saved'?A.renderSaved(box,{wind:w.id,onCard:()=>windCards(w)}):A.renderAutomations(box,{wind:w.id}));return;}const memory=kind==='memory';body.append(el('p','vw-muted',memory?t('Preferències, decisions i coses sobre tu que aquest vent recorda. Valen per a tot el que fa: el xat, les rutines, el que vigila i els recordatoris amb feina. Corregeix-les aquí un cop i ja està.','Preferences, decisions and things about you this wind remembers. They apply to everything it does: chat, routines, what it watches and reminders with a task. Correct them here once and that’s it.'):t('Fets, documents i procediments que pot utilitzar per ajudar-te.','Facts, documents and procedures it can use to help you.')));const area=field(body,memory?t('Memòria','Memory'):t('Coneixement','Knowledge'),memory?data.memory:data.knowledge,true);area.maxLength=memory?8000:24576;body.append(btn(t('Desa els canvis','Save changes'),async()=>{await call(memory?'/api/winds/memoria':'/api/winds/memory',memory?'PUT':'POST',memory?{wind:w.id,memoria:area.value}:{wind:w.id,text:area.value});if(memory)data.memory=area.value;else data.knowledge=area.value;notice.textContent=t('Desat','Saved');},'vw-primary'));
   if(!memory){for(const skill of data.skills||[]){const detail=el('details','vw-document');detail.append(el('summary','',skill.Titol||skill.Nom),el('p','',skill.Cos));body.append(detail);}body.append(iconBtn('plus',t('Afegeix un document o procediment','Add a document or procedure'),()=>knowledgeDocument(w),'vw-button vw-add'));}
  };for(const [id,a,b]of [['calaix','Calaix','Drawer'],['memory','Memòria','Memory'],['knowledge','Coneixement','Knowledge'],['scheduled','Programat','Scheduled'],['saved','Desat','Saved']]){const n=btn(t(a,b),()=>draw(id));n.dataset.kind=id;tabs.append(n);}tabs.setAttribute('aria-label',t('Seccions del perfil','Profile sections'));const notice=el('p','vw-status');notice.setAttribute('role','status');d.append(notice);draw(data.trade?'calaix':'memory');
  d.append(btn(t('Arxiva el vent i conserva la seva informació','Archive wind and keep its information'),async()=>{await call('/api/winds','DELETE',{id:w.id});d.close();await load();route('library');},'vw-archive'));
 }
 async function windCards(w){
  const d=dialog(t('Fitxes de '+w.name,'Cards for '+w.name));d.append(el('p','vw-muted',t('Informació que vols consultar i actualitzar quan calgui.','Information to check and refresh when needed.')));const list=el('div');d.append(list);
  const draw=async()=>{const data=await call('/api/winds/cards?wind='+encodeURIComponent(w.id));list.replaceChildren();if(!data.cards.length)list.append(el('p','vw-muted',t('Encara no hi ha cap fitxa.','No cards yet.')));for(const card of data.cards){const box=el('article','vw-document');box.append(el('h3','',card.title),el('p','vw-muted',card.updated?t('Actualitzada: ','Updated: ')+new Date(card.updated).toLocaleString():t('Encara no actualitzada','Not updated yet')),el('p','vw-muted',card.prompt));const text=el('pre','vw-library-text',card.content||t('Prem Actualitza per cercar informació.','Press Refresh to research this topic.'));box.append(text);const update=btn(t('Actualitza','Refresh'),async()=>{update.disabled=true;update.textContent=t('Actualitzant…','Refreshing…');try{await call('/api/winds/cards','POST',{wind:w.id,action:'refresh',id:card.id});await draw();}catch(e){fail(e);}finally{update.disabled=false;update.textContent=t('Actualitza','Refresh');}},'vw-primary');box.append(update);list.append(box);}};
  const title=field(d,t('Nom de la fitxa','Card title')),prompt=field(d,t('Què ha de cercar i actualitzar?','What should be researched and updated?'),'',true);d.append(btn(t('Crea la fitxa','Create card'),async()=>{await call('/api/winds/cards','POST',{wind:w.id,action:'create',title:title.value,prompt:prompt.value});title.value='';prompt.value='';await draw();},'vw-primary'));await draw();
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
 function projectView(){heading(state.active.name,t('Un lloc per conversar, treballar i trobar els resultats.','A place to talk, work and find the results.'));const path=el('p','vw-project-path');path.append(folder(),el('span','',state.active.path));content.append(path);content.append(iconBtn('plus',t('Nova conversa al projecte','New project conversation'),()=>{state.active={...state.active,project:true};state.view='home';render();},'vw-button vw-add'));const chats=state.chats.filter(c=>c.workspace===state.active.path);for(const c of chats)content.append(linkRow(c.name,()=>openChat(c)));taskList(state.tasks.filter(task=>task.workspace===state.active.path));}
 // El pati: la colla jugant; la feina real els porta al taller (winds-pati.js).
 function patiView(){content.classList.add('vw-pati-view');heading(t('El pati','The playground'),t('La colla a l’hora del pati. Fes clic en un vent per saber què fa i què pensa; arrossega’n uns quants al cercle de guix per fer una rotllana o encarregar-los una feina.','The crew at playtime. Click a wind to see what it is doing and thinking; drag a few into the chalk circle to start a chat or give them a task.'));if(!state.winds.length){content.append(el('p','vw-empty',t('Encara no hi ha cap vent al pati.','No winds in the playground yet.')));return;}root.gregalPati?.mount(content,{winds:state.winds,tasks:state.tasks,talkTo,openChat,openThread:id=>openChat(state.chats.find(c=>c.id===id)||{id,name:'',members:[]}),call,notify:text=>{if(status)status.textContent=text;},refreshTasks:async()=>(await call('/api/winds/tasks?summary=1')).tasks||[]});}
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
  const data=await call('/api/winds/wiki/summary','POST',chat.legacyWind?{wind:chat.wind,session:chat.id}:{rotllana:chat.id});
  status.textContent=(data.updated?t('Pàgina actualitzada: ','Page updated: '):t('Pàgina creada: ','Page created: '))+(data.page?.titol||'');
  await openWikiPage(data.page);
}
async function showRoutines(container){container.replaceChildren(el('h2','vw-subtitle',t('Rutines','Routines')),el('p','vw-muted',t('Cerques i resums recurrents. Mantén Gregal obert perquè s’executin.','Recurring searches and summaries. Keep Gregal open for them to run.')),iconBtn('plus',t('Una nova rutina','New routine'),newRoutine,'vw-button vw-add'));const data=await call('/api/jobs');if(!container.isConnected)return;for(const job of (data.jobs||[]).filter(j=>j.wind)){const row=el('article','vw-routine');row.append(el('strong','',job.name),el('span','vw-muted',(job.enabled?t('Activa','Active'):t('Pausada','Paused'))+' · '+(job.next_due||job.time||'')),btn(job.enabled?t('Pausa','Pause'):t('Activa','Enable'),async()=>{await call('/api/jobs/save','POST',{...job,enabled:!job.enabled});await showRoutines(container);}));row.append(btn(t('Resultats','Results'),async()=>{const data=await call('/api/jobs/runs?job='+encodeURIComponent(job.id));const d=dialog(job.name);for(const run of data.runs||[]){const body=el('div','vw-result-text');const shown=run.output||(run.error?root.gregalFriendlyRunError(run.error):'');if(root.gregalMD)body.innerHTML=root.gregalMD.render(shown||'');else body.textContent=shown||'';d.append(el('p','vw-muted',new Date(run.finished_at).toLocaleString()),body);}if(!(data.runs||[]).length)d.append(el('p','vw-muted',t('Encara no s’ha executat.','No runs yet.')));}));container.append(row);}}
 async function newRoutine(){if(!crew().length)await activate(state.squad);const d=dialog(t('Una petita ajuda, cada dia.','A little help, every day.'));const name=field(d,t('Nom de la rutina','Routine name')),prompt=field(d,t('Què vols que cerqui o resumeixi?','What should it research or summarize?'),state.draft,true),time=field(d,t('Cada dia a les','Every day at'),'09:00');time.type='time';const select=el('select');select.setAttribute('aria-label',t('Vent responsable','Responsible wind'));for(const w of state.winds)select.append(new Option(w.name+' · '+roleName(w),w.id));select.value=crew()[0]?.id||state.winds[0]?.id;d.append(select,el('p','vw-muted',t('Fa servir la memòria i el coneixement actuals del vent. És una rutina de lectura i cerca; Gregal ha d’estar obert.','Uses the wind’s current memory and knowledge. This is a reading and research routine; Gregal must stay open.')),btn(t('Desa la rutina','Save routine'),async()=>{if(!name.value.trim()||!prompt.value.trim()){(!name.value.trim()?name:prompt).focus();return;}await call('/api/jobs/save','POST',{name:name.value,prompt:prompt.value,kind:'daily',time:time.value,enabled:true,notify:false,wind:select.value});d.close();route('tasks');},'vw-primary'));}
 // Cada feina diu qui hi ha treballat, quan, en quin projecte i què ha deixat.
 function taskList(items){for(const task of items){const row=btn('',()=>{state.events=task.events||[];state.snapshot=null;route('task',{...task,name:task.task});},'vw-task-row');
   const who=el('span','vw-task-faces');for(const name of Object.values(task.agents||{}))who.append(face(state.winds.find(w=>w.name===name)||{name,color:'#BFC7DF',style:'psn'}));
   const files=[...new Set((task.events||[]).filter(e=>e.type==='tool_result'&&e.path).map(e=>e.path.split(/[\\/]/).pop()))];
   const head=el('div','vw-task-head');head.append(who,el('strong','',task.task),el('span','vw-task-state is-'+(['done','completed'].includes(task.status)?'done':['failed','cancelled'].includes(task.status)?'stopped':'running'),statusName(task.status)));
   const meta=[(task.workspace||'').replace(/[\\/]+$/,'').split(/[\\/]/).pop(),task.created?new Date(task.created).toLocaleString(document.documentElement.lang||undefined,{day:'numeric',month:'short',hour:'2-digit',minute:'2-digit'}):'',files.slice(0,3).join(', ')].filter(Boolean).join(' · ');
   row.append(head,el('small','',meta));content.append(row);}
  if(!items.length)content.append(el('p','vw-empty',t('Encara no hi ha cap feina. Demana-la des de l’inici («Prepara’m una web per al club», «Analitza aquest CSV») i la colla s’hi posarà; aquí en veuràs el progrés i el resultat.','No work yet. Ask from home (“Build a website for the club”, “Analyze this CSV”) and the crew gets on it; you will see the progress and the result here.')));}
 // Icones de la navegació dels vents: traç fi com les del rail, no caràcters Unicode.
 // Notes: el quadern de l'usuari (winds-notes.js); «Pregunta a un vent» obre el xat amb el text a punt.
 function notesView(){if(!root.gregalNotes){content.append(el('p','vw-muted',t('Carregant…','Loading…')));return;}root.gregalNotes.view(content,{ask:text=>{route('home');if(input){input.value=text;input.focus();input.setSelectionRange(text.length,text.length);}}});}
 function savedView(){heading(t('Desat','Saved'),t('Tot el que els vents han fet per a tu: documents, webs, gràfics, imatges i fitxes, amb qui ho va fer i quan.','Everything the winds made for you: documents, websites, charts, images and cards, with who made it and when.'));const box=el('div','vw-saved');content.append(box);if(root.gregalVentsActivity)root.gregalVentsActivity.renderSaved(box,{onCard:it=>{const w=state.winds.find(x=>x.id===it.wind);if(w)windCards(w);}});else box.append(el('p','vw-muted',t('Carregant…','Loading…')));}
 // Racó i Automatitzacions també són de Vents: al telèfon porten la mateixa barra de baix, que torna a Vents.
 function pageNavs(){for(const id of ['racoPage','automatitzacionsPage']){const page=document.getElementById(id);if(!page)continue;page.querySelector(':scope > .vw-page-nav')?.remove();const bottom=el('nav','vw-mobile-nav vw-page-nav');bottom.setAttribute('aria-label',t('Seccions dels vents','Winds sections'));for(const [view,a,b]of MOBILE_NAV){const n=btn('',()=>{root.gregal?.setView?.('vents');route(view);});n.append(navIcon(view),el('span','',t(a,b)));bottom.append(n);}page.append(bottom);}}
 function navIcon(id){const paths={home:'M3 7.5 8 3.5l5 4V13H3Z M6.5 13V9.5h3V13',pati:'M8 2.6a3.1 3.1 0 0 1 3.1 3.1c0 1.8-1.4 3.2-3.1 3.2S4.9 7.5 4.9 5.7A3.1 3.1 0 0 1 8 2.6Z M8 8.9V13.4 M5.2 13.4h5.6 M11.6 11.6a1.3 1.3 0 1 0 0 .01',tasks:'M8 2.75a5.25 5.25 0 1 0 0 10.5 5.25 5.25 0 0 0 0-10.5Z M8 5v3.2l2.2 1.3',crew:'M5.5 7.5a2 2 0 1 0 0-4 2 2 0 0 0 0 4Z M10.8 7.2a1.7 1.7 0 1 0 0-3.4 M2.5 13c.4-2.2 1.6-3.4 3-3.4s2.6 1.2 3 3.4 M10 9.7c1.4.1 2.6 1.2 3 3.3',library:'M3 3.5h3v9H3Z M7 3.5h3v9H7Z M11 4.2l2.4-.6 1.9 8.4-2.4.6Z',wiki:'M4 2.75h5.5L12 5.25v8H4Z M9.5 2.75v2.5H12 M6 8h4 M6 10.5h4',saved:'M4 2.75h8v10.5L8 10.5 4 13.25Z',notes:'M4.5 2.75h7v10.5h-7Z M6.5 5.5h3 M6.5 8h3 M6.5 10.5h2 M3.2 4.5h1.3 M3.2 7h1.3 M3.2 9.5h1.3',more:'M3.4 8a.8.8 0 1 0 1.6 0 .8.8 0 1 0-1.6 0 M7.2 8a.8.8 0 1 0 1.6 0 .8.8 0 1 0-1.6 0 M11 8a.8.8 0 1 0 1.6 0 .8.8 0 1 0-1.6 0'};const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 16 16');svg.setAttribute('aria-hidden','true');svg.setAttribute('class','vw-nav-svg');const p=document.createElementNS(svg.namespaceURI,'path');p.setAttribute('d',paths[id]||'');p.setAttribute('fill','none');p.setAttribute('stroke','currentColor');p.setAttribute('stroke-width','1.3');p.setAttribute('stroke-linecap','round');p.setAttribute('stroke-linejoin','round');svg.append(p);return svg;}
 async function openChat(chat){if(state.preparing&&!state.busy)return;const ticket=state.chatEpoch=(state.chatEpoch||0)+1;const data=await call('/api/winds/rotllana?rotlla='+encodeURIComponent(chat.id));if(ticket!==state.chatEpoch)return;state.replyTo=null;state.active=chat;state.history=data.history||[];state.squad=state.winds.find(w=>chat.members.includes(w.id))?.squad||state.squad;state.view='chat';state.draft='';render();followCrew(chat);}
 // Una colla convocada treballa al servidor: el fil s'actualitza sol mentre parlen i fan la feina.
 function followCrew(chat){root.gregalCrew?.follow(chat.id,async st=>{if(state.view!=='chat'||state.active?.id!==chat.id)return false;const steps={discussing:t('La colla prepara el pla…','The crew is planning…'),waiting:t('La colla espera torn…','The crew is waiting…'),working:t('La colla hi treballa…','The crew is working…')};status.textContent=st.running?steps[st.step]||'':st.step==='interrupted'?t('La feina es va aturar: el servidor es va reiniciar. Torna-la a demanar si cal.','The work stopped: the server restarted. Ask for it again if needed.'):'';const data=await call('/api/winds/rotllana?rotlla='+encodeURIComponent(chat.id));if(state.active?.id!==chat.id)return false;if((data.history||[]).length!==state.history.length){state.history=data.history||[];paintHistory();}});}
 // «La Marinada en sap més de compres»: el pas el fas tu, no hi ha cap canvi de veu per sorpresa.
 function suggestRow(s){const w=state.winds.find(w=>w.id===s.wind);const row=el('div','vw-suggest');if(!w)return row;const copy=el('span','');copy.append(el('strong','',w.name),document.createTextNode(' '+t('en sap més de ','knows more about ')+(roleName(w)||t('això','this')).toLowerCase()+'.'));
  row.append(face(w),copy,btn(t('Pregunta-li','Ask '+w.name),async()=>{state.suggest=null;const thread=await call('/api/winds/sessions','POST',{wind:w.id});showWindThread(w,thread.name,[]);if(input){input.value=s.text;composer.requestSubmit();}},'vw-button'),iconOnly('close',t('Ara no','Not now'),()=>{state.suggest=null;row.remove();},'vw-small'));return row;}
 // Després d'una resposta a soles: si un altre vent de la colla hi encaixa clarament millor, ho proposa.
 async function suggestOther(chat,text){if(!chat?.legacyWind)return;try{const cards=(await call('/api/winds/specialists?task='+encodeURIComponent(text.slice(0,600)))).specialists||[];const best=cards[0];if(!best||best.score<2||best.id===chat.wind)return;const mine=cards.find(c=>c.id===chat.wind);if(mine&&mine.score>=best.score)return;if(!state.winds.some(w=>w.id===best.id))return;state.suggest={thread:chat.id,wind:best.id,text};if(state.view==='chat'&&state.active?.id===chat.id)paintHistory();}catch{}}
 // L'ofici al perfil: el nivell, què sap, d'on treu les dades i les vigilàncies que t'ofereix (winds_trade.go).
 function tradePanel(w,tr){const box=el('section','vw-trade');const head=el('div','vw-trade-head');head.append(el('strong','',t(tr.name_ca,tr.name_en)),el('span','vw-trade-level',t('Nivell ','Level ')+tr.level));const meter=el('span','vw-trade-meter');meter.style.setProperty('--lvl',String(Math.min(10,tr.level)));head.append(meter);box.append(head);
  box.append(el('p','vw-muted',t(tr.knowledge+' coneixements estudiats · '+tr.lessons+' lliçons apreses de les converses',tr.knowledge+' studied topics · '+tr.lessons+' lessons learned from chats')));
  if((tr.sources||[]).length){const row=el('div','vw-trade-sources');row.append(el('span','',t('Les seves fonts','Its sources')));for(const src of tr.sources)row.append(el('span','vw-trade-source',src));box.append(row);}
  if((tr.watches||[]).length){box.append(el('h3','',t('El que pot vigilar per tu','What it can watch for you')));for(const wt of tr.watches){const row=el('div','vw-trade-watch');const label=t(wt.label_ca,wt.label_en);row.append(el('span','',label),el('small','',wt.interval_h?t('cada '+wt.interval_h+' h','every '+wt.interval_h+' h'):wt.monthly?t('un cop al mes','once a month'):wt.weekly?t('un cop per setmana','once a week'):t('cada dia a les ','daily at ')+wt.time));const on=btn(t('Activa','Turn on'),async()=>{on.disabled=true;try{await call('/api/winds/rutina','POST',wt.interval_h?{wind:w.id,kind:'interval',interval_h:wt.interval_h,prompt:t(wt.prompt_ca,wt.prompt_en),name:label}:{wind:w.id,kind:'daily',time:wt.time,prompt:t(wt.prompt_ca,wt.prompt_en),name:label});on.textContent=t('Activada','On');on.classList.add('is-on');}catch(e){on.disabled=false;fail(e);}},'vw-button');row.append(on);box.append(row);}}
  box.append(btn(t('Fes-lo més expert','Make it more expert'),()=>expertStudy(w,tr),'vw-avui-tune'));
  return box;}
 // «Fes-lo més expert»: una sessió d'estudi (winds_expert.go) que acaba en un coneixement desat.
 function expertStudy(w,tr){const d=dialog(t('Fes-lo més expert','Make it more expert'));d.append(el('p','vw-muted',t(w.name+' cercarà, llegirà i practicarà el tema, i ho desarà com un coneixement seu. Triga uns minuts.',w.name+' will search, read and practise the topic, then keep it as its own knowledge. It takes a few minutes.')));
  const topic=field(d,t('Què vols que estudiï?','What should it study?'),'');topic.placeholder=t('Per exemple: ','For example: ')+(tr?.kit==='sports'?t('la plantilla i el calendari del Barça','Barça squad and fixtures'):tr?.kit==='cooking'?t('cuina de temporada a Catalunya','seasonal cooking'):(roleName(w)||'').toLowerCase());
  const status=el('p','vw-expert-status');d.append(status,el('p','vw-error'));
  const phases={cerca:t('Cercant…','Searching…'),lectura:t('Llegint…','Reading…'),apren:t('Aprenent i escrivint el manual…','Learning and writing it up…'),fet:t('Fet: ja ho sap.','Done: it knows it now.'),error:t('No ha pogut acabar.','It could not finish.')};
  const go=btn(t('Comença a estudiar','Start studying'),async()=>{const tema=topic.value.trim()||topic.placeholder.replace(/^.*?: /,'');go.disabled=true;await call('/api/winds/expert','POST',{wind:w.id,tema});const tick=async()=>{if(!d.open)return;const st=await call('/api/winds/expert?wind='+encodeURIComponent(w.id)).catch(()=>null);if(!st)return;status.textContent=(phases[st.fase]||'')+(st.passos?' '+st.pas+'/'+st.passos:'');if(st.fase==='fet'||st.fase==='error'){go.disabled=false;return;}setTimeout(tick,2000);};tick();},'vw-primary');
  const foot=el('div','vw-dialog-foot');foot.append(btn(t('Tanca','Close'),()=>d.close(),'vw-button'),go);d.append(foot);setTimeout(()=>topic.focus(),0);}
 // ---- El calaix del vent (winds_calaix.go): el que guarda per respondre a l'instant ----
 const rowKey=it=>{for(const k of ['id','key','title','name','team','equip','artist','titol','nom']){const v=it?.[k];if(v!=null&&String(v).trim())return String(v).trim().toLowerCase();}return JSON.stringify(it);};
 const shelfEmpty={classificacio:t('Quan li demanis la classificació, la desarà aquí i la propera vegada ho tindràs a l’instant.','When you ask for the standings, it keeps them here so the next time is instant.'),llancaments:t('Hi apuntarà els llançaments que s’anunciïn dels que segueixes, amb la data.','It notes upcoming releases from the ones you follow, with their date.'),receptari:t('Les receptes que t’agradin aniran aquí. Digues-li «desa-la» quan te’n doni una.','Recipes you like go here. Say “save it” when it gives you one.'),llocs:t('Tria on vols seguir el temps.','Choose where you want to follow the weather.')};
 async function calaixPanel(body,w){const data=await call('/api/winds/calaix?wind='+encodeURIComponent(w.id));body.replaceChildren();
  body.append(el('p','vw-muted',t('El que '+w.name+' guarda per respondre’t a l’instant, sense tornar a buscar. Ho manté al dia ell mateix.','What '+w.name+' keeps to answer you at once without searching again. It keeps it up to date itself.')));
  const redraw=()=>calaixPanel(body,w).catch(fail);
  if(!(data.shelves||[]).length)body.append(el('p','vw-empty',t('Encara no hi ha res al calaix.','Nothing in the drawer yet.')));
  for(const sh of data.shelves||[]){const sec=el('section','vw-shelf');sec.dataset.kind=sh.kind;const head=el('header','vw-shelf-head');head.append(el('h3','',sh.title||sh.name),el('small','',sh.items.length?(sh.items.length+' · '+(sh.updated?new Date(sh.updated*1000).toLocaleDateString(ca()?'ca':'en',{day:'numeric',month:'short'}):'')):''));
   if(sh.kind==='places')head.append(btn(t('Afegeix un lloc','Add a place'),()=>placePicker(w,redraw),'vw-avui-chip'));sec.append(head);if(sh.note)sec.append(el('p','vw-muted',sh.note));
   const remove=it=>iconOnly('close',t('Treu-ho','Remove'),async()=>{await call('/api/winds/calaix','POST',{wind:w.id,shelf:sh.name,action:'remove',key:rowKey(it)});redraw();},'vw-small vw-row-x');
   if(!sh.items.length){sec.append(el('p','vw-shelf-empty',shelfEmpty[sh.name]||t('Buit de moment.','Empty for now.')));body.append(sec);continue;}
   if(sh.kind==='releases'){const today=new Date().toISOString().slice(0,10);let month='';for(const it of sh.items){const date=String(it.date||'');const m=date.slice(0,7);if(m&&m!==month){month=m;let label=m;try{label=new Date(m+'-15').toLocaleDateString(ca()?'ca':'en',{month:'long',year:'numeric'});}catch{}sec.append(el('p','vw-shelf-month',label));}
     const row=el('div','vw-release'+(date&&date<today?' is-past':''));const day=el('span','vw-release-day');try{const dd=new Date(date+'T12:00');day.append(el('b','',String(dd.getDate())),el('small','',dd.toLocaleDateString(ca()?'ca':'en',{weekday:'short'})));}catch{day.textContent=date;}
     const copy=el('span','vw-release-copy');copy.append(el('strong','',it.title||''),el('small','',[it.artist||it.author,it.type||it.publisher].filter(Boolean).join(' · ')));row.append(day,copy);if(it.link){const a=el('a','vw-release-link',t('Obre','Open'));a.href=it.link;a.target='_blank';a.rel='noopener';row.append(a);}row.append(remove(it));sec.append(row);}}
   else if(sh.kind==='recipes'){for(const it of sh.items){const card=el('details','vw-recipe');const sum=el('summary','');sum.append(el('strong','',it.title||''),el('small','',[it.servings?t(it.servings+' racions',it.servings+' servings'):'',it.time||''].filter(Boolean).join(' · ')));card.append(sum);
     const inner=el('div','vw-recipe-body');if((it.ingredients||[]).length){inner.append(el('h4','',t('Ingredients','Ingredients')));const ul=el('ul');for(const x of it.ingredients)ul.append(el('li','',String(x)));inner.append(ul);}if((it.steps||[]).length){inner.append(el('h4','',t('Passos','Steps')));const ol=el('ol');for(const x of it.steps)ol.append(el('li','',String(x)));inner.append(ol);}if(it.tips)inner.append(el('p','vw-recipe-tip',String(it.tips)));
     const meta=[it.source?t('Font: ','Source: ')+it.source:'',it.reviewed?t('Revisada el ','Reviewed on ')+it.reviewed:''].filter(Boolean).join(' · ');if(meta)inner.append(el('p','vw-muted',meta));inner.append(remove(it));card.append(inner);sec.append(card);}}
   else if(sh.kind==='places'){for(const it of sh.items){const row=el('div','vw-place'+(it.primary?' is-primary':''));row.append(el('span','',it.name||''));if(it.primary)row.append(el('small','vw-place-main',t('principal','main')));else row.append(btn(t('Fes-lo principal','Make it main'),async()=>{await call('/api/winds/calaix','POST',{wind:w.id,shelf:sh.name,action:'primary',key:rowKey(it)});redraw();},'vw-avui-tune'));row.append(remove(it));sec.append(row);}}
   else if(sh.kind==='list'){const chips=el('div','vw-shelf-chips');for(const it of sh.items){const c=el('span','vw-trade-source');c.append(document.createTextNode(it.name||it.title||rowKey(it)));const x=remove(it);c.append(x);chips.append(c);}sec.append(chips);}
   else{const cols=[...new Set(sh.items.flatMap(it=>Object.keys(it)))].slice(0,7);const table=el('table','md vw-shelf-table');const tr=el('tr');for(const c of cols)tr.append(el('th','',c));table.append(tr);for(const it of sh.items){const r=el('tr');for(const c of cols)r.append(el('td','',it[c]==null?'':typeof it[c]==='object'?JSON.stringify(it[c]):String(it[c])));table.append(r);}const wrap=el('div','vw-message-text');wrap.append(table);sec.append(wrap);}
   body.append(sec);}
 }
 // ---- El temps: el lloc principal del vent, previsió d'Open-Meteo, gràfic propi ----
 const wxKind=code=>code===0?'clear':code<=2?'partly':code===3?'cloudy':code<=48?'fog':code<=57?'drizzle':code<=67?'rain':code<=77?'snow':code<=82?'rain':code<=86?'snow':'storm';
 const wxName=k=>({clear:t('Serè','Clear'),partly:t('Clarianes','Partly cloudy'),cloudy:t('Ennuvolat','Cloudy'),fog:t('Boira','Fog'),drizzle:t('Plugim','Drizzle'),rain:t('Pluja','Rain'),snow:t('Neu','Snow'),storm:t('Tempesta','Storm')})[k]||'';
 function wxIcon(k,day=true,size=28){const sun=day?'<circle cx="12" cy="12" r="4.2" fill="#FFC857"/><g stroke="#FFC857" stroke-width="1.8" stroke-linecap="round"><path d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5.3 5.3l1.4 1.4M17.3 17.3l1.4 1.4M5.3 18.7l1.4-1.4M17.3 6.7l1.4-1.4"/></g>':'<path d="M15.5 4.5a7.5 7.5 0 1 0 4 12.8 6 6 0 0 1-4-12.8z" fill="#C9D2FF"/>';
  const cloud=(fill,dx=0,dy=0)=>'<path transform="translate('+dx+' '+dy+')" d="M7 19h10.5a4 4 0 0 0 .4-8 5.5 5.5 0 0 0-10.6-1.2A4.2 4.2 0 0 0 7 19z" fill="'+fill+'"/>';
  const drops=c=>'<g stroke="'+c+'" stroke-width="1.8" stroke-linecap="round"><path d="M8 20.5l-1 2M12 20.5l-1 2M16 20.5l-1 2"/></g>';
  const inner={clear:sun,partly:'<g transform="translate(-3 -3) scale(.8)">'+sun+'</g>'+cloud('#E7ECF5',2,2),cloudy:cloud('#C3CAD8',0,-1),fog:cloud('#C3CAD8',0,-3)+'<g stroke="#9AA3B5" stroke-width="1.8" stroke-linecap="round"><path d="M4 20h16M6 23h12"/></g>',drizzle:cloud('#B7C0D2',0,-3)+drops('#7FB8F0'),rain:cloud('#9EA8BC',0,-3)+drops('#5AA2F0'),snow:cloud('#D5DBE6',0,-3)+'<g fill="#FFFFFF"><circle cx="8" cy="21.5" r="1.2"/><circle cx="12" cy="22.5" r="1.2"/><circle cx="16" cy="21.5" r="1.2"/></g>',storm:cloud('#8C95AA',0,-3)+'<path d="M12.5 17l-3 4.5h3l-1.5 3.5 4.5-5h-3l1.5-3z" fill="#FFC857"/>'}[k]||sun;
  return '<svg viewBox="0 0 24 26" width="'+size+'" height="'+size+'" aria-hidden="true">'+inner+'</svg>';}
 function wxChart(f){const H=f.hourly||{},times=H.time||[];const now=new Date();let start=times.findIndex(x=>new Date(x)>=new Date(now.getTime()-3600e3));if(start<0)start=0;const n=Math.min(24,times.length-start);if(n<2)return el('div');
  const temps=H.temperature_2m.slice(start,start+n),rain=(H.precipitation_probability||[]).slice(start,start+n);const W=640,top=24,plotH=92,barH=30,base=top+plotH+barH+22,Ht=base+6,pad=14;const lo=Math.min(...temps),hi=Math.max(...temps),span=Math.max(2,hi-lo);
  const x=i=>pad+i*(W-2*pad)/(n-1),y=v=>top+plotH-((v-lo)/span)*plotH*.82-plotH*.09;const line=temps.map((v,i)=>(i?'L':'M')+x(i).toFixed(1)+' '+y(v).toFixed(1)).join('');const area=line+'L'+x(n-1).toFixed(1)+' '+(top+plotH)+'L'+x(0).toFixed(1)+' '+(top+plotH)+'Z';
  let svg='<svg class="vw-wx-chart" viewBox="0 0 '+W+' '+Ht+'" role="img" aria-label="'+t('Temperatura i probabilitat de pluja de les properes hores','Temperature and chance of rain in the next hours')+'"><defs><linearGradient id="wxg" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="var(--wx-warm)" stop-opacity=".45"/><stop offset="1" stop-color="var(--wx-warm)" stop-opacity="0"/></linearGradient></defs>';
  svg+='<path d="'+area+'" fill="url(#wxg)"/><path d="'+line+'" fill="none" stroke="var(--wx-warm)" stroke-width="2.4" stroke-linejoin="round" stroke-linecap="round"/>';
  const bw=(W-2*pad)/n*.55;rain.forEach((p,i)=>{if(!(p>=5))return;const h=Math.max(2,p/100*barH);svg+='<rect x="'+(x(i)-bw/2).toFixed(1)+'" y="'+(top+plotH+barH-h+4).toFixed(1)+'" width="'+bw.toFixed(1)+'" height="'+h.toFixed(1)+'" rx="2" fill="var(--wx-rain)" opacity="'+(p>=50?.95:.55)+'"/>';});
  const iMax=temps.indexOf(hi),iMin=temps.indexOf(lo);for(const i of new Set([iMax,iMin]))svg+='<text x="'+x(i).toFixed(1)+'" y="'+(y(temps[i])-8).toFixed(1)+'" text-anchor="middle" class="vw-wx-tlabel">'+Math.round(temps[i])+'°</text>';
  for(let i=0;i<n;i+=3){const d=new Date(times[start+i]);svg+='<text x="'+x(i).toFixed(1)+'" y="'+base+'" text-anchor="middle" class="vw-wx-hour">'+(i===0?t('Ara','Now'):String(d.getHours()).padStart(2,'0')+'h')+'</text>';if(rain[i]>=30)svg+='<text x="'+x(i).toFixed(1)+'" y="'+(top+plotH+4).toFixed(1)+'" text-anchor="middle" class="vw-wx-rainlabel">'+rain[i]+'%</text>';}
  const box=el('div','vw-wx-chartbox');box.innerHTML=svg+'</svg>';return box;}
 async function weatherCard(host,w,compact){const data=await call('/api/winds/weather?wind='+encodeURIComponent(w.id));host.replaceChildren();const card=el('section','vw-wx'+(compact?' is-compact':''));
  if(!data.place){if(compact){host.remove();return;}card.append(el('p','vw-muted',t('Tria on vols seguir el temps i '+w.name+' et farà la previsió per hores i per dies.','Choose where to follow the weather and '+w.name+' will forecast it by the hour and by the day.')),btn(t('Tria un lloc','Choose a place'),()=>placePicker(w,()=>weatherCard(host,w,compact)),'vw-primary'));host.append(card);return;}
  const f=data.forecast||{},c=f.current||{},D=f.daily||{};const kind=wxKind(c.weather_code);
  const top=el('div','vw-wx-top'),now=el('div','vw-wx-now');now.innerHTML=wxIcon(kind,c.is_day!==0,compact?40:52);const temp=el('div','vw-wx-temp');temp.append(el('b','',Math.round(c.temperature_2m)+'°'),el('span','',wxName(kind)));now.append(temp);
  const place=el('div','vw-wx-place');const name=btn('',()=>placePicker(w,()=>weatherCard(host,w,compact)),'vw-wx-placebtn');name.append(el('strong','',data.place.name||''));name.title=t('Canvia el lloc','Change the place');place.append(name,el('small','',t('Sensació ','Feels like ')+Math.round(c.apparent_temperature)+'° · '+t('vent ','wind ')+Math.round(c.wind_speed_10m)+' km/h'));
  if(compact){const from=el('small','vw-wx-by');from.append(face(w),document.createTextNode(w.name));place.append(from);}
  top.append(now,place);card.append(top,wxChart(f));
  const days=el('div','vw-wx-days');(D.time||[]).slice(0,7).forEach((day,i)=>{const cell=el('div','vw-wx-day');const dd=new Date(day+'T12:00');cell.append(el('small','',i===0?t('Avui','Today'):dd.toLocaleDateString(ca()?'ca':'en',{weekday:'short'})));const ic=el('span','');ic.innerHTML=wxIcon(wxKind(D.weather_code[i]),true,26);cell.append(ic,el('b','',Math.round(D.temperature_2m_max[i])+'°'),el('span','vw-wx-min',Math.round(D.temperature_2m_min[i])+'°'));const p=(D.precipitation_probability_max||[])[i];if(p>=20)cell.append(el('span','vw-wx-p',p+'%'));days.append(cell);});
  card.append(days);host.append(card);}
 function placePicker(w,after){const d=dialog(t('On vols seguir el temps?','Where do you want to follow the weather?'));const q=field(d,t('Cerca un lloc','Search a place'),'');q.placeholder=t('Barcelona, Puigcerdà, Sóller…','Barcelona, Puigcerdà, Sóller…');const list=el('div','vw-place-results');d.append(list,el('p','vw-error'));let timer=0;
  q.oninput=()=>{clearTimeout(timer);timer=setTimeout(async()=>{const r=await call('/api/winds/weather/places?q='+encodeURIComponent(q.value)).catch(()=>({places:[]}));list.replaceChildren();for(const p of r.places||[]){const b=btn('',async()=>{await call('/api/winds/calaix','POST',{wind:w.id,shelf:'llocs',action:'add',item:{name:p.name,lat:p.lat,lon:p.lon,where:p.where,primary:true}});await call('/api/winds/calaix','POST',{wind:w.id,shelf:'llocs',action:'primary',key:p.name.toLowerCase()});d.close();after?.();},'vw-place-result');b.append(el('strong','',p.name),el('small','',p.where||''));list.append(b);}},250);};setTimeout(()=>q.focus(),0);}
 function chatView(){content.classList.add('vw-thread-view');messages=el('div','vw-messages');messages.dataset.thread=state.active.id;content.append(messages);paintHistory();const linked=state.tasks.filter(task=>task.thread===state.active.id);for(const task of linked)messages.append(linkRow(task.task+' · '+statusName(task.status),()=>{state.events=task.events||[];route('task',{...task,name:task.task,conversation:state.active});}));content.append(el('div','vw-asks'));paintAsks();loadAsks();addComposer(!!state.active?.workspace);}
 function emptyWindThread(){
  const wind=state.winds.find(w=>w.id===state.active?.wind);
  if(!wind)return;
  const card=el('div','vw-thread-empty');
  const orb=el('div','vw-orb');
  orb.append(figure(wind,true));
  card.append(orb,el('h1','',wind.name),el('p','',roleName(wind)||t('Conversa directa','Direct conversation')));
  messages.append(card);
 }
 const choicesStrip=text=>root.gregalChoices?root.gregalChoices.strip(text):text;
 const askTitle=kind=>({install:t('Vol instal·lar programari','Wants to install software'),download:t('Vol baixar un paquet o un fitxer','Wants to download a package or file'),delete:t('Vol esborrar fora del projecte','Wants to delete outside the project'),system:t('Vol canviar un servei del sistema','Wants to change a system service')}[kind]||t('Et demana permís','Asks for your permission'));
 async function answerAsk(a,body){state.asks=state.asks.filter(x=>x.key!==a.key);paintAsks();try{await call('/api/winds/answer','POST',{key:a.key,...body});}catch(e){fail(e);}}
 // Visor de l'ordinador (B3): la finestra s'obre al clic (si no, el navegador la bloqueja) i després hi va el testimoni d'un sol ús.
 async function openComputer(handoff){const win=root.open('about:blank','gregal-ordinador');try{const r=await call('/api/ordinador/viewer','POST',{handoff:handoff||'',control:!!handoff});const lang=ca()?'ca':'en';if(win)win.location.href=r.url+(r.url.includes('?')?'&':'?')+'lang='+lang;}catch(e){if(win)win.close();const m=String(e&&e.message||'');if(/no computer|another account|unknown computer/.test(m))fail(new Error(t("El teu compte encara no té cap ordinador configurat. Les feines que en necessiten esperen fins que n'hi hagi.",'No computer is set up for your account yet. Tasks that need it wait until there is one.')));else if(/no_viewer/.test(m))fail(new Error(t('Aquest servidor no té visor remot.','This server has no remote viewer.')));else fail(e);}}
 // Connecta el compte que una feina espera: el consentiment s'obre al clic (si no, el navegador el bloqueja).
 async function connectFor(a){const win=root.open('about:blank','gregal-connexio');const site=String(a.site||'');const body=site.startsWith('con-')?{id:site}:{service:'mcp',provider:site.replace(/^mcp:/,'')};try{const r=await call('/api/connexions/connect','POST',body);if(win)win.location.href=r.authUrl;}catch(e){if(win)win.close();const m=String(e&&e.message||'');fail(/needs_client_id/.test(m)?new Error(t("Aquest proveïdor necessita que l'administrador hi registri Gregal. Mentrestant, pots iniciar sessió al navegador de l'ordinador.","This provider needs the administrator to register Gregal with it. Meanwhile, you can sign in on the computer's browser.")):e);}}
 async function loadAsks(){try{state.asks=(await call('/api/winds/pending')).pending||[];paintAsks();}catch(e){}}
 // Targetes d'aprovació i de pregunta: el torn del vent espera aquí.
 function paintAsks(){
  const box=page?.querySelector('.vw-asks');if(!box)return;box.replaceChildren();
  for(const a of state.asks){
   const card=el('section','vw-ask'+(a.type==='question'?' vw-ask-question':' vw-ask-approve'));card.setAttribute('role','group');
   if(a.type==='handoff'){
    // B3: la feina espera que la persona faci una cosa a l'ordinador (iniciar sessió, 2FA). El visor s'obre amb un testimoni curt.
    card.className='vw-ask vw-ask-approve vw-ask-handoff';
    card.append(el('strong','',t("Et toca a tu a l'ordinador",'Your turn on the computer')),el('small','vw-ask-wait',t('Espera la teva resposta','Waiting for you')));
    if(a.what)card.append(el('p','',a.what));
    card.append(el('small','',t("Gregal no escriu mai contrasenyes ni codis: ho fas tu a l'ordinador i l'encàrrec continua.",'Gregal never types passwords or codes: you do it on the computer and the task carries on.')));
    const row=el('div','vw-ask-actions');
    row.append(btn(t('Cancel·la','Cancel'),()=>answerAsk(a,{approve:false}),'vw-ask-decline'),btn(t('Ja està','Done'),()=>answerAsk(a,{approve:true}),'vw-ask-decline'),btn(t("Obre l'ordinador",'Open the computer'),()=>openComputer(a.key),'vw-ask-allow'));
    card.append(row);
   }else if(a.type==='connection'){
    // B5: la feina espera que connectis un compte (waiting_connection). En acabar el consentiment continua sola.
    card.className='vw-ask vw-ask-approve vw-ask-handoff';
    card.append(el('strong','',t('Cal connectar un compte','An account needs connecting')),el('small','vw-ask-wait',t('La feina espera','The task is waiting')));
    if(a.what)card.append(el('p','',a.what));
    if(a.reason)card.append(el('small','',a.reason));
    const row=el('div','vw-ask-actions');
    row.append(btn(t('Cancel·la','Cancel'),()=>answerAsk(a,{approve:false}),'vw-ask-decline'),btn(t('Connecta','Connect'),()=>connectFor(a),'vw-ask-allow'));
    card.append(row);
   }else if(a.type==='question'){
    card.append(el('strong','',a.query||''));
    for(const o of a.options||[]){const b=btn(o.label,()=>answerAsk(a,{answer:o.label}),'vw-ask-option');if(o.description)b.append(el('small','',o.description));card.append(b);}
    const free=el('input','vw-ask-free');free.placeholder=t('Una altra cosa…','Something else…');free.setAttribute('aria-label',free.placeholder);free.onkeydown=e=>{if(e.key==='Enter'&&free.value.trim()){e.preventDefault();answerAsk(a,{answer:free.value.trim()});}};card.append(free);
   }else{
    card.append(el('strong','',askTitle(a.kind)),el('small','vw-ask-wait',t('Espera la teva resposta','Waiting for you')));
    if(a.what)card.append(el('code','',a.what));
    const row=el('div','vw-ask-actions');row.append(btn(t('Rebutja','Decline'),()=>answerAsk(a,{approve:false}),'vw-ask-decline'),btn(t('Permet','Allow'),()=>answerAsk(a,{approve:true}),'vw-ask-allow'));card.append(row);
   }
   box.append(card);
  }
 }
 function paintHistory(){const top=messages.scrollTop,follow=nearChatEnd(messages);messages.replaceChildren();let voice='',asked=false;for(const item of state.history){const user=item.from==='user';
  // Un altre vent agafa el fil després d'un missatge teu: una línia ho diu, en lloc que el vent ho expliqui.
  if(user)asked=true;else if(!state.active?.legacyWind){const id=item.speaker_id||'';if(voice&&id&&id!==voice&&asked){const who=state.winds.find(w=>w.id===id);if(who)messages.append(el('p','vw-handoff',t('→ '+who.name+' agafa el fil','→ '+who.name+' takes over')));}if(id)voice=id;asked=false;}const w=state.winds.find(w=>w.id===item.speaker_id||w.id===item.from||w.name===item.from)||(state.active?.legacyWind?state.winds.find(w=>w.id===state.active.wind):null);const row=el('article',user?'vw-message vw-user':'vw-message');if(!user)row.append(face(w));const copy=el('div'),body=el('div','vw-message-text'),text=user?item.text:choicesStrip(withoutSpeaker(item.text,w?.name||item.speaker_name));if(root.gregalMD)body.innerHTML=root.gregalMD.render(text);else body.textContent=text;copy.append(el('strong','',user?t('Tu','You'):w?.name||item.speaker_name||t('La colla','Your crew')),body);
  // Un missatge proactiu (B6, amb run o resultat): «per què ho has fet?».
  const R=root.gregalResponsabilitats;if(!user&&R&&R.isProactive(item)&&item.id){const why=btn(R.whyLabel(),()=>R.openWhy(item.id),'vw-why');why.dataset.message=item.id;row.dataset.proactive='1';copy.append(why);}
  row.append(copy);messages.append(row);}
  // Respostes ràpides que el vent proposa al final del seu missatge.
  if(state.suggest?.thread===state.active?.id&&!state.busy)messages.append(suggestRow(state.suggest));
  const last=state.history[state.history.length-1];if(last&&last.from!=='user'&&!state.busy){const opts=root.gregalChoices?root.gregalChoices.extract(last.text):[];if(opts.length){const row=el('div','vw-choices');for(const o of opts)row.append(btn(o,()=>{if(!input)return;state.replyOnce=last.speaker_id||null;input.value=o;composer.requestSubmit();},'vw-choice'));messages.append(row);}}
  if(!last&&state.active?.legacyWind)emptyWindThread();
  paintPending();messages.scrollTop=follow?messages.scrollHeight:top;}
 async function submit(){if(state.preparing)return;state.preparing=true;try{await submitRequest();}finally{state.preparing=false;}}
 async function submitRequest(){
  const text=input.value.trim();if(!text||state.busy)return;state.draft=text;
  const anyone=state.recipient==='auto'||state.recipient==='crew';
  const prepareWork=state.view!=='chat'&&!state.active?.legacyWind&&(anyone||state.active?.project);
  const work=prepareWork&&(await call('/api/winds/intent','POST',{message:text})).intent==='work';
  if(state.view==='home'&&anyone&&state.squad==='daily'&&work){
   if(/\b(web|website|codi|code|app|bug|html|seguretat|security)\b/i.test(text))state.squad='software';
   else if(/\b(dades|data|regress|machine learning|series|temporal|csv)\b/i.test(text))state.squad='data';
  }
  if(state.view==='home'&&!anyone&&!state.active?.project){
   const wind=state.winds.find(w=>w.id===state.recipient);
   if(wind)await talkTo(wind);
  }
  if(state.view!=='chat'&&!crew().length&&anyone)await activate(state.squad);
  // Automàtic: una conversa nova amb un sol vent, el que millor encaixa amb el que demanes. La
  // conversa en grup només hi és quan la tries («Tota la colla») o hi convides vents.
  if(state.view==='home'&&state.recipient==='auto'&&!work&&!state.active?.project){
   const wind=await bestWind(text);
   if(wind){const thread=await call('/api/winds/sessions','POST',{wind:wind.id});showWindThread(wind,thread.name,[]);}
  }
  const members=state.view==='chat'?state.winds.filter(w=>state.active.members.includes(w.id)):state.recipient&&!anyone?state.winds.filter(w=>w.id===state.recipient):crew();if(!members.length)throw new Error(t('Tria un vent de la colla.','Choose a wind from the crew.'));
  // Cap vent no hi encaixa: proposa convocar una colla (vents existents + especialistes nous).
  if(work&&anyone&&root.gregalCrew&&!(state.crewTried||=new Set()).has(text)){state.crewTried.add(text);status.textContent=t('Mirant qui ho pot fer…','Checking who can do it…');const crewChat=await root.gregalCrew.offer(text,state.winds,members.map(w=>w.id));status.textContent='';if(crewChat){input.value='';state.draft='';state.chats.unshift(crewChat);state.winds=(await call('/api/winds')).winds||state.winds;state.preparing=false;await openChat(crewChat);return;}}
  if(work){
   const selected=root.gregalWindDesk.resolveTask(members,text,{owner:anyone?null:state.recipient});const flow=root.gregalWindDesk.flowFor(selected.owner,selected.reviewer,selected.kind);
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
  state.busy=true;
  state.notice=null;
  const turn={},progress={};
  chatTurn=turn;
  const current=()=>state.view==='chat'&&state.active?.id===sentTo?.id;
  const restore=()=>{
   if(!current())return;
   if(progress.history){state.history=progress.history;state.draft='';}
   else{state.draft=text;state.history=state.history.filter(m=>!m.optimistic);}
  };
  try{
   if(!sentTo){
    const data=await call('/api/winds/rotllanes','POST',{name:text.slice(0,75),members:members.map(w=>w.id),workspace});
    sentTo=data.rotllana;state.chats.unshift(sentTo);
    if(epoch===(state.chatEpoch||0)){state.active=sentTo;state.history=[];}
   }
   if(sentTo.workspace)await switchFolder(sentTo.workspace);
   state.pending={thread:sentTo.id,wind:sentTo.legacyWind?sentTo.wind:sentTo.members?.length===1?sentTo.members[0]:null,phase:'thinking',text:''};
   if(epoch===(state.chatEpoch||0)){
    state.history.push({from:'user',text,optimistic:true});state.draft='';state.view='chat';render();
   }
   const data=await streamChat(sentTo,text,turn,progress);
   if(data.cancelat){restore();state.notice={thread:sentTo.id,text:t('Torn aturat','Turn stopped')};}
   else if(current()){
    state.history=data.history||state.history;
    suggestOther(sentTo,text);
    if(sentTo.legacyWind)refreshSolo().then(()=>renderNav());
    if(data.failed?.length)state.notice={thread:sentTo.id,text:t('No han pogut completar el torn: ','Could not complete the turn: ')+data.failed.join(', ')+(data.failed_reason?' · '+root.gregalFriendlyRunError(data.failed_reason):'')};
   }
  }catch(error){restore();throw error;}
  finally{
   if(chatTurn===turn)chatTurn=null;
   state.pending=null;state.busy=false;
   if(state.view==='chat')render();
  }
 }
 const statusName=value=>({started:t('En marxa','Running'),working:t('En marxa','Running'),done:t('Enllestit','Complete'),completed:t('Enllestit','Complete'),failed:t('Cal revisar-ho','Needs attention'),cancelled:t('Aturat','Stopped'),waiting:t('En espera','Waiting')})[value]||value;
 function taskView(){content.classList.add('vw-thread-view');const title=state.active?.name||state.snapshot?.task||'';heading(title);if(state.active?.conversation)content.append(iconBtn('back',t('Torna a la conversa','Back to conversation'),()=>openChat(state.active.conversation),'vw-back'));const work=el('div','vw-work');work.id='vwWork';content.append(work);paintWork();addComposer(true);input.placeholder=t('Una altra cosa que vulguis fer…','Something else you’d like to do…');}
 function paintWork(){const work=$('vwWork');if(!work)return;const previousScroll=work.scrollTop,following=work.scrollHeight-work.clientHeight-work.scrollTop<48,logOpen=work.querySelector('.vw-log')?.open||false,reviewOpen=work.querySelector('.vw-review')?.open||false;work.replaceChildren();const snapshot=state.active?.live?(state.liveSnapshot||state.snapshot):null;const busy=state.active?.live&&state.busy;work.append(el('p','vw-progress',statusName(snapshot?.state||state.active?.status||(busy?'started':'done'))));
  for(const n of snapshot?.nodes||[]){const row=el('div','vw-live-agent');row.append(face(state.winds.find(w=>n.wind?w.id===n.wind:w.name===n.name),n),el('strong','',n.name),el('span','',statusName(n.state)+(n.activity?' · '+activityName(n.activity):'')));work.append(row);}
  const events=state.active?.live?(state.liveEvents||state.events):state.active?.events||state.events;const paths=[...new Set(events.filter(e=>e.type==='tool_result'&&e.path).map(e=>e.path))];
  // The deliverable leads; the reviewer's verdict is folded under the builder's own summary.
  const outputs=events.filter(e=>e.output),review=outputs.findLast(e=>e.type==='completed'&&e.activity==='review'),made=outputs.findLast(e=>e.type==='completed'&&e.activity!=='review'),final=outputs.findLast(e=>e.type==='done')?.output||outputs.findLast(e=>e.type==='completed')?.output||'',summary=review?made?.output||'':final;
  const markdown=text=>{const n=el('div','vw-result-text');if(root.gregalMD)n.innerHTML=root.gregalMD.render(text);else n.textContent=text;return n;};
  if(paths.length||final){const card=el('section','vw-deliverable');card.append(el('h2','',t('El resultat','The result')));for(const path of paths){const b=iconBtn('open',path.split(/[\\/]/).pop(),async()=>{await switchFolder(state.active?.workspace||state.workspace);await root.gregalWindRunner.openResult(summary||final,titleSafe(),path);},/\.html?$/i.test(path)?'vw-primary':'vw-button');b.title=path;card.append(b);}if(!paths.length)card.append(btn(t('Obre el resultat','Open result'),()=>root.gregalWindRunner.openResult(summary||final,titleSafe()),'vw-primary'));work.append(card);}
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
 function activityName(value){return ({read:t('Llegint fitxers','Reading files'),write:t('Escrivint fitxers','Writing files'),edit:t('Editant fitxers','Editing files'),patch:t('Editant fitxers','Editing files'),glob:t('Buscant fitxers','Finding files'),grep:t('Cercant al codi','Searching the code'),lsp:t('Analitzant el codi','Analysing the code'),bash:t('Comprovant el projecte','Checking the project'),bash_background:t('Comprovant el projecte','Checking the project'),bash_output:t('Comprovant el projecte','Checking the project'),bash_kill:t('Aturant una comprovació','Stopping a check'),browser:t('Fent servir el navegador','Using the browser'),preview:t('Previsualitzant','Previewing'),read_image:t('Mirant una imatge','Viewing an image'),web_search:t('Cercant fonts','Searching sources'),web_fetch:t('Llegint fonts','Reading sources'),skill:t('Consultant un coneixement','Consulting knowledge'),knowledge_search:t('Consultant un coneixement','Consulting knowledge'),knowledge_note:t('Desant una nota','Saving a note'),notes:t('Fent servir les notes','Using the notes'),todowrite:t('Organitzant els passos','Organising the steps'),todoread:t('Organitzant els passos','Organising the steps'),office_create:t('Treballant en un document','Working on a document'),office_edit:t('Treballant en un document','Working on a document'),office_read:t('Llegint un document','Reading a document'),office_open:t('Obrint un document','Opening a document'),office_render:t('Previsualitzant un document','Previewing a document'),git_commit:t('Desant els canvis a git','Committing changes'),delegate:t('Delegant una part','Delegating a part'),question:t('Preparant una pregunta','Preparing a question'),draft:t('Preparant el lliurable','Preparing the deliverable'),analyze:t('Analitzant','Analysing'),plan:t('Planificant','Planning'),revision:t('Corregint','Revising'),reviewing:t('Revisant una acció','Reviewing an action'),review:t('Revisant el resultat','Reviewing the result'),tool_result:t('Comprovació acabada','Check complete'),working:t('Treballant','Working'),done:t('Enllestit','Complete'),completed:t('Pas completat','Step complete'),started:t('Començant','Starting')})[value]||(/^[\w.:-]+$/.test(value||'')?t('Fent servir una eina','Using a tool'):value);}
 async function load(){mount();root.gregalWinds.watchNews?.();const version=++loadVersion;try{const [winds,squads,chats,tasks,meta]=await Promise.all([call('/api/winds'),call('/api/winds/squads'),call('/api/winds/rotllanes'),call('/api/winds/tasks'),call('/api/winds/meta')]);if(version!==loadVersion)return;state.winds=winds.winds||[];state.squads=squads.squads||[];state.chats=chats.rotllanes||[];state.tasks=tasks.tasks||[];state.meta=meta.meta||{};await Promise.all([refreshProjects(),refreshSolo()]);
  if(!state.restoredDraft&&!location.hash){
   state.restoredDraft=true;
   let saved;try{saved=JSON.parse(sessionStorage.getItem(draftKey));}catch{}
   if(saved){
    const wind=state.winds.find(w=>w.id===saved.wind),chat=state.chats.find(c=>c.id===saved.chat);
    try{if(wind)await talkTo(wind,saved.chat);else if(chat)await openChat(chat);}catch{}
    state.draft=typeof saved.draft==='string'?saved.draft:'';
   }
  }
  render();openFromHash();}catch(e){render();fail(e);}}
 // #notes obre les notes; #notes-capture, amb la captura ràpida a punt (drecera Ctrl+Alt+N de l'escriptori).
 function openFromHash(){const h=location.hash;if(!h.startsWith('#notes'))return;history.replaceState(null,'',location.pathname+location.search);openNotes(h==='#notes-capture');}
 function openNotes(capture){if(state.view!=='notes')route('notes');if(capture)setTimeout(()=>root.gregalNotes?.focusCapture?.(),0);}
 root.gregalWindsNotes=openNotes;
 // Les notes (winds-notes.js) fan servir les mateixes icones.
 root.gregalWindIcon=icon;
 root.gregalWindWorkspace={update(snapshot){if(snapshot.source!=='winds')return;state.liveSnapshot=snapshot;if(state.active?.live)state.snapshot=snapshot;paintWork();},event(event){const events=state.liveEvents||state.events;events.push(event);if(events.length>160)events.shift();paintWork();},load};
 // Error de connexió del proveïdor traduït a humà (mateixa taula que Go i l'app).
 root.gregalFriendlyRunError=function(raw){const e=(raw||'').trim();if(!e)return e;const l=e.toLowerCase();const m=e.match(/lookup ([a-z0-9.-]+)/i);const host=m?m[1]:'';if(l.includes('no such host')||l.includes('dns'))return host?t('No s’ha pogut arribar al proveïdor: el dispositiu no resol l’adreça '+host+'. Comprova que hi ha internet i torna-ho a provar.','Could not reach the provider: the device cannot resolve '+host+'. Check the connection and try again.'):t('No s’ha pogut arribar al proveïdor: no es resol la seva adreça. Comprova que hi ha internet i torna-ho a provar.','Could not reach the provider: its address cannot be resolved. Check the connection and try again.');if(l.includes('connection refused'))return t('No s’ha pogut arribar al proveïdor (connexió refusada). Pot ser un tall puntual: torna-ho a provar més tard.','Could not reach the provider (connection refused). Might be a transient outage: try again later.');if(l.includes('context deadline exceeded')||l.includes('timeout'))return t('El proveïdor ha trigat massa a respondre. Torna-ho a provar: si continua així, pot estar saturat.','The provider took too long to respond. Try again: if it persists it may be overloaded.');if(l.includes('certificate')||l.includes('x509'))return t('El proveïdor presenta un certificat que no es confia. Comprova la data/hora del dispositiu.','The provider certificate is not trusted. Check the device date/time.');if(l.includes('401')||l.includes('unauthorized'))return t('El proveïdor ha rebutjat la clau (401). Revisa la clau API a la configuració.','The provider rejected the key (401). Check the API key in settings.');if(l.includes('429')||l.includes('too many requests'))return t('El proveïdor ha limitat les peticions (429). Espera una mica i torna-ho a provar.','The provider is rate limiting (429). Wait a bit and try again.');if(l.includes('no route to host')||l.includes('network is unreachable'))return t('No hi ha ruta cap al servidor. Comprova que hi ha internet i torna-ho a provar.','No route to the server. Check the connection and try again.');if(l==='eof'||l.endsWith('eof'))return t('La connexió amb el proveïdor s’ha tallat a mig camí. Torna-ho a provar.','The connection to the provider was cut midway. Try again.');return e;};
 root.loadWindsPage=load;
 document.addEventListener('gregal:idioma',()=>{if(mounted){state.draft=input?.value??state.draft;render();}});
 document.addEventListener('gregal:state',()=>{page?.querySelectorAll('.vw-model').forEach(paintModel);});
})(window);
