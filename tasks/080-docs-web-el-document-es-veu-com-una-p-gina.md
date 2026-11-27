---
title: "Docs (web): el document es veu com una pàgina"
status: done
depends_on: ["079"]
---

## Why

A document should look like a document: a white page with margins and the same styles the Word export uses, not text spread across the panel in the app's colours.

## Acceptance criteria

- [x] A text document is drawn as a centred A4-width page with page margins, a light page on a neutral background, in both app themes (the page stays light in dark mode, like Word). In the side panel the page scales to fit the width; in its own window it shows at full page width. A node or CSS test of the classes and a screenshot of panel and window in dark and light themes in the notes.
- [x] Headings, paragraphs, lists, quotes, code and tables use the fonts, sizes and colours of the active brand theme the Word export uses, so the page and the exported docx look alike. A test that the page styles read the same theme values the export reads (one source of truth, not two copies).
- [x] The toolbar stays visible at the top while the page scrolls.

## Out of scope

- Page breaks, headers and footers, page numbers.

## Notes

- Theme: the export uses `internal/ooxml/theme.go` and the active brand (`/api/brands/active`); expose what the page needs rather than duplicating values in CSS.
- Built in parallel with 079 and without depending on its internals: the page wraps whatever the text view draws, and `doc-view.js` is untouched.
- Done, server: the docx page size, margins, paragraph spacing and the body, heading 1–3 and code sizes, the code font and the table border colour are now named constants in `internal/ooxml/docx_build.go` (`a4W`, `marginTop`, `szH1`…), used by `NewDocx` and `defaultStylesXML` as before (same values, same output). `ooxml.PageCSS(theme)` turns those constants and the theme's fonts and colours into CSS custom properties (`--doc-page-w`, `--doc-h1-size`, `--doc-h1-color`, `--doc-body-font`…; sizes in points without a unit, colours with `#`, fonts quoted), with the same theme-to-style mapping as the docx (H1/H2 Primary, H3 Dark, quote Accent border and Muted text, code on Light, table header Primary and stripes Light). New read-only route `GET /api/docs/theme` → `{vars}` = `PageCSS(GetTheme(""))`, the theme the Word export uses (the active brand). Not per project; it takes `?scope=mine` like every `/api/docs` route and ignores it. `docs/api-contract.md` updated.
- Done, web (`internal/web/app/docs-panel.js`): for a text document (`kind: doc`) the panel reads `/api/docs/theme`, sets its vars on `#docsBody`, gives it the class `doc-paper` (neutral background: `color-mix` of the app's ink and panel colours, so it is grey in both themes) and puts in it `div.doc-toolbar` (sticky top slot) and `div.doc-page` (the A4 page), and the text view draws into `.doc-page`. Decks, sheets and the Docs home keep `#docsBody` as before (the class is removed). The page is white in both themes. `--u` is one point, or less when the panel is narrower than the page (`min(1pt, 100cqw / page width)`, `#docsBody` is the container), and every size on the page (width, margins, fonts, spacing, borders) is a multiple of it, so the whole page scales down in the side panel and is exactly A4 width (793.7 px) in its own window. Semantic elements inside the page (h1–h3, p, ul/ol, blockquote, pre/code and md.js's `.codeblock`, table/th/td, a, img, hr) are styled from the vars; the chat colours (`.body strong`, `li`…) are reset to inherit, and the doc-view textarea, empty block and 409 notice use page colours. No hex colour inside a `.doc-page` rule except the paper white and the header text white (the node test checks it).
- Toolbar slot for 079: **`.doc-toolbar`**, the first child of `#docsBody` (the scroll container), before `.doc-page`; `position:sticky; top:0`, panel background, a bottom line once it has content (empty, it has no border). 079's toolbar should be appended to `document.querySelector('#docsBody .doc-toolbar')` (or the panel can pass it to the view) rather than drawn inside the page box, which the text view owns. Today the slot is empty: the sticky behaviour is tested in CSS and checked in headless Chrome (after scrolling the body 600 px in the window and 225 px in the panel, the slot's top is still at the body's top), but no toolbar exists in this branch.
- Tests: Go `internal/ooxml/page_css_test.go::TestPageCSSReadsTheDocxExportStyles` (with a registered brand theme set as default, a docx from `NewDocx` with no theme, as the export builds it, has in `styles.xml` and `document.xml` exactly the fonts, sizes, colours, page width and margins `PageCSS` gives), `internal/web/docs_api_test.go::TestDocThemeIsTheActiveExportTheme` (the route returns `PageCSS` of the active theme; POST is 405); node `internal/web/app/docs-panel.test.cjs` «a text document is drawn on a page with the export theme, under a toolbar slot» and «the page CSS takes every font, size and colour from the theme variables».
- Screenshots (synthetic «Informe trimestral» with headings, bold/italic/code/link, nested and numbered lists, a table, a quote, a code block, a rule and long paragraphs) from a sandboxed server (`%TEMP%/g080`, isolated HOME/data, fake provider), headless Chrome, `internal/web/app` served from the worktree: `output/t080/window-dark.png`, `window-light.png` (the doc window `/app/doc/{id}?scope=mine`, 1000 px wide: page 793.7 px, centred) and `panel-dark.png`, `panel-light.png` (the full app at 1300 px with the side panel: page scaled to 427 px). `output/` is git-ignored. The page background is white and H1 is `#1F4E79` Calibri (default theme) in all four.
- Not checked: the packaged desktop app; a real brand pack end to end in the browser (covered by the Go tests); comparing the page side by side with the docx opened in Word.
- Follow-up: in the narrow panel the scaled page text is small (about 6 pt on screen); a zoom control or a minimum scale could help.
- Follow-up: md.js renders a nested list with extra space below the parent item (its `li` content), so a nested list on the page has a gap that the docx does not.
