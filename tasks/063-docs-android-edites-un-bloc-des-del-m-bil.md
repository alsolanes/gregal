---
title: "Docs (Android): edites un bloc des del mòbil"
status: done
depends_on: ["059", "062"]
---

## Why

The person should be able to fix a sentence or add a paragraph from the phone while the agent keeps writing, the same as on the web.

## Acceptance criteria

- [x] Tapping a block in the document screen edits its markdown in place; «Desa» / «Save» sends `PUT /api/docs/{id}/blocks/{block}` with the block's version, «Afegeix un bloc» / «Add a block» inserts below, and saving an empty block deletes it. A unit test of the request each action builds.
- [x] A 409 shows the current text and «L'agent ha canviat aquest bloc» / «The agent changed this block», keeping the person's text. A unit test of the conflict state.
- [x] While a block is being edited, `block` events for that block do not replace the editor; others still apply. A unit test.
- [x] Every string in Catalan and English in `Strings.kt`, edited by hand.

## Out of scope

- Rich formatting toolbar.

## Notes

- Same editing rules as the web panel in 059; reuse its strings where the wording is the same.
- Done: the editing state is pure, in `Docs.kt`: `LiveDoc.edit` (`DocEdit`, `@Transient`) with `open`, `openNew`, `typed`, `saveRequest()` (null = nothing to send: untouched block or empty new one), `skipped()` and `saved(code, body)`. `DocBlock` now carries its `version`, which is the `base_version` sent. `LiveDoc.apply` ignores an `insert` for a block already in the list (the POST answer is applied first, the stream copy is dropped) and keeps the block being edited when a `delete` for it arrives, so the editor does not vanish; a `replace` for it updates the known block but not the editor's text or base, so saving gets the 409.
- `GregalApi.sendDoc` returns code and body instead of throwing, because the 409 carries the current block. A network failure keeps the text and shows `docs.saveError`.
- UI (`DocScreen.kt`): tapping a block opens an editor in its place with «Desa» and «Afegeix un bloc» (saves, then opens a new editor below the saved or inserted block). While an editor is open, tapping another block does nothing (on the web, leaving the text area saves). After a 409 the current text is shown above with «L’agent ha canviat aquest bloc», and «Desa» overwrites against that version.
- Tested with `LiveDocTest` and the whole Android unit suite (978 tests, 0 failures); not tried on a device or emulator with a running agent.
- Follow-up: no way to close an editor without saving (no cancel like Esc on the web); an untouched block closes on «Desa».
- Follow-up: a stream reconnect reads the document again and drops an open editor and its text, as on the web.
- Follow-up: an empty document has no block to tap, so nothing can be added to it from Android (the web has the same gap).
