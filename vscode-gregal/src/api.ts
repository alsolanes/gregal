import * as http from "http";
import * as https from "https";

export interface ApiOptions {
    session?: string;
    acceptedStatuses?: number[];
    signal?: AbortSignal;
    timeoutMs?: number;
    maxResponseBytes?: number;
}

/** Error HTTP amb el codi disponible perquè els clients puguin fer fallback. */
export class ApiError extends Error {
    readonly status: number;
    readonly body: string;

    constructor(status: number, body: string) {
        super("Server: HTTP " + status + " " + body.slice(0, 200));
        this.name = "ApiError";
        this.status = status;
        this.body = body;
    }
}

const MAX_RESPONSE_BYTES = 4 * 1024 * 1024;
const DEFAULT_TIMEOUT_MS = 60 * 1000;

function requestJson(method: "GET" | "POST", base: string, token: string, path: string, payload: unknown, options: ApiOptions): Promise<unknown> {
    const body = method === "POST" ? JSON.stringify(payload) : undefined;
    const u = new URL(base.replace(/\/$/, "") + path);
    const lib = u.protocol === "https:" ? https : http;
    return new Promise((resolve, reject) => {
        let settled = false;
        const finish = (fn: (value?: any) => void, value?: any) => {
            if (settled) return;
            settled = true;
            options.signal?.removeEventListener("abort", abort);
            fn(value);
        };
        const abort = () => req.destroy(options.signal?.reason instanceof Error ? options.signal.reason : new Error("Request aborted"));
        if (options.signal?.aborted) { reject(options.signal.reason instanceof Error ? options.signal.reason : new Error("Request aborted")); return; }
        const req = lib.request(u, {
            method,
            headers: {
                ...(body !== undefined ? { "Content-Type": "application/json", "Content-Length": Buffer.byteLength(body) } : {}),
                ...headers(token, options.session),
            },
        }, (res) => {
            const chunks: Buffer[] = [];
            let size = 0;
            const max = options.maxResponseBytes ?? MAX_RESPONSE_BYTES;
            res.on("data", (d: Buffer) => {
                size += d.length;
                if (size > max) { req.destroy(new Error("Response exceeded size limit")); return; }
                chunks.push(d);
            });
            res.on("aborted", () => finish(reject, new Error("Response aborted")));
            res.on("error", (e) => finish(reject, e));
            res.on("end", () => {
                if (settled) return;
                const raw = Buffer.concat(chunks).toString("utf8");
                const accepted = options.acceptedStatuses || (method === "POST" ? [200, 202] : [200]);
                if (!accepted.includes(res.statusCode || 0)) { finish(reject, new ApiError(res.statusCode || 0, raw)); return; }
                try { finish(resolve, JSON.parse(raw || "{}")); }
                catch (e) { finish(reject, new Error("Invalid response: " + (e instanceof Error ? e.message : String(e)))); }
            });
        });
        options.signal?.addEventListener("abort", abort, { once: true });
        req.on("error", (e) => finish(reject, e));
        req.setTimeout(options.timeoutMs ?? DEFAULT_TIMEOUT_MS, () => req.destroy(new Error("Request timed out")));
        req.end(body);
    });
}

function headers(token: string, session?: string): Record<string, string> {
    return {
        ...(token ? { Authorization: "Bearer " + token } : {}),
        ...(session ? { "X-Gregal-Session": session } : {}),
    };
}

// Client mínim de l'API del Gregal (sense dependre de vscode: testeable
// amb node --test). El token sempre va per capçalera Bearer (E2).
export function apiPost(base: string, token: string, path: string, payload: unknown, options: ApiOptions = {}): Promise<unknown> {
    return requestJson("POST", base, token, path, payload, options);
}

export interface Checkpoint {
    seq: number;
    op: string;
    path: string;
    at: string;
}

export interface SavedConversation {
    name: string;
    title: string;
    msgs: number;
    at: string;
    workspace?: string;
    current?: boolean;
    pinned?: boolean;
}

export interface TranscriptMessage { role: string; content: string }

export interface ResumedConversation {
    name: string;
    title: string;
    workspace?: string;
    transcript: TranscriptMessage[];
}

export function inlineCompletion(
    base: string, token: string, session: string,
    input: { language: string; prefix: string; suffix: string; workspace: string },
    options: Omit<ApiOptions, "session"> = {},
): Promise<{ completion: string }> {
    return apiPost(base, token, "/api/vscode/completion", input, { ...options, session, timeoutMs: options.timeoutMs ?? 12_000 }) as Promise<{ completion: string }>;
}

