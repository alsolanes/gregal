const test = require("node:test");
const assert = require("node:assert/strict");
const { QUICK_NOTE_ACCELERATOR, showQuickNote, registerQuickNote } = require("./quick-note");

function fakeWin({ minimized = false, destroyed = false } = {}) {
    const calls = [];
    return {
        calls,
        isDestroyed: () => destroyed,
        isMinimized: () => minimized,
        restore: () => calls.push("restore"),
        show: () => calls.push("show"),
        focus: () => calls.push("focus"),
        webContents: { send: (ch, name) => calls.push(ch + ":" + name) },
    };
}

test("la nota ràpida porta la finestra al davant i demana la captura", () => {
    const w = fakeWin({ minimized: true });
    assert.equal(showQuickNote(w), true);
    assert.deepEqual(w.calls, ["restore", "show", "focus", "gregal:command:quick-note"]);
    assert.equal(showQuickNote(fakeWin({ destroyed: true })), false);
    assert.equal(showQuickNote(null), false);
});

test("registra Ctrl+Alt+N una sola vegada i no peta si una altra app la té", () => {
    const registered = new Map();
    const gs = {
        isRegistered: a => registered.has(a),
        register: (a, fn) => { registered.set(a, fn); return true; },
    };
    const w = fakeWin();
    assert.equal(registerQuickNote(gs, () => w), true);
    assert.equal(registerQuickNote(gs, () => w), true);
    assert.equal(registered.size, 1);
    registered.get(QUICK_NOTE_ACCELERATOR)();
    assert.ok(w.calls.includes("gregal:command:quick-note"));

    const busy = { isRegistered: () => false, register: () => { throw new Error("taken"); } };
    assert.equal(registerQuickNote(busy, () => w), false);
});

test("quick-note.js viatja a l'instal·lador", () => {
    const pkg = require("./package.json");
    assert.ok(pkg.build.files.includes("quick-note.js"));
});
