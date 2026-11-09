// Finestra nativa del Gregal. En local gestiona el backend automàticament;
// GREGAL_URL continua permetent connectar-se a un servidor extern.
const { app, BrowserWindow, Menu, Notification, dialog, globalShortcut, ipcMain, nativeTheme, screen, shell, session, webContents: electronWebContents } = require("electron");
const { QUICK_NOTE_ACCELERATOR, showQuickNote, registerQuickNote } = require("./quick-note");

// TITLEBAR_H ha de casar amb l'alçada de #sessionTabs a index.html: és
// aquella fila la que fa de barra de títol.
const TITLEBAR_H = 38;

const http = require("http");
const https = require("https");
const net = require("net");
const fs = require("fs");
const path = require("path");
const { startServer, stopServer, appBackendEnv } = require("./server-manager");
const { loadState, saveState, fitBounds } = require("./window-state");
const { allowed, loadPreferences, savePreferences, keys: preferenceKeys } = require("./preferences");
const { sameOrigin, externalURL, backendURLError, localServiceURL, trustedPage } = require('./navigation');
const { createUpdater } = require('./updater');
const { createBrowserBridge, validGuest, guestWindowOpen, AGENT_BROWSER_PARTITION } = require('./browser-bridge');
const { createMascotWindow, PREF_KEY: MASCOT_PREF } = require('./mascot-window');
const chromeExtension = require('./chrome-extension');
const wordAddin = require('./word-addin');
const { loadStartupTheme, saveStartupTheme, themeKey, pageQuery, waitFor } = require('./startup');

let BASE = (process.env.GREGAL_URL || "http://127.0.0.1:8097").replace(/\/$/, "");
const baseError = backendURLError(BASE);
if (baseError) throw new Error(baseError);
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
let updateController = null;
let updateCheckStarted = false;
// Pont del navegador de l'agent (browser-bridge.js): només amb backend
// gestionat, que és l'únic a qui podem passar el token per l'entorn.
let browserBridge = null;
let mainWin = null;
// Mascota flotant (mascot-window.js): només amb la preferència activada.
let mascot = null;
// Preferències de la UI desades a userData (preferences.js). El tema i
// l'idioma fan falta abans que hi hagi cap pàgina carregada.
let preferences = null;
function prefsNow() {
    if (!preferences) preferences = loadPreferences(app.getPath('userData'));
    return preferences;
}
// Aturada volguda del backend (sortir de l'app o instal·lar una versió nova):
// llavors que s'aturi no és cap error que calgui ensenyar.
let backendStopping = false;
// Producte que ha demanat una segona instància («Gregal Vents») mentre l'app
// encara arrencava: es carrega aquest en lloc del de la línia d'ordres.
let requestedPath = null;
let ipcWired = false;
// Últims colors desats a startup-theme.json, per no reescriure'l igual.
let lastThemeSaved = '';
// Es resol quan la finestra principal ja és a la pantalla (createWindow).
let windowShown = Promise.resolve();
// Per què es mostra error.html: 'start' (no ha arrencat), 'stopped' (s'ha
// aturat amb l'app oberta) o 'remote' (GREGAL_URL no respon).
let failureReason = 'start';

function focusMain() {
    const win = mainWin;
    if (!win || win.isDestroyed()) return;
    if (win.isMinimized()) win.restore();
    win.show();
    win.focus();
}

function mascotController() {
    if (!mascot) mascot = createMascotWindow({
        BrowserWindow, screen,
        dir: app.getPath('userData'),
        htmlPath: path.join(__dirname, 'mascot.html'),
        preloadPath: path.join(__dirname, 'mascot-preload.js'),
        onActivate: focusMain,
        lang: idiomaApp,
    });
    return mascot;
}

function notifyAgentBrowser(state, extra) {
    const win = mainWin;
    if (!win || win.isDestroyed()) return;
    win.webContents.send('gregal:agent-browser', extra ? { ...extra, state } : state);
    // El «passa-m'ho» et necessita: si la finestra és amagada, s'avisa.
    const name = state && typeof state === 'object' ? state.state : state;
    if (name === 'handoff' && !win.isFocused()) win.flashFrame(true);
}

async function browserBridgeEnv() {
    if (!MANAGED) return {};
    try {
        if (!browserBridge) {
            browserBridge = createBrowserBridge({ notify: notifyAgentBrowser, blockedOrigin: () => BASE });
            await browserBridge.start();
        }
        return { GREGAL_APP_BROWSER: browserBridge.envValue() };
    } catch (error) {
        // Sense pont el backend fa servir Chrome headless: no és fatal.
        console.error("[gregal] pont del navegador: " + error.message);
        browserBridge = null;
        return {};
    }
}

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

