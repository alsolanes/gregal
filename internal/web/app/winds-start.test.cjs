// La portada d'inici de Vents (winds-workspace.js): la pàgina sencera amb un
// DOM mínim i una API falsa, com winds.test.js.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const notes = require('./winds-notes.js');

class Element {
  constructor(tag, doc) { this.tagName = tag.toUpperCase(); this.doc = doc; this.children = []; this.attributes = {}; this.dataset = {}; this.style = { setProperty() {} }; this.listeners = {}; this.value = ''; this.hidden = false; this.className = ''; this._text = ''; this.parentNode = null; this.namespaceURI = 'http://www.w3.org/2000/svg'; }
  setAttribute(k, v) { this.attributes[k] = String(v); if (k === 'class') this.className = String(v); }
  getAttribute(k) { return this.attributes[k] ?? null; }
  removeAttribute(k) { delete this.attributes[k]; }
  append(...nodes) { for (const n of nodes) { const e = typeof n === 'string' ? Object.assign(new Element('text', this.doc), { _text: n }) : n; e.remove(); e.parentNode = this; this.children.push(e); } }
  prepend(...nodes) { const old = this.children; this.children = []; this.append(...nodes); this.children.push(...old); }
  remove() { if (this.parentNode) this.parentNode.children = this.parentNode.children.filter(c => c !== this); this.parentNode = null; }
  replaceChildren(...nodes) { for (const c of this.children) c.parentNode = null; this.children = []; this._text = ''; this.append(...nodes); }
  replaceWith(n) { const p = this.parentNode; if (!p) return; const i = p.children.indexOf(this); n.remove(); n.parentNode = p; p.children[i] = n; this.parentNode = null; }
  get firstChild() { return this.children[0] || null; }
  get lastElementChild() { return this.children.at(-1) || null; }
  get isConnected() { let n = this; while (n.parentNode) n = n.parentNode; return n === this.doc.root; }
  get textContent() { return this._text + this.children.map(n => n.textContent).join(''); }
  set textContent(v) { this.children = []; this._text = String(v); }
  set innerHTML(v) { this.children = []; this._html = String(v); }
  get innerHTML() { return this._html || ''; }
  get classList() { const names = () => this.className.split(' ').filter(Boolean); return { add: (...c) => { this.className = [...new Set([...names(), ...c])].join(' '); }, remove: c => { this.className = names().filter(x => x !== c).join(' '); }, toggle: (c, on = !names().includes(c)) => { on ? this.classList.add(c) : this.classList.remove(c); return on; }, contains: c => names().includes(c) }; }
  matches(s) { if (s.startsWith('#')) return this.attributes.id === s.slice(1); if (s.startsWith('.')) return s.slice(1).split('.').every(c => this.classList.contains(c)); return this.tagName.toLowerCase() === s; }
  querySelectorAll(s) { const last = s.trim().split(/\s+/).at(-1); return this.children.flatMap(n => [...(n.matches(last) ? [n] : []), ...n.querySelectorAll(last)]); }
  querySelector(s) { return this.querySelectorAll(s)[0] || null; }
  closest(s) { let n = this; while (n && !n.matches(s)) n = n.parentNode; return n; }
  addEventListener(k, f) { (this.listeners[k] ||= []).push(f); }
  async click() { if (this.onclick) await this.onclick({ target: this, stopPropagation() {} }); for (const f of this.listeners.click || []) await f({ target: this }); }
  requestSubmit() { return this.onsubmit && this.onsubmit({ preventDefault() {} }); }
  focus() {}
  scrollIntoView() {}
  getBoundingClientRect() { return { top: 0 }; }
}

const yesterday = new Date(Date.now() - 86400000).toISOString();
const winds = [
  { id: 'xaloc', name: 'Xaloc', color: '#F5B45D', role: 'Música', style: 'pso', squad: 'daily' },
  { id: 'gregal', name: 'Gregal', color: '#8193FF', role: 'Assistent', style: 'psn', squad: 'daily' },
];
const threads = [
  { wind: 'xaloc', name: 'company-xaloc-s2', title: 'Un concert per dissabte', at: yesterday, msgs: 4 },
  { wind: 'gregal', name: 'company-gregal', title: 'Organitza la setmana', at: '2026-10-01T09:00:00Z', msgs: 12 },
];

// Una resposta SSE `done` amb l'historial que el servidor hauria desat.
function stream(history) {
  const bytes = new TextEncoder().encode('event: done\ndata: ' + JSON.stringify({ history }) + '\n\n');
  let sent = false;
  return { ok: true, status: 200, body: { getReader: () => ({ read: async () => { if (sent) return { done: true }; sent = true; return { done: false, value: bytes }; }, cancel: async () => {}, releaseLock() {} }) } };
}

