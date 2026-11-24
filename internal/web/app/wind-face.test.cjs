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
  const frames = [];
  const window = { document:{ createElementNS:(_, tag) => new El(tag), createElement:tag => new El(tag) }, performance:{ now:() => now }, requestAnimationFrame:fn => frames.push(fn), ...extra };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'winds.js'), 'utf8'), { window });
  return { winds:window.gregalWinds, tick:ms => { now += ms; }, frames };
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

// 113: a clay face on the page (characters.js) follows the news like a drawn one.
test('news reaches the clay faces already on the page, and leaves them when read', async () => {
  const ts = Math.floor(Date.now() / 1000), store = {}, moods = [];
  const api = async () => ({ ok:true, json:async () => ({ novetats:[{ ts, wind:'marinada', name:'Marinada', source:'remind', text:'Beu aigua' }] }) });
  const clay = Object.assign(new El('canvas'), { tagName:'CANVAS' });
  clay.setAttribute('class', 'winds-face winds-face-large winds-clay'); clay.setAttribute('data-wind', 'marinada'); clay.setAttribute('data-mood', 'idle');
  const gregalCharacters = { mood(el, mood) { moods.push(mood); el.setAttribute('data-mood', mood); return el; } };
  const document = { createElementNS:(_, tag) => new El(tag), createElement:tag => new El(tag), querySelectorAll:() => [clay] };
  const { winds } = load({ api, gregalCharacters, document, localStorage:{ getItem:k => store[k] ?? null, setItem:(k, v) => { store[k] = v; } } });
  winds.watchNews();
  for (let i = 0; i < 5; i++) await new Promise(r => setImmediate(r));
  assert.deepEqual(moods, ['news']);
  winds.seen('marinada');
  assert.deepEqual(moods, ['news', 'idle']);
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

// Volume: every gradient a face paints with is its own, so many faces on one
// page never borrow each other's light.
const has = (svg, cls) => !!svg.querySelector('.' + cls);
function ownGradients(svg) {
  const ids = svg.all().filter(n => /Gradient$/.test(n.tag)).map(n => n.attrs.id);
  const used = svg.all().flatMap(n => ['fill', 'stroke'].map(k => /^url\(#(.+)\)$/.exec(n.attrs[k] || '')?.[1]).filter(Boolean));
  for (const id of used) assert.ok(ids.includes(id), id + ' is defined in the same face');
  return ids;
}

test('Esperit, Cap de vent and Rosa de rellotge are lit like Cares: shading, rim, highlight and a contact shadow when large', () => {
  const { winds } = load();
  const w = { id:'xaloc', name:'Xaloc', color:'#EFA68F', style:'pso' };
  for (const pack of ['esperit', 'cap', 'rosa']) {
    winds.setAvatarTheme(pack);
    const big = winds.face(w, true), small = winds.face(w, false, 'idle', 24);
    for (const [svg, size] of [[big, 'large'], [small, '24 px']]) {
      assert.ok(svg.all().some(n => n.tag === 'radialGradient'), `${pack} ${size}: a radial gradient lit from the top left`);
      assert.ok(has(svg, 'wf-shade'), `${pack} ${size}: a core shadow away from the light`);
      assert.ok(has(svg, 'wf-rim'), `${pack} ${size}: a rim light`);
      assert.ok(has(svg, 'wf-spec'), `${pack} ${size}: a specular highlight`);
    }
    assert.ok(has(big, 'wf-ground'), pack + ': a soft contact shadow under the large figure');
    assert.ok(has(big, 'wf-bounce'), pack + ': a bounce of its own colour, so the shadow reads on a dark page');
    assert.equal(has(small, 'wf-sheen'), false, pack + ': no broad sheen at 24 px');
    assert.equal(has(small, 'wf-ground'), false, pack + ': no ground under a list face');
    const a = ownGradients(big), b = ownGradients(winds.face(w, true));
    assert.equal(a.filter(id => b.includes(id)).length, 0, pack + ': two faces share no gradient id');
  }
  winds.setAvatarTheme('rosa');
  assert.ok(has(winds.face(w, true), 'wf-glass'), 'the whole Rosa figure has a curved reflection on its glass');
});

test('Cares gets a second shading layer and ambient occlusion at its base, at every size', () => {
  const { winds } = load();
  const w = { id:'xaloc', name:'Xaloc', color:'#EFA68F', style:'pso' };
  for (const svg of [winds.face(w), winds.face(w, true), winds.face({ ...w, style:'psn' }, true)]) {
    assert.ok(has(svg, 'wf-shade'), 'a second shading layer');
    assert.ok(has(svg, 'wf-ao'), 'ambient occlusion where the face meets its feet');
    ownGradients(svg);
  }
});

// Mapa de vent: streaks on a canvas. Enough of a browser for it: a 2D context
// that ignores the drawing, an IntersectionObserver the test drives and a
// requestAnimationFrame that only queues; `motion(reduce)` flips the
// reduced-motion preference like the system setting does.
function mapLoad({ reduce = false } = {}) {
  const frames = [], observers = [], ctx = { fillRect() {}, beginPath() {}, moveTo() {}, lineTo() {}, stroke() {} };
  const document = { hidden:false, addEventListener() {}, createElementNS:(_, tag) => new El(tag), createElement:tag => Object.assign(new El(tag), tag === 'canvas' ? { getContext:() => ctx } : {}) };
  class IntersectionObserver { constructor(cb) { this.cb = cb; observers.push(this); } observe() {} unobserve() {} }
  const listeners = [], reduced = { matches:reduce, addEventListener:(type, fn) => listeners.push(fn) };
  const matchMedia = q => /reduce/.test(q) ? reduced : { matches:false, addEventListener() {} };
  const { winds } = load({ document, IntersectionObserver, matchMedia, requestAnimationFrame:fn => frames.push(fn) });
  winds.setAvatarTheme('mapa');
  const show = (svg, on = true) => observers.forEach(o => o.cb([{ target:svg, isIntersecting:on, boundingClientRect:{ width:40 } }]));
  const motion = on => { reduced.matches = !on; listeners.forEach(fn => fn({ matches:!on })); };
  return { winds, frames, show, document, motion };
}
const ponent = { id:'ponent', name:'Ponent', color:'#EACF87', style:'pso', specialty:'books' };
const picture = (winds, w, steps) => { let st = winds.mapStart(w); for (let i = 0; i < steps; i++) st = winds.mapStep(st, false); return JSON.stringify(st); };

test('Mapa de vent is drawn on a canvas inside the face, with a theme or as a wind\'s own avatar', () => {
  const { winds } = mapLoad();
  const svg = winds.face(ponent, true);
  assert.equal(svg.getAttribute('data-avatar'), 'mapa');
  assert.equal(svg.getAttribute('data-kit'), null, 'the new packs draw no trade kit');
  assert.ok(svg.all().some(n => n.tag === 'canvas'));
  assert.equal(winds.face(ponent, false, 'working').getAttribute('data-mood'), 'working');
  winds.setAvatarTheme('free');
  assert.equal(winds.face({ ...ponent, avatar:'mapa' }).getAttribute('data-avatar'), 'mapa');
  assert.equal(winds.face(ponent).getAttribute('data-avatar'), null);
});

test('the streaks blow away from where the wind comes from, faster while it works, and the same wind gets the same picture', () => {
  const { winds } = mapLoad();
  // Ponent blows from the west, so its streaks run east: +x on the canvas.
  let st = winds.mapStart(ponent);
  const east = Math.PI / 2, sum = [0, 0];
  for (let i = 0; i < 40; i++) {
    st = winds.mapStep(st, false);
    for (const [x0, y0, x1, y1] of st.lines) {
      const ang = Math.atan2(x1 - x0, y0 - y1);
      assert.ok(Math.abs(ang - east) <= .61, 'every streak within the swirl of its course: ' + ang);
      sum[0] += x1 - x0; sum[1] += y1 - y0;
    }
  }
  assert.ok(Math.abs(Math.atan2(sum[0], -sum[1]) - east) < .1, 'on average due east');
  // Tramuntana blows from the north: down the canvas.
  const [x0, y0, x1, y1] = winds.mapStep(winds.mapStart({ id:'t', name:'Tramuntana' }), false).lines[0];
  assert.ok(Math.abs(Math.atan2(x1 - x0, y0 - y1)) > Math.PI - .61);
  const length = busy => winds.mapStep(winds.mapStart(ponent), busy).lines.reduce((total, [a, b, c, d]) => total + Math.hypot(c - a, d - b), 0);
  assert.ok(length(true) > 2 * length(false), 'working moves faster');
  assert.equal(picture(winds, ponent, 200), picture(mapLoad().winds, ponent, 200), 'the same wind always gets the same picture');
  assert.notEqual(picture(winds, ponent, 200), picture(winds, { id:'llevant-2' }, 200));
  assert.equal(picture(winds, { name:'Lupa Ràpida' }, 20), picture(winds, { id:'lupa-rapida', name:'Lupa Ràpida' }, 20), 'the preview of a new wind is the picture the saved wind gets');
});

test('under reduced motion a map is one still frame, even while its wind works; motion and being on screen move it', () => {
  const still = mapLoad({ reduce:true });
  const idle = still.winds.face(ponent);
  still.show(idle);
  assert.equal(still.frames.length, 0, 'idle under reduced motion: a still frame, no loop');
  const busy = still.winds.face(ponent, false, 'working');
  still.show(busy);
  still.winds.mood(idle, 'searching');
  assert.equal(still.frames.length, 0, 'working under reduced motion: still a single frame');
  const lively = mapLoad();
  const work = lively.winds.face(ponent, false, 'working');
  assert.equal(lively.frames.length, 0, 'not on screen yet');
  lively.show(work);
  assert.equal(lively.frames.length, 1, 'with motion allowed the map moves');
  lively.show(lively.winds.face(ponent));
  assert.equal(lively.frames.length, 1, 'one shared loop for every face');
  lively.frames.shift()();
  assert.equal(lively.frames.length, 1, 'the loop goes on while a face moves');
  lively.document.hidden = true;
  lively.frames.shift()();
  assert.equal(lively.frames.length, 0, 'a hidden tab stops the loop');
});

test('a map follows the reduced-motion setting when it changes', () => {
  const m = mapLoad();
  m.show(m.winds.face(ponent, false, 'working'));
  assert.equal(m.frames.length, 1);
  m.motion(false);
  m.frames.shift()();
  assert.equal(m.frames.length, 0, 'turning reduced motion on stops the loop on the frame it shows');
  m.motion(true);
  assert.equal(m.frames.length, 1, 'turning it off again starts the map moving');
});

// 101: what a block of CSS holds, braces matched, and which properties a keyframe animates.
function cssBlock(css, at) {
  if (at < 0) return '';
  const start = css.indexOf('{', at);
  let depth = 0;
  for (let i = start; i < css.length; i++) {
    if (css[i] === '{') depth++;
    else if (css[i] === '}' && --depth === 0) return css.slice(start + 1, i);
  }
  return '';
}
const winds101 = () => fs.readFileSync(path.join(__dirname, 'winds.css'), 'utf8');
const motionBlocks = css => [...css.matchAll(/@media \(prefers-reduced-motion:no-preference\)/g)].map(m => cssBlock(css, m.index));
const outsideMotion = css => motionBlocks(css).reduce((rest, b) => rest.replace(b, ''), css);
const keyframeProps = (css, name) => [...cssBlock(css, css.indexOf('@keyframes ' + name + ' ')).matchAll(/([a-z-]+)\s*:/g)].map(m => m[1]);

test('idle faces blink and glance, each wind its own way, and reduced motion stills them', () => {
  const css = winds101(), motion = motionBlocks(css).join(' ');
  assert.match(motion, /\.winds-face \.wf-eyes \{ animation:wf-blink /, 'faces blink when motion is allowed');
  assert.match(motion, /\.winds-face\[data-mood=idle\] \.wf-gaze \{ animation:wf-look /, 'idle faces glance around');
  assert.equal(outsideMotion(css).includes('animation:wf-blink'), false, 'no blink outside the motion-allowed rules');
  assert.match(cssBlock(css, css.lastIndexOf('@media (prefers-reduced-motion:reduce)')), /\.winds-face, \.winds-face \* \{ animation:none !important;/);
  const { winds } = load();
  const look = id => winds.face({ id, color:'#A8D8EA', style:'pso' }).style.vars['--wf-look'];
  const looks = new Set(['xaloc', 'mestral', 'garbi', 'ponent', 'llevant'].map(look));
  assert.ok(looks.size > 1, 'winds do not all glance the same way');
  assert.equal(look('xaloc'), look('xaloc'), 'a redrawn face keeps its own glance');
  assert.match(look('xaloc'), /^-?\d(\.\d+)?px$/);
});

test('a face squashes when hovered or pressed and springs back, only with motion allowed', () => {
  const css = winds101(), motion = motionBlocks(css).join(' ');
  assert.match(motion, /\.winds-face:hover[^{]*\{ animation:wf-squish /);
  assert.match(motion, /\.winds-face:active[^{]*\{[^}]*transform:/);
  assert.equal(outsideMotion(css).includes('wf-squish .'), false);
});

test('the faces animate transform and opacity only', () => {
  const css = winds101();
  for (const name of ['wf-blink', 'wf-look', 'wf-squish']) {
    const props = keyframeProps(css, name);
    assert.ok(props.length, name + ' exists');
    assert.deepEqual(props.filter(p => !['transform', 'opacity'].includes(p)), [], name);
  }
});

// 110: the redesigned Cares. A pose per mood (body, gaze, eyelids) that springs
// from the last one, a motion signature per mood, and eyes that carry the mood.
const poseVars = ['--wf-x', '--wf-y', '--wf-tilt', '--wf-sx', '--wf-sy', '--wf-gx', '--wf-gy', '--wf-lid', '--wf-brow', '--wf-smile'];
const poseOf = svg => Object.fromEntries(poseVars.map(v => [v, Number.parseFloat(svg.style.vars[v])]));
const allMoods = ['idle', 'thinking', 'searching', 'reading', 'writing', 'working', 'discussing', 'waiting', 'done', 'failed', 'news'];
// The keyframes a mood plays on the Cares figure, gaze and feet, from winds.css.
function moodLoops(css, mood) {
  const names = new Set();
  for (const m of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    const selector = m[1].trim();
    if (!selector.includes('[data-mood=' + mood + ']') || !/\.wf-(figure|gaze|foot)/.test(selector)) continue;
    for (const a of m[2].matchAll(/animation:([^;]+)/g)) for (const part of a[1].split(',')) names.add(part.trim().split(/\s+/)[0]);
  }
  return [...names].sort();
}

test('each mood has its own pose and its own motion signature', () => {
  const { winds } = load(), css = winds101();
  const w = { id:'tramuntana', name:'Tramuntana', color:'#A8CDE2', style:'pso' };
  const poses = new Map(), loops = new Map();
  for (const mood of allMoods) {
    const svg = winds.face({ ...w, id:'pose-' + mood }, true, mood);
    assert.equal(svg.getAttribute('data-mood'), mood);
    const pose = poseOf(svg), moves = moodLoops(css, mood);
    for (const v of poseVars) assert.ok(Number.isFinite(pose[v]), mood + ' sets ' + v);
    assert.ok(moves.length, mood + ' moves in its own way');
    for (const [other, seen] of poses) assert.notDeepEqual(pose, seen, mood + ' and ' + other + ' share a pose');
    for (const [other, seen] of loops) assert.notDeepEqual(moves, seen, mood + ' and ' + other + ' share a motion');
    poses.set(mood, pose); loops.set(mood, moves);
  }
  const p = mood => poses.get(mood);
  assert.ok(p('thinking')['--wf-gy'] < 0, 'thinking looks up');
  assert.ok(p('working')['--wf-lid'] > p('idle')['--wf-lid'] && p('working')['--wf-brow'] > 0, 'working squints, determined');
  assert.ok(p('failed')['--wf-y'] > 0 && p('failed')['--wf-sy'] < 1 && p('failed')['--wf-brow'] < 0, 'failed droops, sad lids');
  assert.ok(p('done')['--wf-smile'] > .4, 'done smiles with its eyes');
  assert.equal(p('waiting')['--wf-lid'], 0, 'waiting looks at you with wide eyes');
  assert.ok(p('waiting')['--wf-tilt'] !== 0 && p('waiting')['--wf-gy'] > 0, 'waiting leans and looks out at the person');
  assert.ok(loops.get('searching').includes('wf-scan') && loops.get('reading').includes('wf-read'), 'searching and reading scan with the eyes');
});

test('every Cares eye has an upper lid and a smiling lower lid, at every size and eye style', () => {
  const { winds } = load();
  for (const eyes of 'pbem') for (const large of [false, true]) {
    const svg = winds.face({ id:'lid' + eyes, color:'#BFCFA4', style:eyes + 'so' }, large);
    const lids = cls => svg.all().filter(n => new RegExp('\\b' + cls + '\\b').test(n.attrs.class || ''));
    assert.equal(lids('wf-lid').length, 2, eyes + ' has two upper lids');
    assert.equal(lids('wf-smile').length, 2, eyes + ' has two lower lids');
    // Lids are cut to the eye, so they never show outside it.
    for (const lid of [...lids('wf-lid'), ...lids('wf-smile')]) assert.match(lid.parent.attrs['clip-path'] || '', /^url\(#wf\d+e\d\)$/);
  }
});

test('the three silhouettes come from the style and are tinted with the wind colour only', () => {
  const { winds } = load();
  const outline = svg => JSON.stringify(svg.all().find(n => n.tag === 'clipPath' && /c$/.test(n.attrs.id)).children.map(n => [n.tag, n.attrs]));
  const outlines = new Set([...'ons'].map(s => outline(winds.face({ id:'s' + s, color:'#A8CDE2', style:'pb' + s }, true))));
  assert.equal(outlines.size, 3, 'pebble, cloud and gust are three outlines');
  assert.equal(outline(winds.face({ id:'a', color:'#EFA68F', style:'eto' })), outline(winds.face({ id:'b', color:'#A8CDE2', style:'pso' })), 'the same style is the same silhouette');
  const tints = color => new Set(winds.face({ id:'t', color, style:'pso' }, true).all().filter(n => n.tag === 'stop').map(n => n.attrs['stop-color']).filter(c => !['#FFFFFF', '#E4577A'].includes(c)));
  const blue = tints('#A8CDE2'), rose = tints('#EFA68F');
  assert.ok(blue.size >= 4);
  assert.deepEqual([...blue].filter(c => rose.has(c)), [], 'every tone of the volume derives from the wind colour');
});

test('a mood change springs from the last pose, even when the face is redrawn', () => {
  const css = winds101(), motion = motionBlocks(css).join(' ');
  const rule = /\.winds-face :is\(\.wf-pose, \.wf-aim, \.wf-lid, \.wf-smile\) \{ transition:transform ([\d.]+)s linear\(([^)]*)\)/.exec(motion);
  assert.ok(rule, 'pose, gaze and lids transition with a spring, only with motion allowed');
  const curve = rule[2].split(',').map(Number);
  assert.equal(curve[0], 0); assert.equal(curve.at(-1), 1);
  assert.ok(Math.max(...curve) > 1.05 && Math.max(...curve) < 1.2, 'it overshoots a little, like a spring');
  assert.ok(curve.slice(-4).every(v => Math.abs(v - 1) < .02), 'and settles');
  assert.equal(outsideMotion(css).includes('linear('), false);

  const { winds, frames } = load();
  const w = { id:'xaloc', name:'Xaloc', color:'#EFA68F', style:'eto' };
  const idle = poseOf(winds.face(w, true, 'idle')), working = poseOf(winds.face({ ...w, id:'other' }, true, 'working'));
  assert.notDeepEqual(idle, working);
  frames.length = 0;
  const redrawn = winds.face(w, true, 'working');
  assert.deepEqual(poseOf(redrawn), idle, 'a redrawn face starts from the pose it had');
  while (frames.length) frames.shift()();
  assert.deepEqual(poseOf(redrawn), working, 'then springs to the new one');
  assert.deepEqual(poseOf(winds.face(w, true, 'working')), working, 'the same mood again does not replay');
  assert.equal(frames.length, 0);
  // A list row of the same wind keeps its own pose apart from the large figure.
  winds.face(w, false);
  assert.deepEqual(poseOf(winds.face(w, true, 'working')), working);
  assert.equal(frames.length, 0);
  // A newer mood before the spring starts wins over the queued one.
  const quick = winds.face(w, true, 'done');
  quick.isConnected = true;
  winds.mood(quick, 'failed');
  while (frames.length) frames.shift()();
  assert.equal(quick.getAttribute('data-mood'), 'failed');
  assert.ok(poseOf(quick)['--wf-y'] > 0 && poseOf(quick)['--wf-smile'] === 0, 'the face keeps the pose of the mood it shows');
  // The same face changing mood in place: the CSS transition does the spring.
  const svg = winds.face({ ...w, id:'inplace' }, true, 'idle');
  svg.isConnected = true;
  winds.mood(svg, 'failed');
  assert.ok(poseOf(svg)['--wf-y'] > 0);
});

test('with reduced motion a face takes its new pose at once and nothing moves', () => {
  const { winds, frames } = load({ matchMedia:q => ({ matches:/reduce/.test(q) }) });
  const w = { id:'gregal', name:'Gregal', color:'#C1B4DE', style:'mso' };
  winds.face(w, true, 'idle');
  frames.length = 0;
  assert.ok(poseOf(winds.face(w, true, 'done'))['--wf-smile'] > .4);
  assert.equal(frames.length, 0);
  const css = winds101(), reduce = cssBlock(css, css.lastIndexOf('@media (prefers-reduced-motion:reduce)'));
  assert.match(reduce, /\.winds-face, \.winds-face \* \{ animation:none !important; transition:none !important;/);
});

test('done anticipates, hops and settles with a spring', () => {
  const css = winds101();
  assert.match(css, /\.winds-face\[data-mood=done\] \.wf-figure \{[^}]*wf-hop [^}]*var\(--wf-since,0s\) 1/);
  const hop = cssBlock(css, css.indexOf('@keyframes wf-hop '));
  const steps = [...hop.matchAll(/translateY\((-?[\d.]+)px\) scale\(([\d.]+),([\d.]+)\)/g)].map(m => ({ y:+m[1], sx:+m[2], sy:+m[3] }));
  const top = steps.findIndex(s => s.y === Math.min(...steps.map(t => t.y)));
  assert.ok(top > 1 && steps.slice(0, top).some(s => s.y === 0 && s.sy < .9 && s.sx > 1.05), 'a squash before the jump');
  assert.ok(steps[top].y <= -6 && steps[top].sy > 1, 'a stretched hop');
  assert.ok(steps.slice(top + 1).some(s => s.y === 0 && s.sy < .95), 'it lands with a squash');
  assert.ok(steps.slice(top + 2).filter(s => s.sy !== 1).length >= 2, 'and wobbles to rest');
  for (const name of ['wf-hop', 'wf-ponder', 'wf-muse', 'wf-peer', 'wf-lean', 'wf-sigh', 'wf-busy']) {
    const props = keyframeProps(css, name);
    assert.ok(props.length, name + ' exists');
    assert.deepEqual(props.filter(p => !['transform', 'opacity'].includes(p)), [], name);
  }
});

// 117: the minairó pack. A small round body in the wind's colour under a long
// white beard and a red barretina folded to one side, with the 110 eyes.
const classed = (svg, cls) => svg.all().filter(n => (n.attrs.class || '').split(' ').includes(cls));
const minairoCss = css => [...css.matchAll(/([^{}]+)\{([^{}]*)\}/g)].filter(m => m[1].includes('[data-avatar=minairo]') || /\.wf-(tip|beard)\b/.test(m[1]));

test('the minairó has a round body in the wind colour, a white beard, a red barretina with a dark band, the 110 eyes and tiny feet', () => {
  const { winds } = load();
  winds.setAvatarTheme('minairo');
  assert.equal(winds.avatarTheme(), 'minairo');
  const w = { id:'tramuntana', name:'Tramuntana', color:'#A8CDE2', style:'pso' };
  for (const [large, size] of [[true, undefined], [false, 40], [false, 24]]) {
    const svg = winds.face(w, large, 'idle', size);
    assert.equal(svg.getAttribute('data-avatar'), 'minairo');
    for (const part of ['wf-pose', 'wf-figure', 'wf-body', 'wf-beard', 'wf-barretina', 'wf-band', 'wf-tip', 'wf-foot-l', 'wf-foot-r', 'wf-eyes', 'wf-mouth', 'wf-worry'])
      assert.equal(classed(svg, part).length, 1, part + ' at ' + (size || 'large'));
    assert.equal(classed(svg, 'wf-lid').length, 2, 'two upper lids');
    assert.equal(classed(svg, 'wf-smile').length, 2, 'two lower lids');
    for (const lid of [...classed(svg, 'wf-lid'), ...classed(svg, 'wf-smile')]) assert.match(lid.parent.attrs['clip-path'] || '', /^url\(#wf\d+e\d\)$/);
  }
  const tones = color => new Set(winds.face({ ...w, color }, true).all().filter(n => n.tag === 'stop').map(n => n.attrs['stop-color']));
  const blue = tones('#A8CDE2'), rose = tones('#EFA68F');
  const shared = [...blue].filter(c => rose.has(c));
  assert.ok(shared.length >= 3, 'the beard and the barretina keep their own colours');
  assert.ok(blue.size - shared.length >= 4, 'the body is modelled in tones of the wind colour');
  const body = classed(winds.face(w, true), 'wf-body')[0];
  assert.match(body.attrs.fill || '', /^url\(#wf\d+/, 'the body is painted with the wind gradient');
  assert.ok(groups(winds.face(w, true)).includes('wf-ground'), 'the whole figure stands on its ground');
});

test('each minairó differs: beard length, the side its barretina falls to and a small accessory, which small faces leave out', () => {
  const { winds } = load();
  winds.setAvatarTheme('minairo');
  const beard = svg => JSON.stringify(classed(svg, 'wf-beard')[0].all().map(n => n.attrs.d));
  const fold = svg => classed(svg, 'wf-barretina')[0].all().find(n => n.children.some(c => (c.attrs.class || '').includes('wf-tip')))?.attrs.transform || '';
  const accessory = svg => ['wf-canut', 'wf-herba', 'wf-pedra'].filter(c => classed(svg, c).length);
  const beards = new Set([...'sont'].map(m => beard(winds.face({ id:'b' + m, color:'#A8CDE2', style:'p' + m + 'o' }, true))));
  assert.equal(beards.size, 4, 'the mouth style sets the beard length');
  const crew = ['tramuntana', 'gregal', 'llevant', 'xaloc', 'migjorn', 'garbi', 'ponent', 'mestral', 'brisa', 'marinada', 'terral', 'embat'];
  const drawn = crew.map(name => winds.face({ id:name, name, color:'#EFA68F', style:'pso' }, true));
  assert.equal(new Set(drawn.map(fold)).size, 2, 'the barretina falls to one side or the other');
  assert.equal(fold(winds.face({ id:'llevant', name:'Llevant', color:'#EFA68F', style:'pso' }, true)), fold(drawn[2]), 'the same wind always the same way');
  for (const svg of drawn) assert.equal(accessory(svg).length, 1, 'one small accessory each');
  assert.deepEqual(new Set(drawn.flatMap(accessory)), new Set(['wf-canut', 'wf-herba', 'wf-pedra']), 'needle case, St John wort and pebble all appear');
  for (const name of crew) assert.deepEqual(accessory(winds.face({ id:name, name, color:'#EFA68F', style:'pso' }, false, 'idle', 24)), [], 'a small face keeps only the barretina and beard silhouette');
});

test('every mood works on the minairó, with its own pose and motion, the barretina and beard moving with it', () => {
  const { winds } = load(), css = winds101();
  winds.setAvatarTheme('minairo');
  const w = { id:'pessic', name:'Pessic', color:'#C1B4DE', style:'bto' };
  const seen = new Map();
  for (const mood of allMoods) {
    const svg = winds.face({ ...w, id:'m-' + mood }, true, mood);
    assert.equal(svg.getAttribute('data-mood'), mood);
    const look = JSON.stringify([poseOf(svg), moodLoops(css, mood), svg.all().filter(n => /wf-badge/.test(n.attrs.class || '')).length]);
    for (const [other, was] of seen) assert.notEqual(look, was, mood + ' looks like ' + other);
    seen.set(mood, look);
  }
  const rules = minairoCss(css), rule = (sel, re) => rules.find(m => sel(m[1]) && re.test(m[2]));
  assert.ok(rule(s => /\.wf-tip/.test(s) && !/data-mood/.test(s), /animation:wf-\w+/), 'the barretina tip bobs');
  assert.ok(rule(s => /\.wf-beard/.test(s) && !/data-mood/.test(s), /animation:wf-\w+/), 'the beard sways');
  assert.ok(rule(s => /\[data-mood=done\]/.test(s) && /\.wf-tip/.test(s), /var\(--wf-since,0s\) 1/), 'done flies the barretina once per hop');
  const restless = rule(s => /\[data-avatar=minairo\]\[data-mood=working\] \.wf-figure/.test(s), /animation:/);
  assert.ok(restless && !/wf-busy/.test(restless[2]), 'working has its own restless energy');
  for (const m of rules) assert.doesNotMatch(m[2], /!important/, 'reduced motion stills it all: ' + m[1].trim());
  for (const name of new Set(rules.flatMap(m => [...m[2].matchAll(/(wf-[\w-]+) [\d.]+s/g)].map(a => a[1])))) {
    const props = keyframeProps(css, name);
    assert.ok(props.length, name + ' exists');
    assert.deepEqual(props.filter(p => !['transform', 'opacity'].includes(p)), [], name + ' animates transform only');
  }
});

test('a minairó takes its new pose at once under reduced motion, and springs from its last pose otherwise', () => {
  const still = load({ matchMedia:q => ({ matches:/reduce/.test(q) }) });
  still.winds.setAvatarTheme('minairo');
  const w = { id:'didal', name:'Didal', color:'#BFCFA4', style:'mno' };
  still.winds.face(w, true, 'idle'); still.frames.length = 0;
  assert.ok(poseOf(still.winds.face(w, true, 'done'))['--wf-smile'] > .4);
  assert.equal(still.frames.length, 0, 'nothing waits to move');
  const { winds, frames } = load();
  winds.setAvatarTheme('minairo');
  const idle = poseOf(winds.face(w, true, 'idle'));
  frames.length = 0;
  const redrawn = winds.face(w, true, 'working');
  assert.deepEqual(poseOf(redrawn), idle, 'a redrawn minairó starts from the pose it had');
  while (frames.length) frames.shift()();
  assert.ok(poseOf(redrawn)['--wf-brow'] > 0, 'then springs to the working pose');
});

// 120: each minairó wears a barretina from a curated folk palette, picked from
// its id and never close in hue to its body; the black band stays.
const barretines = ['#C9342C', '#6E2F6B', '#2E6B3F', '#C8962E', '#26406E', '#7D1F33'];
const capOf = svg => svg.all().find(n => n.tag === 'radialGradient' && /^wf\d+h$/.test(n.attrs.id)).children.find(n => n.attrs.offset === '0.5').attrs['stop-color'];
const hueOf = hex => {
  const [r, g, b] = [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16) / 255), max = Math.max(r, g, b), d = max - Math.min(r, g, b);
  if (!d) return 0;
  const h = max === r ? (g - b) / d % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4;
  return (h * 60 + 360) % 360;
};
const hueGap = (a, b) => { const d = Math.abs(hueOf(a) - hueOf(b)); return Math.min(d, 360 - d); };

test('each minairó wears a barretina from the palette, the same for the same wind, spread across winds and far from its body colour', () => {
  const { winds } = load();
  winds.setAvatarTheme('minairo');
  const bodies = ['#EFA68F', '#A8CDE2', '#BFCFA4', '#C1B4DE', '#EACF87', '#9ED3C5', '#E8AEC2', '#F5B45D', '#BCC4D4', '#C9342C', '#26406E'];
  const seen = new Map();
  for (let i = 0; i < 120; i++) {
    const id = 'vent-' + i, color = bodies[i % bodies.length];
    const cap = capOf(winds.face({ id, name:'Vent ' + i, color, style:'pso' }, true));
    assert.ok(barretines.includes(cap), cap + ' is from the palette');
    assert.ok(hueGap(cap, color) >= 30, `${id}: barretina ${cap} stands apart from body ${color}`);
    assert.equal(capOf(winds.face({ id, name:'Vent ' + i, color, style:'pso' }, false, 'idle', 24)), cap, 'the same wind, the same barretina, at every size');
    assert.equal(winds.face({ id, name:'Vent ' + i, color, style:'pso' }, true).getAttribute('data-barretina'), cap, 'the clay player (characters.js) tints its barretina with it');
    seen.set(cap, (seen.get(cap) || 0) + 1);
  }
  assert.equal(seen.size, barretines.length, 'every colour of the palette is worn');
  for (const [cap, count] of seen) assert.ok(count >= 8, cap + ' is not a rarity: ' + count);
  assert.equal(classed(winds.face({ id:'vent-1', color:'#A8CDE2', style:'pso' }, true), 'wf-band')[0].attrs.fill, '#17141A', 'the black band stays');
});

test('a wind can pick its own barretina in the fourth letter of its style, any palette colour', () => {
  const { winds } = load();
  winds.setAvatarTheme('minairo');
  const picks = { v:'#C9342C', m:'#6E2F6B', f:'#2E6B3F', o:'#C8962E', b:'#26406E', g:'#7D1F33' };
  for (const [letter, hex] of Object.entries(picks)) assert.equal(capOf(winds.face({ id:'llevant', color:'#EFA68F', style:'pso' + letter }, true)), hex, letter);
  const auto = capOf(winds.face({ id:'llevant', color:'#A8CDE2', style:'pso' }, true));
  assert.equal(capOf(winds.face({ id:'llevant', color:'#A8CDE2', style:'psox' }, true)), auto, 'an unknown letter keeps the automatic pick');
});

test('each minairó has its own brows, moustache and now and then round glasses, readable at list size and gone below 28 px', () => {
  const { winds } = load();
  winds.setAvatarTheme('minairo');
  const kinds = (svg, prefix) => classed(svg, prefix).map(n => n.attrs.class.split(' ').find(c => c.startsWith(prefix + '-')));
  const brows = new Set(), moustaches = new Set();
  let glasses = 0;
  for (let i = 0; i < 200; i++) {
    const w = { id:'m-' + i, color:'#A8CDE2', style:'pso' }, svg = winds.face(w, false, 'idle', 48);
    assert.equal(kinds(svg, 'wf-brows').length, 1, 'one pair of brows');
    assert.equal(kinds(svg, 'wf-moustache').length, 1, 'one moustache');
    brows.add(kinds(svg, 'wf-brows')[0]); moustaches.add(kinds(svg, 'wf-moustache')[0]);
    glasses += classed(svg, 'wf-glasses').length;
    assert.deepEqual(kinds(winds.face(w, true), 'wf-brows'), kinds(svg, 'wf-brows'), 'the same wind, the same brows when large');
    const small = winds.face(w, false, 'idle', 24);
    assert.equal(classed(small, 'wf-brows').length + classed(small, 'wf-glasses').length, 0, 'a small face drops brows and glasses');
    assert.deepEqual(kinds(small, 'wf-moustache'), ['wf-moustache-lobes'], 'and keeps the plain moustache');
  }
  assert.deepEqual([...brows].sort(), ['wf-brows-arched', 'wf-brows-bushy', 'wf-brows-straight']);
  assert.deepEqual([...moustaches].sort(), ['wf-moustache-handlebar', 'wf-moustache-lobes', 'wf-moustache-walrus']);
  assert.ok(glasses > 5 && glasses < 50, 'glasses are rare but there: ' + glasses + ' of 200');
  const look = id => { const svg = winds.face({ id, color:'#A8CDE2', style:'pso' }, false, 'idle', 48); return [...kinds(svg, 'wf-brows'), ...kinds(svg, 'wf-moustache')].join(); };
  const neighbours = Array.from({ length:9 }, (_, i) => look('minairo-' + (i + 1)));
  assert.ok(neighbours.every((l, i) => !i || l !== neighbours[i - 1]), 'ids one letter apart look different: ' + neighbours.join(' | '));
});
