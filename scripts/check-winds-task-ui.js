// Start real Wind runner with intercepted API/SSE, without invoking a model.
// Serve internal/web on port 8766; playwright-cli run-code --filename this-file.
async page => {
  const wind={id:'marinada',name:'Marinada',role:'Webs',color:'#F2C88F',style:'psn'};
  const requests=[];
  let running=false, finishRun;
  const runFinished=new Promise(resolve=>{finishRun=resolve;});
  await page.route('**/api/**',async route=>{
    const url=new URL(route.request().url()); requests.push(url.pathname);
    let data={};
    if(url.pathname==='/api/winds') data={winds:[wind]};
    else if(url.pathname==='/api/winds/profile') data={wind,knowledge:'',personal_memory:''};
    else if(url.pathname==='/api/winds/rotllanes') data={rotllanes:[]};
    else if(url.pathname==='/api/sessions') data=[];
    else if(url.pathname==='/api/sessions/live') data={sessions:[{id:'default',project:'Gregal',busy:running}]};
    else if(url.pathname==='/api/state') data={provider:'test',model:'fast',project:'Gregal',cwd:'C:/dev/gregal',lang:'ca',mode:'code'};
    else if(url.pathname==='/api/v2/team/run') {
      running=true;
      await runFinished;
      running=false;
      return route.fulfill({status:200,contentType:'text/event-stream',body:[{type:'working',agent:'builder'},{type:'completed',agent:'builder',output:'Resultat de prova'},{type:'done',output:'Resultat de prova'}].map(e=>'event: team\ndata: '+JSON.stringify(e)+'\n\n').join('')});
    }
    return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(data)});
  });
  await page.setViewportSize({width:1440,height:900});
  await page.goto('http://127.0.0.1:8766');
  await page.waitForFunction(()=>window.gregalWindRunner && window.gregalPanels?.sessions?._loadedSession==='default');
  await page.evaluate(()=>{
    const calls=[]; window.__viewCalls=calls;
    const original=window.setView;
    window.setView=function(view){calls.push({view,stack:new Error().stack});return original(view);};
    window.gregal.setView=window.setView;
    document.getElementById('side').classList.remove('compact','closed');
    window.gregal.setView('vents');
  });
  await page.locator('#windWorkOwner option[value="marinada"]').waitFor({state:'attached'});
  await page.locator('#windWorkTask').fill('Crea una web de prova');
  await page.locator('#windWorkStart').click();
  await page.waitForFunction(()=>!document.getElementById('windWorkProgress').hidden);
  await page.evaluate(async()=>{
    const sessions=window.gregalPanels.sessions;
    await sessions.syncCurrent(await sessions.list());
  });
  const during=await page.evaluate(()=>window.gregal.viewActual());
  finishRun();
  await page.waitForFunction(()=>document.getElementById('windWorkSteps').textContent.includes('Resultat de prova'));
  await page.evaluate(async()=>{
    const sessions=window.gregalPanels.sessions;
    await sessions.syncCurrent(await sessions.list());
  });
  const result=await page.evaluate(()=>({view:window.gregal.viewActual(),calls:window.__viewCalls,visible:!document.getElementById('ventsPage').hidden}));
  await page.evaluate(r=>{window.__windTaskReport=r;},{...result,during,requests});
  if(during!=='vents' || result.view!=='vents' || !result.visible) throw new Error('Wind task navigated away: '+JSON.stringify({...result,during}));
  if(requests.filter(path=>path==='/api/v2/team/run').length!==1) throw new Error('Wind task was not started exactly once');
  if(requests.includes('/api/sessions/open')) throw new Error('Wind task opened another chat session');
}
