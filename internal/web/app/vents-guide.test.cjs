// «Què pot fer Gregal» (vents-guide.js): un full lleuger amb tres targetes i «Més».
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

class Element {
  constructor(tag) { this.tagName = tag.toUpperCase(); this.children = []; this.attributes = {}; this.listeners = {}; this.className = ''; this._text = ''; this.parentNode = null; this.style = { setProperty(k, v) { this[k] = v; } }; }
  setAttribute(k, v) { this.attributes[k] = String(v); }
  getAttribute(k) { return this.attributes[k] ?? null; }
  append(...nodes) { for (const n of nodes) { n.remove(); n.parentNode = this; this.children.push(n); } }
  remove() { const p = this.parentNode; if (p) p.children = p.children.filter(c => c !== this); this.parentNode = null; }
  get textContent() { return this._text + this.children.map(n => n.textContent).join(''); }
  set textContent(v) { this.children = []; this._text = String(v); }
  matches(s) { return s.startsWith('.') ? s.slice(1).split('.').every(c => this.className.split(' ').includes(c)) : this.tagName.toLowerCase() === s; }
  querySelectorAll(s) { return this.children.flatMap(n => [...(n.matches(s) ? [n] : []), ...n.querySelectorAll(s)]); }
  querySelector(s) { return this.querySelectorAll(s)[0] || null; }
  addEventListener(k, f) { (this.listeners[k] ||= []).push(f); }
  click() { this.onclick?.(); }
  focus() { this.doc.focused = this; }
  showModal() { this.open = true; }
  close() { this.open = false; for (const f of this.listeners.close || []) f(); }
}

function load(lang = 'en') {
  const doc = { documentElement: { lang }, focused: null };
  doc.createElement = tag => Object.assign(new Element(tag), { doc });
  doc.body = doc.createElement('body');
  const window = {};
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'vents-guide.js'), 'utf8'), { window, document: doc, localStorage: { getItem: () => null, setItem() {} } });
  return { guide: window.gregalGuide, doc };
}
const byText = (root, sel, text) => root.querySelectorAll(sel).find(n => n.textContent.trim() === text);

test('the guide opens with three cards and «More», which adds the rest in place', () => {
  const { guide, doc } = load();
  guide.open();
  const d = doc.body.querySelector('.vw-guide');
  assert.ok(d.open);
  const cards = () => d.querySelectorAll('.vw-guide-card');
  assert.equal(cards().length, 3, 'three cards by default');
  assert.deepEqual(cards().map(c => c.style['--i']), ['0', '1', '2'], 'staggered one after the other');
  const more = byText(d, 'button', 'More');
  assert.ok(more, 'a «More» button');
  more.click();
  assert.equal(cards().length, guide.topics().length, 'More shows every topic');
  assert.equal(byText(d, 'button', 'More'), undefined, 'and goes away');
  assert.equal(cards()[3].style['--i'], '0', 'the new cards stagger from the start again');
  assert.equal(doc.focused, cards()[3].querySelector('button'), 'focus moves to the first new card instead of getting lost');
});

test('the guide speaks Catalan too', () => {
  const { guide, doc } = load('ca');
  guide.open();
  const d = doc.body.querySelector('.vw-guide');
  assert.equal(d.querySelectorAll('.vw-guide-card').length, 3);
  assert.ok(byText(d, 'button', 'Més'));
});

test('the cards come in about 40 ms apart and settle in about 250 ms, only with motion allowed', () => {
  const css = fs.readFileSync(path.join(__dirname, 'winds-workspace.css'), 'utf8');
  const block = at => { let depth = 0; const start = css.indexOf('{', at); for (let i = start; i < css.length; i++) { if (css[i] === '{') depth++; else if (css[i] === '}' && --depth === 0) return css.slice(start + 1, i); } return ''; };
  const blocks = [...css.matchAll(/@media ?\(prefers-reduced-motion: ?no-preference\)/g)].map(m => block(m.index)), motion = blocks.join('\n');
  const still = blocks.reduce((rest, b) => rest.replace(b, ''), css);
  const m = motion.match(/\.vw-guide-card \{ animation:vw-guide-in (\.\d+)s [^;]*; animation-delay:calc\(var\(--i\) \* (\d+)ms\); \}/);
  assert.ok(m, 'the card animation lives under no-preference');
  const [dur, step] = [Number(m[1]) * 1000, Number(m[2])];
  assert.equal(step, 40);
  const total = dur + 2 * step;
  assert.ok(total >= 220 && total <= 280, 'three cards in about 250 ms, got ' + total);
  assert.doesNotMatch(still, /animation:vw-guide-in/, 'no card animation with reduced motion');
});
