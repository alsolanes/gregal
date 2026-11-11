// Captures del xat dels vents (fosc, clar, mòbil) amb una conversa simulada: ratxes, taula,
// missatges propis i un missatge proactiu. Cal el servidor de proves: node scripts/winds-chat-fixture.cjs
// Ús: NODE_PATH=<node_modules amb playwright> node scripts/shots-winds-chat.cjs → output/playwright/b1-web-*.png
const { chromium } = require(process.env.PLAYWRIGHT || 'playwright');
(async () => {
  const b = await chromium.launch({ executablePath: process.env.CHROME || 'C:/Program Files/Google/Chrome/Application/chrome.exe' });
  const page = await b.newPage(); const errors = []; page.on('pageerror', e => errors.push(e.message));
  const prompt = 'Fes-me una comparativa de portàtils lleugers';
  const hist = [
    { from:'user', text:'Hola! Necessito un portàtil nou' },
    { from:'user', text:prompt },
    { id:'m-1', from:'pet', speaker_id:'marinada', speaker_name:'Marinada', text:'Perfecte, en miro tres que pesin menys d\'1,3 kg.' },
    { id:'m-2', from:'pet', speaker_id:'marinada', speaker_name:'Marinada', text:'| Model | Pes | Bateria |\n|---|---|---|\n| Aire 13 | 1,24 kg | 18 h |\n| Zen 14 | 1,19 kg | 15 h |\n| Swift Go | 1,30 kg | 12 h |' },
    { id:'m-3', from:'pet', speaker_id:'marinada', speaker_name:'Marinada', text:'El **Zen 14** és el més equilibrat pel preu. Vols que vigili si baixa?' },
    { from:'user', text:'Sí, avisa\'m si baixa de 900 €' },
    { id:'m-4', from:'pet', speaker_id:'marinada', speaker_name:'Marinada', text:'Sobre el portàtil · Ha baixat a 879 € a la botiga oficial. Ho he comprovat ara.', relatedRunId:42, resultRefs:['res-1'] },
  ];
  const fs=require('fs');const idx=fs.readFileSync(require('path').resolve(__dirname,'../internal/web/index.html'),'utf8').replace('<html lang="ca">','<html lang="ca" data-product="vents">');
  await page.route(u=>new URL(u).pathname==='/'||new URL(u).pathname==='/vents/', r=>r.fulfill({contentType:'text/html',body:idx}));
  await page.route('**/api/**', async route => {
    const req = route.request(), url = new URL(req.url()); let data = {};
    if (url.pathname.endsWith('/stream')) return route.fulfill({ contentType:'text/event-stream', body:'event: done\ndata: '+JSON.stringify({ reply:{wind:'marinada',text:''}, history:hist })+'\n\n' });
    if (url.pathname==='/api/state') data={provider:'fixture',model:'Model de prova',project:'Projecte',cwd:'C:/p',lang:'ca',mode:'code'};
    else if (url.pathname==='/api/winds') data={winds:[{id:'marinada',name:'Marinada',role:'Compres i comparacions',squad:'daily',specialty:'shopping',color:'#EFC69A',style:'psn'}]};
    else if (url.pathname==='/api/winds/squads') data={squads:[{id:'daily',ca:'Dia a dia',en:'Everyday'}]};
    else if (url.pathname==='/api/winds/rotllanes') data={rotllanes:[{id:'chat',name:'Portàtils lleugers',members:['marinada']}]};
    else if (url.pathname==='/api/winds/rotllana') data={history:hist};
    else if (url.pathname==='/api/winds/collective') data={singular:'',plural:'',asked:true};
    else if (url.pathname==='/api/winds/tasks') data={tasks:[]};
    else if (url.pathname==='/api/workspaces') data={current:'C:/p',workspaces:[{name:'Projecte',path:'C:/p'}]};
    else if (url.pathname==='/api/sessions') data=[];
    else if (url.pathname==='/api/sessions/live') data={sessions:[]};
    return route.fulfill({ contentType:'application/json', body:JSON.stringify(data) });
  });
  for (const [theme, w, h, name] of [['dark',1440,960,'dark'],['light',1440,960,'light'],['dark',390,844,'mobile']]) {
    await page.setViewportSize({ width:w, height:h }); await page.goto('http://127.0.0.1:8766/vents/');
    await page.waitForFunction(() => window.gregalWindWorkspace && window.gregalWindRunner);
    await page.evaluate(async t => { (await import('/app/i18n.js')).posa('ca'); document.documentElement.dataset.theme=t; document.body.classList.add('desktop'); window.gregal.setView('vents'); document.getElementById('side').classList.remove('compact','closed'); }, theme);
    if(!(await page.locator('.vw-composer textarea').count())){await page.evaluate(()=>window.gregal.setView('vents'));} await page.evaluate(t=>{document.documentElement.dataset.theme=t;},theme); await page.waitForTimeout(800); await page.keyboard.press('Escape'); await page.waitForTimeout(300); await page.locator('text=Portàtils lleugers >> visible=true').first().dispatchEvent('click');
    try{await page.locator('.vw-message-text').filter({ hasText:'879' }).waitFor({timeout:8000});}catch(e){await page.screenshot({path:require('path').resolve(__dirname,'../output/playwright')+'/b1-web-debug.png'});console.log('ERRS',errors);throw e;}
    await page.waitForTimeout(400);
    await page.screenshot({ path:require('path').resolve(__dirname,'../output/playwright')+'/b1-web-'+name+'.png' });
  }
  const info = await page.evaluate(() => [...document.querySelectorAll('.vw-messages .vw-message')].map(r => { const d=r.querySelector(':scope > div'), cs=getComputedStyle(d); return r.className.replace('vw-message ','')+' bl='+cs.borderBottomLeftRadius+' br='+cs.borderBottomRightRadius+' bg='+cs.backgroundColor; }));
  console.log(JSON.stringify(info)); console.log('errors', errors);
  await b.close();
})().catch(e => { console.error(e); process.exit(1); });
