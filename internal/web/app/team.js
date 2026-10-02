const $ = id => document.getElementById(id);
const roles = ['coordinator', 'researcher', 'builder', 'reviewer'];
const words = {
  en: { title:'Team', task:'Task', start:'Start', stop:'Stop', idle:'Idle', working:'Working', completed:'Completed', cancelled:'Stopped', failed:'Run failed. Check the model connection and retry.', done:'Complete', started:'Started', handoff:'Handoff', activity:'Activity', coordinator:'Coordinator', researcher:'Researcher', builder:'Builder', reviewer:'Reviewer', waiting:'Waiting', empty:'No deliverable yet', interrupted:'Connection interrupted. Retry the task.', textOnly:'Text only', model:'Active model' },
  ca: { title:'Equip', task:'Tasca', start:'Inicia', stop:'Atura', idle:'Inactiu', working:'Treballant', completed:'Completat', cancelled:'Aturat', failed:'Ha fallat. Comprova la connexi\u00f3 del model i torna-ho a provar.', done:'Completat', started:'Iniciat', handoff:'Trasp\u00e0s', activity:'Activitat', coordinator:'Coordinador', researcher:'Investigador', builder:'Constructor', reviewer:'Revisor', waiting:'En espera', empty:'Encara no hi ha cap resultat', interrupted:'Connexi\u00f3 interrompuda. Torna a iniciar la tasca.', textOnly:'Nom\u00e9s text', model:'Model actiu' },
};
const lang = () => document.documentElement.lang === 'ca' ? 'ca' : 'en';
const t = key => words[lang()][key] || key;
let selected = roles[0], controller = null, overall = 'idle', lastModel = '';
let states = Object.fromEntries(roles.map(role => [role, 'idle']));
let outputs = {}, activity = [], handoff = null;
const positions = [[27,38],[70,38],[27,73],[70,73]];

function render() {
  $('teamTitle').textContent = $('teamNav').textContent = t('title');
  $('teamTaskLabel').textContent = t('task');
  $('teamStart').textContent = t('start'); $('teamStop').textContent = t('stop');
  $('teamStart').disabled = !!controller; $('teamStop').disabled = !controller;
  $('teamTask').disabled = !!controller;
  $('teamStatus').textContent = t(overall);
  $('teamModel').textContent = `${t('textOnly')} / ${lastModel || t('model')}`;
  $('teamSelected').textContent = t(selected);
  $('teamAgentStatus').textContent = t(states[selected]);
  $('teamOutput').textContent = outputs[selected] || t('empty');
  $('teamActivityTitle').textContent = t('activity');
  $('teamAgents').replaceChildren(...roles.map((role, i) => {
    const b = document.createElement('button'); b.type = 'button';
    b.style.left = positions[i][0]+'%'; b.style.top = positions[i][1]+'%';
    b.setAttribute('aria-pressed', String(role === selected)); b.dataset.state = states[role];
    b.append(document.createTextNode(t(role)));
    const status = document.createElement('small'); status.textContent = t(states[role]); b.append(status);
    b.onclick = () => { selected = role; render(); }; return b;
  }));
  $('teamActivity').replaceChildren(...activity.slice().reverse().map(e => {
    const li = document.createElement('li'); li.textContent = `${e.time}  ${t(e.type)}${e.agent ? ' / '+t(e.agent) : ''}${e.to ? ' → '+t(e.to) : ''}`; return li;
  }));
  draw();
}

