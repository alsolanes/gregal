---
title: "Fulls de càlcul (Android): veus i edites les cel·les"
status: done
depends_on: ["071", "075"]
---

## Why

Parity with the web: a live spreadsheet opens on the phone as a grid that updates live, and the person can change a cell.

## Acceptance criteria

- [x] The document screen shows a `sheet` as a scrollable grid with column letters, row numbers and sheet tabs, showing the computed values, and applies `block` events in place. A unit test of applying events to a sheet.
- [x] Tapping a cell shows its raw input in an edit field; saving sends the cell edit with `base_version`, a 409 keeps the person's input with the notice. A unit test of the request and of the conflict state.
- [x] The Docs home has «Full de càlcul nou», and a sheet has «Descarrega en Excel» (`format=xlsx`). A unit test of the export link.
- [x] Every string in Catalan and English in `Strings.kt`, edited by hand.

## Out of scope

- Adding sheets from the phone, copy and paste, formatting.

## Notes

- `Docs.kt`, `DocScreen.kt`, `LiveDocTest.kt`; keep the sheet state pure and testable like `LiveDoc`.
- Run gradle tests with `ANDROID_HOME` set; do not run `regen-strings.py`.
- Done: the state is pure, in `Docs.kt`. `DocBlock` now carries `name`, `cells` and `values`, so `LiveDoc.apply` already applies sheet events, including a `replace` that only changes the values. `LiveDoc.sheet` (`@Transient`) is the tab being viewed. `shownSheet()` falls back to the first sheet when that one is deleted, and `showSheet(id)` picks a tab. `sheetColumn(i)` gives the column letters. `sheetSize(block)` gives the columns and rows to draw: one column and five rows past the last cell with data, at least 6 × 20.
- Cell edits reuse `DocEdit` with a new `cell` field. `openCell(sheet, cell)` opens it with the raw input, and its base is the sheet's block `version`. `saveRequest()` sends `PUT /api/docs/{id}/blocks/{sheet}` with `{cells:{cell: input}, base_version}`, plus the scope. An empty input clears the cell and never deletes the sheet. Nothing is sent if the input is untouched. `saved()` reuses the text-block rules: a 200 replaces the sheet with the recalculated one, and a 409 keeps the input and sets `conflict` to the current sheet.
- UI (`DocScreen.kt`): a sheet shows «Descarrega en Excel», a row of sheet tabs, the cell editor when a cell is open, and then the grid. The column-letter row and every grid row share one horizontal `ScrollState`, so they scroll sideways together while the row numbers stay put. Rows are `LazyColumn` items. A cell shows its value, or its raw input if the value is missing. The editor has the field labelled `Sheet!B2` and «Desa». After a 409 it shows «L’agent ha canviat aquest full» and the cell's current input. Tapping another cell switches to it only when the open one has nothing to save; it closes through `skipped()`, so a sheet deleted meanwhile does not stay as a ghost tab.
- «Full de càlcul nou» is under «Document nou» on the home screen. It uses the same title field, and an empty title becomes «Full de càlcul sense títol». It sends `docCreateRequest(title, "sheet")`, which is `POST /api/docs?scope=mine` with `{title, kind:"sheet"}`. In the list, a sheet's kind reads «Full de càlcul».
- **Server left behind:** at the time of this task, `POST /api/docs` only accepts `kind:"doc"` (070; 073 is widening it to `deck`), and `docs/api-contract.md` says any other kind is a 400. Until the route and the contract accept `"sheet"`, «Full de càlcul nou» shows the `docs.createError` notice. The orchestrator will add it when merging. Viewing, editing and downloading sheets the agent makes works today.
- Tested with `LiveDocTest` (6 new tests) and the whole Android unit suite (998 tests, 0 failures). Not tried on a device or emulator with a running server, so the shared horizontal scroll and the focus in the cell editor are not verified.
- Follow-up: switching tabs does not close an open cell editor; it stays above the other sheet's grid, labelled with its own sheet.
- Follow-up: while a cell has unsaved input, tapping another cell does nothing and says nothing. The only way to drop the input is to restore it and save (as with text blocks, there is no cancel).
- Follow-up: `sheetSize` has no upper bound. A single cell far out, such as `XFD1048576`, would make every row compose thousands of cells.
- Follow-up: a stream reconnect reads the document again and resets the tab to the first sheet, as it drops an open editor.
- Follow-up: if the server creates a sheet with no blocks, the screen shows only the download button. Check this once `POST /api/docs` accepts `"sheet"`; 075's `CreateSheet` makes one empty sheet.
- Follow-up: `docExportPath` still builds `format=docx` for a deck (074).
