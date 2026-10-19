import { createHash } from "crypto";

export const MAX_CONTEXT_CHARS = 24_000;

export interface EditorAttachment {
    label: string;
    detail: string;
    text: string;
    workspace?: string;
}

export function editorAttachment(input: {
    path: string; language: string; text: string; startLine?: number; endLine?: number;
    diagnostics?: string[]; workspace?: string;
}): EditorAttachment {
    const selection = input.startLine !== undefined;
    const range = selection ? `:${input.startLine}-${input.endLine ?? input.startLine}` : "";
    const truncated = input.text.length > MAX_CONTEXT_CHARS;
    const text = input.text.slice(0, MAX_CONTEXT_CHARS);
    const diagnostics = (input.diagnostics || []).slice(0, 10).map(s => s.slice(0, 500)).join("\n");
    return {
        label: input.path + range,
        detail: `${input.language} · ${text.length.toLocaleString()} characters${truncated ? " · truncated" : ""}`,
        text: `Source context: ${input.path}${range} (${input.language})\n\n${text}${truncated ? "\n[Context truncated; read the file for the remainder.]" : ""}${diagnostics ? "\n\nDiagnostics:\n" + diagnostics : ""}`,
        workspace: input.workspace,
    };
}

export function projectSession(configured: string, workspaceUri?: string): string {
    const raw = configured.trim();
    const base = /^[A-Za-z0-9_-]{1,64}$/.test(raw) ? raw : "vscode";
    // A custom session explicitly opts into sharing; defaults isolate projects.
    if (base !== "vscode" || !workspaceUri) return base;
    return "vscode-" + createHash("sha256").update(workspaceUri).digest("hex").slice(0, 12);
}
