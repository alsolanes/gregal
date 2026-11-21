// plainText: el que s'enganxa a WhatsApp/Telegram. Mateixa regla que ShareText.kt.
const test = require('node:test');
const assert = require('node:assert');
const { plainText } = require('./share-msg.js');

test('les targetes passen a text llegible', () => {
  const md = 'Aquí ho tens:\n\n```gregal-plan\n{"title":"Dissabte a Girona","items":[{"time":"10:00","title":"Tren","detail":"Rodalies R11"},{"title":"Dinar"}],"total":"45 €"}\n```\n';
  assert.strictEqual(plainText(md, 'ca'), 'Aquí ho tens:\n\n*Dissabte a Girona*\n10:00 · Tren — Rodalies R11\n• Dinar\nTotal: 45 €');
  const res = '```gregal-result\n{"title":"Mòbils","rows":[{"lead":"Pixel 9a","text":"càmera","value":"399 €","pick":true,"url":"https://x.cat/p"}],"sources":[{"url":"https://x.cat"}]}\n```';
  assert.strictEqual(plainText(res, 'ca'), '*Mòbils*\n• Pixel 9a — càmera: 399 € ✓\n  https://x.cat/p\n\nhttps://x.cat');
  const rec = '```gregal-recipe\n{"title":"Crema","servings":2,"ingredients":[{"name":"carbassa","qty":"500 g"}],"steps":[{"text":"Bull","minutes":20}]}\n```';
  assert.strictEqual(plainText(rec, 'en'), '*Crema*\n2 servings\n\nIngredients:\n• 500 g carbassa\n\nSteps:\n1. Bull (20 min)');
  const route = '```gregal-route\n{"from":"Vic","to":"Barcelona","depart":"8:05","arrive":"9:20","service":"R3","price":"4,6 €","alternatives":[{"time":"8:35","service":"R3"}]}\n```';
  assert.strictEqual(plainText(route, 'ca'), '*Vic → Barcelona*\n8:05 → 9:20 · R3 · 4,6 €\n  · 8:35 · R3');
  const chart = '```chart\n{"title":"Vendes","type":"bar","labels":["gen","feb"],"series":[{"label":"2026","values":[3,5]}]}\n```';
  assert.strictEqual(plainText(chart, 'ca'), '*Vendes*\n• gen: 3\n• feb: 5');
});

test('markdown net: negreta, enllaços, fotos i documents', () => {
  const md = '## Títol\n\n**Molt** bé, mira [la web](https://a.cat) i [el doc](/api/winds/doc?name=a.xlsx).\n\n![foto](https://img.cat/a.jpg)\n![feta](/api/winds/image?id=0123456789abcdef01234567.jpg)\n- un\n- dos';
  assert.strictEqual(plainText(md, 'ca'), '*Títol*\n\n*Molt* bé, mira la web (https://a.cat) i el doc.\n\nhttps://img.cat/a.jpg\n\n• un\n• dos');
});

test('un bloc que no s\'entén es queda com estava', () => {
  const md = '```gregal-plan\n{trencat\n```';
  assert.strictEqual(plainText(md, 'ca'), md);
});

test('desar una resposta envia el markdown tal qual amb kind auto', async () => {
  const { saveReply } = require('./share-msg.js');
  const md = 'Té:\n\n```gregal-recipe\n{"title":"Crema","servings":2,"ingredients":[{"name":"carbassa","qty":"500 g"}]}\n```';
  const calls = [];
  globalThis.api = async (url, opts) => { calls.push({ url, opts }); return { ok: true, json: async () => ({ id: 'n1', kind: 'recipe', title: 'Crema' }) }; };
  try {
    const note = await saveReply(md);
    assert.strictEqual(note.id, 'n1');
  } finally { delete globalThis.api; }
  assert.strictEqual(calls.length, 1);
  assert.strictEqual(calls[0].url, '/api/notes');
  assert.strictEqual(calls[0].opts.method, 'POST');
  assert.deepStrictEqual(JSON.parse(calls[0].opts.body), { action: 'save', body: md, kind: 'auto' });
});
