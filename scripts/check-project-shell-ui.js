async page => {
  await page.route('**/api/workspaces',r=>r.fulfill({json:{workspaces:[{name:'Gregal',path:'C:/dev/gregal'},{name:'Garden',path:'C:/work/garden'}]}}));
  await page.route('**/api/sessions',r=>r.fulfill({json:[{name:'project-chat',title:'Polish the garden',workspace:'C:/work/garden',msgs:2},{name:'general-chat',title:'A quick question',msgs:2}]}));
  await page.setViewportSize({width:1440,height:900});
  await page.reload();
  await page.waitForFunction(()=>window.gregalPanels?.convs.projects.length===2);
  await page.evaluate(()=>{window.gregal.setView('agent');document.getElementById('side').classList.remove('closed','compact');});
  await page.setViewportSize({width:1440,height:900});
  if(await page.locator('#convs .conv').count()!==1)throw Error('Project conversations mixed with general conversations');
  if(await page.locator('#projectList .project-open').count()!==2)throw Error('Missing empty registered project');
  await page.evaluate(()=>{window.__opened=[];window.gregalPanels.sessions.open=async path=>window.__opened.push(path);});
  await page.locator('.project-new[data-path="C:/work/garden"]').click();
  if((await page.evaluate(()=>window.__opened))[0]!=='C:/work/garden')throw Error('Wrong project folder');
  for(const width of [1440,760,390]) {
    await page.setViewportSize({width,height:900});
    if(width<761)await page.evaluate(()=>document.getElementById('side').classList.add('closed'));
    const rects=await page.evaluate(()=>['modelPill','permBtn','attach','send'].map(id=>{const r=document.getElementById(id).getBoundingClientRect();return {id,x:r.x,y:r.y,w:r.width,right:r.right};}));
    if(Math.max(...rects.map(r=>r.y))-Math.min(...rects.map(r=>r.y))>8)throw Error('Composer controls wrap: '+JSON.stringify(rects));
    if(rects.some(r=>r.x<0||r.right>width))throw Error('Composer overflow');
    if(await page.locator('#modeCode').isVisible())throw Error('Modes still take composer space');
    await page.locator('.composer-options summary').click();
    if(!await page.locator('#modeCode').isVisible())throw Error('Advanced modes inaccessible');
    await page.locator('.composer-options summary').click();
    await page.screenshot({path:'output/refined-shell-'+width+'.png',animations:'disabled'});
  }
  await page.setViewportSize({width:1440,height:900});
  await page.evaluate(async()=>{document.getElementById('side').classList.remove('closed','compact');await (await import('/app/i18n.js')).posa('ca');});
  if(await page.locator('[data-i18n="shell.projects"]').textContent()!=='Projectes')throw Error('Catalan project heading');
  await page.screenshot({path:'output/refined-shell-ca.png',animations:'disabled'});
}