async function boot({ lang = 'en', api } = {}) {
  const doc = { createElement: t => new Element(t, doc), createElementNS: (_, t) => new Element(t, doc), documentElement: { lang }, querySelector: () => null, addEventListener() {} };
  doc.root = new Element('root', doc);
  for (const id of ['ventsPage', 'windContextNav', 'windsListCard', 'side']) { const n = new Element('div', doc); n.setAttribute('id', id); doc.root.append(n); }
  doc.getElementById = id => doc.root.querySelector('#' + id);
  doc.body = new Element('body', doc);
  const requests = [];
  const json = data => ({ ok: true, status: 200, json: async () => data, text: async () => JSON.stringify(data) });
  const window = {
    document: doc, gregalNotes: notes, addEventListener() {},
    gregalWinds: { face: w => { const n = new Element('svg', doc); n.dataset.wind = w?.name || ''; return n; }, moodOf: () => '', seen() {}, watchNews() {} },
    api: async (url, opts = {}) => {
      const req = { url, method: opts.method || 'GET', body: opts.body ? JSON.parse(opts.body) : undefined };
      requests.push(req);
      const own = api && await api(req);
      if (own) return own;
      if (url === '/api/winds') return json({ winds });
      if (url === '/api/winds/threads') return json({ threads });
      if (url === '/api/winds/squads') return json({ squads: [] });
      if (url === '/api/winds/rotllanes') return json({ rotllanes: [] });
      if (url.startsWith('/api/winds/chat?')) return json({ history: [{ from: 'assistant', text: 'Fil antic de Gregal' }] });
      if (url === '/api/winds/sessions' && req.method === 'POST') return json({ wind: req.body.wind, name: 'company-' + req.body.wind + '-s9', sessions: [] });
      if (url === '/api/winds/session/open') return json({ wind: req.body.wind, name: req.body.name, history: [{ from: 'user', text: 'Un concert per dissabte' }, { from: 'assistant', text: 'Hi ha jazz al Born' }] });
      if (url === '/api/winds/chat/stream') return stream([{ from: 'user', text: req.body.message }, { from: 'assistant', text: 'Fet' }]);
      return json({});
    },
  };
  function Option(text, value) { const o = new Element('option', doc); o.textContent = text; o.value = value; return o; }
  // The page polls in the background (pending replies, the crew); those timers must not keep the test alive.
  const later = (f, ms) => { const h = setTimeout(f, ms); h.unref(); return h; };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'winds-workspace.js'), 'utf8'), {
    window, document: doc, console, URLSearchParams, setTimeout: later, clearTimeout, queueMicrotask, TextDecoder, Option,
    getComputedStyle: () => ({ getPropertyValue: () => '' }), matchMedia: () => ({ matches: true }),
    sessionStorage: { getItem: () => null, setItem() {} }, localStorage: { getItem: () => null, setItem() {} },
    location: { hash: '', pathname: '/', search: '' }, history: { replaceState() {} }, navigator: { platform: 'Win32' }, requestAnimationFrame: f => f(),
  });
  await window.loadWindsPage();
  await settle();
  const page = doc.getElementById('ventsPage');
  return { window, doc, page, requests, nav: doc.getElementById('windContextNav') };
}
const settle = () => new Promise(r => setTimeout(r, 5));
const byText = (root, sel, text) => root.querySelectorAll(sel).find(n => n.textContent.includes(text));

test('opening Vents shows the start page: faces of the winds and the recent conversations, newest first', async () => {
  const f = await boot();
  assert.ok(!f.page.querySelector('.vw-messages'), 'not a thread');
  assert.ok(!f.requests.some(r => r.url.startsWith('/api/winds/chat?')), 'Gregal’s old thread is not loaded');
  assert.ok(f.page.querySelector('.vw-composer'));
  const faces = f.page.querySelector('.vw-start-faces').querySelectorAll('button').map(b => b.textContent);
  assert.deepEqual(faces, ['Gregal', 'Xaloc'], 'Gregal first, then the other winds');
  const recent = f.page.querySelector('.vw-start-recent');
  assert.match(recent.textContent, /Recent conversations/);
  const rows = recent.querySelectorAll('button');
  assert.equal(rows.length, 2);
  assert.match(rows[0].textContent, /Un concert per dissabte/);
  assert.match(rows[0].textContent, /Xaloc · yesterday/);
  assert.match(rows[1].textContent, /Organitza la setmana/);
  assert.match(rows[1].textContent, /Gregal · /);
  assert.ok(!f.page.querySelector('.vw-prompts'), 'recent conversations take the place of the examples');
  assert.ok(!f.requests.some(r => r.url === '/api/winds/avui'), '«Today» has its own page');
});

