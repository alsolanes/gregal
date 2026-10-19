import * as http from "http";
import * as https from "https";
import { randomUUID } from "crypto";
import { API_PROTOCOL, ApiError, apiHealth, apiPost, cancelRun, getRun, listRunEvents, listRuns, submitRun, type DurableEvent, type Run } from "./api.ts";
import { durableInteraction, feedSse } from "./sse.ts";

export interface RunConnection { base: string; token: string; session: string }
export interface RunRequest { task: string; mode: "code" | "chat" | "inspect"; workspace?: string }
export interface RunHooks {
    onEvent: (kind: string, data: Record<string, unknown>) => void;
    onApproval: (payload: Record<string, unknown>) => Promise<boolean>;
    onQuestion: (payload: Record<string, unknown>) => Promise<string>;
}

const pause = (ms: number, signal: AbortSignal) => new Promise<void>((resolve, reject) => {
    if (signal.aborted) { reject(abortError(signal)); return; }
    const timer = setTimeout(() => { signal.removeEventListener("abort", stop); resolve(); }, ms);
    const stop = () => { clearTimeout(timer); reject(abortError(signal)); };
    signal.addEventListener("abort", stop, { once: true });
});

function abortError(signal: AbortSignal): Error { return signal.reason instanceof Error ? signal.reason : new Error("Run cancelled"); }
function checkAbort(signal: AbortSignal): void { if (signal.aborted) throw abortError(signal); }
function withAbort<T>(promise: Promise<T>, signal: AbortSignal): Promise<T> {
    if (signal.aborted) return Promise.reject(abortError(signal));
    return new Promise<T>((resolve, reject) => {
        const stop = () => reject(abortError(signal));
        signal.addEventListener("abort", stop, { once: true });
        promise.then((v) => { signal.removeEventListener("abort", stop); resolve(v); }, (e) => { signal.removeEventListener("abort", stop); reject(e); });
    });
}

function payloadObject(value: unknown): Record<string, unknown> {
    if (typeof value === "string") { try { value = JSON.parse(value); } catch { return { text: value }; } }
    return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {};
}
function eventData(ev: DurableEvent): Record<string, unknown> {
    const structured = payloadObject(ev.payload ?? ev.data ?? ev.details ?? ev.interaction ?? ev.meta);
    const data: Record<string, unknown> = { ...structured, event_id: ev.id, run_id: ev.run_id, at: ev.at, event_kind: ev.kind };
    if (data.kind === undefined) data.kind = ev.kind;
    if (ev.text !== undefined) {
        if (["token", "text", "done", "error"].includes(ev.kind) && data.text === undefined) data.text = ev.text;
        else data.event_text = ev.text;
    }
    return data;
}
function errorText(data: Record<string, unknown>, fallback = "Run failed"): string {
    return String(data.message || data.error || data.text || data.event_text || fallback);
}
function terminalState(state: string): boolean { return ["completed", "failed", "cancelled"].includes(state); }

async function handleInteraction(kind: string, data: Record<string, unknown>, hooks: RunHooks, connection: RunConnection, signal: AbortSignal): Promise<void> {
    const interaction = durableInteraction({ kind, ...data });
    if (!interaction || typeof interaction.key !== "string") throw new Error("Interactive event has no structured payload");
    if (kind === "approve_request") {
        const approve = await withAbort(hooks.onApproval(interaction), signal);
        checkAbort(signal);
        await apiPost(connection.base, connection.token, "/api/approve", { key: interaction.key, approve }, { session: connection.session, signal });
    } else {
        const answer = await withAbort(hooks.onQuestion(interaction), signal);
        checkAbort(signal);
        await apiPost(connection.base, connection.token, "/api/question", { key: interaction.key, answer }, { session: connection.session, signal });
    }
    hooks.onEvent("status", { state: "running" });
}

async function cancelFrozen(connection: RunConnection, id?: number): Promise<void> {
    // Cancellation must outlive the signal that initiated it.
    if (id !== undefined) await cancelRun(connection.base, connection.token, connection.session, id, undefined);
    else await apiPost(connection.base, connection.token, "/api/agent/cancel", {}, { session: connection.session });
}

