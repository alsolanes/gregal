// Arrencada de la finestra: els colors que es veuen abans que la pàgina
// pugui dir el tema que té, i el ritme amb què s'espera el backend.
//
// La finestra surt de seguida amb loading.html (un fitxer local, sense
// backend) i passa a l'app quan /api/health respon. Perquè no hi hagi cap
// flaix d'un altre color, la finestra, la barra de títol i loading.html fan
// servir els colors que l'app va pintar l'última vegada amb el mateix tema
// (startup-theme.json, a userData); sense res desat, el fosc o el clar de
// la marca segons el tema triat o el del sistema.
const fs = require('fs');
const path = require('path');

const FILE = 'startup-theme.json';

// Els de calma.css (fosc) i index.html (clar): docs/identitat.md.
const FALLBACK = {
    dark: { bg: '#14161B', bar: '#101217', ink: '#F2F4F8', accent: '#8193FF' },
    light: { bg: '#F6F7FA', bar: '#EEF0F5', ink: '#0E1220', accent: '#4F5BE8' },
};
// Temes de Preferències → Tema que són clars; la resta són foscos.
const LIGHT_THEMES = new Set(['light', 'gpt', 'claude', 'mistral', 'gemini']);
const DARK_THEMES = new Set(['dark', 'calma', 'opencode', 'grok']);

const isHex = v => typeof v === 'string' && /^#[0-9a-fA-F]{6}$/.test(v);

// themeKey diu per a quin tema valen uns colors desats: el tema triat o,
// si se segueix el sistema, el sistema en fosc o en clar.
function themeKey(tema, systemDark) {
    const t = typeof tema === 'string' && /^[a-z0-9_-]{1,32}$/.test(tema) ? tema : 'sistema';
    return t === 'sistema' ? 'sistema-' + (systemDark ? 'dark' : 'light') : t;
}

// Clar o fosc segons la lluminositat del fons (per al text de les pàgines locals).
function schemeOf(bg) {
    if (!isHex(bg)) return 'dark';
    const n = parseInt(bg.slice(1), 16);
    const r = (n >> 16) & 255, g = (n >> 8) & 255, b = n & 255;
    return (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255 > 0.5 ? 'light' : 'dark';
}

function fallbackFor(tema, systemDark) {
    const dark = LIGHT_THEMES.has(tema) ? false : DARK_THEMES.has(tema) ? true : !!systemDark;
    const c = FALLBACK[dark ? 'dark' : 'light'];
    return { ...c, scheme: dark ? 'dark' : 'light' };
}

// loadStartupTheme torna { bg, bar, ink, accent, scheme } per al tema d'ara.
function loadStartupTheme(dir, { tema, systemDark } = {}) {
    const key = themeKey(tema, systemDark);
    try {
        const c = JSON.parse(fs.readFileSync(path.join(dir, FILE), 'utf8'));
        if (c && c.key === key && isHex(c.bg) && isHex(c.bar) && isHex(c.ink) && isHex(c.accent)) {
            return { bg: c.bg, bar: c.bar, ink: c.ink, accent: c.accent, scheme: schemeOf(c.bg) };
        }
    } catch (_) { /* sense fitxer o corromput: els de la marca */ }
    return fallbackFor(key.startsWith('sistema-') ? 'sistema' : key, systemDark);
}

// saveStartupTheme desa el que l'app ha pintat. Només colors #rrggbb (un
// accent en un altre format es canvia pel de la marca); torna false si no hi
// ha res a desar o no s'ha pogut.
function saveStartupTheme(dir, key, colors) {
    const c = colors || {};
    if (typeof key !== 'string' || !key || !isHex(c.bg) || !isHex(c.bar) || !isHex(c.ink)) return false;
    const accent = isHex(c.accent) ? c.accent : FALLBACK[schemeOf(c.bg)].accent;
    try {
        fs.mkdirSync(dir, { recursive: true });
        const file = path.join(dir, FILE);
        fs.writeFileSync(file + '.tmp', JSON.stringify({ key, bg: c.bg, bar: c.bar, ink: c.ink, accent }));
        fs.renameSync(file + '.tmp', file);
        return true;
    } catch (_) {
        return false;
    }
}

// Paràmetres de loading.html i error.html: colors, idioma i nom del producte.
function pageQuery(theme, lang, name) {
    return {
        lang: lang === 'ca' ? 'ca' : 'en',
        name: typeof name === 'string' && name ? name.slice(0, 40) : 'Gregal',
        scheme: theme.scheme === 'light' ? 'light' : 'dark',
        bg: theme.bg, bar: theme.bar, ink: theme.ink, accent: theme.accent,
    };
}

// Cada quan es torna a preguntar al backend: de pressa els primers dos
// segons (sol respondre en mig segon i cada pas de 200 ms era temps perdut
// mirant una finestra buida) i després més espaiat.
const FAST_POLL_MS = 50;
const SLOW_POLL_MS = 200;
const FAST_WINDOW_MS = 2000;
function pollDelay(elapsedMs) {
    return elapsedMs < FAST_WINDOW_MS ? FAST_POLL_MS : SLOW_POLL_MS;
}

// waitFor pregunta probe() fins que diu que sí, fins al límit o fins que
// gaveUp() diu que no cal esperar més (el backend ha mort). onSlow avisa
// cada ~2 s d'espera perquè els logs diguin per què no surt res.
async function waitFor(probe, { timeoutMs = 20000, gaveUp = () => false, onSlow, now = Date.now, sleep = ms => new Promise(r => setTimeout(r, ms)) } = {}) {
    const start = now();
    let noticed = 0;
    while (now() - start < timeoutMs) {
        if (await probe()) return true;
        if (gaveUp()) return false;
        const elapsed = now() - start;
        if (onSlow && elapsed - noticed >= FAST_WINDOW_MS) { noticed = elapsed; onSlow(elapsed); }
        await sleep(pollDelay(elapsed));
    }
    return false;
}

module.exports = {
    FILE, FALLBACK, themeKey, schemeOf, loadStartupTheme, saveStartupTheme, pageQuery,
    pollDelay, waitFor, FAST_POLL_MS, SLOW_POLL_MS, FAST_WINDOW_MS,
};
