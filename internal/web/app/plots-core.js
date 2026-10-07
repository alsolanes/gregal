(function(root) {
  'use strict';
  function validate(spec) {
    if (!spec || typeof spec.title !== 'string' || !spec.title.trim() || new TextEncoder().encode(spec.title).length > 200) throw Error('Expected a title (1–200 bytes).');
    if (!['line', 'bar', 'scatter'].includes(spec.type)) throw Error('Expected line, bar or scatter.');
    if (!Array.isArray(spec.labels) || !spec.labels.length || spec.labels.length > 2000 || spec.labels.some(x => typeof x !== 'string' || new TextEncoder().encode(x).length > 200)) throw Error('Expected 1–2000 text labels.');
    if (!Array.isArray(spec.series) || !spec.series.length || spec.series.length > 20) throw Error('Expected 1–20 series.');
    for (const series of spec.series) {
      if (!series || typeof series.label !== 'string' || new TextEncoder().encode(series.label).length > 200 || !Array.isArray(series.values) || series.values.length !== spec.labels.length || series.values.some(x => typeof x !== 'number' || !Number.isFinite(x))) throw Error('Each series needs a label and one finite number per data label.');
    }
    if (spec.type === 'scatter' && spec.labels.some(x => { try { return typeof JSON.parse(x) !== 'number' || !Number.isFinite(JSON.parse(x)); } catch { return true; } })) throw Error('Scatter labels must be numeric x coordinates.');
    return { title: spec.title, type: spec.type, labels: [...spec.labels], series: spec.series.map(s => ({ label: s.label, values: [...s.values] })) };
  }
  function config(input, color = '#888') {
    const spec = validate(input);
    const colors = ['#5185ee', '#e89839', '#4bab88', '#c068cc', '#df6475', '#36a9c3'];
    return { type: spec.type, data: { labels: spec.labels, datasets: spec.series.map((s, i) => ({ label: s.label, data: spec.type === 'scatter' ? s.values.map((y, j) => ({x: Number(spec.labels[j]), y})) : s.values, borderColor: colors[i % colors.length], backgroundColor: colors[i % colors.length], borderWidth: 2, pointRadius: spec.labels.length > 100 ? 0 : 3 })) }, options: { responsive: true, maintainAspectRatio: false, animation: false, plugins: { legend: { labels: { color } } }, scales: { x: { ticks: { color } }, y: { ticks: { color } } } } };
  }
  root.gregalPlotsCore = { validate, config };
  if (typeof module !== 'undefined') module.exports = root.gregalPlotsCore;
})(globalThis);
