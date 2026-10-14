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

// BACKEND_LOG_MAX topa el fitxer de log abans de rotar-lo (.old): el
// backend escriu poc (cicle de vida), però un log sense sostre és un
// problema d'espai a casa de l'usuari.
const BACKEND_LOG_MAX = 2 * 1024 * 1024;

function rotateBackendLog(file) {
  if (!file) return;
  try {
    if (fs.statSync(file).size < BACKEND_LOG_MAX) return;
    fs.renameSync(file, file + ".old");
  } catch (e) { /* primera arrencada o sense permís: mai trenca l'inici */ }
}

function appendBackendLog(file, line) {
  if (!file) return;
  try {
    fs.appendFileSync(file, String(line).trimEnd() + "\n");
  } catch (e) { /* disc ple o sense permís: el backend continua igual */ }
}

// backendEnv és l'entorn del backend: el de l'app més el que hi afegeix
// (p. ex. GREGAL_APP_BROWSER, el pont del navegador de l'app). Mai es
// modifica process.env: el token no ha d'anar a parar a cap altre fill.
function backendEnv(options) {
  const extra = options && options.env;
  if (!extra || typeof extra !== "object") return process.env;
  const env = { ...process.env };
  for (const [key, value] of Object.entries(extra)) {
    if (typeof value === "string" && value) env[key] = value;
  }
  return env;
}

function startServer(options) {
  const candidates = binaryCandidates(options);
  const args = serverArgs(options);
  const logFile = options.logFile || null;
  rotateBackendLog(logFile);

  let lastError;
  for (const binary of candidates) {
    if (binary.includes(path.sep) && !fs.existsSync(binary)) continue;
    try {
      const child = spawn(binary, args, {
        stdio: ["ignore", "pipe", "pipe"],
        windowsHide: true,
        env: backendEnv(options),
      });
      const logs = [];
      const push = line => {
        logs.push(String(line));
        if (logs.length > 50) logs.splice(0, logs.length - 50);
        appendBackendLog(logFile, line);
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

module.exports = { binaryCandidates, serverArgs, backendEnv, startServer, appendBackendLog, rotateBackendLog, BACKEND_LOG_MAX };
