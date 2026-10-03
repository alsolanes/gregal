const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../internal/web/app/team.js'), 'utf8');
const motion = source.slice(source.indexOf('function smooth('), source.indexOf('function drawAgent('));
function fixture() {
  const roles = Array.from({length:8}, (_, i) => `agent-${i}`);
  const context = vm.createContext({
    roles, homes:roles.map((_, i) => [165 + i * 75, 210]),
    states:Object.fromEntries(roles.map(role => [role, 'idle'])),
    reducedMotion:{matches:false}, handoffWalk:null,
  });
  vm.runInContext(motion, context);
  return context;
}

test('every character keeps finite coordinates throughout the idle cycle', () => {
  const context = fixture();
  for (let now = 0; now <= 48000; now += 100) {
    for (let i = 0; i < context.roles.length; i++) {
      const point = context.position(i, now);
      assert.ok(Number.isFinite(point.x) && Number.isFinite(point.y), `agent ${i} at ${now}ms`);
      assert.ok(point.x >= 0 && point.x <= 1024 && point.y >= 0 && point.y <= 540);
    }
  }
});

test('coffee pause has explicit coordinates and does not move', () => {
  const context = fixture();
  const point = context.position(0, 5000);
  assert.equal(point.x, 460);
  assert.equal(point.y, 350);
  assert.equal(point.moving, false);
});

test('working, waiting and reduced-motion characters stay at their desks', () => {
  const context = fixture();
  for (const state of ['working', 'waiting']) {
    context.states['agent-0'] = state;
    assert.equal(context.position(0, 5000).x, context.homes[0][0]);
  }
  context.states['agent-0'] = 'idle';
  context.reducedMotion.matches = true;
  assert.equal(context.position(0, 5000).x, context.homes[0][0]);
});