export function apiGet(base: string, token: string, path: string, options: ApiOptions = {}): Promise<unknown> {
    return requestJson("GET", base, token, path, undefined, options);
}

export function listSavedConversations(base: string, token: string, session: string): Promise<SavedConversation[]> {
    return apiGet(base, token, "/api/sessions", { session }) as Promise<SavedConversation[]>;
}

export function resumeConversation(base: string, token: string, session: string, name: string): Promise<ResumedConversation> {
    return apiPost(base, token, "/api/resume", { name }, { session }) as Promise<ResumedConversation>;
}

export interface Run {
    id: number;
    key?: string;
    session?: string;
    workspace?: string;
    state: "queued" | "running" | "completed" | "failed" | "cancelled" | string;
    error?: string;
    enqueued_at?: string;
    started_at?: string;
    finished_at?: string;
}

export interface DurableEvent {
    id: number;
    at: string;
    session_id?: string;
    run_id?: number;
    kind: string;
    text?: string;
    payload?: unknown;
    data?: unknown;
    details?: unknown;
    interaction?: unknown;
    meta?: unknown;
}

export interface DurableEventsResponse {
    events: DurableEvent[];
    cursor: number;
    next: number;
}

export interface HealthResponse {
    status?: string;
    protocol?: string;
    instance_id?: string;
    capabilities?: Record<string, boolean>;
}

export const API_PROTOCOL = "gregal.v1";

export function apiHealth(base: string, token: string, session: string, options: Omit<ApiOptions, "session"> = {}): Promise<HealthResponse> {
    return apiGet(base, token, "/api/health", { ...options, session }) as Promise<HealthResponse>;
}

export function submitRun(
    base: string,
    token: string,
    session: string,
    task: string,
    idempotencyKey: string,
    mode?: string,
    workspace?: string,
    signal?: AbortSignal,
    timeoutMs?: number,
): Promise<{ run: Run; cursor?: number }> {
    return apiPost(base, token, "/api/v2/runs", {
        task,
        ...(mode ? { mode } : {}),
        ...(workspace ? { workspace } : {}),
        idempotency_key: idempotencyKey,
    }, { session, signal, timeoutMs }) as Promise<{ run: Run; cursor?: number }>;
}

export function listRuns(base: string, token: string, session: string, options: Omit<ApiOptions, "session"> = {}): Promise<{ runs: Run[] }> {
    return apiGet(base, token, "/api/v2/runs", { ...options, session }) as Promise<{ runs: Run[] }>;
}

export function listRunEvents(
    base: string,
    token: string,
    session: string,
    after: number,
    limit = 100,
    options: Omit<ApiOptions, "session"> = {},
): Promise<DurableEventsResponse> {
    const query = new URLSearchParams({ after: String(after), limit: String(limit), session });
    return apiGet(base, token, "/api/v2/events?" + query.toString(), { ...options, session }) as Promise<DurableEventsResponse>;
}

export function cancelRun(base: string, token: string, session: string, id: number, options: Omit<ApiOptions, "session"> = {}): Promise<{ ok: boolean; cancelled: boolean }> {
    return apiPost(base, token, "/api/v2/runs/" + encodeURIComponent(String(id)) + "/cancel", {}, { ...options, session }) as Promise<{ ok: boolean; cancelled: boolean }>;
}

export function getRun(base: string, token: string, session: string, id: number, options: Omit<ApiOptions, "session"> = {}): Promise<{ run: Run }> {
    return apiGet(base, token, "/api/v2/runs/" + encodeURIComponent(String(id)), { ...options, session }) as Promise<{ run: Run }>;
}

export function listCheckpoints(base: string, token: string, session?: string): Promise<Checkpoint[]> {
    return apiGet(base, token, "/api/checkpoints", { session }) as Promise<Checkpoint[]>;
}

export function rewindAll(base: string, token: string, session?: string): Promise<{ summary?: string }> {
    return apiPost(base, token, "/api/rewind", {}, { session }) as Promise<{ summary?: string }>;
}

export function rewindTo(base: string, token: string, seq: number, session?: string): Promise<{ summary?: string }> {
    return apiPost(base, token, "/api/rewind-to", { seq }, { session }) as Promise<{ summary?: string }>;
}
