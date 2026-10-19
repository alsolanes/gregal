import * as vscode from "vscode";
import { apiGet, apiHealth, listSavedConversations, resumeConversation, type SavedConversation } from "./api";
import { runAgent, type RunConnection, type RunRequest } from "./run-client";
import { ChatSidebar, type ChatAction } from "./sidebar";
import { editorAttachment, projectSession, MAX_CONTEXT_CHARS, type EditorAttachment } from "./editor-context";
import { registerEditorFeatures } from "./editor-features";

let extension: vscode.ExtensionContext;
let chat: ChatSidebar;
let output: vscode.OutputChannel;
let status: vscode.StatusBarItem;
let attached: EditorAttachment | undefined;
let conversationFolder: vscode.WorkspaceFolder | undefined;
let displayedRoute = "";
let active: { abort: AbortController; connection?: RunConnection; finished: Promise<void> } | undefined;
let exclusiveAction = false;
const sessions = new Map<string, string>();

function workspaceForEditor(): vscode.WorkspaceFolder | undefined {
    const uri = vscode.window.activeTextEditor?.document.uri;
    return (uri && vscode.workspace.getWorkspaceFolder(uri)) ||
        (vscode.workspace.workspaceFolders?.length === 1 ? vscode.workspace.workspaceFolders[0] : undefined);
}

export async function connectionConfig(folder = conversationFolder || workspaceForEditor()): Promise<RunConnection> {
    const config = vscode.workspace.getConfiguration("gregal", folder?.uri);
    const base = (config.get<string>("baseUrl") || "http://127.0.0.1:8097").trim().replace(/\/+$/, "");
    const url = new URL(base);
    if (!["http:", "https:"].includes(url.protocol) || url.username || url.password || url.hash || url.search) {
        throw new Error(vscode.l10n.t("Use an HTTP or HTTPS server URL without credentials, a query, or a fragment."));
    }
    const token = await extension.secrets.get("gregal.token:" + base) || config.get<string>("token") || "";
    const workspace = folder?.uri.toString() || "";
    return { base, token, session: sessions.get(workspace) || projectSession(config.get<string>("session") || "vscode", workspace) };
}

async function showChat(): Promise<void> { await vscode.commands.executeCommand("gregal.chat.focus"); }

function setBusy(busy: boolean): void {
    chat.update({ busy });
    status.text = busy ? "$(loading~spin) Gregal" : "$(sparkle) Gregal";
    status.tooltip = busy ? vscode.l10n.t("Gregal is working. Open the conversation to follow progress.") : vscode.l10n.t("Open Gregal");
    void vscode.commands.executeCommand("setContext", "gregal.busy", busy);
}

async function refreshConnection(): Promise<void> {
    if (active) return;
    try {
        const connection = await connectionConfig();
        await apiHealth(connection.base, connection.token, connection.session);
        const state = await apiGet(connection.base, connection.token, "/api/state", { session: connection.session }) as { model?: string };
        if (!active) chat.update({ connected: true, model: typeof state.model === "string" ? state.model : "", project: (conversationFolder || workspaceForEditor())?.name || "", error: "" });
    } catch (error) {
        if (!active) chat.update({ connected: false, model: "", error: vscode.l10n.t("Start gregal --serve or check your connection settings.") });
    }
}

function attachContext(): void {
    const editor = vscode.window.activeTextEditor;
    if (!editor) { void vscode.window.showInformationMessage(vscode.l10n.t("Open a file to attach code.")); return; }
    const selection = editor.selection;
    const sourceText = editor.document.getText(selection.isEmpty ? undefined : selection);
    const folder = vscode.workspace.getWorkspaceFolder(editor.document.uri);
    const diagnostics = vscode.languages.getDiagnostics(editor.document.uri).slice(0, 10)
        .map(d => vscode.l10n.t("Line {0}: {1}", d.range.start.line + 1, d.message));
    attached = editorAttachment({
        path: vscode.workspace.asRelativePath(editor.document.uri, true), language: editor.document.languageId,
        text: sourceText,
        ...(selection.isEmpty ? {} : { startLine: selection.start.line + 1,
            endLine: selection.end.line + (selection.end.character === 0 && selection.end.line > selection.start.line ? 0 : 1) }),
        diagnostics, workspace: folder?.uri.fsPath,
    });
    attached.detail = editor.document.languageId + " · " + vscode.l10n.t("{0} characters", Math.min(sourceText.length, MAX_CONTEXT_CHARS));
    chat.update({ context: { label: attached.label, detail: attached.detail } });
}

