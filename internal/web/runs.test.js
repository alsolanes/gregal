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
