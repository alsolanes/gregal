const { test } = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { modelProgress } = require('./model-progress.js');

const tr = async lang => {
  const { CLAUS } = await import(pathToFileURL(path.join(__dirname, 'i18n.js')).href);
  return k => CLAUS[lang][k];
};
const ev = o => Object.assign({ phase: 'model', state: 'receiving', reasoning: '0', text: '0', tool: '0' }, o);

// Vist en viu: 4m42s de «Pensant · 67k» sense saber què feia el model.
test('a tool being written says which file and how many characters', async () => {
  const ca = await tr('ca'), en = await tr('en');
  const d = ev({ reasoning: '900', tool: '67012', tool_name: 'write', tool_target: 'jocs/snake/index.html' });
  assert.deepEqual(modelProgress(d, ca), { key: 'model-tool', text: 'Escrivint jocs/snake/index.html · 67k caràcters', tail: '' });
  assert.equal(modelProgress(d, en).text, 'Writing jocs/snake/index.html · 67k characters');
});

test('a tool without a file yet says which tool', async () => {
  const ca = await tr('ca'), en = await tr('en');
  const d = ev({ tool: '40', tool_name: 'bash' });
  assert.equal(modelProgress(d, ca).text, 'Preparant bash · 40 caràcters');
  assert.equal(modelProgress(d, en).text, 'Preparing bash · 40 characters');
  assert.equal(modelProgress(d, en).key, 'model-tool');
  assert.equal(modelProgress(ev({ tool: '40' }), en).text, 'Preparing a tool call · 40 characters');
});

test('only reasoning shows the count as characters and the reasoning tail', async () => {
  const ca = await tr('ca'), en = await tr('en');
  const d = ev({ reasoning: '67000', tail: 'Faré el joc en un sol fitxer' });
  assert.deepEqual(modelProgress(d, ca), { key: 'thinking', text: 'Pensant · 67k caràcters', tail: 'Faré el joc en un sol fitxer' });
  assert.equal(modelProgress(d, en).text, 'Thinking · 67k characters');
  assert.equal(modelProgress(ev({}), en).text, 'Thinking');
  assert.equal(modelProgress(ev({ reasoning: '1250' }), en).text, 'Thinking · 1.3k characters');
});

test('waiting and answer text keep their labels and drop the tail', async () => {
  const en = await tr('en');
  assert.deepEqual(modelProgress(ev({ state: 'waiting', tail: 'x' }), en), { key: 'model', text: 'Waiting for the model', tail: '' });
  assert.deepEqual(modelProgress(ev({ text: '20', reasoning: '500' }), en), { key: 'writing', text: 'Writing', tail: '' });
});
