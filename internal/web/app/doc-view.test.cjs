const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const pm = require('./vendor/prosemirror.min.js');
const { editor, commands, keys, link, style, align, current, active, table, image, imageName, toDoc, toMd, onDocChanged } = require('./doc-view.js');

const DOC = {
  id: 'd1', version: 3,
  blocks: [
    { id: 'a', md: '# Pla', version: 1 },
    { id: 'b', md: '- primer\n- segon', version: 2 },
    { id: 'c', md: 'Tancament', version: 3 },
  ],
};

// fakeEdit fa de servidor de les rutes de blocs: apunta les crides i torna
// la resposta que toca, per defecte un 200.
function fakeEdit() {
  const e = {
    calls: [], next: [], errors: [],
    reply(r) { e.next.push(r); },
    async put(id, md, base) { e.calls.push(['put', id, md, base]); return e.take({ version: 9, block: { id, md, version: 9 } }); },
    async post(after, md) { e.calls.push(['post', after, md]); return e.take({ version: 9, blocks: [{ id: 'n' + e.calls.length, md, version: 9 }] }); },
    async del(id, base) { e.calls.push(['del', id, base]); return e.take({ version: 9 }); },
    take(body) { return e.next.shift() || { status: 200, body }; },
    error: m => e.errors.push(m),
  };
  return e;
}

const md = ed => toMd(ed.state.doc.content.content);
const ids = ed => { const out = []; ed.state.doc.forEach(n => out.push(n.attrs.id)); return out; };
// pos torna la posició just abans de text (o just després, amb after) dins del document.
function pos(ed, text, after = false) {
  let at = -1;
  ed.state.doc.descendants((n, p) => {
    if (at < 0 && n.isText && n.text.includes(text)) at = p + n.text.indexOf(text) + (after ? text.length : 0);
  });
  assert.ok(at >= 0, text + ' is in the document');
  return at;
}
const select = (ed, a, b = a) => ed.dispatch(ed.state.tr.setSelection(pm.TextSelection.create(ed.state.doc, a, b)));
const type = (ed, text, at) => { select(ed, at); ed.dispatch(ed.state.tr.insertText(text)); };
const block = (s, id = 'a') => ({ version: 1, blocks: [{ id, md: s, version: 1 }] });

test('a document with every basic style goes editor → markdown → editor unchanged', () => {
  const blocks = [
    { id: 'h1', md: '# Títol' }, { id: 'h2', md: '## Secció' }, { id: 'h3', md: '### Apartat' },
    { id: 'p', md: 'Un **negre**, *cursiva*, ++subratllat++, ~~ratllat~~, `codi` i [un enllaç](https://exemple.cat).' },
    { id: 'ul', md: '- u\n- dos' }, { id: 'ol', md: '1. a\n2. b' }, { id: 'q', md: '> cita' },
    { id: 'code', md: '```\nx = 1\n```' }, { id: 'hr', md: '---' },
  ];
  const doc = toDoc(blocks);
  assert.equal(doc.childCount, blocks.length, 'each block is one top-level node');
  const back = [];
  doc.forEach(n => back.push({ id: n.attrs.id, md: toMd([n]) }));
  assert.deepEqual(back, blocks);
  assert.ok(toDoc(back).eq(doc));
});

test('underline and strikethrough use the 078 syntax', () => {
  const p = toDoc([{ id: 'p', md: '++u++ ~~s~~' }]).firstChild;
  assert.deepEqual([p.child(0).marks[0].type.name, p.child(2).marks[0].type.name], ['underline', 'strike']);
});

const ACTIONS = [
  ['bold', '**hola** món'], ['italic', '*hola* món'], ['underline', '++hola++ món'], ['strike', '~~hola~~ món'],
  ['code', '`hola` món'], ['h1', '# hola món'], ['h2', '## hola món'], ['h3', '### hola món'],
  ['bullets', '- hola món'], ['numbers', '1. hola món'], ['quote', '> hola món'], ['rule', 'hola món\n\n---'],
];
for (const [name, want] of ACTIONS) {
  test('toolbar action ' + name + ' writes ' + JSON.stringify(want), () => {
    const ed = editor(block('hola món'), fakeEdit());
    if (name === 'rule') select(ed, pos(ed, 'món', true));
    else select(ed, pos(ed, 'hola'), pos(ed, 'hola', true));
    assert.equal(ed.run(commands[name]), true);
    assert.equal(md(ed), want);
  });
}

