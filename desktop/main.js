// Finestra nativa del Gregal. En local gestiona el backend automàticament;
// GREGAL_URL continua permetent connectar-se a un servidor extern.
const { app, BrowserWindow, Menu, Notification, dialog, ipcMain, nativeTheme, shell } = require("electron");

// TITLEBAR_H ha de casar amb l'alçada de #sessionTabs a index.html: és
// aquella fila la que fa de barra de títol.
const TITLEBAR_H = 38;

// Colors d'arrencada, abans que la pàgina hagi carregat i pugui dir el
// tema que té desat. Surten de docs/identitat.md i es tria pel tema del
// sistema, que és el que el gregal segueix si no tries res: néixer sempre
// fosc feia un flaix negre a qui va en clar.
const TEMA = {
    fosc: { bg: "#171A19", barra: "#101312", glif: "#F4F4EF" },
    clar: { bg: "#F5F7F8", barra: "#EBEFF1", glif: "#0B1220" },
};
function temaInicial() {
    try {
        return nativeTheme.shouldUseDarkColors ? TEMA.fosc : TEMA.clar;
    } catch (e) {
        return TEMA.fosc;
    }
}
const http = require("http");
const https = require("https");
const net = require("net");
const fs = require("fs");
const path = require("path");
const { startServer } = require("./server-manager");
const { loadState, saveState } = require("./window-state");
const { allowed, loadPreferences, savePreferences, keys: preferenceKeys } = require("./preferences");
const { sameOrigin, externalURL, localServiceURL, trustedPage } = require('./navigation');

let BASE = (process.env.GREGAL_URL || "http://127.0.0.1:8097").replace(/\/$/, "");
if (!externalURL(BASE)) throw new Error('GREGAL_URL must be an HTTP(S) URL without embedded credentials');
const MANAGED = !process.env.GREGAL_URL;
let PORT = Number(new URL(BASE).port || 8097);
// El backend gestionat només escolta a loopback: no necessita token i així no
// depèn del query-token, que el servidor Bearer-only ja no accepta. Per a una
// URL externa el token continua sent explícit i s'envia per Authorization des
// de la webapp.
const TOKEN = process.env.GREGAL_TOKEN || "";
let backend = null;
let managedPortChosen = false;
let expectedInstanceID = null;
let ignoreDescriptorOnce = false;

function serviceDescriptorPath() {
    const dir = process.env.GREGAL_DATA_DIR || path.join(app.getPath("home"), ".local", "share", "gregal");
    return path.join(dir, "service.json");
}

function readServiceDescriptor() {
    try {
        const d = JSON.parse(fs.readFileSync(serviceDescriptorPath(), "utf8"));
        if (d && d.protocol === "gregal.v1" && localServiceURL(d.address)) return d;
    } catch (e) {}
    return null;
}

function freePort() {
    return new Promise((resolve, reject) => {
        const probe = net.createServer();
        probe.unref();
        probe.on("error", reject);
        probe.listen(0, "127.0.0.1", () => {
            const port = probe.address().port;
            probe.close(() => resolve(port));
        });
    });
}

function idiomaApp() {
    try {
        const home = app.getPath("home");
        const cfgPath = process.env.GREGAL_CONFIG || path.join(home, ".config", "gregal", "config.yaml");
        const fs = require("fs");
        if (fs.existsSync(cfgPath)) {
            const raw = fs.readFileSync(cfgPath, "utf8");
            const m = raw.match(/^\s*lang:\s*([a-zA-Z]+)/m);
            if (m && m[1]) return m[1].toLowerCase() === 'ca' ? 'ca' : 'en';
        }
    } catch (e) {}
    return "en";
}

function ping() {
    return new Promise((resolve) => {
        // Health is deliberately smaller than /api/state and carries the
        // service identity/protocol. This prevents attaching the window to a
        // different process that happens to reuse the same dynamic port.
        const u = new URL(BASE + "/api/health");
        const lib = u.protocol === "https:" ? https : http;
        const opts = { timeout: 5000, headers: {} };
        if (TOKEN) {
            opts.headers["Authorization"] = "Bearer " + TOKEN;
        }
        const req = lib.get(u, opts, (res) => {
            let body = "";
            res.setEncoding("utf8");
            res.on("data", chunk => { body += chunk; });
            res.on("end", () => {
                if (res.statusCode !== 200) return resolve(false);
                try {
                    const health = JSON.parse(body);
                    if (health.status !== "ok" || health.protocol !== "gregal.v1" || !health.instance_id) return resolve(false);
                    if (expectedInstanceID && health.instance_id !== expectedInstanceID) return resolve(false);
                    if (!expectedInstanceID) expectedInstanceID = health.instance_id;
                    resolve(true);
                } catch (e) {
                    resolve(false);
                }
            });
        });
        req.on("error", () => resolve(false));
        req.on("timeout", () => {
            req.destroy();
            resolve(false);
        });
    });
}

