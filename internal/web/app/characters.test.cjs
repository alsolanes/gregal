// El reproductor dels vents de plastilina (characters.js): un DOM i un canvas
// falsos que apunten què s'hi dibuixa, i les imatges que el test diu que hi ha.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const manifest = JSON.parse(fs.readFileSync(path.join(__dirname, 'characters', 'manifest.json'), 'utf8'));
const SIZE = manifest.size;

class Node {
  constructor(tag, doc) { this.tag = tag; this.doc = doc; this.attrs = {}; this.children = []; this.parent = null; }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  getAttribute(k) { return this.attrs[k] ?? null; }
  removeAttribute(k) { delete this.attrs[k]; }
  append(...nodes) { for (const n of nodes) { n.parent = this; this.children.push(n); } }
  remove() { if (this.parent) this.parent.children = this.parent.children.filter(c => c !== this); this.parent = null; }
  replaceWith(n) { const p = this.parent; if (!p) return; p.children[p.children.indexOf(this)] = n; n.parent = p; this.parent = null; }
  get isConnected() { let n = this; while (n.parent) n = n.parent; return n === this.doc.root; }
}
// A canvas whose pixels are plain arrays: drawImage copies an image's pixels
// (or records a frame drawn from another canvas), getImageData reads them back.
class Canvas extends Node {
  constructor(doc) {
    super('canvas', doc); this.width = 300; this.height = 150; this.draws = [];
    const self = this;
    this.ctx = {
      globalAlpha: 1, globalCompositeOperation: 'source-over',
      clearRect() { self.draws = []; },
      drawImage(src, ...args) {
        if (src.tag !== 'canvas') { self.pixels = Uint8ClampedArray.from(src.pixels); self.sheet = src.sheet; return; }
        self.source = src;
        self.draws.push({ sheet: src.sheet, sx: args[0], alpha: this.globalAlpha, op: this.globalCompositeOperation });
      },
      // The trade badge: a disc and the icon's path, recorded after the frame they are drawn over.
      save() {}, restore() {}, translate() {}, scale() {}, beginPath() {}, arc() {}, fill() {},
      stroke(shape) { if (shape) self.draws.push({ icon: shape.d }); },
      getImageData() { return { data: self.pixels || new Uint8ClampedArray(self.width * self.height * 4) }; },
      putImageData(image) { self.pixels = image.data; },
    };
  }
  getContext() { return this.ctx; }
}

function load({ files = null, reduced = false, winds = {} } = {}) {
  let now = 0;
  const frames = [], fetched = [], events = {};
  const doc = {
    hidden: false,
    createElement: tag => tag === 'canvas' ? new Canvas(doc) : new Node(tag, doc),
    addEventListener(k, f) { (events[k] ||= []).push(f); },
  };
  doc.root = new Node('root', doc);
  let observer;
  class IntersectionObserver { constructor(f) { observer = this; this.f = f; this.targets = []; } observe(t) { this.targets.push(t); } unobserve(t) { this.targets = this.targets.filter(x => x !== t); } }
  // Every sheet exists unless the test lists the ones that do. A sheet's
  // pixels are grey clay; its mask covers the first pixel only, its cap mask the second.
  class Image {
    set src(url) {
      fetched.push(url);
      const name = url.replace('/app/characters/', '');
      setImmediate(() => {
        if (files && !files.includes(name)) return this.onerror(new Error('404'));
        const mood = name.split('/')[1].split('.')[0], width = manifest.moods[mood].frames * SIZE;
        this.pixels = new Uint8ClampedArray(width * SIZE * 4).fill(200);
        if (name.endsWith('.mask.webp')) { this.pixels.fill(0); this.pixels[3] = 255; }
        if (name.endsWith('.cap.webp')) { this.pixels.fill(0); this.pixels[7] = 255; }
        this.sheet = name;
        this.onload();
      });
    }
  }
  const moods = [];
  const window = {
    document: doc, Image, IntersectionObserver, Path2D: class { constructor(d) { this.d = d; } },
    performance: { now: () => now },
    requestAnimationFrame: f => frames.push(f),
    matchMedia: () => ({ matches: reduced }),
    fetch: async url => { fetched.push(url); return { ok: true, json: async () => manifest }; },
    gregalWinds: {
      hasNews: id => (winds.news || []).includes(id),
      kitIcon: kit => ({ books: 'M8 4.5v9' })[kit] || null,
      mood(svg, mood) { moods.push(mood); svg.setAttribute('data-mood', mood); return svg; },
    },
  };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'characters.js'), 'utf8'), { window });
  return {
    chars: window.gregalCharacters, doc, fetched, moods, events,
    // One animation frame `ms` later.
    frame(ms = 16) { now += ms; for (const f of frames.splice(0)) f(now); },
    see(target, visible) { observer.f([{ target, isIntersecting: visible }]); },
    watched: target => observer.targets.includes(target),
  };
}
// The fake images load on setImmediate; a few turns let the manifest, both images and the tint finish.
const settle = async () => { for (let i = 0; i < 10; i++) await new Promise(r => setImmediate(r)); };
// The face winds.js would draw, already on the page.
function svgFace(env, mood = 'idle', attrs = {}) {
  const svg = new Node('svg', env.doc);
  svg.setAttribute('class', 'winds-face winds-face-large');
  svg.setAttribute('data-mood', mood);
  svg.setAttribute('aria-hidden', 'true');
  for (const [k, v] of Object.entries(attrs)) svg.setAttribute(k, v);
  const box = new Node('span', env.doc);
  env.doc.root.append(box); box.append(svg);
  return { svg, box };
}
const wind = { id: 'xaloc', name: 'Xaloc', color: '#FF8000', style: 'pss' };
const lastDraw = canvas => canvas.draws.at(-1);

