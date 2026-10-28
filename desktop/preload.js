// Pont segur entre la finestra i Electron. La pàgina es carrega des de
// 127.0.0.1 i no ha de tenir mai accés directe a node: tot passa per aquí,
// amb contextIsolation activat i una superfície mínima.
const { contextBridge, ipcRenderer } = require("electron");
const preferences = ipcRenderer.sendSync('gregal:preferences');
const connection = ipcRenderer.sendSync('gregal:connection');

contextBridge.exposeInMainWorld("gregalDesktop", {
    preferences: preferences.values,
    preferenceKeys: preferences.keys,
    apiToken: connection.token,
    savePreference: (key, value) => ipcRenderer.send('gregal:preference-save', key, value),
    // La versió d'Electron queda exposada només mitjançant IPC de confiança.
    getUpdateStatus: () => ipcRenderer.invoke('gregal:update-status'),
    checkForUpdates: () => ipcRenderer.invoke('gregal:update-check'),
    downloadUpdate: () => ipcRenderer.invoke('gregal:update-download'),
    installUpdate: () => ipcRenderer.invoke('gregal:update-install'),
    openUpdatePage: () => ipcRenderer.invoke('gregal:update-page'),
    // La finestra no té barra de títol del sistema: la capçalera de la
    // pàgina fa de barra (arrossegable) i deixa lloc als botons natius.
    frameless: true,
    platform: process.platform,
    // Diàleg natiu per triar el projecte de la sessió.
    chooseFolder: () => ipcRenderer.invoke("gregal:choose-folder"),
    setupChromeExtension: () => ipcRenderer.invoke('gregal:chrome-extension'),
    // Complement de Word: 'install', 'remove' o 'status'.
    wordAddin: action => ipcRenderer.invoke('gregal:word-addin', action),
    // Notificació nativa quan la finestra no té focus.
    notify: (title, body) => ipcRenderer.invoke("gregal:notify", { title, body }),
    // Reintent d'arrencada del backend des de la pantalla d'error
    retryBackend: () => ipcRenderer.invoke("gregal:retry-backend"),
    // Els botons de finestra els pinta el sistema i no saben res del CSS:
    // amb el color escrit a mà, en tema clar sortia un requadre fosc a
    // dalt a la dreta. La pàgina li passa els colors del tema actiu.
    setTitleBar: (color, symbolColor) => ipcRenderer.invoke("gregal:titlebar", { color, symbolColor }),
    // Estat de l'actualització automàtica (si n'hi ha).
    // Amb removeListener: cada reload acumulava handlers i un Ctrl+T obria
    // dues sessions. Ara es desregistra l'anterior abans de posar-ne un de nou.
    onUpdate: cb => {
        const listener = (_e, data) => cb(data);
        ipcRenderer.on('gregal:update', listener);
        return () => ipcRenderer.removeListener('gregal:update', listener);
    },
    // Navegador de l'agent (browser-bridge.js): el procés principal demana
    // mostrar el panell ('show') i diu qui mana ('active'/'released'); la
    // pàgina torna l'id del webview quan és a punt.
    onAgentBrowser: cb => {
        const listener = (_e, state) => cb(state && typeof state === 'object' ? state : String(state));
        ipcRenderer.on('gregal:agent-browser', listener);
        return () => ipcRenderer.removeListener('gregal:agent-browser', listener);
    },
    agentBrowserReady: (id, session = '') => ipcRenderer.send('gregal:agent-browser-ready', Number(id), String(session)),
    // Mascota flotant (mascot-window.js): la pàgina n'envia l'ànim i una
    // etiqueta traduïda; el procés principal valida i la reenvia.
    mascotWindow: true,
    mascotState: (mood, label) => ipcRenderer.send('gregal:mascot-state', String(mood), String(label || '').slice(0, 200)),
    // La mascota s'ha amagat des d'ella mateixa (o una altra preferència ha
    // canviat al procés principal): la pàgina actualitza el seu mirall.
    onPreferenceChanged: cb => {
        const listener = (_e, key, value) => cb(String(key), value == null ? null : String(value));
        ipcRenderer.on('gregal:preference-changed', listener);
        return () => ipcRenderer.removeListener('gregal:preference-changed', listener);
    },
    // Obre la carpeta del projecte de la sessió a l'explorador del sistema.
    // Només s'envia l'id de sessió: la ruta la resol el procés principal.
    openWorkspaceFolder: session => ipcRenderer.invoke('gregal:open-workspace', String(session || 'default').slice(0, 64)),
    // Dreceres del menú que la pàgina ha d'atendre.
    onCommand: cb => { ipcRenderer.removeAllListeners("gregal:command"); ipcRenderer.on("gregal:command", (_e, name) => cb(name)); },
});
