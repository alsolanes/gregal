import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import ts from "typescript";

const readJson = (path: string) => JSON.parse(readFileSync(new URL(path, import.meta.url), "utf8"));
const manifest = readJson("../package.json");
const packageEnglish = readJson("../package.nls.json");
const packageCatalan = readJson("../package.nls.ca.json");
const runtimeEnglish = readJson("../l10n/bundle.l10n.json");
const runtimeCatalan = readJson("../l10n/bundle.l10n.ca.json");

function stringsIn(value: unknown): string[] {
    if (typeof value === "string") return [value];
    if (Array.isArray(value)) return value.flatMap(stringsIn);
    if (value && typeof value === "object") return Object.values(value).flatMap(stringsIn);
    return [];
}

function propertyPath(expression: ts.Expression): string {
    const parts: string[] = [];
    let current = expression;
    while (ts.isPropertyAccessExpression(current)) {
        parts.unshift(current.name.text);
        current = current.expression;
    }
    if (ts.isIdentifier(current)) parts.unshift(current.text);
    return parts.join(".");
}

function runtimeMessages(path: string): string[] {
    const source = readFileSync(new URL(path, import.meta.url), "utf8");
    const file = ts.createSourceFile(path, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
    const messages = new Set<string>();
    const visit = (node: ts.Node) => {
        if (ts.isCallExpression(node)) {
            const callee = node.expression;
            const localizer = propertyPath(callee) === "vscode.l10n.t" ||
                (ts.isIdentifier(callee) && callee.text === "localize");
            const first = node.arguments[0];
            if (localizer && first && ts.isStringLiteralLike(first)) messages.add(first.text);
        }
        ts.forEachChild(node, visit);
    };
    visit(file);
    return [...messages];
}

test("manifest and runtime strings have English and Catalan translations", () => {
    assert.equal(manifest.l10n, "./l10n");

    const contributionKeys = [...new Set(stringsIn(manifest).flatMap((value) =>
        [...value.matchAll(/%([^%]+)%/g)].map((match) => match[1]),
    ))].sort();
    assert.deepEqual(Object.keys(packageEnglish).sort(), contributionKeys);
    assert.deepEqual(Object.keys(packageCatalan).sort(), contributionKeys);

    assert.deepEqual(Object.keys(runtimeCatalan).sort(), Object.keys(runtimeEnglish).sort());
    const messages = [
        ...runtimeMessages("../src/extension.ts"),
        ...runtimeMessages("../src/sse.ts"),
    ];
    for (const message of messages) {
        assert.equal(runtimeEnglish[message], message, `English runtime string missing: ${message}`);
        assert.ok(Object.hasOwn(runtimeCatalan, message), `Catalan runtime string missing: ${message}`);
    }
});
