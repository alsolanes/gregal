---
title: "Docs: format ampliat (color, ressaltat, alineació, font i mida) que passa a Word"
status: todo
depends_on: []
---

## Why

The person wants Gregal Docs to replace Word with real styling: colour, highlight, alignment, font and size, besides the markdown basics. Markdown has none of these. Blocks must stay text the agent reads and edits, so this task defines a small extension of the block markdown for them, and makes the Word export honour it. The editor (079, 082) and the agent then both write the same syntax.

## Acceptance criteria

- [ ] A block's markdown accepts, besides standard markdown and underline (`++text++`): inline spans `[text]{color=#c0392b bg=#fff3a3 font="Georgia" size=14}` (any subset of the four keys) and a block attribute line `{align=center}` (`left`, `center`, `right`, `justify`) after a paragraph or heading. Unknown keys and malformed values are kept as plain text, never dropped. A Go test of parsing for each key, combinations, nesting with bold/italic, and malformed input.
- [ ] The Word export renders them: run colour, highlight/shading, font family, size in points, underline, and paragraph alignment. A Go test reads the exported docx back and finds each property on the right run or paragraph.
- [ ] The `doc` tool description tells the agent the syntax in one short sentence, and `read` shows blocks with it unchanged. A Go test that a block written with the syntax reads back identical.
- [ ] `docs/api-contract.md` documents the block markdown extension.

## Out of scope

- The editor and its toolbar (079, 082). Presentations and spreadsheets.

## Notes

- Agreed with the user (2026-11-23): Gregal Docs text documents become a WYSIWYG editor with basic styles, tables and images, colour and alignment, font and size; stored as markdown extended for what markdown lacks.
- The span syntax follows Pandoc's bracketed spans with attributes, so it is not invented from scratch; keep the parser strict and small.
- Export path: `internal/ooxml/markdown.go` (`ParseMarkdown`, inline runs), `docx_build.go`, `runs.go`.
