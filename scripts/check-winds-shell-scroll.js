// Serve internal/web on port 8766; run with playwright-cli run-code --filename.
async page => {
  const winds=[{id:'marinada',name:'Marinada',role:'calma i benestar',color:'#F2C88F',style:'psn'},{id:'mestral',name:'Mestral',role:'aprendre plegats',color:'#BFCFA4',style:'esn'}];
  await page.route('**/api/**',route=>{
    const url=new URL(route.request().url());
    let data={};
    if(url.pathname==='/api/winds') data={winds};
    else if(url.pathname==='/api/winds/profile') data={wind:winds.find(w=>w.id===url.searchParams.get('wind')),knowledge:'',personal_memory:''};
    else if(url.pathname==='/api/winds/rotllanes') data={rotllanes:[]};
    else if(url.pathname==='/api/winds/chat') data={history:url.searchParams.get('wind')==='mestral'?[]:Array.from({length:14},(_,i)=>({from:i%2?'assistant':'user',speaker_id:'marinada',text:('Una conversa llarga per comprovar la distribució i el desplaçament. ').repeat(6)}))};
    else if(url.pathname==='/api/sessions') data=[];
    else if(url.pathname==='/api/state') data={provider:'test',model:'fast',project:'Gregal',cwd:'C:/dev/gregal',lang:'ca',mode:'code'};
    return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(data)});
  });
  await page.setViewportSize({width:1440,height:900});
  await page.goto('http://127.0.0.1:8766');
  await page.waitForFunction(()=>!!window.gregalShell && !!window.gregalWindI18n);
  await page.evaluate(()=>{document.getElementById('side').classList.remove('compact','closed');document.body.classList.add('desktop');document.documentElement.dataset.theme='dark';window.gregal.setView('vents');});
  await page.locator('#windsList .winds-row').first().waitFor();
  const geometry=()=>page.evaluate(()=>({windowY:scrollY,bodyScroll:document.body.scrollTop,rootScroll:document.documentElement.scrollTop,bodyY:document.body.getBoundingClientRect().y,sideY:document.getElementById('side').getBoundingClientRect().y,tabsY:document.getElementById('sessionTabs').getBoundingClientRect().y,pageScroll:document.getElementById('ventsPage').scrollTop,sideScroll:document.querySelector('.side-scroll').scrollTop}));
  const before=await geometry();
  await page.screenshot({path:'output/playwright/winds-before-selection.png'});
  await page.locator('#windsList .winds-row').filter({hasText:'Mestral'}).click();
  await page.waitForFunction(()=>!document.getElementById('windChatCard').hidden && !document.getElementById('windsChatStatus').textContent);
  const empty=await geometry();
  await page.screenshot({path:'output/playwright/winds-after-empty-selection.png'});
  await page.locator('#windsList .winds-row').filter({hasText:'Marinada'}).click();
  await page.waitForFunction(()=>document.querySelectorAll('#windChatHistory .winds-message').length===14);
  const long=await geometry();
  await page.screenshot({path:'output/playwright/winds-after-long-selection.png'});
  await page.locator('#windsChatMembers button').first().click();
  await page.locator('#windsProfileTitle').waitFor();
  const profile=await geometry();
  const report={before,empty,long,profile};
  for(const [state,g] of Object.entries(report)) {
    if(g.windowY!==0 || g.bodyScroll!==0 || g.rootScroll!==0 || g.sideY!==0 || g.tabsY!==0) throw new Error('App shell moved in '+state+': '+JSON.stringify(g));
  }
  await page.evaluate(r=>{window.__windScrollReport=r;},report);
}