test('tint multiplies the clay by the wind colour as far as the mask covers it and keeps its alpha', () => {
  const { chars } = load();
  const pixels = Uint8ClampedArray.from([200, 200, 200, 255, 200, 200, 200, 255, 200, 200, 200, 128]);
  const mask = Uint8ClampedArray.from([255, 255, 255, 255, 255, 255, 255, 0, 255, 255, 255, 128]);
  chars.tint(pixels, mask, [255, 128, 0]);
  assert.deepEqual([...pixels.slice(0, 4)], [200, 100, 0, 255], 'full mask: the colour');
  assert.deepEqual([...pixels.slice(4, 8)], [200, 200, 200, 255], 'no mask: the clay as it is (eyes, blush)');
  assert.deepEqual([...pixels.slice(8, 12)], [200, 150, 100, 128], 'half mask: half way, alpha untouched');
});

test('a large face shows the code-drawn face first and swaps to the tinted clay sheet of its silhouette', async () => {
  const env = load();
  const { svg, box } = svgFace(env);
  assert.equal(env.chars.play(svg, wind), svg);
  await settle();
  assert.ok(env.fetched.includes('/app/characters/curl/idle.webp') && env.fetched.includes('/app/characters/curl/idle.mask.webp'), 'style …s is the curl');
  const canvas = box.children[0];
  assert.equal(canvas.tag, 'canvas');
  assert.equal(canvas.getAttribute('class'), 'winds-face winds-face-large winds-clay');
  assert.equal(canvas.getAttribute('data-mood'), 'idle');
  assert.equal(canvas.getAttribute('aria-hidden'), 'true');
  assert.equal(canvas.width, SIZE);
  assert.deepEqual(lastDraw(canvas), { sheet: 'curl/idle.webp', sx: 0, alpha: 1, op: 'source-over' });
  // The sheet the player draws from is tinted: orange on the masked pixel only.
  assert.deepEqual([...canvas.source.pixels.slice(0, 8)], [200, 100, 0, 200, 200, 200, 200, 200]);
  // A second face of the same wind and mood draws straight away: the sheet is shared.
  const again = svgFace(env);
  const second = env.chars.play(again.svg, wind);
  assert.equal(second.tag, 'canvas');
  assert.equal(env.fetched.filter(u => u.endsWith('curl/idle.webp')).length, 1);
});

test('the sheet plays at its frame rate: loops loop, done and failed hold their last frame', async () => {
  const env = load();
  const { svg, box } = svgFace(env);
  env.chars.play(svg, wind);
  await settle();
  const canvas = box.children[0], step = 1000 / manifest.fps;
  env.frame(3 * step + 1);
  assert.equal(lastDraw(canvas).sx, 3 * SIZE);
  env.frame(manifest.moods.idle.frames * step);
  assert.equal(lastDraw(canvas).sx, 3 * SIZE, 'idle loops');
  env.chars.mood(canvas, 'done');
  await settle();
  env.frame(1000);
  env.frame(60 * step);
  assert.deepEqual(lastDraw(canvas), { sheet: 'curl/done.webp', sx: (manifest.moods.done.frames - 1) * SIZE, alpha: 1, op: 'source-over' });
});

