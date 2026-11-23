---
title: "Docs (web): color, ressaltat, alineació, font i mida a la barra"
status: todo
depends_on: ["078", "079"]
---

## Why

The last part of the agreed toolbar: the styling markdown cannot express, which 078 added to the block syntax and the Word export.

## Acceptance criteria

- [ ] The toolbar sets text colour and highlight (a small palette plus a custom colour), paragraph alignment (left, centre, right, justify), font family (a short list of fonts that exist in Word on Windows and macOS) and size (8–72 pt); each shows its current value for the selection. A node test per control on the produced 078 markdown.
- [ ] A document styled this way round-trips editor → markdown → editor unchanged and exports to Word with the same styles. A node test of the round trip and a Go test of the export of a block produced by the editor.
- [ ] Every string in Catalan and English.

## Out of scope

- Line spacing, indentation, styles gallery.

## Notes

- Syntax and export: 078. Editor: 079.
