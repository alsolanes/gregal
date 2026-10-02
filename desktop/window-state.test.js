const test = require("node:test");
const assert = require("node:assert");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { loadState, saveState, DEFAULTS } = require("./window-state");

function tmp() {
    return fs.mkdtempSync(path.join(os.tmpdir(), "gregal-ws-"));
}

test("sense fitxer torna els valors per defecte", () => {
    assert.deepStrictEqual(loadState(tmp()), { ...DEFAULTS });
});

test("desa i recupera mida i posició", () => {
    const dir = tmp();
    assert.ok(saveState(dir, { width: 1000, height: 700, x: 12, y: 34 }));
    assert.deepStrictEqual(loadState(dir), { width: 1000, height: 700, x: 12, y: 34 });
});

test("un fitxer corromput no tomba l'app", () => {
    const dir = tmp();
    fs.writeFileSync(path.join(dir, "window-state.json"), "{{{no json");
    assert.deepStrictEqual(loadState(dir), { ...DEFAULTS });
});

test("mides absurdes es descarten", () => {
    const dir = tmp();
    saveState(dir, { width: 10, height: 10 });
    const got = loadState(dir);
    assert.strictEqual(got.width, DEFAULTS.width);
    assert.strictEqual(got.height, DEFAULTS.height);
});
