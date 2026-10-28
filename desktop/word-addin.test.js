const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const os = require("os");
const path = require("path");
const addin = require("./word-addin");

const repoManifest = fs.readFileSync(path.join(__dirname, "..", "internal", "web", "app", "addin", "gregal-addin.xml"), "utf8");

test("the manifest follows the backend's real port", () => {
    const xml = addin.withOrigin(repoManifest, "http://127.0.0.1:8123");
    assert.ok(!xml.includes(":8097"), "no 8097 left in the manifest");
    assert.match(xml, /<SourceLocation DefaultValue="http:\/\/127\.0\.0\.1:8123\/app\/addin\/taskpane\.html" \/>/);
    assert.match(xml, /<AppDomain>http:\/\/localhost:8123<\/AppDomain>/);
    assert.match(xml, /<IconUrl DefaultValue="http:\/\/127\.0\.0\.1:8123\/app\/addin\/icon-32\.png" \/>/);
    // Al port de sempre no canvia res.
    assert.equal(addin.withOrigin(repoManifest, "http://127.0.0.1:8097"), repoManifest);
});

test("the registry value is the manifest Id and points at the saved file", async () => {
    const id = addin.manifestId(repoManifest);
    assert.match(id, /^[0-9A-F-]{36}$/i);
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "word-addin-"));
    const file = addin.writeManifest(repoManifest, dir);
    assert.equal(fs.readFileSync(file, "utf8"), repoManifest);

    const calls = [];
    const run = (cmd, args, opts, cb) => {
        calls.push([cmd, ...args]);
        if (args[0] === "query") cb(null, `\r\n${addin.REG_KEY}\r\n    ${id}    REG_SZ    ${file}\r\n`);
        else cb(null, "");
    };
    assert.equal(await addin.register(id, file, run), true);
    assert.deepEqual(calls[0], ["reg", "add", addin.REG_KEY, "/v", id, "/t", "REG_SZ", "/d", file, "/f"]);
    assert.equal(await addin.registeredPath(id, run), file);
    assert.equal(await addin.unregister(id, run), true);
    assert.deepEqual(calls[2], ["reg", "delete", addin.REG_KEY, "/v", id, "/f"]);
});

test("a missing registry value reads as not installed", async () => {
    const run = (cmd, args, opts, cb) => cb(new Error("not found"), "");
    assert.equal(await addin.registeredPath("64C79419-6A50-4552-BF4F-075D1C5F849D", run), "");
});
