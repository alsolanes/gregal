---
title: "Docs (web): taules i imatges a l'editor"
status: done
depends_on: ["079"]
---

## Why

A Word replacement needs tables and pictures: inserting a table and adding rows and columns, and putting an image with a caption in the document.

## Acceptance criteria

- [x] The toolbar inserts a table (rows × columns chosen), and inside a table the person can add and remove rows and columns and type in cells; the table is one block stored as a markdown table. A node test round-trips a table edit to markdown.
- [x] The toolbar inserts an image from the computer: it is uploaded to the document's folder through a new route (`POST /api/docs/{id}/images`, scope-aware, images only, size-limited) and stored as `![caption](images/<name>)` in its own block; it shows in the editor and in the Word export. Go tests of the route (type and size refused, another project's document 404) and a node test of the insert.
- [x] Image paths in a document resolve only inside that document's own images folder, both in the editor and in the Word/PowerPoint export; absolute paths and `..` are refused. A Go test. (This closes the follow-up of 072.)
- [x] Every string in Catalan and English; `docs/api-contract.md` documents the image route.

## Out of scope

- Merged cells, cell colours, image resizing and wrapping.

## Notes

- 072's notes record that slide images and docx `![](path)` can name any image on disk; the third criterion fixes it for both.
- Tables without prosemirror-tables (the vendored bundle is unchanged): `doc-view.js` adds `table` / `table_row` / `table_cell` nodes with `inline*` cells (a markdown cell is one line of inline text), parses GFM tables with markdown-it's `table` rule and writes them back as `| a | b |` with the column alignment of the delimiter row. The first row is the header (bold through CSS). Row/column add and remove rebuild the table node (`reshape`), keeping its block id, so an edit is one PUT. Tab / Shift-Tab move between cells (Tab after the last cell adds a row), Enter does nothing in a cell, Backspace/Delete at a cell edge do not merge cells, and a plugin pads rows left short by a selection deleted across cells.
- Toolbar: its own group `.doc-toolbar-group` after 079's buttons (table, +row, −row, +column, −column, image); the row/column buttons are disabled outside a table. The table size is asked with a prompt («Files × columnes», `3 × 3`), the caption with another one after the file is picked (default: the file name).
- Images: a document's images live in `<data>/docs/<hash>/<id>/images/<random>.<png|jpeg|gif>` (`gdocs.Store.AddImage`, type checked by decoding the bytes). `gdocs.Store.ImagePath` is the one rule for every reader: only `images/<plain name>`. The editor shows only those (`imageName`), fetched through `G().api` as blobs because an `<img src>` carries neither the session header nor the token. The Word and PowerPoint exports use the new `ooxml.Docx/Pptx.ResolveImage` hook instead of `BaseDir`; deck slides no longer resolve against the project root.
- Not checked in a browser: tested with node tests and Go handler tests only.
- Follow-up: the agent's `doc` tool has no way to put an image into a document's folder, so a slide `image` or `![](path)` the agent writes with a project path now shows no picture in the export; it needs an action that copies a project image into `images/`.
- Follow-up: images of a deleted document stay in its folder; nothing deletes documents today.
- Follow-up: the «—» (horizontal line) button inside a table cell inserts the rule at a place ProseMirror picks; it could be disabled in cells like the row/column buttons.
