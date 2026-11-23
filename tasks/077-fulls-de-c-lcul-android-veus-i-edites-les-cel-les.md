---
title: "Fulls de càlcul (Android): veus i edites les cel·les"
status: todo
depends_on: ["071", "075"]
---

## Why

Parity with the web: a live spreadsheet opens on the phone as a grid that updates live, and the person can change a cell.

## Acceptance criteria

- [ ] The document screen shows a `sheet` as a scrollable grid with column letters, row numbers and sheet tabs, showing the computed values, and applies `block` events in place. A unit test of applying events to a sheet.
- [ ] Tapping a cell shows its raw input in an edit field; saving sends the cell edit with `base_version`, a 409 keeps the person's input with the notice. A unit test of the request and of the conflict state.
- [ ] The Docs home has «Full de càlcul nou», and a sheet has «Descarrega en Excel» (`format=xlsx`). A unit test of the export link.
- [ ] Every string in Catalan and English in `Strings.kt`, edited by hand.

## Out of scope

- Adding sheets from the phone, copy and paste, formatting.

## Notes

- `Docs.kt`, `DocScreen.kt`, `LiveDocTest.kt`; keep the sheet state pure and testable like `LiveDoc`.
- Run gradle tests with `ANDROID_HOME` set; do not run `regen-strings.py`.
