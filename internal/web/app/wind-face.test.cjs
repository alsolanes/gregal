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
// Every wind stands on its feet now; the trade kit is what sets specialists apart.
const parts = svg => svg.all().filter(n => !/wf-foot/.test(n.attrs.class || '')).length;
const squadKeys = [...fs.readFileSync(path.join(__dirname, '..', 'winds_squads.go'), 'utf8').matchAll(/^\t\t\{"([a-z]+)", "/gm)].map(m => m[1]);

test('every squad specialist is a mascot with a trade tool; personal winds keep the plain face', () => {
  const { winds } = load();
  const own = winds.face({ id:'mine', name:'Tramuntana', color:'#A8CDE2', style:'pso' }, true);
  assert.equal(squadKeys.length, 16);
  for (const specialty of squadKeys) {
    const svg = winds.face({ id:specialty, specialty, color:'#A8CDE2', style:'pso' }, true);
    assert.equal(svg.getAttribute('data-kit'), specialty);
    assert.ok(parts(svg) >= parts(own) + 3, specialty + ' wears or carries the tool of its trade');
  }
  assert.equal(own.getAttribute('data-kit'), null);
  assert.equal(groups(own).includes('wf-prop'), false);
  assert.equal(own.getAttribute('data-mood'), 'idle');
});

test('the tool is for the whole figure: a small face stays a clean face', () => {
  const { winds } = load();
  const w = { id:'front', specialty:'frontend', color:'#A8D8EA', style:'pso' };
  const plain = parts(winds.face({ id:'mine', color:'#A8D8EA', style:'pso' }));
  for (const specialty of squadKeys) {
    const small = winds.face({ id:specialty, specialty, color:'#A8D8EA', style:'pso' });
    assert.equal(small.getAttribute('data-kit'), specialty);
    assert.equal(parts(small), plain, specialty + ' is told apart by colour and face at list size');
  }
  const hero = winds.face(w, true), scene = winds.face(w, 'figure');
  assert.ok(groups(hero).includes('wf-prop') && groups(scene).includes('wf-prop'));
  assert.ok(groups(hero).includes('wf-ground'));
  // A scene draws its own ground and sizes the figure itself.
  assert.equal(groups(scene).includes('wf-ground'), false);
  assert.equal(scene.getAttribute('class'), 'winds-face');
  assert.equal(hero.getAttribute('class'), 'winds-face winds-face-large');
});

test('the new trade kits dress the whole figure with their hat or tool', () => {
  const { winds } = load();
  const own = parts(winds.face({ id:'mine', color:'#A8CDE2', style:'pso' }, 'figure'));
  const worn = { cooking:['head', 'prop'], weather:['prop'], travel:['prop'], fitness:['head', 'prop'], finance:['prop'], movies:['head', 'prop'] };
  for (const [kit, pieces] of Object.entries(worn)) {
    const svg = winds.face({ id:kit, kit, color:'#A8CDE2', style:'pso' }, 'figure');
    assert.equal(svg.getAttribute('data-kit'), kit);
    assert.ok(groups(svg).includes('wf-prop'), kit + ' holds the tool of its trade');
    assert.ok(parts(svg) >= own + 4, kit + ' draws more than a plain figure');
    if (pieces.includes('head')) assert.ok(parts(svg) > own + svg.all().find(n => n.attrs.class === 'wf-prop').all().length + 1, kit + ' also wears something on its head');
    const small = winds.face({ id:kit, kit, color:'#A8CDE2', style:'pso' });
    assert.equal(small.getAttribute('data-kit'), kit);
    assert.equal(groups(small).includes('wf-prop'), false, kit + ' stays a clean face at list size');
  }
});

test('the gust shape curls on top instead of wearing a fringe', () => {
  const { winds } = load();
  for (const large of [false, true]) {
    assert.ok(groups(winds.face({ id:'a', color:'#A8D8EA', style:'bss' }, large)).includes('wf-curl'));
    assert.equal(groups(winds.face({ id:'b', color:'#A8D8EA', style:'bso' }, large)).includes('wf-curl'), false);
    assert.equal(groups(winds.face({ id:'c', color:'#A8D8EA', style:'bsn' }, large)).includes('wf-curl'), false);
  }
});

test('Android draws the same specialties as the web', () => {
  const kotlin = fs.readFileSync(path.join(__dirname, '..', '..', '..', 'android', 'app', 'src', 'main', 'java', 'xyz', 'solanes', 'gregal', 'WindFace.kt'), 'utf8');
  const android = [...kotlin.matchAll(/^\s+"([a-z]+)" to MascotKit\(/gm)].map(m => m[1]).sort();
  const source = fs.readFileSync(path.join(__dirname, 'winds.js'), 'utf8');
  const block = source.match(/const kits = \{[\s\S]*?\n  \};/)[0];
  const web = [...block.matchAll(/^    ([a-z]+): \{/gm)].map(m => m[1]).sort();
  for (const specialty of squadKeys) assert.ok(web.includes(specialty), specialty + ' has a web kit');
  assert.deepEqual(android, web);
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
