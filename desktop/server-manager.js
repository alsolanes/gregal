const fs = require("fs");
const path = require("path");
const { spawn } = require("child_process");

// backendName és el binari de cada edició: Gregal porta gregal(.exe), sense
// els jocs; Gregal Jocs porta gregal-jocs(.exe) (jocs/cmd/gregal-jocs), el
// mateix servidor amb els jocs. build-backend.js els deixa a backend/ i a
// backend-jocs/; l'empaquetat de cada edició el posa a resources/backend/.
function backendName(edition, platform = process.platform) {
  const base = edition === "jocs" ? "gregal-jocs" : "gregal";
  return platform === "win32" ? base + ".exe" : base;
}

// Cada edició té la seva identitat de servei dins del mateix perfil (el Go
// ho fa amb service.SetProduct): Gregal, service.json i el port 8097 (el que
// busca l'extensió de Chrome); Gregal Jocs, service-jocs.json i el 8197. Així
// les dues apps corren alhora sobre les mateixes dades i cadascuna troba
// el seu servidor.
function serviceFileName(edition) {
  return edition === "jocs" ? "service-jocs.json" : "service.json";
}

function defaultPort(edition) {
  return edition === "jocs" ? 8197 : 8097;
}

function binaryCandidates({ resourcesPath, appDir, platform = process.platform, edition = "gregal" }) {
  const exe = backendName(edition, platform);
  const devDir = edition === "jocs" ? "backend-jocs" : "backend";
  return [
    resourcesPath && path.join(resourcesPath, "backend", exe),
    path.join(appDir, devDir, exe),
    path.resolve(appDir, "..", exe),
    exe,
  ].filter(Boolean);
}

// --lifeline: el backend surt net quan se li tanca l'stdin. L'app el tanca
// per aturar-lo (a Windows, kill és un TerminateProcess que no deixa
// esborrar service.json ni tancar els torns) i, si l'app peta, el sistema
// el tanca igualment i el backend no queda orfe amb el port i el lock.
function serverArgs(options) {
  const args = ["--serve", "--lifeline", "--addr", "127.0.0.1", "--port", String(options.port)];
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

// openBackendLog obre el fitxer de log del backend una sola vegada, en mode
// afegir. Abans cada línia era un appendFileSync al fil principal d'Electron
// (obrir, escriure i tancar el fitxer a cada tros de sortida): amb un backend
// xerraire, la finestra se'n ressentia. Un error del disc (ple, sense
// permís) només fa que es perdin línies: mai tomba l'app ni el backend.
function openBackendLog(file) {
  if (!file) return { write() {}, close(cb) { if (cb) cb(); } };
  let stream = null;
  try {
    stream = fs.createWriteStream(file, { flags: "a" });
    stream.on("error", () => { stream = null; });
  } catch (e) {
    stream = null;
  }
  return {
    write(line) {
      if (!stream) return;
      try { stream.write(String(line).trimEnd() + "\n"); } catch (e) { /* tancat o sense disc */ }
    },
    close(cb) {
      const s = stream;
      stream = null;
      if (!s) { if (cb) cb(); return; }
      s.end(() => { if (cb) cb(); });
    },
  };
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

// appBackendEnv és l'entorn extra del backend que engega l'app. L'app és
// d'un sol usuari, així que les claus d'API que s'hi escriuen es desen al
// magatzem de claus (auth.yaml, 0600) i sobreviuen al reinici, com a
// Android. Qui hagi posat GREGAL_PERSIST_KEYS a l'entorn mana.
function appBackendEnv(extra, baseEnv) {
  const env = { ...(extra || {}) };
  const base = baseEnv || process.env;
  if (!base.GREGAL_PERSIST_KEYS) env.GREGAL_PERSIST_KEYS = "1";
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
        stdio: ["pipe", "pipe", "pipe"],
        windowsHide: true,
        env: backendEnv(options),
      });
      // La línia de vida: no s'hi escriu res. Si el backend ja és mort,
      // tancar-la dona EPIPE, que no ha de tombar l'app.
      child.stdin.on("error", () => {});
      const logs = [];
      const log = openBackendLog(logFile);
      const push = line => {
        logs.push(String(line));
        if (logs.length > 50) logs.splice(0, logs.length - 50);
        log.write(line);
        options.onLog?.(String(line));
      };
      child.stdout.on("data", push);
      child.stderr.on("data", push);
      child.on("error", err => {
        push("backend spawn error: " + err.message);
        log.close();
        options.onExit?.(-1, logs);
      });
      // 'close' (no 'exit'): arriba quan stdout i stderr ja s'han buidat, i
      // així les últimes línies també van al fitxer.
      child.on("close", () => log.close());
      child.on("exit", code => options.onExit?.(code, logs));
      return { child, binary, logs };
    } catch (error) {
      lastError = error;
    }
  }
  throw lastError || new Error("Gregal executable not found");
}

// stopServer atura el backend net: tanca la línia de vida i espera que surti;
// si en timeoutMs no ho ha fet, el mata. Torna true si ha sortit sol.
function stopServer(child, timeoutMs = 1500) {
  return new Promise(resolve => {
    if (!child || child.exitCode !== null || child.signalCode !== null) return resolve(true);
    let done = false;
    const finish = graceful => {
      if (done) return;
      done = true;
      clearTimeout(timer);
      resolve(graceful);
    };
    const timer = setTimeout(() => {
      try { child.kill(); } catch (e) { /* ja no hi és */ }
      finish(false);
    }, timeoutMs);
    child.once("exit", () => finish(true));
    try {
      if (child.stdin && !child.stdin.destroyed) child.stdin.end();
      else child.kill();
    } catch (e) {
      try { child.kill(); } catch (_) {}
    }
  });
}

module.exports = { backendName, serviceFileName, defaultPort, binaryCandidates, serverArgs, backendEnv, appBackendEnv, startServer, stopServer, openBackendLog, rotateBackendLog, BACKEND_LOG_MAX };
