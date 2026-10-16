// Ajudes pures de la repetició (timelapse) d'un torn a la sala d'agents:
// dels events durables de /api/replay?run_id=N en treu una línia de temps
// d'estats (pensant, eines, revisant, fills que entren i acaben) que la
// mateixa escena del directe (workshop-scene.js) sap pintar. Sense DOM: es
// carrega al navegador (window.gregalReplayCore) i als tests de node.
(function (root) {
  'use strict';

  const TERMINAL = { run_completed: 'completed', run_failed: 'failed', run_cancelled: 'cancelled' };
  const PHASE = { model: 'thinking', tools: 'tools', review: 'reviewing', compact: 'compacting' };
  const CHILD = new Set(['working', 'completed', 'failed', 'cancelled']);
  // Un torn pot quedar-se minuts esperant una aprovació: a la repetició
  // els buits llargs s'escurcen perquè el clip expliqui la feina, no l'espera.
  const MAX_GAP = 15000;

  function fields(e) {
    const p = e && e.payload;
    if (p && typeof p === 'object') return p;
    if (typeof p === 'string') { try { const v = JSON.parse(p); return v && typeof v === 'object' ? v : {}; } catch (err) { return {}; } }
    return {};
  }
  const str = v => (typeof v === 'string' ? v : '');
  const clip = (s, n) => { const v = str(s).replace(/\s+/g, ' ').trim(); return v.length > n ? v.slice(0, n - 1) + '…' : v; };

  // buildTimeline: frames ordenats amb t (ms de repetició, buits escurçats)
  // i l'estat complet de la sala en aquell moment.
  function buildTimeline(events, opts) {
    const o = opts || {};
    const maxGap = Number.isFinite(o.maxGap) ? o.maxGap : MAX_GAP;
    const list = (Array.isArray(events) ? events : []).filter(e => e && typeof e.kind === 'string')
      .slice().sort((a, b) => (Number(a.id) || 0) - (Number(b.id) || 0));
    const frames = [];
    const agents = new Map();
    // Carrils de rol i relleus (events «lane» i «handoff»): la sala de rols
    // els pinta igual que en directe.
    const lanes = new Map();
    const handoffs = [];
    let role = '';
    let state = 'idle', tool = '', message = '', tools = 0, outcome = '', model = '';
    let t = 0, lastAt = null, started = null, ended = null;
    const snapshot = at => {
      const agentList = [...agents.values()].map(a => ({ ...a }));
      const laneList = [...lanes.values()].sort((a, b) => a.seq - b.seq).map(l => ({ ...l }));
      const frame = { t, at, state, tool, message, tools, model, agents: agentList, role, lanes: laneList, handoffs: handoffs.slice() };
      const prev = frames[frames.length - 1];
      if (prev && prev.state === frame.state && prev.tool === frame.tool && prev.message === frame.message &&
          prev.tools === frame.tools && prev.role === frame.role && JSON.stringify(prev.agents) === JSON.stringify(frame.agents) &&
          JSON.stringify(prev.lanes) === JSON.stringify(frame.lanes) && prev.handoffs.length === frame.handoffs.length) return;
      // Dos canvis al mateix instant: el darrer guanya, sense frames de 0 ms.
      if (prev && prev.t === t) frames[frames.length - 1] = frame;
      else frames.push(frame);
    };
    for (const e of list) {
      const at = Date.parse(e.at || '') || null;
      if (at != null) {
        if (lastAt != null) t += Math.max(0, Math.min(maxGap, at - lastAt));
        lastAt = at;
        if (started == null) started = at;
        ended = at;
      }
      const d = fields(e);
      switch (e.kind) {
        case 'run_queued': if (state === 'idle') state = 'waiting'; message = ''; break;
        case 'run_started': state = 'thinking'; tool = ''; message = ''; break;
        case 'thinking': state = 'thinking'; tool = ''; message = clip(d.text || e.text, 140); break;
        case 'progress': {
          const s = PHASE[str(d.phase)];
          if (s) { state = s; if (s === 'tools') tool = str(d.tools) || tool; else tool = ''; }
          break;
        }
        case 'tool_call': state = 'tools'; tool = str(d.name) || clip(e.text, 40); tools++; message = ''; break;
        case 'tool_result': message = clip(d.output, 140); break;
        case 'blocked': message = clip(d.reason || e.text, 140); break;
        case 'review': state = 'reviewing'; tool = str(d.name); message = clip([d.decision, d.reason].filter(Boolean).join(': '), 140); break;
        case 'autonomous_checkpoint': state = 'reviewing'; tool = ''; message = clip(d.review, 140); break;
        case 'approve_request': case 'question_request': state = 'waiting'; tool = str(d.name); break;
        case 'delegate': {
          const id = str(d.id);
          if (!id) break;
          const prev = agents.get(id) || { id, task: '', agent: '', status: 'working', model: '', tool: '', message: '' };
          const next = { ...prev };
          if (str(d.task)) next.task = clip(d.task, 120);
          if (str(d.agent)) next.agent = d.agent;
          if (CHILD.has(str(d.status))) next.status = d.status;
          if (str(d.model)) next.model = d.model;
          next.tool = next.status === 'working' ? (str(d.tool) || prev.tool) : '';
          if (str(d.message) || str(d.output)) next.message = clip(d.message || d.output, 140);
          agents.set(id, next);
          break;
        }
        case 'lane': {
          const seq = Number(d.seq);
          if (!Number.isFinite(seq) || !str(d.role)) break;
          const files = Array.isArray(d.files) ? d.files.filter(f => typeof f === 'string') : [];
          lanes.set(seq, { seq, step: Number(d.step) || 0, role: d.role, kind: str(d.kind), target: str(d.target), n: Number(d.n) || 0,
            files, command: str(d.command), result: str(d.result), done: d.done === true, agent: str(d.agent), duration_ms: Number(d.duration_ms) || 0 });
          if (d.done !== true) role = d.role;
          break;
        }
        case 'handoff': {
          const seq = Number(d.seq);
          if (!Number.isFinite(seq) || !str(d.from) || !str(d.to)) break;
          handoffs.push({ seq, step: Number(d.step) || 0, from: d.from, to: d.to, kind: str(d.kind), n: Number(d.n) || 0, target: str(d.target),
            files: Array.isArray(d.files) ? d.files.filter(f => typeof f === 'string') : [] });
          role = d.to;
          break;
        }
        case 'usage': if (str(d.model)) model = d.model; break;
        case 'text': if (state === 'tools') { state = 'thinking'; tool = ''; } break;
        case 'done': state = 'completed'; tool = ''; message = clip(e.text, 140); break;
        case 'error': message = clip(e.text, 140); break;
        default:
          if (TERMINAL[e.kind]) {
            outcome = TERMINAL[e.kind]; state = outcome; tool = '';
            // Un fill que encara «treballa» quan el torn ja s'ha tancat no va
            // acabar mai: a la repetició es veu cancel·lat, no etern.
            for (const [id, a] of agents) if (a.status === 'working' || a.status === 'queued') agents.set(id, { ...a, status: outcome === 'completed' ? 'completed' : 'cancelled', tool: '' });
            for (const [seq, l] of lanes) if (!l.done) lanes.set(seq, { ...l, done: true });
          } else continue;
      }
      snapshot(at);
    }
    const duration = frames.length ? frames[frames.length - 1].t : 0;
    return {
      frames, duration, outcome: outcome || (frames.length ? frames[frames.length - 1].state : ''),
      startedAt: started, endedAt: ended, realMs: started != null && ended != null ? ended - started : 0,
      tools, agents: agents.size, model,
    };
  }

  // frameIndexAt: l'últim frame amb t <= pos (cerca binària).
  function frameIndexAt(timeline, pos) {
    const f = timeline && timeline.frames;
    if (!f || !f.length) return -1;
    let lo = 0, hi = f.length - 1;
    if (pos < f[0].t) return 0;
    while (lo < hi) {
      const mid = (lo + hi + 1) >> 1;
      if (f[mid].t <= pos) lo = mid; else hi = mid - 1;
    }
    return lo;
  }

  // toSession dona a l'escena la mateixa forma que /api/sessions/live.
  function toSession(frame, meta) {
    const m = meta || {};
    if (!frame) return null;
    return {
      id: str(m.id) || 'replay', title: str(m.title), project: str(m.project), busy: frame.state !== 'completed' && frame.state !== 'failed' && frame.state !== 'cancelled' && frame.state !== 'idle',
      activity: {
        state: frame.state, task: str(m.title), model: frame.model || str(m.model), tool: frame.tool, message: frame.message,
        step: frame.tools || null, limit: null,
        agents: frame.agents.map(a => ({ id: a.id, task: a.task, agent: a.agent || '', status: a.status, model: a.model, tool: a.tool, message: a.message })),
        role: frame.role || '', lanes: frame.lanes || [], handoffs: frame.handoffs || [],
      },
    };
  }

  // Avança la posició de repetició segons la velocitat (1×, 4×, 10×).
  function advance(pos, elapsedMs, speed, duration) {
    const s = [1, 4, 10].includes(speed) ? speed : 1;
    const next = pos + Math.max(0, elapsedMs) * s;
    return Math.min(Math.max(0, next), Math.max(0, duration || 0));
  }

  function fmtClock(ms) {
    const total = Math.max(0, Math.round((Number(ms) || 0) / 1000));
    const h = Math.floor(total / 3600), m = Math.floor((total % 3600) / 60), s = total % 60;
    const pad = n => String(n).padStart(2, '0');
    return h ? h + ':' + pad(m) + ':' + pad(s) : m + ':' + pad(s);
  }

  // Quin torn correspon a un grup de feina: el n-èsim des del final entre
  // els acabats (best effort per a converses repintades sense id de torn).
  function runForGroup(runs, indexFromEnd) {
    const done = (Array.isArray(runs) ? runs : []).filter(r => r && r.state && r.state !== 'running')
      .sort((a, b) => (Number(b.run_id) || 0) - (Number(a.run_id) || 0));
    return done[Math.max(0, indexFromEnd | 0)] || null;
  }

  const api = { buildTimeline, frameIndexAt, toSession, advance, fmtClock, runForGroup, MAX_GAP };
  if (typeof module === 'object' && module.exports) module.exports = api;
  if (root) root.gregalReplayCore = api;
})(typeof window !== 'undefined' ? window : null);
