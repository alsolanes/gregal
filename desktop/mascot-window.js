// Penell flotant de l'escriptori (internament «mascota»): una finestreta sense marc, transparent i
// sempre a sobre que fa de mirall de l'estat de l'app (repòs, treballant,
// esperant, fet, error). La pàgina de confiança li envia l'estat (via
// preload.js → main.js) i aquest mòdul el reenvia a mascot.html. Només
// existeix si l'usuari l'activa a Preferències (gregal_desktop_mascot = '1').
// Un clic curt torna el focus a la finestra principal; mantenir premut i
// moure l'arrossega (ho fa mascot-view.js amb pointer events i aquí es mou
// la finestra, sempre dins de l'àrea útil de la pantalla). Es pot plegar en
// un puntet, enganxar a un racó o amagar (apaga la preferència). Recorda la
// posició i si és plegat en un JSON propi, amb les precaucions de
// window-state.js.
const fs = require('fs');
const path = require('path');
const { pathToFileURL } = require('url');

const PREF_KEY = 'gregal_desktop_mascot';
const MOODS = new Set(['idle', 'working', 'waiting', 'done', 'error']);
const SIZE = 120;
// Plegat: un puntet de 32 px dins d'una finestra de 40 (l'ombra hi cap).
const COLLAPSED_SIZE = 40;
const STATE_FILE = 'mascot-window.json';
const MARGIN = 24;
// En deixar-lo a menys d'això d'una vora, s'hi enganxa (a MARGIN).
const MAGNET = 32;
// Desplaçament màxim que s'accepta en un arrossegament (en píxels DIP).
const MAX_DRAG = 20000;

// Textos dels controls: la finestreta no carrega i18n.js.
const STRINGS = {
    ca: { collapse: 'Plega (Esc)', expand: 'Desplega', snap: 'Enganxa a un racó', hide: 'Amaga (es torna a activar a Preferències)', drag: 'Clica per obrir Gregal · mantén premut i mou per arrossegar' },
    en: { collapse: 'Minimize (Esc)', expand: 'Expand', snap: 'Snap to a corner', hide: 'Hide (turn it back on in Preferences)', drag: 'Click to open Gregal · hold and move to drag' },
};

function positionPath(dir) {
    return path.join(dir, STATE_FILE);
}

const sizeFor = collapsed => (collapsed ? COLLAPSED_SIZE : SIZE);

function primaryArea(screen) {
    try { return screen && screen.getPrimaryDisplay().workArea || null; } catch (_) { return null; }
}

// Àrea útil de la pantalla on cau el centre de la finestra (o la més
// propera); si no se'n sap res, la principal.
function workAreaFor(screen, x, y, size) {
    const cx = Math.round(x + size / 2), cy = Math.round(y + size / 2);
    try {
        if (screen && typeof screen.getDisplayNearestPoint === 'function') {
            const d = screen.getDisplayNearestPoint({ x: cx, y: cy });
            if (d && (d.workArea || d.bounds)) return d.workArea || d.bounds;
        }
    } catch (_) {}
    try {
        const displays = screen ? screen.getAllDisplays() : [];
        const hit = displays.find(d => {
            const a = d.workArea || d.bounds;
            return a && cx >= a.x && cx < a.x + a.width && cy >= a.y && cy < a.y + a.height;
        });
        if (hit) return hit.workArea || hit.bounds;
    } catch (_) {}
    return primaryArea(screen);
}

// Manté la finestra sencera dins de l'àrea.
function clampToArea(pos, size, area) {
    if (!area) return { x: Math.round(pos.x), y: Math.round(pos.y) };
    const maxX = area.x + Math.max(0, area.width - size);
    const maxY = area.y + Math.max(0, area.height - size);
    return {
        x: Math.round(Math.min(Math.max(pos.x, area.x), maxX)),
        y: Math.round(Math.min(Math.max(pos.y, area.y), maxY)),
    };
}

