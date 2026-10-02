import * as vscode from "vscode";
import * as http from "http";
import * as https from "https";
import { randomUUID } from "crypto";
import { durableInteraction, feedSse, renderDurableEvent, renderEvent } from "./sse";
import {
    ApiError,
    apiPost,
    apiHealth,
    cancelRun,
    listRunEvents,
    listCheckpoints,
    rewindAll,
    rewindTo,
    submitRun,
} from "./api";

function cfg() {
    const c = vscode.workspace.getConfiguration("gregal");
    return {
        base: (c.get<string>("baseUrl") || "http://127.0.0.1:8097").replace(/\/$/, ""),
        token: c.get<string>("token") || "",
        session: validSession(c.get<string>("session") || "vscode"),
    };
}

function validSession(raw: string): string {
    const id = raw.trim();
    return /^[A-Za-z0-9_-]{1,64}$/.test(id) ? id : "vscode";
}

function diagSnip(): string {
    // Diagnòstics (errors/avís) del fitxer actiu, màxim 10 línies.
    try {
        const ed = vscode.window.activeTextEditor;
        const get = (vscode.languages as any)?.getDiagnostics;
        if (!ed || typeof get !== "function") return "";
        const ds = get(ed.document.uri) as Array<{
            message: string;
            range?: { start?: { line?: number } };
            severity?: number;
        }>;
        if (!ds || !ds.length) return "";
        const rows = ds.slice(0, 10).map((d) =>
            vscode.l10n.t("Line {0}: {1}", (d.range?.start?.line ?? 0) + 1, d.message),
        );
        return `\n${vscode.l10n.t("Diagnostics:")}\n${rows.join("\n")}`;
    } catch {
        return "";
    }
}

function contextSnip(): string {
    const ed = vscode.window.activeTextEditor;
    if (!ed) return vscode.l10n.t("(no file open)");
    const sel = ed.selection;
    const lang = ed.document.languageId;
    const path = vscode.workspace.asRelativePath(ed.document.fileName);
    if (!sel.isEmpty) {
        return vscode.l10n.t("File {0} ({1}), selection:\n```{1}\n{2}\n```", path, lang, ed.document.getText(sel)) + diagSnip();
    }
    return vscode.l10n.t("File {0} ({1}) in full:\n```{1}\n{2}\n```", path, lang, ed.document.getText()) + diagSnip();
}

let activeRun: { base: string; token: string; session: string; id: number } | null = null;

const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

async function postRunV2(task: string, out: vscode.OutputChannel): Promise<boolean> {
    const { base, token, session } = cfg();
    let health;
    try {
        health = await apiHealth(base, token, session);
    } catch (e) {
        // Un servidor anterior pot no conèixer /api/health. En aquest cas la
        // ruta legacy és l'única opció segura.
        if (e instanceof ApiError && ![404, 405, 501].includes(e.status)) throw e;
        return false;
    }
    const caps = health.capabilities || {};
    if (!caps.runs || !caps.durable_events || !caps.interactive_events) return false;
    let submitted;
    try {
        submitted = await submitRun(base, token, session, task, randomUUID());
    } catch (e) {
        if (e instanceof ApiError && [404, 405, 501].includes(e.status)) return false;
        throw e;
    }
    if (!submitted.run || !submitted.run.id) throw new Error(vscode.l10n.t("Incompatible v2 response: missing run ID."));
    const id = submitted.run.id;
    activeRun = { base, token, session, id };
    let cursor = 0;
    let terminal = false;
    try {
        while (!terminal) {
            const page = await listRunEvents(base, token, session, cursor, 100);
            const events = Array.isArray(page.events) ? page.events : [];
            cursor = Number.isFinite(page.next) ? page.next : cursor;
            for (const ev of events) {
                if (ev.run_id !== id) continue;
                if (ev.kind === "approve_request" || ev.kind === "question_request") {
                    const payload = durableInteraction(ev);
                    if (!payload || typeof payload.key !== "string") {
                        await cancelRun(base, token, session, id).catch(() => {});
                        throw new Error(vscode.l10n.t("Interactive v2 event has no structured interaction payload."));
                    }
                    if (ev.kind === "approve_request") {
                        const name = String(payload.name || vscode.l10n.t("tool"));
                        const args = String(payload.args || "");
                        out.appendLine(vscode.l10n.t("⏳ Permission requested: {0} {1}", name, args).slice(0, 300));
                        await askApprove(String(payload.key), name, args);
                    } else {
                        await askQuestion(String(payload.key), String(payload.query || vscode.l10n.t("Which option do you prefer?")), payload.options);
                    }
                    continue;
                }
                const line = renderDurableEvent(ev, (message, ...args) => vscode.l10n.t(message, ...args));
                if (line) out.append(line);
                if (ev.kind === "run_completed" || ev.kind === "run_failed" || ev.kind === "run_cancelled") {
                    terminal = true;
                }
            }
            if (!terminal) await sleep(events.length ? 80 : 250);
        }
    } finally {
        if (activeRun?.id === id) activeRun = null;
    }
    return true;
}

