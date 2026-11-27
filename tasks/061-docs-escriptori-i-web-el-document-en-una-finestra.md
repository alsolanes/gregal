---
title: "Docs (escriptori i web): el document en una finestra pròpia"
status: done
depends_on: ["059"]
---

## Why

A long document needs more room than a side panel: on a second screen, next to the chat, or full width. The person should be able to unpin the panel into its own window and keep both live.

## Acceptance criteria

- [x] The web app serves `/app/doc/{id}`: only the document editor of 058–059, full page, with the same live updates and editing. A node or Go test that the route serves the editor page for an existing id and 404s an unknown one.
- [x] The panel has a «Obre en una finestra» / «Open in a window» button. In the desktop app it opens a new Electron window on `/app/doc/{id}` (one per document: pressing it again focuses the existing one) through an IPC handler that only accepts calls from the app's own pages, and the panel closes; in the browser it uses `window.open`. A test of the desktop handler's rules (one window per id, untrusted sender refused).
- [x] An edit in the window appears in the chat window's panel if it is open, and the other way round, through the same stream. Checked by hand in the desktop app; a screenshot of both side by side in the task notes.
- [x] Every string in Catalan and English.

## Out of scope

- Remembering window positions between launches. Android (062, 063).

## Notes

- `desktop/main.js` (`new BrowserWindow` at ~630, `trusted(event)` guard used by the `ipcMain.handle` calls, `mascot-window.js` as an example of a second window), preload exposes the IPC.
- Docs routes and auth must work for a window opened without a chat session: the document id plus the project, not `X-Gregal-Session`, if 058 depends on it.
- Done: `GET /app/doc/{id}?session=<sid>` (`handleDocPage`, docs_api.go) serves a small page that loads `doc-page.js`, which defines the minimal `window.gregal` (`api` with the token and `X-Gregal-Session` from the query, `sys`) and imports `docs-panel.js` unchanged; the page CSS stretches the panel to full width and hides its header. Same view, same stream, same block editing.
- Project: the window carries the chat session id in `?session=`, which `sessionID` already reads, so it uses that session's project. If that session is gone, the backend creates it again with the default project, and the page then says the document was not found.
- 404: only checked without accounts (local / desktop backend). With a token or accounts the navigation carries no token and the session owner is unknown, so the page is served and its `/api/docs/{id}` call reports the 401/404 instead. In that case the route resolves no session at all, so an anonymous request cannot create sessions.
- Desktop: `desktop/doc-window.js` keeps one window per document id (pressing again restores and focuses it); `gregal:open-doc` in `main.js` only accepts the trusted main page; the doc windows use `preload.js` and `gregal:connection` also gives the token to a doc window's main frame on the backend's origin (needed for a remote `GREGAL_URL`). Links in a doc window open in the system browser; the window never navigates. Closing the main window closes the document windows, so the app can still quit.
- Checked in the browser (sandboxed server, synthetic project, not committed): an edit in the `/app/doc/{id}` tab appeared in the main page's panel and the other way round; «Open in a window» called `window.open('/app/doc/<id>?session=default', 'gregal-doc-<id>')` and closed the panel.
- NOT checked: the third criterion's hand check and side-by-side screenshot in the desktop (Electron) app. I could not drive the desktop app from this session, so that box stays unticked; the Electron part is covered only by `desktop/doc-window.test.js` and `desktop/ipc.test.js`.
- Follow-up: the doc page hides the «Download as Word» link (the `attach.js` handler that adds the token is not loaded there).
- Follow-up: `docs/api-contract.md` does not list `/app/doc/{id}` (the contract test only covers `/api/` routes).
- Packaging bug found in the hand check on 2026-11-23: the packaged app crashed at startup with «Cannot find module './doc-window'» because `desktop/package.json` `build.files` is an explicit list and did not ship `doc-window.js`. Fixed by adding it, with `desktop/release.test.js` now checking that every local module the shipped scripts require is in `build.files`. Criterion 3 stays open until the hand check passes on a build with this fix.
- Also found then: `desktop/navigation.test.js` still asserted the old token expression of `gregal:connection` (only the trusted page), so it failed after this task; `scripts/check.py` does not run the desktop suites, so nobody saw it. Updated to the new rule (trusted page or a document window on the backend's origin, tested in `ipc.test.js`): a requirement change, not a weaker test.
- Follow-up: `scripts/check.py` should run `node --test desktop/*.test.js` when `desktop/` changes.
- Hand check passed on 2026-11-24 by the user in the portable built from main 66004983: the window opens, and edits sync between the window and the panel both ways.
