---
title: "Fulls de càlcul: l'agent fa un full viu amb fórmules i el descarregues en Excel"
status: done
depends_on: []
---

## Why

Gregal Docs should also cover spreadsheets. A live spreadsheet is a document of kind `sheet`: named sheets of cells whose formulas the server calculates, so everyone watching sees the results, and it exports to Excel.

## Acceptance criteria

- [x] The store accepts `kind:"sheet"`: each block is one named sheet holding its cells (`A1` → raw input, a number, text or a formula starting with `=`). A cell edit names the sheet, the cell and the sheet block's `base_version`, with the same author, conflict and stream rules as text blocks. A Go test: set cells, a stale base is a 409, two sheets keep their own cells.
- [x] The server calculates every formula with `excelize` (already a dependency) and sends the computed values with the sheet (`values`: cell → shown value, or an error such as `#DIV/0!`, `#REF!`, `#NAME?`), recalculated after each edit, including references to another sheet. A Go test of `=SUM(A1:A3)`, a cross-sheet reference, a division by zero and an unknown function.
- [x] The `doc` tool creates sheets (`create` with `kind:"sheet"`), sets cells (`set_cells` with a map of cells) and `read` returns the inputs and computed values of a range. A tool test.
- [x] `GET /api/docs/{id}/export?format=xlsx` builds the workbook with `internal/xlsxdoc` keeping the formulas; a text document asked for `xlsx` is a 400. A Go test that the exported file reads back with the same formulas and values. The `doc` and `office_create` descriptions send new spreadsheets to `doc` with `kind:"sheet"` (a Go test).
- [x] `docs/api-contract.md` documents the sheet block shape, `values`, `set_cells` and `format=xlsx`.

## Out of scope

- Charts, conditional formats, styles and column widths. Editing in a client (076, 077).

## Notes

- Agreed with the user: a grid of editable cells with Excel formulas calculated by the server with excelize; no charts or conditional formats in the first version.
- `internal/xlsxdoc/xlsx.go` (`Spec`, `SheetSpec`, `SetString` already writes `=…` as a formula, `Open`); excelize's `CalcCellValue` for the values.
- This is the largest task of the set: if it does not fit under the diff guard, split the export (and the description rule) into their own task.
- Done: a sheet block has `name`, `cells` (cell → raw input) and `values` (cell → shown value). `CreateSheet(title, name, author)` makes the document with one empty sheet (`Sheet1` if no name), `InsertSheet` adds one, `SetCells` merges cells into one sheet (empty input clears a cell; cell names are upper-cased and checked with excelize). The generic `Delete` removes a sheet.
- Values: `gdocs.Workbook(d)` builds the excelize workbook (cells written with `xlsxdoc.SetString`, `FullCalcOnLoad` set); `calc` runs inside `create` and `change` for every sheet document, so values are stored with the blocks and recalculated after every edit. An excelize error that is not an Excel error (`not support X function`) is shown as `#NAME?`; a reference to a missing sheet is `#NAME?` too (excelize's choice). A sheet name Excel refuses, or a duplicate (case-insensitive), fails the write.
- Stream: a sheet whose values change because another sheet changed gets a `replace` event without a new block `version` (`changes` compares `values`), so watchers see cross-sheet results without the untouched sheet causing conflicts.
- Cell edit route: `PUT /api/docs/{id}/blocks/{block}` with `{cells, base_version}` (the sheet is the block id), as 076 needs; 409 like text blocks. The `doc` tool: `create` with `kind:"sheet"` and optional `sheet` (first sheet name), `insert` with `sheet` (a new sheet name), `set_cells` (`block`, `cells`), `read` with optional `range` (`A1:D20`, applied to every sheet) printing `B1 = =SUM(A1:A2) → 30` row by row. `emitDocChanged` also opens the panel after `set_cells`.
- `office_create`'s xlsx sentence now sends new spreadsheets to `doc`; I did not touch the text-document wording (069).
- Follow-up: write errors that are not conflicts or not-found (a cell name like `1A`, a duplicate sheet name, markdown sent to a sheet) answer 500 in the block routes instead of 400 (same as 072's note for decks).
- Follow-up: `POST /api/docs/{id}/blocks` only takes `md`; adding a sheet from a client (076's «Full nou») needs a `{after, name}` body.
- Follow-up: the turn note (`notaDocs`) prints `blk.MD`, empty for a sheet; it should list the changed cells.
- Follow-up: the exported xlsx has no cached values, so a viewer that does not recalculate (some previewers) shows empty formula cells; Excel and LibreOffice recalculate on open.
- Follow-up: a sheet has no size limit; a model that writes thousands of cells recalculates the whole workbook on every edit.
