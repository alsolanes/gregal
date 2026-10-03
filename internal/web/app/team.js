const $ = id => document.getElementById(id);
const roles = ['coordinator', 'researcher', 'builder', 'reviewer'];
const baseRoles = [...roles];
const roleColors = ['#a35c21','#167e88','#7756ab','#377a45'];
const roleColor = role => roleColors[baseRoles.indexOf(metadata[role]?.parent || role)] || '#48676b';
const metadata = {}, bubbles = {}, actions = {};
const words = {
  en: { title:'Team', task:'Task', start:'Start', stop:'Stop', idle:'Idle', working:'Working', completed:'Completed', cancelled:'Stopped', failed:'Run failed. Check the model connection and retry.', done:'Complete', started:'Started', handoff:'Handoff', activity:'Activity', agents:'Agents', coordinator:'Coordinator', researcher:'Researcher', builder:'Builder', reviewer:'Reviewer', waiting:'Waiting', empty:'No deliverable yet', interrupted:'Connection interrupted. Retry the task.', textOnly:'Text only', model:'Active model', select:'Select an agent' },
  ca: { title:'Equip', task:'Tasca', start:'Inicia', stop:'Atura', idle:'Inactiu', working:'Treballant', completed:'Completat', cancelled:'Aturat', failed:'Ha fallat. Comprova la connexió del model i torna-ho a provar.', done:'Completat', started:'Iniciat', handoff:'Traspàs', activity:'Activitat', agents:'Agents', coordinator:'Coordinador', researcher:'Investigador', builder:'Constructor', reviewer:'Revisor', waiting:'En espera', empty:'Encara no hi ha cap resultat', interrupted:'Connexió interrompuda. Torna a iniciar la tasca.', textOnly:'Només text', model:'Model actiu', select:'Tria un agent' },
};
const lang = () => document.documentElement.lang === 'ca' ? 'ca' : 'en';
const t = key => words[lang()][key] || key;
const name = role => metadata[role]?.name || t(role);
const actionLabels = {en:{plan:'Planning',analyze:'Analyzing evidence',draft:'Writing a draft',review:'Checking the result',coding:'Writing code · demo',web:'Searching the web · demo',revision:'Applying feedback',discussion:'Review feedback',coffee:'Coffee break',chat:'Taking a break',delegated:'Delegated task',waiting:'Waiting for inputs'},ca:{plan:'Planificant',analyze:'Analitzant evidències',draft:'Redactant un esborrany',review:'Comprovant el resultat',coding:'Escrivint codi · demo',web:'Cercant al web · demo',revision:'Aplicant correccions',discussion:'Feedback de revisió',coffee:'Pausa per fer cafè',chat:'Fent una pausa',delegated:'Tasca delegada',waiting:'Esperant resultats'}};
const actionLabel = role => actionLabels[lang()][actions[role]] || t(states[role]);
function resetTeam(){roles.splice(0,roles.length,...baseRoles);selected=roles[0];homes.splice(4);agents.splice(4);for(const map of [metadata,bubbles,actions])for(const key of Object.keys(map))delete map[key];}
let selected = roles[0], controller = null, overall = 'idle', lastModel = '';
let states = Object.fromEntries(roles.map(role => [role, 'idle']));
let outputs = {}, activity = [], handoffWalk = null, demo = false, demoTimer = null;
const descriptions = {en:['Plans the task','Analyzes the information','Creates the deliverable','Checks and refines'],ca:['Planifica la tasca','Analitza la informació','Crea el resultat','Comprova i millora']};
const examples = {en:[['Compare options','Compare PostgreSQL and SQLite for an internal chat with 50 users. Include a recommendation and risks.'],['Analyze data','Analyze these monthly ticket counts: January 120, February 180, March 150. Calculate changes and propose a chart specification.'],['Draft a plan','Create a practical onboarding plan for a new employee, with tasks, owners and acceptance criteria.']],ca:[['Compara opcions','Compara PostgreSQL i SQLite per a un xat intern amb 50 usuaris. Inclou una recomanació i riscos.'],['Analitza dades','Analitza aquests recomptes mensuals de tiquets: gener 120, febrer 180, març 150. Calcula els canvis i proposa una especificació de gràfic.'],['Prepara un pla',"Crea un pla pràctic d’acollida per a una persona nova, amb tasques, responsables i criteris d’acceptació."]]};
const sceneHeight = 540;
const homes = [[165,210],[840,210],[165,465],[840,465]];
const spritePaths = roles.map(role=>`/app/assets/pixel-office/${role}.png`);
const canvas = $('teamCanvas'), ctx = canvas.getContext('2d');
const office = new Image(), agents = spritePaths.map(src=>{const image=new Image();image.src=src;image.onload=()=>draw();return image;});
office.src = '/app/assets/pixel-office/furniture.png';
office.onload = () => draw();
const newspaper = new Image();
newspaper.src = '/app/assets/pixel-office/newspaper.png';
newspaper.onload = () => draw();
let visible = false, raf = 0, lastFrame = 0;
const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)');

