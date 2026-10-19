import { test } from "node:test";
import assert from "node:assert/strict";
import * as http from "http";
import type { AddressInfo } from "net";
import {
    apiGet,
    apiHealth,
    apiPost,
    ApiError,
    cancelRun,
    listCheckpoints,
    listRunEvents,
    listSavedConversations,
    inlineCompletion,
    rewindAll,
    rewindTo,
    resumeConversation,
    submitRun,
} from "../src/api.ts";

function srv(handler: (req: http.IncomingMessage, body: string) => { code: number; json: unknown }): Promise<{ base: string; close: () => void }> {
    return new Promise((resolve) => {
        const s = http.createServer((req, res) => {
            let body = "";
            req.on("data", (d) => (body += d));
            req.on("end", () => {
                const r = handler(req, body);
                res.writeHead(r.code, { "Content-Type": "application/json" });
                res.end(JSON.stringify(r.json));
            });
        });
        s.listen(0, "127.0.0.1", () => {
            const a = s.address() as AddressInfo;
            resolve({ base: `http://127.0.0.1:${a.port}`, close: () => s.close() });
        });
    });
}

test("GET checkpoints amb Bearer", async () => {
    let seen = "";
    const { base, close } = await srv((req) => {
        seen = req.headers.authorization || "";
        return { code: 200, json: [{ seq: 1, op: "write", path: "f", at: "t" }] };
    });
    try {
        const cps = await listCheckpoints(base, "tok");
        assert.equal(cps.length, 1);
        assert.equal(cps[0].seq, 1);
        assert.equal(seen, "Bearer tok");
    } finally {
        close();
    }
});

test("llista i reprèn converses desades amb la sessió de projecte", async () => {
    const seen: string[] = [];
    const { base, close } = await srv((req, body) => {
        seen.push(`${req.method} ${req.url} ${req.headers["x-gregal-session"]}`);
        if (req.url === "/api/sessions") return { code: 200, json: [{ name: "saved-1", title: "Fix login", msgs: 4, at: "2026-10-19T09:00:00Z" }] };
        assert.equal(req.url, "/api/resume");
        assert.deepEqual(JSON.parse(body), { name: "saved-1" });
        return { code: 200, json: { name: "saved-1", transcript: [{ role: "user", content: "Fix login" }] } };
    });
    try {
        const rows = await listSavedConversations(base, "tok", "vscode-project");
        assert.equal(rows[0].title, "Fix login");
        const resumed = await resumeConversation(base, "tok", "vscode-project", rows[0].name);
        assert.equal(resumed.transcript[0].content, "Fix login");
        assert.deepEqual(seen, ["GET /api/sessions vscode-project", "POST /api/resume vscode-project"]);
    } finally { close(); }
});

test("inline completion sends only bounded cursor context and abort signal", async () => {
    let body: any;
    let session = "";
    const { base, close } = await srv((req, raw) => {
        body = JSON.parse(raw);
        session = String(req.headers["x-gregal-session"] || "");
        return { code: 200, json: { completion: "return value" } };
    });
    const abort = new AbortController();
    try {
        const result = await inlineCompletion(base, "tok", "vscode-project", {
            language: "go", prefix: "func f() {", suffix: "}", workspace: "C:/project",
        }, { signal: abort.signal });
        assert.equal(result.completion, "return value");
        assert.deepEqual(body, { language: "go", prefix: "func f() {", suffix: "}", workspace: "C:/project" });
        assert.equal(session, "vscode-project");
    } finally { close(); }
});

test("401 sense token", async () => {
    const { base, close } = await srv(() => ({ code: 401, json: { error: "cal token" } }));
    try {
        await assert.rejects(() => apiGet(base, "", "/api/checkpoints"), /401/);
    } finally {
        close();
    }
});

test("rewind-to envia seq", async () => {
    let body = "";
    const { base, close } = await srv((req, b) => {
        body = b;
        assert.equal(req.method, "POST");
        return { code: 200, json: { summary: "fet" } };
    });
    try {
        const r1 = await rewindTo(base, "tok", 2);
        assert.equal(r1.summary, "fet");
        assert.match(body, /"seq":2/);
        const r2 = await rewindAll(base, "tok");
        assert.equal(r2.summary, "fet");
    } finally {
        close();
    }
});