// En deixar-la prop d'una vora, s'hi enganxa amb el marge habitual.
function magnet(pos, size, area) {
    if (!area) return pos;
    let { x, y } = pos;
    const right = area.x + area.width - size - MARGIN, bottom = area.y + area.height - size - MARGIN;
    if (x - area.x < MAGNET) x = area.x + MARGIN;
    else if (area.x + area.width - (x + size) < MAGNET) x = right;
    if (y - area.y < MAGNET) y = area.y + MARGIN;
    else if (area.y + area.height - (y + size) < MAGNET) y = bottom;
    return clampToArea({ x, y }, size, area);
}

// Els quatre racons, en sentit horari: dalt-esquerra, dalt-dreta,
// baix-dreta, baix-esquerra.
function corners(size, area) {
    const l = area.x + MARGIN, t = area.y + MARGIN;
    const r = area.x + area.width - size - MARGIN, b = area.y + area.height - size - MARGIN;
    return [{ x: l, y: t }, { x: r, y: t }, { x: r, y: b }, { x: l, y: b }].map(p => clampToArea(p, size, area));
}

// Si ja és a un racó, passa al següent (horari); si no, al més proper.
function nextCorner(pos, size, area) {
    if (!area) return pos;
    const cs = corners(size, area);
    const at = cs.findIndex(c => Math.abs(c.x - pos.x) <= 2 && Math.abs(c.y - pos.y) <= 2);
    if (at >= 0) return cs[(at + 1) % cs.length];
    let best = cs[0], dist = Infinity;
    for (const c of cs) {
        const d = (c.x - pos.x) ** 2 + (c.y - pos.y) ** 2;
        if (d < dist) { dist = d; best = c; }
    }
    return best;
}

// Canviar de mida (plegar/desplegar) sense que salti: es manté la vora més
// propera de l'àrea (a la meitat dreta, la dreta; a baix, la de baix).
function resizeAnchored(pos, oldSize, newSize, area) {
    let x = pos.x, y = pos.y;
    if (area) {
        const cx = pos.x + oldSize / 2, cy = pos.y + oldSize / 2;
        if (cx > area.x + area.width / 2) x = pos.x + oldSize - newSize;
        if (cy > area.y + area.height / 2) y = pos.y + oldSize - newSize;
    }
    return clampToArea({ x, y }, newSize, area);
}

// La posició només val si cau dins d'alguna pantalla (un monitor
// desconnectat deixaria la mascota invisible); si no, a baix a la dreta de
// la pantalla principal. Torna també si era plegada.
function loadPosition(dir, screen) {
    let pos = null;
    let collapsed = false;
    try {
        const raw = JSON.parse(fs.readFileSync(positionPath(dir), 'utf8'));
        collapsed = raw.collapsed === true;
        if (Number.isFinite(raw.x) && Number.isFinite(raw.y) && raw.x > -2000 && raw.x < 10000 && raw.y > -2000 && raw.y < 10000) {
            pos = { x: Math.round(raw.x), y: Math.round(raw.y) };
        }
    } catch (_) { /* sense fitxer o corromput: posició per defecte */ }
    const size = sizeFor(collapsed);
    let displays = [];
    try { displays = screen ? screen.getAllDisplays() : []; } catch (_) { displays = []; }
    if (pos && displays.length) {
        const cx = pos.x + size / 2, cy = pos.y + size / 2;
        const hit = displays.find(d => {
            const a = d.workArea || d.bounds;
            return a && cx >= a.x && cx < a.x + a.width && cy >= a.y && cy < a.y + a.height;
        });
        pos = hit ? clampToArea(pos, size, hit.workArea || hit.bounds) : null;
    }
    const out = collapsed ? { collapsed: true } : {};
    if (pos) return { ...pos, ...out };
    const a = primaryArea(screen);
    if (a) return { x: a.x + a.width - size - MARGIN, y: a.y + a.height - size - MARGIN, ...out };
    return out;
}