async function waitForServer(timeoutMs = 20000, backendLogs) {
    const limit = Date.now() + timeoutMs;
    let n = 0;
    while (Date.now() < limit) {
        if (await ping()) return true;
        // Mort prematura del backend: no té sentit esperar els 20s amb un
        // missatge genèric. S'atura de seguida i es mostren els logs.
        if (backendLogs && backendLogs.earlyExit) return false;
        if (++n % 10 === 0) console.log("[gregal] esperant el backend… (" + Math.round((Date.now() - (limit - timeoutMs)) / 1000) + "s)");
        await new Promise(resolve => setTimeout(resolve, 200));
    }
    return false;
}

let lastBackendLogs = [];
let backendEarlyExit = null;

async function ensureServer() {
    let descriptorCandidate = false;
    if (MANAGED && !managedPortChosen) {
        if (!ignoreDescriptorOnce) {
            const descriptor = readServiceDescriptor();
            if (descriptor) {
                try {
                    const u = new URL(descriptor.address);
                    BASE = descriptor.address.replace(/\/$/, "");
                    PORT = Number(u.port || (u.protocol === "https:" ? 443 : 80));
                    descriptorCandidate = true;
                } catch (error) {}
            }
        }
        if (!descriptorCandidate) {
            try {
                PORT = await freePort();
            } catch (error) {
                console.error("[gregal] no s'ha pogut trobar un port lliure: " + error.message);
                return false;
            }
            BASE = "http://127.0.0.1:" + PORT;
            ignoreDescriptorOnce = false;
        }
        managedPortChosen = true;
    }
    if (await ping()) return true;
    if (MANAGED && descriptorCandidate) {
        // El descriptor és vell o el procés ja no hi és: ignora'l una vegada
        // i tria un port nou per arrencar el backend gestionat.
        ignoreDescriptorOnce = true;
        managedPortChosen = false;
        expectedInstanceID = null;
        return ensureServer();
    }
    if (!MANAGED) return false;
    try {
        const started = startServer({
            resourcesPath: process.resourcesPath,
            appDir: __dirname,
            platform: process.platform,
            port: PORT,
            token: TOKEN,
            config: process.env.GREGAL_CONFIG,
            dir: process.env.GREGAL_DIR || app.getPath("home"),
            onLog: line => console.log("[gregal] " + line.trimEnd()),
            onExit: (code, logs) => {
                backendEarlyExit = code;
                lastBackendLogs = (logs || []).slice(-20);
            },
        });
        backend = started.child;
        lastBackendLogs = started.logs || [];
        backend.on("error", error => console.error("[gregal] " + error.message));
        backend.on("exit", code => {
            console.log("[gregal] backend aturat (" + code + ")");
            backend = null;
        });
        const ok = await waitForServer(20000, { get earlyExit() { return backendEarlyExit !== null; } });
        if (!ok) {
            const tail = lastBackendLogs.map(l => String(l).trimEnd()).filter(Boolean).slice(-12).join("\n");
            console.error("[gregal] backend no respona a " + BASE + (tail ? "\n" + tail : ""));
        }
        return ok;
    } catch (error) {
        console.error("[gregal] no s'ha pogut iniciar: " + error.message);
        lastBackendLogs = [String(error.message)];
        return false;
    }
}

