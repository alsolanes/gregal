import { test } from "node:test";
import assert from "node:assert/strict";
import { editorAttachment, MAX_CONTEXT_CHARS, projectSession } from "../src/editor-context.ts";

test("default sessions isolate workspaces while explicit shared IDs stay stable", () => {
    assert.equal(projectSession("vscode", "file:///project-a"), projectSession("vscode", "file:///project-a"));
    assert.notEqual(projectSession("vscode", "file:///project-a"), projectSession("vscode", "file:///project-b"));
    assert.equal(projectSession("shared-review", "file:///project-a"), "shared-review");
    assert.match(projectSession("../invalid", "file:///project-a"), /^[A-Za-z0-9_-]{1,64}$/);
});

test("attachments retain the selected range and bound source and diagnostics", () => {
    const attachment = editorAttachment({ path: "src/example.ts", language: "typescript", startLine: 17, endLine: 22,
        text: "x".repeat(MAX_CONTEXT_CHARS + 10), workspace: "/project-a", diagnostics: Array(20).fill("a".repeat(800)) });
    assert.equal(attachment.label, "src/example.ts:17-22");
    assert.ok(attachment.text.includes("Context truncated"));
    assert.ok(attachment.text.length < MAX_CONTEXT_CHARS + 6000);
    assert.equal(attachment.workspace, "/project-a");
    assert.equal((attachment.text.match(/a{500}/g) || []).length, 10);
});