async function cancelCmd(): Promise<void> {
    const run = activeRun;
    if (!run) {
        void vscode.window.showInformationMessage(vscode.l10n.t("Gregal: no active run."));
        return;
    }
    try {
        await cancelRun(run.base, run.token, run.session, run.id);
        void vscode.window.showInformationMessage(vscode.l10n.t("Gregal: run #{0} cancelled.", run.id));
    } catch (e) {
        void vscode.window.showErrorMessage(vscode.l10n.t("Gregal: could not cancel the run: {0}", e instanceof Error ? e.message : String(e)));
    }
}

function postAgent(task: string, out: vscode.OutputChannel): Promise<void> {
    const { base, token, session } = cfg();
    const body = JSON.stringify({ task });
    const u = new URL(base + "/api/agent");
    const lib = u.protocol === "https:" ? https : http;
    return new Promise((resolve, reject) => {
        const req = lib.request(
            u,
            {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    "Content-Length": Buffer.byteLength(body),
                    ...(token ? { Authorization: "Bearer " + token } : {}),
                    "X-Gregal-Session": session,
                },
            },
            (res) => {
                if (res.statusCode !== 200) {
                    reject(new Error("Server: HTTP " + res.statusCode));
                    res.resume();
                    return;
                }
                let carry = "";
                let pendingApprove: { key: string; name: string; args: string } | null = null;
                res.on("data", (d: Buffer) => {
                    const { evs, rest } = feedSse(d.toString("utf8"), carry);
                    carry = rest;
                    for (const ev of evs) {
                        if (ev.name === "approve_request") {
                            try {
                                const o = JSON.parse(ev.data);
                                pendingApprove = { key: o.key, name: o.name, args: o.args };
                                out.appendLine(vscode.l10n.t("⏳ Permission requested: {0} {1}", o.name, o.args).slice(0, 300));
                                void askApprove(o.key, o.name, o.args).catch(() => {});
                            } catch {
                                /* ignora */
                            }
                            continue;
                        }
                        if (ev.name === "question_request") {
                            try {
                                const o = JSON.parse(ev.data);
                                void askQuestion(
                                    String(o.key || o.question_key),
                                    String(o.query || o.question || vscode.l10n.t("Which option do you prefer?")),
                                    o.options || o.choices,
                                ).catch(() => {});
                            } catch {
                                /* ignora */
                            }
                            continue;
                        }
                        const line = renderEvent(ev, (message, ...args) => vscode.l10n.t(message, ...args));
                        if (line) out.append(line);
                    }
                });
                res.on("end", () => {
                    out.appendLine("");
                    out.show(true);
                    void pendingApprove;
                    resolve();
                });
                res.on("error", reject);
            },
        );
        req.on("error", reject);
        req.setTimeout(10 * 60 * 1000, () => req.destroy(new Error("Request timed out (10 min)")));
        req.end(body);
    });
}

