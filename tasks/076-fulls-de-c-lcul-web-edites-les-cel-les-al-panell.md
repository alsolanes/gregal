---
title: "Fulls de càlcul (web): edites les cel·les al panell"
status: todo
depends_on: ["070", "075"]
---

## Why

A live spreadsheet has to be used as a spreadsheet in the app: a grid where the person types values and formulas and sees the results, while the agent can fill it too.

## Acceptance criteria

- [ ] The panel (and its own window) shows a `sheet` as a grid with column letters, row numbers and one tab per sheet, showing the computed values, and applies `block` events in place without moving the person's selection. A node test.
- [ ] Clicking a cell selects it and shows its raw input (the formula) in a formula bar; typing and Enter or Tab saves through the cell edit route and moves down or right; arrow keys move the selection; Esc cancels. A 409 keeps the person's input with the notice, as in 059. A node test of selection, editing, keys and conflict.
- [ ] «Full nou» adds a sheet; the Docs home (070) has «Full de càlcul nou», and a sheet's panel has «Descarrega en Excel» (`format=xlsx`). A node test.
- [ ] Every string in Catalan and English; a screenshot of a sheet with a few formulas in dark and light themes in the notes.

## Out of scope

- Copy and paste of ranges, fill handle, column resizing, formatting. Android (077).

## Notes

- Large sheets: draw only the rows in view if a sheet has more than a few hundred rows.
- Desktop: any new module must be in `desktop/package.json` `build.files` (`release.test.js` checks it).
