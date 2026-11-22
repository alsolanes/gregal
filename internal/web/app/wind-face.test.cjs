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
  // 16 especialistes més el cuiner i el del temps de la colla «Dia a dia».
  assert.equal(squadKeys.length, 18);
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

// Esperit de vent: the face with two wind tails. Android's WindFace mirrors
// the table and the hash, so these literal directions are the contract.
test('the wind direction comes from the wind rose by name, or from a stable hash of the id', () => {
  const { winds } = load();
  const rose = { Tramuntana:0, Gregal:45, Llevant:90, Xaloc:135, Migjorn:180, 'Garbí':225, Ponent:270, Mestral:315 };
  for (const [name, deg] of Object.entries(rose)) assert.equal(winds.direction({ id:'x', name }), deg, name);
  assert.equal(winds.direction({ id:'x', name:'  GARBI ' }), 225, 'case and accents do not matter');
  assert.equal(winds.direction({ id:'bit', name:'Bit' }), 225);
  assert.equal(winds.direction({ id:'teca', name:'Teca' }), 315);
  assert.equal(winds.direction({ id:'llevant-2', name:'Llevant 2' }), 135);
  assert.equal(winds.direction({ id:'espurna', name:'Espurna' }), 90);
  assert.equal(load().winds.direction({ id:'espurna', name:'Espurna' }), 90, 'the same id always gets the same direction');
  assert.equal(winds.direction({ name:'Lupa Ràpida' }), winds.direction({ id:'lupa-rapida', name:'Lupa Ràpida' }), 'the preview of a new wind points where the saved wind will');
});

test('with a theme every face uses its pack; in free mode each wind uses its own avatar', () => {
  const { winds } = load();
  const xaloc = { id:'xaloc', name:'Xaloc', color:'#EFA68F', style:'pso', avatar:'esperit' }, plain = { id:'ponent', name:'Ponent', color:'#EACF87', style:'pso' };
  const pack = svg => svg.getAttribute('data-avatar');
  assert.equal(pack(winds.face(xaloc)), null, 'Cares by default, whatever the wind chose');
  winds.setAvatarTheme('esperit');
  assert.equal(pack(winds.face(xaloc)), 'esperit');
  assert.equal(pack(winds.face(plain, true)), 'esperit');
  winds.setAvatarTheme('free');
  assert.equal(pack(winds.face(xaloc)), 'esperit');
  assert.equal(pack(winds.face(plain)), null, 'no avatar of its own: Cares');
  winds.setAvatarTheme('cares');
  assert.equal(pack(winds.face(xaloc)), null);
  winds.setAvatarTheme('bufador');
  assert.equal(winds.avatarTheme(), 'cares', 'an unknown theme falls back to Cares');
});