async function handleDiagnostic(document: vscode.TextDocument, diagnostic: vscode.Diagnostic, intent: "fix" | "explain"): Promise<void> {
    if (!vscode.workspace.isTrusted) { void vscode.window.showErrorMessage(vscode.l10n.t("Trust this workspace before running Gregal.")); return; }
    const folder = vscode.workspace.getWorkspaceFolder(document.uri);
    if (!folder) { void vscode.window.showErrorMessage(vscode.l10n.t("Open this file inside a workspace to use Gregal diagnostics.")); return; }
    const startLine = Math.max(0, diagnostic.range.start.line - 2);
    const endLine = Math.min(document.lineCount - 1, diagnostic.range.end.line + 2);
    const range = new vscode.Range(startLine, 0, endLine, document.lineAt(endLine).text.length);
    const code = document.getText(range);
    const severity = diagnostic.severity === vscode.DiagnosticSeverity.Error ? vscode.l10n.t("Error") :
        diagnostic.severity === vscode.DiagnosticSeverity.Warning ? vscode.l10n.t("Warning") : vscode.l10n.t("Information");
    const diagnosticText = vscode.l10n.t("{0}: {1}", severity, diagnostic.message);
    const path = vscode.workspace.asRelativePath(document.uri, true);
    attached = editorAttachment({
        path, language: document.languageId, text: code, startLine: startLine + 1, endLine: endLine + 1,
        diagnostics: [vscode.l10n.t("Line {0}: {1}", diagnostic.range.start.line + 1, diagnosticText)],
        workspace: folder.uri.fsPath,
    });
    attached.detail = document.languageId + " · " + vscode.l10n.t("Lines {0}–{1}", startLine + 1, endLine + 1);
    conversationFolder = folder;
    chat.update({ context: { label: attached.label, detail: attached.detail }, project: folder.name });
    await showChat();
    const prompt = intent === "fix"
        ? vscode.l10n.t("Fix this diagnostic with the smallest safe change, explain the cause, and run a focused check if available: {0}", diagnosticText)
        : vscode.l10n.t("Explain this diagnostic and the relevant code. Suggest a fix, but do not edit files: {0}", diagnosticText);
    await send(prompt, intent === "fix" ? "code" : "inspect", true);
}

async function chooseWorkspace(attachment?: EditorAttachment): Promise<vscode.WorkspaceFolder | undefined> {
    if (attachment?.workspace) {
        return vscode.workspace.workspaceFolders?.find(folder => folder.uri.fsPath === attachment.workspace);
    }
    const current = conversationFolder || workspaceForEditor();
    if (current) return current;
    const folders = vscode.workspace.workspaceFolders || [];
    if (folders.length > 1) return vscode.window.showWorkspaceFolderPick({ placeHolder: vscode.l10n.t("Choose the project for this conversation") });
    return undefined;
}

async function approval(payload: Record<string, unknown>, signal: AbortSignal): Promise<boolean> {
    const source = new vscode.CancellationTokenSource();
    const abort = () => source.cancel();
    signal.addEventListener("abort", abort, { once: true });
    if (signal.aborted) source.cancel();
    try {
        const pick = await vscode.window.showQuickPick([
            { label: vscode.l10n.t("Allow"), detail: String(payload.args || "").slice(0, 8000), allow: true },
            { label: vscode.l10n.t("Deny"), allow: false },
        ], { title: vscode.l10n.t("Gregal requests permission to use {0}", String(payload.name || "tool")),
            placeHolder: vscode.l10n.t("Review the tool arguments before allowing this action"), ignoreFocusOut: true }, source.token);
        return pick?.allow === true;
    } finally { signal.removeEventListener("abort", abort); source.dispose(); }
}

async function question(payload: Record<string, unknown>, signal: AbortSignal): Promise<string> {
    const source = new vscode.CancellationTokenSource();
    const abort = () => source.cancel();
    signal.addEventListener("abort", abort, { once: true });
    if (signal.aborted) source.cancel();
    const query = String(payload.query || vscode.l10n.t("Which option do you prefer?"));
    try {
        const options = Array.isArray(payload.options) ? payload.options : [];
        const items = options.map(option => {
            const value = typeof option === "object" && option ? option as Record<string, unknown> : {};
            return { label: typeof option === "string" ? option : String(value.label || value.text || ""), description: String(value.description || ""), free: false };
        }).filter(item => item.label);
        items.push({ label: vscode.l10n.t("Type an answer…"), description: vscode.l10n.t("Free-text answer"), free: true });
        const pick = await vscode.window.showQuickPick(items, { title: query, ignoreFocusOut: true }, source.token);
        if (!pick) return "";
        if (!pick.free) return pick.label;
        const answer = await vscode.window.showInputBox({ prompt: query, ignoreFocusOut: true }, source.token);
        return answer === undefined ? "" : "text:" + answer;
    } finally { signal.removeEventListener("abort", abort); source.dispose(); }
}

