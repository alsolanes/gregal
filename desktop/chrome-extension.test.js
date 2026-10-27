const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const os = require("os");
const path = require("path");
const ext = require("./chrome-extension");

test("l'extensió es copia sense proves ni package.json", () => {
    const src = fs.mkdtempSync(path.join(os.tmpdir(), "ext-src-"));
    const dest = path.join(fs.mkdtempSync(path.join(os.tmpdir(), "ext-dest-")), "chrome-extension");
    fs.writeFileSync(path.join(src, "manifest.json"), "{}");
    fs.writeFileSync(path.join(src, "panel.js"), "x");
    fs.writeFileSync(path.join(src, "logic.test.js"), "x");
    fs.writeFileSync(path.join(src, "package.json"), "{}");
    assert.equal(ext.syncExtension(src, dest), dest);
    assert.deepEqual(fs.readdirSync(dest).sort(), ["manifest.json", "panel.js"]);
    assert.equal(ext.syncExtension(path.join(src, "no"), dest), "");
});

test("on és l'extensió: dins l'app o al repositori", () => {
    assert.equal(ext.sourceDir({ packaged: true, resourcesPath: "/r", appDir: "/a" }), path.join("/r", "chrome-extension"));
    assert.equal(ext.sourceDir({ packaged: false, resourcesPath: "/r", appDir: "/repo/desktop" }), path.join("/repo/desktop", "..", "extension", "chrome"));
});

test("obre la pàgina d'extensions del primer navegador que hi hagi", () => {
    const calls = [];
    const run = (exe, args) => { calls.push([exe, args]); return { unref() {} }; };
    const edgeOnly = exe => exe.endsWith("msedge.exe");
    assert.equal(ext.openExtensionsPage("win32", { "ProgramFiles(x86)": "C:\PF86" }, edgeOnly, run), "edge");
    assert.deepEqual(calls[0][1], ["edge://extensions"]);
    assert.equal(ext.openExtensionsPage("win32", {}, () => false, run), "");
});
