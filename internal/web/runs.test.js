// Proves the browser-side durable event adapter without a DOM or a server.
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

async function module(window = {}) {
  const source = fs.readFileSync(path.join(__dirname, "app", "runs.js"), "utf8");
  const context = vm.createContext({ window, AbortController, TextDecoder, setTimeout });
  const m = new vm.SourceTextModule(source, { context, identifier: "runs.js" });
  await m.link(() => { throw new Error("runs.js no té imports"); });
  await m.evaluate();
  return m.namespace;
}

test("payload d'interacció durable accepta payload, data i text JSON", async () => {
  const { structuredPayload } = await module();
  assert.equal(structuredPayload({ payload: { key: "a", name: "write" } }).key, "a");
  assert.deepEqual(structuredPayload({ data: { key: "q", options: [{ label: "sí" }] } }).options, [{ label: "sí" }]);
  assert.equal(structuredPayload({ payload: { question_key: "q2", choices: [] } }).key, "q2");
  assert.equal(structuredPayload({ text: '{"key":"t","query":"tria"}' }).key, "t");
  assert.equal(structuredPayload({ text: "només text" }), null);
});

test("adapter conserva approve/question i tradueix text/terminal", async () => {
  const { durableToAgent, parseFrame } = await module();
  const seen = [];
  assert.equal(durableToAgent({ kind: "approve_request", payload: { key: "a", name: "write", args: "{}" } }, (k, d) => seen.push([k, d])), false);
  assert.equal(durableToAgent({ kind: "question_request", data: { key: "q", query: "tria", options: [] } }, (k, d) => seen.push([k, d])), false);
  assert.equal(durableToAgent({ kind: "token", text: "hola " }, (k, d) => seen.push([k, d])), false);
  assert.equal(durableToAgent({ kind: "text", text: "hola" }, (k, d) => seen.push([k, d])), false);
  assert.equal(durableToAgent({ kind: "run_completed", text: "fet" }, (k, d) => seen.push([k, d])), true);
  assert.deepEqual(seen.slice(0, 4).map(x => x[0]), ["approve_request", "question_request", "token", "assistant"]);
  assert.equal(parseFrame('id: 4\nevent: text\ndata: {"run_id":7,"text":"ok"}\n\n').data.run_id, 7);
  assert.throws(() => durableToAgent({ kind: "approve_request", text: "només el nom" }, () => {}), /sense dades estructurades/);
});

test("run deduplica errors embolcallats per run, preserva run_failed sol i finalitza polling", async () => {
  let runID = 0, pollPage = 0;
  const delivered = [];
  const pages = [
    { next: 1, events: [{ run_id: 1, kind: "error", text: "⚠️ agent error: detail" }] },
    { next: 2, events: [{ run_id: 1, kind: "run_failed", text: "torn fallit: detail" }] },
    // Identical underlying failure in another run must still be shown.
    { next: 1, events: [{ run_id: 2, kind: "run_failed", text: "torn fallit: detail" }] },
  ];
  const json = value => ({ ok: true, json: async () => value, text: async () => "" });
  const window = {
    gregalSession: "default",
    gregal: { api: async url => {
      if (url === "/api/health") return json({ capabilities: {
        runs: true, durable_events: true, events: true, interactive_events: true,
      } });
      if (url === "/api/v2/runs") return json({ run: { id: ++runID }, cursor: 0 });
      if (url.startsWith("/api/v2/events?")) return json(pages[pollPage++]);
      throw new Error("unexpected API route: " + url);
    } },
  };
  const { run } = await module(window);
  await run("first", [], "", (kind, data) => delivered.push([1, kind, data.message]));
  await run("second", [], "", (kind, data) => delivered.push([2, kind, data.message]));
  assert.deepEqual(delivered, [
    [1, "error", "⚠️ agent error: detail"],
    [2, "error", "torn fallit: detail"],
  ]);
  assert.equal(pollPage, 3, "suppressed duplicate run_failed still terminates polling");
});