// --- IPC: el que la pàgina pot demanar a l'escriptori (via preload) ---
function wireIPC(getWin) {
    const prefDir = app.getPath('userData');
    let preferences = loadPreferences(prefDir);
    const trusted = event => event.sender === getWin()?.webContents && event.senderFrame === getWin()?.webContents.mainFrame && trustedPage(event.senderFrame.url, BASE, path.join(__dirname, 'error.html'));
    ipcMain.on('gregal:preferences', event => {
        event.returnValue = trusted(event) ? { values: preferences, keys: preferenceKeys } : { values: {}, keys: [] };
    });
    ipcMain.on('gregal:connection', event => {
        event.returnValue = trusted(event) ? { token: TOKEN } : { token: '' };
    });
    ipcMain.on('gregal:preference-save', (event, key, value) => {
        if (!trusted(event) || !allowed(key) || (value !== null && (typeof value !== 'string' || value.length > 4096))) return;
        if (value === null) delete preferences[key];
        else preferences[key] = value;
        try { savePreferences(prefDir, preferences); } catch (error) { console.error('[gregal] preferències: ' + error.message); }
    });
    ipcMain.handle("gregal:choose-folder", async event => {
        if (!trusted(event)) return null;
        const win = getWin();
        const isEn = idiomaApp() === "en";
        const res = await dialog.showOpenDialog(win, {
            title: isEn ? "Choose Project" : "Tria el projecte",
            properties: ["openDirectory", "createDirectory"],
        });
        if (res.canceled || !res.filePaths.length) return null;
        return res.filePaths[0];
    });
    // Reintent d'arrencada quan el backend ha fallat
    ipcMain.handle("gregal:retry-backend", async event => {
        if (!trusted(event)) return { ok: false };
        const win = getWin();
        managedPortChosen = false;
        expectedInstanceID = null;
        ignoreDescriptorOnce = false;
        backendEarlyExit = null;
        lastBackendLogs = [];
        const ok = await ensureServer();
        if (ok && win) {
            await win.loadURL(BASE + "/");
            return { ok: true };
        }
        return { ok: false };
    });
    // Els botons de finestra els pinta el sistema, no el CSS: sense això
    // el color es quedava al fosc escrit a mà i en tema clar sortia un
    // requadre negre a dalt a la dreta. Només s'accepten colors #rrggbb,
    // que això ve de la pàgina.
    const colorOK = c => typeof c === "string" && /^#[0-9a-fA-F]{6}$/.test(c);
    ipcMain.handle("gregal:titlebar", (event, values = {}) => {
        if (!trusted(event) || !values || typeof values !== 'object') return false;
        const { color, symbolColor } = values;
        const win = getWin();
        if (!win || !win.setTitleBarOverlay) return false;
        if (!colorOK(color) || !colorOK(symbolColor)) return false;
        try {
            win.setTitleBarOverlay({ color, symbolColor, height: TITLEBAR_H });
            return true;
        } catch (e) {
            // A macOS no hi ha overlay (hi ha els semàfors): no és cap error.
            return false;
        }
    });
    ipcMain.handle("gregal:notify", (event, values = {}) => {
        if (!trusted(event) || !values || typeof values !== 'object') return false;
        const { title, body } = values;
        if (typeof title !== 'string' || typeof body !== 'string' || title.length > 256 || body.length > 4096) return false;
        const win = getWin();
        // Només molestem si la finestra no té el focus: dins de l'app ja
        // es veu el que passa.
        if (win && win.isFocused()) return false;
        if (!Notification.isSupported()) return false;
        new Notification({ title: title || "Gregal", body: body || "" }).show();
        return true;
    });
}