function draw() {
  const canvas = $('teamCanvas'), box = canvas.getBoundingClientRect();
  if (!box.width) return;
  const dpr = Math.min(devicePixelRatio || 1, 2);
  canvas.width = Math.round(box.width*dpr); canvas.height = Math.round(box.height*dpr);
  const c = canvas.getContext('2d'); c.scale(canvas.width/800, canvas.height/540);
  const poly = (points, color) => { c.fillStyle=color; c.beginPath(); points.forEach(([x,y],i)=>i?c.lineTo(x,y):c.moveTo(x,y)); c.closePath(); c.fill(); };
  c.fillStyle='#172524'; c.fillRect(0,0,800,540);
  poly([[45,160],[390,28],[755,160],[755,396],[410,525],[45,396]],'#dee8e3');
  poly([[45,160],[390,28],[390,117],[45,250]],'#7faba5');
  poly([[390,28],[755,160],[755,250],[390,117]],'#aac9c0');
  c.strokeStyle='#baccc3'; c.lineWidth=1;
  for(let i=0;i<8;i++){ c.beginPath();c.moveTo(65+i*90,260);c.lineTo(65+i*90,430);c.stroke(); }
  // Original canvas furniture and avatars; positions match accessible agent controls.
  for(let i=0;i<4;i++){
    const x=positions[i][0]*8, y=positions[i][1]*5.4-45;
    poly([[x-70,y],[x,y-25],[x+70,y],[x,y+25]],'#ffffff');
    poly([[x-70,y],[x,y+25],[x,y+39],[x-70,y+14]],'#b9c4c0');
    poly([[x,y+25],[x+70,y],[x+70,y+14],[x,y+39]],'#8caaa0');
    c.fillStyle='#253b3c'; c.fillRect(x-18,y-28,36,25);
    c.fillStyle=states[roles[i]]==='working'?'#f3bd67':'#5dc3af';c.fillRect(x-14,y-24,28,16);
    c.fillStyle=['#367d9a','#bf697f','#589b76','#927abc'][i];c.fillRect(x+37,y-25,24,28);
    c.fillStyle='#e6b397';c.beginPath();c.arc(x+49,y-31,11,0,Math.PI*2);c.fill();
    c.fillStyle='#304244';c.fillRect(x+39,y-41,20,7);
  }
  // Conference table, windows and plants establish the office without obscuring workstations.
  poly([[340,310],[410,284],[475,310],[410,336]],'#90aaa3');
  for(const x of [480,570,660]){ c.fillStyle='#eaf7f5';c.fillRect(x,115+(x-480)*0.35,44,35); }
  for(const [x,y] of [[96,335],[691,356]]){c.fillStyle='#a96155';c.fillRect(x-12,y,24,20);c.fillStyle='#418966';c.beginPath();c.ellipse(x,y-12,19,28,0,0,Math.PI*2);c.fill();}
  if(handoff){const a=positions[roles.indexOf(handoff.agent)], b=positions[roles.indexOf(handoff.to)];if(a&&b){c.strokeStyle='#dc8b44';c.lineWidth=3;c.setLineDash([7,6]);c.beginPath();c.moveTo(a[0]*8,a[1]*5.4);c.lineTo(b[0]*8,b[1]*5.4);c.stroke();c.setLineDash([]);}}
}

function receive(e) {
  if (e.agent && !roles.includes(e.agent)) return;
  if (e.type === 'working') { states[e.agent]='working'; overall='working'; }
  if (e.type === 'completed') { states[e.agent]='completed'; outputs[e.agent]=e.output || ''; }
  if (e.type === 'handoff' && roles.includes(e.to)) handoff=e;
  if (['done','failed','cancelled'].includes(e.type)) {
    overall=e.type;
    for(const role of roles) if(states[role]==='working'||states[role]==='waiting') states[role]=e.type==='done'?'completed':'cancelled';
    if(e.type==='done') selected='reviewer';
  }
  activity.push({...e, time:new Date().toLocaleTimeString([], {hour:'2-digit',minute:'2-digit',second:'2-digit'})});
  if(activity.length>40) activity.shift(); render();
}

$('teamForm').onsubmit = async event => {
  event.preventDefault(); if(controller) return;
  const task=$('teamTask').value.trim(); if(!task) return;
  controller=new AbortController(); const current=controller;
  outputs={}; activity=[]; handoff=null; overall='started';
  states=Object.fromEntries(roles.map(role=>[role,'waiting']));
  lastModel=window.gregal.state?.model || ''; render();
  let reader;
  try {
    const response=await window.gregal.api('/api/v2/team/run', {method:'POST',signal:current.signal,body:JSON.stringify({task,lang:lang()})});
    if(!response.ok || !response.body) throw new Error('request failed');
    reader=response.body.getReader();const decoder=new TextDecoder();let buffer='';
    while(true){const chunk=await reader.read(); if(chunk.done) break;
      buffer+=decoder.decode(chunk.value,{stream:true});
      let end;while((end=buffer.indexOf('\n\n'))>=0){const frame=buffer.slice(0,end);buffer=buffer.slice(end+2);if(frame.startsWith('event: team\n')){const data=frame.split('\n').find(line=>line.startsWith('data: '));if(data)receive(JSON.parse(data.slice(6)));}}
      if(buffer.length>1000000) throw new Error('oversized event');
    }
    if(!['done','failed','cancelled'].includes(overall)) receive({type:'failed'});
  } catch(error){ receive({type:current.signal.aborted?'cancelled':'failed'}); }
  finally { if(reader) { try{await reader.cancel();}catch{} reader.releaseLock(); } controller=null;render(); }
};
$('teamStop').onclick=()=>controller?.abort();
window.addEventListener('pagehide',()=>controller?.abort());
document.addEventListener('gregal:idioma',render);
new ResizeObserver(draw).observe($('teamCanvas'));
render();
