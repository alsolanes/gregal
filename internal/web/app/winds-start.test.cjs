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
    addEventListener(k, f) { (events[k] ||= []).push(f); }, removeEventListener(k, f) { events[k] = (events[k] || []).filter(x => x !== f); }, dispatchEvent(e) { for (const f of events[e.type] || []) f(e); } };
  doc.root = new Element('root', doc);
  for (const id of ['ventsPage', 'windsListCard', 'side', 'workbench', 'racoPage']) { const n = new Element('div', doc); n.setAttribute('id', id); doc.root.append(n); }
  // Com a index.html: la barra dels vents és dins la part que es desplaça de #side.
  const scroller = new Element('div', doc), sideNav = new Element('div', doc);
  scroller.className = 'side-scroll'; sideNav.setAttribute('id', 'windContextNav');
  scroller.append(sideNav); doc.root.querySelector('#side').append(scroller);
  doc.getElementById = id => doc.root.querySelector('#' + id);
  doc.body = new Element('body', doc);
  const requests = [];
  const json = data => ({ ok: true, status: 200, json: async () => data, text: async () => JSON.stringify(data) });
  const window = {
    document: doc, gregalNotes: notes, addEventListener() {}, gregalGreeting: () => 'Good evening', ...extra,
    gregalWinds: { face: (w, large, mood, size) => { const n = new Element('svg', doc); n.dataset.wind = w?.name || ''; n.dataset.style = w?.style || ''; if (size) n.dataset.size = String(size); return n; }, moodOf: () => '', seen() {}, watchNews(f) { this.onNews = f; }, news: [], hasNews(id) { return this.news.includes(id); }, setAvatarTheme(theme) { this.theme = theme; },
      // winds.js draws the real pickers (winds-avatar-picker.test.cjs); here only Free mode gets a pack one.
      avatarField(parent, w) { if (this.theme !== 'free') return null; const s = new Element('select', doc); s.setAttribute('id', 'windAvatar'); s.value = w?.avatar || 'cares'; parent.append(s); return s; },
      avatarPack(w) { return this.theme !== 'free' ? this.theme : w.avatar || 'cares'; },
      barretinaField(parent, w) { const wrap = new Element('label', doc), s = new Element('select', doc); s.setAttribute('id', 'windBarretina'); s.value = String(w.style || '')[3] || ''; wrap.append(s); parent.append(wrap); return s; } },
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

// 132 (com 133 a l'Android): tocar una cara repren el fil més nou del vent, o el seu xat de sempre; no en crea cap.
test('tapping a face resumes that wind’s newest thread, or its usual chat; a recent one opens through session/open', async () => {
  const f = await boot({ api: r => r.url === '/api/winds' ? { ok: true, status: 200, json: async () => ({ winds: [...winds, { id: 'mestral', name: 'Mestral', color: '#7BBF6A', style: 'pso' }] }) } : null });
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  assert.ok(!f.requests.some(r => r.url === '/api/winds/sessions' && r.method === 'POST'), 'no new conversation');
  assert.deepEqual(f.requests.find(r => r.url === '/api/winds/session/open').body, { wind: 'xaloc', name: 'company-xaloc-s2' });
  assert.ok(f.page.querySelector('.vw-messages'), 'a thread is open');
  assert.match(f.page.querySelector('.vw-top').textContent, /Xaloc/);
  assert.match(f.page.querySelector('.vw-messages').textContent, /Hi ha jazz al Born/);

  await byText(f.nav, 'button', 'New conversation').click();
  await settle();
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Mestral').click();
  await settle();
  assert.ok(f.requests.some(r => r.url === '/api/winds/chat?wind=mestral'), 'a wind with no thread opens its usual chat');
  assert.ok(!f.requests.some(r => r.url === '/api/winds/sessions' && r.method === 'POST'), 'still no new conversation');
  assert.match(f.page.querySelector('.vw-top').textContent, /Mestral/);

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
  // More winds than the sidebar lists; the profiles are under «All winds».
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

const changed = n => { for (const f of n.listeners.change || []) f({ target: n }); };
const headStyle = f => f.doc.body.querySelector('.vw-profile-head').querySelector('svg').dataset.style;

test('a wind’s profile picks its barretina while it is drawn as a minairó, saved as the fourth letter of its style', async () => {
  const f = await avatarBoot('free');
  await byText(f.nav, 'button', 'All winds').click(); await settle();
  await byText(byText(f.page, '.vw-member', 'Xaloc'), 'button', 'Profile').click(); await settle();
  const hat = f.doc.body.querySelector('#windBarretina'), pick = f.doc.body.querySelector('#windAvatar');
  assert.equal(hat.parentNode.hidden, true, 'hidden while Xaloc is a wind spirit');
  pick.value = 'minairo'; await pick.onchange();
  assert.equal(hat.parentNode.hidden, false, 'shown once it is a minairó');
  hat.value = 'm'; await hat.onchange();
  assert.deepEqual([lastWindPost(f).id, lastWindPost(f).style], ['xaloc', 'psom']);
  assert.equal(headStyle(f), 'psom', 'the profile face wears it');
  hat.value = ''; await hat.onchange();
  assert.equal(lastWindPost(f).style, 'pso', 'Automàtica keeps the three letters');
});

test('with the Minairons theme the profile shows the barretina picker, and the create dialog saves and previews it', async () => {
  const f = await avatarBoot('minairo');
  await byText(f.nav, 'button', 'All winds').click(); await settle();
  await byText(byText(f.page, '.vw-member', 'Xaloc'), 'button', 'Profile').click(); await settle();
  assert.equal(f.doc.body.querySelector('#windBarretina').parentNode.hidden, false);
  f.doc.body.querySelector('dialog').close();
  await byText(f.page, 'button', 'Create your own wind').click(); await settle();
  const dialog = f.doc.body.children.at(-1), hat = dialog.querySelector('#windBarretina');
  assert.equal(hat.parentNode.hidden, false);
  hat.value = 'o'; changed(hat);
  assert.equal(dialog.querySelector('.vw-wind-preview').querySelector('svg').dataset.style, 'psno', 'the preview follows');
  dialog.querySelector('input').value = 'Lupa';
  await byText(dialog, 'button', 'Create wind').click();
  assert.deepEqual([lastWindPost(f).name, lastWindPost(f).style], ['Lupa', 'psno']);
});

test('in the create dialog the barretina picker shows only with the Minairons pack', async () => {
  const f = await avatarBoot('free');
  await byText(f.nav, 'button', 'All winds').click(); await settle();
  await byText(f.page, 'button', 'Create your own wind').click(); await settle();
  const dialog = f.doc.body.children.at(-1), hat = dialog.querySelector('#windBarretina'), pick = dialog.querySelector('#windAvatar');
  assert.equal(hat.parentNode.hidden, true);
  pick.value = 'minairo'; changed(pick);
  assert.equal(hat.parentNode.hidden, false);
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
  assert.match(pane.textContent, /Gregal’s computer/);
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
async function askIn(ask, lang = 'en', answer = { ok: true, status: 200, json: async () => ({ ok: true, id: 'k1', state: 'approved' }) }, css) {
  const json = data => ({ ok: true, status: 200, json: async () => data });
  const f = await boot({ lang, css, api: r => r.url === '/api/winds/pending' ? json({ pending: [{ key: 'k1', type: 'approve', wind: 'xaloc', ...ask }] })
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
  assert.equal(f.card.querySelector('.vw-ask-why').textContent, 'Asking because of your rule: «Ask me before buying».');
  assert.equal(f.card.querySelector('.vw-ask-sub'), null, 'the reason is said once, on its own line');
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

// 130: la targeta és del vent que pregunta: la seva cara, la vora del seu color, el perquè en una línia
// i Permet amb el color del vent, amb la tinta del tema que s'hi llegeix (wind-chat-color.test.cjs).
const themeInks = {
  dark: { '--ink': '#F2F4F7', '--bg': '#131518' },
  light: { '--ink': '#10202A', '--bg': '#F4F6F3' },
};
test('an approval card carries the asking wind: its face, its colour, one line of why and Allow in its colour', async () => {
  const f = await askIn({ kind: 'rule', name: 'bash', what: 'rm -rf build', reason: 'Asking because of your rule: «Ask me before deleting».' }, 'en', undefined, n => themeInks.dark[n] || '');
  assert.equal(f.card.querySelector('.vw-ask-head').querySelector('svg').dataset.wind, 'Xaloc', 'the wind’s face, not a shield');
  assert.ok(f.card.classList.contains('vw-ask-wind'));
  assert.equal(f.card.style['--vw-wind'], '#F5B45D');
  assert.equal(f.card.querySelector('.vw-ask-why').textContent, 'Asking because of your rule: «Ask me before deleting».');
  const { fill, ink } = f.window.gregalWindChat.allow('#F5B45D', ['#F2F4F7', '#131518']);
  assert.equal(f.card.style['--vw-allow'], fill);
  assert.equal(f.card.style['--vw-allow-ink'], ink);
});

test('switching between dark and light with an approval card open repaints its Allow button right away', async () => {
  let theme = 'dark';
  const f = await askIn({ kind: 'install', name: 'bash', what: 'npm install -g serve' }, 'en', undefined, n => themeInks[theme][n] || '');
  f.card.querySelector('.vw-ask-actions').append(f.doc.createElement('i'));
  const { allow } = f.window.gregalWindChat;
  assert.equal(f.card.style['--vw-allow-ink'], allow('#F5B45D', ['#F2F4F7', '#131518']).ink);
  theme = 'light';
  f.mutate(f.doc.documentElement, 'data-theme');
  assert.equal(f.card.style['--vw-allow-ink'], allow('#F5B45D', ['#10202A', '#F4F6F3']).ink);
  assert.equal(f.page.querySelector('.vw-ask'), f.card, 'the same card, not a redraw');
});

test('an approval with no reason has no why line, and a card from an unknown wind keeps the shield', async () => {
  const f = await askIn({ kind: 'install', name: 'bash', what: 'npm install -g serve', wind: 'nobody' });
  assert.equal(f.card.querySelector('.vw-ask-why'), null);
  assert.ok(f.card.querySelector('.vw-ask-shield'));
  assert.ok(!f.card.classList.contains('vw-ask-wind'));
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

// 130: en un grup, la cara (20 px) només al primer missatge d'una ratxa del mateix vent, i cada vent té
// una sola forma de missatge, sigui curt o llarg.
test('in a group a wind’s small face starts its run only, and a long reply looks like a short one', async () => {
  const group = { id: 'r1', name: 'La colla del concert', members: ['xaloc', 'gregal'] };
  const json = data => ({ ok: true, status: 200, json: async () => data });
  const long = 'Taula:\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n```\ncodi\n```\n' + 'llarg '.repeat(200);
  const f = await boot({ api: r => r.url === '/api/winds/rotllanes' ? json({ rotllanes: [group] })
    : r.url.startsWith('/api/winds/rotllana?') ? json({ history: [{ from: 'user', text: 'Anem-hi' }, { from: 'xaloc', speaker_id: 'xaloc', text: 'Jo hi vaig' }, { from: 'xaloc', speaker_id: 'xaloc', text: long }, { from: 'gregal', speaker_id: 'gregal', text: 'I jo' }] }) : null });
  await byText(f.nav, 'button', 'La colla del concert').click();
  await settle();
  const rows = f.page.querySelector('.vw-messages').querySelectorAll('article').filter(a => !a.classList.contains('vw-user'));
  assert.deepEqual(rows.map(r => r.querySelector('svg')?.dataset.wind), ['Xaloc', undefined, 'Gregal'], 'the face only starts a run');
  assert.deepEqual(rows.map(r => r.querySelector('svg')?.dataset.size), ['20', undefined, '20']);
  const look = r => r.className.split(' ').filter(c => !c.startsWith('vw-run-')).sort().join(' ');
  assert.equal(look(rows[1]), look(rows[0]), 'the long reply has the same shape as the short one');
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
const reacted = f => f.page.querySelector('.vw-wind-corner').dataset.react;

// 128: la celebració és ara el vent que s'hi acosta (lean) amb l'esclat, i l'arronsada, fer que no (shake).
test('a turn that ends well makes the wind lean in with a burst, once', async () => {
  const f = await turnInXaloc();
  assert.equal(reacted(f), 'lean', 'the wind leans in');
  assert.equal(f.page.querySelectorAll('.vw-burst').length, 1, 'one sparkle burst');
  await f.window.loadWindsPage();
  await settle();
  assert.equal(f.page.querySelector('.vw-wind-mark').textContent, 'Done', 'still the same finished turn');
  assert.equal(reacted(f), undefined, 'a redraw does not react again');
  assert.equal(f.page.querySelectorAll('.vw-burst').length, 0);
});

test('a turn that fails gets a shake, not a burst', async () => {
  const f = await turnInXaloc(r => r.url === '/api/winds/chat/stream' ? { ok: false, status: 502, text: async () => 'boom', json: async () => ({}) } : null);
  assert.equal(reacted(f), 'shake');
  assert.equal(f.page.querySelectorAll('.vw-burst').length, 0);
});

// 101: els missatges nous entren amb una molla, la bombolla d'espera respira i els panells hi llisquen;
// tot plegat només amb moviment permès, i només amb transform i opacity.
test('new messages spring in, the waiting bubble breathes and side panes slide in, all on transform and opacity', () => {
  const css = fs.readFileSync(path.join(__dirname, 'winds-workspace.css'), 'utf8');
  const block = at => { let depth = 0; const start = css.indexOf('{', at); for (let i = start; i < css.length; i++) { if (css[i] === '{') depth++; else if (css[i] === '}' && --depth === 0) return css.slice(start + 1, i); } return ''; };
  const blocks = [...css.matchAll(/@media\(prefers-reduced-motion:no-preference\)/g)].map(m => block(m.index)), motion = blocks.join('\n');
  const still = blocks.reduce((rest, b) => rest.replace(b, ''), css);
  for (const name of ['vw-in', 'vw-bubble', 'vw-pane-in', 'vw-spark', 'vw-ring']) {
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
  // Més vents dels que llista la barra; els perfils són a «All winds».
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

// A l'escriptori, «Tots els vents» és l'única porta a l'equip: crear un vent
// i obrir-ne el perfil (paquet d'avatar, prestatge, activitat). Només sortia
// amb més vents dels que la barra llista, així que amb Gregal i pocs més no
// hi havia manera de crear-ne cap; al mòbil sí, per la barra de sota.
test('with every wind already in the sidebar, it still links to «All winds», to create one and open profiles', async () => {
  const f = await boot({ api: r => r.url === '/api/winds/tidy' ? { ok: true, status: 200, json: async () => ({ groups: [], test: [], unused: [] }) } : null });
  const all = byText(f.nav, 'button', 'All winds');
  assert.ok(all, 'the sidebar links to All winds');
  await all.click(); await settle();
  assert.ok(byText(f.page, 'button', 'Create your own wind'));
  assert.ok(byText(byText(f.page, '.vw-member', 'Xaloc'), 'button', 'Profile'));
});

// ---- 127: coherència ----
// L'ordre en què es llegeix la pàgina (com el DOM: pare, després fills).
const inOrder = root => { const out = []; const walk = n => { out.push(n); n.children.forEach(walk); }; walk(root); return out; };
// Les regles d'un full d'estil: selector → declaracions (les de dins d'una @media també hi són, amb el seu selector).
const cssRules = file => [...fs.readFileSync(path.join(__dirname, file), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '').matchAll(/([^{}]+)\{([^{}]*)\}/g)].map(m => ({ sel: m[1].trim(), body: m[2] }));
const prop = (rule, name) => rule?.body.match(new RegExp('(?:^|;)\\s*' + name + '\\s*:\\s*([^;]+)'))?.[1].trim();

test('a numbered list in a wind’s reply keeps room for the number: no later rule takes the step padding away', () => {
  const rules = cssRules('winds-workspace.css');
  const step = rules.findIndex(r => /ol:not\(\[class\]\) > li$/.test(r.sel) && /padding-left:36px/.test(r.body));
  assert.ok(step >= 0, 'the numbered steps leave 36 px for the circle');
  const later = rules.slice(step + 1).filter(r => /vw-message-text\s*>\s*(?:ol|:is\([^)]*\bol\b[^)]*\))\s*>\s*li\b/.test(r.sel) && prop(r, 'padding-left'));
  assert.deepEqual(later.map(r => r.sel), []);
});

test('the computer has one name: the sidebar row, the header menu entry and the pane it opens say the same', async () => {
  const f = await boot({ extra: { open: () => null }, api: r => r.url === '/api/ordinador/viewer' ? { ok: false, status: 404, text: async () => 'no_viewer' } : null });
  await byText(f.nav, '.vw-nav-channel', 'Gregal').click();
  await settle();
  const pc = byText(f.page.querySelector('.vw-more-list'), 'button', 'computer');
  const name = pc.textContent;
  assert.ok(f.nav.querySelectorAll('button').some(b => b.textContent === name), 'the sidebar row says «' + name + '»');
  await pc.click();
  await settle();
  const pane = f.doc.getElementById('workbench').querySelector('.vw-computer-pane');
  assert.ok(pane, 'the pane opens');
  assert.equal(pane.querySelector('h2').textContent, name);
  assert.equal(pane.getAttribute('aria-label'), name);
});

test('the sidebar computer row opens the pane over the start page, whose faces wrap instead of being cut', async () => {
  const f = await boot({ extra: { open: () => null }, api: r => r.url === '/api/ordinador/viewer' ? { ok: false, status: 404, text: async () => 'no_viewer' } : null });
  const row = f.nav.querySelectorAll('button').find(b => b.textContent === 'Gregal’s computer');
  assert.ok(row, 'the sidebar row');
  await row.click();
  await settle();
  assert.ok(f.doc.getElementById('workbench').querySelector('.vw-computer-pane'), 'the pane opens');
  assert.ok(f.page.querySelector('.vw-start-faces'), 'the start page stays');
  const wide = fs.readFileSync(path.join(__dirname, 'winds-workspace.css'), 'utf8').match(/@media\(min-width:761px\) \{([^@]*)\}/)?.[1] || '';
  assert.match(wide, /\.vw-start-faces \{[^}]*flex-wrap:wrap/, 'on wide screens every face stays visible');
});

test('Code and Winds greet with the same words at the same hour, from one helper', async () => {
  const { salutacio } = await import('./i18n.js');
  assert.deepEqual([2, 7, 15, 21].map(salutacio), ['Bona nit', 'Bon dia', 'Bona tarda', 'Bon vespre']);
  assert.match(fs.readFileSync(path.join(__dirname, 'i18n-boot.js'), 'utf8'), /window\.gregalGreeting = \(\) => salutacio\(new Date\(\)\.getHours\(\)\)/);
  const f = await boot({ extra: { gregalGreeting: () => 'Bona tarda' } });
  assert.equal(f.page.querySelector('.vw-welcome').querySelector('h1').textContent, 'Bona tarda');
  // Codi: home.js amb un DOM mínim.
  const doc = { addEventListener() {} }, ids = {};
  for (const id of ['heroTitle', 'hero', 'homeContext', 'heroMore', 'chips']) { ids[id] = new Element('div', doc); ids[id].style.display = ''; }
  doc.getElementById = id => ids[id] || null;
  const window = { gregalGreeting: () => 'Bona tarda', gregalT: k => k, gregal: { api: async () => ({ ok: false }) }, gregalPanels: {} };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'home.js'), 'utf8'), { window, document: doc, setTimeout: (fn, ms) => { const h = setTimeout(fn, ms); h.unref(); return h; }, setInterval: () => 0, MutationObserver: class { observe() {} } });
  await settle();
  assert.equal(ids.heroTitle.textContent, 'Bona tarda');
});

test('Saved has one title: «Saved» once, as the heading above its tabs', async () => {
  const f = await boot();
  const ctx = { document: f.doc, api: f.window.api, URLSearchParams, setTimeout, clearTimeout, console };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'winds-notes.js'), 'utf8'), ctx);
  f.window.gregalNotes = ctx.gregalNotes;
  await byText(f.nav, 'button', 'Saved').click();
  await settle();
  const all = inOrder(f.page.querySelector('.vw-content'));
  const saying = all.filter(n => ['H1', 'H2', 'BUTTON'].includes(n.tagName) && n.textContent.trim() === 'Saved');
  assert.deepEqual(saying.map(n => n.tagName), ['H1'], 'one title, not a tab, an H1 and an H2');
  assert.ok(!all.some(n => n.tagName === 'H2' && /Your saved items/.test(n.textContent)));
  assert.ok(all.indexOf(saying[0]) < all.indexOf(f.page.querySelector('.vw-page-tabs')), 'the tabs sit under the title');
});

test('Work keeps its title on every tab, with the tabs under it and one text size for tabs, sections and their text', async () => {
  const f = await boot();
  await byText(f.nav, 'button', 'Work').click();
  await settle();
  for (const tab of ['In progress', 'Do', 'Activity']) {
    if (tab !== 'In progress') { await byText(f.page.querySelector('.vw-page-tabs'), 'button', tab).click(); await settle(); }
    const all = inOrder(f.page.querySelector('.vw-content'));
    const h1 = all.find(n => n.tagName === 'H1');
    assert.equal(h1?.textContent, 'Work', tab);
    assert.ok(all.indexOf(h1) < all.indexOf(f.page.querySelector('.vw-page-tabs')), tab + ': tabs under the title');
  }
  const rules = cssRules('winds-workspace.css');
  const size = sel => prop(rules.findLast(r => r.sel === sel && prop(r, 'font-size')), 'font-size');
  const sizes = ['.vw-page-tab', '.vw-feina-h', '.vw-feina-sec .vw-muted', '.vw-heading p'].map(size);
  assert.equal(new Set(sizes).size, 1, 'one size, got ' + sizes.join(', '));
});

test('the primary button wears the send button’s colours in every theme, and «Tune tomorrow» is underlined body text in both', () => {
  const rules = cssRules('winds-workspace.css');
  const send = rules.find(r => r.sel === '.vw-send'), primary = rules.find(r => r.sel === ':is(button,a).vw-primary');
  assert.equal(prop(primary, 'background'), prop(send, 'background'));
  assert.equal(prop(primary, 'color'), prop(send, 'color'));
  const tune = rules.find(r => r.sel === '.vw-avui-tune');
  assert.equal(prop(tune, 'color'), 'var(--text)', 'the same role in light and dark, not the accent that is a link in one and ink in the other');
  assert.match(prop(tune, 'text-decoration') || '', /underline/);
});

test('All winds leads with your winds; tidying up is a small chip under the title', async () => {
  const mine = { id: 'marinada', name: 'Marinada', color: '#C8A27C', role: 'Cerques', style: 'pso' };
  const f = await boot({ api: r => r.url === '/api/winds' && r.method === 'GET' ? { ok: true, status: 200, json: async () => ({ winds: [...winds, mine] }) }
    : r.url === '/api/winds/tidy' ? { ok: true, status: 200, json: async () => ({ total: 3, groups: [], test: [], unused: [{ id: 'xaloc', name: 'Xaloc' }] }) } : null });
  await byText(f.nav, 'button', 'All winds').click();
  await settle();
  const all = inOrder(f.page.querySelector('.vw-content'));
  const first = all.find(n => n.matches('.vw-member'));
  assert.match(first.textContent, /Marinada/, 'your own winds first');
  assert.ok(all.indexOf(first) < all.indexOf(f.page.querySelector('.vw-tabs')), 'before the squads');
  assert.equal(f.page.querySelector('.vw-tidy-banner'), null, 'no banner');
  const chip = f.page.querySelector('.vw-tidy-chip');
  assert.ok(chip && chip.tagName === 'BUTTON', 'a chip');
  assert.match(chip.textContent, /1 unused · Tidy up/);
  assert.ok(chip.closest('.vw-heading'), 'under the title');
});

test('a turn that uses the computer leaves it closed when the person closed the side pane in that chat, with a dot', async () => {
  const f = await paneChat();
  await f.turn(['activity', { phase: 'tool', tool: 'browser' }]);
  await f.computer().querySelector('.vw-close').click();
  assert.equal(f.computer(), null);
  await f.turn(['activity', { phase: 'tool', tool: 'browser' }]);
  assert.equal(f.computer(), null, 'it does not open on its own');
  assert.equal(f.toggle().dataset.fresh, 'true');
  await f.toggle().click();
  assert.ok(f.computer(), 'the toggle opens it');
});

test('switching chats closes the side pane without counting it as the person closing it', async () => {
  const f = await paneChat();
  f.doc.addEventListener('gregal:workpane', e => { if (e.detail !== 'docs') f.docs.hidden = true; });
  await f.turn(letter);
  assert.equal(f.docs.hidden, false);
  await byText(f.nav, '.vw-nav-channel', 'Gregal').click();
  await settle();
  assert.equal(f.docs.hidden, true, 'Xaloc’s document does not stay open over Gregal’s chat');
  assert.ok(!f.toggle() || f.toggle().hidden || f.toggle().getAttribute('aria-pressed') === 'false');
  // Tornar a Xaloc: el seu fil actual és la conversa d'abans.
  const api = f.window.api;
  f.window.api = async (url, o) => url === '/api/winds/chat?wind=xaloc' ? { ok: true, status: 200, json: async () => ({ session: 'company-xaloc-s2', history: [] }) } : api(url, o);
  await byText(f.nav, '.vw-nav-channel', 'Xaloc').click();
  await settle();
  assert.equal(f.toggle().getAttribute('aria-pressed'), 'false');
  await f.turn(letter);
  assert.equal(f.docs.hidden, false, 'a new version opens it again: the person never closed it');
});

// 144: la resposta creix en un sol node, que en arribar es converteix en el missatge; el fil no es refà.
// El torn arriba un esdeveniment per lectura; `each(i)` mira la pàgina just abans de la lectura i.
function slowStream(frames, each) {
  const bytes = frames.map(([k, d]) => new TextEncoder().encode('event: ' + k + '\ndata: ' + JSON.stringify(d) + '\n\n'));
  let i = 0;
  return { ok: true, status: 200, body: { getReader: () => ({ read: async () => { each(i); return i < bytes.length ? { done: false, value: bytes[i++] } : { done: true }; }, cancel: async () => {}, releaseLock() {} }) } };
}
const fifty = Array.from({ length: 50 }, (_, i) => 'mot' + i + ' ');
const replyRows = f => f.page.querySelector('.vw-messages').querySelectorAll('article').filter(a => !a.classList.contains('vw-user'));

test('a streamed reply grows in one node that becomes the final message: the face and dots stay, the thread is not rebuilt', async () => {
  const group = { id: 'r1', name: 'La colla del concert', members: ['xaloc', 'gregal'] };
  const json = data => ({ ok: true, status: 200, json: async () => data });
  const saved = [{ from: 'user', text: 'Anem-hi' }, { from: 'xaloc', speaker_id: 'xaloc', text: fifty.join('').trim() }];
  const frames = [['phase', { phase: 'speaker:xaloc' }], ...fifty.map(token => ['token', { token }]), ['reply', { wind: 'xaloc', history: saved }], ['done', { history: saved }]];
  const seen = []; let thread;
  const f = await boot({ api: r => r.url === '/api/winds/rotllanes' ? json({ rotllanes: [group] })
    : r.url.startsWith('/api/winds/rotllana?') ? json({ history: [] })
    : r.url === '/api/winds/rotllana/stream' ? slowStream(frames, i => { const n = f.page.querySelector('.vw-pending'); if (!i) thread = f.page.querySelector('.vw-messages'); seen[i] = { n, face: n?.firstChild, dots: n?.querySelector('.vw-thinking-dots'), text: n?.textContent || '' }; }) : null });
  await byText(f.nav, 'button', 'La colla del concert').click();
  await settle();
  // The thread as it is once the turn has started (sending your message draws it).
  f.page.querySelector('textarea').value = 'Anem-hi';
  await f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  // seen[k + 1] is the page once token k has arrived.
  const live = seen[2].n;
  assert.ok(live, 'the reply is on the page while it streams');
  for (let k = 1; k <= 50; k++) {
    assert.ok(seen[k + 1].n === live, 'token ' + k + ': the same node');
    assert.ok(seen[k + 1].face === seen[2].face, 'token ' + k + ': the same face');
    assert.ok(seen[k + 1].dots === seen[2].dots, 'token ' + k + ': the same dots');
    assert.match(seen[k + 1].text, new RegExp('mot' + (k - 1) + '\\b'), 'token ' + k + ' is shown');
  }
  assert.ok(replyRows(f).at(-1) === live, 'the reply that landed is the node that grew');
  assert.ok(!live.classList.contains('vw-pending'));
  assert.match(live.textContent, /mot49/);
  assert.ok(f.page.querySelector('.vw-messages') === thread, 'the thread is the same, not repainted from scratch');
  assert.equal(f.page.querySelectorAll('.vw-pending').length, 0);
});

test('a wind’s reply that lands with the turn (no reply event) is the node that streamed too', async () => {
  const saved = [{ from: 'user', text: 'Hola' }, { from: 'assistant', text: fifty.join('').trim() }];
  const frames = [...fifty.map(token => ['token', { token }]), ['done', { history: saved }]];
  const seen = []; let thread;
  const f = await boot({ api: r => r.url === '/api/winds/chat/stream' ? slowStream(frames, i => { if (!i) thread = f.page.querySelector('.vw-messages'); seen[i] = f.page.querySelector('.vw-pending'); }) : null });
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  // The thread as it is once the turn has started.
  f.page.querySelector('textarea').value = 'Hola';
  await f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  assert.ok(seen[1]);
  assert.ok(seen.slice(1, 51).every(n => n === seen[1]), 'one node while it streams');
  assert.ok(replyRows(f).at(-1) === seen[1], 'and that node is the final message');
  assert.ok(f.page.querySelector('.vw-messages') === thread, 'the thread is the same');
  assert.equal(f.page.querySelector('.vw-wind-mark').textContent, 'Done', 'the header still says the turn is done');
});

test('while a reply streams the thread stays pinned to the end, and does not pull you down once you scroll up', async () => {
  // Like a browser: the text makes the thread taller (two pixels per character).
  const height = Object.getOwnPropertyDescriptor(Element.prototype, 'scrollHeight');
  Object.defineProperty(Element.prototype, 'scrollHeight', { configurable: true, get() { return height.get.call(this) + (this.classList.contains('vw-messages') ? 2 * this.textContent.length : 0); } });
  try {
    for (const scrollUpAt of [null, 20]) {
      const saved = [{ from: 'user', text: 'Hola' }, { from: 'assistant', text: fifty.join('').trim() }];
      const frames = [...fifty.map(token => ['token', { token }]), ['done', { history: saved }]];
      const tops = [];
      const f = await boot({ api: r => r.url === '/api/winds/chat/stream' ? slowStream(frames, i => {
        const box = f.page.querySelector('.vw-messages');
        if (i === scrollUpAt) box.scrollTop = 0;
        tops[i] = [box.scrollTop, box.scrollHeight - box.clientHeight];
      }) : null });
      await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
      await settle();
      f.page.querySelector('textarea').value = 'Hola';
      await f.page.querySelector('.vw-composer').requestSubmit();
      await settle();
      const box = f.page.querySelector('.vw-messages'), end = box.scrollHeight - box.clientHeight;
      if (scrollUpAt === null) {
        for (let i = 2; i <= 50; i++) assert.equal(tops[i][0], tops[i][1], 'read ' + i + ': at the end');
        assert.ok(tops[50][1] > tops[10][1], 'the thread grew while it streamed');
        assert.equal(box.scrollTop, end, 'still at the end when the reply lands');
      } else {
        for (let i = scrollUpAt + 1; i <= 50; i++) assert.equal(tops[i][0], 0, 'read ' + i + ': left where the person scrolled');
        assert.equal(box.scrollTop, 0, 'the landed reply does not pull them down');
      }
    }
  } finally { Object.defineProperty(Element.prototype, 'scrollHeight', height); }
});

test('a Today card action with a thread opens that conversation instead of sending a prompt', async () => {
  const card = { id: 'l1', wind: 'xaloc', title: 'Ja hi ha entrades', source: 'loop', section: 'fyi', actions: [{ label: 'Open the conversation', thread: 'company-xaloc-s2' }] };
  const json = data => ({ ok: true, status: 200, json: async () => data });
  const f = await boot({ api: r => r.url === '/api/winds/avui' ? json({ date: '2026-11-24', fyi: [card] }) : null });
  await byText(f.nav, 'button', 'Today').click();
  await settle();
  await byText(f.page.querySelector('.vw-avui-card'), 'button', 'Open the conversation').click();
  await settle();
  assert.deepEqual(f.requests.find(r => r.url === '/api/winds/session/open')?.body, { wind: 'xaloc', name: 'company-xaloc-s2' });
  assert.ok(!f.requests.some(r => r.url === '/api/winds/sessions' && r.method === 'POST'), 'no new conversation');
  assert.ok(!f.requests.some(r => r.url === '/api/winds/chat/stream'), 'nothing is sent');
  assert.match(f.page.querySelector('.vw-messages').textContent, /Hi ha jazz al Born/);
});

// 128: un sol capçal per a tots els vents. La forma del capçal (classes, sense textos) i el menú «⋯».
const shape = n => n.getAttribute('role') === 'menu' ? [n.className] : [n.className, ...n.children.filter(c => c.tagName !== 'TEXT').map(shape)];
const menuOf = f => f.page.querySelector('.vw-top').querySelector('.vw-more-list').children.map(b => b.textContent);
async function headerOf(f, open) {
  await open();
  await settle();
  const top = f.page.querySelector('.vw-top');
  return { top, shape: shape(top), menu: menuOf(f) };
}
test('every wind, Gregal included, has the same header: its face, name, a role chip in its colour and one ⋯ menu', async () => {
  const f = await boot();
  const gregal = await headerOf(f, () => byText(f.nav, '.vw-nav-channel', 'Gregal').click());
  const xaloc = await headerOf(f, () => byText(f.nav, '.vw-nav-channel', 'Xaloc').click());
  assert.deepEqual(gregal.shape, xaloc.shape, 'the same structure');
  assert.deepEqual(xaloc.menu, ['Profile', 'Threads', 'Save to wiki', 'Gregal’s computer']);
  assert.deepEqual(gregal.menu.slice(0, 4), xaloc.menu, 'Gregal’s menu has the same entries, then its guide');
  const who = xaloc.top.querySelector('.vw-owner-who');
  assert.ok(who.querySelector('.vw-wind-corner'), 'the face with its mood is in the header');
  assert.equal(who.querySelector('strong').textContent, 'Xaloc');
  assert.equal(xaloc.top.querySelector('.vw-tag').style['--c'], '#F5B45D', 'the role chip takes the wind’s colour');
  assert.equal(xaloc.top.querySelectorAll('button').filter(b => /^(Threads|Save to wiki)$/.test(b.textContent) && !b.closest('.vw-more-list')).length, 0, 'no loose actions next to the menu');
});

test('tapping the face opens the wind’s profile; the ⋯ menu opens, runs an entry and closes', async () => {
  const json = data => ({ ok: true, status: 200, json: async () => data });
  const f = await boot({ api: r => r.url.startsWith('/api/winds/profile?') ? json({}) : null });
  await byText(f.nav, '.vw-nav-channel', 'Gregal').click();
  await settle();
  await f.page.querySelector('.vw-owner-who').click();
  await settle();
  assert.ok(f.requests.some(r => r.url === '/api/winds/profile?wind=gregal'), 'the profile opens');
  const toggle = f.page.querySelector('.vw-more-btn'), list = f.page.querySelector('.vw-more-list');
  assert.equal(toggle.getAttribute('aria-haspopup'), 'menu');
  assert.ok(list.hidden);
  await toggle.click();
  assert.ok(!list.hidden);
  assert.equal(toggle.getAttribute('aria-expanded'), 'true');
  f.doc.dispatchEvent({ type: 'keydown', key: 'Escape', stopPropagation() {} });
  assert.ok(list.hidden, 'Escape closes it');
  await toggle.click();
  f.requests.length = 0;
  await byText(list, 'button', 'Profile').click();
  await settle();
  assert.ok(list.hidden, 'choosing an entry closes it');
  assert.ok(f.requests.some(r => r.url === '/api/winds/profile?wind=gregal'));
});

test('the header and its menu speak Catalan too', async () => {
  const f = await boot({ lang: 'ca' });
  const x = await headerOf(f, () => byText(f.nav, '.vw-nav-channel', 'Xaloc').click());
  assert.deepEqual(x.menu, ['Perfil', 'Fils', 'Desa a la wiki', 'L’ordinador de Gregal']);
  assert.equal(x.top.querySelector('.vw-more-btn').getAttribute('aria-label'), 'Més opcions');
});

// 128: el vent reacciona amb el cos: assenteix quan envies, s'hi acosta amb un esclat del seu color quan
// arriba la resposta i fa que no quan falla.
test('the wind nods when you send, leans in with a burst in its colour when the reply lands, once', async () => {
  const saved = [{ from: 'user', text: 'Hola' }, { from: 'assistant', text: 'Fet' }];
  let sent;
  const f = await boot({ api: r => r.url === '/api/winds/chat/stream' ? slowStream([['done', { history: saved }]], i => { if (!i) sent = f.page.querySelector('.vw-wind-corner')?.dataset.react; }) : null });
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  f.page.querySelector('textarea').value = 'Hola';
  await f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  assert.equal(sent, 'nod', 'a nod as the message goes');
  const corner = f.page.querySelector('.vw-wind-corner');
  assert.equal(corner.dataset.react, 'lean', 'it leans in when the reply arrives');
  assert.equal(corner.querySelectorAll('.vw-burst').length, 1, 'with one burst');
  await f.window.loadWindsPage();
  await settle();
  assert.equal(f.page.querySelector('.vw-wind-corner').dataset.react, undefined, 'a redraw does not react again');
  assert.equal(f.page.querySelectorAll('.vw-burst').length, 0);
});

test('reactions are short transform animations that only play with motion allowed', () => {
  const css = fs.readFileSync(path.join(__dirname, 'winds-workspace.css'), 'utf8');
  const block = at => { let depth = 0; const start = css.indexOf('{', at); for (let i = start; i < css.length; i++) { if (css[i] === '{') depth++; else if (css[i] === '}' && --depth === 0) return css.slice(start + 1, i); } return ''; };
  const blocks = [...css.matchAll(/@media ?\(prefers-reduced-motion: ?no-preference\)/g)].map(m => block(m.index)), motion = blocks.join('\n');
  const still = blocks.reduce((rest, b) => rest.replace(b, ''), css);
  for (const [react, name] of [['nod', 'vw-corner-nod'], ['lean', 'vw-corner-lean'], ['shake', 'vw-corner-shake']]) {
    assert.match(motion, new RegExp('\\.vw-wind-corner\\[data-react="' + react + '"\\] > \\.winds-face \\{ animation:' + name + ' '), react + ' plays with motion allowed');
    assert.doesNotMatch(still, new RegExp('animation:' + name + ' '), react + ' never plays under reduced motion');
    const props = [...block(css.indexOf('@keyframes ' + name + ' ')).matchAll(/([a-z-]+)\s*:/g)].map(m => m[1]).filter(p => p !== 'transform');
    assert.deepEqual(props, [], name + ' moves with transform only');
  }
});

const failing = r => r.url === '/api/winds/chat/stream' ? { ok: false, status: 502, text: async () => 'boom', json: async () => ({}) } : null;
test('a failed turn shakes the wind and shows in the thread as its bubble with a retry, not as red text under the composer', async () => {
  let fail = true;
  const f = await boot({ api: r => fail ? failing(r) : null });
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  f.page.querySelector('textarea').value = 'Book the jam';
  await f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  assert.equal(f.page.querySelector('.vw-wind-corner').dataset.react, 'shake');
  const status = f.page.querySelector('.vw-status');
  assert.equal(status.textContent, '', 'nothing under the composer');
  assert.notEqual(status.getAttribute('role'), 'alert');
  const box = f.page.querySelector('.vw-messages'), row = box.querySelector('.vw-failed');
  assert.ok(row, 'the failure is in the thread');
  assert.equal(row.querySelector('strong').textContent, 'Xaloc', 'spoken by the wind');
  assert.match(row.textContent, /I couldn’t get to it/);
  assert.equal(row.getAttribute('role'), 'alert');
  assert.ok(box.querySelectorAll('.vw-user').some(n => n.textContent.includes('Book the jam')), 'your message stays in the thread');
  assert.equal(f.page.querySelector('textarea').value, '', 'and is not put back in the composer');
  fail = false;
  await byText(row, 'button', 'Try again').click();
  await settle();
  assert.deepEqual(f.requests.filter(r => r.url === '/api/winds/chat/stream').map(r => r.body.message), ['Book the jam', 'Book the jam']);
  assert.equal(f.page.querySelector('.vw-failed'), null, 'gone once it works');
  assert.equal(f.page.querySelector('.vw-messages').querySelectorAll('.vw-user').filter(n => n.textContent.includes('Book the jam')).length, 1, 'your message once');
});

test('a group turn that fails after a voice already answered keeps your message once and offers no resend', async () => {
  const group = { id: 'r1', name: 'La colla del concert', members: ['xaloc', 'gregal'] };
  const json = data => ({ ok: true, status: 200, json: async () => data });
  const saved = [{ from: 'user', text: 'Anem-hi' }, { from: 'xaloc', speaker_id: 'xaloc', text: 'Jo hi vaig' }];
  const f = await boot({ api: r => r.url === '/api/winds/rotllanes' ? json({ rotllanes: [group] })
    : r.url.startsWith('/api/winds/rotllana?') ? json({ history: [] })
    : r.url === '/api/winds/rotllana/stream' ? slowStream([['phase', { phase: 'speaker:xaloc' }], ['reply', { wind: 'xaloc', history: saved }], ['phase', { phase: 'speaker:gregal' }], ['error', { error: 'boom' }]], () => {}) : null });
  await byText(f.nav, 'button', 'La colla del concert').click();
  await settle();
  f.page.querySelector('textarea').value = 'Anem-hi';
  await f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  const box = f.page.querySelector('.vw-messages');
  assert.equal(box.querySelectorAll('.vw-user').length, 1, 'your message once: the server already has it');
  assert.match(box.textContent, /Jo hi vaig/);
  const row = box.querySelector('.vw-failed');
  assert.equal(row.querySelector('strong').textContent, 'Gregal', 'the voice that could not answer says so');
  assert.ok(!byText(row, 'button', 'Try again'), 'resending would ask the whole group again');
});

test('the failed turn bubble speaks Catalan too', async () => {
  const f = await boot({ lang: 'ca', api: failing });
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  f.page.querySelector('textarea').value = 'Reserva-ho';
  await f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  const row = f.page.querySelector('.vw-failed');
  assert.match(row.textContent, /No he pogut arribar-hi/);
  assert.ok(byText(row, 'button', 'Torna-ho a provar'));
});

// 136: Feina. Una feina en marxa és una targeta amb la cara de qui la porta que s'obre al lloc;
// el que han fet els vents és dins Fet, i Desat ja no ho repeteix.
const running = { id: 't1', task: 'Build the club site', status: 'working', agents: { builder: 'Xaloc' }, workspace: 'C:/ws/club',
  events: [{ type: 'tool', agent: 'builder', activity: 'write', message: '{"file_path":"C:/ws/club/index.html"}' }, { type: 'tool_result', agent: 'builder', path: 'C:/ws/club/index.html' }] };
const withTasks = list => r => r.url === '/api/winds/tasks' ? { ok: true, status: 200, json: async () => ({ tasks: list }) } : null;

test('a running task in Work is a card with its owner’s face that opens in place to its steps and links', async () => {
  const f = await boot({ api: withTasks([running]) });
  await byText(f.nav, 'button', 'Work').click();
  await settle();
  const card = f.page.querySelector('.vw-feina-now').querySelector('.vw-feina-card');
  assert.ok(card, 'the running task is a card under Now');
  const head = card.querySelector('.vw-feina-card-head'), body = card.querySelector('.vw-feina-card-body');
  assert.equal(head.querySelector('svg').dataset.wind, 'Xaloc', 'the owner’s face');
  assert.match(head.textContent, /Build the club site/);
  assert.equal(head.getAttribute('aria-expanded'), 'false');
  assert.equal(body.inert, true, 'closed, its links are out of reach');
  await head.click();
  assert.equal(head.getAttribute('aria-expanded'), 'true');
  assert.ok(card.classList.contains('open'));
  assert.equal(body.inert, false);
  assert.match(body.querySelector('.vw-feina-steps').textContent, /Writing files/, 'its steps');
  assert.ok(byText(body, 'button', 'index.html'), 'a link to the file it made');
  assert.equal(f.page.querySelector('h1').textContent, 'Work', 'it opened in place');
  await head.click();
  assert.equal(head.getAttribute('aria-expanded'), 'false', 'and closes again');
  await byText(body, 'button', 'Open the task').click();
  await settle();
  assert.equal(f.page.querySelector('h1').textContent, 'Build the club site', 'the link opens the whole task');
});

test('the task card speaks Catalan too', async () => {
  const f = await boot({ lang: 'ca', api: withTasks([running]) });
  await byText(f.nav, 'button', 'Feina').click();
  await settle();
  const card = f.page.querySelector('.vw-feina-card');
  await card.querySelector('.vw-feina-card-head').click();
  assert.match(card.textContent, /Escrivint fitxers/);
  assert.ok(byText(card, 'button', 'Obre la feina'));
});

test('a task card opens with a short transition on its height that only plays with motion allowed', () => {
  const css = fs.readFileSync(path.join(__dirname, 'winds-workspace.css'), 'utf8');
  const block = at => { let depth = 0; const start = css.indexOf('{', at); for (let i = start; i < css.length; i++) { if (css[i] === '{') depth++; else if (css[i] === '}' && --depth === 0) return css.slice(start + 1, i); } return ''; };
  const blocks = [...css.matchAll(/@media ?\(prefers-reduced-motion: ?no-preference\)/g)].map(m => block(m.index)), motion = blocks.join('\n');
  const still = blocks.reduce((rest, b) => rest.replace(b, ''), css);
  assert.match(motion, /\.vw-feina-card-body \{ transition:grid-template-rows \.\d+s/);
  assert.doesNotMatch(still, /\.vw-feina-card-body \{[^}]*transition/);
  const rules = cssRules('winds-workspace.css');
  const rows = sel => prop(rules.findLast(r => r.sel === sel && prop(r, 'grid-template-rows')), 'grid-template-rows');
  assert.equal(rows('.vw-feina-card-body'), '0fr', 'closed');
  assert.equal(rows('.vw-feina-card.open .vw-feina-card-body'), '1fr', 'open');
});

test('what the winds made lives in Work › Done; Saved no longer repeats it', async () => {
  const galleries = [];
  const activity = { renderSaved: async box => { galleries.push(box); const p = box.doc.createElement('p'); p.textContent = 'Club rules.md'; box.append(p); }, renderAutomations: async () => {} };
  const f = await boot({ extra: { gregalVentsActivity: activity } });
  const ctx = { document: f.doc, api: f.window.api, URLSearchParams, setTimeout, clearTimeout, console };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'winds-notes.js'), 'utf8'), ctx);
  f.window.gregalNotes = ctx.gregalNotes;
  await byText(f.nav, 'button', 'Saved').click();
  await settle();
  assert.doesNotMatch(f.page.querySelector('.vw-content').textContent, /What the winds made/);
  assert.equal(galleries.length, 0, 'Saved does not draw the gallery');
  await byText(f.nav, 'button', 'Work').click();
  await settle();
  assert.equal(galleries.length, 1, 'Work draws it once');
  assert.ok(galleries[0].closest('.vw-feina-done'), 'inside Done');
  assert.match(f.page.querySelector('.vw-feina-done').textContent, /Club rules\.md/);
});

// 132: la fila de cares de l'inici, viva: tots els vents, a lliscar amb imant al mòbil, esvaïda només
// on queden cares amagades, cada cara respirant al seu ritme i un punt al vent que treballa.
const crowd = [...winds, ...['Mestral', 'Garbí', 'Llebeig', 'Ponent', 'Tramuntana', 'Migjorn'].map((name, i) => ({ id: name.toLowerCase(), name, color: '#7BBF6A', style: 'pso', squad: i % 2 ? 'daily' : 'data' }))];
const crowdApi = r => r.url === '/api/winds' ? { ok: true, status: 200, json: async () => ({ winds: crowd }) } : null;
const css = () => fs.readFileSync(path.join(__dirname, 'winds-workspace.css'), 'utf8');
const mediaList = (text, query) => [...text.matchAll(new RegExp('@media\\(' + query.replace(/[()]/g, '\\$&') + '\\) ?\\{', 'g'))].map(m => { let depth = 0; for (let i = m.index + m[0].length - 1; i < text.length; i++) { if (text[i] === '{') depth++; else if (text[i] === '}' && --depth === 0) return text.slice(m.index + m[0].length, i); } return ''; });
const mediaBlocks = (text, query) => mediaList(text, query).join('\n');

test('every wind is in the start row, Gregal first, each breathing out of step with the others', async () => {
  const f = await boot({ api: crowdApi });
  const faces = f.page.querySelector('.vw-start-faces').querySelectorAll('.vw-start-face');
  assert.deepEqual(faces.map(b => b.querySelector('span').textContent), ['Gregal', ...crowd.filter(w => w.id !== 'gregal').map(w => w.name)]);
  const phases = faces.map(b => b.style['--vw-breath']);
  assert.ok(phases.every(Boolean), 'every face has a breath phase');
  assert.equal(new Set(phases).size, phases.length, 'no two faces breathe in step');
  const moving = mediaBlocks(css(), 'prefers-reduced-motion:no-preference');
  assert.match(moving, /\.vw-start-face > :first-child \{[^}]*animation:vw-face-breathe [^;]*var\(--vw-breath/, 'the breath plays only with motion allowed');
  assert.doesNotMatch(mediaList(css(), 'prefers-reduced-motion:no-preference').reduce((rest, b) => rest.replace(b, ''), css()), /animation:vw-face-breathe/, 'no breath under reduced motion');
});

test('on a phone the start row snaps face by face and fades only on the side where faces are hidden', async () => {
  const f = await boot({ api: crowdApi });
  const row = f.page.querySelector('.vw-start-faces');
  const at = left => { Object.assign(row, { scrollLeft: left, clientWidth: 350, scrollWidth: 700 }); for (const g of row.listeners.scroll || []) g({ target: row }); return row.dataset.fade; };
  assert.equal(at(0), 'end', 'at the start, only the right edge fades');
  assert.equal(at(120), 'start end', 'in the middle, both');
  assert.equal(at(350), 'start', 'at the end, only the left edge');
  Object.assign(row, { scrollLeft: 0, clientWidth: 700, scrollWidth: 700 });
  for (const g of row.listeners.scroll || []) g({ target: row });
  assert.equal(row.dataset.fade, '', 'every face in view: no fade');
  const phone = mediaBlocks(css(), 'max-width:760px');
  assert.match(phone, /\.vw-start-faces \{[^}]*scroll-snap-type:x mandatory/);
  assert.match(phone, /\.vw-start-face \{[^}]*scroll-snap-align:start/);
  for (const side of ['start', 'end', 'start end']) assert.match(phone, new RegExp('\\.vw-start-faces\\[data-fade="' + side + '"\\] \\{[^}]*mask-image:linear-gradient'), side);
});

test('a wind answering right now has a dot on its start face, in its colour and named for screen readers; it goes when the turn ends', async () => {
  let finish;
  const f = await boot({ api: r => r.url === '/api/winds/chat/stream' ? new Promise(res => { finish = () => res(stream([{ from: 'user', text: r.body.message }, { from: 'assistant', text: 'Fet' }])); }) : null });
  f.page.querySelector('textarea').value = 'Primera';
  const first = f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  await byText(f.nav, 'button', 'New conversation').click();
  await settle();
  const faceOf = name => byText(f.page.querySelector('.vw-start-faces'), 'button', name);
  const dot = faceOf('Gregal').querySelector('.vw-busy-dot');
  assert.ok(dot, 'Gregal is answering: its face has the dot');
  assert.equal(dot.style['--c'], '#8193FF');
  assert.equal(dot.textContent, 'Working…');
  assert.ok(!faceOf('Xaloc').querySelector('.vw-busy-dot'), 'a quiet wind has none');
  finish();
  await first;
  await settle();
  assert.ok(!faceOf('Gregal').querySelector('.vw-busy-dot'), 'the dot goes once the turn ends');
  assert.match(mediaBlocks(css(), 'prefers-reduced-motion:no-preference'), /\.vw-busy-dot \{[^}]*animation:/, 'the dot pulses only with motion allowed');
});

test('the busy dot speaks Catalan too', async () => {
  let finish;
  const f = await boot({ lang: 'ca', api: r => r.url === '/api/winds/chat/stream' ? new Promise(res => { finish = () => res(stream([{ from: 'assistant', text: 'Fet' }])); }) : null });
  f.page.querySelector('textarea').value = 'Primera';
  const first = f.page.querySelector('.vw-composer').requestSubmit();
  await settle();
  await byText(f.nav, 'button', 'Nova conversa').click();
  await settle();
  assert.equal(byText(f.page.querySelector('.vw-start-faces'), 'button', 'Gregal').querySelector('.vw-busy-dot').textContent, 'Treballant…');
  finish();
  await first;
});

test('a tapped start face flies into the chat with a view transition, and just opens without one', async () => {
  const flights = [];
  const f = await boot();
  f.doc.startViewTransition = paint => { const from = f.page.querySelectorAll('.vw-start-face').find(b => b.firstChild.style.viewTransitionName === 'vw-face'); const flight = { from: from?.textContent }; flights.push(flight); paint(); flight.to = f.page.querySelector('.vw-wind-corner')?.style.viewTransitionName; return { finished: Promise.resolve() }; };
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Xaloc').click();
  await settle();
  assert.equal(flights.length, 1);
  assert.match(flights[0].from, /Xaloc/, 'the tapped face leaves');
  assert.equal(flights[0].to, 'vw-face', 'and lands on the wind in the chat header');
  assert.equal(f.page.querySelector('.vw-wind-corner').style.viewTransitionName, '', 'the name is freed after the flight');

  await byText(f.nav, 'button', 'New conversation').click();
  await settle();
  f.window.matchMedia = q => ({ matches: /reduce/.test(q) });
  await byText(f.page.querySelector('.vw-start-faces'), 'button', 'Gregal').click();
  await settle();
  assert.equal(flights.length, 1, 'with reduced motion there is no flight');
  assert.match(f.page.querySelector('.vw-top').textContent, /Gregal/, 'the chat still opens');
});

// 132: El pati, a primera fila: a la barra lateral i a la de baix, amb el Racó a dins; l'inici té la seva pestanya.
test('El pati sits in the sidebar and the bottom bar, not under «Settings & more», and the Racó is folded into it', async () => {
  const views = [];
  const f = await boot({ extra: { gregal: { setView: v => views.push(v) } } });
  const more = f.nav.querySelector('details');
  assert.ok(byText(f.nav, '.vw-nav-item', 'Playground') && !byText(more, '.vw-nav-item', 'Playground'), 'the sidebar has El pati, outside the menu');
  assert.ok(!byText(f.nav, '.vw-nav-item', 'The winds’ corner'), 'the Racó has no row of its own');
  const bottom = f.page.querySelector('.vw-mobile-nav');
  assert.deepEqual(bottom.querySelectorAll('button').map(b => b.textContent), ['Home', 'Today', 'Work', 'Saved', 'Playground', 'Winds']);
  assert.equal(byText(bottom, 'button', 'Home').dataset.selected, 'true', 'the start page has its tab, highlighted');
  assert.equal(byText(bottom, 'button', 'Home').getAttribute('aria-current'), 'page');
  assert.match(mediaBlocks(css(), 'max-width:760px'), /\.vw-mobile-nav \{[^}]*grid-template-columns:repeat\(6,/, 'six tabs, one column each');

  await byText(bottom, 'button', 'Playground').click();
  await settle();
  assert.equal(byText(f.page.querySelector('.vw-mobile-nav'), 'button', 'Playground').dataset.selected, 'true');
  assert.equal(byText(f.nav, '.vw-nav-item', 'Playground').getAttribute('aria-current'), 'page');
  assert.ok(!f.nav.querySelector('details').open, 'the menu does not open for El pati');
  await byText(f.page.querySelector('.vw-content'), 'button', 'The winds’ corner').click();
  assert.deepEqual(views, ['raco'], 'the Racó opens from inside El pati');
});

test('El pati and its Racó speak Catalan too; with the Racó switched off there is no way in', async () => {
  const f = await boot({ lang: 'ca', extra: { gregal: { setView() {} }, gregalPlugins: { viewAllowed: v => v !== 'raco' } } });
  assert.deepEqual(f.page.querySelector('.vw-mobile-nav').querySelectorAll('button').map(b => b.textContent), ['Inici', 'Avui', 'Feina', 'Desat', 'El pati', 'Vents']);
  await byText(f.nav, '.vw-nav-item', 'El pati').click();
  await settle();
  assert.ok(!byText(f.page.querySelector('.vw-content'), 'button', 'El Racó dels vents'), 'no link to a switched-off Racó');
});

test('on the Racó page the sidebar marks El pati, the way into it, and nothing else', async () => {
  let view = 'vents';
  const f = await boot({ extra: { gregal: { setView: v => { view = v; }, viewActual: () => view } } });
  await byText(f.nav, '.vw-nav-item', 'Work').click(); await settle();
  view = 'raco'; f.mutate(f.doc.getElementById('racoPage'), 'hidden');
  assert.equal(byText(f.nav, '.vw-nav-item', 'Playground').getAttribute('aria-current'), 'page');
  assert.equal(byText(f.nav, '.vw-nav-item', 'Work').getAttribute('aria-current'), 'false');
  view = 'vents'; f.mutate(f.doc.getElementById('racoPage'), 'hidden');
  assert.equal(byText(f.nav, '.vw-nav-item', 'Work').getAttribute('aria-current'), 'page', 'back on Vents, its own view again');
});
