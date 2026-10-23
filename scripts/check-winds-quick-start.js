// Exercise first use with the actual UI and runner; API/model responses are fixtures.
async page => {
  const winds=[]; const runs=[]; let finish;
  const completed=new Promise(resolve=>{finish=resolve;});
  await page.route('**/api/**',async route=>{
    const req=route.request(), url=new URL(req.url()); let data={};
    if(url.pathname==='/api/state') data={provider:'fixture',model:'fast',project:'Cafe',cwd:'C:/projects/cafe',lang:'ca',mode:'code'};
    else if(url.pathname==='/api/winds') {
      if(req.method()==='POST') { const wind={...req.postDataJSON(),id:req.postDataJSON().name.toLowerCase()}; winds.push(wind); data={wind}; }
      else data={winds};
    } else if(url.pathname==='/api/winds/profile') data={wind:winds.find(w=>w.id===url.searchParams.get('wind')),knowledge:'',personal_memory:''};
    else if(url.pathname==='/api/winds/rotllanes') data={rotllanes:[]};
    else if(url.pathname==='/api/sessions') data=[];
    else if(url.pathname==='/api/sessions/live') data={sessions:[]};
    else if(url.pathname==='/api/v2/team/run') {
      runs.push(req.postDataJSON()); await completed;
      return route.fulfill({contentType:'text/event-stream',body:[{type:'working',agent:'builder'},{type:'tool_result',agent:'builder',activity:'write',path:'index.html',message:'Written'},{type:'completed',agent:'builder',output:'Website created.'},{type:'working',agent:'reviewer'},{type:'completed',agent:'reviewer',output:'Checked the website.'},{type:'done',output:'Checked the website.'}].map(e=>'event: team\ndata: '+JSON.stringify(e)+'\n\n').join('')});
    } else if(url.pathname==='/api/file') data={content:'<!doctype html><html><body><h1>Cafe preview</h1></body></html>'};
    return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  });
  await page.setViewportSize({width:1440,height:1000});
  await page.goto('http://127.0.0.1:8766');
  await page.waitForFunction(()=>window.gregalWindRunner && window.gregalWindSetup);
  await page.evaluate(()=>window.gregal.setView('vents'));
  await page.locator('.wind-desk-empty').waitFor();
  if(await page.locator('#windWorkStart').isDisabled()) throw new Error('First task requires manual crew setup');
  if(await page.locator('#windWorkOwner').isVisible()) throw new Error('Advanced controls are exposed by default');
  const composer=await page.locator('#windWorkForm').boundingBox();
  const roster=await page.locator('#windDeskRoster').boundingBox();
  if(composer.y>=roster.y) throw new Error('Roster hides the request composer');
  await page.evaluate(async()=>{(await import('/app/i18n.js')).posa('en');document.documentElement.setAttribute('data-theme','dark');});
  if((await page.locator('.wind-work-settings > summary').textContent()).trim()!=='Task options') throw new Error('English options label missing');
  await page.screenshot({path:'output/playwright/winds-quick-desktop.png'});
  await page.setViewportSize({width:390,height:844});
  await page.screenshot({path:'output/playwright/winds-quick-mobile.png'});
  if(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth)) throw new Error('Mobile layout overflows');
  await page.setViewportSize({width:1440,height:1000});
  await page.locator('#windWorkTask').fill('Create a website for a cafe');
  await page.locator('#windWorkStart').click();
  await page.waitForFunction(()=>!document.getElementById('windWorkProgress').hidden);
  if(winds.length!==3 || runs.length!==1) throw new Error('First task did not prepare its crew and start exactly once');
  const run=runs[0];
  if(!run.tools || !run.winds || run.flow.nodes[0].label!=='Tramuntana' || run.flow.nodes[1]?.label!=='Mestral') throw new Error('Automatic assignment did not select builder and review');
  finish();
  await page.waitForFunction(()=>document.getElementById('windWorkSummary').textContent==='Task completed');
  await page.locator('#windWorkSteps .wind-work-preview').first().click();
  await page.waitForFunction(()=>window.gregalPreview.getState().tabs.some(t=>t.kind==='html' && t.content.includes('Cafe preview')));
  if(await page.evaluate(()=>window.gregal.viewActual())!=='vents') throw new Error('Task left Winds');
  await page.locator('.wind-work-settings > summary').click();
  await page.locator('#windWorkReviewer').selectOption('');
  await page.evaluate(async()=>{(await import('/app/i18n.js')).posa('ca');});
  if(await page.locator('#windWorkReviewer').inputValue()!=='') throw new Error('Language switch lost manual no-review setting');
  await page.screenshot({path:'output/playwright/winds-quick-completed.png'});
}