async function askApprove(key: string, name: string, args: string): Promise<void> {
    const pick = await vscode.window.showWarningMessage(
        vscode.l10n.t("Gregal requests permission to use {0}", name),
        { detail: args.slice(0, 400), modal: true },
        vscode.l10n.t("Allow"),
        vscode.l10n.t("Deny"),
    );
    const { base, token, session } = cfg();
    const body = JSON.stringify({ key, approve: pick === vscode.l10n.t("Allow") });
    const u = new URL(base + "/api/approve");
    const lib = u.protocol === "https:" ? https : http;
    await new Promise<void>((resolve) => {
        const req = lib.request(
            u,
            {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    "Content-Length": Buffer.byteLength(body),
                    ...(token ? { Authorization: "Bearer " + token } : {}),
                    "X-Gregal-Session": session,
                },
            },
            (res) => {
                res.resume();
                res.on("end", () => resolve());
            },
        );
        req.on("error", () => resolve());
        req.end(body);
    });
}

async function askQuestion(key: string, query: string, options: unknown): Promise<void> {
    type QuestionItem = vscode.QuickPickItem & { free?: boolean };
    const items: QuestionItem[] = [];
    if (Array.isArray(options)) {
        for (const option of options) {
            if (typeof option === "string" || typeof option === "number") {
                items.push({ label: String(option) });
                continue;
            }
            if (!option || typeof option !== "object") continue;
            const value = option as Record<string, unknown>;
            const label = String(value.label || value.text || value.value || "").trim();
            if (label) items.push({ label, description: String(value.description || "") });
        }
    }
    items.push({ label: vscode.l10n.t("Type an answer…"), description: vscode.l10n.t("Free-text answer"), free: true });
    const pick = await vscode.window.showQuickPick(items, { placeHolder: query });
    let answer = "";
    if (pick?.free) {
        answer = (await vscode.window.showInputBox({ prompt: query, placeHolder: vscode.l10n.t("Answer") })) || "";
        answer = "text:" + answer;
    } else if (pick) {
        answer = pick.label;
    }
    const { base, token, session } = cfg();
    await apiPost(base, token, "/api/question", { key, answer }, { session });
}

async function run(question: string | null): Promise<void> {
    const q =
        question ??
        (await vscode.window.showInputBox({
            prompt: vscode.l10n.t("Ask Gregal about the current selection or file"),
            placeHolder: vscode.l10n.t("e.g. Why does this test fail?"),
        }));
    if (!q || !q.trim()) return;
    const out = vscode.window.createOutputChannel("Gregal");
    out.clear();
    out.appendLine("≋ " + q.trim());
    out.appendLine("");
    out.show(true);
        try {
            const task = contextSnip() + "\n\n" + vscode.l10n.t("Question: {0}", q.trim());
            // postRunV2 només torna false abans d'encuar (health o ruta de
            // submit absent); un error posterior no s'ha de repetir en legacy.
            const usedV2 = await postRunV2(task, out);
            if (!usedV2) {
                out.appendLine(vscode.l10n.t("ℹ️ Interactive v2 is unavailable; using /api/agent."));
                await postAgent(task, out);
            }
    } catch (e) {
        out.appendLine(vscode.l10n.t("⚠️ {0}", e instanceof Error ? e.message : String(e)));
        out.appendLine(vscode.l10n.t("Check the Gregal server settings: gregal.baseUrl and gregal.token."));
        out.show(true);
    }
}

