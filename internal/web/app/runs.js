// Client de la cua v2. El camí només s'activa quan el servidor anuncia
// interaccions estructurades: els events durables antics conserven el text de
// approve/question, però no la clau ni les opcions que calen per respondre.
const G = () => window.gregal;

const html = value => String(value == null ? '' : value)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;').replace(/'/g, '&#39;');

function commandFromArgs(args) {
  if (args && typeof args === 'object') return args;
  if (typeof args !== 'string') return {};
  try { return JSON.parse(args); } catch (e) { return {}; }
}

// Resum per torn construït només amb esdeveniments observats. Els paths de
// write/edit/patch provenen de crides completades; els checks són els que el
// servidor publica explícitament als checkpoints autònoms.
export function createSummary() {
  const writes = new Set(), pendingWrites = [];
  const checks = [], checkpoints = [];
  const seenCheckpoints = new Set();
  const session = window.gregalSession || 'default';
  let status = 'completed', rendered = false;
  function record(ev, data) {
    const d = data || {};
    if (ev === 'tool_call') {
      const name = String(d.name || '').toLowerCase();
      if (['write', 'edit', 'patch'].includes(name)) {
        const args = commandFromArgs(d.args);
        const paths = [args.path, args.file, ...(Array.isArray(args.paths) ? args.paths : [])]
          .filter(p => typeof p === 'string' && p.trim());
        pendingWrites.push({ name, paths });
      }
    } else if (ev === 'tool_result') {
      const name = String(d.name || '').toLowerCase();
      const pending = pendingWrites.findIndex(p => p.name === name);
      if (pending >= 0) {
        const [entry] = pendingWrites.splice(pending, 1);
        const output = String(d.output || '').trim();
        const succeeded = entry.name === 'write' ? /^escrit\s/i.test(output)
          : entry.name === 'edit' ? /^edit aplicat a\s/i.test(output)
            : /^patch aplicat a\s/i.test(output);
        if (succeeded) entry.paths.forEach(p => writes.add(p));
      }
    } else if (ev === 'autonomous_checkpoint') {
      // SSE reconnexions i fallback a polling poden tornar a lliurar el mateix
      // event; l'id durable evita comptar-lo dues vegades al resum.
      const eventId = d._eventId;
      if (eventId != null && seenCheckpoints.has(String(eventId))) return;
      if (eventId != null) seenCheckpoints.add(String(eventId));
      const list = Array.isArray(d.checks) ? d.checks : [];
      list.forEach(c => {
        const validCode = c && typeof c.code === 'number' && Number.isFinite(c.code);
        checks.push({ command: String(c && c.command || ''), ok: validCode && c.code === 0, failed: validCode && c.code !== 0 });
      });
      checkpoints.push({ number: d.number, review: String(d.review || '') });
    } else if (ev === 'error') status = 'interrupted';
    else if (ev === 'status' && /cancel·lat|cancelled/i.test(String(d.message || ''))) status = 'cancelled';
  }
  function render(options) {
    if (rendered || typeof window === 'undefined' || session !== (window.gregalSession || 'default')) return null;
    if (!writes.size && !checks.length && !checkpoints.length) return null;
    rendered = true;
    const opts = options || {};
    if (opts.status) status = opts.status === 'failed' ? 'interrupted' : opts.status;
    const files = Array.from(writes).sort();
    const rows = [];
    if (files.length) {
      rows.push('<div class="run-summary-section">' + T('run.summary.files') + ' <span class="run-summary-pill">' + files.length + '</span></div>');
      files.slice(0, 8).forEach(p => rows.push('<div class="run-summary-row run-summary-file">' + html(p) + '</div>'));
    }
    if (checks.length) {
      rows.push('<div class="run-summary-section">' + T('run.summary.checks') + '</div>');
      checks.slice(-8).forEach(c => {
        const state = c.ok ? 'ok' : c.failed ? 'failed' : 'unknown';
        const cls = c.ok ? 'is-ok' : c.failed ? 'is-failed' : 'is-muted';
        rows.push('<div class="run-summary-row"><span>' + html(c.command || T('run.summary.checks')) + '</span><span class="run-summary-pill ' + cls + '">' + T('run.summary.' + state) + '</span></div>');
      });
    }
    if (checkpoints.length) {
      const latest = checkpoints[checkpoints.length - 1];
      rows.push('<div class="run-summary-row"><span>' + T('run.summary.checkpoints') + '</span><span class="run-summary-pill">' + checkpoints.length + '</span></div>');
      if (latest.review) rows.push('<div class="run-summary-row run-summary-muted">' + T('run.summary.review') + ': ' + html(latest.review) + '</div>');
    }
    const actionMarkup = files.length ? '<div class="run-summary-actions"><button type="button" class="run-summary-action">' + html(T('run.summary.changes')) + '</button></div>' : '';
    const card = G()?.add?.('t', '', '<section class="run-summary" aria-label="' + html(T('run.summary.title')) + '"><div class="run-summary-title">' + html(T('run.summary.' + status)) + '</div>' + rows.join('') + actionMarkup + '</section>');
    const root = card && (card.querySelector ? card : card.el || card.node);
    const actionBar = root?.querySelector?.('.run-summary-actions');
    const buttons = actionBar?.querySelectorAll?.('button');
    if (buttons && buttons[0]) buttons[0].onclick = () => G()?.setView?.('canvis');
    return card;
  }
  return { record, render };
}

