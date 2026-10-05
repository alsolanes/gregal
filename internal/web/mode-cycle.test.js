const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

test('Tab cycles the primary modes and returns from an advanced mode to code', () => {
  const window = {};
  vm.runInNewContext(fs.readFileSync(__dirname + '/app/mode-cycle.js', 'utf8'), { window });
  const next = window.gregalModeCycle.next;
  assert.equal(next('code'), 'chat');
  assert.equal(next('chat'), 'code');
  assert.equal(next('goal'), 'code');
  assert.equal(next('autonomous'), 'code');
});

test('mode refresh preserves the user-controlled advanced disclosure', () => {
  const source = fs.readFileSync(__dirname + '/index.html', 'utf8');
  const start = source.indexOf('function paintMode(mode)');
  const end = source.indexOf('// El menú de permisos', start);
  assert.ok(start >= 0 && end > start);
  const advanced = {open:false, dataset:{}};
  const label = {textContent:''};
  const radios = ['code','chat','goal','autonomous'].map(mode => ({
    dataset:{mode}, setAttribute(key,value) { this[key] = value; },
  }));
  const context = {document:{querySelectorAll:() => radios},
    $:id => id === 'advancedModes' ? advanced : label, gregalT:key => key};
  vm.createContext(context);
  vm.runInContext(source.slice(start,end), context);
  for (const open of [true,false]) {
    advanced.open = open;
    for (const mode of ['code','chat','goal','autonomous']) {
      context.paintMode(mode);
      assert.equal(advanced.open, open, mode);
      assert.equal(advanced.dataset.active, String(mode === 'goal' || mode === 'autonomous'));
      assert.equal(radios.filter(radio => radio['aria-checked'] === 'true').length, 1);
    }
  }
});
