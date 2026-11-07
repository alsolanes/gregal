// Panell del navegador de l'agent, només a l'app d'escriptori. Quan l'eina
// `browser` corre en mode app, el procés principal (desktop/browser-bridge.js)
// demana mostrar aquest panell al costat de la conversa i hi condueix el
// webview per CDP. L'usuari ho veu en directe i hi pot clicar, escriure o
// navegar en qualsevol moment; la conversa (i els permisos que l'agent
// demana) queda a la vista. La partició és la del navegador de l'app: les
// sessions iniciades allà serveixen aquí.
//
// El panell és un navegador de debò: enrere/endavant, recarregar o aturar,
// barra d'adreces (o cerca), zoom, obrir al navegador del sistema, i tres
// mides (normal, minimitzat a una píndola, ampliat a tota la finestra).
// Quan l'agent necessita que l'usuari faci una cosa ell mateix (contrasenya,
// codi, CAPTCHA, triar compte) envia un «passa-m'ho» (handoff): una franja
// amb el que cal fer i «Fet, continua» / «Cancel·la».
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

// agentBrowserState: qui mana. 'show' porta el panell a la vista sense
// canviar-ho; 'handoff' és «et toca a tu» fins que arriba 'handoff-end'.
export function agentBrowserState(state, current) {
  if (state === 'active' || state === 'released' || state === 'handoff') return state;
  if (state === 'handoff-end') return current === 'handoff' ? 'active' : (current || 'active');
  return current || 'active';
}

// normalizeAddress: el que escrius a la barra. Una adreça (amb o sense
// esquema, també localhost:3000) hi va; la resta es cerca.
export function normalizeAddress(raw) {
  const v = String(raw || '').trim();
  if (!v) return '';
  if (/^https?:\/\//i.test(v)) {
    try { const u = new URL(v); return /^https?:$/.test(u.protocol) && !u.username && !u.password ? u.href : ''; } catch (e) { return ''; }
  }
  const local = /^(localhost|127\.0\.0\.1|\[::1\])(:\d+)?(\/|$)/i.test(v);
  if (!/\s/.test(v) && (local || /^[^\s/]+\.[a-z]{2,}(:\d+)?(\/.*)?$/i.test(v))) {
    try { return new URL((local ? 'http://' : 'https://') + v).href; } catch (e) { /* cerca */ }
  }
  return 'https://duckduckgo.com/?q=' + encodeURIComponent(v);
}

// Icones de traç (mateix estil que shell-icons.js).
const ICONS = {
  globe: 'M3 12h18 M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18 M12 3c5 5 5 13 0 18-5-5-5-13 0-18',
  min: 'M5 18h14',
  max: 'M4 9V4h5 M20 9V4h-5 M4 15v5h5 M20 15v5h-5',
  unmax: 'M9 4v5H4 M15 4v5h5 M9 20v-5H4 M15 20v-5h5',
  close: 'M6 6l12 12 M18 6L6 18',
  back: 'M15 5l-7 7 7 7',
  forward: 'M9 5l7 7-7 7',
  reload: 'M20 11a8 8 0 1 0-2.3 5.7 M20 5v6h-6',
  external: 'M14 4h6v6 M20 4l-9 9 M18 14v6H4V6h6',
  lock: 'M6 11h12v9H6z M8.5 11V8a3.5 3.5 0 0 1 7 0v3',
  open: 'M6 11h12v9H6z M8.5 11V8a3.5 3.5 0 0 1 6.8-1.2',
  key: 'M8 11a4 4 0 1 0 0 8 4 4 0 0 0 0-8 M11 12l9-9 M17 6l3 3 M15 8l2 2',
};
const icon = name => '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="' + ICONS[name] + '"/></svg>';