function T(key) {
  try { return window.gregalT?.(key) || key; } catch (e) { return key; }
}

let healthPromise = null;

function routeUnavailable(status) {
  return status === 404 || status === 405 || status === 501;
}

function errorFor(resp, body) {
  const e = new Error(body || ('HTTP ' + resp.status));
  e.status = resp.status;
  e.routeUnavailable = routeUnavailable(resp.status);
  return e;
}

function key() {
  try { if (crypto.randomUUID) return crypto.randomUUID(); } catch (e) {}
  return 'web-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2);
}

async function capabilities() {
  if (!healthPromise) {
    healthPromise = G().api('/api/health').then(async r => {
      if (!r.ok) throw errorFor(r, await r.text());
      const d = await r.json();
      return d && d.capabilities || {};
    }).catch(e => {
      healthPromise = null;
      return {};
    });
  }
  return healthPromise;
}

export function canUseV2(caps) {
  return !!(caps.runs && caps.durable_events && (caps.event_stream || caps.events) && caps.interactive_events);
}

function sessionHeaders(session) {
  return { 'X-Gregal-Session': session };
}

async function sendCancel(id, session) {
  if (!id) return;
  try { await G().api('/api/v2/runs/' + encodeURIComponent(id) + '/cancel', { method: 'POST', body: '{}', headers: sessionHeaders(session) }); } catch (e) {}
}

export function parseFrame(frame) {
  let name = '', data = '';
  frame.split('\n').forEach(line => {
    if (line.startsWith('event:')) name = line.slice(6).trim();
    else if (line.startsWith('data:')) data += line.slice(5).trim();
  });
  if (!data) return null;
  try { return { name, data: JSON.parse(data) }; } catch (e) { return null; }
}

// El backend pot transportar el payload en un camp específic (payload/data),
// com a objecte d'interacció, o dins de text per compatibilitat. Acceptem les
// tres formes perquè una reconnexió no faci perdre un diàleg pendent.
export function structuredPayload(e) {
  const candidates = [e && e.payload, e && e.data, e && e.details, e && e.interaction, e && e.meta];
  for (const candidate of candidates) {
    let value = candidate;
    if (typeof value === 'string') { try { value = JSON.parse(value); } catch (x) { continue; } }
    if (value && typeof value === 'object') {
      const nested = value.payload || value.interaction || value.data;
      if (nested && typeof nested === 'object' && nested.key) value = nested;
      const key = value.key || value.approval_key || value.question_key;
      if (key) {
        const out = Object.assign({}, value, { key });
        if (!out.options && out.choices) out.options = out.choices;
        if (!out.query && out.question) out.query = out.question;
        return out;
      }
    }
  }
  if (e && (e.key || e.approval_key || e.question_key)) {
    return Object.assign({}, e, { key: e.key || e.approval_key || e.question_key });
  }
  if (typeof (e && e.text) === 'string') {
    try {
      const parsed = JSON.parse(e.text);
      if (parsed) return structuredPayload(parsed);
    } catch (x) {}
  }
  return null;
}