test('toolbar action paragraph turns a heading back into a paragraph', () => {
  const ed = editor(block('## hola'), fakeEdit());
  ed.run(commands.paragraph);
  assert.equal(md(ed), 'hola');
});

test('toolbar action link wraps the selection in a link', () => {
  const ed = editor(block('hola món'), fakeEdit());
  select(ed, pos(ed, 'món'), pos(ed, 'món', true));
  ed.run(link('https://exemple.cat'));
  assert.equal(md(ed), 'hola [món](https://exemple.cat)');
});

test('undo and redo', () => {
  const ed = editor(block('hola món'), fakeEdit());
  select(ed, pos(ed, 'hola'), pos(ed, 'hola', true));
  ed.run(commands.bold);
  ed.run(commands.undo);
  assert.equal(md(ed), 'hola món');
  ed.run(commands.redo);
  assert.equal(md(ed), '**hola** món');
});

const STYLE_ACTIONS = [
  ['color', '#c0392b', '[hola]{color=#c0392b} món'], ['bg', '#fff3a3', '[hola]{bg=#fff3a3} món'],
  ['font', 'Times New Roman', '[hola]{font="Times New Roman"} món'], ['size', '14', '[hola]{size=14} món'],
];
for (const [key, value, want] of STYLE_ACTIONS) {
  test('toolbar style ' + key + ' writes ' + JSON.stringify(want) + ' and shows its value', () => {
    const ed = editor(block('hola món'), fakeEdit());
    select(ed, pos(ed, 'hola'), pos(ed, 'hola', true));
    assert.equal(current(ed.state)[key], '');
    ed.run(style(key, value));
    assert.equal(md(ed), want);
    assert.equal(current(ed.state)[key], value);
    select(ed, pos(ed, 'món'), pos(ed, 'món', true));
    assert.equal(current(ed.state)[key], '', 'unstyled text shows the default');
  });
}

test('styles add up on the same text, and an empty value takes one off', () => {
  const ed = editor(block('hola **món**'), fakeEdit());
  select(ed, pos(ed, 'hola'), pos(ed, 'món', true));
  ed.run(style('size', '9'));
  ed.run(style('color', '#2e75b6'));
  assert.equal(md(ed), '[hola **món**]{color=#2e75b6 size=9}');
  select(ed, pos(ed, 'món'), pos(ed, 'món', true));
  ed.run(style('size', ''));
  assert.equal(md(ed), '[hola]{color=#2e75b6 size=9} [**món**]{color=#2e75b6}');
  assert.deepEqual(current(ed.state), { align: 'left', color: '#2e75b6', bg: '', font: '', size: '' });
  ed.run(style('color', ''));
  assert.equal(md(ed), '[hola]{color=#2e75b6 size=9} **món**');
});

test('a style picked with nothing selected applies to what is typed next', () => {
  const ed = editor(block('hola'), fakeEdit());
  select(ed, pos(ed, 'hola', true));
  ed.run(style('color', '#27ae60'));
  assert.equal(current(ed.state).color, '#27ae60');
  ed.dispatch(ed.state.tr.insertText(' món'));
  assert.equal(md(ed), 'hola [món]{color=#27ae60}');
});

for (const [value, want] of [['center', '# hola\n{align=center}'], ['right', '# hola\n{align=right}'], ['justify', '# hola\n{align=justify}']]) {
  test('toolbar alignment ' + value + ' writes the 078 line after the heading and shows it', () => {
    const ed = editor(block('# hola'), fakeEdit());
    assert.equal(current(ed.state).align, 'left');
    ed.run(align(value));
    assert.equal(md(ed), want);
    assert.equal(current(ed.state).align, value);
  });
}

test('toolbar alignment left takes the line off a paragraph, and a list cannot be aligned', () => {
  const ed = editor(block('hola món\n{align=center}'), fakeEdit());
  assert.equal(current(ed.state).align, 'center');
  ed.run(align('left'));
  assert.equal(md(ed), 'hola món');
  const list = editor(block('- hola'), fakeEdit());
  assert.equal(list.run(align('center')), false);
  assert.equal(md(list), '- hola');
});

