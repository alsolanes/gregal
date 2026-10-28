// Complement de Word (internal/web/app/addin): instal·lació d'un clic.
// Abans calia compartir una carpeta a la xarxa i afegir-la al Trust Center.
// Ara l'app demana el manifest al backend en marxa (sempre de la mateixa
// versió), hi posa el port real, el desa en una carpeta fixa i el registra a
// HKCU\Software\Microsoft\Office\16.0\WEF\Developer, la clau que fan servir
// les eines de Microsoft per carregar complements en local (valor = Id del
// manifest, dades = camí). A cada arrencada es torna a escriure amb el port
// d'aquell moment. Només Windows: el Word de Mac no llegeix aquesta clau.
const fs = require("fs");
const path = require("path");
const { execFile } = require("child_process");

const REG_KEY = "HKCU\\Software\\Microsoft\\Office\\16.0\\WEF\\Developer";
const DEFAULT_ORIGIN = "http://127.0.0.1:8097";
const FILE = "gregal-addin.xml";

/** L'Id del manifest (el nom del valor al registre), o "" si no en té. */
function manifestId(xml) {
    const m = /<Id>\s*([0-9A-Fa-f-]{36})\s*<\/Id>/.exec(String(xml || ""));
    return m ? m[1] : "";
}

/**
 * Canvia l'origen per defecte (127.0.0.1:8097 i localhost:8097) pel del
 * backend real: SourceLocation, icones i AppDomains.
 */
function withOrigin(xml, origin) {
    const u = new URL(origin);
    const loopback = u.hostname === "127.0.0.1" || u.hostname === "localhost";
    const main = u.origin;
    const alias = loopback ? `${u.protocol}//localhost${u.port ? ":" + u.port : ""}` : main;
    return String(xml)
        .split(DEFAULT_ORIGIN).join(main)
        .split("http://localhost:8097").join(alias);
}

/** Desa el manifest a la carpeta fixa; torna el camí. */
function writeManifest(xml, dir) {
    fs.mkdirSync(dir, { recursive: true });
    const file = path.join(dir, FILE);
    fs.writeFileSync(file, xml);
    return file;
}

function reg(args, run = execFile) {
    return new Promise(resolve => {
        run("reg", args, { windowsHide: true }, (error, stdout) => resolve({ ok: !error, out: String(stdout || "") }));
    });
}

/** Registra el manifest perquè el Word el carregui en obrir-se. */
async function register(id, file, run) {
    return (await reg(["add", REG_KEY, "/v", id, "/t", "REG_SZ", "/d", file, "/f"], run)).ok;
}

/** El treu del registre (el Word el deixa de mostrar en tornar-lo a obrir). */
async function unregister(id, run) {
    return (await reg(["delete", REG_KEY, "/v", id, "/f"], run)).ok;
}

/** Camí registrat per a aquest Id, o "" si no hi és. */
async function registeredPath(id, run) {
    const r = await reg(["query", REG_KEY, "/v", id], run);
    if (!r.ok) return "";
    const m = new RegExp(id.replace(/-/g, "\\-") + "\\s+REG_SZ\\s+(.+)", "i").exec(r.out);
    return m ? m[1].trim() : "";
}

module.exports = { REG_KEY, FILE, manifestId, withOrigin, writeManifest, register, unregister, registeredPath };