export async function runAgent(connection: RunConnection, request: RunRequest, hooks: RunHooks, signal: AbortSignal): Promise<void> {
    // Snapshot all routing/auth data so settings changes cannot redirect a live turn.
    const conn: RunConnection = Object.freeze({ base: connection.base, token: connection.token, session: connection.session });
    const req: RunRequest = Object.freeze({ ...request });
    checkAbort(signal);
    let health;
    try { health = await apiHealth(conn.base, conn.token, conn.session, { signal }); }
    catch (e) {
        if (e instanceof ApiError && [404, 405, 501].includes(e.status)) return runLegacy(conn, req, hooks, signal);
        throw e;
    }
    if (health.status !== undefined && health.status !== "ok") throw new Error(`Gregal health check failed: ${String(health.status)}`);
    if (health.protocol !== undefined && health.protocol !== API_PROTOCOL) throw new Error(`Unsupported Gregal API protocol: ${String(health.protocol)}`);
    const caps = health.capabilities || {};
    const compatibleHealth = health.status === "ok" && health.protocol === API_PROTOCOL && !!health.instance_id &&
        !!caps.runs && !!caps.durable_events && !!caps.events && !!caps.interactive_events;
    if (!compatibleHealth) return runLegacy(conn, req, hooks, signal);

    // Finish submission even if cancellation arrives in flight, so its run ID is
    // known and can be cancelled instead of leaving an orphaned worker.
    checkAbort(signal);
    const idempotencyKey = randomUUID();
    let submitAborted = false;
    const onSubmitAbort = () => {
        submitAborted = true;
        hooks.onEvent("status", { state: "stopping" });
    };
    signal.addEventListener("abort", onSubmitAbort, { once: true });
    let submitted;
    try { submitted = await submitRun(conn.base, conn.token, conn.session, req.task, idempotencyKey, req.mode, req.workspace, undefined, 10_000); }
    catch (e) {
        if (!signal.aborted && e instanceof ApiError && [404, 405, 501].includes(e.status)) {
            signal.removeEventListener("abort", onSubmitAbort);
            return runLegacy(conn, req, hooks, signal);
        }
        // A lost acknowledgement may still have queued the run. Recover by the
        // idempotency key before deciding the submission failed.
        try {
            const page = await listRuns(conn.base, conn.token, conn.session, { timeoutMs: 10_000 });
            const recovered = page.runs?.find((run) => run.key === idempotencyKey);
            if (recovered) submitted = { run: recovered, cursor: 0 };
            else {
                signal.removeEventListener("abort", onSubmitAbort);
                if (signal.aborted) throw abortError(signal);
                throw e;
            }
        } catch (recoveryError) {
            signal.removeEventListener("abort", onSubmitAbort);
            if (signal.aborted) throw abortError(signal);
            if (recoveryError === e) throw e;
            throw e;
        }
    }
    signal.removeEventListener("abort", onSubmitAbort);
    if (!submitted.run || !Number.isFinite(submitted.run.id)) throw new Error("Incompatible v2 response: missing run ID");
    const id = submitted.run.id;
    if (submitAborted || signal.aborted) {
        await cancelFrozen(conn, id).catch(() => {});
        throw abortError(signal);
    }
    let terminalSeen = false;
    let cursor = Number.isFinite(submitted.cursor) ? submitted.cursor! : 0;
    let transientFailures = 0;
    let emptyPolls = 0;
    let statusFailures = 0;
    let assistantReply = "";
    let lastRun: Run = submitted.run;
    const onAbort = () => { void cancelFrozen(conn, id).catch(() => {}); };
    signal.addEventListener("abort", onAbort, { once: true });
    try {
        if (signal.aborted) onAbort();
        while (!terminalSeen) {
            checkAbort(signal);
            let page;
            try {
                page = await listRunEvents(conn.base, conn.token, conn.session, cursor, 100, { signal });
                transientFailures = 0;
            } catch (e) {
                if (signal.aborted) throw abortError(signal);
                if (e instanceof ApiError && [404, 405, 501].includes(e.status)) {
                    // The task is already submitted; never replay it on the legacy API.
                    throw new Error("Durable events became unavailable after the run was submitted");
                }
                if (++transientFailures > 6) throw e;
                await pause(Math.min(250 * 2 ** (transientFailures - 1), 4000), signal);
                continue;
            }
            const events = Array.isArray(page.events) ? page.events : [];
            let maxId = cursor;
            for (const ev of events) {
                if (Number.isFinite(ev.id)) maxId = Math.max(maxId, ev.id);
                if (ev.run_id !== id) continue;
                const data = eventData(ev);
                const kind = ev.kind;
                if (kind === "approve_request" || kind === "question_request") {
                    hooks.onEvent(kind, data);
                    await handleInteraction(kind, data, hooks, conn, signal);
                } else if (kind === "token") hooks.onEvent("token", data);
                else if (kind === "text") {
                    assistantReply = String(data.text || "");
                    hooks.onEvent("assistant", data);
                }
                else if (kind === "run_queued" || kind === "run_started") hooks.onEvent("status", { ...data, state: kind === "run_queued" ? "queued" : "running" });
                else if (kind === "done") {
                    const reply = String(data.reply || data.text || "");
                    if (reply) {
                        if (reply !== assistantReply) hooks.onEvent("assistant", { ...data, text: reply });
                        assistantReply = reply;
                    }
                }
                else if (kind === "run_completed") {
                    hooks.onEvent("status", { ...data, state: "completed" });
                    hooks.onEvent("done", assistantReply ? { run_id: id, reply: assistantReply } : { run_id: id });
                    terminalSeen = true;
                }
                else if (kind === "run_failed") {
                    const message = errorText(data);
                    hooks.onEvent("status", { ...data, state: "failed" });
                    hooks.onEvent("error", { ...data, message });
                    terminalSeen = true;
                    lastRun = { ...lastRun, state: "failed", error: message };
                }
                else if (kind === "run_cancelled") { hooks.onEvent("status", { ...data, state: "cancelled" }); terminalSeen = true; lastRun = { ...lastRun, state: "cancelled" }; }
                else {
                    hooks.onEvent(kind, data);
                    if (kind === "done") terminalSeen = true;
                    if (kind === "error") { terminalSeen = true; lastRun = { ...lastRun, state: "failed", error: errorText(data) }; }
                }
            }
            cursor = Math.max(cursor, Number.isFinite(page.next) ? page.next : maxId);
            if (!events.length) emptyPolls++; else emptyPolls = 0;

            // Durable event logs can be unavailable after restart; run state is
            // the bounded recovery path when the event feed is quiet.
            if (!terminalSeen && emptyPolls >= 3 && events.length === 0) {
                try {
                    const current = await getRun(conn.base, conn.token, conn.session, id, { signal });
                    if (!current.run || current.run.id !== id || (current.run.key && current.run.key !== idempotencyKey)) {
                        terminalSeen = true;
                        throw new Error("Run identity changed; reconnect to the server before starting another run");
                    }
                    statusFailures = 0;
                    lastRun = current.run;
                    hooks.onEvent("status", { state: current.run.state, run_id: id });
                    if (terminalState(current.run.state)) {
                        terminalSeen = true;
                        if (current.run.state === "completed") hooks.onEvent("done", assistantReply ? { run_id: id, state: "completed", reply: assistantReply } : { run_id: id, state: "completed" });
                        else if (current.run.state === "failed") hooks.onEvent("error", { run_id: id, state: "failed", message: current.run.error || "Run failed" });
                        else hooks.onEvent("status", { run_id: id, state: "cancelled" });
                    }
                } catch (e) {
                    if (signal.aborted) throw abortError(signal);
                    if (terminalSeen) throw e;
                    if (e instanceof ApiError && [404, 405, 501].includes(e.status)) {
                        // A server restart can discard its in-memory queue. Do not
                        // spin forever or cancel an ID that could now belong to another run.
                        terminalSeen = true;
                        throw new Error("Run status is no longer available; reconnect to the server");
                    } else {
                        if (++statusFailures > 6) throw e;
                        await pause(Math.min(250 * 2 ** (statusFailures - 1), 4000), signal);
                    }
                }
            }
            if (!terminalSeen) await pause(events.length ? 100 : 350, signal);
        }
        if (lastRun.state === "failed") throw new Error(lastRun.error || "Run failed");
    } catch (e) {
        if (!terminalSeen && !(signal.aborted)) await cancelFrozen(conn, id).catch(() => {});
        throw e;
    } finally {
        signal.removeEventListener("abort", onAbort);
    }
}

