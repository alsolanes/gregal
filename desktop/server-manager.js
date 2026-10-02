const fs = require("fs");
const path = require("path");
const { spawn } = require("child_process");

function binaryCandidates({ resourcesPath, appDir, platform = process.platform }) {
  const exe = platform === "win32" ? "gregal.exe" : "gregal";
  return [
    resourcesPath && path.join(resourcesPath, "backend", exe),
    path.join(appDir, "backend", exe),
    path.resolve(appDir, "..", exe),
    exe,
  ].filter(Boolean);
}

function serverArgs(options) {
  const args = ["--serve", "--addr", "127.0.0.1", "--port", String(options.port)];
  if (options.token) args.push("--token", options.token);
  if (options.dir) args.push("--dir", options.dir);
  if (options.config) args.push("--config", options.config);
  return args;
}

function startServer(options) {
  const candidates = binaryCandidates(options);
  const args = serverArgs(options);

  let lastError;
  for (const binary of candidates) {
    if (binary.includes(path.sep) && !fs.existsSync(binary)) continue;
    try {
      const child = spawn(binary, args, {
        stdio: ["ignore", "pipe", "pipe"],
        windowsHide: true,
        env: process.env,
      });
      const logs = [];
      const push = line => {
        logs.push(String(line));
        if (logs.length > 50) logs.splice(0, logs.length - 50);
        options.onLog?.(String(line));
      };
      child.stdout.on("data", push);
      child.stderr.on("data", push);
      child.on("error", err => {
        push("backend spawn error: " + err.message);
        options.onExit?.(-1, logs);
      });
      child.on("exit", code => options.onExit?.(code, logs));
      return { child, binary, logs };
    } catch (error) {
      lastError = error;
    }
  }
  throw lastError || new Error("Gregal executable not found");
}

module.exports = { binaryCandidates, serverArgs, startServer };
