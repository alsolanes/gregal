import { test } from "node:test";
import assert from "node:assert/strict";
import * as http from "http";
import type { AddressInfo } from "net";
import { runAgent, type RunHooks } from "../src/run-client.ts";

const hooks = (events: Array<[string, Record<string, unknown>]> = []): RunHooks => ({
    onEvent: (kind, data) => events.push([kind, data]),
    onApproval: async () => true,
    onQuestion: async () => "answer",
});
async function server(handler: (req: http.IncomingMessage, res: http.ServerResponse, body: string) => void): Promise<{ base: string; close: () => Promise<void> }> {
    const s = http.createServer((req, res) => {
        const chunks: Buffer[] = [];
        req.on("data", (b: Buffer) => chunks.push(b));
        req.on("end", () => handler(req, res, Buffer.concat(chunks).toString("utf8")));
    });
    await new Promise<void>((resolve) => s.listen(0, "127.0.0.1", resolve));
    return { base: `http://127.0.0.1:${(s.address() as AddressInfo).port}`, close: () => new Promise((r) => s.close(() => r())) };
}
const json = (res: http.ServerResponse, code: number, data: unknown) => { res.writeHead(code, { "Content-Type": "application/json" }); res.end(JSON.stringify(data)); };
const health = { status: "ok", protocol: "gregal.v1", instance_id: "instance-1", capabilities: { runs: true, durable_events: true, events: true, interactive_events: true } };

test("v2 sends mode and workspace and emits run events", async () => {
    let payload: any;
    const seen: Array<[string, Record<string, unknown>]> = [];
    const s = await server((req, res, body) => {
        if (req.url === "/api/health") return json(res, 200, health);
        if (req.url === "/api/v2/runs") { payload = JSON.parse(body); return json(res, 202, { run: { id: 8, state: "queued" } }); }
        if (req.url?.startsWith("/api/v2/events")) return json(res, 200, { events: [
            { id: 1, run_id: 8, kind: "text", text: "héllo" },
            { id: 2, run_id: 8, kind: "tool_call", text: "🔧 read", payload: { id: "call-1", name: "read", args: { path: "x" } } },
            { id: 3, run_id: 8, kind: "run_completed", text: "done" },
        ], next: 3 });
        json(res, 404, {});
    });
    try {
        await runAgent({ base: s.base, token: "tok", session: "sess" }, { task: "task", mode: "inspect", workspace: "C:/repo" }, hooks(seen), new AbortController().signal);
        assert.deepEqual(payload, { task: "task", mode: "inspect", workspace: "C:/repo", idempotency_key: payload.idempotency_key });
        assert.equal(seen[0][0], "assistant");
        assert.equal(seen[0][1].text, "héllo");
        assert.equal(seen[1][0], "tool_call");
        assert.equal(seen[1][1].id, "call-1");
        assert.equal(seen[1][1].event_id, 2);
        assert.equal(seen[1][1].text, undefined);
        assert.equal(seen[1][1].event_text, "🔧 read");
        assert.equal(seen[2][0], "status");
        assert.equal(seen[3][0], "done");
    } finally { await s.close(); }
});

