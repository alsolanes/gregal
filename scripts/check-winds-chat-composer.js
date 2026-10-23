async page=>{
 const errors=[];page.on('pageerror',e=>errors.push(e.message));
 const prompt='Em recomanes un portàtil amb molta bateria, potent i lleuger?';
 await page.route('**/api/**',async route=>{
  const req=route.request(),url=new URL(req.url());let data={};
  if(url.pathname.endsWith('/stream'))return route.continue();
  if(url.pathname==='/api/state')data={provider:'fixture',model:'Model de prova',project:'Projecte',cwd:'C:/projects/cafe',lang:'ca',mode:'code'};
  else if(url.pathname==='/api/winds')data={winds:[{id:'marinada',name:'Marinada',role:'Compres i comparacions',squad:'daily',specialty:'shopping',color:'#EFC69A',style:'psn'}]};
  else if(url.pathname==='/api/winds/squads')data={squads:[{id:'daily',ca:'Dia a dia',en:'Everyday'},{id:'data',ca:'Ciència de dades',en:'Data science'},{id:'software',ca:'Software i seguretat',en:'Software and security'}]};
  else if(url.pathname==='/api/winds/rotllanes')data=req.method()==='POST'?{rotllana:{id:'chat',name:prompt,members:['marinada']}}:{rotllanes:[]};
  else if(url.pathname==='/api/winds/tasks')data={tasks:[]};
  else if(url.pathname==='/api/workspaces')data={current:'C:/projects/cafe',workspaces:[{name:'Projecte',path:'C:/projects/cafe'}]};
  else if(url.pathname==='/api/sessions')data=[];
  else if(url.pathname==='/api/sessions/live')data={sessions:[]};
  return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
 });
 await page.setViewportSize({width:1440,height:960});await page.goto('http://127.0.0.1:8766');
 await page.waitForFunction(()=>window.gregalWindWorkspace&&window.gregalWindRunner);
 await page.evaluate(async()=>{(await import('/app/i18n.js')).posa('ca');document.documentElement.dataset.theme='dark';document.body.classList.add('desktop');window.gregal.setView('vents');document.getElementById('side').classList.remove('compact','closed');});
 await page.locator('.vw-composer textarea').fill(prompt);await page.locator('.vw-send').click();await page.locator('.vw-pending').waitFor();
 if(await page.getByRole('button',{name:'Atura la resposta',exact:true}).isDisabled())throw new Error('Chat stop is disabled');
 async function geometry(){return page.evaluate(()=>{const b=document.querySelector('.vw-send').getBoundingClientRect(),icon=document.querySelector('.vw-send svg').getBoundingClientRect(),composer=document.querySelector('.vw-composer').getBoundingClientRect(),pending=document.querySelector('.vw-pending').getBoundingClientRect();return{dx:Math.abs(b.x+b.width/2-icon.x-icon.width/2),dy:Math.abs(b.y+b.height/2-icon.y-icon.height/2),contained:b.bottom<=composer.bottom&&b.top>=composer.top,pendingAbove:pending.bottom<composer.top,footer:document.querySelector('.vw-status').textContent,overflow:document.documentElement.scrollWidth>innerWidth};});}
 let g=await geometry();if(g.dx>.5||g.dy>.5||!g.contained||!g.pendingAbove||g.footer||g.overflow)throw new Error('Invalid desktop pending layout: '+JSON.stringify(g));
 await page.screenshot({path:'output/playwright/winds-chat-waiting-dark.png'});
 await page.request.post('http://127.0.0.1:8766/qa/phase',{data:{phase:'searching'}});await page.locator('.vw-thinking').filter({hasText:'Cercant informació'}).waitFor();
 await page.setViewportSize({width:390,height:844});g=await geometry();if(g.dx>.5||g.dy>.5||!g.contained||g.overflow)throw new Error('Invalid mobile stop geometry');
 await page.screenshot({path:'output/playwright/winds-chat-waiting-mobile.png'});
 await page.request.post('http://127.0.0.1:8766/qa/token',{data:{token:'Resposta provisional'}});await page.locator('.vw-stream-text').filter({hasText:'Resposta provisional'}).waitFor();
 await page.request.post('http://127.0.0.1:8766/qa/phase',{data:{phase:'working'}});await page.waitForFunction(()=>!document.querySelector('.vw-stream-text'));
 await page.getByRole('button',{name:'Atura la resposta',exact:true}).click();await page.waitForFunction(()=>!document.querySelector('.vw-pending')&&document.querySelector('.vw-send').dataset.busy==='false');
 if(await page.locator('.vw-composer textarea').inputValue()!==prompt)throw new Error('Stopped request lost the draft');
 if((await (await page.request.get('http://127.0.0.1:8766/qa/state')).json()).aborted<1)throw new Error('Stop did not cancel the HTTP request');
 await page.locator('.vw-send').click();await page.locator('.vw-pending').waitFor();
 await page.evaluate(async()=>{(await import('/app/i18n.js')).posa('en');});await page.getByRole('button',{name:'Stop response',exact:true}).waitFor();
 await page.request.post('http://127.0.0.1:8766/qa/finish',{data:{prompt}});await page.waitForFunction(()=>!document.querySelector('.vw-pending')&&document.querySelector('.vw-send').dataset.busy==='false');
 await page.locator('.vw-message-text').filter({hasText:'Ja tinc els criteris'}).waitFor();
 if(errors.length)throw new Error(errors.join('\n'));
 return 'Stop alignment, real cancellation, in-thread waiting, live phases, tokens, speculative text reset, mobile layout and both languages passed.';
}