test('the toolbar shows as pressed the marks of the selection and the block the cursor is in', () => {
  const ed = editor({ version: 1, blocks: [{ id: 'h', md: '## Pla', version: 1 }, { id: 'p', md: 'un **fort** i *suau*', version: 1 }, { id: 'l', md: '> - punt', version: 1 }] }, fakeEdit());
  select(ed, pos(ed, 'fort'), pos(ed, 'fort', true));
  assert.deepEqual([...active(ed.state)].sort(), ['bold', 'paragraph']);
  select(ed, pos(ed, 'un'), pos(ed, 'fort', true));
  assert.deepEqual([...active(ed.state)], ['paragraph'], 'a mark on part of the selection is not pressed');
  select(ed, pos(ed, 'suau', true));
  assert.deepEqual([...active(ed.state)].sort(), ['italic', 'paragraph']);
  select(ed, pos(ed, 'Pla'));
  assert.deepEqual([...active(ed.state)], ['h2']);
  select(ed, pos(ed, 'punt'));
  assert.deepEqual([...active(ed.state)].sort(), ['bullets', 'quote']);
});

test('a styled document goes editor → markdown → editor unchanged', () => {
  const blocks = [
    { id: 'h', md: '# Títol\n{align=center}' },
    { id: 'p', md: 'Un [vermell]{color=#c0392b}, [**ressaltat**]{bg=#fff3a3} i [gran]{color=#2e75b6 bg=#cfe2f3 font="Georgia" size=10.5}.\n{align=justify}' },
    { id: 'q', md: '> [*cita*]{font="Courier New" size=9}' },
    { id: 'r', md: 'A la dreta\n{align=right}' },
  ];
  const doc = toDoc(blocks);
  assert.equal(doc.childCount, blocks.length);
  const back = [];
  doc.forEach(n => back.push({ id: n.attrs.id, md: toMd([n]) }));
  assert.deepEqual(back, blocks);
  assert.ok(toDoc(back).eq(doc));
});

test('styling with the toolbar writes the blocks that the Word export test reads', () => {
  // Les mateixes cadenes que TestDocxExportaElsEstilsDeLEditor (internal/ooxml).
  const ed = editor({ version: 1, blocks: [{ id: 'a', md: '# Títol', version: 1 }, { id: 'b', md: 'Text en vermell i gran.', version: 1 }] }, fakeEdit());
  select(ed, pos(ed, 'Títol'), pos(ed, 'Títol', true));
  ed.run(align('center'));
  ed.run(style('font', 'Georgia'));
  select(ed, pos(ed, 'vermell'), pos(ed, 'vermell', true));
  ed.run(style('color', '#c0392b'));
  ed.run(style('bg', '#fff3a3'));
  select(ed, pos(ed, 'gran'), pos(ed, 'gran', true));
  ed.run(style('size', '14'));
  ed.run(style('font', 'Times New Roman'));
  ed.run(align('justify'));
  const out = [];
  ed.state.doc.forEach(n => out.push(toMd([n])));
  assert.deepEqual(out, ['# [Títol]{font="Georgia"}\n{align=center}', 'Text en [vermell]{color=#c0392b bg=#fff3a3} i [gran]{font="Times New Roman" size=14}.\n{align=justify}']);
});