test('a mood change crossfades from the old sheet to the new one', async () => {
  const env = load();
  const { svg, box } = svgFace(env);
  env.chars.play(svg, wind);
  await settle();
  const canvas = box.children[0];
  assert.equal(env.chars.mood(canvas, 'working'), canvas);
  assert.equal(canvas.getAttribute('data-mood'), 'working');
  assert.deepEqual(env.moods, ['working'], 'the code-drawn face follows, ready as a fallback');
  await settle();
  env.frame(60);
  const [from, to] = canvas.draws;
  assert.equal(from.sheet, 'curl/idle.webp');
  assert.equal(to.sheet, 'curl/working.webp');
  assert.ok(from.alpha > 0 && from.alpha < 1 && Math.abs(from.alpha + to.alpha - 1) < 1e-9, 'one fades out as the other fades in');
  assert.equal(to.op, 'lighter', 'the two add up, so the figure never thins out half way');
  env.frame(1000);
  assert.deepEqual(canvas.draws.map(d => d.sheet), ['curl/working.webp']);
});

// The chat redraws its corner after a turn: the new face goes on from where the old one was.
test('a face drawn again in the same spot starts from the sheet it showed last and crossfades', async () => {
  const env = load();
  const first = svgFace(env);
  env.chars.play(first.svg, wind);
  await settle();
  env.chars.mood(first.box.children[0], 'working');
  await settle();
  env.frame(1000);
  first.box.remove();
  const again = svgFace(env, 'done');
  const canvas = env.chars.play(again.svg, wind);
  assert.equal(canvas.tag, 'canvas', 'no code-drawn face in between');
  assert.equal(canvas.getAttribute('data-mood'), 'done');
  assert.equal(lastDraw(canvas).sheet, 'curl/working.webp');
  again.svg.replaceWith(canvas);
  await settle();
  env.frame(60);
  assert.deepEqual(canvas.draws.map(d => d.sheet), ['curl/working.webp', 'curl/done.webp']);
  // Another spot (a start face, not the corner) has its own last sheet.
  const start = svgFace(env);
  start.svg.setAttribute('class', 'winds-face');
  assert.deepEqual(env.chars.play(start.svg, wind).draws.map(d => d.sheet), ['curl/idle.webp']);
});

test('idle with news plays the news sheet, like the code-drawn face', async () => {
  const env = load({ winds: { news: ['xaloc'] } });
  const { svg, box } = svgFace(env, 'working');
  env.chars.play(svg, wind);
  await settle();
  env.chars.mood(box.children[0], 'idle');
  await settle();
  env.frame(1000);
  assert.equal(lastDraw(box.children[0]).sheet, 'curl/news.webp');
});

test('a missing sheet keeps the code-drawn face', async () => {
  const env = load({ files: ['curl/idle.webp', 'curl/idle.mask.webp'] });
  const { svg, box } = svgFace(env, 'thinking');
  env.chars.play(svg, wind);
  await settle();
  assert.equal(box.children[0], svg, 'no thinking sheet: the svg stays');
  // A clay face whose next mood has no sheet goes back to the svg in that mood.
  const other = svgFace(env);
  env.chars.play(other.svg, wind);
  await settle();
  const canvas = other.box.children[0];
  assert.equal(canvas.tag, 'canvas');
  env.chars.mood(canvas, 'working');
  await settle();
  assert.equal(other.box.children[0], other.svg);
  assert.equal(other.svg.getAttribute('data-mood'), 'working');
});