function render() {
  $('teamTitle').textContent = $('teamNav').textContent = t('title');
  $('teamTaskLabel').textContent = t('task');
  $('teamStart').textContent = t('start'); $('teamStop').textContent = t('stop');
  $('teamStart').disabled = !!controller; $('teamStop').disabled = !controller;
  $('teamTask').disabled = !!controller;
  $('teamStatus').textContent = `${demo ? (lang()==='ca'?'Demo local · ':'Local demo · ') : ''}${t(overall)}`;
  const active=roles.filter(role=>states[role]==='working');
  $('teamProgress').textContent = active.length ? `${lang()==='ca'?'Actius':'Active'}: ${active.map(name).join(' + ')} · ${roles.filter(role=>states[role]==='completed').length}/${roles.length}` : `${roles.filter(role=>states[role]==='completed').length}/${roles.length} · ${t(overall)}`;
  const sampleButtons=examples[lang()].map(([label,task])=>{const b=document.createElement('button');b.type='button';b.textContent=label;b.disabled=!!controller;b.onclick=()=>{$('teamTask').value=task;$('teamTask').focus();};return b;});
  const preview=document.createElement('button');preview.type='button';preview.textContent=lang()==='ca'?'▶ Demo sense API':'▶ Demo without API';preview.disabled=!!controller;preview.onclick=startDemo;
  $('teamExamples').replaceChildren(...sampleButtons,preview);
  $('teamModel').textContent = demo ? (lang()==='ca'?'Demo local · resultats de mostra':'Local demo · sample results') : `${t('textOnly')} / ${lastModel || t('model')}`;
  $('teamSelected').textContent = name(selected);
  $('teamAgentStatus').textContent = `${actionLabel(selected)}${metadata[selected]?.model?' · '+metadata[selected].model:''}${metadata[selected]?.parent?' · '+name(metadata[selected].parent):''}`;
  $('teamOutput').textContent = outputs[selected] || t('empty');
  $('teamActivityTitle').textContent = t('activity');
  $('teamAgentsLabel').textContent = t('agents');
  $('teamAgents').setAttribute('aria-label', t('select'));
  $('teamAgents').replaceChildren(...roles.map((role, i) => {
    const b = document.createElement('button'); b.type = 'button';
    b.setAttribute('aria-pressed', String(role === selected)); b.dataset.state = states[role];
    b.style.setProperty('--role-color',roleColor(role));
    const avatar = document.createElement('img'); avatar.src=agents[i].src; avatar.alt=''; avatar.className='team-agent-avatar';
    const copy = document.createElement('span'); copy.className='team-agent-copy'; copy.textContent=name(role);
    const status = document.createElement('small'); status.textContent=`${t(states[role])} · ${actionLabel(role)}${metadata[role]?.parent?' · '+name(metadata[role].parent):''}${metadata[role]?.model?' · '+metadata[role].model:''}`;
    b.append(avatar,copy,status); b.onclick=()=>{selected=role;render();}; return b;
  }));
  $('teamActivity').replaceChildren(...activity.slice().reverse().map(e => {
    const li=document.createElement('li');li.textContent=`${e.time}  ${t(e.type)}${e.agent?' / '+name(e.agent):''}${e.to?' → '+name(e.to):''}${e.message?' · '+e.message:''}`;return li;
  }));
  draw();
}