test('Esperit de vent points its tails to where the wind comes from and looks the other way', () => {
  const { winds } = load();
  winds.setAvatarTheme('esperit');
  const svg = winds.face({ id:'ponent', name:'Ponent', color:'#EACF87', style:'pso', specialty:'books' }, true);
  assert.equal(svg.getAttribute('data-kit'), null, 'the new packs draw no trade kit');
  const tails = svg.querySelector('.wf-tails');
  assert.ok(tails && tails.children.length >= 2, 'two tails');
  // Ponent blows from the west: the tails, drawn along +x, turn to point west.
  assert.match(tails.parent.attrs.transform, /rotate\(180\)$/);
  const head = svg.all().find(n => n.tag === 'circle' && /^url\(/.test(n.attrs.fill || ''));
  const eyes = svg.all().filter(n => n.tag === 'ellipse' && n.attrs.rx === '2.9');
  assert.equal(eyes.length, 2);
  assert.ok(+eyes[0].attrs.cx > +head.attrs.cx - 8, 'eyes look east, away from the tails');
  assert.equal(svg.getAttribute('data-mood'), 'idle');
});

test('Esperit de vent idles only with motion on; real work always moves the tails', () => {
  const still = load({ matchMedia:q => ({ matches:/reduce/.test(q) }) }).winds;
  still.setAvatarTheme('esperit');
  const w = { id:'xaloc', name:'Xaloc', color:'#EFA68F', style:'pso' };
  assert.doesNotMatch(still.face(w).getAttribute('class'), /wf-idle/, 'no idle animation under reduced motion');
  assert.equal(still.face(w, false, 'working').getAttribute('data-mood'), 'working', 'work is still shown');
  const lively = load({ matchMedia:() => ({ matches:false }) }).winds;
  lively.setAvatarTheme('esperit');
  assert.match(lively.face(w).getAttribute('class'), /wf-idle/);
  const css = fs.readFileSync(path.join(__dirname, 'winds.css'), 'utf8');
  assert.match(css, /\.winds-face\.wf-idle \.wf-tails\s*\{[^}]*animation:/, 'idle sway hangs on wf-idle');
  assert.match(css, /\[data-mood=working\][^{]*\.wf-tails[^{]*\{[^}]*animation:[^}]*!important/, 'working tails move even when motion is reduced');
});

// Cares que bufen: while a Cares face works it blows toward where its wind goes.
const breathOf = svg => svg.querySelector('.wf-breath');
const turnOf = breath => +/rotate\(([-\d.]+)\)/.exec(breath.all().find(n => /rotate/.test(n.attrs.transform || '')).attrs.transform)[1];
const lines = (breath, cls) => breath.querySelector('.' + cls).children.filter(n => n.tag === 'path');

test('a working Cares face blows three breath lines downwind, and two short ones for small faces', () => {
  const { winds } = load();
  const tramuntana = { id:'tramuntana', name:'Tramuntana', color:'#A8D8EA', style:'pso' };
  for (const mood of ['working', 'thinking', 'searching', 'reading', 'writing']) {
    const breath = breathOf(winds.face(tramuntana, false, mood));
    assert.ok(breath, mood + ' blows');
    assert.equal(lines(breath, 'wf-gust').length, 3);
    assert.equal(lines(breath, 'wf-puff').length, 2);
  }
  for (const mood of ['idle', 'done', 'failed', 'news', 'discussing']) assert.equal(breathOf(winds.face(tramuntana, false, mood)), null, mood + ' does not blow');
  // The lines are drawn along +x; Tramuntana blows from the north, so they turn to point south.
  assert.equal(turnOf(breathOf(winds.face(tramuntana, false, 'working'))) % 360, 90);
  assert.equal(turnOf(breathOf(winds.face({ id:'ponent', name:'Ponent', color:'#EACF87', style:'pso' }, true, 'working'))) % 360, 0, 'Ponent blows east');
  assert.equal(winds.face(tramuntana, false, 'working').querySelector('.wf-badge'), null, 'in Cares the breath replaces the cog');
  assert.ok(winds.face(tramuntana, false, 'searching').querySelector('.wf-badge'), 'the other work marks stay');
  winds.setAvatarTheme('esperit');
  const esperit = winds.face(tramuntana, false, 'working');
  assert.equal(breathOf(esperit), null, 'only Cares blows');
  assert.ok(esperit.querySelector('.wf-badge'), 'the other packs keep the cog');
  const css = fs.readFileSync(path.join(__dirname, 'winds.css'), 'utf8');
  assert.match(css, /\.winds-face \.wf-puff\s*\{\s*display:none;/, 'the short lines are for small faces only');
  const small = /:is\(([^)]*)\) \.winds-face \.wf-gust\s*\{\s*display:none;/.exec(css);
  assert.ok(small, 'small faces hide the long lines');
  for (const row of ['.vw-nav', '.vw-pick-card', '.vw-task-faces', '.vw-crew-chip']) assert.ok(small[1].split(',').includes(row), row + ' faces are under 28 px');
  assert.match(css, new RegExp(':is\\(' + small[1].replace(/[.()]/g, '\\$&') + '\\) \\.winds-face \\.wf-puff\\s*\\{\\s*display:inline;'));
});

test('the breath stays inside the face box plus a 10% margin, whatever the direction', () => {
  const { winds } = load();
  for (const name of ['Tramuntana', 'Gregal', 'Llevant', 'Xaloc', 'Migjorn', 'Garbí', 'Ponent', 'Mestral']) {
    const breath = breathOf(winds.face({ id:'w', name, color:'#A8D8EA', style:'pso' }, false, 'working'));
    const box = ['x', 'y', 'width', 'height'].map(k => +breath.attrs[k]);
    assert.deepEqual(box, [-8, -8, 96, 96], 'the 80 box plus 8 on every side');
    assert.equal(breath.attrs.viewBox, '-8 -8 96 96');
    const a = turnOf(breath) * Math.PI / 180;
    for (const cls of ['wf-gust', 'wf-puff']) for (const line of lines(breath, cls)) {
      const n = line.attrs.d.match(/-?[\d.]+/g).map(Number);
      for (let i = 0; i < n.length; i += 2) {
        const x = 40 + n[i] * Math.cos(a) - n[i + 1] * Math.sin(a), y = 40 + n[i] * Math.sin(a) + n[i + 1] * Math.cos(a);
        assert.ok(x >= -8 && x <= 88 && y >= -8 && y <= 88, `${name} ${cls} point ${x},${y} inside the box`);
        assert.ok(Math.hypot(x - 40, y - 40) >= 33, `${name} ${cls} leaves the face`);
      }
    }
  }
});

test('the breath moves only while the wind works and goes away when it is idle again', () => {
  const { winds } = load();
  const svg = winds.face({ id:'gregal', name:'Gregal', color:'#A8D8EA', style:'pso' }, false, 'working');
  assert.ok(breathOf(svg));
  winds.mood(svg, 'writing');
  assert.equal(svg.all().filter(n => (n.attrs.class || '') === 'wf-breath').length, 1, 'a new mood redraws the breath once');
  winds.mood(svg, 'idle');
  assert.equal(breathOf(svg), null);
  assert.equal(svg.querySelector('.wf-badge'), null);
  const css = fs.readFileSync(path.join(__dirname, 'winds.css'), 'utf8');
  const rule = /\.winds-face:is\(([^)]*)\) \.wf-breath path\s*\{([^}]*)\}/.exec(css);
  assert.ok(rule, 'the breath animation hangs on the work moods');
  for (const mood of ['thinking', 'searching', 'reading', 'writing', 'working']) assert.ok(rule[1].includes(`[data-mood=${mood}]`), mood);
  assert.match(rule[2], /animation:wf-blow [^;]*infinite;/);
  assert.doesNotMatch(rule[2], /!important/, 'under reduced motion it stays still, like the rest of the Cares face');
  assert.doesNotMatch(css, /\.winds-face \.wf-breath path\s*\{[^}]*animation/, 'no breath animation outside work');
  // Round caps grow each dash by the stroke width: the gap must be wider, or the dashes merge into a solid line.
  const [dash, gap] = /\.winds-face \.wf-gust path\s*\{\s*stroke-dasharray:([\d.]+) ([\d.]+);/.exec(css).slice(1).map(Number);
  const width = Math.max(...lines(breathOf(winds.face({ id:'gregal', name:'Gregal', color:'#A8D8EA', style:'pso' }, false, 'working')), 'wf-gust').map(n => +n.attrs['stroke-width']));
  assert.ok(gap > width, `gap ${gap} wider than the stroke ${width}`);
  assert.match(css, new RegExp(`@keyframes wf-blow \\{ from \\{ stroke-dashoffset:${dash + gap}; \\}`), 'one whole dash period per loop');
});

// Cap de vent blows away from where the wind comes from; the breath is drawn
// along +x, so turning it by bearing + 90 sends it to bearing + 180.
test('Cap de vent blows toward the opposite of the wind direction, with less detail below 28 px', () => {
  const { winds } = load();
  winds.setAvatarTheme('cap');
  const xaloc = { id:'xaloc', name:'Xaloc', color:'#EFA68F', style:'pso', specialty:'shopping' };
  const big = winds.face(xaloc, true);
  assert.equal(big.getAttribute('data-avatar'), 'cap');
  assert.equal(big.getAttribute('data-kit'), null, 'no trade kit');
  const breath = big.querySelector('.wf-cap-breath');
  assert.match(breath.attrs.transform, /rotate\(225\)$/, 'Xaloc comes from the SE and blows to the NW');
  assert.match(winds.face({ id:'ponent', name:'Ponent', color:'#EACF87' }).querySelector('.wf-cap-breath').attrs.transform, /rotate\(0\)$/, 'Ponent blows east');
  assert.equal(breath.children.length, 3, 'three breath lines');
  winds.mood(big, 'working'); winds.mood(big, 'idle');
  assert.ok(big.querySelector('.wf-cap-breath'), 'a mood change keeps the head blowing: Cares que bufen only redraws its own breath');
  const closedEyes = svg => svg.all().filter(n => n.tag === 'path' && !/wf-cap-breath/.test(n.parent.attrs.class || '')).length;
  assert.ok(closedEyes(big) > 0, 'closed eyes at full size');
  const small = winds.face(xaloc, false, 'idle', 20);
  assert.equal(small.querySelector('.wf-cap-breath').children.length, 2, 'the breath becomes two short lines');
  assert.equal(closedEyes(small), 0, 'no eyes or cheeks below 28 px');
  assert.equal(winds.face(xaloc, false, 'idle', 28).querySelector('.wf-cap-breath').children.length, 3, '28 px keeps the details');
});

test('Rosa de rellotge points its long tip in the wind direction, and only that point below 28 px', () => {
  const { winds } = load();
  winds.setAvatarTheme('rosa');
  const xaloc = { id:'xaloc', name:'Xaloc', color:'#EFA68F', style:'pso' };
  const big = winds.face(xaloc);
  assert.equal(big.getAttribute('data-avatar'), 'rosa');
  const point = big.querySelector('.wf-point');
  assert.match(point.parent.attrs.transform, /rotate\(135\)$/, 'the point is drawn to the north and turned to the SE');
  const paths = svg => svg.all().filter(n => n.tag === 'path').length;
  assert.equal(paths(big), 18, 'the wind point and the eight points of the rose');
  assert.equal(paths(winds.face(xaloc, false, 'idle', 24)), 2, 'only the wind point below 28 px');
});

test('Cap de vent and Rosa de rellotge move while working; idle only with motion on', () => {
  const w = { id:'xaloc', name:'Xaloc', color:'#EFA68F', style:'pso' };
  for (const pack of ['cap', 'rosa']) {
    const still = load({ matchMedia:q => ({ matches:/reduce/.test(q) }) }).winds;
    still.setAvatarTheme(pack);
    assert.doesNotMatch(still.face(w).getAttribute('class'), /wf-idle/, pack + ': no idle motion under reduced motion');
    const lively = load({ matchMedia:() => ({ matches:false }) }).winds;
    lively.setAvatarTheme(pack);
    assert.match(lively.face(w).getAttribute('class'), /wf-idle/, pack + ': idle motion when allowed');
  }
  const css = fs.readFileSync(path.join(__dirname, 'winds.css'), 'utf8');
  assert.match(css, /\.winds-face\.wf-idle \.wf-cap-breath path\s*\{[^}]*animation:/, 'the breath flows idle only with wf-idle');
  assert.match(css, /\.winds-face\.wf-idle \.wf-point\s*\{[^}]*animation:/, 'the point swings idle only with wf-idle');
  assert.match(css, /\[data-mood=working\][^{]*\.wf-cap-breath path[^{]*\{[^}]*animation:[^}]*!important/, 'working breath flows');
  assert.match(css, /\[data-mood=working\][^{]*\.wf-point[^{]*\{[^}]*animation:[^}]*!important/, 'working point swings');
});
