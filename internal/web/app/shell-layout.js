// A permanent app rail, contextual conversations, and project tools beside chat.
(function(root) {
  'use strict';
  const $=id=>document.getElementById(id);
  const tools={fitxers:['fitxersPage','nav.files'],canvis:['canvisPage','nav.changes'],terminal:['terminalPage','nav.terminal'],navegador:['navegadorPage','nav.browser'],knowledge:['knowledgePage','nav.knowledge']};
  if(root.gregalProduct==='codi') tools.memoria=['memoriaPage','nav.memory'];
  const title=key=>root.gregalT?.(key)||key;
  const body=$('toolDockBody');
  let opener=null;
  for(const [id] of Object.values(tools)) if($(id)) body.append($(id));
  if(tools.memoria) {
    const tab=document.createElement('button');tab.type='button';tab.dataset.view='memoria';tab.dataset.i18n='nav.memory';tab.textContent=title('nav.memory');
    tab.onclick=()=>root.gregal.setView('memoria');
    $('toolDock').querySelector('.tool-dock-tabs').append(tab);
  }
  // Calma: una sola barra lateral. El rail (destins i «Més») hi entra sota
  // «Nova conversa», el logo fa de capçalera i les eines del peu (dreceres,
  // preferències) van al peu de la barra. Es mouen els mateixos nodes: els
  // gestors i els selectors (#navRail [data-view]) continuen funcionant.
  const side=$('side'), rail=$('navRail');
  if(side && rail) {
    const brand=side.querySelector('.brand'), quick=side.querySelector('.side-quick'), foot=side.querySelector('.side-foot');
    const logo=rail.querySelector('.rail-brand');
    if(brand && logo) {
      logo.insertAdjacentHTML('beforeend', root.gregalProduct==='vents' ? '<span>Gregal Vents</span>' : root.gregalProduct==='jocs' ? '<span>Gregal Jocs</span>' : '<span>Gregal</span>');
      brand.prepend(logo);
      const sp=document.createElement('span'); sp.className='calma-sp'; logo.after(sp);
      if($('sideCompact')) sp.after($('sideCompact'));
    }
    if(quick) quick.after(rail);
    if(foot) {
      const tools=document.createElement('div'); tools.className='calma-tools';
      for(const id of ['keysBtn','prefsBtn']) if($(id)) tools.append($(id));
      foot.append(tools);
    }
    const more=$('sideMore')?.querySelector('summary');
    const label=()=>{ if(more) more.dataset.label=title('nav.more'); };
    label(); document.addEventListener('gregal:idioma',label);
  }
  // Capçalera: a la vista només Canvis, Inspector i ⋯; Fitxers i Preview
  // passen al menú ⋯ amb el seu nom (els mateixos botons i gestors).
  const moreMenu=$('moreMenu');
  if(moreMenu) {
    const items=[[document.querySelector('#top .top-meta [data-view="fitxers"]'),'nav.files'],[$('previewToggle'),'menu.preview']];
    for(const [b,key] of items.reverse()) if(b) {
      const icon=b.innerHTML;
      b.className='ghost calma-moved';
      b.innerHTML='<span class="calma-ico">'+icon+'</span> <span data-i18n="'+key+'">'+title(key)+'</span>';
      moreMenu.prepend(b);
    }
  }
  // Una sola barra a dalt (Codi): les pestanyes de sessió entren a la
  // capçalera, darrere del xip de la carpeta. Abans eren dues franges de 40 i
  // 48 px amb el nom del projecte repetit. És el mateix node, així que
  // panels.js hi continua pintant igual; a l'escriptori la capçalera passa a
  // fer de barra de títol (index.html) i a Vents no es toca res.
  const tabs=$('sessionTabs'), crumb=document.querySelector('#top .breadcrumb'), wrap=$('wrap');
  if(tabs && crumb && wrap && root.gregalProduct!=='vents') {
    // En pantalla estreta la capçalera ja es reparteix en graella: allà les
    // pestanyes es queden on eren, en una franja a part.
    const ancora=tabs.nextElementSibling, ample=matchMedia('(min-width:761px)');
    const colloca=()=>{
      if(ample.matches) { if(tabs.parentElement!==$('top')) crumb.after(tabs); }
      else if(tabs.parentElement!==wrap) wrap.insertBefore(tabs,ancora);
      document.body.classList.toggle('tabs-inline',ample.matches);
    };
    colloca(); ample.addEventListener('change',colloca);
  }
  const windList=$('windsListCard');
  if(windList) $('windContextNav').append(windList);
  function sync(view) {
    const tool=tools[view];
    if(tool && $('toolDock').hidden) opener=document.activeElement;
    $('toolDock').hidden=!tool;
    document.body.classList.toggle('has-tool-dock',!!tool);
    document.body.classList.toggle('shell-winds',view==='vents');
    // A Vents, el menú és el dels vents també a les pàgines d'eina
    // (Comparticions, Memòria…): abans s'amagava i la barra lateral es
    // quedava amb tres entrades soltes. «shell-winds» continua lligat a la
    // vista, perquè vesteix la zona central; «winds-side», a la barra.
    const sideVents=view==='vents'||root.gregalProduct==='vents';
    document.body.classList.toggle('winds-side',sideVents);
    $('conversationNav').hidden=sideVents;
    $('windContextNav').hidden=!sideVents;
    $('toolDockTitle').textContent=tool?title(tool[1]):'';
  }
  root.gregalShell={isTool:view=>!!tools[view],sync};
  function closeTool() { root.gregal.setView('agent'); if(opener?.isConnected) opener.focus(); }
  $('toolDockClose').onclick=closeTool;
  $('shellProject').onclick=()=>$('activeWorkspace').click();
  document.addEventListener('click',event=>{
    if(root.gregal?.viewActual?.()==='vents' && event.target.closest('#ventsPage, #windContextNav')) {
      root.gregalWindClickActive=true;
      queueMicrotask(()=>{root.gregalWindClickActive=false;});
    }
  },true);
  document.querySelector('.rail-brand').onclick=e=>{e.preventDefault();root.gregal.setView('agent');};
  document.addEventListener('gregal:idioma',()=>sync(root.gregal.viewActual()));
  document.addEventListener('click',e=>{ if(!$('sideMore').contains(e.target) || e.target.closest('#sideMore button')) $('sideMore').open=false; });
  document.addEventListener('keydown',e=>{
    if(e.key==='Escape' && $('sideMore').open) { $('sideMore').open=false; $('sideMore').querySelector('summary').focus(); return; }
    if(e.key==='Escape' && !$('toolDock').hidden && !e.defaultPrevented && e.target.tagName!=='TEXTAREA' && e.target.tagName!=='INPUT') {
      closeTool();
    }
  });
  root.gregal.setView(root.gregal.viewActual());
  // La barra ja té el logo, el rail i les eines al seu lloc: es pot mostrar.
  document.documentElement.classList.add('shell-ready');
})(window);