// 159: a wind with a trade is clay too, its trade a badge over every frame.
test('a wind with a trade plays its clay sheet with the trade badge over every frame', async () => {
  const env = load();
  const kit = svgFace(env, 'idle', { 'data-kit': 'books' });
  env.chars.play(kit.svg, wind);
  await settle();
  const canvas = kit.box.children[0];
  assert.equal(canvas.tag, 'canvas');
  assert.deepEqual(canvas.draws.at(-1), { icon: 'M8 4.5v9' }, 'the badge is drawn over the frame');
  env.frame(1000);
  assert.deepEqual(canvas.draws.at(-1), { icon: 'M8 4.5v9' }, 'and over the next one');
  // The same sheet is ready now: play hands the clay back straight away.
  const plain = env.chars.play(svgFace(env).svg, { ...wind, id: 'mestral' });
  assert.equal(plain.tag, 'canvas');
  assert.ok(!plain.draws.some(d => d.icon), 'no trade, no badge');
});

test('the other avatar packs stay code-drawn', async () => {
  const env = load();
  const cap = svgFace(env, 'idle', { 'data-avatar': 'cap' });
  assert.equal(env.chars.play(cap.svg, { ...wind, avatar: 'cap' }), cap.svg);
  await settle();
  assert.equal(cap.box.children[0], cap.svg);
  assert.equal(env.fetched.length, 0);
});

test('a minairó plays the minairó clay sheets, whatever its silhouette, and keeps its drawing without them', async () => {
  const env = load();
  const { svg, box } = svgFace(env, 'idle', { 'data-avatar': 'minairo' });
  assert.equal(env.chars.play(svg, { ...wind, avatar: 'minairo' }), svg);
  await settle();
  assert.ok(env.fetched.includes('/app/characters/minairo/idle.webp') && env.fetched.includes('/app/characters/minairo/idle.mask.webp'));
  assert.ok(!env.fetched.some(u => u.includes('/curl/')), 'not the curl of its style');
  assert.equal(box.children[0].tag, 'canvas');
  assert.deepEqual(lastDraw(box.children[0]), { sheet: 'minairo/idle.webp', sx: 0, alpha: 1, op: 'source-over' });
  const bare = load({ files: [] });
  const drawn = svgFace(bare, 'idle', { 'data-avatar': 'minairo' });
  bare.chars.play(drawn.svg, { ...wind, avatar: 'minairo' });
  await settle();
  assert.equal(drawn.box.children[0], drawn.svg, 'no minairó sheets: the drawn minairó stays');
});

test('a minairó tints its barretina with its own colour through the cap mask, its body with the wind colour', async () => {
  const env = load();
  const purple = svgFace(env, 'idle', { 'data-avatar': 'minairo', 'data-barretina': '#6E2F6B' });
  env.chars.play(purple.svg, { ...wind, avatar: 'minairo' });
  await settle();
  assert.ok(env.fetched.includes('/app/characters/minairo/idle.cap.webp'));
  const canvas = purple.box.children[0];
  assert.deepEqual([...canvas.source.pixels.slice(0, 12)], [200, 100, 0, 200, 86, 37, 84, 200, 200, 200, 200, 200], 'body orange, barretina purple, the rest clay');
  // Another wind of the same colour with another barretina gets its own sheet.
  const green = svgFace(env, 'idle', { 'data-avatar': 'minairo', 'data-barretina': '#2E6B3F' });
  env.chars.play(green.svg, { ...wind, id: 'garbi', avatar: 'minairo' });
  await settle();
  assert.deepEqual([...green.box.children[0].source.pixels.slice(4, 8)], [36, 84, 49, 200]);
  assert.ok(!env.fetched.some(u => u.includes('/curl/')));
  // The other silhouettes have no cap mask.
  const curl = svgFace(env);
  env.chars.play(curl.svg, wind);
  await settle();
  assert.ok(!env.fetched.some(u => u.endsWith('.cap.webp') && !u.includes('/minairo/')));
});