async function send(text: string, mode: RunRequest["mode"], includeContext: boolean): Promise<void> {
    text = text.trim();
    if (!text || text.length > 20_000) return;
    if (active || exclusiveAction) { void vscode.window.showInformationMessage(vscode.l10n.t("Finish the current operation before starting another.")); return; }
    if (!vscode.workspace.isTrusted) { void vscode.window.showErrorMessage(vscode.l10n.t("Trust this workspace before running Gregal.")); return; }
    const attachment = includeContext ? attached : undefined;
    let finish: () => void = () => {};
    const finished = new Promise<void>(resolve => { finish = resolve; });
    const turn = { abort: new AbortController(), connection: undefined as RunConnection | undefined, finished };
    let reportedError = "";
    let observedEvent = false;
    active = turn;
    setBusy(true);
    chat.update({ error: "" });
    try {
        const folder = await chooseWorkspace(attachment);
        if (!folder && mode !== "chat") throw new Error(vscode.l10n.t("Open or choose a workspace before using Code or Inspect mode."));
        turn.connection = await connectionConfig(folder);
        if (turn.abort.signal.aborted) return;
        conversationFolder = folder;
        const route = turn.connection.base + "/" + turn.connection.session;
        if (displayedRoute && displayedRoute !== route) chat.reset();
        displayedRoute = route;
        chat.update({ project: folder?.name || "" });
        chat.message("user", text);
        output.appendLine("\n≋ " + text);
        await runAgent(turn.connection, { task: attachment ? attachment.text + "\n\nRequest:\n" + text : text, mode, workspace: folder?.uri.fsPath }, {
            onEvent(kind, data) {
                if (!observedEvent) { observedEvent = true; chat.update({ connected: true }); }
                chat.event(kind, data);
                if (kind === "error") reportedError = String(data.message || data.text || "");
                if (kind === "fallback" && typeof data.model === "string") chat.update({ model: data.model });
                if (kind === "token" || kind === "text") output.append(String(data.text || ""));
                else if (kind === "tool_call" || kind === "error") output.appendLine(String(data.name || data.message || data.text || kind));
            },
            onApproval: payload => approval(payload, turn.abort.signal),
            onQuestion: payload => question(payload, turn.abort.signal),
        }, turn.abort.signal);
    } catch (error) {
        const message = turn.abort.signal.aborted ? vscode.l10n.t("Run stopped.") : error instanceof Error ? error.message : String(error);
        if (turn.abort.signal.aborted) chat.event("cancelled", { message });
        else { if (reportedError !== message) chat.event("error", { message }); chat.update({ error: message }); }
        output.appendLine(message);
    } finally {
        if (active === turn) { active = undefined; setBusy(false); }
        finish();
    }
}

async function saveToken(): Promise<void> {
    const connection = await connectionConfig();
    const value = await vscode.window.showInputBox({ title: vscode.l10n.t("Gregal server token"), password: true,
        prompt: vscode.l10n.t("Stored securely in VS Code. Leave empty to remove the saved token."), ignoreFocusOut: true });
    if (value === undefined) return;
    const key = "gregal.token:" + connection.base;
    if (value.trim()) await extension.secrets.store(key, value.trim()); else await extension.secrets.delete(key);
    await refreshConnection();
}

async function newChat(): Promise<void> {
    if (active || exclusiveAction) { void vscode.window.showInformationMessage(vscode.l10n.t("Finish the current operation before starting another.")); return; }
    const folder = await chooseWorkspace();
    const workspace = folder?.uri.toString() || "";
    sessions.set(workspace, "vscode-" + Date.now().toString(36) + "-" + Math.random().toString(36).slice(2, 8));
    conversationFolder = folder;
    displayedRoute = "";
    attached = undefined;
    chat.reset(); chat.update({ context: null, error: "" });
    await refreshConnection();
}

