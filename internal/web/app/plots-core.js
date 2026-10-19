(function(root) {
  'use strict';
  function validate(spec) {
    if (!spec || typeof spec.title !== 'string' || !spec.title.trim() || new TextEncoder().encode(spec.title).length > 200) throw Error('Expected a title (1–200 bytes).');
    if (!['line', 'bar', 'scatter', 'area', 'horizontal-bar', 'pie', 'doughnut', 'radar'].includes(spec.type)) throw Error('Expected line, bar, scatter, area, horizontal-bar, pie, doughnut or radar.');
    if (!Array.isArray(spec.labels) || !spec.labels.length || spec.labels.length > 2000 || spec.labels.some(x => typeof x !== 'string' || new TextEncoder().encode(x).length > 200)) throw Error('Expected 1–2000 text labels.');
    if (!Array.isArray(spec.series) || !spec.series.length || spec.series.length > 20) throw Error('Expected 1–20 series.');
    for (const series of spec.series) {
      if (!series || typeof series.label !== 'string' || new TextEncoder().encode(series.label).length > 200 || !Array.isArray(series.values) || series.values.length !== spec.labels.length || series.values.some(x => typeof x !== 'number' || !Number.isFinite(x))) throw Error('Each series needs a label and one finite number per data label.');
    }
    if (spec.type === 'scatter' && spec.labels.some(x => { try { return typeof JSON.parse(x) !== 'number' || !Number.isFinite(JSON.parse(x)); } catch { return true; } })) throw Error('Scatter labels must be numeric x coordinates.');
    if (spec.type === 'pie' || spec.type === 'doughnut') {
      if (spec.series.length !== 1 || spec.series[0].values.some(x => x < 0) || !spec.series[0].values.some(x => x > 0)) throw Error('Pie and doughnut require one nonnegative series with at least one positive value.');
    }
    if (spec.type === 'radar' && (spec.labels.length < 3 || spec.labels.length > 32 || spec.series.some(s => s.values.some(x => x < 0)))) throw Error('Radar requires 3–32 categories and nonnegative values.');
    return { title: spec.title, type: spec.type, labels: [...spec.labels], series: spec.series.map(s => ({ label: s.label, values: [...s.values] })) };
  }
  // palette: els colors de les sèries. Un branding en pot portar de propis
  // (--chart-1… al seu bloc :root); si no, els de sempre.
  const DEFAULT_COLORS = ['#5185ee', '#e89839', '#4bab88', '#c068cc', '#df6475', '#36a9c3'];
  function config(input, color = '#888', palette) {
    const spec = validate(input);
    const colors = Array.isArray(palette) && palette.length ? palette : DEFAULT_COLORS;
    const area = spec.type === 'area';
    const horizontal = spec.type === 'horizontal-bar';
    const circular = spec.type === 'pie' || spec.type === 'doughnut';
    // Normalize render-only slices before Chart.js sums them; preserve raw JSON.
    const sliceMax = circular ? Math.max(...spec.series[0].values) : 1;
    const sliceDivisor = circular && (sliceMax >= Number.MAX_VALUE / spec.labels.length || sliceMax < 2 ** -1022) ? sliceMax : 1;
    const sliceColors = spec.labels.map((_, i) => colors[i % colors.length]);
    const datasets = spec.series.map((s, i) => ({
      label: s.label,
      data: circular ? s.values.map(x => x / sliceDivisor) : spec.type === 'scatter' ? s.values.map((y, j) => ({x: Number(spec.labels[j]), y})) : s.values,
      borderColor: circular ? sliceColors : colors[i % colors.length], backgroundColor: circular ? sliceColors : colors[i % colors.length],
      borderWidth: 2, pointRadius: spec.labels.length > 100 ? 0 : 3,
      ...(area ? {fill: 'origin'} : {})
    }));
    const scales = circular ? {} : spec.type === 'radar' ? {r: {beginAtZero: true, ticks: {color}, pointLabels: {color}}} : {x: {ticks: {color}}, y: {ticks: {color}}};
    if (area) scales.y.beginAtZero = true;
    if (horizontal) scales.x.beginAtZero = true;
    return {type: area ? 'line' : horizontal ? 'bar' : spec.type, data: {labels: spec.labels, datasets}, options: {
      responsive: true, maintainAspectRatio: false, animation: false,
      ...(horizontal ? {indexAxis: 'y'} : {}),
      plugins: {legend: {labels: {color}}, ...(circular ? {tooltip: {callbacks: {
        // Trusted local code only: never copy callbacks/options from input JSON.
        label: context => `${spec.labels[context.dataIndex]}: ${spec.series[0].values[context.dataIndex]}`
      }}} : {})}, scales
    }};
  }
  root.gregalPlotsCore = { validate, config };
  if (typeof module !== 'undefined') module.exports = root.gregalPlotsCore;
})(globalThis);
