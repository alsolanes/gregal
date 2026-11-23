---
title: "Fulls de càlcul: l'agent fa un full viu amb fórmules i el descarregues en Excel"
status: todo
depends_on: []
---

## Why

Gregal Docs should also cover spreadsheets. A live spreadsheet is a document of kind `sheet`: named sheets of cells whose formulas the server calculates, so everyone watching sees the results, and it exports to Excel.

## Acceptance criteria

- [ ] The store accepts `kind:"sheet"`: each block is one named sheet holding its cells (`A1` → raw input, a number, text or a formula starting with `=`). A cell edit names the sheet, the cell and the sheet block's `base_version`, with the same author, conflict and stream rules as text blocks. A Go test: set cells, a stale base is a 409, two sheets keep their own cells.
- [ ] The server calculates every formula with `excelize` (already a dependency) and sends the computed values with the sheet (`values`: cell → shown value, or an error such as `#DIV/0!`, `#REF!`, `#NAME?`), recalculated after each edit, including references to another sheet. A Go test of `=SUM(A1:A3)`, a cross-sheet reference, a division by zero and an unknown function.
- [ ] The `doc` tool creates sheets (`create` with `kind:"sheet"`), sets cells (`set_cells` with a map of cells) and `read` returns the inputs and computed values of a range. A tool test.
- [ ] `GET /api/docs/{id}/export?format=xlsx` builds the workbook with `internal/xlsxdoc` keeping the formulas; a text document asked for `xlsx` is a 400. A Go test that the exported file reads back with the same formulas and values. The `doc` and `office_create` descriptions send new spreadsheets to `doc` with `kind:"sheet"` (a Go test).
- [ ] `docs/api-contract.md` documents the sheet block shape, `values`, `set_cells` and `format=xlsx`.

## Out of scope

- Charts, conditional formats, styles and column widths. Editing in a client (076, 077).

## Notes

- Agreed with the user: a grid of editable cells with Excel formulas calculated by the server with excelize; no charts or conditional formats in the first version.
- `internal/xlsxdoc/xlsx.go` (`Spec`, `SheetSpec`, `SetString` already writes `=…` as a formula, `Open`); excelize's `CalcCellValue` for the values.
- This is the largest task of the set: if it does not fit under the diff guard, split the export (and the description rule) into their own task.