test('with no conversations yet, the start page offers example prompts instead', async () => {
  const f = await boot({ api: r => r.url === '/api/winds/threads' ? { ok: true, status: 200, json: async () => ({ threads: [] }) } : null });
  assert.ok(!f.page.querySelector('.vw-start-recent'));
  assert.match(f.page.querySelector('.vw-prompts').textContent, /Help me choose/);
});

test('the start page speaks Catalan too', async () => {
  const f = await boot({ lang: 'ca' });
  assert.match(f.page.querySelector('.vw-start-recent').textContent, /Converses recents/);
  assert.match(f.page.querySelector('.vw-start-recent').textContent, /Xaloc · ahir/);
});

test('sending from the start page creates a new Gregal session first and sends the message there', async () => {
  const f = await boot();
  f.page.querySelector('textarea').value = 'Planeja el sopar';
  await f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  const created = f.requests.findIndex(r => r.url === '/api/winds/sessions' && r.method === 'POST');
  const sent = f.requests.findIndex(r => r.url === '/api/winds/chat/stream');
  assert.ok(created >= 0 && sent > created, 'the session is created before the message is sent');
  assert.deepEqual(f.requests[created].body, { wind: 'gregal' });
  assert.deepEqual(f.requests[sent].body, { wind: 'gregal', session: 'company-gregal-s9', message: 'Planeja el sopar' });
  assert.ok(!f.requests.some(r => r.url === '/api/winds/intent'), 'Gregal takes it, no routing step');
  assert.match(f.page.querySelector('.vw-messages').textContent, /Fet/, 'the page became the new conversation');
});

test('if the new session cannot be created, the text stays in the composer and nothing is sent', async () => {
  const f = await boot({ api: r => r.url === '/api/winds/sessions' && r.method === 'POST' ? { ok: false, status: 500, text: async () => 'boom' } : null });
  f.page.querySelector('textarea').value = 'Planeja el sopar';
  await f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  assert.ok(!f.requests.some(r => r.url === '/api/winds/chat/stream'));
  assert.equal(f.page.querySelector('textarea').value, 'Planeja el sopar');
  assert.ok(!f.page.querySelector('.vw-messages'));
});

test('tapping a face opens a new conversation with that wind; a recent one opens through session/open', async () => {
  const f = await boot();
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  assert.deepEqual(f.requests.filter(r => r.url === '/api/winds/sessions' && r.method === 'POST').map(r => r.body), [{ wind: 'xaloc' }]);
  assert.ok(f.page.querySelector('.vw-messages'), 'a thread is open');
  assert.match(f.page.querySelector('.vw-top').textContent, /Xaloc/);

  await byText(f.nav, 'button', 'New conversation').click();
  await settle();
  assert.ok(!f.page.querySelector('.vw-messages'), '«New conversation» goes back to the start page');
  await byText(f.page.querySelector('.vw-start-recent'), 'button', 'Un concert per dissabte').click();
  await settle();
  assert.deepEqual(f.requests.find(r => r.url === '/api/winds/session/open').body, { wind: 'xaloc', name: 'company-xaloc-s2' });
  assert.match(f.page.querySelector('.vw-messages').textContent, /Hi ha jazz al Born/);
});

test('while a wind is answering, the start page opens the running conversation and keeps the text', async () => {
  let finish;
  const f = await boot({ api: r => r.url === '/api/winds/chat/stream' ? new Promise(res => { finish = () => res(stream([{ from: 'user', text: r.body.message }, { from: 'assistant', text: 'Fet' }])); }) : null });
  f.page.querySelector('textarea').value = 'Primera';
  const first = f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  await byText(f.nav, 'button', 'New conversation').click();
  await settle();
  f.page.querySelector('textarea').value = 'Segona';
  await f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  assert.equal(f.requests.filter(r => r.url === '/api/winds/sessions' && r.method === 'POST').length, 1, 'no second conversation');
  assert.ok(f.page.querySelector('.vw-messages'), 'the running conversation is open');
  assert.equal(f.page.querySelector('textarea').value, 'Segona');

  await byText(f.nav, 'button', 'New conversation').click();
  await settle();
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Gregal').click();
  await settle();
  assert.equal(f.requests.filter(r => r.url === '/api/winds/sessions' && r.method === 'POST').length, 1, 'the busy wind’s face opens its running conversation');
  assert.ok(f.page.querySelector('.vw-messages'));
  finish();
  await first;
});
