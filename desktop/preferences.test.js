const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');
const vm = require('vm');
const { allowed, loadPreferences, savePreferences } = require('./preferences');

test('appearance survives restart, while credentials and session state are excluded', t => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gregal-prefs-'));
    t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
    savePreferences(dir, { gregal_tema: 'light', gregal_idioma: 'ca', gregal_token: 'secret', gregal_session: 'old', gregal_mida: 12 });
    assert.deepEqual(loadPreferences(dir), { gregal_tema: 'light', gregal_idioma: 'ca' });
    assert.equal(allowed('gregal_token'), false);
    fs.writeFileSync(path.join(dir, 'preferences.json'), '{bad');
    assert.deepEqual(loadPreferences(dir), {});
});

test('desktop hydrates a new origin and persists edits without copying tokens', () => {
    class Storage {
        constructor() { this.data = new Map(); }
        getItem(k) { return this.data.get(k) ?? null; }
        setItem(k, v) { this.data.set(k, String(v)); }
        removeItem(k) { this.data.delete(k); }
        clear() { this.data.clear(); }
    }
    const localStorage = new Storage();
    const writes = [];
    const context = { Storage, localStorage, window: { gregalDesktop: {
        preferences: { gregal_tema: 'light' }, preferenceKeys: ['gregal_tema'],
        savePreference: (k, v) => writes.push([k, v]),
    } } };
    vm.runInNewContext(fs.readFileSync(path.join(__dirname, '../internal/web/app/desktop-prefs.js'), 'utf8'), context);
    assert.equal(localStorage.getItem('gregal_tema'), 'light');
    localStorage.setItem('gregal_tema', 'dark');
    localStorage.setItem('gregal_token', 'private');
    assert.deepEqual(writes, [['gregal_tema', 'dark']]);
    localStorage.removeItem('gregal_tema');
    assert.deepEqual(writes.at(-1), ['gregal_tema', null]);
});
