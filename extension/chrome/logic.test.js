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
