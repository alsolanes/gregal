const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

function extractFunction(source, name, nextName) {
  const start = source.indexOf(`function ${name}(`);
  assert.notEqual(start, -1, `${name} exists in the inline app script`);
  const end = source.indexOf(`\n}\nfunction ${nextName}(`, start);
  assert.notEqual(end, -1, `${name} ends before ${nextName}`);
  return source.slice(start, end + 2);
}

function makeElement() {
  const classes = new Set();
  return {
    hidden: false, innerHTML: '', title: '', disabled: false, dataset: {}, attributes: {},
    classList: {
      toggle(name, enabled) { enabled ? classes.add(name) : classes.delete(name); },
      contains(name) { return classes.has(name); },
    },
    setAttribute(name, value) { this.attributes[name] = String(value); },
  };
}

function loadTurnStatus() {
  const source = fs.readFileSync(path.join(__dirname, 'index.html'), 'utf8');
  const elements = new Map(['turnBar', 'turnTimer', 'queueList', 'send', 'statusDot'].map(id => [id, makeElement()]));
  elements.get('queueList').querySelectorAll = () => [];
  const sandbox = {
    $, Date, Math,
    setInterval() { return 1; }, clearInterval() {}, setTimeout() {},
    gregalT(key) {
      return ({
        'estat.esperant': 'Esperant resposta',
        'estat.completat': 'Completat',
        'estat.reconnectant': 'Reconnectant',
        'ui.turn.treballant': 'Treballant',
        'ui.cua.etiqueta': 'A la cua',
        'ui.cua.treu': 'Treu de la cua',
      })[key] || key;
    },
    escapeHTML: value => String(value),
    sendLabel: () => 'Envia',
    updateInspector() {}, sys() {}, todoAlTranscript() {},
    window: { gregalSession: 'default' },
    stoppedVisualSessions: new Set(),
    visualWorking: () => sandbox.sending,
    state: {}, sending: false, turnT0: 0, turnTick: null, turnNote: '',
    turnPhase: 'idle', turnCompletedUntil: 0, cua: [], todoActual: [],
  };
  function $(id) { return elements.get(id); }
  sandbox.$ = $;
  const duration = extractFunction(source, 'durada', 'pintaTurnBar');
  const pinta = extractFunction(source, 'pintaTurnBar', 'encua');
  const sending = extractFunction(source, 'setSending', 'onSessionSwitch');
  vm.runInNewContext(`${duration}\n${pinta}\n${sending}\nglobalThis.turnApi = { pintaTurnBar, setSending };`, sandbox, { filename: 'index.html#turn-status' });
  return { api: sandbox.turnApi, elements, sandbox };
}

test('waiting phase keeps its label visible while live announcements use the status line', () => {
  const { api, elements, sandbox } = loadTurnStatus();
  api.setSending(true, true);
  sandbox.turnPhase = 'waiting';
  api.pintaTurnBar();

  assert.equal(elements.get('turnBar').hidden, false);
  assert.equal(elements.get('turnTimer').dataset.phase, 'waiting');
  assert.match(elements.get('turnTimer').innerHTML, /Esperant resposta/);
  assert.equal(elements.get('turnTimer').attributes['aria-live'], 'off');
});

test('completed phase remains visible briefly after sending stops', () => {
  const { api, elements, sandbox } = loadTurnStatus();
  api.setSending(true, true);
  sandbox.turnPhase = 'completed';
  api.setSending(false, true);

  assert.equal(elements.get('turnBar').hidden, false);
  assert.equal(elements.get('turnTimer').dataset.phase, 'completed');
  assert.match(elements.get('turnTimer').innerHTML, /Completat/);
});

test('error return to idle clears the timer text and hides an otherwise empty bar', () => {
  const { api, elements, sandbox } = loadTurnStatus();
  api.setSending(true, true);
  sandbox.turnPhase = 'idle'; // The stream error path sets idle before its finally block.
  api.setSending(false, true);

  assert.equal(elements.get('turnTimer').dataset.phase, 'idle');
  assert.equal(elements.get('turnTimer').innerHTML, '');
  assert.equal(elements.get('turnBar').hidden, true);
});
