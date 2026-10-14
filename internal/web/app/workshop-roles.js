// Ajudes pures dels rols de la sala d'agents: de l'activitat del torn
// (/api/sessions/live o la repetició) en treuen qui té el torn, què diu la
// bombolla de cada personatge, l'etiqueta dels relleus, la línia de temps
// per passos i el resum d'un rol per al panell de detall. Sense DOM: es
// carreguen al navegador (window.gregalWorkshopRoles) i als tests de node.
(function (root) {
  'use strict';

  const ROLES = ['coordinator', 'researcher', 'builder', 'tester', 'reviewer'];
  const ROLE_SET = new Set(ROLES);
  // Un fill real amb aquests noms és l'investigador mateix (el scout o un
  // subagent «researcher» definit per l'usuari): seu a la seva taula.
  const RESEARCHER_AGENTS = new Set(['scout', 'researcher']);
  const LIVE = new Set(['thinking', 'tools', 'reviewing', 'compacting', 'working']);
  const TERMINAL = new Set(['completed', 'failed', 'cancelled']);
  const str = v => (typeof v === 'string' ? v : '');
  const num = v => (Number.isFinite(v) ? v : 0);
  const tr = (T, key, fallback) => {
    const value = typeof T === 'function' ? T(key) : '';
    return value && value !== key ? value : fallback;
  };

  // shortPath: el nom del fitxer i, si hi cap, la carpeta de sobre.
  function shortPath(path, max) {
    const limit = max || 28;
    const clean = str(path).replace(/\\/g, '/').replace(/\/+$/, '');
    if (clean.length <= limit) return clean;
    const parts = clean.split('/');
    const base = parts.pop() || clean;
    const parent = parts.pop();
    const both = parent ? parent + '/' + base : base;
    if (both.length <= limit) return '…/' + both;
    return base.length <= limit ? base : base.slice(0, limit - 1) + '…';
  }

  function clip(text, max) {
    const value = str(text).replace(/\s+/g, ' ').trim();
    return value.length > max ? value.slice(0, max - 1) + '…' : value;
  }

  function normalizeLane(l) {
    if (!l || typeof l !== 'object' || !Number.isFinite(l.seq)) return null;
    return {
      seq: l.seq, step: num(l.step), role: ROLE_SET.has(l.role) ? l.role : 'coordinator', kind: str(l.kind), target: str(l.target),
      n: num(l.n), tools: Array.isArray(l.tools) ? l.tools.filter(t => typeof t === 'string') : [],
      files: Array.isArray(l.files) ? l.files.filter(f => typeof f === 'string') : [], command: str(l.command),
      result: l.result === 'ok' || l.result === 'fail' ? l.result : '', done: l.done === true, agent: str(l.agent),
      started_at: str(l.started_at), duration_ms: num(l.duration_ms),
    };
  }

  function normalizeHandoff(h) {
    if (!h || typeof h !== 'object' || !Number.isFinite(h.seq) || !ROLE_SET.has(h.from) || !ROLE_SET.has(h.to)) return null;
    return { seq: h.seq, step: num(h.step), from: h.from, to: h.to, kind: str(h.kind), n: num(h.n), target: str(h.target),
      files: Array.isArray(h.files) ? h.files.filter(f => typeof f === 'string') : [], at: str(h.at) };
  }

  // normalize: la part de l'activitat que fan servir els rols.
  function normalize(activity) {
    const a = activity && typeof activity === 'object' ? activity : {};
    const lanes = (Array.isArray(a.lanes) ? a.lanes : []).map(normalizeLane).filter(Boolean).sort((x, y) => x.seq - y.seq);
    const handoffs = (Array.isArray(a.handoffs) ? a.handoffs : []).map(normalizeHandoff).filter(Boolean).sort((x, y) => x.seq - y.seq);
    const usage = {};
    if (a.role_usage && typeof a.role_usage === 'object') {
      for (const role of ROLES) {
        const u = a.role_usage[role];
        if (u && typeof u === 'object') usage[role] = { up: num(u.up_tokens), down: num(u.down_tokens), calls: num(u.calls) };
      }
    }
    const agents = (Array.isArray(a.agents) ? a.agents : []).filter(x => x && typeof x.id === 'string' && x.id);
    return { _n: true, state: str(a.state), role: ROLE_SET.has(a.role) ? a.role : '', lanes, handoffs, usage, agents };
  }

  const norm = a => (a && a._n ? a : normalize(a));

  function isResearcherAgent(agent) {
    return Boolean(agent) && RESEARCHER_AGENTS.has(str(agent.agent).toLowerCase());
  }

  // currentRole: qui té el torn. Un investigador real treballant mana.
  function currentRole(activity) {
    const a = norm(activity);
    if (a.agents.some(x => isResearcherAgent(x) && x.status === 'working')) return 'researcher';
    if (a.role) return a.role;
    const last = a.lanes[a.lanes.length - 1];
    return last ? last.role : 'coordinator';
  }

  // roleStates: active (té el torn i el torn corre), done (ha treballat),
  // failed (el seu últim pas ha fallat), idle (encara no ha fet res).
  function roleStates(activity) {
    const a = norm(activity);
    const live = LIVE.has(a.state) || a.state === 'waiting';
    const current = currentRole(a);
    const out = {};
    for (const role of ROLES) {
      const lanes = a.lanes.filter(l => l.role === role);
      const last = lanes[lanes.length - 1];
      let state = lanes.length ? 'done' : 'idle';
      if (last && last.result === 'fail') state = 'failed';
      if (role === 'researcher' && a.agents.some(x => isResearcherAgent(x) && x.status === 'working')) state = 'active';
      else if (live && role === current) state = 'active';
      // The coordinator is always in the room: it took part even when
      // another role holds the turn.
      if (role === 'coordinator' && state === 'idle' && (live || TERMINAL.has(a.state))) state = 'done';
      out[role] = state;
    }
    return out;
  }

  // bubble: icona i text curt per sobre del personatge (null = cap).
  function bubble(activity, role, T) {
    const a = norm(activity);
    const states = roleStates(a);
    const lanes = a.lanes.filter(l => l.role === role);
    const last = lanes[lanes.length - 1];
    if (role === 'tester' && last && last.done && last.result && states.tester !== 'active') {
      return last.result === 'ok'
        ? { icon: 'ok', tone: 'ok', text: tr(T, 'workshop.bubble.testsOk', 'Tests passed') }
        : { icon: 'fail', tone: 'fail', text: tr(T, 'workshop.bubble.testsFail', 'Tests failed') };
    }
    if (role === 'coordinator' && TERMINAL.has(a.state)) {
      if (a.state === 'completed') return { icon: 'ok', tone: 'ok', text: tr(T, 'workshop.bubble.done', 'Done') };
      return { icon: 'fail', tone: 'fail', text: a.state === 'failed' ? tr(T, 'workshop.bubble.failed', 'The turn failed') : tr(T, 'workshop.bubble.cancelled', 'Stopped') };
    }
    if (states[role] !== 'active') return null;
    if (a.state === 'waiting') return { icon: 'wait', tone: 'wait', text: tr(T, 'workshop.bubble.waiting', 'Waiting for you') };
    if (role === 'researcher') {
      const scout = a.agents.find(x => isResearcherAgent(x) && x.status === 'working');
      if (scout) return scoutBubble(scout, T);
    }
    if (last && last.done && last.result && (role === 'tester' || last.result === 'fail')) {
      if (role === 'tester') return last.result === 'ok'
        ? { icon: 'ok', tone: 'ok', text: tr(T, 'workshop.bubble.testsOk', 'Tests passed') }
        : { icon: 'fail', tone: 'fail', text: tr(T, 'workshop.bubble.testsFail', 'Tests failed') };
      return { icon: 'fail', tone: 'fail', text: laneText(last, T) };
    }
    if (!last || (last.done && a.state === 'thinking')) {
      return { icon: 'think', tone: 'live', text: role === 'coordinator' && !a.lanes.length ? tr(T, 'workshop.bubble.planning', 'Planning') : tr(T, 'workshop.bubble.thinking', 'Thinking…') };
    }
    return { icon: laneIcon(last), tone: 'live', text: laneText(last, T) };
  }

  function scoutBubble(agent, T) {
    const tool = str(agent.tool);
    if (tool === 'web_fetch' || tool === 'web_search' || tool === 'browser') return { icon: 'web', tone: 'live', text: tr(T, 'workshop.bubble.webShort', 'Browsing the web') };
    if (tool === 'read' || tool === 'grep' || tool === 'glob' || tool === 'symbols') return { icon: 'read', tone: 'live', text: tr(T, 'workshop.bubble.reading', 'Reading the code') };
    return { icon: 'read', tone: 'live', text: tr(T, 'workshop.bubble.scout', 'Investigating the task') };
  }

  const ICON = { read: 'read', search: 'read', web: 'web', edit: 'edit', commit: 'edit', run: 'run', test: 'run', plan: 'plan', delegate: 'plan', review: 'review', verify: 'review', scout: 'read', tool: 'plan' };
  function laneIcon(lane) { return ICON[lane && lane.kind] || 'plan'; }

  // laneText: la frase d'un pas («Reading src/stats.py», «Editing 2 files»).
  function laneText(lane, T) {
    if (!lane) return '';
    const x = lane.kind === 'web' || lane.kind === 'search' || lane.kind === 'run' ? clip(lane.target, 26) : shortPath(lane.target || lane.files[0] || '', 26);
    const n = lane.n > 1 ? lane.n : lane.files.length > 1 ? lane.files.length : 0;
    const fill = (key, fallback) => tr(T, key, fallback).replace('{x}', x).replace('{n}', String(n));
    switch (lane.kind) {
      case 'read': return n > 1 ? fill('workshop.bubble.readN', 'Reading {n} files') : fill('workshop.bubble.read', 'Reading {x}');
      case 'search': return x ? fill('workshop.bubble.search', 'Searching {x}') : fill('workshop.bubble.searchAny', 'Searching the code');
      case 'web': return x ? fill('workshop.bubble.web', 'Fetching {x}') : fill('workshop.bubble.webShort', 'Browsing the web');
      case 'edit': return n > 1 ? fill('workshop.bubble.editN', 'Editing {n} files') : fill('workshop.bubble.edit', 'Editing {x}');
      case 'commit': return fill('workshop.bubble.commit', 'Committing');
      case 'test':
        if (lane.done && lane.result === 'ok') return fill('workshop.bubble.testsOk', 'Tests passed');
        if (lane.done && lane.result === 'fail') return fill('workshop.bubble.testsFail', 'Tests failed');
        return fill('workshop.bubble.test', 'Running tests');
      case 'run': return fill('workshop.bubble.run', 'Running {x}');
      case 'plan': return fill('workshop.bubble.plan', 'Updating the plan');
      case 'delegate': return n > 1 ? fill('workshop.bubble.delegateN', 'Delegating {n} tasks') : fill('workshop.bubble.delegate', 'Delegating a task');
      case 'review': return lane.done && lane.result === 'fail' ? fill('workshop.bubble.reviewFail', 'Changes requested') : fill('workshop.bubble.review', 'Reviewing');
      case 'verify': return lane.done && lane.result === 'fail' ? fill('workshop.bubble.reviewFail', 'Changes requested') : fill('workshop.bubble.verify', 'Verifying the result');
      case 'scout': return fill('workshop.bubble.scout', 'Investigating the task');
      default: return x ? fill('workshop.bubble.tool', 'Using {x}') : fill('workshop.bubble.thinking', 'Thinking…');
    }
  }

  // handoffLabel: el text del sobre que vola («3 files to edit»).
  function handoffLabel(h, T) {
    if (!h) return '';
    const n = h.n > 1 ? h.n : 0;
    const fill = (key, fallback) => tr(T, key, fallback).replace('{n}', String(n));
    switch (h.kind) {
      case 'read': return n ? fill('workshop.handoff.readN', '{n} files to read') : fill('workshop.handoff.read', 'Something to look into');
      case 'web': return fill('workshop.handoff.web', 'Pages to fetch');
      case 'edit': return n ? fill('workshop.handoff.editN', '{n} files to edit') : fill('workshop.handoff.edit', 'A file to edit');
      case 'run': return fill('workshop.handoff.run', 'Commands to run');
      case 'commit': return fill('workshop.handoff.commit', 'Changes to commit');
      case 'test': return fill('workshop.handoff.test', 'Tests to run');
      case 'review': return fill('workshop.handoff.review', 'Changes to review');
      case 'brief': return fill('workshop.handoff.brief', 'Research brief');
      default: return fill('workshop.handoff.report', 'Results');
    }
  }

  // roleSummary: tot el que el panell de detall ensenya d'un rol.
  function roleSummary(activity, role) {
    const a = norm(activity);
    const lanes = a.lanes.filter(l => l.role === role);
    const files = [];
    const seen = new Set();
    for (const l of lanes) for (const f of l.files) if (!seen.has(f)) { seen.add(f); files.push({ path: f, kind: l.kind }); }
    const commands = lanes.filter(l => l.command).map(l => ({ seq: l.seq, command: l.command, result: l.result, done: l.done, duration_ms: l.duration_ms }));
    const durationMs = lanes.reduce((sum, l) => sum + l.duration_ms, 0);
    const children = role === 'researcher' ? a.agents.filter(isResearcherAgent) : [];
    return { role, state: roleStates(a)[role], lanes, files, commands, durationMs, usage: a.usage[role] || null, children,
      handoffsIn: a.handoffs.filter(h => h.to === role).length, handoffsOut: a.handoffs.filter(h => h.from === role).length };
  }

  // timeline: un segment per pas, per a la tira clicable de sota la sala.
  function timeline(activity) {
    const a = norm(activity);
    return a.lanes.map(l => ({ seq: l.seq, step: l.step, role: l.role, kind: l.kind, result: l.result, done: l.done, files: l.files.length, duration_ms: l.duration_ms }));
  }

  // extraAgents: fills reals que no són l'investigador (seuen a part).
  function extraAgents(activity) {
    const a = norm(activity);
    return a.agents.filter(x => !isResearcherAgent(x));
  }

  // diff: què ha canviat entre dues lectures (per animar només el nou):
  // relleus nous, passos de prova que acaben, rols que canvien d'estat.
  function diff(prev, next) {
    const p = norm(prev);
    const n = norm(next);
    const lastHandoff = p.handoffs.length ? p.handoffs[p.handoffs.length - 1].seq : 0;
    const prevDone = new Map(p.lanes.map(l => [l.seq, l.done]));
    const handoffs = n.handoffs.filter(h => h.seq > lastHandoff);
    const results = n.lanes.filter(l => l.role === 'tester' && l.done && l.result && prevDone.get(l.seq) !== true);
    const before = roleStates(p), after = roleStates(n);
    const activated = ROLES.filter(r => after[r] === 'active' && before[r] !== 'active');
    const released = ROLES.filter(r => before[r] === 'active' && after[r] !== 'active');
    return { handoffs, results, activated, released };
  }

  function fmtDuration(ms) {
    const v = Math.max(0, Number(ms) || 0);
    if (v < 1000) return v ? (Math.round(v / 100) / 10) + 's' : '';
    if (v < 60000) return Math.round(v / 1000) + 's';
    return Math.floor(v / 60000) + 'm ' + String(Math.round((v % 60000) / 1000)).padStart(2, '0') + 's';
  }

  const api = { ROLES, normalize, shortPath, currentRole, roleStates, bubble, laneText, laneIcon, handoffLabel, roleSummary, timeline, extraAgents, isResearcherAgent, diff, fmtDuration };
  if (typeof module === 'object' && module.exports) module.exports = api;
  if (root) root.gregalWorkshopRoles = api;
})(typeof window !== 'undefined' ? window : null);
