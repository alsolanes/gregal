const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { binaryCandidates, serverArgs, backendEnv, appendBackendLog, rotateBackendLog, BACKEND_LOG_MAX } = require("./server-manager");

test('managed server uses loopback and preserves explicit config paths', () => {
  assert.deepEqual(serverArgs({ port: 8097, config: 'C:/Example Project/config.yaml', dir: 'C:/Example Project' }),
    ['--serve', '--addr', '127.0.0.1', '--port', '8097', '--dir', 'C:/Example Project', '--config', 'C:/Example Project/config.yaml']);
});

test("prioritza el backend empaquetat i el backend de desenvolupament", () => {
  const got = binaryCandidates({ resourcesPath: "/app/resources", appDir: "/src/desktop", platform: "linux" });
  assert.equal(got[0], path.join("/app/resources", "backend", "gregal"));
  assert.equal(got[1], path.join("/src/desktop", "backend", "gregal"));
  assert.equal(got[2], path.resolve("/src/desktop", "..", "gregal"));
  assert.equal(got[3], "gregal");
});

test("el log del backend s'afegeix a fitxer i rota quan passa el topall", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gregal-log-"));
  const file = path.join(dir, "gregal-backend.log");
  appendBackendLog(null, "res");
  appendBackendLog(file, "línia u  ");
  appendBackendLog(file, "línia dos");
  assert.equal(fs.readFileSync(file, "utf8"), "línia u\nlínia dos\n");
  rotateBackendLog(file);
  assert.ok(!fs.existsSync(file + ".old"), "fitxer petit no rota");
  fs.writeFileSync(file, "x".repeat(BACKEND_LOG_MAX + 1));
  rotateBackendLog(file);
  assert.ok(fs.existsSync(file + ".old"), "fitxer gros rota a .old");
  rotateBackendLog(path.join(dir, "inexistent.log"));
});

test('backend env adds the app browser bridge without touching process.env', () => {
  const before = process.env.GREGAL_APP_BROWSER;
  const env = backendEnv({ env: { GREGAL_APP_BROWSER: 'http://127.0.0.1:1234/?token=abc', EMPTY: '' } });
  assert.equal(env.GREGAL_APP_BROWSER, 'http://127.0.0.1:1234/?token=abc');
  assert.equal('EMPTY' in env, false);
  assert.equal(env.PATH ?? env.Path, process.env.PATH ?? process.env.Path);
  assert.equal(process.env.GREGAL_APP_BROWSER, before);
  assert.equal(backendEnv({}), process.env);
});
