---
title: "Docs (web): color, ressaltat, alineació, font i mida a la barra"
status: done
depends_on: ["078", "079"]
---

## Why

The last part of the agreed toolbar: the styling markdown cannot express, which 078 added to the block syntax and the Word export.

## Acceptance criteria

- [x] The toolbar sets text colour and highlight (a small palette plus a custom colour), paragraph alignment (left, centre, right, justify), font family (a short list of fonts that exist in Word on Windows and macOS) and size (8–72 pt); each shows its current value for the selection. A node test per control on the produced 078 markdown.
- [x] A document styled this way round-trips editor → markdown → editor unchanged and exports to Word with the same styles. A node test of the round trip and a Go test of the export of a block produced by the editor.
- [x] Every string in Catalan and English.

## Out of scope

- Line spacing, indentation, styles gallery.

## Notes

- Syntax and export: 078. Editor: 079.
- Done (2026-11-23): one `span` mark with attrs `color`, `bg`, `font`, `size` (first in mark order, so it serialises outside bold/italic/link: `[**a**]{…}`), written as `[text]{color=… bg=… font="…" size=N}` in that key order; `style(key, value)` changes one key and keeps the others, an empty value removes it. `paragraph` and `heading` get an `align` attr, written as a `{align=…}` line only at top level (inside a list or quote it would be indented and the export would not read it); `align('left')` removes the line. The markdown-it side is a small plugin in `doc-view.js` (`extended`: inline rule `span` before `link`, core rule `align`) with the same strict attribute rules as `internal/ooxml`. `current(state)` gives the toolbar the value for the selection ('' when mixed or unset).
- Toolbar: its own `.doc-style` group (role=group) of five `<select>`s appended after the 079 buttons; colour and highlight have a «Custom colour…» option that opens a hidden `<input type=color>`; a value not in a list (custom colour, a size the agent wrote) shows as an extra option. CSS in `docs-panel.js` (`#docsBody .doc-style`).
- The Go test `internal/ooxml/docx_build_test.go::TestDocxExportaElsEstilsDeLEditor` uses the exact strings the node test «styling with the toolbar writes the blocks that the Word export test reads» asserts; no Go code changed.
- Not checked in a browser (no DOM in the node tests): the selects and the colour picker are untested in the real page.
- Follow-up: nested spans (`[a [b]{…} c]{…}`) parse with the inner one inheriting the outer keys, as the export does, but are written back flat (`[a]{…} [b]{…} [c]{…}`) once the person edits the block.
- Follow-up: `{align=…}` after a quote or list item (which 078's export accepts) is not read by the editor; it stays as text and, if the person edits that block, joins the previous line.
- Follow-up: setting a heading level or paragraph with the 079 buttons drops the block's alignment (`setBlockType` replaces the attrs).
