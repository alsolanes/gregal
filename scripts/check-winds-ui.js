// Browser regression check. Serve internal/web on localhost:8766, then run
// playwright-cli run-code --filename scripts/check-winds-ui.js
// All API requests are intercepted; no model calls are made.
async page => {
  await (async page => {
  const winds=[{id:'tramuntana',name:'Tramuntana',role:'',persona:'Personal character',style:'pso',color:'#A8CDE2'},{id:'mestral',name:'Mestral',role:'',style:'esn',color:'#BFCFA4'}];
  const errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  await page.route('**/api/**',async route=>{
    const url=new URL(route.request().url()); let data={};
    if(url.pathname==='/api/winds') data={winds};
    else if(url.pathname==='/api/winds/rotllanes') data={rotllanes:[{id:'crew',name:'Personal group',members:winds.map(w=>w.id)}]};
    else if(url.pathname==='/api/winds/profile') data={wind:winds.find(w=>w.id===url.searchParams.get('wind')),knowledge:'My saved knowledge',personal_memory:'My personal memory'};
    else if(url.pathname.startsWith('/api/winds/chat')) data={history:[]};
    else if(url.pathname==='/api/sessions') data={sessions:[]};
    else if(url.pathname==='/api/jobs') data={jobs:[]};
    await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(data)});
  });
  await page.setViewportSize({width:1440,height:1000});
  await page.goto('http://127.0.0.1:8766');
  await page.waitForFunction(()=>!!window.gregalWindRunner && !!window.gregalWindI18n);
  const language=async lang=>page.evaluate(async l=>(await import('/app/i18n.js')).posa(l),lang);
  const assertText=async(selector,text)=>{const actual=await page.locator(selector).textContent();if(actual.trim()!==text)throw new Error(selector+': '+actual+' != '+text);};
  await language('en');
  await page.evaluate(()=>window.gregal.setView('vents'));
  await page.locator('.wind-desk-card').first().waitFor();
  await assertText('#ventsPage .page-title','Make room for your ideas.');
  await assertText('#windWorkExamples button:first-child','A website');
  await assertText('#windWorkReviewer option:first-child','Automatic');
  await page.locator('.wind-work-settings > summary').click();
  await assertText('.wind-desk-card:first-child button','Give them a task ↗');
  await page.locator('#windWorkExamples button').first().click();
  if(!(await page.locator('#windWorkTask').inputValue()).startsWith('Create a warm'))throw new Error('English example prompt missing');
  await page.locator('#windWorkTask').fill('My task draft');
  await page.locator('#windWorkReviewer').selectOption('mestral');
  await page.locator('#windsList button').first().click();
  await page.locator('#windChatInput').fill('My chat draft');
  await language('ca');
  await assertText('#ventsPage .page-title','Fem lloc a les teves idees.');
  await assertText('#windWorkExamples button:first-child','Una web');
  await assertText('#windWorkReviewer option:first-child','Automàtic');
  if(await page.locator('#windWorkTask').inputValue()!=='My task draft')throw new Error('Task draft lost');
  if(await page.locator('#windChatInput').inputValue()!=='My chat draft')throw new Error('Chat draft lost');
  if(await page.locator('#windWorkReviewer').inputValue()!=='mestral')throw new Error('Reviewer selection lost');
  await page.locator('#windNewBtn').click();
  await page.locator('#windsCreateWind').click();
  await language('en');
  await assertText('#windsDialogTitle','Which wind will join you?');
  await page.locator('#windsPresetGallery button').filter({hasText:'Tramuntana'}).click();
  if(await page.locator('#windRole').inputValue()!=='websites and code')throw new Error('Preset not English');
  await page.locator('#windPersona').fill('My custom character');
  await language('ca');
  if(await page.locator('#windPersona').inputValue()!=='My custom character')throw new Error('Profile draft replaced');
  if(await page.locator('#windRole').getAttribute('placeholder')!=='Cerques, webs, codi, revisió…')throw new Error('Placeholder not Catalan');
  await assertText('#windsDialogTitle','Un vent nou');
  await language('en');
  if(await page.locator('#windRole').getAttribute('placeholder')!=='Research, websites, code, reviews…')throw new Error('Placeholder not English');
  await page.locator('#windsDialog button').filter({hasText:'Cancel'}).click();
  // Every marked UI string must follow its catalog, including aria/placeholder attributes.
  const mismatches=await page.evaluate(async()=>{
    const {t}=await import('/app/i18n.js'); const out=[];
    for(const el of document.querySelectorAll('#ventsPage [data-i18n]')) if(el.textContent!==t(el.dataset.i18n)) out.push(el.dataset.i18n);
    for(const el of document.querySelectorAll('#ventsPage [data-wind-i18n]')) {
      const vars=JSON.parse(el.getAttribute('data-wind-vars')||'{}');
      const expected=t(el.getAttribute('data-wind-i18n')).replace(/\{(\w+)\}/g,(m,k)=>vars[k]??m);
      if(el.textContent!==expected)out.push(el.getAttribute('data-wind-i18n')+': '+el.textContent);
    }
    return out;
  });
  if(mismatches.length)throw new Error('Mixed language labels: '+mismatches.join(', '));
  if(errors.length)throw new Error('Browser errors: '+errors.join(', '));
  await page.evaluate(()=>{
    window.gregalWindRunner.start=async task=>{
      const snapshot={source:'winds',busy:true,state:'working',task,nodes:[{id:'builder',name:'Tramuntana',state:'working',output:''}],events:[]};
      window.gregalWindDesk.update(snapshot);
      await new Promise(resolve=>{window.__finishWindLocale=()=>{window.gregalWindDesk.update({...snapshot,busy:false,state:'done',nodes:[{...snapshot.nodes[0],state:'completed',output:'My saved result'}]});resolve();};});
    };
  });
  await page.locator('#windWorkStart').click();
  await page.waitForFunction(()=>document.getElementById('windWorkSummary').textContent==='In progress');
  await language('ca');
  await assertText('#windWorkSummary','En marxa');
  await page.evaluate(()=>window.__finishWindLocale());
  await language('en');
  await assertText('#windWorkSummary','Task completed');
  await assertText('#windWorkStatus','');
  await assertText('#windWorkSteps .wind-work-preview','Open the result ↗');
  if(!await page.locator('#windWorkSteps').textContent().then(t=>t.includes('My saved result')))throw new Error('Result content changed');
  await page.screenshot({path:'output/winds-english.png'});
})(page);
  await (async page => {
  const result=await page.evaluate(async()=>{
    const outputText=Array.from({length:180},(_,i)=>'Result line '+i).join('\n');
    const snapshot={source:'winds',busy:false,state:'done',task:'A stable task',nodes:[{id:'builder',name:'Tramuntana',state:'completed',output:outputText}],events:[]};
    window.gregalWindDesk.update(snapshot);
    const details=document.querySelector('#windWorkSteps details');
    const output=details.querySelector('.wind-work-output');
    const preview=details.querySelector('button');
    preview.focus(); output.scrollTop=120;
    const range=document.createRange();range.setStart(output.firstChild,0);range.setEnd(output.firstChild,6);
    getSelection().removeAllRanges();getSelection().addRange(range);
    let outputMutations=0;
    const observer=new MutationObserver(records=>{outputMutations+=records.length;});observer.observe(output,{childList:true,characterData:true,subtree:true});
    for(let i=0;i<50;i++)window.gregalWindDesk.update({...snapshot,events:[{type:'tool',time:String(i),message:'Activity '+i}]});
    await Promise.resolve();observer.disconnect();
    const stable=details===document.querySelector('#windWorkSteps details') && output===document.querySelector('.wind-work-output');
    const focused=document.activeElement===preview;
    const selected=getSelection().toString()==='Result';
    const scrolled=output.scrollTop===120;
    details.open=false;window.gregalWindDesk.update(snapshot);
    return {stable,focused,selected,scrolled,outputMutations,staysClosed:!details.open};
  });
  if(!result.stable || !result.focused || !result.selected || !result.scrolled || result.outputMutations || !result.staysClosed)throw new Error(JSON.stringify(result));
  const card=await page.locator('.wind-desk-card').first().elementHandle();
  await page.locator('#windsList button').first().click();
  if(!await card.evaluate(el=>el===document.querySelector('.wind-desk-card')))throw new Error('Unchanged character cards were replaced');
  await page.locator('#windWorkTask').fill('Keyboard task');
  await page.locator('#windWorkTask').press('Control+Enter');
  await page.waitForFunction(()=>document.getElementById('windWorkSummary').textContent==='In progress');
  await page.evaluate(()=>window.__finishWindLocale());
  await page.locator('#windWorkJump').click();
  if(!await page.locator('#windWorkTitle').evaluate(el=>el===document.activeElement))throw new Error('Current work shortcut did not focus the task');
})(page);
}
