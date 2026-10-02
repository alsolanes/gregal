const fs = require("fs");
const path = require("path");
const { spawnSync } = require("child_process");

// Compila el backend per a la plataforma objectiu de l'empaquetat. El nom
// del binari ha de coincidir amb el que busca server-manager.js en temps
// d'execució: a Windows és gregal.exe, i abans sortia sempre "gregal" sense
// extensió, o sigui que el portable de Windows no trobava el seu backend.
//
// Objectiu: primer argument (node build-backend.js windows) o
// GREGAL_TARGET=windows|linux|darwin; per defecte, la màquina on corre. Va
// per argument perquè els scripts d'npm passen per cmd.exe a Windows i la
// sintaxi VAR=x ordre no hi funciona.
const desktopDir = __dirname;
const projectDir = path.resolve(desktopDir, "..");
const outDir = path.join(desktopDir, "backend");

const hostOS = { win32: "windows", linux: "linux", darwin: "darwin" }[process.platform] || process.platform;
const targetOS = process.argv[2] || process.env.GREGAL_TARGET || hostOS;
const exe = targetOS === "windows" ? "gregal.exe" : "gregal";

// A Windows no es pot esborrar un executable que està corrent, i el que
// sortia era un EPERM cru amb la ruta escapada quatre vegades, sense dir
// què el tenia agafat. Passa sempre que has deixat obert l'escriptori (o
// un backend de proves) i tornes a empaquetar.
try {
  fs.rmSync(outDir, { recursive: true, force: true });
} catch (e) {
  if (e.code !== "EPERM" && e.code !== "EBUSY") throw e;
  console.error(
    "\nbackend: no es pot esborrar " + outDir + ": hi ha un gregal corrent que en té agafat l'executable.\n" +
    "  Tanca l'app d'escriptori (o atura el backend) i torna-hi. Per veure qui és:\n" +
    "    Windows:  Get-Process gregal | Select-Object Id,Path\n" +
    "    Linux:    pgrep -a gregal\n");
  process.exit(1);
}
fs.mkdirSync(outDir, { recursive: true });

// findGo troba el toolchain: PATH primer; si no, on el deixen l'instal·lador
// oficial i winget a Windows, i GOROOT. Sense això, "spawnSync go ENOENT"
// era tot el que veies.
function findGo() {
  const candidates = [
    process.env.GOROOT && path.join(process.env.GOROOT, "bin", "go.exe"),
    process.env.GOROOT && path.join(process.env.GOROOT, "bin", "go"),
    "C:\\Program Files\\Go\\bin\\go.exe",
    process.env.LOCALAPPDATA && path.join(process.env.LOCALAPPDATA, "Programs", "Go", "bin", "go.exe"),
    "/usr/local/go/bin/go",
  ].filter(Boolean);
  for (const c of candidates) if (fs.existsSync(c)) return c;
  return "go"; // confiem en el PATH
}
const goBin = findGo();

const desktopVersion = require('./package.json').version;
const result = spawnSync(goBin, ["build", "-trimpath", "-buildvcs=false", "-ldflags", "-X main.version=v" + desktopVersion, "-o", path.join(outDir, exe), "."], {
  cwd: projectDir,
  stdio: "inherit",
  env: { ...process.env, GOOS: targetOS, GOARCH: process.env.GOARCH || "amd64", CGO_ENABLED: "0" },
});
if (result.error) {
  // Els checkouts de release ja poden portar el binari verificat a l'arrel.
  // Això permet reconstruir el paquet sense instal·lar el toolchain de Go.
  const existing = path.join(projectDir, exe);
  if (!fs.existsSync(existing)) {
    console.error(
      "\nNo trobo Go (ni al PATH, ni a Program Files, ni a GOROOT) i no hi ha cap " + exe + " compilat a l'arrel del repo.\n" +
      "Opcions:\n" +
      "  1. Instal·la Go 1.27:  winget install GoLang.Go   (o https://go.dev/dl) i torna a obrir el terminal.\n" +
      "  2. O posa un " + exe + " ja compilat a " + projectDir + " i l'empaquetaré tal qual.\n"
    );
    process.exit(1);
  }
  fs.copyFileSync(existing, path.join(outDir, exe));
  fs.chmodSync(path.join(outDir, exe), 0o755);
  console.log("Go no és al PATH; reutilitzo el binari existent " + existing);
  process.exit(0);
}
if (result.status) process.exit(result.status);
console.log("backend: " + path.join(outDir, exe) + " (" + targetOS + "/" + (process.env.GOARCH || "amd64") + ")");
