// Browser regression check with intercepted APIs. Serve internal/web on port 8766.
// playwright-cli run-code --filename scripts/check-shell-ui.js
async page => {
  await page.route('**/api/**', route=>{
    const url=new URL(route.request().url());
    const data=url.pathname==='/api/winds' ? {winds:[{id:'tramuntana',name:'Tramuntana',role:'Research',color:'#A8CDE2',style:'pso'}]} : url.pathname==='/api/winds/rotllanes' ? {rotllanes:[]} : {};
    return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(data)});
  });
  await page.setViewportSize({width:1440,height:1000});
  await page.route('**/api/sessions',route=>route.fulfill({status:200,contentType:'application/json',body:'[]'}));
  await page.route('**/api/state*',route=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({provider:'test',model:'fast-model',project:'Gregal',branch:'main',cwd:'C:/dev/gregal',mode:'code',lang:'en'})}));
  await page.evaluate(()=>{localStorage.setItem('gregal_side_compact','0');localStorage.setItem('gregal_side','open');document.getElementById('side').classList.remove('compact','closed');window.gregal.refresh();});
  await page.route('**/api/models',route=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({models:{test:['fast-model','careful-model']},current:'test/fast-model',roles:{}})}));
  await page.goto('http://127.0.0.1:8766');
  await page.waitForFunction(()=>!!window.gregalShell && !!window.gregalPanels);
  await page.evaluate(async()=>{await (await import('/app/i18n.js')).posa('en');window.gregal.setView('agent');});
  if(!await page.locator('#navRail').isVisible())throw new Error('Missing app rail');
  if(!await page.locator('#conversationNav').isVisible())throw new Error('Missing conversation sidebar');
  if(!await page.locator('#composer #modelPill').count())throw new Error('Model selector outside composer');
  await page.locator('#top [data-view="fitxers"]').click();
  if(!await page.locator('#toolDock #fitxersPage').isVisible() || !await page.locator('#composer').isVisible())throw new Error('Files replaced chat');
  await page.screenshot({path:'output/shell-tools.png',animations:'disabled'});
  await page.locator('#modelPill').click();
  await page.locator('#mpick .mp-search').waitFor();
  const bounds=await page.locator('#mpick').boundingBox();
  if(!bounds || bounds.y<0 || bounds.y+bounds.height>1000)throw new Error('Model menu clipped: '+JSON.stringify(bounds));
  await page.keyboard.press('Escape');
  await page.locator('#toolDockClose').click();
  if(await page.locator('#toolDock').isVisible())throw new Error('Tool dock did not close');
  await page.locator('#sideCompact').click();
  await page.waitForFunction(()=>document.getElementById('side').getBoundingClientRect().width<1);
  if((await page.locator('#side').boundingBox()).width>1)throw new Error('Context sidebar did not collapse');
  if(!await page.locator('#prefsBtn').isVisible() || !await page.locator('#keysBtn').isVisible())throw new Error('Utilities hidden with context');
  await page.locator('#menu').click();
  if(!await page.locator('#conversationNav').isVisible())throw new Error('Context sidebar did not reopen');
  await page.locator('#navRail [data-view="vents"]').click();
  await page.evaluate(()=>{document.getElementById('ventsPage').scrollTop=0;});
  if(!await page.locator('#windContextNav #windsListCard').isVisible())throw new Error('Wind context not moved to sidebar');
  if(await page.locator('#conversationNav').isVisible())throw new Error('Duplicate sidebars');
  await page.screenshot({path:'output/shell-winds.png',animations:'disabled'});
  await page.locator('#sideCompact').click();
  await page.waitForFunction(()=>document.getElementById('side').getBoundingClientRect().width<1);
  await page.setViewportSize({width:390,height:844});
  await page.evaluate(()=>window.gregal.setView('agent'));
  await page.locator('#menu').click();
  if(!await page.locator('#convSearch').isVisible())throw new Error('Desktop collapse hid mobile conversations');
  await page.locator('#sideClose').click();
  await page.locator('#top [data-view="canvis"]').click();
  if(!await page.locator('#toolDock #canvisPage').isVisible())throw new Error('Mobile tool dock missing');
  const box=await page.locator('#toolDock').boundingBox();
  if(box.x<0 || box.x+box.width>390)throw new Error('Mobile tool dock overflow');
  await page.locator('#toolDockClose').click();
  await page.locator('#sideMore summary').click();
  if(!await page.locator('#prefsBtn').isVisible())throw new Error('More hides settings');
  await page.screenshot({path:'output/shell-mobile.png',animations:'disabled'});
}