test("l'error de l'SSE i el run_failed del fallback comparteixen dedup per run", async () => {
  let pollPage = 0;
  const delivered = [];
  const json = value => ({ ok: true, json: async () => value, text: async () => "" });
  const unavailable = () => Object.assign(new Error("stream route unavailable"), { routeUnavailable: true });
  const window = {
    gregalSession: "default",
    gregal: { api: async url => {
      if (url === "/api/health") return json({ capabilities: {
        runs: true, durable_events: true, event_stream: true, interactive_events: true,
      } });
      if (url === "/api/v2/runs") return json({ run: { id: 8 }, cursor: 0 });
      if (url.startsWith("/api/v2/events/stream?")) {
        let sent = false;
        return { ok: true, body: { getReader: () => ({
          read: async () => {
            if (!sent) {
              sent = true;
              return { done: false, value: new TextEncoder().encode('event: error\ndata: {"run_id":8,"kind":"error","text":"⚠️ agent error: detail"}\n\n') };
            }
            throw unavailable();
          }, cancel: async () => {},
        }) } };
      }
      if (url.startsWith("/api/v2/events?")) return json(pollPage++ === 0
        ? { next: 1, events: [{ run_id: 8, kind: "run_failed", text: "torn fallit: detail" }] }
        : { next: 2, events: [{ run_id: 8, kind: "run_failed", text: "torn fallit: detail" }] });
      throw new Error("unexpected API route: " + url);
    } },
  };
  const { run } = await module(window);
  await run("task", [], "", (kind, data) => delivered.push([kind, data.message]));
  assert.deepEqual(delivered, [["error", "⚠️ agent error: detail"]]);
  assert.equal(pollPage, 1, "the terminal run_failed is consumed despite duplicate display suppression");
});

test("v2 exigeix interaccions estructurades abans d'activar-se", async () => {
  const { canUseV2 } = await module();
  assert.equal(canUseV2({ runs: true, durable_events: true, event_stream: true }), false);
  assert.equal(canUseV2({ runs: true, durable_events: true, event_stream: true, interactive_events: true }), true);
});

test("adapter pinta les eines amb el payload durable (nom, args, sortida)", async () => {
  const { durableToAgent } = await module();
  const seen = [];
  const on = (k, d) => seen.push([k, d]);
  durableToAgent({ kind: "thinking", text: "💭 Miro.", payload: { text: "Miro." } }, on);
  durableToAgent({ kind: "tool_call", text: "🔧 read", payload: { id: "c1", name: "read", args: '{"path":"a.go"}' } }, on);
  durableToAgent({ kind: "tool_result", text: "↳ read: x", payload: '{"id":"c1","name":"read","output":"x"}' }, on);
  durableToAgent({ kind: "blocked", text: "⛔️ no", payload: { reason: "no" } }, on);
  assert.deepEqual(JSON.parse(JSON.stringify(seen)), [
    ["thinking", { text: "Miro." }],
    ["tool_call", { name: "read", args: '{"path":"a.go"}' }],
    ["tool_result", { name: "read", output: "x" }],
    ["blocked", { reason: "no" }],
  ]);
});

test("resum només mostra escriptures reeixides, comprovacions i diffs observats", async () => {
  let markup = "", view = "";
  const buttons = [{ onclick: null }];
  const root = { querySelector: () => ({ querySelectorAll: () => buttons }) };
  const window = {
    gregalSession: "default",
    gregalT: key => ({ "run.summary.completed": "Completada", "run.summary.interrupted": "Interrompuda", "run.summary.cancelled": "Cancel·lada", "run.summary.files": "Fitxers", "run.summary.checks": "Comprovacions", "run.summary.ok": "Correcte", "run.summary.failed": "Fallida", "run.summary.checkpoints": "Checkpoints", "run.summary.review": "Revisió", "run.summary.changes": "Revisa canvis", "run.summary.title": "Resum" }[key] || key),
    gregal: { add: (_kind, _who, html) => { markup = html; return root; }, setView: name => { view = name; } },
  };
  const { createSummary } = await module(window);
  const summary = createSummary();
  summary.record("tool_call", { name: "edit", args: JSON.stringify({ path: "src/<main>.go" }) });
  summary.record("tool_result", { name: "edit", output: "edit aplicat a src/<main>.go" });
  summary.record("tool_call", { name: "write", args: JSON.stringify({ path: "bad.go" }) });
  summary.record("tool_result", { name: "write", output: "ERROR: permís denegat" });
  summary.record("autonomous_checkpoint", { number: 2, checks: [{ command: "go test ./...", code: 0 }, { command: "go vet ./...", code: 1 }], review: "CAL REVISAR: revisar error" });
  summary.render();
  assert.match(markup, /src\/&lt;main&gt;\.go/);
  assert.doesNotMatch(markup, /bad\.go/);
  assert.ok(markup.includes("go test ./..."));
  assert.match(markup, /is-ok/);
  assert.match(markup, /is-failed/);
  assert.match(markup, /CAL REVISAR/);
  buttons[0].onclick();
  assert.equal(view, "canvis");
});

