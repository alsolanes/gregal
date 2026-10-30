// La base dels visors de l'Arena (arena-view-base.js): el bucle s'atura quan
// no es veu, en pausa només pinta si alguna cosa canvia, la consulta s'atura
// amagada i torna, i la densitat de píxels té sostre.
const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const load = () => import(pathToFileURL(path.join(__dirname, 'arena-view-base.js')).href);

// Un navegador de mentida: rAF a mà, temporitzadors reals, visibilitat i observadors controlables.
function fakeBrowser({ dpr = 3, coarse = false } = {}) {
  const frames = [], listeners = {}, ios = [];
  global.window = { devicePixelRatio: dpr, addEventListener() {}, removeEventListener() {}, matchMedia: q => ({ matches: coarse && /coarse/.test(q) }) };
  global.document = { hidden: false, addEventListener: (k, f) => { listeners[k] = f; }, removeEventListener() {} };
  global.requestAnimationFrame = f => { frames.push(f); return frames.length; };
  global.cancelAnimationFrame = () => {};
  global.IntersectionObserver = class { constructor(cb) { this.cb = cb; ios.push(this); } observe() {} disconnect() {} };
  let now = 0;
  const step = (ms = 16) => { now += ms; const list = frames.splice(0); for (const f of list) f(now); return list.length; };
  return { frames, listeners, ios, step, setHidden(h) { global.document.hidden = h; listeners.visibilitychange(); } };
}
const canvas = () => ({ nodeType: 1, width: 0, height: 0, getContext: () => ({}), getBoundingClientRect: () => ({ width: 400, height: 300 }) });

test('the device pixel ratio is capped at 2 (1.5 on touch screens)', async () => {
  const B = await load();
  fakeBrowser({ dpr: 3 }); assert.equal(B.viewDpr(), 2);
  fakeBrowser({ dpr: 3, coarse: true }); assert.equal(B.viewDpr(), 1.5); assert.equal(B.lowPower(), true);
  fakeBrowser({ dpr: 1 }); assert.equal(B.viewDpr(), 1);
});

test('paused viewers draw only when something changes; hidden ones stop drawing and polling', async () => {
  const B = await load();
  const br = fakeBrowser({ dpr: 1 });
  let draws = 0, polls = 0;
  class V extends B.ArenaViewBase {
    constructor(cv) { super(cv, { fps: 60 }); this.start(); }
    resize() { this.W = 400; }
    onFrame() { draws++; }
    poll() { polls++; if (!this.stopped) this.schedulePoll(20); }
  }
  const v = new V(canvas());
  assert.equal(polls, 1, 'first poll at once');
  br.step(); br.step(); br.step();
  assert.equal(draws, 3, 'a frame per animation frame while playing');
  v.togglePause();
  br.step(); // the frame that shows the pause
  const d0 = draws;
  for (let i = 0; i < 5; i++) br.step();
  assert.equal(draws, d0, 'paused and unchanged: nothing drawn');
  assert.equal(br.frames.length, 0, 'and no frame requested');
  v.invalidate(); br.step();
  assert.equal(draws, d0 + 1, 'a change while paused: one redraw');
  v.togglePause(); br.step();
  // Off screen (another page, display: none): no frames, no polls.
  br.ios[0].cb([{ isIntersecting: false }]);
  const p0 = polls, d1 = draws;
  for (let i = 0; i < 4; i++) br.step();
  await new Promise(r => setTimeout(r, 80));
  assert.equal(draws, d1, 'hidden: no frames');
  assert.equal(polls, p0, 'hidden: no polls');
  br.ios[0].cb([{ isIntersecting: true }]);
  await new Promise(r => setTimeout(r, 60));
  assert.ok(polls > p0, 'shown again: polling resumes');
  br.step();
  assert.ok(draws > d1, 'shown again: drawing resumes');
  // Hidden tab: the same.
  br.setHidden(true);
  const p1 = polls, d2 = draws;
  br.step(); await new Promise(r => setTimeout(r, 80));
  assert.equal(draws, d2); assert.equal(polls, p1);
  br.setHidden(false); br.step();
  assert.ok(draws > d2);
  v.destroy();
  const p2 = polls; await new Promise(r => setTimeout(r, 60));
  assert.equal(polls, p2, 'destroyed: no more polls');
});

test('30 fps on touch screens', async () => {
  const B = await load();
  const br = fakeBrowser({ dpr: 3, coarse: true });
  let draws = 0;
  class V extends B.ArenaViewBase { constructor(cv) { super(cv); this.start(); } onFrame() { draws++; } }
  const v = new V(canvas());
  for (let i = 0; i < 60; i++) br.step(1000 / 60);
  assert.ok(draws >= 28 && draws <= 32, 'about 30 draws a second: ' + draws);
  v.destroy();
});

test('shared helpers keep the viewers\' behaviour', async () => {
  const B = await load();
  const list = [{ score: [0, 0] }, { score: [7, 0], text: 'TD' }, { hl: true, text: 'big' }];
  assert.deepEqual(B.progressMarks(list).map(m => m.kind), ['score', 'hl']);
  assert.deepEqual(B.progressMarks(list, false).map(m => m.kind), ['hl']);
  assert.deepEqual(B.elapsedAt([0, 5, 9], 1, 2), { times: [0, 5, 9], duration: 9, elapsed: 2 });
  assert.equal(B.esc('<a href="x">&'), '&lt;a href=&quot;x&quot;&gt;&amp;');
  const r1 = B.rng(7), r2 = B.rng(7);
  assert.equal(r1(), r2());
  assert.deepEqual(B.hexToRgb('#ff8000'), [255, 128, 0]);
  assert.deepEqual(B.hexToRgb('nope', [1, 2, 3]), [1, 2, 3]);
});