test('the style controls have their strings in Catalan and English', () => {
  const view = fs.readFileSync(path.join(__dirname, 'doc-view.js'), 'utf8');
  const styles = view.slice(view.indexOf('const STYLES'), view.indexOf('function view('));
  const labels = [...styles.matchAll(/\['([\w#]*)', ('[^']+'|"[^"]+"), ('[^']+'|"[^"]+")/g)];
  assert.ok(labels.length >= 20);
  for (const m of labels) assert.notEqual(m[2], m[3], (m[1] || 'default') + ' has its own Catalan and English label');
  assert.match(view, /edit\.tx\('Estil del text', 'Text style'\)/);
});

test('markdown the export would not style stays text in the editor', () => {
  for (const s of ['[a]{size=big}', '[a]{font=Georgia}', '[a]{size=9 size=10}', 'hola\n\n{align=center}']) {
    const doc = toDoc([{ id: 'x', md: s }]);
    let styled = false;
    doc.descendants(n => { if (n.marks?.some(m => m.type.name === 'span') || n.attrs.align) styled = true; });
    assert.equal(styled, false, s);
  }
});

test('the usual shortcuts run the same commands as the toolbar', () => {
  assert.equal(keys['Mod-b'], commands.bold);
  assert.equal(keys['Mod-i'], commands.italic);
  assert.equal(keys['Mod-u'], commands.underline);
  assert.equal(keys['Mod-Alt-1'], commands.h1);
  assert.equal(keys['Mod-Alt-2'], commands.h2);
  assert.equal(keys['Mod-Alt-3'], commands.h3);
  assert.equal(keys['Mod-z'], commands.undo);
  assert.equal(keys['Mod-y'], commands.redo);
});

test('saving is debounced: a changed node is PUT once with its base_version, an untouched one is not', async t => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const ed = fakeEdit(), e = editor(DOC, ed);
  type(e, ' nou', pos(e, 'Tancament', true));
  type(e, '!', pos(e, 'Tancament nou', true));
  assert.deepEqual(ed.calls, [], 'nothing is sent while typing');
  t.mock.timers.tick(700);
  await new Promise(r => setImmediate(r));
  assert.deepEqual(ed.calls, [['put', 'c', 'Tancament nou!', 3]]);
  type(e, '?', pos(e, 'nou!', true));
  await e.flush();
  assert.deepEqual(ed.calls.at(-1), ['put', 'c', 'Tancament nou!?', 9], 'the next save starts from the saved version');
});

test('a new node is POSTed after its predecessor and keeps the id the server gives', async () => {
  const ed = fakeEdit(), e = editor(DOC, ed);
  select(e, pos(e, 'Pla', true));
  e.run(keys.Enter);
  e.dispatch(e.state.tr.insertText('Nou'));
  await e.flush();
  assert.deepEqual(ed.calls, [['post', 'a', 'Nou']]);
  assert.deepEqual(ids(e), ['a', 'n1', 'b', 'c']);
  e.dispatch(e.state.tr.insertText('!'));
  await e.flush();
  assert.deepEqual(ed.calls.at(-1), ['put', 'n1', 'Nou!', 9]);
});

test('Enter in the middle of a node splits it: the first half is PUT, the second POSTed after it', async () => {
  const ed = fakeEdit(), e = editor(DOC, ed);
  select(e, pos(e, 'ment'));
  e.run(keys.Enter);
  await e.flush();
  assert.deepEqual(ed.calls, [['put', 'c', 'Tanca', 3], ['post', 'c', 'ment']]);
});

test('a removed node is DELETEd, and an emptied one too', async () => {
  const ed = fakeEdit(), e = editor(DOC, ed);
  const r = e.state.doc.child(0).nodeSize;
  e.dispatch(e.state.tr.delete(0, r));
  await e.flush();
  assert.deepEqual(ed.calls, [['del', 'a', 1]]);
  select(e, pos(e, 'Tancament'), pos(e, 'Tancament', true));
  e.dispatch(e.state.tr.deleteSelection());
  await e.flush();
  assert.deepEqual(ed.calls.at(-1), ['del', 'c', 3]);
  assert.deepEqual(ids(e), ['b', null], 'the empty node stays to write in, as a new block');
});

test('turning a paragraph into a heading or a list is a PUT of the same block', async () => {
  const ed = fakeEdit(), e = editor(DOC, ed);
  select(e, pos(e, 'Tancament'));
  e.run(commands.h2);
  e.run(commands.quote);
  await e.flush();
  assert.deepEqual(ed.calls, [['put', 'c', '> ## Tancament', 3]]);
  assert.deepEqual(ids(e), ['a', 'b', 'c']);
});

test('a 409 keeps the person\'s text with the agent\'s version, until they keep theirs or take the agent\'s', async () => {
  const ed = fakeEdit(), e = editor(DOC, ed);
  type(e, ' meu', pos(e, 'Tancament', true));
  ed.reply({ status: 409, body: { error: 'changed', block: { id: 'c', md: 'Final de l\'agent', version: 7 } } });
  await e.flush();
  assert.equal(md(e).split('\n\n')[2], 'Tancament meu');
  assert.equal(e.conflicts.get('c').md, 'Final de l\'agent');
  type(e, '!', pos(e, 'meu', true));
  await e.flush();
  assert.equal(ed.calls.length, 1, 'a conflicted block is not saved on its own');
  await e.keepMine('c');
  assert.deepEqual(ed.calls.at(-1), ['put', 'c', 'Tancament meu!', 7], 'it overwrites the version the notice shows');
  assert.equal(e.conflicts.size, 0);

  type(e, '?', pos(e, 'meu!', true));
  ed.reply({ status: 409, body: { error: 'changed', block: { id: 'c', md: 'Una altra', version: 11 } } });
  await e.flush();
  e.useTheirs('c');
  assert.equal(md(e).split('\n\n')[2], 'Una altra');
  await e.flush();
  assert.equal(ed.calls.length, 3, 'taking the agent\'s text sends nothing');
});

test('a save that fails on the network is reported and keeps the text', async () => {
  const ed = fakeEdit(), e = editor(DOC, ed);
  ed.put = async () => { throw new Error('offline'); };
  type(e, ' x', pos(e, 'Tancament', true));
  await e.flush();
  assert.deepEqual(ed.errors, ['offline']);
  assert.match(md(e), /Tancament x$/);
});

test('agent events update their node in place and the cursor stays on its text', () => {
  const e = editor(DOC, fakeEdit());
  e.focused = true;
  select(e, pos(e, 'Tanca', true));
  e.apply({ op: 'replace', block: { id: 'a', md: '# Pla nou i més llarg', version: 4 }, version: 4 });
  e.apply({ op: 'insert', block: { id: 'x', md: 'Inserit', version: 5 }, after: 'b', version: 5 });
  e.apply({ op: 'insert', block: { id: 'y', md: 'Al principi', version: 6 }, version: 6 });
  e.apply({ op: 'delete', block: { id: 'b' }, version: 7 });
  assert.deepEqual(ids(e), ['y', 'a', 'x', 'c']);
  assert.equal(md(e), 'Al principi\n\n# Pla nou i més llarg\n\nInserit\n\nTancament');
  assert.equal(e.state.selection.head, pos(e, 'Tanca', true));
  assert.equal(e.version, 7);
  assert.equal(e.apply({ op: 'delete', block: { id: 'a' }, version: 3 }), false, 'events the loaded document had are ignored');
});

test('the node being edited waits for the person to leave it before an agent event lands', async () => {
  const ed = fakeEdit(), e = editor(DOC, ed);
  e.focused = true;
  select(e, pos(e, 'Pla', true));
  e.apply({ op: 'replace', block: { id: 'a', md: '# Agent', version: 4 }, version: 4 });
  assert.match(md(e), /^# Pla/);
  select(e, pos(e, 'Tanca'));
  assert.match(md(e), /^# Agent/, 'it lands when the cursor leaves');
  e.apply({ op: 'replace', block: { id: 'c', md: 'Agent c', version: 5 }, version: 5 });
  e.focused = false;
  assert.match(md(e), /Agent c$/, 'or when the editor loses the focus');
  await e.flush();
  assert.deepEqual(ed.calls, []);
});

test('the stream\'s copy of the person\'s own saves is not applied twice', async () => {
  const ed = fakeEdit(), e = editor(DOC, ed);
  select(e, pos(e, 'Tancament', true));
  e.run(keys.Enter);
  e.dispatch(e.state.tr.insertText('Nou'));
  const saving = e.flush();
  e.apply({ op: 'insert', block: { id: 'n1', md: 'Nou', version: 9 }, after: 'c', version: 9 });
  await saving;
  e.apply({ op: 'replace', block: { id: 'n1', md: 'Nou', version: 9 }, version: 9 });
  assert.deepEqual(ids(e), ['a', 'b', 'c', 'n1']);
});

test('an empty document has one empty node, and writing in it saves the first block', async () => {
  const ed = fakeEdit(), e = editor({ version: 1, blocks: [] }, ed);
  assert.deepEqual(ids(e), [null]);
  await e.flush();
  assert.deepEqual(ed.calls, [], 'nothing to save while it is empty');
  type(e, 'Primer dia', 1);
  await e.flush();
  assert.deepEqual(ed.calls, [['post', '', 'Primer dia']]);
});

test('doc_changed opens a closed document and only brings forward the one already open', () => {
  assert.equal(onDocChanged({ id: '', visible: false }, { id: 'd1' }), 'open');
  assert.equal(onDocChanged({ id: 'd1', visible: false }, { id: 'd1' }), 'open');
  assert.equal(onDocChanged({ id: 'd2', visible: true }, { id: 'd1' }), 'open');
  assert.equal(onDocChanged({ id: 'd1', visible: true }, { id: 'd1' }), 'front');
  // Un document d'un vent és d'una altra carpeta: el mateix id no és el mateix document.
  assert.equal(onDocChanged({ id: 'd1', visible: true, scope: '' }, { id: 'd1', scope: 'mine' }), 'open');
  assert.equal(onDocChanged({ id: 'd1', visible: true, scope: 'mine' }, { id: 'd1' }), 'open');
  assert.equal(onDocChanged({ id: 'd1', visible: true, scope: 'mine' }, { id: 'd1', scope: 'mine' }), 'front');
});

test('the panel and the toolbar have their strings in Catalan and English, and the editor library is loaded', () => {
  const src = fs.readFileSync(path.join(__dirname, 'docs-panel.js'), 'utf8');
  assert.match(src, /tx\('Descarrega en Word', 'Download as Word'\)/);
  assert.match(src, /tx\('Descarrega en PowerPoint', 'Download as PowerPoint'\)/);
  assert.match(src, /\/export\?format=' \+ format/);
  assert.match(src, /'\/api\/docs'/);
  assert.match(src, /tx\("L'agent ha canviat aquest bloc", 'The agent changed this block'\)/);
  const view = fs.readFileSync(path.join(__dirname, 'doc-view.js'), 'utf8');
  const tools = [...view.matchAll(/\['(\w+)', '[^']+', '([^']+)', '([^']+)'\]/g)];
  assert.deepEqual(tools.map(m => m[1]).sort(), [...Object.keys(commands), 'link', 'table', 'image'].sort(), 'every command has a button');
  for (const m of tools) assert.notEqual(m[2], m[3], m[1] + ' has its own Catalan and English label');
  for (const s of [/'Desa el meu text', 'Keep my text'/, /"Adreça de l'enllaç", 'Link address'/, /'Files × columnes', 'Rows × columns'/, /'Peu de la imatge', 'Image caption'/]) assert.match(view, s);
  assert.match(src, /'\/images'/);
  const index = fs.readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');
  const page = fs.readFileSync(path.join(__dirname, '..', 'docs_api.go'), 'utf8');
  for (const html of [index, page]) assert.ok(html.indexOf('/app/vendor/prosemirror.min.js') >= 0 && html.indexOf('/app/vendor/prosemirror.min.js') < html.indexOf('/app/doc-view.js'));
});

const TABLE = '| Fase | Data |\n| --- | :---: |\n| Beta | Març |';

test('a markdown table is one block and one node, and comes back to the same markdown', () => {
  const doc = toDoc([{ id: 't', md: TABLE }]);
  assert.equal(doc.childCount, 1);
  assert.equal(doc.firstChild.type.name, 'table');
  assert.equal(doc.firstChild.attrs.id, 't');
  assert.equal(toMd([doc.firstChild]), TABLE);
  const tricky = '| a\\|b | `c` |\n| --- | --- |\n|  | **d** |';
  assert.equal(toMd(toDoc([{ id: 't', md: tricky }]).content.content), tricky);
});

test('the toolbar inserts a table of the rows and columns chosen after the block, and typing in it POSTs it', async () => {
  const ed = fakeEdit(), e = editor(block('hola món'), ed);
  select(e, pos(e, 'hola'));
  assert.equal(e.run(table(2, 3)), true);
  e.dispatch(e.state.tr.insertText('Nom'));
  assert.equal(md(e), 'hola món\n\n| Nom |  |  |\n| --- | --- | --- |\n|  |  |  |');
  await e.flush();
  assert.deepEqual(ed.calls, [['post', 'a', '| Nom |  |  |\n| --- | --- | --- |\n|  |  |  |']]);
});

test('inside a table, rows and columns are added and removed and the block is PUT as a markdown table', async () => {
  const ed = fakeEdit(), e = editor(block(TABLE, 't'), ed);
  const text = editor(block('hola'), fakeEdit());
  for (const c of ['rowAdd', 'rowDelete', 'colAdd', 'colDelete']) assert.equal(text.run(commands[c]), false, c + ' does nothing outside a table');
  select(e, pos(e, 'Beta', true));
  e.run(commands.rowAdd);
  e.dispatch(e.state.tr.insertText('GA'));
  assert.equal(md(e), '| Fase | Data |\n| --- | :---: |\n| Beta | Març |\n| GA |  |');
  e.run(commands.colAdd);
  e.dispatch(e.state.tr.insertText('Juny'));
  assert.equal(md(e), '| Fase |  | Data |\n| --- | --- | :---: |\n| Beta |  | Març |\n| GA | Juny |  |', 'the column goes after the cursor\'s');
  select(e, pos(e, 'Març'));
  e.run(commands.colDelete);
  e.run(commands.rowDelete);
  assert.equal(md(e), '| Fase |  |\n| --- | --- |\n| GA | Juny |');
  await e.flush();
  assert.deepEqual(ed.calls, [['put', 't', '| Fase |  |\n| --- | --- |\n| GA | Juny |', 1]]);
});

test('Tab moves to the next cell and adds a row after the last one; Enter and Backspace do not break cells', () => {
  const e = editor(block('| a | b |\n| --- | --- |\n| c | d |', 't'), fakeEdit());
  select(e, pos(e, 'a', true));
  e.run(keys.Tab);
  e.dispatch(e.state.tr.insertText('1'));
  select(e, pos(e, 'd', true));
  e.run(keys.Tab);
  e.dispatch(e.state.tr.insertText('2'));
  select(e, pos(e, 'c'));
  assert.equal(e.run(keys.Backspace), true);
  assert.equal(e.run(keys.Enter), true);
  assert.equal(md(e), '| a | 1b |\n| --- | --- |\n| c | d |\n| 2 |  |');
});

test('an image goes in its own block as ![caption](images/<name>), and only names in the images folder are shown', async () => {
  const ed = fakeEdit(), e = editor(block('hola món'), ed);
  select(e, pos(e, 'hola'));
  assert.equal(e.run(image('images/ab12.png', 'La foto')), true);
  assert.equal(md(e), 'hola món\n\n![La foto](images/ab12.png)');
  await e.flush();
  assert.deepEqual(ed.calls, [['post', 'a', '![La foto](images/ab12.png)']]);
  assert.equal(imageName('images/ab12.png'), 'ab12.png');
  for (const bad of ['/etc/x.png', '../x.png', 'images/../x.png', 'images/..', 'images/a/b.png', 'images\\..\\x.png', 'C:\\x.png', 'https://exemple.cat/x.png', 'x.png'])
    assert.equal(imageName(bad), null, bad);
});

test('a heading level or paragraph from the toolbar keeps the block\'s alignment', () => {
  const ed = editor(block('hola\n{align=center}'), fakeEdit());
  select(ed, pos(ed, 'hola'));
  ed.run(commands.h2);
  assert.equal(md(ed), '## hola\n{align=center}');
  ed.run(commands.paragraph);
  assert.equal(md(ed), 'hola\n{align=center}');
  assert.deepEqual(ids(ed), ['a']);
});

test('an {align=…} line right after a quote or a list item aligns it and comes back the same', () => {
  for (const s of ['> hola\n{align=center}', '- u\n{align=right}\n- dos', '1. u\n2. dos\n{align=justify}']) {
    const doc = toDoc([{ id: 'x', md: s }]);
    let aligned = 0;
    doc.descendants(n => { if (n.attrs.align) aligned++; });
    assert.equal(aligned, 1, s);
    assert.equal(toMd(doc.content.content), s);
  }
  const ed = editor(block('> hola\n{align=center}'), fakeEdit());
  type(ed, '!', pos(ed, 'hola', true));
  assert.equal(md(ed), '> hola!\n{align=center}', 'editing it does not join the line to the text');
  assert.equal(toDoc([{ id: 'x', md: '> hola\n> {align=center}' }]).firstChild.firstChild.attrs.align, null, 'inside the quote it is text, as in the export');
});

test('nested spans keep every letter\'s style once the block is written back', () => {
  const doc = toDoc([{ id: 'x', md: '[a [b]{color=#c0392b} c]{size=14}' }]);
  const styles = d => { const out = []; d.descendants(n => { if (n.isText) for (const ch of n.text.trim()) out.push([ch, n.marks.map(m => JSON.stringify(m.attrs))]); }); return out; };
  const back = toMd(doc.content.content);
  assert.deepEqual(styles(toDoc([{ id: 'x', md: back }])), styles(doc));
});

test('the horizontal line cannot be inserted inside a table cell', () => {
  const e = editor(block(TABLE, 't'), fakeEdit());
  select(e, pos(e, 'Beta', true));
  assert.equal(e.run(commands.rule), false);
  assert.equal(md(e), TABLE);
});

test('a block the server splits on POST is not lost nor duplicated by later edits and its stream events', async () => {
  const ed = fakeEdit(), e = editor(block('hola'), ed);
  ed.post = async (after, s) => {
    ed.calls.push(['post', after, s]);
    return { status: 200, body: { version: 5, blocks: [{ id: 'p1', md: '- u', version: 5 }, { id: 'p2', md: '- dos', version: 5 }] } };
  };
  select(e, pos(e, 'hola', true));
  e.run(keys.Enter);
  e.dispatch(e.state.tr.insertText('u'));
  e.run(commands.bullets);
  e.run(keys.Enter);
  e.dispatch(e.state.tr.insertText('dos'));
  const saving = e.flush();
  e.apply({ op: 'insert', block: { id: 'p1', md: '- u', version: 5 }, after: 'a', version: 5 });
  e.apply({ op: 'insert', block: { id: 'p2', md: '- dos', version: 5 }, after: 'p1', version: 5 });
  await saving;
  const list = '- u\n\n- dos';
  assert.deepEqual(ed.calls, [['post', 'a', list], ['put', 'p1', list, 5], ['del', 'p2', 5]]);
  assert.equal(md(e), 'hola\n\n' + list);
  assert.deepEqual(ids(e), ['a', 'p1']);
  e.dispatch(e.state.tr.insertText('!'));
  await e.flush();
  assert.deepEqual(ed.calls.at(-1), ['put', 'p1', list + '!', 9]);
});

test('a line break inside a paragraph stays a line break after editing', () => {
  const ed = editor(block('primera\nsegona'), fakeEdit());
  type(ed, '!', pos(ed, 'segona', true));
  assert.equal(md(ed), 'primera\\\nsegona!');
});

test('every toolbar button works in a paragraph, a heading, a list, a quote and a table cell without throwing', () => {
  const buttons = [
    ...Object.entries(commands), ['link', link('https://exemple.cat')],
    ...['color', 'bg'].map(k => [k, style(k, '#2e75b6')]), ['font', style('font', 'Georgia')], ['size', style('size', '14')],
    ...['center', 'right', 'justify', 'left'].map(v => ['align ' + v, align(v)]),
    ['table', table(2, 2)], ['image', image('images/x.png', 'x')],
  ];
  const places = [['paragraph', 'hola món'], ['heading', '## hola món'], ['list', '- hola món'], ['quote', '> hola món'], ['cell', '| hola món | b |\n| --- | --- |\n| c | d |']];
  for (const [where, s] of places) {
    for (const [name, cmd] of buttons) {
      for (const empty of [true, false]) {
        const ed = editor(block(s), fakeEdit());
        if (empty) select(ed, pos(ed, 'món'));
        else select(ed, pos(ed, 'hola'), pos(ed, 'hola', true));
        const before = ed.state.doc;
        assert.doesNotThrow(() => ed.run(cmd), name + ' in a ' + where);
        assert.doesNotThrow(() => { md(ed); current(ed.state); active(ed.state); }, name + ' in a ' + where + ' leaves a document that can be written');
        if (where === 'paragraph' && !empty && !['undo', 'redo', 'paragraph', 'align left', 'rowAdd', 'rowDelete', 'colAdd', 'colDelete'].includes(name))
          assert.ok(!ed.state.doc.eq(before), name + ' changes the paragraph');
      }
    }
  }
});

test('the link, table and image questions are asked in the toolbar, since prompt() does not exist in the desktop app', async () => {
  const src = fs.readFileSync(path.join(__dirname, 'doc-view.js'), 'utf8');
  assert.doesNotMatch(src, /^(?!\s*\/\/).*[^.\w]prompt\(/m);
  const { ask } = require('./doc-view.js');
  class El {
    constructor(doc) { this.ownerDocument = doc; this.children = []; this.attrs = {}; this.parent = null; }
    append(...c) { for (const x of c) { x.parent = this; this.children.push(x); } }
    remove() { if (this.parent) this.parent.children = this.parent.children.filter(x => x !== this); }
    setAttribute(k, v) { this.attrs[k] = v; }
    focus() { this.focused = true; }
    querySelector(sel) { return this.children.find(x => x.className === sel.slice(1)) || null; }
  }
  const doc = { createElement: () => new El(doc) };
  const bar = new El(doc);
  const tx = (ca, en) => en;
  const first = ask(bar, 'Rows × columns', '3 × 3', tx);
  const form = bar.children[0];
  assert.equal(form.className, 'doc-ask');
  const [label, input, ok, no] = form.children;
  assert.equal(label.textContent, 'Rows × columns');
  assert.equal(input.value, '3 × 3');
  assert.ok(input.focused);
  assert.equal(ok.textContent, 'OK');
  input.value = '2 x 4';
  let prevented = false;
  form.onsubmit({ preventDefault: () => { prevented = true; } });
  assert.equal(await first, '2 x 4');
  assert.ok(prevented);
  assert.equal(bar.children.length, 0);

  const second = ask(bar, 'Link address', '', tx);
  bar.children[0].children[3].onclick();
  assert.equal(await second, null);

  const third = ask(bar, 'Image caption', 'foto', tx);
  bar.children[0].children[1].onkeydown({ key: 'Escape', preventDefault() {} });
  assert.equal(await third, null);
  assert.equal(bar.children.length, 0);
});
