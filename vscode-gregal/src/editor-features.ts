import * as vscode from "vscode";
import { inlineCompletion } from "./api";
import type { RunConnection } from "./run-client";

type DiagnosticAction = (document: vscode.TextDocument, diagnostic: vscode.Diagnostic, intent: "fix" | "explain") => Promise<void>;
type ConnectionFor = (folder: vscode.WorkspaceFolder) => Promise<RunConnection>;

function waitForQuietPeriod(ms: number, token: vscode.CancellationToken): Promise<boolean> {
    return new Promise(resolve => {
        if (token.isCancellationRequested) { resolve(false); return; }
        let subscription: vscode.Disposable;
        const timer = setTimeout(() => { subscription.dispose(); resolve(true); }, ms);
        subscription = token.onCancellationRequested(() => { clearTimeout(timer); subscription.dispose(); resolve(false); });
    });
}

export function registerEditorFeatures(
    context: vscode.ExtensionContext,
    connectionFor: ConnectionFor,
    onDiagnostic: DiagnosticAction,
): void {
    const requests = new Map<string, AbortController>();
    const inlineProvider: vscode.InlineCompletionItemProvider = {
        async provideInlineCompletionItems(document, position, _context, token) {
            if (!vscode.workspace.isTrusted || document.isClosed || ["plaintext", "log", "scminput"].includes(document.languageId)) return;
            const folder = vscode.workspace.getWorkspaceFolder(document.uri);
            if (!folder || vscode.workspace.getConfiguration("gregal", document.uri).get<boolean>("inlineCompletions.enabled", true) === false) return;
            const editor = vscode.window.activeTextEditor;
            if (editor?.document.uri.toString() === document.uri.toString() && !editor.selection.isEmpty) return;

            const key = document.uri.toString();
            requests.get(key)?.abort();
            const abort = new AbortController();
            requests.set(key, abort);
            const subscription = token.onCancellationRequested(() => abort.abort());
            try {
                if (_context.triggerKind === vscode.InlineCompletionTriggerKind.Automatic && !await waitForQuietPeriod(300, token)) return;
                if (abort.signal.aborted) return;
                const connection = await connectionFor(folder);
                const source = document.getText();
                const offset = document.offsetAt(position);
                const result = await inlineCompletion(connection.base, connection.token, connection.session, {
                    language: document.languageId,
                    prefix: source.slice(Math.max(0, offset - 7000), offset),
                    suffix: source.slice(offset, offset + 1500),
                    workspace: folder.uri.fsPath,
                }, { signal: abort.signal, timeoutMs: 12_000 });
                if (abort.signal.aborted || token.isCancellationRequested || requests.get(key) !== abort) return;
                const text = typeof result.completion === "string" ? result.completion : "";
                return text ? { items: [new vscode.InlineCompletionItem(text)] } : undefined;
            } catch {
                // Inline completion is opportunistic; network/model errors must not interrupt typing.
                return undefined;
            } finally {
                subscription.dispose();
                if (requests.get(key) === abort) requests.delete(key);
            }
        },
    };

    const codeActions: vscode.CodeActionProvider = {
        provideCodeActions(document, _range, context) {
            if (!vscode.workspace.isTrusted) return [];
            return context.diagnostics.slice(0, 5).flatMap(diagnostic => {
                const fix = new vscode.CodeAction(vscode.l10n.t("Ask Gregal to fix this diagnostic"), vscode.CodeActionKind.QuickFix);
                fix.isPreferred = true;
                fix.diagnostics = [diagnostic];
                fix.command = { title: fix.title, command: "gregal.fixDiagnostic", arguments: [document.uri, diagnostic] };
                const explain = new vscode.CodeAction(vscode.l10n.t("Ask Gregal to explain this diagnostic"), vscode.CodeActionKind.QuickFix);
                explain.diagnostics = [diagnostic];
                explain.command = { title: explain.title, command: "gregal.explainDiagnostic", arguments: [document.uri, diagnostic] };
                return [fix, explain];
            });
        },
    };

    context.subscriptions.push(
        vscode.languages.registerInlineCompletionItemProvider({ scheme: "file" }, inlineProvider),
        vscode.languages.registerCodeActionsProvider({ scheme: "file" }, codeActions, { providedCodeActionKinds: [vscode.CodeActionKind.QuickFix] }),
    );
    context.subscriptions.push(vscode.workspace.onDidCloseTextDocument(document => {
        requests.get(document.uri.toString())?.abort();
        requests.delete(document.uri.toString());
    }));
    context.subscriptions.push(vscode.workspace.onDidChangeConfiguration(event => {
        if (!event.affectsConfiguration("gregal.inlineCompletions.enabled")) return;
        for (const request of requests.values()) request.abort();
        requests.clear();
    }));

    context.subscriptions.push(
        vscode.commands.registerCommand("gregal.fixDiagnostic", (uri: vscode.Uri, diagnostic: vscode.Diagnostic) =>
            vscode.workspace.openTextDocument(uri).then(document => onDiagnostic(document, diagnostic, "fix"))),
        vscode.commands.registerCommand("gregal.explainDiagnostic", (uri: vscode.Uri, diagnostic: vscode.Diagnostic) =>
            vscode.workspace.openTextDocument(uri).then(document => onDiagnostic(document, diagnostic, "explain"))),
    );
}
