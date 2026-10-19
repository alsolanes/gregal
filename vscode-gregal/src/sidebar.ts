import * as vscode from "vscode";
import { randomBytes } from "crypto";

export interface ChatState {
    busy: boolean;
    connected: boolean;
    model?: string;
    project?: string;
    context?: { label: string; detail?: string } | null;
    error?: string;
}

export type ChatAction = { type: string; text?: string; mode?: string; includeContext?: boolean };

export class ChatSidebar implements vscode.WebviewViewProvider {
    private view?: vscode.WebviewView;
    private state: ChatState = { busy: false, connected: false, context: null };
    private history: Array<Record<string, unknown>> = [];
    private ready = false;

    constructor(private readonly extensionUri: vscode.Uri, private readonly onAction: (action: ChatAction) => void) {}

    resolveWebviewView(view: vscode.WebviewView): void {
        this.view = view;
        this.ready = false;
        const media = vscode.Uri.joinPath(this.extensionUri, "media");
        view.webview.options = { enableScripts: true, localResourceRoots: [media] };
        view.webview.html = this.html(view.webview, media);
        const listener = view.webview.onDidReceiveMessage((message: unknown) => {
            if (!message || typeof message !== "object" || Array.isArray(message)) return;
            const value = message as Record<string, unknown>;
            if (typeof value.type !== "string") return;
            if (value.type === "ready") {
                this.ready = true;
                this.post({ type: "reset" });
                for (const item of this.history) this.post(item);
                this.post({ type: "state", ...this.state });
                this.onAction({ type: "ready" });
                return;
            }
            const actions = ["send", "cancel", "attachContext", "clearContext", "settings", "review", "newChat", "savedConversations", "copy"];
            if (!actions.includes(value.type)) return;
            const text = typeof value.text === "string" ? value.text : undefined;
            if (text && text.length > 100_000) return;
            this.onAction({ type: value.type, text, mode: typeof value.mode === "string" ? value.mode : undefined,
                includeContext: value.includeContext === true });
        });
        view.onDidDispose(() => { listener.dispose(); if (this.view === view) { this.view = undefined; this.ready = false; } });
    }

    update(state: Partial<ChatState>): void {
        this.state = { ...this.state, ...state };
        this.post({ type: "state", ...this.state });
    }

    message(role: "user" | "assistant" | "system", text: string): void { this.remember({ type: "message", role, text }); }

    event(kind: string, data: Record<string, unknown>): void {
        // Coalesce tokens for re-opening the view; keep the live path incremental.
        const last = this.history[this.history.length - 1];
        if (kind === "token" && last?.type === "event" && last.kind === "token") {
            const old = last.data as Record<string, unknown>;
            old.text = String(old.text || "") + String(data.text || "");
            this.post({ type: "event", kind, data });
        } else this.remember({ type: "event", kind, data });
    }

    reset(): void { this.history = []; this.post({ type: "reset" }); }

    restoreTranscript(messages: Array<{ role: string; content: string }>): void {
        this.history = [];
        this.post({ type: "reset" });
        for (const message of messages) {
            if (message.role !== "user" && message.role !== "assistant") continue;
            this.remember({ type: "message", role: message.role, text: message.content });
        }
    }

    private remember(message: Record<string, unknown>): void {
        this.history.push(message);
        while (this.history.length > 250 || (this.history.length > 1 && JSON.stringify(this.history).length > 600_000)) this.history.shift();
        this.post(message);
    }

    private post(message: Record<string, unknown>): void {
        if (this.ready && this.view) void this.view.webview.postMessage(message);
    }

    private html(webview: vscode.Webview, media: vscode.Uri): string {
        const nonce = randomBytes(24).toString("base64");
        const resource = (file: string) => webview.asWebviewUri(vscode.Uri.joinPath(media, file)).toString();
        const labels = {
            title: "Gregal", connectionConnected: vscode.l10n.t("Connected"), connectionOffline: vscode.l10n.t("Offline"),
            connectionConnecting: vscode.l10n.t("Connecting"), modelFallback: vscode.l10n.t("Your configured model"),
            newChat: vscode.l10n.t("New conversation"), savedConversations: vscode.l10n.t("Saved conversations"), settings: vscode.l10n.t("Connection settings"),
            welcomeEyebrow: vscode.l10n.t("YOUR CODING COMPANION"), welcomeTitle: vscode.l10n.t("What shall we build?"),
            welcomeBody: vscode.l10n.t("Explore an idea, fix a bug, or work through a change. Add code when you need it."),
            promptExplain: vscode.l10n.t("Explain this code"), promptFix: vscode.l10n.t("Find and fix a bug"),
            promptReview: vscode.l10n.t("Review my changes"), you: vscode.l10n.t("You"), gregal: "Gregal",
            thinking: vscode.l10n.t("Working through it…"), statusReady: vscode.l10n.t("Ready when you are"),
            statusWorking: vscode.l10n.t("Working"), statusWaiting: vscode.l10n.t("Waiting for your decision"),
            statusError: vscode.l10n.t("Something needs attention"), composerPlaceholder: vscode.l10n.t("Ask Gregal to help with your code…"),
            send: vscode.l10n.t("Send message"), stop: vscode.l10n.t("Stop run"), modeCode: vscode.l10n.t("Code"),
            modeChat: vscode.l10n.t("Chat"), modeInspect: vscode.l10n.t("Inspect"), includeContext: vscode.l10n.t("Include attached code"),
            attachSelection: vscode.l10n.t("Add selection or file"), contextLabel: vscode.l10n.t("Attached context"),
            removeContext: vscode.l10n.t("Remove context"), copy: vscode.l10n.t("Copy code"), copied: vscode.l10n.t("Copied"),
            toolRunning: vscode.l10n.t("Running tool"), toolDone: vscode.l10n.t("Tool finished"),
            delegateRunning: vscode.l10n.t("Subagent working"), delegateDone: vscode.l10n.t("Subagent finished"),
            approvalRequired: vscode.l10n.t("Permission required"), error: vscode.l10n.t("Error"),
            clearChat: vscode.l10n.t("Clear conversation"), review: vscode.l10n.t("Review changes"),
            conversation: vscode.l10n.t("Conversation"), modeSelector: vscode.l10n.t("Conversation mode"),
            statusQuestion: vscode.l10n.t("Waiting for your answer"), statusComplete: vscode.l10n.t("Run complete"),
            statusFailed: vscode.l10n.t("Run failed"), statusCancelled: vscode.l10n.t("Run stopped"),
            statusBlocked: vscode.l10n.t("Tool blocked"),
        };
        const json = JSON.stringify(labels).replace(/</g, "\\u003c").replace(/>/g, "\\u003e").replace(/&/g, "\\u0026");
        return `<!doctype html><html lang="${vscode.env.language.startsWith("ca") ? "ca" : "en"}"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src ${webview.cspSource}; style-src ${webview.cspSource}; script-src 'nonce-${nonce}';">
<link rel="stylesheet" href="${resource("chat.css")}"><title>Gregal</title></head><body>
<div id="app" data-icon-uri="${resource("icon.png")}"></div>
<script nonce="${nonce}" id="gregal-labels" type="application/json">${json}</script>
<script nonce="${nonce}" src="${resource("chat.js")}"></script></body></html>`;
    }
}