test('a pelut plays the pelut clay sheets in its colour, a still one holds the still frame, and a face handed twice is played once', async () => {
  const env = load();
  const big = svgFace(env, 'searching', { 'data-avatar': 'pelut' });
  env.chars.play(big.svg, { ...wind, avatar: 'pelut' });
  await settle();
  assert.ok(env.fetched.includes('/app/characters/pelut/searching.webp') && env.fetched.includes('/app/characters/pelut/searching.mask.webp'));
  assert.ok(!env.fetched.some(u => u.includes('/curl/')), 'not the curl of its style');
  const canvas = big.box.children[0];
  assert.deepEqual([...canvas.source.pixels.slice(0, 8)], [200, 100, 0, 200, 200, 200, 200, 200], 'tinted orange where the mask is');
  env.frame(1000);
  assert.notEqual(lastDraw(canvas).sx, 0, 'a large pelut plays');
  const small = svgFace(env, 'reading', { 'data-avatar': 'pelut' });
  small.svg.setAttribute('class', 'winds-face');
  assert.ok(env.chars.play(small.svg, { ...wind, avatar: 'pelut' }, true) === small.svg);
  assert.ok(env.chars.play(small.svg, { ...wind, avatar: 'pelut' }) === small.svg, 'already handed: left alone');
  await settle();
  assert.equal(env.fetched.filter(u => u.endsWith('pelut/reading.webp')).length, 1);
  const held = small.box.children[0];
  assert.equal(held.tag, 'canvas');
  env.frame(1000);
  assert.equal(held.width, 256, 'the pelut has 256 px frames');
  assert.equal(held.source.width, 256, 'a still face tints only its still frame, not the whole sheet');
  assert.deepEqual(lastDraw(held), { sheet: 'pelut/reading.webp', sx: 0, alpha: 1, op: 'source-over' });
  assert.ok(env.watched(canvas) && !env.watched(held), 'the large one animates while on screen; the still one is drawn once');
  assert.ok(env.chars.play(held, { ...wind, avatar: 'pelut' }) === held, 'a clay face stays as it is');
});

test('the tinted sheets kept for later are bounded: many colours do not pile up, the oldest is tinted again', async () => {
  const env = load();
  const colours = Array.from({ length: 30 }, (_, i) => '#' + (0x102030 + i * 0x010101).toString(16).toUpperCase());
  for (const color of colours) {
    env.chars.play(svgFace(env, 'idle', { 'data-avatar': 'pelut' }).svg, { ...wind, id: undefined, color, avatar: 'pelut' }, true);
    await settle();
  }
  const loads = () => env.fetched.filter(u => u.endsWith('pelut/idle.webp')).length;
  assert.equal(loads(), 30);
  env.chars.play(svgFace(env, 'idle', { 'data-avatar': 'pelut' }).svg, { ...wind, id: undefined, color: colours.at(-1), avatar: 'pelut' }, true);
  await settle();
  assert.equal(loads(), 30, 'a recent colour is still kept');
  env.chars.play(svgFace(env, 'idle', { 'data-avatar': 'pelut' }).svg, { ...wind, id: undefined, color: colours[0], avatar: 'pelut' }, true);
  await settle();
  assert.equal(loads(), 31, 'the oldest was let go');
});

// 142: while a turn works, the big wind handles the prop of its work kind.
const props = { searching: 'magnifier', reading: 'book', writing: 'pencil', working: 'laptop', thinking: 'gear', done: 'confetti' };

test('every work mood has a sheet with its prop, and talk and idle have none', () => {
  for (const [mood, prop] of Object.entries(props)) assert.equal(manifest.moods[mood].prop, prop, mood);
  for (const mood of ['idle', 'discussing', 'waiting', 'news', 'failed']) assert.equal(manifest.moods[mood].prop, undefined, mood);
});

test('the clay wind plays the prop sheet of the work kind, loops it while the kind lasts and blends back to talk and idle', async () => {
  const env = load();
  const { svg, box } = svgFace(env);
  env.chars.play(svg, wind);
  await settle();
  const canvas = box.children[0], step = 1000 / manifest.fps;
  for (const mood of ['searching', 'reading', 'writing', 'working', 'thinking']) {
    env.chars.mood(canvas, mood);
    await settle();
    env.frame(1000);
    const shown = lastDraw(canvas);
    assert.equal(shown.sheet, `curl/${mood}.webp`);
    env.frame(manifest.moods[mood].frames * step + 1);
    assert.deepEqual(lastDraw(canvas), shown, `${mood} loops on its own frames`);
  }
  for (const [mood, sheet] of [['discussing', 'curl/discussing.webp'], ['idle', 'curl/idle.webp']]) {
    const before = lastDraw(canvas).sheet;
    env.chars.mood(canvas, mood);
    await settle();
    env.frame(60);
    const [from, to] = canvas.draws;
    assert.deepEqual([from.sheet, to.sheet], [before, sheet], `${mood} blends in over the prop sheet`);
    assert.ok(from.alpha > 0 && from.alpha < 1);
    env.frame(1000);
  }
});

