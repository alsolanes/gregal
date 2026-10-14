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

test('Ask/Act radios and the special-mode chip follow the session mode', () => {
  const source = fs.readFileSync(__dirname + '/index.html', 'utf8');
  const start = source.indexOf('function paintMode(mode)');
  const end = source.indexOf('// El menú de permisos', start);
  assert.ok(start >= 0 && end > start);
  const chip = {hidden:true, dataset:{}};
  const label = {textContent:''};
  const plan = {hidden:false};
  const radios = ['chat','code'].map(mode => ({
    dataset:{mode}, setAttribute(key,value) { this[key] = value; },
  }));
  const context = {document:{querySelectorAll:() => radios},
    $:id => ({modeChip:chip, modeChipLabel:label, planFirst:plan})[id], gregalT:key => key};
  vm.createContext(context);
  vm.runInContext(source.slice(start,end), context);
  for (const mode of ['code','chat','goal','autonomous','debug']) {
    context.paintMode(mode);
    const special = ['goal','autonomous','debug'].includes(mode);
    assert.equal(chip.hidden, !special, mode);
    assert.equal(radios.filter(radio => radio['aria-checked'] === 'true').length, special ? 0 : 1, mode);
    assert.equal(plan.hidden, mode === 'chat' || mode === 'goal', mode);
  }
});