// fields torna el payload estructurat d'un event (objecte o JSON en text), o
// {} si no n'hi ha: els events antics només duien text.
function fields(e) {
  let p = e && e.payload;
  if (typeof p === 'string') { try { p = JSON.parse(p); } catch (x) { p = null; } }
  return p && typeof p === 'object' ? p : {};
}

function checkpointFields(e) {
  const payload = fields(e);
  if (Object.keys(payload).length) return payload;
  // Compatibilitat amb events antics: només fem servir text si és un objecte
  // JSON complet, mai un text de presentació potencialment retallat.
  if (typeof e?.text === 'string') {
    try {
      const parsed = JSON.parse(e.text);
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) return parsed;
    } catch (x) {}
  }
  return {};
}

export function durableToAgent(e, onEvent) {
  if (!e || !e.kind) return false;
  const d = e;
  // La versió actual pot posar el payload en camps nous o dins de text JSON;
  // acceptem les dues formes perquè també funcioni després d'una reconnexió.
  if (e.kind === 'approve_request' || e.kind === 'question_request') {
    const payload = structuredPayload(e);
    if (!payload || !payload.key) {
      const err = new Error('servidor v2 sense dades estructurades per interactuar');
      err.code = 'INTERACTION_PAYLOAD_MISSING';
      throw err;
    }
    onEvent(e.kind, payload);
    return false;
  }
  if (e.kind === 'text') onEvent('assistant', { text: e.text || '' });
  else if (e.kind === 'token') onEvent('token', { text: e.text || '' });
  else if (e.kind === 'done') onEvent('done', { reply: e.text || '' });
  else if (e.kind === 'error') onEvent('error', { message: e.text || 'error' });
  else if (e.kind === 'verify') onEvent('verify', { verdict: e.text || '' });
  else if (e.kind === 'tool_call') onEvent('tool_call', { name: fields(e).name || e.text || 'eina', args: fields(e).args || '' });
  else if (e.kind === 'tool_result') onEvent('tool_result', { name: fields(e).name || e.text || 'eina', output: fields(e).output || '' });
  else if (e.kind === 'autonomous_checkpoint') {
    const payload = checkpointFields(e);
    const validNumber = typeof payload.number === 'number' && Number.isFinite(payload.number);
    const validChecks = Array.isArray(payload.checks) && payload.checks.length > 0;
    const validReview = typeof payload.review === 'string' && payload.review.trim() !== '';
    if (!validNumber && !validChecks && !validReview) return false;
    onEvent('autonomous_checkpoint', {
      number: payload.number,
      checks: Array.isArray(payload.checks) ? payload.checks : [],
      review: payload.review,
      _eventId: e.id,
    });
  }
  else if (e.kind === 'thinking') onEvent('thinking', { text: fields(e).text || e.text || '' });
  else if (e.kind === 'blocked') onEvent('blocked', { reason: fields(e).reason || e.text || '' });
  else if (e.kind === 'run_failed') onEvent('error', { message: e.text || 'el torn ha fallat' });
  else if (e.kind === 'run_cancelled') onEvent('status', { message: 'torn cancel·lat' });
  return e.kind === 'run_completed' || e.kind === 'run_failed' || e.kind === 'run_cancelled';
}

