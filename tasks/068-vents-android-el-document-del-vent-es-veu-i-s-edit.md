---
title: "Vents (Android): el document del vent es veu i s'edita"
status: todo
depends_on: ["066"]
---

## Why

Every feature ships on Android too, and Vents is mostly used from the phone. A document a wind writes should open from the wind's chat on Android, live and editable, like the Codi documents of 062–063.

## Acceptance criteria

- [ ] When a wind chat receives `doc_changed` with `scope:"mine"`, the message shows the document card, which opens the existing document screen for that document. A unit test of the wind stream event parsing.
- [ ] The document screen opened from a wind uses `scope=mine` for the list, the read, the stream, the block edits and the Word export; opened from a Codi chat it never does. A unit test of the URLs built in both cases.
- [ ] Every string in Catalan and English in `Strings.kt`, edited by hand.

## Out of scope

- Changes to the document screen's behaviour beyond the scope (062–063 keep their rules).

## Notes

- Wind chat on Android: `Winds.kt`, `GregalApi.kt`, `GregalViewModel.kt`; documents: `Docs.kt`, `DocScreen.kt`, `LiveDocTest.kt`.
- Run gradle tests with `ANDROID_HOME` set; do not run `regen-strings.py`.
