const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

// Just enough DOM for the avatar pickers: elements, classes, events and focus.
class El {
  constructor(tag, doc) { this.tag = tag; this.doc = doc; this.attrs = {}; this.children = []; this.className = ''; this.listeners = {}; this.hidden = false; this._text = ''; this.style = { setProperty() {} }; }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  removeAttribute(k) { delete this.attrs[k]; }
  getAttribute(k) { return this.attrs[k] ?? null; }
  append(...nodes) { for (const n of nodes) { if (typeof n === 'string') { this._text += n; continue; } n.parent = this; this.children.push(n); } }
  replaceChildren(...nodes) { this.children = []; this.append(...nodes); }
  remove() { this.parent.children = this.parent.children.filter(n => n !== this); }
  get textContent() { return this._text + this.children.map(n => n.textContent).join(''); }
  set textContent(v) { this.children = []; this._text = String(v); }
  get classList() { const names = () => this.className.split(' ').filter(Boolean); return { toggle: (c, on) => { this.className = (on ? [...new Set([...names(), c])] : names().filter(x => x !== c)).join(' '); }, contains: c => names().includes(c) }; }
  all() { return this.children.flatMap(n => [n, ...n.all()]); }
  addEventListener(k, f) { (this.listeners[k] ||= []).push(f); }
  dispatchEvent(e) { e.target ||= this; for (const f of this.listeners[e.type] || []) f(e); if (e.bubbles && this.parent) this.parent.dispatchEvent(e); }
  focus() { this.doc.activeElement = this; }
  async click() { if (this.onclick) await this.onclick({ target:this }); for (const f of this.listeners.click || []) await f({ target:this }); }
  querySelector() { return null; }
}
class Event { constructor(type, opts = {}) { this.type = type; this.bubbles = !!opts.bubbles; } }
class KeyboardEvent extends Event { constructor(type, opts = {}) { super(type, { bubbles:true }); this.key = opts.key; } preventDefault() { this.prevented = true; } }

function load(theme) {
  const document = { activeElement:null, body:null, hidden:false, addEventListener() {} };
  document.createElement = tag => Object.assign(new El(tag, document), tag === 'canvas' ? { getContext:() => ({}) } : {});
  document.createElementNS = (_, tag) => new El(tag, document);
  const posts = [];
  const api = async (url, opts) => { if (opts.method === 'POST') posts.push(JSON.parse(opts.body)); return { ok:true, json:async () => ({ avatarTheme:theme }) }; };
  const window = { document, api, Event, performance:{ now:() => 0 }, IntersectionObserver:class { observe() {} } };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'winds.js'), 'utf8'), { window });
  return { winds:window.gregalWinds, document, posts };
}
const faces = n => n.all().filter(c => c.tag === 'svg');
// Cares is the plain face, the only pack without a data-avatar mark.
const packOf = svg => svg.getAttribute('data-avatar') || 'cares';
const packs = ['cares', 'esperit', 'cap', 'rosa', 'mapa', 'minairo', 'pelut'];

test('Preferences › Avatars shows a sample face of each pack on its choice and keeps the chosen one marked', async () => {
  const { winds, document, posts } = load('esperit');
  const box = new El('div', document);
  await winds.avatarPrefs(box);
  const choices = box.all().filter(n => n.getAttribute('data-v'));
  assert.deepEqual(choices.map(c => c.getAttribute('data-v')), [...packs, 'free']);
  for (const choice of choices.slice(0, packs.length)) {
    const drawn = faces(choice);
    assert.equal(drawn.length, 1, choice.getAttribute('data-v') + ' holds one sample face');
    assert.equal(packOf(drawn[0]), choice.getAttribute('data-v'));
  }
  assert.ok(new Set(faces(choices[packs.length]).map(packOf)).size > 1, 'Lliure shows winds drawn with different packs');
  assert.equal(winds.avatarTheme(), 'esperit', 'drawing the samples leaves the account theme alone');
  const marked = () => choices.filter(c => c.getAttribute('aria-pressed') === 'true' && c.classList.contains('on')).map(c => c.getAttribute('data-v'));
  assert.deepEqual(marked(), ['esperit']);
  await choices[3].click();
  assert.deepEqual(posts, [{ theme:'rosa' }]);
  assert.deepEqual(marked(), ['rosa']);
  assert.equal(packOf(faces(choices[0])[0]), 'cares');
});

