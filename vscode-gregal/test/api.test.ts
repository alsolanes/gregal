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
    rewindAll,
    rewindTo,
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
        return { code: 200, json: { status: "ok", capabilities: { runs: true, durable_events: true, interactive_events: true } } };
    });
    try {
        const health = await apiHealth(base, "tok", "vscode-test");
        assert.equal(health.capabilities?.runs, true);
        assert.equal(health.capabilities?.durable_events, true);
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
