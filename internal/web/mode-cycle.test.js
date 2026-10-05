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