test('the Lliure avatar field offers each pack as a sample in the wind colour, chosen with arrow keys or a click', () => {
  const { winds, document } = load('free');
  winds.setAvatarTheme('free');
  const parent = new El('div', document);
  const field = winds.avatarField(parent, { id:'brisa', name:'Brisa', color:'#123456', style:'pso', avatar:'rosa' });
  assert.equal(field.getAttribute('role'), 'radiogroup');
  assert.equal(field.value, 'rosa');
  const radios = field.all().filter(n => n.getAttribute('role') === 'radio');
  assert.deepEqual(radios.map(r => packOf(faces(r)[0])), packs);
  assert.ok(radios.every(r => r.textContent.trim()), 'each sample keeps its name');
  assert.ok(faces(radios[0])[0].all().some(n => n.getAttribute('stop-color') === '#123456'), 'drawn in the wind colour');
  const checked = () => radios.filter(r => r.getAttribute('aria-checked') === 'true').map(r => packOf(faces(r)[0]));
  assert.deepEqual(checked(), ['rosa']);
  assert.deepEqual(radios.map(r => r.getAttribute('tabindex')), ['-1', '-1', '-1', '0', '-1', '-1', '-1'], 'one tab stop, on the chosen pack');
  let changes = 0; field.addEventListener('change', () => changes++);
  radios[3].dispatchEvent(new KeyboardEvent('keydown', { key:'ArrowRight' }));
  assert.equal(field.value, 'mapa'); assert.deepEqual(checked(), ['mapa']); assert.equal(document.activeElement, radios[4]);
  radios[4].dispatchEvent(new KeyboardEvent('keydown', { key:'ArrowDown' }));
  assert.equal(field.value, 'minairo');
  radios[5].dispatchEvent(new KeyboardEvent('keydown', { key:'ArrowDown' }));
  assert.equal(field.value, 'pelut');
  radios[6].dispatchEvent(new KeyboardEvent('keydown', { key:'ArrowDown' }));
  assert.equal(field.value, 'cares', 'the arrows wrap around');
  radios[0].dispatchEvent(new KeyboardEvent('keydown', { key:'ArrowLeft' }));
  assert.equal(field.value, 'pelut');
  radios[1].click();
  assert.equal(field.value, 'esperit'); assert.deepEqual(checked(), ['esperit']);
  radios[1].click();
  assert.equal(changes, 6, 'every new pick, and only a new one, tells the editor to redraw its preview');
});

test('with a theme the editor names the pack instead of offering a choice', () => {
  const { winds, document } = load('cap');
  winds.setAvatarTheme('cap');
  const parent = new El('div', document);
  assert.equal(winds.avatarField(parent, { id:'brisa', color:'#123456' }), null);
  assert.match(parent.textContent, /Cap de vent/);
});

test('Minairons is a pack in Preferences › Avatars, with a minairó as its sample, and choosing it draws every wind as one', async () => {
  const { winds, document, posts } = load('cares');
  const box = new El('div', document);
  await winds.avatarPrefs(box);
  const choice = box.all().find(n => n.getAttribute('data-v') === 'minairo');
  assert.ok(choice, 'offered next to the other packs');
  assert.match(choice.textContent, /Minairons/);
  assert.deepEqual(faces(choice).map(packOf), ['minairo'], 'its sample is a minairó');
  await choice.click();
  assert.deepEqual(posts, [{ theme:'minairo' }]);
  assert.equal(winds.avatarTheme(), 'minairo');
  assert.equal(packOf(winds.face({ id:'brisa', name:'Brisa', color:'#123456', style:'pso' })), 'minairo');
});

test('the account theme the server sends for minairons draws every wind as a minairó, and the editor names the pack', () => {
  const { winds, document } = load('minairo');
  winds.setAvatarTheme('minairo');
  assert.equal(packOf(winds.face({ id:'brisa', name:'Brisa', color:'#123456', style:'pso', avatar:'rosa' })), 'minairo');
  const parent = new El('div', document);
  assert.equal(winds.avatarField(parent, { id:'brisa', color:'#123456' }), null);
  assert.match(parent.textContent, /Minairons/);
});

test('the barretina field offers Automàtica and the six colours, starts on the wind’s fourth style letter, and avatarPack says when a wind is a minairó', () => {
  const { winds, document } = load('free');
  winds.setAvatarTheme('free');
  const parent = new El('div', document);
  const hat = winds.barretinaField(parent, { id:'brisa', color:'#123456', style:'psom' });
  assert.equal(hat.getAttribute('id'), 'windBarretina');
  assert.deepEqual(hat.children.map(o => o.getAttribute('value')), ['', 'v', 'm', 'f', 'o', 'b', 'g']);
  assert.match(parent.textContent, /Barretina.*Automàtica.*Morada/);
  assert.equal(hat.value, 'm');
  assert.equal(winds.barretinaField(new El('div', document), { style:'pso' }).value, '', 'no letter is automatic');
  assert.equal(winds.avatarPack({ avatar:'minairo' }), 'minairo');
  assert.equal(winds.avatarPack({ avatar:'rosa' }), 'rosa');
  winds.setAvatarTheme('minairo');
  assert.equal(winds.avatarPack({ avatar:'rosa' }), 'minairo', 'the theme draws every wind');
});
