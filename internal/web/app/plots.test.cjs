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
test('streaming markdown exposes escaped plot JSON for the renderer', () => {
  const html = md.render('```gregal-plot\n' + JSON.stringify({...spec, title: '<script>bad</script>'}) + '\n```');
  assert.match(html, /lang-gregal-plot/);
  assert.match(html, /&lt;script&gt;/);
  assert.doesNotMatch(html, /<script>/);
});