async function runLegacy(conn: RunConnection, request: RunRequest, hooks: RunHooks, signal: AbortSignal): Promise<void> {
    checkAbort(signal);
    if (request.workspace) {
        await apiPost(conn.base, conn.token, "/api/workspaces", { path: request.workspace }, { session: conn.session, signal });
    }
    const body = JSON.stringify({ task: request.task, mode: request.mode });
    const u = new URL(conn.base.replace(/\/$/, "") + "/api/agent");
    const lib = u.protocol === "https:" ? https : http;
    await new Promise<void>((resolve, reject) => {
        let settled = false;
        let done = false;
        let carry = "";
        const pending: Promise<void>[] = [];
        const decoder = new TextDecoder("utf-8");
        const finish = (error?: Error) => {
            if (settled) return;
            settled = true;
            signal.removeEventListener("abort", abort);
            if (error && !done && !signal.aborted) void cancelFrozen(conn).catch(() => {});
            if (error) reject(error); else resolve();
        };
        const process = (chunk: string) => {
            const parsed = feedSse(chunk, carry); carry = parsed.rest;
            for (const ev of parsed.evs) {
                let data: Record<string, unknown>;
                try { data = payloadObject(ev.data); } catch { throw new Error("Invalid SSE event payload"); }
                if (ev.name === "error") { hooks.onEvent("error", data); throw new Error(errorText(data)); }
                if (ev.name === "done") { hooks.onEvent("done", data); done = true; continue; }
                if (ev.name === "approve_request" || ev.name === "question_request") {
                    hooks.onEvent(ev.name, data);
                    const prompt = withAbort(handleInteraction(ev.name, data, hooks, conn, signal), signal);
                    pending.push(prompt);
                    void prompt.catch((e) => req.destroy(e instanceof Error ? e : new Error(String(e))));
                    continue;
                }
                hooks.onEvent(ev.name, data);
            }
        };
        if (signal.aborted) { finish(abortError(signal)); return; }
        const req = lib.request(u, { method: "POST", headers: {
            "Content-Type": "application/json", "Content-Length": Buffer.byteLength(body),
            ...(conn.token ? { Authorization: "Bearer " + conn.token } : {}), ...(conn.session ? { "X-Gregal-Session": conn.session } : {}),
        } }, (res) => {
            if (res.statusCode !== 200) { res.resume(); finish(new Error("Server: HTTP " + res.statusCode)); return; }
            res.on("data", (chunk: Buffer) => { try { process(decoder.decode(chunk, { stream: true })); } catch (e) { req.destroy(e instanceof Error ? e : new Error(String(e))); } });
            res.on("end", () => {
                try { process(decoder.decode()); } catch (e) { finish(e instanceof Error ? e : new Error(String(e))); return; }
                Promise.all(pending).then(() => {
                    if (!done) finish(new Error("Legacy stream ended before a done event")); else finish();
                }, (e) => finish(e instanceof Error ? e : new Error(String(e))));
            });
            res.on("aborted", () => finish(new Error("Legacy stream was truncated")));
            res.on("error", (e) => finish(e));
        });
        const abort = () => { req.destroy(abortError(signal)); void cancelFrozen(conn).catch(() => {}); };
        signal.addEventListener("abort", abort, { once: true });
        req.on("error", (e) => finish(e));
        req.setTimeout(10 * 60 * 1000, () => req.destroy(new Error("Legacy stream timed out")));
        req.end(body);
    });
}
