// Pàgina de connectors MCP: estat de cada servidor, autorització OAuth
// (Connecta / Reconnecta), desconnectar, treure i afegir servidors. Script
// clàssic: index.html hi delega des de loadMcpPage amb les seves eines
// (api, traducció i escapament); els tests el carreguen amb require.
(function () {
  'use strict';

  const DEMO_NAME = 'demo';

  function fmt(s, vars) {
    return String(s || '').replace(/\{(\w+)\}/g, (m, k) => (vars && k in vars ? String(vars[k]) : m));
  }

  function label(T, key, fallback) {
    const v = T(key);
    return v && v !== key ? v : fallback;
  }

  // renderHTML pinta l'estat (pur: el test el crida sense DOM).
  function renderHTML(d, T, esc) {
    const servers = (d && d.servers) || [];
    const rows = servers.map(s => {
      const st = s.status || 'error';
      const color = st === 'connected' ? 'var(--green)' : (st === 'needs_auth' ? 'var(--amber)' : 'var(--red)');
      const meta = [];
      meta.push(esc(s.transport || 'stdio') + (s.host ? ' · ' + esc(s.host) : ''));
      if (st === 'connected') {
        meta.push(esc(fmt(label(T, 'mcp.toolsN', '{n} tools'), { n: s.tools || 0 })));
        if (s.prompts) meta.push(esc(fmt(label(T, 'mcp.promptsN', '{n} prompts'), { n: s.prompts })));
      }
      if (s.auth) meta.push(esc(label(T, 'mcp.auth.' + s.auth, s.auth)));
      if (s.readonly) meta.push(esc(label(T, 'mcp.readonly', 'read-only')));
      const remote = s.transport === 'http' || s.transport === 'sse';
      const btn = (act, key, fb) => '<button class="ghost tiny" type="button" data-mcp-act="' + act + '" data-mcp-name="' + esc(s.name) + '">' + esc(label(T, key, fb)) + '</button>';
      const actions = [];
      if (st === 'connected') actions.push(btn('connect', 'mcp.reconnect', 'Reconnect'));
      else actions.push(btn('connect', 'mcp.connect', 'Connect'));
      if (remote && s.auth === 'authorized') actions.push(btn('disconnect', 'mcp.disconnect', 'Disconnect'));
      actions.push(btn('remove', 'mcp.remove', 'Remove'));
      return '<div class="status-line mcp-row" style="padding:10px 0;border-bottom:1px solid var(--line);flex-wrap:wrap;gap:8px">' +
        '<b>' + esc(s.name) + '</b><small style="color:var(--muted)">' + meta.join(' · ') + '</small>' +
        '<span class="spacer"></span><span style="color:' + color + '">' + esc(label(T, 'mcp.status.' + st, st)) + '</span>' +
        '<span style="display:inline-flex;gap:6px">' + actions.join('') + '</span>' +
        (s.error && st !== 'connected' ? '<small style="display:block;width:100%;color:var(--red)">' + esc(s.error) + '</small>' : '') +
        '</div>';
    }).join('');
    const summary = !d || !d.summary || d.summary === 'inactiu' || d.summary === 'inactive' ? label(T, 'mcp.inactive', 'Inactive') : d.summary;
    const tools = ((d && d.tools) || []).map(esc).join(' · ');
    const prompts = ((d && d.prompts) || []).map(p => esc(p.command)).join(' · ');
    const form = '<form class="mcp-add" style="margin-top:18px;display:grid;gap:8px;max-width:640px">' +
      '<b>' + esc(label(T, 'mcp.add', 'Add a server')) + '</b>' +
      '<input name="name" required placeholder="' + esc(label(T, 'mcp.addName', 'Name')) + '">' +
      '<input name="url" placeholder="' + esc(label(T, 'mcp.addUrl', 'URL')) + '">' +
      '<input name="command" placeholder="' + esc(label(T, 'mcp.addCommand', 'or local command')) + '">' +
      '<div style="display:flex;gap:8px;align-items:center"><button class="primary" type="submit">' + esc(label(T, 'mcp.addSave', 'Add')) + '</button>' +
      '<button class="ghost" type="button" data-mcp-act="refresh">' + esc(label(T, 'mcp.refresh', 'Refresh')) + '</button></div>' +
      '<small style="color:var(--muted)">' + esc(label(T, 'mcp.addHint', '')) + '</small></form>';
    // Base de dades de demo: un clic l'afegeix (mentre no hi sigui).
    const demo = hasDemo(d) ? '' : '<div class="mcp-demo" style="margin:12px 0;display:flex;gap:10px;align-items:center;flex-wrap:wrap">' +
      '<button class="primary" type="button" data-mcp-act="demo">' + esc(label(T, 'mcp.demoAdd', 'Add demo database')) + '</button>' +
      '<small style="color:var(--muted);flex:1;min-width:220px">' + esc(label(T, 'mcp.demoHint', '')) + '</small></div>';
    return '<div class="status-line"><b>' + esc(summary) + '</b></div><div class="mcp-msg" role="status" style="margin:8px 0;color:var(--muted)"></div>' +
      (rows || '<div class="activity-empty">' + esc(label(T, 'mcp.empty', 'No MCP servers configured.')) + '</div>') + demo +
      (tools ? '<p class="page-copy" style="margin-top:12px"><b>' + esc(label(T, 'mcp.tools', 'Tools:')) + '</b> ' + tools + '</p>' : '') +
      (prompts ? '<p class="page-copy"><b>' + esc(label(T, 'mcp.prompts', 'Prompts:')) + '</b> ' + prompts + '</p>' : '') +
      form;
  }

  // hasDemo diu si la base de dades de demo ja és entre els servidors.
  function hasDemo(d) {
    return ((d && d.servers) || []).some(s => s.name === DEMO_NAME);
  }

  // demoStep diu què cal fer perquè la demo estigui connectada: res,
  // reconnectar-la o afegir-la (pur: el test el crida amb l'estat).
  function demoStep(d) {
    const s = ((d && d.servers) || []).find(x => x.name === DEMO_NAME);
    if (!s) return 'add';
    return s.status === 'connected' ? 'none' : 'connect';
  }

  // ensureDemo deixa la demo connectada (l'afegeix si cal). Llança un
  // error amb el missatge del servidor si no se'n surt.
  async function ensureDemo(api) {
    const call = async (url, body) => {
      const r = await api(url, body ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) } : undefined);
      const text = await r.text();
      let data = {};
      try { data = JSON.parse(text); } catch (e) { data = { error: text }; }
      if (!r.ok) throw new Error(data.error || text || ('HTTP ' + r.status));
      return data;
    };
    const step = demoStep(await call('/api/mcp'));
    if (step === 'none') return;
    const res = step === 'add' ? await call('/api/mcp/servers', { preset: 'demo' }) : await call('/api/mcp/connect', { name: DEMO_NAME });
    if (res.error) throw new Error(res.error);
  }

  // addBody converteix el formulari en la petició d'afegir.
  function addBody(name, urlText, command) {
    const body = { name: String(name || '').trim() };
    const u = String(urlText || '').trim();
    const c = String(command || '').trim();
    if (u) body.url = u;
    else if (c) {
      const parts = c.match(/"[^"]*"|'[^']*'|\S+/g) || [];
      const clean = parts.map(p => p.replace(/^["']|["']$/g, ''));
      body.command = clean[0] || '';
      body.args = clean.slice(1);
    }
    return body;
  }

  let pollTimer = null;

  async function load(box, deps) {
    if (!box) return;
    const { api, T, esc } = deps;
    const reload = () => load(box, deps);
    let d;
    try {
      const r = await api('/api/mcp');
      if (!r.ok) throw new Error(await r.text());
      d = await r.json();
    } catch (e) {
      box.innerHTML = '<div class="activity-empty">' + esc(label(T, 'mcp.loadFailed', 'Could not load connectors: ')) + esc(e.message || e) + '</div>';
      return;
    }
    box.innerHTML = renderHTML(d, T, esc);
    const msg = box.querySelector('.mcp-msg');
    const say = (text, bad) => { if (msg) { msg.textContent = text; msg.style.color = bad ? 'var(--red)' : 'var(--muted)'; } };
    const post = async (url, body, method) => {
      const r = await api(url, { method: method || 'POST', headers: { 'Content-Type': 'application/json' }, body: body ? JSON.stringify(body) : undefined });
      const text = await r.text();
      let data = {};
      try { data = JSON.parse(text); } catch (e) { data = { error: text }; }
      if (!r.ok) throw new Error(data.error || text || ('HTTP ' + r.status));
      return data;
    };
    box.querySelectorAll('[data-mcp-act]').forEach(el => {
      el.onclick = async () => {
        const act = el.dataset.mcpAct, name = el.dataset.mcpName;
        if (act === 'refresh') return reload();
        el.disabled = true;
        try {
          if (act === 'connect') {
            const res = await post('/api/mcp/connect', { name });
            if (res.auth_url) {
              window.open(res.auth_url, '_blank', 'noopener');
              say(fmt(label(T, 'mcp.authOpened', ''), { name }));
              waitConnected(name, deps, box);
              return;
            }
            say(fmt(label(T, 'mcp.authDone', '{name} connected.'), { name }));
          } else if (act === 'demo') {
            await ensureDemo(api);
            await reload();
            const m = box.querySelector('.mcp-msg');
            if (m) m.textContent = label(T, 'mcp.demoAdded', 'Demo database connected.');
            return;
          } else if (act === 'disconnect') {
            await post('/api/mcp/disconnect', { name });
          } else if (act === 'remove') {
            if (!window.confirm(fmt(label(T, 'mcp.removeConfirm', 'Remove {name}?'), { name }))) { el.disabled = false; return; }
            await post('/api/mcp/servers?name=' + encodeURIComponent(name), null, 'DELETE');
          }
          reload();
        } catch (e) {
          say(e.message || String(e), true);
          el.disabled = false;
        }
      };
    });
    const form = box.querySelector('form.mcp-add');
    if (form) {
      form.onsubmit = async ev => {
        ev.preventDefault();
        const f = new FormData(form);
        try {
          const res = await post('/api/mcp/servers', addBody(f.get('name'), f.get('url'), f.get('command')));
          await reload();
          if (res.error) box.querySelector('.mcp-msg') && (box.querySelector('.mcp-msg').textContent = res.error);
        } catch (e) { say(e.message || String(e), true); }
      };
    }
  }

  // waitConnected mira cada 2 s si el callback ja ha connectat el servidor.
  function waitConnected(name, deps, box) {
    clearInterval(pollTimer);
    const until = Date.now() + 5 * 60 * 1000;
    pollTimer = setInterval(async () => {
      if (Date.now() > until) { clearInterval(pollTimer); return; }
      try {
        const r = await deps.api('/api/mcp');
        const d = await r.json();
        const s = (d.servers || []).find(x => x.name === name);
        if (s && s.status === 'connected') { clearInterval(pollTimer); load(box, deps); }
      } catch (e) { /* es torna a provar */ }
    }, 2000);
  }

  const api = { renderHTML, addBody, load, hasDemo, demoStep, ensureDemo };
  if (typeof window !== 'undefined') window.gregalConnectors = api;
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
})();
