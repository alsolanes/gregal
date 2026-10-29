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

// Amplada del panell: la tria l'usuari arrossegant la vora esquerra i es
// recorda. El màxim es calcula cada cop (si es desa en un monitor gran i
// després s'obre el portàtil, la conversa s'ha de continuar veient).
export const AB_MIN = 340, AB_CHAT_MIN = 380;
export function clampBrowserWidth(px, total) {
  const max = Math.max(AB_MIN, (total || 0) - AB_CHAT_MIN);
  return Math.round(Math.min(Math.max(Number(px) || 0, AB_MIN), max));
}

// Zoom de la pàgina del panell (−, 100 %, +), del 50 % al 200 %.
export const AB_ZOOMS = [0.5, 0.67, 0.75, 0.8, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2];
export function nextZoom(current, dir) {
  const z = Number(current) || 1;
  if (dir === 0) return 1;
  if (dir > 0) return AB_ZOOMS.find(v => v > z + 0.001) ?? AB_ZOOMS[AB_ZOOMS.length - 1];
  return [...AB_ZOOMS].reverse().find(v => v < z - 0.001) ?? AB_ZOOMS[0];
}

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
    + '#agentBrowser .ab-zoom{display:flex;gap:2px;align-items:center}#agentBrowser .ab-zoom button{padding:2px 7px;font:12px var(--mono)}'
    + '#agentBrowserGrip{position:absolute;left:-3px;top:0;bottom:0;width:9px;cursor:col-resize;z-index:3}'
    + '#agentBrowserGrip::after{content:"";position:absolute;left:3px;top:0;bottom:0;width:3px;background:transparent;transition:background .12s}'
    + '#agentBrowserGrip:hover::after,#agentBrowserGrip.dragging::after{background:var(--accent)}'
    + 'body.ab-resizing{cursor:col-resize;user-select:none}body.ab-resizing .agent-browser-view{pointer-events:none}'
    + '@media(max-width:1000px){#agentBrowser{position:fixed;inset:92px 0 0 auto;width:min(100%,560px);z-index:31;box-shadow:-4px 0 16px #0003}#agentBrowserGrip{display:none}}';
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
    $('agentBrowserGrip').setAttribute('aria-label', T('agentBrowser.width'));
    $('agentBrowserZoomOut').title = T('agentBrowser.zoomOut');
    $('agentBrowserZoomIn').title = T('agentBrowser.zoomIn');
    $('agentBrowserZoomReset').title = T('agentBrowser.zoomReset');
    $('agentBrowserZoomReset').textContent = Math.round(zoom * 100) + '%';
    pane.setAttribute('aria-label', T('agentBrowser.title'));
  };
  // Amplada (vora esquerra arrossegable; doble clic torna a la de sempre).
  const KEY_W = 'gregal_agent_browser_width', KEY_Z = 'gregal_agent_browser_zoom';
  const setWidth = px => {
    const total = $('workbench')?.getBoundingClientRect().width || window.innerWidth;
    const v = clampBrowserWidth(px, total);
    document.documentElement.style.setProperty('--agent-browser-width', v + 'px');
    return v;
  };
  const saved = k => { try { return localStorage.getItem(k); } catch (e) { return null; } };
  const keep = (k, v) => { try { v == null ? localStorage.removeItem(k) : localStorage.setItem(k, String(v)); } catch (e) {} };
  if (Number(saved(KEY_W))) setWidth(Number(saved(KEY_W)));
  window.addEventListener('resize', () => { if (Number(saved(KEY_W))) setWidth(Number(saved(KEY_W))); });
  const initGrip = () => {
    const grip = $('agentBrowserGrip');
    const widthAt = x => pane.getBoundingClientRect().right - x;
    grip.addEventListener('pointerdown', e => {
      e.preventDefault();
      grip.classList.add('dragging'); document.body.classList.add('ab-resizing');
      const move = e2 => setWidth(widthAt(e2.clientX));
      const up = e2 => {
        window.removeEventListener('pointermove', move); window.removeEventListener('pointerup', up);
        grip.classList.remove('dragging'); document.body.classList.remove('ab-resizing');
        keep(KEY_W, setWidth(widthAt(e2.clientX)));
      };
      window.addEventListener('pointermove', move); window.addEventListener('pointerup', up);
    });
    grip.addEventListener('dblclick', () => { keep(KEY_W, null); document.documentElement.style.removeProperty('--agent-browser-width'); });
    grip.addEventListener('keydown', e => {
      const step = e.key === 'ArrowLeft' ? 32 : e.key === 'ArrowRight' ? -32 : 0;
      if (!step) return;
      e.preventDefault();
      keep(KEY_W, setWidth(pane.getBoundingClientRect().width + step));
    });
  };
  // Zoom de la pàgina: el mateix per a tots els panells, i es manté en navegar.
  let zoom = Number(saved(KEY_Z)) || 1;
  const applyZoom = item => { try { item.view.setZoomFactor(zoom); } catch (e) {} };
  const setZoom = z => { zoom = z; keep(KEY_Z, z === 1 ? null : z); for (const item of views.values()) applyZoom(item); render(); };
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
      pane.innerHTML='<div id="agentBrowserGrip" role="separator" aria-orientation="vertical" tabindex="0"></div><header><strong></strong><span class="ab-state" role="status"></span><span class="ab-zoom"><button id="agentBrowserZoomOut" type="button">−</button><button id="agentBrowserZoomReset" type="button">100%</button><button id="agentBrowserZoomIn" type="button">+</button></span><button id="agentBrowserClose" type="button">×</button></header><div class="ab-url" id="agentBrowserUrl"></div><div class="ab-view"></div><small></small>';
      $('workbench').append(pane);
      $('agentBrowserClose').onclick=()=>{const item=views.get(selected);if(item)item.open=false;render();};
      initGrip();
      $('agentBrowserZoomOut').onclick=()=>setZoom(nextZoom(zoom,-1));
      $('agentBrowserZoomIn').onclick=()=>setZoom(nextZoom(zoom,1));
      $('agentBrowserZoomReset').onclick=()=>setZoom(1);
    }
    const view=document.createElement('webview');view.className='agent-browser-view';
    view.setAttribute('partition','persist:gregal-browser');view.setAttribute('src','about:blank');
    const item={view,scope,reported:0,url:'',state:'active',open:false};views.set(id,item);
    pane.querySelector('.ab-view').append(view);
    view.addEventListener('dom-ready',()=>{report(item);applyZoom(item);});
    const setURL=e=>{item.url=e.url==='about:blank'?'':e.url;if(id===selected)render();};
    view.addEventListener('did-navigate',e=>{setURL(e);applyZoom(item);});view.addEventListener('did-navigate-in-page',setURL);
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
