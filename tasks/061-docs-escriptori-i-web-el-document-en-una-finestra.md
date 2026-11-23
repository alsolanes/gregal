---
title: "Docs (escriptori i web): el document en una finestra pròpia"
status: todo
depends_on: ["059"]
---

## Why

A long document needs more room than a side panel: on a second screen, next to the chat, or full width. The person should be able to unpin the panel into its own window and keep both live.

## Acceptance criteria

- [ ] The web app serves `/app/doc/{id}`: only the document editor of 058–059, full page, with the same live updates and editing. A node or Go test that the route serves the editor page for an existing id and 404s an unknown one.
- [ ] The panel has a «Obre en una finestra» / «Open in a window» button. In the desktop app it opens a new Electron window on `/app/doc/{id}` (one per document: pressing it again focuses the existing one) through an IPC handler that only accepts calls from the app's own pages, and the panel closes; in the browser it uses `window.open`. A test of the desktop handler's rules (one window per id, untrusted sender refused).
- [ ] An edit in the window appears in the chat window's panel if it is open, and the other way round, through the same stream. Checked by hand in the desktop app; a screenshot of both side by side in the task notes.
- [ ] Every string in Catalan and English.

## Out of scope

- Remembering window positions between launches. Android (062, 063).

## Notes

- `desktop/main.js` (`new BrowserWindow` at ~630, `trusted(event)` guard used by the `ipcMain.handle` calls, `mascot-window.js` as an example of a second window), preload exposes the IPC.
- Docs routes and auth must work for a window opened without a chat session: the document id plus the project, not `X-Gregal-Session`, if 058 depends on it.