test("approval posts its key on the frozen connection then resumes running", async () => {
    const seen: Array<[string, Record<string, unknown>]> = [];
    let approvalBody: Record<string, unknown> | undefined;
    let approvalSession = "";
    let approvalAuthorization = "";
    const s = await server((req, res, body) => {
        if (req.url === "/api/health") return json(res, 200, health);
        if (req.url === "/api/v2/runs") return json(res, 202, { run: { id: 81, state: "queued" }, cursor: 0 });
        if (req.url?.startsWith("/api/v2/events")) {
            const after = Number(new URL(req.url, s.base).searchParams.get("after"));
            if (!after) return json(res, 200, { events: [{ id: 1, run_id: 81, kind: "approve_request", payload: { key: "approve-7", name: "write", args: "{file:x}" } }], next: 1 });
            return json(res, 200, { events: [{ id: 2, run_id: 81, kind: "run_completed", text: "completed" }], next: 2 });
        }
        if (req.url === "/api/approve") {
            approvalBody = JSON.parse(body);
            approvalSession = String(req.headers["x-gregal-session"] || "");
            approvalAuthorization = String(req.headers.authorization || "");
            return json(res, 200, { ok: true });
        }
        json(res, 404, {});
    });
    try {
        await runAgent({ base: s.base, token: "frozen-token", session: "frozen-session" }, { task: "t", mode: "code" }, {
            ...hooks(seen),
            onApproval: async (payload) => { assert.equal(payload.key, "approve-7"); assert.equal(payload.name, "write"); return true; },
        }, new AbortController().signal);
        assert.deepEqual(approvalBody, { key: "approve-7", approve: true });
        assert.equal(approvalSession, "frozen-session");
        assert.equal(approvalAuthorization, "Bearer frozen-token");
        const approvalIndex = seen.findIndex(([kind]) => kind === "approve_request");
        assert.equal(seen[approvalIndex + 1][0], "status");
        assert.deepEqual(seen[approvalIndex + 1][1], { state: "running" });
    } finally { await s.close(); }
});

test("explicit unhealthy status or unsupported protocol fails before legacy fallback", async () => {
    for (const healthResponse of [
        { status: "starting", protocol: "gregal.v1", instance_id: "x", capabilities: {} },
        { status: "ok", protocol: "gregal.v2", instance_id: "x", capabilities: {} },
    ]) {
        let legacyPosts = 0;
        const s = await server((req, res) => {
            if (req.url === "/api/health") return json(res, 200, healthResponse);
            if (req.url === "/api/agent") legacyPosts++;
            json(res, 404, {});
        });
        try {
            await assert.rejects(runAgent({ base: s.base, token: "", session: "s" }, { task: "t", mode: "code" }, hooks(), new AbortController().signal), /health check failed|unsupported.*protocol/i);
            assert.equal(legacyPosts, 0);
        } finally { await s.close(); }
    }
});

test("server cancellation is reported as status, not an error", async () => {
    const seen: Array<[string, Record<string, unknown>]> = [];
    const s = await server((req, res) => {
        if (req.url === "/api/health") return json(res, 200, health);
        if (req.url === "/api/v2/runs") return json(res, 202, { run: { id: 3, state: "queued" } });
        if (req.url?.startsWith("/api/v2/events")) return json(res, 200, { events: [{ id: 1, run_id: 3, kind: "run_cancelled", text: "cancelled" }], next: 1 });
        json(res, 404, {});
    });
    try {
        await runAgent({ base: s.base, token: "", session: "s" }, { task: "t", mode: "code" }, hooks(seen), new AbortController().signal);
        assert.ok(seen.some(([kind, data]) => kind === "status" && data.state === "cancelled"));
        assert.equal(seen.some(([kind]) => kind === "error"), false);
    } finally { await s.close(); }
});

test("abort after submit cancels the exact run", async () => {
    let submitted!: () => void;
    const ready = new Promise<void>((r) => { submitted = r; });
    let cancelled = false;
    const s = await server((req, res) => {
        if (req.url === "/api/health") return json(res, 200, health);
        if (req.url === "/api/v2/runs") { json(res, 202, { run: { id: 42, state: "queued" } }); submitted(); return; }
        if (req.url?.startsWith("/api/v2/events")) { res.writeHead(200, { "Content-Type": "application/json" }); return; }
        if (req.url === "/api/v2/runs/42/cancel") { cancelled = true; return json(res, 200, { ok: true, cancelled: true }); }
        json(res, 404, {});
    });
    const controller = new AbortController();
    try {
        const running = runAgent({ base: s.base, token: "", session: "s" }, { task: "t", mode: "code" }, hooks(), controller.signal);
        await ready;
        controller.abort();
        await assert.rejects(running, /abort|cancel/i);
        for (let i = 0; i < 20 && !cancelled; i++) await new Promise((r) => setTimeout(r, 10));
        assert.equal(cancelled, true);
    } finally { await s.close(); }
});

