import test from 'node:test';
import assert from 'node:assert/strict';
import { baseUrl, connection, parseSSE, splitBlocks, buildMessage, MAX_CONTEXT, isPdfUrl, bytesToBase64 } from './logic.js';

test('adreça i port com al mòbil', () => {
  assert.equal(baseUrl('127.0.0.1', '8097'), 'http://127.0.0.1:8097');
  assert.equal(baseUrl('', ''), 'http://127.0.0.1:8097');
  assert.equal(baseUrl('192.168.1.62', '9000'), 'http://192.168.1.62:9000');
  assert.equal(baseUrl('vents.exemple.cat', ''), 'https://vents.exemple.cat');
  assert.equal(baseUrl('https://vents.exemple.cat/', ''), 'https://vents.exemple.cat');
  assert.equal(baseUrl('http://casa.local:8098', '1'), 'http://casa.local:8098');
});

test('SSE: esdeveniments sencers i la resta per a després', () => {
  const { events, rest } = parseSSE('event: token\ndata: {"token":"Ho"}\n\nevent: token\ndata: {"tok');
  assert.deepEqual(events, [{ event: 'token', data: { token: 'Ho' } }]);
  assert.equal(rest, 'event: token\ndata: {"tok');
  assert.equal(parseSSE(rest + 'en":"la"}\n\n').events[0].data.token, 'la');
});

test('blocs de codi per inserir', () => {
  const b = splitBlocks('Aquí tens:\n```javascript\nfunction a() {}\n```\nI ja està.');
  assert.deepEqual(b.map((x) => x.type), ['text', 'code', 'text']);
  assert.equal(b[1].text, 'function a() {}');
  assert.equal(b[1].lang, 'javascript');
  assert.equal(splitBlocks('```\n=SUMA(A1:A3)')[0].text, '=SUMA(A1:A3)'); // encara arribant
});

test('el missatge porta el context tallat i la pista per inserir', () => {
  assert.equal(buildMessage('hola', null, 'ca'), 'hola');
  // Sense text llegible, com a mínim quina pàgina és.
  assert.equal(buildMessage('mira-ho', { kind: 'web', title: 'AI Models', url: 'https://artificialanalysis.ai/', content: '' }, 'ca'),
    'mira-ho\n\n[Web: «AI Models» — https://artificialanalysis.ai/]');
  const m = buildMessage('Resumeix', { kind: 'sheets', title: 'Pressupost', url: 'https://docs.google.com/x', range: 'B2', content: 'a,b\n1,2' }, 'ca');
  assert.match(m, /^Resumeix\n\n\[Google Sheets: «Pressupost» — https:\/\/docs.google.com\/x — selecció B2\]\na,b\n1,2\n\[\/Google Sheets\]/);
  assert.match(m, /tabuladors/);
  const long = buildMessage('x', { kind: 'docs', title: 'T', url: 'u', content: 'a'.repeat(MAX_CONTEXT + 50) }, 'en');
  assert.match(long, /truncated/);
  assert.ok(long.length < MAX_CONTEXT + 600);
});

test("amb l'app d'escriptori no cal configurar res", () => {
  assert.deepEqual(connection({}), { addr: '127.0.0.1', port: '8097', key: '' });
  assert.deepEqual(connection({ mode: 'desktop', addr: 'x', key: 'k' }), { addr: '127.0.0.1', port: '8097', key: '' });
  assert.deepEqual(connection({ mode: 'server', addr: 'vents.exemple.cat', port: '', key: 'k' }), { addr: 'vents.exemple.cat', port: '', key: 'k' });
});

test('PDF: es reconeix per la ruta i el missatge en duu la ruta a Gregal', () => {
  assert.equal(isPdfUrl('https://exemple.cat/docs/Factura.PDF?v=2#page=3'), true);
  assert.equal(isPdfUrl('https://exemple.cat/pdf/visor.html'), false);
  assert.equal(isPdfUrl('file:///C:/a.pdf'), false);
  assert.equal(isPdfUrl('no és una url'), false);
  assert.equal(bytesToBase64(new Uint8Array([37, 80, 68, 70])), 'JVBERg==');
  const msg = buildMessage('resumeix', { kind: 'pdf', title: 'Factura', url: 'https://x/f.pdf', content: 'Total 10 €', path: '/tmp/gregal-office-1-f.pdf' }, 'ca');
  assert.match(msg, /\[PDF: «Factura»/);
  assert.match(msg, /fitxer a Gregal: \/tmp\/gregal-office-1-f\.pdf — pdf_read/);
  assert.match(msg, /Total 10 €/);
});

test('markdown del vent: segur i amb enllaços, llistes i taules', async () => {
  const { renderMarkdown } = await import('./logic.js');
  assert.equal(renderMarkdown('Hola **món**, mira [DeepSeek](https://deepseek.com/x) i `v4`.'),
    '<p>Hola <b>món</b>, mira <a href="https://deepseek.com/x" target="_blank" rel="noopener">DeepSeek</a> i <code>v4</code>.</p>');
  assert.equal(renderMarkdown('<img src=x onerror=alert(1)> [x](javascript:alert(1))'),
    '<p>&lt;img src=x onerror=alert(1)&gt; [x](javascript:alert(1))</p>');
  assert.equal(renderMarkdown('Punts:\n- un\n- dos\n\n1. a\n2. b'), '<p>Punts:</p><ul><li>un</li><li>dos</li></ul><ol><li>a</li><li>b</li></ol>');
  assert.equal(renderMarkdown('## Resum\n| Model | HLE |\n|---|---|\n| Opus | 56 |'),
    '<h3>Resum</h3><table><tr><th>Model</th><th>HLE</th></tr><tr><td>Opus</td><td>56</td></tr></table>');
  assert.equal(renderMarkdown('a *b* 2*3*4'), '<p>a <i>b</i> 2*3*4</p>');
});
