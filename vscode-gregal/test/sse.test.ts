import { test } from "node:test";
import assert from "node:assert/strict";
import { durableInteraction, feedSse, renderDurableEvent, renderEvent } from "../src/sse.ts";

test("event complet", () => {
    const { evs, rest } = feedSse('event: token\ndata: {"text":"hola"}\n\n', "");
    assert.equal(evs.length, 1);
    assert.equal(evs[0].name, "token");
    assert.equal(renderEvent(evs[0]), "hola");
    assert.equal(rest, "");
});

test("tros partit", () => {
    const r1 = feedSse("event: tok", "");
    assert.equal(r1.evs.length, 0);
    const r2 = feedSse('en\ndata: {"text":"x"}\n\n', r1.rest);
    assert.equal(r2.evs.length, 1);
    assert.equal(r2.evs[0].name, "token");
});

test("tool_call i fallback es renderitzen", () => {
    const { evs } = feedSse(
        'event: tool_call\ndata: {"name":"write","args":"{}"}\n\nevent: fallback\ndata: {"model":"m"}\n\n',
        "",
    );
    assert.equal(evs.length, 2);
    assert.match(renderEvent(evs[0])!, /🔧 write/);
    assert.match(renderEvent(evs[1])!, /Primary provider failed/);
});

test("status de fallback localitzable", () => {
    const ev = { name: "fallback", data: '{"model":"Backup"}' };
    const line = renderEvent(ev, (message, ...args) => `ca:${message.replace("{0}", String(args[0]))}`);
    assert.equal(line, "↪ ca:Primary provider failed; falling back to Backup.");
});

test("done amb reply i event desconegut silenciós", () => {
    const { evs } = feedSse(
        'event: ping\ndata: {}\n\nevent: done\ndata: {"reply":"fet"}\n\n',
        "",
    );
    assert.equal(renderEvent(evs[0]), null);
    assert.match(renderEvent(evs[1])!, /fet/);
});

test("events durables v2 renderitzen text i amaguen el cicle de vida", () => {
    assert.equal(renderDurableEvent({ kind: "run_queued", text: "torn encuat" }), null);
    assert.equal(renderDurableEvent({ kind: "text", text: "resposta" }), "resposta");
    assert.equal(renderDurableEvent({ kind: "done", text: "final" }), "\nfinal");
    assert.equal(renderDurableEvent({ kind: "run_completed", text: "torn completat" }), null);
    assert.match(renderDurableEvent({ kind: "error", text: "ha fallat" })!, /ha fallat/);
});

test("events durables extreuen aprovació i pregunta estructurades", () => {
    const approval = durableInteraction({ kind: "approve_request", payload: { approval_key: "a-1", name: "write", args: "{}" } });
    assert.equal(approval?.key, "a-1");
    assert.equal(approval?.name, "write");

    const question = durableInteraction({ kind: "question_request", data: { question_key: "q-1", question: "Tria", choices: ["sí", "no"] } });
    assert.equal(question?.key, "q-1");
    assert.equal(question?.query, "Tria");
    assert.deepEqual(question?.options, ["sí", "no"]);

    const fromText = durableInteraction({ kind: "approve_request", text: '{"key":"t-1","name":"shell"}' });
    assert.equal(fromText?.key, "t-1");
});
