// Finestres pròpies dels documents vius (/app/doc/{id} del backend): una per
// document. Les demana la pàgina de confiança amb gregal:open-doc (main.js);
// tornar a demanar el mateix document porta al davant la que ja hi ha.
const DOC_ID = /^[a-z0-9]{1,64}$/;
// Com sessIDRe del backend (internal/web/hub.go).
const SESSION = /^[A-Za-z0-9_-]{1,64}$/;

function createDocWindows({ BrowserWindow, base, preloadPath, openExternal }) {
    const open = new Map();
    return {
        open(id, session) {
            if (typeof id !== 'string' || !DOC_ID.test(id) || typeof session !== 'string' || !SESSION.test(session)) return false;
            const old = open.get(id);
            if (old && !old.isDestroyed()) {
                if (old.isMinimized()) old.restore();
                old.focus();
                return true;
            }
            const win = new BrowserWindow({
                width: 900,
                height: 1000,
                autoHideMenuBar: true,
                webPreferences: { preload: preloadPath, contextIsolation: true, nodeIntegration: false, sandbox: true },
            });
            open.set(id, win);
            win.once('closed', () => { if (open.get(id) === win) open.delete(id); });
            // Els enllaços del document, al navegador; la finestra no surt mai de la pàgina.
            win.webContents.setWindowOpenHandler(({ url }) => { openExternal(url); return { action: 'deny' }; });
            win.webContents.on('will-navigate', (event, url) => { event.preventDefault(); openExternal(url); });
            win.loadURL(base() + '/app/doc/' + id + '?session=' + session).catch(() => {});
            return true;
        },
        // Amb la finestra principal tancada, l'app ha de poder sortir.
        closeAll() {
            for (const w of open.values()) if (!w.isDestroyed()) w.close();
        },
        // El missatge ve del marc principal d'una d'aquestes finestres.
        isSender(event) {
            for (const w of open.values()) {
                if (!w.isDestroyed() && event.sender === w.webContents && event.senderFrame === w.webContents.mainFrame) return true;
            }
            return false;
        },
    };
}

module.exports = { createDocWindows };
