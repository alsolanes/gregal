---
title: "Web: els diàlegs de text funcionen a l'app d'escriptori"
status: done
depends_on: []
---

## Why

`prompt()` does not exist in Electron (the desktop app and the app's browser pane): it throws «prompt() is not supported», so every web button that asks for a line of text fails silently there. Gregal Docs was fixed in fe7863fc; the same bug remains in the folder-as-context, Try N ways, Kanban rename, new worktree session and MCP prompt arguments.

## Acceptance criteria

- [x] A shared helper `window.gregalAsk(label, value)` (`internal/web/app/ask.js`) asks for a line of text in an in-page modal with the label, an input holding `value`, OK and Cancel; it resolves with the text on OK/Enter and with `null` on Cancel or Escape, and removes itself. A node behaviour test.
- [x] `attach.js` (folder path), `caps.js` (attempts and models), `kanban.js` (rename) and `panels.js` (worktree name, MCP prompt arguments) use it, and none of them calls `prompt(` outside comments. A node test per file.
- [x] OK and Cancel are in English and Catalan in `i18n.js`.
- [x] Each of those buttons works in the web app, checked by hand in a sandboxed server in the browser pane.

## Out of scope

- `alert()` and `confirm()`: Electron implements both, so they keep working.
- Gregal Docs (`doc-view.js`), already fixed with its own toolbar row.

## Notes

- Shared helper instead of copying `doc-view.js`'s `ask()`: five call sites in four files, and none of them has a toolbar to put a row in, so it is a modal `<dialog>` (`.ask-dialog` CSS in `index.html`). `ask.js` is a classic script so both the ES modules and the classic `caps.js` reach it through `window.gregalAsk`; under node it exports `{ ask }` for the test. Docs keeps its own toolbar row.
- `alert()` and `confirm()`: Electron implements both (only `prompt()` is unsupported); they were not exercised by hand here. In the app's browser pane `prompt()` throws «prompt() is not supported.», which reproduced the bug.
- Checked by hand in a sandboxed server (fake model, isolated HOME/data/auth/memory) in the app's browser pane: new worktree session (OK creates it, Escape cancels), external folder as context, Try N ways (both questions, then two attempts start), Kanban rename by double-clicking the title (the server has the new title), and an MCP prompt with two arguments (a throwaway stdio MCP server in the sandbox; the composer gets the expanded text).
- Escape in the dialog stops there: the page has its own Escape listeners on `document` (the Kanban window would close under a cancelled rename). Checked in the pane too.
- Tests: `internal/web/app/ask.test.cjs`.
