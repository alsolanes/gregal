---
title: "Docs (web): taules i imatges a l'editor"
status: todo
depends_on: ["079"]
---

## Why

A Word replacement needs tables and pictures: inserting a table and adding rows and columns, and putting an image with a caption in the document.

## Acceptance criteria

- [ ] The toolbar inserts a table (rows × columns chosen), and inside a table the person can add and remove rows and columns and type in cells; the table is one block stored as a markdown table. A node test round-trips a table edit to markdown.
- [ ] The toolbar inserts an image from the computer: it is uploaded to the document's folder through a new route (`POST /api/docs/{id}/images`, scope-aware, images only, size-limited) and stored as `![caption](images/<name>)` in its own block; it shows in the editor and in the Word export. Go tests of the route (type and size refused, another project's document 404) and a node test of the insert.
- [ ] Image paths in a document resolve only inside that document's own images folder, both in the editor and in the Word/PowerPoint export; absolute paths and `..` are refused. A Go test. (This closes the follow-up of 072.)
- [ ] Every string in Catalan and English; `docs/api-contract.md` documents the image route.

## Out of scope

- Merged cells, cell colours, image resizing and wrapping.

## Notes

- 072's notes record that slide images and docx `![](path)` can name any image on disk; the third criterion fixes it for both.
