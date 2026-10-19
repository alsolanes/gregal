// Panell del navegador de l'agent, només a l'app d'escriptori. Quan l'eina
// `browser` corre en mode app, el procés principal (desktop/browser-bridge.js)
// demana mostrar aquest panell al costat de la conversa i hi condueix el
// webview per CDP. L'usuari ho veu en directe i hi pot clicar o escriure en
// qualsevol moment; la conversa (i els permisos que l'agent demana) queda a
// la vista. La partició és la del navegador de l'app: les sessions
// iniciades allà serveixen aquí.
const D = window.gregalDesktop;
const $ = id => document.getElementById(id);
const T = key => (window.gregalT ? window.gregalT(key) : '') || key;

export function agentBrowserState(state, current) {
  // 'show' porta el panell a la vista sense canviar qui mana.
  if (state === 'active' || state === 'released') return state;
  return current || 'active';
}

function init() {
  let pane = null, selected = window.gregalSession || 'default';
  const views = new Map();
  const style = document.createElement('style');
  style.textContent = '#agentBrowser{position:relative;flex:0 0 var(--agent-browser-width,46%);width:var(--agent-browser-width,46%);min-width:340px;border-left:1px solid var(--line);display:flex;flex-direction:column;background:var(--panel);color:var(--ink);overflow:hidden}'
    + '#agentBrowser header{display:flex;gap:8px;align-items:center;padding:8px 12px}#agentBrowser header strong{font-size:14px}'
    + '#agentBrowser .ab-state{margin-right:auto;font:11px var(--mono);color:var(--muted);padding:2px 8px;border:1px solid var(--line-hi);border-radius:99px}'
    + '#agentBrowser .ab-state.active{color:var(--accent-hi,var(--accent));border-color:var(--accent)}'
    + '#agentBrowser header button{border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);cursor:pointer;padding:4px 9px}'
    + '#agentBrowser .ab-url{padding:0 12px 8px;font:11px var(--mono);color:var(--muted);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}'
    + '#agentBrowser .ab-view{flex:1;min-height:200px;background:#fff;position:relative}.agent-browser-view{position:absolute;inset:0;width:100%;height:100%;border:0}'
    + '#agentBrowser small{padding:6px 12px;color:var(--muted);font-size:11px}'
    + '@media(max-width:1000px){#agentBrowser{position:fixed;inset:92px 0 0 auto;width:min(100%,560px);z-index:31;box-shadow:-4px 0 16px #0003}}';
  style.textContent += '#agentBrowser[hidden],.agent-browser-view[hidden]{display:none}';
  document.head.append(style);
  const render = () => {
    if (!pane) return;
    const current = views.get(selected);
    pane.hidden = !current?.open;
    for (const [id, item] of views) {
      item.view.hidden = id !== selected;
      item.view.id = id === selected ? 'agentBrowserView' : 'agentBrowserView-' + id;
    }
    pane.querySelector('header strong').textContent = T('agentBrowser.title');
    const state = current?.state || 'released';
    const badge = pane.querySelector('.ab-state');
    badge.textContent = T(state === 'released' ? 'agentBrowser.released' : 'agentBrowser.active');
    badge.classList.toggle('active', state !== 'released');
    const url = $('agentBrowserUrl'); url.textContent = current?.url || ''; url.title = url.textContent;
    const close = $('agentBrowserClose'); close.title = T('agentBrowser.close'); close.setAttribute('aria-label',close.title);
    pane.querySelector('small').textContent = T('agentBrowser.note');
    pane.setAttribute('aria-label', T('agentBrowser.title'));
  };
  const report = item => {
    try {
      const id = item.view.getWebContentsId();
      if (id && id !== item.reported) { item.reported=id; D.agentBrowserReady(id,item.scope); }
    } catch (e) {}
  };
  const ensure = (id, scope) => {
    if (views.has(id)) {
      const item=views.get(id);
      if (scope && scope!==item.scope) {item.scope=scope;item.reported=0;}
      return item;
    }
    if (!pane) {
      pane=document.createElement('aside');pane.id='agentBrowser';
      pane.innerHTML='<header><strong></strong><span class="ab-state" role="status"></span><button id="agentBrowserClose" type="button">×</button></header><div class="ab-url" id="agentBrowserUrl"></div><div class="ab-view"></div><small></small>';
      $('workbench').append(pane);
      $('agentBrowserClose').onclick=()=>{const item=views.get(selected);if(item)item.open=false;render();};
    }
    const view=document.createElement('webview');view.className='agent-browser-view';
    view.setAttribute('partition','persist:gregal-browser');view.setAttribute('src','about:blank');
    const item={view,scope,reported:0,url:'',state:'active',open:false};views.set(id,item);
    pane.querySelector('.ab-view').append(view);
    view.addEventListener('dom-ready',()=>report(item));
    const setURL=e=>{item.url=e.url==='about:blank'?'':e.url;if(id===selected)render();};
    view.addEventListener('did-navigate',setURL);view.addEventListener('did-navigate-in-page',setURL);
    view.addEventListener('destroyed',()=>{item.reported=0;});
    return item;
  };
  D.onAgentBrowser(next=>{
    const scoped=next && typeof next==='object';
    const id=scoped?String(next.session || 'default'):selected;
    const state=scoped?next.state:next;
    const item=ensure(id,scoped?id:'');
    item.state=agentBrowserState(state,item.state);
    if(state==='show'||state==='active')item.open=true;
    render();report(item);
  });
  document.addEventListener('gregal:session',e=>{selected=e.detail.id;render();});
  document.addEventListener('gregal:session-closed',e=>{const item=views.get(e.detail.id);item?.view.remove();views.delete(e.detail.id);render();});
  document.addEventListener('gregal:idioma',render);
}

if (D && typeof D.onAgentBrowser === 'function' && typeof D.agentBrowserReady === 'function' && document.getElementById('workbench')) init();
