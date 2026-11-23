---
title: "Gregal Docs (Android): els teus documents des del calaix"
status: todo
depends_on: ["070"]
---

## Why

The same Docs home as on the web, on the phone: open your documents without going through a chat, and start a new one.

## Acceptance criteria

- [ ] The drawer has a «Gregal Docs» entry that opens a Docs home with «Els meus documents» and, when the current chat has a project, «Aquest projecte»; tapping a document opens the existing document screen with the right scope. A unit test of the list parsing and of the scope each item opens with.
- [ ] «Document nou» calls `POST /api/docs` and opens the new document with an empty block ready to type in; any document with no blocks also shows that empty block. A unit test of the request and of the empty-document state.
- [ ] Every string in Catalan and English in `Strings.kt`, edited by hand.

## Out of scope

- Presentations and spreadsheets (074, 077). Renaming or deleting documents.

## Notes

- `Docs.kt`, `DocScreen.kt`, `Screens.kt`, `MainActivity.kt` (drawer), `LiveDocTest.kt`.
- Run gradle tests with `ANDROID_HOME` set; do not run `regen-strings.py`.
