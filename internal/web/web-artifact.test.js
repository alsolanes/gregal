const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

async function artifactModule() {
  const module = new vm.SourceTextModule(fs.readFileSync('internal/web/app/web-artifact.js', 'utf8'));
  await module.link(() => {});
  await module.evaluate();
  return module.namespace;
}

test('HTML artifacts require a complete document, not prose or partial markup', async () => {
  const { htmlArtifact } = await artifactModule();
  assert.equal(htmlArtifact('Hello world'), null);
  assert.equal(htmlArtifact('```html\n<div>partial</div>\n```'), null);
  assert.equal(htmlArtifact('```html\n<!doctype html><html><body>Festival</body></html>\n```'), '<!doctype html><html><body>Festival</body></html>');
  assert.equal(htmlArtifact('<html>' + 'x'.repeat(200001) + '</html>'), null);
});

test('prototype preview policy precedes model HTML and disables network and forms', async () => {
  const { isolatedHTML } = await artifactModule();
  const source = isolatedHTML('<html><script>fetch("https://example.com")</script></html>');
  assert.ok(source.indexOf('Content-Security-Policy') < source.indexOf('<script>'));
  assert.ok(source.includes("connect-src 'none'"));
  assert.ok(source.includes("form-action 'none'"));
  assert.ok(source.includes("frame-src 'none'"));
});
