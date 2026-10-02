// `node --check` dona un fals positiu amb els mòduls ES: els parseja com a
// script i deixa passar coses que el navegador rebutja (un `const` enmig
// d'una assignació, posem). Al navegador això no és un error visible —el
// mòdul no carrega i la finestra es queda a mitges, sense res a la
// pantalla que ho digui. Aquest test els parseja com el que són.
//
// S'executa amb: node --experimental-vm-modules --test internal/web/moduls.test.js
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const dir = path.join(__dirname, "app");

test("tots els mòduls de la UI es parsegen com a mòdul ES", () => {
  assert.ok(vm.SourceTextModule, "cal --experimental-vm-modules");
  const fitxers = fs.readdirSync(dir).filter((f) => f.endsWith(".js"));
  assert.ok(fitxers.length >= 5, `esperava uns quants mòduls, n'he trobat ${fitxers.length}`);
  for (const f of fitxers) {
    const codi = fs.readFileSync(path.join(dir, f), "utf8");
    assert.doesNotThrow(() => new vm.SourceTextModule(codi, { identifier: f }), `${f} no es parseja`);
  }
});
