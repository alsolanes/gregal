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
        return out;
    } catch (error) {
        return { ...DEFAULTS };
    }
}

function saveState(userDataDir, bounds) {
    try {
        fs.mkdirSync(userDataDir, { recursive: true });
        fs.writeFileSync(statePath(userDataDir), JSON.stringify(bounds), "utf8");
        return true;
    } catch (error) {
        return false;
    }
}

module.exports = { loadState, saveState, statePath, DEFAULTS };
