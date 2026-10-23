// A permanent app rail, contextual conversations, and project tools beside chat.
(function(root) {
  'use strict';
  const $=id=>document.getElementById(id);
  const tools={fitxers:['fitxersPage','nav.files'],canvis:['canvisPage','nav.changes'],terminal:['terminalPage','nav.terminal'],navegador:['navegadorPage','nav.browser'],knowledge:['knowledgePage','nav.knowledge']};
  const title=key=>root.gregalT?.(key)||key;
  const body=$('toolDockBody');
  let opener=null;
  for(const [id] of Object.values(tools)) if($(id)) body.append($(id));
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
  document.addEventListener('click',e=>{ const options=document.querySelector('.composer-options'); if(options && !options.contains(e.target)) options.open=false; });
  document.addEventListener('keydown',e=>{
    const options=document.querySelector('.composer-options');
    if(e.key==='Escape' && options?.open) { options.open=false; options.querySelector('summary').focus(); e.preventDefault(); return; }
    if(e.key==='Escape' && $('sideMore').open) { $('sideMore').open=false; $('sideMore').querySelector('summary').focus(); return; }
    if(e.key==='Escape' && !$('toolDock').hidden && !e.defaultPrevented && e.target.tagName!=='TEXTAREA' && e.target.tagName!=='INPUT') {
      closeTool();
    }
  });
  root.gregal.setView(root.gregal.viewActual());
})(window);
