---
title: "Docs (web): el document es veu com una pàgina"
status: todo
depends_on: ["079"]
---

## Why

A document should look like a document: a white page with margins and the same styles the Word export uses, not text spread across the panel in the app's colours.

## Acceptance criteria

- [ ] A text document is drawn as a centred A4-width page with page margins, a light page on a neutral background, in both app themes (the page stays light in dark mode, like Word). In the side panel the page scales to fit the width; in its own window it shows at full page width. A node or CSS test of the classes and a screenshot of panel and window in dark and light themes in the notes.
- [ ] Headings, paragraphs, lists, quotes, code and tables use the fonts, sizes and colours of the active brand theme the Word export uses, so the page and the exported docx look alike. A test that the page styles read the same theme values the export reads (one source of truth, not two copies).
- [ ] The toolbar stays visible at the top while the page scrolls.

## Out of scope

- Page breaks, headers and footers, page numbers.

## Notes

- Theme: the export uses `internal/ooxml/theme.go` and the active brand (`/api/brands/active`); expose what the page needs rather than duplicating values in CSS.
