// La portada d'inici de Vents (winds-workspace.js) i la seva barra lateral: la
// pàgina sencera amb un DOM mínim i una API falsa, com winds.test.js.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const notes = require('./winds-notes.js');

class Element {
  constructor(tag, doc) { this.tagName = tag.toUpperCase(); this.doc = doc; this.children = []; this.attributes = {}; this.dataset = {}; this.style = { setProperty(k, v) { this[k] = v; } }; this.listeners = {}; this.value = ''; this.hidden = false; this.className = ''; this._text = ''; this.parentNode = null; this.namespaceURI = 'http://www.w3.org/2000/svg'; }
  setAttribute(k, v) { this.attributes[k] = String(v); if (k === 'class') this.className = String(v); }
  getAttribute(k) { return this.attributes[k] ?? null; }
  removeAttribute(k) { delete this.attributes[k]; }
  append(...nodes) { for (const n of nodes) { const e = typeof n === 'string' ? Object.assign(new Element('text', this.doc), { _text: n }) : n; e.remove(); e.parentNode = this; this.children.push(e); } this.layout(); }
  prepend(...nodes) { const old = this.children; this.children = []; this.append(...nodes); this.children.push(...old); }
  remove() { const p = this.parentNode; if (p) p.children = p.children.filter(c => c !== this); this.parentNode = null; p?.layout(); }
  replaceChildren(...nodes) { for (const c of this.children) c.parentNode = null; this.children = []; this._text = ''; this.layout(); this.append(...nodes); }
  // Una caixa que es desplaça, com un navegador que torna a fer el disseny a
  // cada canvi: cada node fa 20 px, la finestra en fa 100, i si el contingut
  // s'escurça el desplaçament queda retallat i no torna sol.
  get scrollHeight() { const count = n => n.children.reduce((a, c) => a + 1 + count(c), 0); return 20 * count(this); }
  get clientHeight() { return 100; }
  get scrollTop() { return this._scrollTop || 0; }
  set scrollTop(v) { this._scrollTop = Math.max(0, Math.min(v, this.scrollHeight - this.clientHeight)); }
  layout() { for (let n = this; n; n = n.parentNode) if (n._scrollTop) n.scrollTop = n._scrollTop; }
  // Com als navegadors: canviar `open` d'un <details> avisa amb un sol
  // «toggle» més tard, tant si ho fa la persona com el codi.
  get open() { return !!this._open; }
  set open(v) {
    if (!!v === this.open) return;
    this._open = !!v;
    if (this.tagName !== 'DETAILS' || this._toggling) return;
    this._toggling = true;
    setTimeout(() => { this._toggling = false; for (const f of this.listeners.toggle || []) f({ type: 'toggle', target: this }); }, 0);
  }
  before(n) { const p = this.parentNode; n.remove(); n.parentNode = p; p.children.splice(p.children.indexOf(this), 0, n); }
  replaceWith(n) { const p = this.parentNode; if (!p) return; const i = p.children.indexOf(this); n.remove(); n.parentNode = p; p.children[i] = n; this.parentNode = null; }
  get firstChild() { return this.children[0] || null; }
  get lastElementChild() { return this.children.at(-1) || null; }
  get lastChild() { return this.children.at(-1) || null; }
  get previousSibling() { const s = this.parentNode?.children || []; return s[s.indexOf(this) - 1] || null; }
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
  showModal() {}
  close() { for (const f of this.listeners.close || []) f({ target: this }); }
  scrollIntoView() { const s = this.closest('.side-scroll'); if (s) s.scrollTop = s.scrollHeight; }
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

async function boot({ lang = 'en', api, storage = {}, css = () => '', extra = {} } = {}) {
  const events = {};
  // Com un MutationObserver: el test diu quin atribut ha canviat i en quin node.
  const observers = [];
  class MutationObserver { constructor(f) { this.f = f; } observe(target, opts) { observers.push({ f: this.f, target, opts }); } }
  const mutate = (target, attr) => { for (const o of observers) if (o.target === target && o.opts.attributeFilter?.includes(attr)) o.f([{ type: 'attributes', attributeName: attr }]); };
  const doc = { createElement: t => new Element(t, doc), createElementNS: (_, t) => new Element(t, doc), documentElement: { lang }, querySelector: () => null,
    addEventListener(k, f) { (events[k] ||= []).push(f); }, dispatchEvent(e) { for (const f of events[e.type] || []) f(e); } };
  doc.root = new Element('root', doc);
  for (const id of ['ventsPage', 'windsListCard', 'side', 'workbench']) { const n = new Element('div', doc); n.setAttribute('id', id); doc.root.append(n); }
  // Com a index.html: la barra dels vents és dins la part que es desplaça de #side.
  const scroller = new Element('div', doc), sideNav = new Element('div', doc);
  scroller.className = 'side-scroll'; sideNav.setAttribute('id', 'windContextNav');
  scroller.append(sideNav); doc.root.querySelector('#side').append(scroller);
  doc.getElementById = id => doc.root.querySelector('#' + id);
  doc.body = new Element('body', doc);
  const requests = [];
  const json = data => ({ ok: true, status: 200, json: async () => data, text: async () => JSON.stringify(data) });
  const window = {
    document: doc, gregalNotes: notes, addEventListener() {}, ...extra,
    gregalWinds: { face: (w, large, mood, size) => { const n = new Element('svg', doc); n.dataset.wind = w?.name || ''; if (size) n.dataset.size = String(size); return n; }, moodOf: () => '', seen() {}, watchNews(f) { this.onNews = f; }, news: [], hasNews(id) { return this.news.includes(id); }, setAvatarTheme(theme) { this.theme = theme; },
      // winds.js draws the real picker (winds.test.js); here only Free mode gets one.
      avatarField(parent, w) { if (this.theme !== 'free') return null; const s = new Element('select', doc); s.setAttribute('id', 'windAvatar'); s.value = w?.avatar || 'cares'; parent.append(s); return s; } },
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
    window, document: doc, console, URL, URLSearchParams, setTimeout: later, clearTimeout, queueMicrotask, TextDecoder, Option, CustomEvent, MutationObserver,
    ResizeObserver: class { observe() {} disconnect() {} },
    getComputedStyle: () => ({ getPropertyValue: css }), matchMedia: () => ({ matches: true }),
    sessionStorage: { getItem: () => null, setItem() {} }, localStorage: { getItem: k => storage[k] ?? null, setItem: (k, v) => { storage[k] = String(v); } },
    location: { hash: '', pathname: '/', search: '' }, history: { replaceState() {} }, navigator: { platform: 'Win32' }, requestAnimationFrame: f => f(),
  });
  await window.loadWindsPage();
  await settle();
  const page = doc.getElementById('ventsPage');
  return { window, doc, page, requests, mutate, nav: doc.getElementById('windContextNav'), scroller };
}
const settle = () => new Promise(r => setTimeout(r, 5));
const byText = (root, sel, text) => root.querySelectorAll(sel).find(n => n.textContent.includes(text));

