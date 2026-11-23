---
title: "Docs (web): el document es veu al panell lateral mentre l'agent l'escriu"
status: done
depends_on: ["057"]
---

## Why

The point of Gregal Docs is watching the document take shape next to the chat. With 057 the agent writes blocks, but the person only sees them by exporting. This task opens the document in a side panel of the web app (and so the desktop app) and updates it live, block by block, without reloading.

## Acceptance criteria

- [x] `GET /api/docs` lists the documents of the session's project (`{items:[{id,title,kind,version,updated}]}`) and `GET /api/docs/{id}` returns the whole document. `GET /api/docs/{id}/stream` is an SSE stream that sends `block` events (`{op: insert|replace|delete, block, after?, version, author}`) for every change after it opens. A Go test: open the stream, change a block through the store, receive the event with the new version; another project's id returns 404.
- [x] When the agent creates or edits a document in a session, the session's event stream emits `doc_changed` `{id, title}`, and the web app opens the document in a side panel next to the chat (or brings it to front if already open). A node test of the panel state: `doc_changed` for a closed document opens it, for the open one does not reopen it.
- [x] The panel renders each block with `md.js` (headings, lists, tables, code) and applies `block` events in place: an inserted block appears in its position, a replaced block changes without the others re-rendering, a deleted block disappears. A node test applies insert/replace/delete events to a rendered document and checks the resulting order and text.
- [x] The panel has a «Descarrega en Word» / «Download as Word» button using the export route of 057, and a list of the project's documents to open another one. Every string in Catalan and English.
- [x] `docs/api-contract.md` documents the three routes and `doc_changed`; a before/after screenshot of the panel in dark and light themes in the task notes.

## Out of scope

- Editing in the panel (059), detaching it into its own window (061), Android (062).

## Notes

- Side panels: `internal/web/app/panels.js` (`browser`, `files` are the closest patterns), SSE helper `sse()` in `internal/web/serve.go:~1015`, session events `internal/events/store.go`.
- An in-memory hub per document id that the store notifies on every change; streams subscribe to it.
- Android does not get this yet: 062 covers it (named here per the parity rule).
- Done: the hub lives in `internal/gdocs` (`Store.Watch(id)` → channel + stop). `change` copies the blocks before the edit, diffs them with the result after writing and notifies inside the per-root lock, so events arrive in version order; Insert/Replace/Delete were not touched. A multi-block insert sends one `insert` per block, chained by `after`, all with the same version. A watcher that falls 64 events behind has its channel closed; the stream ends and the panel reads the document again.
- Delete records no author (057), so a `delete` event has `author: ""`; 059/060 can fill it once Delete gets an author.
- The stream flushes its headers once subscribed; the client opens the stream first, then GETs the document and drops events with `version <= doc.version` (`doc-view.js`). 061 and 059 can reuse `window.gregalDocView.view(box, doc, render)` and `onDocChanged`; the panel shell is `docs-panel.js` (aside in `#workbench`, like `preview.js`).
- `doc_changed` is emitted by the web turn (`motor_web.go`) after a `doc` tool result whose output starts `Created document <id>` / `Document <id>` for create/insert/replace/delete; it goes through the durable log with payload `{id, title}`. Replayed events do not reopen the panel.
- Download uses the existing `a[data-gregal-doc]` handler of `attach.js` with `&name=<title>.docx` for the file name (the server ignores `name`).
- Screenshots (synthetic project, sandboxed server, not committed): `%TEMP%\g058\shots\before-dark.jpg`, `after-dark.jpg`, `before-light.jpg`, `after-light.jpg`. The live update was checked by the Go and node tests, not by a running agent (no model in the sandbox).
- Follow-up: the panel does not follow a session switch; a document of another project stays open until closed (its stream keeps working, but the list and a reopen use the new session's project).
- Follow-up: the panel has a fixed 460 px width; no resize handle like the preview's.