async function rewindCmd(): Promise<void> {
    // Paritat amb el botó Desfés del web: tria checkpoint o desfés-ho tot.
    // Restaura el contingut d'abans de la sessió (no usa git).
    const { base, token, session } = cfg();
    let cps;
    try {
        cps = await listCheckpoints(base, token, session);
    } catch (e) {
        void vscode.window.showErrorMessage(vscode.l10n.t("Gregal: could not read checkpoints: {0}", e instanceof Error ? e.message : String(e)));
        return;
    }
    if (!cps.length) {
        void vscode.window.showInformationMessage(vscode.l10n.t("Gregal: nothing to undo (no files changed this session)."));
        return;
    }
    const items = cps.map((c) => ({
        label: `#${c.seq} ${c.op} ${c.path}`,
        description: c.at,
        seq: c.seq,
    }));
    const countDescription = cps.length === 1
        ? vscode.l10n.t("1 change")
        : vscode.l10n.t("{0} changes", cps.length);
    const pick = await vscode.window.showQuickPick(
        [{ label: vscode.l10n.t("↩ Undo all changes"), description: countDescription, seq: 0 }, ...items],
        { placeHolder: vscode.l10n.t("Return to a checkpoint (restores files; does not use Git)") },
    );
    if (!pick) return;
    const ok = await vscode.window.showWarningMessage(
        pick.seq === 0
            ? cps.length === 1
                ? vscode.l10n.t("Undo this session change?")
                : vscode.l10n.t("Undo all {0} session changes?", cps.length)
            : vscode.l10n.t("Return to checkpoint #{0}?", pick.seq),
        { modal: true },
        vscode.l10n.t("Undo"),
    );
    if (ok !== vscode.l10n.t("Undo")) return;
    try {
        const r = pick.seq === 0 ? await rewindAll(base, token, session) : await rewindTo(base, token, pick.seq, session);
        void vscode.window.showInformationMessage("Gregal: " + (r.summary || vscode.l10n.t("changes undone")));
    } catch (e) {
        void vscode.window.showErrorMessage(vscode.l10n.t("Gregal: could not undo: {0}", e instanceof Error ? e.message : String(e)));
    }
}

async function githubCmd(): Promise<void> {
    const kind = await vscode.window.showQuickPick(
        [{ label: vscode.l10n.t("Issue"), tool: "gh_issue" }, { label: vscode.l10n.t("Pull request"), tool: "gh_pr" }],
        { placeHolder: vscode.l10n.t("What would you like to view on GitHub?") },
    );
    if (!kind) return;
    const actions = kind.tool === "gh_issue"
        ? [{ label: vscode.l10n.t("List"), action: "list" }, { label: vscode.l10n.t("View issue"), action: "view" }]
        : [{ label: vscode.l10n.t("List"), action: "list" }, { label: vscode.l10n.t("View pull request"), action: "view" }, { label: vscode.l10n.t("Diff"), action: "diff" }, { label: vscode.l10n.t("Checks"), action: "checks" }];
    const selected = await vscode.window.showQuickPick(actions, { placeHolder: vscode.l10n.t("Read-only action") });
    if (!selected) return;
    const number = selected.action === "list" ? "" : await vscode.window.showInputBox({ prompt: vscode.l10n.t("Number (leave empty for the current repository)") });
    if (selected.action !== "list" && number && !/^\d+$/.test(number.trim())) {
        void vscode.window.showErrorMessage(vscode.l10n.t("Gregal: the number must be an integer."));
        return;
    }
    const repo = await vscode.window.showInputBox({ prompt: vscode.l10n.t("owner/repo (leave empty for the current repository)"), placeHolder: "solanes/gregal" });
    const query = selected.action === "list" ? await vscode.window.showInputBox({ prompt: vscode.l10n.t("Optional search"), placeHolder: "is:open" }) : "";
    const { base, token, session } = cfg();
    const out = vscode.window.createOutputChannel("Gregal GitHub");
    out.clear(); out.appendLine(`≋ ${kind.label} · ${selected.label}`); out.show(true);
    try {
        const data = await apiPost(base, token, "/api/github", {
            tool: kind.tool, action: selected.action, number: number ? Number(number) : 0,
            repo: repo || "", query: query || "",
        }, { session }) as { output?: string };
        out.appendLine(data.output || vscode.l10n.t("(no results)"));
    } catch (e) {
        void vscode.window.showErrorMessage(vscode.l10n.t("Gregal GitHub: {0}", e instanceof Error ? e.message : String(e)));
    }
}

export function activate(ctx: vscode.ExtensionContext): void {
    ctx.subscriptions.push(
        vscode.commands.registerCommand("gregal.ask", () => run(null)),
        vscode.commands.registerCommand("gregal.cancel", cancelCmd),
        vscode.commands.registerCommand("gregal.rewind", rewindCmd),
        vscode.commands.registerCommand("gregal.github", githubCmd),
        vscode.commands.registerCommand("gregal.explain", () =>
            run(vscode.l10n.t("Explain what this code does, briefly.")),
        ),
    );
}

export function deactivate(): void {}
