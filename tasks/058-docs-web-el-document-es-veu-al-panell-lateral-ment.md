---
title: "Docs (web): el document es veu al panell lateral mentre l'agent l'escriu"
status: todo
depends_on: ["057"]
---

## Why

The point of Gregal Docs is watching the document take shape next to the chat. With 057 the agent writes blocks, but the person only sees them by exporting. This task opens the document in a side panel of the web app (and so the desktop app) and updates it live, block by block, without reloading.

## Acceptance criteria

- [ ] `GET /api/docs` lists the documents of the session's project (`{items:[{id,title,kind,version,updated}]}`) and `GET /api/docs/{id}` returns the whole document. `GET /api/docs/{id}/stream` is an SSE stream that sends `block` events (`{op: insert|replace|delete, block, after?, version, author}`) for every change after it opens. A Go test: open the stream, change a block through the store, receive the event with the new version; another project's id returns 404.
- [ ] When the agent creates or edits a document in a session, the session's event stream emits `doc_changed` `{id, title}`, and the web app opens the document in a side panel next to the chat (or brings it to front if already open). A node test of the panel state: `doc_changed` for a closed document opens it, for the open one does not reopen it.
- [ ] The panel renders each block with `md.js` (headings, lists, tables, code) and applies `block` events in place: an inserted block appears in its position, a replaced block changes without the others re-rendering, a deleted block disappears. A node test applies insert/replace/delete events to a rendered document and checks the resulting order and text.
- [ ] The panel has a «Descarrega en Word» / «Download as Word» button using the export route of 057, and a list of the project's documents to open another one. Every string in Catalan and English.
- [ ] `docs/api-contract.md` documents the three routes and `doc_changed`; a before/after screenshot of the panel in dark and light themes in the task notes.

## Out of scope

- Editing in the panel (059), detaching it into its own window (061), Android (062).

## Notes

- Side panels: `internal/web/app/panels.js` (`browser`, `files` are the closest patterns), SSE helper `sse()` in `internal/web/serve.go:~1015`, session events `internal/events/store.go`.
- An in-memory hub per document id that the store notifies on every change; streams subscribe to it.
- Android does not get this yet: 062 covers it (named here per the parity rule).