test('reduced motion holds each sheet on its still frame, with the prop in view, and switches without a crossfade', async () => {
  const env = load({ reduced: true });
  const { svg, box } = svgFace(env);
  env.chars.play(svg, wind);
  await settle();
  const canvas = box.children[0];
  env.frame(500);
  assert.deepEqual(canvas.draws, [{ sheet: 'curl/idle.webp', sx: 0, alpha: 1, op: 'source-over' }], 'no prop: the first frame');
  for (const mood of Object.keys(props)) {
    const still = manifest.moods[mood].still;
    assert.ok(still > 0 && still < manifest.moods[mood].frames, `${mood} picks a frame of its own`);
    env.chars.mood(canvas, mood);
    await settle();
    env.frame(500);
    assert.deepEqual(canvas.draws, [{ sheet: `curl/${mood}.webp`, sx: still * SIZE, alpha: 1, op: 'source-over' }], mood);
  }
  assert.equal(manifest.moods.done.still, manifest.moods.done.frames - 1, 'done stills on its last frame: the confetti has landed');
});

test('the player pauses off screen and while the tab is hidden', async () => {
  const env = load();
  const { svg, box } = svgFace(env);
  env.chars.play(svg, wind);
  await settle();
  const canvas = box.children[0], step = 1000 / manifest.fps;
  env.see(canvas, false);
  env.frame(2 * step + 1);
  assert.equal(lastDraw(canvas).sx, 0, 'off screen: no new frame');
  env.see(canvas, true);
  env.frame(1);
  assert.equal(lastDraw(canvas).sx, 2 * SIZE);
  env.doc.hidden = true;
  env.frame(2 * step);
  assert.equal(lastDraw(canvas).sx, 2 * SIZE, 'hidden tab: no new frame');
  env.doc.hidden = false;
  for (const f of env.events.visibilitychange) f();
  env.frame(1);
  assert.equal(lastDraw(canvas).sx, 4 * SIZE);
  // A face that left the page stops being drawn.
  box.remove();
  env.frame(5 * step);
  assert.equal(lastDraw(canvas).sx, 4 * SIZE);
});

// 167: a small Cares face (list rows, the chat header) holds the still frame of its mood's sheet.
test('a small Cares face holds its tinted still frame with the trade badge, and a mood change swaps the still without a loop', async () => {
  const env = load();
  const rows = [svgFace(env, 'idle', { 'data-kit': 'books' }), svgFace(env, 'idle', { 'data-kit': 'books' })];
  for (const { svg } of rows) { svg.setAttribute('class', 'winds-face'); assert.equal(env.chars.play(svg, wind, true), svg, 'the drawn face shows until the sheet loads'); }
  await settle();
  assert.equal(env.fetched.filter(u => u.endsWith('curl/idle.webp')).length, 1, 'one still per wind and mood, however many rows');
  const held = rows[0].box.children[0];
  assert.equal(held.tag, 'canvas');
  assert.equal(held.source, rows[1].box.children[0].source, 'the rows share one tinted still');
  assert.equal(held.source.width, SIZE, 'only the still frame is tinted');
  assert.deepEqual([...held.source.pixels.slice(0, 8)], [200, 100, 0, 200, 200, 200, 200, 200], 'in the wind colour');
  assert.deepEqual(held.draws, [{ sheet: 'curl/idle.webp', sx: 0, alpha: 1, op: 'source-over' }, { icon: 'M8 4.5v9' }]);
  assert.ok(!env.watched(held), 'a still face is drawn once, outside the animation loop');
  env.chars.mood(held, 'reading');
  await settle();
  assert.deepEqual(held.draws, [{ sheet: 'curl/reading.webp', sx: 0, alpha: 1, op: 'source-over' }, { icon: 'M8 4.5v9' }], 'the new mood\'s still, at once');
  assert.ok(!env.watched(held), 'still no loop');
  env.frame(1000);
  assert.deepEqual(held.draws.map(d => d.sheet || d.icon), ['curl/reading.webp', 'M8 4.5v9']);
  const bare = load({ files: [] });
  const drawn = svgFace(bare);
  bare.chars.play(drawn.svg, wind, true);
  await settle();
  assert.equal(drawn.box.children[0], drawn.svg, 'no sheet: the drawn face stays');
});
