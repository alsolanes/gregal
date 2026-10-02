const test = require("node:test");
const assert = require("node:assert/strict");
const path = require("node:path");
const { binaryCandidates, serverArgs } = require("./server-manager");

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
