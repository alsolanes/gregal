const { test } = require('node:test');
const assert = require('node:assert/strict');

test('Office controls and action prompts follow the selected language', async () => {
  const { office, quickActions } = await import('./office.js');
  const previousStorage = global.localStorage;
  const previousDocument = global.document;
  const previousDoc = office.doc;
  let language = 'en';
  global.localStorage = { getItem: () => language };
  const controls = Object.fromEntries(['docOpenApp', 'docVista', 'docQuick', 'docAsk'].map(id => [id, {
    textContent: '', children: [], value: '', focus() {},
    appendChild(child) { this.children.push(child); },
    set innerHTML(_) { this.children = []; },
  }]));
  global.document = {
    getElementById: id => controls[id],
    createElement: () => ({ textContent: '', onclick: null }),
  };
  try {
    assert.equal(quickActions('docx')[0][0], 'Summarize');
    assert.match(quickActions('docx')[0][1], /^Summarize/);
    office.doc = { kind: 'xlsx', view: 'vista' };
    office.renderControls();
    assert.equal(controls.docOpenApp.textContent, 'Open with Excel');
    assert.equal(controls.docQuick.children[0].textContent, 'Explain data');
    controls.docQuick.children[2].onclick();
    assert.match(controls.docAsk.value, /^Set the value ___/);

    language = 'ca';
    office.renderControls();
    assert.equal(controls.docOpenApp.textContent, 'Obre amb Excel');
    assert.equal(controls.docQuick.children[0].textContent, 'Explica les dades');
    controls.docQuick.children[2].onclick();
    assert.match(controls.docAsk.value, /^Posa el valor ___/);
    assert.match(quickActions('docx')[0][1], /^Resumeix/);
    assert.equal(controls.docQuick.children.length, 3);
  } finally {
    global.localStorage = previousStorage;
    global.document = previousDocument;
    office.doc = previousDoc;
  }
});
