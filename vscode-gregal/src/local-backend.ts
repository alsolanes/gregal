import { spawn, type ChildProcess } from "node:child_process";
import { createHash, timingSafeEqual } from "node:crypto";
import { createWriteStream } from "node:fs";
import { access, chmod, mkdir, rename, rm } from "node:fs/promises";
import * as https from "node:https";
import * as path from "node:path";
import { Readable } from "node:stream";
import { pipeline } from "node:stream/promises";
import * as vscode from "vscode";

const RELEASE_ASSETS = "https://github.com/alsolanes/gregal/releases/latest/download/";
const MAX_BINARY_BYTES = 180 * 1024 * 1024;

function platformAsset(): string {
    const os = process.platform === "win32" ? "windows" : process.platform === "darwin" ? "darwin" : process.platform;
    const arch = process.arch === "x64" ? "amd64" : process.arch === "arm64" ? "arm64" : "";
    if (!["windows", "linux", "darwin"].includes(os) || !arch) {
        throw new Error(vscode.l10n.t("Automatic backend installation is unavailable for this operating system or processor."));
    }
    return `gregal-${os}-${arch}${os === "windows" ? ".exe" : ""}`;
}

function download(url: string, maxBytes: number, progress?: (received: number, total: number) => void, redirects = 0): Promise<Buffer> {
    return new Promise((resolve, reject) => {
        const req = https.get(url, { headers: { "User-Agent": "Gregal-VSCode", Accept: "application/octet-stream" } }, res => {
            const status = res.statusCode || 0;
            if ([301, 302, 303, 307, 308].includes(status) && res.headers.location && redirects < 5) {
                res.resume();
                const next = new URL(res.headers.location, url);
                if (next.protocol !== "https:") { reject(new Error("Release download redirected to a non-HTTPS address.")); return; }
                void download(next.toString(), maxBytes, progress, redirects + 1).then(resolve, reject);
                return;
            }
            if (status !== 200) { res.resume(); reject(new Error(`GitHub release download returned HTTP ${status}.`)); return; }
            const total = Number(res.headers["content-length"] || 0);
            if (total > maxBytes) { res.destroy(); reject(new Error("Release asset exceeds the allowed download size.")); return; }
            const chunks: Buffer[] = [];
            let received = 0;
            res.on("data", (chunk: Buffer) => {
                received += chunk.length;
                if (received > maxBytes) { res.destroy(new Error("Release asset exceeds the allowed download size.")); return; }
                chunks.push(chunk);
                progress?.(received, total);
            });
            res.once("error", reject);
            res.once("end", () => resolve(Buffer.concat(chunks, received)));
        });
        req.setTimeout(30_000, () => req.destroy(new Error("GitHub release download timed out.")));
        req.once("error", reject);
    });
}

/** Owns only the local process started by this extension. Configured remote
 * servers and processes started outside VS Code are never stopped here. */
export class LocalBackend {
    private child: ChildProcess | undefined;
    private output: vscode.OutputChannel | undefined;

    constructor(private readonly storage: vscode.Uri) {}

    async install(output: vscode.OutputChannel, progress?: (received: number, total: number) => void): Promise<string> {
        const asset = platformAsset();
        output.appendLine(`Downloading ${asset} and SHA256SUMS.txt from the latest Gregal release.`);
        const [checksums, binary] = await Promise.all([
            download(RELEASE_ASSETS + "SHA256SUMS.txt", 1_000_000),
            download(RELEASE_ASSETS + encodeURIComponent(asset), MAX_BINARY_BYTES, progress),
        ]);
        const row = checksums.toString("utf8").split(/\r?\n/).find(line => line.trim().split(/\s+/).at(-1) === asset);
        const expected = row?.trim().split(/\s+/)[0] || "";
        if (!/^[a-f0-9]{64}$/i.test(expected)) throw new Error(vscode.l10n.t("The latest release does not include a valid checksum for {0}.", asset));
        const actual = createHash("sha256").update(binary).digest();
        const declared = Buffer.from(expected, "hex");
        if (declared.length !== actual.length || !timingSafeEqual(declared, actual)) {
            throw new Error(vscode.l10n.t("The checksum for {0} did not match. The download was discarded.", asset));
        }

        await mkdir(this.storage.fsPath, { recursive: true });
        const target = path.join(this.storage.fsPath, asset);
        const temporary = target + `.download-${process.pid}-${Date.now()}`;
        try {
            await pipeline(Readable.from([binary]), createWriteStream(temporary, { mode: 0o700 }));
            if (process.platform !== "win32") await chmod(temporary, 0o700);
            await rename(temporary, target);
        } catch (error) {
            await rm(temporary, { force: true }).catch(() => undefined);
            throw error;
        }
        output.appendLine(`Installed and verified ${asset} at ${target}.`);
        return target;
    }

    async start(output: vscode.OutputChannel, executableOverride?: string): Promise<void> {
        if (this.child && this.child.exitCode === null && !this.child.killed) return;
        const config = vscode.workspace.getConfiguration("gregal");
        const configured = config.get<string>("executablePath")?.trim();
        const managed = path.join(this.storage.fsPath, platformAsset());
        let installed = false;
        try { await access(managed); installed = true; } catch { /* Not installed here. */ }
        const executable = executableOverride || configured || (installed ? managed : "gregal");
        const cwd = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
        this.output = output;
        output.appendLine(`Starting local backend: ${executable} --serve`);

        await new Promise<void>((resolve, reject) => {
            let settled = false;
            const child = spawn(executable, ["--serve"], {
                ...(cwd ? { cwd } : {}),
                windowsHide: true,
                stdio: ["ignore", "pipe", "pipe"],
            });
            this.child = child;
            const fail = (error: Error) => {
                output.appendLine(`Backend could not start: ${error.message}`);
                if (this.child === child) this.child = undefined;
                if (!settled) { settled = true; reject(error); }
            };
            const log = (source: string) => (chunk: Buffer) => {
                for (const line of chunk.toString("utf8").split(/\r?\n/)) {
                    if (line) output.appendLine(`[backend ${source}] ${line}`);
                }
            };
            child.stdout?.on("data", log("out"));
            child.stderr?.on("data", log("err"));
            child.once("error", fail);
            child.once("spawn", () => {
                if (!settled) { settled = true; resolve(); }
                output.appendLine(`Local backend started (pid ${child.pid ?? "unknown"}).`);
            });
            child.once("exit", (code, signal) => {
                output.appendLine(`Local backend stopped (code ${code ?? "none"}, signal ${signal ?? "none"}).`);
                if (this.child === child) this.child = undefined;
                if (!settled) fail(new Error("The Gregal process exited before it started."));
            });
        });
    }

    stop(): boolean {
        const child = this.child;
        if (!child || child.exitCode !== null || child.killed) return false;
        this.output?.appendLine("Stopping the local backend started by VS Code.");
        child.kill();
        this.child = undefined;
        return true;
    }
}