test("abort during a lost submit acknowledgement recovers by idempotency key and cancels", async () => {
    let submitted!: () => void;
    const ready = new Promise<void>((r) => { submitted = r; });
    let key = "";
    let cancelled = false;
    const seen: Array<[string, Record<string, unknown>]> = [];
    const s = await server((req, res, body) => {
        if (req.url === "/api/health") return json(res, 200, health);
        if (req.url === "/api/v2/runs" && req.method === "POST") { key = JSON.parse(body).idempotency_key; submitted(); res.destroy(); return; }
        if (req.url === "/api/v2/runs" && req.method === "GET") { setTimeout(() => json(res, 200, { runs: [{ id: 43, key, state: "queued" }] }), 25); return; }
        if (req.url === "/api/v2/runs/43/cancel") { cancelled = true; return json(res, 200, { ok: true, cancelled: true }); }
        json(res, 404, {});
    });
    const controller = new AbortController();
    try {
        const running = runAgent({ base: s.base, token: "", session: "s" }, { task: "t", mode: "code" }, hooks(seen), controller.signal);
        await ready;
        controller.abort();
        await assert.rejects(running, /abort|cancel/i);
        assert.equal(key.length > 0, true);
        assert.equal(cancelled, true);
        assert.ok(seen.some(([kind, data]) => kind === "status" && data.state === "stopping"));
    } finally { await s.close(); }
});

test("a lost run after a server restart fails instead of polling forever", async () => {
    let cancelled = false;
    const s = await server((req, res) => {
        if (req.url === "/api/health") return json(res, 200, health);
        if (req.url === "/api/v2/runs") return json(res, 202, { run: { id: 9, state: "running" } });
        if (req.url?.startsWith("/api/v2/events")) return json(res, 200, { events: [], next: 0 });
        if (req.url === "/api/v2/runs/9/cancel") { cancelled = true; return json(res, 200, {}); }
        json(res, 404, {});
    });
    try {
        await assert.rejects(runAgent({ base: s.base, token: "", session: "s" }, { task: "t", mode: "chat" }, hooks(), new AbortController().signal), /no longer available/);
        assert.equal(cancelled, false, "a missing ID must not be cancelled after a restart");
    } finally { await s.close(); }
});

test("a reused run ID cannot be mistaken for the submitted task", async () => {
    let cancelled = false;
    const s = await server((req, res) => {
        if (req.url === "/api/health") return json(res, 200, health);
        if (req.url === "/api/v2/runs") return json(res, 202, { run: { id: 9, state: "running" } });
        if (req.url?.startsWith("/api/v2/events")) return json(res, 200, { events: [], next: 0 });
        if (req.url === "/api/v2/runs/9") return json(res, 200, { run: { id: 9, state: "running", key: "another-task" } });
        if (req.url === "/api/v2/runs/9/cancel") { cancelled = true; return json(res, 200, {}); }
        json(res, 404, {});
    });
    try {
        await assert.rejects(runAgent({ base: s.base, token: "", session: "s" }, { task: "t", mode: "chat" }, hooks(), new AbortController().signal), /identity changed/);
        assert.equal(cancelled, false);
    } finally { await s.close(); }
});

test("v2 failed terminal event rejects with the server reason", async () => {
    const s = await server((req, res) => {
        if (req.url === "/api/health") return json(res, 200, health);
        if (req.url === "/api/v2/runs") return json(res, 202, { run: { id: 9, state: "queued" } });
        if (req.url?.startsWith("/api/v2/events")) return json(res, 200, { events: [{ id: 1, run_id: 9, kind: "run_failed", payload: { error: "provider broke" } }], next: 1 });
        if (req.url === "/api/v2/runs/9/cancel") return json(res, 200, { ok: true, cancelled: true });
        json(res, 404, {});
    });
    try { await assert.rejects(runAgent({ base: s.base, token: "", session: "s" }, { task: "t", mode: "chat" }, hooks(), new AbortController().signal), /provider broke/); }
    finally { await s.close(); }
});

