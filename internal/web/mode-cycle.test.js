const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

test('Tab cycles Ask, Plan and Act and returns from an unknown mode to Ask', () => {
  const window = {};
  vm.runInNewContext(fs.readFileSync(__dirname + '/app/mode-cycle.js', 'utf8'), { window });
  const next = window.gregalModeCycle.next;
  assert.equal(next('chat'), 'plan');
  assert.equal(next('plan'), 'act');
  assert.equal(next('act'), 'chat');
  assert.equal(next('goal'), 'chat');
});

test('Ask/Plan/Act radios and the special-mode chip follow the session mode', () => {
  const source = fs.readFileSync(__dirname + '/index.html', 'utf8');
  const start = source.indexOf('function paintMode(mode)');
  const end = source.indexOf('// El menú de permisos', start);
  assert.ok(start >= 0 && end > start);
  const uiStart = source.indexOf('const uiMode = ');
  const uiEnd = source.indexOf('\n', uiStart);
  assert.ok(uiStart >= 0);
  const chip = {hidden:true, dataset:{}};
  const label = {textContent:''};
  const radios = ['chat','plan','act'].map(mode => ({
    dataset:{mode}, setAttribute(key,value) { this[key] = value; },
  }));
  const context = {document:{querySelectorAll:() => radios},
    $:id => ({modeChip:chip, modeChipLabel:label})[id], gregalT:key => key};
  vm.createContext(context);
  vm.runInContext(source.slice(uiStart, uiEnd) + '\n' + source.slice(start,end) + '\nthis.uiMode = uiMode;', context);
  const want = {code:'act', autonomous:'act', debug:'act', chat:'chat', inspect:'chat', plan:'plan', goal:'plan'};
  for (const [mode, radio] of Object.entries(want)) {
    context.paintMode(mode);
    const special = ['goal','debug'].includes(mode);
    assert.equal(chip.hidden, !special, mode);
    const checked = radios.filter(r => r['aria-checked'] === 'true');
    assert.deepEqual(checked.map(r => r.dataset.mode), [radio], mode);
  }
});

test('the permission menu offers exactly the three Act permissions', () => {
  const source = fs.readFileSync(__dirname + '/index.html', 'utf8');
  const start = source.indexOf('const PERMS = [');
  const end = source.indexOf('];', start);
  const perms = vm.runInNewContext(source.slice(start + 'const PERMS = '.length, end + 1));
  assert.equal(JSON.stringify(perms.map(p => p[0])), JSON.stringify(['ask', 'edits', 'full']));
  const i18n = fs.readFileSync(__dirname + '/app/i18n.js', 'utf8');
  for (const [, label, help] of perms) {
    for (const key of [label, help]) assert.equal(i18n.split("'" + key + "':").length - 1, 2, key);
  }
});