const CSS = `
#workbench{position:relative}
#agentBrowser{position:relative;flex:0 0 var(--agent-browser-width,46%);width:var(--agent-browser-width,46%);min-width:340px;border-left:1px solid var(--line);display:flex;flex-direction:column;background:var(--panel);color:var(--ink);overflow:hidden;container-type:inline-size}
#agentBrowser[hidden],.agent-browser-view[hidden],#agentBrowser [hidden]{display:none!important}
#agentBrowser:not(.min){animation:abOpen .18s ease-out}
@keyframes abOpen{from{opacity:.5;translate:8px 0}to{opacity:1;translate:0 0}}
#agentBrowser .ab-tabs{display:flex;align-items:center;gap:4px;margin:0 10px 5px;min-height:30px}
#agentBrowser .ab-tab-list{display:flex;gap:3px;overflow:auto;scrollbar-width:thin}
#agentBrowser .ab-tab{display:flex;align-items:center;flex:none;max-width:170px;border:1px solid transparent;border-radius:8px;color:var(--muted)}
#agentBrowser .ab-tab:has([aria-pressed="true"]){background:var(--surface-hi);border-color:var(--line);color:var(--ink)}
#agentBrowser .ab-tab-label{min-width:0;padding:5px 8px;border:0;border-radius:7px;background:none;color:inherit;font:inherit;font-size:12px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;cursor:pointer}
#agentBrowser .ab-tab .ab-btn{width:23px;height:24px}
#agentBrowser .ab-tab .ab-btn svg{width:12px;height:12px}
#agentBrowser svg{width:16px;height:16px;display:block}
#agentBrowser .ab-head{display:flex;gap:8px;align-items:center;padding:8px 10px 6px;user-select:none}
#agentBrowser .ab-head .ab-logo{display:grid;place-items:center;width:28px;height:28px;flex:none;border-radius:8px;background:var(--surface-hi);color:var(--ink)}
#agentBrowser .ab-head strong{font-size:var(--t3,14px);font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
#agentBrowser .ab-state{display:inline-flex;align-items:center;gap:6px;min-width:0;margin-right:auto;font:500 11px/1.5 var(--sans,system-ui);color:var(--muted);padding:3px 8px;border-radius:99px;background:var(--surface);white-space:nowrap}
#agentBrowser .ab-state span{overflow:hidden;text-overflow:ellipsis}
#agentBrowser .ab-state i{width:7px;height:7px;flex:none;border-radius:50%;background:var(--faint)}
#agentBrowser .ab-state.active{color:var(--green);background:var(--ok-soft)}
#agentBrowser .ab-state.active i{background:currentColor;animation:abPulse 1.4s ease-in-out infinite}
#agentBrowser .ab-state.handoff{color:var(--amber);background:var(--amber-soft)}
#agentBrowser .ab-state.handoff i{background:var(--amber);animation:abPulse .9s ease-in-out infinite}
@keyframes abPulse{50%{opacity:.3;transform:scale(.75)}}
#agentBrowser .ab-btn{width:28px;height:28px;flex:none;display:inline-flex;align-items:center;justify-content:center;border:1px solid transparent;border-radius:7px;background:transparent;color:var(--muted);cursor:pointer;padding:0;transition:background .12s,color .12s,border-color .12s}
#agentBrowser .ab-btn:hover:not(:disabled){background:var(--surface-hi);color:var(--ink)}
#agentBrowser .ab-btn:focus-visible{outline:none;border-color:var(--accent)}
#agentBrowser .ab-btn:disabled{opacity:.35;cursor:default}
#agentBrowser .ab-btn.ab-close:hover{background:var(--red-soft);color:var(--red,#ff8a85)}
#agentBrowser .ab-bar{display:flex;gap:3px;align-items:center;margin:0 10px;padding:4px;border:1px solid var(--line);border-radius:12px;background:var(--bg)}
#agentBrowser .ab-address{flex:1;min-width:0;display:flex;align-items:center;gap:7px;height:32px;padding:0 9px;border:1px solid var(--line);border-radius:8px;background:var(--surface);transition:border-color .12s,box-shadow .12s}
#agentBrowser .ab-address:focus-within{border-color:var(--accent);box-shadow:0 0 0 3px var(--accent-soft)}
#agentBrowser .ab-address svg{width:13px;height:13px;color:var(--faint);flex:none}
#agentBrowser .ab-address.secure svg{color:var(--green)}
#agentBrowser .ab-address input{flex:1;min-width:0;border:0;outline:0;background:transparent;color:var(--ink);font:13px var(--sans,system-ui);padding:0;text-overflow:ellipsis}
#agentBrowser .ab-address input:focus{text-overflow:clip}
#agentBrowser .ab-zoom{display:flex;align-items:center;flex:none;border-left:1px solid var(--line);padding-left:2px}
#agentBrowser .ab-zoom .ab-btn{width:24px;height:26px;font:13px var(--mono)}
#agentBrowser .ab-zoom .ab-pct{width:auto;padding:0 4px;font:11px var(--mono);min-width:40px}
#agentBrowser .ab-progress{flex:none;height:2px;margin:4px 12px 0;position:relative;overflow:hidden;border-radius:2px}
#agentBrowser .ab-progress.loading::after{content:"";position:absolute;top:0;bottom:0;width:40%;background:linear-gradient(90deg,transparent,var(--accent),transparent);animation:abLoad 1.1s ease-in-out infinite}
@keyframes abLoad{from{left:-40%}to{left:100%}}
#agentBrowser .ab-handoff{display:flex;align-items:center;gap:10px;margin:8px 8px 0;padding:10px 12px;border:1px solid var(--amber);border-radius:10px;background:var(--amber-soft);animation:abIn .22s cubic-bezier(.16,1,.3,1)}
@keyframes abIn{from{opacity:0;transform:translateY(-6px)}}
#agentBrowser .ab-handoff .ab-key{width:30px;height:30px;flex:none;border-radius:8px;display:inline-flex;align-items:center;justify-content:center;background:var(--amber);color:var(--bg)}
#agentBrowser .ab-handoff-text{flex:1;min-width:0;display:flex;flex-direction:column;gap:2px;font-size:var(--t2,12.5px);line-height:1.4}
#agentBrowser .ab-handoff-text b{color:var(--ink);font-weight:620}
#agentBrowser .ab-handoff-text span{color:var(--text);overflow-wrap:anywhere}
#agentBrowser .ab-handoff-text small{padding:0;color:var(--muted);font-size:11px}
#agentBrowser .ab-handoff-actions{display:flex;gap:6px;flex-wrap:wrap;justify-content:flex-end}
#agentBrowser .ab-handoff button{font-weight:600;font-size:12px;font-family:inherit;border-radius:8px;padding:7px 12px;cursor:pointer;white-space:nowrap;transition:filter .12s,background .12s}
#agentBrowser .ab-done{border:1px solid var(--accent);background:var(--accent);color:var(--bg)}
#agentBrowser .ab-done:hover{filter:brightness(1.1)}
#agentBrowser .ab-cancel{border:1px solid var(--line-hi);background:transparent;color:var(--ink)}
#agentBrowser .ab-cancel:hover{background:var(--surface-hi)}
#agentBrowser .ab-view{flex:1;min-height:0;margin:4px 10px 8px;border:1px solid var(--line-hi);border-radius:12px;overflow:hidden;background:#fff;position:relative}
#agentBrowser.handoff .ab-view{border-color:var(--amber);box-shadow:0 0 0 3px var(--amber-soft)}
.agent-browser-view{position:absolute;inset:0;width:100%;height:100%;border:0}
#agentBrowser .ab-note{padding:0 12px 8px;color:var(--faint);font-size:11px;line-height:1.4}
#agentBrowser .ab-mini{display:none}
#agentBrowserGrip{position:absolute;left:-3px;top:0;bottom:0;width:9px;cursor:col-resize;z-index:3}
#agentBrowserGrip::after{content:"";position:absolute;left:3px;top:0;bottom:0;width:3px;background:transparent;transition:background .12s}
#agentBrowserGrip:hover::after,#agentBrowserGrip.dragging::after{background:var(--accent)}
body.ab-resizing{cursor:col-resize;user-select:none}body.ab-resizing .agent-browser-view{pointer-events:none}
#agentBrowser.max{position:absolute;inset:0;width:auto;min-width:0;flex:none;z-index:25;border-left:0;box-shadow:0 0 0 1px var(--line)}
#agentBrowser.max #agentBrowserGrip{display:none}
#agentBrowser.min{position:fixed;right:18px;bottom:18px;top:auto;left:auto;width:auto;min-width:0;max-width:min(360px,calc(100vw - 36px));height:auto;flex:none;z-index:31;border:1px solid var(--line-hi);border-radius:99px;background:var(--surface);box-shadow:0 12px 36px var(--ombra);overflow:visible;container-type:normal;transition:transform .18s cubic-bezier(.16,1,.3,1),border-color .18s}
#agentBrowser.min:hover{transform:translateY(-2px);border-color:var(--accent)}
#agentBrowser.min.handoff{border-color:var(--amber);box-shadow:0 0 0 4px var(--amber-soft),0 12px 36px var(--ombra)}
#agentBrowser.min>*:not(.ab-mini):not(.ab-view){display:none!important}
#agentBrowser.min .ab-view{position:fixed;left:-20000px;top:0;width:1100px;height:800px;margin:0}
#agentBrowser.min .ab-mini{display:flex;align-items:center;gap:9px;padding:7px 14px 7px 9px;border:0;background:transparent;color:var(--ink);cursor:pointer;font:inherit;max-width:360px}
#agentBrowser .ab-mini img,#agentBrowser .ab-mini .ab-logo{width:22px;height:22px;flex:none;border-radius:6px;display:inline-flex;align-items:center;justify-content:center;background:var(--accent-soft)}
#agentBrowser .ab-mini b{font-size:var(--t2,12.5px);font-weight:600;white-space:nowrap}
#agentBrowser .ab-mini .ab-mini-page{font:11px var(--mono);color:var(--muted);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;max-width:180px}
#agentBrowser .ab-mini i{width:8px;height:8px;border-radius:50%;background:var(--faint);flex:none}
#agentBrowser.min.active .ab-mini i{background:var(--accent);animation:abPulse 1.4s ease-in-out infinite}
#agentBrowser.min.handoff .ab-mini i{background:var(--amber);animation:abPulse .9s ease-in-out infinite}
@container (max-width:420px){
  #agentBrowser .ab-head{gap:5px}
  #agentBrowser .ab-handoff{flex-wrap:wrap}
  #agentBrowser .ab-handoff-text{flex-basis:calc(100% - 40px)}
  #agentBrowser .ab-handoff-actions{margin-left:auto}
  #agentBrowser .ab-bar .ab-btn{width:25px}
  #agentBrowser .ab-bar .ab-pct{width:auto;min-width:34px}
}
@media(max-width:1000px){#agentBrowser:not(.min){position:fixed;inset:92px 0 0 auto;width:min(100%,560px);min-width:0;z-index:31;box-shadow:-4px 0 16px #0003}#agentBrowser.max{inset:92px 0 0 0;width:100%}#agentBrowserGrip{display:none}}
@media(prefers-reduced-motion:reduce){
  #agentBrowser:not(.min){animation:none}
  #agentBrowser .ab-state i,#agentBrowser .ab-mini i,#agentBrowser .ab-handoff,#agentBrowser .ab-progress.loading::after{animation:none}
  #agentBrowser.min,#agentBrowser .ab-btn,#agentBrowser .ab-address{transition:none}
}
`;

