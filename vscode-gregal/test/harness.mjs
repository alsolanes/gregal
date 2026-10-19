// Optional live backend smoke check (Node.js 24+).
// The deterministic bundle/VS Code-host tests live in extension.test.cjs.
// Run only against a backend/model you intend to use:
// GREGAL_URL=... GREGAL_TOKEN=... node test/harness.mjs
import { runAgent } from "../src/run-client.ts";

const controller = new AbortController();
const timeout = setTimeout(() => controller.abort(), 240_000);
let reply = "";
try {
    await runAgent({
        base: process.env.GREGAL_URL || "http://127.0.0.1:8097",
        token: process.env.GREGAL_TOKEN || "",
        session: "vscode-smoke",
    }, {
        task: "Reply with exactly GREGAL-SMOKE-OK. Do not use tools.",
        mode: "chat",
    }, {
        onEvent(kind, data) {
            if (kind === "token") reply += String(data.text || "");
            if (kind === "assistant") reply = String(data.text || reply);
            if (kind === "done") reply = String(data.reply || reply);
        },
        onApproval: async () => false,
        onQuestion: async () => "",
    }, controller.signal);
    if (!reply.includes("GREGAL-SMOKE-OK")) throw new Error("Smoke check did not receive the expected reply");
    console.log("Gregal backend smoke check passed");
} catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
} finally {
    clearTimeout(timeout);
}
