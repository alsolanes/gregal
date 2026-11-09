const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { binaryCandidates, serverArgs, backendEnv, openBackendLog, rotateBackendLog, BACKEND_LOG_MAX } = require("./server-manager");

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

test("el log del backend s'afegeix a fitxer i rota quan passa el topall", async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gregal-log-"));
  const file = path.join(dir, "gregal-backend.log");
  const none = openBackendLog(null);
  none.write("res");
  await new Promise(resolve => none.close(resolve));
  fs.writeFileSync(file, "d'abans\n");
  const log = openBackendLog(file);
  log.write("línia u  ");
  log.write("línia dos");
  await new Promise(resolve => log.close(resolve));
  log.write("després de tancar: es perd sense petar");
  assert.equal(fs.readFileSync(file, "utf8"), "d'abans\nlínia u\nlínia dos\n");
  rotateBackendLog(file);
  assert.ok(!fs.existsSync(file + ".old"), "fitxer petit no rota");
  fs.writeFileSync(file, "x".repeat(BACKEND_LOG_MAX + 1));
  rotateBackendLog(file);
  assert.ok(fs.existsSync(file + ".old"), "fitxer gros rota a .old");
  rotateBackendLog(path.join(dir, "inexistent.log"));
});

test("un log que no es pot obrir no tomba l'app", async () => {
  // Una carpeta en lloc d'un fitxer: createWriteStream falla de manera asíncrona.
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gregal-log-"));
  const log = openBackendLog(dir);
  log.write("u");
  await new Promise(resolve => setTimeout(resolve, 50));
  log.write("dos");
  await new Promise(resolve => log.close(resolve));
  assert.ok(fs.statSync(dir).isDirectory());
});

test("startServer no escriu el log amb crides síncrones al fil principal", () => {
  const src = fs.readFileSync(path.join(__dirname, "server-manager.js"), "utf8");
  assert.doesNotMatch(src, /fs\.(appendFileSync|writeFileSync)\(/);
  assert.match(src, /createWriteStream\(file, \{ flags: "a" \}\)/);
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

test('app backend saves typed API keys to the key store unless the env says otherwise', () => {
  const { appBackendEnv } = require("./server-manager");
  const env = appBackendEnv({ GREGAL_APP_BROWSER: 'x' }, {});
  assert.equal(env.GREGAL_PERSIST_KEYS, '1');
  assert.equal(env.GREGAL_APP_BROWSER, 'x');
  assert.equal(appBackendEnv({}, { GREGAL_PERSIST_KEYS: '0' }).GREGAL_PERSIST_KEYS, undefined);
  assert.equal(appBackendEnv(undefined, {}).GREGAL_PERSIST_KEYS, '1');
});
