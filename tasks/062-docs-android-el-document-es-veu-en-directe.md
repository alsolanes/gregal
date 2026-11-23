---
title: "Docs (Android): el document es veu en directe"
status: todo
depends_on: ["058"]
---

## Why

Every feature ships on Android too. On the phone there is no room for a side panel, so a document opens in its own screen from the chat and updates live while the agent writes it.

## Acceptance criteria

- [ ] When a chat receives `doc_changed`, the message shows a card with the document title that opens a document screen; the screen also lists the project's documents (`GET /api/docs`). A unit test of the event parsing and of the list parsing.
- [ ] The document screen renders the blocks with the app's markdown rendering and applies `block` events from `/api/docs/{id}/stream` in place (insert, replace, delete), reconnecting when the stream drops. A unit test of applying a sequence of events to the block list, including one for an unknown block.
- [ ] The screen has «Descarrega en Word» / «Download as Word» using the export route, saved and opened like other documents the app downloads.
- [ ] Every string in Catalan and English in `Strings.kt`, edited by hand.

## Out of scope

- Editing on Android (063).

## Notes

- `GregalApi.kt` (existing SSE consumers), `GregalViewModel.kt`, `OfficeScreen.kt` / `PreviewScreen.kt` for downloads and document screens, `Screens.kt` for navigation.
- Run gradle tests with `ANDROID_HOME` set; do not run `regen-strings.py`.
