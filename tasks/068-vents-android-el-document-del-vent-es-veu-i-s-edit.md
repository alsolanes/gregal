---
title: "Vents (Android): el document del vent es veu i s'edita"
status: done
depends_on: ["066"]
---

## Why

Every feature ships on Android too, and Vents is mostly used from the phone. A document a wind writes should open from the wind's chat on Android, live and editable, like the Codi documents of 062–063.

## Acceptance criteria

- [x] When a wind chat receives `doc_changed` with `scope:"mine"`, the message shows the document card, which opens the existing document screen for that document. A unit test of the wind stream event parsing.
- [x] The document screen opened from a wind uses `scope=mine` for the list, the read, the stream, the block edits and the Word export; opened from a Codi chat it never does. A unit test of the URLs built in both cases.
- [x] Every string in Catalan and English in `Strings.kt`, edited by hand.

## Out of scope

- Changes to the document screen's behaviour beyond the scope (062–063 keep their rules).

## Notes

- Wind chat on Android: `Winds.kt`, `GregalApi.kt`, `GregalViewModel.kt`; documents: `Docs.kt`, `DocScreen.kt`, `LiveDocTest.kt`.
- Run gradle tests with `ANDROID_HOME` set; do not run `regen-strings.py`.
- Done: what was missing was only the wind side. `parseWindStreamEvent` ignored `doc_changed`; now it gives `WindStreamEvent.DocChanged(DocRef)` through `DocsJson.changed`, which keeps the event's `scope`. `handleWindAskEvent` (the `else` of both wind streams, send and «continua») adds one `PetMsg(kind = "doc", doc = …)` per document to that wind's chat, before the live bubble if there is one, and `WindConversation` (`CompanyScreen.kt`, not `Winds.kt`) draws it with the Codi `DocCard`, which calls `vm.openDoc`. Tests: `WindDocCardTest` (parsing, one card per document, card before the live bubble, tapping the card opens `Screen.Docs` with the `mine` ref).
- Review fix: «continua» merges its text into the last `pet` message; it now skips `kind == "doc"`, or a card added in the previous turn swallowed the continuation (`WindDocCardTest.aContinuationIsMergedIntoTheAnswerNotIntoTheCard`, against a local fake server).
- The second criterion was already met by 071: `docPath`, `docExportPath` and `LiveDoc.saveRequest` take the scope from the ref, and `followDoc` copies it to the document it reads. `LiveDocTest.aWindDocumentUsesMyScopeAndACodiDocumentNever` proves it for a wind ref and a Codi ref side by side; it passes without this task's code. "The list" is read as the list where the open document is marked: since 071 the screen always shows «Els meus documents» (`/api/docs?scope=mine`) and, in a Codi chat with a project, «Aquest projecte» (`/api/docs`), whoever opened it; the wind ref matches an item of the `mine` list and the Codi ref one of the project list.
- No new strings: the card reuses `docs.card`, already in Catalan and English.
- Whole Android unit suite: 992 tests, 0 failures. Not tried on a device or emulator with a running server.
- Follow-up: the card lives only in the phone's chat state; reloading the wind's history from the server drops it, because the server history does not carry `doc_changed`, and the offline cache (`OfflineLogic.encodeChat`) keeps it as a plain bubble with the title, without the `DocRef`.
- Follow-up: a rotllana stream that receives `doc_changed` shows no card (`handleWindAskEvent` skips it with `rotllana = true`).
