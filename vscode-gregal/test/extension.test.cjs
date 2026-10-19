const { test } = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const path = require('node:path');
const Module = require('node:module');
const { buildSync } = require('esbuild');

const waitFor = async (check) => {
    const until = Date.now() + 5000;
    while (!check()) { if (Date.now() > until) throw new Error('Condition timed out'); await new Promise(r => setTimeout(r, 10)); }
};
const localize = (text, ...args) => text.replace(/\{(\d+)\}/g, (_, index) => String(args[+index]));

function host(baseUrl) {
    const commands = new Map(), states = [], submissions = [], outgoing = [], builtinCommands = [];
    let messageHandler, provider, view, inlineProvider, codeActionProvider;
    const folder = { name: 'sample', uri: { fsPath: '/workspace/sample', toString: () => 'file:///workspace/sample' } };
    const disposable = () => ({ dispose() {} });
    const context = { subscriptions: [], extensionUri: { fsPath: path.resolve(__dirname, '..'), toString() { return this.fsPath; } },
        secrets: { async get() {}, async store() {}, async delete() {} } };
    const vscode = {
        l10n: { t: localize }, env: { language: 'en', clipboard: { async writeText() {} } },
        Uri: { file: fsPath => ({ fsPath, toString() { return this.fsPath; } }),
            joinPath: (uri, ...parts) => ({ fsPath: path.join(uri.fsPath, ...parts), toString() { return this.fsPath; } }) },
        StatusBarAlignment: { Right: 2 },
        CancellationTokenSource: class { token = {}; cancel() {} dispose() {} },
        InlineCompletionTriggerKind: { Invoke: 0, Automatic: 1 },
        InlineCompletionItem: class { constructor(insertText) { this.insertText = insertText; } },
        CodeActionKind: { QuickFix: 'quickfix' },
        CodeAction: class { constructor(title, kind) { this.title = title; this.kind = kind; } },
        Range: class { constructor(startLine, startCharacter, endLine, endCharacter) { this.start = { line: startLine, character: startCharacter }; this.end = { line: endLine, character: endCharacter }; } },
        DiagnosticSeverity: { Error: 0, Warning: 1, Information: 2 },
        workspace: { isTrusted: true, workspaceFolders: [folder], getWorkspaceFolder: () => folder,
            getConfiguration: () => ({ get: key => key === 'baseUrl' ? baseUrl : key === 'session' ? 'vscode' : true }),
            openTextDocument: async uri => ({ uri, languageId: 'typescript', lineCount: 1, lineAt: () => ({ text: 'const x = 1;' }), getText: () => 'const x = 1;' }),
            asRelativePath: () => 'src/sample.ts', onDidChangeConfiguration: disposable, onDidCloseTextDocument: disposable },
        languages: { getDiagnostics: () => [],
            registerInlineCompletionItemProvider(_selector, value) { inlineProvider = value; return disposable(); },
            registerCodeActionsProvider(_selector, value) { codeActionProvider = value; return disposable(); } },
        commands: { registerCommand(id, fn) { commands.set(id, fn); return disposable(); },
            async getCommands() { return ['git.stageSelectedRanges', 'git.revertSelectedRanges']; },
            async executeCommand(id, ...args) {
                if (id === 'setContext') { states.push(args); return; }
                if (id.startsWith('git.')) { builtinCommands.push(id); return; }
                if (id === 'gregal.chat.focus' && !view) {
                    view = { webview: { cspSource: 'vscode-webview:', options: {}, html: '',
                        asWebviewUri: uri => ({ toString: () => 'vscode-webview:/' + path.basename(uri.fsPath) }),
                        postMessage: message => { outgoing.push(message); return Promise.resolve(true); },
                        onDidReceiveMessage(fn) { messageHandler = fn; return disposable(); } }, onDidDispose: disposable };
                    provider.resolveWebviewView(view);
                }
                return commands.get(id)?.(...args);
            } },
        window: { activeTextEditor: { selection: { isEmpty: false, start: { line: 1 }, end: { line: 3 } },
            document: { uri: folder.uri, languageId: 'typescript', getText: () => 'selected source only' } },
            createOutputChannel: () => ({ append() {}, appendLine() {}, dispose() {} }),
            createStatusBarItem: () => ({ show() {}, dispose() {} }),
            registerWebviewViewProvider(_id, value) { provider = value; return disposable(); },
            async showInformationMessage() {}, async showErrorMessage() {}, async showWarningMessage() { return 'Discard lines'; },
            async showQuickPick(items) { return items[0]; } },
    };
    const bundle = buildSync({ entryPoints: [path.resolve(__dirname, '../src/extension.ts')], bundle: true, platform: 'node', format: 'cjs', external: ['vscode'], write: false }).outputFiles[0].text;
    const original = Module._load;
    Module._load = function(id, ...args) { return id === 'vscode' ? vscode : original.call(this, id, ...args); };
    const loaded = new Module(path.resolve(__dirname, 'extension-fixture.cjs'), module);
    loaded.filename = path.resolve(__dirname, 'extension-fixture.cjs'); loaded.paths = module.paths;
    try { loaded._compile(bundle, loaded.filename); } finally { Module._load = original; }
    loaded.exports.activate(context);
    return { vscode, commands, states, submissions, outgoing, builtinCommands, get view() { return view; },
        get inlineProvider() { return inlineProvider; }, get codeActionProvider() { return codeActionProvider; },
        send(message) { messageHandler(message); }, dispose: () => loaded.exports.deactivate() };
}

