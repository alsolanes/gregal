// A permanent app rail, contextual conversations, and project tools beside chat.
(function(root) {
  'use strict';
  const $=id=>document.getElementById(id);
  const tools={fitxers:['fitxersPage','nav.files'],canvis:['canvisPage','nav.changes'],terminal:['terminalPage','nav.terminal'],navegador:['navegadorPage','nav.browser'],knowledge:['knowledgePage','nav.knowledge']};
  const title=key=>root.gregalT?.(key)||key;
  const body=$('toolDockBody');
  let opener=null;
  for(const [id] of Object.values(tools)) if($(id)) body.append($(id));
  // Calma: una sola barra lateral. El rail (destins i «Més») hi entra sota
  // «Nova conversa», el logo fa de capçalera i les eines del peu (dreceres,
  // preferències) van al peu de la barra. Es mouen els mateixos nodes: els
  // gestors i els selectors (#navRail [data-view]) continuen funcionant.
  const side=$('side'), rail=$('navRail');
  if(side && rail) {
    const brand=side.querySelector('.brand'), quick=side.querySelector('.side-quick'), foot=side.querySelector('.side-foot');
    const logo=rail.querySelector('.rail-brand');
    if(brand && logo) {
      logo.insertAdjacentHTML('beforeend','<span>Gregal</span>');
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
  const windList=$('windsListCard');
  if(windList) $('windContextNav').append(windList);
  function sync(view) {
    const tool=tools[view];
    if(tool && $('toolDock').hidden) opener=document.activeElement;
    $('toolDock').hidden=!tool;
    document.body.classList.toggle('has-tool-dock',!!tool);
    document.body.classList.toggle('shell-winds',view==='vents');
    $('conversationNav').hidden=view==='vents';
    $('windContextNav').hidden=view!=='vents';
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
})(window);
