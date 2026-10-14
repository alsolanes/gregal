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
  let pane = null, view = null, reported = 0, state = 'active';
  const style = document.createElement('style');
  style.textContent = '#agentBrowser{position:relative;flex:0 0 var(--agent-browser-width,46%);width:var(--agent-browser-width,46%);min-width:340px;border-left:1px solid var(--line);display:flex;flex-direction:column;background:var(--panel);color:var(--ink);overflow:hidden}'
    + '#agentBrowser header{display:flex;gap:8px;align-items:center;padding:8px 12px}#agentBrowser header strong{font-size:14px}'
    + '#agentBrowser .ab-state{margin-right:auto;font:11px var(--mono);color:var(--muted);padding:2px 8px;border:1px solid var(--line-hi);border-radius:99px}'
    + '#agentBrowser .ab-state.active{color:var(--accent-hi,var(--accent));border-color:var(--accent)}'
    + '#agentBrowser header button{border:1px solid var(--line-hi);border-radius:4px;background:var(--bg);color:var(--ink);cursor:pointer;padding:4px 9px}'
    + '#agentBrowser .ab-url{padding:0 12px 8px;font:11px var(--mono);color:var(--muted);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}'
    + '#agentBrowser .ab-view{flex:1;min-height:200px;background:#fff;position:relative}#agentBrowserView{position:absolute;inset:0;width:100%;height:100%;border:0}'
    + '#agentBrowser small{padding:6px 12px;color:var(--muted);font-size:11px}'
    + '@media(max-width:1000px){#agentBrowser{position:fixed;inset:92px 0 0 auto;width:min(100%,560px);z-index:31;box-shadow:-4px 0 16px #0003}}';
  document.head.append(style);

  const render = () => {
    if (!pane) return;
    pane.querySelector('header strong').textContent = T('agentBrowser.title');
    const st = pane.querySelector('.ab-state');
    st.textContent = T(state === 'released' ? 'agentBrowser.released' : 'agentBrowser.active');
    st.classList.toggle('active', state !== 'released');
    const close = $('agentBrowserClose');
    close.title = T('agentBrowser.close');
    close.setAttribute('aria-label', close.title);
    pane.querySelector('small').textContent = T('agentBrowser.note');
    pane.setAttribute('aria-label', T('agentBrowser.title'));
  };
  const report = () => {
    try {
      const id = view.getWebContentsId();
      if (id && id !== reported) { reported = id; D.agentBrowserReady(id); }
    } catch (e) { /* encara no té guest */ }
  };
  const build = () => {
    pane = document.createElement('aside');
    pane.id = 'agentBrowser';
    pane.innerHTML = '<header><strong></strong><span class="ab-state" role="status"></span><button id="agentBrowserClose" type="button">×</button></header>'
      + '<div class="ab-url" id="agentBrowserUrl"></div><div class="ab-view"></div><small></small>';
    $('workbench').append(pane);
    view = document.createElement('webview');
    view.id = 'agentBrowserView';
    view.setAttribute('partition', 'persist:gregal-browser');
    view.setAttribute('src', 'about:blank');
    pane.querySelector('.ab-view').append(view);
    const setURL = url => { const el = $('agentBrowserUrl'); el.textContent = url === 'about:blank' ? '' : url; el.title = el.textContent; };
    view.addEventListener('dom-ready', report);
    view.addEventListener('did-navigate', e => setURL(e.url));
    view.addEventListener('did-navigate-in-page', e => setURL(e.url));
    // Una recàrrega del guest en canvia l'id: es torna a registrar.
    view.addEventListener('destroyed', () => { reported = 0; });
    $('agentBrowserClose').onclick = () => { pane.hidden = true; };
    render();
  };
  D.onAgentBrowser(next => {
    if (!pane) build();
    state = agentBrowserState(next, state);
    if (next === 'show' || next === 'active') pane.hidden = false;
    render();
    report();
  });
  document.addEventListener('gregal:idioma', render);
}

if (D && typeof D.onAgentBrowser === 'function' && typeof D.agentBrowserReady === 'function' && document.getElementById('workbench')) init();