function freePort(preferred = 0) {
    return new Promise((resolve, reject) => {
        const probe = net.createServer();
        probe.unref();
        probe.on("error", (error) => {
            // El port de sempre ja és d'algú: qualsevol altre de lliure.
            if (preferred) freePort(0).then(resolve, reject);
            else reject(error);
        });
        probe.listen(preferred, "127.0.0.1", () => {
            const port = probe.address().port;
            probe.close(() => resolve(port));
        });
    });
}

// Idioma dels menús i de les pàgines locals: el que hagis triat a
// Preferències (el mateix que fa servir la pàgina) i, si no, el del config.
function idiomaApp() {
    try {
        const triat = prefsNow().gregal_idioma;
        if (triat === 'ca' || triat === 'en') return triat;
    } catch (e) {}
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

// Espera /api/health: cada 50 ms els dos primers segons i després cada 200
// (startup.js). Si el backend mor abans de respondre, no té sentit esperar
// els 20 s amb un missatge genèric: s'atura de seguida i es mostren els logs.
function waitForServer(timeoutMs = 20000) {
    return waitFor(ping, {
        timeoutMs,
        gaveUp: () => backendEarlyExit !== null,
        onSlow: ms => console.log("[gregal] esperant el backend… (" + Math.round(ms / 1000) + "s)"),
    });
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
                // 8097 si és lliure: així l'extensió de Chrome troba l'app
                // sense configurar res (extension/chrome).
                PORT = await freePort(8097);
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
        const backendLog = backendLogPath();
        // spawn és síncron i, la primera vegada que s'executa un gregal.exe
        // nou (acabat d'instal·lar o d'actualitzar), l'antivirus l'escaneja
        // amb el fil principal aturat: 3-4 s sense poder ni pintar
        // loading.html. Primer la finestra (com a molt 1 s d'espera).
        await Promise.race([windowShown, new Promise(resolve => setTimeout(resolve, 1000))]);
        backendStopping = false;
        const started = startServer({
            resourcesPath: process.resourcesPath,
            appDir: __dirname,
            platform: process.platform,
            port: PORT,
            token: TOKEN,
            config: process.env.GREGAL_CONFIG,
            dir: process.env.GREGAL_DIR || app.getPath("home"),
            logFile: backendLog,
            env: appBackendEnv(await browserBridgeEnv()),
            onLog: line => console.log("[gregal] " + line.trimEnd()),
            onExit: (code, logs) => {
                backendEarlyExit = code;
                lastBackendLogs = (logs || []).slice(-20);
            },
        });
        const child = started.child;
        backend = child;
        lastBackendLogs = started.logs || [];
        child.on("error", error => console.error("[gregal] " + error.message));
        child.on("exit", code => {
            console.log("[gregal] backend aturat (" + code + ")");
            if (backend !== child) return;
            backend = null;
            // Mort amb l'app oberta: la pàgina ja no pot fer res. Millor la
            // pantalla d'error, que diu què ha passat i el torna a engegar.
            const win = mainWin;
            if (!backendStopping && win && !win.isDestroyed() && sameOrigin(win.webContents.getURL(), BASE)) {
                void showStartupError(win, 'stopped');
            }
        });
        const ok = await waitForServer(20000);
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

function backendLogPath() {
    try {
        return path.join(app.getPath("userData"), "gregal-backend.log");
    } catch (e) {
        return null; // sense userData no hi ha fitxer, però l'app arrenca
    }
}

// Colors per a la finestra i les pàgines locals abans que l'app pinti: els
// que va pintar l'última vegada amb el mateix tema (startup.js).
function systemDark() {
    try { return nativeTheme.shouldUseDarkColors; } catch (e) { return true; }
}
function temaInicial() {
    return loadStartupTheme(app.getPath("userData"), { tema: prefsNow().gregal_tema, systemDark: systemDark() });
}

// La pàgina ha pintat la barra amb els colors del seu tema: es desen (amb el
// fons i l'accent) perquè la pròxima arrencada surti ja d'aquest color.
async function rememberTheme(win, bar, ink) {
    try {
        const [bg, accent] = await win.webContents.executeJavaScript(
            "(() => { const s = getComputedStyle(document.documentElement); return [s.getPropertyValue('--bg').trim(), s.getPropertyValue('--accent').trim()]; })()");
        const key = themeKey(prefsNow().gregal_tema, systemDark());
        const sig = JSON.stringify([key, bg, bar, ink, accent]);
        if (sig === lastThemeSaved) return;
        if (saveStartupTheme(app.getPath("userData"), key, { bg, bar, ink, accent })) lastThemeSaved = sig;
    } catch (e) { /* pàgina tancada o sense colors: la pròxima vegada, els de la marca */ }
}

const pageOptions = extra => ({ query: { ...pageQuery(temaInicial(), idiomaApp(), PRODUCT), ...extra } });

// Pantalla d'arrencada: es veu de seguida, del color del tema, mentre el
// backend arrenca. El que arriba després (l'app o error.html) la substitueix.
function showLoading(win) {
    return win.loadFile(path.join(__dirname, "loading.html"), pageOptions()).catch(() => {});
}

// error.html amb el motiu; els detalls (codi, logs) els demana per IPC.
function showStartupError(win, reason) {
    if (!win || win.isDestroyed()) return Promise.resolve();
    failureReason = reason;
    return win.loadFile(path.join(__dirname, "error.html"), pageOptions({ reason })).catch(() => {});
}

// Directori de treball d'una sessió segons el backend: només una carpeta
// absoluta que existeixi. Si el backend no respon, res.
async function workspaceOf(sessionId) {
    try {
        const headers = { 'X-Gregal-Session': sessionId };
        if (TOKEN) headers.Authorization = 'Bearer ' + TOKEN;
        const ctrl = new AbortController();
        const timer = setTimeout(() => ctrl.abort(), 5000);
        let r;
        try { r = await fetch(BASE + '/api/state', { headers, signal: ctrl.signal }); } finally { clearTimeout(timer); }
        if (!r.ok) return '';
        const cwd = (await r.json()).cwd;
        if (typeof cwd !== 'string' || !cwd || !path.isAbsolute(cwd)) return '';
        return require('fs').statSync(cwd).isDirectory() ? cwd : '';
    } catch (_) {
        return '';
    }
}

// --- IPC: el que la pàgina pot demanar a l'escriptori (via preload) ---
// Un sol cop per procés i sempre contra la finestra viva: a macOS, tancar la
// finestra i tornar-la a obrir (activate) registrava els handlers dues
// vegades i ipcMain.handle hi petava.
function wireIPC(getWin = () => mainWin) {
    if (ipcWired) return;
    ipcWired = true;
    const prefDir = app.getPath('userData');
    const preferences = prefsNow();
    const trusted = event => event.sender === getWin()?.webContents && event.senderFrame === getWin()?.webContents.mainFrame && trustedPage(event.senderFrame.url, BASE, path.join(__dirname, 'error.html'));
    ipcMain.on('gregal:preferences', event => {
        event.returnValue = trusted(event) ? { values: preferences, keys: preferenceKeys } : { values: {}, keys: [] };
    });
    ipcMain.on('gregal:connection', event => {
        event.returnValue = trusted(event) ? { token: TOKEN, edition: EDITION } : { token: '', edition: EDITION };
    });
    // La pàgina avisa que el webview de l'agent és a punt. Només s'accepta
    // un guest webview d'aquesta finestra i de la partició pròpia.
    ipcMain.on('gregal:agent-browser-ready', (event, id, scope = '') => {
        if (!trusted(event) || !browserBridge || !Number.isInteger(id) || typeof scope !== 'string' || (scope && !/^[a-zA-Z0-9_.-]{1,128}$/.test(scope))) return;
        let guest = null;
        try { guest = electronWebContents.fromId(id); } catch (e) { return; }
        if (!validGuest(guest, getWin()?.webContents, session.fromPartition(AGENT_BROWSER_PARTITION))) return;
        browserBridge.setTarget(guest, scope);
    });
    // L'usuari respon el «passa-m'ho» del panell: «Fet» o «Cancel·la».
    ipcMain.on('gregal:agent-browser-handoff', (event, id, done) => {
        if (!trusted(event) || !browserBridge || typeof id !== 'string' || !/^[a-f0-9]{16}$/.test(id)) return;
        browserBridge.resolveHandoff(id, done === true);
    });
    ipcMain.on('gregal:preference-save', (event, key, value) => {
        if (!trusted(event) || !allowed(key) || (value !== null && (typeof value !== 'string' || value.length > 4096))) return;
        if (value === null) delete preferences[key];
        else preferences[key] = value;
        try { savePreferences(prefDir, preferences); } catch (error) { console.error('[gregal] preferències: ' + error.message); }
        // Apagar la preferència tanca la finestra; encendre-la l'obre.
        if (key === MASCOT_PREF) mascotController().setEnabled(value === '1');
    });
    // Ànim de la mascota: només de la pàgina de confiança. El controlador
    // el guarda i el reenvia si la finestra flotant és oberta.
    ipcMain.on('gregal:mascot-state', (event, mood, label) => {
        if (!trusted(event) || typeof mood !== 'string') return;
        mascotController().setState(mood, label);
    });
    // Clic a la mascota: només de la seva pròpia finestra.
    ipcMain.on('gregal:mascot-activate', event => {
        if (!mascot || !mascot.isSender(event)) return;
        mascot.activate();
    });
    // Arrossegar, plegar i enganxar a un racó: també només de la seva
    // finestra; mascot-window.js valida els desplaçaments i limita la
    // posició a l'àrea útil de la pantalla.
    ipcMain.on('gregal:mascot-drag', (event, phase, dx, dy) => {
        if (!mascot || !mascot.isSender(event) || typeof phase !== 'string') return;
        mascot.drag(phase, dx, dy);
    });
    ipcMain.on('gregal:mascot-collapse', (event, on) => {
        if (!mascot || !mascot.isSender(event) || typeof on !== 'boolean') return;
        mascot.setCollapsed(on);
    });
    ipcMain.on('gregal:mascot-snap', event => {
        if (!mascot || !mascot.isSender(event)) return;
        mascot.snap();
    });
    // Amagar-la des d'ella mateixa apaga la preferència (es torna a
    // encendre a Preferències) i avisa la pàgina perquè el commutador no
    // quedi encès.
    ipcMain.on('gregal:mascot-hide', event => {
        if (!mascot || !mascot.isSender(event)) return;
        preferences[MASCOT_PREF] = '0';
        try { savePreferences(prefDir, preferences); } catch (error) { console.error('[gregal] preferències: ' + error.message); }
        mascot.hide();
        const win = getWin();
        if (win && !win.isDestroyed()) win.webContents.send('gregal:preference-changed', MASCOT_PREF, '0');
    });
    // Obrir la carpeta del projecte a l'explorador del sistema. La pàgina
    // només diu de quina sessió és; la ruta la demanem nosaltres al backend
    // (/api/state amb el token), així no es pot obrir cap altra carpeta.
    ipcMain.handle('gregal:open-workspace', async (event, sessionId) => {
        if (!trusted(event)) return false;
        const id = typeof sessionId === 'string' && /^[A-Za-z0-9._-]{1,64}$/.test(sessionId) ? sessionId : 'default';
        const cwd = await workspaceOf(id);
        if (!cwd) return false;
        const err = await shell.openPath(cwd);
        return !err;
    });
    // Extensió de Chrome: la deixa a punt en una carpeta fixa, obre la pàgina
    // d'extensions del navegador i ensenya la carpeta (instal·lar-la és cosa
    // de l'usuari: Chrome no deixa fer-ho a cap programa).
    ipcMain.handle('gregal:chrome-extension', async event => {
        if (!trusted(event)) return null;
        const dir = prepareChromeExtension();
        if (!dir) return { ok: false };
        const browser = chromeExtension.openExtensionsPage();
        shell.showItemInFolder(path.join(dir, 'manifest.json'));
        return { ok: true, path: dir, browser };
    });
    // Complement de Word: instal·la'l d'un clic (registre WEF\Developer amb
    // el manifest del backend i el seu port), treu-lo o digues si hi és.
    ipcMain.handle('gregal:word-addin', async (event, action) => {
        if (!trusted(event)) return null;
        if (process.platform !== 'win32') return { ok: false, reason: 'platform' };
        try {
            if (action === 'remove') {
                const id = wordAddinId();
                return { ok: !!id && await wordAddin.unregister(id), installed: false };
            }
            if (action === 'install') {
                const r = await installWordAddin();
                return { ok: !!r, installed: !!r, path: r || '' };
            }
            const id = wordAddinId();
            const at = id ? await wordAddin.registeredPath(id) : '';
            return { ok: true, installed: !!at, path: at };
        } catch (e) {
            console.error('[gregal] complement de Word: ' + e.message);
            return { ok: false, reason: 'error' };
        }
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
        if (ok && win && !win.isDestroyed()) {
            void refreshWordAddin();
            win.loadURL(BASE + appPath()).catch(() => {});
            return { ok: true };
        }
        return { ok: false };
    });
    // Per què no arrenca, per a error.html: el codi de sortida, el port i les
    // últimes línies del log (cada una tallada; res d'enviar el fitxer sencer).
    ipcMain.handle("gregal:startup-error", event => {
        if (!trusted(event)) return null;
        const logs = lastBackendLogs.flatMap(l => String(l).split(/\r?\n/)).map(l => l.trimEnd().slice(0, 400)).filter(Boolean).slice(-30);
        const logFile = backendLogPath();
        return {
            reason: failureReason,
            code: Number.isInteger(backendEarlyExit) ? backendEarlyExit : null,
            port: MANAGED ? PORT : null,
            server: MANAGED ? '' : BASE,
            logs,
            logFile: !!(MANAGED && logFile && fs.existsSync(logFile)),
        };
    });
    ipcMain.handle("gregal:open-backend-log", event => {
        if (!trusted(event)) return false;
        const logFile = backendLogPath();
        if (!logFile || !fs.existsSync(logFile)) return false;
        shell.showItemInFolder(logFile);
        return true;
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
            void rememberTheme(win, color, symbolColor);
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
    const updateAction = (name, action) => ipcMain.handle(name, event => trusted(event) ? action() : null);
    updateAction('gregal:update-status', () => updateController?.getStatus() || null);
    updateAction('gregal:update-check', () => updateController?.check() || null);
    updateAction('gregal:update-download', () => updateController?.download() || null);
    updateAction('gregal:update-install', () => updateController?.install() || null);
    updateAction('gregal:update-page', () => updateController?.openPage() || null);
}

// Dos productes al mateix servidor: Gregal Codi (/) i Gregal Vents (/vents/).
// La drecera «Gregal Vents» obre l'app amb --vents; la finestra és la mateixa
// i el desplegable del peu passa d'un producte a l'altre.
// Dues edicions, dos instal·ladors: «Gregal» (Codi i Vents) i «Gregal Jocs»
// (només els jocs). L'edició la posa el build a package.json (gregalEdition);
// sense, és Gregal. Gregal ja no obre els jocs, i Gregal Jocs només obre /jocs/.
const EDITION = (() => { try { return require("./package.json").gregalEdition === "jocs" ? "jocs" : "gregal"; } catch (e) { return "gregal"; } })();
const PRODUCT = EDITION === "jocs" ? "Gregal Jocs" : "Gregal";
const wantsVents = argv => EDITION === "gregal" && (argv || []).includes("--vents");
const pathFor = argv => EDITION === "jocs" ? "/jocs/" : wantsVents(argv) ? "/vents/" : "/";
const startPath = () => pathFor(process.argv);
const appPath = () => requestedPath || startPath();

// La finestra principal no pot ser més gran que la pantalla on surt: en un
// portàtil de 1366×768 al 125 % l'àrea útil fa menys de 600 d'alt.
const MIN_SIZE = { width: 900, height: 600 };
function minSizeFor(workArea) {
    return { width: Math.min(MIN_SIZE.width, workArea.width), height: Math.min(MIN_SIZE.height, workArea.height) };
}

async function createWindow() {
    const saved = loadState(app.getPath("userData"));
    // Dins la pantalla on era (o la principal), sencera i sense passar per sota de la barra de tasques.
    const display = Number.isFinite(saved.x) && Number.isFinite(saved.y)
        ? screen.getDisplayMatching({ x: saved.x, y: saved.y, width: saved.width, height: saved.height })
        : screen.getPrimaryDisplay();
    const state = fitBounds(saved, display.workArea);
    const min = minSizeFor(display.workArea);
    const inici = temaInicial();
    const win = new BrowserWindow({
        width: state.width,
        height: state.height,
        x: state.x,
        y: state.y,
        minWidth: min.width,
        minHeight: min.height,
        // Fons, barra i botons del sistema amb els colors que l'app va
        // pintar l'última vegada amb aquest tema (startup.js): ni flaix blanc
        // ni un fosc d'arrencada per a qui va en clar.
        backgroundColor: inici.bg,
        title: PRODUCT,
        icon: path.join(__dirname, "build", "icon512.png"),
        autoHideMenuBar: true,
        // Es mostra quan loading.html ha pintat (ready-to-show), que és
        // qüestió de mil·lisegons: no cal esperar el backend.
        show: false,
        // Barra superior pròpia: la capçalera de la pàgina fa de barra de
        // títol (arrossegable) i els botons minimitza/maximitza/tanca els
        // dibuixa el sistema amb els nostres colors, dins la mateixa franja.
        // Abans hi havia la barra de Windows a sobre, d'un altre color.
        titleBarStyle: "hidden",
        // Quan l'app carrega, ella els corregeix amb gregal:titlebar segons
        // el tema que tinguis desat (i es tornen a desar per a la pròxima).
        titleBarOverlay: { color: inici.bar, symbolColor: inici.ink, height: TITLEBAR_H },
        webPreferences: {
            preload: path.join(__dirname, "preload.js"),
            contextIsolation: true,
            nodeIntegration: false,
            sandbox: true,
            webviewTag: true,
        },
    });
    mainWin = win;
    win.once('closed', () => { if (mainWin === win) mainWin = null; browserBridge?.clearTarget(); mascot?.close(); });
    wireIPC();
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
        // Enllaços que obren finestra: al mateix panell, o una finestra
        // petita per als inicis de sessió amb popup (browser-bridge.js).
        guest.setWindowOpenHandler(details => {
            const how = guestWindowOpen(details, () => BASE);
            if (how === 'same') { guest.loadURL(details.url).catch(() => {}); return { action: 'deny' }; }
            if (how !== 'popup') return { action: 'deny' };
            return { action: 'allow', overrideBrowserWindowOptions: {
                parent: mainWin || undefined, width: 520, height: 700, autoHideMenuBar: true, backgroundColor: '#ffffff',
                webPreferences: { partition: AGENT_BROWSER_PARTITION, sandbox: true, contextIsolation: true, nodeIntegration: false },
            } };
        });
        guest.on('did-create-window', child => {
            child.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));
            child.webContents.on('will-navigate', (event, url) => { if (!externalURL(url)) event.preventDefault(); });
        });
        guest.on('will-navigate', (event, url) => {
            if (!externalURL(url) && url !== 'about:blank') event.preventDefault();
        });
    });
    // Mida i posició normals, i si era maximitzada: una finestra que tancaves
    // maximitzada tornava a sortir petita. Es desa la mida del contingut
    // (getNormalBounds menys el marc): a Windows, amb la barra amagada, la
    // finestra que surt del constructor és uns píxels més ampla del que es
    // demana i es feia 4 px més gran a cada arrencada; setContentBounds és
    // exacte.
    let saveTimer = null;
    let frame = { width: 0, height: 0 };
    const snapshot = () => {
        const n = win.getNormalBounds();
        return { x: n.x, y: n.y, width: n.width - frame.width, height: n.height - frame.height, maximized: win.isMaximized() };
    };
    const remember = () => {
        clearTimeout(saveTimer);
        saveTimer = setTimeout(() => { if (!win.isDestroyed()) saveState(app.getPath("userData"), snapshot()); }, 400);
    };
    for (const ev of ["resize", "move", "maximize", "unmaximize"]) win.on(ev, remember);
    win.on("close", () => { clearTimeout(saveTimer); saveState(app.getPath("userData"), snapshot()); });
    // Primer pintat de loading.html: la finestra surt ja del color del tema.
    // Si per alguna raó no arriba (GPU encallada), es mostra igualment.
    let revealed = false;
    let markShown = () => {};
    windowShown = new Promise(resolve => { markShown = resolve; });
    const reveal = () => {
        if (revealed || win.isDestroyed()) return;
        revealed = true;
        clearTimeout(revealTimer);
        // Després del show: el backend ja pot arrencar (ensureServer).
        setImmediate(markShown);
        try {
            win.setContentBounds(state);
            const b = win.getBounds(), c = win.getContentBounds();
            frame = { width: Math.max(0, b.width - c.width), height: Math.max(0, b.height - c.height) };
        } catch (e) { /* sense marc conegut: es desa getNormalBounds tal qual */ }
        if (saved.maximized) win.maximize();
        win.show();
        // Mascota flotant: apagada si no s'ha activat mai a Preferències.
        if (prefsNow()[MASCOT_PREF] === '1') mascotController().setEnabled(true);
    };
    const revealTimer = setTimeout(reveal, 3000);
    win.once("ready-to-show", reveal);
    // L'app no carrega (el backend ha caigut entre el health i la càrrega,
    // o un Ctrl+R amb el backend aturat): error.html, no la pàgina d'error
    // de Chromium. -3 és una navegació avortada per una altra.
    win.webContents.on("did-fail-load", (_e, code, _desc, url, isMainFrame) => {
        if (!isMainFrame || code === -3 || !sameOrigin(url, BASE)) return;
        void showStartupError(win, MANAGED ? (backend ? 'start' : 'stopped') : 'remote');
    });
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
        nota: isEn ? "Quick Note" : "Nota ràpida",
        fitxers: isEn ? "Files" : "Fitxers",
        recarrega: isEn ? "Reload" : "Recarrega",
        eines: isEn ? "Developer Tools" : "Eines de desenvolupament",
        quant: isEn ? "About " + PRODUCT : "Quant a " + PRODUCT,
        surt: isEn ? "Quit" : "Surt",
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
            label: PRODUCT,
            submenu: [
                // Les ordres de Codi i Vents no diuen res a l'edició Jocs.
                ...(EDITION === "jocs" ? [] : [
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
                    // La drecera és global (registerQuickNote): aquí només es mostra.
                    { label: L.nota, accelerator: QUICK_NOTE_ACCELERATOR, registerAccelerator: false, click: () => showQuickNote(win) },
                    { label: "Gregal Codi", click: () => win.loadURL(BASE + "/").catch(() => {}) },
                    { label: "Gregal Vents", click: () => win.loadURL(BASE + "/vents/").catch(() => {}) },
                    { type: "separator" },
                ]),
                { label: L.recarrega, accelerator: "CmdOrCtrl+R", click: () => win.reload() },
                {
                    label: L.eines,
                    accelerator: "CmdOrCtrl+Shift+I",
                    click: () => win.webContents.toggleDevTools(),
                },
                { type: "separator" },
                { label: L.quant, click: () => app.showAboutPanel() },
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
        const loadingURL = require("url").pathToFileURL(path.join(__dirname, "loading.html")).href;
        win.webContents.on("did-finish-load", () => {
            // La pantalla d'arrencada no és el que es vol capturar.
            if (win.webContents.getURL().split("?")[0] === loadingURL) return;
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
    // La finestra surt de seguida amb loading.html; el backend arrenca
    // mentrestant i, quan respon, l'app la substitueix sense flaix (Chromium
    // manté la pàgina anterior fins que la nova pinta).
    void showLoading(win);
    if (await ensureServer()) {
        if (win.isDestroyed()) return;
        void refreshWordAddin();
        await win.loadURL(BASE + appPath()).catch(() => {});
    } else {
        // Res de diàleg modal a sobre: error.html ja diu què ha passat,
        // ensenya els logs i té el botó per tornar-ho a provar.
        await showStartupError(win, MANAGED ? 'start' : 'remote');
    }
    startUpdateCheck();
}

// La comprovació d'actualitzacions surt quan l'app (o error.html) ja és a
// la pantalla: no competeix amb l'arrencada, i una versió nova pot ser
// justament el que arregla un backend que no arrenca.
function startUpdateCheck() {
    if (updateCheckStarted || !updateController) return;
    updateCheckStarted = true;
    void updateController.check();
}

// Auto-update: si electron-updater hi és (builds de release), mira si hi
// ha versió nova i avisa. Sense el paquet, l'app funciona igual: no és
// una dependència dura per a `npm start`.
function setupUpdater(win) {
    if (!updateController) updateController = createUpdater({
        app, platform: process.platform, env: process.env, resourcesPath: process.resourcesPath,
        openExternal: url => shell.openExternal(url),
        stopBackend: async () => {
            if (!MANAGED || !backend || backend.killed) return;
            const child = backend;
            backendStopping = true;
            await stopServer(child, 2500);
            if (backend === child) backend = null;
        },
    });
    const unsubscribe = updateController.onStatus(status => {
        if (!win.isDestroyed()) win.webContents.send('gregal:update', status);
    });
    win.once('closed', unsubscribe);
}

// Complement de Word: el manifest viu a userData/word-addin i el registre
// apunta allà. L'Id surt del manifest desat (o del backend si encara no n'hi ha).
const wordAddinDir = () => path.join(app.getPath('userData'), 'word-addin');
function wordAddinId() {
    try { return wordAddin.manifestId(fs.readFileSync(path.join(wordAddinDir(), wordAddin.FILE), 'utf8')); } catch (e) { return ''; }
}
// Demana el manifest al backend, hi posa l'origen real i el registra.
// Torna el camí del fitxer, o "" si no s'ha pogut.
async function installWordAddin() {
    const res = await fetch(BASE + '/app/addin/' + wordAddin.FILE);
    if (!res.ok) throw new Error('manifest HTTP ' + res.status);
    const xml = wordAddin.withOrigin(await res.text(), BASE);
    const id = wordAddin.manifestId(xml);
    if (!id) throw new Error('manifest sense Id');
    const file = wordAddin.writeManifest(xml, wordAddinDir());
    return (await wordAddin.register(id, file)) ? file : '';
}
// A cada arrencada, si ja estava instal·lat, torna a escriure el manifest amb
// el port d'ara (el 8097 pot estar ocupat i el backend en tria un altre).
async function refreshWordAddin() {
    if (process.platform !== 'win32' || !fs.existsSync(path.join(wordAddinDir(), wordAddin.FILE))) return;
    const id = wordAddinId();
    if (!id || !(await wordAddin.registeredPath(id))) return;
    try { await installWordAddin(); } catch (e) { console.error('[gregal] complement de Word: ' + e.message); }
}

// Carpeta fixa de l'extensió de Chrome; si ja s'ha preparat abans, cada
// arrencada la refresca perquè Chrome carregui la versió de l'app.
function prepareChromeExtension() {
    try {
        const src = chromeExtension.sourceDir({ packaged: app.isPackaged, resourcesPath: process.resourcesPath, appDir: __dirname });
        return chromeExtension.syncExtension(src, path.join(app.getPath('userData'), 'chrome-extension'));
    } catch (e) {
        console.error('[gregal] extensió de Chrome: ' + e.message);
        return '';
    }
}

// Una sola instància: obrir «Gregal Vents» amb l'app ja oberta no ha
// d'engegar un segon backend; porta la finestra que hi ha al producte demanat.
// La segona instància no ha de fer res més: abans el whenReady de sota
// corria igualment i creava una finestra (i un backend) mentre sortia.
const gotLock = typeof app.requestSingleInstanceLock !== "function" || app.requestSingleInstanceLock();
if (!gotLock) {
    app.quit();
} else {
    app.on("second-instance", (_event, argv) => {
        const win = mainWin;
        if (!win || win.isDestroyed()) {
            if (app.isReady()) void createWindow();
            return;
        }
        if (win.isMinimized()) win.restore();
        // Si encara no s'ha mostrat, ja ho farà reveal (amb la mida bona).
        if (win.isVisible()) {
            win.show();
            win.focus();
        }
        if (!wantsVents(argv)) return;
        // Encara arrencant (loading.html): es carregarà Vents quan toqui.
        if (sameOrigin(win.webContents.getURL(), BASE)) win.loadURL(BASE + pathFor(argv)).catch(() => {});
        else requestedPath = pathFor(argv);
    });
}

app.whenReady().then(async () => {
    if (!gotLock) return;
    app.setAboutPanelOptions({
        applicationName: PRODUCT,
        applicationVersion: app.getVersion(),
        website: "https://github.com/alsolanes/gregal",
        iconPath: path.join(__dirname, "build", "icon512.png"),
    });
    const created = createWindow();
    // Nota ràpida des de qualsevol app (Ctrl+Alt+N), menys a l'edició Jocs.
    // Sempre la finestra principal: abans, mentre arrencava, podia caure a
    // la finestreta de la mascota.
    if (EDITION !== "jocs") registerQuickNote(globalShortcut, () => mainWin);
    await created;
    // L'extensió de Chrome ja preparada es refresca quan l'app ja és a la
    // pantalla: copiar fitxers no ha de retardar la finestra.
    if (fs.existsSync(path.join(app.getPath('userData'), 'chrome-extension', 'manifest.json'))) prepareChromeExtension();
});
app.on("will-quit", () => globalShortcut.unregisterAll());
app.on("window-all-closed", () => {
    if (process.platform !== "darwin") app.quit();
});
// Sortir: el backend s'atura net (tanca la línia de vida i espera, com a
// molt 1,5 s) abans que l'app acabi; les finestres s'amaguen de seguida
// perquè sortir no faci esperar ningú.
let backendStopped = false;
app.on("before-quit", event => {
    backendStopping = true;
    browserBridge?.close();
    const child = backend;
    if (backendStopped || !child || child.exitCode !== null || child.signalCode !== null) return;
    event.preventDefault();
    backendStopped = true;
    for (const w of BrowserWindow.getAllWindows()) if (!w.isDestroyed()) w.hide();
    stopServer(child, 1500).finally(() => {
        if (backend === child) backend = null;
        app.quit();
    });
});
app.on("activate", () => {
    if (gotLock && !mainWin) void createWindow();
});