async function createWindow() {
    const state = loadState(app.getPath("userData"));
    const inici = temaInicial();
    const win = new BrowserWindow({
        width: state.width,
        height: state.height,
        x: state.x,
        y: state.y,
        minWidth: 900,
        minHeight: 600,
        backgroundColor: inici.bg, // fons (docs/identitat.md)
        title: "Gregal",
        icon: __dirname + "/build/icon512.png",
        autoHideMenuBar: true,
        show: false,
        // Barra superior pròpia: la capçalera de la pàgina fa de barra de
        // títol (arrossegable) i els botons minimitza/maximitza/tanca els
        // dibuixa el sistema amb els nostres colors, dins la mateixa franja.
        // Abans hi havia la barra de Windows a sobre, d'un altre color.
        titleBarStyle: "hidden",
        // Valors inicials pel tema del SISTEMA, que és el que es veu mentre
        // la pàgina encara no ha carregat. Quan carrega, ella els corregeix
        // amb gregal:titlebar segons el tema que tinguis desat.
        titleBarOverlay: { color: inici.barra, symbolColor: inici.glif, height: TITLEBAR_H },
        webPreferences: {
            preload: path.join(__dirname, "preload.js"),
            contextIsolation: true,
            nodeIntegration: false,
            sandbox: true,
            webviewTag: true,
        },
    });
    wireIPC(() => win);
    win.webContents.on('will-attach-webview', (event, preferences, params) => {
        if (params.src && !externalURL(params.src) && params.src !== 'about:blank') {
            event.preventDefault();
            return;
        }
        delete preferences.preload;
        preferences.nodeIntegration = false;
        preferences.nodeIntegrationInSubFrames = false;
        preferences.contextIsolation = true;
        preferences.sandbox = true;
    });
    win.webContents.on('did-attach-webview', (_event, guest) => {
        guest.setWindowOpenHandler(() => ({ action: 'deny' }));
        guest.on('will-navigate', (event, url) => {
            if (!externalURL(url) && url !== 'about:blank') event.preventDefault();
        });
    });
    let saveTimer = null;
    const remember = () => {
        clearTimeout(saveTimer);
        saveTimer = setTimeout(() => saveState(app.getPath("userData"), win.getNormalBounds()), 400);
    };
    win.on("resize", remember);
    win.on("move", remember);
    win.on("close", () => saveState(app.getPath("userData"), win.getNormalBounds()));
    // Evita el flaix blanc mentre el backend local arrenca.
    win.once("ready-to-show", () => win.show());
    // Els enllaços externs, al navegador de debò.
    win.webContents.setWindowOpenHandler(({ url }) => {
        if (externalURL(url) && !sameOrigin(url, BASE)) shell.openExternal(url).catch(() => {});
        return { action: "deny" };
    });
    win.webContents.on('will-navigate', (event, url) => {
        if (sameOrigin(url, BASE)) return;
        event.preventDefault();
        if (externalURL(url)) shell.openExternal(url).catch(() => {});
    });
    const isEn = idiomaApp() === "en";
    const L = {
        edita: isEn ? "Edit" : "Edita",
        desfes: isEn ? "Undo" : "Desfés",
        refes: isEn ? "Redo" : "Refés",
        talla: isEn ? "Cut" : "Talla",
        copia: isEn ? "Copy" : "Copia",
        enganxa: isEn ? "Paste" : "Enganxa",
        tot: isEn ? "Select All" : "Selecciona-ho tot",
        obre: isEn ? "Open Project…" : "Obre projecte…",
        sessio: isEn ? "New Session" : "Sessió nova",
        canvis: isEn ? "Changes" : "Canvis",
        fitxers: isEn ? "Files" : "Fitxers",
        recarrega: isEn ? "Reload" : "Recarrega",
        eines: isEn ? "Developer Tools" : "Eines de desenvolupament",
        surt: isEn ? "Quit" : "Surt",
        errTitol: isEn ? "Could not start local backend" : "No s'ha pogut iniciar el backend local",
        errDetail: isEn ? "Port: " : "Port: ",
        errProv: isEn ? " · try GREGAL_URL to connect to an existing server." : " · prova GREGAL_URL per connectar a un servidor existent.",
        errBoto: isEn ? "OK" : "D'acord",
    };
    win.webContents.on("context-menu", (_ev, p) => {
        const items = [];
        if (p.isEditable) {
            items.push({ label: L.desfes, role: "undo" }, { label: L.refes, role: "redo" }, { type: "separator" });
        }
        if (p.selectionText) items.push({ label: L.copia, role: "copy" });
        if (p.isEditable && p.editFlags.canPaste) items.push({ label: L.enganxa, role: "paste" });
        if (p.isEditable) items.push({ label: L.tot, role: "selectAll" });
        if (items.length) Menu.buildFromTemplate(items).popup({ window: win });
    });
    const menu = Menu.buildFromTemplate([
        {
            label: L.edita,
            submenu: [
                { label: L.desfes, role: "undo" },
                { label: L.refes, role: "redo" },
                { type: "separator" },
                { label: L.talla, role: "cut" },
                { label: L.copia, role: "copy" },
                { label: L.enganxa, role: "paste" },
                { label: L.tot, role: "selectAll" },
            ],
        },
        {
            label: "Gregal",
            submenu: [
                {
                    label: L.obre,
                    accelerator: "CmdOrCtrl+O",
                    click: () => win.webContents.send("gregal:command", "open-project"),
                },
                {
                    label: L.sessio,
                    accelerator: "CmdOrCtrl+T",
                    click: () => win.webContents.send("gregal:command", "new-session"),
                },
                {
                    label: L.canvis,
                    accelerator: "CmdOrCtrl+D",
                    click: () => win.webContents.send("gregal:command", "changes"),
                },
                {
                    label: L.fitxers,
                    accelerator: "CmdOrCtrl+E",
                    click: () => win.webContents.send("gregal:command", "files"),
                },
                { type: "separator" },
                { label: L.recarrega, accelerator: "CmdOrCtrl+R", click: () => win.reload() },
                {
                    label: L.eines,
                    accelerator: "CmdOrCtrl+Shift+I",
                    click: () => win.webContents.toggleDevTools(),
                },
                { type: "separator" },
                { label: L.surt, accelerator: "CmdOrCtrl+Q", click: () => app.quit() },
            ],
        },
    ]);
    Menu.setApplicationMenu(menu);
    // Smoke test (només amb GREGAL_CAPTURE): captura la finestra i surt.
    // Ús: GREGAL_CAPTURE=/tmp/x.png xvfb-run npm start
    // (listener ABANS de carregar: si no, l'event ja ha passat).
    if (process.env.GREGAL_CAPTURE) {
        const out = process.env.GREGAL_CAPTURE;
        win.webContents.on("did-finish-load", () => {
            setTimeout(async () => {
                try {
                    const img = await win.capturePage();
                    require("fs").writeFileSync(out, img.toPNG());
                    console.log("CAPTURE_OK " + out);
                } catch (e) {
                    console.error("CAPTURE_FAIL " + e.message);
                }
                app.quit();
            }, 4000);
        });
        setTimeout(() => {
            console.error("CAPTURE_TIMEOUT");
            process.exitCode = 2;
            app.quit();
        }, 90000);
    }
    setupUpdater(win);
    if (await ensureServer()) {
        await win.loadURL(BASE + "/");
    } else {
        const tail = lastBackendLogs.map(l => String(l).trimEnd()).filter(Boolean).slice(-12).join("\n");
        if (tail || backendEarlyExit !== null) {
            dialog.showMessageBoxSync(win, {
                type: "error",
                message: L.errTitol + (backendEarlyExit !== null ? (isEn ? " (exited with code " : " (ha sortit amb codi ") + backendEarlyExit + ")" : ""),
                detail: (tail || (isEn ? "No logs" : "sense logs")) + "\n\n" + L.errDetail + PORT + L.errProv,
                buttons: [L.errBoto],
            });
        }
        await win.loadFile(__dirname + "/error.html", { query: { lang: idiomaApp() } });
    }
}