test('native sidebar routes a single run to the editor workspace with explicit context', async () => {
    const received = [];
    let eventsAllowed = false;
    const server = http.createServer((req, res) => {
        let raw = ''; req.on('data', chunk => raw += chunk);
        req.on('end', () => {
            const body = raw ? JSON.parse(raw) : {};
            received.push({ url: req.url, session: req.headers['x-gregal-session'], body });
            res.setHeader('content-type', 'application/json');
            if (req.url === '/api/health') res.end(JSON.stringify({ status: 'ok', protocol: 'gregal.v1', instance_id: 'fixture', capabilities: { runs: true, events: true, durable_events: true, interactive_events: true } }));
            else if (req.url === '/api/state') res.end(JSON.stringify({ model: 'fixture-model' }));
            else if (req.url === '/api/v2/runs') { res.statusCode = 202; res.end(JSON.stringify({ run: { id: 7, state: 'running' }, cursor: 0 })); }
            else if (req.url.startsWith('/api/v2/events')) res.end(JSON.stringify({ next: eventsAllowed ? 3 : 0, events: eventsAllowed ? [
                { id: 1, run_id: 7, kind: 'token', payload: { text: 'Done.' }, text: 'Done.' },
                { id: 2, run_id: 7, kind: 'done', text: 'Done.' },
                { id: 3, run_id: 7, kind: 'run_completed', text: 'Run finished' },
            ] : [] }));
            else res.end(JSON.stringify({ run: { id: 7, state: 'running' } }));
        });
    });
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    const editor = host('http://127.0.0.1:' + server.address().port);
    try {
        await editor.commands.get('gregal.openChat')();
        editor.send({ type: 'ready' });
        assert.match(editor.view.webview.html, /default-src 'none'/);
        assert.match(editor.view.webview.html, /script-src 'nonce-/);
        assert.match(editor.view.webview.html, /chat\.css/);
        editor.send({ type: 'send', text: 'Review the project', mode: 'inspect', includeContext: false });
        editor.send({ type: 'send', text: 'Must not submit twice', mode: 'code' });
        await waitFor(() => received.some(r => r.url === '/api/v2/runs'));
        const submitted = received.filter(r => r.url === '/api/v2/runs');
        assert.equal(submitted.length, 1);
        assert.equal(submitted[0].body.workspace, '/workspace/sample');
        assert.equal(submitted[0].body.mode, 'inspect');
        assert.equal(submitted[0].body.task, 'Review the project');
        assert.match(submitted[0].session, /^vscode-[a-f0-9]{12}$/);
        await editor.commands.get('gregal.rewind')();
        assert.equal(received.some(r => r.url === '/api/checkpoints'), false, 'undo must not overlap an active turn');
        eventsAllowed = true;
        await waitFor(() => editor.states.filter(s => s[0] === 'gregal.busy').at(-1)?.[1] === false);
        await editor.commands.get('gregal.ask')();
        assert.ok(editor.outgoing.some(m => m.type === 'state' && m.context?.label === 'src/sample.ts:2-4'));
        editor.send({ type: 'send', text: 'Explain this selection', mode: 'inspect', includeContext: true });
        await waitFor(() => received.filter(r => r.url === '/api/v2/runs').length === 2);
        assert.match(received.filter(r => r.url === '/api/v2/runs')[1].body.task, /selected source only/);
    } finally { editor.dispose(); await new Promise(resolve => server.close(resolve)); }
});

test('saved conversation picker restores transcript on the project session', async () => {
    const received = [];
    const server = http.createServer((req, res) => {
        let raw = ''; req.on('data', chunk => raw += chunk);
        req.on('end', () => {
            received.push({ url: req.url, session: req.headers['x-gregal-session'], body: raw ? JSON.parse(raw) : {} });
            res.setHeader('content-type', 'application/json');
            if (req.url === '/api/sessions') res.end(JSON.stringify([
                { name: 'saved-1', title: 'Fix login', msgs: 2, at: '19-10 10:00', workspace: '/workspace/sample' },
            ]));
            else if (req.url === '/api/resume') res.end(JSON.stringify({ name: 'saved-1', title: 'Fix login', workspace: '/workspace/sample', transcript: [
                { role: 'user', content: 'Fix login' }, { role: 'assistant', content: 'I found the issue.' },
            ] }));
            else res.end('{}');
        });
    });
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    const editor = host('http://127.0.0.1:' + server.address().port);
    try {
        await editor.commands.get('gregal.openChat')();
        editor.send({ type: 'ready' });
        editor.send({ type: 'savedConversations' });
        await waitFor(() => received.some(request => request.url === '/api/resume'));
        const listed = received.find(request => request.url === '/api/sessions');
        const resumed = received.find(request => request.url === '/api/resume');
        assert.ok(listed);
        assert.ok(resumed);
        assert.equal(listed.session, resumed.session);
        assert.deepEqual(resumed.body, { name: 'saved-1' });
        // The server records /api/resume before its response reaches the webview.
        // Wait for the transcript messages too, or this assertion races on Linux.
        await waitFor(() => editor.outgoing.filter(message => message.type === 'message').length === 2);
        const transcript = editor.outgoing.filter(message => message.type === 'message');
        assert.deepEqual(transcript.map(message => [message.role, message.text]), [
            ['user', 'Fix login'], ['assistant', 'I found the issue.'],
        ]);
    } finally { await editor.dispose(); server.close(); }
});

test('selected diff actions use native Git stage and revert commands', async () => {
    const editor = host('http://127.0.0.1:1');
    try {
        await editor.commands.get('gregal.acceptSelectedHunk')();
        await editor.commands.get('gregal.rejectSelectedHunk')();
        assert.deepEqual(editor.builtinCommands, ['git.stageSelectedRanges', 'git.revertSelectedRanges']);
    } finally { await editor.dispose(); }
});

test('inline completion calls the isolated backend endpoint and yields one suggestion', async () => {
    const received = [];
    const server = http.createServer((req, res) => {
        let raw = ''; req.on('data', chunk => raw += chunk);
        req.on('end', () => {
            received.push({ url: req.url, session: req.headers['x-gregal-session'], body: raw ? JSON.parse(raw) : {} });
            res.setHeader('content-type', 'application/json');
            res.end(JSON.stringify({ completion: ' + 1' }));
        });
    });
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    const editor = host('http://127.0.0.1:' + server.address().port);
    try {
        const folder = editor.vscode.workspace.workspaceFolders[0];
        const document = { uri: folder.uri, languageId: 'typescript', isClosed: false,
            getText: () => 'const total = value', offsetAt: () => 20 };
        editor.vscode.window.activeTextEditor = { document, selection: { isEmpty: true } };
        const token = { isCancellationRequested: false, onCancellationRequested: () => ({ dispose() {} }) };
        const result = await editor.inlineProvider.provideInlineCompletionItems(document, {},
            { triggerKind: editor.vscode.InlineCompletionTriggerKind.Invoke }, token);
        assert.equal(result.items.length, 1);
        assert.equal(result.items[0].insertText, ' + 1');
        assert.equal(received[0].url, '/api/vscode/completion');
        assert.equal(received[0].body.language, 'typescript');
        assert.equal(received[0].body.prefix, 'const total = value');
        assert.equal(received[0].session, 'vscode-' + require('node:crypto').createHash('sha256').update(folder.uri.toString()).digest('hex').slice(0, 12));
    } finally { await editor.dispose(); server.close(); }
});

test('diagnostic quick fixes offer targeted fix and explain actions', async () => {
    const editor = host('http://127.0.0.1:1');
    const diagnostic = { message: 'Unexpected token', severity: editor.vscode.DiagnosticSeverity.Error,
        range: { start: { line: 2, character: 4 }, end: { line: 2, character: 9 } } };
    try {
        const actions = editor.codeActionProvider.provideCodeActions({}, {}, { diagnostics: [diagnostic] });
        assert.deepEqual(actions.map(action => action.command.command), ['gregal.fixDiagnostic', 'gregal.explainDiagnostic']);
        assert.equal(actions[0].isPreferred, true);
        assert.deepEqual(actions[0].diagnostics, [diagnostic]);
    } finally { await editor.dispose(); }
});

test('untrusted workspace does not send tasks to a backend', async () => {
    const editor = host('http://127.0.0.1:1');
    try {
        editor.vscode.workspace.isTrusted = false;
        await editor.commands.get('gregal.openChat')();
        editor.send({ type: 'send', text: 'change a file', mode: 'code' });
        await new Promise(resolve => setTimeout(resolve, 20));
        assert.equal(editor.states.filter(s => s[0] === 'gregal.busy' && s[1]).length, 0);
    } finally { editor.dispose(); }
});