test('opening Vents shows the start page: faces of the winds and the recent conversations, newest first', async () => {
  const f = await boot();
  assert.ok(!f.page.querySelector('.vw-messages'), 'not a thread');
  assert.ok(!f.requests.some(r => r.url.startsWith('/api/winds/chat?')), 'Gregal’s old thread is not loaded');
  assert.ok(f.page.querySelector('.vw-composer'));
  const faces = f.page.querySelector('.vw-start-faces').querySelectorAll('button').map(b => b.querySelector('span').textContent);
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

test('a wind that writes a live document opens it in the side panel, as one of my documents', async () => {
  const frames = 'event: doc_changed\ndata: {"id":"w1","title":"Carta","scope":"mine"}\n\n' +
    'event: done\ndata: ' + JSON.stringify({ history: [{ from: 'assistant', text: 'Fet' }] }) + '\n\n';
  const bytes = new TextEncoder().encode(frames);
  let sent = false;
  const f = await boot({ api: r => r.url === '/api/winds/chat/stream' ? { ok: true, status: 200, body: { getReader: () => ({ read: async () => { if (sent) return { done: true }; sent = true; return { done: false, value: bytes }; }, cancel: async () => {}, releaseLock() {} }) } } : null });
  const changed = [];
  f.window.gregalDocs = { changed: d => changed.push({ ...d }) };
  f.page.querySelector('textarea').value = 'Fes-me una carta';
  await f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  assert.deepEqual(changed, [{ id: 'w1', title: 'Carta', scope: 'mine' }]);
  assert.match(f.page.querySelector('.vw-messages').textContent, /Fet/, 'the reply still arrives');
});

test('the Vents page hands the account avatar theme from /api/winds to the face drawer', async () => {
  const f = await boot({ api: r => r.url === '/api/winds' ? { ok: true, status: 200, json: async () => ({ winds, avatarTheme: 'esperit' }) } : null });
  assert.equal(f.window.gregalWinds.theme, 'esperit');
});

test('the sidebar draws its 20 px faces with the small variant of the packs', async () => {
  const f = await boot();
  const svg = byText(f.nav, '.vw-nav-channel', 'Gregal').querySelector('svg');
  assert.ok(+svg.dataset.size < 28, 'the face is asked for its small size');
  assert.equal(f.page.querySelector('.vw-start-faces').querySelector('svg').dataset.size, undefined, 'the 40 px start faces keep the details');
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  assert.equal(f.page.querySelector('.vw-owner > svg').dataset.size, undefined, 'the conversation header shows the wind large, with its details');
});

// A personal wind (no squad) with its own pack, on an account in Free mode or with a theme.
function avatarBoot(theme) {
  // More winds than the sidebar lists, so it links to «All winds».
  const mine = [{ id: 'xaloc', name: 'Xaloc', color: '#F5B45D', role: 'Música', style: 'pso', avatar: 'esperit' }, winds[1],
    { id: 'ponent', name: 'Ponent', color: '#EACF87', style: 'msn' }, { id: 'mestral', name: 'Mestral', color: '#BFCFA4', style: 'esn' }];
  const ok = data => ({ ok: true, status: 200, json: async () => data });
  return boot({ api: r => r.url === '/api/winds' && r.method === 'GET' ? ok({ winds: mine, avatarTheme: theme }) : r.url === '/api/winds' ? ok({ wind: r.body })
    : r.url === '/api/winds/tidy' ? ok({ groups: [], test: [], unused: [] }) : null });
}
const lastWindPost = f => f.requests.findLast(r => r.url === '/api/winds' && r.method === 'POST').body;

test('in Free mode a wind’s profile and the create dialog let you pick its avatar pack', async () => {
  const f = await avatarBoot('free');
  await byText(f.nav, 'button', 'All winds').click(); await settle();
  await byText(byText(f.page, '.vw-member', 'Xaloc'), 'button', 'Profile').click(); await settle();
  const pick = f.doc.body.querySelector('#windAvatar');
  assert.equal(pick.value, 'esperit');
  pick.value = 'cares'; await pick.onchange();
  assert.deepEqual([lastWindPost(f).id, lastWindPost(f).avatar], ['xaloc', 'cares']);
  await byText(f.page, 'button', 'Create your own wind').click(); await settle();
  const dialog = f.doc.body.children.at(-1);
  dialog.querySelector('input').value = 'Lupa';
  dialog.querySelector('#windAvatar').value = 'esperit';
  await byText(dialog, 'button', 'Create wind').click();
  assert.deepEqual([lastWindPost(f).name, lastWindPost(f).avatar], ['Lupa', 'esperit']);
});

test('with a theme set, neither the profile nor the create dialog offers a pack, and saving keeps the wind’s own', async () => {
  const f = await avatarBoot('esperit');
  await byText(f.nav, 'button', 'All winds').click(); await settle();
  await byText(byText(f.page, '.vw-member', 'Xaloc'), 'button', 'Profile').click(); await settle();
  assert.equal(f.doc.body.querySelector('#windAvatar'), null);
  await byText(f.page, 'button', 'Create your own wind').click(); await settle();
  const dialog = f.doc.body.children.at(-1);
  assert.equal(dialog.querySelector('#windAvatar'), null);
  dialog.querySelector('input').value = 'Lupa';
  await byText(dialog, 'button', 'Create wind').click();
  assert.equal('avatar' in lastWindPost(f), false);
});

test('the start page header just says Winds, not Today', async () => {
  const f = await boot();
  assert.equal(f.page.querySelector('.vw-top').textContent.trim(), 'Winds');
});

test('the sidebar «Gregal» row opens Gregal’s current conversation', async () => {
  const f = await boot();
  await byText(f.nav, '.vw-nav-channel', 'Gregal').click();
  await settle();
  assert.ok(f.requests.some(r => r.url === '/api/winds/chat?wind=gregal'));
  assert.ok(!f.requests.some(r => r.url === '/api/winds/sessions' && r.method === 'POST'), 'no new conversation');
  assert.match(f.page.querySelector('.vw-messages').textContent, /Fil antic de Gregal/);
});

test('an old thread still loading cannot replace the new conversation from the start page', async () => {
  let release;
  const old = { ok: true, status: 200, json: async () => ({ history: [{ from: 'assistant', text: 'Fil antic de Gregal' }] }) };
  const f = await boot({ api: r => r.url === '/api/winds/chat?wind=gregal' ? new Promise(res => { release = () => res(old); }) : null });
  const opening = byText(f.nav, '.vw-nav-channel', 'Gregal').click();
  f.page.querySelector('textarea').value = 'Planeja el sopar';
  await f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  release();
  await opening;
  await settle();
  const text = f.page.querySelector('.vw-messages').textContent;
  assert.match(text, /Fet/);
  assert.doesNotMatch(text, /Fil antic de Gregal/);
});

test('a «Do» template starts one new Gregal conversation right away, even on a double tap', async () => {
  const f = await boot();
  await byText(f.nav, 'button', 'Work').click();
  await byText(f.page, '.vw-page-tab', 'Do').click();
  const card = f.page.querySelectorAll('.vw-fes-card').find(c => !c.querySelector('.vw-fes-form'));
  const main = card.querySelector('.vw-fes-main');
  main.click();
  main.click();
  await settle();
  const created = f.requests.filter(r => r.url === '/api/winds/sessions' && r.method === 'POST');
  assert.deepEqual(created.map(r => r.body), [{ wind: 'gregal' }]);
  const sent = f.requests.filter(r => r.url === '/api/winds/chat/stream');
  assert.equal(sent.length, 1);
  assert.ok(f.requests.indexOf(sent[0]) > f.requests.indexOf(created[0]));
});

test('changing wind from the sidebar keeps the sidebar where it was', async () => {
  const f = await boot();
  f.scroller.scrollTop = 160;
  assert.equal(f.scroller.scrollTop, 160, 'the sidebar is long enough to scroll');
  await byText(f.nav, 'button', 'Xaloc').click();
  await settle();
  assert.match(f.page.querySelector('.vw-top').textContent, /Xaloc/, 'the sidebar was drawn again');
  assert.equal(f.scroller.scrollTop, 160);
});

test('a sidebar drawn with «Settings & more» left open does not scroll; opening it by hand does', async () => {
  const f = await boot({ storage: { gregal_vw_more: '1' } });
  f.scroller.scrollTop = 160;
  await byText(f.nav, 'button', 'Xaloc').click();
  await settle();
  const more = f.nav.querySelector('details');
  assert.ok(more.open, 'the menu is drawn open, as it was left');
  assert.equal(f.scroller.scrollTop, 160, 'drawing it open is not opening it');

  more.open = false;
  await settle();
  more.open = true;
  await settle();
  assert.equal(f.scroller.scrollTop, f.scroller.scrollHeight - f.scroller.clientHeight, 'opened by hand, its options come into view');
});

// La pastilla de l'ofici: el rol del vent, tenyit amb el seu color.
const pillOf = (root, name) => byText(root, 'button', name)?.querySelector('.vw-tag');

test('every wind name next to its face carries a pill with its trade in the wind’s colour', async () => {
  const f = await boot({ api: r => r.url === '/api/winds/tidy' ? { ok: true, status: 200, json: async () => ({ groups: [], test: [], unused: [] }) } : null });
  const check = (pill, where) => {
    assert.ok(pill, where + ': there is a pill');
    assert.equal(pill.textContent, 'Música', where + ': the pill says the trade');
    assert.equal(pill.style['--c'], '#F5B45D', where + ': in Xaloc’s colour');
  };
  check(pillOf(f.nav, 'Xaloc'), 'sidebar');
  check(pillOf(f.page.querySelector('.vw-start-faces'), 'Xaloc'), 'start page');

  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  check(f.page.querySelector('.vw-top').querySelector('.vw-tag'), 'chat header');

  await byText(f.page.querySelector('.vw-mobile-nav'), 'button', 'Winds').click();
  await settle();
  check(f.page.querySelectorAll('.vw-member').find(m => m.textContent.includes('Xaloc'))?.querySelector('.vw-tag'), 'All winds');
});

test('the pill shows the kit’s icon when the wind has one, no icon without a kit, and no pill without a role', async () => {
  const list = [
    { id: 'gregal', name: 'Gregal', color: '#8193FF', role: 'Assistent', style: 'psn', squad: 'daily' },
    { id: 'xaloc', name: 'Xaloc', color: '#F5B45D', role: 'Música', style: 'pso', squad: 'daily', kit: 'music' },
    { id: 'oreig', name: 'Oreig', color: '#A9D3F0', role: '', style: 'pso', squad: 'daily', kit: 'weather' },
  ];
  const f = await boot({ api: r => r.url === '/api/winds' ? { ok: true, status: 200, json: async () => ({ winds: list }) } : null });
  for (const root of [f.nav, f.page.querySelector('.vw-start-faces')]) {
    assert.ok(pillOf(root, 'Xaloc')?.querySelector('svg'), 'a kit gives the pill its icon');
    assert.ok(pillOf(root, 'Gregal'), 'a wind without a kit still has its pill');
    assert.equal(pillOf(root, 'Gregal').querySelector('svg'), null, '…with no icon');
    assert.ok(byText(root, 'button', 'Oreig'), 'the wind is listed');
    assert.equal(pillOf(root, 'Oreig'), null, 'an empty role shows no pill');
  }
});

// L'ordinador al costat del xat (089): un torn que el fa servir l'obre en un panell de #workbench.
async function turnWith(...events) {
  const frames = [...events, ['done', { history: [{ from: 'assistant', text: 'Fet' }] }]].map(([k, d]) => 'event: ' + k + '\ndata: ' + JSON.stringify(d) + '\n\n').join('');
  const bytes = new TextEncoder().encode(frames);
  let sent = false;
  const f = await boot({ api: r => r.url === '/api/winds/chat/stream' ? { ok: true, status: 200, body: { getReader: () => ({ read: async () => { if (sent) return { done: true }; sent = true; return { done: false, value: bytes }; }, cancel: async () => {}, releaseLock() {} }) } } : null });
  const workpanes = [];
  f.doc.addEventListener('gregal:workpane', e => workpanes.push(e.detail));
  f.page.querySelector('textarea').value = 'Mira el temps a Girona';
  await f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  return { ...f, workpanes, pane: () => f.doc.getElementById('workbench').querySelector('.vw-computer-pane') };
}

test('a turn that uses the computer opens it live beside the chat, not in a dialog; the person can close it', async () => {
  const f = await turnWith(['activity', { phase: 'tool', tool: 'browser' }], ['activity', { phase: 'tool_done', tool: 'browser' }]);
  const pane = f.pane();
  assert.ok(pane, 'the computer pane is in the workbench');
  assert.equal(f.doc.body.querySelector('dialog'), null, 'no modal dialog');
  assert.ok(f.requests.some(r => r.url === '/api/ordinador/live'), 'it polls the live view');
  assert.match(pane.textContent, /Gregal’s browser|Gregal's browser/);
  await pane.querySelector('.vw-close').click();
  assert.equal(f.pane(), null, 'closed');
});

test('other steps of a turn do not open the computer', async () => {
  const f = await turnWith(['activity', { phase: 'tool', tool: 'read' }], ['activity', { phase: 'tool', tool: 'web_search' }]);
  assert.equal(f.pane(), null);
  assert.ok(!f.requests.some(r => r.url === '/api/ordinador/live'));
});

test('the handoff card opens the computer pane too', async () => {
  const f = await turnWith(['approve_request', { key: 'h1', type: 'handoff', what: 'Inicia sessió al banc' }]);
  assert.ok(f.pane());
});

// 102: una aprovació al xat d'un vent és una targeta sota el vent: el pas, una vista prèvia del que farà i Permet / No.
async function askIn(ask, lang = 'en', answer = { ok: true, status: 200, json: async () => ({ ok: true, id: 'k1', state: 'approved' }) }) {
  const json = data => ({ ok: true, status: 200, json: async () => data });
  const f = await boot({ lang, api: r => r.url === '/api/winds/pending' ? json({ pending: [{ key: 'k1', type: 'approve', wind: 'xaloc', ...ask }] })
    : r.url === '/api/winds/answer' ? answer : null });
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  const card = f.page.querySelector('.vw-asks').querySelector('.vw-ask');
  return { ...f, card, preview: card?.querySelector('.vw-ask-preview'), corner: f.page.querySelector('.vw-wind-corner') };
}
const buttons = card => card.querySelector('.vw-ask-actions').querySelectorAll('button').map(b => b.textContent);

test('an approval for a command shows the step, the command itself and Allow / No, while the wind waits', async () => {
  const f = await askIn({ kind: 'install', name: 'bash', what: 'npm install -g serve', detail: 'npm install -g serve --prefix ~/.local' });
  assert.equal(f.card.querySelector('.vw-ask-step').textContent, 'Getting the command ready…');
  assert.equal(f.preview.dataset.kind, 'command');
  assert.equal(f.preview.querySelector('code').textContent, 'npm install -g serve --prefix ~/.local', 'the whole command, not the short one');
  assert.deepEqual(buttons(f.card), ['Allow', 'No']);
  assert.equal(f.corner.dataset.mood, 'waiting');
});

test('an approval for a page in the browser previews the site and the full address', async () => {
  const f = await askIn({ kind: 'rule', name: 'browser', what: 'browser https://www.example.com/tickets', detail: 'https://www.example.com/tickets?day=sat', reason: 'Asking because of your rule: «Ask me before buying».' });
  assert.equal(f.card.querySelector('.vw-ask-step').textContent, 'In the browser…');
  assert.equal(f.preview.dataset.kind, 'page');
  assert.equal(f.preview.querySelector('strong').textContent, 'example.com');
  assert.equal(f.preview.querySelector('small').textContent, 'https://www.example.com/tickets?day=sat');
  assert.match(f.card.querySelector('.vw-ask-sub').textContent, /Ask me before buying/);
});

test('an approval to write a document previews its name and where it goes', async () => {
  const f = await askIn({ kind: 'rule', name: 'write', what: 'write /home/anna/gregal/projectes/viatge/informe.md', detail: '/home/anna/gregal/projectes/viatge/informe.md' });
  assert.equal(f.card.querySelector('.vw-ask-step').textContent, 'Getting the document ready…');
  assert.equal(f.preview.dataset.kind, 'doc');
  assert.equal(f.preview.querySelector('strong').textContent, 'informe.md');
  assert.equal(f.preview.querySelector('small').textContent, 'Projects › viatge');
});

test('an approval to publish a site previews the public address', async () => {
  const f = await askIn({ kind: 'publish_public', name: 'publish', what: 'publish public https://viatge.example.org/', detail: 'viatge' });
  assert.equal(f.card.querySelector('.vw-ask-step').textContent, 'Getting it ready to publish…');
  assert.equal(f.preview.dataset.kind, 'site');
  assert.equal(f.preview.querySelector('strong').textContent, 'viatge.example.org');
  assert.equal(f.preview.querySelector('small').textContent, 'https://viatge.example.org/');
});

test('Allow answers through /api/winds/answer, folds the card to one line and the wind nods', async () => {
  const f = await askIn({ kind: 'rule', name: 'write', what: 'write /home/anna/gregal/projectes/viatge/informe.md' });
  await byText(f.card, 'button', 'Allow').click();
  await settle();
  assert.deepEqual(f.requests.filter(r => r.url === '/api/winds/answer').map(r => r.body), [{ key: 'k1', approve: true }]);
  assert.equal(f.page.querySelector('.vw-asks').querySelector('.vw-ask'), null, 'the card is gone');
  const line = f.page.querySelector('.vw-ask-done');
  assert.equal(line.textContent, 'You allowed: informe.md');
  assert.ok(line.parentNode === f.page.querySelector('.vw-messages'), 'the line stays in the thread');
  assert.equal(f.corner.dataset.react, 'nod');
});

test('No declines, folds the card to one line and the wind shakes its head', async () => {
  const f = await askIn({ kind: 'install', name: 'bash', what: 'npm install -g serve' });
  await byText(f.card, 'button', 'No').click();
  await settle();
  assert.deepEqual(f.requests.filter(r => r.url === '/api/winds/answer').map(r => r.body), [{ key: 'k1', approve: false }]);
  assert.equal(f.page.querySelector('.vw-ask-done').textContent, 'You said no: npm install -g serve');
  assert.equal(f.corner.dataset.react, 'shake');
});

test('if the answer does not reach the server, the thread does not claim it was allowed', async () => {
  const f = await askIn({ kind: 'install', name: 'bash', what: 'npm install -g serve' }, 'en', { ok: false, status: 409, text: async () => 'no longer pending' });
  await byText(f.card, 'button', 'Allow').click();
  await settle();
  assert.equal(f.page.querySelector('.vw-ask-done'), null);
  assert.equal(f.corner.dataset.react, undefined);
});

test('the approval card speaks Catalan too', async () => {
  const f = await askIn({ kind: 'install', name: 'bash', what: 'npm install -g serve' }, 'ca');
  assert.equal(f.card.querySelector('.vw-ask-step').textContent, 'Preparant l’ordre…');
  assert.deepEqual(buttons(f.card), ['Permet', 'No']);
  await byText(f.card, 'button', 'Permet').click();
  await settle();
  assert.equal(f.page.querySelector('.vw-ask-done').textContent, 'Has permès: npm install -g serve');
});

// The docs side (docs-panel.js) is in docs-panel.test.cjs.
test('only one work pane at a time: the computer pane tells the docs pane to close, and a docs pane closes it', async () => {
  const f = await turnWith(['activity', { phase: 'tool', tool: 'browser' }]);
  assert.ok(f.pane());
  assert.deepEqual(f.workpanes, ['computer'], 'opening the computer announces it, so the docs pane closes');
  f.doc.dispatchEvent(new CustomEvent('gregal:workpane', { detail: 'computer' }));
  assert.ok(f.pane(), 'its own announcement keeps it');
  f.doc.dispatchEvent(new CustomEvent('gregal:workpane', { detail: 'docs' }));
  assert.equal(f.pane(), null, 'the docs pane takes its place');
});

test('a wind’s conversation shows its state in the header: done after a turn, gone once you leave', async () => {
  const f = await boot();
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  assert.ok(f.page.querySelector('.vw-wind-mark').hidden, 'nothing before the first turn');
  assert.ok(f.page.querySelector('.wind-workspace').classList.contains('vw-wind-chat'), 'the page takes the wind’s colour');
  f.page.querySelector('textarea').value = 'Hola';
  await f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  const mark = f.page.querySelector('.vw-wind-mark');
  assert.equal(mark.textContent, 'Done');
  assert.ok(!mark.hidden);
  await byText(f.nav, 'button', 'New conversation').click();
  await settle();
  await byText(f.page.querySelector('.vw-start-recent'), 'button', 'Un concert per dissabte').click();
  await settle();
  assert.ok(f.page.querySelector('.vw-wind-mark').hidden, '«Done» goes away after leaving the conversation');
});

// El vent present (087): a la conversa amb un vent, el vent gran a la cantonada del xat; en un grup, no.
test('a wind’s conversation has the wind large in the chat corner; a group conversation does not', async () => {
  const group = { id: 'r1', name: 'La colla del concert', members: ['xaloc', 'gregal'] };
  const json = data => ({ ok: true, status: 200, json: async () => data });
  const f = await boot({ api: r => r.url === '/api/winds/rotllanes' ? json({ rotllanes: [group] })
    : r.url.startsWith('/api/winds/rotllana?') ? json({ history: [{ from: 'user', text: 'Anem-hi' }] }) : null });
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  const corner = f.page.querySelector('.vw-wind-corner');
  assert.ok(corner, 'the corner wind is there');
  assert.equal(corner.querySelector('svg').dataset.wind, 'Xaloc');
  assert.equal(corner.dataset.mood, 'idle');
  assert.equal(corner.getAttribute('aria-hidden'), 'true', 'the header already names the wind');
  await byText(f.nav, 'button', 'La colla del concert').click();
  await settle();
  assert.equal(f.page.querySelector('.vw-messages').dataset.thread, 'r1');
  assert.equal(f.page.querySelector('.vw-wind-corner'), null, 'no corner wind in a group');
});

// 108: en un xat amb un vent, el vent és un de sol: la cara viu al costat del nom, a dalt, i les seves
// respostes no en porten; en un grup, cada missatge diu qui parla amb la seva cara.
const longChat = (n = 30) => Array.from({ length: n }, (_, i) => i % 2 ? { from: 'assistant', text: 'Resposta ' + i } : { from: 'user', text: 'Pregunta ' + i });
const facesOf = (root, name) => root.querySelectorAll('svg').filter(s => s.dataset.wind === name);
test('a wind’s conversation has exactly one face of the wind, next to its name; a group keeps a face per message', async () => {
  const group = { id: 'r1', name: 'La colla del concert', members: ['xaloc', 'gregal'] };
  const json = data => ({ ok: true, status: 200, json: async () => data });
  const f = await boot({ api: r => r.url === '/api/winds/rotllanes' ? json({ rotllanes: [group] })
    : r.url === '/api/winds/session/open' ? json({ wind: 'xaloc', name: r.body.name, history: longChat(6) })
    : r.url.startsWith('/api/winds/rotllana?') ? json({ history: [{ from: 'user', text: 'Anem-hi' }, { from: 'xaloc', speaker_id: 'xaloc', text: 'Jo hi vaig' }, { from: 'gregal', speaker_id: 'gregal', text: 'I jo' }, { from: 'xaloc', speaker_id: 'xaloc', text: 'Som-hi' }] }) : null });
  await byText(f.page.querySelector('.vw-start-recent'), 'button', 'Un concert per dissabte').click();
  await settle();
  const faces = facesOf(f.page, 'Xaloc');
  assert.equal(faces.length, 1, 'one face of the wind on the page');
  assert.ok(faces[0].closest('.vw-wind-corner'), 'it is the wind that moves');
  assert.ok(faces[0].closest('.vw-owner'), 'its home is the header, next to its name');
  assert.equal(f.page.querySelector('.vw-messages').querySelectorAll('svg').length, 0, 'no face per message');
  await byText(f.nav, 'button', 'La colla del concert').click();
  await settle();
  const rows = f.page.querySelector('.vw-messages').querySelectorAll('article').filter(a => !a.classList.contains('vw-user'));
  assert.deepEqual(rows.map(r => r.querySelector('svg')?.dataset.wind), ['Xaloc', 'Gregal', 'Xaloc'], 'a group tells its speakers apart');
});

// El vent descansa a la cantonada amb el missatge més nou; si tires amunt, vola al costat del nom, i torna.
async function windChat(room) {
  const json = data => ({ ok: true, status: 200, json: async () => data });
  const f = await boot({ api: r => r.url === '/api/winds/session/open' ? json({ wind: 'xaloc', name: r.body.name, history: longChat() }) : null });
  await byText(f.page.querySelector('.vw-start-recent'), 'button', 'Un concert per dissabte').click();
  await settle();
  const box = f.page.querySelector('.vw-messages'), spot = f.page.querySelector('.vw-corner-spot');
  // Al navegador, el lloc de la cantonada només té mida quan hi cap (winds-workspace.css).
  if (room) spot.firstChild.getBoundingClientRect = () => ({ left: 900, top: 600, width: 96, height: 96, right: 996, bottom: 696 });
  const scrollTo = top => { box.scrollTop = top; for (const fn of box.listeners.scroll || []) fn({ target: box }); };
  return { ...f, box, corner: () => f.page.querySelector('.vw-wind-corner'), scrollTo, bottom: () => scrollTo(box.scrollHeight) };
}
test('the wind rests in the corner at the newest message, flies up next to its name when you scroll back, and comes back', async () => {
  const f = await windChat(true);
  f.bottom();
  assert.equal(f.corner().dataset.at, 'corner', 'at the newest message, in the corner');
  f.scrollTo(f.box.scrollTop - 40);
  assert.equal(f.corner().dataset.at, 'corner', 'a small nudge does not make it fly');
  f.scrollTo(0);
  assert.equal(f.corner().dataset.at, 'top', 'reading back, it sits next to its name');
  f.bottom();
  assert.equal(f.corner().dataset.at, 'corner', 'back at the bottom, back in the corner');
  assert.equal(facesOf(f.page, 'Xaloc').length, 1, 'still one face');
});

test('where the corner has no room (narrow screens, a work pane open) the wind stays next to its name, still one', async () => {
  const f = await windChat(false);
  f.bottom();
  assert.equal(f.corner().dataset.at, 'top');
  assert.equal(facesOf(f.page, 'Xaloc').length, 1);
});

test('the flight moves one element with transforms only, and jumps instead of flying with reduced motion', () => {
  const css = fs.readFileSync(path.join(__dirname, 'winds-workspace.css'), 'utf8');
  const block = at => { let depth = 0; const start = css.indexOf('{', at); for (let i = start; i < css.length; i++) { if (css[i] === '{') depth++; else if (css[i] === '}' && --depth === 0) return css.slice(start + 1, i); } return ''; };
  const blocks = [...css.matchAll(/@media\(prefers-reduced-motion:no-preference\)/g)].map(m => block(m.index));
  const still = blocks.reduce((rest, b) => rest.replace(b, ''), css);
  const flights = [...blocks.join('\n').matchAll(/\.vw-wind-corner\[data-fly\][^{]*\{ transition:([^;}]*)/g)].map(m => m[1]);
  assert.equal(flights.length, 2, 'the corner wind flies up and down with motion allowed');
  for (const flight of flights) {
    const props = flight.split(/,(?![^(]*\))/).map(p => p.trim().split(' ')[0]);
    assert.deepEqual(props.filter(p => !['translate', 'transform', 'scale'].includes(p)), [], 'transforms only: ' + props);
  }
  assert.doesNotMatch(still, /\.vw-wind-corner[^{]*\{[^}]*transition:/, 'no flight under reduced motion');
});

// 085: la tinta de la bombolla depèn del tema; canviar de clar a fosc amb el xat obert la refà de seguida.
test('switching between dark and light with a wind’s chat open retints its bubbles right away', async () => {
  const themes = {
    dark: { '--surface-hi': '#24282D', '--text': '#DEE2E7', '--ink': '#F2F4F7', '--muted': '#A7AEB8' },
    light: { '--surface-hi': '#DEE5DE', '--text': '#1C2D35', '--ink': '#10202A', '--muted': '#4C5F5C' },
  };
  let theme = 'dark';
  const f = await boot({ css: n => themes[theme][n] || '' });
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  const ws = f.page.querySelector('.wind-workspace'), { share } = f.window.gregalWindChat;
  const expected = th => String(share('#F5B45D', themes[th]['--surface-hi'], ['--text', '--ink', '--muted'].map(n => themes[th][n])));
  assert.notEqual(expected('dark'), expected('light'), 'the two themes give different shares');
  assert.equal(ws.style['--vw-wind-share'], expected('dark'));
  theme = 'light';
  f.mutate(f.doc.documentElement, 'data-theme');
  assert.equal(ws.style['--vw-wind-share'], expected('light'), 'the light share, without a redraw');
});

// 089: el panell de Gregal Docs i el de l'ordinador passen a flotar sobre el xat a la mateixa amplada.
test('the docs pane and the computer pane switch to an overlay at the same width', () => {
  const docs = fs.readFileSync(path.join(__dirname, 'docs-panel.js'), 'utf8');
  const css = fs.readFileSync(path.join(__dirname, 'winds-workspace.css'), 'utf8');
  const docsAt = docs.match(/@media\(([^)]*)\)\{#docsPanel\{position:fixed/)?.[1];
  const computerAt = css.match(/@media\(([^)]*)\) \{ \.vw-computer-pane \{ position:fixed/)?.[1];
  assert.ok(docsAt && computerAt, 'both panes have an overlay rule');
  assert.equal(computerAt, docsAt);
});

// 092: «Obre el xat» d'una novetat obre el xat del vent al missatge exacte i la marca com a llegida.
test('opening the chat from a news item lands on that message and marks the news read', async () => {
  let open;
  const json = data => ({ ok: true, status: 200, json: async () => data });
  const history = [{ id: 'm1', from: 'assistant', text: 'Bon dia' }, { id: 'm2', from: 'assistant', text: 'Hi ha jazz al Born' }, { id: 'm3', from: 'user', text: 'Gràcies' }];
  const f = await boot({
    extra: { gregalVentsActivity: { renderActivity(box, opts) { open = opts.onOpen; } } },
    api: r => r.url.startsWith('/api/winds/chat?') ? json({ session: 'company-xaloc', history })
      : r.url === '/api/winds/novetats?id=n-7' ? json({ novetat: { id: 'n-7', wind: 'xaloc', message_id: 'm2' } }) : null,
  });
  await byText(f.nav, 'button', 'Work').click();
  await settle();
  await byText(f.page.querySelector('.vw-page-tabs'), 'button', 'Activity').click();
  await settle();
  assert.ok(open, 'the activity view is drawn');
  await open({ kind: 'news', wind: 'xaloc', ref: 'n-7' }, 'chat');
  await settle();
  assert.equal(f.page.querySelector('.vw-messages').dataset.thread, 'company-xaloc');
  const focused = f.page.querySelectorAll('.vw-message-focus');
  assert.equal(focused.length, 1);
  assert.match(focused[0].textContent, /Hi ha jazz al Born/);
  assert.deepEqual(f.requests.filter(r => r.url === '/api/winds/novetats/read').map(r => [r.method, r.body]), [['POST', { ids: ['n-7'] }]]);
});

// 101: un torn que surt bé el celebra el vent, un sol cop; un torn que falla, una arronsada d'espatlles.
// Des de 108 la cara de la capçalera i la de la cantonada són la mateixa.
async function turnInXaloc(api) {
  const f = await boot({ api });
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  f.page.querySelector('textarea').value = 'Hola';
  await f.page.querySelector('.vw-composer').requestSubmit()?.catch?.(() => {});
  await settle();
  return f;
}
const cheered = (f, cls) => f.page.querySelector('.vw-wind-corner').classList.contains(cls);

test('a turn that ends well makes the wind celebrate, once', async () => {
  const f = await turnInXaloc();
  assert.ok(cheered(f, 'vw-cheer'), 'the wind celebrates');
  assert.ok(!cheered(f, 'vw-shrug'));
  assert.equal(f.page.querySelectorAll('.vw-burst').length, 1, 'one sparkle burst');
  await f.window.loadWindsPage();
  await settle();
  assert.equal(f.page.querySelector('.vw-wind-mark').textContent, 'Done', 'still the same finished turn');
  assert.ok(!cheered(f, 'vw-cheer'), 'a redraw does not celebrate again');
  assert.equal(f.page.querySelectorAll('.vw-burst').length, 0);
});

test('a turn that fails gets a shrug, not a celebration', async () => {
  const f = await turnInXaloc(r => r.url === '/api/winds/chat/stream' ? { ok: false, status: 502, text: async () => 'boom', json: async () => ({}) } : null);
  assert.ok(cheered(f, 'vw-shrug'));
  assert.ok(!cheered(f, 'vw-cheer'));
  assert.equal(f.page.querySelectorAll('.vw-burst').length, 0);
});

// 101: els missatges nous entren amb una molla, la bombolla d'espera respira i els panells hi llisquen;
// tot plegat només amb moviment permès, i només amb transform i opacity.
test('new messages spring in, the waiting bubble breathes and side panes slide in, all on transform and opacity', () => {
  const css = fs.readFileSync(path.join(__dirname, 'winds-workspace.css'), 'utf8');
  const block = at => { let depth = 0; const start = css.indexOf('{', at); for (let i = start; i < css.length; i++) { if (css[i] === '{') depth++; else if (css[i] === '}' && --depth === 0) return css.slice(start + 1, i); } return ''; };
  const blocks = [...css.matchAll(/@media\(prefers-reduced-motion:no-preference\)/g)].map(m => block(m.index)), motion = blocks.join('\n');
  const still = blocks.reduce((rest, b) => rest.replace(b, ''), css);
  for (const name of ['vw-in', 'vw-bubble', 'vw-pane-in', 'vw-cheer', 'vw-spark', 'vw-ring', 'vw-shrug']) {
    assert.match(motion, new RegExp('animation:' + name + ' '), name + ' plays with motion allowed');
    assert.doesNotMatch(still, new RegExp('animation:' + name + ' '), name + ' never plays under reduced motion');
    const at = css.indexOf('@keyframes ' + name + ' ');
    assert.ok(at >= 0, name + ' has keyframes');
    const props = [...block(at).matchAll(/([a-z-]+)\s*:/g)].map(m => m[1]).filter(p => !['transform', 'opacity'].includes(p));
    assert.deepEqual(props, [], name + ' animates transform and opacity only');
  }
  assert.match(motion, /\.vw-pending-copy[^{]*\{ animation:vw-bubble /);
  assert.match(motion, /\.vw-computer-pane[^{]*#docsPanel[^{]*\{ animation:vw-pane-in /);
});

// 099: una novetat que arriba amb Vents obert es nota al moment.
const ponent = { id: 'n-9', wind: 'xaloc', name: 'Xaloc', text: '# Concerts\nHi ha **jazz** al Born dissabte', ts: 1, message_id: 'm2' };
async function hear(f, list) { f.window.gregalWinds.news.push(...list.map(n => n.wind)); await f.window.gregalWinds.onNews(list); await settle(); }
const toasts = f => f.doc.body.querySelectorAll('.vw-toast');

test('a new novetat bounces and badges the wind’s face in the bar and on the start page, refreshes the recents and slides in a toast that opens the message', async () => {
  let fresh = false;
  const json = data => ({ ok: true, status: 200, json: async () => data });
  const history = [{ id: 'm1', from: 'assistant', text: 'Bon dia' }, { id: 'm2', from: 'assistant', text: 'Hi ha jazz al Born dissabte' }];
  const f = await boot({ api: r => r.url === '/api/winds/threads' && fresh ? json({ threads: [{ wind: 'xaloc', name: 'company-xaloc', title: 'Concerts', at: new Date().toISOString(), msgs: 1 }, ...threads] })
    : r.url.startsWith('/api/winds/chat?') ? json({ session: 'company-xaloc', history })
    : r.url === '/api/winds/novetats?id=n-9' ? json({ novetat: ponent }) : null });
  assert.equal(typeof f.window.gregalWinds.onNews, 'function', 'the page listens to watchNews');
  assert.equal(toasts(f).length, 0);
  fresh = true;
  await hear(f, [ponent]);
  for (const box of [f.nav, f.page.querySelector('.vw-start-faces')]) {
    const xaloc = box.querySelectorAll('button').find(b => b.dataset.wind === 'xaloc');
    assert.ok(xaloc.querySelector('.vw-news-dot'), 'badge on Xaloc');
    assert.ok(xaloc.classList.contains('vw-bounce'), 'Xaloc bounces');
    assert.ok(!box.querySelectorAll('button').find(b => b.dataset.wind === 'gregal').querySelector('.vw-news-dot'), 'not on Gregal');
  }
  assert.match(f.page.querySelector('.vw-start-recent').textContent, /Concerts/, 'the recents list is fresh');
  assert.ok(f.nav.querySelectorAll('.vw-thread-solo').some(b => /Concerts/.test(b.textContent)), 'and so is the bar');
  assert.equal(toasts(f).length, 1);
  assert.match(toasts(f)[0].textContent, /Xaloc wrote to you: Concerts Hi ha jazz al Born dissabte/);
  await toasts(f)[0].querySelector('.vw-toast-open').click();
  await settle();
  assert.equal(f.page.querySelector('.vw-messages').dataset.thread, 'company-xaloc');
  assert.match(f.page.querySelector('.vw-message-focus').textContent, /Hi ha jazz/);
  assert.equal(toasts(f).length, 0, 'the toast goes once opened');
});

test('the toast speaks Catalan too', async () => {
  const f = await boot({ lang: 'ca' });
  await hear(f, [ponent]);
  assert.match(toasts(f)[0].textContent, /Xaloc t’ha escrit: /);
});

test('with the tab hidden a novetat is a system notification, not a toast; visible, a toast', async () => {
  const shown = [];
  class Notification { constructor(title, opts) { shown.push([title, opts.body]); } close() {} }
  Notification.permission = 'granted';
  const f = await boot({ extra: { Notification, focus() {} } });
  f.doc.hidden = true;
  await hear(f, [ponent]);
  assert.deepEqual(shown, [['Xaloc', 'Concerts Hi ha jazz al Born dissabte']]);
  assert.equal(toasts(f).length, 0);
  f.doc.hidden = false;
  await hear(f, [{ ...ponent, id: 'n-10' }]);
  assert.equal(shown.length, 1);
  assert.equal(toasts(f).length, 1);
});

test('on desktop a hidden window uses the app’s native notification', async () => {
  const sent = [];
  const f = await boot({ extra: { gregalDesktop: { notify: (title, body) => { sent.push([title, body]); return Promise.resolve(true); } } } });
  f.doc.hidden = true;
  await hear(f, [ponent]);
  assert.deepEqual(sent, [['Xaloc', 'Concerts Hi ha jazz al Born dissabte']]);
  assert.equal(toasts(f).length, 0);
});

test('notification permission is asked once, from a button in the toast, never on load', async () => {
  let asked = 0;
  class Notification {}
  Notification.permission = 'default';
  Notification.requestPermission = async () => { asked++; return 'granted'; };
  const storage = {};
  const f = await boot({ storage, extra: { Notification } });
  assert.equal(asked, 0, 'not on load');
  await hear(f, [ponent]);
  const allow = byText(toasts(f)[0], 'button', 'Notify me when I’m away');
  assert.ok(allow);
  await allow.click();
  await settle();
  assert.equal(asked, 1);
  assert.ok(!byText(toasts(f)[0], 'button', 'Notify me when I’m away'), 'the button goes');
  await hear(f, [{ ...ponent, id: 'n-11' }]);
  assert.ok(!toasts(f).some(n => byText(n, 'button', 'Notify me when I’m away')), 'asked once');
});

// 113: on els vents es veuen grans (la cantonada del xat, el perfil, les cares de l'inici, Avui) hi
// juga la plastilina (characters.js); les cares petites de les llistes queden dibuixades.
function clayBoot(api) {
  const played = [], moods = [];
  const gregalCharacters = {
    play(svg, w) { played.push(w.id); const c = new Element('canvas', svg.doc); c.className = 'winds-face winds-clay'; c.dataset.wind = svg.dataset.wind; return c; },
    mood(el, mood) { moods.push([el.dataset.wind, mood]); return el; },
  };
  return boot({ extra: { gregalCharacters }, api }).then(f => ({ ...f, played, moods }));
}
const clayOf = root => root.querySelectorAll('canvas').map(c => c.dataset.wind);

test('large faces play the clay winds: start faces, the chat corner, Today and the profile; list avatars stay drawn', async () => {
  const card = { id: 'c1', wind: 'xaloc', title: 'Concert dissabte', source: 'wind' };
  // Prou vents perquè la barra porti a «All winds», on hi ha els perfils.
  const four = [...winds, { id: 'ponent', name: 'Ponent', color: '#EACF87', style: 'msn' }, { id: 'mestral', name: 'Mestral', color: '#BFCFA4', style: 'esn' }];
  const json = data => ({ ok: true, status: 200, json: async () => data });
  const f = await clayBoot(r => r.url === '/api/winds/avui' ? json({ date: '2026-11-24', now: [card] }) : r.url === '/api/winds' && r.method === 'GET' ? json({ winds: four })
    : r.url === '/api/winds/tidy' ? json({ groups: [], test: [], unused: [] }) : null);
  assert.deepEqual(clayOf(f.page.querySelector('.vw-start-faces')), ['Gregal', 'Xaloc', 'Ponent', 'Mestral']);
  assert.deepEqual(clayOf(f.nav), [], 'the sidebar keeps its drawn faces');
  assert.deepEqual(clayOf(f.page.querySelector('.vw-start-recent')), [], 'so do the recent rows');
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  assert.deepEqual(clayOf(f.page.querySelector('.vw-wind-corner')), ['Xaloc']);
  assert.deepEqual(clayOf(f.page.querySelector('.vw-messages')), []);
  await byText(f.nav, 'button', 'Today').click();
  await settle();
  assert.deepEqual(clayOf(f.page.querySelector('.vw-avui-card')), ['Xaloc']);
  await byText(f.nav, 'button', 'All winds').click(); await settle();
  f.played.length = 0;
  await byText(byText(f.page, '.vw-member', 'Xaloc'), 'button', 'Profile').click(); await settle();
  assert.deepEqual(f.played, ['xaloc'], 'the profile plays it; the roster rows stay drawn');
});

test('the clay corner changes mood in place, so it can crossfade, and still nods', async () => {
  const json = data => ({ ok: true, status: 200, json: async () => data });
  const f = await clayBoot(r => r.url === '/api/winds/pending' ? json({ pending: [{ key: 'k1', type: 'approve', wind: 'xaloc', kind: 'rule', name: 'write', what: 'write informe.md' }] })
    : r.url === '/api/winds/answer' ? json({ ok: true, id: 'k1', state: 'approved' }) : null);
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  const corner = f.page.querySelector('.vw-wind-corner'), face = corner.firstChild;
  assert.equal(corner.dataset.mood, 'waiting');
  await byText(f.page.querySelector('.vw-asks'), 'button', 'Allow').click();
  await settle();
  assert.equal(corner.firstChild, face, 'the same clay face');
  assert.deepEqual(f.moods.at(-1), ['Xaloc', corner.dataset.mood]);
  assert.notEqual(corner.dataset.mood, 'waiting');
  assert.equal(corner.dataset.react, 'nod');
});

// 126: el lateral del xat d'un vent s'obre i es tanca quan vols. Cada torn és
// una resposta de la cua; el panell de documents (docs-panel.js) és fals:
// s'obre amb changed/open i la seva × avisa gregal:docsclosed, com el de debò.
async function paneChat(storage = {}) {
  const turns = [];
  const f = await boot({ storage, api: r => {
    if (r.url !== '/api/winds/chat/stream') return null;
    const frames = [...(turns.shift() || []), ['done', { history: [{ from: 'assistant', text: 'Fet' }] }]].map(([k, d]) => 'event: ' + k + '\ndata: ' + JSON.stringify(d) + '\n\n').join('');
    const bytes = new TextEncoder().encode(frames);
    let sent = false;
    return { ok: true, status: 200, body: { getReader: () => ({ read: async () => { if (sent) return { done: true }; sent = true; return { done: false, value: bytes }; }, cancel: async () => {}, releaseLock() {} }) } };
  } });
  const docs = f.doc.createElement('aside'), close = f.doc.createElement('button');
  docs.setAttribute('id', 'docsPanel'); close.setAttribute('id', 'docsClose'); docs.hidden = true; docs.append(close);
  f.doc.getElementById('workbench').append(docs);
  const show = () => { docs.hidden = false; f.doc.dispatchEvent(new CustomEvent('gregal:workpane', { detail: 'docs' })); };
  close.onclick = () => { docs.hidden = true; f.doc.dispatchEvent(new CustomEvent('gregal:docsclosed')); };
  const changed = [], opened = [];
  f.window.gregalDocs = { changed: d => { changed.push(d.id); show(); }, open: (id, scope) => { opened.push([id, scope]); show(); } };
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  const turn = async (...events) => { turns.push(events); f.page.querySelector('textarea').value = 'Fes-ho'; await f.page.querySelector('.vw-composer').requestSubmit(); await settle(); };
  return { ...f, docs, close, changed, opened, turn, toggle: () => f.page.querySelector('.vw-pane-toggle'), computer: () => f.doc.getElementById('workbench').querySelector('.vw-computer-pane') };
}
const letter = ['doc_changed', { id: 'w1', title: 'Carta', scope: 'mine' }];

test('a wind chat with a document has a header toggle that closes the side pane and reopens the same document', async () => {
  const f = await paneChat();
  assert.ok(!f.toggle() || f.toggle().hidden, 'no toggle before there is anything to show');
  await f.turn(letter);
  assert.equal(f.docs.hidden, false, 'the document opens beside the chat');
  const tg = f.toggle();
  assert.ok(tg && !tg.hidden, 'the toggle is in the header');
  assert.equal(tg.getAttribute('aria-pressed'), 'true');
  assert.match(tg.getAttribute('aria-label'), /document/);
  await tg.click();
  assert.equal(f.docs.hidden, true, 'closed');
  assert.equal(f.toggle().getAttribute('aria-pressed'), 'false');
  await f.toggle().click();
  assert.deepEqual(f.opened, [['w1', 'mine']], 'the same document again');
  assert.equal(f.docs.hidden, false);
  assert.equal(f.toggle().getAttribute('aria-pressed'), 'true');
});

test('the header toggle closes and reopens the computer view too', async () => {
  const f = await paneChat();
  await f.turn(['activity', { phase: 'tool', tool: 'browser' }]);
  assert.ok(f.computer(), 'the computer opens beside the chat');
  assert.equal(f.toggle().getAttribute('aria-pressed'), 'true');
  assert.match(f.toggle().getAttribute('aria-label'), /computer/);
  await f.toggle().click();
  assert.equal(f.computer(), null, 'closed');
  assert.equal(f.toggle().getAttribute('aria-pressed'), 'false');
  await f.toggle().click();
  assert.ok(f.computer(), 'open again');
});

test('a new version of the document leaves a pane the person closed closed, with a dot on the toggle', async () => {
  const f = await paneChat();
  await f.turn(letter);
  await f.close.click();
  assert.equal(f.toggle().getAttribute('aria-pressed'), 'false', 'the × is the person closing it too');
  await f.turn(letter);
  assert.deepEqual(f.changed, ['w1'], 'the wind does not open it again');
  assert.equal(f.docs.hidden, true);
  assert.equal(f.toggle().dataset.fresh, 'true', 'the dot says there is something new');
  assert.match(f.toggle().getAttribute('aria-label'), /new/);
  await f.toggle().click();
  assert.deepEqual(f.opened, [['w1', 'mine']]);
  assert.notEqual(f.toggle().dataset.fresh, 'true', 'seen');
});

test('a new version of the document opens the pane when the person never closed it in this chat', async () => {
  const f = await paneChat();
  await f.turn(letter);
  f.docs.hidden = true;
  await f.turn(letter);
  assert.deepEqual(f.changed, ['w1', 'w1']);
  assert.equal(f.docs.hidden, false);
  assert.notEqual(f.toggle().dataset.fresh, 'true');
});

test('the side pane width is dragged within a minimum for chat and pane, and remembered', async () => {
  const storage = {};
  const f = await paneChat(storage);
  const bench = f.doc.getElementById('workbench');
  bench.getBoundingClientRect = () => ({ left: 0, width: 1200 });
  await f.turn(['activity', { phase: 'tool', tool: 'browser' }]);
  const grip = f.computer().querySelector('.vw-pane-grip');
  assert.ok(grip, 'the computer pane has a grip');
  assert.equal(grip.getAttribute('role'), 'separator');
  f.computer().getBoundingClientRect = () => ({ width: 500 });
  const key = k => grip.listeners.keydown[0]({ key: k, preventDefault() {} });
  key('ArrowLeft');
  assert.equal(bench.style['--work-pane-w'], '524px', 'wider');
  assert.equal(storage.gregal_work_pane_w, '524');
  for (let i = 0; i < 40; i++) key('ArrowLeft');
  assert.equal(bench.style['--work-pane-w'], '780px', 'the chat keeps 420 px');
  for (let i = 0; i < 40; i++) key('ArrowRight');
  assert.equal(bench.style['--work-pane-w'], '340px', 'the pane keeps 340 px');
  const moves = {};
  f.window.addEventListener = (k, fn) => { moves[k] = fn; };
  f.window.removeEventListener = () => {};
  grip.listeners.pointerdown[0]({ clientX: 700, preventDefault() {} });
  moves.pointermove({ clientX: 600 });
  moves.pointerup({ clientX: 650 });
  assert.equal(bench.style['--work-pane-w'], '550px', 'dragging the grip');
  assert.equal(storage.gregal_work_pane_w, '550');
  await f.turn(letter);
  assert.ok(f.docs.querySelector('.vw-pane-grip'), 'the document pane has the grip too');
  const again = await boot({ storage });
  assert.equal(again.doc.getElementById('workbench').style['--work-pane-w'], '550px', 'remembered');
  const css = fs.readFileSync(path.join(__dirname, 'winds-workspace.css'), 'utf8');
  assert.match(css, /@media\(max-width:1000px\) \{ \.vw-computer-pane \{[^}]*\} \.vw-pane-grip \{ display:none; \} \}/, 'no grip on the overlay of narrow screens');
});
