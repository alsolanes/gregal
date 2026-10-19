const { test } = require('node:test');
const assert = require('node:assert/strict');
const { validate, config } = require('./plots-core.js');
const md = require('./md.js');
const spec = { title: 'Sales', type: 'line', labels: ['Jan', 'Feb'], series: [{ label: 'EUR', values: [10, 20] }] };
test('specs reject malformed data and never forward executable options', () => {
  assert.throws(() => validate({ ...spec, type: 'html' }));
  assert.throws(() => validate({ ...spec, series: [{label: 'EUR', values: [1]}] }));
  assert.throws(() => validate({ ...spec, series: [{label: 'EUR', values: [Infinity, 0]}] }));
  assert.throws(() => validate({ ...spec, type: 'scatter' }));
  const clean = validate({ ...spec, options: { onClick: 'alert(1)' } });
  assert.equal(clean.options, undefined);
  clean.series[0].values[0] = 100;
  assert.equal(spec.series[0].values[0], 10);
});
test('scatter uses numeric x/y coordinates, categorical charts preserve labels', () => {
  const scatter = config({...spec, type: 'scatter', labels: ['1', '2.5']});
  assert.deepEqual(scatter.data.datasets[0].data, [{x: 1, y: 10}, {x: 2.5, y: 20}]);
  assert.deepEqual(config(spec).data.labels, ['Jan', 'Feb']);
});
test('area and horizontal bars support signed grouped data with a zero baseline', () => {
  const grouped = {...spec, series: [{label: 'a', values: [-2, 0]}, {label: 'b', values: [3, -1]}]};
  const area = config({...grouped, type: 'area'});
  assert.equal(area.type, 'line');
  assert.equal(area.data.datasets[0].fill, 'origin');
  assert.equal(area.options.scales.y.beginAtZero, true);
  const horizontal = config({...grouped, type: 'horizontal-bar'});
  assert.equal(horizontal.type, 'bar');
  assert.equal(horizontal.options.indexAxis, 'y');
  assert.equal(horizontal.options.scales.x.beginAtZero, true);
  assert.equal(horizontal.options.scales.x.stacked, undefined);
  assert.deepEqual(horizontal.data.datasets.map(s => s.data), [[-2, 0], [3, -1]]);
});
test('pie and doughnut require positive nonnegative slices and use slice colors without axes', () => {
  for (const type of ['pie', 'doughnut']) {
    const input = {...spec, type};
    const c = config(input, '#abc', ['red', 'blue']);
    assert.equal(c.type, type);
    assert.deepEqual(c.data.datasets[0].data, [10, 20]);
    assert.deepEqual(c.data.datasets[0].backgroundColor, ['red', 'blue']);
    assert.deepEqual(c.data.datasets[0].borderColor, ['red', 'blue']);
    assert.deepEqual(c.options.scales, {});
    assert.equal(c.options.plugins.legend.labels.color, '#abc');
    for (const values of [[0, 0], [-1, 2], [Infinity, 1], [NaN, 1], [null, 1]]) {
      assert.throws(() => validate({...input, series: [{label: 'x', values}]}));
    }
    assert.throws(() => validate({...input, series: [...input.series, ...input.series]}));
    const huge = config({...input, series: [{label: 'x', values: [Number.MAX_VALUE, Number.MAX_VALUE]}]});
    assert.deepEqual(huge.data.datasets[0].data, [1, 1]);
    assert.equal(huge.options.plugins.tooltip.callbacks.label({dataIndex: 0}), 'Jan: ' + Number.MAX_VALUE);
    const tiny = config({...input, series: [{label: 'x', values: [0, Number.MIN_VALUE]}]});
    assert.deepEqual(tiny.data.datasets[0].data, [0, 1]);
    assert.deepEqual(input.series[0].values, [10, 20]);
  }
});
test('radar allows 3–32 categories including all-zero series and uses only a radial scale', () => {
  const radar = {...spec, type: 'radar', labels: ['A', 'B', 'C'], series: [{label: 'zero', values: [0, 0, 0]}]};
  const c = config(radar, '#abc', ['red']);
  assert.equal(c.type, 'radar');
  assert.deepEqual(Object.keys(c.options.scales), ['r']);
  assert.equal(c.options.scales.r.beginAtZero, true);
  assert.equal(c.options.scales.r.pointLabels.color, '#abc');
  assert.equal(c.data.datasets[0].borderColor, 'red');
  for (const n of [2, 3, 32, 33]) {
    const p = {...radar, labels: Array(n).fill('x'), series: [{label: 'a', values: Array(n).fill(0)}]};
    if (n >= 3 && n <= 32) assert.doesNotThrow(() => validate(p));
    else assert.throws(() => validate(p));
  }
  for (const values of [[-1, 0, 1], [Infinity, 0, 1], [NaN, 0, 1]]) {
    assert.throws(() => validate({...radar, series: [{label: 'bad', values}]}));
  }
});
test('canonical kinds reject aliases and discard model options for every mapping', () => {
  for (const type of ['line', 'bar', 'scatter', 'area', 'horizontal-bar', 'pie', 'doughnut', 'radar']) {
    const input = {...spec, type, labels: ['1', '2', '3'], series: [{label: 'v', values: [0, 1, 2]}], options: {onClick: 'evil()', indexAxis: 'evil'}, plugins: ['evil'], code: 'evil()'};
    assert.deepEqual(Object.keys(validate(input)).sort(), ['labels', 'series', 'title', 'type']);
    const c = config(input);
    assert.equal(c.options.onClick, undefined);
    assert.equal(c.options.indexAxis, type === 'horizontal-bar' ? 'y' : undefined);
    assert.equal(c.plugins, undefined);
  }
  for (const type of ['', 'Line', 'horizontalBar', 'donut', 'polarArea', 'bubble']) assert.throws(() => validate({...spec, type}));
  const full = {...spec, labels: Array(2000).fill('x'), series: [{label: 'v', values: Array(2000).fill(0)}]};
  assert.doesNotThrow(() => validate(full));
  assert.throws(() => validate({...full, labels: [...full.labels, 'x'], series: [{label: 'v', values: Array(2001).fill(0)}]}));
  assert.doesNotThrow(() => validate({...spec, series: Array(20).fill(spec.series[0])}));
  assert.throws(() => validate({...spec, series: Array(21).fill(spec.series[0])}));
});
test('streaming markdown exposes escaped plot JSON for the renderer', () => {
  const html = md.render('```gregal-plot\n' + JSON.stringify({...spec, title: '<script>bad</script>'}) + '\n```');
  assert.match(html, /lang-gregal-plot/);
  assert.match(html, /&lt;script&gt;/);
  assert.doesNotMatch(html, /<script>/);
});
