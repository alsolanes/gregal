// Pont mínim de la mascota flotant (mascot.html): rep l'estat que li passa
// el procés principal, pot demanar tornar a la finestra principal i
// moure's, plegar-se, enganxar-se a un racó o amagar-se. Res més: sense
// node, amb contextIsolation i sandbox. El procés principal torna a validar
// cada missatge (mascot-window.js).
const { contextBridge, ipcRenderer } = require('electron');

const text = v => String(v == null ? '' : v).slice(0, 200);

contextBridge.exposeInMainWorld('gregalMascot', {
    onState: cb => {
        const listener = (_e, data) => {
            const s = data && typeof data.strings === 'object' && data.strings ? data.strings : {};
            cb({
                mood: String(data && data.mood || 'idle'),
                label: String(data && data.label || ''),
                collapsed: !!(data && data.collapsed === true),
                strings: { collapse: text(s.collapse), expand: text(s.expand), snap: text(s.snap), hide: text(s.hide), drag: text(s.drag) },
            });
        };
        ipcRenderer.on('gregal:mascot-state', listener);
        return () => ipcRenderer.removeListener('gregal:mascot-state', listener);
    },
    activate: () => ipcRenderer.send('gregal:mascot-activate'),
    // phase: 'start' | 'move' | 'end'; dx/dy: desplaçament del punter des
    // de l'inici, en píxels de pantalla.
    drag: (phase, dx, dy) => ipcRenderer.send('gregal:mascot-drag', String(phase), Math.round(Number(dx) || 0), Math.round(Number(dy) || 0)),
    setCollapsed: on => ipcRenderer.send('gregal:mascot-collapse', on === true),
    snap: () => ipcRenderer.send('gregal:mascot-snap'),
    hide: () => ipcRenderer.send('gregal:mascot-hide'),
});
