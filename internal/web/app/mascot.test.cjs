// Penell (internament «mascota»): apagat per defecte (també sense preferència desada), el
// commutador de Preferències només surt a l'escriptori per a la flotant, i
// l'estat s'envia al pont d'escriptori encara que la de la barra s'amagui.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

function storage(init = {}) {
  const data = new Map(Object.entries(init));
  return { getItem: k => (data.has(k) ? data.get(k) : null), setItem: (k, v) => data.set(k, String(v)), data };
}

function el() {
  const node = {
    hidden: false, dataset: {}, attrs: {}, children: [], listeners: {}, className: '', innerHTML: '', title: '',
    setAttribute(k, v) { this.attrs[k] = String(v); },
    appendChild(c) { this.children.push(c); return c; },
    insertBefore(c) { this.children.unshift(c); return c; },
    addEventListener(n, fn) { this.listeners[n] = fn; },
    contains: () => false,
    querySelector: () => null,
  };
  return node;
}

function loadMascot({ stored = {}, bridge = null } = {}) {
  const foot = el();
  const flags = { running: false };
  const ticks = [];
  const document = {
    readyState: 'complete',
    querySelector(sel) {
      if (sel === '#side .side-foot') return foot;
      if (sel === '#main .work-group.running') return flags.running ? {} : null;
      return null;
    },
    querySelectorAll: () => [],
    getElementById: () => null,
    createElement: () => el(),
    addEventListener() {},
    events: [],
    dispatchEvent(e) { this.events.push(e); },
  };
  const window = { gregalDesktop: bridge, gregalT: k => 'T:' + k };
  const sandbox = { window, document, localStorage: storage(stored), console, Date, setInterval: fn => { ticks.push(fn); return 1; }, CustomEvent: class { constructor(type) { this.type = type; } } };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'fun-core.js'), 'utf8'), sandbox);
  let src = fs.readFileSync(path.join(__dirname, 'mascot.js'), 'utf8');
  src = src.replace(/^import .*$/m, '').replace(/^export (const|function) /gm, '$1 ');
  src += '\nglobalThis.__m = { boot, mascotEnabled, SVG, hideMascot };';
  vm.runInNewContext(src, sandbox, { filename: 'mascot.js' });
  return { m: sandbox.__m, foot, flags, tick: () => ticks[0](), sandbox };
}

test('sidebar mascot is off by default and only on when the preference is "1"', () => {
  assert.equal(loadMascot().m.mascotEnabled(), false);
  assert.equal(loadMascot({ stored: { gregal_fun_mascot: '0' } }).m.mascotEnabled(), false);
  assert.equal(loadMascot({ stored: { gregal_fun_mascot: '1' } }).m.mascotEnabled(), true);
  const off = loadMascot();
  off.m.boot();
  assert.equal(off.foot.children[0].hidden, true);
  const on = loadMascot({ stored: { gregal_fun_mascot: '1' } });
  on.m.boot();
  assert.equal(on.foot.children[0].hidden, false);
});

test('"Hide" in the quick chat turns the sidebar weathervane preference off', () => {
  const s = loadMascot({ stored: { gregal_fun_mascot: '1' } });
  s.m.boot();
  assert.equal(s.foot.children[0].hidden, false);
  s.m.hideMascot();
  assert.equal(s.sandbox.localStorage.getItem('gregal_fun_mascot'), '0');
  assert.equal(s.m.mascotEnabled(), false);
  assert.equal(s.sandbox.document.events.at(-1).type, 'gregal:fun-prefs');
  s.tick();
  assert.equal(s.foot.children[0].hidden, true);
  const src = fs.readFileSync(path.join(__dirname, 'mascot.js'), 'utf8');
  assert.match(src, /mascot-pop-hide/);
  assert.match(src, /'contextmenu'/);
});

test('mood changes reach the desktop bridge even when the sidebar mascot is hidden', () => {
  const calls = [];
  const s = loadMascot({ bridge: { mascotState: (mood, label) => calls.push([mood, label]) } });
  s.m.boot();
  assert.deepEqual(calls, [['idle', 'T:mascot.idle']]);
  s.tick();
  assert.equal(calls.length, 1, 'same mood is not resent');
  s.flags.running = true;
  s.tick();
  assert.deepEqual(calls.at(-1), ['working', 'T:mascot.working']);
  s.flags.running = false;
  s.tick();
  assert.deepEqual(calls.at(-1)[0], 'done');
  // Al navegador (sense pont) no peta.
  const web = loadMascot();
  web.m.boot();
  web.tick();
});

