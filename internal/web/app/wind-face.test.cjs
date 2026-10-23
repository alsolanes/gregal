const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

// Just enough SVG DOM for face(): elements, attributes, children and style.
class El {
  constructor(tag) { this.tag = tag; this.attrs = {}; this.children = []; this.style = { vars:{}, setProperty(k, v) { this.vars[k] = v; } }; }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  removeAttribute(k) { delete this.attrs[k]; }
  getAttribute(k) { return this.attrs[k] ?? null; }
  append(...nodes) { for (const n of nodes) { n.parent = this; this.children.push(n); } }
  remove() { this.parent.children = this.parent.children.filter(n => n !== this); }
  all() { return this.children.flatMap(n => [n, ...n.all()]); }
  querySelector(sel) { const classes = sel.split(',').map(s => s.trim().slice(1)); return this.all().find(n => classes.some(c => (n.attrs.class || '').split(' ').includes(c))) || null; }
}
function load(extra = {}) {
  let now = 1000;
  const window = { document:{ createElementNS:(_, tag) => new El(tag), createElement:tag => new El(tag) }, performance:{ now:() => now }, ...extra };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'winds.js'), 'utf8'), { window });
  return { winds:window.gregalWinds, tick:ms => { now += ms; } };
}
const groups = svg => svg.all().map(n => n.attrs.class).filter(Boolean);
const squadKeys = [...fs.readFileSync(path.join(__dirname, '..', 'winds_squads.go'), 'utf8').matchAll(/^\t\t\{"([a-z]+)", "/gm)].map(m => m[1]);

test('every squad specialist is a mascot with a trade tool; personal winds keep the plain face', () => {
  const { winds } = load();
  const own = winds.face({ id:'mine', name:'Tramuntana', color:'#A8CDE2', style:'pso' });
  assert.equal(squadKeys.length, 16);
  for (const specialty of squadKeys) {
    const svg = winds.face({ id:specialty, specialty, color:'#A8CDE2', style:'pso' });
    assert.equal(svg.getAttribute('data-kit'), specialty);
    assert.ok(svg.all().length >= own.all().length + 4, specialty + ' wears or carries the tool of its trade');
  }
  assert.equal(own.getAttribute('data-kit'), null);
  assert.equal(groups(own).includes('wf-prop'), false);
  assert.equal(own.getAttribute('data-mood'), 'idle');
});

test('Android draws the same specialties as the web', () => {
  const kotlin = fs.readFileSync(path.join(__dirname, '..', '..', '..', 'android', 'app', 'src', 'main', 'java', 'xyz', 'solanes', 'gregal', 'WindFace.kt'), 'utf8');
  const android = [...kotlin.matchAll(/^\s+"([a-z]+)" to MascotKit\(/gm)].map(m => m[1]).sort();
  assert.deepEqual(android, [...squadKeys].sort());
});

test('moods come only from observed chat phases and live step state', () => {
  const { winds } = load();
  const cases = [
    [{ phase:'thinking' }, 'thinking'], [{ phase:'searching' }, 'searching'], [{ phase:'scheduling' }, 'working'],
    [{ phase:'thinking', text:'Hola' }, 'writing'], [{ state:'waiting' }, 'idle'], [{ state:'cancelled' }, 'idle'],
    [{ state:'working', activity:'read' }, 'reading'], [{ state:'working', activity:'grep' }, 'searching'],
    [{ state:'working', activity:'write' }, 'writing'], [{ state:'working', activity:'bash' }, 'working'],
    [{ state:'working', activity:'plan' }, 'thinking'], [{ state:'working' }, 'working'],
    [{ state:'completed', activity:'write' }, 'done'], [{ state:'failed' }, 'failed'], [{}, 'idle'],
    [{ state:'working', activity:'discussion' }, 'discussing'], [{ phase:'thinking', voices:2 }, 'discussing'],
    [{ phase:'thinking', voices:2, text:'Jo ho veig diferent' }, 'discussing'], [{ phase:'thinking', voices:1, text:'Hola' }, 'writing'],
  ];
  for (const [work, mood] of cases) assert.equal(winds.moodOf(work), mood, JSON.stringify(work));
});

test('a redrawn face keeps its loop phase and plays the done hop once per transition', () => {
  const { winds, tick } = load();
  const w = { id:'front', specialty:'frontend', color:'#A8D8EA', style:'bss' };
  const first = winds.face(w, false, 'working');
  tick(370);
  const again = winds.face(w, false, 'working');
  const phase = svg => Number.parseFloat(svg.style.vars['--wf-phase']);
  assert.ok(Math.abs((phase(first) - phase(again)) - 0.37) < 0.02 || Math.abs(phase(first) - phase(again) - 0.37 + 12) < 0.02);
  const done = winds.face(w, false, 'done');
  assert.equal(done.style.vars['--wf-since'], '0.00s');
  assert.ok(done.querySelector('.wf-badge'));
  tick(5000);
  assert.equal(winds.face(w, false, 'done').style.vars['--wf-since'], '-5.00s');
  winds.face(w, false, 'working');
  assert.equal(winds.face(w, false, 'done').style.vars['--wf-since'], '0.00s');
  assert.equal(winds.face(w, false, 'idle').querySelector('.wf-badge'), null);
});

test('a proactive voice marks its wind until its chat is opened', async () => {
  const ts = Math.floor(Date.now() / 1000), store = {}, calls = [];
  const api = async url => { calls.push(url); return { ok:true, json:async () => ({ novetats:[{ ts, wind:'marinada', name:'Marinada', source:'remind', text:'Beu aigua' }] }) }; };
  const localStorage = { getItem:k => store[k] ?? null, setItem:(k, v) => { store[k] = v; } };
  const { winds } = load({ api, localStorage });
  winds.watchNews();
  for (let i = 0; i < 5; i++) await new Promise(r => setImmediate(r));
  assert.match(calls[0], /^\/api\/winds\/novetats\?des_de=\d+$/);
  const svg = winds.face({ id:'marinada', specialty:'shopping', color:'#F4B6C2', style:'bso' });
  assert.equal(svg.getAttribute('data-mood'), 'news');
  assert.equal(svg.getAttribute('role'), 'img');
  assert.ok(svg.getAttribute('aria-label'));
  assert.ok(svg.querySelector('.wf-badge'));
  assert.equal(winds.face({ id:'ponent', color:'#EACF87', style:'mno' }).getAttribute('data-mood'), 'idle');
  // Real work still wins over the news bubble.
  assert.equal(winds.face({ id:'marinada', color:'#F4B6C2', style:'bso' }, false, 'searching').getAttribute('data-mood'), 'searching');
  winds.seen('marinada');
  const after = winds.face({ id:'marinada', specialty:'shopping', color:'#F4B6C2', style:'bso' });
  assert.equal(after.getAttribute('data-mood'), 'idle');
  assert.equal(after.getAttribute('aria-hidden'), 'true');
  assert.ok(JSON.parse(store['gregal.winds.newsSeen']).marinada >= ts);
});
