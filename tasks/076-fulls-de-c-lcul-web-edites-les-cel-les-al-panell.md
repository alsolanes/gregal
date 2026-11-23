---
title: "Fulls de càlcul (web): edites les cel·les al panell"
status: done
depends_on: ["070", "075"]
---

## Why

A live spreadsheet has to be used as a spreadsheet in the app: a grid where the person types values and formulas and sees the results, while the agent can fill it too.

## Acceptance criteria

- [x] The panel (and its own window) shows a `sheet` as a grid with column letters, row numbers and one tab per sheet, showing the computed values, and applies `block` events in place without moving the person's selection. A node test.
- [x] Clicking a cell selects it and shows its raw input (the formula) in a formula bar; typing and Enter or Tab saves through the cell edit route and moves down or right; arrow keys move the selection; Esc cancels. A 409 keeps the person's input with the notice, as in 059. A node test of selection, editing, keys and conflict.
- [x] «Full nou» adds a sheet; the Docs home (070) has «Full de càlcul nou», and a sheet's panel has «Descarrega en Excel» (`format=xlsx`). A node test.
- [x] Every string in Catalan and English; a screenshot of a sheet with a few formulas in dark and light themes in the notes.

## Out of scope

- Copy and paste of ranges, fill handle, column resizing, formatting. Android (077).

## Notes

- Large sheets: draw only the rows in view if a sheet has more than a few hundred rows.
- Desktop: any new module must be in `desktop/package.json` `build.files` (`release.test.js` checks it).
- Done, server: `POST /api/docs` takes `kind:"sheet"` (`CreateSheet(title, "", user)`: one empty sheet `Sheet1`), so the Android «Full de càlcul nou» from 077 works now. `POST /api/docs/{id}/blocks` takes `{after, name}` (`InsertSheet`). Sheet input errors are `gdocs.ErrInvalid` (400 through `docWriteErr`): a bad cell name, a range over the limit, a sheet over 50 000 cells, and any `Workbook` error inside `calc` (a duplicate or bad sheet name). The messages the agent's tool sees are unchanged. `docs/api-contract.md` updated.
- Done, web: new UMD `internal/web/app/sheet-view.js` (`window.gregalSheetView.view(box, doc, render, edit)`, the same shape as deck-view's), loaded by `index.html` and by the doc window page. Layout: sheet tabs with «Full nou» at the end, then the cell name and the formula bar, then the grid (sticky column letters and row numbers, right-aligned numbers). The grid size follows Android's `sheetSize`: one column and five rows past the last cell with data, at least 6 × 20, and it grows with the selection. `docs-panel.js` picks the view by kind, the download is `format=xlsx` with «Descarrega en Excel» / "Download as Excel", and `blockEdits` sends a sheet's body as it is (`{cells}` or `{name}`). The home has «Full de càlcul nou» (empty title: «Full de càlcul sense títol»), and a sheet's kind reads «Full de càlcul» in the lists. No desktop file changed: the desktop window loads the same page from the server.
- Editing: a click selects a cell and shows its raw input in the bar; arrows move the selection when the grid has focus; typing a character on the grid starts editing the cell with that character (it replaces the cell, as in Excel); Enter, F2 or a double click edit the current input. In the bar, Enter saves and moves down, Tab saves and moves right, Esc restores the input and goes back to the grid. An untouched cell is not saved. The base version is the sheet's version when the cell was selected; an event that arrives while the person types does not change the bar or the base, so a concurrent agent edit gives a 409. A 409 keeps the input, shows «L'agent ha canviat aquest full · A1: <current input>» and the sheet as it is now, and the next Enter overwrites against that version. A network error keeps the input.
- Large sheets: over 300 rows only the rows in view (plus 10 above and 20 below) are drawn, with two spacer rows keeping the scroll height; rows are 24 px (`ROW_H`, matched in the panel CSS). Columns are not windowed.
- Tests: `internal/web/app/sheet-view.test.cjs` (grid, events, selection and editing keys, 409 and network error, tabs and «New sheet», windowing), `docs-panel.test.cjs` (sheet view, bodies with scope, xlsx link and label), `doc-home.test.cjs` («Full de càlcul nou», sheet-view.js loaded by both pages), Go `TestNewSheetAndItsSheetsFromThePanel` and `TestSheetRefusesBadInputWith400`.
- Screenshots (synthetic «Pressupost 2027», sums, differences and `ROUND` percentages, a second empty sheet «Resum», D4 selected) from a sandboxed server (`%TEMP%/g076`, isolated HOME/data, fake provider) in the doc window `/app/doc/{id}?scope=mine` with headless Chrome: `output/t076/dark.png` and `light.png` (git-ignored). The same run typed `1500` into C2 and pressed Enter: the server recalculated D2 to 2700 and the selection moved to C3 (the light shot shows the result). Not checked: the packaged desktop app, and the panel next to a chat in the full app.
- Follow-up: clicking another cell while typing drops the input without saving (Excel would save it).
- Follow-up: Delete/Backspace on a selected cell does not clear it; today it is cleared from the bar (Enter, empty the bar, Enter).
- Follow-up: columns are not windowed, so a single cell far to the right (`XFD1`) draws thousands of columns per row (same as 077's `sheetSize` follow-up).
- Follow-up: the new sheet's name is `Full N` / `Sheet N` by interface language, while a new spreadsheet's first sheet is always `Sheet1` (server default).
