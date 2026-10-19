import { isolatedHTML } from './web-artifact.js';
const $ = id => document.getElementById(id);
const ca = () => document.documentElement.lang === 'ca';
const labels = () => ca() ? {title:'Preview',url:'Adreça web',open:'Obre',reload:'Actualitza',external:'Obre externament',close:'Tanca preview',desktop:'Escriptori',mobile:'Mòbil',empty:'Cap web oberta',invalid:'Adreça no vàlida o no permesa',loaded:'Navegació completada',loading:'Carregant',note:'Si la web bloqueja la preview, obre-la externament.',offer:'Obre la web detectada',resize:'Amplada de la preview',max:'Pantalla sencera',restore:'Surt de la pantalla sencera',tabs:'Pàgines d’aquesta sessió',newTab:'Pestanya nova',closeTab:'Tanca la pestanya'} : {title:'Preview',url:'Web address',open:'Open',reload:'Reload',external:'Open externally',close:'Close preview',desktop:'Desktop',mobile:'Mobile',empty:'No website open',invalid:'Invalid or disallowed address',loaded:'Navigation completed',loading:'Loading',note:'If the website blocks the preview, open it externally.',offer:'Open detected website',resize:'Preview width',max:'Full screen',restore:'Exit full screen',tabs:'Pages in this session',newTab:'New tab',closeTab:'Close tab'};
const pane = document.createElement('aside');
pane.id='livePreview';pane.hidden=true;
pane.innerHTML=`<div id="previewDivider" role="separator" tabindex="0" aria-orientation="vertical"></div><header><strong>Preview</strong><button id="previewMax" type="button" aria-pressed="false">⤢</button><button id="previewExternal" type="button">↗</button><button id="previewClose" type="button">×</button></header><div class="preview-tab-bar"><div id="previewTabs" role="tablist"></div><button id="previewNewTab" type="button">+</button></div><form id="previewForm"><input id="previewUrl" type="url" required placeholder="http://localhost:3000"><button id="previewGo" type="submit">→</button><button id="previewReload" type="button">↻</button></form><div class="preview-modes"><button id="previewDesktop" type="button" aria-pressed="true"></button><button id="previewMobile" type="button" aria-pressed="false"></button><button id="previewOffer" type="button" hidden></button></div><div id="previewStatus" role="status"></div><div class="preview-viewport" id="previewViewport"><iframe id="previewContent" class="preview-content" hidden sandbox="allow-scripts allow-forms" referrerpolicy="no-referrer"></iframe><div id="previewEmpty"></div></div><small id="previewNote"></small>`;
$('workbench').append(pane);
const style=document.createElement('style');
style.textContent=`#livePreview[hidden]{display:none}#livePreview{position:relative;flex:0 0 var(--preview-width,480px);width:var(--preview-width,480px);min-width:0;border-left:1px solid var(--line);display:flex;flex-direction:column;background:var(--panel);color:var(--ink);overflow:hidden}#livePreview header,#previewForm,.preview-modes{display:flex;gap:6px;align-items:center;padding:8px 12px}#livePreview header strong{margin-right:auto;font-size:14px}#livePreview button{border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);cursor:pointer;padding:6px 9px}#previewForm input{min-width:0;flex:1;border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);padding:8px;font:12px monospace}.preview-modes button[aria-pressed=true]{border-color:var(--accent);color:var(--accent)}#previewStatus,#previewNote{padding:6px 12px;color:var(--muted);font-size:11px}.preview-viewport{flex:1;min-height:200px;overflow:auto;background:#dce2e3;position:relative}.preview-content{display:block;width:100%;height:100%;border:0;background:white}#livePreview.mobile-preview .preview-content{width:390px;max-width:100%;margin:auto}#previewEmpty{position:absolute;inset:0;display:grid;place-items:center;background:var(--panel);color:var(--muted)}#previewEmpty[hidden]{display:none}#previewDivider{position:absolute;left:-3px;top:0;bottom:0;width:9px;cursor:col-resize;z-index:2;touch-action:none}#previewDivider::after{content:'';position:absolute;left:3px;top:50%;width:3px;height:44px;margin-top:-22px;border-radius:3px;background:var(--line-hi);opacity:0;transition:opacity .15s}#livePreview:hover #previewDivider::after,#previewDivider:focus-visible::after,#livePreview.resizing #previewDivider::after{opacity:1;background:var(--accent)}#livePreview.resizing iframe{pointer-events:none}#livePreview.maximized{position:fixed;inset:0;width:auto;flex:none;z-index:70;border-left:0;box-shadow:0 0 0 1px var(--line)}#livePreview.maximized #previewDivider{display:none}body.desktop #livePreview.maximized{top:var(--preview-top,38px)}#previewMax[aria-pressed=true]{border-color:var(--accent);color:var(--accent)}@media(max-width:1000px){#livePreview{position:fixed;inset:92px 0 0 auto;width:min(100%,560px);z-index:30;box-shadow:-4px 0 16px #0003}#previewDivider{display:none}}`;
document.head.append(style);
style.textContent+='#previewForm[hidden],.preview-content[hidden]{display:none}.preview-tab-bar{display:flex;align-items:center;min-height:36px;border-block:1px solid var(--line)}#previewTabs{display:flex;gap:4px;flex:1;min-width:0;overflow-x:auto;padding:4px 6px}#livePreview .preview-tab{display:flex;align-items:center;gap:6px;max-width:200px;padding:5px 8px;white-space:nowrap}#livePreview .preview-tab[aria-selected=true]{background:var(--surface-hi,var(--bg));border-color:var(--accent);color:var(--accent)}.preview-tab-label{overflow:hidden;text-overflow:ellipsis}.preview-tab-close{padding:0 3px;border-radius:3px}#previewNewTab{margin-right:6px}';
const sessions = new Map();
const storageKey = 'gregal_preview_sessions';
let selectedSession = window.gregalSession || 'default', sequence = 0;
let unusedFrame = $('previewContent'), tabSignature = '';
function session(id = selectedSession) {
  id = String(id || 'default');
  if (!sessions.has(id)) sessions.set(id, { tabs: [], active: '', open: false, dismissed: false, offered: '', mobile: false, maxed: false });
  return sessions.get(id);
}
function active(s = session()) { return s.tabs.find(tab => tab.id === s.active); }
function persist() {
  try {
    const saved = {};
    for (const [id, s] of sessions) saved[id] = { ...s, tabs: s.tabs.map(({id, kind, content, title}) => ({id,kind,content,title})) };
    sessionStorage.setItem(storageKey, JSON.stringify(saved));
  } catch (e) { /* A storage quota must never interrupt navigation or a run. */ }
}
try {
  const saved = JSON.parse(sessionStorage.getItem(storageKey) || '{}');
  for (const [id, data] of Object.entries(saved)) {
    if (!data || !Array.isArray(data.tabs)) continue;
    const tabs = data.tabs.filter(t => t && typeof t.id === 'string' && ['url','html','markdown','empty'].includes(t.kind) && typeof t.content === 'string' && t.content.length <= 200000)
      .filter(t => t.kind !== 'url' || previewURL(t.content)).map(t => ({...t, title:String(t.title || '').slice(0,200), frame:null, timer:null}));
    sessions.set(id, {tabs,active:data.active,open:!!data.open,dismissed:!!data.dismissed,offered:'',mobile:!!data.mobile,maxed:false});
  }
} catch (e) {}
export function previewURL(raw){
  try{const u=new URL(raw);if(!['http:','https:'].includes(u.protocol)||u.username||u.password||u.origin===location.origin)return null;return u.href;}catch{return null;}
}
function cancelPending(tab) { if (tab?.timer != null) clearTimeout(tab.timer); if (tab) tab.timer = null; }
function frameFor(tab) {
  if (tab.frame) return tab.frame;
  const frame = unusedFrame || document.createElement('iframe');
  unusedFrame = null;
  frame.id = 'previewContent-' + tab.id;
  frame.className = 'preview-content';
  frame.setAttribute('sandbox', 'allow-scripts allow-forms');
  frame.setAttribute('referrerpolicy', 'no-referrer');
  frame.setAttribute('title', tab.title || labels().title);
  frame.hidden = true;
  $('previewViewport').append(frame);
  tab.frame = frame;
  frame.onload = () => { if (active() === tab && tab.kind === 'url') $('previewStatus').textContent = labels().loaded; };
  return frame;
}
function applyContent(tab, deferred = false) {
  cancelPending(tab);
  const frame = frameFor(tab);
  if (tab.kind === 'url') { frame.removeAttribute('srcdoc'); frame.src = tab.content; }
  else if (tab.kind === 'html' || tab.kind === 'markdown') {
    frame.removeAttribute('src');
    const apply = () => { tab.timer = null; frame.srcdoc = isolatedHTML(tab.content); };
    if (deferred) tab.timer = setTimeout(apply, 0); else apply();
  }
}
function newTab(s = session()) {
  const tab = {id:'p' + Date.now().toString(36) + '-' + (++sequence),kind:'empty',content:'',title:'',frame:null,timer:null};
  s.tabs.push(tab); s.active = tab.id;
  return tab;
}
function show(on) { session().open = on; pane.hidden = !on; $('previewToggle').setAttribute('aria-pressed',String(on)); }
function tabLabel(tab) {
  if (tab.title) return tab.title;
  if (tab.kind === 'url') { const u = new URL(tab.content); return u.host + (u.pathname === '/' ? '' : u.pathname); }
  return tab.kind === 'empty' ? labels().newTab : tab.kind === 'markdown' ? 'Markdown' : 'HTML';
}
function render() {
  const l = labels(), s = session(), tab = active(s);
  for (const state of sessions.values()) for (const item of state.tabs) if (item.frame) {
    item.frame.hidden = item !== tab;
    item.frame.id = item === tab ? 'previewContent' : 'previewContent-' + item.id;
  }
  if (tab && !tab.frame && tab.kind !== 'empty') { applyContent(tab); tab.frame.id='previewContent'; tab.frame.hidden=false; }
  $('previewTabs').setAttribute('aria-label',l.tabs);
  const signature=JSON.stringify([selectedSession,s.active,l.newTab,s.tabs.map(t=>[t.id,tabLabel(t)])]);
  if (signature!==tabSignature) {
  tabSignature=signature;
  $('previewTabs').replaceChildren();
  s.tabs.forEach(item => {
    const button = document.createElement('button');
    button.type='button'; button.className='preview-tab'; button.setAttribute('role','tab');
    button.setAttribute('aria-selected',String(item===tab)); button.title=tabLabel(item);
    button.tabIndex=item===tab?0:-1;
    const label=document.createElement('span'); label.className='preview-tab-label'; label.textContent=tabLabel(item);
    const close=document.createElement('span'); close.className='preview-tab-close'; close.textContent='×';
    close.setAttribute('role','button'); close.setAttribute('aria-label',l.closeTab); close.title=l.closeTab;
    button.append(label,close);
    button.onclick=e=>{ if(e.target===close){closeTab(item.id);return;} s.active=item.id;render();persist(); };
    button.onkeydown=e=>{
      const index=s.tabs.indexOf(item);
      if(e.key==='Delete'){e.preventDefault();closeTab(item.id);}
      else if(['ArrowLeft','ArrowRight','Home','End'].includes(e.key)){
        e.preventDefault();
        const next=e.key==='Home'?0:e.key==='End'?s.tabs.length-1:(index+(e.key==='ArrowRight'?1:-1)+s.tabs.length)%s.tabs.length;
        s.active=s.tabs[next].id;render();$('previewTabs').children[next]?.focus();persist();
      }
    };
    $('previewTabs').append(button);
  });
  }
  pane.querySelector('header strong').textContent=tab?.title || l.title;
  for(const [id,key] of [['previewUrl','url'],['previewGo','open'],['previewReload','reload'],['previewExternal','external'],['previewClose','close'],['previewNewTab','newTab']]){$(id).title=l[key];$(id).setAttribute('aria-label',l[key]);}
  $('previewForm').hidden=!!tab && !['url','empty'].includes(tab.kind);
  $('previewUrl').value=tab?.kind==='url'?tab.content:'';
  $('previewDesktop').textContent=l.desktop;$('previewMobile').textContent=l.mobile;
  $('previewDesktop').setAttribute('aria-pressed',String(!s.mobile));$('previewMobile').setAttribute('aria-pressed',String(s.mobile));
  $('previewEmpty').textContent=l.empty;$('previewEmpty').hidden=!!tab && tab.kind!=='empty';
  $('previewOffer').textContent=l.offer;$('previewOffer').hidden=!s.offered;$('previewOffer').title=s.offered;
  $('previewExternal').disabled=tab?.kind!=='url';
  $('previewStatus').textContent=!tab || tab.kind==='empty'?'':tab.kind==='url'?tab.content:tab.kind==='markdown'?(ca()?'Document Markdown':'Markdown document'):(ca()?'Prototip HTML':'HTML prototype');
  $('previewNote').textContent=!tab || tab.kind==='url'?l.note:tab.kind==='markdown'?'':ca()?'HTML generat · revisa el codi abans de publicar-lo.':'Generated HTML · review the code before publishing.';
  $('previewDivider').setAttribute('aria-label',l.resize);
  pane.classList.toggle('mobile-preview',s.mobile);pane.classList.toggle('maximized',s.maxed);
  $('previewMax').setAttribute('aria-pressed',String(s.maxed));$('previewMax').textContent=s.maxed?'⤡':'⤢';
  $('previewMax').title=s.maxed?l.restore:l.max;$('previewMax').setAttribute('aria-label',$('previewMax').title);
  show(s.open);
}
function closeTab(id) {
  const s=session(), index=s.tabs.findIndex(t=>t.id===id);if(index<0)return;
  const [tab]=s.tabs.splice(index,1);cancelPending(tab);tab.frame?.remove();
  if(s.active===id)s.active=s.tabs[Math.min(index,s.tabs.length-1)]?.id || '';
  render();persist();
}
function navigate(raw, id = selectedSession, automatic = false) {
  const url=previewURL(raw);if(!url){if(id===selectedSession)$('previewStatus').textContent=labels().invalid;return false;}
  const s=session(id);
  let tab=automatic?s.tabs.find(t=>t.kind==='url'&&new URL(t.content).origin===new URL(url).origin):active(s);
  if(!tab)tab=active(s)?.kind==='empty'?active(s):newTab(s);
  const changed=tab.kind!=='url'||tab.content!==url;
  tab.kind='url';tab.content=url;tab.title='';s.active=tab.id;s.dismissed=false;s.open=true;
  if(changed||!tab.frame)applyContent(tab);
  if(id===selectedSession)render();persist();return true;
}
function openHTML(html, id = selectedSession, kind = 'html', title = '') {
  const s=session(id);let tab=active(s);
  if(!tab || tab.kind==='url')tab=newTab(s);
  tab.kind=kind;tab.content=html;tab.title=typeof title==='string'?title.trim().slice(0,200):'';
  s.dismissed=false;s.open=true;applyContent(tab,true);if(id===selectedSession)render();persist();
}
function maximize(on){const s=session();s.maxed=on;const tabs=document.getElementById('sessionTabs');if(on&&tabs?.getBoundingClientRect)pane.style.setProperty('--preview-top',Math.round(tabs.getBoundingClientRect().bottom)+'px');render();}
function dismiss(){const s=session();s.dismissed=true;s.maxed=false;show(false);persist();}
function switchSession(id) { selectedSession=String(id || 'default');render(); }
function forgetSession(id) { const s=sessions.get(id);if(s)for(const tab of s.tabs){cancelPending(tab);tab.frame?.remove();}sessions.delete(id);if(id===selectedSession)render();persist(); }
document.addEventListener('keydown',e=>{if(e.key==='Escape'&&session().maxed){e.stopPropagation();maximize(false);}},true);
document.addEventListener('gregal:session',e=>switchSession(e.detail.id));
document.addEventListener('gregal:session-closed',e=>forgetSession(e.detail.id));
$('previewToggle').onclick=()=>{if(pane.hidden){session().dismissed=false;show(true);persist();}else dismiss();};
$('previewClose').onclick=dismiss;$('previewMax').onclick=()=>maximize(!session().maxed);
$('previewNewTab').onclick=()=>{newTab();show(true);render();$('previewUrl').focus();persist();};
$('previewForm').onsubmit=e=>{e.preventDefault();navigate($('previewUrl').value);};
$('previewReload').onclick=()=>{const tab=active();if(tab && tab.kind!=='empty')applyContent(tab);};
$('previewExternal').onclick=()=>{const tab=active();if(tab?.kind==='url')window.open(tab.content,'_blank','noopener,noreferrer');};
for(const mobile of [false,true])$(mobile?'previewMobile':'previewDesktop').onclick=()=>{session().mobile=mobile;render();persist();};
function width(value,save=true){const w=Math.max(320,Math.min(innerWidth*.9,value));pane.style.setProperty('--preview-width',`${w}px`);$('previewDivider').setAttribute('aria-valuenow',String(Math.round(w)));if(save){try{localStorage.setItem('gregal_preview_width',String(Math.round(w)));}catch{}}}
try{const saved=+localStorage.getItem('gregal_preview_width');if(saved>0)width(saved,false);}catch{}
$('previewDivider').onpointerdown=e=>{e.preventDefault();e.currentTarget.setPointerCapture(e.pointerId);pane.classList.add('resizing');};
$('previewDivider').onpointermove=e=>{if(e.currentTarget.hasPointerCapture(e.pointerId))width(innerWidth-e.clientX);};
$('previewDivider').onpointerup=$('previewDivider').onpointercancel=()=>pane.classList.remove('resizing');
$('previewDivider').ondblclick=()=>{pane.style.removeProperty('--preview-width');try{localStorage.removeItem('gregal_preview_width');}catch{}};
$('previewDivider').onkeydown=e=>{if(['ArrowLeft','ArrowRight'].includes(e.key)){e.preventDefault();width(pane.getBoundingClientRect().width+(e.key==='ArrowLeft'?24:-24));}};
window.gregalPreview={
  open(raw,id=selectedSession){navigate(raw,id);},
  openHTML(html,id=selectedSession){openHTML(html,id);},
  begin(id=selectedSession){session(id).dismissed=false;},
  switchSession, forgetSession,
  newTab(){newTab();show(true);render();persist();},
  selectTab(id){if(session().tabs.some(t=>t.id===id)){session().active=id;render();persist();}},
  closeTab,
  getState(id=selectedSession){const s=session(id);return {active:s.active,open:s.open,tabs:s.tabs.map(({id,kind,content,title})=>({id,kind,content,title}))};},
  offer(raw,id=selectedSession){const url=previewURL(raw);if(!url||!['localhost','127.0.0.1','[::1]'].includes(new URL(url).hostname))return;const s=session(id);if(s.offered===url)return;s.offered=url;if(id===selectedSession)render();},
  request(request,automatic=false,id=selectedSession){
    const s=session(id);
    if(!request||typeof request.content!=='string'||!request.content.trim()||request.content.length>200000||(automatic&&s.dismissed))return false;
    if(request.kind==='url'){
      const url=previewURL(request.content);if(!url||!['localhost','127.0.0.1','[::1]'].includes(new URL(url).hostname))return false;
      navigate(url,id,true);active(s).title=String(request.title||'').trim().slice(0,200);
    }else if(request.kind==='html')openHTML(request.content,id,'html',request.title);
    else if(request.kind==='markdown'){
      const body=window.gregal.md(request.content);
      openHTML('<!doctype html><html><head><meta charset="utf-8"><style>body{margin:0;padding:28px;color:#1F2433;background:#fff;font:15px/1.65 system-ui}h1{font-size:26px}h2{font-size:21px}h3{font-size:17px}pre{overflow:auto;background:#EEF0F5;padding:14px}code{font-family:monospace}table{border-collapse:collapse;width:100%}td,th{border:1px solid #D5DAE4;padding:8px}a{color:#4F5BE8}img{max-width:100%}</style></head><body>'+body+'</body></html>',id,'markdown',request.title);
    }else return false;
    if(id===selectedSession)render();persist();return true;
  }
};
$('previewOffer').onclick=()=>navigate(session().offered,selectedSession,true);
document.addEventListener('gregal:idioma',render);render();
