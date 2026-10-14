import { isolatedHTML } from './web-artifact.js';
const $ = id => document.getElementById(id);
const ca = () => document.documentElement.lang === 'ca';
const labels = () => ca() ? {title:'Preview',url:'Adreça web',open:'Obre',reload:'Actualitza',external:'Obre externament',close:'Tanca preview',desktop:'Escriptori',mobile:'Mòbil',empty:'Cap web oberta',invalid:'Adreça no vàlida o no permesa',loaded:'Navegació completada',loading:'Carregant',note:'Si la web bloqueja la preview, obre-la externament.',offer:'Obre la web detectada',resize:'Amplada de la preview',max:'Pantalla sencera',restore:'Surt de la pantalla sencera'} : {title:'Preview',url:'Web address',open:'Open',reload:'Reload',external:'Open externally',close:'Close preview',desktop:'Desktop',mobile:'Mobile',empty:'No website open',invalid:'Invalid or disallowed address',loaded:'Navigation completed',loading:'Loading',note:'If the website blocks the preview, open it externally.',offer:'Open detected website',resize:'Preview width',max:'Full screen',restore:'Exit full screen'};
const pane = document.createElement('aside');
pane.id='livePreview';pane.hidden=true;
pane.innerHTML=`<div id="previewDivider" role="separator" tabindex="0" aria-orientation="vertical"></div><header><strong>Preview</strong><button id="previewMax" type="button" aria-pressed="false">⤢</button><button id="previewExternal" type="button">↗</button><button id="previewClose" type="button">×</button></header><form id="previewForm"><input id="previewUrl" type="url" required placeholder="http://localhost:3000"><button id="previewGo" type="submit">→</button><button id="previewReload" type="button">↻</button></form><div class="preview-modes"><button id="previewDesktop" type="button" aria-pressed="true"></button><button id="previewMobile" type="button" aria-pressed="false"></button><button id="previewOffer" type="button" hidden></button></div><div id="previewStatus" role="status"></div><div class="preview-viewport"><iframe id="previewContent" sandbox="allow-scripts allow-forms" referrerpolicy="no-referrer"></iframe><div id="previewEmpty"></div></div><small id="previewNote"></small>`;
$('workbench').append(pane);
const style=document.createElement('style');
style.textContent=`#livePreview[hidden]{display:none}#livePreview{position:relative;flex:0 0 var(--preview-width,480px);width:var(--preview-width,480px);min-width:0;border-left:1px solid var(--line);display:flex;flex-direction:column;background:var(--panel);color:var(--ink);overflow:hidden}#livePreview header,#previewForm,.preview-modes{display:flex;gap:6px;align-items:center;padding:8px 12px}#livePreview header strong{margin-right:auto;font-size:14px}#livePreview button{border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);cursor:pointer;padding:6px 9px}#previewForm input{min-width:0;flex:1;border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);padding:8px;font:12px monospace}.preview-modes button[aria-pressed=true]{border-color:var(--accent);color:var(--accent)}#previewStatus,#previewNote{padding:6px 12px;color:var(--muted);font-size:11px}.preview-viewport{flex:1;min-height:200px;overflow:auto;background:#dce2e3;position:relative}#previewContent{display:block;width:100%;height:100%;border:0;background:white}#livePreview.mobile-preview #previewContent{width:390px;max-width:100%;margin:auto}#previewEmpty{position:absolute;inset:0;display:grid;place-items:center;background:var(--panel);color:var(--muted)}#previewEmpty[hidden]{display:none}#previewDivider{position:absolute;left:-3px;top:0;bottom:0;width:9px;cursor:col-resize;z-index:2;touch-action:none}#previewDivider::after{content:'';position:absolute;left:3px;top:50%;width:3px;height:44px;margin-top:-22px;border-radius:3px;background:var(--line-hi);opacity:0;transition:opacity .15s}#livePreview:hover #previewDivider::after,#previewDivider:focus-visible::after,#livePreview.resizing #previewDivider::after{opacity:1;background:var(--accent)}#livePreview.resizing iframe{pointer-events:none}#livePreview.maximized{position:fixed;inset:0;width:auto;flex:none;z-index:70;border-left:0;box-shadow:0 0 0 1px var(--line)}#livePreview.maximized #previewDivider{display:none}body.desktop #livePreview.maximized{top:var(--preview-top,38px)}#previewMax[aria-pressed=true]{border-color:var(--accent);color:var(--accent)}@media(max-width:1000px){#livePreview{position:fixed;inset:92px 0 0 auto;width:min(100%,560px);z-index:30;box-shadow:-4px 0 16px #0003}#previewDivider{display:none}}`;
document.head.append(style);
style.textContent+='#previewForm[hidden]{display:none}';
let current='',offered='',artifact='',dismissed=false,maxed=false;
let currentTitle='',pendingHTMLTimer=null,pendingHTML='';
export function previewURL(raw){
  try{const u=new URL(raw);if(!['http:','https:'].includes(u.protocol)||u.username||u.password||u.origin===location.origin)return null;return u.href;}catch{return null;}
}
function show(on){pane.hidden=!on;$('previewToggle').setAttribute('aria-pressed',String(on));}
function setTitle(value){currentTitle=typeof value==='string'?value.trim().slice(0,200):'';pane.querySelector('header strong').textContent=currentTitle||labels().title;}
function cancelPendingHTML(){if(pendingHTMLTimer!==null)clearTimeout(pendingHTMLTimer);pendingHTMLTimer=null;pendingHTML='';}
function queueHTML(html){cancelPendingHTML();pendingHTML=html;pendingHTMLTimer=setTimeout(()=>{const latest=pendingHTML;pendingHTMLTimer=null;pendingHTML='';if(latest===artifact)$('previewContent').srcdoc=isolatedHTML(latest);},0);}
function navigate(raw){const url=previewURL(raw);if(!url){$('previewStatus').textContent=labels().invalid;return;}cancelPendingHTML();dismissed=false;artifact='';current=url;$('previewForm').hidden=false;setTitle('');$('previewContent').removeAttribute('srcdoc');$('previewUrl').value=url;$('previewEmpty').hidden=true;$('previewStatus').textContent=labels().loading;$('previewContent').src=url;render();show(true);}
function render(){const l=labels();for(const [id,key] of [['previewUrl','url'],['previewGo','open'],['previewReload','reload'],['previewExternal','external'],['previewClose','close']]){$(id).title=l[key];$(id).setAttribute('aria-label',l[key]);}setTitle(currentTitle);$('previewContent').title=l.title;$('previewDesktop').textContent=l.desktop;$('previewMobile').textContent=l.mobile;$('previewEmpty').textContent=l.empty;$('previewNote').textContent=artifact?(ca()?'HTML generat · revisa el codi abans de publicar-lo.':'Generated HTML · review the code before publishing.'):l.note;$('previewOffer').textContent=l.offer;$('previewDivider').setAttribute('aria-label',l.resize);$('previewExternal').disabled=!current;const mx=maxed;$('previewMax').title=mx?l.restore:l.max;$('previewMax').setAttribute('aria-label',mx?l.restore:l.max);}
function maximize(on){maxed=on;const tabs=document.getElementById&&document.getElementById('sessionTabs');if(on&&tabs&&tabs.getBoundingClientRect)pane.style.setProperty('--preview-top',Math.round(tabs.getBoundingClientRect().bottom)+'px');pane.classList.toggle('maximized',on);$('previewMax').setAttribute('aria-pressed',String(on));$('previewMax').textContent=on?'⤡':'⤢';render();}
function dismiss(){dismissed=true;maximize(false);show(false);}
document.addEventListener('keydown',e=>{if(e.key==='Escape'&&maxed){e.stopPropagation();maximize(false);}},true);
$('previewToggle').onclick=()=>{if(pane.hidden){dismissed=false;show(true);}else dismiss();};
$('previewClose').onclick=dismiss;
$('previewMax').onclick=()=>maximize(!maxed);
$('previewForm').onsubmit=e=>{e.preventDefault();navigate($('previewUrl').value);};
$('previewReload').onclick=()=>{if(artifact){cancelPendingHTML();$('previewContent').srcdoc=isolatedHTML(artifact);}else if(current)navigate(current);};
$('previewExternal').onclick=()=>{if(current)window.open(current,'_blank','noopener,noreferrer');};
$('previewContent').onload=()=>{if(current)$('previewStatus').textContent=labels().loaded;};
for(const mobile of [false,true])$(mobile?'previewMobile':'previewDesktop').onclick=()=>{pane.classList.toggle('mobile-preview',mobile);$('previewMobile').setAttribute('aria-pressed',String(mobile));$('previewDesktop').setAttribute('aria-pressed',String(!mobile));};
function width(value,save=true){const w=Math.max(320,Math.min(innerWidth*.9,value));pane.style.setProperty('--preview-width',`${w}px`);$('previewDivider').setAttribute('aria-valuenow',String(Math.round(w)));if(save){try{localStorage.setItem('gregal_preview_width',String(Math.round(w)));}catch{}}}
try{const saved=+localStorage.getItem('gregal_preview_width');if(saved>0)width(saved,false);}catch{}
$('previewDivider').onpointerdown=e=>{e.preventDefault();e.currentTarget.setPointerCapture(e.pointerId);pane.classList.add('resizing');};
$('previewDivider').onpointermove=e=>{if(e.currentTarget.hasPointerCapture(e.pointerId))width(innerWidth-e.clientX);};
$('previewDivider').onpointerup=$('previewDivider').onpointercancel=()=>pane.classList.remove('resizing');
$('previewDivider').ondblclick=()=>{pane.style.removeProperty('--preview-width');try{localStorage.removeItem('gregal_preview_width');}catch{}};
$('previewDivider').onkeydown=e=>{if(['ArrowLeft','ArrowRight'].includes(e.key)){e.preventDefault();width(pane.getBoundingClientRect().width+(e.key==='ArrowLeft'?24:-24));}};
window.gregalPreview={open:navigate,offer(raw){const url=previewURL(raw);if(!url)return;const host=new URL(url).hostname;if(!['localhost','127.0.0.1','[::1]'].includes(host))return;offered=url;$('previewOffer').hidden=false;$('previewOffer').title=url;}};
window.gregalPreview.openHTML=html=>{cancelPendingHTML();dismissed=false;artifact=html;current='';$('previewForm').hidden=true;setTitle('');$('previewUrl').value='';$('previewContent').removeAttribute('src');queueHTML(html);$('previewEmpty').hidden=true;$('previewStatus').textContent=ca()?'Prototip HTML':'HTML prototype';render();show(true);};
window.gregalPreview.begin=()=>{dismissed=false;};
window.gregalPreview.request=(request,automatic=false)=>{
  if(!request||typeof request.content!=='string'||!request.content.trim()||request.content.length>200000||(automatic&&dismissed))return false;
  if(request.kind==='url'){
    const url=previewURL(request.content);if(!url||!['localhost','127.0.0.1','[::1]'].includes(new URL(url).hostname))return false;navigate(url);
  }else if(request.kind==='html')window.gregalPreview.openHTML(request.content);
  else if(request.kind==='markdown'){
    const body=window.gregal.md(request.content);
    window.gregalPreview.openHTML('<!doctype html><html><head><meta charset="utf-8"><style>body{margin:0;padding:28px;color:#1F2433;background:#fff;font:15px/1.65 system-ui}h1{font-size:26px}h2{font-size:21px}h3{font-size:17px}pre{overflow:auto;background:#EEF0F5;padding:14px}code{font-family:monospace}table{border-collapse:collapse;width:100%}td,th{border:1px solid #D5DAE4;padding:8px}a{color:#4F5BE8}img{max-width:100%}</style></head><body>'+body+'</body></html>');
  }else return false;
  $('previewForm').hidden=request.kind!=='url';
  setTitle(request.title);
  if(request.kind==='markdown'){$('previewStatus').textContent=ca()?'Document Markdown':'Markdown document';$('previewNote').textContent='';}
  return true;
};
$('previewOffer').onclick=()=>navigate(offered);
document.addEventListener('gregal:idioma',render);render();
