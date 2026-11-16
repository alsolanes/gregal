const test = require("node:test");
const assert = require("node:assert");
const { previewKind, srcdoc } = require("./md-preview.js");

test("només html i svg sencers tenen vista prèvia", () => {
  assert.equal(previewKind("html", "<!doctype html><html><body><h1>Hola</h1></body></html>"), "html");
  assert.equal(previewKind("HTML", "<button onclick=\"x()\">Prem</button>"), "html");
  assert.equal(previewKind("svg", "<svg viewBox=\"0 0 10 10\"><circle r=\"4\"/></svg>"), "svg");
  assert.equal(previewKind("svg", "<svg viewBox=\"0 0 10 10\">"), "", "a mitges no");
  assert.equal(previewKind("js", "<div></div>"), "");
  assert.equal(previewKind("html", "només text"), "");
  assert.equal(previewKind("html", "<p>" + "x".repeat(200001) + "</p>"), "");
});

test("el document aïllat no pot sortir a la xarxa", () => {
  for (const [kind, code] of [["html", "<p>hola</p>"], ["html", "<!doctype html><html><head><title>t</title></head><body></body></html>"], ["html", "<html><body>x</body></html>"], ["svg", "<svg></svg>"]]) {
    const doc = srcdoc(kind, code);
    assert.match(doc, /Content-Security-Policy/);
    assert.match(doc, /connect-src 'none'/);
    assert.match(doc, /default-src 'none'/);
    // La CSP va dins del <head>, abans de qualsevol altra cosa del model.
    assert.ok(doc.indexOf("Content-Security-Policy") < (doc.indexOf("<title>") >= 0 ? doc.indexOf("<title>") : Infinity));
  }
});
