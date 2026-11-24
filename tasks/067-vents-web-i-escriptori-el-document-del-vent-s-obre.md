---
title: "Vents (web i escriptori): el document del vent s'obre al panell"
status: done
depends_on: ["066"]
---

## Why

With 066 a wind writes live documents, but the Vents chat does not open them. The person should see the document next to the wind's chat, edit it, and unpin it into its own window, as in Codi.

## Acceptance criteria

- [x] When the wind chat receives `doc_changed` with `scope:"mine"`, the web app opens the document in the side panel next to the wind's chat (or brings it forward if already open), the same panel as 058. A node test of the event handling in the wind chat.
- [x] The panel opened for a wind document reads, streams, edits (059) and exports with `scope=mine` on every call, and its list of documents shows the person's wind documents. A node test that a wind-scoped panel builds every URL with `scope=mine`, and a Codi panel never does.
- [x] «Obre en una finestra» on a wind document opens `/app/doc/{id}?scope=mine` (desktop window and browser), and edits sync between the window and the panel. A test of the URL; checked by hand in a desktop build, with that build's commit in the notes.
- [x] Every string in Catalan and English.

## Out of scope

- Android (068). Changing how the agent chooses between `doc` and `office_create` (069).

## Notes

- Panel and view: `internal/web/app/docs-panel.js`, `doc-view.js`, page `doc-page.js`; desktop window `desktop/doc-window.js` (it must stay in `desktop/package.json` `build.files`, which `release.test.js` checks).
- Wind chat clients: `internal/web/app/winds.js`, `winds-workspace.js`; check the panel's container exists in the Vents view, not only in Codi's `#workbench`.
- The hand check runs in a packaged desktop build (portable), not in the dev server: 061 only failed there.
- Done: the wind chat (`streamChat` in `winds-workspace.js`, both the single-wind and the group stream) passes `doc_changed` to `window.gregalDocs.changed`, which now opens `open(d.id, d.scope)`. `onDocChanged` (doc-view.js) compares the scope too, so a project document and a personal one with the same id are not the same document. The panel already lives in `#workbench`, and the Vents page (`#ventsPage`) is inside `#conversationPane` there, so it opens next to the wind's chat.
- «Obre en una finestra» shows again for «Els meus documents»: the browser opens `/app/doc/{id}?session=…&scope=mine`; the desktop passes `scope` through `openDocWindow(id, session, scope)` (preload.js, only `mine` or empty) → `gregal:open-doc` (main.js) → `doc-window.js`, which refuses any other scope and loads the same URL. `doc-page.js` reads `scope` from the query and opens the panel with it. The server side needed nothing: `docStore` already honours `scope=mine` for `/app/doc/{id}` and every docs route (066).
- The panel's list is labelled «Els meus documents» / "My documents" for a personal document (it said «Documents del projecte»).
- Tests: `internal/web/app/docs-panel.test.cjs` loads the real `docs-panel.js` with a fake DOM and API; `winds-start.test.cjs` runs a wind chat stream with `doc_changed`; `doc-view.test.cjs`; `desktop/doc-window.test.js`, `desktop/ipc.test.js`. `node --test desktop/*.test.js`: 76 pass (after `npm ci` in desktop/).
- NOT checked: criterion 3's hand check in a packaged desktop build (window opens, edits sync between window and panel). A portable was built from commit 60179b84 (`npm run dist-win -- --config.directories.output=../output/t067`, output/ is ignored) and not launched, because the user's desktop app was running and the single-instance lock would collide. `npx @electron/asar list` of its app.asar contains every local module main.js requires (recursively: quick-note, server-manager, window-state, preferences, navigation, updater, browser-bridge, mascot-window, doc-window, chrome-extension, word-addin, startup, package.json) plus preload.js, and the packaged doc-window.js has the scope change. The user must run the hand check: open a wind document from a wind chat, «Obre en una finestra», edit in both.
- Follow-up: `doc-window.js` and the browser window name key windows by document id only; a project and a personal document with the same id would share a window (ids are random, so unlikely).
- Follow-up: no test covers `doc-page.js` itself (scope from the query to `open()` and the title fetch); only the hand check does.
- Checked on 2026-11-24 in the packaged desktop build of main fe7863fc (`win-unpacked/Gregal.exe`, driven with Playwright `_electron` against a sandboxed server, isolated `--user-data-dir`): a `doc_changed` with `scope:"mine"` opened the panel, «Obre en una finestra» opened `/app/doc/{id}?session=default&scope=mine`, and with the focus moving between windows an edit in the panel reached the window and a later edit in the window reached the panel, with no 409. Without a blur (automation that keeps both windows focused), an event for the block under the cursor waits by design (059) and the next edit there conflicts.
