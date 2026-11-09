const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync(require('node:path').join(__dirname, 'index.html'), 'utf8').replace(/\r\n/g, '\n');
function slice(start, end) { return source.slice(source.indexOf(start), source.indexOf(end, source.indexOf(start))); }
test('context details and speed remain visible independently of work steps', () => {
  const gauge = { hidden: true, dataset: {}, firstElementChild: { style: {} }, classList: { toggle() {} }, setAttribute(k, v) { this[k] = v; } };
  const speed = { hidden: true };
  const sandbox = { $: id => ({ ctxGauge: gauge, speedReadout: speed })[id], document: { documentElement: { lang: 'ca' } }, gregalT: k => k === 'ui.speed.average' ? '{v} ({n})' : 'help' };
  vm.runInNewContext(slice('function updateCtxGauge(r)', '// --- Temps del torn') + '\n' + slice('let turnSpeed = null;', "let turnPhase = 'idle'"), sandbox);
  sandbox.updateCtxGauge({context_window: 10000, used_tokens: 2500});
  assert.equal(gauge.hidden, false); assert.match(gauge.dataset.tooltip, /25%/); assert.equal(gauge['aria-valuetext'], gauge.dataset.tooltip);
  sandbox.speedReset(); sandbox.speedAdd({pp: 100, tg: 20, prompt: 1000, cached: 0, completion: 100, ttft_ms: 10000, gen_ms: 5000});
  assert.equal(speed.hidden, false); assert.match(speed.textContent, /PP 100.*TG 20/);
  sandbox.updateCtxGauge({context_window: 0}); assert.equal(gauge.hidden, true);
  sandbox.speedReset(); assert.equal(speed.hidden, true);
});
