// Proves the browser-side durable event adapter without a DOM or a server.
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

async function module() {
  const source = fs.readFileSync(path.join(__dirname, "app", "runs.js"), "utf8");
  const context = vm.createContext({ window: {} });
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