test("done and verification do not end a run or replace its reply with lifecycle text", async () => {
    const seen: Array<[string, Record<string, unknown>]> = [];
    const s = await server((req, res) => {
        if (req.url === "/api/health") return json(res, 200, health);
        if (req.url === "/api/v2/runs") return json(res, 202, { run: { id: 18, state: "queued" }, cursor: 40 });
        if (req.url?.startsWith("/api/v2/events")) {
            assert.match(req.url, /after=40/);
            return json(res, 200, { events: [
                { id: 41, run_id: 18, kind: "done", text: "the actual answer" },
                { id: 42, run_id: 18, kind: "verify", text: "verification continues" },
                { id: 43, run_id: 18, kind: "run_completed", text: "torn completat" },
            ], next: 43 });
        }
        if (req.url === "/api/v2/runs/18") return json(res, 200, { run: { id: 18, state: "completed" } });
        json(res, 404, {});
    });
    try {
        await runAgent({ base: s.base, token: "", session: "s" }, { task: "t", mode: "code" }, hooks(seen), new AbortController().signal);
        assert.equal(seen.find(([kind]) => kind === "verify")?.[1].event_text, "verification continues");
        assert.equal(seen.at(-1)?.[0], "done");
        assert.equal(seen.at(-1)?.[1].reply, "the actual answer");
        assert.equal(seen.some(([kind]) => kind === "error"), false);
    } finally { await s.close(); }
});

test("drains paginated durable backlog before using run status fallback", async () => {
    let getCalls = 0;
    let eventCalls = 0;
    const seen: Array<[string, Record<string, unknown>]> = [];
    const s = await server((req, res) => {
        if (req.url === "/api/health") return json(res, 200, health);
        if (req.url === "/api/v2/runs") return json(res, 202, { run: { id: 22, state: "queued" }, cursor: 9 });
        if (req.url?.startsWith("/api/v2/events")) {
            eventCalls++;
            const after = Number(new URL(req.url, s.base).searchParams.get("after"));
            if (after === 9) return json(res, 200, { events: [{ id: 10, run_id: 22, kind: "run_started", text: "started" }], next: 10 });
            return json(res, 200, { events: [
                { id: 11, run_id: 22, kind: "text", text: "answer" },
                { id: 12, run_id: 22, kind: "run_completed", text: "torn completat" },
            ], next: 12 });
        }
        if (req.url === "/api/v2/runs/22") { getCalls++; return json(res, 200, { run: { id: 22, state: "completed" } }); }
        json(res, 404, {});
    });
    try {
        await runAgent({ base: s.base, token: "", session: "s" }, { task: "t", mode: "code" }, hooks(seen), new AbortController().signal);
        assert.equal(eventCalls, 2);
        assert.equal(getCalls, 0);
        assert.equal(seen.at(-1)?.[1].reply, "answer");
    } finally { await s.close(); }
});

test("getRun recovers a terminal result after the durable event history is empty", async () => {
    let polls = 0;
    let statusChecks = 0;
    const seen: Array<[string, Record<string, unknown>]> = [];
    const s = await server((req, res) => {
        if (req.url === "/api/health") return json(res, 200, health);
        if (req.url === "/api/v2/runs") return json(res, 202, { run: { id: 24, state: "queued" }, cursor: 800 });
        if (req.url?.startsWith("/api/v2/events")) { polls++; return json(res, 200, { events: [], next: 800 }); }
        if (req.url === "/api/v2/runs/24") { statusChecks++; return json(res, 200, { run: { id: 24, state: "completed" } }); }
        json(res, 404, {});
    });
    try {
        await runAgent({ base: s.base, token: "", session: "s" }, { task: "t", mode: "code" }, hooks(seen), new AbortController().signal);
        assert.equal(polls, 3);
        assert.equal(statusChecks, 1);
        assert.equal(seen.at(-1)?.[0], "done");
    } finally { await s.close(); }
});

