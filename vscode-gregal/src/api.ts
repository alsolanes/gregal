import * as http from "http";
import * as https from "https";

export interface ApiOptions {
    session?: string;
    acceptedStatuses?: number[];
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

function headers(token: string, session?: string): Record<string, string> {
    return {
        ...(token ? { Authorization: "Bearer " + token } : {}),
        ...(session ? { "X-Gregal-Session": session } : {}),
    };
}

// Client mínim de l'API del Gregal (sense dependre de vscode: testeable
// amb node --test). El token sempre va per capçalera Bearer (E2).
export function apiPost(base: string, token: string, path: string, payload: unknown, options: ApiOptions = {}): Promise<unknown> {
    const body = JSON.stringify(payload);
    const u = new URL(base.replace(/\/$/, "") + path);
    const lib = u.protocol === "https:" ? https : http;
    return new Promise((resolve, reject) => {
        const req = lib.request(
            u,
            {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    "Content-Length": Buffer.byteLength(body),
                    ...headers(token, options.session),
                },
            },
            (res) => {
                let raw = "";
                res.on("data", (d: Buffer) => (raw += d.toString("utf8")));
                res.on("end", () => {
                    const accepted = options.acceptedStatuses || [200, 202];
                    if (!accepted.includes(res.statusCode || 0)) {
                        reject(new ApiError(res.statusCode || 0, raw));
                        return;
                    }
                    try {
                        resolve(JSON.parse(raw || "{}"));
                    } catch (e) {
                        reject(new Error("Invalid response: " + (e instanceof Error ? e.message : String(e))));
                    }
                });
            },
        );
        req.on("error", reject);
        req.setTimeout(60 * 1000, () => req.destroy(new Error("Request timed out (60 s)")));
        req.end(body);
    });
}

export interface Checkpoint {
    seq: number;
    op: string;
    path: string;
    at: string;
}

export function apiGet(base: string, token: string, path: string, options: ApiOptions = {}): Promise<unknown> {
    const u = new URL(base.replace(/\/$/, "") + path);
    const lib = u.protocol === "https:" ? https : http;
    return new Promise((resolve, reject) => {
        const req = lib.request(
            u,
            {
                method: "GET",
                headers: headers(token, options.session),
            },
            (res) => {
                let raw = "";
                res.on("data", (d: Buffer) => (raw += d.toString("utf8")));
                res.on("end", () => {
                    const accepted = options.acceptedStatuses || [200];
                    if (!accepted.includes(res.statusCode || 0)) {
                        reject(new ApiError(res.statusCode || 0, raw));
                        return;
                    }
                    try {
                        resolve(JSON.parse(raw || "{}"));
                    } catch (e) {
                        reject(new Error("Invalid response: " + (e instanceof Error ? e.message : String(e))));
                    }
                });
            },
        );
        req.on("error", reject);
        req.setTimeout(60 * 1000, () => req.destroy(new Error("Request timed out (60 s)")));
        req.end();
    });
}

export interface Run {
    id: number;
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

export function apiHealth(base: string, token: string, session: string): Promise<HealthResponse> {
    return apiGet(base, token, "/api/health", { session }) as Promise<HealthResponse>;
}

export function submitRun(
    base: string,
    token: string,
    session: string,
    task: string,
    idempotencyKey: string,
    mode?: string,
): Promise<{ run: Run }> {
    return apiPost(base, token, "/api/v2/runs", {
        task,
        ...(mode ? { mode } : {}),
        idempotency_key: idempotencyKey,
    }, { session }) as Promise<{ run: Run }>;
}

export function listRunEvents(
    base: string,
    token: string,
    session: string,
    after: number,
    limit = 100,
): Promise<DurableEventsResponse> {
    const query = new URLSearchParams({ after: String(after), limit: String(limit), session });
    return apiGet(base, token, "/api/v2/events?" + query.toString(), { session }) as Promise<DurableEventsResponse>;
}

export function cancelRun(base: string, token: string, session: string, id: number): Promise<{ ok: boolean; cancelled: boolean }> {
    return apiPost(base, token, "/api/v2/runs/" + encodeURIComponent(String(id)) + "/cancel", {}, { session }) as Promise<{ ok: boolean; cancelled: boolean }>;
}

export function getRun(base: string, token: string, session: string, id: number): Promise<{ run: Run }> {
    return apiGet(base, token, "/api/v2/runs/" + encodeURIComponent(String(id)), { session }) as Promise<{ run: Run }>;
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