async function openSavedConversation(): Promise<void> {
    if (active || exclusiveAction) { void vscode.window.showInformationMessage(vscode.l10n.t("Finish the current operation before switching conversations.")); return; }
    exclusiveAction = true;
    try {
        const currentFolder = conversationFolder || workspaceForEditor();
        const current = await connectionConfig(currentFolder);
        const saved = await listSavedConversations(current.base, current.token, current.session);
        if (!saved.length) { void vscode.window.showInformationMessage(vscode.l10n.t("No saved conversations yet.")); return; }
        const items = saved.map((item: SavedConversation) => ({
            label: item.pinned ? "$(pinned) " + (item.title || item.name) : (item.title || item.name),
            description: [item.at, item.msgs ? vscode.l10n.t("{0} messages", item.msgs) : ""].filter(Boolean).join(" · "),
            detail: item.workspace || vscode.l10n.t("No project attached"),
            item,
        }));
        const selected = await vscode.window.showQuickPick(items, { placeHolder: vscode.l10n.t("Choose a saved Gregal conversation"), matchOnDetail: true });
        if (!selected) return;
        let folder = currentFolder;
        if (selected.item.workspace) {
            const targetUri = vscode.Uri.file(selected.item.workspace);
            folder = vscode.workspace.getWorkspaceFolder(targetUri);
            if (!folder || folder.uri.fsPath.toLowerCase() !== targetUri.fsPath.toLowerCase()) {
                void vscode.window.showErrorMessage(vscode.l10n.t("Open the project for this conversation in VS Code, then try again."));
                return;
            }
        }
        const connection = await connectionConfig(folder);
        const restored = await resumeConversation(connection.base, connection.token, connection.session, selected.item.name);
        conversationFolder = folder;
        displayedRoute = connection.base + "/" + connection.session;
        attached = undefined;
        chat.restoreTranscript(Array.isArray(restored.transcript) ? restored.transcript : []);
        chat.update({ context: null, error: "", project: folder?.name || "" });
        void vscode.window.showInformationMessage(vscode.l10n.t("Conversation restored: {0}", restored.title || selected.item.title || selected.item.name));
    } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        chat.update({ error: message });
        void vscode.window.showErrorMessage(vscode.l10n.t("Could not restore conversation: {0}", message));
    } finally { exclusiveAction = false; }
}

function action(value: ChatAction): void {
    const perform = async () => {
        switch (value.type) {
            case "ready": await refreshConnection(); break;
            case "send": await send(value.text || "", ["code", "chat", "inspect"].includes(value.mode || "") ? value.mode as RunRequest["mode"] : "code", value.includeContext === true); break;
            case "cancel": active?.abort.abort(); break;
            case "attachContext": attachContext(); break;
            case "clearContext": attached = undefined; chat.update({ context: null }); break;
            case "settings": await vscode.commands.executeCommand("workbench.action.openSettings", "gregal"); break;
            case "review": await vscode.commands.executeCommand("workbench.view.scm"); break;
            case "newChat": await newChat(); break;
            case "savedConversations": await openSavedConversation(); break;
            case "copy": await vscode.env.clipboard.writeText(value.text || ""); break;
        }
    };
    void perform().catch(error => { chat.update({ error: String(error instanceof Error ? error.message : error) }); });
}

export function activateChat(ctx: vscode.ExtensionContext): void {
    extension = ctx;
    output = vscode.window.createOutputChannel("Gregal");
    status = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 80);
    status.command = "gregal.openChat";
    chat = new ChatSidebar(ctx.extensionUri, action);
    setBusy(false); status.show();
    ctx.subscriptions.push(output, status,
        vscode.window.registerWebviewViewProvider("gregal.chat", chat, { webviewOptions: { retainContextWhenHidden: true } }),
        vscode.commands.registerCommand("gregal.openChat", showChat),
        vscode.commands.registerCommand("gregal.ask", async () => { attachContext(); await showChat(); }),
        vscode.commands.registerCommand("gregal.explain", async () => { attachContext(); await showChat(); await send(vscode.l10n.t("Explain what this code does, briefly."), "inspect", true); }),
        vscode.commands.registerCommand("gregal.cancel", () => active?.abort.abort()),
        vscode.commands.registerCommand("gregal.newChat", newChat),
        vscode.commands.registerCommand("gregal.setToken", saveToken),
        vscode.commands.registerCommand("gregal.reviewChanges", () => vscode.commands.executeCommand("workbench.view.scm")),
        vscode.workspace.onDidChangeConfiguration(event => { if (event.affectsConfiguration("gregal")) void refreshConnection(); }),
    );
    registerEditorFeatures(ctx, folder => connectionConfig(folder), handleDiagnostic);
}

export async function deactivateChat(): Promise<void> {
    const turn = active;
    if (!turn) return;
    turn.abort.abort();
    // Give the exact-run cancellation request time to reach the server.
    let timeout: ReturnType<typeof setTimeout> | undefined;
    try { await Promise.race([turn.finished, new Promise<void>(resolve => { timeout = setTimeout(resolve, 3000); })]); }
    finally { if (timeout) clearTimeout(timeout); }
}

export async function withIdleConversation(operation: () => Promise<void>): Promise<void> {
    if (active || exclusiveAction) {
        void vscode.window.showInformationMessage(vscode.l10n.t("Finish the current operation before undoing changes."));
        return;
    }
    exclusiveAction = true;
    try { await operation(); } finally { exclusiveAction = false; }
}
