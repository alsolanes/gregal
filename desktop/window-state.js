// Recorda mida i posició de la finestra entre sessions. Un fitxer JSON al
// directori de dades de l'app: si es corromp o no hi és, valors per defecte.
const fs = require("fs");
const path = require("path");

const DEFAULTS = { width: 1220, height: 820 };

function statePath(userDataDir) {
    return path.join(userDataDir, "window-state.json");
}

function loadState(userDataDir) {
    try {
        const raw = JSON.parse(fs.readFileSync(statePath(userDataDir), "utf8"));
        const out = { ...DEFAULTS };
        for (const k of ["width", "height", "x", "y"]) {
            if (Number.isFinite(raw[k])) out[k] = raw[k];
        }
        if (out.width < 600) out.width = DEFAULTS.width;
        if (out.height < 400) out.height = DEFAULTS.height;
        // Multi-monitor desconnectat = finestra invisible. Fora d'un rang
        // raonable, es descarta la posició i el sistema la col·loca.
        if (!Number.isFinite(out.x) || out.x < -2000 || out.x > 10000) delete out.x;
        if (!Number.isFinite(out.y) || out.y < -2000 || out.y > 10000) delete out.y;
        // Maximitzada: es torna a maximitzar a la mateixa pantalla; la mida
        // de sobre és la normal, per quan la desmaximitzis.
        if (raw.maximized === true) out.maximized = true;
        return out;
    } catch (error) {
        return { ...DEFAULTS };
    }
}

// fitBounds: la finestra sencera dins l'àrea útil de la pantalla (sense la
// barra de tasques). Una posició desada a -31 o una alçada més gran que la
// pantalla deixaven un tros de finestra fora. Sense posició, centrada.
function fitBounds(state, workArea) {
    const wa = workArea;
    const width = Math.min(state.width, wa.width);
    const height = Math.min(state.height, wa.height);
    const has = Number.isFinite(state.x) && Number.isFinite(state.y);
    let x = has ? state.x : wa.x + Math.round((wa.width - width) / 2);
    let y = has ? state.y : wa.y + Math.round((wa.height - height) / 2);
    x = Math.min(Math.max(x, wa.x), wa.x + wa.width - width);
    y = Math.min(Math.max(y, wa.y), wa.y + wa.height - height);
    return { x, y, width, height };
}

function saveState(userDataDir, bounds) {
    try {
        const out = {};
        for (const k of ["width", "height", "x", "y"]) if (Number.isFinite(bounds && bounds[k])) out[k] = Math.round(bounds[k]);
        if (bounds && bounds.maximized === true) out.maximized = true;
        fs.mkdirSync(userDataDir, { recursive: true });
        fs.writeFileSync(statePath(userDataDir), JSON.stringify(out), "utf8");
        return true;
    } catch (error) {
        return false;
    }
}

module.exports = { loadState, saveState, statePath, fitBounds, DEFAULTS };
