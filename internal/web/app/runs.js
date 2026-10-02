// Client de la cua v2. El camí només s'activa quan el servidor anuncia
// interaccions estructurades: els events durables antics conserven el text de
// approve/question, però no la clau ni les opcions que calen per respondre.
const G = () => window.gregal;

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
  const ctrl = new AbortController();
  const controls = window.gregalRunControls || (window.gregalRunControls = new Map());
  controls.set(session, { id, cancel: () => sendCancel(id, session), abort: () => ctrl.abort() });
  try {
    if (caps.event_stream) {
      try { await consumeStream(id, session, cursor, ctrl, onEvent); }
      catch (e) {
        if (ctrl.signal.aborted) return true;
        if (!e.routeUnavailable) throw e;
        await consumePolling(id, session, cursor, ctrl, onEvent);
      }
    } else {
      await consumePolling(id, session, cursor, ctrl, onEvent);
    }
    return true;
  } catch (e) {
    if (e.code === 'INTERACTION_PAYLOAD_MISSING') {
      await sendCancel(id, session);
      ctrl.abort();
    }
    throw e;
  } finally {
    if (controls.get(session)?.id === id) controls.delete(session);
  }
}

if (typeof window !== 'undefined') window.gregalRuns = { run, capabilities, canUseV2 };