function loadPrefs({ stored = {}, bridge } = {}) {
  let src = fs.readFileSync(path.join(__dirname, 'fun-prefs.js'), 'utf8');
  src = src.replace(/^import .*$/m, 'const T = k => k;').replace(/^export (const|function) /gm, '$1 ');
  src += '\nglobalThis.__p = funPrefs;';
  const events = [];
  const sandbox = {
    window: bridge ? { gregalDesktop: bridge } : {}, localStorage: storage(stored), String,
    document: { dispatchEvent: e => events.push(e) }, CustomEvent: class { constructor(type) { this.type = type; } },
  };
  vm.runInNewContext(src, sandbox, { filename: 'fun-prefs.js' });
  return { p: sandbox.__p, sandbox, events };
}

test('preferences: mascot unchecked by default; floating toggle only in desktop, off by default', () => {
  const web = loadPrefs();
  const html = web.p.section();
  assert.match(html, /id="prefsFunMascot">/);
  assert.doesNotMatch(html, /id="prefsFunMascot" checked/);
  assert.doesNotMatch(html, /prefsDesktopMascot/);
  assert.match(loadPrefs({ stored: { gregal_fun_mascot: '1' } }).p.section(), /id="prefsFunMascot" checked/);
  const desk = loadPrefs({ bridge: { mascotWindow: true } });
  const dhtml = desk.p.section();
  assert.match(dhtml, /id="prefsDesktopMascot">/);
  assert.match(dhtml, /fun\.desktopMascot/);
  assert.match(loadPrefs({ bridge: { mascotWindow: true }, stored: { gregal_desktop_mascot: '1' } }).p.section(), /id="prefsDesktopMascot" checked/);
  // Activar-lo desa '1' a localStorage (desktop-prefs.js ho reenvia al main).
  const box = { checked: true };
  desk.p.bind({ querySelector: sel => (sel === '#prefsDesktopMascot' ? box : null) });
  box.onchange();
  assert.equal(desk.sandbox.localStorage.getItem('gregal_desktop_mascot'), '1');
  assert.equal(desk.events.at(-1).type, 'gregal:fun-prefs');
});

// El penell: el dibuix de la barra lateral i el de la finestra flotant han de
// ser el mateix, i els angles per ànim (el rumb: 45° = NE) també.
const norm = s => s.replace(/\s+/g, ' ').replace(/> </g, '><').trim();
const desktopDir = path.join(__dirname, '..', '..', '..', 'desktop');

test('weathervane SVG is accessible and identical in the sidebar and the floating window', () => {
  const { SVG } = loadMascot().m;
  assert.match(SVG, /role="img"/);
  assert.match(SVG, /aria-label="Gregal"/);
  for (const cls of ['v-post', 'v-cross', 'v-arrow', 'v-head', 'v-tail', 'v-ring', 'v-streaks']) assert.match(SVG, new RegExp('class="' + cls + '"'), cls);
  for (const l of ['N', 'E', 'S', 'W']) assert.match(SVG, new RegExp('class="v-letter"[^>]*>' + l + '<'), l);
  const html = fs.readFileSync(path.join(desktopDir, 'mascot.html'), 'utf8');
  const inHtml = html.slice(html.indexOf('<svg'), html.indexOf('</svg>') + 6);
  assert.equal(norm(inHtml), norm(SVG));
});

function vaneAngles(css, prefix) {
  const out = {};
  for (const m of css.matchAll(/([^{}]+)\{\s*--vane:\s*([-\d.]+)deg;?\s*\}/g)) {
    for (const sel of m[1].split(',')) {
      const mood = sel.trim().match(/^(.*)\[data-mood="(\w+)"\]$/);
      if (mood && mood[1] === prefix) out[mood[2]] = Number(m[2]);
    }
  }
  return out;
}

test('weathervane mood angles: NE at rest and done, towards the composer when waiting; same on desktop', () => {
  const side = fs.readFileSync(path.join(__dirname, 'fun.css'), 'utf8');
  const desk = fs.readFileSync(path.join(desktopDir, 'mascot.css'), 'utf8');
  const a = vaneAngles(side, '.mascot-btn');
  assert.deepEqual(Object.keys(a).sort(), ['done', 'error', 'idle', 'waiting', 'working']);
  assert.equal(a.idle, 45);
  assert.equal(a.done, 45);
  assert.equal(a.waiting, 135);
  assert.ok(a.error > 180 && a.error < 270, 'error droops to an odd angle');
  assert.deepEqual(vaneAngles(desk, ''), a);
  // Tota animació del penell és dins de prefers-reduced-motion: no-preference.
  for (const css of [side, desk]) {
    const start = css.indexOf('/* --- penell') >= 0 ? css.indexOf('/* --- penell') : 0;
    const vane = css.slice(start, css.indexOf('@keyframes v-sway'));
    const block = vane.indexOf('@media (prefers-reduced-motion: no-preference)');
    assert.ok(block > 0);
    assert.doesNotMatch(vane.slice(0, block), /animation:/);
  }
});
