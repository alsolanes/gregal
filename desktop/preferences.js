const fs = require('fs');
const path = require('path');

// Only UI preferences travel across backend origins; credentials stay there.
const keys = new Set(['gregal_tema', 'gregal_densitat', 'gregal_mida', 'gregal_idioma', 'gregal_side', 'gregal_side_compact', 'gregal_inspector', 'gregal_view', 'gregal_todo_obert', 'gregal_recent_models']);
function allowed(key) { return keys.has(key); }
function loadPreferences(dir) {
    try {
        const raw = JSON.parse(fs.readFileSync(path.join(dir, 'preferences.json'), 'utf8'));
        return Object.fromEntries(Object.entries(raw).filter(([k, v]) => allowed(k) && typeof v === 'string'));
    } catch (_) { return {}; }
}
function savePreferences(dir, values) {
    fs.mkdirSync(dir, { recursive: true });
    const file = path.join(dir, 'preferences.json');
    const filtered = Object.fromEntries(Object.entries(values).filter(([k, v]) => allowed(k) && typeof v === 'string'));
    fs.writeFileSync(file + '.tmp', JSON.stringify(filtered), { mode: 0o600 });
    fs.renameSync(file + '.tmp', file);
}
module.exports = { allowed, loadPreferences, savePreferences, keys: [...keys] };
