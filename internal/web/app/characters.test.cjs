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
  // pixels are grey clay; its mask covers the first pixel only.
  class Image {
    set src(url) {
      fetched.push(url);
      const name = url.replace('/app/characters/', '');
      setImmediate(() => {
        if (files && !files.includes(name)) return this.onerror(new Error('404'));
        const mood = name.split('/')[1].split('.')[0], width = manifest.moods[mood].frames * SIZE;
        this.pixels = new Uint8ClampedArray(width * SIZE * 4).fill(200);
        if (name.endsWith('.mask.webp')) { this.pixels.fill(0); this.pixels[3] = 255; }
        this.sheet = name;
        this.onload();
      });
    }
  }
  const moods = [];
  const window = {
    document: doc, Image, IntersectionObserver,
    performance: { now: () => now },
    requestAnimationFrame: f => frames.push(f),
    matchMedia: () => ({ matches: reduced }),
    fetch: async url => { fetched.push(url); return { ok: true, json: async () => manifest }; },
    gregalWinds: {
      hasNews: id => (winds.news || []).includes(id),
      mood(svg, mood) { moods.push(mood); svg.setAttribute('data-mood', mood); return svg; },
    },
  };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'characters.js'), 'utf8'), { window });
  return {
    chars: window.gregalCharacters, doc, fetched, moods, events,
    // One animation frame `ms` later.
    frame(ms = 16) { now += ms; for (const f of frames.splice(0)) f(now); },
    see(target, visible) { observer.f([{ target, isIntersecting: visible }]); },
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

test('kits and the other avatar packs stay code-drawn', async () => {
  const env = load();
  const kit = svgFace(env, 'idle', { 'data-kit': 'books' });
  assert.equal(env.chars.play(kit.svg, wind), kit.svg);
  const cap = svgFace(env, 'idle', { 'data-avatar': 'cap' });
  assert.equal(env.chars.play(cap.svg, { ...wind, avatar: 'cap' }), cap.svg);
  await settle();
  assert.equal(kit.box.children[0], kit.svg);
  assert.equal(cap.box.children[0], cap.svg);
  assert.equal(env.fetched.length, 0);
});

test('reduced motion shows the first frame and switches moods without a crossfade', async () => {
  const env = load({ reduced: true });
  const { svg, box } = svgFace(env);
  env.chars.play(svg, wind);
  await settle();
  const canvas = box.children[0];
  env.frame(500);
  assert.deepEqual(canvas.draws, [{ sheet: 'curl/idle.webp', sx: 0, alpha: 1, op: 'source-over' }]);
  env.chars.mood(canvas, 'done');
  await settle();
  env.frame(500);
  assert.deepEqual(canvas.draws, [{ sheet: 'curl/done.webp', sx: 0, alpha: 1, op: 'source-over' }]);
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