function smooth(v){return v*v*(3-2*v);}
function along(points,progress){
  const p=Math.max(0,Math.min(.9999,progress))*(points.length-1),i=Math.floor(p),f=smooth(p-i),a=points[i],b=points[i+1];
  return {x:a[0]+(b[0]-a[0])*f,y:a[1]+(b[1]-a[1])*f,moving:true};
}
function position(i,now){
  const role=roles[i],home=homes[i];
  if(handoffWalk?.agent===role){
    const progress=Math.min(1,(now-handoffWalk.start)/3200);
    if(progress<1)return along(handoffWalk.path,progress);
  }
  const stationary={x:home[0],y:home[1],moving:false};
  if(reducedMotion.matches||states[role]==='working'||states[role]==='waiting')return stationary;
  const phase=(now/1000+i*4.7)%24,coffee=[460+(i%3)*50,350+(i%2)*20];
  if(phase<3)return along([home,[512,home[1]],coffee],phase/3);
  if(phase<7)return {x:coffee[0],y:coffee[1],moving:false};
  if(phase<10)return along([coffee,[512,home[1]],home],(phase-7)/3);
  return stationary;
}
function drawAgent(i,now){
  const sprite=agents[i],p=position(i,now),scale=i<4?3.2:2.5,sw=sprite.naturalWidth,sh=sprite.naturalHeight;
  const w=sw*scale,h=sh*scale,bob=p.moving?Math.abs(Math.sin(now/105+i))*4:Math.sin(now/260+i)*1;
  const left=p.x-w/2,top=p.y-h+bob-(roles[i]==='builder'&&states[roles[i]]==='working'?28:0);
  if(!sprite.complete||!sprite.naturalWidth)return;
  if(states[roles[i]]==='working'){
    ctx.fillStyle='rgba(255,190,92,.34)';ctx.beginPath();ctx.ellipse(p.x,p.y-4,27,13,0,0,Math.PI*2);ctx.fill();
  }
  ctx.imageSmoothingEnabled=false;
  ctx.drawImage(sprite,left,top,w,h);
  if(states[roles[i]]==='working')drawRoleProp(roles[i],p,now);
  if((states[roles[i]]==='idle'||states[roles[i]]==='completed')&&(now/1000+i*4.7)%24<7&&office.naturalWidth)ctx.drawImage(office,168,92,8,12,p.x+14,p.y-48,16,24);
  if(selected===roles[i]){
    ctx.strokeStyle=states[roles[i]]==='working'?'#f3a64a':'#58bba3';ctx.lineWidth=2.5;
    ctx.beginPath();ctx.ellipse(p.x,p.y+2,20,7,0,0,Math.PI*2);ctx.stroke();
  }
  ctx.font='600 18px sans-serif';ctx.textAlign='center';const label=name(roles[i]);const lw=ctx.measureText(label).width+24;
  ctx.fillStyle=roleColor(roles[i]);ctx.fillRect(p.x-lw/2,p.y+13,lw,29);ctx.fillStyle='#ffffff';ctx.fillText(label,p.x,p.y+34);
  const phase=(now/1000+i*4.7)%24;
  const idle=states[roles[i]]==='idle'||states[roles[i]]==='completed'||states[roles[i]]==='cancelled';
  const message=bubbles[roles[i]] || (idle?(phase<7?actionLabels[lang()].coffee:actionLabels[lang()].chat):actionLabel(roles[i]));
  const lines=message.match(/.{1,27}(?:\s|$)|.{1,27}/g)?.slice(0,2)||[message];
  ctx.font='14px sans-serif';const bw=Math.min(210,Math.max(...lines.map(line=>ctx.measureText(line.trim()).width))+22),bh=lines.length*21+12;
  const bx=Math.max(6,Math.min(1018-bw,p.x-bw/2)),by=p.y-h-bh-12;
  ctx.fillStyle=actions[roles[i]]==='discussion'?'#fff0d9':'#ffffff';ctx.fillRect(bx,by,bw,bh);ctx.strokeStyle=roleColor(roles[i]);ctx.lineWidth=2;ctx.strokeRect(bx,by,bw,bh);ctx.fillStyle=roleColor(roles[i]);ctx.fillRect(bx,by,4,bh);ctx.beginPath();ctx.moveTo(p.x-5,by+bh);ctx.lineTo(p.x,by+bh+8);ctx.lineTo(p.x+5,by+bh);ctx.fill();ctx.fillStyle='#203e35';lines.forEach((line,j)=>ctx.fillText(line.trim(),bx+bw/2,by+23+j*21));
}
function drawRoleProp(role,p,now){
  const wave=reducedMotion.matches?0:Math.sin(now/210);
  if(role==='coordinator'){
    ctx.save();ctx.translate(p.x+24,p.y-47);ctx.rotate(-.6+wave*.25);
    ctx.fillStyle='#e2c79f';ctx.fillRect(-5,-3,10,9);
    ctx.fillStyle='#455044';ctx.fillRect(0,-9,6,12);
    ctx.fillStyle='#fff8db';ctx.fillRect(1,-48,4,39);ctx.restore();
  }else if(role==='researcher'&&actions[role]!=='web'&&newspaper.naturalWidth){
    ctx.save();ctx.translate(p.x,p.y-37);ctx.rotate(wave*.035);
    ctx.imageSmoothingEnabled=true;ctx.drawImage(newspaper,-30,-19,60,38);ctx.restore();
  }else if(role==='builder'){
    // The screen is in front of the sprite so the agent sits behind its PC.
    const x=p.x-43,y=p.y-66;
    ctx.fillStyle='#4a6162';ctx.fillRect(x-5,y-5,96,63);
    ctx.fillStyle='#0d2019';ctx.fillRect(x,y,86,48);
    ctx.textAlign='left';ctx.font='10px monospace';ctx.fillStyle='#86ff9c';
    const coding=actions[role]==='coding';
    const lines=coding?['const chart =','  build(data);','// review fixes','return chart;']:['> draft','... criteria','... response','> revise'];
    lines.forEach((line,j)=>ctx.fillText(line,x+5,y+11+j*11));
    ctx.fillStyle='#6e8582';ctx.fillRect(p.x-8,y+58,16,8);ctx.fillRect(p.x-25,y+64,50,5);
    ctx.fillStyle='#b4c9be';ctx.fillRect(p.x-43,p.y+3,86,9);
    ctx.fillStyle='#e2c79f';ctx.fillRect(p.x-32+(wave>0?3:0),p.y,13,6);ctx.fillRect(p.x+16+(wave<0?3:0),p.y,13,6);
  }else if(role==='reviewer'){
    ctx.fillStyle='#eadfc3';ctx.fillRect(p.x+23,p.y-59,25,36);
    ctx.fillStyle='#477666';for(let j=0;j<3;j++)ctx.fillRect(p.x+28,p.y-51+j*9,15,3);
  }
}
function background(now=0){
  ctx.fillStyle='#dae8df';ctx.fillRect(0,0,1024,540);
  ctx.fillStyle='#cadbd3';for(let y=0;y<540;y+=36)for(let x=0;x<1024;x+=36)if((x/36+y/36)%2===0)ctx.fillRect(x,y,36,36);
  ctx.fillStyle='#edf4ef';ctx.fillRect(0,0,1024,72);ctx.fillStyle='#748f87';ctx.fillRect(0,70,1024,7);
  ctx.imageSmoothingEnabled=false;
  const tile=(sx,sy,sw,sh,x,y,scale=3)=>{if(office.naturalWidth)ctx.drawImage(office,sx,sy,sw,sh,x,y,sw*scale,sh*scale);};
  tile(180,126,28,34,460,8,1.7);tile(168,64,16,20,45,14,2.5);tile(168,64,16,20,940,14,2.5);
  const colors=['#d18a50','#489fa6','#8374b3','#65965c'];
  homes.slice(0,4).forEach(([x,y],i)=>{ctx.fillStyle=colors[i];ctx.fillRect(x-100,y-145,200,5);tile(84,46,26,16,x-50,y-128,4);tile(232,104,24,28,x+20,y-138,2);});
  roles.forEach((role,i)=>{if(states[role]!=='working'||role==='builder')return;const [x,y]=homes[i],kind=actions[role];if(kind==='web'){
    ctx.fillStyle='#526b71';ctx.fillRect(x+42,y-134,114,79);ctx.fillStyle='#f4faf8';ctx.fillRect(x+46,y-130,106,69);ctx.fillStyle='#dce9e5';ctx.fillRect(x+49,y-126,100,14);ctx.textAlign='left';ctx.font='10px sans-serif';ctx.fillStyle='#25483e';ctx.fillText('Search · DEMO',x+52,y-115);ctx.fillStyle='#698baf';ctx.fillRect(x+52,y-104,85,3);ctx.fillRect(x+52,y-87,73,3);ctx.fillStyle='#b1c2b9';ctx.fillRect(x+52,y-98,92,2);ctx.fillRect(x+52,y-81,89,2);return;
  }if(role==='researcher'||role==='coordinator'||role==='reviewer')return;ctx.fillStyle='#102921';ctx.fillRect(x+48,y-133,68,46);ctx.textAlign='left';ctx.font='9px monospace';ctx.fillStyle='#83ff94';['> '+(kind||'draft'),'... context','... evidence','... response'].forEach((line,j)=>ctx.fillText(line.slice(0,12),x+51,y-122+j*10));});
  tile(120,64,32,18,455,385,3);ctx.fillStyle='#345b50';ctx.font='16px sans-serif';ctx.textAlign='center';ctx.fillText(lang()==='ca'?'Punt de trobada':'Meeting point',512,477);
}
function drawFrame(now){
  raf=0;if(!visible||document.hidden)return;
  if(!reducedMotion.matches&&now-lastFrame<32){raf=requestAnimationFrame(drawFrame);return;}
  lastFrame=now;
  const box=canvas.getBoundingClientRect();if(!box.width)return;
  const dpr=Math.min(devicePixelRatio||1,1.5),w=Math.round(box.width*dpr),h=Math.round(box.height*dpr);
  if(canvas.width!==w||canvas.height!==h){canvas.width=w;canvas.height=h;}
  ctx.setTransform(w/1024,0,0,h/sceneHeight,0,0);ctx.clearRect(0,0,1024,sceneHeight);
  background(now);
  if(handoffWalk&&now-handoffWalk.start>3200)handoffWalk=null;
  if(handoffWalk){ctx.strokeStyle='rgba(247,171,91,.88)';ctx.lineWidth=4;ctx.setLineDash([9,8]);ctx.beginPath();handoffWalk.path.forEach(([x,y],i)=>i?ctx.lineTo(x,y):ctx.moveTo(x,y));ctx.stroke();ctx.setLineDash([]);}
  for(let i=0;i<roles.length;i++)drawAgent(i,now);
  if(!reducedMotion.matches)raf=requestAnimationFrame(drawFrame);
}
function draw(){
  if(visible&&!raf)raf=requestAnimationFrame(drawFrame);
  else if(!visible){
    const box=canvas.getBoundingClientRect();if(!box.width)return;
    const dpr=Math.min(devicePixelRatio||1,1.5);canvas.width=Math.round(box.width*dpr);canvas.height=Math.round(box.height*dpr);
    ctx.setTransform(canvas.width/1024,0,0,canvas.height/sceneHeight,0,0);ctx.clearRect(0,0,1024,sceneHeight);
    background();
    for(let i=0;i<roles.length;i++)drawAgent(i,0);
  }
}
function receive(e){
  if(e.type==='spawn'&&e.agent&&!roles.includes(e.agent)&&roles.includes(e.parent)&&roles.length<8){roles.push(e.agent);const i=roles.length-5;homes.push([390+(i%2)*235,230+Math.floor(i/2)*210]);agents.push(agents[roles.indexOf(e.parent)%4]);states[e.agent]='waiting';metadata[e.agent]={parent:e.parent,name:e.name||e.agent,model:e.model};}
  if(e.agent&&!roles.includes(e.agent))return;
  if(e.agent){if(e.activity&&['working','discussion','spawn'].includes(e.type))actions[e.agent]=e.activity;if(e.message)bubbles[e.agent]=e.message;if(e.model)metadata[e.agent]={...metadata[e.agent],model:e.model};}
  if(e.type==='working'){states[e.agent]='working';overall='working';selected=e.agent;}
  if(e.type==='completed'){states[e.agent]='completed';outputs[e.agent]=e.output||'';delete bubbles[e.agent];delete actions[e.agent];}
  if(e.type==='discussion'){actions[e.agent]='discussion';outputs[e.agent]=e.message||outputs[e.agent];if(e.to&&roles.includes(e.to))bubbles[e.to]=lang()==='ca'?'Rebo feedback del revisor':'Receiving review feedback';}
  if(e.type==='handoff'&&roles.includes(e.to)){
    const from=homes[roles.indexOf(e.agent)],to=homes[roles.indexOf(e.to)];
    handoffWalk={agent:e.agent,start:performance.now(),path:[from,[512,from[1]],[512,to[1]],to]};
  }
  if(['done','failed','cancelled'].includes(e.type)){
    overall=e.type;
    for(const role of roles)if(states[role]==='working'||states[role]==='waiting')states[role]=e.type==='done'?'completed':'cancelled';
    if(e.type==='done')selected='reviewer';
    for(const role of roles){delete bubbles[role];delete actions[role];}
  }
  activity.push({...e,time:new Date().toLocaleTimeString([],{hour:'2-digit',minute:'2-digit',second:'2-digit'})});
  if(activity.length>40)activity.shift();render();
}

