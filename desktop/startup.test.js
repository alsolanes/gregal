const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { FILE, FALLBACK, themeKey, schemeOf, loadStartupTheme, saveStartupTheme, pageQuery, pollDelay, waitFor, FAST_POLL_MS, SLOW_POLL_MS } = require('./startup');

const tmp = () => fs.mkdtempSync(path.join(os.tmpdir(), 'gregal-startup-'));

test('sense res desat, el fosc o el clar de la marca segons el tema o el sistema', () => {
  const dir = tmp();
  assert.deepEqual(loadStartupTheme(dir, { systemDark: true }), { ...FALLBACK.dark, scheme: 'dark' });
  assert.deepEqual(loadStartupTheme(dir, { tema: 'sistema', systemDark: false }), { ...FALLBACK.light, scheme: 'light' });
  // Un tema clar triat mana sobre el sistema fosc, i a l'inrevés.
  assert.equal(loadStartupTheme(dir, { tema: 'claude', systemDark: true }).scheme, 'light');
  assert.equal(loadStartupTheme(dir, { tema: 'grok', systemDark: false }).scheme, 'dark');
  // Un nom estrany és com «sistema».
  assert.equal(themeKey('../x', true), 'sistema-dark');
});

test('els colors desats valen només per al mateix tema (i, amb «sistema», el mateix mode)', () => {
  const dir = tmp();
  const calma = { bg: '#1B1A19', bar: '#151413', ink: '#F2EFE9', accent: '#E8A96B' };
  assert.ok(saveStartupTheme(dir, themeKey('calma', false), calma));
  assert.deepEqual(loadStartupTheme(dir, { tema: 'calma', systemDark: false }), { ...calma, scheme: 'dark' });
  assert.deepEqual(loadStartupTheme(dir, { tema: 'calma', systemDark: true }), { ...calma, scheme: 'dark' });
  // Canvies de tema: els desats ja no valen.
  assert.equal(loadStartupTheme(dir, { tema: 'light', systemDark: true }).bg, FALLBACK.light.bg);
  // «Sistema»: el sistema ha passat de clar a fosc entre arrencades.
  const clar = { bg: '#FFFFFF', bar: '#F9F9F9', ink: '#0D0D0D', accent: '#10A37F' };
  assert.ok(saveStartupTheme(dir, themeKey('sistema', false), clar));
  assert.equal(loadStartupTheme(dir, { tema: 'sistema', systemDark: false }).bg, '#FFFFFF');
  assert.equal(loadStartupTheme(dir, { systemDark: true }).bg, FALLBACK.dark.bg);
});

test('només es desen colors #rrggbb; un accent estrany es canvia pel de la marca', () => {
  const dir = tmp();
  assert.equal(saveStartupTheme(dir, 'dark', { bg: 'red', bar: '#101217', ink: '#F2F4F8', accent: '#8193FF' }), false);
  assert.equal(saveStartupTheme(dir, '', { bg: '#14161B', bar: '#101217', ink: '#F2F4F8' }), false);
  assert.equal(fs.existsSync(path.join(dir, FILE)), false);
  assert.ok(saveStartupTheme(dir, 'gpt', { bg: '#FFFFFF', bar: '#F9F9F9', ink: '#0D0D0D', accent: 'rgb(1,2,3)' }));
  assert.equal(loadStartupTheme(dir, { tema: 'gpt' }).accent, FALLBACK.light.accent);
  fs.writeFileSync(path.join(dir, FILE), '{{{');
  assert.equal(loadStartupTheme(dir, { tema: 'gpt' }).bg, FALLBACK.light.bg);
});

test('el text de les pàgines locals segueix la lluminositat del fons', () => {
  assert.equal(schemeOf('#14161B'), 'dark');
  assert.equal(schemeOf('#F0EEE6'), 'light');
  assert.equal(schemeOf('#000000'), 'dark');
  assert.equal(schemeOf('nope'), 'dark');
  const q = pageQuery({ ...FALLBACK.light, scheme: 'light' }, 'ca', 'Gregal Jocs');
  assert.deepEqual(q, { lang: 'ca', name: 'Gregal Jocs', scheme: 'light', ...FALLBACK.light });
  assert.equal(pageQuery({ ...FALLBACK.dark, scheme: 'dark' }, 'fr', '').lang, 'en');
});

test("s'espera el backend de pressa al principi i després més espaiat", async () => {
  assert.equal(pollDelay(0), FAST_POLL_MS);
  assert.equal(pollDelay(1999), FAST_POLL_MS);
  assert.equal(pollDelay(2000), SLOW_POLL_MS);
  assert.ok(FAST_POLL_MS <= 50);
  // Rellotge fals: el backend respon als 600 ms.
  let t = 0;
  const sleeps = [];
  const ok = await waitFor(async () => t >= 600, { now: () => t, sleep: async ms => { sleeps.push(ms); t += ms; } });
  assert.equal(ok, true);
  assert.equal(t, 600);
  assert.ok(sleeps.every(ms => ms === FAST_POLL_MS));
});

test("deixa d'esperar si el backend mor o passa el límit, i avisa quan triga", async () => {
  let t = 0;
  const clock = { now: () => t, sleep: async ms => { t += ms; } };
  let dead = false;
  const died = waitFor(async () => { if (t >= 300) dead = true; return false; }, { ...clock, gaveUp: () => dead });
  assert.equal(await died, false);
  assert.equal(t, 300);
  t = 0;
  const slow = [];
  assert.equal(await waitFor(async () => false, { ...clock, timeoutMs: 7000, onSlow: ms => slow.push(ms) }), false);
  assert.ok(t >= 7000);
  assert.deepEqual(slow.map(ms => Math.floor(ms / 1000)), [2, 4, 6]);
});

test('les pàgines locals són al paquet i no depenen de res de fora', () => {
  const pkg = JSON.parse(fs.readFileSync(path.join(__dirname, 'package.json'), 'utf8'));
  for (const f of ['startup.js', 'loading.html', 'error.html']) assert.ok(pkg.build.files.includes(f), f);
  for (const f of ['loading.html', 'error.html']) {
    const html = fs.readFileSync(path.join(__dirname, f), 'utf8');
    assert.match(html, /Content-Security-Policy" content="default-src 'none'/, f);
    assert.doesNotMatch(html, /(src|href)="https?:/, f);
    // La franja de dalt fa de barra de títol: sense, la finestra no es pot moure.
    assert.match(html, /-webkit-app-region:drag/, f);
    for (const k of ['bg', 'bar', 'ink', 'accent']) assert.match(html, new RegExp("\\['" + k + "'"), f + ' ' + k);
  }
  const error = fs.readFileSync(path.join(__dirname, 'error.html'), 'utf8');
  assert.doesNotMatch(error, /onclick=/, 'sense handlers inline');
  assert.match(error, /retryBackend/);
  assert.match(error, /startupError/);
});
