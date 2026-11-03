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

test('fitBounds keeps the whole window inside the work area', () => {
  const { fitBounds } = require('./window-state');
  const wa = { x: 0, y: 0, width: 1707, height: 1019 };
  // El cas real: desada a x=-31 i més alta que l'àrea útil.
  assert.deepEqual(fitBounds({ x: -31, y: 15, width: 1696, height: 1021 }, wa), { x: 0, y: 0, width: 1696, height: 1019 });
  // Sense posició desada: centrada.
  assert.deepEqual(fitBounds({ width: 1220, height: 820 }, wa), { x: 244, y: 100, width: 1220, height: 820 });
  // Massa a la dreta i a baix: s'arrossega dins.
  assert.deepEqual(fitBounds({ x: 1500, y: 900, width: 1000, height: 600 }, wa), { x: 707, y: 419, width: 1000, height: 600 });
  // Una segona pantalla a la dreta.
  assert.deepEqual(fitBounds({ x: 1800, y: 50, width: 1200, height: 800 }, { x: 1707, y: 0, width: 1920, height: 1040 }), { x: 1800, y: 50, width: 1200, height: 800 });
});
