// Parser SSE minimal (mateix protocol que /api/agent): "event: X\ndata: {...}\n\n".
// Pur i testeable amb node --test (sense vscode).
export interface SseEvent {
    name: string;
    data: string;
}

export interface DurableEventLike {
    kind: string;
    text?: string;
    payload?: unknown;
    data?: unknown;
    details?: unknown;
    interaction?: unknown;
    meta?: unknown;
    key?: string;
    approval_key?: string;
    question_key?: string;
}

export type Localize = (message: string, ...args: Array<string | number | boolean>) => string;

const english: Localize = (message, ...args) =>
    message.replace(/\{(\d+)\}/g, (match, index: string) => String(args[Number(index)] ?? match));

/** Extreu una aprovació/pregunta estructurada d'un event durable. */
export function durableInteraction(ev: DurableEventLike): Record<string, unknown> | null {
    const candidates: unknown[] = [ev.payload, ev.data, ev.details, ev.interaction, ev.meta];
    for (const candidate of candidates) {
        let value: unknown = candidate;
        if (typeof value === "string") {
            try { value = JSON.parse(value); } catch { continue; }
        }
        if (!value || typeof value !== "object" || Array.isArray(value)) continue;
        const obj = value as Record<string, unknown>;
        const nested = obj.payload || obj.interaction || obj.data;
        if (nested && typeof nested === "object" && !Array.isArray(nested)) value = nested;
        const out = value as Record<string, unknown>;
        const key = out.key || out.approval_key || out.question_key;
        if (typeof key === "string" && key.trim()) {
            const normalized: Record<string, unknown> = { ...out, key };
            if (!normalized.options && normalized.choices) normalized.options = normalized.choices;
            if (!normalized.query && normalized.question) normalized.query = normalized.question;
            return normalized;
        }
    }
    const direct = ev.key || ev.approval_key || ev.question_key;
    if (direct) return { ...ev, key: direct };
    if (typeof ev.text === "string") {
        try {
            const parsed = JSON.parse(ev.text) as Record<string, unknown>;
            if (parsed && typeof parsed === "object") return durableInteraction(parsed as unknown as DurableEventLike);
        } catch { /* text normal: no payload */ }
    }
    return null;
}

export function feedSse(chunk: string, carry: string): { evs: SseEvent[]; rest: string } {
    const buf = carry + chunk;
    const evs: SseEvent[] = [];
    let start = 0;
    for (;;) {
        const end = buf.indexOf("\n\n", start);
        if (end < 0) break;
        const block = buf.slice(start, end);
        start = end + 2;
        let name = "";
        const datas: string[] = [];
        for (const line of block.split("\n")) {
            if (line.startsWith("event:")) name = line.slice(6).trim();
            else if (line.startsWith("data:")) datas.push(line.slice(5).trim());
        }
        if (name) evs.push({ name, data: datas.join("\n") });
    }
    return { evs, rest: buf.slice(start) };
}

/** Text humà d'un event d'agent per al canal de sortida. null = silenciós. */
export function renderEvent(ev: SseEvent, localize: Localize = english): string | null {
    let o: Record<string, string> = {};
    try {
        o = JSON.parse(ev.data || "{}");
    } catch {
        return null;
    }
    switch (ev.name) {
        case "token":
        case "assistant":
            return o.text || null;
        case "thinking":
        case "thought":
        case "reasoning":
            return "💭 " + (o.text || o.content || "");
        case "tool_call":
            return "🔧 " + o.name + " " + (o.args || "").slice(0, 120);
        case "tool_result":
            return "↳ " + o.name + ": " + (o.output || "").slice(0, 300);
        case "verify":
            return "✅ " + o.verdict + "\n" + (o.detail || "").slice(0, 500);
        case "fallback":
            return "↪ " + localize("Primary provider failed; falling back to {0}.", o.model || "unknown");
        case "error":
            return "⚠️ " + (o.message || localize("Error"));
        case "done":
            return o.reply ? "\n" + o.reply : null;
        default:
            return null;
    }
}

/** Converteix un event persistent de /api/v2/events a text d'OutputChannel. */
export function renderDurableEvent(ev: DurableEventLike, localize: Localize = english): string | null {
    const text = ev.text || "";
    switch (ev.kind) {
        case "text":
            return text || null;
        case "done":
            return text ? "\n" + text : null;
        case "error":
            return text ? "⚠️ " + text : "⚠️ " + localize("Error");
        case "thinking":
            return text ? "💭 " + text : null;
        case "tool_call":
            return text ? "🔧 " + text : null;
        case "tool_result":
            return text ? "↳ " + text : null;
        case "verify":
            return text ? "✅ " + text : null;
        case "run_queued":
        case "run_started":
        case "run_completed":
        case "run_failed":
        case "run_cancelled":
            return null;
        default:
            return text || null;
    }
}