$('teamForm').onsubmit=async event=>{
  event.preventDefault();if(controller)return;
  const task=$('teamTask').value.trim();if(!task)return;
  demo=false;controller=new AbortController();const current=controller;
  resetTeam();outputs={};activity=[];handoffWalk=null;overall='started';
  states=Object.fromEntries(roles.map(role=>[role,'waiting']));lastModel=window.gregal.state?.model||'';render();
  let reader;
  try{
    const response=await window.gregal.api('/api/v2/team/run',{method:'POST',signal:current.signal,body:JSON.stringify({task,lang:lang()})});
    if(!response.ok||!response.body)throw new Error('request failed');
    reader=response.body.getReader();const decoder=new TextDecoder();let buffer='';
    while(true){const chunk=await reader.read();if(chunk.done)break;
      buffer+=decoder.decode(chunk.value,{stream:true});let end;
      while((end=buffer.indexOf('\n\n'))>=0){const frame=buffer.slice(0,end);buffer=buffer.slice(end+2);if(frame.startsWith('event: team\n')){const data=frame.split('\n').find(line=>line.startsWith('data: '));if(data)receive(JSON.parse(data.slice(6)));}}
      if(buffer.length>1000000)throw new Error('oversized event');
    }
    if(!['done','failed','cancelled'].includes(overall))receive({type:'failed'});
  }catch(error){receive({type:current.signal.aborted?'cancelled':'failed'});}
  finally{if(reader){try{await reader.cancel();}catch{}reader.releaseLock();}controller=null;render();}
};
function startDemo(){
  if(controller)return;resetTeam();demo=true;controller=new AbortController();outputs={};activity=[];handoffWalk=null;overall='started';lastModel='';states=Object.fromEntries(roles.map(role=>[role,'waiting']));
  for(const role of roles)metadata[role]={model:'Demo'};
  const ca=lang()==='ca',say=(en,cat)=>ca?cat:en;
  $('teamTask').value=say('Demo: build a ticket dashboard with parallel research, delegated checks and a review correction.','Demo: crea un tauler de tiquets amb recerca paral·lela, comprovacions delegades i una correcció del revisor.');
  const events=[
    [0,{type:'working',agent:'coordinator',activity:'plan',message:say('Research + prototype in parallel','Recerca + prototip en paral·lel')}],
    [2500,{type:'completed',agent:'coordinator',output:say('Plan: investigate chart choices and build an independent prototype. Merge evidence during review.','Pla: investigar opcions de gràfic i crear un prototip independent. Unir evidències durant la revisió.')}],
    [2500,{type:'working',agent:'researcher',activity:'web',message:say('Comparing chart sources','Comparo fonts de gràfics')}],
    [2500,{type:'working',agent:'builder',activity:'coding',message:say('Building the dashboard','Creo el tauler')}],
    [4500,{type:'spawn',agent:'check-data',parent:'builder',name:say('Data check','Comprova dades'),model:'Demo'}],
    [4500,{type:'working',agent:'check-data',activity:'analyze',message:say('Checking 120 + 180 + 150','Comprovo 120 + 180 + 150')}],
    [4500,{type:'spawn',agent:'check-ui',parent:'builder',name:say('UI check','Comprova UI'),model:'Demo'}],
    [4500,{type:'working',agent:'check-ui',activity:'review',message:say('Checking chart labels','Comprovo les etiquetes')}],
    [8500,{type:'completed',agent:'check-data',output:'120 + 180 + 150 = 450.'}],
    [8500,{type:'completed',agent:'check-ui',output:say('Monthly labels are present. A zero baseline is required.','Hi ha etiquetes mensuals. Cal una base zero.')}],
    [9500,{type:'completed',agent:'researcher',output:say('Demo finding: use a bar chart with a zero baseline. No real web request was made.','Resultat de mostra: gràfic de barres amb base zero. No s’ha fet cap petició web real.')}],
    [10500,{type:'completed',agent:'builder',output:say('Demo prototype: January 120, February 180, March 150. Axis starts at 100.','Prototip de mostra: gener 120, febrer 180, març 150. L’eix comença a 100.')}],
    [11000,{type:'working',agent:'reviewer',activity:'review',message:say('Checking against the plan','Comprovo els criteris')}],
    [14000,{type:'discussion',agent:'reviewer',to:'builder',message:say('The axis exaggerates differences','L’eix exagera les diferències')}],
    [15500,{type:'working',agent:'builder',activity:'revision',message:say('Agreed. Setting baseline to 0','D’acord. Poso la base a 0')}],
    [18500,{type:'completed',agent:'builder',output:say('Corrected demo prototype: zero baseline, three monthly bars, total 450.','Prototip de mostra corregit: base zero, tres barres mensuals i total 450.')}],
    [19000,{type:'working',agent:'reviewer',activity:'review',message:say('Verifying the correction','Verifico la correcció')}],
    [22000,{type:'completed',agent:'reviewer',output:say('Demo approved: 450 tickets. Zero baseline verified. All research, code and discussion in this demo are scripted examples, not executed tools.','Demo aprovada: 450 tiquets. Base zero comprovada. La recerca, el codi i la discussió són exemples preparats, no eines executades.')}],
    [22500,{type:'done'}],
  ];let index=0,previous=0;
  const next=()=>{const [at,event]=events[index++];receive(event);previous=at;if(index<events.length)demoTimer=setTimeout(next,events[index][0]-previous);else{controller=null;demoTimer=null;render();}};render();next();
}
$('teamStop').onclick=()=>{if(demo&&controller){clearTimeout(demoTimer);demoTimer=null;controller.abort();controller=null;receive({type:'cancelled'});}else controller?.abort();};
window.addEventListener('pagehide',()=>{clearTimeout(demoTimer);controller?.abort();});
canvas.addEventListener('click',event=>{const box=canvas.getBoundingClientRect(),x=(event.clientX-box.left)*1024/box.width,y=(event.clientY-box.top)*sceneHeight/box.height;const i=roles.findIndex((role,index)=>{const p=position(index,performance.now());return Math.abs(x-p.x)<45&&y>p.y-100&&y<p.y+45;});if(i>=0){selected=roles[i];render();}});
document.addEventListener('gregal:idioma',render);
new ResizeObserver(draw).observe(canvas);
const pageObserver=new IntersectionObserver(entries=>{
  visible=entries[0]?.isIntersecting===true;
  if(visible)draw();else if(raf){cancelAnimationFrame(raf);raf=0;}
});
pageObserver.observe($('teamPage'));
document.addEventListener('visibilitychange',()=>{if(document.hidden&&raf){cancelAnimationFrame(raf);raf=0;}else if(visible)draw();});
reducedMotion.addEventListener?.('change',draw);
render();
