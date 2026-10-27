// Extensió de Chrome (extension/chrome): l'app la porta dins l'instal·lador i
// la deixa preparada en una carpeta fixa. Chrome no deixa que cap programa
// instal·li una extensió pel seu compte (només la Web Store o les polítiques
// d'empresa), així que l'app fa la resta: copia la carpeta, obre la pàgina
// d'extensions del navegador i ensenya la carpeta. La connexió no cal
// configurar-la: l'extensió troba l'app sola (port 8097, sense clau).
const fs = require("fs");
const path = require("path");
const { spawn } = require("child_process");

/** On és l'extensió: dins l'app instal·lada o, en desenvolupament, al repositori. */
function sourceDir({ packaged, resourcesPath, appDir }) {
    return packaged ? path.join(resourcesPath, "chrome-extension") : path.join(appDir, "..", "extension", "chrome");
}

/** La copia a una carpeta que no canvia entre versions; torna-la, o "" si no hi és. */
function syncExtension(src, dest) {
    if (!fs.existsSync(path.join(src, "manifest.json"))) return "";
    fs.mkdirSync(dest, { recursive: true });
    fs.cpSync(src, dest, {
        recursive: true,
        force: true,
        filter: f => !/\.test\.js$/.test(f) && path.basename(f) !== "package.json",
    });
    return dest;
}

/** Navegadors on es pot carregar: Chrome primer, Edge si no n'hi ha. */
function browserCandidates(platform, env) {
    if (platform === "win32") {
        const pf = env.ProgramFiles || "C:\\Program Files";
        const pf86 = env["ProgramFiles(x86)"] || "C:\\Program Files (x86)";
        const local = env.LOCALAPPDATA || "";
        return [
            { name: "chrome", exe: path.join(pf, "Google", "Chrome", "Application", "chrome.exe"), page: "chrome://extensions" },
            { name: "chrome", exe: path.join(pf86, "Google", "Chrome", "Application", "chrome.exe"), page: "chrome://extensions" },
            { name: "chrome", exe: path.join(local, "Google", "Chrome", "Application", "chrome.exe"), page: "chrome://extensions" },
            { name: "edge", exe: path.join(pf86, "Microsoft", "Edge", "Application", "msedge.exe"), page: "edge://extensions" },
            { name: "edge", exe: path.join(pf, "Microsoft", "Edge", "Application", "msedge.exe"), page: "edge://extensions" },
        ];
    }
    if (platform === "darwin") {
        return [
            { name: "chrome", exe: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", page: "chrome://extensions" },
            { name: "edge", exe: "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge", page: "edge://extensions" },
        ];
    }
    return ["/usr/bin/google-chrome", "/usr/bin/google-chrome-stable", "/usr/bin/chromium", "/usr/bin/chromium-browser"]
        .map(exe => ({ name: "chrome", exe, page: "chrome://extensions" }));
}

/** Obre la pàgina d'extensions al primer navegador que hi hagi; torna quin. */
function openExtensionsPage(platform = process.platform, env = process.env, exists = fs.existsSync, run = spawn) {
    const b = browserCandidates(platform, env).find(c => exists(c.exe));
    if (!b) return "";
    try {
        run(b.exe, [b.page], { detached: true, stdio: "ignore" }).unref();
        return b.name;
    } catch (e) {
        return "";
    }
}

module.exports = { sourceDir, syncExtension, browserCandidates, openExtensionsPage };
