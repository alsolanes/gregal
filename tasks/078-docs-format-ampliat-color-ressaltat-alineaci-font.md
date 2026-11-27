---
title: "Docs: format ampliat (color, ressaltat, alineació, font i mida) que passa a Word"
status: done
depends_on: []
---

## Why

The person wants Gregal Docs to replace Word with real styling: colour, highlight, alignment, font and size, besides the markdown basics. Markdown has none of these. Blocks must stay text the agent reads and edits, so this task defines a small extension of the block markdown for them, and makes the Word export honour it. The editor (079, 082) and the agent then both write the same syntax.

## Acceptance criteria

- [x] A block's markdown accepts, besides standard markdown and underline (`++text++`): inline spans `[text]{color=#c0392b bg=#fff3a3 font="Georgia" size=14}` (any subset of the four keys) and a block attribute line `{align=center}` (`left`, `center`, `right`, `justify`) after a paragraph or heading. Unknown keys and malformed values are kept as plain text, never dropped. A Go test of parsing for each key, combinations, nesting with bold/italic, and malformed input.
- [x] The Word export renders them: run colour, highlight/shading, font family, size in points, underline, and paragraph alignment. A Go test reads the exported docx back and finds each property on the right run or paragraph.
- [x] The `doc` tool description tells the agent the syntax in one short sentence, and `read` shows blocks with it unchanged. A Go test that a block written with the syntax reads back identical.
- [x] `docs/api-contract.md` documents the block markdown extension.

## Out of scope

- The editor and its toolbar (079, 082). Presentations and spreadsheets.

## Notes

- Agreed with the user (2026-11-23): Gregal Docs text documents become a WYSIWYG editor with basic styles, tables and images, colour and alignment, font and size; stored as markdown extended for what markdown lacks.
- The span syntax follows Pandoc's bracketed spans with attributes, so it is not invented from scratch; keep the parser strict and small.
- Export path: `internal/ooxml/markdown.go` (`ParseMarkdown`, inline runs), `docx_build.go`, `runs.go`.
- Done (2026-11-23): `Inline` gains `Underline`, `Color`, `Bg` (hex without `#`), `Font`, `Size` (points); `Block` gains `ParaAlign`. Goldmark leaves spans and `++` as text, so `extendedFormat` reads them over the flat inlines of a paragraph (that is how bold inside a span and a span inside italic both work); code is never touched. The align line becomes a `\x00ALIGN=…\x00` marker before goldmark, like `\pagebreak`; it is put back as text inside code blocks and tables. Word: `w:rFonts`, `w:color`, `w:sz`/`w:szCs` (half points), `w:u single`, `w:shd` fill for `bg` (`w:highlight` only takes named colours), `w:jc` (`justify` → `both`). An align line also works after a quote or list item.
- Size is limited to 1638 pt (Word's maximum); larger is malformed and stays as text.
- Follow-up: `gdocs.Split` (agent `create`/`insert`, `POST /api/docs/{id}/blocks`) puts a heading alone in its block, so `# T\n{align=center}` sent through those becomes two blocks and the align line ends up as plain text after a blank line on export. `replace` keeps it. Split should keep an `{align=…}` line with the heading before it.
- Follow-up: the PowerPoint builder ignores the new `Inline` fields and `ParaAlign`; it now gets the text without the markers.
