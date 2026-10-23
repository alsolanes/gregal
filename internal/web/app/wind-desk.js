// The desk is a view onto the real team runner. No simulated progress or work.
(function (root) {
  'use strict';
  const $ = id => root.document.getElementById(id);
  const make = (tag, cls, text) => {
    const el = root.document.createElement(tag);
    if (cls) el.className = cls;
    if (text !== undefined) el.textContent = text;
    return el;
  };
  const interpolate = (value, vars) => String(value).replace(/\{([\w]+)\}/g, (_, key) => vars?.[key] ?? `{${key}}`);
  function localized(key, fallback, vars) { return root.gregalWindI18n?.text?.(key, fallback, vars); }
  function tr(key, fallback, vars) {
    const value=localized(key,fallback,vars);
    if(value===undefined) return interpolate(fallback,vars);
    if(typeof value==='string') return value;
    return interpolate(String(value),vars);
  }
  function paint(el,key,fallback,vars) {
    if(root.gregalWindI18n?.paint) root.gregalWindI18n.paint(el,localized(key,fallback,vars) ?? fallback);
    else el.textContent=tr(key,fallback,vars);
  }
  function message(el, value) {
    if(root.gregalWindI18n?.paint) root.gregalWindI18n.paint(el,value);
    else el.textContent=String(value || '');
  }
  function failure(key, fallback) {
    return root.gregalWindI18n?.error?.(key,fallback) || new Error(tr(key,fallback));
  }
  const names = () => ({ idle:tr("winds.byYourSide", "Al teu costat"), waiting:tr("winds.waiting", "En espera"), started:tr("winds.preparingTheWay", "Preparant el camí"), working:tr("winds.inProgress", "En marxa"), completed:tr("winds.workCompleted", "Feina enllestida"), done:tr("winds.taskCompleted", "Encàrrec acabat"), failed:tr("winds.somethingNeedsAttention", "Cal revisar un entrebanc"), cancelled:tr("winds.taskStopped", "Encàrrec aturat") });
  const statusKeys={idle:'winds.byYourSide',waiting:'winds.waiting',started:'winds.preparingTheWay',working:'winds.inProgress',completed:'winds.workCompleted',done:'winds.taskCompleted',failed:'winds.somethingNeedsAttention',cancelled:'winds.taskStopped'};
  function instruction(wind, review) {
    return `You are ${wind.name}. Specialty: ${wind.role || 'helping with this project'}. Character: ${wind.persona || 'kind, clear and practical'}.\n` +
      (review ? 'Review the actual deliverable against the request. Identify concrete defects, verify with available read-only tools, and return the required review verdict. Never claim checks you did not perform.' : 'Complete the requested work using the available tools and project context. Produce a concrete deliverable. Distinguish completed work from proposals and explain any limitation. Never claim actions you did not perform.');
  }
  function flowFor(owner, reviewer, kind = 'build') {
    if (!owner?.id) throw failure("winds.chooseAWindForThisTask", "Tria un vent per a l’encàrrec.");
    if (reviewer && reviewer.id === owner.id) throw failure("winds.chooseADifferentWindToReviewTheWork", "Tria un altre vent per revisar la feina.");
    const flow = { name:tr('winds.taskBy','Encàrrec de {name}',{name:owner.name}), nodes:[{id:'builder',label:owner.name,role:kind === 'research' ? 'researcher' : 'builder',mode:'task',instruction:instruction(owner,false)}],edges:[],output:'builder',max_rounds:1 };
    if (reviewer) {
      flow.nodes.push({id:'reviewer',label:reviewer.name,role:'reviewer',mode:'review',instruction:instruction(reviewer,true)});
      flow.edges.push({from:'builder',to:'reviewer',kind:'data'},{from:'reviewer',to:'builder',kind:'revision'});
      flow.output='reviewer';
    }
    flow.nodes[0].wind=owner.id;
    if(reviewer) flow.nodes[1].wind=reviewer.id;
    return flow;
  }
  // Local routing keeps the common path instant: no planning model call.
  function resolveTask(roster, task, options={}) {
    const text=task.normalize('NFD').replace(/[\u0300-\u036f]/g,'').toLowerCase();
    const build=/\b(crea|crear|create|build|make|fes|fer|fix|arregla|implement|modifica|edita|desenvolupa|programa)\b/.test(text);
    const research=/\b(cerca|busca|investiga|research|search|find|compare|compara|analitza|analyze|resum|resumeix|summari[sz]e)\b/.test(text);
    const dataWork=roster.some(w=>w.squad==='data') && /dades|data|csv|regress|machine learning|model|series|temporal/.test(text);
    const kind=options.kind && options.kind!=='auto' ? options.kind : research&&!build&&!dataWork ? 'research' : 'build';
    const code=/\b(web|website|pagina|landing|html|css|javascript|codi|code|app|bug|joc|game)\b/.test(text);
    const preferred=roster.find(w=>kind==='research' ? /marinada|research|cerc|investig/i.test(w.name+' '+w.role) : code ? /tramuntana|codi|code|web/i.test(w.name+' '+w.role) : /gregal|organitz|organis|organiz/i.test(w.name+' '+w.role));
    const specialties=[['timeseries',/series|temporal|forecast|predic/],['regression',/regress|estadistic|statistic/],['ml',/machine learning|classif|cluster|entrena|train/],['security',/seguretat|security|vulnerab/],['backend',/backend|api|database|base de dades/],['frontend',/frontend|web|html|css|interficie/],['shopping',/compra|product|compar|buy/],['books',/llibre|book|lectura/],['sports',/esport|sport|futbol|basket/],['music',/music|musica|concert/]];
    const specialist=specialties.find(([,match])=>match.test(text));
    const owner=roster.find(w=>w.id===options.owner) || (specialist&&roster.find(w=>w.specialty===specialist[0])) || preferred || roster[0];
    const candidates=roster.filter(w=>w.id!==owner?.id);
    const reviewer=options.reviewer==='' ? null : options.reviewer && options.reviewer!=='auto' ? candidates.find(w=>w.id===options.reviewer) : kind==='build'&&(code||dataWork) ? candidates.find(w=>/mestral|revis|quality|qualitat/i.test(w.name+' '+w.role)) || candidates[0] : null;
    return {owner,reviewer:reviewer||null,kind};
  }
  let winds = [], face, bound = false, preparing = false, snapshot = null, owned = false;
  let rosterKey = '', choiceKey = '', stepOrder = '', eventKey = '', choicesInitialized=false;
  const stepRows = new Map();
  function contextHint() {
    const hint=$('windWorkContext'); if(!hint) return;
    const path=root.gregal?.state?.cwd;
    if(path) paint(hint,'winds.filesSavedIn','Els fitxers es desen a {path}.',{path});
    else paint(hint,'winds.readyToHelp','Explica què vols. Els vents es reparteixen la feina i treballen al projecte obert.');
  }
  function presence(next) {
    if(!owned || !next || next.source==='workshop') return;
    const labels=names();
    for(const card of $('windDeskRoster').querySelectorAll('[data-wind]')) {
      const w=winds.find(w=>w.id===card.dataset.wind);
      const participant=next.nodes.find(n=>n.name===w?.name);
      const badge=card.querySelector('.wind-desk-presence');
      const activityLabels={read:tr('winds.readingFiles','Llegint fitxers'),glob:tr('winds.searchingFiles','Cercant fitxers'),grep:tr('winds.searchingFiles','Cercant fitxers'),write:tr('winds.writingFiles','Escrivint fitxers'),edit:tr('winds.writingFiles','Escrivint fitxers'),patch:tr('winds.writingFiles','Escrivint fitxers'),bash:tr('winds.checkingProject','Comprovant el projecte'),web_search:tr('winds.searchingSources','Cercant fonts'),web_fetch:tr('winds.readingSources','Llegint fonts'),reviewing:tr('winds.reviewingAction','Revisant una acció'),review:tr('winds.reviewingAction','Revisant una acció')};
      const stateLabel=participant ? labels[participant.state] || participant.state : '';
      const activity=participant ? activityLabels[participant.activity] : '';
      const label=stateLabel ? [stateLabel,activity].filter(Boolean).join(' · ') : '';
      badge.hidden=!participant;
      if(label && badge.textContent!==label) message(badge,label);
      card.dataset.work=participant?.state || '';
    }
  }
  function choices() {
    const owner=$('windWorkOwner'), reviewer=$('windWorkReviewer');
    const oldOwner=owner.value, oldReviewer=reviewer.value;
    const nextChoiceKey=JSON.stringify([winds.map(w=>[w.id,w.name]),oldOwner]);
    if(nextChoiceKey !== choiceKey) {
    const automatic=new Option('', ''); paint(automatic,'winds.automatic','Automàtic'); owner.replaceChildren(automatic);
    const autoReview=new Option('', 'auto'); paint(autoReview,'winds.automatic','Automàtic');
    const none=new Option('', ''); paint(none,'winds.noReview','Sense revisió'); reviewer.replaceChildren(autoReview,none);
    for (const w of winds) {
      const ownerOption=new Option(w.name,w.id); owner.append(ownerOption);
      if (w.id !== oldOwner) reviewer.append(new Option(w.name,w.id));
    }
    if(winds.some(w=>w.id===oldOwner)) owner.value=oldOwner;
    if(oldReviewer !== owner.value && winds.some(w=>w.id===oldReviewer)) reviewer.value=oldReviewer;
    else reviewer.value=oldReviewer==='' && choicesInitialized ? '' : 'auto';
    choiceKey=JSON.stringify([winds.map(w=>[w.id,w.name]),owner.value]);
    choicesInitialized=true;
    }
    for(const card of $('windDeskRoster').querySelectorAll('[data-wind]')) {
      const selected=card.dataset.wind===owner.value;
      card.dataset.selected=String(selected);
      card.querySelector('button')?.setAttribute('aria-pressed',String(selected));
    }
    $('windWorkStart').disabled=preparing || !!snapshot?.busy;
  }
  function render(windList, drawFace) {
    if(!$('windWorkForm')) return;
    winds=windList; face=drawFace;
    contextHint();
    const nextRosterKey=JSON.stringify(winds.map(w=>[w.id,w.name,w.role,w.color,w.style]));
    if(nextRosterKey === rosterKey && bound) { choices(); return; }
    rosterKey=nextRosterKey;
    const roster=$('windDeskRoster'); roster.replaceChildren();
    for(const [index,w] of winds.entries()) {
      const card=make('article','wind-desk-card'); card.style.setProperty('--wind-tint',w.color && /^#[\da-f]{6}$/i.test(w.color) ? w.color : '#C1B4DE');
      card.dataset.number=String(index+1).padStart(2,'0');
      const badge=make('span','wind-desk-presence'); badge.hidden=true; card.append(badge);
      const avatar=make('div','wind-desk-avatar'); avatar.append(face(w,true));
      const copy=make('div','wind-desk-copy'); const heading=make('h2','',w.name), role=make('p','',w.role || ''); if(!w.role) paint(role,'winds.aHelpingHandForYourDay','Una mà per al teu dia a dia'); copy.append(heading,role);
      const action=make('button','ghost'); paint(action,'winds.giveThemATask','Encarrega-li una cosa ↗'); action.type='button';
      action.onclick=()=>{ $('windWorkOwner').value=w.id; choices(); $('windWorkTask').focus(); $('windWorkForm').scrollIntoView({block:'nearest'}); };
      card.dataset.wind=w.id;
      card.append(avatar,copy,action); roster.append(card);
    }
    if(!winds.length) { const empty=make('p','wind-desk-empty'); paint(empty,'winds.starterCrew','La teva colla es prepara amb el primer encàrrec. Escriu què vols fer i prem Som-hi.'); roster.append(empty); }
    choices();
    presence(snapshot);
    if(!bound) bind();
  }
  function update(next) {
    snapshot=next;
    if(!$('windWorkStart')) return;
    $('windWorkStart').disabled=preparing || next.busy;
    if(!owned || next.source === 'workshop') return;
    presence(next);
    const progress=$('windWorkProgress'); progress.hidden=false;
    if($('windWorkTitle').textContent!==next.task) $('windWorkTitle').textContent=next.task;
    const labels=names();
    const summaryKey=next.demo?'winds.workshopDemoNoRealTaskIsRunning':(statusKeys[next.state]||'');
    const summaryLabel=next.demo ? tr(summaryKey,'Demostració del taller; no s’està executant cap encàrrec real.') : (labels[next.state] || next.state);
    if($('windWorkSummary').textContent!==summaryLabel) paint($('windWorkSummary'),summaryKey,summaryLabel);
    $('windWorkStop').hidden=!next.busy;
    const log=$('windWorkProgress').querySelector('.wind-work-log');
    if(log && next.busy) log.open=true;
    const steps=$('windWorkSteps');
    for(const n of next.nodes) {
      let row=stepRows.get(n.id);
      if(!row) {
      const details=make('details','wind-work-step'); details.dataset.role=n.id;
      const summary=make('summary');
      const avatar=make('span','wind-work-avatar'); summary.append(avatar);
      const title=make('strong','',n.name), state=make('span','wind-work-state'); summary.append(title,state);
      const output=make('div','wind-work-output');
      details.append(summary,output);
        const preview=make('button','ghost wind-work-preview'); paint(preview,'winds.openTheResult','Obre el resultat ↗'); preview.type='button';
        details.append(preview);
      row={details,title,state,output,preview,avatar};
      preview.onclick=()=>root.gregalWindRunner?.openResult(row.outputText,row.name,row.artifactPath);
      stepRows.set(n.id,row);
      }
      if(row.name!==n.name) { row.title.textContent=n.name; row.name=n.name; }
      const w=winds.find(w=>w.name===n.name);
      const avatarKey=w ? JSON.stringify([w.id,w.color,w.style]) : '';
      if(row.avatarKey!==avatarKey) {
        row.avatar.replaceChildren(...(w && face ? [face(w)] : []));
        row.avatar.hidden=!w;
        row.avatarKey=avatarKey;
      }
      const label=labels[n.state] || n.state;
      if(row.stateLabel!==label) { paint(row.state,statusKeys[n.state]||'',label); row.stateLabel=label; }
      row.details.dataset.state=n.state;
      const outputText=n.output || '';
      if(row.outputText!==outputText) {
        if(row.outputText && !outputText) row.details.open=false;
        if(outputText) message(row.output,outputText);
        else paint(row.output,'winds.theResultWillAppearHereWhenThisStep','El resultat apareixerà aquí quan acabi aquest pas.');
        row.outputText=outputText;
      }
      if(!next.busy && n.output && row.wasBusy!==false) row.details.open=true;
      row.wasBusy=next.busy;
      row.preview.hidden=!n.output || !!next.demo;
    }
    const order=JSON.stringify(next.nodes.map(n=>n.id));
    if(order!==stepOrder) {
      steps.replaceChildren(...next.nodes.map(n=>stepRows.get(n.id).details));
      const ids=new Set(next.nodes.map(n=>n.id));
      for(const id of stepRows.keys()) if(!ids.has(id)) stepRows.delete(id);
      stepOrder=order;
    }
    const jump=$('windWorkJump');
    if(jump) { jump.hidden=false; jump.dataset.busy=String(next.busy); }
    const nextEventKey=JSON.stringify([next.events,labels]);
    if(nextEventKey===eventKey) return;
    eventKey=nextEventKey;
    const events=$('windWorkEvents'); events.replaceChildren();
    for(const e of next.events) {
      const speaker=next.nodes.find(n=>n.id===e.agent)?.name;
      const message=e.message || names()[e.type] || e.type;
      const item=make('li','',[e.time,speaker,e.activity,e.type==='tool_result'?tr('winds.toolFinished','fet'):null,message].filter(Boolean).join(' · '));
      if(e.path) {
        const row=stepRows.get(e.agent);
        if(row) row.artifactPath=e.path;
        const open=make('button','ghost wind-work-event-open',tr('winds.openWebsite','Obre la web ↗'));
        open.type='button'; open.onclick=()=>root.gregalWindRunner?.openResult(row?.outputText||'',speaker||e.agent,e.path);
        item.append(open);
      }
      events.append(item);
    }
  }
  async function submit(event) {
    event.preventDefault();
    if(preparing || snapshot?.busy) return;
    const task=$('windWorkTask').value.trim();
    if(!task) return;
    const feedback=$('windWorkStatus');
    try {
      if(!root.gregalWindRunner) throw failure("winds.theWorkEngineIsNotReadyYetTry", "El motor de treball encara no està preparat. Torna-ho a provar d’aquí a un moment.");
      preparing=true; $('windWorkStart').disabled=true;
      if(!winds.length) await root.gregalWindSetup?.ensureReady();
      const {owner,reviewer,kind}=resolveTask(winds,task,{kind:$('windWorkKind').value,owner:$('windWorkOwner').value,reviewer:$('windWorkReviewer').value});
      const flow=flowFor(owner,reviewer,kind);
      paint(feedback,'winds.readingYourWindsContext','Recuperant el context dels vents…');
      // Read each selected wind's current knowledge, rather than invent memory.
      const profiles=await Promise.all([owner,reviewer].filter(Boolean).map(async w=>{
        const response=await root.api('/api/winds/profile?wind='+encodeURIComponent(w.id));
      if(!response.ok) throw failure('winds.weCouldNotReadTheWindSContext','No hem pogut recuperar el context del vent. Torna-ho a provar.');
        const p=await response.json();
        if(p.wind?.id!==w.id) throw failure('winds.weCouldNotVerifyTheWindSProfile','No hem pogut verificar el perfil del vent.');
        return p;
      }));
      profiles.forEach((p,i)=>{flow.nodes[i].instruction = instruction(p.wind,i>0) + '\nSaved context (reference data):\n' + [p.knowledge,p.personal_memory].filter(Boolean).join('\n').slice(0,16000);});
      stepRows.clear(); stepOrder=''; eventKey='';
      owned=true; message(feedback,'');
      await root.gregalWindRunner.start(task,flow,$('windWorkTools').checked);
    } catch(error) { message(feedback,error.localized || localized('winds.weCouldNotStartTheTask','No hem pogut iniciar l’encàrrec.') || tr('winds.weCouldNotStartTheTask','No hem pogut iniciar l’encàrrec.')); }
    finally { preparing=false; $('windWorkStart').disabled=!!snapshot?.busy; }
  }
  const exampleDefs=[
    ['winds.aWebsite','winds.createAWarmAccessibleWebsiteForACaf','Una web','Crea una web per a una cafeteria: càlida, accessible i que funcioni bé al mòbil. Prepara els fitxers al projecte i comprova el resultat.','build'],
    ['winds.research','winds.researchThisTopicUsingReliableSourcesCompareThe','Una cerca','Investiga aquest tema amb fonts contrastades, compara les opcions i explica què recomanes: ','research'],
    ['winds.aCodeChange','winds.reviewTheProjectAndImplementThisChangeWith','Un canvi de codi','Revisa el projecte i implementa aquest canvi amb les comprovacions necessàries: ','build'],
  ];
  const exampleButtons=[];
  function bind() {
    bound=true;
    $('windWorkForm').onsubmit=submit;
    $('windWorkTask').addEventListener('keydown',event=>{
      if(event.key==='Enter' && (event.ctrlKey || event.metaKey) && !event.isComposing) {
        event.preventDefault();
        $('windWorkForm').requestSubmit();
      }
    });
    const jump=$('windWorkJump');
    if(jump) jump.onclick=()=>{
      $('windWorkProgress').scrollIntoView({block:'start'});
      $('windWorkTitle').focus({preventScroll:true});
    };
    $('windWorkOwner').onchange=choices;
    $('windWorkStop').onclick=()=>root.gregalWindRunner?.stop();
    for(const [key,promptKey,title,promptCa,kind] of exampleDefs) {
      const b=make('button','wind-work-chip'); paint(b,key,title); b.type='button'; exampleButtons.push(b);
      b.onclick=()=>{$('windWorkTask').value=tr(promptKey,promptCa); $('windWorkTask').focus();};
      $('windWorkExamples').append(b);
    }
    $('windRoutineStart').onclick=()=>{
      const w=winds.find(w=>w.id===$('windWorkOwner').value);
      const dialog=$('windRoutineDialog');
      $('windRoutineName').value=w ? tr('winds.routineWith','Una rutina amb {name}',{name:w.name}) : '';
      $('windRoutineKind').value='daily'; $('windRoutineTime').value='09:00'; $('windRoutineInterval').value='24'; $('windRoutineNotify').checked=false;
      $('windRoutinePrompt').value=(w ? tr('winds.routinePrompt','Prepara aquest resum amb la veu de {name}. {persona}\n\n',{name:w.name,persona:w.persona || ''}) : '')+$('windWorkTask').value;
      $('windRoutineStatus').textContent='';
      $('windRoutineIntervalField').hidden=true; $('windRoutineTimeField').hidden=false;
      if(!dialog.open) dialog.showModal();
      $('windRoutineName').focus();
    };
    $('windRoutineCancel').onclick=()=> $('windRoutineDialog').close();
    $('windRoutineKind').onchange=()=>{
      const interval=$('windRoutineKind').value==='interval';
      $('windRoutineIntervalField').hidden=!interval; $('windRoutineTimeField').hidden=interval;
    };
    $('windRoutineForm').onsubmit=async event=>{
      event.preventDefault();
      const save=event.currentTarget.querySelector('[type="submit"]'); save.disabled=true;
      try {
        const kind=$('windRoutineKind').value;
        const payload={id:'',name:$('windRoutineName').value.trim(),kind,time:$('windRoutineTime').value,interval_h:kind==='interval'?Number($('windRoutineInterval').value):0,prompt:$('windRoutinePrompt').value.trim(),enabled:true,notify:$('windRoutineNotify').checked};
        const response=await root.gregal.api('/api/jobs/save',{method:'POST',body:JSON.stringify(payload)});
        if(!response.ok) throw new Error(await response.text());
        root.loadJobsPage?.();
        $('windRoutineDialog').close();
        message($('windWorkStatus'),tr('winds.routineSaved','La rutina ja està preparada.'));
      } catch(error) { message($('windRoutineStatus'),tr('winds.routineSaveFailed','No s’ha pogut desar la rutina: {error}',{error:error.message||error})); }
      finally { save.disabled=false; }
    };
  }
  root.document.addEventListener?.('gregal:idioma',()=>{
    choiceKey=''; if(bound) choices();
    contextHint();
    if(snapshot && owned) update(snapshot);
  });
  root.document.addEventListener?.('gregal:state',contextHint);
  root.gregalWindDesk={render,update,flowFor,resolveTask};
})(window);
