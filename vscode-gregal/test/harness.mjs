// Harness: carrega dist/extension.js amb un vscode simulat i executa
// gregal.ask de punta a punta contra el servidor real.
// Ús: GREGAL_URL=... GREGAL_TOKEN=... node test/harness.mjs
import Module from "node:module";
import fs from "node:fs";

const URL = process.env.GREGAL_URL || "http://127.0.0.1:8097";
const TOKEN = process.env.GREGAL_TOKEN || "";
if (!TOKEN) {
    console.error("cal GREGAL_TOKEN");
    process.exit(2);
}

let output = "";
const commands = {};
const fakeVscode = {
    workspace: {
        getConfiguration: () => ({
            get: (k) => (k === "baseUrl" ? URL : k === "token" ? TOKEN : ""),
        }),
        asRelativePath: (p) => p,
    },
    languages: {
        getDiagnostics: () => [
            { message: "b no s'usa (unused)", range: { start: { line: 2 } } },
        ],
    },
    window: {
        activeTextEditor: {
            selection: { isEmpty: false },
            document: {
                languageId: "go",
                fileName: "/tmp/prova.go",
                getText: () => "package main\n\nfunc suma(a, b int) int { return a + b }\n",
            },
        },
        createOutputChannel: () => ({
            clear: () => {},
            append: (t) => {
                output += t;
            },
            appendLine: (t) => {
                output += t + "\n";
            },
            show: () => {},
        }),
        showInputBox: async () => "Què fa aquesta funció? Respon amb EXACTE: SUMA-DOS-NOMBRES.",
        showWarningMessage: async () => "Denega",
    },
    commands: {
        registerCommand: (id, fn) => {
            commands[id] = fn;
            return { dispose: () => {} };
        },
    },
};

const origLoad = Module._load;
Module._load = function (req, ...rest) {
    if (req === "vscode") return fakeVscode;
    return origLoad.call(this, req, ...rest);
};

const ext = (await import("../dist/extension.js")).default ?? (await import("../dist/extension.js"));
const mod = await import("../dist/extension.js");
mod.activate({ subscriptions: [] });
await commands["gregal.ask"]();
// L'SSE és async via events; espera fins a 4 min que arribi el reply.
const t0 = Date.now();
while (!output.includes("SUMA-DOS-NOMBRES") && Date.now() - t0 < 240000) {
    await new Promise((r) => setTimeout(r, 2000));
}
fs.writeFileSync("/tmp/vscode-harness-out.txt", output);
if (!output.includes("SUMA-DOS-NOMBRES")) {
    console.error("HARNESS FAIL: sense reply; sortida a /tmp/vscode-harness-out.txt");
    process.exit(1);
}
console.log("HARNESS OK: reply del servidor via el bundle real");