test("resposta il·legible", async () => {
    const s = http.createServer((_req, res) => {
        res.writeHead(200, { "Content-Type": "application/json" });
        res.end("no-json{{{");
    });
    await new Promise<void>((r) => s.listen(0, "127.0.0.1", r));
    const a = s.address() as AddressInfo;
    try {
        await assert.rejects(() => apiPost(`http://127.0.0.1:${a.port}`, "", "/x", {}), /Invalid response/);
    } finally {
        s.close();
    }
});

test("submit v2 accepta 202 i envia la sessió/idempotència", async () => {
    let seenSession = "";
    let payload: any;
    const { base, close } = await srv((req, body) => {
        seenSession = String(req.headers["x-gregal-session"] || "");
        payload = JSON.parse(body);
        return { code: 202, json: { run: { id: 7, state: "queued" } } };
    });
    try {
        const r = await submitRun(base, "tok", "vscode-test", "fes-ho", "msg-1");
        assert.equal(r.run.id, 7);
        assert.equal(seenSession, "vscode-test");
        assert.equal(payload.idempotency_key, "msg-1");
        assert.equal(payload.task, "fes-ho");
    } finally {
        close();
    }
});

test("health negocia les capacitats v2 i envia la sessió", async () => {
    let seenSession = "";
    const { base, close } = await srv((req) => {
        seenSession = String(req.headers["x-gregal-session"] || "");
        return { code: 200, json: { status: "ok", protocol: "gregal.v1", instance_id: "test-instance", capabilities: { runs: true, durable_events: true, events: true, event_stream: true, interactive_events: true } } };
    });
    try {
        const health = await apiHealth(base, "tok", "vscode-test");
        assert.equal(health.capabilities?.runs, true);
        assert.equal(health.capabilities?.durable_events, true);
        assert.equal(health.capabilities?.events, true);
        assert.equal(health.capabilities?.interactive_events, true);
        assert.equal(seenSession, "vscode-test");
    } finally {
        close();
    }
});

test("events v2 i cancel·lació mantenen sessió i cursor", async () => {
    const seen: string[] = [];
    const { base, close } = await srv((req) => {
        seen.push(`${req.method} ${req.url} ${req.headers["x-gregal-session"]}`);
        if (req.url?.startsWith("/api/v2/events")) {
            return { code: 200, json: { events: [{ id: 4, run_id: 7, kind: "text", text: "hola" }], cursor: 3, next: 4 } };
        }
        return { code: 200, json: { ok: true, cancelled: true } };
    });
    try {
        const page = await listRunEvents(base, "tok", "sess", 3);
        assert.equal(page.events[0].run_id, 7);
        assert.match(seen[0], /after=3/);
        assert.match(seen[0], /limit=100/);
        assert.match(seen[0], /session=sess/);
        assert.match(seen[0], /sess$/);
        const result = await cancelRun(base, "tok", "sess", 7);
        assert.equal(result.cancelled, true);
        assert.match(seen[1], /POST \/api\/v2\/runs\/7\/cancel sess$/);
    } finally {
        close();
    }
});

test("ApiError conserva el status per decidir fallback", async () => {
    const { base, close } = await srv(() => ({ code: 404, json: { error: "missing" } }));
    try {
        await assert.rejects(
            () => apiPost(base, "", "/api/v2/runs", {}),
            (e: unknown) => e instanceof ApiError && e.status === 404,
        );
    } finally {
        close();
    }
});

test("apiGet aborta peticions i limita la mida de resposta", async () => {
    const s = http.createServer((_req, res) => {
        res.writeHead(200, { "Content-Type": "application/json" });
        res.write('{"large":"');
        setTimeout(() => res.end("later".repeat(100) + '"}'), 100);
    });
    await new Promise<void>((r) => s.listen(0, "127.0.0.1", r));
    const a = s.address() as AddressInfo;
    const base = `http://127.0.0.1:${a.port}`;
    try {
        await assert.rejects(() => apiGet(base, "", "/", { maxResponseBytes: 32 }), /size limit/i);
        const controller = new AbortController();
        const pending = apiGet(base, "", "/", { signal: controller.signal });
        controller.abort();
        await assert.rejects(pending, /abort/i);
    } finally { s.close(); }
});