test("temporary event errors retry polling without resubmitting the task", async () => {
    let submissions = 0;
    let polls = 0;
    const s = await server((req, res) => {
        if (req.url === "/api/health") return json(res, 200, health);
        if (req.url === "/api/v2/runs") { submissions++; return json(res, 202, { run: { id: 12, state: "queued" } }); }
        if (req.url?.startsWith("/api/v2/events")) {
            if (polls++ === 0) return json(res, 503, { error: "temporary" });
            return json(res, 200, { events: [{ id: 1, run_id: 12, kind: "run_completed" }], next: 1 });
        }
        json(res, 404, {});
    });
    try {
        await runAgent({ base: s.base, token: "", session: "s" }, { task: "t", mode: "code" }, hooks(), new AbortController().signal);
        assert.equal(submissions, 1);
        assert.equal(polls, 2);
    } finally { await s.close(); }
});

test("network health failure does not replay the task through legacy API", async () => {
    let legacyPosts = 0;
    const s = await server((req, res) => {
        if (req.url === "/api/health") return json(res, 503, { error: "unavailable" });
        if (req.url === "/api/agent") legacyPosts++;
        json(res, 503, {});
    });
    try {
        await assert.rejects(runAgent({ base: s.base, token: "", session: "s" }, { task: "t", mode: "code" }, hooks(), new AbortController().signal), /503/);
        assert.equal(legacyPosts, 0);
    } finally { await s.close(); }
});

test("legacy stream decodes a UTF-8 character split between chunks", async () => {
    const text = 'event: assistant\r\ndata: {"text":"olá 🌊"}\r\n\r\nevent: done\r\ndata: {"reply":"olá 🌊"}\r\n\r\n';
    const bytes = Buffer.from(text);
    const emoji = bytes.indexOf(Buffer.from("🌊"));
    const order: string[] = [];
    const s = await server((req, res) => {
        if (req.url === "/api/health") return json(res, 200, { status: "ok", capabilities: {} });
        if (req.url === "/api/workspaces") { order.push("workspace"); return json(res, 200, { current: "C:/repo" }); }
        if (req.url === "/api/agent") { order.push("agent"); res.writeHead(200, { "Content-Type": "text/event-stream" }); res.write(bytes.subarray(0, emoji + 2)); setTimeout(() => res.end(bytes.subarray(emoji + 2)), 5); return; }
        if (req.url === "/api/agent/cancel") return json(res, 200, { ok: true });
        json(res, 404, {});
    });
    const seen: Array<[string, Record<string, unknown>]> = [];
    try {
        await runAgent({ base: s.base, token: "", session: "s" }, { task: "t", mode: "chat", workspace: "C:/repo" }, hooks(seen), new AbortController().signal);
        assert.deepEqual(order, ["workspace", "agent"]);
        assert.equal(seen[0][1].text, "olá 🌊");
        assert.equal(seen[1][0], "done");
    } finally { await s.close(); }
});

test("parallel runs keep their connection settings isolated", async () => {
    const calls: string[] = [];
    const make = (label: string) => server((req, res) => {
        calls.push(`${label}:${req.headers["x-gregal-session"] || ""}`);
        if (req.url === "/api/health") return json(res, 200, health);
        if (req.url === "/api/v2/runs") return json(res, 202, { run: { id: 1, state: "queued" } });
        if (req.url?.startsWith("/api/v2/events")) return json(res, 200, { events: [{ id: 1, run_id: 1, kind: "run_completed" }], next: 1 });
        json(res, 404, {});
    });
    const a = await make("a"); const b = await make("b");
    try {
        await Promise.all([
            runAgent({ base: a.base, token: "", session: "alpha" }, { task: "a", mode: "code" }, hooks(), new AbortController().signal),
            runAgent({ base: b.base, token: "", session: "beta" }, { task: "b", mode: "inspect" }, hooks(), new AbortController().signal),
        ]);
        assert.ok(calls.includes("a:alpha")); assert.ok(calls.includes("b:beta"));
    } finally { await a.close(); await b.close(); }
});
