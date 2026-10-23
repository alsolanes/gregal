// Real app UI and runner, deterministic API fixtures; no model calls.
async page => {
 const errors=[];page.on('pageerror',e=>errors.push(e.message));
 let winds=[{id:'front',name:'Tramuntana',squad:'software',specialty:'frontend',role:'Frontend',color:'#C1B4DE',style:'psn'},{id:'review',name:'Mestral',squad:'software',specialty:'testing',role:'Qualitat',color:'#BFCFA4',style:'esn'}];
 const squads=[{id:'daily',ca:'Dia a dia',en:'Everyday'},{id:'data',ca:'Ciència de dades',en:'Data science'},{id:'software',ca:'Software i seguretat',en:'Software and security'}];
 let chats=[{id:'thread',name:'Una idea per al cafè',members:['front']}],runs=[],finished,finishChat,holdChat=false,history=[{from:'user',text:'Hola'},{from:'pet',speaker_id:'front',speaker_name:'Tramuntana',text:'Hola! Com et puc ajudar?'}];
 const runDone=new Promise(r=>finished=r);
 const chatDone=new Promise(r=>finishChat=r);
 await page.route('**/api/**',async route=>{
  const req=route.request(),u=new URL(req.url());let data={};
  if(u.pathname==='/api/state')data={provider:'test',model:'fast-model',project:'Cafe',cwd:'C:/projects/cafe',lang:'ca',mode:'code'};
  else if(u.pathname==='/api/winds')data={winds};
  else if(u.pathname==='/api/winds/squads'){if(req.method()==='POST'){const s=req.postDataJSON().id;winds.push({id:s,name:'Marinada',squad:s,role:'Compres',color:'#E7BE94',style:'psn'});}data={squads};}
  else if(u.pathname==='/api/winds/rotllanes'){if(req.method()==='POST'){const c={id:'new',...req.postDataJSON()};chats.push(c);data={rotllana:c};}else data={rotllanes:chats};}
  else if(u.pathname==='/api/winds/rotllana/stream'){if(holdChat)await chatDone;return route.fulfill({contentType:'text/event-stream',body:'event: done\ndata: '+JSON.stringify({history})+'\n\n'});}
  else if(u.pathname==='/api/winds/rotllana')data={history};
  else if(u.pathname==='/api/winds/tasks')data={tasks:[]};
  else if(u.pathname==='/api/winds/profile')data={wind:winds[0],memory:'Prefereix el català.',knowledge:'Web accessible.',skills:[]};
  else if(u.pathname==='/api/winds/library')data={items:[{id:'active-front',kind:'active',wind:winds[0],memory:'Prefereix el català.',knowledge:'Web accessible.',skills:[]},{id:'archive',kind:'archived',wind:{id:'old',name:'Ponent',color:'#BFCFA4',style:'psn'},memory:'Llibres.',knowledge:'',skills:[]}]};
  else if(u.pathname==='/api/workspaces')data={current:'C:/projects/cafe',workspaces:[{path:'C:/projects/cafe',name:'Cafe'}]};
  else if(u.pathname==='/api/sessions')data=[];
  else if(u.pathname==='/api/sessions/live')data={sessions:[]};
  else if(u.pathname==='/api/v2/team/run'){runs.push(req.postDataJSON());await runDone;return route.fulfill({contentType:'text/event-stream',body:[{type:'working',agent:'builder'},{type:'tool_result',agent:'builder',activity:'write',path:'index.html',message:'Written'},{type:'completed',agent:'builder',output:'Website created.'},{type:'done',output:'Website created.'}].map(e=>'event: team\ndata: '+JSON.stringify(e)+'\n\n').join('')});}
  else if(u.pathname==='/api/file')data={content:u.searchParams.get('path')?.endsWith('.css')?'body{background:#e8ddc9}':u.searchParams.get('path')?.endsWith('.js')?'window.cafeReady=true;':'<!doctype html><html><head><link rel="stylesheet" href="style.css"></head><body><h1>Cafe preview</h1><script src="script.js"></script></body></html>'};
  return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
 });
 await page.setViewportSize({width:1440,height:960});await page.goto('http://127.0.0.1:8766');
 await page.waitForFunction(()=>window.gregalWindWorkspace&&window.gregalWindRunner);
 await page.evaluate(async()=>{(await import('/app/i18n.js')).posa('ca');document.body.classList.add('desktop');document.documentElement.dataset.theme='dark';window.gregal.setView('vents');document.getElementById('side').classList.remove('compact','closed');});
 await page.locator('.vw-welcome h1').waitFor();
 await page.screenshot({path:'output/playwright/vents-workspace-home-dark.png'});
 await page.locator('.vw-nav-item').filter({hasText:'Una idea per al cafè'}).click();
 await page.locator('.vw-message').first().waitFor();
 if(await page.evaluate(()=>scrollY!==0||document.body.getBoundingClientRect().y<0))throw new Error('Chat moved the app upwards');
 await page.locator('.vw-composer textarea').fill('draft to preserve');
 await page.evaluate(async()=>{(await import('/app/i18n.js')).posa('en');});
 if(await page.locator('.vw-composer textarea').inputValue()!=='draft to preserve')throw new Error('Language switch lost draft');
 if((await page.locator('.vw-message strong').last().innerText())!=='Tramuntana')throw new Error('Speaker identity missing');
 await page.locator('.vw-project').click();await page.locator('.vw-project-path').waitFor();
 if(!await page.locator('.vw-project svg').count())throw new Error('Project folder icon missing');
 await page.locator('.vw-top-action').click();await page.locator('.vw-tabs button').filter({hasText:'Software and security'}).click();
 await page.locator('.vw-member').first().locator('button').filter({hasText:'Profile'}).click();await page.locator('.vw-dialog').waitFor();
 if(await page.locator('.vw-dialog textarea').inputValue()!=='Prefereix el català.')throw new Error('Per-wind memory missing');
 await page.locator('.vw-dialog .vw-tabs button').filter({hasText:'Knowledge'}).click();
 if(await page.locator('.vw-dialog textarea').inputValue()!=='Web accessible.')throw new Error('Knowledge not separated');
 await page.locator('.vw-dialog .vw-close').click();
 await page.locator('.vw-nav-item').filter({hasText:'Library'}).click();await page.locator('.vw-library-item').first().waitFor();
 await page.locator('.vw-tabs button').filter({hasText:'Archived'}).click();await page.locator('.vw-library-item').first().waitFor();
 if(!await page.locator('.vw-library-item').filter({hasText:'Ponent'}).count())throw new Error('Archive inaccessible');
 await page.locator('.vw-library-item button').filter({hasText:'Assign'}).click();await page.locator('.vw-dialog select').waitFor();await page.locator('.vw-dialog .vw-close').click();
 await page.locator('.vw-nav-item').filter({hasText:'Software and security'}).click();
 await page.locator('.vw-composer textarea').fill('Create a website for a cafe');await page.locator('.vw-send').click();
 await page.locator('#vwWork').waitFor();await page.waitForFunction(()=>document.querySelector('.vw-send').dataset.busy==='true');
 await page.locator('.vw-nav-item').filter({hasText:'◷ Work'}).click();
 await page.getByRole('button',{name:'Return to current work'}).click();
 await page.locator('.vw-log summary').click();
 finished();await page.locator('.vw-deliverable button').filter({hasText:'index.html'}).waitFor();
 if(!await page.locator('.vw-log').evaluate(n=>n.open))throw new Error('Live progress collapsed the activity log');
 if(runs.length!==1||!runs[0].winds||!runs[0].tools||runs[0].flow.nodes[0].wind!=='front')throw new Error('Real runner/context binding missing');
 await page.locator('.vw-deliverable button').filter({hasText:'index.html'}).click();
 await page.waitForFunction(()=>window.gregalPreview.getState().tabs.some(t=>t.kind==='html'&&t.content.includes('Cafe preview')&&t.content.includes('background:#e8ddc9')&&t.content.includes('window.cafeReady=true')));
 await page.frameLocator('#previewViewport iframe:not([hidden])').getByRole('heading',{name:'Cafe preview'}).waitFor();
 if(await page.evaluate(()=>window.gregal.viewActual())!=='vents')throw new Error('Task navigated out of Winds');
 await page.screenshot({path:'output/playwright/vents-workspace-result-dark.png'});
 await page.locator('.vw-new').click();await page.setViewportSize({width:390,height:844});
 await page.screenshot({path:'output/playwright/vents-workspace-mobile-dark.png'});
 for(const width of [320,390,768,1440]){await page.setViewportSize({width,height:900});if(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth))throw new Error('Horizontal overflow at '+width);}
 await page.setViewportSize({width:1440,height:960});await page.evaluate(()=>{document.documentElement.dataset.theme='light';});await page.waitForFunction(()=>getComputedStyle(document.querySelector('.vw-button')).color==='rgb(31, 36, 51)');await page.screenshot({path:'output/playwright/vents-workspace-home-light.png'});
 holdChat=true;await page.locator('.vw-composer textarea').fill('Tell me about coffee');await page.locator('.vw-send').click();await page.locator('.vw-messages').waitFor();await page.locator('.vw-top-action').click();
 const reply=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/winds/rotllana/stream'&&r.request().method()==='POST');finishChat();await reply;await page.evaluate(()=>new Promise(requestAnimationFrame));
 if(!await page.locator('.vw-member').count()||await page.locator('.vw-messages').count())throw new Error('A background reply replaced the newly selected section');
 if(errors.length)throw new Error(errors.join('\n'));
 console.log('Workspace navigation, preserved draft, profiles, recovery, real runner, HTML preview, language and responsive layout passed.');
}