function savePosition(dir, pos) {
    try {
        if (!pos || !Number.isFinite(pos.x) || !Number.isFinite(pos.y)) return false;
        fs.mkdirSync(dir, { recursive: true });
        const data = { x: Math.round(pos.x), y: Math.round(pos.y) };
        if (pos.collapsed === true) data.collapsed = true;
        fs.writeFileSync(positionPath(dir), JSON.stringify(data), 'utf8');
        return true;
    } catch (_) {
        return false;
    }
}

// Desplaçament d'un arrossegament: enters finits i raonables, o res.
function validDelta(dx, dy) {
    return Number.isInteger(dx) && Number.isInteger(dy) && Math.abs(dx) <= MAX_DRAG && Math.abs(dy) <= MAX_DRAG;
}

function createMascotWindow({ BrowserWindow, screen, dir, htmlPath, preloadPath, onActivate, onHide, lang }) {
    let win = null;
    let state = { mood: 'idle', label: '' };
    let collapsed = false;
    let drag = null;
    let saveTimer = null;
    let displayListener = null;
    const alive = () => !!win && !win.isDestroyed();
    const htmlURL = pathToFileURL(htmlPath).href;
    const strings = () => {
        let l = 'en';
        try { l = typeof lang === 'function' && lang() === 'ca' ? 'ca' : 'en'; } catch (_) {}
        return { ...STRINGS[l] };
    };

    function push() {
        if (alive()) win.webContents.send('gregal:mascot-state', { ...state, collapsed, strings: strings() });
    }

    function position() {
        const [x, y] = win.getPosition();
        return { x, y };
    }

    function persistNow() {
        if (!alive()) return;
        try { savePosition(dir, { ...position(), collapsed }); } catch (_) {}
    }

    function remember() {
        clearTimeout(saveTimer);
        saveTimer = setTimeout(persistNow, 400);
    }

    // Sempre amb setBounds i la mida explícita: a Windows, setPosition en
    // pantalles amb escales diferents pot anar canviant la mida d'una
    // finestra no redimensionable.
    function moveTo(p, size) {
        const s = size || sizeFor(collapsed);
        win.setBounds({ x: p.x, y: p.y, width: s, height: s });
    }

    function place(pos, size) {
        if (!alive()) return null;
        const s = size || sizeFor(collapsed);
        const p = clampToArea(pos, s, workAreaFor(screen, pos.x, pos.y, s));
        moveTo(p, s);
        return p;
    }

    // Canvis de pantalles (monitor desconnectat, resolució, barra de
    // tasques): la tornem a posar dins de l'àrea útil.
    function refit() {
        if (!alive()) return;
        place(position());
        remember();
    }

    function watchDisplays() {
        if (!screen || typeof screen.on !== 'function' || displayListener) return;
        displayListener = () => refit();
        for (const ev of ['display-removed', 'display-added', 'display-metrics-changed']) screen.on(ev, displayListener);
    }

    function unwatchDisplays() {
        if (!displayListener || !screen || typeof screen.removeListener !== 'function') { displayListener = null; return; }
        for (const ev of ['display-removed', 'display-added', 'display-metrics-changed']) screen.removeListener(ev, displayListener);
        displayListener = null;
    }

    function open() {
        if (alive()) return win;
        const pos = loadPosition(dir, screen);
        collapsed = pos.collapsed === true;
        const size = sizeFor(collapsed);
        const w = new BrowserWindow({
            width: size,
            height: size,
            x: pos.x,
            y: pos.y,
            frame: false,
            transparent: true,
            backgroundColor: '#00000000',
            hasShadow: false,
            alwaysOnTop: true,
            skipTaskbar: true,
            resizable: false,
            maximizable: false,
            minimizable: false,
            fullscreenable: false,
            show: false,
            title: 'Gregal',
            webPreferences: {
                preload: preloadPath,
                contextIsolation: true,
                nodeIntegration: false,
                sandbox: true,
                webviewTag: false,
                spellcheck: false,
            },
        });
        win = w;
        // Res de navegar ni obrir finestres: és contingut local i prou.
        w.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));
        w.webContents.on('will-navigate', event => event.preventDefault());
        w.webContents.on('did-finish-load', push);
        w.on('move', remember);
        w.once('ready-to-show', () => { if (!w.isDestroyed()) w.showInactive(); });
        w.once('closed', () => {
            clearTimeout(saveTimer);
            if (win === w) { win = null; drag = null; unwatchDisplays(); }
        });
        watchDisplays();
        w.loadFile(htmlPath);
        return w;
    }

    function close() {
        const w = win;
        clearTimeout(saveTimer);
        if (w && !w.isDestroyed()) persistNow();
        win = null;
        drag = null;
        unwatchDisplays();
        if (w && !w.isDestroyed()) w.close();
    }

    return {
        setEnabled(on) { if (on) open(); else close(); },
        // Estat que arriba de la pàgina de confiança: només els cinc ànims i
        // una etiqueta curta (ja traduïda) per al títol accessible.
        setState(mood, label) {
            if (typeof mood !== 'string' || !MOODS.has(mood)) return false;
            state = { mood, label: typeof label === 'string' ? label.slice(0, 200) : '' };
            push();
            return true;
        },
        // Els missatges de la mascota només s'accepten de la seva pròpia
        // finestra, del marc principal i amb mascot.html carregat.
        isSender(event) {
            if (!alive() || !event) return false;
            const frame = event.senderFrame;
            return event.sender === win.webContents && frame === win.webContents.mainFrame && !!frame && typeof frame.url === 'string' && frame.url.split(/[?#]/)[0] === htmlURL;
        },
        activate() { if (typeof onActivate === 'function') onActivate(); },
        // Arrossegament manual: 'start' fixa l'origen, 'move' hi suma el
        // desplaçament del punter (validat i limitat a la pantalla) i 'end'
        // l'enganxa a la vora si hi és a prop i desa la posició.
        drag(phase, dx, dy) {
            if (!alive()) return false;
            if (phase === 'start') { drag = position(); return true; }
            if (phase !== 'move' && phase !== 'end') return false;
            if (!drag) return false;
            if (!validDelta(dx, dy)) { if (phase === 'end') drag = null; return false; }
            const size = sizeFor(collapsed);
            let target = { x: drag.x + dx, y: drag.y + dy };
            const area = workAreaFor(screen, target.x, target.y, size);
            target = clampToArea(target, size, area);
            if (phase === 'end') {
                target = magnet(target, size, area);
                drag = null;
            }
            moveTo(target, size);
            if (phase === 'end') persistNow();
            return true;
        },
        setCollapsed(on) {
            if (typeof on !== 'boolean' || !alive()) return false;
            if (on === collapsed) { push(); return true; }
            const from = sizeFor(collapsed), to = sizeFor(on);
            const pos = position();
            const next = resizeAnchored(pos, from, to, workAreaFor(screen, pos.x, pos.y, from));
            collapsed = on;
            drag = null;
            place(next, to);
            persistNow();
            push();
            return true;
        },
        snap() {
            if (!alive()) return false;
            const size = sizeFor(collapsed);
            const pos = position();
            const area = workAreaFor(screen, pos.x, pos.y, size);
            if (!area) return false;
            const p = nextCorner(pos, size, area);
            moveTo(p, size);
            persistNow();
            return true;
        },
        // Amagar des de la mateixa finestra: qui la crea apaga la
        // preferència (onHide) i la tanca.
        hide() {
            if (!alive()) return false;
            if (typeof onHide === 'function') onHide();
            close();
            return true;
        },
        isOpen: alive,
        isCollapsed: () => collapsed,
        state: () => ({ ...state }),
        refit,
        close,
    };
}

module.exports = {
    createMascotWindow, loadPosition, savePosition, positionPath, clampToArea, magnet, nextCorner, resizeAnchored, workAreaFor, validDelta,
    PREF_KEY, MOODS, SIZE, COLLAPSED_SIZE, MARGIN, STRINGS,
};