test("resum buit no s'insereix i no es pinta dins d'una altra sessió", async () => {
  let adds = 0;
  const window = { gregalSession: "a", gregal: { add: () => { adds++; } } };
  const { createSummary } = await module(window);
  const empty = createSummary();
  assert.equal(empty.render(), null);
  const stale = createSummary();
  stale.record("tool_call", { name: "write", args: '{"path":"a.txt"}' });
  stale.record("tool_result", { name: "write", output: "escrit a.txt (1 bytes)" });
  window.gregalSession = "b";
  assert.equal(stale.render(), null);
  assert.equal(adds, 0);
});

test("checkpoint durable conserva dades, distingeix codi desconegut i deduplica replay", async () => {
  const seen = [];
  const { durableToAgent, createSummary } = await module({
    gregalSession: "default",
    gregalT: key => ({ "run.summary.checks": "Comprovacions", "run.summary.ok": "Correcte", "run.summary.failed": "Fallida", "run.summary.unknown": "Desconegut", "run.summary.checkpoints": "Checkpoints", "run.summary.review": "Revisió", "run.summary.title": "Resum" }[key] || key),
    gregal: { add: (_kind, _who, html) => { seen.push(html); return null; } },
  });
  const event = { id: 42, kind: "autonomous_checkpoint", text: "resum antic", payload: JSON.stringify({ number: 3, checks: [
    { command: "go test ./...", code: 0, output: "ok" },
    { command: "missing code", output: "no exit status" },
    { command: "null code", code: null },
    { command: "invalid code", code: "0" },
  ], review: "APROVAT" }) };
  assert.equal(durableToAgent(event, (kind, data) => seen.push([kind, data])), false);
  const [kind, payload] = seen[0];
  assert.equal(kind, "autonomous_checkpoint");
  assert.equal(payload.number, 3);
  assert.equal(payload.checks[0].output, "ok");
  assert.equal(payload.review, "APROVAT");
  assert.equal(payload._eventId, 42);

  const summary = createSummary();
  summary.record(kind, payload);
  summary.record(kind, payload);
  summary.render();
  const markup = seen[1];
  assert.equal((markup.match(/run-summary-checkpoints/g) || []).length, 0);
  assert.match(markup, /Checkpoints[\s\S]*?>1</);
  assert.equal((markup.match(/Desconegut/g) || []).length, 3);
  assert.match(markup, /is-ok/);
  assert.equal((markup.match(/is-failed/g) || []).length, 0);
});

test("checkpoint només usa text antic quan conté JSON complet", async () => {
  const { durableToAgent } = await module();
  const seen = [];
  durableToAgent({ id: 1, kind: "autonomous_checkpoint", text: '{"number":2,"checks":[],"review":"OK"}' }, (k, d) => seen.push([k, d]));
  assert.equal(durableToAgent({ id: 2, kind: "autonomous_checkpoint", text: "resum truncat..." }, (k, d) => seen.push([k, d])), false);
  assert.equal(seen[0][1].number, 2);
  assert.equal(seen.length, 1);
  assert.deepEqual(JSON.parse(JSON.stringify(seen[0][1].checks)), []);
});
test("resum indica els estats d'interrupció i cancel·lació", async () => {
  const markup = [];
  const window = {
    gregalSession: "default",
    gregalT: key => ({ "run.summary.interrupted": "Interrompuda", "run.summary.cancelled": "Cancel·lada", "run.summary.checkpoints": "Punts" }[key] || key),
    gregal: { add: (_kind, _who, html) => { markup.push(html); return null; } },
  };
  const { createSummary } = await module(window);
  const interrupted = createSummary();
  interrupted.record("autonomous_checkpoint", { number: 1, checks: [{ command: "go test ./...", code: 2 }] });
  interrupted.record("error", { message: "fallada" });
  interrupted.render();
  const cancelled = createSummary();
  cancelled.record("autonomous_checkpoint", { number: 2, checks: [{ command: "go test ./...", code: 0 }] });
  cancelled.record("status", { message: "torn cancel·lat" });
  cancelled.render();
  assert.match(markup[0], /Interrompuda/);
  assert.match(markup[1], /Cancel·lada/);
});