async function consumeStream(id, session, after, ctrl, onEvent) {
  const r = await G().api('/api/v2/events/stream?after=' + after + '&limit=100', { signal: ctrl.signal, headers: sessionHeaders(session) });
  if (!r.ok) throw errorFor(r, await r.text());
  if (!r.body || !r.body.getReader) throw new Error('stream v2 no disponible');
  const rd = r.body.getReader(), dec = new TextDecoder();
  let buf = '', finished = false;
  for (;;) {
    const x = await rd.read();
    if (x.done) break;
    buf += dec.decode(x.value, { stream: true });
    let at;
    while ((at = buf.indexOf('\n\n')) >= 0) {
      const frame = buf.slice(0, at); buf = buf.slice(at + 2);
      const parsed = parseFrame(frame);
      if (!parsed || !parsed.data || Number(parsed.data.run_id) !== Number(id)) continue;
      if (durableToAgent(parsed.data, onEvent)) finished = true;
    }
    if (finished) { try { await rd.cancel(); } catch (e) {} return; }
  }
  if (!ctrl.signal.aborted) throw new Error('stream v2 tancat abans d’acabar el torn');
}

async function consumePolling(id, session, after, ctrl, onEvent) {
  for (;;) {
    const r = await G().api('/api/v2/events?after=' + after + '&limit=100', { signal: ctrl.signal, headers: sessionHeaders(session) });
    if (!r.ok) throw errorFor(r, await r.text());
    const page = await r.json();
    const events = Array.isArray(page.events) ? page.events : [];
    after = Number.isFinite(Number(page.next)) ? Number(page.next) : after;
    for (const e of events) {
      if (Number(e.run_id) !== Number(id)) continue;
      if (durableToAgent(e, onEvent)) return;
    }
    await new Promise(resolve => setTimeout(resolve, events.length ? 80 : 250));
  }
}

/**
 * Envia un torn v2. Retorna false si el servidor no ofereix el contracte
 * complet i el caller ha de conservar el stream legacy /api/agent.
 */
export async function run(task, images, mode, onEvent) {
  const session = window.gregalSession || 'default';
  const caps = await capabilities();
  if (!canUseV2(caps)) return false;
  const payload = { task, images: images || [], mode: mode || '', idempotency_key: key() };
  const r = await G().api('/api/v2/runs', { method: 'POST', body: JSON.stringify(payload), headers: sessionHeaders(session) });
  if (!r.ok) {
    const body = await r.text();
    const e = errorFor(r, body);
    // Només el descobriment d'una ruta inexistent permet tornar a l'API v1.
    if (e.routeUnavailable) return false;
    throw e;
  }
  const out = await r.json();
  const id = out && out.run && out.run.id;
  if (!id) throw new Error('resposta v2 sense id d’execució');
  const cursor = Math.max(0, Number(out.cursor) || 0);
  const summary = createSummary();
  // The event and terminal run_failed records can wrap the same error in
  // different presentation text. Compare their underlying message per run.
  const seenErrors = new Set();
  const receive = (event, data) => {
    if (event === 'error') {
      const message = String(data && data.message || 'error');
      const key = message.trim()
        .replace(/^⚠️\s*/u, '')
        .replace(/^torn fallit:\s*/iu, '')
        .replace(/^agent\s*:\s*error:\s*/iu, '')
        .replace(/^agent\s+error:\s*/iu, '')
        .trim();
      if (seenErrors.has(key)) return;
      seenErrors.add(key);
    }
    summary.record(event, data);
    onEvent(event, data);
  };
  const ctrl = new AbortController();
  const controls = window.gregalRunControls || (window.gregalRunControls = new Map());
  controls.set(session, { id, cancel: () => sendCancel(id, session), abort: () => ctrl.abort() });
  try {
    if (caps.event_stream) {
      try { await consumeStream(id, session, cursor, ctrl, receive); }
      catch (e) {
        if (ctrl.signal.aborted) return true;
        if (!e.routeUnavailable) throw e;
        await consumePolling(id, session, cursor, ctrl, receive);
      }
    } else {
      await consumePolling(id, session, cursor, ctrl, receive);
    }
    summary.render();
    return true;
  } catch (e) {
    summary.render({ status: 'interrupted' });
    if (e.code === 'INTERACTION_PAYLOAD_MISSING') {
      await sendCancel(id, session);
      ctrl.abort();
    }
    throw e;
  } finally {
    if (controls.get(session)?.id === id) controls.delete(session);
  }
}

if (typeof window !== 'undefined') window.gregalRuns = { run, capabilities, canUseV2, createSummary };