function init() {
  let pane = null, selected = window.gregalSession || 'default';
  // size: 'normal', 'min' (píndola) o 'max' (tota la finestra). No es desa:
  // un panell que reapareix ho fa a la mida de sempre.
  let size = 'normal';
  const views = new Map();
  let tabsOwner, shownTab, tabsCount = 0;
  const style = document.createElement('style');
  style.textContent = CSS;
  document.head.append(style);
  const setLabel = (id, key) => { const el = $(id); if (el) { el.title = T(key); el.setAttribute('aria-label', el.title); } };
  const hostOf = url => { try { return new URL(url).host; } catch (e) { return ''; } };
  const render = () => {
    if (!pane) return;
    const owner = views.get(selected);
    const current = owner?.activeTab;
    pane.hidden = !owner?.open;
    for (const [id, session] of views) {
      for (const tab of session.tabs) tab.view.hidden = id !== selected || tab !== current;
      session.view.id = id === selected ? 'agentBrowserView' : 'agentBrowserView-' + id;
    }
    const state = owner?.state || 'released';
    const handoff = owner?.handoff || null;
    renderTabs(owner);
    pane.classList.toggle('min', size === 'min');
    pane.classList.toggle('max', size === 'max');
    pane.classList.toggle('active', state === 'active');
    pane.classList.toggle('handoff', !!handoff);
    pane.querySelector('header strong').textContent = T('agentBrowser.title');
    const badge = pane.querySelector('.ab-state');
    badge.querySelector('span').textContent = T(handoff ? 'agentBrowser.handoff' : state === 'released' ? 'agentBrowser.released' : 'agentBrowser.active');
    badge.classList.toggle('active', !handoff && state !== 'released');
    badge.classList.toggle('handoff', !!handoff);
    // Barra d'adreces: no es trepitja el que l'usuari escriu.
    const url = $('agentBrowserUrl');
    if (current !== shownTab || document.activeElement !== url) url.value = current?.url || '';
    shownTab = current;
    url.title = current?.url || '';
    const secure = /^https:/i.test(current?.url || '');
    const addr = pane.querySelector('.ab-address');
    addr.classList.toggle('secure', secure);
    addr.querySelector('.ab-lock').innerHTML = icon(secure ? 'lock' : 'open');
    addr.querySelector('.ab-lock').title = current?.url ? T(secure ? 'agentBrowser.secure' : 'agentBrowser.insecure') : '';
    $('agentBrowserBack').disabled = !current?.canBack;
    $('agentBrowserForward').disabled = !current?.canForward;
    const reload = $('agentBrowserReload');
    reload.innerHTML = icon(current?.loading ? 'close' : 'reload');
    setLabel('agentBrowserReload', current?.loading ? 'agentBrowser.stop' : 'agentBrowser.reload');
    $('agentBrowserProgress').classList.toggle('loading', !!current?.loading);
    $('agentBrowserExternal').disabled = !current?.url;
    $('agentBrowserMax').innerHTML = icon(size === 'max' ? 'unmax' : 'max');
    setLabel('agentBrowserMax', size === 'max' ? 'agentBrowser.restore' : 'agentBrowser.maximize');
    setLabel('agentBrowserMin', 'agentBrowser.minimize');
    setLabel('agentBrowserClose', 'agentBrowser.close');
    setLabel('agentBrowserBack', 'agentBrowser.back');
    setLabel('agentBrowserForward', 'agentBrowser.forward');
    setLabel('agentBrowserExternal', 'agentBrowser.external');
    url.placeholder = T('agentBrowser.address');
    url.setAttribute('aria-label', url.placeholder);
    // «Passa-m'ho»: què cal fer i els dos botons.
    $('agentBrowserHandoff').hidden = !handoff;
    $('agentBrowserHandoffTitle').textContent = T('agentBrowser.handoffTitle');
    $('agentBrowserHandoffMsg').textContent = handoff?.message || '';
    $('agentBrowserHandoffHint').textContent = T('agentBrowser.handoffHint');
    $('agentBrowserHandoffDone').textContent = T('agentBrowser.handoffDone');
    $('agentBrowserHandoffCancel').textContent = T('agentBrowser.handoffCancel');
    pane.querySelector('small.ab-note').textContent = T('agentBrowser.note');
    // Píndola del panell minimitzat.
    const mini = $('agentBrowserRestore');
    mini.title = T('agentBrowser.open');
    mini.querySelector('b').textContent = handoff ? T('agentBrowser.handoff') : T('agentBrowser.title');
    mini.querySelector('.ab-mini-page').textContent = current?.title || hostOf(current?.url || '');
    $('agentBrowserGrip').setAttribute('aria-label', T('agentBrowser.width'));
    setLabel('agentBrowserZoomOut', 'agentBrowser.zoomOut');
    setLabel('agentBrowserZoomIn', 'agentBrowser.zoomIn');
    setLabel('agentBrowserZoomReset', 'agentBrowser.zoomReset');
    $('agentBrowserZoomReset').textContent = Math.round(zoom * 100) + '%';
    pane.setAttribute('aria-label', T('agentBrowser.title'));
  };
  const setSize = next => { size = next; render(); };
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
  const setZoom = z => { zoom = z; keep(KEY_Z, z === 1 ? null : z); for (const item of views.values()) for (const tab of item.tabs) applyZoom(tab); render(); };
  const report = item => {
    try {
      const id = item.view.getWebContentsId();
      if (id && id !== item.reported) { item.reported = id; D.agentBrowserReady(id, item.scope); }
    } catch (e) {}
  };
  const currentItem = () => views.get(selected)?.activeTab;
  const answerHandoff = done => {
    const item = views.get(selected);
    if (!item?.handoff) return;
    const id = item.handoff.id;
    item.handoff = null;
    item.state = 'active';
    if (typeof D.agentBrowserHandoff === 'function') D.agentBrowserHandoff(id, done);
    render();
  };
  const renderTabs = owner => {
    const list = $('agentBrowserTabs');
    list.setAttribute('aria-label', T('agentBrowser.tabs'));
    setLabel('agentBrowserNewTab', 'agentBrowser.newTab');
    if (tabsOwner !== owner || tabsCount !== (owner?.tabs.length || 0)) {
      list.replaceChildren(...(owner?.tabs || []).map(tab => tab.tabElement));
      tabsOwner = owner;
      tabsCount = owner?.tabs.length || 0;
    }
    for (const tab of owner?.tabs || []) {
      const name = tab.title || hostOf(tab.url) || T(tab === owner ? 'agentBrowser.agentTab' : 'agentBrowser.newTab');
      tab.tabLabel.textContent = name;
      tab.tabLabel.title = tab === owner ? T('agentBrowser.agentTab') + ' · ' + name : name;
      tab.tabLabel.setAttribute('aria-pressed', String(owner.activeTab === tab));
      if (tab.tabClose) {
        tab.tabClose.title = T('agentBrowser.closeTab') + ' · ' + name;
        tab.tabClose.setAttribute('aria-label', tab.tabClose.title);
      }
    }
  };
  const makeTabButton = (owner, tab) => {
    const wrap = document.createElement('div');
    wrap.className = 'ab-tab';
    const label = document.createElement('button');
    label.type = 'button';
    label.className = 'ab-tab-label';
    label.onclick = () => { owner.activeTab = tab; render(); };
    wrap.append(label);
    tab.tabElement = wrap;
    tab.tabLabel = label;
    if (owner === tab) return;
    const close = document.createElement('button');
    close.type = 'button';
    close.className = 'ab-btn ab-close';
    close.innerHTML = icon('close');
    close.onclick = () => {
      const i = owner.tabs.indexOf(tab);
      owner.tabs.splice(i, 1);
      if (owner.activeTab === tab) owner.activeTab = owner.tabs[i - 1];
      tab.view.remove();
      render();
      owner.activeTab.tabLabel.focus?.();
    };
    wrap.append(close);
    tab.tabClose = close;
  };
  const build = () => {
    pane = document.createElement('aside'); pane.id = 'agentBrowser';
    pane.innerHTML = '<div id="agentBrowserGrip" role="separator" aria-orientation="vertical" tabindex="0"></div>'
      + '<header class="ab-head"><span class="ab-logo">' + icon('globe') + '</span><strong></strong><span class="ab-state" role="status"><i></i><span></span></span>'
      + '<button id="agentBrowserMin" class="ab-btn" type="button">' + icon('min') + '</button>'
      + '<button id="agentBrowserMax" class="ab-btn" type="button">' + icon('max') + '</button>'
      + '<button id="agentBrowserClose" class="ab-btn ab-close" type="button">' + icon('close') + '</button></header>'
      + '<div class="ab-tabs"><div id="agentBrowserTabs" class="ab-tab-list" role="group"></div><button id="agentBrowserNewTab" class="ab-btn" type="button">+</button></div>'
      + '<div class="ab-bar"><button id="agentBrowserBack" class="ab-btn" type="button">' + icon('back') + '</button>'
      + '<button id="agentBrowserForward" class="ab-btn" type="button">' + icon('forward') + '</button>'
      + '<button id="agentBrowserReload" class="ab-btn" type="button">' + icon('reload') + '</button>'
      + '<label class="ab-address"><span class="ab-lock">' + icon('open') + '</span><input id="agentBrowserUrl" type="text" spellcheck="false" autocomplete="off"></label>'
      + '<span class="ab-zoom"><button id="agentBrowserZoomOut" class="ab-btn" type="button">−</button><button id="agentBrowserZoomReset" class="ab-btn ab-pct" type="button">100%</button><button id="agentBrowserZoomIn" class="ab-btn" type="button">+</button></span>'
      + '<button id="agentBrowserExternal" class="ab-btn" type="button">' + icon('external') + '</button></div>'
      + '<div class="ab-progress" id="agentBrowserProgress"></div>'
      + '<div class="ab-handoff" id="agentBrowserHandoff" role="alert" hidden><span class="ab-key">' + icon('key') + '</span>'
      + '<div class="ab-handoff-text"><b id="agentBrowserHandoffTitle"></b><span id="agentBrowserHandoffMsg"></span><small id="agentBrowserHandoffHint"></small></div>'
      + '<div class="ab-handoff-actions"><button id="agentBrowserHandoffCancel" class="ab-cancel" type="button"></button><button id="agentBrowserHandoffDone" class="ab-done" type="button"></button></div></div>'
      + '<div class="ab-view"></div><small class="ab-note"></small>'
      + '<button id="agentBrowserRestore" class="ab-mini" type="button"><i></i><span class="ab-logo">' + icon('globe') + '</span><b></b><span class="ab-mini-page"></span></button>';
    $('workbench').append(pane);
    $('agentBrowserClose').onclick = () => { const item = views.get(selected); if (item) item.open = false; size = 'normal'; render(); };
    $('agentBrowserNewTab').onclick = () => {
      const owner = views.get(selected);
      if (!owner) return;
      const tab = createView(selected, undefined);
      makeTabButton(owner, tab);
      owner.tabs.push(tab);
      owner.activeTab = tab;
      render();
      $('agentBrowserUrl').focus?.();
    };
    $('agentBrowserMin').onclick = () => setSize('min');
    $('agentBrowserMax').onclick = () => setSize(size === 'max' ? 'normal' : 'max');
    $('agentBrowserRestore').onclick = () => setSize('normal');
    pane.querySelector('.ab-head').addEventListener('dblclick', e => { if (!e.target.closest('button')) setSize(size === 'max' ? 'normal' : 'max'); });
    pane.addEventListener('keydown', e => { if (e.key === 'Escape' && size === 'max' && e.target !== $('agentBrowserUrl')) setSize('normal'); });
    initGrip();
    $('agentBrowserZoomOut').onclick = () => setZoom(nextZoom(zoom, -1));
    $('agentBrowserZoomIn').onclick = () => setZoom(nextZoom(zoom, 1));
    $('agentBrowserZoomReset').onclick = () => setZoom(1);
    const nav = fn => () => { const item = currentItem(); if (item) { try { fn(item.view, item); } catch (e) {} } };
    $('agentBrowserBack').onclick = nav(v => v.goBack());
    $('agentBrowserForward').onclick = nav(v => v.goForward());
    $('agentBrowserReload').onclick = nav((v, item) => item.loading ? v.stop() : v.reload());
    $('agentBrowserExternal').onclick = () => { const u = currentItem()?.url; if (u) window.open(u, '_blank', 'noopener'); };
    const url = $('agentBrowserUrl');
    url.addEventListener('focus', () => url.select?.());
    url.addEventListener('keydown', e => {
      if (e.key === 'Escape') { url.value = currentItem()?.url || ''; url.blur(); return; }
      if (e.key !== 'Enter') return;
      e.preventDefault();
      const target = normalizeAddress(url.value);
      const item = currentItem();
      if (!target || !item) return;
      url.value = target; url.blur();
      try { item.view.loadURL(target); } catch (x) {}
    });
    $('agentBrowserHandoffDone').onclick = () => answerHandoff(true);
    $('agentBrowserHandoffCancel').onclick = () => answerHandoff(false);
  };
  const ensure = (id, scope) => {
    if (views.has(id)) {
      const item = views.get(id);
      if (scope && scope !== item.scope) { item.scope = scope; item.reported = 0; }
      return item;
    }
    if (!pane) build();
    const item = createView(id, scope);
    item.tabs = [item];
    item.activeTab = item;
    makeTabButton(item, item);
    views.set(id, item);
    return item;
  };
  // undefined scope is a manual tab. It never registers a CDP target.
  const createView = (id, scope) => {
    const view = document.createElement('webview'); view.className = 'agent-browser-view';
    view.setAttribute('partition', 'persist:gregal-browser'); view.setAttribute('src', 'about:blank');
    view.setAttribute('allowpopups', '');
    const item = { view, scope, reported: 0, url: '', title: '', state: 'active', open: false, loading: false, canBack: false, canForward: false, handoff: null };
    pane.querySelector('.ab-view').append(view);
    const refresh = () => { if (id === selected) render(); };
    const syncHistory = () => {
      try { item.canBack = view.canGoBack(); item.canForward = view.canGoForward(); } catch (e) {}
    };
    view.addEventListener('dom-ready', () => { if (item.scope !== undefined) report(item); applyZoom(item); syncHistory(); refresh(); });
    const setURL = e => { item.url = e.url === 'about:blank' ? '' : e.url; syncHistory(); refresh(); };
    view.addEventListener('did-navigate', e => { setURL(e); applyZoom(item); });
    view.addEventListener('did-navigate-in-page', setURL);
    view.addEventListener('did-start-loading', () => { item.loading = true; refresh(); });
    view.addEventListener('did-stop-loading', () => { item.loading = false; syncHistory(); refresh(); });
    view.addEventListener('page-title-updated', e => { item.title = e.title || ''; refresh(); });
    view.addEventListener('destroyed', () => { item.reported = 0; });
    return item;
  };
  D.onAgentBrowser(next => {
    const scoped = next && typeof next === 'object';
    const id = scoped ? String(next.session || selected) : selected;
    const state = scoped ? next.state : next;
    const item = ensure(id, scoped && next.session ? id : '');
    item.state = agentBrowserState(state, item.state);
    if (state === 'handoff') {
      item.handoff = { id: String(next.id || ''), message: String(next.message || '') };
      item.open = true;
      item.activeTab = item;
      // Et toca a tu: el panell es fa visible (no minimitzat) i, si la
      // finestra no té el focus, avís del sistema.
      if (id === selected && size === 'min') size = 'normal';
      if (typeof D.notify === 'function' && !document.hasFocus?.()) D.notify(T('agentBrowser.handoffTitle'), item.handoff.message);
    } else if (state === 'handoff-end') {
      if (!next.id || item.handoff?.id === String(next.id)) item.handoff = null;
    }
    if (state === 'show' || state === 'active') item.open = true;
    render(); report(item);
  });
  document.addEventListener('gregal:session', e => { selected = e.detail.id; render(); });
  document.addEventListener('gregal:session-closed', e => {
    const item = views.get(e.detail.id);
    for (const tab of item?.tabs || []) tab.view.remove();
    views.delete(e.detail.id);
    render();
  });
  document.addEventListener('gregal:idioma', render);
}

if (D && typeof D.onAgentBrowser === 'function' && typeof D.agentBrowserReady === 'function' && document.getElementById('workbench')) init();
