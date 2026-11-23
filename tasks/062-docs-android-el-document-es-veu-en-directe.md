---
title: "Docs (Android): el document es veu en directe"
status: done
depends_on: ["058"]
---

## Why

Every feature ships on Android too. On the phone there is no room for a side panel, so a document opens in its own screen from the chat and updates live while the agent writes it.

## Acceptance criteria

- [x] When a chat receives `doc_changed`, the message shows a card with the document title that opens a document screen; the screen also lists the project's documents (`GET /api/docs`). A unit test of the event parsing and of the list parsing.
- [x] The document screen renders the blocks with the app's markdown rendering and applies `block` events from `/api/docs/{id}/stream` in place (insert, replace, delete), reconnecting when the stream drops. A unit test of applying a sequence of events to the block list, including one for an unknown block.
- [x] The screen has «Descarrega en Word» / «Download as Word» using the export route, saved and opened like other documents the app downloads.
- [x] Every string in Catalan and English in `Strings.kt`, edited by hand.

## Out of scope

- Editing on Android (063).

## Notes

- `GregalApi.kt` (existing SSE consumers), `GregalViewModel.kt`, `OfficeScreen.kt` / `PreviewScreen.kt` for downloads and document screens, `Screens.kt` for navigation.
- Run gradle tests with `ANDROID_HOME` set; do not run `regen-strings.py`.
- Done: the pure part is `Docs.kt` (`DocsJson` parses `doc_changed`, `/api/docs`, a document and a `block` event; `LiveDoc.apply` applies one event and returns the new state, so 063 can add local edits on top). Like `doc-view.js`, events with `version <= base` (the version the document was read at) are dropped, not `<= version`, because a multi-block insert sends several events with the same version. Replace/delete of an unknown block change nothing; an insert after an unknown block goes to the end, as on the web.
- `GregalApi.streamGet` got an `onOpen` callback that runs once the headers arrive: `followDoc` reads the document there, so no event is lost between the GET and the stream. A network failure or a stream the server closes reconnects after 2 s and reads the document again; any other error (an HTTP error such as 404, a body that is not a document) stops and shows `docs.error`. Leaving the screen cancels the coroutine; the OkHttp thread notices at the next event or the 15 s `ping`.
- The chat shows one card per document (`ChatMsg.kind == "doc"`); later `doc_changed` for the same document do not add more cards. The screen is `Screen.Docs` (`DocScreen.kt`), reached from the card; it has no drawer entry.
- «Descarrega en Word» uses `downloadServerDoc` with `/api/docs/{id}/export?format=docx&name=<title>.docx`; `isServerDocLink` now accepts `/api/docs/{id}/export`, so the link the agent writes after `create` also downloads with the app's key instead of opening the browser. Like the other server documents, it is saved to Downloads with a notice, not opened.
- Tested with `LiveDocTest` and the whole Android unit suite (971 tests, 0 failures); not tried on a device or emulator with a running agent.
- Follow-up: no drawer entry for the project's documents; the list is only reachable from a card in the chat.
- Follow-up: a 502/503 while the server or tunnel restarts also stops following (it is an `ApiException`); retrying on 5xx would be better.
- Follow-up: the screen loads the document list once on entry; a document created while it is open appears after reopening it.
- Follow-up: leaving the screen does not cancel the OkHttp call; it ends at the next event or `ping` (up to 15 s).