// Auto-update: si electron-updater hi és (builds de release), mira si hi
// ha versió nova i avisa. Sense el paquet, l'app funciona igual: no és
// una dependència dura per a `npm start`.
function setupUpdater(win) {
    if (!app.isPackaged || process.env.GREGAL_NO_UPDATE) return;
    // Portable builds have no updater manifest; users replace the EXE.
    if (!fs.existsSync(path.join(process.resourcesPath, 'app-update.yml'))) return;
    let updater;
    try {
        updater = require("electron-updater").autoUpdater;
    } catch (error) {
        return;
    }
    updater.autoDownload = true;
    const isEn = idiomaApp() === "en";
    updater.on("update-downloaded", info => {
        win.webContents.send("gregal:update", { state: "ready", version: info?.version || "" });
        dialog.showMessageBox(win, {
            type: "info",
            message: (isEn ? "A new version of Gregal is available" : "Hi ha una versió nova del Gregal") + (info?.version ? " (" + info.version + ")" : ""),
            detail: isEn ? "It will be installed when restarting the application." : "S'instal·larà en reiniciar l'aplicació.",
            buttons: isEn ? ["Restart now", "Later"] : ["Reinicia ara", "Més tard"],
            defaultId: 0,
        }).then(res => {
            if (res.response === 0) updater.quitAndInstall();
        });
    });
    updater.on("error", error => console.error("[gregal] update: " + error.message));
    updater.checkForUpdates().catch(() => {});
}

app.whenReady().then(createWindow);
app.on("window-all-closed", () => {
    if (process.platform !== "darwin") app.quit();
});
app.on("before-quit", () => {
    if (backend && !backend.killed) backend.kill("SIGTERM");
});
app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
});
