const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync(require('node:path').join(__dirname, 'index.html'), 'utf8').replace(/\r\n/g, '\n');
function slice(start, end) { return source.slice(source.indexOf(start), source.indexOf(end, source.indexOf(start))); }
test('context details and speed remain visible independently of work steps', () => {
  const gauge = { hidden: true, dataset: {}, firstElementChild: { style: {} }, classList: { toggle() {} }, setAttribute(k, v) { this[k] = v; } };
  const speed = { hidden: true };
  const sandbox = { sending: true, $: id => ({ ctxGauge: gauge, speedReadout: speed })[id], document: { documentElement: { lang: 'ca' } }, gregalT: k => k === 'ui.speed.average' ? '{v} ({n})' : 'help' };
  vm.runInNewContext(slice('function updateCtxGauge(r)', '// --- Temps del torn') + '\n' + slice('let turnSpeed = null;', "let turnPhase = 'idle'"), sandbox);
  sandbox.updateCtxGauge({context_window: 10000, used_tokens: 2500});
  assert.equal(gauge.hidden, false); assert.match(gauge.dataset.tooltip, /25%/); assert.equal(gauge['aria-valuetext'], gauge.dataset.tooltip);
  sandbox.speedReset(); sandbox.speedAdd({pp: 100, tg: 20, prompt: 1000, cached: 0, completion: 100, ttft_ms: 10000, gen_ms: 5000});
  assert.equal(speed.hidden, false); assert.match(speed.textContent, /20(\.0)? t\/s/); assert.doesNotMatch(speed.textContent, /PP|TG/);
  sandbox.updateCtxGauge({context_window: 0}); assert.equal(gauge.hidden, true);
  sandbox.speedReset(); assert.equal(speed.hidden, true);
});

test('the speed readout only shows while a turn is running', () => {
  const speed = { hidden: true };
  const sandbox = { sending: false, $: id => ({ speedReadout: speed })[id], gregalT: k => k === 'ui.speed.average' ? '{v} ({n})' : 'help' };
  vm.runInNewContext(slice('let turnSpeed = null;', "let turnPhase = 'idle'"), sandbox);
  const call = { tg: 20, completion: 100, gen_ms: 5000 };
  // A session that finished in the background replays its speed events when you come back to it.
  sandbox.speedAdd(call);
  assert.equal(speed.hidden, true, 'a finished turn must not bring the readout back');
  sandbox.sending = true;
  sandbox.speedAdd(call);
  assert.equal(speed.hidden, false);
});

test('the go-to-end button follows the content, not only the scroll', () => {
  const button = { hidden: true };
  const main = { scrollHeight: 3000, scrollTop: 0, clientHeight: 700, addEventListener() {} };
  let onContent = null;
  const frames = [];
  // The page waits for the next animation frame before it looks again.
  const contentChanged = () => { onContent(); frames.splice(0).forEach(fn => fn()); };
  const sandbox = {
    main, hero: { hidden: true }, $: id => ({ toBottom: button })[id], linkFileRefs() {},
    requestAnimationFrame(fn) { frames.push(fn); return frames.length; },
    MutationObserver: class { constructor(fn) { onContent = fn; } observe() {} },
  };
  vm.runInNewContext(slice('function nearBottom()', '// Icones dels missatges') + '\n' +
    slice('function paintScrollDown()', "main.addEventListener('scroll'") + '\n' +
    slice('let frefFrame = 0;', "main.addEventListener('click'"), sandbox);
  sandbox.paintScrollDown();
  assert.equal(button.hidden, false, 'a long conversation read from the top shows the button');
  // Another conversation is opened: the content is replaced by a short one and nothing scrolls.
  main.scrollHeight = 700;
  contentChanged();
  assert.equal(button.hidden, true, 'everything fits: there is no end to go to');
  // The answer grows below while the reader stays at the top.
  main.scrollHeight = 3000;
  contentChanged();
  assert.equal(button.hidden, false);
});
